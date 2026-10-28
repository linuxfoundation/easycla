// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// Package cla_admins projects the CLA managers of an organization's signed CCLAs onto the LFX v2
// member-service cla_admins list (lfx-self-serve#4435); member-service owns the b2b_org cla_admin tuples.
package cla_admins

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

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

// CompanyLister returns every company row stored under one Salesforce account id (company.IRepository satisfies it).
type CompanyLister interface {
	GetCompaniesByExternalID(ctx context.Context, companySFID string, includeChildCompanies bool) ([]*models.Company, error)
}

// UserLookup resolves an LF username to its user record (users.UserRepository satisfies it).
type UserLookup interface {
	GetUserByLFUserName(lfUserName string) (*models.User, error)
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
	Companies CompanyLister
	CCLAs     CCLAReader
	Users     UserLookup
	Members   Publisher
	Now       func() time.Time
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
// index lists for its companies plus the signatureIDs the caller knows changed, all read from the base
// table as they are now, so neither a stale index entry nor an old stream image decides.
func (s Service) Roster(ctx context.Context, sfid string, signatureIDs ...string) (*Roster, error) {
	id18, ok := AccountID(sfid)
	if !ok {
		return nil, member_service.ErrInvalidSFID
	}
	f := logrus.Fields{"functionName": "cla_admins.Service.Roster", "sfid": id18}
	companyIDs, err := s.companyIDs(ctx, id18)
	if err != nil {
		return nil, err
	}
	roster := &Roster{SFID: id18, CompanyIDs: companyIDs, Admins: []member_service.CLAAdmin{}}
	ids := map[string]bool{}
	for _, companyID := range companyIDs {
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
	for _, id := range signatureIDs {
		if id = strings.TrimSpace(id); id != "" {
			ids[id] = true
		}
	}
	rows, err := s.CCLAs.GetCCLAs(ctx, sortedKeys(ids))
	if err != nil {
		return nil, fmt.Errorf("reading the CCLAs of %s: %w", id18, err)
	}
	usernames := map[string]bool{}
	for _, c := range rows {
		if !c.Signed || !utils.StringInSlice(c.CompanyID, companyIDs) {
			continue
		}
		roster.SignedCCLAs++
		for _, u := range c.ACL {
			if u = strings.TrimSpace(u); u != "" {
				usernames[u] = true
			}
		}
	}
	for _, u := range sortedKeys(usernames) {
		roster.Admins = append(roster.Admins, s.entry(f, u))
	}
	return roster, nil
}

// Publish sends the roster stamped with the time its reads began, so a slower, older publish never
// outranks a newer one; a 409 is retried once with a recomputed roster, a 404 is reported as skipped.
func (s Service) Publish(ctx context.Context, sfid string, signatureIDs ...string) (*Result, error) {
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

func (s Service) entry(f logrus.Fields, username string) member_service.CLAAdmin {
	e := member_service.CLAAdmin{Username: username}
	if s.Users == nil {
		return e
	}
	user, err := s.Users.GetUserByLFUserName(username)
	if err != nil || user == nil {
		log.WithFields(f).WithError(err).Debugf("no user record for CLA manager %s - sending the username only", username)
		return e
	}
	e.Email = strings.TrimSpace(string(user.LfEmail))
	e.Name = strings.TrimSpace(user.Username)
	return e
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
		return fmt.Errorf("reading CCLAs: %w", ctx.Err())
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
	var out []CCLA
	for start := 0; start < len(signatureIDs); start += batchGetLimit {
		end := start + batchGetLimit
		if end > len(signatureIDs) {
			end = len(signatureIDs)
		}
		keys := make([]map[string]*dynamodb.AttributeValue, 0, end-start)
		for _, id := range signatureIDs[start:end] {
			keys = append(keys, map[string]*dynamodb.AttributeValue{"signature_id": {S: aws.String(id)}})
		}
		request := map[string]*dynamodb.KeysAndAttributes{d.Table: {
			Keys:                 keys,
			ConsistentRead:       aws.Bool(true),
			ProjectionExpression: aws.String("signature_id, signature_reference_id, signature_type, signature_reference_type, signature_signed, signature_acl"),
		}}
		for attempt := 0; ; attempt++ {
			if attempt == batchGetAttempts {
				return nil, fmt.Errorf("reading CCLAs: %d rows still unprocessed after %d batch read attempts", len(request[d.Table].Keys), attempt)
			}
			if attempt > 0 {
				if err := waitBeforeRetry(ctx, attempt); err != nil {
					return nil, err
				}
			}
			page, err := d.DB.BatchGetItemWithContext(ctx, &dynamodb.BatchGetItemInput{RequestItems: request})
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
			if err = dynamodbattribute.UnmarshalListOfMaps(page.Responses[d.Table], &rows); err != nil {
				return nil, fmt.Errorf("decoding CCLAs: %w", err)
			}
			for _, r := range rows {
				if r.Type != "ccla" || r.ReferenceType != "company" {
					continue
				}
				out = append(out, CCLA{SignatureID: r.SignatureID, CompanyID: r.CompanyID, Signed: r.Signed, ACL: r.ACL})
			}
			pending := page.UnprocessedKeys[d.Table]
			if pending == nil || len(pending.Keys) == 0 {
				break
			}
			request = page.UnprocessedKeys
		}
	}
	return out, nil
}
