// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"testing"

	"github.com/communitybridge/easycla/cla-backend-go/emails"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

func TestV2ContributorApprovalRequestTemplate(t *testing.T) {
	params := emails.V2ContributorApprovalRequestTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			RecipientName: "JohnsClaManager",
			CompanyName:   "JohnsCompany",
		},
		CLAGroupTemplateParams: emails.CLAGroupTemplateParams{
			Projects: []emails.CLAProjectParams{
				{ExternalProjectName: "Project Spaced 1", ProjectSFID: "ProjectSFID2", FoundationSFID: "FoundationSFID2", CorporateConsole: "http://CorporateConsole.com"},
			},
			CorporateConsole: "http://CorporateConsoleV2URL.com",
		},
		UserDetails: "UserDetailsValue",
	}

	result, err := emails.RenderTemplate(utils.V1, emails.V2ContributorApprovalRequestTemplateName, emails.V2ContributorApprovalRequestTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "regarding the organization JohnsCompany")
	assert.Contains(t, result, "The following contributor would like to submit a contribution to the projects(s): Project Spaced 1")
	assert.Contains(t, result, "UserDetailsValue")
	assert.Contains(t, result, "target=\"_blank\">Project Spaced 1</a>")

	assert.Contains(t, result, "CLA Managers can visit the EasyCLA corporate console page for <a href=\"http://CorporateConsole.com/foundation/FoundationSFID2/project/ProjectSFID2/cla\" target=\"_blank\">Project Spaced 1</a>")
	assert.Contains(t, result, "and add the contributor to one of the approval lists.")

	params.SigningEntityName = "SigningEntityNameValue"

	result, err = emails.RenderTemplate(utils.V1, emails.V2ContributorApprovalRequestTemplateName, emails.V2ContributorApprovalRequestTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "regarding the organization JohnsCompany")
	assert.Contains(t, result, "UserDetailsValue")
}

func TestV2OrgAdminTemplate(t *testing.T) {
	params := emails.V2OrgAdminTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			CompanyName:   "JohnsCompany",
			RecipientName: "JohnsClaManager",
		},
		CLAGroupTemplateParams: emails.CLAGroupTemplateParams{
			Projects: []emails.CLAProjectParams{{
				ExternalProjectName: "JohnsProject",
				ProjectSFID:         "ProjectSFIDValue",
				FoundationSFID:      "FoundationSFIDValue",
				CorporateConsole:    "http://CorporateConsole.com",
			}},
			CLAGroupName:     "JohnsCLAGroupName",
			CorporateConsole: "http://CorporateConsole.com",
		},
		SenderName:  "SenderNameValue",
		SenderEmail: "SenderEmailValue",
	}

	result, err := emails.RenderTemplate(utils.V1, emails.V2OrgAdminTemplateName, emails.V2OrgAdminTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "signing process for the organization JohnsCompany")
	assert.Contains(t, result, "SenderNameValue SenderEmailValue has identified you")
	assert.Contains(t, result, "Corporate CLA in support of the following project(s):")
	assert.Contains(t, result, "<li>JohnsProject</li>")
	assert.Contains(t, result, "can login to the EasyCLA portal")
	assert.Contains(t, result, `sign the CLA for this project <a href="http://CorporateConsole.com/foundation/FoundationSFIDValue/project/ProjectSFIDValue/cla" target="_blank">JohnsProject</a>`)
}

func TestV2ContributorToOrgAdminTemplate(t *testing.T) {
	params := emails.V2ContributorToOrgAdminTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			RecipientName: "JohnsClaManager",
			CompanyName:   "JohnsCompany",
		},
		CLAGroupTemplateParams: emails.CLAGroupTemplateParams{
			Projects: []emails.CLAProjectParams{
				{ExternalProjectName: "Project1", ProjectSFID: "ProjectSFID1", FoundationSFID: "FoundationSFID1", CorporateConsole: "http://CorporateConsole.com"},
				{ExternalProjectName: "Project2", ProjectSFID: "ProjectSFID2", FoundationSFID: "FoundationSFID2", CorporateConsole: "http://CorporateConsole.com"},
			},
			CLAGroupName:     "JohnsCLAGroupName",
			CorporateConsole: "http://CorporateConsole.com",
		},

		UserDetails: "UserDetailsValue",
	}

	result, err := emails.RenderTemplate(utils.V1, emails.V2ContributorToOrgAdminTemplateName, emails.V2ContributorToOrgAdminTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "would like to submit a contribution to Project1,Project2")
	assert.Contains(t, result, "your organization must sign a CLA.")
	assert.Contains(t, result, "<p>UserDetailsValue</p>")
	assert.Contains(t, result, "Please notify the contributor once they are added so that they may complete the contribution process")
	assert.Contains(t, result, `CLA for any of the project(s): <a href="http://CorporateConsole.com/foundation/FoundationSFID1/project/ProjectSFID1/cla" target="_blank">Project1</a>,<a href="http://CorporateConsole.com/foundation/FoundationSFID2/project/ProjectSFID2/cla" target="_blank">Project2</a>`)
}

