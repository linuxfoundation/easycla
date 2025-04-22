// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/user"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/strfmt"

	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/gofrs/uuid"
)

const (
	SignatureReferenceIndex = "reference-signature-index"
)

// IRepository interface methods
type IRepository interface { //nolint
	CreateCompany(ctx context.Context, in *models.Company) (*models.Company, error)
	GetCompanies(ctx context.Context) (*models.Companies, error)
	GetCompany(ctx context.Context, companyID string) (*models.Company, error)
	GetCompanyByExternalID(ctx context.Context, companySFID string) (*models.Company, error)
	GetCompaniesByExternalID(ctx context.Context, companySFID string, includeChildCompanies bool) ([]*models.Company, error)
	GetCompanyBySigningEntityName(ctx context.Context, signingEntityName string) (*models.Company, error)
	GetCompanyByName(ctx context.Context, companyName string) (*models.Company, error)
	SearchCompanyByName(ctx context.Context, companyName string, nextKey string) (*models.Companies, error)
	DeleteCompanyByID(ctx context.Context, companyID string) error
	DeleteCompanyBySFID(ctx context.Context, companySFID string) error
	GetCompaniesByUserManager(ctx context.Context, userID string, userModel user.User) (*models.Companies, error)
	GetCompaniesByUserManagerWithInvites(ctx context.Context, userID string, userModel user.User) (*models.CompaniesWithInvites, error)
	AddPendingCompanyInviteRequest(ctx context.Context, companyID string, userModel user.User) (*Invite, error)
	GetCompanyInviteRequest(ctx context.Context, companyInviteID string) (*Invite, error)
	GetCompanyInviteRequests(ctx context.Context, companyID string, status *string) ([]Invite, error)
	GetCompanyUserInviteRequests(ctx context.Context, companyID string, userID string) (*Invite, error)
	GetUserInviteRequests(ctx context.Context, userID string) ([]Invite, error)
	ApproveCompanyAccessRequest(ctx context.Context, companyInviteID string) error
	RejectCompanyAccessRequest(ctx context.Context, companyInviteID string) error
	UpdateCompanyAccessList(ctx context.Context, companyID string, companyACL []string) error
	IsCCLAEnabledForCompany(ctx context.Context, companyID string) (bool, error)
}

type repository struct {
	stage                   string
	dynamoDBClient          *dynamodb.DynamoDB
	companyTableName        string
	signatureTableName      string
	companyInvitesTableName string
}

// NewRepository creates a new company repository instance
func NewRepository(awsSession *session.Session, stage string) IRepository {
	return repository{
		stage:                   stage,
		dynamoDBClient:          dynamodb.New(awsSession),
		companyTableName:        fmt.Sprintf("cla-%s-companies", stage),
		signatureTableName:      fmt.Sprintf("cla-%s-signatures", stage),
		companyInvitesTableName: fmt.Sprintf("cla-%s-company-invites", stage),
	}
}

// GetCompanies retrieves all the companies
func (repo repository) GetCompanies(ctx context.Context) (*models.Companies, error) {
	f := logrus.Fields{
		"functionName":   "company.repository.GetCompanies",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithProjection(buildCompanyProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for get all companies scan error: %v", err)
		return nil, err
	}

	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyTableName),
	}

	var lastEvaluatedKey string
	var companies []models.Company

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, dbErr := repo.dynamoDBClient.Scan(scanInput)
		if dbErr != nil {
			log.WithFields(f).Warnf("error retrieving get all companies, error: %v", dbErr)
			return nil, dbErr
		}

		// Convert the list of DB models to a list of response models
		companyList, modelErr := buildCompanyModels(ctx, results)
		if modelErr != nil {
			log.WithFields(f).Warnf("error retrieving get all companies, error: %v", modelErr)
			return nil, modelErr
		}

		// Add to our response model list
		companies = append(companies, companyList...)

		if results.LastEvaluatedKey["company_id"] != nil {
			//log.Debugf("LastEvaluatedKey: %+v", result.LastEvaluatedKey["signature_id"])
			lastEvaluatedKey = *results.LastEvaluatedKey["company_id"].S
			scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"company_id": {
					S: aws.String(lastEvaluatedKey),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}
	}

	// How many total records do we have - may not be up-to-date as this value is updated only periodically
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: &repo.companyTableName,
	}

	describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving total company record count, error: %v", err)
		return nil, err
	}

	totalCount := *describeTableResult.Table.ItemCount

	return &models.Companies{
		ResultCount:    int64(len(companies)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Companies:      companies,
	}, nil
}

