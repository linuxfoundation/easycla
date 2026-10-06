// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Tiers mirror utils/audit_company_reachability.sh so counts can be cross-checked.
const (
	TierMissing  = "MISSING_SFID"
	TierInvalid  = "INVALID_SFID_FORMAT"
	TierOK       = "SFID_OK"
	TierDangling = "SFID_DANGLING_OR_DELETED"
	TierUnknown  = "UNKNOWN"
)

// AuditRow is one line of audit.csv.
type AuditRow struct {
	Row          *Row
	Shape        IDShape
	ECLACount    int
	ECLACounted  bool
	OrgStatus    string
	Website      string
	Duplicate    bool
	Route        Route
	ManualReason string
	Tier         string
	OrgName      string
	ACSRoles     string
	Suggested    string
}

// AuditResult is the audit output; Duplicates are the candidate-target sets (rows sharing an id,
// a normalized name or a non-shared website domain under different ids).
type AuditResult struct {
	Rows       []*AuditRow
	Groups     []*Group
	Tiers      map[string]int
	Routes     map[Route]int
	Duplicates [][]*Row
	rowByID    map[string]*AuditRow
	shared     SharedDomains
}

// Audit classifies every company row (reads only) and writes audit.csv, unresolvable.csv and
// possible_duplicates.csv into opts.OutDir.
func Audit(ctx context.Context, deps Deps, opts Options) (*AuditResult, error) {
	deps = withDefaults(deps)
	shared, err := LoadSharedDomains(opts.SharedDomains)
	if err != nil {
		return nil, err
	}
	inv, err := LoadInventory(ctx, deps)
	if err != nil {
		return nil, err
	}
	groups := inv.EligibleGroups()
	for _, g := range groups {
		classify(ctx, deps, g, nil, nil, Options{})
	}
	livenessGuard(groups)
	groupByRow := map[string]*Group{}
	for _, g := range groups {
		for _, r := range g.Rows {
			groupByRow[r.CompanyID] = g
		}
	}

	orgs := lookupOrgs(ctx, deps, inv)
	res := &AuditResult{Tiers: map[string]int{}, Routes: map[Route]int{}, rowByID: map[string]*AuditRow{}, shared: shared}
	byName, byDomain := map[string][]*Row{}, map[string][]*Row{}
	known := newAccountIndex()
	for _, row := range inv.Rows {
		ar := &AuditRow{Row: row, Shape: ShapeOf(row.ExternalID)}
		if o := orgs[row.ExternalID]; o != nil {
			ar.OrgStatus = o.status
			if o.org != nil {
				ar.Website, ar.OrgName = o.org.Website, o.org.Name
				known.add(shared, row.ExternalID, o.org, row.CompanyName)
			}
		}
		ar.Tier = tier(row.ExternalID, ar.OrgStatus)
		if g := groupByRow[row.CompanyID]; g != nil {
			ar.Duplicate, ar.Route, ar.ManualReason = g.Duplicate, g.Route, g.ManualReason
			if g.Err != nil {
				ar.ManualReason = "error: " + g.Err.Error()
			}
		}
		res.Tiers[ar.Tier]++
		if ar.Route != "" {
			res.Routes[ar.Route]++
		}
		res.Rows = append(res.Rows, ar)
		res.rowByID[row.CompanyID] = ar
		byName[canonicalEntity(row.CompanyName)] = append(byName[canonicalEntity(row.CompanyName)], row)
		if d, gate := shared.Shared(ar.Website); gate == "" {
			byDomain[d] = append(byDomain[d], row)
		}
	}
	res.Groups = groups

	res.collectDuplicates(byName, byDomain)
	res.countECLAs(ctx, deps.ECLAs)
	inDup := map[string]bool{}
	for _, set := range res.Duplicates {
		for _, r := range set {
			inDup[r.CompanyID] = true
		}
	}
	res.collectACSRoles(ctx, deps.ACS, inDup)
	suggestRows(ctx, deps, shared, known, res.Rows, func(ar *AuditRow) bool {
		if inDup[ar.Row.CompanyID] {
			return true
		}
		g := groupByRow[ar.Row.CompanyID]
		if g == nil || g.Err != nil {
			return false
		}
		return g.Route == RouteManual || !(ar.Shape == ShapeSFID && ar.OrgStatus == "200" && g.Live != LiveDead)
	})

	if opts.OutDir != "" {
		if err = writeAuditReports(opts.OutDir, res); err != nil {
			return nil, err
		}
	}
	for _, ar := range res.Rows {
		r := ar.Row
		fmt.Fprintf(deps.Out, "audit row company_id=%s name=%q external_id=%q shape=%s active_ccla=%t ccla=%d ecla=%d org_service=%s route=%s reason=%q tier=%s\n",
			r.CompanyID, firstNonEmpty(r.SigningEntityName, r.CompanyName), r.RawExternalID, ar.Shape, r.ActiveCCLA, r.CCLACount, ar.ECLACount, ar.OrgStatus, ar.Route, ar.ManualReason, ar.Tier)
	}
	fmt.Fprintf(deps.Out, "audit stage=%s companies=%d eligible_groups=%d", opts.Stage, len(res.Rows), len(groups))
	for _, t := range []string{TierMissing, TierInvalid, TierOK, TierDangling, TierUnknown} {
		fmt.Fprintf(deps.Out, " %s=%d", t, res.Tiers[t])
	}
	unregistered := 0
	for _, g := range groups {
		if g.Live == LiveUnregistered {
			unregistered++
		}
	}
	fmt.Fprintf(deps.Out, " register=%d rewrite=%d manual=%d unregistered=%d duplicates=%d\n", res.Routes[RouteRegister], res.Routes[RouteRewrite], res.Routes[RouteManual], unregistered, len(res.Duplicates))
	return res, nil
}

