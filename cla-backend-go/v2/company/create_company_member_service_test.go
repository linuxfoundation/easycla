// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mock_company_repo "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	v2CompanyOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/company"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	acsService "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	memberService "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
	orgService "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
	orgModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/models"
	v2UserServiceModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
)

const (
	msPlatformHost = "platform.members.invalid"
	msAuthHost     = "auth.members.invalid"
	msMemberHost   = "members.invalid"
	msAccountSFID  = "001410000MemberAQA"
	msOrgPath      = "/organization-service/v1/orgs/" + msAccountSFID
	msOAuthPath    = "/oauth/token"
	msEmptyJSON    = "{}"
	msRolesPath    = "/acs/v1/api/roles"
	msLookupPath   = "/organization-service/v1/lookup"
	msAdminRoleID  = "role-company-admin"
	msGrantPath    = "/acs/v1/api/roles/" + msAdminRoleID + "/members/users"
)

// memberCreateHTTP fakes member-service find-or-create and the platform calls of the new create path; any other request fails the test
type memberCreateHTTP struct {
	t                 *testing.T
	tokenReady        chan struct{}
	memberStatus      int
	memberBody        string
	memberErr         error
	memberTokenStatus int
	orgStatus         int
	rolesStatus       int
	scopeStatus       int
	grantBody         string

	mu       sync.Mutex
	calls    []string
	payloads []map[string]string
}

func (h *memberCreateHTTP) RoundTrip(r *http.Request) (*http.Response, error) {
	h.mu.Lock()
	h.calls = append(h.calls, r.Method+" "+r.URL.Host+r.URL.Path)
	h.mu.Unlock()
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Request: r}
	body := msEmptyJSON
	switch {
	case r.URL.Host == msAuthHost && r.URL.Path == msOAuthPath:
		response.Body = &claGroupsTokenBody{Reader: strings.NewReader(`{"access_token":"unit-test-token","token_type":"Bearer","expires_in":3600}`), ready: h.tokenReady}
		return response, nil
	case r.URL.Host == msMemberHost && r.URL.Path == msOAuthPath:
		body = `{"access_token":"member-token","token_type":"Bearer","expires_in":3600}`
		if h.memberTokenStatus != 0 {
			response.StatusCode, body = h.memberTokenStatus, `{"error":"token failure"}`
		}
	case r.URL.Host == msMemberHost && r.Method == http.MethodPost && r.URL.Path == "/b2b_orgs/find-or-create":
		if h.memberErr != nil {
			return nil, h.memberErr
		}
		var payload map[string]string
		require.NoError(h.t, json.NewDecoder(r.Body).Decode(&payload))
		h.mu.Lock()
		h.payloads = append(h.payloads, payload)
		h.mu.Unlock()
		response.StatusCode, body = h.memberStatus, h.memberBody
	case r.URL.Host == msPlatformHost && r.Method == http.MethodGet && r.URL.Path == msOrgPath:
		response.StatusCode, body = h.orgStatus, `{"ID":"`+msAccountSFID+`","Name":"Member Org"}`
		if h.orgStatus != http.StatusOK {
			body = `{"Message":"organization not found"}`
		}
	case r.URL.Host == msPlatformHost && r.URL.Path == msRolesPath:
		body = `[{"role_id":"` + msAdminRoleID + `","role_name":"company-admin"}]`
		if h.rolesStatus != 0 {
			response.StatusCode, body = h.rolesStatus, `{"Message":"acs failure"}`
		}
	case r.URL.Host == msPlatformHost && r.URL.Path == "/acs/v1/api/object-types":
		body = `[{"name":"organization","type_id":12}]`
	case r.URL.Host == msPlatformHost && r.Method == http.MethodPost && r.URL.Path == msGrantPath:
		response.StatusCode, body = h.grantResponse(r)
	case r.URL.Host == msPlatformHost && r.Method == http.MethodPost && r.URL.Path == msOrgPath+"/scopes":
		response.StatusCode = h.scopeStatus
		if h.scopeStatus >= http.StatusBadRequest {
			body = `{"Message":"scope failure"}`
		}
	case r.URL.Host == msPlatformHost && r.Method == http.MethodGet && r.URL.Path == msLookupPath:
		response.StatusCode, body = http.StatusInternalServerError, `{"Message":"organization service create path reached"}`
	default:
		h.t.Errorf("unexpected HTTP request (no network allowed): %s %s", r.Method, r.URL)
		return nil, fmt.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL)
	}
	response.Body = io.NopCloser(strings.NewReader(body))
	return response, nil
}

