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

// -----------------------------
// Models & stats
// -----------------------------

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

// -----------------------------
// Main
// -----------------------------

func main() {
	stage := strings.TrimSpace(os.Getenv("STAGE"))
	if stage == "" {
		stage = "dev"
	}
	dryRun := os.Getenv("DRY_RUN") == "true"
	allowCurrentTime := os.Getenv("ALLOW_CURRENT_TIME") == "true"

	// Snowflake integration command (reads SQL from stdin and prints CSV to stdout)
	sfCmd := strings.TrimSpace(os.Getenv("SNOWFLAKE_CSV_CMD"))
	if sfCmd == "" {
		sfCmd = "sf_db_csv.sh"
	}
	// Default Snowflake table (override with SNOWFLAKE_TABLE if needed)
	sfTable := strings.TrimSpace(os.Getenv("SNOWFLAKE_TABLE"))
	if sfTable == "" {
		sfTable = fmt.Sprintf("FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_%s_SIGNATURES", stageToSnowflake(stage))
	}
	// Batch size for IN clause
	sfBatchSize := 500

	// CLI fallback output file
	fallbackCLIPath := os.Getenv("FALLBACK_CLI_FILE")
	if fallbackCLIPath == "" {
		fallbackCLIPath = fmt.Sprintf("backfill-fallback-commands-cla-%s-signatures-%s.sh", stage, time.Now().UTC().Format("20060102T150405Z"))
	}

	fmt.Printf("Signature backfill | stage=%s dry-run=%t allow-current-time(after SF)=%t\n", stage, dryRun, allowCurrentTime)
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
		f, e := os.Create(fallbackCLIPath)
		if e != nil {
			log.Printf("WARN: cannot create %s: %v", fallbackCLIPath, e)
			return
		}
		cliFile = f
		fmt.Fprintf(cliFile, "#!/usr/bin/env bash\nset -euo pipefail\n# generated %s UTC, stage=%s, table=%s\n\n", time.Now().UTC().Format(time.RFC3339), stage, tableName)
		cliOpen = true
	}
	defer func() {
		if cliFile != nil {
			_ = cliFile.Close()
		}
	}()

	// 1) First pass: scan DDB, use in-row sources only (NO Snowflake, NO now()).
	stats := newStats()
	updated, skipped, cliCount, pending, err := firstPassScanAndUpdate(context.Background(), ddb, tableName, stage, region, dryRun, &stats, func(cmd string) {
		openCLI()
		if cliFile != nil {
			fmt.Fprintln(cliFile, cmd)
		}
	})
	if err != nil {
		log.Fatalf("First pass failed: %v", err)
	}

	// 2) Snowflake last-resort for any remaining pending (no candidate found)
	sfFixed, sfCliCount, err := snowflakeFix(context.Background(), ddb, tableName, stage, region, dryRun, &stats, pending, sfCmd, sfTable, sfBatchSize, func(cmd string) {
		openCLI()
		if cliFile != nil {
			fmt.Fprintln(cliFile, cmd)
		}
	})
	if err != nil {
		log.Printf("WARN: Snowflake step failed: %v", err)
	}
	cliCount += sfCliCount
	updated += sfFixed

	// 3) Final fill with now() ONLY IF ALLOW_CURRENT_TIME=true
	nowFixed, nowCliCount, err := finalNowFix(context.Background(), ddb, tableName, stage, region, dryRun, &stats, pending, allowCurrentTime, func(cmd string) {
		openCLI()
		if cliFile != nil {
			fmt.Fprintln(cliFile, cmd)
		}
	})
	if err != nil {
		log.Printf("WARN: now()-fill step failed: %v", err)
	}
	cliCount += nowCliCount
	updated += nowFixed
	skipped = len(pending) // anything still pending after SF + now

	fmt.Printf("\nCompleted. Updated: %d  |  Still pending (skipped): %d\n", updated, skipped)
	if cliOpen {
		fmt.Printf("Fallback AWS CLI written to: %s (lines: %d)\n", fallbackCLIPath, cliCount)
	}
	printStats(stats)
}

// -----------------------------
// First pass (DDB only, no SF, no now())
// -----------------------------

type pendingInfo struct {
	Record      SignatureRecord
	MissingC    bool
	MissingM    bool
	NoCandidate bool // true if we couldn't find any external timestamp candidate
}

