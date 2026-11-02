// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/github_organizations"
	repositoriesmock "github.com/linuxfoundation/easycla/cla-backend-go/repositories/mock"
)

func TestAutoEnableServiceProvider_AutoEnabledForGithubOrg(t *testing.T) {

	externalProjectID := "sfd12343"
	claGroupID := "da04291f-75d1-4e84-8275-9bc008205837"
	enabled := true

	testCases := []struct {
		name              string
		githubOrg         github_organizations.GithubOrganization
		repositoryService func(m *repositoriesmock.MockService)
		errStr            string
	}{
		{
			name:      "fail missing installation id",
			githubOrg: github_organizations.GithubOrganization{},
			errStr:    "missing installation id",
		},
		{
			name: "fail repos fetching error",
			githubOrg: github_organizations.GithubOrganization{
				OrganizationInstallationID: 12354,
				ProjectSFID:                externalProjectID,
			},
			repositoryService: func(m *repositoriesmock.MockService) {
				m.
					EXPECT().
					ListProjectRepositories(gomock.Any(), externalProjectID, &enabled).
					Return(nil, fmt.Errorf("fetch error"))
			},
			errStr: "fetch error",
		},
		{
			name: "fail no repos",
			githubOrg: github_organizations.GithubOrganization{
				OrganizationInstallationID: 12354,
				ProjectSFID:                externalProjectID,
			},
			repositoryService: func(m *repositoriesmock.MockService) {
				m.
					EXPECT().
					ListProjectRepositories(gomock.Any(), externalProjectID, &enabled).
					Return(&models.GithubListRepositories{}, nil)
			},
		},
		{
			name: "fail no cla group found",
			githubOrg: github_organizations.GithubOrganization{
				OrganizationInstallationID: 12354,
				ProjectSFID:                externalProjectID,
			},
			repositoryService: func(m *repositoriesmock.MockService) {
				m.
					EXPECT().
					ListProjectRepositories(gomock.Any(), externalProjectID, &enabled).
					Return(&models.GithubListRepositories{
						List: []*models.GithubRepository{
							{
								RepositoryID:          "d7c1050b-2f32-44ea-bad2-3c8ff980ccd4",
								RepositoryProjectSfid: externalProjectID,
							},
							{
								RepositoryID:          "b42216b4-8f6d-41c0-8cde-7b2acbf0656a",
								RepositoryProjectSfid: externalProjectID,
							},
						},
					}, nil)
			},
			errStr: "can't determine the cla group",
		},
		{
			name: "fail multiple cla groups found",
			githubOrg: github_organizations.GithubOrganization{
				OrganizationInstallationID: 12354,
				ProjectSFID:                externalProjectID,
			},
			repositoryService: func(m *repositoriesmock.MockService) {
				m.
					EXPECT().
					ListProjectRepositories(gomock.Any(), externalProjectID, &enabled).
					Return(&models.GithubListRepositories{
						List: []*models.GithubRepository{
							{
								RepositoryID:          "d7c1050b-2f32-44ea-bad2-3c8ff980ccd4",
								RepositoryClaGroupID:  claGroupID,
								RepositoryProjectSfid: externalProjectID,
							},
							{
								RepositoryID:          "b42216b4-8f6d-41c0-8cde-7b2acbf0656a",
								RepositoryClaGroupID:  "anotherclagroup",
								RepositoryProjectSfid: externalProjectID,
							},
						},
					}, nil)
			},
			errStr: "can't determine main cla group",
		},
		{
			name: "success cla set",
			githubOrg: github_organizations.GithubOrganization{
				OrganizationInstallationID: 12354,
				ProjectSFID:                externalProjectID,
				AutoEnabledClaGroupID:      claGroupID,
			},
			repositoryService: func(m *repositoriesmock.MockService) {
				m.
					EXPECT().
					ListProjectRepositories(gomock.Any(), externalProjectID, &enabled).
					Return(&models.GithubListRepositories{
						List: []*models.GithubRepository{
							{
								RepositoryID:          "d7c1050b-2f32-44ea-bad2-3c8ff980ccd4",
								RepositoryProjectSfid: externalProjectID,
							},
							{
								RepositoryID:          "b42216b4-8f6d-41c0-8cde-7b2acbf0656a",
								RepositoryProjectSfid: externalProjectID,
							},
						},
					}, nil)
				m.
					EXPECT().
					UpdateClaGroupID(gomock.Any(), "d7c1050b-2f32-44ea-bad2-3c8ff980ccd4", claGroupID).
					Return(nil)
				m.
					EXPECT().
					UpdateClaGroupID(gomock.Any(), "b42216b4-8f6d-41c0-8cde-7b2acbf0656a", claGroupID).
					Return(nil)
			},
		},
		{
			name: "success no cla set, no call to update",
			githubOrg: github_organizations.GithubOrganization{
				OrganizationInstallationID: 12354,
				ProjectSFID:                externalProjectID,
			},
			repositoryService: func(m *repositoriesmock.MockService) {
				m.
					EXPECT().
					ListProjectRepositories(gomock.Any(), externalProjectID, &enabled).
					Return(&models.GithubListRepositories{
						List: []*models.GithubRepository{
							{
								RepositoryID:          "d7c1050b-2f32-44ea-bad2-3c8ff980ccd4",
								RepositoryProjectSfid: externalProjectID,
								RepositoryClaGroupID:  claGroupID,
							},
							{
								RepositoryID:          "b42216b4-8f6d-41c0-8cde-7b2acbf0656a",
								RepositoryProjectSfid: externalProjectID,
								RepositoryClaGroupID:  claGroupID,
							},
						},
					}, nil)
			},
		},
		{
			name: "success no cla set determine from repos update",
			githubOrg: github_organizations.GithubOrganization{
				OrganizationInstallationID: 12354,
				ProjectSFID:                externalProjectID,
			},
			repositoryService: func(m *repositoriesmock.MockService) {
				m.
					EXPECT().
					ListProjectRepositories(gomock.Any(), externalProjectID, &enabled).
					Return(&models.GithubListRepositories{
						List: []*models.GithubRepository{
							{
								RepositoryID:          "d7c1050b-2f32-44ea-bad2-3c8ff980ccd4",
								RepositoryProjectSfid: externalProjectID,
							},
							{
								RepositoryID:          "b42216b4-8f6d-41c0-8cde-7b2acbf0656a",
								RepositoryProjectSfid: externalProjectID,
								RepositoryClaGroupID:  claGroupID,
							},
						},
					}, nil)

				m.
					EXPECT().
					UpdateClaGroupID(gomock.Any(), "d7c1050b-2f32-44ea-bad2-3c8ff980ccd4", claGroupID).
					Return(nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			ctrl := gomock.NewController(tt)
			defer ctrl.Finish()

			m := repositoriesmock.NewMockService(ctrl)
			if tc.repositoryService != nil {
				tc.repositoryService(m)
			}

			a := &autoEnableServiceProvider{
				repositoryService: m,
			}
			err := a.AutoEnabledForGithubOrg(logrus.Fields{
				"functionName": "TestAutoEnable",
			}, tc.githubOrg, false)

			if tc.errStr == "" {
				assert.NoError(tt, err)
			} else {
				assert.Contains(tt, err.Error(), tc.errStr)
			}
		})
	}
}

func TestAutoEnableServiceProvider_CreateAutoEnabledRepository(t *testing.T) {

}

// the auto-enable notification is HTML: repository, CLA group and organization names are escaped in the body,
// the subject and the recipients stay plaintext (ops#807)
func TestAutoEnabledRepositoryEmailContentEscapesDynamicText(t *testing.T) {
	claGroup := &models.ClaGroup{ProjectName: `Group <b>X</b> & co`, Version: "v2"}
	managers := []*models.ClaManagerUser{{UserEmail: "manager@example.com"}, {UserEmail: ""}}
	repos := []*models.GithubRepository{
		{RepositoryName: `org/<script>alert(1)</script>`},
		{RepositoryName: "org/repo-two"},
	}
	subject, body, recipients := autoEnabledRepositoryEmailContent(claGroup, `org"><img src=x onerror=alert(1)>`, managers, repos)
	assert.Equal(t, "EasyCLA: Auto-Enable CombinedRepository for CLA Group: Group <b>X</b> & co", subject)
	assert.Equal(t, []string{"manager@example.com"}, recipients)
	items := regexp.MustCompile(`<li>(.*?)</li>`).FindAllStringSubmatch(body, -1)
	if assert.Len(t, items, 2, body) {
		assert.Equal(t, "org/&lt;script&gt;alert(1)&lt;/script&gt;", items[0][1])
		assert.Equal(t, "org/repo-two", items[1][1])
	}
	assert.Contains(t, body, "regarding the CLA Group Group &lt;b&gt;X&lt;/b&gt; &amp; co.")
	assert.Contains(t, body, "added to the org&#34;&gt;&lt;img src=x onerror=alert(1)&gt; GitHub Organization.")
	for _, raw := range []string{"<script>", "<b>X</b>", "<img", `org">`} {
		assert.NotContains(t, body, raw)
	}
}
