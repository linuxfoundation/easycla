// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var droppedHeader = []string{"old_id", "new_id", "username", "role", "object_type", "object_id"}

func targetGrants(t *testing.T, fx *fixture) []string {
	t.Helper()
	var out []string
	for _, g := range mustListOrgGrants(t, fx, targetSFID) {
		out = append(out, g.Username+":"+g.RoleName+":"+g.ObjectID)
	}
	sort.Strings(out)
	return out
}

func TestRewriteOntoExistingAccountCopiesOnlyEasyCLARoles(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.addGrant(lfID, "dan", "role-cla-manager-designee", "proj-1")
	fx.platform.addGrant(lfID, "erin", "role-contributor", "proj-1")
	fx.platform.addGrant(lfID, "frank", "role-contact", "")
	fx.platform.addGrant(lfID, "alice", "role-company-admin", "")
	fx.platform.addGrant(lfID, "grace", "role-company-admin", "")
	fx.platform.addGrant(lfID, "heidi", "role-viewer", "")
	fx.platform.addGrant(targetSFID, "grace", "role-company-admin", "")
	lists := 0
	fx.platform.listHook = func(orgID string) {
		if orgID == lfID {
			if lists++; lists == 2 {
				fx.platform.addGrantLocked(lfID, "ivan", "role-viewer", "")
			}
		}
	}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	var printed bytes.Buffer
	plan.Print(&printed)
	assert.Contains(t, printed.String(), "(matched): wait org-service; copy the EasyCLA role grants;")
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)

	assert.Equal(t, []string{
		"alice:cla-manager:" + targetSFID,
		"bob:cla-manager:proj-1|" + targetSFID,
		"carol:cla-signatory:" + targetSFID,
		"dan:cla-manager-designee:proj-1|" + targetSFID,
		"erin:contributor:proj-1|" + targetSFID,
		"frank:contact:" + targetSFID,
		"grace:company-admin:" + targetSFID,
	}, targetGrants(t, fx), "only the EasyCLA roles are copied; grace keeps the grant she already had on the Account")
	assert.Empty(t, mustListOrgGrants(t, fx, lfID), "old grants are deleted, copied or not")
	dropped := readCSV(t, filepath.Join(opts.OutDir, "dropped_grants.csv"))
	assert.Equal(t, droppedHeader, dropped[0])
	assert.ElementsMatch(t, [][]string{
		{lfID, targetSFID, "alice", "company-admin", objectTypeOrganization, lfID},
		{lfID, targetSFID, "heidi", "viewer", objectTypeOrganization, lfID},
		{lfID, targetSFID, "ivan", "viewer", objectTypeOrganization, lfID},
	}, dropped[1:], "one row per grant deleted without a copy, the late one included")
	assert.Contains(t, fx.out.String(), "grants "+lfID+" -> "+targetSFID+": 2 existing, 5 created, 2 not copied")
	assert.Contains(t, fx.out.String(), "grant deleted without a copy "+lfID+" -> "+targetSFID+": company-admin alice on "+lfID)
	journal, err := os.ReadFile(filepath.Clean(statePath))
	require.NoError(t, err)
	assert.NotContains(t, string(journal), "all_grants")
}

func TestRewriteOntoNewAccountCopiesEveryGrant(t *testing.T) {
	fx, _, statePath := rewriteFixture(t)
	fx.platform.addGrant(lfID, "grace", "role-company-admin", "")
	fx.platform.addGrant(lfID, "heidi", "role-viewer", "")
	mapping := writeMapping(t, t.TempDir(), lfID+","+targetSFID+",created,true")
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	var printed bytes.Buffer
	plan.Print(&printed)
	assert.Contains(t, printed.String(), "(created): wait org-service; copy ACS grants;")
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)

	assert.Equal(t, []string{
		"alice:cla-manager:" + targetSFID,
		"bob:cla-manager:proj-1|" + targetSFID,
		"carol:cla-signatory:" + targetSFID,
		"grace:company-admin:" + targetSFID,
		"heidi:viewer:" + targetSFID,
	}, targetGrants(t, fx))
	assert.Empty(t, mustListOrgGrants(t, fx, lfID))
	assert.Equal(t, [][]string{droppedHeader}, readCSV(t, filepath.Join(opts.OutDir, "dropped_grants.csv")))
	journal, err := os.ReadFile(filepath.Clean(statePath))
	require.NoError(t, err)
	assert.Contains(t, string(journal), `"all_grants":true,"step":"done"`)
}