func firstPassScanAndUpdate(
	ctx context.Context,
	ddb *dynamodb.DynamoDB,
	tableName, stage, region string,
	dryRun bool,
	stats *UpdateStats,
	emitCLI func(string),
) (updated int, skipped int, cliCount int, pending map[string]*pendingInfo, err error) {
	pending = map[string]*pendingInfo{}

	// Only approved+signed and missing/empty/NULL date fields
	missingCreated := expression.Or(
		expression.AttributeNotExists(expression.Name("date_created")),
		expression.Equal(expression.Name("date_created"), expression.Value("")),
		expression.AttributeType(expression.Name("date_created"), "NULL"),
	)
	missingModified := expression.Or(
		expression.AttributeNotExists(expression.Name("date_modified")),
		expression.Equal(expression.Name("date_modified"), expression.Value("")),
		expression.AttributeType(expression.Name("date_modified"), "NULL"),
	)
	missingAny := expression.Or(missingCreated, missingModified)
	approvedAndSigned := expression.And(
		expression.Equal(expression.Name("signature_approved"), expression.Value(true)),
		expression.Equal(expression.Name("signature_signed"), expression.Value(true)),
	)
	filter := expression.And(missingAny, approvedAndSigned)

	proj := expression.NamesList(
		expression.Name("signature_id"),
		expression.Name("date_created"),
		expression.Name("date_modified"),
		expression.Name("signed_on"),
		expression.Name("user_docusign_date_signed"),
		expression.Name("user_docusign_raw_xml"),
		expression.Name("signature_sign_url"),
		expression.Name("signature_approved"),
		expression.Name("signature_signed"),
	)

	expr, e := expression.NewBuilder().WithFilter(filter).WithProjection(proj).Build()
	if e != nil {
		return 0, 0, 0, nil, fmt.Errorf("build expression: %w", e)
	}

	scan := &dynamodb.ScanInput{
		TableName:                 aws.String(tableName),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}

	condExpr := "attribute_not_exists(#date_created) OR #date_created = :empty OR " +
		"attribute_not_exists(#date_modified) OR #date_modified = :empty"

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

			// Build candidates from in-row sources only
			cands := gatherPrimaryCandidates(sig)

			// Decide created (prefer earliest candidate; else copy modified if present)
			var newC, srcC string
			if mC {
				if len(cands) > 0 {
					best := earliest(cands)
					// Keep created ≤ modified if modified exists
					if !isMissing(sig.DateModified) && after(best.ts, sig.DateModified) {
						newC, srcC = normalize(best.ts), "signed_or_xml_or_signurl"
						// clamp happens below after we compute newM
					} else {
						newC, srcC = normalize(best.ts), best.src
					}
				} else if !isMissing(sig.DateModified) {
					newC, srcC = normalize(sig.DateModified), "from_modified"
				}
			}

			// Decide modified (prefer copy from created if we have any; else earliest candidate)
			var newM, srcM string
			if mM {
				if !mC && !isMissing(sig.DateCreated) {
					newM, srcM = normalize(sig.DateCreated), "from_created"
				} else if mC && newC != "" {
					newM, srcM = newC, "from_created"
				} else if len(cands) > 0 {
					best := earliest(cands)
					newM, srcM = normalize(best.ts), best.src
				}
			}

			// If nothing to set, put into pending for Snowflake/now() step
			if (mC && newC == "") && (mM && newM == "") {
				pending[sig.SignatureID] = &pendingInfo{
					Record:      sig,
					MissingC:    mC,
					MissingM:    mM,
					NoCandidate: true,
				}
				continue
			}

			// Monotonic clamp created ≤ modified
			finalC := ifEmpty(sig.DateCreated, newC)
			finalM := ifEmpty(sig.DateModified, newM)
			tc := parseTime(finalC)
			tm := parseTime(finalM)
			if !tc.IsZero() && !tm.IsZero() && tm.Before(tc) {
				finalM = finalC
				srcM = "from_created"
			}

			// Build update
			updateExpr := "SET "
			vals := map[string]*dynamodb.AttributeValue{":empty": {S: aws.String("")}}
			names := map[string]*string{
				"#date_created":  aws.String("date_created"),
				"#date_modified": aws.String("date_modified"),
			}
			first := true
			if mC && finalC != "" {
				if !first {
					updateExpr += ", "
				}
				updateExpr += "#date_created = :date_created"
				vals[":date_created"] = &dynamodb.AttributeValue{S: aws.String(finalC)}
				first = false
			}
			if mM && finalM != "" {
				if !first {
					updateExpr += ", "
				}
				updateExpr += "#date_modified = :date_modified"
				vals[":date_modified"] = &dynamodb.AttributeValue{S: aws.String(finalM)}
			}

			if (mC && finalC == "") && (mM && finalM == "") {
				// nothing to do
				pending[sig.SignatureID] = &pendingInfo{Record: sig, MissingC: mC, MissingM: mM, NoCandidate: true}
				continue
			}

			// Stats
			if mC && finalC != "" {
				stats.Created.Inc(srcCOr(bestSrc(srcC, cands), srcC))
				stats.Created.Inc("_total")
			}
			if mM && finalM != "" {
				stats.Modified.Inc(srcM)
				stats.Modified.Inc("_total")
			}

			// Emit CLI in dry-run; real-run only on failures
			cmd := buildAwsCliUpdate(region, stage, tableName, sig.SignatureID, updateExpr, names, vals, condExpr)
			if dryRun {
				if emitCLI != nil {
					emitCLI(cmd)
					cliCount++
				}
				updated++
				continue
			}

			_, uerr := ddb.UpdateItem(&dynamodb.UpdateItemInput{
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
		return updated, skipped, cliCount, nil, fmt.Errorf("scan failed: %w", err)
	}
	if pageErr != nil {
		return updated, skipped, cliCount, nil, pageErr
	}
	return updated, skipped, cliCount, pending, nil
}

func srcCOr(a, b string) string {
	a = strings.TrimSpace(a)
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}

func bestSrc(src string, cands []pair) string {
	if src != "" {
		return src
	}
	if len(cands) == 0 {
		return ""
	}
	return cands[0].src
}

// -----------------------------
// Snowflake pass (for pending IDs with NoCandidate)
// -----------------------------

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

	ids := make([]string, 0, len(pending))
	for id, p := range pending {
		// Consider only those where we truly had no candidate in first pass
		if p.NoCandidate && (p.MissingC || p.MissingM) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return 0, 0, nil
	}

	// Batch in chunks
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]

		// Compose SQL
		inList := "'" + strings.Join(chunk, "','") + "'"
		sql := fmt.Sprintf(`SELECT signature_id, _FIVETRAN_SYNCED FROM %s WHERE signature_id IN (%s)`, sfTable, inList)

		// Run sf_db_csv.sh, feeding SQL via stdin
		out, e := runSnowflakeCSV(sfCmd, sql)
		if e != nil {
			// Don't fail the whole run; log and continue
			log.Printf("WARN: Snowflake batch failed: %v", e)
			continue
		}

		// Parse CSV into map[id]ts
		sfMap := parseSnowflakeCSV(out)

		// Apply updates for all entries returned
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

			if mC {
				newC, srcC = normalize(ts), "fivetran_synced"
				// If modified exists and is before chosen created, clamp created to modified
				if !isMissing(modified) && after(newC, modified) {
					newC, srcC = normalize(modified), "from_modified"
				}
			}
			if mM {
				// Prefer from created to keep >=
				if !isMissing(created) {
					newM, srcM = normalize(created), "from_created"
				} else if mC && newC != "" {
					newM, srcM = newC, "from_created"
				} else {
					newM, srcM = normalize(ts), "fivetran_synced"
				}
			}

			finalC := ifEmpty(created, newC)
			finalM := ifEmpty(modified, newM)
			tc := parseTime(finalC)
			tm := parseTime(finalM)
			if !tc.IsZero() && !tm.IsZero() && tm.Before(tc) {
				finalM = finalC
				srcM = "from_created"
			}

			// Build update
			updateExpr := "SET "
			vals := map[string]*dynamodb.AttributeValue{":empty": {S: aws.String("")}}
			names := map[string]*string{
				"#date_created":  aws.String("date_created"),
				"#date_modified": aws.String("date_modified"),
			}
			first := true
			if mC && finalC != "" {
				if !first {
					updateExpr += ", "
				}
				updateExpr += "#date_created = :date_created"
				vals[":date_created"] = &dynamodb.AttributeValue{S: aws.String(finalC)}
				first = false
			}
			if mM && finalM != "" {
				if !first {
					updateExpr += ", "
				}
				updateExpr += "#date_modified = :date_modified"
				vals[":date_modified"] = &dynamodb.AttributeValue{S: aws.String(finalM)}
			}
			if (mC && finalC == "") && (mM && finalM == "") {
				continue
			}

			// Stats
			if mC && finalC != "" {
				stats.Created.Inc(srcC)
				stats.Created.Inc("_total")
			}
			if mM && finalM != "" {
				stats.Modified.Inc(srcM)
				stats.Modified.Inc("_total")
			}

			cmd := buildAwsCliUpdate(region, stage, tableName, id, updateExpr, names, vals,
				"attribute_not_exists(#date_created) OR #date_created = :empty OR attribute_not_exists(#date_modified) OR #date_modified = :empty")

			if dryRun {
				if emitCLI != nil {
					emitCLI(cmd)
					cliCount++
				}
				fixed++
				// mark as no longer pending
				delete(pending, id)
				continue
			}

			_, uerr := ddb.UpdateItem(&dynamodb.UpdateItemInput{
				TableName:                 aws.String(tableName),
				Key:                       map[string]*dynamodb.AttributeValue{"signature_id": {S: aws.String(id)}},
				UpdateExpression:          aws.String(updateExpr),
				ExpressionAttributeNames:  names,
				ExpressionAttributeValues: vals,
				ConditionExpression:       aws.String("attribute_not_exists(#date_created) OR #date_created = :empty OR attribute_not_exists(#date_modified) OR #date_modified = :empty"),
			})
			if uerr != nil {
				log.Printf("Update failed (SF) %s: %v", id, uerr)
				if emitCLI != nil {
					emitCLI(cmd)
					cliCount++
				}
				// keep pending
				continue
			}
			fixed++
			delete(pending, id)
		}
	}

	return fixed, cliCount, nil
}

