// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ApexRequest is the design §4 find-or-create payload.
type ApexRequest struct {
	Name           string `json:"name"`
	Website        string `json:"website,omitempty"`
	Source         string `json:"source"`
	ExternalKey    string `json:"externalKey"`
	CCLASignedDate string `json:"cclaSignedDate,omitempty"`
	DryRun         bool   `json:"dryRun"`
}

// ApexResult is the Apex response.
type ApexResult struct {
	ID     string `json:"id"`
	Action string `json:"action"`
}

// ApexClient calls the Salesforce Apex REST endpoint (off unless --use-apex and both SSM params exist).
type ApexClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewApexClient returns ErrApexUnavailable when the endpoint is not configured.
func NewApexClient(baseURL, token string) (*ApexClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || strings.TrimSpace(token) == "" {
		return nil, ErrApexUnavailable
	}
	return &ApexClient{BaseURL: baseURL, Token: strings.TrimSpace(token), HTTPClient: &http.Client{Timeout: 60 * time.Second}}, nil
}

// FindOrCreate posts the request; the action is validated, the id must be a Salesforce account id unless ambiguous.
func (c *ApexClient) FindOrCreate(ctx context.Context, req ApexRequest) (*ApexResult, error) {
	if req.Source == "" {
		req.Source = "EasyCLA"
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/services/apexrest/lfx/account", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("apex: %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var res ApexResult
	if err = json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("apex: decoding response: %w", err)
	}
	res.Action = strings.ToLower(strings.TrimSpace(res.Action))
	switch res.Action {
	case ActionMatched, ActionCreated:
		if !IsSFID(res.ID) {
			return nil, fmt.Errorf("apex: action %s returned non-account id %q", res.Action, res.ID)
		}
	case ActionAmbiguous:
	default:
		return nil, fmt.Errorf("apex: unknown action %q", res.Action)
	}
	return &res, nil
}
