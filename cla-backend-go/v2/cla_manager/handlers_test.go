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
	"github.com/go-openapi/strfmt"
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
	testerUsername = "tester"
	testerEmail    = "tester@example.com"

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
	companyIDs []string
	claGroups  []string
	requestIDs []string
}

func (f *fakeRequestsService) record(companyModel *v1Models.Company, claGroupID string) {
	f.calls++
	f.companyIDs = append(f.companyIDs, companyModel.CompanyID)
	f.claGroups = append(f.claGroups, claGroupID)
}

func (f *fakeRequestsService) GetCLAManagerRequests(_ context.Context, companyModel *v1Models.Company, claGroupID string, _, _ *int64) (*models.ClaManagerRequestList, error) {
	f.record(companyModel, claGroupID)
	if f.list != nil || f.listErr != nil {
		return f.list, f.listErr
	}
	return v2ClaManagerRequestList(nil), nil
}

func (f *fakeRequestsService) GetCLAManagerRequest(_ context.Context, companyModel *v1Models.Company, claGroupID, requestID string) (*models.ClaManagerRequest, error) {
	f.record(companyModel, claGroupID)
	f.requestIDs = append(f.requestIDs, requestID)
	return f.request, f.requestErr
}

func (f *fakeRequestsService) ApproveCLAManagerRequest(_ context.Context, _ *auth.User, companyModel *v1Models.Company, claGroupID, requestID string) (*models.ClaManagerRequest, error) {
	f.record(companyModel, claGroupID)
	f.requestIDs = append(f.requestIDs, requestID)
	return f.request, f.requestErr
}

func (f *fakeRequestsService) DenyCLAManagerRequest(_ context.Context, _ *auth.User, companyModel *v1Models.Company, claGroupID, requestID string) (*models.ClaManagerRequest, error) {
	f.record(companyModel, claGroupID)
	f.requestIDs = append(f.requestIDs, requestID)
	return f.request, f.requestErr
}

func (f *fakeRequestsService) CreateCLAManagerDesignee(_ context.Context, companyID string, projectSFID string, userEmail string) (*models.ClaManagerDesignee, error) {
	f.calls++
	f.companyIDs = append(f.companyIDs, companyID)
	f.claGroups = append(f.claGroups, projectSFID)
	if f.requestErr != nil {
		return nil, f.requestErr
	}
	return &models.ClaManagerDesignee{CompanyID: companyID, ProjectSfid: projectSFID, Email: strfmt.Email(userEmail)}, nil
}

type fakeV1CompanyService struct {
	v1Company.IService
	company *v1Models.Company
	err     error
}

func (f *fakeV1CompanyService) GetCompany(_ context.Context, companyID string) (*v1Models.Company, error) {
	return f.company, f.err
}

