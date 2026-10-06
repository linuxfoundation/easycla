// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

// Plan is the classified, filtered work list of one ingest run.
type Plan struct {
	Stage     string
	Apply     bool
	Groups    []*Group
	Register  []*Group
	Rewrite   []*Group
	Targets   []*Target
	Skipped   int
	mapping   *Mapping
	decisions *Decisions
	shared    SharedDomains
	state     *State
	inv       *Inventory
}

// BuildPlan loads the inventory, replays unfinished state, classifies every eligible group and
// applies --ids/--routes/--tranche. It performs reads only.
func BuildPlan(ctx context.Context, deps Deps, opts Options) (*Plan, error) {
	deps = withDefaults(deps)
	routes := routeSet(opts.Routes)
	if opts.UseApex && deps.Apex == nil {
		return nil, ErrApexUnavailable
	}
	mapping, decisions, shared, state, err := loadInputs(opts)
	if err != nil {
		return nil, err
	}
	inv, err := LoadInventory(ctx, deps)
	if err != nil {
		return nil, err
	}
	replayed, err := replayGroups(ctx, deps, inv, state)
	if err != nil {
		return nil, err
	}
	groups := mergeReplayed(inv.EligibleGroups(), replayed)

	for _, g := range groups {
		if g.Replayed {
			continue
		}
		classify(ctx, deps, g, mapping, shared, opts)
	}
	livenessGuard(groups)
	targets := applyDecisions(groups, inv, mapping, decisions)
	known := knownAccountsFromGroups(shared, groups)
	groups = filterIDs(groups, opts.IDs)
	suggestAccounts(ctx, deps, shared, known, groups, func(g *Group) bool {
		return g.Err == nil && (g.Route == RouteRewrite || g.Route == RouteManual)
	})

	plan := &Plan{Stage: opts.Stage, Apply: opts.Apply, Groups: groups, Targets: targets, mapping: mapping, decisions: decisions, shared: shared, state: state, inv: inv}
	budget := opts.Tranche
	for _, g := range groups {
		if g.Err != nil || g.Route == RouteManual || g.Pending() {
			continue
		}
		if !routes[g.Route] || (g.Route == RouteRewrite && state.Done(g.Key)) {
			plan.Skipped++
			continue
		}
		if opts.Tranche > 0 && budget == 0 {
			plan.Skipped++
			continue
		}
		budget--
		switch g.Route {
		case RouteRegister:
			plan.Register = append(plan.Register, g)
		case RouteRewrite:
			plan.Rewrite = append(plan.Rewrite, g)
		}
	}
	return plan, nil
}

func loadInputs(opts Options) (mapping *Mapping, decisions *Decisions, shared SharedDomains, state *State, err error) {
	if opts.Mapping != "" {
		if mapping, err = LoadMapping(opts.Mapping); err != nil {
			return nil, nil, nil, nil, err
		}
	}
	if opts.Decisions != "" {
		if decisions, err = LoadDecisions(opts.Decisions); err != nil {
			return nil, nil, nil, nil, err
		}
	}
	if shared, err = LoadSharedDomains(opts.SharedDomains); err != nil {
		return nil, nil, nil, nil, err
	}
	if state, err = LoadState(opts.State); err != nil {
		return nil, nil, nil, nil, err
	}
	return mapping, decisions, shared, state, nil
}

// mergeReplayed puts replayed (unfinished) groups first and drops the eligible groups they supersede.
func mergeReplayed(groups, replayed []*Group) []*Group {
	if len(replayed) == 0 {
		return groups
	}
	superseded := map[string]bool{}
	for _, g := range replayed {
		superseded[g.Key] = true
		superseded[g.NewID] = true
	}
	kept := groups[:0]
	for _, g := range groups {
		if !superseded[g.Key] {
			kept = append(kept, g)
		}
	}
	return append(replayed, kept...)
}

