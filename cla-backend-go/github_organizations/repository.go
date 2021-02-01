// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_organizations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// indexes
const (
	GithubOrgSFIDIndex               = "github-org-sfid-index"
	GithubOrgLowerNameIndex          = "organization-name-lower-search-index"
	ProjectSFIDOrganizationNameIndex = "project-sfid-organization-name-index"
)

// errors
var (
	ErrOrganizationDoesNotExist = errors.New("github organization does not exist in cla")
)

// Repository interface defines the functions for the github organizations data model
type Repository interface {
	AddGithubOrganization(ctx context.Context, parentProjectSFID string, projectSFID string, input *models.CreateGithubOrganization) (*models.GithubOrganization, error)
	GetGithubOrganizations(ctx context.Context, projectSFID string) (*models.GithubOrganizations, error)
	GetGithubOrganizationsByParent(ctx context.Context, parentProjectSFID string) (*models.GithubOrganizations, error)
	GetGithubOrganization(ctx context.Context, githubOrganizationName string) (*models.GithubOrganization, error)
	GetGithubOrganizationByName(ctx context.Context, githubOrganizationName string) (*models.GithubOrganizations, error)
	UpdateGithubOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error
	DeleteGithubOrganization(ctx context.Context, projectSFID string, githubOrgName string) error
	DeleteGithubOrganizationByParent(ctx context.Context, parentProjectSFID string, githubOrgName string) error
}

type repository struct {
	stage              string
	dynamoDBClient     *dynamodb.DynamoDB
	githubOrgTableName string
}

// NewRepository creates a new instance of the githubOrganizations repository
func NewRepository(awsSession *session.Session, stage string) repository {
	return repository{
		stage:              stage,
		dynamoDBClient:     dynamodb.New(awsSession),
		githubOrgTableName: fmt.Sprintf("cla-%s-github-orgs", stage),
	}
}

