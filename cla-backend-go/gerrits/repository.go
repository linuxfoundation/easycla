// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gerrits

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/sirupsen/logrus"

	"github.com/gofrs/uuid"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// errors
var (
	ErrGerritNotFound = errors.New("gerrit not found")
	HugePageSize      = int64(10000)
)

// Repository defines functions of V3Repositories
type Repository interface {
	AddGerrit(ctx context.Context, input *models.Gerrit) (*models.Gerrit, error)
	GetGerrit(ctx context.Context, gerritID string) (*models.Gerrit, error)
	GetGerritsByID(ctx context.Context, ID string, IDType string) (*models.GerritList, error)
	GetGerritsByProjectSFID(ctx context.Context, projectSFID string) (*models.GerritList, error)
	GetClaGroupGerrits(ctx context.Context, claGroupID string) (*models.GerritList, error)
	ExistsByName(ctx context.Context, gerritName string) ([]*models.Gerrit, error)
	DeleteGerrit(ctx context.Context, gerritID string) error
}

// NewRepository create new Repository
func NewRepository(awsSession *session.Session, stage string) Repository {
	return &repo{
		stage:          stage,
		dynamoDBClient: dynamodb.New(awsSession),
		tableName:      fmt.Sprintf("cla-%s-gerrit-instances", stage),
	}
}

type repo struct {
	stage          string
	dynamoDBClient *dynamodb.DynamoDB
	tableName      string
}

// AddGerrit creates a new gerrit instance
func (repo *repo) AddGerrit(ctx context.Context, input *models.Gerrit) (*models.Gerrit, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.repository.AddGerrit",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	gerritID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	_, currentTime := utils.CurrentTime()
	gerrit := &Gerrit{
		DateCreated:  currentTime,
		DateModified: currentTime,
		GerritID:     gerritID.String(),
		GerritName:   input.GerritName,
		GerritURL:    input.GerritURL.String(),
		GroupIDCcla:  input.GroupIDCcla,
		ProjectID:    input.ProjectID,
		ProjectSFID:  input.ProjectSFID,
		Version:      input.Version,
	}
	av, err := dynamodbattribute.MarshalMap(gerrit)
	if err != nil {
		return nil, err
	}

	_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(repo.tableName),
	})
	if err != nil {
		log.WithFields(f).WithError(err).Warn("cannot add gerrit in dynamodb")
		return nil, err
	}

	return repo.GetGerrit(ctx, gerritID.String())
}

// GetGerrit returns the gerrit instances based on the ID
func (repo *repo) GetGerrit(ctx context.Context, gerritID string) (*models.Gerrit, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.repository.GetGerrit",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"gerritID":       gerritID,
	}

	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"gerrit_id": {
				S: aws.String(gerritID),
			},
		},
		TableName: aws.String(repo.tableName),
	}

	result, err := repo.dynamoDBClient.GetItem(input)
	if err != nil {
		log.WithFields(f).Warnf("error getting gerrit repository : %s. error = %s", gerritID, err)
		return nil, err
	}
	if len(result.Item) == 0 {
		return nil, ErrGerritNotFound
	}
	var gerrit Gerrit
	err = dynamodbattribute.UnmarshalMap(result.Item, &gerrit)
	if err != nil {
		log.WithFields(f).Warnf("unable to read data from gerrit repository : %s. error = %s", gerritID, err)
		return nil, err
	}

	return gerrit.toModel(), nil
}

