// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"errors"
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
	eventOps "github.com/communitybridge/easycla/cla-backend-go/gen/restapi/operations/events"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// constants
const (
	ReturnAllEvents     = true
	LoadRepoDetails     = true
	DontLoadRepoDetails = false
)

// Service interface defines methods of event service
type Service interface {
	LogEvent(args *LogEventArgs)
	SearchEvents(params *eventOps.SearchEventsParams) (*models.EventList, error)
	GetRecentEvents(paramPageSize *int64) (*models.EventList, error)

	GetFoundationEvents(foundationSFID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error)
	GetClaGroupEvents(claGroupID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error)
	GetCompanyFoundationEvents(companySFID, companyID, foundationSFID string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error)
	GetCompanyClaGroupEvents(companySFID, companyID, claGroupID string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error)
	GetCompanyEvents(companyID, eventType string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error)
}

// CombinedRepo contains the various methods of other repositories
type CombinedRepo interface {
	GetCLAGroupByID(ctx context.Context, claGroupID string, loadRepoDetails bool) (*models.ClaGroup, error)
	GetCompany(ctx context.Context, companyID string) (*models.Company, error)
	GetUserByUserName(userName string, fullMatch bool) (*models.User, error)
	GetUser(userID string) (*models.User, error)
}

type service struct {
	repo         Repository
	combinedRepo CombinedRepo
}

// NewService creates new instance of event service
func NewService(repo Repository, combinedRepo CombinedRepo) Service {
	return &service{
		repo:         repo,
		combinedRepo: combinedRepo,
	}
}

func (s *service) CreateEvent(event models.Event) error {
	return s.repo.CreateEvent(&event)
}

// SearchEvents service definition
func (s *service) SearchEvents(params *eventOps.SearchEventsParams) (*models.EventList, error) {
	const defaultPageSize int64 = 50
	var pageSize = defaultPageSize
	if params.PageSize != nil {
		pageSize = *params.PageSize
	}
	return s.repo.SearchEvents(params, pageSize)
}

// GetRecentEvents returns event list of recent events
func (s *service) GetRecentEvents(paramPageSize *int64) (*models.EventList, error) {
	const defaultPageSize int64 = 10
	var pageSize = defaultPageSize
	if paramPageSize != nil {
		pageSize = *paramPageSize
	}
	return s.repo.GetRecentEvents(pageSize)
}

// GetFoundationEvents returns the list of foundation events
func (s *service) GetFoundationEvents(foundationSFID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error) {
	return s.repo.GetFoundationEvents(foundationSFID, nextKey, paramPageSize, all, searchTerm)
}

// GetClaGroupEvents returns the list of project events
func (s *service) GetClaGroupEvents(projectSFDC string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error) {
	return s.repo.GetClaGroupEvents(projectSFDC, nextKey, paramPageSize, all, searchTerm)
}

// GetCompanyFoundationEvents returns list of events for company and foundation
func (s *service) GetCompanyFoundationEvents(companySFID, companyID, foundationSFID string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error) {
	return s.repo.GetCompanyFoundationEvents(companySFID, companyID, foundationSFID, nextKey, paramPageSize, all)
}

// GetCompanyClaGroupEvents returns list of events for company and cla group
func (s *service) GetCompanyClaGroupEvents(companySFID, companyID, claGroupID string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error) {
	return s.repo.GetCompanyClaGroupEvents(companySFID, companyID, claGroupID, nextKey, paramPageSize, all)
}

func (s *service) GetCompanyEvents(companyID, eventType string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error) {
	return s.repo.GetCompanyEvents(companyID, eventType, nextKey, paramPageSize, all)
}

// LogEventArgs is argument to LogEvent function
// EventType, EventData are compulsory.
// One of LfUsername, UserID must be present
type LogEventArgs struct {
	EventType         string
	ProjectID         string
	ClaGroupModel     *models.ClaGroup
	CompanyID         string
	CompanyModel      *models.Company
	LfUsername        string
	UserID            string
	UserModel         *models.User
	ExternalProjectID string
	EventData         EventData
	userName          string
	projectName       string
	companyName       string
}

