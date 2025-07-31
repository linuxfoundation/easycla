// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	gitlab_api "github.com/linuxfoundation/easycla/cla-backend-go/gitlab_api"
	service2 "github.com/linuxfoundation/easycla/cla-backend-go/project/service"

	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	"github.com/linuxfoundation/easycla/cla-backend-go/github_organizations"
	"github.com/linuxfoundation/easycla/cla-backend-go/repositories"

	"github.com/sirupsen/logrus"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	githubpkg "github.com/google/go-github/v37/github"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"golang.org/x/oauth2"
)

// SignatureService interface
type SignatureService interface {
	GetSignature(ctx context.Context, signatureID string) (*models.Signature, error)
	GetIndividualSignature(ctx context.Context, claGroupID, userID string, approved, signed *bool) (*models.Signature, error)
	GetIndividualSignatures(ctx context.Context, claGroupID, userID string, approved, signed *bool) ([]*models.Signature, error)
	GetCorporateSignature(ctx context.Context, claGroupID, companyID string, approved, signed *bool) (*models.Signature, error)
	GetCorporateSignatures(ctx context.Context, claGroupID, companyID string, approved, signed *bool) ([]*models.Signature, error)
	GetProjectSignatures(ctx context.Context, params signatures.GetProjectSignaturesParams) (*models.Signatures, error)
	GetCCLASignatures(ctx context.Context, signed, approved *bool) ([]*ItemSignature, error)
	CreateProjectSummaryReport(ctx context.Context, params signatures.CreateProjectSummaryReportParams) (*models.SignatureReport, error)
	GetProjectCompanySignature(ctx context.Context, companyID, projectID string, approved, signed *bool, nextKey *string, pageSize *int64) (*models.Signature, error)
	GetProjectCompanySignatures(ctx context.Context, params signatures.GetProjectCompanySignaturesParams) (*models.Signatures, error)
	GetProjectCompanyEmployeeSignatures(ctx context.Context, params signatures.GetProjectCompanyEmployeeSignaturesParams, criteria *ApprovalCriteria) (*models.Signatures, error)
	GetCompanySignatures(ctx context.Context, params signatures.GetCompanySignaturesParams) (*models.Signatures, error)
	GetCompanyIDsWithSignedCorporateSignatures(ctx context.Context, claGroupID string) ([]SignatureCompanyID, error)
	GetUserSignatures(ctx context.Context, params signatures.GetUserSignaturesParams, projectID *string) (*models.Signatures, error)
	InvalidateProjectRecords(ctx context.Context, projectID, note string) (int, error)
	CreateSignature(ctx context.Context, signature *ItemSignature) error
	UpdateSignature(ctx context.Context, signatureID string, updates map[string]interface{}) error
	SaveOrUpdateSignature(ctx context.Context, signature *ItemSignature) error
	HasUserSigned(ctx context.Context, user *models.User, projectID string) (*bool, *bool, error)

	GetGithubOrganizationsFromApprovalList(ctx context.Context, signatureID string, githubAccessToken string) ([]models.GithubOrg, error)
	AddGithubOrganizationToApprovalList(ctx context.Context, signatureID string, approvalListParams models.GhOrgWhitelist, githubAccessToken string) ([]models.GithubOrg, error)
	DeleteGithubOrganizationFromApprovalList(ctx context.Context, signatureID string, approvalListParams models.GhOrgWhitelist, githubAccessToken string) ([]models.GithubOrg, error)
	UpdateApprovalList(ctx context.Context, authUser *auth.User, claGroupModel *models.ClaGroup, companyModel *models.Company, claGroupID string, params *models.ApprovalList, projectSFID string) (*models.Signature, error)

	AddCLAManager(ctx context.Context, signatureID, claManagerID string) (*models.Signature, error)
	RemoveCLAManager(ctx context.Context, ignatureID, claManagerID string) (*models.Signature, error)

	GetClaGroupICLASignatures(ctx context.Context, claGroupID string, searchTerm *string, approved, signed *bool, pageSize int64, nextKey string, withExtraDetails bool) (*models.IclaSignatures, error)
	GetClaGroupCCLASignatures(ctx context.Context, claGroupID string, approved, signed *bool) (*models.Signatures, error)
	GetClaGroupCorporateContributors(ctx context.Context, claGroupID string, companyID *string, pageSize *int64, nextKey *string, searchTerm *string) (*models.CorporateContributorList, error)

	// createOrGetEmployeeModels(ctx context.Context, claGroupModel *models.ClaGroup, companyModel *models.Company, corporateSignatureModel *models.Signature) ([]*models.User, error)
	CreateOrUpdateEmployeeSignature(ctx context.Context, claGroupModel *models.ClaGroup, companyModel *models.Company, corporateSignatureModel *models.Signature) ([]*models.User, error)
	UpdateEnvelopeDetails(ctx context.Context, signatureID, envelopeID string, signURL *string) (*models.Signature, error)
	// handleGitHubStatusUpdate(ctx context.Context, employeeUserModel *models.User) error
	ProcessEmployeeSignature(ctx context.Context, companyModel *models.Company, claGroupModel *models.ClaGroup, user *models.User) (*bool, error)
	UserIsApproved(ctx context.Context, user *models.User, cclaSignature *models.Signature) (bool, error)
}

type service struct {
	repo                SignatureRepository
	companyService      company.IService
	usersService        users.Service
	eventsService       events.Service
	githubOrgValidation bool
	repositoryService   repositories.Service
	githubOrgService    github_organizations.ServiceInterface
	claGroupService     service2.Service
	gitLabApp           *gitlab_api.App
	claBaseAPIURL       string
	claLandingPage      string
	claLogoURL          string
}

// NewService creates a new signature service
func NewService(repo SignatureRepository, companyService company.IService, usersService users.Service, eventsService events.Service, githubOrgValidation bool, repositoryService repositories.Service, githubOrgService github_organizations.ServiceInterface, claGroupService service2.Service, gitLabApp *gitlab_api.App, CLABaseAPIURL, CLALandingPage, CLALogoURL string) SignatureService {
	return service{
		repo,
		companyService,
		usersService,
		eventsService,
		githubOrgValidation,
		repositoryService,
		githubOrgService,
		claGroupService,
		gitLabApp,
		CLABaseAPIURL,
		CLALandingPage,
		CLALogoURL,
	}
}

// GetSignature returns the signature associated with the specified signature ID
func (s service) GetSignature(ctx context.Context, signatureID string) (*models.Signature, error) {
	return s.repo.GetSignature(ctx, signatureID)
}

// SaveOrUpdateSignature saves or updates the specified signature
func (s service) SaveOrUpdateSignature(ctx context.Context, signature *ItemSignature) error {
	return s.repo.SaveOrUpdateSignature(ctx, signature)
}

// UpdateSignature updates the specified signature
func (s service) UpdateSignature(ctx context.Context, signatureID string, updates map[string]interface{}) error {
	return s.repo.UpdateSignature(ctx, signatureID, updates)
}

// GetIndividualSignature returns the signature associated with the specified CLA Group and User ID
func (s service) GetIndividualSignature(ctx context.Context, claGroupID, userID string, approved, signed *bool) (*models.Signature, error) {
	return s.repo.GetIndividualSignature(ctx, claGroupID, userID, approved, signed)
}

// GetIndividualSignatures returns the list of signatures associated with the specified CLA Group and User ID
func (s service) GetIndividualSignatures(ctx context.Context, claGroupID, userID string, approved, signed *bool) ([]*models.Signature, error) {
	return s.repo.GetIndividualSignatures(ctx, claGroupID, userID, approved, signed)
}

// GetCorporateSignature returns the signature associated with the specified CLA Group and Company ID
func (s service) GetCorporateSignature(ctx context.Context, claGroupID, companyID string, approved, signed *bool) (*models.Signature, error) {
	return s.repo.GetCorporateSignature(ctx, claGroupID, companyID, approved, signed)
}

// GetCorporateSignatures returns the list of signature associated with the specified CLA Group and Company ID
func (s service) GetCorporateSignatures(ctx context.Context, claGroupID, companyID string, approved, signed *bool) ([]*models.Signature, error) {
	return s.repo.GetCorporateSignatures(ctx, claGroupID, companyID, approved, signed)
}

