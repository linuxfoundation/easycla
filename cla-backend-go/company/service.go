// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/go-openapi/strfmt"

	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/users"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	orgServiceModels "github.com/communitybridge/easycla/cla-backend-go/v2/organization-service/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/user"
	organization_service "github.com/communitybridge/easycla/cla-backend-go/v2/organization-service"
	"github.com/communitybridge/easycla/cla-backend-go/v2/organization-service/client/organizations"
)

type service struct {
	repo                IRepository
	userDynamoRepo      user.RepositoryService
	corporateConsoleURL string
	userService         users.Service
}

const (
	// StatusPending indicates the invitation status is pending
	StatusPending = "pending"
)

// IService interface defining the functions for the company service
type IService interface { // nolint
	CreateOrgFromExternalID(ctx context.Context, signingEntityName, companySFID string) (*models.Company, error)

	GetCompanies(ctx context.Context) (*models.Companies, error)
	GetCompany(ctx context.Context, companyID string) (*models.Company, error)
	GetCompanyByExternalID(ctx context.Context, companySFID string) (*models.Company, error)
	GetCompaniesByExternalID(ctx context.Context, companySFID string, includeChildCompanies bool) ([]*models.Company, error)
	GetCompanyBySigningEntityName(ctx context.Context, signingEntityName, companySFID string) (*models.Company, error)
	SearchCompanyByName(ctx context.Context, companyName string, nextKey string) (*models.Companies, error)
	GetCompaniesByUserManager(ctx context.Context, userID string) (*models.Companies, error)
	GetCompaniesByUserManagerWithInvites(ctx context.Context, userID string) (*models.CompaniesWithInvites, error)

	AddUserToCompanyAccessList(ctx context.Context, companyID, lfid string) error
	GetCompanyInviteRequests(ctx context.Context, companyID string, status *string) ([]models.CompanyInviteUser, error)
	GetCompanyUserInviteRequests(ctx context.Context, companyID string, userID string) (*models.CompanyInviteUser, error)
	AddPendingCompanyInviteRequest(ctx context.Context, companyID string, userID string) (*InviteModel, error)
	ApproveCompanyAccessRequest(ctx context.Context, companyInviteID string) (*InviteModel, error)
	RejectCompanyAccessRequest(ctx context.Context, companyInviteID string) (*InviteModel, error)
	IsCCLAEnabledForCompany(ctx context.Context, companySFID string) (bool, error)

	// calls org service
	SearchOrganizationByName(ctx context.Context, orgName string, websiteName string, includeSigningEntityName bool, filter string) (*models.OrgList, error)

	sendRequestAccessEmail(ctx context.Context, companyModel *models.Company, requesterName, requesterEmail, recipientName, recipientAddress string)
	sendRequestApprovedEmailToRecipient(ctx context.Context, companyModel *models.Company, recipientName, recipientAddress string)
	sendRequestRejectedEmailToRecipient(ctx context.Context, companyModel *models.Company, recipientName, recipientAddress string)
	getPreferredNameAndEmail(ctx context.Context, lfid string) (string, string, error)
}

// NewService creates a new company service object
func NewService(repo IRepository, corporateConsoleURL string, userDynamoRepo user.RepositoryService, userService users.Service) IService {
	return service{
		repo:                repo,
		userDynamoRepo:      userDynamoRepo,
		corporateConsoleURL: corporateConsoleURL,
		userService:         userService,
	}
}

// GetCompanies returns all the companies
func (s service) GetCompanies(ctx context.Context) (*models.Companies, error) {
	return s.repo.GetCompanies(ctx)
}

// GetCompany returns the company associated with the company ID
func (s service) GetCompany(ctx context.Context, companyID string) (*models.Company, error) {
	return s.repo.GetCompany(ctx, companyID)
}

// SearchCompanyByName locates companies by the matching name and return any potential matches
func (s service) SearchCompanyByName(ctx context.Context, companyName string, nextKey string) (*models.Companies, error) {
	companies, err := s.repo.SearchCompanyByName(ctx, companyName, nextKey)
	if err != nil {
		log.Warnf("Error searching company by company name: %s, error: %v", companyName, err)
		return nil, err
	}

	return companies, nil
}

