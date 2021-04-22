// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

// GithubRepositoryActionTemplateParams is email params for GithubRepositoryActionTemplate
type GithubRepositoryActionTemplateParams struct {
	CommonEmailParams
	CLAGroupTemplateParams
	RepositoryName string
}

// GithubRepositoryDisabledTemplateParams is email params for GithubRepositoryDisabledTemplate
type GithubRepositoryDisabledTemplateParams struct {
	GithubRepositoryActionTemplateParams
	GithubAction string
}

const (
	// GithubRepositoryDisabledTemplateName is email template name for GithubRepositoryDisabledTemplate
	GithubRepositoryDisabledTemplateName = "GithubRepositoryDisabledTemplate"
	// GithubRepositoryDisabledTemplate is email template for
	GithubRepositoryDisabledTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the Github Repository {{.RepositoryName}} associated with the CLA Group {{.CLAGroupName}}.</p>
<p>EasyCLA was notified that the Github Repository {{.RepositoryName}} was {{.GithubAction}} from Github. It's now disabled from EasyCLA platform.</p>
`
)

// RenderGithubRepositoryDisabledTemplate renders GithubRepositoryDisabledTemplate
func RenderGithubRepositoryDisabledTemplate(svc EmailTemplateService, claGroupID string, params GithubRepositoryDisabledTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromCLAGroup(claGroupID)
	if err != nil {
		return "", err
	}

	// assign the prefilled struct
	params.CLAGroupTemplateParams = claGroupParams
	return RenderTemplate(params.CLAGroupTemplateParams.Version, GithubRepositoryDisabledTemplateName, GithubRepositoryDisabledTemplate, params)
}

// GithubRepositoryArchivedTemplateParams renders GithubRepositoryArchivedTemplate
type GithubRepositoryArchivedTemplateParams struct {
	GithubRepositoryActionTemplateParams
}

const (
	// GithubRepositoryArchivedTemplateName is email template name for GithubRepositoryArchivedTemplate
	GithubRepositoryArchivedTemplateName = "GithubRepositoryArchivedTemplate"
	// GithubRepositoryArchivedTemplate is email template for
	GithubRepositoryArchivedTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the Github Repository {{.RepositoryName}} associated with the CLA Group {{.CLAGroupName}}.</p>
<p>EasyCLA was notified that the Github Repository {{.RepositoryName}} was archived from Github. No action was taken on EasyCLA platform.</p>
`
)

// RenderGithubRepositoryArchivedTemplate renders GithubRepositoryArchivedTemplate
func RenderGithubRepositoryArchivedTemplate(svc EmailTemplateService, claGroupID string, params GithubRepositoryArchivedTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromCLAGroup(claGroupID)
	if err != nil {
		return "", err
	}

	// assign the prefilled struct
	params.CLAGroupTemplateParams = claGroupParams
	return RenderTemplate(params.CLAGroupTemplateParams.Version, GithubRepositoryArchivedTemplateName, GithubRepositoryArchivedTemplate, params)
}

// GithubRepositoryRenamedTemplateParams is email params for GithubRepositoryRenamedTemplate
type GithubRepositoryRenamedTemplateParams struct {
	GithubRepositoryActionTemplateParams
	OldRepositoryName string
	NewRepositoryName string
}

const (
	// GithubRepositoryRenamedTemplateName is email template name for GithubRepositoryRenamedTemplate
	GithubRepositoryRenamedTemplateName = "GithubRepositoryRenamedTemplate"
	// GithubRepositoryRenamedTemplate is email template for
	GithubRepositoryRenamedTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the Github Repository {{.RepositoryName}} associated with the CLA Group {{.CLAGroupName}}.</p>
<p>EasyCLA was notified that the Github Repository {{.OldRepositoryName}} was renamed to {{.NewRepositoryName}} from Github. The change was reflected to EasyCLA platform.</p>
`
)

// RenderGithubRepositoryRenamedTemplate renders GithubRepositoryRenamedTemplate
func RenderGithubRepositoryRenamedTemplate(svc EmailTemplateService, claGroupID string, params GithubRepositoryRenamedTemplateParams) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromCLAGroup(claGroupID)
	if err != nil {
		return "", err
	}

	// assign the prefilled struct
	params.CLAGroupTemplateParams = claGroupParams
	return RenderTemplate(params.CLAGroupTemplateParams.Version, GithubRepositoryRenamedTemplateName, GithubRepositoryRenamedTemplate, params)
}

// GithubRepositoryTransferredTemplateParams is email params GithubRepositoryTransferredTemplate
type GithubRepositoryTransferredTemplateParams struct {
	GithubRepositoryActionTemplateParams
	OldGithubOrgName string
	NewGithubOrgName string
}

const (
	// GithubRepositoryTransferredTemplateName is email template name for GithubRepositoryTransferredTemplate
	GithubRepositoryTransferredTemplateName = "GithubRepositoryTransferredTemplate"
	// GithubRepositoryTransferredTemplate is email template for
	GithubRepositoryTransferredTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the Github Repository {{.RepositoryName}} associated with the CLA Group {{.CLAGroupName}}.</p>
<p>EasyCLA was notified that the Github Repository {{.RepositoryName}} was transferred from {{.OldGithubOrgName}} Organization to {{.NewGithubOrgName}} Organization from Github. The change was reflected to EasyCLA platform.</p>
`
)

const (
	// GithubRepositoryTransferredFailedTemplateName is email template name for GithubRepositoryTransferredFailedTemplate
	GithubRepositoryTransferredFailedTemplateName = "GithubRepositoryTransferredFailedTemplate"
	// GithubRepositoryTransferredFailedTemplate is email template for
	GithubRepositoryTransferredFailedTemplate = `
<p>Hello {{.RecipientName}},</p>
<p>This is a notification email from EasyCLA regarding the Github Repository {{.RepositoryName}} associated with the CLA Group {{.CLAGroupName}}.</p>
<p>EasyCLA was notified that the Github Repository {{.RepositoryName}} was transferred from {{.OldGithubOrgName}} Organization to {{.NewGithubOrgName}} Organization from Github.</p>
<p>However, we detected that EasyCLA is not enabled for the new Github Organization {{.NewGithubOrgName}}. The Github Repository {{.RepositoryName}} is now disabled from EasyCLA platform.</p>
`
)

// RenderGithubRepositoryTransferredTemplate renders GithubRepositoryTransferredFailedTemplate or GithubRepositoryTransferredTemplate
func RenderGithubRepositoryTransferredTemplate(svc EmailTemplateService, claGroupID string, params GithubRepositoryTransferredTemplateParams, success bool) (string, error) {
	claGroupParams, err := svc.GetCLAGroupTemplateParamsFromCLAGroup(claGroupID)
	if err != nil {
		return "", err
	}

	// assign the prefilled struct
	params.CLAGroupTemplateParams = claGroupParams
	if success {
		return RenderTemplate(params.CLAGroupTemplateParams.Version, GithubRepositoryTransferredTemplateName, GithubRepositoryTransferredTemplate, params)
	}
	return RenderTemplate(params.CLAGroupTemplateParams.Version, GithubRepositoryTransferredFailedTemplateName, GithubRepositoryTransferredFailedTemplate, params)

}
