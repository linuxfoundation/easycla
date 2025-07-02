// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package projects_cla_groups

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	v2ProjectService "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

// constants
const (
	CLAGroupIDIndex     = "cla-group-id-index"
	FoundationSFIDIndex = "foundation-sfid-index"
	NotDefined          = "Not Defined"
	NotFound            = "Not Found"
)

// errors
var (
	ErrProjectNotAssociatedWithClaGroup = errors.New("provided project is not associated with cla_group")
	ErrAssociationAlreadyExist          = errors.New("cla_group project association already exist")
	ErrCLAGroupDoesNotExist             = errors.New("cla group does not exist")
)

// Repository provides interface for interacting with project_cla_groups table
type Repository interface {
	GetClaGroupIDForProject(ctx context.Context, projectSFID string) (*ProjectClaGroup, error)
	GetProjectsIdsForClaGroup(ctx context.Context, claGroupID string) ([]*ProjectClaGroup, error)
	GetProjectsIdsForFoundation(ctx context.Context, foundationSFID string) ([]*ProjectClaGroup, error)
	GetProjectsIdsForAllFoundation(ctx context.Context) ([]*ProjectClaGroup, error)
	AssociateClaGroupWithProject(ctx context.Context, claGroupID string, projectSFID string, foundationSFID string) error
	RemoveProjectAssociatedWithClaGroup(ctx context.Context, claGroupID string, projectSFIDList []string, all bool) error
	GetCLAGroupNameByID(ctx context.Context, claGroupID string) (string, error)
	GetCLAGroup(ctx context.Context, claGroupID string) (*ProjectClaGroup, error)

	IsExistingFoundationLevelCLAGroup(ctx context.Context, foundationSFID string) (bool, error)
	IsAssociated(ctx context.Context, projectSFID string, claGroupID string) (bool, error)
	UpdateRepositoriesCount(ctx context.Context, projectSFID string, diff int64, reset bool) error
	UpdateClaGroupName(ctx context.Context, projectSFID string, claGroupName string) error
	SignedAtFoundation(ctx context.Context, claGroupID string) (bool, error)
}

type repo struct {
	tableName      string
	dynamoDBClient *dynamodb.DynamoDB
	stage          string
}

// NewRepository provides implementation of projects_cla_group repository
func NewRepository(awsSession *session.Session, stage string) Repository {
	return &repo{
		tableName:      fmt.Sprintf("cla-%s-projects-cla-groups", stage),
		dynamoDBClient: dynamodb.New(awsSession),
		stage:          stage,
	}
}

func (repo *repo) queryClaGroupsProjects(ctx context.Context, keyCondition expression.KeyConditionBuilder, indexName *string) ([]*ProjectClaGroup, error) {
	f := logrus.Fields{
		"functionName":   "project_cla_groups.repository.queryClaGroupsProjects",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"indexName":      aws.StringValue(indexName),
		"keyCondition":   fmt.Sprintf("%+v", keyCondition),
	}

	expr, err := expression.NewBuilder().WithKeyCondition(keyCondition).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project cla groups, error: %v", err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		TableName:                 aws.String(repo.tableName),
		IndexName:                 indexName,
	}

	var projectClaGroups []*ProjectClaGroup
	for {
		// log.WithFields(f).Debugf("running query using input: %+v", queryInput)
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving project cla-groups, error: %v", errQuery)
			return nil, errQuery
		}

		var projectClaGroupsTmp []*ProjectClaGroup

		err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &projectClaGroupsTmp)
		if err != nil {
			log.Warnf("error unmarshalling project cla-groups from database table: %s. error: %v", repo.tableName, err)
			return nil, err
		}
		projectClaGroups = append(projectClaGroups, projectClaGroupsTmp...)

		if len(results.LastEvaluatedKey) != 0 {
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}

	return projectClaGroups, nil
}

