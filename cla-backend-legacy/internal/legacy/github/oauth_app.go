// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package githublegacy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHub OAuth application endpoints are fixed for github.com.
// (Legacy Python: cla/config.py sets these constants.)
const (
	githubOAuthAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubOAuthExchangeURL  = "https://github.com/login/oauth/access_token"
)

// OAuthToken is the JSON response returned by GitHub when exchanging a code.
// We keep it as a map-compatible struct to mirror legacy Python which stores
// the whole token dict in session.
type OAuthToken struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// BuildOAuthAuthorizeURL builds the GitHub OAuth authorize URL.
// This mirrors the behavior of requests-oauthlib OAuth2Session.authorization_url.
func BuildOAuthAuthorizeURL(clientID string, redirectURI string, scopes []string, state string) (string, error) {
	if strings.TrimSpace(clientID) == "" {
		return "", errors.New("missing clientID")
	}
	u, err := url.Parse(githubOAuthAuthorizeURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", clientID)
	if strings.TrimSpace(redirectURI) != "" {
		q.Set("redirect_uri", redirectURI)
	}
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, " "))
	}
	if strings.TrimSpace(state) != "" {
		q.Set("state", state)
	}
	// requests-oauthlib sets response_type=code by default.
	q.Set("response_type", "code")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ExchangeOAuthToken exchanges an OAuth2 code for an access token.
//
// Legacy Python: cla.utils.fetch_token() -> requests-oauthlib OAuth2Session.fetch_token.
func (s *Service) ExchangeOAuthToken(ctx context.Context, clientID, clientSecret, code, state string) (map[string]any, error) {
	if strings.TrimSpace(clientID) == "" {
		return nil, errors.New("missing client id")
	}
	if strings.TrimSpace(clientSecret) == "" {
		return nil, errors.New("missing client secret")
	}
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("missing code")
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	if strings.TrimSpace(state) != "" {
		form.Set("state", state)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubOAuthExchangeURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Ask GitHub for JSON rather than query-string payload.
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cla-backend-legacy")

	hc := s.httpClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("github oauth token exchange failed: status=%d body=%s", resp.StatusCode, string(b))
	}

	var tok OAuthToken
	if err := json.Unmarshal(b, &tok); err != nil {
		return nil, err
	}
	if strings.TrimSpace(tok.Error) != "" {
		if strings.TrimSpace(tok.ErrorDescription) != "" {
			return nil, fmt.Errorf("github oauth token error: %s (%s)", tok.Error, tok.ErrorDescription)
		}
		return nil, fmt.Errorf("github oauth token error: %s", tok.Error)
	}
	if strings.TrimSpace(tok.AccessToken) == "" {
		return nil, errors.New("github oauth token response missing access_token")
	}

	// Return as a generic map to store in the session without losing fields.
	out := map[string]any{
		"access_token": tok.AccessToken,
	}
	if strings.TrimSpace(tok.TokenType) != "" {
		out["token_type"] = tok.TokenType
	}
	if strings.TrimSpace(tok.Scope) != "" {
		out["scope"] = tok.Scope
	}
	return out, nil
}

func oauthAuthHeader(tokenType, accessToken string) string {
	tt := strings.ToLower(strings.TrimSpace(tokenType))
	if tt == "" || tt == "bearer" {
		return "Bearer " + accessToken
	}
	return strings.TrimSpace(tokenType) + " " + accessToken
}

// GetOAuthUser calls GET /user using the OAuth access token.
func (s *Service) GetOAuthUser(ctx context.Context, token map[string]any) (map[string]any, error) {
	if token == nil {
		return nil, errors.New("missing token")
	}
	access, _ := token["access_token"].(string)
	if strings.TrimSpace(access) == "" {
		return nil, errors.New("missing access_token")
	}
	tokenType, _ := token["token_type"].(string)

	endpoint := envGitHubAPIBaseURL() + "/user"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", oauthAuthHeader(tokenType, access))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cla-backend-legacy")

	hc := s.httpClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("github oauth user request failed: status=%d body=%s", resp.StatusCode, string(b))
	}

	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

type oauthEmail struct {
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
	Primary  bool   `json:"primary"`
}

// GetOAuthVerifiedEmails returns verified emails, preferring non-noreply addresses.
// Legacy Python excludes emails ending with "noreply.github.com" unless no alternative exists.
func (s *Service) GetOAuthVerifiedEmails(ctx context.Context, token map[string]any) ([]string, error) {
	if token == nil {
		return nil, errors.New("missing token")
	}
	access, _ := token["access_token"].(string)
	if strings.TrimSpace(access) == "" {
		return nil, errors.New("missing access_token")
	}
	tokenType, _ := token["token_type"].(string)

	endpoint := envGitHubAPIBaseURL() + "/user/emails"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", oauthAuthHeader(tokenType, access))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cla-backend-legacy")

	hc := s.httpClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("github oauth emails request failed: status=%d body=%s", resp.StatusCode, string(b))
	}

	var payload []oauthEmail
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, err
	}

	verified := make([]string, 0)
	for _, e := range payload {
		if e.Verified && strings.TrimSpace(e.Email) != "" {
			verified = append(verified, strings.TrimSpace(e.Email))
		}
	}
	if len(verified) == 0 {
		return []string{}, nil
	}

	excluded := make([]string, 0)
	included := make([]string, 0)
	for _, e := range verified {
		if strings.HasSuffix(strings.ToLower(e), "noreply.github.com") {
			excluded = append(excluded, e)
		} else {
			included = append(included, e)
		}
	}
	if len(included) > 0 {
		return included, nil
	}
	return excluded, nil
}
