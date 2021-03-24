// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
//nolint
package events

import (
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
)

// EventData returns event data string which is used for event logging and containsPII field
type EventData interface {
	GetEventDetailsString(args *LogEventArgs) (eventData string, containsPII bool)
	GetEventSummaryString(args *LogEventArgs) (eventData string, containsPII bool)
}

type CLAGroupEnrolledProjectData struct {
}

type CLAGroupUnenrolledProjectData struct {
}

type ProjectServiceCLAEnabledData struct {
}

type ProjectServiceCLADisabledData struct {
}

// RepositoryAddedEventData . . .
type RepositoryAddedEventData struct {
	RepositoryName string
}

// RepositoryDisabledEventData . . .
type RepositoryDisabledEventData struct {
	RepositoryName string
}

// RepositoryUpdatedEventData . . .
type RepositoryUpdatedEventData struct {
	RepositoryName string
}

// RepositoryBranchProtectionAddedEventData . . .
type RepositoryBranchProtectionAddedEventData struct {
	RepositoryName string
}

// RepositoryBranchProtectionDisabledEventData . . .
type RepositoryBranchProtectionDisabledEventData struct {
	RepositoryName string
}

// RepositoryBranchProtectionUpdatedEventData . . .
type RepositoryBranchProtectionUpdatedEventData struct {
	RepositoryName string
}

// GerritProjectDeletedEventData . . .
type GerritProjectDeletedEventData struct {
	DeletedCount int
}

// GerritAddedEventData . . .
type GerritAddedEventData struct {
	GerritRepositoryName string
}

// GerritDeletedEventData . . .
type GerritDeletedEventData struct {
	GerritRepositoryName string
}

// GerritUserAddedEventData . . .
type GerritUserAddedEventData struct {
	Username  string
	GroupName string
}

// GerritUserRemovedEventData . . .
type GerritUserRemovedEventData struct {
	Username  string
	GroupName string
}

// GitHubProjectDeletedEventData . . .
type GitHubProjectDeletedEventData struct {
	DeletedCount int
}

// SignatureProjectInvalidatedEventData . . .
type SignatureProjectInvalidatedEventData struct {
	InvalidatedCount int
}

//SignatureInvalidatedApprovalRejectionEventData . . .
type SignatureInvalidatedApprovalRejectionEventData struct {
	GHUsername  string
	Email       string
	SignatureID string
	CLAManager  *models.User
	CLAGroupID  string
}

// UserCreatedEventData . . .
type UserCreatedEventData struct{}

// UserDeletedEventData . . .
type UserDeletedEventData struct {
	DeletedUserID string
}

// UserUpdatedEventData . . .
type UserUpdatedEventData struct{}

// CompanyACLRequestAddedEventData . . .
type CompanyACLRequestAddedEventData struct {
	UserName  string
	UserID    string
	UserEmail string
}

// CompanyACLRequestApprovedEventData . . .
type CompanyACLRequestApprovedEventData struct {
	UserName  string
	UserID    string
	UserEmail string
}

// CompanyACLRequestDeniedEventData . . .
type CompanyACLRequestDeniedEventData struct {
	UserName  string
	UserID    string
	UserEmail string
}

// CompanyACLUserAddedEventData . . .
type CompanyACLUserAddedEventData struct {
	UserLFID string
}

// CLATemplateCreatedEventData . . .
type CLATemplateCreatedEventData struct{}

// GitHubOrganizationAddedEventData . . .
type GitHubOrganizationAddedEventData struct {
	GitHubOrganizationName  string
	AutoEnabled             bool
	AutoEnabledClaGroupID   string
	BranchProtectionEnabled bool
}

// GitHubOrganizationDeletedEventData . . .
type GitHubOrganizationDeletedEventData struct {
	GitHubOrganizationName string
}

// GitHubOrganizationUpdatedEventData . . .
type GitHubOrganizationUpdatedEventData struct {
	GitHubOrganizationName string
	AutoEnabled            bool
	AutoEnabledClaGroupID  string
}

