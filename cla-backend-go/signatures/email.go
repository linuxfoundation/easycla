// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

// ClaManagerInfoParams represents the CLAManagerInfo used inside of the Email Templates
type ClaManagerInfoParams struct {
	Username string
	Email    string
}

// InvalidateSignatureTemplateParams representing params when invalidating icla/ecla
type InvalidateSignatureTemplateParams struct {
	RecipientName   string
	ClaType         string
	ClaManager      string
	RemovalCriteria string
	ProjectName     string
	ProjectManager  string
	CLAManagers     []ClaManagerInfoParams
	CLaManager      string
	CLAGroupName    string
	Company         string
}

const (
	//InvalidateCCLAICLASignatureTemplateName is email template for InvalidateSignatureTemplate
	InvalidateCCLAICLASignatureTemplateName = "InvalidateSignatureTemplate"
	//InvalidateCCLAICLASignatureTemplate ...
	InvalidateCCLAICLASignatureTemplate = `
	<p>Hello {{.RecipientName}}</p>
    <p>This is a notification email from EasyCLA regarding the CLA Group {{.ProjectName}}</p>
	<p>You were previously authorized to contribute on behalf of your company {{COMPANY-NAME}} under its CLA. However, a CLA Manager has now removed you from the authorization list. This has additionally resulted in invalidating your current signed Individual CLA (ICLA).</p>
	<p>As a result, you will no longer be able to contribute until you are again authorized under another signed CLA.</p>
	<p>Please contact one of the CLA Managers from your company if you have questions about why you were removed. The CLA Managers from your company for this CLA are:</p>
	<ul>
	{{range .CLAManagers}}
		<li>{{.Username}} {{.Email}}</li>
	{{end}}
	</ul>
	`

	//InvalidateCCLASignatureTemplateName is email template upon approval list removal for ccla use case
	InvalidateCCLASignatureTemplateName = "InvalidateCCLAICLASignatureTemplate"
	//InvalidateCCLASignatureTemplate ...
	InvalidateCCLASignatureTemplate = `
	<p>Hello {{.RecipientName}}</p>
	<p>This is a notification email from EasyCLA regarding the CLA Group {{.CLAGroupName}}.</p>
	<p>You were previously authorized to contribute on behalf of your company {{.Company}} under its CLA. However, a CLA Manager {{.ClaManager}} has now removed you from the authorization list.</p>
	<p>As a result, you will no longer be able to contribute until you are again authorized under another signed CLA.</p>
	<p>Please contact one of the CLA Managers from your company if you have questions about why you were removed. The CLA Managers from your company for this CLA are:</p>
	<ul>
	{{range .CLAManagers}}
		<li>{{.Username}} {{.Email}}</li>
	{{end}}
	</ul>
	`

	//InvalidateICLASignatureTemplateName is email template upon approval list removal for ccla use case
	InvalidateICLASignatureTemplateName = "InvalidateICLASignatureTemplate"
	//InvalidateICLASignatureTemplate ...
	InvalidateICLASignatureTemplate = `
	<p>Hello {{.RecipientName}}</p>
	<p>This is a notification email from EasyCLA regarding the CLA Group {{.CLAGroupName}}.</p>
	<p>You had previously signed an Individual CLA (ICLA) to contribute to the project on your own behalf. However, the Project Manager has marked your ICLA as invalidated. This might be because the ICLA may have been signed in error, if your contributions should have been on behalf of your employer rather than on your own behalf.</p>
	<p>As a result, you will no longer be able to contribute until you are again authorized under another signed CLA.</p>
	<p>Please contact the Project Manager for this project if you have questions about why you were removed.</p>
	</ul>
	`

	//InvalidateCCLAICLAECLASignatureTemplateName is email template upon approval list removal for ccla use case
	InvalidateCCLAICLAECLASignatureTemplateName = "InvalidateCCLAICLAECLASignatureTemplate"
	//InvalidateCCLAICLAECLASignatureTemplate ...
	InvalidateCCLAICLAECLASignatureTemplate = `
	<p>Hello {{.RecipientName}}</p>
	<p>This is a notification email from EasyCLA regarding the CLA Group {{.CLAGroupName}}.</p>
	<p>You were previously authorized to contribute on behalf of your company {{.Company}} under its CLA. However, a CLA Manager has now removed you from the authorization list. This has additionally resulted in invalidating your current signed Individual CLA (ICLA) and your acknowledgement.</p>
	<p>As a result, you will no longer be able to contribute until you are again authorized under another signed CLA.</p>
	<p>Please contact one of the CLA Managers from your company if you have questions about why you were removed. The CLA Managers from your company for this CLA are:</p>
	<ul>
	{{range .CLAManagers}}
		<li>{{.Username}} {{.Email}}</li>
	{{end}}
	</ul>
	`
)
