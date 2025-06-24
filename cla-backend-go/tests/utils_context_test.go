// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"context"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

// TestGetUserNameFromContext is a test for the GetUserNameFromContext
func TestGetUserNameFromContext(t *testing.T) {
	reqID := "foo123"
	authUser := &auth.User{
		UserName: "ddeal1",
		Email:    "ddeal1@foo.com",
		ACL:      auth.ACL{},
	}
	ctx := utils.ContextWithRequestAndUser(context.Background(), reqID, authUser) // nolint
	assert.Equal(t, "ddeal1", utils.GetUserNameFromContext(ctx))
}

// TestGetUserEmailFromContext is a test for the GetUserNameFromContext
func TestGetUserEmailFromContext(t *testing.T) {
	reqID := "foo566"
	authUser := &auth.User{
		UserName: "ddeal2",
		Email:    "ddeal2@foo.com",
		ACL:      auth.ACL{},
	}
	ctx := utils.ContextWithRequestAndUser(context.Background(), reqID, authUser) // nolint
	assert.Equal(t, "ddeal2@foo.com", utils.GetUserEmailFromContext(ctx))
}
