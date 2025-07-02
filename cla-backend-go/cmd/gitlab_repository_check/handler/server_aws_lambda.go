//go:build aws_lambda
// +build aws_lambda

// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package handler

import (
	"github.com/aws/aws-lambda-go/lambda"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
)

// RunHandler starts the lambda main handler routine
func RunHandler() {
	f := logrus.Fields{
		"functionName": "cmd.gitlab_repository_check.handler.RunHandler",
	}
	log.WithFields(f).Info("lambda server starting...")
	lambda.Start(Handler)
	log.WithFields(f).Infof("Lambda shutting down...")
}
