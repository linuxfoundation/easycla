// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

func TestTrimSpaceFromItems(t *testing.T) {
	testInputs := [][]string{
		nil,
		{},
		{"  "},
		{" test1"},
		{"test1 "},
		{" test1      "},
		{" test 1      "},
		{" test    1      "},
		{" test1", "test 2 ", " test  3 "},
	}
	expectedResults := [][]string{
		nil,
		{},
		{""},
		{"test1"},
		{"test1"},
		{"test1"},
		{"test1"},
		{"test1"},
		{"test1", "test 2", "test  3"},
	}

	for i := range testInputs {
		assert.ObjectsAreEqualValues(expectedResults[i], utils.TrimSpaceFromItems(testInputs[i]))
	}
}

func TestGetFirstAndLastName(t *testing.T) {

	testInputs := []string{
		"",
		"John",
		"John Smith",
		"John        Smith",
		"John Harold Smith",
		"John       Harold         Smith",
		"John Harold Zeek Smith",
	}
	expectedResults := [][]string{
		{"", ""},
		{"John", ""},
		{"John", "Smith"},
		{"John", "Smith"},
		{"John", "Smith"},
		{"John", "Smith"},
		{"John", "Smith"},
	}

	for i := range testInputs {
		firstName, lastName := utils.GetFirstAndLastName(testInputs[i])
		assert.Equal(t, expectedResults[i][0], firstName)
		assert.Equal(t, expectedResults[i][1], lastName)
	}
}
