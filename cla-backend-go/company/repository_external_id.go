// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/gofrs/uuid"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// ErrExternalIDConditionFailed is returned by UpdateCompanyExternalID when the row no longer carries the expected external ID.
var ErrExternalIDConditionFailed = errors.New("company external id changed concurrently")

// ErrEnsureCompanyConflict is returned by EnsureCompanyForExternalID when the deterministic company id is already taken by a row of another organization.
var ErrEnsureCompanyConflict = errors.New("company id already used by a different organization")

// ErrEmptyExternalID is returned when an external ID is required but empty.
var ErrEmptyExternalID = errors.New("company external id is empty")

const ensureCompanyNote = "created at CCLA signing"

// canonicalSigningEntity returns the identity component of a signing entity: empty for the
// parent record (no name, or the name equals the company name), otherwise the normalized name.
func canonicalSigningEntity(companyName, signingEntityName string) string {
	entity := strings.ToLower(strings.TrimSpace(signingEntityName))
	if entity == "" || entity == strings.ToLower(strings.TrimSpace(companyName)) {
		return ""
	}
	return entity
}

// deterministicCompanyID derives a stable, UUIDv4-shaped company id from the external ID and the
// canonical signing entity so that concurrent first creations for the same organization collide on
// the primary key instead of producing two rows.
func deterministicCompanyID(externalID, canonicalEntity string) string {
	sum := sha256.Sum256([]byte("easycla-company|" + externalID + "|" + canonicalEntity))
	var id uuid.UUID
	copy(id[:], sum[:16])
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return id.String()
}

// pickCompanyForEntity returns the row matching the canonical signing entity, or nil.
func pickCompanyForEntity(rows []*models.Company, canonicalEntity string) *models.Company {
	for _, row := range rows {
		if row != nil && canonicalSigningEntity(row.CompanyName, row.SigningEntityName) == canonicalEntity {
			return row
		}
	}
	return nil
}

// EnsureCompanyForExternalID returns the company row for (externalID, signing entity), creating it
// when it does not exist. Existing rows always win and keep their ids; a new row gets the
// deterministic id and a conditional put, and on a conditional failure the winning row is read back
// and returned when it belongs to the same organization. The returned flag reports a creation.
func (repo repository) EnsureCompanyForExternalID(ctx context.Context, externalID, companyName, signingEntityName string) (*models.Company, bool, error) {
	f := logrus.Fields{
		"functionName":      "company.repository.EnsureCompanyForExternalID",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"companySFID":       externalID,
		"companyName":       companyName,
		"signingEntityName": signingEntityName,
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return nil, false, ErrEmptyExternalID
	}
	companyName = strings.TrimSpace(companyName)
	if companyName == "" {
		return nil, false, errors.New("company name is empty")
	}
	canonical := canonicalSigningEntity(companyName, signingEntityName)

	var existing *models.Company
	if canonical == "" {
		rows, err := repo.GetCompaniesByExternalID(ctx, externalID, false)
		if err != nil {
			if _, notFound := err.(*utils.CompanyNotFound); !notFound {
				return nil, false, err
			}
		} else if len(rows) > 0 {
			existing = rows[0]
		}
	} else {
		rows, err := repo.GetCompaniesByExternalID(ctx, externalID, true)
		if err != nil {
			if _, notFound := err.(*utils.CompanyNotFound); !notFound {
				return nil, false, err
			}
		} else {
			existing = pickCompanyForEntity(rows, canonical)
		}
	}
	if existing != nil {
		log.WithFields(f).Debugf("reusing existing company %s", existing.CompanyID)
		return existing, false, nil
	}

	_, now := utils.CurrentTime()
	record := &DBModel{
		CompanyID:         deterministicCompanyID(externalID, canonical),
		CompanyName:       companyName,
		SigningEntityName: companyName,
		CompanyExternalID: externalID,
		Created:           now,
		Updated:           now,
		Note:              ensureCompanyNote,
		Version:           "v1",
	}
	if canonical != "" {
		record.SigningEntityName = strings.TrimSpace(signingEntityName)
	}
	f["companyID"] = record.CompanyID

	av, err := dynamodbattribute.MarshalMap(record)
	if err != nil {
		return nil, false, err
	}
	_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:                av,
		TableName:           aws.String(repo.companyTableName),
		ConditionExpression: aws.String("attribute_not_exists(company_id)"),
	})
	if err == nil {
		log.WithFields(f).Info("company created")
		created, convErr := record.toModel()
		return created, true, convErr
	}
	if aerr, ok := err.(awserr.Error); !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
		log.WithFields(f).WithError(err).Warn("problem creating company")
		return nil, false, err
	}

	winner, err := repo.getCompanyRecord(ctx, record.CompanyID, true)
	if err != nil {
		return nil, false, err
	}
	if winner.CompanyExternalID != externalID || canonicalSigningEntity(winner.CompanyName, winner.SigningEntityName) != canonical {
		log.WithFields(f).Warnf("company id collision with external id %s / entity %q", winner.CompanyExternalID, winner.SigningEntityName)
		return nil, false, ErrEnsureCompanyConflict
	}
	log.WithFields(f).Debug("company created concurrently - returning the winning row")
	model, err := winner.toModel()
	return model, false, err
}

