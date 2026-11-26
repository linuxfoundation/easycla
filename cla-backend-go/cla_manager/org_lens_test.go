// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-openapi/runtime"
	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	claManagerOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/cla_manager"
	mock_project "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	orgLensTestBase       = "https://app.example.org"
	orgLensTestCompanyID  = "00000000-0000-4000-8000-00000000000c"
	orgLensTestSFID       = "001000000000000AAA"
	orgLensTestCLAGroupID = "00000000-0000-4000-8000-000000000001"
	orgLensTestFoundation = "a0P000000000001AAA"
)

func orgLensTestURL(companySFID, claGroupID string) string {
	return utils.OrgLensURL(orgLensTestBase, companySFID, claGroupID)
}

func TestGetCompanyCLAGroupOrgLens(t *testing.T) {
	company := &models.Company{CompanyID: orgLensTestCompanyID, CompanyExternalID: orgLensTestSFID, CompanyName: "Example Org"}
	claGroup := &models.ClaGroup{ProjectID: orgLensTestCLAGroupID, FoundationSFID: orgLensTestFoundation, ProjectName: "Example Group"}
	page := orgLensTestBase + "/org/" + orgLensTestSFID + "/easycla/" + orgLensTestCLAGroupID

	tests := []struct {
		name       string
		company    *models.Company
		companyErr error
		claGroup   *models.ClaGroup
		groupErr   error
		live       bool
		wantStatus int
		wantBody   string
	}{
		{
			name: "live Account gets the lens page", company: company, claGroup: claGroup, live: true, wantStatus: http.StatusOK,
			wantBody: `{"claGroupID":"` + orgLensTestCLAGroupID + `","companyID":"` + orgLensTestCompanyID + `","companySFID":"` + orgLensTestSFID + `","foundationSFID":"` + orgLensTestFoundation + `","orgLensURL":"` + page + `"}`,
		},
		{
			name: "Account that is not live keeps the Corporate Console", company: company, claGroup: claGroup, wantStatus: http.StatusOK,
			wantBody: `{"claGroupID":"` + orgLensTestCLAGroupID + `","companyID":"` + orgLensTestCompanyID + `","companySFID":"` + orgLensTestSFID + `","foundationSFID":"` + orgLensTestFoundation + `","orgLensURL":""}`,
		},
		{
			name:    "company without a Salesforce Account keeps the Corporate Console",
			company: &models.Company{CompanyID: orgLensTestCompanyID, CompanyExternalID: "lf0000000000000AAA"}, claGroup: claGroup, live: true, wantStatus: http.StatusOK,
			wantBody: `{"claGroupID":"` + orgLensTestCLAGroupID + `","companyID":"` + orgLensTestCompanyID + `","companySFID":"lf0000000000000AAA","foundationSFID":"` + orgLensTestFoundation + `","orgLensURL":""}`,
		},
		{
			name: "unknown company", companyErr: &utils.CompanyNotFound{CompanyID: orgLensTestCompanyID}, live: true, wantStatus: http.StatusNotFound,
			wantBody: `{"Code":"404","Message":"company does not exist  - company ID: ` + orgLensTestCompanyID + `"}`,
		},
		{
			name: "unknown CLA Group", company: company, groupErr: &utils.CLAGroupNotFound{CLAGroupID: orgLensTestCLAGroupID}, live: true, wantStatus: http.StatusNotFound,
			wantBody: `{"Code":"404","Message":"cla group ` + orgLensTestCLAGroupID + ` not found"}`,
		},
		{
			name: "company lookup fails", companyErr: errors.New("dynamodb is down"), live: true, wantStatus: http.StatusInternalServerError,
			wantBody: `{"Code":"500","Message":"dynamodb is down"}`,
		},
		{
			name: "CLA Group lookup fails", company: company, groupErr: errors.New("dynamodb is down"), live: true, wantStatus: http.StatusInternalServerError,
			wantBody: `{"Code":"500","Message":"dynamodb is down"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utils.SetOrgLensLiveCheck(func(string) bool { return tt.live })
			t.Cleanup(func() { utils.SetOrgLensLiveCheck(nil) })
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			companyService := mock_company.NewMockIService(ctrl)
			projectService := mock_project.NewMockService(ctrl)
			companyService.EXPECT().GetCompany(gomock.Any(), orgLensTestCompanyID).Return(tt.company, tt.companyErr)
			if tt.companyErr == nil {
				projectService.EXPECT().GetCLAGroupByID(gomock.Any(), orgLensTestCLAGroupID).Return(tt.claGroup, tt.groupErr)
			}

			api := operations.NewClaAPI(nil)
			configureOrgLens(api, companyService, projectService, orgLensTestURL)
			reqID := "req-lens"
			recorder := httptest.NewRecorder()
			api.ClaManagerGetCompanyCLAGroupOrgLensHandler.Handle(claManagerOps.GetCompanyCLAGroupOrgLensParams{
				HTTPRequest: httptest.NewRequest(http.MethodGet, "/v3/company/"+orgLensTestCompanyID+"/cla-group/"+orgLensTestCLAGroupID+"/org-lens", nil),
				XREQUESTID:  &reqID,
				CompanyID:   orgLensTestCompanyID,
				ClaGroupID:  orgLensTestCLAGroupID,
			}).WriteResponse(recorder, runtime.JSONProducer())

			assert.Equal(t, tt.wantStatus, recorder.Code, recorder.Body.String())
			assert.Equal(t, reqID, recorder.Header().Get("X-Request-Id"))
			assert.JSONEq(t, tt.wantBody, recorder.Body.String())
		})
	}
}

func TestCompanyCLAGroupOrgLensWithoutARecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	companyService := mock_company.NewMockIService(ctrl)
	projectService := mock_project.NewMockService(ctrl)

	companyService.EXPECT().GetCompany(gomock.Any(), orgLensTestCompanyID).Return(nil, nil)
	_, err := companyCLAGroupOrgLens(context.Background(), companyService, projectService, orgLensTestURL, orgLensTestCompanyID, orgLensTestCLAGroupID)
	var companyNotFound *utils.CompanyNotFound
	require.ErrorAs(t, err, &companyNotFound)
	assert.Equal(t, orgLensTestCompanyID, companyNotFound.CompanyID)

	companyService.EXPECT().GetCompany(gomock.Any(), orgLensTestCompanyID).Return(&models.Company{CompanyID: orgLensTestCompanyID}, nil)
	projectService.EXPECT().GetCLAGroupByID(gomock.Any(), orgLensTestCLAGroupID).Return(nil, nil)
	_, err = companyCLAGroupOrgLens(context.Background(), companyService, projectService, orgLensTestURL, orgLensTestCompanyID, orgLensTestCLAGroupID)
	var claGroupNotFound *utils.CLAGroupNotFound
	require.ErrorAs(t, err, &claGroupNotFound)
	assert.Equal(t, orgLensTestCLAGroupID, claGroupNotFound.CLAGroupID)
}
