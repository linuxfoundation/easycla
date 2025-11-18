// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
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
	SignatureApproved      bool   `dynamodbav:"signature_approved"`
	SignatureSigned        bool   `dynamodbav:"signature_signed"`
}

// Stats for created/modified per source
type FieldSourceStats struct {
	Total            int
	FromSignedOn     int
	FromDocusignDate int
	FromXmlSigned    int
	FromXmlCompleted int
	FromXmlCreated   int
	FromNow          int
	FromOtherField   int // created: from modified; modified: from created
}

type UpdateStats struct {
	Created  FieldSourceStats
	Modified FieldSourceStats
}

func main() {
	stage := strings.TrimSpace(os.Getenv("STAGE"))
	if stage == "" {
		stage = "dev"
	}

	dryRun := os.Getenv("DRY_RUN") == "true"
	allowCurrentTime := os.Getenv("ALLOW_CURRENT_TIME") == "true"

	fallbackPath := os.Getenv("FALLBACK_CLI_FILE")
	if fallbackPath == "" {
		fallbackPath = fmt.Sprintf("signatures_backfill_fallback_%s_%s.sh", stage, time.Now().UTC().Format("20060102T150405Z"))
	}
	var fallbackFile *os.File
	var ferr error
	// Only open fallback file when we might need to write failed updates (not in dry-run)
	if !dryRun {
		fallbackFile, ferr = os.Create(fallbackPath)
		if ferr != nil {
			log.Printf("WARN: could not create fallback CLI file %s: %v", fallbackPath, ferr)
		} else {
			fmt.Fprintf(fallbackFile, "#!/usr/bin/env bash\nset -euo pipefail\n# Auto-generated at %s UTC for stage=%s\n\n", time.Now().UTC().Format(time.RFC3339), stage)
		}
		defer func() {
			if fallbackFile != nil {
				_ = fallbackFile.Close()
			}
		}()
	}

	fmt.Printf("Starting signature timestamp backfill for stage: %s (dry-run: %t, allow-current-time: %t)\n", stage, dryRun, allowCurrentTime)

	awsSession, err := session.NewSession(&aws.Config{Region: aws.String("us-east-1")})
	// To rely on AWS_PROFILE/AWS_REGION instead, use:
	// awsSession, err := session.NewSessionWithOptions(session.Options{SharedConfigState: session.SharedConfigEnable})
	if err != nil {
		log.Fatalf("Failed to create AWS session: %v", err)
	}

	dynamoClient := dynamodb.New(awsSession)
	tableName := fmt.Sprintf("cla-%s-signatures", stage)
	region := aws.StringValue(awsSession.Config.Region)

	ctx := context.Background()
	updated, skipped, fallbackCount, stats, err := backfillSignatureTimestamps(ctx, dynamoClient, tableName, stage, dryRun, allowCurrentTime, fallbackFile, region)
	if err != nil {
		log.Fatalf("Failed to backfill timestamps: %v", err)
	}

	fmt.Printf("\nCompleted backfill. Updated: %d signatures, Skipped: %d (no usable timestamps).\n", updated, skipped)
	if fallbackCount > 0 && fallbackFile != nil {
		fmt.Printf("Wrote %d fallback AWS CLI command(s) to: %s\n", fallbackCount, fallbackPath)
	} else if fallbackCount > 0 {
		fmt.Printf("Had %d update failure(s), but could not write fallback commands (no file opened).\n", fallbackCount)
	}

	// Final statistics
	fmt.Println("\nUpdate statistics:")
	fmt.Printf("  created_on updated %d times\n", stats.Created.Total)
	fmt.Printf("    - from signed_on:              %d\n", stats.Created.FromSignedOn)
	fmt.Printf("    - from user_docusign_date:     %d\n", stats.Created.FromDocusignDate)
	fmt.Printf("    - from DocuSign XML <Signed>:  %d\n", stats.Created.FromXmlSigned)
	fmt.Printf("    - from DocuSign XML <Completed>:%d\n", stats.Created.FromXmlCompleted)
	fmt.Printf("    - from DocuSign XML <Created>: %d\n", stats.Created.FromXmlCreated)
	fmt.Printf("    - from now():                   %d\n", stats.Created.FromNow)
	fmt.Printf("    - copied from modified:         %d\n", stats.Created.FromOtherField)

	fmt.Printf("  modified_on updated %d times\n", stats.Modified.Total)
	fmt.Printf("    - from signed_on:               %d\n", stats.Modified.FromSignedOn)
	fmt.Printf("    - from user_docusign_date:      %d\n", stats.Modified.FromDocusignDate)
	fmt.Printf("    - from DocuSign XML <Signed>:   %d\n", stats.Modified.FromXmlSigned)
	fmt.Printf("    - from DocuSign XML <Completed>: %d\n", stats.Modified.FromXmlCompleted)
	fmt.Printf("    - from DocuSign XML <Created>:  %d\n", stats.Modified.FromXmlCreated)
	fmt.Printf("    - from now():                    %d\n", stats.Modified.FromNow)
	fmt.Printf("    - copied from created:           %d\n", stats.Modified.FromOtherField)
}

