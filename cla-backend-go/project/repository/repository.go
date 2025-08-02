// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/linuxfoundation/easycla/cla-backend-go/project/common"
	models2 "github.com/linuxfoundation/easycla/cla-backend-go/project/models"

	"github.com/sirupsen/logrus"

	"github.com/linuxfoundation/easycla/cla-backend-go/gerrits"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/repositories"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/project"

	"github.com/gofrs/uuid"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

// errors
var (
	ErrProjectDoesNotExist = errors.New("project does not exist")
	ErrProjectIDMissing    = errors.New("project id is missing")
)

// constants
const (
	LoadRepoDetails     = true
	DontLoadRepoDetails = false
)

// ProjectRepository defines functions of Project repository
type ProjectRepository interface { //nolint
	CreateCLAGroup(ctx context.Context, claGroupModel *models.ClaGroup) (*models.ClaGroup, error)
	GetCLAGroupByID(ctx context.Context, claGroupID string, loadRepoDetails bool) (*models.ClaGroup, error)
	GetCLAGroupByIDCompat(ctx context.Context, claGroupID string, loadRepoDetails bool) (*models.ClaGroup, error)
	GetCLAGroupsByExternalID(ctx context.Context, params *project.GetProjectsByExternalIDParams, loadRepoDetails bool) (*models.ClaGroups, error)
	GetCLAGroupByName(ctx context.Context, claGroupName string) (*models.ClaGroup, error)
	GetExternalCLAGroup(ctx context.Context, claGroupExternalID string) (*models.ClaGroup, error)
	GetCLAGroups(ctx context.Context, params *project.GetProjectsParams) (*models.ClaGroups, error)
	DeleteCLAGroup(ctx context.Context, claGroupID string) error
	UpdateCLAGroup(ctx context.Context, claGroupModel *models.ClaGroup) (*models.ClaGroup, error)

	GetClaGroupsByFoundationSFID(ctx context.Context, foundationSFID string, loadRepoDetails bool) (*models.ClaGroups, error)
	GetClaGroupByProjectSFID(ctx context.Context, projectSFID string, loadRepoDetails bool) (*models.ClaGroup, error)
	UpdateRootCLAGroupRepositoriesCount(ctx context.Context, claGroupID string, diff int64, reset bool) error
}

// NewRepository creates instance of project repository
func NewRepository(awsSession *session.Session, stage string, ghRepo repositories.RepositoryInterface, gerritRepo gerrits.Repository, projectClaGroupRepo projects_cla_groups.Repository) ProjectRepository {
	return &repo{
		dynamoDBClient:      dynamodb.New(awsSession),
		stage:               stage,
		ghRepo:              ghRepo,
		gerritRepo:          gerritRepo,
		projectClaGroupRepo: projectClaGroupRepo,
		claGroupTable:       fmt.Sprintf("cla-%s-projects", stage),
	}
}

type repo struct {
	stage               string
	dynamoDBClient      *dynamodb.DynamoDB
	ghRepo              repositories.RepositoryInterface
	gerritRepo          gerrits.Repository
	projectClaGroupRepo projects_cla_groups.Repository
	claGroupTable       string
}