func (h *memberCreateHTTP) grantResponse(r *http.Request) (int, string) {
	var payload struct {
		RoleID       string   `json:"role_id"`
		Usernames    []string `json:"usernames"`
		ObjectIDs    []string `json:"object_ids"`
		ObjectTypeID int      `json:"object_type_id"`
	}
	require.NoError(h.t, json.NewDecoder(r.Body).Decode(&payload))
	assert.Equal(h.t, msAdminRoleID, payload.RoleID)
	assert.Equal(h.t, []string{testCallerUsername}, payload.Usernames)
	assert.Equal(h.t, []string{msAccountSFID}, payload.ObjectIDs)
	assert.Equal(h.t, 12, payload.ObjectTypeID)
	body := `[{"grant_id":"creator-admin-grant","username":"` + testCallerUsername + `"}]`
	if h.scopeStatus >= http.StatusBadRequest {
		body = `{"message":"private grant failure for ` + testCallerEmail + `"}`
	}
	if h.grantBody != "" {
		body = h.grantBody
	}
	return h.scopeStatus, body
}

func (h *memberCreateHTTP) count(suffix string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.calls {
		if strings.HasSuffix(c, suffix) {
			n++
		}
	}
	return n
}

// memberCreateRepo configures the gomock company repository: the row the repository answers to CreateCompany
// (nil = the argument itself) and the row an external-ID lookup finds (nil = CompanyNotFound)
type memberCreateRepo struct {
	expectRow       bool
	returnedRow     *v1Models.Company
	existingBySF    *v1Models.Company
	existingByShort *v1Models.Company
	existingByName  *v1Models.Company
	expectLookup    bool
	lookupFailure   error
	nameFailure     error
	createFailure   error
}

type memberCreateFixture struct {
	transport *memberCreateHTTP
	svc       *service
	created   *v1Models.Company
}

func newMemberCreateFixture(t *testing.T, transport *memberCreateHTTP, repoCfg memberCreateRepo) *memberCreateFixture {
	t.Helper()
	transport.t, transport.tokenReady = t, make(chan struct{})
	oldTransport, oldClient := http.DefaultTransport, http.DefaultClient
	http.DefaultTransport = transport
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() { http.DefaultTransport, http.DefaultClient = oldTransport, oldClient })
	token.Init("test-client", "test-secret", "https://"+msAuthHost+msOAuthPath, "test-audience")
	select {
	case <-transport.tokenReady:
	case <-time.After(5 * time.Second):
		t.Fatal("mock token initialization did not complete")
	}
	orgService.InitClient("https://"+msPlatformHost, nil)
	acsService.InitClient("https://"+msPlatformHost, "api-key")

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	userRepo := mock_users.NewMockUserRepository(ctrl)
	userRepo.EXPECT().GetUser("u-jane").Return(&v1Models.User{UserID: "u-jane", LfUsername: testCallerUsername}, nil).AnyTimes()
	companyRepo := mock_company_repo.NewMockIRepository(ctrl) // no CreateCompany expectation = any write fails the test
	companyRepo.EXPECT().GetCompanyByName(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, name string) (*v1Models.Company, error) {
		if repoCfg.nameFailure != nil {
			return nil, repoCfg.nameFailure
		}
		if repoCfg.existingByName != nil && repoCfg.existingByName.CompanyName == name {
			return repoCfg.existingByName, nil
		}
		return nil, nil
	}).AnyTimes()
	fx := &memberCreateFixture{transport: transport, svc: &service{userRepo: userRepo, companyRepo: companyRepo}}
	if repoCfg.expectLookup {
		companyRepo.EXPECT().GetCompanyByExternalID(gomock.Any(), msAccountSFID).DoAndReturn(func(_ context.Context, sfid string) (*v1Models.Company, error) {
			if repoCfg.lookupFailure != nil {
				return nil, repoCfg.lookupFailure
			}
			if repoCfg.existingBySF == nil {
				return nil, &utils.CompanyNotFound{Message: "no company records found for SFID", CompanyID: sfid}
			}
			return repoCfg.existingBySF, nil
		})
		companyRepo.EXPECT().GetCompanyByExternalID(gomock.Any(), msAccountSFID[:15]).DoAndReturn(func(_ context.Context, sfid string) (*v1Models.Company, error) {
			if repoCfg.existingByShort == nil {
				return nil, &utils.CompanyNotFound{Message: "no company records found for SFID", CompanyID: sfid}
			}
			return repoCfg.existingByShort, nil
		}).AnyTimes()
	}
	if repoCfg.expectRow {
		companyRepo.EXPECT().CreateCompany(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, model *v1Models.Company) (*v1Models.Company, error) {
			fx.created = model
			if repoCfg.createFailure != nil {
				return nil, repoCfg.createFailure
			}
			if repoCfg.returnedRow != nil {
				return repoCfg.returnedRow, nil
			}
			return model, nil
		})
	}
	client, err := memberService.NewClient(memberService.Config{BaseURL: "https://" + msMemberHost, Audience: "https://member.example/", OAuthTokenURL: "https://" + msMemberHost + msOAuthPath, ClientID: "client-id", ClientSecret: "client-secret"})
	require.NoError(t, err)
	fx.svc.EnableMemberServiceCreate(client)
	return fx
}

