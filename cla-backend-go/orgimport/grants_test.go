// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	acs_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
)

var droppedHeader = []string{"old_id", "new_id", "username", "role", "object_type", "object_id"}

// droppedReport reads dropped_grants.csv without its last column, grant_id, which every data row must carry.
func droppedReport(t *testing.T, dir string) [][]string {
	t.Helper()
	rows := readCSV(t, filepath.Join(dir, "dropped_grants.csv"))
	require.Equal(t, "grant_id", rows[0][len(rows[0])-1])
	out := make([][]string, 0, len(rows))
	for i, row := range rows {
		if i > 0 {
			require.NotEmpty(t, row[len(row)-1], row)
		}
		out = append(out, row[:len(row)-1])
	}
	return out
}

func grantIDOf(t *testing.T, fx *fixture, org, user, role string) string {
	t.Helper()
	for _, g := range fx.platform.grants[org] {
		if g.Username == user && g.RoleName == role {
			return g.GrantID
		}
	}
	t.Fatalf("no %s grant of %s on %s", role, user, org)
	return ""
}

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
	aliceAdmin, heidiViewer := grantIDOf(t, fx, lfID, "alice", "company-admin"), grantIDOf(t, fx, lfID, "heidi", "viewer")
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
	dropped := droppedReport(t, opts.OutDir)
	assert.Equal(t, droppedHeader, dropped[0])
	assert.ElementsMatch(t, [][]string{
		{lfID, targetSFID, "alice", "company-admin", objectTypeOrganization, lfID},
		{lfID, targetSFID, "heidi", "viewer", objectTypeOrganization, lfID},
		{lfID, targetSFID, "ivan", "viewer", objectTypeOrganization, lfID},
	}, dropped[1:], "one row per grant deleted without a copy, the late one included")
	assert.Contains(t, fx.out.String(), "grants "+lfID+" -> "+targetSFID+": 2 existing, 5 created, 2 not copied")
	assert.Contains(t, fx.out.String(), "grant deleted without a copy "+lfID+" -> "+targetSFID+": company-admin grant "+aliceAdmin+" on "+lfID)
	journal, err := os.ReadFile(filepath.Clean(statePath))
	require.NoError(t, err)
	assert.NotContains(t, string(journal), "all_grants")
	assert.Contains(t, string(journal), `"grant_id":"`+heidiViewer+`"`, "the journal keeps the list by grant id")
	for _, user := range []string{"alice", "heidi", "ivan"} {
		assert.NotContains(t, string(journal), user, "no username in the journal: the sweep workflow publishes it")
		assert.NotContains(t, fx.out.String(), user, "no username in the log")
	}
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
	assert.Equal(t, [][]string{droppedHeader}, droppedReport(t, opts.OutDir))
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
	}, droppedReport(t, opts.OutDir))
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
				assert.Equal(t, [][]string{droppedHeader}, droppedReport(t, opts.OutDir))
				return
			}
			assert.NotContains(t, targetGrants(t, fx), "grace:company-admin:"+targetSFID)
			assert.NotContains(t, string(journal), "all_grants")
			assert.Equal(t, [][]string{droppedHeader, {lfID, targetSFID, "grace", "company-admin", objectTypeOrganization, lfID}}, droppedReport(t, opts.OutDir))
		})
	}
}

func TestDryRunWritesNoDroppedGrantsFile(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.addGrant(lfID, "grace", "role-company-admin", "")
	opts := Options{Stage: "dev", Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	require.NoError(t, os.WriteFile(filepath.Join(opts.OutDir, "dropped_grants.csv"), []byte("left by an earlier apply\n"), 0o600))
	_, err := runIngest(t, fx, opts)
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(opts.OutDir, "dropped_grants.csv"))
	assert.True(t, os.IsNotExist(err), "a dry run lists no grants and keeps no file of an earlier run: an empty or stale file would read as its own result")
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
	assert.Equal(t, [][]string{droppedHeader}, droppedReport(t, opts.OutDir))
}

