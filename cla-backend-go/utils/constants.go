// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

// String400 string version of 400 - http bad request
const String400 = "400"

// String401 string version of 401 - http unauthorized
const String401 = "401"

// String403 string version of 403 - http forbidden
const String403 = "403"

// String404 string version of 404 - http not found
const String404 = "404"

// String409 string version of 409 - http conflict
const String409 = "409"

// String500 string version of 500 - http internal server error
const String500 = "500"

// EasyCLA400BadRequest common string for handler bad request error messages
const EasyCLA400BadRequest = "EasyCLA - 400 Bad Request"

// EasyCLA401Unauthorized common string for handler unauthorized error messages
const EasyCLA401Unauthorized = "EasyCLA - 401 Unauthorized"

// EasyCLA403Forbidden common string for handler forbidden error messages
const EasyCLA403Forbidden = "EasyCLA - 403 Forbidden"

// EasyCLA404NotFound common string for handler not found error messages
const EasyCLA404NotFound = "EasyCLA - 404 Not Found"

// EasyCLA409Conflict common string for handler conflict error messages
const EasyCLA409Conflict = "EasyCLA - 409 Conflict"

// EasyCLA500InternalServerError common string for handler internal server error messages
const EasyCLA500InternalServerError = "EasyCLA - 500 Internal Server Error"

// GitHubBotName is the name of the GitHub bot
const GitHubBotName = "EasyCLA"

// GithubBranchProtectionPatternAll is Github Branch Protection Pattern that matches all branches
const GithubBranchProtectionPatternAll = "**/**"

// TheLinuxFoundation is the name of the super parent for many Salesforce Foundations/Project Groups
const TheLinuxFoundation = "The Linux Foundation"

// LFProjectsLLC is the LF project LLC name of the super parent for many Salesforce Foundations/Project Groups
const LFProjectsLLC = "LF Projects, LLC"

// ProjectUnfunded  is a constant that represents a SF project that is unfunded
const ProjectUnfunded = "Unfunded"

// ProjectFundedSupportedByParent is a constant that represents a SF project that is funded by the parent
const ProjectFundedSupportedByParent = "Supported by Parent Project"

// XREQUESTID is the client request id - used to trace a client request through the system/logs
const XREQUESTID = "x-request-id"

// CtxAuthUser the key for the authenticated user in the context
const CtxAuthUser = "authUser"

// CLAProjectManagerRole CLA project manager role identifier
const CLAProjectManagerRole = "project-manager"

// CLADesigneeRole CLA manager designee role identifier
const CLADesigneeRole = "cla-manager-designee"

// CLAManagerRole CLA manager role identifier
const CLAManagerRole = "cla-manager"

// CompanyAdminRole  Company admin for user
const CompanyAdminRole = "company-admin"

// CLASignatoryRole CLA signatory role identifier
const CLASignatoryRole = "cla-signatory"

// Lead representing type of user
const Lead = "lead"

// ContactRole contact role for user
const ContactRole = "contact"

// ProjectScope is the ACS project scope
const ProjectScope = "project"

// ProjectOrgScope is the ACS project + organiztion scope
const ProjectOrgScope = "project|organization"

// ClaTypeICLA represents individual contributor CLA records
const ClaTypeICLA = "icla"

// ClaTypeECLA represents employee contributor CLA records (acknowledgements)
const ClaTypeECLA = "ecla"

// ClaTypeCCLA represents corporate CLA records (includes approval lists)
const ClaTypeCCLA = "ccla"

// SignatureTypeCLA is the cla signature type in the DB
const SignatureTypeCLA = "cla"

// SignatureTypeCCLA is the ccla signature type in the DB
const SignatureTypeCCLA = "ccla"

// FileTypePDF is the pdf file type
const FileTypePDF = "pdf"

// FileTypeCSV is the csv file type
const FileTypeCSV = "csv"

// SignatureReferenceTypeUser is the signature reference type for user signatures - individual and employee
const SignatureReferenceTypeUser = "user"

// SignatureReferenceTypeCompany is the signature reference type for corporate signatures - signed by CLA Signatories, managed by CLA Managers
const SignatureReferenceTypeCompany = "company"