// GetProjectSignatures returns the list of signatures associated with the specified project
func (s service) GetProjectSignatures(ctx context.Context, params signatures.GetProjectSignaturesParams) (*models.Signatures, error) {

	projectSignatures, err := s.repo.GetProjectSignatures(ctx, params)
	if err != nil {
		return nil, err
	}

	return projectSignatures, nil
}

// CreateOrUpdateEmployeeSignature creates or updates the specified  signature
func (s service) UpdateEnvelopeDetails(ctx context.Context, signatureID, envelopeID string, signURL *string) (*models.Signature, error) {
	return s.repo.UpdateEnvelopeDetails(ctx, signatureID, envelopeID, signURL)
}

// CreateProjectSummaryReport generates a project summary report based on the specified input
func (s service) CreateProjectSummaryReport(ctx context.Context, params signatures.CreateProjectSummaryReportParams) (*models.SignatureReport, error) {

	projectSignatures, err := s.repo.CreateProjectSummaryReport(ctx, params)
	if err != nil {
		return nil, err
	}

	return projectSignatures, nil
}

// GetProjectCompanySignature returns the signature associated with the specified project and company
func (s service) GetProjectCompanySignature(ctx context.Context, companyID, projectID string, approved, signed *bool, nextKey *string, pageSize *int64) (*models.Signature, error) {
	return s.repo.GetProjectCompanySignature(ctx, companyID, projectID, approved, signed, nextKey, pageSize)
}

// CreateIndividualSignature creates a new individual signature
func (s service) CreateSignature(ctx context.Context, signature *ItemSignature) error {
	return s.repo.CreateSignature(ctx, signature)
}

// GetProjectCompanySignatures returns the list of signatures associated with the specified project
func (s service) GetProjectCompanySignatures(ctx context.Context, params signatures.GetProjectCompanySignaturesParams) (*models.Signatures, error) {

	const defaultPageSize int64 = 10
	var pageSize = defaultPageSize
	if params.PageSize != nil {
		pageSize = *params.PageSize
	}

	signed := true
	approved := true

	projectSignatures, err := s.repo.GetProjectCompanySignatures(
		ctx, params.CompanyID, params.ProjectID, &signed, &approved, params.NextKey, params.SortOrder, &pageSize)
	if err != nil {
		return nil, err
	}

	return projectSignatures, nil
}

// GetProjectCompanyEmployeeSignatures returns the list of employee signatures associated with the specified project
func (s service) GetProjectCompanyEmployeeSignatures(ctx context.Context, params signatures.GetProjectCompanyEmployeeSignaturesParams, criteria *ApprovalCriteria) (*models.Signatures, error) {

	if params.PageSize == nil {
		params.PageSize = utils.Int64(10)
	}

	projectSignatures, err := s.repo.GetProjectCompanyEmployeeSignatures(ctx, params, criteria)
	if err != nil {
		return nil, err
	}

	return projectSignatures, nil
}

// GetCompanySignatures returns the list of signatures associated with the specified company
func (s service) GetCompanySignatures(ctx context.Context, params signatures.GetCompanySignaturesParams) (*models.Signatures, error) {

	const defaultPageSize int64 = 50
	var pageSize = defaultPageSize
	if params.PageSize != nil {
		pageSize = *params.PageSize
	}

	companySignatures, err := s.repo.GetCompanySignatures(ctx, params, pageSize, LoadACLDetails)
	if err != nil {
		return nil, err
	}

	return companySignatures, nil
}

// GetCompanyIDsWithSignedCorporateSignatures returns a list of company IDs that have signed a CLA agreement
func (s service) GetCompanyIDsWithSignedCorporateSignatures(ctx context.Context, claGroupID string) ([]SignatureCompanyID, error) {
	return s.repo.GetCompanyIDsWithSignedCorporateSignatures(ctx, claGroupID)
}

// GetCCLASignatures returns the list of CCLA signatures
func (s service) GetCCLASignatures(ctx context.Context, signed, approved *bool) ([]*ItemSignature, error) {
	return s.repo.GetCCLASignatures(ctx, signed, approved)
}

// GetUserSignatures returns the list of user signatures associated with the specified user
func (s service) GetUserSignatures(ctx context.Context, params signatures.GetUserSignaturesParams, projectID *string) (*models.Signatures, error) {

	const defaultPageSize int64 = 10
	var pageSize = defaultPageSize
	if params.PageSize != nil {
		pageSize = *params.PageSize
	}

	userSignatures, err := s.repo.GetUserSignatures(ctx, params, pageSize, projectID)
	if err != nil {
		return nil, err
	}

	return userSignatures, nil
}

// GetGithubOrganizationsFromApprovalList retrieves the organization from the approval list
func (s service) GetGithubOrganizationsFromApprovalList(ctx context.Context, signatureID string, githubAccessToken string) ([]models.GithubOrg, error) {

	if signatureID == "" {
		msg := "unable to get GitHub organizations approval list - signature ID is nil"
		log.Warn(msg)
		return nil, errors.New(msg)
	}

	orgIds, err := s.repo.GetGithubOrganizationsFromApprovalList(ctx, signatureID)
	if err != nil {
		log.Warnf("error loading github organization from approval list using signatureID: %s, error: %v",
			signatureID, err)
		return nil, err
	}

	if githubAccessToken != "" {
		log.Debugf("already authenticated with github - scanning for user's orgs...")

		selectedOrgs := make(map[string]struct{}, len(orgIds))
		for _, selectedOrg := range orgIds {
			selectedOrgs[*selectedOrg.ID] = struct{}{}
		}

		// Since we're logged into github, lets get the list of organization we can add.
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubAccessToken},
		)
		tc := oauth2.NewClient(utils.NewContext(), ts)
		client := githubpkg.NewClient(tc)

		opt := &githubpkg.ListOptions{
			PerPage: 100,
		}

		orgs, _, err := client.Organizations.List(utils.NewContext(), "", opt)
		if err != nil {
			return nil, err
		}

		for _, org := range orgs {
			_, ok := selectedOrgs[*org.Login]
			if ok {
				continue
			}

			orgIds = append(orgIds, models.GithubOrg{ID: org.Login})
		}
	}

	return orgIds, nil
}

// AddGithubOrganizationToApprovalList adds the GH organization to the approval list
func (s service) AddGithubOrganizationToApprovalList(ctx context.Context, signatureID string, approvalListParams models.GhOrgWhitelist, githubAccessToken string) ([]models.GithubOrg, error) {
	organizationID := approvalListParams.OrganizationID

	if signatureID == "" {
		msg := "unable to add GitHub organization from approval list - signature ID is nil"
		log.Warn(msg)
		return nil, errors.New(msg)
	}

	if organizationID == nil {
		msg := "unable to add GitHub organization from approval list - organization ID is nil"
		log.Warn(msg)
		return nil, errors.New(msg)
	}

	// GH_ORG_VALIDATION environment - set to false to test locally which will by-pass the GH auth checks and
	// allow functional tests (e.g. with curl or postmon) - default is enabled

	if s.githubOrgValidation {
		// Verify the authenticated github user has access to the github organization being added.
		if githubAccessToken == "" {
			msg := fmt.Sprintf("unable to add github organization, not logged in using "+
				"signatureID: %s, github organization id: %s",
				signatureID, *organizationID)
			log.Warn(msg)
			return nil, errors.New(msg)
		}

		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubAccessToken},
		)
		tc := oauth2.NewClient(utils.NewContext(), ts)
		client := githubpkg.NewClient(tc)

		opt := &githubpkg.ListOptions{
			PerPage: 100,
		}

		log.Debugf("querying for user's github organizations...")
		orgs, _, err := client.Organizations.List(utils.NewContext(), "", opt)
		if err != nil {
			return nil, err
		}

		found := false
		for _, org := range orgs {
			if *org.Login == *organizationID {
				found = true
				break
			}
		}

		if !found {
			msg := fmt.Sprintf("user is not authorized for github organization id: %s", *organizationID)
			log.Warnf("%s", msg)
			return nil, errors.New(msg)
		}
	}

	gitHubOrgApprovalList, err := s.repo.AddGithubOrganizationToApprovalList(ctx, signatureID, *organizationID)
	if err != nil {
		log.Warnf("issue adding github organization to approval list using signatureID: %s, gh org id: %s, error: %v",
			signatureID, *organizationID, err)
		return nil, err
	}

	return gitHubOrgApprovalList, nil
}

