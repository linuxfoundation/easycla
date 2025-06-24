// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/project/models"

	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/gofrs/uuid"
)

// IRepository interface methods
type IRepository interface { //nolint
	CreateRequest(reqModel *CLAManagerRequest) (*CLAManagerRequest, error)
	GetRequests(companyID, projectID string) (*CLAManagerRequests, error)
	GetRequestsByUserID(companyID, projectID, userID string) (*CLAManagerRequests, error)
	GetRequest(requestID string) (*CLAManagerRequest, error)
	GetRequestsByCLAGroup(claGroupID string) ([]CLAManagerRequest, error)
	UpdateRequestsByCLAGroup(model *models.DBProjectModel) error

	ApproveRequest(companyID, projectID, requestID string) (*CLAManagerRequest, error)
	DenyRequest(companyID, projectID, requestID string) (*CLAManagerRequest, error)
	PendingRequest(companyID, projectID, requestID string) (*CLAManagerRequest, error)
	DeleteRequest(requestID string) error
	updateRequestStatus(companyID, projectID, requestID, status string) (*CLAManagerRequest, error)
}

type repository struct {
	stage          string
	dynamoDBClient *dynamodb.DynamoDB
	tableName      string
}

// NewRepository creates a new company repository instance
func NewRepository(awsSession *session.Session, stage string) IRepository {
	return repository{
		stage:          stage,
		dynamoDBClient: dynamodb.New(awsSession),
		tableName:      fmt.Sprintf("cla-%s-cla-manager-requests", stage),
	}
}

// CreateRequest generates a new request
func (repo repository) CreateRequest(reqModel *CLAManagerRequest) (*CLAManagerRequest, error) {
	f := logrus.Fields{
		"functionName":      "CreateRequest",
		"projectName":       reqModel.ProjectName,
		"projectID":         reqModel.ProjectID,
		"projectExternalID": reqModel.ProjectExternalID,
		"companyName":       reqModel.CompanyName,
		"companyID":         reqModel.CompanyID,
		"companyExternalID": reqModel.CompanyExternalID,
		"userID":            reqModel.UserID,
		"userName":          reqModel.UserName,
		"userEmail":         reqModel.UserEmail,
		"userExternalID":    reqModel.UserExternalID,
		"status":            reqModel.Status,
	}

	requestID, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).Warnf("Unable to generate a UUID for a pending invite, error: %v", err)
		return nil, err
	}

	_, now := utils.CurrentTime()

	log.WithFields(f).Debugf("request model: %+v", reqModel)

	itemMap := map[string]*dynamodb.AttributeValue{
		"request_id": {
			S: aws.String(requestID.String()),
		},
		"company_id": {
			S: aws.String(reqModel.CompanyID),
		},
		"company_name": {
			S: aws.String(reqModel.CompanyName),
		},
		"project_id": {
			S: aws.String(reqModel.ProjectID),
		},
		"project_name": {
			S: aws.String(reqModel.ProjectName),
		},
		"user_id": {
			S: aws.String(reqModel.UserID),
		},
		"user_name": {
			S: aws.String(reqModel.UserName),
		},
		"user_email": {
			S: aws.String(reqModel.UserEmail),
		},
		"status": {
			S: aws.String("pending"),
		},
		"date_created": {
			S: aws.String(now),
		},
		"date_modified": {
			S: aws.String(now),
		},
	}

	// If provided the project external ID - add it
	if reqModel.ProjectExternalID != "" {
		itemMap["project_external_id"] = &dynamodb.AttributeValue{
			S: aws.String(reqModel.ProjectExternalID),
		}
	}
	// If provided the company project external - add it
	if reqModel.CompanyExternalID != "" {
		itemMap["company_external_id"] = &dynamodb.AttributeValue{
			S: aws.String(reqModel.CompanyExternalID),
		}
	}
	// If provided the user external ID - add it
	if reqModel.UserExternalID != "" {
		itemMap["user_external_id"] = &dynamodb.AttributeValue{
			S: aws.String(reqModel.UserExternalID),
		}
	}

	input := &dynamodb.PutItemInput{
		Item:      itemMap,
		TableName: aws.String(repo.tableName),
	}

	_, err = repo.dynamoDBClient.PutItem(input)
	if err != nil {
		log.WithFields(f).Warnf("unable to create a new CLA Manager request, error: %v", err)
		return nil, err
	}

	// Load the created record
	createdRequest, err := repo.GetRequest(requestID.String())
	if err != nil || createdRequest == nil {
		log.WithFields(f).Warnf("unable to query newly created CLA Manager request by id: %s, error: %v",
			requestID.String(), err)
		return nil, err
	}

	return createdRequest, nil
}

