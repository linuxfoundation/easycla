// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_activity

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/linuxfoundation/easycla/cla-backend-go/emails"
	v1GithubOrg "github.com/linuxfoundation/easycla/cla-backend-go/github_organizations"

	"github.com/sirupsen/logrus"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"

	"github.com/linuxfoundation/easycla/cla-backend-go/v2/dynamo_events"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"

	"github.com/linuxfoundation/easycla/cla-backend-go/repositories"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/google/go-github/v37/github"
)

// Service is responsible for handling the github activity events
type Service interface {
	ProcessInstallationRepositoriesEvent(event *github.InstallationRepositoriesEvent) error
	ProcessRepositoryEvent(*github.RepositoryEvent) error
}

type eventHandlerService struct {
	gitV1Repository   repositories.RepositoryInterface
	githubOrgRepo     v1GithubOrg.RepositoryInterface
	eventService      events.Service
	autoEnableService dynamo_events.AutoEnableService
	emailService      emails.Service
	sendEmail         bool
}

// NewService creates a new instance of the Event Handler Service
func NewService(gitV1Repository repositories.RepositoryInterface,
	githubOrgRepo v1GithubOrg.RepositoryInterface,
	eventService events.Service,
	autoEnableService dynamo_events.AutoEnableService,
	emailService emails.Service) Service {

	return newService(gitV1Repository, githubOrgRepo, eventService, autoEnableService, emailService, true)
}

func newService(gitV1Repository repositories.RepositoryInterface,
	githubOrgRepo v1GithubOrg.RepositoryInterface,
	eventService events.Service,
	autoEnableService dynamo_events.AutoEnableService,
	emailService emails.Service,
	sendEmail bool) Service {
	return &eventHandlerService{
		gitV1Repository:   gitV1Repository,
		githubOrgRepo:     githubOrgRepo,
		eventService:      eventService,
		autoEnableService: autoEnableService,
		emailService:      emailService,
		sendEmail:         sendEmail,
	}
}

func (s *eventHandlerService) ProcessRepositoryEvent(event *github.RepositoryEvent) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "v2.github_activity.service.ProcessRepositoryEvent",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	if event.Action == nil {
		return fmt.Errorf("no action found in event payload")
	}

	if event.Repo == nil {
		return fmt.Errorf("missing repository object in event payload")
	}

	log.Debugf("ProcessRepositoryEvent called for action : %s for repository : %s", *event.Action, *event.Repo.Name)
	switch *event.Action {
	case "created":
		return s.handleRepositoryAddedAction(ctx, event.Sender, event.Repo)
	case "renamed":
		return s.handleRepositoryRenamedAction(ctx, event.Sender, event.Repo)
	case "transferred":
		if event.Org == nil {
			return fmt.Errorf("missing org object in event payload")
		}
		return s.handleRepositoryTransferredAction(ctx, event.Sender, event.Repo, event.Org)
	case "deleted":
		return s.handleRepositoryRemovedAction(ctx, event.Sender, event.Repo)
	case "archived":
		return s.handleRepositoryArchivedAction(ctx, event.Sender, event.Repo)
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

	if err := s.autoEnableService.NotifyCLAManagerForRepos(repoModel.RepositoryClaGroupID, []*models.GithubRepository{repoModel}); err != nil {
		log.WithFields(f).Warnf("notifyCLAManager for autoEnabled repo : %s for claGroup : %s failed : %v", repoModel.RepositoryName, repoModel.RepositoryClaGroupID, err)
	}

	if sender == nil || sender.Login == nil || *sender.Login == "" {
		log.WithFields(f).Warnf("not able to send event empty sender")
		return nil
	}

	// sending the log event for the added repository
	log.Debugf("handleRepositoryAddedAction sending RepositoryAdded Event for repo %s", *repo.FullName)
	s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType:   events.RepositoryAdded,
		ProjectSFID: repoModel.RepositoryProjectSfid,
		CLAGroupID:  repoModel.RepositoryClaGroupID,
		UserID:      *sender.Login,
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
	repoModel, err := s.gitV1Repository.GitHubGetRepositoryByExternalID(context.Background(), repositoryExternalID)
	if err != nil {
		if _, ok := err.(*utils.GitHubRepositoryNotFound); ok {
			log.WithFields(f).Warnf("event for non existing local repo : %s, nothing to do", *repo.FullName)
			return nil
		}
		return fmt.Errorf("fetching the repo : %s by external id : %s failed : %v", *repo.FullName, repositoryExternalID, err)
	}
	if !repoModel.Enabled {
		log.WithFields(f).Infof("repo : %s already disabled, set repository as remote deleted", repoModel.RepositoryID)
		err = s.gitV1Repository.GitHubSetRemoteDeletedRepository(ctx, repoModel.RepositoryID, true, false)
		if err != nil {
			return fmt.Errorf("setting repo : %s remote deleted failed : %v", *repo.FullName, err)
		}
		return nil
	}
	err = s.gitV1Repository.GitHubSetRemoteDeletedRepository(ctx, repoModel.RepositoryID, true, true)
	if err != nil {
		return fmt.Errorf("setting repo : %s remote deleted failed : %v", *repo.FullName, err)
	}
	log.WithFields(f).Infof("disabling repo : %s", repoModel.RepositoryID)
	if err := s.gitV1Repository.GitHubDisableRepository(context.Background(), repoModel.RepositoryID); err != nil {
		log.WithFields(f).Warnf("disabling repo : %s failed : %v", *repo.FullName, err)
		return err
	}
	// sending event for the action
	s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType:   events.RepositoryDisabled,
		ProjectSFID: repoModel.RepositoryProjectSfid,
		CLAGroupID:  repoModel.RepositoryClaGroupID,
		UserID:      *sender.Login,
		EventData: &events.RepositoryDisabledEventData{
			RepositoryName: *repo.FullName,
		},
	})

	if s.sendEmail {
		subject := fmt.Sprintf("EasyCLA: Github Repository Was Removed")
		body, err := emails.RenderGithubRepositoryDisabledTemplate(s.emailService, repoModel.RepositoryClaGroupID, emails.GithubRepositoryDisabledTemplateParams{
			GithubRepositoryActionTemplateParams: emails.GithubRepositoryActionTemplateParams{
				CommonEmailParams: emails.CommonEmailParams{
					RecipientName: "CLA Manager",
				},
				RepositoryName: repoModel.RepositoryName,
			},
			GithubAction: "deleted",
		})

		if err != nil {
			log.WithFields(f).Warnf("rendering email template failed : %v", err)
			return nil
		}

		if err := s.emailService.NotifyClaManagersForClaGroupID(context.Background(), repoModel.RepositoryClaGroupID, subject, body); err != nil {
			log.WithFields(f).Warnf("notifying cla managers via email failed : %v", err)
		}

	}

	return nil
}

