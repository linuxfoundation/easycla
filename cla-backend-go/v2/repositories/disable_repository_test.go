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

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	mock "github.com/linuxfoundation/easycla/cla-backend-go/repositories/mock"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// DELETE /v4/project/{projectSFID}/github/repositories/{repositoryID} only disables repositories bound to
// that project (lfx-self-serve-ops#748)
func TestGitHubDisableProjectRepositoryChecksTheProjectBinding(t *testing.T) {
	t.Run("bound repository is disabled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		v1 := mock.NewMockRepositoryInterface(ctrl)
		v1.EXPECT().GitHubGetRepository(gomock.Any(), "repo-1").Return(&v1Models.GithubRepository{RepositoryID: "repo-1", RepositoryName: "org/repo", RepositoryProjectSfid: "project-a"}, nil)
		v1.EXPECT().GitHubDisableRepositoryForProject(gomock.Any(), "repo-1", "project-a").Return(nil)
		ghRepo, err := (&Service{gitV1Repository: v1}).GitHubDisableProjectRepository(context.Background(), "project-a", "repo-1")
		require.NoError(t, err)
		assert.Equal(t, "org/repo", ghRepo.RepositoryName)
	})
	t.Run("another project's repository is reported as not found and left enabled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		v1 := mock.NewMockRepositoryInterface(ctrl)
		v1.EXPECT().GitHubGetRepository(gomock.Any(), "repo-1").Return(&v1Models.GithubRepository{RepositoryID: "repo-1", RepositoryProjectSfid: "project-b"}, nil)
		_, err := (&Service{gitV1Repository: v1}).GitHubDisableProjectRepository(context.Background(), "project-a", "repo-1")
		var notFound *utils.GitHubRepositoryNotFound
		require.True(t, errors.As(err, &notFound), "expected not found, got %v", err)
	})
	t.Run("lookup errors are returned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		v1 := mock.NewMockRepositoryInterface(ctrl)
		v1.EXPECT().GitHubGetRepository(gomock.Any(), "repo-1").Return(nil, errors.New("dynamo down"))
		_, err := (&Service{gitV1Repository: v1}).GitHubDisableProjectRepository(context.Background(), "project-a", "repo-1")
		assert.EqualError(t, err, "dynamo down")
	})
}

// the V4 boundary is the record's project_sfid only: the parent field does not matter, and a record
// without a binding, an unknown record or another project's record is never disabled through this route
func TestGitHubDisableProjectRepositoryPinsTheBindingCases(t *testing.T) {
	cases := []struct {
		name     string
		record   *v1Models.GithubRepository
		disabled bool
	}{
		{name: "same owner with a different parent field", record: &v1Models.GithubRepository{RepositoryID: "repo-1", RepositoryProjectSfid: "project-a", RepositorySfdcID: "foundation-y"}, disabled: true},
		{name: "unbound record", record: &v1Models.GithubRepository{RepositoryID: "repo-1"}},
		{name: "legacy sfdc binding only", record: &v1Models.GithubRepository{RepositoryID: "repo-1", RepositorySfdcID: "project-a"}},
		{name: "foreign record", record: &v1Models.GithubRepository{RepositoryID: "repo-1", RepositoryProjectSfid: "project-b"}},
		{name: "unknown record", record: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			v1 := mock.NewMockRepositoryInterface(ctrl)
			v1.EXPECT().GitHubGetRepository(gomock.Any(), "repo-1").Return(tc.record, nil)
			if tc.disabled {
				v1.EXPECT().GitHubDisableRepositoryForProject(gomock.Any(), "repo-1", "project-a").Return(nil)
			}
			ghRepo, err := (&Service{gitV1Repository: v1}).GitHubDisableProjectRepository(context.Background(), "project-a", "repo-1")
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
	t.Run("a failed disable is returned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		v1 := mock.NewMockRepositoryInterface(ctrl)
		v1.EXPECT().GitHubGetRepository(gomock.Any(), "repo-1").Return(&v1Models.GithubRepository{RepositoryID: "repo-1", RepositoryProjectSfid: "project-a"}, nil)
		v1.EXPECT().GitHubDisableRepositoryForProject(gomock.Any(), "repo-1", "project-a").Return(errors.New("dynamo down"))
		_, err := (&Service{gitV1Repository: v1}).GitHubDisableProjectRepository(context.Background(), "project-a", "repo-1")
		assert.EqualError(t, err, "dynamo down")
	})
}

// the write itself re-checks the binding: a record rebound to another project between the check and the
// write is reported as not found and stays untouched (the v1 primitive carries the DynamoDB condition)
func TestGitHubDisableProjectRepositoryHonoursTheWriteCondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	v1 := mock.NewMockRepositoryInterface(ctrl)
	v1.EXPECT().GitHubGetRepository(gomock.Any(), "repo-1").Return(&v1Models.GithubRepository{RepositoryID: "repo-1", RepositoryProjectSfid: "project-a"}, nil)
	v1.EXPECT().GitHubDisableRepositoryForProject(gomock.Any(), "repo-1", "project-a").Return(&utils.GitHubRepositoryNotFound{Message: "repository repo-1 is not bound to project project-a"})
	ghRepo, err := (&Service{gitV1Repository: v1}).GitHubDisableProjectRepository(context.Background(), "project-a", "repo-1")
	var notFound *utils.GitHubRepositoryNotFound
	require.True(t, errors.As(err, &notFound), "expected not found, got %v", err)
	assert.Nil(t, ghRepo)
}
