// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/communitybridge/easycla/cla-backend-go/utils"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

//DBStore represents DB Model for the store table
type DBStore struct {
	Key    string `dynamodbav:"key"`
	Value  string `dynamodbav:"value"`
	Expire int64  `dynamodbav:"expire"`
}

// Repository interface
type Repository interface {
	SetActiveSignatureMetaData(ctx context.Context, key string, expire int64, value string) error
}

type repo struct {
	stage          string
	dynamoDBClient *dynamodb.DynamoDB
	storeTableName string
}

//NewRepository initiates Store repository instance
func NewRepository(awsSession *session.Session, stage string) Repository {
	return repo{
		stage:          stage,
		dynamoDBClient: dynamodb.New(awsSession),
		storeTableName: fmt.Sprintf("cla-%s-store", stage),
	}
}

// SetActiveSignatureMetaData sets active signature meta data
func (r repo) SetActiveSignatureMetaData(ctx context.Context, key string, expire int64, value string) error {
	f := logrus.Fields{
		"functionName":   "v2.store.repository.SetActiveSignatureMetaData",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"key":            key,
		"value":          value,
		"expire":         expire,
	}

	store := DBStore{
		Key:    key,
		Value:  value,
		Expire: expire,
	}

	log.WithFields(f).Debugf("key: %s ", store.Key)
	log.WithFields(f).Debugf("value: %+s ", store.Value)

	v, err := dynamodbattribute.MarshalMap(store)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem marshalling store record")
		return err
	}

	log.WithFields(f).Debugf("Marshalled values: %+v", v)

	_, err = r.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:      v,
		TableName: &r.storeTableName,
	})

	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to save store record")
		return err
	}

	log.WithFields(f).Debugf("Signature meta record data saved: %+v ", store)

	return nil
}
