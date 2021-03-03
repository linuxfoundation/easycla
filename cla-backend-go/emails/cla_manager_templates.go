// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"github.com/communitybridge/easycla/cla-backend-go/project"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
)

// RemovedCLAManagerTemplateParams is email params for RemovedCLAManagerTemplate
type RemovedCLAManagerTemplateParams struct {
	CLAManagerTemplateParams
}

const (
	// RemovedCLAManagerTemplateName is name of the RemovedCLAManagerTemplate
	RemovedCLAManagerTemplateName = "RemovedCLAManagerTemplate"
	// RemovedCLAManagerTemplate includes the email template for email when user is removed as CLA Manager
	RemovedCLAManagerTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project {{.GetProjectNameOrFoundation}} and CLA Group {{.CLAGroupName}}.</p>
<p>You have been removed as a CLA Manager from {{.CompanyName}} for the project {{.Project.ExternalProjectName}}.</p>
<p>If you have further questions about this, please contact one of the existing managers from
{{.CompanyName}}:</p>
<ul>
	{{range .CLAManagers}}
		<li>{{.LfUsername}} {{.Email}}</li>
	{{end}}
</ul>
`
)

// RenderRemovedCLAManagerTemplate renders the RemovedCLAManagerTemplate
func RenderRemovedCLAManagerTemplate(repository projects_cla_groups.Repository, projectService project.Service, claGroupModelVersion, recipientName, companyName, projectSFID string, claManagers []ClaManagerInfoParams) (string, error) {
	params := CLAManagerTemplateParams{
		RecipientName: recipientName,
		CompanyName:   companyName,
		CLAManagers:   claManagers,
	}

	err := PrefillCLAManagerTemplateParamsFromClaGroup(repository, projectService, projectSFID, &params, nil)
	if err != nil {
		return "", err
	}

	return RenderTemplate(claGroupModelVersion, RemovedCLAManagerTemplateName, RemovedCLAManagerTemplate, RemovedCLAManagerTemplateParams{params})
}

// RequestAccessToCLAManagersTemplateParams is email params for RequestAccessToCLAManagersTemplate
type RequestAccessToCLAManagersTemplateParams struct {
	CLAManagerTemplateParams
	RequesterName  string
	RequesterEmail string
	CorporateURL   string
}

const (
	// RequestAccessToCLAManagersTemplateName is email template name for RequestAccessToCLAManagersTemplate
	RequestAccessToCLAManagersTemplateName = "RequestAccessToCLAManagersTemplateName"
	// RequestAccessToCLAManagersTemplate is email template for
	RequestAccessToCLAManagersTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project {{.Project.ExternalProjectName}}.</p>
<p>You are currently listed as a CLA Manager from {{.CompanyName}} for the project {{.Project.ExternalProjectName}}. This means that you are able to maintain the
list of employees allowed to contribute to {{.Project.ExternalProjectName}} on behalf of your company, as well as view and manage the list of
your company’s CLA Managers for {{.Project.ExternalProjectName}}.</p>
<p>{{.RequesterName}} ({{.RequesterEmail}}) has requested to be added as another CLA Manager from {{.CompanyName}} for {{.Project.ExternalProjectName}}. This would permit them to maintain the
lists of approved contributors and CLA Managers as well.</p>
<p>If you want to permit this, please log into the <a href="{{.CorporateURL}}" target="_blank">EasyCLA Corporate Console</a>,
select your company, then select the {{.Project.ExternalProjectName}} project. From the CLA Manager requests, you can approve this user as an
additional CLA Manager.</p>
`
)

// RequestApprovedToCLAManagersTemplateParams is email params for RequestApprovedToCLAManagersTemplate
type RequestApprovedToCLAManagersTemplateParams struct {
	CLAManagerTemplateParams
	RequesterName  string
	RequesterEmail string
}