// DeleteGithubOrganizationFromApprovalList deletes the specified GH organization from the approval list
func (s service) DeleteGithubOrganizationFromApprovalList(ctx context.Context, signatureID string, approvalListParams models.GhOrgWhitelist, githubAccessToken string) ([]models.GithubOrg, error) {

	// Extract the payload values
	organizationID := approvalListParams.OrganizationID

	if signatureID == "" {
		msg := "unable to delete GitHub organization from approval list - signature ID is nil"
		log.Warn(msg)
		return nil, errors.New(msg)
	}

	if organizationID == nil {
		msg := "unable to delete GitHub organization from approval list - organization ID is nil"
		log.Warn(msg)
		return nil, errors.New(msg)
	}

	// GH_ORG_VALIDATION environment - set to false to test locally which will by-pass the GH auth checks and
	// allow functional tests (e.g. with curl or postmon) - default is enabled

	if s.githubOrgValidation {
		// Verify the authenticated github user has access to the github organization being added.
		if githubAccessToken == "" {
			msg := fmt.Sprintf("unable to delete github organization, not logged in using "+
				"signatureID: %s, github organization id: %s",
				signatureID, *organizationID)
			log.Warn(msg)
			return nil, errors.New(msg)
		}

		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubAccessToken},
		)
		tc := oauth2.NewClient(context.Background(), ts)
		client := githubpkg.NewClient(tc)

		opt := &githubpkg.ListOptions{
			PerPage: 100,
		}

		log.Debugf("querying for user's github organizations...")
		orgs, _, err := client.Organizations.List(context.Background(), "", opt)
		if err != nil {
			return nil, err
		}

		found := false
		for _, org := range orgs {
			if *org.Login == *organizationID {
				found = true
				break
			}
		}

		if !found {
			msg := fmt.Sprintf("user is not authorized for github organization id: %s", *organizationID)
			log.Warnf("%s", msg)
			return nil, errors.New(msg)
		}
	}

	gitHubOrgApprovalList, err := s.repo.DeleteGithubOrganizationFromApprovalList(ctx, signatureID, *organizationID)
	if err != nil {
		return nil, err
	}

	return gitHubOrgApprovalList, nil
}

// UpdateApprovalList service method which handles updating the various approval lists
func (s service) UpdateApprovalList(ctx context.Context, authUser *auth.User, claGroupModel *models.ClaGroup, companyModel *models.Company, claGroupID string, params *models.ApprovalList, projectSFID string) (*models.Signature, error) { // nolint gocyclo
	f := logrus.Fields{
		"functionName":      "v1.signatures.service.UpdateApprovalList",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"authUser.UserName": authUser.UserName,
		"authUser.Email":    authUser.Email,
		"claGroupID":        claGroupID,
		"claGroupName":      claGroupModel.ProjectName,
		"companyName":       companyModel.CompanyName,
		"companyID":         companyModel.CompanyID,
	}

	log.WithFields(f).Debugf("processing update approval list request")

	// Lookup the project corporate signature - should have one
	pageSize := int64(1)
	signed, approved := true, true
	corporateSigModel, sigErr := s.GetProjectCompanySignature(ctx, companyModel.CompanyID, claGroupID, &signed, &approved, nil, &pageSize)
	if sigErr != nil {
		msg := fmt.Sprintf("unable to locate project company signature by Company ID: %s, Project ID: %s, CLA Group ID: %s, error: %+v",
			companyModel.CompanyID, claGroupModel.ProjectID, claGroupID, sigErr)
		log.WithFields(f).WithError(sigErr).Warn(msg)
		return nil, NewBadRequestError(msg)
	}
	// If not found, return error
	if corporateSigModel == nil {
		msg := fmt.Sprintf("unable to locate signature for company ID: %s CLA Group ID: %s, type: ccla, signed: %t, approved: %t",
			companyModel.CompanyID, claGroupID, signed, approved)
		log.WithFields(f).Warn(msg)
		return nil, NewBadRequestError(msg)
	}

	// Ensure current user is in the Signature ACL
	claManagers := corporateSigModel.SignatureACL
	if !utils.CurrentUserInACL(authUser, claManagers) {
		msg := fmt.Sprintf("EasyCLA - 403 Forbidden - CLA Manager %s / %s is not authorized to approve request for company ID: %s / %s / %s, project ID: %s / %s / %s",
			authUser.UserName, authUser.Email,
			companyModel.CompanyName, companyModel.CompanyExternalID, companyModel.CompanyID,
			claGroupModel.ProjectName, claGroupModel.ProjectExternalID, claGroupModel.ProjectID)
		return nil, NewForbiddenError(msg)
	}

	// Lookup the user making the request - should be the CLA Manager
	userModel, userErr := s.usersService.GetUserByUserName(authUser.UserName, true)
	if userErr != nil {
		log.WithFields(f).WithError(userErr).Warnf("unable to lookup CLA Manager user by user name: %s", authUser.UserName)
		return nil, userErr
	}

	// This event is ONLY used when we need to invalidate the signature
	eventArgs := &events.LogEventArgs{
		EventType:     events.InvalidatedSignature, // reviewed and
		ProjectID:     claGroupModel.ProjectExternalID,
		ClaGroupModel: claGroupModel,
		CompanyID:     companyModel.CompanyID,
		CompanyModel:  companyModel,
		LfUsername:    userModel.LfUsername,
		UserID:        userModel.UserID,
		UserModel:     userModel,
		ProjectSFID:   projectSFID,
	}

	updatedCorporateSignature, err := s.repo.UpdateApprovalList(ctx, userModel, claGroupModel, companyModel.CompanyID, params, eventArgs)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem updating approval list for company ID: %s, project ID: %s, cla group ID: %s", companyModel.CompanyID, claGroupModel.ProjectID, claGroupID)
		return updatedCorporateSignature, err
	}

	// If auto create ECLA is enabled for this Corporate Agreement, then create an ECLA for each employee that was added to the approval list
	// we get the complete user list as output from the processing of the approval list
	var userModelList []*models.User
	if corporateSigModel.AutoCreateECLA {
		log.WithFields(f).Debug("auto-create ECLA option is enabled - processing auto-enable request for all items on the approval list...")
		userList, processErr := s.CreateOrUpdateEmployeeSignature(ctx, claGroupModel, companyModel, updatedCorporateSignature)
		if processErr != nil {
			log.WithFields(f).WithError(processErr).Warnf("problem processing auto-enable request for company ID: %s, project ID: %s, cla group ID: %s", companyModel.CompanyID, claGroupModel.ProjectID, claGroupID)
		}
		userModelList = userList
	} else {
		userList, processErr := s.createOrGetEmployeeModels(ctx, claGroupModel, companyModel, updatedCorporateSignature)
		if processErr != nil {
			log.WithFields(f).WithError(processErr).Warnf("problem processing user list for company ID: %s, project ID: %s, cla group ID: %s", companyModel.CompanyID, claGroupModel.ProjectID, claGroupID)
		}
		userModelList = userList
	}

	var wg sync.WaitGroup

	// Log Events that the CLA manager updated the approval lists - do it in a separate go routine
	log.WithFields(f).Debugf("creating event log entry...")
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.createEventLogEntries(ctx, companyModel, claGroupModel, userModel, params, projectSFID)
	}()

	// Send an email to each of the CLA Managers - do it in a separate go routine
	log.WithFields(f).Debugf("sending notification email to cla managers...")
	for _, claManager := range claManagers {
		wg.Add(1)
		go func(companyModel *models.Company, claGroupModel *models.ClaGroup, claManager models.User, params *models.ApprovalList) {
			defer wg.Done()
			claManagerEmail := getBestEmail(&claManager) // nolint
			s.sendApprovalListUpdateEmailToCLAManagers(companyModel, claGroupModel, claManager.Username, claManagerEmail, params)
		}(companyModel, claGroupModel, claManager, params)
	}

	// Send emails to contributors if email or GitHub/GitLab username was added or removed - do it in a separate go routine
	log.WithFields(f).Debugf("sending notification email to contributors...")
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.sendRequestAccessEmailToContributors(authUser, companyModel, claGroupModel, params)
	}()

	// For each employee that was added, update their GitHub PRs - they are now on the approval list (and if auto-acknowledge is enabled, they are also approved)
	// do this in a separate go routine
	for _, employeeUserModel := range userModelList {
		wg.Add(1)
		// Update the GitHub status for the employee in the background
		go func(ctx context.Context, employeeUserModel *models.User) {
			defer wg.Done()
			handleStatusErr := s.handleGitHubStatusUpdate(ctx, employeeUserModel)
			if handleStatusErr != nil {
				log.WithFields(f).WithError(handleStatusErr).Warnf("problem updating GitHub status for user: %+v", employeeUserModel)
			}
		}(utils.NewContextFromParent(ctx), employeeUserModel)
	}

	// Wait until all the go routines are done - if we don't wait, the behavior is undefined
	wg.Wait()

	return updatedCorporateSignature, nil
}