func (repo *repo) SignedAtFoundation(ctx context.Context, claGroupID string) (bool, error) {
	f := logrus.Fields{
		"functionName": "SignedAtFoundation",
		"claGroupID":   claGroupID,
	}
	pcgs, err := repo.GetProjectsIdsForClaGroup(ctx, claGroupID)
	if err != nil {
		return false, err
	}

	log.WithFields(f).Info("checking if claGroup is signed at foundation level..")
	for _, pcg := range pcgs {
		if pcg.FoundationSFID == pcg.ProjectSFID {
			return true, nil
		}
	}

	return false, nil
}

// GetClaGroupIDForProject retrieves the CLA Group ID for the project
func (repo *repo) GetClaGroupIDForProject(ctx context.Context, projectSFID string) (*ProjectClaGroup, error) {
	f := logrus.Fields{
		"functionName":   "project_cla_groups.repository.GetClaGroupIDForProject",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"tableName":      repo.tableName,
		"projectSFID":    projectSFID,
	}

	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(repo.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"project_sfid": {
				S: aws.String(projectSFID),
			},
		},
	})
	if err != nil {
		log.WithFields(f).Warnf("unable to lookup CLA Group associated with project, error: %+v", err)
		return nil, err
	}

	var out ProjectClaGroup
	if len(result.Item) == 0 {
		// Query by foundation sfid index returns multiple results
		log.WithFields(f).Debug("no results querying by project SFID - checking if this is a foundation SFID")
		pcgs, foundationErr := repo.GetProjectsIdsForFoundation(ctx, projectSFID)
		if foundationErr != nil {
			log.WithFields(f).Warnf("unable to lookup CLA Group associated with project, error: %+v", foundationErr)
			return nil, err
		}

		if len(pcgs) == 0 {
			log.WithFields(f).Warn("unable to lookup CLA Group associated with project - missing table entry")
			return nil, ErrProjectNotAssociatedWithClaGroup
		}
		out = *pcgs[0]
	}

	if len(result.Item) > 0 {
		err = dynamodbattribute.UnmarshalMap(result.Item, &out)
		if err != nil {
			log.WithFields(f).Warnf("unable decode results from database, error: %+v", err)
			return nil, err
		}
	}

	return &out, nil
}

func (repo *repo) GetProjectsIdsForClaGroup(ctx context.Context, claGroupID string) ([]*ProjectClaGroup, error) {
	keyCondition := expression.Key("cla_group_id").Equal(expression.Value(claGroupID))
	return repo.queryClaGroupsProjects(ctx, keyCondition, aws.String(CLAGroupIDIndex))
}

func (repo *repo) GetProjectsIdsForFoundation(ctx context.Context, foundationSFID string) ([]*ProjectClaGroup, error) {
	keyCondition := expression.Key("foundation_sfid").Equal(expression.Value(foundationSFID))
	return repo.queryClaGroupsProjects(ctx, keyCondition, aws.String(FoundationSFIDIndex))
}

func (repo *repo) GetProjectsIdsForAllFoundation(ctx context.Context) ([]*ProjectClaGroup, error) {
	f := logrus.Fields{
		"functionName":   "project_cla_groups.repository.GetProjectsIdsForAllFoundation",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"tableName":      repo.tableName,
	}

	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(repo.tableName),
	}
	var resultList []map[string]*dynamodb.AttributeValue
	for {
		results, err := repo.dynamoDBClient.Scan(scanInput) //nolint
		if err != nil {
			log.WithFields(f).Warnf("error retrieving %s, error: %v", repo.tableName, err)
			return nil, err
		}
		resultList = append(resultList, results.Items...)
		if len(results.LastEvaluatedKey) != 0 {
			scanInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}
	var output []*ProjectClaGroup
	err := dynamodbattribute.UnmarshalListOfMaps(resultList, &output)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling %s from database. error: %v", repo.tableName, err)
		return nil, err
	}
	return output, nil
}

