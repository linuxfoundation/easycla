// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"fmt"
	"io"
	"os"
	"testing"

	ini "github.com/communitybridge/easycla/cla-backend-go/init"
	"github.com/spf13/viper"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	gitlab_api "github.com/communitybridge/easycla/cla-backend-go/gitlab_api"
	"github.com/stretchr/testify/assert"
	"github.com/xanzy/go-gitlab"
)

const gitLabTestsEnabled = false                     // nolint
const group = "The Linux Foundation/product/EasyCLA" // nolint
const accessInfo = ""

const easyCLAGroupName = "linuxfoundation/product/easycla" // nolint

func TestGetGroupByName(t *testing.T) { // no lint
	if gitLabTestsEnabled { // nolint
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

		// Create a new GitLab App client instance
		gitLabApp := gitlab_api.Init(config.Gitlab.AppClientID, config.Gitlab.AppClientSecret, config.Gitlab.AppPrivateKey)

		// Create a new client
		gitLabClient, err := gitlab_api.NewGitlabOauthClient(accessInfo, gitLabApp)
		assert.Nil(t, err, "GitLab OAuth Client Error is Nil")
		assert.NotNil(t, gitLabClient, "GitLab OAuth Client is Not Nil")

		ctx := utils.NewContext()
		// Need to look up the GitLab Group/Organization to obtain the ID
		//groupModel, getError := gitlab_api.GetGroupByName(ctx, gitLabClient, easyCLAGroupName)
		//groupModel, getError := gitlab_api.GetGroupByName(ctx, gitLabClient, "EasyCLA")
		groupModel, getError := gitlab_api.GetGroupByName(ctx, gitLabClient, "linuxfoundation/product/asitha")
		assert.Nil(t, getError, "GitLab GetGroup Error should be nil", getError)
		assert.NotNil(t, groupModel, "Group Model should not be nil")
		t.Logf("group ID: %d, name: %s, path: %s, full path: %s", groupModel.ID, groupModel.Name, groupModel.Path, groupModel.FullPath)
	}
}

func TestGetGroupByID(t *testing.T) { // no lint
	if gitLabTestsEnabled { // nolint
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

		// Create a new GitLab App client instance
		gitLabApp := gitlab_api.Init(config.Gitlab.AppClientID, config.Gitlab.AppClientSecret, config.Gitlab.AppPrivateKey)

		// Create a new client
		gitLabClient, err := gitlab_api.NewGitlabOauthClient(accessInfo, gitLabApp)
		assert.Nil(t, err, "GitLab OAuth Client Error is Nil")
		assert.NotNil(t, gitLabClient, "GitLab OAuth Client is Not Nil")

		ctx := utils.NewContext()
		groupModel, getError := gitlab_api.GetGroupByID(ctx, gitLabClient, 13050017)
		assert.Nil(t, getError, "GitLab GetGroup Error should be nil", getError)
		assert.NotNil(t, groupModel, "Group Model should not be nil")
		t.Logf("group ID: %d, name: %s, path: %s, full path: %s", groupModel.ID, groupModel.Name, groupModel.Path, groupModel.FullPath)
	}
}

func TestGetGroupByFullPath(t *testing.T) { // no lint
	if gitLabTestsEnabled { // nolint
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

		// Create a new GitLab App client instance
		gitLabApp := gitlab_api.Init(config.Gitlab.AppClientID, config.Gitlab.AppClientSecret, config.Gitlab.AppPrivateKey)

		// Create a new client
		gitLabClient, err := gitlab_api.NewGitlabOauthClient(accessInfo, gitLabApp)
		assert.Nil(t, err, "GitLab OAuth Client Error is Nil")
		assert.NotNil(t, gitLabClient, "GitLab OAuth Client is Not Nil")

		ctx := utils.NewContext()
		groupModel, getError := gitlab_api.GetGroupByFullPath(ctx, gitLabClient, "linuxfoundation/product/asitha")
		assert.Nil(t, getError, "GitLab GetGroup Error should be nil", getError)
		assert.NotNil(t, groupModel, "Group Model should not be nil")
		t.Logf("group ID: %d, name: %s, path: %s, full path: %s", groupModel.ID, groupModel.Name, groupModel.Path, groupModel.FullPath)
	}
}