// filterIDs keeps the groups whose key, old id or new id is listed (empty list = everything).
func filterIDs(groups []*Group, ids []string) []*Group {
	if len(ids) == 0 {
		return groups
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[strings.TrimSpace(id)] = true
	}
	filtered := groups[:0]
	for _, g := range groups {
		if want[g.Key] || want[g.OldID] || want[g.NewID] {
			filtered = append(filtered, g)
		}
	}
	return filtered
}

// Pending reports the rewrite candidates that still need a mapping row (or Apex).
func (p *Plan) Pending() int {
	n := 0
	for _, g := range p.Groups {
		if g.Err == nil && g.Pending() {
			n++
		}
	}
	return n
}

// Unregistered counts the Accounts whose GET /b2b_orgs answered 403 (no b2b_org yet).
func (p *Plan) Unregistered() int {
	n := 0
	for _, g := range p.Groups {
		if g.Live == LiveUnregistered {
			n++
		}
	}
	return n
}

func routeSet(routes []Route) map[Route]bool {
	set := map[Route]bool{}
	for _, r := range routes {
		set[r] = true
	}
	if len(set) == 0 {
		set[RouteRegister] = true
		set[RouteRewrite] = true
	}
	return set
}

func withDefaults(deps Deps) Deps {
	if deps.Out == nil {
		deps.Out = io.Discard
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Sleep == nil {
		deps.Sleep = func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		}
	}
	return deps
}

// replayGroups rebuilds unfinished rewrite groups from the state file (pinned by company ids). A
// failed Apex resolution is not replayed: the group is classified again from scratch.
func replayGroups(ctx context.Context, deps Deps, inv *Inventory, state *State) ([]*Group, error) {
	var out []*Group
	for _, rec := range state.Unfinished() {
		if rec.Step == StepResolve && rec.Status == statusFailed {
			continue
		}
		if rec.NewID == "" || !IsSFID(rec.NewID) {
			return nil, fmt.Errorf("state: unfinished group %s has no valid new_id", rec.key())
		}
		g := &Group{OldID: rec.OldID, Key: rec.key(), Shape: ShapeOf(rec.OldID), Route: RouteRewrite, NewID: rec.NewID, Replayed: true, Action: "replay:" + rec.Step}
		for _, id := range rec.CompanyIDs {
			dbRow, err := deps.Companies.GetCompanyRecord(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("state: reading company %s of group %s: %w", id, rec.key(), err)
			}
			row := &Row{CompanyID: dbRow.CompanyID, CompanyName: dbRow.CompanyName, SigningEntityName: dbRow.SigningEntityName, ExternalID: strings.TrimSpace(dbRow.CompanyExternalID), RawExternalID: dbRow.CompanyExternalID}
			if cur := inv.ByID[id]; cur != nil {
				row.ActiveCCLA, row.CCLACount, row.CLAGroupIDs, row.CCLASignedOn = cur.ActiveCCLA, cur.CCLACount, cur.CLAGroupIDs, cur.CCLASignedOn
			}
			g.Rows = append(g.Rows, row)
		}
		if len(g.Rows) == 0 {
			return nil, fmt.Errorf("state: unfinished group %s has no company ids", rec.key())
		}
		out = append(out, g)
	}
	return out, nil
}

