// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"fmt"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	gitlab_api "github.com/linuxfoundation/easycla/cla-backend-go/gitlab_api"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/repositories"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

func (s *service) GitLabRepoAddedWebhookEventHandler(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "GitLabRepoAddedWebhookEventHandler",
		"eventID":        event.EventID,
		"eventName":      event.EventName,
		"eventSource":    event.EventSource,
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	var newRepoModel repositories.RepositoryDBModel

	log.WithFields(f).Debugf("processing record %s event...", event.EventName)
	err := unmarshalStreamImage(event.Change.NewImage, &newRepoModel)
	if err != nil {
		log.WithFields(f).Warnf("problem unmarshalling the new repository model event, error: %+v", err)
		return err
	}

	if !s.isGitlabRepo(log.WithFields(f), &newRepoModel) {
		return nil
	}

	if !newRepoModel.Enabled {
		log.WithFields(f).Debugf("gitlab repo is not enabled, nothing to do at this point")
		return nil
	}

	repositoryID := newRepoModel.RepositoryID
	repositoryName := newRepoModel.RepositoryName
	repositoryExternalID := newRepoModel.RepositoryExternalID

	log.WithFields(f).Debugf("adding webhook for repository : %s:%s with external id : %s", repositoryID, repositoryName, repositoryExternalID)

	gitlabOrg, err := s.gitLabOrgRepo.GetGitLabOrganizationByName(ctx, newRepoModel.RepositoryOrganizationName)
	if err != nil {
		return fmt.Errorf("fetching gitlab org : %s failed : %v", newRepoModel.RepositoryOrganizationName, err)
	}

	oauthResponse, err := s.gitLabOrgService.RefreshGitLabOrganizationAuth(ctx, gitlabOrg)
	if err != nil {
		return fmt.Errorf("refreshing gitlab org auth failed : %v", err)
	}

	gitLabClient, err := gitlab_api.NewGitlabOauthClient(*oauthResponse, s.gitLabApp)
	if err != nil {
		return fmt.Errorf("initializing GitLab client failed : %v", err)
	}

	repositoryExternalIDInt, err := strconv.Atoi(repositoryExternalID)
	if err != nil {
		return fmt.Errorf("parsing external repository id failed : %v", err)
	}

	conf := config.GetConfig()
	if err := gitlab_api.SetWebHook(gitLabClient, conf.Gitlab.WebHookURI, repositoryExternalIDInt, gitlabOrg.AuthState); err != nil {
		log.WithFields(f).Errorf("adding gitlab webhook failed : %v", err)
	}
	log.WithFields(f).Debugf("gitlab webhhok added succesfully for repository")

	log.WithFields(f).Debugf("enabling gitlab pipeline protection if not alreasy")
	return gitlab_api.EnableMergePipelineProtection(ctx, gitLabClient, repositoryExternalIDInt)
}

