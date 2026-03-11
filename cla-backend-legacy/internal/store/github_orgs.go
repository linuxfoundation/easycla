// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// GitHubOrgsStore supports lookups of GitHub organization records.
//
// Table: cla-${STAGE}-github-orgs
// Hash key: organization_name
// GSIs:
//   - organization-name-lower-search-index (hash key organization_name_lower)
//   - github-org-sfid-index (hash key organization_sfid)
//   - project-sfid-organization-name-index (hash key project_sfid)

type GitHubOrgsStore struct {
	client *dynamodb.Client
	table  string
}

func NewGitHubOrgsStoreFromEnv(ctx context.Context) (*GitHubOrgsStore, error) {
	client, err := NewDynamoDBClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &GitHubOrgsStore{client: client, table: TableName("github-orgs")}, nil
}

func (s *GitHubOrgsStore) GetByName(ctx context.Context, name string) (map[string]types.AttributeValue, bool, error) {
	if s == nil || s.client == nil {
		return nil, false, nil
	}
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"organization_name": &types.AttributeValueMemberS{Value: name},
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

func (s *GitHubOrgsStore) GetByLowerName(ctx context.Context, lowerName string) (map[string]types.AttributeValue, bool, error) {
	if s == nil || s.client == nil {
		return nil, false, nil
	}
	out, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.table),
		IndexName:              aws.String("organization-name-lower-search-index"),
		KeyConditionExpression: aws.String("organization_name_lower = :n"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":n": &types.AttributeValueMemberS{Value: lowerName},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, false, err
	}
	if len(out.Items) == 0 {
		return nil, false, nil
	}
	return out.Items[0], true, nil
}

func (s *GitHubOrgsStore) PutItem(ctx context.Context, item map[string]types.AttributeValue) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      item,
	})
	return err
}

func (s *GitHubOrgsStore) DeleteByName(ctx context.Context, name string) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"organization_name": &types.AttributeValueMemberS{Value: name},
		},
	})
	return err
}

func (s *GitHubOrgsStore) ScanAll(ctx context.Context) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	items := make([]map[string]types.AttributeValue, 0)
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

func (s *GitHubOrgsStore) QueryBySFID(ctx context.Context, sfid string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	items := make([]map[string]types.AttributeValue, 0)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("github-org-sfid-index"),
			KeyConditionExpression: aws.String("organization_sfid = :s"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":s": &types.AttributeValueMemberS{Value: sfid},
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
