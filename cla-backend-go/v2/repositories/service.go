// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/github_organizations"

	"github.com/linuxfoundation/easycla/cla-backend-go/v2/common"

	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	gitLabApi "github.com/linuxfoundation/easycla/cla-backend-go/gitlab_api"

	"github.com/linuxfoundation/easycla/cla-backend-go/github/branch_protection"

	"github.com/sirupsen/logrus"

	"github.com/go-openapi/swag"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/linuxfoundation/easycla/cla-backend-go/github"

	"github.com/aws/aws-sdk-go/aws"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	v2Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	v1Repositories "github.com/linuxfoundation/easycla/cla-backend-go/repositories"
	v2ProjectService "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
)

// ServiceInterface contains functions of the repositories service
type ServiceInterface interface {
	// GitHub

	GitHubAddRepositories(ctx context.Context, projectSFID string, input *models.GithubRepositoryInput) ([]*v1Models.GithubRepository, error)
	GitHubEnableRepository(ctx context.Context, repositoryID string) error
	GitHubDisableRepository(ctx context.Context, repositoryID string) error
	GitHubListProjectRepositories(ctx context.Context, projectSFID string) (*v1Models.GithubListRepositories, error)
	GitHubGetRepository(ctx context.Context, repositoryID string) (*v1Models.GithubRepository, error)
	GitHubGetRepositoryByName(ctx context.Context, repositoryName string) (*v1Models.GithubRepository, error)
	GitHubGetRepositoryByExternalID(ctx context.Context, repositoryExternalID string) (*v1Models.GithubRepository, error)
	GitHubDisableCLAGroupRepositories(ctx context.Context, claGroupID string) error
	GitHubGetProtectedBranch(ctx context.Context, projectSFID, repositoryID, branchName string) (*v2Models.GithubRepositoryBranchProtection, error)
	GitHubUpdateProtectedBranch(ctx context.Context, projectSFID, repositoryID string, input *v2Models.GithubRepositoryBranchProtectionInput) (*v2Models.GithubRepositoryBranchProtection, error)

	// GitLab

	GitLabGetRepository(ctx context.Context, repositoryID string) (*v2Models.GitlabRepository, error)
	GitLabGetRepositoryByName(ctx context.Context, repositoryName string) (*v2Models.GitlabRepository, error)
	GitLabGetRepositoryByExternalID(ctx context.Context, repositoryExternalID int64) (*v2Models.GitlabRepository, error)
	GitLabGetRepositoriesByProjectSFID(ctx context.Context, projectSFID string) (*v2Models.GitlabRepositoriesList, error)
	GitLabGetRepositoriesByCLAGroup(ctx context.Context, claGroupID string, enabled bool) (*v2Models.GitlabRepositoriesList, error)
	GitLabGetRepositoriesByOrganizationName(ctx context.Context, orgName string) (*v2Models.GitlabRepositoriesList, error)
	GitLabGetRepositoriesByNamePrefix(ctx context.Context, repositoryNamePrefix string) (*v2Models.GitlabRepositoriesList, error)
	GitLabAddRepositories(ctx context.Context, projectSFID string, input *GitLabAddRepoModel) (*v2Models.GitlabRepositoriesList, error)
	GitLabAddRepositoriesWithEnabledFlag(ctx context.Context, projectSFID string, input *GitLabAddRepoModel, enabled bool) (*v2Models.GitlabRepositoriesList, error)
	GitLabAddRepositoriesByApp(ctx context.Context, gitLabOrgModel *common.GitLabOrganization) ([]*v2Models.GitlabRepository, error)
	GitLabEnrollRepositories(ctx context.Context, claGroupID string, repositoryIDList []int64, enrollValue bool) error
	GitLabEnrollRepository(ctx context.Context, claGroupID string, repositoryExternalID int64, enrollValue bool) error
	GitLabEnrollCLAGroupRepositories(ctx context.Context, claGroupID string, enrollValue bool) error
	GitLabDeleteRepositories(ctx context.Context, gitLabGroupPath string) error
	GitLabDeleteRepositoryByExternalID(ctx context.Context, gitLabExternalID int64) error

	// All
	GetRepositoriesByProjectSFID(ctx context.Context, projectSFID string) ([]*v1Repositories.RepositoryDBModel, error)
}