// AssociateClaGroupWithProject creates entry in db to track cla_group association with project/foundation
func (repo *repo) AssociateClaGroupWithProject(ctx context.Context, claGroupID string, projectSFID string, foundationSFID string) error {
	f := logrus.Fields{
		"functionName":   "project_cla_groups.repository.AssociateClaGroupWithProject",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"projectSFID":    projectSFID,
		"foundationSFID": foundationSFID,
		"tableName":      repo.tableName,
		"stage":          repo.stage,
	}
	var foundationName = NotDefined
	// Lookup the foundation name
	projectServiceModel, projErr := v2ProjectService.GetClient().GetProject(foundationSFID)
	if projErr != nil {
		log.WithFields(f).Warnf("unable to lookup foundation by SFID from the platform project service, error: %+v - using value of: '%s'",
			projErr, NotDefined)
	} else {
		foundationName = projectServiceModel.Name
	}

	// Lookup the project name
	var projectName = NotDefined
	projectServiceModel, projErr = v2ProjectService.GetClient().GetProject(projectSFID)
	if projErr != nil {
		log.WithFields(f).Warnf("unable to lookup project by SFID from the platform project service, error: %+v - using '%s'",
			projErr, NotDefined)
	} else {
		projectName = projectServiceModel.Name
	}

	// Lookup the CLA Group name/Project Name
	claGroupName, claGroupLookupErr := repo.GetCLAGroupNameByID(ctx, claGroupID)
	if claGroupLookupErr != nil {
		claGroupName = NotDefined
		log.Warnf("unable to lookup CLA Group/Project by ID, error: %+v - using '%s'",
			claGroupLookupErr, NotDefined)
	}

	_, nowStr := utils.CurrentTime()
	item := map[string]*dynamodb.AttributeValue{
		"project_sfid": {
			S: aws.String(projectSFID),
		},
		"project_name": {
			S: aws.String(projectName),
		},
		"cla_group_id": {
			S: aws.String(claGroupID),
		},
		"cla_group_name": {
			S: aws.String(claGroupName),
		},
		"foundation_sfid": {
			S: aws.String(foundationSFID),
		},
		"foundation_name": {
			S: aws.String(foundationName),
		},
		"note": {
			S: aws.String(fmt.Sprintf("Associate CLA Group with project API request on: %s", nowStr)),
		},
		"version": {
			S: aws.String("v1"),
		},
		"date_created": {
			S: aws.String(nowStr),
		},
		"date_modified": {
			S: aws.String(nowStr),
		},
	}

	log.WithFields(f).Debug("Locating records with matching projectSFID...")
	existingRecord, lookupErr := repo.GetClaGroupIDForProject(ctx, projectSFID)
	if lookupErr != nil {
		log.WithFields(f).Warnf("cannot lookup record by projectSFID, error: %+v", lookupErr)
	}
	if existingRecord == nil {
		log.WithFields(f).Debug("no record found with matching projectSFID")
	} else {
		log.WithFields(f).Debugf("record found with matching projectSFID: %+v", existingRecord)
	}

	log.WithFields(f).Debugf("adding entry into the %s table with: %+v", repo.tableName, item)
	_, err := repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:                item,
		TableName:           aws.String(repo.tableName),
		ConditionExpression: aws.String("attribute_not_exists(project_sfid)"),
	})
	if err != nil {
		log.WithFields(f).Warnf("cannot create association entry of CLA Group and project SFID in the database, error: %+v", err)
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				return ErrAssociationAlreadyExist
			}
			return err
		}
	}

	return nil
}

