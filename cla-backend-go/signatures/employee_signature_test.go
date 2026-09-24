// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignatureInvalidated(t *testing.T) {
	cases := []struct {
		name string
		sig  *ItemSignature
		want bool
	}{
		{"nil record", nil, false},
		{"approved record is never invalidated", &ItemSignature{SignatureApproved: true, DateInvalidated: "2026-09-01T00:00:00Z", Note: "Signature invalidated (approved set to false)"}, false},
		{"unapproved without any evidence", &ItemSignature{SignatureApproved: false, Note: "created via auto-create"}, false},
		{"M2 attribution date", &ItemSignature{DateInvalidated: "2026-09-01T00:00:00Z"}, true},
		{"M2 attribution actor", &ItemSignature{InvalidatedBy: "pcc-admin"}, true},
		{"M2 attribution reason", &ItemSignature{InvalidationReason: "approved list removal"}, true},
		{"M2 attribution note", &ItemSignature{InvalidationNote: "left the company"}, true},
		{"legacy note", &ItemSignature{Note: "Signature invalidated (approved set to false) due to approval list removal"}, true},
		{"legacy note is matched case-insensitively", &ItemSignature{Note: "SIGNATURE INVALIDATED by the project manager"}, true},
		{"unrelated note", &ItemSignature{Note: "signed and approved employee acknowledgment since auto_create_ecla feature flag set to true"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, signatureInvalidated(tc.sig))
		})
	}
}

func TestAppendNote(t *testing.T) {
	assert.Equal(t, "new", appendNote("", " new "))
	assert.Equal(t, "old", appendNote(" old", ""))
	assert.Equal(t, "", appendNote("  ", ""))
	assert.Equal(t, "old note. new note.", appendNote("old note. ", " new note."))
}

// expected condition fragments: a pinned attribute must equal what was read, a blank one may also be missing
func pinnedAs(name, placeholder string) string { return " AND " + name + " = " + placeholder }
func blankAs(name, placeholder string) string {
	return " AND (attribute_not_exists(" + name + ") OR " + name + " = " + placeholder + ")"
}

func TestValidationUpdateExpression(t *testing.T) {
	wantNames := map[string]*string{
		"#ID": aws.String("signature_id"),
		"#A":  aws.String("signature_approved"),
		"#S":  aws.String("note"),
		"#M":  aws.String("date_modified"),
		"#DI": aws.String("date_invalidated"),
		"#IB": aws.String("invalidated_by"),
		"#IR": aws.String("invalidation_reason"),
		"#IN": aws.String("invalidation_note"),
	}

	t.Run("deliberate", func(t *testing.T) {
		existing := &ItemSignature{SignatureApproved: false, Note: "old note", DateInvalidated: "2026-09-01T00:00:00.000000+0000", InvalidatedBy: "pcc-admin"}
		names, values, expr, condition := validationUpdateExpression(existing, "kept note", "2026-09-15T10:00:00.000000+0000", false)

		assert.Equal(t, "SET #A = :a, #S = :s, #M = :m REMOVE #DI, #IB, #IR, #IN", expr)
		assert.Equal(t, "attribute_exists(#ID)"+pinnedAs("#S", ":cs"), condition,
			"the deliberate write only requires the record to exist with the note it was read with - it re-approves invalidated records")
		assert.Equal(t, wantNames, names)
		require.Len(t, values, 4)
		assert.True(t, aws.BoolValue(values[":a"].BOOL))
		assert.Equal(t, "kept note", aws.StringValue(values[":s"].S))
		assert.Equal(t, "2026-09-15T10:00:00.000000+0000", aws.StringValue(values[":m"].S))
		assert.Equal(t, "old note", aws.StringValue(values[":cs"].S), "note snapshot")

		_, values, _, condition = validationUpdateExpression(&ItemSignature{SignatureApproved: true}, "n", "now", false)
		assert.Equal(t, "attribute_exists(#ID)"+blankAs("#S", ":cs"), condition, "a blank note may also be missing")
		assert.Equal(t, "", aws.StringValue(values[":cs"].S))
	})

	t.Run("automatic", func(t *testing.T) {
		names, values, expr, condition := validationUpdateExpression(&ItemSignature{SignatureApproved: false, Note: "old note"}, "kept note", "now", true)

		assert.Equal(t, "SET #A = :a, #S = :s, #M = :m REMOVE #DI, #IB, #IR, #IN", expr)
		assert.Equal(t, "attribute_exists(#ID)"+pinnedAs("#S", ":cs")+blankAs("#A", ":ca")+
			blankAs("#DI", ":cdi")+blankAs("#IB", ":cib")+blankAs("#IR", ":cir")+blankAs("#IN", ":cin"), condition)
		assert.Equal(t, wantNames, names)
		require.Len(t, values, 9)
		assert.False(t, aws.BoolValue(values[":ca"].BOOL), "approval snapshot")
		assert.Equal(t, "old note", aws.StringValue(values[":cs"].S))
		for _, placeholder := range []string{":cdi", ":cib", ":cir", ":cin"} {
			assert.Equal(t, "", aws.StringValue(values[placeholder].S), placeholder)
		}

		approvedWithLeftovers := &ItemSignature{SignatureApproved: true, DateInvalidated: "2026-08-01T00:00:00.000000+0000", InvalidatedBy: "cla-manager"}
		_, values, _, condition = validationUpdateExpression(approvedWithLeftovers, "n", "now", true)
		assert.Equal(t, "attribute_exists(#ID)"+blankAs("#S", ":cs")+pinnedAs("#A", ":ca")+
			pinnedAs("#DI", ":cdi")+pinnedAs("#IB", ":cib")+blankAs("#IR", ":cir")+blankAs("#IN", ":cin"), condition,
			"a true approval must still be present and true, retained attribution must still equal what was read")
		assert.True(t, aws.BoolValue(values[":ca"].BOOL))
		assert.Equal(t, "", aws.StringValue(values[":cs"].S))
		assert.Equal(t, "2026-08-01T00:00:00.000000+0000", aws.StringValue(values[":cdi"].S))
		assert.Equal(t, "cla-manager", aws.StringValue(values[":cib"].S))
	})
}

