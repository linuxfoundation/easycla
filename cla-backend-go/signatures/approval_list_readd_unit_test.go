// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/golang/mock/gomock"
	eventsMock "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readdFakeRepo answers the three restore calls of the service directly
type readdFakeRepo struct {
	SignatureRepository
	ccla    *ItemSignature
	cclaErr error
	// later, when set, answers every consistent read of the corporate signature after the first one
	later          func(read int) (*ItemSignature, error)
	candidates     []*ItemSignature
	candidatesErr  error
	restoreErr     map[string]error
	restoreSkipped map[string]bool
	cclaCalls      int
	candidateCalls int
	restored       []string
	notes          []string
}

func (f *readdFakeRepo) GetItemSignatureConsistent(_ context.Context, _ string) (*ItemSignature, error) {
	f.cclaCalls++
	if f.cclaCalls > 1 && f.later != nil {
		return f.later(f.cclaCalls)
	}
	return f.ccla, f.cclaErr
}

func (f *readdFakeRepo) GetRemovalInvalidatedEmployeeSignatures(_ context.Context, _, _ string) ([]*ItemSignature, error) {
	f.candidateCalls++
	return f.candidates, f.candidatesErr
}

func (f *readdFakeRepo) RestoreRemovalInvalidatedEmployeeSignature(_ context.Context, snapshot *ItemSignature, note string) (bool, error) {
	if err := f.restoreErr[snapshot.SignatureID]; err != nil {
		return false, err
	}
	f.restored = append(f.restored, snapshot.SignatureID)
	f.notes = append(f.notes, note)
	return !f.restoreSkipped[snapshot.SignatureID], nil
}

func readdCandidate(signatureID, userID string) *ItemSignature {
	return &ItemSignature{SignatureID: signatureID, SignatureReferenceID: userID, SignatureProjectID: "cla-group-1", SignatureUserCompanyID: "company-1",
		SignatureSigned: true, InvalidationReason: ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")"}
}

// readdDirectService wires the service to the fake repository and a users table whose GetUser may fail per ID
func readdDirectService(t *testing.T, repo *readdFakeRepo, registry *readdRegistry, userErr map[string]error) service {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockUsers := mock_users.NewMockUserRepository(ctrl)
	mockUsers.EXPECT().GetUser(gomock.Any()).DoAndReturn(func(id string) (*models.User, error) {
		if err := userErr[id]; err != nil {
			return nil, err
		}
		return registry.byID(id)
	}).AnyTimes()
	return service{repo: repo, usersService: users.NewService(mockUsers, eventsMock.NewMockService(ctrl))}
}

