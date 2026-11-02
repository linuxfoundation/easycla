// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/stretchr/testify/assert"
)

func scopedUser(scopes ...auth.Scope) *auth.User {
	return &auth.User{UserName: "scoped-user", ACL: auth.ACL{Allowed: true, Scopes: scopes}}
}

func TestIsUserAuthorizedForProjectOnly(t *testing.T) {
	ctx := context.Background()
	admin := &auth.User{UserName: "admin-user", ACL: auth.ACL{Allowed: true, Admin: true}}
	cases := []struct {
		name  string
		user  *auth.User
		admin bool
		want  bool
	}{
		{"project|organization scope of the same project never qualifies", scopedUser(auth.Scope{Type: auth.ProjectOrganization, ID: "proj-sfid|company-a"}), ALLOW_ADMIN_SCOPE, false},
		{"plain project scope", scopedUser(auth.Scope{Type: auth.Project, ID: "proj-sfid"}), ALLOW_ADMIN_SCOPE, true},
		{"wildcard project scope", scopedUser(auth.Scope{Type: auth.Project, ID: "*"}), ALLOW_ADMIN_SCOPE, true},
		{"another project", scopedUser(auth.Scope{Type: auth.Project, ID: "other-sfid"}), ALLOW_ADMIN_SCOPE, false},
		{"organization scope", scopedUser(auth.Scope{Type: auth.Organization, ID: "company-a"}), ALLOW_ADMIN_SCOPE, false},
		{"admin when allowed", admin, ALLOW_ADMIN_SCOPE, true},
		{"admin when disallowed", admin, DISALLOW_ADMIN_SCOPE, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsUserAuthorizedForProjectOnly(ctx, tc.user, "proj-sfid", tc.admin))
		})
	}
}

func TestIsUserAuthorizedForAnyProjectsOnly(t *testing.T) {
	ctx := context.Background()
	projects := []string{"proj-sfid", "other-sfid"}
	assert.False(t, IsUserAuthorizedForAnyProjectsOnly(ctx, scopedUser(auth.Scope{Type: auth.ProjectOrganization, ID: "proj-sfid|company-a"}), projects, ALLOW_ADMIN_SCOPE))
	assert.False(t, IsUserAuthorizedForAnyProjectsOnly(ctx, scopedUser(auth.Scope{Type: auth.ProjectOrganization, ID: "other-sfid|company-a"}), projects, ALLOW_ADMIN_SCOPE))
	assert.True(t, IsUserAuthorizedForAnyProjectsOnly(ctx, scopedUser(auth.Scope{Type: auth.Project, ID: "other-sfid"}), projects, ALLOW_ADMIN_SCOPE))
	assert.True(t, IsUserAuthorizedForAnyProjectsOnly(ctx, scopedUser(auth.Scope{Type: auth.Project, ID: "foundation-sfid", Related: []string{"proj-sfid"}}), projects, ALLOW_ADMIN_SCOPE))
	assert.True(t, IsUserAuthorizedForAnyProjectsOnly(ctx, &auth.User{UserName: "admin-user", ACL: auth.ACL{Allowed: true, Admin: true}}, projects, ALLOW_ADMIN_SCOPE))
	assert.False(t, IsUserAuthorizedForAnyProjectsOnly(ctx, scopedUser(auth.Scope{Type: auth.Project, ID: "proj-sfid"}), nil, ALLOW_ADMIN_SCOPE))
}
