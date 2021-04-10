// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
	v2Models "github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
)

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
	ICLAs                   []*models.IclaSignature
	ECLAs                   []*models.Signature
}

// GerritUserResponse is a data structure to hold the gerrit user query response
type GerritUserResponse struct {
	gerritGroupResponse *v2Models.GerritGroupResponse
	queryType           string
	Error               error
}

// ICLAUserResponse is struct that supports ICLAUsers
type ICLAUserResponse struct {
	ICLASignature *models.IclaSignature
	Error         error
}
