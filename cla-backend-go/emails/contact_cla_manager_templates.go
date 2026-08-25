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
	ContributorEmail    string
	CompanyName         string
	ProjectName         string
	CLAGroupName        string
	OptionalMessage     string
	ContactOnly         bool
}

const (
	// ContactClaManagerTemplateName is email template name for ContactClaManagerTemplate
	ContactClaManagerTemplateName = "ContactClaManagerTemplate"
	// ContactClaManagerTemplate is the email sent to the selected CLA managers when a
	// contributor requests removal/approval or sends a contact-only message
	ContactClaManagerTemplate = `
<p>Hello CLA Manager,</p>
<p>This is a notification email from EasyCLA regarding the project {{.ProjectName}} and CLA Group {{.CLAGroupName}}.</p>
{{if .ContactOnly}}
<p>{{.ContributorName}} ({{.ContributorIdentity}}) has sent you a message about their employee acknowledgement
under the {{.CompanyName}} corporate CLA. You are receiving this message as a CLA Manager from {{.CompanyName}} for {{.ProjectName}}.</p>
<p>The contributor's message:</p>
<blockquote style="white-space: pre-wrap;">{{.OptionalMessage}}</blockquote>
{{if .ContributorEmail}}
<p>You can reply to the contributor at {{.ContributorEmail}}.</p>
{{end}}
<p>This is a message only - no change was requested and none has been made.</p>
{{else}}
<p>{{.ContributorName}} ({{.ContributorIdentity}}) has requested {{.RequestAction}} for their employee acknowledgement
under the {{.CompanyName}} corporate CLA. You are receiving this message as a CLA Manager from {{.CompanyName}} for {{.ProjectName}}.</p>
{{if .OptionalMessage}}
<p>The contributor included the following message in the request:</p>
<p>{{.OptionalMessage}}</p>
{{end}}
<p>To act on this request, please log into the EasyCLA Corporate Console and update the Approved List for {{.CompanyName}} accordingly.
No change has been made automatically.</p>
{{end}}
`
)

// RenderContactClaManagerTemplate renders ContactClaManagerTemplate
func RenderContactClaManagerTemplate(params ContactClaManagerTemplateParams) (string, error) {
	return RenderTemplate(utils.V2, ContactClaManagerTemplateName, ContactClaManagerTemplate, params)
}
