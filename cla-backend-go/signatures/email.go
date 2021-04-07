// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

//InvalidateSignatureTemplateParams representing params when invalidating icla/ecla
type InvalidateSignatureTemplateParams struct {
	RecipientName   string
	ClaType         string
	ClaManager      string
	RemovalCriteria string
	ProjectName     string
}

const (
	//InvalidateSignatureTemplateName is email template for InvalidateSignatureTemplate
	InvalidateSignatureTemplateName = "InvalidateSignatureTemplate"
	//InvalidateSignatureTemplate ...
	InvalidateSignatureTemplate = `
	<p>Hello {{.RecipientName}}</p>
    <p>This is a notification email from EasyCLA regarding the claGroup {{.ProjectName}}</p>
	<p>The ICLA signature for {{.RecipientName}} has been invalidated.</p>
	<p>Please contact Project Manager for the claGroup {{.ProjectName}} and/or CLA Manager from your company if you have more questions.</p>
	`
)