// GitLabOrgRepo redefine the interface here to avoid circular dependency issues
type GitLabOrgRepo interface {
	AddGitLabOrganization(ctx context.Context, input *common.GitLabAddOrganization, enabled bool) (*v2Models.GitlabOrganization, error)
	GetGitLabOrganizationsByProjectSFID(ctx context.Context, projectSFID string) (*v2Models.GitlabOrganizations, error)
	GetGitLabOrganization(ctx context.Context, gitlabOrganizationID string) (*common.GitLabOrganization, error)
	GetGitLabOrganizationByName(ctx context.Context, gitLabOrganizationName string) (*common.GitLabOrganization, error)
	GetGitLabOrganizationByExternalID(ctx context.Context, gitLabGroupID int64) (*common.GitLabOrganization, error)
	GetGitLabOrganizationByFullPath(ctx context.Context, groupFullPath string) (*common.GitLabOrganization, error)
	UpdateGitLabOrganizationAuth(ctx context.Context, organizationID string, gitLabGroupID int, authExpiryTime int64, authInfo, groupName, groupFullPath, organizationURL string) error
	UpdateGitLabOrganization(ctx context.Context, input *common.GitLabAddOrganization, enabled bool) error
	DeleteGitLabOrganizationByFullPath(ctx context.Context, projectSFID, gitlabOrgFullPath string) error
}

// Service is the service model/structure
type Service struct {
	gitV1Repository       v1Repositories.RepositoryInterface
	gitV2Repository       RepositoryInterface
	projectsClaGroupsRepo projects_cla_groups.Repository
	ghOrgRepo             github_organizations.RepositoryInterface
	glOrgRepo             GitLabOrgRepo
	gitLabApp             *gitLabApi.App
	eventService          events.Service
}

var (
	requiredBranchProtectionChecks = []string{"EasyCLA"}
	// ErrInvalidBranchProtectionName is returned when invalid protection option is supplied
	ErrInvalidBranchProtectionName = errors.New("invalid protection option")
)

// NewService creates a new githubOrganizations service
func NewService(gitV1Repository *v1Repositories.Repository, gitV2Repository RepositoryInterface, pcgRepo projects_cla_groups.Repository, ghOrgRepo github_organizations.RepositoryInterface, glOrgRepo GitLabOrgRepo, eventService events.Service) ServiceInterface {
	return &Service{
		gitV1Repository:       gitV1Repository,
		gitV2Repository:       gitV2Repository,
		projectsClaGroupsRepo: pcgRepo,
		ghOrgRepo:             ghOrgRepo,
		glOrgRepo:             glOrgRepo,
		eventService:          eventService,
		gitLabApp:             gitLabApi.Init(config.GetConfig().Gitlab.AppClientID, config.GetConfig().Gitlab.AppClientSecret, config.GetConfig().Gitlab.AppPrivateKey),
	}
}