func TestGetGroupProjectListByGroupID(t *testing.T) { // no lint
	if gitLabTestsEnabled { // nolint
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

		// Create a new GitLab App client instance
		gitLabApp := gitlab_api.Init(config.Gitlab.AppClientID, config.Gitlab.AppClientSecret, config.Gitlab.AppPrivateKey)

		// Create a new client
		gitLabClient, err := gitlab_api.NewGitlabOauthClient(accessInfo, gitLabApp)
		assert.Nil(t, err, "GitLab OAuth Client Error is Nil")
		assert.NotNil(t, gitLabClient, "GitLab OAuth Client is Not Nil")

		ctx := utils.NewContext()
		gitLabProjects, getError := gitlab_api.GetGroupProjectListByGroupID(ctx, gitLabClient, 13050017)
		assert.Nil(t, getError, "Get Group Projects List by Group ID error should be nil", getError)
		assert.NotNil(t, gitLabProjects, "Get Group Projects Array should not be nil")
		assert.Greaterf(t, len(gitLabProjects), 0, "Get Group Projects Array greater than zero: %d", len(gitLabProjects))
		for _, p := range gitLabProjects {
			t.Logf("id: %d, name: %s, web url: %s, path: %s, full path: %s", p.ID, p.Name, p.WebURL, p.Path, p.PathWithNamespace)
		}
	}
}

