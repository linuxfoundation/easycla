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
// Globals & constants
// -----------------------------------------------------------------------------

var debug bool

const (
	// labels/sources
	labelFromCreated         = "from_created"
	labelFromModified        = "from_modified"
	labelFivetranSynced      = "fivetran_synced"
	labelNow                 = "now"
	labelSignURLCreatedAt    = "signurl_createdat"
	labelSignURLIssuedAt     = "signurl_issuedat"
	labelTotal               = "_total"
	physicalSourceSignedOn   = "signed_on"
	physicalSourceUDS        = "user_docusign_date_signed"
	physicalXMLSigned        = "xml_signed"
	physicalXMLCompleted     = "xml_completed"
	physicalXMLDateSigned    = "xml_datesigned"
	physicalXMLCreated       = "xml_created"
	physicalXMLSent          = "xml_sent"
	physicalXMLDelivered     = "xml_delivered"
	physicalXMLTimeGenerated = "xml_timegenerated"
	physicalXMLACStatusDate  = "xml_acstatusdate"

	// update expression bits
	updateKWSet              = "SET "
	assignDateCreated        = "#date_created = :date_created"
	assignDateModified       = "#date_modified = :date_modified"
	condMissingEither        = "attribute_not_exists(#date_created) OR #date_created = :empty OR attribute_not_exists(#date_modified) OR #date_modified = :empty"
	sepComma                 = ", "
	attrDateCreatedName      = "date_created"
	attrDateModifiedName     = "date_modified"
	exprNameDateCreatedAlias = "#date_created"
	exprNameDateModAlias     = "#date_modified"
	exprValDateCreatedAlias  = ":date_created"
	exprValDateModAlias      = ":date_modified"
	exprValEmptyAlias        = ":empty"
)

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

	// CLI fallback file (sanitize path; keep in CWD)
	fallbackCLIPath := os.Getenv("FALLBACK_CLI_FILE")
	if fallbackCLIPath == "" {
		fallbackCLIPath = fmt.Sprintf("backfill-fallback-commands-cla-%s-signatures-%s.sh", stage, time.Now().UTC().Format("20060102T150405Z"))
	}
	fallbackCLIPath = filepath.Clean(fallbackCLIPath)
	if filepath.Dir(fallbackCLIPath) == "." {
		// ok – staying in current working dir
	} else {
		// force local file—avoid writing outside CWD to keep things safe
		fallbackCLIPath = filepath.Base(fallbackCLIPath)
	}

	fmt.Printf("Signature backfill | stage=%s dry-run=%t allow-current-time(after SF)=%t DEBUG=%t\n", stage, dryRun, allowCurrentTime, debug)
	fmt.Printf("Snowflake: table=%s via %s (batch=%d)\n", sfTable, sfCmd, sfBatchSize)

	awsSession, err := session.NewSession(&aws.Config{Region: aws.String("us-east-1")})
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
		// #nosec G304 -- path is sanitized above (filepath.Clean + Base) and generated by this program.
		f, e := os.Create(fallbackCLIPath)
		if e != nil {
			log.Printf("WARN: cannot create %s: %v", fallbackCLIPath, e)
			return
		}
		cliFile = f
		if _, e := fmt.Fprintf(cliFile, "#!/usr/bin/env bash\nset -euo pipefail\n# generated %s UTC, stage=%s, table=%s\n\n", time.Now().UTC().Format(time.RFC3339), stage, tableName); e != nil {
			log.Printf("WARN: failed to write header to %s: %v", fallbackCLIPath, e)
		}
		cliOpen = true
	}
	defer func() {
		if cliFile != nil {
			if err2 := cliFile.Close(); err2 != nil {
				log.Printf("WARN: closing %s: %v", fallbackCLIPath, err2)
			}
		}
	}()

	// 1) First pass: DDB only, created=earliest, modified=latest; no Snowflake, no now()
	stats := newStats()
	updated, cliCount, pending, err := firstPassScanAndUpdate(context.Background(), ddb, tableName, stage, region, dryRun, &stats, func(cmd string) {
		openCLI()
		if cliFile != nil {
			if _, e := fmt.Fprintln(cliFile, cmd); e != nil {
				log.Printf("WARN: writing CLI line failed: %v", e)
			}
		}
	})
	if err != nil {
		log.Fatalf("First pass failed: %v", err)
	}

	// 2) Snowflake pass for “no candidate” rows (fills from _FIVETRAN_SYNCED)
	sfFixed, sfCliCount, sfErr := snowflakeFix(context.Background(), ddb, tableName, stage, region, dryRun, &stats, pending, sfCmd, sfTable, sfBatchSize, func(cmd string) {
		openCLI()
		if cliFile != nil {
			if _, e := fmt.Fprintln(cliFile, cmd); e != nil {
				log.Printf("WARN: writing CLI line failed: %v", e)
			}
		}
	})
	if sfErr != nil {
		log.Printf("WARN: Snowflake step encountered errors: %v", sfErr)
	}
	cliCount += sfCliCount
	updated += sfFixed

	// 3) Final now() pass (only if allowed)
	nowFixed, nowCliCount, nowErr := finalNowFix(context.Background(), ddb, tableName, stage, region, dryRun, &stats, pending, allowCurrentTime, func(cmd string) {
		openCLI()
		if cliFile != nil {
			if _, e := fmt.Fprintln(cliFile, cmd); e != nil {
				log.Printf("WARN: writing CLI line failed: %v", e)
			}
		}
	})
	if nowErr != nil {
		log.Printf("WARN: now()-fill step encountered errors: %v", nowErr)
	}
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
// First pass (DDB only, created=earliest, modified=latest)
// -----------------------------------------------------------------------------

