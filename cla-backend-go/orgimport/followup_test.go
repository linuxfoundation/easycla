// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

const (
	unregSFID  = "0014100000UnregAAA"
	unregSFID2 = "0014100000FreshBBB"
)

// A 403 on GET /b2b_orgs while other Accounts answer 200 means "no b2b_org yet", not an error.
func TestUnregisteredAccountsArePending(t *testing.T) {
	fx := newFixture()
	fx.company("c-live", "Live Corp", "", liveSFID, "cg-1")
	fx.company("c-unreg", "Fresh Corp", "", unregSFID, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[unregSFID] = true
	fx.platform.forbidden = map[string]int{unregSFID: -1}
	fx.platform.orgs[unregSFID] = &Org{ID: unregSFID, Name: "Fresh Corp", Website: "https://fresh.example"}

	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Apply: true})
	require.NoError(t, err)
	g := groupByKey(plan.Groups, unregSFID)
	require.NotNil(t, g)
	assert.Equal(t, "unregistered", g.Live)
	assert.Equal(t, RouteRegister, g.Route)
	assert.Equal(t, "unregistered", g.ManualReason)
	assert.NoError(t, g.Err)
	assert.True(t, g.Pending())
	assert.Equal(t, "200", g.OrgStatus, "org-service is still consulted for the report")
	require.Len(t, plan.Register, 1)
	assert.Equal(t, liveSFID, plan.Register[0].OldID)

	sum, err := Execute(context.Background(), fx.deps(), Options{Apply: true}, plan)
	require.NoError(t, err)
	assert.Equal(t, Summary{Mode: "apply", Eligible: 2, Registered: 1, Pending: 1}, sum)
	assert.Equal(t, []string{liveSFID}, fx.platform.registered)
	actions := plan.ManualActions()
	require.Len(t, actions, 1)
	assert.Equal(t, "unregistered", actions[0].Reason)
	assert.Contains(t, actions[0].Suggested, "--register-unregistered")

	dir := t.TempDir()
	fx.out.Reset()
	res, err := Audit(context.Background(), fx.deps(), Options{Stage: "dev", OutDir: dir})
	require.NoError(t, err)
	assert.Equal(t, map[Route]int{RouteRegister: 2}, res.Routes)
	rows := readCSV(t, filepath.Join(dir, "audit.csv"))
	for _, r := range rows[1:] {
		if r[0] == "c-unreg" {
			assert.Equal(t, "unregistered", r[12])
		}
	}
	assert.Contains(t, fx.out.String(), " unregistered=1 ")
}

// A usable mapping row moves an Account that answers 403 to the rewrite route; any other one stays pending.
func TestUnregisteredAccountWithMappingIsRewritten(t *testing.T) {
	fx := newFixture()
	fx.company("c-live", "Live Corp", "", liveSFID, "cg-1")
	fx.company("c-unreg", "Fresh Corp", "", unregSFID, "cg-1")
	fx.company("c-unreg2", "Other Corp", "", unregSFID2, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[targetSFID] = true
	fx.platform.sfAccounts[targetSFID2] = true
	fx.platform.forbidden = map[string]int{unregSFID: -1, unregSFID2: -1}
	fx.platform.orgs[unregSFID] = &Org{ID: unregSFID, Name: "Fresh Corp", Website: "https://fresh.example"}
	fx.platform.orgs[targetSFID] = &Org{ID: targetSFID, Name: "Fresh Corporation"}
	fx.platform.addGrant(unregSFID, "alice", "role-cla-manager", "")
	dir := t.TempDir()
	mapping := writeMapping(t, dir, unregSFID+","+targetSFID+",matched,true", unregSFID2+","+targetSFID2+",matched,false")
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: filepath.Join(dir, "state.jsonl"), Routes: []Route{RouteRewrite}}

	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	g := groupByKey(plan.Groups, unregSFID)
	require.NotNil(t, g)
	assert.Equal(t, LiveUnregistered, g.Live)
	assert.Equal(t, RouteRewrite, g.Route)
	assert.Equal(t, targetSFID, g.NewID)
	assert.Equal(t, ActionMatched, g.Action)
	assert.Empty(t, g.ManualReason)
	assert.False(t, g.Pending())
	assert.False(t, g.AllGrants, "a matched Account gets the EasyCLA roles only")
	other := groupByKey(plan.Groups, unregSFID2)
	require.NotNil(t, other)
	assert.Equal(t, RouteRegister, other.Route, "a mapping row that is not approved moves nothing")
	assert.Equal(t, ReasonUnregistered, other.ManualReason)
	assert.Empty(t, other.NewID)
	require.Len(t, plan.Rewrite, 1)
	assert.Equal(t, unregSFID, plan.Rewrite[0].OldID)
	assert.Equal(t, 1, plan.Unregistered(), "only the Account that no mapping row moves")
	assert.Equal(t, 1, plan.Pending())

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Equal(t, 1, sum.Pending)
	assert.Zero(t, sum.Failed)
	assert.Equal(t, []string{"c-unreg:" + unregSFID + "->" + targetSFID}, fx.companies.updates)
	assert.Equal(t, unregSFID, fx.companies.rows["c-unreg"].PreviousCompanyExternalID)
	assert.Equal(t, unregSFID2, fx.companies.rows["c-unreg2"].CompanyExternalID)
	assert.Len(t, mustListOrgGrants(t, fx, targetSFID), 1)
	assert.Empty(t, mustListOrgGrants(t, fx, unregSFID))
	assert.Equal(t, []string{targetSFID}, fx.platform.registered, "the old id is never registered")

	// --register-unregistered does not override the mapping row
	fx2 := newFixture()
	fx2.company("c-live", "Live Corp", "", liveSFID, "cg-1")
	fx2.company("c-unreg", "Fresh Corp", "", unregSFID, "cg-1")
	fx2.platform.sfAccounts[liveSFID] = true
	fx2.platform.sfAccounts[targetSFID] = true
	fx2.platform.forbidden = map[string]int{unregSFID: -1}
	plan, err = BuildPlan(context.Background(), fx2.deps(), Options{Mapping: mapping, RegisterUnregistered: true})
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	assert.Equal(t, unregSFID, plan.Rewrite[0].OldID)
	require.Len(t, plan.Register, 1)
	assert.Equal(t, liveSFID, plan.Register[0].OldID)
}

