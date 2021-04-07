// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/users"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/gen/restapi/operations/signatures"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gerrits"
	"github.com/communitybridge/easycla/cla-backend-go/github"
	"github.com/communitybridge/easycla/cla-backend-go/github_organizations"
	"github.com/communitybridge/easycla/cla-backend-go/repositories"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"

	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
)

// constants
const (
	LoadACLDetails                                 = true
	DontLoadACLDetails                             = false
	SignatureProjectIDIndex                        = "project-signature-index"
	SignatureProjectDateIDIndex                    = "project-signature-date-index"
	SignatureProjectReferenceIndex                 = "signature-project-reference-index"
	SignatureProjectIDSigTypeSignedApprovedIDIndex = "signature-project-id-sigtype-signed-approved-id-index"
	SignatureProjectIDTypeIndex                    = "signature-project-id-type-index"
	SignatureReferenceIndex                        = "reference-signature-index"
	SignatureReferenceSearchIndex                  = "reference-signature-search-index"

	HugePageSize    = 10000
	DefaultPageSize = 10
	BigPageSize     = 100
)

// SignatureRepository interface defines the functions for the github whitelist service
type SignatureRepository interface {
	GetGithubOrganizationsFromWhitelist(ctx context.Context, signatureID string) ([]models.GithubOrg, error)
	AddGithubOrganizationToWhitelist(ctx context.Context, signatureID, githubOrganizationID string) ([]models.GithubOrg, error)
	DeleteGithubOrganizationFromWhitelist(ctx context.Context, signatureID, githubOrganizationID string) ([]models.GithubOrg, error)
	InvalidateProjectRecord(ctx context.Context, signatureID, note string) error

	GetSignature(ctx context.Context, signatureID string) (*models.Signature, error)
	GetIndividualSignature(ctx context.Context, claGroupID, userID string) (*models.Signature, error)
	GetCorporateSignature(ctx context.Context, claGroupID, companyID string) (*models.Signature, error)
	GetSignatureACL(ctx context.Context, signatureID string) ([]string, error)
	GetProjectSignatures(ctx context.Context, params signatures.GetProjectSignaturesParams) (*models.Signatures, error)
	CreateProjectSummaryReport(ctx context.Context, params signatures.CreateProjectSummaryReportParams) (*models.SignatureReport, error)
	GetProjectCompanySignature(ctx context.Context, companyID, projectID string, signed, approved *bool, nextKey *string, pageSize *int64) (*models.Signature, error)
	GetProjectCompanySignatures(ctx context.Context, companyID, projectID string, signed, approved *bool, nextKey *string, sortOrder *string, pageSize *int64) (*models.Signatures, error)
	GetProjectCompanyEmployeeSignatures(ctx context.Context, params signatures.GetProjectCompanyEmployeeSignaturesParams, criteria *ApprovalCriteria, pageSize int64) (*models.Signatures, error)
	GetCompanySignatures(ctx context.Context, params signatures.GetCompanySignaturesParams, pageSize int64, loadACL bool) (*models.Signatures, error)
	GetCompanyIDsWithSignedCorporateSignatures(ctx context.Context, claGroupID string) ([]SignatureCompanyID, error)
	GetUserSignatures(ctx context.Context, params signatures.GetUserSignaturesParams, pageSize int64) (*models.Signatures, error)
	ProjectSignatures(ctx context.Context, projectID string) (*models.Signatures, error)
	UpdateApprovalList(ctx context.Context, claManager *models.User, claGroupModel *models.ClaGroup, companyID string, params *models.ApprovalList, eventArgs *events.LogEventArgs) (*models.Signature, error)

	AddCLAManager(ctx context.Context, signatureID, claManagerID string) (*models.Signature, error)
	RemoveCLAManager(ctx context.Context, signatureID, claManagerID string) (*models.Signature, error)

	removeColumn(ctx context.Context, signatureID, columnName string) (*models.Signature, error)

	AddSigTypeSignedApprovedID(ctx context.Context, signatureID string, val string) error
	AddUsersDetails(ctx context.Context, signatureID string, userID string) error
	AddSignedOn(ctx context.Context, signatureID string) error

	GetClaGroupICLASignatures(ctx context.Context, claGroupID string, searchTerm *string, pageSize int64, nextKey string) (*models.IclaSignatures, error)
	GetClaGroupCorporateContributors(ctx context.Context, claGroupID string, companyID *string, searchTerm *string) (*models.CorporateContributorList, error)
}

type iclaSignatureWithDetails struct {
	IclaSignature        *models.IclaSignature
	SignatureReferenceID string
}

// repository data model
type repository struct {
	stage              string
	dynamoDBClient     *dynamodb.DynamoDB
	companyRepo        company.IRepository
	usersRepo          users.UserRepository
	eventsService      events.Service
	repositoriesRepo   repositories.Repository
	ghOrgRepo          github_organizations.Repository
	gerritService      gerrits.Service
	signatureTableName string
}

// NewRepository creates a new instance of the whitelist service
func NewRepository(awsSession *session.Session, stage string, companyRepo company.IRepository, usersRepo users.UserRepository, eventsService events.Service, repositoriesRepo repositories.Repository, ghOrgRepo github_organizations.Repository, gerritService gerrits.Service) SignatureRepository {
	return repository{
		stage:              stage,
		dynamoDBClient:     dynamodb.New(awsSession),
		companyRepo:        companyRepo,
		usersRepo:          usersRepo,
		eventsService:      eventsService,
		repositoriesRepo:   repositoriesRepo,
		ghOrgRepo:          ghOrgRepo,
		gerritService:      gerritService,
		signatureTableName: fmt.Sprintf("cla-%s-signatures", stage),
	}
}

// GetGithubOrganizationsFromWhitelist returns a list of GH organizations stored in the whitelist
func (repo repository) GetGithubOrganizationsFromWhitelist(ctx context.Context, signatureID string) ([]models.GithubOrg, error) {
	f := logrus.Fields{
		"functionName":   "GetGitHubOrganizationsFromWhitelist",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
	}
	// get item from dynamoDB table
	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
	})

	if err != nil {
		log.WithFields(f).Warnf("Error retrieving GH organization whitelist for signatureID: %s, error: %v", signatureID, err)
		return nil, err
	}

	itemFromMap, ok := result.Item["github_org_whitelist"]
	if !ok {
		return nil, nil
	}

	var orgs []models.GithubOrg
	for _, org := range itemFromMap.L {
		selected := true
		orgs = append(orgs, models.GithubOrg{
			ID:       org.S,
			Selected: &selected,
		})
	}

	// Sort the array based on the ID
	sort.Slice(orgs, func(i, j int) bool {
		return *orgs[i].ID < *orgs[j].ID
	})

	return orgs, nil
}

// AddGithubOrganizationToWhitelist adds the specified GH organization to the whitelist
func (repo repository) AddGithubOrganizationToWhitelist(ctx context.Context, signatureID, GitHubOrganizationID string) ([]models.GithubOrg, error) {
	f := logrus.Fields{
		"functionName":         "AddGitHubOrganizationToWhitelist",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"signatureID":          signatureID,
		"GitHubOrganizationID": GitHubOrganizationID,
	}
	// get item from dynamoDB table
	log.WithFields(f).Debugf("querying database for GitHub organization whitelist using signatureID: %s", signatureID)

	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
	})

	if err != nil {
		log.WithFields(f).Warnf("Error retrieving GitHub organization whitelist for signatureID: %s and GH Org: %s, error: %v",
			signatureID, GitHubOrganizationID, err)
		return nil, err
	}

	itemFromMap, ok := result.Item["github_org_whitelist"]
	if !ok {
		log.WithFields(f).Debugf("signatureID: %s is missing the 'github_org_whitelist' column - will add", signatureID)
		itemFromMap = &dynamodb.AttributeValue{}
	}

	// generate new List L without element to be deleted
	// if we find a org with the same id just return without updating the record
	var newList []*dynamodb.AttributeValue
	for _, element := range itemFromMap.L {
		newList = append(newList, element)
		if *element.S == GitHubOrganizationID {
			log.WithFields(f).Debugf("GitHub organization for signature: %s already in the list - nothing to do, org id: %s",
				signatureID, GitHubOrganizationID)
			return buildResponse(itemFromMap.L), nil
		}
	}

	// Add the organization to list
	log.WithFields(f).Debugf("adding GitHub organization for signature: %s to the list, org id: %s",
		signatureID, GitHubOrganizationID)
	newList = append(newList, &dynamodb.AttributeValue{
		S: aws.String(GitHubOrganizationID),
	})

	// return values flag - Returns all of the attributes of the item, as they appear after the UpdateItem operation.
	addReturnValues := "ALL_NEW" // nolint

	// Update dynamoDB table
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#L": aws.String("github_org_whitelist"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":l": {
				L: newList,
			},
		},
		UpdateExpression: aws.String("SET #L = :l"),
		ReturnValues:     &addReturnValues,
	}

	log.WithFields(f).Warnf("updating database record using signatureID: %s with values: %v", signatureID, newList)
	updatedValues, err := repo.dynamoDBClient.UpdateItem(input)
	if err != nil {
		log.WithFields(f).Warnf("Error updating white list, error: %v", err)
		return nil, err
	}

	updatedItemFromMap, ok := updatedValues.Attributes["github_org_whitelist"]
	if !ok {
		msg := fmt.Sprintf("unable to fetch updated whitelist organization values for "+
			"organization id: %s for signature: %s - list is empty - returning empty list",
			GitHubOrganizationID, signatureID)
		log.WithFields(f).Debugf(msg)
		return []models.GithubOrg{}, nil
	}

	return buildResponse(updatedItemFromMap.L), nil
}

// DeleteGithubOrganizationFromWhitelist removes the specified GH organization from the whitelist
func (repo repository) DeleteGithubOrganizationFromWhitelist(ctx context.Context, signatureID, GitHubOrganizationID string) ([]models.GithubOrg, error) {
	f := logrus.Fields{
		"functionName":         "DeleteGitHubOrganizationFromWhitelist",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"signatureID":          signatureID,
		"GitHubOrganizationID": GitHubOrganizationID,
	}
	// get item from dynamoDB table
	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
	})

	if err != nil {
		log.WithFields(f).Warnf("error retrieving GH organization whitelist for signatureID: %s and GH Org: %s, error: %v",
			signatureID, GitHubOrganizationID, err)
		return nil, err
	}

	itemFromMap, ok := result.Item["github_org_whitelist"]
	if !ok {
		log.WithFields(f).Warnf("unable to remove whitelist organization: %s for signature: %s - list is empty",
			GitHubOrganizationID, signatureID)
		return nil, errors.New("no github_org_whitelist column")
	}

	// generate new List L without element to be deleted
	var newList []*dynamodb.AttributeValue
	for _, element := range itemFromMap.L {
		if *element.S != GitHubOrganizationID {
			newList = append(newList, element)
		}
	}

	if len(newList) == 0 {
		// Since we don't have any items in our list, we can't simply update dynamoDB with an empty list,
		// Instead:
		// We need to set the value to NULL to clear it out (otherwise we'll get a validation error like:)
		// ValidationException: ExpressionAttributeValues contains invalid value: Supplied AttributeValue
		// is empty, must contain exactly one of the supported data types for the key)

		log.WithFields(f).Debugf("clearing out github org whitelist for organization: %s for signature: %s - list is empty",
			GitHubOrganizationID, signatureID)
		nullFlag := true

		// update dynamoDB table
		input := &dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#L": aws.String("github_org_whitelist"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":l": {
					NULL: &nullFlag,
				},
			},
			TableName: aws.String(repo.signatureTableName),
			Key: map[string]*dynamodb.AttributeValue{
				"signature_id": {
					S: aws.String(signatureID),
				},
			},
			UpdateExpression: aws.String("SET #L = :l"),
		}

		_, err = repo.dynamoDBClient.UpdateItem(input)
		if err != nil {
			log.WithFields(f).Warnf("error updating github org whitelist to NULL value, error: %v", err)
			return nil, err
		}

		// Return an empty list
		return []models.GithubOrg{}, nil
	}

	// return values flag - Returns all of the attributes of the item, as they appear after the UpdateItem operation.
	updatedReturnValues := "ALL_NEW" // nolint

	// update dynamoDB table
	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#L": aws.String("github_org_whitelist"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":l": {
				L: newList,
			},
		},
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
		UpdateExpression: aws.String("SET #L = :l"),
		ReturnValues:     &updatedReturnValues,
	}

	updatedValues, err := repo.dynamoDBClient.UpdateItem(input)
	if err != nil {
		log.WithFields(f).Warnf("Error updating github org whitelist, error: %v", err)
		return nil, err
	}

	updatedItemFromMap, ok := updatedValues.Attributes["github_org_whitelist"]
	if !ok {
		msg := fmt.Sprintf("unable to fetch updated whitelist organization values for "+
			"organization id: %s for signature: %s - list is empty - returning empty list",
			GitHubOrganizationID, signatureID)
		log.WithFields(f).Debugf(msg)
		return []models.GithubOrg{}, nil
	}

	return buildResponse(updatedItemFromMap.L), nil

}

