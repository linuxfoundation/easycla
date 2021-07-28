// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_organizations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gofrs/uuid"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	models2 "github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// indexes
const (
	GitlabOrgSFIDIndex                     = "gitlab-org-sfid-index"
	GitlabOrgLowerNameIndex                = "gitlab-organization-name-lower-search-index"
	GitlabProjectSFIDOrganizationNameIndex = "gitlab-project-sfid-organization-name-index"
)

// RepositoryInterface is interface for gitlab org data model
type RepositoryInterface interface {
	AddGitlabOrganization(ctx context.Context, parentProjectSFID string, projectSFID string, input *models2.CreateGitlabOrganization) (*models2.GitlabOrganization, error)
	GetGitlabOrganizations(ctx context.Context, projectSFID string) (*models2.GitlabOrganizations, error)
	GetGitlabOrganization(ctx context.Context, gitlabOrganizationID string) (*GitlabOrganization, error)
	UpdateGitlabOrganizationAuth(ctx context.Context, gitlabOrganizationID, authInfo string) error
	UpdateGitlabOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool, enabled *bool) error
	DeleteGitlabOrganization(ctx context.Context, projectSFID, gitlabOrgName string) error
}

// Repository object/struct
type Repository struct {
	stage              string
	dynamoDBClient     *dynamodb.DynamoDB
	gitlabOrgTableName string
}

// NewRepository creates a new instance of the gitlabOrganizations repository
func NewRepository(awsSession *session.Session, stage string) RepositoryInterface {
	return Repository{
		stage:              stage,
		dynamoDBClient:     dynamodb.New(awsSession),
		gitlabOrgTableName: fmt.Sprintf("cla-%s-gitlab-orgs", stage),
	}
}

func (repo Repository) AddGitlabOrganization(ctx context.Context, parentProjectSFID string, projectSFID string, input *models2.CreateGitlabOrganization) (*models2.GitlabOrganization, error) {
	f := logrus.Fields{
		"functionName":            "v2.gitlab_organizations.repository.AddGitlabOrganization",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"parentProjectSFID":       parentProjectSFID,
		"projectSFID":             projectSFID,
		"organizationName":        utils.StringValue(input.OrganizationName),
		"autoEnabled":             utils.BoolValue(input.AutoEnabled),
		"branchProtectionEnabled": utils.BoolValue(input.BranchProtectionEnabled),
	}

	// First, let's check to see if we have an existing gitlab organization with the same name
	existingRecord, getErr := repo.GetGitlabOrganizationByName(ctx, utils.StringValue(input.OrganizationName))
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
		return nil, fmt.Errorf("record already exists")
	}

	// No existing records - create one
	_, currentTime := utils.CurrentTime()
	organizationID, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Unable to generate a UUID for gitlab org, error: %v", err)
		return nil, err
	}

	authStateNonce, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Unable to generate a auth nonce UUID for gitlab org, error: %v", err)
		return nil, err
	}

	enabled := true
	gitlabOrg := &GitlabOrganization{
		OrganizationID:          organizationID.String(),
		DateCreated:             currentTime,
		DateModified:            currentTime,
		OrganizationName:        *input.OrganizationName,
		OrganizationNameLower:   strings.ToLower(*input.OrganizationName),
		OrganizationSFID:        parentProjectSFID,
		ProjectSFID:             projectSFID,
		Enabled:                 aws.BoolValue(&enabled),
		AutoEnabled:             aws.BoolValue(input.AutoEnabled),
		AutoEnabledClaGroupID:   input.AutoEnabledClaGroupID,
		BranchProtectionEnabled: aws.BoolValue(input.BranchProtectionEnabled),
		AuthState:               authStateNonce.String(),
		Version:                 "v1",
	}

	log.WithFields(f).Debug("Encoding github organization record for adding to the database...")
	av, err := dynamodbattribute.MarshalMap(gitlabOrg)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to marshall request for query")
		return nil, err
	}

	log.WithFields(f).Debug("Adding gitlab organization record to the database...")
	_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:                av,
		TableName:           aws.String(repo.gitlabOrgTableName),
		ConditionExpression: aws.String("attribute_not_exists(organization_name)"),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				log.WithFields(f).WithError(err).Warn("gitlab organization already exists")
				return nil, fmt.Errorf("gitlab organization already exists")
			}
		}
		log.WithFields(f).WithError(err).Warn("cannot put gitlab organization in dynamodb")
		return nil, err
	}

	return ToModel(gitlabOrg), nil
}