func (s *service) loadCompany(ctx context.Context, args *LogEventArgs) error {
	if args.CompanyModel != nil {
		args.companyName = args.CompanyModel.CompanyName
		args.CompanyID = args.CompanyModel.CompanyID
		return nil
	}
	if args.CompanyID != "" {
		companyModel, err := s.combinedRepo.GetCompany(ctx, args.CompanyID)
		if err != nil {
			return err
		}
		args.CompanyModel = companyModel
		args.companyName = companyModel.CompanyName
	}
	return nil
}

func (s *service) loadProject(ctx context.Context, args *LogEventArgs) error {
	if args.ClaGroupModel != nil {
		args.ProjectID = args.ClaGroupModel.ProjectID
		args.projectName = args.ClaGroupModel.ProjectName
		args.ExternalProjectID = args.ClaGroupModel.ProjectExternalID
		return nil
	}
	if args.ProjectID != "" {
		claGroupModel, err := s.combinedRepo.GetCLAGroupByID(ctx, args.ProjectID, DontLoadRepoDetails)
		if err != nil {
			return err
		}
		args.ClaGroupModel = claGroupModel
		args.projectName = claGroupModel.ProjectName
		args.ExternalProjectID = claGroupModel.ProjectExternalID
	}
	return nil
}

func (s *service) loadUser(args *LogEventArgs) error {
	if args.UserModel != nil {
		args.userName = args.UserModel.Username
		args.UserID = args.UserModel.UserID
		args.LfUsername = args.UserModel.LfUsername
		return nil
	}
	if args.UserID == "" && args.LfUsername == "" {
		return errors.New("require userID or LfUsername")
	}
	var userModel *models.User
	var err error
	if args.LfUsername != "" {
		userModel, err = s.combinedRepo.GetUserByUserName(args.LfUsername, true)
		if err != nil {
			return err
		}
	}
	if args.UserID != "" {
		userModel, err = s.combinedRepo.GetUser(args.UserID)
		if err != nil {
			return err
		}
	}

	if userModel != nil {
		args.UserModel = userModel
		args.userName = userModel.Username
		args.UserID = userModel.UserID
		args.LfUsername = userModel.LfUsername
	}

	return nil
}

func (s *service) loadDetails(ctx context.Context, args *LogEventArgs) error {
	err := s.loadCompany(ctx, args)
	if err != nil {
		return err
	}
	err = s.loadProject(ctx, args)
	if err != nil {
		return err
	}
	err = s.loadUser(args)
	if err != nil {
		return err
	}
	return nil
}

// LogEvent logs the event in database
func (s *service) LogEvent(args *LogEventArgs) {
	ctx := utils.NewContext()
	defer func() {
		if r := recover(); r != nil {
			log.Error("panic occurred in CreateEvent", fmt.Errorf("%v", r))
		}
	}()
	if args == nil || args.EventType == "" || args.EventData == nil || (args.UserID == "" && args.LfUsername == "") {
		log.Warnf("invalid arguments to LogEvent, missing one or more required values. args %#v", args)
		return
	}
	err := s.loadDetails(ctx, args)
	if err != nil {
		log.Error("unable to load details for event", err)
		return
	}
	eventData, containsPII := args.EventData.GetEventDetailsString(args)
	eventSummary, _ := args.EventData.GetEventSummaryString(args)
	event := models.Event{
		ContainsPII:            containsPII,
		EventCompanyID:         args.CompanyID,
		EventCompanyName:       args.companyName,
		EventData:              eventData,
		EventSummary:           eventSummary,
		EventProjectExternalID: args.ExternalProjectID,
		EventProjectID:         args.ProjectID,
		EventProjectName:       args.projectName,
		EventType:              args.EventType,
		UserID:                 args.UserID,
		UserName:               args.userName,
		LfUsername:             args.LfUsername,
	}
	err = s.repo.CreateEvent(&event)
	if err != nil {
		log.Error(fmt.Sprintf("unable to create event for args %#v", args), err)
	}
}
