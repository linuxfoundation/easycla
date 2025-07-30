// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_organizations

import "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"

// GithubOrganization is data model for github organizations
type GithubOrganization struct {
	DateCreated                string            `json:"date_created,omitempty"`
	DateModified               string            `json:"date_modified,omitempty"`
	OrganizationInstallationID int64             `json:"organization_installation_id,omitempty"`
	OrganizationName           string            `json:"organization_name,omitempty"`
	OrganizationNameLower      string            `json:"organization_name_lower,omitempty"`
	OrganizationSFID           string            `json:"organization_sfid,omitempty"`
	ProjectSFID                string            `json:"project_sfid"`
	Enabled                    bool              `json:"enabled"`
	AutoEnabled                bool              `json:"auto_enabled"`
	BranchProtectionEnabled    bool              `json:"branch_protection_enabled"`
	AutoEnabledClaGroupID      string            `json:"auto_enabled_cla_group_id,omitempty"`
	Version                    string            `json:"version,omitempty"`
	SkipCLA                    map[string]string `json:"skip_cla,omitempty"`
}

// ToModel converts to models.GithubOrganization
func ToModel(in *GithubOrganization) *models.GithubOrganization {
	return &models.GithubOrganization{
		DateCreated:                in.DateCreated,
		DateModified:               in.DateModified,
		OrganizationInstallationID: in.OrganizationInstallationID,
		OrganizationName:           in.OrganizationName,
		OrganizationSfid:           in.OrganizationSFID,
		Version:                    in.Version,
		Enabled:                    in.Enabled,
		AutoEnabled:                in.AutoEnabled,
		AutoEnabledClaGroupID:      in.AutoEnabledClaGroupID,
		BranchProtectionEnabled:    in.BranchProtectionEnabled,
		ProjectSFID:                in.ProjectSFID,
		SkipCla:                    in.SkipCLA,
	}
}

func toModels(input []*GithubOrganization) []*models.GithubOrganization {
	out := make([]*models.GithubOrganization, 0)
	for _, in := range input {
		out = append(out, ToModel(in))
	}
	return out
}