// GitHubAddRepositories adds the specified GitHub repository to the specified project
func (s *Service) GitHubAddRepositories(ctx context.Context, projectSFID string, input *models.GithubRepositoryInput) ([]*v1Models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":           "v2.repositories.service.GitHubAddRepositories",
		utils.XREQUESTID:         ctx.Value(utils.XREQUESTID),
		"projectSFID":            projectSFID,
		"claGroupID":             utils.StringValue(input.ClaGroupID),
		"githubOrganizationName": utils.StringValue(input.GithubOrganizationName),
		"repositoryGitHubID":     input.RepositoryGithubID,
		"repositoryGithubIds":    input.RepositoryGithubIds,
	}

	log.WithFields(f).Debugf("loading project by SFID: %s", projectSFID)
	psc := v2ProjectService.GetClient()
	project, err := psc.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to load projectSFID from the platform project service")
		return nil, err
	}

	var parentProjectSFID string
	if !utils.IsProjectHaveParent(project) || utils.IsProjectHasRootParent(project) || utils.GetProjectParentSFID(project) == "" {
		parentProjectSFID = projectSFID
	} else {
		parentProjectSFID = utils.GetProjectParentSFID(project)
	}

	allMappings, err := s.projectsClaGroupsRepo.GetProjectsIdsForClaGroup(ctx, aws.StringValue(input.ClaGroupID))
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to get project IDs for CLA Group")
		return nil, err
	}
	var valid bool
	for _, cgm := range allMappings {
		if cgm.ProjectSFID == projectSFID || cgm.FoundationSFID == projectSFID {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("provided cla group id %s is not linked to project sfid %s", utils.StringValue(input.ClaGroupID), projectSFID)
	}

	org, err := s.ghOrgRepo.GetGitHubOrganizationByName(ctx, utils.StringValue(input.GithubOrganizationName))
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to get organization by name")
		return nil, err
	}

	// Updated to process a list of repository IDs - take the list (may be empty) and add the single repository GH ID if it was set
	repositoryIDList := input.RepositoryGithubIds
	if input.RepositoryGithubID != "" {
		repositoryIDList = append(repositoryIDList, input.RepositoryGithubID)
	}

	// Remove any silly duplicates that may come
	repositoryIDList = utils.RemoveDuplicates(repositoryIDList)

	var response []*v1Models.GithubRepository

	// For each repository ID provided...
	// If this is slow, may want to optimize by making separate go routines for each item in the list
	for _, repoID := range repositoryIDList {
		// Convert the string value to an integer
		repoGithubID, err := strconv.ParseInt(repoID, 10, 64)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("unable to convert repository github ID %s to an integer - invalid value", repoID)
			return nil, err
		}

		log.WithFields(f).Debugf("loading GitHub repository by external id: %d", repoGithubID)
		ghRepo, err := github.GetRepositoryByExternalID(ctx, org.List[0].OrganizationInstallationID, repoGithubID)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("unable to load repository by external ID: %d", repoGithubID)
			return nil, err
		}
		f["repositoryName"] = ghRepo.FullName
		f["repositoryURL"] = ghRepo.URL
		f["repositoryGitHubID"] = repoGithubID
		log.WithFields(f).Debugf("loaded GitHub repository by external id: %d - url: %s", repoGithubID, utils.StringValue(ghRepo.URL))

		// Check if this repository exists in our database
		log.WithFields(f).Debugf("checking if GitHub repository by name: %s exists...", utils.StringValue(ghRepo.FullName))
		existingRepositoryModel, lookupErr := s.GitHubGetRepositoryByName(ctx, utils.StringValue(ghRepo.FullName))
		if lookupErr != nil {
			// If we have the repository not found error - this is ok - we are expecting this
			if notFoundErr, ok := lookupErr.(*utils.GitHubRepositoryNotFound); ok {
				log.WithFields(f).WithError(notFoundErr).Debugf("GitHub repository lookup didn't find a match for existing repository name: %s - ok to create", utils.StringValue(ghRepo.FullName))
			} else {
				// Some other error - not good...
				log.WithFields(f).WithError(lookupErr).Warnf("GitHub repository lookup failed for repository name: %s", utils.StringValue(ghRepo.FullName))
				return nil, lookupErr
			}
		}

		// We already have an existing repository model with the same name
		if existingRepositoryModel != nil {
			if !existingRepositoryModel.Enabled {
				msg := fmt.Sprintf("Github repository: %s previously disabled - will re-enabled... ", utils.StringValue(ghRepo.FullName))
				log.WithFields(f).Debug(msg)
				enabled := true

				_, now := utils.CurrentTime()

				log.WithFields(f).Debugf("Updating GitHub repository - setting enabled: true, OrgName: %s, CLA Group ID: %s",
					utils.StringValue(input.GithubOrganizationName), utils.StringValue(input.ClaGroupID))
				v1Input := &v1Models.GithubRepositoryInput{
					Enabled:                    &enabled,
					RepositoryOrganizationName: input.GithubOrganizationName,
					RepositoryProjectID:        input.ClaGroupID,
					Note:                       fmt.Sprintf("re-enabling repository on %s.", now),
				}

				// Update Repo details in case of any changes
				updatedRepository, updateErr := s.gitV1Repository.GitHubUpdateRepository(ctx, existingRepositoryModel.RepositoryID, projectSFID, parentProjectSFID, v1Input)
				if updateErr != nil {
					log.WithFields(f).WithError(updateErr).Warnf("unable to update GitHub repository with name: %s, id: %s, using input: %+v", utils.StringValue(ghRepo.FullName), existingRepositoryModel.RepositoryID, v1Input)
					return nil, updateErr
				}

				// Append the results to our response model
				response = append(response, updatedRepository)
			} else {
				log.WithFields(f).Warnf("GitHub repository already exists with repository name: %s and is already enabled - skipping update", utils.StringValue(ghRepo.FullName))
				continue
			}
		} else {
			// No record exists...
			log.WithFields(f).Debug("no existing GitHub repository configured - creating...")
			in := &v1Models.GithubRepositoryInput{
				RepositoryExternalID:       &repoID, // nolint
				RepositoryName:             ghRepo.FullName,
				RepositoryOrganizationName: input.GithubOrganizationName,
				RepositoryProjectID:        input.ClaGroupID,
				RepositoryType:             aws.String("github"),
				RepositoryURL:              ghRepo.HTMLURL,
			}

			addedModel, addErr := s.gitV1Repository.GitHubAddRepository(ctx, parentProjectSFID, projectSFID, in)
			if addErr != nil {
				log.WithFields(f).WithError(addErr).Warnf("unable to add github repository: %s for project: %s", *ghRepo.FullName, projectSFID)
				return nil, addErr
			}

			// Append the results to our response model
			response = append(response, addedModel)
		}
	}

	return response, nil
}