// CCLAApprovalListRequestCreatedEventData . . .
type CCLAApprovalListRequestCreatedEventData struct {
	RequestID string
}

// CCLAApprovalListRequestApprovedEventData . . .
type CCLAApprovalListRequestApprovedEventData struct {
	RequestID string
}

// CCLAApprovalListRequestRejectedEventData . . .
type CCLAApprovalListRequestRejectedEventData struct {
	RequestID string
}

// CLAManagerCreatedEventData . . .
type CLAManagerCreatedEventData struct {
	CompanyName string
	ProjectName string
	UserName    string
	UserEmail   string
	UserLFID    string
}

// CLAManagerDeletedEventData . . .
type CLAManagerDeletedEventData struct {
	CompanyName string
	ProjectName string
	UserName    string
	UserEmail   string
	UserLFID    string
}

// CLAManagerRequestCreatedEventData . . .
type CLAManagerRequestCreatedEventData struct {
	RequestID   string
	CompanyName string
	ProjectName string
	UserName    string
	UserEmail   string
	UserLFID    string
}

// CLAManagerRequestApprovedEventData . . .
type CLAManagerRequestApprovedEventData struct {
	RequestID    string
	CompanyName  string
	ProjectName  string
	UserName     string
	UserEmail    string
	ManagerName  string
	ManagerEmail string
}

// CLAManagerRequestDeniedEventData . . .
type CLAManagerRequestDeniedEventData struct {
	RequestID    string
	CompanyName  string
	ProjectName  string
	UserName     string
	UserEmail    string
	ManagerName  string
	ManagerEmail string
}

// CLAManagerRequestDeletedEventData . . .
type CLAManagerRequestDeletedEventData struct {
	RequestID    string
	CompanyName  string
	ProjectName  string
	UserName     string
	UserEmail    string
	ManagerName  string
	ManagerEmail string
}

// CLAApprovalListAddEmailData . . .
type CLAApprovalListAddEmailData struct {
	UserName          string
	UserEmail         string
	UserLFID          string
	ApprovalListEmail string
}

// CLAApprovalListRemoveEmailData . . .
type CLAApprovalListRemoveEmailData struct {
	UserName          string
	UserEmail         string
	UserLFID          string
	ApprovalListEmail string
}

// CLAApprovalListAddDomainData . . .
type CLAApprovalListAddDomainData struct {
	UserName           string
	UserEmail          string
	UserLFID           string
	ApprovalListDomain string
}

// CLAApprovalListRemoveDomainData . . .
type CLAApprovalListRemoveDomainData struct {
	UserName           string
	UserEmail          string
	UserLFID           string
	ApprovalListDomain string
}

// CLAApprovalListAddGitHubUsernameData . . .
type CLAApprovalListAddGitHubUsernameData struct {
	UserName                   string
	UserEmail                  string
	UserLFID                   string
	ApprovalListGitHubUsername string
}

// CLAApprovalListRemoveGitHubUsernameData . . .
type CLAApprovalListRemoveGitHubUsernameData struct {
	UserName                   string
	UserEmail                  string
	UserLFID                   string
	ApprovalListGitHubUsername string
}

// CLAApprovalListAddGitHubOrgData . . .
type CLAApprovalListAddGitHubOrgData struct {
	UserName              string
	UserEmail             string
	UserLFID              string
	ApprovalListGitHubOrg string
}

// CLAApprovalListRemoveGitHubOrgData . . .
type CLAApprovalListRemoveGitHubOrgData struct {
	UserName              string
	UserEmail             string
	UserLFID              string
	ApprovalListGitHubOrg string
}

// ApprovalListGitHubOrganizationAddedEventData . . .
type ApprovalListGitHubOrganizationAddedEventData struct {
	GitHubOrganizationName string
}

// ApprovalListGitHubOrganizationDeletedEventData . . .
type ApprovalListGitHubOrganizationDeletedEventData struct {
	GitHubOrganizationName string
}

// ClaManagerAccessRequestAddedEventData . . .
type ClaManagerAccessRequestAddedEventData struct {
	ProjectName string
	CompanyName string
}

