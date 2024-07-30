// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid"

	"github.com/communitybridge/easycla/cla-backend-go/config"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/users"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gerrits"
	"github.com/communitybridge/easycla/cla-backend-go/github"
	"github.com/communitybridge/easycla/cla-backend-go/github_organizations"
	"github.com/communitybridge/easycla/cla-backend-go/repositories"
	"github.com/communitybridge/easycla/cla-backend-go/v2/approvals"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
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
	DefaultPageSize = 100
	BigPageSize     = 500
)

// SignatureRepository interface defines the functions for the the signature repository
type SignatureRepository interface {
	GetGithubOrganizationsFromApprovalList(ctx context.Context, signatureID string) ([]models.GithubOrg, error)
	AddGithubOrganizationToApprovalList(ctx context.Context, signatureID, githubOrganizationID string) ([]models.GithubOrg, error)
	DeleteGithubOrganizationFromApprovalList(ctx context.Context, signatureID, githubOrganizationID string) ([]models.GithubOrg, error)
	ValidateProjectRecord(ctx context.Context, signatureID, note string) error
	InvalidateProjectRecord(ctx context.Context, signatureID, note string) error
	UpdateEnvelopeDetails(ctx context.Context, signatureID, envelopeID string, signURL *string) (*models.Signature, error)
	CreateSignature(ctx context.Context, signature *ItemSignature) error
	UpdateSignature(ctx context.Context, signatureID string, updates map[string]interface{}) error
	SaveOrUpdateSignature(ctx context.Context, signature *ItemSignature) error

	GetSignature(ctx context.Context, signatureID string) (*models.Signature, error)
	GetItemSignature(ctx context.Context, signatureID string) (*ItemSignature, error)
	GetActivePullRequestMetadata(ctx context.Context, gitHubAuthorUsername, gitHubAuthorEmail string) (*ActivePullRequest, error)
	GetIndividualSignature(ctx context.Context, claGroupID, userID string, approved, signed *bool) (*models.Signature, error)
	GetIndividualSignatures(ctx context.Context, claGroupID, userID string, approved, signed *bool) ([]*models.Signature, error)
	GetCorporateSignature(ctx context.Context, claGroupID, companyID string, approved, signed *bool) (*models.Signature, error)
	GetCorporateSignatures(ctx context.Context, claGroupID, companyID string, approved, signed *bool) ([]*models.Signature, error)
	GetCCLASignatures(ctx context.Context, signed, approved *bool) ([]*ItemSignature, error)
	GetSignatureACL(ctx context.Context, signatureID string) ([]string, error)
	GetProjectSignatures(ctx context.Context, params signatures.GetProjectSignaturesParams) (*models.Signatures, error)
	CreateProjectSummaryReport(ctx context.Context, params signatures.CreateProjectSummaryReportParams) (*models.SignatureReport, error)
	GetProjectCompanySignature(ctx context.Context, companyID, projectID string, approved, signed *bool, nextKey *string, pageSize *int64) (*models.Signature, error)
	GetProjectCompanySignatures(ctx context.Context, companyID, projectID string, approved, signed *bool, nextKey *string, sortOrder *string, pageSize *int64) (*models.Signatures, error)
	GetProjectCompanyEmployeeSignatures(ctx context.Context, params signatures.GetProjectCompanyEmployeeSignaturesParams, criteria *ApprovalCriteria) (*models.Signatures, error)
	GetProjectCompanyEmployeeSignature(ctx context.Context, companyModel *models.Company, claGroupModel *models.ClaGroup, employeeUserModel *models.User, wg *sync.WaitGroup, resultChannel chan<- *EmployeeModel, errorChannel chan<- error)
	CreateProjectCompanyEmployeeSignature(ctx context.Context, companyModel *models.Company, claGroupModel *models.ClaGroup, employeeUserModel *models.User) error
	GetCompanySignatures(ctx context.Context, params signatures.GetCompanySignaturesParams, pageSize int64, loadACL bool) (*models.Signatures, error)
	GetCompanyIDsWithSignedCorporateSignatures(ctx context.Context, claGroupID string) ([]SignatureCompanyID, error)
	GetUserSignatures(ctx context.Context, params signatures.GetUserSignaturesParams, pageSize int64, projectID *string) (*models.Signatures, error)
	ProjectSignatures(ctx context.Context, projectID string) (*models.Signatures, error)
	UpdateApprovalList(ctx context.Context, claManager *models.User, claGroupModel *models.ClaGroup, companyID string, params *models.ApprovalList, eventArgs *events.LogEventArgs) (*models.Signature, error)
	AddCLAManager(ctx context.Context, signatureID, claManagerID string) (*models.Signature, error)
	RemoveCLAManager(ctx context.Context, signatureID, claManagerID string) (*models.Signature, error)
	AddSigTypeSignedApprovedID(ctx context.Context, signatureID string, val string) error
	AddUsersDetails(ctx context.Context, signatureID string, userID string) error
	AddSignedOn(ctx context.Context, signatureID string) error
	GetClaGroupICLASignatures(ctx context.Context, claGroupID string, searchTerm *string, approved, signed *bool, pageSize int64, nextKey string, withExtraDetails bool) (*models.IclaSignatures, error)
	GetClaGroupCorporateContributors(ctx context.Context, claGroupID string, companyID *string, pageSize *int64, nextKey *string, searchTerm *string) (*models.CorporateContributorList, error)
	EclaAutoCreate(ctx context.Context, signatureID string, autoCreateECLA bool) error
	ActivateSignature(ctx context.Context, signatureID string) error
	GetICLAByDate(ctx context.Context, startDate string) ([]ItemSignature, error)
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
	repositoriesRepo   repositories.RepositoryInterface
	ghOrgRepo          github_organizations.RepositoryInterface
	gerritService      gerrits.Service
	signatureTableName string
	approvalRepo       approvals.IRepository
}

// NewRepository creates a new instance of the signature repository service
func NewRepository(awsSession *session.Session, stage string, companyRepo company.IRepository, usersRepo users.UserRepository, eventsService events.Service, repositoriesRepo repositories.RepositoryInterface, ghOrgRepo github_organizations.RepositoryInterface, gerritService gerrits.Service, approvalRepo approvals.IRepository) SignatureRepository {
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
		approvalRepo:       approvalRepo,
	}
}

// CreateIndividualSignature creates a new individual signature
func (repo repository) CreateSignature(ctx context.Context, signature *ItemSignature) error {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.CreateIndividualSignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	av, err := dynamodbattribute.MarshalMap(signature)
	if err != nil {
		log.WithFields(f).Warnf("error marshalling signature, error: %v", err)
		return err
	}

	// Add the signature to the database
	_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(repo.signatureTableName),
	})

	if err != nil {
		log.WithFields(f).Warnf("error adding signature to database, error: %v", err)
		return err
	}

	log.WithFields(f).Debugf("successfully added signature to database")

	return nil

}

// GetItemSignature returns the signature for the specified signature id
func (repo repository) GetItemSignature(ctx context.Context, signatureID string) (*ItemSignature, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetItemSignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
	}

	// This is the key we want to match
	condition := expression.Key("signature_id").Equal(expression.Value(signatureID))

	// Use the builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for signature ID query, signatureID: %s, error: %v", signatureID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
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

	var signature ItemSignature
	err = dynamodbattribute.UnmarshalMap(results.Items[0], &signature)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling signature for ID: %s, error: %v", signatureID, err)
		return nil, err
	}

	return &signature, nil
}

// SaveOrUpdateSignature either creates or updates the signature record
func (repo repository) SaveOrUpdateSignature(ctx context.Context, signature *ItemSignature) error {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.SaveOrUpdateSignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	av, err := dynamodbattribute.MarshalMap(signature)

	if err != nil {
		log.WithFields(f).Warnf("error marshalling signature, error: %v", err)
		return err
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(repo.signatureTableName),
	}

	_, err = repo.dynamoDBClient.PutItem(input)
	if err != nil {
		log.WithFields(f).Warnf("error adding signature to database, error: %v", err)
		return err
	}

	log.WithFields(f).Debugf("successfully added/updated  signature to database")

	return nil
}

// GetCCCLASignatures returns a list of CCLA signatures
func (repo repository) GetCCLASignatures(ctx context.Context, signed, approved *bool) ([]*ItemSignature, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetCCLASignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signed":         signed,
		"approved":       approved,
	}

	var filter expression.ConditionBuilder
	pageSize := 1000

	filter = expression.Name("signature_type").Equal(expression.Value("ccla"))
	if signed != nil {
		filter = filter.And(expression.Name("signature_signed").Equal(expression.Value(signed)))
	}
	if approved != nil {
		filter = filter.And(expression.Name("signature_approved").Equal(expression.Value(approved)))
	}

	// Use the expression builder to build the expression
	expr, err := expression.NewBuilder().WithFilter(filter).Build()

	if err != nil {
		log.WithFields(f).Warnf("error building expression for CCLA signatures query, error: %v", err)
		return nil, err
	}

	// Make the DynamoDB Query API call
	input := &dynamodb.ScanInput{
		TableName:                 aws.String(repo.signatureTableName),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int64(int64(pageSize)),
	}

	var signatures []*ItemSignature
	var lastEvaluatedKey map[string]*dynamodb.AttributeValue

	for {
		results, queryErr := repo.dynamoDBClient.Scan(input)
		if queryErr != nil {
			log.WithFields(f).Warnf("error retrieving CCLA signatures, error: %v", queryErr)
			return nil, queryErr
		}

		var items []*ItemSignature
		err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &items)
		if err != nil {
			log.WithFields(f).Warnf("error unmarshalling CCLA signatures from database, error: %v", err)
			return nil, err
		}

		signatures = append(signatures, items...)

		// If the result set is truncated, we'll need to issue another query to fetch the next page
		if results.LastEvaluatedKey == nil {
			break
		}

		lastEvaluatedKey = results.LastEvaluatedKey
		input.ExclusiveStartKey = lastEvaluatedKey
	}

	return signatures, nil

}

// UpdateSignature updates an existing signature
func (repo repository) UpdateSignature(ctx context.Context, signatureID string, updates map[string]interface{}) error {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.UpdateSignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
	}

	if len(updates) == 0 {
		log.WithFields(f).Warnf("no updates provided")
		return errors.New("no updates provided")
	}

	var updateExpression strings.Builder
	updateExpression.WriteString("SET ")
	attributeValues := make(map[string]*dynamodb.AttributeValue)
	expressionAttributeNames := make(map[string]*string)

	count := 1
	for attr, val := range updates {
		attrPlaceholder := fmt.Sprintf("#A%d", count)
		valPlaceholder := fmt.Sprintf(":v%d", count)

		if count > 1 && count <= len(updates) {
			updateExpression.WriteString(", ")
		}
		updateExpression.WriteString(fmt.Sprintf("%s = %s", attrPlaceholder, valPlaceholder))

		expressionAttributeNames[attrPlaceholder] = aws.String(attr)
		av, err := dynamodbattribute.Marshal(val)
		if err != nil {
			return err
		}
		attributeValues[valPlaceholder] = av

		count++
	}

	log.WithFields(f).Debugf("updating signature using expression: %s", updateExpression.String())
	log.WithFields(f).Debugf("expression attribute names : %+v", expressionAttributeNames)

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeValues: attributeValues,
		TableName:                 aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
		UpdateExpression:         aws.String(updateExpression.String()),
		ExpressionAttributeNames: expressionAttributeNames,
		ReturnValues:             aws.String("UPDATED_NEW"),
	}

	// perform the update
	_, err := repo.dynamoDBClient.UpdateItem(input)
	if err != nil {
		log.WithFields(f).Warnf("error updating signature, error: %v", err)
		return err
	}

	log.WithFields(f).Debugf("successfully updated signature")

	return nil

}

// GetGithubOrganizationsFromApprovalList returns a list of GH organizations stored in the approval list
func (repo repository) GetGithubOrganizationsFromApprovalList(ctx context.Context, signatureID string) ([]models.GithubOrg, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetGithubOrganizationsFromApprovalList",
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
		log.WithFields(f).Warnf("Error retrieving GH organization approval list for signatureID: %s, error: %v", signatureID, err)
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

// AddGithubOrganizationToApprovalList adds the specified GH organization to the approval list
func (repo repository) AddGithubOrganizationToApprovalList(ctx context.Context, signatureID, GitHubOrganizationID string) ([]models.GithubOrg, error) {
	f := logrus.Fields{
		"functionName":         "v1.signatures.repository.AddGithubOrganizationToApprovalList",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"signatureID":          signatureID,
		"GitHubOrganizationID": GitHubOrganizationID,
	}
	// get item from dynamoDB table
	log.WithFields(f).Debugf("querying database for GitHub organization approval list using signatureID: %s", signatureID)

	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
	})

	if err != nil {
		log.WithFields(f).Warnf("Error retrieving GitHub organization approval list for signatureID: %s and GH Org: %s, error: %v",
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
		msg := fmt.Sprintf("unable to fetch updated github organization approval list values for "+
			"organization id: %s for signature: %s - list is empty - returning empty list",
			GitHubOrganizationID, signatureID)
		log.WithFields(f).Debugf(msg)
		return []models.GithubOrg{}, nil
	}

	return buildResponse(updatedItemFromMap.L), nil
}

