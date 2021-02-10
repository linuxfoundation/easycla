// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/gofrs/uuid"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// index
const (
	repositoryEnabledColumn = "enabled"

	// RepositoryEnabled flag
	RepositoryEnabled = "enabled"

	// RepositoryDisabled flag
	RepositoryDisabled = "disabled"

	ProjectRepositoryIndex                     = "project-repository-index"
	SFDCRepositoryIndex                        = "sfdc-repository-index"
	ExternalRepositoryIndex                    = "external-repository-index"
	ProjectSFIDRepositoryOrganizationNameIndex = "project-sfid-repository-organization-name-index"
	RepositoryOrganizationNameIndex            = "repository-organization-name-index"
	RepositoryNameIndex                        = "repository-name-index"
)

// ErrRepositoryDoesNotExist ...
var ErrRepositoryDoesNotExist = errors.New("repository does not exist")

// Repository defines functions of Repositories
type Repository interface {
	AddGithubRepository(ctx context.Context, externalProjectID string, projectSFID string, input *models.GithubRepositoryInput) (*models.GithubRepository, error)
	UpdateGithubRepository(ctx context.Context, repositoryID string, input *models.GithubRepositoryInput) (*models.GithubRepository, error)
	UpdateClaGroupID(ctx context.Context, repositoryID, claGroupID string) error
	EnableRepository(ctx context.Context, repositoryID string) error
	EnableRepositoryWithCLAGroupID(ctx context.Context, repositoryID, claGroupID string) error
	DisableRepository(ctx context.Context, repositoryID string) error
	DisableRepositoriesByProjectID(ctx context.Context, projectID string) error
	DisableRepositoriesOfGithubOrganization(ctx context.Context, externalProjectID, githubOrgName string) error
	GetRepository(ctx context.Context, repositoryID string) (*models.GithubRepository, error)
	GetRepositoryByName(ctx context.Context, repositoryName string) (*models.GithubRepository, error)
	GetRepositoryByGithubID(ctx context.Context, externalID string, enabled bool) (*models.GithubRepository, error)
	GetRepositoriesByCLAGroup(ctx context.Context, claGroup string, enabled bool) ([]*models.GithubRepository, error)
	GetRepositoriesByOrganizationName(ctx context.Context, gitHubOrgName string) ([]*models.GithubRepository, error)
	GetCLAGroupRepositoriesGroupByOrgs(ctx context.Context, projectID string, enabled bool) ([]*models.GithubRepositoriesGroupByOrgs, error)
	ListProjectRepositories(ctx context.Context, externalProjectID string, projectSFID string, enabled *bool) (*models.ListGithubRepositories, error)
}

// NewRepository create new Repository
func NewRepository(awsSession *session.Session, stage string) Repository {
	return &repo{
		stage:               stage,
		dynamoDBClient:      dynamodb.New(awsSession),
		repositoryTableName: fmt.Sprintf("cla-%s-repositories", stage),
	}
}

type repo struct {
	stage               string
	dynamoDBClient      *dynamodb.DynamoDB
	repositoryTableName string
}

// AddGithubRepository adds the specified repository
func (r repo) AddGithubRepository(ctx context.Context, externalProjectID string, projectSFID string, input *models.GithubRepositoryInput) (*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":               "repositories.repository.AddGitHubRepository",
		utils.XREQUESTID:             ctx.Value(utils.XREQUESTID),
		"externalProjectID":          externalProjectID,
		"projectSFID":                projectSFID,
		"repositoryName":             *input.RepositoryName,
		"repositoryOrganizationName": *input.RepositoryOrganizationName,
		"repositoryType":             *input.RepositoryType,
		"repositoryURL":              *input.RepositoryURL,
	}

	// Check first to see if the repository already exists
	_, err := r.GetRepositoryByGithubID(ctx, utils.StringValue(input.RepositoryExternalID), true)
	if err != nil {
		// Expecting Not found - no issue if not found - all other error we throw
		if _, ok := err.(*utils.GitHubRepositoryNotFound); !ok {
			return nil, err
		}
	} else {
		return nil, errors.New("github repository already exist")
	}

	_, currentTime := utils.CurrentTime()
	repoID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	repository := &RepositoryDBModel{
		DateCreated:                currentTime,
		DateModified:               currentTime,
		RepositoryExternalID:       utils.StringValue(input.RepositoryExternalID),
		RepositoryID:               repoID.String(),
		RepositoryName:             utils.StringValue(input.RepositoryName),
		RepositoryOrganizationName: utils.StringValue(input.RepositoryOrganizationName),
		RepositoryProjectID:        utils.StringValue(input.RepositoryProjectID),
		RepositorySfdcID:           externalProjectID,
		RepositoryType:             utils.StringValue(input.RepositoryType),
		RepositoryURL:              utils.StringValue(input.RepositoryURL),
		Enabled:                    true, // default is enabled
		Note:                       fmt.Sprintf("created on %s", currentTime),
		ProjectSFID:                projectSFID,
		Version:                    "v1",
	}
	av, err := dynamodbattribute.MarshalMap(repository)
	if err != nil {
		log.WithFields(f).Warnf("problem marshalling the input, error: %+v", err)
		return nil, err
	}

	log.WithFields(f).Debug("creating repository entry")
	_, err = r.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(r.repositoryTableName),
	})
	if err != nil {
		log.WithFields(f).Warnf("cannot add github repository, error: %+v", err)
		return nil, err
	}

	return repository.toModel(), nil
}

