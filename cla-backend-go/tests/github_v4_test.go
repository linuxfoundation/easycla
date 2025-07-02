// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/github/branch_protection"

	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	"github.com/spf13/viper"

	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

const gitHubTestsEnabled = false // nolint

func TestGetRepositoryIDFromName(t *testing.T) {
	if gitHubTestsEnabled {
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
		_, err := ini.GetAWSSession()
		if err != nil {
			assert.Fail(t, "unable to load AWS session", err)
		}
		ini.ConfigVariable()
		config := ini.GetConfig()
		github.Init(config.GitHub.AppID, config.GitHub.AppPrivateKey, config.GitHub.AccessToken)
		installationID, int64Err := strconv.ParseInt(config.GitHub.TestOrganizationInstallationID, 10, 64)
		if int64Err != nil {
			assert.Fail(t, fmt.Sprintf("unable to convert installation ID to string: %s", config.GitHub.TestOrganizationInstallationID), int64Err)
		}

		branchProtectionRepoV4, err := branch_protection.NewBranchProtectionRepositoryV4(installationID)
		if err != nil {
			assert.Fail(t, fmt.Sprintf("initializing branch protection v4 repo failed : %v", err))
		}
		expectedValue := config.GitHub.TestRepositoryID
		actualValue, err := branchProtectionRepoV4.GetRepositoryIDFromName(ctx, config.GitHub.TestOrganization, config.GitHub.TestRepository)
		if err != nil {
			assert.Fail(t, fmt.Sprintf("unable to create GitHub v4 client from installation ID: %d", installationID), err)
		}
		assert.Equal(t, expectedValue, actualValue, "CombinedRepository ID Lookup")
	}

}
