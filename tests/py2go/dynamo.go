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
			log.Printf("Skipping field %s: generic list not supported directly", k)
		default:
			log.Printf("Unsupported type for field %s: %T", k, v)
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
	if dbg {
		fmt.Printf("created entry in %s: %s=%s, %+v\n", tName, keyName, keyValue, extraFields)
	}
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
	if dbg {
		fmt.Printf("deleted entry in %s: %s=%s\n", tName, keyName, keyValue)
	}
}
