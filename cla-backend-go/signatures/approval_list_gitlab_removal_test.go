// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/go-openapi/strfmt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	mock_events "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

const (
	removedGroupURL = "https://gitlab.com/groups/removed-group"
	keptGroupURL    = "https://gitlab.com/groups/kept-group"
)

func gitLabRemovalSession(t *testing.T, table *fakeSignaturesTable) *session.Session {
	server := httptest.NewServer(table)
	t.Cleanup(server.Close)
	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(server.URL),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)
	return awsSession
}

func stubGitLabGroupMembers(t *testing.T, resolver func(ctx context.Context, group string) ([]string, error)) {
	previous := gitLabGroupMembers
	gitLabGroupMembers = resolver
	t.Cleanup(func() { gitLabGroupMembers = previous })
}

// removing a GitLab group re-checks its members against the remaining criteria, like a GitHub
// organization removal does
func TestVerifyUserApprovalsGitLabGroupRemovalHonorsRemainingCoverage(t *testing.T) {
	members := []string{"jane.gl", "otherdev"}
	tests := []struct {
		name        string
		user        *models.User
		approvals   ApprovalList
		publicOrgs  []string
		invalidated bool
	}{
		{name: "member without remaining coverage", user: &models.User{UserID: "user-1", GitlabUsername: "jane.gl", Emails: []string{"jane@corp.example"}}, invalidated: true},
		{name: "member listed with another case", user: &models.User{UserID: "user-1", GitlabUsername: "Jane.GL"}, invalidated: true},
		{name: "member still covered by a domain entry", user: &models.User{UserID: "user-1", GitlabUsername: "jane.gl", Emails: []string{"jane@corp.example"}}, approvals: ApprovalList{DomainApprovals: []string{"corp.example"}}},
		{name: "member still covered by an email entry", user: &models.User{UserID: "user-1", GitlabUsername: "jane.gl", LfEmail: "jane@corp.example"}, approvals: ApprovalList{EmailApprovals: []string{"jane@corp.example"}}},
		{name: "member still covered by a GitLab username entry", user: &models.User{UserID: "user-1", GitlabUsername: "jane.gl"}, approvals: ApprovalList{GitlabUsernameApprovals: []string{"Jane.GL"}}},
		{name: "member still covered by a GitHub username entry", user: &models.User{UserID: "user-1", GitlabUsername: "jane.gl", GithubUsername: "janegh"}, approvals: ApprovalList{GitHubUsernameApprovals: []string{"janegh"}}},
		{name: "member still covered by an approved GitHub organization", user: &models.User{UserID: "user-1", GitlabUsername: "jane.gl", GithubUsername: "janegh"}, approvals: ApprovalList{GitHubOrgApprovals: []string{"kept-org"}}, publicOrgs: []string{"Kept-Org"}},
		{name: "not a member of the removed group", user: &models.User{UserID: "user-1", GitlabUsername: "someone"}},
		{name: "no GitLab username", user: &models.User{UserID: "user-1", GithubUsername: "janegh"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			table := &fakeSignaturesTable{items: []map[string]interface{}{{"signature_id": fakeS("sig-1"), "signature_approved": fakeTrue()}}, invalidated: map[string]int{}}
			awsSession := gitLabRemovalSession(t, table)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockUsers := mock_users.NewMockUserRepository(ctrl)
			mockUsers.EXPECT().GetUser("user-1").Return(test.user, nil)
			stubListUserPublicOrgs(t, test.publicOrgs, nil)

			repo := repository{
				stage:              "test",
				dynamoDBClient:     dynamodb.New(awsSession),
				usersRepo:          mockUsers,
				signatureTableName: "cla-test-signatures",
			}
			approvals := test.approvals
			approvals.Criteria = utils.GitlabOrgCriteria
			approvals.ApprovalList = []string{removedGroupURL}
			approvals.GitlabUsernames = members

			user, invalidated, err := repo.verifyUserApprovals(context.Background(), "user-1", "sig-1", &models.User{LfUsername: "manager-lf"}, &approvals)
			require.NoError(t, err)
			assert.Same(t, test.user, user)
			assert.Equal(t, test.invalidated, invalidated)

			table.mu.Lock()
			defer table.mu.Unlock()
			expectedWrites := 0
			if test.invalidated {
				expectedWrites = 1
				assert.Equal(t, "Signature invalidated (approved set to false) by manager-lf due to Gitlab Org Criteria  removal", fakeItemString(table.find("sig-1"), "note"))
				assert.Equal(t, "approved list removal (Gitlab Org Criteria)", fakeItemString(table.find("sig-1"), "invalidation_reason"))
			}
			assert.Equal(t, expectedWrites, table.invalidated["sig-1"])
		})
	}
}

