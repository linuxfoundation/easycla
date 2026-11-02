// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package self_serve_sign

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
	claAuth "github.com/linuxfoundation/easycla/cla-backend-go/auth"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	selfServeSignOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/self_serve_sign"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/my_clas"
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

	prepareCaller *my_clas.Caller
	prepareEmail  string
	prepareInput  *models.PrepareSignInput
	prepareResult *models.PrepareSign
	prepareErr    error
	prepareCalls  int
}

func (f *fakeSelfServeSignService) PrepareSign(_ context.Context, caller *my_clas.Caller, currentEmail string, input *models.PrepareSignInput) (*models.PrepareSign, error) {
	f.prepareCalls++
	f.prepareCaller, f.prepareEmail, f.prepareInput = caller, currentEmail, input
	if f.prepareErr != nil {
		return nil, f.prepareErr
	}
	return f.prepareResult, nil
}

type fakeVerifier struct {
	enabled bool
	callers map[string]*claAuth.TrustedCaller
	seen    []string
}

func (f *fakeVerifier) Enabled() bool {
	return f.enabled
}

func (f *fakeVerifier) Verify(authorization string) (*claAuth.TrustedCaller, error) {
	f.seen = append(f.seen, authorization)
	if caller, ok := f.callers[authorization]; ok {
		return caller, nil
	}
	return nil, errors.New("unable to verify the bearer token")
}

func respondPrepareSign(t *testing.T, api *operations.EasyclaAPI, authUser *auth.User, authorization string) (int, string) {
	t.Helper()
	require.NotNil(t, api.SelfServeSignPrepareSignHandler)
	request := httptest.NewRequest(http.MethodPost, "/v4/self-serve/prepare-sign", nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	var username, email *string
	if authUser != nil {
		username, email = &authUser.UserName, &authUser.Email
	}
	responder := api.SelfServeSignPrepareSignHandler.Handle(selfServeSignOps.PrepareSignParams{
		HTTPRequest: request,
		XUSERNAME:   username,
		XEMAIL:      email,
		Body: models.PrepareSignInput{
			ClaGroupID:     stringRef(testCLAGroupID),
			GithubID:       12345,
			GithubUsername: "octocat",
			Email:          "octocat@example.org",
		},
	}, authUser)
	recorder := httptest.NewRecorder()
	responder.WriteResponse(recorder, runtime.JSONProducer())
	return recorder.Code, recorder.Body.String()
}

func TestPrepareSignHandlerCallerVerification(t *testing.T) {
	trusted := &claAuth.TrustedCaller{ClientID: "self-serve", Subject: "auth0|ss", Trusted: true}
	untrusted := &claAuth.TrustedCaller{ClientID: "some-other-app", Subject: "auth0|other", Trusted: false}
	regularUser := &auth.User{UserName: "lgryglicki", Email: "l@example.org", ACL: auth.ACL{Allowed: true}}
	adminUser := &auth.User{UserName: "lfadmin", Email: "admin@example.org", ACL: auth.ACL{Admin: true, Allowed: true}}
	anonymous := &auth.User{ACL: auth.ACL{Allowed: true}}

	testCases := []struct {
		name           string
		verifier       my_clas.CallerVerifier
		authUser       *auth.User
		authorization  string
		expectedStatus int
		expectedCalls  int
		expectedCaller *my_clas.Caller
		expectedText   string
	}{
		{
			name:           "no verifier configured",
			verifier:       nil,
			authUser:       regularUser,
			authorization:  "Bearer anything",
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
			expectedCaller: &my_clas.Caller{Username: "lgryglicki"},
		},
		{
			name:           "verifier disabled",
			verifier:       &fakeVerifier{enabled: false},
			authUser:       regularUser,
			authorization:  "Bearer anything",
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
			expectedCaller: &my_clas.Caller{Username: "lgryglicki"},
		},
		{
			name:           "verifier enabled and the token does not verify",
			verifier:       &fakeVerifier{enabled: true, callers: map[string]*claAuth.TrustedCaller{"Bearer good": trusted}},
			authUser:       regularUser,
			authorization:  "Bearer forged",
			expectedStatus: http.StatusUnauthorized,
			expectedCalls:  0,
			expectedText:   unverifiedCallerMsg,
		},
		{
			name:           "verifier enabled and no bearer token",
			verifier:       &fakeVerifier{enabled: true, callers: map[string]*claAuth.TrustedCaller{"Bearer good": trusted}},
			authUser:       regularUser,
			authorization:  "",
			expectedStatus: http.StatusUnauthorized,
			expectedCalls:  0,
			expectedText:   unverifiedCallerMsg,
		},
		{
			name:           "trusted self serve caller",
			verifier:       &fakeVerifier{enabled: true, callers: map[string]*claAuth.TrustedCaller{"Bearer good": trusted}},
			authUser:       regularUser,
			authorization:  "Bearer good",
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
			expectedCaller: &my_clas.Caller{Username: "lgryglicki", Trusted: true},
		},
		{
			name:           "trusted self serve caller without a username",
			verifier:       &fakeVerifier{enabled: true, callers: map[string]*claAuth.TrustedCaller{"Bearer good": trusted}},
			authUser:       anonymous,
			authorization:  "Bearer good",
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
			expectedCaller: &my_clas.Caller{Trusted: true},
		},
		{
			name:           "verified but not allow-listed caller behaves as before",
			verifier:       &fakeVerifier{enabled: true, callers: map[string]*claAuth.TrustedCaller{"Bearer other": untrusted}},
			authUser:       regularUser,
			authorization:  "Bearer other",
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
			expectedCaller: &my_clas.Caller{Username: "lgryglicki"},
		},
		{
			name:           "verified but not allow-listed caller without a username",
			verifier:       &fakeVerifier{enabled: true, callers: map[string]*claAuth.TrustedCaller{"Bearer other": untrusted}},
			authUser:       anonymous,
			authorization:  "Bearer other",
			expectedStatus: http.StatusUnauthorized,
			expectedCalls:  0,
			expectedText:   missingUsernameMsg,
		},
		{
			name:           "trusted admin keeps the admin flag",
			verifier:       &fakeVerifier{enabled: true, callers: map[string]*claAuth.TrustedCaller{"Bearer good": trusted}},
			authUser:       adminUser,
			authorization:  "Bearer good",
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
			expectedCaller: &my_clas.Caller{Username: "lfadmin", Admin: true, Trusted: true},
		},
		{
			name:           "untrusted anonymous principal",
			verifier:       nil,
			authUser:       anonymous,
			authorization:  "",
			expectedStatus: http.StatusUnauthorized,
			expectedCalls:  0,
			expectedText:   missingUsernameMsg,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			service := &fakeSelfServeSignService{prepareResult: &models.PrepareSign{
				ClaGroupID:  testCLAGroupID,
				LfUsername:  "lgryglicki",
				Identity:    []string{"githubId:12345", "githubUsername:octocat", "email:octocat@example.org"},
				IclaEnabled: true,
				CclaEnabled: true,
			}}
			Configure(api, service, tc.verifier)

			status, body := respondPrepareSign(t, api, tc.authUser, tc.authorization)

			assert.Equal(t, tc.expectedStatus, status)
			assert.Equal(t, tc.expectedCalls, service.prepareCalls)
			if tc.expectedText != "" {
				assert.Contains(t, body, tc.expectedText)
			}
			if tc.expectedCalls == 0 {
				return
			}
			assert.Equal(t, tc.expectedCaller, service.prepareCaller)
			assert.Equal(t, tc.authUser.Email, service.prepareEmail)
			require.NotNil(t, service.prepareInput)
			assert.Equal(t, int64(12345), service.prepareInput.GithubID)
			var payload map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(body), &payload))
			assert.Equal(t, testCLAGroupID, payload["claGroupId"])
			assert.Equal(t, "lgryglicki", payload["lfUsername"])
		})
	}
}

