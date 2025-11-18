// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
)

// SignatureRecord represents the minimal signature record for backfill operations
type SignatureRecord struct {
	SignatureID            string `dynamodbav:"signature_id"`
	DateCreated            string `dynamodbav:"date_created"`
	DateModified           string `dynamodbav:"date_modified"`
	SignedOn               string `dynamodbav:"signed_on"`
	UserDocusignDateSigned string `dynamodbav:"user_docusign_date_signed"`
	UserDocusignRawXML     string `dynamodbav:"user_docusign_raw_xml"`
	SignatureApproved      bool   `dynamodbav:"signature_approved"`
	SignatureSigned        bool   `dynamodbav:"signature_signed"`
}

func main() {
	stage := os.Getenv("STAGE")
	if stage == "" {
		stage = "dev"
	}

	dryRun := os.Getenv("DRY_RUN") == "true"
	allowCurrentTime := os.Getenv("ALLOW_CURRENT_TIME") == "true"

	fmt.Printf("Starting signature timestamp backfill for stage: %s (dry-run: %t, allow-current-time: %t)\n", stage, dryRun, allowCurrentTime)

	// If this works for your env, keep it. (Alternative: SharedConfigEnable to honor AWS_PROFILE/AWS_REGION)
	awsSession, err := session.NewSession(&aws.Config{Region: aws.String("us-east-1")})
	// awsSession, err := session.NewSessionWithOptions(session.Options{SharedConfigState: session.SharedConfigEnable})
	if err != nil {
		log.Fatalf("Failed to create AWS session: %v", err)
	}

	dynamoClient := dynamodb.New(awsSession)
	tableName := fmt.Sprintf("cla-%s-signatures", stage)

	ctx := context.Background()
	updated, skipped, altPath, err := backfillSignatureTimestamps(ctx, dynamoClient, tableName, dryRun, allowCurrentTime)
	if err != nil {
		log.Fatalf("Failed to backfill timestamps: %v", err)
	}

	if altPath != "" {
		fmt.Printf("Completed backfill. Updated: %d signatures, Skipped: %d signatures. Alternative AWS CLI commands written to: %s\n", updated, skipped, altPath)
	} else {
		fmt.Printf("Completed backfill. Updated: %d signatures, Skipped: %d signatures (no alternative commands generated).\n", updated, skipped)
	}
}

