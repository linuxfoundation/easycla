// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/strfmt"
	"github.com/sirupsen/logrus"
)

// DBModel data model
type DBModel struct {
	CompanyID         string   `dynamodbav:"company_id" json:"company_id"`
	CompanyName       string   `dynamodbav:"company_name" json:"company_name"`
	SigningEntityName string   `dynamodbav:"signing_entity_name" json:"signing_entity_name"`
	CompanyACL        []string `dynamodbav:"company_acl" json:"company_acl"`
	CompanyExternalID string   `dynamodbav:"company_external_id" json:"company_external_id"`
	CompanyManagerID  string   `dynamodbav:"company_manager_id" json:"company_manager_id"`
	Created           string   `dynamodbav:"date_created" json:"date_created"`
	Updated           string   `dynamodbav:"date_modified" json:"date_modified"`
	Note              string   `dynamodbav:"note" json:"note"`
	IsEmbargoed       bool     `dynamodbav:"is_embargoed" json:"is_embargoed"`
	Version           string   `dynamodbav:"version" json:"version"`
}

// Invite data model
type Invite struct {
	CompanyInviteID    string `dynamodbav:"company_invite_id" json:"company_invite_id"`
	RequestedCompanyID string `dynamodbav:"requested_company_id" json:"requested_company_id"`
	UserID             string `dynamodbav:"user_id" json:"user_id"`
	Status             string `dynamodbav:"status" json:"status"`
	Created            string `dynamodbav:"date_created" json:"date_created"`
	Updated            string `dynamodbav:"date_modified" json:"date_modified"`
	Note               string `dynamodbav:"note" json:"note"`
	Version            string `dynamodbav:"version" json:"version"`
}

// InviteModel data model
type InviteModel struct {
	CompanyInviteID    string `json:"company_invite_id"`
	RequestedCompanyID string `json:"requested_company_id"`
	CompanyName        string `json:"company_name"`
	UserID             string `json:"user_id"`
	UserName           string `json:"user_name"`
	UserEmail          string `json:"user_email"`
	Status             string `json:"status"`
	Created            string `json:"date_created"`
	Updated            string `json:"date_modified"`
	Note               string `json:"note"`
	Version            string `json:"version"`
}

// toModel is a helper routine to convert the (internal) database model to a (public) swagger model
func (dbCompanyModel *DBModel) toModel() (*models.Company, error) {
	// Convert the "string" date time
	createdDateTime, err := utils.ParseDateTime(dbCompanyModel.Created)
	if err != nil {
		log.Warnf("Error converting created date time for company: %s, error: %v", dbCompanyModel.CompanyID, err)
		return nil, err
	}
	updateDateTime, err := utils.ParseDateTime(dbCompanyModel.Updated)
	if err != nil {
		log.Warnf("Error converting updated date time for company: %s, error: %v", dbCompanyModel.CompanyID, err)
		return nil, err
	}

	// For backwards compatibility, if the signing entity name is missing, use the company name
	signingEntityName := dbCompanyModel.SigningEntityName
	if signingEntityName == "" {
		signingEntityName = dbCompanyModel.CompanyName
	}

	// Convert the local DB model to a public swagger model
	return &models.Company{
		CompanyACL:        dbCompanyModel.CompanyACL,
		CompanyID:         dbCompanyModel.CompanyID,
		CompanyName:       dbCompanyModel.CompanyName,
		SigningEntityName: signingEntityName,
		CompanyExternalID: dbCompanyModel.CompanyExternalID,
		CompanyManagerID:  dbCompanyModel.CompanyManagerID,
		Created:           strfmt.DateTime(createdDateTime),
		Updated:           strfmt.DateTime(updateDateTime),
		Note:              dbCompanyModel.Note,
		IsEmbargoed:       dbCompanyModel.IsEmbargoed,
		Version:           dbCompanyModel.Version,
	}, nil
}

// dbModelsToResponseModels is a helper routine to convert the (internal) database model to a (public) swagger model
func dbModelsToResponseModels(ctx context.Context, dbModels []DBModel, includeChildCompanies bool) ([]*models.Company, error) {
	f := logrus.Fields{
		"functionName":          "company.models.dbModelsToResponseModels",
		utils.XREQUESTID:        ctx.Value(utils.XREQUESTID),
		"includeChildCompanies": includeChildCompanies,
	}

	var companyModels []*models.Company
	var err error
	for _, dbModel := range dbModels {
		respModel, conversionErr := dbModel.toModel()
		if conversionErr != nil {
			log.WithFields(f).WithError(conversionErr).Warn("unable to convert db model to company model")
			err = conversionErr
		} else {
			// log.WithFields(f).Debugf("Converted %+v to %+v", dbModel, respModel)
			if includeChildCompanies {
				companyModels = append(companyModels, respModel)
			} else {
				// only include if company is not a signing entity name with different name
				if respModel.SigningEntityName == "" || respModel.CompanyName == respModel.SigningEntityName {
					companyModels = append(companyModels, respModel)
					break // no need to continue
				}
			}
		}
	}

	return companyModels, err
}

// toModel is a helper routine to convert the (internal) database model to a (public) swagger model
func toSwaggerModel(dbCompanyModel *DBModel) (*models.Company, error) {
	// Convert the "string" date time
	createdDateTime, err := utils.ParseDateTime(dbCompanyModel.Created)
	if err != nil {
		log.Warnf("Error converting created date time for company: %s, error: %v", dbCompanyModel.CompanyID, err)
		return nil, err
	}
	updateDateTime, err := utils.ParseDateTime(dbCompanyModel.Updated)
	if err != nil {
		log.Warnf("Error converting updated date time for company: %s, error: %v", dbCompanyModel.CompanyID, err)
		return nil, err
	}

	if dbCompanyModel.SigningEntityName == "" {
		dbCompanyModel.SigningEntityName = dbCompanyModel.CompanyName
	}

	// Convert the local DB model to a public swagger model
	return &models.Company{
		CompanyACL:        dbCompanyModel.CompanyACL,
		CompanyID:         dbCompanyModel.CompanyID,
		CompanyName:       dbCompanyModel.CompanyName,
		SigningEntityName: dbCompanyModel.SigningEntityName,
		IsEmbargoed:       dbCompanyModel.IsEmbargoed,
		CompanyExternalID: dbCompanyModel.CompanyExternalID,
		CompanyManagerID:  dbCompanyModel.CompanyManagerID,
		Created:           strfmt.DateTime(createdDateTime),
		Updated:           strfmt.DateTime(updateDateTime),
		Note:              dbCompanyModel.Note,
		Version:           dbCompanyModel.Version,
	}, nil
}
