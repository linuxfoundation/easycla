// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/emails"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
	sigAPI "github.com/communitybridge/easycla/cla-backend-go/gen/restapi/operations/signatures"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/project"
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

	AddClaManager(ctx context.Context, companyID string, claGroupID string, LFID string, projectSFName string) (*models.Signature, error)
	RemoveClaManager(ctx context.Context, companyID string, claGroupID string, LFID string) (*models.Signature, error)
}

type service struct {
	repo                 IRepository
	projectClaRepository projects_cla_groups.Repository
	companyService       company.IService
	projectService       project.Service
	usersService         users.Service
	sigService           signatures.SignatureService
	eventsService        events.Service
	corporateConsoleURL  string
}

// NewService creates a new service object
func NewService(repo IRepository, projectClaRepository projects_cla_groups.Repository, companyService company.IService, projectService project.Service, usersService users.Service, sigService signatures.SignatureService, eventsService events.Service, corporateConsoleURL string) IService {
	return service{
		repo:                 repo,
		projectClaRepository: projectClaRepository,
		companyService:       companyService,
		projectService:       projectService,
		usersService:         usersService,
		sigService:           sigService,
		eventsService:        eventsService,
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

// AddClaManager Adds LFID to Signature Access Control List list
func (s service) AddClaManager(ctx context.Context, companyID string, claGroupID string, LFID string, projectSFName string) (*models.Signature, error) {

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

	// Look up signature ACL to ensure the user can add cla manager

	signed := true
	approved := true
	sigModel, sigErr := s.sigService.GetProjectCompanySignature(ctx, companyID, claGroupID, &signed, &approved, nil, aws.Int64(5))
	if sigErr != nil || sigModel == nil {
		return nil, sigErr
	}

	claManagers := sigModel.SignatureACL

	log.Debugf("Got Company signatures - Company: %s , Project: %s , signatureID: %s ",
		companyID, claGroupID, sigModel.SignatureID)

	// Update the signature ACL
	addedSignature, aclErr := s.sigService.AddCLAManager(ctx, sigModel.SignatureID, LFID)
	if aclErr != nil {
		return nil, aclErr
	}

	// Update the company ACL record in EasyCLA
	companyACLError := s.companyService.AddUserToCompanyAccessList(ctx, companyID, LFID)
	if companyACLError != nil {
		log.Warnf("AddCLAManager- Unable to add user to company ACL, companyID: %s, user: %s, error: %+v", companyID, LFID, companyACLError)
		return nil, companyACLError
	}

	// Notify CLA Managers - send email to each manager
	for _, manager := range claManagers {
		sendClaManagerAddedEmailToCLAManagers(companyModel, claGroupModel, userModel.Username, userModel.LfEmail,
			manager.Username, manager.LfEmail)
	}
	// Notify the added user
	sendClaManagerAddedEmailToUser(companyModel, claGroupModel, userModel.Username, userModel.LfEmail, projectSFName)

	// Send an event
	s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType:         events.ClaManagerCreated,
		ProjectID:         claGroupModel.ProjectExternalID,
		CLAGroupID:        claGroupID,
		CLAGroupName:      claGroupModel.ProjectName,
		ClaGroupModel:     claGroupModel,
		CompanyID:         companyID,
		CompanyModel:      companyModel,
		LfUsername:        LFID,
		UserID:            LFID,
		UserModel:         userModel,
		ExternalProjectID: claGroupModel.ProjectExternalID,
		EventData: &events.CLAManagerCreatedEventData{
			CompanyName: companyModel.CompanyName,
			ProjectName: claGroupModel.ProjectName,
			UserName:    userModel.Username,
			UserEmail:   userModel.LfEmail,
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
func (s service) RemoveClaManager(ctx context.Context, companyID string, claGroupID string, LFID string) (*models.Signature, error) {

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
		log.Warnf("remove CLA Manager returned an error or empty signature model using Signature ID: %s, error: %+v",
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
		sendClaManagerDeleteEmailToCLAManagers(companyModel, claGroupModel, userModel.LfUsername, userModel.LfEmail,
			manager.Username, manager.LfEmail)
	}

	// Notify the removed manager
	sendRemovedClaManagerEmailToRecipient(s.projectClaRepository, companyModel, claGroupModel, userModel.LfUsername, userModel.LfEmail, claManagers)

	// Send an event
	s.eventsService.LogEvent(&events.LogEventArgs{
		EventType:         events.ClaManagerDeleted,
		ProjectID:         claGroupModel.ProjectExternalID,
		CLAGroupID:        claGroupID,
		CLAGroupName:      claGroupModel.ProjectName,
		ClaGroupModel:     claGroupModel,
		CompanyID:         companyID,
		CompanyModel:      companyModel,
		LfUsername:        userModel.LfUsername,
		UserID:            LFID,
		UserModel:         userModel,
		ExternalProjectID: claGroupModel.ProjectExternalID,
		EventData: &events.CLAManagerDeletedEventData{
			CompanyName: companyModel.CompanyName,
			ProjectName: claGroupModel.ProjectName,
			UserName:    userModel.LfUsername,
			UserEmail:   userModel.LfEmail,
			UserLFID:    LFID,
		},
	})

	return updatedSignature, nil
}

func sendClaManagerAddedEmailToUser(companyModel *models.Company, claGroupModel *models.ClaGroup, requesterName, requesterEmail, projectSFName string) {
	companyName := companyModel.CompanyName
	projectName := claGroupModel.ProjectName
	templateName := emails.ClaManagerAddedEToUserTemplate
	if claGroupModel.Version == "v2" {
		templateName = emails.V2ClaManagerAddedEToUserTemplate
		projectName = projectSFName
	}

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Added as CLA Manager for Project :%s", projectName)
	recipients := []string{requesterEmail}
	body, err := emails.RenderTemplate(
		claGroupModel.Version,
		emails.ClaManagerAddedEToUserTemplateName,
		templateName,
		emails.ClaManagerAddedEToUserTemplateParams{
			CLAManagerTemplateParams: emails.CLAManagerTemplateParams{
				RecipientName: requesterName,
				Project:       emails.CLAProjectParams{ExternalProjectName: projectName},
				CompanyName:   companyName,
				CLAGroupName:  claGroupModel.ProjectName,
			},
			CorporateURL: utils.GetCorporateURL(claGroupModel.Version == utils.V2),
		},
	)
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

func sendClaManagerAddedEmailToCLAManagers(companyModel *models.Company, claGroupModel *models.ClaGroup, name, email, recipientName, recipientAddress string) {
	companyName := companyModel.CompanyName
	projectName := claGroupModel.ProjectName

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: CLA Manager Added Notice for %s", projectName)
	recipients := []string{recipientAddress}
	body, err := emails.RenderTemplate(claGroupModel.Version, emails.ClaManagerAddedToCLAManagersTemplateName,
		emails.ClaManagerAddedToCLAManagersTemplate,
		emails.ClaManagerAddedToCLAManagersTemplateParams{
			CLAManagerTemplateParams: emails.CLAManagerTemplateParams{
				RecipientName: recipientName,
				Project:       emails.CLAProjectParams{ExternalProjectName: projectName},
				CompanyName:   companyName,
			},
			Name:  name,
			Email: email,
		})

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
func sendRemovedClaManagerEmailToRecipient(projectsClaGroupRepository projects_cla_groups.Repository, companyModel *models.Company, claGroupModel *models.ClaGroup, recipientName, recipientAddress string, claManagers []models.User) {
	companyName := companyModel.CompanyName
	projectName := claGroupModel.ProjectName

	emailCLAManagerParams := []emails.ClaManagerInfoParams{}

	log.Debugf("CLA Managers found: %+v", claManagers)

	// Build a fancy text string with CLA Manager name <email> as an HTML unordered list
	for _, companyAdmin := range claManagers {

		// Need to determine which email...
		var whichEmail = ""
		if companyAdmin.LfEmail != "" {
			whichEmail = companyAdmin.LfEmail
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
	recipients := []string{recipientAddress}
	body, err := emails.RenderRemovedCLAManagerTemplate(
		projectsClaGroupRepository,
		claGroupModel.Version,
		recipientName,
		companyName,
		claGroupModel.ProjectExternalID,
		emailCLAManagerParams)

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

func sendClaManagerDeleteEmailToCLAManagers(companyModel *models.Company, claGroupModel *models.ClaGroup, name, email, recipientName, recipientAddress string) {
	companyName := companyModel.CompanyName
	projectName := claGroupModel.ProjectName

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: CLA Manager Removed Notice for %s", projectName)
	recipients := []string{recipientAddress}
	body, err := emails.RenderTemplate(claGroupModel.Version,
		emails.ClaManagerDeletedToCLAManagersTemplateName,
		emails.ClaManagerDeletedToCLAManagersTemplate,
		emails.ClaManagerDeletedToCLAManagersTemplateParams{
			CLAManagerTemplateParams: emails.CLAManagerTemplateParams{
				RecipientName: recipientName,
				Project:       emails.CLAProjectParams{ExternalProjectName: projectName},
				CompanyName:   companyName,
			},
			Name:  name,
			Email: email,
		})

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
