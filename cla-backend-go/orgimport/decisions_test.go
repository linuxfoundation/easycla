// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeDecisions(t *testing.T, dir string, rows ...string) string {
	t.Helper()
	path := filepath.Join(dir, "decisions.csv")
	require.NoError(t, os.WriteFile(path, []byte("decision,old_ids,target_sfid,reviewer,note\n"+strings.Join(rows, "\n")+"\n"), 0o600))
	return path
}

func TestParseDecisions(t *testing.T) {
	d, err := ParseDecisions(strings.NewReader("decision,old_ids,target_sfid,reviewer,note\n" +
		"collapse,lf-a; lf-b," + targetSFID + ",michal,same legal entity\n" +
		"DISTINCT,lf-c;lf-d,,michal,\n"))
	require.NoError(t, err)
	require.Len(t, d.List, 2)
	assert.Equal(t, []string{"lf-a", "lf-b"}, d.List[0].OldIDs)
	assert.NotNil(t, d.Collapse("lf-a", targetSFID))
	assert.Nil(t, d.Collapse("lf-a", targetSFID2), "collapse is bound to its target")
	assert.Nil(t, d.Collapse("lf-c", targetSFID))
	assert.NotNil(t, d.Distinct("lf-c", []string{"lf-d", "lf-x"}))
	assert.Nil(t, d.Distinct("lf-c", []string{"lf-a"}), "distinct only separates ids of the same decision")
	assert.Nil(t, d.Distinct("lf-a", []string{"lf-b"}))
	assert.Nil(t, (*Decisions)(nil).Collapse("lf-a", targetSFID))
	assert.Nil(t, (*Decisions)(nil).Distinct("lf-a", nil))
	spaced, err := ParseDecisions(strings.NewReader("decision,old_ids,target_sfid,reviewer\ncollapse,lf-a lf-b\t" + targetSFID + "," + targetSFID + ",michal\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"lf-a", "lf-b", targetSFID}, spaced.List[0].OldIDs, "old ids may be separated by spaces (workflow input rows use '|')")

	for _, bad := range []string{
		"",
		"decision,old_ids,target_sfid\ncollapse,lf-a," + targetSFID,
		"decision,old_ids,target_sfid,reviewer\ncollapse,lf-a," + targetSFID + ",",
		"decision,old_ids,target_sfid,reviewer\ncollapse,lf-a,not-an-sfid,michal",
		"decision,old_ids,target_sfid,reviewer\ncollapse,," + targetSFID + ",michal",
		"decision,old_ids,target_sfid,reviewer\ndistinct,lf-a,,michal",
		"decision,old_ids,target_sfid,reviewer\ndistinct,lf-a;lf-b," + targetSFID + ",michal",
		"decision,old_ids,target_sfid,reviewer\nmerge,lf-a;lf-b," + targetSFID + ",michal",
		"decision,old_ids,target_sfid,reviewer\ncollapse,lf-a," + targetSFID + ",michal\ndistinct,lf-a;lf-b,,michal",
	} {
		_, err = ParseDecisions(strings.NewReader(bad))
		assert.Error(t, err, bad)
	}
	_, err = LoadDecisions(filepath.Join(t.TempDir(), "missing.csv"))
	assert.Error(t, err)
}

func TestSharedDomains(t *testing.T) {
	def, err := LoadSharedDomains("")
	require.NoError(t, err)
	assert.True(t, def["github.com"])
	assert.True(t, def["nowebsite.com"])
	assert.Equal(t, "legacy.example", Domain("https://www.legacy.example/about?x=1"))
	assert.Equal(t, "legacy.example", Domain("Legacy.Example"))
	assert.Equal(t, "github.com", Domain("github.com/some-org"))
	assert.Equal(t, "", Domain("  "))
	d, reason := def.Shared("https://github.com/acme")
	assert.Equal(t, "github.com", d)
	assert.Equal(t, "shared_domain", reason)
	_, reason = def.Shared("")
	assert.Equal(t, "missing_website", reason)
	d, reason = def.Shared("https://acme.example")
	assert.Equal(t, "acme.example", d)
	assert.Empty(t, reason)

	// registrable domain (public suffix list): hosts collapse to their registrable parent, private
	// suffixes and unparsable hosts stay as they are
	assert.Equal(t, "google.com", Domain("https://startup.google.com/x"))
	assert.Equal(t, "ibm.com", Domain("research.ibm.com"))
	assert.Equal(t, "harvard.edu", Domain("https://rc.fas.harvard.edu"))
	assert.Equal(t, "comcast.github.io", Domain("https://comcast.github.io"))
	assert.Equal(t, "economie.gouv.fr", Domain("economie.gouv.fr"))
	assert.Equal(t, "localhost", Domain("localhost"))
	assert.Equal(t, "127.0.0.1", Domain("http://127.0.0.1:8080/"))
	assert.Equal(t, "acme.example", Domain("https://labs.acme.example"))
	for _, want := range []string{"en.wikipedia.org", "buymeacoffee.com", "nonameaccount.com", "localhost.localhost", "bund.de", "onmicrosoft.com"} {
		assert.True(t, def[want], want)
	}
	// a listed shared parent keeps its subdomains apart (digitalservice.bund.de is not bund.de)
	d, reason = def.Shared("https://digitalservice.bund.de")
	assert.Equal(t, "digitalservice.bund.de", d)
	assert.Empty(t, reason)
	d, reason = def.Shared("bund.de")
	assert.Equal(t, "bund.de", d)
	assert.Equal(t, "shared_domain", reason)
	d, reason = def.Shared("https://mainh.onmicrosoft.com")
	assert.Equal(t, "mainh.onmicrosoft.com", d)
	assert.Empty(t, reason)
	d, reason = def.Shared("https://en.wikipedia.org/wiki/x")
	assert.Equal(t, "en.wikipedia.org", d)
	assert.Equal(t, "shared_domain", reason)
	d, reason = def.Shared("https://de.wikipedia.org")
	assert.Equal(t, "wikipedia.org", d)
	assert.Empty(t, reason)
	d, reason = def.Shared("https://hr.163.com")
	assert.Equal(t, "hr.163.com", d, "a subdomain of a listed parent is its own non-shared key")
	assert.Empty(t, reason)
	_, reason = def.Shared("localhost.localhost")
	assert.Equal(t, "shared_domain", reason)

	path := filepath.Join(t.TempDir(), "shared.txt")
	require.NoError(t, os.WriteFile(path, []byte("# review list\nWWW.Example.ORG # comment\nhttps://foo.test/\n\n"), 0o600))
	custom, err := LoadSharedDomains(path)
	require.NoError(t, err)
	assert.Equal(t, SharedDomains{"example.org": true, "foo.test": true}, custom)
	assert.False(t, custom["github.com"], "a review list replaces the built-in list")
	_, err = LoadSharedDomains(filepath.Join(t.TempDir(), "missing"))
	assert.Error(t, err)
}

func collisionFixture(t *testing.T) (*fixture, string) {
	t.Helper()
	fx := newFixture()
	fx.company("c-a", "Acme Inc", "", "lf-a", "cg-1")
	fx.company("c-b", "Acme GmbH", "", "lf-b", "cg-1")
	fx.company("c-c", "Acme Labs", "", "lf-c", "cg-2")
	fx.platform.sfAccounts[targetSFID] = true
	fx.platform.orgs[targetSFID] = &Org{ID: targetSFID, Name: "Acme"}
	fx.platform.orgs["lf-a"] = &Org{ID: "lf-a", Name: "Acme Inc", Website: "https://acme.example"}
	fx.platform.orgs["lf-b"] = &Org{ID: "lf-b", Name: "Acme GmbH", Website: "https://acme.example/de"}
	fx.platform.orgs["lf-c"] = &Org{ID: "lf-c", Name: "Acme Labs", Website: "https://labs.acme.example"}
	dir := t.TempDir()
	return fx, dir
}

func TestCollisionNeedsDecision(t *testing.T) {
	fx, dir := collisionFixture(t)
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",matched,true", "lf-b,"+targetSFID+",created,true", "lf-c,"+targetSFID2+",created,true")
	opts := Options{Stage: "dev", Mapping: mapping, OutDir: filepath.Join(dir, "out")}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	for _, id := range []string{"lf-a", "lf-b"} {
		g := groupByKey(plan.Groups, id)
		assert.Equal(t, RouteManual, g.Route, id)
		assert.Equal(t, "target_collision", g.ManualReason, id)
		assert.Equal(t, targetSFID, g.NewID, id, "the resolved target stays on the group for the report")
	}
	c := groupByKey(plan.Groups, "lf-c")
	assert.Equal(t, RouteRewrite, c.Route)
	assert.Equal(t, targetSFID2, c.NewID)
	require.Len(t, plan.Targets, 2)
	tgt := targetByID(plan.Targets, targetSFID)
	assert.Equal(t, "needs_decision", tgt.Status)
	assert.Equal(t, []string{"lf-a", "lf-b"}, tgt.OldIDs())
	assert.Equal(t, "ok", targetByID(plan.Targets, targetSFID2).Status)
	assert.Empty(t, plan.Rewrite[0].ManualReason)
	require.Len(t, plan.Rewrite, 1)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 2, sum.Manual)
	targets := readCSV(t, filepath.Join(dir, "out", "targets.csv"))
	require.Len(t, targets, 3)
	assert.Equal(t, []string{targetSFID, "2", "lf-a;lf-b", "c-a;c-b", "Acme Inc;Acme GmbH", "0", "", "", "needs_decision"}, targets[1])
	assert.Equal(t, targetSFID2, targets[2][0])
	assert.Equal(t, "ok", targets[2][8])
}

