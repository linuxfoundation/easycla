// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-openapi/runtime"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	sigOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/savaki/dynastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAllowlistSignatureService struct {
	SignatureService
	signature   *models.Signature
	writeErr    error
	addCalls    int
	deleteCalls int
	actors      []string
}

func (f *fakeAllowlistSignatureService) GetSignature(_ context.Context, _ string) (*models.Signature, error) {
	return f.signature, nil
}

func (f *fakeAllowlistSignatureService) AddGithubOrganizationToApprovalList(_ context.Context, _ string, params models.GhOrgAllowlist, _, lfUsername string) ([]models.GithubOrg, error) {
	f.addCalls++
	f.actors = append(f.actors, lfUsername)
	if f.writeErr != nil {
		return nil, f.writeErr
	}
	return []models.GithubOrg{{ID: params.OrganizationID}}, nil
}

func (f *fakeAllowlistSignatureService) DeleteGithubOrganizationFromApprovalList(_ context.Context, _ string, _ models.GhOrgAllowlist, _, lfUsername string) ([]models.GithubOrg, error) {
	f.deleteCalls++
	f.actors = append(f.actors, lfUsername)
	if f.writeErr != nil {
		return nil, f.writeErr
	}
	return []models.GithubOrg{}, nil
}

// bodyTolerantRecorder captures the response without the 204-with-body check that httptest.ResponseRecorder applies -
// the delete handler has always answered 204 with a payload
type bodyTolerantRecorder struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func newBodyTolerantRecorder() *bodyTolerantRecorder {
	return &bodyTolerantRecorder{header: http.Header{}, code: http.StatusOK}
}

func (r *bodyTolerantRecorder) Header() http.Header         { return r.header }
func (r *bodyTolerantRecorder) WriteHeader(code int)        { r.code = code }
func (r *bodyTolerantRecorder) Write(b []byte) (int, error) { return r.body.Write(b) }

type fakeAllowlistEventsService struct {
	events.Service
	logCalls int
}

func (f *fakeAllowlistEventsService) LogEventWithContext(_ context.Context, _ *events.LogEventArgs) {
	f.logCalls++
}