func TestReplayOntoAnAccountThatBecameSharedCopiesOnlyEasyCLARoles(t *testing.T) {
	fx, dir := collisionFixture(t)
	fx.platform.addGrant("lf-a", "alice", "role-cla-manager", "")
	fx.platform.addGrant("lf-a", "alice", "role-company-admin", "")
	fx.platform.addGrant("lf-b", "bob", "role-cla-manager", "")
	statePath := filepath.Join(dir, "state.jsonl")
	// lf-a was planned alone on a new Account and stopped while waiting for org-service; lf-b was mapped onto it since
	require.NoError(t, os.WriteFile(statePath, []byte(`{"old_id":"lf-a","new_id":"`+targetSFID+`","company_ids":["c-a"],"all_grants":true,`+
		`"step":"wait","ts":"2026-09-29T11:00:00Z","status":"failed","err":"org-service does not serve it yet"}`+"\n"), 0o600))
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",created,true", "lf-b,"+targetSFID+",created,true")
	decisions := writeDecisions(t, dir, "collapse,lf-a;lf-b,"+targetSFID+",michal,one legal entity")
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, Decisions: decisions, State: statePath, Routes: []Route{RouteRewrite}, OutDir: filepath.Join(dir, "out")}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 2)
	require.True(t, plan.Rewrite[0].Replayed)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 2, sum.Rewritten)

	assert.Equal(t, []string{"alice:cla-manager:" + targetSFID, "bob:cla-manager:" + targetSFID}, targetGrants(t, fx),
		"the journal says all grants, but the Account is shared now")
	assert.Equal(t, [][]string{droppedHeader, {"lf-a", targetSFID, "alice", "company-admin", objectTypeOrganization, "lf-a"}},
		droppedReport(t, opts.OutDir))
}

func TestDroppedGrantsReportIsRebuiltFromTheJournal(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	// an earlier run finished another group, deleted this group's old grants and died before its final report
	fx.company("c-two", "Two", "", twoSFID, "cg-1")
	fx.platform.sfAccounts[twoSFID] = true
	fx.companies.rows["c-parent"].CompanyExternalID, fx.companies.rows["c-parent"].PreviousCompanyExternalID = targetSFID, lfID
	fx.companies.rows["c-sub"].CompanyExternalID, fx.companies.rows["c-sub"].PreviousCompanyExternalID = targetSFID, lfID
	fx.platform.grants[lfID] = nil
	grant := func(id, role, org string) string {
		return `{"username":"","role_name":"` + role + `","role_id":"role-` + role + `","grant_id":"` + id + `","object_type_name":"organization","object_id":"` + org + `"}`
	}
	require.NoError(t, os.WriteFile(statePath, []byte(
		`{"old_id":"`+twoLfID+`","new_id":"`+twoSFID+`","company_ids":["c-two"],"dropped":[`+grant("g-92", "viewer", twoLfID)+`],"step":"done","ts":"2026-09-29T11:00:00Z","status":"ok"}`+"\n"+
			`{"old_id":"`+lfID+`","new_id":"`+targetSFID+`","company_ids":["c-parent","c-sub"],"dropped":[`+grant("g-91", "company-admin", lfID)+`],"step":"delete_old_grants","ts":"2026-09-29T11:00:01Z","status":"ok"}`+"\n"), 0o600))
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	sum, err := runIngest(t, fx, opts)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)

	assert.Equal(t, [][]string{
		append(append([]string{}, droppedHeader...), "grant_id"),
		{lfID, targetSFID, "", "company-admin", objectTypeOrganization, lfID, "g-91"},
		{twoLfID, twoSFID, "", "viewer", objectTypeOrganization, twoLfID, "g-92"},
	}, readCSV(t, filepath.Join(opts.OutDir, "dropped_grants.csv")), "the list survives the crash by grant id: the journal keeps no usernames")
	journal, err := os.ReadFile(filepath.Clean(statePath))
	require.NoError(t, err)
	assert.Contains(t, string(journal), `"grant_id":"g-91","scope_id":"","object_type_name":"organization","object_id":"`+lfID+`"}],"step":"done"`)
}

func TestCollapseDecisionAppliesToTheFirstGroupOnTheAccount(t *testing.T) {
	fx, dir := collisionFixture(t)
	fx.platform.addGrant("lf-a", "alice", "role-cla-manager", "")
	fx.platform.addGrant("lf-a", "alice", "role-company-admin", "")
	// lf-b belongs to the same reviewed collapse but has no mapping row yet: lf-a is alone on the Account for now
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",created,true")
	decisions := writeDecisions(t, dir, "collapse,lf-a;lf-b,"+targetSFID+",michal,one legal entity")
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, Decisions: decisions, State: filepath.Join(dir, "state.jsonl"), Routes: []Route{RouteRewrite}, OutDir: filepath.Join(dir, "out")}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	var printed bytes.Buffer
	plan.Print(&printed)
	assert.Contains(t, printed.String(), "(created): wait org-service; copy the EasyCLA role grants;")
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)

	assert.Equal(t, []string{"alice:cla-manager:" + targetSFID}, targetGrants(t, fx))
	assert.Equal(t, [][]string{droppedHeader, {"lf-a", targetSFID, "alice", "company-admin", objectTypeOrganization, "lf-a"}},
		droppedReport(t, opts.OutDir))
}

