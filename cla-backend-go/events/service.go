// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"errors"
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	project_service "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"
	user_service "github.com/communitybridge/easycla/cla-backend-go/v2/user-service"
	userServiceModels "github.com/communitybridge/easycla/cla-backend-go/v2/user-service/models"

	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	eventOps "github.com/communitybridge/easycla/cla-backend-go/gen/v1/restapi/operations/events"
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
	LogEventWithContext(ctx context.Context, args *LogEventArgs)
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
	GetClaGroupIDForProject(ctx context.Context, projectSFID string) (*projects_cla_groups.ProjectClaGroup, error)
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
	EventType string

	UserID     string
	LfUsername string
	UserName   string
	UserModel  *models.User
	LFUser     *userServiceModels.User

	CLAGroupID    string
	CLAGroupName  string
	ClaGroupModel *models.ClaGroup

	ProjectID         string // Should just use CLA GroupID
	ProjectSFID       string
	ProjectName       string
	ParentProjectSFID string
	ParentProjectName string

	CompanyID    string
	CompanyName  string
	CompanySFID  string
	CompanyModel *models.Company

	EventData EventData
}

func (s *service) loadCompany(ctx context.Context, args *LogEventArgs) error {
	f := logrus.Fields{
		"functionName":   "v1.events.service.loadCompany",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	if args == nil {
		return errors.New("unable to load company data - args is nil")
	}

	if args.CompanyModel != nil {
		args.CompanyName = args.CompanyModel.CompanyName
		args.CompanyID = args.CompanyModel.CompanyID
		args.CompanySFID = args.CompanyModel.CompanyExternalID
		return nil
	} else if args.CompanyID != "" {
		companyModel, err := s.combinedRepo.GetCompany(ctx, args.CompanyID)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("failed to load company record ID: %s", args.CompanyID)
			return err
		}
		args.CompanyModel = companyModel
		args.CompanyName = companyModel.CompanyName
		args.CompanySFID = companyModel.CompanyExternalID
	}

	return nil
}

func (s *service) loadCLAGroup(ctx context.Context, args *LogEventArgs) error {
	f := logrus.Fields{
		"functionName":   "v1.events.service.loadCLAGroup",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	if args == nil {
		return errors.New("unable to load CLA Group data - args is nil")
	}

	// First, attempt to user the CLA Group model that was provided...
	if args.ClaGroupModel != nil {
		args.CLAGroupID = args.ClaGroupModel.ProjectID
		args.CLAGroupName = args.ClaGroupModel.ProjectName
	} else {
		// Did they set the CLA Group ID?
		var claGroupID string
		if args.CLAGroupID != "" {
			claGroupID = args.CLAGroupID
		} else if args.ProjectID != "" && utils.IsUUIDv4(args.ProjectID) { // legacy parameter
			claGroupID = args.ProjectID
		}

		// Load the CLA Group ID if set...
		if claGroupID != "" {
			claGroupModel, err := s.combinedRepo.GetCLAGroupByID(ctx, claGroupID, DontLoadRepoDetails)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("failed to load CLA Group by ID: %s", claGroupID)
				return err
			}
			args.ClaGroupModel = claGroupModel
			args.CLAGroupName = claGroupModel.ProjectName
			args.CLAGroupID = claGroupID
		} else if args.ProjectSFID != "" {
			projectCLAGroupModel, projectCLAGroupErr := s.combinedRepo.GetClaGroupIDForProject(ctx, args.ProjectSFID)
			if projectCLAGroupErr != nil || projectCLAGroupModel == nil {
				log.WithFields(f).WithError(projectCLAGroupErr).Warnf("failed to load project CLA Group mapping by SFID: %s", args.ProjectSFID)
				return nil
			}

			args.CLAGroupID = projectCLAGroupModel.ClaGroupID
			args.CLAGroupName = projectCLAGroupModel.ClaGroupName
		}
	}

	return nil
}

