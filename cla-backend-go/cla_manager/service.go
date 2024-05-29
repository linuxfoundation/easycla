// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"fmt"

	service2 "github.com/communitybridge/easycla/cla-backend-go/project/service"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/emails"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	sigAPI "github.com/communitybridge/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/signatures"
	"github.com/communitybridge/easycla/cla-backend-go/users"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	v2UserService "github.com/communitybridge/easycla/cla-backend-go/v2/user-service"
)

// IService interface defining the functions for the company service
type IService interface {
	CreateRequest(reqModel *CLAManagerRequest) (*models.ClaManagerRequest, error)
	GetRequests(companyID, claGroupID string) (*models.ClaManagerRequestList, error)
	GetRequestsByUserID(companyID, claGroupID, userID string) (*models.ClaManagerRequestList, error)
	GetRequest(requestID string) (*models.ClaManagerRequest, error)

	ApproveRequest(companyID, claGroupID, requestID string) (*models.ClaManagerRequest, error)
	DenyRequest(companyID, claGroupID, requestID string) (*models.ClaManagerRequest, error)
	PendingRequest(companyID, claGroupID, requestID string) (*models.ClaManagerRequest, error)
	DeleteRequest(requestID string) error

	AddClaManager(ctx context.Context, authUser *auth.User, companyID string, claGroupID string, LFID string, projectSFName string) (*models.Signature, error)
	RemoveClaManager(ctx context.Context, authUser *auth.User, companyID string, claGroupID string, LFID string, projectSFName string) (*models.Signature, error)
}

type service struct {
	repo                 IRepository
	projectClaRepository projects_cla_groups.Repository
	companyService       company.IService
	projectService       service2.Service
	usersService         users.Service
	sigService           signatures.SignatureService
	eventsService        events.Service
	emailTemplateService emails.EmailTemplateService
	corporateConsoleURL  string
}

// NewService creates a new service object
func NewService(repo IRepository, projectClaRepository projects_cla_groups.Repository, companyService company.IService, projectService service2.Service, usersService users.Service, sigService signatures.SignatureService, eventsService events.Service, emailTemplateService emails.EmailTemplateService, corporateConsoleURL string) IService {
	return service{
		repo:                 repo,
		projectClaRepository: projectClaRepository,
		companyService:       companyService,
		projectService:       projectService,
		usersService:         usersService,
		sigService:           sigService,
		eventsService:        eventsService,
		emailTemplateService: emailTemplateService,
		corporateConsoleURL:  corporateConsoleURL,
	}
}

// CreateRequest creates a request based on the specified parameters
func (s service) CreateRequest(reqModel *CLAManagerRequest) (*models.ClaManagerRequest, error) {
	request, err := s.repo.CreateRequest(reqModel)
	if err != nil {
		log.Warnf("problem with approving request for company ID: %s, project ID: %s, user ID: %s, user name: %s, error :%+v",
			reqModel.CompanyID, reqModel.ProjectID, reqModel.UserID, reqModel.UserName, err)
		return nil, err
	}

	respModel := dbModelToServiceModel(*request)

	return &respModel, err
}

// GetRequests returns a requests object based on the specified parameters
func (s service) GetRequests(companyID, claGroupID string) (*models.ClaManagerRequestList, error) {
	requests, err := s.repo.GetRequests(companyID, claGroupID)
	if err != nil {
		log.Warnf("problem with fetching request for company ID: %s, project ID: %s, error :%+v",
			companyID, claGroupID, err)
		return nil, err
	}

	// Convert to a service response model
	responseModel := models.ClaManagerRequestList{}
	for _, request := range requests.Requests {
		responseModel.Requests = append(responseModel.Requests, dbModelToServiceModel(request))
	}

	return &responseModel, nil
}

// GetRequestsByUserID returns a requests object based on the specified parameters
func (s service) GetRequestsByUserID(companyID, claGroupID, userID string) (*models.ClaManagerRequestList, error) {
	requests, err := s.repo.GetRequestsByUserID(companyID, claGroupID, userID)
	if err != nil {
		log.Warnf("problem with fetching request for company ID: %s, project ID: %s, user ID: %s, error :%+v",
			companyID, claGroupID, userID, err)
		return nil, err
	}

	// Convert to a service response model
	responseModel := models.ClaManagerRequestList{}
	for _, request := range requests.Requests {
		responseModel.Requests = append(responseModel.Requests, dbModelToServiceModel(request))
	}

	return &responseModel, nil
}