func (f *fakeV1CompanyService) ResolveCompany(_ context.Context, companyID string) (*v1Models.Company, error) {
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
	username, email, reqID := testerUsername, testerEmail, "req-id-1"
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
		cgNil          bool
		serviceErr     error
		expectedStatus int
		expectedCalls  int
	}{
		{name: "cla manager with project organization scope", authUser: managerUser, expectedStatus: http.StatusOK, expectedCalls: 1},
		{name: "staff admin is rejected because admin scope is disallowed", authUser: staffAdmin, expectedStatus: http.StatusForbidden},
		{name: "unrelated company scope is rejected", authUser: unrelatedUser, expectedStatus: http.StatusForbidden},
		{name: "company lookup failure", authUser: managerUser, companyErr: errors.New("company not found"), expectedStatus: http.StatusBadRequest},
		{name: "no cla group for project", authUser: managerUser, cgErr: projects_cla_groups.ErrProjectNotAssociatedWithClaGroup, expectedStatus: http.StatusBadRequest},
		{name: "nil cla group mapping without error", authUser: managerUser, cgNil: true, expectedStatus: http.StatusBadRequest},
		{name: "missing request maps to 404", authUser: managerUser, serviceErr: errRequestNotFound, expectedStatus: http.StatusNotFound, expectedCalls: 1},
		{name: "other service failure maps to 500", authUser: managerUser, serviceErr: errors.New("dynamo down"), expectedStatus: http.StatusInternalServerError, expectedCalls: 1},
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
				if tc.cgNil {
					pcgRepo.cginfo = nil
				}
				Configure(api, service, companyService, "", "", pcgRepo, nil)

				status, body := callRequestOp(t, api, op, tc.authUser)

				assert.Equal(t, tc.expectedStatus, status)
				assert.Equal(t, tc.expectedCalls, service.calls)
				if status == http.StatusOK {
					assert.Equal(t, []string{"company-1"}, service.companyIDs, "the internal company ID from the fetched model must reach the service")
					assert.Equal(t, []string{"cla-group-1"}, service.claGroups, "the CLA group mapped from the projectSFID must reach the service")
					if op == opList {
						assert.Contains(t, body, `"requests":[]`, "empty list must serialize as [] not null")
					} else {
						assert.Equal(t, []string{"req-1"}, service.requestIDs)
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

func TestCreateCLAManagerDesigneeHandlerResolvesTheCompany(t *testing.T) {
	const (
		sfid        = "0014100000Te0G7AAJ"
		projectSFID = "a092M00001IfPlSQAV"
		companyUUID = "8f1d8a6a-9f2e-4d6b-a0c1-2f3e4d5c6b7a"
	)
	persisted := &v1Models.Company{CompanyID: companyUUID, CompanyName: "Acme", CompanyExternalID: sfid}

	for _, tc := range []struct {
		name           string
		pathCompanyID  string
		company        *v1Models.Company
		companyErr     error
		expectedStatus int
		expectedCompID string
	}{
		{name: "persisted row by internal id", pathCompanyID: companyUUID, company: persisted, expectedStatus: http.StatusOK, expectedCompID: companyUUID},
		{name: "persisted row by salesforce id", pathCompanyID: sfid, company: persisted, expectedStatus: http.StatusOK, expectedCompID: companyUUID},
		{name: "organization without a row is served virtually", pathCompanyID: sfid, company: v1Company.VirtualCompany(sfid, "Acme"), expectedStatus: http.StatusOK, expectedCompID: sfid},
		{name: "sanctioned company is rejected", pathCompanyID: sfid, company: &v1Models.Company{CompanyID: companyUUID, CompanyExternalID: sfid, IsSanctioned: true}, expectedStatus: http.StatusForbidden},
		{name: "unknown organization", pathCompanyID: sfid, companyErr: errors.New("company not found"), expectedStatus: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			service := &fakeRequestsService{}
			Configure(api, service, &fakeV1CompanyService{company: tc.company, err: tc.companyErr}, "", "", &fakeProjectClaGroupRepo{}, nil)
			require.NotNil(t, api.ClaManagerCreateCLAManagerDesigneeHandler)
			username, email, reqID := testerUsername, testerEmail, "req-id-2"
			recorder := httptest.NewRecorder()

			api.ClaManagerCreateCLAManagerDesigneeHandler.Handle(cla_manager.CreateCLAManagerDesigneeParams{
				HTTPRequest: httptest.NewRequest(http.MethodPost, "/v4/company/"+tc.pathCompanyID+"/project/"+projectSFID+"/cla-manager-designee", nil),
				XUSERNAME:   &username, XEMAIL: &email, XREQUESTID: &reqID,
				CompanyID: tc.pathCompanyID, ProjectSFID: projectSFID,
				Body: cla_manager.CreateCLAManagerDesigneeBody{UserEmail: "designee@example.com"},
			}, &auth.User{UserName: username, Email: email}).WriteResponse(recorder, runtime.JSONProducer())

			assert.Equal(t, tc.expectedStatus, recorder.Code, recorder.Body.String())
			if tc.expectedStatus != http.StatusOK {
				assert.Zero(t, service.calls)
				if tc.expectedStatus == http.StatusForbidden {
					assert.Contains(t, recorder.Body.String(), "company_sanctioned")
				}
				return
			}
			assert.Equal(t, []string{tc.expectedCompID}, service.companyIDs)
			assert.Equal(t, []string{projectSFID}, service.claGroups)
			assert.Contains(t, recorder.Body.String(), `"company_id":"`+tc.expectedCompID+`"`)
		})
	}
}

func TestClaManagerCreateDeleteHandlersRejectUnauthorizedUsers(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	users := []struct {
		name     string
		authUser *auth.User
	}{
		{name: "staff admin", authUser: &auth.User{UserName: "admin-user", Email: "admin@example.com", ACL: auth.ACL{Admin: true, Allowed: true}}},
		{name: "unrelated company scope", authUser: &auth.User{UserName: "other-user", Email: "other@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|other-comp-sfid"}}}}},
	}
	ops := []struct {
		op        string
		operation string
	}{
		{op: opCreateManager, operation: "CreateCLAManager"},
		{op: opDeleteManager, operation: "DeleteCLAManager"},
	}

	for _, o := range ops {
		for _, u := range users {
			t.Run(o.op+" "+u.name, func(t *testing.T) {
				api := operations.NewEasyclaAPI(nil)
				service := &fakeWriteOpsService{}
				companyService := &fakeV1CompanyService{company: &v1Models.Company{CompanyID: "company-1", CompanyName: "Acme", CompanyExternalID: "comp-sfid"}}
				pcgRepo := &fakePCGRepoWithProjects{fakeProjectClaGroupRepo{cginfo: &projects_cla_groups.ProjectClaGroup{ClaGroupID: "cla-group-1"}}}
				Configure(api, service, companyService, "", "", pcgRepo, &fakeEasyCLAUserRepo{})

				status, body, _ := callWriteOp(t, api, o.op, u.authUser)

				assert.Equal(t, http.StatusForbidden, status, body)
				assert.Equal(t, 0, service.calls)
				assert.Contains(t, body, "access to "+o.operation+" ", "the message must name the operation that was refused")
				assert.Contains(t, body, "comp-sfid", "the message must name the organization the scope check used")
			})
		}
	}
}
