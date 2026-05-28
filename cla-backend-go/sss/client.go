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

const (
	defaultTimeout  = 30 * time.Second
	defaultTokenTTL = time.Hour
	userAgent       = "easycla-cla-backend-go/sss-client"
)

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
	req.Header.Set("User-Agent", userAgent)

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
		details := responseErrorDetails(body)
		return nil, &BadRequestError{Message: details.Message, Code: details.Code, RequestID: details.RequestID}
	case http.StatusNotFound:
		details := responseErrorDetails(body)
		return nil, &NotFoundError{Message: details.Message, Code: details.Code, RequestID: details.RequestID}
	case http.StatusUnauthorized, http.StatusForbidden:
		c.invalidateToken(token)
		details := responseErrorDetails(body)
		return nil, &AuthError{Message: details.Message, Code: details.Code, RequestID: details.RequestID}
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		details := responseErrorDetails(body)
		return nil, &RetryableError{Message: details.Message, Code: details.Code, RequestID: details.RequestID, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
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
	req.Header.Set("User-Agent", userAgent)

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
		var auth0Err struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		details := upstreamErrorDetails{}
		if err := json.Unmarshal(body, &auth0Err); err == nil {
			details.Code = strings.TrimSpace(auth0Err.Error)
			details.Message = strings.TrimSpace(auth0Err.ErrorDescription)
			if details.Message == "" && details.Code != "" {
				details.Message = details.Code
			}
			if details.Message == "" {
				details.Message = resp.Status
			}
		} else {
			details.Message = strings.TrimSpace(string(body))
			if details.Message == "" {
				details.Message = resp.Status
			}
		}
		return "", &AuthError{Message: fmt.Sprintf("authentication failed: %s", details.Message), Code: details.Code, RequestID: details.RequestID}
	}

	var tokenResp authResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode auth response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", &AuthError{Message: "empty access token from auth server"}
	}

	expiresIn := time.Duration(tokenResp.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = defaultTokenTTL
	}
	c.token = tokenResp.AccessToken
	c.expiry = time.Now().Add(expiresIn)

	return c.token, nil
}

func (c *Client) invalidateToken(token string) {
	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()

	if c.token == token {
		c.token = ""
		c.expiry = time.Time{}
	}
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

type upstreamErrorDetails struct {
	Message   string
	Code      string
	RequestID string
}

func responseErrorDetails(body []byte) upstreamErrorDetails {
	var errPayload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(body, &errPayload); err == nil {
		details := upstreamErrorDetails{
			Message:   strings.TrimSpace(errPayload.Error.Message),
			Code:      strings.TrimSpace(errPayload.Error.Code),
			RequestID: strings.TrimSpace(errPayload.RequestID),
		}
		if details.Message != "" || details.Code != "" || details.RequestID != "" {
			return details
		}
	}

	return upstreamErrorDetails{Message: strings.TrimSpace(string(body))}
}

func responseMessage(body []byte) string {
	return responseErrorDetails(body).Message
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
