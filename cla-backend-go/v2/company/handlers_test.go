// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	v2CompanyOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/company"
	v2ProjectServiceClient "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service/client/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCompanySFID      = "0014100000Te0000AAE"
	testOtherCompanySFID = "0014100000Te0000AAB"
)

type fakeCompanyService struct {
	Service
	result *models.CompanyClaGroups
	err    error
	calls  int

	company         *models.Company
	claManagersErr  error
	claManagerCalls int
}

func (f *fakeCompanyService) GetCompanyByID(_ context.Context, _ string) (*models.Company, error) {
	return f.company, nil
}

func (f *fakeCompanyService) GetCompanyProjectCLAManagers(_ context.Context, _ *models.Company, _ string) (*models.CompanyClaManagers, error) {
	f.claManagerCalls++
	if f.claManagersErr != nil {
		return nil, f.claManagersErr
	}
	return &models.CompanyClaManagers{List: make([]*models.CompanyClaManager, 0)}, nil
}

func (f *fakeCompanyService) GetCompanyClaGroups(_ context.Context, companySFID string, _, _ *int64) (*models.CompanyClaGroups, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &models.CompanyClaGroups{CompanySFID: companySFID, List: make([]models.CompanyClaGroup, 0)}, nil
}

func respond(t *testing.T, api *operations.EasyclaAPI, companySFID string, authUser *auth.User) (int, string) {
	t.Helper()
	require.NotNil(t, api.CompanyGetCompanyClaGroupsHandler)
	responder := api.CompanyGetCompanyClaGroupsHandler.Handle(v2CompanyOps.GetCompanyClaGroupsParams{
		HTTPRequest: httptest.NewRequest(http.MethodGet, "/v4/company/external/"+companySFID+"/cla-groups", nil),
		CompanySFID: companySFID,
	}, authUser)
	recorder := httptest.NewRecorder()
	responder.WriteResponse(recorder, runtime.JSONProducer())
	return recorder.Code, recorder.Body.String()
}

func TestGetCompanyClaGroupsHandler(t *testing.T) {
	companySFID := testCompanySFID

	testCases := []struct {
		name           string
		authUser       *auth.User
		serviceErr     error
		expectedStatus int
		expectedCalls  int
	}{
		{
			name:           "organization scope",
			authUser:       &auth.User{UserName: "org-user", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.Organization, ID: companySFID}}}},
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
		},
		{
			name:           "project organization scope matching organization",
			authUser:       &auth.User{UserName: "cla-manager", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "project-sfid|" + companySFID}}}},
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
		},
		{
			name:           "admin",
			authUser:       &auth.User{UserName: "admin-user", ACL: auth.ACL{Admin: true, Allowed: true}},
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
		},
		{
			name:           "no matching scope",
			authUser:       &auth.User{UserName: "other-user", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.Organization, ID: "0014100000Te0000AAB"}, {Type: auth.ProjectOrganization, ID: "project-sfid|0014100000Te0000AAB"}}}},
			expectedStatus: http.StatusForbidden,
			expectedCalls:  0,
		},
		{
			name:           "no scopes",
			authUser:       &auth.User{UserName: "scopeless-user", ACL: auth.ACL{Allowed: true}},
			expectedStatus: http.StatusForbidden,
			expectedCalls:  0,
		},
		{
			name:           "service failure",
			authUser:       &auth.User{UserName: "org-user", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.Organization, ID: companySFID}}}},
			serviceErr:     errors.New("dynamodb failure"),
			expectedStatus: http.StatusBadRequest,
			expectedCalls:  1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			service := &fakeCompanyService{err: tc.serviceErr}
			Configure(api, service, nil, "")

			status, body := respond(t, api, companySFID, tc.authUser)

			assert.Equal(t, tc.expectedStatus, status)
			assert.Equal(t, tc.expectedCalls, service.calls)
			if tc.expectedStatus == http.StatusOK {
				var payload models.CompanyClaGroups
				require.Nil(t, json.Unmarshal([]byte(body), &payload))
				assert.Equal(t, companySFID, payload.CompanySFID)
				assert.NotNil(t, payload.List)
			}
		})
	}
}

func respondProjectClaManagers(t *testing.T, api *operations.EasyclaAPI, companyID, projectSFID string, authUser *auth.User) int {
	t.Helper()
	require.NotNil(t, api.CompanyGetCompanyProjectClaManagersHandler)
	responder := api.CompanyGetCompanyProjectClaManagersHandler.Handle(v2CompanyOps.GetCompanyProjectClaManagersParams{
		HTTPRequest: httptest.NewRequest(http.MethodGet, "/v4/company/"+companyID+"/project/"+projectSFID+"/cla-managers", nil),
		CompanyID:   companyID,
		ProjectSFID: projectSFID,
	}, authUser)
	recorder := httptest.NewRecorder()
	responder.WriteResponse(recorder, runtime.JSONProducer())
	return recorder.Code
}

func TestGetCompanyProjectClaManagersHandler(t *testing.T) {
	companyID := "company-uuid-1"
	companySFID := testCompanySFID
	projectSFID := "project-sfid-1"
	orgUser := &auth.User{UserName: "org-user", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.Organization, ID: companySFID}}}}

	testCases := []struct {
		name           string
		authUser       *auth.User
		serviceErr     error
		expectedStatus int
		expectedCalls  int
	}{
		{
			name:           "success",
			authUser:       orgUser,
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
		},
		{
			name:           "project missing from the project service",
			authUser:       orgUser,
			serviceErr:     v2ProjectServiceClient.NewGetProjectNotFound(),
			expectedStatus: http.StatusNotFound,
			expectedCalls:  1,
		},
		{
			name:           "wrapped project not found",
			authUser:       orgUser,
			serviceErr:     fmt.Errorf("loading CLA groups: %w", v2ProjectServiceClient.NewGetProjectNotFound()),
			expectedStatus: http.StatusNotFound,
			expectedCalls:  1,
		},
		{
			name:           "any other service failure",
			authUser:       orgUser,
			serviceErr:     errors.New("dynamodb failure"),
			expectedStatus: http.StatusBadRequest,
			expectedCalls:  1,
		},
		{
			name:           "permission check runs before the project lookup",
			authUser:       &auth.User{UserName: "other-user", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.Organization, ID: testOtherCompanySFID}}}},
			serviceErr:     v2ProjectServiceClient.NewGetProjectNotFound(),
			expectedStatus: http.StatusForbidden,
			expectedCalls:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			service := &fakeCompanyService{
				company:        &models.Company{CompanyID: companyID, CompanyExternalID: companySFID},
				claManagersErr: tc.serviceErr,
			}
			Configure(api, service, nil, "")

			status := respondProjectClaManagers(t, api, companyID, projectSFID, tc.authUser)

			assert.Equal(t, tc.expectedStatus, status)
			assert.Equal(t, tc.expectedCalls, service.claManagerCalls)
		})
	}
}
