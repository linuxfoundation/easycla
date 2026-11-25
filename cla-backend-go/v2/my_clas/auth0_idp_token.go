// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Auth0IDPTokenService returns the provider access token Auth0 holds for a linked identity of an LF login
type Auth0IDPTokenService interface {
	IdentityAccessToken(ctx context.Context, lfUsername, provider, userID string) (string, error)
}

// NewAuth0IDPTokenService builds the lookup from the shared platform M2M credentials, nil when they are not configured
func NewAuth0IDPTokenService(tokenURL, clientID, clientSecret string) Auth0IDPTokenService {
	client := newAuth0Client(tokenURL, clientID, clientSecret)
	if client == nil {
		return nil
	}
	return client
}

// IdentityAccessToken is empty when the identity is not linked or the client lacks the read:user_idp_tokens scope
func (c *auth0Client) IdentityAccessToken(ctx context.Context, lfUsername, provider, userID string) (string, error) {
	lfUsername, userID = strings.TrimSpace(lfUsername), strings.TrimSpace(userID)
	if lfUsername == "" || userID == "" {
		return "", nil
	}
	token, err := c.getToken(ctx)
	if err != nil {
		return "", err
	}
	endpoint := fmt.Sprintf("%s/api/v2/users/%s?fields=identities&include_fields=true",
		c.baseURL, url.PathEscape("auth0|"+auth0UserID(lfUsername)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() // nolint:errcheck
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512)) // nolint:errcheck
		return "", fmt.Errorf("auth0 management api returned status %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		Identities []struct {
			Provider    string          `json:"provider"`
			Connection  string          `json:"connection"`
			UserID      json.RawMessage `json:"user_id"`
			AccessToken string          `json:"access_token"`
		} `json:"identities"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&payload); decodeErr != nil {
		return "", decodeErr
	}
	for _, identity := range payload.Identities {
		source, id := auth0IdentitySource(identity.Provider, identity.Connection, strings.Trim(string(identity.UserID), `"`))
		if source == provider && id == userID {
			return identity.AccessToken, nil
		}
	}
	return "", nil
}