type pendingInfo struct {
	Record      SignatureRecord
	MissingC    bool
	MissingM    bool
	NoCandidate bool
}

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
		expression.AttributeNotExists(expression.Name(attrDateCreatedName)),
		expression.Equal(expression.Name(attrDateCreatedName), expression.Value("")),
		expression.AttributeType(expression.Name(attrDateCreatedName), "NULL"),
	)
	missingModified := expression.Or(
		expression.AttributeNotExists(expression.Name(attrDateModifiedName)),
		expression.Equal(expression.Name(attrDateModifiedName), expression.Value("")),
		expression.AttributeType(expression.Name(attrDateModifiedName), "NULL"),
	)
	missingAny := expression.Or(missingCreated, missingModified)
	approvedAndSigned := expression.And(
		expression.Equal(expression.Name("signature_approved"), expression.Value(true)),
		expression.Equal(expression.Name("signature_signed"), expression.Value(true)),
	)
	filter := expression.And(missingAny, approvedAndSigned)

	proj := expression.NamesList(
		expression.Name("signature_id"),
		expression.Name(attrDateCreatedName),
		expression.Name(attrDateModifiedName),
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

	condExpr := condMissingEither

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

			// Collect *physical* candidates (from DDB attrs, XML, sign URL) — NO Snowflake here
			cands := collectPhysicalCandidates(sig)
			if debug {
				for i, c := range cands {
					dbg("  candidate[%d]: src=%s ts=%s", i, c.src, c.ts)
				}
			}

			// Choose for created: earliest from candidates
			earliest := pickEarliest(cands)

			// Choose for modified: latest from candidates
			latest := pickLatest(cands)

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
					// Fallback: created from modified
					newC, srcC = normalize(sig.DateModified), labelFromModified
				}
			}

			// Decide modified
			var newM, srcM string
			if mM {
				if !isMissing(latest.ts) {
					// Ensure modified >= created if created exists
					if !isMissing(sig.DateCreated) && after(sig.DateCreated, latest.ts) {
						newM, srcM = normalize(sig.DateCreated), labelFromCreated
					} else if mC && newC != "" && after(newC, latest.ts) {
						// created will be set now; keep monotonic
						newM, srcM = newC, labelFromCreated
					} else {
						newM, srcM = normalize(latest.ts), latest.src
					}
				} else {
					// No physical candidate for modified in first pass -> leave for Snowflake
					dbg("  no physical candidate for modified; defer to Snowflake/_FIVETRAN_SYNCED")
				}
			}

			// If nothing to set, send to pending (Snowflake/now pass)
			if (mC && newC == "") && (mM && newM == "") {
				dbg("  -> no in-row choice; marking pending for SF/now")
				pending[sig.SignatureID] = &pendingInfo{Record: sig, MissingC: mC, MissingM: mM, NoCandidate: true}
				continue
			}

			// Monotonic clamp once more (created ≤ modified)
			finalC := ifEmpty(sig.DateCreated, newC)
			finalM := ifEmpty(sig.DateModified, newM)
			tc := parseTime(finalC)
			tm := parseTime(finalM)
			if !tc.IsZero() && !tm.IsZero() && tm.Before(tc) {
				dbg("  clamp modified: modified(%s) < created(%s) -> set modified=created", finalM, finalC)
				finalM = finalC
				srcM = labelFromCreated
			}

			// Build update
			updateExpr := updateKWSet
			vals := map[string]*dynamodb.AttributeValue{exprValEmptyAlias: {S: aws.String("")}}
			names := map[string]*string{
				exprNameDateCreatedAlias: aws.String(attrDateCreatedName),
				exprNameDateModAlias:     aws.String(attrDateModifiedName),
			}
			first := true
			if mC && finalC != "" {
				if !first {
					updateExpr += sepComma
				}
				updateExpr += assignDateCreated
				vals[exprValDateCreatedAlias] = &dynamodb.AttributeValue{S: aws.String(finalC)}
				first = false
			}
			if mM && finalM != "" {
				if !first {
					updateExpr += sepComma
				}
				updateExpr += assignDateModified
				vals[exprValDateModAlias] = &dynamodb.AttributeValue{S: aws.String(finalM)}
			}

			if debug {
				dbg("  updateExpr=%s", updateExpr)
				if v, ok := vals[exprValDateCreatedAlias]; ok && v.S != nil {
					dbg("  :date_created=%s (src=%s)", *v.S, srcC)
				}
				if v, ok := vals[exprValDateModAlias]; ok && v.S != nil {
					dbg("  :date_modified=%s (src=%s)", *v.S, srcM)
				}
			}

			// Stats
			if mC && finalC != "" {
				stats.Created.Inc(srcC)
				stats.Created.Inc(labelTotal)
			}
			if mM && finalM != "" {
				stats.Modified.Inc(srcM)
				stats.Modified.Inc(labelTotal)
			}

			// Emit CLI (always in dry-run; on failure in real-run)
			cmd := buildAwsCliUpdate(region, stage, tableName, sig.SignatureID, updateExpr, names, vals, condExpr)
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
				ConditionExpression:       aws.String(condExpr),
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

