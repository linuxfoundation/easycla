// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	acs_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

const (
	liveSFID    = "0014100000Te0G7AAJ"
	liveSFID2   = "0014100000AbCdEfGH"
	deadSFID    = "0014100000DeadDead"
	targetSFID  = "0014100000NewNewNe"
	targetSFID2 = "0014100000NewTwoXY"
	lfID        = "lf-legacy-0001"
)

type fakeCompanies struct {
	rows    map[string]*company.DBModel
	updates []string
	scans   int
}

func (f *fakeCompanies) GetCompanies(context.Context) (*models.Companies, error) {
	f.scans++
	out := &models.Companies{}
	ids := make([]string, 0, len(f.rows))
	for id := range f.rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := f.rows[id]
		out.Companies = append(out.Companies, models.Company{CompanyID: r.CompanyID, CompanyName: r.CompanyName, SigningEntityName: r.SigningEntityName, CompanyExternalID: r.CompanyExternalID})
	}
	return out, nil
}

func (f *fakeCompanies) GetCompanyRecord(_ context.Context, id string) (*company.DBModel, error) {
	r, ok := f.rows[id]
	if !ok {
		return nil, fmt.Errorf("company %s not found", id)
	}
	cp := *r
	return &cp, nil
}

func (f *fakeCompanies) UpdateCompanyExternalID(_ context.Context, id, oldID, newID string) error {
	r, ok := f.rows[id]
	if !ok || r.CompanyExternalID != oldID {
		return company.ErrExternalIDConditionFailed
	}
	r.PreviousCompanyExternalID, r.CompanyExternalID = oldID, newID
	f.updates = append(f.updates, id+":"+oldID+"->"+newID)
	return nil
}

type fakeSignatures struct{ items []*signatures.ItemSignature }

func (f *fakeSignatures) GetCCLASignatures(context.Context, *bool, *bool) ([]*signatures.ItemSignature, error) {
	return f.items, nil
}

type fakeEvents struct {
	sfidByEvent map[string]string
	byGroup     map[string][]string
	rekeyed     []string
	lagging     map[string]bool // ids the org-wide index does not serve yet (GSI lag)
	listErr     map[string]error
	rekeyErr    map[string]error
	listed      []string // org-wide listings, by sfid
}

func (f *fakeEvents) ListEventIDsByCompanySFID(_ context.Context, sfid string) ([]string, error) {
	f.listed = append(f.listed, sfid)
	if err := f.listErr[sfid]; err != nil {
		return nil, err
	}
	var out []string
	for id, s := range f.sfidByEvent {
		if s == sfid && !f.lagging[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (f *fakeEvents) ListEventIDsByCompanySFIDCLAGroup(_ context.Context, sfid, claGroupID string) ([]string, error) {
	var out []string
	for _, id := range f.byGroup[sfid+"#"+claGroupID] {
		if f.sfidByEvent[id] == sfid {
			out = append(out, id)
		}
	}
	return out, nil
}

func (f *fakeEvents) RekeyEventCompanySFID(_ context.Context, id, oldSFID, newSFID string) (bool, error) {
	if err := f.rekeyErr[id]; err != nil {
		return false, err
	}
	if f.sfidByEvent[id] != oldSFID {
		return false, nil
	}
	f.sfidByEvent[id] = newSFID
	f.rekeyed = append(f.rekeyed, id)
	return true, nil
}

// fakePlatform is org-service + ACS + member-service + Salesforce in one.
type fakePlatform struct {
	orgs        map[string]*Org
	servedAfter map[string]int
	grants      map[string][]acs_service.OrgGrant
	sfAccounts  map[string]bool
	registered  []string
	calls       []string
	nextGrant   int
	failCreate  bool
	failGetB2B  bool
	noMembers   bool
	listHook    func(orgID string)
}

func newPlatform() *fakePlatform {
	return &fakePlatform{orgs: map[string]*Org{}, servedAfter: map[string]int{}, grants: map[string][]acs_service.OrgGrant{}, sfAccounts: map[string]bool{}}
}

func (p *fakePlatform) GetOrganization(_ context.Context, id string) (*Org, error) {
	p.calls = append(p.calls, "get-org:"+id)
	if n := p.servedAfter[id]; n > 0 {
		p.servedAfter[id] = n - 1
		return nil, ErrOrgNotFound
	}
	if o, ok := p.orgs[id]; ok {
		return o, nil
	}
	return nil, ErrOrgNotFound
}

func (p *fakePlatform) CreateUserRoleScope(_ context.Context, username, orgID, objectType, objectID, roleID string) error {
	p.calls = append(p.calls, "create-grant:"+orgID+":"+username+":"+roleID+":"+objectID)
	if p.failCreate {
		return errors.New("org-service down")
	}
	p.nextGrant++
	p.grants[orgID] = append(p.grants[orgID], acs_service.OrgGrant{Username: username, RoleID: roleID, RoleName: "role-" + roleID, GrantID: fmt.Sprintf("g%d", p.nextGrant), ScopeID: fmt.Sprintf("s%d", p.nextGrant), ObjectTypeName: objectType, ObjectID: objectID})
	return nil
}

func (p *fakePlatform) DeleteUserRoleScope(_ context.Context, orgID, roleID, grantID, username string) error {
	p.calls = append(p.calls, "delete-grant:"+orgID+":"+username+":"+roleID+":"+grantID)
	kept := p.grants[orgID][:0]
	for _, g := range p.grants[orgID] {
		if g.GrantID != grantID {
			kept = append(kept, g)
		}
	}
	p.grants[orgID] = kept
	return nil
}

func (p *fakePlatform) ListOrgGrants(_ context.Context, orgID string) ([]acs_service.OrgGrant, error) {
	if p.listHook != nil {
		p.listHook(orgID)
	}
	return append([]acs_service.OrgGrant(nil), p.grants[orgID]...), nil
}

func (p *fakePlatform) GetB2BOrg(_ context.Context, uid string) (*member_service.B2BOrg, error) {
	p.calls = append(p.calls, "get-b2b:"+uid)
	if p.failGetB2B {
		return nil, &member_service.AuthError{Status: 403, Message: "forbidden"}
	}
	if !p.sfAccounts[uid] {
		return nil, member_service.ErrOrgNotFound
	}
	return &member_service.B2BOrg{UID: uid, Name: "acct " + uid}, nil
}

func (p *fakePlatform) RegisterB2BOrg(_ context.Context, sfid string) (*member_service.B2BOrg, error) {
	p.calls = append(p.calls, "register:"+sfid)
	if !p.sfAccounts[sfid] {
		return nil, member_service.ErrOrgNotFound
	}
	p.registered = append(p.registered, sfid)
	return &member_service.B2BOrg{UID: sfid, Name: "acct " + sfid}, nil
}

func (p *fakePlatform) addGrant(orgID, username, roleID, project string) {
	p.nextGrant++
	g := acs_service.OrgGrant{Username: username, RoleID: roleID, RoleName: "role-" + roleID, GrantID: fmt.Sprintf("g%d", p.nextGrant), ScopeID: fmt.Sprintf("s%d", p.nextGrant), ObjectTypeName: objectTypeOrganization, ObjectID: orgID}
	if project != "" {
		g.ObjectTypeName, g.ObjectID = objectTypeProjectOrganization, project+"|"+orgID
	}
	p.grants[orgID] = append(p.grants[orgID], g)
}

type fixture struct {
	companies *fakeCompanies
	sigs      *fakeSignatures
	events    *fakeEvents
	platform  *fakePlatform
	out       *bytes.Buffer
	sleeps    []time.Duration
}

func newFixture() *fixture {
	fx := &fixture{
		companies: &fakeCompanies{rows: map[string]*company.DBModel{}},
		sigs:      &fakeSignatures{},
		events:    &fakeEvents{sfidByEvent: map[string]string{}, byGroup: map[string][]string{}},
		platform:  newPlatform(),
		out:       &bytes.Buffer{},
	}
	return fx
}

func (fx *fixture) company(id, name, entity, external string, activeGroups ...string) {
	fx.companies.rows[id] = &company.DBModel{CompanyID: id, CompanyName: name, SigningEntityName: entity, CompanyExternalID: external}
	for _, g := range activeGroups {
		fx.sigs.items = append(fx.sigs.items, &signatures.ItemSignature{SignatureReferenceID: id, SignatureReferenceType: utils.SignatureReferenceTypeCompany, SignatureProjectID: g, SignatureType: "ccla", SignatureSigned: true, SignatureApproved: true, SignedOn: "2024-01-02T00:00:00Z"})
	}
}

func (fx *fixture) deps() Deps {
	d := Deps{Companies: fx.companies, Signatures: fx.sigs, Events: fx.events, Orgs: fx.platform, ACS: fx.platform, Out: fx.out,
		Now:   func() time.Time { return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) },
		Sleep: func(_ context.Context, d time.Duration) error { fx.sleeps = append(fx.sleeps, d); return nil }}
	if !fx.platform.noMembers {
		d.Members = fx.platform
	}
	return d
}

func writeMapping(t *testing.T, dir string, rows ...string) string {
	t.Helper()
	path := filepath.Join(dir, "mapping.csv")
	require.NoError(t, os.WriteFile(path, []byte("old_id,new_id,action,approved\n"+strings.Join(rows, "\n")+"\n"), 0o600))
	return path
}

const somebodySFID = "0014100000Somebody"

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(filepath.Clean(path))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	rows, err := csv.NewReader(f).ReadAll()
	require.NoError(t, err)
	return rows
}

func groupByKey(groups []*Group, key string) *Group {
	for _, g := range groups {
		if g.Key == key {
			return g
		}
	}
	return nil
}

func TestShapeOf(t *testing.T) {
	assert.Equal(t, ShapeSFID, ShapeOf(liveSFID))
	assert.Equal(t, ShapeSFID, ShapeOf("001410000Te0G7A"))
	assert.Equal(t, ShapeLF, ShapeOf(lfID))
	assert.Equal(t, ShapeEmpty, ShapeOf("  "))
	assert.Equal(t, ShapeOther, ShapeOf("0064100000Te0G7AAJ"))
	assert.Equal(t, ShapeOther, ShapeOf("0014100000Te0G7AA"))
	assert.Equal(t, ShapeOther, ShapeOf("0014100000Te0G7AA!"))
}

func TestEligibleGroups(t *testing.T) {
	fx := newFixture()
	fx.company("c-parent", "Acme", "", liveSFID, "cg-1")
	fx.company("c-sub", "Acme", "Acme GmbH", liveSFID)
	fx.company("c-dup", "Acme", "", liveSFID)
	fx.company("c-inactive", "Idle Corp", "", liveSFID2)
	fx.company("c-empty", "No SFID Inc", "", "", "cg-2")
	fx.company("c-lf", "Legacy Ltd", "", lfID, "cg-1", "cg-3")
	inv, err := LoadInventory(context.Background(), fx.deps())
	require.NoError(t, err)
	groups := inv.EligibleGroups()
	require.Len(t, groups, 3)

	acme := groupByKey(groups, liveSFID)
	require.NotNil(t, acme)
	assert.Equal(t, []string{"c-dup", "c-parent", "c-sub"}, acme.CompanyIDs(), "siblings without an active CCLA move with the group")
	assert.True(t, acme.Duplicate, "two rows with the same empty entity name are a possible duplicate")
	assert.Equal(t, []string{"cg-1"}, acme.CLAGroupIDs())

	empty := groupByKey(groups, "c-empty")
	require.NotNil(t, empty)
	assert.Equal(t, ShapeEmpty, empty.Shape)

	legacy := groupByKey(groups, lfID)
	require.NotNil(t, legacy)
	assert.False(t, legacy.Duplicate)
	assert.Equal(t, 2, legacy.Rows[0].CCLACount)
	assert.Equal(t, []string{"cg-1", "cg-3"}, legacy.CLAGroupIDs())
	assert.Nil(t, groupByKey(groups, liveSFID2), "rows without an active CCLA are not eligible")
}

func TestParseMapping(t *testing.T) {
	m, err := ParseMapping(strings.NewReader("old_id,new_id,action,approved\n" + lfID + "," + targetSFID + ",matched,true\n" + deadSFID + "," + targetSFID2 + ",created,\nlf-x,,ambiguous,false\n"))
	require.NoError(t, err)
	id, action, reason := m.Resolve(lfID)
	assert.Equal(t, targetSFID, id)
	assert.Equal(t, ActionMatched, action)
	assert.Empty(t, reason)
	id, _, reason = m.Resolve(deadSFID)
	assert.Equal(t, targetSFID2, id)
	assert.Empty(t, reason)
	_, _, reason = m.Resolve("lf-x")
	assert.Equal(t, "mapping_ambiguous", reason)
	_, _, reason = m.Resolve("unknown")
	assert.Equal(t, "no_mapping", reason)
	_, _, reason = (*Mapping)(nil).Resolve(lfID)
	assert.Equal(t, "no_mapping", reason)

	m, err = ParseMapping(strings.NewReader("old_id,new_id,action,approved\n" + lfID + "," + targetSFID + ",matched,false\n"))
	require.NoError(t, err)
	_, _, reason = m.Resolve(lfID)
	assert.Equal(t, "mapping_not_approved", reason)

	m, err = ParseMapping(strings.NewReader("old_id,new_id,action,approved\nlf-a," + targetSFID + ",created,true\nlf-b," + targetSFID + ",created,true\n"))
	require.NoError(t, err)
	id, _, reason = m.Resolve("lf-a")
	assert.Equal(t, targetSFID, id, "collisions are decided by the approval-aware pass, not by the mapping")
	assert.Empty(t, reason)
	assert.Equal(t, 2, m.Targets[targetSFID])

	for _, bad := range []string{
		"",
		"old_id,new_id,action\nlf-a,x,created",
		"old_id,new_id,action,approved\nlf-a,not-an-sfid,created,true",
		"old_id,new_id,action,approved\nlf-a," + targetSFID + ",merge,true",
		"old_id,new_id,action,approved\nlf-a," + targetSFID + ",created,maybe",
		"old_id,new_id,action,approved\nlf-a," + targetSFID + ",created,true\nlf-a," + targetSFID2 + ",created,true",
		"old_id,new_id,action,approved\n," + targetSFID + ",created,true",
	} {
		_, err = ParseMapping(strings.NewReader(bad))
		assert.Error(t, err, bad)
	}
}

func TestState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")
	st, err := LoadState(path)
	require.NoError(t, err)
	assert.Empty(t, st.Unfinished())
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	require.NoError(t, st.Append(StateRecord{OldID: lfID, NewID: targetSFID, CompanyIDs: []string{"c-1"}, Step: StepRows, Status: "ok"}, now))
	require.NoError(t, st.Append(StateRecord{OldID: deadSFID, NewID: targetSFID2, CompanyIDs: []string{"c-2"}, Step: StepDone, Status: "ok"}, now))
	require.NoError(t, st.Append(StateRecord{OldID: lfID, NewID: targetSFID, CompanyIDs: []string{"c-1"}, Step: StepCleanup, Status: "failed", Err: "boom"}, now))
	require.NoError(t, st.Append(StateRecord{OldID: "", Key: "c-blank-a", NewID: targetSFID2, CompanyIDs: []string{"c-blank-a"}, Step: StepRows, Status: "ok"}, now))
	require.NoError(t, st.Append(StateRecord{OldID: "", Key: "c-blank-b", NewID: targetSFID2, CompanyIDs: []string{"c-blank-b"}, Step: StepDone, Status: "ok"}, now))

	st, err = LoadState(path)
	require.NoError(t, err)
	unfinished := st.Unfinished()
	require.Len(t, unfinished, 2, "row-targeted records are keyed by company id, not by their blank old id")
	assert.Equal(t, "c-blank-a", unfinished[0].Key)
	assert.Equal(t, lfID, unfinished[1].OldID)
	assert.Equal(t, StepCleanup, unfinished[1].Step)
	assert.Equal(t, "2026-09-29T12:00:00Z", unfinished[1].TS)
	assert.True(t, st.Done(deadSFID))
	assert.False(t, st.Done(lfID))
	assert.True(t, st.Done("c-blank-b"))
	assert.False(t, st.Done("c-blank-a"))
	assert.False(t, st.Done(""))

	none, err := LoadState("")
	require.NoError(t, err)
	require.NoError(t, none.Append(StateRecord{OldID: "x"}, now), "no state file: append is a no-op")

	require.NoError(t, os.WriteFile(path, []byte("{not json}\n"), 0o600))
	_, err = LoadState(path)
	assert.Error(t, err)
	require.NoError(t, os.WriteFile(path, []byte(`{"old_id":"","new_id":"0014100000Te0yqQAB","step":"rows","status":"ok"}`+"\n"), 0o600))
	_, err = LoadState(path)
	assert.ErrorContains(t, err, "empty old_id")
}

