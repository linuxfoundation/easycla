// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sss

import "time"

// SSSConfig holds configuration values for the SSS client.
type SSSConfig struct {
	BaseURL           string
	Auth0Domain       string
	Auth0ClientID     string
	Auth0ClientSecret string
	Auth0Audience     string
	Timeout           time.Duration
}

// ScreeningResult is returned by the SSS organization status endpoint.
type ScreeningResult struct {
	Status     string    `json:"status"`
	EntityID   string    `json:"entity_id"`
	Source     string    `json:"source"`
	ScreenedAt time.Time `json:"screened_at"`
}
