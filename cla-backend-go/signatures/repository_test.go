// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

func TestInvalidationUpdateExpression(t *testing.T) {
	const now = "2024-05-06T07:08:09.000000+0000"

	names, values, expr := invalidationUpdateExpression("a note", now, &InvalidationMetadata{
		InvalidatedBy: "admin-user",
		Reason:        "compliance",
		Note:          "per legal review",
	}, false)

	assert.Contains(t, expr, "#A = :a")
	assert.Contains(t, expr, "#S = :s")
	assert.Contains(t, expr, "#DI = if_not_exists(#DI, :di)")
	assert.Contains(t, expr, "#IB = if_not_exists(#IB, :ib)", "a re-invalidation must not overwrite the first actor")
	assert.Contains(t, expr, "#IR = if_not_exists(#IR, :ir)", "a re-invalidation must not overwrite the first reason")
	assert.Contains(t, expr, "#IN = if_not_exists(#IN, :in)", "a re-invalidation must not overwrite the first note")
	assert.Contains(t, expr, "#M = :m")

	assert.Equal(t, "invalidated_by", *names["#IB"])
	assert.Equal(t, "invalidation_reason", *names["#IR"])
	assert.Equal(t, "invalidation_note", *names["#IN"])
	assert.Equal(t, "admin-user", *values[":ib"].S)
	assert.Equal(t, "compliance", *values[":ir"].S)
	assert.Equal(t, "per legal review", *values[":in"].S)
	assert.Equal(t, now, *values[":di"].S)
	assert.False(t, *values[":a"].BOOL)
}

func TestInvalidationUpdateExpressionWithoutMetadata(t *testing.T) {
	const now = "2024-05-06T07:08:09.000000+0000"

	for _, metadata := range []*InvalidationMetadata{nil, {}} {
		names, values, expr := invalidationUpdateExpression("a note", now, metadata, false)

		assert.NotContains(t, expr, "#IB")
		assert.NotContains(t, expr, "#IR")
		assert.NotContains(t, expr, "#IN")
		assert.Contains(t, expr, "#DI = if_not_exists(#DI, :di)")
		assert.NotContains(t, names, "#IB")
		assert.NotContains(t, values, ":ib")
	}
}

func TestInvalidationUpdateExpressionModes(t *testing.T) {
	const now = "2024-05-06T07:08:09.000000+0000"
	full := &InvalidationMetadata{InvalidatedBy: "admin-user", Reason: "compliance", Note: "per legal review"}
	attributes := map[string]string{"#A": "signature_approved", "#S": "note", "#DI": "date_invalidated",
		"#IB": "invalidated_by", "#IR": "invalidation_reason", "#IN": "invalidation_note", "#M": "date_modified"}

	cases := []struct {
		name       string
		metadata   *InvalidationMetadata
		overwrite  bool
		expr       string
		wantNames  []string
		wantValues []string
	}{
		{"first write wins with full attribution", full, false,
			"SET  #A = :a, #S = :s, #DI = if_not_exists(#DI, :di), #IB = if_not_exists(#IB, :ib), #IR = if_not_exists(#IR, :ir), #IN = if_not_exists(#IN, :in), #M = :m",
			[]string{"#A", "#S", "#DI", "#IB", "#IR", "#IN", "#M"}, []string{":a", ":s", ":di", ":ib", ":ir", ":in", ":m"}},
		{"first write wins with partial attribution", &InvalidationMetadata{InvalidatedBy: "admin-user"}, false,
			"SET  #A = :a, #S = :s, #DI = if_not_exists(#DI, :di), #IB = if_not_exists(#IB, :ib), #M = :m",
			[]string{"#A", "#S", "#DI", "#IB", "#M"}, []string{":a", ":s", ":di", ":ib", ":m"}},
		{"first write wins without attribution", nil, false,
			"SET  #A = :a, #S = :s, #DI = if_not_exists(#DI, :di), #M = :m",
			[]string{"#A", "#S", "#DI", "#M"}, []string{":a", ":s", ":di", ":m"}},
		{"overwrite with full attribution", full, true,
			"SET  #A = :a, #S = :s, #DI = :di, #IB = :ib, #IR = :ir, #IN = :in, #M = :m",
			[]string{"#A", "#S", "#DI", "#IB", "#IR", "#IN", "#M"}, []string{":a", ":s", ":di", ":ib", ":ir", ":in", ":m"}},
		{"overwrite removes the attribution it does not supply", &InvalidationMetadata{InvalidatedBy: "admin-user"}, true,
			"SET  #A = :a, #S = :s, #DI = :di, #IB = :ib, #M = :m REMOVE #IR, #IN",
			[]string{"#A", "#S", "#DI", "#IB", "#IR", "#IN", "#M"}, []string{":a", ":s", ":di", ":ib", ":m"}},
		{"overwrite without attribution", nil, true,
			"SET  #A = :a, #S = :s, #DI = :di, #M = :m REMOVE #IB, #IR, #IN",
			[]string{"#A", "#S", "#DI", "#IB", "#IR", "#IN", "#M"}, []string{":a", ":s", ":di", ":m"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			names, values, expr := invalidationUpdateExpression("a note", now, tc.metadata, tc.overwrite)
			assert.Equal(t, tc.expr, expr)
			var gotNames, gotValues []string
			for name, attribute := range names {
				gotNames = append(gotNames, name)
				assert.Equal(t, attributes[name], *attribute)
			}
			for value := range values {
				gotValues = append(gotValues, value)
			}
			assert.ElementsMatch(t, tc.wantNames, gotNames)
			assert.ElementsMatch(t, tc.wantValues, gotValues)
			assert.False(t, *values[":a"].BOOL)
			assert.Equal(t, "a note", *values[":s"].S)
			assert.Equal(t, now, *values[":di"].S)
			assert.Equal(t, now, *values[":m"].S)
		})
	}
}

