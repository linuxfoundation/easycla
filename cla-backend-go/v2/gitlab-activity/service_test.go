// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_activity

import (
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/xanzy/go-gitlab"
	"testing"
)

func TestIsUserApprovedForSignature(t *testing.T) {
	userModel := &models.User{
		Emails: []string{
			"one@example.com",
			"two@bar.com",
		},
	}
	gitlabUser := &gitlab.User{
		Username: "one",
	}

	testCases := []struct{
		name string
		signature *models.Signature
		expected bool
	}{
		{
			name: "nothing matched",
			signature : &models.Signature{},
		},
		{
			name: "email approval list non empty no match",
			signature : &models.Signature{
				EmailApprovalList: []string{"three@example.com"},
			},
		},
		{
			name: "email approval list match",
			signature : &models.Signature{
				EmailApprovalList: []string{"one@example.com"},
			},
			expected: true,
		},
		{
			name: "domain approval list match no match",
			signature : &models.Signature{
				DomainApprovalList: []string{"*.foo.com"},
			},
			expected: false,
		},
		{
			name: "domain approval list match domain star",
			signature : &models.Signature{
				DomainApprovalList: []string{"*.example.com"},
			},
			expected: true,
		},
		{
			name: "domain approval list match domain star globbing",
			signature : &models.Signature{
				DomainApprovalList: []string{"*example.com"},
			},
			expected: true,
		},
		{
			name: "domain approval list match domain star dot",
			signature : &models.Signature{
				DomainApprovalList: []string{".example.com"},
			},
			expected: true,
		},
		{
			name: "gitlab username approval list no match",
			signature : &models.Signature{
				GitlabUsernameApprovalList: []string{"two"},
			},
			expected: false,
		},
		{
			name: "gitlab username approval list match",
			signature : &models.Signature{
				GitlabUsernameApprovalList: []string{"one"},
			},
			expected: true,
		},
	}

	for _, tc := range testCases{
		t.Run(tc.name, func(tt *testing.T) {
			result := IsUserApprovedForSignature(logrus.Fields{}, tc.signature, userModel, gitlabUser)
			if tc.expected{
				assert.True(tt, result)

			}else{
				assert.False(tt, result)
			}
		})
	}


}