// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_organizations

import (
	"context"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/v2/common"
	"github.com/stretchr/testify/assert"
)

// embeds the interface so only the method under test needs an implementation
type stateLookupRepo struct {
	RepositoryInterface
	org *common.GitLabOrganization
}

func (r *stateLookupRepo) GetGitLabOrganization(_ context.Context, _ string) (*common.GitLabOrganization, error) {
	return r.org, nil
}

// the repository returns (nil, nil) for an unknown organization: the OAuth callback must get an error, not a nil
// dereference (lfx-self-serve-ops#727)
func TestGetGitLabOrganizationByState(t *testing.T) {
	t.Run("unknown organization", func(t *testing.T) {
		svc := &Service{repo: &stateLookupRepo{}}
		org, err := svc.GetGitLabOrganizationByState(context.Background(), "missing", "state-1")
		assert.Nil(t, org)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "not found")
		}
	})
	t.Run("state mismatch", func(t *testing.T) {
		svc := &Service{repo: &stateLookupRepo{org: &common.GitLabOrganization{OrganizationID: "o1", AuthState: "state-2"}}}
		org, err := svc.GetGitLabOrganizationByState(context.Background(), "o1", "state-1")
		assert.Nil(t, org)
		assert.EqualError(t, err, "auth state doesn't match")
	})
	t.Run("matching state", func(t *testing.T) {
		svc := &Service{repo: &stateLookupRepo{org: &common.GitLabOrganization{OrganizationID: "o1", AuthState: "state-1"}}}
		org, err := svc.GetGitLabOrganizationByState(context.Background(), "o1", "state-1")
		assert.NoError(t, err)
		if assert.NotNil(t, org) {
			assert.Equal(t, "o1", org.OrganizationID)
		}
	})
}
