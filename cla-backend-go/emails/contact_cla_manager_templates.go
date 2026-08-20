// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// ContactClaManagerTemplateParams is email params for ContactClaManagerTemplate
type ContactClaManagerTemplateParams struct {
	RequestAction       string
	ContributorName     string
	ContributorIdentity string
	CompanyName         string
	ProjectName         string
	CLAGroupName        string
	OptionalMessage     string
}

const (
	// ContactClaManagerTemplateName is email template name for ContactClaManagerTemplate
	ContactClaManagerTemplateName = "ContactClaManagerTemplate"
	// ContactClaManagerTemplate is the email sent to the selected CLA managers when a
	// contributor requests removal from or (re-)approval under the company CCLA
	ContactClaManagerTemplate = `
<p>Hello CLA Manager,</p>
<p>This is a notification email from EasyCLA regarding the project {{.ProjectName}} and CLA Group {{.CLAGroupName}}.</p>
<p>{{.ContributorName}} ({{.ContributorIdentity}}) has requested {{.RequestAction}} for their employee acknowledgement
under the {{.CompanyName}} corporate CLA. You are receiving this message as a CLA Manager from {{.CompanyName}} for {{.ProjectName}}.</p>
{{if .OptionalMessage}}
<p>The contributor included the following message in the request:</p>
<p>{{.OptionalMessage}}</p>
{{end}}
<p>To act on this request, please log into the EasyCLA Corporate Console and update the Approved List for {{.CompanyName}} accordingly.
No change has been made automatically.</p>
`
)

// RenderContactClaManagerTemplate renders ContactClaManagerTemplate
func RenderContactClaManagerTemplate(params ContactClaManagerTemplateParams) (string, error) {
	return RenderTemplate(utils.V2, ContactClaManagerTemplateName, ContactClaManagerTemplate, params)
}
