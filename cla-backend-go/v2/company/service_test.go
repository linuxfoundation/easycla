// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
package company

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	v1SignatureParams "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	v2Ops "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/company"

	mock_company_repo "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	mock_project_repo "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/project/repository"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_pcg_repo "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	mock_signature_repo "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	mock_user_repo "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"

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
					GitlabUsername: "gitlab-username",
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
				assert.Equal(t, "gitlab-username", response.List[i].GitlabID)
			}
		})
	}
}

func cclaSignaturesParams(companyID string, nextKey *string) v1SignatureParams.GetCompanySignaturesParams {
	return v1SignatureParams.GetCompanySignaturesParams{
		CompanyID:     companyID,
		CompanyName:   aws.String(""),
		SignatureType: aws.String("ccla"),
		NextKey:       nextKey,
	}
}

func TestGetCompanyClaGroups(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	companySFID := "0014100000Te0000AAA"
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{
			CompanyID:         "company-id-1",
			CompanyExternalID: companySFID,
			CompanyName:       "Acme",
			SigningEntityName: "",
		},
		{
			CompanyID:         "company-id-2",
			CompanyExternalID: companySFID,
			CompanyName:       "Acme",
			SigningEntityName: "Acme Sub",
			IsSanctioned:      true,
			SanctionedDate:    "2026-08-20T10:11:12.123456+0000",
		},
		{
			CompanyID:         "company-id-3",
			CompanyExternalID: companySFID,
			CompanyName:       "Acme",
			SigningEntityName: "Acme Cleared",
			IsSanctioned:      false,
			SanctionedDate:    "2026-01-01T00:00:00Z",
		},
	}, nil)

	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-3", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{}, nil)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{
			{
				SignatureID:     "signature-id-1",
				ProjectID:       "cla-group-id",
				SignatureSigned: true,
				SignedOn:        "2023-01-02T03:04:05Z",
				SignatoryName:   "Alex Signer",
				AutoCreateECLA:  true,
				SignatureACL: []v1Models.User{
					{UserID: "user-id-bob", LfUsername: "bob"},
					{UserID: "user-id-alice", LfUsername: "alice"},
				},
				DomainApprovalList:         []string{"acme.example"},
				GithubOrgApprovalList:      []string{"acme-oss", "acme-labs"},
				GitlabUsernameApprovalList: []string{"acme-dev"},
			},
		},
	}, nil)
	// The v1 converter fills SignedOn with the creation date when signed_on is missing - the stored
	// record (GetItemSignature) is what decides whether the lens shows a date
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-2", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{
			{
				SignatureID:      "signature-id-2",
				ProjectID:        "cla-group-id",
				SignatureSigned:  true,
				SignatureCreated: "2023-05-06T07:08:09Z",
				SignedOn:         "2023-05-06T07:08:09Z",
			},
		},
	}, nil)
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, "signature-id-1").Return(&signatures.ItemSignature{SignatureID: "signature-id-1", DateCreated: "2023-01-01T00:00:00.000000+0000", SignedOn: "2023-01-02T03:04:05.678901+0000"}, nil)
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, "signature-id-2").Return(&signatures.ItemSignature{SignatureID: "signature-id-2", DateCreated: "2023-05-06T07:08:09Z"}, nil)
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, "cla-group-id", aws.String("company-id-1"), true, nil).Return(int64(3), nil)
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, "cla-group-id", aws.String("company-id-2"), true, nil).Return(int64(0), nil)

	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-id").Times(1).Return([]*projects_cla_groups.ProjectClaGroup{
		{
			ClaGroupID:     "cla-group-id",
			ClaGroupName:   "Test CLA Group",
			FoundationSFID: "foundation-sfid",
			FoundationName: "Test Foundation",
			ProjectSFID:    "foundation-sfid",
			ProjectName:    "Test Foundation",
		},
		{
			ClaGroupID:     "cla-group-id",
			ClaGroupName:   "Test CLA Group",
			FoundationSFID: "foundation-sfid",
			FoundationName: "Test Foundation",
			ProjectSFID:    "project-sfid-2",
			ProjectName:    "Zeta",
		},
		{
			ClaGroupID:     "cla-group-id",
			ClaGroupName:   "Test CLA Group",
			FoundationSFID: "foundation-sfid",
			FoundationName: "Test Foundation",
			ProjectSFID:    "project-sfid-1",
			ProjectName:    "Alpha",
		},
	}, nil)

	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

	assert.Nil(t, err)
	assert.Equal(t, companySFID, result.CompanySFID)
	assert.Equal(t, int64(2), result.ResultCount)
	assert.Equal(t, int64(2), result.TotalCount)
	assert.Len(t, result.List, 2)

	first := result.List[0]
	assert.Equal(t, "company-id-1", first.CompanyID)
	assert.Equal(t, companySFID, first.CompanySFID)
	assert.Equal(t, "Acme", first.CompanyName)
	assert.Equal(t, "Acme", first.SigningEntityName)
	assert.Equal(t, "cla-group-id", first.ClaGroupID)
	assert.Equal(t, "Test CLA Group", first.ClaGroupName)
	assert.Equal(t, "foundation-sfid", first.FoundationSFID)
	assert.Equal(t, "Test Foundation", first.FoundationName)
	assert.Equal(t, []models.CompanyClaGroupProject{
		{ProjectSFID: "project-sfid-1", ProjectName: "Alpha"},
		{ProjectSFID: "project-sfid-2", ProjectName: "Zeta"},
	}, first.Projects)
	assert.True(t, first.Signed)
	assert.Equal(t, "2023-01-02T03:04:05Z", first.SignedOn)
	assert.Equal(t, "Alex Signer", first.SignedBy)
	assert.Equal(t, "signature-id-1", first.SignatureID)
	assert.False(t, first.Sanctioned)
	assert.Equal(t, "", first.SanctionedAt)
	assert.Equal(t, int64(3), first.ApprovedContributorsCount)
	assert.Equal(t, int64(4), first.ApprovalCriteriaCount)
	assert.Equal(t, int64(2), first.ClaManagersCount)
	assert.Equal(t, []models.CompanyClaGroupManager{
		{UserID: "user-id-alice", LfUsername: "alice"},
		{UserID: "user-id-bob", LfUsername: "bob"},
	}, first.ClaManagers)
	assert.False(t, first.NeedsClaManager)
	assert.True(t, first.AutoCreateECLA)

	second := result.List[1]
	assert.Equal(t, "company-id-2", second.CompanyID)
	assert.Equal(t, "Acme Sub", second.SigningEntityName)
	assert.Equal(t, "signature-id-2", second.SignatureID)
	assert.Equal(t, "", second.SignedOn, "no creation-date fallback for a CCLA without a stored signed_on")
	assert.Equal(t, "", second.SignedBy)
	assert.True(t, second.Sanctioned)
	assert.Equal(t, "2026-08-20T10:11:12Z", second.SanctionedAt)
	assert.Equal(t, int64(0), second.ApprovedContributorsCount)
	assert.Equal(t, int64(0), second.ApprovalCriteriaCount)
	assert.Equal(t, int64(0), second.ClaManagersCount)
	assert.True(t, second.NeedsClaManager)
	assert.False(t, second.AutoCreateECLA)
}

