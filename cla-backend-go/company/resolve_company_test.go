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
	"testing"
	"time"

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
	calls      []string
}

func (h *resolveHTTP) RoundTrip(r *http.Request) (*http.Response, error) {
	h.calls = append(h.calls, r.Method+" "+r.URL.Path)
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
	}, "")
	orgCalls := func() int {
		n := 0
		for _, c := range transport.calls {
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
	transport := setupResolveHTTP(t, nil, `{"Data":[{"ID":"`+resolveSFID+`","Name":"Acme Corp","Link":"https://acme.invalid","SigningEntityName":["Acme Labs"]}],"Metadata":{"TotalSize":1,"Offset":0,"PageSize":1000}}`)
	for _, includeSigningEntityName := range []bool{false, true} {
		t.Run(fmt.Sprintf("includeSigningEntityName=%t", includeSigningEntityName), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			repo := mock_company.NewMockIRepository(ctrl)
			repo.EXPECT().GetCompanyByExternalID(gomock.Any(), resolveSFID).Return(nil, &utils.CompanyNotFound{CompanySFID: resolveSFID})

			result, err := newResolveService(repo).SearchOrganizationByName(context.Background(), "Acme", "", includeSigningEntityName, "")

			require.NoError(t, err)
			require.Len(t, result.List, 1)
			assert.Equal(t, resolveSFID, result.List[0].OrganizationID)
			assert.False(t, *result.List[0].CclaEnabled)
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

		result, err := newResolveService(repo).SearchOrganizationByName(context.Background(), "Acme", "", false, "")

		require.NoError(t, err)
		require.Len(t, result.List, 1)
		assert.Equal(t, resolveSFID, result.List[0].OrganizationID)
		assert.True(t, *result.List[0].CclaEnabled)
	})
	for _, call := range transport.calls {
		if strings.Contains(call, "/organization-service/") {
			assert.True(t, strings.HasPrefix(call, "GET "), "the search must only read from the organization service: %s", call)
		}
	}
}
