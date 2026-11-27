// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	acs_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
)

const (
	objectTypeOrganization        = "organization"
	objectTypeProjectOrganization = "project|organization"
)

// orgAssociated is how ACS refuses an organization scope grant to a user who holds one in another organization
const orgAssociated = "already associated with other organization"

// userGone is how the organization service refuses a grant for a user who no longer exists on the platform
const userGone = "user does not exist on platform"

const (
	stepRetries    = 2
	stepRetryDelay = 10 * time.Second
)

// acsListFailed starts the error of a failed ACS grants listing (acs_service.ListOrgGrants), which is no *runtime.APIError
const acsListFailed = "acs rolescopes/organization returned "

// gatewayError tells a 502, 503 or 504 answer of a platform service: the step is tried again before the group fails
func gatewayError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *runtime.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == http.StatusBadGateway || apiErr.Code == http.StatusServiceUnavailable || apiErr.Code == http.StatusGatewayTimeout
	}
	for _, code := range []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		if strings.Contains(err.Error(), acsListFailed+strconv.Itoa(code)+":") {
			return true
		}
	}
	return false
}

// easyCLARoles are the only roles copied to an Account that existed before the rewrite (lfx-self-serve #5081).
var easyCLARoles = map[string]bool{"cla-manager": true, "cla-manager-designee": true, "cla-signatory": true, "contributor": true, "contact": true}

// rewrite runs steps 6-10 for one group; every step is idempotent so a restart converges. The
// journal is written before the first step and after each one; a journal failure stops the group.
func (r *runner) rewrite(ctx context.Context, g *Group) error {
	steps := []struct {
		name string
		fn   func(context.Context, *Group) error
	}{
		{StepGrants, r.copyGrants},
		{StepRows, r.rewriteRows},
		{StepEvents, r.rekeyEvents},
		{StepCleanup, r.deleteOldGrants},
		{StepRegister, r.registerNew},
	}
	if g.RowTargeted() {
		steps = steps[1:2:2]
		steps = append(steps, struct {
			name string
			fn   func(context.Context, *Group) error
		}{StepRegister, r.registerNew})
		fmt.Fprintf(r.deps.Out, "row %s: company_external_id %q is not an organization id; grants and events are not moved\n", g.Key, g.OldID)
	}
	if err := r.record(g, StepStart, statusOK, nil); err != nil {
		return err
	}
	for _, s := range steps {
		err := s.fn(ctx, g)
		for try := 1; try <= stepRetries && gatewayError(err); try++ {
			fmt.Fprintf(r.deps.Out, "%s %s: %v; trying the step again (%d of %d)\n", s.name, g.Key, err, try, stepRetries)
			if sleepErr := r.deps.Sleep(ctx, stepRetryDelay); sleepErr != nil {
				break
			}
			err = s.fn(ctx, g)
		}
		if recErr := r.record(g, s.name, status(err), err); recErr != nil {
			if err != nil {
				return fmt.Errorf("%s: %w (%v)", s.name, err, recErr)
			}
			return recErr
		}
		if err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
	}
	if err := r.record(g, StepDone, statusOK, nil); err != nil {
		return err
	}
	fmt.Fprintf(r.deps.Out, "rewritten %s -> %s (%d row(s))\n", g.Key, g.NewID, len(g.Rows))
	return nil
}

func grantKey(gr acs_service.OrgGrant, orgID string) string {
	object := orgID
	if p := gr.ProjectSFID(); p != "" {
		object = p + "|" + orgID
	}
	return strings.ToLower(gr.Username) + "|" + gr.RoleID + "|" + gr.ObjectTypeName + "|" + object
}

// copyGrants creates on the new org the old org's grants that the rewrite copies (step 6); existing ones are kept.
func (r *runner) copyGrants(ctx context.Context, g *Group) error {
	oldGrants, have, err := r.listGrantPair(ctx, g)
	if err != nil {
		return err
	}
	created, skipped := 0, 0
	for _, gr := range oldGrants {
		ok, createErr := r.ensureGrant(ctx, g, gr, have)
		if createErr != nil && !heldElsewhere(gr, createErr) && !goneUser(createErr) {
			return createErr
		}
		if ok {
			created++
		} else if uncopied(g, gr, have) {
			skipped++
			g.drop(gr)
		}
	}
	fmt.Fprintf(r.deps.Out, "grants %s -> %s: %d existing, %d created, %d not copied\n", g.OldID, g.NewID, len(oldGrants)-created-skipped, created, skipped)
	return nil
}

