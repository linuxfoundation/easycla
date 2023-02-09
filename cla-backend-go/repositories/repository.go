// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/gofrs/uuid"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// index
const (
	repositoryEnabledColumn = "enabled"
)

// ErrRepositoryDoesNotExist ...
var ErrRepositoryDoesNotExist = errors.New("repository does not exist")

// RepositoryInterface contains functions of the repositories service
type RepositoryInterface interface {
	GitHubAddRepository(ctx context.Context, externalProjectID string, projectSFID string, input *models.GithubRepositoryInput) (*models.GithubRepository, error)
	GitHubUpdateRepository(ctx context.Context, repositoryID, projectSFID, parentProjectSFID string, input *models.GithubRepositoryInput) (*models.GithubRepository, error)
	GitHubUpdateClaGroupID(ctx context.Context, repositoryID, claGroupID string) error
	GitHubEnableRepository(ctx context.Context, repositoryID string) error
	GitHubEnableRepositoryWithCLAGroupID(ctx context.Context, repositoryID, claGroupID string) error
	GitHubDisableRepository(ctx context.Context, repositoryID string) error
	GitHubDisableRepositoriesByProjectID(ctx context.Context, projectID string) error
	GitHubDisableRepositoriesOfOrganization(ctx context.Context, externalProjectID, githubOrgName string) error
	GitHubGetRepository(ctx context.Context, repositoryID string) (*models.GithubRepository, error)
	GitHubGetRepositoryByName(ctx context.Context, repositoryName string) (*models.GithubRepository, error)
	GitHubGetRepositoryByExternalID(ctx context.Context, repositoryExternalID string) (*models.GithubRepository, error)
	GitHubGetRepositoryByGithubID(ctx context.Context, externalID string, enabled bool) (*models.GithubRepository, error)
	GitHubGetRepositoriesByCLAGroup(ctx context.Context, claGroup string, enabled bool) ([]*models.GithubRepository, error)
	GitHubGetRepositoriesByOrganizationName(ctx context.Context, gitHubOrgName string) ([]*models.GithubRepository, error)
	GitHubGetCLAGroupRepositoriesGroupByOrgs(ctx context.Context, projectID string, enabled bool) ([]*models.GithubRepositoriesGroupByOrgs, error)
	GitHubListProjectRepositories(ctx context.Context, projectSFID string, enabled *bool) (*models.GithubListRepositories, error)
}

// NewRepository create new Repository
func NewRepository(awsSession *session.Session, stage string) *Repository {
	return &Repository{
		stage:               stage,
		dynamoDBClient:      dynamodb.New(awsSession),
		repositoryTableName: fmt.Sprintf("cla-%s-repositories", stage),
	}
}

// Repository structure
type Repository struct {
	stage               string
	dynamoDBClient      *dynamodb.DynamoDB
	repositoryTableName string
}

// GitHubAddRepository adds the specified repository
func (r *Repository) GitHubAddRepository(ctx context.Context, externalProjectID string, projectSFID string, input *models.GithubRepositoryInput) (*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":               "v1.repositories.repository.AddGitHubRepository",
		utils.XREQUESTID:             ctx.Value(utils.XREQUESTID),
		"externalProjectID":          externalProjectID,
		"projectSFID":                projectSFID,
		"repositoryName":             *input.RepositoryName,
		"repositoryOrganizationName": *input.RepositoryOrganizationName,
		"repositoryType":             *input.RepositoryType,
		"repositoryURL":              *input.RepositoryURL,
	}

	// Check first to see if the repository already exists
	_, err := r.GitHubGetRepositoryByGithubID(ctx, utils.StringValue(input.RepositoryExternalID), true)
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
		RepositoryCLAGroupID:       utils.StringValue(input.RepositoryProjectID),
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

	return repository.ToGitHubModel(), nil
}

