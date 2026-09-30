// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateApprovalListReAddCriteria restores through every approval criteria EvaluateUserApproval knows,
// exactly as it evaluates them: only a positive match against the entries in effect counts
func TestUpdateApprovalListReAddCriteria(t *testing.T) {
	cases := []struct {
		name     string
		ccla     readdCCLA
		user     func(*models.User)
		params   *models.ApprovalList
		orgs     map[string][]string
		orgErr   error
		restored bool
		// storeKey is the active PR lookup proving the restored user reached the GitHub status refresh
		storeKey    string
		wantLookups int
		wantErr     string
	}{
		{name: "email, case-insensitively", ccla: readdCCLA{emails: []string{"keep@acme.test"}},
			user:   func(u *models.User) { u.LfEmail = "Alice@Acme.Test" },
			params: &models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test"}}, restored: true, storeKey: "active_pr:e:Alice@Acme.Test"},
		{name: "secondary email", ccla: readdCCLA{emails: []string{"keep@acme.test"}},
			user:   func(u *models.User) { u.LfEmail = "primary@acme.test"; u.Emails = []string{"alice@acme.test"} },
			params: &models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test"}}, restored: true},
		{name: "wildcard domain", ccla: readdCCLA{domains: []string{"other.example"}},
			params: &models.ApprovalList{AddDomainApprovalList: []string{"*.acme.test"}}, restored: true, storeKey: "active_pr:e:alice@sub.acme.test",
			user: func(u *models.User) { u.LfEmail = "alice@sub.acme.test" }},
		{name: "plain domain", ccla: readdCCLA{domains: []string{"other.example"}},
			params: &models.ApprovalList{AddDomainApprovalList: []string{"acme.test"}}, restored: true, storeKey: "active_pr:e:alice@acme.test"},
		{name: "unrelated domain", ccla: readdCCLA{domains: []string{"other.example"}},
			params: &models.ApprovalList{AddDomainApprovalList: []string{"elsewhere.test"}}, restored: false},
		{name: "GitHub username, case-insensitively", ccla: readdCCLA{githubUsers: []string{"keeper"}},
			user:   func(u *models.User) { u.GithubUsername = "AliceGH" },
			params: &models.ApprovalList{AddGithubUsernameApprovalList: []string{"alicegh"}}, restored: true, storeKey: "active_pr:u:AliceGH"},
		{name: "GitLab username", ccla: readdCCLA{gitlabUsers: []string{"keeper"}},
			user:   func(u *models.User) { u.GitlabUsername = readdAliceGitLab },
			params: &models.ApprovalList{AddGitlabUsernameApprovalList: []string{"alice-gl"}}, restored: true},
		{name: "GitHub organization membership", ccla: readdCCLA{githubOrgs: []string{"keep-org"}},
			user:   func(u *models.User) { u.GithubUsername = readdAliceGitHub },
			params: &models.ApprovalList{AddGithubOrgApprovalList: []string{"acme-org"}}, orgs: map[string][]string{"alicegh": {"acme-org"}},
			restored: true, storeKey: "active_pr:u:alicegh", wantLookups: 1},
		{name: "GitHub organization lookup failure is reported, restores nothing", ccla: readdCCLA{githubOrgs: []string{"keep-org"}},
			user:   func(u *models.User) { u.GithubUsername = readdAliceGitHub },
			params: &models.ApprovalList{AddGithubOrgApprovalList: []string{"acme-org"}}, orgErr: errors.New("github is down"), restored: false, wantLookups: 1,
			wantErr: "the GitHub organization membership lookup failed"},
		{name: "GitHub organization non-member", ccla: readdCCLA{githubOrgs: []string{"keep-org"}},
			user:   func(u *models.User) { u.GithubUsername = readdAliceGitHub },
			params: &models.ApprovalList{AddGithubOrgApprovalList: []string{"acme-org"}}, orgs: map[string][]string{"alicegh": {"another-org"}}, restored: false, wantLookups: 1},
		{name: "GitHub organization without a GitHub user never looks up", ccla: readdCCLA{githubOrgs: []string{"keep-org"}},
			params: &models.ApprovalList{AddGithubOrgApprovalList: []string{"acme-org"}}, orgs: map[string][]string{"alicegh": {"acme-org"}}, restored: false, wantLookups: 0},
		{name: "GitLab group is not evaluated", ccla: readdCCLA{gitlabOrgs: []string{"keep-group"}},
			user:   func(u *models.User) { u.GitlabUsername = readdAliceGitLab },
			params: &models.ApprovalList{AddGitlabOrgApprovalList: []string{"acme-group"}}, restored: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newReaddHarness(t, []map[string]interface{}{tc.ccla.item(), readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria),
				readdDeliberateRow(2, "carol@acme.test")})
			alice := readdUser("user-001", "alice@acme.test")
			if tc.user != nil {
				tc.user(alice)
			}
			// the entries kept on the CCLA belong to somebody, so the edit never removes a whole list
			keeper := readdUser("user-keep", "keep@acme.test")
			keeper.GithubUsername, keeper.GitlabUsername = readdKeeper, readdKeeper
			h.registry.add(alice, readdUser("user-002", "carol@acme.test"), keeper)
			h.orgs.orgs, h.orgs.err = tc.orgs, tc.orgErr

			before := h.rows()
			_, err := h.call(tc.params)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "re-add the entries to retry")
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			if tc.restored {
				h.assertRestored(t, before["sig-001"])
				h.assertRecoveryReads(t)
			} else {
				h.assertUntouched(t, before["sig-001"])
			}
			h.assertUntouched(t, before["sig-002"])
			if tc.storeKey != "" {
				assert.Contains(t, h.storeReads(), tc.storeKey, "the restored contributor reached the pull request status refresh")
			}
			assert.Equal(t, tc.wantLookups, h.orgs.lookups(), "GitHub organization lookups")
			h.table.mu.Lock()
			assert.Empty(t, h.table.puts)
			assert.Zero(t, h.table.conditionFailures)
			h.table.mu.Unlock()
		})
	}
}