// ProjectTypeProjectGroup is the string that represents the Project Group type in a Project Service record
const ProjectTypeProjectGroup = "Project Group"

// ProjectTypeProject is a salesforce Project that is of Project type
const ProjectTypeProject = "Project"

// GitHubType is the repository type identifier for github
const GitHubType = "github"

// SortOrderAscending ascending sort order constant
const SortOrderAscending = "asc"

// SortOrderDescending descending sort order constant
const SortOrderDescending = "desc"

// RecordDeleted dynamo event for deleting a record
const RecordDeleted = "REMOVE"

// RecordModified dynamo event on modifying a record
const RecordModified = "MODIFY"

// RecordAdded dynami event on adding a record
const RecordAdded = "INSERT"

// GithubRepoNotFound is a string that indicates the GitHub repository is not found
const GithubRepoNotFound = "GitHub repository not found"

// GithubRepoExists  is a string that indicates the GitHub repository already exists
const GithubRepoExists = "GitHub repository exists"

// GitHubEmailLabel represents the GH Email label used for email
const GitHubEmailLabel = "GitHub Email Address"

// GitHubUserLabel represents the GH username Label used for email
const GitHubUserLabel = "GitHub Username"

// GitLab is the GitLab spelled out with the proper case
const GitLab = "GitLab"

// GitLabLower is the GitLab spelled out in lower case
const GitLabLower = "gitlab"

// GitLabRepoNotFound is a string that indicates the GitLab repository is not found
const GitLabRepoNotFound = "GitLab repository not found"

// GitLabDuplicateRepoFound is a string that indicates that duplicate GitLab repositories were found
const GitLabDuplicateRepoFound = "Duplicate GitLab repositories were found"

// GitLabRepoExists  is a string that indicates the GitLab repository already exists
const GitLabRepoExists = "GitLab repository exists"

// GitLabEmailLabel represents the GitLab Email label used for email
const GitLabEmailLabel = "GitLab Email Address"

// GitLabUserLabel represents the GitLab username Label used for email
const GitLabUserLabel = "GitLab Username"

// EmailLabel represents LF/EasyCLA Email address
const EmailLabel = "Email Address"

// UserLabel represents the LF/EasyCLA username
const UserLabel = "Username"

// EmailDomainCriteria represents approval based on email domain
const EmailDomainCriteria = "Email Domain Criteria"

// EmailCriteria represents approvals based on email addresses
const EmailCriteria = "Email Criteria"

// AddApprovals is an action for adding approvals
const AddApprovals = "AddApprovals"

// RemoveApprovals is an action for removing approvals
const RemoveApprovals = "RemoveApprovals"

// GitHubUsernameCriteria represents criteria based on GitHub username
const GitHubUsernameCriteria = "GitHubUsername"

// GitHubOrgCriteria represents approvals based on GitHub org membership
const GitHubOrgCriteria = "GitHub Org Criteria"

// GitlabUsernameCriteria represents criteria based on gitlab username
const GitlabUsernameCriteria = "GitHubUsername"

// GitlabOrgCriteria represents approvals based on gitlab org group membership
const GitlabOrgCriteria = "Gitlab Org Criteria"

// SignatureQueryDefaultAll the signature query default active value - A flag to indicate how a default signature
// query should return data - show only 'active' signatures or 'all' signatures when no other query signed/approved
// params are provided
const SignatureQueryDefaultAll = "all"

// SignatureQueryDefaultActive the signature query default active value - A flag to indicate how a default signature
// query should return data - show only 'active' signatures or 'all' signatures when no other query signed/approved
// params are provided
const SignatureQueryDefaultActive = "active"

// GitLabRepositoryType representing the GitLab repository type
const GitLabRepositoryType = "GitLab"

// GitHubRepositoryType representing the GitLab repository type
const GitHubRepositoryType = "GitHub"

// ContextKey is the key for the context
type contextKey string

const XREQUESTIDKey contextKey = "x-request-id"

const GithubUsernameApprovalCriteria = "githubUsername"

const GithubOrgApprovalCriteria = "githubOrg"

const GitlabUsernameApprovalCriteria = "gitlabUsername"

const GitlabOrgApprovalCriteria = "gitlabOrg"

const EmailApprovalCriteria = "email"

const DomainApprovalCriteria = "domain"
