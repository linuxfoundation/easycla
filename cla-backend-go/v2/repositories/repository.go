// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	repoModels "github.com/communitybridge/easycla/cla-backend-go/repositories"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
)

// RepositoryInterface interface defines the functions for the GitLab repository data model
type RepositoryInterface interface {
	GitLabGetRepository(ctx context.Context, repositoryID string) (*repoModels.RepositoryDBModel, error)
	GitLabGetRepositoryByName(ctx context.Context, repositoryName string) (*repoModels.RepositoryDBModel, error)
	GitHubGetRepositoriesByCLAGroup(ctx context.Context, claGroupID string) ([]*repoModels.RepositoryDBModel, error)
	GitHubGetRepositoriesByCLAGroupEnabled(ctx context.Context, claGroupID string) ([]*repoModels.RepositoryDBModel, error)
	GitHubGetRepositoriesByCLAGroupDisabled(ctx context.Context, claGroupID string) ([]*repoModels.RepositoryDBModel, error)
	GitHubGetRepositoriesByProjectSFID(ctx context.Context, projectSFID string) ([]*repoModels.RepositoryDBModel, error)
	GitHubGetRepositoriesByOrganizationName(ctx context.Context, orgName string) ([]*repoModels.RepositoryDBModel, error)
	GitLabAddRepository(ctx context.Context, projectSFID string, input *repoModels.RepositoryDBModel) (*repoModels.RepositoryDBModel, error)
	GitLabEnableRepositoryByID(ctx context.Context, repositoryID string) error
	GitLabDisableRepositoryByID(ctx context.Context, repositoryID string) error
	GitLabDisableCLAGroupRepositories(ctx context.Context, claGroupID string) error
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

// GitHubGetRepositoriesByOrganizationName returns a list of repositories associated with the specified organization name
func (r *Repository) GitHubGetRepositoriesByOrganizationName(ctx context.Context, orgName string) ([]*repoModels.RepositoryDBModel, error) {
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
			Message:        fmt.Sprintf("GitLab repository with name: %s has alerady been registered", input.RepositoryName),
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

// GitLabEnableRepositoryByID enables the specified repository
func (r *Repository) GitLabEnableRepositoryByID(ctx context.Context, repositoryID string) error {
	return r.setRepositoryEnabledValue(ctx, repositoryID, true)
}

// GitLabDisableRepositoryByID disables the specified repository
func (r *Repository) GitLabDisableRepositoryByID(ctx context.Context, repositoryID string) error {
	return r.setRepositoryEnabledValue(ctx, repositoryID, false)
}

// GitLabEnableCLAGroupRepositories enables the specified CLA Group repositories
func (r *Repository) GitLabEnableCLAGroupRepositories(ctx context.Context, claGroupID string) error {
	repositories, err := r.GitHubGetRepositoriesByCLAGroup(ctx, claGroupID)
	if err != nil {
		return err
	}

	for _, repo := range repositories {
		enableErr := r.GitLabEnableRepositoryByID(ctx, repo.RepositoryID)
		if enableErr != nil {
			return enableErr
		}
	}

	return nil
}

// GitLabDisableCLAGroupRepositories disables the GitLab repositories by the specified CLA Group
func (r *Repository) GitLabDisableCLAGroupRepositories(ctx context.Context, claGroupID string) error {
	repositories, err := r.GitHubGetRepositoriesByCLAGroup(ctx, claGroupID)
	if err != nil {
		return err
	}

	for _, repo := range repositories {
		enableErr := r.GitLabDisableRepositoryByID(ctx, repo.RepositoryID)
		if enableErr != nil {
			return enableErr
		}
	}

	return nil
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

	if len(repositories) > 1 {
		log.WithFields(f).Warn("multiple repositories records with the same repository name and type found")
		// Generic - no details as we don't know what filter content was provided
		return nil, &utils.GitLabDuplicateRepositoriesFound{}
	}

	return repositories[0], nil
}

// getRepositoriesWithConditionFilter fetches the repository entry based on the specified condition and filter criteria
// using the provided index
func (r *Repository) getRepositoriesWithConditionFilter(ctx context.Context, condition expression.KeyConditionBuilder, filter expression.ConditionBuilder, indexName string) ([]*repoModels.RepositoryDBModel, error) {
	f := logrus.Fields{
		"functionName":   "v2.repositories.repository.getRepositoriesWithConditionFilter",
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
func (r *Repository) setRepositoryEnabledValue(ctx context.Context, repositoryID string, enabledValue bool) error {
	// Load the existing model - need to fetch the old values, if available
	existingModel, getErr := r.GitLabGetRepository(ctx, repositoryID)
	if getErr != nil {
		return getErr
	}
	if existingModel == nil {
		return fmt.Errorf("unable to locate existing repository entry by ID: %s", repositoryID)
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

	_, now := utils.CurrentTime()
	_, err := r.dynamoDBClient.UpdateItem(&dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"repository_id": {S: aws.String(repositoryID)},
		},
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
				S: aws.String(fmt.Sprintf("%s Updated enabled flag to %t on %s.", existingNote, enabledValue, now)),
			},
			":dateModifiedValue": {
				S: aws.String(now),
			},
		},
		UpdateExpression: aws.String("SET #enabled = :enabledValue, #note = :noteValue, #dateModified = :dateModifiedValue"),
		TableName:        aws.String(r.repositoryTableName),
	})

	return err
}
