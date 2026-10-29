// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/cla_admins"
	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

type claAdminsCompanies struct{ companies []models.Company }

func (c claAdminsCompanies) GetCompanies(context.Context) (*models.Companies, error) {
	return &models.Companies{Companies: c.companies}, nil
}
func (c claAdminsCompanies) GetCompanyRecord(context.Context, string) (*company.DBModel, error) {
	return nil, errors.New("unused")
}
func (c claAdminsCompanies) UpdateCompanyExternalID(context.Context, string, string, string) error {
	return errors.New("unused")
}

type claAdminsSignatures struct{ rows []*signatures.ItemSignature }

func (s claAdminsSignatures) GetCCLASignatures(context.Context, *bool, *bool) ([]*signatures.ItemSignature, error) {
	return s.rows, nil
}

type claAdminsFakeService struct {
	rosters    []string
	published  []string
	rosterIDs  map[string][]string
	publishIDs map[string][]string
	skip       map[string]bool
	fail       map[string]error
	empty      map[string]bool
}

func (f *claAdminsFakeService) record(store *map[string][]string, sfid string, ids []string) {
	if *store == nil {
		*store = map[string][]string{}
	}
	(*store)[sfid] = append([]string{}, ids...)
}

func (f *claAdminsFakeService) roster(sfid string) *cla_admins.Roster {
	if f.empty[sfid] {
		return &cla_admins.Roster{SFID: sfid, Admins: []member_service.CLAAdmin{}}
	}
	return &cla_admins.Roster{SFID: sfid, CompanyIDs: []string{"c-" + sfid[:4]}, Admins: []member_service.CLAAdmin{{Username: "amy"}, {Username: "bob"}}}
}

func (f *claAdminsFakeService) Roster(_ context.Context, sfid string, ids ...string) (*cla_admins.Roster, error) {
	f.rosters = append(f.rosters, sfid)
	f.record(&f.rosterIDs, sfid, ids)
	if err := f.fail[sfid]; err != nil {
		return nil, err
	}
	return f.roster(sfid), nil
}

func (f *claAdminsFakeService) Publish(_ context.Context, sfid string, ids ...string) (*cla_admins.Result, error) {
	f.published = append(f.published, sfid)
	f.record(&f.publishIDs, sfid, ids)
	if err := f.fail[sfid]; err != nil {
		return nil, err
	}
	return &cla_admins.Result{Roster: f.roster(sfid), Skipped: f.skip[sfid]}, nil
}

func claAdminsDeps(out io.Writer) Deps {
	ccla := func(id, companyID string, signed bool) *signatures.ItemSignature {
		return &signatures.ItemSignature{SignatureID: id, SignatureReferenceID: companyID, SignatureReferenceType: "company", SignatureType: "ccla", SignatureSigned: signed}
	}
	return Deps{
		Out: out,
		Companies: claAdminsCompanies{companies: []models.Company{
			{CompanyID: "c-live-18", CompanyExternalID: "0014100000Te0G7AAJ"},
			{CompanyID: "c-live-15", CompanyExternalID: "0014100000Te0G7"},
			{CompanyID: "c-other", CompanyExternalID: "0012M00002KB7YxQAL"},
			{CompanyID: "c-legacy", CompanyExternalID: "lfSclQhURLCCrv7EZj"},
			{CompanyID: "c-unsigned", CompanyExternalID: "0012M00002PZzFFQA1"},
			{CompanyID: "c-no-ccla", CompanyExternalID: "0012M00002PZzFiQAL"},
		}},
		Signatures: claAdminsSignatures{rows: []*signatures.ItemSignature{
			ccla("s1", "c-live-18", true), ccla("s2", "c-live-15", true), ccla("s3", "c-other", true),
			ccla("s4", "c-legacy", true), ccla("s5", "c-unsigned", false),
		}},
	}
}

