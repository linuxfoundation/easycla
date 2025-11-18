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
	SignatureID            string `json:"signature_id"`
	DateCreated            string `json:"date_created,omitempty"`
	DateModified           string `json:"date_modified,omitempty"`
	SignedOn               string `json:"signed_on,omitempty"`
	UserDocusignDateSigned string `json:"user_docusign_date_signed,omitempty"`
}

func main() {
	stage := os.Getenv("STAGE")
	if stage == "" {
		stage = "dev"
	}

	dryRun := os.Getenv("DRY_RUN") == "true"
	allowCurrentTime := os.Getenv("ALLOW_CURRENT_TIME") == "true"

	fmt.Printf("Starting signature timestamp backfill for stage: %s (dry-run: %t, allow-current-time: %t)\n", stage, dryRun, allowCurrentTime)

	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	})
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

	scanInput := &dynamodb.ScanInput{
		TableName:                 aws.String(tableName),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}

	var updated int
	var skipped int
	err = dynamoClient.ScanPages(scanInput, func(page *dynamodb.ScanOutput, lastPage bool) bool {
		var signatures []SignatureRecord
		err := dynamodbattribute.UnmarshalListOfMaps(page.Items, &signatures)
		if err != nil {
			log.Printf("Failed to unmarshal signatures: %v", err)
			return false
		}

		for _, sig := range signatures {
			needsUpdate := false
			updateExpr := "SET "
			exprAttrVals := make(map[string]*dynamodb.AttributeValue)

			// Determine the best timestamp to use for missing dates
			bestTimestamp := getCurrentTime()

			// Use signed_on if available, otherwise user_docusign_date_signed
			if sig.SignedOn != "" {
				bestTimestamp = sig.SignedOn
			} else if sig.UserDocusignDateSigned != "" {
				bestTimestamp = sig.UserDocusignDateSigned
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
				fmt.Printf("Updating signature %s (created: %s -> %s, modified: %s -> %s)\n",
					sig.SignatureID,
					sig.DateCreated,
					func() string {
						if sig.DateCreated == "" {
							return bestTimestamp
						} else {
							return sig.DateCreated
						}
					}(),
					sig.DateModified,
					bestTimestamp)

				if !dryRun {
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

	return updated, skipped, nil
}

func getCurrentTime() string {
	return time.Now().UTC().Format(time.RFC3339)
}
