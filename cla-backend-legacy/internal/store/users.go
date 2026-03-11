package store

import (
	"context"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// UsersStore provides minimal access patterns required by legacy v1/v2 endpoints.
//
// Table: cla-${STAGE}-users
// Hash key: user_id
// GSIs:
//   - lf-username-index (hash key lf_username)
//   - lf-email-index (hash key lf_email)
//   - github-id-index (hash key user_github_id)
//   - github-username-index (hash key user_github_username)
type UsersStore struct {
	client *dynamodb.Client
	table  string
}

func NewUsersStoreFromEnv(ctx context.Context) (*UsersStore, error) {
	client, err := NewDynamoDBClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &UsersStore{client: client, table: TableName("users")}, nil
}

func (s *UsersStore) GetByID(ctx context.Context, userID string) (map[string]types.AttributeValue, bool, error) {
	if s == nil || s.client == nil {
		return nil, false, nil
	}
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
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

func (s *UsersStore) QueryByLFUsername(ctx context.Context, username string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}

	items := make([]map[string]types.AttributeValue, 0, 1)
	var startKey map[string]types.AttributeValue

	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("lf-username-index"),
			KeyConditionExpression: aws.String("lf_username = :u"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":u": &types.AttributeValueMemberS{Value: username},
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

func (s *UsersStore) QueryByLFEmail(ctx context.Context, email string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, nil
	}

	items := make([]map[string]types.AttributeValue, 0, 1)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("lf-email-index"),
			KeyConditionExpression: aws.String("lf_email = :e"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":e": &types.AttributeValueMemberS{Value: email},
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

// QueryByGitHubID queries the github-id-index (hash key user_github_id).
func (s *UsersStore) QueryByGitHubID(ctx context.Context, githubID int64) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	if githubID <= 0 {
		return nil, nil
	}

	items := make([]map[string]types.AttributeValue, 0, 1)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("github-id-index"),
			KeyConditionExpression: aws.String("user_github_id = :gid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":gid": &types.AttributeValueMemberN{Value: strconv.FormatInt(githubID, 10)},
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

// QueryByGitHubUsername queries the github-username-index (hash key user_github_username).
//
// Legacy Python uses UserModel.user_github_username_index.query(username) when handling bot allowlist logic.
func (s *UsersStore) QueryByGitHubUsername(ctx context.Context, username string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, nil
	}

	items := make([]map[string]types.AttributeValue, 0, 1)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("github-username-index"),
			KeyConditionExpression: aws.String("user_github_username = :name"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":name": &types.AttributeValueMemberS{Value: username},
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

// ScanByUserEmailsContains scans for a user where user_emails contains the given email.
// Legacy Python falls back to a scan when no indexed lookup is available.
func (s *UsersStore) ScanByUserEmailsContains(ctx context.Context, email string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, nil
	}

	items := make([]map[string]types.AttributeValue, 0, 1)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Scan(ctx, &dynamodb.ScanInput{
			TableName:        aws.String(s.table),
			FilterExpression: aws.String("contains(user_emails, :e)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":e": &types.AttributeValueMemberS{Value: email},
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

func (s *UsersStore) QueryByCompanyID(ctx context.Context, companyID string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}

	// Scan for users with this company ID - no index available for user_company_id
	out, err := s.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(s.table),
		FilterExpression: aws.String("user_company_id = :companyID"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":companyID": &types.AttributeValueMemberS{Value: companyID},
		},
	})
	if err != nil {
		return nil, err
	}
	return out.Items, nil
}

func (s *UsersStore) PutItem(ctx context.Context, item map[string]types.AttributeValue) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      item,
	})
	return err
}
