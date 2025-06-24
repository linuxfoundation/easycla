// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"testing"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/stretchr/testify/assert"
)

func TestUserIsApproved(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name               string
		user               *v1Models.User
		cclaSignature      *v1Models.Signature
		expectedIsApproved bool
	}{
		{
			name: "User in GitHub username approval list",
			user: &v1Models.User{
				GithubUsername: "approved-user",
			},
			cclaSignature: &v1Models.Signature{
				GithubUsernameApprovalList: []string{"approved-user"},
			},
			expectedIsApproved: true,
		},
		{
			name: "User not in GitHub username approval list",
			user: &v1Models.User{
				GithubUsername: "unapproved-user",
			},
			cclaSignature: &v1Models.Signature{
				GithubUsernameApprovalList: []string{"approved-user"},
			},
			expectedIsApproved: false,
		},
		{
			name: "User in Email approval list",
			user: &v1Models.User{
				Emails: []string{"foo@gmail.com"},
			},
			cclaSignature: &v1Models.Signature{
				EmailApprovalList: []string{"foo@gmail.com"},
			},
			expectedIsApproved: true,
		},
		{
			name: "User not in Email approval list",
			user: &v1Models.User{
				Emails: []string{"unapproved@gmail.com"},
			},
			cclaSignature: &v1Models.Signature{
				EmailApprovalList: []string{"approved@gmail.com"},
			},
			expectedIsApproved: false,
		},
		{
			name: "User in Domain approval list",
			user: &v1Models.User{
				Emails: []string{"approved@samsung.com"},
			},
			cclaSignature: &v1Models.Signature{
				DomainApprovalList: []string{"samsung.com"},
			},
			expectedIsApproved: true,
		},
		{
			name: "Test user email case - email approval",
			user: &v1Models.User{
				Emails: []string{"Foo@gmail.com"},
			},
			cclaSignature: &v1Models.Signature{
				EmailApprovalList: []string{"foo@gmail.com"},
			},
			expectedIsApproved: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewService(nil, nil, nil, nil, false, nil, nil, nil, nil, "", "", "")

			isApproved, err := service.UserIsApproved(ctx, tc.user, tc.cclaSignature)

			assert.Nil(t, err)
			assert.Equal(t, tc.expectedIsApproved, isApproved)
		})
	}
}
