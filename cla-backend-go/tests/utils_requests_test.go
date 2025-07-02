// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"testing"

	"github.com/gofrs/uuid"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

func TestValidGetRequestID(t *testing.T) {
	uuid, err := uuid.NewV4()
	assert.Nil(t, err, "Create UUID v4")
	uuidStr := uuid.String()
	assert.Equal(t, uuidStr, utils.GetRequestID(&uuidStr))
}

func TestEmptyGetRequestID(t *testing.T) {
	assert.Equal(t, "", utils.GetRequestID(nil))
}