// GetCompanyByExternalID returns a company based on the company external ID
func (repo repository) GetCompanyByExternalID(ctx context.Context, companySFID string) (*models.Company, error) {
	f := logrus.Fields{
		"functionName":   "company.repository.GetCompanyByExternalID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companySFID":    companySFID,
	}

	const includeChildCompanies = false // Include child/other signing entity name records?
	companyRecords, err := repo.GetCompaniesByExternalID(ctx, companySFID, includeChildCompanies)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to unmarshall response from the database")
		return nil, err
	}
	log.WithFields(f).Debugf("loaded %d records", len(companyRecords))

	if len(companyRecords) == 0 {
		log.WithFields(f).Debug("no records found")
		return nil, &utils.CompanyNotFound{
			Message:   "no company records found for SFID",
			CompanyID: companySFID,
		}
	}

	return companyRecords[0], nil
}

// GetCompaniesByExternalID returns a list of companies based on the company external ID. A company will have more than one if/when the SF record has multiple entity names - for which we create separate EasyCLA company records
func (repo repository) GetCompaniesByExternalID(ctx context.Context, companySFID string, includeChildCompanies bool) ([]*models.Company, error) {
	f := logrus.Fields{
		"functionName":          "company.repository.GetCompaniesByExternalID",
		utils.XREQUESTID:        ctx.Value(utils.XREQUESTID),
		"companySFID":           companySFID,
		"includeChildCompanies": includeChildCompanies,
	}

	condition := expression.Key("company_external_id").Equal(expression.Value(companySFID))
	builder := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildCompanyProjection())
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to build query expression")
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.companyTableName),
		IndexName:                 aws.String("external-company-index"),
	}

	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error retrieving company using company_external_id")
		return nil, err
	}

	if len(results.Items) == 0 {
		log.WithFields(f).Debug("no company records found")
		return nil, &utils.CompanyNotFound{
			Message:     "no company records found with matching external SFID",
			CompanySFID: companySFID,
		}
	}

	var dbCompanyModels []DBModel
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &dbCompanyModels)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to unmarshall response from the database")
		return nil, err
	}

	log.WithFields(f).Debug("converting database records to a response model...")
	return dbModelsToResponseModels(ctx, dbCompanyModels, includeChildCompanies)
}

// GetCompanyBySigningEntityName search the company by signing entity name
func (repo repository) GetCompanyBySigningEntityName(ctx context.Context, signingEntityName string) (*models.Company, error) {
	f := logrus.Fields{
		"functionName":      "company.repository.GetCompanyBySigningEntityName",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"signingEntityName": signingEntityName,
	}
	condition := expression.Key("signing_entity_name").Equal(expression.Value(signingEntityName))
	builder := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildCompanyProjection())
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
		TableName:                 aws.String(repo.companyTableName),
		IndexName:                 aws.String("company-signing-entity-name-index"),
	}

	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving company using signing_entity_name. error = %s", err.Error())
		return nil, err
	}

	if len(results.Items) == 0 {
		return nil, &utils.CompanyNotFound{
			Message:                  "no company with signing entity name found",
			CompanySigningEntityName: signingEntityName,
			Err:                      nil,
		}
	}

	dbCompanyModel := DBModel{}
	err = dynamodbattribute.UnmarshalMap(results.Items[0], &dbCompanyModel)
	if err != nil {
		return nil, err
	}

	return dbCompanyModel.toModel()
}

// GetCompanyByName searches the database and returns the matching company names
func (repo repository) GetCompanyByName(ctx context.Context, companyName string) (*models.Company, error) {
	f := logrus.Fields{
		"functionName":   "company.repository.GetCompanyByName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyName":    companyName,
	}
	// This is the key we want to match
	condition := expression.Key("company_name").Equal(expression.Value(companyName))

	// Use the builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildCompanyProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for company query, companyName: %s, error: %v",
			companyName, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyTableName),
		IndexName:                 aws.String("company-name-index"),
	}

	// Make the DynamoDB Query API call
	results, queryErr := repo.dynamoDBClient.Query(queryInput)
	if queryErr != nil {
		log.WithFields(f).Warnf("error retrieving company by companyName: %s, error: %+v", companyName, queryErr)
		return nil, queryErr
	}

	// Didn't find it...
	if *results.Count == 0 {
		log.WithFields(f).Debugf("Company query by name returned no results using companyName: %s", companyName)
		return nil, nil
	}

	// Found it...
	var dbModels []DBModel
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &dbModels)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling db company, error: %+v", err)
		return nil, err
	}
	// TODO: DAD - review projection and unmarshalling logic, the 'note' column is not being loaded into the data model
	//log.Debugf("DB response model: %#v", dbModels)

	return toSwaggerModel(&dbModels[0])
}

