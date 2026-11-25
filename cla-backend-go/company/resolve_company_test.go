// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	organizationService "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	resolvePlatformHost = "platform.resolve.invalid"
	resolveAuthHost     = "auth.resolve.invalid"
	resolveSFID         = "0014100000Te0yqQAB"
	resolveShortSFID    = "0014100000Te0yq"
	resolveLegacyOrgID  = "lfbd1c2b3a4d5e6f7a8"
	resolveCompanyID    = "9b8e7d66-40a5-4cde-9f00-3e1d1a2b3c4d"
	resolveMissingID    = "2f1c6c1e-6a2d-4c3b-9d7e-8a5b4c3d2e1f"
)

// resolveHTTP answers the token endpoint and the organization-service routes the tests need;
// anything else fails the test (no network)
type resolveHTTP struct {
	t          *testing.T
	orgs       map[string]string
	searchBody string
	mu         sync.Mutex
	calls      []string
}

// snapshot of the recorded calls, taken under the lock RoundTrip appends with
func (h *resolveHTTP) recorded() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.calls...)
}

func (h *resolveHTTP) RoundTrip(r *http.Request) (*http.Response, error) {
	h.mu.Lock()
	h.calls = append(h.calls, r.Method+" "+r.URL.Path)
	h.mu.Unlock()
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Request: r}
	switch {
	case r.URL.Host == resolveAuthHost && r.URL.Path == "/oauth/token":
		response.Body = io.NopCloser(strings.NewReader(`{"access_token":"unit-test-token","token_type":"Bearer","expires_in":3600}`))
	case r.URL.Host == resolvePlatformHost && r.URL.Path == "/organization-service/v1/orgs/search":
		response.Body = io.NopCloser(strings.NewReader(h.searchBody))
	case r.URL.Host == resolvePlatformHost && strings.HasPrefix(r.URL.Path, "/organization-service/v1/orgs/"):
		body, ok := h.orgs[strings.TrimPrefix(r.URL.Path, "/organization-service/v1/orgs/")]
		if !ok {
			response.StatusCode = http.StatusNotFound
			body = `{"Message":"not found"}`
		}
		response.Body = io.NopCloser(strings.NewReader(body))
	default:
		h.t.Errorf("unexpected HTTP request (no network allowed): %s %s", r.Method, r.URL)
		return nil, fmt.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL)
	}
	return response, nil
}

func setupResolveHTTP(t *testing.T, orgs map[string]string, searchBody string) *resolveHTTP {
	t.Helper()
	transport := &resolveHTTP{t: t, orgs: orgs, searchBody: searchBody}
	oldTransport, oldClient := http.DefaultTransport, http.DefaultClient
	http.DefaultTransport = transport
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		http.DefaultTransport, http.DefaultClient = oldTransport, oldClient
	})
	token.Init("test-client", "test-secret", "https://"+resolveAuthHost+"/oauth/token", "test-audience")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := token.GetToken(); err == nil {
			break
		}
		require.True(t, time.Now().Before(deadline), "mock token initialization did not complete")
		time.Sleep(10 * time.Millisecond)
	}
	organizationService.InitClient("https://"+resolvePlatformHost, nil)
	return transport
}

func newResolveService(repo company.IRepository) company.IService {
	return company.NewService(repo, "https://corporate.invalid", nil, nil)
}

