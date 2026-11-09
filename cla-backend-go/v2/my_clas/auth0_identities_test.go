// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAuth0TestServer(t *testing.T, tokenCalls *int, userStatus int, userBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		*tokenCalls++
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "client_credentials", body["grant_type"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":86400}`)) // nolint:errcheck
	})
	mux.HandleFunc("/api/v2/users/", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(userStatus)
		_, _ = w.Write([]byte(userBody)) // nolint:errcheck
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestNewAuth0IdentityService(t *testing.T) {
	assert.Nil(t, NewAuth0IdentityService("https://tenant.auth0.com/authorize", "id", "secret"))
	assert.Nil(t, NewAuth0IdentityService("https://tenant.auth0.com/oauth/token", "", "secret"))
	assert.Nil(t, NewAuth0IdentityService("https://tenant.auth0.com/oauth/token", "id", ""))
	assert.NotNil(t, NewAuth0IdentityService("https://tenant.auth0.com/oauth/token", "id", "secret"))
	assert.NotNil(t, NewAuth0IdentityService("https://tenant.auth0.com/oauth/token/", "id", "secret"))
}

func TestAuth0UserIdentities(t *testing.T) {
	tokenCalls := 0
	server := newAuth0TestServer(t, &tokenCalls, http.StatusOK,
		`{"identities":[
			{"provider":"auth0","user_id":"someone"},
			{"provider":"Github","user_id":30514950,"profileData":{"nickname":"ah-med"}},
			{"provider":"gitlab","user_id":"777","profileData":{"nickname":"someone-gl"}},
			{"provider":"bitbucket","user_id":null,"profileData":{"nickname":"someone-bb"}}
		]}`)
	svc := newAuth0Client(server.URL+"/oauth/token", "id", "secret")
	require.NotNil(t, svc)

	identities, err := svc.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)
	assert.Equal(t, []Auth0Identity{
		{Provider: "auth0", UserID: "someone", Username: ""},
		{Provider: "github", UserID: "30514950", Username: "ah-med"},
		{Provider: "gitlab", UserID: "777", Username: "someone-gl"},
		{Provider: "bitbucket", UserID: "", Username: "someone-bb"},
	}, identities)

	_, err = svc.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)
	assert.Equal(t, 1, tokenCalls, "token must be cached across calls")

	identities, err = svc.UserIdentities(context.Background(), " ")
	require.NoError(t, err)
	assert.Nil(t, identities)
}

// golden hashes produced with lfx-v2-auth-service's mapUsernameToSub implementation
func TestAuth0UserID(t *testing.T) {
	assert.Equal(t, "someone", auth0UserID("someone"))
	assert.Equal(t, "Some.One-1_x", auth0UserID("Some.One-1_x"))
	// unsafe shapes (legacy LDAP usernames) map to base58(sha512(username))
	assert.Equal(t, "5kMCBMmM1Nz41roKkv37FeGJnEUxTVQtbWyE7Ead97tr53jpRf4Z5YT3wbmey1qnhdPacvvU5Aw81arBof2ZnoAD",
		auth0UserID("Lonnie K"))
	assert.Equal(t, "27tAPEA6iekKLujHgDMmq6EwyezNnhKPcyNcxkQeCNZ8ZGyar6hQU5zmiCDSginEtJiWAKEFz2gFoyfzfryDdwX6",
		auth0UserID("victord 14"))
	// 24-60 lowercase hex chars collide with Auth0-generated IDs, so they hash too
	assert.Equal(t, "2Ub9T1AHtbbBbaj4QcsdXmR5Gp1HZyYE7jR3mc6qcqq11u99JJbTHXHdXeLftcPwYLJNR4u8shKqXRGD2bEXpF2A",
		auth0UserID("abcdef0123456789abcdef01"))
	// single-character names fail the safe pattern and hash
	assert.NotEqual(t, "a", auth0UserID("a"))
}

func TestAuth0UserIdentitiesUnsafeUsernamePath(t *testing.T) {
	var requestedPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":86400}`)) // nolint:errcheck
	})
	mux.HandleFunc("/api/v2/users/", func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"identities":[]}`)) // nolint:errcheck
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	svc := newAuth0Client(server.URL+"/oauth/token", "id", "secret")

	_, err := svc.UserIdentities(context.Background(), "Lonnie K")
	require.NoError(t, err)
	assert.Equal(t, "/api/v2/users/auth0%7C"+auth0UserID("Lonnie K"), requestedPath)
}

func TestAuth0UserIdentitiesNotFound(t *testing.T) {
	tokenCalls := 0
	server := newAuth0TestServer(t, &tokenCalls, http.StatusNotFound, `{"error":"Not Found"}`)
	svc := newAuth0Client(server.URL+"/oauth/token", "id", "secret")

	identities, err := svc.UserIdentities(context.Background(), "missing")
	require.NoError(t, err)
	assert.Nil(t, identities)
}

func TestAuth0UserIdentitiesError(t *testing.T) {
	tokenCalls := 0
	server := newAuth0TestServer(t, &tokenCalls, http.StatusForbidden, `{"error":"insufficient scope"}`)
	svc := newAuth0Client(server.URL+"/oauth/token", "id", "secret")

	_, err := svc.UserIdentities(context.Background(), "someone")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestAuth0TokenError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"access_denied","error_description":"client-grant missing"}`)) // nolint:errcheck
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	svc := newAuth0Client(server.URL+"/oauth/token", "id", "secret")

	_, err := svc.UserIdentities(context.Background(), "someone")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
	assert.Contains(t, err.Error(), "client-grant missing")
}