// DeleteGithubOrganizationFromApprovalList removes the specified GH organization from the approval list
func (repo repository) DeleteGithubOrganizationFromApprovalList(ctx context.Context, signatureID, GitHubOrganizationID string) ([]models.GithubOrg, error) {
	f := logrus.Fields{
		"functionName":         "v1.signatures.repository.DeleteGithubOrganizationFromApprovalList",
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
		log.WithFields(f).Warnf("error retrieving GH organization approval list for signatureID: %s and GH Org: %s, error: %v",
			signatureID, GitHubOrganizationID, err)
		return nil, err
	}

	itemFromMap, ok := result.Item["github_org_whitelist"]
	if !ok {
		log.WithFields(f).Warnf("unable to remove github organization approval list entry: %s for signature: %s - list is empty",
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

		log.WithFields(f).Debugf("clearing out github org approval list for organization: %s for signature: %s - list is empty",
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
			log.WithFields(f).Warnf("error updating github org approva list to NULL value, error: %v", err)
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
		log.WithFields(f).Warnf("Error updating github org approva list, error: %v", err)
		return nil, err
	}

	updatedItemFromMap, ok := updatedValues.Attributes["github_org_whitelist"]
	if !ok {
		msg := fmt.Sprintf("unable to fetch updated approva list organization values for "+
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
		"functionName":   "v1.signatures.repository.GetSignature",
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

// CreateOrUpdateSignature either creates or updates the signature record
func (repo repository) UpdateEnvelopeDetails(ctx context.Context, signatureID, envelopeID string, signURL *string) (*models.Signature, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.UpdateEnvelopeDetails",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
		"envelopeID":     envelopeID,
	}

	log.WithFields(f).Debugf("setting envelope details....")

	updateExpression := "SET signature_envelope_id = :envelopeId "
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{
		":envelopeId": {
			S: aws.String(envelopeID),
		},
	}

	if signURL != nil {
		updateExpression += ",signature_sign_url = :signUrl "
		expressionAttributeValues[":signUrl"] = &dynamodb.AttributeValue{
			S: aws.String(*signURL),
		}
	}

	// Create the update input
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeValues: expressionAttributeValues,
		ReturnValues:              aws.String("ALL_NEW"),
	}

	// Update the record in the DynamoDB table
	result, err := repo.dynamoDBClient.UpdateItem(input)
	if err != nil {
		log.WithFields(f).Errorf("Error updating signature record: %v", err)
		return nil, err
	}

	// Update the record in the DynamoDB table
	var updatedItem ItemSignature

	if err := dynamodbattribute.UnmarshalMap(result.Attributes, &updatedItem); err != nil {
		log.WithFields(f).Errorf("Error unmarshalling updated item: %v", err)
		return nil, err
	}

	log.WithFields(f).Debugf("updated signature record for: %s", signatureID)
	return &models.Signature{
		SignatureID:          updatedItem.SignatureID,
		SignatureSignURL:     updatedItem.SignatureSignURL,
		ProjectID:            updatedItem.SignatureProjectID,
		SignatureReferenceID: updatedItem.SignatureReferenceID,
	}, nil
}

// GetIndividualSignature returns the signature record for the specified CLA Group and User
func (repo repository) GetIndividualSignature(ctx context.Context, claGroupID, userID string, approved, signed *bool) (*models.Signature, error) {
	f := logrus.Fields{
		"functionName":           "v1.signatures.repository.GetIndividualSignature",
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

	var filterAdded bool
	// These are the keys we want to match for an ICLA Signature with a given CLA Group and User ID
	condition := expression.Key("signature_project_id").Equal(expression.Value(claGroupID)).
		And(expression.Key("signature_reference_id").Equal(expression.Value(userID)))
	var filter expression.ConditionBuilder
	filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeNotExists(), &filterAdded)

	if approved != nil {
		filterAdded = true
		searchTermExpression := expression.Name("signature_approved").Equal(expression.Value(aws.BoolValue(approved)))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}
	if signed != nil {
		filterAdded = true
		log.WithFields(f).Debugf("adding filter signature_signed: %t", aws.BoolValue(signed))
		searchTermExpression := expression.Name("signature_signed").Equal(expression.Value(aws.BoolValue(signed)))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}

	// If no query option was provided for approved and signed and our configuration default is to only show active signatures then we add the required query filters
	if approved == nil && signed == nil && config.GetConfig().SignatureQueryDefault == utils.SignatureQueryDefaultActive {
		filterAdded = true
		// log.WithFields(f).Debug("adding filter signature_approved: true and signature_signed: true")
		filter = addAndCondition(filter, expression.Name("signature_approved").Equal(expression.Value(true)), &filterAdded)
		filter = addAndCondition(filter, expression.Name("signature_signed").Equal(expression.Value(true)), &filterAdded)
	}

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

	return sigs[0], nil // nolint G602: Potentially accessing slice out of bounds (gosec)
}

// GetIndividualSignature returns the signature record for the specified CLA Group and User
func (repo repository) GetIndividualSignatures(ctx context.Context, claGroupID, userID string, approved, signed *bool) ([]*models.Signature, error) {
	f := logrus.Fields{
		"functionName":           "v1.signatures.repository.GetIndividualSignature",
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

	var filterAdded bool
	// These are the keys we want to match for an ICLA Signature with a given CLA Group and User ID
	condition := expression.Key("signature_project_id").Equal(expression.Value(claGroupID)).
		And(expression.Key("signature_reference_id").Equal(expression.Value(userID)))
	var filter expression.ConditionBuilder
	filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeNotExists(), &filterAdded)

	if approved != nil {
		filterAdded = true
		searchTermExpression := expression.Name("signature_approved").Equal(expression.Value(aws.BoolValue(approved)))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}
	if signed != nil {
		filterAdded = true
		log.WithFields(f).Debugf("adding filter signature_signed: %t", aws.BoolValue(signed))
		searchTermExpression := expression.Name("signature_signed").Equal(expression.Value(aws.BoolValue(signed)))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}

	// If no query option was provided for approved and signed and our configuration default is to only show active signatures then we add the required query filters
	if approved == nil && signed == nil && config.GetConfig().SignatureQueryDefault == utils.SignatureQueryDefaultActive {
		filterAdded = true
		// log.WithFields(f).Debug("adding filter signature_approved: true and signature_signed: true")
		filter = addAndCondition(filter, expression.Name("signature_approved").Equal(expression.Value(true)), &filterAdded)
		filter = addAndCondition(filter, expression.Name("signature_signed").Equal(expression.Value(true)), &filterAdded)
	}

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

	return sigs, nil
}

// GetCorporateSignature returns the signature record for the specified CLA Group and Company ID
func (repo repository) GetCorporateSignature(ctx context.Context, claGroupID, companyID string, approved, signed *bool) (*models.Signature, error) {
	f := logrus.Fields{
		"functionName":           "v1.signatures.repository.GetCorporateSignature",
		utils.XREQUESTID:         ctx.Value(utils.XREQUESTID),
		"tableName":              repo.signatureTableName,
		"claGroupID":             claGroupID,
		"companyID":              companyID,
		"signatureType":          "ccla",
		"signatureReferenceType": "company",
		"signatureApproved":      utils.BoolValue(approved),
		"signatureSigned":        utils.BoolValue(signed),
	}

	var filterAdded bool
	// These are the keys we want to match for an CCLA Signature with a given CLA Group and Company ID
	condition := expression.Key("signature_project_id").Equal(expression.Value(claGroupID)).
		And(expression.Key("signature_reference_id").Equal(expression.Value(companyID)))
	var filter expression.ConditionBuilder
	filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCCLA)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeCompany)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeNotExists(), &filterAdded)

	if approved != nil {
		filterAdded = true
		filter = addAndCondition(filter, expression.Name("signature_approved").Equal(expression.Value(aws.BoolValue(approved))), &filterAdded)
	}
	if signed != nil {
		filterAdded = true
		filter = addAndCondition(filter, expression.Name("signature_signed").Equal(expression.Value(aws.BoolValue(signed))), &filterAdded)
	}

	// If no query option was provided for approved and signed and our configuration default is to only show active signatures then we add the required query filters
	if approved == nil && signed == nil && config.GetConfig().SignatureQueryDefault == utils.SignatureQueryDefaultActive {
		filterAdded = true
		//log.WithFields(f).Debug("adding filter signature_approved: true and signature_signed: true")
		filter = addAndCondition(filter, expression.Name("signature_approved").Equal(expression.Value(true)), &filterAdded)
		filter = addAndCondition(filter, expression.Name("signature_signed").Equal(expression.Value(true)), &filterAdded)
	}

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

	return sigs[0], nil // nolint G602: Potentially accessing slice out of bounds (gosec)
}

// GetCorporateSignatures returns the list signature record for the specified CLA Group and Company ID
func (repo repository) GetCorporateSignatures(ctx context.Context, claGroupID, companyID string, approved, signed *bool) ([]*models.Signature, error) {
	f := logrus.Fields{
		"functionName":           "v1.signatures.repository.GetCorporateSignatures",
		utils.XREQUESTID:         ctx.Value(utils.XREQUESTID),
		"tableName":              repo.signatureTableName,
		"claGroupID":             claGroupID,
		"companyID":              companyID,
		"signatureType":          "ccla",
		"signatureReferenceType": "company",
		"signatureApproved":      utils.BoolValue(approved),
		"signatureSigned":        utils.BoolValue(signed),
	}

	var filterAdded bool
	// These are the keys we want to match for an CCLA Signature with a given CLA Group and Company ID
	condition := expression.Key("signature_project_id").Equal(expression.Value(claGroupID)).
		And(expression.Key("signature_reference_id").Equal(expression.Value(companyID)))
	var filter expression.ConditionBuilder
	filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCCLA)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeCompany)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeNotExists(), &filterAdded)

	if approved != nil {
		filterAdded = true
		filter = addAndCondition(filter, expression.Name("signature_approved").Equal(expression.Value(aws.BoolValue(approved))), &filterAdded)
	}
	if signed != nil {
		filterAdded = true
		filter = addAndCondition(filter, expression.Name("signature_signed").Equal(expression.Value(aws.BoolValue(signed))), &filterAdded)
	}

	// If no query option was provided for approved and signed and our configuration default is to only show active signatures then we add the required query filters
	if approved == nil && signed == nil && config.GetConfig().SignatureQueryDefault == utils.SignatureQueryDefaultActive {
		filterAdded = true
		//log.WithFields(f).Debug("adding filter signature_approved: true and signature_signed: true")
		filter = addAndCondition(filter, expression.Name("signature_approved").Equal(expression.Value(true)), &filterAdded)
		filter = addAndCondition(filter, expression.Name("signature_signed").Equal(expression.Value(true)), &filterAdded)
	}

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

	return sigs, nil
}

// GetActivePullRequestMetadata returns the pull request metadata for the given user ID
func (repo repository) GetActivePullRequestMetadata(ctx context.Context, gitHubAuthorUsername, gitHubAuthorEmail string) (*ActivePullRequest, error) {
	f := logrus.Fields{
		"functionName":         "v1.signatures.repository.GetActivePullRequestMetadata",
		utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
		"gitHubAuthorUsername": gitHubAuthorUsername,
		"gitHubAuthorEmail":    gitHubAuthorEmail,
	}

	if gitHubAuthorUsername == "" && gitHubAuthorEmail == "" {
		return nil, nil
	}

	expr, err := expression.NewBuilder().WithProjection(buildSignatureMetadata()).Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error building expression for user ID query")
		return nil, err
	}

	// Try to lookup based on the following keys - could be indexed by either or both (depends if user shared their
	// email and went through the GitHub authorization flow)
	var keys []string
	if gitHubAuthorUsername != "" {
		keys = append(keys, fmt.Sprintf("active_pr:u:%s", gitHubAuthorUsername))
	}
	if gitHubAuthorEmail != "" {
		keys = append(keys, fmt.Sprintf("active_pr:e:%s", gitHubAuthorEmail))
	}

	var activeSignature ActivePullRequest
	for _, key := range keys {
		itemInput := &dynamodb.GetItemInput{
			Key: map[string]*dynamodb.AttributeValue{
				"key": {S: aws.String(key)},
			},
			ExpressionAttributeNames: expr.Names(),
			ProjectionExpression:     expr.Projection(),
			TableName:                aws.String(fmt.Sprintf("cla-%s-store", repo.stage)),
		}

		// Make the DynamoDb Query API call
		// log.WithFields(f).Debugf("loading active signature using key: %s", key)
		result, queryErr := repo.dynamoDBClient.GetItem(itemInput)
		if queryErr != nil {
			if queryErr.Error() == dynamodb.ErrCodeResourceNotFoundException {
				continue
			}
			log.WithFields(f).WithError(queryErr).Warnf("error retrieving active signature metadata using key: %s", key)
			return nil, queryErr
		}

		if result == nil || result.Item == nil || result.Item["value"] == nil || result.Item["value"].S == nil {
			log.WithFields(f).Debugf("query result is empty for key: %s", key)
			continue
		}
		if result.Item["value"] == nil || result.Item["value"].S == nil {
			log.WithFields(f).Debugf("query result value is empty for key: %s", key)
			continue
		}

		// Clean up the JSON string
		strValue := utils.StringValue(result.Item["value"].S)
		// log.WithFields(f).Debugf("decoding value: %s", strValue)
		if strings.HasSuffix(strValue, "\"") {
			// Trim the leading and trailing quotes from the JSON record
			strValue = strValue[1 : len(strValue)-1]
		}
		// Unescape the JSON string
		strValue = strings.Replace(strValue, "\\\"", "\"", -1)
		// log.WithFields(f).Debugf("decoding value: %s", strValue)

		jsonUnMarshallErr := json.Unmarshal([]byte(strValue), &activeSignature)
		if jsonUnMarshallErr != nil {
			log.WithFields(f).WithError(jsonUnMarshallErr).Warn("unable to convert model for active signature ")
			return nil, jsonUnMarshallErr
		}

		return &activeSignature, nil
	}

	return nil, nil
}

// GetSignatureACL returns the signature ACL for the specified signature id
func (repo repository) GetSignatureACL(ctx context.Context, signatureID string) ([]string, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetSignatureACL",
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

// Adds the specified expression to the current filter using the And operator. This routine checks the filter added flag
// to determine if a previous filter was set.  After this function executes, the filterAdded value will be set to true.
func addAndCondition(filter expression.ConditionBuilder, cond expression.ConditionBuilder, filterAdded *bool) expression.ConditionBuilder {
	if *filterAdded {
		return filter.And(cond)
	}
	*filterAdded = true
	return cond
}

// GetProjectSignatures returns a list of signatures for the specified project
func (repo repository) GetProjectSignatures(ctx context.Context, params signatures.GetProjectSignaturesParams) (*models.Signatures, error) { // nolint
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetProjectSignatures",
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

	// Always sort by date
	indexName := SignatureProjectDateIDIndex

	realPageSize := int64(1000)
	if params.PageSize != nil && *params.PageSize > 0 {
		realPageSize = *params.PageSize
	}

	// This is the key we want to match
	condition := expression.Key("signature_project_id").Equal(expression.Value(params.ProjectID))
	builder := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection())

	var filter expression.ConditionBuilder
	var filterAdded = false

	if params.ClaType != nil || params.SignatureType != nil {
		switch getCLATypeFromParams(params) {
		case utils.ClaTypeICLA:
			log.WithFields(f).Debugf("adding ICLA filters: signature_type: %s, signature_reference_type: %s, signature_user_ccla_company_id: not exists", utils.SignatureTypeCLA, utils.SignatureReferenceTypeUser)
			filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeNotExists(), &filterAdded)
		case utils.ClaTypeECLA:
			log.WithFields(f).Debugf("adding ECLA filters: signature_type: %s, signature_reference_type: %s, signature_user_ccla_company_id: exists", utils.SignatureTypeCLA, utils.SignatureReferenceTypeUser)
			filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeExists(), &filterAdded)
		case utils.ClaTypeCCLA:
			log.WithFields(f).Debugf("adding CCLA filters: signature_type: %s, signature_reference_type: %s, signature_user_ccla_company_id: not exists", utils.SignatureTypeCCLA, utils.SignatureReferenceTypeCompany)
			filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCCLA)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeCompany)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeNotExists(), &filterAdded)
		}
	}

	if params.SearchTerm != nil && utils.StringValue(params.SearchTerm) != "" {
		//if *params.FullMatch {
		//	indexName = SignatureReferenceSearchIndex
		//	log.WithFields(f).Debugf("adding filter signature_reference_name_lower: %s", strings.ToLower(utils.StringValue(params.SearchTerm)))
		//	condition = condition.And(expression.Key("signature_reference_name_lower").Equal(expression.Value(strings.ToLower(utils.StringValue(params.SearchTerm)))))
		//} // else {
		log.WithFields(f).Debugf("adding filters signature_reference_name_lower: %s or user_email: %s", strings.ToLower(utils.StringValue(params.SearchTerm)), strings.ToLower(utils.StringValue(params.SearchTerm)))
		searchTermExpression := expression.Name("signature_reference_name_lower").Contains(strings.ToLower(utils.StringValue(params.SearchTerm))).
			Or(expression.Name("user_email").Contains(strings.ToLower(utils.StringValue(params.SearchTerm))))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
		//}
	}

	if params.Approved != nil {
		log.WithFields(f).Debugf("adding signature_approved: %t filter", aws.BoolValue(params.Approved))
		searchTermExpression := expression.Name("signature_approved").Equal(expression.Value(params.Approved))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}

	if params.Signed != nil {
		log.WithFields(f).Debugf("adding signature_signed: %t filter", aws.BoolValue(params.Signed))
		searchTermExpression := expression.Name("signature_signed").Equal(expression.Value(params.Signed))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}

	// If no query option was provided for approved and signed and our configuration default is to only show active signatures then we add the required query filters
	if params.Approved == nil && params.Signed == nil && config.GetConfig().SignatureQueryDefault == utils.SignatureQueryDefaultActive {
		log.WithFields(f).Debug("adding signature_approved: true and signature_signed: true filters")
		filter = addAndCondition(filter, expression.Name("signature_approved").Equal(expression.Value(true)), &filterAdded)
		filter = addAndCondition(filter, expression.Name("signature_signed").Equal(expression.Value(true)), &filterAdded)
	}

	if filterAdded {
		builder = builder.WithFilter(filter)
	}

	// Use the builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error building expression for project signature query, projectID: %s, error: %v", params.ProjectID, err)
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
	log.WithFields(f).Debugf("queryInput: %+v", queryInput)

	if params.NextKey != nil {
		queryInput.ExclusiveStartKey, err = decodeNextKey(*params.NextKey)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem decoding next key value")
			return nil, err
		}
		log.WithFields(f).Debugf("received a nextKey, value: %s - decoded: %+v", *params.NextKey, queryInput.ExclusiveStartKey)
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
		log.WithFields(f).Debugf("returned %d results", len(results.Items))

		// Convert the list of DB models to a list of response models
		signatureList, modelErr := repo.buildProjectSignatureModels(ctx, results, params.ProjectID, LoadACLDetails)
		if modelErr != nil {
			log.WithFields(f).Warnf("error converting DB model to response model for signatures with project %s, error: %v",
				params.ProjectID, modelErr)
			return nil, modelErr
		}

		// Add to the signatures response model to the list
		sigs = append(sigs, signatureList...)

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

	if len(lastEvaluatedKey) > 0 {
		log.WithFields(f).Debug("building next key...")
		encodedString, err := buildNextKey(indexName, sigs[len(sigs)-1])
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to build nextKey")
		}
		lastEvaluatedKey = encodedString
		log.WithFields(f).Debugf("lastEvaluatedKey encoded is: %s", encodedString)
	}

	log.WithFields(f).Debugf("returning %d signatures for CLA Group ID: %s", len(sigs), params.ProjectID)
	return &models.Signatures{
		ProjectID:      params.ProjectID,
		ResultCount:    int64(len(sigs)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Signatures:     sigs,
	}, nil
}

