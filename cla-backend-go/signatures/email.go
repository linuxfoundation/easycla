// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"fmt"
	"strings"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/logging"
	signatureModels "github.com/communitybridge/easycla/cla-backend-go/signatures/models"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// InvalidateSignatureTemplateParams representing params when invalidating icla/ecla
type InvalidateSignatureTemplateParams struct {
	RecipientName   string
	ClaType         string
	ClaManager      string
	RemovalCriteria string
	ProjectName     string
	ProjectManager  string
	CLAManagers     []signatureModels.ClaManagerInfoParams
	CLaManager      string
	CLAGroupName    string
	Company         string
}

const (
	//InvalidateCCLAICLASignatureTemplateName is email template for InvalidateSignatureTemplate
	InvalidateCCLAICLASignatureTemplateName = "InvalidateSignatureTemplate"
	//InvalidateCCLAICLASignatureTemplate ...
	InvalidateCCLAICLASignatureTemplate = `
	<p>Hello {{.RecipientName}}</p>
    <p>This is a notification email from EasyCLA regarding the CLA Group {{.ProjectName}}</p>
	<p>You were previously authorized to contribute on behalf of your company {{COMPANY-NAME}} under its CLA. However, a CLA Manager has now removed you from the authorization list. This has additionally resulted in invalidating your current signed Individual CLA (ICLA).</p>
	<p>As a result, you will no longer be able to contribute until you are again authorized under another signed CLA.</p>
	<p>Please contact one of the CLA Managers from your company if you have questions about why you were removed. The CLA Managers from your company for this CLA are:</p>
	<ul>
	{{range .CLAManagers}}
		<li>{{.Username}} {{.Email}}</li>
	{{end}}
	</ul>
	`

	//InvalidateCCLASignatureTemplateName is email template upon approval list removal for ccla use case
	InvalidateCCLASignatureTemplateName = "InvalidateCCLAICLASignatureTemplate"
	//InvalidateCCLASignatureTemplate ...
	InvalidateCCLASignatureTemplate = `
	<p>Hello {{.RecipientName}}</p>
	<p>This is a notification email from EasyCLA regarding the CLA Group {{.CLAGroupName}}.</p>
	<p>You were previously authorized to contribute on behalf of your company {{.Company}} under its CLA. However, a CLA Manager {{.ClaManager}} has now removed you from the authorization list.</p>
	<p>As a result, you will no longer be able to contribute until you are again authorized under another signed CLA.</p>
	<p>Please contact one of the CLA Managers from your company if you have questions about why you were removed. The CLA Managers from your company for this CLA are:</p>
	<ul>
	{{range .CLAManagers}}
		<li>{{.Username}} {{.Email}}</li>
	{{end}}
	</ul>
	`

	//InvalidateICLASignatureTemplateName is email template upon approval list removal for ccla use case
	InvalidateICLASignatureTemplateName = "InvalidateICLASignatureTemplate"
	//InvalidateICLASignatureTemplate ...
	InvalidateICLASignatureTemplate = `
	<p>Hello {{.RecipientName}}</p>
	<p>This is a notification email from EasyCLA regarding the CLA Group {{.CLAGroupName}}.</p>
	<p>You had previously signed an Individual CLA (ICLA) to contribute to the project on your own behalf. However, the Project Manager has marked your ICLA as invalidated. This might be because the ICLA may have been signed in error, if your contributions should have been on behalf of your employer rather than on your own behalf.</p>
	<p>As a result, you will no longer be able to contribute until you are again authorized under another signed CLA.</p>
	<p>Please contact the Project Manager for this project if you have questions about why you were removed.</p>
	</ul>
	`

	//InvalidateCCLAICLAECLASignatureTemplateName is email template upon approval list removal for ccla use case
	InvalidateCCLAICLAECLASignatureTemplateName = "InvalidateCCLAICLAECLASignatureTemplate"
	//InvalidateCCLAICLAECLASignatureTemplate ...
	InvalidateCCLAICLAECLASignatureTemplate = `
	<p>Hello {{.RecipientName}}</p>
	<p>This is a notification email from EasyCLA regarding the CLA Group {{.CLAGroupName}}.</p>
	<p>You were previously authorized to contribute on behalf of your company {{.Company}} under its CLA. However, a CLA Manager has now removed you from the authorization list. This has additionally resulted in invalidating your current signed Individual CLA (ICLA) and your acknowledgement.</p>
	<p>As a result, you will no longer be able to contribute until you are again authorized under another signed CLA.</p>
	<p>Please contact one of the CLA Managers from your company if you have questions about why you were removed. The CLA Managers from your company for this CLA are:</p>
	<ul>
	{{range .CLAManagers}}
		<li>{{.Username}} {{.Email}}</li>
	{{end}}
	</ul>
	`
)