// GetSignature returns the signature for the specified signature id
func (repo repository) GetSignature(ctx context.Context, signatureID string) (*models.Signature, error) {
	f := logrus.Fields{
		"functionName":   "GetSignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
	}
	// This is the key we want to match
	condition := expression.Key("signature_id").Equal(expression.Value(signatureID))

	// Use the builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for signature ID query, signatureID: %s, error: %v",
			signatureID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.signatureTableName),
	}

	// Make the DynamoDB Query API call
	results, queryErr := repo.dynamoDBClient.Query(queryInput)
	if queryErr != nil {
		log.WithFields(f).Warnf("error retrieving signature ID: %s, error: %v", signatureID, queryErr)
		return nil, queryErr
	}

	// No match, didn't find it
	if *results.Count == 0 {
		return nil, nil
	}

	// Convert the list of DB models to a list of response models - should have zero or 1 given that we query by ID
	signatureList, modelErr := repo.buildProjectSignatureModels(ctx, results, "", LoadACLDetails)
	if modelErr != nil {
		log.WithFields(f).Warnf("error converting DB model to response model for signature: %s, error: %v",
			signatureID, modelErr)
		return nil, modelErr
	}

	if len(signatureList) == 0 {
		return nil, nil
	}

	return signatureList[0], nil
}

// GetIndividualSignature returns the signature record for the specified CLA Group and User
func (repo repository) GetIndividualSignature(ctx context.Context, claGroupID, userID string) (*models.Signature, error) {
	f := logrus.Fields{
		"functionName":           "GetIndividualSignature",
		utils.XREQUESTID:         ctx.Value(utils.XREQUESTID),
		"tableName":              repo.signatureTableName,
		"claGroupID":             claGroupID,
		"userID":                 userID,
		"signatureType":          utils.SignatureTypeCLA,
		"signatureReferenceType": utils.SignatureReferenceTypeUser,
		"signatureApproved":      "true",
		"signatureSigned":        "true",
	}

	log.WithFields(f).Debug("querying signature for icla records ...")

	// These are the keys we want to match for an ICLA Signature with a given CLA Group and User ID
	condition := expression.Key("signature_project_id").Equal(expression.Value(claGroupID)).
		And(expression.Key("signature_reference_id").Equal(expression.Value(userID)))
	filter := expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)).
		And(expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser))).
		And(expression.Name("signature_approved").Equal(expression.Value(aws.Bool(true)))).
		And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(true)))).
		And(expression.Name("signature_user_ccla_company_id").AttributeNotExists())

	builder := expression.NewBuilder().
		WithKeyCondition(condition).
		WithFilter(filter).
		WithProjection(buildProjection())

	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project ICLA signature query, error: %v", err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.signatureTableName),
		Limit:                     aws.Int64(100),                             // The maximum number of items to evaluate (not necessarily the number of matching items)
		IndexName:                 aws.String(SignatureProjectReferenceIndex), // Name of a secondary index to scan
	}

	sigs := make([]*models.Signature, 0)
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		//log.WithFields(f).Debugf("Ran signature project query, results: %+v, error: %+v", results, errQuery)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving project ICLA signature ID, error: %v", errQuery)
			return nil, errQuery
		}

		// Convert the list of DB models to a list of response models
		//log.WithFields(f).Debug("Building response models...")
		signatureList, modelErr := repo.buildProjectSignatureModels(ctx, results, claGroupID, LoadACLDetails)
		if modelErr != nil {
			log.WithFields(f).Warnf("error converting DB model to response model for signatures, error: %v",
				modelErr)
			return nil, modelErr
		}

		// Add to the signatures response model to the list
		sigs = append(sigs, signatureList...)

		//log.WithFields(f).Debugf("LastEvaluatedKey: %+v", results.LastEvaluatedKey)
		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			lastEvaluatedKey = ""
		}
	}

	// Didn't find a matching record
	if len(sigs) == 0 {
		return nil, nil
	}

	if len(sigs) > 1 {
		log.WithFields(f).Warnf("found multiple matching ICLA signatures - found %d total", len(sigs))
	}

	return sigs[0], nil
}

// GetCorporateSignature returns the signature record for the specified CLA Group and Company ID
func (repo repository) GetCorporateSignature(ctx context.Context, claGroupID, companyID string) (*models.Signature, error) {
	f := logrus.Fields{
		"functionName":           "GetCorporateSignature",
		utils.XREQUESTID:         ctx.Value(utils.XREQUESTID),
		"tableName":              repo.signatureTableName,
		"claGroupID":             claGroupID,
		"companyID":              companyID,
		"signatureType":          "ccla",
		"signatureReferenceType": "company",
		"signatureApproved":      "true",
		"signatureSigned":        "true",
	}

	// These are the keys we want to match for an ICLA Signature with a given CLA Group and User ID
	condition := expression.Key("signature_project_id").Equal(expression.Value(claGroupID)).
		And(expression.Key("signature_reference_id").Equal(expression.Value(companyID)))
	filter := expression.Name("signature_type").Equal(expression.Value("ccla")).
		And(expression.Name("signature_reference_type").Equal(expression.Value("company"))).
		And(expression.Name("signature_approved").Equal(expression.Value(aws.Bool(true)))).
		And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(true)))).
		And(expression.Name("signature_user_ccla_company_id").AttributeNotExists())

	builder := expression.NewBuilder().
		WithKeyCondition(condition).
		WithFilter(filter).
		WithProjection(buildProjection())

	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project CCLA signature query, error: %v", err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.signatureTableName),
		Limit:                     aws.Int64(100),                             // The maximum number of items to evaluate (not necessarily the number of matching items)
		IndexName:                 aws.String(SignatureProjectReferenceIndex), // Name of a secondary index to scan
	}

	sigs := make([]*models.Signature, 0)
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving project CCLA signature, error: %v", errQuery)
			return nil, errQuery
		}

		// Convert the list of DB models to a list of response models
		//log.WithFields(f).Debug("Building response models...")
		signatureList, modelErr := repo.buildProjectSignatureModels(ctx, results, claGroupID, LoadACLDetails)
		if modelErr != nil {
			log.WithFields(f).Warnf("error converting DB model to response model for signatures, error: %v",
				modelErr)
			return nil, modelErr
		}

		// Add to the signatures response model to the list
		sigs = append(sigs, signatureList...)

		//log.WithFields(f).Debugf("LastEvaluatedKey: %+v", results.LastEvaluatedKey)
		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			lastEvaluatedKey = ""
		}
	}

	// Didn't find a matching record
	if len(sigs) == 0 {
		return nil, nil
	}

	if len(sigs) > 1 {
		log.WithFields(f).Warnf("found multiple matching ICLA signatures - found %d total", len(sigs))
	}

	return sigs[0], nil
}

// GetSignatureACL returns the signature ACL for the specified signature id
func (repo repository) GetSignatureACL(ctx context.Context, signatureID string) ([]string, error) {
	f := logrus.Fields{
		"functionName":   "GetSignatureACL",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
	}
	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().
		WithProjection(buildSignatureACLProjection()).
		Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for signature ID query, signatureID: %s, error: %v",
			signatureID, err)
		return nil, err
	}

	// Assemble the query input parameters
	itemInput := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {S: aws.String(signatureID)},
		},
		ExpressionAttributeNames: expr.Names(),
		ProjectionExpression:     expr.Projection(),
		TableName:                aws.String(repo.signatureTableName),
	}

	// Make the DynamoDB Query API call
	result, queryErr := repo.dynamoDBClient.GetItem(itemInput)
	if queryErr != nil {
		log.WithFields(f).Warnf("error retrieving signature ID: %s, error: %v", signatureID, queryErr)
		return nil, queryErr
	}

	// No match, didn't find it
	if result.Item == nil {
		return nil, nil
	}

	var dbModel DBManagersModel
	// Unmarshall the DB response
	unmarshallErr := dynamodbattribute.UnmarshalMap(result.Item, &dbModel)
	if unmarshallErr != nil {
		log.WithFields(f).Warnf("error converting DB model signature query using signature ID: %s, error: %v",
			signatureID, unmarshallErr)
		return nil, unmarshallErr
	}

	return dbModel.SignatureACL, nil
}

func addConditionToFilter(filter expression.ConditionBuilder, cond expression.ConditionBuilder, filterAdded *bool) expression.ConditionBuilder {
	if !(*filterAdded) {
		*filterAdded = true
		filter = cond
	} else {
		filter = filter.And(cond)
	}
	return filter
}

// GetProjectSignatures returns a list of signatures for the specified project
func (repo repository) GetProjectSignatures(ctx context.Context, params signatures.GetProjectSignaturesParams) (*models.Signatures, error) {
	f := logrus.Fields{
		"functionName":   "signatures.repository.GetProjectSignatures",
		"tableName":      repo.signatureTableName,
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     params.ProjectID,
		"signatureType":  aws.StringValue(params.SignatureType),
		"searchField":    aws.StringValue(params.SearchField),
		"searchTerm":     aws.StringValue(params.SearchTerm),
		"fullMatch":      aws.BoolValue(params.FullMatch),
		"pageSize":       aws.Int64Value(params.PageSize),
		"nextKey":        aws.StringValue(params.NextKey),
		"sortOrder":      aws.StringValue(params.SortOrder),
	}

	indexName := SignatureProjectIDIndex
	if params.SortOrder != nil && *params.SortOrder != "" {
		indexName = SignatureProjectDateIDIndex
	}

	realPageSize := int64(100)
	if params.PageSize != nil && *params.PageSize > 0 {
		realPageSize = *params.PageSize
	}

	// This is the key we want to match
	condition := expression.Key("signature_project_id").Equal(expression.Value(params.ProjectID))

	builder := expression.NewBuilder().WithProjection(buildProjection())
	var filter expression.ConditionBuilder
	var filterAdded bool

	if params.ClaType != nil {
		filterAdded = true
		if strings.ToLower(*params.ClaType) == utils.ClaTypeICLA {
			filter = expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)).
				And(expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser))).
				And(expression.Name("signature_approved").Equal(expression.Value(aws.Bool(true)))).
				And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(true)))).
				And(expression.Name("signature_user_ccla_company_id").AttributeNotExists())

		} else if strings.ToLower(*params.ClaType) == utils.ClaTypeECLA {
			filter = expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)).
				And(expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser))).
				And(expression.Name("signature_approved").Equal(expression.Value(aws.Bool(true)))).
				And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(true)))).
				And(expression.Name("signature_user_ccla_company_id").AttributeExists())
		} else if strings.ToLower(*params.ClaType) == utils.ClaTypeCCLA {
			filter = expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCCLA)).
				And(expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeCompany))).
				And(expression.Name("signature_approved").Equal(expression.Value(aws.Bool(true)))).
				And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(true)))).
				And(expression.Name("signature_user_ccla_company_id").AttributeNotExists())
		}
	} else {
		if params.SearchField != nil {
			searchFieldExpression := expression.Name("signature_reference_type").Equal(expression.Value(params.SearchField))
			filter = addConditionToFilter(filter, searchFieldExpression, &filterAdded)
		}

		if params.SignatureType != nil {
			if params.SearchTerm != nil && (params.FullMatch != nil && !*params.FullMatch) {
				indexName = SignatureProjectIDTypeIndex
				condition = condition.And(expression.Key("signature_type").Equal(expression.Value(strings.ToLower(*params.SignatureType))))
			} else {
				signatureTypeExpression := expression.Name("signature_type").Equal(expression.Value(params.SignatureType))
				filter = addConditionToFilter(filter, signatureTypeExpression, &filterAdded)
			}
			if *params.SignatureType == utils.ClaTypeCCLA {
				signatureReferenceIDExpression := expression.Name("signature_reference_id").AttributeExists()
				signatureUserCclaCompanyIDExpression := expression.Name("signature_user_ccla_company_id").AttributeNotExists()
				filter = addConditionToFilter(filter, signatureReferenceIDExpression, &filterAdded)
				filter = addConditionToFilter(filter, signatureUserCclaCompanyIDExpression, &filterAdded)
			}
		}

		if params.SearchTerm != nil {
			if *params.FullMatch {
				indexName = SignatureReferenceSearchIndex
				condition = condition.And(expression.Key("signature_reference_name_lower").Equal(expression.Value(strings.ToLower(*params.SearchTerm))))
			} else {
				searchTermExpression := expression.Name("signature_reference_name_lower").Contains(strings.ToLower(*params.SearchTerm)).Or(expression.Name("user_email").Contains(strings.ToLower(*params.SearchTerm)))
				filter = addConditionToFilter(filter, searchTermExpression, &filterAdded)
			}
		}

		// Filter condition to cater for approved and signed signatures
		signatureApprovedExpression := expression.Name("signature_approved").Equal(expression.Value(true))
		filter = addConditionToFilter(filter, signatureApprovedExpression, &filterAdded)

		signatureSignedExpression := expression.Name("signature_signed").Equal(expression.Value(true))
		filter = addConditionToFilter(filter, signatureSignedExpression, &filterAdded)
	}

	if filterAdded {
		builder = builder.WithFilter(filter)
	}
	builder = builder.WithKeyCondition(condition)

	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project signature query, projectID: %s, error: %v",
			params.ProjectID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.signatureTableName),
		Limit:                     aws.Int64(realPageSize), // The maximum number of items to evaluate (not necessarily the number of matching items)
		IndexName:                 aws.String(indexName),   // Name of a secondary index to scan
	}
	f["indexName"] = indexName

	// If we have the next key, set the exclusive start key value
	if params.NextKey != nil {
		log.WithFields(f).Debugf("received a nextKey, value: %s", *params.NextKey)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: params.NextKey,
			},
			"signature_project_id": {
				S: &params.ProjectID,
			},
		}
		if params.FullMatch != nil && *params.FullMatch && params.SearchTerm != nil {
			queryInput.ExclusiveStartKey["signature_reference_name_lower"] = &dynamodb.AttributeValue{
				S: params.SearchTerm,
			}
		}
	}

	sigs := make([]*models.Signature, 0)
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving project signature ID for project: %s, error: %v",
				params.ProjectID, errQuery)
			return nil, errQuery
		}

		// Convert the list of DB models to a list of response models
		signatureList, modelErr := repo.buildProjectSignatureModels(ctx, results, params.ProjectID, LoadACLDetails)
		if modelErr != nil {
			log.WithFields(f).Warnf("error converting DB model to response model for signatures with project %s, error: %v",
				params.ProjectID, modelErr)
			return nil, modelErr
		}

		// Add to the signatures response model to the list
		sigs = append(sigs, signatureList...)

		//log.WithFields(f).Debugf("LastEvaluatedKey: %+v", results.LastEvaluatedKey)
		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			lastEvaluatedKey = ""
		}

		if int64(len(sigs)) >= realPageSize {
			break
		}
	}

	// How many total records do we have - may not be up-to-date as this value is updated only periodically
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: &repo.signatureTableName,
	}
	describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving total record count for project: %s, error: %v", params.ProjectID, err)
		return nil, err
	}

	// Meta-data for the response
	totalCount := *describeTableResult.Table.ItemCount
	if int64(len(sigs)) > realPageSize {
		sigs = sigs[0:realPageSize]
		lastEvaluatedKey = sigs[realPageSize-1].SignatureID
	}

	return &models.Signatures{
		ProjectID:      params.ProjectID,
		ResultCount:    int64(len(sigs)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Signatures:     sigs,
	}, nil
}

