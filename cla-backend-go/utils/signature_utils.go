// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"github.com/LF-Engineering/lfx-kit/auth"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
)

// CurrentUserInACL is a helper function to determine if the current logged in user is in the specified CLA Manager list
func CurrentUserInACL(authUser *auth.User, managers []v1Models.User) bool {
	f := logrus.Fields{
		"functionName": "utils.CurrentUserInACL",
	}
	log.WithFields(f).Debugf("checking if user: %s is in the Signature ACL: %+v", authUser.UserName, managers)
	var inACL = false
	for _, manager := range managers {
		if manager.LfUsername == authUser.UserName {
			inACL = true
			break
		}
	}

	log.WithFields(f).Debugf("user in acl: %t", inACL)
	return inACL
}
