// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"github.com/communitybridge/easycla/cla-backend-go/utils"
)

// Contributor representing GH user details
type Contributor struct {
	Email         string
	Username      string
	EmailLabel    string
	UsernameLabel string
}

// V2ContributorApprovalRequestTemplateParams is email template params for V2ContributorApprovalRequestTemplate
type V2ContributorApprovalRequestTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	SigningEntityName string
	UserDetails       string
}

const (
	// V2ContributorApprovalRequestTemplateName is email template name for V2ContributorApprovalRequestTemplate
	V2ContributorApprovalRequestTemplateName = "V2ContributorApprovalRequestTemplateName"
	// V2ContributorApprovalRequestTemplate is email template for
	V2ContributorApprovalRequestTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the organization {{.CompanyName}}.</p>
<p>The following contributor would like to submit a contribution to the projects(s): {{.GetProjectsOrProject}} and is requesting to be added to the approval list as a contributor for your organization: </p>
<p>{{.UserDetails}}</p>
<p> CLA Managers can visit the EasyCLA corporate console page for {{range $index, $projectName := .Projects}}{{if $index}},{{end}}{{$projectName.GetProjectFullURL}}{{end}} and add the contributor to one of the approval lists.</p>
<p>Please notify the contributor once they are added to the approved list of contributors so that they can complete their contribution.</p>
`
)

// RenderV2ContributorApprovalRequestTemplate renders V2ContributorApprovalRequestTemplate
func RenderV2ContributorApprovalRequestTemplate(svc EmailTemplateService, projectSFIDs []string, params V2ContributorApprovalRequestTemplateParams) (string, error) {

	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(utils.V2, projectSFIDs[0])
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams

	return RenderTemplate(utils.V2, V2ContributorApprovalRequestTemplateName, V2ContributorApprovalRequestTemplate, params)
}

// V2OrgAdminTemplateParams is email params for V2OrgAdminTemplate
type V2OrgAdminTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	SenderName  string
	SenderEmail string
}

const (
	// V2OrgAdminTemplateName is template name for V2OrgAdminTemplate
	V2OrgAdminTemplateName = "V2OrgAdminTemplate"
	// V2OrgAdminTemplate is email template for
	V2OrgAdminTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the CLA setup and signing process for the organization {{.CompanyName}}.</p>
<p>{{.SenderName}} {{.SenderEmail}} has identified you as a potential candidate to setup the Corporate CLA in support of the following project(s): </p>
<ul>
	<li>{{.Project.ExternalProjectName}}</li>
</ul>
<p>Before the contribution can be accepted, your organization must sign a CLA. 
Either you or someone whom to designate from your company can login to this portal ({{.CorporateConsole}}) and sign the CLA for this project {{.Project.GetProjectFullURL}} </p>
<p>If you are not the CLA Manager, please forward this email to the appropriate person so that they can start the CLA process.</p>
<p> Please notify the user once CLA setup is complete.</p>
`
)

// RenderV2OrgAdminTemplate renders V2OrgAdminTemplate
func RenderV2OrgAdminTemplate(svc EmailTemplateService, projectSFID string, params V2OrgAdminTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(utils.V2, projectSFID)
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams
	return RenderTemplate(utils.V2, V2OrgAdminTemplateName, V2OrgAdminTemplate, params)
}

// V2ContributorToOrgAdminTemplateParams is email template params for V2ContributorToOrgAdminTemplate
type V2ContributorToOrgAdminTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	UserDetails string
}