// CreateProjectSummaryReport generates a project summary report based on the specified input
func (repo repository) CreateProjectSummaryReport(ctx context.Context, params signatures.CreateProjectSummaryReportParams) (*models.SignatureReport, error) { // nolint
	f := logrus.Fields{
		"functionName":   "signatures.repository.CreateProjectSummaryReport",
		"tableName":      repo.signatureTableName,
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     params.ProjectID,
		"signatureType":  aws.StringValue(params.SignatureType),
		"searchField":    aws.StringValue(params.SearchField),
		"searchTerm":     aws.StringValue(params.SearchTerm),
		"fullMatch":      aws.BoolValue(params.FullMatch),
		"pageSize":       aws.Int64Value(params.PageSize),
		"nextKey":        aws.StringValue(params.NextKey),
		"sortOrder":      aws.StringValue(params.SortOrder),
		"companyIDList":  params.Body,
	}

	indexName := SignatureProjectIDIndex
	if params.SortOrder != nil && *params.SortOrder != "" {
		indexName = SignatureProjectDateIDIndex
	}

	realPageSize := int64(100)
	if params.PageSize != nil && *params.PageSize > 0 {
		realPageSize = *params.PageSize
	}

	// This is the key we want to match
	condition := expression.Key("signature_project_id").Equal(expression.Value(params.ProjectID))

	builder := expression.NewBuilder().WithProjection(buildProjection())
	var filter expression.ConditionBuilder
	var filterAdded bool

	if params.ClaType != nil {
		filterAdded = true
		if strings.ToLower(*params.ClaType) == utils.ClaTypeICLA {
			filter = expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)).
				And(expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser))).
				And(expression.Name("signature_approved").Equal(expression.Value(aws.Bool(true)))).
				And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(true)))).
				And(expression.Name("signature_user_ccla_company_id").AttributeNotExists())

		} else if strings.ToLower(*params.ClaType) == utils.ClaTypeECLA {
			filter = expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)).
				And(expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser))).
				And(expression.Name("signature_approved").Equal(expression.Value(aws.Bool(true)))).
				And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(true)))).
				And(expression.Name("signature_user_ccla_company_id").AttributeExists())
		} else if strings.ToLower(*params.ClaType) == utils.ClaTypeCCLA {
			filter = expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCCLA)).
				And(expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeCompany))).
				And(expression.Name("signature_approved").Equal(expression.Value(aws.Bool(true)))).
				And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(true)))).
				And(expression.Name("signature_user_ccla_company_id").AttributeNotExists())
		}
	} else {
		if params.SearchField != nil {
			searchFieldExpression := expression.Name("signature_reference_type").Equal(expression.Value(params.SearchField))
			filter = addConditionToFilter(filter, searchFieldExpression, &filterAdded)
		}

		if params.SignatureType != nil {
			if params.SearchTerm != nil && (params.FullMatch != nil && !*params.FullMatch) {
				indexName = SignatureProjectIDTypeIndex
				condition = condition.And(expression.Key("signature_type").Equal(expression.Value(strings.ToLower(*params.SignatureType))))
			} else {
				signatureTypeExpression := expression.Name("signature_type").Equal(expression.Value(params.SignatureType))
				filter = addConditionToFilter(filter, signatureTypeExpression, &filterAdded)
			}
			if *params.SignatureType == utils.ClaTypeCCLA {
				signatureReferenceIDExpression := expression.Name("signature_reference_id").AttributeExists()
				signatureUserCclaCompanyIDExpression := expression.Name("signature_user_ccla_company_id").AttributeNotExists()
				filter = addConditionToFilter(filter, signatureReferenceIDExpression, &filterAdded)
				filter = addConditionToFilter(filter, signatureUserCclaCompanyIDExpression, &filterAdded)
			}
		}

		if params.SearchTerm != nil {
			if *params.FullMatch {
				indexName = SignatureReferenceSearchIndex
				condition = condition.And(expression.Key("signature_reference_name_lower").Equal(expression.Value(strings.ToLower(*params.SearchTerm))))
			} else {
				searchTermExpression := expression.Name("signature_reference_name_lower").Contains(strings.ToLower(*params.SearchTerm)).Or(expression.Name("user_email").Contains(strings.ToLower(*params.SearchTerm)))
				filter = addConditionToFilter(filter, searchTermExpression, &filterAdded)
			}
		}

		// Filter condition to cater for approved and signed signatures
		signatureApprovedExpression := expression.Name("signature_approved").Equal(expression.Value(true))
		filter = addConditionToFilter(filter, signatureApprovedExpression, &filterAdded)

		signatureSignedExpression := expression.Name("signature_signed").Equal(expression.Value(true))
		filter = addConditionToFilter(filter, signatureSignedExpression, &filterAdded)
	}

	if len(params.Body) > 0 {
		// expression.Name("Color").In(expression.Value("red"), expression.Value("green"), expression.Value("blue"))
		var referenceIDExpressions []expression.OperandBuilder
		for _, value := range params.Body {
			referenceIDExpressions = append(referenceIDExpressions, expression.Value(value))
		}
		if len(referenceIDExpressions) == 1 {
			filter = addConditionToFilter(filter, expression.Name("signature_reference_id").In(referenceIDExpressions[0]), &filterAdded)
		} else if len(referenceIDExpressions) > 1 {
			filter = addConditionToFilter(filter, expression.Name("signature_reference_id").In(referenceIDExpressions[0], referenceIDExpressions[1:]...), &filterAdded)
		}
	}

	if filterAdded {
		builder = builder.WithFilter(filter)
	}
	builder = builder.WithKeyCondition(condition)

	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project signature query, projectID: %s, error: %v",
			params.ProjectID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.signatureTableName),
		Limit:                     aws.Int64(realPageSize), // The maximum number of items to evaluate (not necessarily the number of matching items)
		IndexName:                 aws.String(indexName),   // Name of a secondary index to scan
	}
	f["indexName"] = indexName

	// If we have the next key, set the exclusive start key value
	if params.NextKey != nil {
		log.WithFields(f).Debugf("received a nextKey, value: %s", *params.NextKey)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: params.NextKey,
			},
			"signature_project_id": {
				S: &params.ProjectID,
			},
		}
		if params.FullMatch != nil && *params.FullMatch && params.SearchTerm != nil {
			queryInput.ExclusiveStartKey["signature_reference_name_lower"] = &dynamodb.AttributeValue{
				S: params.SearchTerm,
			}
		}
	}

	sigs := make([]*models.SignatureSummary, 0)
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving project signature ID for project: %s, error: %v",
				params.ProjectID, errQuery)
			return nil, errQuery
		}

		// Convert the list of DB models to a list of response models
		signatureList, modelErr := repo.buildProjectSignatureSummaryModels(ctx, results, params.ProjectID)
		if modelErr != nil {
			log.WithFields(f).Warnf("error converting DB model to response model for signatures with project %s, error: %v",
				params.ProjectID, modelErr)
			return nil, modelErr
		}

		// Add to the signatures response model to the list
		sigs = append(sigs, signatureList...)

		//log.WithFields(f).Debugf("LastEvaluatedKey: %+v", results.LastEvaluatedKey)
		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			lastEvaluatedKey = ""
		}

		if int64(len(sigs)) >= realPageSize {
			break
		}
	}

	// How many total records do we have - may not be up-to-date as this value is updated only periodically
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: &repo.signatureTableName,
	}
	describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving total record count for project: %s, error: %v", params.ProjectID, err)
		return nil, err
	}

	// Meta-data for the response
	totalCount := *describeTableResult.Table.ItemCount
	if int64(len(sigs)) > realPageSize {
		sigs = sigs[0:realPageSize]
		lastEvaluatedKey = sigs[realPageSize-1].SignatureID
	}

	return &models.SignatureReport{
		ProjectID:      params.ProjectID,
		ResultCount:    int64(len(sigs)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Signatures:     sigs,
	}, nil
}

// GetProjectCompanySignature returns a the signature for the specified project and specified company with the other query flags
func (repo repository) GetProjectCompanySignature(ctx context.Context, companyID, projectID string, signed, approved *bool, nextKey *string, pageSize *int64) (*models.Signature, error) {
	f := logrus.Fields{
		"functionName":   "GetProjectCompanySignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"projectID":      projectID,
		"approved":       aws.BoolValue(approved),
		"signed":         aws.BoolValue(signed),
		"pageSize":       aws.Int64Value(pageSize),
		"nextKey":        aws.StringValue(nextKey),
	}
	sortOrder := utils.SortOrderAscending
	sigs, getErr := repo.GetProjectCompanySignatures(ctx, companyID, projectID, signed, approved, nextKey, &sortOrder, pageSize)
	if getErr != nil {
		return nil, getErr
	}

	if sigs == nil || sigs.Signatures == nil {
		return nil, nil
	}

	if len(sigs.Signatures) > 1 {
		log.WithFields(f).Warnf("more than 1 project company signatures returned in result using company ID: %s, project ID: %s - will return fist record",
			companyID, projectID)
	}

	return sigs.Signatures[0], nil
}

