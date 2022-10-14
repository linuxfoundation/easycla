// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/communitybridge/easycla/cla-backend-go/project/repository"
	service2 "github.com/communitybridge/easycla/cla-backend-go/project/service"

	v2Repositories "github.com/communitybridge/easycla/cla-backend-go/v2/repositories"

	gitlab_api "github.com/communitybridge/easycla/cla-backend-go/gitlab_api"
	"github.com/communitybridge/easycla/cla-backend-go/v2/gitlab_organizations"

	"github.com/communitybridge/easycla/cla-backend-go/gerrits"

	"github.com/communitybridge/easycla/cla-backend-go/repositories"

	"github.com/communitybridge/easycla/cla-backend-go/github_organizations"

	"github.com/communitybridge/easycla/cla-backend-go/approval_list"
	"github.com/communitybridge/easycla/cla-backend-go/cla_manager"

	claevent "github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	v2Company "github.com/communitybridge/easycla/cla-backend-go/v2/company"

	"github.com/communitybridge/easycla/cla-backend-go/signatures"

	"github.com/sirupsen/logrus"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// constants
const (
	Insert = "INSERT"
	Modify = "MODIFY"
	Remove = "REMOVE"
)

// EventHandlerFunc is type for dynamoDB event handler function
type EventHandlerFunc func(event events.DynamoDBEventRecord) error

type service struct {
	// key : tablename:action
	functions                map[string][]EventHandlerFunc
	signatureRepo            signatures.SignatureRepository
	companyRepo              company.IRepository
	companyService           v2Company.Service
	projectsClaGroupRepo     projects_cla_groups.Repository
	eventsRepo               claevent.Repository
	gitLabOrgRepo            gitlab_organizations.RepositoryInterface
	v2Repository             v2Repositories.RepositoryInterface
	projectRepo              repository.ProjectRepository
	projectService           service2.Service
	githubOrgService         github_organizations.ServiceInterface
	repositoryService        repositories.Service
	gerritService            gerrits.Service
	autoEnableService        *autoEnableServiceProvider
	claManagerRequestsRepo   cla_manager.IRepository
	approvalListRequestsRepo approval_list.IRepository
	gitLabApp                *gitlab_api.App
}

// Service implements DynamoDB stream event handler service
type Service interface {
	ProcessEvents(event events.DynamoDBEvent)
}

// NewService creates DynamoDB stream event handler service
func NewService(stage string,
	signatureRepo signatures.SignatureRepository,
	companyRepo company.IRepository,
	companyService v2Company.Service,
	pcgRepo projects_cla_groups.Repository,
	eventsRepo claevent.Repository,
	projectRepo repository.ProjectRepository,
	gitLabOrgRepo gitlab_organizations.RepositoryInterface,
	v2Repository v2Repositories.RepositoryInterface,
	projService service2.Service,
	githubOrgService github_organizations.ServiceInterface,
	repositoryService repositories.Service,
	gerritService gerrits.Service,
	claManagerRequestsRepo cla_manager.IRepository,
	approvalListRequestsRepo approval_list.IRepository,
	gitLabApp *gitlab_api.App) Service {

	signaturesTable := fmt.Sprintf("cla-%s-signatures", stage)
	eventsTable := fmt.Sprintf("cla-%s-events", stage)
	projectsCLAGroupsTable := fmt.Sprintf("cla-%s-projects-cla-groups", stage)
	githubOrgTableName := fmt.Sprintf("cla-%s-github-orgs", stage)
	repositoryTableName := fmt.Sprintf("cla-%s-repositories", stage)
	gitlabOrgTableName := fmt.Sprintf("cla-%s-gitlab-orgs", stage)
	// gerritTableName := fmt.Sprintf("cla-%s-gerrit-instances", stage)
	claGroupsTable := fmt.Sprintf("cla-%s-projects", stage)

	s := &service{
		functions:                make(map[string][]EventHandlerFunc),
		signatureRepo:            signatureRepo,
		companyRepo:              companyRepo,
		companyService:           companyService,
		projectsClaGroupRepo:     pcgRepo,
		eventsRepo:               eventsRepo,
		projectRepo:              projectRepo,
		gitLabOrgRepo:            gitLabOrgRepo,
		v2Repository:             v2Repository,
		projectService:           projService,
		githubOrgService:         githubOrgService,
		repositoryService:        repositoryService,
		gerritService:            gerritService,
		autoEnableService:        &autoEnableServiceProvider{repositoryService: repositoryService},
		claManagerRequestsRepo:   claManagerRequestsRepo,
		approvalListRequestsRepo: approvalListRequestsRepo,
		gitLabApp:                gitLabApp,
	}

	s.registerCallback(signaturesTable, Modify, s.SignatureSignedEvent)
	s.registerCallback(signaturesTable, Modify, s.SignatureAssignContributorEvent)
	s.registerCallback(signaturesTable, Modify, s.SignatureAddSigTypeSignedApprovedID)
	s.registerCallback(signaturesTable, Insert, s.SignatureAddSigTypeSignedApprovedID)
	s.registerCallback(signaturesTable, Insert, s.SignatureAddUsersDetails)
	// Add or Remove any CLA Permissions
	s.registerCallback(signaturesTable, Modify, s.UpdateCLAPermissions)

	s.registerCallback(eventsTable, Insert, s.EventAddedEvent)

	// Enable or Disable the CLA Service Enabled/Disabled flag/attribute in the platform Project Service
	// These are called by the API via the service layer - includes the user who did it
	//s.registerCallback(projectsCLAGroupsTable, Insert, s.ProjectServiceEnableCLAServiceHandler)
	//s.registerCallback(projectsCLAGroupsTable, Remove, s.ProjectServiceDisableCLAServiceHandler)
	s.registerCallback(projectsCLAGroupsTable, Remove, s.ProjectUnenrolledDisableRepositoryHandler)

	// Add or Remove any CLA Permissions for the specified project
	s.registerCallback(projectsCLAGroupsTable, Insert, s.AddCLAPermissions)
	s.registerCallback(projectsCLAGroupsTable, Remove, s.RemoveCLAPermissions)

	// GitHub organization table modified event
	s.registerCallback(githubOrgTableName, Insert, s.GitHubOrgAddedEvent)
	s.registerCallback(githubOrgTableName, Modify, s.GitHubOrgUpdatedEvent)
	s.registerCallback(githubOrgTableName, Remove, s.GitHubOrgDeletedEvent)

	s.registerCallback(repositoryTableName, Insert, s.GithubRepoModifyAddEvent)
	s.registerCallback(repositoryTableName, Modify, s.GithubRepoModifyAddEvent)
	s.registerCallback(repositoryTableName, Remove, s.GithubRepoModifyAddEvent)

	s.registerCallback(repositoryTableName, Insert, s.GitLabRepoAddedWebhookEventHandler)
	s.registerCallback(repositoryTableName, Modify, s.GitlabRepoModifiedWebhookEventHandler)
	s.registerCallback(repositoryTableName, Remove, s.GitLabRepoRemovedWebhookEventHandler)

	// gitlab org updates handled like branch protection and etc.
	s.registerCallback(gitlabOrgTableName, Modify, s.GitLabOrgUpdatedEvent)

	// Check and enable/disable the branch protection when a project
	s.registerCallback(repositoryTableName, Insert, s.EnableBranchProtectionServiceHandler)
	s.registerCallback(repositoryTableName, Remove, s.DisableBranchProtectionServiceHandler)

	s.registerCallback(claGroupsTable, Modify, s.ProcessCLAGroupUpdateEvents)

	return s
}