// TestUpdateApprovalListReAddAtScale restores a whole department: every employee index page is walked
// without a Limit or filter, the rows are read consistently in batches with the unprocessed keys retried,
// and running the same edit again is a no-op
func TestUpdateApprovalListReAddAtScale(t *testing.T) {
	const employees = 250
	items := []map[string]interface{}{readdCCLA{domains: []string{"other.example"}}.item()}
	for position := 1; position <= employees; position++ {
		items = append(items, readdRemovedRow(position, fmt.Sprintf("dev%03d@acme.test", position), utils.EmailDomainCriteria))
	}
	table := &fakeSignaturesTable{items: items, invalidated: map[string]int{}, maxRawPage: 40, unprocessedOnce: 7}
	h := newReaddHarnessWithTable(t, table)
	for position := 1; position <= employees; position++ {
		h.registry.add(readdUser(fmt.Sprintf("user-%03d", position), fmt.Sprintf("dev%03d@acme.test", position)))
	}

	before := h.rows()
	_, err := h.call(&models.ApprovalList{AddDomainApprovalList: []string{"acme.test"}})
	require.NoError(t, err)
	for id, item := range before {
		if id != readdCCLAID {
			h.assertRestored(t, item)
		}
	}
	h.assertRecoveryReads(t)

	h.table.mu.Lock()
	pages := 0
	for _, query := range h.table.queries {
		if query.indexName == fakeEmployeeIndex && query.filter == "" {
			pages++
			assert.Zero(t, query.limit)
		}
	}
	batches, retried := 0, 0
	for _, read := range h.table.reads {
		if read.tableName == readdSignatures && len(read.signatureIDs) > 1 {
			batches++
			if len(read.signatureIDs) == 7 {
				retried++
			}
		}
	}
	assert.Equal(t, len(h.table.invalidated), employees)
	assert.Zero(t, h.table.conditionFailures)
	assert.Empty(t, h.table.puts)
	h.table.mu.Unlock()
	assert.GreaterOrEqual(t, pages, 7, "every 40-row page of the employee index was walked")
	assert.Equal(t, 4, batches, "3 batches of at most 100 keys plus the retry of the 7 unprocessed keys")
	assert.Equal(t, 1, retried)

	// the same edit again finds nothing to restore
	after := h.rows()
	_, err = h.call(&models.ApprovalList{AddDomainApprovalList: []string{"acme.test"}})
	require.NoError(t, err)
	for id, item := range after {
		if id != readdCCLAID {
			h.assertUntouched(t, item)
		}
	}
}

