// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"fmt"
	"testing"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	gitlab2 "github.com/communitybridge/easycla/cla-backend-go/gitlab"
	"github.com/stretchr/testify/assert"
	"github.com/xanzy/go-gitlab"
)

const enabled = false // nolint
const group = "The Linux Foundation/product/EasyCLA"

func TestGitLabSearchGroup(t *testing.T) { // no lint

	if enabled { // nolint
		// Get the client
		accessToken := "" //update to run this test
		gitLabClient, err := gitlab2.NewGitlabOauthClientFromAccessToken(accessToken)
		assert.Nil(t, err, "GitLab OAuth Client")

		// Need to look up the GitLab Group/Organization to obtain the ID
		opts := &gitlab.ListGroupsOptions{
			ListOptions: gitlab.ListOptions{},
		}
		groups, resp, searchErr := gitLabClient.Groups.ListGroups(opts)
		assert.Nil(t, searchErr, "GitLab OAuth Client")
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			assert.Fail(t, "unable to locate GitLab group by name: %s, status code: %d", group, resp.StatusCode)
		}
		for _, g := range groups {
			t.Logf("group name: %s, ID: %d, path: %s", g.Name, g.ID, g.Path)
		}
		if len(groups) != 1 {

			assert.Fail(t, fmt.Sprintf("expecting 1 result for GitLab group name '%s' search, found: %d - %+v", group, len(groups), groups))
		}
	}
}

func TestGitLabListProjects(t *testing.T) { // no lint

	if enabled { // nolint
		// Get the client
		accessToken := "" //update to run this test
		gitLabClient, err := gitlab2.NewGitlabOauthClientFromAccessToken(accessToken)
		assert.Nil(t, err, "GitLab OAuth Client")

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

		t.Logf("Recevied %d projects", len(projects))
		for _, p := range projects {
			t.Logf("project name: %s, ID: %d, path: %s", p.Name, p.ID, p.PathWithNamespace)
		}
		if len(projects) > 1 {
			assert.Fail(t, fmt.Sprintf("expecting > 1 result for GitLab list projects, found: %d - %+v", len(projects), projects))
		}
	}
}