// getCLATypeFromParams helper function to combine the new CLA Type parameter and the old legacy signature type parameter - returns one of the values from utils.ClaTypeICLA, utils.ClaTypeECLA, utils.ClaTypeCCLA or empty string if nothing matches
func getCLATypeFromParams(params signatures.GetProjectSignaturesParams) string {
	if params.ClaType != nil {
		return strings.ToLower(*params.ClaType)
	} else if params.SignatureType != nil {
		// ICLA -> CLAType == icla, SignatureType == cla
		// ECLA -> CLAType == ecla, SignatureType == ecla
		// CCLA -> CLAType == ccla, SignatureType == ccla
		if strings.ToLower(*params.SignatureType) == "cla" {
			return utils.ClaTypeICLA
		}

		return strings.ToLower(*params.SignatureType)
	}

	return ""
}

// CreateProjectSummaryReport generates a project summary report based on the specified input
func (repo repository) CreateProjectSummaryReport(ctx context.Context, params signatures.CreateProjectSummaryReportParams) (*models.SignatureReport, error) { // nolint
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.CreateProjectSummaryReport",
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
		"approved":       utils.BoolValue(params.Approved),
		"signed":         utils.BoolValue(params.Signed),
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
			log.WithFields(f).Debugf("adding ICLA filters: signature_type: %s, signature_reference_type: %s, signature_user_ccla_company_id: not exists", utils.SignatureTypeCLA, utils.SignatureReferenceTypeUser)
			filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeNotExists(), &filterAdded)
		} else if strings.ToLower(*params.ClaType) == utils.ClaTypeECLA {
			log.WithFields(f).Debugf("adding ECLA filters: signature_type: %s, signature_reference_type: %s, signature_user_ccla_company_id: exists", utils.SignatureTypeCLA, utils.SignatureReferenceTypeUser)
			filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeExists(), &filterAdded)
		} else if strings.ToLower(*params.ClaType) == utils.ClaTypeCCLA {
			log.WithFields(f).Debugf("adding CCLA filters: signature_type: %s, signature_reference_type: %s, signature_user_ccla_company_id: not exists", utils.SignatureTypeCCLA, utils.SignatureReferenceTypeCompany)
			filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCCLA)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeCompany)), &filterAdded)
			filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeNotExists(), &filterAdded)
		}
	} else {
		if params.SearchField != nil {
			searchFieldExpression := expression.Name("signature_reference_type").Equal(expression.Value(params.SearchField))
			filter = addAndCondition(filter, searchFieldExpression, &filterAdded)
		}

		if params.SignatureType != nil {
			if params.SearchTerm != nil && (params.FullMatch != nil && !*params.FullMatch) {
				indexName = SignatureProjectIDTypeIndex
				condition = condition.And(expression.Key("signature_type").Equal(expression.Value(strings.ToLower(*params.SignatureType))))
			} else {
				signatureTypeExpression := expression.Name("signature_type").Equal(expression.Value(params.SignatureType))
				filter = addAndCondition(filter, signatureTypeExpression, &filterAdded)
			}
			if *params.SignatureType == utils.ClaTypeCCLA {
				signatureReferenceIDExpression := expression.Name("signature_reference_id").AttributeExists()
				signatureUserCclaCompanyIDExpression := expression.Name("signature_user_ccla_company_id").AttributeNotExists()
				filter = addAndCondition(filter, signatureReferenceIDExpression, &filterAdded)
				filter = addAndCondition(filter, signatureUserCclaCompanyIDExpression, &filterAdded)
			}
		}

		if params.SearchTerm != nil && utils.StringValue(params.SearchTerm) != "" {
			if utils.BoolValue(params.FullMatch) {
				indexName = SignatureReferenceSearchIndex
				condition = condition.And(expression.Key("signature_reference_name_lower").Equal(expression.Value(strings.ToLower(utils.StringValue(params.SearchTerm)))))
			} else {
				searchTermExpression := expression.Name("signature_reference_name_lower").Contains(strings.ToLower(utils.StringValue(params.SearchTerm))).
					Or(expression.Name("user_email").Contains(strings.ToLower(utils.StringValue(params.SearchTerm))))
				filter = addAndCondition(filter, searchTermExpression, &filterAdded)
			}
		}
	}

	if params.Approved != nil {
		filterAdded = true
		//log.WithFields(f).Debugf("adding filter signature_approved: %t", aws.BoolValue(params.Approved))
		searchTermExpression := expression.Name("signature_approved").Equal(expression.Value(aws.BoolValue(params.Approved)))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}
	if params.Signed != nil {
		filterAdded = true
		//log.WithFields(f).Debugf("adding filter signature_signed: %t", aws.BoolValue(params.Signed))
		searchTermExpression := expression.Name("signature_signed").Equal(expression.Value(aws.BoolValue(params.Signed)))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}

	// If no query option was provided for approved and signed and our configuration default is to only show active signatures then we add the required query filters
	if params.Approved == nil && params.Signed == nil && config.GetConfig().SignatureQueryDefault == utils.SignatureQueryDefaultActive {
		filterAdded = true
		filter = addAndCondition(filter, expression.Name("signature_approved").Equal(expression.Value(true)), &filterAdded)
		filter = addAndCondition(filter, expression.Name("signature_signed").Equal(expression.Value(true)), &filterAdded)
	}

	if len(params.Body) > 0 {
		// expression.Name("Color").In(expression.Value("red"), expression.Value("green"), expression.Value("blue"))
		var referenceIDExpressions []expression.OperandBuilder
		for _, value := range params.Body {
			referenceIDExpressions = append(referenceIDExpressions, expression.Value(value))
		}
		if len(referenceIDExpressions) == 1 {
			filter = addAndCondition(filter, expression.Name("signature_reference_id").In(referenceIDExpressions[0]), &filterAdded)
		} else if len(referenceIDExpressions) > 1 {
			filter = addAndCondition(filter, expression.Name("signature_reference_id").In(referenceIDExpressions[0], referenceIDExpressions[1:]...), &filterAdded)
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
		if params.FullMatch != nil && *params.FullMatch && params.SearchTerm != nil && utils.StringValue(params.SearchTerm) != "" {
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

// GetProjectCompanySignature returns the signature for the specified project and specified company with the other query flags
func (repo repository) GetProjectCompanySignature(ctx context.Context, companyID, projectID string, approved, signed *bool, nextKey *string, pageSize *int64) (*models.Signature, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetProjectCompanySignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"projectID":      projectID,
		"approved":       aws.BoolValue(approved),
		"signed":         aws.BoolValue(signed),
		"pageSize":       aws.Int64Value(pageSize),
		"nextKey":        aws.StringValue(nextKey),
	}

	log.WithFields(f).Debug("querying for project company signature...")
	sortOrder := utils.SortOrderAscending
	sigs, getErr := repo.GetProjectCompanySignatures(ctx, companyID, projectID, signed, approved, nextKey, &sortOrder, pageSize)
	if getErr != nil {
		log.WithFields(f).WithError(getErr).Warn("problem loading project company signatures...")
		return nil, getErr
	}

	if sigs == nil || len(sigs.Signatures) == 0 {
		return nil, nil
	}

	if len(sigs.Signatures) > 1 {
		log.WithFields(f).Warnf("more than 1 project company signatures returned in result using company ID: %s, project ID: %s - will return fist record",
			companyID, projectID)
	}

	log.WithFields(f).Debugf("returning project company signature")
	return sigs.Signatures[0], nil
}

// GetProjectCompanySignatures returns a list of signatures for the specified project and specified company
func (repo repository) GetProjectCompanySignatures(ctx context.Context, companyID, projectID string, approved, signed *bool, nextKey *string, sortOrder *string, pageSize *int64) (*models.Signatures, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetProjectCompanySignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"projectID":      projectID,
		"nextKey":        aws.StringValue(nextKey),
		"sortOrder":      aws.StringValue(sortOrder),
		"pageSize":       aws.Int64Value(pageSize),
		"approved":       utils.BoolValue(approved),
		"signed":         utils.BoolValue(signed),
	}

	var filterAdded bool
	// These are the keys we want to match
	//condition := expression.Key("signature_project_id").Equal(expression.Value(projectID))
	condition := expression.Key("signature_project_id").Equal(expression.Value(projectID)).
		And(expression.Key("signature_reference_id").Equal(expression.Value(companyID)))
	var filter expression.ConditionBuilder
	filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCCLA)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeCompany)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeNotExists(), &filterAdded)

	if approved != nil {
		filterAdded = true
		//log.WithFields(f).Debugf("adding filter signature_approved: %t", aws.BoolValue(approved))
		searchTermExpression := expression.Name("signature_approved").Equal(expression.Value(aws.BoolValue(approved)))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}
	if signed != nil {
		filterAdded = true
		//log.WithFields(f).Debugf("adding filter signature_signed: %t", aws.BoolValue(signed))
		searchTermExpression := expression.Name("signature_signed").Equal(expression.Value(aws.BoolValue(signed)))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}

	// If no query option was provided for approved and signed and our configuration default is to only show active signatures then we add the required query filters
	if approved == nil && signed == nil && config.GetConfig().SignatureQueryDefault == utils.SignatureQueryDefaultActive {
		filterAdded = true
		filter = addAndCondition(filter, expression.Name("signature_approved").Equal(expression.Value(true)), &filterAdded)
		filter = addAndCondition(filter, expression.Name("signature_signed").Equal(expression.Value(true)), &filterAdded)
	}

	limit := int64(10)
	if pageSize != nil {
		limit = *pageSize
	}
	log.WithFields(f).Debugf("page size %d", limit)

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project signature ID query, project: %s, error: %v",
			projectID, err)
		return nil, err
	}

	indexName := SignatureProjectReferenceIndex
	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String(indexName), // Name of a secondary index to scan
		Limit:                     aws.Int64(limit),
	}

	// If we have the next key, set the exclusive start key value
	if nextKey != nil {
		queryInput.ExclusiveStartKey, err = decodeNextKey(*nextKey)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem decoding next key value")
			return nil, err
		}
		//log.WithFields(f).Debugf("received a nextKey, value: %s - decoded: %+v", *nextKey, queryInput.ExclusiveStartKey)
	}

	sigs := make([]*models.Signature, 0)
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		// log.WithFields(f).Debugf("executing query for input: %+v", queryInput)
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).WithError(errQuery).Warnf("error retrieving project signature ID for project: %s with company: %s, error: %v",
				projectID, companyID, errQuery)
			return nil, errQuery
		}
		log.WithFields(f).Debugf("query response received with %d results", len(results.Items))

		// If we have any results - may not have any after filters are applied, but may have more records to page through...
		if len(results.Items) > 0 {
			// Convert the list of DB models to a list of response models
			//log.WithFields(f).Debugf("building response model for %d results", len(results.Items))
			signatureList, modelErr := repo.buildProjectSignatureModels(ctx, results, projectID, LoadACLDetails)
			if modelErr != nil {
				log.WithFields(f).Warnf("error converting DB model to response model for signatures with project %s with company: %s, error: %v",
					projectID, companyID, modelErr)
				return nil, modelErr
			}

			// Add to the signatures response model to the list
			sigs = append(sigs, signatureList...)
		}

		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"signature_id": {
					S: aws.String(lastEvaluatedKey),
				},
				"signature_project_id": {
					S: aws.String(projectID),
				},
				"signature_reference_id": {
					S: aws.String(companyID),
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

	// Calculate the next key - this uses a compound key - need to encode it before sharing with the caller
	if len(lastEvaluatedKey) > 0 {
		log.WithFields(f).Debug("building next key...")
		encodedString, err := buildNextKey(indexName, sigs[len(sigs)-1])
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to build nextKey")
		}
		lastEvaluatedKey = encodedString
		//log.WithFields(f).Debugf("lastEvaluatedKey encoded is: %s", encodedString)
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

// ProjectSignatures - get project signatures with no pagination
func (repo repository) ProjectSignatures(ctx context.Context, projectID string) (*models.Signatures, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.ProjectSignatures",
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
	filter = addAndCondition(filter, signatureApprovedExpression, &filterAdded)

	signatureSignedExpression := expression.Name("signature_signed").Equal(expression.Value(true))
	filter = addAndCondition(filter, signatureSignedExpression, &filterAdded)

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
		"functionName":   "v1.signatures.repository.InvalidateProjectRecord",
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

// ValidateProjectRecord validates the specified project record by setting the signature_approved flag to true
func (repo repository) ValidateProjectRecord(ctx context.Context, signatureID, note string) error {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.ValidateProjectRecord",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
	}

	// Update project signatures for signature_approved and notes attributes
	signatureTableName := fmt.Sprintf("cla-%s-signatures", repo.stage)

	expressionAttributeNames := map[string]*string{}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{}
	updateExpression := "SET " // nolint

	expressionAttributeNames["#A"] = aws.String("signature_approved")
	expressionAttributeValues[":a"] = &dynamodb.AttributeValue{BOOL: aws.Bool(true)}
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
func (repo repository) GetProjectCompanyEmployeeSignatures(ctx context.Context, params signatures.GetProjectCompanyEmployeeSignaturesParams, criteria *ApprovalCriteria) (*models.Signatures, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetProjectCompanyEmployeeSignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectID":      params.ProjectID,
		"companyID":      params.CompanyID,
		"nextKey":        aws.StringValue(params.NextKey),
		"sortOrder":      aws.StringValue(params.SortOrder),
	}

	totalCountChannel := make(chan int64, 1)
	go repo.getProjectCompanyEmployeeSignatureCount(ctx, params, criteria, totalCountChannel)

	pageSize := int64(HugePageSize)
	if params.PageSize != nil {
		pageSize = utils.Int64Value(params.PageSize)
	}
	f["pageSize"] = pageSize

	// This is the keys we want to match
	condition := expression.Key("signature_user_ccla_company_id").Equal(expression.Value(params.CompanyID)).And(
		expression.Key("signature_project_id").Equal(expression.Value(params.ProjectID)))

	var filterAdded bool
	var filter expression.ConditionBuilder

	if criteria != nil && criteria.GitHubUsername != "" {
		//log.WithFields(f).Debugf("adding GitHub username criteria filter for: %s ", criteria.GitHubUsername)
		filter = addAndCondition(filter, expression.Name(SignatureUserGitHubUsername).Equal(expression.Value(criteria.GitHubUsername)), &filterAdded)
	}

	if criteria != nil && criteria.GitlabUsername != "" {
		//log.WithFields(f).Debugf("adding GitLab username criteria filter for :%s ", criteria.GitlabUsername)
		filter = addAndCondition(filter, expression.Name(SignatureUserGitlabUsername).Equal(expression.Value(criteria.GitlabUsername)), &filterAdded)
	}

	if criteria != nil && criteria.UserEmail != "" {
		//log.WithFields(f).Debugf("adding useremail criteria filter for : %s ", criteria.UserEmail)
		filter = addAndCondition(filter, expression.Name("user_email").Equal(expression.Value(criteria.UserEmail)), &filterAdded)
	}

	if params.SearchTerm != nil {
		log.WithFields(f).Debugf("adding search term criteria filter for : %s ", *params.SearchTerm)
		searchExpression := expression.Name("user_name").Contains(*params.SearchTerm).
			Or(expression.Name("user_email").Contains(*params.SearchTerm)).
			Or(expression.Name("user_github_username").Contains(*params.SearchTerm)).
			Or(expression.Name("user_gitlab_username").Contains(*params.SearchTerm)).
			Or(expression.Name("user_lf_username").Contains(*params.SearchTerm))
		filter = addAndCondition(filter, searchExpression, &filterAdded)
	}

	beforeQuery, _ := utils.CurrentTime()
	//log.WithFields(f).Debugf("running signature query on table: %s", repo.signatureTableName)
	// Use the nice builder to create the expression
	expressionBuilder := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection())
	if filterAdded {
		expressionBuilder = expressionBuilder.WithFilter(filter)
	}
	expr, err := expressionBuilder.Build()
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
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String("signature-user-ccla-company-index"), // Name of a secondary index to scan
		Limit:                     aws.Int64(pageSize),
	}

	if filterAdded {
		queryInput.FilterExpression = expr.Filter()
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

		// Add to the signature response model to the list
		sigs = append(sigs, signatureList...)

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
	log.WithFields(f).Debugf("finished signature query on table: %s - duration: %+v", repo.signatureTableName, time.Since(beforeQuery))

	// remove duplicate values
	sigs = getLatestSignatures(sigs)

	// Meta-data for the response
	if int64(len(sigs)) > pageSize {
		sigs = sigs[0:pageSize]
		lastEvaluatedKey = sigs[pageSize-1].SignatureID
	}

	totalCount := <-totalCountChannel
	return &models.Signatures{
		ProjectID:      params.ProjectID,
		ResultCount:    int64(len(sigs)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Signatures:     sigs,
	}, nil
}

func getLatestSignatures(signatures []*models.Signature) []*models.Signature {
	f := logrus.Fields{
		"functionName": "v1.signatures.repository.getLatestSignatures",
	}

	signatureMap := make(map[string]*models.Signature)
	result := []*models.Signature{}

	log.WithFields(f).Debug("get latest signatures per contributor...")

	for _, signature := range signatures {
		if _, ok := signatureMap[signature.SignatureReferenceID]; !ok {
			log.WithFields(f).Debugf("adding signature: %s to map", signature.SignatureReferenceID)
			signatureMap[signature.SignatureReferenceID] = signature
		} else {
			if signature.Modified > signatureMap[signature.SignatureReferenceID].Modified {
				signatureMap[signature.SignatureReferenceID] = signature
			}
		}
	}

	log.WithFields(f).Debugf("signature Map: %+v", signatureMap)

	for _, signature := range signatureMap {
		result = append(result, signature)
	}

	return result
}

type EmployeeModel struct {
	Signature *models.Signature
	User      *models.User
}

func (repo repository) GetProjectCompanyEmployeeSignature(ctx context.Context, companyModel *models.Company, claGroupModel *models.ClaGroup, employeeUserModel *models.User, wg *sync.WaitGroup, resultChannel chan<- *EmployeeModel, errorChannel chan<- error) {
	defer wg.Done()

	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetProjectCompanyEmployeeSignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	if claGroupModel != nil {
		f["projectID"] = claGroupModel.ProjectID
		f["projectName"] = claGroupModel.ProjectName
	}
	if companyModel != nil {
		f["companyID"] = companyModel.CompanyID
		f["companyName"] = companyModel.CompanyName
	}
	if employeeUserModel != nil {
		f["employeeUserID"] = employeeUserModel.UserID
		f["employeeUserName"] = employeeUserModel.Username
		f["employeeEmails"] = strings.Join(employeeUserModel.Emails, ",")
	}

	if companyModel == nil || claGroupModel == nil || employeeUserModel == nil {
		resultChannel <- nil
		return
	}

	// This is the keys we want to match
	condition := expression.Key("signature_reference_id").Equal(expression.Value(employeeUserModel.UserID))

	var filterAdded bool
	var filter expression.ConditionBuilder

	// Check for approved signatures
	filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").Equal(expression.Value(companyModel.CompanyID)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_project_id").Equal(expression.Value(claGroupModel.ProjectID)), &filterAdded)

	log.WithFields(f).Debugf("running employee signature query on table: %s", repo.signatureTableName)
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter).WithProjection(buildProjection()).Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error building expression for employee signature query, company model: %+v, CLA group model: %+v, employee model: %+v",
			companyModel, claGroupModel, employeeUserModel)
		errorChannel <- err
		return
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
		Limit:                     aws.Int64(10),
	}

	// Make the DynamoDB Query API call
	results, errQuery := repo.dynamoDBClient.Query(queryInput)
	if errQuery != nil {
		log.WithFields(f).WithError(errQuery).Warnf("error retrieving project company employee acknowledgement record for company model: %+v, CLA group model: %+v, employee model: %+v",
			companyModel, claGroupModel, employeeUserModel)
		errorChannel <- errQuery
		return
	}

	if results == nil || len(results.Items) == 0 {
		log.WithFields(f).Debug("No ecla records found!")
		resultChannel <- &EmployeeModel{
			Signature: nil,
			User:      employeeUserModel,
		}
		return
	}
	log.WithFields(f).Debugf("returned %d results", len(results.Items))
	// Convert the list of DB models to a list of response models
	signatureList, modelErr := repo.buildProjectSignatureModels(ctx, results, claGroupModel.ProjectID, LoadACLDetails)
	if modelErr != nil {
		log.WithFields(f).WithError(modelErr).Warnf("error converting DB model to response model for project company employee acknowledgement record for company model: %+v, CLA group model: %+v, employee model: %+v",
			companyModel, claGroupModel, employeeUserModel)
		errorChannel <- modelErr
		return
	}

	if len(signatureList) == 0 {
		resultChannel <- nil
		return
	}

	if len(signatureList) > 1 {
		log.WithFields(f).Warnf("found more than one signature for employee company model: %+v, CLA group model: %+v, employee model: %+v",
			companyModel, claGroupModel, employeeUserModel)
	}

	resultChannel <- &EmployeeModel{
		Signature: signatureList[0],
		User:      employeeUserModel,
	}
}