func TestCollisionCollapseDecision(t *testing.T) {
	fx, dir := collisionFixture(t)
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",matched,true", "lf-b,"+targetSFID+",created,true", "lf-c,"+targetSFID2+",created,true")
	decisions := writeDecisions(t, dir, "collapse,lf-a;lf-b,"+targetSFID+",michal,one legal entity")
	opts := Options{Stage: "dev", Mapping: mapping, Decisions: decisions, OutDir: filepath.Join(dir, "out")}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 3)
	for _, id := range []string{"lf-a", "lf-b"} {
		g := groupByKey(plan.Groups, id)
		assert.Equal(t, RouteRewrite, g.Route, id)
		assert.Empty(t, g.ManualReason, id)
		assert.Equal(t, targetSFID, g.NewID, id)
		require.NotNil(t, g.Decision, id)
		assert.Equal(t, "michal", g.Decision.Reviewer)
	}
	tgt := targetByID(plan.Targets, targetSFID)
	assert.Equal(t, "ok", tgt.Status)
	assert.Equal(t, DecisionCollapse, tgt.Decision.Kind)

	// a collapse recorded for another target does not approve this one
	other := writeDecisions(t, dir, "collapse,lf-a;lf-b,"+targetSFID2+",michal,")
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping, Decisions: other})
	require.NoError(t, err)
	assert.Equal(t, "target_collision", groupByKey(plan.Groups, "lf-a").ManualReason)
	assert.Equal(t, "target_collision", groupByKey(plan.Groups, "lf-b").ManualReason)

	// a collapse covering only one of the ids approves only that id
	partial := writeDecisions(t, dir, "collapse,lf-a,"+targetSFID+",michal,")
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping, Decisions: partial})
	require.NoError(t, err)
	assert.Equal(t, RouteRewrite, groupByKey(plan.Groups, "lf-a").Route)
	assert.Equal(t, "target_collision", groupByKey(plan.Groups, "lf-b").ManualReason)
	assert.Equal(t, "needs_decision", targetByID(plan.Targets, targetSFID).Status)

	// dry run of the approved collapse writes the decision into the plan report
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Manual)
	rows := readCSV(t, filepath.Join(dir, "out", "plan.csv"))
	byKey := map[string][]string{}
	for _, r := range rows[1:] {
		byKey[r[0]] = r
	}
	assert.Equal(t, "collapse", byKey["lf-a"][12])
	assert.Equal(t, "michal", byKey["lf-a"][13])
	assert.Equal(t, "acme.example", byKey["lf-a"][8])
	assert.Equal(t, "false", byKey["lf-a"][9])
	var buf bytes.Buffer
	plan.Print(&buf)
	assert.Contains(t, buf.String(), targetSFID+" status=needs_decision old_ids=lf-a;lf-b")
}