func TestGetCompanyClaGroupsPaging(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	companySFID := "0014100000Te0000AAH"
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{CompanyID: "company-id-1", CompanyExternalID: companySFID, CompanyName: "Acme"},
		{CompanyID: "company-id-2", CompanyExternalID: companySFID, CompanyName: "Acme", SigningEntityName: "Acme Sub"},
	}, nil)

	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{
			{SignatureID: "signature-id-1", ProjectID: "cla-group-id", SignatureSigned: true},
		},
	}, nil)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-2", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{
			{SignatureID: "signature-id-2", ProjectID: "cla-group-id", SignatureSigned: true},
		},
	}, nil)
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, gomock.Any()).Times(2).Return(nil, nil)
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, "cla-group-id", aws.String("company-id-1"), true, nil).Return(int64(0), nil)
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, "cla-group-id", aws.String("company-id-2"), true, nil).Return(int64(0), nil)

	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-id").Times(1).Return([]*projects_cla_groups.ProjectClaGroup{
		{ClaGroupID: "cla-group-id", ClaGroupName: "Test CLA Group", FoundationSFID: "foundation-sfid", FoundationName: "Test Foundation", ProjectSFID: "foundation-sfid", ProjectName: "Test Foundation"},
	}, nil)

	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID, aws.Int64(1), aws.Int64(1))

	assert.Nil(t, err)
	assert.Equal(t, int64(2), result.TotalCount, "total before paging")
	assert.Equal(t, int64(1), result.ResultCount, "returned page size")
	if assert.Len(t, result.List, 1) {
		assert.Equal(t, "company-id-2", result.List[0].CompanyID, "second row by signing entity sort order")
		assert.Equal(t, "Acme Sub", result.List[0].SigningEntityName)
	}
}

func TestGetCompanyClaGroupsSignedByOmitsBlankWithManagers(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	companySFID := "0014100000Te0000AAG"
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{
			CompanyID:         "company-id-1",
			CompanyExternalID: companySFID,
			CompanyName:       "Acme",
			SigningEntityName: "Acme",
		},
	}, nil)

	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{
			{
				SignatureID:     "signature-id-1",
				ProjectID:       "cla-group-id",
				SignatureSigned: true,
				SignedOn:        "2023-01-02T03:04:05Z",
				SignatureACL: []v1Models.User{
					{UserID: "user-id-alice", LfUsername: "alice"},
				},
			},
		},
	}, nil)
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, "signature-id-1").Return(&signatures.ItemSignature{SignatureID: "signature-id-1", SignedOn: "2023-01-02T03:04:05Z"}, nil)
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, "cla-group-id", aws.String("company-id-1"), true, nil).Return(int64(0), nil)

	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-id").Return([]*projects_cla_groups.ProjectClaGroup{
		{
			ClaGroupID:     "cla-group-id",
			ClaGroupName:   "Test CLA Group",
			FoundationSFID: "foundation-sfid",
			FoundationName: "Test Foundation",
			ProjectSFID:    "project-sfid-1",
			ProjectName:    "Alpha",
		},
	}, nil)

	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

	assert.Nil(t, err)
	assert.Len(t, result.List, 1)
	assert.Equal(t, int64(1), result.List[0].ClaManagersCount)
	assert.Equal(t, "", result.List[0].SignedBy)
}

func TestApprovalCriteriaCount(t *testing.T) {
	tests := []struct {
		name     string
		sig      *v1Models.Signature
		expected int64
	}{
		{
			name:     "no approval lists",
			sig:      &v1Models.Signature{},
			expected: 0,
		},
		{
			name: "all six lists contribute",
			sig: &v1Models.Signature{
				EmailApprovalList:          []string{"dev@acme.example"},
				DomainApprovalList:         []string{"acme.example", "acme.test"},
				GithubUsernameApprovalList: []string{"acme-dev", "acme-ops", "acme-qa"},
				GithubOrgApprovalList:      []string{"acme-oss"},
				GitlabUsernameApprovalList: []string{"acme-gl-dev", "acme-gl-ops"},
				GitlabOrgApprovalList:      []string{"acme-gl"},
			},
			expected: 10,
		},
		{
			name: "a single domain rule counts once regardless of who it covers",
			sig: &v1Models.Signature{
				DomainApprovalList: []string{"acme.example"},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, approvalCriteriaCount(tt.sig))
		})
	}
}