// CreateProjectCompanyEmployeeSignature creates a new project employee signature using the provided details
func (repo repository) CreateProjectCompanyEmployeeSignature(ctx context.Context, companyModel *models.Company, claGroupModel *models.ClaGroup, employeeUserModel *models.User) error {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.CreateProjectCompanyEmployeeSignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	if claGroupModel != nil {
		f["projectID"] = claGroupModel.ProjectID
		f["projectName"] = claGroupModel.ProjectName
	}
	if companyModel != nil {
		f["companyID"] = companyModel.CompanyID
		f["companyName"] = companyModel.CompanyName
	}
	if employeeUserModel != nil {
		f["employeeUserID"] = employeeUserModel.UserID
		f["employeeUserName"] = employeeUserModel.Username
		f["employeeEmails"] = strings.Join(employeeUserModel.Emails, ",")
	}

	var wg sync.WaitGroup
	resultChan := make(chan *EmployeeModel)
	errorChan := make(chan error)

	wg.Add(1)
	go repo.GetProjectCompanyEmployeeSignature(ctx, companyModel, claGroupModel, employeeUserModel, &wg, resultChan, errorChan)

	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	for result := range resultChan {
		if result != nil {
			existingSig := result.Signature
			// If exists, need to update
			if existingSig != nil {
				log.WithFields(f).Debug("found existing employee acknowledgement")
				if !existingSig.SignatureApproved {
					log.WithFields(f).Debugf("found existing employee acknowledgement, but not currently approved.")
					validateRecordErr := repo.ValidateProjectRecord(ctx, existingSig.SignatureID, fmt.Sprintf(" Enabled previously disabled employee acknowledgement via CLA Manager approval list edit with auto-enable feature flag configured on %s.", utils.CurrentSimpleDateTimeString()))
					if validateRecordErr != nil {
						return validateRecordErr
					}
					return nil
				}

				return nil
			}
		}
	}

	for err := range errorChan {
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("error creating project company employee signature for company model: %+v, CLA group model: %+v, employee model: %+v",
				companyModel, claGroupModel, employeeUserModel)
		}
	}

	log.WithFields(f).Debugf("creating project company employee signature for project: %+v, company: %+v, employee: %+v", claGroupModel, companyModel, employeeUserModel)

	// If not exists, need to create
	// No existing records - create one
	_, currentTime := utils.CurrentTime()
	newSignatureID, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Unable to generate a UUID for signature record")
		return err
	}

	newSignature := &SignatureDynamoDB{
		SignatureID:                   newSignatureID.String(),
		SignatureProjectID:            claGroupModel.ProjectID,
		AutoCreateECLA:                false,
		SignatureType:                 utils.ClaTypeECLA,
		ProjectName:                   claGroupModel.ProjectName,
		ProjectSFID:                   claGroupModel.ProjectExternalID,
		CompanyName:                   companyModel.CompanyName,
		CompanyID:                     companyModel.CompanyID,
		CompanySFID:                   companyModel.CompanyExternalID,
		SignatureUserCCLACompanyID:    companyModel.CompanyID,
		ProjectID:                     claGroupModel.ProjectID,
		SignatureReferenceID:          employeeUserModel.UserID,
		SignatureApproved:             true,
		SignatureSigned:               true,
		SignatureDocumentMajorVersion: 2,
		SignatureDocumentMinorVersion: 0,
		SigTypeSignedApprovedID:       fmt.Sprintf("ecla#true#true#%s", companyModel.CompanyID),
		SignatureReferenceType:        utils.SignatureReferenceTypeUser,
		SignatureACL:                  []string{},
		SignedOn:                      currentTime,
		DateCreated:                   currentTime,
		DateModified:                  currentTime,
		Version:                       "v1",
		UserGitHubUsername:            employeeUserModel.GithubUsername,
		UserGitLabUsername:            employeeUserModel.GitlabUsername,
		Note:                          fmt.Sprintf("automatically created employee ackowledgement via CLA Manager approval list edit/update with auto_create_ecla feature flag set to true on %+v.", currentTime),
	}

	// Try to figure out the employee's name
	// Signature Reference Name fields MUST have a value - cannot be nil because it is indexed (we have a separate index for these columns)
	employeeUserName := employeeUserModel.Username
	if employeeUserName == "" {
		if employeeUserModel.LfUsername != "" {
			employeeUserName = employeeUserModel.LfUsername
		} else if employeeUserModel.GithubUsername != "" {
			employeeUserName = employeeUserModel.GithubUsername
		} else if employeeUserModel.GitlabUsername != "" {
			employeeUserName = employeeUserModel.GitlabUsername
		} else if employeeUserModel.LfEmail != "" {
			employeeUserName = employeeUserModel.LfEmail.String()
		} else if employeeUserModel.Emails != nil && len(employeeUserModel.Emails) > 0 {
			employeeUserName = employeeUserModel.Emails[0]
		}
	}
	if employeeUserName != "" {
		newSignature.SignatureReferenceName = employeeUserName
		newSignature.SignatureReferenceNameLower = strings.ToLower(employeeUserName)
	}

	av, marshalErr := dynamodbattribute.MarshalMap(newSignature)
	if marshalErr != nil {
		log.WithFields(f).WithError(marshalErr).Warn("unable to create new signature record")
		return marshalErr
	}

	_, putErr := repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(repo.signatureTableName),
	})
	if putErr != nil {
		log.WithFields(f).WithError(putErr).Warn("cannot create new signature record")
		return putErr
	}

	return nil
}

// getProjectCompanyEmployeeSignatureCount returns the total count of employee signatures for the specified project and specified company
func (repo repository) getProjectCompanyEmployeeSignatureCount(ctx context.Context, params signatures.GetProjectCompanyEmployeeSignaturesParams, criteria *ApprovalCriteria, responseChannel chan int64) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.getProjectCompanyEmployeeSignatureCount",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectID":      params.ProjectID,
		"companyID":      params.CompanyID,
		"nextKey":        aws.StringValue(params.NextKey),
		"sortOrder":      aws.StringValue(params.SortOrder),
	}

	// Ignore the provided page count in the parameters - we're focused on getting the total count
	pageSize := int64(HugePageSize)
	f["pageSize"] = pageSize

	// This is the keys we want to match
	condition := expression.Key("signature_user_ccla_company_id").Equal(expression.Value(params.CompanyID)).And(
		expression.Key("signature_project_id").Equal(expression.Value(params.ProjectID)))

	var filterAdded bool
	var filter expression.ConditionBuilder

	if criteria != nil && criteria.GitHubUsername != "" {
		filter = addAndCondition(filter, expression.Name(SignatureUserGitHubUsername).Equal(expression.Value(criteria.GitHubUsername)), &filterAdded)
	}

	if criteria != nil && criteria.GitHubUsername != "" {
		filter = addAndCondition(filter, expression.Name(SignatureUserGitlabUsername).Equal(expression.Value(criteria.GitlabUsername)), &filterAdded)
	}

	if criteria != nil && criteria.UserEmail != "" {
		filter = addAndCondition(filter, expression.Name("user_email").Equal(expression.Value(criteria.UserEmail)), &filterAdded)
	}

	beforeQuery, _ := utils.CurrentTime()
	log.WithFields(f).Debugf("running total signature count query on table: %s", repo.signatureTableName)
	// Use the nice builder to create the expression
	expressionBuilder := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildCountProjection())
	if filterAdded {
		expressionBuilder = expressionBuilder.WithFilter(filter)
	}
	expr, err := expressionBuilder.Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for project signature ID query, project: %s, error: %v",
			params.ProjectID, err)
		responseChannel <- 0
		return
	}

	// Assemble the query input parameters - ignore the provided exclusive start key, we're only interested in the total count
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		// FilterExpression:          expr.Filter(),
		ProjectionExpression: expr.Projection(),
		TableName:            aws.String(repo.signatureTableName),
		IndexName:            aws.String("signature-user-ccla-company-index"), // Name of a secondary index to scan
		Limit:                aws.Int64(pageSize),
	}

	if filterAdded {
		queryInput.FilterExpression = expr.Filter()
	}

	var lastEvaluatedKey string
	var totalCount int64

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving project company employee signature ID for project: %s with company: %s, error: %v",
				params.ProjectID, params.CompanyID, errQuery)
			responseChannel <- 0
			return
		}

		// Add to our total count
		totalCount += *results.Count

		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			lastEvaluatedKey = ""
		}
	}
	log.WithFields(f).Debugf("finished signature total count query on table: %s - duration: %+v", repo.signatureTableName, time.Since(beforeQuery))

	responseChannel <- totalCount
}