func uncopied(g *Group, gr acs_service.OrgGrant, have map[string]bool) bool {
	return gr.RoleID != "" && gr.GrantID != "" && !have[grantKey(gr, g.NewID)]
}

func (g *Group) drop(gr acs_service.OrgGrant) {
	for i, d := range g.Dropped {
		if d.GrantID == gr.GrantID {
			g.Dropped[i] = gr
			return
		}
	}
	g.Dropped = append(g.Dropped, gr)
}

func (g *Group) isDropped(gr acs_service.OrgGrant) bool {
	for _, d := range g.Dropped {
		if d.GrantID == gr.GrantID {
			return true
		}
	}
	return false
}

func (g *Group) undrop(gr acs_service.OrgGrant) {
	for i, d := range g.Dropped {
		if d.GrantID == gr.GrantID {
			g.Dropped = append(g.Dropped[:i:i], g.Dropped[i+1:]...)
			return
		}
	}
}

// droppedForJournal returns the list without usernames: the journal and the log are published by the sweep workflow.
func (g *Group) droppedForJournal() []acs_service.OrgGrant {
	if len(g.Dropped) == 0 {
		return nil
	}
	out := make([]acs_service.OrgGrant, len(g.Dropped))
	for i, d := range g.Dropped {
		d.Username = ""
		out[i] = d
	}
	return out
}

// listGrantPair lists the old org's grants and indexes the new org's grants by grantKey.
func (r *runner) listGrantPair(ctx context.Context, g *Group) ([]acs_service.OrgGrant, map[string]bool, error) {
	oldGrants, err := r.deps.ACS.ListOrgGrants(ctx, g.OldID)
	if err != nil {
		return nil, nil, fmt.Errorf("listing grants of %s: %w", g.OldID, err)
	}
	newGrants, err := r.deps.ACS.ListOrgGrants(ctx, g.NewID)
	if err != nil {
		return nil, nil, fmt.Errorf("listing grants of %s: %w", g.NewID, err)
	}
	have := map[string]bool{}
	for _, gr := range newGrants {
		have[grantKey(gr, g.NewID)] = true
	}
	return oldGrants, have, nil
}

// ensureGrant creates the old grant's counterpart on the new org unless it already exists (a grant
// without a role is unusable and skipped); it reports whether a grant was created. A grant without
// a username - or, when only the EasyCLA roles are copied, without a role name - fails the step so that it is never deleted uncopied.
func (r *runner) ensureGrant(ctx context.Context, g *Group, gr acs_service.OrgGrant, have map[string]bool) (bool, error) {
	if gr.RoleID == "" {
		return false, nil
	}
	if gr.Username == "" {
		return false, fmt.Errorf("grant %s (%s) on %s has no username: cannot be copied to %s", gr.GrantID, gr.RoleName, g.OldID, g.NewID)
	}
	key := grantKey(gr, g.NewID)
	if have[key] {
		return false, nil
	}
	if !g.AllGrants && !easyCLARoles[gr.RoleName] {
		if gr.RoleName == "" {
			return false, fmt.Errorf("grant %s (role %s) on %s has no role name: cannot tell whether it is copied to %s", gr.GrantID, gr.RoleID, g.OldID, g.NewID)
		}
		return false, nil
	}
	objectType, objectID := objectTypeOrganization, g.NewID
	if p := gr.ProjectSFID(); p != "" {
		objectType, objectID = objectTypeProjectOrganization, p+"|"+g.NewID
	}
	if err := r.deps.Orgs.CreateUserRoleScope(ctx, gr.Username, g.NewID, objectType, objectID, gr.RoleID); err != nil {
		return false, fmt.Errorf("granting %s (grant %s) on %s: %w", gr.RoleName, gr.GrantID, objectID, nameless(err, gr))
	}
	have[key] = true
	return true, nil
}

