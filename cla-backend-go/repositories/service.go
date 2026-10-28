// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	project_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
)

// Service contains functions of GitHub Repository service
type Service interface {
	AddGithubRepository(ctx context.Context, externalProjectID string, input *models.GithubRepositoryInput) (*models.GithubRepository, error)
	EnableRepository(ctx context.Context, repositoryID string) error
	EnableRepositoryWithCLAGroupID(ctx context.Context, repositoryID, claGroupID string) error
	DisableRepository(ctx context.Context, repositoryID string) error
	UpdateClaGroupID(ctx context.Context, repositoryID, claGroupID string) error
	ListProjectRepositories(ctx context.Context, externalProjectID string, enabled *bool) (*models.GithubListRepositories, error)
	GetRepository(ctx context.Context, repositoryID string) (*models.GithubRepository, error)
	GetRepositoryByProjectSFID(ctx context.Context, projectSFID string, enabled *bool) (*models.GithubListRepositories, error)
	GetRepositoryByName(ctx context.Context, repositoryName string) (*models.GithubRepository, error)
	GetRepositoryByExternalID(ctx context.Context, repositoryExternalID string) (*models.GithubRepository, error)
	DisableRepositoriesByProjectID(ctx context.Context, projectID string) (int, error)
	GetRepositoriesByCLAGroup(ctx context.Context, claGroupID string) ([]*models.GithubRepository, error)
	GetRepositoriesByOrganizationName(ctx context.Context, gitHubOrgName string) ([]*models.GithubRepository, error)
}

// GithubOrgRepo provide method to get github organization by name
type GithubOrgRepo interface {
	GetGitHubOrganizationByName(ctx context.Context, githubOrganizationName string) (*models.GithubOrganizations, error)
	GetGitHubOrganization(ctx context.Context, githubOrganizationName string) (*models.GithubOrganization, error)
	GetGitHubOrganizations(ctx context.Context, projectSFID string) (*models.GithubOrganizations, error)
}

type service struct {
	repo                  RepositoryInterface
	ghOrgRepo             GithubOrgRepo
	projectsClaGroupsRepo projects_cla_groups.Repository
}

// NewService creates a new githubOrganizations service
func NewService(repo RepositoryInterface, ghOrgRepo GithubOrgRepo, pcgRepo projects_cla_groups.Repository) Service {
	return &service{
		repo:                  repo,
		ghOrgRepo:             ghOrgRepo,
		projectsClaGroupsRepo: pcgRepo,
	}
}

// UpdateClaGroupID updates the claGroupID
func (s *service) UpdateClaGroupID(ctx context.Context, repositoryID, claGroupID string) error {
	return s.repo.GitHubUpdateClaGroupID(ctx, repositoryID, claGroupID)
}

