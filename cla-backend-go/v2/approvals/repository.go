// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package approvals

import (
	"errors"
	"fmt"

	// "math/rand"
	"net"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
)

type IRepository interface {
	GetApprovalList(approvalID string) (*ApprovalItem, error)
	DeleteAll() error
	GetApprovalListBySignature(signatureID string) ([]ApprovalItem, error)
	AddApprovalList(approvalItem ApprovalItem) error
	UpdateApprovalItem(approvalItem ApprovalItem) error
	DeleteApprovalList(approvalID string) error
	SearchApprovalList(criteria, approvalListName, claGroupID, companyID, signatureID string) ([]ApprovalItem, error)
	BatchAddApprovalList(approvalItems []ApprovalItem) error
	BatchDeleteApprovalList() error
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

func (repo *repository) getAll() ([]*ApprovalItem, error) {
	f := logrus.Fields{
		"functionName": "getAll",
	}

	log.WithFields(f).Debugf("repository.getAll - fetching all approval lists")

	// Get all the records
	pageSize := int64(100)

	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(repo.tableName),
		Limit:     aws.Int64(pageSize),
	}

	var results []*ApprovalItem
	for {
		result, err := repo.dynamoDBClient.Scan(scanInput)
		if err != nil {
			log.WithFields(f).Warnf("repository.getAll - unable to scan table, error: %+v", err)
			return nil, err
		}

		var items []*ApprovalItem
		err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &items)
		if err != nil {
			log.WithFields(f).Warnf("repository.getAll - unable to unmarshal data from table, error: %+v", err)
			return nil, err
		}

		results = append(results, items...)

		if result.LastEvaluatedKey == nil {
			break
		}

		scanInput.ExclusiveStartKey = result.LastEvaluatedKey
	}

	return results, nil
}

func (repo *repository) DeleteAll() error {
	f := logrus.Fields{
		"functionName": "DeleteAll",
	}

	log.WithFields(f).Debugf("repository.DeleteAll - deleting all approval lists")
	itemsToDelete, err := repo.getAll()

	if err != nil {
		log.WithFields(f).Warnf("repository.DeleteAll - unable to fetch data from table, error: %+v", err)
		return err
	}

	log.WithFields(f).Debugf("repository.DeleteAll - deleting %d approval list items", len(itemsToDelete))

	// Delete all the records
	for _, item := range itemsToDelete {
		retry := 0
		for {
			log.WithFields(f).Debugf("repository.DeleteAll - deleting approval list item: %+v", item)
			deleteRequest := &dynamodb.DeleteItemInput{
				Key: map[string]*dynamodb.AttributeValue{
					"approval_id": {
						S: aws.String(item.ApprovalID),
					},
				},
				TableName: aws.String(repo.tableName),
			}

			_, err = repo.dynamoDBClient.DeleteItem(deleteRequest)
			if err != nil {
				if aerr, ok := err.(awserr.Error); ok && aerr.Code() == dynamodb.ErrCodeProvisionedThroughputExceededException {
					if retry > 5 {
						return fmt.Errorf("unable to delete approval list item - retry limit reached, error: %+v", err)
					}
					retry++
					continue
				}
				return fmt.Errorf("unable to delete approval list item, error: %+v", err)
			}
			break
		}
	}
	return nil
}

func (repo *repository) UpdateApprovalItem(approvalItem ApprovalItem) error {
	f := logrus.Fields{
		"functionName": "v2.approvals.repository.UpdateApprovalItem",
		"approvalID":   approvalItem.ApprovalID,
	}

	log.WithFields(f).Debugf("updating approval item: %+v", approvalItem)

	av, err := dynamodbattribute.MarshalMap(approvalItem)
	if err != nil {
		log.WithFields(f).Warnf("unable to marshal data, error: %+v", err)
		return err
	}

	_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(repo.tableName),
		Item:      av,
	})

	if err != nil {
		log.WithFields(f).Warnf("repository.UpdateApprovalItem - unable to update data in table, error: %+v", err)
		return err
	}

	return nil
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

