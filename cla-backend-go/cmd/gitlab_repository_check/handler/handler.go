// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package handler

import (
	"context"
	"os"
	"strconv"

	"github.com/communitybridge/easycla/cla-backend-go/signatures"
	"github.com/communitybridge/easycla/cla-backend-go/v2/approvals"
	"github.com/communitybridge/easycla/cla-backend-go/v2/common"

	"github.com/communitybridge/easycla/cla-backend-go/project/repository"

	"github.com/communitybridge/easycla/cla-backend-go/config"

	"github.com/aws/aws-sdk-go/aws/session"

	v1Company "github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gerrits"
	"github.com/communitybridge/easycla/cla-backend-go/github_organizations"
	gitLabApi "github.com/communitybridge/easycla/cla-backend-go/gitlab_api"
	gitlab "github.com/communitybridge/easycla/cla-backend-go/gitlab_api"
	ini "github.com/communitybridge/easycla/cla-backend-go/init"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	v1Repositories "github.com/communitybridge/easycla/cla-backend-go/repositories"
	"github.com/communitybridge/easycla/cla-backend-go/users"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/communitybridge/easycla/cla-backend-go/v2/gitlab_organizations"
	v2Repositories "github.com/communitybridge/easycla/cla-backend-go/v2/repositories"
	"github.com/communitybridge/easycla/cla-backend-go/v2/store"

	"strings"

	"github.com/sirupsen/logrus"
	goGitLab "github.com/xanzy/go-gitlab"
)

var (
	awsSession *session.Session
	stage      string
	configFile config.Config
	gitLabApp  *gitLabApi.App
)

// Init initializes the handler
func Init() {
	f := logrus.Fields{
		"functionName": "cmd.gitlab_repository_check.handler.Init",
	}
	ctx := utils.NewContext()
	f[utils.XREQUESTID] = ctx.Value(utils.XREQUESTID)
	log.WithFields(f).Debug("initializing...")

	// General initialization
	ini.Init()

	var awsErr error
	awsSession, awsErr = ini.GetAWSSession()
	if awsErr != nil {
		log.WithFields(f).WithError(awsErr).Panic("unable to load AWS session")
	}

	// Need to initialize the system to load the configuration which contains a number of SSM parameters
	stage = os.Getenv("STAGE")
	if stage == "" {
		log.WithFields(f).Panic("unable to determine STAGE - please set in the environment variable: 'STAGE' - expected one of [DEV, STAGING, PROD]")
	}

	dynamodbRegion := os.Getenv("DYNAMODB_AWS_REGION")
	if dynamodbRegion == "" {
		log.WithFields(f).Panic("unable to determine DYNAMODB_AWS_REGION - please set in the environment variable: 'DYNAMODB_AWS_REGION'")
	}

	var configErr error
	configFile, configErr = config.LoadConfig("", awsSession, stage)
	if configErr != nil {
		log.WithFields(f).WithError(configErr).Panicf("Unable to load config - Error: %v", configErr)
	}

	if configFile.Gitlab.AppClientID == "" {
		log.WithFields(f).Panic("unable to determine configFile.Gitlab.AppClientID value - please set the configuration")
	}
	if configFile.Gitlab.AppClientSecret == "" {
		log.WithFields(f).Panic("unable to determine configFile.Gitlab.AppClientSecret value - please set the configuration")
	}
	if configFile.Gitlab.AppPrivateKey == "" {
		log.WithFields(f).Panic("unable to determine configFile.Gitlab.AppPrivateKey value - please set the configuration")
	}

	// Create a new GitLab App client instance
	gitLabApp = gitlab.Init(configFile.Gitlab.AppClientID, configFile.Gitlab.AppClientSecret, configFile.Gitlab.AppPrivateKey)

}

