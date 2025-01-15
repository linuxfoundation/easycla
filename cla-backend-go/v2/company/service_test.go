// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
package company

import (
	"context"
	"fmt"
	"testing"

	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	v1SignatureParams "github.com/communitybridge/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	v2Ops "github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/company"

	mock_company_repo "github.com/communitybridge/easycla/cla-backend-go/company/mocks"
	mock_project_repo "github.com/communitybridge/easycla/cla-backend-go/project/mocks"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	mock_pcg_repo "github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups/mocks"
	mock_signature_repo "github.com/communitybridge/easycla/cla-backend-go/signatures/mocks"
	mock_user_repo "github.com/communitybridge/easycla/cla-backend-go/users/mocks"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"
)

func TestGetCompanyProjectContributors(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name          string
		signatures    []*v1Models.Signature
		expectedOrder []string
	}{
		{
			name: "With all timestamps",
			signatures: []*v1Models.Signature{
				{
					SignatureID:           "signature-id-2",
					SignatureCreated:      "2021-09-13T11:59:00.981612+0000",
					SignatureApproved:     true,
					SignatureSigned:       true,
					SignatureEmbargoAcked: true,
					SignatureMajorVersion: "1",
					SignatureMinorVersion: "0",
					SignatureReferenceID:  "signature_reference_id",
				},
				{
					SignatureID:           "signature-id",
					SignatureCreated:      "2021-09-15T11:59:00.981612+0000",
					SignatureApproved:     true,
					SignatureSigned:       true,
					SignatureEmbargoAcked: true,
					SignatureMajorVersion: "1",
					SignatureMinorVersion: "0",
					SignatureReferenceID:  "signature_reference_id",
				},
				{
					SignatureID:           "signature-id-3",
					SignatureCreated:      "2021-09-14T11:59:00.981612+0000",
					SignatureApproved:     true,
					SignatureSigned:       true,
					SignatureEmbargoAcked: true,
					SignatureMajorVersion: "1",
					SignatureMinorVersion: "0",
					SignatureReferenceID:  "signature_reference_id",
				},
			},
			expectedOrder: []string{
				"2021-09-15T11:59:00Z",
				"2021-09-14T11:59:00Z",
				"2021-09-13T11:59:00Z",
			},
		},
		{
			name: "With empty timestamp",
			signatures: []*v1Models.Signature{
				{
					SignatureID:           "signature-id-2",
					SignatureCreated:      "2021-09-13T11:59:00.981612+0000",
					SignatureApproved:     true,
					SignatureSigned:       true,
					SignatureEmbargoAcked: true,
					SignatureMajorVersion: "1",
					SignatureMinorVersion: "0",
					SignatureReferenceID:  "signature_reference_id",
				},
				{
					SignatureID:           "signature-id",
					SignatureCreated:      "2021-09-15T11:59:00.981612+0000",
					SignatureApproved:     true,
					SignatureSigned:       true,
					SignatureEmbargoAcked: true,
					SignatureMajorVersion: "1",
					SignatureMinorVersion: "0",
					SignatureReferenceID:  "signature_reference_id",
				},
				{
					SignatureID:           "signature-id-3",
					SignatureCreated:      "2021-09-14T11:59:00.981612+0000",
					SignatureApproved:     true,
					SignatureSigned:       true,
					SignatureEmbargoAcked: true,
					SignatureMajorVersion: "1",
					SignatureMinorVersion: "0",
					SignatureReferenceID:  "signature_reference_id",
				},
				{
					SignatureID:           "signature-id-4",
					SignatureCreated:      "",
					SignatureApproved:     true,
					SignatureSigned:       true,
					SignatureEmbargoAcked: true,
					SignatureMajorVersion: "1",
					SignatureMinorVersion: "0",
					SignatureReferenceID:  "signature_reference_id_empty",
				},
			},
			expectedOrder: []string{
				"2021-09-15T11:59:00Z",
				"2021-09-14T11:59:00Z",
				"2021-09-13T11:59:00Z",
				"",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := v2Ops.GetCompanyProjectContributorsParams{
				CompanyID:   "company-id",
				ProjectSFID: "project-sfid",
			}
			empParams := v1SignatureParams.GetProjectCompanyEmployeeSignaturesParams{
				CompanyID:   "company-id",
				ProjectID:   "project-id",
				HTTPRequest: nil,
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
			mockProjectClaGroupRepo.EXPECT().GetClaGroupIDForProject(ctx, params.ProjectSFID).Return(&projects_cla_groups.ProjectClaGroup{
				ProjectSFID: "project-sfid",
				ClaGroupID:  "cla-group-id",
			}, nil)

			mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
			mockCompanyRepo.EXPECT().GetCompany(ctx, params.CompanyID).Return(&v1Models.Company{
				CompanyID: "company-id",
			}, nil)

			mock_signature_repo := mock_signature_repo.NewMockSignatureRepository(ctrl)
			mock_signature_repo.EXPECT().GetProjectCompanyEmployeeSignatures(ctx, empParams, nil).Return(&v1Models.Signatures{
				Signatures: tc.signatures,
			}, nil)

			mockUserRepo := mock_user_repo.NewMockUserRepository(ctrl)
			for _, sig := range tc.signatures {
				mockUserRepo.EXPECT().GetUser(sig.SignatureReferenceID).Return(&v1Models.User{
					Username:       "username",
					GithubUsername: "github-username",
					LfUsername:     "lf-username",
					UserID:         sig.SignatureReferenceID,
				}, nil)
			}

			mockProjectRepo := mock_project_repo.NewMockProjectRepository(ctrl)
			mockProjectRepo.EXPECT().GetCLAGroupByID(ctx, "cla-group-id", false).Return(&v1Models.ClaGroup{
				ProjectID: "project-id",
			}, nil)

			service := NewService(nil, mock_signature_repo, mockProjectRepo, mockUserRepo, mockCompanyRepo, mockProjectClaGroupRepo, nil)

			response, err := service.GetCompanyProjectContributors(ctx, &params)

			assert.Nil(t, err)

			fmt.Printf("response: %+v\n", response)

			assert.Equal(t, len(tc.expectedOrder), len(response.List))

			// check the timestamp order
			for i, expected := range tc.expectedOrder {
				assert.Equal(t, expected, response.List[i].Timestamp)
			}
		})
	}
}
