// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"fmt"
	"strconv"

	v2GitLabOrg "github.com/communitybridge/easycla/cla-backend-go/v2/common"

	gitlab2 "github.com/communitybridge/easycla/cla-backend-go/gitlab"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/xanzy/go-gitlab"

	"github.com/sirupsen/logrus"

	v2Models "github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	repoModels "github.com/communitybridge/easycla/cla-backend-go/repositories"
)

// GitLabAddRepository service function
func (s *Service) GitLabAddRepository(ctx context.Context, projectSFID string, input *v2Models.GitlabAddRepository) (*v2Models.GitlabRepository, error) {
	dbModel, err := s.gitV2Repository.GitLabAddRepository(ctx, projectSFID, input)
	if err != nil {
		return nil, err
	}

	return dbModelToGitLabRepository(dbModel)
}

// GitLabAddRepositoriesByApp adds the GitLab repositories based on the application credentials
func (s *Service) GitLabAddRepositoriesByApp(ctx context.Context, gitLabOrgModel *v2GitLabOrg.GitlabOrganization) ([]*v2Models.GitlabRepository, error) {
	f := logrus.Fields{
		"functionName":     "v2.repositories.gitlab_services.GitLabAddRepositoriesByApp",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"projectSFID":      gitLabOrgModel.ProjectSFID,
		"organizationName": gitLabOrgModel.OrganizationName,
	}
	// Get the client
	gitlabClient, err := gitlab2.NewGitlabOauthClient(gitLabOrgModel.AuthInfo, s.gitLabApp)
	if err != nil {
		return nil, fmt.Errorf("initializing gitlab client : %v", err)
	}

	// Query GitLab for repos - fetch the list of repositories available to the GitLab App
	opt := &gitlab.ListTreeOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1, // starts with one: https://docs.gitlab.com/ee/api/#offset-based-pagination
			PerPage: 100,
		},
		//Path      *string `url:"path,omitempty" json:"path,omitempty"`
		//Ref       *string `url:"ref,omitempty" json:"ref,omitempty"`
		Recursive: utils.Bool(true),
	}

	tree, resp, listTreeErr := gitlabClient.Repositories.ListTree(gitLabOrgModel.ExternalGroupID, opt)
	if listTreeErr != nil {
		return nil, fmt.Errorf("unable to locate GitLab repositories group ID: %d, error: %+v", gitLabOrgModel.ExternalGroupID, listTreeErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("unable to locate GitLab repositories group ID: %d, status code: %d", gitLabOrgModel.ExternalGroupID, resp.StatusCode)
	}

	// Add repos to table
	for _, tr := range tree {
		log.WithFields(f).Debugf("Repository: %s, path: %s, id: %s", tr.Name, tr.Path, tr.ID)
		//externalID, convertErr := strconv.ParseInt(tr.ID, 10, 64)
		//if convertErr != nil {
		//	return nil, convertErr
		//}

		_, addRepoErr := s.GitLabAddRepository(ctx, gitLabOrgModel.ProjectSFID, &v2Models.GitlabAddRepository{
			Enabled:                    true,
			Note:                       fmt.Sprintf("Added during onboarding of organization: %s", gitLabOrgModel.OrganizationName),
			RepositoryGitlabExternalID: utils.StringRef(tr.ID), // utils.Int64(externalID),
			RepositoryName:             utils.StringRef(tr.Name),
			RepositoryOrganizationName: utils.StringRef(gitLabOrgModel.OrganizationName),
			RepositoryProjectSfid:      utils.StringRef(gitLabOrgModel.ProjectSFID),
			RepositoryURL:              utils.StringRef(tr.Path),
			//RepositoryClaGroupID:       utils.StringRef(gitLabOrgModel.),
		})
		if addRepoErr != nil {
			return nil, addRepoErr
		}
	}

	// Return the list of repos to caller
	dbModels, getRepoErr := s.gitV2Repository.GitHubGetRepositoriesByProjectSFID(ctx, gitLabOrgModel.ProjectSFID)
	if getRepoErr != nil {
		return nil, getRepoErr
	}
	return dbModelsToGitLabRepositories(dbModels)
}

// GitLabGetRepository service function
func (s *Service) GitLabGetRepository(ctx context.Context, repositoryID string) (*v2Models.GitlabRepository, error) {
	dbModel, err := s.gitV2Repository.GitLabGetRepository(ctx, repositoryID)
	if err != nil {
		return nil, err
	}

	return dbModelToGitLabRepository(dbModel)
}

