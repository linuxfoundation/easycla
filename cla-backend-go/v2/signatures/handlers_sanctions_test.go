// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	eventsMock "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	sigOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/signatures"
	mock_project "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	mock_v1_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testReqID = "req-id-1"

type fakeEclaAutoCreateService struct {
	ServiceInterface
	calls   int
	gotSig  string
	gotFlag bool
}

func (f *fakeEclaAutoCreateService) EclaAutoCreate(_ context.Context, signatureID string, autoCreateEcla bool) error {
	f.calls++
	f.gotSig = signatureID
	f.gotFlag = autoCreateEcla
	return nil
}

func sanctionsGateCompanyCases() []struct {
	name    string
	company *v1Models.Company
	blocked bool
} {
	return []struct {
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
}

func assertSanctionedBody(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	assert.Equal(t, testReqID, recorder.Header().Get("X-Request-Id"))
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "company_sanctioned", payload["code"])
	assert.Equal(t, "company-1", payload["company_id"])
	assert.Equal(t, "comp-sfid", payload["company_sfid"])
	assert.Contains(t, payload["message"], "trade compliance")
}

func TestUpdateApprovalListSanctionsGate(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	for _, cc := range sanctionsGateCompanyCases() {
		t.Run(cc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCompanyService := mock_company.NewMockIService(ctrl)
			mockCompanyService.EXPECT().GetCompany(gomock.Any(), "company-1").Return(cc.company, nil)

			// no EXPECTs beyond the clean path - a sanctioned company reaching either mock fails the test
			mockProjectService := mock_project.NewMockService(ctrl)
			mockV1SignatureService := mock_v1_signatures.NewMockSignatureService(ctrl)
			if !cc.blocked {
				mockProjectService.EXPECT().GetCLAGroupsByExternalSFID(gomock.Any(), "proj-sfid").Return(&v1Models.ClaGroups{}, nil)
				mockProjectService.EXPECT().GetCLAGroupByID(gomock.Any(), "cla-group-1").Return(&v1Models.ClaGroup{ProjectID: "cla-group-1"}, nil)
				mockV1SignatureService.EXPECT().UpdateApprovalList(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&v1Models.Signature{SignatureID: "sig-1"}, nil)
			}

			api := operations.NewEasyclaAPI(nil)
			Configure(api, mockProjectService, nil, mockCompanyService, mockV1SignatureService, nil, nil, nil, nil)
			require.NotNil(t, api.SignaturesUpdateApprovalListHandler)

			username, email, reqID := "manager-user", "manager@example.com", testReqID
			authUser := &auth.User{UserName: "manager-user", Email: "manager@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
			recorder := httptest.NewRecorder()
			api.SignaturesUpdateApprovalListHandler.Handle(sigOps.UpdateApprovalListParams{
				HTTPRequest: httptest.NewRequest(http.MethodPut, "/v4/signatures/project/proj-sfid/company/company-1/clagroup/cla-group-1/approval-list", nil),
				XUSERNAME:   &username, XEMAIL: &email, XREQUESTID: &reqID,
				ClaGroupID: "cla-group-1", CompanyID: "company-1", ProjectSFID: "proj-sfid",
				Body: &models.ApprovalList{AddEmailApprovalList: []string{"dev@example.com"}},
			}, authUser).WriteResponse(recorder, runtime.JSONProducer())

			if cc.blocked {
				assertSanctionedBody(t, recorder)
			} else {
				assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestEclaAutoCreateSanctionsGate(t *testing.T) {
	// the gate applies to both enabling and disabling auto-create
	for _, autoCreate := range []bool{true, false} {
		for _, cc := range sanctionsGateCompanyCases() {
			if autoCreate && !cc.blocked {
				// the clean enable path spawns employee-signature workers - out of scope here
				continue
			}
			t.Run(fmt.Sprintf("autoCreate=%v %s", autoCreate, cc.name), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				cclaSig := &v1Models.Signature{SignatureID: "ccla-sig-1", SignatureACL: []v1Models.User{{LfUsername: "org-admin"}}}
				mockV1SignatureService := mock_v1_signatures.NewMockSignatureService(ctrl)
				mockV1SignatureService.EXPECT().GetCorporateSignature(gomock.Any(), "cla-group-1", "company-1", gomock.Any(), gomock.Any()).Return(cclaSig, nil)

				mockCompanyService := mock_company.NewMockIService(ctrl)
				mockCompanyService.EXPECT().GetCompany(gomock.Any(), "company-1").Return(cc.company, nil)

				mockProjectService := mock_project.NewMockService(ctrl)
				mockProjectService.EXPECT().GetCLAGroupByID(gomock.Any(), "cla-group-1").Return(&v1Models.ClaGroup{ProjectName: "My Project"}, nil)

				// no LogEvent EXPECT on the blocked path - a sanctioned company logging an event fails the test
				mockEvents := eventsMock.NewMockService(ctrl)
				if !cc.blocked {
					mockEvents.EXPECT().LogEvent(gomock.Any())
				}

				v2Service := &fakeEclaAutoCreateService{}
				api := operations.NewEasyclaAPI(nil)
				Configure(api, mockProjectService, nil, mockCompanyService, mockV1SignatureService, nil, mockEvents, v2Service, nil)
				require.NotNil(t, api.SignaturesEclaAutoCreateHandler)

				username, email, reqID := "org-admin", "org-admin@example.com", testReqID
				authUser := &auth.User{UserName: "org-admin", Email: "org-admin@example.com", ACL: auth.ACL{Allowed: true}}
				recorder := httptest.NewRecorder()
				api.SignaturesEclaAutoCreateHandler.Handle(sigOps.EclaAutoCreateParams{
					HTTPRequest: httptest.NewRequest(http.MethodPut, "/v4/signatures/cla-group-1/company-1/ecla-auto-create", nil),
					XUSERNAME:   &username, XEMAIL: &email, XREQUESTID: &reqID,
					ClaGroupID: "cla-group-1", CompanyID: "company-1",
					Body: &models.EclaAutoCreate{AutoCreateEcla: autoCreate},
				}, authUser).WriteResponse(recorder, runtime.JSONProducer())

				if cc.blocked {
					assertSanctionedBody(t, recorder)
					assert.Equal(t, 0, v2Service.calls, "sanctioned company must never reach the service")
				} else {
					assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
					assert.Equal(t, 1, v2Service.calls)
					assert.Equal(t, "ccla-sig-1", v2Service.gotSig)
					assert.Equal(t, autoCreate, v2Service.gotFlag)
				}
			})
		}
	}
}