// GetRequests returns the requests by Company ID and Project ID
func (repo repository) GetRequests(companyID, projectID string) (*CLAManagerRequests, error) {
	f := logrus.Fields{
		"functionName": "GetRequests",
		"companyID":    companyID,
		"projectID":    projectID,
	}

	condition := expression.Key("company_id").Equal(expression.Value(companyID)).And(
		expression.Key("project_id").Equal(expression.Value(projectID)))

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().
		WithKeyCondition(condition).
		WithProjection(buildRequestProjection()).
		Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for cla manager request query, error: %v", err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.tableName),
		IndexName:                 aws.String("cla-manager-requests-company-project-index"),
	}

	results, errQuery := repo.dynamoDBClient.Query(queryInput)
	if errQuery != nil {
		log.WithFields(f).Warnf("error running query for cla manager request query, error: %v", err)
		return nil, errQuery
	}

	var requests []CLAManagerRequest

	// Unmarshall the DB response
	unmarshallErr := dynamodbattribute.UnmarshalListOfMaps(results.Items, &requests)
	if unmarshallErr != nil {
		log.WithFields(f).Warnf("error converting DB model cla manager request query, error: %v", unmarshallErr)
		return nil, unmarshallErr
	}

	return &CLAManagerRequests{
		Requests: requests,
	}, nil
}

// GetRequestsByUserID returns the requests by Company ID and Project ID and User ID
func (repo repository) GetRequestsByUserID(companyID, projectID, userID string) (*CLAManagerRequests, error) {
	f := logrus.Fields{
		"functionName": "GetRequestsByUserID",
		"companyID":    companyID,
		"projectID":    projectID,
		"userID":       userID,
	}

	condition := expression.Key("company_id").Equal(expression.Value(companyID)).And(
		expression.Key("project_id").Equal(expression.Value(projectID)))

	filter := expression.Name("user_id").Contains(userID)

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().
		WithKeyCondition(condition).
		WithFilter(filter).
		WithProjection(buildRequestProjection()).
		Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for cla manager request query, error: %v", err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.tableName),
		IndexName:                 aws.String("cla-manager-requests-company-project-index"),
	}

	results, errQuery := repo.dynamoDBClient.Query(queryInput)
	if errQuery != nil {
		log.WithFields(f).Warnf("error running query for cla manager request query, error: %v", err)
		return nil, errQuery
	}

	var requests []CLAManagerRequest

	// Unmarshall the DB response
	unmarshallErr := dynamodbattribute.UnmarshalListOfMaps(results.Items, &requests)
	if unmarshallErr != nil {
		log.WithFields(f).Warnf("error converting DB model cla manager request query, error: %v", unmarshallErr)
		return nil, unmarshallErr
	}

	return &CLAManagerRequests{
		Requests: requests,
	}, nil
}