func TestV2CLAManagerDesigneeCorporateTemplate(t *testing.T) {
	params := emails.V2CLAManagerDesigneeCorporateTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			RecipientName: "JohnsClaManager",
			CompanyName:   "JohnsCompany",
		},
		CLAGroupTemplateParams: emails.CLAGroupTemplateParams{
			CLAGroupName: "JohnsCLAGroupName",
			Projects: []emails.CLAProjectParams{{
				ExternalProjectName: "JohnsProject",
				FoundationSFID:      "FoundationSFIDValue",
				ProjectSFID:         "ProjectSFIDValue",
				CorporateConsole:    "http://CorporateConsole.com",
			}},
			CorporateConsole: "http://CorporateConsole.com",
		},
		SenderName:  "SenderNameValue",
		SenderEmail: "SenderEmailValue",
	}

	result, err := emails.RenderTemplate(utils.V1, emails.V2CLAManagerDesigneeCorporateTemplateName, emails.V2CLAManagerDesigneeCorporateTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "CLA setup and signing process for the organization JohnsCompany")
	assert.Contains(t, result, "SenderNameValue SenderEmailValue has identified you")
	assert.Contains(t, result, "Corporate CLA for the organization JohnsCompany")
	assert.Contains(t, result, "<li>JohnsProject</li>")
	assert.Contains(t, result, "can login and <b>sign the CLA for this project <a href=\"http://CorporateConsole.com/foundation/FoundationSFIDValue/project/ProjectSFIDValue/cla\" target=\"_blank\">JohnsProject</a>")
}

func TestV2ToCLAManagerDesigneeTemplate(t *testing.T) {
	params := emails.V2ToCLAManagerDesigneeTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			RecipientName: "JohnsClaManager",
		},
		CLAGroupTemplateParams: emails.CLAGroupTemplateParams{
			Projects: []emails.CLAProjectParams{
				{ExternalProjectName: "Project1", ProjectSFID: "ProjectSFID1", FoundationSFID: "FoundationSFID1", CorporateConsole: "http://CorporateConsole.com"},
				{ExternalProjectName: "Project2", ProjectSFID: "ProjectSFID2", FoundationSFID: "FoundationSFID2", CorporateConsole: "http://CorporateConsole.com"},
			},
			CorporateConsole: "http://CorporateConsole.com",
		},
		Contributor: emails.Contributor{
			Email:         "ContributorEmailValue",
			Username:      "ContributorNameValue",
			EmailLabel:    utils.EmailLabel,
			UsernameLabel: utils.UserLabel,
		},
	}

	result, err := emails.RenderTemplate(utils.V1, emails.V2ToCLAManagerDesigneeTemplateName, emails.V2ToCLAManagerDesigneeTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "regarding the project(s): Project1, Project2")
	assert.Contains(t, result, "from Username: ContributorNameValue (Email Address: ContributorEmailValue)")
	assert.Contains(t, result, `CLA for any of the project(s): <a href="http://CorporateConsole.com/foundation/FoundationSFID1/project/ProjectSFID1/cla" target="_blank">Project1</a>,<a href="http://CorporateConsole.com/foundation/FoundationSFID2/project/ProjectSFID2/cla" target="_blank">Project2</a>`)

	params.Projects = []emails.CLAProjectParams{
		{ExternalProjectName: "Project1", ProjectSFID: "ProjectSFID1", FoundationSFID: "FoundationSFID1", CorporateConsole: "http://CorporateConsole.com"},
	}
	params.Contributor.EmailLabel = utils.GitHubEmailLabel
	params.Contributor.UsernameLabel = utils.GitHubUserLabel

	result, err = emails.RenderTemplate(utils.V1, emails.V2ToCLAManagerDesigneeTemplateName, emails.V2ToCLAManagerDesigneeTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "regarding the project(s): Project1")
	assert.Contains(t, result, "from GitHub Username: ContributorNameValue (GitHub Email Address: ContributorEmailValue)")
	assert.Contains(t, result, `CLA for any of the project(s): <a href="http://CorporateConsole.com/foundation/FoundationSFID1/project/ProjectSFID1/cla" target="_blank">Project1</a>`)

}