func backfillSignatureTimestamps(ctx context.Context, dynamoClient *dynamodb.DynamoDB, tableName string, dryRun bool, allowCurrentTime bool) (int, int, string, error) {
	// Process only approved & signed AND with any missing/empty/NULL date fields
	missingDates := expression.Or(
		expression.AttributeNotExists(expression.Name("date_created")),
		expression.Equal(expression.Name("date_created"), expression.Value("")),
		expression.AttributeType(expression.Name("date_created"), "NULL"),
		expression.AttributeNotExists(expression.Name("date_modified")),
		expression.Equal(expression.Name("date_modified"), expression.Value("")),
		expression.AttributeType(expression.Name("date_modified"), "NULL"),
	)
	approvedAndSigned := expression.And(
		expression.Equal(expression.Name("signature_approved"), expression.Value(true)),
		expression.Equal(expression.Name("signature_signed"), expression.Value(true)),
	)
	filter := expression.And(approvedAndSigned, missingDates)

	projection := expression.NamesList(
		expression.Name("signature_id"),
		expression.Name("date_created"),
		expression.Name("date_modified"),
		expression.Name("signed_on"),
		expression.Name("user_docusign_date_signed"),
		expression.Name("user_docusign_raw_xml"),
		expression.Name("signature_approved"),
		expression.Name("signature_signed"),
	)

	expr, err := expression.NewBuilder().
		WithFilter(filter).
		WithProjection(projection).
		Build()
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to build expression: %v", err)
	}

	// Race-safe condition: only write if at least one target is still empty/missing
	condExpr := "attribute_not_exists(#date_created) OR #date_created = :empty OR " +
		"attribute_not_exists(#date_modified) OR #date_modified = :empty"

	scanInput := &dynamodb.ScanInput{
		TableName:                 aws.String(tableName),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}

	var updated int
	var skipped int
	var pageErr error

	// Alternative CLI output (lazy open on first failure)
	var altCmdPath string
	var altFile *os.File
	var altWriter *bufio.Writer
	lazyOpenAlt := func() error {
		if altWriter != nil {
			return nil
		}
		altCmdPath = fmt.Sprintf("backfill-fallback-commands-%s-%s.sh", tableName, time.Now().UTC().Format("20060102T150405Z"))
		f, e := os.Create(altCmdPath)
		if e != nil {
			return e
		}
		altFile = f
		altWriter = bufio.NewWriter(f)
		fmt.Fprintln(altWriter, "#!/usr/bin/env bash")
		fmt.Fprintln(altWriter, "set -euo pipefail")
		return nil
	}
	defer func() {
		if altWriter != nil {
			altWriter.Flush()
		}
		if altFile != nil {
			_ = altFile.Close()
		}
	}()

	region := aws.StringValue(dynamoClient.Config.Region)

	err = dynamoClient.ScanPagesWithContext(ctx, scanInput, func(page *dynamodb.ScanOutput, lastPage bool) bool {
		var signatures []SignatureRecord
		if uerr := dynamodbattribute.UnmarshalListOfMaps(page.Items, &signatures); uerr != nil {
			pageErr = fmt.Errorf("unmarshal page: %w", uerr)
			return false
		}

		for _, sig := range signatures {
			// Compute a candidate timestamp from best available sources
			candidate := bestTimestampFor(sig)

			// Decide what we actually need to set
			toSetCreated := shouldSet(sig.DateCreated)
			toSetModified := shouldSet(sig.DateModified)

			// If neither needs changes, skip
			if !toSetCreated && !toSetModified {
				continue
			}

			// Determine values for each field independently, keeping created <= modified when possible
			var newCreated, newModified string

			if toSetCreated {
				switch {
				case candidate != "":
					newCreated = normalizeToRFC3339(candidate)
				case sig.DateModified != "":
					newCreated = normalizeToRFC3339(sig.DateModified)
				case allowCurrentTime:
					newCreated = getCurrentTime()
				default:
					fmt.Printf("Skipping signature %s: cannot derive date_created (no candidate/mod/now disallowed)\n", sig.SignatureID)
					skipped++
					continue
				}
			}

			if toSetModified {
				switch {
				case candidate != "":
					newModified = normalizeToRFC3339(candidate)
				case sig.DateCreated != "":
					newModified = normalizeToRFC3339(sig.DateCreated)
				case allowCurrentTime:
					newModified = getCurrentTime()
				default:
					fmt.Printf("Skipping signature %s: cannot derive date_modified (no candidate/created/now disallowed)\n", sig.SignatureID)
					skipped++
					continue
				}
			}

			// Build the update expression
			updateExpr := "SET "
			exprAttrVals := map[string]*dynamodb.AttributeValue{
				":empty": {S: aws.String("")},
			}
			names := map[string]*string{
				"#date_created":  aws.String("date_created"),
				"#date_modified": aws.String("date_modified"),
			}
			first := true
			if toSetCreated {
				if !first {
					updateExpr += ", "
				}
				updateExpr += "#date_created = :date_created"
				exprAttrVals[":date_created"] = &dynamodb.AttributeValue{S: aws.String(newCreated)}
				first = false
			}
			if toSetModified {
				if !first {
					updateExpr += ", "
				}
				updateExpr += "#date_modified = :date_modified"
				exprAttrVals[":date_modified"] = &dynamodb.AttributeValue{S: aws.String(newModified)}
			}

			// Log the source we used
			fmt.Printf("Updating signature %s (created: %t, modified: %t) candidate-source=%s\n",
				sig.SignatureID, toSetCreated, toSetModified, candidateSourceLabel(sig))

			// Perform update or emit alternative command
			if !dryRun {
				updateInput := &dynamodb.UpdateItemInput{
					TableName:                 aws.String(tableName),
					Key:                       map[string]*dynamodb.AttributeValue{"signature_id": {S: aws.String(sig.SignatureID)}},
					UpdateExpression:          aws.String(updateExpr),
					ExpressionAttributeNames:  names,
					ExpressionAttributeValues: exprAttrVals,
					ConditionExpression:       aws.String(condExpr),
				}
				if _, updateErr := dynamoClient.UpdateItemWithContext(ctx, updateInput); updateErr != nil {
					log.Printf("Failed to update signature %s: %v", sig.SignatureID, updateErr)
					// Emit an alternative AWS CLI command
					if err := lazyOpenAlt(); err == nil {
						cli := buildAwsCliUpdateCmd(region, tableName, sig.SignatureID, updateExpr, names, exprAttrVals, condExpr)
						fmt.Fprintln(altWriter, cli)
					}
					continue
				}
			} else {
				// In dry-run mode, also emit the AWS CLI command so an operator can run it
				if err := lazyOpenAlt(); err == nil {
					cli := buildAwsCliUpdateCmd(region, tableName, sig.SignatureID, updateExpr, names, exprAttrVals, condExpr)
					fmt.Fprintln(altWriter, cli)
				}
			}

			updated++
		}
		return true // Continue to next page
	})

	if err != nil {
		return updated, skipped, altCmdPath, fmt.Errorf("scan failed: %v", err)
	}
	if pageErr != nil {
		return updated, skipped, altCmdPath, pageErr
	}

	return updated, skipped, altCmdPath, nil
}

