// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"context"

	"github.com/linuxfoundation/easycla/cla-backend-go/github/branch_protection"

	"github.com/linuxfoundation/easycla/cla-backend-go/repositories"
	"github.com/sirupsen/logrus"

	"github.com/aws/aws-lambda-go/events"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// GithubRepoModifyAddEvent github repository modify add event
func (s *service) GithubRepoModifyAddEvent(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "GitHubRepoModifyEvent",
		"eventID":        event.EventID,
		"eventName":      event.EventName,
		"eventSource":    event.EventSource,
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	var claGroupID string
	var projectSFID string
	var parentProjectSFID string
	var err error
	var newRepoModel repositories.RepositoryDBModel
	var oldRepoModel repositories.RepositoryDBModel
	// Check if record deleted
	if event.EventName == utils.RecordDeleted {
		log.WithFields(f).Debugf("processing record %s event...", event.EventName)
		err = unmarshalStreamImage(event.Change.OldImage, &oldRepoModel)
		if err != nil {
			log.WithFields(f).Warnf("problem unmarshalling old repository model event, error: %+v", err)
			return err
		}
		claGroupID = oldRepoModel.RepositoryCLAGroupID
		projectSFID = oldRepoModel.ProjectSFID
		parentProjectSFID = oldRepoModel.RepositorySfdcID
	} else {
		log.WithFields(f).Debugf("processing record %s event...", event.EventName)
		err := unmarshalStreamImage(event.Change.NewImage, &newRepoModel)
		if err != nil {
			log.WithFields(f).Warnf("problem unmarshalling the new repository model event, error: %+v", err)
			return err
		}
		claGroupID = newRepoModel.RepositoryCLAGroupID
		projectSFID = newRepoModel.ProjectSFID
		parentProjectSFID = newRepoModel.RepositorySfdcID
	}

	f["claGroupID"] = claGroupID
	f["projectSFID"] = projectSFID
	f["parentProjectSFID"] = parentProjectSFID

	// Set repository count
	log.WithFields(f).Debug("updating repository count")
	updateErr := s.setRepositoryCount(ctx, claGroupID, parentProjectSFID, projectSFID)
	if updateErr != nil {
		log.WithFields(f).WithError(updateErr).Warn("problem updating project-cla-group and project tables")
		return updateErr
	}
	return nil
}

// EnableBranchProtectionServiceHandler handles enabling the CLA Service attribute from the project service
func (s *service) EnableBranchProtectionServiceHandler(event events.DynamoDBEventRecord) error {
	f := logrus.Fields{
		"functionName": "EnableBranchProtectionServiceHandler",
		"eventID":      event.EventID,
		"eventName":    event.EventName,
		"eventSource":  event.EventSource,
	}

	log.WithFields(f).Debug("EnableBranchProtectionServiceHandler called...")
	var newRepoModel repositories.RepositoryDBModel
	err := unmarshalStreamImage(event.Change.NewImage, &newRepoModel)
	if err != nil {
		log.WithFields(f).Warnf("problem unmarshalling the new github organization add event, error: %+v", err)
		return err
	}

	f["repositoryName"] = newRepoModel.RepositoryName
	f["repositoryOrganizationName"] = newRepoModel.RepositoryOrganizationName
	f["projectSFID"] = newRepoModel.ProjectSFID

	// Branch protection only available for GitHub
	if newRepoModel.RepositoryType == utils.GitHubType {
		log.WithFields(f).Debugf("repository type is: %s", utils.GitHubType)

		parentOrgName := newRepoModel.RepositoryOrganizationName
		log.WithFields(f).Warnf("problem locating github organization by name: %s, error: %+v", parentOrgName, err)
		gitHubOrg, err := s.githubOrgService.GetGitHubOrganizationByName(context.Background(), parentOrgName)
		if err != nil {
			log.WithFields(f).Warnf("problem locating github organization by name: %s, error: %+v", parentOrgName, err)
			return nil
		}
		if gitHubOrg == nil {
			log.WithFields(f).Warnf("problem locating github organization by name: %s - record not found", parentOrgName)
			return nil
		}

		if gitHubOrg.BranchProtectionEnabled {
			log.WithFields(f).Debug("branch protection is enabled for this organization")

			ctx := context.Background()
			branchProtectionRepository, err := branch_protection.NewBranchProtectionRepository(gitHubOrg.OrganizationInstallationID, branch_protection.EnableBlockingLimiter())
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("initializing branch protection repository failed")
				return err
			}

			log.WithFields(f).Debugf("enabling branch protection on th default branch %s for the GitHub repository: %s...",
				utils.GithubBranchProtectionPatternAll, newRepoModel.RepositoryName)
			return branchProtectionRepository.EnableBranchProtection(ctx,
				parentOrgName, newRepoModel.RepositoryName,
				utils.GithubBranchProtectionPatternAll, true, []string{utils.GitHubBotName}, []string{})
		}

		log.WithFields(f).Debug("github organization branch protection is not enabled - no action required")
	}

	return nil
}

