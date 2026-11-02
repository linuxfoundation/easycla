// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	v2CompanyOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/company"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCompanyName = "Acme Inc"
	testCompanySite = "https://acme.example.invalid"
)

type fakeCreateCompanyService struct {
	Service
	calls     int
	seen      string
	seenUser  string
	seenEmail string
	err       error
}

func (f *fakeCreateCompanyService) CreateCompany(_ context.Context, params *v2CompanyOps.CreateCompanyParams) (*models.CompanyOutput, error) {
	f.calls++
	f.seen = params.Input.UserEmail.String()
	if params.XUSERNAME != nil {
		f.seenUser = *params.XUSERNAME
	}
	if params.XEMAIL != nil {
		f.seenEmail = *params.XEMAIL
	}
	if f.err != nil {
		return nil, f.err
	}
	return &models.CompanyOutput{CompanyID: "0014100000Created", CompanyName: *params.Input.CompanyName}, nil
}

// the company can only be created by a signed-in user, whose token identity the service binds the request to (lfx-self-serve-ops#343)
func TestCreateCompanyHandlerBindsTheCaller(t *testing.T) {
	var lastService *fakeCreateCompanyService
	call := func(t *testing.T, authUser *auth.User, bodyEmail string, headerUser, headerEmail *string) (int, int) {
		api := operations.NewEasyclaAPI(nil)
		service := &fakeCreateCompanyService{}
		lastService = service
		Configure(api, service, nil, "")
		require.NotNil(t, api.CompanyCreateCompanyHandler)
		email := strfmt.Email(bodyEmail)
		name, site := testCompanyName, testCompanySite
		responder := api.CompanyCreateCompanyHandler.Handle(v2CompanyOps.CreateCompanyParams{
			HTTPRequest: httptest.NewRequest(http.MethodPost, "/v4/user/user-1/company", nil),
			UserID:      "user-1",
			XUSERNAME:   headerUser,
			XEMAIL:      headerEmail,
			Input:       &models.CompanyInput{CompanyName: &name, CompanyWebsite: &site, UserEmail: &email},
		}, authUser)
		recorder := httptest.NewRecorder()
		responder.WriteResponse(recorder, runtime.JSONProducer())
		return recorder.Code, service.calls
	}
	caller := func() *auth.User {
		return &auth.User{UserName: "jane", Email: "jane@example.invalid", ACL: auth.ACL{Allowed: true}}
	}
	status, calls := call(t, caller(), "jane.github@example.invalid", nil, nil)
	assert.Equal(t, http.StatusOK, status, "a body e-mail other than X-EMAIL is accepted (informational); company-admin goes to the token e-mail")
	assert.Equal(t, 1, calls)
	assert.Equal(t, "jane", lastService.seenUser, "the service receives the principal's username even without an X-USERNAME header")
	assert.Equal(t, "jane@example.invalid", lastService.seenEmail, "the service receives the principal's e-mail (the grant target) even without an X-EMAIL header")
	other, otherEmail := "mallory", "mallory@example.invalid"
	status, calls = call(t, caller(), "Jane@Example.invalid", &other, &otherEmail)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, 1, calls)
	assert.Equal(t, other, lastService.seenUser, "a present X-USERNAME is the gateway-asserted principal name (lfx-kit derives authUser.UserName from the same header) and is what the service receives")
	assert.Equal(t, otherEmail, lastService.seenEmail, "same for X-EMAIL")
	status, calls = call(t, &auth.User{UserName: "ghost", ACL: auth.ACL{Allowed: true}}, "ghost@example.invalid", nil, nil)
	assert.Equal(t, http.StatusOK, status, "a principal with a username but no e-mail is bound by username in the service")
	assert.Equal(t, 1, calls)
	status, calls = call(t, &auth.User{ACL: auth.ACL{Allowed: true}}, "nobody@example.invalid", nil, nil)
	assert.Equal(t, http.StatusForbidden, status, "a principal with neither username nor e-mail cannot be bound")
	assert.Equal(t, 0, calls)
	status, calls = call(t, nil, "nobody@example.invalid", nil, nil)
	assert.Equal(t, http.StatusForbidden, status, "a missing principal is refused, not dereferenced")
	assert.Equal(t, 0, calls)
}

// a user ID that is not the caller's own, or a caller without a verified e-mail, is a 403; a failed user lookup is a 500, never a 400 (lfx-self-serve-ops#343)
func TestCreateCompanyHandlerMapsServiceErrors(t *testing.T) {
	cases := map[string]struct {
		err  error
		want int
	}{
		"another user's ID":   {fmt.Errorf("%w: u-victim", ErrCompanyUserMismatch), http.StatusForbidden},
		"no grant target":     {ErrCompanyNoGrantTarget, http.StatusForbidden},
		"user lookup failure": {fmt.Errorf("%w: u-victim: dynamodb unavailable", ErrCompanyUserLookup), http.StatusInternalServerError},
		"other failure":       {fmt.Errorf("salesforce said no"), http.StatusBadRequest},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			service := &fakeCreateCompanyService{err: tc.err}
			Configure(api, service, nil, "")
			email := strfmt.Email("jane@example.invalid")
			cname, site := testCompanyName, testCompanySite
			responder := api.CompanyCreateCompanyHandler.Handle(v2CompanyOps.CreateCompanyParams{
				HTTPRequest: httptest.NewRequest(http.MethodPost, "/v4/user/u-victim/company", nil),
				UserID:      "u-victim",
				Input:       &models.CompanyInput{CompanyName: &cname, CompanyWebsite: &site, UserEmail: &email},
			}, &auth.User{UserName: "jane", Email: "jane@example.invalid", ACL: auth.ACL{Allowed: true}})
			recorder := httptest.NewRecorder()
			responder.WriteResponse(recorder, runtime.JSONProducer())
			assert.Equal(t, tc.want, recorder.Code)
			assert.Equal(t, 1, service.calls)
		})
	}
}
