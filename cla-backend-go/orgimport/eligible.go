// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

const sfidPrefix = "001"

// ShapeOf classifies an external id: 15/18-char alphanumeric starting with 001 → SFID, lf… → lf.
func ShapeOf(id string) IDShape {
	id = strings.TrimSpace(id)
	switch {
	case id == "":
		return ShapeEmpty
	case IsSFID(id):
		return ShapeSFID
	case strings.HasPrefix(id, "lf"):
		return ShapeLF
	default:
		return ShapeOther
	}
}

// IsSFID reports whether id looks like a Salesforce Account ID.
func IsSFID(id string) bool {
	if len(id) != 15 && len(id) != 18 || !strings.HasPrefix(id, sfidPrefix) {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

func canonicalEntity(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// accountKey identifies one Salesforce Account regardless of the 15- or 18-char form of its id
// (the last three characters of an 18-char id are a checksum of the first fifteen).
func accountKey(id string) string {
	if IsSFID(id) {
		return id[:15]
	}
	return id
}

// Inventory is the companies table joined with the active-CCLA set.
type Inventory struct {
	Rows       []*Row
	ByID       map[string]*Row
	ByExternal map[string][]*Row
	ByAccount  map[string][]*Row // Salesforce-shaped external ids keyed by accountKey
}

// LoadInventory scans companies and active (signed+approved, company-referenced) CCLAs.
func LoadInventory(ctx context.Context, deps Deps) (*Inventory, error) {
	cclas, err := deps.Signatures.GetCCLASignatures(ctx, aws.Bool(true), aws.Bool(true))
	if err != nil {
		return nil, fmt.Errorf("listing active CCLAs: %w", err)
	}
	active := map[string][]string{}
	signedOn := map[string]string{}
	for _, s := range cclas {
		if s == nil || s.SignatureReferenceID == "" || s.SignatureReferenceType != utils.SignatureReferenceTypeCompany ||
			s.SignatureType != utils.SignatureTypeCCLA || !s.SignatureSigned || !s.SignatureApproved {
			continue
		}
		active[s.SignatureReferenceID] = append(active[s.SignatureReferenceID], s.SignatureProjectID)
		if d := firstNonEmpty(s.SignedOn, s.DateCreated); d != "" && (signedOn[s.SignatureReferenceID] == "" || d < signedOn[s.SignatureReferenceID]) {
			signedOn[s.SignatureReferenceID] = d
		}
	}
	companies, err := deps.Companies.GetCompanies(ctx)
	if err != nil {
		return nil, fmt.Errorf("scanning companies: %w", err)
	}
	inv := &Inventory{ByID: map[string]*Row{}, ByExternal: map[string][]*Row{}, ByAccount: map[string][]*Row{}}
	for i := range companies.Companies {
		c := companies.Companies[i]
		row := &Row{
			CompanyID:         c.CompanyID,
			CompanyName:       c.CompanyName,
			SigningEntityName: c.SigningEntityName,
			ExternalID:        strings.TrimSpace(c.CompanyExternalID),
			RawExternalID:     c.CompanyExternalID,
		}
		if groups, ok := active[c.CompanyID]; ok {
			row.ActiveCCLA = true
			row.CCLACount = len(groups)
			row.CLAGroupIDs = distinct(groups)
			row.CCLASignedOn = signedOn[c.CompanyID]
		}
		inv.Rows = append(inv.Rows, row)
		inv.ByID[row.CompanyID] = row
		if row.ExternalID != "" {
			inv.ByExternal[row.ExternalID] = append(inv.ByExternal[row.ExternalID], row)
			if IsSFID(row.ExternalID) {
				inv.ByAccount[accountKey(row.ExternalID)] = append(inv.ByAccount[accountKey(row.ExternalID)], row)
			}
		}
	}
	sort.Slice(inv.Rows, func(i, j int) bool { return inv.Rows[i].CompanyID < inv.Rows[j].CompanyID })
	return inv, nil
}

// EligibleGroups returns one group per external id that has at least one active-CCLA row (all
// sibling rows included); rows with an empty or malformed external id form single-row groups keyed
// by company id (a blank or garbage value never identifies one organization).
func (inv *Inventory) EligibleGroups() []*Group {
	var groups []*Group
	seen := map[string]bool{}
	for _, row := range inv.Rows {
		if !row.ActiveCCLA {
			continue
		}
		if shape := ShapeOf(row.ExternalID); shape == ShapeEmpty || shape == ShapeOther {
			groups = append(groups, &Group{OldID: row.RawExternalID, Key: row.CompanyID, Shape: shape, Rows: []*Row{row}})
			continue
		}
		if seen[row.ExternalID] {
			continue
		}
		seen[row.ExternalID] = true
		rows := append([]*Row(nil), inv.ByExternal[row.ExternalID]...)
		sort.Slice(rows, func(i, j int) bool { return rows[i].CompanyID < rows[j].CompanyID })
		g := &Group{OldID: row.ExternalID, Key: row.ExternalID, Shape: ShapeOf(row.ExternalID), Rows: rows}
		g.Duplicate = hasDuplicateEntity(rows)
		g.Aliases = inv.aliasForms(row.ExternalID)
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Key < groups[j].Key })
	return groups
}

// aliasForms lists the other exact forms (15- vs 18-char) of the same Account carried by inventory rows.
func (inv *Inventory) aliasForms(externalID string) []string {
	if !IsSFID(externalID) {
		return nil
	}
	var forms []string
	for _, r := range inv.ByAccount[accountKey(externalID)] {
		if r.ExternalID != externalID && !containsString(forms, r.ExternalID) {
			forms = append(forms, r.ExternalID)
		}
	}
	sort.Strings(forms)
	return forms
}

// hasDuplicateEntity is true when two rows of one external id share the same (or empty) entity name;
// a signing entity equal to the company name is the parent row (the console writes it that way).
func hasDuplicateEntity(rows []*Row) bool {
	seen := map[string]bool{}
	for _, r := range rows {
		k := canonicalEntity(r.SigningEntityName)
		if k == canonicalEntity(r.CompanyName) {
			k = ""
		}
		if seen[k] {
			return true
		}
		seen[k] = true
	}
	return false
}

// rowsCarrying returns the inventory rows whose external id is newID (in either 15- or 18-char form).
func (inv *Inventory) rowsCarrying(newID string) []*Row {
	if IsSFID(newID) {
		return inv.ByAccount[accountKey(newID)]
	}
	return inv.ByExternal[newID]
}

// ExternalIDCollision reports whether a row outside the group already carries newID.
func (inv *Inventory) ExternalIDCollision(g *Group, newID string) bool {
	for _, r := range inv.rowsCarrying(newID) {
		if !containsRow(g.Rows, r.CompanyID) {
			return true
		}
	}
	return false
}

func containsRow(rows []*Row, companyID string) bool {
	for _, r := range rows {
		if r.CompanyID == companyID {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func distinct(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
