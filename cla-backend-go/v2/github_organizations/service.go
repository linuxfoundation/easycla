// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_organizations

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/go-openapi/strfmt"
	"github.com/sirupsen/logrus"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	v1GithubOrg "github.com/communitybridge/easycla/cla-backend-go/github_organizations"
	gitV1Repository "github.com/communitybridge/easycla/cla-backend-go/repositories"
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
	AddGithubOrganization(ctx context.Context, projectSFID string, input *models.GithubCreateOrganization) (*models.GithubOrganization, error)
	DeleteGithubOrganization(ctx context.Context, projectSFID string, githubOrgName string) error
	UpdateGithubOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error
}

type service struct {
	repo                    v1GithubOrg.RepositoryInterface
	gitV1Repository         gitV1Repository.RepositoryInterface
	ghService               v1GithubOrg.ServiceInterface
	projectsCLAGroupService projects_cla_groups.Repository
}

// NewService creates a new githubOrganizations service
func NewService(repo v1GithubOrg.RepositoryInterface, gitV1Repository gitV1Repository.RepositoryInterface, projectsCLAGroupService projects_cla_groups.Repository, ghService v1GithubOrg.ServiceInterface) Service {
	return service{
		repo:                    repo,
		gitV1Repository:         gitV1Repository,
		projectsCLAGroupService: projectsCLAGroupService,
		ghService:               ghService,
	}
}

