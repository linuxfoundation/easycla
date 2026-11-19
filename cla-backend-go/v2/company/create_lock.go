// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/gofrs/uuid"
)

// ErrCreateLockHeld reports a create lock that another creation holds
var ErrCreateLockHeld = errors.New("company create lock is held")

// CreateLocks lets one company creation run per website domain at a time (lfx-self-serve#5129)
type CreateLocks interface {
	Acquire(ctx context.Context, domain string) (string, error)
	Release(ctx context.Context, domain, owner string) error
}

const (
	createLockKeyPrefix = "company_create:"
	// longer than the 300 second Lambda timeout: a lease cannot expire under a creation that still runs
	createLockTTL = 6 * time.Minute
)

// DynamoCreateLocks keeps the locks in the store table: one expiring item per domain
type DynamoCreateLocks struct {
	DB    dynamodbiface.DynamoDBAPI
	Table string
	Now   func() time.Time
}

// Acquire takes the lock of the domain unless an unexpired one exists, and returns its owner token
func (d DynamoCreateLocks) Acquire(ctx context.Context, domain string) (string, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return "", fmt.Errorf("company create lock: %w", err)
	}
	owner, now := id.String(), d.now()
	_, err = d.DB.PutItemWithContext(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.Table),
		Item: map[string]*dynamodb.AttributeValue{
			"key":    {S: aws.String(createLockKeyPrefix + domain)},
			"value":  {S: aws.String(owner)},
			"expire": {N: aws.String(strconv.FormatInt(now.Add(createLockTTL).Unix(), 10))},
		},
		ConditionExpression:       aws.String("attribute_not_exists(#k) OR #e < :now"),
		ExpressionAttributeNames:  map[string]*string{"#k": aws.String("key"), "#e": aws.String("expire")},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{":now": {N: aws.String(strconv.FormatInt(now.Unix(), 10))}},
	})
	if isConditionalCheckFailed(err) {
		return "", ErrCreateLockHeld
	}
	if err != nil {
		return "", fmt.Errorf("company create lock: %w", err)
	}
	return owner, nil
}

// Release removes the lock when owner still holds it
func (d DynamoCreateLocks) Release(ctx context.Context, domain, owner string) error {
	_, err := d.DB.DeleteItemWithContext(ctx, &dynamodb.DeleteItemInput{
		TableName:                 aws.String(d.Table),
		Key:                       map[string]*dynamodb.AttributeValue{"key": {S: aws.String(createLockKeyPrefix + domain)}},
		ConditionExpression:       aws.String("#v = :owner"),
		ExpressionAttributeNames:  map[string]*string{"#v": aws.String("value")},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{":owner": {S: aws.String(owner)}},
	})
	if err != nil && !isConditionalCheckFailed(err) {
		return fmt.Errorf("company create lock: %w", err)
	}
	return nil
}

func (d DynamoCreateLocks) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

func isConditionalCheckFailed(err error) bool {
	var awsErr awserr.Error
	return errors.As(err, &awsErr) && awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException
}
