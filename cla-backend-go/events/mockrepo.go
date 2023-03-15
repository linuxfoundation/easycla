// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"time"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	eventOps "github.com/communitybridge/easycla/cla-backend-go/gen/v1/restapi/operations/events"
	"github.com/go-openapi/strfmt"
)

// mockRepository data model
type mockRepository struct{}

func (repo *mockRepository) AddDataToEvent(eventID, foundationSFID, projectSFID, projectSFName, companySFID, projectID string) error {
	panic("implement me")
}

func (repo *mockRepository) GetCompanyFoundationEvents(companySFID, companyID, foundationSFID string, nextKey *string, paramPageSize *int64, searchterm *string, all bool) (*models.EventList, error) {
	panic("implement me")
}

func (repo *mockRepository) GetCompanyClaGroupEvents(claGroupIDs string, companySFID string, nextKey *string, paramPageSize *int64, searchTerm *string, all bool) (*models.EventList, error) {
	panic("implement me")
}

func (repo *mockRepository) GetCompanyEvents(companyID, eventType string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error) {
	panic("implement me")
}

func (repo *mockRepository) GetFoundationEvents(foundationSFID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error) {
	panic("implement me")
}

func (repo *mockRepository) GetClaGroupEvents(claGroupID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error) {
	panic("implement me")
}

func (repo *mockRepository) GetClaGroupIDForProject(ctx context.Context, projectSFID string) (*projects_cla_groups.ProjectClaGroup, error) {
	return nil, nil
}

func (repo *mockRepository) LogEvent(args *LogEventArgs) {
	repo.LogEventWithContext(utils.NewContext(), args)
}

func (repo *mockRepository) LogEventWithContext(ctx context.Context, args *LogEventArgs) {
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

		//EventData:    eventData,
		//EventSummary: eventSummary,

		//ContainsPII: containsPII,
	}

	err := repo.CreateEvent(&event)
	if err != nil {
		log.WithError(err).Warn("unable to create event")
	}
}

var events []*models.Event

// NewMockRepository creates a new instance of the mock event repository
func NewMockRepository() *mockRepository { // nolint
	return &mockRepository{}
}

func (repo *mockRepository) CreateEvent(event *models.Event) error {
	// Add to our in-memory list
	events = append(events, event)
	return nil
}

func (repo *mockRepository) SearchEvents(params *eventOps.SearchEventsParams, pageSize int64) (*models.EventList, error) {
	// Empty response model
	eventList := &models.EventList{
		Events:  []*models.Event{},
		NextKey: "",
	}

	if params.ProjectID != nil {
		for _, event := range events {
			if event.EventProjectID == *params.ProjectID && !eventInList(eventList.Events, event) {
				eventList.Events = append(eventList.Events, event)
			}
		}
	}

	if params.CompanyID != nil {
		for _, event := range events {
			if event.EventCompanyID == *params.CompanyID && !eventInList(eventList.Events, event) {
				eventList.Events = append(eventList.Events, event)
			}
		}
	}

	return eventList, nil
}

func eventInList(eventList []*models.Event, event *models.Event) bool {
	var retVal = false
	for _, theEvent := range eventList {
		if theEvent == event {
			retVal = true
			break
		}
	}

	return retVal
}

func (repo *mockRepository) GetCLAGroupByID(ctx context.Context, projectID string, loadACLDetails bool) (*models.ClaGroup, error) {
	return &models.ClaGroup{
		DateCreated:             "",
		DateModified:            "",
		ProjectACL:              nil,
		ProjectCCLAEnabled:      false,
		ProjectCCLARequiresICLA: false,
		ProjectExternalID:       "",
		ProjectICLAEnabled:      false,
		ProjectID:               "",
		ProjectName:             "",
		Version:                 "",
	}, nil

}

func (repo *mockRepository) GetCompany(ctx context.Context, companyID string) (*models.Company, error) {
	return &models.Company{
		CompanyACL:  []string{"foo", "bar"},
		CompanyID:   companyID,
		CompanyName: "Mock Company Name",
		Created:     strfmt.DateTime(time.Now().UTC()),
		Updated:     strfmt.DateTime(time.Now().UTC()),
	}, nil
}

func (repo *mockRepository) GetUserByUserName(userName string, fullMatch bool) (*models.User, error) {
	return &models.User{
		Admin:          true,
		CompanyID:      "mock_company_id",
		DateCreated:    "",
		DateModified:   "",
		Emails:         []string{"foo@gmail.com"},
		GithubID:       "",
		GithubUsername: "bar",
		LfEmail:        "foo@gmail.com",
		LfUsername:     "foo",
		Note:           "note",
		UserExternalID: "external",
		UserID:         "mock_user_id",
		Username:       "username",
		Version:        "v1",
	}, nil
}

func (repo *mockRepository) GetUser(userID string) (*models.User, error) {
	return &models.User{
		Admin:          true,
		CompanyID:      "mock_company_id",
		DateCreated:    "",
		DateModified:   "",
		Emails:         []string{"foo@gmail.com"},
		GithubID:       "",
		GithubUsername: "bar",
		LfEmail:        "foo@gmail.com",
		LfUsername:     "foo",
		Note:           "note",
		UserExternalID: "external",
		UserID:         "mock_user_id",
		Username:       "username",
		Version:        "v1",
	}, nil
}

func (repo *mockRepository) GetRecentEvents(pageSize int64) (*models.EventList, error) {
	return &models.EventList{}, nil
}

func (repo *mockRepository) GetRecentEventsForCompanyProject(companyID, projectID string, pageSize int64) (*models.EventList, error) {
	return &models.EventList{}, nil
}
