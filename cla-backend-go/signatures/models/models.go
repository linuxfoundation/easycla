// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package models

import (
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	v2Models "github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
)

// ClaManagerInfoParams represents the CLAManagerInfo used inside of the Email Templates
type ClaManagerInfoParams struct {
	Username string
	Email    string
}

// SimpleUserInfoModel is a  simple/temp user model to consolidate the email list, GitHub username list, and GitLab username list
type SimpleUserInfoModel struct {
	Email          string
	GitHubUserName string
	GitHubUserID   string
	GitLabUserName string
	GitLabUserID   string
}

// SignatureCompanyID is a simple data model to hold the signature ID and come company details for CCLA's
type SignatureCompanyID struct {
	SignatureID string
	CompanyID   string
	CompanySFID string
	CompanyName string
}

// ApprovalCriteria struct representing approval criteria values
type ApprovalCriteria struct {
	UserEmail      string
	GitHubUsername string
	GitlabUsername string
}

type EmployeeModel struct {
	Signature *models.Signature
	User      *models.User
}

// ApprovalList data model
type ApprovalList struct {
	Criteria                string
	ApprovalList            []string
	Action                  string
	ClaGroupID              string
	ClaGroupName            string
	CompanyID               string
	Version                 string
	EmailApprovals          []string
	DomainApprovals         []string
	GitHubUsernameApprovals []string
	GitHubUsernames         []string
	GitHubOrgApprovals      []string
	GitlabUsernameApprovals []string
	GitlabOrgApprovals      []string
	GitlabUsernames         []string
	GerritICLAECLAs         []string
	ICLAs                   []*models.IclaSignature
	ECLAs                   []*models.Signature
	CLAManager              *models.User
	ManagersInfo            []ClaManagerInfoParams
	CCLASignature           *models.Signature
}

// GerritUserResponse is a data structure to hold the gerrit user query response
type GerritUserResponse struct {
	GerritGroupResponse *v2Models.GerritGroupResponse
	QueryType           string
	Error               error
}

// ICLAUserResponse is struct that supports ICLAUsers
type ICLAUserResponse struct {
	ICLASignature *models.IclaSignature
	Error         error
}

const (
	//CCLAICLA representing user removal under CCLA + ICLA
	CCLAICLA = "CCLAICLA"
	//CCLAICLAECLA representing user removal under CCLA + ICLA +ECLA
	CCLAICLAECLA = "CCLAICLAECLA"
	//CCLA representing normal use case of user under CCLA
	CCLA = "ICLA"
	//ICLA representing individual use case
	ICLA = "ICLA"
)

// SignatureDynamoDB is a data model for the signature table. Most of the record create/update happens in the old
// Python code, however, we needed to add this data model after we added the auto-enable feature for employee acknowledgements.
//
// | Type of Signature      | `project_id`       |`signature_reference_type`|`signature_type`|`signature_reference_id`|`signature_user_ccla_company_id`| PDF? | Auto Create ECLA Flag |
// |:-----------------------|:-------------------|:-------------------------|:---------------|:-----------------------|:-------------------------------|------|-----------------------|
// | ICLA (individual)      | <valid_project_id> | user                     | cla            | <user_id_value>        | null/empty                     | Yes  | No                    |
// | CCLA/ECLA (employee)   | <valid_project_id> | user                     | cla            | <user_id_value>        | <company_id_value>             | No   | Yes                   |
// | CCLA (CLA Manager)     | <valid_project_id> | company                  | ccla           | <company_id_value>     | null/empty                     | Yes  | No                    |
type SignatureDynamoDB struct {
	SignatureID                   string   `json:"signature_id"`                     // PK
	SignatureProjectID            string   `json:"signature_project_id"`             // the signature CLA group ID
	SignatureReferenceID          string   `json:"signature_reference_id"`           // value is user_id for icla/ecla, value is company_id for ccla
	SignatureType                 string   `json:"signature_type"`                   // one of: cla, ccla
	SignatureACL                  []string `json:"signature_acl"`                    // [github:1234567]
	SignatureApproved             bool     `json:"signature_approved"`               // true if the signature is approved, false if revoked/invalidated
	SignatureSigned               bool     `json:"signature_signed"`                 // true if the signature has been signed
	SignatureReferenceType        string   `json:"signature_reference_type"`         // one of: user, company
	SignatureReferenceName        string   `json:"signature_reference_name"`         // John Doe
	SignatureReferenceNameLower   string   `json:"signature_reference_name_lower"`   // john doe
	SignatureUserCCLACompanyID    string   `json:"signature_user_ccla_company_id"`   // set for ECLA record types, null/missing otherwise
	SignatureReturnURL            string   `json:"signature_return_url"`             // e.g https://github.com/open-telemetry/opentelemetry-go/pull/1751
	SignatureDocumentMajorVersion int      `json:"signature_document_major_version"` // 2
	SignatureDocumentMinorVersion int      `json:"signature_document_minor_version"` // 0
	SigTypeSignedApprovedID       string   `json:"sig_type_signed_approved_id"`      // e.g. ecla#true#true#e908aefe-27ff-44ea-9f06-ab513f34cb1d
	SignedOn                      string   `json:"signed_on"`                        // 2021-03-29T22:48:10.246463+0000
	AutoCreateECLA                bool     `json:"auto_create_ecla"`                 // flag to indicate if auto-create ECLA feature is enabled (only applies to CCLA signature record types)
	ProjectID                     string   `json:"project_id"`
	ProjectName                   string   `json:"project_name"`
	ProjectSFID                   string   `json:"project_sfid"`
	CompanyID                     string   `json:"company_id"`
	CompanyName                   string   `json:"company_name"`
	CompanySFID                   string   `json:"company_sfid"`
	UserName                      string   `json:"user_name"`
	UserEmail                     string   `json:"user_email"`
	UserLFUsername                string   `json:"user_lf_username"`
	UserGitHubUsername            string   `json:"user_github_username"`
	UserGitLabUsername            string   `json:"user_gitlab_username"`
	DateCreated                   string   `json:"date_created"`  // 2021-03-29T22:48:10.246463+0000
	DateModified                  string   `json:"date_modified"` // 2021-08-23T22:33:03.798606+0000
	Note                          string   `json:"note"`
	Version                       string   `json:"version"` // v1
}

// ActivePullRequest data model
type ActivePullRequest struct {
	GitHubAuthorUsername string `json:"github_author_username"`
	GitHubAuthorEmail    string `json:"github_author_email"`
	CLAGroupID           string `json:"cla_group_id"`
	RepositoryID         string `json:"repository_id"`
	PullRequestID        string `json:"pull_request_id"`
}

// ActiveGitLabPullRequest data model
type ActiveGitLabPullRequest struct {
	GitLabAuthorUserName string `json:"gitlab_author_username"`
	GitLabAuthorEmail    string `json:"gitlab_author_email"`
	ClaGroupID           string `json:"cla_group_id"`
	RepositoryID         string `json:"repository_id"`
	MergeRequestID       string `json:"merge_request_id"`
	ProjectID            string `json:"project_id"`
}
