// Copyright The Linux Foundation.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
)

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------

const (
	regionDefault = "us-east-1"

	// attribute names
	attrDateCreated  = "date_created"
	attrDateModified = "date_modified"

	// update expression helpers
	setPrefix           = "SET "
	commaSep            = ", "
	exprSetDateCreated  = "#date_created = :date_created"
	exprSetDateModified = "#date_modified = :date_modified"
	condAnyMissing      = "attribute_not_exists(#date_created) OR #date_created = :empty OR attribute_not_exists(#date_modified) OR #date_modified = :empty"

	// source labels
	labelFromCreated     = "from_created"
	labelFromModified    = "from_modified"
	labelFivetranSynced  = "fivetran_synced"
	labelNow             = "now"
	labelSignURLCreated  = "signurl_createdat"
	labelSignURLIssued   = "signurl_issuedat"
	labelSignedOn        = "signed_on"
	labelUserDocuSign    = "user_docusign_date_signed"
	labelXMLSigned       = "xml_signed"
	labelXMLCompleted    = "xml_completed"
	labelXMLDateSigned   = "xml_datesigned"
	labelXMLCreated      = "xml_created"
	labelXMLSent         = "xml_sent"
	labelXMLDelivered    = "xml_delivered"
	labelXMLTimeGen      = "xml_timegenerated"
	labelXMLACStatusDate = "xml_acstatusdate"
)

// -----------------------------------------------------------------------------
// Globals & debug helpers
// -----------------------------------------------------------------------------

var debug bool

func getEnvBool(name string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	return v == "1" || v == "true" || v == "yes" || v == "y"
}

func dbg(format string, a ...any) {
	if debug {
		log.Printf("[DEBUG] "+format, a...)
	}
}

// -----------------------------------------------------------------------------
// Models & stats
// -----------------------------------------------------------------------------

type SignatureRecord struct {
	SignatureID            string `dynamodbav:"signature_id"`
	DateCreated            string `dynamodbav:"date_created"`
	DateModified           string `dynamodbav:"date_modified"`
	SignedOn               string `dynamodbav:"signed_on"`
	UserDocusignDateSigned string `dynamodbav:"user_docusign_date_signed"`
	UserDocusignRawXML     string `dynamodbav:"user_docusign_raw_xml"`
	SignatureSignURL       string `dynamodbav:"signature_sign_url"`
	SignatureApproved      bool   `dynamodbav:"signature_approved"`
	SignatureSigned        bool   `dynamodbav:"signature_signed"`
}

type Counter map[string]int

func (c Counter) Inc(label string) { c[label]++ }

type UpdateStats struct {
	Created  Counter
	Modified Counter
}

func newStats() UpdateStats {
	return UpdateStats{Created: Counter{}, Modified: Counter{}}
}

// -----------------------------------------------------------------------------
// Main
// -----------------------------------------------------------------------------

