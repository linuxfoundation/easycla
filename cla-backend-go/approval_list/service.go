// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package approval_list

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/communitybridge/easycla/cla-backend-go/emails"

	"github.com/communitybridge/easycla/cla-backend-go/signatures"
	"github.com/communitybridge/easycla/cla-backend-go/utils"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/project"
	"github.com/communitybridge/easycla/cla-backend-go/user"
	"github.com/communitybridge/easycla/cla-backend-go/users"

	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
)

// errors
var (
	ErrCclaApprovalRequestAlreadyExists = errors.New("approval request already exist")
)

// constants
const (
	DontLoadRepoDetails = true
)

// IService interface defines the service methods/functions
type IService interface {
	AddCclaWhitelistRequest(ctx context.Context, companyID string, claGroupID string, args models.CclaWhitelistRequestInput) (string, error)
	ApproveCclaWhitelistRequest(ctx context.Context, claUser *user.CLAUser, ClacompanyID, claGroupID, requestID string) error
	RejectCclaWhitelistRequest(ctx context.Context, companyID, claGroupID, requestID string) error
	ListCclaWhitelistRequest(companyID string, claGroupID, status *string) (*models.CclaWhitelistRequestList, error)
	ListCclaWhitelistRequestByCompanyProjectUser(companyID string, claGroupID, status, userID *string) (*models.CclaWhitelistRequestList, error)
}

type service struct {
	repo                       IRepository
	projectService             project.Service
	userRepo                   users.UserRepository
	companyRepo                company.IRepository
	projectRepo                project.ProjectRepository
	signatureRepo              signatures.SignatureRepository
	projectsCLAGroupRepository projects_cla_groups.Repository
	corpConsoleURL             string
	httpClient                 *http.Client
}

// NewService creates a new whitelist service
func NewService(repo IRepository, projectsCLAGroupRepository projects_cla_groups.Repository, projService project.Service, userRepo users.UserRepository, companyRepo company.IRepository, projectRepo project.ProjectRepository, signatureRepo signatures.SignatureRepository, corpConsoleURL string, httpClient *http.Client) IService {
	return service{
		repo:                       repo,
		projectsCLAGroupRepository: projectsCLAGroupRepository,
		projectService:             projService,
		userRepo:                   userRepo,
		companyRepo:                companyRepo,
		projectRepo:                projectRepo,
		signatureRepo:              signatureRepo,
		corpConsoleURL:             corpConsoleURL,
		httpClient:                 httpClient,
	}
}

func (s service) AddCclaWhitelistRequest(ctx context.Context, companyID string, claGroupID string, args models.CclaWhitelistRequestInput) (string, error) {
	list, err := s.ListCclaWhitelistRequestByCompanyProjectUser(companyID, &claGroupID, nil, &args.ContributorID)
	if err != nil {
		log.Warnf("AddCclaWhitelistRequest - error looking up existing contributor invite requests for company: %s, project: %s, user by id: %s with name: %s, email: %s, error: %+v",
			companyID, claGroupID, args.ContributorID, args.ContributorName, args.ContributorEmail, err)
		return "", err
	}
	for _, item := range list.List {
		if item.RequestStatus == "pending" || item.RequestStatus == "approved" {
			log.Warnf("AddCclaWhitelistRequest - found existing contributor invite - id: %s, request for company: %s, project: %s, user by id: %s with name: %s, email: %s",
				list.List[0].RequestID, companyID, claGroupID, args.ContributorID, args.ContributorName, args.ContributorEmail)
			return "", ErrCclaApprovalRequestAlreadyExists
		}
	}
	companyModel, err := s.companyRepo.GetCompany(ctx, companyID)
	if err != nil {
		log.Warnf("AddCclaWhitelistRequest - unable to lookup company by id: %s, error: %+v", companyID, err)
		return "", err
	}
	claGroupModel, err := s.projectRepo.GetCLAGroupByID(ctx, claGroupID, DontLoadRepoDetails)
	if err != nil {
		log.Warnf("AddCclaWhitelistRequest - unable to lookup project by id: %s, error: %+v", claGroupID, err)
		return "", err
	}
	userModel, err := s.userRepo.GetUser(args.ContributorID)
	if err != nil {
		log.Warnf("AddCclaWhitelistRequest - unable to lookup user by id: %s with name: %s, email: %s, error: %+v",
			args.ContributorID, args.ContributorName, args.ContributorEmail, err)
		return "", err
	}
	if userModel == nil {
		log.Warnf("AddCclaWhitelistRequest - unable to lookup user by id: %s with name: %s, email: %s, error: user object not found",
			args.ContributorID, args.ContributorName, args.ContributorEmail)
		return "", errors.New("invalid user")
	}

	signed, approved := true, true
	sortOrder := utils.SortOrderAscending
	pageSize := int64(5)
	sig, sigErr := s.signatureRepo.GetProjectCompanySignatures(ctx, companyID, claGroupID, &signed, &approved, nil, &sortOrder, &pageSize)
	if sigErr != nil || sig == nil || sig.Signatures == nil {
		log.Warnf("AddCclaWhitelistRequest - unable to lookup signature by company id: %s project id: %s - (or no managers), sig: %+v, error: %+v",
			companyID, claGroupID, sig, err)
		return "", err
	}

	requestID, addErr := s.repo.AddCclaWhitelistRequest(companyModel, claGroupModel, userModel, args.ContributorName, args.ContributorEmail)
	if addErr != nil {
		log.Warnf("AddCclaWhitelistRequest - unable to add Approval Request for id: %s with name: %s, email: %s, error: %+v",
			args.ContributorID, args.ContributorName, args.ContributorEmail, addErr)
	}

	// Send the emails to the CLA managers for this CCLA Signature which includes the managers in the ACL list
	s.sendRequestSentEmail(companyModel, claGroupModel, sig.Signatures[0], args.ContributorName, args.ContributorEmail, args.RecipientName, args.RecipientEmail, args.Message)

	return requestID, nil
}