func TestCollapseCopiesOnlyEasyCLARoles(t *testing.T) {
	fx, dir := collisionFixture(t)
	fx.platform.addGrant("lf-a", "alice", "role-cla-manager", "")
	fx.platform.addGrant("lf-a", "alice", "role-company-admin", "")
	fx.platform.addGrant("lf-b", "bob", "role-cla-manager", "")
	fx.platform.addGrant("lf-b", "bob", "role-company-admin", "")
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",created,true", "lf-b,"+targetSFID+",created,true")
	decisions := writeDecisions(t, dir, "collapse,lf-a;lf-b,"+targetSFID+",michal,one legal entity")
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, Decisions: decisions, State: filepath.Join(dir, "state.jsonl"), Routes: []Route{RouteRewrite}, OutDir: filepath.Join(dir, "out")}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 2)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 2, sum.Rewritten)

	assert.Equal(t, []string{"alice:cla-manager:" + targetSFID, "bob:cla-manager:" + targetSFID}, targetGrants(t, fx),
		"a collapse lands on a shared Account: neither creator becomes its admin")
	assert.Empty(t, mustListOrgGrants(t, fx, "lf-a"))
	assert.Empty(t, mustListOrgGrants(t, fx, "lf-b"))
	assert.Equal(t, [][]string{
		droppedHeader,
		{"lf-a", targetSFID, "alice", "company-admin", objectTypeOrganization, "lf-a"},
		{"lf-b", targetSFID, "bob", "company-admin", objectTypeOrganization, "lf-b"},
	}, readCSV(t, filepath.Join(opts.OutDir, "dropped_grants.csv")))
}

func TestReplayKeepsTheGrantPolicyOfTheJournal(t *testing.T) {
	for _, tc := range []struct {
		name, journal string
		copied        bool
	}{
		{"new Account", `"all_grants":true,`, true},
		{"existing Account", ``, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx, mapping, statePath := rewriteFixture(t)
			// a run that died after rewriting the rows, before the cleanup
			fx.companies.rows["c-parent"].CompanyExternalID, fx.companies.rows["c-parent"].PreviousCompanyExternalID = targetSFID, lfID
			fx.companies.rows["c-sub"].CompanyExternalID, fx.companies.rows["c-sub"].PreviousCompanyExternalID = targetSFID, lfID
			fx.platform.addGrant(lfID, "grace", "role-company-admin", "")
			require.NoError(t, os.WriteFile(statePath, []byte(`{"old_id":"`+lfID+`","new_id":"`+targetSFID+`","company_ids":["c-parent","c-sub"],`+tc.journal+
				`"step":"rewrite_rows","ts":"2026-09-29T11:00:00Z","status":"ok"}`+"\n"), 0o600))
			opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
			plan, err := BuildPlan(context.Background(), fx.deps(), opts)
			require.NoError(t, err)
			require.Len(t, plan.Rewrite, 1)
			require.True(t, plan.Rewrite[0].Replayed)
			sum, err := Execute(context.Background(), fx.deps(), opts, plan)
			require.NoError(t, err)
			assert.Equal(t, 1, sum.Rewritten)

			assert.Empty(t, mustListOrgGrants(t, fx, lfID))
			journal, err := os.ReadFile(filepath.Clean(statePath))
			require.NoError(t, err)
			if tc.copied {
				assert.Contains(t, targetGrants(t, fx), "grace:company-admin:"+targetSFID)
				assert.Contains(t, string(journal), `"all_grants":true,"step":"done"`)
				assert.Equal(t, [][]string{droppedHeader}, readCSV(t, filepath.Join(opts.OutDir, "dropped_grants.csv")))
				return
			}
			assert.NotContains(t, targetGrants(t, fx), "grace:company-admin:"+targetSFID)
			assert.NotContains(t, string(journal), "all_grants")
			assert.Equal(t, [][]string{droppedHeader, {lfID, targetSFID, "grace", "company-admin", objectTypeOrganization, lfID}}, readCSV(t, filepath.Join(opts.OutDir, "dropped_grants.csv")))
		})
	}
}

func TestDryRunWritesNoDroppedGrantsFile(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.addGrant(lfID, "grace", "role-company-admin", "")
	opts := Options{Stage: "dev", Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	_, err := runIngest(t, fx, opts)
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(opts.OutDir, "dropped_grants.csv"))
	assert.True(t, os.IsNotExist(err), "a dry run does not list the grants: an empty file would read as nothing dropped")
	_, err = os.Stat(filepath.Join(opts.OutDir, "plan.csv"))
	assert.NoError(t, err)
	assert.Len(t, mustListOrgGrants(t, fx, lfID), 4)
}

func TestGroupStoppedBeforeCleanupReportsNoDroppedGrant(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.addGrant(lfID, "grace", "role-company-admin", "")
	fx.companies.rows["c-sub"].CompanyExternalID = somebodySFID
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	fx.companies.rows["c-parent"].CompanyExternalID = somebodySFID
	_, err = Execute(context.Background(), fx.deps(), opts, plan)
	require.Error(t, err)
	assert.Contains(t, fx.out.String(), "1 not copied")
	assert.Len(t, mustListOrgGrants(t, fx, lfID), 4, "the group stopped at the row rewrite: nothing is deleted")
	assert.Equal(t, [][]string{droppedHeader}, readCSV(t, filepath.Join(opts.OutDir, "dropped_grants.csv")))
}
