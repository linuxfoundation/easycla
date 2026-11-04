// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package user_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	mu             sync.Mutex
	requests       []*http.Request
	username       string
	tokenDone      chan struct{}
	once           sync.Once
	responseStatus int
	responseErr    error
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
		status, responseErr := f.responseStatus, f.responseErr
		f.mu.Unlock()
		if responseErr != nil {
			return nil, responseErr
		}
		if status == 0 {
			status = http.StatusOK
		}
		body, err := json.Marshal(models.UserList{Data: []*models.User{{Username: f.username}}})
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body))), Request: r}, nil
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
	requests := append([]*http.Request(nil), transport.requests...)
	transport.mu.Unlock()
	require.Len(t, requests, 2)
	for i, r := range requests {
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

	t.Run("status errors retain the username", func(t *testing.T) {
		transport.mu.Lock()
		transport.responseStatus = http.StatusServiceUnavailable
		transport.mu.Unlock()
		_, err := usc.GetUserByUsernameContext(context.Background(), raw)
		require.ErrorContains(t, err, raw)
		assert.ErrorContains(t, err, "503")
	})
	t.Run("transport errors retain their URL and cause", func(t *testing.T) {
		failure := errors.New("user-service transport unavailable")
		transport.mu.Lock()
		transport.responseErr = failure
		transport.mu.Unlock()
		_, err := usc.GetUserByUsernameContext(context.Background(), raw)
		require.ErrorIs(t, err, failure)
		var requestErr *url.Error
		require.ErrorAs(t, err, &requestErr)
		assert.Contains(t, requestErr.URL, "username="+url.QueryEscape(raw))
	})
}
