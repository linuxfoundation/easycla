// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	v2CompanyOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/company"
	v2UserServiceModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
)

func TestCreateCompanyHandlerMapsMemberServiceErrors(t *testing.T) {
	cases := map[string]struct {
		err  error
		want int
		code string
	}{
		"shared domain":                   {fmt.Errorf("%w: https://gmail.com", ErrCompanySharedDomain), http.StatusUnprocessableEntity, "company_shared_domain"},
		"ambiguous domain":                {fmt.Errorf("%w: https://acme.example.invalid", ErrCompanyAmbiguousDomain), http.StatusConflict, "company_ambiguous_domain"},
		"not configured":                  {ErrMemberServiceNotConfigured, http.StatusInternalServerError, "500"},
		"already exists":                  {fmt.Errorf("%w: 001410000MemberAQA", ErrCompanyExists), http.StatusConflict, ""},
		"not eligible":                    {fmt.Errorf("%w: member-service: person account", ErrCompanyNotEligible), http.StatusUnprocessableEntity, "company_not_eligible"},
		"unavailable":                     {fmt.Errorf("%w: member-service: POST returned 503: upstream body", ErrCompanyDependencyFailed), http.StatusInternalServerError, "500"},
		"exists with a website-like name": {fmt.Errorf("%w: Example website already exists Ltd", ErrCompanyExists), http.StatusConflict, ""},
		"unavailable with legacy words":   {fmt.Errorf("%w: member-service: POST returned 503: website already exists", ErrCompanyDependencyFailed), http.StatusInternalServerError, "500"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			service := &fakeCreateCompanyService{err: tc.err}
			Configure(api, service, nil, "")
			email := strfmt.Email("jane@example.invalid")
			cname, site := testCompanyName, testCompanySite
			responder := api.CompanyCreateCompanyHandler.Handle(v2CompanyOps.CreateCompanyParams{
				HTTPRequest: httptest.NewRequest(http.MethodPost, "/v4/user/u-jane/company", nil),
				UserID:      "u-jane",
				Input:       &models.CompanyInput{CompanyName: &cname, CompanyWebsite: &site, UserEmail: &email},
			}, &auth.User{UserName: "jane", Email: "jane@example.invalid", ACL: auth.ACL{Allowed: true}})
			recorder := httptest.NewRecorder()
			responder.WriteResponse(recorder, runtime.JSONProducer())
			assert.Equal(t, tc.want, recorder.Code, recorder.Body.String())
			var payload models.ErrorResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			assert.Equal(t, tc.code, payload.Code)
			if errors.Is(tc.err, ErrCompanyExists) {
				assert.Contains(t, payload.Message, "organization already exists", "same body as the legacy conflict")
				assert.NotContains(t, payload.Message, "website already exists", "typed conflicts are mapped before the legacy substring check")
				return
			}
			if errors.Is(tc.err, ErrCompanyNotEligible) || errors.Is(tc.err, ErrCompanyDependencyFailed) {
				assert.NotContains(t, payload.Message, "member-service", "no upstream detail in the body")
				assert.NotContains(t, payload.Message, "upstream body")
				assert.NotContains(t, payload.Message, "person account")
				if errors.Is(tc.err, ErrCompanyNotEligible) {
					assert.Equal(t, ErrCompanyNotEligible.Error(), payload.Message)
				} else {
					assert.Contains(t, payload.Message, "temporarily unavailable")
				}
				return
			}
			assert.Contains(t, payload.Message, tc.err.Error())
		})
	}
}

func TestCreateCompanyHandlerDependencyFailuresAre500(t *testing.T) {
	previous := lfUserByEmail
	t.Cleanup(func() { lfUserByEmail = previous })
	lfUserByEmail = func(string) (*v2UserServiceModels.User, error) {
		return &v2UserServiceModels.User{Username: testCallerUsername}, nil
	}
	accountJSON := `{"uid":"` + msAccountSFID + `","name":"` + testCompanyName + `","website":"` + testCompanySite + `"}`
	call := func(t *testing.T, fx *memberCreateFixture) (int, models.ErrorResponse) {
		t.Helper()
		api := operations.NewEasyclaAPI(nil)
		Configure(api, fx.svc, nil, "")
		params := *memberCreateParams(testCompanySite, "")
		params.HTTPRequest = httptest.NewRequest(http.MethodPost, "/v4/user/u-jane/company", nil)
		responder := api.CompanyCreateCompanyHandler.Handle(params, &auth.User{UserName: testCallerUsername, Email: testCallerEmail, ACL: auth.ACL{Allowed: true}})
		recorder := httptest.NewRecorder()
		responder.WriteResponse(recorder, runtime.JSONProducer())
		var payload models.ErrorResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload), recorder.Body.String())
		return recorder.Code, payload
	}
	for name, tc := range map[string]struct {
		transport *memberCreateHTTP
		repo      memberCreateRepo
		leaked    string
	}{
		"member-service 503":                   {&memberCreateHTTP{memberStatus: http.StatusServiceUnavailable, memberBody: `{"message":"upstream body"}`}, memberCreateRepo{}, "upstream body"},
		"name lookup failure":                  {&memberCreateHTTP{memberStatus: http.StatusCreated, memberBody: accountJSON}, memberCreateRepo{nameFailure: errors.New("dynamodb unavailable")}, "dynamodb"},
		"scope failure after row":              {&memberCreateHTTP{memberStatus: http.StatusCreated, memberBody: accountJSON, orgStatus: http.StatusOK, scopeStatus: http.StatusInternalServerError}, memberCreateRepo{expectRow: true}, "scope failure"},
		"member-service 503 with legacy words": {&memberCreateHTTP{memberStatus: http.StatusServiceUnavailable, memberBody: `{"message":"website already exists"}`}, memberCreateRepo{}, "website already exists"},
	} {
		t.Run(name, func(t *testing.T) {
			fx := newMemberCreateFixture(t, tc.transport, tc.repo)
			code, payload := call(t, fx)
			assert.Equal(t, http.StatusInternalServerError, code)
			assert.Equal(t, "500", payload.Code)
			assert.Contains(t, payload.Message, "temporarily unavailable")
			assert.NotContains(t, payload.Message, tc.leaked, "no upstream or repository detail in the body")
			assert.NotContains(t, payload.Message, "member-service")
		})
	}

	t.Run("a plain legacy website conflict still answers the legacy 409", func(t *testing.T) {
		api := operations.NewEasyclaAPI(nil)
		Configure(api, &fakeCreateCompanyService{err: errors.New("company website already exists")}, nil, "")
		params := *memberCreateParams(testCompanySite, "")
		params.HTTPRequest = httptest.NewRequest(http.MethodPost, "/v4/user/u-jane/company", nil)
		responder := api.CompanyCreateCompanyHandler.Handle(params, &auth.User{UserName: testCallerUsername, Email: testCallerEmail, ACL: auth.ACL{Allowed: true}})
		recorder := httptest.NewRecorder()
		responder.WriteResponse(recorder, runtime.JSONProducer())
		assert.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
		assert.Contains(t, recorder.Body.String(), "website already exists")
	})

	t.Run("switch off keeps the legacy mapping", func(t *testing.T) {
		fx := newMemberCreateFixture(t, &memberCreateHTTP{}, memberCreateRepo{})
		fx.svc.createViaMemberService, fx.svc.members = false, nil
		code, _ := call(t, fx)
		assert.Equal(t, http.StatusBadRequest, code, "the Organization Service path still answers as today")
		assert.Equal(t, 1, fx.transport.count(msLookupPath))
	})
}
