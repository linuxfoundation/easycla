// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

func TestRequestCompanyAdminBodyEscapesCallerContent(t *testing.T) {
	body := requestCompanyAdminBody("<script>alert(1)</script>", "Proj & Co", "Mal<b>ory", "mal@example.com\" onclick=\"x", "Acme <Inc>", "https://corp.example/?a=1&b=2")
	assert.NotContains(t, body, "<script>")
	assert.Contains(t, body, "Hello &lt;script&gt;alert(1)&lt;/script&gt;,")
	assert.Contains(t, body, "Proj &amp; Co")
	assert.Contains(t, body, "Mal&lt;b&gt;ory")
	assert.Contains(t, body, "mal@example.com&#34; onclick=&#34;x")
	assert.Contains(t, body, "Acme &lt;Inc&gt;")
	assert.Contains(t, body, "https://corp.example/?a=1&amp;b=2")
}

func TestRequestCompanyAdminBodyRendersPlainValuesInPlace(t *testing.T) {
	body := requestCompanyAdminBody("Jane Manager", "Project X", "Chris Contributor", "chris@example.com", "Acme Inc", "https://corporate.example/")
	assert.Contains(t, body, "<p>Hello Jane Manager,</p>")
	assert.Contains(t, body, "regarding the project Project X.")
	assert.Contains(t, body, "Chris Contributor (chris@example.com) has designated you")
	assert.Contains(t, body, "from Acme Inc to Project X.")
	assert.Contains(t, body, "Corporate Console at https://corporate.example/ to begin")
	assert.Contains(t, body, utils.GetEmailHelpContent(true), "the trusted footer is rendered as is")
	assert.Contains(t, body, utils.GetEmailSignOffContent())
}
