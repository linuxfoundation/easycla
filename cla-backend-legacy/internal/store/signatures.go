package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// SignaturesStore provides minimal access patterns for signature lookups.
//
// Table: cla-${STAGE}-signatures
// Hash key: signature_id
// GSI: signature-project-reference-index (hash key signature_project_id, range key signature_reference_id)
type SignaturesStore struct {
	client *dynamodb.Client
	table  string
}

func NewSignaturesStoreFromEnv(ctx context.Context) (*SignaturesStore, error) {
	client, err := NewDynamoDBClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &SignaturesStore{client: client, table: TableName("signatures")}, nil
}

func (s *SignaturesStore) GetByID(ctx context.Context, signatureID string) (map[string]types.AttributeValue, bool, error) {
	if s == nil || s.client == nil {
		return nil, false, nil
	}
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"signature_id": &types.AttributeValueMemberS{Value: signatureID},
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

func (s *SignaturesStore) QueryByProjectAndReference(ctx context.Context, projectID string, referenceID string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}

	items := make([]map[string]types.AttributeValue, 0, 4)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("signature-project-reference-index"),
			KeyConditionExpression: aws.String("signature_project_id = :pid AND signature_reference_id = :rid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pid": &types.AttributeValueMemberS{Value: projectID},
				":rid": &types.AttributeValueMemberS{Value: referenceID},
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

// QueryByProjectID returns signatures for a CLA group (project_id) using the project-signature-index.
func (s *SignaturesStore) QueryByProjectID(ctx context.Context, projectID string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}

	items := make([]map[string]types.AttributeValue, 0, 8)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("project-signature-index"),
			KeyConditionExpression: aws.String("signature_project_id = :pid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pid": &types.AttributeValueMemberS{Value: projectID},
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

// QueryByReferenceID returns signatures for a reference (user_id or company_id) using the reference-signature-index.
func (s *SignaturesStore) QueryByReferenceID(ctx context.Context, referenceID string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}

	items := make([]map[string]types.AttributeValue, 0, 8)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("reference-signature-index"),
			KeyConditionExpression: aws.String("signature_reference_id = :rid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":rid": &types.AttributeValueMemberS{Value: referenceID},
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

func (s *SignaturesStore) DeleteByID(ctx context.Context, signatureID string) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"signature_id": &types.AttributeValueMemberS{Value: signatureID},
		},
	})
	return err
}

func (s *SignaturesStore) PutItem(ctx context.Context, item map[string]types.AttributeValue) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      item,
	})
	return err
}
