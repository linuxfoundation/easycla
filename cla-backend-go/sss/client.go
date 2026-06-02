// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// Package sss re-exports the shared SSS client from cla-sss-base for backward compatibility.
package sss

import sssbase "github.com/linuxfoundation/easycla/cla-sss-base"

// Re-export types and functions for backward compatibility
type Client = sssbase.Client
type SSSConfig = sssbase.SSSConfig
type OrganizationStatusRequest = sssbase.OrganizationStatusRequest
type ScreeningResult = sssbase.ScreeningResult
type BadRequestError = sssbase.BadRequestError
type AuthError = sssbase.AuthError
type RetryableError = sssbase.RetryableError
type NotFoundError = sssbase.NotFoundError
type TimeoutError = sssbase.TimeoutError

const (
	StatusClean   = sssbase.StatusClean
	StatusFlagged = sssbase.StatusFlagged

	SourceScreeningDB  = sssbase.SourceScreeningDB
	SourceSFDC         = sssbase.SourceSFDC
	SourceDescartesAPI = sssbase.SourceDescartesAPI
)

var (
	NewClient                        = sssbase.NewClient
	NewClientFromPlatformCredentials = sssbase.NewClientFromPlatformCredentials
)
