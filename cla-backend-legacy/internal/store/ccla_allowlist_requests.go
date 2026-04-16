// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// CCLAAllowlistRequestsStore writes CCLA allowlist (whitelist) request records.
//
// Table: cla-${STAGE}-ccla-whitelist-requests
// Hash key: request_id
//
// This is used by legacy Python cla.controllers.user.invite_cla_manager and
// cla.controllers.user.request_company_ccla.
type CCLAAllowlistRequestsStore struct {
	client *dynamodb.Client
	table  string
}

func NewCCLAAllowlistRequestsStoreFromEnv(ctx context.Context) (*CCLAAllowlistRequestsStore, error) {
	client, err := NewDynamoDBClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &CCLAAllowlistRequestsStore{client: client, table: TableName("ccla-whitelist-requests")}, nil
}

func (s *CCLAAllowlistRequestsStore) PutItem(ctx context.Context, item map[string]types.AttributeValue) error {
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