func TestBuildPlanClassification(t *testing.T) {
	fx := newFixture()
	fx.company("c-live", "Live Corp", "", liveSFID, "cg-1")
	fx.company("c-dead", "Dead Corp", "", deadSFID, "cg-1")
	fx.company("c-lf", "Legacy Ltd", "", lfID, "cg-1")
	fx.company("c-lf-nomap", "Unmapped Ltd", "", "lf-unmapped", "cg-1")
	fx.company("c-lf-amb", "Ambiguous Ltd", "", "lf-ambiguous", "cg-1")
	fx.company("c-lf-unapproved", "Unapproved Ltd", "", "lf-unapproved", "cg-1")
	fx.company("c-lf-coll", "Collision Ltd", "", "lf-collision", "cg-1")
	fx.company("c-existing-target", "Already There", "", targetSFID2)
	fx.company("c-dup-a", "Dup Inc", "", "lf-dup", "cg-1")
	fx.company("c-dup-b", "Dup Inc", "", "lf-dup")
	fx.company("c-empty", "Empty Inc", "", "", "cg-1")
	fx.company("c-empty-mapped", "Empty Mapped Inc", "", "", "cg-1")
	fx.company("c-empty-unapproved", "Empty Unapproved Inc", "", "", "cg-1")
	fx.company("c-other", "Other Inc", "", "0064100000Te0G7AAJ", "cg-1")
	fx.company("c-other-mapped", "Other Mapped Inc", "", "N/A", "cg-1")
	fx.company("c-inactive", "Inactive", "", liveSFID2)
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[targetSFID] = true
	fx.platform.orgs[lfID] = &Org{ID: lfID, Name: "Legacy Ltd (org-service)", Website: "https://legacy.example"}
	fx.platform.orgs[targetSFID] = &Org{ID: targetSFID, Name: "Legacy Ltd"}

	dir := t.TempDir()
	mapping := writeMapping(t, dir,
		lfID+","+targetSFID+",matched,true",
		deadSFID+","+"0014100000DeadNew1"+",created,",
		"lf-ambiguous,,ambiguous,",
		"lf-unapproved,"+"0014100000Unappr01"+",matched,false",
		"lf-collision,"+targetSFID2+",created,true",
		"lf-dup,"+"0014100000DupNew01"+",created,true",
		"c-empty-mapped,"+"0014100000EmptyNew"+",matched,true",
		"c-empty-unapproved,"+"0014100000EmpUnap1"+",matched,false", // not the Account of 0014100000EmptyNew (ids sharing 15 chars are one Account)
		"c-other-mapped,"+"0014100000OtherNew"+",matched,true",
	)
	opts := Options{Stage: "dev", Mapping: mapping, OutDir: filepath.Join(dir, "out")}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)

	expect := map[string]struct {
		route  Route
		reason string
		newID  string
	}{
		liveSFID:             {RouteRegister, "", ""},
		deadSFID:             {RouteRewrite, "", "0014100000DeadNew1"},
		lfID:                 {RouteRewrite, "", targetSFID},
		"lf-unmapped":        {RouteRewrite, "no_mapping", ""},
		"lf-ambiguous":       {RouteManual, "mapping_ambiguous", ""},
		"lf-unapproved":      {RouteManual, "mapping_not_approved", ""},
		"lf-collision":       {RouteManual, "target_collision", targetSFID2},
		"lf-dup":             {RouteRewrite, "", "0014100000DupNew01"},
		"c-empty":            {RouteManual, "empty_external_id", ""},
		"c-empty-mapped":     {RouteRewrite, "", "0014100000EmptyNew"},
		"c-empty-unapproved": {RouteManual, "mapping_not_approved", ""},
		"c-other":            {RouteManual, "invalid_id_shape", ""},
		"c-other-mapped":     {RouteRewrite, "", "0014100000OtherNew"},
	}
	require.Len(t, plan.Groups, len(expect))
	for key, want := range expect {
		g := groupByKey(plan.Groups, key)
		require.NotNil(t, g, key)
		assert.Equal(t, want.route, g.Route, key)
		assert.Equal(t, want.reason, g.ManualReason, key)
		assert.Equal(t, want.newID, g.NewID, key)
		assert.NoError(t, g.Err, key)
	}
	assert.Equal(t, "live", groupByKey(plan.Groups, liveSFID).Live)
	assert.Equal(t, "dead", groupByKey(plan.Groups, deadSFID).Live)
	assert.True(t, groupByKey(plan.Groups, "c-empty-mapped").RowTargeted())
	assert.True(t, groupByKey(plan.Groups, "c-other-mapped").RowTargeted())
	assert.Equal(t, "N/A", groupByKey(plan.Groups, "c-other-mapped").OldID)
	assert.False(t, groupByKey(plan.Groups, lfID).RowTargeted())
	assert.Equal(t, "200", groupByKey(plan.Groups, lfID).OrgStatus)
	assert.Equal(t, "404", groupByKey(plan.Groups, deadSFID).OrgStatus)
	assert.True(t, groupByKey(plan.Groups, "lf-dup").Duplicate, "same-SFID duplicates are reported, not routed to manual")
	require.Len(t, plan.Register, 1)
	require.Len(t, plan.Rewrite, 5)
	collision := targetByID(plan.Targets, targetSFID2)
	require.NotNil(t, collision)
	assert.Equal(t, "needs_decision", collision.Status)
	assert.Len(t, collision.Existing, 1, "rows already carrying the target count as co-targets")

	// dry run: no writes anywhere, reports written
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, Summary{Stage: "dev", Mode: "dry-run", Eligible: 13, Pending: 1, Manual: 6}, sum)
	assert.Empty(t, fx.companies.updates)
	assert.Empty(t, fx.platform.registered)
	assert.Empty(t, fx.events.rekeyed)
	for _, c := range fx.platform.calls {
		assert.False(t, strings.HasPrefix(c, "create-grant") || strings.HasPrefix(c, "delete-grant") || strings.HasPrefix(c, "register"), c)
	}
	rows := readCSV(t, filepath.Join(dir, "out", "plan.csv"))
	assert.Len(t, rows, 14)
	toSF := readCSV(t, filepath.Join(dir, "out", "to_salesforce.csv"))
	require.Len(t, toSF, 2, "only rewrite candidates without a mapping go to Salesforce")
	assert.Equal(t, []string{"lf-unmapped", "Unmapped Ltd", "", "2024-01-02T00:00:00Z", "", "false"}, toSF[1])
	manual := readCSV(t, filepath.Join(dir, "out", "manual_actions.csv"))
	require.Len(t, manual, 8, "manual + pending groups")
	byKey := map[string][]string{}
	for _, r := range manual[1:] {
		byKey[r[0]] = r
	}
	assert.Equal(t, "target_collision", byKey["lf-collision"][3])
	assert.Contains(t, byKey["lf-collision"][4], "decisions file")
	assert.Equal(t, "no_mapping", byKey["lf-unmapped"][3])
	assert.Equal(t, "", byKey["c-empty"][1], "old_id column is blank for a blank external id")
	assert.Contains(t, byKey["c-empty"][4], "company_id,new_id,matched,true")
	assert.Equal(t, "mapping_not_approved", byKey["c-empty-unapproved"][3])
	targets := readCSV(t, filepath.Join(dir, "out", "targets.csv"))
	require.Len(t, targets, 7)
	assert.Equal(t, "c-empty-mapped", targetByID(plan.Targets, "0014100000EmptyNew").OldIDs()[0], "row-targeted groups are identified by company id in targets")
	assert.Contains(t, fx.out.String(), "")

	var buf bytes.Buffer
	plan.Print(&buf)
	assert.Contains(t, buf.String(), "PLAN register "+liveSFID)
	assert.Contains(t, buf.String(), "PLAN rewrite "+lfID+" -> "+targetSFID+" (matched)")
	assert.Contains(t, buf.String(), "reason=target_collision")
	assert.Contains(t, buf.String(), "TARGETS")
	assert.Contains(t, buf.String(), targetSFID2+" status=needs_decision")
	assert.Contains(t, buf.String(), "MANUAL ACTIONS (7)")
	assert.Contains(t, buf.String(), "PLAN rewrite row c-empty-mapped (company_external_id \"\") -> 0014100000EmptyNew (matched)")
}

