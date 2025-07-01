// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"fmt"
	"strings"

	v2AcsService "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"

	"github.com/linuxfoundation/easycla/cla-backend-go/emails"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// EmailToCLAManagerModel data model for sending emails to CLA Managers
type EmailToCLAManagerModel struct {
	Contributor         *v1Models.User
	CLAManagerName      string
	CLAManagerEmail     string
	CompanyName         string
	CLAGroupName        string
	CorporateConsoleURL string
}

// ToCLAManagerDesigneeCorporateModel data model for sending emails
type ToCLAManagerDesigneeCorporateModel struct {
	companyName   string
	projectSFID   string
	projectName   string
	designeeEmail string
	designeeName  string
	senderEmail   string
	senderName    string
}

// ToCLAManagerDesigneeModel data model for sending emails
type ToCLAManagerDesigneeModel struct {
	designeeName     string
	designeeEmail    string
	companyName      string
	projectNames     []string
	projectSFIDs     []string
	contributorModel emails.Contributor
}

// DesigneeEmailToUserWithNoLFIDModel data model for sending emails
type DesigneeEmailToUserWithNoLFIDModel struct {
	userWithNoLFIDName  string
	userWithNoLFIDEmail string
	contributorModel    emails.Contributor
	projectNames        []string
	projectSFIDs        []string
	foundationSFID      string
	role                string
	companyName         string
	organizationID      string
}

// EmailToUserWithNoLFIDModel data model for sending emails
type EmailToUserWithNoLFIDModel struct {
	projectName         string
	requesterUsername   string
	requesterEmail      string
	userWithNoLFIDName  string
	userWithNoLFIDEmail string
	organizationID      string
	companyName         string
	projectID           string
	role                string
}

// EmailToOrgAdminModel data model for sending emails
type EmailToOrgAdminModel struct {
	adminEmail  string
	adminName   string
	companyName string
	projectName string
	projectSFID string
	senderName  string
	senderEmail string
}

// ContributorEmailToOrgAdminModel data model for sending emails
type ContributorEmailToOrgAdminModel struct {
	adminEmail   string
	adminName    string
	companyName  string
	projectSFIDs []string
	contributor  *v1Models.User
	userDetails  string
}

// SendEmailToCLAManager handles sending an email to the specified CLA Manager
func (s *service) SendEmailToCLAManager(ctx context.Context, input *EmailToCLAManagerModel, projectSFIDs []string) {
	f := logrus.Fields{
		"functionName":              "cla_manager.service.SendEmailToCLAManager",
		utils.XREQUESTID:            ctx.Value(utils.XREQUESTID),
		"contributorUsername":       input.Contributor.Username,
		"contributorLFUsername":     input.Contributor.LfUsername,
		"contributorGitHubID":       input.Contributor.GithubID,
		"contributorGitHubUsername": input.Contributor.GithubUsername,
		"contributorLFEmail":        input.Contributor.LfEmail,
		"contributorEmails":         strings.Join(input.Contributor.Emails, ","),
		"claManagerName":            input.CLAManagerName,
		"claManagerEmail":           input.CLAManagerEmail,
		"companyName":               input.CompanyName,
		"claGroupName":              input.CLAGroupName,
	}

	subject := fmt.Sprintf("EasyCLA: Approval Request for contributor: %s", getBestUserName(input.Contributor))
	recipients := []string{input.CLAManagerEmail}
	body, err := emails.RenderV2ContributorApprovalRequestTemplate(s.emailTemplateService, projectSFIDs, emails.V2ContributorApprovalRequestTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			RecipientName: input.CLAManagerName,
			CompanyName:   input.CompanyName,
		},
		SigningEntityName: input.CompanyName,
		UserDetails:       getFormattedUserDetails(input.Contributor),
	})
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("rendering email template: %s", emails.V2ContributorApprovalRequestTemplateName)
		return
	}

	log.WithFields(f).Debugf("sending email with subject: %s to recipients: %+v...", subject, recipients)
	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.WithFields(f).Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

// SendEmailToOrgAdmin sends an email to the organization admin
func (s *service) SendEmailToOrgAdmin(ctx context.Context, input EmailToOrgAdminModel) {
	f := logrus.Fields{
		"functionName":   "cla_manager.service.SendEmailToOrgAdmin",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"adminEmail":     input.adminEmail,
		"adminName":      input.adminName,
		"companyName":    input.companyName,
		"projectName":    input.projectName,
		"projectSFID":    input.projectSFID,
		"senderName":     input.senderName,
		"senderEmail":    input.senderEmail,
	}

	subject := fmt.Sprintf("EasyCLA:  Invitation to Sign the %s Corporate CLA ", input.companyName)
	recipients := []string{input.adminEmail}
	body, err := emails.RenderV2OrgAdminTemplate(s.emailTemplateService, input.projectSFID, emails.V2OrgAdminTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			RecipientName: input.adminName,
			CompanyName:   input.companyName,
		},
		SenderName:  input.senderName,
		SenderEmail: input.senderEmail,
	})
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("rendering email template : %s failed : %v", emails.V2OrgAdminTemplateName, err)
		return
	}
	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.WithFields(f).Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

