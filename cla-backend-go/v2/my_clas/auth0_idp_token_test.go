// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuth0IDPTokenService(t *testing.T) {
	assert.Nil(t, NewAuth0IDPTokenService("https://tenant.auth0.com/authorize", "id", "secret"))
	assert.Nil(t, NewAuth0IDPTokenService("https://tenant.auth0.com/oauth/token", "", "secret"))
	assert.NotNil(t, NewAuth0IDPTokenService("https://tenant.auth0.com/oauth/token", "id", "secret"))
}

func TestAuth0IdentityAccessToken(t *testing.T) {
	tokenCalls := 0
	server := newAuth0TestServer(t, &tokenCalls, http.StatusOK,
		`{"identities":[
			{"provider":"auth0","user_id":"someone"},
			{"provider":"Github","user_id":30514950,"access_token":"gho_test","profileData":{"nickname":"ah-med"}},
			{"provider":"github","user_id":"777","profileData":{"nickname":"no-token"}},
			{"provider":"oauth2","connection":"gitlab","user_id":"gitlab|888","access_token":"glpat_test"}
		]}`)
	svc := newAuth0Client(server.URL+"/oauth/token", "id", "secret")
	require.NotNil(t, svc)

	tests := []struct {
		name, lfUsername, provider, userID, want string
	}{
		{name: "the token of the linked GitHub identity", lfUsername: "someone", provider: "github", userID: "30514950", want: "gho_test"},
		{name: "an identity without a token", lfUsername: "someone", provider: "github", userID: "777"},
		{name: "an identity that is not linked", lfUsername: "someone", provider: "github", userID: "1"},
		{name: "the same id under another provider", lfUsername: "someone", provider: "gitlab", userID: "30514950"},
		{name: "a custom social connection", lfUsername: "someone", provider: "gitlab", userID: "888", want: "glpat_test"},
		{name: "no LF username", lfUsername: " ", provider: "github", userID: "30514950"},
		{name: "no identity id", lfUsername: "someone", provider: "github", userID: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := svc.IdentityAccessToken(context.Background(), tt.lfUsername, tt.provider, tt.userID)

			require.NoError(t, err)
			assert.Equal(t, tt.want, token)
		})
	}
	assert.Equal(t, 1, tokenCalls)
}

func TestAuth0IdentityAccessTokenFailures(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{name: "no Auth0 user", status: http.StatusNotFound, body: `{}`},
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"message":"Too Many Requests"}`, wantErr: true},
		{name: "server error", status: http.StatusInternalServerError, body: `oops`, wantErr: true},
		{name: "not JSON", status: http.StatusOK, body: `oops`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenCalls := 0
			server := newAuth0TestServer(t, &tokenCalls, tt.status, tt.body)
			svc := newAuth0Client(server.URL+"/oauth/token", "id", "secret")
			require.NotNil(t, svc)

			token, err := svc.IdentityAccessToken(context.Background(), "someone", "github", "30514950")

			assert.Empty(t, token)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
