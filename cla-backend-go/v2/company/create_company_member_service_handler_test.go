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
)

func TestCreateCompanyHandlerMapsMemberServiceErrors(t *testing.T) {
	cases := map[string]struct {
		err  error
		want int
		code string
	}{
		"shared domain":    {fmt.Errorf("%w: https://gmail.com", ErrCompanySharedDomain), http.StatusUnprocessableEntity, "company_shared_domain"},
		"ambiguous domain": {fmt.Errorf("%w: https://acme.example.invalid", ErrCompanyAmbiguousDomain), http.StatusConflict, "company_ambiguous_domain"},
		"not configured":   {ErrMemberServiceNotConfigured, http.StatusInternalServerError, "500"},
		"already exists":   {fmt.Errorf("%w: 001410000MemberAQA", ErrCompanyExists), http.StatusConflict, ""},
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
				return
			}
			assert.Contains(t, payload.Message, tc.err.Error())
		})
	}
}