func (s service) createOrGetEmployeeModels(ctx context.Context, claGroupModel *models.ClaGroup, companyModel *models.Company, corporateSignatureModel *models.Signature) ([]*models.User, error) { // nolint gocyclomatic
	f := logrus.Fields{
		"functionName":   "v2.company.service.createOrGetEmployeeModels",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupModel.ProjectID,
		"claGroupName":   claGroupModel.ProjectName,
		"companyName":    companyModel.CompanyName,
		"companyID":      companyModel.CompanyID,
	}
	var employeeUserModel *models.User
	var userLookupErr error

	log.WithFields(f).Debugf("processing %d approval list entries", len(corporateSignatureModel.EmailApprovalList)+len(corporateSignatureModel.GithubUsernameApprovalList)+len(corporateSignatureModel.GitlabUsernameApprovalList))

	// Most of the following business logic is all the same - however, we need to handle the different types of approval lists entries and process them in the same way
	// We build a list of users to process - this is a list of simple user models that contain the email, GitHub username, and GitLab username - typically only one of the values in the model will be set
	//userList := make([]simpleUserInfoModel, len(corporateSignatureModel.EmailApprovalList)+len(corporateSignatureModel.GithubUsernameApprovalList)+len(corporateSignatureModel.GitlabUsernameApprovalList))
	var userList []simpleUserInfoModel
	for _, email := range corporateSignatureModel.EmailApprovalList {
		log.WithFields(f).Debugf("adding email: %s", email)
		userList = append(userList, simpleUserInfoModel{
			Email: email,
		})
	}
	for _, gitHubUserName := range corporateSignatureModel.GithubUsernameApprovalList {
		log.WithFields(f).Debugf("adding GitHub username: %s", gitHubUserName)
		userList = append(userList, simpleUserInfoModel{
			GitHubUserName: gitHubUserName,
		})
	}
	for _, gitLabUserName := range corporateSignatureModel.GitlabUsernameApprovalList {
		log.WithFields(f).Debugf("adding GitLab username: %s", gitLabUserName)
		userList = append(userList, simpleUserInfoModel{
			GitLabUserName: gitLabUserName,
		})
	}

	// employeeUserModels := make([]*models.User, len(corporateSignatureModel.EmailApprovalList)+len(corporateSignatureModel.GithubUsernameApprovalList)+len(corporateSignatureModel.GitlabUsernameApprovalList))
	var employeeUserModels []*models.User
	var responseErr error

	// For each item in the email approval list...
	for _, simpleUserInfoModelEntry := range userList {
		log.WithFields(f).Debugf("processing approval list entry: %+v", simpleUserInfoModelEntry)

		// Grab the current time
		_, currentTime := utils.CurrentTime()

		if simpleUserInfoModelEntry.Email != "" {
			employeeUserModel, userLookupErr = s.usersService.GetUserByEmail(simpleUserInfoModelEntry.Email)
			if userLookupErr == nil && employeeUserModel != nil {
				updatedEmployeeUserModel, updateErr := s.updateUserCompanyID(ctx, employeeUserModel, companyModel)
				if updatedEmployeeUserModel != nil && updateErr == nil {
					// Use the updated user model
					employeeUserModel = updatedEmployeeUserModel
				}
				if updatedEmployeeUserModel != nil && updateErr == nil {
					employeeUserModel = updatedEmployeeUserModel
				}
				employeeUserModels = append(employeeUserModels, employeeUserModel)

				continue
			}
		}

		if simpleUserInfoModelEntry.GitHubUserName != "" {
			employeeUserModel, userLookupErr = s.usersService.GetUserByGitHubUsername(simpleUserInfoModelEntry.GitHubUserName)
			if userLookupErr != nil {
				log.WithFields(f).WithError(userLookupErr).Warnf("problem looking up user by GitHub username: %s", simpleUserInfoModelEntry.GitHubUserName)
			} else if userLookupErr == nil && employeeUserModel != nil {
				updatedEmployeeUserModel, updateErr := s.updateUserCompanyID(ctx, employeeUserModel, companyModel)
				if updatedEmployeeUserModel != nil && updateErr == nil {
					// Use the updated user model
					employeeUserModel = updatedEmployeeUserModel
				}
				employeeUserModels = append(employeeUserModels, employeeUserModel)

				continue
			}

			// Additional lookup logic - use the GitHub API to grab additional user information
			if employeeUserModel == nil {
				// Need more information before we can create a new user record - attempt to locate the GitHub user
				// record by the GitHub username - we need the GitHub numeric ID value which was not provided by the UI/API call
				gitHubUserModel, gitHubErr := github.GetUserDetails(simpleUserInfoModelEntry.GitHubUserName)
				// Should get a model, no errors and have at least the ID
				if gitHubErr != nil || gitHubUserModel == nil || gitHubUserModel.ID == nil {
					log.WithFields(f).WithError(gitHubErr).Warnf("problem looking up GitHub user details for user: %s, model: %+v, error: %+v", simpleUserInfoModelEntry.GitHubUserName, gitHubUserModel, gitHubErr)
					responseErr = gitHubErr
					continue
				}

				if gitHubUserModel.ID != nil {
					simpleUserInfoModelEntry.GitHubUserID = strconv.FormatInt(*gitHubUserModel.ID, 10)
				}
				// User may not have a public email
				if gitHubUserModel.Email != nil {
					simpleUserInfoModelEntry.Email = *gitHubUserModel.Email
				}
			}
		}

		if simpleUserInfoModelEntry.GitLabUserName != "" {
			employeeUserModel, userLookupErr = s.usersService.GetUserByGitLabUsername(simpleUserInfoModelEntry.GitLabUserName)
			if userLookupErr != nil {
				log.WithFields(f).WithError(userLookupErr).Warnf("problem looking up user by GitLab username: %s", simpleUserInfoModelEntry.GitHubUserName)
			} else if userLookupErr == nil && employeeUserModel != nil {
				updatedEmployeeUserModel, updateErr := s.updateUserCompanyID(ctx, employeeUserModel, companyModel)
				if updatedEmployeeUserModel != nil && updateErr == nil {
					// Use the updated user model
					employeeUserModel = updatedEmployeeUserModel
				}
				employeeUserModels = append(employeeUserModels, employeeUserModel)

				continue
			}

			// Additional lookup logic - use the GitLab API to grab additional user information
			if employeeUserModel == nil {
				// Harold - this bit of logic needs finishing/review/testing
				// Take the CLA Group ID and look into the GitLab Orgs table and fine one of the GitLab Project/Org records
				// From one of records, we need to decode the access token and use that to create a GitLab client
				// This will give us the accessInfo we need to create the GitLab client
				accessInfo := "" // TODO: Need to get the access token from one of the exising GitLab repositories ?
				gitLabClient, gitLabClientErr := gitlab_api.NewGitlabOauthClient(accessInfo, s.gitLabApp)
				if gitLabClientErr != nil {
					log.WithFields(f).WithError(gitLabClientErr).Warnf("problem creating GitLab client for user: %s, error: %+v", simpleUserInfoModelEntry.GitLabUserName, gitLabClientErr)
					responseErr = gitLabClientErr
					continue
				}

				// Attempt to lookup the GitLab user record by the GitLab username - we need the GitLab numeric ID value which was not provided by the UI/API call
				gitLabUserModel, gitLabErr := gitlab_api.GetUserByName(ctx, gitLabClient, simpleUserInfoModelEntry.GitLabUserName)
				// Should get a model, no errors and have at least the ID
				if gitLabErr != nil || gitLabUserModel == nil || gitLabUserModel.ID == 0 {
					log.WithFields(f).WithError(gitLabErr).Warnf("problem looking up GitLab user details for user: %s, model: %+v, error: %+v", simpleUserInfoModelEntry.GitLabUserName, gitLabUserModel, gitLabErr)
					responseErr = gitLabErr
					continue
				}

				if gitLabUserModel.ID != 0 {
					simpleUserInfoModelEntry.GitHubUserID = strconv.FormatInt(int64(gitLabUserModel.ID), 10)
				}
				// User may not have a public email
				if gitLabUserModel.Email != "" {
					simpleUserInfoModelEntry.Email = gitLabUserModel.Email
				}
			}
		}

		// If we couldn't find the user, then create a user record
		if employeeUserModel == nil {
			log.WithFields(f).WithError(userLookupErr).Debugf("unable to lookup existing user by one of the values: %+v", simpleUserInfoModelEntry)
			var userCreateErr error

			// Create a new user record based on the email and company ID
			employeeUserModel, userCreateErr = s.createUserModel(
				simpleUserInfoModelEntry.GitHubUserName, simpleUserInfoModelEntry.GitHubUserID,
				simpleUserInfoModelEntry.GitLabUserName, simpleUserInfoModelEntry.GitLabUserID,
				simpleUserInfoModelEntry.Email, companyModel.CompanyID,
				fmt.Sprintf("auto generated ECLA user from CLA Manager adding user to the approval list with auto_create_ecla feature flag set to true on %+v.", currentTime))
			if userCreateErr != nil || employeeUserModel == nil {
				log.WithFields(f).WithError(userCreateErr).Warnf("unable to create a new user by one of the values: %+v", simpleUserInfoModelEntry)
				continue
			}

			employeeUserModels = append(employeeUserModels, employeeUserModel)
			log.WithFields(f).Debugf("created user using: %+v with company ID: %s", simpleUserInfoModelEntry, companyModel.CompanyID)
		}
	}

	return employeeUserModels, responseErr
}