func TestResolveCompanyOrder(t *testing.T) {
	transport := setupResolveHTTP(t, map[string]string{
		resolveSFID:        `{"ID":"` + resolveSFID + `","Name":"Acme Corp","SigningEntityName":["Acme Labs"]}`,
		resolveLegacyOrgID: `{"ID":"` + resolveLegacyOrgID + `","Name":"Legacy Ltd"}`,
		resolveShortSFID:   `{"ID":"` + resolveSFID + `","Name":"Acme Corp","SigningEntityName":["Acme Labs"]}`,
	}, "")
	orgCalls := func() int {
		n := 0
		for _, c := range transport.recorded() {
			if strings.Contains(c, "/organization-service/v1/orgs/") {
				n++
			}
		}
		return n
	}
	notFoundByID := &utils.CompanyNotFound{CompanyID: resolveSFID}
	notFoundBySFID := &utils.CompanyNotFound{CompanySFID: resolveSFID}
	dynamoErr := errors.New("dynamodb unavailable")
	persisted := &models.Company{CompanyID: resolveCompanyID, CompanyExternalID: resolveSFID, CompanyName: "Acme", IsSanctioned: true}

	t.Run("internal id wins", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompany(gomock.Any(), resolveCompanyID).Return(persisted, nil)

		got, err := newResolveService(repo).ResolveCompany(context.Background(), resolveCompanyID)

		require.NoError(t, err)
		assert.Same(t, persisted, got)
	})
	t.Run("external id row wins over the organization", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompany(gomock.Any(), resolveSFID).Return(nil, notFoundByID)
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveSFID).Return(persisted, nil)

		got, err := newResolveService(repo).ResolveCompany(context.Background(), resolveSFID)

		require.NoError(t, err)
		assert.Same(t, persisted, got)
		assert.True(t, got.IsSanctioned)
	})
	t.Run("virtual company when no row exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompany(gomock.Any(), resolveSFID).Return(nil, notFoundByID)
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveSFID).Return(nil, notFoundBySFID)

		got, err := newResolveService(repo).ResolveCompany(context.Background(), resolveSFID)

		require.NoError(t, err)
		assert.Equal(t, &models.Company{CompanyID: resolveSFID, CompanyExternalID: resolveSFID, CompanyName: "Acme Corp", SigningEntityName: "Acme Corp"}, got)
	})
	t.Run("legacy organization id falls back to the organization service", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompany(gomock.Any(), resolveLegacyOrgID).Return(nil, notFoundByID)
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveLegacyOrgID).Return(nil, notFoundBySFID)

		got, err := newResolveService(repo).ResolveCompany(context.Background(), resolveLegacyOrgID)

		require.NoError(t, err)
		assert.Equal(t, company.VirtualCompany(resolveLegacyOrgID, "Legacy Ltd"), got)
	})
	t.Run("internal id without a row is not found and never asks the organization service", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_company.NewMockIRepository(ctrl)
		missing := &utils.CompanyNotFound{CompanyID: resolveMissingID}
		repo.EXPECT().GetCompany(gomock.Any(), resolveMissingID).Return(nil, missing)
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveMissingID).Return(nil, missing)
		before := orgCalls()

		got, err := newResolveService(repo).ResolveCompany(context.Background(), resolveMissingID)

		var notFound *utils.CompanyNotFound
		require.ErrorAs(t, err, &notFound)
		assert.Same(t, missing, err)
		assert.Nil(t, got)
		assert.Equal(t, before, orgCalls(), "a UUID cannot name an organization")
	})
	t.Run("unknown organization is not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompany(gomock.Any(), "0014100000Unknown0").Return(nil, notFoundByID)
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), "0014100000Unknown0").Return(nil, notFoundBySFID)

		got, err := newResolveService(repo).ResolveCompany(context.Background(), "0014100000Unknown0")

		var notFound *utils.CompanyNotFound
		require.ErrorAs(t, err, &notFound)
		assert.Nil(t, got)
	})
	t.Run("repository errors propagate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompany(gomock.Any(), resolveSFID).Return(nil, notFoundByID)
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveSFID).Return(nil, dynamoErr)

		got, err := newResolveService(repo).ResolveCompany(context.Background(), resolveSFID)

		require.ErrorIs(t, err, dynamoErr)
		assert.Nil(t, got)
	})
	t.Run("row stored under the id the organization service returns wins over a virtual company", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompany(gomock.Any(), resolveShortSFID).Return(nil, &utils.CompanyNotFound{CompanyID: resolveShortSFID})
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveShortSFID).Return(nil, &utils.CompanyNotFound{CompanySFID: resolveShortSFID})
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveSFID).Return(persisted, nil)

		got, err := newResolveService(repo).ResolveCompany(context.Background(), resolveShortSFID)

		require.NoError(t, err)
		assert.Same(t, persisted, got)
	})
	t.Run("virtual company carries the id the organization service returns", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompany(gomock.Any(), resolveShortSFID).Return(nil, &utils.CompanyNotFound{CompanyID: resolveShortSFID})
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveShortSFID).Return(nil, &utils.CompanyNotFound{CompanySFID: resolveShortSFID})
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveSFID).Return(nil, notFoundBySFID)

		got, err := newResolveService(repo).ResolveCompany(context.Background(), resolveShortSFID)

		require.NoError(t, err)
		assert.Equal(t, company.VirtualCompany(resolveSFID, "Acme Corp"), got)
	})
	t.Run("repository errors on the returned id propagate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompany(gomock.Any(), resolveShortSFID).Return(nil, &utils.CompanyNotFound{CompanyID: resolveShortSFID})
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveShortSFID).Return(nil, &utils.CompanyNotFound{CompanySFID: resolveShortSFID})
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveSFID).Return(nil, dynamoErr)

		got, err := newResolveService(repo).ResolveCompany(context.Background(), resolveShortSFID)

		require.ErrorIs(t, err, dynamoErr)
		assert.Nil(t, got)
	})
}

