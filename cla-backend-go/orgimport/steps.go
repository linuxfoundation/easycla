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

// rewrite runs steps 6-10 for one group; every step is idempotent so a restart converges.
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
	for _, s := range steps {
		err := s.fn(ctx, g)
		r.record(g, s.name, status(err), err)
		if err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
	}
	r.record(g, StepDone, statusOK, nil)
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
	oldGrants, err := r.deps.ACS.ListOrgGrants(ctx, g.OldID)
	if err != nil {
		return fmt.Errorf("listing grants of %s: %w", g.OldID, err)
	}
	newGrants, err := r.deps.ACS.ListOrgGrants(ctx, g.NewID)
	if err != nil {
		return fmt.Errorf("listing grants of %s: %w", g.NewID, err)
	}
	have := map[string]bool{}
	for _, gr := range newGrants {
		have[grantKey(gr, g.NewID)] = true
	}
	created := 0
	for _, gr := range oldGrants {
		if gr.Username == "" || gr.RoleID == "" {
			continue
		}
		key := grantKey(gr, g.NewID)
		if have[key] {
			continue
		}
		objectType, objectID := objectTypeOrganization, g.NewID
		if p := gr.ProjectSFID(); p != "" {
			objectType, objectID = objectTypeProjectOrganization, p+"|"+g.NewID
		}
		if err = r.deps.Orgs.CreateUserRoleScope(ctx, gr.Username, g.NewID, objectType, objectID, gr.RoleID); err != nil {
			return fmt.Errorf("granting %s %s on %s: %w", gr.RoleName, gr.Username, objectID, err)
		}
		have[key] = true
		created++
	}
	fmt.Fprintf(r.deps.Out, "grants %s -> %s: %d existing, %d created\n", g.OldID, g.NewID, len(oldGrants)-created, created)
	return nil
}

// rewriteRows swaps company_external_id on every row of the group (step 7, CAS + read-back).
func (r *runner) rewriteRows(ctx context.Context, g *Group) error {
	for _, row := range g.Rows {
		err := r.deps.Companies.UpdateCompanyExternalID(ctx, row.CompanyID, g.OldID, g.NewID)
		if err == nil {
			row.ExternalID = g.NewID
			continue
		}
		if !errors.Is(err, company.ErrExternalIDConditionFailed) {
			return fmt.Errorf("company %s: %w", row.CompanyID, err)
		}
		rec, readErr := r.deps.Companies.GetCompanyRecord(ctx, row.CompanyID)
		if readErr != nil {
			return fmt.Errorf("company %s: reading back after condition failure: %w", row.CompanyID, readErr)
		}
		if rec.CompanyExternalID == g.NewID && (rec.PreviousCompanyExternalID == g.OldID || g.OldID == "") {
			row.ExternalID = g.NewID
			continue
		}
		return fmt.Errorf("company %s: external id is %q (previous %q), expected %s or %s: conflict, group stopped",
			row.CompanyID, rec.CompanyExternalID, rec.PreviousCompanyExternalID, g.OldID, g.NewID)
	}
	return nil
}

// rekeyEvents moves the activity history (step 7b): union of the org-wide and per-CLA-group listings.
func (r *runner) rekeyEvents(ctx context.Context, g *Group) error {
	if r.deps.Events == nil {
		fmt.Fprintf(r.deps.Out, "events %s -> %s: skipped (no events repository)\n", g.OldID, g.NewID)
		return nil
	}
	ids, err := r.deps.Events.ListEventIDsByCompanySFID(ctx, g.OldID)
	if err != nil {
		return fmt.Errorf("listing events of %s: %w", g.OldID, err)
	}
	for _, claGroupID := range g.CLAGroupIDs() {
		more, listErr := r.deps.Events.ListEventIDsByCompanySFIDCLAGroup(ctx, g.OldID, claGroupID)
		if listErr != nil {
			return fmt.Errorf("listing events of %s/%s: %w", g.OldID, claGroupID, listErr)
		}
		ids = append(ids, more...)
	}
	ids = distinct(ids)
	rekeyed := 0
	for _, id := range ids {
		done, rekeyErr := r.deps.Events.RekeyEventCompanySFID(ctx, id, g.OldID, g.NewID)
		if rekeyErr != nil {
			return fmt.Errorf("event %s: %w", id, rekeyErr)
		}
		if done {
			rekeyed++
		}
	}
	fmt.Fprintf(r.deps.Out, "events %s -> %s: %d listed, %d re-keyed\n", g.OldID, g.NewID, len(ids), rekeyed)
	return nil
}

// deleteOldGrants removes the old org's grants (step 8); a missing grant is success.
func (r *runner) deleteOldGrants(ctx context.Context, g *Group) error {
	oldGrants, err := r.deps.ACS.ListOrgGrants(ctx, g.OldID)
	if err != nil {
		return fmt.Errorf("listing grants of %s: %w", g.OldID, err)
	}
	sort.Slice(oldGrants, func(i, j int) bool { return oldGrants[i].GrantID < oldGrants[j].GrantID })
	for _, gr := range oldGrants {
		if gr.GrantID == "" || gr.RoleID == "" {
			continue
		}
		if err = r.deps.Orgs.DeleteUserRoleScope(ctx, g.OldID, gr.RoleID, gr.GrantID, gr.Username); err != nil {
			return fmt.Errorf("deleting %s grant %s of %s: %w", gr.RoleName, gr.GrantID, gr.Username, err)
		}
	}
	fmt.Fprintf(r.deps.Out, "grants %s: %d deleted\n", g.OldID, len(oldGrants))
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