// POST/DELETE /v3/signatures/{signatureID}/gh-org-whitelist must only be served to the CLA managers on the
// signature ACL (lfx-self-serve-ops#801)
func TestGitHubOrgAllowlistWritesRequireSignatureACL(t *testing.T) {
	// a cookie-less request yields a fresh in-memory session - no DynamoDB round trip
	sessionStore, err := dynastore.New(dynastore.AWSConfig(&aws.Config{Region: aws.String("us-east-1")}))
	require.NoError(t, err)

	ccla := &models.Signature{
		SignatureID:          "sig-1",
		ProjectID:            "cla-group-1",
		SignatureReferenceID: "company-1",
		SignatureACL:         []models.User{{UserID: "manager-id", LfUsername: "manager"}},
	}
	manager := &user.CLAUser{UserID: "manager-id", LFUsername: "manager", LFEmail: "manager@example.com"}
	managerByUsernameOnly := &user.CLAUser{UserID: "manager-other-record", LFUsername: "manager", LFEmail: "manager@example.com"}
	managerByIDOnly := &user.CLAUser{UserID: "manager-id"}
	outsider := &user.CLAUser{UserID: "outsider-id", LFUsername: "outsider", LFEmail: "outsider@example.com"}

	testCases := []struct {
		name           string
		signature      *models.Signature
		claUser        *user.CLAUser
		expectedStatus int
		expectedCalls  int
		writeErr       error
	}{
		{name: "cla manager on the ACL", signature: ccla, claUser: manager, expectedStatus: http.StatusOK, expectedCalls: 1},
		{name: "cla manager revoked before the write", signature: ccla, claUser: manager, expectedStatus: http.StatusForbidden, expectedCalls: 1, writeErr: NewForbiddenError("revoked")},
		{name: "cla manager matched by LF username", signature: ccla, claUser: managerByUsernameOnly, expectedStatus: http.StatusOK, expectedCalls: 1},
		{name: "cla manager matched by user id only", signature: ccla, claUser: managerByIDOnly, expectedStatus: http.StatusOK, expectedCalls: 1},
		{name: "authenticated user outside the ACL", signature: ccla, claUser: outsider, expectedStatus: http.StatusForbidden},
		{name: "signature without an ACL", signature: &models.Signature{SignatureID: "sig-1"}, claUser: manager, expectedStatus: http.StatusForbidden},
		{name: "unknown signature", signature: nil, claUser: manager, expectedStatus: http.StatusNotFound},
		{name: "missing principal", signature: ccla, claUser: nil, expectedStatus: http.StatusUnauthorized},
		{name: "principal without identity", signature: ccla, claUser: &user.CLAUser{LFEmail: "nobody@example.com"}, expectedStatus: http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeAllowlistSignatureService{signature: tc.signature, writeErr: tc.writeErr}
			eventsService := &fakeAllowlistEventsService{}
			expectedEvents := tc.expectedCalls
			if tc.writeErr != nil {
				expectedEvents = 0
			}
			api := operations.NewClaAPI(nil)
			Configure(api, service, sessionStore, eventsService)
			require.NotNil(t, api.SignaturesAddGitHubOrgAllowlistHandler)
			require.NotNil(t, api.SignaturesDeleteGitHubOrgAllowlistHandler)
			reqID := "req-801"

			t.Run("add", func(t *testing.T) {
				recorder := httptest.NewRecorder()
				api.SignaturesAddGitHubOrgAllowlistHandler.Handle(sigOps.AddGitHubOrgAllowlistParams{
					HTTPRequest: httptest.NewRequest(http.MethodPost, "/v3/signatures/sig-1/gh-org-whitelist", nil),
					XREQUESTID:  &reqID,
					SignatureID: "sig-1",
					Body:        models.GhOrgAllowlist{OrganizationID: aws.String("some-org")},
				}, tc.claUser).WriteResponse(recorder, runtime.JSONProducer())

				assert.Equal(t, tc.expectedStatus, recorder.Code, recorder.Body.String())
				assert.Equal(t, "req-801", recorder.Header().Get("X-Request-Id"))
				assert.Equal(t, tc.expectedCalls, service.addCalls, "the approval list must only change for ACL members")
				assert.Equal(t, expectedEvents, eventsService.logCalls)
				if tc.expectedCalls > 0 {
					assert.Equal(t, []string{"manager"}, service.actors, "the stored ACL username is the acting manager")
				}
				assertAllowlistErrorPayload(t, tc.expectedStatus, recorder.Body.Bytes())
			})

			t.Run("delete", func(t *testing.T) {
				recorder := newBodyTolerantRecorder()
				api.SignaturesDeleteGitHubOrgAllowlistHandler.Handle(sigOps.DeleteGitHubOrgAllowlistParams{
					HTTPRequest: httptest.NewRequest(http.MethodDelete, "/v3/signatures/sig-1/gh-org-whitelist", nil),
					XREQUESTID:  &reqID,
					SignatureID: "sig-1",
					Body:        models.GhOrgAllowlist{OrganizationID: aws.String("some-org")},
				}, tc.claUser).WriteResponse(recorder, runtime.JSONProducer())

				expectedStatus := tc.expectedStatus
				if expectedStatus == http.StatusOK {
					expectedStatus = http.StatusNoContent
				}
				assert.Equal(t, expectedStatus, recorder.code, recorder.body.String())
				assert.Equal(t, "req-801", recorder.header.Get("X-Request-Id"))
				assert.Equal(t, tc.expectedCalls, service.deleteCalls, "the approval list must only change for ACL members")
				if tc.expectedCalls > 0 {
					assert.Equal(t, []string{"manager", "manager"}, service.actors)
				}
				assertAllowlistErrorPayload(t, tc.expectedStatus, recorder.body.Bytes())
			})
		})
	}
}

func assertAllowlistErrorPayload(t *testing.T, expectedStatus int, body []byte) {
	t.Helper()
	if expectedStatus == http.StatusOK {
		return
	}
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &payload))
	switch expectedStatus {
	case http.StatusForbidden:
		assert.Equal(t, "403", payload["Code"])
		assert.Contains(t, payload["Message"], "not authorized to modify the approval list")
	case http.StatusNotFound:
		assert.Equal(t, "404", payload["Code"])
	case http.StatusUnauthorized:
		assert.Equal(t, "401", payload["Code"])
		assert.Contains(t, payload["Message"], "missing authenticated user")
	}
}
