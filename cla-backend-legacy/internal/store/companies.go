// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// CompaniesStore provides access patterns required by legacy endpoints.
//
// Table: cla-${STAGE}-companies
// Hash key: company_id
// GSIs:
//   - company-name-index (hash key company_name)
//   - signing-entity-name-index (hash key signing_entity_name)
//   - external-company-index (hash key company_external_id)
//
// Note: The legacy Python service primarily uses Scan() and then sorts client-side.
// We keep the same behavior for parity/minimality.
type CompaniesStore struct {
	client *dynamodb.Client
	table  string
}

func NewCompaniesStoreFromEnv(ctx context.Context) (*CompaniesStore, error) {
	client, err := NewDynamoDBClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &CompaniesStore{client: client, table: TableName("companies")}, nil
}

func (s *CompaniesStore) GetByID(ctx context.Context, companyID string) (map[string]types.AttributeValue, bool, error) {
	if s == nil || s.client == nil {
		return nil, false, nil
	}
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"company_id": &types.AttributeValueMemberS{Value: companyID},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, false, err
	}
	if out.Item == nil {
		return nil, false, nil
	}
	return out.Item, true, nil
}

func (s *CompaniesStore) QueryByName(ctx context.Context, companyName string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}

	items := make([]map[string]types.AttributeValue, 0, 1)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("company-name-index"),
			KeyConditionExpression: aws.String("company_name = :n"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":n": &types.AttributeValueMemberS{Value: companyName},
			},
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, out.Items...)
		if len(out.LastEvaluatedKey) == 0 {
			break
		}
		startKey = out.LastEvaluatedKey
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items, nil
}

func (s *CompaniesStore) ScanAll(ctx context.Context) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}

	items := make([]map[string]types.AttributeValue, 0, 128)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(s.table),
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, out.Items...)
		if len(out.LastEvaluatedKey) == 0 {
			break
		}
		startKey = out.LastEvaluatedKey
	}
	return items, nil
}

func (s *CompaniesStore) PutItem(ctx context.Context, item map[string]types.AttributeValue) error {
	if s == nil || s.client == nil {
		return nil
	}
	if item == nil {
		return fmt.Errorf("nil item")
	}
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      item,
	})
	return err
}

func (s *CompaniesStore) DeleteByID(ctx context.Context, companyID string) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"company_id": &types.AttributeValueMemberS{Value: companyID},
		},
	})
	return err
}

// sanctionUpdate is the DynamoDB update for one sanction status change.
type sanctionUpdate struct {
	expression string
	condition  *string
	names      map[string]string
	values     map[string]types.AttributeValue
}

// buildSanctionUpdate assembles the update for UpdateCompanySanctionStatus. All SET assignments
// stay contiguous ahead of any REMOVE, as DynamoDB requires.
func buildSanctionUpdate(sanctioned bool, origin, now string) sanctionUpdate {
	update := sanctionUpdate{
		expression: "SET #S = :s, #M = :m",
		names: map[string]string{
			"#S": "is_sanctioned",
			"#M": "date_modified",
			"#O": "sanction_origin",
		},
		values: map[string]types.AttributeValue{
			":s": &types.AttributeValueMemberBOOL{Value: sanctioned},
			":m": &types.AttributeValueMemberS{Value: now},
		},
	}

	// Setting the flag stamps sanctioned_date; clearing it leaves the stored date alone.
	if sanctioned {
		update.names["#D"] = "sanctioned_date"
		update.values[":d"] = &types.AttributeValueMemberS{Value: now}
		update.expression += ", #D = :d"
	}

	if origin != "" {
		update.values[":o"] = &types.AttributeValueMemberS{Value: origin}
		update.expression += ", #O = :o"
	} else {
		// Manual/admin update: remove any stale SSS-set origin so the record becomes a
		// sticky admin block (origin absent) that SSS will never auto-clear.
		update.expression += " REMOVE #O"
	}

	// When SSS sets a block, never overwrite a manual/admin block (is_sanctioned=true
	// with absent or non-"sss" origin). Only set the SSS flag when the company is
	// currently unblocked or already SSS-blocked; a ConditionalCheckFailedException
	// means a manual/admin block is already present and must be preserved.
	if sanctioned && origin == "sss" {
		update.values[":false"] = &types.AttributeValueMemberBOOL{Value: false}
		update.condition = aws.String("attribute_not_exists(#S) OR #S = :false OR #O = :o")
	}

	return update
}

// UpdateCompanySanctionStatus sets is_sanctioned and, when origin is non-empty, sanction_origin.
// Pass origin="sss" when flagging via SSS; pass origin="" for manual admin updates.
func (s *CompaniesStore) UpdateCompanySanctionStatus(ctx context.Context, companyID string, sanctioned bool, origin string) error {
	if s == nil || s.client == nil {
		return nil
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000-0700") // Best effort for date_modified parity
	update := buildSanctionUpdate(sanctioned, origin, now)

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"company_id": &types.AttributeValueMemberS{Value: companyID},
		},
		UpdateExpression:          aws.String(update.expression),
		ConditionExpression:       update.condition,
		ExpressionAttributeNames:  update.names,
		ExpressionAttributeValues: update.values,
	}

	_, err := s.client.UpdateItem(ctx, input)
	if err != nil {
		if update.condition != nil {
			var condErr *types.ConditionalCheckFailedException
			if errors.As(err, &condErr) {
				return nil // Preserve the existing manual/admin block
			}
		}
		return err
	}
	return nil
}

// ClearCompanySanctionStatusIfSSS clears is_sanctioned only when sanction_origin="sss".
func (s *CompaniesStore) ClearCompanySanctionStatusIfSSS(ctx context.Context, companyID string) (bool, error) {
	if s == nil || s.client == nil {
		return false, nil
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000-0700")

	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"company_id": &types.AttributeValueMemberS{Value: companyID},
		},
		UpdateExpression:    aws.String("SET #S = :false, #M = :m REMOVE #O"),
		ConditionExpression: aws.String("#O = :sss"),
		ExpressionAttributeNames: map[string]string{
			"#S": "is_sanctioned",
			"#M": "date_modified",
			"#O": "sanction_origin",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":false": &types.AttributeValueMemberBOOL{Value: false},
			":m":     &types.AttributeValueMemberS{Value: now},
			":sss":   &types.AttributeValueMemberS{Value: "sss"},
		},
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return false, nil // Already manual/admin or not SSS-flagged
		}
		return false, err
	}
	return true, nil
}
