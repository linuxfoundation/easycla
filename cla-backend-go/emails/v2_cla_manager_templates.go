// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
)

// V2ContributorApprovalRequestTemplateParams is email template params for V2ContributorApprovalRequestTemplate
type V2ContributorApprovalRequestTemplateParams struct {
	CLAManagerTemplateParams
	SigningEntityName string
	UserDetails       string
	LfxPortalURL      string
}

const (
	// V2ContributorApprovalRequestTemplateName is email template name for V2ContributorApprovalRequestTemplate
	V2ContributorApprovalRequestTemplateName = "V2ContributorApprovalRequestTemplateName"
	// V2ContributorApprovalRequestTemplate is email template for
	V2ContributorApprovalRequestTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the organization {{.CompanyName}}.</p>
<p>The following contributor would like to submit a contribution to the {{.SigningEntityName}} CLA Group
and is requesting to be approved as a contributor for your organization: </p>
<p>{{.CLAGroupName}} - Signing Entity Name: {{.UserDetails}}</p>
<p> Approval can be done at {{.LfxPortalURL}} </p>
<p>Please notify the contributor once they are added so that they may complete the contribution process.</p>
`
)

// V2OrgAdminTemplateParams is email params for V2OrgAdminTemplate
type V2OrgAdminTemplateParams struct {
	CLAManagerTemplateParams
	SenderName       string
	SenderEmail      string
	ProjectList      []string
	CorporateConsole string
}

const (
	// V2OrgAdminTemplateName is template name for V2OrgAdminTemplate
	V2OrgAdminTemplateName = "V2OrgAdminTemplate"
	// V2OrgAdminTemplate is email template for
	V2OrgAdminTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the CLA setup and signing process for {{.CompanyName}}.</p>
<p> {{.SenderName}} {{.SenderEmail}} has identified you as a potential candidate to setup the Corporate CLA for {{.CompanyName}} in support of the following projects: </p>
<ul>
	{{range .ProjectList}}
		<li>{{.}}</li>
	{{end}}
</ul>
<p>Before the contribution can be accepted, your organization must sign a CLA. 
Either you or someone whom to designate from your company can login to this portal ({{.CorporateConsole}}) and sign the CLA for this project {{.ProjectName}} </p>
<p>If you are not the CLA Manager, please forward this email to the appropriate person so that they can start the CLA process.</p>
<p> Please notify the user once CLA setup is complete.</p>
`
)

// V2ContributorToOrgAdminTemplateParams is email template params for V2ContributorToOrgAdminTemplate
type V2ContributorToOrgAdminTemplateParams struct {
	CLAManagerTemplateParams
	ProjectNames     []string
	UserDetails      string
	CorporateConsole string
}

const (
	// V2ContributorToOrgAdminTemplateName is email template name for V2ContributorToOrgAdminTemplate
	V2ContributorToOrgAdminTemplateName = "V2ContributorToOrgAdminTemplate"
	// V2ContributorToOrgAdminTemplate is email template for
	V2ContributorToOrgAdminTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project(s) {{range $index, $projectName := .ProjectNames}}{{if $index}},{{end}}{{$projectName}}{{end}}</p>
<p>The following contributor is requesting to sign CLA for organization: {{.CompanyName}}</p>
<p>{{.UserDetails}}</p>
<p>Before the user contribution can be accepted, your organization must sign a CLA.
<p>Kindly login to this portal {{.CorporateConsole}} and sign the CLA for any of the projects {{range $index, $projectName := .ProjectNames}}{{if $index}},{{end}}{{$projectName}}{{end}}.</p>
<p>Please notify the contributor once they are added so that they may complete the contribution process.</p>
`
)

// V2CLAManagerDesigneeCorporateTemplateParams is email params for V2CLAManagerDesigneeCorporateTemplate
type V2CLAManagerDesigneeCorporateTemplateParams struct {
	CLAManagerTemplateParams
	SenderName       string
	SenderEmail      string
	ProjectList      []string
	CorporateConsole string
}

const (
	// V2CLAManagerDesigneeCorporateTemplateName is email template name for V2CLAManagerDesigneeCorporateTemplate
	V2CLAManagerDesigneeCorporateTemplateName = "V2CLAManagerDesigneeCorporateTemplate"
	// V2CLAManagerDesigneeCorporateTemplate is email template for
	V2CLAManagerDesigneeCorporateTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the CLA setup and signing process for {{.CompanyName}}.</p>
<p> {{.SenderName}} {{.SenderEmail}} has identified you as a potential candidate to setup the Corporate CLA for {{.CompanyName}} in support of the following projects: </p>
<ul>
	{{range .ProjectList}}
		<li>{{.}}</li>
	{{end}}
</ul>
<p>Before the contribution can be accepted, your organization must sign a CLA. 
Either you or someone whom to designate from your company can login to this portal ({{.CorporateConsole}}) and sign the CLA for this project {{.ProjectName}} </p>
<p>If you are not the CLA Manager, please forward this email to the appropriate person so that they can start the CLA process.</p>
<p> Please notify the user once CLA setup is complete.</p>
`
)

