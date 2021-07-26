package gitlab_organizations

// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

import (
	models2 "github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
)

// GitlabOrganization is data model for gitlab organizations
type GitlabOrganization struct {
	OrganizationID          string `json:"organization_id"`
	DateCreated             string `json:"date_created,omitempty"`
	DateModified            string `json:"date_modified,omitempty"`
	OrganizationName        string `json:"organization_name,omitempty"`
	OrganizationNameLower   string `json:"organization_name_lower,omitempty"`
	OrganizationSFID        string `json:"organization_sfid,omitempty"`
	ProjectSFID             string `json:"project_sfid"`
	Enabled                 bool   `json:"enabled"`
	AutoEnabled             bool   `json:"auto_enabled"`
	BranchProtectionEnabled bool   `json:"branch_protection_enabled"`
	AutoEnabledClaGroupID   string `json:"auto_enabled_cla_group_id,omitempty"`
	AuthInfo                string `json:"auth_info"`
	AuthState               string `json:"auth_state"`
	Version                 string `json:"version,omitempty"`
}

// ToModel converts to models.GitlabOrganization
func ToModel(in *GitlabOrganization) *models2.GitlabOrganization {
	return &models2.GitlabOrganization{
		OrganizationID:        in.OrganizationID,
		DateCreated:           in.DateCreated,
		DateModified:          in.DateModified,
		OrganizationName:      in.OrganizationName,
		OrganizationSfid:      in.OrganizationSFID,
		Version:               in.Version,
		Enabled:               in.Enabled,
		AutoEnabled:           in.AutoEnabled,
		AutoEnabledClaGroupID: in.AutoEnabledClaGroupID,
		ProjectSFID:           in.ProjectSFID,
	}
}

func toModels(input []*GitlabOrganization) []*models2.GitlabOrganization {
	out := make([]*models2.GitlabOrganization, 0)
	for _, in := range input {
		out = append(out, ToModel(in))
	}
	return out
}