// Handler is invoked each time the lambda is triggered - https://docs.aws.amazon.com/lambda/latest/dg/golang-handler.html
func Handler(ctx context.Context) error {
	f := logrus.Fields{
		"functionName": "cmd.update-project-statistics.Handler",
	}

	// Add the x-request-id to the context
	ctx = utils.NewContextFromParent(ctx)
	f[utils.XREQUESTID] = ctx.Value(utils.XREQUESTID)

	// Repository Layer
	usersRepo := users.NewRepository(awsSession, stage)
	eventsRepo := events.NewRepository(awsSession, stage)
	v1CompanyRepo := v1Company.NewRepository(awsSession, stage)
	gerritRepo := gerrits.NewRepository(awsSession, stage)
	v1ProjectClaGroupRepo := projects_cla_groups.NewRepository(awsSession, stage)
	gitV1Repository := v1Repositories.NewRepository(awsSession, stage)
	gitV2Repository := v2Repositories.NewRepository(awsSession, stage)
	githubOrganizationsRepo := github_organizations.NewRepository(awsSession, stage)
	gitlabOrganizationRepo := gitlab_organizations.NewRepository(awsSession, stage)
	v1CLAGroupRepo := repository.NewRepository(awsSession, stage, gitV1Repository, gerritRepo, v1ProjectClaGroupRepo)
	storeRepo := store.NewRepository(awsSession, stage)

	// Service Layer

	type combinedRepo struct {
		users.UserRepository
		v1Company.IRepository
		repository.ProjectRepository
		projects_cla_groups.Repository
	}

	// Our service layer handlers
	eventsService := events.NewService(eventsRepo, combinedRepo{
		usersRepo,
		v1CompanyRepo,
		v1CLAGroupRepo,
		v1ProjectClaGroupRepo,
	})

	gerritService := gerrits.NewService(gerritRepo, &gerrits.LFGroup{
		LfBaseURL:    configFile.LFGroup.ClientURL,
		ClientID:     configFile.LFGroup.ClientID,
		ClientSecret: configFile.LFGroup.ClientSecret,
		RefreshToken: configFile.LFGroup.RefreshToken,
	})

	approvalsTableName := "cla-" + stage + "-approvals"

	usersService := users.NewService(usersRepo, eventsService)
	approvalsRepo := approvals.NewRepository(stage, awsSession, approvalsTableName)
	signaturesRepo := signatures.NewRepository(awsSession, stage, v1CompanyRepo, usersRepo, eventsService, gitV1Repository, githubOrganizationsRepo, gerritService, approvalsRepo)
	v2RepositoriesService := v2Repositories.NewService(gitV1Repository, gitV2Repository, v1ProjectClaGroupRepo, githubOrganizationsRepo, gitlabOrganizationRepo, eventsService)
	// gitlabOrganizationsService := gitlab_organizations.NewService(gitlabOrganizationRepo, v2RepositoriesService, v1ProjectClaGroupRepo)
	gitlabOrganizationService := gitlab_organizations.NewService(gitlabOrganizationRepo, v2RepositoriesService, v1ProjectClaGroupRepo, storeRepo, usersService, signaturesRepo, v1CompanyRepo)

	// Query GitLab Groups
	// for each group
	//    if enabled and auto-enabled = true
	//       load token and client
	//       query for GitLab API repository list
	//       query for GitLab repositories in DB for this group path
	//       identify deltas
	//       if new, add to DB, create event log
	//       if deleted, remove from DB, create event log

	// gitLabGroups, err := gitlabOrganizationRepo.GetGitLabOrganizationsEnabled(ctx)
	// if err != nil {
	// 	log.WithFields(f).WithError(err).Warnf("problem querying for GitLab group/organizations that are enabled with auto-enabled flag set to true")
	// 	return err
	// }
	gitLabGroups, err := gitlabOrganizationRepo.GetGitLabOrganizationsByProjectSFID(ctx, "a092h000004x5sGAAQ")
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem querying for GitLab group/organizations that are enabled with auto-enabled flag set to true")
		return err
	}

	log.WithFields(f).Debugf("start - checking %d GitLab projects for add/delete events", len(gitLabGroups.List))
	for _, gitLabGroup := range gitLabGroups.List {
		claGroupID := gitLabGroup.AutoEnabledClaGroupID
		log.WithFields(f).Debugf("start - processing GitLab group/organization: %s with group ID: %d associated with project SFID: %s", gitLabGroup.OrganizationURL, gitLabGroup.OrganizationExternalID, gitLabGroup.ProjectSfid)

		if claGroupID == "" {
			log.WithFields(f).Debugf("GitLab group/organization: %s not fully onboarded - missing CLA Group ID", gitLabGroup.OrganizationURL)
			pcg, err := v1ProjectClaGroupRepo.GetClaGroupIDForProject(ctx, gitLabGroup.ProjectSfid)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("problem querying for CLA Group ID for project SFID: %s", gitLabGroup.ProjectSfid)
				continue
			}
			log.WithFields(f).Debug("found CLA Group ID: ", pcg.ClaGroupID)
			claGroupID = pcg.ClaGroupID
		}

		if gitLabGroup.AuthInfo == "" {
			log.WithFields(f).Debugf("GitLab group/organization: %s not fully onboarded - missing authentication info - skipping", gitLabGroup.OrganizationURL)
			continue
		}

		oauthResponse, err := gitlabOrganizationService.RefreshGitLabOrganizationAuth(ctx, common.ToCommonModel(gitLabGroup))

		if err != nil {
			log.WithFields(f).WithError(err).Warnf("problem refreshing GitLab group/organization: %s authentication info - skipping", gitLabGroup.OrganizationURL)
			continue
		}

		gitLabClient, gitLabClientErr := gitLabApi.NewGitlabOauthClient(*oauthResponse, gitLabApp)
		if gitLabClientErr != nil {
			log.WithFields(f).WithError(gitLabClientErr).Warnf("problem loading GitLab client for group/organization: %s - skipping", gitLabGroup.OrganizationURL)
			continue
		}

		gitLabProjects, getGitLabAPIError := gitLabApi.GetGroupProjectListByGroupID(ctx, gitLabClient, int(gitLabGroup.OrganizationExternalID))
		if getGitLabAPIError != nil {
			log.WithFields(f).WithError(getGitLabAPIError).Warnf("problem loading GitLab projects for group/organization: %s using the groupID: %d - skipping GitLab Group/Organziation - skipping", gitLabGroup.OrganizationFullPath, gitLabGroup.OrganizationExternalID)
			continue
		}
		log.WithFields(f).Debugf("found %d GitLab projects for group/organization: %s", len(gitLabProjects), gitLabGroup.OrganizationFullPath)

		gitLabDBProjects, getProjectListDBErr := gitV2Repository.GitLabGetRepositoriesByOrganizationName(ctx, gitLabGroup.OrganizationFullPath)
		if getProjectListDBErr != nil {
			if _, ok := getProjectListDBErr.(*utils.GitLabRepositoryNotFound); ok {
				log.WithFields(f).Debugf("GitLab group/organization: %s does not have any repositories in the database", gitLabGroup.OrganizationFullPath)
			} else {
				log.WithFields(f).WithError(getProjectListDBErr).Warnf("problem loading GitLab projects for group/organization: %s from the database - skipping GitLab Group/Organziation - skipping", gitLabGroup.OrganizationFullPath)
				continue
			}
		}

		newGitLabProjects := getNewProjects(gitLabProjects, gitLabDBProjects)
		log.WithFields(f).Debugf("Found %d GitLab projects/repositories that are to be added for GitLab Group: %s", len(newGitLabProjects), gitLabGroup.OrganizationFullPath)
		if len(newGitLabProjects) > 0 {
			var gitLabProjectIDList []int64

			// Build a quick list of the GitLab Project/repo ID values - the add repositories takes a list
			for _, newGitLabProject := range newGitLabProjects {
				gitLabProjectIDList = append(gitLabProjectIDList, int64(newGitLabProject.ID))
			}

			// Add the repositories - will generate a log event
			_, addErr := v2RepositoriesService.GitLabAddRepositoriesWithEnabledFlag(ctx, gitLabGroup.ProjectSfid, &v2Repositories.GitLabAddRepoModel{
				ClaGroupID:    claGroupID,
				GroupName:     gitLabGroup.OrganizationName,
				ExternalID:    gitLabGroup.OrganizationExternalID,
				GroupFullPath: gitLabGroup.OrganizationFullPath,
				ProjectIDList: gitLabProjectIDList,
			}, true) // set to enabled when adding since this was added as a result of the auto-enable feature
			if addErr != nil {
				log.WithFields(f).WithError(addErr).Warnf("problem adding GitLab projects for group/organization: %s to the database", gitLabGroup.OrganizationFullPath)
			} else {
				log.WithFields(f).Debugf("added %d GitLab projects for group/organization: %s to the database", len(newGitLabProjects), gitLabGroup.OrganizationFullPath)
			}
		}

		gitLabProjects, getGitLabAPIError = gitLabApi.GetGroupProjectListByGroupID(ctx, gitLabClient, int(gitLabGroup.OrganizationExternalID))
		if getGitLabAPIError != nil {
			log.WithFields(f).WithError(getGitLabAPIError).Warnf("problem loading GitLab projects for group/organization: %s using the groupID: %d - skipping GitLab Group/Organziation - skipping", gitLabGroup.OrganizationFullPath, gitLabGroup.OrganizationExternalID)
			continue
		}
		log.WithFields(f).Debugf("found %d GitLab projects for group/organization: %s", len(gitLabProjects), gitLabGroup.OrganizationFullPath)

		dBProjects, getProjectListDBErr := gitV2Repository.GitLabGetRepositoriesByOrganizationName(ctx, gitLabGroup.OrganizationFullPath)
		if getProjectListDBErr != nil {
			if _, ok := getProjectListDBErr.(*utils.GitLabRepositoryNotFound); ok {
				log.WithFields(f).Debugf("GitLab group/organization: %s does not have any repositories in the database", gitLabGroup.OrganizationFullPath)
			} else {
				log.WithFields(f).WithError(getProjectListDBErr).Warnf("problem loading GitLab projects for group/organization: %s from the database - skipping GitLab Group/Organziation - skipping", gitLabGroup.OrganizationFullPath)
				continue
			}
		}
		log.WithFields(f).Debugf("Found %d GitLab projects/repositories for GitLab Group: %s", len(dBProjects), gitLabGroup.OrganizationFullPath)

		deletedGitLabProjects := getDeletedProjects(gitLabProjects, dBProjects)
		log.WithFields(f).Debugf("Found %d GitLab projects/repositories that are to be removed from the GitLab Group: %s", len(deletedGitLabProjects), gitLabGroup.OrganizationFullPath)
		if len(deletedGitLabProjects) > 0 {
			for _, gitLabProjectDBRecord := range deletedGitLabProjects {
				repositoryExternalID, parseIntErr := strconv.ParseInt(gitLabProjectDBRecord.RepositoryExternalID, 10, 64)
				if parseIntErr != nil {
					log.WithFields(f).WithError(parseIntErr).Warnf("problem converting repository %s external ID string value: %s to integer", gitLabProjectDBRecord.RepositoryFullPath, gitLabProjectDBRecord.RepositoryExternalID)
				} else {
					deleteErr := v2RepositoriesService.GitLabDeleteRepositoryByExternalID(ctx, repositoryExternalID)
					if deleteErr != nil {
						log.WithFields(f).WithError(deleteErr).Warnf("problem deleting repository %s external ID string value: %s to integer", gitLabProjectDBRecord.RepositoryFullPath, gitLabProjectDBRecord.RepositoryExternalID)
					} else {
						log.WithFields(f).Debugf("deleted GitLab project %s for group/organization: %s from the database", gitLabProjectDBRecord.RepositoryName, gitLabGroup.OrganizationFullPath)
					}
				}
			}
		}

		log.WithFields(f).Debugf("done - processed GitLab group/organization: %s with group ID: %d associated with project SFID: %s", gitLabGroup.OrganizationURL, gitLabGroup.OrganizationExternalID, gitLabGroup.ProjectSfid)
	}

	log.WithFields(f).Debugf("done - checked %d GitLab projects for add/delete events", len(gitLabGroups.List))
	return nil
}