func (s service) GetGithubOrganizations(ctx context.Context, projectSFID string) (*models.ProjectGithubOrganizations, error) {
	f := logrus.Fields{
		"functionName":   "v2.github_organizations.service.GetGitHubOrganizations",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
	}
	var orgs *v1Models.GithubOrganizations
	var orgErr error
	pcg, pcgErr := s.projectsCLAGroupService.GetClaGroupIDForProject(ctx, projectSFID)
	if pcgErr != nil {
		if pcgErr == projects_cla_groups.ErrProjectNotAssociatedWithClaGroup {
			log.WithFields(f).Warnf("unable to locate project CLA Group mapping for project SFID: %s, error: %+v", projectSFID, pcgErr)
		} else {
			log.WithFields(f).WithError(pcgErr).Warnf("unable to load project CLA group for project SFID: %s", projectSFID)
			return nil, pcgErr
		}
	}

	if pcg != nil && pcg.FoundationSFID != "" {
		log.WithFields(f).Debugf("Getting Github Organizations under foundation : %s", pcg.FoundationSFID)
		orgs, orgErr = s.repo.GetGitHubOrganizationsByParent(ctx, pcg.FoundationSFID)
	} else {
		log.WithFields(f).Debugf("Getting Github Organizations under project : %s", projectSFID)
		orgs, orgErr = s.repo.GetGitHubOrganizations(ctx, projectSFID)
	}

	if orgErr != nil {
		log.WithFields(f).Warnf("problem loading github organizations for project : %s, error: %+v", projectSFID, orgErr)
		return nil, orgErr
	}

	// Load the GitHub Organization and Repository details - result will be missing CLA Group info and ProjectSFID details
	log.WithFields(f).Debugf("discovered %d GitHub organizations for projectSFID: %s", len(orgs.List), projectSFID)
	orgs.List = s.ghService.RemoveDuplicates(orgs.List)

	psc := v2ProjectService.GetClient()
	log.WithFields(f).Debug("loading project details from the project service...")
	project, err := psc.GetProject(projectSFID)

	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading project details from the project service")
		return nil, err
	}
	if project == nil {
		log.WithFields(f).Warnf("unable to load project by project SFID: %s", projectSFID)
		return nil, nil
	}
	f["projectName"] = project.Name
	f["projectType"] = project.Type
	f["projectStatus"] = project.Status

	var parentProjectSFID string
	if !utils.IsProjectHaveParent(project) {
		parentProjectSFID = projectSFID
	} else {
		parentProjectSFID = utils.GetProjectParentSFID(project)
		// If we don't have a valid parent project SFID...
		if parentProjectSFID == "" {
			parentProjectSFID = projectSFID
		}
	}

	f["parentProjectSFID"] = parentProjectSFID
	log.WithFields(f).Debug("located parentProjectID...")

	// Our response model
	out := &models.ProjectGithubOrganizations{
		List: make([]*models.ProjectGithubOrganization, 0),
	}

	// Next, we need to load a bunch of additional data for the response including the github status (if it's still connected/live, not renamed/moved), the CLA Group details, etc.

	// A temp data model for holding the intermediate results
	type githubRepoInfo struct {
		orgName  string
		repoInfo *v1Models.GithubRepositoryInfo
	}

	// connectedRepo contains list of repositories for which github app have permission to see
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
			claGroupMode, claGroupLookupErr := s.projectsCLAGroupService.GetCLAGroup(ctx, org.AutoEnabledClaGroupID)
			if claGroupLookupErr != nil {
				log.WithFields(f).WithError(claGroupLookupErr).Warnf("Unable to lookup CLA Group by ID: %s", org.AutoEnabledClaGroupID)
			}
			if claGroupMode != nil {
				autoEnabledCLAGroupName = claGroupMode.ProjectName
			}
		}

		installURL := url.URL{
			Scheme: "https",
			Host:   "github.com",
			Path:   fmt.Sprintf("/organizations/%s/settings/installations/%d", org.OrganizationName, org.OrganizationInstallationID),
		}
		installationURL := strfmt.URI(installURL.String())

		rorg := &models.ProjectGithubOrganization{
			AutoEnabled:             org.AutoEnabled,
			AutoEnableCLAGroupID:    org.AutoEnabledClaGroupID,
			AutoEnabledCLAGroupName: autoEnabledCLAGroupName,
			BranchProtectionEnabled: org.BranchProtectionEnabled,
			ConnectionStatus:        "", // updated below
			GithubOrganizationName:  org.OrganizationName,
			Repositories:            make([]*models.ProjectGithubRepository, 0),
			InstallationURL:         &installationURL,
		}

		orgmap[org.OrganizationName] = rorg
		out.List = append(out.List, rorg)
		if org.OrganizationInstallationID == 0 {
			rorg.ConnectionStatus = utils.NoConnection
		} else {
			if org.Repositories.Error != "" {
				rorg.ConnectionStatus = utils.ConnectionFailure
			} else {
				rorg.ConnectionStatus = utils.Connected
			}
		}
	}

	// We need to search the repository list based on two criteria
	// Need to search by projectSFID and/or Organization ID????
	log.WithFields(f).Debugf("loading github repositories from %d organizations for projectSFID: %s...", len(orgs.List), projectSFID)
	var repoList []*v1Models.GithubRepository
	for _, org := range orgs.List {
		orgRepos, orgReposErr := s.gitV1Repository.GitHubGetRepositoriesByOrganizationName(ctx, org.OrganizationName)
		if orgReposErr != nil || len(orgRepos) == 0 {
			if _, ok := orgReposErr.(*utils.GitHubRepositoryNotFound); ok {
				log.WithFields(f).Debug(orgReposErr)
			} else {
				log.WithFields(f).WithError(orgReposErr).Warn("problem loading github repositories by org name")
			}
		} else {
			repoList = append(repoList, orgRepos...)
		}
	}

	// Remove any duplicates
	log.WithFields(f).Debugf("processing %d github repositories...", len(repoList))
	for _, repo := range repoList {
		if repo == nil || repo.RepositoryOrganizationName == "" {
			log.WithFields(f).Warnf("repositories record nil or is missing the organization name: %+v - skipping", repo)
			continue
		}
		//log.WithFields(f).Debugf("processing repository: %s", repo.RepositoryURL)

		rorg, ok := orgmap[repo.RepositoryOrganizationName]
		if !ok {
			log.WithFields(f).Warnf("repositories table contain stale data for organization %s", repo.RepositoryOrganizationName)
			continue
		}
		key := fmt.Sprintf("%s#%v", repo.RepositoryOrganizationName, repo.RepositoryExternalID)

		parentProjectSFID = repo.RepositoryProjectSfid
		parentProjectModel, projectModelErr := v2ProjectService.GetClient().GetParentProjectModel(repo.RepositoryProjectSfid)
		if projectModelErr == nil && parentProjectModel != nil {
			parentProjectSFID = parentProjectModel.ID
		}

		if _, ok := connectedRepo[key]; ok {

			rorg.Repositories = append(rorg.Repositories, &models.ProjectGithubRepository{
				ConnectionStatus:   utils.Connected,
				Enabled:            repo.Enabled,
				RepositoryID:       repo.RepositoryID,
				RepositoryName:     repo.RepositoryName,
				RepositoryGithubID: repo.RepositoryExternalID,
				ClaGroupID:         repo.RepositoryClaGroupID,
				ProjectID:          repo.RepositoryProjectSfid,
				ParentProjectID:    parentProjectSFID,
			})

			// delete it from connectedRepo array since we have processed it
			// connectedArray after this loop will contain repo for which github app have permission but
			// they are enabled in cla
			delete(connectedRepo, key)
		} else {
			rorg.Repositories = append(rorg.Repositories, &models.ProjectGithubRepository{
				ConnectionStatus: utils.ConnectionFailure,
				Enabled:          repo.Enabled,
				RepositoryID:     repo.RepositoryID,
				RepositoryName:   repo.RepositoryName,
				ClaGroupID:       repo.RepositoryClaGroupID,
				ProjectID:        repo.RepositoryProjectSfid,
				ParentProjectID:  parentProjectSFID,
			})

			if rorg.ConnectionStatus == utils.Connected {
				rorg.ConnectionStatus = utils.PartialConnection
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
			ConnectionStatus:   utils.Connected,
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

func (s service) AddGithubOrganization(ctx context.Context, projectSFID string, input *models.GithubCreateOrganization) (*models.GithubOrganization, error) {
	f := logrus.Fields{
		"functionName":            "v2.github_organizations.service.AddGitHubOrganization",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"projectSFID":             projectSFID,
		"autoEnabled":             utils.BoolValue(input.AutoEnabled),
		"branchProtectionEnabled": utils.BoolValue(input.BranchProtectionEnabled),
		"organizationName":        utils.StringValue(input.OrganizationName),
	}

	var in v1Models.GithubCreateOrganization
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
	if !utils.IsProjectHaveParent(project) || utils.IsProjectHasRootParent(project) || utils.GetProjectParentSFID(project) == "" {
		parentProjectSFID = projectSFID
	} else {
		parentProjectSFID = utils.GetProjectParentSFID(project)
	}

	f["parentProjectSFID"] = parentProjectSFID
	log.WithFields(f).Debug("located parentProjectID...")

	log.WithFields(f).Debug("adding github organization...")
	resp, err := s.repo.AddGitHubOrganization(ctx, parentProjectSFID, projectSFID, &in)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem adding github organization for project")
		return nil, err
	}

	return v2GithubOrganizationModel(resp)
}

func (s service) UpdateGithubOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error {
	return s.repo.UpdateGitHubOrganization(ctx, projectSFID, organizationName, autoEnabled, autoEnabledClaGroupID, branchProtectionEnabled, nil)
}

func (s service) DeleteGithubOrganization(ctx context.Context, projectSFID string, githubOrgName string) error {
	f := logrus.Fields{
		"functionName":   "v2.github_organizations.service.DeleteGitHubOrganization",
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
	err := s.gitV1Repository.GitHubDisableRepositoriesOfOrganization(ctx, projectSFID, githubOrgName)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem disabling repositories for github organization")
		return err
	}

	log.WithFields(f).Debug("deleting github github organization...")
	return s.repo.DeleteGitHubOrganization(ctx, projectSFID, githubOrgName)
}
