// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package approval_list

import (
	"errors"
	"fmt"

	models2 "github.com/linuxfoundation/easycla/cla-backend-go/project/models"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

const (
	// Version is version of CclaWhitelistRequest
	Version = "v1"
	// StatusPending is status of CclaWhitelistRequest
	StatusPending = "pending"

	// ProjectIDIndex is the index for for the project_id secondary index
	ProjectIDIndex = "ccla-approval-list-request-project-id-index"
)

// IRepository interface defines the functions for the approval list service
type IRepository interface {
	AddCclaApprovalRequest(company *models.Company, project *models.ClaGroup, user *models.User, requesterName, requesterEmail string) (string, error)
	GetCclaApprovalListRequest(requestID string) (*CLARequestModel, error)
	ApproveCclaApprovalListRequest(requestID string) error
	RejectCclaApprovalListRequest(requestID string) error
	ListCclaApprovalListRequests(companyID string, projectID, status, userID *string) (*models.CclaWhitelistRequestList, error)
	GetRequestsByCLAGroup(claGroupID string) ([]CLARequestModel, error)
	UpdateRequestsByCLAGroup(model *models2.DBProjectModel) error
}

type repository struct {
	stage          string
	dynamoDBClient *dynamodb.DynamoDB
	tableName      string
}

// NewRepository creates a new instance of the approval list service
func NewRepository(awsSession *session.Session, stage string) IRepository {
	return repository{
		stage:          stage,
		dynamoDBClient: dynamodb.New(awsSession),
		tableName:      fmt.Sprintf("cla-%s-ccla-whitelist-requests", stage), // TODO: rename table
	}
}

// AddCclaApprovalRequest adds the specified request
func (repo repository) AddCclaApprovalRequest(company *models.Company, project *models.ClaGroup, user *models.User, requesterName, requesterEmail string) (string, error) {
	f := logrus.Fields{
		"functionName":   "v1.approval_list.repository.AddCclaApprovalRequest",
		"requesterName":  requesterName,
		"requesterEmail": requesterEmail,
	}
	requestID, err := uuid.NewV4()
	status := "status:fail"

	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to generate a UUID for a approval request")
		return status, err
	}

	_, currentTime := utils.CurrentTime()
	input := &dynamodb.PutItemInput{
		Item:      map[string]*dynamodb.AttributeValue{},
		TableName: aws.String(repo.tableName),
	}
	addStringAttribute(input.Item, "request_id", requestID.String())
	addStringAttribute(input.Item, "request_status", StatusPending)
	addStringAttribute(input.Item, "company_id", company.CompanyID)
	addStringAttribute(input.Item, "company_name", company.CompanyName)
	addStringAttribute(input.Item, "project_id", project.ProjectID)
	addStringAttribute(input.Item, "project_name", project.ProjectName)
	addStringAttribute(input.Item, "user_id", user.UserID)
	addStringSliceAttribute(input.Item, "user_emails", []string{requesterEmail})
	addStringAttribute(input.Item, "user_name", requesterName)
	addStringAttribute(input.Item, "user_github_id", user.GithubID)
	addStringAttribute(input.Item, "user_github_username", user.GithubUsername)
	addStringAttribute(input.Item, "date_created", currentTime)
	addStringAttribute(input.Item, "date_modified", currentTime)
	addStringAttribute(input.Item, "version", Version)

	_, err = repo.dynamoDBClient.PutItem(input)
	if err != nil {
		log.WithFields(f).Warnf("AddCclaApprovalRequest - unable to create a new ccla approval request, error: %v", err)
		return status, err
	}

	return requestID.String(), nil
}

// GetCclaApprovalListRequest fetches the specified request by ID
func (repo repository) GetCclaApprovalListRequest(requestID string) (*CLARequestModel, error) {
	f := logrus.Fields{
		"functionName": "v1.approval_list.repository.GetCclaApprovalListRequest",
		"requestID":    requestID,
	}

	response, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(repo.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"request_id": {
				S: aws.String(requestID),
			},
		},
	})

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error fetching request by ID: %s, error: %v", requestID, err)
		return nil, err
	}

	requestModel := CLARequestModel{}
	err = dynamodbattribute.UnmarshalMap(response.Item, &requestModel)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error unmarshalling %s table response model data, error: %v", repo.tableName, err)
		return nil, err
	}

	return &requestModel, nil
}

// ApproveCclaApprovalListRequest approves the specified request
func (repo repository) ApproveCclaApprovalListRequest(requestID string) error {
	f := logrus.Fields{
		"functionName": "v1.approval_list.repository.ApproveCclaApprovalListRequest",
		"requestID":    requestID,
	}

	_, currentTime := utils.CurrentTime()
	input := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"request_id": {
				S: aws.String(requestID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#S": aws.String("request_status"),
			"#M": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":s": {
				S: aws.String("approved"),
			},
			":m": {
				S: aws.String(currentTime),
			},
		},
		UpdateExpression: aws.String("SET #S = :s, #M = :m"),
		TableName:        aws.String(repo.tableName),
	}

	_, err := repo.dynamoDBClient.UpdateItem(input)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to update approval request with approved status, error: %v", err)
		return err
	}

	return nil
}

