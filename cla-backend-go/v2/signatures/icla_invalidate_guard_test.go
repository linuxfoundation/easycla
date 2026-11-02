// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	sigOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_projects_cla_groups "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeIclaInvalidateService struct {
	ServiceInterface
	calls         int
	gotClaGroupID string
	gotUserID     string
}

func (f *fakeIclaInvalidateService) InvalidateICLA(_ context.Context, claGroupID string, userID string, _ *auth.User, _ events.Service, _ *events.LogEventArgs, _ *models.IclaInvalidationInput) error {
	f.calls++
	f.gotClaGroupID = claGroupID
	f.gotUserID = userID
	return nil
}

// PUT /v4/cla-group/{claGroupID}/user/{userID}/icla must apply the same project-scope gate as the
// ICLA list endpoint for that CLA group (lfx-self-serve-ops#802)
func TestInvalidateICLAHandlerRequiresCLAGroupProjectScope(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	testCases := []struct {
		name           string
		authUser       *auth.User
		expectedStatus int
		expectedCalls  int
	}{
		{name: "project scope on the CLA group project", authUser: projectScopedUser(auth.Project, "proj-sfid"), expectedStatus: http.StatusOK, expectedCalls: 1},
		{name: "admin", authUser: &auth.User{UserName: "admin", Email: "admin@example.com", ACL: auth.ACL{Admin: true, Allowed: true}}, expectedStatus: http.StatusOK, expectedCalls: 1},
		{name: "cla manager of a company on the project", authUser: projectScopedUser(auth.ProjectOrganization, "proj-sfid|company-a"), expectedStatus: http.StatusForbidden},
		{name: "project scope on another project", authUser: projectScopedUser(auth.Project, "other-proj-sfid"), expectedStatus: http.StatusForbidden},
		{name: "no scopes", authUser: &auth.User{UserName: "nobody", Email: "nobody@example.com", ACL: auth.ACL{Allowed: true}}, expectedStatus: http.StatusForbidden},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
			mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), "cla-group-1").
				Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-sfid", ProjectExternalID: "proj-sfid", FoundationSFID: "foundation-sfid", ClaGroupID: "cla-group-1"}}, nil).AnyTimes()

			service := &fakeIclaInvalidateService{}
			api := operations.NewEasyclaAPI(nil)
			Configure(api, nil, nil, nil, nil, nil, nil, service, mockProjectClaGroupsRepo)
			require.NotNil(t, api.SignaturesInvalidateICLAHandler)

			username, email, reqID := tc.authUser.UserName, tc.authUser.Email, "req-802"
			recorder := httptest.NewRecorder()
			api.SignaturesInvalidateICLAHandler.Handle(sigOps.InvalidateICLAParams{
				HTTPRequest: httptest.NewRequest(http.MethodPut, "/v4/cla-group/cla-group-1/user/user-1/icla", nil),
				XUSERNAME:   &username, XEMAIL: &email, XREQUESTID: &reqID,
				ClaGroupID: "cla-group-1", UserID: "user-1",
				Body: models.IclaInvalidationInput{Reason: "compliance"},
			}, tc.authUser).WriteResponse(recorder, runtime.JSONProducer())

			assert.Equal(t, tc.expectedStatus, recorder.Code, recorder.Body.String())
			assert.Equal(t, "req-802", recorder.Header().Get("X-Request-Id"))
			assert.Equal(t, tc.expectedCalls, service.calls, "service must only run for entitled users")
			if tc.expectedStatus == http.StatusOK {
				assert.Equal(t, "cla-group-1", service.gotClaGroupID)
				assert.Equal(t, "user-1", service.gotUserID)
				return
			}
			var payload map[string]interface{}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			assert.Equal(t, "403", payload["Code"])
			assert.Equal(t, "req-802", payload["x-request-id"])
			assert.Contains(t, payload["Message"], "not authorized to invalidate ICLA signatures")
		})
	}
}