func employeeSignatureItems() []map[string]interface{} {
	items := corporateContributorItems()
	unapproved := fakeEclaItem(6, "unapproved.dev@example.com")
	unapproved["signature_approved"] = fakeFalse()
	unapproved["note"] = fakeS("created via auto-create, approval pending")
	return append(items, unapproved)
}

func lookupEmployeeSignature(t *testing.T, repo repository, userID string) *EmployeeModel {
	t.Helper()
	var wg sync.WaitGroup
	resultChan := make(chan *EmployeeModel, 1)
	errChan := make(chan error, 1)
	wg.Add(1)
	repo.GetProjectCompanyEmployeeSignature(context.Background(),
		&models.Company{CompanyID: "company-1", CompanyName: "Acme"},
		&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project"},
		&models.User{UserID: userID}, &wg, resultChan, errChan)
	wg.Wait()
	close(resultChan)
	close(errChan)
	for err := range errChan {
		require.NoError(t, err)
	}
	return <-resultChan
}

func TestGetProjectCompanyEmployeeSignatureFlagsInvalidatedRecords(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, employeeSignatureItems())

	cases := []struct {
		userID      string
		signatureID string
		invalidated bool
	}{
		{"user-001", "sig-001", false}, // approved
		{"user-002", "sig-002", true},  // M2 attribution attributes
		{"user-003", "sig-003", true},  // legacy note-only invalidation
		{"user-004", "sig-004", false}, // unsigned, never invalidated
		{"user-006", "sig-006", false}, // unapproved without invalidation evidence
	}
	for _, tc := range cases {
		t.Run(tc.userID, func(t *testing.T) {
			result := lookupEmployeeSignature(t, repo, tc.userID)
			require.NotNil(t, result)
			require.NotNil(t, result.Signature)
			assert.Equal(t, tc.signatureID, result.Signature.SignatureID)
			assert.Equal(t, tc.invalidated, result.Invalidated)
			assert.Equal(t, tc.userID, result.User.UserID)
		})
	}

	t.Run("other company's record is not the user's acknowledgment", func(t *testing.T) {
		result := lookupEmployeeSignature(t, repo, "user-005")
		require.NotNil(t, result)
		assert.Nil(t, result.Signature)
		assert.False(t, result.Invalidated)
	})

	require.NotEmpty(t, table.queries)
	assert.Equal(t, SignatureProjectReferenceIndex, table.queries[0].indexName, "the key range is the user's records under the CLA group, not every signature the user ever made")
	assert.Equal(t, int64(100), table.queries[0].limit)
	assert.Subset(t, table.queries[0].attributeNames, []string{"note", "date_invalidated", "invalidated_by", "invalidation_reason", "invalidation_note"},
		"the lookup projects the invalidation attributes")
}

func busyUserItems(unrelated int) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, unrelated+6)
	for i := 0; i < unrelated; i++ {
		other := fakeEclaItem(100+i, "removed.dev@example.com")
		other["signature_reference_id"] = fakeS("user-002")
		other["signature_project_id"] = fakeS(fmt.Sprintf("cla-group-%d", 100+i))
		if i%2 == 0 {
			delete(other, "signature_user_ccla_company_id")
			other["signature_type"] = fakeS("cla")
		} else {
			other["signature_user_ccla_company_id"] = fakeS(fmt.Sprintf("company-%d", 100+i))
		}
		items = append(items, other)
	}
	return append(items, employeeSignatureItems()...)
}

func TestGetProjectCompanyEmployeeSignatureFindsTargetBehindUnrelatedSignatures(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, busyUserItems(12))

	result := lookupEmployeeSignature(t, repo, "user-002")
	require.NotNil(t, result)
	require.NotNil(t, result.Signature, "twelve unrelated signatures of the same user must not hide the acknowledgment")
	assert.Equal(t, "sig-002", result.Signature.SignatureID)
	assert.True(t, result.Invalidated)

	company := &models.Company{CompanyID: "company-1", CompanyName: "Acme"}
	claGroup := &models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project"}
	require.NoError(t, repo.CreateProjectCompanyEmployeeSignature(context.Background(), company, claGroup, &models.User{UserID: "user-002"}))
	assert.Empty(t, table.updates, "the invalidated acknowledgment is found, so nothing is re-approved and no fresh record is created")
}

func TestGetProjectCompanyEmployeeSignatureFollowsEveryPage(t *testing.T) {
	items := make([]map[string]interface{}, 0, 111)
	for i := 0; i < 105; i++ {
		other := fakeEclaItem(200+i, "removed.dev@example.com")
		other["signature_reference_id"] = fakeS("user-002")
		other["signature_user_ccla_company_id"] = fakeS(fmt.Sprintf("company-%d", 200+i))
		items = append(items, other)
	}
	items = append(items, employeeSignatureItems()...)
	repo, table := newCorporateContributorsRepo(t, items)

	result := lookupEmployeeSignature(t, repo, "user-002")
	require.NotNil(t, result)
	require.NotNil(t, result.Signature)
	assert.Equal(t, "sig-002", result.Signature.SignatureID)
	assert.True(t, result.Invalidated)
	assert.GreaterOrEqual(t, len(table.queries), 2, "the second window was fetched")
	for _, query := range table.queries {
		assert.Equal(t, SignatureProjectReferenceIndex, query.indexName)
	}
}