// GetCompanyUserManager the get a list of companies when provided the company id and user manager
func (s service) GetCompaniesByUserManager(ctx context.Context, userID string) (*models.Companies, error) {
	userModel, err := s.userDynamoRepo.GetUser(userID)
	if err != nil {
		log.Warnf("Unable to lookup user by user id: %s, error: %v", userID, err)
		return nil, err
	}

	return s.repo.GetCompaniesByUserManager(ctx, userID, userModel)
}

// GetCompanyUserManagerWithInvites the get a list of companies including status when provided the company id and user manager
func (s service) GetCompaniesByUserManagerWithInvites(ctx context.Context, userID string) (*models.CompaniesWithInvites, error) {
	userModel, err := s.userDynamoRepo.GetUser(userID)
	if err != nil {
		log.Warnf("Unable to lookup user by user id: %s, error: %v", userID, err)
		return nil, err
	}

	return s.repo.GetCompaniesByUserManagerWithInvites(ctx, userID, userModel)
}

// GetCompanyInviteRequests returns a list of company invites when provided the company ID
func (s service) GetCompanyInviteRequests(ctx context.Context, companyID string, status *string) ([]models.CompanyInviteUser, error) {
	companyInvites, err := s.repo.GetCompanyInviteRequests(ctx, companyID, status)
	if err != nil {
		return nil, err
	}

	var users []models.CompanyInviteUser
	for _, invite := range companyInvites {

		dbUserModel, err := s.userDynamoRepo.GetUser(invite.UserID)
		if err != nil {
			log.Warnf("Error fetching user with userID: %s, error: %v", invite.UserID, err)
			continue
		}

		// Default status is pending if there's a record but no status
		if invite.Status == "" {
			invite.Status = StatusPending
		}

		users = append(users, models.CompanyInviteUser{
			InviteID:  invite.CompanyInviteID,
			UserName:  dbUserModel.UserName,
			UserEmail: dbUserModel.LFEmail,
			UserLFID:  dbUserModel.LFUsername,
			Status:    invite.Status,
		})
	}

	return users, nil
}

// GetCompanyUserInviteRequests returns a list of company invites when provided the company ID
func (s service) GetCompanyUserInviteRequests(ctx context.Context, companyID string, userID string) (*models.CompanyInviteUser, error) {
	invite, err := s.repo.GetCompanyUserInviteRequests(ctx, companyID, userID)
	if err != nil {
		return nil, err
	}

	if invite == nil {
		return nil, nil
	}

	//var users []models.CompanyInviteUser

	dbUserModel, err := s.userDynamoRepo.GetUser(invite.UserID)
	if err != nil {
		log.Warnf("Error fetching company invite user with company id: %s and user id: %s, error: %v",
			companyID, userID, err)
		return nil, err
	}

	// Default status is pending if there's a record but no status
	if invite.Status == "" {
		invite.Status = StatusPending
	}

	// Let's do a company lookup so we can grab the company name
	company, err := s.repo.GetCompany(ctx, companyID)
	if err != nil {
		log.Warnf("Error fetching company with company id: %s, error: %v", companyID, err)
		return nil, err
	}

	return &models.CompanyInviteUser{
		InviteID:    invite.CompanyInviteID,
		UserName:    dbUserModel.UserName,
		UserEmail:   dbUserModel.LFEmail,
		UserLFID:    dbUserModel.LFUsername,
		Status:      invite.Status,
		CompanyName: company.CompanyName,
	}, nil
}