func memberCreateParams(website, signingEntity string) *v2CompanyOps.CreateCompanyParams {
	name, username, email := testCompanyName, testCallerUsername, testCallerEmail
	bodyEmail := strfmt.Email(email)
	return &v2CompanyOps.CreateCompanyParams{
		UserID: "u-jane", XUSERNAME: &username, XEMAIL: &email,
		Input: &models.CompanyInput{CompanyName: &name, CompanyWebsite: &website, SigningEntityName: signingEntity, UserEmail: &bodyEmail, Note: "from-test"},
	}
}

func withCompanyRoleFlags(t *testing.T, createCompany, directRoles bool) {
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
	updated.MemberService.CreateCompanyEnabled = createCompany
	updated.ACSDirectCLARolesEnabled = directRoles
	load(updated)
}

func TestCompanyCreatorUsesNativeOrganizationGrant(t *testing.T) {
	previous := lfUserByEmail
	t.Cleanup(func() { lfUserByEmail = previous })
	lfUserByEmail = func(string) (*v2UserServiceModels.User, error) {
		return &v2UserServiceModels.User{Username: testCallerUsername}, nil
	}
	for _, memberCreation := range []bool{true, false} {
		for _, tc := range []struct {
			name        string
			grantStatus int
			roleStatus  int
			grantBody   string
			wantError   bool
		}{
			{name: "created", grantStatus: http.StatusCreated},
			{name: "already assigned", grantStatus: http.StatusConflict},
			{name: "grant outage", grantStatus: http.StatusServiceUnavailable, wantError: true},
			{name: "native failure is not an Org Service association conflict", grantStatus: http.StatusBadRequest, grantBody: fmt.Sprintf(`{"message":%q}`, OrgAssociated), wantError: true},
			{name: "role lookup outage", roleStatus: http.StatusServiceUnavailable, wantError: true},
		} {
			t.Run(fmt.Sprintf("member=%t/%s", memberCreation, tc.name), func(t *testing.T) {
				withCompanyRoleFlags(t, memberCreation, true)
				logger := log.GetLogger()
				oldLevel := logger.GetLevel()
				oldHooks := logger.ReplaceHooks(make(logrus.LevelHooks))
				logger.SetLevel(logrus.WarnLevel)
				t.Cleanup(func() {
					logger.ReplaceHooks(oldHooks)
					logger.SetLevel(oldLevel)
				})
				hook := logrustest.NewLocal(logger)
				transport := &memberCreateHTTP{
					memberStatus: http.StatusCreated,
					memberBody:   `{"uid":"` + msAccountSFID + `","name":"` + testCompanyName + `"}`,
					orgStatus:    http.StatusNotFound, rolesStatus: tc.roleStatus, scopeStatus: tc.grantStatus,
					grantBody: tc.grantBody,
				}
				fx := newMemberCreateFixture(t, transport, memberCreateRepo{expectRow: memberCreation || !tc.wantError})
				var output *models.CompanyOutput
				var err error
				if memberCreation {
					output, err = fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
				} else {
					output, err = fx.svc.createCompanyForOrg(context.Background(), nil,
						&orgModels.Organization{ID: msAccountSFID, Name: testCompanyName},
						testCompanyName, "", testCompanySite, testCallerEmail, "u-jane", "",
						&v2UserServiceModels.User{Username: testCallerUsername, Account: &v2UserServiceModels.Account{ID: NoAccount}})
				}
				if tc.wantError {
					require.ErrorIs(t, err, ErrCompanyDependencyFailed)
					assert.Nil(t, output)
				} else {
					require.NoError(t, err)
					require.NotNil(t, output)
					assert.Equal(t, msAccountSFID, output.CompanyID)
				}
				if memberCreation || !tc.wantError {
					require.NotNil(t, fx.created)
					assert.Equal(t, []string{testCallerUsername}, fx.created.CompanyACL)
					assert.Equal(t, "u-jane", fx.created.CompanyManagerID)
				}
				assert.Zero(t, transport.count(msOrgPath), "an ACS grant must not require Organization Service ingestion")
				assert.Zero(t, transport.count(msOrgPath+"/scopes"))
				wantGrants := 1
				if tc.roleStatus != 0 {
					wantGrants = 0
				}
				assert.Equal(t, wantGrants, transport.count(msGrantPath))
				for _, entry := range hook.AllEntries() {
					text, formatErr := entry.String()
					require.NoError(t, formatErr)
					assert.NotContains(t, text, testCallerEmail)
					assert.NotContains(t, text, "private grant failure")
				}
			})
		}
	}
	t.Run("matched account never gains a creator grant", func(t *testing.T) {
		withCompanyRoleFlags(t, true, true)
		transport := &memberCreateHTTP{
			memberStatus: http.StatusOK,
			memberBody:   `{"uid":"` + msAccountSFID + `","name":"` + testCompanyName + `"}`,
			orgStatus:    http.StatusNotFound,
		}
		fx := newMemberCreateFixture(t, transport, memberCreateRepo{expectRow: true, expectLookup: true})
		output, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		require.NoError(t, err)
		require.NotNil(t, output)
		assert.Empty(t, fx.created.CompanyACL)
		assert.Empty(t, fx.created.CompanyManagerID)
		assert.Zero(t, transport.count(msGrantPath))
		assert.Zero(t, transport.count(msOrgPath))
		assert.Zero(t, transport.count(msOrgPath+"/scopes"))
	})
}

