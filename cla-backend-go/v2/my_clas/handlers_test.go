// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	claAuth "github.com/linuxfoundation/easycla/cla-backend-go/auth"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	myClasOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/my_clas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeService struct {
	callers []*Caller
	err     error
	nilPdf  bool
}

func (f *fakeService) GetMyClas(_ context.Context, caller *Caller, _ *Identity) (*models.MyClaList, error) {
	f.callers = append(f.callers, caller)
	if f.err != nil {
		return nil, f.err
	}
	return &models.MyClaList{}, nil
}

func (f *fakeService) GetMyClaPdfURL(_ context.Context, caller *Caller, _ *Identity, _ string) (*models.MyClaPdf, error) {
	f.callers = append(f.callers, caller)
	if f.err != nil {
		return nil, f.err
	}
	if f.nilPdf {
		return nil, nil
	}
	return &models.MyClaPdf{}, nil
}

func (f *fakeService) GetMyIdentities(_ context.Context, currentUsername string) (*models.MyIdentityList, error) {
	f.callers = append(f.callers, &Caller{Username: currentUsername})
	if f.err != nil {
		return nil, f.err
	}
	return &models.MyIdentityList{}, nil
}

type fakeVerifier struct {
	enabled bool
	callers map[string]*claAuth.TrustedCaller
	seen    []string
	noop    string
}

func (f *fakeVerifier) Enabled() bool {
	return f.enabled
}

func (f *fakeVerifier) Verify(authorization string) (*claAuth.TrustedCaller, error) {
	f.seen = append(f.seen, authorization)
	if f.noop != "" && authorization == f.noop {
		return nil, nil
	}
	if caller, ok := f.callers[authorization]; ok {
		return caller, nil
	}
	return nil, errors.New("unable to verify the bearer token")
}

func configuredAPI(t *testing.T, verifier CallerVerifier) (*operations.EasyclaAPI, *fakeService) {
	t.Helper()
	api := operations.NewEasyclaAPI(nil)
	service := &fakeService{}
	Configure(api, service, verifier)
	return api, service
}

func request(t *testing.T, authorization string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v4/my-clas", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	return req
}

func statusOf(t *testing.T, responder interface {
	WriteResponse(http.ResponseWriter, runtime.Producer)
}) int {
	t.Helper()
	recorder := httptest.NewRecorder()
	responder.WriteResponse(recorder, runtime.JSONProducer())
	return recorder.Code
}

// once the allow-list is configured every request must carry a verifiable bearer token - a
// missing one (the traefik lambda fork drops duplicated headers) is denied, never trusted
func TestHandlersDenyUnverifiedCallers(t *testing.T) {
	verifier := &fakeVerifier{enabled: true, callers: map[string]*claAuth.TrustedCaller{
		"Bearer trusted": {ClientID: "ss-client", Subject: "ss-client@clients", Trusted: true},
	}}
	api, service := configuredAPI(t, verifier)
	authUser := &auth.User{UserName: "someone"}

	// a verifier that reports neither a caller nor an error must deny rather than panic
	verifier.noop = "Bearer nothing"

	for _, authorization := range []string{"", "Bearer forged", "Bearer nothing"} {
		req := request(t, authorization)
		assert.Equal(t, http.StatusUnauthorized, statusOf(t, api.MyClasGetMyClasHandler.Handle(myClasOps.GetMyClasParams{HTTPRequest: req}, authUser)))
		assert.Equal(t, http.StatusUnauthorized, statusOf(t, api.MyClasGetMyClaPdfHandler.Handle(myClasOps.GetMyClaPdfParams{HTTPRequest: req, SignatureID: "sig-1"}, authUser)))
		assert.Equal(t, http.StatusUnauthorized, statusOf(t, api.MyClasGetMyIdentitiesHandler.Handle(myClasOps.GetMyIdentitiesParams{HTTPRequest: req}, authUser)))
	}
	assert.Empty(t, service.callers, "an unverified caller must never reach the service")
	assert.Len(t, verifier.seen, 9, "every request must be verified")
}

