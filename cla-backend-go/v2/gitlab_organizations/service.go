// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_organizations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/v2/repositories"

	"github.com/communitybridge/easycla/cla-backend-go/v2/common"

	"github.com/communitybridge/easycla/cla-backend-go/config"
	gitlab2 "github.com/communitybridge/easycla/cla-backend-go/gitlab"
	"github.com/go-openapi/strfmt"

	project_service "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gitlab"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	v2ProjectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"
	"github.com/sirupsen/logrus"
	goGitLab "github.com/xanzy/go-gitlab"
)

// ServiceInterface contains functions of GitlabOrganizations service
type ServiceInterface interface {
	AddGitlabOrganization(ctx context.Context, projectSFID string, input *models.GitlabCreateOrganization) (*models.GitlabProjectOrganizations, error)
	GetGitlabOrganization(ctx context.Context, gitlabOrganizationID string) (*models.GitlabOrganization, error)
	GetGitlabOrganizationByID(ctx context.Context, gitlabOrganizationID string) (*common.GitLabOrganization, error)
	GetGitlabOrganizations(ctx context.Context, projectSFID string) (*models.GitlabProjectOrganizations, error)
	GetGitlabOrganizationByState(ctx context.Context, gitlabOrganizationID, authState string) (*models.GitlabOrganization, error)
	UpdateGitlabOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error
	UpdateGitlabOrganizationAuth(ctx context.Context, gitlabOrganizationID string, oauthResp *gitlab.OauthSuccessResponse) error
	DeleteGitlabOrganization(ctx context.Context, projectSFID string, gitlabOrgName string) error
}

// Service data model
type Service struct {
	repo               RepositoryInterface
	v2GitRepoService   repositories.ServiceInterface
	claGroupRepository projects_cla_groups.Repository
	gitLabApp          *gitlab.App
}

// NewService creates a new gitlab organization service
func NewService(repo RepositoryInterface, v2GitRepoService repositories.ServiceInterface, claGroupRepository projects_cla_groups.Repository) *Service {
	return &Service{
		repo:               repo,
		v2GitRepoService:   v2GitRepoService,
		claGroupRepository: claGroupRepository,
		gitLabApp:          gitlab.Init(config.GetConfig().Gitlab.AppClientID, config.GetConfig().Gitlab.AppClientSecret, config.GetConfig().Gitlab.AppPrivateKey),
	}
}

