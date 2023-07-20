package interfaces

import (
	"context"
	"sync"

	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	signatureModels "github.com/communitybridge/easycla/cla-backend-go/signatures/models"
)

// SignatureRepository interface defines the functions for the the signature repository
type SignatureRepository interface {
	GetGithubOrganizationsFromApprovalList(ctx context.Context, signatureID string) ([]models.GithubOrg, error)
	AddGithubOrganizationToApprovalList(ctx context.Context, signatureID, githubOrganizationID string) ([]models.GithubOrg, error)
	DeleteGithubOrganizationFromApprovalList(ctx context.Context, signatureID, githubOrganizationID string) ([]models.GithubOrg, error)
	ValidateProjectRecord(ctx context.Context, signatureID, note string) error
	InvalidateProjectRecord(ctx context.Context, signatureID, note string) error

	GetSignature(ctx context.Context, signatureID string) (*models.Signature, error)
	GetActivePullRequestMetadata(ctx context.Context, gitHubAuthorUsername, gitHubAuthorEmail string) (*signatureModels.ActivePullRequest, error)
	GetGitLabActiveMergeRequestMetadata(ctx context.Context, gitLabAuthorUsername, gitLabAuthorEmail string) (*signatureModels.ActiveGitLabPullRequest, error)
	GetIndividualSignature(ctx context.Context, claGroupID, userID string, approved, signed *bool) (*models.Signature, error)
	GetCorporateSignature(ctx context.Context, claGroupID, companyID string, approved, signed *bool) (*models.Signature, error)
	GetSignatureACL(ctx context.Context, signatureID string) ([]string, error)
	GetProjectSignatures(ctx context.Context, params signatures.GetProjectSignaturesParams) (*models.Signatures, error)
	CreateProjectSummaryReport(ctx context.Context, params signatures.CreateProjectSummaryReportParams) (*models.SignatureReport, error)
	GetProjectCompanySignature(ctx context.Context, companyID, projectID string, approved, signed *bool, nextKey *string, pageSize *int64) (*models.Signature, error)
	GetProjectCompanySignatures(ctx context.Context, companyID, projectID string, approved, signed *bool, nextKey *string, sortOrder *string, pageSize *int64) (*models.Signatures, error)
	GetProjectCompanyEmployeeSignatures(ctx context.Context, params signatures.GetProjectCompanyEmployeeSignaturesParams, criteria *signatureModels.ApprovalCriteria) (*models.Signatures, error)
	GetProjectCompanyEmployeeSignature(ctx context.Context, companyModel *models.Company, claGroupModel *models.ClaGroup, employeeUserModel *models.User, wg *sync.WaitGroup, resultChannel chan<- *signatureModels.EmployeeModel, errorChannel chan<- error)
	CreateProjectCompanyEmployeeSignature(ctx context.Context, companyModel *models.Company, claGroupModel *models.ClaGroup, employeeUserModel *models.User) error
	GetCompanySignatures(ctx context.Context, params signatures.GetCompanySignaturesParams, pageSize int64, loadACL bool) (*models.Signatures, error)
	GetCompanyIDsWithSignedCorporateSignatures(ctx context.Context, claGroupID string) ([]signatureModels.SignatureCompanyID, error)
	GetUserSignatures(ctx context.Context, params signatures.GetUserSignaturesParams, pageSize int64) (*models.Signatures, error)
	ProjectSignatures(ctx context.Context, projectID string) (*models.Signatures, error)
	UpdateApprovalList(ctx context.Context, claManager *models.User, claGroupModel *models.ClaGroup, companyID string, params *models.ApprovalList, eventArgs *events.LogEventArgs) (*models.Signature, error)
	AddCLAManager(ctx context.Context, signatureID, claManagerID string) (*models.Signature, error)
	RemoveCLAManager(ctx context.Context, signatureID, claManagerID string) (*models.Signature, error)
	AddSigTypeSignedApprovedID(ctx context.Context, signatureID string, val string) error
	AddUsersDetails(ctx context.Context, signatureID string, userID string) error
	AddSignedOn(ctx context.Context, signatureID string) error
	GetClaGroupICLASignatures(ctx context.Context, claGroupID string, searchTerm *string, approved, signed *bool, pageSize int64, nextKey string, withExtraDetails bool) (*models.IclaSignatures, error)
	GetClaGroupCorporateContributors(ctx context.Context, claGroupID string, companyID *string, pageSize *int64, nextKey *string, searchTerm *string) (*models.CorporateContributorList, error)
	EclaAutoCreate(ctx context.Context, signatureID string, autoCreateECLA bool) error
	ActivateSignature(ctx context.Context, signatureID string) error
}
