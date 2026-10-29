// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// Package cla_admins projects the CLA managers of an organization's signed CCLAs onto the LFX v2
// member-service cla_admins list (lfx-self-serve#4435); member-service owns the b2b_org cla_admin tuples.
package cla_admins

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/sirupsen/logrus"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

// CCLA is a corporate CLA row as the signatures table holds it now.
type CCLA struct {
	SignatureID string
	CompanyID   string
	Signed      bool
	ACL         []string
}

// CCLAReader finds a company's corporate CLA rows through the eventually consistent index and reads their
// current state from the base table.
type CCLAReader interface {
	ListCompanyCCLAIDs(ctx context.Context, companyID string) ([]string, error)
	GetCCLAs(ctx context.Context, signatureIDs []string) ([]CCLA, error)
}

// CompanyLister discovers the company rows the (eventually consistent) index stores under one Salesforce
// account id (company.IRepository satisfies it).
type CompanyLister interface {
	GetCompaniesByExternalID(ctx context.Context, companySFID string, includeChildCompanies bool) ([]*models.Company, error)
}

// CompanyRowReader reads company rows as the base table holds them now: company id -> company_external_id.
type CompanyRowReader interface {
	GetCompanyAccounts(ctx context.Context, companyIDs []string) (map[string]string, error)
}

// PendingIDs is the durable per-account catalog of the CCLA signature ids publishers registered before
// publishing, so a later publisher reads them even while the signatures index has not listed them yet.
type PendingIDs interface {
	Add(ctx context.Context, sfid18 string, signatureIDs ...string) error
	List(ctx context.Context, sfid18 string) ([]string, error)
}

// ErrNoPendingCatalog reports a Service without the pending-id catalog, which every publisher needs.
var ErrNoPendingCatalog = errors.New("cla_admins: pending-id catalog not configured")

// UserLookup resolves an LF username, or a legacy github:<id> ACL entry, to its user record
// (users.UserRepository satisfies it).
type UserLookup interface {
	GetUserByLFUserName(lfUserName string) (*models.User, error)
	GetUserByUserName(userName string, fullMatch bool) (*models.User, error)
}

// Publisher replaces the organization's CLA manager list (*member_service.Client satisfies it).
type Publisher interface {
	PutB2BOrgCLAAdmins(ctx context.Context, uid string, admins []member_service.CLAAdmin, sourceUpdatedAt time.Time) error
}

// Roster is one organization's CLA manager list: the union of signature_acl over the signed CCLAs of
// every company row sharing its Salesforce account id.
type Roster struct {
	SFID        string
	CompanyIDs  []string
	SignedCCLAs int
	Admins      []member_service.CLAAdmin
}

// Usernames lists the roster's LF usernames.
func (r *Roster) Usernames() []string {
	names := make([]string, 0, len(r.Admins))
	for _, a := range r.Admins {
		names = append(names, a.Username)
	}
	return names
}

// Result reports one publish.
type Result struct {
	Roster  *Roster
	Skipped bool // no b2b_org registered yet (404); the backfill picks the organization up later
	Retried bool // the first PUT lost a concurrent update (409) and a recomputed roster was sent
}

// Service computes and publishes rosters.
type Service struct {
	Companies   CompanyLister
	CompanyRows CompanyRowReader
	CCLAs       CCLAReader
	Pending     PendingIDs
	Users       UserLookup
	Members     Publisher
	Now         func() time.Time
}

// AccountID returns the 18-character form of a Salesforce Account id (001...); legacy org-service
// ids (lf...) and anything else are rejected because they never become b2b orgs.
func AccountID(id string) (string, bool) {
	id18, ok := member_service.SFID18(strings.TrimSpace(id))
	if !ok || !strings.HasPrefix(id18, "001") {
		return "", false
	}
	return id18, true
}

