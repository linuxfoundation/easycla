// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMemberAccounts struct {
	calls  [][2]string
	answer func(name, website string) (*member_service.B2BOrg, member_service.Action, error)
}

func (f *fakeMemberAccounts) FindOrCreateB2BOrg(_ context.Context, name, website string) (*member_service.B2BOrg, member_service.Action, error) {
	f.calls = append(f.calls, [2]string{name, website})
	return f.answer(name, website)
}

// creating makes the fake platform serve the new Account, as Salesforce and the organization service do after a create
func creating(fx *fixture, sfid string) *fakeMemberAccounts {
	return &fakeMemberAccounts{answer: func(name, website string) (*member_service.B2BOrg, member_service.Action, error) {
		fx.platform.sfAccounts[sfid] = true
		fx.platform.orgs[sfid] = &Org{ID: sfid, Name: name, Website: website}
		return &member_service.B2BOrg{UID: sfid, Name: name}, member_service.ActionCreated, nil
	}}
}

func TestCreateRowCreatesTheAccountAtApply(t *testing.T) {
	fx, _, statePath := rewriteFixture(t)
	mapping := writeMapping(t, t.TempDir(), lfID+",,create,true")
	members := creating(fx, targetSFID2)
	deps := fx.deps()
	deps.Apex = MemberApex{Members: members}

	dry := Options{Stage: "dev", Mapping: mapping, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), deps, dry)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	g := plan.Rewrite[0]
	assert.True(t, g.ApexCreates())
	assert.False(t, g.Pending())
	assert.Empty(t, g.NewID)
	assert.Equal(t, ActionCreated, g.Action)
	assert.True(t, g.AllGrants, "the Account is created for this organization alone")
	var printed bytes.Buffer
	plan.Print(&printed)
	assert.Contains(t, printed.String(), "PLAN rewrite "+lfID+" -> <new Account, id assigned by Apex at apply> (created)")
	_, err = Execute(context.Background(), deps, dry, plan)
	require.NoError(t, err)
	assert.Empty(t, members.calls, "a dry run creates nothing")
	assert.Empty(t, fx.companies.updates)

	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err = BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)
	sum, err := Execute(context.Background(), deps, opts, plan)

	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Equal(t, [][2]string{{"Legacy Ltd", "https://legacy.example"}}, members.calls)
	assert.Equal(t, targetSFID2, fx.companies.rows["c-parent"].CompanyExternalID)
	assert.Equal(t, targetSFID2, fx.companies.rows["c-sub"].CompanyExternalID)
	assert.Empty(t, mustListOrgGrants(t, fx, lfID))
	var moved []string
	for _, gr := range mustListOrgGrants(t, fx, targetSFID2) {
		moved = append(moved, gr.Username+":"+gr.RoleName+":"+gr.ObjectID)
	}
	assert.ElementsMatch(t, []string{"alice:cla-manager:" + targetSFID2, "bob:cla-manager:proj-1|" + targetSFID2, "carol:cla-signatory:" + targetSFID2}, moved)
	assert.Contains(t, fx.platform.registered, targetSFID2)
	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.True(t, st.Done(lfID))
	assert.Equal(t, targetSFID2, st.Last[lfID].NewID)

	plan, err = BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)
	assert.Empty(t, plan.Rewrite)
	_, err = Execute(context.Background(), deps, opts, plan)
	require.NoError(t, err)
	assert.Len(t, members.calls, 1, "a finished organization is not created again")
}

func TestCreateRowResumesWithTheCreatedAccount(t *testing.T) {
	fx, _, statePath := rewriteFixture(t)
	mapping := writeMapping(t, t.TempDir(), lfID+",,create,true")
	members := creating(fx, targetSFID2)
	deps := fx.deps()
	deps.Apex = MemberApex{Members: members}
	fx.platform.failCreate = true
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)

	sum, err := Execute(context.Background(), deps, opts, plan)

	require.Error(t, err, "the copy of the roles fails after the Account was created")
	assert.Equal(t, 1, sum.Failed)
	assert.Empty(t, fx.companies.updates)

	fx.platform.failCreate = false
	plan, err = BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	assert.True(t, plan.Rewrite[0].Replayed)
	assert.Equal(t, targetSFID2, plan.Rewrite[0].NewID)
	sum, err = Execute(context.Background(), deps, opts, plan)

	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Len(t, members.calls, 1, "the journal holds the Account id: nothing is created twice")
	assert.Equal(t, targetSFID2, fx.companies.rows["c-parent"].CompanyExternalID)
}