func TestPrepareSignHandlerErrorMapping(t *testing.T) {
	testCases := []struct {
		name           string
		serviceErr     error
		expectedStatus int
	}{
		{"cla group not found", ErrCLAGroupNotFound, http.StatusNotFound},
		{"identity not verified", ErrIdentityNotVerified, http.StatusForbidden},
		{"identity required", ErrIdentityRequired, http.StatusBadRequest},
		{"signing not enabled", ErrSigningNotEnabled, http.StatusBadRequest},
		{"return url not supported", ErrReturnURLNotSupported, http.StatusBadRequest},
		{"anything else", errors.New("dynamodb unavailable"), http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			service := &fakeSelfServeSignService{prepareErr: tc.serviceErr}
			Configure(api, service, nil)

			status, _ := respondPrepareSign(t, api, &auth.User{UserName: "lgryglicki", ACL: auth.ACL{Allowed: true}}, "")

			assert.Equal(t, tc.expectedStatus, status)
			assert.Equal(t, 1, service.prepareCalls)
		})
	}
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
	return respondCorporateSignatureInput(t, api, authUser, *corporateInput())
}

func respondCorporateSignatureInput(t *testing.T, api *operations.EasyclaAPI, authUser *auth.User, input models.SelfServeCorporateSignatureInput) (int, string) {
	t.Helper()
	require.NotNil(t, api.SelfServeSignSelfServeRequestCorporateSignatureHandler)
	responder := api.SelfServeSignSelfServeRequestCorporateSignatureHandler.Handle(selfServeSignOps.SelfServeRequestCorporateSignatureParams{
		HTTPRequest:   httptest.NewRequest(http.MethodPost, "/v4/self-serve/request-corporate-signature", nil),
		Authorization: "Bearer handler-token",
		XUSERNAME:     &authUser.UserName,
		Input:         input,
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
			name:           "another project",
			authUser:       &auth.User{UserName: "other-project-user", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "a09P000000DsCFAIA3|" + testCompanySFID}}}},
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
			Configure(api, service, nil)

			status, body := respondCorporateSignature(t, api, tc.authUser)

			assert.Equal(t, tc.expectedStatus, status)
			assert.Equal(t, tc.expectedCalls, service.calls)
			if tc.expectedStatus == http.StatusOK {
				assert.Equal(t, tc.authUser.UserName, service.lfUsername)
				assert.Equal(t, "Bearer handler-token", service.authorization)
				assert.True(t, service.input.AuthorityAcked)
				assert.True(t, service.input.EmbargoAcked)
				assert.Equal(t, testCLAGroupID, service.input.ClaGroupID)
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
		{"signatory missing", ErrSignatoryRequired, http.StatusBadRequest, "authority_name and authority_email"},
		{"selected group missing", v2Sign.ErrCLAGroupRequired, http.StatusBadRequest, "cla_group_id is required"},
		{"selected group mismatch", v2Sign.ErrCLAGroupMismatch, http.StatusBadRequest, "cla_group_id does not match"},
		{"selected group mismatch wrapped", fmt.Errorf("signing group changed: %w", v2Sign.ErrCLAGroupMismatch), http.StatusBadRequest, "cla_group_id does not match"},
		{"signing entity mismatch", ErrSigningEntityMismatch, http.StatusForbidden, "signing entity name does not belong to the provided company SFID"},
		{"company unknown", errors.New("company does not exist"), http.StatusNotFound, "company does not exist"},
		{"platform failure", errors.New("internal server error - docusign unavailable"), http.StatusInternalServerError, "internal server error"},
		{"sanctioned company (legacy message)", errors.New("company sanctioned-co requires further review for trade compliance"), http.StatusForbidden, "requires additional trade compliance review"},
		{"sanctioned company (typed)", &utils.SanctionedCompanyError{CompanyID: "company-1", CompanySFID: testCompanySFID, CompanyName: "Sanctioned Co", Guidance: utils.CompanySanctionedSigningGuidance}, http.StatusForbidden, `"code":"company_sanctioned"`},
		{"sanctioned company (typed, wrapped)", fmt.Errorf("unable to request the corporate signature: %w", &utils.SanctionedCompanyError{CompanyID: "company-1", CompanySFID: testCompanySFID, CompanyName: "Sanctioned Co"}), http.StatusForbidden, `"company_sfid":"` + testCompanySFID + `"`},
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
			Configure(api, service, nil)

			status, body := respondCorporateSignature(t, api, projectOrganizationUser("cla-signatory-user"))

			assert.Equal(t, tc.expectedStatus, status)
			assert.Equal(t, 1, service.calls)
			assert.Contains(t, body, tc.expectedText)
		})
	}
}

