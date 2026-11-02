// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-openapi/runtime"
	"github.com/savaki/dynastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/signatures"
	signatureService "github.com/linuxfoundation/easycla/cla-backend-go/signatures"
)

type allowlistSignatureService struct {
	signatureService.SignatureService
	signature   *v1Models.Signature
	readErr     error
	row         *signatureService.ItemSignature
	rowErr      error
	writeErr    error
	addCalls    int
	deleteCalls int
	actors      []string
}

func (f *allowlistSignatureService) GetSignature(_ context.Context, _ string) (*v1Models.Signature, error) {
	return f.signature, f.readErr
}

func (f *allowlistSignatureService) GetItemSignature(_ context.Context, _ string) (*signatureService.ItemSignature, error) {
	return f.row, f.rowErr
}

func (f *allowlistSignatureService) AddGithubOrganizationToApprovalList(_ context.Context, _ string, params v1Models.GhOrgAllowlist, _, lfUsername string) ([]v1Models.GithubOrg, error) {
	f.addCalls++
	f.actors = append(f.actors, lfUsername)
	if f.writeErr != nil {
		return nil, f.writeErr
	}
	return []v1Models.GithubOrg{{ID: params.OrganizationID}}, nil
}

func (f *allowlistSignatureService) DeleteGithubOrganizationFromApprovalList(_ context.Context, _ string, _ v1Models.GhOrgAllowlist, _, lfUsername string) ([]v1Models.GithubOrg, error) {
	f.deleteCalls++
	f.actors = append(f.actors, lfUsername)
	if f.writeErr != nil {
		return nil, f.writeErr
	}
	return []v1Models.GithubOrg{}, nil
}

type allowlistEvents struct {
	events.Service
	logged []*events.LogEventArgs
}

func (f *allowlistEvents) LogEvent(args *events.LogEventArgs) {
	f.logged = append(f.logged, args)
}

// the delete handler answers 204 with a payload, which httptest.ResponseRecorder refuses to record
type tolerantRecorder struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func newTolerantRecorder() *tolerantRecorder {
	return &tolerantRecorder{header: http.Header{}, code: http.StatusOK}
}

func (r *tolerantRecorder) Header() http.Header         { return r.header }
func (r *tolerantRecorder) WriteHeader(code int)        { r.code = code }
func (r *tolerantRecorder) Write(b []byte) (int, error) { return r.body.Write(b) }

