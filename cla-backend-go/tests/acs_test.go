// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	acs_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestGetAssignedRoles(t *testing.T) {
	if false {

		viper.AutomaticEnv()
		defaults := map[string]interface{}{
			"PORT":                    8080,
			"APP_ENV":                 "local",
			"USE_MOCK":                "False",
			"DB_MAX_CONNECTIONS":      1,
			"STAGE":                   "dev",
			"GH_ORG_VALIDATION":       "false",
			"COMPANY_USER_VALIDATION": "false",
			"LOG_FORMAT":              "text",
		}

		for key, value := range defaults {
			viper.SetDefault(key, value)
		}

		ini.Init()
		ini.AWSInit()
		awsSession, err := ini.GetAWSSession()
		assert.Nil(t, err, "aws session returns success")
		assert.NotNil(t, awsSession, "valid AWS session")
		stage := viper.GetString("STAGE")
		configFile, err := config.LoadConfig("", awsSession, stage)
		assert.Nil(t, err, "get config returns success")
		assert.NotNil(t, configFile, "valid config file")

		token.Init(configFile.Auth0Platform.ClientID, configFile.Auth0Platform.ClientSecret, configFile.Auth0Platform.URL, configFile.Auth0Platform.Audience)
		acs_service.InitClient(configFile.APIGatewayURL, configFile.AcsAPIKey)
		client := acs_service.GetClient()
		roleScope, err := client.GetAssignedRoles(utils.CLADesigneeRole, "a096s00000037xqAAA", "0016s000004ENL6AAO")
		assert.Nil(t, err, "get assigned roles returns success")
		assert.NotNil(t, roleScope, "role scope is not nil")
	}
}