// GetCompany returns a company based on the company ID
func (repo repository) GetCompany(ctx context.Context, companyID string) (*models.Company, error) {
	f := logrus.Fields{
		"functionName":   "company.repository.GetCompany",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
	}
	companyTableData, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(repo.companyTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"company_id": {
				S: aws.String(companyID),
			},
		},
	})

	if err != nil {
		log.WithFields(f).Warnf("error fetching company table data using company id: %s, error: %v", companyID, err)
		return nil, err
	}

	if len(companyTableData.Item) == 0 {
		return nil, &utils.CompanyNotFound{
			Message:   "no company matching company record",
			CompanyID: companyID,
		}
	}

	dbCompanyModel := DBModel{}
	err = dynamodbattribute.UnmarshalMap(companyTableData.Item, &dbCompanyModel)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling company table data, error: %v", err)
		return nil, err
	}

	return dbCompanyModel.toModel()
}

// SearchCompanyByName locates companies by the matching name and return any potential matches
func (repo repository) SearchCompanyByName(ctx context.Context, companyName string, nextKey string) (*models.Companies, error) {
	f := logrus.Fields{
		"functionName":   "company.repository.SearchCompanyByName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyName":    companyName,
		"nextKey":        nextKey,
	}

	// Sorry, no results if empty company name
	if strings.TrimSpace(companyName) == "" {
		return &models.Companies{
			Companies:      []models.Company{},
			LastKeyScanned: "",
			ResultCount:    0,
			SearchTerms:    companyName,
			TotalCount:     0,
		}, nil
	}

	// This is the company name we want to match
	filter := expression.Name("company_name").Contains(companyName)

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithFilter(filter).WithProjection(buildCompanyProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for company scan, companyName: %s, error: %v",
			companyName, err)
		return nil, err
	}

	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyTableName),
	}

	// If we have the next key, set the exclusive start key value
	if nextKey != "" {
		log.WithFields(f).Debugf("Received a nextKey, value: %s", nextKey)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"company_id": {
				S: aws.String(nextKey),
			},
		}
	}

	//log.Debugf("Running company search scan using queryInput: %+v", scanInput)

	var lastEvaluatedKey string
	var companies []models.Company

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, dbErr := repo.dynamoDBClient.Scan(scanInput)
		if dbErr != nil {
			log.Warnf("error retrieving companies for search term: %s, error: %v", companyName, dbErr)
			return nil, dbErr
		}

		// Convert the list of DB models to a list of response models
		companyList, modelErr := buildCompanyModels(ctx, results)
		if modelErr != nil {
			log.WithFields(f).Warnf("error retrieving companies for companyName %s in ACL, error: %v", companyName, modelErr)
			return nil, modelErr
		}

		// Add to our response model list
		companies = append(companies, companyList...)

		if results.LastEvaluatedKey["company_id"] != nil {
			//log.Debugf("LastEvaluatedKey: %+v", result.LastEvaluatedKey["signature_id"])
			lastEvaluatedKey = *results.LastEvaluatedKey["company_id"].S
			scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"company_id": {
					S: aws.String(lastEvaluatedKey),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}
	}

	// How many total records do we have - may not be up-to-date as this value is updated only periodically
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: &repo.companyTableName,
	}

	describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving total company record count for companyName: %s, error: %v", companyName, err)
		return nil, err
	}

	totalCount := *describeTableResult.Table.ItemCount

	return &models.Companies{
		ResultCount:    int64(len(companies)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Companies:      companies,
	}, nil
}

// DeleteCompanyByID deletes the company by ID
func (repo repository) DeleteCompanyByID(ctx context.Context, companyID string) error {
	f := logrus.Fields{
		"functionName":   "company.repository.DeleteCompanyByID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
	}
	log.WithFields(f).Debug("deleting company by ID")
	_, err := repo.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"company_id": {S: aws.String(companyID)},
		},
		TableName: aws.String(repo.companyTableName),
	})

	if err != nil {
		return err
	}

	return nil
}

// DeleteCompanyBySFID deletes the company by SFID
func (repo repository) DeleteCompanyBySFID(ctx context.Context, companySFID string) error {
	f := logrus.Fields{
		"functionName":   "company.repository.DeleteCompanyBySFID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companySFID":    companySFID,
	}

	log.WithFields(f).Debug("deleting company by SFID...")
	_, err := repo.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"company_external_id": {S: aws.String(companySFID)},
		},
		TableName: aws.String(repo.companyTableName),
	})

	if err != nil {
		return err
	}

	return nil
}

