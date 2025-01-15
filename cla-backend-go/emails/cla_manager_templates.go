// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

// RemovedCLAManagerTemplateParams is email params for RemovedCLAManagerTemplate
type RemovedCLAManagerTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	CLAManagers []ClaManagerInfoParams
}

const (
	// RemovedCLAManagerTemplateName is name of the RemovedCLAManagerTemplate
	RemovedCLAManagerTemplateName = "RemovedCLAManagerTemplate"
	// RemovedCLAManagerTemplate includes the email template for email when user is removed as CLA Manager
	RemovedCLAManagerTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the CLA Group {{.CLAGroupName}}.</p>
<p>You have been removed as a CLA Manager from {{.CompanyName}} for the CLA Group {{.CLAGroupName}}.</p>
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
func RenderRemovedCLAManagerTemplate(svc EmailTemplateService, claGroupModelVersion string, params RemovedCLAManagerTemplateParams) (string, error) {
	return RenderTemplate(claGroupModelVersion, RemovedCLAManagerTemplateName, RemovedCLAManagerTemplate, params)
}

// RequestAccessToCLAManagersTemplateParams is email params for RequestAccessToCLAManagersTemplate
type RequestAccessToCLAManagersTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	RequesterName  string
	RequesterEmail string
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
<p>If you want to permit this, please log into the <a href="{{.CorporateConsole}}" target="_blank">EasyCLA Corporate Console</a>,
select your company, then select the {{.Project.ExternalProjectName}} project. From the CLA Manager requests, you can approve this user as an
additional CLA Manager.</p>
`
)

// RenderRequestAccessToCLAManagersTemplate renders the RemovedCLAManagerTemplate
func RenderRequestAccessToCLAManagersTemplate(svc EmailTemplateService, claGroupModelVersion, projectSFID string, params RequestAccessToCLAManagersTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(claGroupModelVersion, projectSFID)
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams

	return RenderTemplate(claGroupModelVersion, RequestAccessToCLAManagersTemplateName, RequestAccessToCLAManagersTemplate, params)
}

// RequestApprovedToCLAManagersTemplateParams is email params for RequestApprovedToCLAManagersTemplate
type RequestApprovedToCLAManagersTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
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

// RenderRequestApprovedToCLAManagersTemplate renders the RemovedCLAManagerTemplate
func RenderRequestApprovedToCLAManagersTemplate(svc EmailTemplateService, claGroupModelVersion, projectSFID string, params RequestApprovedToCLAManagersTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(claGroupModelVersion, projectSFID)
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams

	return RenderTemplate(claGroupModelVersion, RequestApprovedToCLAManagersTemplateName, RequestApprovedToCLAManagersTemplate, params)
}

// RequestApprovedToRequesterTemplateParams email template params for RequestApprovedToRequesterTemplate
type RequestApprovedToRequesterTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
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
<p> To get started, please log into the <a href="{{.CorporateConsole}}" target="_blank">EasyCLA Corporate Console</a>, and select your
company and then the project {{.Project.ExternalProjectName}}. From here you will be able to edit the list of approved employees and CLA Managers.</p>
`
)

// RenderRequestApprovedToRequesterTemplate renders the RemovedCLAManagerTemplate
func RenderRequestApprovedToRequesterTemplate(svc EmailTemplateService, claGroupModelVersion, projectSFID string, params RequestApprovedToRequesterTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(claGroupModelVersion, projectSFID)
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams

	return RenderTemplate(claGroupModelVersion, RequestApprovedToRequesterTemplateName, RequestApprovedToRequesterTemplate, params)
}

// RequestDeniedToCLAManagersTemplateParams is email params for RequestDeniedToCLAManagersTemplate
type RequestDeniedToCLAManagersTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
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

// RenderRequestDeniedToCLAManagersTemplate renders the RemovedCLAManagerTemplate
func RenderRequestDeniedToCLAManagersTemplate(svc EmailTemplateService, claGroupModelVersion, projectSFID string, params RequestDeniedToCLAManagersTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(claGroupModelVersion, projectSFID)
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams

	return RenderTemplate(claGroupModelVersion, RequestDeniedToCLAManagersTemplateName, RequestDeniedToCLAManagersTemplate, params)
}

// RequestDeniedToRequesterTemplateParams is email params for RequestDeniedToRequesterTemplate
type RequestDeniedToRequesterTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
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

// RenderRequestDeniedToRequesterTemplate renders the RemovedCLAManagerTemplate
func RenderRequestDeniedToRequesterTemplate(svc EmailTemplateService, claGroupModelVersion, projectSFID string, params RequestDeniedToRequesterTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(claGroupModelVersion, projectSFID)
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams

	return RenderTemplate(claGroupModelVersion, RequestDeniedToRequesterTemplateName, RequestDeniedToRequesterTemplate, params)
}

