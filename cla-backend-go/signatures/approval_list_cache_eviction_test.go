// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/github"
)

type failingApprovalListRepo struct {
	SignatureRepository
	err error
}

func (f *failingApprovalListRepo) UpdateApprovalList(_ context.Context, _ *models.User, _ *models.ClaGroup, _ string, _ *models.ApprovalList, _ *events.LogEventArgs) (*models.Signature, error) {
	return nil, f.err
}

// a refused removal may already have invalidated acknowledgments, so the project's cached PR-check decisions
// are evicted even when the repository reports the failure
func TestUpdateApprovalListEvictsTheProjectCacheOnFailure(t *testing.T) {
	h := newReaddHarness(t, []map[string]interface{}{readdCCLA{emails: []string{"keep@acme.test"}}.item()})
	h.svc.repo = &failingApprovalListRepo{SignatureRepository: h.repo, err: errors.New("GitHub organization removal not committed")}
	key := github.ProjectUserKey("cla-group-1", "user-001", "alice", "alice@acme.test")
	github.ModelProjectUserCache.Set(key, &models.User{UserID: "user-001"}, true, true)
	t.Cleanup(func() { github.ModelProjectUserCache.Delete(key) })

	_, err := h.call(&models.ApprovalList{RemoveGithubOrgApprovalList: []string{"old-org"}})
	require.ErrorContains(t, err, "not committed")
	_, _, _, cached := github.ModelProjectUserCache.Get(key)
	assert.False(t, cached, "the stale authorized decision is evicted although the update failed")
}
