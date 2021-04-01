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
		if _, err := s.claRepository.GetCLAGroupNameByID(input.AutoEnabledClaGroupID); err != nil {
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

	log.WithFields(f).Debug("factoring github orgs for parent...")
	// Lookup the parent
	parentProjectSFID, projErr := v2ProjectService.GetClient().GetParentProject(projectSFID)
	if projErr != nil {
		log.WithFields(f).Warnf("problem fetching project parent SFID, error: %+v", projErr)
		return nil, projErr
	}

	if parentProjectSFID != projectSFID {
		log.WithFields(f).Debugf("searching github organization by parent SFID: %s", parentProjectSFID)
		parentGithubModels, parentErr := s.repo.GetGithubOrganizationsByParent(ctx, parentProjectSFID)
		if parentErr != nil {
			log.WithFields(f).Warnf("problem fetching github organizations by paarent projectSFID: %s , error: %+v", parentProjectSFID, err)
			return nil, parentErr
		}

		if len(parentGithubModels.List) >= 0 {
			githubOrgs = append(githubOrgs, parentGithubModels.List...)
		}
	}

	log.WithFields(f).Debugf("no parent or parent is %s or %s - search criteria exhausted",
		utils.TheLinuxFoundation, utils.TheLinuxFoundation)

	gitHubOrgModels.List = githubOrgs
	// Remove potential duplicates
	s.removeDuplicateGHOrgs(gitHubOrgModels.List)

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
		if _, err := s.claRepository.GetCLAGroupNameByID(autoEnabledClaGroupID); err != nil {
			return err
		}
	}
	return s.repo.UpdateGithubOrganization(ctx, projectSFID, organizationName, autoEnabled, autoEnabledClaGroupID, branchProtectionEnabled)
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

// filter ghOrgs duplicates
func (s service) removeDuplicateGHOrgs(input []*models.GithubOrganization) []*models.GithubOrganization {
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