// AddGitlabOrganization adds the specified GitLab organization
func (s *Service) AddGitlabOrganization(ctx context.Context, projectSFID string, input *models.GitlabCreateOrganization) (*models.GitlabProjectOrganizations, error) {
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

// GetGitlabOrganization returns the GitLab organization based on the specified GitLab Organization ID
func (s *Service) GetGitlabOrganization(ctx context.Context, gitlabOrganizationID string) (*models.GitlabOrganization, error) {
	dbModel, err := s.GetGitlabOrganizationByID(ctx, gitlabOrganizationID)
	if err != nil {
		return nil, err
	}

	if dbModel == nil {
		return nil, nil
	}

	return common.ToModel(dbModel), err
}

// GetGitlabOrganizationByID returns the record associated with the GitLab Organization ID
func (s *Service) GetGitlabOrganizationByID(ctx context.Context, gitlabOrganizationID string) (*common.GitLabOrganization, error) {
	f := logrus.Fields{
		"functionName":         "v2.gitlab_organizations.service.GetGitlabOrganizationByID",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"gitlabOrganizationID": gitlabOrganizationID,
	}

	log.WithFields(f).Debugf("fetching gitlab organization for gitlab org id : %s", gitlabOrganizationID)
	dbModel, err := s.repo.GetGitlabOrganization(ctx, gitlabOrganizationID)
	if err != nil {
		return nil, err
	}

	return dbModel, nil
}

// GetGitlabOrganizations returns a collection of GitLab organizations based on the specified project SFID value
func (s *Service) GetGitlabOrganizations(ctx context.Context, projectSFID string) (*models.GitlabProjectOrganizations, error) {
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

		orgDetailed, orgErr := s.repo.GetGitlabOrganization(ctx, org.OrganizationID)
		if orgErr != nil {
			log.WithFields(f).Errorf("fetching gitlab org failed : %s : %v", org.OrganizationID, orgErr)
			continue
		}

		reposFromOrg, repoErr := s.v2GitRepoService.GitLabGetRepositoriesByOrganizationName(ctx, org.OrganizationName)
		if repoErr != nil {
			if errors.Is(repoErr, &utils.GitLabRepositoryNotFound{}) {
				log.WithFields(f).Debugf("no repositories onboarded for GitLab Org: %s", org.OrganizationName)
			} else {
				log.WithFields(f).Debugf("unexpected error while fetching gitlab org repositories for GitLab Org: %s : %v", org.OrganizationName, repoErr)
			}
		}

		rorg := &models.GitlabProjectOrganization{
			AutoEnabled:             org.AutoEnabled,
			AutoEnableClaGroupID:    org.AutoEnabledClaGroupID,
			AutoEnabledClaGroupName: strings.TrimSpace(autoEnabledCLAGroupName),
			OrganizationName:        org.OrganizationName,
			OrganizationURL:         org.OrganizationURL,
			OrganizationFullPath:    org.OrganizationFullPath,
			InstallationURL:         buildInstallationURL(org.OrganizationID, orgDetailed.AuthState),
			BranchProtectionEnabled: false,
			ConnectionStatus:        "", // updated below
			Repositories:            []*models.GitlabProjectRepository{},
		}

		if orgDetailed.AuthInfo == "" {
			rorg.ConnectionStatus = utils.NoConnection
		} else {
			if err != nil {
				log.WithFields(f).Errorf("initializing gitlab client for gitlab org : %s failed : %v", org.OrganizationID, err)
				rorg.ConnectionStatus = utils.ConnectionFailure
			} else {
				// We've been authenticated by the user - great, see if we can determine the list of repos...
				glClient, clientErr := gitlab.NewGitlabOauthClient(orgDetailed.AuthInfo, s.gitLabApp)
				if clientErr != nil {
					log.WithFields(f).Errorf("using gitlab client for gitlab org : %s failed : %v", org.OrganizationID, clientErr)
					rorg.ConnectionStatus = utils.ConnectionFailure
				} else {
					rorg.Repositories = s.updateRepositoryStatus(glClient, toGitLabProjectResponse(reposFromOrg))

					user, _, userErr := glClient.Users.CurrentUser()
					if userErr != nil {
						log.WithFields(f).Errorf("using gitlab client for gitlab org : %s failed : %v", org.OrganizationID, userErr)
						rorg.ConnectionStatus = utils.ConnectionFailure
					} else {
						log.WithFields(f).Debugf("connected to user : %s for gitlab org : %s", user.Name, org.OrganizationID)
						rorg.ConnectionStatus = utils.Connected
					}
				}
			}
		}

		orgmap[org.OrganizationName] = rorg
		out.List = append(out.List, rorg)
	}

	// Sort everything nicely
	sort.Slice(out.List, func(i, j int) bool {
		return strings.ToLower(out.List[i].OrganizationName) < strings.ToLower(out.List[j].OrganizationName)
	})
	for _, orgList := range out.List {
		sort.Slice(orgList.Repositories, func(i, j int) bool {
			return strings.ToLower(orgList.Repositories[i].RepositoryName) < strings.ToLower(orgList.Repositories[j].RepositoryName)
		})
	}

	return out, nil
}

// GetGitlabOrganizationByState returns the GitLab organization by the auth state
func (s *Service) GetGitlabOrganizationByState(ctx context.Context, gitlabOrganizationID, authState string) (*models.GitlabOrganization, error) {
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

	return common.ToModel(dbModel), nil
}

