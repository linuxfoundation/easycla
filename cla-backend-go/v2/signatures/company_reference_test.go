// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-openapi/runtime"
	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	sigOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_projects_cla_groups "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	mock_v1_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/approvals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type companyReferenceApprovals struct {
	approvals.IRepository
}

func (companyReferenceApprovals) GetApprovalListBySignature(string) ([]approvals.ApprovalItem, error) {
	return nil, nil
}

func TestGetProjectCompanySignaturesCompanyReference(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	const (
		companyID   = "9b8e7d66-40a5-4cde-9f00-3e1d1a2b3c4d"
		companySFID = "0014100000Te0yqQAB"
		projectSFID = "a0941000005ouJFAAY"
		claGroupID  = "e1e30240-a722-4c82-a648-121681d959c7"
		signatureID = "2f1c6c1e-6a2d-4c3b-9d7e-8a5b4c3d2e1f"
	)
	persisted := &v1Models.Company{CompanyID: companyID, CompanyExternalID: companySFID}
	signed := &v1Models.Signature{
		SignatureID: signatureID, ProjectID: claGroupID, SignatureReferenceID: companyID,
		SignatureType: utils.SignatureTypeCCLA, SignatureSigned: true, SignatureApproved: true,
		AutoCreateECLA: true,
	}
	tests := []struct {
		name         string
		reference    string
		company      *v1Models.Company
		companyErr   error
		denied       bool
		mappingErr   error
		signature    *v1Models.Signature
		signatureErr error
		status       int
		errorCode    string
	}{
		{
			name: "virtual SFID returns empty signatures", reference: companySFID,
			company: company.VirtualCompany(companySFID, "Acme"), status: http.StatusOK,
		},
		{
			name: "short SFID authorizes the resolved organization", reference: "0014100000Te0yq",
			company: company.VirtualCompany(companySFID, "Acme"), status: http.StatusOK,
		},
		{
			name: "materialized SFID queries the real company UUID", reference: companySFID,
			company: persisted, signature: signed, status: http.StatusOK,
		},
		{
			name: "existing UUID retains signed CCLA", reference: companyID,
			company: persisted, signature: signed, status: http.StatusOK,
		},
		{
			name: "existing UUID without CCLA retains empty response", reference: companyID,
			company: persisted, status: http.StatusOK,
		},
		{
			name: "sanctioned company remains readable", reference: companySFID,
			company:   &v1Models.Company{CompanyID: companyID, CompanyExternalID: companySFID, IsSanctioned: true},
			signature: signed, status: http.StatusOK,
		},
		{
			name: "missing company preserves bad request with not found code", reference: companySFID,
			companyErr: &utils.CompanyNotFound{CompanySFID: companySFID},
			status:     http.StatusBadRequest, errorCode: "404",
		},
		{
			name: "company lookup error remains a bad request", reference: companyID,
			companyErr: errors.New("company lookup failed"),
			status:     http.StatusBadRequest, errorCode: "400",
		},
		{
			name: "unauthorized virtual company read is forbidden", reference: companySFID,
			company: company.VirtualCompany(companySFID, "Acme"), denied: true,
			status: http.StatusForbidden, errorCode: "403",
		},
		{
			name: "project mapping error remains a bad request", reference: companySFID,
			company: persisted, mappingErr: errors.New("project mapping failed"),
			status: http.StatusBadRequest, errorCode: "400",
		},
		{
			name: "signature query error remains a bad request", reference: companySFID,
			company: persisted, signatureErr: errors.New("signature query failed"),
			status: http.StatusBadRequest, errorCode: "400",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			companyService := mock_company.NewMockIService(ctrl)
			companyService.EXPECT().ResolveCompany(gomock.Any(), tc.reference).Return(tc.company, tc.companyErr)
			projectRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
			signatureService := mock_v1_signatures.NewMockSignatureService(ctrl)
			if tc.companyErr == nil {
				if tc.denied {
					projectRepo.EXPECT().GetClaGroupIDForProject(gomock.Any(), projectSFID).Return(nil, nil)
				} else {
					projectRepo.EXPECT().GetClaGroupIDForProject(gomock.Any(), projectSFID).
						Return(&projects_cla_groups.ProjectClaGroup{ClaGroupID: claGroupID}, tc.mappingErr)
					if tc.mappingErr == nil {
						signatureService.EXPECT().GetProjectCompanySignature(
							gomock.Any(), tc.company.CompanyID, claGroupID, aws.Bool(true), aws.Bool(true), nil, aws.Int64(HugePageSize),
						).Return(tc.signature, tc.signatureErr)
					}
				}
			}
			v2Service := &Service{
				v1SignatureService: signatureService, projectsClaGroupsRepo: projectRepo,
				approvalsRepos: companyReferenceApprovals{},
			}
			api := operations.NewEasyclaAPI(nil)
			Configure(api, nil, nil, companyService, signatureService, nil, nil, v2Service, projectRepo)
			scopeID := companySFID
			if tc.denied {
				scopeID = "0014100000Other0AAA"
			}
			authUser := &auth.User{
				UserName: "company-manager", Email: "manager@example.com",
				ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.Organization, ID: scopeID}}},
			}
			reqID := testReqID
			recorder := httptest.NewRecorder()
			api.SignaturesGetProjectCompanySignaturesHandler.Handle(sigOps.GetProjectCompanySignaturesParams{
				HTTPRequest: httptest.NewRequest(http.MethodGet, "/v4/signatures/project/"+projectSFID+"/company/"+tc.reference, nil),
				CompanyID:   tc.reference, ProjectSFID: projectSFID,
				XUSERNAME: &authUser.UserName, XEMAIL: &authUser.Email, XREQUESTID: &reqID,
			}, authUser).WriteResponse(recorder, runtime.JSONProducer())

			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			assert.Equal(t, reqID, recorder.Header().Get("X-Request-Id"))
			if tc.status != http.StatusOK {
				var payload models.ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
				assert.Equal(t, tc.errorCode, payload.Code)
				return
			}
			var payload models.CorporateSignatures
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			if tc.signature == nil {
				// Preserve the existing empty response produced by the signature converters.
				assert.JSONEq(t, `{"resultCount":0,"signatures":null,"totalCount":0}`, recorder.Body.String())
				assert.Empty(t, payload.Signatures)
				assert.Zero(t, payload.ResultCount)
				assert.Zero(t, payload.TotalCount)
			} else {
				require.Len(t, payload.Signatures, 1)
				assert.Equal(t, signatureID, payload.Signatures[0].SignatureID)
				assert.Equal(t, companySFID, payload.Signatures[0].SignatureReferenceID)
				assert.True(t, payload.Signatures[0].AutoCreateECLA)
				assert.EqualValues(t, 1, payload.ResultCount)
				assert.EqualValues(t, 1, payload.TotalCount)
			}
		})
	}
}