// newAuth0RateLimitServer answers the user lookup with each status in turn, repeating the last one,
// and sets the given headers on every 429
func newAuth0RateLimitServer(t *testing.T, userCalls *int, rateLimitHeaders map[string]string, statuses ...int) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":86400}`)) // nolint:errcheck
	})
	mux.HandleFunc("/api/v2/users/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		status := statuses[min(*userCalls, len(statuses)-1)]
		*userCalls++
		mu.Unlock()
		if status == http.StatusTooManyRequests {
			for name, value := range rateLimitHeaders {
				w.Header().Set(name, value)
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"Too Many Requests","message":"Global limit has been reached"}`)) // nolint:errcheck
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"identities":[{"provider":"gitlab","user_id":"777","profileData":{"nickname":"someone-gl"}}]}`)) // nolint:errcheck
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// recordSleeps replaces the client's sleep with one that records each wait and returns sleepErr
func recordSleeps(client *auth0Client, sleepErr error) *[]time.Duration {
	var waits []time.Duration
	client.sleep = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return sleepErr
	}
	return &waits
}

func TestAuth0UserIdentitiesRetriesRateLimit(t *testing.T) {
	userCalls := 0
	server := newAuth0RateLimitServer(t, &userCalls,
		map[string]string{"X-RateLimit-Reset": strconv.FormatInt(time.Now().Unix(), 10)},
		http.StatusTooManyRequests, http.StatusOK)
	client := newAuth0Client(server.URL+"/oauth/token", "id", "secret")
	waits := recordSleeps(client, nil)

	identities, err := client.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)
	assert.Equal(t, []Auth0Identity{{Provider: "gitlab", UserID: "777", Username: "someone-gl"}}, identities)
	assert.Equal(t, 2, userCalls)
	require.Len(t, *waits, 1)
	assert.LessOrEqual(t, (*waits)[0], auth0RetryBudget)
}

func TestAuth0UserIdentitiesRateLimitExhausted(t *testing.T) {
	userCalls := 0
	server := newAuth0RateLimitServer(t, &userCalls, nil, http.StatusTooManyRequests)
	client := newAuth0Client(server.URL+"/oauth/token", "id", "secret")
	waits := recordSleeps(client, nil)

	_, err := client.UserIdentities(context.Background(), "someone")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errAuth0RateLimited))
	assert.Contains(t, err.Error(), "429")
	assert.Contains(t, err.Error(), "after 3 attempts")
	assert.Contains(t, err.Error(), "Global limit has been reached")
	assert.Equal(t, 1+auth0MaxRetries, userCalls)
	require.Len(t, *waits, auth0MaxRetries)
	var total time.Duration
	for _, wait := range *waits {
		assert.GreaterOrEqual(t, wait, auth0DefaultRetryWait)
		total += wait
	}
	assert.LessOrEqual(t, total, auth0RetryBudget)
}

func TestAuth0UserIdentitiesRateLimitResetBeyondBudget(t *testing.T) {
	userCalls := 0
	server := newAuth0RateLimitServer(t, &userCalls, map[string]string{"Retry-After": "10"}, http.StatusTooManyRequests)
	client := newAuth0Client(server.URL+"/oauth/token", "id", "secret")
	waits := recordSleeps(client, nil)

	_, err := client.UserIdentities(context.Background(), "someone")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errAuth0RateLimited))
	assert.Contains(t, err.Error(), "after 1 attempts")
	assert.Equal(t, 1, userCalls)
	assert.Empty(t, *waits, "a reset beyond the budget must fail without sleeping")
}

func TestAuth0UserIdentitiesRetryContextCancelled(t *testing.T) {
	userCalls := 0
	server := newAuth0RateLimitServer(t, &userCalls, nil, http.StatusTooManyRequests, http.StatusOK)
	client := newAuth0Client(server.URL+"/oauth/token", "id", "secret")
	recordSleeps(client, context.Canceled)

	identities, err := client.UserIdentities(context.Background(), "someone")
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, identities)
	assert.Equal(t, 1, userCalls)
}

func TestSleepContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, sleepContext(ctx, time.Hour), context.Canceled)
	assert.NoError(t, sleepContext(context.Background(), time.Millisecond))
}

func TestAuth0RetryWait(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tests := []struct {
		name    string
		headers map[string]string
		want    time.Duration
	}{
		{name: "retry-after seconds", headers: map[string]string{"Retry-After": "2"}, want: 2 * time.Second},
		{name: "retry-after wins over reset", headers: map[string]string{"Retry-After": "1", "X-RateLimit-Reset": "1700000009"}, want: time.Second},
		{name: "reset in the future", headers: map[string]string{"X-RateLimit-Reset": "1700000002"}, want: 2 * time.Second},
		{name: "reset in the past", headers: map[string]string{"X-RateLimit-Reset": "1699999990"}, want: 0},
		{name: "malformed headers", headers: map[string]string{"Retry-After": "soon", "X-RateLimit-Reset": "later"}, want: auth0DefaultRetryWait},
		{name: "no headers", headers: nil, want: auth0DefaultRetryWait},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{}
			for name, value := range tt.headers {
				h.Set(name, value)
			}
			assert.Equal(t, tt.want, auth0RetryWait(h, now))
		})
	}
}