func classify(ctx context.Context, deps Deps, g *Group, mapping *Mapping, shared SharedDomains, opts Options) {
	switch g.Shape {
	case ShapeEmpty, ShapeOther:
		classifyRowTargeted(g, mapping)
		return
	case ShapeSFID:
		if len(g.Aliases) > 0 {
			g.Route, g.ManualReason = RouteManual, ReasonSFIDAliasForms
			return
		}
		var liveErr error
		g.Live, liveErr = liveness(ctx, deps, g.OldID)
		switch g.Live {
		case LiveLive:
			g.Route = RouteRegister
			return
		case LiveUnverified:
			g.Route, g.ManualReason = RouteRegister, ReasonCRMUnverified
			lookupOrg(ctx, deps, g)
			return
		case LiveUnregistered:
			g.Route, g.liveErr = RouteRegister, liveErr
			if !opts.RegisterUnregistered {
				g.ManualReason = ReasonUnregistered
			}
			lookupOrg(ctx, deps, g)
			return
		case LiveDead:
			g.Route = RouteRewrite
		default:
			g.Err = fmt.Errorf("liveness check failed for %s: %w", g.OldID, liveErr)
			return
		}
	case ShapeLF:
		g.Route = RouteRewrite
	}

	lookupOrg(ctx, deps, g)
	newID, action, reason := resolveNewID(ctx, deps, g, mapping, shared, opts)
	if reason == ReasonNoMapping {
		g.ManualReason = reason
		return
	}
	if reason != "" {
		g.Route, g.ManualReason, g.Action = RouteManual, reason, action
		return
	}
	g.NewID, g.Action = newID, action
}

// lookupOrg records the org-service view of the group's old id.
func lookupOrg(ctx context.Context, deps Deps, g *Group) {
	org, err := deps.Orgs.GetOrganization(ctx, g.OldID)
	switch {
	case err == nil:
		g.Org, g.OrgStatus = org, "200"
	case errors.Is(err, ErrOrgNotFound):
		g.OrgStatus = "404"
	default:
		g.OrgStatus = orgStatusErr
	}
}

// classifyRowTargeted routes an empty/malformed-id row: a mapping row keyed by the company id
// (old_id = company_id) makes it a rewrite of that one row, anything else is manual.
func classifyRowTargeted(g *Group, mapping *Mapping) {
	newID, action, reason := mapping.Resolve(g.Key)
	switch {
	case reason == ReasonNoMapping:
		g.Route, g.ManualReason = RouteManual, "empty_external_id"
		if g.Shape == ShapeOther {
			g.ManualReason = "invalid_id_shape"
		}
	case reason != "":
		g.Route, g.ManualReason, g.Action = RouteManual, reason, action
	default:
		g.Route, g.NewID, g.Action = RouteRewrite, newID, action
	}
}

// liveness asks member-service (the CRM view) whether the Account exists; without member-service
// the answer is unverified — org-service may still serve an Account deleted from Salesforce. A 403
// from member-service itself means the Account has no b2b_org yet (unregistered).
func liveness(ctx context.Context, deps Deps, id string) (string, error) {
	if deps.Members == nil {
		return LiveUnverified, nil
	}
	_, err := deps.Members.GetB2BOrg(ctx, id)
	var authErr *member_service.AuthError
	switch {
	case err == nil:
		return LiveLive, nil
	case errors.Is(err, member_service.ErrOrgNotFound):
		return LiveDead, nil
	case errors.As(err, &authErr) && authErr.Status == 403 && !authErr.Token:
		return LiveUnregistered, err
	}
	return LiveError, err
}

// livenessGuard turns "unregistered" back into the access error when no Account at all answered
// 200 or 404: the client then cannot see any b2b_org (missing access tuple), not just these ones.
func livenessGuard(groups []*Group) {
	seen, unregistered := 0, 0
	for _, g := range groups {
		switch g.Live {
		case LiveLive, LiveDead:
			seen++
		case LiveUnregistered:
			unregistered++
		}
	}
	if unregistered == 0 || seen > 0 {
		return
	}
	for _, g := range groups {
		if g.Live != LiveUnregistered {
			continue
		}
		g.Live, g.Route, g.ManualReason = LiveError, "", ""
		g.Err = fmt.Errorf("liveness check failed for %s: member-service answered (403) for every Account checked (%d) and 200 for none: the tool's client cannot see any b2b_org (access tuple missing?): %w", g.OldID, unregistered, g.liveErr)
	}
}