// ClaManagerAccessRequestDeletedEventData . . .
type ClaManagerAccessRequestDeletedEventData struct {
	RequestID string
}

// CLAGroupCreatedEventData . . .
type CLAGroupCreatedEventData struct{}

// CLAGroupUpdatedEventData . . .
type CLAGroupUpdatedEventData struct {
	ClaGroupName        string
	ClaGroupDescription string
}

// CLAGroupDeletedEventData . . .
type CLAGroupDeletedEventData struct{}

// ContributorNotifyCompanyAdminData . . .
type ContributorNotifyCompanyAdminData struct {
	AdminName  string
	AdminEmail string
}

// ContributorNotifyCLADesignee . . .
type ContributorNotifyCLADesignee struct {
	DesigneeName  string
	DesigneeEmail string
}

// ContributorAssignCLADesignee . . .
type ContributorAssignCLADesignee struct {
	DesigneeName  string
	DesigneeEmail string
}

// UserConvertToContactData . . .
type UserConvertToContactData struct{}

// AssignRoleScopeData . . .
type AssignRoleScopeData struct {
	Role  string
	Scope string
}

// ClaManagerRoleCreatedData . . .
type ClaManagerRoleCreatedData struct {
	Role      string
	Scope     string
	UserName  string
	UserEmail string
}

// ClaManagerRoleDeletedData . . .
type ClaManagerRoleDeletedData struct {
	Role      string
	Scope     string
	UserName  string
	UserEmail string
}

// GetEventDetailsString . . .
func (ed *CLAGroupEnrolledProjectData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	return fmt.Sprintf("%s (%s/%s) enabled the the project %s (%s) from the CLA Group %s (%s).",
		args.UserName, args.LFUser.Name, args.UserModel.LfEmail, args.ProjectName, args.ProjectID, args.CLAGroupName, args.CLAGroupID), false
}

// GetEventDetailsString . . .
func (ed *CLAGroupUnenrolledProjectData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	return fmt.Sprintf("%s (%s/%s) unenrolled the the project %s (%s) from the CLA Group %s (%s).",
		args.UserName, args.LFUser.Name, args.UserModel.LfEmail, args.ProjectName, args.ProjectID, args.CLAGroupName, args.CLAGroupID), false
}

// GetEventDetailsString . . .
func (ed *ProjectServiceCLAEnabledData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	return fmt.Sprintf("%s (%s/%s) enabled the CLA Service for the project %s (%s)",
		args.UserName, args.LFUser.Name, args.UserModel.LfEmail, args.ProjectName, args.ProjectID), false
}

// GetEventDetailsString . . .
func (ed *ProjectServiceCLADisabledData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	return fmt.Sprintf("%s (%s/%s) disabled the CLA Service for the project %s (%s)",
		args.UserName, args.LFUser.Name, args.UserModel.LfEmail, args.ProjectName, args.ProjectID), false
}

// GetEventDetailsString . . .
func (ed *RepositoryAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository: %s was added for the project %s by the user %s.", ed.RepositoryName, args.ProjectName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *RepositoryDisabledEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository %s was deleted for the project %s by the user %s.", ed.RepositoryName, args.ProjectName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *RepositoryUpdatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository %s was updated for the project %s by the user %s.", ed.RepositoryName, args.ProjectName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *RepositoryBranchProtectionAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository branch protection %s was added for the project %s by the user %s.", ed.RepositoryName, args.ProjectName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *RepositoryBranchProtectionDisabledEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository branch protection %s was disabled for the project %s by the user %s.", ed.RepositoryName, args.ProjectName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *RepositoryBranchProtectionUpdatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository branch protection %s was updated for the project %s by the user %s.", ed.RepositoryName, args.ProjectName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *UserCreatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s added. User Details: %+v.", args.UserName, args.UserModel)
	return data, true
}

// GetEventDetailsString . . .
func (ed *UserUpdatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	return fmt.Sprintf("User: %s updated. User Details:  %+v.", args.UserName, *args.UserModel), true
}

