// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-openapi/runtime"
	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	companyOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCompanyEventsService struct {
	events.Service
	logCalls int
}

func (f *fakeCompanyEventsService) LogEventWithContext(_ context.Context, _ *events.LogEventArgs) {
	f.logCalls++
}

func assertCompanyRejected(t *testing.T, recorder *httptest.ResponseRecorder, expectedCode int, fragment string) {
	t.Helper()
	assert.Equal(t, expectedCode, recorder.Code, recorder.Body.String())
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, strconv.Itoa(expectedCode), payload["Code"])
	assert.Contains(t, payload["Message"], fragment)
}

// POST /v3/company/{companyID}/cla/accesslist is reserved for existing company ACL members (lfx-self-serve-ops#805)
func TestAddUserToCompanyAccessListRequiresCompanyACL(t *testing.T) {
	acme := &models.Company{CompanyID: "company-1", CompanyName: "Acme", CompanyACL: []string{"manager"}}
	manager := &user.CLAUser{UserID: "manager-id", LFUsername: "manager", LFEmail: "manager@example.com"}
	outsider := &user.CLAUser{UserID: "outsider-id", LFUsername: "outsider", LFEmail: "outsider@example.com"}

	testCases := []struct {
		name          string
		company       *models.Company
		claUser       *user.CLAUser
		expectedCode  int
		expectedCalls int
		fragment      string
	}{
		{name: "acl member", company: acme, claUser: manager, expectedCode: http.StatusOK, expectedCalls: 1},
		{name: "authenticated outsider", company: acme, claUser: outsider, expectedCode: http.StatusForbidden, fragment: "is not a CLA manager of company"},
		{name: "company without acl", company: &models.Company{CompanyID: "company-1"}, claUser: manager, expectedCode: http.StatusForbidden, fragment: "is not a CLA manager of company"},
		{name: "caller without lf username", company: acme, claUser: &user.CLAUser{UserID: "github-only"}, expectedCode: http.StatusUnauthorized, fragment: "missing authenticated LF user"},
		{name: "missing principal", company: acme, claUser: nil, expectedCode: http.StatusUnauthorized, fragment: "missing authenticated LF user"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			service := mock_company.NewMockIService(ctrl)
			service.EXPECT().GetCompany(gomock.Any(), "company-1").Return(tc.company, nil).AnyTimes()
			service.EXPECT().AddUserToCompanyAccessList(gomock.Any(), "company-1", "newcomer").Return(nil).Times(tc.expectedCalls)
			eventsService := &fakeCompanyEventsService{}

			api := operations.NewClaAPI(nil)
			company.Configure(api, service, nil, false, eventsService)
			require.NotNil(t, api.CompanyAddUsertoCompanyAccessListHandler)

			reqID := "req-805"
			recorder := httptest.NewRecorder()
			api.CompanyAddUsertoCompanyAccessListHandler.Handle(companyOps.AddUsertoCompanyAccessListParams{
				HTTPRequest: httptest.NewRequest(http.MethodPost, "/v3/company/company-1/cla/accesslist", nil),
				XREQUESTID:  &reqID,
				CompanyID:   "company-1",
				User:        models.AccessListUser{UserLFID: "newcomer"},
			}, tc.claUser).WriteResponse(recorder, runtime.JSONProducer())

			assert.Equal(t, "req-805", recorder.Header().Get("X-Request-Id"))
			assert.Equal(t, tc.expectedCalls, eventsService.logCalls)
			if tc.expectedCode == http.StatusOK {
				assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				return
			}
			assertCompanyRejected(t, recorder, tc.expectedCode, tc.fragment)
		})
	}
}