func TestGrantWithoutRoleNameIsNeverDeletedUncopied(t *testing.T) {
	for _, tc := range []struct {
		action string
		stops  bool
	}{{"matched", true}, {"created", false}} {
		t.Run(tc.action, func(t *testing.T) {
			fx, _, statePath := rewriteFixture(t)
			fx.platform.addGrant(lfID, "judy", "role-cla-manager", "")
			nameless := len(fx.platform.grants[lfID]) - 1
			fx.platform.grants[lfID][nameless].RoleName = ""
			mapping := writeMapping(t, t.TempDir(), lfID+","+targetSFID+","+tc.action+",true")
			opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
			plan, err := BuildPlan(context.Background(), fx.deps(), opts)
			require.NoError(t, err)
			require.Len(t, plan.Rewrite, 1)
			sum, err := Execute(context.Background(), fx.deps(), opts, plan)
			if !tc.stops {
				require.NoError(t, err)
				assert.Equal(t, 1, sum.Rewritten)
				assert.Contains(t, targetGrants(t, fx), "judy:cla-manager:"+targetSFID, "a new Account takes every grant, named or not")
				return
			}
			require.Error(t, err)
			assert.ErrorContains(t, plan.Rewrite[0].Err, "has no role name")
			assert.Empty(t, fx.companies.updates, "the group stops before the row cutover")
			assert.Len(t, mustListOrgGrants(t, fx, lfID), 4, "nothing is deleted on the old org")
			st, err := LoadState(statePath)
			require.NoError(t, err)
			assert.Equal(t, StepGrants, st.Last[lfID].Step)
			assert.Equal(t, statusFailed, st.Last[lfID].Status)
		})
	}
}

func TestResumedGroupFindsItsAccountSharedThroughTheMappingOrAnExistingRow(t *testing.T) {
	for _, tc := range []struct {
		name  string
		share func(fx *fixture) []string
	}{
		{"another mapping row", func(*fixture) []string { return []string{"lf-not-in-this-run," + targetSFID + ",created,true"} }},
		{"a row already on the Account", func(fx *fixture) []string {
			fx.company("c-x", "Acme Old", "", targetSFID, "cg-9")
			return nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx, dir := collisionFixture(t)
			fx.platform.addGrant("lf-a", "alice", "role-cla-manager", "")
			fx.platform.addGrant("lf-a", "alice", "role-company-admin", "")
			statePath := filepath.Join(dir, "state.jsonl")
			require.NoError(t, os.WriteFile(statePath, []byte(`{"old_id":"lf-a","new_id":"`+targetSFID+`","company_ids":["c-a"],"all_grants":true,`+
				`"step":"wait","ts":"2026-09-29T11:00:00Z","status":"failed","err":"org-service does not serve it yet"}`+"\n"), 0o600))
			mapping := writeMapping(t, dir, append([]string{"lf-a," + targetSFID + ",created,true"}, tc.share(fx)...)...)
			opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: filepath.Join(dir, "out")}
			plan, err := BuildPlan(context.Background(), fx.deps(), opts)
			require.NoError(t, err)
			require.Len(t, plan.Rewrite, 1)
			require.True(t, plan.Rewrite[0].Replayed)
			sum, err := Execute(context.Background(), fx.deps(), opts, plan)
			require.NoError(t, err)
			assert.Equal(t, 1, sum.Rewritten)

			assert.Equal(t, []string{"alice:cla-manager:" + targetSFID}, targetGrants(t, fx))
			assert.Equal(t, [][]string{droppedHeader, {"lf-a", targetSFID, "alice", "company-admin", objectTypeOrganization, "lf-a"}},
				droppedReport(t, opts.OutDir))
		})
	}
}

func TestGrantAddedOnTheAccountBeforeCleanupIsNotReported(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.addGrant(lfID, "grace", "role-company-admin", "")
	fx.platform.addGrant(lfID, "heidi", "role-viewer", "")
	// grace is made an admin of the Account by somebody else between the copy and the cleanup
	lists := 0
	fx.platform.listHook = func(orgID string) {
		if orgID == targetSFID {
			if lists++; lists == 2 {
				fx.platform.addGrantLocked(targetSFID, "grace", "role-company-admin", "")
			}
		}
	}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	sum, err := runIngest(t, fx, opts)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Contains(t, fx.out.String(), "2 not copied")
	assert.Equal(t, [][]string{droppedHeader, {lfID, targetSFID, "heidi", "viewer", objectTypeOrganization, lfID}},
		droppedReport(t, opts.OutDir))
}

