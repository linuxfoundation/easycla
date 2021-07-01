// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"

// RepositoryDBModel represent repositories table
type RepositoryDBModel struct {
	DateCreated                string `dynamodbav:"date_created" json:"date_created,omitempty"`
	DateModified               string `dynamodbav:"date_modified" json:"date_modified,omitempty"`
	RepositoryExternalID       string `dynamodbav:"repository_external_id" json:"repository_external_id,omitempty"`
	RepositoryID               string `dynamodbav:"repository_id" json:"repository_id,omitempty"`
	RepositoryName             string `dynamodbav:"repository_name" json:"repository_name,omitempty"`
	RepositoryOrganizationName string `dynamodbav:"repository_organization_name" json:"repository_organization_name,omitempty"`
	RepositoryProjectID        string `dynamodbav:"repository_project_id" json:"repository_project_id,omitempty"`
	RepositorySfdcID           string `dynamodbav:"repository_sfdc_id" json:"repository_sfdc_id,omitempty"`
	RepositoryType             string `dynamodbav:"repository_type" json:"repository_type,omitempty"`
	RepositoryURL              string `dynamodbav:"repository_url" json:"repository_url,omitempty"`
	ProjectSFID                string `dynamodbav:"project_sfid" json:"project_sfid,omitempty"`
	Enabled                    bool   `dynamodbav:"enabled" json:"enabled"`
	Note                       string `dynamodbav:"note" json:"note,omitempty"`
	Version                    string `dynamodbav:"version" json:"version,omitempty"`
}

func convertModels(dbModels []*RepositoryDBModel) []*models.GithubRepository {
	var responseModels []*models.GithubRepository
	for _, dbModel := range dbModels {
		responseModels = append(responseModels, dbModel.toModel())

	}
	return responseModels
}

func (gr *RepositoryDBModel) toModel() *models.GithubRepository {
	return &models.GithubRepository{
		DateCreated:                gr.DateCreated,
		DateModified:               gr.DateModified,
		RepositoryExternalID:       gr.RepositoryExternalID,
		RepositoryID:               gr.RepositoryID,
		RepositoryName:             gr.RepositoryName,
		RepositoryOrganizationName: gr.RepositoryOrganizationName,
		RepositoryProjectID:        gr.RepositoryProjectID,
		RepositorySfdcID:           gr.RepositorySfdcID,
		RepositoryType:             gr.RepositoryType,
		RepositoryURL:              gr.RepositoryURL,
		ProjectSFID:                gr.ProjectSFID,
		Enabled:                    gr.Enabled,
		Note:                       gr.Note,
		Version:                    gr.Version,
	}
}
