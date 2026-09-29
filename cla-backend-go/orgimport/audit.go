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
	OrgStatus    string
	Website      string
	Duplicate    bool
	Route        Route
	ManualReason string
	Tier         string
}

// AuditResult is the audit output.
type AuditResult struct {
	Rows       []*AuditRow
	Groups     []*Group
	Tiers      map[string]int
	Routes     map[Route]int
	Duplicates [][]*Row
}

// Audit classifies every company row (reads only) and writes audit.csv, unresolvable.csv and
// possible_duplicates.csv into opts.OutDir.
func Audit(ctx context.Context, deps Deps, opts Options) (*AuditResult, error) {
	deps = withDefaults(deps)
	inv, err := LoadInventory(ctx, deps)
	if err != nil {
		return nil, err
	}
	groups := inv.EligibleGroups()
	for _, g := range groups {
		classify(ctx, deps, g, nil, nil, Options{})
	}
	groupByRow := map[string]*Group{}
	for _, g := range groups {
		for _, r := range g.Rows {
			groupByRow[r.CompanyID] = g
		}
	}

	orgs := lookupOrgs(ctx, deps, inv)
	res := &AuditResult{Tiers: map[string]int{}, Routes: map[Route]int{}}
	byName := map[string][]*Row{}
	for _, row := range inv.Rows {
		ar := &AuditRow{Row: row, Shape: ShapeOf(row.ExternalID)}
		if o := orgs[row.ExternalID]; o != nil {
			ar.OrgStatus = o.status
			if o.org != nil {
				ar.Website = o.org.Website
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
		byName[canonicalEntity(row.CompanyName)] = append(byName[canonicalEntity(row.CompanyName)], row)
	}
	res.Groups = groups

	for _, g := range groups {
		if g.Duplicate {
			res.Duplicates = append(res.Duplicates, g.Rows)
		}
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		rows := byName[n]
		if n == "" || len(rows) < 2 || !distinctExternalIDs(rows) {
			continue
		}
		res.Duplicates = append(res.Duplicates, rows)
	}

	if deps.ECLAs != nil {
		for _, ar := range res.Rows {
			if ar.Row.ActiveCCLA && (ar.Route == RouteManual || ar.Duplicate || ar.unresolvable()) {
				if n, cErr := deps.ECLAs.CountECLAs(ctx, ar.Row.CompanyID); cErr == nil {
					ar.ECLACount = n
				} else {
					ar.ECLACount = -1
				}
			}
		}
	}

	if opts.OutDir != "" {
		if err = writeAuditReports(opts.OutDir, res); err != nil {
			return nil, err
		}
	}
	fmt.Fprintf(deps.Out, "audit stage=%s companies=%d eligible_groups=%d", opts.Stage, len(res.Rows), len(groups))
	for _, t := range []string{TierMissing, TierInvalid, TierOK, TierDangling, TierUnknown} {
		fmt.Fprintf(deps.Out, " %s=%d", t, res.Tiers[t])
	}
	fmt.Fprintf(deps.Out, " register=%d rewrite=%d manual=%d duplicates=%d\n", res.Routes[RouteRegister], res.Routes[RouteRewrite], res.Routes[RouteManual], len(res.Duplicates))
	return res, nil
}

func (ar *AuditRow) unresolvable() bool {
	return ar.Shape == ShapeEmpty || ar.Shape == ShapeOther || (ar.Shape == ShapeSFID && ar.OrgStatus == "404")
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
	rows := [][]string{{"company_id", "company_name", "signing_entity_name", "company_external_id", "id_shape", "active_ccla", "ccla_count", "ecla_count", "org_service", "website", "duplicate_sfid_group", "route", "manual_reason", "tier"}}
	var unresolvable [][]string
	unresolvable = append(unresolvable, []string{"company_id", "company_name", "signing_entity_name", "company_external_id", "id_shape", "ccla_count", "ecla_count", "org_service", "manual_reason"})
	for _, ar := range res.Rows {
		r := ar.Row
		rows = append(rows, []string{r.CompanyID, r.CompanyName, r.SigningEntityName, r.ExternalID, string(ar.Shape), strconv.FormatBool(r.ActiveCCLA), strconv.Itoa(r.CCLACount), strconv.Itoa(ar.ECLACount), ar.OrgStatus, ar.Website, strconv.FormatBool(ar.Duplicate), string(ar.Route), ar.ManualReason, ar.Tier})
		if r.ActiveCCLA && ar.unresolvable() {
			unresolvable = append(unresolvable, []string{r.CompanyID, r.CompanyName, r.SigningEntityName, r.ExternalID, string(ar.Shape), strconv.Itoa(r.CCLACount), strconv.Itoa(ar.ECLACount), ar.OrgStatus, ar.ManualReason})
		}
	}
	dups := [][]string{{"group", "company_id", "company_name", "signing_entity_name", "company_external_id", "active_ccla", "ccla_count"}}
	for i, set := range res.Duplicates {
		for _, r := range set {
			dups = append(dups, []string{strconv.Itoa(i + 1), r.CompanyID, r.CompanyName, r.SigningEntityName, r.ExternalID, strconv.FormatBool(r.ActiveCCLA), strconv.Itoa(r.CCLACount)})
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
	rows := [][]string{{"key", "old_id", "id_shape", "route", "manual_reason", "live", "org_service", "website", "domain", "shared_domain", "new_id", "action", "decision", "reviewer", "company_ids", "company_names", "error"}}
	toSF := [][]string{{"old_id", "name", "website", "ccla_signed_date", "domain", "shared_domain"}}
	manual := [][]string{{"key", "old_id", "route", "reason", "suggested_action", "live", "org_service", "website", "domain", "shared_domain", "new_id", "action", "company_ids", "company_names", "error"}}
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
		rows = append(rows, []string{g.Key, g.OldID, string(g.Shape), string(g.Route), g.ManualReason, g.Live, g.OrgStatus, g.Website(), domain, shared, g.NewID, g.Action, decision, reviewer, strings.Join(g.CompanyIDs(), ";"), names, errText})
		if g.ManualReason == ReasonNoMapping || g.ManualReason == ReasonDeadAccount {
			req := apexRequest(g, true)
			toSF = append(toSF, []string{g.OldID, req.Name, req.Website, req.CCLASignedDate, domain, shared})
		}
	}
	for _, a := range plan.ManualActions() {
		g := a.Group
		errText := ""
		if g.Err != nil {
			errText = g.Err.Error()
		}
		domain, shared := plan.domainOf(g)
		manual = append(manual, []string{g.Key, g.OldID, string(g.Route), a.Reason, a.Suggested, g.Live, g.OrgStatus, g.Website(), domain, shared, g.NewID, g.Action, strings.Join(g.CompanyIDs(), ";"), strings.Join(g.Names(), ";"), errText})
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
