// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"fmt"
	"testing"

	gitlab2 "github.com/communitybridge/easycla/cla-backend-go/gitlab"
	"github.com/stretchr/testify/assert"
	"github.com/xanzy/go-gitlab"
)

const enabled = false // nolint

func TestGitLabSearchGroup(t *testing.T) { // no lint

	if enabled { // nolint
		// Get the client
		accessToken := "" //update to run this test
		group := "The Linux Foundation/product/EasyCLA"
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
