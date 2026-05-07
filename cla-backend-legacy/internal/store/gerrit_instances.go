// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// GerritInstancesStore wraps the gerrit-instances table.
//
// Legacy Python: GerritModel (cla-${stage}-gerrit-instances)
// GSIs:
//   - gerrit-project-id-index (hash key project_id)
//   - gerrit-project-sfid-index (hash key project_sfid)
//
// Primary key is gerrit_id.
type GerritInstancesStore struct {
	client *dynamodb.Client
	table  string
}

func NewGerritInstancesStoreFromEnv(ctx context.Context) (*GerritInstancesStore, error) {
	client, err := NewDynamoDBClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &GerritInstancesStore{client: client, table: TableNameFromSuffix("gerrit-instances")}, nil
}

func (s *GerritInstancesStore) QueryByProjectSFID(ctx context.Context, projectSFID string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	items := make([]map[string]types.AttributeValue, 0)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("gerrit-project-sfid-index"),
			KeyConditionExpression: aws.String("project_sfid = :ps"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":ps": &types.AttributeValueMemberS{Value: projectSFID},
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
	return items, nil
}

func (s *GerritInstancesStore) QueryByProjectID(ctx context.Context, projectID string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	items := make([]map[string]types.AttributeValue, 0)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("gerrit-project-id-index"),
			KeyConditionExpression: aws.String("project_id = :pid"),
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
	return items, nil
}

func (s *GerritInstancesStore) GetByID(ctx context.Context, gerritID string) (map[string]types.AttributeValue, bool, error) {
	if s == nil || s.client == nil {
		return nil, false, nil
	}
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"gerrit_id": &types.AttributeValueMemberS{Value: gerritID},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, false, err
	}
	if len(out.Item) == 0 {
		return nil, false, nil
	}
	return out.Item, true, nil
}

func (s *GerritInstancesStore) PutItem(ctx context.Context, item map[string]types.AttributeValue) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      item,
	})
	return err
}

func (s *GerritInstancesStore) DeleteByID(ctx context.Context, gerritID string) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"gerrit_id": &types.AttributeValueMemberS{Value: gerritID},
		},
	})
	return err
}
