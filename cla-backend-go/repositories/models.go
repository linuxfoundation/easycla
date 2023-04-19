// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"strconv"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// RepositoryDBModel represent repositories table
type RepositoryDBModel struct {
	DateCreated                string `dynamodbav:"date_created" json:"date_created,omitempty"`
	DateModified               string `dynamodbav:"date_modified" json:"date_modified,omitempty"`
	RepositoryExternalID       string `dynamodbav:"repository_external_id" json:"repository_external_id,omitempty"` // Integer value from GitHub
	RepositoryID               string `dynamodbav:"repository_id" json:"repository_id,omitempty"`
	RepositoryName             string `dynamodbav:"repository_name" json:"repository_name,omitempty"`
	RepositoryFullPath         string `dynamodbav:"repository_full_path" json:"repository_full_path,omitempty"`
	RepositoryOrganizationName string `dynamodbav:"repository_organization_name" json:"repository_organization_name,omitempty"`
	RepositoryCLAGroupID       string `dynamodbav:"repository_project_id" json:"repository_project_id,omitempty"`
	RepositorySfdcID           string `dynamodbav:"repository_sfdc_id" json:"repository_sfdc_id,omitempty"`
	RepositoryType             string `dynamodbav:"repository_type" json:"repository_type,omitempty"`
	RepositoryURL              string `dynamodbav:"repository_url" json:"repository_url,omitempty"`
	ProjectSFID                string `dynamodbav:"project_sfid" json:"project_sfid,omitempty"`
	Enabled                    bool   `dynamodbav:"enabled" json:"enabled"`
	Note                       string `dynamodbav:"note" json:"note,omitempty"`
	Version                    string `dynamodbav:"version" json:"version,omitempty"`
	IsRemoteDeleted            bool   `dynamodbav:"is_remote_deleted" json:"is_transfered,omitempty"`
	WasCLAEnforced             bool   `dynamodbav:"was_cla_enforced" json:"was_cla_enforced,omitempty"`
}

func convertModels(dbModels []*RepositoryDBModel) []*models.GithubRepository {
	var responseModels []*models.GithubRepository
	for _, dbModel := range dbModels {
		// Apply condition, don't return repositories which are remotely deleted.
		if !dbModel.IsRemoteDeleted {
			responseModels = append(responseModels, dbModel.ToGitHubModel())
		}
	}
	return responseModels
}

// ToGitHubModel returns the database model to a GitHub repository model suitable for marshalling to the client
func (gr *RepositoryDBModel) ToGitHubModel() *models.GithubRepository {
	gitLabExternalID, err := strconv.ParseInt(gr.RepositoryExternalID, 10, 64)
	if err != nil {
		log.WithError(err).Warnf("unable to convert repository external ID to an int64 value: %s", gr.RepositoryExternalID)
		return nil
	}

	return &models.GithubRepository{
		DateCreated:                gr.DateCreated,
		DateModified:               gr.DateModified,
		RepositoryExternalID:       gitLabExternalID,
		RepositoryID:               gr.RepositoryID,
		RepositoryName:             gr.RepositoryName,
		RepositoryOrganizationName: gr.RepositoryOrganizationName,
		RepositoryProjectSfid:      gr.ProjectSFID,
		RepositoryType:             gr.RepositoryType,
		RepositoryURL:              gr.RepositoryURL,
		RepositoryClaGroupID:       gr.RepositoryCLAGroupID,
		Enabled:                    gr.Enabled,
		Note:                       gr.Note,
		Version:                    gr.Version,
		WasClaEnforced:             gr.WasCLAEnforced,
		IsRemoteDeleted:            gr.IsRemoteDeleted,
	}
}