// GetCompanyUserManager the get a list of companies when provided the company id and user manager
func (repo repository) GetCompaniesByUserManager(ctx context.Context, userID string, userModel user.User) (*models.Companies, error) {
	f := logrus.Fields{
		"functionName":   "company.repository.GetCompaniesByUserManager",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"userID":         userID,
		"userModel":      userModel,
	}

	// Sorry, no results if empty user ID
	if strings.TrimSpace(userID) == "" {
		return &models.Companies{
			Companies:      []models.Company{},
			LastKeyScanned: "",
			ResultCount:    0,
			TotalCount:     0,
		}, nil
	}

	// This is the user name we want to match
	var filter expression.ConditionBuilder
	if userModel.LFUsername != "" {
		filter = expression.Name("company_acl").Contains(userModel.LFUsername)
	} else if userModel.UserName != "" {
		filter = expression.Name("company_acl").Contains(userModel.UserName)
	} else {
		log.WithFields(f).Warnf("unable to query user with no LF username or username in their data model - user iD: %s.", userID)
		return &models.Companies{
			Companies:      []models.Company{},
			LastKeyScanned: "",
			ResultCount:    0,
			TotalCount:     0,
		}, nil
	}

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithFilter(filter).WithProjection(buildCompanyProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for company scan, userID %s in ACL, error: %v", userID, err)
		return nil, err
	}

	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyTableName),
	}

	//log.Debugf("Running company search scan using queryInput: %+v", scanInput)
	var lastEvaluatedKey string
	var companies []models.Company

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, dbErr := repo.dynamoDBClient.Scan(scanInput)
		if dbErr != nil {
			log.WithFields(f).Warnf("error retrieving companies for userID %s in ACL, error: %v", userID, dbErr)
			return nil, dbErr
		}

		// Convert the list of DB models to a list of response models
		companyList, modelErr := buildCompanyModels(ctx, results)
		if modelErr != nil {
			log.WithFields(f).Warnf("error retrieving companies for userID %s in ACL, error: %v", userID, modelErr)
			return nil, modelErr
		}

		// Add to our response model list
		companies = append(companies, companyList...)

		if results.LastEvaluatedKey["company_invite_id"] != nil {
			//log.Debugf("LastEvaluatedKey: %+v", result.LastEvaluatedKey["signature_id"])
			lastEvaluatedKey = *results.LastEvaluatedKey["company_invite_id"].S
			scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"company_invite_id": {
					S: aws.String(lastEvaluatedKey),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}
	}

	// How many total records do we have - may not be up-to-date as this value is updated only periodically
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: &repo.companyTableName,
	}

	describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving total company record count, error: %v", err)
		return nil, err
	}

	totalCount := *describeTableResult.Table.ItemCount

	return &models.Companies{
		ResultCount:    int64(len(companies)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Companies:      companies,
	}, nil
}

// IsCCLAEnabled returns true if company is enabled for CCLA
func (repo repository) IsCCLAEnabledForCompany(ctx context.Context, companyID string) (bool, error) {
	f := logrus.Fields{
		"functionName":   "v1.signature.repository.IsCCLAEnabled",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
	}

	// Build the query
	condition := expression.Key("signature_reference_id").Equal(expression.Value(companyID))

	filter := expression.Name("signature_signed").Equal(expression.Value(true)).And(expression.Name("signature_approved").Equal(expression.Value(true))).And(expression.Name("signature_type").Equal(expression.Value("ccla")))

	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithFilter(filter).Build()

	if err != nil {
		log.WithFields(f).Warnf("error building expression for company: %s, error: %v", companyID, err)
		return false, err
	}

	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.signatureTableName),
		IndexName:                 aws.String(SignatureReferenceIndex),
	}

	results, queryErr := repo.dynamoDBClient.QueryWithContext(ctx, queryInput)
	if queryErr != nil {
		log.WithFields(f).Warnf("error querying signatures for company: %s, error: %v", companyID, queryErr)
		return false, queryErr
	}

	if *results.Count > 0 {
		log.WithFields(f).Debugf("company: %s is enabled for CCLA", companyID)
		return true, nil
	}

	return false, nil

}

// GetCompanyUserManagerWithInvites the get a list of companies including status when provided the company id and user manager
func (repo repository) GetCompaniesByUserManagerWithInvites(ctx context.Context, userID string, userModel user.User) (*models.CompaniesWithInvites, error) {
	f := logrus.Fields{
		"functionName":   "company.repository.GetCompaniesByUserManagerWithInvites",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"userID":         userID,
		"userModel":      userModel,
	}

	companies, err := repo.GetCompaniesByUserManager(ctx, userID, userModel)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving companies for userID %s in ACL, error: %v", userID, err)
		return nil, err
	}

	// Query the invites table for list of invitations for this user
	invites, err := repo.GetUserInviteRequests(ctx, userID)
	if err != nil {
		log.WithFields(f).Warnf("error retrieving companies invites for userID %s, error: %v", userID, err)
		return nil, err
	}

	return repo.buildCompaniesByUserManagerWithInvites(ctx, companies, invites), nil
}