// AddPendingCompanyInviteRequest adds a new company invite request
func (s service) AddPendingCompanyInviteRequest(ctx context.Context, companyID string, userID string) (*InviteModel, error) {
	f := logrus.Fields{
		"functionName":   "company.service.AddPendingCompanyInviteRequest",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"userID":         userID,
	}

	log.WithFields(f).Debug("Fetching company by company ID")
	companyModel, companyErr := s.GetCompany(ctx, companyID)
	if companyErr != nil {
		log.WithFields(f).WithError(companyErr).Warn("AddPendingCompanyInviteRequest - unable to locate company model by ID")
		return nil, companyErr
	}

	log.WithFields(f).Debug("Fetching user by user ID")
	userModel, userErr := s.userDynamoRepo.GetUser(userID)
	if userErr != nil {
		log.WithFields(f).WithError(userErr).Warn("unable to locate user model by ID")
		return nil, userErr
	}

	log.WithFields(f).Debug("Adding pending company invite request")
	newInvite, err := s.repo.AddPendingCompanyInviteRequest(ctx, companyID, userModel)
	if err != nil {
		log.WithFields(f).WithError(userErr).Warn("problem adding pending company invite request")
		return nil, err
	}

	// Need to determine which email...
	var requesterEmail = ""
	if userModel.LFEmail != "" {
		requesterEmail = userModel.LFEmail
	}

	// If no LF Email try to grab the first other email in their email list
	if userModel.LFEmail == "" && userModel.UserEmails != nil {
		requesterEmail = userModel.UserEmails[0]
	}

	// Send the email to each company manager
	for _, companyManagerLFID := range companyModel.CompanyACL {
		companyManagerName, companyManagerEmail, err := s.getPreferredNameAndEmail(ctx, companyManagerLFID)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("unable to lookup company manager's name and email using LFID: %s - unable to send email, error: %+v",
				companyManagerLFID, err)
			continue
		}

		// Send an email to this company manager
		s.sendRequestAccessEmail(ctx, companyModel, userModel.UserName, requesterEmail, companyManagerName, companyManagerEmail)
	}

	return &InviteModel{
		CompanyInviteID:    newInvite.CompanyInviteID,
		RequestedCompanyID: newInvite.RequestedCompanyID,
		CompanyName:        companyModel.CompanyName,
		UserName:           userModel.UserName,
		UserEmail:          userModel.LFEmail,
		UserID:             newInvite.UserID,
		Status:             newInvite.Status,
		Created:            newInvite.Created,
		Updated:            newInvite.Updated,
	}, nil
}

// ApproveCompanyAccessRequest approve access request service method
func (s service) ApproveCompanyAccessRequest(ctx context.Context, companyInviteID string) (*InviteModel, error) {
	f := logrus.Fields{
		"functionName":    "company.service.ApproveCompanyAccessRequest",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"companyInviteID": companyInviteID,
	}

	log.WithFields(f).Debug("Approve company access request")
	err := s.repo.ApproveCompanyAccessRequest(ctx, companyInviteID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("Error approving company access request")
		return nil, err
	}

	inviteModel, inviteErr := s.repo.GetCompanyInviteRequest(ctx, companyInviteID)
	if inviteErr != nil || inviteModel == nil {
		log.WithFields(f).Warnf("unable to locate company invite: %s, error: %+v",
			companyInviteID, inviteErr)
		return nil, inviteErr
	}

	companyModel, companyErr := s.GetCompany(ctx, inviteModel.RequestedCompanyID)
	if companyErr != nil {
		log.WithFields(f).WithError(companyErr).Warnf("unable to locate company model by ID: %s, error: %+v",
			inviteModel.RequestedCompanyID, companyErr)
		return nil, companyErr
	}

	userModel, userErr := s.userDynamoRepo.GetUser(inviteModel.UserID)
	if userErr != nil {
		log.WithFields(f).WithError(userErr).Warnf("unable to locate user model by ID: %s, error: %+v",
			inviteModel.UserID, userErr)
		return nil, userErr
	}

	updatedUserModel, userUpdateErr := s.userDynamoRepo.SetCompanyID(userModel.UserID, companyModel.CompanyID)
	if userUpdateErr != nil {
		log.WithFields(f).WithError(userUpdateErr).Warnf("unable to update user model by ID: %s, with company ID: %s error: %+v",
			inviteModel.UserID, companyModel.CompanyID, userUpdateErr)
		return nil, userUpdateErr
	}

	// update the company ACL
	aclErr := s.AddUserToCompanyAccessList(ctx, inviteModel.RequestedCompanyID, updatedUserModel.LFUsername)
	if aclErr != nil {
		log.Warnf("ApproveCompanyAccessRequest - unable to add user to Company ACL, company ID: %s, user LFID: %s, error: %+v",
			inviteModel.RequestedCompanyID, updatedUserModel.UserName, err)
		return nil, aclErr
	}

	// Need to determine which email...
	var whichEmail = ""
	if updatedUserModel.LFEmail != "" {
		whichEmail = userModel.LFEmail
	}

	// If no LF Email try to grab the first other email in their email list
	if updatedUserModel.LFEmail == "" && updatedUserModel.UserEmails != nil {
		whichEmail = updatedUserModel.UserEmails[0]
	}

	s.sendRequestApprovedEmailToRecipient(ctx, companyModel, updatedUserModel.UserName, whichEmail)

	return &InviteModel{
		CompanyInviteID:    inviteModel.CompanyInviteID,
		RequestedCompanyID: inviteModel.RequestedCompanyID,
		CompanyName:        companyModel.CompanyName,
		UserName:           updatedUserModel.UserName,
		UserEmail:          updatedUserModel.LFEmail,
		UserID:             inviteModel.UserID,
		Status:             inviteModel.Status,
		Created:            inviteModel.Created,
		Updated:            inviteModel.Updated,
	}, nil
}

