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
	return formatError("bad request", e.Message, e.Code, e.RequestID)
}

// AuthError indicates a 401 or 403 response from the SSS API.
type AuthError struct {
	Message   string
	Code      string
	RequestID string
}

func (e *AuthError) Error() string {
	return formatError("authentication error", e.Message, e.Code, e.RequestID)
}

// RetryableError indicates a 503 response from the SSS API.
type RetryableError struct {
	Message    string
	Code       string
	RequestID  string
	RetryAfter time.Duration
}

func (e *RetryableError) Error() string {
	return formatError("retryable error", e.Message, e.Code, e.RequestID)
}

// NotFoundError indicates a 404 response from the SSS API.
type NotFoundError struct {
	Message   string
	Code      string
	RequestID string
}

func (e *NotFoundError) Error() string {
	return formatError("not found", e.Message, e.Code, e.RequestID)
}

// TimeoutError indicates the request timed out.
type TimeoutError struct {
	Message   string
	Code      string
	RequestID string
}

func (e *TimeoutError) Error() string {
	return formatError("timeout", e.Message, e.Code, e.RequestID)
}

func formatError(prefix, message, code, requestID string) string {
	if code != "" || requestID != "" {
		return fmt.Sprintf("%s: %s (code=%s request_id=%s)", prefix, message, code, requestID)
	}
	return fmt.Sprintf("%s: %s", prefix, message)
}