// GetCompanyRecord returns the raw company row (including previous_company_external_id) by id.
func (repo repository) GetCompanyRecord(ctx context.Context, companyID string) (*DBModel, error) {
	return repo.getCompanyRecord(ctx, companyID, false)
}

func (repo repository) getCompanyRecord(ctx context.Context, companyID string, consistent bool) (*DBModel, error) {
	f := logrus.Fields{
		"functionName":   "company.repository.getCompanyRecord",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
	}
	if companyID == "" {
		return nil, &utils.CompanyNotFound{Message: "company_id cannot be empty", CompanyID: companyID}
	}
	out, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName:      aws.String(repo.companyTableName),
		Key:            map[string]*dynamodb.AttributeValue{"company_id": {S: aws.String(companyID)}},
		ConsistentRead: aws.Bool(consistent),
	})
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error fetching company record")
		return nil, err
	}
	if len(out.Item) == 0 {
		return nil, &utils.CompanyNotFound{Message: "no company matching company record", CompanyID: companyID}
	}
	record := &DBModel{}
	if err = dynamodbattribute.UnmarshalMap(out.Item, record); err != nil {
		return nil, err
	}
	return record, nil
}

// UpdateCompanyExternalID rewrites company_external_id from oldExternalID to newExternalID for one
// row, remembering the old value in previous_company_external_id. The write is conditional on the
// row still carrying oldExternalID (an empty oldExternalID means the row must have no external id
// yet, and no previous value is recorded); otherwise ErrExternalIDConditionFailed is returned.
func (repo repository) UpdateCompanyExternalID(ctx context.Context, companyID, oldExternalID, newExternalID string) error {
	f := logrus.Fields{
		"functionName":   "company.repository.UpdateCompanyExternalID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"oldExternalID":  oldExternalID,
		"newExternalID":  newExternalID,
	}
	if strings.TrimSpace(companyID) == "" || strings.TrimSpace(newExternalID) == "" {
		return fmt.Errorf("company id and new external id are required")
	}
	update, condition := "SET #E = :new, #P = :old, #M = :m", "#E = :old"
	if strings.TrimSpace(oldExternalID) == "" {
		oldExternalID = ""
		update, condition = "SET #E = :new, #M = :m", "attribute_not_exists(#E) OR #E = :old"
	}
	_, now := utils.CurrentTime()
	_, err := repo.dynamoDBClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:           aws.String(repo.companyTableName),
		Key:                 map[string]*dynamodb.AttributeValue{"company_id": {S: aws.String(companyID)}},
		UpdateExpression:    aws.String(update),
		ConditionExpression: aws.String(condition),
		ExpressionAttributeNames: map[string]*string{
			"#E": aws.String("company_external_id"),
			"#P": aws.String("previous_company_external_id"),
			"#M": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":new": {S: aws.String(newExternalID)},
			":old": {S: aws.String(oldExternalID)},
			":m":   {S: aws.String(now)},
		},
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
			return ErrExternalIDConditionFailed
		}
		log.WithFields(f).WithError(err).Warn("error updating company external id")
		return err
	}
	log.WithFields(f).Info("company external id rewritten")
	return nil
}