func TestGetCompanyClaGroupsCompanyNotFound(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, "0014100000Te0000AAB", true).Return(nil, &utils.CompanyNotFound{CompanySFID: "0014100000Te0000AAB"})

	service := NewService(nil, mock_signature_repo.NewMockSignatureRepository(ctrl), mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mock_pcg_repo.NewMockRepository(ctrl), nil)
	result, err := service.GetCompanyClaGroups(ctx, "0014100000Te0000AAB", nil, nil)

	assert.Nil(t, err)
	assert.Equal(t, "0014100000Te0000AAB", result.CompanySFID)
	assert.Equal(t, int64(0), result.ResultCount)
	assert.NotNil(t, result.List)
	assert.Len(t, result.List, 0)
}

func TestGetCompanyClaGroupsOrphanClaGroup(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	companySFID := "0014100000Te0000AAC"
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{
			CompanyID:         "company-id-1",
			CompanyExternalID: companySFID,
			CompanyName:       "Acme",
			SigningEntityName: "Acme",
		},
	}, nil)

	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{
			{
				SignatureID: "signature-id-1",
				ProjectID:   "orphan-cla-group-id",
			},
		},
	}, nil)
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, "signature-id-1").Return(nil, nil)
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, "orphan-cla-group-id", aws.String("company-id-1"), true, nil).Return(int64(0), nil)

	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "orphan-cla-group-id").Return([]*projects_cla_groups.ProjectClaGroup{}, nil)

	mockProjectRepo := mock_project_repo.NewMockProjectRepository(ctrl)
	mockProjectRepo.EXPECT().GetCLAGroupByID(ctx, "orphan-cla-group-id", DontLoadRepoDetails).Return(&v1Models.ClaGroup{ProjectName: "Orphan Group", FoundationSFID: "orphan-foundation-sfid"}, nil)

	service := NewService(nil, mockSignatureRepo, mockProjectRepo, mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

	assert.Nil(t, err)
	assert.Len(t, result.List, 1)
	row := result.List[0]
	assert.Equal(t, "Orphan Group", row.ClaGroupName)
	assert.Equal(t, "orphan-foundation-sfid", row.FoundationSFID)
	assert.Equal(t, "", row.FoundationName)
	assert.Len(t, row.Projects, 0)
	assert.False(t, row.Signed)
	assert.False(t, row.NeedsClaManager)
}

func TestGetCompanyClaGroupsOrphanClaGroupErrors(t *testing.T) {
	ctx := context.Background()
	companySFID := "0014100000Te0000AAF"

	newMocks := func(ctrl *gomock.Controller) (*mock_signature_repo.MockSignatureRepository, *mock_project_repo.MockProjectRepository, Service) {
		mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
		mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
			{
				CompanyID:         "company-id-1",
				CompanyExternalID: companySFID,
				CompanyName:       "Acme",
				SigningEntityName: "Acme",
			},
		}, nil)
		mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
		mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
			Signatures: []*v1Models.Signature{
				{
					SignatureID: "signature-id-1",
					ProjectID:   "orphan-cla-group-id",
				},
			},
		}, nil)
		mockSignatureRepo.EXPECT().GetItemSignature(ctx, "signature-id-1").Return(&signatures.ItemSignature{SignatureID: "signature-id-1"}, nil)
		mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
		mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "orphan-cla-group-id").Return([]*projects_cla_groups.ProjectClaGroup{}, nil)
		mockProjectRepo := mock_project_repo.NewMockProjectRepository(ctrl)
		return mockSignatureRepo, mockProjectRepo, NewService(nil, mockSignatureRepo, mockProjectRepo, mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	}

	for _, tc := range []struct {
		name  string
		cgErr error
	}{
		{"cla group not found tolerated", &utils.CLAGroupNotFound{CLAGroupID: "orphan-cla-group-id"}},
		{"project does not exist tolerated", repository.ErrProjectDoesNotExist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockSignatureRepo, mockProjectRepo, service := newMocks(ctrl)
			mockProjectRepo.EXPECT().GetCLAGroupByID(ctx, "orphan-cla-group-id", DontLoadRepoDetails).Return(nil, tc.cgErr)
			mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, "orphan-cla-group-id", aws.String("company-id-1"), true, nil).Return(int64(0), nil)

			result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

			assert.Nil(t, err)
			assert.Len(t, result.List, 1)
			row := result.List[0]
			assert.Equal(t, "orphan-cla-group-id", row.ClaGroupID)
			assert.Equal(t, "", row.ClaGroupName)
			assert.Equal(t, "", row.FoundationSFID)
		})
	}

	t.Run("repository error propagated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, mockProjectRepo, service := newMocks(ctrl)
		repoErr := errors.New("dynamodb failure")
		mockProjectRepo.EXPECT().GetCLAGroupByID(ctx, "orphan-cla-group-id", DontLoadRepoDetails).Return(nil, repoErr)

		result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

		assert.Nil(t, result)
		assert.Equal(t, repoErr, err)
	})
}