func (s *service) ContributorEmailToOrgAdmin(ctx context.Context, input ContributorEmailToOrgAdminModel) {
	f := logrus.Fields{
		"functionName":              "cla_manager.service.SendEmailToOrgAdmin",
		utils.XREQUESTID:            ctx.Value(utils.XREQUESTID),
		"adminEmail":                input.adminEmail,
		"adminName":                 input.adminName,
		"companyName":               input.companyName,
		"contributorName":           input.contributor.Username,
		"contributorGitHubID":       input.contributor.GithubID,
		"contributorGitHubUsername": input.contributor.GithubUsername,
		"contributorLFUsername":     input.contributor.LfUsername,
		"contributorLFEmail":        input.contributor.LfEmail,
		"contributorEmails":         strings.Join(input.contributor.Emails, ","),
	}

	subject := fmt.Sprintf("EasyCLA:  Invitation to Sign the %s Corporate CLA and add to approved list %s ", input.companyName, getBestUserName(input.contributor))
	recipients := []string{input.adminEmail}
	body, err := emails.RenderV2ContributorToOrgAdminTemplate(s.emailTemplateService, input.projectSFIDs, emails.V2ContributorToOrgAdminTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			RecipientName: input.adminName,
			CompanyName:   input.companyName,
		},
		UserDetails: input.userDetails,
	})
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("rendering template : %s failed : %v", emails.V2ContributorToOrgAdminTemplateName, err)
		return
	}
	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.WithFields(f).Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

func (s *service) SendEmailToCLAManagerDesigneeCorporate(ctx context.Context, input ToCLAManagerDesigneeCorporateModel) {
	f := logrus.Fields{
		"functionName":   "cla_manager.service.SendEmailToCLAManagerDesigneeCorporate",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyName":    input.companyName,
		"projectName":    input.projectName,
		"designeeEmail":  input.designeeEmail,
		"designeeName":   input.designeeName,
		"senderEmail":    input.senderEmail,
		"senderName":     input.senderName,
	}

	subject := fmt.Sprintf("EasyCLA:  Invitation to Sign the %s Corporate CLA ", input.companyName)
	recipients := []string{input.designeeEmail}
	body, err := emails.RenderV2CLAManagerDesigneeCorporateTemplate(s.emailTemplateService, input.projectSFID, emails.V2CLAManagerDesigneeCorporateTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			RecipientName: input.designeeName,
			CompanyName:   input.companyName,
		},
		SenderName:  input.senderName,
		SenderEmail: input.senderEmail,
	})
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("rendering template : %s : failed: %v", emails.V2CLAManagerDesigneeCorporateTemplateName, err)
		return
	}
	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.WithFields(f).Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

func (s *service) SendEmailToCLAManagerDesignee(ctx context.Context, input ToCLAManagerDesigneeModel) {
	f := logrus.Fields{
		"functionName":     "cla_manager.service.SendEmailToCLAManagerDesignee",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"companyName":      input.companyName,
		"projectNames":     strings.Join(input.projectNames, ","),
		"designeeEmail":    input.designeeEmail,
		"designeeName":     input.designeeName,
		"contributorEmail": input.contributorModel.Email,
		"contributorName":  input.contributorModel.Username,
	}

	subject := fmt.Sprintf("EasyCLA:  Invitation to Sign the %s Corporate CLA and add to approved list %s ",
		input.companyName, input.contributorModel.Email)
	recipients := []string{input.designeeEmail}
	body, err := emails.RenderV2ToCLAManagerDesigneeTemplate(s.emailTemplateService, input.projectSFIDs,
		emails.V2ToCLAManagerDesigneeTemplateParams{
			CommonEmailParams: emails.CommonEmailParams{
				RecipientName: input.designeeName,
				CompanyName:   input.companyName,
			},
			Contributor: input.contributorModel,
		}, emails.V2ToCLAManagerDesigneeTemplate, emails.V2ToCLAManagerDesigneeTemplateName)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("rendering template : %s failed : %v", emails.V2ToCLAManagerDesigneeTemplateName, err)
		return
	}
	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.WithFields(f).Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