func targetByID(targets []*Target, sfid string) *Target {
	for _, t := range targets {
		if t.SFID == sfid {
			return t
		}
	}
	return nil
}

func TestBuildPlanFilters(t *testing.T) {
	fx := newFixture()
	fx.company("c-a", "A", "", liveSFID, "cg-1")
	fx.company("c-b", "B", "", liveSFID2, "cg-1")
	fx.company("c-lf", "L", "", lfID, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[liveSFID2] = true
	fx.platform.sfAccounts[targetSFID] = true
	fx.platform.orgs[targetSFID] = &Org{ID: targetSFID, Name: "L"}
	mapping := writeMapping(t, t.TempDir(), lfID+","+targetSFID+",created,true")

	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Routes: []Route{RouteRegister}, Mapping: mapping})
	require.NoError(t, err)
	assert.Len(t, plan.Register, 2)
	assert.Empty(t, plan.Rewrite, "--routes register never rewrites")
	assert.Equal(t, 1, plan.Skipped)

	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Tranche: 1, Mapping: mapping})
	require.NoError(t, err)
	assert.Equal(t, 1, len(plan.Register)+len(plan.Rewrite))
	assert.Equal(t, 2, plan.Skipped)

	plan, err = BuildPlan(context.Background(), fx.deps(), Options{IDs: []string{targetSFID}, Mapping: mapping})
	require.NoError(t, err)
	require.Len(t, plan.Groups, 1, "--ids accepts the new id")
	assert.Equal(t, lfID, plan.Groups[0].OldID)

	plan, err = BuildPlan(context.Background(), fx.deps(), Options{IDs: []string{liveSFID2}})
	require.NoError(t, err)
	require.Len(t, plan.Groups, 1)
	assert.Equal(t, RouteRegister, plan.Groups[0].Route)

	_, err = BuildPlan(context.Background(), fx.deps(), Options{Mapping: filepath.Join(t.TempDir(), "missing.csv")})
	assert.Error(t, err)
	_, err = BuildPlan(context.Background(), fx.deps(), Options{UseApex: true})
	assert.ErrorIs(t, err, ErrApexUnavailable)
}

func TestLivenessErrorsNeverMeanDead(t *testing.T) {
	fx := newFixture()
	fx.company("c-a", "A", "", liveSFID, "cg-1")
	fx.platform.failGetB2B = true
	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Apply: true})
	require.NoError(t, err)
	g := plan.Groups[0]
	assert.Equal(t, "error", g.Live)
	assert.Error(t, g.Err)
	var authErr *member_service.AuthError
	assert.ErrorAs(t, g.Err, &authErr, "the cause must be visible to the operator")
	assert.Empty(t, plan.Register)
	sum, err := Execute(context.Background(), fx.deps(), Options{Apply: true}, plan)
	assert.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.Empty(t, fx.platform.registered)

	// dry run: the same failure is reported (non-zero exit) and still nothing is written
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{})
	require.NoError(t, err)
	sum, err = Execute(context.Background(), fx.deps(), Options{}, plan)
	assert.ErrorContains(t, err, "1 group(s) failed")
	assert.Equal(t, Summary{Mode: "dry-run", Eligible: 1, Failed: 1}, sum)
	assert.Empty(t, fx.platform.registered)

	// no member-service: liveness is unverified, the group waits (pending) and nothing is registered
	fx = newFixture()
	fx.company("c-a", "A", "", liveSFID, "cg-1")
	fx.platform.noMembers = true
	fx.platform.orgs[liveSFID] = &Org{ID: liveSFID, Name: "A"}
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Apply: true})
	require.NoError(t, err)
	g = plan.Groups[0]
	assert.Equal(t, LiveUnverified, g.Live, "org-service serving an id is not Salesforce liveness")
	assert.Equal(t, RouteRegister, g.Route)
	assert.Equal(t, ReasonCRMUnverified, g.ManualReason)
	assert.Equal(t, "200", g.OrgStatus)
	assert.True(t, g.Pending())
	assert.Empty(t, plan.Register)
	assert.Equal(t, 1, plan.Pending())
	sum, err = Execute(context.Background(), fx.deps(), Options{Apply: true}, plan)
	require.NoError(t, err)
	assert.Equal(t, Summary{Mode: "apply", Eligible: 1, Pending: 1}, sum)
	assert.Empty(t, fx.platform.registered)
	var buf bytes.Buffer
	plan.Print(&buf)
	assert.Contains(t, buf.String(), "pending=1")
	actions := plan.ManualActions()
	require.Len(t, actions, 1)
	assert.Equal(t, ReasonCRMUnverified, actions[0].Reason)
	assert.Contains(t, actions[0].Suggested, "cla-member-service-base-url")
}

func TestApplyWithoutMemberServiceStopsBeforeMutation(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.noMembers = true
	opts := Options{Apply: true, Mapping: mapping, State: statePath, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	assert.ErrorIs(t, err, ErrNotConfigured)
	assert.Equal(t, 1, sum.Failed)
	assert.ErrorIs(t, plan.Rewrite[0].Err, ErrNotConfigured)
	assert.Empty(t, fx.companies.updates)
	assert.Empty(t, fx.events.rekeyed)
	assert.Len(t, mustListOrgGrants(t, fx, lfID), 3)
	for _, c := range fx.platform.calls {
		assert.False(t, strings.HasPrefix(c, "create-grant"), "nothing is written when the run could not finish")
	}
	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.Empty(t, st.Last, "no step was started")
	_, err = os.Stat(filepath.Join(opts.OutDir, "manual_actions.csv"))
	assert.NoError(t, err, "reports are still written")

	// the same apply without a state file is refused before anything else
	fx, mapping, _ = rewriteFixture(t)
	opts = Options{Apply: true, Mapping: mapping}
	plan, err = BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Register, 1)
	sum, err = Execute(context.Background(), fx.deps(), opts, plan)
	assert.ErrorIs(t, err, ErrStateRequired)
	assert.Equal(t, 2, sum.Failed, "the register group is refused too: the run stops before its first write")
	assert.Empty(t, fx.platform.registered)
	assert.Empty(t, fx.companies.updates)
	assert.Len(t, mustListOrgGrants(t, fx, lfID), 3)
	// a register-only apply needs no journal
	fx = newFixture()
	fx.company("c-a", "A", "", liveSFID, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Apply: true, Routes: []Route{RouteRegister}})
	require.NoError(t, err)
	sum, err = Execute(context.Background(), fx.deps(), Options{Apply: true, Routes: []Route{RouteRegister}}, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Registered)
}

