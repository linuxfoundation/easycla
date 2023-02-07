// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_organizations

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/v2/common"

	"github.com/gofrs/uuid"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	v2Models "github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// indexes
const (
	// GitLabOrgOrganizationSFIDIndex the index for the Project Parent SFID
	GitLabOrgOrganizationSFIDIndex = "gitlab-org-sfid-index"
	// GitLabOrgProjectSFIDIndex the index for the Project SFID
	GitLabOrgProjectSFIDIndex = "gitlab-project-sfid-index"
	// GitLabOrgLowerNameIndex the index for the group/org name in lower case
	GitLabOrgLowerNameIndex = "gitlab-organization-name-lower-search-index"
	// GitLabExternalIDIndex the index for the external ID
	GitLabExternalIDIndex = "gitlab-external-group-id-index"
	// GitLabFullPathIndex the index for the full path
	GitLabFullPathIndex = "gitlab-full-path-index"
	// GitlabOrgURLIndex the index for the org url
	GitlabOrgURLIndex = "gitlab-org-url-index"
)

// RepositoryInterface is interface for gitlab org data model
type RepositoryInterface interface {
	AddGitLabOrganization(ctx context.Context, input *common.GitLabAddOrganization, enabled bool) (*v2Models.GitlabOrganization, error)
	GetGitLabOrganizations(ctx context.Context) (*v2Models.GitlabOrganizations, error)
	GetGitLabOrganizationsEnabled(ctx context.Context) (*v2Models.GitlabOrganizations, error)
	GetGitLabOrganizationsEnabledWithAutoEnabled(ctx context.Context) (*v2Models.GitlabOrganizations, error)
	GetGitLabOrganizationsByProjectSFID(ctx context.Context, projectSFID string) (*v2Models.GitlabOrganizations, error)
	GetGitLabOrganization(ctx context.Context, gitlabOrganizationID string) (*common.GitLabOrganization, error)
	GetGitLabOrganizationByName(ctx context.Context, gitLabOrganizationName string) (*common.GitLabOrganization, error)
	GetGitLabOrganizationByExternalID(ctx context.Context, gitLabGroupID int64) (*common.GitLabOrganization, error)
	GetGitLabOrganizationByFullPath(ctx context.Context, groupFullPath string) (*common.GitLabOrganization, error)
	GetGitLabOrganizationByURL(ctx context.Context, url string) (*common.GitLabOrganization, error)
	UpdateGitLabOrganizationAuth(ctx context.Context, organizationID string, gitLabGroupID, authExpiryTime int, authInfo, groupName, groupFullPath, organizationURL string) error
	UpdateGitLabOrganization(ctx context.Context, input *common.GitLabAddOrganization, enabled bool) error
	DeleteGitLabOrganizationByFullPath(ctx context.Context, projectSFID, gitlabOrgFullPath string) error
}

// Repository object/struct
type Repository struct {
	stage              string
	dynamoDBClient     *dynamodb.DynamoDB
	gitlabOrgTableName string
}

// NewRepository creates a new instance of the gitlabOrganizations repository
func NewRepository(awsSession *session.Session, stage string) RepositoryInterface {
	return &Repository{
		stage:              stage,
		dynamoDBClient:     dynamodb.New(awsSession),
		gitlabOrgTableName: fmt.Sprintf("cla-%s-gitlab-orgs", stage),
	}
}

