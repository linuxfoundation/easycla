// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

var awsSession = session.Must(session.NewSession(&aws.Config{}))
var gitHubRepo RepositoryInterface
var stage string

type RepositoryInterface interface {
	UpdateRepository(ctx context.Context, repositoryID string) error
	GetDisabledRepositories(ctx context.Context) ([]*Repository, error)
}

type repo struct {
	tableName      string
	dynamoDBClient *dynamodb.DynamoDB
	stage          string
}

type Repository struct {
	RepositoryID      string `json:"repository_id"`
	Enabled           bool   `json:"enabled"`
	ProjectSFID       string `json:"project_sfid"`
	ParentProjectSFID string `json:"repository_sfdc_id"`
}

func (repo *repo) UpdateRepository(ctx context.Context, repositoryID string) error {
	updateExpression := expression.Remove(expression.Name("project_sfid")).Remove(expression.Name("repository_sfdc_id"))
	expr, err := expression.NewBuilder().WithUpdate(updateExpression).Build()
	if err != nil {
		return err
	}

	_, err = repo.dynamoDBClient.UpdateItemWithContext(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(repo.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"repository_id": {
				S: aws.String(repositoryID),
			},
		},
		UpdateExpression:         expr.Update(),
		ExpressionAttributeNames: expr.Names(),
	})
	if err != nil {
		return err
	}

	log.Debugf("Updated repository: %s", repositoryID)

	return nil
}

func (repo *repo) GetDisabledRepositories(ctx context.Context) ([]*Repository, error) {
	builder := expression.NewBuilder()
	filter := expression.Name("enabled").Equal(expression.Value(false)).And(expression.Name("project_sfid").AttributeExists()).And(expression.Name("repository_sfdc_id").AttributeExists())
	builder = builder.WithFilter(filter)
	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}
	result, err := repo.dynamoDBClient.ScanWithContext(ctx, &dynamodb.ScanInput{
		TableName:                 aws.String(repo.tableName),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})

	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		log.Warn("No disabled repositories found")
		return nil, nil
	} else {
		log.Debugf("Found %d disabled repositories", len(result.Items))
	}

	var out []*Repository
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func NewRepository(awsSession *session.Session, stage string) RepositoryInterface {
	return &repo{
		tableName:      fmt.Sprintf("cla-%s-repositories", stage),
		dynamoDBClient: dynamodb.New(awsSession),
		stage:          stage,
	}
}

func init() {
	stage = os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("stage not set")
	}
	log.Infof("STAGE set to %s\n", stage)

	gitHubRepo = NewRepository(awsSession, stage)
}

func main() {
	log.Debugf("Getting disabled repositories that have project details...")
	context := context.Background()
	disabledRepos, err := gitHubRepo.GetDisabledRepositories(context)
	if err != nil {
		log.Fatalf("Unable to get disabled repositories, error: %v", err)
	}
	log.Debugf("disabled repositories with existing project details: %v", disabledRepos)
	for _, repo := range disabledRepos {
		log.Debugf("Updating repository: %+v", *repo)
		err := gitHubRepo.UpdateRepository(context, repo.RepositoryID)
		if err != nil {
			log.Fatalf("Unable to update repository: %s, error: %v", repo.RepositoryID, err)
		}
	}
}