// GetProjectCompanySignatures returns a list of signatures for the specified project and specified company
func (repo repository) GetProjectCompanySignatures(ctx context.Context, companyID, projectID string, signed, approved *bool, nextKey *string, sortOrder *string, pageSize *int64) (*models.Signatures, error) {
	f := logrus.Fields{
		"functionName":   "GetProjectCompanySignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"projectID":      projectID,
		"signed":         aws.BoolValue(signed),
		"approved":       aws.BoolValue(approved),
		"nextKey":        aws.StringValue(nextKey),
		"sortOrder":      aws.StringValue(sortOrder),
		"pageSize":       aws.Int64Value(pageSize),
	}

	// These are the keys we want to match
	condition := expression.Key("signature_project_id").Equal(expression.Value(projectID))
	filter := expression.Name("signature_reference_id").Equal(expression.Value(companyID)).
		And(expression.Name("signature_type").Equal(expression.Value("ccla"))).
		And(expression.Name("signature_reference_type").Equal(expression.Value("company")))

	// If the caller provided a signature signed value...add the appropriate filter
	if signed != nil {
		log.WithFields(f).Debugf("Filtering signature_signed: %+v", *signed)
		filter = filter.And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(*signed))))
	}

	// If the caller provided a signature approved value...add the appropriate filter
	if approved != nil {
		log.WithFields(f).Debugf("Filter by signature_approved: %+v", *approved)
		filter = filter.And(expression.Name("signature_approved").Equal(expression.Value(aws.Bool(*approved))))
	}

	limit := int64(10)
	if pageSize != nil {
		limit = *pageSize
	}

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project signature ID query, project: %s, error: %v",
			projectID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String("project-signature-index"), // Name of a secondary index to scan
		Limit:                     aws.Int64(limit),
	}

	// If we have the next key, set the exclusive start key value
	if nextKey != nil {
		log.WithFields(f).Debugf("Received a nextKey, value: %s", *nextKey)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: nextKey,
			},
			"signature_project_id": {
				S: &projectID,
			},
		}
	}

	var sigs []*models.Signature
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving project signature ID for project: %s with company: %s, error: %v",
				projectID, companyID, errQuery)
			return nil, errQuery
		}

		// Convert the list of DB models to a list of response models
		signatureList, modelErr := repo.buildProjectSignatureModels(ctx, results, projectID, LoadACLDetails)
		if modelErr != nil {
			log.WithFields(f).Warnf("error converting DB model to response model for signatures with project %s with company: %s, error: %v",
				projectID, companyID, modelErr)
			return nil, modelErr
		}

		// Add to the signatures response model to the list
		sigs = append(sigs, signatureList...)

		// log.WithFields(f).Debugf("LastEvaluatedKey: %+v", results.LastEvaluatedKey["signature_id"])
		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"signature_id": {
					S: aws.String(lastEvaluatedKey),
				},
				"signature_project_id": {
					S: &projectID,
				},
			}
		} else {
			lastEvaluatedKey = ""
		}

		if int64(len(sigs)) >= limit {
			break
		}
	}

	// How many total records do we have - may not be up-to-date as this value is updated only periodically
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: &repo.signatureTableName,
	}
	describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving total record count for project: %s, error: %v", projectID, err)
		return nil, err
	}

	// Meta-data for the response
	totalCount := *describeTableResult.Table.ItemCount

	return &models.Signatures{
		ProjectID:      projectID,
		ResultCount:    int64(len(sigs)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Signatures:     sigs,
	}, nil
}

// Get project signatures with no pagination
func (repo repository) ProjectSignatures(ctx context.Context, projectID string) (*models.Signatures, error) {
	f := logrus.Fields{
		"functionName":   "ProjectSignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectID":      projectID,
	}

	indexName := SignatureProjectIDIndex

	// This is the key we want to match
	condition := expression.Key("signature_project_id").Equal(expression.Value(projectID))

	builder := expression.NewBuilder().WithProjection(buildProjection())
	var filter expression.ConditionBuilder
	var filterAdded bool

	// Filter condition to cater for approved and signed signatures
	signatureApprovedExpression := expression.Name("signature_approved").Equal(expression.Value(true))
	filter = addConditionToFilter(filter, signatureApprovedExpression, &filterAdded)

	signatureSignedExpression := expression.Name("signature_signed").Equal(expression.Value(true))
	filter = addConditionToFilter(filter, signatureSignedExpression, &filterAdded)

	if filterAdded {
		builder = builder.WithFilter(filter)
	}
	builder = builder.WithKeyCondition(condition)

	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project signature query, projectID: %s, error: %v",
			projectID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String(indexName), // Name of a secondary index to scan
	}

	results, errQuery := repo.dynamoDBClient.Query(queryInput)

	if errQuery != nil {
		log.WithFields(f).Warnf("error retrieving project signature ID for project: %s, error: %v",
			projectID, errQuery)
		return nil, errQuery
	}

	// Convert the list of DB models to a list of response models
	sigs, modelErr := repo.buildProjectSignatureModels(ctx, results, projectID, LoadACLDetails)
	if modelErr != nil {
		log.WithFields(f).Warnf("error converting DB model to response model for signatures with project %s, error: %v",
			projectID, modelErr)
		return nil, modelErr
	}

	return &models.Signatures{
		ProjectID:  projectID,
		Signatures: sigs,
	}, nil
}

// InvalidateProjectRecord invalidates the specified project record by setting the signature_approved flag to false
func (repo repository) InvalidateProjectRecord(ctx context.Context, signatureID, note string) error {
	f := logrus.Fields{
		"functionName":   "InvalidateProjectRecord",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
	}

	// Update project signatures for signature_approved and notes attributes
	signatureTableName := fmt.Sprintf("cla-%s-signatures", repo.stage)

	expressionAttributeNames := map[string]*string{}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{}
	updateExpression := "SET " // nolint

	expressionAttributeNames["#A"] = aws.String("signature_approved")
	expressionAttributeValues[":a"] = &dynamodb.AttributeValue{BOOL: aws.Bool(false)}
	updateExpression = updateExpression + " #A = :a,"

	expressionAttributeNames["#S"] = aws.String("note")
	expressionAttributeValues[":s"] = &dynamodb.AttributeValue{S: aws.String(note)}
	updateExpression = updateExpression + " #S = :s"

	input := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		UpdateExpression:          &updateExpression,
		TableName:                 aws.String(signatureTableName),
	}

	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("error updating signature_approved for signature_id : %s error : %v ", signatureID, updateErr)
		return updateErr
	}

	return nil
}

// GetProjectCompanyEmployeeSignatures returns a list of employee signatures for the specified project and specified company
func (repo repository) GetProjectCompanyEmployeeSignatures(ctx context.Context, params signatures.GetProjectCompanyEmployeeSignaturesParams, criteria *ApprovalCriteria, pageSize int64) (*models.Signatures, error) {
	f := logrus.Fields{
		"functionName":   "GetProjectCompanyEmployeeSignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectID":      params.ProjectID,
		"companyID":      params.CompanyID,
		"nextKey":        aws.StringValue(params.NextKey),
		"sortOrder":      aws.StringValue(params.SortOrder),
		"pageSize":       aws.Int64Value(params.PageSize),
	}

	// This is the keys we want to match
	condition := expression.Key("signature_user_ccla_company_id").Equal(expression.Value(params.CompanyID)).And(
		expression.Key("signature_project_id").Equal(expression.Value(params.ProjectID)))
	// Check for approved signatures
	filter := expression.Name("signature_approved").Equal(expression.Value(aws.Bool(true))).
		And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(true))))

	if criteria != nil && criteria.GitHubUsername != "" {
		log.WithFields(f).Debugf("adding Githubusername criteria filter for :%s ", criteria.GitHubUsername)
		filter = filter.And(expression.Name("user_github_username").Equal(expression.Value(criteria.GitHubUsername)))
	}

	if criteria != nil && criteria.UserEmail != "" {
		log.WithFields(f).Debugf("adding useremail criteria filter for : %s ", criteria.UserEmail)
		filter = filter.And(expression.Name("user_email").Equal(expression.Value(criteria.UserEmail)))
	}

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project signature ID query, project: %s, error: %v",
			params.ProjectID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String("signature-user-ccla-company-index"), // Name of a secondary index to scan
	}

	// If we have the next key, set the exclusive start key value
	if params.NextKey != nil {
		log.WithFields(f).Debugf("Received a nextKey, value: %s", *params.NextKey)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: params.NextKey,
			},
			"signature_user_ccla_company_id": {
				S: &params.CompanyID,
			},
			"signature_project_id": {
				S: &params.ProjectID,
			},
		}
	}

	sigs := make([]*models.Signature, 0)
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		//log.WithFields(f).Debugf("Running signature project company query using queryInput: %+v", queryInput)
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving project company employee signature ID for project: %s with company: %s, error: %v",
				params.ProjectID, params.CompanyID, errQuery)
			return nil, errQuery
		}

		// Convert the list of DB models to a list of response models
		signatureList, modelErr := repo.buildProjectSignatureModels(ctx, results, params.ProjectID, LoadACLDetails)
		if modelErr != nil {
			log.WithFields(f).Warnf("error converting DB model to response model for employee signatures with project %s with company: %s, error: %v",
				params.ProjectID, params.CompanyID, modelErr)
			return nil, modelErr
		}

		// Add to the signatures response model to the list
		sigs = append(sigs, signatureList...)

		// log.WithFields(f).Debugf("LastEvaluatedKey: %+v", results.LastEvaluatedKey["signature_id"])
		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			lastEvaluatedKey = ""
		}

		if int64(len(sigs)) >= pageSize {
			break
		}
	}

	// How many total records do we have - may not be up-to-date as this value is updated only periodically
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: &repo.signatureTableName,
	}
	describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving total record count for project: %s, error: %v", params.ProjectID, err)
		return nil, err
	}

	// Meta-data for the response
	totalCount := *describeTableResult.Table.ItemCount
	if int64(len(sigs)) > pageSize {
		sigs = sigs[0:pageSize]
		lastEvaluatedKey = sigs[pageSize-1].SignatureID
	}

	return &models.Signatures{
		ProjectID:      params.ProjectID,
		ResultCount:    int64(len(sigs)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Signatures:     sigs,
	}, nil
}

// GetCompanySignatures returns a list of company signatures for the specified company
func (repo repository) GetCompanySignatures(ctx context.Context, params signatures.GetCompanySignaturesParams, pageSize int64, loadACL bool) (*models.Signatures, error) {
	f := logrus.Fields{
		"functionName":   "GetCompanySignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	// This is the keys we want to match
	condition := expression.Key("signature_reference_id").Equal(expression.Value(params.CompanyID))

	// Check for approved signatures
	filter := expression.Name("signature_approved").Equal(expression.Value(aws.Bool(true))).
		And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(true))))

	if params.SignatureType != nil {
		filter = filter.And(expression.Name("signature_type").Equal(expression.Value(*params.SignatureType)))
	}

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for company signature query, companyID: %s, error: %v",
			params.CompanyID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String("reference-signature-index"), // Name of a secondary index to scan
		//Limit:                     aws.Int64(pageSize),                   // The maximum number of items to evaluate (not necessarily the number of matching items)
	}

	// If we have the next key, set the exclusive start key value
	if params.NextKey != nil {
		log.WithFields(f).Debugf("Received a nextKey, value: %s", *params.NextKey)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: params.NextKey,
			},
			"signature_reference_id": {
				S: &params.CompanyID,
			},
		}
	}

	sigs := make([]*models.Signature, 0)
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		//log.WithFields(f).Debugf("Running signature project company query using queryInput: %+v", queryInput)
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving company signature ID for company: %s with company: %s, error: %v",
				params.CompanyID, params.CompanyID, errQuery)
			return nil, errQuery
		}

		// Convert the list of DB models to a list of response models
		signatureList, modelErr := repo.buildProjectSignatureModels(ctx, results, "", loadACL)
		if modelErr != nil {
			log.WithFields(f).Warnf("error converting DB model to response model for signatures with company: %s, error: %v",
				params.CompanyID, modelErr)
			return nil, modelErr
		}

		// Add to the signatures response model to the list
		sigs = append(sigs, signatureList...)

		// log.WithFields(f).Debugf("LastEvaluatedKey: %+v", results.LastEvaluatedKey["signature_id"])
		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			lastEvaluatedKey = ""
		}

		if int64(len(sigs)) >= pageSize {
			break
		}
	}

	// How many total records do we have - may not be up-to-date as this value is updated only periodically
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: &repo.signatureTableName,
	}
	describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving total record count for company: %s/%s, error: %v",
			params.CompanyID, *params.CompanyName, err)
		return nil, err
	}
	if int64(len(sigs)) > pageSize {
		sigs = sigs[0:pageSize]
		lastEvaluatedKey = sigs[pageSize-1].SignatureID
	}

	// Meta-data for the response
	totalCount := *describeTableResult.Table.ItemCount

	return &models.Signatures{
		ProjectID:      "",
		ResultCount:    int64(len(sigs)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Signatures:     sigs,
	}, nil
}

// GetCompanyIDsWithSignedCorporateSignatures returns a list of company IDs that have signed a CLA agreement
func (repo repository) GetCompanyIDsWithSignedCorporateSignatures(ctx context.Context, claGroupID string) ([]SignatureCompanyID, error) {
	f := logrus.Fields{
		"functionName":             "GetCompanyIDsWithSignedCorporateSignatures",
		"claGroupID":               claGroupID,
		"signature_project_id":     claGroupID,
		"signature_type":           "ccla",
		"signature_reference_type": "company",
		"signature_signed":         "true",
		"signature_approved":       "true",
		"tableName":                repo.signatureTableName,
		"stage":                    repo.stage,
	}

	// These are the keys we want to match
	condition := expression.Key("signature_project_id").Equal(expression.Value(claGroupID))
	filter := expression.Name("signature_type").Equal(expression.Value("ccla")).
		And(expression.Name("signature_reference_type").Equal(expression.Value("company"))).
		And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(true)))).
		And(expression.Name("signature_approved").Equal(expression.Value(aws.Bool(true))))

	// Batch size
	limit := int64(100)

	// Use the nice builder to create the expression - this one uses a simple projection with only the signature id (required) and company id - which is the signature reference id field
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter).WithProjection(buildCompanyIDProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression, error: %v", err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String("project-signature-index"), // Name of a secondary index to scan
		Limit:                     aws.Int64(limit),
	}

	var companyIDs []SignatureCompanyID
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving signature record, error: %v", errQuery)
			return nil, errQuery
		}

		companyIDList, buildErr := repo.buildCompanyIDList(ctx, results)
		if buildErr != nil {
			log.WithFields(f).Warnf("problem converting db model to list of company IDs, error: %+v", buildErr)
			return nil, buildErr
		}

		// Convert the list of DB models to a list of response models
		companyIDs = append(companyIDs, companyIDList...)

		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"signature_id": {
					S: aws.String(lastEvaluatedKey),
				},
				"signature_project_id": {
					S: &claGroupID,
				},
			}
		} else {
			lastEvaluatedKey = ""
		}
	}

	return companyIDs, nil
}

