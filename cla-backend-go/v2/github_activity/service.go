// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_activity

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/gen/models"

	"github.com/communitybridge/easycla/cla-backend-go/v2/dynamo_events"

	"github.com/communitybridge/easycla/cla-backend-go/events"

	"github.com/communitybridge/easycla/cla-backend-go/repositories"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/google/go-github/v33/github"
)

// Service is responsible for handling the github activity events
type Service interface {
	ProcessInstallationRepositoriesEvent(event *github.InstallationRepositoriesEvent) error
	ProcessRepositoryEvent(*github.RepositoryEvent) error
}

type eventHandlerService struct {
	githubRepo        repositories.Repository
	eventService      events.Service
	autoEnableService dynamo_events.AutoEnableService
}

// NewService creates a new instance of the Event Handler Service
func NewService(githubRepo repositories.Repository,
	eventService events.Service,
	autoEnableService dynamo_events.AutoEnableService) Service {
	return &eventHandlerService{
		githubRepo:        githubRepo,
		eventService:      eventService,
		autoEnableService: autoEnableService,
	}
}

func (s *eventHandlerService) ProcessRepositoryEvent(event *github.RepositoryEvent) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "v2.github_activity.service.ProcessRepositoryEvent",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	log.Debugf("ProcessRepositoryEvent called for action : %s", *event.Action)
	if event.Action == nil {
		return fmt.Errorf("no action found in event payload")
	}
	switch *event.Action {
	case "created":
		return s.handleRepositoryAddedAction(ctx, event.Sender, event.Repo)
	case "deleted":
		return s.handleRepositoryRemovedAction(ctx, event.Sender, event.Repo)
	default:
		log.WithFields(f).Warnf("no handler for action : %s", *event.Action)
	}

	return nil

}

func (s *eventHandlerService) handleRepositoryAddedAction(ctx context.Context, sender *github.User, repo *github.Repository) error {
	f := logrus.Fields{
		"functionName":   "v2.github_activity.service.handleRepositoryAddedAction",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	if repo.ID == nil || *repo.ID == 0 {
		return fmt.Errorf("missing repo id")
	}

	if repo.Name == nil || *repo.Name == "" {
		return fmt.Errorf("repo name is missing")
	}

	if repo.FullName == nil || *repo.FullName == "" {
		return fmt.Errorf("repo full name missing")
	}

	repoModel, err := s.autoEnableService.CreateAutoEnabledRepository(repo)
	if err != nil {
		if errors.Is(err, dynamo_events.ErrAutoEnabledOff) {
			log.WithFields(f).Warnf("autoEnable is off for this repo : %s can't continue", *repo.FullName)
			return nil
		}
		return err
	}

	if err := s.autoEnableService.NotifyCLAManagerForRepos(repoModel.RepositoryProjectID, []*models.GithubRepository{repoModel}); err != nil {
		log.WithFields(f).Warnf("notifyCLAManager for autoEnabled repo : %s for claGroup : %s failed : %v", repoModel.RepositoryName, repoModel.RepositoryProjectID, err)
	}

	if sender == nil || sender.Login == nil || *sender.Login == "" {
		log.WithFields(f).Warnf("not able to send event empty sender")
		return nil
	}

	// sending the log event for the added repository
	log.Debugf("handleRepositoryAddedAction sending RepositoryAdded Event for repo %s", *repo.FullName)
	s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType: events.RepositoryAdded,
		ProjectID: repoModel.RepositoryProjectID,
		UserID:    *sender.Login,
		EventData: &events.RepositoryAddedEventData{
			RepositoryName: *repo.FullName,
		},
	})

	return nil
}

func (s *eventHandlerService) handleRepositoryRemovedAction(ctx context.Context, sender *github.User, repo *github.Repository) error {
	f := logrus.Fields{
		"functionName":   "v2.github_activity.service.handleRepositoryRemovedAction",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	if repo.ID == nil || *repo.ID == 0 {
		return fmt.Errorf("missing repo id")
	}
	repositoryExternalID := strconv.FormatInt(*repo.ID, 10)
	repoModel, err := s.githubRepo.GetRepositoryByGithubID(context.Background(), repositoryExternalID, true)
	if err != nil {
		if _, ok := err.(*utils.GitHubRepositoryNotFound); ok {
			log.WithFields(f).Warnf("event for non existing local repo : %s, nothing to do", *repo.FullName)
			return nil
		}
		return fmt.Errorf("fetching the repo : %s by external id : %s failed : %v", *repo.FullName, repositoryExternalID, err)
	}

	if err := s.githubRepo.DisableRepository(context.Background(), repoModel.RepositoryID); err != nil {
		log.WithFields(f).Warnf("disabling repo : %s failed : %v", *repo.FullName, err)
		return err
	}

	// sending event for the action
	s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType: events.RepositoryDisabled,
		ProjectID: repoModel.RepositoryProjectID,
		UserID:    *sender.Login,
		EventData: &events.RepositoryDisabledEventData{
			RepositoryName: *repo.FullName,
		},
	})

	return nil
}

func (s *eventHandlerService) ProcessInstallationRepositoriesEvent(event *github.InstallationRepositoriesEvent) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "v2.github_activity.service.ProcessInstallationRepositoriesEvent",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	log.Debugf("ProcessInstallationRepositoriesEvent called for action : %s", *event.Action)
	if event.Action == nil {
		return fmt.Errorf("no action found in event payload")
	}
	switch *event.Action {
	case "added":
		if len(event.RepositoriesAdded) == 0 {
			log.WithFields(f).Warnf("repositories list is empty nothing to add")
			return nil
		}

		for _, r := range event.RepositoriesAdded {
			if err := s.handleRepositoryAddedAction(ctx, event.Sender, r); err != nil {
				// we just log it don't want to stop the whole process at this stage
				log.WithFields(f).Warnf("adding the repository : %s failed : %v", *r.FullName, err)
			}
		}
	case "removed":
		if len(event.RepositoriesRemoved) == 0 {
			log.WithFields(f).Warnf("repositories list is empty nothing to remove")
			return nil
		}
		for _, r := range event.RepositoriesRemoved {
			if err := s.handleRepositoryRemovedAction(ctx, event.Sender, r); err != nil {
				log.WithFields(f).Warnf("removing the repository : %s failed : %v", *r.FullName, err)
			}
		}
	default:
		log.WithFields(f).Warnf("ProcessInstallationRepositoriesEvent no handler for action : %s", *event.Action)
	}

	return nil
}