func main() {
	stage := strings.TrimSpace(os.Getenv("STAGE"))
	if stage == "" {
		stage = "dev"
	}
	dryRun := getEnvBool("DRY_RUN")
	allowCurrentTime := getEnvBool("ALLOW_CURRENT_TIME")
	debug = getEnvBool("DEBUG")

	// Snowflake helper & table
	sfCmd := strings.TrimSpace(os.Getenv("SNOWFLAKE_CSV_CMD"))
	if sfCmd == "" {
		sfCmd = "sf_db_csv.sh"
	}
	sfTable := strings.TrimSpace(os.Getenv("SNOWFLAKE_TABLE"))
	if sfTable == "" {
		sfTable = stageToSnowflake(stage)
	}
	sfBatchSize := 500

	// CLI fallback file
	fallbackCLIPath := os.Getenv("FALLBACK_CLI_FILE")
	if fallbackCLIPath == "" {
		fallbackCLIPath = fmt.Sprintf("backfill-fallback-commands-cla-%s-signatures-%s.sh", stage, time.Now().UTC().Format("20060102T150405Z"))
	}
	fmt.Printf("Signature backfill | stage=%s dry-run=%t allow-current-time(after SF)=%t DEBUG=%t\n", stage, dryRun, allowCurrentTime, debug)
	fmt.Printf("Snowflake: table=%s via %s (batch=%d)\n", sfTable, sfCmd, sfBatchSize)

	awsSession, err := session.NewSession(&aws.Config{Region: aws.String(regionDefault)})
	if err != nil {
		log.Fatalf("AWS session error: %v", err)
	}
	region := aws.StringValue(awsSession.Config.Region)
	ddb := dynamodb.New(awsSession)
	tableName := fmt.Sprintf("cla-%s-signatures", stage)

	// Prepare CLI fallback writer (open on first use)
	var cliFile *os.File
	var cliOpen bool
	openCLI := func() {
		if cliOpen {
			return
		}
		// ensure path is clean and inside cwd
		clean := filepath.Clean(fallbackCLIPath)
		//nolint:gosec // path provided by env; repository linter excludes G304; additionally we clean the path.
		f, e := os.Create(clean)
		if e != nil {
			log.Printf("WARN: cannot create %s: %v", clean, e)
			return
		}
		cliFile = f
		if _, e := fmt.Fprintf(cliFile, "#!/usr/bin/env bash\nset -euo pipefail\n# generated %s UTC, stage=%s, table=%s\n\n", time.Now().UTC().Format(time.RFC3339), stage, tableName); e != nil {
			log.Printf("WARN: writing header to %s: %v", clean, e)
		}
		cliOpen = true
	}
	defer func() {
		if cliFile != nil {
			_ = cliFile.Close()
		}
	}()

	stats := newStats()

	// 1) First pass: DDB only, created=earliest, modified=latest; no Snowflake, no now()
	updated, cliCount, pending, err := firstPassScanAndUpdate(
		context.Background(), ddb, tableName, stage, region, dryRun, &stats,
		func(cmd string) {
			openCLI()
			if cliFile != nil {
				if _, werr := fmt.Fprintln(cliFile, cmd); werr != nil {
					log.Printf("WARN: could not append CLI line: %v", werr)
				}
			}
		},
	)
	if err != nil {
		log.Fatalf("First pass failed: %v", err)
	}

	// 2) Snowflake pass for “no candidate” rows (fills from _FIVETRAN_SYNCED)
	sfFixed, sfCliCount := snowflakeFix(
		ddb, tableName, stage, region, dryRun, &stats, pending, sfCmd, sfTable, sfBatchSize,
		func(cmd string) {
			openCLI()
			if cliFile != nil {
				if _, werr := fmt.Fprintln(cliFile, cmd); werr != nil {
					log.Printf("WARN: could not append CLI line: %v", werr)
				}
			}
		},
	)
	cliCount += sfCliCount
	updated += sfFixed

	// 3) Final now() pass (only if allowed)
	nowFixed, nowCliCount := finalNowFix(
		ddb, tableName, stage, region, dryRun, &stats, pending, allowCurrentTime,
		func(cmd string) {
			openCLI()
			if cliFile != nil {
				if _, werr := fmt.Fprintln(cliFile, cmd); werr != nil {
					log.Printf("WARN: could not append CLI line: %v", werr)
				}
			}
		},
	)
	cliCount += nowCliCount
	updated += nowFixed
	skipped := len(pending)

	fmt.Printf("\nCompleted. Updated: %d  |  Still pending (skipped): %d\n", updated, skipped)
	if cliOpen {
		fmt.Printf("Fallback AWS CLI written to: %s (lines: %d)\n", fallbackCLIPath, cliCount)
	}
	printStats(stats)
}

// -----------------------------------------------------------------------------
// First pass (DDB only) — created = earliest, modified = latest
// -----------------------------------------------------------------------------

type pendingInfo struct {
	Record      SignatureRecord
	MissingC    bool
	MissingM    bool
	NoCandidate bool
}