// GitHubEnableRepository service function
func (s *Service) GitHubEnableRepository(ctx context.Context, repositoryID string) error {
	return s.gitV1Repository.GitHubEnableRepository(ctx, repositoryID)
}

// GitHubDisableRepository service function
func (s *Service) GitHubDisableRepository(ctx context.Context, repositoryID string) error {
	return s.gitV1Repository.GitHubDisableRepository(ctx, repositoryID)
}

// GitHubListProjectRepositories service function
func (s *Service) GitHubListProjectRepositories(ctx context.Context, projectSFID string) (*v1Models.GithubListRepositories, error) {
	f := logrus.Fields{
		"functionName":   "v2.repositories.service.GitHubListProjectRepositories",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
	}

	log.WithFields(f).Debug("querying project service for project...")
	psc := v2ProjectService.GetClient()
	projectModel, err := psc.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup project by id in the project service")
		return nil, err
	}
	if projectModel == nil {
		log.WithFields(f).Warn("unable to lookup project by id in the project service - no record found")
		return nil, err
	}
	f["projectName"] = projectModel.Name
	if utils.IsProjectHaveParent(projectModel) {
		f["projectParentSFID"] = utils.GetProjectParentSFID(projectModel)
	}
	log.WithFields(f).Debug("loaded project from the project service")
	enabled := true
	return s.gitV1Repository.GitHubListProjectRepositories(ctx, projectSFID, &enabled)

	//// Lookup orgs via projectSFID
	//log.WithFields(f).Debug("querying EasyCLA for organizations by project id...")
	//var githubOrgList *v1Models.GithubOrganizations
	//githubOrgList, err = s.ghOrgRepo.GetGitHubOrganizations(ctx, projectSFID)
	//if err != nil {
	//	log.WithFields(f).WithError(err).Warn("unable to lookup project by id in the github organization table")
	//	if projectModel.Parent != "" {
	//		log.WithFields(f).Debugf("querying for organizations by parent project id: %s...", projectModel.Parent)
	//		var ghOrgErr error
	//		githubOrgList, ghOrgErr = s.ghOrgRepo.GetGitHubOrganizations(ctx, projectModel.Parent)
	//		if ghOrgErr != nil {
	//			log.WithFields(f).WithError(ghOrgErr).Warn("unable to lookup project by parent id in the github organization table")
	//			return nil, ghOrgErr
	//		}
	//	}
	//}
	//
	//// Our response - empty to start with
	//response := &v1Models.GithubListRepositories{
	//	List: []*v1Models.GithubRepository{},
	//}
	//
	//if githubOrgList == nil {
	//	log.WithFields(f).Warn("unable to lookup project by id - no records found")
	//	return response, err
	//}
	//log.WithFields(f).Debugf("loaded %d EasyCLA GitHub organizations for project", len(githubOrgList.List))
	//
	//// For each of the organizations we have in our database for this project...
	//for _, gitHubOrg := range githubOrgList.List {
	//	// Query GitHub for the list of public repositories...
	//	log.WithFields(f).Debugf("querying github by organization: %s", gitHubOrg.OrganizationName)
	//	ghRepoList, getRepoErr := github.GetRepositories(ctx, gitHubOrg.OrganizationName)
	//	if getRepoErr != nil {
	//		log.WithFields(f).WithError(getRepoErr).Warn("unable to lookup github organization details")
	//		return response, getRepoErr
	//	}
	//
	//	// Add to our response model...use default values (enabled = false)
	//	log.WithFields(f).Debugf("found %d github repositories for organization: %s", len(ghRepoList), gitHubOrg.OrganizationName)
	//	for _, ghRepo := range ghRepoList {
	//		response.List = append(response.List, &v1Models.GithubRepository{
	//			Enabled:                    false,
	//			ProjectSFID:                projectSFID,
	//			RepositoryExternalID:       projectSFID,
	//			RepositoryName:             utils.StringValue(ghRepo.Name),
	//			RepositoryOrganizationName: gitHubOrg.OrganizationName,
	//			RepositoryType:             "github",
	//			RepositoryURL:              utils.StringValue(ghRepo.URL),
	//			Version:                    "v1",
	//		})
	//	}
	//}
	//
	//// Now, query our DB....
	//listOurGitHubRepos, err := s.gitV1Repository.GitHubListProjectRepositories(ctx, "", projectSFID, true)
	//if err != nil {
	//	log.WithFields(f).WithError(err).Warn("unable to lookup repository records by id in our repositories table ")
	//	return response, err
	//}
	//if listOurGitHubRepos == nil || len(listOurGitHubRepos.List) == 0 {
	//	log.WithFields(f).Warn("unable to lookup repository records by id in our repositories table ")
	//	return response, err
	//}
	//
	//// For each gitV1Repository that we have...
	//for _, ourGitHubRepo := range listOurGitHubRepos.List {
	//	// Inefficient, but ok if the number of repos is relatively small
	//	for _, r := range response.List {
	//		// Copy over the additional details
	//		if ourGitHubRepo.RepositoryName == r.RepositoryName {
	//			r.RepositoryID = ourGitHubRepo.RepositoryID
	//			r.Enabled = ourGitHubRepo.Enabled
	//			r.DateCreated = ourGitHubRepo.DateCreated
	//			r.DateModified = ourGitHubRepo.DateModified
	//			r.Note = ourGitHubRepo.Note
	//			r.Version = ourGitHubRepo.Version
	//			break
	//		}
	//	}
	//}
	//
	//return response, nil
}

