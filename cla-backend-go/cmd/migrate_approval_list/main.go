// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	// "strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gerrits"
	"github.com/communitybridge/easycla/cla-backend-go/github_organizations"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/project/repository"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/repositories"
	"github.com/communitybridge/easycla/cla-backend-go/signatures"
	"github.com/communitybridge/easycla/cla-backend-go/users"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/communitybridge/easycla/cla-backend-go/v2/approvals"
)

var stage string
var approvalRepo approvals.IRepository
var signatureRepo signatures.SignatureRepository
var eventsRepo events.Repository
var usersRepo users.UserRepository
var eventsService events.Service
var awsSession = session.Must(session.NewSession(&aws.Config{}))
var approvalsTableName string
var companyRepo company.IRepository
var v1ProjectClaGroupRepo projects_cla_groups.Repository
var ghRepo repositories.Repository
var gerritsRepo gerrits.Repository
var ghOrgRepo github_organizations.Repository
var gerritService gerrits.Service
var eventsByType []*v1Models.Event
var toUpdateApprovalItems []approvals.ApprovalItem

type combinedRepo struct {
	users.UserRepository
	company.IRepository
	repository.ProjectRepository
	projects_cla_groups.Repository
}

func init() {
	stage = os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("stage not set")
	}

	log.Infof("STAGE set to %s\n", stage)
	approvalsTableName = fmt.Sprintf("cla-%s-approvals", stage)
	approvalRepo = approvals.NewRepository(stage, awsSession, approvalsTableName)
	eventsRepo = events.NewRepository(awsSession, stage)
	usersRepo = users.NewRepository(awsSession, stage)
	companyRepo = company.NewRepository(awsSession, stage)
	ghRepo = *repositories.NewRepository(awsSession, stage)
	gerritsRepo = gerrits.NewRepository(awsSession, stage)
	v1CLAGroupRepo := repository.NewRepository(awsSession, stage, &ghRepo, gerritsRepo, v1ProjectClaGroupRepo)
	v1ProjectClaGroupRepo = projects_cla_groups.NewRepository(awsSession, stage)
	eventsService = events.NewService(eventsRepo, combinedRepo{
		usersRepo,
		companyRepo,
		v1CLAGroupRepo,
		v1ProjectClaGroupRepo,
	})
	ghOrgRepo = github_organizations.NewRepository(awsSession, stage)
	gerritService = gerrits.NewService(gerritsRepo, nil)
	signatureRepo = signatures.NewRepository(awsSession, stage, companyRepo, usersRepo, eventsService, &ghRepo, ghOrgRepo, gerritService, approvalRepo)

	log.Info("initialized repositories\n")
}

func main() {
	f := logrus.Fields{
		"functionName": "main",
	}
	log.WithFields(f).Info("Starting migration")
	log.Info("Fetching ccla signatures")
	signed := true
	approved := true
	// signatureID := flag.String("signature-id", "ALL", "signature ID to migrate")
	delete := flag.Bool("delete", false, "delete approval items")
	signatureID := flag.String("signature-id", "ALL", "signature ID to migrate")
	flag.Parse()

	if *delete {
		log.Info("Deleting approval items")
		err := approvalRepo.BatchDeleteApprovalList()
		if err != nil {
			log.WithFields(f).WithError(err).Error("error deleting approval items")
			return
		}
		log.Info("Deleted all approval items")
		return

	} else if *signatureID != "ALL" {
		log.Infof("Migrating approval items for signature : %s", *signatureID)
		signature, err := signatureRepo.GetItemSignature(context.Background(), *signatureID)
		if err != nil {
			log.WithFields(f).WithError(err).Errorf("error fetching signature : %s", *signatureID)
			return
		}
		log.WithFields(f).Debugf("Processing signature : %+v", signature)
		err = updateApprovalsTable(signature)
		if err != nil {
			log.WithFields(f).WithError(err).Errorf("error updating approvals table for signature : %s", *signatureID)
			return
		}
		log.Infof("batch update %d approvals ", len(toUpdateApprovalItems))
		err = approvalRepo.BatchAddApprovalList(toUpdateApprovalItems)
		if err != nil {
			log.WithFields(f).WithError(err).Error("error adding approval items")
			return
		}
		return
	}

	log.Info("Fetching all ccla signatures...")
	cclaSignatures, err := signatureRepo.GetCCLASignatures(context.Background(), &signed, &approved)
	if err != nil {
		log.Fatalf("Error fetching ccla signatures : %v", err)
	}
	log.Infof("Fetched %d ccla signatures", len(cclaSignatures))

	log.WithFields(f).Debugf("Fetching events by type : %s", events.ClaApprovalListUpdated)
	eventsByType, err = eventsRepo.GetEventsByType(events.ClaApprovalListUpdated, 100)

	if err != nil {
		log.WithFields(f).WithError(err).Errorf("error fetching events by type : %s", events.ClaApprovalListUpdated)
		return
	}

	var wg sync.WaitGroup

	for _, cclaSignature := range cclaSignatures {
		wg.Add(1)
		go func(signature *signatures.ItemSignature) {
			defer wg.Done()
			log.WithFields(f).Debugf("Processing company : %s, project : %s", signature.SignatureReferenceName, signature.SignatureProjectID)
			updateErr := updateApprovalsTable(signature)
			if updateErr != nil {
				log.WithFields(f).Warnf("Error updating approvals table for signature : %s, error: %v", signature.SignatureID, updateErr)
			}
		}(cclaSignature)
	}
	wg.Wait()
	log.WithFields(f).Infof("batch update %d approvals ", len(toUpdateApprovalItems))
	err = approvalRepo.BatchAddApprovalList(toUpdateApprovalItems)
	if err != nil {
		log.WithFields(f).WithError(err).Error("error adding approval items")
		return
	}

}