// CreateCLAGroup creates a new CLA Group
func (repo *repo) CreateCLAGroup(ctx context.Context, claGroupModel *models.ClaGroup) (*models.ClaGroup, error) {
	f := logrus.Fields{
		"functionName":      "project.repository.CreateCLAGroup",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"projectName":       claGroupModel.ProjectName,
		"projectExternalID": claGroupModel.ProjectExternalID,
		"foundationSFID":    claGroupModel.FoundationSFID,
		"tableName":         repo.claGroupTable,
	}
	// Generate a new CLA Group ID
	claGroupID, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).Warnf("Unable to generate a UUID for a new CLA Group request, error: %v", err)
		return nil, err
	}
	f["claGroupID"] = claGroupID

	_, currentTimeString := utils.CurrentTime()
	input := &dynamodb.PutItemInput{
		Item:      map[string]*dynamodb.AttributeValue{},
		TableName: aws.String(repo.claGroupTable),
	}

	//var individualDocs []*dynamodb.AttributeValue
	//var corporateDocs []*dynamodb.AttributeValue
	common.AddStringAttribute(input.Item, "project_id", claGroupID.String())
	common.AddStringAttribute(input.Item, "project_external_id", claGroupModel.ProjectExternalID)
	common.AddStringAttribute(input.Item, "foundation_sfid", claGroupModel.FoundationSFID)
	common.AddStringAttribute(input.Item, "project_description", claGroupModel.ProjectDescription)
	common.AddStringAttribute(input.Item, "project_name", claGroupModel.ProjectName)
	common.AddStringAttribute(input.Item, "project_template_id", claGroupModel.ProjectTemplateID)
	common.AddStringAttribute(input.Item, "project_name_lower", strings.ToLower(claGroupModel.ProjectName))
	common.AddStringSliceAttribute(input.Item, "project_acl", claGroupModel.ProjectACL)
	common.AddBooleanAttribute(input.Item, "project_icla_enabled", claGroupModel.ProjectICLAEnabled)
	common.AddBooleanAttribute(input.Item, "project_ccla_enabled", claGroupModel.ProjectCCLAEnabled)
	common.AddBooleanAttribute(input.Item, "project_ccla_requires_icla_signature", claGroupModel.ProjectCCLARequiresICLA)
	common.AddBooleanAttribute(input.Item, "project_live", claGroupModel.ProjectLive)

	// Empty documents for now - will add the template details later
	common.AddListAttribute(input.Item, "project_corporate_documents", []*dynamodb.AttributeValue{})
	common.AddListAttribute(input.Item, "project_individual_documents", []*dynamodb.AttributeValue{})
	common.AddListAttribute(input.Item, "project_member_documents", []*dynamodb.AttributeValue{})

	common.AddStringAttribute(input.Item, "date_created", currentTimeString)
	common.AddStringAttribute(input.Item, "date_modified", currentTimeString)
	// Set the version attribute if not already set
	if claGroupModel.Version == "" {
		claGroupModel.Version = utils.V1 // default value
	}
	common.AddStringAttribute(input.Item, "version", claGroupModel.Version)

	_, err = repo.dynamoDBClient.PutItem(input)
	if err != nil {
		log.WithFields(f).Warnf("Unable to create a new CLA Group record, error: %v", err)
		return nil, err
	}

	// Re-use the provided model - just update the dynamically assigned values
	claGroupModel.ProjectID = claGroupID.String()
	claGroupModel.DateCreated = currentTimeString
	claGroupModel.DateModified = currentTimeString

	return claGroupModel, nil
}

func (repo *repo) getCLAGroupByID(ctx context.Context, claGroupID string, loadCLAGroupDetails bool, claEnabledDefaultIsTrue bool) (*models.ClaGroup, error) {
	f := logrus.Fields{
		"functionName":       "project.repository.getCLAGroupByID",
		utils.XREQUESTID:     ctx.Value(utils.XREQUESTID),
		"claGroupID":         claGroupID,
		"loadProjectDetails": loadCLAGroupDetails,
		"tableName":          repo.claGroupTable}
	log.WithFields(f).Debugf("loading cla group...")
	// This is the key we want to match
	condition := expression.Key("project_id").Equal(expression.Value(claGroupID))

	// Use the builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for CLA Group query, claGroupID: %s, error: %v",
			claGroupID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.claGroupTable),
	}

	// Make the DynamoDB Query API call
	results, queryErr := repo.dynamoDBClient.Query(queryInput)
	if queryErr != nil {
		log.WithFields(f).Warnf("error retrieving cla group by claGroupID: %s, error: %v", claGroupID, queryErr)
		return nil, queryErr
	}

	if len(results.Items) < 1 {
		return nil, &utils.CLAGroupNotFound{CLAGroupID: claGroupID}
	}
	var dbModel models2.DBProjectModel
	rawItem := results.Items[0]
	err = dynamodbattribute.UnmarshalMap(rawItem, &dbModel)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling db cla group model, error: %+v", err)
		return nil, err
	}
	if claEnabledDefaultIsTrue {
		// If missing, assume true like Pynamo default=True
		if _, ok := rawItem["project_icla_enabled"]; !ok {
			dbModel.ProjectIclaEnabled = true
		}
		if _, ok := rawItem["project_ccla_enabled"]; !ok {
			dbModel.ProjectCclaEnabled = true
		}
	}

	// Convert the database model to an API response model
	return repo.buildCLAGroupModel(ctx, dbModel, loadCLAGroupDetails), nil
}

