// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateApprovalListReAddRestoresRemovalInvalidatedAcknowledgments is the #2980 scenario end to end
// through the service: an email removal invalidates the contributor's acknowledgment, re-adding the email
// re-approves that very record (no new one), while deliberate and unexplained invalidations stay as they are.
func TestUpdateApprovalListReAddRestoresRemovalInvalidatedAcknowledgments(t *testing.T) {
	bob := readdRemovedRow(2, "bob@acme.test", utils.EmailCriteria)
	bob["signature_type"] = fakeS("ecla")
	bob["sig_type_signed_approved_id"] = fakeS("ecla#true#true#company-1")
	items := []map[string]interface{}{
		readdCCLA{emails: []string{"alice@acme.test", "keep@acme.test"}, domains: []string{"other.example"}}.item(),
		fakeEclaItem(1, "alice@acme.test"),
		bob,
		readdDeliberateRow(3, "carol@acme.test"),
		fakeEclaItem(4, "dave@acme.test"),
	}
	items[4]["signature_approved"] = fakeFalse()
	h := newReaddHarness(t, items)
	h.registry.add(readdUser("user-001", "alice@acme.test"), readdUser("user-002", "bob@acme.test"),
		readdUser("user-003", "carol@acme.test"), readdUser("user-004", "dave@acme.test"), readdUser("user-keep", "keep@acme.test"))

	// the removal
	before := h.rows()
	_, err := h.call(&models.ApprovalList{RemoveEmailApprovalList: []string{"alice@acme.test"}})
	require.NoError(t, err)
	alice := h.row(t, "sig-001")
	assert.False(t, readdBool(alice, "signature_approved"), "alice's acknowledgment was invalidated by the removal")
	assert.Equal(t, readdRemovalNote(utils.EmailCriteria), fakeItemString(alice, "note"))
	assert.Equal(t, ApprovalListRemovalReasonPrefix+utils.EmailCriteria+")", fakeItemString(alice, "invalidation_reason"))
	assert.Equal(t, readdManager, fakeItemString(alice, "invalidated_by"))
	assert.NotEmpty(t, fakeItemString(alice, "date_invalidated"))
	assert.Empty(t, h.signatureReads(), "a remove-only edit reads no signature by key")
	h.assertUntouched(t, before["sig-002"])
	h.assertUntouched(t, before["sig-003"])
	h.assertUntouched(t, before["sig-004"])

	// the re-add (Bob's entry with a different case than his stored email)
	before = h.rows()
	logged := atomic.LoadInt64(h.events)
	updated, err := h.call(&models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test", "Bob@acme.test"}})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.ElementsMatch(t, []string{"keep@acme.test", "alice@acme.test", "Bob@acme.test"}, updated.EmailApprovalList)

	h.assertRestored(t, before["sig-001"])
	h.assertRestored(t, before["sig-002"])
	h.assertUntouched(t, before["sig-003"])
	h.assertUntouched(t, before["sig-004"])
	h.assertRecoveryReads(t)

	h.table.mu.Lock()
	assert.Empty(t, h.table.puts, "no acknowledgment was created")
	assert.Empty(t, h.table.upserts, "no acknowledgment was upserted")
	assert.Zero(t, h.table.conditionFailures)
	assert.Equal(t, map[string]int{"sig-001": 2, "sig-002": 1}, h.table.invalidated, "approval flips: alice's removal + both restores")
	h.table.mu.Unlock()
	assert.Greater(t, atomic.LoadInt64(h.events), logged, "the approval list update was logged")
	h.emails.mu.Lock()
	assert.Contains(t, h.emails.recipients, "manager@example.com")
	h.emails.mu.Unlock()
	assert.Subset(t, h.storeReads(), []string{"active_pr:e:alice@acme.test", "active_pr:e:bob@acme.test"},
		"the restored contributors had their pull request status refreshed")
}

// TestUpdateApprovalListReAddWithAutoCreateRestoresBeforeCreating: with auto-create enabled the restore runs
// first, so the restored contributor keeps the original acknowledgment, a brand-new contributor gets a new
// one, and a deliberately invalidated contributor gets neither
func TestUpdateApprovalListReAddWithAutoCreateRestoresBeforeCreating(t *testing.T) {
	keep := fakeEclaItem(9, "keep@acme.test")
	items := []map[string]interface{}{
		readdCCLA{emails: []string{"keep@acme.test"}, domains: []string{"other.example"}, autoCreate: true}.item(),
		readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria),
		readdDeliberateRow(3, "carol@acme.test"),
		keep,
	}
	h := newReaddHarness(t, items)
	h.registry.add(readdUser("user-001", "alice@acme.test"), readdUser("user-003", "carol@acme.test"), readdUser("user-009", "keep@acme.test"))

	before := h.rows()
	_, err := h.call(&models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test", "new@acme.test", "carol@acme.test"}})
	require.NoError(t, err)

	h.assertRestored(t, before["sig-001"])
	h.assertUntouched(t, before["sig-003"])
	h.assertUntouched(t, before["sig-009"])
	h.table.mu.Lock()
	defer h.table.mu.Unlock()
	require.Len(t, h.table.puts, 1, "only the new contributor gets a new acknowledgment")
	assert.Equal(t, "created-1", fakeItemString(h.table.puts[0], "signature_reference_id"))
	assert.Equal(t, "cla-group-1", fakeItemString(h.table.puts[0], "signature_project_id"))
	assert.Empty(t, h.table.upserts)
	assert.Zero(t, h.table.conditionFailures)
	assert.Equal(t, map[string]int{"sig-001": 1}, h.table.invalidated)
}

// readdMatrixItems seeds one acknowledgment per case the restore must decide on
func readdMatrixItems() []map[string]interface{} {
	otherCompany := readdRemovedRow(7, "grace@acme.test", utils.EmailCriteria)
	otherCompany["signature_user_ccla_company_id"] = fakeS("company-2")
	otherProject := readdRemovedRow(8, "heidi@acme.test", utils.EmailCriteria)
	otherProject["signature_project_id"] = fakeS("cla-group-2")
	eclaTyped := readdRemovedRow(9, "ivan@acme.test", utils.GitHubUsernameCriteria)
	eclaTyped["signature_type"] = fakeS("ecla")
	unsigned := readdRemovedRow(6, "frank@acme.test", utils.EmailCriteria)
	unsigned["signature_signed"] = fakeFalse()
	ambiguous := fakeEclaItem(5, "erin@acme.test")
	ambiguous["signature_approved"] = fakeFalse()
	legacyDeliberate := fakeEclaItem(4, "dave@acme.test")
	legacyDeliberate["signature_approved"] = fakeFalse()
	legacyDeliberate["note"] = fakeS(readdDeliberateNote)
	return []map[string]interface{}{
		readdCCLA{emails: []string{"keep@acme.test", "nobody@acme.test"}, domains: []string{"other.example"}}.item(),
		readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria),
		readdLegacyRemovedRow(2, "bob@acme.test", utils.EmailDomainCriteria),
		readdDeliberateRow(3, "carol@acme.test"),
		legacyDeliberate,
		ambiguous,
		unsigned,
		otherCompany,
		otherProject,
		eclaTyped,
		readdRemovedRow(10, "judy@acme.test", utils.EmailCriteria),
		fakeEclaItem(11, "kate@acme.test"),
		readdRemovedRow(12, "leo@acme.test", utils.EmailCriteria),
		readdRemovedRow(13, "mia@acme.test", utils.GitHubOrgCriteria),
	}
}

