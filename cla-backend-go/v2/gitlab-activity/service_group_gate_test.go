// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_activity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goGitLab "github.com/xanzy/go-gitlab"

	v2Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	gitlab_api "github.com/linuxfoundation/easycla/cla-backend-go/gitlab_api"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/common"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/gitlab_organizations"
)

// fakeGroupMembersAPI serves /groups/42/members in two pages (X-Next-Page), optionally failing one page
type fakeGroupMembersAPI struct {
	mu       sync.Mutex
	pages    map[string][]map[string]interface{}
	failPage string
	requests []string
}

func (f *fakeGroupMembersAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.URL.RequestURI())
	if r.URL.Path != "/api/v4/groups/42/members" {
		http.NotFound(w, r)
		return
	}
	page := r.URL.Query().Get("page")
	if page == "" {
		page = "1"
	}
	if page == f.failPage {
		http.Error(w, `{"message":"403 Forbidden"}`, http.StatusForbidden)
		return
	}
	members, ok := f.pages[page]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, more := f.pages["2"]; more && page == "1" {
		w.Header().Set("X-Next-Page", "2")
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(members); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (f *fakeGroupMembersAPI) memberRequests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var requests []string
	for _, r := range f.requests {
		if strings.Contains(r, "/groups/42/members") {
			requests = append(requests, r)
		}
	}
	return requests
}

func member(id int, username string) map[string]interface{} {
	return map[string]interface{}{"id": id, "username": username, "name": username}
}

// fakeGitLabOrgService resolves every URL to the one registered group; no other method is reached
type fakeGitLabOrgService struct {
	gitlab_organizations.ServiceInterface
	org *v2Models.GitlabOrganization
}

func (f *fakeGitLabOrgService) GetGitLabOrganizationByURL(context.Context, string) (*v2Models.GitlabOrganization, error) {
	return f.org, nil
}

func (f *fakeGitLabOrgService) RefreshGitLabOrganizationAuth(context.Context, *common.GitLabOrganization) (*string, error) {
	authInfo := "oauth-blob"
	return &authInfo, nil
}

func gateService(t *testing.T, api *fakeGroupMembersAPI) *service {
	t.Helper()
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	previous := newGitLabClient
	newGitLabClient = func(string, *gitlab_api.App) (*goGitLab.Client, error) {
		return goGitLab.NewOAuthClient("token", goGitLab.WithBaseURL(server.URL+"/api/v4"))
	}
	t.Cleanup(func() { newGitLabClient = previous })
	org := &v2Models.GitlabOrganization{OrganizationID: "org-1", OrganizationName: "acme", OrganizationExternalID: 42, OrganizationURL: "https://gitlab.com/groups/acme"}
	return &service{gitlabOrgService: &fakeGitLabOrgService{org: org}}
}

// the merge request gate reads the member pages of an approved group until the first match: a member listed
// only on a later page is approved, a non-member is denied after every page, a member found before a failing
// page is approved, and a failing page before any match denies with the error instead of guessing
func TestCheckGitLabGroupApprovalReadsEveryMemberPage(t *testing.T) {
	const groupURL = "https://gitlab.com/groups/acme"
	pages := func() map[string][]map[string]interface{} {
		return map[string][]map[string]interface{}{"1": {member(1, "alice")}, "2": {member(2, "carol")}}
	}
	t.Run("member on the second page is approved", func(t *testing.T) {
		api := &fakeGroupMembersAPI{pages: pages()}
		approved, err := gateService(t, api).checkGitLabGroupApproval(context.Background(), "carol", groupURL)
		require.NoError(t, err)
		assert.True(t, approved)
	})
	t.Run("non-member is denied after every page was read", func(t *testing.T) {
		api := &fakeGroupMembersAPI{pages: pages()}
		approved, err := gateService(t, api).checkGitLabGroupApproval(context.Background(), "mallory", groupURL)
		require.NoError(t, err)
		assert.False(t, approved)
		requests := api.memberRequests()
		require.Len(t, requests, 2)
		assert.Contains(t, requests[1], "page=2")
	})
	t.Run("a member found before a failing page is approved", func(t *testing.T) {
		api := &fakeGroupMembersAPI{pages: pages(), failPage: "2"}
		approved, err := gateService(t, api).checkGitLabGroupApproval(context.Background(), "alice", groupURL)
		require.NoError(t, err)
		assert.True(t, approved)
		assert.Len(t, api.memberRequests(), 1, "pagination stops at the first match")
	})
	t.Run("a failing page before any match denies with the error", func(t *testing.T) {
		api := &fakeGroupMembersAPI{pages: pages(), failPage: "2"}
		approved, err := gateService(t, api).checkGitLabGroupApproval(context.Background(), "carol", groupURL)
		require.Error(t, err)
		assert.False(t, approved)
	})
}