func TestGitLabListGroups(t *testing.T) { // no lint

	if gitLabTestsEnabled { // nolint
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

		// Create a new GitLab App client instance
		gitLabApp := gitlab_api.Init(config.Gitlab.AppClientID, config.Gitlab.AppClientSecret, config.Gitlab.AppPrivateKey)

		// Create a new client
		gitLabClient, err := gitlab_api.NewGitlabOauthClient(accessInfo, gitLabApp)
		assert.Nil(t, err, "GitLab OAuth Client Error is Nil")
		assert.NotNil(t, gitLabClient, "GitLab OAuth Client is Not Nil")

		// Need to look up the GitLab Group/Organization to obtain the ID
		opts := &gitlab.ListGroupsOptions{
			ListOptions: gitlab.ListOptions{
				Page:    1,
				PerPage: 100,
			},
		}

		groups, resp, searchErr := gitLabClient.Groups.ListGroups(opts)
		assert.Nil(t, searchErr, "GitLab List Groups Error is Nil")
		if searchErr != nil {
			t.Logf("list groups error: %+v", searchErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			respBody, readErr := io.ReadAll(resp.Body)
			assert.Nil(t, readErr, "GitLab Response Body Read is Nil")
			assert.Fail(t, fmt.Sprintf("unable to list GitLab groups, status code: %d, body: %s", resp.StatusCode, respBody))
		}
		for _, g := range groups {
			t.Logf("name: %s, id: %d, path: %s, full path: %s, web url: %s", g.Name, g.ID, g.Path, g.FullPath, g.WebURL)
		}
	}
}

func TestGitLabListProjects(t *testing.T) { // no lint

	if gitLabTestsEnabled { // nolint
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

		// Create a new GitLab App client instance
		gitLabApp := gitlab_api.Init(config.Gitlab.AppClientID, config.Gitlab.AppClientSecret, config.Gitlab.AppPrivateKey)

		// Create a new client
		gitLabClient, err := gitlab_api.NewGitlabOauthClient(accessInfo, gitLabApp)
		assert.Nil(t, err, "GitLab OAuth Client Error is Nil")
		assert.NotNil(t, gitLabClient, "GitLab OAuth Client is Not Nil")

		// Query GitLab for repos - fetch the list of repositories available to the GitLab App
		listProjectsOpts := &gitlab.ListProjectsOptions{
			ListOptions: gitlab.ListOptions{
				Page:    1, // starts with one: https://docs.gitlab.com/ee/api/#offset-based-pagination
				PerPage: 100,
			},
			Archived:                 nil,
			Visibility:               nil,
			OrderBy:                  nil,
			Sort:                     nil,
			Search:                   utils.StringRef("linuxfoundation"),
			SearchNamespaces:         utils.Bool(true),
			Simple:                   nil,
			Owned:                    nil,
			Membership:               utils.Bool(true),
			Starred:                  nil,
			Statistics:               nil,
			Topic:                    nil,
			WithCustomAttributes:     nil,
			WithIssuesEnabled:        nil,
			WithMergeRequestsEnabled: nil,
			WithProgrammingLanguage:  nil,
			WikiChecksumFailed:       nil,
			RepositoryChecksumFailed: nil,
			MinAccessLevel:           gitlab.AccessLevel(gitlab.MaintainerPermissions),
			IDAfter:                  nil,
			IDBefore:                 nil,
			LastActivityAfter:        nil,
			LastActivityBefore:       nil,
		}

		// Need to use this func to get the list of projects the user has access to, see: https://gitlab.com/gitlab-org/gitlab-foss/-/issues/63811
		projects, resp, listProjectsErr := gitLabClient.Projects.ListProjects(listProjectsOpts)
		assert.Nil(t, listProjectsErr, "GitLab OAuth Client")
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			assert.Fail(t, "unable to locate GitLab group by name: %s, status code: %d", group, resp.StatusCode)
		}

		// DEBUG
		//t.Logf("Recevied %d projects", len(projects))
		//for _, p := range projects {
		//	t.Logf("project name: %s, ID: %d, path: %s", p.Name, p.ID, p.PathWithNamespace)
		//}

		// DEBUG
		//t.Log("projects:")
		//for _, p := range projects {
		//byteArr, err := json.Marshal(p)
		//assert.Nil(t, err)
		//t.Logf("project: %s", byteArr)
		//}

		if len(projects) > 1 {
			assert.Fail(t, fmt.Sprintf("expecting > 1 result for GitLab list projects, found: %d - %+v", len(projects), projects))
		}
	}
}

func TestGitLabGetUserByUsername(t *testing.T) {

	if gitLabTestsEnabled { // nolint
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

		// Create a new GitLab App client instance
		gitLabApp := gitlab_api.Init(config.Gitlab.AppClientID, config.Gitlab.AppClientSecret, config.Gitlab.AppPrivateKey)
		assert.NotNil(t, gitLabApp, "GitLab App reference is Not Nil")

		// Create a new client
		gitLabClient, err := gitlab_api.NewGitlabOauthClient(accessInfo, gitLabApp)
		assert.Nil(t, err, "GitLab OAuth Client Error is Nil")
		assert.NotNil(t, gitLabClient, "GitLab OAuth Client is Not Nil")

		// Test data - dogfood my own user account
		var gitLabUsername = "dealako"

		opts := &gitlab.ListUsersOptions{
			ListOptions: gitlab.ListOptions{
				Page:    1, // starts with one: https://docs.gitlab.com/ee/api/#offset-based-pagination
				PerPage: 100,
			},
			Username: utils.StringRef(gitLabUsername),
		}
		userList, resp, err := gitLabClient.Users.ListUsers(opts, nil)
		assert.Nil(t, err, "GitLab OAuth Client")
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			assert.Failf(t, "GitLab List Users API Response Error", "unable to locate GitLab user by name: %s, status code: %d", gitLabUsername, resp.StatusCode)
		}
		assert.NotEqualf(t, 1, len(userList), "GitLab List Users Response Error - expecting 1 result for GitLab list users, found: %d - %+v", len(userList), userList)
	}
}