func fakeCompany() *models.Company {
	return &models.Company{CompanyID: "company-1", CompanyName: "Acme"}
}
func fakeClaGroup() *models.ClaGroup {
	return &models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project"}
}

func assertReApproved(t *testing.T, repo repository, signatureID, wantNote string, before *ItemSignature) {
	t.Helper()
	after, err := repo.GetItemSignature(context.Background(), signatureID)
	require.NoError(t, err)
	assert.True(t, after.SignatureApproved)
	assert.Equal(t, wantNote, after.Note)
	assert.Empty(t, after.DateInvalidated)
	assert.Empty(t, after.InvalidatedBy)
	assert.Empty(t, after.InvalidationReason)
	assert.Empty(t, after.InvalidationNote)
	assert.NotEqual(t, before.DateModified, after.DateModified, "date_modified refreshed")
	assert.False(t, signatureInvalidated(after))
}

func attributedItems() []map[string]interface{} {
	items := employeeSignatureItems()
	single := map[string]string{"date_invalidated": "2026-09-01T10:11:12.123456+0000", "invalidated_by": "pcc-admin",
		"invalidation_reason": "left the company", "invalidation_note": "manual"}
	position := 10
	for _, attr := range []string{"date_invalidated", "invalidated_by", "invalidation_reason", "invalidation_note"} {
		item := fakeEclaItem(position, fmt.Sprintf("%s@example.com", attr))
		item["signature_approved"] = fakeFalse()
		item[attr] = fakeS(single[attr])
		items = append(items, item)
		position++
	}
	approvedWithLeftovers := fakeEclaItem(14, "leftovers@example.com")
	approvedWithLeftovers["date_invalidated"] = fakeS("2026-08-01T00:00:00.000000+0000")
	approvedWithLeftovers["invalidated_by"] = fakeS("cla-manager")
	bare := fakeEclaItem(15, "bare.dev@example.com")
	delete(bare, "signature_approved")
	delete(bare, "note")
	history := fakeEclaItem(16, "history.dev@example.com")
	history["note"] = fakeS(legacyHistoryNote)
	return append(items, approvedWithLeftovers, bare, history)
}

// an approved record whose note keeps the trail of an earlier invalidation
const legacyHistoryNote = "Signature invalidated (approved set to false) by pcc-admin for history-dev Re-approved by the CLA manager."

func TestValidateProjectRecordReApprovesDeliberately(t *testing.T) {
	pinnedNote := "attribute_exists(#ID)" + pinnedAs("#S", ":cs")
	blankNote := "attribute_exists(#ID)" + blankAs("#S", ":cs")
	cases := []struct {
		name          string
		signatureID   string
		wantCondition string
		wantNote      string
	}{
		{"every M2 attribute", "sig-002", pinnedNote, "Signature invalidated (approved set to false) due to approval list removal Re-approved by the CLA manager."},
		{"legacy note only", "sig-003", pinnedNote, "Signature invalidated (approved set to false) by pcc-admin for legacy-dev Re-approved by the CLA manager."},
		{"date_invalidated only", "sig-010", blankNote, "Re-approved by the CLA manager."},
		{"invalidated_by only", "sig-011", blankNote, "Re-approved by the CLA manager."},
		{"invalidation_reason only", "sig-012", blankNote, "Re-approved by the CLA manager."},
		{"invalidation_note only", "sig-013", blankNote, "Re-approved by the CLA manager."},
		{"approved record with retained attribution", "sig-014", blankNote, "Re-approved by the CLA manager."},
		{"no approval and no note attributes", "sig-015", blankNote, "Re-approved by the CLA manager."},
		{"approved record with legacy note history", "sig-016", pinnedNote, legacyHistoryNote + " Re-approved by the CLA manager."},
		{"unapproved without evidence", "sig-006", pinnedNote, "created via auto-create, approval pending Re-approved by the CLA manager."},
		{"unsigned", "sig-004", blankNote, "Re-approved by the CLA manager."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, table := newCorporateContributorsRepo(t, attributedItems())
			before, err := repo.GetItemSignature(context.Background(), tc.signatureID)
			require.NoError(t, err)

			require.NoError(t, repo.ValidateProjectRecord(context.Background(), tc.signatureID, " Re-approved by the CLA manager. "))

			require.Len(t, table.updates, 1)
			update := table.updates[0]
			assert.Equal(t, tc.signatureID, aws.StringValue(update.Key["signature_id"].S))
			assert.Equal(t, "SET #A = :a, #S = :s, #M = :m REMOVE #DI, #IB, #IR, #IN", update.UpdateExpression)
			assert.Equal(t, tc.wantCondition, update.ConditionExpression)
			assert.Equal(t, 0, table.conditionFailures)
			assert.Empty(t, table.upserts)
			assertReApproved(t, repo, tc.signatureID, tc.wantNote, before)
		})
	}
}