const (
	// V2ContributorToOrgAdminTemplateName is email template name for V2ContributorToOrgAdminTemplate
	V2ContributorToOrgAdminTemplateName = "V2ContributorToOrgAdminTemplate"
	// V2ContributorToOrgAdminTemplate is email template for
	V2ContributorToOrgAdminTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>The following contributor would like to submit a contribution to {{range $index, $projectName := .Projects}}{{if $index}},{{end}}{{$projectName.ExternalProjectName}}{{end}} and is requesting to be added to the approval list as a contributor for your organization:</p>
<p>{{.UserDetails}}</p>
<p>Before the contribution can be accepted, your organization must sign a CLA. Either you or someone whom you designate from your company can login to this portal and sign the CLA for any of the project(s): {{range $index, $projectName := .Projects}}{{if $index}},{{end}}{{$projectName.GetProjectFullURL}}{{end}}.</p>
<p>Please notify the contributor once they are added so that they may complete the contribution process.</p>

`
)

// RenderV2ContributorToOrgAdminTemplate renders V2ContributorToOrgAdminTemplate
func RenderV2ContributorToOrgAdminTemplate(svc EmailTemplateService, projectSFIDs []string, params V2ContributorToOrgAdminTemplateParams) (string, error) {
	// prefill the projects data
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(utils.V2, projectSFIDs[0])
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams

	return RenderTemplate(utils.V2, V2ContributorToOrgAdminTemplateName,
		V2ContributorToOrgAdminTemplate, params)
}

// V2CLAManagerDesigneeCorporateTemplateParams is email params for V2CLAManagerDesigneeCorporateTemplate
type V2CLAManagerDesigneeCorporateTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	SenderName  string
	SenderEmail string
}

const (
	// V2CLAManagerDesigneeCorporateTemplateName is email template name for V2CLAManagerDesigneeCorporateTemplate
	V2CLAManagerDesigneeCorporateTemplateName = "V2CLAManagerDesigneeCorporateTemplate"
	// V2CLAManagerDesigneeCorporateTemplate is email template for
	V2CLAManagerDesigneeCorporateTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the CLA setup and signing process for the organization {{.CompanyName}}.</p>
<p>{{.SenderName}} {{.SenderEmail}} has identified you as a potential candidate to setup the Corporate CLA for the organization {{.CompanyName}} in support of the following project(s): </p>
<ul>
	<li>{{.Project.ExternalProjectName}}</li>
</ul>
<p>Before the contribution can be accepted, your organization must sign a CLA. 
Either you or someone whom you designate from your company can login and <b>sign the CLA for this project {{.Project.GetProjectFullURL}}</b> </p>
<p>If you are not the CLA Manager, please forward this email to the appropriate person so that they can start the CLA process.</p>
<p> Please notify the user once CLA setup is complete.</p>
`
)

// RenderV2CLAManagerDesigneeCorporateTemplate renders V2CLAManagerDesigneeCorporateTemplate
func RenderV2CLAManagerDesigneeCorporateTemplate(emailSvc EmailTemplateService, projectSFID string, params V2CLAManagerDesigneeCorporateTemplateParams) (string, error) {
	claGroupParams, err := emailSvc.GetCLAGroupTemplateParamsFromProjectSFID(utils.V2, projectSFID)
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams

	return RenderTemplate(utils.V2, V2CLAManagerDesigneeCorporateTemplateName, V2CLAManagerDesigneeCorporateTemplate, params)
}

// V2ToCLAManagerDesigneeTemplateParams is email params for V2ToCLAManagerDesigneeTemplate
type V2ToCLAManagerDesigneeTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	Contributor Contributor
}