const (
	// RequestApprovedToCLAManagersTemplateName is email template name for RequestApprovedToCLAManagersTemplate
	RequestApprovedToCLAManagersTemplateName = "RequestApprovedToCLAManagersTemplateName"
	// RequestApprovedToCLAManagersTemplate is email template for
	RequestApprovedToCLAManagersTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project {{.Project.ExternalProjectName}}.</p>
<p>The following user has been approved as a CLA Manager from {{.CompanyName}} for the project {{.Project.ExternalProjectName}}. This means that they can now
maintain the list of employees allowed to contribute to {{.Project.ExternalProjectName}} on behalf of your company, as well as view and manage the
list of company’s CLA Managers for {{.Project.ExternalProjectName}}.</p>
<ul>
<li>{{.RequesterName}} ({{.RequesterEmail}})</li>
</ul>
`
)

// RequestApprovedToRequesterTemplateParams email template params for RequestApprovedToRequesterTemplate
type RequestApprovedToRequesterTemplateParams struct {
	CLAManagerTemplateParams
	CorporateURL string
}

const (
	// RequestApprovedToRequesterTemplateName is email template name for RequestApprovedToRequesterTemplate
	RequestApprovedToRequesterTemplateName = "RequestApprovedToRequesterTemplate"
	// RequestApprovedToRequesterTemplate is email template for
	RequestApprovedToRequesterTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project {{.Project.ExternalProjectName}}.</p>
<p>You have now been approved as a CLA Manager from {{.CompanyName}} for the project {{.Project.ExternalProjectName}}.  This means that you can now maintain the
list of employees allowed to contribute to {{.Project.ExternalProjectName}} on behalf of your company, as well as view and manage the list of your
company’s CLA Managers for {{.Project.ExternalProjectName}}.</p>
<p> To get started, please log into the <a href="{{.CorporateURL}}" target="_blank">EasyCLA Corporate Console</a>, and select your
company and then the project {{.Project.ExternalProjectName}}. From here you will be able to edit the list of approved employees and CLA Managers.</p>
`
)

// RequestDeniedToCLAManagersTemplateParams is email params for RequestDeniedToCLAManagersTemplate
type RequestDeniedToCLAManagersTemplateParams struct {
	CLAManagerTemplateParams
	RequesterName  string
	RequesterEmail string
}

const (
	// RequestDeniedToCLAManagersTemplateName is email template name for RequestDeniedToCLAManagersTemplate
	RequestDeniedToCLAManagersTemplateName = "RequestDeniedToCLAManagersTemplate"
	// RequestDeniedToCLAManagersTemplate is email template for
	RequestDeniedToCLAManagersTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project {{.Project.ExternalProjectName}}.</p>
<p>The following user has been denied as a CLA Manager from {{.CompanyName}} for the project {{.Project.ExternalProjectName}}. This means that they will not
be able to maintain the list of employees allowed to contribute to {{.Project.ExternalProjectName}} on behalf of your company.</p>
<ul>
<li>{{.RequesterName}} ({{.RequesterEmail}})</li>
</ul>
`
)

// RequestDeniedToRequesterTemplateParams is email params for RequestDeniedToRequesterTemplate
type RequestDeniedToRequesterTemplateParams struct {
	CLAManagerTemplateParams
}

const (
	// RequestDeniedToRequesterTemplateName is email template name for RequestDeniedToRequesterTemplate
	RequestDeniedToRequesterTemplateName = "RequestDeniedToRequesterTemplate"
	// RequestDeniedToRequesterTemplate is email template for
	RequestDeniedToRequesterTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project {{.Project.ExternalProjectName}}.</p>
<p>You have been denied as a CLA Manager from {{.CompanyName}} for the project {{.Project.ExternalProjectName}}. This means that you can not maintain the
list of employees allowed to contribute to {{.Project.ExternalProjectName}} on behalf of your company.</p>
`
)

