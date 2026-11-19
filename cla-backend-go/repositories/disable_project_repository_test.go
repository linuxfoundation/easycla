// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	eventsmock "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/github_repositories"
	"github.com/linuxfoundation/easycla/cla-backend-go/repositories/mock"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// DELETE /v3/project/{projectSFID}/github/repositories/{repositoryID} only disables repositories bound to
// that project, as the /v4 route does (lfx-self-serve-ops#748)
func TestDisableProjectRepositoryChecksTheProjectBinding(t *testing.T) {
	cases := []struct {
		name     string
		record   *models.GithubRepository
		disabled bool
	}{
		{name: "bound repository", record: &models.GithubRepository{RepositoryID: "repo-1", RepositoryName: "org/repo", RepositoryProjectSfid: "project-a"}, disabled: true},
		{name: "same owner with a different parent field", record: &models.GithubRepository{RepositoryID: "repo-1", RepositoryProjectSfid: "project-a", RepositorySfdcID: "foundation-y"}, disabled: true},
		{name: "unbound record", record: &models.GithubRepository{RepositoryID: "repo-1"}},
		{name: "legacy sfdc binding only", record: &models.GithubRepository{RepositoryID: "repo-1", RepositorySfdcID: "project-a"}},
		{name: "another project's record", record: &models.GithubRepository{RepositoryID: "repo-1", RepositoryProjectSfid: "project-b"}},
		{name: "unknown record", record: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			repo := mock.NewMockRepositoryInterface(ctrl)
			repo.EXPECT().GitHubGetRepository(gomock.Any(), "repo-1").Return(tc.record, nil)
			if tc.disabled {
				repo.EXPECT().GitHubDisableRepositoryForProject(gomock.Any(), "repo-1", "project-a").Return(nil)
			}
			ghRepo, err := (&service{repo: repo}).DisableProjectRepository(context.Background(), "project-a", "repo-1")
			if tc.disabled {
				require.NoError(t, err)
				assert.Same(t, tc.record, ghRepo)
				return
			}
			var notFound *utils.GitHubRepositoryNotFound
			require.True(t, errors.As(err, &notFound), "expected not found, got %v", err)
			assert.Nil(t, ghRepo)
		})
	}
	t.Run("lookup and write errors are returned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockRepositoryInterface(ctrl)
		repo.EXPECT().GitHubGetRepository(gomock.Any(), "repo-1").Return(nil, errors.New("dynamo down"))
		_, err := (&service{repo: repo}).DisableProjectRepository(context.Background(), "project-a", "repo-1")
		assert.EqualError(t, err, "dynamo down")

		repo.EXPECT().GitHubGetRepository(gomock.Any(), "repo-1").Return(&models.GithubRepository{RepositoryID: "repo-1", RepositoryProjectSfid: "project-a"}, nil)
		repo.EXPECT().GitHubDisableRepositoryForProject(gomock.Any(), "repo-1", "project-a").Return(&utils.GitHubRepositoryNotFound{Message: "rebound meanwhile"})
		ghRepo, err := (&service{repo: repo}).DisableProjectRepository(context.Background(), "project-a", "repo-1")
		var notFound *utils.GitHubRepositoryNotFound
		require.True(t, errors.As(err, &notFound))
		assert.Nil(t, ghRepo)
	})
}

func TestDeleteProjectGithubRepositoryHandler(t *testing.T) {
	params := github_repositories.DeleteProjectGithubRepositoryParams{ProjectSFID: "project-a", RepositoryID: "repo-1"}
	manager := &user.CLAUser{UserID: "user-1", LFUsername: "manager", ProjectIDs: []string{"project-a"}}
	handler := func(t *testing.T, svc Service, eventService events.Service) github_repositories.DeleteProjectGithubRepositoryHandler {
		api := &operations.ClaAPI{}
		Configure(api, svc, eventService)
		require.NotNil(t, api.GithubRepositoriesDeleteProjectGithubRepositoryHandler)
		return api.GithubRepositoriesDeleteProjectGithubRepositoryHandler
	}

	t.Run("the project's repository is disabled and the event names it", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, eventService := mock.NewMockService(ctrl), eventsmock.NewMockService(ctrl)
		svc.EXPECT().DisableProjectRepository(gomock.Any(), "project-a", "repo-1").Return(&models.GithubRepository{RepositoryID: "repo-1", RepositoryName: "org/repo", RepositoryProjectSfid: "project-a"}, nil)
		eventService.EXPECT().LogEventWithContext(gomock.Any(), gomock.Any()).Do(func(_ context.Context, args *events.LogEventArgs) {
			assert.Equal(t, events.RepositoryDisabled, args.EventType)
			assert.Equal(t, "project-a", args.ProjectSFID)
			assert.Equal(t, &events.RepositoryDisabledEventData{RepositoryName: "org/repo"}, args.EventData)
		})
		assert.IsType(t, &github_repositories.DeleteProjectGithubRepositoryNoContent{}, handler(t, svc, eventService).Handle(params, manager))
	})
	t.Run("another project's repository answers 404 and nothing is logged", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, eventService := mock.NewMockService(ctrl), eventsmock.NewMockService(ctrl)
		svc.EXPECT().DisableProjectRepository(gomock.Any(), "project-a", "repo-1").Return(nil, &utils.GitHubRepositoryNotFound{Message: "repository repo-1 does not belong to project project-a"})
		assert.IsType(t, &github_repositories.DeleteProjectGithubRepositoryNotFound{}, handler(t, svc, eventService).Handle(params, manager))
	})
	t.Run("a failed disable answers 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, eventService := mock.NewMockService(ctrl), eventsmock.NewMockService(ctrl)
		svc.EXPECT().DisableProjectRepository(gomock.Any(), "project-a", "repo-1").Return(nil, errors.New("dynamo down"))
		assert.IsType(t, &github_repositories.DeleteProjectGithubRepositoryBadRequest{}, handler(t, svc, eventService).Handle(params, manager))
	})
	t.Run("a user without the project answers 403 before any lookup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, eventService := mock.NewMockService(ctrl), eventsmock.NewMockService(ctrl)
		other := &user.CLAUser{UserID: "user-2", LFUsername: "other", ProjectIDs: []string{"project-b"}}
		assert.IsType(t, &github_repositories.DeleteProjectGithubRepositoryForbidden{}, handler(t, svc, eventService).Handle(params, other))
	})
}