// Roster computes the roster of the organization identified by sfid (15 or 18 characters): the rows the
// index lists for its companies, the ids registered in the catalog and the signatureIDs the caller names,
// all read from the base table as they are now; only rows of companies whose base row carries this
// account count, so neither a stale index entry nor an old stream image decides.
func (s Service) Roster(ctx context.Context, sfid string, signatureIDs ...string) (*Roster, error) {
	id18, ok := AccountID(sfid)
	if !ok {
		return nil, member_service.ErrInvalidSFID
	}
	if s.Pending == nil {
		return nil, ErrNoPendingCatalog
	}
	f := logrus.Fields{"functionName": "cla_admins.Service.Roster", "sfid": id18}
	companyIDs, err := s.companyIDs(ctx, id18)
	if err != nil {
		return nil, err
	}
	registered, err := s.Pending.List(ctx, id18)
	if err != nil {
		return nil, fmt.Errorf("reading the pending-id catalog of %s: %w", id18, err)
	}
	seed := nonEmpty(append(append([]string{}, registered...), signatureIDs...))
	discovered := map[string]bool{}
	for _, companyID := range companyIDs {
		discovered[companyID] = true
	}
	if len(seed) > 0 {
		// a registered row may name a company the company index does not list yet - its other rows count too
		seedRows, readErr := s.CCLAs.GetCCLAs(ctx, seed)
		if readErr != nil {
			return nil, fmt.Errorf("reading the registered CCLAs of %s: %w", id18, readErr)
		}
		for _, c := range seedRows {
			if c.CompanyID != "" {
				discovered[c.CompanyID] = true
			}
		}
	}
	ids := map[string]bool{}
	for _, id := range seed {
		ids[id] = true
	}
	for _, companyID := range sortedKeys(discovered) {
		listed, listErr := s.CCLAs.ListCompanyCCLAIDs(ctx, companyID)
		if listErr != nil {
			return nil, fmt.Errorf("listing CCLAs of company %s: %w", companyID, listErr)
		}
		for _, id := range listed {
			if id != "" {
				ids[id] = true
			}
		}
	}
	rows, err := s.CCLAs.GetCCLAs(ctx, sortedKeys(ids))
	if err != nil {
		return nil, fmt.Errorf("reading the CCLAs of %s: %w", id18, err)
	}
	verified, err := s.verifyCompanies(ctx, id18, rows)
	if err != nil {
		return nil, err
	}
	roster := &Roster{SFID: id18, CompanyIDs: verified, Admins: []member_service.CLAAdmin{}}
	usernames := map[string]bool{}
	skipped := map[string]bool{}
	for _, c := range rows {
		if !c.Signed || !utils.StringInSlice(c.CompanyID, verified) {
			continue
		}
		roster.SignedCCLAs++
		for _, u := range c.ACL {
			if u = strings.TrimSpace(u); u == "" {
				continue
			}
			if strings.HasPrefix(u, "github:") {
				resolved, lookupErr := s.legacyUsername(u)
				if lookupErr != nil {
					log.WithFields(f).Warn("legacy CLA manager entry lookup failed - roster not published")
					return nil, errLegacyManagerLookup
				}
				if resolved != "" {
					usernames[resolved] = true
				} else {
					skipped[u] = true
				}
				continue
			}
			if !plainUsername(u) {
				skipped[u] = true
				continue
			}
			usernames[u] = true
		}
	}
	if len(skipped) > 0 {
		log.WithFields(f).WithFields(logrus.Fields{"skipped": len(skipped), "reason": "not a publishable LF username"}).Warn("CLA manager usernames skipped - not publishable to member-service")
	}
	for _, u := range sortedKeys(usernames) {
		roster.Admins = append(roster.Admins, s.entry(f, u))
	}
	return roster, nil
}

// Publish registers the changed signatureIDs in the catalog, then sends the roster stamped with the time
// its reads began, so a slower, older publish never outranks a newer one and a concurrent publisher
// reads the registered ids; a 409 is retried once with a recomputed roster, a 404 is reported as skipped.
func (s Service) Publish(ctx context.Context, sfid string, signatureIDs ...string) (*Result, error) {
	id18, ok := AccountID(sfid)
	if !ok {
		return nil, member_service.ErrInvalidSFID
	}
	if s.Pending == nil {
		return nil, ErrNoPendingCatalog
	}
	if ids := nonEmpty(signatureIDs); len(ids) > 0 {
		if err := s.Pending.Add(ctx, id18, ids...); err != nil {
			return nil, fmt.Errorf("registering the changed CCLAs of %s: %w", id18, err)
		}
	}
	res := &Result{}
	for attempt := 0; ; attempt++ {
		at := s.now()
		roster, err := s.Roster(ctx, sfid, signatureIDs...)
		if err != nil {
			return nil, err
		}
		res.Roster = roster
		err = s.Members.PutB2BOrgCLAAdmins(ctx, roster.SFID, roster.Admins, at)
		switch {
		case errors.Is(err, member_service.ErrConflict) && attempt == 0:
			res.Retried = true
		case errors.Is(err, member_service.ErrOrgNotFound):
			res.Skipped = true
			return res, nil
		case err != nil:
			return res, err
		default:
			return res, nil
		}
	}
}