func TestHandlersTrustAllowListedCallers(t *testing.T) {
	verifier := &fakeVerifier{enabled: true, callers: map[string]*claAuth.TrustedCaller{
		"Bearer trusted":   {ClientID: "ss-client", Subject: "ss-client@clients", Trusted: true},
		"Bearer untrusted": {ClientID: "other-client", Subject: "someone@clients"},
	}}
	api, service := configuredAPI(t, verifier)

	// a trusted caller needs no username of its own - its identity list is authoritative
	githubID := int64(999)
	params := myClasOps.GetMyClasParams{HTTPRequest: request(t, "Bearer trusted"), GithubID: []int64{githubID}}
	assert.Equal(t, http.StatusOK, statusOf(t, api.MyClasGetMyClasHandler.Handle(params, &auth.User{})))
	require.Len(t, service.callers, 1)
	assert.Equal(t, &Caller{Trusted: true}, service.callers[0])

	pdf := myClasOps.GetMyClaPdfParams{HTTPRequest: request(t, "Bearer trusted"), SignatureID: "sig-1", GithubID: []int64{githubID}}
	assert.Equal(t, http.StatusOK, statusOf(t, api.MyClasGetMyClaPdfHandler.Handle(pdf, &auth.User{})))
	require.Len(t, service.callers, 2)
	assert.Equal(t, &Caller{Trusted: true}, service.callers[1])

	// a verified token from a client that is not on the allow-list keeps the per-identity checks
	params.HTTPRequest = request(t, "Bearer untrusted")
	assert.Equal(t, http.StatusOK, statusOf(t, api.MyClasGetMyClasHandler.Handle(params, &auth.User{UserName: "someone"})))
	require.Len(t, service.callers, 3)
	assert.Equal(t, &Caller{Username: "someone"}, service.callers[2])

	// ... and it is still subject to the username requirement
	assert.Equal(t, http.StatusUnauthorized, statusOf(t, api.MyClasGetMyClasHandler.Handle(params, &auth.User{})))
	assert.Len(t, service.callers, 3)

	// an admin whose token is verified but untrusted keeps the admin bypass
	assert.Equal(t, http.StatusOK, statusOf(t, api.MyClasGetMyClasHandler.Handle(params, &auth.User{ACL: auth.ACL{Admin: true}})))
	require.Len(t, service.callers, 4)
	assert.Equal(t, &Caller{Admin: true}, service.callers[3])

	// an allow-listed admin is both, and the identity list stays authoritative
	trustedAdmin := myClasOps.GetMyClasParams{HTTPRequest: request(t, "Bearer trusted"), GithubID: []int64{githubID}}
	assert.Equal(t, http.StatusOK, statusOf(t, api.MyClasGetMyClasHandler.Handle(trustedAdmin, &auth.User{UserName: "admin", ACL: auth.ACL{Admin: true}})))
	require.Len(t, service.callers, 5)
	assert.Equal(t, &Caller{Username: "admin", Admin: true, Trusted: true}, service.callers[4])

	// a trusted caller with neither a username nor an identity has nothing to look up
	empty := myClasOps.GetMyClasParams{HTTPRequest: request(t, "Bearer trusted")}
	assert.Equal(t, http.StatusBadRequest, statusOf(t, api.MyClasGetMyClasHandler.Handle(empty, &auth.User{})))
	emptyPdf := myClasOps.GetMyClaPdfParams{HTTPRequest: request(t, "Bearer trusted"), SignatureID: "sig-1"}
	assert.Equal(t, http.StatusBadRequest, statusOf(t, api.MyClasGetMyClaPdfHandler.Handle(emptyPdf, &auth.User{})))
	assert.Len(t, service.callers, 5, "a request with nothing to look up must not reach the service")

	// GetMyIdentities always reports the authenticated principal's own identities
	identities := myClasOps.GetMyIdentitiesParams{HTTPRequest: request(t, "Bearer trusted")}
	assert.Equal(t, http.StatusUnauthorized, statusOf(t, api.MyClasGetMyIdentitiesHandler.Handle(identities, &auth.User{})))
	assert.Equal(t, http.StatusOK, statusOf(t, api.MyClasGetMyIdentitiesHandler.Handle(identities, &auth.User{UserName: "someone"})))

	// every request above is verified with the raw Authorization header, in order
	assert.Equal(t, []string{
		"Bearer trusted", "Bearer trusted",
		"Bearer untrusted", "Bearer untrusted", "Bearer untrusted",
		"Bearer trusted", "Bearer trusted", "Bearer trusted", "Bearer trusted", "Bearer trusted",
	}, verifier.seen)
}

