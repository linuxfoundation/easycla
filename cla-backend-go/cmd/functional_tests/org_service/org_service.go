// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package org_service

import (
	"context"

	"github.com/linuxfoundation/easycla/cla-backend-go/project/repository"

	"github.com/linuxfoundation/easycla/cla-backend-go/cmd/functional_tests/test_models"
	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gerrits"
	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/repositories"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	acs_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	organization_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
	"github.com/spf13/viper"
)

// TestBehaviour data model
type TestBehaviour struct {
	apiURL           string
	auth0User1Config test_models.Auth0Config
	auth0User2Config test_models.Auth0Config
}

// NewTestBehaviour creates a new test behavior model
func NewTestBehaviour(apiURL string, auth0User1Config, auth0User2Config test_models.Auth0Config) *TestBehaviour {
	return &TestBehaviour{
		apiURL + "/v3",
		auth0User1Config,
		auth0User2Config,
	}
}

// RunIsUserHaveRoleScope test
func (t *TestBehaviour) RunIsUserHaveRoleScope() {
	stage := viper.GetString("STAGE")

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		log.Panicf("Unable to load AWS session - Error: %v", err)
	}

	configFile, err := config.LoadConfig("", awsSession, stage)
	if err != nil {
		log.Panicf("Unable to load config - Error: %v", err)
	}

	// Our service layer handlers
	type combinedRepo struct {
		users.UserRepository
		company.IRepository
		repository.ProjectRepository
		projects_cla_groups.Repository
	}

	eventsRepo := events.NewRepository(awsSession, stage)
	usersRepo := users.NewRepository(awsSession, stage)
	companyRepo := company.NewRepository(awsSession, stage)
	repositoriesRepo := repositories.NewRepository(awsSession, stage)
	gerritRepo := gerrits.NewRepository(awsSession, stage)
	projectClaGroupRepo := projects_cla_groups.NewRepository(awsSession, stage)
	projectRepo := repository.NewRepository(awsSession, stage, repositoriesRepo, gerritRepo, projectClaGroupRepo)

	eventsService := events.NewService(eventsRepo, combinedRepo{
		usersRepo,
		companyRepo,
		projectRepo,
		projectClaGroupRepo,
	})

	organization_service.InitClient(configFile.APIGatewayURL, eventsService)
	acs_service.InitClient(configFile.APIGatewayURL, configFile.AcsAPIKey)
	acsClient := acs_service.GetClient()
	roleID, roleErr := acsClient.GetRoleID("cla-manager")
	if roleErr != nil {
		log.Fatalf("unable to read role: cla-manager from ACS Client, error: %+v", roleErr)
	}

	log.Debugf("Role ID for cla-manager-role : %s", roleID)
	orgClient := organization_service.GetClient()
	userSFID := "clamanager1devintel"
	hasScope, err := orgClient.IsUserHaveRoleScope(context.Background(), roleID, userSFID, "00117000015vpjXAAQ", "a092M00001IfVmKQAV")
	if err != nil {
		log.Fatalf("unable to invoke org client IsUserHaveRoleScope, error: %+v", err)
	}
	if !hasScope {
		log.Fatalf("user: %s does not have scope for a092M00001IfVmKQAV|00117000015vpjXAAQ", userSFID)
	}
}

// RunAllTests runs all the Organization Service Tests
func (t *TestBehaviour) RunAllTests() {
	t.RunIsUserHaveRoleScope()
}
