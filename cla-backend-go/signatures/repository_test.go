// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
)

func TestInvalidationUpdateExpression(t *testing.T) {
	const now = "2024-05-06T07:08:09.000000+0000"

	names, values, expr := invalidationUpdateExpression("a note", now, &InvalidationMetadata{
		InvalidatedBy: "admin-user",
		Reason:        "compliance",
		Note:          "per legal review",
	})

	assert.Contains(t, expr, "#A = :a")
	assert.Contains(t, expr, "#S = :s")
	assert.Contains(t, expr, "#DI = if_not_exists(#DI, :di)")
	assert.Contains(t, expr, "#IB = if_not_exists(#IB, :ib)", "a re-invalidation must not overwrite the first actor")
	assert.Contains(t, expr, "#IR = if_not_exists(#IR, :ir)", "a re-invalidation must not overwrite the first reason")
	assert.Contains(t, expr, "#IN = if_not_exists(#IN, :in)", "a re-invalidation must not overwrite the first note")
	assert.Contains(t, expr, "#M = :m")

	assert.Equal(t, "invalidated_by", *names["#IB"])
	assert.Equal(t, "invalidation_reason", *names["#IR"])
	assert.Equal(t, "invalidation_note", *names["#IN"])
	assert.Equal(t, "admin-user", *values[":ib"].S)
	assert.Equal(t, "compliance", *values[":ir"].S)
	assert.Equal(t, "per legal review", *values[":in"].S)
	assert.Equal(t, now, *values[":di"].S)
	assert.False(t, *values[":a"].BOOL)
}

func TestInvalidationUpdateExpressionWithoutMetadata(t *testing.T) {
	const now = "2024-05-06T07:08:09.000000+0000"

	for _, metadata := range []*InvalidationMetadata{nil, {}} {
		names, values, expr := invalidationUpdateExpression("a note", now, metadata)

		assert.NotContains(t, expr, "#IB")
		assert.NotContains(t, expr, "#IR")
		assert.NotContains(t, expr, "#IN")
		assert.Contains(t, expr, "#DI = if_not_exists(#DI, :di)")
		assert.NotContains(t, names, "#IB")
		assert.NotContains(t, values, ":ib")
	}
}

func TestEffectiveApprovals(t *testing.T) {
	// removals subtracted, additions appended, add+remove of the same entry removes it
	assert.Equal(t, []string{"a", "c", "d"},
		effectiveApprovals([]string{"a", "b", "c"}, []string{"d", "b", "a"}, []string{"b"}))
	assert.Empty(t, effectiveApprovals([]string{"a"}, nil, []string{"a"}))
	assert.Equal(t, []string{"a"}, effectiveApprovals(nil, []string{"a"}, nil))

	// entries are trimmed like persistence, so a padded add matches the persisted value
	assert.Equal(t, []string{"janegh"}, effectiveApprovals(nil, []string{" janegh "}, nil))

	// duplicate current entries are deduped like persistence
	assert.Equal(t, []string{"a", "b"}, effectiveApprovals([]string{"a", "a", "b"}, nil, nil))

	// removals stay exact-match on raw entries (persistence parity): a padded remove entry
	// removes nothing
	assert.Equal(t, []string{"a"}, effectiveApprovals([]string{"a"}, nil, []string{" a "}))
}

// userStillApproved receives approval lists that already reflect the full pending update
// (built via effectiveApprovals), so removed entries are simply absent
func TestUserStillApproved(t *testing.T) {
	user := &models.User{
		Emails:         []string{"jane@corp.example"},
		GithubUsername: "janegh",
		GitlabUsername: "janegl",
	}

	// GH username removed but still on the email approved list (the #5166 scenario)
	assert.True(t, userStillApproved(user, &ApprovalList{
		EmailApprovals: []string{"jane@corp.example"},
	}))

	// GH username removed, no other coverage
	assert.False(t, userStillApproved(user, &ApprovalList{}))

	// email removed but still covered by the domain approved list
	assert.True(t, userStillApproved(user, &ApprovalList{
		DomainApprovals: []string{"corp.example"},
	}))

	// email removed, remaining GH username approval covers the user
	assert.True(t, userStillApproved(user, &ApprovalList{
		GitHubUsernameApprovals: []string{"janegh"},
	}))

	// GitLab username coverage
	assert.True(t, userStillApproved(user, &ApprovalList{
		GitlabUsernameApprovals: []string{"janegl"},
	}))

	// approvals for other users don't cover this one
	assert.False(t, userStillApproved(user, &ApprovalList{
		GitHubUsernameApprovals: []string{"someoneelse"},
		EmailApprovals:          []string{"other@corp.example"},
	}))

	// cross-criteria removal in one request: email and GH username both removed,
	// effective lists no longer contain either entry, so the user is not covered
	assert.False(t, userStillApproved(user, &ApprovalList{
		EmailApprovals:          effectiveApprovals([]string{"jane@corp.example"}, nil, []string{"jane@corp.example"}),
		GitHubUsernameApprovals: effectiveApprovals([]string{"janegh"}, nil, []string{"janegh"}),
	}))

	// email approval entries match case-insensitively
	assert.True(t, userStillApproved(user, &ApprovalList{
		EmailApprovals: []string{"Jane@Corp.Example"},
	}))

	// domain entries match with the gate's pattern semantics: case-sensitive and untrimmed,
	// so a padded mixed-case entry the gate would not honor grants no coverage here either
	assert.False(t, userStillApproved(user, &ApprovalList{
		DomainApprovals: []string{" Corp.Example "},
	}))

	// usernames fold like the enforcement gate (EqualFold)
	assert.True(t, userStillApproved(user, &ApprovalList{
		GitHubUsernameApprovals: []string{"JaneGH"},
	}))
	assert.True(t, userStillApproved(user, &ApprovalList{
		GitlabUsernameApprovals: []string{"JaneGL"},
	}))

	// wildcard domain entries cover subdomains like the enforcement gate
	assert.True(t, userStillApproved(&models.User{Emails: []string{"jane@dev.example.com"}}, &ApprovalList{
		DomainApprovals: []string{"*.example.com"},
	}))

	// a malformed domain pattern neither panics nor grants coverage
	assert.False(t, userStillApproved(&models.User{Emails: []string{"jane@corp.example"}}, &ApprovalList{
		DomainApprovals: []string{"("},
	}))
}
