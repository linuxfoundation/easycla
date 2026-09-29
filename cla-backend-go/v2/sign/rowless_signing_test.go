// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	mock_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	organizationService "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	oauthEndpoint       = "/oauth/token"
	rowlessPlatformHost = "platform.rowless.invalid"
	rowlessAuthHost     = "auth.rowless.invalid"
	rowlessSFID         = "0014100000Rowless1"
	rowlessProjectSFID  = "a09P000000Rowless1"
	rowlessCompanyID    = "6f1d2c3b-4a5e-4f60-8b7c-9d0e1f2a3b4c"
	rowlessProjectID    = "1b2c3d4e-5f60-4718-9a0b-1c2d3e4f5a6b"
	rowlessOrgJSON      = `{"ID":"` + rowlessSFID + `","Name":"Acme Corp","Domains":"acme.invalid","Link":"https://acme.invalid","SigningEntityName":["Acme Labs"]}`
)

// rowlessHTTP stubs the token endpoint, the organization service and the SSS (auth + status)
// endpoints; any other request fails the test so no network is ever touched
type rowlessHTTP struct {
	t         *testing.T
	orgs      map[string]string
	sssStatus string
	sssCalls  int
	orgCalls  int
}

func (h *rowlessHTTP) RoundTrip(r *http.Request) (*http.Response, error) {
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Request: r}
	switch {
	case r.URL.Host == rowlessAuthHost && r.URL.Path == oauthEndpoint,
		r.URL.Host == "example.auth0.com" && r.URL.Path == oauthEndpoint:
		response.Body = io.NopCloser(strings.NewReader(`{"access_token":"unit-test-token","token_type":"Bearer","expires_in":3600}`))
	case r.URL.Host == "sss.example.com" && r.URL.Path == "/api/v1/organizations/status":
		h.sssCalls++
		response.Body = io.NopCloser(strings.NewReader(`{"status":"` + h.sssStatus + `","entity_id":"e1","source":"unit","org_name":"Acme Corp","domain":"acme.invalid"}`))
	case r.URL.Host == rowlessPlatformHost && strings.HasPrefix(r.URL.Path, "/organization-service/v1/orgs/"):
		h.orgCalls++
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

func setupRowlessHTTP(t *testing.T, sssStatus string) *rowlessHTTP {
	t.Helper()
	transport := &rowlessHTTP{t: t, orgs: map[string]string{rowlessSFID: rowlessOrgJSON}, sssStatus: sssStatus}
	oldTransport, oldClient := http.DefaultTransport, http.DefaultClient
	http.DefaultTransport = transport
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		http.DefaultTransport, http.DefaultClient = oldTransport, oldClient
	})
	token.Init("test-client", "test-secret", "https://"+rowlessAuthHost+oauthEndpoint, "test-audience")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := token.GetToken(); err == nil {
			break
		}
		require.True(t, time.Now().Before(deadline), "mock token initialization did not complete")
		time.Sleep(10 * time.Millisecond)
	}
	organizationService.InitClient("https://"+rowlessPlatformHost, nil)
	return transport
}

func newComplianceService(t *testing.T, companyRepo *mock_company.MockIRepository) *service {
	return &service{
		companyRepo:       companyRepo,
		sssEnabled:        true,
		sssClient:         newTestSSSClient(t),
		complianceCache:   map[string]complianceCacheEntry{},
		complianceCacheMu: &sync.Mutex{},
	}
}