// GitHubUpdateRepository updates the repository record for given ID
func (r *Repository) GitHubUpdateRepository(ctx context.Context, repositoryID, projectSFID, parentProjectSFID string, input *models.GithubRepositoryInput) (*models.GithubRepository, error) {

	externalID := utils.StringValue(input.RepositoryExternalID)
	repositoryName := utils.StringValue(input.RepositoryName)
	repositoryOrganizationName := utils.StringValue(input.RepositoryOrganizationName)
	repositoryType := utils.StringValue(input.RepositoryType)
	repositoryURL := utils.StringValue(input.RepositoryURL)
	note := input.Note

	f := logrus.Fields{
		"functionName":               "v1.repositories.repository.UpdateGitHubRepository",
		utils.XREQUESTID:             ctx.Value(utils.XREQUESTID),
		"repositoryID":               repositoryID,
		"externalProjectID":          externalID,
		"repositoryName":             repositoryName,
		"repositoryOrganizationName": repositoryOrganizationName,
		"repositoryType":             repositoryType,
		"repositoryURL":              repositoryURL,
		"projectSFDID":               projectSFID,
		"parentProjectSFID":          parentProjectSFID,
	}

	log.WithFields(f).Debugf("updating CombinedRepository : %s... ", repositoryID)

	repoModel, repoErr := r.GitHubGetRepository(ctx, repositoryID)
	if repoErr != nil {
		log.WithFields(f).Warnf("update error locating the repository ID : %s , error: %+v ", repositoryID, repoErr)
		return nil, repoErr
	}

	if repoModel == nil {
		log.WithFields(f).Warnf("CombinedRepository does not exist for *Repository: %s ", repositoryID)
		return nil, ErrRepositoryDoesNotExist
	}

	expressionAttributeNames := map[string]*string{}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{}
	updateExpression := "SET "

	// Convert the numeric value to a string for the DB
	externalIDStr := strconv.FormatInt(repoModel.RepositoryExternalID, 10)
	if externalID != "" && externalIDStr != externalID {
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

	if note != "" {
		log.WithFields(f).Debugf("adding note: %s ", note)
		noteValue := note
		if !strings.HasSuffix(noteValue, ".") {
			noteValue = fmt.Sprintf("%s.", noteValue)
		}
		// If we have a previous value - just concat the value to the end
		if repoModel.Note != "" {
			if strings.HasSuffix(strings.TrimSpace(repoModel.Note), ".") {
				noteValue = fmt.Sprintf("%s %s", repoModel.Note, noteValue)
			} else {
				noteValue = fmt.Sprintf("%s. %s", repoModel.Note, noteValue)
			}
		}
		expressionAttributeNames["#NO"] = aws.String("note")
		expressionAttributeValues[":no"] = &dynamodb.AttributeValue{S: aws.String(noteValue)}
		updateExpression = updateExpression + " #NO = :no, "
	}

	if input.Enabled != nil && repoModel.Enabled != *input.Enabled {
		log.WithFields(f).Debugf("adding enabled flag: %+v", *input.Enabled)
		expressionAttributeNames["#EN"] = aws.String("enabled")
		expressionAttributeValues[":en"] = &dynamodb.AttributeValue{BOOL: input.Enabled}
		updateExpression = updateExpression + " #EN = :en, "
	}

	if projectSFID != "" && repoModel.RepositoryProjectSfid != projectSFID {
		log.WithFields(f).Debugf("adding projectSFID : %s ", projectSFID)
		expressionAttributeNames["#P"] = aws.String("project_sfid")
		expressionAttributeValues[":p"] = &dynamodb.AttributeValue{S: aws.String(projectSFID)}
		updateExpression = updateExpression + " #P = :p, "
	}

	if parentProjectSFID != "" {
		log.WithFields(f).Debugf("adding parentProjectSFID : %s ", parentProjectSFID)
		expressionAttributeNames["#PP"] = aws.String("repository_sfdc_id")
		expressionAttributeValues[":pp"] = &dynamodb.AttributeValue{S: aws.String(parentProjectSFID)}
		updateExpression = updateExpression + " #PP = :pp, "
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

	return r.GitHubGetRepository(ctx, repositoryID)
}

// GitHubUpdateClaGroupID updates the claGroupID of the repository
func (r *Repository) GitHubUpdateClaGroupID(ctx context.Context, repositoryID, claGroupID string) error {
	return r.setClaGroupIDGithubRepository(ctx, repositoryID, claGroupID)
}

// GitHubEnableRepository enables the repository entry
func (r *Repository) GitHubEnableRepository(ctx context.Context, repositoryID string) error {
	return r.enableGithubRepository(ctx, repositoryID)
}

// GitHubEnableRepositoryWithCLAGroupID enables the repository entry with the specified CLA Group ID
func (r *Repository) GitHubEnableRepositoryWithCLAGroupID(ctx context.Context, repositoryID, claGroupID string) error {
	return r.enableGithubRepositoryWithCLAGroupID(ctx, repositoryID, claGroupID)
}

// GitHubDisableRepository disables the repository entry (we don't delete)
func (r *Repository) GitHubDisableRepository(ctx context.Context, repositoryID string) error {
	return r.disableGithubRepository(ctx, repositoryID)
}

// GitHubDisableRepositoriesByProjectID  disables the repository by the project ID
func (r *Repository) GitHubDisableRepositoriesByProjectID(ctx context.Context, projectID string) error {
	repoModels, err := r.getProjectRepositories(ctx, projectID, true)
	if err != nil {
		return err
	}

	// For each model...
	for _, repoModel := range repoModels {
		disableErr := r.GitHubDisableRepository(ctx, repoModel.RepositoryID)
		if disableErr != nil {
			return disableErr
		}
	}

	return nil
}

// GitHubDisableRepositoriesOfOrganization disables the repositories under the GitHub organization
func (r *Repository) GitHubDisableRepositoriesOfOrganization(ctx context.Context, projectSFID, githubOrgName string) error {
	repoModels, err := r.getRepositoriesByGithubOrg(ctx, githubOrgName)
	if err != nil {
		return err
	}
	for _, repoModel := range repoModels {
		if repoModel.RepositoryProjectSfid == projectSFID {
			err = r.disableGithubRepository(ctx, repoModel.RepositoryID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// GitHubGetRepository by repository id
func (r *Repository) GitHubGetRepository(ctx context.Context, repositoryID string) (*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "v1.repositories.repository.GitHubGetRepository",
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

	return out.ToGitHubModel(), nil
}

// GitHubGetRepositoryByName fetches the repository by repository name
func (r *Repository) GitHubGetRepositoryByName(ctx context.Context, repositoryName string) (*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "v1.repositories.repository.GitHubGetRepositoryByName",
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
		log.WithFields(f).Warnf("no repositories found with repository name: %s", repositoryName)
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

	if len(repositories) > 1 {
		log.WithFields(f).Warn("multiple repositories records with the same repository name")
	}

	return repositories[0].ToGitHubModel(), nil
}

// GitHubGetRepositoryByExternalID fetches the repository by repository ID
func (r *Repository) GitHubGetRepositoryByExternalID(ctx context.Context, repositoryExternalID string) (*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":         "v1.repositories.repository.GitHubGetRepositoryByExternalID",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"repositoryExternalID": repositoryExternalID,
	}
	builder := expression.NewBuilder()
	condition := expression.Key("repository_external_id").Equal(expression.Value(repositoryExternalID))
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
		IndexName:                 aws.String(RepositoryExternalIDIndex),
	}

	results, err := r.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to get repositories by name")
		return nil, err
	}

	if len(results.Items) == 0 {
		log.WithFields(f).Warnf("no repositories found with repository external ID: %s", repositoryExternalID)
		return nil, &utils.GitHubRepositoryNotFound{
			RepositoryName: repositoryExternalID,
		}
	}

	var repositories []*RepositoryDBModel
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &repositories)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem unmarshalling response")
		return nil, err
	}

	if len(repositories) > 1 {
		log.WithFields(f).Warn("multiple repositories records with the same repository name")
	}

	return repositories[0].ToGitHubModel(), nil
}

// GitHubGetRepositoriesByCLAGroup gets the list of repositories based on the CLA Group ID
func (r *Repository) GitHubGetRepositoriesByCLAGroup(ctx context.Context, claGroupID string, enabled bool) ([]*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "v1.repositories.repository.GetRepositoryByCLAGroup",
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
		IndexName:                 aws.String(RepositoryProjectIndex),
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

// GitHubGetRepositoriesByOrganizationName gets the repositories by organization name
func (r *Repository) GitHubGetRepositoriesByOrganizationName(ctx context.Context, gitHubOrgName string) ([]*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "v1.repositories.repository.GitHubGetRepositoriesByOrganizationName",
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
		log.WithFields(f).Debug(msg)
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

// GitHubGetCLAGroupRepositoriesGroupByOrgs returns a list of GH organizations by CLA Group - enabled flag indicates that we search the enabled repositories list
func (r *Repository) GitHubGetCLAGroupRepositoriesGroupByOrgs(ctx context.Context, projectID string, enabled bool) ([]*models.GithubRepositoriesGroupByOrgs, error) {
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

// GitHubListProjectRepositories lists GitHub repositories of project by external/salesforce project id
func (r *Repository) GitHubListProjectRepositories(ctx context.Context, projectSFID string, enabled *bool) (*models.GithubListRepositories, error) {
	f := logrus.Fields{
		"functionName":   "v1.repositories.repository.GitHubListProjectRepositories",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"enabled":        utils.BoolValue(enabled),
	}

	out := &models.GithubListRepositories{
		List: make([]*models.GithubRepository, 0),
	}

	condition := expression.Key("project_sfid").Equal(expression.Value(projectSFID))

	// Add the enabled filter, if set
	var filter expression.ConditionBuilder
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
		IndexName:                 aws.String(RepositoryProjectSFIDOrganizationNameIndex),
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
		out.List = append(out.List, gr.ToGitHubModel())
	}
	return out, nil
}

// getProjectRepositories returns an array of GH repositories for the specified project ID
func (r *Repository) getProjectRepositories(ctx context.Context, projectID string, enabled bool) ([]*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "v1.repositories.repository.getProjectRepositories",
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
		IndexName:                 aws.String(RepositoryProjectIndex),
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
		out = append(out, gr.ToGitHubModel())
	}
	return out, nil
}

// getRepositoriesByGithubOrg returns an array of GH repositories for the specified project ID
func (r *Repository) getRepositoriesByGithubOrg(ctx context.Context, githubOrgName string) ([]*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "v1.repositories.repository.getRepositoriesByGitHubOrg",
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
		out = append(out, gr.ToGitHubModel())
	}
	return out, nil
}

// GitHubGetRepositoryByGithubID fetches the repository model by its external GitHub id
func (r *Repository) GitHubGetRepositoryByGithubID(ctx context.Context, externalID string, enabled bool) (*models.GithubRepository, error) {
	f := logrus.Fields{
		"functionName":   "v1.repositories.repository.GetRepositoryByGitHubID",
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
		IndexName:                 aws.String(RepositoryExternalIDIndex),
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

	return result.ToGitHubModel(), nil
}

func (r *Repository) enableGithubRepository(ctx context.Context, repositoryID string) error {
	return r.setEnabledGithubRepository(ctx, repositoryID, true)
}

func (r *Repository) enableGithubRepositoryWithCLAGroupID(ctx context.Context, repositoryID, claGroupID string) error {
	return r.setEnabledGithubRepositoryWithCLAGroupID(ctx, repositoryID, claGroupID, true)
}

func (r *Repository) disableGithubRepository(ctx context.Context, repositoryID string) error {
	return r.setEnabledGithubRepository(ctx, repositoryID, false)
}

// setEnabledGithubRepository updates the existing repository record by setting the enabled flag to false
func (r *Repository) setEnabledGithubRepository(ctx context.Context, repositoryID string, enabled bool) error {
	f := logrus.Fields{
		"functionName":   "v1.repositories.repository.setEnabledGitHubRepository",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"repositoryID":   repositoryID,
		"enabled":        enabled,
	}

	// Load the existing model - need to fetch the old values, if available
	existingModel, getErr := r.GitHubGetRepository(ctx, repositoryID)
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

	note := fmt.Sprintf("%s%s on %s", existingNote, enabledString, now)

	updateExpression := expression.Set(expression.Name(repositoryEnabledColumn), expression.Value(enabled)).Set(expression.Name("note"), expression.Value(note)).Set(expression.Name("date_modified"), expression.Value(now))

	// delete project_sfid ,repository_sfdc_id and repository_project_id if enabled is false
	if !enabled {
		updateExpression = updateExpression.Remove(expression.Name("project_sfid")).Remove(expression.Name("repository_sfdc_id")).Remove(expression.Name("repository_project_id"))
	}

	expr, exprErr := expression.NewBuilder().WithUpdate(updateExpression).Build()
	if exprErr != nil {
		log.WithFields(f).Warnf("error building expression for updating repository record, error: %v", exprErr)
		return exprErr
	}

	_, err := r.dynamoDBClient.UpdateItem(&dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"repository_id": {S: aws.String(repositoryID)},
		},
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
		TableName:                 aws.String(r.repositoryTableName),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				return errors.New("github repository entry does not exist or *Repositorysitory_sfdc_id does not match with specified project id")
			}
		}
		log.WithFields(f).WithError(err).Warn("error disabling github repository")
		return err
	}

	return nil
}

