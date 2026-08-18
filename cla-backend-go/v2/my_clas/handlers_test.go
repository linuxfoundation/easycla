// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	v2Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	myClasOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/my_clas"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolvedUserID stands for whatever record the service settled on. The handler never
// derives it, so its value carries no meaning beyond being echoed back.
const resolvedUserID = "user-1"

// stubService answers the binding call with whatever the test configures, so the handler's
// status and reason-code mapping can be exercised without a repository.
type stubService struct {
	result *v2Models.SigningIdentity
	err    error
	// gotGithubUsername records what the handler forwarded, so a handler that drops the
	// handle is caught here rather than surfacing later as an approval list that matches
	// nothing.
	gotGithubUsername string
}

func (s *stubService) GetMyClas(context.Context, string, bool, *Identity) (*v2Models.MyClaList, error) {
	return nil, nil
}

func (s *stubService) GetMyClaPdfURL(context.Context, string, bool, *Identity, string) (*v2Models.MyClaPdf, error) {
	return nil, nil
}

func (s *stubService) GetMyIdentities(context.Context, string) (*v2Models.MyIdentityList, error) {
	return nil, nil
}

func (s *stubService) BindSigningIdentity(_ context.Context, _ int64, githubUsername string) (*v2Models.SigningIdentity, error) {
	s.gotGithubUsername = githubUsername
	return s.result, s.err
}

func bindResponse(t *testing.T, svc Service, githubID *int64, username string) *httptest.ResponseRecorder {
	t.Helper()

	api := &operations.EasyclaAPI{}
	Configure(api, svc)
	require.NotNil(t, api.MyClasBindSigningIdentityHandler)

	params := myClasOps.BindSigningIdentityParams{
		HTTPRequest: httptest.NewRequest(http.MethodPost, "/v4/my-clas/signing-identity", nil),
		Body:        v2Models.SigningIdentityRequest{GithubID: githubID, GithubUsername: chosenGithubUsername},
	}

	responder := api.MyClasBindSigningIdentityHandler.Handle(params, &auth.User{UserName: username})

	recorder := httptest.NewRecorder()
	responder.WriteResponse(recorder, runtime.JSONProducer())
	return recorder
}

// bindResponseWithTokenIdentity issues the same request with a verified caller on the
// context, so the two identities the handler sees can be made to disagree.
func bindResponseWithTokenIdentity(t *testing.T, svc Service, githubID *int64, gatewayUsername, tokenUsername string) *httptest.ResponseRecorder {
	t.Helper()

	api := &operations.EasyclaAPI{}
	Configure(api, svc)
	require.NotNil(t, api.MyClasBindSigningIdentityHandler)

	request := httptest.NewRequest(http.MethodPost, "/v4/my-clas/signing-identity", nil)
	request = request.WithContext(user.ContextWithVerifiedCaller(request.Context(), &user.CLAUser{
		LFUsername: tokenUsername,
	}))

	responder := api.MyClasBindSigningIdentityHandler.Handle(myClasOps.BindSigningIdentityParams{
		HTTPRequest: request,
		Body:        v2Models.SigningIdentityRequest{GithubID: githubID, GithubUsername: chosenGithubUsername},
	}, &auth.User{UserName: gatewayUsername})

	recorder := httptest.NewRecorder()
	responder.WriteResponse(recorder, runtime.JSONProducer())
	return recorder
}

func TestBindSigningIdentityHandler_DisagreeingIdentitiesAreRefused(t *testing.T) {
	githubID := chosenGithubID
	userID := resolvedUserID
	svc := &stubService{result: &v2Models.SigningIdentity{UserID: &userID, GithubID: &githubID, Outcome: outcomeMatched}}

	recorder := bindResponseWithTokenIdentity(t, svc, &githubID, callerLFUsername, "someone-else")

	// The write uses the token's identity, so proceeding here would record the submitted
	// account against whichever person the gateway did not authenticate. With the account
	// itself taken on trust, this identity is the only verified part of the write.
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), ReasonIdentityMismatch)
}

