// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_organizations

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/config"
	"github.com/go-openapi/strfmt"

	//v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gitlab"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	v2ProjectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"
	"github.com/sirupsen/logrus"
)

// Service contains functions of GitlabOrganizations service
type Service interface {
	GetGitlabOrganizations(ctx context.Context, projectSFID string) (*models.GitlabProjectOrganizations, error)
	AddGitlabOrganization(ctx context.Context, projectSFID string, input *models.GitlabCreateOrganization) (*models.GitlabProjectOrganizations, error)
	GetGitlabOrganization(ctx context.Context, gitlabOrganizationID string) (*models.GitlabOrganization, error)
	GetGitlabOrganizationByState(ctx context.Context, gitlabOrganizationID, authState string) (*models.GitlabOrganization, error)
	UpdateGitlabOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error
	UpdateGitlabOrganizationAuth(ctx context.Context, gitlabOrganizationID string, oauthResp *gitlab.OauthSuccessResponse) error
	DeleteGitlabOrganization(ctx context.Context, projectSFID string, gitlabOrgName string) error
}

type service struct {
	repo               RepositoryInterface
	claGroupRepository projects_cla_groups.Repository
}

// NewService creates a new gitlab organization service
func NewService(repo RepositoryInterface, claGroupRepository projects_cla_groups.Repository) Service {
	return service{
		repo:               repo,
		claGroupRepository: claGroupRepository,
	}
}

func (s service) GetGitlabOrganization(ctx context.Context, gitlabOrganizationID string) (*models.GitlabOrganization, error) {
	f := logrus.Fields{
		"functionName":         "v2.gitlab_organizations.service.GetGitlabOrganization",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"gitlabOrganizationID": gitlabOrganizationID,
	}

	log.WithFields(f).Debugf("fetching gitlab organization for gitlab org id : %s", gitlabOrganizationID)
	dbModel, err := s.repo.GetGitlabOrganization(ctx, gitlabOrganizationID)
	if err != nil {
		return nil, err
	}

	return ToModel(dbModel), nil
}

func (s service) UpdateGitlabOrganizationAuth(ctx context.Context, gitlabOrganizationID string, oauthResp *gitlab.OauthSuccessResponse) error {
	f := logrus.Fields{
		"functionName":         "v2.gitlab_organizations.service.UpdateGitlabOrganizationAuth",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"gitlabOrganizationID": gitlabOrganizationID,
	}

	log.WithFields(f).Debugf("updating gitlab org auth")
	authInfoEncrypted, err := gitlab.EncryptAuthInfo(oauthResp)
	if err != nil {
		return fmt.Errorf("encrypt failed : %v", err)
	}

	return s.repo.UpdateGitlabOrganizationAuth(ctx, gitlabOrganizationID, authInfoEncrypted)

}

func (s service) UpdateGitlabOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error {
	// check if valid cla group id is passed
	if autoEnabledClaGroupID != "" {
		if _, err := s.claGroupRepository.GetCLAGroupNameByID(ctx, autoEnabledClaGroupID); err != nil {
			return err
		}
	}

	return s.repo.UpdateGitlabOrganization(ctx, projectSFID, organizationName, autoEnabled, autoEnabledClaGroupID, branchProtectionEnabled, nil)
}

func (s service) GetGitlabOrganizationByState(ctx context.Context, gitlabOrganizationID, authState string) (*models.GitlabOrganization, error) {
	f := logrus.Fields{
		"functionName":         "v2.gitlab_organizations.service.GetGitlabOrganization",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"gitlabOrganizationID": gitlabOrganizationID,
		"authState":            authState,
	}

	log.WithFields(f).Debugf("fetching gitlab organization for gitlab org id : %s", gitlabOrganizationID)
	dbModel, err := s.repo.GetGitlabOrganization(ctx, gitlabOrganizationID)
	if err != nil {
		return nil, err
	}

	if dbModel.AuthState != authState {
		return nil, fmt.Errorf("auth state doesn't match")
	}

	return ToModel(dbModel), nil
}

