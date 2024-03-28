// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package approvals

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
)

type IRepository interface {
	GetApprovalList(approvalID string) (*ApprovalItem, error)
	GetApprovalListBySignature(signatureID string) ([]ApprovalItem, error)
	AddApprovalList(approvalItem ApprovalItem) error
	DeleteApprovalList(approvalID string) error
	SearchApprovalList(criteria, approvalListName, claGroupID, companyID, signatureID string) ([]ApprovalItem, error)
}

type repository struct {
	stage          string
	dynamoDBClient *dynamodb.DynamoDB
	tableName      string
}

func NewRepository(stage string, awsSession *session.Session, tableName string) IRepository {
	return &repository{
		stage:          stage,
		dynamoDBClient: dynamodb.New(awsSession),
		tableName:      tableName,
	}
}

func (repo *repository) GetApprovalList(approvalID string) (*ApprovalItem, error) {
	f := logrus.Fields{
		"functionName": "GetApprovalList",
		"approvalID":   approvalID,
	}

	log.WithFields(f).Debugf("repository.GetApprovalList - fetching approval list by approvalID: %s", approvalID)

	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(repo.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"approval_id": {
				S: aws.String(approvalID),
			},
		},
	})
	if err != nil {
		log.WithFields(f).Warnf("repository.GetApprovalList - unable to read data from table, error: %+v", err)
		return nil, err
	}

	if len(result.Item) == 0 {
		log.WithFields(f).Warnf("repository.GetApprovalList - no approval list found for approvalID: %s", approvalID)
		return nil, errors.New("approval list not found")
	}

	approvalItem := ApprovalItem{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &approvalItem)
	if err != nil {
		log.WithFields(f).Warnf("repository.GetApprovalList - unable to unmarshal data from table, error: %+v", err)
		return nil, err
	}

	return &approvalItem, nil
}

func (repo *repository) GetApprovalListBySignature(signatureID string) ([]ApprovalItem, error) {
	f := logrus.Fields{
		"functionName": "GetApprovalListBySignature",
		"signatureID":  signatureID,
	}

	log.WithFields(f).Debugf("repository.GetApprovalListBySignature - fetching approval list by signatureID: %s", signatureID)

	result, err := repo.dynamoDBClient.Scan(&dynamodb.ScanInput{
		TableName:        aws.String(repo.tableName),
		FilterExpression: aws.String("signature_id = :signature_id"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":signature_id": {
				S: aws.String(signatureID),
			},
		},
	})
	if err != nil {
		log.WithFields(f).Warnf("repository.GetApprovalListBySignature - unable to read data from table, error: %+v", err)
		return nil, err
	}

	approvalItems := make([]ApprovalItem, 0)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &approvalItems)
	if err != nil {
		log.WithFields(f).Warnf("repository.GetApprovalListBySignature - unable to unmarshal data from table, error: %+v", err)
		return nil, err
	}

	return approvalItems, nil
}

func (repo *repository) AddApprovalList(approvalItem ApprovalItem) error {
	f := logrus.Fields{
		"functionName": "v2.approvals.repository.AddApprovalList",
		"approvalID":   approvalItem.ApprovalID,
		"approvalName": approvalItem.ApprovalName,
		"tableName":    repo.tableName,
	}

	log.WithFields(f).Debugf("repository.AddApprovalList - adding approval list: %+v", approvalItem)

	av, err := dynamodbattribute.MarshalMap(approvalItem)
	if err != nil {
		log.WithFields(f).Warnf("repository.AddApprovalList - unable to marshal data, error: %+v", err)
		return err
	}

	_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(repo.tableName),
		Item:      av,
	})
	if err != nil {
		log.WithFields(f).Warnf("repository.AddApprovalList - unable to add data to table, error: %+v", err)
		return err
	}

	return nil
}

func (repo *repository) DeleteApprovalList(approvalID string) error {
	f := logrus.Fields{
		"functionName": "DeleteApprovalList",
		"approvalID":   approvalID,
	}

	log.WithFields(f).Debugf("repository.DeleteApprovalList - deleting approval list by approvalID: %s", approvalID)

	_, err := repo.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String(repo.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"approval_id": {
				S: aws.String(approvalID),
			},
		},
	})
	if err != nil {
		log.WithFields(f).Warnf("repository.DeleteApprovalList - unable to delete data from table, error: %+v", err)
		return err
	}

	return nil
}

func (repo *repository) SearchApprovalList(criteria, approvalListName, claGroupID, companyID, signatureID string) ([]ApprovalItem, error) {
	f := logrus.Fields{
		"functionName": "approvals.repository.SearchApprovalList",
		"criteria":     criteria,
		"approvalName": approvalListName,
		"claGroupID":   claGroupID,
		"companyID":    companyID,
		"signatureID":  signatureID,
	}

	pageSize := int64(100)

	if signatureID == "" {
		return nil, errors.New("signatureID is required")
	}
	if approvalListName == "" {
		return nil, errors.New("approvalListName is required")
	}

	condition := expression.Key("signature_id").Equal(expression.Value(signatureID))

	log.WithFields(f).Debugf("searching for approval list by approvalName: %s", approvalListName)
	filter := expression.Name("approval_name").Contains(approvalListName)

	if criteria != "" {
		log.WithFields(f).Debugf("searching for criteria: %s", criteria)
		filter = filter.And(expression.Name("approval_criteria").Contains(criteria))
	}

	if claGroupID != "" {
		log.WithFields(f).Debugf("searching for claGroupID: %s", claGroupID)
		filter = filter.And(expression.Name("project_id").Equal(expression.Value(claGroupID)))
	}

	if companyID != "" {
		log.WithFields(f).Debugf("searching for companyID: %s", companyID)
		filter = filter.And(expression.Name("company_id").Equal(expression.Value(companyID)))
	}

	expr, err := expression.NewBuilder().WithFilter(filter).WithKeyCondition(condition).Build()

	if err != nil {
		log.WithFields(f).Warnf("error building expression, error: %+v", err)
		return nil, err
	}

	input := &dynamodb.QueryInput{
		TableName:                 aws.String(repo.tableName),
		IndexName:                 aws.String("signature-id-index"),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int64(pageSize),
	}

	var results []ApprovalItem

	for {
		output, err := repo.dynamoDBClient.Query(input)
		if err != nil {
			log.WithFields(f).Warnf("error retrieving approval list, error: %+v", err)
			return nil, err
		}

		var items []ApprovalItem
		err = dynamodbattribute.UnmarshalListOfMaps(output.Items, &items)
		if err != nil {
			log.WithFields(f).Warnf("error unmarshalling data, error: %+v", err)
			return nil, err
		}

		results = append(results, items...)

		if output.LastEvaluatedKey == nil {
			break
		}

		input.ExclusiveStartKey = output.LastEvaluatedKey
	}

	return results, nil

}