func (s *service) loadSFProject(ctx context.Context, args *LogEventArgs) error {
	if args == nil {
		return errors.New("unable to load SF project data - args is nil")
	}

	f := logrus.Fields{
		"functionName":   "v1.events.service.loadSFProject",
		"projectID":      args.ProjectID,
		"projectSFID":    args.ProjectSFID,
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	// if it's a legacy model (v1) we need ot set the project sfid from project id
	if args.ProjectSFID == "" && args.ProjectID != "" && utils.IsSalesForceID(args.ProjectID) {
		args.ProjectSFID = args.ProjectID
	}

	// if project sfid not there try to set it from claGroupModel (v2)
	if args.ProjectSFID == "" && args.ClaGroupModel != nil && args.ClaGroupModel.ProjectExternalID != "" {
		args.ProjectSFID = args.ClaGroupModel.ProjectExternalID
	}

	if args.ProjectSFID != "" && utils.IsSalesForceID(args.ProjectSFID) {
		// Check if project exists in platform project service
		//log.WithFields(f).Debugf("loading salesforce project by ID: %s...", args.ProjectSFID)
		project, projectErr := project_service.GetClient().GetProject(args.ProjectSFID)
		if projectErr != nil || project == nil {
			log.WithFields(f).Warnf("failed to load salesforce project by ID: %s", args.ProjectSFID)
			return nil
		}
		//log.WithFields(f).Debugf("loaded salesforce project by ID: %s", args.ProjectSFID)
		args.ProjectName = project.Name

		// Try to load and set the parent information
		if utils.IsProjectHaveParent(project) {
			//log.WithFields(f).Debugf("loading project parent by ID: %s...", utils.GetProjectParentSFID(project))
			parentProjectModel, parentProjectErr := project_service.GetClient().GetParentProjectModel(project.ID)
			if parentProjectErr != nil || parentProjectModel == nil {
				log.WithFields(f).Warnf("failed to load project parent by ID: %s", utils.GetProjectParentSFID(project))
				return nil
			}

			var parentProjectName, parentProjectID string
			if !utils.IsProjectHasRootParent(project) {
				parentProjectName = parentProjectModel.Name
				parentProjectID = parentProjectModel.ID
			} else {
				parentProjectName = project.Name
				parentProjectID = project.ID
			}
			//log.WithFields(f).Debugf("loaded project by parent ID: %s - resulting in ID: %s with name: %s",
			//	project.Foundation.ID, parentProjectID, parentProjectName)
			args.ParentProjectSFID = parentProjectID
			args.ParentProjectName = parentProjectName
		} else {
			// No parent, just use the current project as the parent
			args.ParentProjectSFID = project.ID
			args.ParentProjectName = project.Name
		}
	} else {
		log.WithFields(f).Warnf("project sfid %s was not set properly can't set parent project fields in event", args.ProjectSFID)
	}

	return nil
}

func (s *service) loadLFUser(ctx context.Context, args *LogEventArgs) error {
	f := logrus.Fields{
		"functionName":   "v1.events.service.LFUser",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	if args == nil {
		return errors.New("unable to load lf user data - args is nil")
	}

	if args.LfUsername != "" {
		lfUser, lfErr := user_service.GetClient().GetUserByUsername(args.LfUsername)
		if lfErr != nil || lfUser == nil {
			log.WithFields(f).Warnf("unable to fetch user by username: %s ", args.LfUsername)
			return nil
		}
		args.LFUser = lfUser
	}
	return nil
}

func (s *service) loadUser(ctx context.Context, args *LogEventArgs) error {
	f := logrus.Fields{
		"functionName":   "v1.events.service.loadUser",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	if args == nil {
		return errors.New("unable to load user data - args is nil")
	}

	if args.UserModel != nil {
		args.UserName = args.UserModel.Username
		args.UserID = args.UserModel.UserID
		args.LfUsername = args.UserModel.LfUsername
		log.WithFields(f).Debug("loaded user for event log by caller provided user model")
		return nil
	} else if args.UserID == "" && args.LfUsername == "" {
		log.WithFields(f).Warn("failed to load user for event log - user ID and username were not set")
		return errors.New("require userID or LfUsername")
	}

	var userModel *models.User
	var err error
	// Try loading by LF username
	if args.LfUsername != "" {
		log.WithFields(f).Debugf("loading user by LF username: %s...", args.LfUsername)
		userModel, err = s.combinedRepo.GetUserByUserName(args.LfUsername, true)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("failed to load user by username: %s", args.LfUsername)
		}
	}

	// Try loading by user ID
	if args.UserID != "" {
		log.WithFields(f).Debugf("loading user by user ID: %s...", args.UserID)
		userModel, err = s.combinedRepo.GetUser(args.UserID)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("failed to load user by ID: %s", args.UserID)
		}
	}

	// Did we finally load the user model?
	if userModel != nil {
		args.UserModel = userModel
		// Update username with LF Name value if exists ...
		if args.LFUser != nil {
			args.UserName = args.LFUser.Name
		} else {
			args.UserName = userModel.Username
		}
		args.UserID = userModel.UserID
		args.LfUsername = userModel.LfUsername
	} else {
		log.WithFields(f).Warnf("unable to set user information for event log entry")
	}

	return nil
}

// loadDetails fetches and sets additional information into the data model required to fill out the event log entry
func (s *service) loadDetails(ctx context.Context, args *LogEventArgs) error {
	f := logrus.Fields{
		"functionName":   "v1.events.service.loadDetails",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	err := s.loadCompany(ctx, args)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to load company details...")
		return err
	}

	err = s.loadSFProject(ctx, args)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to load SF project details...")
		return err
	}

	err = s.loadCLAGroup(ctx, args)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to load CLA Group details...")
		return err
	}

	err = s.loadLFUser(ctx, args)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to load LF User details...")
		return err
	}

	err = s.loadUser(ctx, args)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to load user details...")
		return err
	}

	err = s.loadLFUser(ctx, args)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to load LF user details...")
		return err
	}

	return nil
}