const (
	// V2ToCLAManagerDesigneeTemplateName is email template name for V2ToCLAManagerDesigneeTemplate
	V2ToCLAManagerDesigneeTemplateName = "V2ToCLAManagerDesigneeTemplateName"
	// V2ToCLAManagerDesigneeTemplate is email template for
	V2ToCLAManagerDesigneeTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project(s): {{.GetProjectsOrProject}}.</p>
<p>We received a request from {{.Contributor.UsernameLabel}}: {{.Contributor.Username}} ({{.Contributor.EmailLabel}}: {{.Contributor.Email}}) to contribute to the above projects on behalf of your organization.</p>
<p>Before the user contribution can be accepted, your organization must sign a Corporate CLA (CCLA).The requester has stated that you would be the initial CLA Manager for this CCLA, to coordinate the signing of the CCLA and then manage the list of employees who are authorized to contribute.</p>
<p>Please complete the following steps:</p>
<ol>
<li>After login, you will be redirected to the portal {{.CorporateConsole}} where you can either sign the CLA for any of the project(s): {{range $index, $projectName := .Projects}}{{if $index}},{{end}}{{$projectName.GetProjectFullURL}}{{end}}, or send it to an authorized signatory for your company.</li>
<li>After signing the CLA, you will need to add this contributor to the approved list in the CLA Manager console.</li>
<li>After adding the contributor, please notify them so that they can complete the contribution process.</li>
</ol>
`
)

// RenderV2ToCLAManagerDesigneeTemplate renders V2ToCLAManagerDesigneeTemplate
func RenderV2ToCLAManagerDesigneeTemplate(svc EmailTemplateService, projectSFIDs []string, params V2ToCLAManagerDesigneeTemplateParams, template string, templateName string) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(utils.V2, projectSFIDs[0])
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams

	return RenderTemplate(utils.V2, templateName,
		template, params)
}

const (
	// V2DesigneeToUserWithNoLFIDTemplateName is email template name for V2DesigneeToUserWithNoLFIDTemplate
	V2DesigneeToUserWithNoLFIDTemplateName = "V2DesigneeToUserWithNoLFIDTemplateName"
	// V2DesigneeToUserWithNoLFIDTemplate is email template for
	V2DesigneeToUserWithNoLFIDTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project(s): {{.GetProjectsOrProject}}.</p>
<p>We received a request from {{.Contributor.UsernameLabel}}: {{.Contributor.Username}} ({{.Contributor.EmailLabel}}: {{.Contributor.Email}}) to contribute to any of the above projects on behalf of your
organization {{.CompanyName}}. <p>
<p>Before the user contribution can be accepted, your organization must sign a Corporate CLA(CCLA). 
The requester has stated that you would be the initial CLA Manager for this CCLA, to coordinate the signing of the CCLA and then manage the list of employees who are authorized to contribute.</p>
<p>Please complete the following steps:</p>
<ol>
<li>Please click on <a href="USERACCEPTLINK">Accept Invite</a> to create your LF Login.This is used to access the EasyCLA CLA Manager console.</li>
<li>After login, you will be redirected to the portal {{.CorporateConsole}} where you can either sign the CLA for any of the project(s): {{range $index, $projectName := .Projects}}{{if $index}},{{end}}{{$projectName.GetProjectFullURL}}{{end}}, or send it to an authorized signatory for your company.</li>
<li>After signing the CLA, you will need to add this contributor to the approved list in the CLA Manager console.</li>
<li>After adding the contributor, please notify them so that they can complete the contribution process.</li>
</ol>
`
)

// V2CLAManagerToUserWithNoLFIDTemplateParams is email params for V2CLAManagerToUserWithNoLFIDTemplate
type V2CLAManagerToUserWithNoLFIDTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	RequesterUserName string
	RequesterEmail    string
	Projects          []CLAProjectParams
}

const (
	// V2CLAManagerToUserWithNoLFIDTemplateName is email template name
	V2CLAManagerToUserWithNoLFIDTemplateName = "V2CLAManagerToUserWithNoLFIDTemplate"
	// V2CLAManagerToUserWithNoLFIDTemplate is email template
	V2CLAManagerToUserWithNoLFIDTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the CLA setup and signing process for the organization {{.CompanyName}}.The user {{.RequesterUserName}} ({{.RequesterEmail}}) has identified you as a potential candidate to setup the Corporate CLA for the organization {{.CompanyName }} and the project {{.GetProjectNameOrFoundation}}</p>
<p>Before the user contribution can be accepted, your organization must sign a Corporate CLA(CCLA).</p>
<p>Please complete the following steps:</p>
<ol>
<li>Please click on <a href="USERACCEPTLINK">Accept Invite</a> to create your LF Login.This is used to access the EasyCLA CLA Manager console.</li>
<li>After login, you will be redirected to the portal {{.Project.CorporateConsole}} where you can either sign the CLA for the project: {{.Project.GetProjectFullURL}}, or send it to an authorized signatory for your company.</li>
<li>After signing the CLA, you will need to add this contributor to the approved list in the CLA Manager console.</li>
<li>After adding the contributor, please notify them so that they can complete the contribution process.</li>
</ol>

`
)

// RenderV2CLAManagerToUserWithNoLFIDTemplate renders V2CLAManagerToUserWithNoLFIDTemplate
func RenderV2CLAManagerToUserWithNoLFIDTemplate(svc EmailTemplateService, projectSFID string, params V2CLAManagerToUserWithNoLFIDTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(utils.V2, projectSFID)
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams

	body, err := RenderTemplate(utils.V2, V2CLAManagerToUserWithNoLFIDTemplateName,
		V2CLAManagerToUserWithNoLFIDTemplate,
		params)
	return body, err
}
