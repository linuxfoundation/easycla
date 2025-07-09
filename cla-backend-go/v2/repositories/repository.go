// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/gofrs/uuid"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	repoModels "github.com/linuxfoundation/easycla/cla-backend-go/repositories"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// RepositoryInterface interface defines the functions for the GitLab repository data model
type RepositoryInterface interface {
	GitHubGetRepositoriesByCLAGroup(ctx context.Context, claGroupID string) ([]*repoModels.RepositoryDBModel, error)
	GitHubGetRepositoriesByCLAGroupEnabled(ctx context.Context, claGroupID string) ([]*repoModels.RepositoryDBModel, error)
	GitHubGetRepositoriesByCLAGroupDisabled(ctx context.Context, claGroupID string) ([]*repoModels.RepositoryDBModel, error)
	GitHubGetRepositoriesByProjectSFID(ctx context.Context, projectSFID string) ([]*repoModels.RepositoryDBModel, error)
	GitHubGetRepositoriesByOrganizationName(ctx context.Context, orgName string) ([]*repoModels.RepositoryDBModel, error)
	GetRepositoriesByProjectSFID(ctx context.Context, projectSFID string) ([]*repoModels.RepositoryDBModel, error)

	GitLabGetRepository(ctx context.Context, repositoryID string) (*repoModels.RepositoryDBModel, error)
	GitLabGetRepositoryByName(ctx context.Context, repositoryName string) (*repoModels.RepositoryDBModel, error)
	GitLabGetRepositoriesByOrganizationName(ctx context.Context, orgName string) ([]*repoModels.RepositoryDBModel, error)
	GitLabGetRepositoriesByNamePrefix(ctx context.Context, repositoryNamePrefix string) ([]*repoModels.RepositoryDBModel, error)
	GitLabGetRepositoryByExternalID(ctx context.Context, repositoryExternalID int64) (*repoModels.RepositoryDBModel, error)
	GitLabAddRepository(ctx context.Context, projectSFID string, input *repoModels.RepositoryDBModel) (*repoModels.RepositoryDBModel, error)
	GitLabEnrollRepositoryByID(ctx context.Context, claGroupID string, repositoryID int64, enrollValue bool) error
	GitLabEnableCLAGroupRepositories(ctx context.Context, claGroupID string, enrollValue bool) error
	GitLabDeleteRepositories(ctx context.Context, gitLabGroupPath string) error
	GitLabDeleteRepositoryByExternalID(ctx context.Context, gitLabExternalID int64) error
}

// Repository object/struct
type Repository struct {
	stage               string
	dynamoDBClient      *dynamodb.DynamoDB
	repositoryTableName string
	gitLabOrgTableName  string
}

// NewRepository creates a new instance of the GitLab repository service
func NewRepository(awsSession *session.Session, stage string) *Repository {
	return &Repository{
		stage:               stage,
		dynamoDBClient:      dynamodb.New(awsSession),
		repositoryTableName: fmt.Sprintf("cla-%s-repositories", stage),
		gitLabOrgTableName:  fmt.Sprintf("cla-%s-gitlab-orgs", stage),
	}
}

// GitLabGetRepository returns the database model for the internal repository ID
func (r *Repository) GitLabGetRepository(ctx context.Context, repositoryID string) (*repoModels.RepositoryDBModel, error) {
	f := logrus.Fields{
		"functionName":   "v2.repositories.repositories.GitLabGetRepository",
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
		log.WithFields(f).WithError(err).Warn("problem querying using repository ID")
		return nil, err
	}
	if len(result.Item) == 0 {
		msg := fmt.Sprintf("repository with ID: %s does not exist", repositoryID)
		log.WithFields(f).Warn(msg)
		return nil, &utils.GitHubRepositoryNotFound{
			Message: msg,
		}
	}

	// Decode the results into a model
	var out repoModels.RepositoryDBModel
	err = dynamodbattribute.UnmarshalMap(result.Item, &out)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem unmarshalling database repository response")
		return nil, err
	}

	return &out, nil
}