func TestJournalFailureStopsBeforeRowRewrite(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	require.NoError(t, os.WriteFile(statePath, nil, 0o400))
	opts := Options{Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	assert.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.ErrorContains(t, plan.Rewrite[0].Err, "cannot record")
	assert.Empty(t, fx.companies.updates, "rows are not rewritten when the journal cannot be written")
	assert.Empty(t, fx.events.rekeyed)
	assert.Len(t, mustListOrgGrants(t, fx, lfID), 3, "old grants are kept")
	for _, c := range fx.platform.calls {
		assert.False(t, strings.HasPrefix(c, "create-grant"), "the first journal line precedes every write")
	}
}

func TestCleanupPreservesUncopiedGrant(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	// a grant appears on the old org after the copy step (an ACS write racing the import)
	lists := 0
	fx.platform.listHook = func(orgID string) {
		if orgID == lfID {
			if lists++; lists == 2 {
				fx.platform.addGrant(lfID, "late-user", "role-cla-manager", "cg-1")
			}
		}
	}
	opts := Options{Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	newGrants := mustListOrgGrants(t, fx, targetSFID)
	var users []string
	for _, g := range newGrants {
		users = append(users, g.Username)
	}
	assert.Contains(t, users, "late-user", "a grant found only at cleanup time is copied before the old one is deleted")
	assert.Empty(t, mustListOrgGrants(t, fx, lfID))
}

func TestRegisterApply(t *testing.T) {
	fx := newFixture()
	fx.company("c-a", "A", "", liveSFID, "cg-1")
	fx.company("c-b", "B", "", liveSFID2, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[liveSFID2] = true
	plan, err := BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Apply: true})
	require.NoError(t, err)
	require.Len(t, plan.Register, 2)
	delete(fx.platform.sfAccounts, liveSFID2) // account deleted between GET and POST

	sum, err := Execute(context.Background(), fx.deps(), Options{Apply: true}, plan)
	assert.Error(t, err)
	assert.Equal(t, Summary{Stage: "dev", Mode: "apply", Eligible: 2, Registered: 1, Failed: 1}, sum)
	assert.Equal(t, []string{liveSFID}, fx.platform.registered)
	dead := groupByKey(plan.Groups, liveSFID2)
	assert.Equal(t, "dead_account", dead.ManualReason)
	assert.Equal(t, RouteRewrite, dead.Route)
	assert.Empty(t, fx.companies.updates)

	// repeat is a no-op on the EasyCLA side and idempotent on the member-service side
	fx.platform.sfAccounts[liveSFID2] = true
	plan, err = BuildPlan(context.Background(), fx.deps(), Options{Stage: "dev", Apply: true})
	require.NoError(t, err)
	sum, err = Execute(context.Background(), fx.deps(), Options{Apply: true}, plan)
	require.NoError(t, err)
	assert.Equal(t, 2, sum.Registered)
}

func mustListOrgGrants(t *testing.T, fx *fixture, sfid string) []acs_service.OrgGrant {
	grants, err := fx.platform.ListOrgGrants(context.Background(), sfid)
	require.NoError(t, err)
	return grants
}

func rewriteFixture(t *testing.T) (*fixture, string, string) {
	t.Helper()
	fx := newFixture()
	fx.company("c-parent", "Legacy Ltd", "", lfID, "cg-1")
	fx.company("c-sub", "Legacy Ltd", "Legacy Ltd Europe", lfID, "cg-2")
	fx.company("c-other", "Other", "", liveSFID, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[targetSFID] = true
	fx.platform.orgs[lfID] = &Org{ID: lfID, Name: "Legacy Ltd", Website: "https://legacy.example"}
	fx.platform.orgs[targetSFID] = &Org{ID: targetSFID, Name: "Legacy Ltd"}
	fx.platform.addGrant(lfID, "alice", "role-mgr", "")
	fx.platform.addGrant(lfID, "bob", "role-mgr", "proj-1")
	fx.platform.addGrant(lfID, "carol", "role-signatory", "")
	fx.platform.addGrant(targetSFID, "alice", "role-mgr", "") // already granted on the new org
	fx.platform.addGrant(liveSFID, "dave", "role-mgr", "")
	fx.events.sfidByEvent = map[string]string{"e1": lfID, "e2": lfID, "e3": liveSFID, "e4": lfID}
	fx.events.byGroup = map[string][]string{lfID + "#cg-2": {"e4"}}
	dir := t.TempDir()
	mapping := writeMapping(t, dir, lfID+","+targetSFID+",matched,true")
	return fx, mapping, filepath.Join(dir, "state.jsonl")
}

func TestRewriteApply(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, Summary{Stage: "dev", Mode: "apply", Eligible: 2, Rewritten: 1}, sum)

	assert.Equal(t, []string{"c-parent:" + lfID + "->" + targetSFID, "c-sub:" + lfID + "->" + targetSFID}, fx.companies.updates)
	assert.Equal(t, targetSFID, fx.companies.rows["c-sub"].CompanyExternalID)
	assert.Equal(t, lfID, fx.companies.rows["c-sub"].PreviousCompanyExternalID)
	assert.Equal(t, liveSFID, fx.companies.rows["c-other"].CompanyExternalID, "rows outside the group are untouched")

	newGrants := mustListOrgGrants(t, fx, targetSFID)
	keys := map[string]bool{}
	for _, g := range newGrants {
		keys[g.Username+":"+g.ObjectID] = true
	}
	assert.Equal(t, map[string]bool{"alice:" + targetSFID: true, "bob:proj-1|" + targetSFID: true, "carol:" + targetSFID: true}, keys)
	assert.Len(t, newGrants, 3, "existing grant is not duplicated")
	oldGrants := mustListOrgGrants(t, fx, lfID)
	assert.Empty(t, oldGrants)
	otherGrants := mustListOrgGrants(t, fx, liveSFID)
	assert.Len(t, otherGrants, 1)

	assert.ElementsMatch(t, []string{"e1", "e2", "e4"}, fx.events.rekeyed)
	assert.Equal(t, liveSFID, fx.events.sfidByEvent["e3"])
	assert.Equal(t, []string{targetSFID}, fx.platform.registered)

	// ordering: grants copied before rows rewritten, old grants deleted after, register last
	var firstCreate, firstDelete, register int
	for i, c := range fx.platform.calls {
		switch {
		case strings.HasPrefix(c, "create-grant") && firstCreate == 0:
			firstCreate = i + 1
		case strings.HasPrefix(c, "delete-grant") && firstDelete == 0:
			firstDelete = i + 1
		case strings.HasPrefix(c, "register"):
			register = i + 1
		}
	}
	assert.True(t, firstCreate > 0 && firstDelete > firstCreate && register > firstDelete, fx.platform.calls)

	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.True(t, st.Done(lfID))
	assert.Equal(t, []string{"c-parent", "c-sub"}, st.Last[lfID].CompanyIDs)

	// second run converges: nothing left to do, the group is skipped as done
	plan, err = BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	assert.Empty(t, plan.Rewrite)
	assert.Nil(t, groupByKey(plan.Groups, lfID), "rows now carry the new id")
	g := groupByKey(plan.Groups, targetSFID)
	require.NotNil(t, g)
	assert.Equal(t, RouteRegister, g.Route)
	assert.Len(t, g.Rows, 2, "the group keeps both rewritten rows")
}

func TestRewriteReplayAfterCrash(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	// simulate a run that died after rewriting rows and re-keying events, before deleting old grants
	fx.companies.rows["c-parent"].CompanyExternalID, fx.companies.rows["c-parent"].PreviousCompanyExternalID = targetSFID, lfID
	fx.companies.rows["c-sub"].CompanyExternalID, fx.companies.rows["c-sub"].PreviousCompanyExternalID = targetSFID, lfID
	fx.platform.addGrant(targetSFID, "bob", "role-mgr", "proj-1")
	fx.platform.addGrant(targetSFID, "carol", "role-signatory", "")
	st, err := LoadState(statePath)
	require.NoError(t, err)
	require.NoError(t, st.Append(StateRecord{OldID: lfID, NewID: targetSFID, CompanyIDs: []string{"c-parent", "c-sub"}, Step: StepEvents, Status: "ok"}, time.Now()))

	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	g := plan.Rewrite[0]
	assert.True(t, g.Replayed)
	assert.Equal(t, lfID, g.OldID)
	assert.Equal(t, targetSFID, g.NewID)
	assert.Nil(t, groupByKey(plan.Groups, targetSFID), "the current grouping under the new id is superseded by the replay")
	assert.Equal(t, []string{"cg-1", "cg-2"}, g.CLAGroupIDs())

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Empty(t, fx.companies.updates, "rows were already rewritten: condition failure + read-back means done")
	newGrants := mustListOrgGrants(t, fx, targetSFID)
	assert.Len(t, newGrants, 3, "no duplicate grants on replay")
	oldGrants := mustListOrgGrants(t, fx, lfID)
	assert.Empty(t, oldGrants)
	assert.Equal(t, []string{liveSFID, targetSFID}, fx.platform.registered, "default routes: c-other is registered too")
	assert.ElementsMatch(t, []string{"e1", "e2", "e4"}, fx.events.rekeyed)
	st, err = LoadState(statePath)
	require.NoError(t, err)
	assert.True(t, st.Done(lfID))
}

// completedRewrite runs rewriteFixture's group to done and clears the fakes' call logs.
func completedRewrite(t *testing.T) (*fixture, string, string) {
	t.Helper()
	fx, mapping, statePath := rewriteFixture(t)
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	_, err = Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"e1", "e2", "e4"}, fx.events.rekeyed)
	fx.events.rekeyed, fx.events.listed, fx.companies.updates = nil, nil, nil
	fx.out.Reset()
	return fx, mapping, statePath
}

func runIngest(t *testing.T, fx *fixture, opts Options) (Summary, error) {
	t.Helper()
	fx.out.Reset()
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	return Execute(context.Background(), fx.deps(), opts, plan)
}

const (
	twoLfID   = "lf-legacy-0002"
	twoSFID   = "0014100000TwoTwoTw"
	threeLfID = "lf-legacy-0003"
	threeSFID = "0014100000ThreeThr"
)

// appendDone journals rec as a completed rewrite group.
func appendDone(t *testing.T, statePath string, rec StateRecord) {
	t.Helper()
	st, err := LoadState(statePath)
	require.NoError(t, err)
	rec.Step, rec.Status = StepDone, statusOK
	require.NoError(t, st.Append(rec, time.Now()))
}

func TestRecheckRekeysLateEventsOfCompletedGroups(t *testing.T) {
	fx, mapping, statePath := completedRewrite(t)
	// e5: served only by the per-CLA-group index during the rewrite; e6: written under the old id afterwards
	fx.events.sfidByEvent["e5"], fx.events.sfidByEvent["e6"] = lfID, lfID
	fx.events.byGroup[lfID+"#cg-2"] = append(fx.events.byGroup[lfID+"#cg-2"], "e5")
	fx.events.lagging = map[string]bool{"e5": true}
	opts := Options{Stage: "dev", Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}

	sum, err := runIngest(t, fx, opts)
	require.NoError(t, err)
	assert.Equal(t, Summary{Stage: "dev", Mode: ModeDryRun, Eligible: 2}, sum)
	assert.Empty(t, fx.events.rekeyed, "a dry run never writes")
	assert.Contains(t, fx.out.String(), "events recheck "+lfID+" -> "+targetSFID+": 2 listed (dry run)\n")

	opts.Apply = true
	sum, err = runIngest(t, fx, opts)
	require.NoError(t, err)
	assert.Equal(t, Summary{Stage: "dev", Mode: ModeApply, Eligible: 2}, sum, "the completed group is not rewritten again")
	assert.ElementsMatch(t, []string{"e5", "e6"}, fx.events.rekeyed)
	assert.Equal(t, targetSFID, fx.events.sfidByEvent["e5"])
	assert.Empty(t, fx.companies.updates)
	out := fx.out.String()
	assert.Contains(t, out, "events recheck "+lfID+" -> "+targetSFID+": 2 listed, 2 re-keyed\n")
	assert.Contains(t, out, "events recheck: 1 previously completed group(s) checked, 2 event(s) listed, 2 re-keyed, 0 failed, 0 skipped (tranche budget)\n")
	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.True(t, st.Done(lfID), "the recheck is never journaled")
	assert.Equal(t, StepDone, st.Last[lfID].Step)

	// converged: 0 listed, and the group keeps being rechecked by later runs
	fx.events.rekeyed, fx.events.listed = nil, nil
	_, err = runIngest(t, fx, opts)
	require.NoError(t, err)
	assert.Empty(t, fx.events.rekeyed)
	assert.Contains(t, fx.out.String(), "events recheck "+lfID+" -> "+targetSFID+": 0 listed, 0 re-keyed\n")
	assert.Equal(t, []string{lfID}, fx.events.listed)
}