// RemoveProjectAssociatedWithClaGroup removes all associated project with cla_group
func (repo *repo) RemoveProjectAssociatedWithClaGroup(ctx context.Context, claGroupID string, projectSFIDList []string, all bool) error {
	f := logrus.Fields{
		"functionName":    "project_cla_groups.repository.RemoveProjectAssociatedWithClaGroup",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"claGroupID":      claGroupID,
		"projectSFIDList": projectSFIDList,
		"all":             all,
	}
	list, err := repo.GetProjectsIdsForClaGroup(ctx, claGroupID)
	if err != nil {
		log.WithFields(f).Warnf("unable to fetch projects IDs for CLA Group, error: %+v", err)
		return err
	}
	var projectFilter *utils.StringSet
	if !all {
		projectFilter = utils.NewStringSetFromStringArray(projectSFIDList)
	}
	var errs []string
	for _, pr := range list {
		if !all && !projectFilter.Include(pr.ProjectSFID) {
			// ignore project not present in projectSFIDList
			continue
		}
		_, err = repo.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
			Key: map[string]*dynamodb.AttributeValue{
				"project_sfid": {S: aws.String(pr.ProjectSFID)},
			},
			TableName: aws.String(repo.tableName),
		})
		if err != nil {
			log.WithFields(f).Warnf("unable to delete cla_group_association cla_group_id: %s, project_sfid: %s",
				claGroupID, pr.ProjectSFID)
			errs = append(errs, err.Error())
		}
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, ","))
	}
	return nil
}

// GetCLAGroupNameByID helper function to fetch the CLA Group name
func (repo *repo) GetCLAGroupNameByID(ctx context.Context, claGroupID string) (string, error) {
	tableName := fmt.Sprintf("cla-%s-projects", repo.stage)
	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"project_id": {
				S: aws.String(claGroupID),
			},
		},
	})
	if err != nil {
		return NotFound, err
	}
	if len(result.Item) == 0 {
		return NotFound, ErrCLAGroupDoesNotExist
	}

	var claGroupModel claGroupIDNameModel
	err = dynamodbattribute.UnmarshalMap(result.Item, &claGroupModel)
	if err != nil {
		return NotFound, err
	}

	return claGroupModel.ProjectName, nil
}