// GetRequest returns the request object based on the specified parameters
func (s service) GetRequest(requestID string) (*models.ClaManagerRequest, error) {
	request, err := s.repo.GetRequest(requestID)
	if err != nil {
		log.Warnf("problem with fetching request for request ID: %s, error :%+v",
			requestID, err)
		return nil, err
	}

	if request == nil {
		log.Debugf("request not found for Request ID: %s", requestID)
		return nil, nil
	}

	respModel := dbModelToServiceModel(*request)

	return &respModel, err
}

// ApproveRequest approves the request based on the specified parameters
func (s service) ApproveRequest(companyID, claGroupID, requestID string) (*models.ClaManagerRequest, error) {
	request, err := s.repo.ApproveRequest(companyID, claGroupID, requestID)
	if err != nil {
		log.Warnf("problem with approving request for company ID: %s, project ID: %s, request ID: %s, error :%+v",
			companyID, claGroupID, requestID, err)
		return nil, err
	}

	respModel := dbModelToServiceModel(*request)

	return &respModel, err
}

// PendingRequest updates the specified request to the pending state
func (s service) PendingRequest(companyID, claGroupID, requestID string) (*models.ClaManagerRequest, error) {
	request, err := s.repo.PendingRequest(companyID, claGroupID, requestID)
	if err != nil {
		log.Warnf("problem with setting the pending status for company ID: %s, project ID: %s, request ID: %s, error :%+v",
			companyID, claGroupID, requestID, err)
		return nil, err
	}

	respModel := dbModelToServiceModel(*request)

	return &respModel, err
}

// DenyRequest denies the request based on the specified parameters
func (s service) DenyRequest(companyID, claGroupID, requestID string) (*models.ClaManagerRequest, error) {
	request, err := s.repo.DenyRequest(companyID, claGroupID, requestID)
	if err != nil {
		log.Warnf("problem with denying request for company ID: %s, project ID: %s, request ID: %s, error :%+v",
			companyID, claGroupID, requestID, err)
		return nil, err
	}

	respModel := dbModelToServiceModel(*request)

	return &respModel, err
}

// DeleteRequest deletes the request based on the specified parameters
func (s service) DeleteRequest(requestID string) error {
	err := s.repo.DeleteRequest(requestID)
	if err != nil {
		log.Warnf("problem deleting request ID: %s, error :%+v",
			requestID, err)
		return err
	}
	return nil
}

// AddClaManager Adds LFID to Signature Access Control list
func (s service) AddClaManager(ctx context.Context, authUser *auth.User, companyID string, claGroupID string, LFID string, projectSFName string) (*models.Signature, error) {

	f := logrus.Fields{
		"functionName":   "v1.cla_manager.AddClaManager",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"claGroupID":     claGroupID,
		"LFID":           LFID,
		"projectName":    projectSFName,
	}

	userModel, userErr := s.usersService.GetUserByLFUserName(LFID)
	if userErr != nil || userModel == nil {
		return nil, userErr
	}
	companyModel, companyErr := s.companyService.GetCompany(ctx, companyID)
	if companyErr != nil || companyModel == nil {
		return nil, companyErr
	}

	claGroupModel, projectErr := s.projectService.GetCLAGroupByID(ctx, claGroupID)
	if projectErr != nil || claGroupModel == nil {
		return nil, projectErr
	}

	// if projectSFName is empty, we can set clagroup project name.
	if projectSFName == "" {
		projectSFName = claGroupModel.ProjectName
	}

	// Look up signature ACL to ensure the user can add cla manager

	signed := true
	approved := true
	sigModel, sigErr := s.sigService.GetProjectCompanySignature(ctx, companyID, claGroupID, &signed, &approved, nil, aws.Int64(5))
	if sigErr != nil || sigModel == nil {
		return nil, sigErr
	}

	claManagers := sigModel.SignatureACL

	log.WithFields(f).Debugf("Got Company signatures - Company: %s , Project: %s , signatureID: %s ",
		companyID, claGroupID, sigModel.SignatureID)

	// Update the signature ACL
	addedSignature, aclErr := s.sigService.AddCLAManager(ctx, sigModel.SignatureID, LFID)
	if aclErr != nil {
		return nil, aclErr
	}

	// Update the company ACL record in EasyCLA
	companyACLError := s.companyService.AddUserToCompanyAccessList(ctx, companyID, LFID)
	if companyACLError != nil {
		log.WithFields(f).Warnf("AddCLAManager- Unable to add user to company ACL, companyID: %s, user: %s, error: %+v", companyID, LFID, companyACLError)
		return nil, companyACLError
	}

	// Notify CLA Managers - send email to each manager
	for _, manager := range claManagers {
		sendClaManagerAddedEmailToCLAManagers(s.emailTemplateService, emails.ClaManagerAddedToCLAManagersTemplateParams{
			CommonEmailParams: emails.CommonEmailParams{
				RecipientName:    manager.Username,
				RecipientAddress: manager.LfEmail.String(),
				CompanyName:      companyModel.CompanyName,
			},
			Name:  userModel.Username,
			Email: userModel.LfEmail.String(),
		}, claGroupModel)
	}
	// Notify the added user
	s.sendClaManagerAddedEmailToUser(s.emailTemplateService, emails.CommonEmailParams{
		RecipientName:    userModel.Username,
		RecipientAddress: userModel.LfEmail.String(),
		CompanyName:      companyModel.CompanyName,
	}, claGroupModel)

	// Send an event
	s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType:     events.ClaManagerCreated,
		UserName:      authUser.UserName,
		LfUsername:    authUser.UserName,
		CLAGroupID:    claGroupID,
		CLAGroupName:  claGroupModel.ProjectName,
		ClaGroupModel: claGroupModel,
		ProjectID:     claGroupModel.ProjectExternalID,
		ProjectSFID:   claGroupModel.ProjectExternalID,
		CompanyID:     companyID,
		CompanyModel:  companyModel,
		EventData: &events.CLAManagerCreatedEventData{
			CompanyName: companyModel.CompanyName,
			ProjectName: projectSFName,
			UserName:    userModel.Username,
			UserEmail:   userModel.LfEmail.String(),
			UserLFID:    userModel.LfUsername,
		},
	})

	return addedSignature, nil
}

