// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package self_serve_sign

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
	selfServeSignOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/self_serve_sign"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
	v2Sign "github.com/linuxfoundation/easycla/cla-backend-go/v2/sign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSelfServeSignService struct {
	Service
	lfUsername    string
	authorization string
	input         *models.SelfServeCorporateSignatureInput
	result        *models.SelfServeCorporateSignatureOutput
	err           error
	calls         int
}

func (f *fakeSelfServeSignService) RequestCorporateSignature(_ context.Context, lfUsername, authorizationHeader string, input *models.SelfServeCorporateSignatureInput) (*models.SelfServeCorporateSignatureOutput, error) {
	f.calls++
	f.lfUsername, f.authorization, f.input = lfUsername, authorizationHeader, input
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func respondCorporateSignature(t *testing.T, api *operations.EasyclaAPI, authUser *auth.User) (int, string) {
	t.Helper()
	require.NotNil(t, api.SelfServeSignSelfServeRequestCorporateSignatureHandler)
	responder := api.SelfServeSignSelfServeRequestCorporateSignatureHandler.Handle(selfServeSignOps.SelfServeRequestCorporateSignatureParams{
		HTTPRequest:   httptest.NewRequest(http.MethodPost, "/v4/self-serve/request-corporate-signature", nil),
		Authorization: "Bearer handler-token",
		XUSERNAME:     &authUser.UserName,
		Input: models.SelfServeCorporateSignatureInput{
			ProjectSfid:    stringRef(testProjectSFID),
			CompanySfid:    stringRef(testCompanySFID),
			AuthorityAcked: true,
			EmbargoAcked:   true,
		},
	}, authUser)
	recorder := httptest.NewRecorder()
	responder.WriteResponse(recorder, runtime.JSONProducer())
	return recorder.Code, recorder.Body.String()
}

func projectOrganizationUser(username string) *auth.User {
	return &auth.User{UserName: username, ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: testProjectSFID + "|" + testCompanySFID}}}}
}

func TestSelfServeRequestCorporateSignatureHandlerAuth(t *testing.T) {
	testCases := []struct {
		name           string
		authUser       *auth.User
		expectedStatus int
		expectedCalls  int
	}{
		{
			name:           "project organization scope",
			authUser:       projectOrganizationUser("cla-signatory-user"),
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
		},
		{
			name:           "project organization tree scope",
			authUser:       &auth.User{UserName: "designee-user", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "a09P000000DsCFAIA3|" + testCompanySFID, Related: []string{testProjectSFID}}}}},
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
		},
		{
			name:           "admin scope disallowed",
			authUser:       &auth.User{UserName: "admin-user", ACL: auth.ACL{Admin: true, Allowed: true}},
			expectedStatus: http.StatusForbidden,
			expectedCalls:  0,
		},
		{
			name:           "another organization",
			authUser:       &auth.User{UserName: "other-user", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: testProjectSFID + "|0014100000Te0fYAAR"}}}},
			expectedStatus: http.StatusForbidden,
			expectedCalls:  0,
		},
		{
			name:           "organization scope only",
			authUser:       &auth.User{UserName: "org-user", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.Organization, ID: testCompanySFID}}}},
			expectedStatus: http.StatusForbidden,
			expectedCalls:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			service := &fakeSelfServeSignService{result: &models.SelfServeCorporateSignatureOutput{
				SignatureID: testSignatureID,
				SignURL:     testSignURL,
				ClaGroupID:  testCLAGroupID,
				ProjectSfid: testProjectSFID,
				CompanyID:   testCompanyID,
				CompanySfid: testCompanySFID,
			}}
			Configure(api, service)

			status, body := respondCorporateSignature(t, api, tc.authUser)

			assert.Equal(t, tc.expectedStatus, status)
			assert.Equal(t, tc.expectedCalls, service.calls)
			if tc.expectedStatus == http.StatusOK {
				assert.Equal(t, tc.authUser.UserName, service.lfUsername)
				assert.Equal(t, "Bearer handler-token", service.authorization)
				assert.True(t, service.input.AuthorityAcked)
				assert.True(t, service.input.EmbargoAcked)
				var payload map[string]interface{}
				require.NoError(t, json.Unmarshal([]byte(body), &payload))
				assert.Equal(t, testSignatureID, payload["signature_id"])
				assert.Equal(t, testSignURL, payload["sign_url"])
				assert.Equal(t, testCLAGroupID, payload["cla_group_id"])
				assert.Equal(t, testProjectSFID, payload["project_sfid"])
				assert.Equal(t, testCompanyID, payload["company_id"])
				assert.Equal(t, testCompanySFID, payload["company_sfid"])
			}
		})
	}
}

func TestSelfServeRequestCorporateSignatureHandlerErrorMapping(t *testing.T) {
	testCases := []struct {
		name           string
		serviceErr     error
		expectedStatus int
		expectedText   string
	}{
		{"attestations missing", ErrAttestationRequired, http.StatusBadRequest, "authority_acked and embargo_acked"},
		{"signing entity mismatch", ErrSigningEntityMismatch, http.StatusForbidden, "signing entity name does not belong to the provided company SFID"},
		{"company unknown", errors.New("company does not exist"), http.StatusNotFound, "company does not exist"},
		{"platform failure", errors.New("internal server error - docusign unavailable"), http.StatusInternalServerError, "internal server error"},
		{"sanctioned company", errors.New("company sanctioned-co requires further review for trade compliance"), http.StatusForbidden, "requires additional trade compliance review"},
		{"project not associated", projects_cla_groups.ErrProjectNotAssociatedWithClaGroup, http.StatusBadRequest, "not associated with cla_group"},
		{"ccla not enabled", v2Sign.ErrCCLANotEnabled, http.StatusBadRequest, "corporate license agreement is not enabled"},
		{"template not configured", v2Sign.ErrTemplateNotConfigured, http.StatusBadRequest, "cla template not configured"},
		{"signatory role scopes missing", &organizations.ListOrgUsrAdminScopesNotFound{}, http.StatusNotFound, "user role scopes not found for cla-signatory role"},
		{"signatory role scope conflict", &organizations.CreateOrgUsrRoleScopesConflict{}, http.StatusConflict, "user role scope conflict"},
		{"anything else", errors.New("docusign envelope rejected"), http.StatusBadRequest, "docusign envelope rejected"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			service := &fakeSelfServeSignService{err: tc.serviceErr}
			Configure(api, service)

			status, body := respondCorporateSignature(t, api, projectOrganizationUser("cla-signatory-user"))

			assert.Equal(t, tc.expectedStatus, status)
			assert.Equal(t, 1, service.calls)
			assert.Contains(t, body, tc.expectedText)
		})
	}
}

func TestSelfServeCorporateSignatureJSONContract(t *testing.T) {
	inputJSON, err := json.Marshal(&models.SelfServeCorporateSignatureInput{})
	require.NoError(t, err)
	for _, key := range []string{"project_sfid", "company_sfid", "authority_acked", "embargo_acked"} {
		assert.Contains(t, string(inputJSON), `"`+key+`"`)
	}

	outputJSON, err := json.Marshal(&models.SelfServeCorporateSignatureOutput{})
	require.NoError(t, err)
	for _, key := range []string{"signature_id", "sign_url", "cla_group_id", "project_sfid", "company_id", "company_sfid"} {
		assert.Contains(t, string(outputJSON), `"`+key+`"`)
	}
}