// firstPassScanAndUpdate scans DynamoDB for signed+approved signatures missing
// created/modified, picks physical candidates, and emits updates.
//
//nolint:gocyclo
func firstPassScanAndUpdate(
	ctx context.Context,
	ddb *dynamodb.DynamoDB,
	tableName, stage, region string,
	dryRun bool,
	stats *UpdateStats,
	emitCLI func(string),
) (updated int, cliCount int, pending map[string]*pendingInfo, err error) {
	pending = map[string]*pendingInfo{}

	// Only approved+signed with missing/empty/NULL dates
	missingCreated := expression.Or(
		expression.AttributeNotExists(expression.Name(attrDateCreated)),
		expression.Equal(expression.Name(attrDateCreated), expression.Value("")),
		expression.AttributeType(expression.Name(attrDateCreated), "NULL"),
	)
	missingModified := expression.Or(
		expression.AttributeNotExists(expression.Name(attrDateModified)),
		expression.Equal(expression.Name(attrDateModified), expression.Value("")),
		expression.AttributeType(expression.Name(attrDateModified), "NULL"),
	)
	missingAny := expression.Or(missingCreated, missingModified)
	approvedAndSigned := expression.And(
		expression.Equal(expression.Name("signature_approved"), expression.Value(true)),
		expression.Equal(expression.Name("signature_signed"), expression.Value(true)),
	)
	filter := expression.And(missingAny, approvedAndSigned)

	proj := expression.NamesList(
		expression.Name("signature_id"),
		expression.Name(attrDateCreated),
		expression.Name(attrDateModified),
		expression.Name("signed_on"),
		expression.Name("user_docusign_date_signed"),
		expression.Name("user_docusign_raw_xml"),
		expression.Name("signature_sign_url"),
		expression.Name("signature_approved"),
		expression.Name("signature_signed"),
	)

	expr, e := expression.NewBuilder().WithFilter(filter).WithProjection(proj).Build()
	if e != nil {
		return 0, 0, nil, fmt.Errorf("build expression: %w", e)
	}

	scan := &dynamodb.ScanInput{
		TableName:                 aws.String(tableName),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}

	var pageErr error
	err = ddb.ScanPagesWithContext(ctx, scan, func(page *dynamodb.ScanOutput, last bool) bool {
		var rows []SignatureRecord
		if uerr := dynamodbattribute.UnmarshalListOfMaps(page.Items, &rows); uerr != nil {
			pageErr = fmt.Errorf("unmarshal page: %w", uerr)
			return false
		}

		for _, sig := range rows {
			mC := isMissing(sig.DateCreated)
			mM := isMissing(sig.DateModified)
			if !mC && !mM {
				continue
			}

			dbg("ID=%s missingCreated=%t missingModified=%t created=%q modified=%q", sig.SignatureID, mC, mM, sig.DateCreated, sig.DateModified)

			// Physical candidates only (no Snowflake, no now)
			cands := collectPhysicalCandidates(sig)
			if debug {
				for i, c := range cands {
					dbg("  candidate[%d]: src=%s ts=%s", i, c.src, c.ts)
				}
			}

			earliest := pickEarliest(cands) // for created
			latest := pickLatest(cands)     // for modified
			if debug {
				dbg("  earliest: src=%s ts=%s", earliest.src, earliest.ts)
				dbg("  latest  : src=%s ts=%s", latest.src, latest.ts)
			}

			// Decide created
			var newC, srcC string
			if mC {
				if !isMissing(earliest.ts) {
					// If modified exists and earliest > modified, clamp to modified
					if !isMissing(sig.DateModified) && after(earliest.ts, sig.DateModified) {
						dbg("  clamp created: earliest(%s) > modified(%s) -> use modified", earliest.ts, sig.DateModified)
						newC, srcC = normalize(sig.DateModified), labelFromModified
					} else {
						newC, srcC = normalize(earliest.ts), earliest.src
					}
				} else if !isMissing(sig.DateModified) {
					// Fallback for created
					newC, srcC = normalize(sig.DateModified), labelFromModified
				}
			}

			// Decide modified
			var newM, srcM string
			if mM {
				if !isMissing(latest.ts) {
					// Ensure modified >= created
					switch {
					case !isMissing(sig.DateCreated) && after(sig.DateCreated, latest.ts):
						newM, srcM = normalize(sig.DateCreated), labelFromCreated
					case mC && newC != "" && after(newC, latest.ts):
						newM, srcM = newC, labelFromCreated
					default:
						newM, srcM = normalize(latest.ts), latest.src
					}
				} else {
					dbg("  no physical candidate for modified; defer to Snowflake/_FIVETRAN_SYNCED if still missing")
				}
			}

			// Proposed finals (respect existing values, keep monotonic ordering)
			finalC := ifEmpty(sig.DateCreated, newC)
			finalM := ifEmpty(sig.DateModified, newM)

			// Decide what we can actually set
			setCreated := mC && finalC != ""
			setModified := mM && finalM != ""

			// If neither field can be set now, queue for later passes
			if !setCreated && !setModified {
				dbg("  -> no updatable fields in-pass; mark pending for SF/now")
				pending[sig.SignatureID] = &pendingInfo{
					Record:      sig,
					MissingC:    mC,
					MissingM:    mM,
					NoCandidate: isMissing(earliest.ts) && isMissing(latest.ts),
				}
				continue
			}

			// Monotonic clamp once more (created ≤ modified)
			tc := parseTime(finalC)
			tm := parseTime(finalM)
			if !tc.IsZero() && !tm.IsZero() && tm.Before(tc) {
				dbg("  clamp modified: modified(%s) < created(%s) -> set modified=created", finalM, finalC)
				finalM = finalC
				srcM = labelFromCreated
				setModified = mM // only set if it was missing
			}

			updateExpr := setPrefix
			vals := map[string]*dynamodb.AttributeValue{":empty": {S: aws.String("")}}
			names := map[string]*string{
				"#date_created":  aws.String(attrDateCreated),
				"#date_modified": aws.String(attrDateModified),
			}
			first := true
			if setCreated {
				if !first {
					updateExpr += commaSep
				}
				updateExpr += exprSetDateCreated
				vals[":date_created"] = &dynamodb.AttributeValue{S: aws.String(finalC)}
				first = false
			}
			if setModified {
				if !first {
					updateExpr += commaSep
				}
				updateExpr += exprSetDateModified
				vals[":date_modified"] = &dynamodb.AttributeValue{S: aws.String(finalM)}
			}

			if debug {
				dbg("  updateExpr=%s", updateExpr)
				if v, ok := vals[":date_created"]; ok && v.S != nil {
					dbg("  :date_created=%s (src=%s)", *v.S, srcC)
				}
				if v, ok := vals[":date_modified"]; ok && v.S != nil {
					dbg("  :date_modified=%s (src=%s)", *v.S, srcM)
				}
			}

			// Stats
			if setCreated {
				stats.Created.Inc(srcC)
				stats.Created.Inc("_total")
			}
			if setModified {
				stats.Modified.Inc(srcM)
				stats.Modified.Inc("_total")
			}

			// Build CLI (always emitted in dry-run; emitted on failure in live-run)
			cmd := buildAwsCliUpdate(region, stage, tableName, sig.SignatureID, updateExpr, names, vals, condAnyMissing)
			dbg("  CLI: %s", cmd)

			if dryRun {
				if emitCLI != nil {
					emitCLI(cmd)
					cliCount++
				}
				updated++
				continue
			}

			_, uerr := ddb.UpdateItemWithContext(ctx, &dynamodb.UpdateItemInput{
				TableName:                 aws.String(tableName),
				Key:                       map[string]*dynamodb.AttributeValue{"signature_id": {S: aws.String(sig.SignatureID)}},
				UpdateExpression:          aws.String(updateExpr),
				ExpressionAttributeNames:  names,
				ExpressionAttributeValues: vals,
				ConditionExpression:       aws.String(condAnyMissing),
			})
			if uerr != nil {
				log.Printf("Update failed %s: %v", sig.SignatureID, uerr)
				if emitCLI != nil {
					emitCLI(cmd)
					cliCount++
				}
				continue
			}
			updated++
		}
		return true
	})

	if err != nil {
		return updated, cliCount, nil, fmt.Errorf("scan failed: %w", err)
	}
	if pageErr != nil {
		return updated, cliCount, nil, pageErr
	}
	return updated, cliCount, pending, nil
}