func TestGetCompanyClaGroupsSignaturePagination(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	companySFID := "0014100000Te0000AAD"
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{
			CompanyID:         "company-id-1",
			CompanyExternalID: companySFID,
			CompanyName:       "Acme",
			SigningEntityName: "Acme",
		},
	}, nil)

	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures:     []*v1Models.Signature{{SignatureID: "signature-id-1", ProjectID: "cla-group-id", SignatureSigned: true, SignedOn: "2020-01-01T00:00:00Z"}},
		LastKeyScanned: "next-key",
	}, nil)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", aws.String("next-key")), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{
			{SignatureID: "signature-id-2", ProjectID: "cla-group-id", SignatureSigned: true, SignedOn: "2020-01-01T00:01:30Z"},
			{SignatureID: "signature-id-3", ProjectID: "cla-group-id-2", SignatureSigned: true, SignedOn: "2021-01-01T00:00:00Z"},
		},
	}, nil)
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, "signature-id-2").Return(&signatures.ItemSignature{SignatureID: "signature-id-2", SignedOn: "2020-01-01T00:01:30Z"}, nil)
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, "signature-id-3").Return(&signatures.ItemSignature{SignatureID: "signature-id-3", SignedOn: "2021-01-01T00:00:00Z"}, nil)
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, "cla-group-id", aws.String("company-id-1"), true, nil).Times(1).Return(int64(1), nil)
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, "cla-group-id-2", aws.String("company-id-1"), true, nil).Times(1).Return(int64(2), nil)

	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-id").Times(1).Return([]*projects_cla_groups.ProjectClaGroup{
		{
			ClaGroupID:     "cla-group-id",
			ClaGroupName:   "Test CLA Group",
			FoundationSFID: "foundation-sfid",
			FoundationName: "Test Foundation",
			ProjectSFID:    "project-sfid-1",
			ProjectName:    "Alpha",
		},
	}, nil)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-id-2").Times(1).Return([]*projects_cla_groups.ProjectClaGroup{
		{
			ClaGroupID:     "cla-group-id-2",
			ClaGroupName:   "Second CLA Group",
			FoundationSFID: "foundation-sfid",
			FoundationName: "Test Foundation",
			ProjectSFID:    "foundation-sfid",
			ProjectName:    "Test Foundation",
		},
	}, nil)

	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

	assert.Nil(t, err)
	assert.Len(t, result.List, 2)
	assert.Equal(t, "signature-id-3", result.List[0].SignatureID)
	assert.Equal(t, "cla-group-id-2", result.List[0].ClaGroupID)
	assert.Equal(t, int64(2), result.List[0].ApprovedContributorsCount)
	assert.Len(t, result.List[0].Projects, 0)
	assert.Equal(t, "foundation-sfid", result.List[0].FoundationSFID)
	assert.Equal(t, "Test Foundation", result.List[0].FoundationName)
	assert.Equal(t, "Second CLA Group", result.List[0].ClaGroupName)
	assert.Equal(t, "signature-id-2", result.List[1].SignatureID)
	assert.Equal(t, "cla-group-id", result.List[1].ClaGroupID)
	assert.Equal(t, "2020-01-01T00:01:30Z", result.List[1].SignedOn)
	assert.Equal(t, int64(1), result.List[1].ApprovedContributorsCount)
}

func TestGetCompanyClaGroupsStoredSignedOnErrors(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	companySFID := "0014100000Te0000AAJ"
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{CompanyID: "company-id-1", CompanyExternalID: companySFID, CompanyName: "Acme"},
	}, nil)
	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{{SignatureID: "signature-id-1", ProjectID: "cla-group-id", SignatureSigned: true, SignedOn: "2020-01-01T00:00:00Z"}},
	}, nil)
	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-id").Return([]*projects_cla_groups.ProjectClaGroup{
		{ClaGroupID: "cla-group-id", ClaGroupName: "Test CLA Group", FoundationSFID: "foundation-sfid", ProjectSFID: "foundation-sfid"},
	}, nil)
	repoErr := errors.New("dynamodb failure")
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, "signature-id-1").Return(nil, repoErr)

	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

	assert.Nil(t, result)
	assert.Equal(t, repoErr, err)
}

func TestGetCompanyClaGroupsApprovedContributorsCountErrors(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	companySFID := "0014100000Te0000AAK"
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{CompanyID: "company-id-1", CompanyExternalID: companySFID, CompanyName: "Acme"},
	}, nil)
	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{{SignatureID: "signature-id-1", ProjectID: "cla-group-id", SignatureSigned: true}},
	}, nil)
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, "signature-id-1").Return(nil, nil)
	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-id").Return([]*projects_cla_groups.ProjectClaGroup{
		{ClaGroupID: "cla-group-id", ClaGroupName: "Test CLA Group", FoundationSFID: "foundation-sfid", ProjectSFID: "foundation-sfid"},
	}, nil)
	repoErr := errors.New("dynamodb failure")
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, "cla-group-id", aws.String("company-id-1"), true, nil).Return(int64(0), repoErr)

	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

	assert.Nil(t, result)
	assert.Equal(t, repoErr, err)
}

func TestNewerSignature(t *testing.T) {
	older := &v1Models.Signature{SignatureID: "signature-id-b", SignedOn: "2020-01-01T00:00:00Z", SignatureCreated: "2020-01-01T00:00:00Z"}
	assert.True(t, newerSignature(&v1Models.Signature{SignedOn: "2020-01-01T00:01:30Z"}, older))
	assert.False(t, newerSignature(&v1Models.Signature{SignedOn: "2019-12-31T23:59:59Z"}, older))
	assert.True(t, newerSignature(&v1Models.Signature{SignedOn: "2020-01-01T00:00:00Z", SignatureCreated: "2020-01-01T00:00:01Z"}, older))
	assert.True(t, newerSignature(&v1Models.Signature{SignedOn: "2020-01-01T00:00:00Z", SignatureCreated: "2020-01-01T00:00:00Z", SignatureID: "signature-id-c"}, older))
	assert.False(t, newerSignature(&v1Models.Signature{SignedOn: "2020-01-01T00:00:00Z", SignatureCreated: "2020-01-01T00:00:00Z", SignatureID: "signature-id-a"}, older))
}

// inFlightGauge counts the repository lookups running at the same time and remembers the peak
type inFlightGauge struct {
	current atomic.Int32
	peak    atomic.Int32
}

