// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"errors"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AddGithubRepository must not re-bind a repository record registered under another project (lfx-self-serve-ops#751)
func TestRepositoryBoundToOtherProject(t *testing.T) {
	const authorizedProject = "a0941000002wBz4AAE"

	t.Run("record registered under the authorized project is reusable", func(t *testing.T) {
		assert.NoError(t, repositoryBoundToOtherProject(&models.GithubRepository{
			RepositoryName:        "org/repo",
			RepositoryProjectSfid: authorizedProject,
			RepositorySfdcID:      authorizedProject,
		}, authorizedProject))
	})

	t.Run("legacy record without any project binding is reusable", func(t *testing.T) {
		assert.NoError(t, repositoryBoundToOtherProject(&models.GithubRepository{RepositoryName: "org/repo"}, authorizedProject))
		assert.NoError(t, repositoryBoundToOtherProject(nil, authorizedProject))
	})

	t.Run("record registered under another project is refused", func(t *testing.T) {
		err := repositoryBoundToOtherProject(&models.GithubRepository{
			RepositoryName:        "org/repo",
			RepositoryProjectSfid: "a0941000002wBz4ZZZ",
			RepositorySfdcID:      "a0941000002wBz4ZZZ",
		}, authorizedProject)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already registered under project a0941000002wBz4ZZZ")
	})

	t.Run("either stored binding alone is enough to refuse", func(t *testing.T) {
		require.Error(t, repositoryBoundToOtherProject(&models.GithubRepository{RepositorySfdcID: "a0941000002wBz4ZZZ"}, authorizedProject))
		require.Error(t, repositoryBoundToOtherProject(&models.GithubRepository{RepositoryProjectSfid: "a0941000002wBz4ZZZ"}, authorizedProject))
	})

	t.Run("project_sfid wins over the parent foundation stored in repository_sfdc_id", func(t *testing.T) {
		// v2 stores the parent foundation in repository_sfdc_id and the owning child project in project_sfid
		assert.NoError(t, repositoryBoundToOtherProject(&models.GithubRepository{
			RepositoryName:        "org/repo",
			RepositoryProjectSfid: authorizedProject,
			RepositorySfdcID:      "a0941000002wBz4FND",
		}, authorizedProject))
		require.Error(t, repositoryBoundToOtherProject(&models.GithubRepository{
			RepositoryName:        "org/repo",
			RepositoryProjectSfid: "a0941000002wBz4ZZZ",
			RepositorySfdcID:      authorizedProject,
		}, authorizedProject), "a foundation-level match must not override a different owning project")
	})
}

// The record found by external ID is only re-used by GitHubAddRepository when it is a valid record of this project
func TestExternalIDRecordReusable(t *testing.T) {
	const (
		authorizedProject = "a0941000002wBz4AAE"
		externalID        = "123456"
	)

	t.Run("not found means nothing to re-bind", func(t *testing.T) {
		assert.NoError(t, externalIDRecordReusable(nil, &utils.GitHubRepositoryNotFound{RepositoryName: externalID}, externalID, authorizedProject))
	})

	t.Run("other lookup errors are returned as-is", func(t *testing.T) {
		lookupErr := errors.New("dynamodb unavailable")
		assert.Equal(t, lookupErr, externalIDRecordReusable(nil, lookupErr, externalID, authorizedProject))
	})

	t.Run("nil record without error is refused", func(t *testing.T) {
		err := externalIDRecordReusable(nil, nil, externalID, authorizedProject)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "external id 123456 has an invalid stored record")
	})

	t.Run("record of this project is reusable", func(t *testing.T) {
		assert.NoError(t, externalIDRecordReusable(&models.GithubRepository{RepositoryName: "org/repo", RepositoryProjectSfid: authorizedProject}, nil, externalID, authorizedProject))
	})

	t.Run("record of another project is refused", func(t *testing.T) {
		err := externalIDRecordReusable(&models.GithubRepository{RepositoryName: "org/repo", RepositoryProjectSfid: "a0941000002wBz4ZZZ"}, nil, externalID, authorizedProject)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already registered under project a0941000002wBz4ZZZ")
	})
}
