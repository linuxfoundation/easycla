// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNonNoReplyUserEmail(t *testing.T) {
	testCases := []struct {
		name        string
		emails      []string
		resultEmail string
	}{
		{
			name:        "empty emails",
			emails:      []string{},
			resultEmail: "",
		},
		{
			name: "single noreply email",
			emails: []string{
				"single@users.noreply.github.com",
			},
			resultEmail: "single@users.noreply.github.com",
		},
		{
			name: "multiple emails with noreply",
			emails: []string{
				"single@users.noreply.github.com",
				"pumacat@gmail.com",
			},
			resultEmail: "pumacat@gmail.com",
		},
		{
			name: "multiple emails without noreply",
			emails: []string{
				"pumacat@gmail.com",
				"pumacat2@gmail.com",
			},
			resultEmail: "pumacat@gmail.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			result := GetNonNoReplyUserEmail(tc.emails)
			assert.Equal(tt, tc.resultEmail, result)
		})
	}
}