// GetCompanySignatures returns a list of company signatures for the specified company
func (repo repository) GetCompanySignatures(ctx context.Context, params signatures.GetCompanySignaturesParams, pageSize int64, loadACL bool) (*models.Signatures, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetCompanySignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	// This is the keys we want to match
	condition := expression.Key("signature_reference_id").Equal(expression.Value(params.CompanyID))

	var filterAdded bool
	var filter expression.ConditionBuilder
	filter = addAndCondition(filter, expression.Name("signature_approved").Equal(expression.Value(true)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_signed").Equal(expression.Value(true)), &filterAdded)

	if params.SignatureType != nil {
		filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(*params.SignatureType)), &filterAdded)
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
		"functionName":             "v1.signatures.repository.GetCompanyIDsWithSignedCorporateSignatures",
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

	var filterAdded bool
	var filter expression.ConditionBuilder

	filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCCLA)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeCompany)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeNotExists(), &filterAdded)
	// Check for approved signatures
	filter = addAndCondition(filter, expression.Name("signature_approved").Equal(expression.Value(true)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_signed").Equal(expression.Value(true)), &filterAdded)

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
func (repo repository) GetUserSignatures(ctx context.Context, params signatures.GetUserSignaturesParams, pageSize int64, projectID *string) (*models.Signatures, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetUserSignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	// This is the keys we want to match
	condition := expression.Key("signature_reference_id").Equal(expression.Value(params.UserID))

	filterExpression := expression.Name("signature_user_ccla_company_id").AttributeNotExists()

	// Check for approved signatures
	expressionBuilder := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection())

	if projectID != nil {
		filterExpression = filterExpression.And(expression.Name("signature_project_id").Equal(expression.Value(*projectID)))
	}

	expressionBuilder = expressionBuilder.WithFilter(filterExpression)

	// Use the nice builder to create the expression
	expr, err := expressionBuilder.Build()
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

		log.WithFields(f).Debugf("query results count: %d", len(results.Items))

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
		"functionName":   "v1.signatures.repository.AddCLAManager",
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
		"functionName":   "v1.signatures.repository.RemoveCLAManager",
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
	f := logrus.Fields{
		"functionName": "v1.signatures.repository.UpdateApprovalList",
		"projectID":    projectID,
		"companyID":    companyID,
	}
	log.WithFields(f).Debug("querying database for approval list details")

	approved, signed := true, true

	// Get CCLA signature - For Approval List info
	cclaSignature, err := repo.GetCorporateSignature(ctx, projectID, companyID, &approved, &signed)
	if err != nil || cclaSignature == nil {
		msg := fmt.Sprintf("unable to get corporate signature for CLA Group: %s and company: %s", projectID, companyID)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	signatureID := cclaSignature.SignatureID

	// Get CLA Manager
	var cclaManagers []ClaManagerInfoParams
	for i := range cclaSignature.SignatureACL {
		cclaManagers = append(cclaManagers, ClaManagerInfoParams{
			Username: utils.GetBestUsername(&cclaSignature.SignatureACL[i]),
			Email:    getBestEmail(&cclaSignature.SignatureACL[i]),
		})
	}

	// Keep track of existing company approvals
	approvalList := ApprovalList{
		EmailApprovals:          cclaSignature.EmailApprovalList,
		DomainApprovals:         cclaSignature.DomainApprovalList,
		GitHubUsernameApprovals: cclaSignature.GithubUsernameApprovalList,
		GitHubOrgApprovals:      cclaSignature.GithubOrgApprovalList,
		GitlabUsernameApprovals: cclaSignature.GitlabUsernameApprovalList,
		GitlabOrgApprovals:      cclaSignature.GitlabOrgApprovalList,
		CLAManager:              claManager,
		ICLAs:                   make([]*models.IclaSignature, 0),
		ECLAs:                   make([]*models.Signature, 0),
		ManagersInfo:            cclaManagers,
		CCLASignature:           cclaSignature,
	}

	// Just grab and use the first one - need to figure out conflict resolution if more than one
	expressionAttributeNames := map[string]*string{}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{}
	haveAdditions := false
	updateExpression := ""

	employeeSignatureParams := signatures.GetProjectCompanyEmployeeSignaturesParams{
		ProjectID: projectID,
		CompanyID: companyID,
		PageSize:  utils.Int64(10),
	}

	authUser := auth.User{
		Email:    claManager.LfEmail.String(),
		UserName: claManager.LfUsername,
	}

	// Keep track of gerrit users under a give CLA Group
	var gerritICLAECLAs []string

	// Only load the gerrit user information, which is costly, if we have updates to remove email or email domains
	if (params.RemoveEmailApprovalList != nil && len(params.RemoveEmailApprovalList) > 0) || (params.RemoveDomainApprovalList != nil && len(params.RemoveDomainApprovalList) > 0) {

		goRoutines := 2
		gerritResultChannel := make(chan *GerritUserResponse, goRoutines)
		gerritQueryStartTime, _ := utils.CurrentTime()
		go repo.getGerritUsers(ctx, &authUser, projectID, utils.ClaTypeICLA, gerritResultChannel)
		go repo.getGerritUsers(ctx, &authUser, projectID, utils.ClaTypeECLA, gerritResultChannel)

		log.WithFields(f).Debug("waiting on gerrit user query results from 2 go routines...")
		for i := 0; i < goRoutines; i++ {
			results := <-gerritResultChannel
			log.WithFields(f).Debugf("received gerrit user query results response for %s - took: %+v", results.queryType, time.Since(gerritQueryStartTime))
			if results.Error != nil {
				log.WithFields(f).WithError(results.Error).Warnf("problem retrieving gerrit users for %s, error: %+v", results.queryType, results.Error)
			} else {
				for _, member := range results.gerritGroupResponse.Members {
					gerritICLAECLAs = append(gerritICLAECLAs, member.Username)
				}
				log.WithFields(f).Debugf("updated gerrit user query results response for %s - list size is %d...", results.queryType, len(gerritICLAECLAs))
			}
		}
		log.WithFields(f).Debugf("received the gerrit user query results from %d go routines...", goRoutines)
	}

	// If we have an add or remove email list...we need to run an update for this column
	if (params.AddEmailApprovalList != nil && len(params.AddEmailApprovalList) > 0) || (params.RemoveEmailApprovalList != nil && len(params.RemoveEmailApprovalList) > 0) {
		columnName := SignatureEmailApprovalListColumn
		attrList := buildApprovalAttributeList(ctx, cclaSignature.EmailApprovalList, params.AddEmailApprovalList, params.RemoveEmailApprovalList)
		// If no entries after consolidating all the updates, we need to remove the column
		if attrList == nil || attrList.L == nil {
			var rmColErr error
			cclaSignature, rmColErr = repo.removeColumn(ctx, cclaSignature.SignatureID, columnName)
			if rmColErr != nil {
				msg := fmt.Sprintf("unable to remove column %s for signature for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
					columnName, companyID, projectID, true, true)
				log.WithFields(f).Warn(msg)
				return nil, errors.New(msg)
			}
		} else {
			haveAdditions = true
			expressionAttributeNames["#E"] = aws.String(columnName)
			expressionAttributeValues[":e"] = attrList
			updateExpression = updateExpression + " #E = :e, "
		}

		log.WithFields(f).Debugf("updating approval list table")

		if params.AddEmailApprovalList != nil {
			repo.updateApprovalTable(ctx, params.AddEmailApprovalList, utils.EmailApprovalCriteria, signatureID, projectID, companyID, cclaSignature.SignatureReferenceName, true)
		}

		// if email removal update signature approvals
		if params.RemoveEmailApprovalList != nil {
			repo.updateApprovalTable(ctx, params.AddEmailApprovalList, utils.EmailApprovalCriteria, signatureID, projectID, companyID, cclaSignature.SignatureReferenceName, false)
			log.WithFields(f).Debugf("removing email: %+v the approval list", params.RemoveDomainApprovalList)
			var wg sync.WaitGroup
			wg.Add(len(params.RemoveEmailApprovalList))
			approvalList.Criteria = utils.EmailCriteria
			approvalList.ApprovalList = params.RemoveEmailApprovalList
			approvalList.Action = utils.RemoveApprovals
			approvalList.Version = claGroupModel.Version
			for _, email := range params.RemoveEmailApprovalList {
				go func(email string) {
					defer wg.Done()
					var iclas []*models.IclaSignature
					var eclas []*models.Signature
					log.WithFields(f).Debugf("getting cla user record for email: %s ", email)
					userSearch, userErr := repo.usersRepo.SearchUsers("user_emails", email, false)
					if userErr != nil || userSearch == nil {
						log.WithFields(f).Debugf("error getting user by email: %s ", email)
						return
					}
					criteria := &ApprovalCriteria{
						UserEmail: email,
					}
					log.WithFields(f).Debugf("Updating signature records for email approval list: %+v ", params.RemoveEmailApprovalList)
					signs, appErr := repo.GetProjectCompanyEmployeeSignatures(ctx, employeeSignatureParams, criteria)
					if appErr != nil {
						log.WithFields(f).Debugf("unable to get Company Employee signatures : %+v ", appErr)
						return
					}

					if len(signs.Signatures) == 0 {
						log.WithFields(f).Debugf("company employee signatures do not exist for company: %s and project: %s ", companyID, projectID)
					}

					if len(signs.Signatures) > 0 {
						approvalList.ECLAs = signs.Signatures
						eclas = signs.Signatures
					}

					if len(userSearch.Users) > 0 {
						// Try and grab iclaSignature records for users
						results := make(chan *ICLAUserResponse, len(userSearch.Users))
						go func() {
							defer close(results)
							for _, user := range userSearch.Users {
								icla, iclaErr := repo.GetIndividualSignature(ctx, projectID, user.UserID, &approved, &signed)
								if iclaErr != nil || icla == nil {
									results <- &ICLAUserResponse{
										Error: fmt.Errorf("unable to get icla for user: %s ", user.UserID),
									}
								} else {

									// Update gerrit user
									if utils.StringInSlice(user.LfUsername, gerritICLAECLAs) {
										gerritIclaErr := repo.gerritService.RemoveUserFromGroup(ctx, &authUser, approvalList.ClaGroupID, user.LfUsername, utils.ClaTypeICLA)
										if gerritIclaErr != nil {
											msg := fmt.Sprintf("unable to remove gerrit user: %s from group: %s", user.LfUsername, approvalList.ClaGroupID)
											log.WithFields(f).WithError(gerritIclaErr).Warn(msg)
										}
										eclaErr := repo.gerritService.RemoveUserFromGroup(ctx, &authUser, approvalList.ClaGroupID, user.LfUsername, utils.ClaTypeECLA)
										if eclaErr != nil {
											msg := fmt.Sprintf("unable to remove gerrit user: %s from group: %s", user.LfUsername, approvalList.ClaGroupID)
											log.WithFields(f).WithError(eclaErr).Warn(msg)
										}
									}
									results <- &ICLAUserResponse{
										ICLASignature: &models.IclaSignature{
											GithubUsername: icla.UserGHUsername,
											LfUsername:     user.LfUsername,
											SignatureID:    icla.SignatureID,
										},
									}
								}
							}
						}()

						for result := range results {
							if result.Error == nil {
								log.WithFields(f).Debug("processing icla...")
								approvalList.ICLAs = append(approvalList.ICLAs, result.ICLASignature)
								iclas = append(iclas, result.ICLASignature)
							}
						}

					}

					// Invalidate signatures
					repo.invalidateSignatures(ctx, &approvalList, claManager, eventArgs)

					// Send email
					repo.sendEmail(ctx, email, &approvalList, iclas, eclas)

				}(email)
			}
			wg.Wait()
		}
	}

	if (params.AddDomainApprovalList != nil && len(params.AddDomainApprovalList) > 0) || (params.RemoveDomainApprovalList != nil && len(params.RemoveDomainApprovalList) > 0) {

		columnName := SignatureDomainApprovalListColumn
		attrList := buildApprovalAttributeList(ctx, cclaSignature.DomainApprovalList, params.AddDomainApprovalList, params.RemoveDomainApprovalList)
		// If no entries after consolidating all the updates, we need to remove the column
		if attrList == nil || attrList.L == nil {
			var rmColErr error
			cclaSignature, rmColErr = repo.removeColumn(ctx, cclaSignature.SignatureID, columnName)
			if rmColErr != nil {
				msg := fmt.Sprintf("unable to remove column %s for signature for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
					columnName, companyID, projectID, true, true)
				log.WithFields(f).Warn(msg)
				return nil, errors.New(msg)
			}
		} else {
			haveAdditions = true
			expressionAttributeNames["#D"] = aws.String(columnName)
			expressionAttributeValues[":d"] = attrList
			updateExpression = updateExpression + " #D = :d, "
		}

		log.WithFields(f).Debugf("updating approval list table")
		if params.AddDomainApprovalList != nil {
			repo.updateApprovalTable(ctx, params.AddDomainApprovalList, utils.EmailApprovalCriteria, signatureID, projectID, companyID, cclaSignature.SignatureReferenceName, true)
		}

		if params.RemoveDomainApprovalList != nil {
			// Get ICLAs
			log.WithFields(f).Debug("getting icla records... ")
			iclas, iclaErr := repo.GetClaGroupICLASignatures(ctx, approvalList.ClaGroupID, nil, &approved, &signed, 0, "", true)
			if iclaErr != nil {
				log.WithFields(f).Warn("unable to get iclas")
			}
			// Get ECLAs
			log.WithFields(f).Debug("getting ecla records... ")
			companyProjectParams := signatures.GetProjectCompanyEmployeeSignaturesParams{
				CompanyID: approvalList.CompanyID,
				ProjectID: approvalList.ClaGroupID,
				PageSize:  utils.Int64(10),
			}

			criteria := ApprovalCriteria{}
			eclas, eclaErr := repo.GetProjectCompanyEmployeeSignatures(ctx, companyProjectParams, &criteria)
			if eclaErr != nil {
				log.WithFields(f).Warnf("unable to get cclas for company: %s and project: %s ", approvalList.CompanyID, approvalList.ClaGroupID)
			}

			approvalList.Criteria = utils.EmailDomainCriteria
			approvalList.ApprovalList = params.RemoveDomainApprovalList
			approvalList.Action = utils.RemoveApprovals
			approvalList.GerritICLAECLAs = gerritICLAECLAs
			approvalList.ClaGroupID = projectID
			approvalList.ClaGroupName = claGroupModel.ProjectName
			approvalList.CompanyID = companyID
			approvalList.Version = claGroupModel.Version
			if iclas != nil {
				approvalList.ICLAs = iclas.List
			}
			if eclas != nil {
				approvalList.ECLAs = eclas.Signatures
			}

			repo.invalidateSignatures(ctx, &approvalList, claManager, eventArgs)
			repo.updateApprovalTable(ctx, params.AddDomainApprovalList, utils.EmailApprovalCriteria, signatureID, projectID, companyID, cclaSignature.SignatureReferenceName, false)
		}
	}

	if (params.AddGithubUsernameApprovalList != nil && len(params.AddGithubUsernameApprovalList) > 0) || (params.RemoveGithubUsernameApprovalList != nil && len(params.RemoveGithubUsernameApprovalList) > 0) {
		columnName := SignatureGitHubUsernameApprovalListColumn
		attrList := buildApprovalAttributeList(ctx, cclaSignature.GithubUsernameApprovalList, params.AddGithubUsernameApprovalList, params.RemoveGithubUsernameApprovalList)
		// If no entries after consolidating all the updates, we need to remove the column
		if attrList == nil || attrList.L == nil {
			var rmColErr error
			cclaSignature, rmColErr = repo.removeColumn(ctx, cclaSignature.SignatureID, columnName)
			if rmColErr != nil {
				msg := fmt.Sprintf("unable to remove column %s for signature for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
					columnName, companyID, projectID, true, true)
				log.WithFields(f).Warn(msg)
				return nil, errors.New(msg)
			}
		} else {
			haveAdditions = true
			expressionAttributeNames["#GHU"] = aws.String(columnName)
			expressionAttributeValues[":ghu"] = attrList
			updateExpression = updateExpression + " #GHU = :ghu, "
		}

		if params.AddGithubUsernameApprovalList != nil {
			repo.updateApprovalTable(ctx, params.AddGithubUsernameApprovalList, utils.GithubUsernameApprovalCriteria, signatureID, projectID, companyID, cclaSignature.SignatureReferenceName, true)
		}
		if params.RemoveGithubUsernameApprovalList != nil {

			repo.updateApprovalTable(ctx, params.AddGithubUsernameApprovalList, utils.GithubUsernameApprovalCriteria, signatureID, projectID, companyID, cclaSignature.SignatureReferenceName, false)
			// if email removal update signature approvals
			if params.RemoveGithubUsernameApprovalList != nil {
				var wg sync.WaitGroup
				approvalList.Criteria = utils.GitHubUsernameCriteria
				approvalList.ApprovalList = params.RemoveGithubUsernameApprovalList
				approvalList.Action = utils.RemoveApprovals
				approvalList.ClaGroupID = projectID
				approvalList.ClaGroupName = claGroupModel.ProjectName
				approvalList.CompanyID = companyID
				approvalList.Version = claGroupModel.Version
				wg.Add(len(params.RemoveGithubUsernameApprovalList))
				for _, ghUsername := range params.RemoveGithubUsernameApprovalList {
					go func(ghUsername string) {
						defer wg.Done()
						var iclas []*models.IclaSignature
						var eclas []*models.Signature

						criteria := &ApprovalCriteria{
							GitHubUsername: ghUsername,
						}
						log.WithFields(f).Debugf("Updating signature records for github username apporval list: %+v ", params.RemoveGithubUsernameApprovalList)
						signs, ghUserErr := repo.GetProjectCompanyEmployeeSignatures(ctx, employeeSignatureParams, criteria)
						if ghUserErr != nil {
							log.WithFields(f).Debugf("unable to get Company Employee signatures : %+v ", ghUserErr)
							return
						}
						if signs.Signatures != nil {
							approvalList.ECLAs = signs.Signatures
							eclas = signs.Signatures
						}
						// Get ICLAs
						claUser, claErr := repo.usersRepo.GetUserByGitHubUsername(ghUsername)
						if claErr != nil {
							log.WithFields(f).Debugf("unable to get user by github username: %s ", ghUsername)
							return
						}
						if claUser != nil {
							icla, iclaErr := repo.GetIndividualSignature(ctx, projectID, claUser.UserID, &approved, &signed)
							if iclaErr != nil || icla == nil {
								log.WithFields(f).Debugf("unable to get icla signature for user with github username: %s ", ghUsername)
							}
							if icla != nil {
								// Convert to IclSignature instance to leverage invalidateSignatures helper function
								approvalList.ICLAs = []*models.IclaSignature{{
									GithubUsername: icla.UserGHUsername,
									LfUsername:     icla.UserLFID,
									SignatureID:    icla.SignatureID,
								}}
							}
						}

						repo.invalidateSignatures(ctx, &approvalList, claManager, eventArgs)

						// Send Email
						repo.sendEmail(ctx, getBestEmail(claUser), &approvalList, iclas, eclas)

					}(ghUsername)
				}
				wg.Wait()
			}
		}
	}

	if (params.AddGithubOrgApprovalList != nil && len(params.AddGithubOrgApprovalList) > 0) || (params.RemoveGithubOrgApprovalList != nil && len(params.RemoveGithubOrgApprovalList) > 0) {
		columnName := SignatureGitHubOrgApprovalListColumn
		attrList := buildApprovalAttributeList(ctx, cclaSignature.GithubOrgApprovalList, params.AddGithubOrgApprovalList, params.RemoveGithubOrgApprovalList)
		// If no entries after consolidating all the updates, we need to remove the column
		if attrList == nil || attrList.L == nil {
			var rmColErr error
			cclaSignature, rmColErr = repo.removeColumn(ctx, cclaSignature.SignatureID, columnName)
			if rmColErr != nil {
				msg := fmt.Sprintf("unable to remove column %s for signature for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
					columnName, companyID, projectID, true, true)
				log.WithFields(f).Warn(msg)
				return nil, errors.New(msg)
			}
		} else {
			haveAdditions = true
			expressionAttributeNames["#GHO"] = aws.String(columnName)
			expressionAttributeValues[":gho"] = attrList
			updateExpression = updateExpression + " #GHO = :gho, "
		}

		if params.AddGithubOrgApprovalList != nil {
			repo.updateApprovalTable(ctx, params.AddGithubOrgApprovalList, utils.GithubOrgApprovalCriteria, signatureID, projectID, companyID, cclaSignature.SignatureReferenceName, true)
		}

		if params.RemoveGithubOrgApprovalList != nil {
			approvalList.Criteria = utils.GitHubOrgCriteria
			approvalList.ApprovalList = params.RemoveGithubOrgApprovalList
			approvalList.Action = utils.RemoveApprovals
			approvalList.Version = claGroupModel.Version
			// Get repositories by CLAGroup
			repositories, getRepoByCLAGroupErr := repo.repositoriesRepo.GitHubGetRepositoriesByCLAGroup(ctx, projectID, true)
			if getRepoByCLAGroupErr != nil {
				msg := fmt.Sprintf("unable to fetch repositories for cla group ID: %s ", projectID)
				log.WithFields(f).WithError(getRepoByCLAGroupErr).Warn(msg)
				return nil, errors.New(msg)
			}
			var ghOrgRepositories []*models.GithubRepository
			var ghOrgs []*models.GithubOrganization
			for _, repository := range repositories {
				// Check for matching organization name in repositories table against approvalList removal GitHub organizations
				if utils.StringInSlice(repository.RepositoryOrganizationName, approvalList.ApprovalList) {
					ghOrgRepositories = append(ghOrgRepositories, repository)
				}
			}

			for _, ghOrgRepo := range ghOrgRepositories {
				ghOrg, getGHOrgErr := repo.ghOrgRepo.GetGitHubOrganization(ctx, ghOrgRepo.RepositoryOrganizationName)
				if getGHOrgErr != nil {
					msg := fmt.Sprintf("unable to get gh org by name: %s ", ghOrgRepo.RepositoryOrganizationName)
					log.WithFields(f).WithError(getGHOrgErr).Warn(msg)
					return nil, errors.New(msg)
				}
				ghOrgs = append(ghOrgs, ghOrg)
			}

			var ghUsernames []string
			for _, ghOrg := range ghOrgs {
				ghOrgUsers, getOrgMembersErr := github.GetOrganizationMembers(ctx, ghOrg.OrganizationName, ghOrg.OrganizationInstallationID)
				if getOrgMembersErr != nil {
					msg := fmt.Sprintf("unable to fetch github organization users for org: %s ", ghOrg.OrganizationName)
					log.WithFields(f).WithError(getOrgMembersErr).Warnf(msg)
					return nil, errors.New(msg)
				}
				ghUsernames = append(ghUsernames, ghOrgUsers...)
			}
			approvalList.GitHubUsernames = utils.RemoveDuplicates(ghUsernames)

			repo.invalidateSignatures(ctx, &approvalList, claManager, eventArgs)
			repo.updateApprovalTable(ctx, params.AddGithubOrgApprovalList, utils.GithubOrgApprovalCriteria, signatureID, projectID, companyID, cclaSignature.SignatureReferenceName, false)
		}
	}

	if (params.AddGitlabUsernameApprovalList != nil && len(params.AddGitlabUsernameApprovalList) > 0) || (params.RemoveGitlabUsernameApprovalList != nil && len(params.RemoveGitlabUsernameApprovalList) > 0) {
		columnName := SignatureGitlabUsernameApprovalListColumn
		attrList := buildApprovalAttributeList(ctx, cclaSignature.GitlabUsernameApprovalList, params.AddGitlabUsernameApprovalList, params.RemoveGitlabUsernameApprovalList)
		// If no entries after consolidating all the updates, we need to remove the column
		if attrList == nil || attrList.L == nil {
			var rmColErr error
			cclaSignature, rmColErr = repo.removeColumn(ctx, cclaSignature.SignatureID, columnName)
			if rmColErr != nil {
				msg := fmt.Sprintf("unable to remove column %s for signature for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
					columnName, companyID, projectID, true, true)
				log.WithFields(f).Warn(msg)
				return nil, errors.New(msg)
			}
		} else {
			haveAdditions = true
			expressionAttributeNames["#GLU"] = aws.String(columnName)
			expressionAttributeValues[":glu"] = attrList
			updateExpression = updateExpression + " #GLU = :glu, "
		}
		if params.AddGitlabUsernameApprovalList != nil {
			repo.updateApprovalTable(ctx, params.AddGitlabUsernameApprovalList, utils.GitlabUsernameApprovalCriteria, signatureID, projectID, companyID, cclaSignature.SignatureReferenceName, true)
		}
		if params.RemoveGitlabUsernameApprovalList != nil {
			repo.updateApprovalTable(ctx, params.AddGitlabUsernameApprovalList, utils.GitlabUsernameApprovalCriteria, signatureID, projectID, companyID, cclaSignature.SignatureReferenceName, false)
			// if email removal update signature approvals
			if params.RemoveGitlabUsernameApprovalList != nil {
				approvalList.Criteria = utils.GitlabUsernameCriteria
				approvalList.ApprovalList = params.RemoveGitlabUsernameApprovalList
				approvalList.Action = utils.RemoveApprovals
				approvalList.ClaGroupID = projectID
				approvalList.ClaGroupName = claGroupModel.ProjectName
				approvalList.CompanyID = companyID
				approvalList.Version = claGroupModel.Version

				// Get ICLAs
				var wg sync.WaitGroup
				wg.Add(len(params.RemoveGitlabUsernameApprovalList))
				for _, ghUsername := range params.RemoveGitlabUsernameApprovalList {
					go func(gitLabUsername string) {
						defer wg.Done()
						var iclas []*models.IclaSignature
						var eclas []*models.Signature

						criteria := &ApprovalCriteria{
							GitlabUsername: gitLabUsername,
						}
						log.WithFields(f).Debugf("Updating signature records for gitlab username apporval list: %+v ", params.RemoveGitlabUsernameApprovalList)
						signs, ghUserErr := repo.GetProjectCompanyEmployeeSignatures(ctx, employeeSignatureParams, criteria)
						if ghUserErr != nil {
							log.WithFields(f).Debugf("unable to get Company Employee signatures : %+v ", ghUserErr)
							return
						}
						if signs.Signatures != nil {
							approvalList.ECLAs = signs.Signatures
							eclas = signs.Signatures
						}

						claUser, claErr := repo.usersRepo.GetUserByGitLabUsername(gitLabUsername)
						if claErr != nil {
							log.WithFields(f).Debugf("unable to get User by gitlab username: %s ", gitLabUsername)
							return
						}
						if claUser != nil {
							icla, iclaErr := repo.GetIndividualSignature(ctx, projectID, claUser.UserID, &approved, &signed)
							if iclaErr != nil || icla == nil {
								log.WithFields(f).Debugf("unable to get icla signature for user with gitlab username: %s ", gitLabUsername)
							}
							if icla != nil {
								// Convert to IclSignature instance to leverage invalidateSignatures helper function
								approvalList.ICLAs = []*models.IclaSignature{{
									GitlabUsername: icla.UserGHUsername,
									LfUsername:     icla.UserLFID,
									SignatureID:    icla.SignatureID,
								}}
							}
						}

						repo.invalidateSignatures(ctx, &approvalList, claManager, eventArgs)

						// Send Email
						repo.sendEmail(ctx, getBestEmail(claUser), &approvalList, iclas, eclas)

					}(ghUsername)
				}
				wg.Wait()
			}
		}
	}

	if (params.AddGitlabOrgApprovalList != nil && len(params.AddGitlabOrgApprovalList) > 0) || (params.RemoveGitlabOrgApprovalList != nil && len(params.RemoveGitlabOrgApprovalList) > 0) {
		columnName := SignatureGitlabOrgApprovalListColumn
		attrList := buildApprovalAttributeList(ctx, cclaSignature.GitlabOrgApprovalList, params.AddGitlabOrgApprovalList, params.RemoveGitlabOrgApprovalList)
		// If no entries after consolidating all the updates, we need to remove the column
		if attrList == nil || attrList.L == nil {
			var rmColErr error
			cclaSignature, rmColErr = repo.removeColumn(ctx, cclaSignature.SignatureID, columnName)
			if rmColErr != nil {
				msg := fmt.Sprintf("unable to remove column %s for signature for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t",
					columnName, companyID, projectID, true, true)
				log.WithFields(f).Warn(msg)
				return nil, errors.New(msg)
			}
		} else {
			haveAdditions = true
			expressionAttributeNames["#GLO"] = aws.String(columnName)
			expressionAttributeValues[":glo"] = attrList
			updateExpression = updateExpression + " #GLO = :glo, "
		}

		if params.AddGitlabOrgApprovalList != nil {
			repo.updateApprovalTable(ctx, params.AddGitlabOrgApprovalList, utils.GitlabOrgApprovalCriteria, signatureID, projectID, companyID, cclaSignature.SignatureReferenceName, true)
		}

		if params.RemoveGitlabOrgApprovalList != nil {
			repo.updateApprovalTable(ctx, params.AddGitlabOrgApprovalList, utils.GitlabOrgApprovalCriteria, signatureID, projectID, companyID, cclaSignature.SignatureReferenceName, false)
			approvalList.Criteria = utils.GitlabOrgCriteria
			approvalList.ApprovalList = params.RemoveGitlabOrgApprovalList
			approvalList.Action = utils.RemoveApprovals
			approvalList.Version = claGroupModel.Version
			// Get repositories by CLAGroup
			repositories, getRepoByCLAGroupErr := repo.repositoriesRepo.GitHubGetRepositoriesByCLAGroup(ctx, projectID, true)
			if getRepoByCLAGroupErr != nil {
				msg := fmt.Sprintf("unable to fetch repositories for cla group ID: %s ", projectID)
				log.WithFields(f).WithError(getRepoByCLAGroupErr).Warn(msg)
				return nil, errors.New(msg)
			}
			var gitLabOrgRepositories []*models.GithubRepository
			var gitLabOrgs []*models.GithubOrganization
			for _, repository := range repositories {
				// Check for matching organization name in repositories table against approvalList removal gitlab organizations/groups
				if utils.StringInSlice(repository.RepositoryOrganizationName, approvalList.ApprovalList) {
					gitLabOrgRepositories = append(gitLabOrgRepositories, repository)
				}
			}

			for _, gitLabOrgRepo := range gitLabOrgRepositories {
				gitLabOrg, getGitlabOrgErr := repo.ghOrgRepo.GetGitHubOrganization(ctx, gitLabOrgRepo.RepositoryOrganizationName)
				if getGitlabOrgErr != nil {
					msg := fmt.Sprintf("unable to get gitlab organization by name: %s ", gitLabOrgRepo.RepositoryOrganizationName)
					log.WithFields(f).WithError(getGitlabOrgErr).Warn(msg)
					return nil, errors.New(msg)
				}
				gitLabOrgs = append(gitLabOrgs, gitLabOrg)
			}

			var gitLabUsernames []string
			for _, gitLabOrg := range gitLabOrgs {
				gitLabOrgUsers, getOrgMembersErr := github.GetOrganizationMembers(ctx, gitLabOrg.OrganizationName, gitLabOrg.OrganizationInstallationID)
				if getOrgMembersErr != nil {
					msg := fmt.Sprintf("unable to fetch gitLabOrgUsers for org: %s ", gitLabOrg.OrganizationName)
					log.WithFields(f).WithError(getOrgMembersErr).Warnf(msg)
					return nil, errors.New(msg)
				}
				gitLabUsernames = append(gitLabUsernames, gitLabOrgUsers...)
			}
			approvalList.GitlabUsernames = utils.RemoveDuplicates(gitLabUsernames)

			repo.invalidateSignatures(ctx, &approvalList, claManager, eventArgs)
		}
	}
	// Ensure at least one value is set for us to update
	if !haveAdditions {
		log.WithFields(f).Debugf("no updates required to any of the approved list values company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t - expecting at least something to update",
			companyID, projectID, true, true)
		return cclaSignature, nil
	}

	// Remove trailing comma from the expression, if present
	updateExpression = utils.TrimRemoveTrailingComma("SET " + updateExpression)

	// Update dynamoDB table
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(repo.signatureTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(cclaSignature.SignatureID),
			},
		},
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		UpdateExpression:          aws.String(updateExpression),
	}

	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("error updating approval lists for company ID: %s project ID: %s, type: ccla, signed: %t, approved: %t, error: %v",
			companyID, projectID, signed, approved, updateErr)
		return nil, updateErr
	}

	// Query the CCLA signature once again to load the most recent updates which include approval list updates from above
	updatedSig, err := repo.GetCorporateSignature(ctx, projectID, companyID, &approved, &signed)
	if err != nil || cclaSignature == nil {
		msg := fmt.Sprintf("unable to get corporate signature for CLA Group: %s and company: %s", projectID, companyID)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	// Just grab and use the first one - need to figure out conflict resolution if more than one
	return updatedSig, nil
}

