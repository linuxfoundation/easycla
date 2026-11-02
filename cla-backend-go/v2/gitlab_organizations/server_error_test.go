// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_organizations

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/go-openapi/runtime"
	"github.com/stretchr/testify/assert"
)

// the installation error page is served as text/html, so the group name and the error text (which carries the
// caller's state parameter) are escaped (ops#827); benign values render unchanged
func TestServerErrorPageEscapesDynamicText(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewServerError("req-1", `acme<b>`, errors.New(`invalid state variable passed : "><script>alert(1)</script>`)).WriteResponse(recorder, runtime.TextProducer())
	page := recorder.Body.String()
	assert.Equal(t, "text/html", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "req-1", recorder.Header().Get("X-REQUEST-ID"))
	assert.Contains(t, page, "Unable to install the GitLab Group acme&lt;b&gt; due to the following error: invalid state variable passed : &#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;.")
	assert.NotContains(t, page, "<script>alert(1)</script>")
	assert.NotContains(t, page, "acme<b>")

	benign := httptest.NewRecorder()
	NewServerError("req-2", "acme-group", errors.New("group not found")).WriteResponse(benign, runtime.TextProducer())
	assert.Contains(t, benign.Body.String(), "Unable to install the GitLab Group acme-group due to the following error: group not found.")
}