// GetEventDetailsString . . .
func (ed *UserDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s deleted. User ID: %s.", args.UserName, ed.DeletedUserID)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CompanyACLRequestAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s added pending invite with ID: %s and Email: %s for Company: %s.",
		ed.UserName, ed.UserID, ed.UserEmail, args.CompanyName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CompanyACLRequestApprovedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("Access Aproved for User: %s, ID: %s, Email: %s Company Group: %s.",
		ed.UserName, args.CompanyName, ed.UserID, ed.UserEmail)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CompanyACLRequestDeniedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("Access Denied for User: %s, ID: %s, Email: %s Company Group: %s.",
		ed.UserName, args.CompanyName, ed.UserID, ed.UserEmail)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CompanyACLUserAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User with LF Username: %s added to the ACL for Company: %s by: %s.",
		args.LFUser.Name, args.CompanyName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLATemplateCreatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("PDF Templates created for Project: %s by: %s.", args.UserName, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *GitHubOrganizationAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("GitHub Organization: %s was added with auto-enabled: %t, with branch protection enabled: %t",
		ed.GitHubOrganizationName, ed.AutoEnabled, ed.BranchProtectionEnabled)
	if ed.AutoEnabledClaGroupID != "" {
		data = data + fmt.Sprintf(" with auto-enabled-cla-group: %s", ed.AutoEnabledClaGroupID)
	}
	data = data + fmt.Sprintf(" by: %s.", args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *GitHubOrganizationDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("GitHub Organization: %s was deleted by: %s.",
		ed.GitHubOrganizationName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *GitHubOrganizationUpdatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("GitHub Organization:%s was updated with auto-enabled: %t",
		ed.GitHubOrganizationName, ed.AutoEnabled)
	if ed.AutoEnabledClaGroupID != "" {
		data = data + fmt.Sprintf(" with auto-enabled-cla-group: %s", ed.AutoEnabledClaGroupID)
	}
	data = data + fmt.Sprintf("by: %s.", args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CCLAApprovalListRequestApprovedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s approved a CCLA Approval Request for Project: %s and Company: %s with Request ID: %s.",
		args.UserName, args.ProjectName, args.CompanyName, ed.RequestID)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CCLAApprovalListRequestRejectedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s rejected a CCLA Approval Request for Project: %s, Company: %s with Request ID: %s.",
		args.UserName, args.ProjectName, args.CompanyName, ed.RequestID)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAManagerRequestCreatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s, LFID: %s, Email: %s added CLA Manager Request: %s for Company: %s, Project: %s.",
		ed.UserName, ed.UserLFID, ed.UserEmail, ed.RequestID, ed.CompanyName, ed.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAManagerCreatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s, LFID: %s, Email: %s was added as CLA Manager for Company: %s, Project: %s.",
		ed.UserName, ed.UserLFID, ed.UserEmail, ed.CompanyName, ed.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAManagerDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s LFID: %s, Email: %s was removed as CLA Manager for Company: %s, Project: %s.",
		ed.UserName, ed.UserLFID, ed.UserEmail, ed.CompanyName, ed.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAManagerRequestApprovedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager Request: %s was approved for User %s, Email: %s by Manager: %s, Email: %s for Company: %s, Project: %s.",
		ed.RequestID, ed.UserName, ed.UserEmail, ed.ManagerName, ed.ManagerEmail, ed.CompanyName, ed.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAManagerRequestDeniedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager Request: %s was denied for User %s, Email: %s by Manager: %s, Email: %s for Company: %s, Project: %s.",
		ed.RequestID, ed.UserName, ed.UserEmail, ed.ManagerName, ed.ManagerEmail, ed.CompanyName, ed.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAManagerRequestDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager Request: %s was deleted for User %s, Email: %s by Manager: %s, Email: %s for Company: %s, Project: %s.",
		ed.RequestID, ed.UserName, ed.UserEmail, ed.ManagerName, ed.ManagerEmail, ed.CompanyName, ed.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAApprovalListAddEmailData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager: %s, Email: %s, LFID: %s added Email: %s to the approval list for Company: %s, Project: %s.",
		ed.UserName, ed.UserEmail, ed.UserLFID, ed.ApprovalListEmail, args.CompanyName, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAApprovalListRemoveEmailData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager: %s, Email: %s, LFID: %s removed Email: %s from the approval list for Company: %s, Project: %s.",
		ed.UserName, ed.UserEmail, ed.UserLFID, ed.ApprovalListEmail, args.CompanyName, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAApprovalListAddDomainData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager: %s, Email: %s, LFID: %s added Domain: %s to the approval list for Company: %s, Project: %s.",
		ed.UserName, ed.UserEmail, ed.UserLFID, ed.ApprovalListDomain, args.CompanyName, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAApprovalListRemoveDomainData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager: %s, Email: %s, LFID: %s removed Domain %s from the approval list for Company: %s, Project: %s.",
		ed.UserName, ed.UserEmail, ed.UserLFID, ed.ApprovalListDomain, args.CompanyName, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAApprovalListAddGitHubUsernameData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager: %s, Email: %s, LFID: %s added GitHub Username: %s to the approval list for Company: %s, Project: %s.",
		ed.UserName, ed.UserEmail, ed.UserLFID, ed.ApprovalListGitHubUsername, args.CompanyName, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAApprovalListRemoveGitHubUsernameData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager: %s, Email: %s, LFID: %s removed GitHub Username: %s from the approval list for Company: %s, Project: %s.",
		ed.UserName, ed.UserEmail, ed.UserLFID, ed.ApprovalListGitHubUsername, args.CompanyName, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAApprovalListAddGitHubOrgData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager: %s, Email: %s, LFID: %s added GitHub Organization: %s to the approval list for Company: %s, Project: %s.",
		ed.UserName, ed.UserEmail, ed.UserLFID, ed.ApprovalListGitHubOrg, args.CompanyName, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAApprovalListRemoveGitHubOrgData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager: %s, Email: %s, LFID: %s removed GitHub Organization: %s from the approval list for Company: %s, Project: %s.",
		ed.UserName, ed.UserEmail, ed.UserLFID, ed.ApprovalListGitHubOrg, args.CompanyName, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CCLAApprovalListRequestCreatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s created a CCLA Approval Request for Project: %s, Company: %s with Request ID: %s.",
		args.UserName, args.ProjectName, args.CompanyName, ed.RequestID)
	return data, true
}

// GetEventDetailsString . . .
func (ed *ApprovalListGitHubOrganizationAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager: %s added GitHub Organization: %s to the whitelist for Company %s, Project: %s.",
		args.UserName, ed.GitHubOrganizationName, args.CompanyName, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *ApprovalListGitHubOrganizationDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager: %s removed GitHub Organization: %s from the whitelist for Company: %s, Project: %s.",
		args.UserName, ed.GitHubOrganizationName, args.CompanyName, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *ClaManagerAccessRequestAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s has requested to be CLA Manager for Company %s, Project: %s.",
		args.UserName, ed.CompanyName, ed.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *ClaManagerAccessRequestDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s has deleted CLA Manager Request with ID: %s.",
		args.UserName, ed.RequestID)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAGroupCreatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Group ID: %s, Name: %s was created by: %s.",
		args.ProjectID, args.ProjectName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAGroupUpdatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Group ID: %s was updated by: %s with Name: %s, Description: %s.",
		args.ProjectID, args.UserName, ed.ClaGroupName, ed.ClaGroupDescription)
	return data, true
}

