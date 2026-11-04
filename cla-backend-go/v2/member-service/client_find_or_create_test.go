// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package member_service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type findOrCreateFake struct {
	t          *testing.T
	status     int
	body       string
	tokenCalls int
	calls      []string
	payloads   []map[string]string
}

func (f *findOrCreateFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == claAdminsOAuthPath {
		f.tokenCalls++
		require.NoError(f.t, json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "member-token", "token_type": "Bearer", "expires_in": 3600}))
		return
	}
	f.calls = append(f.calls, r.Method+" "+r.URL.RequestURI())
	assert.Equal(f.t, "Bearer member-token", r.Header.Get("Authorization"))
	assert.Equal(f.t, "application/json", r.Header.Get("Content-Type"))
	raw, err := io.ReadAll(r.Body)
	require.NoError(f.t, err)
	var payload map[string]string
	require.NoError(f.t, json.Unmarshal(raw, &payload))
	f.payloads = append(f.payloads, payload)
	w.WriteHeader(f.status)
	_, err = w.Write([]byte(f.body))
	require.NoError(f.t, err)
}

func newFindOrCreateClient(t *testing.T, fake *findOrCreateFake) *Client {
	fake.t = t
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client, err := NewClient(Config{BaseURL: server.URL, Audience: "https://member.example/", OAuthTokenURL: server.URL + claAdminsOAuthPath, ClientID: "client-id", ClientSecret: "client-secret"})
	require.NoError(t, err)
	return client
}

func TestFindOrCreateB2BOrg(t *testing.T) {
	const uid = "001410000FindOrAQI"
	t.Run("created", func(t *testing.T) {
		fake := &findOrCreateFake{status: http.StatusCreated, body: `{"uid":"` + uid + `","name":"New Org","website":"https://new.example","primary_domain":"new.example"}`}
		org, action, err := newFindOrCreateClient(t, fake).FindOrCreateB2BOrg(context.Background(), " New Org ", " https://new.example ")
		require.NoError(t, err)
		assert.Equal(t, ActionCreated, action)
		assert.Equal(t, uid, org.UID)
		assert.Equal(t, "New Org", org.Name)
		assert.Equal(t, []string{"POST /b2b_orgs/find-or-create?v=1"}, fake.calls)
		assert.Equal(t, []map[string]string{{"name": "New Org", "website": "https://new.example"}}, fake.payloads, "values are trimmed")
	})
	t.Run("matched", func(t *testing.T) {
		fake := &findOrCreateFake{status: http.StatusOK, body: `{"uid":"` + uid + `","name":"Existing Org"}`}
		org, action, err := newFindOrCreateClient(t, fake).FindOrCreateB2BOrg(context.Background(), "New Org", "https://existing.example")
		require.NoError(t, err)
		assert.Equal(t, ActionMatched, action)
		assert.Equal(t, "Existing Org", org.Name, "the Salesforce account name wins")
	})
	t.Run("ambiguous domain", func(t *testing.T) {
		fake := &findOrCreateFake{status: http.StatusConflict, body: `{"message":"3 accounts match example.com"}`}
		org, action, err := newFindOrCreateClient(t, fake).FindOrCreateB2BOrg(context.Background(), "Org", "https://example.com")
		assert.True(t, errors.Is(err, ErrAmbiguousDomain), "%v", err)
		assert.Contains(t, err.Error(), "3 accounts match example.com")
		assert.Nil(t, org)
		assert.Empty(t, action)
	})
	t.Run("rejected request", func(t *testing.T) {
		fake := &findOrCreateFake{status: http.StatusBadRequest, body: `{"message":"website is not a URL"}`}
		_, _, err := newFindOrCreateClient(t, fake).FindOrCreateB2BOrg(context.Background(), "Org", "not a url")
		assert.True(t, errors.Is(err, ErrInvalidRequest), "%v", err)
		assert.False(t, errors.Is(err, ErrInvalidSFID), "a 400 here is not an SFID problem")
	})
	t.Run("not eligible", func(t *testing.T) {
		fake := &findOrCreateFake{status: http.StatusUnprocessableEntity, body: `{"message":"account type"}`}
		_, _, err := newFindOrCreateClient(t, fake).FindOrCreateB2BOrg(context.Background(), "Org", "https://example.org")
		assert.True(t, errors.Is(err, ErrNotEligible), "%v", err)
	})
	t.Run("server error names the operation", func(t *testing.T) {
		fake := &findOrCreateFake{status: http.StatusServiceUnavailable, body: `{"message":"salesforce down"}`}
		_, _, err := newFindOrCreateClient(t, fake).FindOrCreateB2BOrg(context.Background(), "Org", "https://example.org")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "POST /b2b_orgs/find-or-create")
		assert.Contains(t, err.Error(), "503")
	})
	t.Run("unauthorized forgets the cached token", func(t *testing.T) {
		fake := &findOrCreateFake{status: http.StatusUnauthorized, body: `{"message":"expired"}`}
		client := newFindOrCreateClient(t, fake)
		_, _, err := client.FindOrCreateB2BOrg(context.Background(), "Org", "https://example.org")
		var authErr *AuthError
		require.True(t, errors.As(err, &authErr), "%v", err)
		assert.Equal(t, http.StatusUnauthorized, authErr.Status)
		_, _, err = client.FindOrCreateB2BOrg(context.Background(), "Org", "https://example.org")
		require.Error(t, err)
		assert.Equal(t, 2, fake.tokenCalls, "a fresh token is requested after a 401")
	})
	t.Run("missing name or website is refused before any call", func(t *testing.T) {
		fake := &findOrCreateFake{status: http.StatusCreated, body: `{"uid":"` + uid + `"}`}
		client := newFindOrCreateClient(t, fake)
		for _, in := range [][2]string{{"", "https://example.org"}, {"Org", ""}, {" ", " "}} {
			_, _, err := client.FindOrCreateB2BOrg(context.Background(), in[0], in[1])
			assert.True(t, errors.Is(err, ErrInvalidRequest), "%q: %v", in, err)
		}
		assert.Empty(t, fake.calls)
		assert.Equal(t, 0, fake.tokenCalls)
	})
	t.Run("response without uid", func(t *testing.T) {
		fake := &findOrCreateFake{status: http.StatusCreated, body: `{"name":"No UID"}`}
		org, _, err := newFindOrCreateClient(t, fake).FindOrCreateB2BOrg(context.Background(), "Org", "https://example.org")
		require.Error(t, err)
		assert.Nil(t, org)
	})
	t.Run("15-character account id is canonicalised", func(t *testing.T) {
		fake := &findOrCreateFake{status: http.StatusOK, body: `{"uid":"001410000FindOr","name":"Short"}`}
		org, action, err := newFindOrCreateClient(t, fake).FindOrCreateB2BOrg(context.Background(), "Org", "https://example.org")
		require.NoError(t, err)
		assert.Equal(t, uid, org.UID)
		assert.Equal(t, ActionMatched, action)
	})
	t.Run("malformed or non-account uid is rejected", func(t *testing.T) {
		for _, bad := range []string{"003410000FindOrAQI", "0014100000MemberSvAAA", "not-an-id", ""} {
			fake := &findOrCreateFake{status: http.StatusCreated, body: `{"uid":"` + bad + `","name":"Bad"}`}
			org, action, err := newFindOrCreateClient(t, fake).FindOrCreateB2BOrg(context.Background(), "Org", "https://example.org")
			assert.True(t, errors.Is(err, ErrInvalidSFID), "%s: %v", bad, err)
			assert.Nil(t, org)
			assert.Empty(t, action)
		}
	})
}
