// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	acs_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
)

const (
	objectTypeOrganization        = "organization"
	objectTypeProjectOrganization = "project|organization"
)

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

// copyGrants creates on the new org every grant the old org has (step 6); existing ones are kept.
func (r *runner) copyGrants(ctx context.Context, g *Group) error {
	oldGrants, have, err := r.listGrantPair(ctx, g)
	if err != nil {
		return err
	}
	created := 0
	for _, gr := range oldGrants {
		ok, createErr := r.ensureGrant(ctx, g, gr, have)
		if createErr != nil {
			return createErr
		}
		if ok {
			created++
		}
	}
	fmt.Fprintf(r.deps.Out, "grants %s -> %s: %d existing, %d created\n", g.OldID, g.NewID, len(oldGrants)-created, created)
	return nil
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
// a username cannot be copied and fails the step so that it is never deleted uncopied.
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
	objectType, objectID := objectTypeOrganization, g.NewID
	if p := gr.ProjectSFID(); p != "" {
		objectType, objectID = objectTypeProjectOrganization, p+"|"+g.NewID
	}
	if err := r.deps.Orgs.CreateUserRoleScope(ctx, gr.Username, g.NewID, objectType, objectID, gr.RoleID); err != nil {
		return false, fmt.Errorf("granting %s %s on %s: %w", gr.RoleName, gr.Username, objectID, err)
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

// deleteOldGrants removes the old org's grants (step 8) after making sure each one exists on the new
// org (a grant added since step 6 is copied first); a missing grant is success.
func (r *runner) deleteOldGrants(ctx context.Context, g *Group) error {
	oldGrants, have, err := r.listGrantPair(ctx, g)
	if err != nil {
		return err
	}
	sort.Slice(oldGrants, func(i, j int) bool { return oldGrants[i].GrantID < oldGrants[j].GrantID })
	deleted, copied := 0, 0
	for _, gr := range oldGrants {
		if gr.GrantID == "" || gr.RoleID == "" {
			continue
		}
		created, ensureErr := r.ensureGrant(ctx, g, gr, have)
		if ensureErr != nil {
			return ensureErr
		}
		if created {
			copied++
		}
		if err = r.deps.Orgs.DeleteUserRoleScope(ctx, g.OldID, gr.RoleID, gr.GrantID, gr.Username); err != nil {
			return fmt.Errorf("deleting %s grant %s of %s: %w", gr.RoleName, gr.GrantID, gr.Username, err)
		}
		deleted++
	}
	fmt.Fprintf(r.deps.Out, "grants %s: %d deleted (%d copied late)\n", g.OldID, deleted, copied)
	return nil
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
	return nil
}
