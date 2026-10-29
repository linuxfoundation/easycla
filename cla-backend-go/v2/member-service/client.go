// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// Package member_service is a minimal client for the LFX v2 member-service used by the org import
// tool to register EasyCLA organizations as B2B orgs (POST /b2b_orgs).
package member_service

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
	"sync"
	"time"
)

// ErrOrgNotFound is returned when member-service (Salesforce) does not know the account SFID.
var ErrOrgNotFound = errors.New("member-service: organization not found")

// ErrInvalidSFID is returned when member-service rejects the SFID (400).
var ErrInvalidSFID = errors.New("member-service: invalid organization sfid")

// ErrNotEligible is returned when member-service refuses the account as a b2b org (422).
var ErrNotEligible = errors.New("member-service: account not eligible for a b2b org")

// ErrNotConfigured is returned when the client has no base URL or audience.
var ErrNotConfigured = errors.New("member-service: client not configured")

// ErrConflict is returned when member-service reports a concurrent update (409); retry with fresh state.
var ErrConflict = errors.New("member-service: concurrent update")

// ErrInvalidRequest is returned when member-service rejects a request body (400) other than the SFID.
var ErrInvalidRequest = errors.New("member-service: request rejected")

// CLAAdmin is one CLA manager of the organization's cla_admins list (lfx-self-serve#4362).
type CLAAdmin struct {
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
}

// AuthError is returned for token or authorization failures (Auth0 non-2xx, member-service 401/403).
type AuthError struct {
	Status  int
	Message string
	Token   bool // the Auth0 token request failed, not the member-service call
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("member-service: authorization failed (%d): %s", e.Status, e.Message)
}

// Config is the member-service client configuration; the Auth0 client credentials are the shared
// LFX platform M2M credentials already used by EasyCLA, only the audience differs.
type Config struct {
	BaseURL       string
	Audience      string
	OAuthTokenURL string
	ClientID      string
	ClientSecret  string
}

// B2BOrg is the member-service B2B organization record (subset).
type B2BOrg struct {
	UID           string `json:"uid"`
	Name          string `json:"name"`
	Website       string `json:"website,omitempty"`
	PrimaryDomain string `json:"primary_domain,omitempty"`
}

// Client is the member-service client
type Client struct {
	cfg        Config
	httpClient *http.Client

	mu     sync.Mutex
	token  string
	expiry time.Time
}

// NewClient creates a client; it returns ErrNotConfigured when the base URL or audience is empty so
// callers can treat member-service registration as an unavailable route.
func NewClient(cfg Config) (*Client, error) {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.Audience = strings.TrimSpace(cfg.Audience)
	cfg.OAuthTokenURL = strings.TrimSpace(cfg.OAuthTokenURL)
	if cfg.BaseURL == "" || cfg.Audience == "" {
		return nil, ErrNotConfigured
	}
	if cfg.OAuthTokenURL == "" || strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, fmt.Errorf("member-service: auth0 token url, client id and client secret are required")
	}
	if !strings.Contains(cfg.OAuthTokenURL, "://") {
		cfg.OAuthTokenURL = "https://" + cfg.OAuthTokenURL
	}
	if !strings.HasSuffix(cfg.OAuthTokenURL, "/oauth/token") {
		cfg.OAuthTokenURL = strings.TrimRight(cfg.OAuthTokenURL, "/") + "/oauth/token"
	}
	return &Client{cfg: cfg, httpClient: &http.Client{Timeout: 30 * time.Second}}, nil
}

const sfidSuffixChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"

// sfid18 returns the canonical 18-character form of a 15- or 18-character Salesforce ID; the
// gateway matches the path id against b2b_org:<18-char> FGA tuples, so a 15-character id is
// always refused.
func sfid18(id string) (string, bool) {
	id = strings.TrimSpace(id)
	if len(id) != 15 && len(id) != 18 {
		return "", false
	}
	var b [18]byte
	copy(b[:], id[:15])
	for _, c := range b[:15] {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return "", false
		}
	}
	if len(id) == 18 && !restoreSFIDCase(b[:15], id[15:]) {
		return "", false
	}
	for g := 0; g < 3; g++ {
		bits := 0
		for j := 0; j < 5; j++ {
			if c := b[g*5+j]; c >= 'A' && c <= 'Z' {
				bits |= 1 << j
			}
		}
		b[15+g] = sfidSuffixChars[bits]
	}
	return string(b[:]), true
}

// SFID18 returns the canonical 18-character form of a 15- or 18-character Salesforce ID.
func SFID18(id string) (string, bool) {
	return sfid18(id)
}

// restoreSFIDCase re-applies the letter case encoded by the case-insensitive 3-character suffix
// (bit j of suffix character g set <=> position g*5+j is an uppercase letter).
func restoreSFIDCase(id []byte, suffix string) bool {
	for g := 0; g < 3; g++ {
		s := suffix[g]
		if s >= 'a' && s <= 'z' {
			s -= 'a' - 'A'
		}
		bits := strings.IndexByte(sfidSuffixChars, s)
		if bits < 0 {
			return false
		}
		for j := 0; j < 5; j++ {
			c := &id[g*5+j]
			switch {
			case bits&(1<<j) == 0:
				if *c >= 'A' && *c <= 'Z' {
					*c += 'a' - 'A'
				}
			case *c >= 'a' && *c <= 'z':
				*c -= 'a' - 'A'
			case *c < 'A' || *c > 'Z':
				return false
			}
		}
	}
	return true
}