func TestBindSigningIdentityHandler_AgreeingIdentitiesProceed(t *testing.T) {
	githubID := chosenGithubID
	userID := resolvedUserID
	svc := &stubService{result: &v2Models.SigningIdentity{UserID: &userID, GithubID: &githubID, Outcome: outcomeMatched}}

	// Case differences come from different sources normalising differently, and are not a
	// disagreement about who is calling.
	recorder := bindResponseWithTokenIdentity(t, svc, &githubID, callerLFUsername, strings.ToUpper(callerLFUsername))

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestBindSigningIdentityHandler_RefusalStatusMapping(t *testing.T) {
	githubID := chosenGithubID

	// Each refusal gets its own status because the contributor's next step differs:
	// forbidden means the caller was not identified, conflict means the data needs a human
	// before anything can be signed.
	cases := []struct {
		reason string
		status int
	}{
		{ReasonIdentityUnavailable, http.StatusForbidden},
		{ReasonRecordConflict, http.StatusConflict},
		{ReasonRecordUnclaimed, http.StatusConflict},
		{ReasonDuplicateGithubID, http.StatusConflict},
		{ReasonLFRecordAlreadyBound, http.StatusConflict},
		{ReasonRecordedMismatch, http.StatusConflict},
	}

	for _, testCase := range cases {
		t.Run(testCase.reason, func(t *testing.T) {
			svc := &stubService{err: refuse(testCase.reason, "refused")}

			recorder := bindResponse(t, svc, &githubID, callerLFUsername)

			assert.Equal(t, testCase.status, recorder.Code)
			// The reason has to reach the caller, not just the log - the BFF routes on it.
			assert.Contains(t, recorder.Body.String(), testCase.reason)
		})
	}
}

func TestBindSigningIdentityHandler_MissingGithubIDIsABadRequest(t *testing.T) {
	recorder := bindResponse(t, &stubService{}, nil, callerLFUsername)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestBindSigningIdentityHandler_NoPrincipalIsUnauthorized(t *testing.T) {
	githubID := chosenGithubID

	recorder := bindResponse(t, &stubService{}, &githubID, "")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestBindSigningIdentityHandler_UnexpectedErrorIsAServerError(t *testing.T) {
	githubID := chosenGithubID
	svc := &stubService{err: errors.New("dynamodb unavailable")}

	recorder := bindResponse(t, svc, &githubID, callerLFUsername)

	// An infrastructure failure must not be dressed up as a refusal: a refusal means a
	// decision was reached, and counting one as the other corrupts the refusal counts.
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestBindSigningIdentityHandler_SuccessReturnsTheResolvedRecord(t *testing.T) {
	githubID := chosenGithubID
	userID := resolvedUserID
	svc := &stubService{result: &v2Models.SigningIdentity{
		UserID:   &userID,
		GithubID: &githubID,
		Outcome:  outcomeMatched,
	}}

	recorder := bindResponse(t, svc, &githubID, callerLFUsername)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), userID)
	// Echoed so the caller can check for itself that what was recorded is what was chosen.
	assert.Contains(t, recorder.Body.String(), "87654321")
}

// The handle is the only field on this request the service cannot derive for itself, and
// dropping it costs nothing visible at signing time - it surfaces much later, as an
// approval list written against a handle that matches no record.
func TestBindSigningIdentityHandler_ForwardsTheSubmittedHandle(t *testing.T) {
	githubID := chosenGithubID
	userID := resolvedUserID
	svc := &stubService{result: &v2Models.SigningIdentity{UserID: &userID, GithubID: &githubID, Outcome: outcomeMatched}}

	bindResponse(t, svc, &githubID, callerLFUsername)

	assert.Equal(t, chosenGithubUsername, svc.gotGithubUsername)
}
