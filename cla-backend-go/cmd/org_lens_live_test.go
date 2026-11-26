// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgLensLiveCheckNeedsOnlyTheLensBase(t *testing.T) {
	const sfid = "0014100000Te0G7AAJ"
	lookups := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := ""
		switch r.URL.Path {
		case "/oauth/token":
			body = `{"access_token":"member-token","token_type":"Bearer","expires_in":3600}`
		case "/b2b_orgs/" + sfid:
			lookups++
			body = `{"uid":"` + sfid + `","name":"Example Org"}`
		default:
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	cfg := config.Config{
		OrgLensURL:    "https://app.example.org",
		MemberService: config.MemberService{BaseURL: server.URL, Audience: "member-audience"},
		Auth0Platform: config.Auth0Platform{URL: server.URL + "/oauth/token", ClientID: "test-client", ClientSecret: "test-secret"},
	}
	members, err := newMemberServiceClient(cfg)
	require.NoError(t, err)
	require.Nil(t, members, "both member-service flags are off")

	assert.True(t, newOrgLensLiveCheck(cfg, members)(context.Background(), sfid), "the lens base alone is enough for the live check")
	assert.Equal(t, 1, lookups)

	cfg.OrgLensURL = ""
	assert.False(t, newOrgLensLiveCheck(cfg, members)(context.Background(), sfid), "without a lens base nothing asks member-service")
	assert.Equal(t, 1, lookups)

	cfg.OrgLensURL, cfg.MemberService.BaseURL = "https://app.example.org", ""
	assert.False(t, newOrgLensLiveCheck(cfg, members)(context.Background(), sfid), "member-service not configured: the Corporate Console link stays")

	cfg.MemberService.BaseURL = server.URL
	cfg.MemberService.CreateCompanyEnabled = true
	members, err = newMemberServiceClient(cfg)
	require.NoError(t, err)
	require.NotNil(t, members)
	assert.True(t, newOrgLensLiveCheck(cfg, members)(context.Background(), sfid), "the client of the rollout flags is reused")
}
