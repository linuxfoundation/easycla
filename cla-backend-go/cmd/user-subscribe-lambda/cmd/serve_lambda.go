//go:build aws_lambda
// +build aws_lambda

// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	log "github.com/sirupsen/logrus"
)

type fn func(ctx context.Context, sqsEvent events.SNSEvent) error

// Start starts the specified handler
func Start(handler fn) error {
	log.Println("starting lambda handler...")

	lambda.Start(handler)
	return nil
}
