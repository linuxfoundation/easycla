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
	"github.com/go-openapi/strfmt"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// Repository interface defines the data access methods for the My CLAs module
type Repository interface {
	GetUserCLASignatures(ctx context.Context, userID string) ([]*signatures.ItemSignature, error)
	GetUsersBySecondaryEmails(ctx context.Context, emails []string) ([]*v1Models.User, error)
}

type repository struct {
	dynamoDBClient     *dynamodb.DynamoDB
	signatureTableName string
	usersTableName     string
}

// NewRepository creates a new instance of the My CLAs repository
func NewRepository(awsSession *session.Session, stage string) Repository {
	return repository{
		dynamoDBClient:     dynamodb.New(awsSession),
		signatureTableName: fmt.Sprintf("cla-%s-signatures", stage),
		usersTableName:     fmt.Sprintf("cla-%s-users", stage),
	}
}

// GetUserCLASignatures returns all ICLA and ECLA signature records referencing the given EasyCLA user ID
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
		queryResults, queryErr := repo.dynamoDBClient.QueryWithContext(ctx, queryInput)
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

// GetUsersBySecondaryEmails returns the user records whose additional-emails set
// (user_emails) contains any of the given emails - a single table scan for all values,
// as the set attribute cannot be indexed
func (repo repository) GetUsersBySecondaryEmails(ctx context.Context, emails []string) ([]*v1Models.User, error) {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.repository.GetUsersBySecondaryEmails",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"emails":         emails,
	}

	if len(emails) == 0 {
		return nil, nil
	}

	filter := expression.Name("user_emails").Contains(emails[0])
	for _, email := range emails[1:] {
		filter = filter.Or(expression.Name("user_emails").Contains(email))
	}

	expr, err := expression.NewBuilder().WithFilter(filter).Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error building expression for secondary emails scan")
		return nil, err
	}

	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.usersTableName),
	}

	var dbUsers []users.DBUser
	for {
		scanResults, scanErr := repo.dynamoDBClient.ScanWithContext(ctx, scanInput)
		if scanErr != nil {
			log.WithFields(f).WithError(scanErr).Warn("error scanning users by secondary emails")
			return nil, scanErr
		}

		var page []users.DBUser
		if unmarshalErr := dynamodbattribute.UnmarshalListOfMaps(scanResults.Items, &page); unmarshalErr != nil {
			log.WithFields(f).WithError(unmarshalErr).Warn("error unmarshalling users from secondary emails scan")
			return nil, unmarshalErr
		}
		dbUsers = append(dbUsers, page...)

		if len(scanResults.LastEvaluatedKey) == 0 {
			break
		}
		scanInput.ExclusiveStartKey = scanResults.LastEvaluatedKey
	}

	userModels := make([]*v1Models.User, 0, len(dbUsers))
	for _, dbUser := range dbUsers {
		userModels = append(userModels, &v1Models.User{
			UserID:         dbUser.UserID,
			LfUsername:     dbUser.LFUsername,
			LfEmail:        strfmt.Email(dbUser.LFEmail),
			Emails:         dbUser.UserEmails,
			GithubID:       dbUser.UserGithubID,
			GithubUsername: dbUser.UserGithubUsername,
			GitlabID:       dbUser.UserGitlabID,
			GitlabUsername: dbUser.UserGitlabUsername,
		})
	}

	return userModels, nil
}
