// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	v1Company "github.com/linuxfoundation/easycla/cla-backend-go/company"
	v1Events "github.com/linuxfoundation/easycla/cla-backend-go/events"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeEventsService struct {
	v1Events.Service
	claGroupIDs  []string
	companySFIDs []string
}

func (f *fakeEventsService) GetCompanyClaGroupEvents(claGroupID string, companySFID string, _ *string, _ *int64, _ *string, _ bool) (*v1Models.EventList, error) {
	f.claGroupIDs = append(f.claGroupIDs, claGroupID)
	f.companySFIDs = append(f.companySFIDs, companySFID)
	return &v1Models.EventList{Events: []*v1Models.Event{{EventID: "evt-1", EventType: "test", EventCompanyID: "company-1", EventCompanySFID: companySFID}}}, nil
}

type fakeV1CompanyService struct {
	v1Company.IService
	company *v1Models.Company
	err     error
}

func (f *fakeV1CompanyService) ResolveCompany(_ context.Context, _ string) (*v1Models.Company, error) {
	return f.company, f.err
}

type fakeProjectClaGroupRepo struct {
	projects_cla_groups.Repository
}

func (f *fakeProjectClaGroupRepo) GetClaGroupIDForProject(_ context.Context, _ string) (*projects_cla_groups.ProjectClaGroup, error) {
	return &projects_cla_groups.ProjectClaGroup{ClaGroupID: "cla-group-1"}, nil
}

func TestGetCompanyProjectEventsHandlerResolvesTheCompany(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")
	const (
		sfid        = "0014100000Te0G7AAJ"
		projectSFID = "a092M00001IfPlSQAV"
		companyUUID = "8f1d8a6a-9f2e-4d6b-a0c1-2f3e4d5c6b7a"
	)
	orgUser := &auth.User{UserName: "org-user", Email: "org@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.Organization, ID: sfid}}}}
	otherUser := &auth.User{UserName: "other-user", Email: "other@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.Organization, ID: "001000000000OTHERAA"}}}}
	persisted := &v1Models.Company{CompanyID: companyUUID, CompanyName: "Acme", CompanyExternalID: sfid}

	for _, tc := range []struct {
		name           string
		pathCompanyID  string
		authUser       *auth.User
		company        *v1Models.Company
		companyErr     error
		expectedStatus int
	}{
		{name: "persisted row by internal id", pathCompanyID: companyUUID, authUser: orgUser, company: persisted, expectedStatus: http.StatusOK},
		{name: "persisted row by salesforce id", pathCompanyID: sfid, authUser: orgUser, company: persisted, expectedStatus: http.StatusOK},
		{name: "organization without a row is served virtually", pathCompanyID: sfid, authUser: orgUser, company: v1Company.VirtualCompany(sfid, "Acme"), expectedStatus: http.StatusOK},
		{name: "scope of another organization is rejected", pathCompanyID: sfid, authUser: otherUser, company: v1Company.VirtualCompany(sfid, "Acme"), expectedStatus: http.StatusForbidden},
		{name: "unknown organization", pathCompanyID: sfid, authUser: orgUser, companyErr: &utils.CompanyNotFound{CompanySFID: sfid}, expectedStatus: http.StatusBadRequest},
		{name: "lookup failure", pathCompanyID: sfid, authUser: orgUser, companyErr: errors.New("organization service unavailable"), expectedStatus: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			service := &fakeEventsService{}
			Configure(api, service, &fakeV1CompanyService{company: tc.company, err: tc.companyErr}, &fakeProjectClaGroupRepo{}, nil)
			require.NotNil(t, api.EventsGetCompanyProjectEventsHandler)
			username, email, reqID := tc.authUser.UserName, tc.authUser.Email, "req-id-1"
			recorder := httptest.NewRecorder()

			api.EventsGetCompanyProjectEventsHandler.Handle(events.GetCompanyProjectEventsParams{
				HTTPRequest: httptest.NewRequest(http.MethodGet, "/v4/company/"+tc.pathCompanyID+"/project/"+projectSFID+"/events", nil),
				XUSERNAME:   &username, XEMAIL: &email, XREQUESTID: &reqID,
				CompanyID: tc.pathCompanyID, ProjectSFID: projectSFID,
			}, tc.authUser).WriteResponse(recorder, runtime.JSONProducer())

			assert.Equal(t, tc.expectedStatus, recorder.Code, recorder.Body.String())
			if tc.expectedStatus != http.StatusOK {
				assert.Empty(t, service.claGroupIDs)
				return
			}
			assert.Equal(t, []string{"cla-group-1"}, service.claGroupIDs)
			assert.Equal(t, []string{sfid}, service.companySFIDs, "events are keyed by the company SFID for rows and virtual companies alike")
			assert.Contains(t, recorder.Body.String(), `"EventID":"evt-1"`)
		})
	}
}