func (repo repository) buildCompaniesByUserManagerWithInvites(ctx context.Context, companies *models.Companies, invites []Invite) *models.CompaniesWithInvites {
	f := logrus.Fields{
		"functionName":   "company.repository.buildCompaniesByUserManagerWithInvites",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	companiesWithInvites := models.CompaniesWithInvites{
		ResultCount: int64(len(companies.Companies) + len(invites)),
		TotalCount:  companies.TotalCount + int64(len(invites)),
	}

	var companyWithInvite []models.CompanyWithInvite
	for _, company := range companies.Companies {
		companyWithInvite = append(companyWithInvite, models.CompanyWithInvite{
			CompanyName:       company.CompanyName,
			CompanyID:         company.CompanyID,
			CompanyExternalID: company.CompanyExternalID,
			CompanyACL:        company.CompanyACL,
			Created:           company.Created,
			Updated:           company.Updated,
			Status:            "Joined",
		})
	}

	for _, invite := range invites {
		company, err := repo.GetCompany(ctx, invite.RequestedCompanyID)
		if err != nil {
			log.WithFields(f).Warnf("error retrieving company with company ID %s, error: %v - skipping invite", company, err)
			continue
		}

		// Default status is pending if there's a record but no status
		if invite.Status == "" {
			invite.Status = StatusPending
		}

		companyWithInvite = append(companyWithInvite, models.CompanyWithInvite{
			CompanyName: company.CompanyName,
			CompanyID:   company.CompanyID,
			CompanyACL:  company.CompanyACL,
			Created:     company.Created,
			Updated:     company.Updated,
			Status:      invite.Status,
		})
	}

	companiesWithInvites.CompaniesWithInvites = companyWithInvite

	return &companiesWithInvites
}

// buildCompanyModels converts the response model into a response data model
func buildCompanyModels(ctx context.Context, results *dynamodb.ScanOutput) ([]models.Company, error) {
	f := logrus.Fields{
		"functionName":   "company.repository.buildCompanyModels",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	var companies []models.Company

	type ItemSignature struct {
		CompanyID         string   `json:"company_id"`
		CompanyName       string   `json:"company_name"`
		SigningEntityName string   `json:"signing_entity_name"`
		CompanyACL        []string `json:"company_acl"`
		CompanyExternalID string   `json:"company_external_id"`
		Created           string   `json:"date_created"`
		Note              string   `json:"note"`
		IsEmbargoed       bool     `json:"is_embargoed"`
		Modified          string   `json:"date_modified"`
	}

	// The DB company model
	var dbCompanies []ItemSignature

	err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &dbCompanies)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling companies from database, error: %v", err)
		return nil, err
	}

	now, _ := utils.CurrentTime()

	for _, dbCompany := range dbCompanies {
		createdDateTime, err := utils.ParseDateTime(dbCompany.Created)
		if err != nil {
			log.WithFields(f).Warnf("Unable to parse company created date time: %s, error: %v - using current time",
				dbCompany.Created, err)
			createdDateTime = now
		}

		modifiedDateTime, err := utils.ParseDateTime(dbCompany.Modified)
		if err != nil {
			log.WithFields(f).Warnf("Unable to parse company modified date time: %s, error: %v - using current time",
				dbCompany.Created, err)
			modifiedDateTime = now
		}

		if dbCompany.SigningEntityName == "" {
			dbCompany.SigningEntityName = dbCompany.CompanyName
		}

		companies = append(companies, models.Company{
			CompanyACL:        dbCompany.CompanyACL,
			CompanyID:         dbCompany.CompanyID,
			CompanyName:       dbCompany.CompanyName,
			SigningEntityName: dbCompany.SigningEntityName,
			CompanyExternalID: dbCompany.CompanyExternalID,
			Created:           strfmt.DateTime(createdDateTime),
			Note:              dbCompany.Note,
			IsEmbargoed:       dbCompany.IsEmbargoed,
			Updated:           strfmt.DateTime(modifiedDateTime),
		})
	}

	return companies, nil
}