// GetEventDetailsString . . .
func (ed *CLAGroupDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Group ID: %s was deleted by: %s.",
		args.ProjectID, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *GerritProjectDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("%d Gerrit Repositories were deleted due to CLA Group/Project: %s deletion.",
		ed.DeletedCount, args.ProjectName)
	return data, false
}

// GetEventDetailsString . . .
func (ed *GerritAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("Gerrit Repository: %s was added by: %s.", ed.GerritRepositoryName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *GerritDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("Gerrit Repository: %s was deleted by: %s.", ed.GerritRepositoryName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *GerritUserAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The username %s was add to the gerrit group %s by the user %s.", ed.Username, ed.GroupName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *GerritUserRemovedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The username %s was removed from the gerrit group %s by the user %s.", ed.Username, ed.GroupName, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *GitHubProjectDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("%d GitHub Repositories were deleted due to CLA Group/Project: [%s] deletion.",
		ed.DeletedCount, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *SignatureProjectInvalidatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("%d Signatures were invalidated (approved set to false) due to CLA Group/Project: %s deletion.",
		ed.InvalidatedCount, args.ProjectName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *SignatureInvalidatedApprovalRejectionEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	reason := "No reason"
	if ed.Email != "" {
		reason = fmt.Sprintf("GH Username: %s approval removal ", ed.GHUsername)
	} else if ed.GHUsername != "" {
		reason = fmt.Sprintf("GH Username: %s approval removal ", ed.GHUsername)
	}
	data := fmt.Sprintf("Signature ID: %s invalidated by %s (approved set to false) due to %s ", utils.GetBestUsername(ed.CLAManager), ed.SignatureID, reason)
	return data, true
}

// GetEventDetailsString . . .
func (ed *ContributorNotifyCompanyAdminData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s notified Company Admin: %s by Email: %s for Company ID: %s, Name: %s.",
		args.UserName, ed.AdminName, ed.AdminEmail, args.CompanyName, args.CompanyID)
	return data, true
}

// GetEventDetailsString . . .
func (ed *ContributorNotifyCLADesignee) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s notified CLA Designee: %s by Email: %s for Project Name : %s, ID: %s and Company Name: %s, ID: %s.",
		args.UserName, ed.DesigneeName, ed.DesigneeEmail,
		args.ProjectName, args.ProjectSFID,
		args.CompanyName, args.CompanyID)
	return data, true
}

// GetEventDetailsString . . .
func (ed *ContributorAssignCLADesignee) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User Name: %s, Email: %s was assigned as CLA Manager Designee for project Name: %s, ID:  %s and Company Name: %s, ID: %s by: %s.",
		ed.DesigneeName, ed.DesigneeEmail,
		args.ProjectName, args.ProjectSFID,
		args.CompanyName, args.CompanyID, args.UserName)
	return data, true
}

