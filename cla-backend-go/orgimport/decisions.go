// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Decision kinds recorded by the duplicate review (lfx-self-serve #3085).
const (
	DecisionCollapse = "collapse"
	DecisionDistinct = "distinct"
)

// Decision is one reviewed group: collapse (all old ids land on TargetSFID) or distinct (separate Accounts).
type Decision struct {
	Kind     string
	OldIDs   []string
	Target   string
	Reviewer string
	Note     string
}

// Decisions indexes the review file by old id.
type Decisions struct {
	ByOldID map[string]*Decision
	List    []*Decision
}

// LoadDecisions reads and validates the decisions CSV (header required).
func LoadDecisions(path string) (*Decisions, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ParseDecisions(f)
}

// ParseDecisions parses `decision,old_ids,target_sfid,reviewer,note` rows; old_ids are separated by `;` or spaces.
func ParseDecisions(r io.Reader) (*Decisions, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("decisions: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("decisions: empty file")
	}
	col := map[string]int{}
	for i, h := range records[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, name := range []string{"decision", "old_ids", "target_sfid", "reviewer"} {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("decisions: missing column %q (need decision,old_ids,target_sfid,reviewer[,note])", name)
		}
	}
	field := func(rec []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}
	d := &Decisions{ByOldID: map[string]*Decision{}}
	for n, rec := range records[1:] {
		line := n + 2
		if len(rec) == 0 || (len(rec) == 1 && strings.TrimSpace(rec[0]) == "") {
			continue
		}
		dec := &Decision{Kind: strings.ToLower(field(rec, "decision")), Target: field(rec, "target_sfid"), Reviewer: field(rec, "reviewer"), Note: field(rec, "note")}
		dec.OldIDs = append(dec.OldIDs, strings.FieldsFunc(field(rec, "old_ids"), func(r rune) bool { return r == ';' || r == ' ' || r == '\t' })...)
		if dec.Reviewer == "" {
			return nil, fmt.Errorf("decisions line %d: reviewer is required", line)
		}
		if len(dec.OldIDs) == 0 {
			return nil, fmt.Errorf("decisions line %d: old_ids is empty", line)
		}
		switch dec.Kind {
		case DecisionCollapse:
			if !IsSFID(dec.Target) {
				return nil, fmt.Errorf("decisions line %d: collapse needs a Salesforce account id in target_sfid", line)
			}
		case DecisionDistinct:
			if dec.Target != "" {
				return nil, fmt.Errorf("decisions line %d: distinct must not set target_sfid", line)
			}
			if len(dec.OldIDs) < 2 {
				return nil, fmt.Errorf("decisions line %d: distinct needs at least two old ids", line)
			}
		default:
			return nil, fmt.Errorf("decisions line %d: decision must be collapse|distinct", line)
		}
		for _, id := range dec.OldIDs {
			if prev, dup := d.ByOldID[id]; dup {
				return nil, fmt.Errorf("decisions line %d: %s already appears in a %s decision", line, id, prev.Kind)
			}
			d.ByOldID[id] = dec
		}
		d.List = append(d.List, dec)
	}
	return d, nil
}

// Collapse reports whether a recorded collapse decision lets oldID land on target.
func (d *Decisions) Collapse(oldID, target string) *Decision {
	if d == nil {
		return nil
	}
	dec := d.ByOldID[oldID]
	if dec != nil && dec.Kind == DecisionCollapse && accountKey(dec.Target) == accountKey(target) {
		return dec
	}
	return nil
}

// Distinct reports whether a recorded distinct decision separates oldID from any id in others.
func (d *Decisions) Distinct(oldID string, others []string) *Decision {
	if d == nil {
		return nil
	}
	dec := d.ByOldID[oldID]
	if dec == nil || dec.Kind != DecisionDistinct {
		return nil
	}
	for _, o := range others {
		if o != oldID && d.ByOldID[o] == dec {
			return dec
		}
	}
	return nil
}

// Target groups the plan by destination Account (dry-run report of design §5 step 3).
type Target struct {
	SFID     string
	Groups   []*Group
	Existing []*Row
	Mapped   []string
	Decision *Decision
	Status   string
}

// OldIDs lists the old ids landing on the target: tranche groups, rows already carrying it and
// mapping rows outside the tranche.
func (t *Target) OldIDs() []string {
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, g := range t.Groups {
		add(g.Key)
	}
	for _, r := range t.Existing {
		add(r.ExternalID)
	}
	for _, id := range t.Mapped {
		add(id)
	}
	return ids
}

