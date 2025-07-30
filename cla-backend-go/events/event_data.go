// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

const (
	// noReason constant value
	noReason = "No reason"
)

// EventData returns event data string which is used for event logging and containsPII field
type EventData interface {
	GetEventDetailsString(args *LogEventArgs) (eventData string, containsPII bool)
	GetEventSummaryString(args *LogEventArgs) (eventData string, containsPII bool)
}

// CLAGroupEnrolledProjectData event data model
type CLAGroupEnrolledProjectData struct {
}

// CLAGroupUnenrolledProjectData event data model
type CLAGroupUnenrolledProjectData struct {
}

// ProjectServiceCLAEnabledData event data model
type ProjectServiceCLAEnabledData struct {
}

// ProjectServiceCLADisabledData event data model
type ProjectServiceCLADisabledData struct {
}

// RepositoryAddedEventData event data model
type RepositoryAddedEventData struct {
	RepositoryName string
	RepositoryType string
}

// RepositoryDisabledEventData event data model
type RepositoryDisabledEventData struct {
	RepositoryName       string
	RepositoryExternalID int64
	RepositoryType       string
}

// RepositoryDeletedEventData event data model
type RepositoryDeletedEventData struct {
	RepositoryName       string
	RepositoryExternalID int64
}

// RepositoryRenamedEventData event data model
type RepositoryRenamedEventData struct {
	NewRepositoryName string
	OldRepositoryName string
}

// RepositoryTransferredEventData event data model
type RepositoryTransferredEventData struct {
	RepositoryName   string
	OldGithubOrgName string
	NewGithubOrgName string
}

// RepositoryUpdatedEventData event data model
type RepositoryUpdatedEventData struct {
	RepositoryName string
}

// RepositoryBranchProtectionAddedEventData event data model
type RepositoryBranchProtectionAddedEventData struct {
	RepositoryName string
}

// RepositoryBranchProtectionDisabledEventData event data model
type RepositoryBranchProtectionDisabledEventData struct {
	RepositoryName string
}

// RepositoryBranchProtectionUpdatedEventData event data model
type RepositoryBranchProtectionUpdatedEventData struct {
	RepositoryName string
}

// GerritProjectDeletedEventData event data model
type GerritProjectDeletedEventData struct {
	DeletedCount int
}

// GerritAddedEventData data model
type GerritAddedEventData struct {
	GerritRepositoryName string
}

// GerritDeletedEventData data model
type GerritDeletedEventData struct {
	GerritRepositoryName string
}

// GerritUserAddedEventData data model
type GerritUserAddedEventData struct {
	Username  string
	GroupName string
}

// GerritUserRemovedEventData data model
type GerritUserRemovedEventData struct {
	Username  string
	GroupName string
}

// GitHubProjectDeletedEventData data model
type GitHubProjectDeletedEventData struct {
	DeletedCount int
}

// SignatureProjectInvalidatedEventData data model
type SignatureProjectInvalidatedEventData struct {
	InvalidatedCount int
}

// SignatureInvalidatedApprovalRejectionEventData data model
type SignatureInvalidatedApprovalRejectionEventData struct {
	GHUsername  string
	Email       string
	SignatureID string
	CLAManager  *models.User
	CLAGroupID  string
}

// UserCreatedEventData data model
type UserCreatedEventData struct{}

// UserDeletedEventData data model
type UserDeletedEventData struct {
	DeletedUserID string
}

// UserUpdatedEventData data model
type UserUpdatedEventData struct{}

// CompanyACLRequestAddedEventData data model
type CompanyACLRequestAddedEventData struct {
	UserName  string
	UserID    string
	UserEmail string
}

// CompanyACLRequestApprovedEventData data model
type CompanyACLRequestApprovedEventData struct {
	UserName  string
	UserID    string
	UserEmail string
}

// CompanyACLRequestDeniedEventData data model
type CompanyACLRequestDeniedEventData struct {
	UserName  string
	UserID    string
	UserEmail string
}

// CompanyACLUserAddedEventData data model
type CompanyACLUserAddedEventData struct {
	UserLFID string
}

// CLATemplateCreatedEventData data model
type CLATemplateCreatedEventData struct {
	TemplateName string
	OldPOC       string
	NewPOC       string
}

// GitHubOrganizationAddedEventData data model
type GitHubOrganizationAddedEventData struct {
	GitHubOrganizationName  string
	AutoEnabled             bool
	AutoEnabledClaGroupID   string
	BranchProtectionEnabled bool
}

// GitHubOrganizationDeletedEventData data model
type GitHubOrganizationDeletedEventData struct {
	GitHubOrganizationName string
}

// GitHubOrganizationUpdatedEventData data model
type GitHubOrganizationUpdatedEventData struct {
	GitHubOrganizationName  string
	AutoEnabled             bool
	AutoEnabledClaGroupID   string
	BranchProtectionEnabled bool
}

// GitLabOrganizationAddedEventData data model
type GitLabOrganizationAddedEventData struct {
	GitLabOrganizationName  string
	AutoEnabled             bool
	AutoEnabledClaGroupID   string
	BranchProtectionEnabled bool
}

