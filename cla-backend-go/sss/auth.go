// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// Package sss re-exports Auth functions from the shared cla-sss-base module.
package sss

import sssbase "github.com/linuxfoundation/easycla/cla-sss-base"

// Re-export factory function for backward compatibility
var NewClientFromPlatformCredentials = sssbase.NewClientFromPlatformCredentials
