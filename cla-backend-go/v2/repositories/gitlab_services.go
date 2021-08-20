// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/communitybridge/easycla/cla-backend-go/events"

	v2GitLabOrg "github.com/communitybridge/easycla/cla-backend-go/v2/common"

	gitlab_api "github.com/communitybridge/easycla/cla-backend-go/gitlab_api"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"

	v2Models "github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	repoModels "github.com/communitybridge/easycla/cla-backend-go/repositories"
)

// GitLabAddRepositories service function
func (s *Service) GitLabAddRepositories(ctx context.Context, projectSFID string, input *v2Models.GitlabRepositoriesAdd) (*v2Models.GitlabRepositoriesList, error) {
	f := logrus.Fields{
		"functionName":     "v2.repositories.gitlab_services.GitLabAddRepositories",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"projectSFID":      projectSFID,
		"organizationName": input.GitlabOrganizationName,
		"claGroupID":       input.ClaGroupID,
		"groupFullPath":    input.OrganizationFullPath,
		"groupID":          input.OrganizationExternalID,
	}

	gitLabOrgModel, orgErr := s.glOrgRepo.GetGitLabOrganizationByName(ctx, input.GitlabOrganizationName)
	if orgErr != nil {
		msg := fmt.Sprintf("problem loading gitlab organization by name: %s, error: %v", input.GitlabOrganizationName, orgErr)
		log.WithFields(f).WithError(orgErr).Warn(msg)
		return nil, errors.New(msg)
	}

	// Get the client
	gitLabClient, err := gitlab_api.NewGitlabOauthClient(gitLabOrgModel.AuthInfo, s.gitLabApp)
	if err != nil {
		return nil, fmt.Errorf("initializing gitlab client : %v", err)
	}

	for _, gitLabProjectID := range input.RepositoryGitlabIds {
		project, getProjectErr := gitlab_api.GetProjectByID(ctx, gitLabClient, int(gitLabProjectID)) // ok to down-cast as the IDs are not 64 bit
		if getProjectErr != nil {
			return nil, fmt.Errorf("unable to load project by ID: %d, error: %v", int(gitLabProjectID), getProjectErr)
		}

		// Convert int to string
		repositoryExternalIDString := strconv.Itoa(project.ID)

		inputDBModel := &repoModels.RepositoryDBModel{
			RepositorySfdcID:           projectSFID,
			ProjectSFID:                projectSFID,
			RepositoryExternalID:       repositoryExternalIDString,
			RepositoryName:             project.Name,
			RepositoryFullPath:         project.PathWithNamespace,
			RepositoryURL:              project.WebURL,
			RepositoryOrganizationName: input.GitlabOrganizationName,
			RepositoryCLAGroupID:       input.ClaGroupID,
			RepositoryType:             utils.GitLabLower, // should always be gitlab
			Enabled:                    false,             // we don't enable by default
		}

		_, addErr := s.gitV2Repository.GitLabAddRepository(ctx, projectSFID, inputDBModel)
		if addErr != nil {
			log.WithFields(f).WithError(addErr).Warnf("problem adding GitLab repository with name: %s, error: %+v", project.PathWithNamespace, addErr)
			return nil, addErr
		}

		// Log the event
		s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:   events.RepositoryAdded,
			ProjectSFID: projectSFID,
			CLAGroupID:  input.ClaGroupID,
			LfUsername:  utils.GetUserNameFromContext(ctx),
			EventData: &events.RepositoryAddedEventData{
				RepositoryName: project.PathWithNamespace, // give the full path/name
			},
		})
	}

	return s.GitLabGetRepositoriesByProjectSFID(ctx, projectSFID)
}

