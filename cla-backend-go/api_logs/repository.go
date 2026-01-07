// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api_logs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	// APILogTableName is the DynamoDB table name for API logs
	APILogTableName = "cla-%s-api-log"
)

// Repository interface for API logs
type Repository interface {
	LogAPIRequest(ctx context.Context, url string) error
}

// repository implements the Repository interface
type repository struct {
	stage          string
	dynamoDBClient *dynamodb.DynamoDB
}

// NewRepository creates a new API logs repository
func NewRepository(stage string, dynamoDBClient *dynamodb.DynamoDB) Repository {
	return &repository{
		stage:          stage,
		dynamoDBClient: dynamoDBClient,
	}
}

// LogAPIRequest logs an API request to the DynamoDB table
// Creates three entries: ALL bucket, daily bucket (YYYY-MM-DD), and monthly bucket (YYYY-MM)
// IMPORTANT: table key is (url, dt). To avoid overwrites, dt is shifted by -1/0/+1 ms per bucket.
func (r *repository) LogAPIRequest(ctx context.Context, url string) error {
	// 200% fail-safe: never panic on nil ctx/client
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.dynamoDBClient == nil {
		return fmt.Errorf("dynamodb client is nil")
	}

	now := time.Now().UTC()
	timestamp := now.UnixMilli()

	// Generate bucket names
	dailyBucket := now.Format("2006-01-02") // YYYY-MM-DD
	monthlyBucket := now.Format("2006-01")  // YYYY-MM

	entries := []*APILog{
		{URL: url, DT: timestamp - 1, Bucket: "ALL"},
		{URL: url, DT: timestamp, Bucket: dailyBucket},
		{URL: url, DT: timestamp + 1, Bucket: monthlyBucket},
	}
	tableName := fmt.Sprintf(APILogTableName, r.stage)

	var errs []string
	for _, logEntry := range entries {
		// Convert to DynamoDB attribute value
		av, err := dynamodbattribute.MarshalMap(logEntry)
		if err != nil {
			errs = append(errs, fmt.Sprintf("bucket=%s marshal=%v", logEntry.Bucket, err))
			continue
		}

		// Put item to DynamoDB
		input := &dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item:      av,
		}

		_, err = r.dynamoDBClient.PutItemWithContext(ctx, input)
		if err != nil {
			errs = append(errs, fmt.Sprintf("bucket=%s put=%v", logEntry.Bucket, err))
			continue
		}
	}

	// Return error so middleware can emit a single LG:* line.
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