// GetRequest returns the request by Request ID
func (repo repository) GetRequest(requestID string) (*CLAManagerRequest, error) {
	f := logrus.Fields{
		"functionName": "GetRequest",
		"requestID":    requestID,
	}

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().
		WithProjection(buildRequestProjection()).
		Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for cla manager request query, error: %v", err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"request_id": {S: aws.String(requestID)},
		},
		ProjectionExpression:     expr.Projection(),
		ExpressionAttributeNames: expr.Names(),
		TableName:                aws.String(repo.tableName),
	}

	result, errQuery := repo.dynamoDBClient.GetItem(queryInput)
	if errQuery != nil {
		log.WithFields(f).Warnf("error running query for cla manager request query, error: %v", err)
		return nil, errQuery
	}

	// If no response...
	if result.Item == nil {
		return nil, nil
	}

	var request CLAManagerRequest

	// Unmarshall the DB response
	unmarshallErr := dynamodbattribute.UnmarshalMap(result.Item, &request)
	if unmarshallErr != nil {
		log.WithFields(f).Warnf("error converting DB model cla manager request query, error: %v", unmarshallErr)
		return nil, unmarshallErr
	}

	return &request, nil
}

// DeleteRequest deletes the request by Request ID
func (repo repository) DeleteRequest(requestID string) error {
	f := logrus.Fields{
		"functionName": "DeleteRequest",
		"requestID":    requestID,
	}

	_, err := repo.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"request_id": {S: aws.String(requestID)},
		},
		TableName: aws.String(repo.tableName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				return fmt.Errorf("request ID %s does not exist", requestID)
			}
		}
		log.WithFields(f).Warnf("error deleting request, error: %+v", err)
		return err
	}
	return nil
}

// ApproveRequest approves the specified request
func (repo repository) updateRequestStatus(companyID, projectID, requestID, status string) (*CLAManagerRequest, error) {
	f := logrus.Fields{
		"functionName": "updateRequestStatus",
		"companyID":    companyID,
		"projectID":    projectID,
		"requestID":    requestID,
		"status":       status,
	}

	// First, let's check if we already have a previous request
	requestModel, err := repo.GetRequest(requestID)
	if err != nil || requestModel == nil {
		log.WithFields(f).Warnf("CLA Manager updateRequestStatus - unable to locate previous request, error: %v", err)
		return nil, err
	}

	_, now := utils.CurrentTime()

	input := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"request_id": {
				S: aws.String(requestID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#S": aws.String("status"),
			"#M": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":s": {
				S: aws.String(status),
			},
			":m": {
				S: aws.String(now),
			},
		},
		UpdateExpression: aws.String("SET #S = :s, #M = :m"),
		TableName:        aws.String(repo.tableName),
	}

	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("CLA Manager ApproveRequest - unable to update request, error: %v", updateErr)
		return nil, updateErr
	}

	// Load the updated document and return it
	updatedRequestModel, err := repo.GetRequest(requestID)
	if err != nil {
		log.WithFields(f).Warnf("CLA Manager updateRequestStatus - unable to locate previous request, error: %v",
			err)
		return nil, err
	}

	return updatedRequestModel, nil
}

// ApproveRequest approves the specified request
func (repo repository) ApproveRequest(companyID, projectID, requestID string) (*CLAManagerRequest, error) {
	return repo.updateRequestStatus(companyID, projectID, requestID, "approved")
}

// DenyRequest denies the specified request
func (repo repository) DenyRequest(companyID, projectID, requestID string) (*CLAManagerRequest, error) {
	return repo.updateRequestStatus(companyID, projectID, requestID, "denied")
}

// PendingRequest updates the status of an existing request to pending
func (repo repository) PendingRequest(companyID, projectID, requestID string) (*CLAManagerRequest, error) {
	return repo.updateRequestStatus(companyID, projectID, requestID, "pending")
}