func (s service) updateUserCompanyID(ctx context.Context, employeeUserModel *models.User, companyModel *models.Company) (*models.User, error) {
	f := logrus.Fields{
		"functionName":   "v2.company.service.updateUserCompanyID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyName":    companyModel.CompanyName,
		"companyID":      companyModel.CompanyID,
	}

	if employeeUserModel.CompanyID == "" || employeeUserModel.CompanyID != companyModel.CompanyID {
		_, currentTime := utils.CurrentTime()

		log.WithFields(f).Debugf("updating user record - set company ID = %s - previous value was: %s", companyModel.CompanyID, employeeUserModel.CompanyID)
		employeeUserModel.CompanyID = companyModel.CompanyID
		userUpdateErr := s.usersService.UpdateUserCompanyID(
			employeeUserModel.UserID,
			companyModel.CompanyID,
			fmt.Sprintf("auto assign companyID from CLA Manager adding user to the company approval list with auto_create_ecla feature flag set to true on %+v.", currentTime))
		if userUpdateErr != nil {
			log.WithFields(f).WithError(userUpdateErr).Warnf("problem updating user record with company ID: %s", companyModel.CompanyID)
			return nil, userUpdateErr
		}

		log.WithFields(f).Debugf("updated user record with company ID: %s", companyModel.CompanyID)
		// Reload and return the updated user model
		return s.usersService.GetUser(employeeUserModel.UserID)
	}

	return employeeUserModel, nil
}

// CreateOrUpdateEmployeeSignature creates or updates the employee signature for the given company
func (s service) CreateOrUpdateEmployeeSignature(ctx context.Context, claGroupModel *models.ClaGroup, companyModel *models.Company, corporateSignatureModel *models.Signature) ([]*models.User, error) {
	f := logrus.Fields{
		"functionName":   "v2.company.service.CreateOrUpdateEmployeeSignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupModel.ProjectID,
		"claGroupName":   claGroupModel.ProjectName,
		"companyName":    companyModel.CompanyName,
		"companyID":      companyModel.CompanyID,
	}

	// Most of the following business logic is all the same - however, we need to handle the different types of approval lists entries and process them in the same way
	// We build a list of users to process - this is a list of simple user models that contain the email, GitHub username, and GitLab username - typically only one of the values in the model will be set
	userList, userErr := s.createOrGetEmployeeModels(ctx, claGroupModel, companyModel, corporateSignatureModel)
	if userErr != nil {
		log.WithFields(f).WithError(userErr).Warnf("problem creating or loading user records from the approval list")
	}

	responseErr := s.processEmployeeSignatures(ctx, companyModel, claGroupModel, userList)

	if responseErr != nil {
		log.WithFields(f).WithError(responseErr).Warnf("problem processing employee signatures")
	}

	return userList, responseErr
}

func (s service) processEmployeeSignatures(ctx context.Context, companyModel *models.Company, claGroupModel *models.ClaGroup, userList []*models.User) error {
	f := logrus.Fields{
		"functionName":   "v2.company.service.processEmployeeSignatures",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupModel.ProjectID,
		"companyName":    companyModel.CompanyName,
		"companyID":      companyModel.CompanyID,
	}

	var responseErr error
	var wg sync.WaitGroup
	resultChan := make(chan *EmployeeModel)
	errChan := make(chan error)

	// For each item in the email approval list...
	for _, employeeUserModel := range userList {
		wg.Add(1)
		go s.repo.GetProjectCompanyEmployeeSignature(ctx, companyModel, claGroupModel, employeeUserModel, &wg, resultChan, errChan)
	}

	// Wait for all the go routines to complete
	go func() {
		wg.Wait()
		close(resultChan)
		close(errChan)
	}()

	for employeeModel := range resultChan {
		if employeeModel != nil {
			employeeSignatureModel := employeeModel.Signature
			employeeUserModel := employeeModel.User
			log.WithFields(f).Debugf("processing employee signature record for user: %+s", employeeUserModel.UserID)
			if employeeSignatureModel != nil {
				if !employeeSignatureModel.SignatureApproved || !employeeSignatureModel.SignatureSigned {
					// If record exists, this will update the record
					log.WithFields(f).Debugf("updating employee signature record for: %+v", employeeSignatureModel)
					updateErr := s.repo.ValidateProjectRecord(ctx, employeeSignatureModel.SignatureID, "signed and approved employee acknowledgement since auto_create_ecla feature flag set to true")
					if updateErr != nil {
						log.WithFields(f).WithError(updateErr).Warnf("problem updating employee signature record for: %+v", employeeSignatureModel)
						responseErr = updateErr
					}
				} else {
					log.WithFields(f).Debugf("employee signature record already exists for: %+v", employeeUserModel)
				}
			} else {
				// Ok, auto-create the employee acknowledgement record
				log.WithFields(f).Debugf("creating employee signature record for user: %+s", employeeUserModel.UserID)
				createErr := s.repo.CreateProjectCompanyEmployeeSignature(ctx, companyModel, claGroupModel, employeeUserModel)
				if createErr != nil {
					log.WithFields(f).WithError(createErr).Warnf("unable to create project company employee signature record for: %+v", employeeUserModel)
					responseErr = createErr
				}
			}
		}
	}

	// Check for any errors
	for err := range errChan {
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("problem looking up employee signature record ")
			responseErr = err
		}
	}

	log.WithFields(f).Debugf("completed processing employee signatures")

	return responseErr
}

// InvalidateProjectRecords disassociates project signatures
func (s service) InvalidateProjectRecords(ctx context.Context, projectID, note string) (int, error) {
	f := logrus.Fields{
		"functionName": "v1.signatures.service.InvalidateProjectRecords",
		"projectID":    projectID,
	}

	result, err := s.repo.ProjectSignatures(ctx, projectID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Unable to get signatures for project: %s", projectID)
		return 0, err
	}

	if len(result.Signatures) > 0 {
		var wg sync.WaitGroup
		wg.Add(len(result.Signatures))
		log.WithFields(f).Debugf("Invalidating %d signatures for project: %s ",
			len(result.Signatures), projectID)
		for _, signature := range result.Signatures {
			// Do this in parallel, as we could have a lot to invalidate
			go func(sigID, projectID string) {
				defer wg.Done()
				updateErr := s.repo.InvalidateProjectRecord(ctx, sigID, note)
				if updateErr != nil {
					log.WithFields(f).WithError(updateErr).Warnf("Unable to update signature: %s with project ID: %s, error: %v", sigID, projectID, updateErr)
				}
			}(signature.SignatureID, projectID)
		}

		// Wait until all the workers are done
		wg.Wait()
	}

	return len(result.Signatures), nil
}

