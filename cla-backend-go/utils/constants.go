// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

// String400 string version of 400 - http bad request
const String400 = "400"

// String403 string version of 403 - http not authorized
const String403 = "403"

// String404 string version of 404 - http not found
const String404 = "404"

// String409 string version of 409 - http conflict
const String409 = "409"

// String500 string version of 500 - http internal server error
const String500 = "500"

// EasyCLA400BadRequest common string for handler bad request error messages
const EasyCLA400BadRequest = "EasyCLA - 400 Bad Request"

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

// TheLinuxFoundation is the name of the super parent for many Salesforce Foundations/Project Groups
const TheLinuxFoundation = "The Linux Foundation"

// ProjectUnfunded  is a constant that represents a SF project that is unfunded
const ProjectUnfunded = "Unfunded"

// ProjectFundedSupportedByParent is a constant that represents a SF project that is funded by the parent
const ProjectFundedSupportedByParent = "Supported by Parent Project"

// XREQUESTID is the client request id - used to trace a client request through the system/logs
const XREQUESTID = "x-request-id"

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

//ContactRole contact role for user
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

//RecordModified dynamo event on modifying a record
const RecordModified = "MODIFY"

//RecordAdded dynami event on adding a record
const RecordAdded = "INSERT"

//GithubRepoNotFound representing not found repos for CLAGroupID
const GithubRepoNotFound = "github repository not found"