// ClaManagerAddedEToUserTemplateParams is email params for ClaManagerAddedEToUserTemplate
type ClaManagerAddedEToUserTemplateParams struct {
	CLAManagerTemplateParams
	CorporateURL string
}

const (
	// ClaManagerAddedEToUserTemplateName is email template name for ClaManagerAddedEToUserTemplate
	ClaManagerAddedEToUserTemplateName = "ClaManagerAddedEToUserTemplate"
	// ClaManagerAddedEToUserTemplate is email template for
	ClaManagerAddedEToUserTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project {{.CLAGroupName}}.</p>
<p>You have been added as a CLA Manager from {{.CompanyName}} for the project {{.CLAGroupName}}.  This means that you can now maintain the
list of employees allowed to contribute to {{.Project.ExternalProjectName}} on behalf of your company, as well as view and manage the list of your
company’s CLA Managers for {{.CLAGroupName}}.</p>
<p> To get started, please log into the <a href="{{.CorporateURL}}" target="_blank">EasyCLA Corporate Console</a>, and select your
company and then the project {{.CLAGroupName}}. From here you will be able to edit the list of approved employees and CLA Managers.</p>
`
	//V2ClaManagerAddedEToUserTemplate email template for cla manager v2
	V2ClaManagerAddedEToUserTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project {{.Project.ExternalProjectName}} and CLA Group {{.CLAGroupName}}.</p>
<p>You have been added as a CLA Manager for the organization {{.CompanyName}} and the project {{.Project.ExternalProjectName}}.  This means that you can now maintain the
list of employees allowed to contribute to the project {{.Project.ExternalProjectName}} on behalf of your company, as well as view and manage the list of your
company’s CLA Managers for the CLA Group {{.CLAGroupName}}.</p>
<p> To get started, please log into the <a href="{{.CorporateURL}}" target="_blank">EasyCLA Corporate Console</a>, and select your
company and then the project {{.Project.ExternalProjectName}}. From here you will be able to edit the list of approved employees and CLA Managers.</p>
`
)

// ClaManagerAddedToCLAManagersTemplateParams is email params for ClaManagerAddedToCLAManagersTemplate
type ClaManagerAddedToCLAManagersTemplateParams struct {
	CLAManagerTemplateParams
	Name  string
	Email string
}

const (
	// ClaManagerAddedToCLAManagersTemplateName is email template name for ClaManagerAddedToCLAManagersTemplate
	ClaManagerAddedToCLAManagersTemplateName = "ClaManagerAddedToCLAManagersTemplate"
	// ClaManagerAddedToCLAManagersTemplate is email template for
	ClaManagerAddedToCLAManagersTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project {{.Project.ExternalProjectName}}.</p>
<p>The following user has been added as a CLA Manager from {{.CompanyName}} for the project {{.Project.ExternalProjectName}}. This means that they can now
maintain the list of employees allowed to contribute to {{.Project.ExternalProjectName}} on behalf of your company, as well as view and manage the
list of company’s CLA Managers for {{.Project.ExternalProjectName}}.</p>
<ul>
<li>{{.Name}} ({{.Email}})</li>
</ul>
`
)

// ClaManagerDeletedToCLAManagersTemplateParams is template params for ClaManagerDeletedToCLAManagersTemplate
type ClaManagerDeletedToCLAManagersTemplateParams struct {
	CLAManagerTemplateParams
	Name  string
	Email string
}

const (
	// ClaManagerDeletedToCLAManagersTemplateName is template name for ClaManagerDeletedToCLAManagersTemplate
	ClaManagerDeletedToCLAManagersTemplateName = "ClaManagerDeletedToCLAManagersTemplate"
	// ClaManagerDeletedToCLAManagersTemplate is template for
	ClaManagerDeletedToCLAManagersTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project {{.Project.ExternalProjectName}}.</p>
<p>{{.Name}} ({{.Email}}) has been removed as a CLA Manager from {{.CompanyName}} for the project {{.Project.ExternalProjectName}}.</p>
`
)
