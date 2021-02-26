// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/project"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
)

// V2ContributorApprovalRequestTemplateParams is email template params for V2ContributorApprovalRequestTemplate
type V2ContributorApprovalRequestTemplateParams struct {
	CLAManagerTemplateParams
	SigningEntityName     string
	UserDetails           string
	CorporateConsoleV2URL string
}

const (
	// V2ContributorApprovalRequestTemplateName is email template name for V2ContributorApprovalRequestTemplate
	V2ContributorApprovalRequestTemplateName = "V2ContributorApprovalRequestTemplateName"
	// V2ContributorApprovalRequestTemplate is email template for
	V2ContributorApprovalRequestTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the organization {{.CompanyName}}.</p>
<p>The following contributor would like to submit a contribution to the CLA Group {{.CLAGroupName}} and is requesting to be approved as a contributor for your organization: </p>
<p>{{.UserDetails}}</p>
<p> Approval can be done at {{.CorporateConsoleV2URL}} </p>
<p>Please notify the contributor once they are added to the approved list of contributors so that they can complete their contribution.</p>
`
)

// V2OrgAdminTemplateParams is email params for V2OrgAdminTemplate
type V2OrgAdminTemplateParams struct {
	CLAManagerTemplateParams
	SenderName       string
	SenderEmail      string
	CorporateConsole string
}

const (
	// V2OrgAdminTemplateName is template name for V2OrgAdminTemplate
	V2OrgAdminTemplateName = "V2OrgAdminTemplate"
	// V2OrgAdminTemplate is email template for
	V2OrgAdminTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the CLA setup and signing process for the organization {{.CompanyName}}.</p>
<p> {{.SenderName}} {{.SenderEmail}} has identified you as a potential candidate to setup the Corporate CLA in support of the following project(s): </p>
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
func RenderV2OrgAdminTemplate(repository projects_cla_groups.Repository, projectService project.Service, projectSFID string, params V2OrgAdminTemplateParams) (string, error) {
	if err := PrefillCLAManagerTemplateParamsFromClaGroup(repository, projectSFID, &params.CLAManagerTemplateParams); err != nil {
		return "", err
	}

	projectParams, err := PrefillCLAProjectParams(repository, projectService, []string{projectSFID}, params.CorporateConsole)
	if err != nil {
		return "", err
	}

	params.Project = projectParams[0]

	return RenderTemplate(utils.V2, V2OrgAdminTemplateName, V2OrgAdminTemplate, params)
}

// V2ContributorToOrgAdminTemplateParams is email template params for V2ContributorToOrgAdminTemplate
type V2ContributorToOrgAdminTemplateParams struct {
	CLAManagerTemplateParams
	Projects         []CLAProjectParams
	UserDetails      string
	CorporateConsole string
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
func RenderV2ContributorToOrgAdminTemplate(repository projects_cla_groups.Repository, projectService project.Service, projectSFIDs []string, params V2ContributorToOrgAdminTemplateParams) (string, error) {
	// prefill the projects data
	projects, err := PrefillCLAProjectParams(repository, projectService, projectSFIDs, params.CorporateConsole)
	if err != nil {
		return "", err
	}

	params.Projects = projects

	return RenderTemplate(utils.V2, V2ContributorToOrgAdminTemplateName,
		V2ContributorToOrgAdminTemplate, params)
}

// V2CLAManagerDesigneeCorporateTemplateParams is email params for V2CLAManagerDesigneeCorporateTemplate
type V2CLAManagerDesigneeCorporateTemplateParams struct {
	CLAManagerTemplateParams
	SenderName       string
	SenderEmail      string
	CorporateConsole string
}

const (
	// V2CLAManagerDesigneeCorporateTemplateName is email template name for V2CLAManagerDesigneeCorporateTemplate
	V2CLAManagerDesigneeCorporateTemplateName = "V2CLAManagerDesigneeCorporateTemplate"
	// V2CLAManagerDesigneeCorporateTemplate is email template for
	V2CLAManagerDesigneeCorporateTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the CLA setup and signing process for the organization {{.CompanyName}}.</p>
<p> {{.SenderName}} {{.SenderEmail}} has identified you as a potential candidate to setup the Corporate CLA for the organization {{.CompanyName}} in support of the following project(s): </p>
<ul>
	<li>{{.Project.ExternalProjectName}}</li>
</ul>
<p>Before the contribution can be accepted, your organization must sign a CLA. 
Either you or someone whom to designate from your company can login to this portal ({{.CorporateConsole}}) and sign the CLA for this project {{.Project.GetProjectFullURL}} </p>
<p>If you are not the CLA Manager, please forward this email to the appropriate person so that they can start the CLA process.</p>
<p> Please notify the user once CLA setup is complete.</p>
`
)