func TestCreateRowStopsWhenAnAccountAlreadyMatches(t *testing.T) {
	fx, _, statePath := rewriteFixture(t)
	mapping := writeMapping(t, t.TempDir(), lfID+",,create,true")
	members := &fakeMemberAccounts{answer: func(name, _ string) (*member_service.B2BOrg, member_service.Action, error) {
		return &member_service.B2BOrg{UID: targetSFID, Name: name}, member_service.ActionMatched, nil
	}}
	deps := fx.deps()
	deps.Apex = MemberApex{Members: members}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)

	sum, err := Execute(context.Background(), deps, opts, plan)

	require.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.ErrorContains(t, plan.Rewrite[0].Err, "Account "+targetSFID+" already matches its website")
	assert.ErrorContains(t, plan.Rewrite[0].Err, lfID+","+targetSFID+",matched,true")
	assert.Empty(t, fx.companies.updates, "an Account nobody reviewed is never used")
	assert.Len(t, mustListOrgGrants(t, fx, lfID), 3)
	assert.Equal(t, []string{"alice:cla-manager:" + targetSFID}, targetGrants(t, fx))
	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.Equal(t, StepResolve, st.Last[lfID].Step)
	assert.Equal(t, statusFailed, st.Last[lfID].Status)

	plan, err = BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	assert.False(t, plan.Rewrite[0].Replayed, "a failed resolution is classified again from scratch")
	assert.Empty(t, plan.Rewrite[0].NewID)
}

func TestCreateRowStopsWhenSeveralAccountsShareTheDomain(t *testing.T) {
	fx, _, statePath := rewriteFixture(t)
	mapping := writeMapping(t, t.TempDir(), lfID+",,create,true")
	members := &fakeMemberAccounts{answer: func(string, string) (*member_service.B2BOrg, member_service.Action, error) {
		return nil, "", member_service.ErrAmbiguousDomain
	}}
	deps := fx.deps()
	deps.Apex = MemberApex{Members: members}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)

	sum, err := Execute(context.Background(), deps, opts, plan)

	require.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.ErrorContains(t, plan.Rewrite[0].Err, ActionAmbiguous)
	assert.Empty(t, fx.companies.updates)
}

func TestCreateRowNeedsApprovalAWebsiteAndMemberService(t *testing.T) {
	members := &fakeMemberAccounts{answer: func(string, string) (*member_service.B2BOrg, member_service.Action, error) {
		return nil, "", errors.New("must not be called")
	}}
	t.Run("not approved", func(t *testing.T) {
		fx, _, _ := rewriteFixture(t)
		deps := fx.deps()
		deps.Apex = MemberApex{Members: members}
		plan, err := BuildPlan(context.Background(), deps, Options{Stage: "dev", Mapping: writeMapping(t, t.TempDir(), lfID+",,create,false")})
		require.NoError(t, err)
		g := groupByKey(plan.Groups, lfID)
		assert.Equal(t, RouteManual, g.Route)
		assert.Equal(t, "mapping_not_approved", g.ManualReason)
		assert.Empty(t, plan.Rewrite)
	})
	t.Run("no website", func(t *testing.T) {
		fx, _, _ := rewriteFixture(t)
		fx.platform.orgs[lfID].Website = ""
		deps := fx.deps()
		deps.Apex = MemberApex{Members: members}
		plan, err := BuildPlan(context.Background(), deps, Options{Stage: "dev", Mapping: writeMapping(t, t.TempDir(), lfID+",,create,true")})
		require.NoError(t, err)
		g := groupByKey(plan.Groups, lfID)
		assert.Equal(t, RouteManual, g.Route)
		assert.Equal(t, ReasonMissingWebsite, g.ManualReason)
		assert.Empty(t, plan.Rewrite)
	})
	t.Run("row without an organization id", func(t *testing.T) {
		fx, _, _ := rewriteFixture(t)
		fx.company("c-empty", "No Id Ltd", "", "", "cg-1")
		deps := fx.deps()
		deps.Apex = MemberApex{Members: members}
		plan, err := BuildPlan(context.Background(), deps, Options{Stage: "dev", Mapping: writeMapping(t, t.TempDir(), "c-empty,,create,true")})
		require.NoError(t, err)
		g := groupByKey(plan.Groups, "c-empty")
		assert.Equal(t, RouteManual, g.Route)
		assert.Equal(t, ReasonMissingWebsite, g.ManualReason)
	})
	t.Run("no member-service", func(t *testing.T) {
		fx, _, _ := rewriteFixture(t)
		plan, err := BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: writeMapping(t, t.TempDir(), lfID+",,create,true")})
		require.NoError(t, err)
		g := groupByKey(plan.Groups, lfID)
		assert.ErrorIs(t, g.Err, ErrNotConfigured)
		assert.Empty(t, plan.Rewrite)
	})
	assert.Empty(t, members.calls)
}

