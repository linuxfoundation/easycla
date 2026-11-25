// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package users

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	mock_events "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddUserEmailsRecordsTheUserUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, log := mock_users.NewMockUserRepository(ctrl), mock_events.NewMockService(ctrl)
	svc := NewService(repo, log)

	repo.EXPECT().AddUserEmails("user-1", []string{"gh@example.org"}).Return(&models.User{UserID: "user-1"}, nil)
	log.EXPECT().LogEvent(gomock.Any()).Do(func(args *events.LogEventArgs) {
		assert.Equal(t, events.UserUpdated, args.EventType)
		assert.Equal(t, "user-1", args.UserID)
	})
	user, err := svc.AddUserEmails("user-1", []string{"gh@example.org"})
	require.NoError(t, err)
	assert.Equal(t, "user-1", user.UserID)

	repo.EXPECT().AddUserEmails("user-1", []string{"gh@example.org"}).Return(nil, errors.New("conditional check failed"))
	_, err = svc.AddUserEmails("user-1", []string{"gh@example.org"})
	require.Error(t, err, "a failed update records no event")

	repo.EXPECT().AddUserEmails("user-1", nil).Return(&models.User{UserID: "user-1"}, nil)
	_, err = svc.AddUserEmails("user-1", nil)
	require.NoError(t, err, "nothing to add records no event")
}
