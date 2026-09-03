// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
package company

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	v1SignatureParams "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	v2Ops "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/company"

	mock_company_repo "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	mock_project_repo "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_pcg_repo "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	mock_signature_repo "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	mock_user_repo "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

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
				AutoCreateECLA:  true,
				SignatureACL: []v1Models.User{
					{UserID: "user-id-bob", LfUsername: "bob"},
					{UserID: "user-id-alice", LfUsername: "alice"},
				},
			},
		},
	}, nil)
	mockSignatureRepo.EXPECT().GetCompanySignatures(ctx, cclaSignaturesParams("company-id-2", nil), HugePageSize, signatures.LoadACLDetails).Return(&v1Models.Signatures{
		Signatures: []*v1Models.Signature{
			{
				SignatureID:      "signature-id-2",
				ProjectID:        "cla-group-id",
				SignatureSigned:  true,
				SignatureCreated: "2023-05-06T07:08:09Z",
			},
		},
	}, nil)
	mockSignatureRepo.EXPECT().GetClaGroupCorporateContributors(ctx, "cla-group-id", aws.String("company-id-1"), aws.Int64(1), nil, nil).Return(&v1Models.CorporateContributorList{TotalCount: 3}, nil)
	mockSignatureRepo.EXPECT().GetClaGroupCorporateContributors(ctx, "cla-group-id", aws.String("company-id-2"), aws.Int64(1), nil, nil).Return(&v1Models.CorporateContributorList{TotalCount: 0}, nil)

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
	result, err := service.GetCompanyClaGroups(ctx, companySFID)

	assert.Nil(t, err)
	assert.Equal(t, companySFID, result.CompanySFID)
	assert.Equal(t, int64(2), result.ResultCount)
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
	assert.Equal(t, "signature-id-1", first.SignatureID)
	assert.False(t, first.Sanctioned)
	assert.Equal(t, int64(3), first.ApprovedContributorsCount)
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
	assert.Equal(t, "2023-05-06T07:08:09Z", second.SignedOn)
	assert.True(t, second.Sanctioned)
	assert.Equal(t, int64(0), second.ApprovedContributorsCount)
	assert.Equal(t, int64(0), second.ClaManagersCount)
	assert.True(t, second.NeedsClaManager)
	assert.False(t, second.AutoCreateECLA)
}

func TestGetCompanyClaGroupsCompanyNotFound(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCompanyRepo := mock_company_repo.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompaniesByExternalID(ctx, "0014100000Te0000AAB", true).Return(nil, &utils.CompanyNotFound{CompanySFID: "0014100000Te0000AAB"})

	service := NewService(nil, mock_signature_repo.NewMockSignatureRepository(ctrl), mock_project_repo.NewMockProjectRepository(ctrl), mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mock_pcg_repo.NewMockRepository(ctrl), nil)
	result, err := service.GetCompanyClaGroups(ctx, "0014100000Te0000AAB")

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
	mockSignatureRepo.EXPECT().GetClaGroupCorporateContributors(ctx, "orphan-cla-group-id", aws.String("company-id-1"), aws.Int64(1), nil, nil).Return(&v1Models.CorporateContributorList{TotalCount: 0}, nil)

	mockProjectClaGroupRepo := mock_pcg_repo.NewMockRepository(ctrl)
	mockProjectClaGroupRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "orphan-cla-group-id").Return([]*projects_cla_groups.ProjectClaGroup{}, nil)

	mockProjectRepo := mock_project_repo.NewMockProjectRepository(ctrl)
	mockProjectRepo.EXPECT().GetCLAGroupByID(ctx, "orphan-cla-group-id", DontLoadRepoDetails).Return(&v1Models.ClaGroup{ProjectName: "Orphan Group", FoundationSFID: "orphan-foundation-sfid"}, nil)

	service := NewService(nil, mockSignatureRepo, mockProjectRepo, mock_user_repo.NewMockUserRepository(ctrl), mockCompanyRepo, mockProjectClaGroupRepo, nil)
	result, err := service.GetCompanyClaGroups(ctx, companySFID)

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
	mockSignatureRepo.EXPECT().GetClaGroupCorporateContributors(ctx, "cla-group-id", aws.String("company-id-1"), aws.Int64(1), nil, nil).Times(1).Return(&v1Models.CorporateContributorList{TotalCount: 1}, nil)
	mockSignatureRepo.EXPECT().GetClaGroupCorporateContributors(ctx, "cla-group-id-2", aws.String("company-id-1"), aws.Int64(1), nil, nil).Times(1).Return(&v1Models.CorporateContributorList{TotalCount: 2}, nil)

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
	result, err := service.GetCompanyClaGroups(ctx, companySFID)

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

func TestNewerSignature(t *testing.T) {
	older := &v1Models.Signature{SignatureID: "signature-id-b", SignedOn: "2020-01-01T00:00:00Z", SignatureCreated: "2020-01-01T00:00:00Z"}
	assert.True(t, newerSignature(&v1Models.Signature{SignedOn: "2020-01-01T00:01:30Z"}, older))
	assert.False(t, newerSignature(&v1Models.Signature{SignedOn: "2019-12-31T23:59:59Z"}, older))
	assert.True(t, newerSignature(&v1Models.Signature{SignedOn: "2020-01-01T00:00:00Z", SignatureCreated: "2020-01-01T00:00:01Z"}, older))
	assert.True(t, newerSignature(&v1Models.Signature{SignedOn: "2020-01-01T00:00:00Z", SignatureCreated: "2020-01-01T00:00:00Z", SignatureID: "signature-id-c"}, older))
	assert.False(t, newerSignature(&v1Models.Signature{SignedOn: "2020-01-01T00:00:00Z", SignatureCreated: "2020-01-01T00:00:00Z", SignatureID: "signature-id-a"}, older))
}

func TestCompanyClaGroupsJSONContract(t *testing.T) {
	b, err := json.Marshal(models.CompanyClaGroup{})
	assert.Nil(t, err)
	for _, key := range []string{"companyID", "companySFID", "companyName", "signingEntityName", "claGroupID", "claGroupName", "foundationSFID", "foundationName", "projects", "signed", "signedOn", "signatureID", "sanctioned", "approvedContributorsCount", "claManagersCount", "claManagers", "needsClaManager", "autoCreateECLA"} {
		assert.Contains(t, string(b), fmt.Sprintf("%q:", key))
	}

	lb, err := json.Marshal(models.CompanyClaGroups{List: make([]models.CompanyClaGroup, 0)})
	assert.Nil(t, err)
	for _, key := range []string{"companySFID", "resultCount", "list"} {
		assert.Contains(t, string(lb), fmt.Sprintf("%q:", key))
	}
	assert.Contains(t, string(lb), `"list":[]`)
}
