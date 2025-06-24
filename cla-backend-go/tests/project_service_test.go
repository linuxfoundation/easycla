// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	project_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
	"github.com/stretchr/testify/assert"
)

var functionalTestEnabled = false

func TestProjectServiceSummary(t *testing.T) {
	if functionalTestEnabled { // nolint
		var awsSession = session.Must(session.NewSession(&aws.Config{}))
		stage := os.Getenv("STAGE")
		assert.NotEmpty(t, stage)
		configFile, err := config.LoadConfig("", awsSession, stage)
		assert.Nil(t, err, "load config error")
		token.Init(configFile.Auth0Platform.ClientID, configFile.Auth0Platform.ClientSecret, configFile.Auth0Platform.URL, configFile.Auth0Platform.Audience)
		project_service.InitClient(configFile.PlatformAPIGatewayURL)

		client := project_service.GetClient()
		projectSummaryModel, err := client.GetSummary(utils.NewContext(), "a096s000000VluyAAC")
		assert.Nil(t, err, "Error is nil")
		assert.NotNil(t, projectSummaryModel, "Project Summary Response not nil")
	}
}