// GitLabGetRepositoryByName returns the database model for the specified repository
func (r *Repository) GitLabGetRepositoryByName(ctx context.Context, repositoryName string) (*repoModels.RepositoryDBModel, error) {
	condition := expression.Key(repoModels.RepositoryNameColumn).Equal(expression.Value(repositoryName))
	filter := expression.Name(repoModels.RepositoryTypeColumn).Equal(expression.Value(utils.GitLabLower))
	record, err := r.getRepositoryWithConditionFilter(ctx, condition, filter, repoModels.RepositoryNameIndex)
	if err != nil {
		// Catch the error - return the same error with the appropriate details
		if _, ok := err.(*utils.GitLabRepositoryNotFound); ok {
			return nil, &utils.GitLabRepositoryNotFound{
				RepositoryName: repositoryName,
			}
		}
		// Catch the error - return the same error with the appropriate details
		if _, ok := err.(*utils.GitLabDuplicateRepositoriesFound); ok {
			return nil, &utils.GitLabDuplicateRepositoriesFound{
				RepositoryName: repositoryName,
			}
		}
		// Some other error
		return nil, err
	}

	return record, nil
}

// GitLabGetRepositoriesByNamePrefix returns a list of repositories matching the specified name prefix
func (r *Repository) GitLabGetRepositoriesByNamePrefix(ctx context.Context, repositoryNamePrefix string) ([]*repoModels.RepositoryDBModel, error) {
	f := logrus.Fields{
		"functionName":         "v2.repositories.repositories.GitLabGetRepositoriesByNamePrefix",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"repositoryNamePrefix": repositoryNamePrefix,
	}

	log.WithFields(f).Debug("querying for repositories with name prefix")
	condition := expression.Key(repoModels.RepositoryTypeColumn).Equal(expression.Value(utils.GitLabLower))
	filter := expression.Name(repoModels.RepositoryNameColumn).BeginsWith(repositoryNamePrefix)
	records, err := r.getRepositoriesWithConditionFilter(ctx, condition, filter, repoModels.RepositoryTypeIndex)
	if err != nil {
		// Catch the error - return the same error with the appropriate details
		if _, ok := err.(*utils.GitLabRepositoryNotFound); ok {
			return nil, &utils.GitLabRepositoryNotFound{
				RepositoryName: repositoryNamePrefix,
			}
		}
		// Catch the error - return the same error with the appropriate details
		if _, ok := err.(*utils.GitLabDuplicateRepositoriesFound); ok {
			return nil, &utils.GitLabDuplicateRepositoriesFound{
				RepositoryName: repositoryNamePrefix,
			}
		}
		// Some other error
		return nil, err
	}

	return records, nil
}

// GitLabGetRepositoryByExternalID returns the database model for the specified repository by external ID
func (r *Repository) GitLabGetRepositoryByExternalID(ctx context.Context, repositoryExternalID int64) (*repoModels.RepositoryDBModel, error) {
	str := strconv.FormatInt(repositoryExternalID, 10)
	condition := expression.Key(repoModels.RepositoryExternalIDColumn).Equal(expression.Value(str))
	filter := expression.Name(repoModels.RepositoryTypeColumn).Equal(expression.Value(utils.GitLabLower))
	record, err := r.getRepositoryWithConditionFilter(ctx, condition, filter, repoModels.RepositoryExternalIDIndex)
	if err != nil {
		// Catch the error - return the same error with the appropriate details
		if _, ok := err.(*utils.GitLabRepositoryNotFound); ok {
			return nil, &utils.GitLabRepositoryNotFound{
				RepositoryExternalID: repositoryExternalID,
			}
		}
		// Catch the error - return the same error with the appropriate details
		if _, ok := err.(*utils.GitLabDuplicateRepositoriesFound); ok {
			return nil, &utils.GitLabDuplicateRepositoriesFound{
				RepositoryExternalID: repositoryExternalID,
			}
		}
		// Some other error
		return nil, err
	}

	return record, nil
}