// GetUserSignatures returns a list of user signatures for the specified user
func (repo repository) GetUserSignatures(ctx context.Context, params signatures.GetUserSignaturesParams, pageSize int64) (*models.Signatures, error) {
	f := logrus.Fields{
		"functionName":   "GetUserSignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	// This is the keys we want to match
	condition := expression.Key("signature_reference_id").Equal(expression.Value(params.UserID))

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for user signature query, userID: %s, error: %v",
			params.UserID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String(SignatureReferenceIndex), // Name of a secondary index to scan
		Limit:                     aws.Int64(pageSize),                 // The maximum number of items to evaluate (not necessarily the number of matching items)
	}

	// If we have the next key, set the exclusive start key value
	if params.NextKey != nil {
		log.WithFields(f).Debugf("Received a nextKey, value: %s", *params.NextKey)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: params.NextKey,
			},
			"signature_reference_id": {
				S: &params.UserID,
			},
		}
	}

	sigs := make([]*models.Signature, 0)
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving user signatures for user: %s/%s, error: %v",
				params.UserID, *params.UserName, errQuery)
			return nil, errQuery
		}

		// Convert the list of DB models to a list of response models
		signatureList, modelErr := repo.buildProjectSignatureModels(ctx, results, "", LoadACLDetails)
		if modelErr != nil {
			log.WithFields(f).Warnf("error converting DB model to response model for signatures for user %s/%s, error: %v",
				params.UserID, *params.UserName, modelErr)
			return nil, modelErr
		}

		// Add to the signatures response model to the list
		sigs = append(sigs, signatureList...)

		// log.WithFields(f).Debugf("LastEvaluatedKey: %+v", results.LastEvaluatedKey["signature_id"])
		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			lastEvaluatedKey = ""
		}

		if int64(len(sigs)) >= pageSize {
			break
		}
	}

	// How many total records do we have - may not be up-to-date as this value is updated only periodically
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: &repo.signatureTableName,
	}
	describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving total record count for user: %s/%s, error: %v",
			params.UserID, *params.UserName, err)
		return nil, err
	}

	// Meta-data for the response
	totalCount := *describeTableResult.Table.ItemCount

	return &models.Signatures{
		ProjectID:      "",
		ResultCount:    int64(len(sigs)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Signatures:     sigs,
	}, nil
}

func (repo repository) AddCLAManager(ctx context.Context, signatureID, claManagerID string) (*models.Signature, error) {
	f := logrus.Fields{
		"functionName":   "AddCLAManager",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
		"claManagerID":   claManagerID,
	}
	aclEntries, err := repo.GetSignatureACL(ctx, signatureID)
	if err != nil {
		log.WithFields(f).Warnf("unable to fetch signature by ID: %s, error: %+v", signatureID, err)
		return nil, err
	}

	if aclEntries == nil {
		log.WithFields(f).Warnf("unable to fetch signature by ID: %s - record not found", signatureID)
		return nil, nil
	}

	for _, manager := range aclEntries {
		if claManagerID == manager {
			return nil, errors.New("manager already in signature ACL")
		}
	}

	aclEntries = append(aclEntries, claManagerID)
	log.WithFields(f).Debugf("To be updated ACL List : %+v", aclEntries)

	_, now := utils.CurrentTime()

	input := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#A": aws.String("signature_acl"),
			"#M": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":a": {
				SS: aws.StringSlice(aclEntries),
			},
			":m": {
				S: aws.String(now),
			},
		},
		UpdateExpression: aws.String("SET #A = :a, #M = :m"),
		TableName:        aws.String(fmt.Sprintf("cla-%s-signatures", repo.stage)),
	}

	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("add CLA manager - unable to update request with new ACL entry of '%s' for signature ID: %s, error: %v",
			claManagerID, signatureID, updateErr)
		return nil, updateErr
	}

	// Load the updated document and return it
	sigModel, err := repo.GetSignature(ctx, signatureID)
	if err != nil {
		log.WithFields(f).Warnf("unable to fetch signature by ID: %s - record not found", signatureID)
		return nil, err
	}

	return sigModel, nil
}

func (repo repository) RemoveCLAManager(ctx context.Context, signatureID, claManagerID string) (*models.Signature, error) {
	f := logrus.Fields{
		"functionName":   "RemoveCLAManager",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
		"claManagerID":   claManagerID,
	}
	aclEntries, err := repo.GetSignatureACL(ctx, signatureID)
	if err != nil {
		log.WithFields(f).Warnf("unable to fetch signature by ID: %s, error: %+v", signatureID, err)
		return nil, err
	}

	if aclEntries == nil {
		log.WithFields(f).Warnf("unable to fetch signature by ID: %s - record not found", signatureID)
		return nil, nil
	}

	// A bit of logic to determine if the manager is listed and to build the new list without the specified manager
	found := false
	var updateEntries []string
	for _, manager := range aclEntries {
		if claManagerID == manager {
			found = true
		} else {
			updateEntries = append(updateEntries, manager)
		}
	}

	if !found {
		return nil, fmt.Errorf("manager ID: %s not found in signature ACL", claManagerID)
	}

	_, now := utils.CurrentTime()

	input := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#A": aws.String("signature_acl"),
			"#M": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":a": {
				SS: aws.StringSlice(updateEntries),
			},
			":m": {
				S: aws.String(now),
			},
		},
		UpdateExpression: aws.String("SET #A = :a, #M = :m"),
		TableName:        aws.String(fmt.Sprintf("cla-%s-signatures", repo.stage)),
	}

	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("remove CLA manager - unable to remove ACL entry of '%s' for signature ID: %s, error: %v",
			claManagerID, signatureID, updateErr)
		return nil, updateErr
	}

	// Load the updated document and return it
	sigModel, err := repo.GetSignature(ctx, signatureID)
	if err != nil {
		log.WithFields(f).Warnf("unable to fetch signature by ID: %s - record not found", signatureID)
		return nil, err
	}

	return sigModel, nil
}