// Utility function that returns company signature
func (s service) getCompanySignature(ctx context.Context, companyID string, claGroupID string) (*models.Signature, error) {
	// Look up signature ACL to ensure the user can remove given cla manager
	sigModels, sigErr := s.sigService.GetProjectCompanySignatures(ctx, sigAPI.GetProjectCompanySignaturesParams{
		HTTPRequest: nil,
		CompanyID:   companyID,
		ProjectID:   claGroupID,
		NextKey:     nil,
		PageSize:    aws.Int64(5),
	})
	if sigErr != nil || sigModels == nil {
		log.Warnf("Unable to lookup project company signature using Project ID: %s, Company ID: %s, error: %+v",
			claGroupID, companyID, sigErr)
		return nil, sigErr
	}

	if len(sigModels.Signatures) > 1 {
		log.Warnf("returned multiple CCLA signature models for company ID: %s, project ID: %s",
			companyID, claGroupID)
	}
	return sigModels.Signatures[0], nil
}

// RemoveClaManager removes lfid from signature acl with given company and project
func (s service) RemoveClaManager(ctx context.Context, authUser *auth.User, companyID string, claGroupID string, LFID string, projectSFName string) (*models.Signature, error) {

	f := logrus.Fields{
		"functionName":   "v1.cla_manager.RemoveClaManager",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"LFID":           LFID,
		"companyID":      companyID,
	}

	userModel, userErr := s.usersService.GetUserByLFUserName(LFID)
	if userErr != nil || userModel == nil {
		return nil, userErr
	}
	companyModel, companyErr := s.companyService.GetCompany(ctx, companyID)
	if companyErr != nil || companyModel == nil {
		return nil, companyErr
	}

	claGroupModel, projectErr := s.projectService.GetCLAGroupByID(ctx, claGroupID)
	if projectErr != nil || claGroupModel == nil {
		return nil, projectErr
	}

	// if projectSFName is empty, we can set clagroup project name.
	if projectSFName == "" {
		projectSFName = claGroupModel.ProjectName
	}

	signed := true
	approved := true
	sigModel, sigErr := s.sigService.GetProjectCompanySignature(ctx, companyID, claGroupID, &signed, &approved, nil, aws.Int64(5))
	if sigErr != nil || sigModel == nil {
		return nil, sigErr
	}

	if len(sigModel.SignatureACL) <= 1 {
		// Can't delete the only remaining CLA Manager....
		return nil, &utils.CLAManagerError{
			Message: "unable to remove the only remaining CLA Manager - signed CLAs must have at least one CLA Manager",
		}
	}

	// Update the signature ACL
	updatedSignature, aclErr := s.sigService.RemoveCLAManager(ctx, sigModel.SignatureID, LFID)
	if aclErr != nil || updatedSignature == nil {
		log.WithFields(f).Warnf("remove CLA Manager returned an error or empty signature model using Signature ID: %s, error: %+v",
			sigModel.SignatureID, sigErr)
		return nil, aclErr
	}

	// Get Updated cla manager list with removed manager for email purposes
	sigModel, sigErr = s.getCompanySignature(ctx, companyID, claGroupID)
	if sigErr != nil {
		return nil, sigErr
	}
	claManagers := sigModel.SignatureACL
	// Notify CLA Managers - send email to each manager
	for _, manager := range claManagers {
		s.sendClaManagerDeleteEmailToCLAManagers(s.emailTemplateService, emails.ClaManagerDeletedToCLAManagersTemplateParams{
			CommonEmailParams: emails.CommonEmailParams{
				RecipientName:    manager.Username,
				RecipientAddress: manager.LfEmail.String(),
				CompanyName:      companyModel.CompanyName,
			},
			Name:  userModel.LfUsername,
			Email: userModel.LfEmail.String(),
		}, claGroupModel)
	}

	// Notify the removed manager
	sendRemovedClaManagerEmailToRecipient(s.emailTemplateService, emails.CommonEmailParams{
		RecipientName:    userModel.LfUsername,
		RecipientAddress: userModel.LfEmail.String(),
		CompanyName:      companyModel.CompanyName,
	}, claGroupModel, claManagers)

	// Send an event
	s.eventsService.LogEvent(&events.LogEventArgs{
		EventType:     events.ClaManagerDeleted,
		LfUsername:    authUser.UserName,
		UserName:      authUser.UserName,
		CLAGroupID:    claGroupID,
		CLAGroupName:  claGroupModel.ProjectName,
		ClaGroupModel: claGroupModel,
		ProjectID:     claGroupModel.ProjectExternalID,
		ProjectSFID:   claGroupModel.ProjectExternalID,
		CompanyID:     companyID,
		CompanyModel:  companyModel,
		EventData: &events.CLAManagerDeletedEventData{
			CompanyName: companyModel.CompanyName,
			ProjectName: projectSFName,
			UserName:    userModel.LfUsername,
			UserEmail:   userModel.LfEmail.String(),
			UserLFID:    LFID,
		},
	})

	return updatedSignature, nil
}