func TestCollisionDistinctDecision(t *testing.T) {
	fx, dir := collisionFixture(t)
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",matched,true", "lf-b,"+targetSFID+",created,true")
	decisions := writeDecisions(t, dir, "distinct,lf-a;lf-b,,michal,separate entities")
	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping, Decisions: decisions})
	require.NoError(t, err)
	for _, id := range []string{"lf-a", "lf-b"} {
		g := groupByKey(plan.Groups, id)
		assert.Equal(t, RouteManual, g.Route, id)
		assert.Equal(t, "distinct_conflict", g.ManualReason, id)
	}
	assert.Equal(t, "distinct_conflict", targetByID(plan.Targets, targetSFID).Status)
	assert.Contains(t, Suggest("distinct_conflict"), "distinct decision")

	// distinct ids landing on different accounts are fine
	mapping = writeMapping(t, dir, "lf-a,"+targetSFID+",matched,true", "lf-b,"+targetSFID2+",created,true")
	fx.platform.orgs[targetSFID2] = &Org{ID: targetSFID2}
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping, Decisions: decisions})
	require.NoError(t, err)
	assert.Equal(t, RouteRewrite, groupByKey(plan.Groups, "lf-a").Route)
	assert.Equal(t, RouteRewrite, groupByKey(plan.Groups, "lf-b").Route)
}