// GitHubGetRepositoriesByCLAGroup returns the database models for the specified CLA Group ID
func (r *Repository) GitHubGetRepositoriesByCLAGroup(ctx context.Context, claGroupID string) ([]*repoModels.RepositoryDBModel, error) {
	condition := expression.Key(repoModels.RepositoryCLAGroupIDColumn).Equal(expression.Value(claGroupID))
	filter := expression.Name(repoModels.RepositoryTypeColumn).Equal(expression.Value(utils.GitLabLower))
	records, err := r.getRepositoriesWithConditionFilter(ctx, condition, filter, repoModels.RepositoryProjectIndex)
	if err != nil {
		// Catch the error - return the same error with the appropriate details
		if _, ok := err.(*utils.GitLabRepositoryNotFound); ok {
			return nil, &utils.GitLabRepositoryNotFound{
				CLAGroupID: claGroupID,
			}
		}

		// Some other error
		return nil, err
	}

	return records, nil
}

// GitHubGetRepositoriesByCLAGroupEnabled returns the database models for the specified CLA Group ID that are enabled
func (r *Repository) GitHubGetRepositoriesByCLAGroupEnabled(ctx context.Context, claGroupID string) ([]*repoModels.RepositoryDBModel, error) {
	condition := expression.Key(repoModels.RepositoryCLAGroupIDColumn).Equal(expression.Value(claGroupID))
	filter := expression.Name(repoModels.RepositoryTypeColumn).Equal(expression.Value(utils.GitLabLower)).
		And(expression.Name(repoModels.RepositoryEnabledColumn).Equal(expression.Value(true)))
	records, err := r.getRepositoriesWithConditionFilter(ctx, condition, filter, repoModels.RepositoryProjectIndex)
	if err != nil {
		// Catch the error - return the same error with the appropriate details
		if _, ok := err.(*utils.GitLabRepositoryNotFound); ok {
			return nil, &utils.GitLabRepositoryNotFound{
				CLAGroupID: claGroupID,
			}
		}

		// Some other error
		return nil, err
	}

	return records, nil
}

// GitHubGetRepositoriesByCLAGroupDisabled returns the database models for the specified CLA Group ID that are disabled
func (r *Repository) GitHubGetRepositoriesByCLAGroupDisabled(ctx context.Context, claGroupID string) ([]*repoModels.RepositoryDBModel, error) {
	condition := expression.Key(repoModels.RepositoryCLAGroupIDColumn).Equal(expression.Value(claGroupID))
	filter := expression.Name(repoModels.RepositoryTypeColumn).Equal(expression.Value(utils.GitLabLower)).
		And(expression.Name(repoModels.RepositoryEnabledColumn).Equal(expression.Value(false)))
	records, err := r.getRepositoriesWithConditionFilter(ctx, condition, filter, repoModels.RepositoryProjectIndex)
	if err != nil {
		// Catch the error - return the same error with the appropriate details
		if _, ok := err.(*utils.GitLabRepositoryNotFound); ok {
			return nil, &utils.GitLabRepositoryNotFound{
				CLAGroupID: claGroupID,
			}
		}

		// Some other error
		return nil, err
	}

	return records, nil
}

