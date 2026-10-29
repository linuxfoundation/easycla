// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_admins

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/go-openapi/strfmt"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

const (
	sfid18 = "0014100000Te0G7AAJ"
	sfid15 = "0014100000Te0G7"
)

var baseTime = time.Date(2026, 10, 28, 7, 0, 0, 0, time.UTC)

type fakeCompanies struct {
	rows        map[string][]*models.Company
	accounts    map[string]string
	accountsErr error
	log         *[]string
}

func (f fakeCompanies) GetCompaniesByExternalID(_ context.Context, sfid string, _ bool) ([]*models.Company, error) {
	rows, ok := f.rows[sfid]
	if !ok {
		return nil, &utils.CompanyNotFound{CompanySFID: sfid}
	}
	return rows, nil
}

func (f fakeCompanies) GetCompanyAccounts(_ context.Context, ids []string) (map[string]string, error) {
	if f.log != nil {
		*f.log = append(*f.log, "companies "+strings.Join(ids, ","))
	}
	if f.accountsErr != nil {
		return nil, f.accountsErr
	}
	out := map[string]string{}
	for _, id := range ids {
		if v, ok := f.accounts[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

// fakeCCLAs: index = company -> ids (eventually consistent, may be stale), base = id -> row as the table holds
// it now; once more than switchAfter base reads happened, laterBase replaces base; every call is logged
type fakeCCLAs struct {
	index       map[string][]string
	base        map[string]CCLA
	laterBase   map[string]CCLA
	switchAfter int
	reads       int
	log         *[]string
}

func (f *fakeCCLAs) ListCompanyCCLAIDs(_ context.Context, companyID string) ([]string, error) {
	if f.log != nil {
		*f.log = append(*f.log, "list "+companyID)
	}
	return f.index[companyID], nil
}

func (f *fakeCCLAs) GetCCLAs(_ context.Context, ids []string) ([]CCLA, error) {
	f.reads++
	if f.log != nil {
		*f.log = append(*f.log, "read "+strings.Join(ids, ","))
	}
	base := f.base
	if f.laterBase != nil && f.reads > f.switchAfter {
		base = f.laterBase
	}
	var out []CCLA
	for _, id := range ids {
		if row, ok := base[id]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}

type fakeUsers struct {
	byName   map[string]*models.User
	byEntry  map[string]*models.User
	entryErr map[string]error
}

func (f fakeUsers) GetUserByLFUserName(name string) (*models.User, error) {
	u, ok := f.byName[name]
	if !ok {
		return nil, fmt.Errorf("user %s not found", name)
	}
	return u, nil
}

// GetUserByUserName mirrors the users repository: a missing record is (nil, nil), a failure is an error
func (f fakeUsers) GetUserByUserName(name string, _ bool) (*models.User, error) {
	if err, failed := f.entryErr[name]; failed {
		return nil, err
	}
	return f.byEntry[name], nil
}

type fakePublisher struct {
	errs  []error
	calls []publishCall
	log   *[]string
}

type publishCall struct {
	uid    string
	admins []member_service.CLAAdmin
	at     time.Time
}

func (f *fakePublisher) PutB2BOrgCLAAdmins(_ context.Context, uid string, admins []member_service.CLAAdmin, at time.Time) error {
	if f.log != nil {
		*f.log = append(*f.log, "put")
	}
	f.calls = append(f.calls, publishCall{uid: uid, admins: admins, at: at})
	if len(f.errs) == 0 {
		return nil
	}
	err := f.errs[0]
	f.errs = f.errs[1:]
	return err
}

// clock hands out times one second apart and logs every call
type clock struct {
	next time.Time
	log  *[]string
}

func (c *clock) Now() time.Time {
	if c.log != nil {
		*c.log = append(*c.log, "now")
	}
	t := c.next
	c.next = t.Add(time.Second)
	return t
}

// fakePending is the durable catalog; one instance can be shared by several publisher instances
type fakePending struct {
	sets    map[string]map[string]bool
	addErr  error
	listErr error
	log     *[]string
}

func newFakePending() *fakePending { return &fakePending{sets: map[string]map[string]bool{}} }

func (f *fakePending) Add(_ context.Context, sfid string, ids ...string) error {
	if f.log != nil {
		*f.log = append(*f.log, "register "+strings.Join(ids, ","))
	}
	if f.addErr != nil {
		return f.addErr
	}
	if f.sets[sfid] == nil {
		f.sets[sfid] = map[string]bool{}
	}
	for _, id := range ids {
		f.sets[sfid][id] = true
	}
	return nil
}

func (f *fakePending) List(_ context.Context, sfid string) ([]string, error) {
	if f.log != nil {
		*f.log = append(*f.log, "catalog")
	}
	if f.listErr != nil {
		return nil, f.listErr
	}
	return sortedKeys(f.sets[sfid]), nil
}

type harness struct {
	svc     Service
	cclas   *fakeCCLAs
	pub     *fakePublisher
	pending *fakePending
	log     []string
}

func baseRows() map[string]CCLA {
	return map[string]CCLA{
		"s1":  {SignatureID: "s1", CompanyID: "c-entity-1", Signed: true, ACL: []string{"zoe", " amy "}},
		"s2":  {SignatureID: "s2", CompanyID: "c-entity-1", Signed: false, ACL: []string{"unsigned-manager"}},
		"s3":  {SignatureID: "s3", CompanyID: "c-entity-2", Signed: true, ACL: []string{"amy", "bob", ""}},
		"s7":  {SignatureID: "s7", CompanyID: "c-other", Signed: true, ACL: []string{"mallory"}},
		"s9":  {SignatureID: "s9", CompanyID: "c-entity-2", Signed: true, ACL: []string{"zed"}},
		"s11": {SignatureID: "s11", CompanyID: "c-entity-3", Signed: true, ACL: []string{"neo"}},
	}
}

func newHarness(pub *fakePublisher) *harness {
	return newHarnessWith(pub, nil, baseTime)
}

// newHarnessWith builds a publisher around a shared catalog (nil = its own) and its own clock start
func newHarnessWith(pub *fakePublisher, shared *fakePending, start time.Time) *harness {
	h := &harness{pub: pub, pending: shared}
	if h.pending == nil {
		h.pending = newFakePending()
		h.pending.log = &h.log
	}
	h.cclas = &fakeCCLAs{index: map[string][]string{"c-entity-1": {"s1", "s2"}, "c-entity-2": {"s3"}}, base: baseRows(), log: &h.log}
	pub.log = &h.log
	companies := fakeCompanies{rows: map[string][]*models.Company{
		sfid18: {{CompanyID: "c-entity-2", CompanyExternalID: sfid18}},
		sfid15: {{CompanyID: "c-entity-1", CompanyExternalID: sfid15}, {CompanyID: "c-entity-2", CompanyExternalID: sfid18}},
	}, accounts: map[string]string{
		"c-entity-1": sfid15,
		"c-entity-2": sfid18,
		"c-entity-3": sfid15,
		"c-other":    "0014100000Te0G8AAJ",
	}, log: &h.log}
	h.svc = Service{
		Companies:   companies,
		CompanyRows: companies,
		CCLAs:       h.cclas,
		Pending:     h.pending,
		Users: fakeUsers{byName: map[string]*models.User{
			"amy": {LfUsername: "amy", LfEmail: strfmt.Email("amy@example.com"), Username: "Amy Adams"},
			"bob": {LfUsername: "bob"},
		}},
		Members: pub,
		Now:     (&clock{next: start, log: &h.log}).Now,
	}
	return h
}

func newService(pub *fakePublisher) Service {
	return newHarness(pub).svc
}

func usernamesOf(admins []member_service.CLAAdmin) []string {
	r := Roster{Admins: admins}
	return r.Usernames()
}

func TestRosterUnionsEveryRowSharingTheAccount(t *testing.T) {
	h := newHarness(&fakePublisher{})
	roster, err := h.svc.Roster(context.Background(), sfid15)
	require.NoError(t, err)
	assert.Equal(t, sfid18, roster.SFID)
	assert.Equal(t, []string{"c-entity-1", "c-entity-2"}, roster.CompanyIDs)
	assert.Equal(t, 2, roster.SignedCCLAs)
	assert.Equal(t, []member_service.CLAAdmin{
		{Username: "amy", Email: "amy@example.com", Name: "Amy Adams"},
		{Username: "bob"},
		{Username: "zoe"},
	}, roster.Admins, "unsigned CCLA managers are left out, duplicates collapse, unknown users keep the username only")
	assert.Equal(t, []string{"amy", "bob", "zoe"}, roster.Usernames())
	assert.Equal(t, []string{"catalog", "list c-entity-1", "list c-entity-2", "read s1,s2,s3", "companies c-entity-1,c-entity-2"}, h.log, "ids come from the catalog and the index, state and company bindings from base reads")
}

// interim input filtering before the member-service call (PR 5277 review): a username with whitespace or
// control characters is skipped, an email that is not a plain address is omitted while the manager stays,
// case and punctuation are sent untouched; the exact member-service rules are a pre-activation item
func TestRosterSkipsUnpublishableIdentities(t *testing.T) {
	h := newHarness(&fakePublisher{})
	row := h.cclas.base["s1"]
	row.ACL = []string{"zoe", " amy ", "bad user", "tab\tname", "ctl\x01x", "Zoe.Q-1", "   ", "github:42", "github:99", "github:7", "github:abc", "github:", "github:12x", "github:0", "github:-5", "github:99999999999999999999"}
	h.cclas.base["s1"] = row
	h.svc.Users = fakeUsers{byName: map[string]*models.User{
		"amy":     {LfUsername: "amy", LfEmail: strfmt.Email(" amy@example.com "), Username: "Amy Adams"},
		"bob":     {LfUsername: "bob", LfEmail: strfmt.Email("bob@example.com (home)"), Username: "Bob"},
		"Zoe.Q-1": {LfUsername: "Zoe.Q-1", LfEmail: strfmt.Email("Zoe Q <zoe@example.com>"), Username: "Zoe Q"},
		"zoe":     {LfUsername: "zoe", LfEmail: strfmt.Email("not an address")},
		"gina":    {LfUsername: "gina", LfEmail: strfmt.Email("gina@example.com"), Username: "Gina"},
	}, byEntry: map[string]*models.User{
		"github:42": {LfUsername: "gina"},
		"github:7":  {LfUsername: "amy"},
	}, entryErr: map[string]error{
		"github:abc":                  errors.New("malformed entries must not be looked up"),
		"github:":                     errors.New("malformed entries must not be looked up"),
		"github:12x":                  errors.New("malformed entries must not be looked up"),
		"github:0":                    errors.New("malformed entries must not be looked up"),
		"github:-5":                   errors.New("malformed entries must not be looked up"),
		"github:99999999999999999999": errors.New("malformed entries must not be looked up"),
	}}
	roster, err := h.svc.Roster(context.Background(), sfid15)
	require.NoError(t, err)
	assert.Equal(t, []member_service.CLAAdmin{
		{Username: "Zoe.Q-1", Name: "Zoe Q"},
		{Username: "amy", Email: "amy@example.com", Name: "Amy Adams"},
		{Username: "bob", Name: "Bob"},
		{Username: "gina", Email: "gina@example.com", Name: "Gina"},
		{Username: "zoe"},
	}, roster.Admins, "github:42 resolves to gina, github:7 is amy again, github:99 has no record, malformed github entries are skipped without a lookup")
	res, err := h.svc.Publish(context.Background(), sfid15)
	require.NoError(t, err)
	assert.Equal(t, []string{"Zoe.Q-1", "amy", "bob", "gina", "zoe"}, res.Roster.Usernames())
	require.Len(t, h.pub.calls, 1)
	assert.Equal(t, roster.Admins, h.pub.calls[0].admins, "the filtered roster is what member-service receives")
}

// no CLA-manager username reaches the logs: skipped names are counted, an omitted email and a missing
// user record are reported without the name
func TestRosterLogsNoManagerUsernames(t *testing.T) {
	logger := log.GetLogger()
	oldHooks := logger.ReplaceHooks(make(logrus.LevelHooks))
	oldLevel := logger.GetLevel()
	logger.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() {
		logger.ReplaceHooks(oldHooks)
		logger.SetLevel(oldLevel)
	})
	hook := logtest.NewLocal(logger)

	h := newHarness(&fakePublisher{})
	row := h.cclas.base["s1"]
	row.ACL = []string{"bad user", "tab\tname", "ctl\x01x", "zoe", "Zoe.Q-1", "ghost"}
	h.cclas.base["s1"] = row
	h.svc.Users = fakeUsers{byName: map[string]*models.User{
		"amy":     {LfUsername: "amy", LfEmail: strfmt.Email("amy@example.com"), Username: "Amy"},
		"bob":     {LfUsername: "bob", LfEmail: strfmt.Email("bob@example.com"), Username: "Bob"},
		"zoe":     {LfUsername: "zoe", LfEmail: strfmt.Email("not an address")},
		"Zoe.Q-1": {LfUsername: "Zoe.Q-1", LfEmail: strfmt.Email("Zoe Q <zoe@example.com>"), Username: "Zoe Q"},
	}}
	roster, err := h.svc.Roster(context.Background(), sfid15)
	require.NoError(t, err)
	assert.Equal(t, []string{"Zoe.Q-1", "amy", "bob", "ghost", "zoe"}, roster.Usernames(), "the other rows of the account add amy and bob")
	names := []string{"bad user", "tab\tname", "ctl\x01x", "zoe", "Zoe.Q-1", "ghost", "amy", "bob"}

	var skipped, omitted, missing int
	for _, e := range hook.AllEntries() {
		text := e.Message
		for k, v := range e.Data {
			text += " " + k + "=" + fmt.Sprint(v)
		}
		for _, name := range names {
			assert.NotContains(t, text, name, "log entry %q", e.Message)
		}
		switch e.Message {
		case "CLA manager usernames skipped - not publishable to member-service":
			skipped++
			assert.Equal(t, 3, e.Data["skipped"])
			assert.Equal(t, logrus.WarnLevel, e.Level)
		case "CLA manager email omitted - the username is sent alone":
			omitted++
			assert.Equal(t, logrus.WarnLevel, e.Level)
		case "no user record for a CLA manager - sending the username only":
			missing++
			assert.Equal(t, true, e.Data["lookupFailed"])
		}
	}
	assert.Equal(t, 1, skipped, "one counted Warn for the three unpublishable names")
	assert.Equal(t, 2, omitted, "zoe and Zoe.Q-1 keep their username, lose the email")
	assert.Equal(t, 1, missing, "ghost has no user record")
}

// a legacy entry whose lookup FAILS aborts the roster before anything reaches member-service; the entry and
// the lookup error stay out of the returned error
func TestRosterAbortsWhenALegacyEntryLookupFails(t *testing.T) {
	h := newHarness(&fakePublisher{})
	row := h.cclas.base["s1"]
	row.ACL = []string{"zoe", "github:500"}
	h.cclas.base["s1"] = row
	h.svc.Users = fakeUsers{byName: map[string]*models.User{}, entryErr: map[string]error{"github:500": errors.New("dynamo down for github:500")}}
	_, err := h.svc.Roster(context.Background(), sfid15)
	require.ErrorIs(t, err, errLegacyManagerLookup)
	assert.NotContains(t, err.Error(), "github:500")
	assert.NotContains(t, err.Error(), "dynamo down")
	_, err = h.svc.Publish(context.Background(), sfid15)
	require.Error(t, err)
	assert.Empty(t, h.pub.calls, "nothing is sent to member-service")
}

func TestRosterEdgeCases(t *testing.T) {
	svc := newService(&fakePublisher{})
	_, err := svc.Roster(context.Background(), "lfSclQhURLCCrv7EZj")
	assert.ErrorIs(t, err, member_service.ErrInvalidSFID, "legacy org-service ids are not Salesforce accounts")
	_, err = svc.Roster(context.Background(), "a092M00001IfTjEQAV")
	assert.ErrorIs(t, err, member_service.ErrInvalidSFID, "project ids are not accounts")

	roster, err := svc.Roster(context.Background(), "001Ab00000CdEfGIAV")
	require.NoError(t, err)
	assert.Empty(t, roster.CompanyIDs)
	assert.NotNil(t, roster.Admins)
	assert.Empty(t, roster.Admins, "an organization without rows clears the list instead of failing")
}

// the index only says which rows exist; what they contain is read from the base table, so neither a stale
// index entry nor the image an old event carried decides
func TestRosterReadsTheBaseTable(t *testing.T) {
	t.Run("a stale index entry does not resurrect an acl", func(t *testing.T) {
		h := newHarness(&fakePublisher{})
		h.cclas.base["s3"] = CCLA{SignatureID: "s3", CompanyID: "c-entity-2", Signed: true, ACL: []string{"amy"}}
		roster, err := h.svc.Roster(context.Background(), sfid18)
		require.NoError(t, err)
		assert.Equal(t, []string{"amy", "zoe"}, roster.Usernames(), "bob's removal is honoured whatever the index projected")
	})
	t.Run("a row the index does not list yet counts when the caller names it", func(t *testing.T) {
		h := newHarness(&fakePublisher{})
		roster, err := h.svc.Roster(context.Background(), sfid18, "s9")
		require.NoError(t, err)
		assert.Equal(t, []string{"amy", "bob", "zed", "zoe"}, roster.Usernames())
		assert.Equal(t, 3, roster.SignedCCLAs)
		assert.Contains(t, h.log, "read s1,s2,s3,s9")
	})
	t.Run("a deleted row named by an old event is left out", func(t *testing.T) {
		h := newHarness(&fakePublisher{})
		roster, err := h.svc.Roster(context.Background(), sfid18, "s-gone", " ")
		require.NoError(t, err)
		assert.Equal(t, []string{"amy", "bob", "zoe"}, roster.Usernames())
	})
	t.Run("a row of another organization is left out", func(t *testing.T) {
		h := newHarness(&fakePublisher{})
		roster, err := h.svc.Roster(context.Background(), sfid18, "s7")
		require.NoError(t, err)
		assert.Equal(t, []string{"amy", "bob", "zoe"}, roster.Usernames())
		assert.Contains(t, h.log, "companies c-entity-1,c-entity-2,c-other", "every referenced company is verified from its base row")
		assert.Equal(t, []string{"c-entity-1", "c-entity-2"}, roster.CompanyIDs)
	})
	t.Run("a stale listing of a re-keyed company does not keep its managers", func(t *testing.T) {
		h := newHarness(&fakePublisher{})
		companies, ok := h.svc.Companies.(fakeCompanies)
		require.True(t, ok)
		companies.accounts = map[string]string{"c-entity-1": sfid15, "c-entity-2": "0014100000Te0G8AAJ"}
		h.svc.Companies, h.svc.CompanyRows = companies, companies
		roster, err := h.svc.Roster(context.Background(), sfid18)
		require.NoError(t, err)
		assert.Equal(t, []string{"amy", "zoe"}, roster.Usernames(), "the index still lists c-entity-2 under this account, its base row says otherwise")
		assert.Equal(t, []string{"c-entity-1"}, roster.CompanyIDs)
	})
	t.Run("a row of a company the company index does not list yet counts when its base row carries this account", func(t *testing.T) {
		h := newHarness(&fakePublisher{})
		roster, err := h.svc.Roster(context.Background(), sfid18, "s11")
		require.NoError(t, err)
		assert.Equal(t, []string{"amy", "bob", "neo", "zoe"}, roster.Usernames())
		assert.Equal(t, []string{"c-entity-1", "c-entity-2", "c-entity-3"}, roster.CompanyIDs)
		assert.Equal(t, 3, roster.SignedCCLAs)
	})
	t.Run("a company read failure fails the roster instead of dropping managers", func(t *testing.T) {
		h := newHarness(&fakePublisher{})
		companies, ok := h.svc.Companies.(fakeCompanies)
		require.True(t, ok)
		companies.accountsErr = errors.New("dynamo down")
		h.svc.Companies, h.svc.CompanyRows = companies, companies
		_, err := h.svc.Roster(context.Background(), sfid18, "s11")
		assert.ErrorContains(t, err, "dynamo down")
	})
	t.Run("a row that is unsigned now is left out", func(t *testing.T) {
		h := newHarness(&fakePublisher{})
		h.cclas.base["s3"] = CCLA{SignatureID: "s3", CompanyID: "c-entity-2", Signed: false, ACL: []string{"amy", "bob"}}
		roster, err := h.svc.Roster(context.Background(), sfid18, "s3")
		require.NoError(t, err)
		assert.Equal(t, []string{"amy", "zoe"}, roster.Usernames())
		assert.Equal(t, 1, roster.SignedCCLAs)
	})
	t.Run("every signed row gone clears the list", func(t *testing.T) {
		h := newHarness(&fakePublisher{})
		delete(h.cclas.base, "s1")
		delete(h.cclas.base, "s3")
		roster, err := h.svc.Roster(context.Background(), sfid18, "s3")
		require.NoError(t, err)
		assert.Equal(t, []member_service.CLAAdmin{}, roster.Admins)
		assert.Equal(t, 0, roster.SignedCCLAs)
	})
}

func TestPublish(t *testing.T) {
	t.Run("published", func(t *testing.T) {
		pub := &fakePublisher{}
		h := newHarness(pub)
		res, err := h.svc.Publish(context.Background(), sfid15)
		require.NoError(t, err)
		assert.False(t, res.Skipped)
		assert.False(t, res.Retried)
		require.Len(t, pub.calls, 1)
		assert.Equal(t, sfid18, pub.calls[0].uid)
		assert.Equal(t, []string{"amy", "bob", "zoe"}, res.Roster.Usernames())
		assert.Equal(t, baseTime, pub.calls[0].at)
		assert.Equal(t, []string{"now", "catalog", "list c-entity-1", "list c-entity-2", "read s1,s2,s3", "companies c-entity-1,c-entity-2", "put"}, h.log, "the stamp is taken before any read")
	})
	t.Run("no b2b_org yet is skipped", func(t *testing.T) {
		pub := &fakePublisher{errs: []error{member_service.ErrOrgNotFound}}
		res, err := newService(pub).Publish(context.Background(), sfid18)
		require.NoError(t, err)
		assert.True(t, res.Skipped)
		assert.Len(t, pub.calls, 1)
	})
	t.Run("a conflict is retried with the rows as they are now, the triggering row included", func(t *testing.T) {
		pub := &fakePublisher{errs: []error{member_service.ErrConflict, nil}}
		h := newHarness(pub)
		later := baseRows()
		later["s3"] = CCLA{SignatureID: "s3", CompanyID: "c-entity-2", Signed: true, ACL: []string{"amy"}}
		h.cclas.laterBase = later
		h.cclas.switchAfter = 2 // the first attempt reads the registered row, then every row
		res, err := h.svc.Publish(context.Background(), sfid18, "s3")
		require.NoError(t, err)
		assert.True(t, res.Retried)
		require.Len(t, pub.calls, 2)
		assert.Equal(t, []string{"amy", "bob", "zoe"}, usernamesOf(pub.calls[0].admins))
		assert.Equal(t, []string{"amy", "zoe"}, usernamesOf(pub.calls[1].admins), "bob, removed from the triggering row by another writer meanwhile, is not re-added")
		assert.Equal(t, baseTime, pub.calls[0].at)
		assert.Equal(t, baseTime.Add(time.Second), pub.calls[1].at)
		assert.Equal(t, []string{"amy", "zoe"}, res.Roster.Usernames())
		assert.Equal(t, []string{"register s3", "now", "catalog", "read s3", "list c-entity-1", "list c-entity-2", "read s1,s2,s3", "companies c-entity-1,c-entity-2", "put", "now", "catalog", "read s3", "list c-entity-1", "list c-entity-2", "read s1,s2,s3", "companies c-entity-1,c-entity-2", "put"}, h.log, "registration precedes the first clock read; every attempt re-reads the catalog and the base rows")
	})
	t.Run("a conflict after the triggering row was deleted drops it", func(t *testing.T) {
		pub := &fakePublisher{errs: []error{member_service.ErrConflict, nil}}
		h := newHarness(pub)
		later := baseRows()
		delete(later, "s3")
		h.cclas.laterBase = later
		h.cclas.switchAfter = 2
		res, err := h.svc.Publish(context.Background(), sfid18, "s3")
		require.NoError(t, err)
		require.Len(t, pub.calls, 2)
		assert.Equal(t, []string{"amy", "zoe"}, usernamesOf(pub.calls[1].admins), "only the other entity's row remains")
		assert.Equal(t, []string{"amy", "zoe"}, res.Roster.Usernames())
	})
	t.Run("a second conflict is reported", func(t *testing.T) {
		pub := &fakePublisher{errs: []error{member_service.ErrConflict, member_service.ErrConflict}}
		res, err := newService(pub).Publish(context.Background(), sfid18)
		assert.ErrorIs(t, err, member_service.ErrConflict)
		assert.True(t, res.Retried)
		assert.Len(t, pub.calls, 2)
	})
	t.Run("other errors are returned", func(t *testing.T) {
		pub := &fakePublisher{errs: []error{errors.New("boom")}}
		_, err := newService(pub).Publish(context.Background(), sfid18)
		assert.EqualError(t, err, "boom")
	})
	t.Run("the last manager's removal publishes an empty list", func(t *testing.T) {
		pub := &fakePublisher{}
		h := newHarness(pub)
		delete(h.cclas.base, "s1")
		delete(h.cclas.base, "s3")
		res, err := h.svc.Publish(context.Background(), sfid18, "s3")
		require.NoError(t, err)
		require.Len(t, pub.calls, 1)
		assert.Equal(t, []member_service.CLAAdmin{}, pub.calls[0].admins, "an empty, non-nil list replaces the previous managers")
		assert.Equal(t, 0, res.Roster.SignedCCLAs)
	})
}

type fakeDB struct {
	dynamodbiface.DynamoDBAPI
	pages       []*dynamodb.QueryOutput
	inputs      []*dynamodb.QueryInput
	batches     []*dynamodb.BatchGetItemOutput
	batchInputs []*dynamodb.BatchGetItemInput
	onBatch     func()
	updates     []*dynamodb.UpdateItemInput
	updateErr   error
	gets        []*dynamodb.GetItemInput
	getOut      *dynamodb.GetItemOutput
	getErr      error
}

func (f *fakeDB) UpdateItemWithContext(_ aws.Context, in *dynamodb.UpdateItemInput, _ ...request.Option) (*dynamodb.UpdateItemOutput, error) {
	f.updates = append(f.updates, in)
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func (f *fakeDB) GetItemWithContext(_ aws.Context, in *dynamodb.GetItemInput, _ ...request.Option) (*dynamodb.GetItemOutput, error) {
	f.gets = append(f.gets, in)
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getOut, nil
}

func fastBackoff(t *testing.T) {
	previous := batchGetBackoff
	batchGetBackoff = time.Millisecond
	t.Cleanup(func() { batchGetBackoff = previous })
}

func (f *fakeDB) QueryWithContext(_ aws.Context, in *dynamodb.QueryInput, _ ...request.Option) (*dynamodb.QueryOutput, error) {
	f.inputs = append(f.inputs, in)
	page := f.pages[0]
	f.pages = f.pages[1:]
	return page, nil
}

func (f *fakeDB) BatchGetItemWithContext(_ aws.Context, in *dynamodb.BatchGetItemInput, _ ...request.Option) (*dynamodb.BatchGetItemOutput, error) {
	f.batchInputs = append(f.batchInputs, in)
	if f.onBatch != nil {
		f.onBatch()
	}
	page := f.batches[0]
	if len(f.batches) > 1 {
		f.batches = f.batches[1:]
	}
	return page, nil
}

func TestDynamoCCLAsListsIDsThroughTheIndex(t *testing.T) {
	db := &fakeDB{pages: []*dynamodb.QueryOutput{
		{Items: []map[string]*dynamodb.AttributeValue{{"signature_id": {S: aws.String("s1")}}}, LastEvaluatedKey: map[string]*dynamodb.AttributeValue{"signature_id": {S: aws.String("s1")}}},
		{Items: []map[string]*dynamodb.AttributeValue{{"signature_id": {S: aws.String("s2")}}}},
	}}
	ids, err := DynamoCCLAs{DB: db, Table: "cla-dev-signatures"}.ListCompanyCCLAIDs(context.Background(), "c-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"s1", "s2"}, ids)
	require.Len(t, db.inputs, 2)
	assert.Equal(t, "reference-signature-index", aws.StringValue(db.inputs[0].IndexName))
	assert.Equal(t, "cla-dev-signatures", aws.StringValue(db.inputs[0].TableName))
	assert.Equal(t, "c-1", aws.StringValue(db.inputs[0].ExpressionAttributeValues[":id"].S))
	assert.Contains(t, aws.StringValue(db.inputs[0].FilterExpression), "signature_reference_type = :company")
	assert.Equal(t, "signature_id", aws.StringValue(db.inputs[0].ProjectionExpression))
	assert.NotNil(t, db.inputs[1].ExclusiveStartKey)
}

func cclaItem(id, company, sigType, refType string, signed bool, acl ...string) map[string]*dynamodb.AttributeValue {
	members := make([]*dynamodb.AttributeValue, 0, len(acl))
	for _, m := range acl {
		members = append(members, &dynamodb.AttributeValue{S: aws.String(m)})
	}
	return map[string]*dynamodb.AttributeValue{
		"signature_id":             {S: aws.String(id)},
		"signature_reference_id":   {S: aws.String(company)},
		"signature_type":           {S: aws.String(sigType)},
		"signature_reference_type": {S: aws.String(refType)},
		"signature_signed":         {BOOL: aws.Bool(signed)},
		"signature_acl":            {L: members},
	}
}

func TestDynamoCCLAsReadsRowsConsistently(t *testing.T) {
	fastBackoff(t)
	const table = "cla-dev-signatures"
	ids := make([]string, 0, 101)
	for i := 0; i < 101; i++ {
		ids = append(ids, fmt.Sprintf("s%03d", i))
	}
	db := &fakeDB{batches: []*dynamodb.BatchGetItemOutput{
		{Responses: map[string][]map[string]*dynamodb.AttributeValue{table: {cclaItem("s000", "c-1", "ccla", "company", true, "amy")}},
			UnprocessedKeys: map[string]*dynamodb.KeysAndAttributes{table: {Keys: []map[string]*dynamodb.AttributeValue{{"signature_id": {S: aws.String("s001")}}}, ConsistentRead: aws.Bool(true)}}},
		{Responses: map[string][]map[string]*dynamodb.AttributeValue{table: {
			cclaItem("s001", "c-1", "ccla", "company", false),
			cclaItem("s002", "u-1", "cla", "user", true, "ignored"),
		}}},
		{Responses: map[string][]map[string]*dynamodb.AttributeValue{table: {cclaItem("s100", "c-2", "ccla", "company", true, "zed")}}},
	}}
	rows, err := DynamoCCLAs{DB: db, Table: table}.GetCCLAs(context.Background(), ids)
	require.NoError(t, err)
	assert.Equal(t, []CCLA{
		{SignatureID: "s000", CompanyID: "c-1", Signed: true, ACL: []string{"amy"}},
		{SignatureID: "s001", CompanyID: "c-1", Signed: false, ACL: []string{}},
		{SignatureID: "s100", CompanyID: "c-2", Signed: true, ACL: []string{"zed"}},
	}, rows, "missing rows are absent, rows that are not corporate CLAs are skipped")
	require.Len(t, db.batchInputs, 3, "100 keys, the unprocessed key, then the 101st key")
	first := db.batchInputs[0].RequestItems[table]
	assert.Len(t, first.Keys, 100)
	assert.True(t, aws.BoolValue(first.ConsistentRead))
	assert.Contains(t, aws.StringValue(first.ProjectionExpression), "signature_acl")
	assert.Len(t, db.batchInputs[1].RequestItems[table].Keys, 1)
	assert.Len(t, db.batchInputs[2].RequestItems[table].Keys, 1)

	empty, err := DynamoCCLAs{DB: db, Table: table}.GetCCLAs(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, empty)
}

const retryTable = "cla-dev-signatures"

func unprocessed(ids ...string) *dynamodb.BatchGetItemOutput {
	keys := make([]map[string]*dynamodb.AttributeValue, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, map[string]*dynamodb.AttributeValue{"signature_id": {S: aws.String(id)}})
	}
	return &dynamodb.BatchGetItemOutput{
		Responses:       map[string][]map[string]*dynamodb.AttributeValue{retryTable: {cclaItem("s0", "c-1", "ccla", "company", true, "amy")}},
		UnprocessedKeys: map[string]*dynamodb.KeysAndAttributes{retryTable: {Keys: keys, ConsistentRead: aws.Bool(true)}},
	}
}

// unprocessed keys are reissued with a bounded, incremental, cancellable wait - never forever, and never
// turned into a partial roster
func TestDynamoCCLAsBoundsUnprocessedKeyRetries(t *testing.T) {
	const table = retryTable
	t.Run("a later attempt completes the read", func(t *testing.T) {
		fastBackoff(t)
		db := &fakeDB{batches: []*dynamodb.BatchGetItemOutput{
			unprocessed("s1"),
			unprocessed("s1"),
			{Responses: map[string][]map[string]*dynamodb.AttributeValue{table: {cclaItem("s1", "c-1", "ccla", "company", true, "bob")}}},
		}}
		rows, err := DynamoCCLAs{DB: db, Table: table}.GetCCLAs(context.Background(), []string{"s0", "s1"})
		require.NoError(t, err)
		assert.Equal(t, []string{"s0", "s0", "s1"}, func() (ids []string) {
			for _, r := range rows {
				ids = append(ids, r.SignatureID)
			}
			return
		}(), "every page's rows are kept")
		require.Len(t, db.batchInputs, 3)
		assert.Len(t, db.batchInputs[1].RequestItems[table].Keys, 1, "only the unprocessed key is reissued")
	})
	t.Run("keys that stay unprocessed end in an error, not a partial roster", func(t *testing.T) {
		fastBackoff(t)
		db := &fakeDB{batches: []*dynamodb.BatchGetItemOutput{unprocessed("s1")}}
		rows, err := DynamoCCLAs{DB: db, Table: table}.GetCCLAs(context.Background(), []string{"s0", "s1"})
		require.EqualError(t, err, "reading CCLAs: 1 rows still unprocessed after 5 batch read attempts")
		assert.Nil(t, rows)
		assert.Len(t, db.batchInputs, batchGetAttempts)
	})
	t.Run("a cancelled context stops the retries", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		db := &fakeDB{batches: []*dynamodb.BatchGetItemOutput{unprocessed("s1")}, onBatch: cancel}
		rows, err := DynamoCCLAs{DB: db, Table: table}.GetCCLAs(ctx, []string{"s0", "s1"})
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, rows)
		assert.Len(t, db.batchInputs, 1, "no reissue after cancellation")
	})
}

func TestDynamoCompaniesReadsAccountsConsistently(t *testing.T) {
	fastBackoff(t)
	const table = "cla-dev-companies"
	db := &fakeDB{batches: []*dynamodb.BatchGetItemOutput{{Responses: map[string][]map[string]*dynamodb.AttributeValue{table: {
		{"company_id": {S: aws.String("c-1")}, "company_external_id": {S: aws.String(sfid18)}},
		{"company_id": {S: aws.String("c-2")}},
	}}}}}
	accounts, err := DynamoCompanies{DB: db, Table: table}.GetCompanyAccounts(context.Background(), []string{"c-1", "c-2", "c-3"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"c-1": sfid18, "c-2": ""}, accounts, "a row without an account id and a missing row never match an account")
	require.Len(t, db.batchInputs, 1)
	in := db.batchInputs[0].RequestItems[table]
	assert.True(t, aws.BoolValue(in.ConsistentRead))
	assert.Equal(t, "company_id, company_external_id", aws.StringValue(in.ProjectionExpression))
	assert.Len(t, in.Keys, 3)
}

func TestPublishRegistersTheChangedRowsBeforeTheClock(t *testing.T) {
	pub := &fakePublisher{}
	h := newHarness(pub)
	_, err := h.svc.Publish(context.Background(), sfid18, "s3", " ", "s3")
	require.NoError(t, err)
	assert.Equal(t, []string{"register s3", "now", "catalog"}, h.log[:3], "the id is in the durable catalog before the stamp is taken")
	assert.Equal(t, map[string]bool{"s3": true}, h.pending.sets[sfid18])
}

// two publishers (separate instances, separate clocks) sharing the durable catalog, empty indexes, two new
// CCLAs of distinct companies of one Account: the later replacement carries both managers
func TestConcurrentNewCCLAsSharingTheCatalog(t *testing.T) {
	shared := newFakePending()
	emptyIndexes := func(h *harness) {
		companies, ok := h.svc.Companies.(fakeCompanies)
		require.True(t, ok)
		companies.rows = map[string][]*models.Company{}
		h.svc.Companies, h.svc.CompanyRows = companies, companies
		h.cclas.index = map[string][]string{}
		h.cclas.base = map[string]CCLA{
			"sA": {SignatureID: "sA", CompanyID: "c-entity-1", Signed: true, ACL: []string{"amy"}},
			"sB": {SignatureID: "sB", CompanyID: "c-entity-2", Signed: true, ACL: []string{"bob"}},
		}
	}
	pubA, pubB := &fakePublisher{}, &fakePublisher{}
	a := newHarnessWith(pubA, shared, baseTime)
	b := newHarnessWith(pubB, shared, baseTime.Add(time.Second))
	emptyIndexes(a)
	emptyIndexes(b)

	resA, err := a.svc.Publish(context.Background(), sfid18, "sA")
	require.NoError(t, err)
	assert.Equal(t, []string{"amy"}, resA.Roster.Usernames())
	resB, err := b.svc.Publish(context.Background(), sfid18, "sB")
	require.NoError(t, err)
	assert.Equal(t, []string{"amy", "bob"}, resB.Roster.Usernames(), "the later publisher reads the catalog entry of the earlier one although no index lists either row")
	assert.Equal(t, []string{"c-entity-1", "c-entity-2"}, resB.Roster.CompanyIDs)
	require.Len(t, pubB.calls, 1)
	assert.True(t, pubB.calls[0].at.After(pubA.calls[0].at), "the complete roster carries the later stamp")
	assert.Equal(t, map[string]bool{"sA": true, "sB": true}, shared.sets[sfid18])
}

// a registered row whose company no index lists yet makes that company's other rows count too
func TestRosterCatalogEntrySeedsCompanyDiscovery(t *testing.T) {
	h := newHarness(&fakePublisher{})
	companies, ok := h.svc.Companies.(fakeCompanies)
	require.True(t, ok)
	companies.rows = map[string][]*models.Company{}
	h.svc.Companies, h.svc.CompanyRows = companies, companies
	h.cclas.index = map[string][]string{"c-entity-2": {"s3", "s9"}}
	require.NoError(t, h.pending.Add(context.Background(), sfid18, "s9"))
	h.log = nil
	roster, err := h.svc.Roster(context.Background(), sfid18)
	require.NoError(t, err)
	assert.Equal(t, []string{"amy", "bob", "zed"}, roster.Usernames())
	assert.Equal(t, []string{"catalog", "read s9", "list c-entity-2", "read s3,s9", "companies c-entity-2"}, h.log)
}

func TestPublishCatalogFailures(t *testing.T) {
	t.Run("a failed registration publishes nothing", func(t *testing.T) {
		pub := &fakePublisher{}
		h := newHarness(pub)
		h.pending.addErr = errors.New("dynamo down")
		_, err := h.svc.Publish(context.Background(), sfid18, "s3")
		assert.ErrorContains(t, err, "registering the changed CCLAs of "+sfid18+": dynamo down")
		assert.Empty(t, pub.calls)
		assert.Equal(t, []string{"register s3"}, h.log)
	})
	t.Run("a failed catalog read publishes nothing", func(t *testing.T) {
		pub := &fakePublisher{}
		h := newHarness(pub)
		h.pending.listErr = errors.New("dynamo down")
		_, err := h.svc.Publish(context.Background(), sfid18)
		assert.ErrorContains(t, err, "reading the pending-id catalog of "+sfid18+": dynamo down")
		assert.Empty(t, pub.calls)
	})
	t.Run("a publisher without the catalog is a configuration error", func(t *testing.T) {
		pub := &fakePublisher{}
		h := newHarness(pub)
		h.svc.Pending = nil
		_, err := h.svc.Publish(context.Background(), sfid18)
		assert.ErrorIs(t, err, ErrNoPendingCatalog)
		_, err = h.svc.Roster(context.Background(), sfid18)
		assert.ErrorIs(t, err, ErrNoPendingCatalog)
		assert.Empty(t, pub.calls)
	})
}

func TestAccountOf(t *testing.T) {
	h := newHarness(&fakePublisher{})
	account, found, err := h.svc.AccountOf(context.Background(), "c-entity-2")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, sfid18, account)
	_, found, err = h.svc.AccountOf(context.Background(), "c-gone")
	require.NoError(t, err)
	assert.False(t, found)
	companies, ok := h.svc.Companies.(fakeCompanies)
	require.True(t, ok)
	companies.accountsErr = errors.New("dynamo down")
	h.svc.CompanyRows = companies
	_, _, err = h.svc.AccountOf(context.Background(), "c-entity-2")
	assert.ErrorContains(t, err, "reading company c-entity-2: dynamo down")
}

func TestDynamoPendingIDs(t *testing.T) {
	const table = "cla-dev-store"
	db := &fakeDB{getOut: &dynamodb.GetItemOutput{Item: map[string]*dynamodb.AttributeValue{"signature_ids": {SS: aws.StringSlice([]string{"s9", "s1"})}}}}
	catalog := DynamoPendingIDs{DB: db, Table: table}
	require.NoError(t, catalog.Add(context.Background(), sfid18, "s1", "s9"))
	require.NoError(t, catalog.Add(context.Background(), sfid18), "nothing to register, no call")
	require.Len(t, db.updates, 1)
	assert.Equal(t, table, aws.StringValue(db.updates[0].TableName))
	assert.Equal(t, "cla_admins:"+sfid18, aws.StringValue(db.updates[0].Key["key"].S))
	assert.Equal(t, "ADD signature_ids :ids", aws.StringValue(db.updates[0].UpdateExpression))
	assert.Equal(t, []string{"s1", "s9"}, aws.StringValueSlice(db.updates[0].ExpressionAttributeValues[":ids"].SS))

	ids, err := catalog.List(context.Background(), sfid18)
	require.NoError(t, err)
	assert.Equal(t, []string{"s1", "s9"}, ids)
	require.Len(t, db.gets, 1)
	assert.True(t, aws.BoolValue(db.gets[0].ConsistentRead))
	assert.Equal(t, "cla_admins:"+sfid18, aws.StringValue(db.gets[0].Key["key"].S))

	db.getOut = &dynamodb.GetItemOutput{}
	ids, err = catalog.List(context.Background(), sfid18)
	require.NoError(t, err)
	assert.Nil(t, ids, "no item yet, nothing registered")

	db.getErr = errors.New("dynamo down")
	_, err = catalog.List(context.Background(), sfid18)
	assert.ErrorContains(t, err, "pending-id catalog: dynamo down")
	db.updateErr = errors.New("item too large")
	assert.ErrorContains(t, catalog.Add(context.Background(), sfid18, "s2"), "pending-id catalog: item too large")
}