func TestCollisionWithExistingRowsAndMappingCoTargets(t *testing.T) {
	fx, dir := collisionFixture(t)
	fx.company("c-existing", "Acme Holdings", "", targetSFID)
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",matched,true")
	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping})
	require.NoError(t, err)
	a := groupByKey(plan.Groups, "lf-a")
	assert.Equal(t, "target_collision", a.ManualReason, "rows already carrying the target need a collapse decision")
	tgt := targetByID(plan.Targets, targetSFID)
	assert.Equal(t, []string{"lf-a", targetSFID}, tgt.OldIDs())
	require.Len(t, tgt.Existing, 1)

	decisions := writeDecisions(t, dir, "collapse,lf-a;"+targetSFID+","+targetSFID+",michal,")
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping, Decisions: decisions})
	require.NoError(t, err)
	assert.Equal(t, RouteRewrite, groupByKey(plan.Groups, "lf-a").Route)

	// a mapping row outside the tranche landing on the same target is a co-target too
	fx, dir = collisionFixture(t)
	mapping = writeMapping(t, dir, "lf-a,"+targetSFID+",matched,true", "lf-z,"+targetSFID+",created,true")
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping})
	require.NoError(t, err)
	assert.Equal(t, "target_collision", groupByKey(plan.Groups, "lf-a").ManualReason)
	assert.Equal(t, []string{"lf-a", "lf-z"}, targetByID(plan.Targets, targetSFID).OldIDs())
	decisions = writeDecisions(t, dir, "collapse,lf-a;lf-z,"+targetSFID+",michal,")
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping, Decisions: decisions})
	require.NoError(t, err)
	assert.Equal(t, RouteRewrite, groupByKey(plan.Groups, "lf-a").Route)
}

func TestReplayedGroupSkipsDecisionPass(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.company("c-existing", "Legacy Holdings", "", targetSFID)
	require.NoError(t, os.WriteFile(statePath, []byte(`{"ts":"2026-09-29T11:00:00Z","old_id":"`+lfID+`","new_id":"`+targetSFID+`","company_ids":["c-parent","c-sub"],"step":"rows","status":"ok"}`+"\n"), 0o600))
	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Mapping: mapping, State: statePath})
	require.NoError(t, err)
	g := groupByKey(plan.Groups, lfID)
	require.NotNil(t, g)
	assert.True(t, g.Replayed)
	assert.Equal(t, RouteRewrite, g.Route, "an unfinished rewrite resumes regardless of collisions")
	assert.Empty(t, g.ManualReason)
}

const legacyGitHubSite = "https://github.com/legacy-ltd"