// GitLabOrganizationDeletedEventData data model
type GitLabOrganizationDeletedEventData struct {
	GitLabOrganizationName string
}

// GitLabOrganizationUpdatedEventData data model
type GitLabOrganizationUpdatedEventData struct {
	GitLabOrganizationName string
	GitLabGroupID          int64
	AutoEnabled            bool
	AutoEnabledClaGroupID  string
}

// CCLAApprovalListRequestCreatedEventData data model
type CCLAApprovalListRequestCreatedEventData struct {
	RequestID string
}

// CCLAApprovalListRequestApprovedEventData data model
type CCLAApprovalListRequestApprovedEventData struct {
	RequestID string
}

// CCLAApprovalListRequestRejectedEventData data model
type CCLAApprovalListRequestRejectedEventData struct {
	RequestID string
}

// CLAManagerCreatedEventData data model
type CLAManagerCreatedEventData struct {
	CompanyName string
	ProjectName string
	UserName    string
	UserEmail   string
	UserLFID    string
}

// CLAManagerDeletedEventData data model
type CLAManagerDeletedEventData struct {
	CompanyName string
	ProjectName string
	UserName    string
	UserEmail   string
	UserLFID    string
}

// CLAManagerRequestCreatedEventData data model
type CLAManagerRequestCreatedEventData struct {
	RequestID   string
	CompanyName string
	ProjectName string
	UserName    string
	UserEmail   string
	UserLFID    string
}

// CLAManagerRequestApprovedEventData data model
type CLAManagerRequestApprovedEventData struct {
	RequestID    string
	CompanyName  string
	ProjectName  string
	UserName     string
	UserEmail    string
	ManagerName  string
	ManagerEmail string
}

// CLAManagerRequestDeniedEventData data model
type CLAManagerRequestDeniedEventData struct {
	RequestID    string
	CompanyName  string
	ProjectName  string
	UserName     string
	UserEmail    string
	ManagerName  string
	ManagerEmail string
}

// CLAManagerRequestDeletedEventData data model
type CLAManagerRequestDeletedEventData struct {
	RequestID    string
	CompanyName  string
	ProjectName  string
	UserName     string
	UserEmail    string
	ManagerName  string
	ManagerEmail string
}

// CLAApprovalListAddEmailData data model
type CLAApprovalListAddEmailData struct {
	ApprovalListEmail string
}

// CLAApprovalListRemoveEmailData data model
type CLAApprovalListRemoveEmailData struct {
	ApprovalListEmail string
}

// CLAApprovalListAddDomainData data model
type CLAApprovalListAddDomainData struct {
	ApprovalListDomain string
}

// CLAApprovalListRemoveDomainData data model
type CLAApprovalListRemoveDomainData struct {
	ApprovalListDomain string
}

// CLAApprovalListAddGitHubUsernameData data model
type CLAApprovalListAddGitHubUsernameData struct {
	ApprovalListGitHubUsername string
}

// CLAApprovalListRemoveGitHubUsernameData data model
type CLAApprovalListRemoveGitHubUsernameData struct {
	ApprovalListGitHubUsername string
}

// CLAApprovalListAddGitHubOrgData data model
type CLAApprovalListAddGitHubOrgData struct {
	ApprovalListGitHubOrg string
}

// CLAApprovalListRemoveGitHubOrgData data model
type CLAApprovalListRemoveGitHubOrgData struct {
	ApprovalListGitHubOrg string
}

// CLAApprovalListAddGitLabUsernameData data model
type CLAApprovalListAddGitLabUsernameData struct {
	ApprovalListGitLabUsername string
}

// CLAApprovalListRemoveGitLabUsernameData data model
type CLAApprovalListRemoveGitLabUsernameData struct {
	ApprovalListGitLabUsername string
}

// CLAApprovalListAddGitLabGroupData data model
type CLAApprovalListAddGitLabGroupData struct {
	ApprovalListGitLabGroup string
}

// CLAApprovalListRemoveGitLabGroupData data model
type CLAApprovalListRemoveGitLabGroupData struct {
	ApprovalListGitLabGroup string
}

// ApprovalListGitHubOrganizationAddedEventData data model
type ApprovalListGitHubOrganizationAddedEventData struct {
	GitHubOrganizationName string
}

// ApprovalListGitHubOrganizationDeletedEventData data model
type ApprovalListGitHubOrganizationDeletedEventData struct {
	GitHubOrganizationName string
}

// ClaManagerAccessRequestAddedEventData data model
type ClaManagerAccessRequestAddedEventData struct {
	ProjectName string
	CompanyName string
}

// ClaManagerAccessRequestDeletedEventData data model
type ClaManagerAccessRequestDeletedEventData struct {
	RequestID string
}

// CLAGroupCreatedEventData data model
type CLAGroupCreatedEventData struct{}

// CLAGroupUpdatedEventData data model
type CLAGroupUpdatedEventData struct {
	NewClaGroupName        string
	NewClaGroupDescription string
	OldClaGroupName        string
	OldClaGroupDescription string
}

// CLAGroupDeletedEventData data model
type CLAGroupDeletedEventData struct{}