func TestV2DesigneeToUserWithNoLFIDTemplate(t *testing.T) {
	params := emails.V2ToCLAManagerDesigneeTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			RecipientName: "JohnsClaManager",
		},
		CLAGroupTemplateParams: emails.CLAGroupTemplateParams{
			Projects: []emails.CLAProjectParams{
				{ExternalProjectName: "Project1", ProjectSFID: "ProjectSFID1", FoundationSFID: "FoundationSFID1", CorporateConsole: "https://corporate.dev.lfcla.com"},
				{ExternalProjectName: "Project2", ProjectSFID: "ProjectSFID2", FoundationSFID: "FoundationSFID2", CorporateConsole: "https://corporate.dev.lfcla.com"},
			},
			CorporateConsole: "https://corporate.dev.lfcla.com",
		},

		Contributor: emails.Contributor{
			Email:         "ContributorEmail",
			Username:      "ContributorUsername",
			EmailLabel:    utils.EmailLabel,
			UsernameLabel: utils.UserLabel,
		},
	}

	result, err := emails.RenderTemplate(utils.V1, emails.V2DesigneeToUserWithNoLFIDTemplateName, emails.V2DesigneeToUserWithNoLFIDTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager,")
	assert.Contains(t, result, "We received a request from Username: ContributorUsername (Email Address: ContributorEmail)")
	assert.Contains(t, result, "After login, you will be redirected to the portal https://corporate.dev.lfcla.com ")
	assert.Contains(t, result, `where you can either sign the CLA for any of the project(s): <a href="https://corporate.dev.lfcla.com/foundation/FoundationSFID1/project/ProjectSFID1/cla" target="_blank">Project1</a>`)
	assert.Contains(t, result, "or send it to an authorized signatory for your company.")

	params.Contributor.EmailLabel = utils.GitHubEmailLabel
	params.Contributor.UsernameLabel = utils.GitHubUserLabel

	result, err = emails.RenderTemplate(utils.V1, emails.V2DesigneeToUserWithNoLFIDTemplateName, emails.V2DesigneeToUserWithNoLFIDTemplate,
		params)

	assert.NoError(t, err)
	assert.Contains(t, result, "We received a request from GitHub Username: ContributorUsername (GitHub Email Address: ContributorEmail)")
}

func TestV2CLAManagerToUserWithNoLFIDTemplate(t *testing.T) {
	params := emails.V2CLAManagerToUserWithNoLFIDTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			RecipientName: "JohnsClaManager",
			CompanyName:   "JohnsCompany",
		},
		CLAGroupTemplateParams: emails.CLAGroupTemplateParams{
			Projects: []emails.CLAProjectParams{{ExternalProjectName: "JohnsProjectExternal",
				CorporateConsole:        "http://CorporateConsole.com",
				SignedAtFoundationLevel: false,
				ProjectSFID:             "ProjectSFID",
				FoundationSFID:          "FoundationSFID",
			}},
			CLAGroupName: "JohnsCLAGroupName",
		},
		RequesterUserName: "RequesterUserNameValue",
		RequesterEmail:    "RequesterEmailValue",
	}

	result, err := emails.RenderTemplate(utils.V1, emails.V2CLAManagerToUserWithNoLFIDTemplateName, emails.V2CLAManagerToUserWithNoLFIDTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "regarding the CLA setup and signing process for the organization JohnsCompany")
	assert.Contains(t, result, "The user RequesterUserNameValue (RequesterEmailValue) has identified you as a potential candidate to setup the Corporate CLA for the organization JohnsCompany and the project JohnsProjectExternal")
	assert.Contains(t, result, "After login, you will be redirected to the portal http://CorporateConsole.com")
	assert.Contains(t, result, "After adding the contributor, please notify them")
	assert.Contains(t, result, `where you can either sign the CLA for the project: <a href="http://CorporateConsole.com/foundation/FoundationSFID/project/ProjectSFID/cla" target="_blank">JohnsProjectExternal</a>`)
}