// GetCLAGroupByID returns the cla group model associated for the specified claGroupID
func (repo *repo) GetCLAGroupByID(ctx context.Context, claGroupID string, loadRepoDetails bool) (*models.ClaGroup, error) {
	return repo.getCLAGroupByID(ctx, claGroupID, loadRepoDetails, false)
}

// GetCLAGroupByIDCompat returns the cla group model associated for the specified claGroupID
func (repo *repo) GetCLAGroupByIDCompat(ctx context.Context, claGroupID string, loadRepoDetails bool) (*models.ClaGroup, error) {
	// Uses compatible mode (with python v2): claEnabledDefaultIsTrue - means if project_ccla_enabled or project_icla_enabled
	// aren't set on dynamoDB item - they will default to true as in Py V2 API
	return repo.getCLAGroupByID(ctx, claGroupID, loadRepoDetails, true)
}

// GetCLAGroupsByExternalID queries the database and returns a list of the cla groups
func (repo *repo) GetCLAGroupsByExternalID(ctx context.Context, params *project.GetProjectsByExternalIDParams, loadRepoDetails bool) (*models.ClaGroups, error) {
	f := logrus.Fields{
		"functionName":    "project.repository.GetCLAGroupsByExternalID",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"ProjectSFID":     params.ProjectSFID,
		"NextKey":         params.NextKey,
		"PageSize":        params.PageSize,
		"loadRepoDetails": loadRepoDetails,
		"tableName":       repo.claGroupTable}
	log.WithFields(f).Debugf("loading cla group")

	// This is the key we want to match
	condition := expression.Key("project_external_id").Equal(expression.Value(params.ProjectSFID))

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for cla group query, error: %v", err)
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.claGroupTable),
		IndexName:                 aws.String("external-project-index"),
	}

	// If we have the next key, set the exclusive start key value
	if params.NextKey != nil && *params.NextKey != "" {
		log.WithFields(f).Debugf("Received a nextKey, value: %s", *params.NextKey)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"project_id": {
				S: params.NextKey,
			},
		}
	}

	var pageSize *int64
	// If we have a page size, set the limit value - make sure it's a positive value
	if params.PageSize != nil && *params.PageSize > 0 {
		log.WithFields(f).Debugf("Received a pageSize parameter, value: %d", *params.PageSize)
		pageSize = params.PageSize
	} else {
		// Default page size
		pageSize = aws.Int64(50)
	}
	queryInput.Limit = pageSize

	var projects []models.ClaGroup
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving projects, error: %v", errQuery)
			return nil, errQuery
		}

		// Convert the list of DB models to a list of response models
		projectList, modelErr := repo.buildCLAGroupModels(ctx, results.Items, loadRepoDetails)
		if modelErr != nil {
			log.WithFields(f).Warnf("error converting project DB model to response model, error: %v",
				modelErr)
			return nil, modelErr
		}

		// Add to the project response models to the list
		projects = append(projects, projectList...)

		if results.LastEvaluatedKey["project_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["project_id"].S
			queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"project_id": {
					S: aws.String(lastEvaluatedKey),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}

		if int64(len(projects)) >= *pageSize {
			break
		}
	}

	return &models.ClaGroups{
		LastKeyScanned: lastEvaluatedKey,
		PageSize:       *pageSize,
		ResultCount:    int64(len(projects)),
		Projects:       projects,
	}, nil
}

// GetClaGroupsByFoundationID queries the database and returns a list of all cla_groups associated with foundation
func (repo *repo) GetClaGroupsByFoundationSFID(ctx context.Context, foundationSFID string, loadRepoDetails bool) (*models.ClaGroups, error) {
	f := logrus.Fields{
		"functionName":    "project.repository.GetClaGroupsByFoundationSFID",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"foundationSFID":  foundationSFID,
		"loadRepoDetails": loadRepoDetails,
		"tableName":       repo.claGroupTable,
	}
	log.WithFields(f).Debugf("loading CLA Group by foundation SFID - using foundation_sfid field...")

	// This is the key we want to match
	condition := expression.Key("foundation_sfid").Equal(expression.Value(foundationSFID))

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project scan, error: %v", err)
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.claGroupTable),
		IndexName:                 aws.String("foundation-sfid-project-name-index"),
	}

	var projects []models.ClaGroup
	for {
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving projects, error: %v", errQuery)
			return nil, errQuery
		}

		// Convert the list of DB models to a list of response models
		projectList, modelErr := repo.buildCLAGroupModels(ctx, results.Items, loadRepoDetails)
		if modelErr != nil {
			log.WithFields(f).Warnf("error converting project DB model to response model, error: %v",
				modelErr)
			return nil, modelErr
		}

		// Add to the project response models to the list
		projects = append(projects, projectList...)

		if len(results.LastEvaluatedKey) != 0 {
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}

	return &models.ClaGroups{
		ResultCount: int64(len(projects)),
		Projects:    projects,
	}, nil
}

