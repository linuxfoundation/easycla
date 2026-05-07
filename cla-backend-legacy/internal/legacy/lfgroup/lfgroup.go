// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package lfgroup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/logging"
)

// Client is a minimal port of the legacy Python LFGroup controller (cla/controllers/lf_group.py).
//
// It is used by a subset of legacy endpoints (for example Gerrit instance creation) to validate
// LDAP group IDs.
type Client struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	RefreshToken string

	httpClient *http.Client
}

func NewFromEnv(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		BaseURL:      strings.TrimSpace(os.Getenv("LF_GROUP_CLIENT_URL")),
		ClientID:     strings.TrimSpace(os.Getenv("LF_GROUP_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("LF_GROUP_CLIENT_SECRET")),
		RefreshToken: strings.TrimSpace(os.Getenv("LF_GROUP_REFRESH_TOKEN")),
		httpClient:   httpClient,
	}
}

func (c *Client) oauthTokenURL() string {
	return strings.TrimRight(c.BaseURL, "/") + "/oauth2/token"
}

func (c *Client) groupURL(groupID string) string {
	return strings.TrimRight(c.BaseURL, "/") + "/rest/auth0/og/" + url.PathEscape(groupID)
}

func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	if c == nil {
		return "", errors.New("lfgroup client is nil")
	}
	if c.BaseURL == "" || c.ClientID == "" || c.ClientSecret == "" || c.RefreshToken == "" {
		return "", errors.New("lfgroup client not configured")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", c.RefreshToken)
	form.Set("scope", "manage_groups")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.oauthTokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.ClientID, c.ClientSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logging.Warnf("LFGroup: unable to get access token using url: %s, error: %v", c.oauthTokenURL(), err)
		return "", err
	}
	defer resp.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}

	accessToken, _ := payload["access_token"].(string)
	return strings.TrimSpace(accessToken), nil
}

// GetGroup returns the LDAP group details for the given group id.
//
// Parity: legacy Python returns a dict with an "error" key for most failure modes.
func (c *Client) GetGroup(ctx context.Context, groupID string) map[string]any {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return map[string]any{"error": "Unable to get group"}
	}

	tok, err := c.getAccessToken(ctx)
	if err != nil || tok == "" {
		return map[string]any{"error": "Unable to retrieve access token"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.groupURL(groupID), http.NoBody)
	if err != nil {
		return map[string]any{"error": "Unable to get group"}
	}
	req.Header.Set("Authorization", "bearer "+tok)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logging.Warnf("LFGroup: unable to get group id: %s using url: %s, error: %v", groupID, c.groupURL(groupID), err)
		return map[string]any{"error": "Unable to get group"}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Mirror Python message.
		return map[string]any{"error": "The LDAP Group does not exist for this group ID."}
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return map[string]any{"error": "Unable to get group"}
	}
	return out
}

// AddUserToGroup adds a user to a given LDAP group.
//
// This isn't currently used by the ported endpoints, but it exists for parity with the Python
// controller as additional endpoints are migrated.
func (c *Client) AddUserToGroup(ctx context.Context, groupID, username string) map[string]any {
	groupID = strings.TrimSpace(groupID)
	username = strings.TrimSpace(username)
	if groupID == "" || username == "" {
		return map[string]any{"error": "Unable to update group"}
	}

	tok, err := c.getAccessToken(ctx)
	if err != nil || tok == "" {
		return map[string]any{"error": "Unable to retrieve access token"}
	}

	data, _ := json.Marshal(map[string]string{"username": username})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.groupURL(groupID), bytes.NewReader(data))
	if err != nil {
		return map[string]any{"error": "Unable to update group"}
	}
	req.Header.Set("Authorization", "bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("cache-control", "no-cache")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logging.Warnf("LFGroup: unable to update group id: %s using url: %s, error: %v", groupID, c.groupURL(groupID), err)
		return map[string]any{"error": "Unable to update group"}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		logging.Warnf("LFGroup: failed adding user %s into group %s", username, groupID)
		return map[string]any{"error": "failed to add a user to the ldap group."}
	}

	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return map[string]any{"error": "Unable to update group"}
	}
	return out
}
