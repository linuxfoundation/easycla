//go:build !aws_lambda
// +build !aws_lambda

// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package handler

import (
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// RunHandler starts the lambda in local testing model by invoking the handler directly
func RunHandler() {
	f := logrus.Fields{
		"functionName": "cmd.gitlab_repository_check.handler.RunHandler",
	}
	log.WithFields(f).Debug("creating a new handler")
	err := Handler(utils.NewContext())
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error returned from handler")
	}
	log.Infof("handler completed")
}
