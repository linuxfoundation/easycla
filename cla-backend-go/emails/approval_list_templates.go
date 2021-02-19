// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import "github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

// ApprovalListRejectedTemplateParams is email params for ApprovalListRejectedTemplate
type ApprovalListRejectedTemplateParams struct {
	CLAManagerTemplateParams
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

// RequestToAuthorizeTemplateParams is email params for RequestToAuthorizeTemplate
type RequestToAuthorizeTemplateParams struct {
	CLAManagerTemplateParams
	ContributorName     string
	ContributorEmail    string
	OptionalMessage     string
	CorporateConsoleURL string
	CompanyID           string
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
<a href="https://{{.CorporateConsoleURL}}#/company/{{.CompanyID}}" target="_blank">log into the EasyCLA Corporate
Console</a>, where you can approve this user's request by selecting the 'Manage Approved List' and adding the
contributor's email, the contributor's entire email domain, their GitHub ID or the entire GitHub Organization for the
repository. This will permit them to begin contributing to {{.Project.ExternalProjectName}} on behalf of {{.CompanyName}}.</p>
<p>If you are not certain whether to add them to the Approved List, please reach out to them directly to discuss.</p>
`
)

// RenderRequestToAuthorizeTemplate renders RequestToAuthorizeTemplate
func RenderRequestToAuthorizeTemplate(repository projects_cla_groups.Repository, claGroupVersion string, projecSFID string, params RequestToAuthorizeTemplateParams) (string, error) {
	if err := PrefillCLAManagerTemplateParamsFromClaGroup(repository, projecSFID, &params.CLAManagerTemplateParams); err != nil {
		return "", err
	}

	return RenderTemplate(claGroupVersion, RequestToAuthorizeTemplateName, RequestToAuthorizeTemplate,
		params,
	)

}