// AccountOf returns the Salesforce account id a company's base row carries now, read consistently;
// found is false when the row is gone.
func (s Service) AccountOf(ctx context.Context, companyID string) (string, bool, error) {
	accounts, err := s.CompanyRows.GetCompanyAccounts(ctx, []string{companyID})
	if err != nil {
		return "", false, fmt.Errorf("reading company %s: %w", companyID, err)
	}
	account, found := accounts[companyID]
	return account, found, nil
}

// nonEmpty returns the trimmed, deduplicated, sorted non-empty entries.
func nonEmpty(ids []string) []string {
	set := map[string]bool{}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			set[id] = true
		}
	}
	return sortedKeys(set)
}

// verifyCompanies returns the companies of the read rows whose base row carries this account id - listed
// by the index or not - so a stale listing or a missing one never decides.
func (s Service) verifyCompanies(ctx context.Context, id18 string, rows []CCLA) ([]string, error) {
	referenced := map[string]bool{}
	for _, c := range rows {
		if c.CompanyID != "" {
			referenced[c.CompanyID] = true
		}
	}
	if len(referenced) == 0 {
		return []string{}, nil
	}
	accounts, err := s.CompanyRows.GetCompanyAccounts(ctx, sortedKeys(referenced))
	if err != nil {
		return nil, fmt.Errorf("reading the companies of %s: %w", id18, err)
	}
	verified := map[string]bool{}
	for companyID := range referenced {
		if owner, ok := AccountID(accounts[companyID]); ok && owner == id18 {
			verified[companyID] = true
		}
	}
	return sortedKeys(verified), nil
}

// companyIDs returns the ids of the rows stored under the 18- or the 15-character form of the account id.
func (s Service) companyIDs(ctx context.Context, id18 string) ([]string, error) {
	seen := map[string]bool{}
	for _, key := range []string{id18, id18[:15]} {
		companies, err := s.Companies.GetCompaniesByExternalID(ctx, key, true)
		if err != nil {
			var notFound *utils.CompanyNotFound
			if errors.As(err, &notFound) {
				continue
			}
			return nil, fmt.Errorf("listing companies of %s: %w", key, err)
		}
		for _, c := range companies {
			if c != nil && c.CompanyID != "" {
				seen[c.CompanyID] = true
			}
		}
	}
	return sortedKeys(seen), nil
}

// errLegacyManagerLookup aborts a roster whose legacy github:<id> entry could not be looked up; the entry itself
// stays out of logs and error text
var errLegacyManagerLookup = errors.New("cla_admins: legacy CLA manager entry lookup failed")

// legacyUsername resolves a github:<id> ACL entry to the LF username of its user record, as the signature
// converter does; a missing record or one without a usable LF username is not publishable (""), a lookup
// failure is an error
func (s Service) legacyUsername(entry string) (string, error) {
	if s.Users == nil {
		return "", nil
	}
	user, err := s.Users.GetUserByUserName(entry, true)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", nil
	}
	if u := strings.TrimSpace(user.LfUsername); u != "" && plainUsername(u) {
		return u, nil
	}
	return "", nil
}

func (s Service) entry(f logrus.Fields, username string) member_service.CLAAdmin {
	e := member_service.CLAAdmin{Username: username}
	if s.Users == nil {
		return e
	}
	user, err := s.Users.GetUserByLFUserName(username)
	if err != nil || user == nil {
		log.WithFields(f).WithField("lookupFailed", err != nil).Debug("no user record for a CLA manager - sending the username only")
		return e
	}
	if email := strings.TrimSpace(string(user.LfEmail)); email != "" {
		if plainEmail(email) {
			e.Email = email
		} else {
			log.WithFields(f).WithField("reason", "not a plain email address").Warn("CLA manager email omitted - the username is sent alone")
		}
	}
	e.Name = strings.TrimSpace(user.Username)
	return e
}