// -----------------------------------------------------------------------------
// Snowflake pass
// -----------------------------------------------------------------------------

// snowflakeFix fills remaining missing values from _FIVETRAN_SYNCED,
// keeping monotonic ordering and using created->modified when helpful.
//
//nolint:gocyclo
func snowflakeFix(
	ddb *dynamodb.DynamoDB,
	tableName, stage, region string,
	dryRun bool,
	stats *UpdateStats,
	pending map[string]*pendingInfo,
	sfCmd, sfTable string,
	batchSize int,
	emitCLI func(string),
) (fixed int, cliCount int) {

	var ids []string
	for id, p := range pending {
		if p.NoCandidate && (p.MissingC || p.MissingM) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return 0, 0
	}

	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]

		// Compose SQL
		inList := "'" + strings.Join(chunk, "','") + "'"
		sql := fmt.Sprintf(`SELECT signature_id, _FIVETRAN_SYNCED FROM %s WHERE signature_id IN (%s)`, sfTable, inList)
		dbg("Snowflake batch %d..%d of %d, SQL: %s", start, end-1, len(ids), sql)

		out, e := runSnowflakeCSV(sfCmd, sql)
		if e != nil {
			log.Printf("WARN: Snowflake batch failed: %v", e)
			continue
		}

		sfMap := parseSnowflakeCSV(out)
		dbg("Snowflake returned %d rows", len(sfMap))

		for id, ts := range sfMap {
			info, ok := pending[id]
			if !ok || (!info.MissingC && !info.MissingM) {
				continue
			}
			created := info.Record.DateCreated
			modified := info.Record.DateModified
			mC := info.MissingC
			mM := info.MissingM

			var newC, srcC string
			var newM, srcM string

			// CREATED: use _fivetran_synced; clamp to modified if needed
			if mC {
				newC, srcC = normalize(ts), labelFivetranSynced
				if !isMissing(modified) && after(newC, modified) {
					dbg("  SF clamp created: fivetran(%s) > modified(%s) -> modified", newC, modified)
					newC, srcC = normalize(modified), labelFromModified
				}
			}

			// MODIFIED: prefer created (if exists/being set), else _fivetran_synced
			if mM {
				switch {
				case !isMissing(created):
					newM, srcM = normalize(created), labelFromCreated
				case mC && newC != "":
					newM, srcM = newC, labelFromCreated
				default:
					newM, srcM = normalize(ts), labelFivetranSynced
				}
			}

			finalC := ifEmpty(created, newC)
			finalM := ifEmpty(modified, newM)

			setCreated := mC && finalC != ""
			setModified := mM && finalM != ""

			if !setCreated && !setModified {
				continue
			}

			// Monotonic clamp
			tc := parseTime(finalC)
			tm := parseTime(finalM)
			if !tc.IsZero() && !tm.IsZero() && tm.Before(tc) {
				dbg("  SF clamp modified: modified(%s) < created(%s) -> created", finalM, finalC)
				finalM = finalC
				srcM = labelFromCreated
				setModified = mM
			}

			updateExpr := setPrefix
			vals := map[string]*dynamodb.AttributeValue{":empty": {S: aws.String("")}}
			names := map[string]*string{
				"#date_created":  aws.String(attrDateCreated),
				"#date_modified": aws.String(attrDateModified),
			}
			first := true
			if setCreated {
				if !first {
					updateExpr += commaSep
				}
				updateExpr += exprSetDateCreated
				vals[":date_created"] = &dynamodb.AttributeValue{S: aws.String(finalC)}
				first = false
			}
			if setModified {
				if !first {
					updateExpr += commaSep
				}
				updateExpr += exprSetDateModified
				vals[":date_modified"] = &dynamodb.AttributeValue{S: aws.String(finalM)}
			}

			// Stats
			if setCreated {
				stats.Created.Inc(srcC)
				stats.Created.Inc("_total")
			}
			if setModified {
				stats.Modified.Inc(srcM)
				stats.Modified.Inc("_total")
			}

			cmd := buildAwsCliUpdate(region, stage, tableName, id, updateExpr, names, vals, condAnyMissing)
			dbg("  SF CLI: %s", cmd)

			if dryRun {
				if emitCLI != nil {
					emitCLI(cmd)
					cliCount++
				}
				fixed++
				delete(pending, id)
				continue
			}

			_, uerr := ddb.UpdateItem(&dynamodb.UpdateItemInput{
				TableName:                 aws.String(tableName),
				Key:                       map[string]*dynamodb.AttributeValue{"signature_id": {S: aws.String(id)}},
				UpdateExpression:          aws.String(updateExpr),
				ExpressionAttributeNames:  names,
				ExpressionAttributeValues: vals,
				ConditionExpression:       aws.String(condAnyMissing),
			})
			if uerr != nil {
				log.Printf("Update failed (SF) %s: %v", id, uerr)
				if emitCLI != nil {
					emitCLI(cmd)
					cliCount++
				}
				continue
			}
			fixed++
			delete(pending, id)
		}
	}
	return fixed, cliCount
}

