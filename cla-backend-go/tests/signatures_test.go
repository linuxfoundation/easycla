// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"testing"

	"github.com/communitybridge/easycla/cla-backend-go/signatures"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

//TestInvalidateSignatureTemplate validates email sent when signature is invalidated
func TestInvalidateSignatureTemplate(t *testing.T) {
	params := signatures.InvalidateSignatureTemplateParams{
		RecipientName:   "TestUser",
		ClaType:         utils.ClaTypeICLA,
		ClaManager:      "claManager",
		RemovalCriteria: "email removal",
	}

	result, err := utils.RenderTemplate(utils.V1, signatures.InvalidateSignatureTemplateName, signatures.InvalidateSignatureTemplate, params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello TestUser")
	assert.Contains(t, result, "regarding approval list removal for email removal")
	assert.Contains(t, result, "your signature record has been invalidated")

}