func TestValidateProjectRecordUnlessInvalidatedReApprovesCleanRecords(t *testing.T) {
	blankAttribution := blankAs("#DI", ":cdi") + blankAs("#IB", ":cib") + blankAs("#IR", ":cir") + blankAs("#IN", ":cin")
	cases := []struct {
		name          string
		signatureID   string
		wantCondition string
		wantNote      string
	}{
		{"unapproved without evidence", "sig-006", "attribute_exists(#ID)" + pinnedAs("#S", ":cs") + blankAs("#A", ":ca") + blankAttribution, "created via auto-create, approval pending approved now"},
		{"unsigned", "sig-004", "attribute_exists(#ID)" + blankAs("#S", ":cs") + pinnedAs("#A", ":ca") + blankAttribution, "approved now"},
		{"no approval and no note attributes", "sig-015", "attribute_exists(#ID)" + blankAs("#S", ":cs") + blankAs("#A", ":ca") + blankAttribution, "approved now"},
		{"approved record with retained attribution", "sig-014", "attribute_exists(#ID)" + blankAs("#S", ":cs") + pinnedAs("#A", ":ca") +
			pinnedAs("#DI", ":cdi") + pinnedAs("#IB", ":cib") + blankAs("#IR", ":cir") + blankAs("#IN", ":cin"), "approved now"},
		{"approved record with legacy note history", "sig-016", "attribute_exists(#ID)" + pinnedAs("#S", ":cs") + pinnedAs("#A", ":ca") + blankAttribution, legacyHistoryNote + " approved now"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := attributedItems()
			items[5]["date_invalidated"] = fakeS("")
			items[5]["invalidation_reason"] = fakeS("")
			repo, table := newCorporateContributorsRepo(t, items)
			before, err := repo.GetItemSignature(context.Background(), tc.signatureID)
			require.NoError(t, err)

			require.NoError(t, repo.ValidateProjectRecordUnlessInvalidated(context.Background(), tc.signatureID, "approved now"))

			require.Len(t, table.updates, 1)
			assert.Equal(t, tc.wantCondition, table.updates[0].ConditionExpression)
			assert.Equal(t, 0, table.conditionFailures, "blank or retained attribution on a record that is not invalidated is pinned as read - the write goes through and removes it")
			assertReApproved(t, repo, tc.signatureID, tc.wantNote, before)
		})
	}
}

func TestValidateProjectRecordUnlessInvalidatedSkipsInvalidatedRecords(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, attributedItems())

	for _, signatureID := range []string{"sig-002", "sig-003", "sig-010", "sig-011", "sig-012", "sig-013"} {
		require.NoError(t, repo.ValidateProjectRecordUnlessInvalidated(context.Background(), signatureID, "approved now"), signatureID)
	}
	assert.Empty(t, table.updates, "M2-attributed and legacy note-only invalidations are left alone without a write")

	after, err := repo.GetItemSignature(context.Background(), "sig-002")
	require.NoError(t, err)
	assert.False(t, after.SignatureApproved)
	assert.Equal(t, "cla-manager", after.InvalidatedBy)
}

func TestValidateProjectRecordUnknownSignature(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, employeeSignatureItems())

	err := repo.ValidateProjectRecord(context.Background(), "sig-missing", "approved now")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature sig-missing not found")

	err = repo.ValidateProjectRecordUnlessInvalidated(context.Background(), "sig-missing", "approved now")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature sig-missing not found")

	assert.Empty(t, table.updates, "a missing record is never upserted into existence")
	assert.Empty(t, table.upserts)
}

func TestValidateProjectRecordDeletedBetweenReadAndWrite(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		t.Run(fmt.Sprintf("automatic=%t", automatic), func(t *testing.T) {
			repo, table := newCorporateContributorsRepo(t, employeeSignatureItems())
			table.beforeUpdate = func(item map[string]interface{}) { table.removeLocked(fakeItemString(item, "signature_id")) }

			var err error
			if automatic {
				err = repo.ValidateProjectRecordUnlessInvalidated(context.Background(), "sig-006", "approved now")
				assert.NoError(t, err, "the automatic path treats a vanished record as a lost race")
			} else {
				err = repo.ValidateProjectRecord(context.Background(), "sig-006", "approved now")
				assert.ErrorIs(t, err, ErrSignatureModifiedConcurrently, "the deliberate path reports it")
			}
			require.Len(t, table.updates, 1)
			assert.Equal(t, 1, table.conditionFailures)
			assert.Empty(t, table.upserts, "the write never recreates the deleted record")
			after, err := repo.GetItemSignature(context.Background(), "sig-006")
			require.NoError(t, err)
			assert.Nil(t, after)
		})
	}
}

func TestValidateProjectRecordConflictsWithAConcurrentNoteChange(t *testing.T) {
	cases := []struct {
		name       string
		concurrent func(item map[string]interface{})
		wantNote   string
	}{
		{"note rewritten after the re-read", func(item map[string]interface{}) {
			item["note"] = fakeS("Signature invalidated (approved set to false) by pcc-admin for user-006 ")
			item["signature_approved"] = fakeFalse()
		}, "Signature invalidated (approved set to false) by pcc-admin for user-006 "},
		{"nonempty note deleted after the re-read", func(item map[string]interface{}) {
			delete(item, "note")
			item["signature_approved"] = fakeFalse()
		}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, table := newCorporateContributorsRepo(t, employeeSignatureItems())
			table.beforeUpdate = tc.concurrent

			err := repo.ValidateProjectRecord(context.Background(), "sig-006", "approved now")

			assert.ErrorIs(t, err, ErrSignatureModifiedConcurrently)
			assert.Contains(t, err.Error(), "sig-006")
			require.Len(t, table.updates, 1)
			assert.Equal(t, 1, table.conditionFailures)
			after, err := repo.GetItemSignature(context.Background(), "sig-006")
			require.NoError(t, err)
			assert.False(t, after.SignatureApproved, "the concurrent write is left intact")
			assert.Equal(t, tc.wantNote, after.Note, "the old note is not restored")
		})
	}
}

// the deliberate write pins the note it appends to, not the approval it is about to set
func TestValidateProjectRecordReApprovesAfterAConcurrentApprovalRemoval(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, attributedItems())
	before, err := repo.GetItemSignature(context.Background(), "sig-016")
	require.NoError(t, err)
	table.beforeUpdate = func(item map[string]interface{}) { delete(item, "signature_approved") }

	require.NoError(t, repo.ValidateProjectRecord(context.Background(), "sig-016", "approved now"))

	require.Len(t, table.updates, 1)
	assert.Equal(t, 0, table.conditionFailures)
	assertReApproved(t, repo, "sig-016", legacyHistoryNote+" approved now", before)
}

