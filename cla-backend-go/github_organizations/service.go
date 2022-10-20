// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_organizations

import (
	"context"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	v2ProjectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"

	"github.com/sirupsen/logrus"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/repositories"
)

// ServiceInterface contains functions of GithubOrganizations service
type ServiceInterface interface {
	AddGitHubOrganization(ctx context.Context, projectSFID string, input *models.GithubCreateOrganization) (*models.GithubOrganization, error)
	GetGitHubOrganizations(ctx context.Context, projectSFID string) (*models.GithubOrganizations, error)
	GetGitHubOrganizationsByParent(ctx context.Context, parentProjectSFID string) (*models.GithubOrganizations, error)
	GetGitHubOrganizationByName(ctx context.Context, githubOrgName string) (*models.GithubOrganization, error)
	UpdateGitHubOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error
	DeleteGitHubOrganization(ctx context.Context, projectSFID string, githubOrgName string) error
	RemoveDuplicates(input []*models.GithubOrganization) []*models.GithubOrganization
}

// Service object/struct
type Service struct {
	repo          RepositoryInterface
	ghRepository  repositories.RepositoryInterface
	claRepository projects_cla_groups.Repository
}

// NewService creates a new githubOrganizations service
func NewService(repo RepositoryInterface, ghRepository repositories.RepositoryInterface, claRepository projects_cla_groups.Repository) Service {
	return Service{
		repo:          repo,
		ghRepository:  ghRepository,
		claRepository: claRepository,
	}
}

// AddGitHubOrganization adds the GitHub organization for the specified project
func (s Service) AddGitHubOrganization(ctx context.Context, projectSFID string, input *models.GithubCreateOrganization) (*models.GithubOrganization, error) {
	f := logrus.Fields{
		"functionName":            "AddGitHubOrganization",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"projectSFID":             projectSFID,
		"organizationName":        input.OrganizationName,
		"autoEnabled":             input.AutoEnabled,
		"branchProtectionEnabled": input.BranchProtectionEnabled,
	}
	// Lookup the parent
	parentProjectSFID, projErr := v2ProjectService.GetClient().GetParentProject(projectSFID)
	if projErr != nil {
		log.WithFields(f).Warnf("problem fetching github organizations by projectSFID, error: %+v", projErr)
		return nil, projErr
	}
	if parentProjectSFID == "" {
		parentProjectSFID = projectSFID
	}

	// check if valid cla group id is passed
	if input.AutoEnabledClaGroupID != "" {
		if _, err := s.claRepository.GetCLAGroupNameByID(ctx, input.AutoEnabledClaGroupID); err != nil {
			return nil, err
		}
	}

	return s.repo.AddGitHubOrganization(ctx, parentProjectSFID, projectSFID, input)
}

// GetGitHubOrganizations returns the GitHub organization for the specified project
func (s Service) GetGitHubOrganizations(ctx context.Context, projectSFID string) (*models.GithubOrganizations, error) {
	f := logrus.Fields{
		"functionName":   "GetGitHubOrganizations",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
	}

	// track githubOrgs based on parent/child anchor
	var gitHubOrgModels = models.GithubOrganizations{}
	var githubOrgs = make([]*models.GithubOrganization, 0)

	projectGithubModels, err := s.repo.GetGitHubOrganizations(ctx, projectSFID)
	if err != nil {
		log.WithFields(f).Warnf("problem fetching github organizations by projectSFID, error: %+v", err)
		return nil, err
	}

	if len(projectGithubModels.List) >= 0 {
		githubOrgs = append(githubOrgs, projectGithubModels.List...)
	}
	log.WithFields(f).Debugf("loaded %d GitHub organizations using projectSFID: %s", len(projectGithubModels.List), projectSFID)

	// Lookup the parent
	log.WithFields(f).Debugf("looking up parent for projectSFID: %s...", projectSFID)
	parentProjectSFID, projErr := v2ProjectService.GetClient().GetParentProject(projectSFID)
	if projErr != nil {
		log.WithFields(f).Warnf("problem fetching project parent SFID, error: %+v", projErr)
		return nil, projErr
	}
	if parentProjectSFID == "" {
		parentProjectSFID = projectSFID
	}

	//Get SF Project
	projectDetails, projDetailsErr := v2ProjectService.GetClient().GetProject(projectSFID)
	if projDetailsErr != nil {
		log.WithFields(f).Warnf("problem fetching parent project details for :%s ", projectSFID)
		return nil, projDetailsErr
	}

	if parentProjectSFID != projectSFID && (projectDetails != nil && !utils.IsProjectHasRootParent(projectDetails)) {
		log.WithFields(f).Debugf("found parent of projectSFID: %s to be %s. Searching github organization by parent SFID: %s...", projectSFID, parentProjectSFID, parentProjectSFID)
		// parentGithubModels, parentErr := s.repo.GetGitHubOrganizationsByParent(ctx, parentProjectSFID)
		parentGithubModels, parentErr := s.repo.GetGitHubOrganizations(ctx, parentProjectSFID)
		if parentErr != nil {
			log.WithFields(f).Warnf("problem fetching github organizations by paarent projectSFID: %s , error: %+v", parentProjectSFID, err)
			return nil, parentErr
		}

		if len(parentGithubModels.List) >= 0 {
			githubOrgs = append(githubOrgs, parentGithubModels.List...)
		}
		log.WithFields(f).Debugf("loaded %d GitHub organizations using projectSFID: %s", len(parentGithubModels.List), parentProjectSFID)
	}

	gitHubOrgModels.List = githubOrgs

	// Remove potential duplicates
	gitHubOrgModels.List = s.RemoveDuplicates(gitHubOrgModels.List)

	return &gitHubOrgModels, err
}

