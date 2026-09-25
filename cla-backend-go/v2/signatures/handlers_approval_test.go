// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	sigOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/signatures"
	mock_project "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	signatureService "github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	mock_v1_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateApprovalListResponse(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	for _, tc := range []struct {
		name   string
		result *v1Models.Signature
		err    error
		status int
	}{
		{name: "success", result: &v1Models.Signature{SignatureID: "sig-1"}, status: http.StatusOK},
		{name: "signature ACL forbidden", err: signatureService.NewForbiddenError("caller is not in the signature ACL"), status: http.StatusForbidden},
		{name: "forbidden with result", result: &v1Models.Signature{SignatureID: "sig-1"}, err: signatureService.NewForbiddenError("caller is not in the signature ACL"), status: http.StatusForbidden},
		{name: "other service error", err: errors.New("approval update failed"), status: http.StatusBadRequest},
		{name: "missing result", status: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			companyModel := &v1Models.Company{CompanyID: "company-1", CompanyExternalID: "comp-sfid"}
			claGroup := &v1Models.ClaGroup{ProjectID: "cla-group-1"}
			authUser := &auth.User{
				UserName: "manager-user", Email: "manager@example.com",
				ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}},
			}
			mockCompanyService := mock_company.NewMockIService(ctrl)
			mockCompanyService.EXPECT().GetCompany(gomock.Any(), "company-1").Return(companyModel, nil)
			mockProjectService := mock_project.NewMockService(ctrl)
			mockProjectService.EXPECT().GetCLAGroupsByExternalSFID(gomock.Any(), "proj-sfid").Return(&v1Models.ClaGroups{}, nil)
			mockProjectService.EXPECT().GetCLAGroupByID(gomock.Any(), "cla-group-1").Return(claGroup, nil)
			mockSignatureService := mock_v1_signatures.NewMockSignatureService(ctrl)
			mockSignatureService.EXPECT().UpdateApprovalList(gomock.Any(), authUser, claGroup, companyModel, "cla-group-1",
				&v1Models.ApprovalList{AddEmailApprovalList: []string{"dev@example.com"}}, "proj-sfid").Return(tc.result, tc.err)

			api := operations.NewEasyclaAPI(nil)
			Configure(api, mockProjectService, nil, mockCompanyService, mockSignatureService, nil, nil, nil, nil)
			username, email, reqID := authUser.UserName, authUser.Email, testReqID
			recorder := httptest.NewRecorder()
			api.SignaturesUpdateApprovalListHandler.Handle(sigOps.UpdateApprovalListParams{
				HTTPRequest: httptest.NewRequest(http.MethodPut, "/v4/signatures/project/proj-sfid/company/company-1/clagroup/cla-group-1/approval-list", nil),
				XUSERNAME:   &username, XEMAIL: &email, XREQUESTID: &reqID,
				ClaGroupID: "cla-group-1", CompanyID: "company-1", ProjectSFID: "proj-sfid",
				Body: &models.ApprovalList{AddEmailApprovalList: []string{"dev@example.com"}},
			}, authUser).WriteResponse(recorder, runtime.JSONProducer())

			assert.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			assert.Equal(t, reqID, recorder.Header().Get("X-Request-Id"))
			if tc.status == http.StatusOK {
				var payload models.Signature
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
				assert.Equal(t, tc.result.SignatureID, payload.SignatureID)
			} else {
				var payload models.ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
				assert.Equal(t, strconv.Itoa(tc.status), payload.Code)
				assert.Equal(t, reqID, payload.XRequestID)
				if tc.err != nil {
					assert.Contains(t, payload.Message, tc.err.Error())
				}
			}
		})
	}
}