// GitHubGetRepository service function
func (s *Service) GitHubGetRepository(ctx context.Context, repositoryID string) (*v1Models.GithubRepository, error) {
	return s.gitV1Repository.GitHubGetRepository(ctx, repositoryID)
}

// GitHubGetRepositoryByName service function
func (s *Service) GitHubGetRepositoryByName(ctx context.Context, repositoryName string) (*v1Models.GithubRepository, error) {
	return s.gitV1Repository.GitHubGetRepositoryByName(ctx, repositoryName)
}

// GitHubGetRepositoryByExternalID service function
func (s *Service) GitHubGetRepositoryByExternalID(ctx context.Context, repositoryExternalID string) (*v1Models.GithubRepository, error) {
	return s.gitV1Repository.GitHubGetRepositoryByExternalID(ctx, repositoryExternalID)
}

// GitHubGetProtectedBranch service function
func (s *Service) GitHubGetProtectedBranch(ctx context.Context, projectSFID, repositoryID, branchName string) (*v2Models.GithubRepositoryBranchProtection, error) {
	f := logrus.Fields{
		"functionName":   "v2.repositories.service.GitHubGetProtectedBranch",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"repositoryID":   repositoryID,
		"branchName":     branchName,
	}

	githubRepository, err := s.getGithubRepo(ctx, projectSFID, repositoryID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("fetching repository %s, failed, error: %v", repositoryID, err)
		return nil, err
	}

	githubOrgName := githubRepository.RepositoryOrganizationName
	githubRepoName := githubRepository.RepositoryName
	githubRepoName = branch_protection.CleanGithubRepoName(githubRepoName)

	branchProtectionRepository, err := s.getBranchProtectionRepositoryForOrgName(ctx, githubOrgName)
	if err != nil {
		return nil, err
	}

	owner, err := s.getGithubOwner(ctx, branchProtectionRepository, githubOrgName, githubRepoName)
	if err != nil {
		return nil, err
	}

	result := &v2Models.GithubRepositoryBranchProtection{
		BranchName: &branchName,
	}

	branchProtection, err := branchProtectionRepository.GetProtectedBranch(ctx, owner, githubRepoName, branchName)
	if err != nil {
		if errors.Is(err, branch_protection.ErrBranchNotProtected) {
			return result, nil
		}
		log.WithFields(f).WithError(err).Warnf("getting the github protected branch for owner : %s, gitV1Repository : %s and branch : %s failed : %v", owner, githubRepoName, branchName, err)
		return nil, err
	}

	result.ProtectionEnabled = true
	if branch_protection.IsEnforceAdminEnabled(branchProtection) {
		result.EnforceAdmin = true
	}

	requiredChecks := requiredBranchProtectionChecks
	requiredChecksResult := s.getRequiredProtectedBranchCheckStatus(branchProtection, requiredChecks)
	result.StatusChecks = requiredChecksResult

	return result, nil
}