func TestReinvalidationCondition(t *testing.T) {
	attributes := map[string]string{"#ID": "signature_id", "#A": "signature_approved", "#S": "note", "#DI": "date_invalidated",
		"#IB": "invalidated_by", "#IR": "invalidation_reason", "#IN": "invalidation_note", "#M": "date_modified"}
	cases := []struct {
		name      string
		existing  *ItemSignature
		condition string
	}{
		{"attributed removal snapshot", &ItemSignature{SignatureID: "sig-1", Note: "removal note", DateInvalidated: "2026-09-15T10:00:00.000000+0000",
			InvalidatedBy: "cla-manager", InvalidationReason: ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")", InvalidationNote: "left"},
			"attribute_exists(#ID) AND (attribute_not_exists(#A) OR #A = :ca) AND #S = :cs AND #DI = :cdi AND #IB = :cib AND #IR = :cir AND #IN = :cin"},
		{"legacy note-only snapshot", &ItemSignature{SignatureID: "sig-1", Note: "removal note"},
			"attribute_exists(#ID) AND (attribute_not_exists(#A) OR #A = :ca) AND #S = :cs AND (attribute_not_exists(#DI) OR #DI = :cdi) AND (attribute_not_exists(#IB) OR #IB = :cib)" +
				" AND (attribute_not_exists(#IR) OR #IR = :cir) AND (attribute_not_exists(#IN) OR #IN = :cin)"},
		{"empty note snapshot", &ItemSignature{SignatureID: "sig-1", InvalidationReason: "compliance"},
			"attribute_exists(#ID) AND (attribute_not_exists(#A) OR #A = :ca) AND (attribute_not_exists(#S) OR #S = :cs) AND (attribute_not_exists(#DI) OR #DI = :cdi)" +
				" AND (attribute_not_exists(#IB) OR #IB = :cib) AND #IR = :cir AND (attribute_not_exists(#IN) OR #IN = :cin)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			names, values, _ := invalidationUpdateExpression("a note", "2024-05-06T07:08:09.000000+0000", &InvalidationMetadata{InvalidatedBy: "admin-user"}, true)
			assert.Equal(t, tc.condition, reinvalidationCondition(tc.existing, names, values))
			for name, attribute := range names {
				assert.Equal(t, attributes[name], *attribute, name)
			}
			assert.False(t, *values[":ca"].BOOL)
			assert.Equal(t, tc.existing.Note, *values[":cs"].S)
			assert.Equal(t, tc.existing.DateInvalidated, *values[":cdi"].S)
			assert.Equal(t, tc.existing.InvalidatedBy, *values[":cib"].S)
			assert.Equal(t, tc.existing.InvalidationReason, *values[":cir"].S)
			assert.Equal(t, tc.existing.InvalidationNote, *values[":cin"].S)
			assert.False(t, *values[":a"].BOOL, "the update placeholders are untouched")
			assert.Equal(t, "a note", *values[":s"].S)
		})
	}
}

