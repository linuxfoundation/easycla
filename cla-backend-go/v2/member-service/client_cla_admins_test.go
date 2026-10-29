// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package member_service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const claAdminsOAuthPath = "/oauth/token"

type claAdminsFake struct {
	t          *testing.T
	status     int
	body       string
	tokenCalls int
	paths      []string
	payloads   []map[string]interface{}
}

func (f *claAdminsFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == claAdminsOAuthPath {
		f.tokenCalls++
		require.NoError(f.t, json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "member-token", "token_type": "Bearer", "expires_in": 3600}))
		return
	}
	assert.Equal(f.t, http.MethodPut, r.Method)
	assert.Equal(f.t, "Bearer member-token", r.Header.Get("Authorization"))
	assert.Equal(f.t, "application/json", r.Header.Get("Content-Type"))
	assert.Equal(f.t, "1", r.URL.Query().Get("v"))
	f.paths = append(f.paths, r.URL.Path)
	raw, err := io.ReadAll(r.Body)
	require.NoError(f.t, err)
	var payload map[string]interface{}
	require.NoError(f.t, json.Unmarshal(raw, &payload))
	f.payloads = append(f.payloads, payload)
	w.WriteHeader(f.status)
	_, err = w.Write([]byte(f.body))
	require.NoError(f.t, err)
}

func newCLAAdminsClient(t *testing.T, fake *claAdminsFake) *Client {
	fake.t = t
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client, err := NewClient(Config{BaseURL: server.URL, Audience: "https://member.example/", OAuthTokenURL: server.URL + claAdminsOAuthPath, ClientID: "id", ClientSecret: "secret"})
	require.NoError(t, err)
	return client
}

func TestPutB2BOrgCLAAdmins(t *testing.T) {
	fake := &claAdminsFake{status: http.StatusOK, body: `{"uid":"0014100000Te0G7AAJ","cla_admins":[]}`}
	client := newCLAAdminsClient(t, fake)
	at := time.Date(2026, 10, 28, 12, 40, 5, 0, time.FixedZone("CET", 3600))

	err := client.PutB2BOrgCLAAdmins(context.Background(), "0014100000Te0G7", []CLAAdmin{{Username: "jdoe", Email: "jdoe@example.com", Name: "J Doe"}, {Username: "nomail"}}, at)
	require.NoError(t, err)
	require.Len(t, fake.payloads, 1)
	assert.Equal(t, "/b2b_orgs/0014100000Te0G7AAJ/settings/cla_admins", fake.paths[0], "15-character ids are sent in their 18-character form")
	assert.Equal(t, map[string]interface{}{
		"cla_admins": []interface{}{
			map[string]interface{}{"username": "jdoe", "email": "jdoe@example.com", "name": "J Doe"},
			map[string]interface{}{"username": "nomail"},
		},
		"send_invite":       false,
		"source_updated_at": "2026-10-28T11:40:05Z",
	}, fake.payloads[0])

	require.NoError(t, client.PutB2BOrgCLAAdmins(context.Background(), "0014100000Te0G7AAJ", nil, at))
	assert.Equal(t, []interface{}{}, fake.payloads[1]["cla_admins"], "a nil list clears the managers instead of sending null")
	assert.Equal(t, 1, fake.tokenCalls, "the token is cached across calls")

	assert.ErrorIs(t, client.PutB2BOrgCLAAdmins(context.Background(), "lf-not-an-sfid", nil, at), ErrInvalidSFID)
	assert.Len(t, fake.payloads, 2, "malformed ids never reach the service")
}

func TestPutB2BOrgCLAAdminsErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"no b2b_org yet", http.StatusNotFound, `{"message":"organization not found"}`, ErrOrgNotFound},
		{"concurrent update", http.StatusConflict, `{"message":"revision conflict"}`, ErrConflict},
		{"rejected entry", http.StatusBadRequest, `{"message":"username required"}`, ErrInvalidRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newCLAAdminsClient(t, &claAdminsFake{status: tc.status, body: tc.body})
			err := client.PutB2BOrgCLAAdmins(context.Background(), "0014100000Te0G7AAJ", []CLAAdmin{{Username: "jdoe"}}, time.Now())
			assert.ErrorIs(t, err, tc.want)
			if tc.want == ErrConflict {
				assert.ErrorContains(t, err, "revision conflict")
			}
			if tc.want == ErrInvalidRequest {
				assert.ErrorContains(t, err, "username required")
			}
		})
	}

	t.Run("forbidden caller", func(t *testing.T) {
		client := newCLAAdminsClient(t, &claAdminsFake{status: http.StatusForbidden, body: `{"message":"not a cla_admins caller"}`})
		err := client.PutB2BOrgCLAAdmins(context.Background(), "0014100000Te0G7AAJ", nil, time.Now())
		var authErr *AuthError
		require.ErrorAs(t, err, &authErr)
		assert.Equal(t, http.StatusForbidden, authErr.Status)
		assert.False(t, authErr.Token)
	})

	t.Run("server error names the operation", func(t *testing.T) {
		client := newCLAAdminsClient(t, &claAdminsFake{status: http.StatusBadGateway, body: "upstream"})
		err := client.PutB2BOrgCLAAdmins(context.Background(), "0014100000Te0G7AAJ", nil, time.Now())
		assert.ErrorContains(t, err, "PUT /b2b_orgs/{uid}/settings/cla_admins returned 502")
	})
}

func TestSFID18Exported(t *testing.T) {
	id, ok := SFID18("0014100000Te0G7")
	assert.True(t, ok)
	assert.Equal(t, "0014100000Te0G7AAJ", id)
	_, ok = SFID18("lf-not-an-sfid")
	assert.False(t, ok)
}

// two updates within the same second must stay ordered for member-service, which ignores an older
// source_updated_at
func TestPutB2BOrgCLAAdminsKeepsSubsecondOrder(t *testing.T) {
	fake := &claAdminsFake{status: http.StatusOK, body: `{}`}
	client := newCLAAdminsClient(t, fake)
	first := time.Date(2026, 10, 28, 12, 40, 5, 150_000_000, time.UTC)
	second := first.Add(20 * time.Millisecond)
	require.NoError(t, client.PutB2BOrgCLAAdmins(context.Background(), "0014100000Te0G7AAJ", nil, first))
	require.NoError(t, client.PutB2BOrgCLAAdmins(context.Background(), "0014100000Te0G7AAJ", nil, second))
	require.Len(t, fake.payloads, 2)
	sentAt := func(i int) string {
		v, ok := fake.payloads[i]["source_updated_at"].(string)
		require.True(t, ok)
		return v
	}
	assert.Equal(t, "2026-10-28T12:40:05.15Z", sentAt(0))
	assert.Equal(t, "2026-10-28T12:40:05.17Z", sentAt(1))
	t1, err := time.Parse(time.RFC3339Nano, sentAt(0))
	require.NoError(t, err)
	t2, err := time.Parse(time.RFC3339Nano, sentAt(1))
	require.NoError(t, err)
	assert.True(t, t2.After(t1), "same-second updates stay ordered on the wire")
}