// UpdateGithubRepository updates the repository record for given ID
func (r *repo) UpdateGithubRepository(ctx context.Context, repositoryID string, input *models.GithubRepositoryInput) (*models.GithubRepository, error) {

	externalID := utils.StringValue(input.RepositoryExternalID)
	repositoryName := utils.StringValue(input.RepositoryName)
	repositoryOrganizationName := utils.StringValue(input.RepositoryOrganizationName)
	repositoryType := utils.StringValue(input.RepositoryType)
	repositoryURL := utils.StringValue(input.RepositoryURL)

	f := logrus.Fields{
		"functionName":               "repositories.repository.UpdateGitHubRepository",
		utils.XREQUESTID:             ctx.Value(utils.XREQUESTID),
		"repositoryID":               repositoryID,
		"externalProjectID":          externalID,
		"repositoryName":             repositoryName,
		"repositoryOrganizationName": repositoryOrganizationName,
		"repositoryType":             repositoryType,
		"repositoryURL":              repositoryURL,
	}

	log.WithFields(f).Debugf("updating Repository : %s... ", repositoryID)

	repoModel, repoErr := r.GetRepository(ctx, repositoryID)
	if repoErr != nil {
		log.WithFields(f).Warnf("update error locating the repository ID : %s , error: %+v ", repositoryID, repoErr)
		return nil, repoErr
	}

	if repoModel == nil {
		log.WithFields(f).Warnf("Repository does not exist for repo: %s ", repositoryID)
		return nil, ErrRepositoryDoesNotExist
	}

	expressionAttributeNames := map[string]*string{}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{}
	updateExpression := "SET "

	if externalID != "" && repoModel.RepositoryExternalID != externalID {
		log.WithFields(f).Debugf("adding externalID : %s ", externalID)
		expressionAttributeNames["#E"] = aws.String("repository_external_id")
		expressionAttributeValues[":e"] = &dynamodb.AttributeValue{S: aws.String(externalID)}
		updateExpression = updateExpression + " #E = :e, "
	}

	if repositoryName != "" && repoModel.RepositoryName != repositoryName {
		log.WithFields(f).Debugf("adding repositoryName : %s ", repositoryName)
		expressionAttributeNames["#N"] = aws.String("repository_name")
		expressionAttributeValues[":n"] = &dynamodb.AttributeValue{S: aws.String(repositoryName)}
		updateExpression = updateExpression + " #N = :n, "
	}

	if repositoryOrganizationName != "" && repoModel.RepositoryOrganizationName != repositoryOrganizationName {
		log.WithFields(f).Debugf("adding repositoryOrganizationName : %s ", repositoryOrganizationName)
		expressionAttributeNames["#O"] = aws.String("repository_organization_name")
		expressionAttributeValues[":o"] = &dynamodb.AttributeValue{S: aws.String(repositoryOrganizationName)}
		updateExpression = updateExpression + " #O = :o, "
	}

	if repositoryType != "" && repoModel.RepositoryType != repositoryType {
		log.WithFields(f).Debugf("adding repositoryType : %s ", repositoryType)
		expressionAttributeNames["#T"] = aws.String("repository_type")
		expressionAttributeValues[":t"] = &dynamodb.AttributeValue{S: aws.String(repositoryType)}
		updateExpression = updateExpression + " #T = :t, "
	}

	if repositoryURL != "" && repoModel.RepositoryURL != repositoryURL {
		log.WithFields(f).Debugf("adding repositoryURL : %s ", repositoryURL)
		expressionAttributeNames["#U"] = aws.String("repository_url")
		expressionAttributeValues[":u"] = &dynamodb.AttributeValue{S: aws.String(repositoryURL)}
		updateExpression = updateExpression + " #U = :u, "
	}

	if input.Enabled != nil && repoModel.Enabled != *input.Enabled {
		log.WithFields(f).Debugf("adding enabled flag: %+v", *input.Enabled)
		expressionAttributeNames["#EN"] = aws.String("enabled")
		expressionAttributeValues[":en"] = &dynamodb.AttributeValue{BOOL: input.Enabled}
		updateExpression = updateExpression + " #EN = :en, "
	}

	_, currentTimeString := utils.CurrentTime()
	log.WithFields(f).Debugf("adding date_modified: %s", currentTimeString)
	expressionAttributeNames["#M"] = aws.String("date_modified")
	expressionAttributeValues[":m"] = &dynamodb.AttributeValue{S: aws.String(currentTimeString)}
	updateExpression = updateExpression + " #M = :m "

	// Assemble the query input parameters
	updateInput := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"repository_id": {
				S: aws.String(repositoryID),
			},
		},
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		UpdateExpression:          &updateExpression,
		TableName:                 aws.String(r.repositoryTableName),
	}

	_, updateErr := r.dynamoDBClient.UpdateItem(updateInput)
	if updateErr != nil {
		log.WithFields(f).Warnf("error updatingRepository by repositoryID: %s, error: %v", repositoryID, updateErr)
		return nil, updateErr
	}

	return r.GetRepository(ctx, repositoryID)
}

