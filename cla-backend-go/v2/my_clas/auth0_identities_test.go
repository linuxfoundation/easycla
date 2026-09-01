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
}

func TestAuth0UserIdentities(t *testing.T) {
	tokenCalls := 0
	server := newAuth0TestServer(t, &tokenCalls, http.StatusOK,
		`{"identities":[
			{"provider":"auth0","user_id":"someone"},
			{"provider":"Github","user_id":30514950,"profileData":{"nickname":"ah-med"}},
			{"provider":"gitlab","user_id":"777","profileData":{"nickname":"someone-gl"}}
		]}`)
	svc := NewAuth0IdentityService(server.URL+"/oauth/token", "id", "secret")
	require.NotNil(t, svc)

	identities, err := svc.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)
	assert.Equal(t, []Auth0Identity{
		{Provider: "auth0", UserID: "someone", Username: ""},
		{Provider: "github", UserID: "30514950", Username: "ah-med"},
		{Provider: "gitlab", UserID: "777", Username: "someone-gl"},
	}, identities)

	_, err = svc.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)
	assert.Equal(t, 1, tokenCalls, "token must be cached across calls")

	identities, err = svc.UserIdentities(context.Background(), " ")
	require.NoError(t, err)
	assert.Nil(t, identities)
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
