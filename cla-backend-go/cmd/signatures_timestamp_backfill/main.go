// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log"
	"os"
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
}

func main() {
	stage := os.Getenv("STAGE")
	if stage == "" {
		stage = "dev"
	}

	dryRun := os.Getenv("DRY_RUN") == "true"
	allowCurrentTime := os.Getenv("ALLOW_CURRENT_TIME") == "true"

	fmt.Printf("Starting signature timestamp backfill for stage: %s (dry-run: %t, allow-current-time: %t)\n", stage, dryRun, allowCurrentTime)

	awsSession, err := session.NewSession(&aws.Config{Region: aws.String("us-east-1")})
	// awsSession, err := session.NewSessionWithOptions(session.Options{SharedConfigState: session.SharedConfigEnable})
	if err != nil {
		log.Fatalf("Failed to create AWS session: %v", err)
	}

	dynamoClient := dynamodb.New(awsSession)
	tableName := fmt.Sprintf("cla-%s-signatures", stage)

	ctx := context.Background()
	updated, skipped, err := backfillSignatureTimestamps(ctx, dynamoClient, tableName, dryRun, allowCurrentTime)
	if err != nil {
		log.Fatalf("Failed to backfill timestamps: %v", err)
	}

	fmt.Printf("Completed backfill. Updated: %d signatures, Skipped: %d signatures (no usable timestamps).\n", updated, skipped)
}

func backfillSignatureTimestamps(ctx context.Context, dynamoClient *dynamodb.DynamoDB, tableName string, dryRun bool, allowCurrentTime bool) (int, int, error) {
	// Find signatures missing timestamps
	filter := expression.Or(
		expression.AttributeNotExists(expression.Name("date_created")),
		expression.AttributeNotExists(expression.Name("date_modified")),
		expression.Equal(expression.Name("date_created"), expression.Value("")),
		expression.Equal(expression.Name("date_modified"), expression.Value("")),
	)

	projection := expression.NamesList(
		expression.Name("signature_id"),
		expression.Name("date_created"),
		expression.Name("date_modified"),
		expression.Name("signed_on"),
		expression.Name("user_docusign_date_signed"),
	)

	expr, err := expression.NewBuilder().
		WithFilter(filter).
		WithProjection(projection).
		Build()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to build expression: %v", err)
	}

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
	err = dynamoClient.ScanPages(scanInput, func(page *dynamodb.ScanOutput, lastPage bool) bool {
		var signatures []SignatureRecord
		if uerr := dynamodbattribute.UnmarshalListOfMaps(page.Items, &signatures); uerr != nil {
			pageErr = fmt.Errorf("unmarshal page: %w", uerr)
			return false
		}

		for _, sig := range signatures {
			needsUpdate := false
			updateExpr := "SET "
			exprAttrVals := make(map[string]*dynamodb.AttributeValue)

			// Determine the best timestamp to use for missing dates
			var bestTimestamp string
			var hasUsableTimestamp bool

			// Priority 1: Use signed_on if available
			if sig.SignedOn != "" {
				bestTimestamp = sig.SignedOn
				hasUsableTimestamp = true
			} else if sig.UserDocusignDateSigned != "" {
				// Priority 2: Use user_docusign_date_signed
				bestTimestamp = sig.UserDocusignDateSigned
				hasUsableTimestamp = true
			} else if allowCurrentTime {
				// Priority 3: Use current time only if explicitly allowed
				bestTimestamp = getCurrentTime()
				hasUsableTimestamp = true
			} else {
				// No usable timestamp found and current time not allowed
				hasUsableTimestamp = false
			}

			if !hasUsableTimestamp {
				fmt.Printf("Skipping signature %s: no usable timestamp (signed_on: %q, docusign_date: %q, allow_current_time: %t)\n",
					sig.SignatureID, sig.SignedOn, sig.UserDocusignDateSigned, allowCurrentTime)
				skipped++
				continue
			}

			if sig.DateCreated == "" {
				if needsUpdate {
					updateExpr += ", "
				}
				updateExpr += "#date_created = :date_created"
				exprAttrVals[":date_created"] = &dynamodb.AttributeValue{S: aws.String(bestTimestamp)}
				needsUpdate = true
			}

			if sig.DateModified == "" {
				if needsUpdate {
					updateExpr += ", "
				}
				updateExpr += "#date_modified = :date_modified"
				exprAttrVals[":date_modified"] = &dynamodb.AttributeValue{S: aws.String(bestTimestamp)}
				needsUpdate = true
			}

			if needsUpdate {
				timestampSource := "current time"
				if sig.SignedOn != "" {
					timestampSource = "signed_on"
				} else if sig.UserDocusignDateSigned != "" {
					timestampSource = "docusign_date"
				}

				fmt.Printf("Updating signature %s (source: %s, timestamp: %s)\n",
					sig.SignatureID, timestampSource, bestTimestamp)

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

					_, updateErr := dynamoClient.UpdateItem(updateInput)
					if updateErr != nil {
						log.Printf("Failed to update signature %s: %v", sig.SignatureID, updateErr)
						continue
					}
				}
				updated++
			}
		}
		return true // Continue to next page
	})

	if err != nil {
		return updated, skipped, fmt.Errorf("scan failed: %v", err)
	}
	if pageErr != nil {
		return updated, skipped, pageErr
	}

	return updated, skipped, nil
}

func getCurrentTime() string {
	return time.Now().UTC().Format(time.RFC3339)
}