func (s *service) registerCallback(tableName, eventName string, callbackFunction EventHandlerFunc) {
	key := fmt.Sprintf("%s:%s", tableName, eventName)
	funcArr := s.functions[key]
	funcArr = append(funcArr, callbackFunction)
	s.functions[key] = funcArr
}

func (s *service) ProcessEvents(dynamoDBEvents events.DynamoDBEvent) {
	for _, event := range dynamoDBEvents.Records {
		tableName := strings.Split(event.EventSourceArn, "/")[1]
		fields := logrus.Fields{
			"functionName": "dynamo_events.ProcessEvents",
			"table_name":   tableName,
			"eventID":      event.EventID,
			"eventName":    event.EventName,
			"eventSource":  event.EventSource,
			// Dumping the event is super verbose
			// "event":      event,
		}

		log.WithFields(fields).Debug("processing event record")
		key := fmt.Sprintf("%s:%s", tableName, event.EventName)

		// If we have any functions registered
		if len(s.functions[key]) > 0 {

			// Setup a wait group for the go routine
			var wg sync.WaitGroup
			wg.Add(len(s.functions[key]))

			// For each function handler...
			for _, eventHandlerFunction := range s.functions[key] {
				go func(f EventHandlerFunc, e events.DynamoDBEventRecord) {
					defer wg.Done()

					fnType := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
					log.WithFields(fields).
						WithField("key", key).
						WithField("functionType", fnType).
						Debug("invoking handler")

					err := f(event)
					if err != nil {
						log.WithFields(fields).
							WithField("key", key).
							WithField("functionType", fnType).
							WithError(err).
							WithField("event", e).
							Error("unable to process event")
					}

					log.WithFields(fields).
						WithField("key", key).
						WithField("functionType", fnType).
						Debug("done with handler")
				}(eventHandlerFunction, event)
			}

			// Wait until the registered handlers/functions have completed for this event type...
			log.WithFields(fields).Debugf("waiting for %d event handler functions to complete...", len(s.functions[key]))
			wg.Wait()
			log.WithFields(fields).Debugf("%d event handler functions completed", len(s.functions[key]))
		}
	}
}

// UnmarshalStreamImage converts events.DynamoDBAttributeValue to struct
func unmarshalStreamImage(attribute map[string]events.DynamoDBAttributeValue, out interface{}) error {
	dbAttrMap := make(map[string]*dynamodb.AttributeValue)
	for k, v := range attribute {
		var dbAttr dynamodb.AttributeValue
		bytes, marshalErr := v.MarshalJSON()
		if marshalErr != nil {
			return marshalErr
		}
		err := json.Unmarshal(bytes, &dbAttr)
		if err != nil {
			return err
		}
		dbAttrMap[k] = &dbAttr
	}
	return dynamodbattribute.UnmarshalMap(dbAttrMap, out)
}