func invalidateWithAttribution(item map[string]interface{}) {
	item["signature_approved"] = fakeFalse()
	item["note"] = fakeS("Signature invalidated (approved set to false) due to approval list removal")
	item["date_invalidated"] = fakeS("2026-09-15T10:00:00.000000+0000")
	item["invalidated_by"] = fakeS("cla-manager")
}

func TestValidateProjectRecordUnlessInvalidatedLosesTheRaceAgainstAConcurrentInvalidation(t *testing.T) {
	cases := []struct {
		name        string
		signatureID string
		concurrent  func(item map[string]interface{})
		wantNote    string
	}{
		{"M2 attribution stamped after the re-read", "sig-006", invalidateWithAttribution, "Signature invalidated (approved set to false) due to approval list removal"},
		{"legacy note-only invalidation after the re-read", "sig-006", func(item map[string]interface{}) {
			item["signature_approved"] = fakeFalse()
			item["note"] = fakeS("Signature invalidated (approved set to false) by pcc-admin for user-006 ")
		}, "Signature invalidated (approved set to false) by pcc-admin for user-006 "},
		{"approval revoked after the re-read", "sig-004", func(item map[string]interface{}) {
			item["signature_approved"] = fakeFalse()
		}, ""},
		{"nonempty note deleted after the re-read", "sig-006", func(item map[string]interface{}) {
			delete(item, "note")
		}, ""},
		{"true approval deleted with retained legacy history", "sig-016", func(item map[string]interface{}) {
			delete(item, "signature_approved")
		}, legacyHistoryNote},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, table := newCorporateContributorsRepo(t, attributedItems())
			before, err := repo.GetItemSignature(context.Background(), tc.signatureID)
			require.NoError(t, err)
			table.beforeUpdate = tc.concurrent

			require.NoError(t, repo.ValidateProjectRecordUnlessInvalidated(context.Background(), tc.signatureID, "approved now"), "losing the race is not an error")

			require.Len(t, table.updates, 1, "the conditional write was attempted once")
			assert.Equal(t, 1, table.conditionFailures)

			after, err := repo.GetItemSignature(context.Background(), tc.signatureID)
			require.NoError(t, err)
			assert.False(t, after.SignatureApproved, "the concurrent write wins")
			assert.Equal(t, tc.wantNote, after.Note, "nothing was appended or restored")
			assert.NotEqual(t, before.Note+" approved now", after.Note)
			assert.Equal(t, before.DateModified, after.DateModified, "date_modified untouched")
			if after.DateInvalidated != "" {
				assert.Equal(t, "cla-manager", after.InvalidatedBy, "attribution kept intact")
			}
			if tc.signatureID == "sig-016" {
				assert.True(t, signatureInvalidated(after), "the retained history now counts as an invalidation - it is not re-approved automatically")
			}
		})
	}
}

func TestInvalidateThenValidateRoundTrip(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, employeeSignatureItems())
	ctx := context.Background()
	original, err := repo.GetItemSignature(ctx, "sig-001")
	require.NoError(t, err)
	require.True(t, original.SignatureApproved)

	require.NoError(t, repo.InvalidateProjectRecordWithMetadata(ctx, "sig-001", "Signature invalidated (approved set to false) due to approval list removal",
		&InvalidationMetadata{InvalidatedBy: "cla-manager", Reason: "approved list removal (EmailApprovalList)", Note: "left the company"}))
	invalidated, err := repo.GetItemSignature(ctx, "sig-001")
	require.NoError(t, err)
	assert.False(t, invalidated.SignatureApproved)
	assert.Equal(t, "Signature invalidated (approved set to false) due to approval list removal", invalidated.Note)
	assert.NotEmpty(t, invalidated.DateInvalidated)
	assert.Equal(t, invalidated.DateInvalidated, invalidated.DateModified)
	assert.Equal(t, "cla-manager", invalidated.InvalidatedBy)
	assert.Equal(t, "approved list removal (EmailApprovalList)", invalidated.InvalidationReason)
	assert.Equal(t, "left the company", invalidated.InvalidationNote)
	assert.True(t, signatureInvalidated(invalidated))
	assert.True(t, lookupEmployeeSignature(t, repo, "user-001").Invalidated)

	require.NoError(t, repo.InvalidateProjectRecordWithMetadata(ctx, "sig-001", "Signature invalidated (approved set to false) by pcc-admin",
		&InvalidationMetadata{InvalidatedBy: "pcc-admin", Reason: "manual", Note: "second"}))
	twice, err := repo.GetItemSignature(ctx, "sig-001")
	require.NoError(t, err)
	assert.Equal(t, "Signature invalidated (approved set to false) by pcc-admin", twice.Note)
	assert.Equal(t, invalidated.DateInvalidated, twice.DateInvalidated, "first write wins")
	assert.Equal(t, "cla-manager", twice.InvalidatedBy)
	assert.Equal(t, "approved list removal (EmailApprovalList)", twice.InvalidationReason)
	assert.Equal(t, "left the company", twice.InvalidationNote)

	require.NoError(t, repo.ValidateProjectRecordUnlessInvalidated(ctx, "sig-001", "auto"))
	require.Len(t, table.updates, 2, "the automatic path does not touch an invalidated record")

	require.NoError(t, repo.ValidateProjectRecord(ctx, "sig-001", "Re-approved by the CLA manager."))
	require.Len(t, table.updates, 3)
	assert.Equal(t, 0, table.conditionFailures)
	assertReApproved(t, repo, "sig-001", "Signature invalidated (approved set to false) by pcc-admin Re-approved by the CLA manager.", original)
	result := lookupEmployeeSignature(t, repo, "user-001")
	require.NotNil(t, result.Signature)
	assert.True(t, result.Signature.SignatureApproved)
	assert.False(t, result.Invalidated)
}