func TestApexSharedDomainGate(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":%q,"action":%q}`, targetSFID, ActionCreated)
	}))
	defer srv.Close()
	apex, err := NewApexClient(srv.URL, "apex-token")
	require.NoError(t, err)

	fx, mapping, _ := rewriteFixture(t)
	fx.platform.orgs[lfID].Website = legacyGitHubSite
	deps := fx.deps()
	deps.Apex = apex
	dir := t.TempDir()
	opts := Options{Stage: "dev", UseApex: true, OutDir: dir}
	plan, err := BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)
	g := groupByKey(plan.Groups, lfID)
	assert.Equal(t, RouteManual, g.Route)
	assert.Equal(t, "shared_domain", g.ManualReason)
	assert.Equal(t, 0, calls, "shared domains never reach Apex")
	_, err = Execute(context.Background(), deps, opts, plan)
	require.NoError(t, err)
	rows := readCSV(t, filepath.Join(dir, "plan.csv"))
	var lfRow []string
	for _, r := range rows[1:] {
		if r[0] == lfID {
			lfRow = r
		}
	}
	require.NotNil(t, lfRow)
	assert.Equal(t, legacyGitHubSite, lfRow[7])
	assert.Equal(t, "github.com", lfRow[8])
	assert.Equal(t, "true", lfRow[9])
	manual := readCSV(t, filepath.Join(dir, "manual_actions.csv"))
	require.Len(t, manual, 2)
	assert.Equal(t, "shared_domain", manual[1][3])
	assert.Contains(t, manual[1][4], "never matched automatically")

	fx.platform.orgs[lfID].Website = ""
	plan, err = BuildPlan(context.Background(), deps, Options{UseApex: true})
	require.NoError(t, err)
	assert.Equal(t, "missing_website", groupByKey(plan.Groups, lfID).ManualReason)
	assert.Equal(t, 0, calls)

	// an approved mapping row is the operator's resolution and bypasses the gate and Apex
	plan, err = BuildPlan(context.Background(), deps, Options{UseApex: true, Mapping: mapping})
	require.NoError(t, err)
	assert.Equal(t, RouteRewrite, groupByKey(plan.Groups, lfID).Route)
	assert.Equal(t, targetSFID, groupByKey(plan.Groups, lfID).NewID)
	assert.Equal(t, 0, calls)

	// a review list replaces the built-in list
	fx.platform.orgs[lfID].Website = legacyGitHubSite
	shared := filepath.Join(dir, "shared.txt")
	require.NoError(t, os.WriteFile(shared, []byte("nowebsite.com\n"), 0o600))
	plan, err = BuildPlan(context.Background(), deps, Options{UseApex: true, SharedDomains: shared})
	require.NoError(t, err)
	assert.Equal(t, RouteRewrite, groupByKey(plan.Groups, lfID).Route)
	assert.Equal(t, 1, calls)
	_, err = BuildPlan(context.Background(), deps, Options{UseApex: true, SharedDomains: filepath.Join(dir, "missing")})
	assert.Error(t, err)
	_, err = BuildPlan(context.Background(), deps, Options{Decisions: filepath.Join(dir, "missing")})
	assert.Error(t, err)
}

func TestManualActionsAndSuggestions(t *testing.T) {
	fx := newFixture()
	fx.company("c-empty", "Empty Inc", "", "", "cg-1")
	fx.company("c-lf", "Legacy Ltd", "", lfID, "cg-1")
	fx.company("c-dead", "Dead Corp", "", deadSFID, "cg-1")
	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev"})
	require.NoError(t, err)
	actions := plan.ManualActions()
	require.Len(t, actions, 3)
	reasons := map[string]string{}
	for _, a := range actions {
		reasons[a.Group.Key] = a.Reason
		assert.NotEmpty(t, a.Suggested)
		assert.Contains(t, a.String(), a.Reason)
	}
	assert.Equal(t, map[string]string{"c-empty": "empty_external_id", lfID: "no_mapping", deadSFID: "no_mapping"}, reasons)
	assert.Contains(t, Suggest("empty_external_id"), "m3-org-cleanup.md")
	assert.Contains(t, Suggest("error: boom"), "re-run")
	assert.Contains(t, SuggestError(errors.New("liveness check failed for 001: member-service: authorization failed (403): Client \"x\" is not authorized to access resource server \"y\". You need to create a \"client-grant\" associated to this API.")), "client grant")
	assert.Contains(t, SuggestError(errors.New("member-service: authorization failed (403): forbidden")), "Heimdall")
	assert.Contains(t, SuggestError(ErrNotConfigured), "cla-member-service-base-url")
	assert.Contains(t, SuggestError(errors.New("boom")), "re-run")
	assert.Contains(t, Suggest("something_new"), "run.log")

	// a failed group is listed with its error
	fx.platform.failGetB2B = true
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev"})
	require.NoError(t, err)
	var failed *ManualAction
	for _, a := range plan.ManualActions() {
		if a.Group.Key == deadSFID {
			cp := a
			failed = &cp
		}
	}
	require.NotNil(t, failed)
	assert.Equal(t, "error", failed.Reason)
	assert.Contains(t, failed.String(), "liveness check failed")
}