func (repo repository) GetRequestsByCLAGroup(claGroupID string) ([]CLAManagerRequest, error) {
	f := logrus.Fields{
		"functionName": "GetRequestsByCLAGroup",
		"claGroupID":   claGroupID,
		"tableName":    repo.tableName,
	}

	// This is the key we want to match
	condition := expression.Key("project_id").Equal(expression.Value(claGroupID))

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildRequestProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project requests query, error: %v", err)
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.tableName),
		IndexName:                 aws.String("cla-manager-requests-project-index"),
	}

	var claManagerRequests []CLAManagerRequest
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving project requests, error: %v", errQuery)
			return nil, errQuery
		}

		// The DB project model
		var requests []CLAManagerRequest
		err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &requests)
		if err != nil {
			log.Warnf("error unmarshalling cla manager requests from database, error: %v", err)
			return nil, err
		}

		// Add to the project response models to the list
		claManagerRequests = append(claManagerRequests, requests...)

		if results.LastEvaluatedKey["request_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["request_id"].S
			queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"request_id": {
					S: aws.String(lastEvaluatedKey),
				},
				"project_id": {
					S: aws.String(claGroupID),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}
	}

	return claManagerRequests, nil
}

// UpdateRequestsByCLAGroup handles updating the existing requests in our table based on the modified/updated CLA Group
func (repo repository) UpdateRequestsByCLAGroup(model *models.DBProjectModel) error {
	f := logrus.Fields{
		"functionName": "UpdateRequestsByCLAGroup",
		"claGroupID":   model.ProjectID,
		"tableName":    repo.tableName,
	}

	requests, err := repo.GetRequestsByCLAGroup(model.ProjectID)
	if err != nil {
		log.WithFields(f).Warnf("unable to query approval list requests by CLA Group ID")
	}
	log.WithFields(f).Debugf("updating %d CLA Manager Requests for CLA Group", len(requests))

	// For each request for this CLA Group...
	for _, request := range requests {
		log.WithFields(f).Debugf("processing request: %+v", request)

		// Only update if one of the fields that we have in our database column list
		// is updated - no need to update if other internal CLA Group record stuff is
		// updated as we don't care about those
		if request.ProjectName == model.ProjectName && request.ProjectExternalID == model.ProjectExternalID {
			log.WithFields(f).Debugf("ignoring update - project name or project external ID didn't change")
			continue
		}

		_, currentTime := utils.CurrentTime()
		expressionAttributeNames := map[string]*string{
			"#M": aws.String("date_modified"),
		}
		expressionAttributeValues := map[string]*dynamodb.AttributeValue{
			":m": {
				S: aws.String(currentTime),
			},
		}
		updateExpression := "SET #M = :m"

		// CLA Group Name has been updated
		if request.ProjectName != model.ProjectName {
			log.WithFields(f).Debugf("project name differs: %s vs %s", request.ProjectName, model.ProjectName)

			expressionAttributeNames["#N"] = aws.String("project_name")
			expressionAttributeValues[":n"] = &dynamodb.AttributeValue{
				S: aws.String(model.ProjectName),
			}
			updateExpression = fmt.Sprintf("%s, %s", updateExpression, " #N = :n ")
		}

		// CLA Group External ID was added or updated
		if request.ProjectExternalID != model.ProjectExternalID {
			log.WithFields(f).Debugf("project external ID differs: %s vs %s", request.ProjectExternalID, model.ProjectExternalID)

			expressionAttributeNames["#E"] = aws.String("project_external_id")
			expressionAttributeValues[":e"] = &dynamodb.AttributeValue{
				S: aws.String(model.ProjectExternalID),
			}
			updateExpression = fmt.Sprintf("%s, %s", updateExpression, " #E = :e ")
		}

		input := &dynamodb.UpdateItemInput{
			Key: map[string]*dynamodb.AttributeValue{
				"request_id": {
					S: aws.String(request.RequestID),
				},
			},
			ExpressionAttributeNames:  expressionAttributeNames,
			ExpressionAttributeValues: expressionAttributeValues,
			UpdateExpression:          aws.String(updateExpression),
			TableName:                 aws.String(repo.tableName),
		}

		_, err := repo.dynamoDBClient.UpdateItem(input)
		if err != nil {
			log.WithFields(f).Warnf("unable to update cla manager request with updated project information, error: %v",
				err)
			return err
		}
	}

	return nil
}