// RejectCompanyAccessRequest approve access request service method
func (s service) RejectCompanyAccessRequest(ctx context.Context, companyInviteID string) (*InviteModel, error) {
	f := logrus.Fields{
		"functionName":    "company.service.RejectCompanyAccessRequest",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"companyInviteID": companyInviteID,
	}

	log.WithFields(f).Debug("Rejecting company access request")
	err := s.repo.RejectCompanyAccessRequest(ctx, companyInviteID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("Error rejecting company access request")
		return nil, err
	}

	inviteModel, inviteErr := s.repo.GetCompanyInviteRequest(ctx, companyInviteID)
	if inviteErr != nil || inviteModel == nil {
		log.Warnf("RejectCompanyAccessRequest - unable to locate company invite: %s, error: %+v",
			companyInviteID, inviteErr)
		return nil, inviteErr
	}

	companyModel, companyErr := s.GetCompany(ctx, inviteModel.RequestedCompanyID)
	if companyErr != nil {
		log.Warnf("RejectCompanyAccessRequest - unable to locate company model by ID: %s, error: %+v",
			inviteModel.RequestedCompanyID, companyErr)
		return nil, companyErr
	}

	userModel, userErr := s.userDynamoRepo.GetUser(inviteModel.UserID)
	if userErr != nil {
		log.Warnf("RejectCompanyAccessRequest - unable to locate user model by ID: %s, error: %+v",
			inviteModel.UserID, userErr)
		return nil, userErr
	}

	// Need to determine which email...
	var whichEmail = ""
	if userModel.LFEmail != "" {
		whichEmail = userModel.LFEmail
	}

	// If no LF Email try to grab the first other email in their email list
	if userModel.LFEmail == "" && userModel.UserEmails != nil {
		whichEmail = userModel.UserEmails[0]
	}

	s.sendRequestRejectedEmailToRecipient(ctx, companyModel, userModel.UserName, whichEmail)

	return &InviteModel{
		CompanyInviteID:    inviteModel.CompanyInviteID,
		RequestedCompanyID: inviteModel.RequestedCompanyID,
		CompanyName:        companyModel.CompanyName,
		UserName:           userModel.UserName,
		UserEmail:          userModel.LFEmail,
		UserID:             inviteModel.UserID,
		Status:             inviteModel.Status,
		Created:            inviteModel.Created,
		Updated:            inviteModel.Updated,
	}, nil
}

// AddUserToCompanyAccessList adds a user to the specified company
func (s service) AddUserToCompanyAccessList(ctx context.Context, companyID, lfid string) error {
	f := logrus.Fields{
		"functionName":   "company.service.AddUserToCompanyAccessList",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"lfid":           lfid,
	}
	// call the get company function
	company, err := s.repo.GetCompany(ctx, companyID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Error retrieving company by company ID: %s", companyID)
		return err
	}

	// perform ACL check
	// check if user already exists in the company acl
	for _, acl := range company.CompanyACL {
		if acl == lfid {
			log.WithFields(f).Warnf(fmt.Sprintf("User %s has already been added to the company acl - will not update ACL", lfid))
			return nil
		}
	}
	// add user to string set
	company.CompanyACL = append(company.CompanyACL, lfid)

	err = s.repo.UpdateCompanyAccessList(ctx, companyID, company.CompanyACL)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Error updating company access list with company ID: %s, company ACL: %v, error: %v", companyID, company.CompanyACL, err)
		return err
	}

	return nil
}

