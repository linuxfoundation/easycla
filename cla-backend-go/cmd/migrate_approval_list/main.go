// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
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

var emailDomainSuccessCount int
var emailSuccessCount int
var githubOrgSuccessCount int
var githubUsernameSuccessCount int
var gitlabOrgSuccessCount int
var gitlabUsernameSuccessCount int

// var emailFailureCount int
// var githubOrgFailureCount int
// var githubUsernameFailureCount int
// var gitlabOrgFailureCount int
// var gitlabUsernameFailureCount int

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
	flag.Parse()

	if *delete {
		log.Info("Deleting approval items")
		err := approvalRepo.DeleteAll()
		if err != nil {
			log.WithFields(f).WithError(err).Error("error deleting approval items")
			return
		}
		log.Info("Deleted all approval items")
		return
	}
	
	log.Info("Fetching all ccla signatures...")
	cclaSignatures, err := signatureRepo.GetCCLASignatures(context.Background(), &signed, &approved)
	if err != nil {
		log.Fatalf("Error fetching ccla signatures : %v", err)
	}
	log.Infof("Fetched %d ccla signatures", len(cclaSignatures))
	eventFound = 0
	eventNotFound = 0
	recordExists = 0

	var wg sync.WaitGroup

	for _, cclaSignature := range cclaSignatures {
		wg.Add(1)
		go func(signature *signatures.ItemSignature) {
			defer wg.Done()
			log.WithFields(f).Debugf("Processing company : %s, project : %s", signature.SignatureReferenceName, signature.SignatureProjectID)
			err := updateApprovalsTable(signature)
			if err != nil {
				log.WithFields(f).Warnf("Error updating approvals table for signature : %s, error: %v", signature.SignatureID, err)
			}
		}(cclaSignature)
	}
	wg.Wait()
	log.WithFields(f).Debugf("Events found : %d, Events not found : %d , existing Record count: %d", eventFound, eventNotFound, recordExists)
	
	

}

func counter(listType string) {
	switch listType {
	case utils.DomainApprovalCriteria:
		emailDomainSuccessCount++
	case utils.EmailApprovalCriteria:
		emailSuccessCount++
	case utils.GithubOrgApprovalCriteria:
		githubOrgSuccessCount++
	case utils.GithubUsernameApprovalCriteria:
		githubUsernameSuccessCount++
	case utils.GitlabOrgApprovalCriteria:
		gitlabOrgSuccessCount++
	case utils.GitlabUsernameApprovalCriteria:
		gitlabUsernameSuccessCount++
	}
}

func approvalExists(approvalItems []approvals.ApprovalItem, listType, item, projectID, signatureID string) bool {
	if len(approvalItems) == 0 {
		return false
	}
	for _, approval := range approvalItems {
		if approval.ApprovalCriteria == listType && approval.ApprovalName == item && approval.ProjectID == projectID && approval.SignatureID == signatureID {
			return true
		}
	}

	return false
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

	approvalItems, err := approvalRepo.GetApprovalListBySignature(signature.SignatureID)
	if err != nil {
		log.WithFields(f).Warnf("Error fetching approval list items for signature : %s, error: %v", signature.SignatureID, err)
		return err
	}

	log.WithFields(f).Debugf("Fetched %d approval list items for signature: %s", len(approvalItems), signature.SignatureID)

	company, err := companyRepo.GetCompany(context.Background(), signature.SignatureReferenceID)

	if err != nil {
		log.WithFields(f).Warnf("Error fetching company : %s, error: %v", signature.SignatureReferenceID, err)
		return err
	}

	if company == nil {
		log.WithFields(f).Warnf("Company not found for : %s", signature.SignatureReferenceID)
		return fmt.Errorf("company not found for : %s", signature.SignatureReferenceID)
	}

	searchTerm := "was added to the approval list"

	// Get Company Project list
	companyEvents, err := eventsRepo.GetCompanyClaGroupEvents(signature.SignatureProjectID, company.CompanyExternalID, nil, nil, &searchTerm, true)

	if err != nil {
		log.WithFields(f).Warnf("Error fetching company events : %s, error: %v", signature.SignatureProjectID, err)
		return err
	}

	log.WithFields(f).Debugf("Fetched %d company events for project : %s and company: %s", len(companyEvents.Events), signature.SignatureProjectID, company.CompanyName)

	update := func(approvalList []string, listType string) {
		defer wg.Done()
		for _, item := range approvalList {
			searchTerm := fmt.Sprintf("%s was added to the approval list", item)
			eventType := events.ClaApprovalListUpdated

			if approvalExists(approvalItems, listType, item, signature.SignatureProjectID, signature.SignatureID) {
				log.WithFields(f).Debugf("Approval item already exists for : %s, %s", listType, item)
				recordExists++
				return
			}

			log.WithFields(f).Debugf("searching for events with search term : %s, projectID: %s, eventType: %s ", searchTerm, signature.SignatureProjectID, eventType)

			dateAdded := signature.DateModified

			// search events for the item
			for _, event := range companyEvents.Events {
				if event.EventType == eventType && event.EventCompanyID == company.CompanyID && event.EventCLAGroupID == signature.SignatureProjectID && strings.Contains(strings.ToLower(event.EventData), strings.ToLower(searchTerm)) {
					log.WithFields(f).Debugf("found event with id: %s, event : %+v ", event.EventID, event)
					dateAdded = event.EventTime
					eventFound++
					break
				}
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

			log.WithFields(f).Debugf("Adding approval item : %+v", approvalItem)
			approvalErr = approvalRepo.AddApprovalList(approvalItem)
			if err != nil {
				errMutex.Lock()
				err = approvalErr
				errMutex.Unlock()
				log.WithFields(f).Warnf("Error adding approval item : %v", err)
				return
			}

			counter(listType)
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

	log.WithFields(f).Debugf("processed for company : %s, project : %s", signature.SignatureReferenceName, signature.SignatureProjectID)
	log.WithFields(f).Debugf("Email domain success count : %d, Email success count : %d, Github org success count : %d, Github username success count : %d, Gitlab org success count : %d, Gitlab username success count : %d", emailDomainSuccessCount, emailSuccessCount, githubOrgSuccessCount, githubUsernameSuccessCount, gitlabOrgSuccessCount, gitlabUsernameSuccessCount)

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
