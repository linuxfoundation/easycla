// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	v1Company "github.com/linuxfoundation/easycla/cla-backend-go/company"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/cla_manager"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	opList    = "list"
	opGet     = "get"
	opApprove = "approve"
	opDeny    = "deny"
)

type fakeRequestsService struct {
	Service
	list       *models.ClaManagerRequestList
	listErr    error
	request    *models.ClaManagerRequest
	requestErr error
	calls      int
}

func (f *fakeRequestsService) GetCLAManagerRequests(_ context.Context, companyID, claGroupID string) (*models.ClaManagerRequestList, error) {
	f.calls++
	if f.list != nil || f.listErr != nil {
		return f.list, f.listErr
	}
	return v2ClaManagerRequestList(nil), nil
}

func (f *fakeRequestsService) GetCLAManagerRequest(_ context.Context, companyID, claGroupID, requestID string) (*models.ClaManagerRequest, error) {
	f.calls++
	return f.request, f.requestErr
}

func (f *fakeRequestsService) ApproveCLAManagerRequest(_ context.Context, _ *auth.User, _ *v1Models.Company, claGroupID, requestID string) (*models.ClaManagerRequest, error) {
	f.calls++
	return f.request, f.requestErr
}

func (f *fakeRequestsService) DenyCLAManagerRequest(_ context.Context, _ *auth.User, _ *v1Models.Company, claGroupID, requestID string) (*models.ClaManagerRequest, error) {
	f.calls++
	return f.request, f.requestErr
}

type fakeV1CompanyService struct {
	v1Company.IService
	company *v1Models.Company
	err     error
}

func (f *fakeV1CompanyService) GetCompany(_ context.Context, companyID string) (*v1Models.Company, error) {
	return f.company, f.err
}

type fakeProjectClaGroupRepo struct {
	projects_cla_groups.Repository
	cginfo *projects_cla_groups.ProjectClaGroup
	err    error
}

func (f *fakeProjectClaGroupRepo) GetClaGroupIDForProject(_ context.Context, projectSFID string) (*projects_cla_groups.ProjectClaGroup, error) {
	return f.cginfo, f.err
}

func callRequestOp(t *testing.T, api *operations.EasyclaAPI, op string, authUser *auth.User) (int, string) {
	t.Helper()
	username, email, reqID := "tester", "tester@example.com", "req-id-1"
	httpRequest := httptest.NewRequest(http.MethodGet, "/v4/company/company-1/project/proj-sfid/cla-manager/requests", nil)

	recorder := httptest.NewRecorder()
	switch op {
	case opList:
		require.NotNil(t, api.ClaManagerGetCLAManagerRequestsHandler)
		api.ClaManagerGetCLAManagerRequestsHandler.Handle(cla_manager.GetCLAManagerRequestsParams{
			HTTPRequest: httpRequest, XUSERNAME: &username, XEMAIL: &email, XREQUESTID: &reqID,
			CompanyID: "company-1", ProjectSFID: "proj-sfid",
		}, authUser).WriteResponse(recorder, runtime.JSONProducer())
	case opGet:
		require.NotNil(t, api.ClaManagerGetCLAManagerRequestHandler)
		api.ClaManagerGetCLAManagerRequestHandler.Handle(cla_manager.GetCLAManagerRequestParams{
			HTTPRequest: httpRequest, XUSERNAME: &username, XEMAIL: &email, XREQUESTID: &reqID,
			CompanyID: "company-1", ProjectSFID: "proj-sfid", RequestID: "req-1",
		}, authUser).WriteResponse(recorder, runtime.JSONProducer())
	case opApprove:
		require.NotNil(t, api.ClaManagerApproveCLAManagerRequestHandler)
		api.ClaManagerApproveCLAManagerRequestHandler.Handle(cla_manager.ApproveCLAManagerRequestParams{
			HTTPRequest: httpRequest, XUSERNAME: &username, XEMAIL: &email, XREQUESTID: &reqID,
			CompanyID: "company-1", ProjectSFID: "proj-sfid", RequestID: "req-1",
		}, authUser).WriteResponse(recorder, runtime.JSONProducer())
	case opDeny:
		require.NotNil(t, api.ClaManagerDenyCLAManagerRequestHandler)
		api.ClaManagerDenyCLAManagerRequestHandler.Handle(cla_manager.DenyCLAManagerRequestParams{
			HTTPRequest: httpRequest, XUSERNAME: &username, XEMAIL: &email, XREQUESTID: &reqID,
			CompanyID: "company-1", ProjectSFID: "proj-sfid", RequestID: "req-1",
		}, authUser).WriteResponse(recorder, runtime.JSONProducer())
	default:
		t.Fatalf("unknown op %s", op)
	}
	return recorder.Code, recorder.Body.String()
}

