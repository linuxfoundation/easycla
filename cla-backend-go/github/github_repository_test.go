// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"testing"

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