// GetCompanyInviteRequest returns the specified request
func (repo repository) GetCompanyInviteRequest(ctx context.Context, companyInviteID string) (*Invite, error) {
	f := logrus.Fields{
		"functionName":    "company.repository.GetCompanyInviteRequest",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"companyInviteID": companyInviteID,
	}

	condition := expression.Key("company_invite_id").Equal(expression.Value(companyInviteID))

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildInvitesProjection()).Build()

	if err != nil {
		log.WithFields(f).Warnf("error building expression for company invites, invite ID: %s, error: %v", companyInviteID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyInvitesTableName),
	}

	queryResults, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("Unable to query the company invite based on invite ID: %s, error: %v", companyInviteID, err)
		return nil, err
	}

	var companyInvites []Invite
	err = dynamodbattribute.UnmarshalListOfMaps(queryResults.Items, &companyInvites)
	if err != nil || companyInvites == nil {
		log.WithFields(f).Warnf("unable to unmarshall the company invite based on invite ID: %s, error: %v", companyInviteID, err)
		return nil, err
	}
	if len(companyInvites) == 0 {
		log.WithFields(f).Warnf("unable to locate the company invite based on invite ID: %s, error: %v", companyInviteID, err)
		return nil, nil
	}

	return &companyInvites[0], nil
}

// GetCompanyInviteRequests returns a list of company invites when provided the company ID
func (repo repository) GetCompanyInviteRequests(ctx context.Context, companyID string, status *string) ([]Invite, error) {
	f := logrus.Fields{
		"functionName":   "company.repository.GetCompanyInviteRequests",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"status":         aws.StringValue(status),
	}

	// These are the keys we want to match
	condition := expression.Key("requested_company_id").Equal(expression.Value(companyID))

	// Use the nice builder to create the expression
	builder := expression.NewBuilder().
		WithKeyCondition(condition).
		WithProjection(buildInvitesProjection())

	if status != nil {
		builder.WithFilter(expression.Name("status").Equal(expression.Value(*status)))
	}

	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for company invite query, companyID: %s, error: %v",
			companyID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyInvitesTableName),
		IndexName:                 aws.String("requested-company-index"), // Name of a secondary index
	}

	companyInviteAV, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("Unable to retrieve data from Company-Invites table, error: %v", err)
		return nil, err
	}

	var companyInvites []Invite
	err = dynamodbattribute.UnmarshalListOfMaps(companyInviteAV.Items, &companyInvites)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling company invite data, error: %v", err)
		return nil, err
	}

	return companyInvites, nil
}

// GetCompanyUserInviteRequests returns a list of company invites when provided the company ID and user ID
func (repo repository) GetCompanyUserInviteRequests(ctx context.Context, companyID string, userID string) (*Invite, error) {
	f := logrus.Fields{
		"functionName":   "company.repository.GetCompanyUserInviteRequests",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"userID":         userID,
	}

	// These are the keys we want to match
	condition := expression.Key("requested_company_id").Equal(expression.Value(companyID))
	filter := expression.Name("user_id").Equal(expression.Value(userID))

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().
		WithKeyCondition(condition).
		WithFilter(filter).
		WithProjection(buildInvitesProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for company scan, companyID: %s with userID: %s, error: %v",
			companyID, userID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyInvitesTableName),
		IndexName:                 aws.String("requested-company-index"), // Name of a secondary index
	}

	queryResults, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.WithFields(f).Warnf("Unable to retrieve data from Company-Invites table using company id: %s and user id: %s, error: %v", companyID, userID, err)
		return nil, err
	}

	var companyInvites []Invite
	err = dynamodbattribute.UnmarshalListOfMaps(queryResults.Items, &companyInvites)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling company invite data using company id: %s and user id: %s, error: %v",
			companyID, userID, err)
		return nil, err
	}

	if len(companyInvites) == 0 {
		log.WithFields(f).Debugf("Unable to find company invite for company id: %s and user id: %s", companyID, userID)
		return nil, nil
	}

	if len(companyInvites) > 1 {
		log.WithFields(f).Warnf("Company invite should have one result, found: %d for company id: %s and user id: %s",
			len(companyInvites), companyID, userID)
	}

	return &companyInvites[0], nil
}

// GetUserInviteRequests returns a list of company invites when provided the user ID
func (repo repository) GetUserInviteRequests(ctx context.Context, userID string) ([]Invite, error) {
	f := logrus.Fields{
		"functionName":   "company.repository.GetUserInviteRequests",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"userID":         userID,
	}

	filter := expression.Name("user_id").Equal(expression.Value(userID))

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().
		WithFilter(filter).
		WithProjection(buildInvitesProjection()).Build()
	if err != nil {
		log.WithFields(f).Warnf("error building expression for company scan with userID: %s, error: %v", userID, err)
		return nil, err
	}

	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyInvitesTableName),
	}

	var lastEvaluatedKey string
	var companyInvites []Invite

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {

		queryResults, err := repo.dynamoDBClient.Scan(scanInput)
		if err != nil {
			log.WithFields(f).Warnf("Unable to retrieve data from Company-Invites table using user id: %s, error: %v", userID, err)
			return nil, err
		}

		var companyInvitesList []Invite
		err = dynamodbattribute.UnmarshalListOfMaps(queryResults.Items, &companyInvitesList)
		if err != nil {
			log.WithFields(f).Warnf("error unmarshalling company invite data using user id: %s, error: %v", userID, err)
			return nil, err
		}

		// Add to our response model
		companyInvites = append(companyInvites, companyInvitesList...)

		// Determine if we have more records - if so, update the start key and loop again
		if queryResults.LastEvaluatedKey["company_invite_id"] != nil {
			//log.Debugf("LastEvaluatedKey: %+v", result.LastEvaluatedKey["signature_id"])
			lastEvaluatedKey = *queryResults.LastEvaluatedKey["company_invite_id"].S
			scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"company_invite_id": {
					S: aws.String(lastEvaluatedKey),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}
	}

	return companyInvites, nil
}

