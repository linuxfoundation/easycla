// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

type DocumentSignedTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	ICLA    bool
	PdfLink string
}

const (
	// DocumentSignedTemplateName is email template name for DocumentSignedTemplate
	DocumentSignedTemplateName = "DocumentSignedTemplate"

	// DocumentSignedTemplate is email template for
	DocumentSignedICLATemplate = `
		<p>Hello {{.RecipientName}},</p>
		<p>This is a notification email from EasyCLA regarding the project {{.Project.ExternalProjectName}}.</p>
		<p>The CLA has now been signed. You can download the signed CLA as a PDF <a href="{{.PdfLink}}" target="_blank" alt="ICLA Document Link">here</a>.</p>
		`

	DocumentSignedCCLATemplate = `
		<p>Hello {{.RecipientName}},</p>
		<p>This is a notification email from EasyCLA regarding the project {{.Project.ExternalProjectName}}.</p>
		<p>The CLA has now been signed. You can download the signed CLA as a PDF <a href="{{.PdfLink}}" target="_blank" alt="CCLA Document Link">here</a>, or from within the <a href="{{.CorporateConsole}}" target="_blank"> EasyCLA CLA Manager console </a>.</p>
		`
)

// RenderDocumentSignedTemplate renders RenderDocumentSignedTemplate
func RenderDocumentSignedTemplate(svc EmailTemplateService, claGroupModelVersion, projectSFID string, params DocumentSignedTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromProjectSFID(claGroupModelVersion, projectSFID)
	if err != nil {
		return "", err
	}

	params.CLAGroupTemplateParams = claGroupParams
	var template string
	if params.ICLA {
		template = DocumentSignedICLATemplate
	} else {
		template = DocumentSignedCCLATemplate
	}

	return RenderTemplate(claGroupModelVersion, DocumentSignedTemplateName, template, params)
}