// POST/DELETE /v4/signatures/{signatureID}/gh-org-whitelist run only for a CLA manager of the signature
// (lfx-self-serve-ops#979): the guard sits in both configured handlers, before the mutation and the event
func TestGitHubOrgAllowlistHandlersRequireTheSignatureManager(t *testing.T) {
	// a cookie-less request yields a fresh in-memory session - no DynamoDB round trip
	sessionStore, err := dynastore.New(dynastore.AWSConfig(&aws.Config{Region: aws.String("us-east-1")}))
	require.NoError(t, err)
	ccla := &v1Models.Signature{SignatureID: "sig-1", ProjectID: "cla-group-1", SignatureReferenceID: "company-1", SignatureACL: []v1Models.User{{UserID: "manager-id", LfUsername: "manager"}}}
	cases := []struct {
		name      string
		signature *v1Models.Signature
		user      *auth.User
		status    int
		calls     int
		readErr   error
		writeErr  error
		rowErr    error
		row       *signatureService.ItemSignature
	}{
		{name: "exact manager", signature: ccla, user: &auth.User{UserName: "manager", Email: "manager@example.com"}, status: http.StatusOK, calls: 1},
		{name: "manager without a user record, named by the stored ACL", signature: ccla, user: &auth.User{UserName: "ghost"}, status: http.StatusOK, calls: 1},
		{name: "stored ACL unreadable for a caller the hydration missed", signature: ccla, user: &auth.User{UserName: "ghost"}, rowErr: errors.New("dynamo down"), status: http.StatusInternalServerError},
		{name: "manager revoked before the write", signature: ccla, user: &auth.User{UserName: "manager"}, status: http.StatusForbidden, calls: 1, writeErr: signatureService.NewForbiddenError("revoked")},
		{name: "authenticated outsider", signature: ccla, user: &auth.User{UserName: "outsider", Email: "outsider@example.com"}, status: http.StatusForbidden},
		{name: "manager in another case", signature: ccla, user: &auth.User{UserName: "Manager"}, status: http.StatusForbidden},
		{name: "empty LF identity", signature: ccla, user: &auth.User{Email: "nobody@example.com"}, status: http.StatusForbidden},
		{name: "signature without an ACL", signature: &v1Models.Signature{SignatureID: "sig-1"}, user: &auth.User{UserName: "manager"}, status: http.StatusForbidden, row: &signatureService.ItemSignature{SignatureID: "sig-1"}},
		{name: "unknown signature", user: &auth.User{UserName: "manager"}, status: http.StatusForbidden},
		{name: "signature read failure", readErr: errors.New("dynamo down"), user: &auth.User{UserName: "manager"}, status: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := tc.row
			if row == nil {
				row = &signatureService.ItemSignature{SignatureID: "sig-1", SignatureACL: []string{"manager", "ghost"}}
			}
			svc := &allowlistSignatureService{signature: tc.signature, readErr: tc.readErr, writeErr: tc.writeErr, row: row, rowErr: tc.rowErr}
			denied, code := "caller is not a CLA manager of signature sig-1", `"Code":"403"`
			if tc.readErr != nil || tc.rowErr != nil {
				denied, code = "unable to load signature sig-1", `"Code":"500"`
			}
			expectedEvents := tc.calls
			if tc.writeErr != nil {
				expectedEvents = 0
			}
			logged := &allowlistEvents{}
			api := operations.NewEasyclaAPI(nil)
			Configure(api, nil, nil, nil, svc, sessionStore, logged, nil, nil)
			require.NotNil(t, api.SignaturesAddGitHubOrgAllowlistHandler)
			require.NotNil(t, api.SignaturesDeleteGitHubOrgAllowlistHandler)
			reqID := "req-979"
			body := models.GhOrgAllowlist{OrganizationID: aws.String("some-org")}

			t.Run("add", func(t *testing.T) {
				recorder := httptest.NewRecorder()
				api.SignaturesAddGitHubOrgAllowlistHandler.Handle(signatures.AddGitHubOrgAllowlistParams{
					HTTPRequest: httptest.NewRequest(http.MethodPost, "/v4/signatures/sig-1/gh-org-whitelist", nil),
					XREQUESTID:  &reqID,
					SignatureID: "sig-1",
					Body:        body,
				}, tc.user).WriteResponse(recorder, runtime.JSONProducer())
				assert.Equal(t, tc.status, recorder.Code, recorder.Body.String())
				assert.Equal(t, "req-979", recorder.Header().Get("X-Request-Id"))
				assert.Equal(t, tc.calls, svc.addCalls, "the approval list changes only for the signature's CLA manager")
				if tc.calls > 0 {
					assert.Equal(t, []string{tc.user.UserName}, svc.actors, "the exact LF username is the acting manager")
				}
				require.Len(t, logged.logged, expectedEvents, "an event is logged only with the mutation")
				if expectedEvents == 0 {
					assert.Contains(t, recorder.Body.String(), denied)
					assert.Contains(t, recorder.Body.String(), code)
					assert.NotContains(t, recorder.Body.String(), "dynamo down", "the storage error stays in the log")
					return
				}
				event := logged.logged[0]
				assert.Equal(t, events.ApprovalListGitHubOrganizationAdded, event.EventType)
				assert.Equal(t, "cla-group-1", event.ProjectID)
				assert.Equal(t, "company-1", event.CompanyID)
				assert.Equal(t, tc.user.UserName, event.LfUsername)
				assert.Equal(t, &events.ApprovalListGitHubOrganizationAddedEventData{GitHubOrganizationName: "some-org"}, event.EventData)
			})
			logged.logged = nil

			t.Run("delete", func(t *testing.T) {
				recorder := newTolerantRecorder()
				api.SignaturesDeleteGitHubOrgAllowlistHandler.Handle(signatures.DeleteGitHubOrgAllowlistParams{
					HTTPRequest: httptest.NewRequest(http.MethodDelete, "/v4/signatures/sig-1/gh-org-whitelist", nil),
					XREQUESTID:  &reqID,
					SignatureID: "sig-1",
					Body:        body,
				}, tc.user).WriteResponse(recorder, runtime.JSONProducer())
				status := tc.status
				if status == http.StatusOK {
					status = http.StatusNoContent
				}
				assert.Equal(t, status, recorder.code, recorder.body.String())
				assert.Equal(t, "req-979", recorder.header.Get("X-Request-Id"))
				assert.Equal(t, tc.calls, svc.deleteCalls, "the approval list changes only for the signature's CLA manager")
				require.Len(t, logged.logged, expectedEvents)
				if expectedEvents == 0 {
					assert.Contains(t, recorder.body.String(), denied)
					assert.Contains(t, recorder.body.String(), code)
					assert.NotContains(t, recorder.body.String(), "dynamo down", "the storage error stays in the log")
					return
				}
				event := logged.logged[0]
				assert.Equal(t, events.ApprovalListGitHubOrganizationDeleted, event.EventType)
				assert.Equal(t, "cla-group-1", event.ProjectID)
				assert.Equal(t, "company-1", event.CompanyID)
				assert.Equal(t, tc.user.UserName, event.LfUsername)
				assert.Equal(t, &events.ApprovalListGitHubOrganizationDeletedEventData{GitHubOrganizationName: "some-org"}, event.EventData)
			})
		})
	}
}
