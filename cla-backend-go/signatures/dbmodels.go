// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

// ItemSignature database model
type ItemSignature struct {
	SignatureID                   string   `json:"signature_id"`
	DateCreated                   string   `json:"date_created"`
	DateModified                  string   `json:"date_modified"`
	SignatureApproved             bool     `json:"signature_approved"`
	SignatureSigned               bool     `json:"signature_signed"`
	SignatureDocumentMajorVersion string   `json:"signature_document_major_version"`
	SignatureDocumentMinorVersion string   `json:"signature_document_minor_version"`
	SignatureSignURL              string   `json:"signature_sign_url"`
	SignatureReturnURL            string   `json:"signature_return_url"`
	SignatureReturnURLType        string   `json:"signature_return_url_type"`
	SignatureCallbackURL          string   `json:"signature_callback_url"`
	SignatureReferenceID          string   `json:"signature_reference_id"`
	SignatureReferenceName        string   `json:"signature_reference_name"`
	SignatureReferenceNameLower   string   `json:"signature_reference_name_lower"`
	SignatureProjectID            string   `json:"signature_project_id"`
	SignatureReferenceType        string   `json:"signature_reference_type"`
	SignatureType                 string   `json:"signature_type"`
	SignatureUserCompanyID        string   `json:"signature_user_ccla_company_id"`
	EmailApprovalList             []string `json:"email_whitelist"`
	EmailDomainApprovalList       []string `json:"domain_whitelist"`
	GitHubUsernameApprovalList    []string `json:"github_whitelist"`
	GitHubOrgApprovalList         []string `json:"github_org_whitelist"`
	GitlabUsernameApprovalList    []string `json:"gitlab_username_approval_list"`
	GitlabOrgApprovalList         []string `json:"gitlab_org_approval_list"`
	SignatureACL                  []string `json:"signature_acl"`
	UserGithubID                  string   `json:"user_github_id"`
	UserGithubUsername            string   `json:"user_github_username"`
	UserGitlabID                  string   `json:"user_gitlab_id"`
	UserGitlabUsername            string   `json:"user_gitlab_username"`
	UserLFUsername                string   `json:"user_lf_username"`
	UserName                      string   `json:"user_name"`
	UserEmail                     string   `json:"user_email"`
	SigtypeSignedApprovedID       string   `json:"sigtype_signed_approved_id"`
	SignedOn                      string   `json:"signed_on"`
	SignatoryName                 string   `json:"signatory_name"`
	UserDocusignName              string   `json:"user_docusign_name"`
	UserDocusignDateSigned        string   `json:"user_docusign_date_signed"`
	AutoCreateECLA                bool     `json:"auto_create_ecla"`
}

// DBManagersModel is a database model for only the ACL/Manager column
type DBManagersModel struct {
	SignatureID  string   `json:"signature_id"`
	SignatureACL []string `json:"signature_acl"`
}

// DBSignatureUsersModel is a database model for only the signature ID and signature_reference_id fields
type DBSignatureUsersModel struct {
	SignatureID string `json:"signature_id"`
	UserID      string `json:"signature_reference_id"`
}

// DBSignatureMetadata is a database model for the key and value fields for a given signature
type DBSignatureMetadata struct {
	Key    string `json:"key"`
	Expire int64  `json:"expire"`
	Value  string `json:"value"`
}
