// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"os"
	"testing"

	gitlab_api "github.com/linuxfoundation/easycla/cla-backend-go/gitlab_api"
	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/gitlab_organizations"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestGitLabAccessTokenDecode(t *testing.T) { // no lint
	if false { // nolint
		ctx := utils.NewContext()
		// Need to initialize the system to load the configuration which contains a number of SSM parameters
		stage := os.Getenv("STAGE")
		if stage == "" {
			assert.Fail(t, "set STAGE environment variable to run unit and functional tests.")
		}
		dynamodbRegion := os.Getenv("DYNAMODB_AWS_REGION")
		if dynamodbRegion == "" {
			assert.Fail(t, "set DYNAMODB_AWS_REGION environment variable to run unit and functional tests.")
		}

		viper.Set("STAGE", stage)
		viper.Set("DYNAMODB_AWS_REGION", dynamodbRegion)
		ini.Init()
		awsSession, err := ini.GetAWSSession()
		if err != nil {
			assert.Fail(t, "unable to load AWS session", err)
		}
		ini.ConfigVariable()
		config := ini.GetConfig()

		// Create a new GitLab App client instance
		gitLabApp := gitlab_api.Init(config.Gitlab.AppClientID, config.Gitlab.AppClientSecret, config.Gitlab.AppPrivateKey)

		gitLabOrgRepo := gitlab_organizations.NewRepository(awsSession, stage)

		gitlabOrg, err := gitLabOrgRepo.GetGitLabOrganizationByFullPath(ctx, "linuxfoundation/product/easycla")
		assert.Nil(t, err, "get gitlab organization by name error should be nil")
		assert.NotNil(t, gitlabOrg, "gitlab organization should not nil")
		oauthResp, err := gitlab_api.DecryptAuthInfo(gitlabOrg.AuthInfo, gitLabApp)
		assert.Nil(t, err, "decrypt auth info error should be nil")
		assert.NotNil(t, oauthResp, "oauth response should not be nil")
		t.Logf("decoded oauth client with access token : %s", oauthResp.AccessToken)
	}
}
