// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package user_service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
)

// answers the user-service lookups for api.example with one user and records every request; anything else
// (the fake Auth0 server) goes to the real transport
type capturingTransport struct {
	mu       sync.Mutex
	requests []*http.Request
	fallback http.RoundTripper
	username string
}

func (c *capturingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != "api.example" {
		return c.fallback.RoundTrip(r)
	}
	c.mu.Lock()
	c.requests = append(c.requests, r.Clone(r.Context()))
	c.mu.Unlock()
	body, err := json.Marshal(models.UserList{Data: []*models.User{{Username: c.username}}})
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body))), Request: r}, nil
}

// both user-service lookups send a raw LF username as one query-encoded `username` parameter (ops#808): the
// upstream request keeps the endpoint, the API key and the bearer token, and the matched user comes back
func TestUserLookupsSendTheRawUsernameAsOneQueryParameter(t *testing.T) {
	const raw = "a&b=c#d e+f/g?h"
	const fakeToken = "fake-token-abcdef1234"
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"access_token":"` + fakeToken + `","scope":"","expires_in":3600,"token_type":"Bearer"}`)); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(auth.Close)
	token.Init("client-id", "client-secret", auth.URL+"/oauth/token", "aud")
	require.Eventually(t, func() bool {
		tok, err := token.GetToken()
		return err == nil && tok == fakeToken
	}, 5*time.Second, 50*time.Millisecond, "the fake Auth0 token must be in place before the lookups")

	original := http.DefaultTransport
	transport := &capturingTransport{fallback: original, username: raw}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })

	usc := &Client{apiKey: "api-key", apiGwURL: "api.example"}
	listed, err := usc.ListUsersByUsername(raw)
	require.NoError(t, err)
	assert.Equal(t, raw, listed.Username)
	byContext, err := usc.GetUserByUsernameContext(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, raw, byContext.Username)

	transport.mu.Lock()
	defer transport.mu.Unlock()
	require.Len(t, transport.requests, 2)
	for i, r := range transport.requests {
		assert.Equal(t, http.MethodGet, r.Method, i)
		assert.Equal(t, "api.example", r.URL.Host, i)
		assert.Equal(t, "/user-service/v1/users", r.URL.Path, i)
		assert.Equal(t, []string{raw}, r.URL.Query()["username"], "request %d sends the whole username as one parameter", i)
		assert.Len(t, r.URL.Query(), 1, "request %d carries no injected parameter", i)
		assert.Empty(t, r.URL.Fragment, "request %d loses nothing to a fragment", i)
		assert.Equal(t, "api-key", r.Header.Get("X-API-KEY"), i)
		assert.Equal(t, "Bearer "+fakeToken, r.Header.Get("Authorization"), i)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"), i)
	}
}
