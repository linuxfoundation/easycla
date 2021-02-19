// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"testing"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

func TestApprovalListRejectedTemplate(t *testing.T) {
	params := ApprovalListRejectedTemplateParams{
		CLAManagerTemplateParams: CLAManagerTemplateParams{
			RecipientName: "JohnsClaManager",
			Project:       CLAProjectParams{ExternalProjectName: "JohnsProject"},
			CompanyName:   "JohnsCompany",
			CLAManagers: []ClaManagerInfoParams{
				{LfUsername: "LFUserName", Email: "LFEmail"},
			},
		},
	}

	result, err := RenderTemplate(utils.V1, ApprovalListRejectedTemplateName, ApprovalListRejectedTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "regarding the project JohnsProject")
	assert.Contains(t, result, "approval list from JohnsCompany for JohnsProject")
	assert.Contains(t, result, "<li>LFUserName LFEmail</li>")
}

func TestRequestToAuthorizeTemplate(t *testing.T) {
	params := RequestToAuthorizeTemplateParams{
		CLAManagerTemplateParams: CLAManagerTemplateParams{
			RecipientName: "JohnsClaManager",
			Project:       CLAProjectParams{ExternalProjectName: "JohnsProjectExternal"},
			CLAGroupName:  "JohnsProject",
			CompanyName:   "JohnsCompany",
			CLAManagers: []ClaManagerInfoParams{
				{LfUsername: "LFUserName", Email: "LFEmail"},
			},
		},
		ContributorName:     "ContributorNameValue",
		ContributorEmail:    "ContributorEmailValue",
		CorporateConsoleURL: "CorporateConsoleURLValue",
		CompanyID:           "CompanyIDValue",
	}

	result, err := RenderTemplate(utils.V1, RequestToAuthorizeTemplateName, RequestToAuthorizeTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello JohnsClaManager")
	assert.Contains(t, result, "regarding the project JohnsProjectExternal and CLA Group JohnsProject")
	assert.Contains(t, result, "ContributorNameValue (ContributorEmailValue) has requested")
	assert.Contains(t, result, "<a href=\"https://CorporateConsoleURLValue#/company/CompanyIDValue\" target=\"_blank\">")
	assert.Contains(t, result, "contributing to JohnsProjectExternal on behalf of JohnsCompany")

	params.OptionalMessage = "OptionalMessageValue"
	result, err = RenderTemplate(utils.V1, RequestToAuthorizeTemplateName, RequestToAuthorizeTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "ContributorNameValue included the following message")
	assert.Contains(t, result, "<br/><p>OptionalMessageValue</p><br/>")

}
