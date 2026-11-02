// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/go-openapi/strfmt"
	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	eventsMock "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shared fixtures and harness for the approval list re-add tests (#2980): re-adding approval list
// entries must re-approve the signed employee acknowledgments that only an earlier approval list
// removal had invalidated, and never the deliberately invalidated ones.

const (
	readdManager       = "manager-lf"
	readdSignatures    = "cla-test-signatures"
	readdCCLAID        = "ccla-sig"
	readdAliceID       = "sig-001"
	readdAliceGitHub   = "alicegh"
	readdAliceGitLab   = "alice-gl"
	readdKeeper        = "keeper"
	readdRestorePrefix = "Re-enabled employee acknowledgment previously disabled by approval list removal via CLA Manager " + readdManager + " approval list edit on "
	// the note the removal path writes (verifyUserApprovals keeps two spaces before "removal")
	readdRemovalNoteFormat = "Signature invalidated (approved set to false) by " + readdManager + " due to %s  removal"
	readdDeliberateNote    = "Signature invalidated (approved set to false) by pcc-admin for legacy-dev"
)

func readdRemovalNote(criteria string) string { return fmt.Sprintf(readdRemovalNoteFormat, criteria) }

// readdRemovedRow is a signed acknowledgment an approval list removal invalidated, with the M2 attribution
func readdRemovedRow(position int, email, criteria string) map[string]interface{} {
	item := readdLegacyRemovedRow(position, email, criteria)
	item["date_invalidated"] = fakeS("2026-09-01T10:11:12.123456+0000")
	item["invalidated_by"] = fakeS(readdManager)
	item["invalidation_reason"] = fakeS(ApprovalListRemovalReasonPrefix + criteria + ")")
	return item
}

// readdLegacyRemovedRow is a removal-invalidated acknowledgment carrying only the pre-M2 note
func readdLegacyRemovedRow(position int, email, criteria string) map[string]interface{} {
	item := fakeEclaItem(position, email)
	item["signature_approved"] = fakeFalse()
	item["note"] = fakeS(readdRemovalNote(criteria))
	return item
}

// readdDeliberateRow is an acknowledgment somebody invalidated on purpose
func readdDeliberateRow(position int, email string) map[string]interface{} {
	item := fakeEclaItem(position, email)
	item["signature_approved"] = fakeFalse()
	item["note"] = fakeS(readdDeliberateNote)
	item["date_invalidated"] = fakeS("2026-09-02T10:11:12.123456+0000")
	item["invalidated_by"] = fakeS("pcc-admin")
	item["invalidation_reason"] = fakeS("left the company")
	item["invalidation_note"] = fakeS("manual")
	return item
}

// readdCCLA builds the company's signed and approved corporate signature with the given approval lists
type readdCCLA struct {
	emails, domains, githubUsers, githubOrgs, gitlabUsers, gitlabOrgs []string
	autoCreate                                                        bool
}

func (c readdCCLA) item() map[string]interface{} {
	item := map[string]interface{}{
		"signature_id":             fakeS("ccla-sig"),
		"signature_project_id":     fakeS("cla-group-1"),
		"signature_reference_id":   fakeS("company-1"),
		"signature_reference_type": fakeS("company"),
		"signature_reference_name": fakeS("Acme"),
		"signature_type":           fakeS("ccla"),
		"signature_approved":       fakeTrue(),
		"signature_signed":         fakeTrue(),
		"signature_acl":            fakeStringList(readdManager),
		"date_created":             fakeS("2022-01-01T00:00:00Z"),
		"date_modified":            fakeS("2022-01-01T00:00:00Z"),
	}
	if c.autoCreate {
		item["auto_create_ecla"] = fakeTrue()
	}
	for name, values := range map[string][]string{
		"email_whitelist": c.emails, "domain_whitelist": c.domains, "github_whitelist": c.githubUsers,
		"github_org_whitelist": c.githubOrgs, "gitlab_username_approval_list": c.gitlabUsers, "gitlab_org_approval_list": c.gitlabOrgs,
	} {
		if len(values) > 0 {
			item[name] = fakeStringList(values...)
		}
	}
	return item
}

// readdUser is an employee of company-1 known to the users table
func readdUser(id, email string) *models.User {
	return &models.User{UserID: id, LfEmail: strfmt.Email(email), Username: id, CompanyID: "company-1"}
}

