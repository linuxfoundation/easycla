// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"testing"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	githublib "github.com/linuxfoundation/easycla/cla-backend-go/github"
	"github.com/stretchr/testify/assert"
)

// stubListUserPublicOrgs replaces the package-level listUserPublicOrgs hook
// for the duration of a test, restoring the real GitHub client wrapper on
// cleanup. Returning the recorded user lets callers assert the value passed
// in.
func stubListUserPublicOrgs(t *testing.T, orgs []string, err error) *string {
	t.Helper()
	original := listUserPublicOrgs
	var captured string
	listUserPublicOrgs = func(_ context.Context, user string) ([]string, error) {
		captured = user
		return orgs, err
	}
	t.Cleanup(func() { listUserPublicOrgs = original })
	return &captured
}

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

// TestUserIsApproved_GithubOrgApprovalList covers the regression that caused
// the post-cutover EasyCLA outage: the original Go port called
// /orgs/<org>/memberships/<user> (403 for the bot), so all org-approved
// contributors were silently rejected. The fix calls
// github.ListUserPublicOrgs (GET /users/<user>/orgs) and compares
// case-insensitively against the approval list, matching the pre-cutover
// Python behavior. These cases lock in that contract.
func TestUserIsApproved_GithubOrgApprovalList(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name           string
		username       string
		approvalList   []string
		userOrgs       []string
		listErr        error
		wantApproved   bool
		wantListCalled bool
		wantUser       string
	}{
		{
			name:           "exact-case match",
			username:       "alice",
			approvalList:   []string{"acme"},
			userOrgs:       []string{"acme"},
			wantApproved:   true,
			wantListCalled: true,
			wantUser:       "alice",
		},
		{
			name:           "user-org casing differs from approval list",
			username:       "bob",
			approvalList:   []string{"sap-cloudfoundry"},
			userOrgs:       []string{"SAP", "sap-cloudfoundry", "sap-contributions"},
			wantApproved:   true,
			wantListCalled: true,
			wantUser:       "bob",
		},
		{
			name:           "approval-list casing differs from user orgs",
			username:       "carol",
			approvalList:   []string{"PIVOTAL-CF"},
			userOrgs:       []string{"pivotal-cf", "vmware"},
			wantApproved:   true,
			wantListCalled: true,
			wantUser:       "carol",
		},
		{
			name:           "approval list contains whitespace around org",
			username:       "dave",
			approvalList:   []string{"  morganstanley  "},
			userOrgs:       []string{"morganstanley"},
			wantApproved:   true,
			wantListCalled: true,
			wantUser:       "dave",
		},
		{
			name:           "no overlap between user orgs and approval list",
			username:       "eve",
			approvalList:   []string{"acme"},
			userOrgs:       []string{"contoso", "initech"},
			wantApproved:   false,
			wantListCalled: true,
			wantUser:       "eve",
		},
		{
			name:           "user has no public orgs",
			username:       "frank",
			approvalList:   []string{"acme"},
			userOrgs:       nil,
			wantApproved:   false,
			wantListCalled: true,
			wantUser:       "frank",
		},
		{
			name:           "github API error treated as no match",
			username:       "grace",
			approvalList:   []string{"acme"},
			listErr:        errors.New("simulated 502 from github"),
			wantApproved:   false,
			wantListCalled: true,
			wantUser:       "grace",
		},
		{
			name:           "github username trimmed before lookup",
			username:       "  henry  ",
			approvalList:   []string{"acme"},
			userOrgs:       []string{"acme"},
			wantApproved:   true,
			wantListCalled: true,
			wantUser:       "henry",
		},
		{
			name:           "whitespace-only username never reaches github",
			username:       "   ",
			approvalList:   []string{"acme"},
			wantApproved:   false,
			wantListCalled: false,
		},
		{
			name:           "empty approval list never queries github",
			username:       "ivy",
			approvalList:   []string{},
			wantApproved:   false,
			wantListCalled: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			captured := stubListUserPublicOrgs(t, tc.userOrgs, tc.listErr)
			// Sentinel pre-test state: an empty captured value means the
			// stub was never invoked.
			*captured = ""

			svc := NewService(nil, nil, nil, nil, false, nil, nil, nil, nil, "", "", "")
			user := &v1Models.User{GithubUsername: tc.username}
			ccla := &v1Models.Signature{GithubOrgApprovalList: tc.approvalList}

			ok, err := svc.UserIsApproved(ctx, user, ccla)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantApproved, ok)
			if tc.wantListCalled {
				assert.Equal(t, tc.wantUser, *captured, "ListUserPublicOrgs called with wrong user")
			} else {
				assert.Equal(t, "", *captured, "ListUserPublicOrgs should not have been called")
			}
		})
	}
}

