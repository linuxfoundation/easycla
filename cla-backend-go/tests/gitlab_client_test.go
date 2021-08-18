// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"testing"

	ini "github.com/communitybridge/easycla/cla-backend-go/init"
	"github.com/spf13/viper"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	gitlab2 "github.com/communitybridge/easycla/cla-backend-go/gitlab"
	"github.com/stretchr/testify/assert"
	"github.com/xanzy/go-gitlab"
)

const enabled = false // nolint
const group = "The Linux Foundation/product/EasyCLA"
const accessInfo = ""

const easyCLAGroupName = "linuxfoundation/product/easycla"

func TestGitLabGetGroup(t *testing.T) { // no lint

	if enabled { // nolint
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
		gitLabApp := gitlab2.Init(config.Gitlab.AppClientID, config.Gitlab.AppClientSecret, config.Gitlab.AppPrivateKey)

		// Create a new client
		gitLabClient, err := gitlab2.NewGitlabOauthClient(accessInfo, gitLabApp)
		assert.Nil(t, err, "GitLab OAuth Client Error is Nil")
		assert.NotNil(t, gitLabClient, "GitLab OAuth Client is Not Nil")

		// Need to look up the GitLab Group/Organization to obtain the ID
		groupModel, resp, getError := gitLabClient.Groups.GetGroup(url.QueryEscape(easyCLAGroupName))
		assert.Nil(t, getError, "GitLab GetGroup Error is Nil")
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			assert.Fail(t, fmt.Sprintf("unable to locate GitLab group by value: %s, status code: %d", easyCLAGroupName, resp.StatusCode))
		}
		assert.NotNil(t, groupModel, "Group Model is not nil")
		t.Logf("group name: %s, ID: %d, path: %s", groupModel.Name, groupModel.ID, groupModel.Path)
	}
}

func TestGitLabListGroups(t *testing.T) { // no lint

	if enabled { // nolint
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
		gitLabApp := gitlab2.Init(config.Gitlab.AppClientID, config.Gitlab.AppClientSecret, config.Gitlab.AppPrivateKey)

		// Create a new client
		gitLabClient, err := gitlab2.NewGitlabOauthClient(accessInfo, gitLabApp)
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
			t.Logf("name: %s, id: %d, web url: %s, path: %s, full path: %s", g.Name, g.ID, g.WebURL, g.Path, g.FullPath)
		}
	}
}

func TestGitLabListProjects(t *testing.T) { // no lint

	if enabled { // nolint
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
		gitLabApp := gitlab2.Init(config.Gitlab.AppClientID, config.Gitlab.AppClientSecret, config.Gitlab.AppPrivateKey)

		// Create a new client
		gitLabClient, err := gitlab2.NewGitlabOauthClient(accessInfo, gitLabApp)
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
		t.Logf("Recevied %d projects", len(projects))
		for _, p := range projects {
			t.Logf("project name: %s, ID: %d, path: %s", p.Name, p.ID, p.PathWithNamespace)
		}

		// DEBUG
		t.Log("projects:")
		for _, p := range projects {
			byteArr, err := json.Marshal(p)
			assert.Nil(t, err)
			t.Logf("project: %s", byteArr)
		}

		if len(projects) > 1 {
			assert.Fail(t, fmt.Sprintf("expecting > 1 result for GitLab list projects, found: %d - %+v", len(projects), projects))
		}
	}
}
