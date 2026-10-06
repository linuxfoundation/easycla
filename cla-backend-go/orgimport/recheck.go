// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// recheckEvents revisits every previously completed rewrite group of the journal: the event listings
// of step 7b read eventually consistent indexes and a writer that read the company row before step 7a
// can still add events under the old id, so the old id is listed again (and, in apply mode, the
// stragglers re-keyed) on every rewrite-route run. Groups rewritten by this run are left to the next
// one (their step 7b just ran and the inventory of a replayed group is stale). Nothing is journaled,
// so the recheck repeats until it reports 0 listed. --ids and the tranche budget left after the
// planned groups bound the selection; a recorded row that is gone or no longer carries the recorded
// new id fails its recheck instead of re-keying events towards a stale destination. Returns the
// number of failed rechecks.
func (r *runner) recheckEvents(ctx context.Context, budget int) int {
	if r.deps.Events == nil || r.plan.state == nil || r.plan.inv == nil {
		return 0
	}
	want := map[string]bool{}
	for _, id := range r.opts.IDs {
		want[strings.TrimSpace(id)] = true
	}
	planned := map[string]bool{}
	for _, g := range r.plan.Rewrite {
		planned[g.Key] = true
	}
	var recs []StateRecord
	for _, rec := range r.plan.state.Last {
		if rec.Step != StepDone || rec.Status != statusOK || rec.Key != "" || rec.NewID == "" || rec.OldID == rec.NewID || planned[rec.OldID] {
			continue
		}
		if len(want) > 0 && !want[rec.OldID] && !want[rec.NewID] {
			continue
		}
		recs = append(recs, rec)
	}
	if len(recs) == 0 {
		return 0
	}
	sortRecords(recs)
	checked, listed, rekeyed, failed, skipped := 0, 0, 0, 0, 0
	for _, rec := range recs {
		if r.opts.Tranche > 0 && budget <= 0 {
			skipped++
			continue
		}
		budget--
		g, err := r.doneGroup(rec)
		var n, m int
		if err == nil {
			n, m, err = r.recheckGroup(ctx, g)
		}
		if err != nil {
			failed++
			fmt.Fprintf(r.deps.Out, "FAILED events recheck %s -> %s: %v\n", rec.OldID, rec.NewID, err)
			continue
		}
		checked++
		listed += n
		rekeyed += m
	}
	fmt.Fprintf(r.deps.Out, "events recheck: %d previously completed group(s) checked, %d event(s) listed, %d re-keyed, %d failed, %d skipped (tranche budget)\n",
		checked, listed, rekeyed, failed, skipped)
	return failed
}

// doneGroup rebuilds a completed group from its journal record and the current inventory.
func (r *runner) doneGroup(rec StateRecord) (*Group, error) {
	g := &Group{OldID: rec.OldID, Key: rec.OldID, Shape: ShapeOf(rec.OldID), Route: RouteRewrite, NewID: rec.NewID}
	for _, id := range rec.CompanyIDs {
		row := r.plan.inv.ByID[id]
		if row == nil {
			return nil, fmt.Errorf("company %s is gone", id)
		}
		if row.ExternalID != rec.NewID {
			return nil, fmt.Errorf("company %s carries %q, not the recorded %s", id, row.RawExternalID, rec.NewID)
		}
		g.Rows = append(g.Rows, row)
	}
	if len(g.Rows) == 0 {
		return nil, errors.New("no company ids recorded")
	}
	return g, nil
}

// recheckGroup lists the events still keyed by the group's old id; a dry run only reports them.
func (r *runner) recheckGroup(ctx context.Context, g *Group) (listed, rekeyed int, err error) {
	if !r.plan.Apply {
		ids, listErr := r.listOldEvents(ctx, g)
		if listErr != nil {
			return 0, 0, listErr
		}
		fmt.Fprintf(r.deps.Out, "events recheck %s -> %s: %d listed (dry run)\n", g.OldID, g.NewID, len(ids))
		return len(ids), 0, nil
	}
	listed, rekeyed, err = r.rekeyListed(ctx, g)
	if err != nil {
		return listed, rekeyed, err
	}
	fmt.Fprintf(r.deps.Out, "events recheck %s -> %s: %d listed, %d re-keyed\n", g.OldID, g.NewID, listed, rekeyed)
	return listed, rekeyed, nil
}