// ContributorNotifyCompanyAdminData data model
type ContributorNotifyCompanyAdminData struct {
	AdminName  string
	AdminEmail string
}

// ContributorNotifyCLADesignee data model
type ContributorNotifyCLADesignee struct {
	DesigneeName  string
	DesigneeEmail string
}

// ContributorAssignCLADesignee data model
type ContributorAssignCLADesignee struct {
	DesigneeName  string
	DesigneeEmail string
}

// UserConvertToContactData data model
type UserConvertToContactData struct {
	UserName  string
	UserEmail string
}

// AssignRoleScopeData data model
type AssignRoleScopeData struct {
	Role      string
	Scope     string
	UserName  string
	UserEmail string
}

// ClaManagerRoleCreatedData data model
type ClaManagerRoleCreatedData struct {
	Role      string
	Scope     string
	UserName  string
	UserEmail string
}

// ClaManagerRoleDeletedData data model
type ClaManagerRoleDeletedData struct {
	Role      string
	Scope     string
	UserName  string
	UserEmail string
}

// SignatureAutoCreateECLAUpdatedEventData data model
type SignatureAutoCreateECLAUpdatedEventData struct {
	AutoCreateECLA bool
}

type IndividualSignatureSignedEventData struct {
	ProjectName string
	Username    string
	ProjectID   string
}

type CorporateSignatureSignedEventData struct {
	ProjectName   string
	CompanyName   string
	SignatoryName string
}

// BypassCLAEventData event data model
type BypassCLAEventData struct {
	Repo   string
	Config string
	Actor  string
}

func (ed *BypassCLAEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("repo='%s', config='%s', actor='%s'", ed.Repo, ed.Config, ed.Actor)
	return data, true
}

func (ed *BypassCLAEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("repo='%s', config='%s', actor='%s'", ed.Repo, ed.Config, ed.Actor)
	return data, true
}

func (ed *CorporateSignatureSignedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The signature was signed for the project %s and company %s by %s", args.ProjectName, ed.CompanyName, ed.SignatoryName)
	if args.UserName != "" {
		data = fmt.Sprintf("%s by the user %s", data, args.UserName)
	}
	data = fmt.Sprintf("%s.", data)
	return data, true
}