// RenderV2CLAManagerDesigneeCorporateTemplate renders V2CLAManagerDesigneeCorporateTemplate
func RenderV2CLAManagerDesigneeCorporateTemplate(repository projects_cla_groups.Repository, projectSFID string, params V2CLAManagerDesigneeCorporateTemplateParams) (string, error) {
	if err := PrefillCLAManagerTemplateParamsFromClaGroup(repository, projectSFID, &params.CLAManagerTemplateParams); err != nil {
		return "", err
	}

	return RenderTemplate(utils.V2, V2CLAManagerDesigneeCorporateTemplateName, V2CLAManagerDesigneeCorporateTemplate, params)
}

// V2ToCLAManagerDesigneeTemplateParams is email params for V2ToCLAManagerDesigneeTemplate
type V2ToCLAManagerDesigneeTemplateParams struct {
	RecipientName    string
	ProjectNames     []string
	ContributorID    string
	ContributorName  string
	CorporateConsole string
}

const (
	// V2ToCLAManagerDesigneeTemplateName is email template name for V2ToCLAManagerDesigneeTemplate
	V2ToCLAManagerDesigneeTemplateName = "V2ToCLAManagerDesigneeTemplateName"
	// V2ToCLAManagerDesigneeTemplate is email template for
	V2ToCLAManagerDesigneeTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project(s) {{range $index, $projectName := .ProjectNames}}{{if $index}},{{end}}{{$projectName}}{{end}}.</p>
<p>The following contributor is requesting to sign CLA for organization: </p>
<p> {{.ContributorID}} ({{.ContributorName}}) </p>
<p>Before the user contribution can be accepted, your organization must sign a CLA.
<p>Kindly login to this portal {{.CorporateConsole}} and sign the CLA for one of the project(s) {{ range $index, $projectName := .ProjectNames}}{{if $index}},{{end}}{{$projectName}}{{end}}. </p>
<p>Please notify the contributor once they are added so that they may complete the contribution process.</p>
`
)

// V2DesigneeToUserWithNoLFIDTemplateParams is email params for V2DesigneeToUserWithNoLFIDTemplate
type V2DesigneeToUserWithNoLFIDTemplateParams struct {
	CLAManagerTemplateParams
	RequesterUserName string
	RequesterEmail    string
}

const (
	// V2DesigneeToUserWithNoLFIDTemplateName is email template name for V2DesigneeToUserWithNoLFIDTemplate
	V2DesigneeToUserWithNoLFIDTemplateName = "V2DesigneeToUserWithNoLFIDTemplateName"
	// V2DesigneeToUserWithNoLFIDTemplate is email template for
	V2DesigneeToUserWithNoLFIDTemplate = `
<p>Hello {{.RecipientName}}, </p>
<p> User {{.RequesterUserName}} ({{.RequesterEmail}}) was trying to add you as a CLA Manager for Project {{.GetProjectNameOrFoundation}}, CLA Group {{.CLAGroupName}} and Company {{.CompanyName}} but was unable to identify your account details in the EasyCLA system </p>
<p> This email will guide you to completing the CLA Manager role assignment </p>
<p>1. Accept Invite link below will take you SSO login page where you can login with your LF Login or create a LF Login and then login.</p>
<p>2. After logging in SSO screen should direct you to CLA Corporate Console page where you will see the project you a re associated with.</p>
<p>3. Click on workflow steps to complete the signup process. Please follow this documentation to help you guide through the process - https://docs.linuxfoundation.org/lfx/v/v2/easycla/corporate-cla-manager-designee-or-initial-cla-manager/sign-corporate-cla-for-a-company</p>
<p>4. Once you have completed CLA Manager workflow you will be able to manage the approved list of contributors </p>
<p> <a href="USERACCEPTLINK">Accept Invite</a> </p>
`
)

// RenderV2DesigneeToUserWithNoLFIDTemplate renders V2DesigneeToUserWithNoLFIDTemplate
func RenderV2DesigneeToUserWithNoLFIDTemplate(repository projects_cla_groups.Repository, projectSFID string, params V2DesigneeToUserWithNoLFIDTemplateParams) (string, error) {
	if err := PrefillCLAManagerTemplateParamsFromClaGroup(repository, projectSFID, &params.CLAManagerTemplateParams); err != nil {
		return "", err
	}

	return RenderTemplate(utils.V2, V2DesigneeToUserWithNoLFIDTemplateName,
		V2DesigneeToUserWithNoLFIDTemplate, params)
}

// V2CLAManagerToUserWithNoLFIDTemplateParams is email params for V2CLAManagerToUserWithNoLFIDTemplate
type V2CLAManagerToUserWithNoLFIDTemplateParams struct {
	CLAManagerTemplateParams
	RequesterUserName string
	RequesterEmail    string
}

const (
	// V2CLAManagerToUserWithNoLFIDTemplateName is email template name
	V2CLAManagerToUserWithNoLFIDTemplateName = "V2CLAManagerToUserWithNoLFIDTemplate"
	// V2CLAManagerToUserWithNoLFIDTemplate is email template
	V2CLAManagerToUserWithNoLFIDTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the Project {{.GetProjectNameOrFoundation}} and CLA Group {{.CLAGroupName}} in the EasyCLA system.</p>
<p>User {{.RequesterUserName}} ({{.RequesterEmail}}) was trying to add you as a CLA Manager for Project {{.ProjectName}} but was unable to identify your account details in
the EasyCLA system. In order to become a CLA Manager for Project {{.ProjectName}}, you will need to accept invite below.
Once complete, notify the user {{.RequesterUserName}} and they will be able to add you as a CLA Manager.</p>
<p> <a href="USERACCEPTLINK">Accept Invite</a> </p>
`
)

// RenderV2CLAManagerToUserWithNoLFIDTemplate renders V2CLAManagerToUserWithNoLFIDTemplate
func RenderV2CLAManagerToUserWithNoLFIDTemplate(repository projects_cla_groups.Repository, recipientName, projectName, projectSFID, requesterName, requesterEmail string) (string, error) {
	params := V2CLAManagerToUserWithNoLFIDTemplateParams{
		CLAManagerTemplateParams: CLAManagerTemplateParams{
			RecipientName: recipientName,
			ProjectName:   projectName,
		},
		RequesterUserName: requesterName,
		RequesterEmail:    requesterEmail,
	}

	err := PrefillCLAManagerTemplateParamsFromClaGroup(repository, projectSFID, &params.CLAManagerTemplateParams)
	if err != nil {
		return "", err
	}

	body, err := RenderTemplate(utils.V2, V2CLAManagerToUserWithNoLFIDTemplateName,
		V2CLAManagerToUserWithNoLFIDTemplate,
		params)
	return body, err
}
