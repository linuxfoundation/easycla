// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	v1Repositories "github.com/linuxfoundation/easycla/cla-backend-go/repositories"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// POST /v4/project/{projectSFID}/github/repositories only uses organizations the project may list
// (lfx-self-serve-ops#982, #747)
func TestOrganizationEntitled(t *testing.T) {
	mappings := []*projects_cla_groups.ProjectClaGroup{
		{ProjectSFID: "project-a", FoundationSFID: "foundation-x"},
		{ProjectSFID: "project-c", FoundationSFID: "foundation-x"},
	}
	org := func(projectSFID, parentSFID string) *v1Models.GithubOrganizations {
		return &v1Models.GithubOrganizations{List: []*v1Models.GithubOrganization{{OrganizationName: "org", ProjectSFID: projectSFID, OrganizationSfid: parentSFID}}}
	}
	cases := []struct {
		name     string
		org      *v1Models.GithubOrganizations
		project  string
		mappings []*projects_cla_groups.ProjectClaGroup
		want     bool
	}{
		{name: "registered by the project", org: org("project-a", ""), project: "project-a", mappings: mappings, want: true},
		{name: "registered by the project with another parent", org: org("project-a", "foundation-y"), project: "project-a", mappings: mappings, want: true},
		{name: "registered under the project's foundation", org: org("", "foundation-x"), project: "project-a", mappings: mappings, want: true},
		{name: "foundation addressed directly", org: org("", "foundation-x"), project: "foundation-x", mappings: mappings, want: true},
		{name: "registered under the project itself as parent", org: org("", "project-a"), project: "project-a", want: true},
		{name: "another project's organization", org: org("project-b", ""), project: "project-a", mappings: mappings, want: false},
		{name: "another foundation's organization", org: org("project-b", "foundation-y"), project: "project-a", mappings: mappings, want: false},
		{name: "foundation of a mapping the project is not part of", org: org("", "foundation-y"), project: "project-a", mappings: []*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "project-z", FoundationSFID: "foundation-y"}}, want: false},
		{name: "unknown organization", org: &v1Models.GithubOrganizations{}, project: "project-a", mappings: mappings, want: false},
		{name: "nil", project: "project-a", mappings: mappings, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, organizationEntitled(tc.org, tc.project, tc.mappings))
		})
	}
}

// the v4 add-repositories path re-uses the v1 enrollment predicates before re-enabling a disabled record found
// by name and before GitHubAddRepository may re-bind a record found by GitHub id (lfx-self-serve-ops#982, #747)
func TestV4EnrollmentGuardsReuseTheV1Predicates(t *testing.T) {
	bound := &v1Models.GithubRepository{RepositoryName: "org/repo", RepositoryProjectSfid: "project-b"}
	legacy := &v1Models.GithubRepository{RepositoryName: "org/repo", RepositorySfdcID: "project-b"}
	unbound := &v1Models.GithubRepository{RepositoryName: "org/repo"}
	own := &v1Models.GithubRepository{RepositoryName: "org/repo", RepositoryProjectSfid: "project-a"}

	assert.Error(t, v1Repositories.RepositoryBoundToOtherProject(bound, "project-a"), "another project's disabled record is not re-enabled")
	assert.Error(t, v1Repositories.RepositoryBoundToOtherProject(legacy, "project-a"), "the legacy sfdc binding counts when project_sfid is empty")
	assert.NoError(t, v1Repositories.RepositoryBoundToOtherProject(unbound, "project-a"), "an unbound record stays reusable")
	assert.NoError(t, v1Repositories.RepositoryBoundToOtherProject(own, "project-a"), "the owning project re-enrolls")

	assert.NoError(t, v1Repositories.ExternalIDRecordReusable(nil, &utils.GitHubRepositoryNotFound{}, "123", "project-a"), "no record by GitHub id - create")
	assert.Error(t, v1Repositories.ExternalIDRecordReusable(bound, nil, "123", "project-a"), "another project's record by GitHub id is not re-bound")
	assert.NoError(t, v1Repositories.ExternalIDRecordReusable(own, nil, "123", "project-a"))
	assert.Error(t, v1Repositories.ExternalIDRecordReusable(nil, nil, "123", "project-a"), "an unreadable record is refused")
	lookupErr := errors.New("dynamo down")
	assert.ErrorIs(t, v1Repositories.ExternalIDRecordReusable(nil, lookupErr, "123", "project-a"), lookupErr)
}