// AddGitLabOrganization adds the specified values to the GitLab Group/Org table
func (repo *Repository) AddGitLabOrganization(ctx context.Context, input *common.GitLabAddOrganization, enabled bool) (*v2Models.GitlabOrganization, error) {
	f := logrus.Fields{
		"functionName":            "v2.gitlab_organizations.repository.AddGitLabOrganization",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"parentProjectSFID":       input.ParentProjectSFID,
		"projectSFID":             input.ProjectSFID,
		"groupID":                 input.ExternalGroupID,
		"organizationName":        input.OrganizationName,
		"groupFullPath":           input.OrganizationFullPath,
		"autoEnabled":             input.AutoEnabled,
		"autoEnabledClaGroupID":   input.AutoEnabledClaGroupID,
		"branchProtectionEnabled": input.BranchProtectionEnabled,
		"enabled":                 enabled,
	}

	var existingRecord *common.GitLabOrganization
	var getErr error
	if input.ExternalGroupID != 0 {
		log.WithFields(f).Debugf("checking to see if we have an existing GitLab organization with ID: %d", input.ExternalGroupID)
		// First, let's check to see if we have an existing gitlab organization with the same name
		existingRecord, getErr = repo.GetGitLabOrganizationByExternalID(ctx, input.ExternalGroupID)
		if getErr != nil {
			log.WithFields(f).WithError(getErr).Debugf("unable to locate existing GitLab group by ID: %d - ok to create a new record", input.ExternalGroupID)
		}
	} else if input.OrganizationFullPath != "" {
		log.WithFields(f).Debugf("checking to see if we have an existing GitLab group full path with value: %s", input.OrganizationFullPath)
		// First, let's check to see if we have an existing gitlab organization with the same name
		existingRecord, getErr = repo.GetGitLabOrganizationByFullPath(ctx, input.OrganizationFullPath)
		if getErr != nil {
			log.WithFields(f).WithError(getErr).Debugf("unable to locate existing GitLab group by full path: %s - ok to create a new record", input.OrganizationFullPath)
		}
	}

	if existingRecord != nil {
		log.WithFields(f).Debugf("An existing GitLab organization with ID %d or full path: %s exists in our database", input.ExternalGroupID, input.OrganizationFullPath)
		// If everything matches...
		if input.ProjectSFID == existingRecord.ProjectSFID {
			log.WithFields(f).Debug("existing GitLab organization with same SFID - should be able to update it")
			updateErr := repo.UpdateGitLabOrganization(ctx, input, enabled)
			if updateErr != nil {
				return nil, updateErr
			}

			if input.ExternalGroupID > 0 {
				// Return the updated record
				if gitlabOrg, err := repo.GetGitLabOrganizationByExternalID(ctx, input.ExternalGroupID); err != nil {
					return nil, err
				} else {
					return common.ToModel(gitlabOrg), nil
				}
			} else if input.OrganizationFullPath != "" {
				// Return the updated record
				if gitlabOrg, err := repo.GetGitLabOrganizationByFullPath(ctx, input.OrganizationFullPath); err != nil {
					return nil, err
				} else {
					return common.ToModel(gitlabOrg), nil
				}
			}
		}

		msg := fmt.Sprintf("record already exists - existing GitLab group with a different project SFID - won't be able to update it")
		log.WithFields(f).Debug(msg)
		return nil, errors.New(msg)
	}

	// No existing records - create one
	_, currentTime := utils.CurrentTime()
	organizationID, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Unable to generate a UUID for gitlab org, error: %v2Models", err)
		return nil, err
	}

	authStateNonce, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Unable to generate a auth nonce UUID for gitlab org, error: %v2Models", err)
		return nil, err
	}

	gitlabOrg := &common.GitLabOrganization{
		OrganizationID:          organizationID.String(),
		DateCreated:             currentTime,
		DateModified:            currentTime,
		OrganizationName:        input.OrganizationName,
		OrganizationNameLower:   strings.ToLower(input.OrganizationName),
		OrganizationURL:         input.OrganizationURL,
		OrganizationFullPath:    input.OrganizationFullPath,
		ExternalGroupID:         input.ExternalGroupIDAsInt(),
		OrganizationSFID:        input.ParentProjectSFID,
		ProjectSFID:             input.ProjectSFID,
		Enabled:                 enabled,
		AutoEnabled:             input.AutoEnabled,
		AutoEnabledClaGroupID:   input.AutoEnabledClaGroupID,
		BranchProtectionEnabled: input.BranchProtectionEnabled,
		AuthState:               authStateNonce.String(),
		Version:                 "v1",
	}

	log.WithFields(f).Debug("encoding GitLab organization record for adding to the database...")
	av, err := dynamodbattribute.MarshalMap(gitlabOrg)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to marshall request for query")
		return nil, err
	}

	log.WithFields(f).Debug("adding gitlab organization record to the database...")
	_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:                av,
		TableName:           aws.String(repo.gitlabOrgTableName),
		ConditionExpression: aws.String("attribute_not_exists(organization_name)"),
	})
	if err != nil {
		if aErr, ok := err.(awserr.Error); ok {
			switch aErr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				log.WithFields(f).WithError(err).Warn("gitlab group/organization already exists")
				return nil, fmt.Errorf("gitlab group/organization already exists")
			}
		}
		log.WithFields(f).WithError(err).Warn("cannot put gitlab group/organization in dynamodb")
		return nil, err
	}

	return common.ToModel(gitlabOrg), nil
}