// ApproveCclaWhitelistRequest is the handler for the approve CLA request
func (s service) ApproveCclaWhitelistRequest(ctx context.Context, claUser *user.CLAUser, companyID, claGroupID, requestID string) error {

	f := logrus.Fields{
		"functionName": "ApproveCclaWhitelistRequest",
		"companyID":    companyID,
		"claGroupID":   claGroupID,
		"requestID":    requestID,
		"Approver":     claUser.Name,
	}
	err := s.repo.ApproveCclaWhitelistRequest(requestID)
	if err != nil {
		log.WithFields(f).Warnf("ApproveCclaWhitelistRequest - problem updating approved list with 'approved' status for request: %s, error: %+v",
			requestID, err)
		return err
	}

	requestModel, err := s.repo.GetCclaWhitelistRequest(requestID)
	if err != nil {
		log.Warnf("ApproveCclaWhitelistRequest - unable to lookup request by id: %s, error: %+v", requestID, err)
		return err
	}

	companyModel, err := s.companyRepo.GetCompany(ctx, companyID)
	if err != nil {
		log.Warnf("ApproveCclaWhitelistRequest - unable to lookup company by id: %s, error: %+v", companyID, err)
		return err
	}
	claGroupModel, err := s.projectRepo.GetCLAGroupByID(ctx, claGroupID, DontLoadRepoDetails)
	if err != nil {
		log.Warnf("ApproveCclaWhitelistRequest - unable to lookup project by id: %s, error: %+v", claGroupID, err)
		return err
	}

	if requestModel.UserEmails == nil {
		msg := fmt.Sprintf("ApproveCclaWhitelistRequest - unable to send approval email - email missing for request: %+v, error: %+v",
			requestModel, err)
		log.Warnf(msg)
		return errors.New(msg)
	}

	// Get project cla Group records
	log.WithFields(f).Debugf("Getting SalesForce Projects for claGroup: %s ", claGroupID)
	projectCLAGroups, getErr := s.projectsCLAGroupRepository.GetProjectsIdsForClaGroup(claGroupID)
	if getErr != nil {
		msg := fmt.Sprintf("Error getting SF projects for claGroup: %s ", claGroupID)
		log.Debug(msg)
	}

	if len(projectCLAGroups) == 0 {
		msg := fmt.Sprintf("Error getting SF projects for claGroup: %s ", claGroupID)
		return errors.New(msg)
	}

	signedAtFoundation, signedErr := s.projectService.SignedAtFoundationLevel(ctx, projectCLAGroups[0].FoundationSFID)
	if signedErr != nil {
		msg := fmt.Sprintf("Problem checking project: %s , error: %+v", claGroupID, signedErr)
		log.WithFields(f).Warn(msg)
		return signedErr
	}

	var projectSFIDs []string
	foundationSFID := projectCLAGroups[0].FoundationSFID

	if signedAtFoundation {
		// Get salesforce project by FoundationID
		log.WithFields(f).Debugf("querying project service for project details...")
		projectSFIDs = append(projectSFIDs, foundationSFID)
	} else {
		for _, pcg := range projectCLAGroups {
			log.WithFields(f).Debugf("Getting salesforce project by SFID: %s ", pcg.ProjectSFID)
			projectSFIDs = append(projectSFIDs, pcg.ProjectSFID)
		}
	}

	// Send the email
	s.sendRequestApprovedEmailToRecipient(ctx, s.projectService, s.projectsCLAGroupRepository, *claUser, companyModel, claGroupModel,
		requestModel.UserName, requestModel.UserEmails[0], projectSFIDs)

	return nil
}