func (s *service) GitlabRepoModifiedWebhookEventHandler(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "GitlabRepoModifiedWebhookEventHandler",
		"eventID":        event.EventID,
		"eventName":      event.EventName,
		"eventSource":    event.EventSource,
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	var newRepoModel repositories.RepositoryDBModel
	var oldRepoModel repositories.RepositoryDBModel

	log.WithFields(f).Debugf("processing record %s event...", event.EventName)
	err := unmarshalStreamImage(event.Change.OldImage, &oldRepoModel)
	if err != nil {
		log.WithFields(f).Warnf("problem unmarshalling the new repository model event, error: %+v", err)
		return err
	}

	err = unmarshalStreamImage(event.Change.NewImage, &newRepoModel)
	if err != nil {
		log.WithFields(f).Warnf("problem unmarshalling the old repository model event, error: %+v", err)
		return err
	}

	if !s.isGitlabRepo(log.WithFields(f), &newRepoModel) {
		return nil
	}

	if newRepoModel.Enabled == oldRepoModel.Enabled {
		log.WithFields(f).Debugf("only changes of Enabled field are processed")
		return nil
	}

	repositoryID := oldRepoModel.RepositoryID
	repositoryName := oldRepoModel.RepositoryName
	repositoryExternalID := oldRepoModel.RepositoryExternalID

	if newRepoModel.Enabled {
		log.WithFields(f).Debugf("adding webhook for repository : %s:%s with external id : %s", repositoryID, repositoryName, repositoryExternalID)
	} else {
		log.WithFields(f).Debugf("removing webhook for repository : %s:%s with external id : %s", repositoryID, repositoryName, repositoryExternalID)
	}

	gitlabOrg, err := s.gitLabOrgRepo.GetGitLabOrganizationByName(ctx, oldRepoModel.RepositoryOrganizationName)
	if err != nil {
		return fmt.Errorf("fetching gitlab org : %s failed : %v", oldRepoModel.RepositoryOrganizationName, err)
	}

	oauthResponse, err := s.gitLabOrgService.RefreshGitLabOrganizationAuth(ctx, gitlabOrg)
	if err != nil {
		return fmt.Errorf("refreshing gitlab org auth failed : %v", err)
	}

	gitLabClient, err := gitlab_api.NewGitlabOauthClient(*oauthResponse, s.gitLabApp)
	if err != nil {
		return fmt.Errorf("initializing GitLab client failed : %v", err)
	}

	repositoryExternalIDInt, err := strconv.Atoi(repositoryExternalID)
	if err != nil {
		return fmt.Errorf("parding external repository id failed : %v", err)
	}

	conf := config.GetConfig()

	if newRepoModel.Enabled {
		if err := gitlab_api.SetWebHook(gitLabClient, conf.Gitlab.WebHookURI, repositoryExternalIDInt, gitlabOrg.AuthState); err != nil {
			log.WithFields(f).Errorf("adding gitlab webhook failed : %v", err)
		}
		log.WithFields(f).Debugf("enabling gitlab pipeline protection if not alreasy")
		if err := gitlab_api.EnableMergePipelineProtection(ctx, gitLabClient, repositoryExternalIDInt); err != nil {
			return err
		}
	} else {
		if err := gitlab_api.RemoveWebHook(gitLabClient, conf.Gitlab.WebHookURI, repositoryExternalIDInt); err != nil {
			log.WithFields(f).Errorf("removing gitlab webhook failed : %v", err)
		}
	}

	log.WithFields(f).Debugf("gitlab webhhok processed succesfully for repository")
	return nil
}

func (s *service) GitLabRepoRemovedWebhookEventHandler(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "GitLabRepoRemovedWebhookEventHandler",
		"eventID":        event.EventID,
		"eventName":      event.EventName,
		"eventSource":    event.EventSource,
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	var oldRepoModel repositories.RepositoryDBModel

	log.WithFields(f).Debugf("processing record %s event...", event.EventName)
	err := unmarshalStreamImage(event.Change.OldImage, &oldRepoModel)
	if err != nil {
		log.WithFields(f).Warnf("problem unmarshalling the old repository model event, error: %+v", err)
		return err
	}

	if !s.isGitlabRepo(log.WithFields(f), &oldRepoModel) {
		return nil
	}

	repositoryID := oldRepoModel.RepositoryID
	repositoryName := oldRepoModel.RepositoryName
	repositoryExternalID := oldRepoModel.RepositoryExternalID

	log.WithFields(f).Debugf("removing webhook for repository : %s:%s with external id : %s", repositoryID, repositoryName, repositoryExternalID)

	gitlabOrg, err := s.gitLabOrgRepo.GetGitLabOrganizationByName(ctx, oldRepoModel.RepositoryOrganizationName)
	if err != nil {
		return fmt.Errorf("fetching gitlab org : %s failed : %v", oldRepoModel.RepositoryOrganizationName, err)
	}

	oauthResponse, err := s.gitLabOrgService.RefreshGitLabOrganizationAuth(ctx, gitlabOrg)
	if err != nil {
		return fmt.Errorf("refreshing gitlab org auth failed : %v", err)
	}

	gitLabClient, err := gitlab_api.NewGitlabOauthClient(*oauthResponse, s.gitLabApp)
	if err != nil {
		return fmt.Errorf("initializing GitLab client failed : %v", err)
	}

	repositoryExternalIDInt, err := strconv.Atoi(repositoryExternalID)
	if err != nil {
		return fmt.Errorf("parding external repository id failed : %v", err)
	}

	conf := config.GetConfig()
	if err := gitlab_api.RemoveWebHook(gitLabClient, conf.Gitlab.WebHookURI, repositoryExternalIDInt); err != nil {
		log.WithFields(f).Errorf("removing gitlab webhook failed : %v", err)
	}

	log.WithFields(f).Debugf("gitlab webhhok removed succesfully for repository")
	return nil
}

func (s *service) isGitlabRepo(logEntry *logrus.Entry, repoModel *repositories.RepositoryDBModel) bool {
	if repoModel.RepositoryType != utils.GitLabLower {
		logEntry.Debugf("only processing gitlab instances")
		return false
	}
	return true
}
