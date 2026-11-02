// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/cla_admins"
)

// CLAAdminsService computes and publishes one organization's CLA-manager roster (cla_admins.Service satisfies it).
type CLAAdminsService interface {
	Roster(ctx context.Context, sfid string, signatureIDs ...string) (*cla_admins.Roster, error)
	Publish(ctx context.Context, sfid string, signatureIDs ...string) (*cla_admins.Result, error)
}

// CLAAdminsOptions are the cla-admins flags.
type CLAAdminsOptions struct {
	Apply  bool
	IDs    []string
	OutDir string
}

const claAdminsResultError = "error"

// CLAAdminsRow is one organization of the cla_admins.csv report.
type CLAAdminsRow struct {
	SFID       string
	CompanyIDs []string
	Managers   []string
	Result     string
	Err        string
}

// CLAAdminsSummary counts the backfill outcome.
type CLAAdminsSummary struct {
	Orgs      int
	Published int
	DryRun    int
	Skipped   int
	Errors    int
	NoAccount int
}

func (s CLAAdminsSummary) String() string {
	return fmt.Sprintf("cla-admins orgs=%d published=%d dry_run=%d skipped_no_b2b_org=%d errors=%d companies_without_account=%d",
		s.Orgs, s.Published, s.DryRun, s.Skipped, s.Errors, s.NoAccount)
}

// BackfillCLAAdmins publishes (or, without Apply, only computes) the CLA-manager roster of every
// organization with a signed CCLA, and of every Account id named in IDs even when it has none left,
// so a failed last removal can still be replaced by an empty list; companies whose id is not a
// Salesforce Account are counted and skipped. The rows are written to cla_admins.csv in OutDir.
func BackfillCLAAdmins(ctx context.Context, deps Deps, svc CLAAdminsService, opts CLAAdminsOptions) (*CLAAdminsSummary, []CLAAdminsRow, error) {
	out := deps.Out
	if out == nil {
		out = io.Discard
	}
	want := map[string]bool{}
	for _, id := range opts.IDs {
		id18, ok := cla_admins.AccountID(id)
		if !ok {
			return nil, nil, fmt.Errorf("--ids: %q is not a Salesforce Account id", id)
		}
		want[id18] = true
	}
	cclas, err := deps.Signatures.GetCCLASignatures(ctx, aws.Bool(true), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("listing signed CCLAs: %w", err)
	}
	signedCompanies := map[string]bool{}
	signedIDs := map[string][]string{}
	for _, c := range cclas {
		if c != nil && c.SignatureSigned && c.SignatureReferenceID != "" && c.SignatureType == utils.SignatureTypeCCLA &&
			c.SignatureReferenceType == utils.SignatureReferenceTypeCompany {
			signedCompanies[c.SignatureReferenceID] = true
			signedIDs[c.SignatureReferenceID] = append(signedIDs[c.SignatureReferenceID], c.SignatureID)
		}
	}
	companies, err := deps.Companies.GetCompanies(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("scanning companies: %w", err)
	}
	summary := &CLAAdminsSummary{}
	sfids := map[string]bool{}
	// the scanned CCLA ids of every entity of the Account go to the publisher, so a row the indexes do not
	// list yet is still read from the base table
	scanned := map[string]map[string]bool{}
	for _, c := range companies.Companies {
		if !signedCompanies[c.CompanyID] {
			continue
		}
		id18, ok := cla_admins.AccountID(c.CompanyExternalID)
		if !ok {
			summary.NoAccount++
			continue
		}
		sfids[id18] = true
		if scanned[id18] == nil {
			scanned[id18] = map[string]bool{}
		}
		for _, id := range signedIDs[c.CompanyID] {
			scanned[id18][id] = true
		}
	}
	for id18 := range want {
		sfids[id18] = true
	}
	var rows []CLAAdminsRow
	for _, sfid := range sortedBoolKeys(sfids) {
		if len(want) > 0 && !want[sfid] {
			continue
		}
		summary.Orgs++
		row := CLAAdminsRow{SFID: sfid}
		if opts.Apply {
			res, pubErr := svc.Publish(ctx, sfid, sortedBoolKeys(scanned[sfid])...)
			if res != nil && res.Roster != nil {
				row.CompanyIDs, row.Managers = res.Roster.CompanyIDs, res.Roster.Usernames()
			}
			switch {
			case pubErr != nil:
				row.Result, row.Err = claAdminsResultError, pubErr.Error()
				summary.Errors++
			case res.Skipped:
				row.Result = "skipped_no_b2b_org"
				summary.Skipped++
			default:
				row.Result = "published"
				summary.Published++
			}
		} else {
			roster, rosterErr := svc.Roster(ctx, sfid, sortedBoolKeys(scanned[sfid])...)
			if rosterErr != nil {
				row.Result, row.Err = claAdminsResultError, rosterErr.Error()
				summary.Errors++
			} else {
				row.CompanyIDs, row.Managers, row.Result = roster.CompanyIDs, roster.Usernames(), "dry_run"
				summary.DryRun++
			}
		}
		fmt.Fprintf(out, "%s %s companies=%d managers=%d %s\n", row.Result, sfid, len(row.CompanyIDs), len(row.Managers), row.Err)
		rows = append(rows, row)
	}
	if opts.OutDir != "" {
		if err := writeCLAAdminsCSV(filepath.Join(opts.OutDir, "cla_admins.csv"), rows); err != nil {
			return summary, rows, err
		}
	}
	return summary, rows, nil
}

func writeCLAAdminsCSV(path string, rows []CLAAdminsRow) error {
	file, err := os.Create(filepath.Clean(path))
	if err != nil {
		return err
	}
	w := csv.NewWriter(file)
	records := [][]string{{"sfid", "company_ids", "managers", "result", "error"}}
	for _, r := range rows {
		records = append(records, []string{r.SFID, strings.Join(r.CompanyIDs, ";"), strings.Join(r.Managers, ";"), r.Result, r.Err})
	}
	if err := w.WriteAll(records); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func sortedBoolKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