// A mapping target that member-service cannot read is never a rewrite destination.
func TestMappingTargetMustBeReadable(t *testing.T) {
	const targetSFID3 = "0014100000NewThree"
	fx := newFixture()
	fx.company("c-dead", "Dead Corp", "", deadSFID, "cg-1")
	fx.company("c-lf", "Legacy Corp", "", lfID, "cg-1")
	fx.company("c-unreg", "Fresh Corp", "", unregSFID, "cg-1")
	fx.platform.sfAccounts[targetSFID3] = true
	fx.platform.forbidden = map[string]int{unregSFID: -1, targetSFID2: -1}
	fx.platform.orgs[targetSFID] = &Org{ID: targetSFID, Name: "Gone Corp"}
	fx.platform.orgs[targetSFID2] = &Org{ID: targetSFID2, Name: "Merged Corp"}
	fx.platform.orgs[targetSFID3] = &Org{ID: targetSFID3, Name: "Fresh Corporation"}
	fx.platform.addGrant(deadSFID, "alice", "role-cla-manager", "")
	dir := t.TempDir()
	mapping := writeMapping(t, dir, deadSFID+","+targetSFID+",matched,true", lfID+","+targetSFID2+",matched,true", unregSFID+","+targetSFID3+",matched,true")
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: filepath.Join(dir, "state.jsonl"), Routes: []Route{RouteRewrite}, OutDir: dir}

	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	for key, target := range map[string]string{deadSFID: targetSFID, lfID: targetSFID2} {
		g := groupByKey(plan.Groups, key)
		require.NotNil(t, g, key)
		assert.Equal(t, RouteManual, g.Route, key)
		assert.Equal(t, "target_not_live", g.ManualReason, key)
		assert.Equal(t, target, g.NewID, key)
		assert.NoError(t, g.Err, key)
	}
	require.Len(t, plan.Rewrite, 1)
	assert.Equal(t, unregSFID, plan.Rewrite[0].OldID)
	require.Len(t, plan.Targets, 1)
	assert.Equal(t, targetSFID3, plan.Targets[0].SFID)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Equal(t, 2, sum.Manual)
	assert.Zero(t, sum.Failed)
	assert.Equal(t, []string{"c-unreg:" + unregSFID + "->" + targetSFID3}, fx.companies.updates)
	assert.Equal(t, []string{targetSFID3}, fx.platform.registered)
	assert.Len(t, mustListOrgGrants(t, fx, deadSFID), 1, "the grants of a group without a readable target stay where they are")
	assert.Empty(t, mustListOrgGrants(t, fx, targetSFID))
	assert.Equal(t, map[string]string{"route": "manual", "manual_reason": "target_not_live", "new_id": targetSFID},
		csvCells(t, filepath.Join(dir, "plan.csv"), "key", deadSFID, "route", "manual_reason", "new_id"))
	assert.Equal(t, map[string]string{"reason": "target_not_live", "new_id": targetSFID2},
		csvCells(t, filepath.Join(dir, "manual_actions.csv"), "old_id", lfID, "reason", "new_id"))
	actions := plan.ManualActions()
	require.Len(t, actions, 2)
	assert.Contains(t, actions[0].Suggested, "fix the mapping row")
}

