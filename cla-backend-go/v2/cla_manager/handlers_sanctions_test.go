// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/cla_manager"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	v1User "github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	opCreateManager   = "createCLAManager"
	opDeleteManager   = "deleteCLAManager"
	opDesignee        = "createCLAManagerDesignee"
	opDesigneeByGroup = "createCLAManagerDesigneeByGroup"
	opInviteAdmin     = "inviteCompanyAdmin"
	opCreateRequest   = "createCLAManagerRequest"
	opApproveRequest  = "approveCLAManagerRequest"
	opDenyRequest     = "denyCLAManagerRequest"
	testReqID         = "req-id-1"
)

type fakeWriteOpsService struct {
	Service
	calls int
}

func (f *fakeWriteOpsService) CreateCLAManager(_ context.Context, _ *auth.User, _ string, _ cla_manager.CreateCLAManagerParams, _ string) (*models.CompanyClaManager, *models.ErrorResponse) {
	f.calls++
	return &models.CompanyClaManager{}, nil
}

func (f *fakeWriteOpsService) DeleteCLAManager(_ context.Context, _ *auth.User, _ string, _ cla_manager.DeleteCLAManagerParams) *models.ErrorResponse {
	f.calls++
	return nil
}

func (f *fakeWriteOpsService) CreateCLAManagerDesignee(_ context.Context, _ string, _ string, _ string) (*models.ClaManagerDesignee, error) {
	f.calls++
	return &models.ClaManagerDesignee{}, nil
}

func (f *fakeWriteOpsService) CreateCLAManagerDesigneeByGroup(_ context.Context, _ cla_manager.CreateCLAManagerDesigneeByGroupParams, _ []*projects_cla_groups.ProjectClaGroup) ([]*models.ClaManagerDesignee, string, error) {
	f.calls++
	return []*models.ClaManagerDesignee{}, "", nil
}

func (f *fakeWriteOpsService) InviteCompanyAdmin(_ context.Context, _ bool, _ string, _ string, _ string, _ string, _ *v1User.User, _ string) ([]*models.ClaManagerDesignee, error) {
	f.calls++
	return []*models.ClaManagerDesignee{}, nil
}

func (f *fakeWriteOpsService) CreateCLAManagerRequest(_ context.Context, _ bool, _ string, _ string, _ string, _ string, _ *auth.User) (*models.ClaManagerDesignee, error) {
	f.calls++
	return &models.ClaManagerDesignee{}, nil
}

func (f *fakeWriteOpsService) ApproveCLAManagerRequest(_ context.Context, _ *auth.User, _ *v1Models.Company, _ string, _ string) (*models.ClaManagerRequest, error) {
	f.calls++
	return &models.ClaManagerRequest{RequestID: "req-1"}, nil
}

func (f *fakeWriteOpsService) DenyCLAManagerRequest(_ context.Context, _ *auth.User, _ *v1Models.Company, _ string, _ string) (*models.ClaManagerRequest, error) {
	f.calls++
	return &models.ClaManagerRequest{RequestID: "req-1"}, nil
}

type fakeEasyCLAUserRepo struct {
	v1User.RepositoryService
}

func (f *fakeEasyCLAUserRepo) GetUser(userID string) (v1User.User, error) {
	return v1User.User{UserID: userID}, nil
}

type fakePCGRepoWithProjects struct {
	fakeProjectClaGroupRepo
}

func (f *fakePCGRepoWithProjects) GetProjectsIdsForClaGroup(_ context.Context, claGroupID string) ([]*projects_cla_groups.ProjectClaGroup, error) {
	return []*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-sfid", ClaGroupID: claGroupID}}, nil
}