func exponentialBackoff(attempt int) time.Duration {
	const maxBackoff = 30 * time.Second
	if attempt < 0 || attempt > 63 {
		return time.Duration(30) * time.Second
	}
	backoff := time.Duration(1<<uint(attempt)) * time.Second
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return backoff
}

func (repo *repository) GetApprovalListBySignature(signatureID string) ([]ApprovalItem, error) {
	f := logrus.Fields{
		"functionName": "GetApprovalListBySignature",
		"signatureID":  signatureID,
	}

	log.WithFields(f).Debugf("repository.GetApprovalListBySignature - fetching approval list by signatureID: %s", signatureID)

	condition := expression.Key("signature_id").Equal(expression.Value(signatureID))

	expr, err := expression.NewBuilder().WithKeyCondition(condition).Build()

	if err != nil {
		return nil, err
	}

	pageSize := int64(100)

	input := &dynamodb.QueryInput{
		TableName:                 aws.String(repo.tableName),
		IndexName:                 aws.String("signature-id-index"),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int64(pageSize),
	}

	var results []ApprovalItem
	maxRetries := 5

	for attempt := 0; attempt < maxRetries; attempt++ {
		output, err := repo.dynamoDBClient.Query(input)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.WithFields(f).Warnf("error building expression, error: %+v", err)
			} else if awsErr, ok := err.(awserr.Error); ok {
				log.WithFields(f).Warnf("error building expression, error: %+v", awsErr)
			} else if err.Error() == "connection reset by peer" {
				log.WithFields(f).Warnf("error building expression, error: %+v", err)
			} else {
				return nil, err
			}

			time.Sleep(exponentialBackoff(attempt))
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

func (repo *repository) BatchDeleteApprovalList() error {
	f := logrus.Fields{
		"functionName": "v2.BatchDeleteApprovalList",
		"tableName":    repo.tableName,
	}

	log.WithFields(f).Debugf("repository.BatchDeleteApprovalList - deleting all approval list items")

	itemsToDelete, err := repo.getAll()
	startTime := time.Now()

	if err != nil {
		log.WithFields(f).Warnf("repository.BatchDeleteApprovalList - unable to fetch data from table, error: %+v", err)
		return err
	}

	log.WithFields(f).Debugf("repository.BatchDeleteApprovalList - deleting %d approval list items", len(itemsToDelete))

	batchSize := 25
	deleted := len(itemsToDelete)
	processed := 0
	var wg sync.WaitGroup

	for num := 0; num < len(itemsToDelete); num += batchSize {
		start := num
		end := num + batchSize
		if end > len(itemsToDelete) {
			end = len(itemsToDelete)
		}

		wg.Add(1)

		go func(s, e int) {
			defer wg.Done()
			var batchWriteItems []*dynamodb.WriteRequest
			for _, approvalItem := range itemsToDelete[s:e] {
				batchWriteItems = append(batchWriteItems, &dynamodb.WriteRequest{
					DeleteRequest: &dynamodb.DeleteRequest{
						Key: map[string]*dynamodb.AttributeValue{
							"approval_id": {
								S: aws.String(approvalItem.ApprovalID),
							},
						},
					},
				})
			}

			input := &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]*dynamodb.WriteRequest{
					repo.tableName: batchWriteItems,
				},
			}

			maxAttempts := 3

			for attempt := 0; attempt < maxAttempts; attempt++ {
				op, err := repo.dynamoDBClient.BatchWriteItem(input)
				if err != nil {
					if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == dynamodb.ErrCodeProvisionedThroughputExceededException {
						log.WithFields(f).Warnf("repository.BatchDeleteApprovalList - provisioned throughput exceeded, retrying in :%d seconds", exponentialBackoff(attempt))
						time.Sleep(exponentialBackoff(attempt))
						continue
					}
					log.WithFields(f).Warnf("repository.BatchDeleteApprovalList - unable to batch delete data from table, error: %+v", err)
					return
				}

				if len(op.UnprocessedItems) != 0 {
					log.WithFields(f).Warn("unprocessed items found")
					deleted -= len(op.UnprocessedItems)
				}
				break
			}

			processed += len(batchWriteItems)

			log.WithFields(f).Debugf("repository.BatchDeleteApprovalList - processed %d of %d approval list items", processed, len(itemsToDelete))

		}(start, end)

	}

	log.WithFields(f).Debug("repository.BatchDeleteApprovalList - waiting for batch delete to complete")
	wg.Wait()

	log.WithFields(f).Debugf("all batches completed: deleted %d records in %s", deleted, time.Since(startTime))

	return nil

}