// handles the event when a repository is renamed so we rename the repo in our records as well
func (s *eventHandlerService) handleRepositoryRenamedAction(ctx context.Context, sender *github.User, repo *github.Repository) error {
	f := logrus.Fields{
		"functionName":   "v2.github_activity.service.handleRepositoryRenamedAction",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	if repo.ID == nil || *repo.ID == 0 {
		return fmt.Errorf("missing repo id")
	}
	repositoryExternalID := strconv.FormatInt(*repo.ID, 10)
	repoModel, err := s.gitV1Repository.GitHubGetRepositoryByGithubID(context.Background(), repositoryExternalID, true)
	if err != nil {
		if _, ok := err.(*utils.GitHubRepositoryNotFound); ok {
			log.WithFields(f).Warnf("event for non existing local repo : %s, nothing to do", *repo.FullName)
			return nil
		}
		return fmt.Errorf("fetching the repo : %s by external id : %s failed : %v", *repo.FullName, repositoryExternalID, err)
	}

	log.WithFields(f).Infof("renaming Github Repository from : %s to : %s", repoModel.RepositoryName, *repo.Name)

	if _, err := s.gitV1Repository.GitHubUpdateRepository(ctx, repoModel.RepositoryID, "", "", &models.GithubRepositoryInput{
		RepositoryName: repo.Name,
		Note:           "repository was renamed externally",
	}); err != nil {
		log.WithFields(f).Warnf("renaming repo : %s failed : %v", *repo.FullName, err)
		return err
	}

	if sender == nil || sender.Login == nil {
		return fmt.Errorf("missing sender can not log the event")
	}

	// sending event for the action
	s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType:   events.RepositoryRenamed,
		ProjectSFID: repoModel.RepositoryProjectSfid,
		CLAGroupID:  repoModel.RepositoryClaGroupID,
		UserID:      *sender.Login,
		EventData: &events.RepositoryRenamedEventData{
			NewRepositoryName: *repo.Name,
			OldRepositoryName: repoModel.RepositoryName,
		},
	})

	if s.sendEmail {
		subject := fmt.Sprintf("EasyCLA: Github Repository Was Renamed")
		body, err := emails.RenderGithubRepositoryRenamedTemplate(s.emailService, repoModel.RepositoryClaGroupID, emails.GithubRepositoryRenamedTemplateParams{
			GithubRepositoryActionTemplateParams: emails.GithubRepositoryActionTemplateParams{
				CommonEmailParams: emails.CommonEmailParams{
					RecipientName: "CLA Manager",
				},
				RepositoryName: repoModel.RepositoryName,
			},
			NewRepositoryName: *repo.Name,
			OldRepositoryName: repoModel.RepositoryName,
		})

		if err != nil {
			log.WithFields(f).Warnf("rendering email template failed : %v", err)
			return nil
		}

		if err := s.emailService.NotifyClaManagersForClaGroupID(context.Background(), repoModel.RepositoryClaGroupID, subject, body); err != nil {
			log.WithFields(f).Warnf("notifying cla managers via email failed : %v", err)
		}

	}

	return nil
}

