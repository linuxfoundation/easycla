// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	gitlab_api "github.com/linuxfoundation/easycla/cla-backend-go/gitlab_api"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/common"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// GitLabOrgUpdatedEvent handles branch protection functionality
func (s *service) GitLabOrgUpdatedEvent(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "dynamodb_events.gitlab_organization.GitLabOrgUpdatedEvent",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"eventName":      event.EventName,
		"eventSource":    event.EventSource,
		"eventID":        event.EventID,
	}

	log.WithFields(f).Debug("processing event")
	var newGitLabOrg, oldGitLabOrg common.GitLabOrganization
	err := unmarshalStreamImage(event.Change.NewImage, &newGitLabOrg)
	if err != nil {
		log.WithFields(f).Warnf("problem unmarshalling the new gitlab organization model from the updated event, error: %+v", err)
		return err
	}
	err = unmarshalStreamImage(event.Change.OldImage, &oldGitLabOrg)
	if err != nil {
		log.WithFields(f).Warnf("problem unmarshalling the old gitlab organization model from the updated event, error: %+v", err)
		return err
	}

	f["gitlabOrgID"] = newGitLabOrg.OrganizationID
	f["gitlabOrgName"] = newGitLabOrg.OrganizationName

	if !newGitLabOrg.Enabled {
		log.WithFields(f).Debugf("gitlab org is not enabled, nothing to do this time")
		return nil
	}

	// If the branch protection value was updated from false to true....
	if !oldGitLabOrg.BranchProtectionEnabled && newGitLabOrg.BranchProtectionEnabled {
		log.WithFields(f).Debug("transition of branchProtectionEnabled false => true - processing...")
		return s.enableBranchProtectionForGitLabOrg(ctx, newGitLabOrg)
	}

	// it might be a new gitlab org that was just authenticated
	if oldGitLabOrg.AuthInfo != newGitLabOrg.AuthInfo && newGitLabOrg.BranchProtectionEnabled {
		log.WithFields(f).Debug("auth info was set for the org, processing the branch protection")
		return s.enableBranchProtectionForGitLabOrg(ctx, newGitLabOrg)
	}

	log.WithFields(f).Debug("no transition of branchProtectionEnabled false => true - ignoring...")
	return nil
}

func (s *service) enableBranchProtectionForGitLabOrg(ctx context.Context, newGitLabOrg common.GitLabOrganization) error {
	f := logrus.Fields{
		"functionName":            "dynamo_events.gitlab_organization.enableBranchProtectionForGitLabOrg",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"projectSFID":             newGitLabOrg.ProjectSFID,
		"organizationName":        newGitLabOrg.OrganizationName,
		"organizationSFID":        newGitLabOrg.OrganizationSFID,
		"autoEnabled":             newGitLabOrg.AutoEnabled,
		"branchProtectionEnabled": newGitLabOrg.BranchProtectionEnabled,
	}

	gitlabOrg, err := s.gitLabOrgRepo.GetGitLabOrganizationByName(ctx, newGitLabOrg.OrganizationName)
	if err != nil {
		return fmt.Errorf("fetching gitlab org : %s failed : %v", newGitLabOrg.OrganizationName, err)
	}

	oauthResponse, err := s.gitLabOrgService.RefreshGitLabOrganizationAuth(ctx, gitlabOrg)
	if err != nil {
		return fmt.Errorf("refreshing gitlab org auth failed : %v", err)
	}

	log.WithFields(f).Debugf("creating a new gitlab client object for org: %s...", newGitLabOrg.OrganizationName)
	gitLabClient, err := gitlab_api.NewGitlabOauthClient(*oauthResponse, s.gitLabApp)
	if err != nil {
		return fmt.Errorf("initializing GitLab client failed : %v", err)
	}

	// Locate the repositories already saved under this organization
	log.WithFields(f).Debugf("loading repositories under the organization : %s", newGitLabOrg.OrganizationName)
	repos, err := s.v2Repository.GitLabGetRepositoriesByOrganizationName(context.Background(), newGitLabOrg.OrganizationName)
	if err != nil {
		log.WithFields(f).Warnf("problem locating repositories by organization name, error: %+v", err)
		return err
	}

	var eg errgroup.Group
	// a pool of 5 concurrent workers
	var workerTokens = make(chan struct{}, 5)
	for _, repo := range repos {
		// this is for goroutine local variables
		repo := repo
		// acquire a worker token to create a new goroutine
		workerTokens <- struct{}{}
		// Update the branch protection in a go routine...
		eg.Go(func() error {
			defer func() {
				<-workerTokens // release the workerToken
			}()
			log.WithFields(f).Debugf("enabling branch protection for repository: %s", repo.RepositoryName)

			repositoryExternalIDInt, err := strconv.Atoi(repo.RepositoryExternalID)
			if err != nil {
				return fmt.Errorf("parsing external repository id failed : %v", err)
			}

			gitlabDefaultBranch, err := gitlab_api.GetDefaultBranch(gitLabClient, repositoryExternalIDInt)
			if err != nil {
				return fmt.Errorf("fetching default branch failed : %v", err)
			}

			err = gitlab_api.SetOrCreateBranchProtection(ctx, gitLabClient, repositoryExternalIDInt, gitlabDefaultBranch.Name)
			if err != nil {
				return fmt.Errorf("enabling branch protection for pattern : %s, failed : %v", gitlabDefaultBranch.Name, err)
			}
			return nil
		})
	}

	// Wait for the go routines to finish
	log.WithFields(f).Debugf("waiting for %d repositories to complete...", len(repos))
	var branchProtectionErr error
	if loadErr := eg.Wait(); loadErr != nil {
		log.WithFields(f).Warnf("encountered branch protection setup error: %+v", loadErr)
		branchProtectionErr = loadErr
	}

	return branchProtectionErr
}