// LogEventWithContext logs the event in database
func (s *service) LogEventWithContext(ctx context.Context, args *LogEventArgs) {
	f := logrus.Fields{
		"functionName":   "events.service.LogEventWithContext",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	defer func() {
		if r := recover(); r != nil {
			log.WithFields(f).Errorf("panic occurred - %+v", r)
		}
	}()

	if args == nil || args.EventType == "" || args.EventData == nil || (args.UserID == "" && args.LfUsername == "") {
		log.WithFields(f).Warnf("invalid arguments to LogEvent, missing one or more required values. Need EventType, EventData or one of UserID or LfUsername. args %#v", args)
		return
	}

	err := s.loadDetails(ctx, args)
	if err != nil {
		log.WithFields(f).Error("unable to load details for event", err)
		return
	}

	eventData, containsPII := args.EventData.GetEventDetailsString(args)
	eventSummary, _ := args.EventData.GetEventSummaryString(args)
	event := models.Event{
		EventType: args.EventType,

		UserID:     args.UserID,
		UserName:   args.UserName,
		LfUsername: args.LfUsername,

		EventCLAGroupID:   args.CLAGroupID,
		EventCLAGroupName: args.CLAGroupName,

		EventCompanyID:   args.CompanyID,
		EventCompanySFID: args.CompanySFID,
		EventCompanyName: args.CompanyName,

		EventProjectID:         args.ProjectID,
		EventProjectSFID:       args.ProjectSFID,
		EventProjectName:       args.ProjectName,
		EventParentProjectSFID: args.ParentProjectSFID,
		EventParentProjectName: args.ParentProjectName,

		EventData:    eventData,
		EventSummary: eventSummary,

		ContainsPII: containsPII,
	}
	err = s.repo.CreateEvent(&event)
	if err != nil {
		log.WithFields(f).Error(fmt.Sprintf("unable to create event for args %#v", args), err)
	}
}

// LogEvent logs the event in database
func (s *service) LogEvent(args *LogEventArgs) {
	s.LogEventWithContext(utils.NewContext(), args)
}
