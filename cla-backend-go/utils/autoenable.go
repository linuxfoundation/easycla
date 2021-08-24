// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

// ValidateAutoEnabledClaGroupID checks for validation if autoEnabled flag is on autoEnabledClaGroupID is enabled as well
func ValidateAutoEnabledClaGroupID(autoEnabled bool, autoEnabledClaGroupID string) bool {
	if !autoEnabled {
		return true
	}

	return autoEnabledClaGroupID != ""
}
