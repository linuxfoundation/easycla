// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// RepositoryDynamo data model
type RepositoryDynamo struct {
	Stage          string
	DynamoDBClient *dynamodb.DynamoDB
}

// RepositoryService interface methods
type RepositoryService interface {
	GetUserAndProfilesByLFID(lfidUsername string) (CLAUser, error)
	GetUserProjectIDs(userID string) ([]string, error)
	GetClaManagerCorporateClaIDs(userID string) ([]string, error)
	GetUserCompanyIDs(userID string) ([]string, error)
	GetUser(userID string) (User, error)
	SetCompanyID(userID, companyID string) (*User, error)
}

// User data model
type User struct {
	UserID             string   `json:"user_id"`
	LFEmail            string   `json:"lf_email"`
	LFUsername         string   `json:"lf_username"`
	DateCreated        string   `json:"date_created"`
	DateModified       string   `json:"date_modified"`
	UserName           string   `json:"user_name"`
	Version            string   `json:"version"`
	UserEmails         []string `json:"user_emails"`
	UserGithubID       string   `json:"user_github_id"`
	UserCompanyID      string   `json:"user_company_id"`
	UserGithubUsername string   `json:"user_github_username"`
}

// NewDynamoRepository creates a new dynamo repository model
func NewDynamoRepository(awsSession *session.Session, stage string) RepositoryService {
	return RepositoryDynamo{
		Stage:          stage,
		DynamoDBClient: dynamodb.New(awsSession),
	}
}

// GetUserAndProfilesByLFID returns the user profile by LFID
func (repo RepositoryDynamo) GetUserAndProfilesByLFID(lfidUsername string) (CLAUser, error) {
	tableName := fmt.Sprintf("cla-%s-users", repo.Stage)

	input := &dynamodb.QueryInput{
		KeyConditions: map[string]*dynamodb.Condition{
			"lf_username": {
				ComparisonOperator: aws.String("EQ"),
				AttributeValueList: []*dynamodb.AttributeValue{
					{
						S: aws.String(lfidUsername),
					},
				},
			},
		},
		TableName: aws.String(tableName),
		IndexName: aws.String("lf-username-index"),
	}
	result, err := repo.DynamoDBClient.Query(input)

	if err != nil {
		log.Warnf("problem querying the database for user: %s , error: %+v", lfidUsername, err)
		return CLAUser{
			LFUsername: lfidUsername,
		}, err
	}

	users := []User{}
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &users)
	if err != nil {
		log.Warnf("problem unmarshalling the database results, error: %+v", err)
		return CLAUser{
			LFUsername: lfidUsername,
		}, err
	}

	if len(users) < 1 {
		log.Debugf("Get User And Profiles By LFID - user not found given LFID: %s", lfidUsername)
		return CLAUser{
			LFUsername: lfidUsername,
		}, errors.New("user not found")
	}

	claUser := CLAUser{
		UserID:     users[0].UserID,
		Name:       users[0].UserName,
		LFEmail:    users[0].LFEmail,
		LFUsername: users[0].LFUsername,
	}

	return claUser, nil
}

// GetUserProjectIDs returns a list of user's projects when provided the user id
func (repo RepositoryDynamo) GetUserProjectIDs(LfUsername string) ([]string, error) {
	tableName := fmt.Sprintf("cla-%s-user-permissions", repo.Stage)
	result, err := repo.DynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"username": {
				S: aws.String(LfUsername),
			},
		},
	})

	if err != nil {
		log.Warnf("error fetching user project IDs: error: %v", err)
		return []string{}, err
	}

	projects, ok := result.Item["projects"]
	if !ok {
		projects = &dynamodb.AttributeValue{}
	}

	//take off pointer from []*string
	return aws.StringValueSlice(projects.SS), nil
}

// GetClaManagerCorporateClaIDs returns a list of corporate CLAs when provided the user ID
func (repo RepositoryDynamo) GetClaManagerCorporateClaIDs(userID string) ([]string, error) {
	return []string{}, nil
}

// GetUserCompanyIDs returns a list of company IDs associated with the specified user
func (repo RepositoryDynamo) GetUserCompanyIDs(userID string) ([]string, error) {
	return []string{}, nil
}

// GetUser returns the user model when provided the user ID
func (repo RepositoryDynamo) GetUser(userID string) (User, error) {
	tableName := fmt.Sprintf("cla-%s-users", repo.Stage)
	userAV, err := repo.DynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"user_id": {
				S: aws.String(userID),
			},
		},
	})
	if err != nil {
		log.Warnf("Error fetching user: %s, error: %v", userID, err)
		return User{}, err
	}

	user := User{}
	err = dynamodbattribute.UnmarshalMap(userAV.Item, &user)
	if err != nil {
		log.Warnf("Error processing user value for: %s, error: %v", userID, err)
		return User{}, err
	}

	return user, err
}

// SetCompanyID sets the specified user's company id
func (repo RepositoryDynamo) SetCompanyID(userID, companyID string) (*User, error) {
	tableName := fmt.Sprintf("cla-%s-users", repo.Stage)

	_, now := utils.CurrentTime()

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#C": aws.String("company_id"),
			"#M": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":c": {
				S: aws.String(companyID),
			},
			":m": {
				S: aws.String(now),
			},
		},
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"user_id": {
				S: aws.String(userID),
			},
		},
		UpdateExpression: aws.String("SET #C = :c, #M = :m"),
	}

	_, err := repo.DynamoDBClient.UpdateItem(input)
	if err != nil {
		log.Warnf("Error updating User: %s with Company ID: %s, error: %v",
			userID, companyID, err)
		return nil, err
	}

	user, getErr := repo.GetUser(userID)
	if getErr != nil {
		log.Warnf("Error fetching user record by ID: %s, error: %v",
			userID, getErr)
		return nil, err
	}

	return &user, nil
}