func resolveNewID(ctx context.Context, deps Deps, g *Group, mapping *Mapping, shared SharedDomains, opts Options) (newID, action, reason string) {
	if !opts.UseApex {
		return mapping.Resolve(g.OldID)
	}
	// An approved mapping row is the operator's manual resolution and wins over Apex and the domain gate.
	if id, mapAction, mapReason := mapping.Resolve(g.OldID); mapReason == "" && id != "" {
		return id, mapAction, ""
	}
	if _, gate := shared.Shared(g.Website()); gate != "" {
		return "", "", gate
	}
	g.ViaApex = true
	res, err := deps.Apex.FindOrCreate(ctx, apexRequest(g, true))
	if err != nil {
		g.Err = fmt.Errorf("apex dry run for %s: %w", g.OldID, err)
		return "", "", "apex_error"
	}
	switch res.Action {
	case ActionAmbiguous:
		return "", res.Action, ReasonMappingAmbiguous
	case ActionMatched:
		approvedID, _, mapReason := mapping.Resolve(g.OldID)
		if mapReason != "" || approvedID != res.ID {
			return "", res.Action, "apex_match_needs_approval"
		}
	case ActionCreated:
		if res.ID == "" {
			return "", res.Action, ""
		}
	}
	if accountKey(res.ID) == accountKey(g.OldID) {
		return "", res.Action, ReasonMappingSameID
	}
	return res.ID, res.Action, ""
}

func apexRequest(g *Group, dryRun bool) ApexRequest {
	req := ApexRequest{Source: "EasyCLA", ExternalKey: g.OldID, DryRun: dryRun, Name: g.Rows[0].CompanyName}
	if g.Org != nil {
		req.Name, req.Website = firstNonEmpty(g.Org.Name, req.Name), g.Org.Website
	}
	for _, r := range g.Rows {
		if r.CCLASignedOn != "" && (req.CCLASignedDate == "" || r.CCLASignedOn < req.CCLASignedDate) {
			req.CCLASignedDate = r.CCLASignedOn
		}
	}
	return req
}

// Print writes the human-readable plan.
func (p *Plan) Print(w io.Writer) {
	mode := ModeDryRun
	if p.Apply {
		mode = ModeApply
	}
	fmt.Fprintf(w, "org_import ingest stage=%s mode=%s eligible_groups=%d register=%d rewrite=%d pending=%d unregistered=%d skipped=%d\n", p.Stage, mode, len(p.Groups), len(p.Register), len(p.Rewrite), p.Pending(), p.Unregistered(), p.Skipped)
	for _, g := range p.Groups {
		fmt.Fprintf(w, "%s\n", describe(g))
	}
	for _, g := range p.Register {
		fmt.Fprintf(w, "PLAN register %s: POST /b2b_orgs {sfid:%s} (rows: %s)\n", g.OldID, g.OldID, strings.Join(g.CompanyIDs(), ","))
	}
	for _, g := range p.Rewrite {
		if g.RowTargeted() {
			fmt.Fprintf(w, "PLAN rewrite row %s (company_external_id %q) -> %s (%s): wait org-service; set company_external_id; POST /b2b_orgs {sfid:%s}\n",
				g.Key, g.OldID, g.NewID, g.Action, g.NewID)
			continue
		}
		newID := g.NewID
		if g.ApexCreates() {
			newID = "<new Account, id assigned by Apex at apply>"
		}
		fmt.Fprintf(w, "PLAN rewrite %s -> %s (%s): wait org-service; copy ACS grants; rewrite %d row(s) %s; re-key events; delete old grants; POST /b2b_orgs {sfid:%s}\n",
			g.OldID, newID, g.Action, len(g.Rows), strings.Join(g.CompanyIDs(), ","), newID)
	}
	if len(p.Targets) > 0 {
		fmt.Fprintf(w, "TARGETS (rewrite destinations grouped by Account):\n")
		for _, t := range p.Targets {
			fmt.Fprintf(w, "  %s\n", describeTarget(t))
		}
	}
	actions := p.ManualActions()
	if len(actions) > 0 {
		fmt.Fprintf(w, "MANUAL ACTIONS (%d):\n", len(actions))
		for _, a := range actions {
			fmt.Fprintf(w, "  %s\n", a.String())
		}
	}
}