// UpdateApprovalList updates the specified project/company signature with the updated approval list information
func (repo repository) UpdateApprovalList(ctx context.Context, claManager *models.User, claGroupModel *models.ClaGroup, companyID string, params *models.ApprovalList, eventArgs *events.LogEventArgs) (*models.Signature, error) { // nolint

	projectID := claGroupModel.ProjectID
	projectName := claGroupModel.ProjectName
	f := logrus.Fields{
		"functionName": "UpdateApprovalList",
		"projectID":    projectID,
		"companyID":    companyID,
	}
	log.WithFields(f).Debug("querying database for approval list details")

	signed, approved := true, true
	pageSize := int64(10)
	log.WithFields(f).Debugf("querying database for approval list details using company ID: %s project ID: %s, type: ccla, signed: true, approved: true",
		companyID, projectID)
	sortOrder := utils.SortOrderAscending
	sigs, sigErr := repo.GetProjectCompanySignatures(ctx, companyID, projectID, &signed, &approved, nil, &sortOrder, &pageSize)
	if sigErr != nil {
		return nil, sigErr
	}

	if sigs == nil || sigs.Signatures == nil {
		msg := fmt.Sprintf("unable to locate signature for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
			companyID, projectID, signed, approved)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	if len(sigs.Signatures) > 1 {
		log.WithFields(f).Warnf("more than 1 CCLA signature returned for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t - expecting zero or 1 - using first record",
			companyID, projectID, signed, approved)
	}

	// Get CCLA signature - For Approval List info
	cclaSignature, err := repo.GetCorporateSignature(ctx, projectID, companyID)
	if err != nil {
		msg := "unable to get corporate signature"
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	// Keep track of existing company approvals
	approvalList := ApprovalList{
		DomainApprovals:         cclaSignature.DomainApprovalList,
		GHOrgApprovals:          cclaSignature.GithubOrgApprovalList,
		GitHubUsernameApprovals: cclaSignature.GithubUsernameApprovalList,
		EmailApprovals:          cclaSignature.EmailApprovalList,
	}

	// Just grab and use the first one - need to figure out conflict resolution if more than one
	sig := sigs.Signatures[0]
	expressionAttributeNames := map[string]*string{}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{}
	haveAdditions := false
	updateExpression := ""

	employeeSignatureParams := signatures.GetProjectCompanyEmployeeSignaturesParams{
		ProjectID: projectID,
		CompanyID: companyID,
	}

	authUser := auth.User{
		Email:    claManager.LfEmail,
		UserName: claManager.LfUsername,
	}

	// Keep track of gerrit users under a give CLA Group
	var gerritICLAECLAs []string

	log.WithFields(f).Debug("aggregating ICLA and ECLA gerrit users...")
	gerritIclaUsers, err := repo.gerritService.GetUsersOfGroup(ctx, &authUser, projectID, utils.ClaTypeICLA)

	if err != nil {
		msg := fmt.Sprintf("unable to fetch gerrit users for claGroup: %s , claType: %s ", projectID, utils.ClaTypeICLA)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	if gerritIclaUsers != nil {
		for _, member := range gerritIclaUsers.Members {
			gerritICLAECLAs = append(gerritICLAECLAs, member.Username)
		}
	}

	gerritEclaUsers, err := repo.gerritService.GetUsersOfGroup(ctx, &authUser, projectID, utils.ClaTypeECLA)

	if err != nil {
		msg := fmt.Sprintf("unable to fetch gerrit users for claGroup: %s , claType: %s ", projectID, utils.ClaTypeECLA)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	if gerritEclaUsers != nil {
		for _, member := range gerritEclaUsers.Members {
			gerritICLAECLAs = append(gerritICLAECLAs, member.Username)
		}
	}

	// If we have an add or remove email list...we need to run an update for this column
	if params.AddEmailApprovalList != nil || params.RemoveEmailApprovalList != nil {
		columnName := "email_whitelist"
		attrList := buildApprovalAttributeList(ctx, sig.EmailApprovalList, params.AddEmailApprovalList, params.RemoveEmailApprovalList)
		// If no entries after consolidating all the updates, we need to remove the column
		if attrList == nil || attrList.L == nil {
			var rmColErr error
			sig, rmColErr = repo.removeColumn(ctx, sig.SignatureID, columnName)
			if rmColErr != nil {
				msg := fmt.Sprintf("unable to remove column %s for signature for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
					columnName, companyID, projectID, signed, approved)
				log.WithFields(f).Warn(msg)
				return nil, errors.New(msg)
			}
		} else {
			haveAdditions = true
			expressionAttributeNames["#E"] = aws.String("email_whitelist")
			expressionAttributeValues[":e"] = attrList
			updateExpression = updateExpression + " #E = :e, "
		}

		// if email removal update signature approvals
		if params.RemoveEmailApprovalList != nil {
			log.WithFields(f).Debugf("removing email: %+v the approval list", params.RemoveDomainApprovalList)
			var wg sync.WaitGroup
			wg.Add(len(params.RemoveEmailApprovalList))
			for _, email := range params.RemoveEmailApprovalList {
				go func(email string) {
					defer wg.Done()
					log.WithFields(f).Debugf("getting cla user record for email: %s ", email)
					claUser, userErr := repo.usersRepo.GetUserByEmail(email)
					if userErr != nil || claUser == nil {
						log.WithFields(f).Debugf("error getting user by email: %s ", email)
						return
					}
					criteria := &ApprovalCriteria{
						UserEmail: email,
					}
					log.WithFields(f).Debugf("Updating signature records for emailApprovalList: %+v ", params.RemoveEmailApprovalList)
					signs, appErr := repo.GetProjectCompanyEmployeeSignatures(ctx, employeeSignatureParams, criteria, pageSize)
					if appErr != nil {
						log.WithFields(f).Debugf("unable to get Company Employee signatures : %+v ", appErr)
						return
					}

					if len(signs.Signatures) == 0 {
						log.WithFields(f).Debugf("company employee signatures do not exist for company:%s and project: %s ", companyID, projectID)
					}

					if len(signs.Signatures) > 0 {
						log.WithFields(f).Debugf("Updating signature : %s ", signs.Signatures[0].SignatureID)
						note := fmt.Sprintf("Signature invalidated (approved set to false) by %s due to email approval list removal : %+v", utils.GetBestUsername(claManager), params.RemoveEmailApprovalList)

						signErr := repo.InvalidateProjectRecord(ctx, signs.Signatures[0].SignatureID, note)
						if signErr != nil {
							log.WithFields(f).Debugf("error invalidating signature ID: %s error: %+v ", sigs.Signatures[0].SignatureID, signErr)
							return
						}

						// update user by email
						// send email
						eclaEmailSubject := fmt.Sprintf("EasyCLA: ICLA invalidated for %s on %s", email, projectName)
						log.WithFields(f).Debugf("sending invalidation email to user: %s ", email)
						body, err := utils.RenderTemplate(claGroupModel.Version, InvalidateSignatureTemplateName,
							InvalidateSignatureTemplate, InvalidateSignatureTemplateParams{
								RecipientName:   utils.GetBestUsername(claUser),
								ClaType:         utils.ClaTypeCCLA,
								ClaManager:      utils.GetBestUsername(claManager),
								RemovalCriteria: approvalList.Criteria,
								ProjectName:     projectName,
							})
						if err != nil {
							log.WithFields(f).Debugf("unable to render invalidation notice for user : %s ", email)
							return
						}
						err = utils.SendEmail(eclaEmailSubject, body, []string{email})
						if err != nil {
							log.WithFields(f).Debugf("unable to send email invalidation notice to user: %s", email)
							return
						}

						//Log Event
						eventArgs.EventData = &events.SignatureInvalidatedApprovalRejectionEventData{
							SignatureID: signs.Signatures[0].SignatureID,
							Email:       email,
							CLAManager:  claManager,
							CLAGroupID:  signs.Signatures[0].ProjectID,
						}
						repo.eventsService.LogEventWithContext(ctx, eventArgs)
					}

					// invalidate icla
					log.WithFields(f).Debugf("invalidating icla...:user: %+v", claUser)

					if claUser != nil {
						icla, iclaErr := repo.GetIndividualSignature(ctx, projectID, claUser.UserID)
						if iclaErr != nil || icla == nil {
							log.WithFields(f).Debugf("unable to get icla signature for user: %s ", email)
						}
						if icla != nil {
							note := fmt.Sprintf("Signature invalidated (approved set to false) by %s due to %s removal ", utils.GetBestUsername(claManager), email)
							err := repo.InvalidateProjectRecord(ctx, icla.SignatureID, note)
							if err != nil {
								log.WithFields(f).Warnf("unable to invalidate record for user:%s ", email)
							}
							// send email
							log.WithFields(f).Debugf("sending invalidation email to user: %s ", email)
							body, renderErr := utils.RenderTemplate(approvalList.Version, InvalidateSignatureTemplateName,
								InvalidateSignatureTemplate, InvalidateSignatureTemplateParams{
									RecipientName:   utils.GetBestUsername(claUser),
									ClaType:         utils.ClaTypeICLA,
									ClaManager:      utils.GetBestUsername(claManager),
									RemovalCriteria: approvalList.Criteria,
									ProjectName:     projectName,
								})
							if renderErr != nil {
								log.WithFields(f).Debugf("unable to render invalidation notice for user : %s ", email)
								return
							}
							iclaEmailSubject := fmt.Sprintf("EasyCLA: ICLA invalidated for %s on %s", email, projectName)
							err = utils.SendEmail(iclaEmailSubject, body, []string{email})
							if err != nil {
								log.WithFields(f).Debugf("unable to send email invalidation notice to user: %s", email)
								return
							}
						}
					}

					//update gerrit permissions
					gerritUser, err := repo.getGerritUserByEmail(ctx, email, gerritICLAECLAs)
					if err != nil || gerritUser == nil {
						msg := fmt.Sprintf("unable to get gerrit user by email : %s ", email)
						log.WithFields(f).Warn(msg)
						return
					}
					iclaErr := repo.gerritService.RemoveUserFromGroup(ctx, &authUser, approvalList.ClaGroupID, gerritUser.LfUsername, utils.ClaTypeICLA)
					if iclaErr != nil {
						msg := fmt.Sprintf("unable to remove gerrit user:%s from group:%s", gerritUser.LfUsername, approvalList.ClaGroupID)
						log.WithFields(f).Warn(msg)
						return
					}
					eclaErr := repo.gerritService.RemoveUserFromGroup(ctx, &authUser, approvalList.ClaGroupID, gerritUser.LfUsername, utils.ClaTypeECLA)
					if eclaErr != nil {
						msg := fmt.Sprintf("unable to remove gerrit user:%s from group:%s", gerritUser.LfUsername, approvalList.ClaGroupID)
						log.WithFields(f).Warn(msg)
						return
					}
				}(email)
			}
			wg.Wait()
		}
	}

	if params.AddDomainApprovalList != nil || params.RemoveDomainApprovalList != nil {

		columnName := "domain_whitelist"
		attrList := buildApprovalAttributeList(ctx, sig.DomainApprovalList, params.AddDomainApprovalList, params.RemoveDomainApprovalList)
		// If no entries after consolidating all the updates, we need to remove the column
		if attrList == nil || attrList.L == nil {
			var rmColErr error
			sig, rmColErr = repo.removeColumn(ctx, sig.SignatureID, columnName)
			if rmColErr != nil {
				msg := fmt.Sprintf("unable to remove column %s for signature for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
					columnName, companyID, projectID, signed, approved)
				log.WithFields(f).Warn(msg)
				return nil, errors.New(msg)
			}
		} else {
			haveAdditions = true
			expressionAttributeNames["#D"] = aws.String(columnName)
			expressionAttributeValues[":d"] = attrList
			updateExpression = updateExpression + " #D = :d, "
		}
		if params.RemoveDomainApprovalList != nil {
			var invalidateErr error
			approvalList.Criteria = utils.EmailDomainCriteria
			approvalList.ApprovalList = params.RemoveDomainApprovalList
			approvalList.Action = utils.RemoveApprovals
			approvalList.GerritICLAECLAs = gerritICLAECLAs
			approvalList.ClaGroupID = projectID
			approvalList.ClaGroupName = claGroupModel.ProjectName
			approvalList.CompanyID = companyID
			approvalList.Version = claGroupModel.Version
			invalidateErr = repo.invalidateSignatures(ctx, &approvalList, claManager)
			if invalidateErr != nil {
				msg := fmt.Sprintf("unable to invalidate signatures based on Approval List : %+v ", approvalList)
				log.WithFields(f).Warn(msg)
			}
		}
	}

	if params.AddGithubUsernameApprovalList != nil || params.RemoveGithubUsernameApprovalList != nil {
		columnName := "github_whitelist"
		attrList := buildApprovalAttributeList(ctx, sig.GithubUsernameApprovalList, params.AddGithubUsernameApprovalList, params.RemoveGithubUsernameApprovalList)
		// If no entries after consolidating all the updates, we need to remove the column
		if attrList == nil || attrList.L == nil {
			var rmColErr error
			sig, rmColErr = repo.removeColumn(ctx, sig.SignatureID, columnName)
			if rmColErr != nil {
				msg := fmt.Sprintf("unable to remove column %s for signature for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
					columnName, companyID, projectID, signed, approved)
				log.WithFields(f).Warn(msg)
				return nil, errors.New(msg)
			}
		} else {
			haveAdditions = true
			expressionAttributeNames["#G"] = aws.String(columnName)
			expressionAttributeValues[":g"] = attrList
			updateExpression = updateExpression + " #G = :g, "
		}
		if params.RemoveGithubUsernameApprovalList != nil {
			// if email removal update signature approvals
			if params.RemoveGithubUsernameApprovalList != nil {
				var wg sync.WaitGroup
				wg.Add(len(params.RemoveGithubUsernameApprovalList))
				for _, ghUsername := range params.RemoveGithubUsernameApprovalList {
					go func(ghUsername string) {
						defer wg.Done()
						criteria := &ApprovalCriteria{
							GitHubUsername: ghUsername,
						}
						log.WithFields(f).Debugf("Updating signature records for ghUsernameApporvalList: %+v ", params.RemoveGithubUsernameApprovalList)
						signs, ghUserErr := repo.GetProjectCompanyEmployeeSignatures(ctx, employeeSignatureParams, criteria, pageSize)
						if sigErr != nil {
							log.WithFields(f).Debugf("unable to get Company Employee signatures : %+v ", ghUserErr)
							return
						}
						log.WithFields(f).Debugf("Updating signature : %s ", signs.Signatures[0].SignatureID)
						note := fmt.Sprintf("Signature invalidated (approved set to false) by %s due to ghUsernames approval list removal : %+v", utils.GetBestUsername(claManager), params.RemoveGithubUsernameApprovalList)
						signErr := repo.InvalidateProjectRecord(ctx, signs.Signatures[0].SignatureID, note)
						if signErr != nil {
							log.WithFields(f).Debugf("error invalidating signature ID: %s error: %+v ", sigs.Signatures[0].SignatureID, signErr)
							return
						}

						// Log Event
						eventArgs.EventData = &events.SignatureInvalidatedApprovalRejectionEventData{
							SignatureID: signs.Signatures[0].SignatureID,
							GHUsername:  ghUsername,
							CLAManager:  claManager,
							CLAGroupID:  signs.Signatures[0].ProjectID,
						}
						repo.eventsService.LogEventWithContext(ctx, eventArgs)
					}(ghUsername)
				}
				wg.Wait()
			}
		}
	}

	if params.AddGithubOrgApprovalList != nil || params.RemoveGithubOrgApprovalList != nil {
		columnName := "github_org_whitelist"
		attrList := buildApprovalAttributeList(ctx, sig.GithubOrgApprovalList, params.AddGithubOrgApprovalList, params.RemoveGithubOrgApprovalList)
		// If no entries after consolidating all the updates, we need to remove the column
		if attrList == nil || attrList.L == nil {
			var rmColErr error
			sig, rmColErr = repo.removeColumn(ctx, sig.SignatureID, columnName)
			if rmColErr != nil {
				msg := fmt.Sprintf("unable to remove column %s for signature for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
					columnName, companyID, projectID, signed, approved)
				log.WithFields(f).Warn(msg)
				return nil, errors.New(msg)
			}
		} else {
			haveAdditions = true
			expressionAttributeNames["#GO"] = aws.String("github_org_whitelist")
			expressionAttributeValues[":go"] = attrList
			updateExpression = updateExpression + " #GO = :go, "
		}

		if params.RemoveGithubOrgApprovalList != nil {
			var invalidateErr error
			approvalList.Criteria = utils.GitHubOrgCriteria
			approvalList.ApprovalList = params.RemoveGithubOrgApprovalList
			approvalList.Action = utils.RemoveApprovals
			approvalList.Version = claGroupModel.Version
			// Get repositories by CLAGroup
			repositories, err := repo.repositoriesRepo.GetRepositoriesByCLAGroup(ctx, projectID, true)
			if err != nil {
				msg := fmt.Sprintf("unable to fetch repositories for claGroupID: %s ", projectID)
				log.WithFields(f).Warn(msg)
				return nil, errors.New(msg)
			}
			var ghOrgRepositories []*models.GithubRepository
			var ghOrgs []*models.GithubOrganization
			for _, repository := range repositories {
				// Check for matching organization name in repositories table against approvalList removal GH Orgs
				if utils.StringInSlice(repository.RepositoryOrganizationName, approvalList.ApprovalList) {
					ghOrgRepositories = append(ghOrgRepositories, repository)
				}
			}

			for _, ghOrgRepo := range ghOrgRepositories {
				ghOrg, err := repo.ghOrgRepo.GetGithubOrganization(ctx, ghOrgRepo.RepositoryOrganizationName)
				if err != nil {
					msg := fmt.Sprintf("unable to get gh org by name: %s ", ghOrgRepo.RepositoryOrganizationName)
					log.WithFields(f).Warn(msg)
					return nil, errors.New(msg)
				}
				ghOrgs = append(ghOrgs, ghOrg)
			}

			var ghUsernames []string
			for _, ghOrg := range ghOrgs {
				ghOrgUsers, err := github.GetOrganizationMembers(ctx, ghOrg.OrganizationName, ghOrg.OrganizationInstallationID)
				if err != nil {
					msg := fmt.Sprintf("unable to fetch ghOrgUsers for org: %s ", ghOrg.OrganizationName)
					log.WithFields(f).Warnf(msg)
					return nil, errors.New(msg)
				}
				ghUsernames = append(ghUsernames, ghOrgUsers...)
			}
			approvalList.GHUsernames = utils.RemoveDuplicates(ghUsernames)

			invalidateErr = repo.invalidateSignatures(ctx, &approvalList, claManager)
			if invalidateErr != nil {
				msg := fmt.Sprintf("unable to invalidate signatures based on Approval List: %+v ", approvalList)
				log.WithFields(f).Warn(msg)
				return nil, errors.New(msg)
			}
		}
	}

	// Ensure at least one value is set for us to update
	if !haveAdditions {
		log.WithFields(f).Debugf("no updates required to any of the approved list values company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t - expecting at least something to update",
			companyID, projectID, signed, approved)
		return sig, nil
	}

	// Remove trailing comma from the expression, if present
	updateExpression = utils.TrimRemoveTrailingComma("SET " + updateExpression)

	// Update dynamoDB table
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(sig.SignatureID),
			},
		},
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		UpdateExpression:          aws.String(updateExpression), //aws.String("SET #L = :l"),
	}

	log.WithFields(f).Debugf("updating approval list for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
		companyID, projectID, signed, approved)

	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("error updating approval lists for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t, error: %v",
			companyID, projectID, signed, approved, updateErr)
		return nil, updateErr
	}

	log.WithFields(f).Debugf("querying database for approval list details after update using company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
		companyID, projectID, signed, approved)

	updatedSig, sigErr := repo.GetProjectCompanySignatures(ctx, companyID, projectID, &signed, &approved, nil, &sortOrder, &pageSize)
	if sigErr != nil {
		return nil, sigErr
	}

	if updatedSig == nil || updatedSig.Signatures == nil {
		msg := fmt.Sprintf("unable to locate signature after update for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
			companyID, projectID, signed, approved)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	if len(updatedSig.Signatures) > 1 {
		log.WithFields(f).Warnf("more than 1 CCLA signature returned after update for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t - expecting zero or 1 - using first record",
			companyID, projectID, signed, approved)
	}

	// Just grab and use the first one - need to figure out conflict resolution if more than one
	return updatedSig.Signatures[0], nil
}

// invalidateSignatures is a helper function that invalidates signature records based on approval list
func (repo repository) invalidateSignatures(ctx context.Context, approvalList *ApprovalList, claManager *models.User) error {
	f := logrus.Fields{
		"functionName":   "invalidateSignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     &approvalList,
	}

	// Get ICLAs
	log.WithFields(f).Debug("getting icla records... ")
	iclas, err := repo.GetClaGroupICLASignatures(ctx, approvalList.ClaGroupID, nil, 0, "")
	if err != nil {
		log.WithFields(f).Warn("unable to get iclas")
	}

	if iclas != nil {
		var iclaWg sync.WaitGroup
		//Iterate iclas
		iclaWg.Add(len(iclas.List))
		log.WithFields(f).Debug("invalidating signature icla records... ")
		for _, icla := range iclas.List {
			go func(icla *models.IclaSignature) {
				defer iclaWg.Done()
				signature, sigErr := repo.GetSignature(ctx, icla.SignatureID)
				if sigErr != nil {
					log.WithFields(f).Warnf("unable to fetch signature for ID: %s ", icla.SignatureID)
					return
				}
				// Grab user record
				if signature.SignatureReferenceID == "" {
					log.WithFields(f).Warnf("no signatureReferenceID for signature: %+v ", signature)
					return
				}
				user, verifyErr := repo.verifyUserApprovals(ctx, signature.SignatureReferenceID, signature.SignatureID, claManager, approvalList)
				if verifyErr != nil {
					log.WithFields(f).Warnf("unable to verify user: %s ", signature.SignatureReferenceID)
					return
				}
				email := getBestEmail(user)
				// send email
				domainSubject := fmt.Sprintf("EasyCLA: ICLA invalidated for %s on %s", email, approvalList.ClaGroupName)
				log.WithFields(f).Debugf("sending invalidation email to user: %s ", email)
				body, renderErr := utils.RenderTemplate(approvalList.Version, InvalidateSignatureTemplateName,
					InvalidateSignatureTemplate, InvalidateSignatureTemplateParams{
						RecipientName:   utils.GetBestUsername(user),
						ClaType:         utils.ClaTypeICLA,
						ClaManager:      utils.GetBestUsername(claManager),
						RemovalCriteria: approvalList.Criteria,
					})
				if renderErr != nil {
					log.WithFields(f).Debugf("unable to render invalidation notice for user : %s ", email)
					return
				}
				err = utils.SendEmail(domainSubject, body, []string{email})
				if err != nil {
					log.WithFields(f).Debugf("unable to send email invalidation notice to user: %s", email)
					return
				}
			}(icla)
		}
		iclaWg.Wait()
	}

	// Get ECLAs
	log.WithFields(f).Debug("getting ecla records... ")
	companyProjectParams := signatures.GetProjectCompanyEmployeeSignaturesParams{
		CompanyID: approvalList.CompanyID,
		ProjectID: approvalList.ClaGroupID,
	}

	criteria := ApprovalCriteria{}
	eclas, err := repo.GetProjectCompanyEmployeeSignatures(ctx, companyProjectParams, &criteria, int64(10))
	if err != nil {
		log.WithFields(f).Warnf("unable to get cclas for company: %s and project: %s ", approvalList.CompanyID, approvalList.ClaGroupID)
		return err
	}

	if eclas != nil {
		var eclaWg sync.WaitGroup
		log.WithFields(f).Debug("invalidating signature ecla records... ")
		// Iterate eclas
		eclaWg.Add(len(eclas.Signatures))
		for _, ecla := range eclas.Signatures {
			go func(ecla *models.Signature) {
				defer eclaWg.Done()
				// Grab user record
				if ecla.SignatureReferenceID == "" {
					log.WithFields(f).Warnf("no signatureReferenceID for signature: %+v ", ecla)
					return
				}
				user, verifyErr := repo.verifyUserApprovals(ctx, ecla.SignatureReferenceID, ecla.SignatureID, claManager, approvalList)
				if verifyErr != nil {
					log.WithFields(f).Warnf("unable to verify user: %s ", ecla.SignatureReferenceID)
					return
				}
				email := getBestEmail(user)
				// send email
				log.WithFields(f).Debugf("sending invalidation email to user: %s ", email)
				_, err := utils.RenderTemplate(approvalList.Version, InvalidateSignatureTemplateName,
					InvalidateSignatureTemplate, InvalidateSignatureTemplateParams{
						RecipientName:   utils.GetBestUsername(user),
						ClaType:         utils.ClaTypeECLA,
						ClaManager:      utils.GetBestUsername(claManager),
						RemovalCriteria: approvalList.Criteria,
					})
				if err != nil {
					log.WithFields(f).Debugf("unable to send invalidation signature email to : %s ", email)
					return
				}
			}(ecla)
		}
		eclaWg.Wait()
	}
	return nil
}

// getGerritUsersByEmail searches gerrit instances for users with given email
func (repo repository) getGerritUserByEmail(ctx context.Context, email string, gerritICLAECLAs []string) (*models.User, error) {
	f := logrus.Fields{
		"functionName":   "getGerritUserByEmailDomain",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"email":          email,
	}

	log.WithFields(f).Debugf("checking gerrit user for email: %s ", email)
	if email != "" {
		claUser, err := repo.usersRepo.GetUserByEmail(email)
		if err != nil {
			msg := fmt.Sprintf("unable to get easyclauser by email: %s ", email)
			log.WithFields(f).Warn(msg)
			return nil, err
		}
		if utils.StringInSlice(claUser.LfUsername, gerritICLAECLAs) {
			return claUser, nil
		}
	}

	return nil, nil
}

// verify UserApprovals checks user
func (repo repository) verifyUserApprovals(ctx context.Context, userID, signatureID string, claManager *models.User, approvalList *ApprovalList) (*models.User, error) {
	f := logrus.Fields{
		"functionName":   "verifyUserApprovals",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"userID":         userID,
	}

	user, err := repo.usersRepo.GetUser(userID)
	if err != nil {
		log.WithFields(f).Warnf("unable to get user record for ID: %s ", userID)
		return nil, err
	}

	authUser := auth.User{
		Email:    claManager.LfEmail,
		UserName: claManager.LfUsername,
	}

	if approvalList.Criteria == utils.EmailDomainCriteria {
		// Handle Domains
		email := getBestEmail(user)
		domain := strings.Split(email, "@")[1]
		if utils.StringInSlice(domain, approvalList.DomainApprovals) {
			if !utils.StringInSlice(user.GithubUsername, approvalList.GitHubUsernameApprovals) && !utils.StringInSlice(getBestEmail(user), approvalList.EmailApprovals) {
				//Invalidate record
				note := fmt.Sprintf("Signature invalidated (approved set to false) by %s due to %s  removal", utils.GetBestUsername(claManager), utils.EmailDomainCriteria)
				err := repo.InvalidateProjectRecord(ctx, signatureID, note)
				if err != nil {
					log.WithFields(f).Warnf("unable to invalidate record for signatureID: %s ", signatureID)
					return user, err
				}

				log.WithFields(f).Debugf("removing gerrit user:%s  from claGroup: %s ...", user.LfUsername, approvalList.ClaGroupID)
				iclaErr := repo.gerritService.RemoveUserFromGroup(ctx, &authUser, approvalList.ClaGroupID, user.LfUsername, utils.ClaTypeICLA)
				if iclaErr != nil {
					msg := fmt.Sprintf("unable to remove gerrit user:%s from group:%s", user.LfUsername, approvalList.ClaGroupID)
					log.WithFields(f).Warn(msg)
					return user, iclaErr
				}
				eclaErr := repo.gerritService.RemoveUserFromGroup(ctx, &authUser, approvalList.ClaGroupID, user.LfUsername, utils.ClaTypeECLA)
				if eclaErr != nil {
					msg := fmt.Sprintf("unable to remove gerrit user:%s from group:%s", user.LfUsername, approvalList.ClaGroupID)
					log.WithFields(f).Warn(msg)
					return user, eclaErr
				}
			}
		}
	} else if approvalList.Criteria == utils.GitHubOrgCriteria {
		// Handle GH Org Approvals
		if utils.StringInSlice(user.GithubUsername, approvalList.GHUsernames) {
			if !utils.StringInSlice(getBestEmail(user), approvalList.EmailApprovals) && !utils.StringInSlice(user.GithubUsername, approvalList.GitHubUsernameApprovals) {
				//Invalidate record
				note := fmt.Sprintf("Signature invalidated (approved set to false) by %s due to %s  removal", utils.GetBestUsername(claManager), utils.GitHubOrgCriteria)
				err := repo.InvalidateProjectRecord(ctx, signatureID, note)
				if err != nil {
					log.WithFields(f).Warnf("unable to invalidate record for signatureID: %s ", signatureID)
					return user, err
				}
			}
		}
	}

	return user, nil
}

// removeColumn is a helper function to remove a given column when we need to zero out the column value - typically the approval list
func (repo repository) removeColumn(ctx context.Context, signatureID, columnName string) (*models.Signature, error) {
	f := logrus.Fields{
		"functionName": "removeColumn",
		"signatureID":  signatureID,
		"columnName":   columnName,
	}
	log.WithFields(f).Debug("removing column from signature")

	// Update dynamoDB table
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#" + columnName: aws.String(columnName),
		},
		//ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
		//	":a": {
		//		S: aws.String("bar"),
		//	},
		//},
		UpdateExpression: aws.String("REMOVE #" + columnName), //aws.String("REMOVE github_org_whitelist"),
		ReturnValues:     aws.String(dynamodb.ReturnValueNone),
	}

	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("error removing approval lists column %s for signature ID: %s, error: %v", columnName, signatureID, updateErr)
		return nil, updateErr
	}

	updatedSig, sigErr := repo.GetSignature(ctx, signatureID)
	if sigErr != nil {
		return nil, sigErr
	}

	return updatedSig, nil
}