// An unfinished rewrite resumes only onto a target that member-service can still read: nothing more is written for it.
func TestUnfinishedRewriteStopsWhenItsTargetIsNotReadable(t *testing.T) {
	for name, forbidden := range map[string]bool{"404": false, "403": true} {
		t.Run(name, func(t *testing.T) {
			fx, mapping, statePath := rewriteFixture(t)
			delete(fx.platform.sfAccounts, targetSFID)
			if forbidden {
				fx.platform.forbidden = map[string]int{targetSFID: -1}
			}
			require.NoError(t, os.WriteFile(statePath, []byte(`{"ts":"2026-09-29T11:00:00Z","old_id":"`+lfID+`","new_id":"`+targetSFID+`","company_ids":["c-parent","c-sub"],"step":"copy_grants","status":"ok"}`+"\n"), 0o600))
			opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}

			plan, err := BuildPlan(context.Background(), fx.deps(), opts)
			require.NoError(t, err)
			g := groupByKey(plan.Groups, lfID)
			require.NotNil(t, g)
			assert.True(t, g.Replayed)
			assert.ErrorContains(t, g.Err, "target check failed for "+targetSFID)
			assert.Empty(t, plan.Rewrite)

			sum, err := Execute(context.Background(), fx.deps(), opts, plan)
			assert.Error(t, err)
			assert.Equal(t, 1, sum.Failed)
			assert.Empty(t, fx.companies.updates)
			assert.Empty(t, fx.platform.registered)
			assert.Len(t, mustListOrgGrants(t, fx, lfID), 3, "the old grants stay")
		})
	}
}

// A resumed rewrite with nothing left but its registration runs it although the target still answers 403: that POST settles it.
func TestUnfinishedRewriteRegistersATargetThatAnswers403(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.forbidden = map[string]int{targetSFID: 2}
	require.NoError(t, os.WriteFile(statePath, []byte(`{"ts":"2026-09-29T11:00:00Z","old_id":"`+lfID+`","new_id":"`+targetSFID+`","company_ids":["c-parent","c-sub"],"step":"delete_old_grants","status":"ok"}`+"\n"), 0o600))
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}

	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	g := groupByKey(plan.Groups, lfID)
	require.NotNil(t, g)
	assert.True(t, g.Replayed)
	assert.NoError(t, g.Err)
	require.Len(t, plan.Rewrite, 1)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Equal(t, []string{targetSFID}, fx.platform.registered)

	delete(fx.platform.sfAccounts, targetSFID)
	fx.platform.forbidden = nil
	require.NoError(t, os.WriteFile(statePath, []byte(`{"ts":"2026-09-29T11:00:00Z","old_id":"`+lfID+`","new_id":"`+targetSFID+`","company_ids":["c-parent","c-sub"],"step":"register","status":"failed"}`+"\n"), 0o600))
	plan, err = BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	assert.ErrorContains(t, groupByKey(plan.Groups, lfID).Err, "target check failed for "+targetSFID, "a 404 still stops it")
}

// Without one Account that answers 200 or 404, a target that answers 403 is the access problem, not a missing Account.
func TestForbiddenTargetWithoutAVisibleAccountIsAnAccessError(t *testing.T) {
	fx := newFixture()
	fx.company("c-lf", "Legacy Corp", "", lfID, "cg-1")
	fx.platform.sfAccounts[targetSFID] = true
	fx.platform.forbidden = map[string]int{targetSFID: -1}
	mapping := writeMapping(t, t.TempDir(), lfID+","+targetSFID+",matched,true")

	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Mapping: mapping})
	require.NoError(t, err)
	g := groupByKey(plan.Groups, lfID)
	require.NotNil(t, g)
	require.Error(t, g.Err)
	assert.ErrorContains(t, g.Err, "target check failed for "+targetSFID)
	assert.ErrorContains(t, g.Err, "cannot see any b2b_org")
	assert.Contains(t, SuggestError(g.Err), "auditor")
	assert.Empty(t, plan.Rewrite)

	fx.platform.forbidden = nil
	fx.platform.tokenDenied = true
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Mapping: mapping})
	require.NoError(t, err)
	g = groupByKey(plan.Groups, lfID)
	require.NotNil(t, g)
	require.Error(t, g.Err)
	assert.ErrorContains(t, g.Err, "target check failed for "+targetSFID)
	assert.NotContains(t, g.Err.Error(), "for no Account")
	assert.Empty(t, plan.Rewrite)
}

// A mapping row does not turn a missing access tuple into rewrites: when no Account answers 200 or 404 nothing is planned.
func TestAllForbiddenWithMappingIsStillAnAccessError(t *testing.T) {
	fx := newFixture()
	fx.company("c-a", "A", "", unregSFID, "cg-1")
	fx.company("c-b", "B", "", unregSFID2, "cg-1")
	fx.platform.sfAccounts[targetSFID] = true
	fx.platform.forbidden = map[string]int{unregSFID: -1, unregSFID2: -1}
	mapping := writeMapping(t, t.TempDir(), unregSFID+","+targetSFID+",matched,true")

	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Mapping: mapping})
	require.NoError(t, err)
	g := groupByKey(plan.Groups, unregSFID)
	require.NotNil(t, g)
	assert.Equal(t, LiveError, g.Live)
	assert.Empty(t, g.Route)
	require.Error(t, g.Err)
	assert.ErrorContains(t, g.Err, "403 for 2")
	assert.Empty(t, plan.Rewrite)
	assert.Empty(t, plan.Register)
}