// -----------------------------
// Final now()-fill pass (only if allowed)
// -----------------------------

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
	now := time.Now().UTC().Format(time.RFC3339)

	for id, info := range pending {
		mC := info.MissingC
		mM := info.MissingM
		if !mC && !mM {
			delete(pending, id)
			continue
		}

		// For date_created, if modified exists and is earlier than now, prefer modified
		newC := ""
		if mC {
			if !isMissing(info.Record.DateModified) {
				newC = normalize(info.Record.DateModified)
				stats.Created.Inc("from_modified")
			} else {
				newC = now
				stats.Created.Inc("now")
			}
			stats.Created.Inc("_total")
		}

		// For date_modified, prefer created (existing or new)
		newM := ""
		if mM {
			if !isMissing(info.Record.DateCreated) {
				newM = normalize(info.Record.DateCreated)
				stats.Modified.Inc("from_created")
			} else if newC != "" {
				newM = newC
				stats.Modified.Inc("from_created")
			} else {
				newM = now
				stats.Modified.Inc("now")
			}
			stats.Modified.Inc("_total")
		}

		// Monotonic clamp
		finalC := ifEmpty(info.Record.DateCreated, newC)
		finalM := ifEmpty(info.Record.DateModified, newM)
		tc := parseTime(finalC)
		tm := parseTime(finalM)
		if !tc.IsZero() && !tm.IsZero() && tm.Before(tc) {
			finalM = finalC
		}

		// Build update
		updateExpr := "SET "
		vals := map[string]*dynamodb.AttributeValue{":empty": {S: aws.String("")}}
		names := map[string]*string{
			"#date_created":  aws.String("date_created"),
			"#date_modified": aws.String("date_modified"),
		}
		first := true
		if mC && finalC != "" {
			if !first {
				updateExpr += ", "
			}
			updateExpr += "#date_created = :date_created"
			vals[":date_created"] = &dynamodb.AttributeValue{S: aws.String(finalC)}
			first = false
		}
		if mM && finalM != "" {
			if !first {
				updateExpr += ", "
			}
			updateExpr += "#date_modified = :date_modified"
			vals[":date_modified"] = &dynamodb.AttributeValue{S: aws.String(finalM)}
		}

		cmd := buildAwsCliUpdate(region, stage, tableName, id, updateExpr, names, vals,
			"attribute_not_exists(#date_created) OR #date_created = :empty OR attribute_not_exists(#date_modified) OR #date_modified = :empty")

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
			ConditionExpression:       aws.String("attribute_not_exists(#date_created) OR #date_created = :empty OR attribute_not_exists(#date_modified) OR #date_modified = :empty"),
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
	return fixed, cliCount, nil
}