func TestVirtualCompany(t *testing.T) {
	got := company.VirtualCompany(resolveSFID, "Acme Corp")
	assert.Equal(t, resolveSFID, got.CompanyID)
	assert.Equal(t, resolveSFID, got.CompanyExternalID)
	assert.Equal(t, "Acme Corp", got.CompanyName)
	assert.Equal(t, "Acme Corp", got.SigningEntityName)
	assert.False(t, got.IsSanctioned)
}

func TestGetCompanyByExternalIDDoesNotCreate(t *testing.T) {
	t.Run("no row", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		notFound := &utils.CompanyNotFound{CompanySFID: resolveSFID}
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveSFID).Return(nil, notFound)

		got, err := newResolveService(repo).GetCompanyByExternalID(context.Background(), resolveSFID)

		require.ErrorIs(t, err, notFound)
		assert.Nil(t, got)
	})
	t.Run("persisted row", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		persisted := &models.Company{CompanyID: resolveCompanyID, CompanyExternalID: resolveSFID, CompanyName: "Acme"}
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveSFID).Return(persisted, nil)

		got, err := newResolveService(repo).GetCompanyByExternalID(context.Background(), resolveSFID)

		require.NoError(t, err)
		assert.Same(t, persisted, got)
	})
}

func TestSearchOrganizationByNameDoesNotCreate(t *testing.T) {
	const prospectSFID = "0014100000Te0yrQAB"
	transport := setupResolveHTTP(t, nil, `{"Data":[`+
		`{"ID":"`+resolveSFID+`","Name":"Acme Corp","Link":"https://acme.invalid","SigningEntityName":["Acme Labs"],"SalesforceB2BAccountID":"b2b-acme"},`+
		`{"ID":"`+prospectSFID+`","Name":"Acme Prospect","Link":"https://prospect.invalid","SigningEntityName":["Acme Prospect"]}`+
		`],"Metadata":{"TotalSize":2,"Offset":0,"PageSize":1000}}`)
	notFound := func(repo *mock_company.MockIRepository, ids ...string) {
		for _, id := range ids {
			repo.EXPECT().GetCompanyByExternalID(gomock.Any(), id).Return(nil, &utils.CompanyNotFound{CompanySFID: id})
		}
	}
	for _, includeSigningEntityName := range []bool{false, true} {
		t.Run(fmt.Sprintf("includeSigningEntityName=%t", includeSigningEntityName), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			repo := mock_company.NewMockIRepository(ctrl)
			notFound(repo, resolveSFID, prospectSFID)
			repo.EXPECT().GetCompanyByName(gomock.Any(), "Acme").Return(nil, nil)

			result, err := newResolveService(repo).SearchOrganizationByName(context.Background(), "Acme", "", includeSigningEntityName, "")

			require.NoError(t, err)
			require.Len(t, result.List, 2)
			assert.Equal(t, resolveSFID, result.List[0].OrganizationID)
			assert.False(t, *result.List[0].CclaEnabled)
			require.NotNil(t, result.List[0].HasB2bAccount)
			assert.True(t, *result.List[0].HasB2bAccount, "an organization linked to a Salesforce B2B account")
			assert.Equal(t, prospectSFID, result.List[1].OrganizationID)
			require.NotNil(t, result.List[1].HasB2bAccount, "every row carries the flag")
			assert.False(t, *result.List[1].HasB2bAccount, "an organization with no Salesforce B2B account")
			if includeSigningEntityName {
				assert.Equal(t, []string{"Acme Labs"}, result.List[0].SigningEntityNames)
			}
		})
	}
	t.Run("persisted company reports its CCLA state", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveSFID).Return(&models.Company{CompanyID: resolveCompanyID, CompanyExternalID: resolveSFID, CompanyName: "Acme"}, nil)
		repo.EXPECT().IsCCLAEnabledForCompany(gomock.Any(), resolveCompanyID).Return(true, nil)
		notFound(repo, prospectSFID)
		repo.EXPECT().GetCompanyByName(gomock.Any(), "Acme").Return(nil, nil)

		result, err := newResolveService(repo).SearchOrganizationByName(context.Background(), "Acme", "", false, "")

		require.NoError(t, err)
		require.Len(t, result.List, 2)
		assert.Equal(t, resolveSFID, result.List[0].OrganizationID)
		assert.True(t, *result.List[0].CclaEnabled)
		assert.False(t, *result.List[1].CclaEnabled)
	})
	for _, call := range transport.recorded() {
		if strings.Contains(call, "/organization-service/") {
			assert.True(t, strings.HasPrefix(call, "GET "), "the search must only read from the organization service: %s", call)
		}
	}
}