// TestUpdateApprovalListReAddLosesToConcurrentWriters: whatever happens to the acknowledgment between the
// decision and the write - a deliberate invalidation, a deletion, a replacement, a signed or approved flip -
// the pinned condition fails, nothing is upserted and the edit still succeeds
func TestUpdateApprovalListReAddLosesToConcurrentWriters(t *testing.T) {
	cases := []struct {
		name           string
		hook           func(item map[string]interface{}, table *fakeSignaturesTable)
		approvedByHook bool
	}{
		{"deliberate invalidation lands first", func(item map[string]interface{}, _ *fakeSignaturesTable) {
			item["invalidation_reason"] = fakeS("left the company")
			item["invalidated_by"] = fakeS("pcc-admin")
		}, false},
		{"acknowledgment deleted", func(_ map[string]interface{}, table *fakeSignaturesTable) { table.removeLocked("sig-001") }, false},
		{"acknowledgment replaced by another user's", func(item map[string]interface{}, _ *fakeSignaturesTable) {
			item["signature_reference_id"] = fakeS("user-999")
		}, false},
		{"acknowledgment unsigned meanwhile", func(item map[string]interface{}, _ *fakeSignaturesTable) { item["signature_signed"] = fakeFalse() }, false},
		{"acknowledgment approved meanwhile", func(item map[string]interface{}, _ *fakeSignaturesTable) { item["signature_approved"] = fakeTrue() }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table := &fakeSignaturesTable{items: []map[string]interface{}{readdCCLA{emails: []string{"keep@acme.test"}}.item(),
				readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria)}, invalidated: map[string]int{}}
			fired := false
			table.beforeUpdate = func(item map[string]interface{}) {
				if fired || fakeItemString(item, "signature_id") != readdAliceID {
					return
				}
				fired = true
				tc.hook(item, table)
			}
			h := newReaddHarnessWithTable(t, table)
			h.registry.add(readdUser("user-001", "alice@acme.test"), readdUser("user-keep", "keep@acme.test"))

			_, err := h.call(&models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test"}})
			require.NoError(t, err)
			h.table.mu.Lock()
			defer h.table.mu.Unlock()
			assert.True(t, fired, "the restore write was attempted")
			assert.Equal(t, 1, h.table.conditionFailures, "the pinned condition rejected the write")
			assert.Empty(t, h.table.upserts)
			assert.Empty(t, h.table.invalidated)
			if item := h.table.find("sig-001"); item != nil {
				assert.Equal(t, readdRemovalNote(utils.EmailCriteria), fakeItemString(item, "note"), "note not rewritten")
				assert.Equal(t, tc.approvedByHook, readdBool(item, "signature_approved"))
				assert.False(t, readdHas(item, "date_modified") && fakeItemString(item, "date_modified") != "2023-01-01T00:00:01Z", "date_modified not rewritten")
			}
		})
	}
}

// TestUpdateApprovalListReAddObservesListChangedDuringLookup: entries removed by somebody else while the GitHub
// organization membership of a candidate is being looked up are observed by the consistent re-read of the corporate
// signature - nothing is restored on the strength of a list no longer in effect, and a user is evaluated again
// against what remains of the added entries
func TestUpdateApprovalListReAddObservesListChangedDuringLookup(t *testing.T) {
	cases := []struct {
		name string
		// remaining is the organization list another manager commits during the first lookup
		remaining []string
		restored  bool
		lookups   int
	}{
		{"the added organizations are removed", []string{"keep-org"}, false, 1},
		{"the whole list is removed", nil, false, 1},
		{"one added organization is removed, the user is evaluated again against the other", []string{"keep-org", "acme-org"}, true, 2},
		{"the organization the user belongs to is removed, the other added one stays", []string{"keep-org", "extra-org"}, false, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newReaddHarness(t, []map[string]interface{}{readdCCLA{githubOrgs: []string{"keep-org"}}.item(),
				readdRemovedRow(1, "alice@acme.test", utils.GitHubOrgCriteria)})
			alice := readdUser("user-001", "alice@acme.test")
			alice.GithubUsername = readdAliceGitHub
			keeper := readdUser("user-keep", "keep@acme.test")
			keeper.GithubUsername = readdKeeper
			h.registry.add(alice, keeper)
			lookups := 0
			listUserPublicOrgs = func(context.Context, string) ([]string, error) {
				lookups++
				if lookups == 1 {
					h.table.mu.Lock()
					ccla := h.table.find("ccla-sig")
					if len(tc.remaining) == 0 {
						delete(ccla, "github_org_whitelist")
					} else {
						ccla["github_org_whitelist"] = fakeStringList(tc.remaining...)
					}
					h.table.mu.Unlock()
				}
				return []string{"acme-org"}, nil
			}

			before := h.rows()
			_, err := h.call(&models.ApprovalList{AddGithubOrgApprovalList: []string{"acme-org", "extra-org"}})
			require.NoError(t, err)
			if tc.restored {
				h.assertRestored(t, before["sig-001"])
			} else {
				h.assertUntouched(t, before["sig-001"])
			}
			assert.Equal(t, tc.lookups, lookups, "GitHub organization lookups")
			h.table.mu.Lock()
			defer h.table.mu.Unlock()
			assert.Zero(t, h.table.conditionFailures)
			assert.Empty(t, h.table.upserts)
		})
	}
}
