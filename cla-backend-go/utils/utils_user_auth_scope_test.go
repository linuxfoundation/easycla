// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/stretchr/testify/assert"
)

// a project|organization scope reaches a nested project only for its own organization (lfx-self-serve-ops#382)
func TestIsUserAuthorizedForProjectOrganizationTreeEnforcesTheOrganization(t *testing.T) {
	ctx := context.Background()
	foundationManagerA := scopedUser(auth.Scope{Type: auth.ProjectOrganization, ID: "foundation-sfid|company-a", Related: []string{"proj-sfid", "other-sfid"}})
	admin := &auth.User{UserName: "admin-user", ACL: auth.ACL{Allowed: true, Admin: true}}
	cases := []struct {
		name    string
		user    *auth.User
		company string
		admin   bool
		want    bool
	}{
		{"nested project of the same company", foundationManagerA, "company-a", ALLOW_ADMIN_SCOPE, true},
		{"nested project of another company", foundationManagerA, "company-b", ALLOW_ADMIN_SCOPE, false},
		{"exact project|organization scope", scopedUser(auth.Scope{Type: auth.ProjectOrganization, ID: "proj-sfid|company-b"}), "company-b", ALLOW_ADMIN_SCOPE, true},
		{"exact scope of another company", scopedUser(auth.Scope{Type: auth.ProjectOrganization, ID: "proj-sfid|company-a"}), "company-b", ALLOW_ADMIN_SCOPE, false},
		{"scope without an organization half", scopedUser(auth.Scope{Type: auth.ProjectOrganization, ID: "proj-sfid", Related: []string{"proj-sfid"}}), "company-b", ALLOW_ADMIN_SCOPE, false},
		{"plain project scope never qualifies", scopedUser(auth.Scope{Type: auth.Project, ID: "foundation-sfid", Related: []string{"proj-sfid"}}), "company-b", ALLOW_ADMIN_SCOPE, false},
		{"wildcard project|organization scope", scopedUser(auth.Scope{Type: auth.ProjectOrganization, ID: "*"}), "company-b", ALLOW_ADMIN_SCOPE, true},
		{"admin when allowed", admin, "company-b", ALLOW_ADMIN_SCOPE, true},
		{"admin when disallowed", admin, "company-b", DISALLOW_ADMIN_SCOPE, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsUserAuthorizedForProjectOrganizationTree(ctx, tc.user, "proj-sfid", tc.company, tc.admin))
		})
	}
}

func TestIsUserAuthorizedForAnyProjectOrganizationEnforcesTheOrganization(t *testing.T) {
	ctx := context.Background()
	projects := []string{"proj-sfid", "other-sfid"}
	foundationManagerA := scopedUser(auth.Scope{Type: auth.ProjectOrganization, ID: "foundation-sfid|company-a", Related: []string{"other-sfid"}})
	assert.True(t, IsUserAuthorizedForAnyProjectOrganization(ctx, foundationManagerA, projects, "company-a", ALLOW_ADMIN_SCOPE))
	assert.False(t, IsUserAuthorizedForAnyProjectOrganization(ctx, foundationManagerA, projects, "company-b", ALLOW_ADMIN_SCOPE))
	assert.True(t, IsUserAuthorizedForAnyProjectOrganization(ctx, scopedUser(auth.Scope{Type: auth.ProjectOrganization, ID: "other-sfid|company-b"}), projects, "company-b", DISALLOW_ADMIN_SCOPE))
}

// the organization-only branch of the company-data reads admits plain organization scopes, never project|organization ones (lfx-self-serve-ops#382)
func TestIsUserAuthorizedForOrganizationOnly(t *testing.T) {
	ctx := context.Background()
	admin := &auth.User{UserName: "admin-user", ACL: auth.ACL{Allowed: true, Admin: true}}
	cases := []struct {
		name  string
		user  *auth.User
		admin bool
		want  bool
	}{
		{"plain organization scope", scopedUser(auth.Scope{Type: auth.Organization, ID: "company-a"}), ALLOW_ADMIN_SCOPE, true},
		{"wildcard organization scope", scopedUser(auth.Scope{Type: auth.Organization, ID: "*"}), ALLOW_ADMIN_SCOPE, true},
		{"another organization", scopedUser(auth.Scope{Type: auth.Organization, ID: "company-b"}), ALLOW_ADMIN_SCOPE, false},
		{"project|organization scope of the same company never qualifies", scopedUser(auth.Scope{Type: auth.ProjectOrganization, ID: "proj-sfid|company-a"}), ALLOW_ADMIN_SCOPE, false},
		{"admin when allowed", admin, ALLOW_ADMIN_SCOPE, true},
		{"admin when disallowed", admin, DISALLOW_ADMIN_SCOPE, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsUserAuthorizedForOrganizationOnly(ctx, tc.user, "company-a", tc.admin))
		})
	}
}