func (s *service) AddGithubRepository(ctx context.Context, externalProjectID string, input *models.GithubRepositoryInput) (*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":               "AddGitHubRepository",
		utils.XREQUESTID:             ctx.Value(utils.XREQUESTID),
		"projectSFID":                externalProjectID,
		"claGroupID":                 utils.StringValue(input.RepositoryProjectID),
		"repositoryName":             utils.StringValue(input.RepositoryName),
		"repositoryOrganizationName": utils.StringValue(input.RepositoryOrganizationName),
		"repositoryType":             utils.StringValue(input.RepositoryType),
		"repositoryProjectID":        utils.StringValue(input.RepositoryProjectID),
		"repositoryURL":              utils.StringValue(input.RepositoryURL),
	}
	if input.RepositoryName != nil && *input.RepositoryName == "" {
		return nil, errors.New("github repository name required")
	}
	projectSFID := externalProjectID
	// Check if project exists in project service
	psc := project_service.GetClient()
	project, projectErr := psc.GetProject(projectSFID)
	if projectErr != nil || project == nil {
		msg := fmt.Sprintf("Failed to get salesforce project: %s", projectSFID)
		log.WithFields(f).Warn(msg)
		return nil, projectErr
	}

	org, err := s.ghOrgRepo.GetGitHubOrganizationByName(ctx, utils.StringValue(input.RepositoryOrganizationName))
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem loading github organization by name: %s", utils.StringValue(input.RepositoryOrganizationName))
		return nil, err
	}
	if len(org.List) == 0 {
		log.WithFields(f).Warnf("github app not installed on github organization: %s", utils.StringValue(input.RepositoryOrganizationName))
		return nil, errors.New("github app not installed on github organization")
	}
	repoGithubID, err := strconv.ParseInt(utils.StringValue(input.RepositoryExternalID), 10, 64)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem converting repository external ID - should be an integer value: %s", utils.StringValue(input.RepositoryExternalID))
		return nil, err
	}
	ghRepo, err := github.GetRepositoryByExternalID(ctx, org.List[0].OrganizationInstallationID, repoGithubID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem loading repository by organization installation ID: %d and repo github id: %d", org.List[0].OrganizationInstallationID, repoGithubID)
		return nil, err
	}

	log.Debugf("ghRepo.HTMLURL %s, input.RepositoryURL  %s", *ghRepo.HTMLURL, *input.RepositoryURL)
	if !strings.EqualFold(*ghRepo.HTMLURL, *input.RepositoryURL) {
		return nil, errors.New("github repository not found")
	}

	// Check to see if the repository already exists...
	existingModel, err := s.GetRepositoryByName(ctx, utils.StringValue(input.RepositoryName))
	if err != nil {
		// If not found - ok, otherwise we have a bigger problem
		if notFoundErr, ok := err.(*utils.GitHubRepositoryNotFound); ok {
			log.WithFields(f).WithError(notFoundErr).Debug("existing repository not found - will create")
		} else {
			return nil, err
		}
	}

	if existingModel != nil {
		// Never re-bind a repository record that was registered under another project - the caller is only
		// authorized for projectSFID (lfx-self-serve-ops#751)
		if bindErr := repositoryBoundToOtherProject(existingModel, projectSFID); bindErr != nil {
			log.WithFields(f).WithError(bindErr).Warn("existing repository belongs to another project - refusing to re-bind it")
			return nil, bindErr
		}
		log.WithFields(f).Debug("existing repository found - enabling it...")
		enableErr := s.EnableRepositoryWithCLAGroupID(ctx, existingModel.RepositoryID, utils.StringValue(input.RepositoryProjectID))
		if enableErr != nil {
			log.WithFields(f).WithError(enableErr).Warn("problem enabling repository")
			return nil, enableErr
		}

		return s.repo.GitHubGetRepository(ctx, existingModel.RepositoryID)
	}

	// GitHubAddRepository re-uses (and re-binds) a disabled record found by external ID even when the supplied name
	// differs from the stored one, so apply the same project-binding guard to that record (lfx-self-serve-ops#751)
	existingByExternalID, err := s.repo.GitHubGetRepositoryByExternalID(ctx, utils.StringValue(input.RepositoryExternalID))
	if guardErr := externalIDRecordReusable(existingByExternalID, err, utils.StringValue(input.RepositoryExternalID), projectSFID); guardErr != nil {
		log.WithFields(f).WithError(guardErr).Warn("repository with this external ID cannot be re-used for this project - refusing to add it")
		return nil, guardErr
	}

	// Doesn't exist - create it
	return s.repo.GitHubAddRepository(ctx, externalProjectID, projectSFID, input)
}

// externalIDRecordReusable validates the GitHubGetRepositoryByExternalID result before GitHubAddRepository may re-use
// the record: not-found means there is nothing to re-bind, any other lookup error is returned as-is, a record that
// could not be converted (nil without error, e.g. a non-numeric stored external ID) is refused rather than silently
// re-bound, and a record bound to another project is refused by repositoryBoundToOtherProject.
func externalIDRecordReusable(existing *models.GithubRepository, lookupErr error, externalID, projectSFID string) error {
	if lookupErr != nil {
		if _, ok := lookupErr.(*utils.GitHubRepositoryNotFound); ok {
			return nil
		}
		return lookupErr
	}
	if existing == nil {
		return fmt.Errorf("github repository with external id %s has an invalid stored record", externalID)
	}
	return repositoryBoundToOtherProject(existing, projectSFID)
}

// repositoryBoundToOtherProject returns an error when the stored repository record carries a project binding that
// differs from the project the caller is authorized for; records without any project binding are left untouched.
// project_sfid is the owning project; repository_sfdc_id is only consulted when project_sfid is empty because v2
// stores the parent foundation there (v2/repositories/service.go passes the foundation as externalProjectID)
func repositoryBoundToOtherProject(existing *models.GithubRepository, projectSFID string) error {
	if existing == nil {
		return nil
	}
	boundProjectSFID := existing.RepositoryProjectSfid
	if boundProjectSFID == "" {
		boundProjectSFID = existing.RepositorySfdcID
	}
	if boundProjectSFID != "" && boundProjectSFID != projectSFID {
		return fmt.Errorf("github repository %s is already registered under project %s and cannot be added to project %s",
			existing.RepositoryName, boundProjectSFID, projectSFID)
	}
	return nil
}

