// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// Repository interface defines the data access methods for the My CLAs module
type Repository interface {
	GetUserCLASignatures(ctx context.Context, userID string) ([]*signatures.ItemSignature, error)
}

type repository struct {
	stage              string
	dynamoDBClient     *dynamodb.DynamoDB
	signatureTableName string
}

// NewRepository creates a new instance of the My CLAs repository
func NewRepository(awsSession *session.Session, stage string) Repository {
	return repository{
		stage:              stage,
		dynamoDBClient:     dynamodb.New(awsSession),
		signatureTableName: fmt.Sprintf("cla-%s-signatures", stage),
	}
}

// GetUserCLASignatures returns all ICLA and ECLA signature records referencing the
// given EasyCLA user ID. Unlike the v1 signatures repository GetUserSignatures, the
// query does not exclude records with signature_user_ccla_company_id set (ECLAs).
func (repo repository) GetUserCLASignatures(ctx context.Context, userID string) ([]*signatures.ItemSignature, error) {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.repository.GetUserCLASignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"userID":         userID,
	}

	condition := expression.Key("signature_reference_id").Equal(expression.Value(userID))
	// ICLAs and DocuSign-era ECLAs are stored with signature_type=cla; ECLAs auto-created
	// from approval-list changes are stored with signature_type=ecla - accept both.
	filter := expression.Name("signature_type").In(expression.Value(utils.SignatureTypeCLA), expression.Value(utils.ClaTypeECLA)).
		And(expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser)))

	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter).Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error building expression for user CLA signatures query")
		return nil, err
	}

	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String(signatures.SignatureReferenceIndex),
	}

	var results []*signatures.ItemSignature
	for {
		queryResults, queryErr := repo.dynamoDBClient.Query(queryInput)
		if queryErr != nil {
			log.WithFields(f).WithError(queryErr).Warn("error retrieving user CLA signatures")
			return nil, queryErr
		}

		var page []*signatures.ItemSignature
		if unmarshalErr := dynamodbattribute.UnmarshalListOfMaps(queryResults.Items, &page); unmarshalErr != nil {
			log.WithFields(f).WithError(unmarshalErr).Warn("error unmarshalling user CLA signatures")
			return nil, unmarshalErr
		}
		results = append(results, page...)

		if len(queryResults.LastEvaluatedKey) == 0 {
			break
		}
		queryInput.ExclusiveStartKey = queryResults.LastEvaluatedKey
	}

	return results, nil
}