// GetGitLabOrganizations returns the complete list of GitLab groups/organizations
func (repo *Repository) GetGitLabOrganizations(ctx context.Context) (*v2Models.GitlabOrganizations, error) {
	// No filter, return all
	return repo.getScanResults(ctx, nil)
}

// GetGitLabOrganizationsEnabled returns the list of GitLab groups/organizations that are enabled
func (repo *Repository) GetGitLabOrganizationsEnabled(ctx context.Context) (*v2Models.GitlabOrganizations, error) {
	// Build the scan/query expression
	filter := expression.Name(GitLabOrganizationsEnabledColumn).Equal(expression.Value(true))
	return repo.getScanResults(ctx, &filter)
}

// GetGitLabOrganizationsEnabledWithAutoEnabled returns the list of GitLab groups/organizations that are enabled with the auto enabled flag set to true
func (repo *Repository) GetGitLabOrganizationsEnabledWithAutoEnabled(ctx context.Context) (*v2Models.GitlabOrganizations, error) {
	// Build the scan/query expression
	filter := expression.Name(GitLabOrganizationsEnabledColumn).Equal(expression.Value(true)).
		And(expression.Name(GitLabOrganizationsAutoEnabledColumn).Equal(expression.Value(true)))
	return repo.getScanResults(ctx, &filter)
}

// GetGitLabOrganizationsByProjectSFID get GitLab organizations based on the project SFID or parent project SFID
func (repo *Repository) GetGitLabOrganizationsByProjectSFID(ctx context.Context, projectSFID string) (*v2Models.GitlabOrganizations, error) {
	f := logrus.Fields{
		"functionName":   "v2.gitlab_organizations.repository.GetGitLabOrganizationsByProjectSFID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
	}

	condition := expression.Key(GitLabOrganizationsProjectSFIDColumn).Equal(expression.Value(projectSFID))
	filter := expression.Name(GitLabOrganizationsEnabledColumn).Equal(expression.Value(true))
	response, err := repo.getOrganizationsWithConditionFilter(ctx, condition, filter, GitLabOrgProjectSFIDIndex)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Error getting GitLab organizations by project SFID, error: %v2Models", err)
		return nil, err
	}

	return response, nil
}

// GetGitLabOrganizationByName get GitLab organization by name
func (repo *Repository) GetGitLabOrganizationByName(ctx context.Context, gitLabOrganizationName string) (*common.GitLabOrganization, error) {
	f := logrus.Fields{
		"functionName":           "v1.gitlab_organizations.repository.GetGitLabOrganizationByName",
		utils.XREQUESTID:         ctx.Value(utils.XREQUESTID),
		"gitLabOrganizationName": gitLabOrganizationName,
	}

	gitLabOrganizationName = strings.ToLower(gitLabOrganizationName)

	condition := expression.Key(GitLabOrganizationsOrganizationNameLowerColumn).Equal(expression.Value(strings.ToLower(gitLabOrganizationName)))
	builder := expression.NewBuilder().WithKeyCondition(condition)
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}
	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.gitlabOrgTableName),
		IndexName:                 aws.String(GitLabOrgLowerNameIndex),
	}

	log.WithFields(f).Debugf("querying for GitLab organization by name using organization_name_lower=%s...", strings.ToLower(gitLabOrganizationName))
	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error retrieving gitlab_organizations using gitLabOrganizationName = %s", gitLabOrganizationName)
		return nil, err
	}
	if len(results.Items) == 0 {
		log.WithFields(f).Debug("Unable to find GitLab organization by name - no results")
		return nil, nil
	}

	var resultOutput []*common.GitLabOrganization
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &resultOutput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem decoding database results, error: %+v", err)
		return nil, err
	}

	return resultOutput[0], nil
}