func (g *inFlightGauge) enter() {
	now := g.current.Add(1)
	for {
		peak := g.peak.Load()
		if now <= peak || g.peak.CompareAndSwap(peak, now) {
			return
		}
	}
}

func (g *inFlightGauge) leave() { g.current.Add(-1) }

// slow runs one mocked lookup under the gauge, holding its slot long enough for siblings to overlap
func (g *inFlightGauge) slow(fn func()) {
	g.enter()
	defer g.leave()
	time.Sleep(5 * time.Millisecond)
	fn()
}

func TestGetCompanyClaGroupsFansOutRowLookups(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// two signing entities, 12 CLA groups, 8 of them signed by both: 20 rows, 12 distinct mappings
	companySFID := "0014100000Te0000AAL"
	const groupCount = 12
	const firstCompanyID, secondCompanyID = "company-id-1", "company-id-2"
	claGroupIDs := make([]string, 0, groupCount)
	for i := 1; i <= groupCount; i++ {
		claGroupIDs = append(claGroupIDs, fmt.Sprintf("cla-group-%02d", i))
	}
	// Distinct per-row fixtures: the stored signing date differs per signature and the approved
	// contributors count per (CLA group, company), so a row showing a sibling's result is caught
	storedSignedOn := make(map[string]string)
	wantSignedOn := make(map[string]string)
	firstSigs := make([]*v1Models.Signature, 0, groupCount)
	secondSigs := make([]*v1Models.Signature, 0, groupCount)
	for i, claGroupID := range claGroupIDs {
		firstID := "sig-first-" + claGroupID
		storedSignedOn[firstID] = fmt.Sprintf("2026-08-%02dT07:46:07.000000+0000", i+1)
		wantSignedOn[firstID] = fmt.Sprintf("2026-08-%02dT07:46:07Z", i+1)
		firstSigs = append(firstSigs, &v1Models.Signature{SignatureID: firstID, ProjectID: claGroupID, SignatureSigned: true})
		if i >= 4 {
			secondID := "sig-second-" + claGroupID
			storedSignedOn[secondID] = fmt.Sprintf("2026-07-%02dT18:%02d:00.000000+0000", i+1, i)
			wantSignedOn[secondID] = fmt.Sprintf("2026-07-%02dT18:%02d:00Z", i+1, i)
			secondSigs = append(secondSigs, &v1Models.Signature{SignatureID: secondID, ProjectID: claGroupID, SignatureSigned: true, SignatureACL: []v1Models.User{{UserID: "user-id-1", LfUsername: "manager"}}})
		}
	}
	// 100 × the CLA group number, plus one for the second signing entity (-1 marks an unexpected id;
	// the mock runs on a worker goroutine, so it must not call t.Fatal)
	wantCount := func(claGroupID, companyID string) int64 {
		number, convErr := strconv.Atoi(strings.TrimPrefix(claGroupID, "cla-group-"))
		if convErr != nil {
			return -1
		}
		count := int64(number) * 100
		if companyID == secondCompanyID {
			count++
		}
		return count
	}
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{CompanyID: firstCompanyID, CompanyExternalID: companySFID, CompanyName: "Acme"},
		{CompanyID: secondCompanyID, CompanyExternalID: companySFID, CompanyName: "Acme", SigningEntityName: "Acme Sub"},
	}, nil)
	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams(firstCompanyID, nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{Signatures: firstSigs}, nil)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams(secondCompanyID, nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{Signatures: secondSigs}, nil)

	// one gauge per stage, so that a serialized stage cannot hide behind the other one's overlap
	var mappingGauge, rowGauge inFlightGauge
	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	for _, claGroupID := range claGroupIDs {
		// exactly one mapping lookup per CLA group, even when both signing entities signed it
		mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, claGroupID).Times(1).DoAndReturn(func(_ context.Context, id string) ([]*projects_cla_groups.ProjectClaGroup, error) {
			var pcgs []*projects_cla_groups.ProjectClaGroup
			mappingGauge.slow(func() {
				pcgs = []*projects_cla_groups.ProjectClaGroup{{ClaGroupID: id, ClaGroupName: "Group " + id, FoundationSFID: "foundation-sfid", FoundationName: "Test Foundation", ProjectSFID: "project-" + id, ProjectName: "Project " + id}}
			})
			return pcgs, nil
		})
	}
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, gomock.Any()).Times(len(firstSigs) + len(secondSigs)).DoAndReturn(func(_ context.Context, signatureID string) (*signatures.ItemSignature, error) {
		var item *signatures.ItemSignature
		rowGauge.slow(func() {
			item = &signatures.ItemSignature{SignatureID: signatureID, SignedOn: storedSignedOn[signatureID]}
		})
		return item, nil
	})
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, gomock.Any(), gomock.Any(), true, nil).Times(len(firstSigs) + len(secondSigs)).DoAndReturn(func(_ context.Context, claGroupID string, companyID *string, _ bool, _ *string) (int64, error) {
		var count int64
		rowGauge.slow(func() {
			count = wantCount(claGroupID, *companyID)
		})
		return count, nil
	})

	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

	assert.Nil(t, err)
	assert.Equal(t, int64(20), result.TotalCount)
	assert.Equal(t, int64(20), result.ResultCount)
	if !assert.Len(t, result.List, 20) {
		return
	}
	mappingPeak := mappingGauge.peak.Load()
	assert.GreaterOrEqual(t, mappingPeak, int32(2), "mapping lookups are expected to overlap")
	assert.LessOrEqual(t, mappingPeak, int32(companyClaGroupsConcurrency), "mapping lookups must stay within the concurrency cap")
	assert.Equal(t, int32(0), mappingGauge.current.Load(), "no mapping lookup may still be running after the call returns")
	rowPeak := rowGauge.peak.Load()
	assert.GreaterOrEqual(t, rowPeak, int32(2), "row lookups are expected to overlap")
	assert.LessOrEqual(t, rowPeak, int32(companyClaGroupsConcurrency), "row lookups must stay within the concurrency cap")
	assert.Equal(t, int32(0), rowGauge.current.Load(), "no row lookup may still be running after the call returns")

	// "Acme" rows first (all 12 groups, by name), then the 8 "Acme Sub" rows; every field comes
	// from the row's own lookups
	for i, row := range result.List {
		var wantEntity, wantCompanyID, wantSigPrefix string
		var wantGroup string
		var wantManagers int64
		if i < groupCount {
			wantEntity, wantCompanyID, wantSigPrefix, wantGroup = "Acme", firstCompanyID, "sig-first-", claGroupIDs[i]
		} else {
			wantEntity, wantCompanyID, wantSigPrefix, wantGroup, wantManagers = "Acme Sub", secondCompanyID, "sig-second-", claGroupIDs[i-groupCount+4], 1
		}
		wantSignatureID := wantSigPrefix + wantGroup
		assert.Equal(t, wantEntity, row.SigningEntityName, "row %d", i)
		assert.Equal(t, wantCompanyID, row.CompanyID, "row %d", i)
		assert.Equal(t, wantGroup, row.ClaGroupID, "row %d", i)
		assert.Equal(t, "Group "+wantGroup, row.ClaGroupName, "row %d", i)
		assert.Equal(t, wantSignatureID, row.SignatureID, "row %d", i)
		assert.Equal(t, []models.CompanyClaGroupProject{{ProjectSFID: "project-" + wantGroup, ProjectName: "Project " + wantGroup}}, row.Projects, "row %d", i)
		assert.Equal(t, wantSignedOn[wantSignatureID], row.SignedOn, "row %d", i)
		assert.Equal(t, wantCount(wantGroup, wantCompanyID), row.ApprovedContributorsCount, "row %d", i)
		assert.Equal(t, wantManagers, row.ClaManagersCount, "row %d", i)
		assert.Equal(t, wantManagers == 0, row.NeedsClaManager, "row %d", i)
	}
	// the fixtures themselves are distinct - otherwise the per-row assertions above prove nothing
	assert.Len(t, distinctValues(wantSignedOn), 20)
}