func (repo *repository) updateApprovalTable(ctx context.Context, approvalList []string, criteria, signatureID, projectID, companyID, companyName string, add bool) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.addApprovalList",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	for _, item := range approvalList {
		log.WithFields(f).Debugf("adding approval request for item: %s with criteria: %s", item, criteria)
		approvalID, err := uuid.NewV4()
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("unable to generate UUID for email: %s", item)
			continue
		}
		_, currentTime := utils.CurrentTime()

		// Check if it exists
		approvalItems, apprErr := repo.approvalRepo.SearchApprovalList(criteria, item, projectID, companyID, signatureID)
		if apprErr != nil {
			log.WithFields(f).WithError(apprErr).Warnf("unable to search approval list for item: %s", item)
			continue
		}

		if len(approvalItems) > 0 {
			// Update the existing record
			approvalItem := approvalItems[0]
			if add {
				log.WithFields(f).Debugf("approval request for item: %s with criteria: %s already exists", item, criteria)
				approvalItem.DateModified = currentTime
				approvalItem.DateAdded = currentTime
				approvalItem.Active = true
			} else {
				log.WithFields(f).Debugf("approval request for item: %s with criteria: %s already exists", item, criteria)
				approvalItem.DateModified = currentTime
				approvalItem.DateRemoved = currentTime
				approvalItem.Active = false
			}
			err = repo.approvalRepo.UpdateApprovalItem(approvalItem)

			if err != nil {
				log.WithFields(f).WithError(err).Warnf("unable to update approval request for item: %s", item)
				continue
			}
			log.WithFields(f).Debugf("updated approval request for item: %s with criteria: %s", item, criteria)
			continue
		}

		// create a new record
		approvalItem := approvals.ApprovalItem{
			ApprovalID:          approvalID.String(),
			SignatureID:         signatureID,
			ApprovalName:        item,
			ProjectID:           projectID,
			CompanyID:           companyID,
			ApprovalCriteria:    criteria,
			DateCreated:         currentTime,
			DateModified:        currentTime,
			ApprovalCompanyName: companyName,
		}

		if add {
			approvalItem.Active = true
			approvalItem.DateAdded = currentTime
			approvalItem.Note = "Auto-Added"
		} else {
			approvalItem.Active = false
			approvalItem.DateRemoved = currentTime
			approvalItem.Note = "Auto-Removed"
		}

		err = repo.approvalRepo.AddApprovalList(approvalItem)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("unable to add approval request for item: %s", item)
			continue
		}
		log.WithFields(f).Debugf("added approval request for item: %s with criteria: %s", item, criteria)
	}
}

