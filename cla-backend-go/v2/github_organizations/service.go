// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_organizations

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/sirupsen/logrus"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	v1GithubOrg "github.com/communitybridge/easycla/cla-backend-go/github_organizations"
	v1Repositories "github.com/communitybridge/easycla/cla-backend-go/repositories"
	v2ProjectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"
	"github.com/jinzhu/copier"
)

func v2GithubOrganizationModel(in *v1Models.GithubOrganization) (*models.GithubOrganization, error) {
	var response models.GithubOrganization
	err := copier.Copy(&response, in)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// Service contains functions of GithubOrganizations service
type Service interface {
	GetGithubOrganizations(ctx context.Context, projectSFID string) (*models.ProjectGithubOrganizations, error)
	AddGithubOrganization(ctx context.Context, projectSFID string, input *models.CreateGithubOrganization) (*models.GithubOrganization, error)
	DeleteGithubOrganization(ctx context.Context, projectSFID string, githubOrgName string) error
	UpdateGithubOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error
}

type service struct {
	repo                    v1GithubOrg.Repository
	ghRepository            v1Repositories.Repository
	projectsCLAGroupService projects_cla_groups.Repository
}

// NewService creates a new githubOrganizations service
func NewService(repo v1GithubOrg.Repository, ghRepository v1Repositories.Repository, projectsCLAGroupService projects_cla_groups.Repository) Service {
	return service{
		repo:                    repo,
		ghRepository:            ghRepository,
		projectsCLAGroupService: projectsCLAGroupService,
	}
}

const (
	// Connected status
	Connected = "connected"
	// PartialConnection status
	PartialConnection = "partial_connection"
	// ConnectionFailure status
	ConnectionFailure = "connection_failure"
	// NoConnection status
	NoConnection = "no_connection"
)

func (s service) GetGithubOrganizations(ctx context.Context, projectSFID string) (*models.ProjectGithubOrganizations, error) {
	f := logrus.Fields{
		"functionName":   "GetGitHubOrganizations",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
	}

	psc := v2ProjectService.GetClient()
	log.WithFields(f).Debug("loading project details from the project service...")
	projectServiceRecord, err := psc.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading project details from the project service")
		return nil, err
	}

	var parentProjectSFID string
	if (projectServiceRecord.Foundation != nil && projectServiceRecord.Foundation.Name == utils.TheLinuxFoundation) || projectServiceRecord.ProjectType == utils.ProjectTypeProjectGroup {
		parentProjectSFID = projectSFID
	} else {
		parentProjectSFID = projectServiceRecord.Parent
	}
	f["parentProjectSFID"] = parentProjectSFID
	log.WithFields(f).Debug("located parentProjectID...")

	log.WithFields(f).Debug("loading github organization details by parentProjectSFID...")
	orgs, err := s.repo.GetGithubOrganizationsByParent(ctx, parentProjectSFID)
	// log.WithFields(f).Debug("loading github organization details by projectSFID...")
	//orgs, err := s.repo.GetGithubOrganizations(ctx, projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading github organizations from the project service")
		return nil, err
	}

	out := &models.ProjectGithubOrganizations{
		List: make([]*models.ProjectGithubOrganization, 0),
	}
	type githubRepoInfo struct {
		orgName  string
		repoInfo *v1Models.GithubRepositoryInfo
	}
	// connectedRepo contains list of repositories for which github app have permission
	connectedRepo := make(map[string]*githubRepoInfo)
	orgmap := make(map[string]*models.ProjectGithubOrganization)
	for _, org := range orgs.List {
		for _, repoInfo := range org.Repositories.List {
			key := fmt.Sprintf("%s#%v", org.OrganizationName, repoInfo.RepositoryGithubID)
			connectedRepo[key] = &githubRepoInfo{
				orgName:  org.OrganizationName,
				repoInfo: repoInfo,
			}
		}

		autoEnabledCLAGroupName := ""
		if org.AutoEnabledClaGroupID != "" {
			log.WithFields(f).Debugf("Loading CLA Group by ID: %s to obtain the name for GitHub auth enabled CLA Group response", org.AutoEnabledClaGroupID)
			claGroupMode, claGroupLookupErr := s.projectsCLAGroupService.GetCLAGroup(org.AutoEnabledClaGroupID)
			if claGroupLookupErr != nil {
				log.WithFields(f).WithError(claGroupLookupErr).Warnf("Unable to lookup CLA Group by ID: %s", org.AutoEnabledClaGroupID)
			}
			if claGroupMode != nil {
				autoEnabledCLAGroupName = claGroupMode.ProjectName
			}
		}

		rorg := &models.ProjectGithubOrganization{
			AutoEnabled:             org.AutoEnabled,
			AutoEnableCLAGroupID:    org.AutoEnabledClaGroupID,
			AutoEnabledCLAGroupName: autoEnabledCLAGroupName,
			BranchProtectionEnabled: org.BranchProtectionEnabled,
			ConnectionStatus:        "", // updated below
			GithubOrganizationName:  org.OrganizationName,
			Repositories:            make([]*models.ProjectGithubRepository, 0),
		}

		orgmap[org.OrganizationName] = rorg
		out.List = append(out.List, rorg)
		if org.OrganizationInstallationID == 0 {
			rorg.ConnectionStatus = NoConnection
		} else {
			if org.Repositories.Error != "" {
				rorg.ConnectionStatus = ConnectionFailure
			} else {
				rorg.ConnectionStatus = Connected
			}
		}
	}

	log.WithFields(f).Debug("listing github repositories...")
	enabled := true
	repos, err := s.ghRepository.ListProjectRepositories(ctx, parentProjectSFID, projectSFID, &enabled)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading github repositories")
		return nil, err
	}

	log.WithFields(f).Debugf("processing %d github repositories...", len(repos.List))
	for _, repo := range repos.List {
		rorg, ok := orgmap[repo.RepositoryOrganizationName]
		if !ok {
			log.WithFields(f).Warnf("repositories table contain stale data for organization %s", repo.RepositoryOrganizationName)
			continue
		}
		key := fmt.Sprintf("%s#%v", repo.RepositoryOrganizationName, repo.RepositoryExternalID)
		if _, ok := connectedRepo[key]; ok {
			repoGithubID, err := strconv.ParseInt(repo.RepositoryExternalID, 10, 64)
			if err != nil {
				log.WithFields(f).WithError(err).Warn("repository github id is not integer")
			}
			rorg.Repositories = append(rorg.Repositories, &models.ProjectGithubRepository{
				ConnectionStatus:   Connected,
				Enabled:            true,
				RepositoryID:       repo.RepositoryID,
				RepositoryName:     repo.RepositoryName,
				RepositoryGithubID: repoGithubID,
				ClaGroupID:         repo.RepositoryProjectID,
				ProjectID:          repo.ProjectSFID,
				ParentProjectID:    repo.RepositorySfdcID,
			})
			// delete it from connectedRepo array since we have processed it
			// connectedArray after this loop will contain repo for which github app have permission but
			// they are enabled in cla
			delete(connectedRepo, key)
		} else {
			rorg.Repositories = append(rorg.Repositories, &models.ProjectGithubRepository{
				ConnectionStatus: ConnectionFailure,
				Enabled:          true,
				RepositoryID:     repo.RepositoryID,
				RepositoryName:   repo.RepositoryName,
				ClaGroupID:       repo.RepositoryProjectID,
				ProjectID:        repo.ProjectSFID,
				ParentProjectID:  repo.RepositorySfdcID,
			})
			if rorg.ConnectionStatus == Connected {
				rorg.ConnectionStatus = PartialConnection
			}
		}
	}

	for _, notEnabledRepo := range connectedRepo {
		rorg, ok := orgmap[notEnabledRepo.orgName]
		if !ok {
			log.WithFields(f).Warnf("failed to get org %s", notEnabledRepo.orgName)
			continue
		}
		rorg.Repositories = append(rorg.Repositories, &models.ProjectGithubRepository{
			ConnectionStatus:   Connected,
			Enabled:            false,
			RepositoryID:       "",
			RepositoryName:     notEnabledRepo.repoInfo.RepositoryName,
			RepositoryGithubID: notEnabledRepo.repoInfo.RepositoryGithubID,
		})
	}

	// Sort everything nicely
	sort.Slice(out.List, func(i, j int) bool {
		return strings.ToLower(out.List[i].GithubOrganizationName) < strings.ToLower(out.List[j].GithubOrganizationName)
	})
	for _, orgList := range out.List {
		sort.Slice(orgList.Repositories, func(i, j int) bool {
			return strings.ToLower(orgList.Repositories[i].RepositoryName) < strings.ToLower(orgList.Repositories[j].RepositoryName)
		})
	}

	return out, nil
}