// distinctValues returns the set of values of a string map
func distinctValues(m map[string]string) map[string]bool {
	set := make(map[string]bool, len(m))
	for _, v := range m {
		set[v] = true
	}
	return set
}

func TestGetCompanyClaGroupsFanOutPropagatesFirstError(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	companySFID := "0014100000Te0000AAM"
	sigs := make([]*v1Models.Signature, 0, 6)
	for i := 1; i <= 6; i++ {
		sigs = append(sigs, &v1Models.Signature{SignatureID: fmt.Sprintf("signature-id-%d", i), ProjectID: fmt.Sprintf("cla-group-%d", i), SignatureSigned: true})
	}
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{CompanyID: "company-id-1", CompanyExternalID: companySFID, CompanyName: "Acme"},
	}, nil)
	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{Signatures: sigs}, nil)
	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, gomock.Any()).AnyTimes().DoAndReturn(func(_ context.Context, id string) ([]*projects_cla_groups.ProjectClaGroup, error) {
		return []*projects_cla_groups.ProjectClaGroup{{ClaGroupID: id, ClaGroupName: "Group " + id, FoundationSFID: "foundation-sfid", ProjectSFID: "foundation-sfid"}}, nil
	})
	// sibling rows may or may not reach their lookups before the failure is noticed
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, gomock.Any()).AnyTimes().Return(nil, nil)
	repoErr := errors.New("dynamodb failure")
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, gomock.Any(), aws.String("company-id-1"), true, nil).AnyTimes().DoAndReturn(func(_ context.Context, claGroupID string, _ *string, _ bool, _ *string) (int64, error) {
		if claGroupID == "cla-group-4" {
			return 0, repoErr
		}
		time.Sleep(2 * time.Millisecond)
		return 1, nil
	})

	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

	assert.Nil(t, result)
	assert.Equal(t, repoErr, err)
}

func TestGetCompanyClaGroupsMappingErrorPropagated(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	companySFID := "0014100000Te0000AAN"
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{CompanyID: "company-id-1", CompanyExternalID: companySFID, CompanyName: "Acme"},
	}, nil)
	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{
			{SignatureID: "signature-id-1", ProjectID: "cla-group-id", SignatureSigned: true},
			{SignatureID: "signature-id-2", ProjectID: "cla-group-id-2", SignatureSigned: true},
		},
	}, nil)
	repoErr := errors.New("dynamodb failure")
	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-id").AnyTimes().Return([]*projects_cla_groups.ProjectClaGroup{}, nil)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-id-2").Return(nil, repoErr)

	// the row lookups never start when a mapping lookup fails
	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

	assert.Nil(t, result)
	assert.Equal(t, repoErr, err)
}

