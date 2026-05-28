// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sss

import (
	"fmt"
	"time"
)

// BadRequestError indicates a 400 response from the SSS API.
type BadRequestError struct {
	Message   string
	Code      string
	RequestID string
}

func (e *BadRequestError) Error() string {
	return fmt.Sprintf("bad request: %s", e.Message)
}

// AuthError indicates a 401 or 403 response from the SSS API.
type AuthError struct {
	Message   string
	Code      string
	RequestID string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication error: %s", e.Message)
}

// RetryableError indicates a 503 response from the SSS API.
type RetryableError struct {
	Message    string
	Code       string
	RequestID  string
	RetryAfter time.Duration
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable error: %s", e.Message)
}

// NotFoundError indicates a 404 response from the SSS API.
type NotFoundError struct {
	Message   string
	Code      string
	RequestID string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not found: %s", e.Message)
}

// TimeoutError indicates the request timed out.
type TimeoutError struct {
	Message string
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timeout: %s", e.Message)
}
