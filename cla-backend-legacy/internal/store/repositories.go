package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// RepositoriesStore supports lookups and CRUD for repository records.
//
// Table: cla-${STAGE}-repositories
// Hash key: repository_id
// GSIs:
//   - external-repository-index (hash key repository_external_id)
//   - repository-project-id-index (hash key repository_project_id)
//   - repository-sfdc-id-index (hash key repository_sfdc_id)
//
// The legacy Python code queries external-repository-index and then filters by
// repository_type in memory.
type RepositoriesStore struct {
	client *dynamodb.Client
	table  string
}

func NewRepositoriesStoreFromEnv(ctx context.Context) (*RepositoriesStore, error) {
	client, err := NewDynamoDBClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &RepositoriesStore{client: client, table: TableName("repositories")}, nil
}

func (s *RepositoriesStore) GetByID(ctx context.Context, repositoryID string) (map[string]types.AttributeValue, bool, error) {
	if s == nil || s.client == nil {
		return nil, false, nil
	}
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"repository_id": &types.AttributeValueMemberS{Value: repositoryID},
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

func (s *RepositoriesStore) PutItem(ctx context.Context, item map[string]types.AttributeValue) error {
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

func (s *RepositoriesStore) DeleteByID(ctx context.Context, repositoryID string) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"repository_id": &types.AttributeValueMemberS{Value: repositoryID},
		},
	})
	return err
}

func (s *RepositoriesStore) GetByExternalIDAndType(ctx context.Context, externalID string, repositoryType string) (map[string]types.AttributeValue, bool, error) {
	if s == nil || s.client == nil {
		return nil, false, nil
	}

	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("external-repository-index"),
			KeyConditionExpression: aws.String("repository_external_id = :eid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":eid": &types.AttributeValueMemberS{Value: externalID},
			},
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, false, err
		}

		for _, it := range out.Items {
			// Filter by repository_type (Python does this after index query).
			if repositoryType == "" {
				return it, true, nil
			}
			if av, ok := it["repository_type"].(*types.AttributeValueMemberS); ok {
				if strings.EqualFold(av.Value, repositoryType) {
					return it, true, nil
				}
			}
		}

		if len(out.LastEvaluatedKey) == 0 {
			break
		}
		startKey = out.LastEvaluatedKey
	}

	return nil, false, nil
}

// QueryByProjectID returns all repositories linked to a CLA group (project_id UUID) via repository_project_id.
// Legacy Python: Repository().get_repository_by_project_id -> RepositoryModel.repository_project_index.query(project_id)
func (s *RepositoriesStore) QueryByProjectID(ctx context.Context, projectID string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	items := make([]map[string]types.AttributeValue, 0)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("project-repository-index"),
			KeyConditionExpression: aws.String("repository_project_id = :pid"),
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

// QueryByOrganizationName returns all repositories under a given organization.
// Legacy Python: Repository().get_repositories_by_organization -> RepositoryModel.repository_org_index.query(organization_name)
func (s *RepositoriesStore) QueryByOrganizationName(ctx context.Context, organizationName string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	items := make([]map[string]types.AttributeValue, 0)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("repository-organization-name-index"),
			KeyConditionExpression: aws.String("repository_organization_name = :org"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":org": &types.AttributeValueMemberS{Value: organizationName},
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

// QueryBySFDCID returns all repositories keyed by repository_sfdc_id (legacy helper: get_repository_by_sfdc_id).
func (s *RepositoriesStore) QueryBySFDCID(ctx context.Context, repositorySFDCID string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	items := make([]map[string]types.AttributeValue, 0)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("sfdc-repository-index"),
			KeyConditionExpression: aws.String("repository_sfdc_id = :sid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":sid": &types.AttributeValueMemberS{Value: repositorySFDCID},
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

// QueryByProjectSFID returns all repositories keyed by project_sfid (used by v2 get_project mapping).
func (s *RepositoriesStore) QueryByProjectSFID(ctx context.Context, projectSFID string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	items := make([]map[string]types.AttributeValue, 0)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("project-sfid-repository-index"),
			KeyConditionExpression: aws.String("project_sfid = :psfid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":psfid": &types.AttributeValueMemberS{Value: projectSFID},
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