func readdMatrixHarness(t *testing.T) *readdHarness {
	t.Helper()
	h := newReaddHarness(t, readdMatrixItems())
	for position, name := range []string{"alice", "bob", "carol", "dave", "erin", "frank", "grace", "heidi", "ivan", "judy", "kate"} {
		h.registry.add(readdUser(fmt.Sprintf("user-%03d", position+1), name+"@acme.test"))
	}
	// leo (user-012) is unknown to the users table; mia is known
	h.registry.add(readdUser("user-013", "mia@acme.test"), readdUser("user-keep", "keep@acme.test"))
	return h
}

// TestUpdateApprovalListReAddMatrix decides every seeded case in one edit: only signed, unapproved
// acknowledgments whose only invalidation evidence is an approval list removal, under this company and
// CLA group, whose user the re-added entries cover, are restored - whatever the criteria of the removal was
func TestUpdateApprovalListReAddMatrix(t *testing.T) {
	h := readdMatrixHarness(t)
	before := h.rows()
	added := []string{"alice@acme.test", "bob@acme.test", "carol@acme.test", "dave@acme.test", "erin@acme.test", "frank@acme.test",
		"grace@acme.test", "heidi@acme.test", "ivan@acme.test", "kate@acme.test", "leo@acme.test", "mia@acme.test"}
	_, err := h.call(&models.ApprovalList{AddEmailApprovalList: added})
	require.NoError(t, err)

	for _, restored := range []string{"sig-001", "sig-002", "sig-009", "sig-013"} {
		h.assertRestored(t, before[restored])
	}
	for _, untouched := range []string{"sig-003", "sig-004", "sig-005", "sig-006", "sig-007", "sig-008", "sig-010", "sig-011", "sig-012"} {
		h.assertUntouched(t, before[untouched])
	}
	h.assertRecoveryReads(t)
	h.table.mu.Lock()
	assert.Empty(t, h.table.puts)
	assert.Empty(t, h.table.upserts)
	assert.Zero(t, h.table.conditionFailures)
	assert.Equal(t, map[string]int{"sig-001": 1, "sig-002": 1, "sig-009": 1, "sig-013": 1}, h.table.invalidated)
	h.table.mu.Unlock()
	assert.Equal(t, 1, h.registry.lookups("user-012"), "the unknown user was looked up once and skipped")
}

