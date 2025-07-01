// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

// DBStore represents DB Model for the store table
type DBStore struct {
	Key    string  `dynamodbav:"key"`
	Value  string  `dynamodbav:"value"`
	Expire float64 `dynamodbav:"expire"`
}

// Repository interface
type Repository interface {
	SetActiveSignatureMetaData(ctx context.Context, key string, expire int64, value string) error
	GetActiveSignatureMetaData(ctx context.Context, UserId string) (map[string]interface{}, error)
	DeleteActiveSignatureMetaData(ctx context.Context, key string) error
}

type repo struct {
	stage          string
	dynamoDBClient *dynamodb.DynamoDB
	storeTableName string
}

// NewRepository initiates Store repository instance
func NewRepository(awsSession *session.Session, stage string) Repository {
	return repo{
		stage:          stage,
		dynamoDBClient: dynamodb.New(awsSession),
		storeTableName: fmt.Sprintf("cla-%s-store", stage),
	}
}

// GetActiveSignatureMetaData returns active signature meta data
func (r repo) GetActiveSignatureMetaData(ctx context.Context, userId string) (map[string]interface{}, error) {
	f := logrus.Fields{
		"functionName":   "v2.store.repository.GetActiveSignatureMetaData",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"userId":         userId,
	}
	var metadata map[string]interface{}

	log.WithFields(f).Debugf("querying for user: %s", userId)

	key := fmt.Sprintf("active_signature:%s", userId)

	result, err := r.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: &r.storeTableName,
		Key: map[string]*dynamodb.AttributeValue{
			"key": {
				S: &key,
			},
		},
	})

	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem querying store table")
		return metadata, err
	}

	if result.Item == nil {
		log.WithFields(f).Warn("no record found")
		return metadata, nil
	}

	var jsonStr string

	err = dynamodbattribute.Unmarshal(result.Item["value"], &jsonStr)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem unmarshalling store record")
		return metadata, err
	}

	formatJson := strings.ReplaceAll(jsonStr, "\\\"", "\"")

	formatJson = strings.Trim(formatJson, "\"")

	log.WithFields(f).Debugf("format: %s", formatJson)

	jsonErr := json.Unmarshal([]byte(formatJson), &metadata)

	if jsonErr != nil {
		log.WithFields(f).WithError(jsonErr).Warn("problem unmarshalling json string for metadata")
		return nil, jsonErr
	}

	log.WithFields(f).Debugf("metadata: %+v", metadata)
	return metadata, nil
}

// func findDifferences(str1, str2 string) string {
// 	f := logrus.Fields{
// 		"functionName": "findDifference",
// 	}
// 	var differences string

// 	// Find the minimum length of the two strings
// 	minLength := len(str1)
// 	if len(str2) < minLength {
// 		minLength = len(str2)
// 	}

// 	// Compare each character and append the differences to the result string
// 	for i := 0; i < minLength; i++ {
// 		if str1[i] != str2[i] {
// 			differences += string(str1[i]) + string(str2[i]) + " "
// 			log.WithFields(f).Debugf("%s and %s", string(str1[i]), string(str2[i]))
// 		}
// 	}

// 	// If the strings have different lengths, append the remaining characters
// 	if len(str1) > len(str2) {
// 		differences += str1[minLength:]
// 	} else if len(str2) > len(str1) {
// 		differences += str2[minLength:]
// 	}

// 	return differences
// }

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
		Expire: float64(expire),
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

func (r repo) DeleteActiveSignatureMetaData(ctx context.Context, key string) error {
	f := logrus.Fields{
		"functionName":   "v2.store.repository.DeleteActiveSignatureMetaData",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"key":            key,
	}

	log.WithFields(f).Debugf("key: %s ", key)

	_, err := r.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"key": {
				S: &key,
			},
		},
		TableName: &r.storeTableName,
	})

	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to delete store record")
		return err
	}

	log.WithFields(f).Debugf("Signature meta record data deleted: %+v ", key)

	return nil
}
