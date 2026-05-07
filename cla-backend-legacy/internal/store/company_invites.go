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

// CompanyInvitesStore writes CompanyInvite records.
//
// Table: cla-${STAGE}-company-invites
// Hash key: company_invite_id
//
// This is used by legacy Python cla.controllers.user.invite_cla_manager.
// We only implement PutItem (create) for migration parity.
type CompanyInvitesStore struct {
	client *dynamodb.Client
	table  string
}

func NewCompanyInvitesStoreFromEnv(ctx context.Context) (*CompanyInvitesStore, error) {
	client, err := NewDynamoDBClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &CompanyInvitesStore{client: client, table: TableName("company-invites")}, nil
}

func (s *CompanyInvitesStore) PutItem(ctx context.Context, item map[string]types.AttributeValue) error {
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
