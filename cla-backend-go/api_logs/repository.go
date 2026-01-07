// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api_logs

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/sirupsen/logrus"
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
func (r *repository) LogAPIRequest(ctx context.Context, url string) error {
	f := logrus.Fields{
		"functionName": "api_logs.repository.LogAPIRequest",
		"url":          url,
	}

	now := time.Now().UTC()
	timestamp := now.UnixMilli()

	// Generate bucket names
	dailyBucket := now.Format("2006-01-02") // YYYY-MM-DD
	monthlyBucket := now.Format("2006-01")  // YYYY-MM

	buckets := []string{"ALL", dailyBucket, monthlyBucket}
	tableName := fmt.Sprintf(APILogTableName, r.stage)

	for _, bucket := range buckets {
		logEntry := &APILog{
			URL:    url,
			DT:     timestamp,
			Bucket: bucket,
		}

		// Convert to DynamoDB attribute value
		av, err := dynamodbattribute.MarshalMap(logEntry)
		if err != nil {
			logrus.WithFields(f).WithError(err).Warnf("failed to marshal API log entry for bucket: %s", bucket)
			continue
		}

		// Put item to DynamoDB
		input := &dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item:      av,
		}

		_, err = r.dynamoDBClient.PutItemWithContext(ctx, input)
		if err != nil {
			logrus.WithFields(f).WithError(err).Warnf("failed to save API log entry to DynamoDB for bucket: %s", bucket)
			continue
		}

		logrus.WithFields(f).Debugf("successfully logged API request for bucket: %s", bucket)
	}

	return nil
}
