// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// membersServer serves GET /orgs/acme/members page by page from pages (JSON bodies), linking
// each page to the next one, and records the page numbers requested. A page listed in failing
// answers with a 502 instead.
func membersServer(t *testing.T, pages []string, failing map[int]bool) (*httptest.Server, *[]int) {
	t.Helper()
	requested := make([]int, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/orgs/acme/members", r.URL.Path)
		assert.Equal(t, "100", r.URL.Query().Get("per_page"), "members must be requested by the largest page")
		page := 1
		if raw := r.URL.Query().Get("page"); raw != "" {
			_, err := fmt.Sscanf(raw, "%d", &page)
			require.NoError(t, err)
		}
		requested = append(requested, page)
		w.Header().Set("Content-Type", "application/json")
		if failing[page] {
			w.WriteHeader(http.StatusBadGateway)
			_, err := io.WriteString(w, `{"message":"bad gateway"}`)
			assert.NoError(t, err)
			return
		}
		require.LessOrEqual(t, page, len(pages), "unexpected page requested")
		if page < len(pages) {
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/orgs/acme/members?per_page=100&page=%d>; rel="next", <http://%s/orgs/acme/members?per_page=100&page=%d>; rel="last"`,
				r.Host, page+1, r.Host, len(pages)))
		}
		_, err := io.WriteString(w, pages[page-1])
		assert.NoError(t, err)
	}))
	return server, &requested
}

func TestListOrganizationMembersWalksEveryPage(t *testing.T) {
	server, requested := membersServer(t, []string{
		`[{"login":"alice","id":1},{"login":"bob","id":2}]`,
		`[{"login":"carol","id":3},{"id":4}]`,
		`[{"login":"dave","id":5}]`,
	}, nil)
	defer server.Close()

	members, err := listOrganizationMembers(context.Background(), newTestGithubClient(t, server), "acme")
	require.NoError(t, err)
	// the member without a login is skipped, everybody past the first page is kept
	assert.Equal(t, []string{"alice", "bob", "carol", "dave"}, members)
	assert.Equal(t, []int{1, 2, 3}, *requested)
}

func TestListOrganizationMembersSinglePage(t *testing.T) {
	server, requested := membersServer(t, []string{`[{"login":"alice","id":1}]`}, nil)
	defer server.Close()

	members, err := listOrganizationMembers(context.Background(), newTestGithubClient(t, server), "acme")
	require.NoError(t, err)
	assert.Equal(t, []string{"alice"}, members)
	assert.Equal(t, []int{1}, *requested)
}

func TestListOrganizationMembersLaterPageFailureIsNotAPartialResult(t *testing.T) {
	server, requested := membersServer(t, []string{
		`[{"login":"alice","id":1}]`,
		`[{"login":"bob","id":2}]`,
	}, map[int]bool{2: true})
	defer server.Close()

	members, err := listOrganizationMembers(context.Background(), newTestGithubClient(t, server), "acme")
	require.Error(t, err)
	assert.Nil(t, members, "a failed page must not surface the members read so far as the complete set")
	assert.Contains(t, err.Error(), "acme")
	assert.Contains(t, err.Error(), "page 2")
	assert.Equal(t, []int{1, 2}, *requested)
}

func TestListOrganizationMembersTransportFailure(t *testing.T) {
	server, _ := membersServer(t, []string{`[]`}, nil)
	client := newTestGithubClient(t, server)
	// a connection failure yields no response at all - it must be reported, not dereferenced
	server.Close()

	members, err := listOrganizationMembers(context.Background(), client, "acme")
	require.Error(t, err)
	assert.Nil(t, members)
	assert.True(t, strings.Contains(err.Error(), "page 1"), err.Error())
}