// GetClaGroupByProjectSFID returns cla_group associated with project
func (repo *repo) GetClaGroupByProjectSFID(ctx context.Context, projectSFID string, loadRepoDetails bool) (*models.ClaGroup, error) {
	f := logrus.Fields{
		"functionName":    "project.repository.GetClaGroupByProjectSFID",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"projectSFID":     projectSFID,
		"loadRepoDetails": loadRepoDetails,
		"tableName":       repo.claGroupTable}
	log.WithFields(f).Debugf("loading project")

	claGroupProject, err := repo.projectClaGroupRepo.GetClaGroupIDForProject(ctx, projectSFID)
	if err != nil {
		log.WithFields(f).Warnf("error fetching CLA Group ID for project, error: %v", err)
		return nil, err
	}

	log.WithFields(f).Debugf("found CLA Group ID: %s for project SFID: %s", claGroupProject.ClaGroupID, projectSFID)

	return repo.getCLAGroupByID(ctx, claGroupProject.ClaGroupID, loadRepoDetails, false)
}

// GetCLAGroupByName returns the project model associated for the specified project name
func (repo *repo) GetCLAGroupByName(ctx context.Context, projectName string) (*models.ClaGroup, error) {
	f := logrus.Fields{
		"functionName":   "project.repository.GetCLAGroupByName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectName":    projectName,
		"tableName":      repo.claGroupTable,
	}
	log.WithFields(f).Debugf("loading project")

	// This is the key we want to match
	condition := expression.Key("project_name_lower").Equal(expression.Value(strings.ToLower(projectName)))

	// Use the builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error building expression for CLAGroup query, projectName: %s", projectName)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.claGroupTable),
		IndexName:                 aws.String("project-name-lower-search-index"),
	}

	// Make the DynamoDB Query API call
	results, queryErr := repo.dynamoDBClient.Query(queryInput)
	if queryErr != nil {
		log.WithFields(f).Warnf("error retrieving project by projectName: %s, error: %v", projectName, queryErr)
		return nil, queryErr
	}

	// Should only have one result
	if *results.Count > 1 {
		log.WithFields(f).Warnf("CLAGroup scan by name returned more than one result using projectName: %s", projectName)
	}

	// Didn't find it...
	if *results.Count == 0 {
		log.WithFields(f).Debugf("CLAGroup scan by name returned no results using projectName: %s", projectName)
		return nil, nil
	}

	// Found it...
	var dbModel models2.DBProjectModel
	err = dynamodbattribute.UnmarshalMap(results.Items[0], &dbModel)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling db project model, error: %+v", err)
		return nil, err
	}

	// Convert the database model to an API response model
	return repo.buildCLAGroupModel(ctx, dbModel, LoadRepoDetails), nil
}

// GetExternalCLAGroup returns the project model associated for the specified external project ID
func (repo *repo) GetExternalCLAGroup(ctx context.Context, projectExternalID string) (*models.ClaGroup, error) {
	f := logrus.Fields{
		"functionName":      "project.repository.GetExternalCLAGroup",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"projectExternalID": projectExternalID,
		"tableName":         repo.claGroupTable}
	log.WithFields(f).Debugf("loading project")
	// This is the key we want to match
	condition := expression.Key("project_external_id").Equal(expression.Value(projectExternalID))

	// Use the builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for CLAGroup query, projectExternalID: %s, error: %v",
			projectExternalID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.claGroupTable),
		IndexName:                 aws.String("external-project-index"),
	}

	// Make the DynamoDB Query API call
	results, queryErr := repo.dynamoDBClient.Query(queryInput)
	if queryErr != nil {
		log.WithFields(f).Warnf("error retrieving project by projectExternalID: %s, error: %v", projectExternalID, queryErr)
		return nil, queryErr
	}

	// No match, didn't find it
	if *results.Count == 0 {
		return nil, nil
	}

	// Should only have one result
	if *results.Count > 1 {
		log.WithFields(f).Warnf("CLAGroup query returned more than one result using projectExternalID: %s", projectExternalID)
	}

	var dbModel models2.DBProjectModel
	err = dynamodbattribute.UnmarshalMap(results.Items[0], &dbModel)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling db project model, error: %+v", err)
		return nil, err
	}

	// Convert the database model to an API response model
	return repo.buildCLAGroupModel(ctx, dbModel, LoadRepoDetails), nil
}

