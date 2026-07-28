// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package users_test

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	mock_user_repo "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/stretchr/testify/assert"
)

func TestGetUsersByIdentity(t *testing.T) {
	t.Run("unions matches across all three key types", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_user_repo.NewMockUserRepository(ctrl)

		repo.EXPECT().GetUserByLFUserName("alice").Return(&models.User{UserID: "u-name"}, nil)
		repo.EXPECT().GetUserByEmail("a@x.org").Return(&models.User{UserID: "u-email"}, nil)
		repo.EXPECT().GetUsersByLFEmail("a@x.org").Return([]*models.User{{UserID: "u-lfemail"}}, nil)
		repo.EXPECT().GetUserByGitHubID("13434323").Return(&models.User{UserID: "u-gh"}, nil)

		svc := users.NewService(repo, nil)
		got, err := svc.GetUsersByIdentity("alice", []string{"a@x.org"}, []string{"13434323"})

		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"u-name", "u-email", "u-lfemail", "u-gh"}, userIDs(got))
	})

	t.Run("dedupes the same user matched by multiple keys", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_user_repo.NewMockUserRepository(ctrl)

		same := &models.User{UserID: "u-1"}
		repo.EXPECT().GetUserByLFUserName("alice").Return(same, nil)
		repo.EXPECT().GetUserByEmail("a@x.org").Return(same, nil)
		repo.EXPECT().GetUsersByLFEmail("a@x.org").Return([]*models.User{same}, nil)
		repo.EXPECT().GetUserByGitHubID("42").Return(same, nil)

		svc := users.NewService(repo, nil)
		got, err := svc.GetUsersByIdentity("alice", []string{"a@x.org"}, []string{"42"})

		assert.NoError(t, err)
		assert.Equal(t, []string{"u-1"}, userIDs(got))
	})

	t.Run("skips a key whose lookup errors and still returns the others", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_user_repo.NewMockUserRepository(ctrl)

		// GitHub lookup errors (e.g. not found) — must not fail the whole union.
		repo.EXPECT().GetUserByLFUserName("alice").Return(&models.User{UserID: "u-name"}, nil)
		repo.EXPECT().GetUserByGitHubID("99").Return(nil, errors.New("not found"))

		svc := users.NewService(repo, nil)
		got, err := svc.GetUsersByIdentity("alice", nil, []string{"99"})

		assert.NoError(t, err)
		assert.Equal(t, []string{"u-name"}, userIDs(got))
	})

	t.Run("empty input returns an empty, non-nil slice and makes no lookups", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_user_repo.NewMockUserRepository(ctrl) // no EXPECT — asserts zero calls

		svc := users.NewService(repo, nil)
		got, err := svc.GetUsersByIdentity("", nil, nil)

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("skips blank/whitespace keys without a lookup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_user_repo.NewMockUserRepository(ctrl)

		// Only the non-blank github id triggers a lookup; blank username/email are skipped.
		repo.EXPECT().GetUserByGitHubID("7").Return(&models.User{UserID: "u-gh"}, nil)

		svc := users.NewService(repo, nil)
		got, err := svc.GetUsersByIdentity("", []string{"", "   "}, []string{"7", " "})

		assert.NoError(t, err)
		assert.Equal(t, []string{"u-gh"}, userIDs(got))
	})

	t.Run("drops records with an empty user ID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_user_repo.NewMockUserRepository(ctrl)

		repo.EXPECT().GetUserByLFUserName("ghost").Return(&models.User{UserID: ""}, nil)

		svc := users.NewService(repo, nil)
		got, err := svc.GetUsersByIdentity("ghost", nil, nil)

		assert.NoError(t, err)
		assert.Empty(t, got)
	})
}

func userIDs(us []*models.User) []string {
	ids := make([]string, 0, len(us))
	for _, u := range us {
		ids = append(ids, u.UserID)
	}
	return ids
}
