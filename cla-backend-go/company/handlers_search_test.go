// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/organization"
	"github.com/stretchr/testify/assert"
)

func TestShortWebsiteSearch(t *testing.T) {
	tests := []struct {
		name   string
		params organization.SearchOrganizationParams
		want   bool
	}{
		{"a website of one character alone", organization.SearchOrganizationParams{WebsiteName: aws.String(" a ")}, true},
		{"an empty website alone", organization.SearchOrganizationParams{WebsiteName: aws.String("")}, true},
		{"a website of three characters", organization.SearchOrganizationParams{WebsiteName: aws.String("a.b")}, false},
		{"a short website next to a company name", organization.SearchOrganizationParams{CompanyName: aws.String("Acme"), WebsiteName: aws.String("a")}, false},
		{"a short website next to a filter", organization.SearchOrganizationParams{DollarFilter: aws.String("name eq Acme"), WebsiteName: aws.String("a")}, false},
		{"no website", organization.SearchOrganizationParams{CompanyName: aws.String("Acme")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shortWebsiteSearch(tt.params))
		})
	}
}
