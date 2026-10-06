// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 15-char forms of the 18-char fixture ids (same Salesforce Account, checksum suffix dropped)
var (
	liveSFID15   = liveSFID[:15]
	targetSFID15 = targetSFID[:15]
)

func TestAccountKey(t *testing.T) {
	assert.Equal(t, targetSFID15, accountKey(targetSFID))
	assert.Equal(t, targetSFID15, accountKey(targetSFID15))
	assert.NotEqual(t, accountKey(targetSFID), accountKey(targetSFID2), "distinct Accounts stay distinct")
	assert.Equal(t, lfID, accountKey(lfID), "legacy ids are compared as stored")
	assert.Equal(t, "", accountKey(""))
	assert.Equal(t, "0064100000Te0G7AAJ", accountKey("0064100000Te0G7AAJ"), "non-Account ids are never shortened")
	assert.Equal(t, ShapeSFID, ShapeOf(targetSFID15))
}

func TestMappingSameAccountFormIsSameID(t *testing.T) {
	m, err := ParseMapping(strings.NewReader("old_id,new_id,action,approved\n" + deadSFID + "," + deadSFID[:15] + ",matched,true\n" + lfID + "," + targetSFID + ",matched,true\n"))
	require.NoError(t, err)
	_, _, reason := m.Resolve(deadSFID)
	assert.Equal(t, ReasonMappingSameID, reason, "the 15-char form of the old id is the same Account")
	newID, _, reason := m.Resolve(lfID)
	assert.Empty(t, reason)
	assert.Equal(t, targetSFID, newID)
	assert.Contains(t, Suggest(ReasonMappingSameID), "same Account")
}

func TestCollisionAcrossSFIDForms(t *testing.T) {
	fx, dir := collisionFixture(t)
	fx.platform.sfAccounts[targetSFID15] = true
	// two old ids mapped to the two forms of one Account collide like any co-targets
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",matched,true", "lf-b,"+targetSFID15+",created,true")
	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping})
	require.NoError(t, err)
	require.Len(t, plan.Targets, 1, "both forms are one destination")
	tgt := plan.Targets[0]
	assert.Equal(t, []string{"lf-a", "lf-b"}, tgt.OldIDs())
	assert.Equal(t, ReasonTargetFormsDiffer, tgt.Status)
	for _, id := range []string{"lf-a", "lf-b"} {
		g := groupByKey(plan.Groups, id)
		assert.Equal(t, RouteManual, g.Route, id)
		assert.Equal(t, ReasonTargetFormsDiffer, g.ManualReason, id)
	}
	// a collapse decision does not make the tool write two forms of one id
	decisions := writeDecisions(t, dir, "collapse,lf-a;lf-b,"+targetSFID+",michal,")
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping, Decisions: decisions})
	require.NoError(t, err)
	assert.Equal(t, ReasonTargetFormsDiffer, groupByKey(plan.Groups, "lf-a").ManualReason)
	assert.Equal(t, ReasonTargetFormsDiffer, groupByKey(plan.Groups, "lf-b").ManualReason)
	assert.Empty(t, plan.Rewrite)

	// mapping normalized to one form: an ordinary collision that the decision (in either form) approves
	mapping = writeMapping(t, dir, "lf-a,"+targetSFID15+",matched,true", "lf-b,"+targetSFID15+",created,true")
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping})
	require.NoError(t, err)
	assert.Equal(t, "target_collision", groupByKey(plan.Groups, "lf-a").ManualReason)
	assert.Equal(t, "needs_decision", targetByID(plan.Targets, targetSFID15).Status)
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping, Decisions: decisions})
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 2)
	for _, id := range []string{"lf-a", "lf-b"} {
		g := groupByKey(plan.Groups, id)
		assert.Equal(t, RouteRewrite, g.Route, id)
		assert.Equal(t, targetSFID15, g.NewID, id, "the mapped form is written as given")
		require.NotNil(t, g.Decision, id)
	}
	assert.Equal(t, "ok", targetByID(plan.Targets, targetSFID15).Status)

	// a mapping row outside the tranche pointing at the other form is a co-target too
	mapping = writeMapping(t, dir, "lf-a,"+targetSFID+",matched,true", "lf-z,"+targetSFID15+",created,true")
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping})
	require.NoError(t, err)
	assert.Equal(t, ReasonTargetFormsDiffer, groupByKey(plan.Groups, "lf-a").ManualReason)
	assert.Equal(t, []string{"lf-a", "lf-z"}, targetByID(plan.Targets, targetSFID).OldIDs())
	mapping = writeMapping(t, dir, "lf-a,"+targetSFID+",matched,true", "lf-z,"+targetSFID+",created,true", "lf-c,"+targetSFID2+",created,true")
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping})
	require.NoError(t, err)
	assert.Equal(t, "target_collision", groupByKey(plan.Groups, "lf-a").ManualReason)
	assert.Equal(t, RouteRewrite, groupByKey(plan.Groups, "lf-c").Route, "a different Account is not a collision")
	assert.Contains(t, Suggest(ReasonTargetFormsDiffer), "one form")
}