func TestItemSignatureInvalidatedByApprovalListRemoval(t *testing.T) {
	const removalNote = "Signature invalidated (approved set to false) by cla-manager due to " + utils.EmailCriteria + "  removal"
	cases := []struct {
		name string
		sig  *ItemSignature
		want bool
	}{
		{"nil record", nil, false},
		{"approved record with a removal reason", &ItemSignature{SignatureApproved: true, InvalidationReason: ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")"}, false},
		{"removal reason", &ItemSignature{InvalidationReason: ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")", Note: removalNote}, true},
		{"removal reason of another criteria", &ItemSignature{InvalidationReason: ApprovalListRemovalReasonPrefix + utils.GitHubOrgCriteria + ")"}, true},
		{"removal reason matches what verifyUserApprovals records", &ItemSignature{InvalidationReason: "approved list removal (Email Criteria)"}, true},
		{"deliberate reason", &ItemSignature{InvalidationReason: "compliance", Note: removalNote}, false},
		{"deliberate reason resembling a removal", &ItemSignature{InvalidationReason: "approved list removal requested by legal"}, false},
		{"pre-attribution removal note", &ItemSignature{Note: removalNote}, true},
		{"pre-attribution removal note with trailing blanks", &ItemSignature{Note: removalNote + " "}, true},
		{"pre-attribution deliberate note", &ItemSignature{Note: "Signature invalidated (approved set to false) by pcc-admin for user-006 "}, false},
		{"pre-attribution note mentioning a removal elsewhere", &ItemSignature{Note: "Signature invalidated (approved set to false) by pcc-admin due to removal of access rights"}, false},
		{"unapproved record without any note", &ItemSignature{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.sig.InvalidatedByApprovalListRemoval())
		})
	}
}

func TestEffectiveApprovals(t *testing.T) {
	// removals subtracted, additions appended, add+remove of the same entry removes it
	assert.Equal(t, []string{"a", "c", "d"},
		effectiveApprovals([]string{"a", "b", "c"}, []string{"d", "b", "a"}, []string{"b"}))
	assert.Empty(t, effectiveApprovals([]string{"a"}, nil, []string{"a"}))
	assert.Equal(t, []string{"a"}, effectiveApprovals(nil, []string{"a"}, nil))

	// entries are trimmed like persistence, so a padded add matches the persisted value
	assert.Equal(t, []string{"janegh"}, effectiveApprovals(nil, []string{" janegh "}, nil))

	// duplicate current entries are deduped like persistence
	assert.Equal(t, []string{"a", "b"}, effectiveApprovals([]string{"a", "a", "b"}, nil, nil))

	// removals stay exact-match on raw entries (persistence parity): a padded remove entry
	// removes nothing
	assert.Equal(t, []string{"a"}, effectiveApprovals([]string{"a"}, nil, []string{" a "}))
}

// a panic inside an invalidation goroutine must never escape - the non-nil entry reaches
// verifyUserApprovals, panics on the nil usersRepo, and recover contains it; the nil entry
// exercises the nil-skip and the nil-safe recovery logging. The ICLA loop shares the same
// containment but is compile-time disabled (invalidateICLAsOnApprovalListRemoval), so only
// the ECLA loop is exercisable here.
func TestInvalidateSignaturesPanicContainment(t *testing.T) {
	repo := repository{}
	eclas := []*models.Signature{
		nil,
		{SignatureID: "sig-1", SignatureReferenceID: "user-1", ProjectID: "p"},
	}

	// the ICLA entry proves the disabled-invalidation contract: with
	// invalidateICLAsOnApprovalListRemoval false the loop is never entered
	icla, ecla := repo.invalidateSignatures(context.Background(),
		&ApprovalList{
			Criteria: utils.EmailDomainCriteria,
			ICLAs:    []*models.IclaSignature{{SignatureID: "icla-1"}},
			ECLAs:    eclas,
		},
		&models.User{}, nil)

	assert.Empty(t, icla)
	assert.Empty(t, ecla)
}