func (s *eventHandlerService) handleRepositoryTransferredAction(ctx context.Context, sender *github.User, repo *github.Repository, org *github.Organization) error {
	if repo.Name == nil {
		return fmt.Errorf("missing repo name can't proceed with transfer")
	}
	repoName := *repo.Name

	if org.Login == nil {
		return fmt.Errorf("missing organization login information can't proceed with transferring the rpo : %s", *org.Name)
	}

	f := logrus.Fields{
		"functionName":          "v2.github_activity.service.handleRepositoryTransferredAction",
		"repositoryName":        repoName,
		"newGithubOrganization": *org.Login,
		utils.XREQUESTID:        ctx.Value(utils.XREQUESTID),
	}

	if repo.ID == nil || *repo.ID == 0 {
		return fmt.Errorf("missing repo id")
	}

	repositoryExternalID := strconv.FormatInt(*repo.ID, 10)
	repoModel, err := s.gitV1Repository.GitHubGetRepositoryByGithubID(context.Background(), repositoryExternalID, true)
	if err != nil {
		if _, ok := err.(*utils.GitHubRepositoryNotFound); ok {
			log.WithFields(f).Warnf("event for non existing local repo : %s, nothing to do", repoName)
			return nil
		}
		return fmt.Errorf("fetching the repo : %s by external id : %s failed : %v", repoName, repositoryExternalID, err)
	}

	newOrganizationName := *org.Login
	oldOrganizationName := repoModel.RepositoryOrganizationName

	log.WithFields(f).Infof("running transfer for repository : %s from Github Org : %s to Github Org : %s", repoName, oldOrganizationName, newOrganizationName)

	// first check if it's a different organization name (could be a duplicate event)
	if oldOrganizationName == newOrganizationName {
		msg := fmt.Sprintf("nothing to change for github repo : %s, probably duplicate event was sent", repoModel.RepositoryName)
		log.WithFields(f).Warnf("%s", msg)
		return fmt.Errorf("%s", msg)
	}

	// fetch the old and the new github orgs from the db
	oldGithubOrg, err := s.githubOrgRepo.GetGitHubOrganization(ctx, oldOrganizationName)
	if err != nil {
		return fmt.Errorf("fetching the old organization name : %s failed : %v", oldOrganizationName, err)
	}

	newGithubOrg, err := s.githubOrgRepo.GetGitHubOrganization(ctx, newOrganizationName)
	if err != nil {
		disabledErr := s.disableFailedTransferRepo(ctx, sender, f, repoModel, oldGithubOrg, newGithubOrg)
		if disabledErr != nil {
			return disabledErr
		}

		return fmt.Errorf("fetching the new organization name : %s failed : %v", newOrganizationName, err)
	}

	// we need to check if the new org name has autoenabled and have a cla group set otherwise we can't proceed
	if !newGithubOrg.AutoEnabled || newGithubOrg.AutoEnabledClaGroupID == "" {
		disabledErr := s.disableFailedTransferRepo(ctx, sender, f, repoModel, oldGithubOrg, newGithubOrg)
		if disabledErr != nil {
			return disabledErr
		}

		return fmt.Errorf("aborting the repository : %s transfer, new githubOrg : %s doesn't have claGroupID set", repoModel.RepositoryName, newGithubOrg.OrganizationName)
	}

	_, err = s.gitV1Repository.GitHubUpdateRepository(ctx, repoModel.RepositoryID, "", "", &models.GithubRepositoryInput{
		Note:                       fmt.Sprintf("repository was transferred from org : %s to : %s", oldGithubOrg.OrganizationName, newGithubOrg.OrganizationName),
		RepositoryOrganizationName: aws.String(newGithubOrg.OrganizationName),
		RepositoryURL:              repo.HTMLURL,
	})

	if err != nil {
		return fmt.Errorf("repository : %s transfer failed for new github org : %s : %v", repoModel.RepositoryID, newGithubOrg.OrganizationName, err)
	}

	// sending event for the action
	s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType:   events.RepositoryTransferred,
		ProjectSFID: repoModel.RepositoryProjectSfid,
		CLAGroupID:  repoModel.RepositoryClaGroupID,
		UserID:      *sender.Login,
		EventData: &events.RepositoryTransferredEventData{
			RepositoryName:   repoModel.RepositoryName,
			OldGithubOrgName: oldGithubOrg.OrganizationName,
			NewGithubOrgName: newGithubOrg.OrganizationName,
		},
	})

	if s.sendEmail {
		if err := s.notifyForGithubRepositoryTransferred(ctx, repoModel, oldGithubOrg, newGithubOrg, true); err != nil {
			log.WithFields(f).Warnf("notifying cla managers via email failed : %v", err)
		}
	}

	return nil
}