func callWriteOp(t *testing.T, api *operations.EasyclaAPI, op string, authUser *auth.User) (int, string, http.Header) {
	t.Helper()
	username, email, reqID := "gate-tester", "gate-tester@example.com", testReqID
	fullName := "Some User"
	claGroupID := "cla-group-1"
	httpRequest := httptest.NewRequest(http.MethodPost, "/v4/cla-manager-op", nil)

	recorder := httptest.NewRecorder()
	switch op {
	case opCreateManager:
		require.NotNil(t, api.ClaManagerCreateCLAManagerHandler)
		api.ClaManagerCreateCLAManagerHandler.Handle(cla_manager.CreateCLAManagerParams{
			HTTPRequest: httpRequest, XUSERNAME: &username, XEMAIL: &email, XREQUESTID: &reqID,
			CompanyID: "company-1", ProjectSFID: "proj-sfid", Body: models.ClaManagerUser{},
		}, authUser).WriteResponse(recorder, runtime.JSONProducer())
	case opDeleteManager:
		require.NotNil(t, api.ClaManagerDeleteCLAManagerHandler)
		api.ClaManagerDeleteCLAManagerHandler.Handle(cla_manager.DeleteCLAManagerParams{
			HTTPRequest: httpRequest, XUSERNAME: &username, XEMAIL: &email, XREQUESTID: &reqID,
			CompanyID: "company-1", ProjectSFID: "proj-sfid", UserLFID: "some-user",
		}, authUser).WriteResponse(recorder, runtime.JSONProducer())
	case opDesignee:
		require.NotNil(t, api.ClaManagerCreateCLAManagerDesigneeHandler)
		api.ClaManagerCreateCLAManagerDesigneeHandler.Handle(cla_manager.CreateCLAManagerDesigneeParams{
			HTTPRequest: httpRequest, XUSERNAME: &username, XEMAIL: &email, XREQUESTID: &reqID,
			CompanyID: "company-1", ProjectSFID: "proj-sfid", Body: cla_manager.CreateCLAManagerDesigneeBody{UserEmail: "designee@example.com"},
		}, authUser).WriteResponse(recorder, runtime.JSONProducer())
	case opDesigneeByGroup:
		require.NotNil(t, api.ClaManagerCreateCLAManagerDesigneeByGroupHandler)
		api.ClaManagerCreateCLAManagerDesigneeByGroupHandler.Handle(cla_manager.CreateCLAManagerDesigneeByGroupParams{
			HTTPRequest: httpRequest, XUSERNAME: &username, XEMAIL: &email, XREQUESTID: &reqID,
			CompanyID: "company-1", ClaGroupID: claGroupID, Body: cla_manager.CreateCLAManagerDesigneeByGroupBody{UserEmail: "designee@example.com"},
		}, authUser).WriteResponse(recorder, runtime.JSONProducer())
	case opInviteAdmin:
		require.NotNil(t, api.ClaManagerInviteCompanyAdminHandler)
		api.ClaManagerInviteCompanyAdminHandler.Handle(cla_manager.InviteCompanyAdminParams{
			HTTPRequest: httpRequest, XREQUESTID: &reqID, UserID: "user-1",
			Body: cla_manager.InviteCompanyAdminBody{CompanyID: "company-1", ClaGroupID: &claGroupID, UserEmail: "admin@example.com", Name: fullName},
		}).WriteResponse(recorder, runtime.JSONProducer())
	case opCreateRequest:
		require.NotNil(t, api.ClaManagerCreateCLAManagerRequestHandler)
		api.ClaManagerCreateCLAManagerRequestHandler.Handle(cla_manager.CreateCLAManagerRequestParams{
			HTTPRequest: httpRequest, XUSERNAME: &username, XEMAIL: &email, XREQUESTID: &reqID,
			CompanyID: "company-1", ProjectSFID: "proj-sfid", Body: cla_manager.CreateCLAManagerRequestBody{UserEmail: "requester@example.com", FullName: &fullName},
		}, authUser).WriteResponse(recorder, runtime.JSONProducer())
	case opApproveRequest:
		require.NotNil(t, api.ClaManagerApproveCLAManagerRequestHandler)
		api.ClaManagerApproveCLAManagerRequestHandler.Handle(cla_manager.ApproveCLAManagerRequestParams{
			HTTPRequest: httpRequest, XUSERNAME: &username, XEMAIL: &email, XREQUESTID: &reqID,
			CompanyID: "company-1", ProjectSFID: "proj-sfid", RequestID: "req-1",
		}, authUser).WriteResponse(recorder, runtime.JSONProducer())
	case opDenyRequest:
		require.NotNil(t, api.ClaManagerDenyCLAManagerRequestHandler)
		api.ClaManagerDenyCLAManagerRequestHandler.Handle(cla_manager.DenyCLAManagerRequestParams{
			HTTPRequest: httpRequest, XUSERNAME: &username, XEMAIL: &email, XREQUESTID: &reqID,
			CompanyID: "company-1", ProjectSFID: "proj-sfid", RequestID: "req-1",
		}, authUser).WriteResponse(recorder, runtime.JSONProducer())
	default:
		t.Fatalf("unknown op %s", op)
	}
	return recorder.Code, recorder.Body.String(), recorder.Header()
}