// GetGitLabOrganizationByExternalID returns the GitLab Group/Org based on the external GitLab Group ID value
func (repo *Repository) GetGitLabOrganizationByExternalID(ctx context.Context, gitLabGroupID int64) (*common.GitLabOrganization, error) {
	f := logrus.Fields{
		"functionName":   "v1.gitlab_organizations.repository.GetGitLabOrganizationByExternalID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"gitLabGroupID":  gitLabGroupID,
	}

	condition := expression.Key(GitLabOrganizationsExternalGitLabGroupIDColumn).Equal(expression.Value(gitLabGroupID))
	builder := expression.NewBuilder().WithKeyCondition(condition)
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.gitlabOrgTableName),
		IndexName:                 aws.String(GitLabExternalIDIndex),
	}

	log.WithFields(f).Debugf("querying for GitLab organization by external group ID: %d...", gitLabGroupID)
	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error retrieving gitlab_organizations using external ID = %d", gitLabGroupID)
		return nil, err
	}
	if len(results.Items) == 0 {
		log.WithFields(f).Debugf("Unable to find GitLab organization by group ID: %d - no results", gitLabGroupID)
		return nil, nil
	}

	var resultOutput []*common.GitLabOrganization
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &resultOutput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem decoding database results, error: %+v", err)
		return nil, err
	}

	return resultOutput[0], nil
}

// GetGitLabOrganizationByURL loads the organization based on the url
func (repo *Repository) GetGitLabOrganizationByURL(ctx context.Context, url string) (*common.GitLabOrganization, error) {
	f := logrus.Fields{
		"functionName":   "v1.gitlab_organizations.repository. GetGitLabOrganizationByURL",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"URL":            url,
	}

	condition := expression.Key(GitLabOrganizationsOrganizationURLColumn).Equal(expression.Value(url))
	builder := expression.NewBuilder().WithKeyCondition(condition)
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.gitlabOrgTableName),
		IndexName:                 aws.String(GitlabOrgURLIndex),
	}

	log.WithFields(f).Debugf("querying for GitLab group by url: %s...", url)
	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error retrieving GitLab group by url: %s", url)
		return nil, err
	}
	if len(results.Items) == 0 {
		log.WithFields(f).Debugf("Unable to find GitLab group by url: %s - no results", url)
		return nil, nil
	}

	var resultOutput []*common.GitLabOrganization
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &resultOutput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem decoding database results, error: %+v", err)
		return nil, err
	}

	return resultOutput[0], nil
}

// GetGitLabOrganizationByFullPath loads the organization based on the full path value
func (repo *Repository) GetGitLabOrganizationByFullPath(ctx context.Context, groupFullPath string) (*common.GitLabOrganization, error) {
	f := logrus.Fields{
		"functionName":   "v1.gitlab_organizations.repository.GetGitLabOrganizationByFullPath",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"groupFullPath":  groupFullPath,
	}

	condition := expression.Key(GitLabOrganizationsOrganizationFullPathColumn).Equal(expression.Value(groupFullPath))
	builder := expression.NewBuilder().WithKeyCondition(condition)
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.gitlabOrgTableName),
		IndexName:                 aws.String(GitLabFullPathIndex),
	}

	log.WithFields(f).Debugf("querying for GitLab group by full path: %s...", groupFullPath)
	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error retrieving GitLab group by full path: %s", groupFullPath)
		return nil, err
	}
	if len(results.Items) == 0 {
		log.WithFields(f).Debugf("Unable to find GitLab group by full path: %s - no results", groupFullPath)
		return nil, nil
	}

	var resultOutput []*common.GitLabOrganization
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &resultOutput)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem decoding database results, error: %+v", err)
		return nil, err
	}

	return resultOutput[0], nil
}