func TestReinvalidateAfterApprovalListRemoval(t *testing.T) {
	const removalReason = ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")"
	const removalNote = "Signature invalidated (approved set to false) by cla-manager due to " + utils.EmailCriteria + "  removal"
	const removedAt = "2026-09-15T10:00:00.000000+0000"
	const deliberateNote = "Signature invalidated (approved set to false) by org-admin for user-001"

	voidedByRemoval := func(item map[string]interface{}) {
		item["signature_approved"] = fakeFalse()
		item["note"] = fakeS(removalNote)
		item["date_invalidated"] = fakeS(removedAt)
		item["invalidated_by"] = fakeS("cla-manager")
		item["invalidation_reason"] = fakeS(removalReason)
	}

	t.Run("the deliberate invalidation replaces the removal attribution", func(t *testing.T) {
		items := employeeSignatureItems()
		voidedByRemoval(items[0])
		repo, table := newCorporateContributorsRepo(t, items)
		ctx := context.Background()
		voided, err := repo.GetItemSignature(ctx, "sig-001")
		require.NoError(t, err)
		require.True(t, voided.InvalidatedByApprovalListRemoval())

		require.NoError(t, repo.ReinvalidateProjectRecordWithMetadata(ctx, voided, deliberateNote,
			&InvalidationMetadata{InvalidatedBy: "org-admin", Reason: "compliance", Note: "per legal review"}))

		require.Len(t, table.updates, 1)
		assert.Equal(t, "SET  #A = :a, #S = :s, #DI = :di, #IB = :ib, #IR = :ir, #IN = :in, #M = :m", table.updates[0].UpdateExpression)
		assert.Equal(t, "attribute_exists(#ID) AND (attribute_not_exists(#A) OR #A = :ca) AND #S = :cs AND #DI = :cdi AND #IB = :cib AND #IR = :cir AND (attribute_not_exists(#IN) OR #IN = :cin)",
			table.updates[0].ConditionExpression)
		assert.Equal(t, 0, table.conditionFailures)
		assert.Equal(t, "sig-001", aws.StringValue(table.updates[0].Key["signature_id"].S))
		after, err := repo.GetItemSignature(ctx, "sig-001")
		require.NoError(t, err)
		assert.False(t, after.SignatureApproved)
		assert.Equal(t, deliberateNote, after.Note)
		assert.NotEqual(t, removedAt, after.DateInvalidated, "the removal date is replaced")
		assert.Equal(t, after.DateModified, after.DateInvalidated)
		assert.Equal(t, "org-admin", after.InvalidatedBy)
		assert.Equal(t, "compliance", after.InvalidationReason)
		assert.Equal(t, "per legal review", after.InvalidationNote)
		assert.False(t, after.InvalidatedByApprovalListRemoval(), "a second deliberate invalidation now conflicts")
		assert.True(t, signatureInvalidated(after))
		assert.True(t, lookupEmployeeSignature(t, repo, "user-001").Invalidated)

		require.NoError(t, repo.InvalidateProjectRecordWithMetadata(ctx, "sig-001", "Signature invalidated (approved set to false) due to approval list removal",
			&InvalidationMetadata{InvalidatedBy: "cla-manager", Reason: removalReason}))
		later, err := repo.GetItemSignature(ctx, "sig-001")
		require.NoError(t, err)
		assert.Equal(t, "org-admin", later.InvalidatedBy, "a later removal keeps the deliberate attribution")
		assert.Equal(t, "compliance", later.InvalidationReason)
		assert.Equal(t, "per legal review", later.InvalidationNote)
		assert.Equal(t, after.DateInvalidated, later.DateInvalidated)
	})

	t.Run("attribution the deliberate invalidation does not supply is removed", func(t *testing.T) {
		items := employeeSignatureItems()
		voidedByRemoval(items[0])
		repo, table := newCorporateContributorsRepo(t, items)
		ctx := context.Background()
		voided, err := repo.GetItemSignature(ctx, "sig-001")
		require.NoError(t, err)

		require.NoError(t, repo.ReinvalidateProjectRecordWithMetadata(ctx, voided, deliberateNote, &InvalidationMetadata{InvalidatedBy: "org-admin"}))

		require.Len(t, table.updates, 1)
		assert.Equal(t, "SET  #A = :a, #S = :s, #DI = :di, #IB = :ib, #M = :m REMOVE #IR, #IN", table.updates[0].UpdateExpression)
		after, err := repo.GetItemSignature(ctx, "sig-001")
		require.NoError(t, err)
		assert.False(t, after.SignatureApproved)
		assert.Equal(t, deliberateNote, after.Note)
		assert.Equal(t, "org-admin", after.InvalidatedBy)
		assert.Empty(t, after.InvalidationReason, "the stale removal reason does not survive")
		assert.Empty(t, after.InvalidationNote)
		assert.False(t, after.InvalidatedByApprovalListRemoval())
	})

	t.Run("a legacy note-only removal is re-invalidated the same way and can be re-approved", func(t *testing.T) {
		items := employeeSignatureItems()
		items[0]["signature_approved"] = fakeFalse()
		items[0]["note"] = fakeS(removalNote)
		repo, table := newCorporateContributorsRepo(t, items)
		ctx := context.Background()
		legacy, err := repo.GetItemSignature(ctx, "sig-001")
		require.NoError(t, err)
		require.True(t, legacy.InvalidatedByApprovalListRemoval())

		require.NoError(t, repo.ReinvalidateProjectRecordWithMetadata(ctx, legacy, deliberateNote,
			&InvalidationMetadata{InvalidatedBy: "org-admin", Reason: "compliance"}))

		require.Len(t, table.updates, 1)
		assert.Equal(t, "SET  #A = :a, #S = :s, #DI = :di, #IB = :ib, #IR = :ir, #M = :m REMOVE #IN", table.updates[0].UpdateExpression)
		assert.Equal(t, "attribute_exists(#ID) AND (attribute_not_exists(#A) OR #A = :ca) AND #S = :cs AND (attribute_not_exists(#DI) OR #DI = :cdi) AND (attribute_not_exists(#IB) OR #IB = :cib)"+
			" AND (attribute_not_exists(#IR) OR #IR = :cir) AND (attribute_not_exists(#IN) OR #IN = :cin)", table.updates[0].ConditionExpression)
		assert.Equal(t, 0, table.conditionFailures)
		after, err := repo.GetItemSignature(ctx, "sig-001")
		require.NoError(t, err)
		assert.False(t, after.SignatureApproved)
		assert.Equal(t, deliberateNote, after.Note)
		assert.Equal(t, after.DateModified, after.DateInvalidated)
		assert.Equal(t, "org-admin", after.InvalidatedBy)
		assert.Equal(t, "compliance", after.InvalidationReason)
		assert.Empty(t, after.InvalidationNote)
		assert.False(t, after.InvalidatedByApprovalListRemoval())

		require.NoError(t, repo.ValidateProjectRecord(ctx, "sig-001", "Re-approved by the CLA manager."))
		require.Len(t, table.updates, 2)
		assertReApproved(t, repo, "sig-001", deliberateNote+" Re-approved by the CLA manager.", legacy)
	})

	t.Run("a deliberate invalidation that lands first wins and the stale one conflicts", func(t *testing.T) {
		items := employeeSignatureItems()
		voidedByRemoval(items[0])
		repo, table := newCorporateContributorsRepo(t, items)
		ctx := context.Background()
		voided, err := repo.GetItemSignature(ctx, "sig-001")
		require.NoError(t, err)
		table.beforeUpdate = func(item map[string]interface{}) {
			item["note"] = fakeS("Signature invalidated (approved set to false) by other-admin for user-001")
			item["date_invalidated"] = fakeS("2026-09-24T09:00:00.000000+0000")
			item["invalidated_by"] = fakeS("other-admin")
			item["invalidation_reason"] = fakeS("should-be-corporate")
			delete(item, "invalidation_note")
		}

		err = repo.ReinvalidateProjectRecordWithMetadata(ctx, voided, deliberateNote,
			&InvalidationMetadata{InvalidatedBy: "org-admin", Reason: "compliance", Note: "per legal review"})
		assert.ErrorIs(t, err, ErrSignatureModifiedConcurrently)

		require.Len(t, table.updates, 1)
		assert.Equal(t, 1, table.conditionFailures)
		after, err := repo.GetItemSignature(ctx, "sig-001")
		require.NoError(t, err)
		assert.False(t, after.SignatureApproved)
		assert.Equal(t, "other-admin", after.InvalidatedBy, "the first deliberate attribution is kept")
		assert.Equal(t, "should-be-corporate", after.InvalidationReason)
		assert.Equal(t, "2026-09-24T09:00:00.000000+0000", after.DateInvalidated)
		assert.Empty(t, after.InvalidationNote)
		assert.False(t, after.InvalidatedByApprovalListRemoval())
	})

	t.Run("a re-approval that lands first is not silently undone", func(t *testing.T) {
		items := employeeSignatureItems()
		voidedByRemoval(items[0])
		repo, table := newCorporateContributorsRepo(t, items)
		ctx := context.Background()
		voided, err := repo.GetItemSignature(ctx, "sig-001")
		require.NoError(t, err)
		table.beforeUpdate = func(item map[string]interface{}) {
			item["signature_approved"] = fakeTrue()
			item["note"] = fakeS(removalNote + " Re-approved by the CLA manager.")
			for _, attribute := range []string{"date_invalidated", "invalidated_by", "invalidation_reason", "invalidation_note"} {
				delete(item, attribute)
			}
		}

		err = repo.ReinvalidateProjectRecordWithMetadata(ctx, voided, deliberateNote, &InvalidationMetadata{InvalidatedBy: "org-admin", Reason: "compliance"})
		assert.ErrorIs(t, err, ErrSignatureModifiedConcurrently)

		assert.Equal(t, 1, table.conditionFailures)
		after, err := repo.GetItemSignature(ctx, "sig-001")
		require.NoError(t, err)
		assert.True(t, after.SignatureApproved)
		assert.Empty(t, after.InvalidatedBy)
		assert.Empty(t, after.InvalidationReason)
		assert.False(t, signatureInvalidated(after))
	})
}