func (repo repo) GetGerritsByID(ctx context.Context, ID string, IDType string) (*models.GerritList, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.repository.GetGerritsByID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"ID":             ID,
		"IDType":         IDType,
	}
	var filter expression.ConditionBuilder
	resultList := make([]*models.Gerrit, 0)

	if IDType == "ICLA" {
		filter = expression.Name("group_id_icla").Equal(expression.Value(ID))
	} else if IDType == "CCLA" {
		filter = expression.Name("group_id_ccla").Equal(expression.Value(ID))
	} else {
		return nil, errors.New("invalid IDType")
	}
	// filter = filter.And(expression.Name("project_sfid").Equal(expression.Value(*projectSFID)))

	expr, err := expression.NewBuilder().WithFilter(filter).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for gerrit instances scan, error: %v", err)
		return nil, err
	}
	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.tableName),
	}

	for {
		results, err := repo.dynamoDBClient.Scan(scanInput)
		if err != nil {
			log.WithFields(f).Warnf("error retrieving gerrit instances, error: %v", err)
			return nil, err
		}

		var gerrits []*Gerrit

		err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &gerrits)
		if err != nil {
			log.WithFields(f).Warnf("error unmarshalling gerrit from database. error: %v", err)
			return nil, err
		}

		for _, g := range gerrits {
			resultList = append(resultList, g.toModel())
		}

		if len(results.LastEvaluatedKey) != 0 {
			scanInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}
	sort.Slice(resultList, func(i, j int) bool {
		return resultList[i].GerritName < resultList[j].GerritName
	})
	return &models.GerritList{List: resultList}, nil
}

func (repo repo) GetGerritsByProjectSFID(ctx context.Context, projectSFID string) (*models.GerritList, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.repository.GetGerritsByProjectSFID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
	}

	resultList := make([]*models.Gerrit, 0)
	condition := expression.Key("project_sfid").Equal(expression.Value(projectSFID))
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error building expression for gerrit instances scan, error: %v", err)
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
		IndexName:                 aws.String("gerrit-project-sfid-index"),
	}

	for {
		results, err := repo.dynamoDBClient.Query(queryInput)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("error retrieving gerrit instances by projectSFID, error: %v", err)
			return nil, err
		}

		var gerrits []*Gerrit

		err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &gerrits)
		if err != nil {
			log.WithFields(f).Warnf("error unmarshalling gerrit from database. error: %v", err)
			return nil, err
		}

		for _, g := range gerrits {
			resultList = append(resultList, g.toModel())
		}

		if len(results.LastEvaluatedKey) != 0 {
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}

	// Sort by gerrit name
	sort.Slice(resultList, func(i, j int) bool {
		return resultList[i].GerritName < resultList[j].GerritName
	})

	return &models.GerritList{List: resultList}, nil
}

// GetClaGroupGerrits returns the CLA Group gerrit instances based on the CLA Group ID
func (repo repo) GetClaGroupGerrits(ctx context.Context, claGroupID string) (*models.GerritList, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.repository.GetClaGroupGerrits",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
	}

	resultList := make([]*models.Gerrit, 0)
	condition := expression.Key("project_id").Equal(expression.Value(claGroupID))

	expr, err := expression.NewBuilder().WithKeyCondition(condition).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for gerrit instances query, error: %v", err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.tableName),
		IndexName:                 aws.String("gerrit-project-id-index"),
		Limit:                     aws.Int64(HugePageSize),
	}

	for {
		results, err := repo.dynamoDBClient.Query(queryInput)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("error retrieving gerrit instances, error: %v", err)
			return nil, err
		}

		var gerrits []*Gerrit

		err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &gerrits)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("error unmarshalling gerrit from database. error: %v", err)
			return nil, err
		}

		for _, g := range gerrits {
			resultList = append(resultList, g.toModel())
		}

		if len(results.LastEvaluatedKey) != 0 {
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}

	// Sort the results
	sort.Slice(resultList, func(i, j int) bool {
		return resultList[i].GerritName < resultList[j].GerritName
	})

	return &models.GerritList{List: resultList}, nil
}

// DeleteGerrit removes the gerrit instance based on the gerrit ID
func (repo *repo) DeleteGerrit(ctx context.Context, gerritID string) error {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.repository.DeleteGerrit",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"gerritID":       gerritID,
	}

	input := &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"gerrit_id": {
				S: aws.String(gerritID),
			},
		},
		TableName: aws.String(repo.tableName),
	}

	_, err := repo.dynamoDBClient.DeleteItem(input)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error updating gerrit repository : %s during delete project process ", gerritID)
		return err
	}

	return nil
}

