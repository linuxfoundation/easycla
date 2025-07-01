//go:build aws_lambda
// +build aws_lambda

// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/LF-Engineering/aws-lambda-go-api-proxy/httpadapter"
	"github.com/aws/aws-lambda-go/lambda"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/spf13/cobra"
)

func runServer(cmd *cobra.Command, args []string) {
	log.Info("Lambda server starting...")
	handler := server(false)

	lambdaHandler := httpadapter.New(handler)

	lambda.Start(lambdaHandler.Proxy)
	log.Infof("Lambda shutting down...")
}