// GitHubUpdateProtectedBranch service function
func (s *Service) GitHubUpdateProtectedBranch(ctx context.Context, projectSFID, repositoryID string, input *v2Models.GithubRepositoryBranchProtectionInput) (*v2Models.GithubRepositoryBranchProtection, error) {
	f := logrus.Fields{
		"functionName":   "v2.repositories.service.GitHubUpdateProtectedBranch",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"repositoryID":   repositoryID,
		"enforceAdmin":   aws.BoolValue(input.EnforceAdmin),
	}

	githubRepository, err := s.getGithubRepo(ctx, projectSFID, repositoryID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("fetching repository %s, failed", repositoryID)
		return nil, err
	}

	githubOrgName := githubRepository.RepositoryOrganizationName
	githubRepoName := githubRepository.RepositoryName
	githubRepoName = branch_protection.CleanGithubRepoName(githubRepoName)

	branchProtectionRepository, err := s.getBranchProtectionRepositoryForOrgName(ctx, githubOrgName)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem locating github client for organization name")
		return nil, err
	}

	branchName := input.BranchName
	if branchName == "" {
		branchName = branch_protection.DefaultBranchName
	}

	owner, err := s.getGithubOwner(ctx, branchProtectionRepository, githubOrgName, githubRepoName)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem locating github owner branch name")
		return nil, err
	}
	f["owner"] = owner
	f["branchName"] = input.BranchName

	var requiredChecks []string
	var disabledChecks []string
	if input.StatusChecks != nil {
		for _, inputCheck := range input.StatusChecks {
			// we want to make sure we only mutate checks related to lf
			var found bool
			for _, rc := range requiredBranchProtectionChecks {
				if rc == *inputCheck.Name {
					found = true
					break
				}
			}

			// just ignore that check if it's something not in our options
			if !found {
				log.WithFields(f).Warnf("invalid branch protection option was found : %s", *inputCheck.Name)
				return nil, ErrInvalidBranchProtectionName
			}

			if !*inputCheck.Enabled {
				disabledChecks = append(disabledChecks, *inputCheck.Name)
				continue
			}
			requiredChecks = append(requiredChecks, *inputCheck.Name)
		}
	}

	log.WithFields(f).Debugf("enabling branch protection on repository...")
	err = branchProtectionRepository.EnableBranchProtection(ctx, owner, githubRepoName, branchName, *input.EnforceAdmin, requiredChecks, disabledChecks)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem enabling github branch protection")
		return nil, err
	}

	return s.GitHubGetProtectedBranch(ctx, projectSFID, repositoryID, branchName)
}

// getGithubRepo service function
func (s *Service) getGithubRepo(ctx context.Context, projectSFID, repositoryID string) (*v1Models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "v2.repositories.service.getGitHubRepo",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"repositoryID":   repositoryID,
	}

	psc := v2ProjectService.GetClient()
	_, err := psc.GetProject(projectSFID)
	if err != nil {
		return nil, err
	}
	githubRepository, err := s.GitHubGetRepository(ctx, repositoryID)
	if err != nil {
		log.WithFields(f).Warnf("fetching repository failed : %s : %v", repositoryID, err)
		return nil, err
	}

	// check if project and gitV1Repository are actually associated
	if githubRepository.RepositoryProjectSfid != projectSFID {
		msg := fmt.Sprintf("github repository %s doesn't belong to project : %s", repositoryID, projectSFID)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	return githubRepository, nil
}

