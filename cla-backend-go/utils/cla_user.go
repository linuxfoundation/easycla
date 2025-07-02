// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
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

// GetBestEmail is a helper function to return the best email address for the user model
func GetBestEmail(userModel *models.User) string {
	if userModel.LfEmail != "" {
		return userModel.LfEmail.String()
	}

	for _, email := range userModel.Emails {
		if email != "" && !strings.Contains(email, "noreply.github.com") {
			return email
		}
	}

	return ""
}