// forms lists the exact id strings (15- or 18-char) that would end up on the target.
func (t *Target) forms(mapping *Mapping) []string {
	var forms []string
	add := func(id string) {
		if id != "" && !containsString(forms, id) {
			forms = append(forms, id)
		}
	}
	for _, g := range t.Groups {
		add(g.NewID)
	}
	for _, r := range t.Existing {
		add(r.ExternalID)
	}
	if mapping != nil {
		for _, id := range t.Mapped {
			add(mapping.Rows[id].NewID)
		}
	}
	return forms
}

// applyDecisions is the approval-aware collision pass: a rewrite group may share its destination with
// other groups, other mapping rows or rows already carrying the id only under a recorded collapse decision.
// Destinations are compared per Account (15- and 18-char forms of one id are the same target).
func applyDecisions(groups []*Group, inv *Inventory, mapping *Mapping, decisions *Decisions) []*Target {
	byTarget := map[string]*Target{}
	for _, g := range groups {
		if g.Route != RouteRewrite || g.NewID == "" || g.Err != nil {
			continue
		}
		t := byTarget[accountKey(g.NewID)]
		if t == nil {
			t = &Target{SFID: g.NewID}
			t.Existing = append(t.Existing, inv.rowsCarrying(g.NewID)...)
			byTarget[accountKey(g.NewID)] = t
		}
		t.Groups = append(t.Groups, g)
	}
	targets := make([]*Target, 0, len(byTarget))
	for _, t := range byTarget {
		targets = append(targets, t)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].SFID < targets[j].SFID })
	for _, t := range targets {
		t.Status = statusOK
		if mapping != nil {
			seen := t.OldIDs()
			for oldID, row := range mapping.Rows {
				if row.NewID != "" && accountKey(row.NewID) == accountKey(t.SFID) && !containsString(seen, oldID) {
					t.Mapped = append(t.Mapped, oldID)
				}
			}
			sort.Strings(t.Mapped)
		}
		ids := t.OldIDs()
		if len(ids) < 2 {
			continue
		}
		if forms := t.forms(mapping); len(forms) > 1 {
			// the tool never writes a second form of one Account id: the mapping (or the existing rows) must agree first
			for _, g := range t.Groups {
				if !g.Replayed {
					g.Route, g.ManualReason = RouteManual, ReasonTargetFormsDiffer
				}
			}
			t.Status = ReasonTargetFormsDiffer
			continue
		}
		for _, g := range t.Groups {
			if g.Replayed {
				continue
			}
			if dec := decisions.Distinct(g.Key, ids); dec != nil {
				g.Route, g.ManualReason = RouteManual, ReasonDistinctConflict
				t.Decision, t.Status = dec, ReasonDistinctConflict
				continue
			}
			dec := decisions.Collapse(g.Key, t.SFID)
			if dec == nil {
				g.Route, g.ManualReason = RouteManual, "target_collision"
				if t.Status == statusOK {
					t.Status = "needs_decision"
				}
				continue
			}
			g.Decision, t.Decision = dec, dec
		}
	}
	return targets
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// SharedDomains is the list of domains that never match an Account automatically (design §4).
type SharedDomains map[string]bool

// DefaultSharedDomains is the built-in list; the #3085 review file replaces it (--shared-domains).
var DefaultSharedDomains = []string{
	"github.com", "nowebsite.com",
	"gmail.com", "googlemail.com", "yahoo.com", "hotmail.com", "outlook.com", "live.com", "icloud.com", "protonmail.com", "qq.com", "163.com",
}

// LoadSharedDomains reads one domain per line (`#` comments); an empty path yields the default list.
func LoadSharedDomains(path string) (SharedDomains, error) {
	set := SharedDomains{}
	if path == "" {
		for _, d := range DefaultSharedDomains {
			set[d] = true
		}
		return set, nil
	}
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		set[normalizeDomain(line)] = true
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("shared domains: %w", err)
	}
	return set, nil
}

// Domain extracts the registrable host of a website value (scheme, path and www. stripped).
func Domain(website string) string {
	website = strings.TrimSpace(strings.ToLower(website))
	if website == "" {
		return ""
	}
	if !strings.Contains(website, "://") {
		website = "http://" + website
	}
	u, err := url.Parse(website)
	if err != nil || u.Hostname() == "" {
		return normalizeDomain(website)
	}
	return normalizeDomain(u.Hostname())
}

func normalizeDomain(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	if i := strings.IndexAny(host, "/:?"); i >= 0 {
		host = host[:i]
	}
	host = strings.TrimSuffix(host, ".")
	return strings.TrimPrefix(host, "www.")
}

// Shared reports whether website has no domain or a listed shared domain; the reason is the manual reason.
func (s SharedDomains) Shared(website string) (string, string) {
	d := Domain(website)
	switch {
	case d == "":
		return "", "missing_website"
	case s[d]:
		return d, ReasonSharedDomain
	}
	return d, ""
}