// When every GET answers 403 the client cannot see any b2b_org at all: that is the old error shape.
func TestAllForbiddenIsAnAccessError(t *testing.T) {
	fx := newFixture()
	fx.company("c-a", "A", "", unregSFID, "cg-1")
	fx.company("c-b", "B", "", unregSFID2, "cg-1")
	fx.company("c-dead", "Dead Corp", "", deadSFID, "cg-1")
	fx.platform.sfAccounts[unregSFID] = true
	fx.platform.sfAccounts[unregSFID2] = true
	fx.platform.forbidden = map[string]int{unregSFID: -1, unregSFID2: -1, deadSFID: -1}

	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Apply: true})
	require.NoError(t, err)
	for _, key := range []string{unregSFID, unregSFID2, deadSFID} {
		g := groupByKey(plan.Groups, key)
		require.NotNil(t, g, key)
		assert.Equal(t, LiveError, g.Live)
		assert.Empty(t, g.Route)
		assert.Empty(t, g.ManualReason)
		require.Error(t, g.Err)
		assert.ErrorContains(t, g.Err, "403 for 3")
		var authErr *member_service.AuthError
		assert.ErrorAs(t, g.Err, &authErr)
		assert.Contains(t, SuggestError(g.Err), "auditor")
	}
	assert.Empty(t, plan.Register)
	sum, err := Execute(context.Background(), fx.deps(), Options{Apply: true}, plan)
	assert.Error(t, err)
	assert.Equal(t, 3, sum.Failed)
	assert.Empty(t, fx.platform.registered)

	dir := t.TempDir()
	fx.out.Reset()
	_, err = Audit(context.Background(), fx.deps(), Options{Stage: "dev", OutDir: dir})
	require.NoError(t, err)
	rows := readCSV(t, filepath.Join(dir, "audit.csv"))
	for _, r := range rows[1:] {
		assert.Contains(t, r[12], "403 for 3", r[0])
	}
	assert.Contains(t, fx.out.String(), " unregistered=0 ")
}

// suggested_account: EasyCLA-internal candidates (same registrable domain, then same normalized
// name) among Accounts served by org-service, verified through member-service.
func TestAuditSuggestedAccountFromInventory(t *testing.T) {
	const (
		oldSFID  = "0014100000OldAAAAA"
		old2SFID = "0014100000OldBBBBB"
		newSFID  = "0014100000NewCCCCC"
	)
	fx := newFixture()
	fx.company("c-old", "Acme", "", oldSFID, "cg-1")            // dead, same name as the live Acme
	fx.company("c-old2", "Widgets", "", old2SFID, "cg-1")       // dead, website under acme.example
	fx.company("c-new", "Acme", "", newSFID, "cg-1")            // live Account
	fx.company("c-lf", "Acme", "", "lf-acme-legacy-01", "cg-1") // lf id, same name
	fx.company("c-unrelated", "Zeta", "", liveSFID2, "cg-1")
	fx.platform.sfAccounts[newSFID] = true
	fx.platform.sfAccounts[liveSFID2] = true
	fx.platform.orgs[newSFID] = &Org{ID: newSFID, Name: "Acme Inc.", Website: "https://www.acme.example"}
	fx.platform.orgs[old2SFID] = &Org{ID: old2SFID, Name: "Widgets", Website: "https://labs.acme.example/x"}
	fx.platform.orgs[liveSFID2] = &Org{ID: liveSFID2, Name: "Zeta", Website: "https://zeta.example"}

	dir := t.TempDir()
	res, err := Audit(context.Background(), fx.deps(), Options{Stage: "dev", OutDir: dir})
	require.NoError(t, err)
	rows := readCSV(t, filepath.Join(dir, "audit.csv"))
	byID := map[string][]string{}
	for _, r := range rows[1:] {
		byID[r[0]] = r
	}
	assert.Equal(t, newSFID+" Acme Inc. [inventory:name]", byID["c-old"][15])
	assert.Equal(t, newSFID+" Acme Inc. [inventory:domain]", byID["c-old2"][15])
	assert.Equal(t, newSFID+" Acme Inc. [inventory:name]", byID["c-lf"][15])
	assert.Equal(t, "", byID["c-new"][15], "a live Account never suggests itself")
	assert.Equal(t, "", byID["c-unrelated"][15])
	assert.NotEmpty(t, res.Duplicates)
	assert.Empty(t, fx.platform.registered)
	gets := map[string]int{}
	for _, c := range fx.platform.calls {
		if strings.HasPrefix(c, "get-org:") {
			gets[c]++
		}
	}
	assert.Equal(t, 1, gets["get-org:"+newSFID], "audit looks each Account up once")
	assert.Equal(t, 1, gets["get-org:"+old2SFID])
}