func (s service) AddGitlabOrganization(ctx context.Context, projectSFID string, input *models.GitlabCreateOrganization) (*models.GitlabProjectOrganizations, error) {
	f := logrus.Fields{
		"functionName":            "v2.gitlab_organizations.service.AddGitlabOrganization",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"projectSFID":             projectSFID,
		"autoEnabled":             utils.BoolValue(input.AutoEnabled),
		"branchProtectionEnabled": utils.BoolValue(input.BranchProtectionEnabled),
		"organizationName":        utils.StringValue(input.OrganizationName),
	}

	psc := v2ProjectService.GetClient()
	project, err := psc.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading project details from the project service")
		return nil, err
	}

	var parentProjectSFID string
	if utils.StringValue(project.Parent) == "" || (project.Foundation != nil &&
		(project.Foundation.Name == utils.TheLinuxFoundation || project.Foundation.Name == utils.LFProjectsLLC)) {
		parentProjectSFID = projectSFID
	} else {
		parentProjectSFID = utils.StringValue(project.Parent)
	}
	f["parentProjectSFID"] = parentProjectSFID
	log.WithFields(f).Debug("located parentProjectID...")

	log.WithFields(f).Debug("adding gitlab organization...")
	resp, err := s.repo.AddGitlabOrganization(ctx, parentProjectSFID, projectSFID, input)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem adding gitlab organization for project")
		return nil, err
	}
	log.WithFields(f).Debugf("created GitLab organization with ID: %s", resp.OrganizationID)

	return s.GetGitlabOrganizations(ctx, projectSFID)
}

func (s service) GetGitlabOrganizations(ctx context.Context, projectSFID string) (*models.GitlabProjectOrganizations, error) {
	f := logrus.Fields{
		"functionName":   "v2.gitlab_organizations.service.GetGitlabOrganizations",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
	}

	// Load the GitLab Organization and Repository details - result will be missing CLA Group info and ProjectSFID details
	log.WithFields(f).Debugf("loading Gitlab organizations for projectSFID: %s", projectSFID)
	orgs, err := s.repo.GetGitlabOrganizations(ctx, projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading gitlab organizations from the project service")
		return nil, err
	}

	psc := v2ProjectService.GetClient()
	log.WithFields(f).Debug("loading project details from the project service...")
	projectServiceRecord, err := psc.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading project details from the project service")
		return nil, err
	}

	var parentProjectSFID string
	if utils.IsProjectHasRootParent(projectServiceRecord) {
		parentProjectSFID = projectSFID
	} else {
		parentProjectSFID = utils.StringValue(projectServiceRecord.Parent)
	}
	f["parentProjectSFID"] = parentProjectSFID
	log.WithFields(f).Debug("located parentProjectID...")

	// Our response model
	out := &models.GitlabProjectOrganizations{
		List: make([]*models.GitlabProjectOrganization, 0),
	}

	// Next, we need to load a bunch of additional data for the response including the GitLab status (if it's still connected/live, not renamed/moved), the CLA Group details, etc.

	//// A temp data model for holding the intermediate results
	//type gitlabRepoInfo struct {
	//	orgName  string
	//	repoInfo *v1Models.GitLabRepositoryInfo
	//}

	orgmap := make(map[string]*models.GitlabProjectOrganization)
	for _, org := range orgs.List {
		autoEnabledCLAGroupName := ""
		if org.AutoEnabledClaGroupID != "" {
			log.WithFields(f).Debugf("Loading CLA Group by ID: %s to obtain the name for GitLab auth enabled CLA Group response", org.AutoEnabledClaGroupID)
			claGroupMode, claGroupLookupErr := s.claGroupRepository.GetCLAGroup(ctx, org.AutoEnabledClaGroupID)
			if claGroupLookupErr != nil {
				log.WithFields(f).WithError(claGroupLookupErr).Warnf("Unable to lookup CLA Group by ID: %s", org.AutoEnabledClaGroupID)
			}
			if claGroupMode != nil {
				autoEnabledCLAGroupName = claGroupMode.ProjectName
			}
		}

		orgDetailed, err := s.repo.GetGitlabOrganization(ctx, org.OrganizationID)
		if err != nil {
			log.WithFields(f).Errorf("fetching gitlab org failed : %s : %v", org.OrganizationID, err)
			continue
		}

		installationURL := buildInstallationURL(org.OrganizationID, orgDetailed.AuthState)
		rorg := &models.GitlabProjectOrganization{
			AutoEnabled:             org.AutoEnabled,
			AutoEnableCLAGroupID:    org.AutoEnabledClaGroupID,
			AutoEnabledCLAGroupName: autoEnabledCLAGroupName,
			GitlabOrganizationName:  org.OrganizationName,
			Repositories:            make([]*models.GitlabProjectRepository, 0),
			InstallationURL:         installationURL,
		}

		if orgDetailed.AuthInfo == "" {
			rorg.ConnectionStatus = utils.NoConnection
		} else {
			glClient, err := gitlab.NewGitlabOauthClient(orgDetailed.AuthInfo)
			if err != nil {
				log.WithFields(f).Errorf("initializing gitlab client for gitlab org : %s failed : %v", org.OrganizationID, err)
				rorg.ConnectionStatus = utils.ConnectionFailure
			} else {
				user, _, err := glClient.Users.CurrentUser()
				if err != nil {
					log.WithFields(f).Errorf("using gitlab client for gitlab org : %s failed : %v", org.OrganizationID, err)
					rorg.ConnectionStatus = utils.ConnectionFailure
				} else {
					log.WithFields(f).Debugf("connected to user : %s for gitlab org : %s", user.Name, org.OrganizationID)
					rorg.ConnectionStatus = utils.Connected
				}
			}
		}

		orgmap[org.OrganizationName] = rorg
		out.List = append(out.List, rorg)
	}

	// Sort everything nicely
	sort.Slice(out.List, func(i, j int) bool {
		return strings.ToLower(out.List[i].GitlabOrganizationName) < strings.ToLower(out.List[j].GitlabOrganizationName)
	})
	for _, orgList := range out.List {
		sort.Slice(orgList.Repositories, func(i, j int) bool {
			return strings.ToLower(orgList.Repositories[i].RepositoryName) < strings.ToLower(orgList.Repositories[j].RepositoryName)
		})
	}

	return out, nil
}