// RejectCclaApprovalListRequest rejects the specified request
func (repo repository) RejectCclaApprovalListRequest(requestID string) error {
	f := logrus.Fields{
		"functionName": "v1.approval_list.repository.RejectCclaApprovalListRequest",
		"requestID":    requestID,
	}

	_, currentTime := utils.CurrentTime()
	input := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"request_id": {
				S: aws.String(requestID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#S": aws.String("request_status"),
			"#M": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":s": {
				S: aws.String("rejected"),
			},
			":m": {
				S: aws.String(currentTime),
			},
		},
		UpdateExpression: aws.String("SET #S = :s, #M = :m"),
		TableName:        aws.String(repo.tableName),
	}

	_, err := repo.dynamoDBClient.UpdateItem(input)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to update approval request with rejected status, error: %v",
			err)
		return err
	}

	return nil
}

// ListCclaApprovalListRequests list the requests for the specified query parameters
func (repo repository) ListCclaApprovalListRequests(companyID string, projectID, status, userID *string) (*models.CclaWhitelistRequestList, error) {
	f := logrus.Fields{
		"functionName": "v1.approval_list.repository.ListCclaApprovalListRequests",
		"companyID":    companyID,
		"projectID":    projectID,
		"status":       status,
		"userID":       utils.StringValue(userID),
	}

	if projectID == nil {
		return nil, errors.New("project ID can not be nil for ListCclaApprovalListRequests")
	}

	log.WithFields(f).Debugf("ListCclaApprovalListRequests with Company ID: %s, Project ID: %+v, Status: %+v, User ID: %+v",
		companyID, projectID, status, userID)

	// hashkey is company_id, range key is project_id
	indexName := "company-id-project-id-index"

	condition := expression.Key("company_id").Equal(expression.Value(companyID))
	projectExpression := expression.Key("project_id").Equal(expression.Value(projectID))
	condition = condition.And(projectExpression)

	builder := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection())

	var filter expression.ConditionBuilder
	var filterAdded bool

	// Add the status filter if provided
	if status != nil {
		log.WithFields(f).Debugf("ListCclaApprovalListRequests - Adding status: %s", *status)
		statusFilterExpression := expression.Name("request_status").Equal(expression.Value(*status))
		filter = addConditionToFilter(filter, statusFilterExpression, &filterAdded)
	}

	// Add the user ID filter if provided
	if userID != nil {
		userFilterExpression := expression.Name("user_id").Equal(expression.Value(userID))
		filter = addConditionToFilter(filter, userFilterExpression, &filterAdded)
	}
	if filterAdded {
		builder = builder.WithFilter(filter)
	}

	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error building query")
		return nil, err
	}

	// Assemble the query input parameters
	input := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.tableName),
		IndexName:                 aws.String(indexName),
	}

	queryOutput, queryErr := repo.dynamoDBClient.Query(input)
	if queryErr != nil {
		log.WithFields(f).WithError(queryErr).Warnf("list requests error while querying, error: %+v", queryErr)
		return nil, queryErr
	}

	list, err := buildCclaWhitelistRequestsModels(queryOutput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unmarshall requests error while decoding the response, error: %+v", err)
		return nil, err
	}

	return &models.CclaWhitelistRequestList{List: list}, nil
}

// GetRequestsByCLAGroup retrieves a list of requests for the specified CLA Group
func (repo repository) GetRequestsByCLAGroup(claGroupID string) ([]CLARequestModel, error) {
	f := logrus.Fields{
		"functionName": "v1.approval_list.repository.GetRequestsByCLAGroup",
		"claGroupID":   claGroupID,
		"tableName":    repo.tableName,
		"indexName":    ProjectIDIndex,
	}

	log.WithFields(f).Debugf("querying contributor approval requests by CLA group id")

	// This is the key we want to match
	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().
		WithKeyCondition(expression.Key("project_id").Equal(expression.Value(claGroupID))).
		WithProjection(buildProjection()).
		Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for contributor approval requests query by CLA gorup id, error: %+v", err)
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.tableName),
		IndexName:                 aws.String(ProjectIDIndex),
	}

	var projectRequests []CLARequestModel
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving contributor approval requests by project ID, query: %+v, error: %+v",
				queryInput, errQuery)
			return nil, errQuery
		}

		// The DB project model
		var requests []CLARequestModel
		err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &requests)
		if err != nil {
			log.Warnf("error unmarshalling contributor approval requests from database, error: %+v", err)
			return nil, err
		}

		// Add to the project response models to the list
		projectRequests = append(projectRequests, requests...)

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

	return projectRequests, nil
}

// UpdateRequestsByCLAGroup updates a list of requests for the specified CLA Group
func (repo repository) UpdateRequestsByCLAGroup(model *models2.DBProjectModel) error {
	f := logrus.Fields{
		"functionName": "v1.approval_list.repository.UpdateRequestsByCLAGroup",
		"claGroupID":   model.ProjectID,
		"tableName":    repo.tableName,
	}

	requests, err := repo.GetRequestsByCLAGroup(model.ProjectID)
	if err != nil {
		log.WithFields(f).Warnf("unable to query approval list requests by CLA Group ID")
	}
	log.WithFields(f).Debugf("updating %d contributor ccla authorization requests", len(requests))

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
			log.WithFields(f).Warnf("unable to update contributor approval request with updated project information, error: %v",
				err)
			return err
		}
	}

	return nil
}
