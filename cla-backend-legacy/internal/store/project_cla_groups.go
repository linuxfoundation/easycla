// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ProjectCLAGroupsStore wraps the projects-cla-groups table.
//
// Legacy Python: ProjectCLAGroupModel (cla-${stage}-projects-cla-groups)
// GSIs:
//   - cla-group-id-index (hash key cla_group_id)
//   - foundation-sfid-index (hash key foundation_sfid)
//
// Primary key is project_sfid.
type ProjectCLAGroupsStore struct {
	client *dynamodb.Client
	table  string
}

func NewProjectCLAGroupsStoreFromEnv(ctx context.Context) (*ProjectCLAGroupsStore, error) {
	client, err := NewDynamoDBClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &ProjectCLAGroupsStore{client: client, table: TableNameFromSuffix("projects-cla-groups")}, nil
}

func (s *ProjectCLAGroupsStore) QueryByCLAGroupID(ctx context.Context, claGroupID string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	items := make([]map[string]types.AttributeValue, 0)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("cla-group-id-index"),
			KeyConditionExpression: aws.String("cla_group_id = :cid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":cid": &types.AttributeValueMemberS{Value: claGroupID},
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

// QueryByFoundationSFID returns all project CLA group mappings for a given foundation_sfid.
//
// Legacy Python: ProjectCLAGroup.get_by_foundation_sfid()
// GSI: foundation-sfid-index (hash key foundation_sfid)
func (s *ProjectCLAGroupsStore) QueryByFoundationSFID(ctx context.Context, foundationSFID string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	items := make([]map[string]types.AttributeValue, 0)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("foundation-sfid-index"),
			KeyConditionExpression: aws.String("foundation_sfid = :fid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":fid": &types.AttributeValueMemberS{Value: foundationSFID},
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