func plainUsername(u string) bool {
	return strings.IndexFunc(u, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) < 0
}

// plainEmail accepts a bare address only: display-name and comment forms parse but are not the plain
// email member-service expects
func plainEmail(e string) bool {
	addr, err := mail.ParseAddress(e)
	return err == nil && addr.Address == e
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// DynamoCCLAs reads corporate CLA rows of the signatures table: ids through the reference-signature-index,
// state through consistent base-table reads.
type DynamoCCLAs struct {
	DB    dynamodbiface.DynamoDBAPI
	Table string
}

// batchGetLimit is DynamoDB's BatchGetItem key limit per request; unprocessed keys are reissued at most
// batchGetAttempts times with an incremental wait, like signatures.getItemSignaturesConsistent.
const (
	batchGetLimit    = 100
	batchGetAttempts = 5
)

var batchGetBackoff = 50 * time.Millisecond

func waitBeforeRetry(ctx context.Context, attempt int) error {
	timer := time.NewTimer(time.Duration(attempt) * batchGetBackoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ListCompanyCCLAIDs returns the ids of every ccla row the index references for the company, signed or not.
func (d DynamoCCLAs) ListCompanyCCLAIDs(ctx context.Context, companyID string) ([]string, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(d.Table),
		IndexName:              aws.String("reference-signature-index"),
		KeyConditionExpression: aws.String("signature_reference_id = :id"),
		FilterExpression:       aws.String("signature_type = :ccla AND signature_reference_type = :company"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":id":      {S: aws.String(companyID)},
			":ccla":    {S: aws.String("ccla")},
			":company": {S: aws.String("company")},
		},
		ProjectionExpression: aws.String("signature_id"),
	}
	var out []string
	for {
		page, err := d.DB.QueryWithContext(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("listing CCLAs of %s: %w", companyID, err)
		}
		for _, item := range page.Items {
			if v, ok := item["signature_id"]; ok && v.S != nil {
				out = append(out, *v.S)
			}
		}
		if len(page.LastEvaluatedKey) == 0 {
			return out, nil
		}
		input.ExclusiveStartKey = page.LastEvaluatedKey
	}
}

// GetCCLAs reads the rows from the base table with consistent reads; rows that are gone or are not
// corporate CLAs any more are left out. Keys DynamoDB leaves unprocessed are reissued a bounded number
// of times; when they stay unprocessed an error is returned rather than a partial roster.
func (d DynamoCCLAs) GetCCLAs(ctx context.Context, signatureIDs []string) ([]CCLA, error) {
	items, err := consistentBatchGet(ctx, d.DB, d.Table, "signature_id", signatureIDs,
		"signature_id, signature_reference_id, signature_type, signature_reference_type, signature_signed, signature_acl")
	if err != nil {
		return nil, fmt.Errorf("reading CCLAs: %w", err)
	}
	var rows []struct {
		SignatureID   string   `json:"signature_id"`
		CompanyID     string   `json:"signature_reference_id"`
		Type          string   `json:"signature_type"`
		ReferenceType string   `json:"signature_reference_type"`
		Signed        bool     `json:"signature_signed"`
		ACL           []string `json:"signature_acl"`
	}
	if err = dynamodbattribute.UnmarshalListOfMaps(items, &rows); err != nil {
		return nil, fmt.Errorf("decoding CCLAs: %w", err)
	}
	var out []CCLA
	for _, r := range rows {
		if r.Type != "ccla" || r.ReferenceType != "company" {
			continue
		}
		out = append(out, CCLA{SignatureID: r.SignatureID, CompanyID: r.CompanyID, Signed: r.Signed, ACL: r.ACL})
	}
	return out, nil
}

// pendingKeyPrefix namespaces the catalog items in the store table: one item per account, a string set of
// signature ids, no expiry.
const pendingKeyPrefix = "cla_admins:"

// DynamoPendingIDs keeps the per-account catalog in the store table.
type DynamoPendingIDs struct {
	DB    dynamodbiface.DynamoDBAPI
	Table string
}

// Add unions the ids into the account's set atomically (creating the item on first use).
func (d DynamoPendingIDs) Add(ctx context.Context, sfid18 string, signatureIDs ...string) error {
	if len(signatureIDs) == 0 {
		return nil
	}
	_, err := d.DB.UpdateItemWithContext(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(d.Table),
		Key:                       map[string]*dynamodb.AttributeValue{"key": {S: aws.String(pendingKeyPrefix + sfid18)}},
		UpdateExpression:          aws.String("ADD signature_ids :ids"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{":ids": {SS: aws.StringSlice(signatureIDs)}},
	})
	if err != nil {
		return fmt.Errorf("pending-id catalog: %w", err)
	}
	return nil
}

// List reads the account's set with a consistent read; an account without an item has no ids.
func (d DynamoPendingIDs) List(ctx context.Context, sfid18 string) ([]string, error) {
	out, err := d.DB.GetItemWithContext(ctx, &dynamodb.GetItemInput{
		TableName:            aws.String(d.Table),
		Key:                  map[string]*dynamodb.AttributeValue{"key": {S: aws.String(pendingKeyPrefix + sfid18)}},
		ConsistentRead:       aws.Bool(true),
		ProjectionExpression: aws.String("signature_ids"),
	})
	if err != nil {
		return nil, fmt.Errorf("pending-id catalog: %w", err)
	}
	set, ok := out.Item["signature_ids"]
	if !ok || set == nil {
		return nil, nil
	}
	ids := aws.StringValueSlice(set.SS)
	sort.Strings(ids)
	return ids, nil
}

// DynamoCompanies reads company rows of the companies table with consistent reads.
type DynamoCompanies struct {
	DB    dynamodbiface.DynamoDBAPI
	Table string
}

// GetCompanyAccounts returns company id -> company_external_id for the rows that exist now.
func (d DynamoCompanies) GetCompanyAccounts(ctx context.Context, companyIDs []string) (map[string]string, error) {
	items, err := consistentBatchGet(ctx, d.DB, d.Table, "company_id", companyIDs, "company_id, company_external_id")
	if err != nil {
		return nil, fmt.Errorf("reading companies: %w", err)
	}
	var rows []struct {
		CompanyID  string `json:"company_id"`
		ExternalID string `json:"company_external_id"`
	}
	if err = dynamodbattribute.UnmarshalListOfMaps(items, &rows); err != nil {
		return nil, fmt.Errorf("decoding companies: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.CompanyID] = r.ExternalID
	}
	return out, nil
}

// consistentBatchGet reads the keyed items with ConsistentRead in chunks of batchGetLimit; keys DynamoDB
// leaves unprocessed are reissued at most batchGetAttempts times with an incremental, cancellable wait,
// and an error is returned when they stay unprocessed.
func consistentBatchGet(ctx context.Context, db dynamodbiface.DynamoDBAPI, table, keyName string, ids []string, projection string) ([]map[string]*dynamodb.AttributeValue, error) {
	var out []map[string]*dynamodb.AttributeValue
	for start := 0; start < len(ids); start += batchGetLimit {
		end := start + batchGetLimit
		if end > len(ids) {
			end = len(ids)
		}
		keys := make([]map[string]*dynamodb.AttributeValue, 0, end-start)
		for _, id := range ids[start:end] {
			keys = append(keys, map[string]*dynamodb.AttributeValue{keyName: {S: aws.String(id)}})
		}
		request := map[string]*dynamodb.KeysAndAttributes{table: {
			Keys:                 keys,
			ConsistentRead:       aws.Bool(true),
			ProjectionExpression: aws.String(projection),
		}}
		for attempt := 0; ; attempt++ {
			if attempt == batchGetAttempts {
				return nil, fmt.Errorf("%d rows still unprocessed after %d batch read attempts", len(request[table].Keys), attempt)
			}
			if attempt > 0 {
				if err := waitBeforeRetry(ctx, attempt); err != nil {
					return nil, err
				}
			}
			page, err := db.BatchGetItemWithContext(ctx, &dynamodb.BatchGetItemInput{RequestItems: request})
			if err != nil {
				return nil, err
			}
			out = append(out, page.Responses[table]...)
			pending := page.UnprocessedKeys[table]
			if pending == nil || len(pending.Keys) == 0 {
				break
			}
			request = page.UnprocessedKeys
		}
	}
	return out, nil
}
