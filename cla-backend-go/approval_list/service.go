// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package approval_list

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	repository2 "github.com/communitybridge/easycla/cla-backend-go/project/repository"
	service2 "github.com/communitybridge/easycla/cla-backend-go/project/service"

	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/communitybridge/easycla/cla-backend-go/emails"

	"github.com/communitybridge/easycla/cla-backend-go/signatures"
	"github.com/communitybridge/easycla/cla-backend-go/utils"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/user"
	"github.com/communitybridge/easycla/cla-backend-go/users"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
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
	AddCclaApprovalListRequest(ctx context.Context, companyID string, claGroupID string, args models.CclaWhitelistRequestInput) (string, error)
	ApproveCclaApprovalListRequest(ctx context.Context, claUser *user.CLAUser, ClacompanyID, claGroupID, requestID string) error
	RejectCclaApprovalListRequest(ctx context.Context, companyID, claGroupID, requestID string) error
	ListCclaApprovalListRequest(companyID string, claGroupID, status *string) (*models.CclaWhitelistRequestList, error)
	ListCclaApprovalListRequestByCompanyProjectUser(companyID string, claGroupID, status, userID *string) (*models.CclaWhitelistRequestList, error)
}

type service struct {
	repo                       IRepository
	projectService             service2.Service
	userRepo                   users.UserRepository
	companyRepo                company.IRepository
	projectRepo                repository2.ProjectRepository
	signatureRepo              signatures.SignatureRepository
	projectsCLAGroupRepository projects_cla_groups.Repository
	emailTemplateService       emails.EmailTemplateService
	corpConsoleURL             string
	httpClient                 *http.Client
}

// NewService creates a new approval list service
func NewService(repo IRepository, projectsCLAGroupRepository projects_cla_groups.Repository, projService service2.Service, userRepo users.UserRepository, companyRepo company.IRepository, projectRepo repository2.ProjectRepository, signatureRepo signatures.SignatureRepository, emailTemplateService emails.EmailTemplateService, corpConsoleURL string, httpClient *http.Client) IService {
	return service{
		repo:                       repo,
		projectService:             projService,
		userRepo:                   userRepo,
		companyRepo:                companyRepo,
		projectRepo:                projectRepo,
		signatureRepo:              signatureRepo,
		projectsCLAGroupRepository: projectsCLAGroupRepository,
		emailTemplateService:       emailTemplateService,
		corpConsoleURL:             corpConsoleURL,
		httpClient:                 httpClient,
	}
}

