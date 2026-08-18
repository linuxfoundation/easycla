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
	GetUsersByLFUsername(ctx context.Context, lfUsername string) ([]*v1Models.User, error)
	GetUsersByPrimaryEmail(ctx context.Context, email string) ([]*v1Models.User, error)
	GetUsersByGithubID(ctx context.Context, githubID int64) ([]*v1Models.User, error)
	GetUsersByGithubUsername(ctx context.Context, githubUsername string) ([]*v1Models.User, error)
	GetUsersByGitlabID(ctx context.Context, gitlabID int64) ([]*v1Models.User, error)
	GetUsersByGitlabUsername(ctx context.Context, gitlabUsername string) ([]*v1Models.User, error)
	GetUsersBySecondaryEmails(ctx context.Context, emails []string) ([]*v1Models.User, error)
	GetUserByIDConsistent(ctx context.Context, userID string) (*v1Models.User, error)
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

// GetUsersByLFUsername returns all user records matching the given LF username
func (repo repository) GetUsersByLFUsername(ctx context.Context, lfUsername string) ([]*v1Models.User, error) {
	return repo.queryUsers(ctx, "lf-username-index", expression.Key("lf_username").Equal(expression.Value(lfUsername)))
}

// GetUsersByPrimaryEmail returns all user records matching the given primary (lf_email) address
func (repo repository) GetUsersByPrimaryEmail(ctx context.Context, email string) ([]*v1Models.User, error) {
	return repo.queryUsers(ctx, "lf-email-index", expression.Key("lf_email").Equal(expression.Value(email)))
}

// GetUsersByGithubID returns all user records matching the given GitHub numeric ID
func (repo repository) GetUsersByGithubID(ctx context.Context, githubID int64) ([]*v1Models.User, error) {
	return repo.queryUsers(ctx, "github-id-index", expression.Key("user_github_id").Equal(expression.Value(githubID)))
}

// GetUsersByGithubUsername returns all user records matching the given GitHub username
func (repo repository) GetUsersByGithubUsername(ctx context.Context, githubUsername string) ([]*v1Models.User, error) {
	return repo.queryUsers(ctx, "github-username-index", expression.Key("user_github_username").Equal(expression.Value(githubUsername)))
}

// GetUsersByGitlabID returns all user records matching the given GitLab numeric ID
func (repo repository) GetUsersByGitlabID(ctx context.Context, gitlabID int64) ([]*v1Models.User, error) {
	return repo.queryUsers(ctx, "gitlab-id-index", expression.Key("user_gitlab_id").Equal(expression.Value(gitlabID)))
}

// GetUsersByGitlabUsername returns all user records matching the given GitLab username
func (repo repository) GetUsersByGitlabUsername(ctx context.Context, gitlabUsername string) ([]*v1Models.User, error) {
	return repo.queryUsers(ctx, "gitlab-username-index", expression.Key("user_gitlab_username").Equal(expression.Value(gitlabUsername)))
}

// GetUsersBySecondaryEmails returns the user records whose additional-emails set contains
// any of the given emails - a single table scan for all values (the set attribute cannot
// be indexed)
func (repo repository) GetUsersBySecondaryEmails(ctx context.Context, emails []string) ([]*v1Models.User, error) {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.repository.GetUsersBySecondaryEmails",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"emailCount":     len(emails),
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

	return toUserModels(dbUsers), nil
}

// GetUserByIDConsistent reads one user record by its primary key with a strongly
// consistent read.
//
// Every other lookup here goes through a global secondary index, which is only ever
// eventually consistent. That is fine for reading, and wrong for confirming a write: a
// stale read after a successful write looks identical to the write having recorded the
// wrong thing, and the caller cannot tell the two apart. Confirming on the base table with
// ConsistentRead set removes the ambiguity, so a mismatch means a real mismatch.
func (repo repository) GetUserByIDConsistent(ctx context.Context, userID string) (*v1Models.User, error) {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.repository.GetUserByIDConsistent",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"userID":         userID,
	}

	expr, err := expression.NewBuilder().
		WithKeyCondition(expression.Key("user_id").Equal(expression.Value(userID))).
		Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error building expression for consistent user read")
		return nil, err
	}

	queryResults, err := repo.dynamoDBClient.QueryWithContext(ctx, &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		TableName:                 aws.String(repo.usersTableName),
		ConsistentRead:            aws.Bool(true),
	})
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error reading user record by ID")
		return nil, err
	}

	var dbUsers []users.DBUser
	if unmarshalErr := dynamodbattribute.UnmarshalListOfMaps(queryResults.Items, &dbUsers); unmarshalErr != nil {
		log.WithFields(f).WithError(unmarshalErr).Warn("error unmarshalling user record")
		return nil, unmarshalErr
	}

	if len(dbUsers) == 0 {
		return nil, nil
	}

	return toUserModels(dbUsers)[0], nil
}

func (repo repository) queryUsers(ctx context.Context, indexName string, condition expression.KeyConditionBuilder) ([]*v1Models.User, error) {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.repository.queryUsers",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"indexName":      indexName,
	}

	expr, err := expression.NewBuilder().WithKeyCondition(condition).Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error building expression for users query")
		return nil, err
	}

	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		TableName:                 aws.String(repo.usersTableName),
		IndexName:                 aws.String(indexName),
	}

	var dbUsers []users.DBUser
	for {
		queryResults, queryErr := repo.dynamoDBClient.QueryWithContext(ctx, queryInput)
		if queryErr != nil {
			log.WithFields(f).WithError(queryErr).Warn("error querying users")
			return nil, queryErr
		}

		var page []users.DBUser
		if unmarshalErr := dynamodbattribute.UnmarshalListOfMaps(queryResults.Items, &page); unmarshalErr != nil {
			log.WithFields(f).WithError(unmarshalErr).Warn("error unmarshalling users")
			return nil, unmarshalErr
		}
		dbUsers = append(dbUsers, page...)

		if len(queryResults.LastEvaluatedKey) == 0 {
			break
		}
		queryInput.ExclusiveStartKey = queryResults.LastEvaluatedKey
	}

	return toUserModels(dbUsers), nil
}

func toUserModels(dbUsers []users.DBUser) []*v1Models.User {
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
	return userModels
}