type ProjectDetails struct {
	ProjectName string
	ProjectSFID string
}

func (s service) getProjectDetails(ctx context.Context, claGroupModel *models.ClaGroup) ProjectDetails {
	f := logrus.Fields{
		"functionName":   "v1.cla_manager.getProjectDetails",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupModel.ProjectID,
	}

	projectDetails := ProjectDetails{
		ProjectName: claGroupModel.ProjectName,
		ProjectSFID: claGroupModel.ProjectExternalID,
	}
	signedAtFoundation := false

	pcg, err := s.projectClaRepository.GetCLAGroup(ctx, claGroupModel.ProjectID)
	if err != nil {
		log.WithFields(f).Warnf("unable to fetch project cla group by project id: %s, error: %+v", claGroupModel.ProjectID, err)
	}

	// check if cla group is signed at foundation level
	if pcg != nil && pcg.FoundationSFID != "" {
		signedAtFoundation, err = s.projectClaRepository.IsExistingFoundationLevelCLAGroup(ctx, pcg.FoundationSFID)
		if err != nil {
			log.WithFields(f).Warnf("unable to fetch foundation level cla group by foundation id: %s, error: %+v", pcg.FoundationSFID, err)
		}

		if signedAtFoundation {
			log.WithFields(f).Debugf("cla group is signed at foundation level...")
			projectDetails.ProjectName = pcg.FoundationName
			projectDetails.ProjectSFID = pcg.FoundationSFID
		}
	}

	return projectDetails
}