// AddPendingCompanyInviteRequest adds a pending company invite when provided the company ID and user ID
func (repo repository) AddPendingCompanyInviteRequest(ctx context.Context, companyID string, userModel user.User) (*Invite, error) {
	f := logrus.Fields{
		"functionName":       "company.repository.AddPendingCompanyInviteRequest",
		utils.XREQUESTID:     ctx.Value(utils.XREQUESTID),
		"companyID":          companyID,
		"UserID":             userModel.UserID,
		"UserName":           userModel.UserName,
		"UserGitHubID":       userModel.UserGithubID,
		"UserGitHubUsername": userModel.UserGithubUsername,
		"LFUsername":         userModel.LFUsername,
	}

	// First, let's check if we already have a previous invite for this company and user ID pair
	previousInvite, err := repo.GetCompanyUserInviteRequests(ctx, companyID, userModel.UserID)
	if err != nil {
		log.WithFields(f).Warnf("Previous invite already exists for company id: %s and user: %s, error: %v",
			companyID, userModel.UserID, err)
		return nil, err
	}

	// We we already have an invite...don't create another one
	if previousInvite != nil {
		// Update rejected invite request
		if previousInvite.Status == "rejected" {
			updateErr := repo.updateInviteRequestStatus(ctx, previousInvite.CompanyInviteID, "pending")
			if updateErr != nil {
				return nil, updateErr
			}
			return previousInvite, nil
		}
		log.WithFields(f).Warnf("Invite already exists for company id: %s and user: %s - skipping creation",
			companyID, userModel.UserID)
		return previousInvite, nil
	}

	companyInviteID, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).Warnf("Unable to generate a UUID for a pending invite, error: %v", err)
		return nil, err
	}

	_, now := utils.CurrentTime()

	attributes := map[string]*dynamodb.AttributeValue{
		"company_invite_id": {
			S: aws.String(companyInviteID.String()),
		},
		"requested_company_id": {
			S: aws.String(companyID),
		},
		"user_id": {
			S: aws.String(userModel.UserID),
		},
		"status": {
			S: aws.String("pending"),
		},
		"date_created": {
			S: aws.String(now),
		},
		"date_modified": {
			S: aws.String(now),
		},
	}

	// Add a few more fields, if they are available
	if userModel.UserName != "" {
		attributes["user_name"] = &dynamodb.AttributeValue{S: aws.String(userModel.UserName)}
	}
	if userModel.UserGithubID != "" {
		attributes["user_github_id"] = &dynamodb.AttributeValue{S: aws.String(userModel.UserGithubID)}
	}
	if userModel.UserGithubUsername != "" {
		attributes["user_github_username"] = &dynamodb.AttributeValue{S: aws.String(userModel.UserGithubUsername)}
	}
	if userModel.LFUsername != "" {
		attributes["user_lf_user_name"] = &dynamodb.AttributeValue{S: aws.String(userModel.LFUsername)}
	}
	if userModel.UserName != "" {
		attributes["user_name"] = &dynamodb.AttributeValue{S: aws.String(userModel.UserName)}
	}

	input := &dynamodb.PutItemInput{
		Item:      attributes,
		TableName: aws.String(fmt.Sprintf("cla-%s-company-invites", repo.stage)),
	}

	_, err = repo.dynamoDBClient.PutItem(input)
	if err != nil {
		log.WithFields(f).Warnf("Unable to create a new pending invite, error: %v", err)
		return nil, err
	}

	createdInvite, err := repo.GetCompanyInviteRequest(ctx, companyInviteID.String())
	if err != nil || createdInvite == nil {
		log.WithFields(f).Warnf("Unable to query newly created company invite id: %s, error: %v",
			companyInviteID.String(), err)
		return nil, err
	}

	return createdInvite, nil
}

// ApproveCompanyAccessRequest approves the specified company invite
func (repo repository) ApproveCompanyAccessRequest(ctx context.Context, companyInviteID string) error {
	return repo.updateInviteRequestStatus(ctx, companyInviteID, "approved")
}