// GetEventDetailsString . . .
func (ed *UserConvertToContactData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s was converted to Contact state for Project: %s with ID: %s.",
		args.LFUser.Name, args.ProjectName, args.ProjectSFID)
	return data, true
}

// GetEventDetailsString . . .
func (ed *AssignRoleScopeData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s was assigned Scope: %s with Role: %s for Project: %s with ID: %s.",
		args.LFUser.Name,
		ed.Scope, ed.Role, args.ProjectName, args.ProjectSFID)
	return data, true
}

// GetEventDetailsString . . .
func (ed *ClaManagerRoleCreatedData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s, Email: %s was added to Role: %s with Scope: %s by: %s.", ed.UserName, ed.UserEmail, ed.Role, ed.Scope, args.UserName)
	return data, false
}

// GetEventDetailsString . . .
func (ed *ClaManagerRoleDeletedData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s, Email: %s was removed from Role: %s with Scope: %s by: %s.", ed.UserName, ed.UserEmail, ed.Role, ed.Scope, args.UserName)
	return data, false
}

// Event Summary started

// GetEventDetailsString . . .
func (ed *CLAGroupEnrolledProjectData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	return fmt.Sprintf("The user %s enabled the the project %s from the CLA Group %s.",
		args.UserName, args.ProjectName, args.CLAGroupName), false
}

// GetEventDetailsString . . .
func (ed *CLAGroupUnenrolledProjectData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	return fmt.Sprintf("The user %s unenrolled the the project %s from the CLA Group %s.",
		args.UserName, args.ProjectName, args.CLAGroupName), false
}

// GetEventDetailsString . . .
func (ed *ProjectServiceCLAEnabledData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s enabled the CLA Service", args.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, false
}