// -----------------------------
// Candidate gathering (in-row only)
// -----------------------------

type pair struct {
	ts  string
	src string
}

func gatherPrimaryCandidates(sig SignatureRecord) []pair {
	var out []pair
	push := func(val, src string) {
		val = strings.TrimSpace(val)
		if val != "" {
			out = append(out, pair{normalize(val), src})
		}
	}
	if sig.SignedOn != "" {
		push(sig.SignedOn, "signed_on")
	}
	if sig.UserDocusignDateSigned != "" {
		push(sig.UserDocusignDateSigned, "docusign_signed_on")
	}
	// DocuSign XML tags
	for _, p := range extractDocuSignXML(sig.UserDocusignRawXML) {
		push(p.ts, p.src)
	}
	// CreatedAt/IssuedAt from DocuSign sign URL
	if ts, lbl := createdOrIssuedAtFromSignURL(sig.SignatureSignURL); ts != "" {
		push(ts, lbl)
	}
	return out
}

func earliest(in []pair) pair {
	var best pair
	var bestT time.Time
	first := true
	for _, p := range in {
		t := parseTime(p.ts)
		if t.IsZero() {
			continue
		}
		if first || t.Before(bestT) {
			best = p
			bestT = t
			first = false
		}
	}
	return best
}

// -----------------------------
// Parsing helpers
// -----------------------------

