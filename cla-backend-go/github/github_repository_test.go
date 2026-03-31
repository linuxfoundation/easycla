// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	gh "github.com/google/go-github/v37/github"
	"github.com/stretchr/testify/assert"
)

func TestGetCommentBodyIncludesCoAuthorRemovalGuidance(t *testing.T) {
	signed := []*UserCommitSummary{{
		SHA: "abc1234xyz-123",
		CommitAuthor: &gh.User{
			ID:    gh.Int64(1234),
			Login: gh.String("login_value"),
			Name:  gh.String("author name"),
			Email: gh.String("foo@bar.com"),
		},
		Affiliated: true,
		Authorized: true,
	}}

	missing := []*UserCommitSummary{{
		SHA: "some_other_sha",
		CommitAuthor: &gh.User{
			ID:    gh.Int64(123456),
			Login: gh.String("login_value2"),
			Name:  gh.String("author name2"),
			Email: gh.String("foo2@bar.com"),
		},
		Affiliated: false,
		Authorized: false,
	}}

	body := getCommentBody("github", "https://foo.com", signed, missing, true)

	assert.Contains(t, body, "One or more co-authors of this pull request were not found")
	assert.Contains(t, body, "Alternatively, if the co-author should not be included, remove the `Co-authored-by:` line from the commit message.")
	assert.NotContains(t, body, "|Co-authored-by:|")
}

func TestGetCommentBodyOmitsCoAuthorRemovalGuidanceWhenNoCoAuthorIsMissing(t *testing.T) {
	missing := []*UserCommitSummary{{
		SHA: "some_other_sha",
		CommitAuthor: &gh.User{
			ID:    gh.Int64(123456),
			Login: gh.String("login_value2"),
			Name:  gh.String("author name2"),
			Email: gh.String("foo2@bar.com"),
		},
		Affiliated: false,
		Authorized: false,
	}}

	body := getCommentBody("github", "https://foo.com", nil, missing, false)

	assert.NotContains(t, body, "One or more co-authors of this pull request were not found")
	assert.NotContains(t, body, "Alternatively, if the co-author should not be included, remove the `Co-authored-by:` line from the commit message.")
}

func newTestGithubClient(server *httptest.Server) *gh.Client {
	client := gh.NewClient(server.Client())
	baseURL, _ := url.Parse(server.URL + "/")
	client.BaseURL = baseURL
	client.UploadURL = baseURL
	return client
}

func TestFetchComparePageRetriesThenSucceeds(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/o/r/compare/base...head", r.URL.Path)
		attempts++
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, `{"message":"bad gateway"}`)
			return
		}
		_, _ = io.WriteString(w, `{
		  "commits": [{
		    "sha": "abc123",
		    "commit": {
		      "message": "hello",
		      "author": {
		        "name": "Commit Name",
		        "email": "commit@example.com"
		      }
		    },
		    "author": {
		      "id": 123,
		      "login": "login",
		      "name": "Profile Name",
		      "email": "profile@example.com"
		    }
		  }]
		}`)
	}))
	defer srv.Close()

	client := newTestGithubClient(srv)
	commits, _, err := fetchComparePage(context.Background(), client, "o", "r", "base", "head", 1, 100, 7)
	assert.NoError(t, err)
	if assert.Len(t, commits, 1) {
		assert.Equal(t, "abc123", commits[0].GetSHA())
		assert.Equal(t, "login", commits[0].GetAuthor().GetLogin())
		assert.Equal(t, "Commit Name", commits[0].GetAuthor().GetName())
		assert.Equal(t, "profile@example.com", commits[0].GetAuthor().GetEmail())
		assert.Equal(t, "hello", commits[0].GetCommit().GetMessage())
	}
	assert.Equal(t, 2, attempts)
}

func TestListPullRequestCommitsComparePreservesPageOrder(t *testing.T) {
	oldLimit := ListCommitsParallelLimit
	ListCommitsParallelLimit = 4
	defer func() { ListCommitsParallelLimit = oldLimit }()

	var serverURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/repos/o/r/pulls/7":
			_, _ = io.WriteString(w, `{
			  "number": 7,
			  "base": { "sha": "base" },
			  "head": { "sha": "head" }
			}`)
			return

		case "/repos/o/r/compare/base...head":
			switch r.URL.Query().Get("page") {
			case "1":
				w.Header().Set(
					"Link",
					fmt.Sprintf(
						`<%s/repos/o/r/compare/base...head?page=2&per_page=100>; rel="next", <%s/repos/o/r/compare/base...head?page=3&per_page=100>; rel="last"`,
						serverURL, serverURL,
					),
				)
				_, _ = io.WriteString(w, `{
				  "commits": [{
				    "sha": "sha1",
				    "commit": { "message": "msg1", "author": { "name": "name1", "email": "e1@example.com" } },
				    "author": { "id": 1, "login": "u1" }
				  }]
				}`)
				return

			case "2":
				time.Sleep(50 * time.Millisecond)
				_, _ = io.WriteString(w, `{
				  "commits": [{
				    "sha": "sha2",
				    "commit": { "message": "msg2", "author": { "name": "name2", "email": "e2@example.com" } },
				    "author": { "id": 2, "login": "u2" }
				  }]
				}`)
				return

			case "3":
				_, _ = io.WriteString(w, `{
				  "commits": [{
				    "sha": "sha3",
				    "commit": { "message": "msg3", "author": { "name": "name3", "email": "e3@example.com" } },
				    "author": { "id": 3, "login": "u3" }
				  }]
				}`)
				return
			}
		}

		t.Fatalf("unexpected request: %s?%s", r.URL.Path, r.URL.RawQuery)
	}))
	defer srv.Close()
	serverURL = srv.URL

	client := newTestGithubClient(srv)
	commits, err := ListPullRequestCommitsCompare(context.Background(), client, "o", "r", 7)
	assert.NoError(t, err)
	if assert.Len(t, commits, 3) {
		assert.Equal(t, "sha1", commits[0].GetSHA())
		assert.Equal(t, "sha2", commits[1].GetSHA())
		assert.Equal(t, "sha3", commits[2].GetSHA())
	}
}