func (s service) AddCclaApprovalListRequest(ctx context.Context, companyID string, claGroupID string, args models.CclaWhitelistRequestInput) (string, error) {
	f := logrus.Fields{
		"functionName":     "v1.approval_list.service.AddCclaApprovalListRequest",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"companyID":        companyID,
		"claGroupID":       claGroupID,
		"RecipientName":    args.RecipientName,
		"RecipientEmail":   args.RecipientEmail,
		"ContributorID":    args.ContributorID,
		"ContributorName":  args.ContributorName,
		"ContributorEmail": args.ContributorEmail,
		"Message":          args.Message,
	}

	list, err := s.ListCclaApprovalListRequestByCompanyProjectUser(companyID, &claGroupID, nil, &args.ContributorID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error looking up existing contributor invite requests for company: %s, project: %s, user by id: %s with name: %s, email: %s, error: %+v",
			companyID, claGroupID, args.ContributorID, args.ContributorName, args.ContributorEmail, err)
		return "", err
	}
	for _, item := range list.List {
		if item.RequestStatus == "pending" || item.RequestStatus == "approved" {
			log.WithFields(f).Warnf("found existing contributor invite - id: %s, request for company: %s, project: %s, user by id: %s with name: %s, email: %s",
				list.List[0].RequestID, companyID, claGroupID, args.ContributorID, args.ContributorName, args.ContributorEmail)
			return "", ErrCclaApprovalRequestAlreadyExists
		}
	}
	companyModel, err := s.companyRepo.GetCompany(ctx, companyID)
	if err != nil {
		log.WithFields(f).Warnf("unable to lookup company by id: %s, error: %+v", companyID, err)
		return "", err
	}
	claGroupModel, err := s.projectRepo.GetCLAGroupByID(ctx, claGroupID, DontLoadRepoDetails)
	if err != nil {
		log.WithFields(f).Warnf("unable to lookup project by id: %s, error: %+v", claGroupID, err)
		return "", err
	}

	log.WithFields(f).Debugf("looking up user by user ID: %s", args.ContributorID)
	userModel, err := s.userRepo.GetUser(args.ContributorID)
	if err != nil || userModel == nil {
		log.WithFields(f).WithError(err).Warnf("unable to lookup user by id: %s with name: %s, email: %s, error: %+v",
			args.ContributorID, args.ContributorName, args.ContributorEmail, err)

		log.WithFields(f).Debugf("looking up user by user email: %s", args.ContributorEmail)
		userModel, err = s.userRepo.GetUserByEmail(args.ContributorEmail)
		if err != nil || userModel == nil {
			log.WithFields(f).WithError(err).Warnf("unable to lookup user by email: %s with name: %s, error: %+v",
				args.ContributorName, args.ContributorEmail, err)
			if err != nil {
				return "", err
			}
			return "", errors.New("invalid user")
		}
	}

	signed, approved := true, true
	sortOrder := utils.SortOrderAscending
	pageSize := int64(5)
	sig, sigErr := s.signatureRepo.GetProjectCompanySignatures(ctx, companyID, claGroupID, &signed, &approved, nil, &sortOrder, &pageSize)
	if sigErr != nil || sig == nil || sig.Signatures == nil {
		log.WithFields(f).Warnf("unable to lookup signature by company id: %s project id: %s - (or no managers), sig: %+v, error: %+v",
			companyID, claGroupID, sig, err)
		return "", err
	}

	requestID, addErr := s.repo.AddCclaApprovalRequest(companyModel, claGroupModel, userModel, args.ContributorName, args.ContributorEmail)
	if addErr != nil {
		log.WithFields(f).Warnf("unable to add Approval Request for id: %s with name: %s, email: %s, error: %+v",
			args.ContributorID, args.ContributorName, args.ContributorEmail, addErr)
	}

	// Send the emails to the CLA managers for this CCLA Signature which includes the managers in the ACL list
	s.sendRequestSentEmail(companyModel, claGroupModel, sig.Signatures[0], args.ContributorName, args.ContributorEmail, args.RecipientName, args.RecipientEmail, args.Message)

	return requestID, nil
}

// ApproveCclaApprovalListRequest is the handler for the approve CLA request
func (s service) ApproveCclaApprovalListRequest(ctx context.Context, claUser *user.CLAUser, companyID, claGroupID, requestID string) error {
	f := logrus.Fields{
		"functionName": "v1.approval_list.service.ApproveCclaApprovalListRequest",
		"companyID":    companyID,
		"claGroupID":   claGroupID,
		"requestID":    requestID,
		"Approver":     claUser.Name,
	}

	err := s.repo.ApproveCclaApprovalListRequest(requestID)
	if err != nil {
		log.WithFields(f).Warnf("ApproveCclaApprovalListRequest - problem updating approved list with 'approved' status for request: %s, error: %+v",
			requestID, err)
		return err
	}

	requestModel, err := s.repo.GetCclaApprovalListRequest(requestID)
	if err != nil {
		log.Warnf("ApproveCclaApprovalListRequest - unable to lookup request by id: %s, error: %+v", requestID, err)
		return err
	}

	companyModel, err := s.companyRepo.GetCompany(ctx, companyID)
	if err != nil {
		log.Warnf("ApproveCclaApprovalListRequest - unable to lookup company by id: %s, error: %+v", companyID, err)
		return err
	}
	_, err = s.projectRepo.GetCLAGroupByID(ctx, claGroupID, DontLoadRepoDetails)
	if err != nil {
		log.Warnf("ApproveCclaApprovalListRequest - unable to lookup project by id: %s, error: %+v", claGroupID, err)
		return err
	}

	if requestModel.UserEmails == nil {
		msg := fmt.Sprintf("ApproveCclaApprovalListRequest - unable to send approval email - email missing for request: %+v, error: %+v",
			requestModel, err)
		log.Warnf("%s", msg)
		return errors.New(msg)
	}

	// Get project cla Group records
	log.WithFields(f).Debugf("Getting SalesForce Projects for claGroup: %s ", claGroupID)
	projectCLAGroups, getErr := s.projectsCLAGroupRepository.GetProjectsIdsForClaGroup(ctx, claGroupID)
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
	s.sendRequestApprovedEmailToRecipient(ctx,
		emails.CommonEmailParams{
			RecipientName:    requestModel.UserName,
			RecipientAddress: requestModel.UserEmails[0],
			CompanyName:      companyModel.CompanyName,
		}, *claUser, projectSFIDs)

	return nil
}