// GetGitLabOrganization by organization name
func (repo *Repository) GetGitLabOrganization(ctx context.Context, gitLabOrganizationID string) (*common.GitLabOrganization, error) {
	f := logrus.Fields{
		"functionName":         "gitlab_organizations.repository.GetGitLabOrganization",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"gitLabOrganizationID": gitLabOrganizationID,
	}

	log.WithFields(f).Debugf("Querying for GitLab organization by ID: %s", gitLabOrganizationID)
	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			GitLabOrganizationsOrganizationIDColumn: {
				S: aws.String(gitLabOrganizationID),
			},
		},
		TableName: aws.String(repo.gitlabOrgTableName),
	})
	if err != nil {
		return nil, err
	}
	if len(result.Item) == 0 {
		log.WithFields(f).Debugf("Unable to find GitLab organization by ID: %s - no results", gitLabOrganizationID)
		return nil, nil
	}

	var org common.GitLabOrganization
	err = dynamodbattribute.UnmarshalMap(result.Item, &org)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling organization table data, error: %v2Models", err)
		return nil, err
	}
	return &org, nil
}

// UpdateGitLabOrganizationAuth updates the specified Gitlab organization oauth info
func (repo *Repository) UpdateGitLabOrganizationAuth(ctx context.Context, organizationID string, gitLabGroupID, authExpiryTime int, authInfo, groupName, groupFullPath, organizationURL string) error {
	f := logrus.Fields{
		"functionName":    "gitlab_organizations.repository.UpdateGitLabOrganizationAuth",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"organizationID":  organizationID,
		"groupName":       groupName,
		"groupFullPath":   groupFullPath,
		"organizationURL": organizationURL,
		"tableName":       repo.gitlabOrgTableName,
	}

	_, currentTime := utils.CurrentTime()
	gitlabOrg, lookupErr := repo.GetGitLabOrganization(ctx, organizationID)
	if lookupErr != nil || gitlabOrg == nil {
		log.WithFields(f).WithError(lookupErr).Warnf("error looking up Gitlab organization by id: %s, error: %+v", organizationID, lookupErr)
		return lookupErr
	}

	expressionAttributeNames := map[string]*string{
		"#A":  aws.String(GitLabOrganizationsAuthInfoColumn),
		"#U":  aws.String(GitLabOrganizationsOrganizationURLColumn),
		"#FP": aws.String(GitLabOrganizationsOrganizationFullPathColumn),
		"#M":  aws.String(GitLabOrganizationsDateModifiedColumn),
		"#P":  aws.String(GitLabOrganizationsExternalGitLabGroupIDColumn),
		"#E":  aws.String(GitLabOrganizationsAuthExpiryTimeColumn),
	}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{
		":a": {
			S: aws.String(authInfo),
		},
		":u": {
			S: aws.String(organizationURL),
		},
		":fp": {
			S: aws.String(groupFullPath),
		},
		":m": {
			S: aws.String(currentTime),
		},
		":p": {
			N: aws.String(strconv.Itoa(gitLabGroupID)),
		},
		":e": {
			N: aws.String(strconv.Itoa(authExpiryTime)),
		},
	}
	updateExpression := "SET #A = :a, #U = :u, #FP = :fp, #M = :m, #P = :p, #E = :e"

	if groupName != "" {
		expressionAttributeNames["#N"] = aws.String(GitLabOrganizationsOrganizationNameColumn)
		expressionAttributeValues[":n"] = &dynamodb.AttributeValue{S: aws.String(groupName)}
		updateExpression = fmt.Sprintf("%s, #N = :n ", updateExpression)

		expressionAttributeNames["#NL"] = aws.String(GitLabOrganizationsOrganizationNameLowerColumn)
		expressionAttributeValues[":nl"] = &dynamodb.AttributeValue{S: aws.String(strings.ToLower(groupName))}
		updateExpression = fmt.Sprintf("%s, #NL = :nl ", updateExpression)
	}

	input := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			GitLabOrganizationsOrganizationIDColumn: {
				S: aws.String(gitlabOrg.OrganizationID),
			},
		},
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		UpdateExpression:          &updateExpression,
		TableName:                 aws.String(repo.gitlabOrgTableName),
	}

	log.WithFields(f).Debug("updating gitlab organization record...")
	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).WithError(updateErr).Warnf("unable to update Gitlab organization record, error: %+v", updateErr)
		return updateErr
	}

	return nil
}

