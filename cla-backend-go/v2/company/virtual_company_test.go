// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"errors"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	mock_project_repo "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_pcg_repo "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	mock_signature_repo "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	mock_user_repo "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	virtualSFID      = "0014100000Te0yqQAB"
	virtualCompanyID = "9b8e7d66-40a5-4cde-9f00-3e1d1a2b3c4d"
)

// the company repository mock carries no expectations: any read or write on it fails the test
func newVirtualCompanyService(t *testing.T, ctrl *gomock.Controller, v1CompanyService *mock_company.MockIService, companyRepo *mock_company.MockIRepository, sigRepo *mock_signature_repo.MockSignatureRepository, projectRepo *mock_project_repo.MockProjectRepository, pcgRepo *mock_pcg_repo.MockRepository) *service {
	t.Helper()
	svc, ok := NewService(v1CompanyService, sigRepo, projectRepo, mock_user_repo.NewMockUserRepository(ctrl), companyRepo, pcgRepo, nil).(*service)
	require.True(t, ok)
	return svc
}

func TestGetCompanyBySFIDResolvesWithoutWriting(t *testing.T) {
	transportErr := errors.New("organization service unavailable")
	for _, tc := range []struct {
		name     string
		resolved *v1Models.Company
		err      error
	}{
		{"virtual company when no row exists", company.VirtualCompany(virtualSFID, "Acme"), nil},
		{"persisted row wins", &v1Models.Company{CompanyID: virtualCompanyID, CompanyExternalID: virtualSFID, CompanyName: "Acme", IsSanctioned: true}, nil},
		{"unknown organization", nil, &utils.CompanyNotFound{CompanySFID: virtualSFID}},
		{"transport error propagates", nil, transportErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			v1Service := mock_company.NewMockIService(ctrl)
			v1Service.EXPECT().ResolveCompany(gomock.Any(), virtualSFID).Return(tc.resolved, tc.err)
			svc := newVirtualCompanyService(t, ctrl, v1Service, mock_company.NewMockIRepository(ctrl), mock_signature_repo.NewMockSignatureRepository(ctrl), mock_project_repo.NewMockProjectRepository(ctrl), mock_pcg_repo.NewMockRepository(ctrl))

			result, err := svc.GetCompanyBySFID(context.Background(), virtualSFID)

			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.resolved.CompanyID, result.CompanyID)
			assert.Equal(t, virtualSFID, result.CompanyExternalID)
			assert.Equal(t, "Acme", result.CompanyName)
			assert.Equal(t, tc.resolved.IsSanctioned, result.IsSanctioned)
		})
	}
}

func TestGetCompanyByIDAcceptsASalesforceID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	v1Service := mock_company.NewMockIService(ctrl)
	v1Service.EXPECT().ResolveCompany(gomock.Any(), virtualSFID).Return(company.VirtualCompany(virtualSFID, "Acme"), nil)
	svc := newVirtualCompanyService(t, ctrl, v1Service, mock_company.NewMockIRepository(ctrl), mock_signature_repo.NewMockSignatureRepository(ctrl), mock_project_repo.NewMockProjectRepository(ctrl), mock_pcg_repo.NewMockRepository(ctrl))

	result, err := svc.GetCompanyByID(context.Background(), virtualSFID)

	require.NoError(t, err)
	assert.Equal(t, virtualSFID, result.CompanyID)
	assert.Equal(t, virtualSFID, result.CompanyExternalID)
}

func TestGetCompanyCLAGroupManagersAcceptsASalesforceID(t *testing.T) {
	t.Run("virtual company has no managers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		v1Service := mock_company.NewMockIService(ctrl)
		v1Service.EXPECT().ResolveCompany(gomock.Any(), virtualSFID).Return(company.VirtualCompany(virtualSFID, "Acme"), nil)
		svc := newVirtualCompanyService(t, ctrl, v1Service, mock_company.NewMockIRepository(ctrl), mock_signature_repo.NewMockSignatureRepository(ctrl), mock_project_repo.NewMockProjectRepository(ctrl), mock_pcg_repo.NewMockRepository(ctrl))

		result, err := svc.GetCompanyCLAGroupManagers(context.Background(), virtualSFID, "cg-1")

		require.NoError(t, err)
		assert.Equal(t, &models.CompanyClaManagers{}, result)
	})
	t.Run("persisted row is queried by its company id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		v1Service := mock_company.NewMockIService(ctrl)
		v1Service.EXPECT().ResolveCompany(gomock.Any(), virtualSFID).Return(&v1Models.Company{CompanyID: virtualCompanyID, CompanyExternalID: virtualSFID}, nil)
		sigRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
		sigRepo.EXPECT().GetProjectCompanySignature(gomock.Any(), virtualCompanyID, "cg-1", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		svc := newVirtualCompanyService(t, ctrl, v1Service, mock_company.NewMockIRepository(ctrl), sigRepo, mock_project_repo.NewMockProjectRepository(ctrl), mock_pcg_repo.NewMockRepository(ctrl))

		result, err := svc.GetCompanyCLAGroupManagers(context.Background(), virtualSFID, "cg-1")

		require.NoError(t, err)
		assert.Equal(t, &models.CompanyClaManagers{}, result)
	})
}