func TestRestoreRemovalInvalidatedEmployeeSignaturesDirect(t *testing.T) {
	manager := &models.User{UserID: "manager-user", Username: readdManager}
	signedCCLA := func(emails ...string) *ItemSignature {
		return &ItemSignature{SignatureID: "ccla-sig", SignatureSigned: true, SignatureApproved: true, EmailApprovalList: emails}
	}
	addAlice := &models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test"}}
	run := func(t *testing.T, repo *readdFakeRepo, params *models.ApprovalList, userErr map[string]error) ([]*models.User, error) {
		registry := newReaddRegistry()
		registry.add(readdUser("user-001", "alice@acme.test"), readdUser("user-002", "bob@acme.test"))
		svc := readdDirectService(t, repo, registry, userErr)
		return svc.restoreRemovalInvalidatedEmployeeSignatures(context.Background(), manager, fakeClaGroup(), fakeCompany(), "ccla-sig", params)
	}

	t.Run("nil params or no additions restore nothing without reading", func(t *testing.T) {
		for _, params := range []*models.ApprovalList{nil, {}, {RemoveEmailApprovalList: []string{"alice@acme.test"}}} {
			repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test"), candidates: []*ItemSignature{readdCandidate("sig-001", "user-001")}}
			restored, err := run(t, repo, params, nil)
			require.NoError(t, err)
			assert.Nil(t, restored)
			assert.Zero(t, repo.cclaCalls)
			assert.Zero(t, repo.candidateCalls)
		}
	})
	t.Run("corporate signature load failure", func(t *testing.T) {
		repo := &readdFakeRepo{cclaErr: errors.New("dynamo is down")}
		restored, err := run(t, repo, addAlice, nil)
		assert.EqualError(t, err, "unable to load corporate signature ccla-sig: dynamo is down")
		assert.Nil(t, restored)
		assert.Zero(t, repo.candidateCalls)
	})
	t.Run("corporate signature gone, unsigned or unapproved", func(t *testing.T) {
		for _, ccla := range []*ItemSignature{nil, {SignatureID: "ccla-sig", SignatureApproved: true, EmailApprovalList: []string{"alice@acme.test"}},
			{SignatureID: "ccla-sig", SignatureSigned: true, EmailApprovalList: []string{"alice@acme.test"}}} {
			repo := &readdFakeRepo{ccla: ccla, candidates: []*ItemSignature{readdCandidate("sig-001", "user-001")}}
			restored, err := run(t, repo, addAlice, nil)
			require.NoError(t, err)
			assert.Nil(t, restored)
			assert.Zero(t, repo.candidateCalls)
		}
	})
	t.Run("added entry not in effect", func(t *testing.T) {
		repo := &readdFakeRepo{ccla: signedCCLA("keep@acme.test"), candidates: []*ItemSignature{readdCandidate("sig-001", "user-001")}}
		restored, err := run(t, repo, addAlice, nil)
		require.NoError(t, err)
		assert.Nil(t, restored)
		assert.Zero(t, repo.candidateCalls)
	})
	t.Run("candidates load failure", func(t *testing.T) {
		repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test"), candidatesErr: errors.New("index is down")}
		restored, err := run(t, repo, addAlice, nil)
		assert.EqualError(t, err, "unable to load the removal-invalidated employee acknowledgments: index is down")
		assert.Nil(t, restored)
	})
	t.Run("no candidates", func(t *testing.T) {
		repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test")}
		restored, err := run(t, repo, addAlice, nil)
		require.NoError(t, err)
		assert.Nil(t, restored)
		assert.Equal(t, 1, repo.candidateCalls)
	})
	t.Run("restores the covered users, once each, with the attributed note", func(t *testing.T) {
		repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test", "bob@acme.test"), candidates: []*ItemSignature{
			readdCandidate("sig-001", "user-001"), readdCandidate("sig-002", "user-002"), readdCandidate("sig-003", "user-001")}}
		restored, err := run(t, repo, &models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test"}}, nil)
		require.NoError(t, err)
		require.Len(t, restored, 1, "bob's entry was not part of this edit")
		assert.Equal(t, "user-001", restored[0].UserID)
		assert.Equal(t, []string{"sig-001", "sig-003"}, repo.restored)
		for _, note := range repo.notes {
			assert.True(t, strings.HasPrefix(note, readdRestorePrefix) && strings.HasSuffix(note, "Z."), note)
		}
	})
	t.Run("the user of several acknowledgments is loaded once", func(t *testing.T) {
		repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test"), candidates: []*ItemSignature{readdCandidate("sig-001", "user-001"), readdCandidate("sig-003", "user-001")}}
		registry := newReaddRegistry()
		registry.add(readdUser("user-001", "alice@acme.test"))
		svc := readdDirectService(t, repo, registry, nil)
		_, err := svc.restoreRemovalInvalidatedEmployeeSignatures(context.Background(), manager, fakeClaGroup(), fakeCompany(), "ccla-sig", addAlice)
		require.NoError(t, err)
		assert.Equal(t, 1, registry.lookups("user-001"))
	})
	t.Run("unknown user is skipped without error", func(t *testing.T) {
		repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test"), candidates: []*ItemSignature{readdCandidate("sig-009", "user-unknown"), readdCandidate("sig-001", "user-001")}}
		restored, err := run(t, repo, addAlice, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"sig-001"}, repo.restored)
		require.Len(t, restored, 1)
	})
	t.Run("a restore reported as not done keeps the user off the list", func(t *testing.T) {
		repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test"), candidates: []*ItemSignature{readdCandidate("sig-001", "user-001")}, restoreSkipped: map[string]bool{"sig-001": true}}
		restored, err := run(t, repo, addAlice, nil)
		require.NoError(t, err)
		assert.Nil(t, restored)
		assert.Equal(t, []string{"sig-001"}, repo.restored)
	})
	t.Run("failures are collected and the rest is still restored", func(t *testing.T) {
		// alice restores, bob's write fails, dave's user cannot be loaded, erin's evaluation trips over a broken domain pattern
		repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test", "bob@acme.test"), candidates: []*ItemSignature{
			readdCandidate("sig-001", "user-001"), readdCandidate("sig-002", "user-002"), readdCandidate("sig-004", "user-004"), readdCandidate("sig-005", "user-005")},
			restoreErr: map[string]error{"sig-002": errors.New("write failed")}}
		repo.ccla.EmailDomainApprovalList = []string{"acme.(test"}
		params := &models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test", "bob@acme.test"}, AddDomainApprovalList: []string{"acme.(test"}}
		registry := newReaddRegistry()
		registry.add(readdUser("user-001", "alice@acme.test"), readdUser("user-002", "bob@acme.test"), readdUser("user-005", "erin@acme.test"))
		svc := readdDirectService(t, repo, registry, map[string]error{"user-004": errors.New("users table is down")})
		restored, err := svc.restoreRemovalInvalidatedEmployeeSignatures(context.Background(), manager, fakeClaGroup(), fakeCompany(), "ccla-sig", params)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to restore signature sig-002: write failed")
		assert.Contains(t, err.Error(), "unable to load user user-004 of signature sig-004: users table is down")
		assert.Contains(t, err.Error(), "unable to evaluate user user-005 of signature sig-005: error parsing regexp")
		require.Len(t, restored, 1)
		assert.Equal(t, "user-001", restored[0].UserID)
		assert.Equal(t, []string{"sig-001"}, repo.restored)
	})
	t.Run("the corporate signature is re-read before every restore", func(t *testing.T) {
		repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test", "bob@acme.test"), candidates: []*ItemSignature{
			readdCandidate("sig-001", "user-001"), readdCandidate("sig-002", "user-002"), readdCandidate("sig-003", "user-001")}}
		restored, err := run(t, repo, &models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test", "bob@acme.test"}}, nil)
		require.NoError(t, err)
		assert.Len(t, restored, 2)
		assert.Equal(t, []string{"sig-001", "sig-002", "sig-003"}, repo.restored)
		assert.Equal(t, 4, repo.cclaCalls, "the initial read plus one per restore")
	})
	t.Run("a removal committed meanwhile stops the restoring", func(t *testing.T) {
		repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test", "bob@acme.test"), candidates: []*ItemSignature{
			readdCandidate("sig-001", "user-001"), readdCandidate("sig-002", "user-002")}}
		repo.later = func(int) (*ItemSignature, error) { return signedCCLA("keep@acme.test"), nil }
		restored, err := run(t, repo, &models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test", "bob@acme.test"}}, nil)
		require.NoError(t, err)
		assert.Nil(t, restored)
		assert.Empty(t, repo.restored)
		assert.Equal(t, 2, repo.cclaCalls, "the remaining candidates are not evaluated once the entries are gone")
	})
	t.Run("a corporate signature deactivated or deleted meanwhile stops the restoring", func(t *testing.T) {
		for _, later := range []*ItemSignature{nil, {SignatureID: "ccla-sig", SignatureSigned: true, EmailApprovalList: []string{"alice@acme.test"}},
			{SignatureID: "ccla-sig", SignatureApproved: true, EmailApprovalList: []string{"alice@acme.test"}}} {
			repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test"), candidates: []*ItemSignature{readdCandidate("sig-001", "user-001"), readdCandidate("sig-003", "user-001")}}
			repo.later = func(int) (*ItemSignature, error) { return later, nil }
			restored, err := run(t, repo, addAlice, nil)
			require.NoError(t, err)
			assert.Nil(t, restored)
			assert.Empty(t, repo.restored)
			assert.Equal(t, 2, repo.cclaCalls)
		}
	})
	t.Run("a failed re-read is reported and stops the restoring", func(t *testing.T) {
		repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test"), candidates: []*ItemSignature{readdCandidate("sig-001", "user-001"), readdCandidate("sig-003", "user-001")}}
		repo.later = func(int) (*ItemSignature, error) { return nil, errors.New("dynamo is down") }
		restored, err := run(t, repo, addAlice, nil)
		assert.EqualError(t, err, "unable to reload corporate signature ccla-sig: dynamo is down")
		assert.Nil(t, restored)
		assert.Empty(t, repo.restored)
	})
	t.Run("a list changed meanwhile is evaluated again, restoring only the users it still covers", func(t *testing.T) {
		both := &models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test", "bob@acme.test"}}
		for _, order := range [][]*ItemSignature{{readdCandidate("sig-001", "user-001"), readdCandidate("sig-002", "user-002")},
			{readdCandidate("sig-002", "user-002"), readdCandidate("sig-001", "user-001")}} {
			repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test", "bob@acme.test"), candidates: order}
			repo.later = func(int) (*ItemSignature, error) { return signedCCLA("alice@acme.test"), nil }
			restored, err := run(t, repo, both, nil)
			require.NoError(t, err)
			require.Len(t, restored, 1)
			assert.Equal(t, "user-001", restored[0].UserID)
			assert.Equal(t, []string{"sig-001"}, repo.restored)
		}
	})
	t.Run("a list that keeps changing is reported instead of decided", func(t *testing.T) {
		repo := &readdFakeRepo{ccla: signedCCLA("alice@acme.test", "bob@acme.test"), candidates: []*ItemSignature{readdCandidate("sig-001", "user-001")}}
		repo.later = func(read int) (*ItemSignature, error) {
			if read%2 == 0 {
				return signedCCLA("alice@acme.test"), nil
			}
			return signedCCLA("alice@acme.test", "bob@acme.test"), nil
		}
		restored, err := run(t, repo, &models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test", "bob@acme.test"}}, nil)
		assert.EqualError(t, err, "unable to evaluate user user-001 of signature sig-001: the approval list kept changing")
		assert.Nil(t, restored)
		assert.Empty(t, repo.restored)
		assert.Equal(t, 1+restoreDecisionRounds, repo.cclaCalls)
	})
	t.Run("an unanswered GitHub organization lookup is a failure, not a negative decision", func(t *testing.T) {
		previous := listUserPublicOrgs
		listUserPublicOrgs = func(context.Context, string) ([]string, error) { return nil, errors.New("github is down") }
		t.Cleanup(func() { listUserPublicOrgs = previous })
		repo := &readdFakeRepo{ccla: &ItemSignature{SignatureID: "ccla-sig", SignatureSigned: true, SignatureApproved: true, GitHubOrgApprovalList: []string{"acme-org"}},
			candidates: []*ItemSignature{readdCandidate("sig-001", "user-001"), readdCandidate("sig-002", "user-002")}}
		registry := newReaddRegistry()
		alice, bob := readdUser("user-001", "alice@acme.test"), readdUser("user-002", "bob@acme.test")
		alice.GithubUsername = readdAliceGitHub
		registry.add(alice, bob)
		svc := readdDirectService(t, repo, registry, nil)
		restored, err := svc.restoreRemovalInvalidatedEmployeeSignatures(context.Background(), manager, fakeClaGroup(), fakeCompany(), "ccla-sig",
			&models.ApprovalList{AddGithubOrgApprovalList: []string{"acme-org"}})
		assert.EqualError(t, err, "unable to evaluate user user-001 of signature sig-001: the GitHub organization membership lookup failed",
			"bob has no GitHub username, so nothing was looked up for him")
		assert.Nil(t, restored)
		assert.Empty(t, repo.restored)
	})
	t.Run("a direct match is decided without the failing GitHub organization lookup", func(t *testing.T) {
		previous := listUserPublicOrgs
		listUserPublicOrgs = func(context.Context, string) ([]string, error) { return nil, errors.New("github is down") }
		t.Cleanup(func() { listUserPublicOrgs = previous })
		repo := &readdFakeRepo{ccla: &ItemSignature{SignatureID: "ccla-sig", SignatureSigned: true, SignatureApproved: true,
			EmailApprovalList: []string{"alice@acme.test"}, GitHubOrgApprovalList: []string{"acme-org"}},
			candidates: []*ItemSignature{readdCandidate("sig-001", "user-001")}}
		registry := newReaddRegistry()
		alice := readdUser("user-001", "alice@acme.test")
		alice.GithubUsername = readdAliceGitHub
		registry.add(alice)
		svc := readdDirectService(t, repo, registry, nil)
		restored, err := svc.restoreRemovalInvalidatedEmployeeSignatures(context.Background(), manager, fakeClaGroup(), fakeCompany(), "ccla-sig",
			&models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test"}, AddGithubOrgApprovalList: []string{"acme-org"}})
		require.NoError(t, err)
		require.Len(t, restored, 1)
		assert.Equal(t, []string{"sig-001"}, repo.restored)
	})
}

// readdFailingCandidates wraps the real repository and breaks only the candidate listing
type readdFailingCandidates struct {
	SignatureRepository
	err error
}

func (f *readdFailingCandidates) GetRemovalInvalidatedEmployeeSignatures(_ context.Context, _, _ string) ([]*ItemSignature, error) {
	return nil, f.err
}

// TestUpdateApprovalListReAddReportsRecoveryFailure: the edit itself is persisted and its side effects run,
// but an incomplete recovery is reported to the caller so the entries can be re-added to retry
func TestUpdateApprovalListReAddReportsRecoveryFailure(t *testing.T) {
	h := newReaddHarness(t, []map[string]interface{}{readdCCLA{emails: []string{"keep@acme.test"}}.item(), readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria)})
	h.registry.add(readdUser("user-001", "alice@acme.test"), readdUser("user-keep", "keep@acme.test"))
	h.svc.repo = &readdFailingCandidates{SignatureRepository: h.repo, err: errors.New("index is down")}
	before := h.rows()
	logged := atomic.LoadInt64(h.events)

	updated, err := h.call(&models.ApprovalList{AddEmailApprovalList: []string{"alice@acme.test"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "re-add the entries to retry")
	assert.Contains(t, err.Error(), "unable to load the removal-invalidated employee acknowledgments: index is down")
	assert.Nil(t, updated)
	h.assertUntouched(t, before["sig-001"])
	ccla := h.row(t, "ccla-sig")
	assert.Equal(t, fakeStringList("keep@acme.test", "alice@acme.test"), ccla["email_whitelist"], "the list edit was persisted")
	assert.Greater(t, atomic.LoadInt64(h.events), logged, "the edit was logged")
	h.emails.mu.Lock()
	assert.Contains(t, h.emails.recipients, "manager@example.com")
	h.emails.mu.Unlock()
	assert.Contains(t, h.storeReads(), "active_pr:e:alice@acme.test", "alice's pull request status was still refreshed")
}

func TestAddedApprovalCriteria(t *testing.T) {
	ccla := &ItemSignature{SignatureID: "ccla-sig", EmailApprovalList: []string{"a@x.test", "B@x.test"}, EmailDomainApprovalList: []string{"x.test"},
		GitHubUsernameApprovalList: []string{"gh-a"}, GitHubOrgApprovalList: []string{"org-a"}, GitlabUsernameApprovalList: []string{"gl-a"}, GitlabOrgApprovalList: []string{"group-a"}}
	t.Run("entries in effect, trimmed, exact case, deduplicated", func(t *testing.T) {
		added := addedApprovalCriteria(ccla, &models.ApprovalList{
			AddEmailApprovalList:          []string{" a@x.test ", "a@x.test", "b@x.test", "missing@x.test", " "},
			AddDomainApprovalList:         []string{"x.test", "y.test"},
			AddGithubUsernameApprovalList: []string{"GH-A", "gh-a"},
			AddGithubOrgApprovalList:      []string{"org-a"},
			AddGitlabUsernameApprovalList: []string{"gl-a"},
			AddGitlabOrgApprovalList:      []string{"group-a", "group-b"},
		})
		require.NotNil(t, added)
		assert.Equal(t, "ccla-sig", added.SignatureID)
		assert.Equal(t, []string{"a@x.test"}, added.EmailApprovalList)
		assert.Equal(t, []string{"x.test"}, added.DomainApprovalList)
		assert.Equal(t, []string{"gh-a"}, added.GithubUsernameApprovalList)
		assert.Equal(t, []string{"org-a"}, added.GithubOrgApprovalList)
		assert.Equal(t, []string{"gl-a"}, added.GitlabUsernameApprovalList)
		assert.Equal(t, []string{"group-a"}, added.GitlabOrgApprovalList)
	})
	t.Run("nothing in effect", func(t *testing.T) {
		assert.Nil(t, addedApprovalCriteria(ccla, &models.ApprovalList{AddEmailApprovalList: []string{"b@x.test"}, AddDomainApprovalList: []string{"y.test"}}))
		assert.Nil(t, addedApprovalCriteria(ccla, &models.ApprovalList{RemoveEmailApprovalList: []string{"a@x.test"}}))
		assert.Nil(t, addedApprovalCriteria(ccla, &models.ApprovalList{}))
	})
}

func TestAppendMissingUsers(t *testing.T) {
	alice, bob := &models.User{UserID: "user-001"}, &models.User{UserID: "user-002"}
	list := appendMissingUsers(nil, []*models.User{alice, nil, bob, alice})
	require.Len(t, list, 2)
	assert.Same(t, alice, list[0])
	assert.Same(t, bob, list[1])
	list = appendMissingUsers(list, []*models.User{{UserID: "user-002"}, {UserID: "user-003"}})
	require.Len(t, list, 3)
	assert.Equal(t, "user-003", list[2].UserID)
}

func TestRestoreRemovalInvalidatedEmployeeSignatureWrite(t *testing.T) {
	const note = "Re-enabled by the test."
	pins := pinnedAs("#SG", ":csg") + pinnedAs("#RT", ":crt") + pinnedAs("#RID", ":crid") + pinnedAs("#PID", ":cpid") + pinnedAs("#CID", ":ccid")
	cases := []struct {
		name          string
		item          map[string]interface{}
		wantCondition string
	}{
		{"attributed removal", readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria), "attribute_exists(#ID)" + pinnedAs("#S", ":cs") + blankAs("#A", ":ca") +
			pinnedAs("#DI", ":cdi") + pinnedAs("#IB", ":cib") + pinnedAs("#IR", ":cir") + blankAs("#IN", ":cin") + pins},
		{"legacy note-only removal", readdLegacyRemovedRow(1, "alice@acme.test", utils.GitHubUsernameCriteria), "attribute_exists(#ID)" + pinnedAs("#S", ":cs") + blankAs("#A", ":ca") +
			blankAs("#DI", ":cdi") + blankAs("#IB", ":cib") + blankAs("#IR", ":cir") + blankAs("#IN", ":cin") + pins},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newReaddHarness(t, []map[string]interface{}{tc.item})
			before, err := h.repo.GetItemSignature(context.Background(), "sig-001")
			require.NoError(t, err)
			require.True(t, before.InvalidatedByApprovalListRemoval())

			done, err := h.repo.RestoreRemovalInvalidatedEmployeeSignature(context.Background(), before, note)
			require.NoError(t, err)
			assert.True(t, done)
			assertReApproved(t, h.repo, "sig-001", before.Note+" "+note, before)

			h.table.mu.Lock()
			defer h.table.mu.Unlock()
			require.Len(t, h.table.updates, 1)
			update := h.table.updates[0]
			assert.Equal(t, "SET #A = :a, #S = :s, #M = :m REMOVE #DI, #IB, #IR, #IN", update.UpdateExpression)
			assert.Equal(t, tc.wantCondition, update.ConditionExpression)
			assert.Equal(t, map[string]string{"#ID": "signature_id", "#A": "signature_approved", "#S": "note", "#M": "date_modified", "#DI": "date_invalidated",
				"#IB": "invalidated_by", "#IR": "invalidation_reason", "#IN": "invalidation_note", "#SG": "signature_signed", "#RT": "signature_reference_type",
				"#RID": "signature_reference_id", "#PID": "signature_project_id", "#CID": "signature_user_ccla_company_id"}, update.ExpressionAttributeNames)
			values := update.ExpressionAttributeValues
			assert.True(t, *values[":a"].BOOL)
			assert.Equal(t, before.Note+" "+note, *values[":s"].S)
			assert.Equal(t, before.Note, *values[":cs"].S)
			assert.False(t, *values[":ca"].BOOL)
			assert.True(t, *values[":csg"].BOOL)
			assert.Equal(t, before.DateInvalidated, *values[":cdi"].S)
			assert.Equal(t, before.InvalidatedBy, *values[":cib"].S)
			assert.Equal(t, before.InvalidationReason, *values[":cir"].S)
			assert.Equal(t, "", *values[":cin"].S)
			assert.Equal(t, "user", *values[":crt"].S)
			assert.Equal(t, "user-001", *values[":crid"].S)
			assert.Equal(t, "cla-group-1", *values[":cpid"].S)
			assert.Equal(t, "company-1", *values[":ccid"].S)
			assert.Empty(t, h.table.upserts)
			assert.Zero(t, h.table.conditionFailures)
		})
	}
}