var (
	reSigned        = regexp.MustCompile(`(?i)<Signed>([^<]+)</Signed>`)
	reCompleted     = regexp.MustCompile(`(?i)<Completed>([^<]+)</Completed>`)
	reCreated       = regexp.MustCompile(`(?i)<Created>([^<]+)</Created>`)
	reSent          = regexp.MustCompile(`(?i)<Sent>([^<]+)</Sent>`)
	reDelivered     = regexp.MustCompile(`(?i)<Delivered>([^<]+)</Delivered>`)
	reTimeGenerated = regexp.MustCompile(`(?i)<TimeGenerated>([^<]+)</TimeGenerated>`)
	reACStatusDate  = regexp.MustCompile(`(?i)<ACStatusDate>([^<]+)</ACStatusDate>`)
)

func extractDocuSignXML(raw string) []pair {
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
	grab(reSigned, "xml_signed")
	grab(reCompleted, "xml_completed")
	grab(reCreated, "xml_created")
	grab(reSent, "xml_sent")
	grab(reDelivered, "xml_delivered")
	grab(reTimeGenerated, "xml_timegenerated")
	grab(reACStatusDate, "xml_acstatusdate")
	return out
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
		// sometimes the whole token is in path; try decode whole URL once
		dec, _ := url.QueryUnescape(raw)
		if ts := extractJSONField(dec, "CreatedAt"); ts != "" {
			return normalize(ts), "signurl_createdat"
		}
		if ts := extractJSONField(dec, "IssuedAt"); ts != "" {
			return normalize(ts), "signurl_issuedat"
		}
		return "", ""
	}
	parts := strings.Split(slt, ".")
	for _, seg := range parts {
		if ts := extractBase64JSON(seg, "CreatedAt"); ts != "" {
			return normalize(ts), "signurl_createdat"
		}
		if ts := extractBase64JSON(seg, "IssuedAt"); ts != "" {
			return normalize(ts), "signurl_issuedat"
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
	// Snowflake/Fivetran: "2006-01-02 15:04:05.999 -0700" variants
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
		return "PROD"
	case "STAGE", "STAGING":
		return "STAGING"
	default:
		return "DEV"
	}
}

// -----------------------------
// Snowflake execution & CSV parse
// -----------------------------

func runSnowflakeCSV(cmdPath, sql string) ([]byte, error) {
	cmd := exec.Command(cmdPath) // expects SQL on stdin
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

// -----------------------------
// AWS CLI builder & stats print
// -----------------------------

func buildAwsCliUpdate(region, stage, table, sigID, updateExpr string, names map[string]*string, values map[string]*dynamodb.AttributeValue, condExpr string) string {
	key := map[string]map[string]string{"signature_id": {"S": sigID}}
	namesFlat := map[string]string{}
	for k, v := range names {
		if v != nil {
			namesFlat[k] = *v
		}
	}
	valsFlat := map[string]map[string]string{":empty": {"S": ""}}
	if av, ok := values[":date_created"]; ok != false && av != nil && av.S != nil {
		valsFlat[":date_created"] = map[string]string{"S": *av.S}
	}
	if av, ok := values[":date_modified"]; ok != false && av != nil && av.S != nil {
		valsFlat[":date_modified"] = map[string]string{"S": *av.S}
	}

	kb, _ := json.Marshal(key)
	nb, _ := json.Marshal(namesFlat)
	vb, _ := json.Marshal(valsFlat)

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
	print("date_created", stats.Created)
	print("date_modified", stats.Modified)
}
