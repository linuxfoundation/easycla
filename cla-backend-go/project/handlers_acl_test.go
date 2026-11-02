// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package project_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-openapi/runtime"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	projectOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/project"
	"github.com/linuxfoundation/easycla/cla-backend-go/gerrits"
	"github.com/linuxfoundation/easycla/cla-backend-go/project"
	"github.com/linuxfoundation/easycla/cla-backend-go/project/service"
	"github.com/linuxfoundation/easycla/cla-backend-go/repositories"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeACLProjectService serves one CLA group and records the write calls reached by the handlers
type fakeACLProjectService struct {
	service.Service
	claGroup    *models.ClaGroup
	deleteCalls int
	updateCalls int
}

func (f *fakeACLProjectService) GetCLAGroupByID(_ context.Context, claGroupID string) (*models.ClaGroup, error) {
	if f.claGroup != nil && f.claGroup.ProjectID == claGroupID {
		return f.claGroup, nil
	}
	return nil, nil
}

func (f *fakeACLProjectService) DeleteCLAGroup(_ context.Context, _ string) error {
	f.deleteCalls++
	return nil
}

func (f *fakeACLProjectService) UpdateCLAGroup(_ context.Context, claGroupModel *models.ClaGroup) (*models.ClaGroup, error) {
	f.updateCalls++
	return claGroupModel, nil
}

type fakeACLGerritService struct{ gerrits.Service }

func (fakeACLGerritService) DeleteClaGroupGerrits(_ context.Context, _ string) (int, error) {
	return 0, nil
}

type fakeACLRepositoriesService struct{ repositories.Service }

func (fakeACLRepositoriesService) DisableRepositoriesByProjectID(_ context.Context, _ string) (int, error) {
	return 0, nil
}

type fakeACLSignatureService struct{ signatures.SignatureService }

func (fakeACLSignatureService) InvalidateProjectRecords(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

type fakeACLEventsService struct {
	events.Service
	logCalls int
}

func (f *fakeACLEventsService) LogEventWithContext(_ context.Context, _ *events.LogEventArgs) {
	f.logCalls++
}

func assertProjectRejected(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int) {
	t.Helper()
	assert.Equal(t, expectedStatus, recorder.Code, recorder.Body.String())
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, strconv.Itoa(expectedStatus), payload["Code"])
	if expectedStatus == http.StatusForbidden {
		assert.Contains(t, payload["Message"], "not in project ACL")
	} else {
		assert.Contains(t, payload["Message"], "missing authenticated LF user")
	}
}

func newProjectACLAPI(t *testing.T, claGroup *models.ClaGroup) (*operations.ClaAPI, *fakeACLProjectService, *fakeACLEventsService) {
	t.Helper()
	svc := &fakeACLProjectService{claGroup: claGroup}
	eventsSvc := &fakeACLEventsService{}
	api := operations.NewClaAPI(nil)
	project.Configure(api, svc, eventsSvc, fakeACLGerritService{}, fakeACLRepositoriesService{}, fakeACLSignatureService{})
	return api, svc, eventsSvc
}