// GetCLAGroups queries the database and returns a list of the projects
func (repo *repo) GetCLAGroups(ctx context.Context, params *project.GetProjectsParams) (*models.ClaGroups, error) {
	f := logrus.Fields{
		"functionName":   "project.repository.GetCLAGroups",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"searchField":    params.SearchField,
		"searchTerm":     params.SearchTerm,
		"nextKey":        params.NextKey,
		"pageSize":       params.PageSize,
		"fullMatch":      params.FullMatch,
		"tableName":      repo.claGroupTable}
	log.WithFields(f).Debugf("searching project")

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project scan, error: %v", err)
	}

	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.claGroupTable),
	}

	// If we have the next key, set the exclusive start key value
	if params.NextKey != nil && *params.NextKey != "" {
		log.WithFields(f).Debugf("Received a nextKey, value: %s", *params.NextKey)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"project_id": {
				S: params.NextKey,
			},
		}
	}
	var pageSize int64
	// If we have a page size, set the limit value - make sure it's a positive value
	if params.PageSize != nil && *params.PageSize > 0 {
		log.WithFields(f).Debugf("Received a pageSize parameter, value: %d", *params.PageSize)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		scanInput.Limit = params.PageSize
	} else {
		// Default page size
		pageSize = 50
		params.PageSize = &pageSize
	}

	var projects []models.ClaGroup
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		results, errQuery := repo.dynamoDBClient.Scan(scanInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving projects, error: %v", errQuery)
			return nil, errQuery
		}

		// Convert the list of DB models to a list of response models
		projectList, modelErr := repo.buildCLAGroupModels(ctx, results.Items, DontLoadRepoDetails)
		if modelErr != nil {
			log.WithFields(f).Warnf("error converting project DB model to response model, error: %v",
				modelErr)
			return nil, modelErr
		}

		// Add to the project response models to the list
		projects = append(projects, projectList...)

		if results.LastEvaluatedKey["project_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["project_id"].S
			scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"project_id": {
					S: aws.String(lastEvaluatedKey),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}

		if int64(len(projects)) >= *params.PageSize {
			break
		}
	}

	return &models.ClaGroups{
		LastKeyScanned: lastEvaluatedKey,
		PageSize:       *params.PageSize,
		Projects:       projects,
	}, nil
}

// DeleteCLAGroup deletes the CLAGroup by claGroupID
func (repo *repo) DeleteCLAGroup(ctx context.Context, claGroupID string) error {
	f := logrus.Fields{
		"functionName":   "project.repository.DeleteCLAGroup",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"tableName":      repo.claGroupTable}
	log.WithFields(f).Debugf("deleting CLA Group")

	existingCLAGroup, getErr := repo.GetCLAGroupByID(ctx, claGroupID, DontLoadRepoDetails)
	if getErr != nil {
		log.WithFields(f).Warnf("delete - error locating the CLA Group, error: %+v", getErr)
		return getErr
	}

	if existingCLAGroup == nil {
		log.WithFields(f).Warn("unable to locate CLA Group by ID - CLA Group does not exist")
		return ErrProjectDoesNotExist
	}

	var deleteErr error
	// Perform the delete
	_, deleteErr = repo.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String(repo.claGroupTable),
		Key: map[string]*dynamodb.AttributeValue{
			"project_id": {
				S: aws.String(existingCLAGroup.ProjectID),
			},
		},
	})

	if deleteErr != nil {
		log.WithFields(f).Warnf("Error deleting project with CLA Group ID : %s, error: %v", claGroupID, deleteErr)
		return deleteErr
	}

	return nil
}