// GetGitlabOrganizations get github organizations based on the project SFID
func (repo Repository) GetGitlabOrganizations(ctx context.Context, projectSFID string) (*models2.GitlabOrganizations, error) {
	f := logrus.Fields{
		"functionName":   "v2.gitlab_organizations.repository.GetGitHubOrganizations",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
	}

	condition := expression.Key("organization_sfid").Equal(expression.Value(projectSFID))
	builder := expression.NewBuilder().WithKeyCondition(condition)

	filter := expression.Name("enabled").Equal(expression.Value(true))
	builder = builder.WithFilter(filter)

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
		TableName:                 aws.String(repo.gitlabOrgTableName),
		IndexName:                 aws.String(GitlabOrgSFIDIndex),
	}

	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving github_organizations using project_sfid = %s. error = %s", projectSFID, err.Error())
		return nil, err
	}

	if len(results.Items) == 0 {
		log.WithFields(f).Debug("no results from query")
		return &models2.GitlabOrganizations{
			List: []*models2.GitlabOrganization{},
		}, nil
	}

	var resultOutput []*GitlabOrganization
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &resultOutput)
	if err != nil {
		return nil, err
	}

	log.WithFields(f).Debug("building response model...")
	gitlabOrgList := buildGitlabOrganizationListModels(ctx, resultOutput)
	return &models2.GitlabOrganizations{List: gitlabOrgList}, nil
}

// GetGitlabOrganizationByName get github organization by name
func (repo Repository) GetGitlabOrganizationByName(ctx context.Context, githubOrganizationName string) (*models2.GitlabOrganizations, error) {
	f := logrus.Fields{
		"functionName":           "v1.github_organizations.repository.GetGitHubOrganizationByName",
		utils.XREQUESTID:         ctx.Value(utils.XREQUESTID),
		"githubOrganizationName": githubOrganizationName,
	}

	githubOrganizationName = strings.ToLower(githubOrganizationName)

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
		TableName:                 aws.String(repo.gitlabOrgTableName),
		IndexName:                 aws.String(GitlabOrgLowerNameIndex),
	}

	log.WithFields(f).Debugf("querying for github organization by name using organization_name_lower=%s...", strings.ToLower(githubOrganizationName))
	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error retrieving github_organizations using githubOrganizationName = %s", githubOrganizationName)
		return nil, err
	}
	if len(results.Items) == 0 {
		log.WithFields(f).Debug("Unable to find github organization by name - no results")
		return &models2.GitlabOrganizations{
			List: []*models2.GitlabOrganization{},
		}, nil
	}
	var resultOutput []*GitlabOrganization
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &resultOutput)
	if err != nil {
		log.WithFields(f).Warnf("problem decoding database results, error: %+v", err)
		return nil, err
	}

	ghOrgList := buildGitlabOrganizationListModels(ctx, resultOutput)
	return &models2.GitlabOrganizations{List: ghOrgList}, nil
}

// GetGitlabOrganization by organization name
func (repo Repository) GetGitlabOrganization(ctx context.Context, gitlabOrganizationID string) (*GitlabOrganization, error) {
	f := logrus.Fields{
		"functionName":         "gitlab_organizations.repository.GetGitlabOrganization",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"gitlabOrganizationID": gitlabOrganizationID,
	}

	log.WithFields(f).Debug("Querying for github organization by name...")
	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"organization_id": {
				S: aws.String(gitlabOrganizationID),
			},
		},
		TableName: aws.String(repo.gitlabOrgTableName),
	})
	if err != nil {
		return nil, err
	}
	if len(result.Item) == 0 {
		log.WithFields(f).Debug("Unable to find github organization by name - no results")
		return nil, nil
	}

	var org GitlabOrganization
	err = dynamodbattribute.UnmarshalMap(result.Item, &org)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling organization table data, error: %v", err)
		return nil, err
	}
	return &org, nil
}

// UpdateGitlabOrganizationAuth updates the specified Gitlab organization oauth info
func (repo Repository) UpdateGitlabOrganizationAuth(ctx context.Context, gitlabOrganizationID, authInfo string) error {
	f := logrus.Fields{
		"functionName":         "gitlab_organizations.repository.UpdateGitlabOrganizationAuth",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"gitlabOrganizationID": gitlabOrganizationID,
		"tableName":            repo.gitlabOrgTableName,
	}

	_, currentTime := utils.CurrentTime()
	gitlabOrg, lookupErr := repo.GetGitlabOrganization(ctx, gitlabOrganizationID)
	if lookupErr != nil {
		log.WithFields(f).Warnf("error looking up Gitlab organization by id, error: %+v", lookupErr)
		return lookupErr
	}
	if gitlabOrg == nil {
		lookupErr := errors.New("unable to lookup Gitlab organization by id")
		log.WithFields(f).Warnf("error looking up Gitlab organization, error: %+v", lookupErr)
		return lookupErr
	}

	expressionAttributeNames := map[string]*string{
		"#A": aws.String("auth_info"),
		"#M": aws.String("date_modified"),
	}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{
		":a": {
			S: aws.String(authInfo),
		},
		":m": {
			S: aws.String(currentTime),
		},
	}
	updateExpression := "SET #A = :a, #M = :m"

	input := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"organization_id": {
				S: aws.String(gitlabOrg.OrganizationID),
			},
		},
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		UpdateExpression:          &updateExpression,
		TableName:                 aws.String(repo.gitlabOrgTableName),
	}

	log.WithFields(f).Debug("updating gitlab organization record...")
	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("unable to update Gitlab organization record, error: %+v", updateErr)
		return updateErr
	}

	return nil
}