func TestUpdateApprovalListReAddNothingToRestore(t *testing.T) {
	t.Run("remove-only edit restores nothing and reads no signature by key", func(t *testing.T) {
		h := readdMatrixHarness(t)
		before := h.rows()
		_, err := h.call(&models.ApprovalList{RemoveEmailApprovalList: []string{"nobody@acme.test"}})
		require.NoError(t, err)
		for id, item := range before {
			if id != readdCCLAID {
				h.assertUntouched(t, item)
			}
		}
		assert.Empty(t, h.signatureReads())
		h.table.mu.Lock()
		assert.Empty(t, h.table.invalidated)
		h.table.mu.Unlock()
	})
	t.Run("unrelated add evaluates the candidates and restores none", func(t *testing.T) {
		h := readdMatrixHarness(t)
		before := h.rows()
		_, err := h.call(&models.ApprovalList{AddEmailApprovalList: []string{"zed@acme.test"}})
		require.NoError(t, err)
		for id, item := range before {
			if id != readdCCLAID {
				h.assertUntouched(t, item)
			}
		}
		h.assertRecoveryReads(t)
		h.table.mu.Lock()
		assert.Empty(t, h.table.invalidated)
		assert.Empty(t, h.table.puts)
		h.table.mu.Unlock()
	})
	t.Run("adding and removing the same entry in one edit restores nothing", func(t *testing.T) {
		h := readdMatrixHarness(t)
		before := h.rows()
		_, err := h.call(&models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test"}, RemoveEmailApprovalList: []string{"alice@acme.test"}})
		require.NoError(t, err)
		h.assertUntouched(t, before["sig-001"])
		for _, read := range h.signatureReads() {
			assert.Equal(t, []string{"ccla-sig"}, read.signatureIDs, "the entry is not in effect - no candidate is read")
		}
		h.table.mu.Lock()
		assert.Equal(t, 1, h.table.conditionFailures, "the removal pass re-tried alice's already invalidated acknowledgment")
		assert.Empty(t, h.table.invalidated)
		h.table.mu.Unlock()
	})
}