func gitLabRemovalItems(t *testing.T, logins map[string]string) []map[string]interface{} {
	t.Helper()
	items := []map[string]interface{}{{
		"signature_id":             fakeS("ccla-sig"),
		"signature_project_id":     fakeS("cla-group-1"),
		"signature_reference_id":   fakeS("company-1"),
		"signature_reference_type": fakeS("company"),
		"signature_reference_name": fakeS("Acme"),
		"signature_type":           fakeS("ccla"),
		"signature_approved":       fakeTrue(),
		"signature_signed":         fakeTrue(),
		"gitlab_org_approval_list": fakeStringList(removedGroupURL, keptGroupURL),
		"email_whitelist":          fakeStringList("dave@corp.example"),
		"signature_acl":            fakeStringList("manager-lf"),
		"date_created":             fakeS("2023-01-01T00:00:00Z"),
		"date_modified":            fakeS("2023-01-01T00:00:00Z"),
	}}
	for i := 1; i <= len(logins); i++ {
		item := fakeEclaItem(i, fmt.Sprintf("%s@corp.example", logins[fmt.Sprintf("user-%03d", i)]))
		if i == 3 {
			item["signature_approved"] = map[string]interface{}{"BOOL": false}
			item["note"] = fakeS("Signature invalidated (approved set to false) by admin due to Email  removal")
		}
		items = append(items, item)
	}
	return items
}

func gitLabRemovalRepo(t *testing.T, ctrl *gomock.Controller, awsSession *session.Session, logins map[string]string) repository {
	t.Helper()
	mockUsers := mock_users.NewMockUserRepository(ctrl)
	mockUsers.EXPECT().GetUser(gomock.Any()).DoAndReturn(func(userID string) (*models.User, error) {
		login, ok := logins[userID]
		require.Truef(t, ok, "unexpected user lookup: %s", userID)
		return &models.User{UserID: userID, GitlabUsername: login, LfEmail: strfmt.Email(login + "@corp.example")}, nil
	}).AnyTimes()
	mockUsers.EXPECT().GetUserByUserName("manager-lf", true).
		Return(&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"}, nil).AnyTimes()
	mockCompanyRepo := mock_company.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompany(gomock.Any(), "company-1").
		Return(&models.Company{CompanyID: "company-1", CompanyName: "Acme"}, nil).AnyTimes()
	mockEvents := mock_events.NewMockService(ctrl)
	mockEvents.EXPECT().LogEventWithContext(gomock.Any(), gomock.Any()).AnyTimes()

	previousSender := utils.GetEmailSender()
	utils.SetEmailSender(&recordingEmailSender{})
	t.Cleanup(func() { utils.SetEmailSender(previousSender) })

	return repository{
		stage:              "test",
		dynamoDBClient:     dynamodb.New(awsSession),
		companyRepo:        mockCompanyRepo,
		usersRepo:          mockUsers,
		eventsService:      mockEvents,
		signatureTableName: "cla-test-signatures",
		approvalRepo:       &fakeApprovalRepo{},
	}
}

// TestUpdateApprovalListStandaloneGitLabGroupRemovalInvalidatesMembers: before, the GitLab group
// removal resolved the group through the GitHub clients and never loaded the company's acknowledgments,
// so nobody was re-checked. Now the group's members (through the resolver the API server installs) are
// re-checked against the remaining criteria; already invalid acknowledgments are left alone.
func TestUpdateApprovalListStandaloneGitLabGroupRemovalInvalidatesMembers(t *testing.T) {
	// user-001 alice: member (listed with another case), no other coverage -> invalidated
	// user-002 bob:   not a member                                        -> kept
	// user-003 carol: member, acknowledgment already invalid               -> not touched again
	// user-004 dave:  member, still on the email list                      -> kept
	// user-005 erin:  member, also a member of the group that stays listed -> kept
	logins := map[string]string{"user-001": "alice", "user-002": "bob", "user-003": "carol", "user-004": "dave", "user-005": "erin"}
	table := &fakeSignaturesTable{items: gitLabRemovalItems(t, logins), invalidated: map[string]int{}}
	awsSession := gitLabRemovalSession(t, table)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := gitLabRemovalRepo(t, ctrl, awsSession, logins)

	var memberLookups int64
	stubGitLabGroupMembers(t, func(_ context.Context, group string) ([]string, error) {
		atomic.AddInt64(&memberLookups, 1)
		switch group {
		case removedGroupURL:
			return []string{"Alice", "carol", "dave", "erin", "stranger"}, nil
		case keptGroupURL:
			return []string{"Erin"}, nil
		}
		return nil, fmt.Errorf("unexpected group lookup: %s", group)
	})

	updated, err := repo.UpdateApprovalList(context.Background(),
		&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"},
		&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project", Version: "v2"},
		"company-1",
		&models.ApprovalList{RemoveGitlabOrgApprovalList: []string{removedGroupURL}},
		&events.LogEventArgs{EventType: events.InvalidatedSignature})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "ccla-sig", updated.SignatureID)
	assert.Equal(t, int64(2), atomic.LoadInt64(&memberLookups), "the removed group and the group that stays listed")

	table.mu.Lock()
	defer table.mu.Unlock()
	assert.Equal(t, map[string]int{"sig-001": 1}, table.invalidated, "only alice loses her acknowledgment")
	assert.Equal(t, "Signature invalidated (approved set to false) by manager-lf due to Gitlab Org Criteria  removal", fakeItemString(table.find("sig-001"), "note"))
	assert.Equal(t, "Signature invalidated (approved set to false) by admin due to Email  removal", fakeItemString(table.find("sig-003"), "note"), "carol's earlier invalidation is untouched")
	assert.Equal(t, fakeStringList(keptGroupURL), table.find("ccla-sig")["gitlab_org_approval_list"], "the removed group left the list")
}

