// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"
	"testing"

	"github.com/go-openapi/strfmt"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
)

type stubUsersService struct {
	called  bool
	updates map[string]interface{}
}

func (s *stubUsersService) UpdateUser(userID string, updates map[string]interface{}) (*models.User, error) {
	s.called = true
	s.updates = updates
	return &models.User{UserID: userID}, nil
}
func (s *stubUsersService) CreateUser(*models.User, *user.CLAUser) (*models.User, error) { return nil, nil }
func (s *stubUsersService) Save(*models.UserUpdate, *user.CLAUser) (*models.User, error) { return nil, nil }
func (s *stubUsersService) Delete(string, *user.CLAUser) error                          { return nil }
func (s *stubUsersService) GetUser(string) (*models.User, error)                        { return nil, nil }
func (s *stubUsersService) GetUserByLFUserName(string) (*models.User, error)            { return nil, nil }
func (s *stubUsersService) GetUserByUserName(string, bool) (*models.User, error)        { return nil, nil }
func (s *stubUsersService) GetUserByEmail(string) (*models.User, error)                 { return nil, nil }
func (s *stubUsersService) GetUsersByEmail(string) ([]*models.User, error)              { return nil, nil }
func (s *stubUsersService) GetUsersByLFEmail(string) ([]*models.User, error)            { return nil, nil }
func (s *stubUsersService) GetUserByGitHubID(string) (*models.User, error)              { return nil, nil }
func (s *stubUsersService) GetUserByGitHubUsername(string) (*models.User, error)        { return nil, nil }
func (s *stubUsersService) GetUserByGitlabID(int) (*models.User, error)                 { return nil, nil }
func (s *stubUsersService) GetUserByGitLabUsername(string) (*models.User, error)        { return nil, nil }
func (s *stubUsersService) SearchUsers(string, string, bool) (*models.Users, error)     { return nil, nil }
func (s *stubUsersService) UpdateUserCompanyID(string, string, string) error            { return nil }
func (s *stubUsersService) ConvertUserModelToUserCompatModel(*models.User) (*models.UserCompat, error) {
	return nil, nil
}

func makeStoredUser(name, email string) *models.User {
	return &models.User{Username: name, LfEmail: strfmt.Email(email)}
}
func makeTestCLAUser(name, email string) *user.CLAUser {
	return &user.CLAUser{Name: name, LFEmail: email}
}

func TestRefreshStoredUserName(t *testing.T) {
	cases := []struct {
		desc         string
		stored       *models.User
		claUser      *user.CLAUser
		wantCalled   bool
		wantNameKey  bool
		wantEmailKey bool
		wantEmailVal string
	}{
		{
			desc:       "nil userModel → no-op",
			stored:     nil,
			claUser:    makeTestCLAUser("Alice", "alice@example.com"),
			wantCalled: false,
		},
		{
			desc:       "nil claUser → no-op",
			stored:     makeStoredUser("Alice", "alice@example.com"),
			claUser:    nil,
			wantCalled: false,
		},
		{
			desc:       "name unchanged, email unchanged → no-op",
			stored:     makeStoredUser("Alice", "alice@example.com"),
			claUser:    makeTestCLAUser("Alice", "alice@example.com"),
			wantCalled: false,
		},
		{
			desc:         "name changed only → only user_name updated",
			stored:       makeStoredUser("Alice", "alice@example.com"),
			claUser:      makeTestCLAUser("Alice Smith", "alice@example.com"),
			wantCalled:   true,
			wantNameKey:  true,
			wantEmailKey: false,
		},
		{
			desc:         "email changed only → only lf_email updated",
			stored:       makeStoredUser("Alice", "alice-old@example.com"),
			claUser:      makeTestCLAUser("Alice", "alice-new@example.com"),
			wantCalled:   true,
			wantNameKey:  false,
			wantEmailKey: true,
			wantEmailVal: "alice-new@example.com",
		},
		{
			desc:         "both changed → both updated",
			stored:       makeStoredUser("Alice", "alice-old@example.com"),
			claUser:      makeTestCLAUser("Alice Smith", "alice-new@example.com"),
			wantCalled:   true,
			wantNameKey:  true,
			wantEmailKey: true,
			wantEmailVal: "alice-new@example.com",
		},
		{
			desc:       "case-insensitive email match → no-op",
			stored:     makeStoredUser("Alice", "Alice@Example.COM"),
			claUser:    makeTestCLAUser("Alice", "alice@example.com"),
			wantCalled: false,
		},
		{
			desc:         "email uppercased in JWT → lowercased on write",
			stored:       makeStoredUser("Alice", "old@example.com"),
			claUser:      makeTestCLAUser("Alice", "NEW@Example.COM"),
			wantCalled:   true,
			wantEmailKey: true,
			wantEmailVal: "new@example.com",
		},
		{
			desc:         "empty name in claUser → name not touched, email still synced",
			stored:       makeStoredUser("Alice", "old@example.com"),
			claUser:      makeTestCLAUser("", "new@example.com"),
			wantCalled:   true,
			wantNameKey:  false,
			wantEmailKey: true,
		},
		{
			desc:         "empty email in claUser → email not touched, name still synced",
			stored:       makeStoredUser("Alice", "old@example.com"),
			claUser:      makeTestCLAUser("Alice Smith", ""),
			wantCalled:   true,
			wantNameKey:  true,
			wantEmailKey: false,
		},
		{
			desc:       "both empty in claUser → no-op",
			stored:     makeStoredUser("Alice", "old@example.com"),
			claUser:    makeTestCLAUser("", ""),
			wantCalled: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			svc := &stubUsersService{}
			result := refreshStoredUserName(svc, tc.stored, tc.claUser)

			if svc.called != tc.wantCalled {
				t.Fatalf("UpdateUser called=%v, want %v", svc.called, tc.wantCalled)
			}
			if !svc.called {
				return
			}
			_, hasName := svc.updates["user_name"]
			if hasName != tc.wantNameKey {
				t.Errorf("user_name in updates=%v, want %v", hasName, tc.wantNameKey)
			}
			emailVal, hasEmail := svc.updates["lf_email"]
			if hasEmail != tc.wantEmailKey {
				t.Errorf("lf_email in updates=%v, want %v", hasEmail, tc.wantEmailKey)
			}
			if hasEmail {
				got, ok := emailVal.(string)
				if !ok {
					t.Fatalf("lf_email value is not string: %T", emailVal)
				}
				if got != strings.ToLower(got) {
					t.Errorf("lf_email not lowercased: %q", got)
				}
				if tc.wantEmailVal != "" && got != tc.wantEmailVal {
					t.Errorf("lf_email=%q, want %q", got, tc.wantEmailVal)
				}
			}
			if _, hasDate := svc.updates["date_modified"]; !hasDate {
				t.Error("date_modified must always be present when updating")
			}
			if result == nil {
				t.Error("result must not be nil on success")
			}
		})
	}
}