func TestCollisionWithExistingRowInOtherForm(t *testing.T) {
	fx, dir := collisionFixture(t)
	fx.company("c-existing", "Acme Holdings", "", targetSFID15)
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",matched,true")
	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping})
	require.NoError(t, err)
	a := groupByKey(plan.Groups, "lf-a")
	assert.Equal(t, RouteManual, a.Route)
	assert.Equal(t, ReasonTargetFormsDiffer, a.ManualReason, "a row carrying the 15-char form of the target is on the same Account")
	tgt := targetByID(plan.Targets, targetSFID)
	require.NotNil(t, tgt)
	require.Len(t, tgt.Existing, 1)
	assert.Equal(t, []string{"lf-a", targetSFID15}, tgt.OldIDs())
	assert.Equal(t, ReasonTargetFormsDiffer, tgt.Status)
	assert.True(t, plan.inv.ExternalIDCollision(a, targetSFID))

	// mapping normalized to the form the existing row carries: a plain collision, lifted by a collapse
	// decision that may name the Account in either form
	mapping = writeMapping(t, dir, "lf-a,"+targetSFID15+",matched,true")
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping})
	require.NoError(t, err)
	assert.Equal(t, "target_collision", groupByKey(plan.Groups, "lf-a").ManualReason)
	decisions := writeDecisions(t, dir, "collapse,lf-a;"+targetSFID15+","+targetSFID+",michal,")
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping, Decisions: decisions})
	require.NoError(t, err)
	assert.Equal(t, RouteRewrite, groupByKey(plan.Groups, "lf-a").Route)
	assert.Equal(t, targetSFID15, groupByKey(plan.Groups, "lf-a").NewID)
	assert.Equal(t, "ok", targetByID(plan.Targets, targetSFID15).Status)
}

func TestSFIDAliasFormsAreManual(t *testing.T) {
	fx := newFixture()
	fx.company("c-18", "Acme", "", liveSFID, "cg-1")
	fx.company("c-15", "Acme", "Acme Labs", liveSFID15, "cg-2")
	fx.company("c-other", "Other", "", liveSFID2, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[liveSFID15] = true
	fx.platform.sfAccounts[liveSFID2] = true
	dir := t.TempDir()
	opts := Options{Stage: "dev", Apply: true, OutDir: dir}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	for key, alias := range map[string]string{liveSFID: liveSFID15, liveSFID15: liveSFID} {
		g := groupByKey(plan.Groups, key)
		require.NotNil(t, g, key)
		assert.Equal(t, RouteManual, g.Route, key)
		assert.Equal(t, ReasonSFIDAliasForms, g.ManualReason, key)
		assert.Equal(t, []string{alias}, g.Aliases, key)
		assert.Equal(t, key, g.OldID, key, "the stored id is kept as is")
		assert.Empty(t, g.Live, key)
	}
	other := groupByKey(plan.Groups, liveSFID2)
	assert.Equal(t, RouteRegister, other.Route, "a different Account is unaffected")
	assert.Empty(t, other.Aliases)
	for _, c := range fx.platform.calls {
		assert.False(t, strings.HasPrefix(c, "get-b2b:"+liveSFID15), "no liveness call before the rows are normalized: %s", c)
	}
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 2, sum.Manual)
	assert.Equal(t, 1, sum.Registered)
	assert.Equal(t, []string{liveSFID2}, fx.platform.registered, "neither form of the aliased Account is registered")
	assert.Empty(t, fx.companies.updates)
	var buf bytes.Buffer
	plan.Print(&buf)
	assert.Contains(t, buf.String(), "reason="+ReasonSFIDAliasForms+" aliases="+liveSFID15)
	manual := readCSV(t, filepath.Join(dir, "manual_actions.csv"))
	require.Len(t, manual, 3)
	assert.Contains(t, Suggest(ReasonSFIDAliasForms), "normalize")

	// the audit reports the same gate
	res, err := Audit(context.Background(), fx.deps(), Options{Stage: "dev", OutDir: t.TempDir()})
	require.NoError(t, err)
	assert.Equal(t, ReasonSFIDAliasForms, groupByKey(res.Groups, liveSFID).ManualReason)
	assert.Equal(t, ReasonSFIDAliasForms, groupByKey(res.Groups, liveSFID15).ManualReason)
	assert.Equal(t, 2, res.Routes[RouteManual])
}

func TestSFIDAliasSiblingWithoutCCLAIsManual(t *testing.T) {
	// only the 18-char rows have an active CCLA; the 15-char sibling would otherwise be left behind
	fx := newFixture()
	fx.company("c-18", "Acme", "", deadSFID, "cg-1")
	fx.company("c-15", "Acme", "Acme Labs", deadSFID[:15])
	dir := t.TempDir()
	mapping := writeMapping(t, dir, deadSFID+","+targetSFID+",matched,true")
	fx.platform.sfAccounts[targetSFID] = true
	fx.platform.orgs[targetSFID] = &Org{ID: targetSFID}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: filepath.Join(dir, "state.jsonl")}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Groups, 1, "a row without an active CCLA is not a group of its own")
	g := plan.Groups[0]
	assert.Equal(t, RouteManual, g.Route)
	assert.Equal(t, ReasonSFIDAliasForms, g.ManualReason)
	assert.Equal(t, []string{deadSFID[:15]}, g.Aliases)
	assert.Empty(t, plan.Rewrite)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Manual)
	assert.Empty(t, fx.companies.updates, "neither representation is rewritten on its own")
	assert.Empty(t, fx.platform.registered)
}

