package store

import (
	"context"
	"fmt"

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
