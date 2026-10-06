// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

const (
	maxSuggestions = 3

	verifiedOK      = "ok"
	verifiedDropped = "dropped"
	verifiedUnknown = "?"
)

// knownAccount is an Account served by org-service under a Salesforce id already present in EasyCLA.
type knownAccount struct {
	ID   string
	Name string
}

// accountIndex finds known Accounts by matching key (website domain) and by normalized name.
type accountIndex struct {
	byKey    map[string]*knownAccount
	byDomain map[string][]*knownAccount
	byName   map[string][]*knownAccount
}

func newAccountIndex() *accountIndex {
	return &accountIndex{byKey: map[string]*knownAccount{}, byDomain: map[string][]*knownAccount{}, byName: map[string][]*knownAccount{}}
}

func (ix *accountIndex) add(shared SharedDomains, id string, org *Org, rowNames ...string) {
	if org == nil || ShapeOf(id) != ShapeSFID {
		return
	}
	key := accountKey(id)
	acct := ix.byKey[key]
	if acct == nil {
		acct = &knownAccount{ID: id, Name: firstNonEmpty(strings.TrimSpace(org.Name), firstNonEmpty(rowNames...))}
		ix.byKey[key] = acct
	}
	if d, gate := shared.Shared(org.Website); gate == "" {
		ix.byDomain[d] = appendAccount(ix.byDomain[d], acct)
	}
	for _, n := range append([]string{org.Name}, rowNames...) {
		if c := canonicalEntity(n); c != "" {
			ix.byName[c] = appendAccount(ix.byName[c], acct)
		}
	}
}

func appendAccount(list []*knownAccount, acct *knownAccount) []*knownAccount {
	for _, a := range list {
		if a == acct {
			return list
		}
	}
	return append(list, acct)
}

// knownAccountsFromGroups indexes the Accounts org-service served for the plan's groups.
func knownAccountsFromGroups(shared SharedDomains, groups []*Group) *accountIndex {
	ix := newAccountIndex()
	for _, g := range groups {
		ix.add(shared, g.OldID, g.Org, g.Names()...)
	}
	return ix
}

// suggester computes the advisory suggested_account cell: EasyCLA-internal matches first, then
// org-service lookups, each verified through member-service when possible. Never fatal.
type suggester struct {
	deps     Deps
	shared   SharedDomains
	ix       *accountIndex
	mu       sync.Mutex
	crm      map[string]*Org
	verified map[string]string
	warned   map[string]bool
}

func newSuggester(deps Deps, shared SharedDomains, ix *accountIndex) *suggester {
	return &suggester{deps: deps, shared: shared, ix: ix, crm: map[string]*Org{}, verified: map[string]string{}, warned: map[string]bool{}}
}

type candidate struct {
	id, name, how string
}

// suggest returns the cell for one row/group: ownKey is the accountKey of its own id (never
// suggested), website and names are what it is matched by.
func (s *suggester) suggest(ctx context.Context, ownKey, website string, names []string) string {
	domain, gate := s.shared.Shared(website)
	if gate != "" {
		domain = ""
	}
	canon := map[string]bool{}
	var display []string
	for _, n := range names {
		if c := canonicalEntity(n); c != "" && !canon[c] {
			canon[c] = true
			display = append(display, strings.TrimSpace(n))
		}
	}
	var cands []candidate
	if domain != "" {
		for _, a := range s.ix.byDomain[domain] {
			cands = append(cands, candidate{a.ID, a.Name, "inventory:domain"})
		}
	}
	for _, n := range display {
		for _, a := range s.ix.byName[canonicalEntity(n)] {
			cands = append(cands, candidate{a.ID, a.Name, "inventory:name"})
		}
	}
	if s.deps.Lookup != nil {
		if domain != "" {
			if o := s.lookup(ctx, "", domain); o != nil {
				cands = append(cands, candidate{o.ID, o.Name, "crm:domain"})
			}
		}
		for _, n := range display {
			if o := s.lookup(ctx, n, ""); o != nil {
				cands = append(cands, candidate{o.ID, o.Name, "crm:name"})
			}
		}
	}
	seen := map[string]bool{}
	var cells []string
	for _, c := range cands {
		key := accountKey(c.id)
		if ShapeOf(c.id) != ShapeSFID || key == ownKey || seen[key] {
			continue
		}
		seen[key] = true
		switch s.verify(ctx, c.id) {
		case verifiedDropped:
			continue
		case verifiedUnknown:
			c.how += verifiedUnknown
		}
		cells = append(cells, strings.TrimSpace(fmt.Sprintf("%s %s", c.id, c.name))+" ["+c.how+"]")
		if len(cells) == maxSuggestions {
			break
		}
	}
	return strings.Join(cells, "; ")
}