// UpdateClaGroupID updates the claGroupID of the repository
func (r *repo) UpdateClaGroupID(ctx context.Context, repositoryID, claGroupID string) error {
	return r.setClaGroupIDGithubRepository(ctx, repositoryID, claGroupID)
}

// EnableRepository enables the repository entry
func (r *repo) EnableRepository(ctx context.Context, repositoryID string) error {
	return r.enableGithubRepository(ctx, repositoryID)
}

// EnableRepositoryWithCLAGroupID enables the repository entry with the specified CLA Group ID
func (r *repo) EnableRepositoryWithCLAGroupID(ctx context.Context, repositoryID, claGroupID string) error {
	return r.enableGithubRepositoryWithCLAGroupID(ctx, repositoryID, claGroupID)
}

// DisableRepository disables the repository entry (we don't delete)
func (r *repo) DisableRepository(ctx context.Context, repositoryID string) error {
	return r.disableGithubRepository(ctx, repositoryID)
}

func (r *repo) DisableRepositoriesByProjectID(ctx context.Context, projectID string) error {
	repoModels, err := r.getProjectRepositories(ctx, projectID, true)
	if err != nil {
		return err
	}

	// For each model...
	for _, repoModel := range repoModels {
		disableErr := r.DisableRepository(ctx, repoModel.RepositoryID)
		if disableErr != nil {
			return disableErr
		}
	}

	return nil
}

