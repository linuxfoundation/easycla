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
	// Auth0Audience is the Auth0 API audience/resource server identifier.
	// Production values may require the exact identifier configured in Auth0,
	// including a trailing slash when the resource server uses one.
	Auth0Audience string
	// Timeout is shared by SSS API requests and Auth0 token acquisition requests.
	Timeout time.Duration
}

// OrganizationStatusRequest holds parameters for querying organization screening status.
type OrganizationStatusRequest struct {
	Domain     string `json:"domain"`
	OrgName    string `json:"org_name"`
	Country    string `json:"country,omitempty"`
	City       string `json:"city,omitempty"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	SFDCID     string `json:"sfdc_id,omitempty"`
	ClearbitID string `json:"clearbit_id,omitempty"`
}

const (
	StatusClean   = "clean"
	StatusFlagged = "flagged"

	SourceScreeningDB  = "screening_db"
	SourceSFDC         = "sfdc"
	SourceDescartesAPI = "descartes_api"
)

// ScreeningResult is returned by the SSS organization status endpoint.
type ScreeningResult struct {
	Status           string    `json:"status"`
	EntityID         string    `json:"entity_id"`
	Source           string    `json:"source"`
	ScreenedAt       time.Time `json:"screened_at"`
	ClearbitID       string    `json:"clearbit_id"`
	SFDCID           *string   `json:"sfdc_id"`
	OrgName          string    `json:"org_name"`
	Domain           string    `json:"domain"`
	Vendor           string    `json:"vendor"`
	ClearbitEnriched bool      `json:"clearbit_enriched"`
}
