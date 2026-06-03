// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// Package sss re-exports the shared SSS client from cla-sss-base for backward compatibility.
package sss

import sssbase "github.com/linuxfoundation/easycla/cla-sss-base"

// Re-export types and functions for backward compatibility
type Client = sssbase.Client

var (
	NewClient = sssbase.NewClient
)
