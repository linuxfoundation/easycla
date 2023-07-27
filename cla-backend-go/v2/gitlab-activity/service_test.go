// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_activity

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/xanzy/go-gitlab"
)

const enabled = false //nolint

func TestIsUserApprovedForSignature(t *testing.T) {
	if enabled {
		userModel := &models.User{
			Emails: []string{
				"one@example.com",
				"two@bar.com",
			},
		}
		gitlabUser := &gitlab.User{
			Username: "one",
		}

		testCases := []struct {
			name      string
			signature *models.Signature
			expected  bool
		}{
			{
				name:      "nothing matched",
				signature: &models.Signature{},
			},
			{
				name: "email approval list non empty no match",
				signature: &models.Signature{
					EmailApprovalList: []string{"three@example.com"},
				},
			},
			{
				name: "email approval list match",
				signature: &models.Signature{
					EmailApprovalList: []string{"one@example.com"},
				},
				expected: true,
			},
			{
				name: "domain approval list match no match",
				signature: &models.Signature{
					DomainApprovalList: []string{"*.foo.com"},
				},
				expected: false,
			},
			{
				name: "domain approval list match domain star",
				signature: &models.Signature{
					DomainApprovalList: []string{"*.example.com"},
				},
				expected: true,
			},
			{
				name: "domain approval list match domain star globbing",
				signature: &models.Signature{
					DomainApprovalList: []string{"*example.com"},
				},
				expected: true,
			},
			{
				name: "domain approval list match domain star dot",
				signature: &models.Signature{
					DomainApprovalList: []string{".example.com"},
				},
				expected: true,
			},
			{
				name: "gitlab username approval list no match",
				signature: &models.Signature{
					GitlabUsernameApprovalList: []string{"two"},
				},
				expected: false,
			},
			{
				name: "gitlab username approval list match",
				signature: &models.Signature{
					GitlabUsernameApprovalList: []string{"one"},
				},
				expected: true,
			},
		}
		activityService := NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil)

		for _, tc := range testCases {
			t.Run(tc.name, func(tt *testing.T) {
				result := activityService.IsUserApprovedForSignature(context.Background(), logrus.Fields{}, tc.signature, userModel, gitlabUser)
				if tc.expected {
					assert.True(tt, result)

				} else {
					assert.False(tt, result)
				}
			})
		}
	}

}

func TestPrepareMrCommentContent(t *testing.T) {
	if enabled {
		signedContains := ":white_check_mark: %s"
		missingUserContains := ":x: The commit associated with %s is missing the User's ID"
		missingAffiliationContains := "%s is authorized, but they must confirm their affiliation"
		missingApprovalContains := "%s's commit is not authorized under a signed CLA"

		testCases := []struct {
			name          string
			signed        []*gitlab.User
			missing       []*utils.GatedGitlabUser
			expectedMsgs  []string
			expectedBadge string
		}{
			{
				name: "all signed",
				signed: []*gitlab.User{
					{ID: 1, Username: "neo"},
					{ID: 2, Username: "oracle"},
				},
				expectedMsgs:  []string{signedContains, signedContains},
				expectedBadge: "cla-signed.svg",
			},
			{
				name: "missing id",
				signed: []*gitlab.User{
					{ID: 1, Username: "neo"},
				},
				missing: []*utils.GatedGitlabUser{
					{Err: missingID, User: &gitlab.User{ID: 3, Username: "missing"}},
				},
				expectedMsgs:  []string{signedContains, missingUserContains},
				expectedBadge: "cla-missing-id.svg",
			},
			{
				name: "missing affiliation",
				signed: []*gitlab.User{
					{ID: 1, Username: "neo"},
				},
				missing: []*utils.GatedGitlabUser{
					{Err: missingCompanyAffiliation, User: &gitlab.User{ID: 4, Username: "affiliationUser"}},
				},
				expectedMsgs:  []string{signedContains, missingAffiliationContains},
				expectedBadge: "cla-confirmation-needed.svg",
			},
			{
				name: "missing approval",
				signed: []*gitlab.User{
					{ID: 1, Username: "neo"},
				},
				missing: []*utils.GatedGitlabUser{
					{Err: missingCompanyApproval, User: &gitlab.User{ID: 5, Username: "approvalUser"}},
				},
				expectedMsgs:  []string{signedContains, missingApprovalContains},
				expectedBadge: "cla-not-signed.svg",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(tt *testing.T) {
				result := utils.PrepareMrCommentContent(tc.missing, tc.signed, "https://sign.com")
				tt.Logf("the result is : %s", result)
				parts := strings.Split(result, "<li>")
				assert.Len(tt, parts, len(tc.expectedMsgs)+1)

				var allUsers []*gitlab.User

				if len(tc.signed) > 0 {
					for _, s := range tc.signed {
						allUsers = append(allUsers, s)
					}
				}

				if len(tc.missing) > 0 {
					for _, m := range tc.missing {
						allUsers = append(allUsers, m.User)
					}
				}

				for i, p := range parts[1:] {
					expected := fmt.Sprintf(tc.expectedMsgs[i], utils.GetAuthorInfo(allUsers[i]))
					assert.Contains(tt, p, expected)
				}

				assert.Contains(tt, result, tc.expectedBadge)
			})
		}
	}

}