// sendEmail is a helper function used to render email for (CCLA, ICLA, ECLA cases)
func (repo repository) sendEmail(ctx context.Context, email string, approvalList *ApprovalList, iclas []*models.IclaSignature, eclas []*models.Signature) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.sendEmail",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	companyName := ""
	company, companyErr := repo.companyRepo.GetCompany(ctx, approvalList.CompanyID)
	if companyErr != nil {
		log.WithFields(f).Debugf("unable to get company")
	}
	if company != nil {
		companyName = company.CompanyName
	}

	params := InvalidateSignatureTemplateParams{
		Company:       companyName,
		RecipientName: email,
		ClaManager:    utils.GetBestUsername(approvalList.CLAManager),
		CLAManagers:   approvalList.ManagersInfo,
		CLAGroupName:  approvalList.ClaGroupName,
	}

	// check for signature type (CCLA, ICLA, ECLA)
	var removalType = ""

	// case 1 CCLA
	if len(iclas) == 0 && len(eclas) == 0 {
		removalType = CCLA
	} else if len(iclas) > 0 && len(eclas) == 0 {
		// case 2 ccla + icla
		removalType = CCLAICLA
	} else if len(iclas) > 0 && len(eclas) > 0 {
		// case 3 ccla + icla + ecla
		removalType = CCLAICLAECLA
	}

	// Send CCLA Email
	if removalType == CCLA {
		subject := fmt.Sprintf("EasyCLA: CCLA invalidated  for :%s ", approvalList.ClaGroupName)
		log.WithFields(f).Debugf("sending ccla invalidation email to :%s ", email)
		body, renderErr := utils.RenderTemplate(approvalList.Version, InvalidateCCLASignatureTemplateName, InvalidateCCLASignatureTemplate, params)
		if renderErr != nil {
			log.WithFields(f).Debugf("unable to render email approval template for user: %s ", email)
		} else {
			err := utils.SendEmail(subject, body, []string{email})
			if err != nil {
				log.WithFields(f).Debugf("unable to send approval list update email to : %s ", email)
			}
		}
	} else if removalType == ICLA {
		subject := fmt.Sprintf("EasyCLA: ICLA invalidated  for :%s ", approvalList.ClaGroupName)
		log.WithFields(f).Debugf("sending icla invalidation email to :%s ", email)
		body, renderErr := utils.RenderTemplate(approvalList.Version, InvalidateICLASignatureTemplateName, InvalidateICLASignatureTemplate, params)
		if renderErr != nil {
			log.WithFields(f).Debugf("unable to render email approval template for user: %s ", email)
		} else {
			err := utils.SendEmail(subject, body, []string{email})
			if err != nil {
				log.WithFields(f).Debugf("unable to send approval list update email to : %s ", email)
			}
		}
	} else if removalType == CCLAICLA {
		subject := fmt.Sprintf("EasyCLA: ICLA invalidated  for :%s ", approvalList.ClaGroupName)
		log.WithFields(f).Debugf("sending icla invalidation email to :%s ", email)
		body, renderErr := utils.RenderTemplate(approvalList.Version, InvalidateCCLAICLASignatureTemplateName, InvalidateCCLASignatureTemplate, params)
		if renderErr != nil {
			log.WithFields(f).Debugf("unable to render email approval template for user: %s ", email)
		} else {
			err := utils.SendEmail(subject, body, []string{email})
			if err != nil {
				log.WithFields(f).Debugf("unable to send approval list update email to : %s ", email)
			}
		}
	} else if removalType == CCLAICLAECLA {
		subject := fmt.Sprintf("EasyCLA: Employee Acknowledgement invalidated  for :%s ", approvalList.ClaGroupName)
		log.WithFields(f).Debugf("sending employee acknowledgement invalidation email to :%s ", email)
		body, renderErr := utils.RenderTemplate(approvalList.Version, InvalidateCCLAICLAECLASignatureTemplateName, InvalidateCCLAICLAECLASignatureTemplate, params)
		if renderErr != nil {
			log.WithFields(f).Debugf("unable to render email approval template for user: %s ", email)
		} else {
			err := utils.SendEmail(subject, body, []string{email})
			if err != nil {
				log.WithFields(f).Debugf("unable to send approval list update email to : %s ", email)
			}
		}
	}
}

// invalidateSignatures is a helper function that invalidates signature records based on approval list
func (repo repository) invalidateSignatures(ctx context.Context, approvalList *ApprovalList, claManager *models.User, eventArgs *events.LogEventArgs) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.invalidateSignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     &approvalList,
	}

	if approvalList.ICLAs != nil {
		var iclaWg sync.WaitGroup
		//Iterate iclas
		iclaWg.Add(len(approvalList.ICLAs))
		log.WithFields(f).Debug("invalidating signature icla records... ")
		for _, icla := range approvalList.ICLAs {
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
				// Map representing CLA types against email ....
				email := getBestEmail(user)
				// Log Event
				eventArgs.EventData = &events.SignatureInvalidatedApprovalRejectionEventData{
					SignatureID: icla.SignatureID,
					CLAManager:  claManager,
					CLAGroupID:  signature.ProjectID,
					Email:       email,
					GHUsername:  user.GithubUsername,
				}
				repo.eventsService.LogEventWithContext(ctx, eventArgs)
			}(icla)
		}
		iclaWg.Wait()
	}

	if approvalList.ECLAs != nil {
		var eclaWg sync.WaitGroup
		log.WithFields(f).Debug("invalidating signature ecla records... ")
		// Iterate eclas
		eclaWg.Add(len(approvalList.ECLAs))
		for _, ecla := range approvalList.ECLAs {
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
				// Log Event
				eventArgs.EventData = &events.SignatureInvalidatedApprovalRejectionEventData{
					SignatureID: ecla.SignatureID,
					CLAManager:  claManager,
					CLAGroupID:  ecla.ProjectID,
					Email:       email,
					GHUsername:  user.GithubUsername,
				}
				repo.eventsService.LogEventWithContext(ctx, eventArgs)
			}(ecla)
		}
		eclaWg.Wait()
	}
}

// verify UserApprovals checks user
func (repo repository) verifyUserApprovals(ctx context.Context, userID, signatureID string, claManager *models.User, approvalList *ApprovalList) (*models.User, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.verifyUserApprovals",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"userID":         userID,
	}

	user, err := repo.usersRepo.GetUser(userID)
	if err != nil {
		log.WithFields(f).Warnf("unable to get user record for ID: %s ", userID)
		return nil, err
	}
	email := getBestEmail(user)

	authUser := auth.User{
		Email:    claManager.LfEmail.String(),
		UserName: claManager.LfUsername,
	}

	if approvalList.Criteria == utils.EmailDomainCriteria {
		// Handle Domains
		log.WithFields(f).Debugf("Handling domain for user email: %s  with approval list: %+v ", email, approvalList.ApprovalList)
		domain := strings.Split(email, "@")[1]
		if utils.StringInSlice(domain, approvalList.ApprovalList) {
			if (!utils.StringInSlice(user.GithubUsername, approvalList.GitHubUsernameApprovals) || utils.StringInSlice(user.LfUsername, approvalList.GerritICLAECLAs)) && !utils.StringInSlice(email, approvalList.EmailApprovals) {
				//Invalidate record
				note := fmt.Sprintf("Signature invalidated (approved set to false) by %s due to %s  removal", utils.GetBestUsername(claManager), utils.EmailDomainCriteria)
				err := repo.InvalidateProjectRecord(ctx, signatureID, note)
				if err != nil {
					log.WithFields(f).Warnf("unable to invalidate record for signatureID: %s ", signatureID)
					return user, err
				}

				// Update Gerrit group users
				if utils.StringInSlice(user.LfUsername, approvalList.GerritICLAECLAs) {
					log.WithFields(f).Debugf("removing gerrit user:%s  from claGroup: %s ...", user.LfUsername, approvalList.ClaGroupID)
					iclaErr := repo.gerritService.RemoveUserFromGroup(ctx, &authUser, approvalList.ClaGroupID, user.LfUsername, utils.ClaTypeICLA)
					if iclaErr != nil {
						msg := fmt.Sprintf("unable to remove gerrit user:%s from group:%s", user.LfUsername, approvalList.ClaGroupID)
						log.WithFields(f).Warn(msg)
					}
					eclaErr := repo.gerritService.RemoveUserFromGroup(ctx, &authUser, approvalList.ClaGroupID, user.LfUsername, utils.ClaTypeECLA)
					if eclaErr != nil {
						msg := fmt.Sprintf("unable to remove gerrit user:%s from group:%s", user.LfUsername, approvalList.ClaGroupID)
						log.WithFields(f).Warn(msg)
					}
				}
			}
		}
	} else if approvalList.Criteria == utils.GitHubOrgCriteria {
		// Handle GH Org Approvals
		if utils.StringInSlice(user.GithubUsername, approvalList.GitHubUsernames) {
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
	} else if approvalList.Criteria == utils.GitHubUsernameCriteria || approvalList.Criteria == utils.EmailCriteria {
		note := fmt.Sprintf("Signature invalidated (approved set to false) by %s due to %s  removal", utils.GetBestUsername(claManager), approvalList.Criteria)
		err := repo.InvalidateProjectRecord(ctx, signatureID, note)
		if err != nil {
			log.WithFields(f).Warnf("unable to invalidate record for signatureID: %s ", signatureID)
			return user, err
		}

	}

	return user, nil
}

// removeColumn is a helper function to remove a given column when we need to zero out the column value - typically the approval list
func (repo repository) removeColumn(ctx context.Context, signatureID, columnName string) (*models.Signature, error) {
	f := logrus.Fields{
		"functionName": "v1.signatures.repository.removeColumn",
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
		UpdateExpression: aws.String("REMOVE #" + columnName),
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
		"functionName":            "v1.signatures.repository.AddSigTypeSignedApprovedID",
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
		"functionName":   "v1.signatures.repository.AddUserDetails",
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
		email = userModel.LfEmail.String()
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
	ue.AddAttributeName("#gh_username", SignatureUserGitHubUsername, userModel.GithubUsername != "")
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
		"functionName":   "v1.signatures.repository.AddSignedOn",
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

func (repo repository) GetClaGroupICLASignatures(ctx context.Context, claGroupID string, searchTerm *string, approved, signed *bool, pageSize int64, nextKey string, withExtraDetails bool) (*models.IclaSignatures, error) {
	f := logrus.Fields{
		"functionName":     "v1.signatures.repository.GetClaGroupICLASignatures",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"claGroupID":       claGroupID,
		"searchTerm":       utils.StringValue(searchTerm),
		"withExtraDetails": withExtraDetails,
	}

	condition := expression.Key("signature_project_id").Equal(expression.Value(claGroupID))

	var filter expression.ConditionBuilder
	var filterAdded bool
	filter = addAndCondition(filter, expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_reference_type").Equal(expression.Value(utils.SignatureReferenceTypeUser)), &filterAdded)
	filter = addAndCondition(filter, expression.Name("signature_user_ccla_company_id").AttributeNotExists(), &filterAdded)

	if approved != nil {
		f["approved"] = utils.BoolValue(approved)
		//log.WithFields(f).Debugf("adding filter signature_approved: %t", aws.BoolValue(approved))
		searchTermExpression := expression.Name("signature_approved").Equal(expression.Value(aws.BoolValue(approved)))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}
	if signed != nil {
		f["signed"] = utils.BoolValue(signed)
		//log.WithFields(f).Debugf("adding filter signature_signed: %t", aws.BoolValue(signed))
		searchTermExpression := expression.Name("signature_signed").Equal(expression.Value(aws.BoolValue(signed)))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}

	// If no query option was provided for approved and signed and our configuration default is to only show active signatures then we add the required query filters
	if approved == nil && signed == nil && config.GetConfig().SignatureQueryDefault == utils.SignatureQueryDefaultActive {
		filter = addAndCondition(filter, expression.Name("signature_approved").Equal(expression.Value(true)), &filterAdded)
		filter = addAndCondition(filter, expression.Name("signature_signed").Equal(expression.Value(true)), &filterAdded)
	}

	if searchTerm != nil {
		searchTermValue := utils.StringValue(searchTerm)
		f["searchTerm"] = searchTermValue
		log.WithFields(f).Debugf("adding search term filter for: '%s'", searchTermValue)
		searchTermExpression := expression.Name("signature_reference_name_lower").Contains(strings.ToLower(searchTermValue)).
			Or(expression.Name("user_email").Contains(strings.ToLower(searchTermValue))).
			Or(expression.Name("user_lf_username").Contains(strings.ToLower(searchTermValue))).
			Or(expression.Name("user_name").Contains(strings.ToLower(searchTermValue))).
			Or(expression.Name(SignatureUserGitHubUsername).Contains(strings.ToLower(searchTermValue))).
			Or(expression.Name("user_docusign_name").Contains(strings.ToLower(searchTermValue)))
		filter = addAndCondition(filter, searchTermExpression, &filterAdded)
	}

	//log.WithFields(f).Debugf("filter: %+v", filter)

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

	// If we have the next key, set the exclusive start key value
	if nextKey != "" {
		// log.WithFields(f).Debugf("Received a nextKey, value: %s", nextKey)
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

		intermediateResponse = append(intermediateResponse, repo.getIntermediateICLAResponse(f, dbSignatures)...)

		//log.WithFields(f).Debugf("LastEvaluatedKey: %+v", results.LastEvaluatedKey["signature_id"])
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

	var iclaSignatures []*models.IclaSignature
	if withExtraDetails {
		iclaSignatures, err = repo.addAdditionalICLAMetaData(f, intermediateResponse)
	} else {
		for _, sig := range intermediateResponse {
			iclaSignatures = append(iclaSignatures, sig.IclaSignature)
		}
	}
	if err != nil {
		return nil, err
	}

	out.List = iclaSignatures
	return out, nil
}

func (repo repository) GetICLAByDate(ctx context.Context, startDate string) ([]ItemSignature, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetICLAs",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"startDate":      startDate,
	}

	var signatures []ItemSignature

	log.WithFields(f).Debug("querying for icla signatures by date...")

	filter := expression.Name("date_created").GreaterThanEqual(expression.Value(startDate)).
		And(expression.Name("signature_type").Equal(expression.Value(utils.SignatureTypeCLA))).
		And(expression.Name("signature_signed").Equal(expression.Value(true))).
		And(expression.Name("signature_approved").Equal(expression.Value(true)))

	// Use the expression builder to create the expression
	expr, err := expression.NewBuilder().WithFilter(filter).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for query icla signatures by date: %v", err)
		return nil, err
	}

	var lastEvaluatedKey map[string]*dynamodb.AttributeValue

	for {
		scanInput := &dynamodb.ScanInput{
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			FilterExpression:          expr.Filter(),
			TableName:                 aws.String(repo.signatureTableName),
			ExclusiveStartKey:         lastEvaluatedKey,
		}

		result, err := repo.dynamoDBClient.Scan(scanInput)
		if err != nil {
			log.WithFields(f).Warnf("error retrieving icla signatures by date: %v", err)
			return nil, err
		}

		log.WithFields(f).Debugf("retrieved %d icla signatures by date", len(result.Items))

		var dbSignatures []ItemSignature

		unmarshallError := dynamodbattribute.UnmarshalListOfMaps(result.Items, &dbSignatures)
		if unmarshallError != nil {
			log.WithFields(f).Warnf("error unmarshalling icla signatures from database by date: %v", unmarshallError)
			return nil, unmarshallError
		}

		signatures = append(signatures, dbSignatures...)

		// log.WithFields(f).Debugf("last evaluated key: %+v", result.LastEvaluatedKey)

		if result.LastEvaluatedKey == nil {
			break
		}
		lastEvaluatedKey = result.LastEvaluatedKey
	}

	return signatures, nil
}

func (repo repository) getIntermediateICLAResponse(f logrus.Fields, dbSignatures []ItemSignature) []*iclaSignatureWithDetails {
	var intermediateResponse []*iclaSignatureWithDetails

	for _, sig := range dbSignatures {
		// Set the signed date/time
		var sigSignedTime string
		// Use the user docusign date signed value if it is present - older signatures do not have this
		if sig.UserDocusignDateSigned != "" {
			// Put the date into a standard format
			t, err := utils.ParseDateTime(sig.UserDocusignDateSigned)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("unable to parse signature docusign date signed time: %s", sig.UserDocusignDateSigned)
			} else {
				sigSignedTime = utils.TimeToString(t)
			}
		} else if sig.DateCreated != "" {
			// Put the date into a standard format
			t, err := utils.ParseDateTime(sig.DateCreated)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("unable to parse signature date created time: %s", sig.DateCreated)
			} else {
				sigSignedTime = utils.TimeToString(t)
			}
		}

		intermediateResponse = append(intermediateResponse, &iclaSignatureWithDetails{
			IclaSignature: &models.IclaSignature{
				GithubUsername:         sig.UserGithubUsername,
				GitlabUsername:         sig.UserGitlabUsername,
				UserID:                 sig.SignatureReferenceID,
				LfUsername:             sig.UserLFUsername,
				SignatureApproved:      sig.SignatureApproved,
				SignatureSigned:        sig.SignatureSigned,
				SignatureModified:      sig.DateModified,
				SignatureID:            sig.SignatureID,
				SignedOn:               sigSignedTime,
				UserDocusignDateSigned: sigSignedTime,
				UserDocusignName:       sig.UserDocusignName,
				UserEmail:              sig.UserEmail,
				UserName:               sig.UserName,
			},
			SignatureReferenceID: sig.SignatureReferenceID,
		})
	}

	return intermediateResponse
}