// getBranchProtectionRepositoryForOrgName service function
func (s *Service) getBranchProtectionRepositoryForOrgName(ctx context.Context, githubOrgName string) (*branch_protection.BranchProtectionRepository, error) {
	f := logrus.Fields{
		"functionName":   "v2.repositories.service.getGitHubClientForOrgName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"githubOrgName":  githubOrgName,
	}

	githubOrg, err := s.ghOrgRepo.GetGitHubOrganizationByName(ctx, githubOrgName)
	if err != nil {
		log.WithFields(f).Warnf("fetching githubOrg %s failed, error: %v", githubOrgName, err)
		return nil, err
	}
	if len(githubOrg.List) == 0 {
		return nil, errors.New("github app not installed on github organization")
	}

	branchProtectionRepo, err := branch_protection.NewBranchProtectionRepository(githubOrg.List[0].OrganizationInstallationID, branch_protection.EnableNonBlockingLimiter())
	if err != nil {
		return nil, err
	}
	return branchProtectionRepo, nil
}

// getGithubOwner service function
func (s *Service) getGithubOwner(ctx context.Context, branchProtectionRepository *branch_protection.BranchProtectionRepository, githubOrgName, githubRepoName string) (string, error) {
	owner, err := branchProtectionRepository.GetOwnerName(ctx, githubOrgName, githubRepoName)
	if err != nil {
		log.Warnf("getting the owner name for org : %s and gitV1Repository : %s failed : %v", githubOrgName, githubRepoName, err)
		return "", err
	}

	if owner == "" {
		log.Warnf("GitHub returned empty owner name for org : %s and gitV1Repository : %s", githubOrgName, githubRepoName)
		return "", fmt.Errorf("empty owner name")
	}
	return owner, nil
}

// getRequiredProtectedBranchCheckStatus service function to get the required protected branch check status
func (s *Service) getRequiredProtectedBranchCheckStatus(branchProtectionRule *branch_protection.BranchProtectionRule, requiredChecks []string) []*v2Models.GithubRepositoryBranchProtectionStatusChecks {
	f := logrus.Fields{
		"functionName": "v2.repositories.service.getRequiredProtectedBranchCheckStatus",
	}

	log.WithFields(f).Debug("querying GitHub for status checks...")
	var result []*v2Models.GithubRepositoryBranchProtectionStatusChecks
	resultMap := map[string]bool{}
	for _, rc := range requiredChecks {
		result = append(result, &v2Models.GithubRepositoryBranchProtectionStatusChecks{
			Name:    swag.String(rc),
			Enabled: swag.Bool(false),
		})
		resultMap[rc] = true
	}
	if len(branchProtectionRule.RequiredStatusCheckContexts) == 0 {
		return result
	}

	for _, existingCheck := range branchProtectionRule.RequiredStatusCheckContexts {
		if !resultMap[existingCheck] {
			continue
		}

		// we mark it as enabled in this case
		for i := range result {
			if *result[i].Name == existingCheck {
				result[i].Enabled = swag.Bool(true)
			}
		}
	}
	return result
}

// GitHubDisableCLAGroupRepositories service function to disable CLA group repositories
func (s *Service) GitHubDisableCLAGroupRepositories(ctx context.Context, claGroupID string) error {
	f := logrus.Fields{
		"functionName":   "v2.repositories.service.GitHubDisableCLAGroupRepositories",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
	}

	var deleteErr error
	ghOrgs, err := s.gitV1Repository.GitHubGetCLAGroupRepositoriesGroupByOrgs(ctx, claGroupID, true)
	if err != nil {
		return err
	}
	if len(ghOrgs) > 0 {
		log.WithFields(f).Debugf("Deleting repositories for cla-group :%s", claGroupID)
		for _, ghOrg := range ghOrgs {
			for _, item := range ghOrg.List {
				deleteErr = s.gitV1Repository.GitHubDisableRepository(ctx, item.RepositoryID)
				if deleteErr != nil {
					log.WithFields(f).Warnf("Unable to remove repository: %s for project :%s error :%v", item.RepositoryID, claGroupID, deleteErr)
				}
			}
		}
	}
	return nil
}

// GetRepositoriesByProjectSFID service function
func (s *Service) GetRepositoriesByProjectSFID(ctx context.Context, projectSFID string) ([]*v1Repositories.RepositoryDBModel, error) {
	data, err := s.gitV2Repository.GetRepositoriesByProjectSFID(ctx, projectSFID)
	if err != nil {
		return nil, err
	}
	return data, nil
}
