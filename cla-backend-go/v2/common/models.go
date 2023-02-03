// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package common

import (
	models2 "github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
)

// GitLabOrganization is data model for gitlab organizations
type GitLabOrganization struct {
	OrganizationID          string `json:"organization_id"`
	ExternalGroupID         int    `json:"external_gitlab_group_id"`
	DateCreated             string `json:"date_created,omitempty"`
	DateModified            string `json:"date_modified,omitempty"`
	OrganizationName        string `json:"organization_name,omitempty"`
	OrganizationNameLower   string `json:"organization_name_lower,omitempty"`
	OrganizationFullPath    string `json:"organization_full_path,omitempty"`
	OrganizationURL         string `json:"organization_url,omitempty"`
	OrganizationSFID        string `json:"organization_sfid,omitempty"`
	ProjectSFID             string `json:"project_sfid"`
	Enabled                 bool   `json:"enabled"`
	AutoEnabled             bool   `json:"auto_enabled"`
	BranchProtectionEnabled bool   `json:"branch_protection_enabled"`
	AutoEnabledClaGroupID   string `json:"auto_enabled_cla_group_id,omitempty"`
	AuthInfo                string `json:"auth_info"`
	AuthState               string `json:"auth_state"`
	Note                    string `json:"note,omitempty"`
	Version                 string `json:"version,omitempty"`
}

// ToModel converts to models.GitlabOrganization
func ToModel(in *GitLabOrganization) *models2.GitlabOrganization {
	return &models2.GitlabOrganization{
		AuthInfo:                in.AuthInfo,
		OrganizationID:          in.OrganizationID,
		DateCreated:             in.DateCreated,
		DateModified:            in.DateModified,
		OrganizationName:        in.OrganizationName,
		OrganizationFullPath:    in.OrganizationFullPath,
		OrganizationURL:         in.OrganizationURL,
		OrganizationSfid:        in.OrganizationSFID,
		Version:                 in.Version,
		Enabled:                 in.Enabled,
		AutoEnabled:             in.AutoEnabled,
		AutoEnabledClaGroupID:   in.AutoEnabledClaGroupID,
		BranchProtectionEnabled: in.BranchProtectionEnabled,
		ProjectSfid:             in.ProjectSFID,
		OrganizationExternalID:  int64(in.ExternalGroupID),
		AuthState:               in.AuthState,
	}
}

// ToModels converts a list of GitLab organizations to a list of external GitLab organization response models
func ToModels(input []*GitLabOrganization) []*models2.GitlabOrganization {
	out := make([]*models2.GitlabOrganization, 0)
	for _, in := range input {
		out = append(out, ToModel(in))
	}
	return out
}

// GitLabAddOrganization is data model for GitLab add organization requests
type GitLabAddOrganization struct {
	OrganizationID          string `json:"organization_id"`
	ExternalGroupID         int64  `json:"external_gitlab_group_id"`
	DateCreated             string `json:"date_created,omitempty"`
	DateModified            string `json:"date_modified,omitempty"`
	OrganizationName        string `json:"organization_name,omitempty"`
	OrganizationNameLower   string `json:"organization_name_lower,omitempty"`
	OrganizationFullPath    string `json:"organization_full_path,omitempty"`
	OrganizationURL         string `json:"organization_url,omitempty"`
	OrganizationSFID        string `json:"organization_sfid,omitempty"`
	ProjectSFID             string `json:"project_sfid"`
	ParentProjectSFID       string `json:"parent_project_sfid"`
	Enabled                 bool   `json:"enabled"`
	AutoEnabled             bool   `json:"auto_enabled"`
	BranchProtectionEnabled bool   `json:"branch_protection_enabled"`
	AutoEnabledClaGroupID   string `json:"auto_enabled_cla_group_id,omitempty"`
	AuthInfo                string `json:"auth_info"`
	AuthState               string `json:"auth_state"`
	Version                 string `json:"version,omitempty"`
}

// ExternalGroupIDAsInt returns the external group ID as an integer value
func (m *GitLabAddOrganization) ExternalGroupIDAsInt() int {
	return int(m.ExternalGroupID)
}