// RejectCompanyInviteRequest rejects the specified company invite
func (repo repository) RejectCompanyAccessRequest(ctx context.Context, companyInviteID string) error {
	return repo.updateInviteRequestStatus(ctx, companyInviteID, "rejected")
}

// updateInviteRequestStatus updates the specified invite with the specified status
func (repo repository) updateInviteRequestStatus(ctx context.Context, companyInviteID, status string) error {
	f := logrus.Fields{
		"functionName":   "company.repository.updateInviteRequestStatus",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	// First, let's check if we already have a previous invite
	inviteModel, err := repo.GetCompanyInviteRequest(ctx, companyInviteID)
	if err != nil || inviteModel == nil {
		log.WithFields(f).Warnf("ApproveCompanyAccessRequest - unable to locate previous invite, error: %v",
			err)
		return err
	}

	_, now := utils.CurrentTime()

	input := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"company_invite_id": {
				S: aws.String(companyInviteID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#C": aws.String("requested_company_id"),
			"#U": aws.String("user_id"),
			"#S": aws.String("status"),
			"#M": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":c": {
				S: aws.String(inviteModel.RequestedCompanyID),
			},
			":u": {
				S: aws.String(inviteModel.UserID),
			},
			":s": {
				S: aws.String(status),
			},
			":m": {
				S: aws.String(now),
			},
		},
		UpdateExpression: aws.String("SET #C = :c, #U = :u, #S = :s, #M = :m"),
		TableName:        aws.String(fmt.Sprintf("cla-%s-company-invites", repo.stage)),
	}

	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).Warnf("ApproveCompanyAccessRequest - unable to update request with approved status, error: %v",
			updateErr)
		return updateErr
	}

	return nil
}

// UpdateCompanyAccessList updates the company ACL when provided the company ID and ACL list
func (repo repository) UpdateCompanyAccessList(ctx context.Context, companyID string, companyACL []string) error {
	f := logrus.Fields{
		"functionName":   "company.repository.UpdateCompanyAccessList",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"companyACL":     strings.Join(companyACL, ","),
	}
	_, now := utils.CurrentTime()

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#S": aws.String("company_acl"),
			"#M": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":s": {
				SS: aws.StringSlice(companyACL),
			},
			":m": {
				S: aws.String(now),
			},
		},
		TableName: aws.String(repo.companyTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"company_id": {
				S: aws.String(companyID),
			},
		},
		UpdateExpression: aws.String("SET #S = :s, #M = :m"),
	}

	_, err := repo.dynamoDBClient.UpdateItem(input)
	if err != nil {
		log.WithFields(f).Warnf("Error updating Company Access List, error: %v", err)
		return err
	}

	return nil
}

// CreateCompany creates a new company record
func (repo repository) CreateCompany(ctx context.Context, in *models.Company) (*models.Company, error) {
	f := logrus.Fields{
		"functionName":      "company.repository.CreateCompany",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"companyName":       in.CompanyName,
		"signingEntityName": in.SigningEntityName,
		"isEmbargoed":       in.IsEmbargoed,
		"companySFID":       in.CompanyExternalID,
	}

	// Don't create duplicates - check to see if any exist
	existingModel, queryErr := repo.GetCompanyByName(ctx, in.CompanyName)
	if queryErr != nil {
		log.WithFields(f).WithError(queryErr).Warn("problem querying for existing company record by name")
		return nil, queryErr
	}
	// Already exists - don't re-create
	if existingModel != nil {
		return existingModel, nil
	}

	companyID, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).Warnf("Unable to generate a UUID for a pending invite, error: %v", err)
		return nil, err
	}
	f["companyID"] = companyID

	_, now := utils.CurrentTime()
	comp := &DBModel{
		CompanyID:         companyID.String(),
		CompanyName:       in.CompanyName,
		CompanyExternalID: in.CompanyExternalID,
		SigningEntityName: in.SigningEntityName,
		IsEmbargoed:       in.IsEmbargoed,
		Created:           now,
		Updated:           now,
		Version:           "v1",
	}

	// Use the company name if signing entity name is not provided
	if in.SigningEntityName == "" {
		comp.SigningEntityName = in.CompanyName
	}
	if in.CompanyACL != nil {
		comp.CompanyACL = in.CompanyACL
	}
	if in.CompanyManagerID != "" {
		comp.CompanyManagerID = in.CompanyManagerID
	}
	if in.Note != "" {
		comp.Note = in.Note
	}

	av, err := dynamodbattribute.MarshalMap(&comp)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem marshing company record")
		return nil, err
	}
	_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(repo.companyTableName),
	})
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem creating new company")
		return nil, err
	}

	log.WithFields(f).Debugf("company created %#v\n", comp)
	return comp.toModel()
}