// sendRequestAccessEmailToContributors sends the request access email to the specified contributors
func sendRequestAccessEmailToContributorRecipient(authUser *auth.User, companyModel *models.Company, claGroupModel *models.ClaGroup, recipientName, recipientAddress, addRemove, toFrom, authorizedString string) {
	companyName := companyModel.CompanyName
	projectName := claGroupModel.ProjectName

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Approval List Update for %s on %s", companyName, projectName)
	recipients := []string{recipientAddress}
	body := fmt.Sprintf(`
<p>Hello %s,</p>
<p>This is a notification email from EasyCLA regarding the project %s.</p>
<p>You have been %s %s the Approval List of %s for %s by CLA Manager %s. This means that %s.</p>
<b>
<p>If you are a GitHub user and If you had previously submitted a pull request to EasyCLA Test Group that had failed, you can now go back to it, re-click the “Not Covered” button in the EasyCLA message in your pull request, and then follow these steps</p>
<ol>
<li>Select “Corporate Contributor”.</li>
<li>Select your company from the organization drop down list</li>
<li>Click Proceed</li>
</ol>
<p>If you are a Gerrit user and if you had previously submitted a pull request to EasyCLA Test Group that had failed, then navigate to Agreements Settings page on Gerrit, click on "New Contributor Agreement" link under Agreements section, select the radio button corresponding to Corporate CLA, click on "Please review the agreement" link, and then follow these steps</p>
<ol>
<li>Select “Corporate Contributor”.</li>
<li>Select your company from the organization drop down list</li>
<li>Click Proceed</li>
</ol>
<p>These steps will confirm your organization association and you will only need to do these once. After completing these steps, the EasyCLA check will be complete and enabled for all future code contributions for this project.</p>
</b>
%s
%s`,
		recipientName, projectName, addRemove, toFrom,
		companyName, projectName, authUser.UserName, authorizedString,
		utils.GetEmailHelpContent(claGroupModel.Version == utils.V2), utils.GetEmailSignOffContent())

	err := utils.SendEmail(subject, body, recipients)
	if err != nil {
		logging.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		logging.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

// getBestEmail is a helper function to return the best email address for the user model
func getBestEmail(userModel *models.User) string {
	f := logrus.Fields{
		"functionName": "getBestEmail",
	}

	if userModel != nil {
		if userModel.LfEmail != "" {
			return userModel.LfEmail.String()
		}

		for _, email := range userModel.Emails {
			if email != "" && !strings.Contains(email, "noreply.github.com") {
				return email
			}
		}
	} else {
		logging.WithFields(f).Warn("user model is nil")
	}

	return ""
}

func (s service) sendRequestAccessEmailToContributors(authUser *auth.User, companyModel *models.Company, claGroupModel *models.ClaGroup, approvalList *models.ApprovalList) {
	addEmailUsers := s.getAddEmailContributors(approvalList)
	for _, user := range addEmailUsers {
		sendRequestAccessEmailToContributorRecipient(authUser, companyModel, claGroupModel, user.Username, user.LfEmail.String(), "added", "to",
			fmt.Sprintf("you are authorized to contribute to %s on behalf of %s", claGroupModel.ProjectName, companyModel.CompanyName))
	}
	removeEmailUsers := s.getRemoveEmailContributors(approvalList)
	for _, user := range removeEmailUsers {
		sendRequestAccessEmailToContributorRecipient(authUser, companyModel, claGroupModel, user.Username, user.LfEmail.String(), "removed", "from",
			fmt.Sprintf("you are no longer authorized to contribute to %s on behalf of %s ", claGroupModel.ProjectName, companyModel.CompanyName))
	}
	addGitHubUsers := s.getAddGitHubContributors(approvalList)
	for _, user := range addGitHubUsers {
		sendRequestAccessEmailToContributorRecipient(authUser, companyModel, claGroupModel, user.Username, user.LfEmail.String(), "added", "to",
			fmt.Sprintf("you are authorized to contribute to %s on behalf of %s", claGroupModel.ProjectName, companyModel.CompanyName))
	}
	removeGitHubUsers := s.getRemoveGitHubContributors(approvalList)
	for _, user := range removeGitHubUsers {
		sendRequestAccessEmailToContributorRecipient(authUser, companyModel, claGroupModel, user.Username, user.LfEmail.String(), "removed", "from",
			fmt.Sprintf("you are no longer authorized to contribute to %s on behalf of %s ", claGroupModel.ProjectName, companyModel.CompanyName))
	}
	addGitlabUsers := s.getAddGitlabContributors(approvalList)
	for _, user := range addGitlabUsers {
		sendRequestAccessEmailToContributorRecipient(authUser, companyModel, claGroupModel, user.Username, user.LfEmail.String(), "added", "to",
			fmt.Sprintf("you are authorized to contribute to %s on behalf of %s", claGroupModel.ProjectName, companyModel.CompanyName))
	}
	removeGitlabUsers := s.getRemoveGitlabContributors(approvalList)
	for _, user := range removeGitlabUsers {
		sendRequestAccessEmailToContributorRecipient(authUser, companyModel, claGroupModel, user.Username, user.LfEmail.String(), "removed", "from",
			fmt.Sprintf("you are no longer authorized to contribute to %s on behalf of %s ", claGroupModel.ProjectName, companyModel.CompanyName))
	}
}

// sendRequestAccessEmailToCLAManagers sends the request access email to the specified CLA Managers
func (s service) sendApprovalListUpdateEmailToCLAManagers(companyModel *models.Company, claGroupModel *models.ClaGroup, recipientName, recipientAddress string, approvalListChanges *models.ApprovalList) {
	f := logrus.Fields{
		"functionName":      "sendApprovalListUpdateEmailToCLAManagers",
		"projectName":       claGroupModel.ProjectName,
		"projectExternalID": claGroupModel.ProjectExternalID,
		"foundationSFID":    claGroupModel.FoundationSFID,
		"companyName":       companyModel.CompanyName,
		"companyExternalID": companyModel.CompanyExternalID,
		"recipientName":     recipientName,
		"recipientAddress":  recipientAddress}

	companyName := companyModel.CompanyName
	projectName := claGroupModel.ProjectName

	// subject string, body string, recipients []string
	subject := fmt.Sprintf("EasyCLA: Approval List Update for %s on %s", companyName, projectName)
	recipients := []string{recipientAddress}
	body := fmt.Sprintf(`
<p>Hello %s,</p>
<p>This is a notification email from EasyCLA regarding the project %s.</p>
<p>The EasyCLA approval list for %s for project %s was modified.</p>
<p>The modification was as follows:</p>
%s
%s
%s`,
		recipientName, projectName, companyName, projectName, buildApprovalListSummary(approvalListChanges),
		utils.GetEmailHelpContent(claGroupModel.Version == utils.V2), utils.GetEmailSignOffContent())

	err := utils.SendEmail(subject, body, recipients)
	if err != nil {
		logging.WithFields(f).Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		logging.WithFields(f).Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}