// lookup asks org-service for one Account by name or by domain (cached; one warning per key).
func (s *suggester) lookup(ctx context.Context, name, domain string) *Org {
	key := "name:" + canonicalEntity(name)
	if domain != "" {
		key = "domain:" + domain
	}
	s.mu.Lock()
	o, ok := s.crm[key]
	s.mu.Unlock()
	if ok {
		return o
	}
	o, err := s.deps.Lookup.LookupOrganization(ctx, name, domain)
	if err != nil {
		o = nil
		if !errors.Is(err, ErrOrgNotFound) {
			s.warn(key, fmt.Sprintf("WARNING: account lookup %s failed: %v\n", key, err))
		}
	} else if o != nil && ShapeOf(o.ID) != ShapeSFID {
		o = nil
	}
	s.mu.Lock()
	s.crm[key] = o
	s.mu.Unlock()
	return o
}

// verify checks a candidate in member-service: "ok" (200), "dropped" (404) or "?" (403, error, no client).
func (s *suggester) verify(ctx context.Context, id string) string {
	key := accountKey(id)
	s.mu.Lock()
	v, ok := s.verified[key]
	s.mu.Unlock()
	if ok {
		return v
	}
	v = verifiedUnknown
	if s.deps.Members != nil {
		_, err := s.deps.Members.GetB2BOrg(ctx, id)
		switch {
		case err == nil:
			v = verifiedOK
		case errors.Is(err, member_service.ErrOrgNotFound):
			v = verifiedDropped
		default:
			var authErr *member_service.AuthError
			if !(errors.As(err, &authErr) && authErr.Status == 403 && !authErr.Token) {
				s.warn("verify:"+key, fmt.Sprintf("WARNING: account check %s failed: %v\n", id, err))
			}
		}
	}
	s.mu.Lock()
	s.verified[key] = v
	s.mu.Unlock()
	return v
}

func (s *suggester) warn(key, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.warned[key] {
		return
	}
	s.warned[key] = true
	fmt.Fprint(s.deps.Out, msg)
}

// suggestAccounts fills Group.Suggested for the groups fill selects (4 workers).
func suggestAccounts(ctx context.Context, deps Deps, shared SharedDomains, ix *accountIndex, groups []*Group, fill func(*Group) bool) {
	s := newSuggester(deps, shared, ix)
	var selected []*Group
	for _, g := range groups {
		if fill(g) {
			selected = append(selected, g)
		}
	}
	runWorkers(len(selected), func(i int) {
		g := selected[i]
		names := g.Names()
		if g.Org != nil {
			names = append(names, g.Org.Name)
		}
		g.Suggested = s.suggest(ctx, accountKey(g.OldID), g.Website(), names)
	})
}

// suggestRows fills AuditRow.Suggested for the rows fill selects (4 workers).
func suggestRows(ctx context.Context, deps Deps, shared SharedDomains, ix *accountIndex, rows []*AuditRow, fill func(*AuditRow) bool) {
	s := newSuggester(deps, shared, ix)
	var selected []*AuditRow
	for _, ar := range rows {
		if fill(ar) {
			selected = append(selected, ar)
		}
	}
	runWorkers(len(selected), func(i int) {
		ar := selected[i]
		names := []string{ar.Row.CompanyName}
		if ar.OrgName != "" {
			names = append(names, ar.OrgName)
		}
		ar.Suggested = s.suggest(ctx, accountKey(ar.Row.ExternalID), ar.Website, names)
	})
}

// runWorkers runs fn(0..n-1) on four goroutines.
func runWorkers(n int, fn func(i int)) {
	work := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				fn(i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		work <- i
	}
	close(work)
	wg.Wait()
}

// acsRoles formats ACS grants as role=count;... sorted by role name; "err" when the listing failed.
func acsRoles(ctx context.Context, acs ACSService, id string) string {
	grants, err := acs.ListOrgGrants(ctx, id)
	if err != nil {
		return orgStatusErr
	}
	counts := map[string]int{}
	for _, g := range grants {
		counts[firstNonEmpty(g.RoleName, g.RoleID)]++
	}
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s=%d", n, counts[n]))
	}
	return strings.Join(parts, ";")
}
