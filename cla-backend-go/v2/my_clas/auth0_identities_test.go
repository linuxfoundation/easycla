// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	svc := NewAuth0IdentityService(server.URL+"/oauth/token", "id", "secret")
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
	svc := NewAuth0IdentityService(server.URL+"/oauth/token", "id", "secret")

	_, err := svc.UserIdentities(context.Background(), "Lonnie K")
	require.NoError(t, err)
	assert.Equal(t, "/api/v2/users/auth0%7C"+auth0UserID("Lonnie K"), requestedPath)
}

func TestAuth0UserIdentitiesNotFound(t *testing.T) {
	tokenCalls := 0
	server := newAuth0TestServer(t, &tokenCalls, http.StatusNotFound, `{"error":"Not Found"}`)
	svc := NewAuth0IdentityService(server.URL+"/oauth/token", "id", "secret")

	identities, err := svc.UserIdentities(context.Background(), "missing")
	require.NoError(t, err)
	assert.Nil(t, identities)
}

func TestAuth0UserIdentitiesError(t *testing.T) {
	tokenCalls := 0
	server := newAuth0TestServer(t, &tokenCalls, http.StatusForbidden, `{"error":"insufficient scope"}`)
	svc := NewAuth0IdentityService(server.URL+"/oauth/token", "id", "secret")

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
	svc := NewAuth0IdentityService(server.URL+"/oauth/token", "id", "secret")

	_, err := svc.UserIdentities(context.Background(), "someone")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
	assert.Contains(t, err.Error(), "client-grant missing")
}