// UpdateGitLabOrganization updates the GitLab group based on the specified values
func (repo *Repository) UpdateGitLabOrganization(ctx context.Context, input *common.GitLabAddOrganization, enabled bool) error {
	f := logrus.Fields{
		"functionName":            "gitlab_organizations.repository.UpdateGitLabOrganization",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"projectSFID":             input.ProjectSFID,
		"groupID":                 input.ExternalGroupID,
		"groupFullPath":           input.OrganizationFullPath,
		"organizationName":        input.OrganizationName,
		"autoEnabled":             input.AutoEnabled,
		"autoEnabledClaGroupID":   input.AutoEnabledClaGroupID,
		"branchProtectionEnabled": input.BranchProtectionEnabled,
		"enabled":                 enabled,
		"tableName":               repo.gitlabOrgTableName,
	}

	var existingRecord *common.GitLabOrganization
	var getErr error
	if input.ExternalGroupID > 0 {
		log.WithFields(f).Debugf("checking to see if we have an existing GitLab organization with ID: %d", input.ExternalGroupID)
		existingRecord, getErr = repo.GetGitLabOrganizationByExternalID(ctx, input.ExternalGroupID)
		if getErr != nil {
			msg := fmt.Sprintf("unable to locate existing GitLab group by ID: %d, error: %+v", input.ExternalGroupID, input.OrganizationFullPath)
			log.WithFields(f).WithError(getErr).Warn(msg)
			return errors.New(msg)
		}
	} else if input.OrganizationFullPath != "" {
		log.WithFields(f).Debugf("checking to see if we have an existing GitLab group full path with value: %s", input.OrganizationFullPath)
		existingRecord, getErr = repo.GetGitLabOrganizationByFullPath(ctx, input.OrganizationFullPath)
		if getErr != nil {
			msg := fmt.Sprintf("unable to locate existing GitLab group by full path: %s, error: %+v", input.OrganizationFullPath, getErr)
			log.WithFields(f).WithError(getErr).Warn(msg)
			return errors.New(msg)
		}
	}

	if existingRecord == nil {
		msg := fmt.Sprintf("error looking up GitLab group using group ID: %d or full path: %s - no results", input.ExternalGroupID, input.OrganizationFullPath)
		log.WithFields(f).Warn(msg)
		return errors.New(msg)
	}

	_, currentTime := utils.CurrentTime()
	note := fmt.Sprintf("Updated configuration on %s by %s.", currentTime, utils.GetUserNameFromContext(ctx))
	if existingRecord.Note != "" {
		note = fmt.Sprintf("%s. %s", existingRecord.Note, note)
	}

	expressionAttributeNames := map[string]*string{
		"#AE":    aws.String(GitLabOrganizationsAutoEnabledColumn),
		"#AECLA": aws.String(GitLabOrganizationsAutoEnabledCLAGroupIDColumn),
		"#BP":    aws.String(GitLabOrganizationsBranchProtectionEnabledColumn),
		"#M":     aws.String(GitLabOrganizationsDateModifiedColumn),
		"#E":     aws.String(GitLabOrganizationsEnabledColumn),
		"#N":     aws.String(GitLabOrganizationsNoteColumn),
	}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{
		":ae": {
			BOOL: aws.Bool(input.AutoEnabled),
		},
		":aecla": {
			S: aws.String(input.AutoEnabledClaGroupID),
		},
		":bp": {
			BOOL: aws.Bool(input.BranchProtectionEnabled),
		},
		":m": {
			S: aws.String(currentTime),
		},
		":e": {
			BOOL: aws.Bool(enabled),
		},
		":n": {
			S: aws.String(note),
		},
	}
	updateExpression := "SET #AE = :ae, #AECLA = :aecla, #BP = :bp, #M = :m, #E = :e, #N = :n "

	if input.OrganizationName != "" {
		expressionAttributeNames["#N"] = aws.String(GitLabOrganizationsOrganizationNameColumn)
		expressionAttributeValues[":n"] = &dynamodb.AttributeValue{S: aws.String(input.OrganizationName)}
		updateExpression = fmt.Sprintf("%s, #N = :n ", updateExpression)

		expressionAttributeNames["#NL"] = aws.String(GitLabOrganizationsOrganizationNameColumn)
		expressionAttributeValues[":nl"] = &dynamodb.AttributeValue{S: aws.String(strings.ToLower(input.OrganizationName))}
		updateExpression = fmt.Sprintf("%s, #NL = :nl ", updateExpression)
	}

	updateItemInput := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			GitLabOrganizationsOrganizationIDColumn: {
				S: aws.String(existingRecord.OrganizationID),
			},
		},
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		UpdateExpression:          &updateExpression,
		TableName:                 aws.String(repo.gitlabOrgTableName),
	}

	log.WithFields(f).Debugf("updating GitLab organization record: %+v", input)
	_, updateErr := repo.dynamoDBClient.UpdateItem(updateItemInput)
	if updateErr != nil {
		log.WithFields(f).WithError(updateErr).Warnf("unable to update GitLab organization record, error: %+v", updateErr)
		return updateErr
	}

	return nil
}

