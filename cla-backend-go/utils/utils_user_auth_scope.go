// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"strings"

	"github.com/LF-Engineering/lfx-kit/auth"
)

// projectOrganizationScopeMatches accepts a project|organization scope only when its organization half is companySFID
// and its project half is projectSFID or lists projectSFID in Related; lfx-kit's nested check ignores the organization
// half (lfx-self-serve-ops#382)
func projectOrganizationScopeMatches(user *auth.User, projectSFID, companySFID string) bool {
	for _, s := range user.Scopes {
		if s.Type != auth.ProjectOrganization {
			continue
		}
		if s.ID == "*" {
			return true
		}
		parts := strings.SplitN(s.ID, "|", 2)
		if len(parts) != 2 || parts[1] != companySFID {
			continue
		}
		if parts[0] == projectSFID || StringInSlice(projectSFID, s.Related) {
			return true
		}
	}
	return false
}
