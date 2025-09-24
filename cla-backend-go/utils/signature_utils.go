// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"github.com/LF-Engineering/lfx-kit/auth"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
)

// CurrentUserInACL is a helper function to determine if the current logged in user is in the specified CLA Manager list
func CurrentUserInACL(authUser *auth.User, managers []v1Models.User) bool {
	uname := "(null)"
	if authUser != nil {
		uname = authUser.UserName
	}
	f := logrus.Fields{
		"functionName": "utils.signature_utils.CurrentUserInACL",
		"authUser":     uname,
	}

	var inACL = false
	for _, manager := range managers {
		log.WithFields(f).Debugf("ACL check: %+v", manager)
		if manager.LfUsername == authUser.UserName {
			inACL = true
			break
		}
	}

	return inACL
}
