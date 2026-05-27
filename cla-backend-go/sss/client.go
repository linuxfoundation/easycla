// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sss

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultTimeout = 10 * time.Second

// Client is a reusable HTTP client for the Sanctions Screening Service.
type Client struct {
	cfg        SSSConfig
	httpClient *http.Client
	token      string
	expiry     time.Time
	tokenMutex sync.RWMutex
}

// NewClient creates a new SSS client configured for Auth0 client credentials.
func NewClient(cfg SSSConfig) (*Client, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	if strings.TrimSpace(cfg.Auth0Domain) == "" {
		return nil, fmt.Errorf("Auth0 domain is required")
	}
	if strings.TrimSpace(cfg.Auth0ClientID) == "" {
		return nil, fmt.Errorf("Auth0 client ID is required")
	}
	if strings.TrimSpace(cfg.Auth0ClientSecret) == "" {
		return nil, fmt.Errorf("Auth0 client secret is required")
	}
	if strings.TrimSpace(cfg.Auth0Audience) == "" {
		return nil, fmt.Errorf("Auth0 audience is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}

	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}, nil
}

// GetOrganizationStatus retrieves the sanctions screening result for an organization.
func (c *Client) GetOrganizationStatus(ctx context.Context, statusReq OrganizationStatusRequest) (*ScreeningResult, error) {
	if strings.TrimSpace(statusReq.Domain) == "" {
		return nil, &BadRequestError{Message: "domain is required"}
	}
	if strings.TrimSpace(statusReq.OrgName) == "" {
		return nil, &BadRequestError{Message: "org_name is required"}
	}

	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/api/v1/organizations/status"
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	query := reqURL.Query()
	query.Set("domain", strings.TrimSpace(statusReq.Domain))
	query.Set("org_name", strings.TrimSpace(statusReq.OrgName))
	if v := strings.TrimSpace(statusReq.Country); v != "" {
		query.Set("country", v)
	}
	if v := strings.TrimSpace(statusReq.City); v != "" {
		query.Set("city", v)
	}
	if v := strings.TrimSpace(statusReq.State); v != "" {
		query.Set("state", v)
	}
	if v := strings.TrimSpace(statusReq.PostalCode); v != "" {
		query.Set("postal_code", v)
	}
	if v := strings.TrimSpace(statusReq.SFDCID); v != "" {
		query.Set("sfdc_id", v)
	}
	if v := strings.TrimSpace(statusReq.ClearbitID); v != "" {
		query.Set("clearbit_id", v)
	}
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, toClientError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var result ScreeningResult
		if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode screening result: %w", err)
		}
		return &result, nil
	case http.StatusBadRequest:
		return nil, &BadRequestError{Message: responseMessage(body)}
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, &AuthError{Message: responseMessage(body)}
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return nil, &RetryableError{Message: responseMessage(body), RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	default:
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, responseMessage(body))
	}
}

func (c *Client) getToken(ctx context.Context) (string, error) {
	c.tokenMutex.RLock()
	currentToken := c.token
	expiry := c.expiry
	c.tokenMutex.RUnlock()

	if currentToken == "" || time.Until(expiry) <= time.Minute {
		return c.fetchToken(ctx)
	}

	return currentToken, nil
}

func (c *Client) fetchToken(ctx context.Context) (string, error) {
	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()

	if c.token != "" && time.Until(c.expiry) > time.Minute {
		return c.token, nil
	}

	requestPayload := authRequest{
		GrantType:    "client_credentials",
		ClientID:     c.cfg.Auth0ClientID,
		ClientSecret: c.cfg.Auth0ClientSecret,
		Audience:     c.cfg.Auth0Audience,
	}
	payload, err := json.Marshal(requestPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth request: %w", err)
	}

	authURL := c.authTokenURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", toClientError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read auth response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", &AuthError{Message: fmt.Sprintf("authentication failed: %s", strings.TrimSpace(string(body)))}
	}

	var authResponse authResponse
	if err := json.Unmarshal(body, &authResponse); err != nil {
		return "", fmt.Errorf("failed to decode auth response: %w", err)
	}

	if authResponse.AccessToken == "" {
		return "", &AuthError{Message: "empty access token from auth server"}
	}

	expiresIn := time.Duration(authResponse.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = defaultTimeout
	}
	c.token = authResponse.AccessToken
	c.expiry = time.Now().Add(expiresIn)

	return c.token, nil
}

func (c *Client) authTokenURL() string {
	domain := strings.TrimSpace(c.cfg.Auth0Domain)
	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		return strings.TrimRight(domain, "/") + "/oauth/token"
	}
	return "https://" + strings.TrimRight(domain, "/") + "/oauth/token"
}

func parseRetryAfter(value string) time.Duration {
	if strings.TrimSpace(value) == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		if seconds < 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}

	if parsedTime, err := http.ParseTime(value); err == nil {
		d := time.Until(parsedTime)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

func responseMessage(body []byte) string {
	var errPayload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errPayload); err == nil && strings.TrimSpace(errPayload.Error.Message) != "" {
		return strings.TrimSpace(errPayload.Error.Message)
	}
	return strings.TrimSpace(string(body))
}

func toClientError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &TimeoutError{Message: err.Error()}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &TimeoutError{Message: err.Error()}
	}

	return err
}