// AddCLAManager adds the specified manager to the signature ACL list
func (s service) AddCLAManager(ctx context.Context, signatureID, claManagerID string) (*models.Signature, error) {
	return s.repo.AddCLAManager(ctx, signatureID, claManagerID)
}

// RemoveCLAManager removes the specified manager from the signature ACL list
func (s service) RemoveCLAManager(ctx context.Context, signatureID, claManagerID string) (*models.Signature, error) {
	return s.repo.RemoveCLAManager(ctx, signatureID, claManagerID)
}

// appendList is a helper function to generate the email content of the Approval List changes
func appendList(approvalList []string, message string) string {
	approvalListSummary := ""

	if len(approvalList) > 0 {
		for _, value := range approvalList {
			approvalListSummary += fmt.Sprintf("<li>%s %s</li>", message, value)
		}
	}

	return approvalListSummary
}

// buildApprovalListSummary is a helper function to generate the email content of the Approval List changes
func buildApprovalListSummary(approvalListChanges *models.ApprovalList) string {
	approvalListSummary := "<ul>"
	approvalListSummary += appendList(approvalListChanges.AddEmailApprovalList, "Added Email:")
	approvalListSummary += appendList(approvalListChanges.RemoveEmailApprovalList, "Removed Email:")
	approvalListSummary += appendList(approvalListChanges.AddDomainApprovalList, "Added Domain:")
	approvalListSummary += appendList(approvalListChanges.RemoveDomainApprovalList, "Removed Domain:")
	approvalListSummary += appendList(approvalListChanges.AddGithubUsernameApprovalList, "Added GitHub User:")
	approvalListSummary += appendList(approvalListChanges.RemoveGithubUsernameApprovalList, "Removed GitHub User:")
	approvalListSummary += appendList(approvalListChanges.AddGithubOrgApprovalList, "Added GitHub Organization:")
	approvalListSummary += appendList(approvalListChanges.RemoveGithubOrgApprovalList, "Removed GitHub Organization:")
	approvalListSummary += appendList(approvalListChanges.AddGitlabUsernameApprovalList, "Added Gitlab User:")
	approvalListSummary += appendList(approvalListChanges.RemoveGitlabUsernameApprovalList, "Removed Gitlab User:")
	approvalListSummary += appendList(approvalListChanges.AddGitlabOrgApprovalList, "Added Gitlab Organization:")
	approvalListSummary += appendList(approvalListChanges.RemoveGitlabOrgApprovalList, "Removed Gitlab Organization:")
	approvalListSummary += "</ul>"
	return approvalListSummary
}

func (s service) GetClaGroupICLASignatures(ctx context.Context, claGroupID string, searchTerm *string, approved, signed *bool, pageSize int64, nextKey string, withExtraDetails bool) (*models.IclaSignatures, error) {
	return s.repo.GetClaGroupICLASignatures(ctx, claGroupID, searchTerm, approved, signed, pageSize, nextKey, withExtraDetails)
}

func (s service) GetClaGroupCCLASignatures(ctx context.Context, claGroupID string, approved, signed *bool) (*models.Signatures, error) {
	pageSize := utils.Int64(1000)
	return s.repo.GetProjectSignatures(ctx, signatures.GetProjectSignaturesParams{
		ClaType:   aws.String(utils.ClaTypeCCLA),
		ProjectID: claGroupID,
		PageSize:  pageSize,
		Approved:  approved,
		Signed:    signed,
	})
}

func (s service) GetClaGroupCorporateContributors(ctx context.Context, claGroupID string, companyID *string, pageSize *int64, nextKey *string, searchTerm *string) (*models.CorporateContributorList, error) {
	return s.repo.GetClaGroupCorporateContributors(ctx, claGroupID, companyID, pageSize, nextKey, searchTerm)
}

// updateChangeRequest is a helper function that updates PR - typically after the auto ecla update
func (s service) updateChangeRequest(ctx context.Context, ghOrg *models.GithubOrganization, repositoryID, pullRequestID int64, projectID string) error {
	f := logrus.Fields{
		"functionName":  "v1.signatures.service.updateChangeRequest",
		"repositoryID":  repositoryID,
		"pullRequestID": pullRequestID,
		"projectID":     projectID,
	}

	githubRepository, ghErr := github.GetGitHubRepository(ctx, ghOrg.OrganizationInstallationID, repositoryID)
	if ghErr != nil {
		log.WithFields(f).WithError(ghErr).Warn("unable to get github repository")
		return ghErr
	}
	if githubRepository == nil || githubRepository.Owner == nil {
		msg := "unable to get github repository - repository response is nil or owner is nil"
		log.WithFields(f).Warn(msg)
		return errors.New(msg)
	}
	// log.WithFields(f).Debugf("githubRepository: %+v", githubRepository)
	if githubRepository.Name == nil || githubRepository.Owner.Login == nil {
		msg := fmt.Sprintf("unable to get github repository - missing repository name or owner name for repository ID: %d", repositoryID)
		log.WithFields(f).Warn(msg)
		return errors.New(msg)
	}

	gitHubOrgName := utils.StringValue(githubRepository.Owner.Login)
	gitHubRepoName := utils.StringValue(githubRepository.Name)

	// Fetch committers
	log.WithFields(f).Debugf("fetching commit authors for PR: %d using repository owner: %s, repo: %s", pullRequestID, gitHubOrgName, gitHubRepoName)
	authors, latestSHA, authorsErr := github.GetPullRequestCommitAuthors(ctx, ghOrg.OrganizationInstallationID, int(pullRequestID), gitHubOrgName, gitHubRepoName)
	if authorsErr != nil {
		log.WithFields(f).WithError(authorsErr).Warnf("unable to get commit authors for %s/%s for PR: %d", gitHubOrgName, gitHubRepoName, pullRequestID)
		return authorsErr
	}
	log.WithFields(f).Debugf("found %d commit authors for %s/%s for PR: %d", len(authors), gitHubOrgName, gitHubRepoName, pullRequestID)

	signed := make([]*github.UserCommitSummary, 0)
	unsigned := make([]*github.UserCommitSummary, 0)

	// triage signed and unsigned users
	log.WithFields(f).Debugf("triaging %d commit authors for PR: %d using repository %s/%s",
		len(authors), pullRequestID, gitHubOrgName, gitHubRepoName)
	for _, userSummary := range authors {

		if !userSummary.IsValid() {
			log.WithFields(f).Debugf("invalid user summary: %+v", *userSummary)
			unsigned = append(unsigned, userSummary)
			continue
		}

		commitAuthorID := userSummary.GetCommitAuthorID()
		commitAuthorUsername := userSummary.GetCommitAuthorUsername()
		commitAuthorEmail := userSummary.GetCommitAuthorEmail()

		log.WithFields(f).Debugf("checking user - sha: %s, user ID: %s, username: %s, email: %s",
			userSummary.SHA, commitAuthorID, commitAuthorUsername, commitAuthorEmail)

		var user *models.User
		var userErr error

		if commitAuthorID != "" {
			log.WithFields(f).Debugf("looking up user by ID: %s", commitAuthorID)
			user, userErr = s.usersService.GetUserByGitHubID(commitAuthorID)
			if userErr != nil {
				log.WithFields(f).WithError(userErr).Warnf("unable to get user by github id: %s", commitAuthorID)
			}
			if user != nil {
				log.WithFields(f).Debugf("found user by ID: %s", commitAuthorID)
			}
		}
		if user == nil && commitAuthorUsername != "" {
			log.WithFields(f).Debugf("looking up user by username: %s", commitAuthorUsername)
			user, userErr = s.usersService.GetUserByGitHubUsername(commitAuthorUsername)
			if userErr != nil {
				log.WithFields(f).WithError(userErr).Warnf("unable to get user by github username: %s", commitAuthorUsername)
			}
			if user != nil {
				log.WithFields(f).Debugf("found user by username: %s", commitAuthorUsername)
			}
		}
		if user == nil && commitAuthorEmail != "" {
			log.WithFields(f).Debugf("looking up user by email: %s", commitAuthorEmail)
			user, userErr = s.usersService.GetUserByEmail(commitAuthorEmail)
			if userErr != nil {
				log.WithFields(f).WithError(userErr).Warnf("unable to get user by user email: %s", commitAuthorEmail)
			}
			if user != nil {
				log.WithFields(f).Debugf("found user by email: %s", commitAuthorEmail)
			}
		}

		if user == nil {
			log.WithFields(f).Debugf("unable to find user for commit author - sha: %s, user ID: %s, username: %s, email: %s",
				userSummary.SHA, commitAuthorID, commitAuthorUsername, commitAuthorEmail)
			unsigned = append(unsigned, userSummary)
			continue
		}

		log.WithFields(f).Debugf("checking to see if user has signed an ICLA or ECLA for project: %s", projectID)
		userSigned, companyAffiliation, signedErr := s.HasUserSigned(ctx, user, projectID)
		if signedErr != nil {
			log.WithFields(f).WithError(signedErr).Warnf("has user signed error - user: %+v, project: %s", user, projectID)
			unsigned = append(unsigned, userSummary)
			continue
		}

		if companyAffiliation != nil {
			userSummary.Affiliated = *companyAffiliation
		}

		if userSigned != nil {
			userSummary.Authorized = *userSigned
			if userSummary.Authorized {
				signed = append(signed, userSummary)
			} else {
				unsigned = append(unsigned, userSummary)
			}
		}
	}

	log.WithFields(f).Debugf("commit authors status => signed: %+v and missing: %+v", signed, unsigned)
	var whitelisted []*github.UserCommitSummary
	unsigned, whitelisted = github.SkipWhitelistedBots(s.eventsService, ghOrg, gitHubRepoName, projectID, unsigned)
	if len(whitelisted) > 0 {
		log.WithFields(f).Debugf("adding %d whitelisted actors to signed list", len(whitelisted))
		signed = append(signed, whitelisted...)
	}
	log.WithFields(f).Debugf("commit authors status after whitelisting bots => signed: %+v, missing: %+v, whitelisted: %+v", signed, unsigned, whitelisted)

	// update pull request
	updateErr := github.UpdatePullRequest(ctx, ghOrg.OrganizationInstallationID, int(pullRequestID), gitHubOrgName, gitHubRepoName, githubRepository.ID, *latestSHA, signed, unsigned, s.claBaseAPIURL, s.claLandingPage, s.claLogoURL)
	if updateErr != nil {
		log.WithFields(f).Debugf("unable to update PR: %d", pullRequestID)
		return updateErr
	}

	return nil
}