func TestSearchOrganizationByNameBoundsParallelLookups(t *testing.T) {
	const (
		limit  = 32
		listed = 4 * limit
	)
	rows := make([]string, 0, listed)
	for i := 0; i < listed; i++ {
		rows = append(rows, fmt.Sprintf(`{"ID":"001QP%013d","Name":"Acme %03d","Link":"https://acme%d.invalid"}`, i, i, i))
	}
	setupResolveHTTP(t, nil, `{"Data":[`+strings.Join(rows, ",")+`],"Metadata":{"TotalSize":`+fmt.Sprint(listed)+`,"Offset":0,"PageSize":1000}}`)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mock_company.NewMockIRepository(ctrl)
	var running, peak atomic.Int32
	repo.EXPECT().GetCompanyByExternalID(gomock.Any(), gomock.Any()).Times(listed).DoAndReturn(func(_ context.Context, id string) (*models.Company, error) {
		now := running.Add(1)
		for {
			seen := peak.Load()
			if now <= seen || peak.CompareAndSwap(seen, now) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		running.Add(-1)
		return nil, &utils.CompanyNotFound{CompanySFID: id}
	})
	repo.EXPECT().GetCompanyByName(gomock.Any(), "Acme").Return(nil, nil)

	result, err := newResolveService(repo).SearchOrganizationByName(context.Background(), "Acme", "", false, "")

	require.NoError(t, err)
	assert.Len(t, result.List, listed)
	assert.LessOrEqual(t, int(peak.Load()), limit)
}

func TestSearchOrganizationByNameAddsNewCompanyNotListed(t *testing.T) {
	const (
		newSFID      = "001QP00001AbCdEfGH"
		newCompanyID = "5d2c4f1a-7b3e-4c9d-8a6f-1e2d3c4b5a69"
	)
	transport := setupResolveHTTP(t, nil, `{"Data":[`+
		`{"ID":"`+resolveSFID+`","Name":"Acme Corp","Link":"https://acme.invalid","SigningEntityName":["Acme Labs"],"SalesforceB2BAccountID":"b2b-acme"}`+
		`],"Metadata":{"TotalSize":1,"Offset":0,"PageSize":1000}}`)
	row := func(externalID string, age time.Duration) *models.Company {
		return &models.Company{CompanyID: newCompanyID, CompanyExternalID: externalID, CompanyName: "Acme New", SigningEntityName: "Acme New Inc", Created: strfmt.DateTime(time.Now().Add(-age))}
	}
	search := func(t *testing.T, name, website, filter string, includeSigningEntityName bool, expect func(repo *mock_company.MockIRepository)) []*models.Org {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock_company.NewMockIRepository(ctrl)
		repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveSFID).Return(nil, &utils.CompanyNotFound{CompanySFID: resolveSFID})
		expect(repo)
		result, err := newResolveService(repo).SearchOrganizationByName(context.Background(), name, website, includeSigningEntityName, filter)
		require.NoError(t, err)
		return result.List
	}
	for _, includeSigningEntityName := range []bool{false, true} {
		t.Run(fmt.Sprintf("created minutes ago, includeSigningEntityName=%t", includeSigningEntityName), func(t *testing.T) {
			list := search(t, " Acme New ", "", "", includeSigningEntityName, func(repo *mock_company.MockIRepository) {
				repo.EXPECT().GetCompanyByName(gomock.Any(), "Acme New").Return(row(newSFID, 5*time.Minute), nil)
				repo.EXPECT().IsCCLAEnabledForCompany(gomock.Any(), newCompanyID).Return(true, nil)
			})
			require.Len(t, list, 2)
			assert.Equal(t, resolveSFID, list[0].OrganizationID)
			assert.Equal(t, newSFID, list[1].OrganizationID)
			assert.Equal(t, "Acme New", list[1].OrganizationName)
			assert.True(t, *list[1].CclaEnabled)
			require.NotNil(t, list[1].HasB2bAccount)
			assert.False(t, *list[1].HasB2bAccount, "a B2B account is only reported once the organization service lists it")
			if includeSigningEntityName {
				assert.Equal(t, []string{"Acme New Inc"}, list[1].SigningEntityNames)
			} else {
				assert.Empty(t, list[1].SigningEntityNames)
			}
		})
	}
	notAdded := map[string]*models.Company{
		"already listed under the 15-character id": row(resolveShortSFID, 5*time.Minute),
		"older than the window":                    row(newSFID, 25*time.Hour),
		"no creation time":                         {CompanyID: newCompanyID, CompanyExternalID: newSFID, CompanyName: "Acme New"},
		"legacy id":                                row("lfoG2DJ5TSL5x3It1V", 5*time.Minute),
		"no external id":                           row("", 5*time.Minute),
		"no company of that name":                  nil,
	}
	for name, found := range notAdded {
		t.Run(name, func(t *testing.T) {
			list := search(t, "Acme New", "", "", false, func(repo *mock_company.MockIRepository) {
				repo.EXPECT().GetCompanyByName(gomock.Any(), "Acme New").Return(found, nil)
			})
			require.Len(t, list, 1)
			assert.Equal(t, resolveSFID, list[0].OrganizationID)
		})
	}
	t.Run("a failed lookup leaves the organization service results", func(t *testing.T) {
		list := search(t, "Acme New", "", "", false, func(repo *mock_company.MockIRepository) {
			repo.EXPECT().GetCompanyByName(gomock.Any(), "Acme New").Return(nil, errors.New("dynamodb unavailable"))
		})
		require.Len(t, list, 1)
	})
	t.Run("listed accounts are compared by their canonical id", func(t *testing.T) {
		const canonical, short, lowered = "001QP00001AbCdEYAV", "001QP00001AbCdE", "001qp00001abcdeyav"
		listed := func(signingEntities string) string {
			return `{"Data":[{"ID":"` + canonical + `","Name":"Acme New Listed","Link":"https://acme-new.invalid","SigningEntityName":[` + signingEntities + `],"SalesforceB2BAccountID":"b2b-new"}],"Metadata":{"TotalSize":1,"Offset":0,"PageSize":1000}}`
		}
		original := transport.searchBody
		defer func() { transport.searchBody = original }()
		for _, tc := range []struct {
			name                     string
			rowID, signingEntities   string
			includeSigningEntityName bool
			cclaLookups              int
			wantSigningEntityNames   []string
		}{
			{name: "another letter case of the 18-character id", rowID: lowered, cclaLookups: 1},
			{name: "the 15-character id", rowID: short, cclaLookups: 1},
			{name: "listed with its signing entities", rowID: canonical, signingEntities: `"Acme New Corp"`, includeSigningEntityName: true, cclaLookups: 1, wantSigningEntityNames: []string{"Acme New Corp"}},
			{name: "listed without signing entities yet keeps the listed record", rowID: lowered, includeSigningEntityName: true, cclaLookups: 2, wantSigningEntityNames: []string{"Acme New Inc"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				transport.searchBody = listed(tc.signingEntities)
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				repo := mock_company.NewMockIRepository(ctrl)
				found := row(tc.rowID, 5*time.Minute)
				repo.EXPECT().GetCompanyByExternalID(gomock.Any(), canonical).Return(found, nil)
				repo.EXPECT().GetCompanyByName(gomock.Any(), "Acme New").Return(found, nil)
				repo.EXPECT().IsCCLAEnabledForCompany(gomock.Any(), newCompanyID).Return(false, nil).Times(tc.cclaLookups)

				result, err := newResolveService(repo).SearchOrganizationByName(context.Background(), "Acme New", "", tc.includeSigningEntityName, "")

				require.NoError(t, err)
				require.Len(t, result.List, 1, "one account, one row")
				assert.Equal(t, canonical, result.List[0].OrganizationID)
				assert.Equal(t, "Acme New Listed", result.List[0].OrganizationName)
				assert.Equal(t, "https://acme-new.invalid", result.List[0].OrganizationWebsite)
				require.NotNil(t, result.List[0].HasB2bAccount)
				assert.True(t, *result.List[0].HasB2bAccount, "the organization service lists the account with a B2B id")
				assert.Equal(t, tc.wantSigningEntityNames, result.List[0].SigningEntityNames)
			})
		}
	})
	t.Run("only a pure name search looks up the name", func(t *testing.T) {
		none := func(*mock_company.MockIRepository) {}
		require.Len(t, search(t, "", "acme.invalid", "", false, none), 1)
		require.Len(t, search(t, "Acme New", "acme.invalid", "", false, none), 1)
		require.Len(t, search(t, "Acme New", "", "name eq Acme New", false, none), 1)
	})
}