func TestCreateProjectCompanyEmployeeSignatureRaceWithInvalidation(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, employeeSignatureItems())
	table.beforeUpdate = invalidateWithAttribution

	require.NoError(t, repo.CreateProjectCompanyEmployeeSignature(context.Background(), fakeCompany(), fakeClaGroup(), &models.User{UserID: "user-006"}))

	require.Len(t, table.updates, 1)
	assert.Equal(t, 1, table.conditionFailures)
	assert.Empty(t, table.puts, "a lost race does not fall through to creating a second record")
	after, err := repo.GetItemSignature(context.Background(), "sig-006")
	require.NoError(t, err)
	assert.False(t, after.SignatureApproved)
	assert.Equal(t, "2026-09-15T10:00:00Z", formatStoredTime(after.DateInvalidated))
	assert.True(t, signatureInvalidated(after))

	table.beforeUpdate = nil
	require.NoError(t, repo.CreateProjectCompanyEmployeeSignature(context.Background(), fakeCompany(), fakeClaGroup(), &models.User{UserID: "user-006"}))
	assert.Len(t, table.updates, 1, "once invalidated the record is recognized as such and no write is attempted")
	assert.Empty(t, table.puts)
}

func TestCreateProjectCompanyEmployeeSignatureCreatesAMissingRecord(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, employeeSignatureItems())

	require.NoError(t, repo.CreateProjectCompanyEmployeeSignature(context.Background(), fakeCompany(), fakeClaGroup(),
		&models.User{UserID: "user-009", GithubUsername: "nine", Emails: []string{"nine@example.com"}}))

	assert.Empty(t, table.updates)
	require.Len(t, table.puts, 1)
	put := table.puts[0]
	assert.Equal(t, "ecla", fakeItemString(put, "signature_type"))
	assert.Equal(t, "cla-group-1", fakeItemString(put, "signature_project_id"))
	assert.Equal(t, "user-009", fakeItemString(put, "signature_reference_id"))
	assert.Equal(t, "company-1", fakeItemString(put, "signature_user_ccla_company_id"))
	// the create path writes the sig_type_* variant; the GSI key sigtype_signed_approved_id is
	// stamped by the dynamo-events stream handler (v2/dynamo_events/signatures.go)
	assert.Equal(t, "ecla#true#true#company-1", fakeItemString(put, "sig_type_signed_approved_id"))
	assert.NotContains(t, put, "sigtype_signed_approved_id")
	assert.Equal(t, "nine", fakeItemString(put, "signature_reference_name"))
	assert.Equal(t, map[string]interface{}{"BOOL": true}, put["signature_approved"])
	assert.Equal(t, map[string]interface{}{"BOOL": true}, put["signature_signed"])

	result := lookupEmployeeSignature(t, repo, "user-009")
	require.NotNil(t, result.Signature, "the created record is found by the lookup")
	assert.Equal(t, fakeItemString(put, "signature_id"), result.Signature.SignatureID)
	assert.False(t, result.Invalidated)
}