// DisableBranchProtectionServiceHandler handles disabling/removing the CLA Service attribute from the project service
func (s *service) DisableBranchProtectionServiceHandler(event events.DynamoDBEventRecord) error {
	f := logrus.Fields{
		"functionName": "DisableBranchProtectionServiceHandler",
		"eventID":      event.EventID,
		"eventName":    event.EventName,
		"eventSource":  event.EventSource,
	}

	log.WithFields(f).Debug("DisableBranchProtectionServiceHandler called")
	var oldRepoModel repositories.RepositoryDBModel
	err := unmarshalStreamImage(event.Change.OldImage, &oldRepoModel)
	if err != nil {
		log.WithFields(f).Warnf("problem unmarshalling the old github organization removed event, error: %+v", err)
		return err
	}

	// Branch protection only available for GitHub
	if oldRepoModel.RepositoryType == utils.GitHubType {
		parentOrgName := oldRepoModel.RepositoryOrganizationName
		gitHubOrg, err := s.githubOrgService.GetGitHubOrganizationByName(context.Background(), parentOrgName)
		if err != nil {
			log.WithFields(f).Warnf("problem locating github organization by name: %s, error: %+v", parentOrgName, err)
			return nil
		}
		if gitHubOrg == nil {
			log.WithFields(f).Warnf("problem locating github organization by name: %s - record not found", parentOrgName)
			return nil
		}

		if gitHubOrg.BranchProtectionEnabled {
			log.Debug("github organization branch protection is enabled - no cleanup action required")
			return nil
		}
		log.Debug("github organization branch protection is not enabled - no action required")
	}

	return nil
}

// setRepositoryCount helper function that sets repository count
func (s *service) setRepositoryCount(ctx context.Context, claGroupID string, parentProjectSFID, projectSFID string) error {
	f := logrus.Fields{
		"functionName":      "setRepositoryCount",
		"claGroupID":        claGroupID,
		"projectSFID":       projectSFID,
		"parentProjectSFID": parentProjectSFID,
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
	}

	log.WithFields(f).Debugf("Getting repositories for claGroup: %s ", claGroupID)
	repoCount := 0
	repos, repoErr := s.repositoryService.GetRepositoriesByCLAGroup(ctx, claGroupID)
	if repoErr != nil {
		log.WithFields(f).WithError(repoErr).Debugf("failed to get repositories for claGroup: %s ", claGroupID)
	} else {
		repoCount = len(repos)
	}
	log.WithFields(f).Debugf("Found %d repositories for claGroup: %s ", repoCount, claGroupID)

	//Update projects table
	log.WithFields(f).Debugf("Updating the root_projects_repository_count for claGroup : %s ", claGroupID)
	updateErr := s.projectRepo.UpdateRootCLAGroupRepositoriesCount(ctx, claGroupID, int64(repoCount), true)
	if updateErr != nil {
		log.WithFields(f).WithError(updateErr).Debugf("Failed to set repositories_count for claGroup: %s ", claGroupID)
		return updateErr
	}
	log.WithFields(f).Debugf("Updated the root_projects_repository_count in the cla-group table for claGroup : %s ", claGroupID)

	// Update projects-cla-group table
	log.WithFields(f).Debugf("Updating the projects-cla-groups-table for projectSFID: %s ", projectSFID)
	pcgErr := s.projectsClaGroupRepo.UpdateRepositoriesCount(ctx, projectSFID, int64(repoCount), true)
	if pcgErr != nil {
		log.WithFields(f).WithError(updateErr).Debugf("Failed to set repositories_count for project: %s ", projectSFID)
		return pcgErr
	}
	log.WithFields(f).Debugf("Updated the repository_count in the projects-cla-groups-table for projectSFID : %s ", projectSFID)

	return nil
}