// IsCCLAEnabledForCompany determines if the specified company has CCLA enabled
func (s service) IsCCLAEnabledForCompany(ctx context.Context, companyID string) (bool, error) {
	return s.repo.IsCCLAEnabledForCompany(ctx, companyID)
}

// sendRequestAccessEmail sends the request access email
func (s service) sendRequestAccessEmail(ctx context.Context, companyModel *models.Company, requesterName, requesterEmail, recipientName, recipientAddress string) {
	f := logrus.Fields{
		"functionName":     "company.service.sendRequestAccessEmail",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"requesterName":    requesterName,
		"requesterEmail":   requesterEmail,
		"recipientName":    recipientName,
		"recipientAddress": recipientAddress,
	}
	companyName := companyModel.CompanyName

	requestedUserInfo := fmt.Sprintf("<ul><li>%s (%s)</li></ul>", requesterName, requesterEmail)

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: New Company Manager Access Request for %s", companyName)
	recipients := []string{recipientAddress}
	body := fmt.Sprintf(`
<p>Hello %s,</p>
<p>This is a notification email from EasyCLA regarding the company %s.</p>
<p>The following user has requested to join %s as a Company Manager. 
By approving this request the user could view and apply for CLA Manager
status on projects associated with your company. </p>
%s
<p>To get started, please log into the <a href="%s" target="_blank">EasyCLA Corporate Console</a>, and select your
company.You can choose to accept or deny the request.
</p>
%s
%s`,
		recipientName, companyName, companyName, requestedUserInfo, utils.GetCorporateURL(false),
		utils.GetEmailHelpContent(false), utils.GetEmailSignOffContent())

	err := utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.WithFields(f).Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

// sendRequestApprovedEmailToRecipient generates and sends an email to the specified recipient
func (s service) sendRequestApprovedEmailToRecipient(ctx context.Context, companyModel *models.Company, recipientName, recipientAddress string) {
	f := logrus.Fields{
		"functionName":     "company.service.sendRequestApprovedEmailToRecipient",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"recipientName":    recipientName,
		"recipientAddress": recipientAddress,
	}
	companyName := companyModel.CompanyName

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Company Manager Access Approved for %s", companyName)
	recipients := []string{recipientAddress}
	body := fmt.Sprintf(`
<p>Hello %s,</p>
<p>This is a notification email from EasyCLA regarding the company %s.</p>
<p>You have now been approved as a Company Manager for %s.
This means that you can now view and apply for CLA Manager status on
projects associated with your company.
</p>
<p>To get started, please log into the <a href="%s" target="_blank">EasyCLA Corporate Console</a>, and select your
company. From there you will be able to view the list of projects which have EasyCLA configured and apply for CLA
Manager status.
</p>
%s
%s`,
		recipientName, companyName, companyName, utils.GetCorporateURL(false),
		utils.GetEmailHelpContent(false), utils.GetEmailSignOffContent())

	err := utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.WithFields(f).Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

// sendRequestRejectedEmailToRecipient generates and sends an email to the specified recipient
func (s service) sendRequestRejectedEmailToRecipient(ctx context.Context, companyModel *models.Company, recipientName, recipientAddress string) {
	f := logrus.Fields{
		"functionName":     "company.service.sendRequestRejectedEmailToRecipient",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"recipientName":    recipientName,
		"recipientAddress": recipientAddress,
	}

	companyName := companyModel.CompanyName

	var companyManagerText = ""
	companyManagerText += "<ul>"

	// Build a fancy text string with CLA Manager name <email> as an HTML unordered list
	for _, companyAdminLFID := range companyModel.CompanyACL {

		userModel, userErr := s.userDynamoRepo.GetUserAndProfilesByLFID(companyAdminLFID)
		if userErr != nil {
			log.WithFields(f).Warnf("unable to locate user model by ID: %s, error: %+v",
				companyAdminLFID, userErr)
		}

		// Need to determine which email...
		var whichEmail = ""
		if userModel.LFEmail != "" {
			whichEmail = userModel.LFEmail
		}

		// If no LF Email try to grab the first other email in their email list
		if userModel.LFEmail == "" && userModel.Emails != nil {
			whichEmail = userModel.Emails[0]
		}

		if whichEmail == "" {
			log.WithFields(f).Warnf("unable to send email to manager: %+v - no email on file...", userModel)
		} else {
			companyManagerText += fmt.Sprintf("<li>%s <%s></li>", userModel.Name, whichEmail)
		}
	}

	companyManagerText += "</ul>"

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: CLA Manager Access Denied for %s", companyName)
	recipients := []string{recipientAddress}
	body := fmt.Sprintf(`
<p>Hello %s,</p>
<p>This is a notification email from EasyCLA regarding the company %s.</p>
<p>Your request to become a Company Manager was denied by one of the existing Company Managers.
If you have further questions about this denial, please contact one of the existing managers from
%s:</p>
%s
%s
%s`,
		recipientName, companyName, companyName, companyManagerText,
		utils.GetEmailHelpContent(false), utils.GetEmailSignOffContent())

	err := utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.WithFields(f).Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

// getPreferredNameAndEmail when given the user LFID, this routine returns the user's name and preferred email
func (s service) getPreferredNameAndEmail(ctx context.Context, lfid string) (string, string, error) {
	f := logrus.Fields{
		"functionName":   "company.service.getPreferredNameAndEmail",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"lfid":           lfid,
	}
	log.WithFields(f).Debug("Searching user by LF User ID...")
	userModel, userErr := s.userService.GetUserByLFUserName(lfid)
	if userErr != nil {
		log.WithFields(f).WithError(userErr).Warnf("getPreferredNameAndEmail - unable to locate user model by ID: %s, error: %+v",
			lfid, userErr)
		return "", "", userErr
	}

	var updatedUserModel models.User

	if userModel == nil {
		userModels, userErr := s.userService.SearchUsers("user_name", lfid, true)
		if userErr != nil {
			log.WithFields(f).WithError(userErr).Warnf("SearchUsers - unable to locate user model by ID: %s, error: %+v",
				lfid, userErr)
			return "", "", userErr
		}
		if len(userModels.Users) > 0 {
			for _, user := range userModels.Users {
				if user.Username == lfid {
					updatedUserModel = user
				}
			}
		}
	}
	userModel = &updatedUserModel

	userName := userModel.Username
	// If no user name specified - then use the user's LF user name I guess
	if userName == "" {
		userName = userModel.LfUsername
	}

	userEmail := userModel.LfEmail
	if userEmail == "" && userModel.Emails != nil && len(userModel.Emails) > 0 {
		userEmail = strfmt.Email(userModel.Emails[0])
	}

	return userName, userEmail.String(), nil
}

func (s service) GetCompanyByExternalID(ctx context.Context, companySFID string) (*models.Company, error) {
	f := logrus.Fields{
		"functionName":   "company.service.GetCompanyByExternalID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companySFID":    companySFID,
	}
	log.WithFields(f).Debug("Searching company by external ID...")
	comp, err := s.repo.GetCompanyByExternalID(ctx, companySFID)
	if err == nil {
		log.WithFields(f).Debugf("Loaded and returning company: %+v...", comp)
		return comp, nil
	}

	if _, ok := err.(*utils.CompanyNotFound); ok {
		comp, err = s.CreateOrgFromExternalID(ctx, "", companySFID)
		if err != nil {
			return comp, err
		}
		return comp, nil
	}
	return nil, err
}

func (s service) GetCompaniesByExternalID(ctx context.Context, companySFID string, includeChildCompanies bool) ([]*models.Company, error) {
	f := logrus.Fields{
		"functionName":          "company.service.GetCompaniesByExternalID",
		utils.XREQUESTID:        ctx.Value(utils.XREQUESTID),
		"companySFID":           companySFID,
		"includeChildCompanies": includeChildCompanies,
	}

	log.WithFields(f).Debug("Searching companies by external ID...")
	comp, err := s.repo.GetCompaniesByExternalID(ctx, companySFID, includeChildCompanies)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to locate matching records by companySFID")
		return nil, err
	}

	return comp, nil
}

func (s service) GetCompanyBySigningEntityName(ctx context.Context, signingEntityName, companySFID string) (*models.Company, error) {
	f := logrus.Fields{
		"functionName":      "company.service.GetCompanyBySigningEntityName",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"signingEntityName": signingEntityName,
		"companySFID":       companySFID,
	}
	log.WithFields(f).Debug("Searching company by signing entity name...")
	comp, err := s.repo.GetCompanyBySigningEntityName(ctx, signingEntityName)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem searching organizations by signing entity name")
		return nil, err
	}

	return comp, nil
}

func (s service) SearchOrganizationByName(ctx context.Context, orgName string, websiteName string, includeSigningEntityName bool, filter string) (*models.OrgList, error) {
	f := logrus.Fields{
		"functionName":             "company.service.SearchOrganizationByName",
		utils.XREQUESTID:           ctx.Value(utils.XREQUESTID),
		"orgName":                  orgName,
		"websiteName":              websiteName,
		"includeSigningEntityName": includeSigningEntityName,
		"filter":                   filter,
	}

	osc := organization_service.GetClient()
	log.WithFields(f).Debug("Searching organizations by name and website...")
	orgs, err := osc.SearchOrganization(ctx, orgName, websiteName, filter)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem searching organizations by name and website")
		return nil, err
	}

	resultsChannel := make(chan *models.Org, len(orgs))
	var wg sync.WaitGroup

	wg.Add(len(orgs))
	for _, org := range orgs {
		go func(org *orgServiceModels.Organization) {
			defer wg.Done()
			// get company by external ID
			cclaEnabled := false
			company, err := s.repo.GetCompanyByExternalID(ctx, org.ID)
			if err != nil {
				if _, ok := err.(*utils.CompanyNotFound); ok {
					// company not found, so ccla is not enabled
					log.WithFields(f).WithError(err).Warnf("company not found by name: %s", org.Name)
					cclaEnabled = false
				} else {
					log.WithFields(f).WithError(err).Warnf("problem searching company by external ID: %s", org.ID)
					return
				}
			}

			if company != nil {
				cclaEnabled, err = s.IsCCLAEnabledForCompany(ctx, company.CompanyID)
				if err != nil {
					log.WithFields(f).WithError(err).Warnf("problem checking if ccla is enabled for company: %s", company.CompanyID)
					return
				}
			}

			if includeSigningEntityName {

				if len(org.SigningEntityName) > 0 {
					var signingEntityNames []string
					if len(org.SigningEntityName) > 0 {
						signingEntityNames = utils.TrimSpaceFromItems(org.SigningEntityName)
						for _, signingEntityName := range signingEntityNames {
							// Auto-create the internal record, if needed
							_, err = s.CreateOrgFromExternalID(ctx, signingEntityName, org.ID)
							if err != nil {
								log.WithFields(f).WithError(err).Warnf("Unable to create organization from external ID: %s using signing entity name: %s", org.ID, signingEntityName)
							}
						}
						resultsChannel <- &models.Org{
							OrganizationID:      org.ID,
							OrganizationName:    org.Name,
							SigningEntityNames:  signingEntityNames,
							OrganizationWebsite: org.Link,
							CclaEnabled:         &cclaEnabled,
						}

					}
				}
			} else {
				resultsChannel <- &models.Org{
					OrganizationID:      org.ID,
					OrganizationName:    org.Name,
					OrganizationWebsite: org.Link,
					CclaEnabled:         &cclaEnabled,
				}
			}
		}(org)
	}

	go func() {
		wg.Wait()
		close(resultsChannel)
	}()

	result := &models.OrgList{
		List: make([]*models.Org, 0),
	}

	for orgResult := range resultsChannel {
		result.List = append(result.List, orgResult)
	}

	// Sort the results
	sort.Slice(result.List, func(i, j int) bool {
		switch strings.Compare(strings.ToLower(result.List[i].OrganizationName), strings.ToLower(result.List[j].OrganizationName)) {
		case -1:
			return true
		case 1:
			return false
		}
		return strings.ToLower(result.List[i].OrganizationWebsite) > strings.ToLower(result.List[j].OrganizationWebsite)
	})

	return result, nil
}