// GetB2BOrg returns the registered B2B org (dry-run liveness check; member-service reads Salesforce).
func (c *Client) GetB2BOrg(ctx context.Context, uid string) (*B2BOrg, error) {
	uid, ok := sfid18(uid)
	if !ok {
		return nil, ErrInvalidSFID
	}
	tok, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+"/b2b_orgs/"+url.PathEscape(uid)+"?"+url.Values{"v": {"1"}}.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	return c.do(req, tok, "GET /b2b_orgs/{uid}")
}

// RegisterB2BOrg registers the Salesforce account as a B2B org (idempotent on the member-service
// side) and returns the resulting record.
func (c *Client) RegisterB2BOrg(ctx context.Context, sfid string) (*B2BOrg, error) {
	sfid, ok := sfid18(sfid)
	if !ok {
		return nil, ErrInvalidSFID
	}
	tok, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]string{"sfid": sfid})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/b2b_orgs?"+url.Values{"v": {"1"}}.Encode(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return c.do(req, tok, "POST /b2b_orgs")
}

// PutB2BOrgCLAAdmins replaces the organization's CLA manager list (PUT /b2b_orgs/{uid}/settings/cla_admins);
// member-service writes the cla_admin tuples itself. An empty list clears it; 404 means no b2b_org yet.
func (c *Client) PutB2BOrgCLAAdmins(ctx context.Context, uid string, admins []CLAAdmin, sourceUpdatedAt time.Time) error {
	uid, ok := sfid18(uid)
	if !ok {
		return ErrInvalidSFID
	}
	tok, err := c.getToken(ctx)
	if err != nil {
		return err
	}
	if admins == nil {
		admins = []CLAAdmin{}
	}
	payload, err := json.Marshal(struct {
		CLAAdmins       []CLAAdmin `json:"cla_admins"`
		SendInvite      bool       `json:"send_invite"`
		SourceUpdatedAt string     `json:"source_updated_at"`
	}{CLAAdmins: admins, SendInvite: false, SourceUpdatedAt: sourceUpdatedAt.UTC().Format(time.RFC3339Nano)})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.cfg.BaseURL+"/b2b_orgs/"+url.PathEscape(uid)+"/settings/cla_admins?"+url.Values{"v": {"1"}}.Encode(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return nil
	}
	if resp.StatusCode == http.StatusBadRequest {
		return fmt.Errorf("%w: %s", ErrInvalidRequest, responseMessage(body, resp.Status))
	}
	return c.statusError(resp.StatusCode, resp.Status, body, tok, "PUT /b2b_orgs/{uid}/settings/cla_admins")
}

func (c *Client) do(req *http.Request, tok, operation string) (*B2BOrg, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated:
		var org B2BOrg
		if err = json.Unmarshal(body, &org); err != nil {
			return nil, fmt.Errorf("member-service: decoding b2b org response: %w", err)
		}
		if org.UID == "" {
			return nil, fmt.Errorf("member-service: b2b org response has no uid")
		}
		return &org, nil
	default:
		return nil, c.statusError(resp.StatusCode, resp.Status, body, tok, operation)
	}
}

// statusError maps a non-2xx member-service response to the package sentinels.
func (c *Client) statusError(status int, statusText string, body []byte, tok, operation string) error {
	switch status {
	case http.StatusNotFound:
		return ErrOrgNotFound
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrInvalidSFID, responseMessage(body, statusText))
	case http.StatusConflict:
		return fmt.Errorf("%w: %s", ErrConflict, responseMessage(body, statusText))
	case http.StatusUnprocessableEntity:
		return fmt.Errorf("%w: %s", ErrNotEligible, responseMessage(body, statusText))
	case http.StatusUnauthorized, http.StatusForbidden:
		if status == http.StatusUnauthorized {
			c.forgetToken(tok)
		}
		return &AuthError{Status: status, Message: responseMessage(body, statusText)}
	default:
		return fmt.Errorf("member-service: %s returned %d: %s", operation, status, responseMessage(body, statusText))
	}
}

func (c *Client) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Until(c.expiry) > time.Minute {
		return c.token, nil
	}
	payload, err := json.Marshal(map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     strings.TrimSpace(c.cfg.ClientID),
		"client_secret": strings.TrimSpace(c.cfg.ClientSecret),
		"audience":      c.cfg.Audience,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OAuthTokenURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if readErr != nil {
		return "", readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", &AuthError{Status: resp.StatusCode, Message: responseMessage(body, resp.Status), Token: true}
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err = json.Unmarshal(body, &tr); err != nil || tr.AccessToken == "" {
		return "", &AuthError{Status: resp.StatusCode, Message: "empty access token", Token: true}
	}
	c.token = tr.AccessToken
	c.expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return c.token, nil
}

func (c *Client) forgetToken(tok string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == tok {
		c.token, c.expiry = "", time.Time{}
	}
}

func responseMessage(body []byte, fallback string) string {
	var payload struct {
		Message          string `json:"message"`
		Name             string `json:"name"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &payload) == nil {
		for _, s := range []string{payload.Message, payload.ErrorDescription, payload.Error, payload.Name} {
			if strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	if s := strings.TrimSpace(string(body)); s != "" {
		if len(s) > 512 {
			s = s[:512]
		}
		return s
	}
	return fallback
}