func TestDuplicateEntityParentKey(t *testing.T) {
	assert.True(t, hasDuplicateEntity([]*Row{
		{CompanyID: "a", CompanyName: "Acme", SigningEntityName: ""},
		{CompanyID: "b", CompanyName: "Acme", SigningEntityName: "Acme"},
	}), "an entity name equal to the company name is the parent row, like an empty one")
	assert.True(t, hasDuplicateEntity([]*Row{
		{CompanyID: "a", CompanyName: "Acme", SigningEntityName: "acme "},
		{CompanyID: "b", CompanyName: "Acme", SigningEntityName: ""},
	}))
	assert.False(t, hasDuplicateEntity([]*Row{
		{CompanyID: "a", CompanyName: "Acme", SigningEntityName: "Acme"},
		{CompanyID: "b", CompanyName: "Acme", SigningEntityName: "Acme GmbH"},
	}), "distinct signing entities stay distinct")
	assert.True(t, hasDuplicateEntity([]*Row{
		{CompanyID: "a", CompanyName: "Acme", SigningEntityName: ""},
		{CompanyID: "b", CompanyName: "Acme GmbH", SigningEntityName: "Acme GmbH"},
	}), "two parent-style rows of one Account are reported even when their names differ (review aid, never fewer flags)")

	fx := newFixture()
	fx.company("c-parent", "Acme", "Acme", liveSFID, "cg-1")
	fx.company("c-dup", "Acme", "", liveSFID)
	fx.company("c-sub", "Acme", "Acme GmbH", liveSFID)
	inv, err := LoadInventory(context.Background(), fx.deps())
	require.NoError(t, err)
	acme := groupByKey(inv.EligibleGroups(), liveSFID)
	require.NotNil(t, acme)
	assert.True(t, acme.Duplicate)
	assert.Empty(t, acme.Aliases)
}

func TestCopyGrantsRefusesGrantWithoutUsername(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.addGrant(lfID, "", "role-mgr", "")
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	require.Error(t, groupByKey(plan.Groups, lfID).Err)
	assert.Contains(t, groupByKey(plan.Groups, lfID).Err.Error(), "has no username")
	assert.Empty(t, fx.companies.updates, "the group stops before the row cutover")
	assert.Equal(t, lfID, fx.companies.rows["c-parent"].CompanyExternalID)
	assert.Len(t, mustListOrgGrants(t, fx, lfID), 4, "nothing is deleted on the old org")
	assert.Empty(t, fx.events.rekeyed)
	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.False(t, st.Done(lfID))
	assert.Equal(t, StepGrants, st.Last[lfID].Step)
	assert.Equal(t, statusFailed, st.Last[lfID].Status)
	assert.Contains(t, st.Last[lfID].Err, "has no username")
}

func TestCleanupRefusesToDeleteGrantWithoutUsername(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	// the grant appears on the old org after the copy step (an ACS write racing the import)
	lists := 0
	fx.platform.listHook = func(orgID string) {
		if orgID == lfID {
			if lists++; lists == 2 {
				fx.platform.addGrantLocked(lfID, "", "role-mgr", "cg-1")
			}
		}
	}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.Error(t, err)
	assert.Equal(t, 0, sum.Rewritten)
	require.Error(t, groupByKey(plan.Groups, lfID).Err)
	assert.Contains(t, groupByKey(plan.Groups, lfID).Err.Error(), "has no username")
	assert.Equal(t, targetSFID, fx.companies.rows["c-parent"].CompanyExternalID, "rows were cut over before cleanup")
	var kept []string
	for _, g := range mustListOrgGrants(t, fx, lfID) {
		kept = append(kept, g.Username+"/"+g.RoleID)
	}
	assert.Contains(t, kept, "/role-mgr", "the uncopiable grant is never deleted")
	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.False(t, st.Done(lfID), "no false done record")
	assert.Equal(t, StepCleanup, st.Last[lfID].Step)

	// once the grant is fixed (a username is known) the replay converges
	fx.platform.listHook = nil
	for i := range fx.platform.grants[lfID] {
		if fx.platform.grants[lfID][i].Username == "" {
			fx.platform.grants[lfID][i].Username = "late-user"
		}
	}
	plan, err = BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	sum, err = Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Empty(t, mustListOrgGrants(t, fx, lfID))
	var users []string
	for _, g := range mustListOrgGrants(t, fx, targetSFID) {
		users = append(users, g.Username)
	}
	assert.Contains(t, users, "late-user")
	st, err = LoadState(statePath)
	require.NoError(t, err)
	assert.True(t, st.Done(lfID))
}
