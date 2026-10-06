// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/v2/approvals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeApprovalRepo struct {
	approvals.IRepository
	existing []approvals.ApprovalItem
	updated  []approvals.ApprovalItem
	added    []approvals.ApprovalItem
}

func (f *fakeApprovalRepo) SearchApprovalList(criteria, approvalListName, claGroupID, companyID, signatureID string) ([]approvals.ApprovalItem, error) {
	return f.existing, nil
}

func (f *fakeApprovalRepo) UpdateApprovalItem(approvalItem approvals.ApprovalItem) error {
	f.updated = append(f.updated, approvalItem)
	return nil
}

func (f *fakeApprovalRepo) AddApprovalList(approvalItem approvals.ApprovalItem) error {
	f.added = append(f.added, approvalItem)
	return nil
}

func TestUpdateApprovalTableReAddRefreshesDateAdded(t *testing.T) {
	fake := &fakeApprovalRepo{existing: []approvals.ApprovalItem{{
		ApprovalID:       "approval-1",
		SignatureID:      "sig-1",
		ApprovalName:     "x@example.com",
		ApprovalCriteria: "email",
		DateAdded:        "2020-01-01T00:00:00.000000+0000",
		DateRemoved:      "2021-01-01T00:00:00.000000+0000",
		Active:           false,
	}}}
	repo := &repository{approvalRepo: fake}

	repo.updateApprovalTable(context.Background(), []string{"x@example.com"}, "email", "sig-1", "proj-1", "comp-1", "Acme", true)

	require.Len(t, fake.updated, 1)
	assert.Empty(t, fake.added)
	item := fake.updated[0]
	assert.True(t, item.Active)
	assert.NotEmpty(t, item.DateAdded)
	assert.NotEqual(t, "2020-01-01T00:00:00.000000+0000", item.DateAdded, "re-adding an entry must refresh date_added")
	assert.Equal(t, item.DateModified, item.DateAdded)
}

func TestUpdateApprovalTableRemoveExisting(t *testing.T) {
	fake := &fakeApprovalRepo{existing: []approvals.ApprovalItem{{
		ApprovalID:       "approval-1",
		SignatureID:      "sig-1",
		ApprovalName:     "x@example.com",
		ApprovalCriteria: "email",
		DateAdded:        "2020-01-01T00:00:00.000000+0000",
		Active:           true,
	}}}
	repo := &repository{approvalRepo: fake}

	repo.updateApprovalTable(context.Background(), []string{"x@example.com"}, "email", "sig-1", "proj-1", "comp-1", "Acme", false)

	require.Len(t, fake.updated, 1)
	item := fake.updated[0]
	assert.False(t, item.Active)
	assert.NotEmpty(t, item.DateRemoved)
	assert.Equal(t, "2020-01-01T00:00:00.000000+0000", item.DateAdded, "removal must not change date_added")
}

func TestUpdateApprovalTableAddNewEntry(t *testing.T) {
	fake := &fakeApprovalRepo{}
	repo := &repository{approvalRepo: fake}

	repo.updateApprovalTable(context.Background(), []string{"x@example.com"}, "email", "sig-1", "proj-1", "comp-1", "Acme", true)

	require.Len(t, fake.added, 1)
	assert.Empty(t, fake.updated)
	item := fake.added[0]
	assert.NotEmpty(t, item.ApprovalID)
	assert.Equal(t, "sig-1", item.SignatureID)
	assert.Equal(t, "x@example.com", item.ApprovalName)
	assert.Equal(t, "email", item.ApprovalCriteria)
	assert.Equal(t, "proj-1", item.ProjectID)
	assert.Equal(t, "comp-1", item.CompanyID)
	assert.Equal(t, "Acme", item.ApprovalCompanyName)
	assert.True(t, item.Active)
	assert.NotEmpty(t, item.DateAdded)
	assert.Equal(t, "Auto-Added", item.Note)
}
