// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

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
	signOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/sign"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	handlerTestProjectSFID = "a09P000000DsCE9IAN"
	handlerTestCompanySFID = "0014100000Te0fZAAR"
)

type fakeSignService struct {
	Service
	err   error
	calls int
}

func (f *fakeSignService) RequestCorporateSignature(_ context.Context, _ string, _ string, _ *models.CorporateSignatureInput) (*models.CorporateSignatureOutput, error) {
	f.calls++
	return nil, f.err
}

func respondRequestCorporateSignature(t *testing.T, serviceErr error) (int, http.Header, string) {
	t.Helper()
	api := operations.NewEasyclaAPI(nil)
	service := &fakeSignService{err: serviceErr}
	Configure(api, service, nil)
	require.NotNil(t, api.SignRequestCorporateSignatureHandler)

	username := "cla-signatory-user"
	authUser := &auth.User{UserName: username, ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: handlerTestProjectSFID + "|" + handlerTestCompanySFID}}}}
	projectSFID, companySFID := handlerTestProjectSFID, handlerTestCompanySFID
	responder := api.SignRequestCorporateSignatureHandler.Handle(signOps.RequestCorporateSignatureParams{
		HTTPRequest:   httptest.NewRequest(http.MethodPost, "/v4/request-corporate-signature", nil),
		Authorization: "******",
		XUSERNAME:     &username,
		Input:         &models.CorporateSignatureInput{ProjectSfid: &projectSFID, CompanySfid: &companySFID},
	}, authUser)
	recorder := httptest.NewRecorder()
	responder.WriteResponse(recorder, runtime.JSONProducer())
	assert.Equal(t, 1, service.calls)
	return recorder.Code, recorder.Header(), recorder.Body.String()
}

func TestRequestCorporateSignatureHandlerSanctionedCompanyIsTyped(t *testing.T) {
	status, header, body := respondRequestCorporateSignature(t, &utils.SanctionedCompanyError{
		CompanyID:   "0ca30016-6457-466c-bc41-a09560c1f9bf",
		CompanySFID: handlerTestCompanySFID,
		CompanyName: "Sanctioned Co",
		Guidance:    utils.CompanySanctionedSigningGuidance,
	})

	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "application/json", header.Get("Content-Type"))

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &payload))
	assert.Equal(t, utils.CompanySanctionedCode, payload["code"])
	assert.Equal(t, "company Sanctioned Co requires further review for trade compliance\n"+utils.CompanySanctionedSigningGuidance, payload["message"])
	assert.Equal(t, "0ca30016-6457-466c-bc41-a09560c1f9bf", payload["company_id"])
	assert.Equal(t, handlerTestCompanySFID, payload["company_sfid"])
}

func TestRequestCorporateSignatureHandlerLegacySanctionMessageStillMapsTo403(t *testing.T) {
	status, _, body := respondRequestCorporateSignature(t, errors.New("company "+handlerTestCompanySFID+" requires further review for trade compliance"))

	assert.Equal(t, http.StatusForbidden, status)
	assert.Contains(t, body, "requires additional trade compliance review")
	assert.NotContains(t, body, utils.CompanySanctionedCode)
}

func TestRequestCorporateSignatureHandlerOtherErrorsAreBadRequests(t *testing.T) {
	status, _, body := respondRequestCorporateSignature(t, errors.New("docusign envelope rejected"))

	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, body, "docusign envelope rejected")
}

type fakeCclaCallbackService struct {
	Service
	err          error
	gotCompanyID string
	gotProjectID string
}

func (f *fakeCclaCallbackService) SignedCorporateCallback(_ context.Context, _ []byte, companyID, projectID string) error {
	f.gotCompanyID = companyID
	f.gotProjectID = projectID
	return f.err
}

func respondCclaCallback(t *testing.T, serviceErr error) *httptest.ResponseRecorder {
	t.Helper()
	api := operations.NewEasyclaAPI(nil)
	service := &fakeCclaCallbackService{err: serviceErr}
	Configure(api, service, nil)
	require.NotNil(t, api.SignCclaCallbackHandler)

	reqID := "req-ccla-callback"
	recorder := httptest.NewRecorder()
	api.SignCclaCallbackHandler.Handle(signOps.CclaCallbackParams{
		HTTPRequest: httptest.NewRequest(http.MethodPost, "/v4/signed/corporate/project-1/company-1", nil),
		XREQUESTID:  &reqID,
		ProjectID:   "project-1",
		CompanyID:   "company-1",
	}).WriteResponse(recorder, runtime.JSONProducer())
	assert.Equal(t, "company-1", service.gotCompanyID)
	assert.Equal(t, "project-1", service.gotProjectID)
	return recorder
}

func TestCclaCallbackHandlerSanctionedCompanyIsTyped(t *testing.T) {
	recorder := respondCclaCallback(t, &utils.SanctionedCompanyError{
		CompanyID:   "0ca30016-6457-466c-bc41-a09560c1f9bf",
		CompanySFID: handlerTestCompanySFID,
		CompanyName: "Sanctioned Co",
		Guidance:    utils.CompanySanctionedSigningGuidance,
	})

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "req-ccla-callback", recorder.Header().Get(utils.XREQUESTID))
	assert.JSONEq(t, `{"code":"company_sanctioned","message":"company Sanctioned Co requires further review for trade compliance\n`+utils.CompanySanctionedSigningGuidance+
		`","company_id":"0ca30016-6457-466c-bc41-a09560c1f9bf","company_sfid":"`+handlerTestCompanySFID+`","x-request-id":"req-ccla-callback"}`, recorder.Body.String())
}

func TestCclaCallbackHandlerOtherErrorsStayBareBadRequests(t *testing.T) {
	for _, serviceErr := range []error{
		errors.New("unable to lookup company by ID"),
		errors.New("company company-1 requires further review for trade compliance; corporate CLA cannot be finalized"),
	} {
		recorder := respondCclaCallback(t, serviceErr)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Empty(t, recorder.Body.String(), "the pre-existing 400 carries no body")
	}
}

func TestCclaCallbackHandlerSuccess(t *testing.T) {
	recorder := respondCclaCallback(t, nil)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, recorder.Body.String())
}