// readdRegistry is the users table: lookups hand out copies, so the service may mutate them freely
type readdRegistry struct {
	mu           sync.Mutex
	users        map[string]*models.User
	created      int
	getUserCalls []string
}

func newReaddRegistry() *readdRegistry {
	registry := &readdRegistry{users: map[string]*models.User{}}
	registry.add(&models.User{UserID: "manager-user", LfUsername: readdManager, Username: readdManager, LfEmail: "manager@example.com", CompanyID: "company-1"})
	return registry
}

func (r *readdRegistry) add(list ...*models.User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, user := range list {
		r.users[user.UserID] = user
	}
}

func (r *readdRegistry) find(match func(*models.User) bool) *models.User {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, user := range r.users {
		if match(user) {
			clone := *user
			return &clone
		}
	}
	return nil
}

func (r *readdRegistry) byID(id string) (*models.User, error) {
	r.mu.Lock()
	r.getUserCalls = append(r.getUserCalls, id)
	r.mu.Unlock()
	return r.find(func(u *models.User) bool { return u.UserID == id }), nil
}

func (r *readdRegistry) byUserName(name string, _ bool) (*models.User, error) {
	return r.find(func(u *models.User) bool { return u.LfUsername == name }), nil
}

func readdUserHasEmail(u *models.User, email string) bool {
	if strings.EqualFold(string(u.LfEmail), email) {
		return true
	}
	for _, candidate := range u.Emails {
		if strings.EqualFold(candidate, email) {
			return true
		}
	}
	return false
}

func (r *readdRegistry) byEmail(email string) (*models.User, error) {
	if user := r.find(func(u *models.User) bool { return readdUserHasEmail(u, email) }); user != nil {
		return user, nil
	}
	return nil, &utils.UserNotFound{Message: "user not found", UserEmail: email}
}

func (r *readdRegistry) byGitHub(login string) (*models.User, error) {
	if user := r.find(func(u *models.User) bool { return u.GithubUsername != "" && strings.EqualFold(u.GithubUsername, login) }); user != nil {
		return user, nil
	}
	return nil, errors.New("github user not found: " + login)
}

func (r *readdRegistry) byGitLab(login string) (*models.User, error) {
	if user := r.find(func(u *models.User) bool { return u.GitlabUsername != "" && strings.EqualFold(u.GitlabUsername, login) }); user != nil {
		return user, nil
	}
	return nil, errors.New("gitlab user not found: " + login)
}

func (r *readdRegistry) search(_ string, term string, _ bool) (*models.Users, error) {
	result := &models.Users{Users: []models.User{}}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, user := range r.users {
		if readdUserHasEmail(user, term) {
			result.Users = append(result.Users, *user)
		}
	}
	return result, nil
}

func (r *readdRegistry) updateCompany(userID, companyID, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if user, ok := r.users[userID]; ok {
		user.CompanyID = companyID
	}
	return nil
}

func (r *readdRegistry) create(user *models.User) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.created++
	clone := *user
	clone.UserID = fmt.Sprintf("created-%d", r.created)
	r.users[clone.UserID] = &clone
	result := clone
	return &result, nil
}

func (r *readdRegistry) lookups(id string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, call := range r.getUserCalls {
		if call == id {
			count++
		}
	}
	return count
}

// readdOrgStub stands in for the GitHub public organization lookup
type readdOrgStub struct {
	mu    sync.Mutex
	orgs  map[string][]string
	err   error
	calls []string
}

func (o *readdOrgStub) list(_ context.Context, login string) ([]string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls = append(o.calls, login)
	if o.err != nil {
		return nil, o.err
	}
	return o.orgs[login], nil
}

func (o *readdOrgStub) lookups() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.calls)
}

type readdHarness struct {
	table     *fakeSignaturesTable
	repo      repository
	svc       service
	registry  *readdRegistry
	approvals *fakeApprovalRepo
	emails    *recordingEmailSender
	events    *int64
	orgs      *readdOrgStub
}

func newReaddHarness(t *testing.T, items []map[string]interface{}) *readdHarness {
	t.Helper()
	return newReaddHarnessWithTable(t, &fakeSignaturesTable{items: items, invalidated: map[string]int{}})
}

