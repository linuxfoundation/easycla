// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// ErrEventNotFound is returned by RekeyEventCompanySFID when the event does not exist.
var ErrEventNotFound = errors.New("event not found")

// ErrEventCompanySFIDMismatch is returned when an event carries a company SFID that is neither the old nor the new one.
var ErrEventCompanySFIDMismatch = errors.New("event carries an unexpected company sfid")

// ErrEventRekeyConflict is returned when the event changed between the read and the conditional write; the call can be retried.
var ErrEventRekeyConflict = errors.New("event changed concurrently")

// company SFID prefixed composite index keys maintained by AddDataToEvent
var companySFIDCompositeAttributes = []string{"company_sfid_foundation_sfid", "company_sfid_project_id", "company_sfid_cla_group_id"}

// RekeyRepository is the org-import view of the events table: locate the events of a company SFID and move them to a new SFID.
type RekeyRepository interface {
	ListEventIDsByCompanySFID(ctx context.Context, companySFID string) ([]string, error)
	ListEventIDsByCompanySFIDCLAGroup(ctx context.Context, companySFID, claGroupID string) ([]string, error)
	RekeyEventCompanySFID(ctx context.Context, eventID, oldSFID, newSFID string) (bool, error)
}

// NewRekeyRepository creates the org-import events repository
func NewRekeyRepository(awsSession *session.Session, stage string) RekeyRepository {
	return &repository{
		stage:          stage,
		dynamoDBClient: dynamodb.New(awsSession),
		eventsTable:    fmt.Sprintf("cla-%s-events", stage),
	}
}

// ListEventIDsByCompanySFID returns the ids of all events with event_company_sfid = companySFID
func (repo *repository) ListEventIDsByCompanySFID(ctx context.Context, companySFID string) ([]string, error) {
	if strings.TrimSpace(companySFID) == "" {
		return nil, errors.New("company sfid is required")
	}
	return repo.listEventIDs(ctx, EventCompanySFIDEventDataLowerIndex, expression.Key("event_company_sfid").Equal(expression.Value(companySFID)))
}

// ListEventIDsByCompanySFIDCLAGroup returns the ids of all events with company_sfid_cla_group_id = companySFID#claGroupID
func (repo *repository) ListEventIDsByCompanySFIDCLAGroup(ctx context.Context, companySFID, claGroupID string) ([]string, error) {
	if strings.TrimSpace(companySFID) == "" || strings.TrimSpace(claGroupID) == "" {
		return nil, errors.New("company sfid and cla group id are required")
	}
	return repo.listEventIDs(ctx, CompanySFIDClaGroupIDEpochIndex, expression.Key("company_sfid_cla_group_id").Equal(expression.Value(companySFID+"#"+claGroupID)))
}

func (repo *repository) listEventIDs(ctx context.Context, indexName string, keyCondition expression.KeyConditionBuilder) ([]string, error) {
	f := logrus.Fields{
		"functionName":   "v1.events.repository.listEventIDs",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"indexName":      indexName,
	}
	expr, err := expression.NewBuilder().WithKeyCondition(keyCondition).WithProjection(expression.NamesList(expression.Name("event_id"))).Build()
	if err != nil {
		return nil, err
	}
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(repo.eventsTable),
		IndexName:                 aws.String(indexName),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	var ids []string
	for {
		out, queryErr := repo.dynamoDBClient.Query(input)
		if queryErr != nil {
			log.WithFields(f).WithError(queryErr).Warn("error listing events")
			return nil, queryErr
		}
		for _, item := range out.Items {
			if id := item["event_id"]; id != nil && id.S != nil {
				ids = append(ids, *id.S)
			}
		}
		if len(out.LastEvaluatedKey) == 0 {
			return ids, nil
		}
		input.ExclusiveStartKey = out.LastEvaluatedKey
	}
}

// RekeyEventCompanySFID moves one event from oldSFID to newSFID: event_company_sfid and the
// company_sfid_* composite keys that still start with oldSFID are rewritten with a conditional
// update. An event already carrying newSFID is left alone (false, nil); any other company SFID is
// an ErrEventCompanySFIDMismatch.
func (repo *repository) RekeyEventCompanySFID(ctx context.Context, eventID, oldSFID, newSFID string) (bool, error) {
	f := logrus.Fields{
		"functionName":   "v1.events.repository.RekeyEventCompanySFID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"eventID":        eventID,
		"oldSFID":        oldSFID,
		"newSFID":        newSFID,
	}
	if strings.TrimSpace(eventID) == "" || strings.TrimSpace(oldSFID) == "" || strings.TrimSpace(newSFID) == "" || oldSFID == newSFID {
		return false, errors.New("event id and distinct old/new company sfids are required")
	}
	key := map[string]*dynamodb.AttributeValue{"event_id": {S: aws.String(eventID)}}
	out, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName:      aws.String(repo.eventsTable),
		Key:            key,
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error reading event")
		return false, err
	}
	if len(out.Item) == 0 {
		return false, ErrEventNotFound
	}

	names := map[string]*string{}
	values := map[string]*dynamodb.AttributeValue{}
	var sets, conditions []string
	rewrite := func(attribute, current, updated string) {
		placeholder := fmt.Sprintf("#a%d", len(sets))
		names[placeholder] = aws.String(attribute)
		values[":new"+placeholder[1:]] = &dynamodb.AttributeValue{S: aws.String(updated)}
		values[":cur"+placeholder[1:]] = &dynamodb.AttributeValue{S: aws.String(current)}
		sets = append(sets, fmt.Sprintf("%s = :new%s", placeholder, placeholder[1:]))
		conditions = append(conditions, fmt.Sprintf("%s = :cur%s", placeholder, placeholder[1:]))
	}
	if current := stringAttribute(out.Item, "event_company_sfid"); current != "" && current != newSFID {
		if current != oldSFID {
			return false, ErrEventCompanySFIDMismatch
		}
		rewrite("event_company_sfid", current, newSFID)
	}
	for _, attribute := range companySFIDCompositeAttributes {
		current := stringAttribute(out.Item, attribute)
		if current == "" || strings.HasPrefix(current, newSFID+"#") {
			continue
		}
		if !strings.HasPrefix(current, oldSFID+"#") {
			return false, ErrEventCompanySFIDMismatch
		}
		rewrite(attribute, current, newSFID+strings.TrimPrefix(current, oldSFID))
	}
	if len(sets) == 0 {
		return false, nil
	}

	_, err = repo.dynamoDBClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:                 aws.String(repo.eventsTable),
		Key:                       key,
		UpdateExpression:          aws.String("SET " + strings.Join(sets, ", ")),
		ConditionExpression:       aws.String(strings.Join(conditions, " AND ")),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
			return false, ErrEventRekeyConflict
		}
		log.WithFields(f).WithError(err).Warn("error rekeying event")
		return false, err
	}
	log.WithFields(f).Debugf("event rekeyed (%d attributes)", len(sets))
	return true, nil
}

func stringAttribute(item map[string]*dynamodb.AttributeValue, name string) string {
	if attr := item[name]; attr != nil && attr.S != nil {
		return *attr.S
	}
	return ""
}
