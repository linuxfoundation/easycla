// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/communitybridge/easycla/cla-backend-go/v2/common"

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
func (s *Service) GitLabAddRepositories(ctx context.Context, projectSFID string, input *v2Models.GitlabRepositoriesEnable) (*v2Models.GitlabRepositoriesList, error) {
	f := logrus.Fields{
		"functionName":     "v2.repositories.gitlab_services.GitLabAddRepositories",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"projectSFID":      projectSFID,
		"organizationName": input.GitlabOrganizationName,
		"claGroupID":       input.ClaGroupID,
		"groupFullPath":    input.OrganizationFullPath,
		"groupID":          input.OrganizationExternalID,
	}

	var gitLabOrgModel *common.GitLabOrganization
	var getOrgErr error
	if input.GitlabOrganizationName != "" {
		log.WithFields(f).Debugf("fetching GitLab organization by name: %s", input.GitlabOrganizationName)
		gitLabOrgModel, getOrgErr = s.glOrgRepo.GetGitLabOrganizationByName(ctx, input.GitlabOrganizationName)
		if getOrgErr != nil {
			msg := fmt.Sprintf("problem loading GitLab organization by name: %s, error: %v", input.GitlabOrganizationName, getOrgErr)
			log.WithFields(f).WithError(getOrgErr).Warn(msg)
			return nil, errors.New(msg)
		}
	} else if input.OrganizationFullPath != "" {
		log.WithFields(f).Debugf("fetching GitLab organization by full path: %s", input.OrganizationFullPath)
		gitLabOrgModel, getOrgErr = s.glOrgRepo.GetGitLabOrganizationByFullPath(ctx, input.OrganizationFullPath)
		if getOrgErr != nil {
			msg := fmt.Sprintf("problem loading GitLab organization by full path: %s, error: %v", input.OrganizationFullPath, getOrgErr)
			log.WithFields(f).WithError(getOrgErr).Warn(msg)
			return nil, errors.New(msg)
		}
	}
	if gitLabOrgModel == nil {
		msg := fmt.Sprintf("problem loading GitLab organization by name '%s' or full path '%s'", input.GitlabOrganizationName, input.OrganizationFullPath)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}
	log.WithFields(f).Debugf("successfully loaded GitLab group/organization")

	// Get the client
	gitLabClient, err := gitlab_api.NewGitlabOauthClient(gitLabOrgModel.AuthInfo, s.gitLabApp)
	if err != nil {
		return nil, fmt.Errorf("initializing GitLab client : %v", err)
	}

	type GitLabAddRepositoryResponse struct {
		RepositoryName     string
		RepositoryFullPath string
		Error              error
	}
	addRepoRespChan := make(chan *GitLabAddRepositoryResponse, len(input.RepositoryGitlabIds))

	// Add each repo - could be a lot of repos, so we run this in a go routine
	for _, gitLabProjectID := range input.RepositoryGitlabIds {
		go func(gitLabProjectID int) {
			log.WithFields(f).Debugf("loading GitLab project from GitLab using projectID: %d...", gitLabProjectID)
			project, getProjectErr := gitlab_api.GetProjectByID(ctx, gitLabClient, gitLabProjectID)
			if getProjectErr != nil {
				newErr := fmt.Errorf("unable to load GitLab project using ID: %d, error: %v", gitLabProjectID, getProjectErr)
				log.WithFields(f).WithError(newErr)
				addRepoRespChan <- &GitLabAddRepositoryResponse{
					Error: newErr,
				}
				return
			}
			log.WithFields(f).Debugf("loaded GitLab project from GitLab using projectID: %d", gitLabProjectID)

			// Convert int to string
			repositoryExternalIDString := strconv.Itoa(project.ID)

			inputDBModel := &repoModels.RepositoryDBModel{
				RepositorySfdcID:           projectSFID,
				ProjectSFID:                projectSFID,
				RepositoryExternalID:       repositoryExternalIDString,
				RepositoryName:             project.PathWithNamespace, // Name column is actually the full path for both GitHub and GitLab
				RepositoryURL:              project.WebURL,
				RepositoryOrganizationName: input.GitlabOrganizationName,
				RepositoryCLAGroupID:       input.ClaGroupID,
				RepositoryType:             utils.GitLabLower, // should always be gitlab
				Enabled:                    false,             // we don't enable by default
			}

			repoModel, addErr := s.gitV2Repository.GitLabAddRepository(ctx, projectSFID, inputDBModel)
			if addErr != nil || repoModel == nil {
				log.WithFields(f).WithError(addErr).Warnf("problem adding GitLab repository with name: %s, error: %+v", project.PathWithNamespace, addErr)
				addRepoRespChan <- &GitLabAddRepositoryResponse{
					Error: addErr,
				}
				return
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
			addRepoRespChan <- &GitLabAddRepositoryResponse{
				RepositoryName:     repoModel.RepositoryName,
				RepositoryFullPath: repoModel.RepositoryFullPath,
				Error:              nil,
			}
		}(int(gitLabProjectID)) // ok to down cast
	}

	// Wait for the go routines to finish and load up the results
	log.WithFields(f).Debug("waiting for add repos to finish...")
	var lastErr error
	for range input.RepositoryGitlabIds {
		select {
		case response := <-addRepoRespChan:
			if response.Error != nil {
				log.WithFields(f).WithError(response.Error).Warn(response.Error.Error())
				lastErr = response.Error
			} else {
				log.WithFields(f).Debugf("added repo: %s with full path: %s", response.RepositoryName, response.RepositoryFullPath)
			}
		case <-ctx.Done():
			log.WithFields(f).WithError(ctx.Err()).Warnf("waiting for add repositories timed out")
			lastErr = fmt.Errorf("add repositories failed with timeout, error: %v", ctx.Err())
		}
	}

	if lastErr != nil {
		return nil, lastErr
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
	input := &v2Models.GitlabRepositoriesEnable{
		ClaGroupID:             projectCLAGroupModel.ClaGroupID,
		GitlabOrganizationName: gitLabOrgModel.OrganizationName,
		OrganizationExternalID: int64(gitLabOrgModel.ExternalGroupID),
		OrganizationFullPath:   gitLabOrgModel.OrganizationFullPath,
		RepositoryGitlabIds:    listProjectIDs,
	}
	log.WithFields(f).Debugf("adding %d GitLab repositories", len(listProjectIDs))
	_, addRepoErr := s.GitLabAddRepositories(ctx, gitLabOrgModel.ProjectSFID, input)
	if addRepoErr != nil {
		log.WithFields(f).WithError(addRepoErr).Warnf("problem adding %d GitLab repositories", len(listProjectIDs))
		return nil, addRepoErr
	}

	// Return the list of repos to caller
	log.WithFields(f).Debugf("fetching complete repository list by project SFID: %s", gitLabOrgModel.ProjectSFID)
	dbModels, getRepoErr := s.gitV2Repository.GitHubGetRepositoriesByProjectSFID(ctx, gitLabOrgModel.ProjectSFID)
	if getRepoErr != nil {
		log.WithFields(f).WithError(getRepoErr).Warnf("problem fetching repositories by project SFID: %s", gitLabOrgModel.ProjectSFID)
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

// GitLabGetRepositoryByExternalID service function
func (s *Service) GitLabGetRepositoryByExternalID(ctx context.Context, repositoryExternalID int64) (*v2Models.GitlabRepository, error) {
	dbModel, err := s.gitV2Repository.GitLabGetRepositoryByExternalID(ctx, repositoryExternalID)
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

// GitLabEnableRepositories assigns repos to
func (s *Service) GitLabEnableRepositories(ctx context.Context, claGroupID string, repositoryIDList []int64) error {
	f := logrus.Fields{
		"functionName":   "v2.repositories.gitlab_services.GitLabEnableRepositories",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	type GitLabUpdateRepositoryResponse struct {
		RepositoryID int64
		Error        error
	}
	updateRepoChan := make(chan *GitLabUpdateRepositoryResponse, len(repositoryIDList))

	for _, repoID := range repositoryIDList {
		go func(claGroupID string, repoID int64) {
			updateErr := s.GitLabEnableRepository(ctx, claGroupID, repoID)
			updateRepoChan <- &GitLabUpdateRepositoryResponse{
				RepositoryID: repoID,
				Error:        updateErr,
			}
		}(claGroupID, repoID)
	}

	// Wait for the go routines to finish and load up the results
	log.WithFields(f).Debug("waiting for update repos to finish...")
	var lastErr error
	for range repositoryIDList {
		select {
		case response := <-updateRepoChan:
			if response.Error != nil {
				log.WithFields(f).WithError(response.Error).Warn(response.Error.Error())
				lastErr = response.Error
			} else {
				log.WithFields(f).Debugf("updated repo with ID: %d", response.RepositoryID)
			}
		case <-ctx.Done():
			log.WithFields(f).WithError(ctx.Err()).Warnf("waiting for update GitLab repos timed out")
			lastErr = fmt.Errorf("waiting for update GitLab repositories timed out, error: %v", ctx.Err())
		}
	}

	return lastErr
}

// GitLabEnableRepository service function
func (s *Service) GitLabEnableRepository(ctx context.Context, claGroupID string, repositoryExternalID int64) error {
	return s.gitV2Repository.GitLabEnableRepositoryByID(ctx, claGroupID, repositoryExternalID)
}

// GitLabDisableRepository service function
func (s *Service) GitLabDisableRepository(ctx context.Context, claGroupID string, repositoryExternalID int64) error {
	return s.gitV2Repository.GitLabDisableRepositoryByID(ctx, claGroupID, repositoryExternalID)
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
