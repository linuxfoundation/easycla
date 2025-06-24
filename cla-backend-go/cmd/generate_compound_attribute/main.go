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
var events EventInterface
var stage string

const (
	tableName = "cla-%s-events"
	attr1     = "event_company_sfid"
	attr2     = "event_cla_group_id"
	newAttr   = "company_sfid_cla_group_id"
	sep       = "#"
)

type EventInterface interface {
	FetchAndUpdateDocuments(ctx context.Context) error
	InsertCompoundAttribute(ctx context.Context, event EventModel) error
}

type Config struct {
	tableName      string
	dynamoDBClient *dynamodb.DynamoDB
	stage          string
}

type EventModel struct {
	EventID          string `json:"event_id"`
	EventCompanySFID string `json:"event_company_sfid"`
	EventCLAGroupID  string `json:"event_cla_group_id"`
}

func (c Config) FetchAndUpdateDocuments(ctx context.Context) error {
	builder := expression.NewBuilder()
	filter := expression.Name(attr1).AttributeExists().And(expression.Name(attr2).AttributeExists()).And(expression.Name(newAttr).AttributeNotExists())
	builder = builder.WithFilter(filter)
	expr, err := builder.Build()
	if err != nil {
		log.Error("stage not set", err)
		return err
	}
	var lastEvaluatedKey string
	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		TableName:                 aws.String(c.tableName),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	var total int
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		results, err := c.dynamoDBClient.ScanWithContext(ctx, scanInput)
		if err != nil {
			log.Error("Found error on ScanWithContext", err)
			return err
		}
		log.Debugf("Found ---> %d Items in a batch", len(results.Items))
		if len(results.Items) > 0 {
			total += len(results.Items)

			var events []EventModel
			err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &events)
			if err != nil {
				log.Error("Found error on UnmarshalListOfMaps", err)
				return err
			}
			for _, event := range events {
				err = c.InsertCompoundAttribute(ctx, event)
				if err != nil {
					log.Error("Found error on InsertCompoundAttribute", err)
					return err
				}
			}
			log.Debugf("All items are updated of the batch %d", len(results.Items))
		}

		if results.LastEvaluatedKey["event_id"] != nil {
			//log.Debugf("LastEvaluatedKey: %+v", result.LastEvaluatedKey["signature_id"])
			lastEvaluatedKey = *results.LastEvaluatedKey["event_id"].S
			scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"event_id": {
					S: aws.String(lastEvaluatedKey),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}
	}
	fmt.Printf("total Items: %d", total)
	return nil
}

func (c Config) InsertCompoundAttribute(ctx context.Context, event EventModel) error {
	updateExpression := expression.Set(expression.Name(newAttr), expression.Value(fmt.Sprintf("%s#%s", event.EventCompanySFID, event.EventCLAGroupID)))
	expr, err := expression.NewBuilder().WithUpdate(updateExpression).Build()
	if err != nil {
		log.Error("Found error on NewBuilder", err)
		return err
	}

	_, err = c.dynamoDBClient.UpdateItemWithContext(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"event_id": {
				S: aws.String(event.EventID),
			},
		},
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	if err != nil {
		log.Error("Found error on UpdateItemWithContext", err)
		return err
	}
	log.Debugf("Updates event - %s", event.EventID)
	return nil
}

func NewRepository(awsSession *session.Session, stage string) EventInterface {
	return &Config{
		dynamoDBClient: dynamodb.New(awsSession),
		stage:          stage,
		tableName:      fmt.Sprintf(tableName, stage),
	}
}

func init() {
	stage = os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("stage not set")
	}
	log.Infof("STAGE set to %s\n", stage)
	events = NewRepository(awsSession, stage)
}

func main() {
	log.Debugf("Getting events that should be updated...")

	context := context.Background()
	err := events.FetchAndUpdateDocuments(context)
	if err != nil {
		panic(err)
	}
}
