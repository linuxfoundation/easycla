// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
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
	IsSanctioned      bool     `dynamodbav:"is_sanctioned" json:"is_sanctioned"`
	SanctionOrigin    string   `dynamodbav:"sanction_origin" json:"sanction_origin,omitempty"`
	SanctionedDate    string   `dynamodbav:"sanctioned_date" json:"sanctioned_date,omitempty"`
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

// formatSanctionedDate normalizes the stored date - this backend writes RFC3339, the legacy one
// pynamo format - and leaves an unset value empty rather than warning on it.
func formatSanctionedDate(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	return utils.FormatTimeString(dateStr)
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
		IsSanctioned:      dbCompanyModel.IsSanctioned,
		SanctionOrigin:    dbCompanyModel.SanctionOrigin,
		SanctionedDate:    formatSanctionedDate(dbCompanyModel.SanctionedDate),
		Version:           dbCompanyModel.Version,
	}, nil
}

// dbModelsToResponseModels converts DB rows to swagger models. includeChildCompanies=true
// returns every row (the org lens). includeChildCompanies=false returns exactly one
// deterministic "parent" row: duplicate rows per SFID are a known data issue (separate merge
// runbook), so ties are broken by oldest date_created then smallest company_id, and are logged.
func dbModelsToResponseModels(ctx context.Context, dbModels []DBModel, includeChildCompanies bool) ([]*models.Company, error) {
	f := logrus.Fields{
		"functionName":          "company.models.dbModelsToResponseModels",
		utils.XREQUESTID:        ctx.Value(utils.XREQUESTID),
		"includeChildCompanies": includeChildCompanies,
	}

	var companyModels []*models.Company
	var all, candidates []*models.Company
	var err error
	for _, dbModel := range dbModels {
		respModel, conversionErr := dbModel.toModel()
		if conversionErr != nil {
			log.WithFields(f).WithError(conversionErr).Warn("unable to convert db model to company model")
			err = conversionErr
			continue
		}
		if includeChildCompanies {
			companyModels = append(companyModels, respModel)
			continue
		}
		all = append(all, respModel)
		// only a candidate if company is not a signing entity name with a different name
		if respModel.SigningEntityName == "" || respModel.CompanyName == respModel.SigningEntityName {
			candidates = append(candidates, respModel)
		}
	}
	if includeChildCompanies {
		return companyModels, err
	}
	if len(all) == 0 {
		return companyModels, err // every row (if any) failed conversion
	}

	pick := candidates
	if len(pick) == 0 {
		// Rows exist for this SFID, just none look like a bare "parent" record - still a real
		// company (the org lens lists these same rows), so fall back across all of them instead
		// of reporting company-not-found.
		log.WithFields(f).Warnf("no parent-like company record for this SFID - falling back across %d signing-entity row(s)", len(all))
		pick = all
	}
	winner := pick[0]
	for _, c := range pick[1:] {
		if companyIsOlder(c, winner) {
			winner = c
		}
	}
	if len(pick) > 1 {
		ids := make([]string, 0, len(pick))
		for _, c := range pick {
			ids = append(ids, c.CompanyID)
		}
		log.WithFields(f).WithField("candidateCompanyIDs", ids).Warn("multiple company records share this external SFID - picked deterministically, remaining need a data merge")
	}
	return []*models.Company{winner}, nil
}

// companyIsOlder reports whether c predates winner, tie-breaking on the smaller company_id so
// repeated calls return the same row.
func companyIsOlder(c, winner *models.Company) bool {
	ct, wt := time.Time(c.Created), time.Time(winner.Created)
	if !ct.Equal(wt) {
		return ct.Before(wt)
	}
	return c.CompanyID < winner.CompanyID
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
		IsSanctioned:      dbCompanyModel.IsSanctioned,
		SanctionOrigin:    dbCompanyModel.SanctionOrigin,
		SanctionedDate:    formatSanctionedDate(dbCompanyModel.SanctionedDate),
		CompanyExternalID: dbCompanyModel.CompanyExternalID,
		CompanyManagerID:  dbCompanyModel.CompanyManagerID,
		Created:           strfmt.DateTime(createdDateTime),
		Updated:           strfmt.DateTime(updateDateTime),
		Note:              dbCompanyModel.Note,
		Version:           dbCompanyModel.Version,
	}, nil
}