// RejectCclaApprovalListRequest is the handler for the decline CLA request
func (s service) RejectCclaApprovalListRequest(ctx context.Context, companyID, claGroupID, requestID string) error {
	f := logrus.Fields{
		"functionName": "v1.approval_list.service.RejectCclaApprovalListRequest",
		"companyID":    companyID,
		"claGroupID":   claGroupID,
		"requestID":    requestID,
	}

	err := s.repo.RejectCclaApprovalListRequest(requestID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem updating approved list with 'rejected' status for request: %s, error: %+v", requestID, err)
		return err
	}

	requestModel, err := s.repo.GetCclaApprovalListRequest(requestID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to lookup request by id: %s, error: %+v", requestID, err)
		return err
	}

	companyModel, err := s.companyRepo.GetCompany(ctx, companyID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to lookup company by id: %s, error: %+v", companyID, err)
		return err
	}
	claGroupModel, err := s.projectRepo.GetCLAGroupByID(ctx, claGroupID, DontLoadRepoDetails)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to lookup project by id: %s, error: %+v", claGroupID, err)
		return err
	}

	signed, approved := true, true
	sortOrder := utils.SortOrderAscending
	pageSize := int64(5)
	sig, sigErr := s.signatureRepo.GetProjectCompanySignatures(ctx, companyID, claGroupID, &signed, &approved, nil, &sortOrder, &pageSize)
	if sigErr != nil || sig == nil || sig.Signatures == nil {
		log.WithFields(f).WithError(sigErr).Warnf("unable to lookup signature by company id: %s project id: %s - (or no managers), sig: %+v, error: %+v",
			companyID, claGroupID, sig, err)
		return err
	}

	if requestModel.UserEmails == nil {
		msg := fmt.Sprintf("unable to send approval email - email missing for request: %+v, error: %+v",
			requestModel, err)
		log.WithFields(f).Warnf("%s", msg)
		return errors.New(msg)
	}

	// Send the email
	s.sendRequestRejectedEmailToRecipient(emails.CommonEmailParams{
		RecipientName:    requestModel.UserName,
		RecipientAddress: requestModel.UserEmails[0],
		CompanyName:      companyModel.CompanyName,
	}, claGroupModel, sig.Signatures[0])

	return nil
}

// ListCclaApprovalListRequest is the handler for the list CLA request
func (s service) ListCclaApprovalListRequest(companyID string, claGroupID, status *string) (*models.CclaWhitelistRequestList, error) {
	return s.repo.ListCclaApprovalListRequests(companyID, claGroupID, status, nil)
}

// ListCclaApprovalListRequestByCompanyProjectUser is the handler for the list CLA request
func (s service) ListCclaApprovalListRequestByCompanyProjectUser(companyID string, claGroupID, status, userID *string) (*models.CclaWhitelistRequestList, error) {
	return s.repo.ListCclaApprovalListRequests(companyID, claGroupID, status, userID)
}