// GetRepositoriesByProjectSFID returns a list of repositories associated with the specified project
func (r *Repository) GetRepositoriesByProjectSFID(ctx context.Context, projectSFID string) ([]*repoModels.RepositoryDBModel, error) {
	condition := expression.Key(repoModels.RepositoryProjectIDColumn).Equal(expression.Value(projectSFID))
	records, err := r.getRepositoriesWithConditionFilter(ctx, condition, expression.ConditionBuilder{}, repoModels.RepositoryProjectSFIDIndex)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// GitHubGetRepositoriesByProjectSFID returns a list of repositories associated with the specified project
func (r *Repository) GitHubGetRepositoriesByProjectSFID(ctx context.Context, projectSFID string) ([]*repoModels.RepositoryDBModel, error) {
	condition := expression.Key(repoModels.RepositoryProjectIDColumn).Equal(expression.Value(projectSFID))
	filter := expression.Name(repoModels.RepositoryTypeColumn).Equal(expression.Value(utils.GitLabLower))

	records, err := r.getRepositoriesWithConditionFilter(ctx, condition, filter, repoModels.RepositoryProjectSFIDIndex)
	if err != nil {
		// Catch the error - return the same error with the appropriate details
		if _, ok := err.(*utils.GitLabRepositoryNotFound); ok {
			return nil, &utils.GitLabRepositoryNotFound{
				ProjectSFID: projectSFID,
			}
		}

		// Some other error
		return nil, err
	}

	return records, nil
}

// GitHubGetRepositoriesByOrganizationName returns a list of GitHub repositories associated with the specified organization name
func (r *Repository) GitHubGetRepositoriesByOrganizationName(ctx context.Context, orgName string) ([]*repoModels.RepositoryDBModel, error) {
	condition := expression.Key(repoModels.RepositoryOrganizationNameColumn).Equal(expression.Value(orgName))
	filter := expression.Name(repoModels.RepositoryTypeColumn).Equal(expression.Value(utils.GitHubType))

	records, err := r.getRepositoriesWithConditionFilter(ctx, condition, filter, repoModels.RepositoryOrganizationNameIndex)
	if err != nil {
		// Catch the error - return the same error with the appropriate details
		if _, ok := err.(*utils.GitLabRepositoryNotFound); ok {
			return nil, &utils.GitLabRepositoryNotFound{
				OrganizationName: orgName,
			}
		}

		// Some other error
		return nil, err
	}

	return records, nil
}

// GitLabGetRepositoriesByOrganizationName returns a list of GitLab repositories associated with the specified organization name
func (r *Repository) GitLabGetRepositoriesByOrganizationName(ctx context.Context, orgName string) ([]*repoModels.RepositoryDBModel, error) {
	condition := expression.Key(repoModels.RepositoryOrganizationNameColumn).Equal(expression.Value(orgName))
	filter := expression.Name(repoModels.RepositoryTypeColumn).Equal(expression.Value(utils.GitLabLower))

	records, err := r.getRepositoriesWithConditionFilter(ctx, condition, filter, repoModels.RepositoryOrganizationNameIndex)
	if err != nil {
		// Catch the error - return the same error with the appropriate details
		if _, ok := err.(*utils.GitLabRepositoryNotFound); ok {
			return nil, &utils.GitLabRepositoryNotFound{
				OrganizationName: orgName,
			}
		}

		// Some other error
		return nil, err
	}

	return records, nil
}

// GitLabAddRepository creates a new entry in the repositories table using the specified input parameters
func (r *Repository) GitLabAddRepository(ctx context.Context, projectSFID string, input *repoModels.RepositoryDBModel) (*repoModels.RepositoryDBModel, error) {
	f := logrus.Fields{
		"functionName":               "v2.repositories.repositories.GitHubAddRepositories",
		utils.XREQUESTID:             ctx.Value(utils.XREQUESTID),
		"projectSFID":                projectSFID,
		"repositoryExternalID":       input.RepositoryExternalID,
		"repositoryURL":              input.RepositoryURL,
		"repositoryName":             input.RepositoryName,
		"repositoryFullPath":         input.RepositoryFullPath,
		"repositoryType":             utils.GitLabLower,
		"repositoryCLAGroupID":       input.RepositoryCLAGroupID,
		"repositoryProjectSFID":      input.RepositorySfdcID,
		"repositoryOrganizationName": input.RepositoryOrganizationName,
	}

	// Check first to see if the repository already exists
	_, err := r.GitLabGetRepositoryByName(ctx, input.RepositoryName)
	if err != nil {
		// Expecting Not found - no issue if not found - all other error we throw
		if _, ok := err.(*utils.GitLabRepositoryNotFound); !ok {
			return nil, err
		}
	} else {
		return nil, &utils.GitLabRepositoryExists{
			Message:        fmt.Sprintf("GitLab repository with name: %s has already been registered", input.RepositoryName),
			RepositoryName: "",
			Err:            nil,
		}
	}

	_, currentTime := utils.CurrentTime()
	repoID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	input.RepositoryID = repoID.String()
	input.DateCreated = currentTime
	input.DateModified = currentTime
	input.Note = fmt.Sprintf("created on %s", currentTime)
	input.Version = "v1"

	av, err := dynamodbattribute.MarshalMap(input)
	if err != nil {
		log.WithFields(f).Warnf("problem marshalling the input, error: %+v", err)
		return nil, err
	}

	_, err = r.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(r.repositoryTableName),
	})
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to add github repository")
		return nil, err
	}

	return input, nil
}