// GetCLAGroup helper function to fetch the CLA Group
func (repo *repo) GetCLAGroup(ctx context.Context, claGroupID string) (*ProjectClaGroup, error) {
	tableName := fmt.Sprintf("cla-%s-projects", repo.stage)
	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"project_id": {
				S: aws.String(claGroupID),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(result.Item) == 0 {
		return nil, ErrCLAGroupDoesNotExist
	}

	var claGroupModel ProjectClaGroup
	err = dynamodbattribute.UnmarshalMap(result.Item, &claGroupModel)
	if err != nil {
		return nil, err
	}

	return &claGroupModel, nil
}

// UpdateRepositoriesCount updates the repositories count
func (repo *repo) UpdateRepositoriesCount(ctx context.Context, projectSFID string, diff int64, reset bool) error {
	f := logrus.Fields{
		"functionName":   "project_cla_groups.repository.UpdateRepositoriesCount",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"diff":           diff,
		"reset":          reset,
	}

	// Check to see if we have an existing record
	existingProjectCLAGroupMapping, err := repo.GetClaGroupIDForProject(ctx, projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup existing project cla group mapping")
		return err
	}
	if existingProjectCLAGroupMapping == nil {
		log.WithFields(f).Warn("unable to lookup existing project cla group mapping - response is empty")
		return &utils.ProjectCLAGroupMappingNotFound{
			ProjectSFID: projectSFID,
			CLAGroupID:  "",
			Err:         nil,
		}
	}

	// TODO: DAD remove the above check and use the DB key exists condition

	val := strconv.FormatInt(diff, 10)
	expressionAttributeNames := map[string]*string{}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{}
	var updateExpression string

	// update repositories_count based on reset flag
	if reset {
		expressionAttributeNames["#R"] = aws.String("repositories_count")
		expressionAttributeValues[":r"] = &dynamodb.AttributeValue{N: aws.String(val)}
		updateExpression = "SET #R = :r"

		_, now := utils.CurrentTime()
		expressionAttributeNames["#M"] = aws.String("date_modified")
		expressionAttributeValues[":m"] = &dynamodb.AttributeValue{S: aws.String(now)}
		updateExpression = updateExpression + ", #M = :m"
	} else {
		expressionAttributeValues[":val"] = &dynamodb.AttributeValue{N: aws.String(val)}
		updateExpression = "ADD repositories_count :val"
	}

	_, updateErr := repo.dynamoDBClient.UpdateItem(&dynamodb.UpdateItemInput{
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		Key: map[string]*dynamodb.AttributeValue{
			"project_sfid": {S: aws.String(projectSFID)},
		},
		TableName: aws.String(repo.tableName),
	})

	if updateErr != nil {
		log.WithFields(f).WithError(updateErr).Warn("update repositories count failed")
	}

	return updateErr
}

// UpdateClaGroupName updates cla group name for given projectSFID
func (repo *repo) UpdateClaGroupName(ctx context.Context, projectSFID string, claGroupName string) error {
	f := logrus.Fields{
		"functionName":   "project_cla_groups.repository.UpdateClaGroupName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"claGroupName":   claGroupName,
	}

	// Check to see if we have an existing record
	existingProjectCLAGroupMapping, err := repo.GetClaGroupIDForProject(ctx, projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup existing project cla group mapping")
		return err
	}
	if existingProjectCLAGroupMapping == nil {
		log.WithFields(f).Warn("unable to lookup existing project cla group mapping - response is empty")
		return &utils.ProjectCLAGroupMappingNotFound{
			ProjectSFID: projectSFID,
			CLAGroupID:  "",
			Err:         nil,
		}
	}

	expressionAttributeNames := map[string]*string{}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{}
	var updateExpression string

	// update repositories_count based on reset flag
	expressionAttributeNames["#N"] = aws.String("cla_group_name")
	expressionAttributeValues[":n"] = &dynamodb.AttributeValue{S: &claGroupName}
	updateExpression = "SET #N = :n"

	_, now := utils.CurrentTime()
	expressionAttributeNames["#M"] = aws.String("date_modified")
	expressionAttributeValues[":m"] = &dynamodb.AttributeValue{S: aws.String(now)}
	updateExpression = updateExpression + ", #M = :m"

	_, updateErr := repo.dynamoDBClient.UpdateItem(&dynamodb.UpdateItemInput{
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		Key: map[string]*dynamodb.AttributeValue{
			"project_sfid": {S: aws.String(projectSFID)},
		},
		TableName: aws.String(repo.tableName),
	})

	if updateErr != nil {
		log.WithFields(f).WithError(updateErr).Warn("update cla group name failed")
	}

	return updateErr
}

// IsExistingFoundationLevelCLAGroup is a query helper function to determine if the
// specified foundation SFID has an entry in the mapping table to signify that
// it's a foundation level CLA Group (foundationSFID == projectSFID)
func (repo *repo) IsExistingFoundationLevelCLAGroup(ctx context.Context, foundationSFID string) (bool, error) {
	projectCLAGroupModels, err := repo.GetProjectsIdsForFoundation(ctx, foundationSFID)
	if err != nil {
		return false, err
	}

	for _, projectCLAGroupModel := range projectCLAGroupModels {
		if projectCLAGroupModel.FoundationSFID == foundationSFID && projectCLAGroupModel.ProjectSFID == foundationSFID {
			return true, nil
		}
	}

	return false, nil
}

func (repo *repo) IsAssociated(ctx context.Context, projectSFID string, claGroupID string) (bool, error) {
	pmlist, err := repo.GetProjectsIdsForClaGroup(ctx, claGroupID)
	if err != nil {
		return false, err
	}
	if len(pmlist) == 0 {
		return false, errors.New("no cla-group mapping found for cla-group")
	}
	for _, pm := range pmlist {
		if pm.ProjectSFID == projectSFID || pm.FoundationSFID == projectSFID {
			return true, nil
		}
	}

	return false, nil
}
