// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"os"
	"strconv"

	"github.com/LF-Engineering/lfx-kit/auth"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
)

// SetAuthUserProperties adds username and email to auth user
func SetAuthUserProperties(authUser *auth.User, xUserName *string, xEmail *string) {
	f := logrus.Fields{
		"functionName": "utils.SetAuthUserProperties",
		"userName":     authUser.UserName,
		"userEmail":    authUser.Email,
	}

	if xUserName != nil {
		authUser.UserName = *xUserName
	}
	if xEmail != nil {
		authUser.Email = *xEmail
	}

	tracingEnabled, conversionErr := strconv.ParseBool(os.Getenv("USER_AUTH_TRACING"))
	if conversionErr == nil && tracingEnabled {
		log.WithFields(f).Debugf("set authuser x-username: %s and x-email: %s from Auth User model", authUser.UserName, authUser.Email)
	}
}
