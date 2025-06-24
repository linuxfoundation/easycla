// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"

// CLAManagerRequests data model
type CLAManagerRequests struct {
	Requests []CLAManagerRequest
}

// CLAManagerRequest data model
type CLAManagerRequest struct {
	RequestID         string `json:"request_id"`
	CompanyID         string `json:"company_id"`
	CompanyExternalID string `json:"company_external_id"`
	CompanyName       string `json:"company_name"`
	ProjectID         string `json:"project_id"`
	ProjectExternalID string `json:"project_external_id"`
	ProjectName       string `json:"project_name"`
	UserID            string `json:"user_id"`
	UserExternalID    string `json:"user_external_id"`
	UserName          string `json:"user_name"`
	UserEmail         string `json:"user_email"`
	Status            string `json:"status"`
	Created           string `json:"date_created"`
	Updated           string `json:"date_modified"`
}

// dbModelToServiceModel converts a database model to a service model
func dbModelToServiceModel(dbModel CLAManagerRequest) models.ClaManagerRequest {
	return models.ClaManagerRequest{
		RequestID:         dbModel.RequestID,
		CompanyID:         dbModel.CompanyID,
		CompanyExternalID: dbModel.CompanyExternalID,
		CompanyName:       dbModel.CompanyName,
		ProjectID:         dbModel.ProjectID,
		ProjectExternalID: dbModel.ProjectExternalID,
		ProjectName:       dbModel.ProjectName,
		UserID:            dbModel.UserID,
		UserExternalID:    dbModel.UserExternalID,
		UserName:          dbModel.UserName,
		UserEmail:         dbModel.UserEmail,
		Status:            dbModel.Status,
		Created:           dbModel.Created,
		Updated:           dbModel.Updated,
	}
}