func (s service) AddGithubOrganization(ctx context.Context, projectSFID string, input *models.CreateGithubOrganization) (*models.GithubOrganization, error) {
	f := logrus.Fields{
		"functionName":            "AddGitHubOrganization",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"projectSFID":             projectSFID,
		"autoEnabled":             utils.BoolValue(input.AutoEnabled),
		"branchProtectionEnabled": utils.BoolValue(input.BranchProtectionEnabled),
		"organizationName":        utils.StringValue(input.OrganizationName),
	}

	var in v1Models.CreateGithubOrganization
	err := copier.Copy(&in, input)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem converting the github organization details")
		return nil, err
	}

	log.WithFields(f).Debug("looking up project in project service...")
	psc := v2ProjectService.GetClient()
	project, err := psc.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading project details from the project service")
		return nil, err
	}

	var parentProjectSFID string
	if project.Parent == "" || project.Parent == utils.TheLinuxFoundation {
		parentProjectSFID = projectSFID
	} else {
		parentProjectSFID = project.Parent
	}
	f["parentProjectSFID"] = parentProjectSFID
	log.WithFields(f).Debug("located parentProjectID...")

	log.WithFields(f).Debug("adding github organization...")
	resp, err := s.repo.AddGithubOrganization(ctx, parentProjectSFID, projectSFID, &in)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem adding github organization for project")
		return nil, err
	}

	return v2GithubOrganizationModel(resp)
}

func (s service) UpdateGithubOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error {
	return s.repo.UpdateGithubOrganization(ctx, projectSFID, organizationName, autoEnabled, autoEnabledClaGroupID, branchProtectionEnabled)
}

func (s service) DeleteGithubOrganization(ctx context.Context, projectSFID string, githubOrgName string) error {
	f := logrus.Fields{
		"functionName":   "DeleteGitHubOrganization",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"githubOrgName":  githubOrgName,
	}

	psc := v2ProjectService.GetClient()
	log.WithFields(f).Debug("loading project details from the project service...")
	_, projectErr := psc.GetProject(projectSFID)
	if projectErr != nil {
		log.WithFields(f).WithError(projectErr).Warn("problem loading project details from the project service")
		return projectErr
	}

	log.WithFields(f).Debug("disabling repositories for github organization...")
	err := s.ghRepository.DisableRepositoriesOfGithubOrganization(ctx, projectSFID, githubOrgName)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem disabling repositories for github organization")
		return err
	}

	log.WithFields(f).Debug("deleting github github organization...")
	return s.repo.DeleteGithubOrganization(ctx, projectSFID, githubOrgName)
}