func TestResolveSigningCompany(t *testing.T) {
	transport := setupRowlessHTTP(t, "clean")
	notFoundBySFID := &utils.CompanyNotFound{CompanySFID: rowlessSFID}
	notFoundByName := &utils.CompanyNotFound{CompanyName: "Acme Labs"}
	dynamoErr := errors.New("dynamodb unavailable")
	persisted := &v1Models.Company{CompanyID: rowlessCompanyID, CompanyExternalID: rowlessSFID, CompanyName: "Acme Corp", SigningEntityName: "Acme Labs"}

	tests := []struct {
		name              string
		sfid              string
		signingEntityName string
		expect            func(repo *mock_company.MockIRepository)
		want              *v1Models.Company
		wantErr           error
		wantOrgCalls      int
	}{
		{
			name: "persisted row by SFID",
			sfid: rowlessSFID,
			expect: func(repo *mock_company.MockIRepository) {
				repo.EXPECT().GetCompanyByExternalID(gomock.Any(), rowlessSFID).Return(persisted, nil)
			},
			want: persisted,
		},
		{
			name:              "persisted row by signing entity name",
			sfid:              rowlessSFID,
			signingEntityName: "Acme Labs",
			expect: func(repo *mock_company.MockIRepository) {
				repo.EXPECT().GetCompanyBySigningEntityName(gomock.Any(), "Acme Labs").Return(persisted, nil)
			},
			want: persisted,
		},
		{
			name: "transient company from the organization",
			sfid: rowlessSFID,
			expect: func(repo *mock_company.MockIRepository) {
				repo.EXPECT().GetCompanyByExternalID(gomock.Any(), rowlessSFID).Return(nil, notFoundBySFID)
			},
			want:         &v1Models.Company{CompanyExternalID: rowlessSFID, CompanyName: "Acme Corp", SigningEntityName: "Acme Corp"},
			wantOrgCalls: 1,
		},
		{
			name:              "transient company with a canonical signing entity name",
			sfid:              rowlessSFID,
			signingEntityName: " acme labs ",
			expect: func(repo *mock_company.MockIRepository) {
				repo.EXPECT().GetCompanyBySigningEntityName(gomock.Any(), " acme labs ").Return(nil, notFoundByName)
			},
			want:         &v1Models.Company{CompanyExternalID: rowlessSFID, CompanyName: "Acme Corp", SigningEntityName: "Acme Labs"},
			wantOrgCalls: 1,
		},
		{
			name:              "unknown signing entity name",
			sfid:              rowlessSFID,
			signingEntityName: "Evil Corp",
			expect: func(repo *mock_company.MockIRepository) {
				repo.EXPECT().GetCompanyBySigningEntityName(gomock.Any(), "Evil Corp").Return(nil, notFoundByName)
			},
			wantErr:      notFoundByName,
			wantOrgCalls: 1,
		},
		{
			name: "unknown organization",
			sfid: "0014100000Unknown0",
			expect: func(repo *mock_company.MockIRepository) {
				repo.EXPECT().GetCompanyByExternalID(gomock.Any(), "0014100000Unknown0").Return(nil, notFoundBySFID)
			},
			wantErr:      notFoundBySFID,
			wantOrgCalls: 1,
		},
		{
			name: "repository errors propagate without an organization lookup",
			sfid: rowlessSFID,
			expect: func(repo *mock_company.MockIRepository) {
				repo.EXPECT().GetCompanyByExternalID(gomock.Any(), rowlessSFID).Return(nil, dynamoErr)
			},
			wantErr: dynamoErr,
		},
		{
			name:              "no SFID means no organization lookup",
			signingEntityName: "Acme Labs",
			expect: func(repo *mock_company.MockIRepository) {
				repo.EXPECT().GetCompanyBySigningEntityName(gomock.Any(), "Acme Labs").Return(nil, notFoundByName)
			},
			wantErr: notFoundByName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			repo := mock_company.NewMockIRepository(ctrl)
			tt.expect(repo)
			transport.orgCalls = 0

			got, err := ResolveSigningCompany(context.Background(), repo, tt.sfid, tt.signingEntityName)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
			assert.Equal(t, tt.wantOrgCalls, transport.orgCalls)
		})
	}
}

