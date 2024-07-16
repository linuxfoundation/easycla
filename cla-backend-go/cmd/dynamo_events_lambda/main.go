// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/communitybridge/easycla/cla-backend-go/project/repository"
	"github.com/communitybridge/easycla/cla-backend-go/project/service"

	v2Repositories "github.com/communitybridge/easycla/cla-backend-go/v2/repositories"
	"github.com/communitybridge/easycla/cla-backend-go/v2/store"

	"github.com/communitybridge/easycla/cla-backend-go/v2/gitlab_organizations"

	gitlab "github.com/communitybridge/easycla/cla-backend-go/gitlab_api"

	"github.com/communitybridge/easycla/cla-backend-go/github_organizations"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/approval_list"
	"github.com/communitybridge/easycla/cla-backend-go/cla_manager"

	"github.com/communitybridge/easycla/cla-backend-go/gerrits"
	"github.com/communitybridge/easycla/cla-backend-go/repositories"

	acs_service "github.com/communitybridge/easycla/cla-backend-go/v2/acs-service"
	organization_service "github.com/communitybridge/easycla/cla-backend-go/v2/organization-service"
	project_service "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"
	user_service "github.com/communitybridge/easycla/cla-backend-go/v2/user-service"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/communitybridge/easycla/cla-backend-go/v2/approvals"
	"github.com/communitybridge/easycla/cla-backend-go/v2/dynamo_events"

	"github.com/communitybridge/easycla/cla-backend-go/token"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/github"
	v2Company "github.com/communitybridge/easycla/cla-backend-go/v2/company"

	claevents "github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/user"
	"github.com/communitybridge/easycla/cla-backend-go/users"

	"github.com/communitybridge/easycla/cla-backend-go/signatures"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/communitybridge/easycla/cla-backend-go/config"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

var (
	// version the application version
	version string

	// build/Commit the application build number
	commit string

	// branch the build branch
	branch string

	// build date
	buildDate string
)

var dynamoEventsService dynamo_events.Service

func init() {
	var awsSession = session.Must(session.NewSession(&aws.Config{}))
	stage := os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("stage not set")
	}
	log.Infof("STAGE set to %s\n", stage)
	configFile, err := config.LoadConfig("", awsSession, stage)
	if err != nil {
		log.Panicf("Unable to load config - Error: %v", err)
	}
	usersRepo := users.NewRepository(awsSession, stage)
	userRepo := user.NewDynamoRepository(awsSession, stage)
	companyRepo := company.NewRepository(awsSession, stage)
	projectClaGroupRepo := projects_cla_groups.NewRepository(awsSession, stage)
	v2Repository := v2Repositories.NewRepository(awsSession, stage)
	repositoriesRepo := repositories.NewRepository(awsSession, stage)
	gerritRepo := gerrits.NewRepository(awsSession, stage)
	projectRepo := repository.NewRepository(awsSession, stage, repositoriesRepo, gerritRepo, projectClaGroupRepo)
	eventsRepo := claevents.NewRepository(awsSession, stage)
	claManagerRequestsRepo := cla_manager.NewRepository(awsSession, stage)
	approvalListRequestsRepo := approval_list.NewRepository(awsSession, stage)
	githubOrganizationsRepo := github_organizations.NewRepository(awsSession, stage)
	gitlabOrganizationRepo := gitlab_organizations.NewRepository(awsSession, stage)
	storeRepo := store.NewRepository(awsSession, stage)
	approvalsTableName := fmt.Sprintf("cla-%s-approvals", stage)
	approvalRepo := approvals.NewRepository(stage, awsSession, approvalsTableName)

	token.Init(configFile.Auth0Platform.ClientID, configFile.Auth0Platform.ClientSecret, configFile.Auth0Platform.URL, configFile.Auth0Platform.Audience)
	github.Init(configFile.GitHub.AppID, configFile.GitHub.AppPrivateKey, configFile.GitHub.AccessToken)
	// initialize gitlab
	gitlabApp := gitlab.Init(configFile.Gitlab.AppClientID, configFile.Gitlab.AppClientSecret, configFile.Gitlab.AppPrivateKey)

	user_service.InitClient(configFile.APIGatewayURL, configFile.AcsAPIKey)
	project_service.InitClient(configFile.APIGatewayURL)
	githubOrganizationsService := github_organizations.NewService(githubOrganizationsRepo, repositoriesRepo, projectClaGroupRepo)
	repositoriesService := repositories.NewService(repositoriesRepo, githubOrganizationsRepo, projectClaGroupRepo)

	gerritService := gerrits.NewService(gerritRepo)
	// Services
	projectService := service.NewService(projectRepo, repositoriesRepo, gerritRepo, projectClaGroupRepo, usersRepo)

	type combinedRepo struct {
		users.UserRepository
		company.IRepository
		repository.ProjectRepository
		projects_cla_groups.Repository
	}

	eventsService := claevents.NewService(eventsRepo, combinedRepo{
		usersRepo,
		companyRepo,
		projectRepo,
		projectClaGroupRepo,
	})

	usersService := users.NewService(usersRepo, eventsService)
	signaturesRepo := signatures.NewRepository(awsSession, stage, companyRepo, usersRepo, eventsService, repositoriesRepo, githubOrganizationsRepo, gerritService, approvalRepo)
	v2RepositoryService := v2Repositories.NewService(repositoriesRepo, v2Repository, projectClaGroupRepo, githubOrganizationsRepo, gitlabOrganizationRepo, eventsService)
	gitlabOrgService := gitlab_organizations.NewService(gitlabOrganizationRepo, v2RepositoryService, projectClaGroupRepo, storeRepo, usersService, signaturesRepo, companyRepo)

	companyService := company.NewService(companyRepo, configFile.CorporateConsoleV1URL, userRepo, usersService)
	v2CompanyService := v2Company.NewService(companyService, signaturesRepo, projectRepo, usersRepo, companyRepo, projectClaGroupRepo, eventsService)
	organization_service.InitClient(configFile.APIGatewayURL, eventsService)
	acs_service.InitClient(configFile.APIGatewayURL, configFile.AcsAPIKey)
	dynamoEventsService = dynamo_events.NewService(
		stage,
		signaturesRepo,
		companyRepo,
		v2CompanyService,
		projectClaGroupRepo,
		eventsRepo,
		projectRepo,
		gitlabOrganizationRepo,
		v2Repository,
		projectService,
		githubOrganizationsService,
		repositoriesService,
		gerritService,
		claManagerRequestsRepo,
		approvalListRequestsRepo,
		gitlabApp,
		gitlabOrgService,
	)
}

func handler(ctx context.Context, event events.DynamoDBEvent) {
	dynamoEventsService.ProcessEvents(event)
}

func printBuildInfo() {
	log.Infof("Version                 : %s", version)
	log.Infof("Git commit hash         : %s", commit)
	log.Infof("Branch                  : %s", branch)
	log.Infof("Build date              : %s", buildDate)
}

func main() {
	log.Info("Lambda server starting...")
	printBuildInfo()
	if os.Getenv("LOCAL_MODE") == "true" {
		var dynamodbEvent events.DynamoDBEvent
		args := os.Args[1:]
		if len(args) > 0 {
			if err := json.Unmarshal([]byte(args[0]), &dynamodbEvent); err != nil {
				log.Fatal(err)
			}
		}
		handler(utils.NewContext(), dynamodbEvent)
	} else {
		lambda.Start(handler)
	}
	log.Infof("Lambda shutting down...")
}