// DisableRepositoriesOfGithubOrganization disables the repositories under the GitHub organization
func (r repo) DisableRepositoriesOfGithubOrganization(ctx context.Context, externalProjectID, githubOrgName string) error {
	repoModels, err := r.getRepositoriesByGithubOrg(ctx, githubOrgName)
	if err != nil {
		return err
	}
	for _, repoModel := range repoModels {
		if repoModel.RepositoryExternalID == externalProjectID || repoModel.RepositorySfdcID == externalProjectID {
			err = r.disableGithubRepository(ctx, repoModel.RepositoryID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// GetRepository by repository id
func (r *repo) GetRepository(ctx context.Context, repositoryID string) (*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "repositories.repository.GetRepository",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"repositoryID":   repositoryID,
	}
	result, err := r.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(r.repositoryTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"repository_id": {
				S: aws.String(repositoryID),
			},
		},
	})
	if err != nil {
		log.WithFields(f).Warn("problem querying using repository ID")
		return nil, err
	}
	if len(result.Item) == 0 {
		msg := fmt.Sprintf("repository with ID: %s does not exist", repositoryID)
		log.WithFields(f).Warn(msg)
		return nil, &utils.GitHubRepositoryNotFound{
			Message: msg,
		}
	}

	var out RepositoryDBModel
	err = dynamodbattribute.UnmarshalMap(result.Item, &out)
	if err != nil {
		log.WithFields(f).Warn("problem unmarshalling response")
		return nil, err
	}

	return out.toModel(), nil
}

// GetRepositoryByName fetches the repository by repository name
func (r *repo) GetRepositoryByName(ctx context.Context, repositoryName string) (*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "repositories.repository.GetRepositoryByName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"repositoryName": repositoryName,
	}
	builder := expression.NewBuilder()
	condition := expression.Key("repository_name").Equal(expression.Value(repositoryName))
	builder = builder.WithKeyCondition(condition)

	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem creating builder")
		return nil, err
	}

	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(r.repositoryTableName),
		IndexName:                 aws.String(RepositoryNameIndex),
	}

	results, err := r.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to get repositories by name")
		return nil, err
	}

	if len(results.Items) == 0 {
		log.WithFields(f).Warn("no repositories found with repository name")
		return nil, &utils.GitHubRepositoryNotFound{
			RepositoryName: repositoryName,
		}
	}

	var repositories []*RepositoryDBModel
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &repositories)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem unmarshalling response")
		return nil, err
	}

	if len(repositories) > 0 {
		log.WithFields(f).Warn("multiple repositories records with the same repository name")
	}

	return repositories[0].toModel(), nil
}

// GetRepositoryByCLAGroup gets the list of repositories based on the CLA Group ID
func (r *repo) GetRepositoriesByCLAGroup(ctx context.Context, claGroupID string, enabled bool) ([]*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "repositories.repository.GetRepositoryByCLAGroup",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"enabled":        enabled,
	}
	builder := expression.NewBuilder()
	condition := expression.Key("repository_project_id").Equal(expression.Value(claGroupID))
	filter := expression.Name(repositoryEnabledColumn).Equal(expression.Value(enabled))
	builder = builder.WithKeyCondition(condition).WithFilter(filter)

	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}

	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(r.repositoryTableName),
		IndexName:                 aws.String(ProjectRepositoryIndex),
	}

	results, err := r.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("unable to get project github repositories. error: %+v", err)
		return nil, err
	}

	if len(results.Items) == 0 {
		msg := fmt.Sprintf("no repositories found associated with CLA Group ID: %s that is enabled", claGroupID)
		log.WithFields(f).Warn(msg)
		return nil, &utils.GitHubRepositoryNotFound{
			Message: msg,
		}
	}

	var repositories []*RepositoryDBModel
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &repositories)
	if err != nil {
		log.WithFields(f).Warnf("problem unmarshalling response, error: %+v", err)
		return nil, err
	}

	return convertModels(repositories), nil
}

func (r *repo) GetRepositoriesByOrganizationName(ctx context.Context, gitHubOrgName string) ([]*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "repositories.repository.GetRepositoriesByOrganizationName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"gitHubOrgName":  gitHubOrgName,
	}

	builder := expression.NewBuilder()
	condition := expression.Key("repository_organization_name").Equal(expression.Value(gitHubOrgName))
	builder = builder.WithKeyCondition(condition)

	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}

	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(r.repositoryTableName),
		IndexName:                 aws.String(RepositoryOrganizationNameIndex),
	}

	log.WithFields(f).Debug("querying repositories table by github organization name")
	results, err := r.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("unable to get github repositories by organization name. error: %+v", err)
		return nil, err
	}

	if len(results.Items) == 0 {
		msg := fmt.Sprintf("no repositories found associated GitHub Organization: %s", gitHubOrgName)
		log.WithFields(f).Warn(msg)
		return nil, &utils.GitHubRepositoryNotFound{
			Message: msg,
		}
	}

	var repositories []*RepositoryDBModel
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &repositories)
	if err != nil {
		log.WithFields(f).Warnf("problem unmarshalling repository response, error: %+v", err)
		return nil, err
	}

	return convertModels(repositories), nil
}

