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
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
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

func newTestGithubClient(t *testing.T, server *httptest.Server) *gh.Client {
	t.Helper()
	client := gh.NewClient(server.Client())
	baseURL, err := url.Parse(server.URL + "/")
	assert.NoError(t, err)
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
			_, err := io.WriteString(w, `{"message":"bad gateway"}`)
			assert.NoError(t, err)
			return
		}
		_, err := io.WriteString(w, `{
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
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := newTestGithubClient(t, srv)
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
			_, err := io.WriteString(w, `{
			  "number": 7,
			  "base": { "sha": "base" },
			  "head": { "sha": "head" }
			}`)
			assert.NoError(t, err)
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
				_, err := io.WriteString(w, `{
				  "commits": [{
				    "sha": "sha1",
				    "commit": { "message": "msg1", "author": { "name": "name1", "email": "e1@example.com" } },
				    "author": { "id": 1, "login": "u1" }
				  }]
				}`)
				assert.NoError(t, err)
				return

			case "2":
				time.Sleep(50 * time.Millisecond)
				_, err := io.WriteString(w, `{
				  "commits": [{
				    "sha": "sha2",
				    "commit": { "message": "msg2", "author": { "name": "name2", "email": "e2@example.com" } },
				    "author": { "id": 2, "login": "u2" }
				  }]
				}`)
				assert.NoError(t, err)
				return

			case "3":
				_, err := io.WriteString(w, `{
				  "commits": [{
				    "sha": "sha3",
				    "commit": { "message": "msg3", "author": { "name": "name3", "email": "e3@example.com" } },
				    "author": { "id": 3, "login": "u3" }
				  }]
				}`)
				assert.NoError(t, err)
				return
			}
		}

		t.Fatalf("unexpected request: %s?%s", r.URL.Path, r.URL.RawQuery)
	}))
	defer srv.Close()
	serverURL = srv.URL

	client := newTestGithubClient(t, srv)
	commits, err := ListPullRequestCommitsCompare(context.Background(), client, "o", "r", 7)
	assert.NoError(t, err)
	if assert.Len(t, commits, 3) {
		assert.Equal(t, "sha1", commits[0].GetSHA())
		assert.Equal(t, "sha2", commits[1].GetSHA())
		assert.Equal(t, "sha3", commits[2].GetSHA())
	}
}

// TestUpdateCacheAfterSignatureInvalidatesUnknownEmailKeys verifies that a
// pre-signature negative cache entry keyed on a commit-email shape that is NOT
// in the user record (e.g. the GitHub noreply form emitted when email privacy
// is on) is wiped by UpdateCacheAfterSignature, while the user's known emails
// are pre-populated as authorized positives.
func TestUpdateCacheAfterSignatureInvalidatesUnknownEmailKeys(t *testing.T) {
	ModelProjectUserCache.Clear()
	ModelUserCache.Clear()
	t.Cleanup(func() {
		ModelProjectUserCache.Clear()
		ModelUserCache.Clear()
	})

	const (
		projectID    = "01af041c-0000-0000-0000-000000000000"
		githubID     = "196905385"
		githubLogin  = "lukaszgryglicki2"
		noreplyEmail = "196905385+lukaszgryglicki2@users.noreply.github.com"
		realEmail    = "lukaszgryglicki2@proton.me"
		anotherEmail = "lg2@elsewhere.example"
	)

	// Pre-populate: pre-signature webhook stored a negative entry on the
	// noreply form, plus an unrelated negative on a different email.
	staleNoreply := ProjectUserKey(projectID, githubID, githubLogin, noreplyEmail)
	ModelProjectUserCache.SetWithTTL(staleNoreply, nil, false, false, NegativeCacheTTL)
	staleOther := ProjectUserKey(projectID, githubID, githubLogin, anotherEmail)
	ModelProjectUserCache.SetWithTTL(staleOther, nil, false, false, NegativeCacheTTL)

	staleNoreplyUser := UserKey(githubID, githubLogin, noreplyEmail)
	ModelUserCache.SetWithTTL(staleNoreplyUser, nil, NegativeCacheTTL)

	// An unrelated entry under a different login must NOT be touched.
	otherLoginKey := ProjectUserKey(projectID, "999", "someoneelse", "x@y.example")
	ModelProjectUserCache.SetWithTTL(otherLoginKey, nil, false, false, NegativeCacheTTL)

	user := &models.User{
		GithubID:       githubID,
		GithubUsername: githubLogin,
		Emails:         []string{realEmail},
		CompanyID:      "f7c7ac9c-1111-2222-3333-444444444444",
	}

	err := UpdateCacheAfterSignature(context.Background(), user, projectID)
	assert.NoError(t, err)

	// All stale entries for this user (regardless of email shape) must be
	// gone from the project cache. They were not in the user.Emails set, so
	// the previous code would have left them behind.
	_, _, _, ok := ModelProjectUserCache.Get(staleNoreply)
	assert.False(t, ok, "noreply project-cache entry must be invalidated")
	_, _, _, ok = ModelProjectUserCache.Get(staleOther)
	assert.False(t, ok, "unrelated-email project-cache entry must be invalidated")

	_, ok = ModelUserCache.Get(staleNoreplyUser)
	assert.False(t, ok, "noreply user-cache entry must be invalidated")

	// Unrelated user under a different login must survive.
	_, _, _, ok = ModelProjectUserCache.Get(otherLoginKey)
	assert.True(t, ok, "unrelated user's cache entry must NOT be invalidated")

	// The user's known email is pre-populated as an authorized positive.
	realKey := ProjectUserKey(projectID, githubID, githubLogin, realEmail)
	cachedUser, signed, affiliated, ok := ModelProjectUserCache.Get(realKey)
	assert.True(t, ok, "real-email project-cache entry must be set")
	assert.NotNil(t, cachedUser)
	assert.True(t, signed, "real-email entry must be marked signed")
	assert.True(t, affiliated, "user has CompanyID, must be marked affiliated")
}

// TestInvalidateByProjectScopesToProject verifies that InvalidateByProject
// drops every entry for the given project regardless of user, but does not
// touch entries for other projects. Used after UpdateApprovalList because
// approval-list mutations may flip authorization in either direction for
// any user who has cached state under that project.
func TestInvalidateByProjectScopesToProject(t *testing.T) {
	ModelProjectUserCache.Clear()
	t.Cleanup(func() { ModelProjectUserCache.Clear() })

	const (
		targetProj = "01af041c-0000-0000-0000-000000000000"
		otherProj  = "ffffffff-1111-2222-3333-444444444444"
	)

	a := ProjectUserKey(targetProj, "1", "userA", "a@example.com")
	b := ProjectUserKey(targetProj, "2", "userB", "b@example.com")
	c := ProjectUserKey(otherProj, "3", "userC", "c@example.com")
	ModelProjectUserCache.SetWithTTL(a, nil, false, false, NegativeCacheTTL)
	ModelProjectUserCache.SetWithTTL(b, nil, true, true, NegativeCacheTTL)
	ModelProjectUserCache.SetWithTTL(c, nil, true, true, NegativeCacheTTL)

	n := ModelProjectUserCache.InvalidateByProject(targetProj)
	assert.Equal(t, 2, n, "must invalidate exactly the target project's entries")

	_, _, _, ok := ModelProjectUserCache.Get(a)
	assert.False(t, ok, "target-project entry A must be gone")
	_, _, _, ok = ModelProjectUserCache.Get(b)
	assert.False(t, ok, "target-project entry B must be gone")
	_, _, _, ok = ModelProjectUserCache.Get(c)
	assert.True(t, ok, "other-project entry C must NOT be touched")
}

// TestInvalidateByUserNormalizesLoginCase verifies that InvalidateByUser
// lowercases the login internally and matches entries regardless of the
// caller's casing. UserKey/ProjectUserKey lowercase the login on insert,
// so callers passing a mixed-case login must still hit those entries.
func TestInvalidateByUserNormalizesLoginCase(t *testing.T) {
	ModelProjectUserCache.Clear()
	ModelUserCache.Clear()
	t.Cleanup(func() {
		ModelProjectUserCache.Clear()
		ModelUserCache.Clear()
	})

	const (
		projectID = "01af041c-0000-0000-0000-000000000000"
		githubID  = "12345"
		mixedCase = "MixedCaseLogin"
		email     = "u@example.com"
	)

	// Insert via canonical keys (login is lowercased by the *Key helpers).
	pKey := ProjectUserKey(projectID, githubID, mixedCase, email)
	ModelProjectUserCache.SetWithTTL(pKey, nil, false, false, NegativeCacheTTL)
	uKey := UserKey(githubID, mixedCase, email)
	ModelUserCache.SetWithTTL(uKey, nil, NegativeCacheTTL)

	// Caller passes the original mixed-case login (the common mistake).
	pn := ModelProjectUserCache.InvalidateByUser(projectID, githubID, mixedCase)
	un := ModelUserCache.InvalidateByUser(githubID, mixedCase)

	assert.Equal(t, 1, pn, "project cache entry must be invalidated despite mixed-case login")
	assert.Equal(t, 1, un, "user cache entry must be invalidated despite mixed-case login")

	_, _, _, ok := ModelProjectUserCache.Get(pKey)
	assert.False(t, ok, "project cache entry must be gone")
	_, ok = ModelUserCache.Get(uKey)
	assert.False(t, ok, "user cache entry must be gone")
}
