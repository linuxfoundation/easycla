// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

func TestCCLAInvalidateSignatureTemplate(t *testing.T) {
	params := signatures.InvalidateSignatureTemplateParams{
		RecipientName:   "CCLATest",
		ClaType:         utils.ClaTypeCCLA,
		ClaManager:      "claManager",
		CLAGroupName:    "claGroup test",
		RemovalCriteria: "email removal",
		CLAManagers: []signatures.ClaManagerInfoParams{
			{Username: "mgr_one", Email: "mgr_one_email"},
			{Username: "mgr_two", Email: "mgr_two_email"},
		},
		Company: "TestCompany",
	}

	result, err := utils.RenderTemplate(utils.V2, signatures.InvalidateCCLASignatureTemplateName, signatures.InvalidateCCLASignatureTemplate, params)
	assert.NoError(t, err)
	assert.Contains(t, result, "This is a notification email from EasyCLA regarding the CLA Group claGroup test")
	assert.Contains(t, result, "You were previously authorized to contribute on behalf of your company TestCompany under its CLA. However, a CLA Manager claManager has now removed you from the authorization list")
	assert.Contains(t, result, "<li>mgr_one mgr_one_email</li>")

}