// GetCLAGroupRepositoriesGroupByOrgs returns a list of GH organizations by CLA Group - enabled flag indicates that we search the enabled repositories list
func (r repo) GetCLAGroupRepositoriesGroupByOrgs(ctx context.Context, projectID string, enabled bool) ([]*models.GithubRepositoriesGroupByOrgs, error) {
	out := make([]*models.GithubRepositoriesGroupByOrgs, 0)
	outMap := make(map[string]*models.GithubRepositoriesGroupByOrgs)
	ghrepos, err := r.getProjectRepositories(ctx, projectID, enabled)
	if err != nil {
		return nil, err
	}
	for _, ghrepo := range ghrepos {
		ghrepoGroup, ok := outMap[ghrepo.RepositoryOrganizationName]
		if !ok {
			ghrepoGroup = &models.GithubRepositoriesGroupByOrgs{
				OrganizationName: ghrepo.RepositoryOrganizationName,
			}
			out = append(out, ghrepoGroup)
			outMap[ghrepo.RepositoryOrganizationName] = ghrepoGroup
		}
		ghrepoGroup.List = append(ghrepoGroup.List, ghrepo)
	}
	return out, nil
}

// List github repositories of project by external/salesforce project id
func (r repo) ListProjectRepositories(ctx context.Context, externalProjectID string, projectSFID string, enabled *bool) (*models.ListGithubRepositories, error) {
	f := logrus.Fields{
		"functionName":      "repositories.repository.ListProjectRepositories",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"externalProjectID": externalProjectID,
		"projectSFID":       projectSFID,
		"enabled":           utils.BoolValue(enabled),
	}

	var indexName string
	out := &models.ListGithubRepositories{
		List: make([]*models.GithubRepository, 0),
	}
	var condition expression.KeyConditionBuilder
	var filter expression.ConditionBuilder

	if externalProjectID != "" {
		condition = expression.Key("repository_sfdc_id").Equal(expression.Value(externalProjectID))
		indexName = SFDCRepositoryIndex
	} else {
		condition = expression.Key("project_sfid").Equal(expression.Value(projectSFID))
		indexName = ProjectSFIDRepositoryOrganizationNameIndex
	}

	// Add the enabled filter, if set
	if enabled != nil {
		filter = expression.Name(repositoryEnabledColumn).Equal(expression.Value(enabled))
	}

	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter).Build()
	if err != nil {
		return nil, err
	}
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(r.repositoryTableName),
		IndexName:                 aws.String(indexName),
	}

	results, err := r.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("unable to get project github repositories. error = %s", err.Error())
		return nil, err
	}
	if len(results.Items) == 0 {
		return out, nil
	}
	var result []*RepositoryDBModel
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &result)
	if err != nil {
		return nil, err
	}
	for _, gr := range result {
		out.List = append(out.List, gr.toModel())
	}
	return out, nil
}

// getProjectRepositories returns an array of GH repositories for the specified project ID
func (r repo) getProjectRepositories(ctx context.Context, projectID string, enabled bool) ([]*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "repositories.repository.getProjectRepositories",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectID":      projectID,
		"enabled":        enabled,
	}
	var out []*models.GithubRepository

	condition := expression.Key("repository_project_id").Equal(expression.Value(projectID))
	filter := expression.Name(repositoryEnabledColumn).Equal(expression.Value(enabled))
	builder := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter)
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
		TableName:                 aws.String(r.repositoryTableName),
		IndexName:                 aws.String(ProjectRepositoryIndex),
	}

	results, err := r.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("unable to get project github repositories. error = %s", err.Error())
		return nil, err
	}
	if len(results.Items) == 0 {
		return out, nil
	}
	var result []*RepositoryDBModel
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &result)
	if err != nil {
		return nil, err
	}
	for _, gr := range result {
		out = append(out, gr.toModel())
	}
	return out, nil
}

// getRepositoriesByGithubOrg returns an array of GH repositories for the specified project ID
func (r repo) getRepositoriesByGithubOrg(ctx context.Context, githubOrgName string) ([]*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "repositories.repository.getRepositoriesByGitHubOrg",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"githubOrgName":  githubOrgName,
	}

	var out []*models.GithubRepository
	builder := expression.NewBuilder()
	filter := expression.Name("repository_organization_name").Equal(expression.Value(githubOrgName))
	builder = builder.WithFilter(filter)
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}
	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(r.repositoryTableName),
	}

	results, err := r.dynamoDBClient.Scan(scanInput)
	if err != nil {
		log.WithFields(f).Warnf("unable to get github organizations repositories. error = %s", err.Error())
		return nil, err
	}
	if len(results.Items) == 0 {
		return out, nil
	}
	var result []*RepositoryDBModel
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &result)
	if err != nil {
		return nil, err
	}
	for _, gr := range result {
		out = append(out, gr.toModel())
	}
	return out, nil
}

