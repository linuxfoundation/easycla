// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

const (
	lensTestBase       = "https://app.example.org"
	lensTestConsole    = "https://organization.example.org"
	lensTestCompany    = "001000000000000AAA"
	lensTestCLAGroupID = "00000000-0000-4000-8000-000000000001"
	lensTestPage       = lensTestBase + "/org/" + lensTestCompany + "/easycla/" + lensTestCLAGroupID
)

func lensTestCommon() CommonEmailParams {
	return CommonEmailParams{RecipientName: "Example Manager", CompanyName: "Example Org", CompanySFID: lensTestCompany}
}

func lensTestCLAGroup(lensBase string) CLAGroupTemplateParams {
	return CLAGroupTemplateParams{
		CorporateConsole:  lensTestConsole,
		OrgLensBase:       lensBase,
		CLAGroupID:        lensTestCLAGroupID,
		CLAGroupName:      "Example Group",
		ChildProjectCount: 1,
		Version:           utils.V2,
		Projects: []CLAProjectParams{{
			ExternalProjectName: "Example Project",
			ProjectSFID:         "a0P000000000000AAA",
			FoundationSFID:      "a0P000000000001AAA",
			CorporateConsole:    lensTestConsole,
		}},
	}
}

func TestCLAManagerEmailsLinkToOrgLens(t *testing.T) {
	tests := []struct {
		name     string
		template string
		params   func(lensBase string) interface{}
	}{
		{"request approved to requester", RequestApprovedToRequesterTemplate, func(b string) interface{} {
			return RequestApprovedToRequesterTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(b)}
		}},
		{"CLA manager added", ClaManagerAddedEToUserTemplate, func(b string) interface{} {
			return ClaManagerAddedEToUserTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(b)}
		}},
		{"contributor approval request", V2ContributorApprovalRequestTemplate, func(b string) interface{} {
			return V2ContributorApprovalRequestTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(b), UserDetails: "details"}
		}},
		{"org admin invite", V2OrgAdminTemplate, func(b string) interface{} {
			return V2OrgAdminTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(b), SenderName: "Sender", SenderEmail: "sender@example.org"}
		}},
		{"contributor to org admin", V2ContributorToOrgAdminTemplate, func(b string) interface{} {
			return V2ContributorToOrgAdminTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(b), UserDetails: "details"}
		}},
		{"designee corporate invite", V2CLAManagerDesigneeCorporateTemplate, func(b string) interface{} {
			return V2CLAManagerDesigneeCorporateTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(b), SenderName: "Sender", SenderEmail: "sender@example.org"}
		}},
		{"designee invite", V2ToCLAManagerDesigneeTemplate, func(b string) interface{} {
			return V2ToCLAManagerDesigneeTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(b), Contributor: Contributor{Email: "contributor@example.org", Username: "contributor"}}
		}},
		{"designee invite with no LF login", V2DesigneeToUserWithNoLFIDTemplate, func(b string) interface{} {
			return V2ToCLAManagerDesigneeTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(b), Contributor: Contributor{Email: "contributor@example.org", Username: "contributor"}}
		}},
		{"CLA manager invite with no LF login", V2CLAManagerToUserWithNoLFIDTemplate, func(b string) interface{} {
			return V2CLAManagerToUserWithNoLFIDTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(b), RequesterUserName: "requester", RequesterEmail: "requester@example.org"}
		}},
		{"request to authorize", RequestToAuthorizeTemplate, func(b string) interface{} {
			return RequestToAuthorizeTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(b), ContributorName: "contributor", ContributorEmail: "contributor@example.org", CompanyID: "company-id"}
		}},
		{"pending CLA manager request", RequestAccessToCLAManagersTemplate, func(b string) interface{} {
			return RequestAccessToCLAManagersTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(b), RequesterName: "Requester", RequesterEmail: "requester@example.org"}
		}},
		{"CCLA signed", DocumentSignedCCLATemplate, func(b string) interface{} {
			return DocumentSignedTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(b), PdfLink: "https://api.example.org/pdf"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lens, err := RenderTemplate(utils.V2, tt.name, tt.template, tt.params(lensTestBase))
			assert.NoError(t, err)
			assert.Contains(t, lens, `<a href="`+lensTestPage+`" target="_blank">Example Group in the Organization lens</a>`)
			assert.NotContains(t, lens, lensTestConsole)

			console, err := RenderTemplate(utils.V2, tt.name, tt.template, tt.params(""))
			assert.NoError(t, err)
			assert.Contains(t, console, lensTestConsole)
			assert.NotContains(t, console, "Organization lens")
		})
	}
}

