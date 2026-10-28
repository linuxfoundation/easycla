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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

const (
	sfid18 = "0014100000Te0G7AAJ"
	sfid15 = "0014100000Te0G7"
)

var baseTime = time.Date(2026, 10, 28, 7, 0, 0, 0, time.UTC)

type fakeCompanies struct{ rows map[string][]*models.Company }

func (f fakeCompanies) GetCompaniesByExternalID(_ context.Context, sfid string, _ bool) ([]*models.Company, error) {
	rows, ok := f.rows[sfid]
	if !ok {
		return nil, &utils.CompanyNotFound{CompanySFID: sfid}
	}
	return rows, nil
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

type fakeUsers struct{ byName map[string]*models.User }

func (f fakeUsers) GetUserByLFUserName(name string) (*models.User, error) {
	u, ok := f.byName[name]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
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

type harness struct {
	svc   Service
	cclas *fakeCCLAs
	pub   *fakePublisher
	log   []string
}

func baseRows() map[string]CCLA {
	return map[string]CCLA{
		"s1": {SignatureID: "s1", CompanyID: "c-entity-1", Signed: true, ACL: []string{"zoe", " amy "}},
		"s2": {SignatureID: "s2", CompanyID: "c-entity-1", Signed: false, ACL: []string{"unsigned-manager"}},
		"s3": {SignatureID: "s3", CompanyID: "c-entity-2", Signed: true, ACL: []string{"amy", "bob", ""}},
		"s7": {SignatureID: "s7", CompanyID: "c-other", Signed: true, ACL: []string{"mallory"}},
		"s9": {SignatureID: "s9", CompanyID: "c-entity-2", Signed: true, ACL: []string{"zed"}},
	}
}

func newHarness(pub *fakePublisher) *harness {
	h := &harness{pub: pub}
	h.cclas = &fakeCCLAs{index: map[string][]string{"c-entity-1": {"s1", "s2"}, "c-entity-2": {"s3"}}, base: baseRows(), log: &h.log}
	pub.log = &h.log
	h.svc = Service{
		Companies: fakeCompanies{rows: map[string][]*models.Company{
			sfid18: {{CompanyID: "c-entity-2", CompanyExternalID: sfid18}},
			sfid15: {{CompanyID: "c-entity-1", CompanyExternalID: sfid15}, {CompanyID: "c-entity-2", CompanyExternalID: sfid18}},
		}},
		CCLAs: h.cclas,
		Users: fakeUsers{byName: map[string]*models.User{
			"amy": {LfUsername: "amy", LfEmail: strfmt.Email("amy@example.com"), Username: "Amy Adams"},
			"bob": {LfUsername: "bob"},
		}},
		Members: pub,
		Now:     (&clock{next: baseTime, log: &h.log}).Now,
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
	assert.Equal(t, []string{"list c-entity-1", "list c-entity-2", "read s1,s2,s3"}, h.log, "ids come from the index, state from one base read")
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
		assert.Equal(t, "read s1,s2,s3,s9", h.log[len(h.log)-1])
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
		assert.Equal(t, []string{"now", "list c-entity-1", "list c-entity-2", "read s1,s2,s3", "put"}, h.log, "the stamp is taken before any read")
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
		h.cclas.switchAfter = 1
		res, err := h.svc.Publish(context.Background(), sfid18, "s3")
		require.NoError(t, err)
		assert.True(t, res.Retried)
		require.Len(t, pub.calls, 2)
		assert.Equal(t, []string{"amy", "bob", "zoe"}, usernamesOf(pub.calls[0].admins))
		assert.Equal(t, []string{"amy", "zoe"}, usernamesOf(pub.calls[1].admins), "bob, removed from the triggering row by another writer meanwhile, is not re-added")
		assert.Equal(t, baseTime, pub.calls[0].at)
		assert.Equal(t, baseTime.Add(time.Second), pub.calls[1].at)
		assert.Equal(t, []string{"amy", "zoe"}, res.Roster.Usernames())
		assert.Equal(t, []string{"now", "list c-entity-1", "list c-entity-2", "read s1,s2,s3", "put", "now", "list c-entity-1", "list c-entity-2", "read s1,s2,s3", "put"}, h.log)
	})
	t.Run("a conflict after the triggering row was deleted drops it", func(t *testing.T) {
		pub := &fakePublisher{errs: []error{member_service.ErrConflict, nil}}
		h := newHarness(pub)
		later := baseRows()
		delete(later, "s3")
		h.cclas.laterBase = later
		h.cclas.switchAfter = 1
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