func (s *service) EnableRepository(ctx context.Context, repositoryID string) error {
	return s.repo.GitHubEnableRepository(ctx, repositoryID)
}

func (s *service) EnableRepositoryWithCLAGroupID(ctx context.Context, repositoryID, claGroupID string) error {
	return s.repo.GitHubEnableRepositoryWithCLAGroupID(ctx, repositoryID, claGroupID)
}

func (s *service) DisableRepository(ctx context.Context, repositoryID string) error {
	return s.repo.GitHubDisableRepository(ctx, repositoryID)
}

func (s *service) ListProjectRepositories(ctx context.Context, externalProjectID string, enabled *bool) (*models.GithubListRepositories, error) {
	return s.repo.GitHubListProjectRepositories(ctx, externalProjectID, enabled)
}

func (s *service) GetRepository(ctx context.Context, repositoryID string) (*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName": "v1.repository.GitHubGetRepository",
		"repositoryID": repositoryID,
	}
	log.WithFields(f).Debug("Searching for repository...")
	ghRepo, err := s.repo.GitHubGetRepository(ctx, repositoryID)
	if err != nil || ghRepo != nil {
		log.WithFields(f).WithError(err).Debug("unable to get repository")
		return nil, err
	}

	log.WithFields(f).Debugf("Found repository : %+v ", ghRepo)

	return ghRepo, nil
}

func (s *service) GetRepositoryByProjectSFID(ctx context.Context, projectSFID string, enabled *bool) (*models.GithubListRepositories, error) {
	return s.repo.GitHubListProjectRepositories(ctx, projectSFID, enabled)
}

// GetRepositoryByName returns the repository by name: project-level/cla-project
func (s *service) GetRepositoryByName(ctx context.Context, repositoryName string) (*models.GithubRepository, error) {
	return s.repo.GitHubGetRepositoryByName(ctx, repositoryName)
}

// GetRepositoryByExternalID returns the repository by externalID
func (s *service) GetRepositoryByExternalID(ctx context.Context, repositoryExternalID string) (*models.GithubRepository, error) {
	return s.repo.GitHubGetRepositoryByExternalID(ctx, repositoryExternalID)
}

// DisableRepositoriesByProjectID disables the repositories by project ID
func (s *service) DisableRepositoriesByProjectID(ctx context.Context, projectID string) (int, error) {
	var deleteErr error
	// Return the list of GitHub repositories by CLA Group for those that are currently enabled
	ghOrgs, err := s.repo.GitHubGetCLAGroupRepositoriesGroupByOrgs(ctx, projectID, true)
	if err != nil {
		return 0, err
	}
	if len(ghOrgs) > 0 {
		log.Debugf("Deleting repositories for project :%s", projectID)
		for _, ghOrg := range ghOrgs {
			for _, item := range ghOrg.List {
				deleteErr = s.repo.GitHubDisableRepository(ctx, item.RepositoryID)
				if deleteErr != nil {
					log.Warnf("Unable to remove repository: %s for project :%s error :%v", item.RepositoryID, projectID, deleteErr)
				}
			}
		}
	}

	return len(ghOrgs), nil
}

// GetRepositoriesByCLAGroup returns the list of repositories for the specified CLA Group
func (s *service) GetRepositoriesByCLAGroup(ctx context.Context, claGroupID string) ([]*models.GithubRepository, error) {
	// Return the list of github repositories that are enabled
	return s.repo.GitHubGetRepositoriesByCLAGroup(ctx, claGroupID, true)
}

// GetRepositoriesByOrganizationName get repositories by organization name
func (s *service) GetRepositoriesByOrganizationName(ctx context.Context, gitHubOrgName string) ([]*models.GithubRepository, error) {
	return s.repo.GitHubGetRepositoriesByOrganizationName(ctx, gitHubOrgName)
}

// RepositoryBoundToOtherProject exposes repositoryBoundToOtherProject to the v4 enrollment path (lfx-self-serve-ops#982, #747).
func RepositoryBoundToOtherProject(existing *models.GithubRepository, projectSFID string) error {
	return repositoryBoundToOtherProject(existing, projectSFID)
}

// ExternalIDRecordReusable exposes externalIDRecordReusable to the v4 enrollment path (lfx-self-serve-ops#982, #747).
func ExternalIDRecordReusable(existing *models.GithubRepository, lookupErr error, externalID, projectSFID string) error {
	return externalIDRecordReusable(existing, lookupErr, externalID, projectSFID)
}