// RenderV2CLAManagerDesigneeCorporateTemplate renders V2CLAManagerDesigneeCorporateTemplate
func RenderV2CLAManagerDesigneeCorporateTemplate(repository projects_cla_groups.Repository, projectService project.Service, projectSFID string, params V2CLAManagerDesigneeCorporateTemplateParams) (string, error) {
	if err := PrefillCLAManagerTemplateParamsFromClaGroup(repository, projectSFID, &params.CLAManagerTemplateParams); err != nil {
		return "", err
	}

	projects, err := PrefillCLAProjectParams(repository, projectService, []string{projectSFID}, params.CorporateConsole)
	if err != nil {
		return "", err
	}

	// assing the prefilled project
	params.Project = projects[0]

	return RenderTemplate(utils.V2, V2CLAManagerDesigneeCorporateTemplateName, V2CLAManagerDesigneeCorporateTemplate, params)
}

// V2ToCLAManagerDesigneeTemplateParams is email params for V2ToCLAManagerDesigneeTemplate
type V2ToCLAManagerDesigneeTemplateParams struct {
	RecipientName    string
	Projects         []CLAProjectParams
	ContributorEmail string
	ContributorName  string
	CorporateConsole string
	CompanyName      string
}

// GetProjectsOrProject returns the single Project or comma separated projects if more than one
func (p V2ToCLAManagerDesigneeTemplateParams) GetProjectsOrProject() string {
	if len(p.Projects) == 1 {
		return p.Projects[0].ExternalProjectName
	}

	var projectNames []string
	for _, p := range p.Projects {
		projectNames = append(projectNames, p.ExternalProjectName)
	}

	return strings.Join(projectNames, ", ")
}

const (
	// V2ToCLAManagerDesigneeTemplateName is email template name for V2ToCLAManagerDesigneeTemplate
	V2ToCLAManagerDesigneeTemplateName = "V2ToCLAManagerDesigneeTemplateName"
	// V2ToCLAManagerDesigneeTemplate is email template for
	V2ToCLAManagerDesigneeTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project(s): {{.GetProjectsOrProject}}.</p>
<p>We received a request from {{.ContributorName}} ({{.ContributorEmail}}) to contribute to the above projects on behalf of your organization.</p>
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
func RenderV2ToCLAManagerDesigneeTemplate(repository projects_cla_groups.Repository, projectService project.Service, projectSFIDs []string, params V2ToCLAManagerDesigneeTemplateParams, template string, templateName string) (string, error) {
	// prefill the projects data
	projects, err := PrefillCLAProjectParams(repository, projectService, projectSFIDs, params.CorporateConsole)
	if err != nil {
		return "", err
	}

	params.Projects = projects

	return RenderTemplate(utils.V2, templateName,
		template, params)
}

// V2DesigneeToUserWithNoLFIDTemplateParams is email params for V2DesigneeToUserWithNoLFIDTemplate
type V2DesigneeToUserWithNoLFIDTemplateParams struct {
	CLAManagerTemplateParams
	RequesterUserName string
	RequesterEmail    string
	CorporateConsole  string
}

const (
	// V2DesigneeToUserWithNoLFIDTemplateName is email template name for V2DesigneeToUserWithNoLFIDTemplate
	V2DesigneeToUserWithNoLFIDTemplateName = "V2DesigneeToUserWithNoLFIDTemplateName"
	// V2DesigneeToUserWithNoLFIDTemplate is email template for
	V2DesigneeToUserWithNoLFIDTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project(s): {{.GetProjectsOrProject}}.</p>
<p>We received a request from {{.ContributorName}} ({{.ContributorEmail}}) to contribute to any of the above projects on behalf of your
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
<p>This is a notification email from EasyCLA regarding the Project {{.GetProjectNameOrFoundation}} and CLA Group {{.CLAGroupName}}.</p>
<p>User {{.RequesterUserName}} ({{.RequesterEmail}}) was trying to add you as a CLA Manager for the Project {{.Project.ExternalProjectName}} but was unable to identify your account details in
the EasyCLA system. In order to become a CLA Manager for the Project {{.Project.ExternalProjectName}}, you will need to accept the invite below.
Once complete, notify the user {{.RequesterUserName}} and they will be able to add you as a CLA Manager.</p>
<p> <a href="USERACCEPTLINK">Accept Invite</a> </p>
`
)

// RenderV2CLAManagerToUserWithNoLFIDTemplate renders V2CLAManagerToUserWithNoLFIDTemplate
func RenderV2CLAManagerToUserWithNoLFIDTemplate(repository projects_cla_groups.Repository, recipientName, projectName, projectSFID, requesterName, requesterEmail string) (string, error) {
	params := V2CLAManagerToUserWithNoLFIDTemplateParams{
		CLAManagerTemplateParams: CLAManagerTemplateParams{
			RecipientName: recipientName,
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
