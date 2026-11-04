// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/sss"
	acsService "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	memberService "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func enableDirectCLARoles(t *testing.T, enabled bool) {
	t.Helper()
	original := config.GetConfig()
	configPath := filepath.Join(t.TempDir(), "config.json")
	load := func(value config.Config) {
		data, err := json.Marshal(value)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(configPath, data, 0600))
		_, err = config.LoadConfig(configPath, nil, "")
		require.NoError(t, err)
	}
	t.Cleanup(func() { load(original) })
	updated := original
	updated.ACSDirectCLARolesEnabled = enabled
	load(updated)
}

type directCLATransport func(*http.Request) (*http.Response, error)

func (f directCLATransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func directCLAResponse(r *http.Request, status int, body string) (*http.Response, error) {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

func TestMemberDomainFallbackPreservesScreeningPolicy(t *testing.T) {
	const companySFID = "0014100000Te0G7AAJ"
	for _, tc := range []struct {
		name                string
		disableDirect       bool
		knownOrg            bool
		orgFailure          bool
		memberStatus        int
		memberUID           string
		noDomain            bool
		primaryDomain       string
		sssStatus           string
		optional            bool
		disableSSS          bool
		persistedOrigin     string
		wantBlocked         bool
		wantErr             bool
		wantMember, wantSSS int
		wantDomain          string
	}{
		{name: "authoritative primary domain is screened", primaryDomain: "primary.invalid", wantBlocked: true, wantMember: 1, wantSSS: 1, wantDomain: "primary.invalid"},
		{name: "canonical matching short UID and website", memberUID: companySFID[:15], wantBlocked: true, wantMember: 1, wantSSS: 1, wantDomain: "trusted.invalid"},
		{name: "different Account is rejected", memberUID: "0014100000Te02DAAR", wantErr: true, wantMember: 1},
		{name: "missing authoritative domain fails closed", noDomain: true, wantErr: true, wantMember: 1},
		{name: "member authorization failure fails closed", memberStatus: http.StatusForbidden, wantErr: true, wantMember: 1},
		{name: "organization outage is not a missing Account", orgFailure: true, wantErr: true},
		{name: "disabled direct roles preserves missing-Org failure", disableDirect: true, wantErr: true},
		{name: "known organization preserves its domain", knownOrg: true, wantBlocked: true, wantSSS: 1, wantDomain: "platform.invalid"},
		{name: "manual block never calls upstream", persistedOrigin: "manual", wantBlocked: true},
		{name: "SSS kill switch remains authoritative", persistedOrigin: "sss", disableSSS: true},
		{name: "optional unavailable never clears stored sanction", persistedOrigin: "sss", optional: true, memberStatus: http.StatusServiceUnavailable, wantBlocked: true, wantMember: 1},
		{name: "unknown screening result never clears stored sanction", persistedOrigin: "sss", sssStatus: "pending", wantErr: true, wantMember: 1, wantSSS: 1, wantDomain: "trusted.invalid"},
		{name: "live clean result still clears only SSS origin", persistedOrigin: "sss", sssStatus: "clean", wantMember: 1, wantSSS: 1, wantDomain: "trusted.invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enableDirectCLARoles(t, !tc.disableDirect)
			status := tc.sssStatus
			if status == "" {
				status = sss.StatusFlagged
			}
			base := setupRowlessHTTP(t, status)
			if tc.knownOrg {
				base.orgs[companySFID] = `{"ID":"` + companySFID + `","Name":"Company","Domains":"platform.invalid"}`
			}
			memberCalls := 0
			var screening sss.OrganizationStatusRequest
			rowlessSwitch.set(directCLATransport(func(r *http.Request) (*http.Response, error) {
				if tc.orgFailure && r.URL.Host == rowlessPlatformHost && strings.HasPrefix(r.URL.Path, "/organization-service/") {
					return directCLAResponse(r, http.StatusInternalServerError, `{"Message":"unavailable"}`)
				}
				if r.URL.Host == "member.direct.invalid" {
					memberCalls++
					assert.Equal(t, http.MethodGet, r.Method)
					assert.Equal(t, "/b2b_orgs/"+companySFID, r.URL.Path)
					assert.Equal(t, "1", r.URL.Query().Get("v"))
					if tc.memberStatus != 0 {
						return directCLAResponse(r, tc.memberStatus, `{"message":"unavailable"}`)
					}
					uid := tc.memberUID
					if uid == "" {
						uid = companySFID
					}
					website := "https://www.trusted.invalid/path"
					if tc.noDomain {
						website = ""
					}
					body, err := json.Marshal(memberService.B2BOrg{UID: uid, Name: "Company", Website: website, PrimaryDomain: tc.primaryDomain})
					require.NoError(t, err)
					return directCLAResponse(r, http.StatusOK, string(body))
				}
				if r.URL.Host == rowlessSSSHost && r.URL.Path == rowlessSSSStatusPath {
					assert.Equal(t, http.MethodGet, r.Method)
					screening = sss.OrganizationStatusRequest{
						Domain:  r.URL.Query().Get("domain"),
						OrgName: r.URL.Query().Get("org_name"),
						SFDCID:  r.URL.Query().Get("sfdc_id"),
					}
				}
				return base.RoundTrip(r)
			}))
			member, err := memberService.NewClient(memberService.Config{
				BaseURL: "https://member.direct.invalid", Audience: "member-audience",
				OAuthTokenURL: "https://example.auth0.com", ClientID: "test-client", ClientSecret: "test-secret",
			})
			require.NoError(t, err)
			ctrl := gomock.NewController(t)
			repo := mock_company.NewMockIRepository(ctrl)
			if tc.wantSSS != 0 {
				switch status {
				case sss.StatusFlagged:
					repo.EXPECT().UpdateCompanySanctionStatus(gomock.Any(), rowlessCompanyID, true, sanctionOriginSSS).Return(nil)
				case sss.StatusClean:
					repo.EXPECT().ClearCompanySanctionStatusIfSSS(gomock.Any(), rowlessCompanyID).Return(true, nil)
				}
			}
			svc := newComplianceService(t, repo)
			svc.sssRequired, svc.sssEnabled = !tc.optional, !tc.disableSSS
			svc.SetMemberServiceClient(member)
			company := &v1Models.Company{
				CompanyID: rowlessCompanyID, CompanyExternalID: companySFID, CompanyName: "Saved Company",
				IsSanctioned: tc.persistedOrigin != "", SanctionOrigin: tc.persistedOrigin,
			}
			blocked, err := svc.checkCompanyCompliance(context.Background(), company)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantBlocked, blocked)
			assert.Equal(t, tc.wantMember, memberCalls)
			assert.Equal(t, tc.wantSSS, base.sssCalls)
			if tc.wantSSS != 0 {
				assert.Equal(t, tc.wantDomain, screening.Domain)
				assert.Equal(t, companySFID, screening.SFDCID)
				assert.Equal(t, "Saved Company", screening.OrgName)
			}
			if tc.wantErr || tc.memberStatus != 0 {
				assert.Empty(t, svc.complianceCache)
				assert.Equal(t, tc.persistedOrigin != "", company.IsSanctioned)
			}
		})
	}
}