// collectACSRoles fills ACSRoles for the external ids of the eligible groups (001/lf) and of every
// row in a candidate set (4 workers); a failed listing shows as "err".
func (res *AuditResult) collectACSRoles(ctx context.Context, acs ACSService, inDup map[string]bool) {
	if acs == nil {
		return
	}
	want := map[string]bool{}
	for _, g := range res.Groups {
		if g.Shape == ShapeSFID || g.Shape == ShapeLF {
			want[g.OldID] = true
		}
	}
	for _, ar := range res.Rows {
		if inDup[ar.Row.CompanyID] && (ar.Shape == ShapeSFID || ar.Shape == ShapeLF) {
			want[ar.Row.ExternalID] = true
		}
	}
	ids := make([]string, 0, len(want))
	for id := range want {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	roles := make([]string, len(ids))
	runWorkers(len(ids), func(i int) { roles[i] = acsRoles(ctx, acs, ids[i]) })
	byID := make(map[string]string, len(ids))
	for i, id := range ids {
		byID[id] = roles[i]
	}
	for _, ar := range res.Rows {
		ar.ACSRoles = byID[ar.Row.ExternalID]
	}
}

func (ar *AuditRow) unresolvable() bool {
	return ar.Shape == ShapeEmpty || ar.Shape == ShapeOther || (ar.Shape == ShapeSFID && ar.OrgStatus == "404")
}

// collectDuplicates builds the candidate-target sets: rows sharing an id (with the same or empty
// signing entity), then rows with the same normalized name or the same non-shared domain under
// different ids. Overlapping sets are merged into one (union by company id).
func (res *AuditResult) collectDuplicates(byName, byDomain map[string][]*Row) {
	var sets [][]*Row
	for _, g := range res.Groups {
		if g.Duplicate {
			sets = append(sets, g.Rows)
		}
	}
	for _, m := range []map[string][]*Row{byName, byDomain} {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if rows := m[k]; k != "" && len(rows) >= 2 && distinctExternalIDs(rows) {
				sets = append(sets, rows)
			}
		}
	}
	for _, set := range mergeOverlapping(sets) {
		res.addDuplicates(set)
	}
}

// mergeOverlapping unions sets sharing a row; sets and rows keep their first-appearance order.
func mergeOverlapping(sets [][]*Row) [][]*Row {
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	var order []*Row
	seenRow := map[string]bool{}
	for _, set := range sets {
		for _, r := range set {
			if _, ok := parent[r.CompanyID]; !ok {
				parent[r.CompanyID] = r.CompanyID
			}
			if !seenRow[r.CompanyID] {
				seenRow[r.CompanyID] = true
				order = append(order, r)
			}
			parent[find(r.CompanyID)] = find(set[0].CompanyID)
		}
	}
	index := map[string]int{}
	var out [][]*Row
	for _, r := range order {
		root := find(r.CompanyID)
		i, ok := index[root]
		if !ok {
			i = len(out)
			index[root] = i
			out = append(out, nil)
		}
		out[i] = append(out[i], r)
	}
	return out
}

