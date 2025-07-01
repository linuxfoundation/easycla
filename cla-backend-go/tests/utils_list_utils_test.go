// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

func TestFindInt64Duplicates(t *testing.T) {
	type TestCase struct {
		A        []int64
		B        []int64
		Expected []int64
	}
	testInputs := []TestCase{
		{nil, nil, []int64{}},
		{nil, []int64{}, []int64{}},
		{[]int64{}, nil, []int64{}},
		{nil, nil, []int64{}},
		{[]int64{}, []int64{}, []int64{}},
		{[]int64{1}, []int64{}, []int64{}},
		{[]int64{}, []int64{1}, []int64{}},
		{[]int64{1, 2}, []int64{}, []int64{}},
		{[]int64{}, []int64{1, 2}, []int64{}},
		{[]int64{1}, []int64{1}, []int64{1}},
		{[]int64{1, 2}, []int64{1}, []int64{1}},
		{[]int64{1, 2}, []int64{1, 3, 4}, []int64{1}},
		{[]int64{1, 2, 3, 4, 5}, []int64{1, 5, 3, 4}, []int64{1, 3, 5, 4}},
	}

	for _, testInput := range testInputs {
		assert.ElementsMatch(t, testInput.Expected, utils.FindInt64Duplicates(testInput.A, testInput.B))
	}
}
