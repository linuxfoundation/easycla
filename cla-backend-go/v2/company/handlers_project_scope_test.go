// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_projects_cla_groups "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	"github.com/stretchr/testify/assert"
)

// a cla manager of company A on project P must not pass the project branch for company B (lfx-self-serve-ops#382)
func TestIsUserHaveAccessToCLAProjectOrganizationIgnoresProjectOrganizationScopes(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mapping := &projects_cla_groups.ProjectClaGroup{ProjectSFID: "proj-sfid", ProjectExternalID: "proj-sfid", FoundationSFID: "foundation-sfid", ClaGroupID: "cla-group-1"}
	repo := mock_projects_cla_groups.NewMockRepository(ctrl)
	repo.EXPECT().GetClaGroupIDForProject(gomock.Any(), "proj-sfid").Return(mapping, nil).AnyTimes()
	repo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), "cla-group-1").Return([]*projects_cla_groups.ProjectClaGroup{mapping}, nil).AnyTimes()
	user := func(scopeType, id string) *auth.User {
		return &auth.User{UserName: "scoped-user", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: scopeType, ID: id}}}}
	}
	assert.False(t, isUserHaveAccessToCLAProjectOrganization(ctx, user(auth.ProjectOrganization, "proj-sfid|company-a"), "proj-sfid", "company-b", repo))
	assert.True(t, isUserHaveAccessToCLAProjectOrganization(ctx, user(auth.ProjectOrganization, "proj-sfid|company-b"), "proj-sfid", "company-b", repo))
	assert.True(t, isUserHaveAccessToCLAProjectOrganization(ctx, user(auth.Project, "proj-sfid"), "proj-sfid", "company-b", repo))
	foundationManagerA := &auth.User{UserName: "foundation-manager", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "foundation-sfid|company-a", Related: []string{"proj-sfid"}}}}}
	assert.False(t, isUserHaveAccessToCLAProjectOrganization(ctx, foundationManagerA, "proj-sfid", "company-b", repo))
	assert.True(t, isUserHaveAccessToCLAProjectOrganization(ctx, foundationManagerA, "proj-sfid", "company-a", repo))
	otherMapping := &projects_cla_groups.ProjectClaGroup{ProjectSFID: "other-sfid", ProjectExternalID: "other-sfid", FoundationSFID: "other-foundation", ClaGroupID: "cla-group-2", ClaGroupName: "CLA Group 2"}
	repo.EXPECT().GetClaGroupIDForProject(gomock.Any(), "other-sfid").Return(otherMapping, nil).AnyTimes()
	repo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), "cla-group-2").Return([]*projects_cla_groups.ProjectClaGroup{otherMapping}, nil).AnyTimes()
	assert.False(t, isUserHaveAccessToCLAProjectOrganization(ctx, user(auth.ProjectOrganization, "proj-sfid|company-b"), "other-sfid", "company-b", repo), "a project|organization scope of another CLA group does not pass the organization-only branch (ops#382 item 2)")
	assert.True(t, isUserHaveAccessToCLAProjectOrganization(ctx, user(auth.Organization, "company-b"), "other-sfid", "company-b", repo))
	assert.False(t, isUserHaveAccessToCLAProjectOrganization(ctx, user(auth.Organization, "company-b"), "proj-sfid", "company-a", repo), "an organization scope of another company never passes")
	sibling := &projects_cla_groups.ProjectClaGroup{ProjectSFID: "proj-2", ProjectExternalID: "proj-2", FoundationSFID: "foundation-sfid", ClaGroupID: "cla-group-1", ClaGroupName: "CLA Group 1"}
	repo.EXPECT().GetClaGroupIDForProject(gomock.Any(), "proj-2").Return(sibling, nil).AnyTimes()
	assert.True(t, isUserHaveAccessToCLAProjectOrganization(ctx, user(auth.ProjectOrganization, "proj-sfid|company-b"), "proj-2", "company-b", repo), "a sibling project of the same CLA group is reached through the CLA-group loop")
	assert.True(t, isUserHaveAccessToCLAProjectOrganization(ctx, &auth.User{UserName: "admin", ACL: auth.ACL{Allowed: true, Admin: true}}, "other-sfid", "company-b", repo))
}