func TestSuggestErrorAuthError(t *testing.T) {
	wrap := func(e error) error { return fmt.Errorf("liveness check failed for X: %w", e) }
	assert.Contains(t, SuggestError(wrap(&member_service.AuthError{Status: 403, Message: "Client is not authorized", Token: true})), "client grant")
	assert.Contains(t, SuggestError(wrap(&member_service.AuthError{Status: 401, Message: "bad secret", Token: true})), "token request for the member-service audience failed (401)")
	assert.Contains(t, SuggestError(wrap(&member_service.AuthError{Status: 403, Message: "forbidden"})), "auditor")
}

// --register-unregistered POSTs the 403 Accounts too and re-reads each one until Heimdall serves it.
func TestRegisterUnregisteredRetriesTheReadBack(t *testing.T) {
	fx := newFixture()
	fx.company("c-live", "Live Corp", "", liveSFID, "cg-1")
	fx.company("c-unreg", "Fresh Corp", "", unregSFID, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[unregSFID] = true
	fx.platform.forbidden = map[string]int{unregSFID: -1}
	opts := Options{Apply: true, RegisterUnregistered: true}

	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	g := groupByKey(plan.Groups, unregSFID)
	require.NotNil(t, g)
	assert.Equal(t, "unregistered", g.Live)
	assert.Equal(t, RouteRegister, g.Route)
	assert.Empty(t, g.ManualReason)
	assert.False(t, g.Pending())
	require.Len(t, plan.Register, 2)

	fx.platform.forbidden[unregSFID] = 2
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, Summary{Mode: "apply", Eligible: 2, Registered: 2}, sum)
	assert.ElementsMatch(t, []string{liveSFID, unregSFID}, fx.platform.registered)
	assert.Equal(t, []time.Duration{3 * time.Second, 3 * time.Second}, fx.sleeps, "two 403 answers, then served")
	assert.Contains(t, fx.out.String(), "b2b_org "+unregSFID+" visible after 3 check(s)")
	assert.NotContains(t, fx.out.String(), "b2b_org "+liveSFID+" visible", "an Account that was already live is not re-read")

	fx = newFixture()
	fx.company("c-unreg", "Fresh Corp", "", unregSFID, "cg-1")
	fx.company("c-live", "Live Corp", "", liveSFID, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[unregSFID] = true
	fx.platform.forbidden = map[string]int{unregSFID: -1}
	plan, err = BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	before := len(fx.platform.calls)
	sum, err = Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 2, sum.Registered, "a pending read-back never fails the registration")
	assert.Len(t, fx.sleeps, 4)
	gets := 0
	for _, c := range fx.platform.calls[before:] {
		if c == "get-b2b:"+unregSFID {
			gets++
		}
	}
	assert.Equal(t, 5, gets)
	assert.Contains(t, fx.out.String(), "WARNING: b2b_org "+unregSFID+" not yet visible after 5 checks")
}

// A 422 on POST /b2b_orgs (the Account is neither a company Account nor a member) excludes the group
// as not_eligible: nothing is registered, re-read or written for it and the run stays green.
func TestRegisterUnregisteredNotEligible(t *testing.T) {
	fx := newFixture()
	fx.company("c-live", "Live Corp", "", liveSFID, "cg-1")
	fx.company("c-unreg", "Fresh Corp", "", unregSFID, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[unregSFID] = true
	fx.platform.forbidden = map[string]int{unregSFID: -1}
	fx.platform.notEligible = map[string]bool{unregSFID: true}
	dir := t.TempDir()
	opts := Options{Stage: "dev", Apply: true, RegisterUnregistered: true, OutDir: dir}

	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Register, 2)
	before := len(fx.platform.calls)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, Summary{Stage: "dev", Mode: "apply", Eligible: 2, Registered: 1, NotEligible: 1}, sum)
	assert.Contains(t, sum.String(), " not_eligible=1 ")
	assert.Equal(t, []string{liveSFID}, fx.platform.registered)
	assert.ElementsMatch(t, []string{"register:" + liveSFID, "register:" + unregSFID}, fx.platform.calls[before:], "no read-back or any other call after the 422")
	assert.Empty(t, fx.sleeps)
	assert.Empty(t, fx.companies.updates)
	assert.Contains(t, fx.out.String(), "not eligible "+unregSFID+": ")
	assert.NotContains(t, fx.out.String(), "FAILED")

	g := groupByKey(plan.Groups, unregSFID)
	require.NotNil(t, g)
	assert.Equal(t, RouteManual, g.Route)
	assert.Equal(t, "not_eligible", g.ManualReason)
	assert.Equal(t, "unregistered", g.Live)
	assert.NoError(t, g.Err)
	assert.False(t, g.Pending())

	actions := plan.ManualActions()
	require.Len(t, actions, 1)
	assert.Equal(t, unregSFID, actions[0].Group.Key)
	assert.Equal(t, "not_eligible", actions[0].Reason)
	assert.Contains(t, actions[0].Suggested, "nothing to do")

	assert.Equal(t, map[string]string{"route": "manual", "manual_reason": "not_eligible", "live": "unregistered"},
		csvCells(t, filepath.Join(dir, "plan.csv"), "key", unregSFID, "route", "manual_reason", "live"))
	assert.Equal(t, map[string]string{"route": "manual", "reason": "not_eligible"},
		csvCells(t, filepath.Join(dir, "manual_actions.csv"), "key", unregSFID, "route", "reason"))
	for _, row := range readCSV(t, filepath.Join(dir, "to_salesforce.csv"))[1:] {
		assert.NotEqual(t, unregSFID, row[0], "a not_eligible Account is not a Salesforce hand-off")
	}
}