// DeleteGitLabOrganizationByFullPath deletes the specified GitLab organization
func (repo *Repository) DeleteGitLabOrganizationByFullPath(ctx context.Context, projectSFID, gitlabOrgFullPath string) error {
	f := logrus.Fields{
		"functionName":      "v1.gitlab_organizations.repository.DeleteGitLabOrganizationByFullPath",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"projectSFID":       projectSFID,
		"gitlabOrgFullPath": gitlabOrgFullPath,
	}

	log.WithFields(f).Debugf("loading GitLab group/organizations list for path: %s", gitlabOrgFullPath)
	org, orgErr := repo.GetGitLabOrganizationByFullPath(ctx, gitlabOrgFullPath)
	if orgErr != nil {
		errMsg := fmt.Sprintf("GitLab group/organization is not found using group/organization: %s, error: %+v", gitlabOrgFullPath, orgErr)
		log.WithFields(f).WithError(orgErr).Warn(errMsg)
		return errors.New(errMsg)
	}
	// Nothing to delete or disable
	if org == nil || !org.Enabled {
		return nil
	}

	log.WithFields(f).Debugf("deleting GitLab group/organization under path: %s...", gitlabOrgFullPath)
	// Update enabled flag as false
	_, currentTime := utils.CurrentTime()
	note := fmt.Sprintf("Enabled set to false due to org deletion on %s by %s.", currentTime, utils.GetUserNameFromContext(ctx))
	if org.Note != "" {
		note = fmt.Sprintf("%s. %s", org.Note, note)
	}
	_, err := repo.dynamoDBClient.UpdateItem(
		&dynamodb.UpdateItemInput{
			Key: map[string]*dynamodb.AttributeValue{
				GitLabOrganizationsOrganizationIDColumn: {
					S: aws.String(org.OrganizationID),
				},
			},
			ExpressionAttributeNames: map[string]*string{
				"#E":     aws.String(GitLabOrganizationsEnabledColumn),
				"#N":     aws.String(GitLabOrganizationsNoteColumn),
				"#D":     aws.String(GitLabOrganizationsDateModifiedColumn),
				"#AI":    aws.String(GitLabOrganizationsAuthInfoColumn),
				"#AE":    aws.String(GitLabOrganizationsAutoEnabledColumn),
				"#AECLA": aws.String(GitLabOrganizationsAutoEnabledCLAGroupIDColumn),
				"#EID":   aws.String(GitLabOrganizationsExternalGitLabGroupIDColumn),
				"#BP":    aws.String(GitLabOrganizationsBranchProtectionEnabledColumn),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":e": {
					BOOL: aws.Bool(false),
				},
				":n": {
					S: aws.String(note),
				},
				":d": {
					S: aws.String(currentTime),
				},
				":ai": {
					S: aws.String(""),
				},
				":ae": {
					BOOL: aws.Bool(false),
				},
				":aecla": {
					S: aws.String(""),
				},
				":eid": {
					N: aws.String("0"),
				},
				":bp": {
					BOOL: aws.Bool(false),
				},
			},
			UpdateExpression: aws.String("SET #E = :e, #N = :n, #D = :d, #AI = :ai, #AE = :ae, #AECLA = :aecla, #EID = :eid, #BP = :bp"),
			TableName:        aws.String(repo.gitlabOrgTableName),
		},
	)
	if err != nil {
		errMsg := fmt.Sprintf("error updating gitlab organization by path: %s using GitLab group/organization ID: %s - %+v", gitlabOrgFullPath, org.OrganizationID, err)
		log.WithFields(f).WithError(err).Warnf(errMsg)
		return errors.New(errMsg)
	}

	return nil
}