// GetRepositoryByGithubID fetches the repository model by its external github id
func (r repo) GetRepositoryByGithubID(ctx context.Context, externalID string, enabled bool) (*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "repositories.repository.GetRepositoryByGitHubID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"externalID":     externalID,
		"enabled":        enabled,
	}

	var condition expression.KeyConditionBuilder
	builder := expression.NewBuilder()
	condition = expression.Key("repository_external_id").Equal(expression.Value(externalID))
	filter := expression.Name(repositoryEnabledColumn).Equal(expression.Value(enabled))

	builder = builder.WithKeyCondition(condition).WithFilter(filter)
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
		TableName:                 aws.String(r.repositoryTableName),
		IndexName:                 aws.String(ExternalRepositoryIndex),
	}

	results, err := r.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("unable to get project github repositories. error = %s", err.Error())
		return nil, err
	}
	var result *RepositoryDBModel
	if len(results.Items) == 0 {
		msg := fmt.Sprintf("no repository found matching external repository ID: %s", externalID)
		return nil, &utils.GitHubRepositoryNotFound{
			Message: msg,
		}
	}
	err = dynamodbattribute.UnmarshalMap(results.Items[0], &result)
	if err != nil {
		return nil, err
	}

	return result.toModel(), nil
}

func (r repo) enableGithubRepository(ctx context.Context, repositoryID string) error {
	return r.setEnabledGithubRepository(ctx, repositoryID, true)
}

func (r repo) enableGithubRepositoryWithCLAGroupID(ctx context.Context, repositoryID, claGroupID string) error {
	return r.setEnabledGithubRepositoryWithCLAGroupID(ctx, repositoryID, claGroupID, true)
}

func (r repo) disableGithubRepository(ctx context.Context, repositoryID string) error {
	return r.setEnabledGithubRepository(ctx, repositoryID, false)
}

// setEnabledGithubRepository updates the existing repository record by setting the enabled flag to false
func (r repo) setEnabledGithubRepository(ctx context.Context, repositoryID string, enabled bool) error {
	f := logrus.Fields{
		"functionName":   "repositories.repository.setEnabledGitHubRepository",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"repositoryID":   repositoryID,
		"enabled":        enabled,
	}

	// Load the existing model - need to fetch the old note value, if available
	existingModel, getErr := r.GetRepository(ctx, repositoryID)
	if getErr != nil {
		log.WithFields(f).WithError(getErr).Warn("unable to load repository by repository id")
		return getErr
	}
	if existingModel == nil {
		return fmt.Errorf("unable to locate existing repository entry by ID: %s", repositoryID)
	}

	// Enabled string for the note...
	var enabledString = RepositoryDisabled
	if enabled {
		enabledString = RepositoryEnabled
	}

	// If we have an old note - grab it/save it
	var existingNote = ""
	if existingModel.Note != "" {
		existingNote = existingModel.Note + ". "
	}

	_, now := utils.CurrentTime()
	log.WithFields(f).Debug("updating repository record")
	_, err := r.dynamoDBClient.UpdateItem(&dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"repository_id": {S: aws.String(repositoryID)},
		},
		ExpressionAttributeNames: map[string]*string{
			"#enabled":      aws.String(repositoryEnabledColumn),
			"#note":         aws.String("note"),
			"#dateModified": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":enabledValue": {
				BOOL: aws.Bool(enabled),
			},
			":noteValue": {
				S: aws.String(fmt.Sprintf("%s%s on %s", existingNote, enabledString, now)), // Add to existing note, if set
			},
			":dateModifiedValue": {
				S: aws.String(now),
			},
		},
		UpdateExpression: aws.String("SET #enabled = :enabledValue, #note = :noteValue, #dateModified = :dateModifiedValue"),
		TableName:        aws.String(r.repositoryTableName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				return errors.New("github repository entry does not exist or repository_sfdc_id does not match with specified project id")
			}
		}
		log.WithFields(f).WithError(err).Warn("error disabling github repository")
		return err
	}

	return nil
}