// RejectCclaWhitelistRequest is the handler for the decline CLA request
func (s service) RejectCclaWhitelistRequest(ctx context.Context, companyID, claGroupID, requestID string) error {
	err := s.repo.RejectCclaWhitelistRequest(requestID)
	if err != nil {
		log.Warnf("RejectCclaWhitelistRequest - problem updating approved list with 'rejected' status for request: %s, error: %+v", requestID, err)
		return err
	}

	requestModel, err := s.repo.GetCclaWhitelistRequest(requestID)
	if err != nil {
		log.Warnf("RejectCclaWhitelistRequest - unable to lookup request by id: %s, error: %+v", requestID, err)
		return err
	}

	companyModel, err := s.companyRepo.GetCompany(ctx, companyID)
	if err != nil {
		log.Warnf("RejectCclaWhitelistRequest - unable to lookup company by id: %s, error: %+v", companyID, err)
		return err
	}
	claGroupModel, err := s.projectRepo.GetCLAGroupByID(ctx, claGroupID, DontLoadRepoDetails)
	if err != nil {
		log.Warnf("RejectCclaWhitelistRequest - unable to lookup project by id: %s, error: %+v", claGroupID, err)
		return err
	}

	signed, approved := true, true
	sortOrder := utils.SortOrderAscending
	pageSize := int64(5)
	sig, sigErr := s.signatureRepo.GetProjectCompanySignatures(ctx, companyID, claGroupID, &signed, &approved, nil, &sortOrder, &pageSize)
	if sigErr != nil || sig == nil || sig.Signatures == nil {
		log.Warnf("RejectCclaWhitelistRequest - unable to lookup signature by company id: %s project id: %s - (or no managers), sig: %+v, error: %+v",
			companyID, claGroupID, sig, err)
		return err
	}

	if requestModel.UserEmails == nil {
		msg := fmt.Sprintf("RejectCclaWhitelistRequest - unable to send approval email - email missing for request: %+v, error: %+v",
			requestModel, err)
		log.Warnf(msg)
		return errors.New(msg)
	}

	// Send the email
	s.sendRequestRejectedEmailToRecipient(companyModel, claGroupModel, sig.Signatures[0], requestModel.UserName, requestModel.UserEmails[0])

	return nil
}

// ListCclaWhitelistRequest is the handler for the list CLA request
func (s service) ListCclaWhitelistRequest(companyID string, claGroupID, status *string) (*models.CclaWhitelistRequestList, error) {
	return s.repo.ListCclaWhitelistRequest(companyID, claGroupID, status, nil)
}

// ListCclaWhitelistRequestByCompanyProjectUser is the handler for the list CLA request
func (s service) ListCclaWhitelistRequestByCompanyProjectUser(companyID string, claGroupID, status, userID *string) (*models.CclaWhitelistRequestList, error) {
	return s.repo.ListCclaWhitelistRequest(companyID, claGroupID, status, userID)
}

// sendRequestSentEmail sends emails to the CLA managers specified in the signature record
func (s service) sendRequestSentEmail(companyModel *models.Company, claGroupModel *models.ClaGroup, signature *models.Signature, contributorName, contributorEmail, recipientName, recipientEmail, message string) {

	// If we have an override name and email from the request - possibly from the web form where the user selected the
	// CLA Manager Name/Email from a list, send this to this recipient (CLA Manager) - otherwise we will send to all
	// CLA Managers on the Signature ACL
	if recipientName != "" && recipientEmail != "" {
		s.sendRequestEmailToRecipient(s.projectsCLAGroupRepository, companyModel, claGroupModel, contributorName, contributorEmail, recipientName, recipientEmail, message)
		return
	}

	// Send an email to each manager
	for _, manager := range signature.SignatureACL {

		// Need to determine which email...
		var whichEmail = ""
		if manager.LfEmail != "" {
			whichEmail = manager.LfEmail
		}

		// If no LF Email try to grab the first other email in their email list
		if manager.LfEmail == "" && manager.Emails != nil {
			whichEmail = manager.Emails[0]
		}
		if whichEmail == "" {
			log.Warnf("unable to send email to manager: %+v - no email on file...", manager)
		} else {
			// Send the email
			s.sendRequestEmailToRecipient(s.projectsCLAGroupRepository, companyModel, claGroupModel, contributorName, contributorEmail, manager.Username, whichEmail, message)
		}
	}
}