// hasUserSigned checks to see if the user has signed an ICLA or ECLA for the project, returns:
// false, false, nil if user is not authorized for ICLA or ECLA
// false, false, some error if user is not authorized for ICLA or ECLA - we has some problem looking up stuff
// true, false, nil if user has an ICLA (authorized, but not company affiliation, no error)
// true, true, nil if user has an ECLA (authorized, with company affiliation, no error)
func (s service) HasUserSigned(ctx context.Context, user *models.User, projectID string) (*bool, *bool, error) {
	f := logrus.Fields{
		"functionName": "v1.signatures.service.HasUserSigned",
		"projectID":    projectID,
		"user":         user,
	}
	var hasSigned bool
	var companyAffiliation bool

	approved := true
	signed := true

	// Check for ICLA
	log.WithFields(f).Debugf("checking to see if user has signed an ICLA")
	signature, sigErr := s.GetIndividualSignature(ctx, projectID, user.UserID, &approved, &signed)
	if sigErr != nil {
		log.WithFields(f).WithError(sigErr).Warnf("problem checking for ICLA signature for user: %s", user.UserID)
		return &hasSigned, &companyAffiliation, sigErr
	}
	if signature != nil {
		hasSigned = true
		log.WithFields(f).Debugf("ICLA signature check passed for user: %+v on project : %s", user, projectID)
		return &hasSigned, &companyAffiliation, nil // ICLA passes, no company affiliation
	} else {
		log.WithFields(f).Debugf("ICLA signature check failed for user: %+v on project: %s - ICLA not signed", user, projectID)
	}

	// Check for Employee Acknowledgment ECLA
	companyID := user.CompanyID
	log.WithFields(f).Debugf("checking to see if user has signed a ECLA for company: %s", companyID)

	if companyID != "" {
		companyAffiliation = true

		// Get employee signature
		log.WithFields(f).Debugf("ECLA signature check - user has a company: %s - looking for user's employee acknowledgement...", companyID)

		// Load the company - make sure it is valid
		companyModel, compModelErr := s.companyService.GetCompany(ctx, companyID)
		if compModelErr != nil {
			log.WithFields(f).WithError(compModelErr).Warnf("problem looking up company: %s", companyID)
			return &hasSigned, &companyAffiliation, compModelErr
		}

		// Load the CLA Group - make sure it is valid
		claGroupModel, claGroupModelErr := s.claGroupService.GetCLAGroupByID(ctx, projectID)
		if claGroupModelErr != nil {
			log.WithFields(f).WithError(claGroupModelErr).Warnf("problem looking up project: %s", projectID)
			return &hasSigned, &companyAffiliation, claGroupModelErr
		}

		employeeSigned, err := s.ProcessEmployeeSignature(ctx, companyModel, claGroupModel, user)

		if err != nil {
			log.WithFields(f).WithError(err).Warnf("problem looking up employee signature for company: %s", companyID)
			return &hasSigned, &companyAffiliation, err
		}
		if employeeSigned != nil {
			hasSigned = *employeeSigned
		}

	} else {
		log.WithFields(f).Debugf("ECLA signature check - user does not have a company ID assigned - skipping...")
	}

	return &hasSigned, &companyAffiliation, nil
}

func (s service) ProcessEmployeeSignature(ctx context.Context, companyModel *models.Company, claGroupModel *models.ClaGroup, user *models.User) (*bool, error) {
	f := logrus.Fields{
		"functionName":   "v2.signatures.service.ProcessEmployeeSignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyModel.CompanyID,
		"projectID":      claGroupModel.ProjectID,
		"userID":         user.UserID,
	}
	var wg sync.WaitGroup
	resultChannel := make(chan *EmployeeModel)
	errorChannel := make(chan error)
	hasSigned := false
	projectID := claGroupModel.ProjectID
	companyID := companyModel.CompanyID
	approved := true
	signed := true

	wg.Add(1)
	go s.repo.GetProjectCompanyEmployeeSignature(ctx, companyModel, claGroupModel, user, &wg, resultChannel, errorChannel)

	go func() {
		wg.Wait()
		close(resultChannel)
		close(errorChannel)
	}()

	for result := range resultChannel {
		if result != nil {
			employeeSignature := result.Signature
			if employeeSignature != nil {
				// log.WithFields(f).Debugf("ECLA Signature check - located employee acknowledgement - signature id: %s", employeeSignature.SignatureID)
				log.WithFields(f).Debugf("ecla signature check -  :%+v", employeeSignature)

				// Get corporate ccla signature of company to access the approval list
				cclaSignature, cclaErr := s.GetCorporateSignature(ctx, projectID, companyID, &approved, &signed)
				if cclaErr != nil {
					log.WithFields(f).WithError(cclaErr).Warnf("problem looking up ECLA signature for company: %s, project: %s", companyID, projectID)
					return &hasSigned, cclaErr
				}

				if cclaSignature != nil {
					log.WithFields(f).Debug("found ccla signature")
					userApproved, approvedErr := s.UserIsApproved(ctx, user, cclaSignature)
					if approvedErr != nil {
						log.WithFields(f).WithError(approvedErr).Warnf("problem determining if user: %s is approved for project: %s", user.UserID, projectID)
						return &hasSigned, approvedErr
					}
					log.WithFields(f).Debugf("ECLA Signature check - user approved: %t for projectID: %s for company: %s", userApproved, projectID, user.CompanyID)

					if userApproved {
						log.WithFields(f).Debugf("user: %s is in the approval list for signature: %s", user.UserID, employeeSignature.SignatureID)
						hasSigned = true
					} else {
						log.WithFields(f).Debugf("user: %s is not in the approval list for signature: %s", user.UserID, employeeSignature.SignatureID)
					}
				}
			} else {
				log.WithFields(f).Debugf("ECLA Signature check - unable to locate employee acknowledgement for user: %s, company: %s, project: %s", user.UserID, companyID, projectID)
			}
		}
	}

	for empSigErr := range errorChannel {
		log.WithFields(f).WithError(empSigErr).Warnf("problem looking up employee signature for user: %s, company: %s, project: %s", user.UserID, companyID, projectID)
		return &hasSigned, empSigErr
	}

	return &hasSigned, nil

}