// GetEventDetailsString . . .
func (ed *ProjectServiceCLADisabledData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s disabled the CLA Service", args.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, false
}

// GetEventSummaryString . . .
func (ed *RepositoryAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository %s was added", ed.RepositoryName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *RepositoryDisabledEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository %s was deleted", ed.RepositoryName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *RepositoryUpdatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository %s was updated", ed.RepositoryName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString . . .
func (ed *RepositoryBranchProtectionAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository branch protection %s was added", ed.RepositoryName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString . . .
func (ed *RepositoryBranchProtectionDisabledEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository branch protection %s was disabled", ed.RepositoryName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString . . .
func (ed *RepositoryBranchProtectionUpdatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository branch protection %s was updated", ed.RepositoryName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *UserCreatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s was added with the user details: %+v.", args.UserName, args.UserModel)
	return data, true
}

// GetEventSummaryString . . .
func (ed *UserUpdatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	return fmt.Sprintf("The user %s was updated with the user details: %+v.", args.UserName, *args.UserModel), true
}

// GetEventSummaryString . . .
func (ed *UserDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user ID %s was deleted by the user %s.", ed.DeletedUserID, args.UserName)
	return data, true
}

// GetEventSummaryString . . .
func (ed *CompanyACLRequestAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s with ID %s and with the email %s requested a company invitation",
		ed.UserName, ed.UserID, ed.UserEmail)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CompanyACLRequestApprovedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("A company invite was approved for the user %s with the ID of %s with the email %s",
		ed.UserName, ed.UserID, ed.UserEmail)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CompanyACLRequestDeniedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("A company invite was denied for the user %s with the ID of %s with the email %s",
		ed.UserName, ed.UserID, ed.UserEmail)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CompanyACLUserAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user with LF username %s was added to the access list for the company %s by the user %s.",
		args.LFUser.Name, args.CompanyName, args.UserName)
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLATemplateCreatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := "The PDF templates were created"
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *GitHubOrganizationAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub organization %s was added with auto-enabled set to %t with branch protection enabled set to %t",
		ed.GitHubOrganizationName, ed.AutoEnabled, ed.BranchProtectionEnabled)
	if ed.AutoEnabledClaGroupID != "" {
		data = data + fmt.Sprintf(" with auto-enabled-cla-group set to %s", ed.AutoEnabledClaGroupID)
	}
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *GitHubOrganizationDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub organization %s was deleted", ed.GitHubOrganizationName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for project %s", args.ProjectName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *GitHubOrganizationUpdatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("GitHub Organization: %s was updated with auto-enabled: %t",
		ed.GitHubOrganizationName, ed.AutoEnabled)
	if ed.AutoEnabledClaGroupID != "" {
		data = data + fmt.Sprintf(" with auto-enabled-cla-group: %s", ed.AutoEnabledClaGroupID)
	}
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for project %s", args.ProjectName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CCLAApprovalListRequestApprovedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s approved a CCLA approval request", args.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CCLAApprovalListRequestRejectedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s rejected a CCLA approval request", args.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAManagerRequestCreatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s added a CLA Manager request", args.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAManagerCreatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s was added as CLA Manager", ed.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAManagerDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s was removed as CLA Manager", ed.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAManagerRequestApprovedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager request for the user %s was approved by the CLA Manager %s",
		ed.UserName, ed.ManagerName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAManagerRequestDeniedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager request for the user %s was denied by the CLA Manager %s",
		ed.UserName, ed.ManagerName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAManagerRequestDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager request for the user %s was deleted by the CLA Manager %s",
		ed.UserName, ed.ManagerName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAApprovalListAddEmailData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager %s added the email %s to the approval list", ed.UserName, ed.ApprovalListEmail)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAApprovalListRemoveEmailData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager %s removed the email %s from the approval list", ed.UserName, ed.ApprovalListEmail)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAApprovalListAddDomainData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager %s added the domain %s to the approval list", ed.UserName, ed.ApprovalListDomain)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAApprovalListRemoveDomainData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager %s removed the domain %s from the approval list", ed.UserName, ed.ApprovalListDomain)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAApprovalListAddGitHubUsernameData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager %s added the GitHub username %s to the approval list", ed.UserName, ed.ApprovalListGitHubUsername)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAApprovalListRemoveGitHubUsernameData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager %s removed the GitHub username %s from the approval list", ed.UserName, ed.ApprovalListGitHubUsername)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAApprovalListAddGitHubOrgData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager %s added the GitHub organization %s to the approval list", ed.UserName, ed.ApprovalListGitHubOrg)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAApprovalListRemoveGitHubOrgData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager %s removed the GitHub organization %s from the approval list", ed.UserName, ed.ApprovalListGitHubOrg)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CCLAApprovalListRequestCreatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s created a CCLA Approval Request", args.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *ApprovalListGitHubOrganizationAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager %s added the GitHub organization %s to the approval list", args.UserName, ed.GitHubOrganizationName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *ApprovalListGitHubOrganizationDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Manager %s removed the GitHub organization %s from the approval list", args.UserName, ed.GitHubOrganizationName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *ClaManagerAccessRequestAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s has requested to be CLA Manager", args.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *ClaManagerAccessRequestDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s has deleted a request to be CLA Manager", args.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAGroupCreatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Group %s was created by the user %s.", args.ProjectName, args.UserName)
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAGroupUpdatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Group %s was updated by the user %s.", args.ProjectName, args.UserName)
	return data, true
}