func TestClaManagerWriteHandlersSanctionsGate(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	managerUser := &auth.User{UserName: "manager-user", Email: "manager@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
	orgAdminUser := &auth.User{UserName: "org-admin-user", Email: "org-admin@example.com", ACL: auth.ACL{Admin: true, Allowed: true}}

	authUserForOp := map[string]*auth.User{
		opCreateManager:   managerUser,
		opDeleteManager:   managerUser,
		opDesignee:        managerUser,
		opDesigneeByGroup: managerUser,
		opInviteAdmin:     nil,
		opCreateRequest:   orgAdminUser,
		opApproveRequest:  managerUser,
		opDenyRequest:     managerUser,
	}

	expectedCleanStatus := map[string]int{
		opCreateManager:   http.StatusOK,
		opDeleteManager:   http.StatusNoContent,
		opDesignee:        http.StatusOK,
		opDesigneeByGroup: http.StatusOK,
		opInviteAdmin:     http.StatusOK,
		opCreateRequest:   http.StatusOK,
		opApproveRequest:  http.StatusOK,
		opDenyRequest:     http.StatusOK,
	}

	companyCases := []struct {
		name    string
		company *v1Models.Company
		blocked bool
	}{
		{
			name:    "clean company",
			company: &v1Models.Company{CompanyID: "company-1", CompanyName: "Acme", CompanyExternalID: "comp-sfid"},
			blocked: false,
		},
		{
			name:    "sanctioned company via SSS",
			company: &v1Models.Company{CompanyID: "company-1", CompanyName: "Acme", CompanyExternalID: "comp-sfid", IsSanctioned: true, SanctionOrigin: "sss"},
			blocked: true,
		},
		{
			name:    "manually blocked company with empty origin",
			company: &v1Models.Company{CompanyID: "company-1", CompanyName: "Acme", CompanyExternalID: "comp-sfid", IsSanctioned: true, SanctionOrigin: ""},
			blocked: true,
		},
	}

	allOps := []string{opCreateManager, opDeleteManager, opDesignee, opDesigneeByGroup, opInviteAdmin, opCreateRequest, opApproveRequest, opDenyRequest}

	for _, op := range allOps {
		for _, cc := range companyCases {
			t.Run(op+" "+cc.name, func(t *testing.T) {
				api := operations.NewEasyclaAPI(nil)
				service := &fakeWriteOpsService{}
				companyService := &fakeV1CompanyService{company: cc.company}
				pcgRepo := &fakePCGRepoWithProjects{fakeProjectClaGroupRepo{cginfo: &projects_cla_groups.ProjectClaGroup{ClaGroupID: "cla-group-1"}}}
				userRepo := &fakeEasyCLAUserRepo{}
				Configure(api, service, companyService, "", "", pcgRepo, userRepo)

				status, body, headers := callWriteOp(t, api, op, authUserForOp[op])

				if !cc.blocked {
					assert.Equal(t, expectedCleanStatus[op], status, body)
					assert.Equal(t, 1, service.calls, "clean company must reach the service exactly once")
					return
				}

				assert.Equal(t, http.StatusForbidden, status, body)
				assert.Equal(t, 0, service.calls, "sanctioned company must never reach the service")
				assert.Equal(t, testReqID, headers.Get("X-Request-Id"))

				var payload map[string]interface{}
				require.NoError(t, json.Unmarshal([]byte(body), &payload))
				assert.Equal(t, "company_sanctioned", payload["code"])
				assert.Equal(t, "company-1", payload["company_id"])
				assert.Equal(t, "comp-sfid", payload["company_sfid"])
				assert.Contains(t, payload["message"], "trade compliance")
			})
		}
	}
}

// the ops whose company lookup exists only for the sanctions gate must fail closed, not skip the gate
func TestClaManagerCompanyLookupFailureFailsClosed(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	lookupCases := []struct {
		name    string
		company *v1Models.Company
		err     error
	}{
		{name: "lookup error", company: nil, err: errors.New("company lookup timeout")},
		{name: "nil company without error", company: nil, err: nil},
	}

	for _, op := range []string{opDesigneeByGroup, opInviteAdmin} {
		for _, lc := range lookupCases {
			t.Run(op+" "+lc.name, func(t *testing.T) {
				api := operations.NewEasyclaAPI(nil)
				service := &fakeWriteOpsService{}
				companyService := &fakeV1CompanyService{company: lc.company, err: lc.err}
				pcgRepo := &fakePCGRepoWithProjects{fakeProjectClaGroupRepo{cginfo: &projects_cla_groups.ProjectClaGroup{ClaGroupID: "cla-group-1"}}}
				userRepo := &fakeEasyCLAUserRepo{}
				Configure(api, service, companyService, "", "", pcgRepo, userRepo)

				var authUser *auth.User
				if op != opInviteAdmin {
					authUser = &auth.User{UserName: "lookup-tester", Email: "lookup-tester@example.com", ACL: auth.ACL{Allowed: true}}
				}
				status, body, _ := callWriteOp(t, api, op, authUser)

				assert.Equal(t, http.StatusBadRequest, status, body)
				assert.Equal(t, 0, service.calls, "failed company lookup must never reach the service")
			})
		}
	}
}