// rewriteRows swaps company_external_id on every row of the group (step 7, CAS + read-back).
func (r *runner) rewriteRows(ctx context.Context, g *Group) error {
	for _, row := range g.Rows {
		old := g.pinnedOld(row)
		err := r.deps.Companies.UpdateCompanyExternalID(ctx, row.CompanyID, old, g.NewID)
		if err == nil {
			row.ExternalID, row.RawExternalID = g.NewID, g.NewID
			continue
		}
		if !errors.Is(err, company.ErrExternalIDConditionFailed) {
			return fmt.Errorf("company %s: %w", row.CompanyID, err)
		}
		rec, readErr := r.deps.Companies.GetCompanyRecord(ctx, row.CompanyID)
		if readErr != nil {
			return fmt.Errorf("company %s: reading back after condition failure: %w", row.CompanyID, readErr)
		}
		if rec.CompanyExternalID == g.NewID && (strings.TrimSpace(rec.PreviousCompanyExternalID) == strings.TrimSpace(old) || strings.TrimSpace(old) == "") {
			row.ExternalID, row.RawExternalID = g.NewID, g.NewID
			continue
		}
		return fmt.Errorf("company %s: external id is %q (previous %q), expected %q or %s: conflict, group stopped",
			row.CompanyID, rec.CompanyExternalID, rec.PreviousCompanyExternalID, old, g.NewID)
	}
	return nil
}

// rekeyEvents moves the activity history (step 7b): union of the org-wide and per-CLA-group listings.
func (r *runner) rekeyEvents(ctx context.Context, g *Group) error {
	if r.deps.Events == nil {
		fmt.Fprintf(r.deps.Out, "events %s -> %s: skipped (no events repository)\n", g.OldID, g.NewID)
		return nil
	}
	listed, rekeyed, err := r.rekeyListed(ctx, g)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.deps.Out, "events %s -> %s: %d listed, %d re-keyed\n", g.OldID, g.NewID, listed, rekeyed)
	return nil
}

// listOldEvents returns the distinct ids of the events still keyed by the group's old id.
func (r *runner) listOldEvents(ctx context.Context, g *Group) ([]string, error) {
	ids, err := r.deps.Events.ListEventIDsByCompanySFID(ctx, g.OldID)
	if err != nil {
		return nil, fmt.Errorf("listing events of %s: %w", g.OldID, err)
	}
	for _, claGroupID := range g.CLAGroupIDs() {
		more, listErr := r.deps.Events.ListEventIDsByCompanySFIDCLAGroup(ctx, g.OldID, claGroupID)
		if listErr != nil {
			return nil, fmt.Errorf("listing events of %s/%s: %w", g.OldID, claGroupID, listErr)
		}
		ids = append(ids, more...)
	}
	return distinct(ids), nil
}

// rekeyListed re-keys every listed event and returns the listed and re-keyed counts.
func (r *runner) rekeyListed(ctx context.Context, g *Group) (listed, rekeyed int, err error) {
	ids, err := r.listOldEvents(ctx, g)
	if err != nil {
		return 0, 0, err
	}
	for _, id := range ids {
		done, rekeyErr := r.deps.Events.RekeyEventCompanySFID(ctx, id, g.OldID, g.NewID)
		if rekeyErr != nil {
			return len(ids), rekeyed, fmt.Errorf("event %s: %w", id, rekeyErr)
		}
		if done {
			rekeyed++
		}
	}
	return len(ids), rekeyed, nil
}