func (repo repository) addAdditionalICLAMetaData(f logrus.Fields, intermediateResponse []*iclaSignatureWithDetails) ([]*models.IclaSignature, error) {
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
				// If the GitHub username is empty, see if it was set in the user model
				if iclaSignatureWithDetails.IclaSignature.GithubUsername == "" {
					// Grab and set the GitHub username from the user model
					iclaSignatureWithDetails.IclaSignature.GithubUsername = userModel.GithubUsername
				}
				// If the GitLab username is empty, see if it was set in the user model
				if iclaSignatureWithDetails.IclaSignature.GitlabUsername == "" {
					// Grab and set the GitLab username from the user model
					iclaSignatureWithDetails.IclaSignature.GitlabUsername = userModel.GitlabUsername
				}
				// If the username is empty, see if it was set in the user model
				if iclaSignatureWithDetails.IclaSignature.UserName == "" {
					if userModel.Username != "" {
						// Grab and set the username
						iclaSignatureWithDetails.IclaSignature.UserName = userModel.Username
					} else if userModel.LfUsername != "" {
						iclaSignatureWithDetails.IclaSignature.UserName = userModel.LfUsername
					}
				}
				// If the user email is empty, see if it was set in the user model
				if iclaSignatureWithDetails.IclaSignature.UserEmail == "" {
					// Grab and set the email from the user record
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

func (repo repository) GetClaGroupCorporateContributors(ctx context.Context, claGroupID string, companyID *string, pageSize *int64, nextKey *string, searchTerm *string) (*models.CorporateContributorList, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.GetClaGroupCorporateContributors",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"companyID":      aws.StringValue(companyID),
	}

	totalCountChannel := make(chan int64, 1)
	go repo.getTotalCorporateContributorCount(ctx, claGroupID, companyID, searchTerm, totalCountChannel)

	condition := expression.Key("signature_project_id").Equal(expression.Value(claGroupID))
	// if companyID != nil {
	// 	sortKey := fmt.Sprintf("%s#%v#%v#%v", utils.ClaTypeECLA, true, true, *companyID)
	// 	condition = condition.And(expression.Key("sigtype_signed_approved_id").Equal(expression.Value(sortKey)))
	// } else {
	// 	sortKeyPrefix := fmt.Sprintf("%s#%v#%v", utils.ClaTypeECLA, true, true)
	// 	condition = condition.And(expression.Key("sigtype_signed_approved_id").BeginsWith(sortKeyPrefix))
	// }
	filter := expression.Name("signature_user_ccla_company_id").Equal(expression.Value(companyID))
	// filter = filter.And(expression.Name("signature_type").Equal(expression.Value(utils.ClaTypeECLA)))

	// Create our builder
	builder := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection()).WithFilter(filter)

	if searchTerm != nil {
		searchTermValue := utils.StringValue(searchTerm)
		f["searchTerm"] = searchTermValue
		log.WithFields(f).Debugf("adding search term filter for: '%s'", searchTermValue)
		builder.WithFilter(expression.Name("signature_reference_name_lower").Contains(strings.ToLower(searchTermValue)).
			Or(expression.Name("user_email").Contains(strings.ToLower(searchTermValue))).
			Or(expression.Name("github_username").Contains(strings.ToLower(searchTermValue))).
			Or(expression.Name("userDocusignName").Contains(strings.ToLower(searchTermValue))))
	}

	// Use the builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for get cla group icla signatures, claGroupID: %s, error: %v",
			claGroupID, err)
		return nil, err
	}

	// If the page size is nil, set it to the default
	if pageSize == nil {
		pageSize = aws.Int64(10)
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String(SignatureProjectIDIndex),
		Limit:                     aws.Int64(*pageSize),
	}

	if nextKey != nil {
		log.WithFields(f).Debugf("adding next key to query input: %s", *nextKey)
		queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"signature_project_id": {
				S: aws.String(claGroupID),
			},
			"signature_id": {
				S: aws.String(*nextKey),
			},
		}
	}

	out := &models.CorporateContributorList{List: make([]*models.CorporateContributor, 0)}
	var lastEvaluatedKey string

	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		log.WithFields(f).Debug("querying signatures...")
		results, queryErr := repo.dynamoDBClient.Query(queryInput)
		if queryErr != nil {
			log.WithFields(f).Warnf("error retrieving ecla signatures for project: %s, error: %v", claGroupID, queryErr)
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

			signatureVersion := fmt.Sprintf("v%s.%s", strconv.Itoa(sig.SignatureDocumentMajorVersion), strconv.Itoa(sig.SignatureDocumentMinorVersion))

			sigName := sig.UserName
			user, userErr := repo.usersRepo.GetUser(sig.SignatureReferenceID)
			if userErr != nil {
				log.WithFields(f).Warnf("unable to get user for id: %s, error: %v ", sig.SignatureReferenceID, userErr)
			}
			if user != nil && sigName == "" {
				sigName = user.Username
			}

			out.List = append(out.List, &models.CorporateContributor{
				SignatureID:            sig.SignatureID,
				GithubID:               sig.UserGithubUsername,
				LinuxFoundationID:      sig.UserLFUsername,
				Name:                   sigName,
				SignatureVersion:       signatureVersion,
				Email:                  sig.UserEmail,
				Timestamp:              sigCreatedTime,
				UserDocusignName:       sig.UserDocusignName,
				UserDocusignDateSigned: sigSignedTime,
				SignatureModified:      sig.DateModified,
				SignatureApproved:      sig.SignatureApproved,
				SignatureSigned:        sig.SignatureSigned,
			})
		}

		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			lastEvaluatedKey = ""
		}

		if int64(len(out.List)) >= *pageSize {
			break
		}

	}
	sort.Slice(out.List, func(i, j int) bool {
		return out.List[i].Name < out.List[j].Name
	})

	out.ResultCount = int64(len(out.List))
	out.TotalCount = <-totalCountChannel
	out.NextKey = lastEvaluatedKey

	return out, nil
}

func (repo repository) getTotalCorporateContributorCount(ctx context.Context, claGroupID string, companyID, searchTerm *string, totalCountChannel chan int64) {
	f := logrus.Fields{
		"functionName":   "v1.signature.repository.getTotalCorporateContributorCount",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"companyID":      companyID,
		"searchTerm":     searchTerm,
	}

	pageSize := int64(HugePageSize)
	f["pageSize"] = pageSize

	condition := expression.Key("signature_project_id").Equal(expression.Value(claGroupID))

	filter := expression.Name("signature_user_ccla_company_id").Equal(expression.Value(companyID)).And(expression.Name("signature_approved").Equal(expression.Value(true))).And(expression.Name("signature_signed").Equal(expression.Value(true)))

	builder := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter)

	if searchTerm != nil {
		searchTermValue := *searchTerm
		builder = builder.WithFilter(expression.Name("user_name").Contains(strings.ToLower(searchTermValue)).
			Or(expression.Name("user_email").Contains(strings.ToLower(searchTermValue))).
			Or(expression.Name("github_username").Contains(strings.ToLower(searchTermValue))).
			Or(expression.Name("userDocusignName").Contains(strings.ToLower(searchTermValue))))
	}

	beforeQuery, _ := utils.CurrentTime()
	log.WithFields(f).Debugf("running total signature count query for claGroupID: %s, companyID: %s", claGroupID, *companyID)

	expr, err := builder.WithProjection(buildCountProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for cla group: %s, error: %v", claGroupID, err)
		totalCountChannel <- 0
		return
	}

	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String(SignatureProjectIDIndex),
		Limit:                     aws.Int64(pageSize),
	}

	var lastEvaluatedKey string
	var totalCount int64

	// Loop until we have all the records - we'll get a nil lastEvaluatedKey when we're done
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		results, errQuery := repo.dynamoDBClient.QueryWithContext(ctx, queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error querying signatures for cla group: %s, error: %v", claGroupID, errQuery)
			totalCountChannel <- 0
			return
		}

		// Add the count to the total
		totalCount += *results.Count

		// Set the last evaluated key
		if results.LastEvaluatedKey["signature_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["signature_id"].S
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			lastEvaluatedKey = ""
		}
	}

	log.WithFields(f).Debugf("total signature count query took: %s", time.Since(beforeQuery))

	totalCountChannel <- totalCount

}

// EclaAutoCreate this routine updates the CCLA signature record by adjusting the auto_create_ecla column to the specified value
func (repo repository) EclaAutoCreate(ctx context.Context, signatureID string, autoCreateECLA bool) error {
	f := logrus.Fields{
		"functionName":   "v1.signature.repository.EclaAutoCreate",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
		"autoCreateECLA": autoCreateECLA,
	}

	// Build the expression
	expressionUpdate := expression.Set(expression.Name("auto_create_ecla"), expression.Value(autoCreateECLA))

	expr, err := expression.NewBuilder().WithUpdate(expressionUpdate).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for signature: %s, error: %v", signatureID, err)
		return err
	}

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
		ConditionExpression: expr.KeyCondition(),
		TableName:           aws.String(repo.signatureTableName),
		UpdateExpression:    expr.Update(),
	}

	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("error updating signature: %s, error: %v", signatureID, updateErr)
		return updateErr
	}

	return nil
}

// ActivateSignature used to activate signature again, in case of deactivated signature found
func (repo repository) ActivateSignature(ctx context.Context, signatureID string) error {
	f := logrus.Fields{
		"functionName":   "v1.signature.repository.ActivateSignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
	}

	// Build the expression
	expressionUpdate := expression.Set(expression.Name("signature_approved"), expression.Value(true)).Set(expression.Name("signature_signed"), expression.Value(false))

	expr, err := expression.NewBuilder().WithUpdate(expressionUpdate).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for signature: %s, error: %v", signatureID, err)
		return err
	}

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Key: map[string]*dynamodb.AttributeValue{
			"signature_id": {
				S: aws.String(signatureID),
			},
		},
		ConditionExpression: expr.KeyCondition(),
		TableName:           aws.String(repo.signatureTableName),
		UpdateExpression:    expr.Update(),
	}

	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("error updating signature: %s, error: %v", signatureID, updateErr)
		return updateErr
	}
	return nil
}

// getGerritUsers is a helper function to fetch the list of gerrit users for the specified type - results are returned through the specified results channel
func (repo repository) getGerritUsers(ctx context.Context, authUser *auth.User, projectSFID string, claType string, gerritResultChannel chan *GerritUserResponse) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.repository.getGerritUsers",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
	}
	log.WithFields(f).Debugf("querying gerrit for %s gerrit users...", claType)
	gerritIclaUsers, getGerritQueryErr := repo.gerritService.GetUsersOfGroup(ctx, authUser, projectSFID, claType)
	if getGerritQueryErr != nil || gerritIclaUsers == nil {
		msg := fmt.Sprintf("unable to fetch gerrit users for claGroup: %s , claType: %s ", projectSFID, claType)
		log.WithFields(f).WithError(getGerritQueryErr).Warn(msg)
		gerritResultChannel <- &GerritUserResponse{
			gerritGroupResponse: nil,
			queryType:           claType,
			Error:               errors.New(msg),
		}
		return
	}

	log.WithFields(f).Debugf("retrieved %d gerrit users for CLA type: %s...", len(gerritIclaUsers.Members), claType)
	gerritResultChannel <- &GerritUserResponse{
		gerritGroupResponse: gerritIclaUsers,
		queryType:           claType,
		Error:               nil,
	}
}

func buildNextKey(indexName string, signature *models.Signature) (string, error) {
	nextKey := make(map[string]*dynamodb.AttributeValue)
	nextKey["signature_id"] = &dynamodb.AttributeValue{S: aws.String(signature.SignatureID)}
	switch indexName {
	// TODO: review all these use-cases
	case SignatureProjectIDIndex:
		nextKey["signature_project_id"] = &dynamodb.AttributeValue{S: aws.String(signature.ProjectID)}
	case SignatureProjectDateIDIndex:
		nextKey["signature_project_id"] = &dynamodb.AttributeValue{S: aws.String(signature.ProjectID)}
		nextKey["date_modified"] = &dynamodb.AttributeValue{S: aws.String(signature.SignatureModified)}
	case SignatureProjectReferenceIndex:
		nextKey["signature_project_id"] = &dynamodb.AttributeValue{S: aws.String(signature.ProjectID)}
		nextKey["signature_reference_id"] = &dynamodb.AttributeValue{S: aws.String(signature.SignatureReferenceID)}
	case SignatureProjectIDSigTypeSignedApprovedIDIndex:
		nextKey["signature_project_id"] = &dynamodb.AttributeValue{S: aws.String(signature.ProjectID)}
		nextKey["signature_signed_approved_id"] = &dynamodb.AttributeValue{S: aws.String(fmt.Sprintf("%t#%t", signature.SignatureSigned, signature.SignatureApproved))}
	case SignatureProjectIDTypeIndex:
		nextKey["signature_project_id"] = &dynamodb.AttributeValue{S: aws.String(signature.ProjectID)}
		nextKey["signature_type"] = &dynamodb.AttributeValue{S: aws.String(signature.SignatureType)}
	case SignatureReferenceIndex:
		nextKey["signature_reference_id"] = &dynamodb.AttributeValue{S: aws.String(signature.SignatureReferenceID)}
	case SignatureReferenceSearchIndex:
		nextKey["signature_project_id"] = &dynamodb.AttributeValue{S: aws.String(signature.ProjectID)}
		nextKey["signature_reference_name_lower"] = &dynamodb.AttributeValue{S: aws.String(signature.SignatureReferenceNameLower)}
	}

	return encodeNextKey(nextKey)
}

// encodeNextKey encodes the map as a string
func encodeNextKey(in map[string]*dynamodb.AttributeValue) (string, error) {
	if len(in) == 0 {
		return "", nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// decodeNextKey decodes the next key value into a dynamodb attribute value
func decodeNextKey(str string) (map[string]*dynamodb.AttributeValue, error) {
	f := logrus.Fields{
		"functionName": "v1.events.repository.decodeNextKey",
	}

	sDec, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error decoding string %s", str)
		return nil, err
	}

	var m map[string]*dynamodb.AttributeValue
	err = json.Unmarshal(sDec, &m)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error unmarshalling string after decoding: %s", sDec)
		return nil, err
	}

	return m, nil
}
