// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// Package sss re-exports error types from the shared cla-sss-base module.
package sss

import sssbase "github.com/linuxfoundation/easycla/cla-sss-base"

// Error type aliases for backward compatibility
type BadRequestError = sssbase.BadRequestError
type AuthError = sssbase.AuthError
type RetryableError = sssbase.RetryableError
type NotFoundError = sssbase.NotFoundError
type TimeoutError = sssbase.TimeoutError
