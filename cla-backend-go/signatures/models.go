// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

// SignatureCompanyID is a simple data model to hold the signature ID and come company details for CCLA's
type SignatureCompanyID struct {
	SignatureID string
	CompanyID   string
	CompanySFID string
	CompanyName string
}

//ApprovalCriteria struct representing approval criteria values
type ApprovalCriteria struct {
	UserEmail      string
	GitHubUsername string
}

//ApprovalList ...
type ApprovalList struct {
	Criteria                string
	ApprovalList            []string
	Action                  string
	ClaGroupID              string
	ClaGroupName            string
	CompanyID               string
	Version                 string
	DomainApprovals         []string
	GHOrgApprovals          []string
	GitHubUsernameApprovals []string
	EmailApprovals          []string
	GHUsernames             []string
	GerritICLAECLAs         []string
}