func TestGetCompanyProjectCLAServesTheVirtualCompany(t *testing.T) {
	setupClaGroupsHTTP(t, map[string]string{
		"proj-ok": `{"ID":"proj-ok","Name":"Project OK","ProjectType":"Project"}`,
	})
	mapping := &projects_cla_groups.ProjectClaGroup{ClaGroupID: "cg-1", ProjectSFID: "proj-ok", FoundationSFID: "found-1"}
	persisted := &v1Models.Company{CompanyID: virtualCompanyID, CompanyExternalID: virtualSFID, CompanyName: "Acme", SigningEntityName: "Acme"}

	for _, tc := range []struct {
		name            string
		rows            []*v1Models.Company
		rowsErr         error
		companyID       *string
		wantEntityID    string
		wantResolveCall bool
	}{
		{"no row: virtual unsigned entry", nil, &utils.CompanyNotFound{CompanySFID: virtualSFID}, nil, virtualSFID, true},
		{"no row, filtered by the SFID itself", nil, &utils.CompanyNotFound{CompanySFID: virtualSFID}, utils.StringRef(virtualSFID), virtualSFID, true},
		{"persisted row, filtered by the SFID itself", []*v1Models.Company{persisted}, nil, utils.StringRef(virtualSFID), virtualCompanyID, false},
		{"persisted row, filtered by its company id", []*v1Models.Company{persisted}, nil, utils.StringRef(virtualCompanyID), virtualCompanyID, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			companyRepo := mock_company.NewMockIRepository(ctrl)
			companyRepo.EXPECT().GetCompaniesByExternalID(gomock.Any(), virtualSFID, false).Return(tc.rows, tc.rowsErr)
			v1Service := mock_company.NewMockIService(ctrl)
			if tc.wantResolveCall {
				v1Service.EXPECT().ResolveCompany(gomock.Any(), virtualSFID).Return(company.VirtualCompany(virtualSFID, "Acme"), nil)
			}
			pcgRepo := mock_pcg_repo.NewMockRepository(ctrl)
			pcgRepo.EXPECT().GetClaGroupIDForProject(gomock.Any(), "proj-ok").Return(mapping, nil).AnyTimes()
			pcgRepo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), "cg-1").Return([]*projects_cla_groups.ProjectClaGroup{mapping}, nil).AnyTimes()
			projectRepo := mock_project_repo.NewMockProjectRepository(ctrl)
			projectRepo.EXPECT().GetCLAGroupByID(gomock.Any(), "cg-1", DontLoadRepoDetails).Return(&v1Models.ClaGroup{ProjectID: "cg-1", ProjectName: "CLA Group One", ProjectCCLAEnabled: true}, nil).AnyTimes()
			sigRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
			sigRepo.EXPECT().GetCompanySignatures(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, params interface{}, _ int64, _ bool) (*v1Models.Signatures, error) {
					return &v1Models.Signatures{Signatures: []*v1Models.Signature{}}, nil
				})
			svc := newVirtualCompanyService(t, ctrl, v1Service, companyRepo, sigRepo, projectRepo, pcgRepo)

			result, err := svc.GetCompanyProjectCLA(context.Background(), &auth.User{UserName: "tester"}, virtualSFID, "proj-ok", tc.companyID)

			require.NoError(t, err)
			require.Len(t, result.List, 1)
			assert.Empty(t, result.List[0].SignedClaList)
			require.Len(t, result.List[0].UnsignedProjectList, 1)
			unsigned := result.List[0].UnsignedProjectList[0]
			assert.Equal(t, tc.wantEntityID, unsigned.SigningEntityID)
			assert.Equal(t, "Acme", unsigned.CompanyName)
			assert.Equal(t, "cg-1", unsigned.ClaGroupID)
			assert.False(t, unsigned.CanSign)
		})
	}
}