func (repo repository) AddSigTypeSignedApprovedID(ctx context.Context, signatureID string, val string) error {
	f := logrus.Fields{
		"functionName":            "AddSigTypeSignedApprovedID",
		utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
		"signatureID":             signatureID,
		"sigtypeSignedApprovedID": val,
	}
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#signature_project_id_skey": aws.String("sigtype_signed_approved_id"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":val": {
				S: aws.String(val),
			},
		},
		UpdateExpression: aws.String("SET #signature_project_id_skey = :val"),
	}
	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("unable to update sigtype_signed_approved_id for signature_id: %s with input: %+v, error: %+v",
			signatureID, input, updateErr)
		return updateErr
	}
	return nil
}
func (repo repository) AddUsersDetails(ctx context.Context, signatureID string, userID string) error {
	f := logrus.Fields{
		"functionName":   "AddUserDetails",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
		"userID":         userID,
	}
	userModel, err := repo.usersRepo.GetUser(userID)
	if err != nil {
		return err
	}
	if userModel == nil {
		log.WithFields(f).Error("invalid user_id")
		return fmt.Errorf("invalid user id : %s for signature : %s", userID, signatureID)
	}
	var email string
	if userModel.LfEmail != "" {
		email = userModel.LfEmail
	} else {
		if len(userModel.Emails) > 0 {
			email = userModel.Emails[0]
		}
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
	}
	ue := utils.NewDynamoUpdateExpression()
	ue.AddAttributeName("#gh_username", "user_github_username", userModel.GithubUsername != "")
	ue.AddAttributeName("#lf_username", "user_lf_username", userModel.LfUsername != "")
	ue.AddAttributeName("#name", "user_name", userModel.Username != "")
	ue.AddAttributeName("#email", "user_email", email != "")

	ue.AddAttributeValue(":gh_username", &dynamodb.AttributeValue{S: aws.String(userModel.GithubUsername)}, userModel.GithubUsername != "")
	ue.AddAttributeValue(":lf_username", &dynamodb.AttributeValue{S: aws.String(userModel.LfUsername)}, userModel.LfUsername != "")
	ue.AddAttributeValue(":name", &dynamodb.AttributeValue{S: aws.String(userModel.Username)}, userModel.Username != "")
	ue.AddAttributeValue(":email", &dynamodb.AttributeValue{S: aws.String(email)}, email != "")

	ue.AddUpdateExpression("#gh_username = :gh_username", userModel.GithubUsername != "")
	ue.AddUpdateExpression("#lf_username = :lf_username", userModel.LfUsername != "")
	ue.AddUpdateExpression("#name = :name", userModel.Username != "")
	ue.AddUpdateExpression("#email = :email", email != "")
	if ue.Expression == "" {
		// nothing to update
		log.WithFields(f).Debug("no fields to update")
		return nil
	}
	input.UpdateExpression = aws.String(ue.Expression)
	input.ExpressionAttributeNames = ue.ExpressionAttributeNames
	input.ExpressionAttributeValues = ue.ExpressionAttributeValues
	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("unable to add users details to signature ID: %s with input: %+v, error = %s",
			signatureID, input, updateErr.Error())
		return updateErr
	}

	return nil
}

