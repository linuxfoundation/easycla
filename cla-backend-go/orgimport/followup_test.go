// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