func TestCheckCompanyComplianceTransientCompanyPersistsNothing(t *testing.T) {
	t.Run("flagged blocks and is remembered for the row created later", func(t *testing.T) {
		transport := setupRowlessHTTP(t, "flagged")
		// nil companyRepo: any persistence attempt would panic
		svc := newComplianceService(t, nil)
		transient := &v1Models.Company{CompanyExternalID: rowlessSFID, CompanyName: "Acme Corp", SigningEntityName: "Acme Corp"}

		blocked, err := svc.checkCompanyCompliance(context.Background(), transient)

		require.NoError(t, err)
		assert.True(t, blocked)
		assert.True(t, transient.IsSanctioned)
		assert.Equal(t, sanctionOriginSSS, transient.SanctionOrigin)
		assert.Equal(t, 1, transport.sssCalls)

		lateRow := &v1Models.Company{CompanyID: rowlessCompanyID, CompanyExternalID: rowlessSFID, CompanyName: "Acme Corp"}
		blocked, err = svc.checkCompanyCompliance(context.Background(), lateRow)

		require.NoError(t, err)
		assert.True(t, blocked)
		assert.True(t, lateRow.IsSanctioned)
		assert.Equal(t, 1, transport.sssCalls, "the cached decision must be reused")
	})
	t.Run("clean allows", func(t *testing.T) {
		transport := setupRowlessHTTP(t, "clean")
		svc := newComplianceService(t, nil)
		transient := &v1Models.Company{CompanyExternalID: rowlessSFID, CompanyName: "Acme Corp", SigningEntityName: "Acme Corp"}

		blocked, err := svc.checkCompanyCompliance(context.Background(), transient)

		require.NoError(t, err)
		assert.False(t, blocked)
		assert.False(t, transient.IsSanctioned)
		assert.Empty(t, transient.SanctionOrigin)
		assert.Equal(t, 1, transport.sssCalls)
	})
}

func TestRequestCorporateSignatureBlocksASanctionedTransientCompanyBeforeAnyWrite(t *testing.T) {
	setupRowlessHTTP(t, "flagged")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mock_company.NewMockIRepository(ctrl)
	repo.EXPECT().GetCompanyByExternalID(gomock.Any(), rowlessSFID).Return(nil, &utils.CompanyNotFound{CompanySFID: rowlessSFID})
	svc := newComplianceService(t, repo)

	out, err := svc.RequestCorporateSignature(context.Background(), "lgryglicki", "Bearer unit", &models.CorporateSignatureInput{
		CompanySfid: utils.StringRef(rowlessSFID),
		ProjectSfid: utils.StringRef(rowlessProjectSFID),
		ReturnURL:   strfmt.URI("https://return.invalid/done"),
	})

	var sanctioned *utils.SanctionedCompanyError
	require.ErrorAs(t, err, &sanctioned)
	assert.Nil(t, out)
	assert.Equal(t, "", sanctioned.CompanyID)
	assert.Equal(t, rowlessSFID, sanctioned.CompanySFID)
	assert.Equal(t, "Acme Corp", sanctioned.CompanyName)
}

func TestRequestCorporateSignatureBlocksAPersistedSanctionedCompany(t *testing.T) {
	transport := setupRowlessHTTP(t, "clean")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mock_company.NewMockIRepository(ctrl)
	repo.EXPECT().GetCompanyByExternalID(gomock.Any(), rowlessSFID).Return(&v1Models.Company{
		CompanyID: rowlessCompanyID, CompanyExternalID: rowlessSFID, CompanyName: "Acme Corp", SigningEntityName: "Acme Corp",
		IsSanctioned: true, SanctionOrigin: "manual",
	}, nil)
	svc := newComplianceService(t, repo)

	out, err := svc.RequestCorporateSignature(context.Background(), "lgryglicki", "******", &models.CorporateSignatureInput{
		CompanySfid: utils.StringRef(rowlessSFID),
		ProjectSfid: utils.StringRef(rowlessProjectSFID),
		ReturnURL:   strfmt.URI("https://return.invalid/done"),
	})

	var sanctioned *utils.SanctionedCompanyError
	require.ErrorAs(t, err, &sanctioned)
	assert.Nil(t, out)
	assert.Equal(t, rowlessCompanyID, sanctioned.CompanyID)
	assert.Equal(t, rowlessSFID, sanctioned.CompanySFID)
	assert.Equal(t, "Acme Corp", sanctioned.CompanyName)
	assert.Zero(t, transport.orgCalls, "a stored manual block must not consult the organization service")
	assert.Zero(t, transport.sssCalls, "a stored manual block must not call SSS")
}

