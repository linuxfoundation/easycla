// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-openapi/runtime"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	claManagerOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/cla_manager"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClaManagerRequestService struct {
	IService
	createCalls int
	created     *CLAManagerRequest
}

func (f *fakeClaManagerRequestService) GetRequestsByUserID(_, _, _ string) (*models.ClaManagerRequestList, error) {
	return nil, nil
}

func (f *fakeClaManagerRequestService) CreateRequest(reqModel *CLAManagerRequest) (*models.ClaManagerRequest, error) {
	f.createCalls++
	f.created = reqModel
	return &models.ClaManagerRequest{RequestID: "req-1", Status: "pending"}, nil
}

type fakeContextEventsService struct {
	events.Service
	logCalls int
}

func (f *fakeContextEventsService) LogEventWithContext(_ context.Context, _ *events.LogEventArgs) {
	f.logCalls++
}

// POST /v3/company/{companyID}/project/{projectID}/cla-manager/requests must only file a request in the
// caller's own name (lfx-self-serve-ops#791)
func TestCreateCLAManagerRequestRejectsForeignLFID(t *testing.T) {
	testCases := []struct {
		name           string
		bodyLFID       string
		expectedStatus int
		expectedCalls  int
	}{
		{name: "own LFID", bodyLFID: "requester", expectedStatus: http.StatusOK, expectedCalls: 1},
		{name: "another user's LFID", bodyLFID: "victim", expectedStatus: http.StatusForbidden},
		{name: "empty LFID", bodyLFID: "", expectedStatus: http.StatusForbidden},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			requestService := &fakeClaManagerRequestService{}
			eventsService := &fakeContextEventsService{}
			api := operations.NewClaAPI(nil)
			Configure(api, requestService,
				&fakeCompanyService{company: &models.Company{CompanyID: "company-1", CompanyName: "Acme"}},
				&fakeProjectService{claGroup: &models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project"}},
				&fakeUsersService{user: &models.User{UserID: "user-1", LfUsername: "requester"}},
				&fakeSignatureService{signature: &models.Signature{SignatureID: "sig-1"}},
				eventsService, nil)
			require.NotNil(t, api.ClaManagerCreateCLAManagerRequestHandler)

			reqID := "req-791"
			recorder := httptest.NewRecorder()
			api.ClaManagerCreateCLAManagerRequestHandler.Handle(claManagerOps.CreateCLAManagerRequestParams{
				HTTPRequest: httptest.NewRequest(http.MethodPost, "/v3/company/company-1/project/cla-group-1/cla-manager/requests", nil),
				XREQUESTID:  &reqID,
				CompanyID:   "company-1",
				ProjectID:   "cla-group-1",
				Body:        models.ClaManagerUser{UserLFID: tc.bodyLFID, UserName: "Spoofed Name", UserEmail: "spoofed@example.com"},
			}, &user.CLAUser{UserID: "user-1", LFUsername: "requester", LFEmail: "requester@example.com", Name: "Requester"}).WriteResponse(recorder, runtime.JSONProducer())

			assert.Equal(t, tc.expectedStatus, recorder.Code, recorder.Body.String())
			assert.Equal(t, "req-791", recorder.Header().Get("X-Request-Id"))
			assert.Equal(t, tc.expectedCalls, requestService.createCalls, "a request must only be created for the caller's own LFID")
			assert.Equal(t, tc.expectedCalls, eventsService.logCalls)
			if tc.expectedStatus == http.StatusOK {
				if assert.NotNil(t, requestService.created) {
					assert.Equal(t, "requester", requestService.created.UserID)
					assert.Equal(t, "pending", requestService.created.Status)
				}
				return
			}
			var payload map[string]interface{}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			assert.Equal(t, "403", payload["Code"])
			assert.Contains(t, payload["Message"], "not authorized to create a CLA Manager request")
		})
	}
}