// ClaManagerAddedEToUserTemplateParams is email params
type ClaManagerAddedEToUserTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
}

const (
	// ClaManagerAddedEToUserTemplateName is template name of ClaManagerAddedEToUserTemplate
	ClaManagerAddedEToUserTemplateName = "V2ClaManagerAddedEToUserTemplate"
	//ClaManagerAddedEToUserTemplate email template for cla manager v2
	ClaManagerAddedEToUserTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the  CLA Group {{.CLAGroupName}}.</p>
<p>You have been added as a CLA Manager for the organization {{.CompanyName}} and the CLAGroup {{.CLAGroupName}}.  This means that you can now maintain the
list of employees allowed to contribute to the CLA Group {{.CLAGroupName}} on behalf of your company, as well as view and manage the list of your
company’s CLA Managers for the CLA Group {{.CLAGroupName}}.</p>
<p> To get started, please log into the <a href="{{.CorporateConsole}}" target="_blank">EasyCLA Corporate Console</a>, and select your
company and then the project {{.CLAGroupName}}. From here you will be able to edit the list of approved employees and CLA Managers.</p>
`
)

// RenderClaManagerAddedEToUserTemplate renders the RemovedCLAManagerTemplate
func RenderClaManagerAddedEToUserTemplate(svc EmailTemplateService, claGroupModelVersion, projectSFID string, params ClaManagerAddedEToUserTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(claGroupModelVersion, projectSFID)
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams

	return RenderTemplate(claGroupModelVersion, ClaManagerAddedEToUserTemplateName, ClaManagerAddedEToUserTemplate, params)
}

// ClaManagerAddedToCLAManagersTemplateParams is email params for ClaManagerAddedToCLAManagersTemplate
type ClaManagerAddedToCLAManagersTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	Name        string
	Email       string
	ProjectSFID string
}

const (
	// ClaManagerAddedToCLAManagersTemplateName is email template name for ClaManagerAddedToCLAManagersTemplate
	ClaManagerAddedToCLAManagersTemplateName = "ClaManagerAddedToCLAManagersTemplate"
	// ClaManagerAddedToCLAManagersTemplate is email template for
	ClaManagerAddedToCLAManagersTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the CLA Group {{.CLAGroupName}}.</p>
<p>The following user has been added as a CLA Manager from {{.CompanyName}} for the CLA Group {{.CLAGroupName}}. This means that they can now
maintain the list of employees allowed to contribute to {{.CLAGroupName}} on behalf of your company, as well as view and manage the
list of company’s CLA Managers for CLA Group {{.CLAGroupName}}.</p>
<ul>
<li>{{.Name}} ({{.Email}})</li>
</ul>
`
)

// RenderClaManagerAddedToCLAManagersTemplate renders the ClaManagerAddedToCLAManagersTemplate
func RenderClaManagerAddedToCLAManagersTemplate(svc EmailTemplateService, claGroupModelVersion, claGroupName string, params ClaManagerAddedToCLAManagersTemplateParams) (string, error) {
	// claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(claGroupModelVersion, projectSFID)
	// if err != nil {
	// 	return "", err
	// }
	// params.CLAGroupTemplateParams = claGroupParams
	params.CLAGroupTemplateParams = CLAGroupTemplateParams{
		CLAGroupName: claGroupName,
	}

	return RenderTemplate(claGroupModelVersion, ClaManagerAddedToCLAManagersTemplateName, ClaManagerAddedToCLAManagersTemplate, params)
}

// ClaManagerDeletedToCLAManagersTemplateParams is template params for ClaManagerDeletedToCLAManagersTemplate
type ClaManagerDeletedToCLAManagersTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	Name  string
	Email string
}

const (
	// ClaManagerDeletedToCLAManagersTemplateName is template name for ClaManagerDeletedToCLAManagersTemplate
	ClaManagerDeletedToCLAManagersTemplateName = "ClaManagerDeletedToCLAManagersTemplate"
	// ClaManagerDeletedToCLAManagersTemplate is template for
	ClaManagerDeletedToCLAManagersTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the CLA Group {{.CLAGroupName}}.</p>
<p>{{.Name}} ({{.Email}}) has been removed as a CLA Manager from {{.CompanyName}} for CLA Group {{.CLAGroupName}}.</p>
`
)

// RenderClaManagerDeletedToCLAManagersTemplate renders the RemovedCLAManagerTemplate
func RenderClaManagerDeletedToCLAManagersTemplate(svc EmailTemplateService, claGroupModelVersion, claGroupName string) (string, error) {

	params := CLAGroupTemplateParams{
		CLAGroupName: claGroupName,
	}

	return RenderTemplate(claGroupModelVersion, ClaManagerDeletedToCLAManagersTemplateName, ClaManagerDeletedToCLAManagersTemplate, params)
}