// UpdateGitlabOrganizationAuth updates the GitLab organization authentication information
func (s *Service) UpdateGitlabOrganizationAuth(ctx context.Context, gitlabOrganizationID string, oauthResp *gitlab.OauthSuccessResponse) error {
	f := logrus.Fields{
		"functionName":         "v2.gitlab_organizations.service.UpdateGitlabOrganizationAuth",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"gitlabOrganizationID": gitlabOrganizationID,
	}

	log.WithFields(f).Debugf("updating gitlab org auth")
	authInfoEncrypted, err := gitlab.EncryptAuthInfo(oauthResp, s.gitLabApp)
	if err != nil {
		return fmt.Errorf("encrypt failed : %v", err)
	}

	gitLabOrgModel, err := s.GetGitlabOrganizationByID(ctx, gitlabOrganizationID)
	if err != nil {
		return fmt.Errorf("gitlab organization lookup error: %+v", err)
	}

	// Get the client
	gitLabClient, err := gitlab2.NewGitlabOauthClientFromAccessToken(oauthResp.AccessToken)
	if err != nil {
		return fmt.Errorf("initializing gitlab client : %v", err)
	}

	// Need to look up the GitLab Group/Organization to obtain the ID
	//groups, resp, searchErr := gitLabClient.Groups.SearchGroup(gitLabOrgModel.OrganizationName)
	// Need to look up the GitLab Group/Organization to obtain the ID
	opts := &goGitLab.ListGroupsOptions{
		ListOptions: goGitLab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}
	groups, resp, searchErr := gitLabClient.Groups.ListGroups(opts)
	if searchErr != nil {
		return fmt.Errorf("GitLab search error while locating Group by name: %s, error: %v", gitLabOrgModel.OrganizationName, searchErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("unable to locate GitLab group by name: %s, status code: %d", gitLabOrgModel.OrganizationName, resp.StatusCode)
	}
	for _, g := range groups {
		if g.Name == gitLabOrgModel.OrganizationName {
			return s.repo.UpdateGitlabOrganizationAuth(ctx, gitlabOrganizationID, g.ID, authInfoEncrypted, g.FullPath, g.WebURL)
		}
	}

	return fmt.Errorf("unable to locate GitLab group name '%s' using search, found: %d", gitLabOrgModel.OrganizationName, len(groups))
}

// UpdateGitlabOrganization updates the GitLab organization
func (s *Service) UpdateGitlabOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error {
	// check if valid cla group id is passed
	if autoEnabledClaGroupID != "" {
		if _, err := s.claGroupRepository.GetCLAGroupNameByID(ctx, autoEnabledClaGroupID); err != nil {
			return err
		}
	}

	return s.repo.UpdateGitlabOrganization(ctx, projectSFID, organizationName, autoEnabled, autoEnabledClaGroupID, branchProtectionEnabled, nil)
}

// DeleteGitlabOrganization deletes the specified GitLab organization
func (s *Service) DeleteGitlabOrganization(ctx context.Context, projectSFID string, gitlabOrgName string) error {
	f := logrus.Fields{
		"functionName":   "v2.gitlab_organizations.service.DeleteGitlabOrganization",
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
	params.Add("client_id", c.Gitlab.AppClientID)
	params.Add("redirect_uri", c.Gitlab.RedirectURI)
	//params.Add("redirect_uri", "http://localhost:8080/v4/gitlab/oauth/callback")
	params.Add("response_type", "code")
	params.Add("state", state)
	params.Add("scope", "api read_user read_api read_repository write_repository email")

	installationURL := strfmt.URI(base + "?" + params.Encode())
	return &installationURL
}

func toGitLabProjectResponse(gitLabListRepos *models.GitlabListRepositories) []*models.GitlabProjectRepository {
	f := logrus.Fields{
		"functionName": "v2.gitlab_organizations.service.toGitLabProjectResponse",
	}

	if gitLabListRepos == nil {
		return []*models.GitlabProjectRepository{}
	}

	var repoList []*models.GitlabProjectRepository
	for _, repo := range gitLabListRepos.List {
		parentProjectSFID, err := project_service.GetClient().GetParentProject(repo.RepositoryProjectSfid)
		if err != nil {
			log.WithFields(f).Warnf("unable to lookup project parent SFID using SFID: %s", repo.RepositoryProjectSfid)
		}

		repoList = append(repoList, &models.GitlabProjectRepository{
			ClaGroupID: repo.RepositoryClaGroupID,
			//ConnectionStatus:   "", // set via another function
			Enabled:            repo.Enabled,
			ParentProjectID:    parentProjectSFID,
			ProjectID:          repo.RepositoryProjectSfid,
			RepositoryGitlabID: repo.RepositoryExternalID,
			RepositoryID:       repo.RepositoryID,
			RepositoryName:     repo.RepositoryName,
			RepositoryFullPath: repo.RepositoryFullPath,
			RepositoryURL:      repo.RepositoryURL,
		})
	}

	return repoList
}

// updateRepositoryStatus is a helper function to set/add the repository connection status
func (s *Service) updateRepositoryStatus(glClient *goGitLab.Client, gitLabProjectRepos []*models.GitlabProjectRepository) []*models.GitlabProjectRepository {
	f := logrus.Fields{
		"functionName": "v2.gitlab_organizations.service.updateRepositoryStatus",
	}

	if gitLabProjectRepos == nil {
		return []*models.GitlabProjectRepository{}
	}

	type responseChannelModel struct {
		RepositoryExternalID int64
		ConnectionStatus     string
		Error                error
	}
	// A channel for the responses from the go routines
	responseChannel := make(chan responseChannelModel)

	opts := &goGitLab.GetProjectOptions{}
	for _, repo := range gitLabProjectRepos {
		// Create a go routine to this concurrently
		go func(glClient *goGitLab.Client, repo *models.GitlabProjectRepository) {
			projectModel, resp, lookupErr := glClient.Projects.GetProject(int(repo.RepositoryGitlabID), opts) // OK to convert int64 to int as it is the ID and probably should be typed as a int anyway
			if lookupErr != nil {
				log.WithFields(f).WithError(lookupErr).Warnf("problem loading GitLab project by external ID: %d, error: %v", repo.RepositoryGitlabID, lookupErr)
				repo.ConnectionStatus = utils.ConnectionFailure
				responseChannel <- responseChannelModel{
					RepositoryExternalID: repo.RepositoryGitlabID,
					ConnectionStatus:     utils.ConnectionFailure,
					Error:                lookupErr,
				}
				return
			}
			if resp.StatusCode < 200 || resp.StatusCode > 299 {
				responseBody, readErr := io.ReadAll(resp.Body)
				if readErr != nil {
					log.WithFields(f).WithError(lookupErr).Warnf("problem loading GitLab project by external ID: %d, error: %v", repo.RepositoryGitlabID, readErr)
					responseChannel <- responseChannelModel{
						RepositoryExternalID: repo.RepositoryGitlabID,
						ConnectionStatus:     utils.ConnectionFailure,
						Error:                readErr,
					}
					return
				}
				msg := fmt.Sprintf("problem loading GitLab project by external ID: %d, response code: %d, body: %s", repo.RepositoryGitlabID, resp.StatusCode, responseBody)
				log.WithFields(f).Warnf(msg)
				responseChannel <- responseChannelModel{
					RepositoryExternalID: repo.RepositoryGitlabID,
					ConnectionStatus:     utils.ConnectionFailure,
					Error:                errors.New(msg),
				}
				return
			}

			log.WithFields(f).Debugf("connected to project/repo: %s", projectModel.PathWithNamespace)
			responseChannel <- responseChannelModel{
				RepositoryExternalID: repo.RepositoryGitlabID,
				ConnectionStatus:     utils.Connected,
				Error:                nil,
			}
		}(glClient, repo)
	}

	for i := 0; i < len(gitLabProjectRepos); i++ {
		resp := <-responseChannel
		// Find the matching repo for this response
		for _, repo := range gitLabProjectRepos {
			if repo.RepositoryGitlabID == resp.RepositoryExternalID {
				repo.ConnectionStatus = resp.ConnectionStatus
			}
		}
		if resp.Error != nil {
			log.WithFields(f).Warnf("problem connecting to GitLab repoistory, error: %+v", resp.Error)
		}
	}

	return gitLabProjectRepos
}
