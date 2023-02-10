// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	"github.com/aws/aws-sdk-go/service/dynamodb"

	"github.com/aws/aws-lambda-go/events"

	"github.com/aws/aws-sdk-go/service/lambda"

	awslambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

var (
	// version the application version
	version string

	// build/Commit the application build number
	commit string

	// branch the build branch
	branch string

	// build date
	buildDate string
)

// ClaGroup is cla-group dynamodb model
type ClaGroup struct {
	ProjectID          string `json:"project_id"`
	ProjectIclaEnabled bool   `json:"project_icla_enabled"`
	ProjectCclaEnabled bool   `json:"project_ccla_enabled"`
}

// BuildZipEvent is argument to zipbuilder
type BuildZipEvent struct {
	ClaGroupID    string `json:"cla_group_id"`
	SignatureType string `json:"signature_type"`
	FileType      string `json:"file_type"`
}

func handler(ctx context.Context, event events.CloudWatchEvent) {
	var awsSession = session.Must(session.NewSession(&aws.Config{}))
	stage := os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("stage not set")
	}
	dynamoDBClient := dynamodb.New(awsSession)
	claGroups, err := getClaGroups(dynamoDBClient, stage)
	if err != nil {
		log.Error("unable to get cla groups", err)
		return
	}
	var eventPayloads []BuildZipEvent
	for _, claGroup := range claGroups {
		if claGroup.ProjectCclaEnabled {
			eventPayloads = append(eventPayloads, BuildZipEvent{
				ClaGroupID:    claGroup.ProjectID,
				SignatureType: utils.ClaTypeCCLA,
				FileType:      utils.FileTypePDF,
			})
			eventPayloads = append(eventPayloads, BuildZipEvent{
				ClaGroupID:    claGroup.ProjectID,
				SignatureType: utils.ClaTypeCCLA,
				FileType:      utils.FileTypeCSV,
			})
			eventPayloads = append(eventPayloads, BuildZipEvent{
				ClaGroupID:    claGroup.ProjectID,
				SignatureType: utils.ClaTypeECLA,
				FileType:      utils.FileTypeCSV,
			})
		}
		if claGroup.ProjectIclaEnabled {
			eventPayloads = append(eventPayloads, BuildZipEvent{
				ClaGroupID:    claGroup.ProjectID,
				SignatureType: utils.ClaTypeICLA,
				FileType:      utils.FileTypePDF,
			})
			eventPayloads = append(eventPayloads, BuildZipEvent{
				ClaGroupID:    claGroup.ProjectID,
				SignatureType: utils.ClaTypeICLA,
				FileType:      utils.FileTypeCSV,
			})
		}
	}
	if len(eventPayloads) == 0 {
		log.Debug("no cla group found")
		return
	}
	lambdaClient := lambda.New(awsSession)
	wg := &sync.WaitGroup{}
	wg.Add(len(eventPayloads))
	for _, buildZipArg := range eventPayloads {
		go invokeLambda(wg, lambdaClient, stage, buildZipArg)
	}
	wg.Wait()
}

func invokeLambda(wg *sync.WaitGroup, lambdaClient *lambda.Lambda, stage string, buildZipEvent BuildZipEvent) {
	defer wg.Done()
	log.WithField("buildZipEvent", buildZipEvent).Debug("invoking zipbuilder-lambda")
	payload, err := json.Marshal(buildZipEvent)
	if err != nil {
		log.Error("Error marshalling BuildZip request", err)
		return
	}
	functionName := fmt.Sprintf("cla-backend-%s-zipbuilder-lambda", stage)

	_, err = lambdaClient.Invoke(&lambda.InvokeInput{FunctionName: aws.String(functionName), Payload: payload})
	if err != nil {
		log.WithField("input", buildZipEvent).Error("unable to create zip", err)
	}
}

func getClaGroups(dynamoDBClient *dynamodb.DynamoDB, stage string) ([]*ClaGroup, error) {
	var output []*ClaGroup
	tableName := fmt.Sprintf("cla-%s-projects", stage)
	projection := expression.NamesList(
		expression.Name("project_id"),
		expression.Name("project_icla_enabled"),
		expression.Name("project_ccla_enabled"),
	)
	builder := expression.NewBuilder()
	builder = builder.WithProjection(projection)
	expr, err := builder.Build()
	if err != nil {
		log.Warnf("error building expression for %s scan, error: %v", tableName, err)
		return nil, err
	}
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(tableName),
	}
	var resultList []map[string]*dynamodb.AttributeValue
	for {
		results, err := dynamoDBClient.Scan(scanInput) //nolint
		if err != nil {
			log.Warnf("error retrieving %s, error: %v", tableName, err)
			return nil, err
		}
		resultList = append(resultList, results.Items...)
		if len(results.LastEvaluatedKey) != 0 {
			scanInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}
	err = dynamodbattribute.UnmarshalListOfMaps(resultList, &output)
	if err != nil {
		log.Warnf("error unmarshalling %s from database. error: %v", tableName, err)
		return nil, err
	}
	return output, nil
}

func printBuildInfo() {
	log.Infof("Version                 : %s", version)
	log.Infof("Git commit hash         : %s", commit)
	log.Infof("Branch                  : %s", branch)
	log.Infof("Build date              : %s", buildDate)
}

func main() {
	log.Info("Lambda server starting...")
	printBuildInfo()
	if os.Getenv("LOCAL_MODE") == "true" {
		handler(utils.NewContext(), events.CloudWatchEvent{})
	} else {
		awslambda.Start(handler)
	}
	log.Infof("Lambda shutting down...")
}