func TestCreateRowRewritesAnAccountMemberServiceCannotRead(t *testing.T) {
	const unreadable = "0014100000GoneGone"
	build := func(t *testing.T, mappingRows ...string) *Group {
		fx, _, _ := rewriteFixture(t)
		fx.company("c-gone", "Gone Co", "", unreadable, "cg-1")
		fx.platform.orgs[unreadable] = &Org{ID: unreadable, Name: "Gone Co", Website: "https://gone.example"}
		fx.platform.forbidden = map[string]int{unreadable: -1}
		deps := fx.deps()
		deps.Apex = MemberApex{Members: creating(fx, targetSFID2)}
		plan, err := BuildPlan(context.Background(), deps, Options{Stage: "dev", Mapping: writeMapping(t, t.TempDir(), mappingRows...)})
		require.NoError(t, err)
		return groupByKey(plan.Groups, unreadable)
	}
	g := build(t, lfID+","+targetSFID+",matched,true")
	assert.Equal(t, RouteRegister, g.Route)
	assert.Equal(t, ReasonUnregistered, g.ManualReason, "without a row the Account stays where it is")

	g = build(t, lfID+","+targetSFID+",matched,true", unreadable+",,create,true")
	assert.Equal(t, RouteRewrite, g.Route)
	assert.True(t, g.ApexCreates(), "an approved create row is the reviewer's resolution")
	assert.Empty(t, g.ManualReason)
}

func TestParseMappingCreateRows(t *testing.T) {
	m, err := ParseMapping(strings.NewReader("old_id,new_id,action,approved\nlf-a,,create,true\nlf-b,,Create,false\nlf-c," + targetSFID + ",matched,true\n"))
	require.NoError(t, err)
	assert.True(t, m.Creates("lf-a"))
	assert.False(t, m.Creates("lf-b"), "a create row needs approval")
	assert.False(t, m.Creates("lf-c"))
	assert.False(t, m.Creates("lf-unknown"))
	var none *Mapping
	assert.False(t, none.Creates("lf-a"))
	assert.Empty(t, m.Targets[""], "a create row names no target")
	newID, action, reason := m.Resolve("lf-b")
	assert.Equal(t, []string{"", ActionCreate, "mapping_not_approved"}, []string{newID, action, reason})

	_, err = ParseMapping(strings.NewReader("old_id,new_id,action,approved\nlf-a," + targetSFID + ",create,true\n"))
	assert.ErrorContains(t, err, "action create takes no new_id")
}

func TestMemberApex(t *testing.T) {
	answer := func(org *member_service.B2BOrg, action member_service.Action, err error) MemberApex {
		return MemberApex{Members: &fakeMemberAccounts{answer: func(string, string) (*member_service.B2BOrg, member_service.Action, error) {
			return org, action, err
		}}}
	}
	req := ApexRequest{Name: "Acme", Website: "https://acme.example"}

	_, err := answer(nil, "", nil).FindOrCreate(context.Background(), ApexRequest{Name: "Acme", Website: "https://acme.example", DryRun: true})
	assert.ErrorContains(t, err, "no dry run")

	res, err := answer(&member_service.B2BOrg{UID: targetSFID}, member_service.ActionCreated, nil).FindOrCreate(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, ApexResult{ID: targetSFID, Action: ActionCreated}, *res)

	res, err = answer(&member_service.B2BOrg{UID: targetSFID}, member_service.ActionMatched, nil).FindOrCreate(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, ApexResult{ID: targetSFID, Action: ActionMatched}, *res)

	res, err = answer(nil, "", member_service.ErrAmbiguousDomain).FindOrCreate(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, ActionAmbiguous, res.Action)

	_, err = answer(nil, "", member_service.ErrInvalidRequest).FindOrCreate(context.Background(), req)
	assert.ErrorIs(t, err, member_service.ErrInvalidRequest)

	_, err = answer(&member_service.B2BOrg{UID: "lf-not-an-account"}, member_service.ActionCreated, nil).FindOrCreate(context.Background(), req)
	assert.ErrorContains(t, err, "unusable answer")
}