// GetEventSummaryString . . .
func (ed *CLAGroupDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Group %s was deleted by the user %s.", args.ProjectName, args.UserName)
	return data, true
}

// GetEventSummaryString . . .
func (ed *GerritProjectDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("%d Gerrit repositories were deleted due to CLA Group/Project deletion", ed.DeletedCount)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	data = data + "."
	return data, false
}

// GetEventSummaryString . . .
func (ed *GerritAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The Gerrit repository %s was added by the user %s", ed.GerritRepositoryName, args.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *GerritDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The Gerrit repository %s was deleted by the user %s", ed.GerritRepositoryName, args.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *GerritUserAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The username %s was add to the gerrit group %s", ed.Username, ed.GroupName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *GerritUserRemovedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The username %s was removed from the gerrit group %s", ed.Username, ed.GroupName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *GitHubProjectDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("%d GitHub repositories were deleted due to CLA Group/project deletion",
		ed.DeletedCount)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *SignatureProjectInvalidatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("%d signatures were invalidated (approved set to false) due to CLA Group/Project %s deletion.",
		ed.InvalidatedCount, args.ProjectName)
	return data, true
}

// GetEventSummaryString . . .
func (ed *SignatureInvalidatedApprovalRejectionEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	reason := "No reason"
	if ed.Email != "" {
		reason = fmt.Sprintf("Email: %s approval removal ", ed.Email)
	} else if ed.GHUsername != "" {
		reason = fmt.Sprintf("GH Username: %s approval removal ", ed.GHUsername)
	}
	data := fmt.Sprintf("Signature ID: %s invalidated (approved set to false) due to %s ", ed.SignatureID, reason)
	return data, true
}

// GetEventSummaryString . . .
func (ed *ContributorNotifyCompanyAdminData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s notified the company admin %s by the email address %s",
		args.UserName, ed.AdminName, ed.AdminEmail)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *ContributorNotifyCLADesignee) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s notified the CLA Designee %s by email %s", args.UserName, ed.DesigneeName, ed.DesigneeEmail)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *ContributorAssignCLADesignee) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s was assigned as CLA Manager Designee", ed.DesigneeName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *UserConvertToContactData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s was converted to a contact", args.LFUser.Name)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *AssignRoleScopeData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s was added to the role %s", args.LFUser.Name, ed.Role)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString . . .
func (ed *ClaManagerRoleCreatedData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s was added to the role %s", ed.UserName, ed.Role)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, false
}

// GetEventSummaryString . . .
func (ed *ClaManagerRoleDeletedData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s was removed from the role %s", ed.UserName, ed.Role)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, false
}