func TestRecheckSelection(t *testing.T) {
	fx, mapping, statePath := completedRewrite(t)
	fx.company("c-two", "Two Inc", "", twoSFID, "cg-1")
	fx.platform.sfAccounts[twoSFID] = true
	appendDone(t, statePath, StateRecord{OldID: twoLfID, NewID: twoSFID, CompanyIDs: []string{"c-two"}})
	// row-targeted: nothing in the events table is keyed by its old value
	appendDone(t, statePath, StateRecord{OldID: "malformed", Key: "c-row", NewID: targetSFID, CompanyIDs: []string{"c-row"}})
	fx.events.sfidByEvent["e6"], fx.events.sfidByEvent["t1"] = lfID, twoLfID
	base := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}

	// register-only runs never touch events
	_, err := runIngest(t, fx, Options{Stage: "dev", Apply: true, State: statePath, Routes: []Route{RouteRegister}})
	require.NoError(t, err)
	assert.Empty(t, fx.events.listed)
	assert.NotContains(t, fx.out.String(), "events recheck")

	// --ids narrows the recheck (old or new id)
	opts := base
	opts.IDs = []string{twoSFID}
	_, err = runIngest(t, fx, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{twoLfID}, fx.events.listed)
	assert.Equal(t, []string{"t1"}, fx.events.rekeyed)
	assert.Equal(t, twoSFID, fx.events.sfidByEvent["t1"])

	// the tranche budget left after the planned groups bounds the recheck, in key order; row-targeted groups are never listed
	fx.events.listed, fx.events.rekeyed = nil, nil
	opts = base
	opts.Tranche = 1
	_, err = runIngest(t, fx, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{lfID}, fx.events.listed)
	assert.Equal(t, []string{"e6"}, fx.events.rekeyed)
	assert.Contains(t, fx.out.String(), "events recheck: 1 previously completed group(s) checked, 1 event(s) listed, 1 re-keyed, 0 failed, 1 skipped (tranche budget)\n")

	// a planned rewrite consumes the budget first and is itself left to the next run
	fx.company("c-three", "Three Inc", "", threeLfID, "cg-1")
	fx.platform.sfAccounts[threeSFID] = true
	fx.platform.orgs[threeSFID] = &Org{ID: threeSFID, Name: "Three Inc"}
	opts.Mapping = writeMapping(t, filepath.Dir(statePath), lfID+","+targetSFID+",matched,true", threeLfID+","+threeSFID+",matched,true")
	fx.events.listed = nil
	sum, err := runIngest(t, fx, opts)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Equal(t, []string{threeLfID}, fx.events.listed, "only the rewrite step listed events")
	assert.Contains(t, fx.out.String(), "events recheck: 0 previously completed group(s) checked, 0 event(s) listed, 0 re-keyed, 0 failed, 2 skipped (tranche budget)\n")

	// without --tranche every completed group is rechecked
	fx.events.listed = nil
	opts.Tranche = 0
	_, err = runIngest(t, fx, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{lfID, twoLfID, threeLfID}, fx.events.listed)
	assert.NotContains(t, fx.events.listed, "malformed")
}

func TestRecheckRefusesDriftedRows(t *testing.T) {
	fx, mapping, statePath := completedRewrite(t)
	fx.company("c-moved", "Moved Inc", "", liveSFID, "cg-1")
	appendDone(t, statePath, StateRecord{OldID: twoLfID, NewID: twoSFID, CompanyIDs: []string{"c-moved"}}) // re-pointed by hand since
	appendDone(t, statePath, StateRecord{OldID: threeLfID, NewID: threeSFID, CompanyIDs: []string{"c-gone"}})
	fx.events.sfidByEvent["e6"], fx.events.sfidByEvent["t1"] = lfID, twoLfID
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}

	sum, err := runIngest(t, fx, opts)
	require.EqualError(t, err, "2 group(s) failed")
	assert.Equal(t, 2, sum.Failed)
	assert.Equal(t, []string{lfID}, fx.events.listed, "a drifted group is not listed")
	assert.Equal(t, []string{"e6"}, fx.events.rekeyed, "the healthy group is still rechecked")
	assert.Equal(t, twoLfID, fx.events.sfidByEvent["t1"], "never re-keyed towards a stale destination")
	out := fx.out.String()
	assert.Contains(t, out, "FAILED events recheck "+twoLfID+" -> "+twoSFID+": company c-moved carries \""+liveSFID+"\", not the recorded "+twoSFID+"\n")
	assert.Contains(t, out, "FAILED events recheck "+threeLfID+" -> "+threeSFID+": company c-gone is gone\n")
	assert.Contains(t, out, "events recheck: 1 previously completed group(s) checked, 1 event(s) listed, 1 re-keyed, 2 failed, 0 skipped (tranche budget)\n")
	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.True(t, st.Done(twoLfID), "the journal is not rewritten")
}

func TestRecheckErrorsExitNonZeroAndRetryNextRun(t *testing.T) {
	fx, mapping, statePath := completedRewrite(t)
	fx.events.sfidByEvent["e6"] = lfID
	fx.events.listErr = map[string]error{lfID: errors.New("throttled")}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}

	sum, err := runIngest(t, fx, opts)
	require.EqualError(t, err, "1 group(s) failed")
	assert.Equal(t, 1, sum.Failed)
	assert.Contains(t, fx.out.String(), "FAILED events recheck "+lfID+" -> "+targetSFID+": listing events of "+lfID+": throttled\n")
	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.True(t, st.Done(lfID))

	fx.events.listErr = nil
	_, err = runIngest(t, fx, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"e6"}, fx.events.rekeyed)

	fx.events.sfidByEvent["e7"] = lfID
	fx.events.rekeyErr = map[string]error{"e7": errors.New("conditional check failed")}
	sum, err = runIngest(t, fx, opts)
	require.EqualError(t, err, "1 group(s) failed")
	assert.Equal(t, 1, sum.Failed)
	assert.Contains(t, fx.out.String(), "FAILED events recheck "+lfID+" -> "+targetSFID+": event e7: conditional check failed\n")

	// a dry run reports listing failures the same way
	opts.Apply = false
	fx.events.listErr = map[string]error{lfID: errors.New("throttled")}
	sum, err = runIngest(t, fx, opts)
	require.EqualError(t, err, "1 group(s) failed")
	assert.Equal(t, ModeDryRun, sum.Mode)
	assert.Equal(t, targetSFID, fx.events.sfidByEvent["e6"])
	assert.Equal(t, lfID, fx.events.sfidByEvent["e7"])
}

// A replayed group carries fresh rows, so the inventory of this run still shows its old id after the
// rewrite: the group is not rechecked by the run that completes it, but by the next one.
func TestRecheckLeavesGroupsRewrittenByThisRunToTheNextRun(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "resumed before rows"
		if partial {
			name = "resumed with partially rewritten rows"
		}
		t.Run(name, func(t *testing.T) {
			fx, mapping, statePath := rewriteFixture(t)
			if partial {
				fx.companies.rows["c-parent"].CompanyExternalID, fx.companies.rows["c-parent"].PreviousCompanyExternalID = targetSFID, lfID
			}
			st, err := LoadState(statePath)
			require.NoError(t, err)
			require.NoError(t, st.Append(StateRecord{OldID: lfID, NewID: targetSFID, CompanyIDs: []string{"c-parent", "c-sub"}, Step: StepRows, Status: statusFailed}, time.Now()))
			opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
			plan, err := BuildPlan(context.Background(), fx.deps(), opts)
			require.NoError(t, err)
			require.Len(t, plan.Rewrite, 1)
			require.True(t, plan.Rewrite[0].Replayed)
			sum, err := Execute(context.Background(), fx.deps(), opts, plan)
			require.NoError(t, err, fx.out.String())
			assert.Equal(t, Summary{Stage: "dev", Mode: ModeApply, Eligible: 2, Rewritten: 1}, sum)
			assert.Equal(t, targetSFID, fx.companies.rows["c-parent"].CompanyExternalID)
			assert.Equal(t, targetSFID, fx.companies.rows["c-sub"].CompanyExternalID)
			assert.ElementsMatch(t, []string{"e1", "e2", "e4"}, fx.events.rekeyed)
			assert.NotContains(t, fx.out.String(), "events recheck")

			fx.events.sfidByEvent["e6"] = lfID
			_, err = runIngest(t, fx, opts)
			require.NoError(t, err)
			assert.Contains(t, fx.out.String(), "events recheck "+lfID+" -> "+targetSFID+": 1 listed, 1 re-keyed\n")
			assert.Equal(t, targetSFID, fx.events.sfidByEvent["e6"])
		})
	}
}

func TestRecheckRepairsLateEventAfterEmptyResultAndContinuesPastErrors(t *testing.T) {
	fx, mapping, statePath := completedRewrite(t)
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
	_, err := runIngest(t, fx, opts)
	require.NoError(t, err)
	require.Contains(t, fx.out.String(), "0 event(s) listed")
	fx.events.sfidByEvent["late-after-empty"] = lfID
	_, err = runIngest(t, fx, opts)
	require.NoError(t, err)
	assert.Equal(t, targetSFID, fx.events.sfidByEvent["late-after-empty"])

	fx.company("c-two", "Two Inc", "", twoSFID, "cg-1")
	fx.platform.sfAccounts[twoSFID] = true
	appendDone(t, statePath, StateRecord{OldID: twoLfID, NewID: twoSFID, CompanyIDs: []string{"c-two"}})
	fx.events.sfidByEvent["next-group"] = twoLfID
	fx.events.listErr = map[string]error{lfID: errors.New("throttled")}
	sum, err := runIngest(t, fx, opts)
	require.EqualError(t, err, "1 group(s) failed")
	assert.Equal(t, 1, sum.Failed)
	assert.Equal(t, twoSFID, fx.events.sfidByEvent["next-group"], "the next healthy group is still repaired")
}