// GitLabEnrollRepositoryByID enables the specified repository
func (r *Repository) GitLabEnrollRepositoryByID(ctx context.Context, claGroupID string, repositoryExternalID int64, enrollValue bool) error {
	return r.setRepositoryEnabledValue(ctx, claGroupID, repositoryExternalID, enrollValue)
}

// GitLabEnableCLAGroupRepositories enables the specified CLA Group repositories
func (r *Repository) GitLabEnableCLAGroupRepositories(ctx context.Context, claGroupID string, enrollValue bool) error {
	repositories, err := r.GitHubGetRepositoriesByCLAGroup(ctx, claGroupID)
	if err != nil {
		return err
	}

	for _, repo := range repositories {
		int64I, parseErr := strconv.ParseInt(repo.RepositoryExternalID, 10, 64)
		if parseErr != nil {
			return parseErr
		}

		enableErr := r.GitLabEnrollRepositoryByID(ctx, claGroupID, int64I, enrollValue)
		if enableErr != nil {
			return enableErr
		}
	}

	return nil
}

// GitLabDeleteRepositories deletes the specified repositories under the GitLap group path
func (r *Repository) GitLabDeleteRepositories(ctx context.Context, gitLabGroupPath string) error {
	f := logrus.Fields{
		"functionName":    "v2.repositories.repository.GitLabDeleteRepositories",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"gitLabGroupPath": gitLabGroupPath,
	}

	log.WithFields(f).Debugf("loading repositories with name prefix: %s", gitLabGroupPath)
	repositories, err := r.GitLabGetRepositoriesByNamePrefix(ctx, gitLabGroupPath)
	if err != nil {
		// If nothing to delete...
		if _, ok := err.(*utils.GitLabRepositoryNotFound); ok {
			return nil
		}
		log.WithFields(f).WithError(err).Warnf("problem loading repositories with name prefix: %s", gitLabGroupPath)
		return err
	}
	log.WithFields(f).Debugf("processing repository delete request for %d repositories", len(repositories))

	type GitLabDeleteRepositoryResponse struct {
		RepositoryID       string
		RepositoryName     string
		RepositoryFullPath string
		Error              error
	}
	deleteRepoRespChan := make(chan *GitLabDeleteRepositoryResponse, len(repositories))

	for _, repo := range repositories {
		go func(repo *repoModels.RepositoryDBModel) {
			_, err = r.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
				Key: map[string]*dynamodb.AttributeValue{
					repoModels.RepositoryIDColumn: {S: aws.String(repo.RepositoryID)},
				},
				TableName: aws.String(r.repositoryTableName),
			})
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("error deleting repository with ID:%s", repo.RepositoryID)
			}
			deleteRepoRespChan <- &GitLabDeleteRepositoryResponse{
				RepositoryID:       repo.RepositoryID,
				RepositoryName:     repo.RepositoryName,
				RepositoryFullPath: repo.RepositoryFullPath,
				Error:              err,
			}
		}(repo)
	}

	// Wait for the go routines to finish and load up the results
	log.WithFields(f).Debug("waiting for delete repos to finish...")
	var lastErr error
	for range repositories {
		select {
		case response := <-deleteRepoRespChan:
			if response.Error != nil {
				log.WithFields(f).WithError(response.Error).Warn(response.Error.Error())
				lastErr = response.Error
			} else {
				log.WithFields(f).Debugf("delete repo: %s with ID: %s with full path: %s", response.RepositoryName, response.RepositoryID, response.RepositoryFullPath)
			}
		case <-ctx.Done():
			log.WithFields(f).WithError(ctx.Err()).Warnf("waiting for delete repositories timed out")
			lastErr = fmt.Errorf("delete repositories failed with timeout, error: %v", ctx.Err())
		}
	}

	// Return the last error, hopefully nil if no error occurred...
	return lastErr
}

