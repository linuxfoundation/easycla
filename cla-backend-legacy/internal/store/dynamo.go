// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// StageFromEnv returns the current deployment stage.
//
// Legacy Python defaults to "dev" when STAGE is unset.
func StageFromEnv() string {
	stage := strings.TrimSpace(os.Getenv("STAGE"))
	if stage == "" {
		stage = "dev"
	}
	return stage
}

// TableName returns the fully-qualified DynamoDB table name for a logical suffix.
// Example: suffix "users" => "cla-dev-users".
func TableName(suffix string) string {
	return fmt.Sprintf("cla-%s-%s", StageFromEnv(), suffix)
}

// TableNameFromSuffix is a small compatibility shim used by some store files.
//
// Earlier iterations used this helper name; keep it to avoid churn.
func TableNameFromSuffix(suffix string) string {
	return TableName(suffix)
}

// NewDynamoDBClientFromEnv creates a DynamoDB client using the ambient AWS environment.
//
// For legacy parity we keep this intentionally minimal and rely on the Lambda execution
// role + standard AWS_REGION/AWS_DEFAULT_REGION behavior.
func NewDynamoDBClientFromEnv(ctx context.Context) (*dynamodb.Client, error) {
	region := strings.TrimSpace(os.Getenv("DYNAMODB_AWS_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_REGION"))
	}
	if region == "" {
		region = strings.TrimSpace(os.Getenv("REGION"))
	}
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	return dynamodb.NewFromConfig(cfg), nil
}