// TestEvaluateUserApproval covers the one thing the UserIsApproved boolean cannot express:
// a false result caused by a failed GitHub public-orgs lookup ("could not tell") rather than
// by a genuine approval-list miss. Callers that must not present a guess to the user - the
// My CLAs listing - key off this second return value.
func TestEvaluateUserApproval(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name             string
		user             *v1Models.User
		ccla             *v1Models.Signature
		userOrgs         []string
		listErr          error
		wantApproved     bool
		wantLookupFailed bool
	}{
		{
			name:         "org match",
			user:         &v1Models.User{GithubUsername: "alice"},
			ccla:         &v1Models.Signature{GithubOrgApprovalList: []string{"acme"}},
			userOrgs:     []string{"acme"},
			wantApproved: true,
		},
		{
			name:     "no overlap is a genuine miss",
			user:     &v1Models.User{GithubUsername: "eve"},
			ccla:     &v1Models.Signature{GithubOrgApprovalList: []string{"acme"}},
			userOrgs: []string{"contoso"},
		},
		{
			name:             "lookup failure is unevaluable, not a miss",
			user:             &v1Models.User{GithubUsername: "grace"},
			ccla:             &v1Models.Signature{GithubOrgApprovalList: []string{"acme"}},
			listErr:          errors.New("simulated 502 from github"),
			wantLookupFailed: true,
		},
		{
			name:         "approved by email before github is consulted",
			user:         &v1Models.User{GithubUsername: "heidi", Emails: []string{"heidi@acme.org"}},
			ccla:         &v1Models.Signature{EmailApprovalList: []string{"heidi@acme.org"}, GithubOrgApprovalList: []string{"acme"}},
			listErr:      errors.New("would fail if reached"),
			wantApproved: true,
		},
		{
			name: "no approval list at all",
			user: &v1Models.User{GithubUsername: "ivan"},
			ccla: &v1Models.Signature{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubListUserPublicOrgs(t, tc.userOrgs, tc.listErr)
			svc := NewService(nil, nil, nil, nil, false, nil, nil, nil, nil, "", "", "")

			approved, lookupFailed, err := svc.EvaluateUserApproval(ctx, tc.user, tc.ccla)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantApproved, approved)
			assert.Equal(t, tc.wantLookupFailed, lookupFailed)

			// UserIsApproved must stay byte-identical for the signing flow
			legacyApproved, legacyErr := svc.UserIsApproved(ctx, tc.user, tc.ccla)
			assert.NoError(t, legacyErr)
			assert.Equal(t, approved, legacyApproved)
		})
	}
}

// TestListUserPublicOrgs_RejectsEmptyUser guards the public helper itself:
// go-github routes an empty user string to GET /user/orgs (the authenticated
// bot's own orgs), so an empty argument must never silently succeed.
func TestListUserPublicOrgs_RejectsEmptyUser(t *testing.T) {
	for _, in := range []string{"", "   ", "\t\n"} {
		got, err := githublib.ListUserPublicOrgs(context.Background(), in)
		assert.Nil(t, got, "expected nil slice for input %q", in)
		assert.Error(t, err, "expected error for input %q", in)
	}
}
