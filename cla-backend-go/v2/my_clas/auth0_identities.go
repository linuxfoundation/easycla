// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/akamensky/base58"
	"github.com/sirupsen/logrus"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// mirrors lfx-v2-auth-service's mapUsernameToSub: LF usernames Auth0 cannot hold verbatim are
// stored under base58(sha512(username)) instead
var (
	auth0SafeNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,58}[A-Za-z0-9]$`)
	auth0HexUserRE  = regexp.MustCompile(`^[0-9a-f]{24,60}$`)
)

func auth0UserID(lfUsername string) string {
	if auth0SafeNameRE.MatchString(lfUsername) && !auth0HexUserRE.MatchString(lfUsername) {
		return lfUsername
	}
	hash := sha512.Sum512([]byte(lfUsername))
	return base58.Encode(hash[:])
}

// Auth0Identity is one identity linked to the LF login's Auth0 user record
type Auth0Identity struct {
	Provider string
	UserID   string
	Username string
}

// Auth0IdentityService lists the identities linked to the LF login's Auth0 user record - the
// third identity source checked after the EasyCLA user records and the platform user-service
type Auth0IdentityService interface {
	UserIdentities(ctx context.Context, lfUsername string) ([]Auth0Identity, error)
}

type auth0Client struct {
	baseURL      string
	tokenURL     string
	clientID     string
	clientSecret string
	httpClient   *http.Client

	mu     sync.Mutex
	token  string
	expiry time.Time
}

// NewAuth0IdentityService builds an Auth0 Management API client from the shared platform M2M
// credentials - tokenURL is the tenant token endpoint (cla-auth0-platform-url-{stage}), from
// which the tenant base URL and the Management API audience are derived
func NewAuth0IdentityService(tokenURL, clientID, clientSecret string) Auth0IdentityService {
	u := strings.TrimRight(strings.TrimSpace(tokenURL), "/")
	base, ok := strings.CutSuffix(u, "/oauth/token")
	if !ok || clientID == "" || clientSecret == "" {
		return nil
	}
	return &auth0Client{
		baseURL:      base,
		tokenURL:     u,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *auth0Client) UserIdentities(ctx context.Context, lfUsername string) ([]Auth0Identity, error) {
	lfUsername = strings.TrimSpace(lfUsername)
	if lfUsername == "" {
		return nil, nil
	}
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}
	auth0ID := "auth0|" + auth0UserID(lfUsername)
	endpoint := fmt.Sprintf("%s/api/v2/users/%s?fields=identities&include_fields=true",
		c.baseURL, url.PathEscape(auth0ID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint:errcheck
	if resp.StatusCode == http.StatusNotFound {
		log.WithFields(logrus.Fields{
			"functionName":   "v2.my_clas.auth0Client.UserIdentities",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"lfUsername":     lfUsername,
			"auth0UserID":    auth0ID,
		}).Warn("no Auth0 user record for the LF username - skipping the Auth0 identity source")
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512)) // nolint:errcheck
		return nil, fmt.Errorf("auth0 management api returned status %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		Identities []struct {
			Provider    string          `json:"provider"`
			UserID      json.RawMessage `json:"user_id"`
			ProfileData struct {
				Nickname string `json:"nickname"`
			} `json:"profileData"`
		} `json:"identities"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&payload); decodeErr != nil {
		return nil, decodeErr
	}
	identities := make([]Auth0Identity, 0, len(payload.Identities))
	for _, identity := range payload.Identities {
		userID := strings.Trim(string(identity.UserID), `"`)
		if userID == "null" {
			userID = ""
		}
		identities = append(identities, Auth0Identity{
			Provider: strings.ToLower(strings.TrimSpace(identity.Provider)),
			UserID:   userID,
			Username: identity.ProfileData.Nickname,
		})
	}
	return identities, nil
}

func (c *auth0Client) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Until(c.expiry) > time.Minute {
		return c.token, nil
	}
	body, err := json.Marshal(map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     c.clientID,
		"client_secret": c.clientSecret,
		"audience":      c.baseURL + "/api/v2/",
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() // nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512)) // nolint:errcheck
		return "", fmt.Errorf("auth0 token request returned status %d: %s", resp.StatusCode, string(body))
	}
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&tokenResponse); decodeErr != nil {
		return "", decodeErr
	}
	if tokenResponse.AccessToken == "" {
		return "", errors.New("empty auth0 token response")
	}
	c.token = tokenResponse.AccessToken
	c.expiry = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
	return c.token, nil
}