func (s service) UserIsApproved(ctx context.Context, user *models.User, cclaSignature *models.Signature) (bool, error) {
	// add lf email to emails
	f := logrus.Fields{
		"functionName": "v1.signatures.service.UserIsApproved",
	}

	emails := user.Emails

	if user.LfEmail != "" {
		log.WithFields(f).Debugf("adding lf email: %s to emails", user.LfEmail)
		emails = append(emails, string(user.LfEmail))
		// remove duplicates
		log.WithFields(f).Debug("removing duplicates")
		emails = utils.RemoveDuplicates(emails)
	}

	// check GitHub username approval list
	log.WithFields(f).Debug("checking if user is in the approval list")
	gitHubUsernameApprovalList := cclaSignature.GithubUsernameApprovalList
	if len(gitHubUsernameApprovalList) > 0 {
		for _, gitHubUsername := range gitHubUsernameApprovalList {
			if strings.EqualFold(gitHubUsername, strings.TrimSpace(user.GithubUsername)) {
				return true, nil
			}
		}
	} else {
		log.WithFields(f).Debugf("no matching github username found in ccla: %s", cclaSignature.SignatureID)
	}

	// check GitLab username approval list
	gitLabUsernameApprovalList := cclaSignature.GitlabUsernameApprovalList
	if len(gitLabUsernameApprovalList) > 0 {
		for _, gitLabUsername := range gitLabUsernameApprovalList {
			if strings.EqualFold(gitLabUsername, strings.TrimSpace(user.GitlabUsername)) {
				return true, nil
			}
		}
	} else {
		log.WithFields(f).Debugf("no matching gitlab username found in ccla: %s", cclaSignature.SignatureID)
	}

	// check email email approval list
	emailApprovalList := cclaSignature.EmailApprovalList
	log.WithFields(f).Debugf("checking if user is in the email approval list: %+v with emails :%v", emailApprovalList, emails)
	if len(emailApprovalList) > 0 {
		for _, email := range emails {
			log.WithFields(f).Debugf("checking email: %s", email)
			// case insensitive search
			for _, emailApproval := range emailApprovalList {
				if strings.EqualFold(email, emailApproval) {
					return true, nil
				}
			}
		}
	} else {
		log.WithFields(f).Debugf("no matching email found in ccla: %s", cclaSignature.SignatureID)
	}

	// check domain email approval list
	domainApprovalList := cclaSignature.DomainApprovalList
	if len(domainApprovalList) > 0 {
		matched, err := s.processPattern(emails, domainApprovalList)
		if err != nil {
			return false, err
		}
		if matched != nil && *matched {
			return true, nil
		}
	}

	// check github org email ApprovalList
	if user.GithubUsername != "" {
		githubOrgApprovalList := cclaSignature.GithubOrgApprovalList
		if len(githubOrgApprovalList) > 0 {
			log.WithFields(f).Debugf("determining if github user :%s is associated with ant of the github orgs : %+v", user.GithubUsername, githubOrgApprovalList)
		}

		for _, org := range githubOrgApprovalList {
			membership, err := github.GetMembership(ctx, user.GithubUsername, org)
			if err != nil {
				break
			}
			if membership != nil {
				log.WithFields(f).Debugf("found matching github organization: %s for user: %s", org, user.GithubUsername)
				return true, nil
			} else {
				log.WithFields(f).Debugf("user: %s is not in the organization: %s", user.GithubUsername, org)
			}
		}
	}

	return false, nil
}

func (s service) processPattern(emails []string, patterns []string) (*bool, error) {
	matched := false

	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "*.") {
			pattern = strings.Replace(pattern, "*.", ".*", -1)
		} else if strings.HasPrefix(pattern, "*") {
			pattern = strings.Replace(pattern, "*", ".*", -1)
		} else if strings.HasPrefix(pattern, ".") {
			pattern = strings.Replace(pattern, ".", ".*", -1)
		}

		preProcessedPattern := fmt.Sprintf("^.*@%s$", pattern)
		compiled, err := regexp.Compile(preProcessedPattern)
		if err != nil {
			return nil, err
		}

		for _, email := range emails {
			if compiled.MatchString(email) {
				matched = true
				break
			}
		}
	}

	return &matched, nil
}

func (s service) handleGitHubStatusUpdate(ctx context.Context, employeeUserModel *models.User) error {
	if employeeUserModel == nil {
		return fmt.Errorf("employee user model is nil")
	}

	f := logrus.Fields{
		"functionName":   "v1.signatures.service.handleGitHubStatusUpdate",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"userID":         employeeUserModel.UserID,
		"gitHubUsername": employeeUserModel.GithubUsername,
		"gitHubID":       employeeUserModel.GithubID,
		"userEmail":      employeeUserModel.LfEmail.String(),
	}

	log.WithFields(f).Debugf("processing GitHub status check request for user: %s", employeeUserModel.GitlabUsername)
	signatureMetadata, activeSigErr := s.repo.GetActivePullRequestMetadata(ctx, employeeUserModel.GithubUsername, employeeUserModel.LfEmail.String())
	if activeSigErr != nil {
		log.WithFields(f).WithError(activeSigErr).Warnf("unable to get active pull request metadata for user: %+v - unable to update GitHub status", employeeUserModel)
		return activeSigErr
	}
	if signatureMetadata == nil {
		log.WithFields(f).Debugf("unable to get active pull requst metadata for user: %+v - unable to update GitHub status", employeeUserModel)
		return nil
	}

	// Fetch easycla repository
	claRepository, repoErr := s.repositoryService.GetRepositoryByExternalID(ctx, signatureMetadata.RepositoryID)
	if repoErr != nil {
		log.WithFields(f).WithError(repoErr).Warnf("unable to fetch repository by ID: %s - unable to update GitHub status", signatureMetadata.RepositoryID)
		return repoErr
	}

	if !claRepository.Enabled {
		log.WithFields(f).Debugf("repository: %s associated with PR: %s is NOT enabled - unable to update GitHub status", claRepository.RepositoryURL, signatureMetadata.PullRequestID)
		return nil
	}

	// fetch GitHub org details
	githubOrg, githubOrgErr := s.githubOrgService.GetGitHubOrganizationByName(ctx, claRepository.RepositoryOrganizationName)
	if githubOrgErr != nil {
		log.WithFields(f).WithError(githubOrgErr).Warnf("unable to lookup GitHub organization by name: %s - unable to update GitHub status", claRepository.RepositoryOrganizationName)
		return githubOrgErr
	}

	repositoryID, idErr := strconv.Atoi(signatureMetadata.RepositoryID)
	if idErr != nil {
		log.WithFields(f).WithError(idErr).Warnf("unable to convert repository ID: %s to integer - unable to update GitHub status", signatureMetadata.RepositoryID)
		return idErr
	}

	pullRequestID, idErr := strconv.Atoi(signatureMetadata.PullRequestID)
	if idErr != nil {
		log.WithFields(f).WithError(idErr).Warnf("unable to convert pull request ID: %s to integer - unable to update GitHub status", signatureMetadata.RepositoryID)
		return idErr
	}

	// Update change request
	log.WithFields(f).Debugf("updating change request for repository: %d, pull request: %d", repositoryID, pullRequestID)
	updateErr := s.updateChangeRequest(ctx, githubOrg, int64(repositoryID), int64(pullRequestID), signatureMetadata.CLAGroupID)
	if updateErr != nil {
		log.WithFields(f).WithError(updateErr).Warnf("unable to update pull request: %d", pullRequestID)
		return updateErr
	}

	return nil
}