func describeTarget(t *Target) string {
	s := fmt.Sprintf("%s status=%s old_ids=%s", t.SFID, t.Status, strings.Join(t.OldIDs(), ";"))
	if t.Decision != nil {
		s += " decision=" + t.Decision.Kind + " reviewer=" + t.Decision.Reviewer
	}
	if len(t.Existing) > 0 {
		s += fmt.Sprintf(" existing_rows=%d", len(t.Existing))
	}
	return s
}

func describe(g *Group) string {
	names := make([]string, 0, len(g.Rows))
	for _, r := range g.Rows {
		n := r.CompanyName
		if r.SigningEntityName != "" && !strings.EqualFold(r.SigningEntityName, r.CompanyName) {
			n += " / " + r.SigningEntityName
		}
		names = append(names, n)
	}
	s := fmt.Sprintf("%-8s %-20s shape=%-5s rows=%d", g.Route, g.Key, g.Shape, len(g.Rows))
	if g.Live != "" {
		s += " live=" + g.Live
	}
	if g.NewID != "" {
		s += " new_id=" + g.NewID + " action=" + g.Action
	} else if g.ApexCreates() {
		s += " new_id=<apex-at-apply> action=" + g.Action
	}
	if g.ManualReason != "" {
		s += " reason=" + g.ManualReason
	}
	if len(g.Aliases) > 0 {
		s += " aliases=" + strings.Join(g.Aliases, ",")
	}
	if d := g.Domain(); d != "" {
		s += " domain=" + d
	}
	if g.Err != nil {
		s += " error=" + g.Err.Error()
	}
	return s + " | " + strings.Join(names, "; ")
}

// Execute runs the plan; with Apply=false nothing is written anywhere (report files aside).
func Execute(ctx context.Context, deps Deps, opts Options, plan *Plan) (Summary, error) {
	deps = withDefaults(deps)
	sum := Summary{Stage: plan.Stage, Mode: ModeDryRun, Eligible: len(plan.Groups)}
	if plan.Apply {
		sum.Mode = ModeApply
	}
	for _, g := range plan.Groups {
		switch {
		case g.Err != nil:
			sum.Failed++
		case g.Route == RouteManual:
			sum.Manual++
		case g.Pending():
			sum.Pending++
		}
	}
	if opts.OutDir != "" {
		if err := writeIngestReports(opts.OutDir, plan); err != nil {
			return sum, err
		}
	}
	r := &runner{deps: deps, opts: opts, plan: plan}
	recheck := routeSet(opts.Routes)[RouteRewrite]
	recheckBudget := opts.Tranche - len(plan.Register) - len(plan.Rewrite)
	if !plan.Apply {
		if recheck {
			sum.Failed += r.recheckEvents(ctx, recheckBudget)
		}
		if sum.Failed > 0 {
			return sum, fmt.Errorf("%d group(s) failed", sum.Failed)
		}
		return sum, nil
	}
	if err := applyPreconditions(deps, plan); err != nil {
		for _, g := range append(append([]*Group(nil), plan.Register...), plan.Rewrite...) {
			g.Err = err
			sum.Failed++
		}
		if opts.OutDir != "" {
			if wErr := writeIngestReports(opts.OutDir, plan); wErr != nil {
				return sum, wErr
			}
		}
		return sum, err
	}
	for _, g := range plan.Register {
		if err := r.register(ctx, g); err != nil {
			g.Err = err
			sum.Failed++
			fmt.Fprintf(deps.Out, "FAILED register %s: %v\n", g.OldID, err)
			continue
		}
		sum.Registered++
	}
	if len(plan.Rewrite) > 0 {
		r.resolveApex(ctx)
		r.waitForOrgs(ctx)
		for _, g := range plan.Rewrite {
			if g.Err != nil {
				sum.Failed++
				fmt.Fprintf(deps.Out, "FAILED rewrite %s: %v\n", g.Key, g.Err)
				continue
			}
			if err := r.rewrite(ctx, g); err != nil {
				g.Err = err
				sum.Failed++
				fmt.Fprintf(deps.Out, "FAILED rewrite %s -> %s: %v\n", g.Key, g.NewID, err)
				continue
			}
			sum.Rewritten++
		}
	}
	if recheck {
		sum.Failed += r.recheckEvents(ctx, recheckBudget)
	}
	if opts.OutDir != "" {
		if err := writeIngestReports(opts.OutDir, plan); err != nil {
			return sum, err
		}
	}
	if sum.Failed > 0 {
		return sum, fmt.Errorf("%d group(s) failed", sum.Failed)
	}
	return sum, nil
}