// getNewProjects is a helper function to determine if we have any new GitLab projects that are not in our database
func getNewProjects(gitLabProjects []*goGitLab.Project, gitLabDBProjects []*v1Repositories.RepositoryDBModel) []*goGitLab.Project {
	var response []*goGitLab.Project
	f := logrus.Fields{
		"functionName": "getNewProjects",
	}
	if len(gitLabDBProjects) == 0 {
		// No projects in the database - return all the projects from GitLab
		log.WithFields(f).Debugf("no projects in the database - returning all projects from GitLab: %+v", gitLabProjects)
		return gitLabProjects
	}

	// For each GitLab Project/Repo
	for _, gitLabProject := range gitLabProjects {
		found := false

		// For each GitLab Project/Repo in the database
		for _, gitLabDBProject := range gitLabDBProjects {
			// Compare the full name/path
			if strings.ToLower(gitLabProject.PathWithNamespace) == strings.ToLower(gitLabDBProject.RepositoryFullPath) {
				found = true
				break
			}
		}

		// Didn't find the GitLab Project Repo from GitLab defined in our database - must have been added!
		if !found {
			// Add to our list
			response = append(response, gitLabProject)
		}
	}

	return response
}

// getDeletedProjects is a helper function to determine if we have any new GitLab projects that were removed from GitLab but are still in our database
func getDeletedProjects(gitLabProjects []*goGitLab.Project, gitLabDBProjects []*v1Repositories.RepositoryDBModel) []*v1Repositories.RepositoryDBModel {
	response := make([]*v1Repositories.RepositoryDBModel, 0)
	f := logrus.Fields{
		"functionName": "getDeletedProjects",
	}

	if len(gitLabProjects) == 0 {
		// No projects in GitLab - return all the projects from the database
		log.WithFields(f).Debugf("no projects in GitLab - returning all projects from the database: %+v", gitLabDBProjects)
		return gitLabDBProjects
	}

	log.WithFields(f).Debugf("len(gitLabProjects): %d and len(gitLabDbProjects): %d", len(gitLabProjects), len(gitLabDBProjects))

	// For each GitLab Project/Repo in the database
	for _, gitLabDBProject := range gitLabDBProjects {
		found := false

		// For each GitLab Project/Repo
		for _, gitLabProject := range gitLabProjects {
			// Compare the full name/path
			log.WithFields(f).Debugf("comparing GitLab project: %s with GitLab DB project: %s", gitLabProject.PathWithNamespace, gitLabDBProject.RepositoryFullPath)
			if strings.ToLower(gitLabProject.PathWithNamespace) == strings.ToLower(gitLabDBProject.RepositoryFullPath) {
				found = true
				break
			}

		}

		// Didn't find the GitLab Project Repo from the database defined in GitLab - must have been removed!
		if !found {
			// Add to our list
			log.WithFields(f).Debugf("adding GitLab project: %s to the list of projects to be deleted", gitLabDBProject.RepositoryFullPath)
			response = append(response, gitLabDBProject)
		} else {
			log.WithFields(f).Debugf("GitLab project: %s was not found in the list of projects to be deleted", gitLabDBProject.RepositoryFullPath)
		}
	}

	return response
}