// approve/reject of a company access request need an ACL member of the path company, a request that belongs to that
// company, and a caller who is not the requester (lfx-self-serve-ops#731)
func TestCompanyAccessRequestDecisionsRequireCompanyACL(t *testing.T) {
	acme := &models.Company{CompanyID: "company-1", CompanyName: "Acme", CompanyACL: []string{"manager", "self-approver"}}
	invites := []models.CompanyInviteUser{
		{InviteID: "invite-1", UserLFID: "requester", Status: "pending"},
		{InviteID: "invite-self", UserLFID: "self-approver", Status: "pending"},
	}
	manager := &user.CLAUser{UserID: "manager-id", LFUsername: "manager", LFEmail: "manager@example.com"}
	selfApprover := &user.CLAUser{UserID: "self-id", LFUsername: "self-approver", LFEmail: "self@example.com"}
	outsider := &user.CLAUser{UserID: "outsider-id", LFUsername: "outsider", LFEmail: "outsider@example.com"}

	testCases := []struct {
		name          string
		claUser       *user.CLAUser
		requestID     string
		expectedCode  int
		expectedCalls int
		fragment      string
	}{
		{name: "acl member decides a company request", claUser: manager, requestID: "invite-1", expectedCode: http.StatusOK, expectedCalls: 1},
		{name: "outsider", claUser: outsider, requestID: "invite-1", expectedCode: http.StatusForbidden, fragment: "is not a CLA manager of company"},
		{name: "request of another company", claUser: manager, requestID: "invite-of-other-company", expectedCode: http.StatusForbidden, fragment: "does not belong to company"},
		{name: "requester decides their own request", claUser: selfApprover, requestID: "invite-self", expectedCode: http.StatusForbidden, fragment: "cannot decide their own access request"},
		{name: "missing principal", claUser: nil, requestID: "invite-1", expectedCode: http.StatusUnauthorized, fragment: "missing authenticated LF user"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			service := mock_company.NewMockIService(ctrl)
			service.EXPECT().GetCompany(gomock.Any(), "company-1").Return(acme, nil).AnyTimes()
			service.EXPECT().GetCompanyInviteRequests(gomock.Any(), "company-1", nil).Return(invites, nil).AnyTimes()
			decided := &company.InviteModel{CompanyInviteID: tc.requestID, RequestedCompanyID: "company-1", UserID: "requester-id", UserName: "Requester", UserEmail: "requester@example.com"}
			service.EXPECT().ApproveCompanyAccessRequest(gomock.Any(), tc.requestID).Return(decided, nil).Times(tc.expectedCalls)
			service.EXPECT().RejectCompanyAccessRequest(gomock.Any(), tc.requestID).Return(decided, nil).Times(tc.expectedCalls)
			eventsService := &fakeCompanyEventsService{}

			api := operations.NewClaAPI(nil)
			company.Configure(api, service, nil, false, eventsService)
			require.NotNil(t, api.CompanyApproveCompanyAccessRequestHandler)
			require.NotNil(t, api.CompanyRejectCompanyAccessRequestHandler)
			reqID := "req-731"

			approve := httptest.NewRecorder()
			api.CompanyApproveCompanyAccessRequestHandler.Handle(companyOps.ApproveCompanyAccessRequestParams{
				HTTPRequest: httptest.NewRequest(http.MethodPut, "/v3/company/company-1/cla/accesslist/"+tc.requestID+"/approve", nil),
				XREQUESTID:  &reqID, CompanyID: "company-1", RequestID: tc.requestID,
			}, tc.claUser).WriteResponse(approve, runtime.JSONProducer())

			reject := httptest.NewRecorder()
			api.CompanyRejectCompanyAccessRequestHandler.Handle(companyOps.RejectCompanyAccessRequestParams{
				HTTPRequest: httptest.NewRequest(http.MethodPut, "/v3/company/company-1/cla/accesslist/"+tc.requestID+"/reject", nil),
				XREQUESTID:  &reqID, CompanyID: "company-1", RequestID: tc.requestID,
			}, tc.claUser).WriteResponse(reject, runtime.JSONProducer())

			assert.Equal(t, "req-731", approve.Header().Get("X-Request-Id"))
			assert.Equal(t, "req-731", reject.Header().Get("X-Request-Id"))
			assert.Equal(t, 2*tc.expectedCalls, eventsService.logCalls)
			if tc.expectedCode == http.StatusOK {
				assert.Equal(t, http.StatusOK, approve.Code, approve.Body.String())
				assert.Equal(t, http.StatusOK, reject.Code, reject.Body.String())
				return
			}
			assertCompanyRejected(t, approve, tc.expectedCode, tc.fragment)
			assertCompanyRejected(t, reject, tc.expectedCode, tc.fragment)
		})
	}
}
