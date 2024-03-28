// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"
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
var eventFound int
var eventNotFound int
var recordExists int

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
	cclaSignatures, err := signatureRepo.GetCCLASignatures(context.Background(), &signed, &approved)
	if err != nil {
		log.Fatalf("Error fetching ccla signatures : %v", err)
	}
	log.Info("Fetched ccla signatures")
	eventFound = 0
	eventNotFound = 0
	recordExists = 0

	var wg sync.WaitGroup

	for _, cclaSignature := range cclaSignatures {
		wg.Add(1)
		go func(signature *signatures.ItemSignature) {
			defer wg.Done()
			err := updateApprovalsTable(signature)
			if err != nil {
				log.WithFields(f).Warnf("Error updating approvals table for signature : %s, error: %v", signature.SignatureID, err)
			}
		}(cclaSignature)
	}
	wg.Wait()
	log.WithFields(f).Debugf("Events found : %d, Events not found : %d , existing Record count: %d", eventFound, eventNotFound, recordExists)
}

func updateApprovalsTable(signature *signatures.ItemSignature) error {
	f := logrus.Fields{
		"functionName": "updateApprovalsTable",
		"signatureID":  signature.SignatureID,
	}
	log.WithFields(f).Debugf("updating approvals table for signature : %s", signature.SignatureID)
	var wg sync.WaitGroup
	var errMutex sync.Mutex
	var err error

	update := func(approvalList []string, listType string) {
		defer wg.Done()
		for _, item := range approvalList {
			searchTerm := fmt.Sprintf("%s was added to the approval list", item)
			pageSize := int64(1000)
			eventType := events.ClaApprovalListUpdated

			// check if approval item already exists
			approvalItems, searchErr := approvalRepo.SearchApprovalList(listType, item, signature.SignatureProjectID, "", signature.SignatureID)
			if err != nil {
				errMutex.Lock()
				err = searchErr
				errMutex.Unlock()
				log.WithFields(f).Warnf("Error searching approval list for item : %s, error: %v", item, err)
				return
			}

			if len(approvalItems) > 0 {
				log.WithFields(f).Debugf("Approval item already exists for : %s, %s", listType, item)
				recordExists++
				return
			}

			log.WithFields(f).Debugf("searching for events with search term : %s, projectID: %s, eventType: %s ", searchTerm, signature.SignatureProjectID, eventType)

			events, eventErr := eventsRepo.GetCCLAEvents(signature.SignatureProjectID, signature.SignatureReferenceID, searchTerm, eventType, pageSize)

			if eventErr != nil {
				errMutex.Lock()
				err = eventErr
				errMutex.Unlock()
				return
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
			}

			log.WithFields(f).Debugf("Adding approval item : %+v", approvalItem)

			if len(events) > 0 {
				event := getLatestEvent(events)
				approvalItem.DateAdded = event.EventTime
				log.WithFields(f).Debugf("found event with id: %s , approval: %+v ", event.EventID, approvalItem)
				eventFound++
			} else {
				log.WithFields(f).Debugf("no events found for %s: %s", listType, item)
				approvalItem.DateAdded = signature.DateModified
				eventNotFound++
			}

			log.WithFields(f).Debugf("adding approval item : %+v", approvalItem)
			approvalErr = approvalRepo.AddApprovalList(approvalItem)
			if err != nil {
				errMutex.Lock()
				err = approvalErr
				errMutex.Unlock()
				log.WithFields(f).Warnf("Error adding approval item : %v", err)
				return
			}
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
