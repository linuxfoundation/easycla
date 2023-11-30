// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

// ItemSignature database model
type ItemSignature struct {
	SignatureID                   string   `json:"signature_id"` // No omitempty, always included
	DateCreated                   string   `json:"date_created,omitempty"`
	DateModified                  string   `json:"date_modified,omitempty"`
	SignatureApproved             bool     `json:"signature_approved,omitempty"`
	SignatureSigned               bool     `json:"signature_signed"`
	SignatureDocumentMajorVersion int      `json:"signature_document_major_version,omitempty"`
	SignatureDocumentMinorVersion int      `json:"signature_document_minor_version,omitempty"`
	SignatureSignURL              string   `json:"signature_sign_url,omitempty"`
	SignatureReturnURL            string   `json:"signature_return_url,omitempty"`
	SignatureReturnURLType        string   `json:"signature_return_url_type,omitempty"`
	SignatureCallbackURL          string   `json:"signature_callback_url,omitempty"`
	SignatureReferenceID          string   `json:"signature_reference_id,omitempty"`
	SignatureReferenceName        string   `json:"signature_reference_name,omitempty"`
	SignatureReferenceNameLower   string   `json:"signature_reference_name_lower,omitempty"`
	SignatureProjectID            string   `json:"signature_project_id,omitempty"`
	SignatureReferenceType        string   `json:"signature_reference_type,omitempty"`
	SignatureType                 string   `json:"signature_type,omitempty"`
	SignatureEnvelopeID           string   `json:"signature_envelope_id,omitempty"`
	SignatureUserCompanyID        string   `json:"signature_user_ccla_company_id,omitempty"`
	EmailApprovalList             []string `json:"email_whitelist,omitempty"`
	EmailDomainApprovalList       []string `json:"domain_whitelist,omitempty"`
	GitHubUsernameApprovalList    []string `json:"github_whitelist,omitempty"`
	GitHubOrgApprovalList         []string `json:"github_org_whitelist,omitempty"`
	GitlabUsernameApprovalList    []string `json:"gitlab_username_approval_list,omitempty"`
	GitlabOrgApprovalList         []string `json:"gitlab_org_approval_list,omitempty"`
	SignatureACL                  []string `json:"signature_acl,omitempty"`
	UserGithubID                  string   `json:"user_github_id,omitempty"`
	UserGithubUsername            string   `json:"user_github_username,omitempty"`
	UserGitlabID                  string   `json:"user_gitlab_id,omitempty"`
	UserGitlabUsername            string   `json:"user_gitlab_username,omitempty"`
	UserLFUsername                string   `json:"user_lf_username,omitempty"`
	UserName                      string   `json:"user_name,omitempty"`
	UserEmail                     string   `json:"user_email,omitempty"`
	SigtypeSignedApprovedID       string   `json:"sigtype_signed_approved_id,omitempty"`
	SignedOn                      string   `json:"signed_on,omitempty"`
	SignatoryName                 string   `json:"signatory_name,omitempty"`
	UserDocusignName              string   `json:"user_docusign_name,omitempty"`
	UserDocusignDateSigned        string   `json:"user_docusign_date_signed,omitempty"`
	AutoCreateECLA                bool     `json:"auto_create_ecla,omitempty"`
	UserDocusignRawXML            string   `json:"user_docusign_raw_xml,omitempty"`
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