// setEnabledGithubRepositoryWithCLAGroupID updates the existing repository record by setting the enabled flag to false
func (r repo) setEnabledGithubRepositoryWithCLAGroupID(ctx context.Context, repositoryID, claGroupID string, enabled bool) error {
	f := logrus.Fields{
		"functionName":   "repositories.repository.setEnabledGitHubRepositoryWithCLAGroupID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"repositoryID":   repositoryID,
		"claGroupID":     claGroupID,
		"enabled":        enabled,
	}

	// Load the existing model - need to fetch the old note value, if available
	existingModel, getErr := r.GetRepository(ctx, repositoryID)
	if getErr != nil {
		log.WithFields(f).WithError(getErr).Warn("unable to load repository by repository id")
		return getErr
	}
	if existingModel == nil {
		return fmt.Errorf("unable to locate existing repository entry by ID: %s", repositoryID)
	}

	// Enabled string for the note...
	var enabledString = RepositoryDisabled
	if enabled {
		enabledString = RepositoryEnabled
	}

	// If we have an old note - grab it/save it
	var existingNote = ""
	if existingModel.Note != "" {
		existingNote = existingModel.Note + ". "
	}

	_, now := utils.CurrentTime()
	log.WithFields(f).Debug("updating repository record")
	_, err := r.dynamoDBClient.UpdateItem(&dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"repository_id": {S: aws.String(repositoryID)},
		},
		ExpressionAttributeNames: map[string]*string{
			"#claGroupID":   aws.String("repository_project_id"),
			"#enabled":      aws.String(repositoryEnabledColumn),
			"#note":         aws.String("note"),
			"#dateModified": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":claGroupID": {
				S: aws.String(claGroupID),
			},
			":enabledValue": {
				BOOL: aws.Bool(enabled),
			},
			":noteValue": {
				S: aws.String(fmt.Sprintf("%s, %s for cla group %s on %s", existingNote, enabledString, claGroupID, now)), // Add to existing note, if set
			},
			":dateModifiedValue": {
				S: aws.String(now),
			},
		},
		UpdateExpression: aws.String("SET #claGroupID = :claGroupID, #enabled = :enabledValue, #note = :noteValue, #dateModified = :dateModifiedValue"),
		TableName:        aws.String(r.repositoryTableName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				return errors.New("github repository entry does not exist or repository_sfdc_id does not match with specified project id")
			}
		}
		log.WithFields(f).WithError(err).Warn("error disabling github repository")
		return err
	}

	return nil
}

// setEnabledGithubRepository updates the existing repository record by setting the enabled flag to false
func (r repo) setClaGroupIDGithubRepository(ctx context.Context, repositoryID, claGroupID string) error {
	f := logrus.Fields{
		"functionName":   "repositories.repository.setClaGroupIDGitHubRepository",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"repositoryID":   repositoryID,
		"claGroupID":     claGroupID,
	}

	// Load the existing model - need to fetch the old note value, if available
	existingModel, getErr := r.GetRepository(ctx, repositoryID)
	if getErr != nil {
		log.WithFields(f).WithError(getErr).Warn("unable to load repository by repository id")
		return getErr
	}
	if existingModel == nil {
		return fmt.Errorf("unable to locate existing repository entry by ID: %s", repositoryID)
	}

	// Enabled string for the note...
	var claGroupString = "claGroupID"

	// If we have an old note - grab it/save it
	var existingNote = ""
	if existingModel.Note != "" {
		existingNote = existingModel.Note + ". "
	}

	_, now := utils.CurrentTime()
	log.WithFields(f).Debug("updating repository record with cla group id")
	_, err := r.dynamoDBClient.UpdateItem(&dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"repository_id": {S: aws.String(repositoryID)},
		},
		ExpressionAttributeNames: map[string]*string{
			"#repositoryProjectID": aws.String("repository_project_id"),
			"#note":                aws.String("note"),
			"#dateModified":        aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":repositoryProjectIDValue": {
				S: aws.String(claGroupID),
			},
			":noteValue": {
				S: aws.String(fmt.Sprintf("%s%s on %s", existingNote, claGroupString, now)), // Add to existing note, if set
			},
			":dateModifiedValue": {
				S: aws.String(now),
			},
		},
		UpdateExpression: aws.String("SET #repositoryProjectID = :repositoryProjectIDValue, #note = :noteValue, #dateModified = :dateModifiedValue"),
		TableName:        aws.String(r.repositoryTableName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				return errors.New("github repository entry does not exist or repository_sfdc_id does not match with specified project id")
			}
		}
		log.WithFields(f).WithError(err).Warn("error disabling github repository")
		return err
	}

	return nil
}