func getSearchTermEvents(events []*v1Models.Event, searchTerm, companyID, claGroupID string) []*v1Models.Event {
	f := logrus.Fields{
		"functionName": "getSearchTermEvents",
		"searchTerm":   searchTerm,
		"companyID":    companyID,
	}
	log.WithFields(f).Debug("searching for events ...")
	var result []*v1Models.Event
	for _, event := range events {
		if strings.Contains(strings.ToLower(event.EventData), strings.ToLower(searchTerm)) && event.EventCompanyID == companyID && event.EventCLAGroupID == claGroupID {
			log.WithFields(f).Debugf("found event with search term : %s", searchTerm)
			result = append(result, event)
		}
	}
	return result
}

func updateApprovalsTable(signature *signatures.ItemSignature) error {
	f := logrus.Fields{
		"functionName": "updateApprovalsTable",
		"signatureID":  signature.SignatureID,
		"companyName":  signature.SignatureReferenceName,
	}
	log.WithFields(f).Debugf("updating approvals table for signature : %s", signature.SignatureID)
	var wg sync.WaitGroup
	var errMutex sync.Mutex
	var err error

	update := func(approvalList []string, listType string) {
		defer wg.Done()
		for _, item := range approvalList {
			searchIdentifier := ""
			switch listType {
			case utils.DomainApprovalCriteria:
				searchIdentifier = "email address domain"
			case utils.EmailApprovalCriteria:
				searchIdentifier = "email address"
			case utils.GithubUsernameApprovalCriteria:
				searchIdentifier = "GitHub username"
			case utils.GitlabUsernameApprovalCriteria:
				searchIdentifier = "GitLab username"
			case utils.GithubOrgApprovalCriteria:
				searchIdentifier = "GitHub organization"
			case utils.GitlabOrgApprovalCriteria:
				searchIdentifier = "GitLab group"
			default:
				searchIdentifier = ""
			}
			searchTerm := fmt.Sprintf("%s %s was added to the approval list", searchIdentifier, item)
			events := getSearchTermEvents(eventsByType, searchTerm, signature.SignatureReferenceID, signature.SignatureProjectID)
			dateAdded := signature.DateModified

			if len(events) > 0 {
				latestEvent := getLatestEvent(events)
				dateAdded = latestEvent.EventTime
			}

			approvalID, approvalErr := uuid.NewV4()
			if err != nil {
				errMutex.Lock()
				err = approvalErr
				log.WithFields(f).Warnf("Error creating new UUIDv4, error: %v", err)
				errMutex.Unlock()
				return
			}
			currentTime := time.Now().UTC().String()
			note := fmt.Sprintf("Approval item added by migration script on %s", currentTime)
			approvalItem := approvals.ApprovalItem{
				ApprovalID:          approvalID.String(),
				SignatureID:         signature.SignatureID,
				DateCreated:         currentTime,
				DateModified:        currentTime,
				ApprovalName:        item,
				ApprovalCriteria:    listType,
				CompanyID:           signature.SignatureReferenceID,
				ProjectID:           signature.SignatureProjectID,
				ApprovalCompanyName: signature.SignatureReferenceName,
				DateAdded:           dateAdded,
				Note:                note,
				Active:              true,
			}

			toUpdateApprovalItems = append(toUpdateApprovalItems, approvalItem)
		}
	}

	wg.Add(1)
	go update(signature.EmailDomainApprovalList, utils.DomainApprovalCriteria)

	wg.Add(1)
	go update(signature.EmailApprovalList, utils.EmailApprovalCriteria)

	wg.Add(1)
	go update(signature.GitHubOrgApprovalList, utils.GithubOrgApprovalCriteria)

	wg.Add(1)
	go update(signature.GitHubUsernameApprovalList, utils.GithubUsernameApprovalCriteria)

	wg.Add(1)
	go update(signature.GitlabOrgApprovalList, utils.GitlabOrgApprovalCriteria)

	wg.Add(1)
	go update(signature.GitlabUsernameApprovalList, utils.GitlabUsernameApprovalCriteria)

	wg.Wait()

	return err
}

func getLatestEvent(events []*v1Models.Event) *v1Models.Event {
	var latest *v1Models.Event
	var latestTime time.Time

	for _, item := range events {
		t, err := utils.ParseDateTime(item.EventTime)
		if err != nil {
			log.Debugf("Error parsing time: %+v ", err)
			continue
		}

		if latest == nil || t.After(latestTime) {
			latest = item
			latestTime = t
		}
	}

	return latest
}