func (s service) sendClaManagerAddedEmailToUser(emailSvc emails.EmailTemplateService, emailParams emails.CommonEmailParams, claGroupModel *models.ClaGroup) {
	projectDetails := s.getProjectDetails(context.Background(), claGroupModel)
	projectName := projectDetails.ProjectName
	projectSFID := projectDetails.ProjectSFID
	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Added as CLA Manager for Project :%s", projectName)
	recipients := []string{emailParams.RecipientAddress}
	body, err := emails.RenderClaManagerAddedEToUserTemplate(emailSvc, claGroupModel.Version, projectSFID, emails.ClaManagerAddedEToUserTemplateParams{
		CommonEmailParams: emailParams,
	})
	if err != nil {
		log.Warnf("email template render : %s failed : %v", emails.ClaManagerAddedEToUserTemplateName, err)
		return
	}

	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

func sendClaManagerAddedEmailToCLAManagers(emailSvc emails.EmailTemplateService, emailParams emails.ClaManagerAddedToCLAManagersTemplateParams, claGroupModel *models.ClaGroup) {
	projectName := claGroupModel.ProjectName

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: CLA Manager Added Notice for %s", projectName)
	recipients := []string{emailParams.RecipientAddress}
	body, err := emails.RenderClaManagerAddedToCLAManagersTemplate(emailSvc, claGroupModel.Version, claGroupModel.ProjectExternalID, emailParams)
	if err != nil {
		log.Warnf("email template render : %s failed : %v", emails.ClaManagerAddedToCLAManagersTemplate, err)
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
func sendRemovedClaManagerEmailToRecipient(emailSvc emails.EmailTemplateService, emailParams emails.CommonEmailParams, claGroupModel *models.ClaGroup, claManagers []models.User) {
	projectName := claGroupModel.ProjectName

	var emailCLAManagerParams []emails.ClaManagerInfoParams

	log.Debugf("CLA Managers found: %+v", claManagers)

	// Build a fancy text string with CLA Manager name <email> as an HTML unordered list
	for _, companyAdmin := range claManagers {

		// Need to determine which email...
		var whichEmail = ""
		if companyAdmin.LfEmail != "" {
			whichEmail = companyAdmin.LfEmail.String()
			log.Debugf("Found email : %s ", whichEmail)
		}

		// If no LF Email try to grab the first other email in their email list
		if companyAdmin.LfEmail == "" && companyAdmin.Emails != nil {
			whichEmail = companyAdmin.Emails[0]
		}

		// Try getting user email from userservice
		userClient := v2UserService.GetClient()
		if companyAdmin.LfUsername != "" && whichEmail == "" {
			email, emailErr := userClient.GetUserEmail(companyAdmin.LfUsername)
			if emailErr != nil {
				log.Warnf("unable to get user by username: %s , error: %+v ", companyAdmin.LfUsername, emailErr)
			} else if email != "" {
				whichEmail = email
			}
		}

		if whichEmail == "" {
			log.Warnf("unable to send email to manager: %+v - no email on file...", companyAdmin)
		} else {
			log.Warnf("Username: %s", companyAdmin.LfUsername)
			log.Warnf("Email: %s ", whichEmail)
			emailCLAManagerParams = append(emailCLAManagerParams, emails.ClaManagerInfoParams{
				LfUsername: companyAdmin.LfUsername,
				Email:      whichEmail,
			})
		}
	}

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Removed as CLA Manager for Project %s", projectName)
	recipients := []string{emailParams.RecipientAddress}
	body, err := emails.RenderRemovedCLAManagerTemplate(
		emailSvc,
		claGroupModel.Version,
		claGroupModel.ProjectExternalID,
		emails.RemovedCLAManagerTemplateParams{
			CommonEmailParams: emailParams,
			CLAManagers:       emailCLAManagerParams,
		})

	if err != nil {
		log.Warnf("rendering the email content failed for : %s", emails.RemovedCLAManagerTemplateName)
		return
	}

	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

func (s service) sendClaManagerDeleteEmailToCLAManagers(emailSvc emails.EmailTemplateService, emailParams emails.ClaManagerDeletedToCLAManagersTemplateParams, claGroupModel *models.ClaGroup) {
	projectDetails := s.getProjectDetails(context.Background(), claGroupModel)
	projectName := projectDetails.ProjectName
	projectSFID := projectDetails.ProjectSFID

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: CLA Manager Removed Notice for %s", projectName)
	recipients := []string{emailParams.RecipientAddress}
	body, err := emails.RenderClaManagerDeletedToCLAManagersTemplate(emailSvc, claGroupModel.Version, projectSFID, emailParams)

	if err != nil {
		log.Warnf("email template render : %s failed : %v", emails.ClaManagerDeletedToCLAManagersTemplateName, err)
		return
	}

	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}
