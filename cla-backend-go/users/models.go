// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package users

// DBUser data model
type DBUser struct {
	UserID             string   `json:"user_id"`
	UserExternalID     string   `json:"user_external_id"`
	LFEmail            string   `json:"lf_email"`
	Admin              bool     `json:"admin"`
	LFUsername         string   `json:"lf_username"`
	DateCreated        string   `json:"date_created"`
	DateModified       string   `json:"date_modified"`
	UserName           string   `json:"user_name"`
	Version            string   `json:"version"`
	UserEmails         []string `json:"user_emails"`
	UserGithubID       string   `json:"user_github_id"`
	UserGithubUsername string   `json:"user_github_username"`
	UserGitlabID       string   `json:"user_gitlab_id"`
	UserGitlabUsername string   `json:"user_gitlab_username"`
	UserCompanyID      string   `json:"user_company_id"`
	Note               string   `json:"note"`
	LFSub              string   `json:"lf_sub"`
}

type UserEmails struct {
	SS []string `json:"SS"`
}

// CompanySanctioned data model (extracts OFAC sanctioned flag from the company)
type CompanySanctioned struct {
	CompanyID    string `dynamodbav:"company_id" json:"company_id"`
	IsSanctioned bool   `dynamodbav:"is_sanctioned" json:"is_sanctioned"`
}
