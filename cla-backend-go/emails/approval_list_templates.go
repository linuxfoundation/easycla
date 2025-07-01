// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"errors"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// ApprovalListRejectedTemplateParams is email params for ApprovalListRejectedTemplate
type ApprovalListRejectedTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	CLAManagers []ClaManagerInfoParams
}

const (
	// ApprovalListRejectedTemplateName is email template name for ApprovalListRejectedTemplate
	ApprovalListRejectedTemplateName = "ApprovalListRejectedTemplate"
	// ApprovalListRejectedTemplate is email template for
	ApprovalListRejectedTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project {{.Project.ExternalProjectName}}.</p>
<p>Your request to get added to the approval list from {{.CompanyName}} for {{.Project.ExternalProjectName}} was denied by one of the existing CLA Managers.
If you have further questions about this denial, please contact one of the existing CLA Managers from
{{.CompanyName}} for {{.CompanyName}}:</p>
<ul>
	{{range .CLAManagers}}
		<li>{{.LfUsername}} {{.Email}}</li>
	{{end}}
</ul>
`
)

// RenderApprovalListRejectedTemplate renders RequestToAuthorizeTemplate
func RenderApprovalListRejectedTemplate(svc EmailTemplateService, claGroupVersion string, projectSFID string, params ApprovalListRejectedTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(claGroupVersion, projectSFID)
	if err != nil {
		return "", err
	}

	// assign the prefilled struct
	params.CLAGroupTemplateParams = claGroupParams
	return RenderTemplate(claGroupVersion, ApprovalListRejectedTemplateName, ApprovalListRejectedTemplate,
		params,
	)

}

// ApprovalListApprovedTemplateParams is email params for Approval
type ApprovalListApprovedTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	Approver string
}

const (
	// ApprovalListApprovedTemplateName is email template name for ApprovalListRejectedTemplate
	ApprovalListApprovedTemplateName = "ApprovalListApprovedTemplate"
	// ApprovalListApprovedTemplate is email template for
	ApprovalListApprovedTemplate = `
		<p>Hello {{.RecipientName}},</p>
		<p>This is a notification email from EasyCLA regarding the CLA Group {{.CLAGroupName}}.</p>
		<p>You have been added to the Approval list of {{.CompanyName}} for {{.CLAGroupName}} by CLA Manager {{.Approver}}. 
		<p>This means that you are authorized to contribute to the any of the following project(s) associated with the CLA Group {{.CLAGroupName}}: {{.GetProjectsOrProject}}</p>
		<p>If you had previously submitted a pull request to any any the above project(s) that had failed, you can now go back to it and follow the link to verify with your organization.</p>
		`
)

// RenderApprovalListTemplate renders RenderApprovalListTemplate
func RenderApprovalListTemplate(svc EmailTemplateService, projectSFIDs []string, params ApprovalListApprovedTemplateParams) (string, error) {
	if len(projectSFIDs) == 0 {
		return "", errors.New("projectSFIDs list is empty")
	}

	// prefill the projects data
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(utils.V2, projectSFIDs[0])
	if err != nil {
		return "", err
	}
	params.CLAGroupTemplateParams = claGroupParams

	return RenderTemplate(utils.V2, ApprovalListApprovedTemplateName, ApprovalListApprovedTemplate, params)
}

// RequestToAuthorizeTemplateParams is email params for RequestToAuthorizeTemplate
type RequestToAuthorizeTemplateParams struct {
	CommonEmailParams
	// This field is prefilled most of the time with EmailService
	CLAGroupTemplateParams
	CLAManagers      []ClaManagerInfoParams
	ContributorName  string
	ContributorEmail string
	OptionalMessage  string
	CompanyID        string
}

const (
	// RequestToAuthorizeTemplateName is email template name for RequestToAuthorizeTemplate
	RequestToAuthorizeTemplateName = "RequestToAuthorizeTemplate"
	// RequestToAuthorizeTemplate is email template for
	RequestToAuthorizeTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the project {{.GetProjectNameOrFoundation}} and CLA Group {{.CLAGroupName}}.</p>
<p>{{.ContributorName}} ({{.ContributorEmail}}) has requested to be added to the Approved List as an authorized contributor from
{{.CompanyName}} to the project {{.Project.ExternalProjectName}}. You are receiving this message as a CLA Manager from {{.CompanyName}} for
{{.Project.ExternalProjectName}}.</p>
{{if .OptionalMessage}}
<p>{{.ContributorName}} included the following message in the request:</p>
<br/><p>{{.OptionalMessage}}</p><br/>
{{end}}
<p>If you want to add them to the Approved List, please
<a href="{{.CorporateConsole}}#/company/{{.CompanyID}}" target="_blank">log into the EasyCLA Corporate
Console</a>, where you can approve this user's request by selecting the 'Manage Approved List' and adding the
contributor's email, the contributor's entire email domain, their GitHub ID or the entire GitHub Organization for the
repository. This will permit them to begin contributing to {{.Project.ExternalProjectName}} on behalf of {{.CompanyName}}.</p>
<p>If you are not certain whether to add them to the Approved List, please reach out to them directly to discuss.</p>
`
)

// RenderRequestToAuthorizeTemplate renders RequestToAuthorizeTemplate
func RenderRequestToAuthorizeTemplate(svc EmailTemplateService, claGroupVersion string, projectSFID string, params RequestToAuthorizeTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(claGroupVersion, projectSFID)
	if err != nil {
		return "", err
	}

	// assign the prefilled struct
	params.CLAGroupTemplateParams = claGroupParams
	return RenderTemplate(claGroupVersion, RequestToAuthorizeTemplateName, RequestToAuthorizeTemplate, params)
}