// AddGithubOrganization add github organization logic
func (repo repository) AddGithubOrganization(ctx context.Context, parentProjectSFID string, projectSFID string, input *models.CreateGithubOrganization) (*models.GithubOrganization, error) {
	f := logrus.Fields{
		"functionName":            "AddGitHubOrganization",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"parentProjectSFID":       parentProjectSFID,
		"projectSFID":             projectSFID,
		"organizationName":        utils.StringValue(input.OrganizationName),
		"autoEnabled":             utils.BoolValue(input.AutoEnabled),
		"branchProtectionEnabled": utils.BoolValue(input.BranchProtectionEnabled),
	}

	// First, let's check to see if we have an existing github organization with the same name
	existingRecord, getErr := repo.GetGithubOrganizationByName(ctx, utils.StringValue(input.OrganizationName))
	if getErr != nil {
		log.WithFields(f).WithError(getErr).Debug("unable to locate existing github organization by name")
	}

	if existingRecord != nil && len(existingRecord.List) > 0 {
		log.WithFields(f).Debugf("Existing github organization exists in our database, count: %d", len(existingRecord.List))
		if len(existingRecord.List) > 1 {
			log.WithFields(f).Warning("more than one github organization with the same name in the database")
		}
		if parentProjectSFID == existingRecord.List[0].OrganizationSfid {
			log.WithFields(f).Debug("Existing github organization with same parent SFID - should be able to update it")
		} else {
			log.WithFields(f).Debug("Existing github organization with different parent SFID - won't be able to update it - will return conflict")
		}
	}

	// Existing record with the same GH organization name and the same parent...update it
	if existingRecord != nil &&
		len(existingRecord.List) == 1 &&
		parentProjectSFID == existingRecord.List[0].OrganizationSfid {

		// These are our rules for updating
		autoEnabled := existingRecord.List[0].AutoEnabled || utils.BoolValue(input.AutoEnabled)
		branchProtectionEnabled := existingRecord.List[0].BranchProtectionEnabled || utils.BoolValue(input.BranchProtectionEnabled)

		// Only update if previous value was unset
		autoEnabledCLAGroupID := existingRecord.List[0].AutoEnabledClaGroupID
		if autoEnabledCLAGroupID == "" && input.AutoEnabledClaGroupID != "" {
			autoEnabledCLAGroupID = input.AutoEnabledClaGroupID
		}

		// Attempt to simply update the existing record - we should only have one
		updateErr := repo.UpdateGithubOrganization(ctx,
			projectSFID,
			utils.StringValue(input.OrganizationName),
			autoEnabled,
			autoEnabledCLAGroupID,
			branchProtectionEnabled,
		)
		if updateErr != nil {
			log.WithFields(f).WithError(updateErr).Warn("unable to update existing github organization record")
			return nil, updateErr
		}

		// we could simply update the record we initially loaded or simply query the updated record again...
		// we're using a key lookup, so it should be fast...
		existingUpdatedRecord, getUpdatedRecordErr := repo.GetGithubOrganizationByName(ctx, utils.StringValue(input.OrganizationName))
		if getUpdatedRecordErr != nil {
			log.WithFields(f).WithError(getUpdatedRecordErr).Warn("unable to locate existing github organization by name")
			return nil, getUpdatedRecordErr
		}
		// this would be odd...
		if len(existingRecord.List) == 0 {
			log.WithFields(f).Warn("unable to locate existing github organization by name")
			return nil, &utils.GitHubOrgNotFound{
				ProjectSFID:      projectSFID,
				OrganizationName: utils.StringValue(input.OrganizationName),
				Err:              fmt.Errorf("organization name not found: %s", utils.StringValue(input.OrganizationName)),
			}
		}
		return existingUpdatedRecord.List[0], nil
	}

	// No existing records - create one
	_, currentTime := utils.CurrentTime()
	githubOrg := &GithubOrganization{
		DateCreated:                currentTime,
		DateModified:               currentTime,
		OrganizationInstallationID: 0,
		OrganizationName:           *input.OrganizationName,
		OrganizationNameLower:      strings.ToLower(*input.OrganizationName),
		OrganizationSFID:           parentProjectSFID,
		ProjectSFID:                projectSFID,
		AutoEnabled:                aws.BoolValue(input.AutoEnabled),
		AutoEnabledClaGroupID:      input.AutoEnabledClaGroupID,
		BranchProtectionEnabled:    aws.BoolValue(input.BranchProtectionEnabled),
		Version:                    "v1",
	}

	log.WithFields(f).Debug("Encoding github organization record for adding to the database...")
	av, err := dynamodbattribute.MarshalMap(githubOrg)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to marshall request for query")
		return nil, err
	}

	log.WithFields(f).Debug("Adding github organization record to the database...")
	_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:                av,
		TableName:           aws.String(repo.githubOrgTableName),
		ConditionExpression: aws.String("attribute_not_exists(organization_name)"),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				log.WithFields(f).WithError(err).Warn("github organization already exists")
				return nil, errors.New("github organization already exists")
			}
		}
		log.WithFields(f).WithError(err).Warn("cannot put github organization in dynamodb")
		return nil, err
	}

	return ToModel(githubOrg), nil
}

// GetGithubOrganizations get github organizations based on the project SFID
func (repo repository) GetGithubOrganizations(ctx context.Context, projectSFID string) (*models.GithubOrganizations, error) {
	f := logrus.Fields{
		"functionName":   "GetGitHubOrganizations",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
	}

	condition := expression.Key("project_sfid").Equal(expression.Value(projectSFID))
	builder := expression.NewBuilder().WithKeyCondition(condition)

	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).Warnf("problem building query expression, error: %+v", err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.githubOrgTableName),
		IndexName:                 aws.String(ProjectSFIDOrganizationNameIndex),
	}

	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving github_organizations using project_sfid = %s. error = %s", projectSFID, err.Error())
		return nil, err
	}

	if len(results.Items) == 0 {
		log.WithFields(f).Debug("no results from query")
		return &models.GithubOrganizations{
			List: []*models.GithubOrganization{},
		}, nil
	}

	var resultOutput []*GithubOrganization
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &resultOutput)
	if err != nil {
		return nil, err
	}

	log.WithFields(f).Debug("building response model...")
	ghOrgList := buildGithubOrganizationListModels(ctx, resultOutput)
	return &models.GithubOrganizations{List: ghOrgList}, nil
}