// -----------------------------------------------------------------------------
// Final now()-fill pass (only if allowed)
// -----------------------------------------------------------------------------

func finalNowFix(
	ddb *dynamodb.DynamoDB,
	tableName, stage, region string,
	dryRun bool,
	stats *UpdateStats,
	pending map[string]*pendingInfo,
	allowNow bool,
	emitCLI func(string),
) (fixed int, cliCount int) {
	if !allowNow || len(pending) == 0 {
		return 0, 0
	}
	now := time.Now().UTC().Format(time.RFC3339)

	for id, info := range pending {
		mC := info.MissingC
		mM := info.MissingM
		if !mC && !mM {
			delete(pending, id)
			continue
		}

		var newC, srcC string
		if mC {
			if !isMissing(info.Record.DateModified) {
				newC, srcC = normalize(info.Record.DateModified), labelFromModified
			} else {
				newC, srcC = now, labelNow
			}
		}

		var newM, srcM string
		if mM {
			switch {
			case !isMissing(info.Record.DateCreated):
				newM, srcM = normalize(info.Record.DateCreated), labelFromCreated
			case mC && newC != "":
				newM, srcM = newC, labelFromCreated
			default:
				newM, srcM = now, labelNow
			}
		}

		finalC := ifEmpty(info.Record.DateCreated, newC)
		finalM := ifEmpty(info.Record.DateModified, newM)

		setCreated := mC && finalC != ""
		setModified := mM && finalM != ""
		if !setCreated && !setModified {
			delete(pending, id)
			continue
		}

		// Monotonic clamp
		tc := parseTime(finalC)
		tm := parseTime(finalM)
		if !tc.IsZero() && !tm.IsZero() && tm.Before(tc) {
			finalM = finalC
			srcM = labelFromCreated
			setModified = mM
		}

		updateExpr := setPrefix
		vals := map[string]*dynamodb.AttributeValue{":empty": {S: aws.String("")}}
		names := map[string]*string{
			"#date_created":  aws.String(attrDateCreated),
			"#date_modified": aws.String(attrDateModified),
		}
		first := true
		if setCreated {
			if !first {
				updateExpr += commaSep
			}
			updateExpr += exprSetDateCreated
			vals[":date_created"] = &dynamodb.AttributeValue{S: aws.String(finalC)}
			first = false
		}
		if setModified {
			if !first {
				updateExpr += commaSep
			}
			updateExpr += exprSetDateModified
			vals[":date_modified"] = &dynamodb.AttributeValue{S: aws.String(finalM)}
		}

		// Stats
		if setCreated {
			stats.Created.Inc(srcC)
			stats.Created.Inc("_total")
		}
		if setModified {
			stats.Modified.Inc(srcM)
			stats.Modified.Inc("_total")
		}

		cmd := buildAwsCliUpdate(region, stage, tableName, id, updateExpr, names, vals, condAnyMissing)
		dbg("  NOW CLI: %s", cmd)

		if dryRun {
			if emitCLI != nil {
				emitCLI(cmd)
				cliCount++
			}
			fixed++
			delete(pending, id)
			continue
		}

		_, uerr := ddb.UpdateItem(&dynamodb.UpdateItemInput{
			TableName:                 aws.String(tableName),
			Key:                       map[string]*dynamodb.AttributeValue{"signature_id": {S: aws.String(id)}},
			UpdateExpression:          aws.String(updateExpr),
			ExpressionAttributeNames:  names,
			ExpressionAttributeValues: vals,
			ConditionExpression:       aws.String(condAnyMissing),
		})
		if uerr != nil {
			log.Printf("Update failed (now) %s: %v", id, uerr)
			if emitCLI != nil {
				emitCLI(cmd)
				cliCount++
			}
			continue
		}
		fixed++
		delete(pending, id)
	}
	return fixed, cliCount
}

