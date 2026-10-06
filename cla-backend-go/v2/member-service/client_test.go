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

const syntheticSFID18 = "001Ab00000CdEfGIAV"

type fakeMemberService struct {
	t            *testing.T
	tokenCalls   int
	getCalls     int
	getPaths     []string
	registerBody []map[string]string
	status       int
	response     interface{}
	tokenStatus  int
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
		if f.tokenStatus != 0 {
			w.WriteHeader(f.tokenStatus)
			f.encode(w, map[string]string{"error": "access_denied", "error_description": "Client is not authorized"})
			return
		}
		f.encode(w, map[string]interface{}{"access_token": "member-token", "token_type": "Bearer", "expires_in": 3600})
	case "/b2b_orgs/0014100000Te0G7AAJ", "/b2b_orgs/" + syntheticSFID18:
		assert.Equal(f.t, http.MethodGet, r.Method)
		assert.Equal(f.t, "Bearer member-token", r.Header.Get("Authorization"))
		assert.Equal(f.t, "1", r.URL.Query().Get("v"))
		f.getCalls++
		f.getPaths = append(f.getPaths, r.URL.Path)
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
	org, err = client.GetB2BOrg(context.Background(), " 0014100000Te0G7 ")
	require.NoError(t, err, "15-char ids are sent in the 18-char form the gateway matches tuples on")
	assert.Equal(t, "Infosys Limited", org.Name)
	assert.Equal(t, 2, fake.getCalls)
	fake.status = http.StatusCreated
	_, err = client.RegisterB2BOrg(context.Background(), "0014100000Te0G7")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"sfid": "0014100000Te0G7AAJ"}, fake.registerBody[len(fake.registerBody)-1])
	fake.status = http.StatusNotFound
	_, err = client.GetB2BOrg(context.Background(), "0014100000Te0G7AAJ")
	assert.ErrorIs(t, err, ErrOrgNotFound)
	_, err = client.GetB2BOrg(context.Background(), "lf-not-an-sfid")
	assert.ErrorIs(t, err, ErrInvalidSFID)
}

func TestSFID18(t *testing.T) {
	// real Account id pairs from the dev companies table
	for in, want := range map[string]string{
		"0014100000Te0Rk":    "0014100000Te0RkAAJ",
		"0012h00000hFI9F":    "0012h00000hFI9FAAW",
		"0014100000Te0G7":    "0014100000Te0G7AAJ",
		"0012M00002VjHnZ":    "0012M00002VjHnZQAV",
		"0012M00002p9y2q":    "0012M00002p9y2qQAA",
		"0014100000Te0yq":    "0014100000Te0yqAAB",
		"0014100000Te0RkAAJ": "0014100000Te0RkAAJ",
		"0014100000Te0Rkaaj": "0014100000Te0RkAAJ",
		" 0012M00002VjHnZ\n": "0012M00002VjHnZQAV",
		// synthetic pair: the suffix restores the letter case of any case-folded 18-char form
		"001Ab00000CdEfG":    syntheticSFID18,
		syntheticSFID18:      syntheticSFID18,
		"001ab00000cdefgiav": syntheticSFID18,
		"001AB00000CDEFGIAV": syntheticSFID18,
		"001aB00000cDeFgIaV": syntheticSFID18,
		"001Ab00000CdEfGiav": syntheticSFID18,
		"001ab00000cdefg":    "001ab00000cdefgAAA",
	} {
		got, ok := sfid18(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	// suffix outside A-Z/0-5, or an uppercase bit on a digit position, is malformed
	for _, in := range []string{"", "0014100000Te0R", "0014100000Te0RkA", "0014100000Te0RkAAJX", "0014100000Te0R-", "lf-not-an-sfid-xxx",
		"001Ab00000CdEfGIA6", "001Ab00000CdEfGIA-", "001Ab00000CdEfG-AV", "001Ab00000CdEfGJAV", "001Ab00000CdEfGIBV"} {
		got, ok := sfid18(in)
		assert.False(t, ok, in)
		assert.Empty(t, got, in)
	}
}

func TestB2BOrgCaseFoldedSFID(t *testing.T) {
	fake := &fakeMemberService{status: http.StatusOK, response: map[string]string{"uid": syntheticSFID18, "name": "Synthetic Org"}}
	client := newTestClient(t, fake)

	org, err := client.GetB2BOrg(context.Background(), "001ab00000cdefgiav")
	require.NoError(t, err)
	assert.Equal(t, syntheticSFID18, org.UID)
	assert.Equal(t, []string{"/b2b_orgs/" + syntheticSFID18}, fake.getPaths, "GET path carries the case-restored canonical id")

	fake.status = http.StatusCreated
	_, err = client.RegisterB2BOrg(context.Background(), "001AB00000CDEFGIAV")
	require.NoError(t, err)
	assert.Equal(t, []map[string]string{{"sfid": syntheticSFID18}}, fake.registerBody, "POST payload carries the case-restored canonical id")

	client = newTestClient(t, fake)
	for _, in := range []string{"001Ab00000CdEfGJAV", "001Ab00000CdEfGIA6"} {
		_, err = client.GetB2BOrg(context.Background(), in)
		assert.ErrorIs(t, err, ErrInvalidSFID, in)
		_, err = client.RegisterB2BOrg(context.Background(), in)
		assert.ErrorIs(t, err, ErrInvalidSFID, in)
	}
	assert.Equal(t, 1, fake.getCalls, "malformed suffixes never reach the service")
	assert.Len(t, fake.registerBody, 1)
	assert.Equal(t, 1, fake.tokenCalls, "malformed suffixes are refused before a token is minted")
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
	assert.False(t, authErr.Token, "a member-service 403 is not a token failure")
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

func TestTokenForbiddenIsFlagged(t *testing.T) {
	fake := &fakeMemberService{tokenStatus: http.StatusForbidden}
	client := newTestClient(t, fake)
	_, err := client.GetB2BOrg(context.Background(), "0014100000Te0G7AAJ")
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, http.StatusForbidden, authErr.Status)
	assert.True(t, authErr.Token)
	assert.Contains(t, err.Error(), "Client is not authorized")
	assert.Zero(t, fake.getCalls)
}
