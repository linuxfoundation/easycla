// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"fmt"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/emails"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/project"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	v2AcsService "github.com/communitybridge/easycla/cla-backend-go/v2/acs-service"
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

// SendEmailToCLAManager handles sending an email to the specified CLA Manager
func (s *service) SendEmailToCLAManager(ctx context.Context, input *EmailToCLAManagerModel) {
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
	body, err := emails.RenderTemplate(
		utils.V2, emails.V2ContributorApprovalRequestTemplateName,
		emails.V2ContributorApprovalRequestTemplate,
		emails.V2ContributorApprovalRequestTemplateParams{
			CLAManagerTemplateParams: emails.CLAManagerTemplateParams{
				RecipientName: input.CLAGroupName,
				CompanyName:   input.CompanyName,
				CLAGroupName:  input.CLAGroupName,
			},
			SigningEntityName:     input.CompanyName,
			UserDetails:           getFormattedUserDetails(input.Contributor),
			CorporateConsoleV2URL: input.CorporateConsoleURL,
		},
	)
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
func (s *service) SendEmailToOrgAdmin(ctx context.Context, repository projects_cla_groups.Repository, projectService project.Service, adminEmail string, adminName string, companyName string, projectName, projectSFID string, senderEmail string, senderName string, corporateConsole string) {
	f := logrus.Fields{
		"functionName":        "cla_manager.service.SendEmailToOrgAdmin",
		utils.XREQUESTID:      ctx.Value(utils.XREQUESTID),
		"adminEmail":          adminEmail,
		"adminName":           adminName,
		"companyName":         companyName,
		"projectName":         projectName,
		"projectSFID":         projectSFID,
		"senderName":          senderName,
		"senderEmail":         senderEmail,
		"corporateConsoleURL": corporateConsole,
	}

	subject := fmt.Sprintf("EasyCLA:  Invitation to Sign the %s Corporate CLA ", companyName)
	recipients := []string{adminEmail}
	body, err := emails.RenderV2OrgAdminTemplate(repository, projectService, projectSFID, emails.V2OrgAdminTemplateParams{
		CLAManagerTemplateParams: emails.CLAManagerTemplateParams{
			RecipientName: adminName,
			CompanyName:   companyName,
			Project:       emails.CLAProjectParams{ExternalProjectName: projectName},
		},
		SenderName:       senderName,
		SenderEmail:      senderEmail,
		CorporateConsole: corporateConsole,
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

func (s *service) ContributorEmailToOrgAdmin(ctx context.Context, repository projects_cla_groups.Repository, projectService project.Service, adminEmail string, adminName string, companyName string, projectSFIDs []string, contributor *v1Models.User, corporateConsole string) {
	f := logrus.Fields{
		"functionName":              "cla_manager.service.SendEmailToOrgAdmin",
		utils.XREQUESTID:            ctx.Value(utils.XREQUESTID),
		"adminEmail":                adminEmail,
		"adminName":                 adminName,
		"companyName":               companyName,
		"contributorName":           contributor.Username,
		"contributorGitHubID":       contributor.GithubID,
		"contributorGitHubUsername": contributor.GithubUsername,
		"contributorLFUsername":     contributor.LfUsername,
		"contributorLFEmail":        contributor.LfEmail,
		"contributorEmails":         strings.Join(contributor.Emails, ","),
		"corporateConsoleURL":       corporateConsole,
	}

	subject := fmt.Sprintf("EasyCLA:  Invitation to Sign the %s Corporate CLA and add to approved list %s ", companyName, getBestUserName(contributor))
	recipients := []string{adminEmail}
	body, err := emails.RenderV2ContributorToOrgAdminTemplate(repository, projectService, projectSFIDs, emails.V2ContributorToOrgAdminTemplateParams{
		CLAManagerTemplateParams: emails.CLAManagerTemplateParams{
			RecipientName: adminName,
			CompanyName:   companyName,
		},
		UserDetails:      getFormattedUserDetails(contributor),
		CorporateConsole: corporateConsole,
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

func (s *service) SendEmailToCLAManagerDesigneeCorporate(ctx context.Context, repository projects_cla_groups.Repository, projectService project.Service, corporateConsole string, companyName string, projectSFID string, projectName string, designeeEmail string, designeeName string, senderEmail string, senderName string) {
	f := logrus.Fields{
		"functionName":     "cla_manager.service.SendEmailToCLAManagerDesigneeCorporate",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"corporateConsole": corporateConsole,
		"companyName":      companyName,
		"projectName":      projectName,
		"designeeEmail":    designeeEmail,
		"designeeName":     designeeName,
		"senderEmail":      senderEmail,
		"senderName":       senderName,
	}

	subject := fmt.Sprintf("EasyCLA:  Invitation to Sign the %s Corporate CLA ", companyName)
	recipients := []string{designeeEmail}
	body, err := emails.RenderV2CLAManagerDesigneeCorporateTemplate(repository, projectService, projectSFID, emails.V2CLAManagerDesigneeCorporateTemplateParams{
		CLAManagerTemplateParams: emails.CLAManagerTemplateParams{
			RecipientName: designeeName,
			CompanyName:   companyName,
		},
		SenderName:       senderName,
		SenderEmail:      senderEmail,
		CorporateConsole: corporateConsole,
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

func (s *service) SendEmailToCLAManagerDesignee(ctx context.Context, repository projects_cla_groups.Repository, projectService project.Service, corporateConsole string, companyName string, projectNames, projectSFIDs []string, designeeEmail string, designeeName string, contributorID string, contributorName string) {
	f := logrus.Fields{
		"functionName":     "cla_manager.service.SendEmailToCLAManagerDesignee",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"corporateConsole": corporateConsole,
		"companyName":      companyName,
		"projectNames":     strings.Join(projectNames, ","),
		"designeeEmail":    designeeEmail,
		"designeeName":     designeeName,
		"contributorID":    contributorID,
		"contributorName":  contributorName,
	}

	subject := fmt.Sprintf("EasyCLA:  Invitation to Sign the %s Corporate CLA and add to approved list %s ",
		companyName, contributorID)
	recipients := []string{designeeEmail}
	body, err := emails.RenderV2ToCLAManagerDesigneeTemplate(repository, projectService, projectSFIDs,
		emails.V2ToCLAManagerDesigneeTemplateParams{
			RecipientName:    designeeName,
			ContributorID:    contributorID,
			ContributorName:  contributorName,
			CorporateConsole: corporateConsole,
		})

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

func (s *service) SendDesigneeEmailToUserWithNoLFID(ctx context.Context, repository projects_cla_groups.Repository, requesterUsername, requesterEmail, userWithNoLFIDName, userWithNoLFIDEmail, organizationName, organizationID, projectName string, projectID *string, role string, corporateConsoleV2URL string) error {
	f := logrus.Fields{
		"functionName":          "cla_manager.service.SendDesigneeEmailToUserWithNoLFID",
		utils.XREQUESTID:        ctx.Value(utils.XREQUESTID),
		"userWithNoLFIDName":    userWithNoLFIDName,
		"userWithNoLFIDEmail":   userWithNoLFIDEmail,
		"organizationID":        organizationID,
		"projectID":             utils.StringValue(projectID),
		"role":                  role,
		"corporateConsoleV2URL": corporateConsoleV2URL,
	}

	subject := fmt.Sprintf("EasyCLA: Invitation to create LF Login and complete process of becoming CLA Manager for project: %s ", projectName)
	body, err := emails.RenderV2DesigneeToUserWithNoLFIDTemplate(repository, *projectID,
		emails.V2DesigneeToUserWithNoLFIDTemplateParams{
			CLAManagerTemplateParams: emails.CLAManagerTemplateParams{
				RecipientName: userWithNoLFIDName,
				Project:       emails.CLAProjectParams{ExternalProjectName: projectName},
				CompanyName:   organizationName,
			},
			RequesterUserName: requesterUsername,
			RequesterEmail:    requesterEmail,
			CorporateConsole:  corporateConsoleV2URL,
		})

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("rendering template : %s failed : %v", emails.V2DesigneeToUserWithNoLFIDTemplateName, err)
		return err
	}

	acsClient := v2AcsService.GetClient()
	automate := false
	log.WithFields(f).Debug("sending user invite request...")
	return acsClient.SendUserInvite(ctx, &userWithNoLFIDEmail, role, utils.ProjectOrgScope, projectID, organizationID, "userinvite", &subject, &body, automate)
}

// sendEmailToUserWithNoLFID helper function to send email to a given user with no LFID
func (s *service) SendEmailToUserWithNoLFID(ctx context.Context, repository projects_cla_groups.Repository, projectName, requesterUsername, requesterEmail, userWithNoLFIDName, userWithNoLFIDEmail, organizationID string, projectID *string, role string) error {
	f := logrus.Fields{
		"functionName":        "cla_manager.service.SendEmailToUserWithNoLFID",
		utils.XREQUESTID:      ctx.Value(utils.XREQUESTID),
		"projectName":         projectName,
		"requesterUsername":   requesterUsername,
		"requesterEmail":      requesterEmail,
		"userWithNoLFIDName":  userWithNoLFIDName,
		"userWithNoLFIDEmail": userWithNoLFIDEmail,
		"organizationID":      organizationID,
		"projectID":           utils.StringValue(projectID),
		"role":                role,
	}

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Invitation to create LF Login and complete process of becoming CLA Manager with %s role", role)
	body, err := emails.RenderV2CLAManagerToUserWithNoLFIDTemplate(repository, userWithNoLFIDName, projectName, *projectID, requesterUsername, requesterEmail)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("rendering email : %s failed : %v", emails.V2CLAManagerToUserWithNoLFIDTemplateName, err)
		return err
	}
	acsClient := v2AcsService.GetClient()
	automate := false

	log.WithFields(f).Debug("sending user invite request...")
	return acsClient.SendUserInvite(ctx, &userWithNoLFIDEmail, role, utils.ProjectOrgScope, projectID, organizationID, "userinvite", &subject, &body, automate)
}
