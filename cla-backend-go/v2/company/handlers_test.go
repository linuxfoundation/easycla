// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	v2CompanyOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/company"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCompanyService struct {
	Service
	result *models.CompanyClaGroups
	err    error
	calls  int
}

func (f *fakeCompanyService) GetCompanyClaGroups(_ context.Context, companySFID string) (*models.CompanyClaGroups, error) {
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
	companySFID := "0014100000Te0000AAE"

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
