package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	STAGE   string
	PROFILE string
	REGION  string
)

func init() {
	REGION = os.Getenv("AWS_REGION")
	if REGION == "" {
		REGION = "us-east-1"
	}
	STAGE = os.Getenv("STAGE")
	if STAGE == "" {
		STAGE = "dev"
	}
	PROFILE = os.Getenv("AWS_PROFILE")
	if PROFILE == "" {
		PROFILE = "lfproduct-" + STAGE
	}
}

func putTestItem(tableName, keyName string, keyValue interface{}, keyType string, extraFields map[string]interface{}, dbg bool) {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(REGION),
		config.WithSharedConfigProfile(PROFILE),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	client := dynamodb.NewFromConfig(cfg)

	item := make(map[string]types.AttributeValue)

	switch keyType {
	case "S":
		item[keyName] = &types.AttributeValueMemberS{Value: fmt.Sprint(keyValue)}
	case "N":
		item[keyName] = &types.AttributeValueMemberN{Value: fmt.Sprint(keyValue)}
	default:
		log.Fatalf("Unsupported key type: %s", keyType)
	}

	for k, v := range extraFields {
		switch val := v.(type) {
		case string:
			item[k] = &types.AttributeValueMemberS{Value: val}
		case int, int64, float64:
			item[k] = &types.AttributeValueMemberN{Value: fmt.Sprint(val)}
		case bool:
			item[k] = &types.AttributeValueMemberBOOL{Value: val}
		case []string:
			item[k] = &types.AttributeValueMemberSS{Value: val}
		case []interface{}:
			Debugf("Skipping field %s: generic list not supported directly", k)
		default:
			Debugf("Unsupported type for field %s: %T", k, v)
		}
	}

	tName := "cla-" + STAGE + "-" + tableName
	_, err = client.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(tName),
		Item:      item,
	})
	if err != nil {
		log.Fatalf("PutItem error: %v", err)
	}
	Debugf("created entry in %s: %s=%s, %+v\n", tName, keyName, keyValue, extraFields)
}

func deleteTestItem(tableName, keyName string, keyValue interface{}, keyType string, dbg bool) {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(REGION),
		config.WithSharedConfigProfile(PROFILE),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	client := dynamodb.NewFromConfig(cfg)

	var key types.AttributeValue

	switch keyType {
	case "S":
		key = &types.AttributeValueMemberS{Value: fmt.Sprint(keyValue)}
	case "N":
		key = &types.AttributeValueMemberN{Value: fmt.Sprint(keyValue)}
	case "BOOL":
		b, ok := keyValue.(bool)
		if !ok {
			log.Fatalf("Key value must be boolean for BOOL type")
		}
		key = &types.AttributeValueMemberBOOL{Value: b}
	default:
		log.Fatalf("Unsupported key type: %s", keyType)
	}

	tName := "cla-" + STAGE + "-" + tableName
	_, err = client.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
		TableName: aws.String(tName),
		Key: map[string]types.AttributeValue{
			keyName: key,
		},
	})
	if err != nil {
		log.Fatalf("DeleteItem error: %v", err)
	}
	Debugf("deleted entry in %s: %s=%s\n", tName, keyName, keyValue)
}

func getAllPrimaryKeys(tableName, keyName, keyType string) []interface{} {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(REGION),
		config.WithSharedConfigProfile(PROFILE),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	client := dynamodb.NewFromConfig(cfg)

	tName := "cla-" + STAGE + "-" + tableName
	Debugf("getting all keys form %s\n", tName)
	var results []interface{}
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName:            aws.String(tName),
			ProjectionExpression: aws.String("#k"),
			ExpressionAttributeNames: map[string]string{
				"#k": keyName,
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		output, err := client.Scan(context.TODO(), input)
		if err != nil {
			log.Fatalf("Scan error on table %s: %v", tName, err)
		}

		for _, item := range output.Items {
			attr, ok := item[keyName]
			if !ok {
				Debugf("Key %s not found in item: %+v", keyName, item)
				continue
			}

			switch keyType {
			case "S":
				if v, ok := attr.(*types.AttributeValueMemberS); ok {
					results = append(results, v.Value)
				}
			case "N":
				if v, ok := attr.(*types.AttributeValueMemberN); ok {
					results = append(results, v.Value)
				}
			case "BOOL":
				if v, ok := attr.(*types.AttributeValueMemberBOOL); ok {
					results = append(results, v.Value)
				}
			default:
				log.Fatalf("Unsupported key type: %s", keyType)
			}
		}

		if output.LastEvaluatedKey == nil || len(output.LastEvaluatedKey) == 0 {
			break
		}
		lastEvaluatedKey = output.LastEvaluatedKey
	}

	Debugf("got keys: %+v\n", results)
	return results
}
