// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	sigOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/signatures"
	mock_project_repo "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_projects_cla_groups "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCorporateContributorsService struct {
	ServiceInterface
	calls     int
	gotParams sigOps.ListClaGroupCorporateContributorsParams
	result    *models.CorporateContributorList
	err       error
}

func (f *fakeCorporateContributorsService) GetClaGroupCorporateContributors(_ context.Context, params sigOps.ListClaGroupCorporateContributorsParams) (*models.CorporateContributorList, error) {
	f.calls++
	f.gotParams = params
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func TestListCompanyClaGroupCorporateContributors(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	const (
		claGroupID  = "cla-group-cc-1"
		companySFID = "0014100000CCAliasAAA"
		companyID   = "company-cc-1"
		foundation  = "found-cc-sfid"
	)

	orgScopedUser := func() *auth.User {
		return &auth.User{
			UserName: "cla-manager-user",
			Email:    "cla-manager@example.com",
			ACL:      auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.Organization, ID: companySFID}}},
		}
	}

	serve := func(t *testing.T, api *operations.EasyclaAPI, authUser *auth.User, params sigOps.ListCompanyClaGroupCorporateContributorsParams) *httptest.ResponseRecorder {
		t.Helper()
		require.NotNil(t, api.SignaturesListCompanyClaGroupCorporateContributorsHandler)
		username, email, reqID := authUser.UserName, authUser.Email, testReqID
		params.HTTPRequest = httptest.NewRequest(http.MethodGet, "/v4/company/external/"+companySFID+"/cla-group/"+claGroupID+"/corporate-contributors", nil)
		params.XUSERNAME, params.XEMAIL, params.XREQUESTID = &username, &email, &reqID
		recorder := httptest.NewRecorder()
		api.SignaturesListCompanyClaGroupCorporateContributorsHandler.Handle(params, authUser).WriteResponse(recorder, runtime.JSONProducer())
		return recorder
	}

	t.Run("org-scoped caller gets the delegated list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockProjectRepo := mock_project_repo.NewMockProjectRepository(ctrl)
		mockProjectRepo.EXPECT().GetCLAGroupByID(gomock.Any(), claGroupID, false).Return(&v1Models.ClaGroup{ProjectID: claGroupID, ProjectCCLAEnabled: true}, nil)

		mockCompanyService := mock_company.NewMockIService(ctrl)
		mockCompanyService.EXPECT().GetCompanyByExternalID(gomock.Any(), companySFID).Return(&v1Models.Company{CompanyID: companyID, CompanyExternalID: companySFID}, nil)

		mockPcgRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
		mockPcgRepo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), claGroupID).Return([]*projects_cla_groups.ProjectClaGroup{{ClaGroupID: claGroupID, FoundationSFID: foundation}}, nil)

		searchTerm, pageSize, nextKey := "ali", int64(7), "next-key-1"
		v2Service := &fakeCorporateContributorsService{result: &models.CorporateContributorList{List: []*models.CorporateContributor{{LinuxFoundationID: "alice-lfid"}}}}

		api := operations.NewEasyclaAPI(nil)
		Configure(api, nil, mockProjectRepo, mockCompanyService, nil, nil, nil, v2Service, mockPcgRepo)

		recorder := serve(t, api, orgScopedUser(), sigOps.ListCompanyClaGroupCorporateContributorsParams{
			ClaGroupID: claGroupID, CompanySFID: companySFID,
			SearchTerm: &searchTerm, PageSize: &pageSize, NextKey: &nextKey,
		})

		assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Equal(t, 1, v2Service.calls)
		require.NotNil(t, v2Service.gotParams.CompanyID)
		assert.Equal(t, companyID, *v2Service.gotParams.CompanyID, "the SFID is resolved to the internal company ID before delegation")
		assert.Equal(t, claGroupID, v2Service.gotParams.ClaGroupID)
		require.NotNil(t, v2Service.gotParams.SearchTerm)
		assert.Equal(t, searchTerm, *v2Service.gotParams.SearchTerm)
		require.NotNil(t, v2Service.gotParams.PageSize)
		assert.Equal(t, pageSize, *v2Service.gotParams.PageSize)
		require.NotNil(t, v2Service.gotParams.NextKey)
		assert.Equal(t, nextKey, *v2Service.gotParams.NextKey)
		assert.Contains(t, recorder.Body.String(), "alice-lfid")
	})

	t.Run("caller scoped to another organization is forbidden", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockProjectRepo := mock_project_repo.NewMockProjectRepository(ctrl)
		mockProjectRepo.EXPECT().GetCLAGroupByID(gomock.Any(), claGroupID, false).Return(&v1Models.ClaGroup{ProjectID: claGroupID, ProjectCCLAEnabled: true}, nil)

		mockCompanyService := mock_company.NewMockIService(ctrl)
		mockCompanyService.EXPECT().GetCompanyByExternalID(gomock.Any(), companySFID).Return(&v1Models.Company{CompanyID: companyID, CompanyExternalID: companySFID}, nil)

		mockPcgRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
		mockPcgRepo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), claGroupID).Return([]*projects_cla_groups.ProjectClaGroup{{ClaGroupID: claGroupID, FoundationSFID: foundation}}, nil)
		// the scope fallback walks the foundation -> CLA group mapping before giving up
		mockPcgRepo.EXPECT().GetClaGroupIDForProject(gomock.Any(), foundation).Return(nil, errors.New("no mapping"))

		v2Service := &fakeCorporateContributorsService{}
		api := operations.NewEasyclaAPI(nil)
		Configure(api, nil, mockProjectRepo, mockCompanyService, nil, nil, nil, v2Service, mockPcgRepo)

		otherOrgUser := &auth.User{
			UserName: "other-manager",
			Email:    "other@example.com",
			ACL:      auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.Organization, ID: "some-other-org-sfid"}}},
		}
		recorder := serve(t, api, otherOrgUser, sigOps.ListCompanyClaGroupCorporateContributorsParams{ClaGroupID: claGroupID, CompanySFID: companySFID})

		assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
		assert.Equal(t, 0, v2Service.calls, "an unauthorized caller must never reach the service")
	})

	t.Run("unknown company SFID is not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockProjectRepo := mock_project_repo.NewMockProjectRepository(ctrl)
		mockProjectRepo.EXPECT().GetCLAGroupByID(gomock.Any(), claGroupID, false).Return(&v1Models.ClaGroup{ProjectID: claGroupID, ProjectCCLAEnabled: true}, nil)

		mockCompanyService := mock_company.NewMockIService(ctrl)
		mockCompanyService.EXPECT().GetCompanyByExternalID(gomock.Any(), companySFID).Return(nil, errors.New("company not found"))

		v2Service := &fakeCorporateContributorsService{}
		api := operations.NewEasyclaAPI(nil)
		Configure(api, nil, mockProjectRepo, mockCompanyService, nil, nil, nil, v2Service, mock_projects_cla_groups.NewMockRepository(ctrl))

		recorder := serve(t, api, orgScopedUser(), sigOps.ListCompanyClaGroupCorporateContributorsParams{ClaGroupID: claGroupID, CompanySFID: companySFID})

		assert.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
		assert.Equal(t, 0, v2Service.calls)
	})

	t.Run("CLA group without CCLA support is a bad request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockProjectRepo := mock_project_repo.NewMockProjectRepository(ctrl)
		mockProjectRepo.EXPECT().GetCLAGroupByID(gomock.Any(), claGroupID, false).Return(&v1Models.ClaGroup{ProjectID: claGroupID, ProjectCCLAEnabled: false}, nil)

		mockCompanyService := mock_company.NewMockIService(ctrl)
		mockCompanyService.EXPECT().GetCompanyByExternalID(gomock.Any(), companySFID).Return(&v1Models.Company{CompanyID: companyID, CompanyExternalID: companySFID}, nil)

		v2Service := &fakeCorporateContributorsService{}
		api := operations.NewEasyclaAPI(nil)
		Configure(api, nil, mockProjectRepo, mockCompanyService, nil, nil, nil, v2Service, mock_projects_cla_groups.NewMockRepository(ctrl))

		recorder := serve(t, api, orgScopedUser(), sigOps.ListCompanyClaGroupCorporateContributorsParams{ClaGroupID: claGroupID, CompanySFID: companySFID})

		assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		assert.Equal(t, 0, v2Service.calls)
	})
}