// newReaddHarnessWithTable wires the real signature repository and service to the fake table, a users
// table registry, stubbed company/events dependencies, a recording email sender and an org stub
func newReaddHarnessWithTable(t *testing.T, table *fakeSignaturesTable) *readdHarness {
	t.Helper()
	sess, closeServer := newApprovalRemovalSession(t, table)
	t.Cleanup(closeServer)
	ctrl := gomock.NewController(t)

	registry := newReaddRegistry()
	mockUsers := mock_users.NewMockUserRepository(ctrl)
	mockUsers.EXPECT().GetUser(gomock.Any()).DoAndReturn(registry.byID).AnyTimes()
	mockUsers.EXPECT().GetUserByUserName(gomock.Any(), gomock.Any()).DoAndReturn(registry.byUserName).AnyTimes()
	mockUsers.EXPECT().GetUserByEmail(gomock.Any()).DoAndReturn(registry.byEmail).AnyTimes()
	mockUsers.EXPECT().GetUserByGitHubUsername(gomock.Any()).DoAndReturn(registry.byGitHub).AnyTimes()
	mockUsers.EXPECT().GetUserByGitLabUsername(gomock.Any()).DoAndReturn(registry.byGitLab).AnyTimes()
	mockUsers.EXPECT().SearchUsers(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(registry.search).AnyTimes()
	mockUsers.EXPECT().UpdateUserCompanyID(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(registry.updateCompany).AnyTimes()
	mockUsers.EXPECT().CreateUser(gomock.Any()).DoAndReturn(registry.create).AnyTimes()

	mockCompany := mock_company.NewMockIRepository(ctrl)
	mockCompany.EXPECT().GetCompany(gomock.Any(), gomock.Any()).Return(fakeCompany(), nil).AnyTimes()

	logged := new(int64)
	mockEvents := eventsMock.NewMockService(ctrl)
	mockEvents.EXPECT().LogEvent(gomock.Any()).Do(func(*events.LogEventArgs) { atomic.AddInt64(logged, 1) }).AnyTimes()
	mockEvents.EXPECT().LogEventWithContext(gomock.Any(), gomock.Any()).Do(func(context.Context, *events.LogEventArgs) { atomic.AddInt64(logged, 1) }).AnyTimes()

	emails := &recordingEmailSender{}
	previousSender := utils.GetEmailSender()
	utils.SetEmailSender(emails)
	t.Cleanup(func() { utils.SetEmailSender(previousSender) })

	orgs := &readdOrgStub{orgs: map[string][]string{}}
	previousOrgs := listUserPublicOrgs
	listUserPublicOrgs = orgs.list
	t.Cleanup(func() { listUserPublicOrgs = previousOrgs })

	approvalsRepo := &fakeApprovalRepo{}
	repo := repository{stage: "test", dynamoDBClient: dynamodb.New(sess), companyRepo: mockCompany, usersRepo: mockUsers,
		eventsService: mockEvents, signatureTableName: readdSignatures, approvalRepo: approvalsRepo}
	svc := service{repo: repo, usersService: users.NewService(mockUsers, mockEvents), eventsService: mockEvents}
	return &readdHarness{table: table, repo: repo, svc: svc, registry: registry, approvals: approvalsRepo, emails: emails, events: logged, orgs: orgs}
}

// call edits the approval list as the CLA manager through the service, the way the v4 handler does
func (h *readdHarness) call(params *models.ApprovalList) (*models.Signature, error) {
	return h.svc.UpdateApprovalList(context.Background(), &auth.User{UserName: readdManager, Email: "manager@example.com"},
		&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project", Version: "v2"}, fakeCompany(), "cla-group-1", params, "project-sfid")
}

// row returns a shallow copy of the stored item, read directly so the table's read log stays untouched
func (h *readdHarness) row(t *testing.T, signatureID string) map[string]interface{} {
	t.Helper()
	h.table.mu.Lock()
	defer h.table.mu.Unlock()
	item := h.table.find(signatureID)
	require.NotNil(t, item, "signature %s exists", signatureID)
	return fakeCopyItem(item)
}

// rows snapshots every stored item by signature ID
func (h *readdHarness) rows() map[string]map[string]interface{} {
	h.table.mu.Lock()
	defer h.table.mu.Unlock()
	out := map[string]map[string]interface{}{}
	for _, item := range h.table.items {
		out[fakeItemString(item, "signature_id")] = fakeCopyItem(item)
	}
	return out
}

func readdBool(item map[string]interface{}, name string) bool {
	if attr, ok := item[name].(map[string]interface{}); ok {
		if value, ok := attr["BOOL"].(bool); ok {
			return value
		}
	}
	return false
}

func readdHas(item map[string]interface{}, name string) bool {
	_, ok := item[name]
	return ok
}

// signatureReads returns the GetItem/BatchGetItem calls that hit the signatures table
func (h *readdHarness) signatureReads() []fakeCapturedRead {
	h.table.mu.Lock()
	defer h.table.mu.Unlock()
	var reads []fakeCapturedRead
	for _, read := range h.table.reads {
		if read.tableName == readdSignatures {
			reads = append(reads, read)
		}
	}
	return reads
}

// storeReads returns the keys looked up in the store table (the active PR metadata of a user)
func (h *readdHarness) storeReads() []string {
	h.table.mu.Lock()
	defer h.table.mu.Unlock()
	var keys []string
	for _, read := range h.table.reads {
		if read.tableName != readdSignatures && len(read.signatureIDs) == 1 {
			keys = append(keys, read.signatureIDs[0])
		}
	}
	return keys
}

// assertRestored checks that the acknowledgment is approved again with the attribution cleared and the
// restore note appended to what was there, and that nothing else about it changed
func (h *readdHarness) assertRestored(t *testing.T, before map[string]interface{}) {
	t.Helper()
	signatureID := fakeItemString(before, "signature_id")
	after := h.row(t, signatureID)
	assert.True(t, readdBool(after, "signature_approved"), "%s approved again", signatureID)
	assert.True(t, readdBool(after, "signature_signed"), "%s still signed", signatureID)
	for _, attr := range []string{"date_invalidated", "invalidated_by", "invalidation_reason", "invalidation_note"} {
		assert.False(t, readdHas(after, attr), "%s %s cleared", signatureID, attr)
	}
	note := fakeItemString(after, "note")
	wantPrefix := strings.TrimSpace(fakeItemString(before, "note") + " " + readdRestorePrefix)
	assert.True(t, strings.HasPrefix(note, wantPrefix), "%s note %q keeps the history and gets the restore note", signatureID, note)
	assert.True(t, strings.HasSuffix(note, "Z."), "%s note ends with the edit timestamp: %q", signatureID, note)
	modified, parseErr := time.Parse(time.RFC3339, fakeItemString(after, "date_modified"))
	require.NoError(t, parseErr, "%s date_modified is a timestamp", signatureID)
	assert.Less(t, time.Since(modified), time.Minute, "%s date_modified refreshed", signatureID)
	for _, attr := range []string{"signature_id", "signature_project_id", "signature_reference_id", "signature_reference_type", "signature_type",
		"signature_user_ccla_company_id", "user_email", "date_created"} {
		assert.Equal(t, before[attr], after[attr], "%s %s unchanged", signatureID, attr)
	}
}

// assertUntouched checks that the stored item is exactly what it was
func (h *readdHarness) assertUntouched(t *testing.T, before map[string]interface{}) {
	t.Helper()
	signatureID := fakeItemString(before, "signature_id")
	assert.Equal(t, before, h.row(t, signatureID), "%s untouched", signatureID)
}

// assertRecoveryReads checks the candidate query shape and that every signatures-table read was strongly
// consistent, including the corporate signature GetItem and at least one BatchGetItem
func (h *readdHarness) assertRecoveryReads(t *testing.T) {
	t.Helper()
	reads := h.signatureReads()
	require.NotEmpty(t, reads)
	sawCCLA, sawBatch := false, false
	for _, read := range reads {
		assert.True(t, read.consistentRead, "read of %v is strongly consistent", read.signatureIDs)
		if len(read.signatureIDs) == 1 && read.signatureIDs[0] == readdCCLAID {
			sawCCLA = true
		}
		if len(read.signatureIDs) > 1 {
			sawBatch = true
		}
	}
	assert.True(t, sawCCLA, "the corporate signature was re-read consistently")
	assert.True(t, sawBatch, "the candidates were read consistently in batch")
	h.table.mu.Lock()
	defer h.table.mu.Unlock()
	found := false
	for _, query := range h.table.queries {
		if query.indexName != fakeEmployeeIndex || query.filter != "" {
			continue
		}
		found = true
		assert.Zero(t, query.limit, "candidate query has no Limit")
		assert.ElementsMatch(t, []string{"signature_user_ccla_company_id", "signature_project_id", "signature_id"}, query.attributeNames)
	}
	assert.True(t, found, "the candidates were listed through the employee index without a filter")
}