func TestCreateCompanyViaMemberService(t *testing.T) {
	withCompanyRoleFlags(t, true, false)
	previous := lfUserByEmail
	t.Cleanup(func() { lfUserByEmail = previous })
	lfUserByEmail = func(string) (*v2UserServiceModels.User, error) {
		return &v2UserServiceModels.User{Username: testCallerUsername}, nil
	}
	accountJSON := func(name string) string {
		return `{"uid":"` + msAccountSFID + `","name":"` + name + `","website":"` + testCompanySite + `"}`
	}
	created := func(name string) *memberCreateHTTP {
		return &memberCreateHTTP{memberStatus: http.StatusCreated, memberBody: accountJSON(name), orgStatus: http.StatusOK, scopeStatus: http.StatusCreated}
	}
	matched := func(name string) *memberCreateHTTP {
		return &memberCreateHTTP{memberStatus: http.StatusOK, memberBody: accountJSON(name), orgStatus: http.StatusOK, scopeStatus: http.StatusCreated}
	}

	t.Run("switch on without a client fails explicitly", func(t *testing.T) {
		fx := newMemberCreateFixture(t, &memberCreateHTTP{}, memberCreateRepo{})
		fx.svc.EnableMemberServiceCreate(nil)
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrMemberServiceNotConfigured), "%v", err)
		assert.Equal(t, 0, fx.transport.count("/b2b_orgs/find-or-create"))
		assert.Equal(t, 0, fx.transport.count(msLookupPath), "no fallback to the Organization Service path")
	})

	t.Run("shared and missing domains are refused before any call", func(t *testing.T) {
		fx := newMemberCreateFixture(t, &memberCreateHTTP{}, memberCreateRepo{})
		for _, site := range []string{"https://gmail.com", "http://www.onmicrosoft.com/", "bund.de", "github.com", "", "https://"} {
			_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(site, ""))
			assert.True(t, errors.Is(err, ErrCompanySharedDomain), "%q: %v", site, err)
		}
		assert.Equal(t, 0, fx.transport.count("/b2b_orgs/find-or-create"))
	})

	t.Run("tenant hosts under a shared parent are accepted", func(t *testing.T) {
		for _, site := range []string{"https://tenant.onmicrosoft.com", "https://stadt.bund.de/portal", "http://mail.gmail.com/"} {
			fx := newMemberCreateFixture(t, &memberCreateHTTP{memberStatus: http.StatusConflict, memberBody: `{"message":"2 accounts"}`}, memberCreateRepo{})
			_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(site, ""))
			assert.True(t, errors.Is(err, ErrCompanyAmbiguousDomain), "%q reached member-service: %v", site, err)
			assert.Equal(t, 1, fx.transport.count("/b2b_orgs/find-or-create"), site)
		}
	})

	t.Run("ambiguous domain", func(t *testing.T) {
		fx := newMemberCreateFixture(t, &memberCreateHTTP{memberStatus: http.StatusConflict, memberBody: `{"message":"2 accounts match acme.example.invalid"}`}, memberCreateRepo{})
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrCompanyAmbiguousDomain), "%v", err)
		assert.Equal(t, 1, fx.transport.count("/b2b_orgs/find-or-create"))
		assert.Equal(t, 0, fx.transport.count(msOrgPath))
	})

	t.Run("created account gets the company-admin grant and the row", func(t *testing.T) {
		fx := newMemberCreateFixture(t, created(testCompanyName), memberCreateRepo{expectRow: true})
		out, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		require.NoError(t, err)
		assert.Equal(t, msAccountSFID, out.CompanyID)
		assert.Equal(t, testCompanyName, out.CompanyName)
		assert.Equal(t, testCompanyName, out.SigningEntityName)
		assert.Equal(t, testCompanySite, out.CompanyWebsite)
		require.NotNil(t, fx.created)
		assert.Equal(t, msAccountSFID, fx.created.CompanyExternalID)
		assert.Equal(t, "u-jane", fx.created.CompanyManagerID)
		assert.Equal(t, testCompanyName, fx.created.CompanyName)
		assert.Equal(t, testCompanyName, fx.created.SigningEntityName)
		assert.Equal(t, []string{testCallerUsername}, fx.created.CompanyACL)
		assert.Equal(t, "from-test", fx.created.Note, "the note is the caller's, nothing appended")
		assert.Equal(t, []map[string]string{{"name": testCompanyName, "website": testCompanySite}}, fx.transport.payloads)
		assert.Equal(t, 1, fx.transport.count(msRolesPath))
		assert.Equal(t, 1, fx.transport.count(msOrgPath+"/scopes"))
	})

	t.Run("created account keeps an explicit signing entity when the account name differs", func(t *testing.T) {
		fx := newMemberCreateFixture(t, created("Acme Incorporated"), memberCreateRepo{expectRow: true})
		out, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, "Acme Signing LLC"))
		require.NoError(t, err)
		assert.Equal(t, "Acme Incorporated", out.CompanyName)
		assert.Equal(t, "Acme Signing LLC", out.SigningEntityName)
		assert.Equal(t, "Acme Incorporated", fx.created.CompanyName)
		assert.Equal(t, "Acme Signing LLC", fx.created.SigningEntityName)
	})

	t.Run("matched account is recorded without any company-admin grant", func(t *testing.T) {
		fx := newMemberCreateFixture(t, matched("Acme Incorporated"), memberCreateRepo{expectRow: true, expectLookup: true})
		out, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		require.NoError(t, err)
		assert.Equal(t, msAccountSFID, out.CompanyID)
		assert.Equal(t, "Acme Incorporated", out.CompanyName)
		require.NotNil(t, fx.created)
		assert.Equal(t, "Acme Incorporated", fx.created.CompanyName)
		assert.Equal(t, "Acme Incorporated", fx.created.SigningEntityName)
		assert.Equal(t, []string{}, fx.created.CompanyACL, "a matched website is not authority over the EasyCLA company either")
		assert.Equal(t, "", fx.created.CompanyManagerID)
		assert.Equal(t, 0, fx.transport.count(msOrgPath), "no organization read for a matched account")
		assert.Equal(t, 0, fx.transport.count(msRolesPath), "no role lookup for a matched account")
		assert.Equal(t, 0, fx.transport.count(msOrgPath+"/scopes"), "a matched website is not authority over the organization")
	})

	t.Run("matched account already known to EasyCLA is a conflict and untouched", func(t *testing.T) {
		existing := &v1Models.Company{CompanyID: "existing-uuid", CompanyExternalID: msAccountSFID, CompanyName: "Acme Incorporated", SigningEntityName: "Acme EU", CompanyACL: []string{"owner"}}
		fx := newMemberCreateFixture(t, matched("Acme Incorporated"), memberCreateRepo{expectLookup: true, existingBySF: existing})
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrCompanyExists), "%v", err)
		assert.Equal(t, []string{"owner"}, existing.CompanyACL)
		assert.Equal(t, 0, fx.transport.count(msOrgPath+"/scopes"))
	})

	t.Run("matched lookup outage fails before any write", func(t *testing.T) {
		fx := newMemberCreateFixture(t, matched(testCompanyName), memberCreateRepo{expectLookup: true, lookupFailure: errors.New("dynamodb unavailable")})
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrCompanyDependencyFailed), "%v", err)
		assert.False(t, errors.Is(err, ErrCompanyExists))
	})

	t.Run("repository failures are dependency failures", func(t *testing.T) {
		fx := newMemberCreateFixture(t, created(testCompanyName), memberCreateRepo{nameFailure: errors.New("dynamodb unavailable")})
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrCompanyDependencyFailed), "%v", err)
		assert.Equal(t, 0, fx.transport.count("/b2b_orgs/find-or-create"), "no Salesforce write when the name cannot be checked")

		fx = newMemberCreateFixture(t, created(testCompanyName), memberCreateRepo{expectRow: true, createFailure: errors.New("dynamodb unavailable")})
		_, err = fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrCompanyDependencyFailed), "%v", err)
		assert.Equal(t, 0, fx.transport.count(msOrgPath+"/scopes"), "the row is written before any grant, so a failed write grants nothing")
	})

	t.Run("organization service lag skips the grant but creates the company", func(t *testing.T) {
		fx := newMemberCreateFixture(t, &memberCreateHTTP{memberStatus: http.StatusCreated, memberBody: accountJSON(testCompanyName), orgStatus: http.StatusNotFound}, memberCreateRepo{expectRow: true})
		out, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		require.NoError(t, err)
		assert.Equal(t, msAccountSFID, out.CompanyID)
		require.NotNil(t, fx.created)
		assert.Equal(t, []string{testCallerUsername}, fx.created.CompanyACL, "the creator still becomes the EasyCLA company manager")
		assert.Equal(t, 0, fx.transport.count(msRolesPath))
		assert.Equal(t, 0, fx.transport.count(msOrgPath+"/scopes"))
	})

	t.Run("grant dependency failures keep the creator-owned row and fail typed", func(t *testing.T) {
		for name, transport := range map[string]*memberCreateHTTP{
			"organization service outage": {memberStatus: http.StatusCreated, memberBody: accountJSON(testCompanyName), orgStatus: http.StatusInternalServerError},
			"role lookup failure":         {memberStatus: http.StatusCreated, memberBody: accountJSON(testCompanyName), orgStatus: http.StatusOK, rolesStatus: http.StatusInternalServerError},
			"scope failure":               {memberStatus: http.StatusCreated, memberBody: accountJSON(testCompanyName), orgStatus: http.StatusOK, scopeStatus: http.StatusInternalServerError},
		} {
			fx := newMemberCreateFixture(t, transport, memberCreateRepo{expectRow: true})
			out, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
			assert.True(t, errors.Is(err, ErrCompanyDependencyFailed), "%s: %v", name, err)
			assert.Nil(t, out, name)
			require.NotNil(t, fx.created, "%s: the creator-owned row is persisted before the grant", name)
			assert.Equal(t, []string{testCallerUsername}, fx.created.CompanyACL, name)
			assert.Equal(t, "u-jane", fx.created.CompanyManagerID, name)
			assert.Equal(t, msAccountSFID, fx.created.CompanyExternalID, name)
		}
	})

	t.Run("a retry after a failed grant finds the owned row instead of creating an unowned one", func(t *testing.T) {
		first := newMemberCreateFixture(t, &memberCreateHTTP{memberStatus: http.StatusCreated, memberBody: accountJSON(testCompanyName), orgStatus: http.StatusOK, scopeStatus: http.StatusInternalServerError}, memberCreateRepo{expectRow: true})
		_, err := first.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrCompanyDependencyFailed), "%v", err)
		require.NotNil(t, first.created)
		retry := newMemberCreateFixture(t, matched(testCompanyName), memberCreateRepo{expectLookup: true, existingBySF: first.created})
		_, err = retry.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrCompanyExists), "%v", err)
		assert.Equal(t, []string{testCallerUsername}, first.created.CompanyACL, "the creator keeps the row")
		assert.Equal(t, 0, retry.transport.count(msOrgPath+"/scopes"))
	})

	t.Run("a legacy row under the same name is a conflict, never claimed as the new account", func(t *testing.T) {
		legacy := &v1Models.Company{CompanyID: "legacy-uuid", CompanyExternalID: "001410000LegacyAQA", CompanyName: testCompanyName, SigningEntityName: "Acme Legacy", CompanyACL: []string{"owner"}}
		fx := newMemberCreateFixture(t, &memberCreateHTTP{memberStatus: http.StatusCreated, memberBody: accountJSON(testCompanyName), orgStatus: http.StatusNotFound}, memberCreateRepo{expectRow: true, returnedRow: legacy})
		out, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrCompanyExists), "%v", err)
		assert.Nil(t, out)
		assert.Equal(t, []string{"owner"}, legacy.CompanyACL)
	})

	t.Run("an existing company name is refused before any member-service call", func(t *testing.T) {
		taken := &v1Models.Company{CompanyID: "taken-uuid", CompanyExternalID: "001410000LegacyAQA", CompanyName: testCompanyName, CompanyACL: []string{"owner"}}
		fx := newMemberCreateFixture(t, created(testCompanyName), memberCreateRepo{existingByName: taken})
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrCompanyExists), "%v", err)
		assert.Equal(t, 0, fx.transport.count("/b2b_orgs/find-or-create"), "no Salesforce account for a taken name")
		assert.Equal(t, []string{"owner"}, taken.CompanyACL)
	})

	t.Run("member-service rejections keep their meaning", func(t *testing.T) {
		fx := newMemberCreateFixture(t, &memberCreateHTTP{memberStatus: http.StatusBadRequest, memberBody: `{"message":"website is not a valid domain"}`}, memberCreateRepo{})
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, memberService.ErrInvalidRequest), "%v", err)
		assert.False(t, errors.Is(err, ErrCompanyDependencyFailed))

		fx = newMemberCreateFixture(t, &memberCreateHTTP{memberStatus: http.StatusUnprocessableEntity, memberBody: `{"message":"person account"}`}, memberCreateRepo{})
		_, err = fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrCompanyNotEligible), "%v", err)
		assert.False(t, errors.Is(err, ErrCompanyDependencyFailed))
	})

	t.Run("member-service outages are server failures, not caller errors", func(t *testing.T) {
		for name, transport := range map[string]*memberCreateHTTP{
			"503":       {memberStatus: http.StatusServiceUnavailable, memberBody: `{"message":"down"}`},
			"transport": {memberErr: errors.New("connection reset")},
			"token":     {memberTokenStatus: http.StatusInternalServerError},
		} {
			fx := newMemberCreateFixture(t, transport, memberCreateRepo{})
			_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
			assert.True(t, errors.Is(err, ErrCompanyDependencyFailed), "%s: %v", name, err)
			assert.False(t, errors.Is(err, memberService.ErrInvalidRequest), name)
			assert.Equal(t, 0, fx.transport.count(msOrgPath), name)
			assert.Equal(t, 0, fx.transport.count(msOrgPath+"/scopes"), name)
		}
	})

	t.Run("matched account recorded under its 15-character id is a conflict and untouched", func(t *testing.T) {
		short := &v1Models.Company{CompanyID: "short-uuid", CompanyExternalID: msAccountSFID[:15], CompanyName: "Acme Old Name", CompanyACL: []string{"owner"}}
		fx := newMemberCreateFixture(t, matched("Acme Incorporated"), memberCreateRepo{expectLookup: true, existingByShort: short})
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrCompanyExists), "%v", err)
		assert.Equal(t, []string{"owner"}, short.CompanyACL)
		assert.Equal(t, 0, fx.transport.count(msOrgPath+"/scopes"))
	})

	t.Run("a canonical-name conflict stops before any grant or local write", func(t *testing.T) {
		taken := &v1Models.Company{CompanyID: "taken-uuid", CompanyExternalID: "001410000LegacyAQA", CompanyName: "Acme Incorporated", CompanyACL: []string{"owner"}}
		fx := newMemberCreateFixture(t, created("Acme Incorporated"), memberCreateRepo{existingByName: taken})
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.True(t, errors.Is(err, ErrCompanyExists), "%v", err)
		assert.Equal(t, 1, fx.transport.count("/b2b_orgs/find-or-create"), "the request name was free, the account's name is not")
		assert.Equal(t, 0, fx.transport.count(msOrgPath), "no grant attempt")
		assert.Equal(t, 0, fx.transport.count(msOrgPath+"/scopes"))
		assert.Equal(t, []string{"owner"}, taken.CompanyACL)
	})

	t.Run("the new path never logs the caller's e-mail or username", func(t *testing.T) {
		hook := logrustest.NewLocal(log.GetLogger())
		t.Cleanup(hook.Reset)
		fx := newMemberCreateFixture(t, &memberCreateHTTP{memberStatus: http.StatusCreated, memberBody: accountJSON(testCompanyName), orgStatus: http.StatusOK, scopeStatus: http.StatusInternalServerError}, memberCreateRepo{expectRow: true})
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		require.Error(t, err)
		fx = newMemberCreateFixture(t, &memberCreateHTTP{memberStatus: http.StatusCreated, memberBody: accountJSON(testCompanyName), orgStatus: http.StatusNotFound}, memberCreateRepo{expectRow: true})
		_, err = fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		require.NoError(t, err)
		checked := 0
		for _, entry := range hook.AllEntries() {
			if !strings.Contains(entry.Message, "company-admin") && !strings.Contains(entry.Message, "member-service") && !strings.Contains(entry.Message, "Organization Service") {
				continue
			}
			checked++
			_, hasEmail := entry.Data["userEmail"]
			assert.False(t, hasEmail, "%s carries userEmail", entry.Message)
			assert.NotContains(t, entry.Message, testCallerEmail)
			assert.NotContains(t, strings.ReplaceAll(entry.Message, "u-jane", ""), testCallerUsername, "only the user id is logged")
			assert.Equal(t, "u-jane", entry.Data["userID"], entry.Message)
		}
		assert.GreaterOrEqual(t, checked, 2, "the scope failure and the lag warnings were logged")
		filtered := memberServiceLogFields(logrus.Fields{"userEmail": testCallerEmail, "userID": "u-jane", "companyName": testCompanyName})
		assert.Equal(t, logrus.Fields{"userID": "u-jane", "companyName": testCompanyName}, filtered)
	})

	t.Run("a wrapped organization not-found still counts as lag", func(t *testing.T) {
		assert.True(t, isOrgNotFound(fmt.Errorf("fetching org: %w", &organizations.GetOrgNotFound{})))
		assert.True(t, isOrgNotFound(&organizations.GetOrgNotFound{}))
		assert.False(t, isOrgNotFound(errors.New("organization not found")))
	})

	t.Run("switch off keeps the organization service path", func(t *testing.T) {
		fx := newMemberCreateFixture(t, &memberCreateHTTP{}, memberCreateRepo{})
		fx.svc.createViaMemberService, fx.svc.members = false, nil
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		require.Error(t, err)
		assert.Equal(t, 0, fx.transport.count("/b2b_orgs/find-or-create"))
		assert.Equal(t, 1, fx.transport.count(msLookupPath))
	})
}
