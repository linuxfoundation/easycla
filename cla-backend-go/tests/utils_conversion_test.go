// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

// TestValidCompanyName is a test for the GetNilSliceIfEmpty
func TestGetNilSliceIfEmptyWithData(t *testing.T) {
	slice := []string{"dog", "cat"}
	assert.Equal(t, []string{"dog", "cat"}, utils.GetNilSliceIfEmpty(slice), "GetNilSliceIfEmpty - With Data")
}

// TestGetNilSliceIfEmptyWithEmptySlice is a test for the GetNilSliceIfEmpty
func TestGetNilSliceIfEmptyWithEmptySlice(t *testing.T) {
	var slice []string
	assert.Nil(t, utils.GetNilSliceIfEmpty(slice), "GetNilSliceIfEmpty - Empty Slice")
}

// TestGetNilSliceIfEmptyWithNil is a test for the GetNilSliceIfEmpty
func TestGetNilSliceIfEmptyWithNil(t *testing.T) {
	assert.Nil(t, utils.GetNilSliceIfEmpty(nil), "GetNilSliceIfEmpty - Nil")
}