func TestRestoreRemovalInvalidatedEmployeeSignatureRefuses(t *testing.T) {
	unsigned := readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria)
	unsigned["signature_signed"] = fakeFalse()
	moved := readdCandidate("sig-001", "user-999")
	manualNote := readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria)
	manualNote["note"] = fakeS("Signature invalidated (approved set to false) by pcc-admin for user-001")
	withInvalidationNote := readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria)
	withInvalidationNote["invalidation_note"] = fakeS("manual")
	companyReference := readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria)
	companyReference["signature_reference_type"] = fakeS("company")
	cclaTyped := readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria)
	cclaTyped["signature_type"] = fakeS("ccla")
	cases := []struct {
		name     string
		item     map[string]interface{}
		snapshot *ItemSignature
	}{
		{"already approved", fakeEclaItem(1, "alice@acme.test"), nil},
		{"deliberately invalidated", readdDeliberateRow(1, "alice@acme.test"), nil},
		{"unsigned", unsigned, nil},
		{"gone", readdRemovedRow(2, "bob@acme.test", utils.EmailCriteria), readdCandidate("sig-001", "user-001")},
		{"replaced since the decision", readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria), moved},
		{"later manual note over the retained removal attribution", manualNote, nil},
		{"invalidation note present", withInvalidationNote, nil},
		{"not a user acknowledgment", companyReference, nil},
		{"not an individual or employee acknowledgment", cclaTyped, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newReaddHarness(t, []map[string]interface{}{tc.item})
			snapshot := tc.snapshot
			if snapshot == nil {
				var err error
				snapshot, err = h.repo.GetItemSignature(context.Background(), "sig-001")
				require.NoError(t, err)
			}
			before := h.rows()
			done, err := h.repo.RestoreRemovalInvalidatedEmployeeSignature(context.Background(), snapshot, "note")
			require.NoError(t, err)
			assert.False(t, done)
			for _, item := range before {
				h.assertUntouched(t, item)
			}
			h.table.mu.Lock()
			assert.Empty(t, h.table.updates, "nothing was written")
			h.table.mu.Unlock()
			reads := h.signatureReads()
			require.Len(t, reads, 1, "one consistent re-read of the acknowledgment")
			assert.True(t, reads[0].consistentRead)
		})
	}
}

