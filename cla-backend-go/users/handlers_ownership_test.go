// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package users_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-openapi/runtime"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	usersOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/users"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOwnershipUsersService only implements the lookups and writes exercised by the update/delete handlers
type fakeOwnershipUsersService struct {
	users.Service
	byGitHubUsername map[string]*models.User
	byUserID         map[string]*models.User
	saveCalls        int
	deleteCalls      int
	lastDeletedID    string
}

func (f *fakeOwnershipUsersService) GetUserByGitHubUsername(gitHubUsername string) (*models.User, error) {
	return f.byGitHubUsername[gitHubUsername], nil
}

func (f *fakeOwnershipUsersService) GetUser(userID string) (*models.User, error) {
	return f.byUserID[userID], nil
}

func (f *fakeOwnershipUsersService) Save(update *models.UserUpdate, _ *user.CLAUser) (*models.User, error) {
	f.saveCalls++
	return &models.User{UserID: update.UserID, LfUsername: update.LfUsername, GithubUsername: update.GithubUsername}, nil
}

func (f *fakeOwnershipUsersService) Delete(userID string, _ *user.CLAUser) error {
	f.deleteCalls++
	f.lastDeletedID = userID
	return nil
}

func assertUsersForbidden(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "403", payload["Code"])
	assert.Contains(t, payload["Message"], "403 Forbidden")
}

func TestUpdateUserByGitHubUsernameRequiresOwnership(t *testing.T) {
	const (
		callerLogin = "owner-login"
		otherLogin  = "other-login"
	)
	caller := &user.CLAUser{UserID: "caller-id", LFUsername: callerLogin}

	tests := []struct {
		name           string
		existing       *models.User
		body           *models.UserUpdate
		claUser        *user.CLAUser
		expectedStatus int
		expectSave     bool
	}{
		{
			name:           "owner updates their own linked record",
			existing:       &models.User{UserID: "caller-id", LfUsername: callerLogin, GithubUsername: "gh-owner"},
			body:           &models.UserUpdate{GithubUsername: "gh-owner", LfUsername: callerLogin, LfEmail: "owner@example.com"},
			claUser:        caller,
			expectedStatus: http.StatusOK,
			expectSave:     true,
		},
		{
			// v3 tokens carry no GitHub identity, so an unlinked GitHub record cannot be claimed through this endpoint
			name:           "unlinked record claimed with the caller's own LF login",
			existing:       &models.User{UserID: "gh-only-id", GithubUsername: "gh-victim"},
			body:           &models.UserUpdate{GithubUsername: "gh-victim", LfUsername: callerLogin},
			claUser:        caller,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "record linked to another LF login",
			existing:       &models.User{UserID: "victim-id", LfUsername: otherLogin, GithubUsername: "gh-victim"},
			body:           &models.UserUpdate{GithubUsername: "gh-victim", LfEmail: "attacker@example.com"},
			claUser:        caller,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "unlinked record claimed for a different LF login",
			existing:       &models.User{UserID: "gh-only-id", GithubUsername: "gh-victim"},
			body:           &models.UserUpdate{GithubUsername: "gh-victim", LfUsername: otherLogin},
			claUser:        caller,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "own record but body names another LF login",
			existing:       &models.User{UserID: "caller-id", LfUsername: callerLogin, GithubUsername: "gh-owner"},
			body:           &models.UserUpdate{GithubUsername: "gh-owner", LfUsername: otherLogin},
			claUser:        caller,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "unlinked record without naming any LF login",
			existing:       &models.User{UserID: "gh-only-id", GithubUsername: "gh-victim"},
			body:           &models.UserUpdate{GithubUsername: "gh-victim", LfEmail: "attacker@example.com"},
			claUser:        caller,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "caller without LF login",
			existing:       &models.User{UserID: "caller-id", LfUsername: callerLogin, GithubUsername: "gh-owner"},
			body:           &models.UserUpdate{GithubUsername: "gh-owner"},
			claUser:        &user.CLAUser{UserID: "anonymous-id"},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeOwnershipUsersService{
				byGitHubUsername: map[string]*models.User{tc.existing.GithubUsername: tc.existing},
			}
			api := operations.NewClaAPI(nil)
			users.Configure(api, service, nil)

			recorder := httptest.NewRecorder()
			api.UsersUpdateUserHandler.Handle(usersOps.UpdateUserParams{
				HTTPRequest: httptest.NewRequest(http.MethodPut, "/v3/users", nil),
				Body:        tc.body,
			}, tc.claUser).WriteResponse(recorder, runtime.JSONProducer())

			if tc.expectedStatus == http.StatusForbidden {
				assertUsersForbidden(t, recorder)
			} else {
				assert.Equal(t, tc.expectedStatus, recorder.Code, recorder.Body.String())
			}
			if tc.expectSave {
				assert.Equal(t, 1, service.saveCalls)
			} else {
				assert.Zero(t, service.saveCalls, "Save must not be reached for rejected callers")
			}
		})
	}
}

func TestDeleteUserRequiresSelfOrAdmin(t *testing.T) {
	tests := []struct {
		name           string
		targetUserID   string
		claUser        *user.CLAUser
		callerRecord   *models.User
		expectedStatus int
		expectDelete   bool
	}{
		{
			name:           "user deletes their own record",
			targetUserID:   "self-id",
			claUser:        &user.CLAUser{UserID: "self-id", LFUsername: "self"},
			callerRecord:   &models.User{UserID: "self-id"},
			expectedStatus: http.StatusNoContent,
			expectDelete:   true,
		},
		{
			name:           "admin deletes another user",
			targetUserID:   "victim-id",
			claUser:        &user.CLAUser{UserID: "admin-id", LFUsername: "admin"},
			callerRecord:   &models.User{UserID: "admin-id", Admin: true},
			expectedStatus: http.StatusNoContent,
			expectDelete:   true,
		},
		{
			name:           "regular user deletes another user",
			targetUserID:   "victim-id",
			claUser:        &user.CLAUser{UserID: "regular-id", LFUsername: "regular"},
			callerRecord:   &models.User{UserID: "regular-id"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "caller without a user record deletes another user",
			targetUserID:   "victim-id",
			claUser:        &user.CLAUser{UserID: "ghost-id", LFUsername: "ghost"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "caller without user id",
			targetUserID:   "victim-id",
			claUser:        &user.CLAUser{LFUsername: "no-id"},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeOwnershipUsersService{byUserID: map[string]*models.User{}}
			if tc.callerRecord != nil {
				service.byUserID[tc.callerRecord.UserID] = tc.callerRecord
			}
			api := operations.NewClaAPI(nil)
			users.Configure(api, service, nil)

			recorder := httptest.NewRecorder()
			api.UsersDeleteUserHandler.Handle(usersOps.DeleteUserParams{
				HTTPRequest: httptest.NewRequest(http.MethodDelete, "/v3/users/"+tc.targetUserID, nil),
				UserID:      tc.targetUserID,
			}, tc.claUser).WriteResponse(recorder, runtime.JSONProducer())

			if tc.expectedStatus == http.StatusForbidden {
				assertUsersForbidden(t, recorder)
			} else {
				assert.Equal(t, tc.expectedStatus, recorder.Code, recorder.Body.String())
			}
			if tc.expectDelete {
				assert.Equal(t, 1, service.deleteCalls)
				assert.Equal(t, tc.targetUserID, service.lastDeletedID)
			} else {
				assert.Zero(t, service.deleteCalls, "Delete must not be reached for rejected callers")
			}
		})
	}
}