func TestCollapseDecisionForAnotherAccountDoesNotApply(t *testing.T) {
	fx, dir := collisionFixture(t)
	fx.platform.addGrant("lf-a", "alice", "role-cla-manager", "")
	fx.platform.addGrant("lf-a", "alice", "role-company-admin", "")
	// the decision collapses lf-a onto another Account than the new one the mapping creates for it alone
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",created,true")
	decisions := writeDecisions(t, dir, "collapse,lf-a;lf-b,"+targetSFID2+",michal,one legal entity")
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, Decisions: decisions, State: filepath.Join(dir, "state.jsonl"), Routes: []Route{RouteRewrite}, OutDir: filepath.Join(dir, "out")}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)

	assert.Equal(t, []string{"alice:cla-manager:" + targetSFID, "alice:company-admin:" + targetSFID}, targetGrants(t, fx))
	assert.Equal(t, [][]string{droppedHeader}, droppedReport(t, opts.OutDir))
}

func TestFailedGrantStepNamesNoUser(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail func(fx *fixture)
		step string
	}{
		{"copy", func(fx *fixture) { fx.platform.failCreate = true }, StepGrants},
		{"cleanup", func(fx *fixture) { fx.platform.failDelete = true }, StepCleanup},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx, mapping, statePath := rewriteFixture(t)
			tc.fail(fx)
			opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
			plan, err := BuildPlan(context.Background(), fx.deps(), opts)
			require.NoError(t, err)
			_, err = Execute(context.Background(), fx.deps(), opts, plan)
			require.Error(t, err)
			journal, err := os.ReadFile(filepath.Clean(statePath))
			require.NoError(t, err)
			assert.Contains(t, string(journal), `"step":"`+tc.step+`"`)
			for _, user := range []string{"alice", "bob", "carol"} {
				assert.NotContains(t, plan.Rewrite[0].Err.Error(), user)
				assert.NotContains(t, string(journal), user)
				assert.NotContains(t, fx.out.String(), user)
			}
		})
	}
}

func TestDroppedGrantsOfAFinishedRewriteStayListedWhenItsKeyIsPlannedAgain(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	// the rewrite of lfID finished earlier; its rows were put back on the old id by hand since, so the plan holds that key again
	require.NoError(t, os.WriteFile(statePath, []byte(`{"old_id":"`+lfID+`","new_id":"`+targetSFID+`","company_ids":["c-parent","c-sub"],"dropped":[`+
		`{"username":"","role_name":"viewer","role_id":"role-viewer","grant_id":"g-91","object_type_name":"organization","object_id":"`+lfID+`"}],`+
		`"step":"done","ts":"2026-09-29T11:00:00Z","status":"ok"}`+"\n"), 0o600))
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.NotNil(t, groupByKey(plan.Groups, lfID))
	require.Empty(t, plan.Rewrite, "a finished group is not rewritten again")
	_, err = Execute(context.Background(), fx.deps(), opts, plan)
	require.Error(t, err, "the events recheck refuses rows that no longer carry the recorded new id")
	assert.Equal(t, [][]string{droppedHeader, {lfID, targetSFID, "", "viewer", objectTypeOrganization, lfID}}, droppedReport(t, opts.OutDir))
}

func TestJournalLineLargerThanOneMebibyteIsReadBack(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.jsonl")
	st, err := LoadState(statePath)
	require.NoError(t, err)
	dropped := make([]acs_service.OrgGrant, 6000)
	for i := range dropped {
		dropped[i] = acs_service.OrgGrant{RoleName: "viewer", RoleID: "0123456789abcdef0123456789abcdef", GrantID: fmt.Sprintf("grant-%032d", i), ObjectTypeName: objectTypeOrganization, ObjectID: lfID}
	}
	require.NoError(t, st.Append(StateRecord{OldID: lfID, NewID: targetSFID, CompanyIDs: []string{"c-parent"}, Dropped: dropped, Step: StepDone, Status: statusOK}, time.Now()))
	info, err := os.Stat(statePath)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(1<<20))
	st, err = LoadState(statePath)
	require.NoError(t, err)
	assert.Len(t, st.Last[lfID].Dropped, 6000)
}
