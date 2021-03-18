// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
)

// GetBestUsername gets best username of CLA User
func GetBestUsername(user *models.User) string {
	if user.Username != "" {
		return user.Username
	}

	if user.GithubUsername != "" {
		return user.GithubUsername
	}

	if user.LfUsername != "" {
		return user.LfUsername
	}

	return "User Name Unknown"
}
