// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

//InvalidateSignatureTemplateParams representing params when invalidating icla/ecla
type InvalidateSignatureTemplateParams struct {
	RecipientName   string
	ClaType         string
	ClaManager      string
	RemovalCriteria string
}

const (
	//InvalidateSignatureTemplateName is email template for InvalidateSignatureTemplate
	InvalidateSignatureTemplateName = "InvalidateSignatureTemplate"
	//InvalidateSignatureTemplate ...
	InvalidateSignatureTemplate = `
	<p>Hello {{.RecipientName}}</p>
    <p>This is a notification email from EasyCLA regarding approval list removal for {{.RemovalCriteria}}</p>
	<p>Due to this change your individual contribution authorization has been removed and you will be blocked from making subsequent code contributions on behalf of this project.</p>
	`
)
