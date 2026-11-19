// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-openapi/runtime/middleware"
	"github.com/stretchr/testify/assert"

	v1Organization "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/organization"
	v2Company "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/company"
)

// both lookups of the "Add your organization" dialog accept hyphens inside the host labels (lfx-self-serve#5479)
func TestWebsiteNameAcceptsHyphenatedHosts(t *testing.T) {
	bind := map[string]func(*http.Request) error{
		"/v3/organization/search": func(r *http.Request) error {
			params := v1Organization.NewSearchOrganizationParams()
			return params.BindRequest(r, &middleware.MatchedRoute{})
		},
		"/v4/company/lookup": func(r *http.Request) error {
			params := v2Company.NewSearchCompanyLookupParams()
			return params.BindRequest(r, &middleware.MatchedRoute{})
		},
	}
	for website, valid := range map[string]bool{
		"https://www.foo-bar.com":   true,
		"foo-bar.co.uk":             true,
		"shop.foo-bar.com":          true,
		"xn--bcher-kva.example":     true,
		"foo-bar.com/about?x=1":     true,
		"https://www.example.com":   true,
		"example.com/about":         true,
		"www.linuxfoundation.org":   true,
		"-foo.com":                  false,
		"foo-.com":                  false,
		"foo.-bar.com":              false,
		"foo.bar-.com":              false,
		"https://foo_bar.com":       false,
		"https://foo-bar.com/<svg>": false,
	} {
		for path, bindRequest := range bind {
			err := bindRequest(httptest.NewRequest(http.MethodGet, path+"?websiteName="+url.QueryEscape(website), nil))
			if valid {
				assert.NoError(t, err, "%s %s", path, website)
			} else {
				assert.Error(t, err, "%s %s", path, website)
			}
		}
	}
}