func backfillSignatureTimestamps(
	ctx context.Context,
	dynamoClient *dynamodb.DynamoDB,
	tableName string,
	stage string,
	dryRun bool,
	allowCurrentTime bool,
	fallbackFile *os.File,
	region string,
) (int, int, int, UpdateStats, error) {
	var stats UpdateStats

	// Filter: only approved & signed AND missing/empty/NULL date fields
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

	projection := expression.NamesList(
		expression.Name("signature_id"),
		expression.Name("date_created"),
		expression.Name("date_modified"),
		expression.Name("signed_on"),
		expression.Name("user_docusign_date_signed"),
		expression.Name("signature_approved"),
		expression.Name("signature_signed"),
	)

	expr, err := expression.NewBuilder().
		WithFilter(filter).
		WithProjection(projection).
		Build()
	if err != nil {
		return 0, 0, 0, stats, fmt.Errorf("failed to build expression: %v", err)
	}

	// Race-safe condition
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
	var fallbackCount int
	var pageErr error

	err = dynamoClient.ScanPages(scanInput, func(page *dynamodb.ScanOutput, lastPage bool) bool {
		var signatures []SignatureRecord
		if uerr := dynamodbattribute.UnmarshalListOfMaps(page.Items, &signatures); uerr != nil {
			pageErr = fmt.Errorf("unmarshal page: %w", uerr)
			return false
		}

		for _, sig := range signatures {
			if !sig.SignatureApproved || !sig.SignatureSigned {
				continue
			}

			// 1) Choose candidate + label: signed_on -> user_docusign_date_signed -> DocuSign XML -> now (if allowed)
			bestTimestamp := ""
			candidateLabel := ""
			if sig.SignedOn != "" {
				bestTimestamp = sig.SignedOn
				candidateLabel = "signed_on"
			} else if sig.UserDocusignDateSigned != "" {
				bestTimestamp = sig.UserDocusignDateSigned
				candidateLabel = "user_docusign_date"
			} else {
				if ts, lbl, ok := tryFetchFromDocusignXML(ctx, dynamoClient, tableName, sig.SignatureID); ok {
					bestTimestamp = ts
					candidateLabel = lbl // docusign_xml_signed/completed/created
				} else if allowCurrentTime {
					bestTimestamp = getCurrentTime()
					candidateLabel = "now"
				}
			}

			if bestTimestamp == "" {
				fmt.Printf("Skipping signature %s: no usable timestamp (signed_on: %q, docusign_date: %q, allow_current_time: %t)\n",
					sig.SignatureID, sig.SignedOn, sig.UserDocusignDateSigned, allowCurrentTime)
				skipped++
				continue
			}

			// Existing values
			existingCreated := strings.TrimSpace(sig.DateCreated)
			existingModified := strings.TrimSpace(sig.DateModified)

			// 2) Decide fields to set, preserving monotonicity
			newCreated := existingCreated
			newModified := existingModified

			setCreated := existingCreated == ""
			setModified := existingModified == ""

			var createdSrc, modifiedSrc string

			if setCreated {
				// Prefer candidate, then modified, then now (if allowed)
				if candidateLabel != "" && candidateLabel != "now" {
					newCreated = bestTimestamp
					createdSrc = candidateLabel
				} else if existingModified != "" {
					newCreated = existingModified
					createdSrc = "from_modified"
				} else if candidateLabel == "now" {
					newCreated = bestTimestamp
					createdSrc = "now"
				} else {
					// Shouldn't happen due to earlier guard, but be safe
					skipped++
					continue
				}
			}

			if setModified {
				// Prefer copying from created when available to keep modified >= created
				if newCreated != "" {
					newModified = newCreated
					modifiedSrc = "from_created"
				} else if candidateLabel != "" {
					newModified = bestTimestamp
					modifiedSrc = candidateLabel
				} else if allowCurrentTime {
					newModified = getCurrentTime()
					modifiedSrc = "now"
				} else {
					skipped++
					continue
				}
			}

			if !setCreated && !setModified {
				continue
			}

			// 3) Build update expression
			updateParts := make([]string, 0, 2)
			exprAttrVals := make(map[string]*dynamodb.AttributeValue)

			if setCreated {
				updateParts = append(updateParts, "#date_created = :date_created")
				exprAttrVals[":date_created"] = &dynamodb.AttributeValue{S: aws.String(newCreated)}
			}
			if setModified {
				updateParts = append(updateParts, "#date_modified = :date_modified")
				exprAttrVals[":date_modified"] = &dynamodb.AttributeValue{S: aws.String(newModified)}
			}
			updateExpr := "SET " + strings.Join(updateParts, ", ")

			// 4) Stats
			if setCreated {
				stats.Created.Total++
				switch createdSrc {
				case "signed_on":
					stats.Created.FromSignedOn++
				case "user_docusign_date":
					stats.Created.FromDocusignDate++
				case "docusign_xml_signed":
					stats.Created.FromXmlSigned++
				case "docusign_xml_completed":
					stats.Created.FromXmlCompleted++
				case "docusign_xml_created":
					stats.Created.FromXmlCreated++
				case "now":
					stats.Created.FromNow++
				case "from_modified":
					stats.Created.FromOtherField++
				}
			}
			if setModified {
				stats.Modified.Total++
				switch modifiedSrc {
				case "signed_on":
					stats.Modified.FromSignedOn++
				case "user_docusign_date":
					stats.Modified.FromDocusignDate++
				case "docusign_xml_signed":
					stats.Modified.FromXmlSigned++
				case "docusign_xml_completed":
					stats.Modified.FromXmlCompleted++
				case "docusign_xml_created":
					stats.Modified.FromXmlCreated++
				case "now":
					stats.Modified.FromNow++
				case "from_created":
					stats.Modified.FromOtherField++
				}
			}

			fmt.Printf("Updating signature %s (created:%t src=%s, modified:%t src=%s)\n",
				sig.SignatureID, setCreated, createdSrc, setModified, modifiedSrc)

			// 5) Perform update (or write fallback on failure)
			if !dryRun {
				exprAttrVals[":empty"] = &dynamodb.AttributeValue{S: aws.String("")}
				updateInput := &dynamodb.UpdateItemInput{
					TableName: aws.String(tableName),
					Key: map[string]*dynamodb.AttributeValue{
						"signature_id": {S: aws.String(sig.SignatureID)},
					},
					UpdateExpression: aws.String(updateExpr),
					ExpressionAttributeNames: map[string]*string{
						"#date_created":  aws.String("date_created"),
						"#date_modified": aws.String("date_modified"),
					},
					ExpressionAttributeValues: exprAttrVals,
					ConditionExpression:       aws.String(condExpr),
				}

				if _, updateErr := dynamoClient.UpdateItem(updateInput); updateErr != nil {
					log.Printf("Failed to update signature %s: %v", sig.SignatureID, updateErr)
					// Emit fallback CLI
					if fallbackFile != nil {
						if err := writeFallbackCLI(fallbackFile, region, stage, tableName, sig.SignatureID, updateExpr, setCreated, setModified, newCreated, newModified); err == nil {
							fallbackCount++
						} else {
							log.Printf("WARN: could not write fallback CLI for %s: %v", sig.SignatureID, err)
						}
					}
					continue
				}
			}

			updated++
		}
		return true // Continue to next page
	})

	if err != nil {
		return updated, skipped, fallbackCount, stats, fmt.Errorf("scan failed: %v", err)
	}
	if pageErr != nil {
		return updated, skipped, fallbackCount, stats, pageErr
	}

	return updated, skipped, fallbackCount, stats, nil
}