// CreateOrgFromExternalID creates a new EasyCLA company from the external SF Organization ID
func (s service) CreateOrgFromExternalID(ctx context.Context, signingEntityName, companySFID string) (*models.Company, error) {
	f := logrus.Fields{
		"functionName":      "company.service.CreateOrgFromExternalID",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"companySFID":       companySFID,
		"signingEntityName": signingEntityName,
	}

	var companyModel *models.Company
	var lookupErr error

	// Lookup the company in our database...does it exist?
	companyModel, lookupErr = s.GetCompanyBySigningEntityName(ctx, signingEntityName, companySFID)
	if lookupErr != nil {
		log.WithFields(f).WithError(lookupErr).Debug("problem locating internal company record by signing entity name and SFID - must not exist yet")
	}

	// Already exists - no need to create in our own database
	if companyModel != nil {
		return companyModel, nil
	}

	osc := organization_service.GetClient()
	log.WithFields(f).Debugf("Searching organization by company SFID in the organization service...")
	org, err := osc.GetOrganization(ctx, companySFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("getting organization details failed")
		return nil, err
	}

	// Add some fields to the logger
	f["companyName"] = org.Name
	f["companyStatus"] = org.Status

	// Query the platform user service to locate the company admin
	log.WithFields(f).Debugf("getting company-admin information...")
	companyAdmin, err := getCompanyAdmin(ctx, companySFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to load company admin information for company: %s", companySFID)
	}

	var claUser *models.User
	if companyAdmin != nil {
		f["company-admin"] = companyAdmin
		log.WithFields(f).Debugf("loaded company admin: %+v", companyAdmin)

		log.WithFields(f).Debugf("getting user information from cla")
		claUser, err = s.userService.GetUserByLFUserName(companyAdmin.LfUsername)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("problem loading user by username: %s", companyAdmin.LfUsername)
			return nil, err
		}

		if claUser == nil {
			// create cla-user
			log.WithFields(f).Debugf("cla user not found. creating cla user.")
			claUser, err = s.userService.CreateUser(companyAdmin, nil)
			if err != nil {
				log.WithFields(f).WithError(err).Warn("creating cla user failed")
				return nil, err
			}
		}
	} else {
		log.WithFields(f).Debug("unable to load company admin from companySFID - admin not found")
	}

	additionalNote := ""
	if signingEntityName == "" {
		additionalNote = fmt.Sprintf("signing entity name not set - using organization name: %s", org.Name)
		log.WithFields(f).Debugf(additionalNote)
		signingEntityName = org.Name
	}

	_, now := utils.CurrentTime()
	newComp := &models.Company{
		CompanyExternalID: org.ID,
		CompanyName:       org.Name,
		SigningEntityName: signingEntityName,
		Note:              fmt.Sprintf("%s - Created based on SF Organization Service record - %s", now, additionalNote),
	}
	if companyAdmin != nil {
		newComp.CompanyACL = []string{companyAdmin.LfUsername}
	}
	if claUser != nil {
		newComp.CompanyManagerID = claUser.UserID
	}

	f["company"] = newComp
	log.WithFields(f).Debugf("creating cla company record")
	// create company
	comp, err := s.repo.CreateCompany(ctx, newComp)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("creating cla company failed")
		return nil, err
	}

	log.WithFields(f).Debugf("Created company %s with Signing Entity Name: %s with ID: %s",
		comp.CompanyName, signingEntityName, comp.CompanyID)
	return comp, nil
}