func TestSelfServeRequestCorporateSignatureHandlerRejectsEmailGroupBeforeDelegation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		claGroupID string
		message    string
	}{
		{"omitted", "", v2Sign.ErrCLAGroupRequired.Error()},
		{"blank", "  ", v2Sign.ErrCLAGroupRequired.Error()},
		{"different", "62db1b81-6f4a-4b2e-9a4a-0f2d9f0a1b22", v2Sign.ErrCLAGroupMismatch.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			corporateSign := &fakeCorporateSign{}
			companies, mappings := corporateFakes()
			Configure(api, newCorporateTestService(corporateSign, companies, mappings), nil)
			input := corporateInput()
			input.SendAsEmail = true
			input.AuthorityAcked = false
			input.EmbargoAcked = false
			input.AuthorityName = testAuthorityName
			input.AuthorityEmail = testAuthorityEmail
			input.ClaGroupID = tc.claGroupID

			status, body := respondCorporateSignatureInput(t, api, projectOrganizationUser("cla-signatory-user"), *input)

			assert.Equal(t, http.StatusBadRequest, status)
			assert.Contains(t, body, tc.message)
			assert.Zero(t, corporateSign.calls)
		})
	}
}

func TestSelfServeCorporateSignatureJSONContract(t *testing.T) {
	inputJSON, err := json.Marshal(&models.SelfServeCorporateSignatureInput{})
	require.NoError(t, err)
	for _, key := range []string{"project_sfid", "company_sfid", "authority_acked", "embargo_acked"} {
		assert.Contains(t, string(inputJSON), `"`+key+`"`)
	}
	inputJSON, err = json.Marshal(corporateInput())
	require.NoError(t, err)
	assert.Contains(t, string(inputJSON), `"cla_group_id":"`+testCLAGroupID+`"`)
	var input models.SelfServeCorporateSignatureInput
	require.NoError(t, json.Unmarshal(inputJSON, &input))
	assert.Equal(t, testCLAGroupID, input.ClaGroupID)

	outputJSON, err := json.Marshal(&models.SelfServeCorporateSignatureOutput{})
	require.NoError(t, err)
	for _, key := range []string{"signature_id", "sign_url", "cla_group_id", "project_sfid", "company_id", "company_sfid"} {
		assert.Contains(t, string(outputJSON), `"`+key+`"`)
	}
}
