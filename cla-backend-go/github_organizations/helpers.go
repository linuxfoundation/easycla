// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_organizations

import (
	"context"
	"fmt"
	netURL "net/url"
	"sync"

	"github.com/go-openapi/strfmt"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

func buildGithubOrganizationListModels(ctx context.Context, githubOrganizations []*GithubOrganization) []*models.GithubOrganization {
	f := logrus.Fields{
		"functionName":   "buildGitHubOrganizationListModels",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	// Convert the database model to a response model
	ghOrgList := toModels(githubOrganizations)

	if len(ghOrgList) > 0 {
		var wg sync.WaitGroup
		wg.Add(len(ghOrgList))

		for _, ghorganization := range ghOrgList {
			go func(ghorg *models.GithubOrganization) {
				defer wg.Done()
				ghorg.GithubInfo = &models.GithubOrganizationGithubInfo{}
				log.WithFields(f).Debugf("loading GitHub organization details: %s...", ghorg.OrganizationName)
				user, err := github.GetUserDetails(ghorg.OrganizationName)
				if err != nil {
					ghorg.GithubInfo.Error = err.Error()
				} else {
					url := strfmt.URI(*user.HTMLURL)
					installURL := netURL.URL{
						Scheme: "https",
						Host:   "github.com",
						Path:   fmt.Sprintf("/organizations/%s/settings/installations/%d", ghorg.OrganizationName, ghorg.OrganizationInstallationID),
					}
					installationURL := strfmt.URI(installURL.String())
					ghorg.GithubInfo.Details = &models.GithubOrganizationGithubInfoDetails{
						Bio:             user.Bio,
						HTMLURL:         &url,
						ID:              user.ID,
						InstallationURL: &installationURL,
					}
				}

				ghorg.Repositories = &models.GithubOrganizationRepositories{
					List: make([]*models.GithubRepositoryInfo, 0),
				}

				if ghorg.OrganizationInstallationID != 0 {
					log.WithFields(f).Debugf("loading GitHub repository list directly from GitHub based on the installation id: %d...", ghorg.OrganizationInstallationID)
					list, err := github.GetInstallationRepositories(ctx, ghorg.OrganizationInstallationID)
					if err != nil {
						log.WithFields(f).Warnf("unable to get repositories from GitHub for the installation id: %d", ghorg.OrganizationInstallationID)
						ghorg.Repositories.Error = err.Error()
						return
					}

					log.WithFields(f).Debugf("found %d repositories from GitHub using the installation id: %d...",
						len(list), ghorg.OrganizationInstallationID)
					for _, repoInfo := range list {
						ghorg.Repositories.List = append(ghorg.Repositories.List, &models.GithubRepositoryInfo{
							RepositoryGithubID: utils.Int64Value(repoInfo.ID),
							RepositoryName:     utils.StringValue(repoInfo.FullName),
							RepositoryURL:      utils.StringValue(repoInfo.URL),
							RepositoryType:     "github",
						})
					}
				}
			}(ghorganization)
		}
		wg.Wait()
	}
	return ghOrgList
}