type runner struct {
	deps Deps
	opts Options
	plan *Plan
}

// applyPreconditions rejects an apply that could not finish: rewrites need the resume journal and
// every route ends with a member-service registration.
func applyPreconditions(deps Deps, plan *Plan) error {
	if len(plan.Register) == 0 && len(plan.Rewrite) == 0 {
		return nil
	}
	if len(plan.Rewrite) > 0 && plan.state.path == "" {
		return ErrStateRequired
	}
	if deps.Members == nil {
		return ErrNotConfigured
	}
	return nil
}

// record appends one state line; a failure to journal is an error for the group.
func (r *runner) record(g *Group, step, status string, err error) error {
	rec := StateRecord{OldID: g.OldID, NewID: g.NewID, CompanyIDs: g.CompanyIDs(), Step: step, Status: status}
	if g.RowTargeted() {
		rec.Key = g.Key
	}
	if err != nil {
		rec.Err = err.Error()
	}
	if aErr := r.plan.state.Append(rec, r.deps.Now()); aErr != nil {
		return fmt.Errorf("state file %s: cannot record %s for %s: %w", r.plan.state.path, step, g.Key, aErr)
	}
	return nil
}

func (r *runner) register(ctx context.Context, g *Group) error {
	if r.deps.Members == nil {
		return ErrNotConfigured
	}
	org, err := r.deps.Members.RegisterB2BOrg(ctx, g.OldID)
	if errors.Is(err, member_service.ErrOrgNotFound) {
		g.Live, g.Route, g.ManualReason = LiveDead, RouteRewrite, ReasonDeadAccount
		return fmt.Errorf("account %s does not exist in Salesforce (rewrite candidate)", g.OldID)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(r.deps.Out, "registered %s as b2b_org %s (%s)\n", g.OldID, org.UID, org.Name)
	if g.Live != LiveLive {
		r.confirmRegistered(ctx, g.OldID)
	}
	return nil
}

const (
	registerChecks     = 5
	registerCheckDelay = 3 * time.Second
)

// confirmRegistered re-reads a freshly registered b2b_org: Heimdall answers 403 until the FGA tuples
// land, so 403/404 are retried within a small budget and the result is only reported.
func (r *runner) confirmRegistered(ctx context.Context, sfid string) {
	for i := 1; i <= registerChecks; i++ {
		_, err := r.deps.Members.GetB2BOrg(ctx, sfid)
		var authErr *member_service.AuthError
		switch {
		case err == nil:
			fmt.Fprintf(r.deps.Out, "b2b_org %s visible after %d check(s)\n", sfid, i)
			return
		case errors.Is(err, member_service.ErrOrgNotFound), errors.As(err, &authErr) && authErr.Status == 403 && !authErr.Token:
			if i < registerChecks {
				if sErr := r.deps.Sleep(ctx, registerCheckDelay); sErr != nil {
					return
				}
			}
		default:
			fmt.Fprintf(r.deps.Out, "WARNING: b2b_org %s check failed: %v; the registration itself succeeded\n", sfid, err)
			return
		}
	}
	fmt.Fprintf(r.deps.Out, "WARNING: b2b_org %s not yet visible after %d checks (FGA tuples pending); the registration itself succeeded\n", sfid, registerChecks)
}

// resolveApex performs the real Apex call for every rewrite group resolved through --use-apex; a
// group created in the dry run receives its Account id here, anything else must match the dry run.
func (r *runner) resolveApex(ctx context.Context) {
	if !r.opts.UseApex {
		return
	}
	for _, g := range r.plan.Rewrite {
		if g.Replayed || g.Err != nil || !g.ViaApex {
			continue
		}
		res, err := r.deps.Apex.FindOrCreate(ctx, apexRequest(g, false))
		switch {
		case err != nil:
			g.Err = err
		case res.Action != g.Action || (g.NewID != "" && res.ID != g.NewID):
			g.Err = fmt.Errorf("apex result changed between dry run (%s %s) and call (%s %s)", g.Action, g.NewID, res.Action, res.ID)
		case g.NewID == "" && (!IsSFID(res.ID) || res.ID == g.OldID):
			g.Err = fmt.Errorf("apex %s returned unusable account id %q", res.Action, res.ID)
		default:
			g.NewID = res.ID
		}
		if recErr := r.record(g, StepResolve, status(g.Err), g.Err); recErr != nil && g.Err == nil {
			g.Err = recErr
		}
	}
}

// waitForOrgs polls org-service until every new id is served (one shared wait for the tranche). A
// target org-service serves under another id than the reviewed one fails before any cutover: the tool
// never writes a second spelling of an Account id.
func (r *runner) waitForOrgs(ctx context.Context) {
	pending := map[string][]*Group{}
	for _, g := range r.plan.Rewrite {
		if g.Err == nil && g.NewID == "" {
			g.Err = errors.New("no new id resolved")
		}
		if g.Err == nil {
			pending[g.NewID] = append(pending[g.NewID], g)
		}
	}
	fail := func(id string, err error) {
		for _, g := range pending[id] {
			g.Err = err
			if recErr := r.record(g, StepWait, statusFailed, err); recErr != nil {
				fmt.Fprintf(r.deps.Out, "WARNING: %v\n", recErr)
			}
		}
		delete(pending, id)
	}
	waitMax, poll := r.opts.WaitMax, r.opts.WaitPoll
	if waitMax <= 0 {
		waitMax = 40 * time.Minute
	}
	if poll <= 0 {
		poll = 30 * time.Second
	}
	deadline := r.deps.Now().Add(waitMax)
	lastErr := map[string]error{}
	for len(pending) > 0 {
		ids := make([]string, 0, len(pending))
		for id := range pending {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			org, err := r.deps.Orgs.GetOrganization(ctx, id)
			if err == nil {
				if org.ID != id {
					fail(id, fmt.Errorf("org-service serves target %s as %q: the mapping/decisions must use the id org-service returns", id, org.ID))
					continue
				}
				for _, g := range pending[id] {
					if g.Org == nil {
						g.Org = org
					}
				}
				delete(pending, id)
				continue
			}
			lastErr[id] = err
		}
		if len(pending) == 0 || r.opts.SkipWait || !r.deps.Now().Before(deadline) || ctx.Err() != nil {
			break
		}
		fmt.Fprintf(r.deps.Out, "waiting for org-service to serve %d new id(s)...\n", len(pending))
		if err := r.deps.Sleep(ctx, poll); err != nil {
			break
		}
	}
	ids := make([]string, 0, len(pending))
	for id := range pending {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		fail(id, fmt.Errorf("org-service does not serve %s yet: %v", id, lastErr[id]))
	}
}

const (
	statusOK     = "ok"
	statusFailed = "failed"
)

func status(err error) string {
	if err != nil {
		return statusFailed
	}
	return statusOK
}
