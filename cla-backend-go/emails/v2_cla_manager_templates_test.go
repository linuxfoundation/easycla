// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"testing"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

func TestV2ContributorApprovalRequestTemplate(t *testing.T) {
	params := V2ContributorApprovalRequestTemplateParams{
		CLAManagerTemplateParams: CLAManagerTemplateParams{
			RecipientName: "JohnsClaManager",
			ProjectName:   "JohnsProject",
			CLAGroupName:  "JohnsCLAGroupName",
			CompanyName:   "JohnsCompany",
		},
		UserDetails:           "UserDetailsValue",
		CorporateConsoleV2URL: "http://CorporateConsoleV2URL.com",
	}

	result, err := RenderTemplate(utils.V1, V2ContributorApprovalRequestTemplateName, V2ContributorApprovalRequestTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "regarding the organization JohnsCompany")
	assert.Contains(t, result, "contribution to the JohnsCompany CLA Group JohnsCLAGroupName")
	assert.Contains(t, result, "JohnsCLAGroupName - UserDetailsValue")
	assert.Contains(t, result, "Approval can be done at http://CorporateConsoleV2URL.com")

	params.SigningEntityName = "SigningEntityNameValue"

	result, err = RenderTemplate(utils.V1, V2ContributorApprovalRequestTemplateName, V2ContributorApprovalRequestTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "regarding the organization JohnsCompany")
	assert.Contains(t, result, "contribution to the SigningEntityNameValue CLA Group JohnsCLAGroupName")
	assert.Contains(t, result, "JohnsCLAGroupName - UserDetailsValue")
	assert.Contains(t, result, "Approval can be done at http://CorporateConsoleV2URL.com")
}

func TestV2OrgAdminTemplate(t *testing.T) {
	params := V2OrgAdminTemplateParams{
		CLAManagerTemplateParams: CLAManagerTemplateParams{
			RecipientName: "JohnsClaManager",
			ProjectName:   "JohnsProject",
			CLAGroupName:  "JohnsCLAGroupName",
			CompanyName:   "JohnsCompany",
		},
		SenderName:       "SenderNameValue",
		SenderEmail:      "SenderEmailValue",
		ProjectList:      []string{"Project1", "Project2"},
		CorporateConsole: "http://CorporateConsole.com",
	}

	result, err := RenderTemplate(utils.V1, V2OrgAdminTemplateName, V2OrgAdminTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "signing process for JohnsCompany")
	assert.Contains(t, result, "SenderNameValue SenderEmailValue has identified you")
	assert.Contains(t, result, "Corporate CLA for JohnsCompany")
	assert.Contains(t, result, "<li>Project1</li>")
	assert.Contains(t, result, "<li>Project2</li>")
	assert.Contains(t, result, "can login to this portal (http://CorporateConsole.com)")
	assert.Contains(t, result, "sign the CLA for this project JohnsProject")
}

func TestV2ContributorToOrgAdminTemplate(t *testing.T) {
	params := V2ContributorToOrgAdminTemplateParams{
		CLAManagerTemplateParams: CLAManagerTemplateParams{
			RecipientName: "JohnsClaManager",
			ProjectName:   "JohnsProject",
			CLAGroupName:  "JohnsCLAGroupName",
			CompanyName:   "JohnsCompany",
		},
		ProjectNames:     []string{"Project1", "Project2"},
		UserDetails:      "UserDetailsValue",
		CorporateConsole: "http://CorporateConsole.com",
	}

	result, err := RenderTemplate(utils.V1, V2ContributorToOrgAdminTemplateName, V2ContributorToOrgAdminTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "regarding the project(s) Project1,Project2")
	assert.Contains(t, result, "sign CLA for organization: JohnsCompany")
	assert.Contains(t, result, "<p>UserDetailsValue</p>")
	assert.Contains(t, result, "Kindly login to this portal http://CorporateConsole.com")
	assert.Contains(t, result, "CLA for any of the projects Project1,Project2")
}