func getCurrentTime() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// --- DocuSign XML helpers ---

var (
	reSigned    = regexp.MustCompile(`(?i)<Signed>([^<]+)</Signed>`)
	reCompleted = regexp.MustCompile(`(?i)<Completed>([^<]+)</Completed>`)
	reCreated   = regexp.MustCompile(`(?i)<Created>([^<]+)</Created>`)
)

// tryFetchFromDocusignXML fetches user_docusign_raw_xml for a single record and extracts
// a timestamp using <Signed>, then <Completed>, then <Created>.
// Returns (ts, label, true) if found; label is one of docusign_xml_signed/completed/created.
func tryFetchFromDocusignXML(ctx context.Context, ddb *dynamodb.DynamoDB, table, sigID string) (string, string, bool) {
	out, err := ddb.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {S: aws.String(sigID)},
		},
		ProjectionExpression: aws.String("user_docusign_raw_xml"),
	})
	if err != nil || out.Item == nil {
		return "", "", false
	}
	var holder struct {
		Raw string `dynamodbav:"user_docusign_raw_xml"`
	}
	if uerr := dynamodbattribute.UnmarshalMap(out.Item, &holder); uerr != nil {
		return "", "", false
	}
	raw := holder.Raw
	if raw == "" {
		return "", "", false
	}
	if m := reSigned.FindStringSubmatch(raw); len(m) == 2 {
		return strings.TrimSpace(m[1]), "docusign_xml_signed", true
	}
	if m := reCompleted.FindStringSubmatch(raw); len(m) == 2 {
		return strings.TrimSpace(m[1]), "docusign_xml_completed", true
	}
	if m := reCreated.FindStringSubmatch(raw); len(m) == 2 {
		return strings.TrimSpace(m[1]), "docusign_xml_created", true
	}
	return "", "", false
}

