// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package email

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Service is a minimal interface for sending email notifications.
//
// This mirrors the legacy Python behavior where the backend delegates email delivery
// to one of several backends (SNS, SES, SMTP). For the migration we only implement
// SNS + SES, as those are the only ones used in production configurations.
type Service interface {
	Send(ctx context.Context, subject, body string, recipients []string) error
}

// NewFromEnv selects an email service implementation based on the EMAIL_SERVICE
// environment variable.
//
// Python default: EMAIL_SERVICE="SNS".
func NewFromEnv(ctx context.Context) (Service, error) {
	mode := strings.TrimSpace(os.Getenv("EMAIL_SERVICE"))
	if mode == "" {
		mode = "SNS"
	}
	mode = strings.ToUpper(mode)

	switch mode {
	case "SNS":
		return NewSNSFromEnv(ctx)
	case "SES":
		return NewSESFromEnv(ctx)
	default:
		return nil, fmt.Errorf("unsupported EMAIL_SERVICE=%q (supported: SNS, SES)", mode)
	}
}