func TestProjectDeleteAndUpdateRequireProjectACL(t *testing.T) {
	const claGroupID = "11111111-2222-3333-4444-555555555555"
	manager := &user.CLAUser{UserID: "manager-id", LFUsername: "manager", LFEmail: "manager@example.com"}
	outsider := &user.CLAUser{UserID: "outsider-id", LFUsername: "outsider", LFEmail: "outsider@example.com"}
	anonymous := &user.CLAUser{UserID: "anonymous-id"}

	tests := []struct {
		name           string
		projectACL     []string
		claUser        *user.CLAUser
		expectWrite    bool
		expectedStatus int
	}{
		{name: "project ACL member", projectACL: []string{"someone-else", "manager"}, claUser: manager, expectWrite: true},
		{name: "authenticated outsider", projectACL: []string{"manager"}, claUser: outsider, expectedStatus: http.StatusForbidden},
		{name: "empty project ACL", projectACL: nil, claUser: manager, expectedStatus: http.StatusForbidden},
		{name: "caller without LF username", projectACL: []string{"manager"}, claUser: anonymous, expectedStatus: http.StatusUnauthorized},
		{name: "missing principal", projectACL: []string{"manager"}, claUser: nil, expectedStatus: http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claGroup := func() *models.ClaGroup {
				return &models.ClaGroup{
					ProjectID:          claGroupID,
					ProjectName:        "Guarded CLA Group",
					ProjectDescription: "before",
					ProjectExternalID:  "a0941000002wBz4AAE",
					ProjectACL:         tc.projectACL,
				}
			}

			t.Run("delete", func(t *testing.T) {
				api, svc, eventsSvc := newProjectACLAPI(t, claGroup())
				recorder := httptest.NewRecorder()
				api.ProjectDeleteProjectByIDHandler.Handle(projectOps.DeleteProjectByIDParams{
					HTTPRequest: httptest.NewRequest(http.MethodDelete, "/v3/project/"+claGroupID, nil),
					ProjectID:   claGroupID,
				}, tc.claUser).WriteResponse(recorder, runtime.JSONProducer())

				if tc.expectWrite {
					assert.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
					assert.Equal(t, 1, svc.deleteCalls)
					assert.Equal(t, 1, eventsSvc.logCalls, "only the CLA group deleted event is logged when nothing else was removed")
					return
				}
				assertProjectRejected(t, recorder, tc.expectedStatus)
				assert.Zero(t, svc.deleteCalls, "DeleteCLAGroup must not be reached for rejected callers")
				assert.Zero(t, eventsSvc.logCalls)
			})

			t.Run("update", func(t *testing.T) {
				api, svc, eventsSvc := newProjectACLAPI(t, claGroup())
				recorder := httptest.NewRecorder()
				api.ProjectUpdateProjectHandler.Handle(projectOps.UpdateProjectParams{
					HTTPRequest: httptest.NewRequest(http.MethodPut, "/v3/project", nil),
					Body: models.ClaGroup{
						ProjectID:          claGroupID,
						ProjectName:        "Guarded CLA Group",
						ProjectDescription: "after",
						ProjectExternalID:  "a0941000002wBz4AAE",
					},
				}, tc.claUser).WriteResponse(recorder, runtime.JSONProducer())

				if tc.expectWrite {
					assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
					assert.Equal(t, 1, svc.updateCalls)
					assert.Equal(t, 1, eventsSvc.logCalls)
					return
				}
				assertProjectRejected(t, recorder, tc.expectedStatus)
				assert.Zero(t, svc.updateCalls, "UpdateCLAGroup must not be reached for rejected callers")
				assert.Zero(t, eventsSvc.logCalls)
			})
		})
	}
}

func TestProjectUpdateUnknownCLAGroupStillReturnsNotFound(t *testing.T) {
	api, svc, _ := newProjectACLAPI(t, nil)
	recorder := httptest.NewRecorder()
	api.ProjectUpdateProjectHandler.Handle(projectOps.UpdateProjectParams{
		HTTPRequest: httptest.NewRequest(http.MethodPut, "/v3/project", nil),
		Body:        models.ClaGroup{ProjectID: "missing"},
	}, &user.CLAUser{UserID: "manager-id", LFUsername: "manager"}).WriteResponse(recorder, runtime.JSONProducer())

	assert.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	assert.Zero(t, svc.updateCalls)
}

func TestProjectDeleteUnknownCLAGroupReturnsNotFound(t *testing.T) {
	api, svc, eventsSvc := newProjectACLAPI(t, nil)
	recorder := httptest.NewRecorder()
	api.ProjectDeleteProjectByIDHandler.Handle(projectOps.DeleteProjectByIDParams{
		HTTPRequest: httptest.NewRequest(http.MethodDelete, "/v3/project/missing", nil),
		ProjectID:   "missing",
	}, &user.CLAUser{UserID: "manager-id", LFUsername: "manager"}).WriteResponse(recorder, runtime.JSONProducer())

	assert.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	assert.Zero(t, svc.deleteCalls)
	assert.Zero(t, eventsSvc.logCalls)
}