func (repo Repository) UpdateGitlabOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, autoEnabledClaGroupID string, branchProtectionEnabled bool, enabled *bool) error {
	f := logrus.Fields{
		"functionName":            "gitlab_organizations.repository.UpdateGitlabOrganization",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"projectSFID":             projectSFID,
		"organizationName":        organizationName,
		"autoEnabled":             autoEnabled,
		"autoEnabledClaGroupID":   autoEnabledClaGroupID,
		"branchProtectionEnabled": branchProtectionEnabled,
		"tableName":               repo.gitlabOrgTableName,
	}

	_, currentTime := utils.CurrentTime()
	gitlabOrgs, lookupErr := repo.GetGitlabOrganizationByName(ctx, organizationName)
	if lookupErr != nil {
		log.WithFields(f).Warnf("error looking up Gitlab organization by name, error: %+v", lookupErr)
		return lookupErr
	}
	if gitlabOrgs == nil || len(gitlabOrgs.List) == 0 {
		lookupErr := errors.New("unable to lookup Gitlab organization by name")
		log.WithFields(f).Warnf("error looking up Gitlab organization, error: %+v", lookupErr)
		return lookupErr
	}

	gitlabOrg := gitlabOrgs.List[0]

	expressionAttributeNames := map[string]*string{
		"#A": aws.String("auto_enabled"),
		"#C": aws.String("auto_enabled_cla_group_id"),
		"#B": aws.String("branch_protection_enabled"),
		"#M": aws.String("date_modified"),
	}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{
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
	}
	updateExpression := "SET #A = :a, #C = :c, #B = :b, #M = :m"

	if enabled != nil {
		expressionAttributeNames["#E"] = aws.String("enabled")
		expressionAttributeValues[":e"] = &dynamodb.AttributeValue{
			BOOL: aws.Bool(*enabled),
		}
		updateExpression = updateExpression + ", #E = :e "
	}

	input := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"organization_id": {
				S: aws.String(gitlabOrg.OrganizationID),
			},
		},
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		UpdateExpression:          &updateExpression,
		TableName:                 aws.String(repo.gitlabOrgTableName),
	}

	log.WithFields(f).Debugf("updating gitlab organization record: %+v", input)
	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("unable to update Gitlab organization record, error: %+v", updateErr)
		return updateErr
	}

	return nil
}

func (repo Repository) DeleteGitlabOrganization(ctx context.Context, projectSFID, gitlabOrgName string) error {
	f := logrus.Fields{
		"functionName":   "v1.github_organizations.repository.DeleteGitHubOrganization",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"githubOrgName":  gitlabOrgName,
	}

	var gitlabOrganizationID string
	orgs, orgErr := repo.GetGitlabOrganizations(ctx, projectSFID)
	if orgErr != nil {
		errMsg := fmt.Sprintf("gitlab organization is not found using projectSFID: %s, error: %+v", projectSFID, orgErr)
		log.WithFields(f).Warn(errMsg)
		return errors.New(errMsg)
	}

	for _, githubOrg := range orgs.List {
		if strings.EqualFold(githubOrg.OrganizationName, gitlabOrgName) {
			gitlabOrganizationID = githubOrg.OrganizationID
			break
		}
	}

	log.WithFields(f).Debug("Deleting GitHub organization...")
	// Update enabled flag as false
	_, currentTime := utils.CurrentTime()
	note := fmt.Sprintf("Enabled set to false due to org deletion at %s ", currentTime)
	_, err := repo.dynamoDBClient.UpdateItem(
		&dynamodb.UpdateItemInput{
			Key: map[string]*dynamodb.AttributeValue{
				"organization_id": {
					S: aws.String(gitlabOrganizationID),
				},
			},
			ExpressionAttributeNames: map[string]*string{
				"#E": aws.String("enabled"),
				"#N": aws.String("note"),
				"#D": aws.String("date_modified"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":e": {
					BOOL: aws.Bool(false),
				},
				":n": {
					S: aws.String(note),
				},
				":d": {
					S: aws.String(currentTime),
				},
			},
			UpdateExpression: aws.String("SET #E = :e, #N = :n, #D = :d"),
			TableName:        aws.String(repo.gitlabOrgTableName),
		},
	)
	if err != nil {
		errMsg := fmt.Sprintf("error deleting gitlab organization: %s - %+v", gitlabOrgName, err)
		log.WithFields(f).Warnf(errMsg)
		return errors.New(errMsg)
	}

	return nil
}

func buildGitlabOrganizationListModels(ctx context.Context, gitlabOrganizations []*GitlabOrganization) []*models2.GitlabOrganization {
	f := logrus.Fields{
		"functionName":   "buildGitlabOrganizationListModels",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	log.WithFields(f).Debugf("fetching gitlab info for the list")
	// Convert the database model to a response model
	return toModels(gitlabOrganizations)

	// TODO: Fetch the gitlab information
}