func (repo *repo) ExistsByName(ctx context.Context, gerritName string) ([]*models.Gerrit, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.repository.ExistsByName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"gerritName":     gerritName,
	}
	resultList := make([]*models.Gerrit, 0)

	var condition expression.KeyConditionBuilder

	// hashkey is gerrit-name
	indexName := "gerrit-name-index"

	builder := expression.NewBuilder().WithProjection(buildProjection())

	filter := expression.Name("gerrit_id").AttributeExists()
	condition = expression.Key("gerrit_name").Equal(expression.Value(gerritName))

	builder = builder.WithKeyCondition(condition).WithFilter(filter)
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
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
		ScanIndexForward:          aws.Bool(false),
	}

	for {
		results, errQuery := repo.dynamoDBClient.Query(input)
		if errQuery != nil {
			log.WithFields(f).WithError(errQuery).Warnf("error retrieving Gerrit. error = %s", errQuery.Error())
			return nil, errQuery
		}

		var gerrits []*Gerrit

		err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &gerrits)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("error unmarshalling gerrit from database. error: %v", err)
			return nil, err
		}

		for _, g := range gerrits {
			resultList = append(resultList, g.toModel())
		}

		if len(results.LastEvaluatedKey) != 0 {
			input.ExclusiveStartKey = results.LastEvaluatedKey

		} else {
			break
		}
	}

	// Sort the results
	sort.Slice(resultList, func(i, j int) bool {
		return resultList[i].GerritName < resultList[j].GerritName
	})

	return resultList, nil
}

func (repo *repo) ExistsByID(ctx context.Context, gerritID string) ([]*models.Gerrit, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.repository.ExistsByID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"gerritID":       gerritID,
	}
	resultList := make([]*models.Gerrit, 0)

	var condition expression.KeyConditionBuilder

	// hashkey is gerrit-name
	indexName := "gerrit-id-index"

	builder := expression.NewBuilder().WithProjection(buildProjection())

	filter := expression.Name("gerrit_id").AttributeExists()
	condition = expression.Key("group_id_icla").Equal(expression.Value(gerritID))
	//condition = expression.Key("group_id_ccla").Equal(expression.Value(gerritID))

	builder = builder.WithKeyCondition(condition).WithFilter(filter)
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
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
		ScanIndexForward:          aws.Bool(false),
	}

	for {
		results, errQuery := repo.dynamoDBClient.Query(input)
		if errQuery != nil {
			log.WithFields(f).WithError(errQuery).Warnf("error retrieving Gerrit. error = %s", errQuery.Error())
			return nil, errQuery
		}

		var gerrits []*Gerrit

		err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &gerrits)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("error unmarshalling gerrit from database. error: %v", err)
			return nil, err
		}

		for _, g := range gerrits {
			resultList = append(resultList, g.toModel())
		}

		if len(results.LastEvaluatedKey) != 0 {
			input.ExclusiveStartKey = results.LastEvaluatedKey

		} else {
			break
		}
	}

	// Sort the results
	sort.Slice(resultList, func(i, j int) bool {
		return resultList[i].GerritName < resultList[j].GerritName
	})

	return resultList, nil
}

// buildProjection builds the query projection
func buildProjection() expression.ProjectionBuilder {
	// These are the columns we want returned
	return expression.NamesList(
		expression.Name("date_created"),
		expression.Name("date_modified"),
		expression.Name("gerrit_id"),
		expression.Name("gerrit_name"),
		expression.Name("gerrit_url"),
		expression.Name("group_id_ccla"),
		expression.Name("group_id_icla"),
		expression.Name("group_name_ccla"),
		expression.Name("group_name_icla"),
		expression.Name("project_id"),
		expression.Name("project_sfid"),
	)
}