func TestBackfillCLAAdminsDryRun(t *testing.T) {
	var out bytes.Buffer
	svc := &claAdminsFakeService{}
	dir := t.TempDir()
	summary, rows, err := BackfillCLAAdmins(context.Background(), claAdminsDeps(&out), svc, CLAAdminsOptions{OutDir: dir})
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{"0012M00002KB7YxQAL": {"s3"}, "0014100000Te0G7AAJ": {"s1", "s2"}}, svc.rosterIDs, "the scanned CCLA ids of every entity of the Account reach the roster, never another Account's")
	assert.Equal(t, []string{"0012M00002KB7YxQAL", "0014100000Te0G7AAJ"}, svc.rosters, "the 15- and 18-character rows collapse onto one organization; legacy and unsigned companies are left out")
	assert.Empty(t, svc.published)
	assert.Equal(t, CLAAdminsSummary{Orgs: 2, DryRun: 2, NoAccount: 1}, *summary)
	require.Len(t, rows, 2)
	assert.Equal(t, "dry_run", rows[0].Result)
	assert.Equal(t, []string{"amy", "bob"}, rows[0].Managers)
	csvBytes, readErr := os.ReadFile(filepath.Clean(filepath.Join(dir, "cla_admins.csv")))
	require.NoError(t, readErr)
	assert.Contains(t, string(csvBytes), "0014100000Te0G7AAJ,c-0014,amy;bob,dry_run,")
	assert.Contains(t, out.String(), "dry_run 0014100000Te0G7AAJ companies=1 managers=2")
}

func TestBackfillCLAAdminsApply(t *testing.T) {
	svc := &claAdminsFakeService{skip: map[string]bool{"0012M00002KB7YxQAL": true}}
	summary, rows, err := BackfillCLAAdmins(context.Background(), claAdminsDeps(nil), svc, CLAAdminsOptions{Apply: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"0012M00002KB7YxQAL", "0014100000Te0G7AAJ"}, svc.published)
	assert.Equal(t, map[string][]string{"0012M00002KB7YxQAL": {"s3"}, "0014100000Te0G7AAJ": {"s1", "s2"}}, svc.publishIDs, "the scanned ids are registered and read by Publish")
	assert.Empty(t, svc.rosters)
	assert.Equal(t, CLAAdminsSummary{Orgs: 2, Published: 1, Skipped: 1, NoAccount: 1}, *summary)
	assert.Equal(t, "skipped_no_b2b_org", rows[0].Result)
	assert.Equal(t, "published", rows[1].Result)
}

func TestBackfillCLAAdminsIDsAndErrors(t *testing.T) {
	svc := &claAdminsFakeService{fail: map[string]error{"0014100000Te0G7AAJ": errors.New("boom")}}
	summary, rows, err := BackfillCLAAdmins(context.Background(), claAdminsDeps(nil), svc, CLAAdminsOptions{Apply: true, IDs: []string{"0014100000Te0G7"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"0014100000Te0G7AAJ"}, svc.published, "--ids is normalised to 18 characters and limits the run")
	assert.Equal(t, 1, summary.Errors)
	assert.Equal(t, "error", rows[0].Result)
	assert.Equal(t, "boom", rows[0].Err)

	_, _, err = BackfillCLAAdmins(context.Background(), claAdminsDeps(nil), svc, CLAAdminsOptions{IDs: []string{"lfSclQhURLCCrv7EZj"}})
	assert.ErrorContains(t, err, "not a Salesforce Account id")
}

// an organization named with --ids is processed even when no signed CCLA of it remains, so a failed last
// removal can still be replaced by an empty list
func TestBackfillCLAAdminsExplicitIDWithoutSignedCCLA(t *testing.T) {
	gone, ok := cla_admins.AccountID("0014100000Te0G9")
	require.True(t, ok)

	svc := &claAdminsFakeService{empty: map[string]bool{gone: true}}
	summary, rows, err := BackfillCLAAdmins(context.Background(), claAdminsDeps(nil), svc, CLAAdminsOptions{IDs: []string{"0014100000Te0G9"}})
	require.NoError(t, err)
	assert.Equal(t, []string{gone}, svc.rosters)
	assert.Empty(t, svc.rosterIDs[gone], "no scanned CCLA for an explicitly named Account")
	assert.Equal(t, CLAAdminsSummary{Orgs: 1, DryRun: 1, NoAccount: 1}, *summary)
	require.Len(t, rows, 1)
	assert.Equal(t, "dry_run", rows[0].Result)
	assert.Empty(t, rows[0].CompanyIDs)
	assert.Empty(t, rows[0].Managers)

	svc = &claAdminsFakeService{empty: map[string]bool{gone: true}}
	summary, rows, err = BackfillCLAAdmins(context.Background(), claAdminsDeps(nil), svc, CLAAdminsOptions{Apply: true, IDs: []string{"0014100000Te0G9"}})
	require.NoError(t, err)
	assert.Equal(t, []string{gone}, svc.published, "the empty roster is published")
	assert.Equal(t, CLAAdminsSummary{Orgs: 1, Published: 1, NoAccount: 1}, *summary)
	assert.Equal(t, "published", rows[0].Result)
	assert.Empty(t, rows[0].Managers)
}