// csvCells returns the named columns of the row whose keyColumn equals key.
func csvCells(t *testing.T, path, keyColumn, key string, columns ...string) map[string]string {
	t.Helper()
	rows := readCSV(t, path)
	require.NotEmpty(t, rows)
	index := map[string]int{}
	for i, name := range rows[0] {
		index[name] = i
	}
	for _, row := range rows[1:] {
		if row[index[keyColumn]] != key {
			continue
		}
		out := map[string]string{}
		for _, c := range columns {
			out[c] = row[index[c]]
		}
		return out
	}
	t.Fatalf("%s: no row with %s=%s", path, keyColumn, key)
	return nil
}

// A 403 from the Auth0 token endpoint is a credentials problem, never "unregistered".
func TestTokenForbiddenIsALivenessError(t *testing.T) {
	fx := newFixture()
	fx.company("c-live", "Live Corp", "", liveSFID, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.tokenDenied = true
	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Apply: true, RegisterUnregistered: true})
	require.NoError(t, err)
	g := groupByKey(plan.Groups, liveSFID)
	require.NotNil(t, g)
	assert.Equal(t, LiveError, g.Live)
	assert.Empty(t, g.Route)
	require.Error(t, g.Err)
	assert.NotContains(t, g.Err.Error(), "for no Account")
	assert.Empty(t, plan.Register)
	assert.Zero(t, plan.Unregistered())
	sum, err := Execute(context.Background(), fx.deps(), Options{Apply: true, RegisterUnregistered: true}, plan)
	assert.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.Empty(t, fx.platform.registered)
}

