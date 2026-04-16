// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package email

import (
	"context"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

func awsRegionFromEnv() string {
	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION"))
	}
	if region == "" {
		region = strings.TrimSpace(os.Getenv("REGION"))
	}
	if region == "" {
		region = strings.TrimSpace(os.Getenv("DYNAMODB_AWS_REGION"))
	}
	if region == "" {
		region = "us-east-1"
	}
	return region
}

func newSNSClientFromEnv(ctx context.Context) (*sns.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegionFromEnv()))
	if err != nil {
		return nil, err
	}
	return sns.NewFromConfig(cfg), nil
}

func newSESClientFromEnv(ctx context.Context) (*ses.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegionFromEnv()))
	if err != nil {
		return nil, err
	}
	return ses.NewFromConfig(cfg), nil
}