func TestGetCompanyClaGroupsCancelledBeforeMappingsFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// the request is cancelled while the signatures load, before any mapping lookup started: the
	// skipped mappings must surface as an error, never as a successful list of blank rows
	companySFID := "0014100000Te0000AAP"
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{CompanyID: "company-id-1", CompanyExternalID: companySFID, CompanyName: "Acme"},
	}, nil)
	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).DoAndReturn(func(context.Context, v1SignatureParams.GetCompanySignaturesParams, int64, bool) (*v1Models.Signatures, error) {
		cancel()
		return &v1Models.Signatures{Signatures: []*v1Models.Signature{
			{SignatureID: "signature-id-1", ProjectID: "cla-group-id", SignatureSigned: true},
			{SignatureID: "signature-id-2", ProjectID: "cla-group-id-2", SignatureSigned: true},
		}}, nil
	})
	// no mapping or row lookup is expected - gomock fails the test on any such call
	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)

	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestClaGroupProjectMappingsCancelledFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// The mapping stage on its own: on a cancelled request it must fail rather than hand back
	// silently unresolved mappings. Through GetCompanyClaGroups the row stage would fail anyway, so
	// this guard is only observable here. No lookup is expected - gomock fails the test on any call.
	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	svc, ok := NewService(nil, mock_signature_repo.NewMockSignatureRepository(ctrl), mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mock_company_repo.NewMockIRepository(ctrl), mockProjectClaGroupRepo, nil).(*service)
	if !assert.True(t, ok) {
		return
	}
	mappings, err := svc.claGroupProjectMappings(ctx, []string{"cla-group-id", "cla-group-id-2"})

	assert.Nil(t, mappings)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestGetCompanyClaGroupsCancelledBeforeRowLookupsFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// two signing entities on one CLA group: the single mapping lookup succeeds but cancels the
	// request on its way out, so both row lookups are skipped and the call must fail
	companySFID := "0014100000Te0000AAQ"
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{CompanyID: "company-id-1", CompanyExternalID: companySFID, CompanyName: "Acme"},
		{CompanyID: "company-id-2", CompanyExternalID: companySFID, CompanyName: "Acme", SigningEntityName: "Acme Sub"},
	}, nil)
	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{{SignatureID: "signature-id-1", ProjectID: "cla-group-id", SignatureSigned: true}},
	}, nil)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-2", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{{SignatureID: "signature-id-2", ProjectID: "cla-group-id", SignatureSigned: true}},
	}, nil)
	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-id").Times(1).DoAndReturn(func(_ context.Context, id string) ([]*projects_cla_groups.ProjectClaGroup, error) {
		cancel()
		return []*projects_cla_groups.ProjectClaGroup{{ClaGroupID: id, ClaGroupName: "Test CLA Group", FoundationSFID: "foundation-sfid", ProjectSFID: "foundation-sfid"}}, nil
	})
	// no GetItemSignature / CountClaGroupCorporateContributors expectation: any row lookup fails the test

	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.Canceled)
}

// skippedRowsObserver is a logrus hook counting the rows GetCompanyClaGroups skipped after a
// cancellation; reached is closed once the expected number of rows was skipped
type skippedRowsObserver struct {
	skipped atomic.Int32
	reached chan struct{}
	target  int32
}

func (o *skippedRowsObserver) Levels() []logrus.Level { return []logrus.Level{logrus.DebugLevel} }

func (o *skippedRowsObserver) Fire(entry *logrus.Entry) error {
	if entry.Message == skippedRowLookupsMessage && o.skipped.Add(1) == o.target {
		close(o.reached)
	}
	return nil
}

// observeSkippedRows installs the observer on the package logger for the duration of a test
func observeSkippedRows(t *testing.T, target int32) *skippedRowsObserver {
	observer := &skippedRowsObserver{reached: make(chan struct{}), target: target}
	logger := log.GetLogger()
	previousLevel := logger.GetLevel()
	previousHooks := logger.ReplaceHooks(make(logrus.LevelHooks))
	logger.SetLevel(logrus.DebugLevel)
	logger.AddHook(observer)
	t.Cleanup(func() {
		logger.ReplaceHooks(previousHooks)
		logger.SetLevel(previousLevel)
	})
	return observer
}

func TestGetCompanyClaGroupsQueuedRowsSkippedAfterFirstError(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// More rows than the concurrency cap. The first row to reach its contributors count fails at
	// once; every other started row holds its slot until the test releases it, so the failed row's
	// slot is the only one that frees up in the meantime: each queued row is therefore submitted
	// after the failure and has to be skipped without a single repository call. The test releases
	// the held rows only once the queued rows were seen being skipped, the skipped rows'
	// cancellation must not replace the real error, and no partial list may come back.
	const rowCount = companyClaGroupsConcurrency + 4
	const queuedRows = rowCount - companyClaGroupsConcurrency
	observer := observeSkippedRows(t, queuedRows)
	companySFID := "0014100000Te0000AAR"
	sigs := make([]*v1Models.Signature, 0, rowCount)
	for i := 0; i < rowCount; i++ {
		sigs = append(sigs, &v1Models.Signature{SignatureID: fmt.Sprintf("signature-id-%02d", i), ProjectID: fmt.Sprintf("cla-group-%02d", i), SignatureSigned: true})
	}
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).Return([]*v1Models.Company{
		{CompanyID: "company-id-1", CompanyExternalID: companySFID, CompanyName: "Acme"},
	}, nil)
	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{Signatures: sigs}, nil)
	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, gomock.Any()).Times(rowCount).DoAndReturn(func(_ context.Context, id string) ([]*projects_cla_groups.ProjectClaGroup, error) {
		return []*projects_cla_groups.ProjectClaGroup{{ClaGroupID: id, ClaGroupName: "Group " + id, FoundationSFID: "foundation-sfid", ProjectSFID: "foundation-sfid"}}, nil
	})

	var gauge inFlightGauge
	var itemLookups, countLookups atomic.Int32
	release := make(chan struct{})
	repoErr := errors.New("dynamodb failure")
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, gomock.Any()).AnyTimes().DoAndReturn(func(_ context.Context, signatureID string) (*signatures.ItemSignature, error) {
		gauge.enter()
		defer gauge.leave()
		itemLookups.Add(1)
		return &signatures.ItemSignature{SignatureID: signatureID}, nil
	})
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, gomock.Any(), aws.String("company-id-1"), true, nil).AnyTimes().DoAndReturn(func(context.Context, string, *string, bool, *string) (int64, error) {
		gauge.enter()
		defer gauge.leave()
		if countLookups.Add(1) == 1 {
			return 0, repoErr
		}
		<-release
		return 1, nil
	})

	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	var result *models.CompanyClaGroups
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		result, err = service.GetCompanyClaGroups(ctx, companySFID, nil, nil)
	}()
	skippedInTime := true
	select {
	case <-observer.reached:
	case <-time.After(10 * time.Second):
		skippedInTime = false
	}
	close(release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("GetCompanyClaGroups did not return within 10s of the release")
	}
	if !assert.True(t, skippedInTime, "the %d queued rows were not skipped within 10s of the failure", queuedRows) {
		return
	}

	assert.Nil(t, result)
	assert.Equal(t, repoErr, err, "the first repository error must win over the cancellation of the skipped rows")
	assert.Equal(t, int32(0), gauge.current.Load(), "no row lookup may still be running after the call returns")
	// every row either ran both of its lookups (it held a slot before the failure) or was skipped
	started := itemLookups.Load()
	assert.GreaterOrEqual(t, started, int32(1))
	assert.LessOrEqual(t, started, int32(companyClaGroupsConcurrency), "queued rows must be skipped after the first failure")
	assert.Equal(t, started, countLookups.Load())
	assert.Equal(t, int32(rowCount), started+observer.skipped.Load())
}