// GitLabGetRepositoryByName service function
func (s *Service) GitLabGetRepositoryByName(ctx context.Context, repositoryName string) (*v2Models.GitlabRepository, error) {
	dbModel, err := s.gitV2Repository.GitLabGetRepositoryByName(ctx, repositoryName)
	if err != nil {
		return nil, err
	}

	return dbModelToGitLabRepository(dbModel)
}

// GitLabGetRepositoriesByProjectSFID service function
func (s *Service) GitLabGetRepositoriesByProjectSFID(ctx context.Context, projectSFID string) (*v2Models.GitlabListRepositories, error) {
	dbModel, err := s.gitV2Repository.GitHubGetRepositoriesByProjectSFID(ctx, projectSFID)
	if err != nil {
		return nil, err
	}

	responses, err := dbModelsToGitLabRepositories(dbModel)
	if err != nil {
		return nil, err
	}

	return &v2Models.GitlabListRepositories{
		List: responses,
	}, nil
}

// GitLabGetRepositoriesByCLAGroup service function
func (s *Service) GitLabGetRepositoriesByCLAGroup(ctx context.Context, claGroupID string, enabled bool) (*v2Models.GitlabListRepositories, error) {
	var dbModels []*repoModels.RepositoryDBModel
	var err error
	if enabled {
		dbModels, err = s.gitV2Repository.GitHubGetRepositoriesByCLAGroupEnabled(ctx, claGroupID)
	} else {
		dbModels, err = s.gitV2Repository.GitHubGetRepositoriesByCLAGroupDisabled(ctx, claGroupID)
	}
	if err != nil {
		return nil, err
	}

	responses, err := dbModelsToGitLabRepositories(dbModels)
	if err != nil {
		return nil, err
	}

	return &v2Models.GitlabListRepositories{
		List: responses,
	}, nil
}

// GitLabEnableRepository service function
func (s *Service) GitLabEnableRepository(ctx context.Context, repositoryID string) error {
	return s.gitV2Repository.GitLabEnableRepositoryByID(ctx, repositoryID)
}

// GitLabDisableRepository service function
func (s *Service) GitLabDisableRepository(ctx context.Context, repositoryID string) error {
	return s.gitV2Repository.GitLabDisableRepositoryByID(ctx, repositoryID)
}

// GitLabDisableCLAGroupRepositories service function
func (s *Service) GitLabDisableCLAGroupRepositories(ctx context.Context, claGroupID string) error {
	return s.gitV2Repository.GitLabDisableCLAGroupRepositories(ctx, claGroupID)
}

// dbModelToGitLabRepository converts the database model to a v2 response model
func dbModelToGitLabRepository(dbModel *repoModels.RepositoryDBModel) (*v2Models.GitlabRepository, error) {

	gitLabExternalID, err := strconv.ParseInt(dbModel.RepositoryExternalID, 10, 64)
	if err != nil {
		return nil, err
	}

	response := v2Models.GitlabRepository{
		RepositoryID:               dbModel.RepositoryID,               // Internal database ID for this repository record
		RepositoryProjectSfid:      dbModel.ProjectSFID,                // Project SFID
		RepositoryClaGroupID:       dbModel.RepositoryCLAGroupID,       // CLA Group ID
		RepositoryExternalID:       gitLabExternalID,                   // GitLab unique gitV1Repository ID
		RepositoryName:             dbModel.RepositoryName,             // Short repository name
		RepositoryOrganizationName: dbModel.RepositoryOrganizationName, // Group/Organization name
		RepositoryURL:              dbModel.RepositoryURL,              // full url
		RepositoryType:             dbModel.RepositoryType,             // gitlab
		Enabled:                    dbModel.Enabled,                    // Enabled flag
		DateCreated:                dbModel.DateCreated,                // date created
		DateModified:               dbModel.DateModified,               // date updated
		Note:                       dbModel.Note,                       // Optional note
		Version:                    dbModel.Version,                    // record version
	}

	return &response, nil
}

// dbModelsToGitLabRepositories converts the slice of database models to a slice of v2 response model
func dbModelsToGitLabRepositories(dbModels []*repoModels.RepositoryDBModel) ([]*v2Models.GitlabRepository, error) {
	var responses []*v2Models.GitlabRepository
	for _, dbModel := range dbModels {
		response, err := dbModelToGitLabRepository(dbModel)
		if err != nil {
			return nil, err
		}
		// Add to the list
		responses = append(responses, response)
	}
	return responses, nil
}
