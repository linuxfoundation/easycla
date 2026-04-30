// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var ErrNotFound = errors.New("User Permissions not found")

// UserPermissions matches the legacy DynamoDB record used by the Python backend.
//
// Table name: cla-${STAGE}-user-permissions
// Hash key:   username (S)
// Attributes:
//   - projects  (SS)
//   - companies (SS)
type UserPermissions struct {
	Username  string   `dynamodbav:"username" json:"username"`
	Projects  []string `dynamodbav:"projects" json:"projects"`
	Companies []string `dynamodbav:"companies" json:"companies"`
}

type UserPermissionsStore struct {
	ddb    *dynamodb.Client
	table  string
	stage  string
	region string
}

func NewUserPermissionsStoreFromEnv(ctx context.Context) (*UserPermissionsStore, error) {
	stage := strings.TrimSpace(os.Getenv("STAGE"))
	if stage == "" {
		stage = "dev"
	}

	region := strings.TrimSpace(os.Getenv("DYNAMODB_AWS_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_REGION"))
	}
	if region == "" {
		region = strings.TrimSpace(os.Getenv("REGION"))
	}
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return &UserPermissionsStore{
		ddb:    dynamodb.NewFromConfig(cfg),
		table:  fmt.Sprintf("cla-%s-user-permissions", stage),
		stage:  stage,
		region: region,
	}, nil
}

func (s *UserPermissionsStore) Get(ctx context.Context, username string) (*UserPermissions, error) {
	if s == nil || s.ddb == nil {
		return nil, errors.New("user permissions store not configured")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("username is required")
	}

	key, err := attributevalue.MarshalMap(map[string]string{"username": username})
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}

	out, err := s.ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key:       key,
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb get item: %w", err)
	}
	if len(out.Item) == 0 {
		return nil, ErrNotFound
	}

	var up UserPermissions
	if err := attributevalue.UnmarshalMap(out.Item, &up); err != nil {
		return nil, fmt.Errorf("unmarshal user permissions: %w", err)
	}
	return &up, nil
}

// formatPynamoDateTimeUTC formats timestamps like Python's datetime.utcnow().isoformat().
//
// Python stores naive UTC datetimes (no timezone suffix). It only includes fractional
// seconds when microseconds are non-zero.
func formatPynamoDateTimeUTC(t time.Time) string {
	s := t.UTC().Format("2006-01-02T15:04:05.000000")
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// AddProject adds (or creates) a user-permissions record with the given project SFID.
//
// Mirrors legacy Python behavior in cla.controllers.project.add_permission():
//   - If the record doesn't exist, create it.
//   - Add the project SFID to the projects set.
//   - Touch date_modified; set date_created/version if not already present.
func (s *UserPermissionsStore) AddProject(ctx context.Context, username, projectSFID string) error {
	if s == nil || s.ddb == nil {
		return errors.New("user permissions store not configured")
	}
	username = strings.TrimSpace(username)
	projectSFID = strings.TrimSpace(projectSFID)
	if username == "" {
		return errors.New("username is required")
	}
	if projectSFID == "" {
		return errors.New("project_sfid is required")
	}

	key, err := attributevalue.MarshalMap(map[string]string{"username": username})
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}

	now := formatPynamoDateTimeUTC(time.Now().UTC())
	_, err = s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.table),
		Key:       key,
		UpdateExpression: aws.String(
			"SET date_created = if_not_exists(date_created, :dc), " +
				"version = if_not_exists(version, :ver), " +
				"date_modified = :dm " +
				"ADD projects :p",
		),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":dc":  &types.AttributeValueMemberS{Value: now},
			":dm":  &types.AttributeValueMemberS{Value: now},
			":ver": &types.AttributeValueMemberS{Value: "v1"},
			":p":   &types.AttributeValueMemberSS{Value: []string{projectSFID}},
		},
	})
	if err != nil {
		return fmt.Errorf("dynamodb update item: %w", err)
	}
	return nil
}

// RemoveProject removes a project SFID from the user's projects set.
//
// Mirrors legacy Python behavior in cla.controllers.project.remove_permission():
//   - If the record doesn't exist, return ErrNotFound.
//   - Otherwise remove the SFID from the projects set and touch date_modified.
func (s *UserPermissionsStore) RemoveProject(ctx context.Context, username, projectSFID string) error {
	if s == nil || s.ddb == nil {
		return errors.New("user permissions store not configured")
	}
	username = strings.TrimSpace(username)
	projectSFID = strings.TrimSpace(projectSFID)
	if username == "" {
		return errors.New("username is required")
	}
	if projectSFID == "" {
		return errors.New("project_sfid is required")
	}

	// Ensure record exists (Python returns an error when load() fails).
	if _, err := s.Get(ctx, username); err != nil {
		return err
	}

	key, err := attributevalue.MarshalMap(map[string]string{"username": username})
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}

	now := formatPynamoDateTimeUTC(time.Now().UTC())
	_, err = s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.table),
		Key:       key,
		UpdateExpression: aws.String(
			"SET date_modified = :dm " +
				"DELETE projects :p",
		),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":dm": &types.AttributeValueMemberS{Value: now},
			":p":  &types.AttributeValueMemberSS{Value: []string{projectSFID}},
		},
	})
	if err != nil {
		return fmt.Errorf("dynamodb update item: %w", err)
	}
	return nil
}