func (s *eventHandlerService) disableFailedTransferRepo(ctx context.Context, sender *github.User, f logrus.Fields, repoModel *models.GithubRepository, oldGithubOrg *models.GithubOrganization, newGithubOrg *models.GithubOrganization) error {
	log.WithFields(f).Warnf("can't proceed with repo transfer operation because the new org doesn't have autoenabled=true, disabling the repo : %s", repoModel.RepositoryName)
	if err := s.gitV1Repository.GitHubDisableRepository(ctx, repoModel.RepositoryID); err != nil {
		return fmt.Errorf("disabling the repo : %s failed : %v", repoModel.RepositoryID, err)
	}

	// send event for the disabled repository.
	s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType:   events.RepositoryDisabled,
		ProjectSFID: repoModel.RepositoryProjectSfid,
		CLAGroupID:  repoModel.RepositoryClaGroupID,
		UserID:      *sender.Login,
		EventData: &events.RepositoryDisabledEventData{
			RepositoryName: repoModel.RepositoryName,
		},
	})

	if s.sendEmail {
		if err := s.notifyForGithubRepositoryTransferred(ctx, repoModel, oldGithubOrg, newGithubOrg, false); err != nil {
			log.WithFields(f).Warnf("notifying cla managers via email failed : %v", err)
		}
	}
	return nil
}

func (s *eventHandlerService) notifyForGithubRepositoryTransferred(ctx context.Context, repoModel *models.GithubRepository, oldGithubOrg *models.GithubOrganization, newGithubOrg *models.GithubOrganization, success bool) error {
	subject := fmt.Sprintf("EasyCLA: Github Repository Was Transferred")
	body, err := emails.RenderGithubRepositoryTransferredTemplate(s.emailService, repoModel.RepositoryClaGroupID, emails.GithubRepositoryTransferredTemplateParams{
		GithubRepositoryActionTemplateParams: emails.GithubRepositoryActionTemplateParams{
			CommonEmailParams: emails.CommonEmailParams{
				RecipientName: "CLA Manager",
			},
			RepositoryName: repoModel.RepositoryName,
		},
		OldGithubOrgName: oldGithubOrg.OrganizationName,
		NewGithubOrgName: newGithubOrg.OrganizationName,
	}, success)

	if err != nil {
		return fmt.Errorf("rendering email template failed : %v", err)
	}

	err = s.emailService.NotifyClaManagersForClaGroupID(ctx, repoModel.RepositoryClaGroupID, subject, body)
	return err
}

func (s *eventHandlerService) handleRepositoryArchivedAction(ctx context.Context, sender *github.User, repo *github.Repository) error {
	f := logrus.Fields{
		"functionName":   "v2.github_activity.service.handleRepositoryArchivedAction",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	if repo.ID == nil || *repo.ID == 0 {
		return fmt.Errorf("missing repo id")
	}
	repositoryExternalID := strconv.FormatInt(*repo.ID, 10)
	repoModel, err := s.gitV1Repository.GitHubGetRepositoryByGithubID(context.Background(), repositoryExternalID, true)
	if err != nil {
		if _, ok := err.(*utils.GitHubRepositoryNotFound); ok {
			log.WithFields(f).Warnf("event for non existing local repo : %s, nothing to do", *repo.FullName)
			return nil
		}
		return fmt.Errorf("fetching the repo : %s by external id : %s failed : %v", *repo.FullName, repositoryExternalID, err)
	}

	log.WithFields(f).Infof("archiving repository : %s", repoModel.RepositoryName)

	if s.sendEmail {
		subject := fmt.Sprintf("EasyCLA: Github Repository Was Archived")
		body, err := emails.RenderGithubRepositoryArchivedTemplate(s.emailService, repoModel.RepositoryClaGroupID, emails.GithubRepositoryArchivedTemplateParams{
			GithubRepositoryActionTemplateParams: emails.GithubRepositoryActionTemplateParams{
				CommonEmailParams: emails.CommonEmailParams{
					RecipientName: "CLA Manager",
				},
				RepositoryName: repoModel.RepositoryName,
			},
		})

		if err != nil {
			log.WithFields(f).Warnf("rendering email template failed : %v", err)
			return nil
		}

		if err := s.emailService.NotifyClaManagersForClaGroupID(ctx, repoModel.RepositoryClaGroupID, subject, body); err != nil {
			log.WithFields(f).Warnf("notifying cla managers via email failed : %v", err)
		}

	}

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
