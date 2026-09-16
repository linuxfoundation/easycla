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