func TestHandlersMapServiceFailures(t *testing.T) {
	api, service := configuredAPI(t, nil)
	service.err = errors.New("boom")
	authUser := &auth.User{UserName: "someone"}
	req := request(t, "")

	assert.Equal(t, http.StatusInternalServerError, statusOf(t, api.MyClasGetMyClasHandler.Handle(myClasOps.GetMyClasParams{HTTPRequest: req}, authUser)))
	assert.Equal(t, http.StatusInternalServerError, statusOf(t, api.MyClasGetMyClaPdfHandler.Handle(myClasOps.GetMyClaPdfParams{HTTPRequest: req, SignatureID: "sig-1"}, authUser)))
	assert.Equal(t, http.StatusInternalServerError, statusOf(t, api.MyClasGetMyIdentitiesHandler.Handle(myClasOps.GetMyIdentitiesParams{HTTPRequest: req}, authUser)))

	service.err = nil
	service.nilPdf = true
	assert.Equal(t, http.StatusNotFound, statusOf(t, api.MyClasGetMyClaPdfHandler.Handle(myClasOps.GetMyClaPdfParams{HTTPRequest: req, SignatureID: "sig-1"}, authUser)))
}

func TestPrincipal(t *testing.T) {
	username, admin := principal(nil)
	assert.Empty(t, username)
	assert.False(t, admin)

	username, admin = principal(&auth.User{UserName: "someone"})
	assert.Equal(t, "someone", username)
	assert.False(t, admin)

	username, admin = principal(&auth.User{UserName: "admin", ACL: auth.ACL{Admin: true}})
	assert.Equal(t, "admin", username)
	assert.True(t, admin)
}

// while no allow-list is configured nothing is trusted and no bearer token is required, so the
// endpoints keep behaving exactly as they did before
func TestHandlersWithoutAnAllowList(t *testing.T) {
	verifier := &fakeVerifier{}
	api, service := configuredAPI(t, verifier)
	params := myClasOps.GetMyClasParams{HTTPRequest: request(t, "")}

	assert.Equal(t, http.StatusOK, statusOf(t, api.MyClasGetMyClasHandler.Handle(params, &auth.User{UserName: "someone"})))
	assert.Equal(t, http.StatusUnauthorized, statusOf(t, api.MyClasGetMyClasHandler.Handle(params, &auth.User{})))
	assert.Empty(t, verifier.seen, "a disabled verifier must not be consulted")

	admin := myClasOps.GetMyClasParams{HTTPRequest: request(t, ""), LfUsername: &[]string{"victim"}[0]}
	assert.Equal(t, http.StatusOK, statusOf(t, api.MyClasGetMyClasHandler.Handle(admin, &auth.User{ACL: auth.ACL{Admin: true}})))
	require.Len(t, service.callers, 2)
	assert.Equal(t, &Caller{Username: "someone"}, service.callers[0])
	assert.Equal(t, &Caller{Admin: true}, service.callers[1])
}

func TestHandlersWithoutAVerifier(t *testing.T) {
	api, _ := configuredAPI(t, nil)
	params := myClasOps.GetMyClasParams{HTTPRequest: request(t, "")}

	assert.Equal(t, http.StatusOK, statusOf(t, api.MyClasGetMyClasHandler.Handle(params, &auth.User{UserName: "someone"})))
}