// the approval list removal write only lands while the record is still approved: an approved
// record is invalidated first-write-wins, anything else is reported as not invalidated without
// touching (or creating) the record, and only a real write failure surfaces as an error
func TestInvalidateApprovedProjectRecord(t *testing.T) {
	metadata := &InvalidationMetadata{InvalidatedBy: "manager-lf", Reason: ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")"}
	wantCondition := "attribute_exists(#ID) AND #A = :ca"

	newRepo := func(t *testing.T, table *fakeSignaturesTable) repository {
		t.Helper()
		awsSession, closeServer := newApprovalRemovalSession(t, table)
		t.Cleanup(closeServer)
		return repository{stage: "test", dynamoDBClient: dynamodb.New(awsSession), signatureTableName: "cla-test-signatures"}
	}

	t.Run("an approved record is invalidated with a preserve-mode conditional write", func(t *testing.T) {
		table := &fakeSignaturesTable{items: []map[string]interface{}{{"signature_id": fakeS("sig-1"), "signature_approved": fakeTrue()}}, invalidated: map[string]int{}}
		repo := newRepo(t, table)

		invalidated, err := repo.invalidateApprovedProjectRecord(context.Background(), "sig-1", "removal note", metadata)
		require.NoError(t, err)
		assert.True(t, invalidated)

		table.mu.Lock()
		defer table.mu.Unlock()
		require.Len(t, table.updates, 1)
		update := table.updates[0]
		assert.Equal(t, wantCondition, update.ConditionExpression)
		assert.Equal(t, "signature_id", update.ExpressionAttributeNames["#ID"])
		require.NotNil(t, update.ExpressionAttributeValues[":ca"].BOOL)
		assert.True(t, *update.ExpressionAttributeValues[":ca"].BOOL)
		assert.Contains(t, update.UpdateExpression, "#A = :a")
		assert.Contains(t, update.UpdateExpression, "if_not_exists(#DI, :di)")
		assert.Contains(t, update.UpdateExpression, "if_not_exists(#IB, :ib)")
		assert.Contains(t, update.UpdateExpression, "if_not_exists(#IR, :ir)")
		assert.NotContains(t, update.UpdateExpression, "REMOVE")
		assert.Equal(t, 0, table.conditionFailures)
		assert.Equal(t, map[string]int{"sig-1": 1}, table.invalidated)
		item := table.find("sig-1")
		require.NotNil(t, item)
		assert.Equal(t, "removal note", fakeItemString(item, "note"))
		assert.Equal(t, "manager-lf", fakeItemString(item, "invalidated_by"))
		assert.Equal(t, ApprovalListRemovalReasonPrefix+utils.EmailCriteria+")", fakeItemString(item, "invalidation_reason"))
	})

	t.Run("an already invalidated record is left untouched and reported as not invalidated", func(t *testing.T) {
		table := &fakeSignaturesTable{items: []map[string]interface{}{{
			"signature_id":       fakeS("sig-1"),
			"signature_approved": fakeFalse(),
			"note":               fakeS("Invalidated by manager-lf"),
			"date_invalidated":   fakeS("2024-05-01T00:00:00Z"),
			"invalidated_by":     fakeS("manager-lf"),
		}}, invalidated: map[string]int{}}
		repo := newRepo(t, table)

		invalidated, err := repo.invalidateApprovedProjectRecord(context.Background(), "sig-1", "removal note", metadata)
		require.NoError(t, err)
		assert.False(t, invalidated)

		table.mu.Lock()
		defer table.mu.Unlock()
		require.Len(t, table.updates, 1)
		assert.Equal(t, wantCondition, table.updates[0].ConditionExpression)
		assert.Equal(t, 1, table.conditionFailures)
		assert.Empty(t, table.invalidated)
		item := table.find("sig-1")
		require.NotNil(t, item)
		assert.Equal(t, "Invalidated by manager-lf", fakeItemString(item, "note"))
		assert.Equal(t, "2024-05-01T00:00:00Z", fakeItemString(item, "date_invalidated"))
		_, hasReason := item["invalidation_reason"]
		assert.False(t, hasReason)
		_, hasModified := item["date_modified"]
		assert.False(t, hasModified)
	})

	t.Run("a missing record is neither invalidated nor created", func(t *testing.T) {
		table := &fakeSignaturesTable{invalidated: map[string]int{}}
		repo := newRepo(t, table)

		invalidated, err := repo.invalidateApprovedProjectRecord(context.Background(), "sig-1", "removal note", metadata)
		require.NoError(t, err)
		assert.False(t, invalidated)

		table.mu.Lock()
		defer table.mu.Unlock()
		assert.Equal(t, 1, table.conditionFailures)
		assert.Empty(t, table.upserts)
		assert.Empty(t, table.items)
		assert.Empty(t, table.invalidated)
	})

	t.Run("any other write failure is returned", func(t *testing.T) {
		server := httptest.NewServer(&fakeSignaturesTable{invalidated: map[string]int{}})
		awsSession, err := session.NewSession(&aws.Config{
			Region:      aws.String("us-east-1"),
			Endpoint:    aws.String(server.URL),
			Credentials: credentials.NewStaticCredentials("test", "test", ""),
			DisableSSL:  aws.Bool(true),
			MaxRetries:  aws.Int(0),
		})
		require.NoError(t, err)
		server.Close()
		repo := repository{stage: "test", dynamoDBClient: dynamodb.New(awsSession), signatureTableName: "cla-test-signatures"}

		invalidated, err := repo.invalidateApprovedProjectRecord(context.Background(), "sig-1", "removal note", metadata)
		require.Error(t, err)
		assert.False(t, invalidated)
		assert.False(t, errors.Is(err, ErrSignatureModifiedConcurrently))
	})
}