// setEnabledGithubRepositoryWithCLAGroupID updates the existing repository record by setting the enabled flag to false
func (r *Repository) setEnabledGithubRepositoryWithCLAGroupID(ctx context.Context, repositoryID, claGroupID string, enabled bool) error {
	f := logrus.Fields{
		"functionName":   "v1.repositories.repository.setEnabledGitHubRepositoryWithCLAGroupID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"repositoryID":   repositoryID,
		"claGroupID":     claGroupID,
		"enabled":        enabled,
	}

	// Load the existing model - need to fetch the old note value, if available
	existingModel, getErr := r.GitHubGetRepository(ctx, repositoryID)
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
				return errors.New("github repository entry does not exist or *Repositorysitory_sfdc_id does not match with specified project id")
			}
		}
		log.WithFields(f).WithError(err).Warn("error disabling github repository")
		return err
	}

	return nil
}

// setEnabledGithubRepository updates the existing repository record by setting the enabled flag to false
func (r *Repository) setClaGroupIDGithubRepository(ctx context.Context, repositoryID, claGroupID string) error {
	f := logrus.Fields{
		"functionName":   "v1.repositories.repository.setClaGroupIDGitHubRepository",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"repositoryID":   repositoryID,
		"claGroupID":     claGroupID,
	}

	// Load the existing model - need to fetch the old note value, if available
	existingModel, getErr := r.GitHubGetRepository(ctx, repositoryID)
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
				return errors.New("github repository entry does not exist or *Repositorysitory_sfdc_id does not match with specified project id")
			}
		}
		log.WithFields(f).WithError(err).Warn("error disabling github repository")
		return err
	}

	return nil
}