func (repo repository) GetGithubOrganizationsByParent(ctx context.Context, parentProjectSFID string) (*models.GithubOrganizations, error) {
	f := logrus.Fields{
		"functionName":      "GetGithubOrganizationsByParent",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"parentProjectSFID": parentProjectSFID,
	}

	condition := expression.Key("organization_sfid").Equal(expression.Value(parentProjectSFID))
	builder := expression.NewBuilder().WithKeyCondition(condition)

	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.githubOrgTableName),
		IndexName:                 aws.String(GithubOrgSFIDIndex),
	}

	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving github_organizations using organization_sfid = %s, error = %+v", parentProjectSFID, err)
		return nil, err
	}

	if len(results.Items) == 0 {
		log.WithFields(f).Debug("no results from query")
		return &models.GithubOrganizations{
			List: []*models.GithubOrganization{},
		}, nil
	}

	var resultOutput []*GithubOrganization
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &resultOutput)
	if err != nil {
		return nil, err
	}

	log.WithFields(f).Debug("building response model...")
	ghOrgList := buildGithubOrganizationListModels(ctx, resultOutput)
	return &models.GithubOrganizations{List: ghOrgList}, nil
}

func (repo repository) GetGithubOrganizationByName(ctx context.Context, githubOrganizationName string) (*models.GithubOrganizations, error) {
	f := logrus.Fields{
		"functionName":           "GetGitHubOrganizationByName",
		utils.XREQUESTID:         ctx.Value(utils.XREQUESTID),
		"githubOrganizationName": githubOrganizationName,
	}

	condition := expression.Key("organization_name_lower").Equal(expression.Value(strings.ToLower(githubOrganizationName)))
	builder := expression.NewBuilder().WithKeyCondition(condition)
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}
	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.githubOrgTableName),
		IndexName:                 aws.String(GithubOrgLowerNameIndex),
	}

	log.WithFields(f).Debugf("querying for github organization by name using organization_name_lower=%s...", strings.ToLower(githubOrganizationName))
	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error retrieving github_organizations using githubOrganizationName = %s", githubOrganizationName)
		return nil, err
	}
	if len(results.Items) == 0 {
		log.WithFields(f).Debug("Unable to find github organization by name - no results")
		return &models.GithubOrganizations{
			List: []*models.GithubOrganization{},
		}, nil
	}
	var resultOutput []*GithubOrganization
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &resultOutput)
	if err != nil {
		log.WithFields(f).Warnf("problem decoding database results, error: %+v", err)
		return nil, err
	}

	ghOrgList := buildGithubOrganizationListModels(ctx, resultOutput)
	return &models.GithubOrganizations{List: ghOrgList}, nil
}

func (repo repository) GetGithubOrganization(ctx context.Context, githubOrganizationName string) (*models.GithubOrganization, error) {
	f := logrus.Fields{
		"functionName":           "GetGitHubOrganization",
		utils.XREQUESTID:         ctx.Value(utils.XREQUESTID),
		"githubOrganizationName": githubOrganizationName,
	}

	log.WithFields(f).Debug("Querying for github organization by name...")
	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"organization_name": {
				S: aws.String(githubOrganizationName),
			},
		},
		TableName: aws.String(repo.githubOrgTableName),
	})
	if err != nil {
		return nil, err
	}
	if len(result.Item) == 0 {
		log.WithFields(f).Debug("Unable to find github organization by name - no results")
		return nil, ErrOrganizationDoesNotExist
	}

	var org GithubOrganization
	err = dynamodbattribute.UnmarshalMap(result.Item, &org)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling organization table data, error: %v", err)
		return nil, err
	}
	return ToModel(&org), nil
}