func TestGetCompanyClaGroupsOrderIsStableForDuplicateSigningEntities(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// two company records resolve to the same signing entity name and signed the same CLA group:
	// their relative order (and therefore the paging) must not depend on lookup completion order
	companySFID := "0014100000Te0000AAO"
	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, companySFID, true).AnyTimes().Return([]*v1Models.Company{
		{CompanyID: "company-id-1", CompanyExternalID: companySFID, CompanyName: "Acme"},
		{CompanyID: "company-id-2", CompanyExternalID: companySFID, CompanyName: "Acme Legacy", SigningEntityName: "Acme"},
	}, nil)
	mockSignatureRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-1", nil), HugePageSize, signatures.LoadACLDetails).AnyTimes().Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{{SignatureID: "signature-id-b", ProjectID: "cla-group-id", SignatureSigned: true}},
	}, nil)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-2", nil), HugePageSize, signatures.LoadACLDetails).AnyTimes().Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{{SignatureID: "signature-id-a", ProjectID: "cla-group-id", SignatureSigned: true}},
	}, nil)
	mockSignatureRepo.EXPECT().GetItemSignature(ctx, gomock.Any()).AnyTimes().Return(nil, nil)
	mockSignatureRepo.EXPECT().CountClaGroupCorporateContributors(ctx, "cla-group-id", gomock.Any(), true, nil).AnyTimes().Return(int64(0), nil)
	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-id").AnyTimes().Return([]*projects_cla_groups.ProjectClaGroup{
		{ClaGroupID: "cla-group-id", ClaGroupName: "Test CLA Group", FoundationSFID: "foundation-sfid", ProjectSFID: "foundation-sfid"},
	}, nil)

	service := NewService(nil, mockSignatureRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	for attempt := 0; attempt < 25; attempt++ {
		result, err := service.GetCompanyClaGroups(ctx, companySFID, nil, nil)
		assert.Nil(t, err)
		if assert.Len(t, result.List, 2, "attempt %d", attempt) {
			assert.Equal(t, "signature-id-a", result.List[0].SignatureID, "attempt %d", attempt)
			assert.Equal(t, "company-id-2", result.List[0].CompanyID, "attempt %d", attempt)
			assert.Equal(t, "signature-id-b", result.List[1].SignatureID, "attempt %d", attempt)
		}
		page, pageErr := service.GetCompanyClaGroups(ctx, companySFID, aws.Int64(1), aws.Int64(1))
		assert.Nil(t, pageErr)
		if assert.Len(t, page.List, 1, "attempt %d", attempt) {
			assert.Equal(t, "signature-id-b", page.List[0].SignatureID, "second page attempt %d", attempt)
		}
	}
}

func TestCompanyClaGroupsJSONContract(t *testing.T) {
	b, err := json.Marshal(models.CompanyClaGroup{})
	assert.Nil(t, err)
	for _, key := range []string{"companyID", "companySFID", "companyName", "signingEntityName", "claGroupID", "claGroupName", "foundationSFID", "foundationName", "projects", "signed", "signatureID", "sanctioned", "approvedContributorsCount", "approvalCriteriaCount", "claManagersCount", "claManagers", "needsClaManager", "autoCreateECLA"} {
		assert.Contains(t, string(b), fmt.Sprintf("%q:", key))
	}
	assert.NotContains(t, string(b), `"signedBy"`)
	assert.NotContains(t, string(b), `"signedOn"`, "no stored signing date - no signedOn")
	assert.NotContains(t, string(b), `"sanctionedAt"`)

	dated, err := json.Marshal(models.CompanyClaGroup{SignedOn: "2023-01-02T03:04:05Z", SanctionedAt: "2026-08-20T10:11:12Z"})
	assert.Nil(t, err)
	assert.Contains(t, string(dated), `"signedOn":"2023-01-02T03:04:05Z"`)
	assert.Contains(t, string(dated), `"sanctionedAt":"2026-08-20T10:11:12Z"`)

	named, err := json.Marshal(models.CompanyClaGroup{SignedBy: "Alex Signer"})
	assert.Nil(t, err)
	assert.Contains(t, string(named), `"signedBy":"Alex Signer"`)

	lb, err := json.Marshal(models.CompanyClaGroups{List: make([]models.CompanyClaGroup, 0)})
	assert.Nil(t, err)
	for _, key := range []string{"companySFID", "resultCount", "totalCount", "list"} {
		assert.Contains(t, string(lb), fmt.Sprintf("%q:", key))
	}
	assert.Contains(t, string(lb), `"list":[]`)
}
