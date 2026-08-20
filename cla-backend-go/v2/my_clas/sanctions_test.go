// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"fmt"
	"testing"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/sss"
	orgModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSSSClient struct {
	result  *sss.ScreeningResult
	err     error
	request sss.OrganizationStatusRequest
	calls   int
}

func (f *fakeSSSClient) GetOrganizationStatus(_ context.Context, req sss.OrganizationStatusRequest) (*sss.ScreeningResult, error) {
	f.calls++
	f.request = req
	return f.result, f.err
}

func testScreener(client sssStatusClient, required bool, org *orgModels.Organization, orgErr error) *sssScreener {
	return &sssScreener{
		client:   client,
		enabled:  true,
		required: required,
		getOrganization: func(_ context.Context, _ string) (*orgModels.Organization, error) {
			return org, orgErr
		},
	}
}

func company(sanctioned bool, origin string) *v1Models.Company {
	return &v1Models.Company{
		CompanyID:         "company-1",
		CompanyName:       "Acme Corp",
		CompanyExternalID: "0014100000Te0lqAAB",
		IsSanctioned:      sanctioned,
		SanctionOrigin:    origin,
	}
}

func TestScreenerMode(t *testing.T) {
	assert.Equal(t, models.MyClaListSssModeDisabled, NewSanctionsScreener(nil, true, false).Mode(), "an unconfigured client cannot screen")
	assert.Equal(t, models.MyClaListSssModeDisabled, (&sssScreener{client: &fakeSSSClient{}}).Mode(), "the kill switch wins")
	assert.Equal(t, models.MyClaListSssModeOptional, (&sssScreener{client: &fakeSSSClient{}, enabled: true}).Mode())
	assert.Equal(t, models.MyClaListSssModeRequired, (&sssScreener{client: &fakeSSSClient{}, enabled: true, required: true}).Mode())
}

func TestScreenCompanyLiveResult(t *testing.T) {
	org := &orgModels.Organization{Domains: " , www.acme.org "}

	t.Run("flagged", func(t *testing.T) {
		client := &fakeSSSClient{result: &sss.ScreeningResult{Status: sss.StatusFlagged}}
		flagged, check := testScreener(client, false, org, nil).ScreenCompany(context.Background(), company(false, ""))
		assert.True(t, flagged)
		assert.Equal(t, models.MyClaFlaggedCheckLive, check)
		assert.Equal(t, "acme.org", client.request.Domain)
		assert.Equal(t, "Acme Corp", client.request.OrgName)
		assert.Equal(t, "0014100000Te0lqAAB", client.request.SFDCID, "only 001-prefixed external IDs are Salesforce accounts")
	})

	t.Run("clean clears a stored sss sanction", func(t *testing.T) {
		client := &fakeSSSClient{result: &sss.ScreeningResult{Status: sss.StatusClean}}
		flagged, check := testScreener(client, true, org, nil).ScreenCompany(context.Background(), company(true, sanctionOriginSSS))
		assert.False(t, flagged)
		assert.Equal(t, models.MyClaFlaggedCheckLive, check)
	})
}

func TestScreenCompanyStoredWithoutLiveCall(t *testing.T) {
	t.Run("administrator block", func(t *testing.T) {
		client := &fakeSSSClient{result: &sss.ScreeningResult{Status: sss.StatusClean}}
		flagged, check := testScreener(client, true, nil, nil).ScreenCompany(context.Background(), company(true, "manual"))
		assert.True(t, flagged, "a non-SSS block is authoritative")
		assert.Equal(t, models.MyClaFlaggedCheckStored, check)
		assert.Zero(t, client.calls)
	})

	t.Run("screening disabled", func(t *testing.T) {
		client := &fakeSSSClient{result: &sss.ScreeningResult{Status: sss.StatusFlagged}}
		screener := testScreener(client, false, nil, nil)
		screener.enabled = false
		flagged, check := screener.ScreenCompany(context.Background(), company(true, sanctionOriginSSS))
		assert.True(t, flagged, "the persisted flag stands in when screening is off")
		assert.Equal(t, models.MyClaFlaggedCheckStored, check)
		assert.Zero(t, client.calls)
	})

	t.Run("client not configured", func(t *testing.T) {
		flagged, check := (&sssScreener{enabled: true}).ScreenCompany(context.Background(), company(false, ""))
		assert.False(t, flagged)
		assert.Equal(t, models.MyClaFlaggedCheckStored, check)
	})
}

// The listing must survive every screening failure: the answer degrades to the persisted flag
// and says so, in both required and optional mode.
func TestScreenCompanyUnavailable(t *testing.T) {
	org := &orgModels.Organization{Domains: "acme.org"}

	cases := []struct {
		name   string
		client sssStatusClient
		org    *orgModels.Organization
		orgErr error
		noID   bool
	}{
		{name: "no external id", client: &fakeSSSClient{result: &sss.ScreeningResult{Status: sss.StatusClean}}, org: org, noID: true},
		{name: "organization lookup failed", client: &fakeSSSClient{}, orgErr: fmt.Errorf("org-service down")},
		{name: "organization missing", client: &fakeSSSClient{}},
		{name: "no resolvable domain", client: &fakeSSSClient{}, org: &orgModels.Organization{}},
		{name: "sss call failed", client: &fakeSSSClient{err: fmt.Errorf("502 from sss")}, org: org},
		{name: "unexpected status", client: &fakeSSSClient{result: &sss.ScreeningResult{Status: "pending"}}, org: org},
		{name: "nil result", client: &fakeSSSClient{}, org: org},
	}

	for _, tc := range cases {
		for _, required := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/required=%v", tc.name, required), func(t *testing.T) {
				for _, stored := range []bool{false, true} {
					companyModel := company(stored, sanctionOriginSSS)
					if tc.noID {
						companyModel.CompanyExternalID = ""
					}
					flagged, check := testScreener(tc.client, required, tc.org, tc.orgErr).ScreenCompany(context.Background(), companyModel)
					assert.Equal(t, stored, flagged, "an unusable screen falls back to the persisted flag")
					assert.Equal(t, models.MyClaFlaggedCheckUnavailable, check)
				}
			})
		}
	}
}

func TestScreenCompanyNilCompany(t *testing.T) {
	flagged, check := testScreener(&fakeSSSClient{}, true, nil, nil).ScreenCompany(context.Background(), nil)
	assert.False(t, flagged)
	assert.Empty(t, check, "an unresolved employer has no sanctions answer at all")
}

func TestResolveOrgDomain(t *testing.T) {
	cases := []struct {
		org  *orgModels.Organization
		want string
	}{
		{org: &orgModels.Organization{Domains: "acme.org,acme.com"}, want: "acme.org"},
		{org: &orgModels.Organization{Domains: " , www.acme.org"}, want: "acme.org"},
		{org: &orgModels.Organization{Link: "https://www.acme.org/about"}, want: "acme.org"},
		{org: &orgModels.Organization{Link: "acme.org"}, want: "acme.org"},
		{org: &orgModels.Organization{}, want: ""},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, resolveOrgDomain(tc.org))
	}
}

func TestNewSanctionsScreenerNilClient(t *testing.T) {
	screener := NewSanctionsScreener(nil, true, false)
	require.NotNil(t, screener)
	flagged, check := screener.ScreenCompany(context.Background(), company(true, sanctionOriginSSS))
	assert.True(t, flagged, "a typed nil client must not be treated as usable")
	assert.Equal(t, models.MyClaFlaggedCheckStored, check)
}