//nolint:gocyclo
func snowflakeFix(
	ctx context.Context,
	ddb *dynamodb.DynamoDB,
	tableName, stage, region string,
	dryRun bool,
	stats *UpdateStats,
	pending map[string]*pendingInfo,
	sfCmd, sfTable string,
	batchSize int,
	emitCLI func(string),
) (fixed int, cliCount int, err error) {

	var ids []string
	for id, p := range pending {
		if p.NoCandidate && (p.MissingC || p.MissingM) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return 0, 0, nil
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
			// keep going, but remember the error so the caller can see that some batches failed
			if err == nil {
				err = fmt.Errorf("snowflake batch error(s): %w", e)
			} else {
				err = fmt.Errorf("%v; %w", err, e)
			}
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

			// CREATED: use _fivetran_synced (earliest/only candidate at this stage), clamp to modified if needed
			if mC {
				newC, srcC = normalize(ts), labelFivetranSynced
				if !isMissing(modified) && after(newC, modified) {
					dbg("  SF clamp created: fivetran(%s) > modified(%s) -> modified", newC, modified)
					newC, srcC = normalize(modified), labelFromModified
				}
			}

			// MODIFIED: use _fivetran_synced if no physical candidates existed (that's why we're here)
			if mM {
				if !isMissing(created) {
					newM, srcM = normalize(created), labelFromCreated
				} else if mC && newC != "" {
					newM, srcM = newC, labelFromCreated
				} else {
					newM, srcM = normalize(ts), labelFivetranSynced
				}
			}

			finalC := ifEmpty(created, newC)
			finalM := ifEmpty(modified, newM)
			tc := parseTime(finalC)
			tm := parseTime(finalM)
			if !tc.IsZero() && !tm.IsZero() && tm.Before(tc) {
				dbg("  SF clamp modified: modified(%s) < created(%s) -> created", finalM, finalC)
				finalM = finalC
				srcM = labelFromCreated
			}

			updateExpr := updateKWSet
			vals := map[string]*dynamodb.AttributeValue{exprValEmptyAlias: {S: aws.String("")}}
			names := map[string]*string{
				exprNameDateCreatedAlias: aws.String(attrDateCreatedName),
				exprNameDateModAlias:     aws.String(attrDateModifiedName),
			}
			first := true
			if mC && finalC != "" {
				if !first {
					updateExpr += sepComma
				}
				updateExpr += assignDateCreated
				vals[exprValDateCreatedAlias] = &dynamodb.AttributeValue{S: aws.String(finalC)}
				first = false
			}
			if mM && finalM != "" {
				if !first {
					updateExpr += sepComma
				}
				updateExpr += assignDateModified
				vals[exprValDateModAlias] = &dynamodb.AttributeValue{S: aws.String(finalM)}
			}
			if (mC && finalC == "") && (mM && finalM == "") {
				continue
			}

			if mC && finalC != "" {
				stats.Created.Inc(srcC)
				stats.Created.Inc(labelTotal)
			}
			if mM && finalM != "" {
				stats.Modified.Inc(srcM)
				stats.Modified.Inc(labelTotal)
			}

			cmd := buildAwsCliUpdate(region, stage, tableName, id, updateExpr, names, vals, condMissingEither)
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

			_, uerr := ddb.UpdateItemWithContext(ctx, &dynamodb.UpdateItemInput{
				TableName:                 aws.String(tableName),
				Key:                       map[string]*dynamodb.AttributeValue{"signature_id": {S: aws.String(id)}},
				UpdateExpression:          aws.String(updateExpr),
				ExpressionAttributeNames:  names,
				ExpressionAttributeValues: vals,
				ConditionExpression:       aws.String(condMissingEither),
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
	return fixed, cliCount, err
}

// -----------------------------------------------------------------------------
// Final now()-fill pass (only if allowed)
// -----------------------------------------------------------------------------

func finalNowFix(
	ctx context.Context,
	ddb *dynamodb.DynamoDB,
	tableName, stage, region string,
	dryRun bool,
	stats *UpdateStats,
	pending map[string]*pendingInfo,
	allowNow bool,
	emitCLI func(string),
) (fixed int, cliCount int, err error) {
	if !allowNow || len(pending) == 0 {
		return 0, 0, nil
	}
	nowTS := time.Now().UTC().Format(time.RFC3339)

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
				newC, srcC = nowTS, labelNow
			}
			stats.Created.Inc(srcC)
			stats.Created.Inc(labelTotal)
		}

		var newM, srcM string
		if mM {
			if !isMissing(info.Record.DateCreated) {
				newM, srcM = normalize(info.Record.DateCreated), labelFromCreated
			} else if newC != "" {
				newM, srcM = newC, labelFromCreated
			} else {
				newM, srcM = nowTS, labelNow
			}
			stats.Modified.Inc(srcM)
			stats.Modified.Inc(labelTotal)
		}

		finalC := ifEmpty(info.Record.DateCreated, newC)
		finalM := ifEmpty(info.Record.DateModified, newM)
		tc := parseTime(finalC)
		tm := parseTime(finalM)
		if !tc.IsZero() && !tm.IsZero() && tm.Before(tc) {
			finalM = finalC
		}

		updateExpr := updateKWSet
		vals := map[string]*dynamodb.AttributeValue{exprValEmptyAlias: {S: aws.String("")}}
		names := map[string]*string{
			exprNameDateCreatedAlias: aws.String(attrDateCreatedName),
			exprNameDateModAlias:     aws.String(attrDateModifiedName),
		}
		first := true
		if mC && finalC != "" {
			if !first {
				updateExpr += sepComma
			}
			updateExpr += assignDateCreated
			vals[exprValDateCreatedAlias] = &dynamodb.AttributeValue{S: aws.String(finalC)}
			first = false
		}
		if mM && finalM != "" {
			if !first {
				updateExpr += sepComma
			}
			updateExpr += assignDateModified
			vals[exprValDateModAlias] = &dynamodb.AttributeValue{S: aws.String(finalM)}
		}

		cmd := buildAwsCliUpdate(region, stage, tableName, id, updateExpr, names, vals, condMissingEither)
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

		_, uerr := ddb.UpdateItemWithContext(ctx, &dynamodb.UpdateItemInput{
			TableName:                 aws.String(tableName),
			Key:                       map[string]*dynamodb.AttributeValue{"signature_id": {S: aws.String(id)}},
			UpdateExpression:          aws.String(updateExpr),
			ExpressionAttributeNames:  names,
			ExpressionAttributeValues: vals,
			ConditionExpression:       aws.String(condMissingEither),
		})
		if uerr != nil {
			// record the last error seen so the caller can see something went wrong,
			// but keep going to handle remaining rows
			err = uerr
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
	return fixed, cliCount, err
}

// -----------------------------------------------------------------------------
// Candidate collection & selection
// -----------------------------------------------------------------------------

type pair struct {
	ts  string
	src string
}

// Collect physical candidates in a fixed priority order (for stability if parsing fails):
// signed_on, user_docusign_date_signed, xml_signed, xml_completed, xml_datesigned,
// xml_created, xml_sent, xml_delivered, xml_timegenerated, xml_acstatusdate,
// signurl_createdat, signurl_issuedat
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
		push(sig.SignedOn, physicalSourceSignedOn)
	}

	// 2) user_docusign_date_signed
	if sig.UserDocusignDateSigned != "" {
		push(sig.UserDocusignDateSigned, physicalSourceUDS)
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
	// No parseable times -> fall back to first by priority order
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
	// No parseable times -> fall back to first by priority order
	if len(cands) > 0 {
		return cands[0]
	}
	return pair{}
}

// -----------------------------------------------------------------------------
// Parsing helpers
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
	// Strict internal order (used only if all are parse-failed and we must pick one)
	grab(reSigned, physicalXMLSigned)
	grab(reCompleted, physicalXMLCompleted)
	if ds := extractDateSignedFromXML(raw); ds != "" {
		out = append(out, pair{normalizeDateSigned(ds), physicalXMLDateSigned})
	}
	grab(reCreated, physicalXMLCreated)
	grab(reSent, physicalXMLSent)
	grab(reDelivered, physicalXMLDelivered)
	grab(reTimeGenerated, physicalXMLTimeGenerated)
	grab(reACStatusDate, physicalXMLACStatusDate)
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

// DateSigned samples like "11/17/2025 | 5:30 AM PST"
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
		if derr == nil {
			if ts := extractJSONField(dec, "CreatedAt"); ts != "" {
				return normalize(ts), labelSignURLCreatedAt
			}
			if ts := extractJSONField(dec, "IssuedAt"); ts != "" {
				return normalize(ts), labelSignURLIssuedAt
			}
		}
		return "", ""
	}
	parts := strings.Split(slt, ".")
	for _, seg := range parts {
		if ts := extractBase64JSON(seg, "CreatedAt"); ts != "" {
			return normalize(ts), labelSignURLCreatedAt
		}
		if ts := extractBase64JSON(seg, "IssuedAt"); ts != "" {
			return normalize(ts), labelSignURLIssuedAt
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
	up := strings.ToUpper(stage)
	switch up {
	case "PROD", "PRODUCTION":
		return "FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_SIGNATURES"
	default:
		// DEV
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
	valsFlat := map[string]map[string]string{exprValEmptyAlias: {"S": ""}}
	if av, ok := values[exprValDateCreatedAlias]; ok && av != nil && av.S != nil {
		valsFlat[exprValDateCreatedAlias] = map[string]string{"S": *av.S}
	}
	if av, ok := values[exprValDateModAlias]; ok && av != nil && av.S != nil {
		valsFlat[exprValDateModAlias] = map[string]string{"S": *av.S}
	}

	kb, ke := json.Marshal(key)
	if ke != nil {
		log.Printf("WARN: marshal key: %v", ke)
		kb = []byte("{}")
	}
	nb, ne := json.Marshal(namesFlat)
	if ne != nil {
		log.Printf("WARN: marshal names: %v", ne)
		nb = []byte("{}")
	}
	vb, ve := json.Marshal(valsFlat)
	if ve != nil {
		log.Printf("WARN: marshal values: %v", ve)
		vb = []byte("{}")
	}

	return fmt.Sprintf(
		"aws --profile lfproduct-%s --region %s dynamodb update-item --table-name %s --key '%s' --update-expression '%s' --expression-attribute-names '%s' --expression-attribute-values '%s' --condition-expression '%s'",
		stage, region, table, kb, updateExpr, nb, vb, condExpr,
	)
}

func printStats(stats UpdateStats) {
	fmt.Println("\nUpdate statistics:")
	print := func(title string, c Counter) {
		total := c[labelTotal]
		fmt.Printf("  %s updated %d time(s)\n", title, total)
		keys := make([]string, 0, len(c))
		for k := range c {
			if k == labelTotal {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("    - %-22s %d\n", k, c[k])
		}
	}
	print(attrDateCreatedName, stats.Created)
	print(attrDateModifiedName, stats.Modified)
}