// UpdateCLAGroup updates the project by claGroupID
func (repo *repo) UpdateCLAGroup(ctx context.Context, claGroupModel *models.ClaGroup) (*models.ClaGroup, error) {
	f := logrus.Fields{
		"functionName":            "project.repository.UpdateCLAGroup",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"ProjectID":               claGroupModel.ProjectID,
		"ProjectName":             claGroupModel.ProjectName,
		"FoundationSFID":          claGroupModel.FoundationSFID,
		"ProjectExternalID":       claGroupModel.ProjectExternalID,
		"ProjectICLAEnabled":      claGroupModel.ProjectICLAEnabled,
		"ProjectCCLAEnabled":      claGroupModel.ProjectCCLAEnabled,
		"ProjectCCLARequiresICLA": claGroupModel.ProjectCCLARequiresICLA,
		"ProjectTemplateID":       claGroupModel.ProjectTemplateID,
		"ProjectLive":             claGroupModel.ProjectLive,
		"tableName":               repo.claGroupTable}
	log.WithFields(f).Debugf("processing update CLA Group request")

	if claGroupModel.ProjectID == "" {
		return nil, ErrProjectIDMissing
	}

	existingCLAGroup, getErr := repo.GetCLAGroupByID(ctx, claGroupModel.ProjectID, DontLoadRepoDetails)
	if getErr != nil {
		log.WithFields(f).Warnf("update - error locating the project id: %s, error: %+v", claGroupModel.ProjectID, getErr)
		return nil, getErr
	}

	if existingCLAGroup == nil {
		return nil, ErrProjectDoesNotExist
	}

	expressionAttributeNames := map[string]*string{}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{}
	updateExpression := "SET "

	// An update to the CLA Group name...
	if claGroupModel.ProjectName != "" && existingCLAGroup.ProjectName != claGroupModel.ProjectName {
		log.WithFields(f).Debugf("adding project_name: %s", claGroupModel.ProjectName)
		expressionAttributeNames["#N"] = aws.String("project_name")
		expressionAttributeValues[":n"] = &dynamodb.AttributeValue{S: aws.String(claGroupModel.ProjectName)}
		updateExpression = updateExpression + " #N = :n, "
		log.WithFields(f).Debugf("adding project name lower: %s", strings.ToLower(claGroupModel.ProjectName))
		expressionAttributeNames["#LOW"] = aws.String("project_name_lower")
		expressionAttributeValues[":low"] = &dynamodb.AttributeValue{S: aws.String(strings.ToLower(claGroupModel.ProjectName))}
		updateExpression = updateExpression + " #LOW = :low, "
	}

	// An update to the CLA Group description...
	if existingCLAGroup.ProjectDescription != claGroupModel.ProjectDescription {
		log.WithFields(f).Debugf("adding project_description: %s", claGroupModel.ProjectDescription)
		expressionAttributeNames["#DESC"] = aws.String("project_description")
		expressionAttributeValues[":desc"] = &dynamodb.AttributeValue{S: aws.String(claGroupModel.ProjectDescription)}
		updateExpression = updateExpression + " #DESC = :desc, "
	}

	// An update to the project ACL
	if len(claGroupModel.ProjectACL) > 0 {
		log.WithFields(f).Debugf("adding project_acl: %s", claGroupModel.ProjectACL)
		expressionAttributeNames["#A"] = aws.String("project_acl")
		expressionAttributeValues[":a"] = &dynamodb.AttributeValue{SS: aws.StringSlice(claGroupModel.ProjectACL)}
		updateExpression = updateExpression + " #A = :a, "
	}

	// An update to the ICLA enabled flag
	if claGroupModel.ProjectICLAEnabled != existingCLAGroup.ProjectICLAEnabled {
		log.WithFields(f).Debugf("adding project_icla_enabled: %t", claGroupModel.ProjectICLAEnabled)
		expressionAttributeNames["#I"] = aws.String("project_icla_enabled")
		expressionAttributeValues[":i"] = &dynamodb.AttributeValue{BOOL: aws.Bool(claGroupModel.ProjectICLAEnabled)}
		updateExpression = updateExpression + " #I = :i, "
	}

	// An update to the CCLA enabled flag
	if claGroupModel.ProjectCCLAEnabled != existingCLAGroup.ProjectCCLAEnabled {
		log.WithFields(f).Debugf("adding project_ccla_enabled: %t", claGroupModel.ProjectCCLAEnabled)
		expressionAttributeNames["#C"] = aws.String("project_ccla_enabled")
		expressionAttributeValues[":c"] = &dynamodb.AttributeValue{BOOL: aws.Bool(claGroupModel.ProjectCCLAEnabled)}
		updateExpression = updateExpression + " #C = :c, "
	}

	// An update to the CCLA requires ICLA flag
	if claGroupModel.ProjectCCLARequiresICLA != existingCLAGroup.ProjectCCLARequiresICLA {
		log.WithFields(f).Debugf("adding project_ccla_requires_icla_signature: %t", claGroupModel.ProjectCCLARequiresICLA)
		expressionAttributeNames["#CI"] = aws.String("project_ccla_requires_icla_signature")
		expressionAttributeValues[":ci"] = &dynamodb.AttributeValue{BOOL: aws.Bool(claGroupModel.ProjectCCLARequiresICLA)}
		updateExpression = updateExpression + " #CI = :ci, "
	}

	// An update to the project live flag
	if claGroupModel.ProjectLive != existingCLAGroup.ProjectLive {
		log.WithFields(f).Debugf("adding project_live: %t", claGroupModel.ProjectLive)
		expressionAttributeNames["#PL"] = aws.String("project_live")
		expressionAttributeValues[":pl"] = &dynamodb.AttributeValue{BOOL: aws.Bool(claGroupModel.ProjectLive)}
		updateExpression = updateExpression + " #PL = :pl, "
	}

	// We'll update the date modified time
	_, currentTimeString := utils.CurrentTime()
	log.WithFields(f).Debugf("adding date_modified: %s", currentTimeString)
	expressionAttributeNames["#M"] = aws.String("date_modified")
	expressionAttributeValues[":m"] = &dynamodb.AttributeValue{S: aws.String(currentTimeString)}
	updateExpression = updateExpression + " #M = :m "

	// Assemble the query input parameters
	updateInput := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"project_id": {
				S: aws.String(existingCLAGroup.ProjectID),
			},
		},
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		UpdateExpression:          &updateExpression,
		TableName:                 aws.String(repo.claGroupTable),
	}

	// Make the DynamoDB Update API call
	_, updateErr := repo.dynamoDBClient.UpdateItem(updateInput)
	if updateErr != nil {
		log.WithFields(f).Warnf("error updating CLAGroup by claGroupID: %s, error: %v", claGroupModel.ProjectID, updateErr)
		return nil, updateErr
	}

	// Read the updated record back from the DB and return - probably could
	// just create/update a new model in memory and return it to make it fast,
	// but this approach return exactly what the DB has
	return repo.GetCLAGroupByID(ctx, claGroupModel.ProjectID, LoadRepoDetails)
}