func TestRowTargetedRewriteApply(t *testing.T) {
	fx := newFixture()
	fx.company("c-empty", "Empty Inc", "", "", "cg-1")
	fx.company("c-empty-2", "Other Empty Inc", "", "", "cg-1")
	fx.company("c-garbage", "Garbage Inc", "", "N/A", "cg-1")
	fx.company("c-garbage-2", "Garbage Two", "", "N/A", "cg-1")
	fx.company("c-live", "Live", "", liveSFID, "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[targetSFID] = true
	fx.platform.sfAccounts[targetSFID2] = true
	fx.platform.orgs[targetSFID] = &Org{ID: targetSFID, Name: "Empty Inc"}
	fx.platform.orgs[targetSFID2] = &Org{ID: targetSFID2, Name: "Garbage Inc"}
	fx.platform.addGrant(liveSFID, "dave", "role-mgr", "")
	fx.events.sfidByEvent = map[string]string{"e1": liveSFID, "e2": "N/A", "e3": ""}
	dir := t.TempDir()
	mapping := writeMapping(t, dir, "c-empty,"+targetSFID+",matched,true", "c-garbage,"+targetSFID2+",matched,true")
	statePath := filepath.Join(dir, "state.jsonl")
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}

	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 2)
	for _, key := range []string{"c-empty-2", "c-garbage-2"} {
		g := groupByKey(plan.Groups, key)
		require.NotNil(t, g, key)
		assert.Equal(t, RouteManual, g.Route, "an unmapped blank/garbage row is never grouped with a mapped one")
		assert.Len(t, g.Rows, 1)
	}
	var out bytes.Buffer
	plan.Print(&out)
	assert.Contains(t, out.String(), "PLAN rewrite row c-empty (company_external_id \"\") -> "+targetSFID)
	assert.Contains(t, out.String(), "PLAN rewrite row c-garbage (company_external_id \"N/A\") -> "+targetSFID2)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 2, sum.Rewritten)
	assert.Equal(t, []string{"c-empty:->" + targetSFID, "c-garbage:N/A->" + targetSFID2}, fx.companies.updates)
	assert.Equal(t, "", fx.companies.rows["c-empty-2"].CompanyExternalID, "the other blank row is untouched")
	assert.Equal(t, "N/A", fx.companies.rows["c-garbage-2"].CompanyExternalID, "the other N/A row is untouched")
	assert.Empty(t, fx.events.rekeyed, "nothing is re-keyed under a blank or garbage old value")
	for _, c := range fx.platform.calls {
		assert.False(t, strings.HasPrefix(c, "create-grant") || strings.HasPrefix(c, "delete-grant"), c)
	}
	assert.Equal(t, []string{targetSFID, targetSFID2}, fx.platform.registered)
	assert.Contains(t, fx.out.String(), "grants and events are not moved")

	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.True(t, st.Done("c-empty"))
	assert.True(t, st.Done("c-garbage"))
	assert.False(t, st.Done(""))
	assert.Equal(t, "", st.Last["c-empty"].OldID)
	assert.Equal(t, "c-empty", st.Last["c-empty"].Key)
	assert.Equal(t, "N/A", st.Last["c-garbage"].OldID)

	// second run converges: the rewritten rows now form register groups under their new ids
	plan, err = BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	assert.Empty(t, plan.Rewrite)
	assert.Nil(t, groupByKey(plan.Groups, "c-empty"))
	assert.Equal(t, RouteRegister, groupByKey(plan.Groups, targetSFID).Route)
	assert.Equal(t, RouteRegister, groupByKey(plan.Groups, targetSFID2).Route)
}