func rowlessSigningFixtures(t *testing.T, ctrl *gomock.Controller) (*mock_company.MockIRepository, *mock_signatures.MockSignatureService, *service, *v1Models.ClaGroup) {
	t.Helper()
	userRepo := mock_users.NewMockUserRepository(ctrl)
	userRepo.EXPECT().GetUserByUserName("lgryglicki", true).Return(&v1Models.User{UserID: "user-1", Username: "Lukasz", LfUsername: "lgryglicki"}, nil).AnyTimes()
	companyRepo := mock_company.NewMockIRepository(ctrl)
	signatureService := mock_signatures.NewMockSignatureService(ctrl)
	svc := &service{
		companyRepo:       companyRepo,
		userService:       users.NewService(userRepo, nil),
		signatureService:  signatureService,
		sssEnabled:        false,
		complianceCache:   map[string]complianceCacheEntry{},
		complianceCacheMu: &sync.Mutex{},
	}
	proj := &v1Models.ClaGroup{
		ProjectID: rowlessProjectID,
		ProjectCorporateDocuments: []v1Models.ClaGroupDocument{{
			DocumentMajorVersion: "2",
			DocumentMinorVersion: "0",
			DocumentCreationDate: "2026-01-01T00:00:00Z",
			DocumentName:         "CCLA",
		}},
	}
	return companyRepo, signatureService, svc, proj
}

func transientSigningCompany() *v1Models.Company {
	return &v1Models.Company{CompanyExternalID: rowlessSFID, CompanyName: "Acme Corp", SigningEntityName: "Acme Labs"}
}