func TestDirectSignatoryGrantRequiresPlatformLFID(t *testing.T) {
	const roleID, grantID = "11111111-2222-4333-8444-555555555555", "66666666-7777-4888-8999-aaaaaaaaaaaa"
	const signatoryEmail = "signer+native@example.invalid"
	lookupFailure := errors.New("user-service transport unavailable")
	for _, tc := range []struct {
		name       string
		userJSON   string
		userStatus int
		userErr    error
		roleStatus int
		blankEmail bool
		grantCode  int
		wantWrite  int
		wantOwned  bool
		wantErr    bool
		wantWarn   bool
	}{
		{name: "missing LFID is optional", userJSON: `{"Data":[]}`},
		{name: "contact without LFID is optional", userJSON: `{"Data":[{"ID":"contact"}]}`},
		{name: "lookup outage does not block signing", userStatus: http.StatusServiceUnavailable, wantWarn: true},
		{name: "lookup transport failure stays non-fatal and private", userErr: lookupFailure, wantWarn: true},
		{name: "blank email cannot select an unrelated user", blankEmail: true, wantErr: true},
		{name: "ACS role lookup failure is a dependency error", userJSON: `{"Data":[{"ID":"contact","Username":"jane"}]}`, roleStatus: http.StatusServiceUnavailable, wantErr: true},
		{name: "verified LFID owns a newly created grant", userJSON: `{"Data":[{"ID":"contact","Username":"jane"}]}`, grantCode: http.StatusCreated, wantWrite: 1, wantOwned: true},
		{name: "verified LFID does not own a conflicting grant", userJSON: `{"Data":[{"ID":"contact","Username":"jane"}]}`, grantCode: http.StatusConflict, wantWrite: 1},
		{name: "ACS grant outage does not block signing", userJSON: `{"Data":[{"ID":"contact","Username":"jane"}]}`, grantCode: http.StatusServiceUnavailable, wantWrite: 1, wantWarn: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := setupRowlessSigningHTTP(t)
			acsService.InitClient("https://"+rowlessPlatformHost, "test-api-key")
			logger := log.GetLogger()
			oldLevel := logger.GetLevel()
			oldHooks := logger.ReplaceHooks(make(logrus.LevelHooks))
			logger.SetLevel(logrus.DebugLevel)
			t.Cleanup(func() {
				logger.ReplaceHooks(oldHooks)
				logger.SetLevel(oldLevel)
			})
			hook := logtest.NewLocal(logger)
			writes := 0
			rowlessSwitch.set(directCLATransport(func(r *http.Request) (*http.Response, error) {
				switch {
				case r.URL.Host == rowlessPlatformHost && r.URL.Path == rowlessUsersPath:
					assert.False(t, tc.blankEmail, "blank email must not send an unfiltered platform query")
					assert.Equal(t, signatoryEmail, r.URL.Query().Get("email"))
					assert.Equal(t, "test-api-key", r.Header.Get("X-API-KEY"))
					if tc.userErr != nil {
						return nil, tc.userErr
					}
					if tc.userStatus != 0 {
						return directCLAResponse(r, tc.userStatus, `{"Message":"unavailable"}`)
					}
					return directCLAResponse(r, http.StatusOK, tc.userJSON)
				case r.URL.Path == rowlessACSRolesPath:
					if tc.roleStatus != 0 {
						return directCLAResponse(r, tc.roleStatus, `{"message":"role service unavailable"}`)
					}
					return directCLAResponse(r, http.StatusOK, `[{"role_id":"`+roleID+`","role_name":"cla-signatory"}]`)
				case r.URL.Path == rowlessACSObjectTypesPath:
					return directCLAResponse(r, http.StatusOK, `[{"name":"project|organization","type_id":11}]`)
				case r.URL.Path == "/acs/v1/api/roles/"+roleID+"/members/users":
					writes++
					assert.Equal(t, http.MethodPost, r.Method)
					return directCLAResponse(r, tc.grantCode, `[{"grant_id":"`+grantID+`","username":"jane"}]`)
				default:
					return base.RoundTrip(r)
				}
			}))
			email := signatoryEmail
			if tc.blankEmail {
				email = " "
			}
			grant, err := prepareDirectUserForSigning(context.Background(), email, rowlessSFID, rowlessProjectSFID)
			if tc.wantErr {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), signatoryEmail)
				assert.NotContains(t, err.Error(), url.QueryEscape(signatoryEmail))
			} else {
				require.NoError(t, err)
			}
			if tc.wantErr {
				status, _, _ := respondRequestCorporateSignature(t, err)
				expectedStatus := http.StatusInternalServerError
				if tc.blankEmail {
					expectedStatus = http.StatusBadRequest
					assert.NotErrorIs(t, err, ErrSigningDependencyFailed)
				} else {
					assert.ErrorIs(t, err, ErrSigningDependencyFailed)
				}
				assert.Equal(t, expectedStatus, status)
			}
			continuedWithoutGrant := false
			for _, entry := range hook.AllEntries() {
				text, formatErr := entry.String()
				require.NoError(t, formatErr)
				assert.NotContains(t, text, signatoryEmail)
				assert.NotContains(t, text, url.QueryEscape(signatoryEmail))
				if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "continuing without") {
					continuedWithoutGrant = true
				}
			}
			if tc.wantWarn {
				assert.True(t, continuedWithoutGrant, "optional dependency failures must be logged")
			}
			assert.Equal(t, tc.wantWrite, writes)
			assert.Zero(t, base.orgCalls, "signatory grants must not query Organization Service")
			if tc.wantOwned {
				require.NotNil(t, grant)
				assert.Equal(t, roleID, grant.RoleID)
				assert.Equal(t, grantID, grant.GrantID)
			} else {
				assert.Nil(t, grant)
			}
		})
	}
}