func (ed *CorporateSignatureSignedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The signature was signed for the project %s and company %s by %s", args.ProjectName, ed.CompanyName, ed.SignatoryName)
	data = fmt.Sprintf("%s.", data)
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *SignatureAutoCreateECLAUpdatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {

	data := "Auto-create ECLAs for contributors was"
	if ed.AutoCreateECLA {
		data = data + " enabled"
	} else {
		data = data + " disabled"
	}
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAGroupEnrolledProjectData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The project %s (%s) was enrolled into the CLA Group %s (%s)", args.ProjectName, args.ProjectID, args.CLAGroupName, args.CLAGroupID)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAGroupUnenrolledProjectData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The project %s (%s) was unenrolled from the CLA Group %s (%s)", args.ProjectName, args.ProjectID, args.CLAGroupName, args.CLAGroupID)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *ProjectServiceCLAEnabledData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Service for the project %s (%s) was enabled", args.ProjectName, args.ProjectID)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *ProjectServiceCLADisabledData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA Service for the project %s (%s) was disabled", args.ProjectName, args.ProjectID)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *RepositoryAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	repositoryType := utils.GitHubRepositoryType
	if ed.RepositoryType != "" {
		repositoryType = ed.RepositoryType
	}
	data := fmt.Sprintf("The %s repository: %s was added for the project %s", repositoryType, ed.RepositoryName, args.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *RepositoryDisabledEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	repositoryType := utils.GitHubRepositoryType
	if repositoryType != "" {
		repositoryType = ed.RepositoryType
	}
	data := fmt.Sprintf("The %s repository", repositoryType) // nolint
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if ed.RepositoryName != "" {
		data = data + fmt.Sprintf(" with repository name %s", ed.RepositoryName)
	}
	if ed.RepositoryExternalID > 0 {
		data = data + fmt.Sprintf(" with repository external ID %d", ed.RepositoryExternalID)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectSFID)
	}
	data = data + " was disabled"
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *RepositoryDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := "The GitHub repository " // nolint
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if ed.RepositoryName != "" {
		data = data + fmt.Sprintf(" with repository name %s", ed.RepositoryName)
	}
	if ed.RepositoryExternalID > 0 {
		data = data + fmt.Sprintf(" with repository external ID %d", ed.RepositoryExternalID)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectSFID)
	}
	data = data + " was deleted"
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *RepositoryRenamedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository renamed from %s to %s for the project %s", ed.OldRepositoryName, ed.NewRepositoryName, args.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *RepositoryTransferredEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository : %s transferred from %s to %s Github Organization for the project %s", ed.RepositoryName, ed.OldGithubOrgName, ed.NewGithubOrgName, args.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *RepositoryUpdatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository %s was updated for the project %s", ed.RepositoryName, args.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *RepositoryBranchProtectionAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository branch protection %s was added for the project %s", ed.RepositoryName, args.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *RepositoryBranchProtectionDisabledEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository branch protection %s was disabled for the project %s", ed.RepositoryName, args.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *RepositoryBranchProtectionUpdatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository branch protection %s was updated for the project %s", ed.RepositoryName, args.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *UserCreatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User was added : %+v", args.UserModel)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *UserUpdatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User details updated: %+v", *args.UserModel)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *UserDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User ID: %s was deleted", ed.DeletedUserID)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CompanyACLRequestAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s added pending invite with ID: %s and Email: %s for Company: %s",
		ed.UserName, ed.UserID, ed.UserEmail, args.CompanyName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CompanyACLRequestApprovedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("Access Aproved for User: %s, ID: %s, Email: %s Company Group: %s",
		ed.UserName, args.CompanyName, ed.UserID, ed.UserEmail)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CompanyACLRequestDeniedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("Access Denied for User: %s, ID: %s, Email: %s Company Group: %s.",
		ed.UserName, args.CompanyName, ed.UserID, ed.UserEmail)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CompanyACLUserAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User with LF Username: %s added to the ACL for Company: %s",
		args.LFUser.Name, args.CompanyName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLATemplateCreatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := "A CLA Group template was created or updated" // nolint
	if args.CLAGroupName != "" {
		data = fmt.Sprintf("%s for the CLA Group %s", data, args.CLAGroupName)
	}
	if ed.TemplateName != "" {
		data = fmt.Sprintf("%s using template: %s", data, ed.TemplateName)
	}
	if args.ProjectName != "" {
		data = fmt.Sprintf("%s for the project %s", data, args.ProjectName)
	}
	if args.UserName != "" {
		data = fmt.Sprintf("%s by the user %s", data, args.UserName)
	}
	data = fmt.Sprintf("%s.", data)

	if ed.OldPOC != "" && ed.NewPOC != "" {
		data = fmt.Sprintf("%s The point of contact email was changed from %s to %s.", data, ed.OldPOC, ed.NewPOC)
	} else if ed.NewPOC != "" {
		data = fmt.Sprintf("%s The point of contact email was set to %s.", data, ed.NewPOC)
	}

	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *GitHubOrganizationAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("GitHub Organization: %s was added with auto-enabled: %t, with branch protection enabled: %t",
		ed.GitHubOrganizationName, ed.AutoEnabled, ed.BranchProtectionEnabled)
	if ed.AutoEnabledClaGroupID != "" {
		data = data + fmt.Sprintf(" with auto-enabled-cla-group: %s", ed.AutoEnabledClaGroupID)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *GitHubOrganizationDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("GitHub Organization: %s was deleted ", ed.GitHubOrganizationName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *GitHubOrganizationUpdatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub Organization '%s' was updated", ed.GitHubOrganizationName)
	data = data + fmt.Sprintf(" with auto-enabled set to %t", ed.AutoEnabled)
	data = data + fmt.Sprintf(" with branch protection set to %t", ed.BranchProtectionEnabled)
	if ed.AutoEnabledClaGroupID != "" {
		data = data + fmt.Sprintf(" with auto-enabled-cla-group ID value of %s", ed.AutoEnabledClaGroupID)
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

// GetEventDetailsString returns the details string for this event
func (ed *GitLabOrganizationAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("GitLab Group: %s was added with auto-enabled: %t, with branch protection enabled: %t",
		ed.GitLabOrganizationName, ed.AutoEnabled, ed.BranchProtectionEnabled)
	if ed.AutoEnabledClaGroupID != "" {
		data = data + fmt.Sprintf(" with auto-enabled-cla-group: %s", ed.AutoEnabledClaGroupID)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *GitLabOrganizationDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("GitLab Group: %s was deleted ", ed.GitLabOrganizationName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *GitLabOrganizationUpdatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := "GitLab Group" // nolint
	if ed.GitLabOrganizationName != "" {
		data = fmt.Sprintf("%s with name: %s", data, ed.GitLabOrganizationName)
	}
	if ed.GitLabGroupID > 0 {
		data = fmt.Sprintf("%s with group ID: %d", data, ed.GitLabGroupID)
	}
	data = fmt.Sprintf("%s was updated with auto-enabled: %t", data, ed.AutoEnabled)
	if ed.AutoEnabledClaGroupID != "" {
		data = fmt.Sprintf("%s with auto-enabled-cla-group: %s", data, ed.AutoEnabledClaGroupID)
	}
	if args.ProjectName != "" {
		data = fmt.Sprintf("%s for the project %s", data, args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.UserName != "" {
		data = fmt.Sprintf("%s by the user %s", data, args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CCLAApprovalListRequestApprovedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s approved a CCLA Approval Request for Project: %s and Company: %s with Request ID: %s.",
		args.UserName, args.ProjectName, args.CompanyName, ed.RequestID)
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CCLAApprovalListRequestRejectedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s rejected a CCLA Approval Request for Project: %s, Company: %s with Request ID: %s.",
		args.UserName, args.ProjectName, args.CompanyName, ed.RequestID)
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAManagerRequestCreatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s, LFID: %s, Email: %s added CLA Manager Request: %s for Company: %s, Project: %s.",
		ed.UserName, ed.UserLFID, ed.UserEmail, ed.RequestID, ed.CompanyName, ed.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAManagerCreatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user: %s LFID: %s, email: %s was added as CLA Manager", ed.UserName, ed.UserLFID, ed.UserEmail)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if ed.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", ed.ProjectName)
	} else {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectSFID)
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

// GetEventDetailsString returns the details string for this event
func (ed *CLAManagerDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user: %s LFID: %s, email: %s was removed as CLA Manager", ed.UserName, ed.UserLFID, ed.UserEmail)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if ed.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", ed.ProjectName)
	} else {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
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

// GetEventDetailsString returns the details string for this event
func (ed *CLAManagerRequestApprovedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager Request: %s was approved for User %s, Email: %s by Manager: %s, Email: %s for Company: %s, Project: %s",
		ed.RequestID, ed.UserName, ed.UserEmail, ed.ManagerName, ed.ManagerEmail, ed.CompanyName, ed.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAManagerRequestDeniedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager Request: %s was denied for User %s, Email: %s by Manager: %s, Email: %s for Company: %s, Project: %s",
		ed.RequestID, ed.UserName, ed.UserEmail, ed.ManagerName, ed.ManagerEmail, ed.CompanyName, ed.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAManagerRequestDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("CLA Manager Request: %s was deleted for User %s, Email: %s by Manager: %s, Email: %s for Company: %s, Project: %s",
		ed.RequestID, ed.UserName, ed.UserEmail, ed.ManagerName, ed.ManagerEmail, ed.CompanyName, ed.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAApprovalListAddEmailData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The email address %s was added to the approval list", ed.ApprovalListEmail)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAApprovalListRemoveEmailData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The email address %s was removed from the approval list", ed.ApprovalListEmail)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAApprovalListAddDomainData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The email address domain %s was added to the approval list", ed.ApprovalListDomain)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAApprovalListRemoveDomainData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The email address domain %s was removed from the approval list", ed.ApprovalListDomain)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAApprovalListAddGitHubUsernameData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub username %s was added to the approval list", ed.ApprovalListGitHubUsername)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAApprovalListRemoveGitHubUsernameData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub username %s was removed from the approval list", ed.ApprovalListGitHubUsername)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAApprovalListAddGitHubOrgData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub organization %s was added to the approval list", ed.ApprovalListGitHubOrg)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAApprovalListRemoveGitHubOrgData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub organization %s was removed from the approval list", ed.ApprovalListGitHubOrg)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAApprovalListAddGitLabUsernameData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitLab username %s was added to the approval list", ed.ApprovalListGitLabUsername)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAApprovalListRemoveGitLabUsernameData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitLab username %s was removed from the approval list", ed.ApprovalListGitLabUsername)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAApprovalListAddGitLabGroupData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitLab group %s was added to the approval list", ed.ApprovalListGitLabGroup)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAApprovalListRemoveGitLabGroupData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitLab group %s was removed from the approval list", ed.ApprovalListGitLabGroup)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
	}
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CCLAApprovalListRequestCreatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CCLA Approval Request was created for the Project: %s, Company: %s with Request ID: %s",
		args.ProjectName, args.CompanyName, ed.RequestID)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *ApprovalListGitHubOrganizationAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub Organization: %s was added to the approval list for the Company %s, Project: %s",
		ed.GitHubOrganizationName, args.CompanyName, args.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *ApprovalListGitHubOrganizationDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub Organization: %s was removed from the approval list for the Company: %s, Project: %s",
		ed.GitHubOrganizationName, args.CompanyName, args.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *ClaManagerAccessRequestAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s has requested to be CLA Manager for Company %s, Project: %s.",
		args.UserName, ed.CompanyName, ed.ProjectName)
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *ClaManagerAccessRequestDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s has deleted CLA Manager Request with ID: %s.",
		args.UserName, ed.RequestID)
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAGroupCreatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA group %s was created", args.CLAGroupName)
	if args.CLAGroupID != "" {
		data = data + fmt.Sprintf(" with the CLA group ID %s", args.CLAGroupID)
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

// GetEventDetailsString returns the details string for this event
func (ed *CLAGroupUpdatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	var nameUpdated, descriptionUpdated bool

	data := "The CLA Group" // nolint
	if ed.NewClaGroupName != "" && ed.OldClaGroupName != ed.NewClaGroupName {
		data = fmt.Sprintf("%s name was updated to '%s'", data, ed.NewClaGroupName)
		nameUpdated = true
	}

	if args.CLAGroupID != "" {
		data = data + fmt.Sprintf(" with the CLA group ID %s", args.CLAGroupID)
	}

	if ed.NewClaGroupDescription != "" && ed.OldClaGroupDescription != ed.NewClaGroupDescription {
		descriptionUpdated = true
		if nameUpdated {
			data = fmt.Sprintf("%s and the description was updated to '%s'", data, ed.NewClaGroupDescription)
		} else {
			data = fmt.Sprintf("%s description was updated to '%s'", data, ed.NewClaGroupDescription)
		}
	}

	//shouldn't happen
	if !nameUpdated && !descriptionUpdated {
		data = data + " was updated"
	}

	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	return data + ".", true
}

// GetEventDetailsString returns the details string for this event
func (ed *CLAGroupDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA group %s was deleted", args.CLAGroupName)
	if args.CLAGroupID != "" {
		data = data + fmt.Sprintf(" with the CLA group ID %s", args.CLAGroupID)
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

// GetEventDetailsString returns the details string for this event
func (ed *GerritProjectDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("%d Gerrit Repositories were deleted due to CLA Group/Project: %s deletion",
		ed.DeletedCount, args.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *GerritAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("Gerrit Repository: %s was added", ed.GerritRepositoryName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *GerritDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("Gerrit Repository: %s was deleted", ed.GerritRepositoryName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *GerritUserAddedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The username %s was add to the gerrit group %s", ed.Username, ed.GroupName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *GerritUserRemovedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The username %s was removed from the gerrit group %s", ed.Username, ed.GroupName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *GitHubProjectDeletedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("%d GitHub Repositories were deleted due to CLA Group/Project: [%s] deletion",
		ed.DeletedCount, args.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *SignatureProjectInvalidatedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("%d Signatures were invalidated (approved set to false) due to CLA Group/Project: %s deletion",
		ed.InvalidatedCount, args.ProjectName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *SignatureInvalidatedApprovalRejectionEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	reason := noReason
	if ed.Email != "" {
		reason = fmt.Sprintf("Email: %s approval removal ", ed.Email)
	} else if ed.GHUsername != "" {
		reason = fmt.Sprintf("GH Username: %s approval removal ", ed.GHUsername)
	}
	data := fmt.Sprintf("Signature invalidated by %s (approved set to false) due to %s ", utils.GetBestUsername(ed.CLAManager), reason)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *ContributorNotifyCompanyAdminData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s notified Company Admin: %s by Email: %s for Company ID: %s, Name: %s.",
		args.UserName, ed.AdminName, ed.AdminEmail, args.CompanyName, args.CompanyID)
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *ContributorNotifyCLADesignee) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s notified CLA Designee: %s by Email: %s for Project Name : %s, ID: %s and Company Name: %s, ID: %s.",
		args.UserName, ed.DesigneeName, ed.DesigneeEmail,
		args.ProjectName, args.ProjectSFID,
		args.CompanyName, args.CompanyID)
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *ContributorAssignCLADesignee) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User Name: %s, Email: %s was assigned as CLA Manager Designee for project Name: %s, ID:  %s and Company Name: %s, ID: %s",
		ed.DesigneeName, ed.DesigneeEmail,
		args.ProjectName, args.ProjectSFID,
		args.CompanyName, args.CompanyID)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *UserConvertToContactData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s was converted to Contact state for Project: %s with ID: %s.",
		args.UserName, args.ProjectName, args.ProjectSFID)
	return data, true
}

// GetEventDetailsString returns the details string for this event
func (ed *AssignRoleScopeData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user '%s' with email '%s' was added to the role %s", ed.UserName, ed.UserEmail, ed.Role)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" with project SFID %s", args.ProjectName)
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

// GetEventDetailsString returns the details string for this event
func (ed *ClaManagerRoleCreatedData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s, Email: %s was added to Role: %s with Scope: %s by: %s.", ed.UserName, ed.UserEmail, ed.Role, ed.Scope, args.UserName)
	return data, false
}

// GetEventDetailsString returns the details string for this event
func (ed *ClaManagerRoleDeletedData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("User: %s, Email: %s was removed from Role: %s with Scope: %s by: %s.", ed.UserName, ed.UserEmail, ed.Role, ed.Scope, args.UserName)
	return data, false
}

// Event Summary started

// GetEventSummaryString returns the summary string for this event
func (ed *CLAGroupEnrolledProjectData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The project %s was enrolled into the CLA Group %s", args.ProjectName, args.CLAGroupName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAGroupUnenrolledProjectData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The project %s was unenrolled from the CLA Group %s", args.ProjectName, args.CLAGroupName)
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *ProjectServiceCLAEnabledData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := "CLA Service was enabled"
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

// GetEventSummaryString returns the summary string for this event
func (ed *ProjectServiceCLADisabledData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := "CLA Service was disabled"
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

// GetEventSummaryString returns the summary string for this event
func (ed *RepositoryAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	repositoryType := utils.GitHubRepositoryType
	if ed.RepositoryType != "" {
		repositoryType = ed.RepositoryType
	}
	data := fmt.Sprintf("The %s repository %s was added", repositoryType, ed.RepositoryName)
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

// GetEventSummaryString returns the summary string for this event
func (ed *RepositoryDisabledEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	repositoryType := utils.GitHubRepositoryType
	if ed.RepositoryType != "" {
		repositoryType = ed.RepositoryType
	}
	data := fmt.Sprintf("The %s repository", repositoryType) // nolint
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if ed.RepositoryName != "" {
		data = data + fmt.Sprintf(" with repository name %s", ed.RepositoryName)
	}
	if ed.RepositoryExternalID > 0 {
		data = data + fmt.Sprintf(" with repository external ID %d", ed.RepositoryExternalID)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectSFID)
	}
	data = data + " was disabled"
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *RepositoryDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := "The GitHub repository " // nolint
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if ed.RepositoryName != "" {
		data = data + fmt.Sprintf(" with repository name %s", ed.RepositoryName)
	}
	if ed.RepositoryExternalID > 0 {
		data = data + fmt.Sprintf(" with repository external ID %d", ed.RepositoryExternalID)
	}
	if args.ProjectSFID != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectSFID)
	}
	data = data + " was deleted"
	if args.CompanyName != "" {
		data = data + fmt.Sprintf(" for the company %s", args.CompanyName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *RepositoryRenamedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository was renamed from %s to %s", ed.OldRepositoryName, ed.NewRepositoryName)
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

// GetEventSummaryString returns the summary string for this event
func (ed *RepositoryTransferredEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub repository : %s was transferred from %s to %s Github Organization", ed.RepositoryName, ed.OldGithubOrgName, ed.NewGithubOrgName)
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the details string for this event
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

// GetEventSummaryString returns the details string for this event
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

// GetEventSummaryString returns the details string for this event
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

// GetEventSummaryString returns the summary string for this event
func (ed *UserCreatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s was added with the user details: %+v.", args.UserName, args.UserModel)
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *UserUpdatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	return fmt.Sprintf("The user %s was updated with the user details: %+v.", args.UserName, *args.UserModel), true
}

// GetEventSummaryString returns the summary string for this event
func (ed *UserDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user ID %s was deleted by the user %s.", ed.DeletedUserID, args.UserName)
	return data, true
}

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
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
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
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
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CompanyACLUserAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user with LF username %s was added to the access list for the company %s by the user %s.",
		args.LFUser.Name, args.CompanyName, args.UserName)
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLATemplateCreatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	// Same output as the details
	return ed.GetEventDetailsString(args)
}

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
func (ed *GitHubOrganizationUpdatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub Organization '%s' was updated", ed.GitHubOrganizationName)
	data = data + fmt.Sprintf(" with auto-enabled set to %t", ed.AutoEnabled)
	data = data + fmt.Sprintf(" with branch protection set to %t", ed.BranchProtectionEnabled)
	if ed.AutoEnabledClaGroupID != "" {
		data = data + fmt.Sprintf(" with auto-enabled-cla-group ID value of %s", ed.AutoEnabledClaGroupID)
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

// GetEventSummaryString returns the summary string for this event
func (ed *GitLabOrganizationAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitLab group %s was added with auto-enabled set to %t with branch protection enabled set to %t",
		ed.GitLabOrganizationName, ed.AutoEnabled, ed.BranchProtectionEnabled)
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

// GetEventSummaryString returns the summary string for this event
func (ed *GitLabOrganizationDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitLab group %s was deleted", ed.GitLabOrganizationName)
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

// GetEventSummaryString returns the summary string for this event
func (ed *GitLabOrganizationUpdatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := "The GitLab group" // nolint
	if ed.GitLabOrganizationName != "" {
		data = fmt.Sprintf("%s with name: %s", data, ed.GitLabOrganizationName)
	}
	if ed.GitLabGroupID > 0 {
		data = fmt.Sprintf("%s with group ID: %d", data, ed.GitLabGroupID)
	}
	data = fmt.Sprintf("%s was updated with auto-enabled: %t", data, ed.AutoEnabled)
	if ed.AutoEnabledClaGroupID != "" {
		data = fmt.Sprintf("%s with auto-enabled-cla-group: %s", data, ed.AutoEnabledClaGroupID)
	}
	if args.ProjectName != "" {
		data = fmt.Sprintf("%s for the project %s", data, args.ProjectName)
	}
	if args.UserName != "" {
		data = fmt.Sprintf("%s by the user %s", data, args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
func (ed *CLAManagerCreatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s was added as CLA Manager", ed.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if ed.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", ed.ProjectName)
	} else {
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

// GetEventSummaryString returns the summary string for this event
func (ed *CLAManagerDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s was removed as CLA Manager", ed.UserName)
	if args.CLAGroupName != "" {
		data = data + fmt.Sprintf(" for the CLA Group %s", args.CLAGroupName)
	}
	if ed.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", ed.ProjectName)
	} else {
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

// GetEventSummaryString returns the summary string for this event
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
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
func (ed *CLAApprovalListAddEmailData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The email address %s was added to the approval list", ed.ApprovalListEmail)
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
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAApprovalListRemoveEmailData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The email address %s was removed from the approval list", ed.ApprovalListEmail)
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
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAApprovalListAddDomainData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The email address domain %s was added to the approval list", ed.ApprovalListDomain)
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
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAApprovalListRemoveDomainData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The email address domain %s was removed from the approval list", ed.ApprovalListDomain)
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
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAApprovalListAddGitHubUsernameData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub username %s was added to the approval list", ed.ApprovalListGitHubUsername)
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
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAApprovalListRemoveGitHubUsernameData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub username %s was removed from the approval list", ed.ApprovalListGitHubUsername)
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
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAApprovalListAddGitHubOrgData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub organization %s was added to the approval list", ed.ApprovalListGitHubOrg)
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
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAApprovalListRemoveGitHubOrgData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitHub organization %s was removed from the approval list", ed.ApprovalListGitHubOrg)
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
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAApprovalListAddGitLabUsernameData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitLab username %s was added to the approval list", ed.ApprovalListGitLabUsername)
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
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAApprovalListRemoveGitLabUsernameData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitLab username %s was removed from the approval list", ed.ApprovalListGitLabUsername)
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
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAApprovalListAddGitLabGroupData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitLab group %s was added to the approval list", ed.ApprovalListGitLabGroup)
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
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAApprovalListRemoveGitLabGroupData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The GitLab group %s was removed from the approval list", ed.ApprovalListGitLabGroup)
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
		data = data + fmt.Sprintf(" by the CLA Manager %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
func (ed *CLAGroupCreatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA group %s was created", args.CLAGroupName)
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	return data + ".", true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAGroupUpdatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	var nameUpdated, descriptionUpdated bool

	data := "The CLA Group" // nolint
	if ed.NewClaGroupName != "" && ed.OldClaGroupName != ed.NewClaGroupName {
		data = fmt.Sprintf("%s name was updated to '%s'", data, ed.NewClaGroupName)
		nameUpdated = true
	}

	if ed.NewClaGroupDescription != "" && ed.OldClaGroupDescription != ed.NewClaGroupDescription {
		descriptionUpdated = true
		if nameUpdated {
			data = fmt.Sprintf("%s and the description was updated to '%s'", data, ed.NewClaGroupDescription)
		} else {
			data = fmt.Sprintf("%s description was updated to '%s'", data, ed.NewClaGroupDescription)
		}
	}

	//shouldn't happen
	if !nameUpdated && !descriptionUpdated {
		data = data + " was updated"
	}

	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	return data + ".", true
}

// GetEventSummaryString returns the summary string for this event
func (ed *CLAGroupDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The CLA group %s was deleted", args.CLAGroupName)
	if args.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", args.ProjectName)
	}
	if args.UserName != "" {
		data = data + fmt.Sprintf(" by the user %s", args.UserName)
	}
	data = data + "."
	return data, true
}

// GetEventSummaryString returns the summary string for this event
func (ed *GerritProjectDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("%d Gerrit repositories were deleted due to CLA Group/Project deletion", ed.DeletedCount)
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

// GetEventSummaryString returns the summary string for this event
func (ed *GerritAddedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The Gerrit repository %s was added", ed.GerritRepositoryName)
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

// GetEventSummaryString returns the summary string for this event
func (ed *GerritDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The Gerrit repository %s was deleted", ed.GerritRepositoryName)
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
func (ed *GitHubProjectDeletedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("%d GitHub repositories were deleted due to CLA Group/project deletion",
		ed.DeletedCount)
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

// GetEventSummaryString returns the summary string for this event
func (ed *SignatureProjectInvalidatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("%d signatures were invalidated (approved set to false) due to CLA Group/Project %s deletion",
		ed.InvalidatedCount, args.ProjectName)
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

// GetEventSummaryString returns the summary string for this event
func (ed *SignatureInvalidatedApprovalRejectionEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	reason := noReason
	if ed.Email != "" {
		reason = fmt.Sprintf("Email: %s approval removal ", ed.Email)
	} else if ed.GHUsername != "" {
		reason = fmt.Sprintf("GH Username: %s approval removal ", ed.GHUsername)
	}
	data := fmt.Sprintf("Signature invalidated by %s (approved set to false) due to %s", utils.GetBestUsername(ed.CLAManager), reason)
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
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

// GetEventSummaryString returns the summary string for this event
func (ed *UserConvertToContactData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user '%s' with email '%s' was converted to a contact", ed.UserName, ed.UserEmail)
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

// GetEventSummaryString returns the summary string for this event
func (ed *AssignRoleScopeData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user '%s' with email '%s' was added to the role %s", ed.UserName, ed.UserEmail, ed.Role)
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

// GetEventSummaryString returns the summary string for this event
func (ed *ClaManagerRoleCreatedData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user '%s' with email '%s' was added to the role %s", ed.UserName, ed.UserEmail, ed.Role)
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

// GetEventSummaryString returns the summary string for this event
func (ed *ClaManagerRoleDeletedData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user '%s' with email '%s' was added to the role %s", ed.UserName, ed.UserEmail, ed.Role)
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

// GetEventSummaryString returns the summary string for this event
func (ed *SignatureAutoCreateECLAUpdatedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s updated the auto-create ECLA flag to %t", args.LfUsername, ed.AutoCreateECLA)
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

func (ed *IndividualSignatureSignedEventData) GetEventSummaryString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s signed an individual CLA", args.LfUsername)
	if ed.ProjectName != "" {
		data = data + fmt.Sprintf(" for the project %s", ed.ProjectName)
	}
	if ed.ProjectID != "" {
		data = data + fmt.Sprintf(" with project ID: %s", ed.ProjectID)
	}
	return data + ".", false
}

func (ed *IndividualSignatureSignedEventData) GetEventDetailsString(args *LogEventArgs) (string, bool) {
	data := fmt.Sprintf("The user %s signed an individual CLA for project %s",
		args.LfUsername, ed.ProjectName)
	return data, false
}
