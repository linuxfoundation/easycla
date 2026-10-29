// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package user_service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
)

const (
	apiHost   = "api.example"
	authHost  = "auth.example"
	fakeToken = "fake-token-abcdef1234"
)

// answers the fake Auth0 token endpoint and the user-service lookups in-process; closing the token response
// body is the end of token.Init's asynchronous request, which is signalled through tokenDone
type fakeTransport struct {
	mu        sync.Mutex
	requests  []*http.Request
	username  string
	tokenDone chan struct{}
	once      sync.Once
}

type closeSignal struct {
	io.Reader
	signal func()
}

func (c *closeSignal) Close() error {
	c.signal()
	return nil
}

func (f *fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	switch r.URL.Host {
	case authHost:
		body := &closeSignal{
			Reader: strings.NewReader(`{"access_token":"` + fakeToken + `","scope":"","expires_in":3600,"token_type":"Bearer"}`),
			signal: func() { f.once.Do(func() { close(f.tokenDone) }) },
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: body, Request: r}, nil
	case apiHost:
		f.mu.Lock()
		f.requests = append(f.requests, r.Clone(r.Context()))
		f.mu.Unlock()
		body, err := json.Marshal(models.UserList{Data: []*models.User{{Username: f.username}}})
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body))), Request: r}, nil
	default:
		return nil, fmt.Errorf("unexpected request to %s", r.URL.Host)
	}
}

// both user-service lookups send a raw LF username as one query-encoded `username` parameter (ops#808): the
// upstream request keeps the endpoint, the API key and the bearer token, and the matched user comes back
func TestUserLookupsSendTheRawUsernameAsOneQueryParameter(t *testing.T) {
	const raw = "a&b=c#d e+f/g?h"
	transport := &fakeTransport{username: raw, tokenDone: make(chan struct{})}
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })

	token.Init("client-id", "client-secret", "https://"+authHost+"/oauth/token", "aud")
	select {
	case <-transport.tokenDone:
	case <-time.After(10 * time.Second):
		t.Fatal("the token.Init request did not complete")
	}

	usc := &Client{apiKey: "api-key", apiGwURL: apiHost}
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
		assert.Equal(t, apiHost, r.URL.Host, i)
		assert.Equal(t, "/user-service/v1/users", r.URL.Path, i)
		assert.Equal(t, []string{raw}, r.URL.Query()["username"], "request %d sends the whole username as one parameter", i)
		assert.Len(t, r.URL.Query(), 1, "request %d carries no injected parameter", i)
		assert.Empty(t, r.URL.Fragment, "request %d loses nothing to a fragment", i)
		assert.Equal(t, "api-key", r.Header.Get("X-API-KEY"), i)
		assert.Equal(t, "Bearer "+fakeToken, r.Header.Get("Authorization"), i)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"), i)
	}
}