// suggested_account: org-service lookups by domain and by name fill in after the inventory; an
// unverifiable candidate (403) is kept with a "?" tag and a lookup failure only warns.
func TestSuggestedAccountFromCRM(t *testing.T) {
	const (
		oldSFID  = "0014100000OldAAAAA"
		crmSFID  = "0014100000CrmAAAAA"
		crmSFID2 = "0014100000CrmBBBBB"
	)
	fx := newFixture()
	fx.company("c-old", "Acme", "", oldSFID, "cg-1")
	fx.company("c-live", "Live Corp", "", liveSFID, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[crmSFID] = true
	fx.platform.forbidden = map[string]int{crmSFID2: -1}
	fx.platform.orgs[oldSFID] = &Org{ID: oldSFID, Name: "Acme", Website: "https://www.old.example/about"}
	fx.platform.orgs[liveSFID] = &Org{ID: liveSFID, Name: "Live Corp", Website: "https://live.example"}
	fx.platform.lookups = map[string]*Org{
		"domain:old.example":  {ID: crmSFID, Name: "Old Example Inc", Website: "https://old.example"},
		"name:Acme":           {ID: crmSFID2, Name: "Acme Holdings"},
		"domain:live.example": {ID: liveSFID, Name: "Live Corp"},
	}

	dir := t.TempDir()
	_, err := Audit(context.Background(), fx.deps(), Options{Stage: "dev", OutDir: dir})
	require.NoError(t, err)
	rows := readCSV(t, filepath.Join(dir, "audit.csv"))
	byID := map[string][]string{}
	for _, r := range rows[1:] {
		byID[r[0]] = r
	}
	assert.Equal(t, crmSFID+" Old Example Inc [crm:domain]; "+crmSFID2+" Acme Holdings [crm:name?]", byID["c-old"][15])
	assert.Equal(t, "", byID["c-live"][15], "a live, registered Account gets no suggestion")
	assert.Contains(t, fx.platform.lookupCalls, "domain:old.example")
	assert.Contains(t, fx.platform.lookupCalls, "name:Acme")
	assert.NotContains(t, fx.platform.lookupCalls, "domain:live.example")
	assert.NotContains(t, fx.out.String(), "WARNING: account lookup")

	fx.out.Reset()
	dir = t.TempDir()
	opts := Options{Stage: "dev", OutDir: dir}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	_, err = Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	toSF := readCSV(t, filepath.Join(dir, "to_salesforce.csv"))
	require.Len(t, toSF, 2)
	assert.Equal(t, "suggested_account", toSF[0][len(toSF[0])-1])
	assert.Equal(t, oldSFID, toSF[1][0])
	assert.Equal(t, crmSFID+" Old Example Inc [crm:domain]; "+crmSFID2+" Acme Holdings [crm:name?]", toSF[1][len(toSF[1])-1])
	manual := readCSV(t, filepath.Join(dir, "manual_actions.csv"))
	require.NotEmpty(t, manual)
	assert.Equal(t, "suggested_account", manual[0][len(manual[0])-1])

	fx = newFixture()
	fx.company("c-old", "Acme", "", oldSFID, "cg-1")
	fx.platform.orgs[oldSFID] = &Org{ID: oldSFID, Name: "Acme", Website: "https://old.example"}
	fx.platform.lookupErr = errors.New("org-service 500")
	dir = t.TempDir()
	_, err = Audit(context.Background(), fx.deps(), Options{Stage: "dev", OutDir: dir})
	require.NoError(t, err)
	rows = readCSV(t, filepath.Join(dir, "audit.csv"))
	require.Len(t, rows, 2)
	assert.Equal(t, "", rows[1][15])
	assert.Equal(t, 2, strings.Count(fx.out.String(), "WARNING: account lookup"), "one warning per lookup key (domain, name)")
}

type eclaActivity struct {
	latest map[string]string
	fail   map[string]bool
}

func (f eclaActivity) CountECLAs(context.Context, string) (int, error) { return 0, nil }

func (f eclaActivity) LatestECLA(_ context.Context, id string) (string, error) {
	if f.fail[id] {
		return "", errors.New("dynamo down")
	}
	return f.latest[id], nil
}

// last_cla_activity: the newest of the active CCLA signing dates and of the employee acknowledgements of a company row.
func TestAuditLastCLAActivity(t *testing.T) {
	fx := newFixture()
	fx.company("c-ecla", "Acme", "", liveSFID, "cg-1")
	fx.company("c-ccla", "Beta", "", liveSFID2, "cg-1", "cg-2")
	fx.company("c-err", "Gamma", "", lfID, "cg-1")
	fx.company("c-inactive", "Delta", "", "lf-inactive")
	fx.company("c-docusign", "Epsilon", "", "lf-docusign", "cg-1")
	fx.sigs.items[2].SignedOn = "2025-06-07T08:09:10Z"
	fx.sigs.items[4].SignedOn, fx.sigs.items[4].DateCreated, fx.sigs.items[4].DateModified = "", "2022-03-04T05:06:07Z", "2026-01-01T00:00:00Z"
	fx.sigs.items[4].UserDocusignDateSigned = "2022-03-09T23:30:00.000000-0700"
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[liveSFID2] = true
	deps := fx.deps()
	deps.ECLAs = eclaActivity{latest: map[string]string{"c-ecla": "2026-05-03", "c-ccla": "2023-01-01"}, fail: map[string]bool{"c-err": true}}
	dir := t.TempDir()
	_, err := Audit(context.Background(), deps, Options{Stage: "dev", OutDir: dir})
	require.NoError(t, err)
	rows := readCSV(t, filepath.Join(dir, "audit.csv"))
	require.Equal(t, "last_cla_activity", rows[0][len(rows[0])-1])
	last := map[string]string{}
	for _, r := range rows[1:] {
		last[r[0]] = r[len(r)-1]
	}
	assert.Equal(t, map[string]string{"c-ecla": "2026-05-03", "c-ccla": "2025-06-07", "c-err": "err", "c-inactive": "", "c-docusign": "2022-03-10"}, last,
		"a CCLA without signed_on counts by its DocuSign completion, as a UTC day")

	deps.ECLAs = eclaCounterFunc(func(context.Context, string) (int, error) { return 0, nil })
	dir = t.TempDir()
	_, err = Audit(context.Background(), deps, Options{Stage: "dev", OutDir: dir})
	require.NoError(t, err)
	for _, r := range readCSV(t, filepath.Join(dir, "audit.csv"))[1:] {
		if r[0] == "c-ecla" {
			assert.Equal(t, "2024-01-02", r[len(r)-1], "without the acknowledgement dates the CCLA date stands")
		}
	}
}

type eclaQueryDB struct {
	dynamodbiface.DynamoDBAPI
	inputs []dynamodb.QueryInput
	pages  []*dynamodb.QueryOutput
}

func (d *eclaQueryDB) QueryWithContext(_ aws.Context, in *dynamodb.QueryInput, _ ...request.Option) (*dynamodb.QueryOutput, error) {
	d.inputs = append(d.inputs, *in)
	return d.pages[len(d.inputs)-1], nil
}

// LatestECLA dates every signed row the way my_clas does and returns the newest UTC day over all pages.
func TestLatestECLAReadsSignedRowsInTheMyCLAsDateOrder(t *testing.T) {
	s := func(v string) *dynamodb.AttributeValue { return &dynamodb.AttributeValue{S: aws.String(v)} }
	db := &eclaQueryDB{pages: []*dynamodb.QueryOutput{
		{Items: []map[string]*dynamodb.AttributeValue{
			{"signed_on": s("2023-04-05T06:07:08Z"), "date_created": s("2020-01-01T00:00:00Z"), "date_modified": s("2026-11-01T00:00:00Z")},
			{"date_modified": s("2024-08-09T23:30:00.000000-0700")},
		}, LastEvaluatedKey: map[string]*dynamodb.AttributeValue{"signature_id": s("s-2")}},
		{Items: []map[string]*dynamodb.AttributeValue{
			{"user_docusign_date_signed": s("2022-02-03T04:05:06Z"), "date_created": s("2021-01-01T00:00:00Z")},
			{},
		}},
	}}
	got, err := DynamoECLACounter{DB: db, Table: "cla-dev-signatures"}.LatestECLA(context.Background(), "c-1")
	require.NoError(t, err)
	assert.Equal(t, "2024-08-10", got, "a row without date_created counts by its next date, a later date_modified of a dated row does not")
	require.Len(t, db.inputs, 2)
	in := db.inputs[0]
	assert.Equal(t, "signature_signed = :signed", aws.StringValue(in.FilterExpression))
	require.NotNil(t, in.ExpressionAttributeValues[":signed"])
	assert.True(t, aws.BoolValue(in.ExpressionAttributeValues[":signed"].BOOL))
	assert.Equal(t, "c-1", aws.StringValue(in.ExpressionAttributeValues[":id"].S))
	assert.Equal(t, "signed_on, user_docusign_date_signed, date_created, approx_date_created, date_modified", aws.StringValue(in.ProjectionExpression))
	assert.Empty(t, in.ExclusiveStartKey)
	assert.Equal(t, db.pages[0].LastEvaluatedKey, db.inputs[1].ExclusiveStartKey)
}

// acs_roles: a per-org ACS failure is reported in the cell, not as an audit error.
func TestAuditACSRolesError(t *testing.T) {
	fx := newFixture()
	fx.company("c-live", "Live Corp", "", liveSFID, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.orgs[liveSFID] = &Org{ID: liveSFID, Name: "Live Corp", Website: "https://live.example"}
	fx.platform.grantsErr = map[string]error{liveSFID: errors.New("acs down")}
	dir := t.TempDir()
	_, err := Audit(context.Background(), fx.deps(), Options{Stage: "dev", OutDir: dir})
	require.NoError(t, err)
	rows := readCSV(t, filepath.Join(dir, "audit.csv"))
	require.Len(t, rows, 2)
	assert.Equal(t, "err", rows[1][14])
}

func TestPlanSuggestedAccountFromInventory(t *testing.T) {
	const (
		oldSFID  = "0014100000OldAAAAA"
		old2SFID = "0014100000OldBBBBB"
		newSFID  = "0014100000NewCCCCC"
	)
	fx := newFixture()
	fx.company("c-old", "Acme", "", oldSFID, "cg-1")
	fx.company("c-old2", "Widgets", "", old2SFID, "cg-1")
	fx.company("c-new", "Acme", "", newSFID, "cg-1")
	fx.company("c-lf", "Acme", "", "lf-acme-legacy-01", "cg-1")
	fx.company("c-unrelated", "Zeta", "", liveSFID2, "cg-1")
	fx.platform.sfAccounts[newSFID] = true
	fx.platform.sfAccounts[liveSFID2] = true
	fx.platform.orgs[newSFID] = &Org{ID: newSFID, Name: "Acme Inc.", Website: "https://www.acme.example"}
	fx.platform.orgs[old2SFID] = &Org{ID: old2SFID, Name: "Widgets", Website: "https://labs.acme.example/x"}
	fx.platform.orgs[liveSFID2] = &Org{ID: liveSFID2, Name: "Zeta", Website: "https://zeta.example"}

	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev"})
	require.NoError(t, err)
	live := groupByKey(plan.Groups, newSFID)
	require.NotNil(t, live)
	assert.Equal(t, LiveLive, live.Live)
	assert.Equal(t, "200", live.OrgStatus)
	assert.Equal(t, "", live.Suggested, "a live Account never suggests itself")
	assert.Equal(t, newSFID+" Acme Inc. [inventory:name]", groupByKey(plan.Groups, oldSFID).Suggested)
	assert.Equal(t, newSFID+" Acme Inc. [inventory:domain]", groupByKey(plan.Groups, old2SFID).Suggested)
	assert.Equal(t, newSFID+" Acme Inc. [inventory:name]", groupByKey(plan.Groups, "lf-acme-legacy-01").Suggested)
	assert.Equal(t, "", groupByKey(plan.Groups, liveSFID2).Suggested)
	assert.Empty(t, fx.platform.registered)
}