func shouldSet(v string) bool {
	// Empty string covers missing/NULL after unmarshalling; explicit NULL is filtered in the scan.
	return strings.TrimSpace(v) == ""
}

// bestTimestampFor chooses the best historical timestamp we can find:
// 1) signed_on
// 2) user_docusign_date_signed
// 3) from DocuSign raw XML: <Signed>, else <Completed>, else <Created>
// Returns raw string; caller normalizes to RFC3339.
func bestTimestampFor(sig SignatureRecord) string {
	if sig.SignedOn != "" {
		return sig.SignedOn
	}
	if sig.UserDocusignDateSigned != "" {
		return sig.UserDocusignDateSigned
	}
	if ts := extractFromDocuSignXML(sig.UserDocusignRawXML); ts != "" {
		return ts
	}
	return ""
}

func candidateSourceLabel(sig SignatureRecord) string {
	if sig.SignedOn != "" {
		return "signed_on"
	}
	if sig.UserDocusignDateSigned != "" {
		return "user_docusign_date_signed"
	}
	if strings.Contains(sig.UserDocusignRawXML, "<Signed>") || strings.Contains(sig.UserDocusignRawXML, "<Completed>") || strings.Contains(sig.UserDocusignRawXML, "<Created>") {
		return "user_docusign_raw_xml"
	}
	return "none"
}

func extractFromDocuSignXML(raw string) string {
	if raw == "" {
		return ""
	}
	// Try in order of most authoritative for signing:
	for _, tag := range []string{"Signed", "Completed", "Created"} {
		if v := extractXMLTag(raw, tag); v != "" {
			return v
		}
	}
	return ""
}

func extractXMLTag(raw, tag string) string {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	i := strings.Index(raw, open)
	if i < 0 {
		return ""
	}
	i += len(open)
	j := strings.Index(raw[i:], close)
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(raw[i : i+j])
}

func normalizeToRFC3339(s string) string {
	if s == "" {
		return s
	}
	// Try RFC3339 with/without nanos
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	// Try fractional seconds without timezone, e.g. "2025-11-17T05:30:47.48"
	if t, err := time.Parse("2006-01-02T15:04:05.999999999", s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	// Try plain ISO without fractions, no timezone
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	// Try "2025-11-17 20:39:31.097 +0000"
	if t, err := time.Parse("2006-01-02 15:04:05.999 -0700", s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	// As a last resort, return original (DDB stores strings; downstream code should be resilient)
	return s
}

func buildAwsCliUpdateCmd(region, table, signatureID, updateExpr string, names map[string]*string, values map[string]*dynamodb.AttributeValue, condExpr string) string {
	// JSON pieces for CLI (single-quoted)
	keyJSON := fmt.Sprintf(`'{"signature_id":{"S":"%s"}}'`, escapeForJSON(signatureID))

	// Names JSON: include only names actually referenced in updateExpr
	namesKV := []string{}
	if strings.Contains(updateExpr, "#date_created") {
		namesKV = append(namesKV, `"#date_created":"date_created"`)
	}
	if strings.Contains(updateExpr, "#date_modified") {
		namesKV = append(namesKV, `"#date_modified":"date_modified"`)
	}
	namesJSON := fmt.Sprintf(`'{%s}'`, strings.Join(namesKV, ","))

	// Values JSON
	valsKV := []string{`":empty":{"S":""}`}
	if av, ok := values[":date_created"]; ok && av.S != nil {
		valsKV = append(valsKV, fmt.Sprintf(`":date_created":{"S":"%s"}`, escapeForJSON(*av.S)))
	}
	if av, ok := values[":date_modified"]; ok && av.S != nil {
		valsKV = append(valsKV, fmt.Sprintf(`":date_modified":{"S":"%s"}`, escapeForJSON(*av.S)))
	}
	valuesJSON := fmt.Sprintf(`'{%s}'`, strings.Join(valsKV, ","))

	// Build CLI
	return fmt.Sprintf(
		`aws dynamodb update-item --table-name %s --key %s --update-expression '%s' --expression-attribute-names %s --expression-attribute-values %s --condition-expression '%s' --region %s`,
		table, keyJSON, updateExpr, namesJSON, valuesJSON, condExpr, region,
	)
}

func escapeForJSON(s string) string {
	// Minimal escaping for quotes/backslashes in CLI JSON blobs
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func getCurrentTime() string {
	return time.Now().UTC().Format(time.RFC3339)
}