func TestV2CLAManagerDesigneeCorporateTemplate(t *testing.T) {
	params := V2CLAManagerDesigneeCorporateTemplateParams{
		CLAManagerTemplateParams: CLAManagerTemplateParams{
			RecipientName: "JohnsClaManager",
			ProjectName:   "JohnsProject",
			CLAGroupName:  "JohnsCLAGroupName",
			CompanyName:   "JohnsCompany",
		},
		SenderName:       "SenderNameValue",
		SenderEmail:      "SenderEmailValue",
		ProjectList:      []string{"Project1", "Project2"},
		CorporateConsole: "http://CorporateConsole.com",
	}

	result, err := RenderTemplate(utils.V1, V2CLAManagerDesigneeCorporateTemplateName, V2CLAManagerDesigneeCorporateTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "CLA setup and signing process for JohnsCompany")
	assert.Contains(t, result, "SenderNameValue SenderEmailValue has identified you")
	assert.Contains(t, result, "Corporate CLA for JohnsCompany")
	assert.Contains(t, result, "<li>Project1</li>")
	assert.Contains(t, result, "<li>Project2</li>")
	assert.Contains(t, result, "can login to this portal (http://CorporateConsole.com)")
	assert.Contains(t, result, "sign the CLA for this project JohnsProject")
}

func TestV2ToCLAManagerDesigneeTemplate(t *testing.T) {
	params := V2ToCLAManagerDesigneeTemplateParams{
		RecipientName:    "JohnsClaManager",
		ProjectNames:     []string{"Project1", "Project2"},
		ContributorID:    "ContributorIDValue",
		ContributorName:  "ContributorNameValue",
		CorporateConsole: "http://CorporateConsole.com",
	}

	result, err := RenderTemplate(utils.V1, V2ToCLAManagerDesigneeTemplateName, V2ToCLAManagerDesigneeTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "regarding the project(s) Project1,Project2")
	assert.Contains(t, result, "<p> ContributorIDValue (ContributorNameValue) </p>")
	assert.Contains(t, result, "Kindly login to this portal http://CorporateConsole.com")
	assert.Contains(t, result, "CLA for one of the project(s) Project1,Project2")

}

func TestV2DesigneeToUserWithNoLFIDTemplate(t *testing.T) {
	params := V2DesigneeToUserWithNoLFIDTemplateParams{
		CLAManagerTemplateParams: CLAManagerTemplateParams{
			RecipientName:       "JohnsClaManager",
			ProjectName:         "JohnsProject",
			ExternalProjectName: "JohnsProjectExternal",
			CLAGroupName:        "JohnsCLAGroupName",
			CompanyName:         "JohnsCompany",
		},
		RequesterUserName: "RequesterUserNameValue",
		RequesterEmail:    "RequesterEmailValue",
		CorporateConsole:  "https://corporate.dev.lfcla.com",
	}

	result, err := RenderTemplate(utils.V1, V2DesigneeToUserWithNoLFIDTemplateName, V2DesigneeToUserWithNoLFIDTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager,")
	assert.Contains(t, result, "The following contributor would like to contribute to JohnsProjectExternal on behalf of your organization: JohnsCompany.")
	assert.Contains(t, result, "Kindly login to this portal https://corporate.dev.lfcla.com and sign the CLA for the project JohnsProjectExternal.")
}

func TestV2CLAManagerToUserWithNoLFIDTemplate(t *testing.T) {
	params := V2CLAManagerToUserWithNoLFIDTemplateParams{
		CLAManagerTemplateParams: CLAManagerTemplateParams{
			RecipientName:       "JohnsClaManager",
			ProjectName:         "JohnsProject",
			ExternalProjectName: "JohnsProjectExternal",
			CLAGroupName:        "JohnsCLAGroupName",
			CompanyName:         "JohnsCompany",
		},
		RequesterUserName: "RequesterUserNameValue",
		RequesterEmail:    "RequesterEmailValue",
	}

	result, err := RenderTemplate(utils.V1, V2CLAManagerToUserWithNoLFIDTemplateName, V2CLAManagerToUserWithNoLFIDTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "regarding the Project JohnsProjectExternal and CLA Group JohnsCLAGroupName in the")
	assert.Contains(t, result, "User RequesterUserNameValue (RequesterEmailValue) was trying")
	assert.Contains(t, result, "CLA Manager for Project JohnsProject")
	assert.Contains(t, result, "notify the user RequesterUserNameValue")
}