// -----------------------------------------------------------------------------
// Candidate collection & selection
// -----------------------------------------------------------------------------

type pair struct {
	ts  string
	src string
}

// Collect physical candidates in fixed priority order:
//
// signed_on
// user_docusign_date_signed
// (DocuSign XML, ordered): Signed, Completed, DateSigned, Created, Sent, Delivered, TimeGenerated, ACStatusDate
// signature_sign_url: CreatedAt, IssuedAt
func collectPhysicalCandidates(sig SignatureRecord) []pair {
	var out []pair
	push := func(val, src string) {
		val = strings.TrimSpace(val)
		if val != "" {
			out = append(out, pair{normalize(val), src})
		}
	}

	// 1) signed_on
	if sig.SignedOn != "" {
		push(sig.SignedOn, labelSignedOn)
	}

	// 2) user_docusign_date_signed
	if sig.UserDocusignDateSigned != "" {
		push(sig.UserDocusignDateSigned, labelUserDocuSign)
	}

	// 3) DocuSign XML (ordered)
	for _, p := range extractDocuSignXMLOrdered(sig.UserDocusignRawXML) {
		push(p.ts, p.src)
	}

	// 4) signature_sign_url (CreatedAt, then IssuedAt)
	if ts, lbl := createdOrIssuedAtFromSignURL(sig.SignatureSignURL); ts != "" {
		push(ts, lbl)
	}

	return out
}

func pickEarliest(cands []pair) pair {
	var (
		best pair
		init bool
	)
	for _, c := range cands {
		t := parseTime(c.ts)
		if t.IsZero() {
			continue
		}
		if !init || t.Before(parseTime(best.ts)) {
			best = c
			init = true
		}
	}
	if init {
		return best
	}
	if len(cands) > 0 {
		return cands[0]
	}
	return pair{}
}

func pickLatest(cands []pair) pair {
	var (
		best pair
		init bool
	)
	for _, c := range cands {
		t := parseTime(c.ts)
		if t.IsZero() {
			continue
		}
		if !init || t.After(parseTime(best.ts)) {
			best = c
			init = true
		}
	}
	if init {
		return best
	}
	if len(cands) > 0 {
		return cands[0]
	}
	return pair{}
}