func TestGetItemSignatureConsistent(t *testing.T) {
	h := newReaddHarness(t, []map[string]interface{}{readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria)})
	row, err := h.repo.GetItemSignatureConsistent(context.Background(), "sig-001")
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, "user-001", row.SignatureReferenceID)
	assert.Equal(t, readdRemovalNote(utils.EmailCriteria), row.Note)
	assert.True(t, row.InvalidatedByApprovalListRemoval())
	missing, err := h.repo.GetItemSignatureConsistent(context.Background(), "sig-404")
	require.NoError(t, err)
	assert.Nil(t, missing)
	reads := h.signatureReads()
	require.Len(t, reads, 2)
	for _, read := range reads {
		assert.True(t, read.consistentRead)
	}
	assert.Equal(t, []string{"sig-404"}, reads[1].signatureIDs)
}

func TestGetRemovalInvalidatedEmployeeSignatures(t *testing.T) {
	t.Run("classifies every row of the company under the CLA group, walking all index pages", func(t *testing.T) {
		table := &fakeSignaturesTable{items: readdMatrixItems(), invalidated: map[string]int{}, maxRawPage: 5, unprocessedOnce: 2}
		h := newReaddHarnessWithTable(t, table)
		candidates, err := h.repo.GetRemovalInvalidatedEmployeeSignatures(context.Background(), "company-1", "cla-group-1")
		require.NoError(t, err)
		var ids []string
		for _, candidate := range candidates {
			ids = append(ids, candidate.SignatureID)
			assert.True(t, candidate.SignatureSigned)
			assert.False(t, candidate.SignatureApproved)
			assert.NotEmpty(t, candidate.SignatureReferenceID, "the snapshot carries the full row, not the index projection")
		}
		assert.ElementsMatch(t, []string{"sig-001", "sig-002", "sig-009", "sig-010", "sig-012", "sig-013"}, ids)

		h.table.mu.Lock()
		defer h.table.mu.Unlock()
		require.Len(t, h.table.queries, 3, "11 index rows in pages of 5")
		for i, query := range h.table.queries {
			assert.Equal(t, fakeEmployeeIndex, query.indexName)
			assert.Empty(t, query.filter)
			assert.Zero(t, query.limit)
			assert.ElementsMatch(t, []string{"signature_user_ccla_company_id", "signature_project_id", "signature_id"}, query.attributeNames)
			assert.Equal(t, i > 0, query.startKey != "", "pages after the first continue from the last key")
		}
		require.Len(t, h.table.reads, 2, "one batch plus the retry of the unprocessed keys")
		assert.Len(t, h.table.reads[0].signatureIDs, 11)
		assert.Len(t, h.table.reads[1].signatureIDs, 2)
		for _, read := range h.table.reads {
			assert.Equal(t, readdSignatures, read.tableName)
			assert.True(t, read.consistentRead)
		}
	})
	t.Run("nothing under the CLA group", func(t *testing.T) {
		h := newReaddHarness(t, readdMatrixItems())
		candidates, err := h.repo.GetRemovalInvalidatedEmployeeSignatures(context.Background(), "company-1", "cla-group-9")
		require.NoError(t, err)
		assert.Empty(t, candidates)
		assert.Empty(t, h.signatureReads(), "no keys, no batch read")
	})
	t.Run("index query failure", func(t *testing.T) {
		table := &fakeSignaturesTable{items: readdMatrixItems(), invalidated: map[string]int{}, failIndex: fakeEmployeeIndex}
		h := newReaddHarnessWithTable(t, table)
		_, err := h.repo.GetRemovalInvalidatedEmployeeSignatures(context.Background(), "company-1", "cla-group-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "injected query failure")
	})
	t.Run("keys still unprocessed after every attempt", func(t *testing.T) {
		table := &fakeSignaturesTable{items: readdMatrixItems(), invalidated: map[string]int{}, unprocessedAlways: 1}
		h := newReaddHarnessWithTable(t, table)
		_, err := h.repo.GetRemovalInvalidatedEmployeeSignatures(context.Background(), "company-1", "cla-group-1")
		assert.EqualError(t, err, "1 signatures still unprocessed after 5 batch read attempts")
		assert.Len(t, h.signatureReads(), 5)
	})
}

// TestGetRemovalInvalidatedEmployeeSignaturesEvidenceAndShape: the classification is taken on the consistent base
// row - a later invalidation over the retained removal attribution, an invalidation note or a row that is not a
// user's individual/employee acknowledgment is not a candidate
func TestGetRemovalInvalidatedEmployeeSignaturesEvidenceAndShape(t *testing.T) {
	manualNote := readdRemovedRow(2, "bob@acme.test", utils.EmailCriteria)
	manualNote["note"] = fakeS("Signature invalidated (approved set to false) by pcc-admin for user-002")
	withInvalidationNote := readdRemovedRow(3, "carol@acme.test", utils.EmailCriteria)
	withInvalidationNote["invalidation_note"] = fakeS("manual")
	companyReference := readdRemovedRow(4, "dave@acme.test", utils.EmailCriteria)
	companyReference["signature_reference_type"] = fakeS("company")
	cclaTyped := readdRemovedRow(5, "erin@acme.test", utils.EmailCriteria)
	cclaTyped["signature_type"] = fakeS("ccla")
	eclaTyped := readdRemovedRow(6, "frank@acme.test", utils.EmailCriteria)
	eclaTyped["signature_type"] = fakeS("ecla")
	blankNote := readdRemovedRow(7, "grace@acme.test", utils.EmailCriteria)
	blankNote["note"] = fakeS("   ")
	noNote := readdRemovedRow(8, "heidi@acme.test", utils.EmailCriteria)
	delete(noNote, "note")
	h := newReaddHarness(t, []map[string]interface{}{readdCCLA{}.item(), readdRemovedRow(1, "alice@acme.test", utils.EmailCriteria),
		manualNote, withInvalidationNote, companyReference, cclaTyped, eclaTyped, blankNote, noNote})
	candidates, err := h.repo.GetRemovalInvalidatedEmployeeSignatures(context.Background(), "company-1", "cla-group-1")
	require.NoError(t, err)
	var ids []string
	for _, candidate := range candidates {
		ids = append(ids, candidate.SignatureID)
	}
	assert.ElementsMatch(t, []string{"sig-001", "sig-006", "sig-007", "sig-008"}, ids)
}

func TestRestorableEmployeeAcknowledgment(t *testing.T) {
	removalOnly := func() *ItemSignature {
		return &ItemSignature{SignatureID: "sig-001", SignatureReferenceID: "user-001", SignatureReferenceType: utils.SignatureReferenceTypeUser,
			SignatureType: utils.SignatureTypeCLA, SignatureUserCompanyID: "company-1", SignatureProjectID: "cla-group-1", SignatureSigned: true,
			InvalidationReason: ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")", Note: readdRemovalNote(utils.EmailCriteria)}
	}
	cases := []struct {
		name   string
		change func(*ItemSignature)
		want   bool
	}{
		{"removal-only employee acknowledgment", func(*ItemSignature) {}, true},
		{"ecla typed", func(s *ItemSignature) { s.SignatureType = utils.ClaTypeECLA }, true},
		{"legacy note-only attribution", func(s *ItemSignature) { s.InvalidationReason = "" }, true},
		{"attribution without a note", func(s *ItemSignature) { s.Note = "" }, true},
		{"other company", func(s *ItemSignature) { s.SignatureUserCompanyID = "company-2" }, false},
		{"other CLA group", func(s *ItemSignature) { s.SignatureProjectID = "cla-group-2" }, false},
		{"company reference", func(s *ItemSignature) { s.SignatureReferenceType = utils.SignatureReferenceTypeCompany }, false},
		{"ccla typed", func(s *ItemSignature) { s.SignatureType = utils.SignatureTypeCCLA }, false},
		{"unsigned", func(s *ItemSignature) { s.SignatureSigned = false }, false},
		{"not invalidated by a removal", func(s *ItemSignature) { s.InvalidationReason = "left the company" }, false},
		{"later manual note", func(s *ItemSignature) {
			s.Note = "Signature invalidated (approved set to false) by pcc-admin for user-001"
		}, false},
		{"invalidation note", func(s *ItemSignature) { s.InvalidationNote = "manual" }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := removalOnly()
			tc.change(row)
			assert.Equal(t, tc.want, restorableEmployeeAcknowledgment(row, "company-1", "cla-group-1"))
		})
	}
	assert.False(t, restorableEmployeeAcknowledgment(nil, "company-1", "cla-group-1"))
}

