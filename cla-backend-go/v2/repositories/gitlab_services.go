// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/v2/common"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"

	v2GitLabOrg "github.com/linuxfoundation/easycla/cla-backend-go/v2/common"

	gitLabApi "github.com/linuxfoundation/easycla/cla-backend-go/gitlab_api"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"

	v2Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	repoModels "github.com/linuxfoundation/easycla/cla-backend-go/repositories"
)

// GitLabAddRepositories add a lst of GitLab repositories to the collection - default is not enabled/used/active by a CLA Group
func (s *Service) GitLabAddRepositories(ctx context.Context, projectSFID string, input *GitLabAddRepoModel) (*v2Models.GitlabRepositoriesList, error) {
	return s.GitLabAddRepositoriesWithEnabledFlag(ctx, projectSFID, input, false)
}

// GitLabAddRepositoriesWithEnabledFlag add a lst of GitLab repositories to the collection
func (s *Service) GitLabAddRepositoriesWithEnabledFlag(ctx context.Context, projectSFID string, input *GitLabAddRepoModel, enabled bool) (*v2Models.GitlabRepositoriesList, error) {
	f := logrus.Fields{
		"functionName":   "v2.repositories.gitlab_services.GitLabAddRepositories",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"groupName":      input.GroupName,
		"claGroupID":     input.ClaGroupID,
		"groupFullPath":  input.GroupFullPath,
		"groupID":        input.ExternalID,
	}

	var gitLabOrgModel *common.GitLabOrganization
	var getOrgErr error
	if input.GroupName != "" {
		log.WithFields(f).Debugf("fetching GitLab group/organization by name: %s", input.GroupName)
		gitLabOrgModel, getOrgErr = s.glOrgRepo.GetGitLabOrganizationByName(ctx, input.GroupName)
		if getOrgErr != nil {
			msg := fmt.Sprintf("problem loading GitLab group/organization by name: %s, error: %v", input.GroupName, getOrgErr)
			log.WithFields(f).WithError(getOrgErr).Warn(msg)
			return nil, errors.New(msg)
		}
	} else if input.GroupFullPath != "" {
		log.WithFields(f).Debugf("fetching GitLab group/organization by full path: %s", input.GroupFullPath)
		gitLabOrgModel, getOrgErr = s.glOrgRepo.GetGitLabOrganizationByFullPath(ctx, input.GroupFullPath)
		if getOrgErr != nil {
			msg := fmt.Sprintf("problem loading GitLab group/organization by full path: %s, error: %v", input.GroupFullPath, getOrgErr)
			log.WithFields(f).WithError(getOrgErr).Warn(msg)
			return nil, errors.New(msg)
		}
	}
	if gitLabOrgModel == nil {
		msg := fmt.Sprintf("problem loading GitLab group/organization by name '%s' or full path '%s'", input.GroupName, input.GroupFullPath)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}
	log.WithFields(f).Debugf("successfully loaded GitLab group/organization")

	// Get the client
	gitLabClient, err := gitLabApi.NewGitlabOauthClient(gitLabOrgModel.AuthInfo, s.gitLabApp)
	if err != nil {
		return nil, fmt.Errorf("initializing GitLab client : %v", err)
	}

	type GitLabAddRepositoryResponse struct {
		RepositoryName     string
		RepositoryFullPath string
		Error              error
	}
	addRepoRespChan := make(chan *GitLabAddRepositoryResponse, len(input.ProjectIDList))

	// Add each repo - could be a lot of repos, so we run this in a go routine
	for _, gitLabProjectID := range input.ProjectIDList {
		go func(gitLabProjectID int) {
			log.WithFields(f).Debugf("loading GitLab project from GitLab using projectID: %d...", gitLabProjectID)
			project, getProjectErr := gitLabApi.GetProjectByID(ctx, gitLabClient, gitLabProjectID)
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
				RepositoryFullPath:         project.PathWithNamespace,
				RepositoryURL:              project.WebURL,
				RepositoryOrganizationName: input.GroupName,
				RepositoryCLAGroupID:       input.ClaGroupID,
				RepositoryType:             utils.GitLabLower, // should always be gitlab
				Enabled:                    enabled,
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
	for range input.ProjectIDList {
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
	gitLabClient, err := gitLabApi.NewGitlabOauthClient(gitLabOrgModel.AuthInfo, s.gitLabApp)
	if err != nil {
		return nil, fmt.Errorf("initializing gitlab client : %v", err)
	}

	// lookup CLA Group for this project SFID
	projectCLAGroupModel, projectCLAGroupLookupErr := s.projectsClaGroupsRepo.GetClaGroupIDForProject(ctx, gitLabOrgModel.ProjectSFID)
	if projectCLAGroupLookupErr != nil || projectCLAGroupModel == nil {
		return nil, fmt.Errorf("unable to locate Project CLAGroup using projectSFID: %s for GitLab repositories group ID: %d, error: %+v", gitLabOrgModel.ProjectSFID, gitLabOrgModel.ExternalGroupID, projectCLAGroupLookupErr)
	}

	// Query the project list by organization name
	projectList, projectListErr := gitLabApi.GetGroupProjectListByGroupID(ctx, gitLabClient, gitLabOrgModel.ExternalGroupID)
	if projectListErr != nil {
		return nil, projectListErr
	}

	var listProjectIDs []int64

	// Build a list of project IDs
	for _, proj := range projectList {
		// Ensure we only add GitLab projects/repositories that are in the tree path of the GitLab group/organization - we don't want to reach over into a different tree if the user has access
		// The GetGroupProjectListByGroupID call in some cases returns more than expected (need to investigate why)
		if strings.HasPrefix(proj.PathWithNamespace, gitLabOrgModel.OrganizationFullPath) {
			log.WithFields(f).Debugf("adding - id: %d, repo: %s, path: %s, full path: %s, weburl: %s", proj.ID, proj.Name, proj.Path, proj.PathWithNamespace, proj.WebURL)
			listProjectIDs = append(listProjectIDs, int64(proj.ID))
		} else {
			log.WithFields(f).Debugf("skipping - id: %d, repo: %s, path: %s, full path: %s, weburl: %s", proj.ID, proj.Name, proj.Path, proj.PathWithNamespace, proj.WebURL)
		}
	}

	// Build input to the add function
	input := &GitLabAddRepoModel{
		ClaGroupID:    projectCLAGroupModel.ClaGroupID,
		GroupName:     gitLabOrgModel.OrganizationName,
		GroupFullPath: gitLabOrgModel.OrganizationFullPath,
		ExternalID:    int64(gitLabOrgModel.ExternalGroupID),
		ProjectIDList: listProjectIDs,
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

	// sort result by name
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].RepositoryName < responses[j].RepositoryName
	})

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
	dbModels, err := s.gitV2Repository.GitLabGetRepositoriesByOrganizationName(ctx, orgName)
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

// GitLabGetRepositoriesByNamePrefix returns a list of repositories that match the name prefix
func (s *Service) GitLabGetRepositoriesByNamePrefix(ctx context.Context, repositoryNamePrefix string) (*v2Models.GitlabRepositoriesList, error) {
	dbModels, err := s.gitV2Repository.GitLabGetRepositoriesByNamePrefix(ctx, repositoryNamePrefix)
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

// GitLabEnrollRepositories assigns repos to a CLA Group
func (s *Service) GitLabEnrollRepositories(ctx context.Context, claGroupID string, repositoryIDList []int64, enrollValue bool) error {
	f := logrus.Fields{
		"functionName":   "v2.repositories.gitlab_services.GitLabEnrollRepositories",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"enrollValue":    enrollValue,
	}

	type GitLabUpdateRepositoryResponse struct {
		RepositoryID int64
		Error        error
	}
	updateRepoChan := make(chan *GitLabUpdateRepositoryResponse, len(repositoryIDList))

	for _, repoID := range repositoryIDList {
		go func(claGroupID string, repoID int64) {
			updateErr := s.GitLabEnrollRepository(ctx, claGroupID, repoID, enrollValue)
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

// GitLabEnrollRepository service function enrolls a single GitLab repository to the specified CLA Group
func (s *Service) GitLabEnrollRepository(ctx context.Context, claGroupID string, repositoryExternalID int64, enrollValue bool) error {
	return s.gitV2Repository.GitLabEnrollRepositoryByID(ctx, claGroupID, repositoryExternalID, enrollValue)
}

// GitLabEnrollCLAGroupRepositories service function
func (s *Service) GitLabEnrollCLAGroupRepositories(ctx context.Context, claGroupID string, enrollValue bool) error {
	return s.gitV2Repository.GitLabEnableCLAGroupRepositories(ctx, claGroupID, enrollValue)
}

// GitLabDeleteRepositories deletes the repositories under the specified path
func (s *Service) GitLabDeleteRepositories(ctx context.Context, gitLabGroupPath string) error {
	return s.gitV2Repository.GitLabDeleteRepositories(ctx, gitLabGroupPath)
}

// GitLabDeleteRepositoryByExternalID deletes the specified repository
func (s *Service) GitLabDeleteRepositoryByExternalID(ctx context.Context, gitLabExternalID int64) error {
	// Load the record - needed for the event log after we delete
	record, getErr := s.gitV2Repository.GitLabGetRepositoryByExternalID(ctx, gitLabExternalID)
	if getErr != nil {
		return getErr
	}

	// Delete the record
	err := s.gitV2Repository.GitLabDeleteRepositoryByExternalID(ctx, gitLabExternalID)
	if err != nil {
		return err
	}

	// Convert the external ID value
	repositoryExternalID, parseIntErr := strconv.ParseInt(record.RepositoryExternalID, 10, 64)
	if err == nil {
		return parseIntErr
	}

	// Log the event
	s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType:   events.RepositoryDeleted,
		ProjectSFID: record.ProjectSFID,
		CLAGroupID:  record.RepositoryCLAGroupID,
		LfUsername:  utils.GetUserNameFromContext(ctx),
		EventData: &events.RepositoryDeletedEventData{
			RepositoryName:       record.RepositoryFullPath, // give the full path/name
			RepositoryExternalID: repositoryExternalID,
		},
	})
	return err
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
		RepositoryFullPath:         dbModel.RepositoryFullPath,         // Repository full path
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