func TestOrgLensLinkWithoutValidCompanySFIDKeepsCorporateConsole(t *testing.T) {
	for _, sfid := range []string{"", "lf0000000000000AAA", "001000000000000"} {
		t.Run(sfid, func(t *testing.T) {
			params := ClaManagerAddedEToUserTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: lensTestCLAGroup(lensTestBase)}
			params.CompanySFID = sfid

			result, err := RenderTemplate(utils.V2, ClaManagerAddedEToUserTemplateName, ClaManagerAddedEToUserTemplate, params)
			assert.NoError(t, err)
			assert.Contains(t, result, lensTestConsole)
			assert.NotContains(t, result, "Organization lens")
		})
	}
}

func TestContactClaManagerEmailLinksToOrgLens(t *testing.T) {
	params := ContactClaManagerTemplateParams{
		RequestAction:       "removal",
		ContributorName:     "contributor",
		ContributorIdentity: "contributor@example.org",
		CompanyName:         "Example Org",
		ProjectName:         "Example Project",
		CLAGroupName:        "Example Group",
		OrgLensURL:          lensTestPage,
	}
	lens, err := RenderContactClaManagerTemplate(params)
	assert.NoError(t, err)
	assert.Contains(t, lens, `<a href="`+lensTestPage+`" target="_blank">Example Group in the Organization lens</a>`)
	assert.NotContains(t, lens, "Corporate Console")

	params.OrgLensURL = ""
	console, err := RenderContactClaManagerTemplate(params)
	assert.NoError(t, err)
	assert.Contains(t, console, "log into the EasyCLA Corporate Console")
	assert.NotContains(t, console, "Organization lens")
}

func TestCLAManagerEmailsKeepTheCLAGroupOfTheSender(t *testing.T) {
	const own = "00000000-0000-4000-8000-00000000000a"
	svc := &documentSignedTemplateServiceStub{projectParams: lensTestCLAGroup(lensTestBase)}
	ownPage := lensTestBase + "/org/" + lensTestCompany + "/easycla/" + own
	group := CLAGroupTemplateParams{CLAGroupID: own, CLAGroupName: "Own Group"}
	renders := map[string]func() (string, error){
		"request access": func() (string, error) {
			return RenderRequestAccessToCLAManagersTemplate(svc, utils.V2, "a0P000000000001AAA", RequestAccessToCLAManagersTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: group})
		},
		"request approved to requester": func() (string, error) {
			return RenderRequestApprovedToRequesterTemplate(svc, utils.V2, "a0P000000000001AAA", RequestApprovedToRequesterTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: group})
		},
		"CLA manager added": func() (string, error) {
			return RenderClaManagerAddedEToUserTemplate(svc, utils.V2, "a0P000000000001AAA", ClaManagerAddedEToUserTemplateParams{CommonEmailParams: lensTestCommon(), CLAGroupTemplateParams: group})
		},
	}
	for name, render := range renders {
		t.Run(name, func(t *testing.T) {
			body, err := render()
			assert.NoError(t, err)
			assert.Contains(t, body, `<a href="`+ownPage+`" target="_blank">Own Group in the Organization lens</a>`, "the link and its name are the CLA Group of the request")
			assert.NotContains(t, body, lensTestCLAGroupID, "the CLA Group of the project lookup is not used for the link")
			assert.NotContains(t, body, "Example Group", "the name of another CLA Group is not shown")
		})
	}
	t.Run("a sender that names no CLA Group keeps the lookup", func(t *testing.T) {
		body, err := RenderRequestAccessToCLAManagersTemplate(svc, utils.V2, "a0P000000000001AAA", RequestAccessToCLAManagersTemplateParams{CommonEmailParams: lensTestCommon()})
		assert.NoError(t, err)
		assert.Contains(t, body, `<a href="`+lensTestPage+`"`)
	})
}