// countECLAs fills ECLACount for the active manual/duplicate/unresolvable rows and for every row of a
// candidate set (inactive ones included); -1 marks a failed count.
func (res *AuditResult) countECLAs(ctx context.Context, counter ECLACounter) {
	if counter == nil {
		return
	}
	reported := map[string]bool{}
	for _, set := range res.Duplicates {
		for _, r := range set {
			reported[r.CompanyID] = true
		}
	}
	for _, ar := range res.Rows {
		if reported[ar.Row.CompanyID] || (ar.Row.ActiveCCLA && (ar.Route == RouteManual || ar.Duplicate || ar.unresolvable())) {
			ar.ECLACounted = true
			if n, cErr := counter.CountECLAs(ctx, ar.Row.CompanyID); cErr == nil {
				ar.ECLACount = n
			} else {
				ar.ECLACount = -1
			}
		}
	}
}

// addDuplicates appends a candidate set once: the same rows reached by name and by domain are one set.
func (res *AuditResult) addDuplicates(rows []*Row) {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.CompanyID)
	}
	sort.Strings(ids)
	key := strings.Join(ids, "\x00")
	for _, set := range res.Duplicates {
		other := make([]string, 0, len(set))
		for _, r := range set {
			other = append(other, r.CompanyID)
		}
		sort.Strings(other)
		if strings.Join(other, "\x00") == key {
			return
		}
	}
	res.Duplicates = append(res.Duplicates, rows)
}

func distinctExternalIDs(rows []*Row) bool {
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.ExternalID] = true
	}
	return len(seen) > 1
}

func tier(externalID, orgStatus string) string {
	if externalID == "" {
		return TierMissing
	}
	if len(externalID) != 15 && len(externalID) != 18 || !alnum(externalID) {
		return TierInvalid
	}
	switch orgStatus {
	case "200":
		return TierOK
	case "404":
		return TierDangling
	default:
		return TierUnknown
	}
}

func alnum(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

type orgLookup struct {
	org    *Org
	status string
}

// lookupOrgs resolves every distinct 15/18-char external id in org-service (4 workers).
func lookupOrgs(ctx context.Context, deps Deps, inv *Inventory) map[string]*orgLookup {
	ids := make([]string, 0, len(inv.ByExternal))
	for id := range inv.ByExternal {
		if (len(id) == 15 || len(id) == 18) && alnum(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := make(map[string]*orgLookup, len(ids))
	var mu sync.Mutex
	var wg sync.WaitGroup
	work := make(chan string)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range work {
				l := &orgLookup{}
				org, err := deps.Orgs.GetOrganization(ctx, id)
				switch {
				case err == nil:
					l.org, l.status = org, "200"
				case errors.Is(err, ErrOrgNotFound):
					l.status = "404"
				default:
					l.status = orgStatusErr
				}
				mu.Lock()
				out[id] = l
				mu.Unlock()
			}
		}()
	}
	for _, id := range ids {
		work <- id
	}
	close(work)
	wg.Wait()
	return out
}

func writeAuditReports(dir string, res *AuditResult) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	rows := [][]string{{"company_id", "company_name", "signing_entity_name", "company_external_id", "id_shape", "active_ccla", "ccla_count", "ecla_count", "org_service", "website", "duplicate_sfid_group", "route", "manual_reason", "tier", "acs_roles", "suggested_account"}}
	var unresolvable [][]string
	unresolvable = append(unresolvable, []string{"company_id", "company_name", "signing_entity_name", "company_external_id", "id_shape", "ccla_count", "ecla_count", "org_service", "manual_reason"})
	for _, ar := range res.Rows {
		r := ar.Row
		rows = append(rows, []string{r.CompanyID, r.CompanyName, r.SigningEntityName, r.ExternalID, string(ar.Shape), strconv.FormatBool(r.ActiveCCLA), strconv.Itoa(r.CCLACount), strconv.Itoa(ar.ECLACount), ar.OrgStatus, ar.Website, strconv.FormatBool(ar.Duplicate), string(ar.Route), ar.ManualReason, ar.Tier, ar.ACSRoles, ar.Suggested})
		if r.ActiveCCLA && ar.unresolvable() {
			unresolvable = append(unresolvable, []string{r.CompanyID, r.CompanyName, r.SigningEntityName, r.ExternalID, string(ar.Shape), strconv.Itoa(r.CCLACount), strconv.Itoa(ar.ECLACount), ar.OrgStatus, ar.ManualReason})
		}
	}
	dups := [][]string{{"group", "company_id", "company_name", "signing_entity_name", "company_external_id", "id_shape", "domain", "active_ccla", "ccla_count", "ecla_count"}}
	for i, set := range res.Duplicates {
		for _, r := range set {
			shape, domain, ecla := string(ShapeOf(r.ExternalID)), "", ""
			if ar := res.rowByID[r.CompanyID]; ar != nil {
				shape, domain = string(ar.Shape), res.shared.Domain(ar.Website)
				if ar.ECLACounted {
					ecla = strconv.Itoa(ar.ECLACount)
				}
			}
			dups = append(dups, []string{strconv.Itoa(i + 1), r.CompanyID, r.CompanyName, r.SigningEntityName, r.ExternalID, shape, domain, strconv.FormatBool(r.ActiveCCLA), strconv.Itoa(r.CCLACount), ecla})
		}
	}
	for name, data := range map[string][][]string{"audit.csv": rows, "unresolvable.csv": unresolvable, "possible_duplicates.csv": dups} {
		if err := writeCSV(filepath.Join(dir, name), data); err != nil {
			return err
		}
	}
	return nil
}

