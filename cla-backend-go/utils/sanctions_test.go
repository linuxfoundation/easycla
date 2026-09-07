// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/stretchr/testify/assert"
)

func TestCheckCompanySanctioned(t *testing.T) {
	testCases := []struct {
		name    string
		company *v1Models.Company
		blocked bool
	}{
		{
			name:    "nil company",
			company: nil,
			blocked: false,
		},
		{
			name: "clean company",
			company: &v1Models.Company{
				CompanyID:         "internal-id",
				CompanyExternalID: "external-sfid",
				CompanyName:       "Clean Co",
				IsSanctioned:      false,
			},
			blocked: false,
		},
		{
			name: "sanctioned via SSS",
			company: &v1Models.Company{
				CompanyID:         "internal-id",
				CompanyExternalID: "external-sfid",
				CompanyName:       "Sanctioned Co",
				IsSanctioned:      true,
				SanctionOrigin:    "sss",
			},
			blocked: true,
		},
		{
			name: "sanctioned manually with empty origin",
			company: &v1Models.Company{
				CompanyID:         "internal-id",
				CompanyExternalID: "external-sfid",
				CompanyName:       "Manually Blocked Co",
				IsSanctioned:      true,
				SanctionOrigin:    "",
			},
			blocked: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sanctionedErr := CheckCompanySanctioned(tc.company)
			if !tc.blocked {
				assert.Nil(t, sanctionedErr)
				return
			}
			if assert.NotNil(t, sanctionedErr) {
				assert.Equal(t, tc.company.CompanyID, sanctionedErr.CompanyID)
				assert.Equal(t, tc.company.CompanyExternalID, sanctionedErr.CompanySFID)
				assert.Equal(t, tc.company.CompanyName, sanctionedErr.CompanyName)
				assert.Contains(t, sanctionedErr.Error(), tc.company.CompanyName)
				assert.Contains(t, sanctionedErr.Error(), "trade compliance")
			}
		})
	}
}

func TestRejectIfCompanySanctioned(t *testing.T) {
	ctx := context.WithValue(context.Background(), XREQUESTID, "req-123") // nolint

	assert.Nil(t, RejectIfCompanySanctioned(ctx, nil))
	assert.Nil(t, RejectIfCompanySanctioned(ctx, &v1Models.Company{CompanyID: "id", IsSanctioned: false}))
	assert.NotNil(t, RejectIfCompanySanctioned(ctx, &v1Models.Company{CompanyID: "id", IsSanctioned: true}))
}

func TestCompanySanctionedResponderContract(t *testing.T) {
	responder := RejectIfCompanySanctioned(
		context.WithValue(context.Background(), XREQUESTID, "req-123"), // nolint
		&v1Models.Company{
			CompanyID:         "0ca30016-6457-466c-bc41-a09560c1f9bf",
			CompanyExternalID: "0014100000Te0yqAAB",
			CompanyName:       "Sanctioned Co",
			IsSanctioned:      true,
		})
	if !assert.NotNil(t, responder) {
		return
	}

	recorder := httptest.NewRecorder()
	responder.WriteResponse(recorder, nil)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "req-123", recorder.Header().Get(XREQUESTID))

	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "company_sanctioned", body["code"])
	assert.Equal(t, "company Sanctioned Co requires further review for trade compliance", body["message"])
	assert.Equal(t, "0ca30016-6457-466c-bc41-a09560c1f9bf", body["company_id"])
	assert.Equal(t, "0014100000Te0yqAAB", body["company_sfid"])
	assert.Equal(t, "req-123", body["x-request-id"])
	for key := range body {
		assert.Contains(t, []string{"code", "message", "company_id", "company_sfid", "x-request-id"}, key)
	}
}

func TestCompanySanctionedResponderNoRequestID(t *testing.T) {
	responder := CompanySanctionedResponder("", &SanctionedCompanyError{
		CompanyID:   "internal-id",
		CompanySFID: "external-sfid",
		CompanyName: "Sanctioned Co",
	})

	recorder := httptest.NewRecorder()
	responder.WriteResponse(recorder, nil)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Empty(t, recorder.Header().Get(XREQUESTID))

	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	_, hasRequestID := body["x-request-id"]
	assert.False(t, hasRequestID)
}
