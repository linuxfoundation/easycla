// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

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
)

// fakeGroupMembersAPI serves /groups/42/members page by page like GitLab does (X-Next-Page), optionally
// failing one page
type fakeGroupMembersAPI struct {
	mu       sync.Mutex
	pages    map[string][]map[string]interface{}
	failPage string
	onPage   func(page string)
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
	if f.onPage != nil {
		f.onPage(page)
	}
	members, ok := f.pages[page]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, more := f.pages[next(page)]; more {
		w.Header().Set("X-Next-Page", next(page))
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(members); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func next(page string) string {
	switch page {
	case "1":
		return "2"
	case "2":
		return "3"
	}
	return ""
}

func member(id int, username string) map[string]interface{} {
	return map[string]interface{}{"id": id, "username": username, "name": username}
}

func membersClient(t *testing.T, api *fakeGroupMembersAPI) *goGitLab.Client {
	t.Helper()
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	client, err := goGitLab.NewOAuthClient("token", goGitLab.WithBaseURL(server.URL+"/api/v4"))
	require.NoError(t, err)
	return client
}

func TestListAllDirectGroupMembersFetchesEveryPage(t *testing.T) {
	api := &fakeGroupMembersAPI{pages: map[string][]map[string]interface{}{
		"1": {member(1, "alice"), member(2, "bob")},
		"2": {member(3, "carol")},
	}}
	members, err := ListAllDirectGroupMembers(context.Background(), membersClient(t, api), 42)
	require.NoError(t, err)
	usernames := make([]string, 0, len(members))
	for _, m := range members {
		usernames = append(usernames, m.Username)
	}
	assert.Equal(t, []string{"alice", "bob", "carol"}, usernames)
	var memberRequests []string
	for _, r := range api.requests {
		if strings.Contains(r, "/groups/42/members") {
			memberRequests = append(memberRequests, r)
		}
	}
	require.Len(t, memberRequests, 2)
	assert.Contains(t, memberRequests[0], "per_page=100")
	assert.Contains(t, memberRequests[0], "page=1")
	assert.Contains(t, memberRequests[1], "page=2")
}

func TestListAllDirectGroupMembersFailsWithoutPartialResult(t *testing.T) {
	api := &fakeGroupMembersAPI{pages: map[string][]map[string]interface{}{
		"1": {member(1, "alice")},
		"2": {member(2, "bob")},
	}, failPage: "2"}
	members, err := ListAllDirectGroupMembers(context.Background(), membersClient(t, api), 42)
	require.Error(t, err)
	assert.Nil(t, members, "no first-page-only answer when a later page fails")
}

func TestListAllDirectGroupMembersStopsWhenTheContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api := &fakeGroupMembersAPI{pages: map[string][]map[string]interface{}{
		"1": {member(1, "alice")},
		"2": {member(2, "bob")},
	}}
	api.onPage = func(page string) {
		if page == "1" {
			cancel()
		}
	}
	members, err := ListAllDirectGroupMembers(ctx, membersClient(t, api), 42)
	require.Error(t, err)
	assert.Nil(t, members, "no partial roster after cancellation")
	api.mu.Lock()
	defer api.mu.Unlock()
	for _, r := range api.requests {
		assert.NotContains(t, r, "page=2", "pagination stops once the context is canceled")
	}
}