func buildGitlabOrganizationListModels(ctx context.Context, gitlabOrganizations []*common.GitLabOrganization) []*v2Models.GitlabOrganization {
	f := logrus.Fields{
		"functionName":   "v2.gitlab_organizations.repository.buildGitlabOrganizationListModels",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	log.WithFields(f).Debugf("fetching gitlab info for the list")
	// Convert the database model to a response model
	return common.ToModels(gitlabOrganizations)
}

// getOrganizationsWithConditionFilter fetches the repository entry based on the specified condition and filter criteria
// using the provided index
func (repo *Repository) getOrganizationsWithConditionFilter(ctx context.Context, condition expression.KeyConditionBuilder, filter expression.ConditionBuilder, indexName string) (*v2Models.GitlabOrganizations, error) {
	f := logrus.Fields{
		"functionName":   "v2.gitlab_organizations.repository.getOrganizationsWithConditionFilter",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"indexName":      indexName,
	}

	builder := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter)

	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem building query expression, error: %+v", err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.gitlabOrgTableName),
		IndexName:                 aws.String(indexName),
	}

	log.WithFields(f).Debugf("query: %+v", queryInput)
	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("problem retrieving gitlab_organizations, error = %s", err.Error())
		return nil, err
	}

	if len(results.Items) == 0 {
		log.WithFields(f).Debug("no results from query")
		return &v2Models.GitlabOrganizations{
			List: []*v2Models.GitlabOrganization{},
		}, nil
	}

	var resultOutput []*common.GitLabOrganization
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &resultOutput)
	if err != nil {
		return nil, err
	}

	log.WithFields(f).Debugf("building response model for %d results...", len(resultOutput))
	gitlabOrgList := buildGitlabOrganizationListModels(ctx, resultOutput)
	return &v2Models.GitlabOrganizations{List: gitlabOrgList}, nil
}

func updateResponse(fullResponse, response *v2Models.GitlabOrganizations) {
	if fullResponse.List == nil {
		fullResponse.List = response.List
		return
	}

	if response != nil && response.List != nil {
		for _, item := range response.List {
			found := false
			for _, fr := range fullResponse.List {
				if fr.OrganizationID == item.OrganizationID {
					found = true
					break
				}
			}
			if !found {
				fullResponse.List = append(fullResponse.List, item)
			}
		}
	}
}

func (repo *Repository) getScanResults(ctx context.Context, filter *expression.ConditionBuilder) (*v2Models.GitlabOrganizations, error) {
	f := logrus.Fields{
		"functionName":   "v2.gitlab_organizations.repository.GetGitLabOrganizations",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	builder := expression.NewBuilder()

	// Add the filter if provided
	if filter != nil {
		builder = builder.WithFilter(*filter)
	}

	// Build the scan/query expression
	expr, builderErr := builder.Build()
	if builderErr != nil {
		log.WithFields(f).Warnf("error building expression for %s scan, error: %v", repo.gitlabOrgTableName, builderErr)
		return nil, builderErr
	}

	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.gitlabOrgTableName),
	}

	var resultList []map[string]*dynamodb.AttributeValue
	for {
		results, scanErr := repo.dynamoDBClient.Scan(scanInput) //nolint
		if scanErr != nil {
			log.WithFields(f).Warnf("error retrieving scan results from table %s, error: %v", repo.gitlabOrgTableName, scanErr)
			return nil, scanErr
		}
		resultList = append(resultList, results.Items...)
		if len(results.LastEvaluatedKey) != 0 {
			scanInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}

	var resultOutput []*common.GitLabOrganization
	unmarshalErr := dynamodbattribute.UnmarshalListOfMaps(resultList, &resultOutput)
	if unmarshalErr != nil {
		log.Warnf("error unmarshalling %s from database. error: %v", repo.gitlabOrgTableName, unmarshalErr)
		return nil, unmarshalErr
	}

	return &v2Models.GitlabOrganizations{List: common.ToModels(resultOutput)}, nil
}
