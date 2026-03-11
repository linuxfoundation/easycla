package store

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// GitLabOrgsStore provides read access to the legacy gitlab-orgs table.
//
// Python: cla/models/gitlab_org.py -> GitlabOrgModel / GitlabOrg
// Table: cla-{stage}-gitlab-orgs
//
// The legacy Python implementation commonly scans the full table when it needs
// to map a GitLab group URL to an internal organization_id.
//
// Minimal-effort port: we do a Scan with a filter on organization_url.
// (This is still a Scan under the hood, same class of operation as Python.)
//
// NOTE: If you later want to optimize, consider adding and using a dedicated
// GSI for organization_url.
type GitLabOrgsStore struct {
	client *dynamodb.Client
	table  string
}

func NewGitLabOrgsStoreFromEnv(ctx context.Context) (*GitLabOrgsStore, error) {
	cli, err := NewDynamoDBClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &GitLabOrgsStore{client: cli, table: TableName("gitlab-orgs")}, nil
}

func (s *GitLabOrgsStore) ScanAll(ctx context.Context) ([]map[string]types.AttributeValue, error) {
	var out []map[string]types.AttributeValue
	var startKey map[string]types.AttributeValue
	for {
		resp, err := s.client.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(s.table),
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, resp.Items...)
		if len(resp.LastEvaluatedKey) == 0 {
			break
		}
		startKey = resp.LastEvaluatedKey
	}
	return out, nil
}

// FindByOrganizationURL returns the first GitLab org record with organization_url == orgURL.
func (s *GitLabOrgsStore) FindByOrganizationURL(ctx context.Context, orgURL string) (map[string]types.AttributeValue, bool, error) {
	orgURL = strings.TrimSpace(orgURL)
	if orgURL == "" {
		return nil, false, nil
	}

	// Minimal-effort parity with Python's scan().
	var startKey map[string]types.AttributeValue
	for {
		resp, err := s.client.Scan(ctx, &dynamodb.ScanInput{
			TableName:        aws.String(s.table),
			FilterExpression: aws.String("organization_url = :u"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":u": &types.AttributeValueMemberS{Value: orgURL},
			},
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, false, err
		}
		if len(resp.Items) > 0 {
			return resp.Items[0], true, nil
		}
		if len(resp.LastEvaluatedKey) == 0 {
			break
		}
		startKey = resp.LastEvaluatedKey
	}
	return nil, false, nil
}
