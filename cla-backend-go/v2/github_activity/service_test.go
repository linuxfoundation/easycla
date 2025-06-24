// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_activity

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	eventsMock "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	githubOrgMock "github.com/linuxfoundation/easycla/cla-backend-go/github_organizations/mock"
	"github.com/linuxfoundation/easycla/cla-backend-go/repositories/mock"
	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v37/github"
	"github.com/stretchr/testify/assert"
)

func TestEventHandlerService_ProcessRepositoryEvent_HandleRepositoryRenamedAction(t *testing.T) {
	repoID := "1f15f478-0659-43f3-bcf1-383052de7616"
	repoName := "org1/repo-name"
	newRepoName := "org1/repo-name-new"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	githubOrganizationRepo := githubOrgMock.NewMockRepositoryInterface(ctrl)
	githubRepo := mock.NewMockRepositoryInterface(ctrl)
	githubRepo.EXPECT().
		GitHubGetRepositoryByGithubID(gomock.Any(), "1", true).
		Return(&models.GithubRepository{
			Enabled:                    true,
			RepositoryExternalID:       1,
			RepositoryID:               repoID,
			RepositoryName:             repoName,
			RepositoryOrganizationName: "org1",
		}, nil)

	githubRepo.EXPECT().
		GitHubUpdateRepository(gomock.Any(), repoID, "", "", &models.GithubRepositoryInput{
			RepositoryName: &newRepoName,
			Note:           "repository was renamed externally",
		}).Return(nil, nil)

	eventsService := eventsMock.NewMockService(ctrl)
	eventsService.EXPECT().
		LogEventWithContext(gomock.Any(), &events.LogEventArgs{
			EventType: events.RepositoryRenamed,
			UserID:    "githubLoginValue",
			ProjectID: "",
			EventData: &events.RepositoryRenamedEventData{
				NewRepositoryName: newRepoName,
				OldRepositoryName: repoName,
			},
		}).Return()

	activityService := newService(githubRepo, githubOrganizationRepo, eventsService, nil, nil, false)
	err := activityService.ProcessRepositoryEvent(&github.RepositoryEvent{
		Action: aws.String("renamed"),
		Repo: &github.Repository{
			ID:   aws.Int64(1),
			Name: &newRepoName,
		},
		Org: nil,
		Sender: &github.User{
			Login: aws.String("githubLoginValue"),
		},
		Installation: nil,
	})

	assert.NoError(t, err)
}

func TestEventHandlerService_ProcessRepositoryEvent_HandleRepositoryTransferredAction(t *testing.T) {
	repoID := "1f15f478-0659-43f3-bcf1-383052de7616"
	repoName := "org1/repo-name"
	oldOrgName := "org1"
	newOrgName := "org2"
	newRepoUrl := "org2/repo-name"

	testCases := []struct {
		name         string
		newGithubOrg *models.GithubOrganization
	}{
		{
			name: "success new org is enabled and and has cla group",
			newGithubOrg: &models.GithubOrganization{
				OrganizationName:      newOrgName,
				AutoEnabled:           true,
				AutoEnabledClaGroupID: "c057ed9a-4235-4acf-80bd-c7b4c235eff9",
			},
		},
		{
			name: "failure new org is disabled and no cla group",
			newGithubOrg: &models.GithubOrganization{
				OrganizationName: newOrgName,
				AutoEnabled:      false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			githubOrganizationRepo := githubOrgMock.NewMockRepositoryInterface(ctrl)
			githubRepo := mock.NewMockRepositoryInterface(ctrl)
			githubRepo.EXPECT().
				GitHubGetRepositoryByGithubID(gomock.Any(), "1", true).
				Return(&models.GithubRepository{
					Enabled:                    true,
					RepositoryExternalID:       1,
					RepositoryID:               repoID,
					RepositoryName:             repoName,
					RepositoryOrganizationName: oldOrgName,
				}, nil)

			// return the old one
			githubOrganizationRepo.EXPECT().
				GetGitHubOrganization(gomock.Any(), oldOrgName).
				Return(&models.GithubOrganization{
					OrganizationName: oldOrgName,
				}, nil)

			// return the new one
			githubOrganizationRepo.EXPECT().
				GetGitHubOrganization(gomock.Any(), newOrgName).
				Return(tc.newGithubOrg, nil)

			eventsService := eventsMock.NewMockService(ctrl)
			if tc.newGithubOrg.AutoEnabled {
				githubRepo.EXPECT().
					GitHubUpdateRepository(gomock.Any(), repoID, gomock.Any(), gomock.Any(), &models.GithubRepositoryInput{
						RepositoryOrganizationName: &newOrgName,
						RepositoryURL:              &newRepoUrl,
						Note:                       fmt.Sprintf("repository was transferred from org : %s to : %s", oldOrgName, newOrgName),
					}).Return(nil, nil)

				eventsService.EXPECT().
					LogEventWithContext(gomock.Any(), &events.LogEventArgs{
						EventType: events.RepositoryTransferred,
						UserID:    "githubLoginValue",
						ProjectID: "",
						EventData: &events.RepositoryTransferredEventData{
							RepositoryName:   repoName,
							OldGithubOrgName: oldOrgName,
							NewGithubOrgName: newOrgName,
						},
					}).Return()
			} else {
				githubRepo.EXPECT().
					GitHubDisableRepository(gomock.Any(), repoID).Return(nil)
				eventsService.EXPECT().
					LogEventWithContext(gomock.Any(), &events.LogEventArgs{
						EventType: events.RepositoryDisabled,
						UserID:    "githubLoginValue",
						ProjectID: "",
						EventData: &events.RepositoryDisabledEventData{
							RepositoryName: repoName,
						},
					}).Return()
			}

			activityService := newService(githubRepo, githubOrganizationRepo, eventsService, nil, nil, false)
			err := activityService.ProcessRepositoryEvent(&github.RepositoryEvent{
				Action: aws.String("transferred"),
				Repo: &github.Repository{
					ID:      aws.Int64(1),
					Name:    &repoName,
					HTMLURL: &newRepoUrl,
				},
				Org: &github.Organization{
					Login: &newOrgName,
				},
				Sender: &github.User{
					Login: aws.String("githubLoginValue"),
				},
				Installation: nil,
			})

			if tc.newGithubOrg.AutoEnabled {
				assert.NoError(tt, err)
			} else {
				assert.Error(tt, err)
			}
		})
	}
}