// a membership lookup failure stops the removal before anything is written
func TestUpdateApprovalListGitLabGroupRemovalFailsBeforeWritingWhenTheLookupFails(t *testing.T) {
	logins := map[string]string{"user-001": "alice"}
	table := &fakeSignaturesTable{items: gitLabRemovalItems(t, logins), invalidated: map[string]int{}}
	awsSession := gitLabRemovalSession(t, table)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := gitLabRemovalRepo(t, ctrl, awsSession, logins)
	stubGitLabGroupMembers(t, func(context.Context, string) ([]string, error) { return nil, errors.New("gitlab: 503") })

	_, err := repo.UpdateApprovalList(context.Background(),
		&models.User{LfUsername: "manager-lf"},
		&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project", Version: "v2"},
		"company-1",
		&models.ApprovalList{RemoveGitlabOrgApprovalList: []string{removedGroupURL}},
		&events.LogEventArgs{EventType: events.InvalidatedSignature})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to fetch GitLab group members")

	table.mu.Lock()
	defer table.mu.Unlock()
	assert.Empty(t, table.invalidated)
	assert.Equal(t, fakeStringList(removedGroupURL, keptGroupURL), table.find("ccla-sig")["gitlab_org_approval_list"], "the list is untouched")
}

// a group that stays on the list still covers its members; when it cannot be resolved nobody is
// re-checked and the list is still updated
func TestUpdateApprovalListGitLabGroupRemovalSkipsTheReCheckWhenARemainingGroupIsUnresolvable(t *testing.T) {
	logins := map[string]string{"user-001": "alice"}
	table := &fakeSignaturesTable{items: gitLabRemovalItems(t, logins), invalidated: map[string]int{}}
	awsSession := gitLabRemovalSession(t, table)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := gitLabRemovalRepo(t, ctrl, awsSession, logins)
	stubGitLabGroupMembers(t, func(_ context.Context, group string) ([]string, error) {
		if group == removedGroupURL {
			return []string{"alice"}, nil
		}
		return nil, errors.New("gitlab: 503")
	})

	updated, err := repo.UpdateApprovalList(context.Background(),
		&models.User{LfUsername: "manager-lf"},
		&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project", Version: "v2"},
		"company-1",
		&models.ApprovalList{RemoveGitlabOrgApprovalList: []string{removedGroupURL}},
		&events.LogEventArgs{EventType: events.InvalidatedSignature})
	require.NoError(t, err)
	require.NotNil(t, updated)

	table.mu.Lock()
	defer table.mu.Unlock()
	assert.Empty(t, table.invalidated, "alice may still be covered by the unresolvable group")
	assert.Equal(t, fakeStringList(keptGroupURL), table.find("ccla-sig")["gitlab_org_approval_list"])
}

// without a resolver (binaries other than the API server) the list changes and nobody is re-checked,
// which is the pre-existing behaviour
func TestUpdateApprovalListGitLabGroupRemovalWithoutResolverOnlyUpdatesTheList(t *testing.T) {
	logins := map[string]string{"user-001": "alice"}
	table := &fakeSignaturesTable{items: gitLabRemovalItems(t, logins), invalidated: map[string]int{}}
	awsSession := gitLabRemovalSession(t, table)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := gitLabRemovalRepo(t, ctrl, awsSession, logins)
	stubGitLabGroupMembers(t, nil)

	updated, err := repo.UpdateApprovalList(context.Background(),
		&models.User{LfUsername: "manager-lf"},
		&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project", Version: "v2"},
		"company-1",
		&models.ApprovalList{RemoveGitlabOrgApprovalList: []string{removedGroupURL}},
		&events.LogEventArgs{EventType: events.InvalidatedSignature})
	require.NoError(t, err)
	require.NotNil(t, updated)

	table.mu.Lock()
	defer table.mu.Unlock()
	assert.Empty(t, table.invalidated)
	assert.Equal(t, fakeStringList(keptGroupURL), table.find("ccla-sig")["gitlab_org_approval_list"])
}