// -----------------------------------------------------------------------------
// Parsing helpers (DocuSign XML + sign URL)
// -----------------------------------------------------------------------------

var (
	reSigned        = regexp.MustCompile(`(?i)<Signed>([^<]+)</Signed>`)
	reCompleted     = regexp.MustCompile(`(?i)<Completed>([^<]+)</Completed>`)
	reCreated       = regexp.MustCompile(`(?i)<Created>([^<]+)</Created>`)
	reSent          = regexp.MustCompile(`(?i)<Sent>([^<]+)</Sent>`)
	reDelivered     = regexp.MustCompile(`(?i)<Delivered>([^<]+)</Delivered>`)
	reTimeGenerated = regexp.MustCompile(`(?i)<TimeGenerated>([^<]+)</TimeGenerated>`)
	reACStatusDate  = regexp.MustCompile(`(?i)<ACStatusDate>([^<]+)</ACStatusDate>`)

	// DateSigned can appear as TabStatus->TabType DateSigned/TabValue
	reDateSigned1 = regexp.MustCompile(`(?is)<TabStatus>.*?<TabType>\s*DateSigned\s*</TabType>.*?<TabValue>\s*([^<]+)\s*</TabValue>.*?</TabStatus>`)
	// Or in XFDF fields
	reDateSigned2 = regexp.MustCompile(`(?is)<field\s+name="DateSigned"\s*>\s*<value>\s*([^<]+)\s*</value>\s*</field>`)
)

func extractDocuSignXMLOrdered(raw string) []pair {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []pair
	grab := func(re *regexp.Regexp, label string) {
		if m := re.FindStringSubmatch(raw); len(m) == 2 && strings.TrimSpace(m[1]) != "" {
			out = append(out, pair{strings.TrimSpace(m[1]), label})
		}
	}
	// Strict internal order
	grab(reSigned, labelXMLSigned)
	grab(reCompleted, labelXMLCompleted)
	if ds := extractDateSignedFromXML(raw); ds != "" {
		out = append(out, pair{normalizeDateSigned(ds), labelXMLDateSigned})
	}
	grab(reCreated, labelXMLCreated)
	grab(reSent, labelXMLSent)
	grab(reDelivered, labelXMLDelivered)
	grab(reTimeGenerated, labelXMLTimeGen)
	grab(reACStatusDate, labelXMLACStatusDate)
	return out
}

