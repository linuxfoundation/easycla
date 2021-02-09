// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package projects_cla_groups

// ProjectClaGroup is database model for projects_cla_group table
type ProjectClaGroup struct {
	ProjectExternalID string `dynamodbav:"project_external_id" json:"project_external_id"`
	ProjectSFID       string `dynamodbav:"project_sfid" json:"project_sfid"`
	ProjectName       string `dynamodbav:"project_name" son:"project_name"`
	ClaGroupID        string `dynamodbav:"cla_group_id" json:"cla_group_id"`
	ClaGroupName      string `dynamodbav:"cla_group_name" json:"cla_group_name"`
	FoundationSFID    string `dynamodbav:"foundation_sfid" json:"foundation_sfid"`
	FoundationName    string `dynamodbav:"foundation_name" json:"foundation_name"`
	RepositoriesCount int64  `dynamodbav:"repositories_count" json:"repositories_count"`
	Note              string `dynamodbav:"version" json:"note"`
	Version           string `dynamodbav:"version" json:"version"`
	DateCreated       string `dynamodbav:"version" json:"date_created"`
	DateModified      string `dynamodbav:"version" json:"date_modified"`
}

// Quick model to grab the bare minimum values
type claGroupIDNameModel struct {
	ProjectID   string `dynamodbav:"project_id"`
	ProjectName string `dynamodbav:"project_name"`
}