func TestWhitespaceSourceValueIsPinnedExactly(t *testing.T) {
	fx := newFixture()
	fx.company("c-space", "Space Inc", "", " ", "cg-1")
	fx.company("c-padded", "Padded Inc", "", " "+liveSFID+" ", "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.sfAccounts[targetSFID] = true
	fx.platform.orgs[targetSFID] = &Org{ID: targetSFID, Name: "Space Inc"}
	dir := t.TempDir()
	mapping := writeMapping(t, dir, "c-space,"+targetSFID+",matched,true")
	opts := Options{Apply: true, Mapping: mapping, State: filepath.Join(dir, "state.jsonl"), Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	assert.Equal(t, " ", plan.Rewrite[0].OldID, "the stored value, not its trimmed form, is the CAS precondition")
	assert.Equal(t, " ", plan.Rewrite[0].Rows[0].RawExternalID)
	assert.Equal(t, "", plan.Rewrite[0].Rows[0].ExternalID)
	padded := groupByKey(plan.Groups, liveSFID)
	require.NotNil(t, padded, "a padded SFID is grouped under its trimmed id")
	assert.Equal(t, " "+liveSFID+" ", padded.Rows[0].RawExternalID)
	assert.Equal(t, RouteRegister, padded.Route)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Equal(t, []string{"c-space: ->" + targetSFID}, fx.companies.updates, "the fake compares the exact stored value")
	assert.Equal(t, targetSFID, fx.companies.rows["c-space"].CompanyExternalID)
	assert.Equal(t, " ", fx.companies.rows["c-space"].PreviousCompanyExternalID)
	st, err := LoadState(opts.State)
	require.NoError(t, err)
	assert.Equal(t, " ", st.Last["c-space"].OldID)

	// a replayed row-targeted group keeps the exact value too
	fx = newFixture()
	fx.company("c-space", "Space Inc", "", " ", "cg-1")
	fx.platform.sfAccounts[targetSFID] = true
	fx.platform.orgs[targetSFID] = &Org{ID: targetSFID, Name: "Space Inc"}
	statePath := filepath.Join(t.TempDir(), "state.jsonl")
	st, err = LoadState(statePath)
	require.NoError(t, err)
	require.NoError(t, st.Append(StateRecord{Key: "c-space", OldID: " ", NewID: targetSFID, CompanyIDs: []string{"c-space"}, Step: StepStart, Status: statusOK}, time.Now()))
	opts = Options{Apply: true, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err = BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	assert.True(t, plan.Rewrite[0].Replayed)
	assert.Equal(t, " ", plan.Rewrite[0].OldID)
	sum, err = Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Equal(t, targetSFID, fx.companies.rows["c-space"].CompanyExternalID)
}

func TestActiveCCLAPredicateRequiresCompanyReference(t *testing.T) {
	fx := newFixture()
	fx.company("c-a", "A", "", liveSFID)
	// a signed+approved signature that is not a company CCLA (user reference / not ccla / unsigned) never makes the row eligible
	fx.sigs.items = append(fx.sigs.items,
		&signatures.ItemSignature{SignatureReferenceID: "c-a", SignatureReferenceType: utils.SignatureReferenceTypeUser, SignatureProjectID: "cg-1", SignatureType: "ccla", SignatureSigned: true, SignatureApproved: true},
		&signatures.ItemSignature{SignatureReferenceID: "c-a", SignatureReferenceType: utils.SignatureReferenceTypeCompany, SignatureProjectID: "cg-1", SignatureType: "cla", SignatureSigned: true, SignatureApproved: true},
		&signatures.ItemSignature{SignatureReferenceID: "c-a", SignatureReferenceType: utils.SignatureReferenceTypeCompany, SignatureProjectID: "cg-1", SignatureType: "ccla", SignatureSigned: false, SignatureApproved: true},
		&signatures.ItemSignature{SignatureReferenceID: "c-a", SignatureReferenceType: utils.SignatureReferenceTypeCompany, SignatureProjectID: "cg-1", SignatureType: "ccla", SignatureSigned: true, SignatureApproved: false},
	)
	inv, err := LoadInventory(context.Background(), fx.deps())
	require.NoError(t, err)
	assert.Empty(t, inv.EligibleGroups())
	fx.sigs.items = append(fx.sigs.items, &signatures.ItemSignature{SignatureReferenceID: "c-a", SignatureReferenceType: utils.SignatureReferenceTypeCompany, SignatureProjectID: "cg-1", SignatureType: "ccla", SignatureSigned: true, SignatureApproved: true})
	inv, err = LoadInventory(context.Background(), fx.deps())
	require.NoError(t, err)
	require.Len(t, inv.EligibleGroups(), 1)
	assert.Equal(t, 1, inv.EligibleGroups()[0].Rows[0].CCLACount)
}

func TestRowTargetedReplayAfterCrash(t *testing.T) {
	fx := newFixture()
	fx.company("c-empty", "Empty Inc", "", "", "cg-1")
	fx.platform.sfAccounts[targetSFID] = true
	fx.platform.orgs[targetSFID] = &Org{ID: targetSFID, Name: "Empty Inc"}
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.jsonl")
	st, err := LoadState(statePath)
	require.NoError(t, err)
	// died after the rows step: the row already carries the new id, register is still pending
	fx.companies.rows["c-empty"].CompanyExternalID = targetSFID
	require.NoError(t, st.Append(StateRecord{OldID: "", Key: "c-empty", NewID: targetSFID, CompanyIDs: []string{"c-empty"}, Step: StepRows, Status: "ok"}, time.Now()))

	opts := Options{Stage: "dev", Apply: true, State: statePath}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	g := plan.Rewrite[0]
	assert.True(t, g.Replayed)
	assert.True(t, g.RowTargeted())
	assert.Equal(t, "c-empty", g.Key)
	assert.Equal(t, "", g.OldID)
	assert.Nil(t, groupByKey(plan.Groups, targetSFID), "the current grouping under the new id is superseded by the replay")

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Empty(t, fx.companies.updates, "row already rewritten: condition failure + read-back means done")
	assert.Equal(t, []string{targetSFID}, fx.platform.registered)
	st, err = LoadState(statePath)
	require.NoError(t, err)
	assert.True(t, st.Done("c-empty"))
}

func TestRewriteConflictStopsGroup(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.companies.rows["c-sub"].CompanyExternalID = somebodySFID // someone re-pointed a sibling meanwhile
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	require.Equal(t, []string{"c-parent"}, plan.Rewrite[0].CompanyIDs())
	fx.companies.rows["c-parent"].CompanyExternalID = somebodySFID // and the parent, after planning

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	assert.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.ErrorContains(t, plan.Rewrite[0].Err, "conflict")
	oldGrants := mustListOrgGrants(t, fx, lfID)
	assert.Len(t, oldGrants, 3, "old grants are never deleted after a conflict")
	assert.Empty(t, fx.platform.registered)
	assert.Empty(t, fx.events.rekeyed)
	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.Equal(t, StepRows, st.Last[lfID].Step)
	assert.Equal(t, "failed", st.Last[lfID].Status)
}

func TestRewriteGrantFailureLeavesRowsUntouched(t *testing.T) {
	fx, mapping, _ := rewriteFixture(t)
	fx.platform.failCreate = true
	opts := Options{Apply: true, Mapping: mapping, Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	_, err = Execute(context.Background(), fx.deps(), opts, plan)
	assert.Error(t, err)
	assert.Empty(t, fx.companies.updates, "managers never lose access: rows are rewritten only after grants exist on the new org")
	assert.Equal(t, lfID, fx.companies.rows["c-parent"].CompanyExternalID)
}

func TestRewriteWaitsForOrgService(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.servedAfter[targetSFID] = 2
	opts := Options{Apply: true, Mapping: mapping, State: statePath, WaitPoll: time.Second, WaitMax: time.Hour}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Equal(t, []time.Duration{time.Second, time.Second}, fx.sleeps)

	fx, mapping, statePath = rewriteFixture(t)
	fx.platform.servedAfter[targetSFID] = 5
	opts = Options{Apply: true, Mapping: mapping, State: statePath, SkipWait: true, Routes: []Route{RouteRewrite}}
	plan, err = BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	sum, err = Execute(context.Background(), fx.deps(), opts, plan)
	assert.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.ErrorContains(t, plan.Rewrite[0].Err, "does not serve")
	assert.Empty(t, fx.sleeps)
	assert.Empty(t, fx.companies.updates)
	for _, c := range fx.platform.calls {
		assert.False(t, strings.HasPrefix(c, "create-grant"), "no grant copied before the new org is served")
	}
}

func assertNoCutover(t *testing.T, fx *fixture) {
	t.Helper()
	assert.Empty(t, fx.companies.updates, "no row rewritten")
	assert.Empty(t, fx.events.rekeyed, "no event re-keyed")
	assert.Empty(t, fx.platform.registered, "nothing registered in B2B")
	for _, c := range fx.platform.calls {
		assert.False(t, strings.HasPrefix(c, "create-grant") || strings.HasPrefix(c, "delete-grant"), "no grant touched: %s", c)
	}
}

func TestRewriteFailsWhenOrgServiceServesAnotherID(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	fx.platform.orgs[targetSFID] = &Org{ID: strings.ToLower(targetSFID), Name: "Legacy Ltd"}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	assert.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.Zero(t, sum.Rewritten)
	assert.ErrorContains(t, plan.Rewrite[0].Err, "serves target "+targetSFID+" as "+strconv.Quote(strings.ToLower(targetSFID)))
	assertNoCutover(t, fx)
	assert.Equal(t, lfID, fx.companies.rows["c-parent"].CompanyExternalID)
	assert.Equal(t, lfID, fx.companies.rows["c-sub"].CompanyExternalID)
	assert.Empty(t, fx.sleeps, "a served-under-another-id target is not retried")

	st, err := LoadState(statePath)
	require.NoError(t, err)
	rec, ok := st.Last[lfID]
	require.True(t, ok)
	assert.Equal(t, StepWait, rec.Step)
	assert.Equal(t, statusFailed, rec.Status)
	assert.Equal(t, targetSFID, rec.NewID, "the journal keeps the reviewed id, never the other spelling")
	assert.Contains(t, rec.Err, "serves target")
	assert.False(t, st.Done(lfID))

	// the second run replays the journaled group and fails the same way: nothing is cut over
	plan, err = BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	assert.True(t, plan.Rewrite[0].Replayed)
	assert.Equal(t, targetSFID, plan.Rewrite[0].NewID)
	sum, err = Execute(context.Background(), fx.deps(), opts, plan)
	assert.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assertNoCutover(t, fx)
}

func TestReplayFailsWhenOrgServiceServesAnotherID(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	// a run died after the resolve step; before resuming, org-service serves the target under another spelling
	st, err := LoadState(statePath)
	require.NoError(t, err)
	require.NoError(t, st.Append(StateRecord{OldID: lfID, NewID: targetSFID, CompanyIDs: []string{"c-parent", "c-sub"}, Step: StepResolve, Status: statusOK}, time.Now()))
	fx.platform.orgs[targetSFID] = &Org{ID: strings.ToLower(targetSFID), Name: "Legacy Ltd"}

	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	require.True(t, plan.Rewrite[0].Replayed)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	assert.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.ErrorContains(t, plan.Rewrite[0].Err, "serves target")
	assertNoCutover(t, fx)
	st, err = LoadState(statePath)
	require.NoError(t, err)
	assert.Equal(t, StepWait, st.Last[lfID].Step)
	assert.Equal(t, statusFailed, st.Last[lfID].Status)
	assert.Equal(t, targetSFID, st.Last[lfID].NewID)
}

func TestApexResolution(t *testing.T) {
	var requests []ApexRequest
	results := map[bool]ApexResult{true: {ID: targetSFID, Action: ActionCreated}, false: {ID: targetSFID, Action: ActionCreated}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services/apexrest/lfx/account", r.URL.Path)
		assert.Equal(t, "Bearer apex-token", r.Header.Get("Authorization"))
		var req ApexRequest
		require.NoError(t, jsonDecode(r, &req))
		requests = append(requests, req)
		res := results[req.DryRun]
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":%q,"action":%q}`, res.ID, res.Action)
	}))
	defer srv.Close()
	apex, err := NewApexClient(srv.URL, "apex-token")
	require.NoError(t, err)
	_, err = NewApexClient("", "x")
	assert.ErrorIs(t, err, ErrApexUnavailable)

	fx, _, statePath := rewriteFixture(t)
	deps := fx.deps()
	deps.Apex = apex
	opts := Options{Apply: true, UseApex: true, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	assert.Equal(t, targetSFID, plan.Rewrite[0].NewID)
	assert.Equal(t, ActionCreated, plan.Rewrite[0].Action)
	require.Len(t, requests, 1)
	assert.True(t, requests[0].DryRun)
	assert.Equal(t, ApexRequest{Name: "Legacy Ltd", Website: "https://legacy.example", Source: "EasyCLA", ExternalKey: lfID, CCLASignedDate: "2024-01-02T00:00:00Z", DryRun: true}, requests[0])

	sum, err := Execute(context.Background(), deps, opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	require.Len(t, requests, 2)
	assert.False(t, requests[1].DryRun)

	// matched needs an approved mapping row with the same id
	results[true] = ApexResult{ID: targetSFID, Action: ActionMatched}
	fx, mapping, statePath := rewriteFixture(t)
	deps = fx.deps()
	deps.Apex = apex
	plan, err = BuildPlan(context.Background(), deps, Options{UseApex: true})
	require.NoError(t, err)
	assert.Equal(t, "apex_match_needs_approval", groupByKey(plan.Groups, lfID).ManualReason)
	plan, err = BuildPlan(context.Background(), deps, Options{UseApex: true, Mapping: mapping})
	require.NoError(t, err)
	assert.Equal(t, targetSFID, groupByKey(plan.Groups, lfID).NewID)

	// the real call must return what the dry run returned
	results[false] = ApexResult{ID: targetSFID2, Action: ActionMatched}
	fx.platform.orgs[targetSFID2] = &Org{ID: targetSFID2}
	opts = Options{Apply: true, UseApex: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}}
	calls := len(requests)
	plan, err = BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)
	assert.False(t, plan.Rewrite[0].ViaApex, "an approved mapping row is the operator's resolution: Apex is not consulted")
	sum, err = Execute(context.Background(), deps, opts, plan)
	require.NoError(t, err, "a mapping-resolved group never depends on the live Apex answer")
	assert.Equal(t, 1, sum.Rewritten)
	assert.Equal(t, targetSFID, fx.companies.rows["c-parent"].CompanyExternalID)
	assert.Len(t, requests, calls, "no Apex call was made for the mapping-resolved group")

	// an Apex-resolved group must get the same answer from the real call as from the dry run
	results[true] = ApexResult{ID: targetSFID, Action: ActionCreated}
	fx, _, statePath = rewriteFixture(t)
	deps = fx.deps()
	deps.Apex = apex
	opts = Options{Apply: true, UseApex: true, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err = BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	assert.True(t, plan.Rewrite[0].ViaApex)
	sum, err = Execute(context.Background(), deps, opts, plan)
	assert.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.ErrorContains(t, plan.Rewrite[0].Err, "changed between dry run")
	assert.Empty(t, fx.companies.updates)
	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.Equal(t, StepResolve, st.Last[lfID].Step)
	assert.Equal(t, "failed", st.Last[lfID].Status)
	// a failed resolution is never replayed as approved: the next run classifies from scratch again
	results[false] = ApexResult{ID: targetSFID, Action: ActionCreated}
	plan, err = BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	assert.False(t, plan.Rewrite[0].Replayed)
	assert.Equal(t, targetSFID, plan.Rewrite[0].NewID)

	results[true] = ApexResult{ID: "", Action: ActionAmbiguous}
	plan, err = BuildPlan(context.Background(), deps, Options{UseApex: true})
	require.NoError(t, err)
	assert.Equal(t, "mapping_ambiguous", groupByKey(plan.Groups, lfID).ManualReason)

	results[true] = ApexResult{ID: "nope", Action: ActionCreated}
	plan, err = BuildPlan(context.Background(), deps, Options{UseApex: true})
	require.NoError(t, err)
	assert.Equal(t, "apex_error", groupByKey(plan.Groups, lfID).ManualReason)
	assert.Error(t, groupByKey(plan.Groups, lfID).Err)

	// a dry-run "created" has no Account id yet: the group is a planned rewrite that gets its id from the real call
	results[true] = ApexResult{ID: "", Action: ActionCreated}
	results[false] = ApexResult{ID: targetSFID, Action: ActionCreated}
	fx, _, statePath = rewriteFixture(t)
	deps = fx.deps()
	deps.Apex = apex
	opts = Options{Apply: true, UseApex: true, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err = BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)
	require.Len(t, plan.Rewrite, 1)
	g := plan.Rewrite[0]
	assert.True(t, g.ApexCreates())
	assert.Empty(t, g.NewID)
	assert.Empty(t, g.ManualReason)
	assert.Equal(t, 0, plan.Pending())
	var buf bytes.Buffer
	plan.Print(&buf)
	assert.Contains(t, buf.String(), "id assigned by Apex at apply")
	sum, err = Execute(context.Background(), deps, opts, plan)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Rewritten)
	assert.Equal(t, targetSFID, g.NewID)
	assert.Equal(t, targetSFID, fx.companies.rows["c-parent"].CompanyExternalID)
	st, err = LoadState(statePath)
	require.NoError(t, err)
	assert.Equal(t, targetSFID, st.Last[lfID].NewID)

	// the real call must return a usable id
	results[false] = ApexResult{ID: "", Action: ActionCreated}
	fx, _, statePath = rewriteFixture(t)
	deps = fx.deps()
	deps.Apex = apex
	opts = Options{Apply: true, UseApex: true, State: statePath, Routes: []Route{RouteRewrite}}
	plan, err = BuildPlan(context.Background(), deps, opts)
	require.NoError(t, err)
	sum, err = Execute(context.Background(), deps, opts, plan)
	assert.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.ErrorContains(t, plan.Rewrite[0].Err, "non-account id")
	assert.Empty(t, fx.companies.updates)
	assert.Empty(t, fx.events.rekeyed)
	assert.Len(t, mustListOrgGrants(t, fx, lfID), 3)
}

func TestAudit(t *testing.T) {
	fx := newFixture()
	fx.company("c-live", "Live Corp", "", liveSFID, "cg-1")
	fx.company("c-live-sub", "Live Corp", "Live Corp Asia", liveSFID)
	fx.company("c-dead", "Dead Corp", "", deadSFID, "cg-1")
	fx.company("c-lf", "Legacy Ltd", "", lfID, "cg-1")
	fx.company("c-empty", "Empty Inc", "", "", "cg-1")
	fx.company("c-inactive", "Idle", "", liveSFID2)
	fx.company("c-samename-a", "Twin", "", "0014100000TwinAAAA", "cg-1")
	fx.company("c-samename-b", "Twin", "", "0014100000TwinBBBB")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.orgs[liveSFID] = &Org{ID: liveSFID, Name: "Live Corp", Website: "https://live.example"}
	fx.platform.orgs[liveSFID2] = &Org{ID: liveSFID2, Name: "Idle"}
	fx.platform.orgs["0014100000TwinAAAA"] = &Org{ID: "0014100000TwinAAAA"}
	fx.platform.orgs["0014100000TwinBBBB"] = &Org{ID: "0014100000TwinBBBB"}
	fx.platform.sfAccounts["0014100000TwinAAAA"] = true
	counts := map[string]int{"c-dead": 4, "c-empty": 2}
	deps := fx.deps()
	deps.ECLAs = eclaCounterFunc(func(_ context.Context, id string) (int, error) { return counts[id], nil })
	dir := t.TempDir()
	res, err := Audit(context.Background(), deps, Options{Stage: "dev", OutDir: dir})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{TierMissing: 1, TierInvalid: 1, TierOK: 5, TierDangling: 1}, res.Tiers)
	assert.Equal(t, map[Route]int{RouteRegister: 3, RouteRewrite: 2, RouteManual: 1}, res.Routes)
	assert.Len(t, res.Duplicates, 1, "same name under different SFIDs; distinct signing entities are not duplicates")
	assert.Empty(t, fx.companies.updates)
	assert.Empty(t, fx.platform.registered)

	rows := readCSV(t, filepath.Join(dir, "audit.csv"))
	require.Len(t, rows, 9)
	byID := map[string][]string{}
	for _, r := range rows[1:] {
		byID[r[0]] = r
	}
	assert.Equal(t, []string{"c-live", "Live Corp", "", liveSFID, "001", "true", "1", "0", "200", "https://live.example", "false", "register", "", TierOK}, byID["c-live"])
	assert.Equal(t, "rewrite", byID["c-dead"][11])
	assert.Equal(t, "4", byID["c-dead"][7], "ECLA counts only for manual/duplicate rows")
	assert.Equal(t, TierDangling, byID["c-dead"][13])
	assert.Equal(t, "manual", byID["c-empty"][11])
	assert.Equal(t, "2", byID["c-empty"][7])
	assert.Equal(t, "", byID["c-inactive"][11], "rows without an active CCLA have no route")
	assert.Equal(t, TierInvalid, byID["c-lf"][13])

	unresolvable := readCSV(t, filepath.Join(dir, "unresolvable.csv"))
	ids := []string{}
	for _, r := range unresolvable[1:] {
		ids = append(ids, r[0])
	}
	assert.ElementsMatch(t, []string{"c-dead", "c-empty"}, ids)
	dups := readCSV(t, filepath.Join(dir, "possible_duplicates.csv"))
	assert.Len(t, dups, 3)
	assert.Contains(t, fx.out.String(), "SFID_OK=5")

	// run.log (deps.Out) carries one line per company row, so the CloudWatch copy is the full decision record
	var auditLines []string
	for _, line := range strings.Split(fx.out.String(), "\n") {
		if strings.HasPrefix(line, "audit row ") {
			auditLines = append(auditLines, line)
		}
	}
	assert.Len(t, auditLines, 8)
	assert.Contains(t, fx.out.String(), "audit row company_id=c-dead name=\"Dead Corp\" external_id=\""+deadSFID+"\" shape=001 active_ccla=true ccla=1 ecla=4 org_service=404 route=rewrite reason=\"no_mapping\" tier="+TierDangling)
	assert.Contains(t, fx.out.String(), "audit row company_id=c-empty ")
	assert.Contains(t, fx.out.String(), "audit row company_id=c-inactive ")
}

type eclaCounterFunc func(context.Context, string) (int, error)

func (f eclaCounterFunc) CountECLAs(ctx context.Context, id string) (int, error) { return f(ctx, id) }

// dupSets reads possible_duplicates.csv into ordered sets of company ids plus the row cells by company id.
func dupSets(t *testing.T, dir string) ([][]string, map[string][]string) {
	t.Helper()
	rows := readCSV(t, filepath.Join(dir, "possible_duplicates.csv"))
	require.Equal(t, []string{"group", "company_id", "company_name", "signing_entity_name", "company_external_id", "id_shape", "domain", "active_ccla", "ccla_count", "ecla_count"}, rows[0])
	var sets [][]string
	cells := map[string][]string{}
	last := ""
	for _, r := range rows[1:] {
		if r[0] != last {
			sets, last = append(sets, nil), r[0]
		}
		sets[len(sets)-1] = append(sets[len(sets)-1], r[1])
		cells[r[1]] = r
	}
	for _, set := range sets {
		sort.Strings(set)
	}
	return sets, cells
}

func TestAuditDuplicateCandidates(t *testing.T) {
	const (
		ibmA     = "0014100000IbmAAAAA"
		ibmB     = "0014100000IbmBBBBB"
		huaweiSF = "0014100000HuaweiAA"
		huaweiLF = "lfHuaweiLegacy0001"
		ghA      = "0014100000GithubAA"
		ghB      = "0014100000GithubBB"
		twinA    = "0014100000TwinAAAA"
		twinB    = "0014100000TwinBBBB"
		soloSF   = "0014100000SoloAAAA"
	)
	setup := func() *fixture {
		fx := newFixture()
		fx.company("c-ibm-a", "IBM", "", ibmA, "cg-1")
		fx.company("c-ibm-b", "IBM Corporation", "", ibmB) // inactive, still a candidate
		fx.company("c-huawei-sf", "Huawei Technologies", "", huaweiSF, "cg-1")
		fx.company("c-huawei-lf", "Huawei", "", huaweiLF, "cg-1")
		fx.company("c-gh-a", "Alpha", "", ghA, "cg-1")
		fx.company("c-gh-b", "Beta", "", ghB, "cg-1")
		fx.company("c-twin-a", "Twin", "", twinA, "cg-1")
		fx.company("c-twin-b", "Twin", "", twinB, "cg-1")
		fx.company("c-solo", "Solo", "", soloSF, "cg-1")
		fx.company("c-solo-sub", "Solo", "Solo Europe", soloSF, "cg-1") // same id: signing entities, not duplicates
		fx.company("c-nosite-a", "Nosite", "", "", "cg-1")
		fx.platform.orgs[ibmA] = &Org{ID: ibmA, Name: "IBM", Website: "https://www.ibm.com/us"}
		fx.platform.orgs[ibmB] = &Org{ID: ibmB, Name: "IBM Corporation", Website: "ibm.com"}
		fx.platform.orgs[huaweiSF] = &Org{ID: huaweiSF, Name: "Huawei Technologies", Website: "https://huawei.com"}
		fx.platform.orgs[huaweiLF] = &Org{ID: huaweiLF, Name: "Huawei", Website: "http://www.huawei.com/"}
		fx.platform.orgs[ghA] = &Org{ID: ghA, Name: "Alpha", Website: "https://github.com/alpha"}
		fx.platform.orgs[ghB] = &Org{ID: ghB, Name: "Beta", Website: "github.com"}
		fx.platform.orgs[twinA] = &Org{ID: twinA, Name: "Twin", Website: "https://twin.example"}
		fx.platform.orgs[twinB] = &Org{ID: twinB, Name: "Twin", Website: "https://twin.example"}
		fx.platform.orgs[soloSF] = &Org{ID: soloSF, Name: "Solo", Website: "https://solo.example"}
		for _, id := range []string{ibmA, ibmB, huaweiSF, ghA, ghB, twinA, twinB, soloSF} {
			fx.platform.sfAccounts[id] = true
		}
		return fx
	}

	t.Run("domain and name candidates with default shared domains", func(t *testing.T) {
		fx := setup()
		counts := map[string]int{"c-ibm-a": 12, "c-ibm-b": 3, "c-huawei-lf": 290, "c-twin-a": 1}
		deps := fx.deps()
		deps.ECLAs = eclaCounterFunc(func(_ context.Context, id string) (int, error) {
			if id == "c-huawei-sf" {
				return 0, errors.New("dynamodb unavailable")
			}
			return counts[id], nil
		})
		dir := t.TempDir()
		res, err := Audit(context.Background(), deps, Options{Stage: "dev", OutDir: dir})
		require.NoError(t, err)

		sets, cells := dupSets(t, dir)
		assert.Len(t, res.Duplicates, 3)
		assert.ElementsMatch(t, [][]string{{"c-twin-a", "c-twin-b"}, {"c-huawei-lf", "c-huawei-sf"}, {"c-ibm-a", "c-ibm-b"}}, sets,
			"same name and same domain reach one set once; github.com never groups; one id with signing entities is not a duplicate; no website means no domain group")
		assert.Equal(t, []string{"c-ibm-b", "IBM Corporation", "", ibmB, "001", "ibm.com", "false", "0", "3"}, cells["c-ibm-b"][1:], "inactive rows in the report get an ECLA count too")
		assert.Equal(t, []string{"lf", "huawei.com"}, cells["c-huawei-lf"][5:7])
		assert.Equal(t, "290", cells["c-huawei-lf"][9])
		assert.Equal(t, "-1", cells["c-huawei-sf"][9], "a failed count is visible")
		assert.Equal(t, "12", cells["c-ibm-a"][9])
		assert.Equal(t, "0", cells["c-twin-b"][9], "a measured zero")
		assert.Equal(t, "1", cells["c-twin-a"][9])
		for _, id := range []string{"c-gh-a", "c-gh-b", "c-solo", "c-solo-sub", "c-nosite-a"} {
			assert.NotContains(t, cells, id)
		}
		assert.Contains(t, fx.out.String(), "duplicates=3")
	})

	t.Run("a configured shared-domain list replaces the default one", func(t *testing.T) {
		fx := setup()
		dir := t.TempDir()
		sharedPath := filepath.Join(dir, "shared.txt")
		require.NoError(t, os.WriteFile(sharedPath, []byte("# review list\nibm.com\n"), 0o600))
		res, err := Audit(context.Background(), fx.deps(), Options{Stage: "dev", OutDir: dir, SharedDomains: sharedPath})
		require.NoError(t, err)
		sets, cells := dupSets(t, dir)
		assert.Len(t, res.Duplicates, 3)
		assert.ElementsMatch(t, [][]string{{"c-twin-a", "c-twin-b"}, {"c-huawei-lf", "c-huawei-sf"}, {"c-gh-a", "c-gh-b"}}, sets,
			"github.com groups once it is not shared; ibm.com no longer does")
		assert.Equal(t, "", cells["c-gh-a"][9], "no ECLA counter configured: the count is not measured")

		_, err = Audit(context.Background(), fx.deps(), Options{Stage: "dev", OutDir: dir, SharedDomains: filepath.Join(dir, "missing.txt")})
		assert.Error(t, err)
	})

	t.Run("overlapping name and domain sets are both reported", func(t *testing.T) {
		fx := setup()
		fx.company("c-twin-c", "Twin", "", "0014100000TwinCCCC", "cg-1")
		fx.platform.orgs["0014100000TwinCCCC"] = &Org{ID: "0014100000TwinCCCC", Name: "Twin", Website: "https://other.example"}
		dir := t.TempDir()
		res, err := Audit(context.Background(), fx.deps(), Options{Stage: "dev", OutDir: dir})
		require.NoError(t, err)
		sets, _ := dupSets(t, dir)
		assert.Len(t, res.Duplicates, 4)
		assert.Contains(t, sets, []string{"c-twin-a", "c-twin-b", "c-twin-c"}, "by name")
		assert.Contains(t, sets, []string{"c-twin-a", "c-twin-b"}, "by domain")
	})
}

func jsonDecode(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