// runWithDeadline guards the caller paths against the historical deadlock: a lookup error on an
// unbuffered channel that nobody was reading yet
func runWithDeadline(t *testing.T, run func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- run() }()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("call did not return - lookup error deadlock")
		return nil
	}
}

func TestCreateProjectCompanyEmployeeSignatureReturnsLookupErrors(t *testing.T) {
	cases := []struct {
		name        string
		items       []map[string]interface{}
		failQueryAt int
	}{
		{"first page fails", employeeSignatureItems(), 1},
		{"a later page fails", func() []map[string]interface{} {
			items := make([]map[string]interface{}, 0, 111)
			for i := 0; i < 105; i++ {
				other := fakeEclaItem(200+i, "removed.dev@example.com")
				other["signature_reference_id"] = fakeS("user-002")
				other["signature_user_ccla_company_id"] = fakeS(fmt.Sprintf("company-%d", 200+i))
				items = append(items, other)
			}
			return append(items, employeeSignatureItems()...)
		}(), 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, table := newCorporateContributorsRepo(t, tc.items)
			table.failQueryAt = tc.failQueryAt

			err := runWithDeadline(t, func() error {
				return repo.CreateProjectCompanyEmployeeSignature(context.Background(), fakeCompany(), fakeClaGroup(), &models.User{UserID: "user-002"})
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "injected query failure")
			assert.Len(t, table.queries, tc.failQueryAt)
			assert.Empty(t, table.puts, "a failed lookup must not create a duplicate acknowledgment")
			assert.Empty(t, table.updates)
		})
	}
}

func TestProcessEmployeeSignaturesReturnsLookupErrors(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, employeeSignatureItems())
	table.failQueryAt = 1
	s := service{repo: repo}

	err := runWithDeadline(t, func() error {
		return s.processEmployeeSignatures(context.Background(), fakeCompany(), fakeClaGroup(), []*models.User{{UserID: "user-002"}})
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected query failure")
	assert.Empty(t, table.puts)
	assert.Empty(t, table.updates)
}

func TestProcessEmployeeSignatureReturnsLookupErrors(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, employeeSignatureItems())
	table.failQueryAt = 1
	s := service{repo: repo}

	var signed *bool
	err := runWithDeadline(t, func() error {
		var err error
		signed, err = s.ProcessEmployeeSignature(context.Background(), fakeCompany(), fakeClaGroup(), &models.User{UserID: "user-001"})
		return err
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected query failure")
	require.NotNil(t, signed)
	assert.False(t, *signed, "a failed lookup never reads as signed")
}

func TestCreateProjectCompanyEmployeeSignatureLeavesInvalidatedRecordsAlone(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, employeeSignatureItems())
	company, claGroup := fakeCompany(), fakeClaGroup()

	for _, userID := range []string{"user-002", "user-003"} {
		require.NoError(t, repo.CreateProjectCompanyEmployeeSignature(context.Background(), company, claGroup, &models.User{UserID: userID}))
	}
	assert.Empty(t, table.updates, "invalidated acknowledgments are neither re-approved nor re-created")

	require.NoError(t, repo.CreateProjectCompanyEmployeeSignature(context.Background(), company, claGroup, &models.User{UserID: "user-006"}))
	require.Len(t, table.updates, 1, "an unapproved record without invalidation evidence is still re-validated")
	update := table.updates[0]
	assert.Equal(t, "sig-006", aws.StringValue(update.Key["signature_id"].S))
	assert.Contains(t, update.UpdateExpression, "REMOVE #DI, #IB, #IR, #IN")
	note := aws.StringValue(update.ExpressionAttributeValues[":s"].S)
	assert.Contains(t, note, "created via auto-create, approval pending Enabled previously disabled employee acknowledgment via CLA Manager approval list edit with auto-enable feature flag configured on ")

	require.NoError(t, repo.CreateProjectCompanyEmployeeSignature(context.Background(), company, claGroup, &models.User{UserID: "user-001"}))
	assert.Len(t, table.updates, 1, "an approved record is left untouched")
}
