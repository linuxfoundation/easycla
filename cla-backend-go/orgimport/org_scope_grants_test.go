// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const thirdSFID = "0014100000ThirdOrg"

func TestRewriteMovesOrganizationScopeGrants(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.oneOrg = true
	fx.platform.addGrant(lfID, "frank", "role-company-admin", "")
	fx.platform.addGrant(lfID, "frank", "role-contact", "")
	fx.platform.addGrant(lfID, "heidi", "role-contact", "")
	fx.platform.addGrant(thirdSFID, "heidi", "role-contact", "")
	frankAdmin, heidiContact := grantIDOf(t, fx, lfID, "frank", "company-admin"), grantIDOf(t, fx, lfID, "heidi", "contact")
	heidiElsewhere := grantIDOf(t, fx, thirdSFID, "heidi", "contact")
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)

	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Zero(t, sum.Failed)
	assert.Equal(t, []string{
		"alice:cla-manager:" + targetSFID,
		"bob:cla-manager:proj-1|" + targetSFID,
		"carol:cla-signatory:" + targetSFID,
		"frank:contact:" + targetSFID,
	}, targetGrants(t, fx), "an organization scope grant reaches the Account once the user holds none on the old id")
	assert.Empty(t, mustListOrgGrants(t, fx, lfID))
	assert.Equal(t, heidiElsewhere, grantIDOf(t, fx, thirdSFID, "heidi", "contact"), "the grant in another organization is not touched")
	assert.ElementsMatch(t, [][]string{
		{lfID, targetSFID, "frank", "company-admin", objectTypeOrganization, lfID},
		{lfID, targetSFID, "heidi", "contact", objectTypeOrganization, lfID},
	}, droppedReport(t, opts.OutDir)[1:], "a user who holds an organization role in another organization keeps that one")
	assert.Contains(t, fx.out.String(), "grant deleted without a copy "+lfID+" -> "+targetSFID+": company-admin grant "+frankAdmin+" on "+lfID)
	assert.Contains(t, fx.out.String(), "grant deleted without a copy "+lfID+" -> "+targetSFID+": contact grant "+heidiContact+" on "+lfID)
	journal, err := os.ReadFile(filepath.Clean(statePath))
	require.NoError(t, err)
	for _, user := range []string{"frank", "heidi", "carol"} {
		assert.NotContains(t, string(journal), user)
		assert.NotContains(t, fx.out.String(), user)
	}
}

func TestRewriteRestoresAnOrganizationScopeGrantWhenItsMoveFails(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	const user = "frank"
	fx.platform.oneOrg = true
	fx.platform.addGrant(lfID, user, "role-contact", "")
	failures := 0
	fx.platform.createHook = func(username, orgID, objectType string) error {
		if username != user || orgID != targetSFID {
			return nil
		}
		for _, g := range fx.platform.grants[lfID] {
			if g.Username == user {
				return nil
			}
		}
		failures++
		return errors.New("org-service down")
	}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)

	require.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.Equal(t, 1, failures)
	var restored []string
	for _, g := range mustListOrgGrants(t, fx, lfID) {
		restored = append(restored, g.Username+":"+g.RoleName)
	}
	assert.Equal(t, []string{user + ":contact"}, restored, "the grant is back on the old id, so the next run finds it")
	assert.NotContains(t, targetGrants(t, fx), user+":contact:"+targetSFID)

	fx.platform.createHook = nil
	plan, err = BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	sum, err = Execute(context.Background(), fx.deps(), opts, plan)

	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Contains(t, targetGrants(t, fx), user+":contact:"+targetSFID)
	assert.Empty(t, mustListOrgGrants(t, fx, lfID))
}