// getCompanyAdmin is helper function which queries org-service to get first company-admin
func getCompanyAdmin(ctx context.Context, companySFID string) (*models.User, error) {
	f := logrus.Fields{
		"functionName":   "company.service.getCompanyAdmin",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companySFID":    companySFID,
	}
	osc := organization_service.GetClient()
	result, err := osc.ListOrgUserAdminScopes(ctx, companySFID, nil)
	if err != nil {
		if _, ok := err.(*organizations.ListOrgUsrAdminScopesNotFound); !ok {
			log.WithFields(f).Warnf("getting company-admin failed. error = %s", err.Error())
			return nil, err
		}
	}
	if result != nil {
		for _, usc := range result.Userroles {
			for _, rs := range usc.RoleScopes {
				if rs.RoleName == "company-admin" {
					companyAdmin := &models.User{
						LfEmail:        strfmt.Email(usc.Contact.EmailAddress),
						LfUsername:     usc.Contact.Username,
						UserExternalID: usc.Contact.ID,
						Username:       usc.Contact.Name,
					}
					log.WithFields(f).WithField("company-admin", companyAdmin).Debug("company-admin found")
					return companyAdmin, nil
				}
			}
		}
	}
	log.WithFields(f).Warnf("no company-admin found")
	return nil, &utils.CompanyAdminNotFound{
		CompanySFID: companySFID,
		Err:         nil,
	}
}