func extractDateSignedFromXML(raw string) string {
	if m := reDateSigned1.FindStringSubmatch(raw); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	if m := reDateSigned2.FindStringSubmatch(raw); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// DateSigned sample "11/17/2025 | 5:30 AM PST"
func normalizeDateSigned(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if t, err := time.Parse("01/02/2006 | 3:04 PM MST", s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	if t, err := time.Parse("01/02/2006 | 3:04 PM", s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	return s
}

func createdOrIssuedAtFromSignURL(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", ""
	}
	slt := u.Query().Get("slt")
	if slt == "" {
		dec, derr := url.QueryUnescape(raw)
		if derr != nil {
			dec = raw
		}
		if ts := extractJSONField(dec, "CreatedAt"); ts != "" {
			return normalize(ts), labelSignURLCreated
		}
		if ts := extractJSONField(dec, "IssuedAt"); ts != "" {
			return normalize(ts), labelSignURLIssued
		}
		return "", ""
	}
	parts := strings.Split(slt, ".")
	for _, seg := range parts {
		if ts := extractBase64JSON(seg, "CreatedAt"); ts != "" {
			return normalize(ts), labelSignURLCreated
		}
		if ts := extractBase64JSON(seg, "IssuedAt"); ts != "" {
			return normalize(ts), labelSignURLIssued
		}
	}
	return "", ""
}

func extractBase64JSON(seg, key string) string {
	decoders := []func(string) ([]byte, error){
		base64.RawURLEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.StdEncoding.DecodeString,
	}
	for _, d := range decoders {
		b, err := d(seg)
		if err != nil || len(b) == 0 {
			continue
		}
		if ts := extractJSONField(string(b), key); ts != "" {
			return ts
		}
	}
	return ""
}

func extractJSONField(s, field string) string {
	p := `"` + field + `"\s*:\s*"(.*?)"`
	re := regexp.MustCompile(p)
	m := re.FindStringSubmatch(s)
	if len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// -----------------------------------------------------------------------------
// Time helpers
// -----------------------------------------------------------------------------

func isMissing(v string) bool { return strings.TrimSpace(v) == "" }

func ifEmpty(existing, candidate string) string {
	if isMissing(existing) {
		return normalize(candidate)
	}
	return normalize(existing)
}

func normalize(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	// RFC3339(/Nano)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	// Snowflake/Fivetran formats
	layouts := []string{
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05.999 -0700",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	for _, L := range layouts {
		if t, err := time.Parse(L, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return s
}

func after(a, b string) bool {
	ta, ea := time.Parse(time.RFC3339, normalize(a))
	tb, eb := time.Parse(time.RFC3339, normalize(b))
	if ea != nil || eb != nil {
		return false
	}
	return ta.After(tb)
}

func parseTime(s string) time.Time {
	if strings.TrimSpace(s) == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func stageToSnowflake(stage string) string {
	up := strings.ToUpper(strings.TrimSpace(stage))
	switch up {
	case "PROD", "PRODUCTION":
		return "FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_SIGNATURES"
	default:
		// Keep the known DEV name; adjust if your Snowflake schema differs.
		return "FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST1_DEV.CLA_DEV_SIGNATURES"
	}
}

// -----------------------------------------------------------------------------
// Snowflake execution & CSV parse
// -----------------------------------------------------------------------------

func runSnowflakeCSV(cmdPath, sql string) ([]byte, error) {
	cmd := exec.Command(cmdPath) // reads SQL on stdin, prints CSV on stdout
	cmd.Stdin = strings.NewReader(sql)
	return cmd.Output()
}

func parseSnowflakeCSV(b []byte) map[string]string {
	res := map[string]string{}
	r := csv.NewReader(strings.NewReader(string(b)))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil || len(rows) == 0 {
		return res
	}
	// Find header indexes
	h := rows[0]
	idxID, idxTS := -1, -1
	for i, col := range h {
		l := strings.ToLower(strings.TrimSpace(col))
		if l == "signature_id" {
			idxID = i
		}
		if l == "_fivetran_synced" || strings.Contains(l, "synced") || strings.Contains(l, "time") || strings.Contains(l, "timestamp") {
			if idxTS == -1 {
				idxTS = i
			}
		}
	}
	if idxID == -1 || idxTS == -1 {
		return res
	}
	for _, row := range rows[1:] {
		if len(row) <= idxID || len(row) <= idxTS {
			continue
		}
		id := strings.TrimSpace(row[idxID])
		ts := strings.TrimSpace(row[idxTS])
		if id == "" || ts == "" {
			continue
		}
		res[id] = normalize(ts)
	}
	return res
}

// -----------------------------------------------------------------------------
// AWS CLI builder & stats print
// -----------------------------------------------------------------------------

func buildAwsCliUpdate(region, stage, table, sigID, updateExpr string, names map[string]*string, values map[string]*dynamodb.AttributeValue, condExpr string) string {
	key := map[string]map[string]string{"signature_id": {"S": sigID}}
	namesFlat := map[string]string{}
	for k, v := range names {
		if v != nil {
			namesFlat[k] = *v
		}
	}
	valsFlat := map[string]map[string]string{":empty": {"S": ""}}
	if av, ok := values[":date_created"]; ok && av != nil && av.S != nil {
		valsFlat[":date_created"] = map[string]string{"S": *av.S}
	}
	if av, ok := values[":date_modified"]; ok && av != nil && av.S != nil {
		valsFlat[":date_modified"] = map[string]string{"S": *av.S}
	}

	kb, kerr := json.Marshal(key)
	if kerr != nil {
		kb = []byte(fmt.Sprintf(`{"signature_id":{"S":"%s"}}`, sigID))
	}
	nb, nerr := json.Marshal(namesFlat)
	if nerr != nil {
		nb = []byte(`{"#date_created":"date_created","#date_modified":"date_modified"}`)
	}
	vb, verr := json.Marshal(valsFlat)
	if verr != nil {
		vb = []byte(`{":empty":{"S":""}}`)
	}

	return fmt.Sprintf(
		"aws --profile lfproduct-%s --region %s dynamodb update-item --table-name %s --key '%s' --update-expression '%s' --expression-attribute-names '%s' --expression-attribute-values '%s' --condition-expression '%s'",
		stage, region, table, kb, updateExpr, nb, vb, condExpr,
	)
}

func printStats(stats UpdateStats) {
	fmt.Println("\nUpdate statistics:")
	print := func(title string, c Counter) {
		total := c["_total"]
		fmt.Printf("  %s updated %d time(s)\n", title, total)
		keys := make([]string, 0, len(c))
		for k := range c {
			if k == "_total" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("    - %-22s %d\n", k, c[k])
		}
	}
	print(attrDateCreated, stats.Created)
	print(attrDateModified, stats.Modified)
}