// deleteOldGrants removes the old org's grants (step 8) after making sure each copied one exists on the
// new org (a grant added since step 6 is copied first); a missing grant is success.
func (r *runner) deleteOldGrants(ctx context.Context, g *Group) error {
	oldGrants, have, err := r.listGrantPair(ctx, g)
	if err != nil {
		return err
	}
	sort.Slice(oldGrants, func(i, j int) bool { return oldGrants[i].GrantID < oldGrants[j].GrantID })
	deleted, copied := 0, 0
	var held []acs_service.OrgGrant
	// restoreHeld puts the grants still waiting for their move back on the old id, so that the next run finds them
	restoreHeld := func(cause error) error {
		for _, gr := range held {
			if restoreErr := r.deps.Orgs.CreateUserRoleScope(ctx, gr.Username, g.OldID, objectTypeOrganization, g.OldID, gr.RoleID); restoreErr != nil {
				cause = fmt.Errorf("%w; restoring %s grant %s on %s: %v", cause, firstNonEmpty(gr.RoleName, gr.RoleID), gr.GrantID, g.OldID, nameless(restoreErr, gr))
				continue
			}
			g.undrop(gr)
		}
		return cause
	}
	for _, gr := range oldGrants {
		if gr.GrantID == "" || gr.RoleID == "" {
			continue
		}
		created, ensureErr := r.ensureGrant(ctx, g, gr, have)
		moving := heldElsewhere(gr, ensureErr)
		if ensureErr != nil && !moving && !goneUser(ensureErr) {
			return restoreHeld(ensureErr)
		}
		if created {
			copied++
		}
		if moving || uncopied(g, gr, have) {
			journaled := g.isDropped(gr)
			g.drop(gr)
			if !journaled {
				if recErr := r.record(g, StepCleanup, statusPartial, nil); recErr != nil {
					return restoreHeld(recErr)
				}
			}
		}
		if err = r.deps.Orgs.DeleteUserRoleScope(ctx, g.OldID, gr.RoleID, gr.GrantID, gr.Username); err != nil {
			switch {
			case moving:
				held = append(held, gr)
			case uncopied(g, gr, have):
				g.drop(gr)
			}
			return restoreHeld(fmt.Errorf("deleting %s grant %s: %w", gr.RoleName, gr.GrantID, nameless(err, gr)))
		}
		deleted++
		switch {
		case moving:
			held = append(held, gr)
		case uncopied(g, gr, have):
			g.dropUncopied(r, gr)
		default:
			g.undrop(gr)
		}
	}
	for len(held) > 0 {
		gr := held[0]
		created, ensureErr := r.ensureGrant(ctx, g, gr, have)
		refused := heldElsewhere(gr, ensureErr) || goneUser(ensureErr)
		if ensureErr != nil && !refused {
			return restoreHeld(ensureErr)
		}
		held = held[1:]
		if refused {
			if heldElsewhere(gr, ensureErr) {
				g.heldBack = append(g.heldBack, gr)
			}
			g.dropUncopied(r, gr)
			continue
		}
		if created {
			copied++
		}
		g.undrop(gr)
		if recErr := r.record(g, StepCleanup, statusPartial, nil); recErr != nil {
			return restoreHeld(recErr)
		}
	}
	fmt.Fprintf(r.deps.Out, "grants %s: %d deleted (%d copied late)\n", g.OldID, deleted, copied)
	return nil
}

// retryHeld tries once more the organization scope grants that ACS refused while their user held one in another
// organization: a later group of the run may have moved that one (several old ids collapsing onto one Account)
func (r *runner) retryHeld(ctx context.Context) (failed int) {
	for _, g := range r.plan.Rewrite {
		if g.Err != nil || len(g.heldBack) == 0 {
			continue
		}
		var newGrants []acs_service.OrgGrant
		err := r.again(ctx, g, func() (err error) {
			newGrants, err = r.deps.ACS.ListOrgGrants(ctx, g.NewID)
			return err
		})
		if err != nil {
			fmt.Fprintf(r.deps.Out, "WARNING: %d organization grant(s) of %s not tried again on %s: %v\n", len(g.heldBack), g.OldID, g.NewID, err)
			continue
		}
		have := map[string]bool{}
		for _, gr := range newGrants {
			have[grantKey(gr, g.NewID)] = true
		}
		moved := 0
		for _, gr := range g.heldBack {
			ensureErr := r.again(ctx, g, func() error {
				_, err := r.ensureGrant(ctx, g, gr, have)
				return err
			})
			if ensureErr != nil {
				if !heldElsewhere(gr, ensureErr) {
					fmt.Fprintf(r.deps.Out, "WARNING: %s grant %s of %s not copied to %s: %v\n", firstNonEmpty(gr.RoleName, gr.RoleID), gr.GrantID, g.OldID, g.NewID, ensureErr)
				}
				continue
			}
			g.undrop(gr)
			moved++
		}
		g.heldBack = nil
		if moved == 0 {
			continue
		}
		fmt.Fprintf(r.deps.Out, "grants %s -> %s: %d copied after the other organizations of the run\n", g.OldID, g.NewID, moved)
		if recErr := r.record(g, StepDone, statusOK, nil); recErr != nil {
			g.Err = fmt.Errorf("%d grant(s) of %s are on %s now but the journal still lists them as dropped: %w", moved, g.OldID, g.NewID, recErr)
			fmt.Fprintf(r.deps.Out, "FAILED rewrite %s -> %s: %v\n", g.Key, g.NewID, g.Err)
			failed++
		}
	}
	return failed
}