// GitLabDeleteRepositoryByExternalID deletes the specified repository
func (r *Repository) GitLabDeleteRepositoryByExternalID(ctx context.Context, gitLabExternalID int64) error {
	f := logrus.Fields{
		"functionName":     "v2.repositories.repository.GitLabDeleteRepositoryByExternalID",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"gitLabExternalID": gitLabExternalID,
	}

	repositoryRecord, err := r.GitLabGetRepositoryByExternalID(ctx, gitLabExternalID)
	if err != nil {
		// If nothing to delete...
		if _, ok := err.(*utils.GitLabRepositoryNotFound); ok {
			return nil
		}
		log.WithFields(f).WithError(err).Warnf("problem loading existing repository by external ID: %d", gitLabExternalID)
		return err
	}
	if repositoryRecord == nil {
		return nil
	}

	_, deleteErr := r.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			repoModels.RepositoryIDColumn: {S: aws.String(repositoryRecord.RepositoryID)},
		},
		TableName: aws.String(r.repositoryTableName),
	})

	// Return the error
	return deleteErr
}

// getRepositoryWithConditionFilter fetches the repository entry based on the specified condition and filter criteria using the provided index
func (r *Repository) getRepositoryWithConditionFilter(ctx context.Context, condition expression.KeyConditionBuilder, filter expression.ConditionBuilder, indexName string) (*repoModels.RepositoryDBModel, error) {
	f := logrus.Fields{
		"functionName":   "v2.repositories.repository.getRepositoryWithConditionFilter",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"indexName":      indexName,
	}

	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter).Build()
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
		IndexName:                 aws.String(indexName),
	}

	results, err := r.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to get repositories using query: %+v", queryInput)
		return nil, err
	}

	if len(results.Items) == 0 {
		// Generic - no details as we don't know what filter content was provided
		return nil, &utils.GitLabRepositoryNotFound{}
	}

	var repositories []*repoModels.RepositoryDBModel
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &repositories)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem unmarshalling response")
		return nil, err
	}

	if len(repositories) > 1 {
		log.WithFields(f).Warn("multiple repositories records with the same repository name and type found")
		// Generic - no details as we don't know what filter content was provided
		return nil, &utils.GitLabDuplicateRepositoriesFound{}
	}

	return repositories[0], nil
}

func isZeroCondition(filter expression.ConditionBuilder) bool {
	return reflect.ValueOf(filter).IsZero()
}