func (s *service) SendDesigneeEmailToUserWithNoLFID(ctx context.Context, input DesigneeEmailToUserWithNoLFIDModel) error {
	f := logrus.Fields{
		"functionName":        "cla_manager.service.SendDesigneeEmailToUserWithNoLFID",
		utils.XREQUESTID:      ctx.Value(utils.XREQUESTID),
		"userWithNoLFIDName":  input.userWithNoLFIDName,
		"userWithNoLFIDEmail": input.userWithNoLFIDEmail,
		"organizationID":      input.organizationID,
		"projectNames":        strings.Join(input.projectNames, ","),
		"role":                input.role,
		"requesterUsername":   input.contributorModel.Username,
		"requesterEmail":      input.contributorModel.Email,
	}

	subject := "EasyCLA: Invitation to create LF Login and complete process of becoming CLA Manager"

	body, err := emails.RenderV2ToCLAManagerDesigneeTemplate(s.emailTemplateService, input.projectSFIDs,
		emails.V2ToCLAManagerDesigneeTemplateParams{
			CommonEmailParams: emails.CommonEmailParams{
				RecipientName: input.userWithNoLFIDName,
				CompanyName:   input.companyName,
			},
			Contributor: input.contributorModel,
		}, emails.V2DesigneeToUserWithNoLFIDTemplate, emails.V2DesigneeToUserWithNoLFIDTemplateName)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("rendering template : %s failed : %v", emails.V2DesigneeToUserWithNoLFIDTemplateName, err)
		return err
	}

	acsClient := v2AcsService.GetClient()
	log.WithFields(f).Debug("sending user invite request...")

	// Parse the provided user's name
	userFirstName, userLastName := utils.GetFirstAndLastName(input.userWithNoLFIDName)

	return acsClient.SendUserInvite(ctx, &v2AcsService.SendUserInviteInput{
		InviteUserFirstName: userFirstName,
		InviteUserLastName:  userLastName,
		InviteUserEmail:     input.userWithNoLFIDEmail,
		RoleName:            input.role,
		Scope:               utils.ProjectOrgScope,
		ProjectSFID:         input.foundationSFID,
		OrganizationSFID:    input.organizationID,
		InviteType:          "userinvite",
		Subject:             subject,
		EmailContent:        body,
		Automate:            false,
	})
}

// sendEmailToUserWithNoLFID helper function to send email to a given user with no LFID
func (s *service) SendEmailToUserWithNoLFID(ctx context.Context, input EmailToUserWithNoLFIDModel) error {
	f := logrus.Fields{
		"functionName":        "cla_manager.service.SendEmailToUserWithNoLFID",
		utils.XREQUESTID:      ctx.Value(utils.XREQUESTID),
		"projectName":         input.projectName,
		"requesterUsername":   input.requesterUsername,
		"requesterEmail":      input.requesterEmail,
		"userWithNoLFIDName":  input.userWithNoLFIDName,
		"userWithNoLFIDEmail": input.userWithNoLFIDEmail,
		"organizationID":      input.organizationID,
		"projectID":           input.projectID,
		"role":                input.role,
	}

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Invitation to create LF Login and complete process of becoming CLA Manager with %s role", input.role)
	body, err := emails.RenderV2CLAManagerToUserWithNoLFIDTemplate(s.emailTemplateService, input.projectID, emails.V2CLAManagerToUserWithNoLFIDTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			RecipientName: input.userWithNoLFIDName,
			CompanyName:   input.companyName,
		},
		RequesterUserName: input.requesterUsername,
		RequesterEmail:    input.requesterEmail,
	})

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("rendering email : %s failed : %v", emails.V2CLAManagerToUserWithNoLFIDTemplateName, err)
		return err
	}
	acsClient := v2AcsService.GetClient()

	// Parse the provided user's name
	userFirstName, userLastName := utils.GetFirstAndLastName(input.userWithNoLFIDName)

	log.WithFields(f).Debug("sending user invite request...")
	//return acsClient.SendUserInvite(ctx, &userWithNoLFIDEmail, role, utils.ProjectOrgScope, projectID, organizationID, "userinvite", &subject, &body, automate)
	return acsClient.SendUserInvite(ctx, &v2AcsService.SendUserInviteInput{
		InviteUserFirstName: userFirstName,
		InviteUserLastName:  userLastName,
		InviteUserEmail:     input.userWithNoLFIDEmail,
		RoleName:            input.role,
		Scope:               utils.ProjectOrgScope,
		ProjectSFID:         input.projectID,
		OrganizationSFID:    input.organizationID,
		InviteType:          "userinvite",
		Subject:             subject,
		EmailContent:        body,
		Automate:            false,
	})
}