// GitLabAddRepositoriesByApp adds the GitLab repositories based on the application credentials
func (s *Service) GitLabAddRepositoriesByApp(ctx context.Context, gitLabOrgModel *v2GitLabOrg.GitLabOrganization) ([]*v2Models.GitlabRepository, error) {
	f := logrus.Fields{
		"functionName":     "v2.repositories.gitlab_services.GitLabAddRepositoriesByApp",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"projectSFID":      gitLabOrgModel.ProjectSFID,
		"organizationName": gitLabOrgModel.OrganizationName,
		"groupFullPath":    gitLabOrgModel.OrganizationFullPath,
		"groupID":          gitLabOrgModel.ExternalGroupID,
	}

	// Get the client
	gitLabClient, err := gitlab_api.NewGitlabOauthClient(gitLabOrgModel.AuthInfo, s.gitLabApp)
	if err != nil {
		return nil, fmt.Errorf("initializing gitlab client : %v", err)
	}

	// lookup CLA Group for this project SFID
	projectCLAGroupModel, projectCLAGroupLookupErr := s.projectsClaGroupsRepo.GetClaGroupIDForProject(ctx, gitLabOrgModel.ProjectSFID)
	if projectCLAGroupLookupErr != nil || projectCLAGroupModel == nil {
		return nil, fmt.Errorf("unable to locate Project CLAGroup using projectSFID: %s for GitLab repositories group ID: %d, error: %+v", gitLabOrgModel.ProjectSFID, gitLabOrgModel.ExternalGroupID, projectCLAGroupLookupErr)
	}

	// Query the project list by organization name
	projectList, projectListErr := gitlab_api.GetGroupProjectListByGroupID(ctx, gitLabClient, gitLabOrgModel.ExternalGroupID)
	if projectListErr != nil {
		return nil, projectListErr
	}

	var listProjectIDs []int64

	// Build a list of project IDs
	for _, proj := range projectList {
		log.WithFields(f).Debugf("id: %d, repo: %s, path: %s, full path: %s, weburl: %s", proj.ID, proj.Name, proj.Path, proj.PathWithNamespace, proj.WebURL)
		listProjectIDs = append(listProjectIDs, int64(proj.ID))
	}

	// Build input to the add function
	input := &v2Models.GitlabRepositoriesAdd{
		ClaGroupID:             projectCLAGroupModel.ClaGroupID,
		GitlabOrganizationName: gitLabOrgModel.OrganizationName,
		OrganizationExternalID: int64(gitLabOrgModel.ExternalGroupID),
		OrganizationFullPath:   gitLabOrgModel.OrganizationFullPath,
		RepositoryGitlabIds:    listProjectIDs,
	}
	_, addRepoErr := s.GitLabAddRepositories(ctx, gitLabOrgModel.ProjectSFID, input)
	if addRepoErr != nil {
		return nil, addRepoErr
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
func (s *Service) GitLabGetRepositoriesByProjectSFID(ctx context.Context, projectSFID string) (*v2Models.GitlabRepositoriesList, error) {
	dbModel, err := s.gitV2Repository.GitHubGetRepositoriesByProjectSFID(ctx, projectSFID)
	if err != nil {
		return nil, err
	}

	responses, err := dbModelsToGitLabRepositories(dbModel)
	if err != nil {
		return nil, err
	}

	return &v2Models.GitlabRepositoriesList{
		List: responses,
	}, nil
}

// GitLabGetRepositoriesByCLAGroup service function
func (s *Service) GitLabGetRepositoriesByCLAGroup(ctx context.Context, claGroupID string, enabled bool) (*v2Models.GitlabRepositoriesList, error) {
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

	return &v2Models.GitlabRepositoriesList{
		List: responses,
	}, nil
}

// GitLabGetRepositoriesByOrganizationName returns the list of repositories associated with the Organization/Group name
func (s *Service) GitLabGetRepositoriesByOrganizationName(ctx context.Context, orgName string) (*v2Models.GitlabRepositoriesList, error) {
	dbModels, err := s.gitV2Repository.GitHubGetRepositoriesByOrganizationName(ctx, orgName)
	if err != nil {
		return nil, err
	}

	responses, err := dbModelsToGitLabRepositories(dbModels)
	if err != nil {
		return nil, err
	}

	return &v2Models.GitlabRepositoriesList{
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
		RepositoryFullPath:         dbModel.RepositoryFullPath,         // Full repository path
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