func (s service) DeleteGitlabOrganization(ctx context.Context, projectSFID string, gitlabOrgName string) error {
	f := logrus.Fields{
		"functionName":   "DeleteGitlabOrganization",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"gitlabOrgName":  gitlabOrgName,
	}

	// Lookup the parent
	parentProjectSFID, projErr := v2ProjectService.GetClient().GetParentProject(projectSFID)
	if projErr != nil {
		log.WithFields(f).Warnf("problem fetching project parent SFID, error: %+v", projErr)
		return projErr
	}

	log.WithFields(f).Debugf("retrieved parent of project sfid : %s -> %s", projectSFID, parentProjectSFID)

	// Todo: Enable this when the repositories are implemented
	//err := s.ghRepository.GitHubDisableRepositoriesOfOrganization(ctx, parentProjectSFID, gitlabOrgName)
	//if err != nil {
	//	log.WithFields(f).Warnf("problem disabling repositories for github organizations, error: %+v", projErr)
	//	return err
	//}

	return s.repo.DeleteGitlabOrganization(ctx, projectSFID, gitlabOrgName)
}

func buildInstallationURL(gitlabOrgID string, authStateNonce string) *strfmt.URI {
	base := "https://gitlab.com/oauth/authorize"
	c := config.GetConfig()
	state := fmt.Sprintf("%s:%s", gitlabOrgID, authStateNonce)

	params := url.Values{}
	params.Add("client_id", c.Gitlab.AppID)
	params.Add("redirect_uri", c.Gitlab.RedirectURI)
	//params.Add("redirect_uri", "http://localhost:8080/v4/gitlab/oauth/callback")
	params.Add("response_type", "code")
	params.Add("state", state)
	params.Add("scope", "api read_user read_api read_repository write_repository email")

	installationURL := strfmt.URI(base + "?" + params.Encode())
	return &installationURL
}