// UpdateGithubOrganization updates the specified GitHub organization based on the update model provided
func (repo repository) UpdateGithubOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool) error {
	f := logrus.Fields{
		"functionName":            "UpdateGitHubOrganization",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"projectSFID":             projectSFID,
		"organizationName":        organizationName,
		"autoEnabled":             autoEnabled,
		"autoEnabledClaGroupID":   autoEnabledClaGroupID,
		"branchProtectionEnabled": branchProtectionEnabled,
		"tableName":               repo.githubOrgTableName,
	}

	_, currentTime := utils.CurrentTime()
	githubOrg, lookupErr := repo.GetGithubOrganization(ctx, organizationName)
	if lookupErr != nil {
		log.WithFields(f).Warnf("error looking up GitHub organization by name, error: %+v", lookupErr)
		return lookupErr
	}
	if githubOrg == nil {
		lookupErr := errors.New("unable to lookup GitHub organization by name")
		log.WithFields(f).Warnf("error looking up GitHub organization, error: %+v", lookupErr)
		return lookupErr
	}

	input := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"organization_name": {
				S: aws.String(githubOrg.OrganizationName),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#A": aws.String("auto_enabled"),
			"#C": aws.String("auto_enabled_cla_group_id"),
			"#B": aws.String("branch_protection_enabled"),
			"#M": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":a": {
				BOOL: aws.Bool(autoEnabled),
			},
			":c": {
				S: aws.String(autoEnabledClaGroupID),
			},
			":b": {
				BOOL: aws.Bool(branchProtectionEnabled),
			},
			":m": {
				S: aws.String(currentTime),
			},
		},
		UpdateExpression: aws.String("SET #A = :a, #C = :c, #B = :b, #M = :m"),
		TableName:        aws.String(repo.githubOrgTableName),
	}

	log.WithFields(f).Debug("updating github organization record...")
	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("unable to update GitHub organization record, error: %+v", updateErr)
		return updateErr
	}

	return nil
}

func (repo repository) DeleteGithubOrganization(ctx context.Context, projectSFID string, githubOrgName string) error {
	f := logrus.Fields{
		"functionName":   "DeleteGitHubOrganization",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"githubOrgName":  githubOrgName,
	}

	var githubOrganizationName string
	orgs, orgErr := repo.GetGithubOrganizations(ctx, projectSFID)
	if orgErr != nil {
		errMsg := fmt.Sprintf("github organization is not found using projectSFID: %s, error: %+v", projectSFID, orgErr)
		log.WithFields(f).Warn(errMsg)
		return errors.New(errMsg)
	}

	for _, githubOrg := range orgs.List {
		if strings.EqualFold(githubOrg.OrganizationName, githubOrgName) {
			githubOrganizationName = githubOrg.OrganizationName
		}
	}

	log.WithFields(f).Debug("Deleting GitHub organization...")
	_, err := repo.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"organization_name": {
				S: aws.String(githubOrganizationName),
			},
		},
		TableName: aws.String(repo.githubOrgTableName),
	})
	if err != nil {
		errMsg := fmt.Sprintf("error deleting github organization: %s - %+v", githubOrgName, err)
		log.WithFields(f).Warnf(errMsg)
		return errors.New(errMsg)
	}

	return nil
}

func (repo repository) DeleteGithubOrganizationByParent(ctx context.Context, parentProjectSFID string, githubOrgName string) error {
	f := logrus.Fields{
		"functionName":      "DeleteGitHubOrganization",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"parentProjectSFID": parentProjectSFID,
		"githubOrgName":     githubOrgName,
	}

	var githubOrganizationName string
	orgs, orgErr := repo.GetGithubOrganizationsByParent(ctx, parentProjectSFID)
	if orgErr != nil {
		errMsg := fmt.Sprintf("github organization is not found using parentProjectSFID %s, error: - %+v", parentProjectSFID, orgErr)
		log.WithFields(f).Warn(errMsg)
		return errors.New(errMsg)
	}

	for _, githubOrg := range orgs.List {
		if strings.EqualFold(githubOrg.OrganizationName, githubOrgName) {
			githubOrganizationName = githubOrg.OrganizationName
		}
	}

	log.WithFields(f).Debug("Deleting GitHub organization...")
	_, err := repo.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"organization_name": {
				S: aws.String(githubOrganizationName),
			},
		},
		TableName: aws.String(repo.githubOrgTableName),
	})
	if err != nil {
		errMsg := fmt.Sprintf("error deleting github organization: %s - %+v", githubOrgName, err)
		log.WithFields(f).Warnf(errMsg)
		return errors.New(errMsg)
	}

	return nil
}