func writeIngestReports(dir string, plan *Plan) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	rows := [][]string{{"key", "old_id", "id_shape", "route", "manual_reason", "live", "org_service", "website", "domain", "shared_domain", "new_id", "action", "decision", "reviewer", "company_ids", "company_names", "error", "suggested_account"}}
	toSF := [][]string{{"old_id", "name", "website", "ccla_signed_date", "domain", "shared_domain", "suggested_account"}}
	manual := [][]string{{"key", "old_id", "route", "reason", "suggested_action", "live", "org_service", "website", "domain", "shared_domain", "new_id", "action", "company_ids", "company_names", "error", "suggested_account"}}
	for _, g := range plan.Groups {
		names := strings.Join(g.Names(), ";")
		errText := ""
		if g.Err != nil {
			errText = g.Err.Error()
		}
		domain, shared := plan.domainOf(g)
		decision, reviewer := "", ""
		if g.Decision != nil {
			decision, reviewer = g.Decision.Kind, g.Decision.Reviewer
		}
		rows = append(rows, []string{g.Key, g.OldID, string(g.Shape), string(g.Route), g.ManualReason, g.Live, g.OrgStatus, g.Website(), domain, shared, g.NewID, g.Action, decision, reviewer, strings.Join(g.CompanyIDs(), ";"), names, errText, g.Suggested})
		if g.ManualReason == ReasonNoMapping || g.ManualReason == ReasonDeadAccount {
			req := apexRequest(g, true)
			toSF = append(toSF, []string{g.OldID, req.Name, req.Website, req.CCLASignedDate, domain, shared, g.Suggested})
		}
	}
	for _, a := range plan.ManualActions() {
		g := a.Group
		errText := ""
		if g.Err != nil {
			errText = g.Err.Error()
		}
		domain, shared := plan.domainOf(g)
		manual = append(manual, []string{g.Key, g.OldID, string(g.Route), a.Reason, a.Suggested, g.Live, g.OrgStatus, g.Website(), domain, shared, g.NewID, g.Action, strings.Join(g.CompanyIDs(), ";"), strings.Join(g.Names(), ";"), errText, g.Suggested})
	}
	targets := [][]string{{"target_sfid", "groups", "old_ids", "company_ids", "company_names", "existing_rows", "decision", "reviewer", "status"}}
	for _, t := range plan.Targets {
		var ids, names []string
		for _, g := range t.Groups {
			ids = append(ids, g.CompanyIDs()...)
			names = append(names, g.Names()...)
		}
		decision, reviewer := "", ""
		if t.Decision != nil {
			decision, reviewer = t.Decision.Kind, t.Decision.Reviewer
		}
		targets = append(targets, []string{t.SFID, strconv.Itoa(len(t.Groups)), strings.Join(t.OldIDs(), ";"), strings.Join(ids, ";"), strings.Join(names, ";"), strconv.Itoa(len(t.Existing)), decision, reviewer, t.Status})
	}
	for name, data := range map[string][][]string{"plan.csv": rows, "to_salesforce.csv": toSF, "manual_actions.csv": manual, "targets.csv": targets} {
		if err := writeCSV(filepath.Join(dir, name), data); err != nil {
			return err
		}
	}
	return nil
}

// domainOf returns the group's website domain and "true"/"false" for the shared-domain flag.
func (p *Plan) domainOf(g *Group) (string, string) {
	domain, reason := p.shared.Shared(g.Website())
	return domain, strconv.FormatBool(reason == ReasonSharedDomain)
}

func writeCSV(path string, rows [][]string) error {
	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err = w.WriteAll(rows); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
