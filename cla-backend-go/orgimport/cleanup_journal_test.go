// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupJournalsAMovingGrantBeforeItsDelete(t *testing.T) {
	const mover = "frank"
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.oneOrg = true
	fx.platform.addGrant(lfID, mover, "role-contact", "")
	moving := grantIDOf(t, fx, lfID, mover, "contact")
	var atDelete StateRecord
	fx.platform.deleteHook = func(grantID string) error {
		if grantID == moving {
			st, err := LoadState(statePath)
			require.NoError(t, err)
			atDelete = st.Last[lfID]
		}
		return nil
	}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)

	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	var journaled []string
	for _, d := range atDelete.Dropped {
		journaled = append(journaled, d.GrantID)
		assert.Empty(t, d.Username)
	}
	assert.Contains(t, journaled, moving, "the journal lists the grant before it leaves the old id")
	assert.Contains(t, targetGrants(t, fx), mover+":contact:"+targetSFID)
	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.Equal(t, StepDone, st.Last[lfID].Step)
	assert.Empty(t, st.Last[lfID].Dropped)
	assert.Empty(t, droppedReport(t, opts.OutDir)[1:])
}

func TestReplayAfterAKillInsideTheCleanupReportsExactlyTheLostGrant(t *testing.T) {
	const late = "frank"
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.oneOrg = true
	added := false
	fx.platform.listHook = func(orgID string) {
		journal, readErr := os.ReadFile(filepath.Clean(statePath))
		if readErr != nil || orgID != lfID || added || !strings.Contains(string(journal), StepEvents) {
			return
		}
		added = true
		fx.platform.addGrantLocked(lfID, late, "role-contact", "")
	}
	fx.platform.createHook = func(username, orgID, _ string) error {
		if username != late || orgID != targetSFID {
			return nil
		}
		for _, g := range fx.platform.grants[lfID] {
			if g.Username == late {
				return nil
			}
		}
		panic("killed")
	}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)

	require.PanicsWithValue(t, "killed", func() {
		if _, runErr := Execute(context.Background(), fx.deps(), opts, plan); runErr != nil {
			t.Log(runErr)
		}
	})
	require.True(t, added, "the grant appeared after the copy step")
	require.Contains(t, targetGrants(t, fx), "carol:cla-signatory:"+targetSFID, "a grant that waited for its move reached the Account before the kill")
	for _, g := range mustListOrgGrants(t, fx, lfID) {
		require.NotEqual(t, late, g.Username, "the kill came after the delete on the old id")
	}

	fx.platform.listHook, fx.platform.createHook = nil, nil
	plan, err = BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	require.True(t, plan.Rewrite[0].Replayed)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)

	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Equal(t, [][]string{{lfID, targetSFID, "", "contact", objectTypeOrganization, lfID}}, droppedReport(t, opts.OutDir)[1:],
		"the grant that is on neither id is reported, the one that was moved before the kill is not")
}

func TestCollapseTriesARefusedOrganizationGrantAgainAfterTheOtherSources(t *testing.T) {
	for _, tc := range []struct {
		name, roleOnB string
		want          []string
	}{
		{"another role on the other source", "role-cla-signatory", []string{"uma:cla-signatory:" + targetSFID, "uma:contact:" + targetSFID}},
		{"the same role on the other source", "role-contact", []string{"uma:contact:" + targetSFID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx, dir := collisionFixture(t)
			fx.platform.oneOrg = true
			fx.platform.addGrant("lf-a", "uma", "role-contact", "")
			fx.platform.addGrant("lf-b", "uma", tc.roleOnB, "")
			mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",created,true", "lf-b,"+targetSFID+",created,true")
			decisions := writeDecisions(t, dir, "collapse,lf-a;lf-b,"+targetSFID+",michal,one legal entity")
			statePath := filepath.Join(dir, "state.jsonl")
			opts := Options{Stage: "dev", Apply: true, Mapping: mapping, Decisions: decisions, State: statePath, Routes: []Route{RouteRewrite}, OutDir: filepath.Join(dir, "out")}
			plan, err := BuildPlan(context.Background(), fx.deps(), opts)
			require.NoError(t, err)
			require.Len(t, plan.Rewrite, 2)

			sum, err := Execute(context.Background(), fx.deps(), opts, plan)

			require.NoError(t, err)
			assert.Equal(t, 2, sum.Rewritten)
			assert.Equal(t, tc.want, targetGrants(t, fx))
			assert.Empty(t, mustListOrgGrants(t, fx, "lf-a"))
			assert.Empty(t, mustListOrgGrants(t, fx, "lf-b"))
			assert.Empty(t, droppedReport(t, opts.OutDir)[1:], "a role the user holds on the Account after the run is not reported as dropped")
			assert.Contains(t, fx.out.String(), "grants lf-a -> "+targetSFID+": 1 copied after the other organizations of the run")
			st, err := LoadState(statePath)
			require.NoError(t, err)
			assert.Equal(t, StepDone, st.Last["lf-a"].Step)
			assert.Equal(t, statusOK, st.Last["lf-a"].Status)
			assert.Empty(t, st.Last["lf-a"].Dropped)
		})
	}
}