func TestClaManagerRequestHandlers(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	managerUser := &auth.User{UserName: "manager-user", Email: "manager@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
	staffAdmin := &auth.User{UserName: "admin-user", Email: "admin@example.com", ACL: auth.ACL{Admin: true, Allowed: true}}
	unrelatedUser := &auth.User{UserName: "other-user", Email: "other@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|other-comp-sfid"}}}}

	company := &v1Models.Company{CompanyID: "company-1", CompanyName: "Acme", CompanyExternalID: "comp-sfid"}
	cginfo := &projects_cla_groups.ProjectClaGroup{ClaGroupID: "cla-group-1"}

	testCases := []struct {
		name           string
		authUser       *auth.User
		companyErr     error
		cgErr          error
		serviceErr     error
		expectedStatus int
		expectedCalls  int
	}{
		{name: "cla manager with project organization scope", authUser: managerUser, expectedStatus: http.StatusOK, expectedCalls: 1},
		{name: "staff admin is rejected because admin scope is disallowed", authUser: staffAdmin, expectedStatus: http.StatusForbidden},
		{name: "unrelated company scope is rejected", authUser: unrelatedUser, expectedStatus: http.StatusForbidden},
		{name: "company lookup failure", authUser: managerUser, companyErr: errors.New("company not found"), expectedStatus: http.StatusBadRequest},
		{name: "no cla group for project", authUser: managerUser, cgErr: projects_cla_groups.ErrProjectNotAssociatedWithClaGroup, expectedStatus: http.StatusBadRequest},
		{name: "missing request maps to 404", authUser: managerUser, serviceErr: errRequestNotFound, expectedStatus: http.StatusNotFound, expectedCalls: 1},
		{name: "other service failure maps to 400", authUser: managerUser, serviceErr: errors.New("dynamo down"), expectedStatus: http.StatusBadRequest, expectedCalls: 1},
	}

	for _, op := range []string{opList, opGet, opApprove, opDeny} {
		for _, tc := range testCases {
			if op == opList && tc.serviceErr == errRequestNotFound {
				continue
			}
			t.Run(op+" "+tc.name, func(t *testing.T) {
				api := operations.NewEasyclaAPI(nil)
				service := &fakeRequestsService{listErr: tc.serviceErr, requestErr: tc.serviceErr}
				if tc.serviceErr == nil {
					service.request = &models.ClaManagerRequest{RequestID: "req-1", CompanyID: "company-1", ProjectID: "cla-group-1", UserID: "user-9", Status: "pending"}
				}
				companyService := &fakeV1CompanyService{company: company, err: tc.companyErr}
				if tc.companyErr != nil {
					companyService.company = nil
				}
				pcgRepo := &fakeProjectClaGroupRepo{cginfo: cginfo, err: tc.cgErr}
				Configure(api, service, companyService, "", "", pcgRepo, nil)

				status, body := callRequestOp(t, api, op, tc.authUser)

				assert.Equal(t, tc.expectedStatus, status)
				assert.Equal(t, tc.expectedCalls, service.calls)
				if status == http.StatusOK {
					if op == opList {
						assert.Contains(t, body, `"requests":[]`, "empty list must serialize as [] not null")
					} else {
						assert.Contains(t, body, `"requestID":"req-1"`)
						assert.Contains(t, body, `"userID":"user-9"`)
					}
				}
				if status == http.StatusForbidden {
					assert.True(t, strings.Contains(body, "does not have access"), body)
				}
			})
		}
	}
}
