// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// KVStore is a minimal port of the legacy Python key_value_store_service.
//
// Table: cla-${STAGE}-store
// Hash key: key (string)
// Attributes: value (string), expire (number, epoch seconds)
type KVStore struct {
	client *dynamodb.Client
	table  string
}

func NewKVStoreFromEnv(ctx context.Context) (*KVStore, error) {
	client, err := NewDynamoDBClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &KVStore{client: client, table: TableName("store")}, nil
}

func (s *KVStore) Get(ctx context.Context, key string) (string, bool, error) {
	if s == nil || s.client == nil {
		return "", false, nil
	}

	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"key": &types.AttributeValueMemberS{Value: key},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return "", false, err
	}
	if out.Item == nil {
		return "", false, nil
	}

	av, ok := out.Item["value"].(*types.AttributeValueMemberS)
	if !ok {
		// Key exists but value unset.
		return "", true, nil
	}
	return av.Value, true, nil
}

func (s *KVStore) Exists(ctx context.Context, key string) (bool, error) {
	_, ok, err := s.Get(ctx, key)
	return ok, err
}

func (s *KVStore) SetWithTTL(ctx context.Context, key string, value string, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return nil
	}
	expire := time.Now().Add(ttl).Unix()

	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item: map[string]types.AttributeValue{
			"key":    &types.AttributeValueMemberS{Value: key},
			"value":  &types.AttributeValueMemberS{Value: value},
			"expire": &types.AttributeValueMemberN{Value: strconv.FormatInt(expire, 10)},
		},
	})
	return err
}

// Set stores a value with the legacy default TTL of 45 minutes.
func (s *KVStore) Set(ctx context.Context, key string, value string) error {
	return s.SetWithTTL(ctx, key, value, 45*time.Minute)
}

func (s *KVStore) Delete(ctx context.Context, key string) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"key": &types.AttributeValueMemberS{Value: key},
		},
	})
	return err
}