func TestCleanupJournalsALateUncopiedGrantBeforeItsDelete(t *testing.T) {
	const late = "grace"
	fx, mapping, statePath := rewriteFixture(t)
	lateGrant := ""
	fx.platform.listHook = func(orgID string) {
		journal, readErr := os.ReadFile(filepath.Clean(statePath))
		if readErr != nil || orgID != lfID || lateGrant != "" || !strings.Contains(string(journal), StepEvents) {
			return
		}
		fx.platform.addGrantLocked(lfID, late, "role-company-admin", "")
		for _, g := range fx.platform.grants[lfID] {
			if g.Username == late {
				lateGrant = g.GrantID
			}
		}
	}
	var atDelete StateRecord
	fx.platform.deleteHook = func(grantID string) error {
		if grantID == lateGrant {
			st, err := LoadState(statePath)
			require.NoError(t, err)
			atDelete = st.Last[lfID]
		}
		return nil
	}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)

	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	require.NotEmpty(t, lateGrant, "the role appeared after the copy step")
	assert.Equal(t, StepCleanup, atDelete.Step)
	assert.Equal(t, statusPartial, atDelete.Status)
	var journaled []string
	for _, d := range atDelete.Dropped {
		journaled = append(journaled, d.GrantID)
	}
	assert.Contains(t, journaled, lateGrant, "a role that is not copied is in the journal before it is deleted")
	assert.Equal(t, [][]string{{lfID, targetSFID, late, "company-admin", objectTypeOrganization, lfID}}, droppedReport(t, opts.OutDir)[1:])
}

func TestARetryThatCannotBeJournaledFailsTheGroup(t *testing.T) {
	fx, dir := collisionFixture(t)
	fx.platform.oneOrg = true
	fx.platform.addGrant("lf-a", "uma", "role-contact", "")
	fx.platform.addGrant("lf-b", "uma", "role-cla-signatory", "")
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",created,true", "lf-b,"+targetSFID+",created,true")
	decisions := writeDecisions(t, dir, "collapse,lf-a;lf-b,"+targetSFID+",michal,one legal entity")
	statePath := filepath.Join(dir, "state.jsonl")
	t.Cleanup(func() {
		if err := os.Chmod(statePath, 0o600); err != nil {
			t.Log(err)
		}
	})
	fx.platform.listHook = func(string) {
		st, err := LoadState(statePath)
		if err == nil && st.Done("lf-a") && st.Done("lf-b") {
			if chmodErr := os.Chmod(statePath, 0o400); chmodErr != nil {
				t.Error(chmodErr)
			}
		}
	}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, Decisions: decisions, State: statePath, Routes: []Route{RouteRewrite}, OutDir: filepath.Join(dir, "out")}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)

	require.Error(t, err, "the run must not report success with a wrong loss report")
	assert.Equal(t, 2, sum.Rewritten)
	assert.Equal(t, 1, sum.Failed)
	assert.ErrorContains(t, groupByKey(plan.Groups, "lf-a").Err, "the journal still lists them as dropped")
	assert.Contains(t, targetGrants(t, fx), "uma:contact:"+targetSFID, "the role itself was copied")
}
