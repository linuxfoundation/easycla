// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package member_service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMemberService struct {
	t            *testing.T
	tokenCalls   int
	getCalls     int
	registerBody []map[string]string
	status       int
	response     interface{}
}

func (f *fakeMemberService) decode(r *http.Request, v interface{}) {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		f.t.Error(err)
	}
}

func (f *fakeMemberService) encode(w http.ResponseWriter, v interface{}) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		f.t.Error(err)
	}
}

func (f *fakeMemberService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/oauth/token":
		f.tokenCalls++
		var req map[string]string
		f.decode(r, &req)
		assert.Equal(f.t, "client_credentials", req["grant_type"])
		assert.Equal(f.t, "https://member.example/", req["audience"])
		f.encode(w, map[string]interface{}{"access_token": "member-token", "token_type": "Bearer", "expires_in": 3600})
	case "/b2b_orgs/0014100000Te0G7AAJ":
		assert.Equal(f.t, http.MethodGet, r.Method)
		assert.Equal(f.t, "Bearer member-token", r.Header.Get("Authorization"))
		assert.Equal(f.t, "1", r.URL.Query().Get("v"))
		f.getCalls++
		w.WriteHeader(f.status)
		if f.response != nil {
			f.encode(w, f.response)
		}
	case "/b2b_orgs":
		assert.Equal(f.t, http.MethodPost, r.Method)
		assert.Equal(f.t, "Bearer member-token", r.Header.Get("Authorization"))
		assert.Equal(f.t, "1", r.URL.Query().Get("v"))
		var req map[string]string
		f.decode(r, &req)
		f.registerBody = append(f.registerBody, req)
		w.WriteHeader(f.status)
		if f.response != nil {
			f.encode(w, f.response)
		}
	default:
		http.NotFound(w, r)
	}
}

func newTestClient(t *testing.T, fake *fakeMemberService) *Client {
	fake.t = t
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client, err := NewClient(Config{BaseURL: server.URL + "/", Audience: "https://member.example/", OAuthTokenURL: server.URL + "/oauth/token", ClientID: "id", ClientSecret: "secret"})
	require.NoError(t, err)
	return client
}

func TestNewClient(t *testing.T) {
	_, err := NewClient(Config{})
	assert.ErrorIs(t, err, ErrNotConfigured)
	_, err = NewClient(Config{BaseURL: "https://m", Audience: "a"})
	assert.Error(t, err, "credentials required once configured")
	c, err := NewClient(Config{BaseURL: "https://m/", Audience: "a", OAuthTokenURL: "tenant.auth0.com", ClientID: "i", ClientSecret: "s"})
	require.NoError(t, err)
	assert.Equal(t, "https://tenant.auth0.com/oauth/token", c.cfg.OAuthTokenURL)
	assert.Equal(t, "https://m", c.cfg.BaseURL)
	c, err = NewClient(Config{BaseURL: "https://m", Audience: "a", OAuthTokenURL: "https://tenant.auth0.com/oauth/token", ClientID: "i", ClientSecret: "s"})
	require.NoError(t, err)
	assert.Equal(t, "https://tenant.auth0.com/oauth/token", c.cfg.OAuthTokenURL)
}

func TestRegisterB2BOrg(t *testing.T) {
	fake := &fakeMemberService{status: http.StatusCreated, response: map[string]string{"uid": "0014100000Te0G7AAJ", "name": "Infosys Limited", "website": "https://infosys.com"}}
	client := newTestClient(t, fake)

	org, err := client.RegisterB2BOrg(context.Background(), "0014100000Te0G7AAJ")
	require.NoError(t, err)
	assert.Equal(t, &B2BOrg{UID: "0014100000Te0G7AAJ", Name: "Infosys Limited", Website: "https://infosys.com"}, org)
	assert.Equal(t, []map[string]string{{"sfid": "0014100000Te0G7AAJ"}}, fake.registerBody)

	_, err = client.RegisterB2BOrg(context.Background(), "0014100000Te0G7AAJ")
	require.NoError(t, err)
	assert.Equal(t, 1, fake.tokenCalls, "token is cached")

	_, err = client.RegisterB2BOrg(context.Background(), "lf-not-an-sfid")
	assert.ErrorIs(t, err, ErrInvalidSFID)
	assert.Len(t, fake.registerBody, 2, "invalid ids never reach the service")

	fake.status = http.StatusOK
	org, err = client.GetB2BOrg(context.Background(), "0014100000Te0G7AAJ")
	require.NoError(t, err)
	assert.Equal(t, "Infosys Limited", org.Name)
	assert.Equal(t, 1, fake.getCalls)
	fake.status = http.StatusNotFound
	_, err = client.GetB2BOrg(context.Background(), "0014100000Te0G7AAJ")
	assert.ErrorIs(t, err, ErrOrgNotFound)
	_, err = client.GetB2BOrg(context.Background(), "lf-not-an-sfid")
	assert.ErrorIs(t, err, ErrInvalidSFID)
}

func TestRegisterB2BOrgErrors(t *testing.T) {
	fake := &fakeMemberService{status: http.StatusNotFound, response: map[string]string{"name": "NotFound", "message": "b2b org not found"}}
	client := newTestClient(t, fake)
	_, err := client.RegisterB2BOrg(context.Background(), "0014100000Te0G7AAJ")
	assert.ErrorIs(t, err, ErrOrgNotFound)

	fake.status, fake.response = http.StatusBadRequest, map[string]string{"message": "invalid Account SFID"}
	_, err = client.RegisterB2BOrg(context.Background(), "0014100000Te0G7AAJ")
	assert.ErrorIs(t, err, ErrInvalidSFID)
	assert.Contains(t, err.Error(), "invalid Account SFID")

	fake.status, fake.response = http.StatusForbidden, map[string]string{"message": "forbidden"}
	_, err = client.RegisterB2BOrg(context.Background(), "0014100000Te0G7AAJ")
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, http.StatusForbidden, authErr.Status)
	tokenCalls := fake.tokenCalls

	fake.status, fake.response = http.StatusServiceUnavailable, map[string]string{"message": "down"}
	_, err = client.RegisterB2BOrg(context.Background(), "0014100000Te0G7AAJ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
	assert.Equal(t, tokenCalls+1, fake.tokenCalls, "token is re-minted after an authorization failure")

	fake.status, fake.response = http.StatusCreated, map[string]string{"name": "no uid"}
	_, err = client.RegisterB2BOrg(context.Background(), "0014100000Te0G7AAJ")
	assert.Error(t, err)
}
