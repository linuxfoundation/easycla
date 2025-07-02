// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

/*
import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUserRepo(t *testing.T) {
	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, fmt.Sprintf("Unable to load AWS session - Error: %v", err))
	}
	theUserID, err := uuid.NewUUID()
	if err != nil {
		assert.Fail(t, fmt.Sprintf("Unable to create a new UUIDv4 - Error: %v", err))
	}
	theExternalID, err := uuid.NewUUID()
	if err != nil {
		assert.Fail(t, fmt.Sprintf("Unable to create a new UUIDv4 - Error: %v", err))
	}
	theGitHubID, err := uuid.NewUUID()
	if err != nil {
		assert.Fail(t, fmt.Sprintf("Unable to create a new UUIDv4 - Error: %v", err))
	}
	usersRepo := NewRepository(awsSession, "dev")
	user, err := usersRepo.CreateUser(&models.User{
		UserID:         theUserID.String(),
		Username:       "Unit Test User",
		UserExternalID: theExternalID.String(),
		LfUsername:     "unit_test_user",
		Admin:          false,
		GithubID:       theGitHubID.String(),
		GithubUsername: "Unit Test GH Username",
		Note:           "test user note",
	})
	if err != nil {
		assert.Fail(t, fmt.Sprintf("Unable to create a new user - Error: %v", err))
	}

	assert.Same(t, theUserID, uuid.MustParse(user.UserID))
	assert.Same(t, aws.String("Unit Test User"), &user.Username)
	assert.Same(t, theExternalID, uuid.MustParse(user.UserExternalID))
	assert.Same(t, aws.String("unit_test_user"), &user.LfUsername)
	assert.False(t, user.Admin)
	assert.Same(t, theGitHubID, uuid.MustParse(user.GithubID))
	assert.Same(t, aws.String("Unit Test GH Username"), &user.GithubUsername)
	assert.Same(t, aws.String("test user note"), &user.Note)
}
*/