// sendRequestEmailToRecipient generates and sends an email to the specified recipient
func (s service) sendRequestEmailToRecipient(projectClaGroupRepository projects_cla_groups.Repository, companyModel *models.Company, claGroupModel *models.ClaGroup, contributorName, contributorEmail, recipientName, recipientAddress, message string) {
	companyName := companyModel.CompanyName
	projectName := claGroupModel.ProjectName

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Request to Authorize %s for %s", contributorName, projectName)
	recipients := []string{recipientAddress}
	body, err := emails.RenderRequestToAuthorizeTemplate(projectClaGroupRepository, s.projectService, claGroupModel.Version, claGroupModel.ProjectExternalID,
		emails.RequestToAuthorizeTemplateParams{
			CLAManagerTemplateParams: emails.CLAManagerTemplateParams{
				RecipientName: recipientName,
				Project:       emails.CLAProjectParams{ExternalProjectName: projectName},
				CompanyName:   companyName,
			},
			ContributorName:     contributorName,
			ContributorEmail:    contributorEmail,
			OptionalMessage:     message,
			CorporateConsoleURL: s.corpConsoleURL,
			CompanyID:           companyModel.CompanyID,
		})
	if err != nil {
		log.Warnf("rendering email template : %s failed : %v", emails.RequestToAuthorizeTemplateName, err)
		return
	}
	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

// sendRequestRejectedEmailToRecipient generates and sends an email to the specified recipient
func (s service) sendRequestRejectedEmailToRecipient(companyModel *models.Company, claGroupModel *models.ClaGroup, signature *models.Signature, recipientName, recipientAddress string) {
	companyName := companyModel.CompanyName
	projectName := claGroupModel.ProjectName

	emailCLAManagerParams := []emails.ClaManagerInfoParams{}
	// Build a fancy text string with CLA Manager name <email> as an HTML unordered list
	for _, manager := range signature.SignatureACL {

		// Need to determine which email...
		var whichEmail = ""
		if manager.LfEmail != "" {
			whichEmail = manager.LfEmail
		}

		// If no LF Email try to grab the first other email in their email list
		if manager.LfEmail == "" && manager.Emails != nil {
			whichEmail = manager.Emails[0]
		}
		if whichEmail == "" {
			log.Warnf("unable to send email to manager: %+v - no email on file...", manager)
		} else {
			emailCLAManagerParams = append(emailCLAManagerParams, emails.ClaManagerInfoParams{
				LfUsername: manager.Username,
				Email:      whichEmail,
			})
		}
	}

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Approval List Request Denied for Project %s", projectName)
	recipients := []string{recipientAddress}
	body, err := emails.RenderTemplate(claGroupModel.Version, emails.ApprovalListRejectedTemplateName,
		emails.ApprovalListRejectedTemplate,
		emails.ApprovalListRejectedTemplateParams{
			CLAManagerTemplateParams: emails.CLAManagerTemplateParams{
				RecipientName: recipientName,
				Project:       emails.CLAProjectParams{ExternalProjectName: projectName},
				CompanyName:   companyName,
				CLAManagers:   emailCLAManagerParams,
			},
		},
	)
	if err != nil {
		log.Warnf("rendering email failed for : %s : %v", emails.ApprovalListRejectedTemplateName, err)
		return
	}
	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

func (s service) sendRequestApprovedEmailToRecipient(ctx context.Context, projectService project.Service, repository projects_cla_groups.Repository, claUser user.CLAUser, companyModel *models.Company, claGroupModel *models.ClaGroup, recipientName, recipientAddress string, projectSFIDs []string) {

	f := logrus.Fields{
		"functionName":     "sendRequestApprovedEmailToRecipient",
		utils.XREQUESTID:   ctx.Value((utils.XREQUESTID)),
		"claGroupName":     claGroupModel.ProjectName,
		"claGroupID":       claGroupModel.ProjectID,
		"companyName":      companyModel.CompanyName,
		"recipientName":    recipientName,
		"recipientAddress": recipientAddress,
	}

	companyName := companyModel.CompanyName
	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Approved List Request Accepted for %s", companyName)
	recipients := []string{recipientAddress}

	approver := ""
	if claUser.LFUsername != "" {
		approver = claUser.LFUsername
	} else if claUser.LFEmail != "" {
		approver = claUser.LFEmail
	} else if claUser.Emails != nil {
		approver = claUser.Emails[0]
	}

	body, err := emails.RenderApprovalListTemplate(
		repository, projectService, projectSFIDs, emails.ApprovalListApprovedTemplateParams{
			ApprovalTemplateParams: emails.ApprovalTemplateParams{
				RecipientName: recipientName,
				CompanyName:   companyName,
				CLAGroupName:  claGroupModel.ProjectName,
				Approver:      approver,
			},
		},
	)
	if err != nil {
		log.WithFields(f).Warnf("rendering email failed for : %s : %v", emails.ApprovalListApprovedTemplateName, err)
		return
	}
	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.WithFields(f).Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.WithFields(f).Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}