func (repo repository) AddSignedOn(ctx context.Context, signatureID string) error {
	f := logrus.Fields{
		"functionName":   "AddSignedOn",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
	}
	_, currentTime := utils.CurrentTime()
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#signed_on": aws.String("signed_on"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":current_time": {
				S: aws.String(currentTime),
			},
		},
		UpdateExpression: aws.String("SET #signed_on = :current_time"),
	}

	log.WithFields(f).Debug("updating signed on date...")
	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("unable to signed_on for signature ID: %s using update input: %+v, error = %s",
			signatureID, input, updateErr.Error())
		return updateErr
	}

	log.WithFields(f).Debug("successfully updated signed on date...")
	return nil
}

func (repo repository) GetClaGroupICLASignatures(ctx context.Context, claGroupID string, searchTerm *string, pageSize int64, nextKey string) (*models.IclaSignatures, error) {
	f := logrus.Fields{
		"functionName":   "GetClaGroupICLASignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"searchTerm":     utils.StringValue(searchTerm),
	}

	condition := expression.Key("signature_project_id").Equal(expression.Value(claGroupID))
	filter := expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)).
		And(expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser))).
		And(expression.Name("signature_approved").Equal(expression.Value(aws.Bool(true)))).
		And(expression.Name("signature_signed").Equal(expression.Value(aws.Bool(true)))).
		And(expression.Name("signature_user_ccla_company_id").AttributeNotExists())

	// Use the builder to create the expression
	expr, err := expression.NewBuilder().
		WithKeyCondition(condition).
		WithFilter(filter).
		WithProjection(buildProjection()).
		Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for get cla group icla signatures, claGroupID: %s, error: %v",
			claGroupID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String(SignatureProjectIDIndex),
	}

	if pageSize == 0 {
		pageSize = DefaultPageSize
	}

	if pageSize > BigPageSize {
		pageSize = BigPageSize
	}

	queryInput.Limit = &pageSize

	if searchTerm != nil {
		searchTerm = aws.String(strings.ToLower(*searchTerm))
	}

	// If we have the next key, set the exclusive start key value
	if nextKey != "" {
		log.WithFields(f).Debugf("Received a nextKey, value: %s", nextKey)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(nextKey),
			},
			"signature_project_id": {
				S: aws.String(claGroupID),
			},
		}
	}

	var intermediateResponse []*iclaSignatureWithDetails
	var lastEvaluatedKey string
	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving icla signatures for project: %s , error: %v",
				claGroupID, errQuery)
			return nil, errQuery
		}

		var dbSignatures []ItemSignature

		unmarshallError := dynamodbattribute.UnmarshalListOfMaps(results.Items, &dbSignatures)
		if unmarshallError != nil {
			log.WithFields(f).Warnf("error unmarshalling icla signatures from database for cla group: %s, error: %v",
				claGroupID, unmarshallError)
			return nil, unmarshallError
		}

		intermediateResponse = append(intermediateResponse, repo.getIntermediateICLAResponse(f, dbSignatures, searchTerm)...)

		log.WithFields(f).Debugf("LastEvaluatedKey: %+v", results.LastEvaluatedKey["signature_id"])
		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			lastEvaluatedKey = ""
		}

		if int64(len(intermediateResponse)) >= pageSize {
			break
		}
	}

	if int64(len(intermediateResponse)) > pageSize {
		intermediateResponse = intermediateResponse[0:pageSize]
		lastEvaluatedKey = intermediateResponse[pageSize-1].IclaSignature.SignatureID
	}

	// Append all the responses to our list
	out := &models.IclaSignatures{
		LastKeyScanned: lastEvaluatedKey,
		PageSize:       pageSize,
		ResultCount:    int64(len(intermediateResponse)),
	}

	iclaSignatures, err := repo.addAdditionalICLAMetaData(f, intermediateResponse)
	if err != nil {
		return nil, err
	}

	out.List = iclaSignatures
	return out, nil
}

func (repo repository) getIntermediateICLAResponse(f logrus.Fields, dbSignatures []ItemSignature, searchTerm *string) []*iclaSignatureWithDetails {
	var intermediateResponse []*iclaSignatureWithDetails

	for _, sig := range dbSignatures {
		if searchTerm != nil {
			if !strings.Contains(sig.SignatureReferenceNameLower, *searchTerm) {
				continue
			}
		}

		// Set the signed date/time
		var sigSignedTime string
		// Use the user docusign date signed value if it is present - older signatures do not have this
		if sig.UserDocusignDateSigned != "" {
			// Put the date into a standard format
			t, err := utils.ParseDateTime(sig.UserDocusignDateSigned)
			if err != nil {
				log.WithFields(f).WithError(err).Warn("unable to parse signature docusign date signed time")
			} else {
				sigSignedTime = utils.TimeToString(t)
			}
		} else {
			// Put the date into a standard format
			t, err := utils.ParseDateTime(sig.DateCreated)
			if err != nil {
				log.WithFields(f).WithError(err).Warn("unable to parse signature date created time")
			} else {
				sigSignedTime = utils.TimeToString(t)
			}
		}

		intermediateResponse = append(intermediateResponse, &iclaSignatureWithDetails{
			IclaSignature: &models.IclaSignature{
				GithubUsername:         sig.UserGithubUsername,
				LfUsername:             sig.UserLFUsername,
				SignatureID:            sig.SignatureID,
				UserEmail:              sig.UserEmail,
				UserName:               sig.UserName,
				SignedOn:               sigSignedTime,
				UserDocusignName:       sig.UserDocusignName,
				UserDocusignDateSigned: sigSignedTime,
				SignatureModified:      sig.DateModified,
			},
			SignatureReferenceID: sig.SignatureReferenceID,
		})
	}

	return intermediateResponse
}

func (repo repository) addAdditionalICLAMetaData(f logrus.Fields, intermediateResponse []*iclaSignatureWithDetails) ([]*models.IclaSignature, error) {
	log.WithFields(f).Debugf("Adding additional meta-data for %d records...", len(intermediateResponse))
	// For some older ICLA signatures, we are missing the user's info, but we have their internal ID - let's look up those values before returning
	responseChannel := make(chan *models.IclaSignature)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	for _, iclaDetails := range intermediateResponse {
		go func(iclaSignatureWithDetails *iclaSignatureWithDetails) {
			userModel, userLookupErr := repo.usersRepo.GetUser(iclaSignatureWithDetails.SignatureReferenceID)
			if userLookupErr != nil || userModel == nil {
				log.WithFields(f).WithError(userLookupErr).Warnf("unable to lookup user with id: %s", iclaSignatureWithDetails.SignatureReferenceID)
			} else {
				// If the github username is empty, see if it was set in the user model
				if iclaSignatureWithDetails.IclaSignature.GithubUsername == "" {
					// Grab and set the github username
					iclaSignatureWithDetails.IclaSignature.GithubUsername = userModel.GithubUsername
				}
				// If the github username is empty, see if it was set in the user model
				if iclaSignatureWithDetails.IclaSignature.UserName == "" {
					if userModel.Username != "" {
						// Grab and set the github username
						iclaSignatureWithDetails.IclaSignature.UserName = userModel.Username
					} else if userModel.LfUsername != "" {
						iclaSignatureWithDetails.IclaSignature.UserName = userModel.LfUsername
					}
				}
				// If the github username is empty, see if it was set in the user model
				if iclaSignatureWithDetails.IclaSignature.UserEmail == "" {
					// Grab and set the github username
					iclaSignatureWithDetails.IclaSignature.UserEmail = getBestEmail(userModel)
				}
			}

			responseChannel <- iclaSignatureWithDetails.IclaSignature
		}(iclaDetails)
	}

	var finalResults []*models.IclaSignature
	for i := 0; i < len(intermediateResponse); i++ {
		select {
		case result := <-responseChannel:
			finalResults = append(finalResults, result)
		case <-ctx.Done():
			log.WithError(ctx.Err()).Warnf("timeout during adding additional meta to icla signatures")
			return nil, ctx.Err()
		}
	}

	return finalResults, nil
}

func (repo repository) GetClaGroupCorporateContributors(ctx context.Context, claGroupID string, companyID *string, searchTerm *string) (*models.CorporateContributorList, error) {
	f := logrus.Fields{
		"functionName":   "GetClaGroupCorporateContributors",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"companyID":      aws.StringValue(companyID),
	}

	condition := expression.Key("signature_project_id").Equal(expression.Value(claGroupID))
	if companyID != nil {
		sortKey := fmt.Sprintf("%s#%v#%v#%v", utils.ClaTypeECLA, true, true, *companyID)
		condition = condition.And(expression.Key("sigtype_signed_approved_id").Equal(expression.Value(sortKey)))
	} else {
		sortKeyPrefix := fmt.Sprintf("%s#%v#%v", utils.ClaTypeECLA, true, true)
		condition = condition.And(expression.Key("sigtype_signed_approved_id").BeginsWith(sortKeyPrefix))
	}

	// Use the builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for get cla group icla signatures, claGroupID: %s, error: %v",
			claGroupID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String(SignatureProjectIDSigTypeSignedApprovedIDIndex),
		Limit:                     aws.Int64(HugePageSize),
	}

	out := &models.CorporateContributorList{List: make([]*models.CorporateContributor, 0)}
	if searchTerm != nil {
		searchTerm = aws.String(strings.ToLower(*searchTerm))
	}

	for {
		// Make the DynamoDB Query API call
		log.WithFields(f).Debug("querying signatures...")
		results, queryErr := repo.dynamoDBClient.Query(queryInput)
		if queryErr != nil {
			log.WithFields(f).Warnf("error retrieving icla signatures for project: %s, error: %v", claGroupID, queryErr)
			return nil, queryErr
		}

		var dbSignatures []ItemSignature

		err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &dbSignatures)
		if err != nil {
			log.WithFields(f).Warnf("error unmarshalling icla signatures from database for cla group: %s, error: %v",
				claGroupID, err)
			return nil, err
		}

		log.WithFields(f).Debugf("located %d signatures...", len(dbSignatures))
		for _, sig := range dbSignatures {
			if searchTerm != nil {
				if !strings.Contains(sig.SignatureReferenceNameLower, *searchTerm) {
					continue
				}
			}

			var sigCreatedTime = sig.DateCreated
			t, err := utils.ParseDateTime(sig.DateCreated)
			if err != nil {
				log.WithFields(f).WithError(err).Warn("unable to parse signature date created time")
			} else {
				sigCreatedTime = utils.TimeToString(t)
			}

			// Set the signed date/time
			var sigSignedTime string
			// Use the user docusign date signed value if it is present - older signatures do not have this
			if sig.UserDocusignDateSigned != "" {
				// Put the date into a standard format
				t, err = utils.ParseDateTime(sig.UserDocusignDateSigned)
				if err != nil {
					log.WithFields(f).WithError(err).Warn("unable to parse signature docusign date signed time")
				} else {
					sigSignedTime = utils.TimeToString(t)
				}
			} else {
				// Put the date into a standard format
				t, err = utils.ParseDateTime(sig.DateCreated)
				if err != nil {
					log.WithFields(f).WithError(err).Warn("unable to parse signature date created time")
				} else {
					sigSignedTime = utils.TimeToString(t)
				}
			}

			signatureVersion := fmt.Sprintf("v%s.%s", sig.SignatureDocumentMajorVersion, sig.SignatureDocumentMinorVersion)
			out.List = append(out.List, &models.CorporateContributor{
				SignatureID:            sig.SignatureID,
				GithubID:               sig.UserGithubUsername,
				LinuxFoundationID:      sig.UserLFUsername,
				Name:                   sig.UserName,
				SignatureVersion:       signatureVersion,
				Email:                  sig.UserEmail,
				Timestamp:              sigCreatedTime,
				UserDocusignName:       sig.UserDocusignName,
				UserDocusignDateSigned: sigSignedTime,
				SignatureModified:      sig.DateModified,
			})
		}

		if len(results.LastEvaluatedKey) == 0 {
			break
		}
		queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		log.WithFields(f).Debug("querying next page")
	}
	sort.Slice(out.List, func(i, j int) bool {
		return out.List[i].Name < out.List[j].Name
	})

	return out, nil
}
