// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ProjectsStore provides minimal access patterns required by legacy endpoints.
//
// Table: cla-${STAGE}-projects
// Hash key: project_id
// GSI: external-project-index (hash key project_external_id)
type ProjectsStore struct {
	client *dynamodb.Client
	table  string
}

func NewProjectsStoreFromEnv(ctx context.Context) (*ProjectsStore, error) {
	client, err := NewDynamoDBClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &ProjectsStore{client: client, table: TableName("projects")}, nil
}

func (s *ProjectsStore) GetByID(ctx context.Context, projectID string) (map[string]types.AttributeValue, bool, error) {
	if s == nil || s.client == nil {
		return nil, false, nil
	}
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"project_id": &types.AttributeValueMemberS{Value: projectID},
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

func (s *ProjectsStore) QueryByExternalID(ctx context.Context, externalID string) ([]map[string]types.AttributeValue, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}

	items := make([]map[string]types.AttributeValue, 0, 4)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("external-project-index"),
			KeyConditionExpression: aws.String("project_external_id = :eid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":eid": &types.AttributeValueMemberS{Value: externalID},
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

func (s *ProjectsStore) QueryByNameLower(ctx context.Context, projectName string) ([]map[string]types.AttributeValue, error) {
	// Python: ProjectModel.project_name_lower_search_index.query(project_name.lower())
	if s == nil || s.client == nil {
		return nil, nil
	}
	name := strings.TrimSpace(strings.ToLower(projectName))
	if name == "" {
		return nil, nil
	}

	items := make([]map[string]types.AttributeValue, 0, 4)
	var startKey map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("project-name-lower-search-index"),
			KeyConditionExpression: aws.String("project_name_lower = :n"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":n": &types.AttributeValueMemberS{Value: name},
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

func (s *ProjectsStore) ScanAll(ctx context.Context) ([]map[string]types.AttributeValue, error) {
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

func parseIntAttr(av types.AttributeValue) int {
	switch v := av.(type) {
	case *types.AttributeValueMemberN:
		i, err := strconv.Atoi(v.Value)
		if err == nil {
			return i
		}
	case *types.AttributeValueMemberS:
		i, err := strconv.Atoi(v.Value)
		if err == nil {
			return i
		}
	}
	return 0
}

func parsePynamoDateTimeString(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	// Try layouts used by legacy Python/pynamodb.
	// pynamodb's canonical UTCDateTimeAttribute format uses a "+0000" suffix
	// (no colon), so the no-colon layouts must come before the colon ones.
	layouts := []string{
		"2006-01-02T15:04:05.999999-0700",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05.99999",
		"2006-01-02T15:04:05.9999",
		"2006-01-02T15:04:05.999",
		"2006-01-02T15:04:05",
		// Some records may include colon-style timezone offsets.
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999-07:00",
		"2006-01-02T15:04:05-07:00",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parsePynamoDateTimeAttr(av types.AttributeValue) (time.Time, bool) {
	if av == nil {
		return time.Time{}, false
	}
	if s, ok := av.(*types.AttributeValueMemberS); ok {
		return parsePynamoDateTimeString(s.Value)
	}
	return time.Time{}, false
}

func latestDocVersionFromProjectDocs(docsAV types.AttributeValue) (major int, minor int, ok bool) {
	list, okList := docsAV.(*types.AttributeValueMemberL)
	if !okList {
		return 0, -1, false
	}

	lastMajor := 0
	lastMinor := -1
	var lastDate time.Time
	hasDate := false

	for _, el := range list.Value {
		m, okM := el.(*types.AttributeValueMemberM)
		if !okM {
			continue
		}
		curMajor := parseIntAttr(m.Value["document_major_version"])
		curMinor := parseIntAttr(m.Value["document_minor_version"])
		curDate, curHasDate := parsePynamoDateTimeAttr(m.Value["document_creation_date"])

		if curMajor > lastMajor || (curMajor == lastMajor && curMinor > lastMinor) {
			lastMajor = curMajor
			lastMinor = curMinor
			if curHasDate {
				lastDate = curDate
				hasDate = true
			} else {
				hasDate = false
			}
			continue
		}

		// Tie-breaker when major/minor are equal: pick the latest creation_date.
		if curMajor == lastMajor && curMinor == lastMinor {
			if hasDate && curHasDate {
				if curDate.After(lastDate) {
					lastDate = curDate
				}
			} else if !hasDate && curHasDate {
				lastDate = curDate
				hasDate = true
			}
		}
	}

	// If no documents were present, Python returns (0,-1) which later causes a None deref.
	// We surface this explicitly as ok=false.
	if lastMajor == 0 && lastMinor == -1 {
		return lastMajor, lastMinor, false
	}
	return lastMajor, lastMinor, true
}

func (s *ProjectsStore) LatestIndividualDocumentVersion(ctx context.Context, projectID string) (int, int, error) {
	item, found, err := s.GetByID(ctx, projectID)
	if err != nil {
		return 0, 0, err
	}
	if !found {
		return 0, 0, errors.New("Project not found")
	}
	docs, ok := item["project_individual_documents"]
	if !ok {
		return 0, 0, errors.New("No individual document exists for this project")
	}
	maj, min, ok2 := latestDocVersionFromProjectDocs(docs)
	if !ok2 {
		return 0, 0, errors.New("No individual document exists for this project")
	}
	return maj, min, nil
}

func (s *ProjectsStore) LatestCorporateDocumentVersion(ctx context.Context, projectID string) (int, int, error) {
	item, found, err := s.GetByID(ctx, projectID)
	if err != nil {
		return 0, 0, err
	}
	if !found {
		return 0, 0, errors.New("Project not found")
	}
	docs, ok := item["project_corporate_documents"]
	if !ok {
		return 0, 0, errors.New("No corporate document exists for this project")
	}
	maj, min, ok2 := latestDocVersionFromProjectDocs(docs)
	if !ok2 {
		return 0, 0, errors.New("No corporate document exists for this project")
	}
	return maj, min, nil
}

func (s *ProjectsStore) PutItem(ctx context.Context, item map[string]types.AttributeValue) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      item,
	})
	return err
}

func (s *ProjectsStore) DeleteByID(ctx context.Context, projectID string) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"project_id": &types.AttributeValueMemberS{Value: projectID},
		},
	})
	return err
}