// a missing user record (GetUser returning nil, nil) must skip the re-check without error
// and without invalidating - invalidation would nil-panic on the repository's dynamoDBClient
func TestVerifyUserApprovalsMissingUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockUsers := mock_users.NewMockUserRepository(ctrl)
	mockUsers.EXPECT().GetUser("missing-user").Return(nil, nil)
	repo := repository{usersRepo: mockUsers}

	user, invalidated, err := repo.verifyUserApprovals(context.Background(),
		"missing-user", "sig-1", &models.User{}, &ApprovalList{Criteria: utils.EmailDomainCriteria})

	assert.Nil(t, user)
	assert.False(t, invalidated)
	assert.NoError(t, err)
}

// userStillApproved receives approval lists that already reflect the full pending update
// (built via effectiveApprovals), so removed entries are simply absent
func TestUserStillApproved(t *testing.T) {
	user := &models.User{
		Emails:         []string{"jane@corp.example"},
		GithubUsername: "janegh",
		GitlabUsername: "janegl",
	}

	// a missing user record can never count as approved
	assert.False(t, userStillApproved(nil, &ApprovalList{
		EmailApprovals: []string{"jane@corp.example"},
	}))

	// GH username removed but still on the email approved list (the #5166 scenario)
	assert.True(t, userStillApproved(user, &ApprovalList{
		EmailApprovals: []string{"jane@corp.example"},
	}))

	// GH username removed, no other coverage
	assert.False(t, userStillApproved(user, &ApprovalList{}))

	// email removed but still covered by the domain approved list
	assert.True(t, userStillApproved(user, &ApprovalList{
		DomainApprovals: []string{"corp.example"},
	}))

	// email removed, remaining GH username approval covers the user
	assert.True(t, userStillApproved(user, &ApprovalList{
		GitHubUsernameApprovals: []string{"janegh"},
	}))

	// GitLab username coverage
	assert.True(t, userStillApproved(user, &ApprovalList{
		GitlabUsernameApprovals: []string{"janegl"},
	}))

	// approvals for other users don't cover this one
	assert.False(t, userStillApproved(user, &ApprovalList{
		GitHubUsernameApprovals: []string{"someoneelse"},
		EmailApprovals:          []string{"other@corp.example"},
	}))

	// cross-criteria removal in one request: email and GH username both removed,
	// effective lists no longer contain either entry, so the user is not covered
	assert.False(t, userStillApproved(user, &ApprovalList{
		EmailApprovals:          effectiveApprovals([]string{"jane@corp.example"}, nil, []string{"jane@corp.example"}),
		GitHubUsernameApprovals: effectiveApprovals([]string{"janegh"}, nil, []string{"janegh"}),
	}))

	// email approval entries match case-insensitively
	assert.True(t, userStillApproved(user, &ApprovalList{
		EmailApprovals: []string{"Jane@Corp.Example"},
	}))

	// domain entries match with the gate's pattern semantics: case-sensitive and untrimmed,
	// so a padded mixed-case entry the gate would not honor grants no coverage here either
	assert.False(t, userStillApproved(user, &ApprovalList{
		DomainApprovals: []string{" Corp.Example "},
	}))

	// usernames fold like the enforcement gate (EqualFold)
	assert.True(t, userStillApproved(user, &ApprovalList{
		GitHubUsernameApprovals: []string{"JaneGH"},
	}))
	assert.True(t, userStillApproved(user, &ApprovalList{
		GitlabUsernameApprovals: []string{"JaneGL"},
	}))

	// wildcard domain entries cover subdomains like the enforcement gate
	assert.True(t, userStillApproved(&models.User{Emails: []string{"jane@dev.example.com"}}, &ApprovalList{
		DomainApprovals: []string{"*.example.com"},
	}))

	// a malformed domain pattern neither panics nor grants coverage
	assert.False(t, userStillApproved(&models.User{Emails: []string{"jane@corp.example"}}, &ApprovalList{
		DomainApprovals: []string{"("},
	}))
}