// getRepositoriesWithConditionFilter fetches the repository entry based on the specified condition and filter criteria
// using the provided index
func (r *Repository) getRepositoriesWithConditionFilter(ctx context.Context, condition expression.KeyConditionBuilder, filter expression.ConditionBuilder, indexName string) ([]*repoModels.RepositoryDBModel, error) {
	f := logrus.Fields{
		"functionName":   "v2.repositories.repository.getRepositoriesWithConditionFilter",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"indexName":      indexName,
	}

	builder := expression.NewBuilder().WithKeyCondition(condition)
	if !isZeroCondition(filter) {
		builder = builder.WithFilter(filter)
	}
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
		IndexName:                 aws.String(indexName),
	}

	results, err := r.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to get repositories using query: %+v", queryInput)
		return nil, err
	}

	if len(results.Items) == 0 {
		log.WithFields(f).Debugf("no repositories found matching filter critera: %+v", queryInput)
		// Generic - no details as we don't know what filter content was provided
		return nil, &utils.GitLabRepositoryNotFound{}
	}

	var repositories []*repoModels.RepositoryDBModel
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &repositories)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem unmarshalling response")
		return nil, err
	}

	return repositories, nil
}

// setRepositoryEnabledValue sets the specified repository to the specified enabled value
func (r *Repository) setRepositoryEnabledValue(ctx context.Context, claGroupID string, repositoryExternalID int64, enabledValue bool) error {
	f := logrus.Fields{
		"functionName":         "v2.repositories.repository.setRepositoryEnabledValue",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"claGroupID":           claGroupID,
		"repositoryExternalID": repositoryExternalID,
		"enabledValue":         enabledValue,
	}

	// Load the existing model - need to fetch the old values, if available
	existingModel, getErr := r.GitLabGetRepositoryByExternalID(ctx, repositoryExternalID)
	if getErr != nil {
		return getErr
	}
	if existingModel == nil {
		return fmt.Errorf("unable to locate existing repository entry by external ID: %d", repositoryExternalID)
	}

	// If we have an old note - grab it/save it
	var existingNote = ""
	if existingModel.Note != "" {
		if !strings.HasSuffix(strings.TrimSpace(existingModel.Note), ".") {
			existingNote = strings.TrimSpace(existingModel.Note) + ". "
		} else {
			existingNote = strings.TrimSpace(existingModel.Note) + " "
		}
	}
	userNameFromCtx := utils.GetUserNameFromContext(ctx)
	byUserStr := ""
	if userNameFromCtx != "" {
		byUserStr = fmt.Sprintf("by user: %s", userNameFromCtx)
	}

	_, now := utils.CurrentTime()
	updateInput := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#enabled":      aws.String(repoModels.RepositoryEnabledColumn),
			"#note":         aws.String(repoModels.RepositoryNoteColumn),
			"#dateModified": aws.String(repoModels.RepositoryDateModifiedColumn),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":enabledValue": {
				BOOL: aws.Bool(enabledValue),
			},
			":noteValue": {
				S: aws.String(fmt.Sprintf("%s Updated enabled flag to %t on %s %s.", existingNote, enabledValue, now, byUserStr)),
			},
			":dateModifiedValue": {
				S: aws.String(now),
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			repoModels.RepositoryIDColumn: {S: aws.String(existingModel.RepositoryID)},
		},
		TableName:        aws.String(r.repositoryTableName),
		UpdateExpression: aws.String("SET #enabled = :enabledValue, #note = :noteValue, #dateModified = :dateModifiedValue"),
	}

	if claGroupID != "" {
		updateInput.ExpressionAttributeNames["#claGroupID"] = aws.String(repoModels.RepositoryCLAGroupIDColumn)
		updateInput.ExpressionAttributeValues[":claGroupIDValue"] = &dynamodb.AttributeValue{S: aws.String(claGroupID)}
		updateExpression := fmt.Sprintf("%s, #claGroupID = :claGroupIDValue ", *updateInput.UpdateExpression)
		updateInput.UpdateExpression = aws.String(updateExpression)
	}

	_, err := r.dynamoDBClient.UpdateItem(updateInput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem with update, error: %+v", err.Error())
	}

	return err
}
