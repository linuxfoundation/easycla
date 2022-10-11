// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/communitybridge/easycla/cla-backend-go/github"
	ini "github.com/communitybridge/easycla/cla-backend-go/init"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestGitHubGetUserDetails(t *testing.T) {
	if gitHubTestsEnabled {
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
		_, err := ini.GetAWSSession()
		if err != nil {
			assert.Fail(t, "unable to load AWS session", err)
		}
		ini.ConfigVariable()
		config := ini.GetConfig()
		github.Init(config.GitHub.AppID, config.GitHub.AppPrivateKey, config.GitHub.AccessToken)

		// Test data - dogfood my own user account
		var gitHubUsername = "dealako"
		var gitHubUserEmail = "ddeal@linuxfoundation.org"
		var gitHubUserID = int64(519609)

		gitHubUserModel, gitHubErr := github.GetUserDetails(gitHubUsername)
		assert.Nil(t, gitHubErr, fmt.Sprintf("unable to get GitHub user details using GitHub username: %s", gitHubUsername))
		assert.NotNil(t, gitHubUserModel, fmt.Sprintf("GitHub user model is nil for GitHub username: %s", gitHubUsername))

		assert.NotNil(t, gitHubUserModel.Login, fmt.Sprintf("GitHub user model login value is nil for GitHub username: %s", gitHubUsername))
		assert.Equal(t, gitHubUsername, *gitHubUserModel.Login, fmt.Sprintf("GitHub username does not match for GitHub username: %s", gitHubUsername))

		assert.NotNil(t, gitHubUserModel.ID, fmt.Sprintf("GitHub user model response ID is nil for GitHub username: %s", gitHubUsername))
		assert.Equal(t, gitHubUserID, *gitHubUserModel.ID, fmt.Sprintf("GitHub user ID does not match for GitHub username: %s - expecting: %d", gitHubUsername, gitHubUserID))

		assert.NotNil(t, gitHubUserModel.Email, fmt.Sprintf("GitHub user model email is nil for GitHub username: %s", gitHubUsername))
		assert.Equal(t, gitHubUserEmail, *gitHubUserModel.Email, fmt.Sprintf("GitHub user email does not match for GitHub username: %s - expecting: %s", gitHubUsername, gitHubUserEmail))
	}
}