// again repeats a call of the end-of-run pass after a gateway error, as rewrite does for a step
func (r *runner) again(ctx context.Context, g *Group, call func() error) error {
	err := call()
	for try := 1; try <= stepRetries && gatewayError(err); try++ {
		fmt.Fprintf(r.deps.Out, "%s %s: %v; trying again (%d of %d)\n", StepCleanup, g.Key, err, try, stepRetries)
		if sleepErr := r.deps.Sleep(ctx, stepRetryDelay); sleepErr != nil {
			break
		}
		err = call()
	}
	return err
}

// RemovedName stands in for a username or an e-mail address in a text that is published
const RemovedName = "[removed]"

var emailAddress = regexp.MustCompile(`[A-Za-z0-9._%+'-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)

// WithoutEmails takes every e-mail address out of a text
func WithoutEmails(text string) string {
	return emailAddress.ReplaceAllString(text, RemovedName)
}

// WithoutName takes a name out of a text, as a whole word and in any letter case
func WithoutName(text, name string) string {
	if name == "" {
		return text
	}
	pattern := regexp.QuoteMeta(name)
	if wordCharacter(name[0]) {
		pattern = `\b` + pattern
	}
	if wordCharacter(name[len(name)-1]) {
		pattern += `\b`
	}
	return regexp.MustCompile(`(?i)`+pattern).ReplaceAllString(text, RemovedName)
}

func wordCharacter(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// namelessError is an upstream error whose text names no user: the journal and the reports are published
type namelessError struct {
	err  error
	text string
}

func (e *namelessError) Error() string { return e.text }

func (e *namelessError) Unwrap() error { return e.err }

// nameless takes the grant holder's username and every e-mail address out of the text of an upstream error
func nameless(err error, gr acs_service.OrgGrant) error {
	if err == nil {
		return nil
	}
	text := WithoutName(WithoutEmails(err.Error()), gr.Username)
	if text == err.Error() {
		return err
	}
	return &namelessError{err: err, text: text}
}

// upstreamText is the text of an error as the platform sent it
func upstreamText(err error) string {
	var hidden *namelessError
	if errors.As(err, &hidden) {
		return hidden.err.Error()
	}
	return err.Error()
}

// goneUser reports a grant that no organization can take any more: its user is gone, so it is dropped and reported
func goneUser(err error) bool {
	return err != nil && strings.Contains(upstreamText(err), userGone)
}

// heldElsewhere tells the ACS refusal of an organization scope grant whose user holds one in another organization
func heldElsewhere(gr acs_service.OrgGrant, err error) bool {
	return err != nil && gr.ProjectSFID() == "" && strings.Contains(upstreamText(err), orgAssociated)
}

func (g *Group) dropUncopied(r *runner, gr acs_service.OrgGrant) {
	g.drop(gr)
	fmt.Fprintf(r.deps.Out, "grant deleted without a copy %s -> %s: %s grant %s on %s\n", g.OldID, g.NewID, firstNonEmpty(gr.RoleName, gr.RoleID), gr.GrantID, gr.ObjectID)
}

// registerNew registers the new id as a B2B org (step 9).
func (r *runner) registerNew(ctx context.Context, g *Group) error {
	if r.deps.Members == nil {
		return ErrNotConfigured
	}
	org, err := r.deps.Members.RegisterB2BOrg(ctx, g.NewID)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.deps.Out, "registered %s as b2b_org %s (%s)\n", g.NewID, org.UID, org.Name)
	r.confirmRegistered(ctx, g.NewID)
	return nil
}
