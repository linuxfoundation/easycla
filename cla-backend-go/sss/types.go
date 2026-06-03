// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// Package sss re-exports types from the shared cla-sss-base module.
package sss

import sssbase "github.com/linuxfoundation/easycla/cla-sss-base"

// Type aliases for backward compatibility
type SSSConfig = sssbase.SSSConfig
type OrganizationStatusRequest = sssbase.OrganizationStatusRequest
type ScreeningResult = sssbase.ScreeningResult

const (
	StatusClean   = sssbase.StatusClean
	StatusFlagged = sssbase.StatusFlagged

	SourceScreeningDB  = sssbase.SourceScreeningDB
	SourceSFDC         = sssbase.SourceSFDC
	SourceDescartesAPI = sssbase.SourceDescartesAPI
)