func (repo *repo) UpdateRootCLAGroupRepositoriesCount(ctx context.Context, claGroupID string, diff int64, reset bool) error {
	f := logrus.Fields{
		"functionName":   "project.repository.UpdateRootCLAGroupRepositoriesCount",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"diff":           diff,
		"reset":          reset,
	}

	val := strconv.FormatInt(diff, 10)
	expressionAttributeNames := map[string]*string{}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{}
	var updateExpression string

	// update root_project_repositories based on reset flag
	if reset {
		expressionAttributeNames["#R"] = aws.String("root_project_repositories_count")
		expressionAttributeValues[":r"] = &dynamodb.AttributeValue{N: aws.String(val)}
		updateExpression = "SET #R = :r"

		_, now := utils.CurrentTime()
		expressionAttributeNames["#M"] = aws.String("date_modified")
		expressionAttributeValues[":m"] = &dynamodb.AttributeValue{S: aws.String(now)}
		updateExpression = updateExpression + ", #M = :m"
	} else {
		expressionAttributeValues[":val"] = &dynamodb.AttributeValue{N: aws.String(val)}
		updateExpression = "ADD root_project_repositories_count :val"
	}

	input := &dynamodb.UpdateItemInput{
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,

		Key: map[string]*dynamodb.AttributeValue{
			"project_id": {S: aws.String(claGroupID)},
		},

		TableName: aws.String(repo.claGroupTable),
	}

	_, err := repo.dynamoDBClient.UpdateItem(input)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to update repositories count")
	}

	return err
}

