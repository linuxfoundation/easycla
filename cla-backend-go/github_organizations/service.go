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

	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
	"github.com/communitybridge/easycla/cla-backend-go/repositories"
)

// Service contains functions of GithubOrganizations service
type Service interface {
	AddGithubOrganization(ctx context.Context, projectSFID string, input *models.CreateGithubOrganization) (*models.GithubOrganization, error)
	GetGithubOrganizations(ctx context.Context, projectSFID string) (*models.GithubOrganizations, error)
	GetGithubOrganizationsByParent(ctx context.Context, parentProjectSFID string) (*models.GithubOrganizations, error)
	GetGithubOrganizationByName(ctx context.Context, githubOrgName string) (*models.GithubOrganization, error)
	UpdateGithubOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error
	DeleteGithubOrganization(ctx context.Context, projectSFID string, githubOrgName string) error
	RemoveDuplicates(input []*models.GithubOrganization) []*models.GithubOrganization
}

type service struct {
	repo          Repository
	ghRepository  repositories.Repository
	claRepository projects_cla_groups.Repository
}

// NewService creates a new githubOrganizations service
func NewService(repo Repository, ghRepository repositories.Repository, claRepository projects_cla_groups.Repository) Service {
	return service{
		repo:          repo,
		ghRepository:  ghRepository,
		claRepository: claRepository,
	}
}

func (s service) AddGithubOrganization(ctx context.Context, projectSFID string, input *models.CreateGithubOrganization) (*models.GithubOrganization, error) {
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

	// check if valid cla group id is passed
	if input.AutoEnabledClaGroupID != "" {
		if _, err := s.claRepository.GetCLAGroupNameByID(ctx, input.AutoEnabledClaGroupID); err != nil {
			return nil, err
		}
	}

	return s.repo.AddGithubOrganization(ctx, parentProjectSFID, projectSFID, input)
}

func (s service) GetGithubOrganizations(ctx context.Context, projectSFID string) (*models.GithubOrganizations, error) {
	f := logrus.Fields{
		"functionName":   "GetGitHubOrganizations",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
	}

	// track githubOrgs based on parent/child anchor
	var gitHubOrgModels = models.GithubOrganizations{}
	var githubOrgs = make([]*models.GithubOrganization, 0)

	projectGithubModels, err := s.repo.GetGithubOrganizations(ctx, projectSFID)
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

	//Get SF Project
	projectDetails, projDetailsErr := v2ProjectService.GetClient().GetProject(projectSFID)
	if projDetailsErr != nil {
		log.WithFields(f).Warnf("problem fetching parent project details for :%s ", projectSFID)
		return nil, projDetailsErr
	}

	if parentProjectSFID != projectSFID && (projectDetails != nil && !utils.IsProjectHasRootParent(projectDetails)) {
		log.WithFields(f).Debugf("found parent of projectSFID: %s to be %s. Searching github organization by parent SFID: %s...", projectSFID, parentProjectSFID, parentProjectSFID)
		parentGithubModels, parentErr := s.repo.GetGithubOrganizationsByParent(ctx, parentProjectSFID)
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

func (s service) GetGithubOrganizationsByParent(ctx context.Context, parentProjectSFID string) (*models.GithubOrganizations, error) {
	return s.repo.GetGithubOrganizationsByParent(ctx, parentProjectSFID)
}

func (s service) GetGithubOrganizationByName(ctx context.Context, githubOrgName string) (*models.GithubOrganization, error) {
	f := logrus.Fields{
		"functionName":   "GetGitHubOrganizationByName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"githubOrgName":  githubOrgName,
	}

	gitHubOrgs, err := s.repo.GetGithubOrganizationByName(ctx, githubOrgName)
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

func (s service) UpdateGithubOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error {
	// check if valid cla group id is passed
	if autoEnabledClaGroupID != "" {
		if _, err := s.claRepository.GetCLAGroupNameByID(ctx, autoEnabledClaGroupID); err != nil {
			return err
		}
	}
	return s.repo.UpdateGithubOrganization(ctx, projectSFID, organizationName, autoEnabled, autoEnabledClaGroupID, branchProtectionEnabled, nil)
}

func (s service) DeleteGithubOrganization(ctx context.Context, projectSFID string, githubOrgName string) error {
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

	err := s.ghRepository.DisableRepositoriesOfGithubOrganization(ctx, parentProjectSFID, githubOrgName)
	if err != nil {
		log.WithFields(f).Warnf("problem disabling repositories for github organizations, error: %+v", projErr)
		return err
	}

	return s.repo.DeleteGithubOrganization(ctx, projectSFID, githubOrgName)
}

// RemoveDuplicates removes any duplicates from the specified list
func (s service) RemoveDuplicates(input []*models.GithubOrganization) []*models.GithubOrganization {
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
