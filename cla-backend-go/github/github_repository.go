// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"errors"
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/google/go-github/v37/github"
)

// errors
var (
	ErrGithubRepositoryNotFound = errors.New("github repository not found")
)

// GetRepositoryByExternalID finds github repository by github repository id
func GetRepositoryByExternalID(ctx context.Context, installationID, id int64) (*github.Repository, error) {
	client, err := NewGithubAppClient(installationID)
	if err != nil {
		return nil, err
	}
	org, resp, err := client.Repositories.GetByID(ctx, id)
	if err != nil {
		logging.Warnf("GetRepository %v failed. error = %s", id, err.Error())
		if resp.StatusCode == 404 {
			return nil, ErrGithubRepositoryNotFound
		}
		return nil, err
	}
	return org, nil
}

// GetRepositories gets github repositories by organization
func GetRepositories(ctx context.Context, organizationName string) ([]*github.Repository, error) {
	f := logrus.Fields{
		"functionName":     "GetRepositories",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"organizationName": organizationName,
	}

	// Get the client with token
	client := NewGithubOauthClient()

	var responseRepoList []*github.Repository
	var nextPage = 1
	for {
		// API https://docs.github.com/en/free-pro-team@latest/rest/reference/repos
		// API Pagination: https://docs.github.com/en/free-pro-team@latest/rest/guides/traversing-with-pagination
		repoList, resp, err := client.Repositories.ListByOrg(ctx, organizationName, &github.RepositoryListByOrgOptions{
			Type:      "public",
			Sort:      "full_name",
			Direction: "asc",
			ListOptions: github.ListOptions{
				Page:    nextPage,
				PerPage: 100,
			},
		})
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to list repositories for organization")
			if resp != nil && resp.StatusCode == 404 {
				return nil, ErrGithubOrganizationNotFound
			}
			return nil, err
		}

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			msg := fmt.Sprintf("GetRepositories %s failed with no success response code %d. error = %s", organizationName, resp.StatusCode, err.Error())
			log.WithFields(f).Warnf(msg)
			return nil, errors.New(msg)
		}

		// Append our results to the response...
		responseRepoList = append(responseRepoList, repoList...)
		// if no more pages...
		if resp.NextPage == 0 {
			break
		}

		// update our next page value
		nextPage = resp.NextPage
	}

	return responseRepoList, nil
}