func (repo *repository) BatchAddApprovalList(approvalItems []ApprovalItem) error {
	f := logrus.Fields{
		"functionName": "BatchAddApprovalList",
		"tableName":    repo.tableName,
	}

	log.WithFields(f).Debugf("repository.BatchAddApprovalList - adding %d approval list items", len(approvalItems))
	batchSize := 25
	processed := 0
	inserted := len(approvalItems)
	startTime := time.Now()
	var wg sync.WaitGroup

	for num := 0; num < len(approvalItems); num += batchSize {
		start := num
		end := num + batchSize
		if end > len(approvalItems) {
			end = len(approvalItems)
		}

		wg.Add(1)

		go func(s, e int) {
			defer wg.Done()
			var batchWriteItems []*dynamodb.WriteRequest
			for _, approvalItem := range approvalItems[s:e] {
				av, err := dynamodbattribute.MarshalMap(approvalItem)
				if err != nil {
					log.WithFields(f).Warnf("repository.BatchAddApprovalList - unable to marshal data, error: %+v", err)
					return
				}

				batchWriteItems = append(batchWriteItems, &dynamodb.WriteRequest{
					PutRequest: &dynamodb.PutRequest{
						Item: av,
					},
				})
			}

			input := &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]*dynamodb.WriteRequest{
					repo.tableName: batchWriteItems,
				},
			}

			maxAttempts := 3

			for attempt := 0; attempt < maxAttempts; attempt++ {
				op, err := repo.dynamoDBClient.BatchWriteItem(input)
				if err != nil {
					if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == dynamodb.ErrCodeProvisionedThroughputExceededException {
						log.WithFields(f).Warnf("repository.BatchAddApprovalList - provisioned throughput exceeded, retrying in :%d seconds", exponentialBackoff(attempt))
						time.Sleep(exponentialBackoff(attempt))
						continue
					}
					log.WithFields(f).Warnf("repository.BatchAddApprovalList - unable to batch add data to table, error: %+v", err)
					return
				}

				if len(op.UnprocessedItems) != 0 {
					log.WithFields(f).Warn("unprocessed items found")
					inserted -= len(op.UnprocessedItems)
				}
				break
			}
			processed += len(batchWriteItems)
			log.WithFields(f).Debugf("repository.BatchAddApprovalList - processed %d of %d approval list items", processed, len(approvalItems))

		}(start, end)

	}

	log.WithFields(f).Debug("repository.BatchAddApprovalList - waiting for batch add to complete")
	wg.Wait()

	log.WithFields(f).Debugf("all batches completed: inserted %d records in %s", inserted, time.Since(startTime))

	return nil
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

	const maxRetries = 5
	var retryDelay time.Duration = 1

	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(repo.tableName),
			Item:      av,
		})

		if err != nil {
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() {
				log.WithFields(f).Warnf("repository.AddApprovalList - timeout error, retrying in %d seconds", retryDelay)
				time.Sleep(exponentialBackoff(attempt))
				continue
			}
			awsErr, ok := err.(awserr.Error)
			if !ok {
				log.WithFields(f).Warnf("repository.AddApprovalList - unable to add data to table, error: %+v", err)
				return err
			}

			switch awsErr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				log.WithFields(f).Warnf("repository.AddApprovalList - provisioned throughput exceeded, retrying in %d seconds", retryDelay)
				time.Sleep(retryDelay * time.Second)
				retryDelay = retryDelay * 2 // exponential backoff
				continue
			default:
				log.WithFields(f).Warnf("repository.AddApprovalList - unable to add data to table, error: %+v", err)
				return err
			}
		}
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