// --- Fallback CLI writer ---
// Includes both --region and --profile lfproduct-{stage}, as requested.
func writeFallbackCLI(
	w *os.File,
	region, stage, table, sigID, updateExpr string,
	setCreated, setModified bool,
	newCreated, newModified string,
) error {
	// Build AWS CLI JSON blobs
	key := map[string]map[string]string{
		"signature_id": {"S": sigID},
	}
	names := map[string]string{
		"#date_created":  "date_created",
		"#date_modified": "date_modified",
	}
	vals := map[string]map[string]string{
		":empty": {"S": ""},
	}
	if setCreated {
		vals[":date_created"] = map[string]string{"S": newCreated}
	}
	if setModified {
		vals[":date_modified"] = map[string]string{"S": newModified}
	}

	keyJSON, _ := json.Marshal(key)
	namesJSON, _ := json.Marshal(names)
	valsJSON, _ := json.Marshal(vals)

	cmd := fmt.Sprintf(
		`aws --profile %s dynamodb update-item --table-name %s --key '%s' --update-expression '%s' --expression-attribute-names '%s' --expression-attribute-values '%s' --condition-expression 'attribute_not_exists(#date_created) OR #date_created = :empty OR attribute_not_exists(#date_modified) OR #date_modified = :empty' --region %s`,
		shellEscape(fmt.Sprintf("lfproduct-%s", stage)),
		shellEscape(table),
		keyJSON,
		updateExpr,
		namesJSON,
		valsJSON,
		shellEscape(region),
	)

	_, err := fmt.Fprintln(w, cmd)
	return err
}

// Basic shell escaping for plain tokens we control (profile/table/region).
func shellEscape(s string) string {
	// conservative: wrap in single quotes and escape existing single quotes
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, `'`, `'\''`) + "'"
}
