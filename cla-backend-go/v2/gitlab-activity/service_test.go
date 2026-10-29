// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_activity

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
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
		activityService := NewService(nil, nil, nil, nil, nil, nil, nil, nil)

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
			missing       []*gatedGitlabUser
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
				missing: []*gatedGitlabUser{
					{err: missingID, User: &gitlab.User{ID: 3, Username: "missing"}},
				},
				expectedMsgs:  []string{signedContains, missingUserContains},
				expectedBadge: "cla-missing-id.svg",
			},
			{
				name: "missing affiliation",
				signed: []*gitlab.User{
					{ID: 1, Username: "neo"},
				},
				missing: []*gatedGitlabUser{
					{err: missingCompanyAffiliation, User: &gitlab.User{ID: 4, Username: "affiliationUser"}},
				},
				expectedMsgs:  []string{signedContains, missingAffiliationContains},
				expectedBadge: "cla-confirmation-needed.svg",
			},
			{
				name: "missing approval",
				signed: []*gitlab.User{
					{ID: 1, Username: "neo"},
				},
				missing: []*gatedGitlabUser{
					{err: missingCompanyApproval, User: &gitlab.User{ID: 5, Username: "approvalUser"}},
				},
				expectedMsgs:  []string{signedContains, missingApprovalContains},
				expectedBadge: "cla-not-signed.svg",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(tt *testing.T) {
				result := PrepareMrCommentContent(tc.missing, tc.signed, "https://sign.com")
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
					expected := fmt.Sprintf(tc.expectedMsgs[i], getAuthorInfo(allUsers[i]))
					assert.Contains(tt, p, expected)
				}

				assert.Contains(tt, result, tc.expectedBadge)
			})
		}
	}

}

// author text is HTML-escaped where it enters the merge-request note (ops#826): markup in a GitLab name cannot
// inject tags into the note, benign names render unchanged, the sign link is untouched
func TestPrepareMrCommentContentEscapesAuthorText(t *testing.T) {
	signed := []*gitlab.User{{ID: 1, Username: `neo"`, Name: `<img src=x onerror=alert(1)>`}}
	missing := []*gatedGitlabUser{{err: fmt.Errorf("not authorized"), User: &gitlab.User{ID: 2, Username: "trinity", Name: "Trinity O'Neil & Co"}}}
	body := PrepareMrCommentContent(missing, signed, "https://sign.example/s?a=1")

	items := regexp.MustCompile(`(?s)<li>(.*?)</li>`).FindAllStringSubmatch(body, -1)
	if assert.Len(t, items, 2, body) {
		assert.Contains(t, items[0][1], "login:@neo&#34;/name:&lt;img src=x onerror=alert(1)&gt;")
		assert.NotContains(t, items[0][1], "<img")
		assert.Contains(t, items[1][1], "login:@trinity/name:Trinity O&#39;Neil &amp; Co")
		assert.Contains(t, items[1][1], "<a href='https://sign.example/s?a=1' target='_blank'>")
	}
	assert.NotContains(t, body, "<img src=x")
}
