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
func (s *Service) GitLabAddRepositoriesByApp(ctx context.Context, gitLabOrgModel *v2GitLabOrg.GitLabOrganization) ([]*v2Models.GitlabRepository, error) {
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

	// lookup CLA Group for this project SFID
	projectCLAGroupModel, projectCLAGroupLookupErr := s.projectsClaGroupsRepo.GetClaGroupIDForProject(ctx, gitLabOrgModel.ProjectSFID)
	if projectCLAGroupLookupErr != nil || projectCLAGroupModel == nil {
		return nil, fmt.Errorf("unable to locate Project CLAGroup using projectSFID: %s for GitLab repositories group ID: %d, error: %+v", gitLabOrgModel.ProjectSFID, gitLabOrgModel.ExternalGroupID, projectCLAGroupLookupErr)
	}

	user, resp, userErr := gitlabClient.Users.CurrentUser()
	if userErr != nil {
		return nil, fmt.Errorf("unable to locate current user, error: %+v", userErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("unable to locate current user, status code: %d", resp.StatusCode)
	}

	log.WithFields(f).Debugf("fetched current username: %s with name: %s with email: %s", user.Username, user.Name, user.PublicEmail)

	// Query GitLab for repos - fetch the list of repositories available to the GitLab App
	listProjectsOpts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,   // starts with one: https://docs.gitlab.com/ee/api/#offset-based-pagination
			PerPage: 100, // max is 100
		},
		Search:           utils.StringRef(gitLabOrgModel.OrganizationName), // filter by our organization
		SearchNamespaces: utils.Bool(true),
		Membership:       utils.Bool(true),
		MinAccessLevel:   gitlab.AccessLevel(gitlab.MaintainerPermissions),
	}

	// TODO - DAD - loop until no more repos, could be more than 100
	log.WithFields(f).Debugf("searching for GitLab projects based on the search critera: %s", gitLabOrgModel.OrganizationName)
	// Need to use this func to get the list of projects the user has access to, see: https://gitlab.com/gitlab-org/gitlab-foss/-/issues/63811
	projects, resp, listProjectsErr := gitlabClient.Projects.ListProjects(listProjectsOpts)
	//projects, resp, listProjectsErr := gitlabClient.Projects.ListUserProjects(user.ID, listProjectsOpts)
	if listProjectsErr != nil {
		return nil, fmt.Errorf("unable to list projects for current user, error: %+v", listProjectsErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("unable to list projects for current user, status code: %d", resp.StatusCode)
	}

	// Add repos to table
	for _, proj := range projects {
		log.WithFields(f).Debugf("Repository: %s, path: %s, id: %d", proj.Name, proj.Path, proj.ID)

		// TODO - make sure we don't have duplicates?
		/*
		  Name: "easycla-test-repo-demo-1",
		  NameWithNamespace: "The Linux Foundation / product / EasyCLA / Demo / easycla-test-repo-demo-1",
		  Path: "easycla-test-repo-demo-1",
		  PathWithNamespace: "linuxfoundation/product/easycla/demo/easycla-test-repo-demo-1",
		*/
		_, addRepoErr := s.GitLabAddRepository(ctx, gitLabOrgModel.ProjectSFID, &v2Models.GitlabAddRepository{
			Enabled:                    false, // default is false
			Note:                       fmt.Sprintf("Added during onboarding of organization: %s", gitLabOrgModel.OrganizationName),
			RepositoryExternalID:       utils.Int64(int64(proj.ID)),
			RepositoryName:             utils.StringRef(proj.PathWithNamespace),
			RepositoryOrganizationName: utils.StringRef(gitLabOrgModel.OrganizationName),
			RepositoryProjectSfid:      utils.StringRef(gitLabOrgModel.ProjectSFID),
			RepositoryURL:              utils.StringRef(proj.WebURL),
			RepositoryClaGroupID:       utils.StringRef(projectCLAGroupModel.ClaGroupID),
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

// GitLabGetRepositoriesByOrganizationName returns the list of repositories associated with the Organization/Group name
func (s *Service) GitLabGetRepositoriesByOrganizationName(ctx context.Context, orgName string) (*v2Models.GitlabListRepositories, error) {
	dbModels, err := s.gitV2Repository.GitHubGetRepositoriesByOrganizationName(ctx, orgName)
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
