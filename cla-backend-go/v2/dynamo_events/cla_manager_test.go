// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// an ACS 409 (the signer already holds cla-manager for that project|organization) is not a failure: every project is
// still attempted and the sign event goes on to the cla-manager-designee cleanup
func TestAssignCLAManagerTreatsAnExistingRoleAsAssigned(t *testing.T) {
	const projA, projB = "proj-a", "proj-b"
	projects := []*projects_cla_groups.ProjectClaGroup{{ProjectSFID: projA}, {ProjectSFID: projB}}
	withSeams := func(t *testing.T, create func(projectSFID string) error) *[]string {
		previousLookup, previousCreator := claManagerRoleIDLookup, claManagerRoleScopeCreator
		t.Cleanup(func() { claManagerRoleIDLookup, claManagerRoleScopeCreator = previousLookup, previousCreator })
		claManagerRoleIDLookup = func() (string, error) { return "role-cla-manager", nil }
		var mu sync.Mutex
		attempted := &[]string{}
		claManagerRoleScopeCreator = func(_ context.Context, email, projectSFID, companySFID, roleID string) error {
			mu.Lock()
			*attempted = append(*attempted, projectSFID)
			mu.Unlock()
			assert.Equal(t, "jane@example.invalid", email)
			assert.Equal(t, "company-sfid", companySFID)
			assert.Equal(t, "role-cla-manager", roleID)
			return create(projectSFID)
		}
		return attempted
	}

	t.Run("already assigned on one project", func(t *testing.T) {
		attempted := withSeams(t, func(projectSFID string) error {
			if projectSFID == projA {
				return &organizations.CreateOrgUsrRoleScopesConflict{}
			}
			return nil
		})
		require.NoError(t, service{}.assignCLAManager(context.Background(), "jane@example.invalid", "jane", "company-sfid", projects))
		sort.Strings(*attempted)
		assert.Equal(t, []string{projA, projB}, *attempted, "both projects are attempted")
	})
	t.Run("failures on every project are collected without a race", func(t *testing.T) {
		boomA, boomB := errors.New("proj-a failed"), errors.New("proj-b failed")
		attempted := withSeams(t, func(projectSFID string) error {
			if projectSFID == projA {
				return boomA
			}
			return boomB
		})
		err := service{}.assignCLAManager(context.Background(), "jane@example.invalid", "jane", "company-sfid", projects)
		assert.True(t, errors.Is(err, boomA) || errors.Is(err, boomB), "one of the failures is returned: %v", err)
		assert.Len(t, *attempted, 2)
	})
	t.Run("any other failure is returned", func(t *testing.T) {
		boom := errors.New("organization service down")
		attempted := withSeams(t, func(projectSFID string) error {
			if projectSFID == projB {
				return boom
			}
			return nil
		})
		assert.True(t, errors.Is(service{}.assignCLAManager(context.Background(), "jane@example.invalid", "jane", "company-sfid", projects), boom))
		assert.Len(t, *attempted, 2)
	})
}