// GetGitHubOrganizationsByParent returns the GitHub organizations for the specified parent project SFID
func (s Service) GetGitHubOrganizationsByParent(ctx context.Context, parentProjectSFID string) (*models.GithubOrganizations, error) {
	return s.repo.GetGitHubOrganizationsByParent(ctx, parentProjectSFID)
}

// GetGitHubOrganizationByName returns the GitHub organizations for the specified GitHub organization name
func (s Service) GetGitHubOrganizationByName(ctx context.Context, githubOrgName string) (*models.GithubOrganization, error) {
	f := logrus.Fields{
		"functionName":   "GetGitHubOrganizationByName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"githubOrgName":  githubOrgName,
	}

	gitHubOrgs, err := s.repo.GetGitHubOrganizationByName(ctx, githubOrgName)
	if err != nil {
		log.WithFields(f).Warnf("problem fetching github organizations by name, error: %+v", err)
		return nil, err
	}
	if len(gitHubOrgs.List) == 0 {
		log.WithFields(f).Debugf("no matching github organization matches organization name: %s", githubOrgName)
		return nil, nil
	}

	if len(gitHubOrgs.List) > 1 {
		log.WithFields(f).Warnf("More than 1 github organization matches organization name: %s - using first one", githubOrgName)
	}

	return gitHubOrgs.List[0], err
}

// UpdateGitHubOrganization updates the specified github organization based on the project SFID, organization name provided values
func (s Service) UpdateGitHubOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error {
	// check if valid cla group id is passed
	if autoEnabledClaGroupID != "" {
		if _, err := s.claRepository.GetCLAGroupNameByID(ctx, autoEnabledClaGroupID); err != nil {
			return err
		}
	}
	return s.repo.UpdateGitHubOrganization(ctx, projectSFID, organizationName, autoEnabled, autoEnabledClaGroupID, branchProtectionEnabled, nil)
}

// DeleteGitHubOrganization removes the specified github organization under the projectSFID
func (s Service) DeleteGitHubOrganization(ctx context.Context, projectSFID string, githubOrgName string) error {
	f := logrus.Fields{
		"functionName":   "DeleteGitHubOrganization",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"githubOrgName":  githubOrgName,
	}

	// Lookup the parent
	parentProjectSFID, projErr := v2ProjectService.GetClient().GetParentProject(projectSFID)
	if projErr != nil {
		log.WithFields(f).Warnf("problem fetching project parent SFID, error: %+v", projErr)
		return projErr
	}
	if parentProjectSFID == "" {
		parentProjectSFID = projectSFID
	}

	err := s.ghRepository.GitHubDisableRepositoriesOfOrganization(ctx, parentProjectSFID, githubOrgName)
	if err != nil {
		log.WithFields(f).Warnf("problem disabling repositories for github organizations, error: %+v", projErr)
		return err
	}

	return s.repo.DeleteGitHubOrganization(ctx, projectSFID, githubOrgName)
}

// RemoveDuplicates removes any duplicates from the specified list
func (s Service) RemoveDuplicates(input []*models.GithubOrganization) []*models.GithubOrganization {
	if input == nil {
		return nil
	}
	keys := make(map[string]bool)

	output := []*models.GithubOrganization{}
	for _, ghOrg := range input {
		if _, value := keys[ghOrg.OrganizationName]; !value {
			keys[ghOrg.OrganizationName] = true
			output = append(output, ghOrg)
		}
	}

	return output
}