func TestInvalidatedOnlyByApprovalListRemoval(t *testing.T) {
	cases := []struct {
		name          string
		row           ItemSignature
		byRemoval     bool
		onlyByRemoval bool
	}{
		{"attributed removal with its note", ItemSignature{InvalidationReason: ApprovalListRemovalReasonPrefix + utils.GitHubOrgCriteria + ")",
			Note: readdRemovalNote(utils.GitHubOrgCriteria)}, true, true},
		{"attributed removal, note appended by a restore", ItemSignature{InvalidationReason: ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")",
			Note: readdRemovalNote(utils.EmailCriteria) + " Re-enabled employee acknowledgment previously disabled by approval list removal."}, true, false},
		{"attributed removal without a note", ItemSignature{InvalidationReason: ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")"}, true, true},
		{"legacy note only", ItemSignature{Note: readdRemovalNote(utils.EmailDomainCriteria)}, true, true},
		{"legacy note with extra whitespace", ItemSignature{Note: "  Signature invalidated (approved set to false) by  manager-lf   due to " + utils.GitlabUsernameCriteria + "  removal "}, true, true},
		{"retained attribution, later manual note", ItemSignature{InvalidationReason: ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")",
			Note: "Signature invalidated (approved set to false) by pcc-admin for user-001"}, true, false},
		{"retained attribution, CLA group deletion note", ItemSignature{InvalidationReason: ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")",
			Note: "Signature invalidated (approved set to false) by pcc-admin due to CLA Group/Project: cla-group-1 deletion"}, true, false},
		{"retained attribution with an invalidation note", ItemSignature{InvalidationReason: ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")",
			Note: readdRemovalNote(utils.EmailCriteria), InvalidationNote: "manual"}, true, false},
		{"deliberate", ItemSignature{InvalidationReason: "left the company", Note: readdDeliberateNote, InvalidationNote: "manual"}, false, false},
		{"legacy deliberate note", ItemSignature{Note: readdDeliberateNote}, false, false},
		{"nothing", ItemSignature{}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := tc.row
			assert.Equal(t, tc.byRemoval, row.InvalidatedByApprovalListRemoval(), "InvalidatedByApprovalListRemoval")
			assert.Equal(t, tc.onlyByRemoval, row.InvalidatedOnlyByApprovalListRemoval(), "InvalidatedOnlyByApprovalListRemoval")
		})
	}
}