// buildCLAGroupModels converts the database response model into an API response data model
func (repo *repo) buildCLAGroupModels(ctx context.Context, results []map[string]*dynamodb.AttributeValue, loadRepoDetails bool) ([]models.ClaGroup, error) {
	var projects []models.ClaGroup

	// The DB project model
	var dbProjects []models2.DBProjectModel

	err := dynamodbattribute.UnmarshalListOfMaps(results, &dbProjects)
	if err != nil {
		log.Warnf("error unmarshalling projects from database, error: %v", err)
		return nil, err
	}

	// Create an output channel to receive the results
	responseChannel := make(chan *models.ClaGroup)

	// For each project, convert to a response model - using a go routine
	for _, dbProject := range dbProjects {
		go func(dbProject models2.DBProjectModel) {
			// Send the results to the output channel
			responseChannel <- repo.buildCLAGroupModel(ctx, dbProject, loadRepoDetails)
		}(dbProject)
	}

	// Append all the responses to our list
	for i := 0; i < len(dbProjects); i++ {
		projects = append(projects, *<-responseChannel)
	}

	return projects, nil
}

// buildCLAGroupModel maps the database model to the API response model
func (repo *repo) buildCLAGroupModel(ctx context.Context, dbModel models2.DBProjectModel, loadRepoDetails bool) *models.ClaGroup {

	var ghOrgs []*models.GithubRepositoriesGroupByOrgs
	var gerrits []*models.Gerrit

	if loadRepoDetails {
		if dbModel.ProjectID != "" {
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				var err error
				ghOrgs, err = repo.ghRepo.GitHubGetCLAGroupRepositoriesGroupByOrgs(ctx, dbModel.ProjectID, true)
				if err != nil {
					log.Warnf("buildPCLAGroupModel - unable to load GH organizations by project ID: %s, error: %+v",
						dbModel.ProjectID, err)
					// Reset to empty array
					ghOrgs = make([]*models.GithubRepositoriesGroupByOrgs, 0)
				}
			}()

			go func() {
				defer wg.Done()
				var err error
				var gerritsList *models.GerritList
				gerritsList, err = repo.gerritRepo.GetClaGroupGerrits(ctx, dbModel.ProjectID)
				if err != nil {
					log.Warnf("buildCLAGroupModel - unable to load Gerrit repositories by project ID: %s, error: %+v",
						dbModel.ProjectID, err)
					// Reset to empty array
					gerrits = make([]*models.Gerrit, 0)
					return
				}
				gerrits = gerritsList.List
			}()
			wg.Wait()
		} else {
			log.Warnf("buildCLAGroupModel - project ID missing for project '%s' - ID: %s - unable to load GH and Gerrit repository details",
				dbModel.ProjectName, dbModel.ProjectID)
		}
	}

	return &models.ClaGroup{
		ProjectID:                    dbModel.ProjectID,
		FoundationSFID:               dbModel.FoundationSFID,
		RootProjectRepositoriesCount: dbModel.RootProjectRepositoriesCount,
		ProjectExternalID:            dbModel.ProjectExternalID,
		ProjectName:                  dbModel.ProjectName,
		ProjectDescription:           dbModel.ProjectDescription,
		ProjectACL:                   dbModel.ProjectACL,
		ProjectCCLAEnabled:           dbModel.ProjectCclaEnabled,
		ProjectICLAEnabled:           dbModel.ProjectIclaEnabled,
		ProjectCCLARequiresICLA:      dbModel.ProjectCclaRequiresIclaSignature,
		ProjectTemplateID:            dbModel.ProjectTemplateID,
		ProjectLive:                  dbModel.ProjectLive,
		ProjectCorporateDocuments:    common.BuildCLAGroupDocumentModels(dbModel.ProjectCorporateDocuments),
		ProjectIndividualDocuments:   common.BuildCLAGroupDocumentModels(dbModel.ProjectIndividualDocuments),
		ProjectMemberDocuments:       common.BuildCLAGroupDocumentModels(dbModel.ProjectMemberDocuments),
		GithubRepositories:           ghOrgs,
		Gerrits:                      gerrits,
		DateCreated:                  dbModel.DateCreated,
		DateModified:                 dbModel.DateModified,
		Version:                      dbModel.Version,
	}
}