func TestRequestCorporateSignatureCreatesTheCompanyOnlyAfterValidation(t *testing.T) {
	errStop := errors.New("stop after the company step")
	ensured := &v1Models.Company{CompanyID: rowlessCompanyID, CompanyExternalID: rowlessSFID, CompanyName: "Acme Corp", SigningEntityName: "Acme Labs"}

	t.Run("first CCLA creates the row and signs with it", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		companyRepo, signatureService, svc, proj := rowlessSigningFixtures(t, ctrl)
		companyRepo.EXPECT().EnsureCompanyForExternalID(gomock.Any(), rowlessSFID, "Acme Corp", "Acme Labs").Return(ensured, true, nil)
		signatureService.EXPECT().GetCorporateSignatures(gomock.Any(), rowlessProjectID, rowlessCompanyID, gomock.Any(), nil).Return(nil, errStop)
		comp := transientSigningCompany()
		input := &requestCorporateSignatureInput{ProjectID: rowlessProjectID, SigningEntityName: "Acme Labs", ReturnURL: "https://return.invalid/done"}

		_, err := svc.requestCorporateSignature(context.Background(), "https://api.invalid", input, comp, proj, "lgryglicki", "lg@acme.invalid")

		require.ErrorIs(t, err, errStop)
		assert.Equal(t, ensured, comp, "the signer must continue with the persisted row")
		assert.Equal(t, rowlessCompanyID, input.CompanyID)
	})
	t.Run("a request that fails validation creates nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, _, svc, proj := rowlessSigningFixtures(t, ctrl)
		proj.ProjectCorporateDocuments = nil
		comp := transientSigningCompany()
		input := &requestCorporateSignatureInput{ProjectID: rowlessProjectID}

		_, err := svc.requestCorporateSignature(context.Background(), "https://api.invalid", input, comp, proj, "lgryglicki", "lg@acme.invalid")

		require.Error(t, err)
		assert.Equal(t, "", comp.CompanyID)
		assert.Equal(t, "", input.CompanyID)
	})
	t.Run("a row created meanwhile is adopted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		companyRepo, signatureService, svc, proj := rowlessSigningFixtures(t, ctrl)
		existing := &v1Models.Company{CompanyID: rowlessCompanyID, CompanyExternalID: rowlessSFID, CompanyName: "Acme Corp", SigningEntityName: "Acme Labs"}
		companyRepo.EXPECT().EnsureCompanyForExternalID(gomock.Any(), rowlessSFID, "Acme Corp", "Acme Labs").Return(existing, false, nil)
		signatureService.EXPECT().GetCorporateSignatures(gomock.Any(), rowlessProjectID, rowlessCompanyID, gomock.Any(), nil).Return(nil, errStop)
		comp := transientSigningCompany()
		input := &requestCorporateSignatureInput{ProjectID: rowlessProjectID}

		_, err := svc.requestCorporateSignature(context.Background(), "https://api.invalid", input, comp, proj, "lgryglicki", "lg@acme.invalid")

		require.ErrorIs(t, err, errStop)
		assert.Equal(t, rowlessCompanyID, comp.CompanyID)
		assert.Equal(t, rowlessCompanyID, input.CompanyID)
	})
	t.Run("a sanctioned row created meanwhile blocks", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		companyRepo, _, svc, proj := rowlessSigningFixtures(t, ctrl)
		existing := &v1Models.Company{CompanyID: rowlessCompanyID, CompanyExternalID: rowlessSFID, CompanyName: "Acme Corp", IsSanctioned: true}
		companyRepo.EXPECT().EnsureCompanyForExternalID(gomock.Any(), rowlessSFID, "Acme Corp", "Acme Labs").Return(existing, false, nil)
		comp := transientSigningCompany()
		input := &requestCorporateSignatureInput{ProjectID: rowlessProjectID}

		_, err := svc.requestCorporateSignature(context.Background(), "https://api.invalid", input, comp, proj, "lgryglicki", "lg@acme.invalid")

		var sanctioned *utils.SanctionedCompanyError
		require.ErrorAs(t, err, &sanctioned)
		assert.Equal(t, rowlessCompanyID, sanctioned.CompanyID)
		assert.Equal(t, rowlessSFID, sanctioned.CompanySFID)
	})
	t.Run("ensure errors propagate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		companyRepo, _, svc, proj := rowlessSigningFixtures(t, ctrl)
		ensureErr := errors.New("conditional write failed")
		companyRepo.EXPECT().EnsureCompanyForExternalID(gomock.Any(), rowlessSFID, "Acme Corp", "Acme Labs").Return(nil, false, ensureErr)
		comp := transientSigningCompany()

		_, err := svc.requestCorporateSignature(context.Background(), "https://api.invalid", &requestCorporateSignatureInput{ProjectID: rowlessProjectID}, comp, proj, "lgryglicki", "lg@acme.invalid")

		require.ErrorIs(t, err, ensureErr)
		assert.Equal(t, "", comp.CompanyID)
	})
	t.Run("a persisted company is never re-created", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, signatureService, svc, proj := rowlessSigningFixtures(t, ctrl)
		signatureService.EXPECT().GetCorporateSignatures(gomock.Any(), rowlessProjectID, rowlessCompanyID, gomock.Any(), nil).Return(nil, errStop)
		comp := &v1Models.Company{CompanyID: rowlessCompanyID, CompanyExternalID: rowlessSFID, CompanyName: "Acme Corp"}
		input := &requestCorporateSignatureInput{ProjectID: rowlessProjectID, CompanyID: rowlessCompanyID}

		_, err := svc.requestCorporateSignature(context.Background(), "https://api.invalid", input, comp, proj, "lgryglicki", "lg@acme.invalid")

		require.ErrorIs(t, err, errStop)
	})
}

func TestCompanyReferencePatternAcceptsRowsAndOrganizations(t *testing.T) {
	for _, id := range []string{rowlessCompanyID, "0014100000Te0yq", rowlessSFID} {
		assert.NoError(t, (&models.Company{CompanyID: id}).Validate(strfmt.Default), id)
		assert.NoError(t, (&models.UnsignedProject{SigningEntityID: id, ClaGroupName: "CLA Group", ClaGroupID: rowlessProjectID}).Validate(strfmt.Default), id)
	}
	for _, id := range []string{"lf-x", "0014100000Te0yq!", "not-a-uuid-or-sfid"} {
		assert.Error(t, (&models.Company{CompanyID: id}).Validate(strfmt.Default), id)
	}
}