// sendRequestSentEmail sends emails to the CLA managers specified in the signature record
func (s service) sendRequestSentEmail(companyModel *models.Company, claGroupModel *models.ClaGroup, signature *models.Signature, contributorName, contributorEmail, recipientName, recipientEmail, message string) {

	// If we have an override name and email from the request - possibly from the web form where the user selected the
	// CLA Manager Name/Email from a list, send this to this recipient (CLA Manager) - otherwise we will send to all
	// CLA Managers on the Signature ACL
	if recipientName != "" && recipientEmail != "" {
		s.sendRequestEmailToRecipient(emails.RequestToAuthorizeTemplateParams{
			CommonEmailParams: emails.CommonEmailParams{
				RecipientName:    recipientName,
				RecipientAddress: recipientEmail,
				CompanyName:      companyModel.CompanyName,
			},
			ContributorName:  contributorName,
			ContributorEmail: contributorEmail,
			OptionalMessage:  message,
			CompanyID:        companyModel.CompanyID,
		}, claGroupModel)
		return
	}

	// Send an email to each manager
	for _, manager := range signature.SignatureACL {

		// Need to determine which email...
		var whichEmail = ""
		if manager.LfEmail != "" {
			whichEmail = manager.LfEmail.String()
		}

		// If no LF Email try to grab the first other email in their email list
		if manager.LfEmail == "" && manager.Emails != nil {
			whichEmail = manager.Emails[0]
		}
		if whichEmail == "" {
			log.Warnf("unable to send email to manager: %+v - no email on file...", manager)
		} else {
			// Send the email
			s.sendRequestEmailToRecipient(emails.RequestToAuthorizeTemplateParams{
				CommonEmailParams: emails.CommonEmailParams{
					RecipientName:    manager.Username,
					RecipientAddress: whichEmail,
					CompanyName:      companyModel.CompanyName,
				},
				ContributorName:  contributorName,
				ContributorEmail: contributorEmail,
				OptionalMessage:  message,
			}, claGroupModel)
		}
	}
}

// sendRequestEmailToRecipient generates and sends an email to the specified recipient
func (s service) sendRequestEmailToRecipient(emailParams emails.RequestToAuthorizeTemplateParams, claGroupModel *models.ClaGroup) {
	projectName := claGroupModel.ProjectName
	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Request to Authorize %s for %s", emailParams.ContributorName, projectName)
	recipients := []string{emailParams.RecipientAddress}
	body, err := emails.RenderRequestToAuthorizeTemplate(s.emailTemplateService, claGroupModel.Version, claGroupModel.ProjectExternalID, emailParams)
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
func (s service) sendRequestRejectedEmailToRecipient(emailParams emails.CommonEmailParams, claGroupModel *models.ClaGroup, signature *models.Signature) {
	projectName := claGroupModel.ProjectName

	emailCLAManagerParams := []emails.ClaManagerInfoParams{}
	// Build a fancy text string with CLA Manager name <email> as an HTML unordered list
	for _, manager := range signature.SignatureACL {

		// Need to determine which email...
		var whichEmail = ""
		if manager.LfEmail != "" {
			whichEmail = manager.LfEmail.String()
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
	recipients := []string{emailParams.RecipientAddress}
	body, err := emails.RenderApprovalListRejectedTemplate(
		s.emailTemplateService, claGroupModel.Version, claGroupModel.ProjectExternalID, emails.ApprovalListRejectedTemplateParams{
			CommonEmailParams: emailParams,
			CLAManagers:       emailCLAManagerParams,
		})
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

func (s service) sendRequestApprovedEmailToRecipient(ctx context.Context, emailParams emails.CommonEmailParams, claUser user.CLAUser, projectSFIDs []string) {
	f := logrus.Fields{
		"functionName":     "v1.approval_list.service.sendRequestApprovedEmailToRecipient",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"companyName":      emailParams.CompanyName,
		"recipientName":    emailParams.RecipientName,
		"recipientAddress": emailParams.RecipientAddress,
	}

	companyName := emailParams.CompanyName
	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Approved List Request Accepted for %s", companyName)
	recipients := []string{emailParams.RecipientAddress}

	approver := ""
	if claUser.LFUsername != "" {
		approver = claUser.LFUsername
	} else if claUser.LFEmail != "" {
		approver = claUser.LFEmail
	} else if claUser.Emails != nil {
		approver = claUser.Emails[0]
	}

	body, err := emails.RenderApprovalListTemplate(
		s.emailTemplateService, projectSFIDs, emails.ApprovalListApprovedTemplateParams{
			CommonEmailParams: emailParams,
			Approver:          approver,
		})
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
