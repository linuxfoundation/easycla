// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/golang/mock/gomock"
	mock_company_repo "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	v1SignatureParams "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	mock_project_repo "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_pcg_repo "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	mock_signature_repo "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	mock_user_repo "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	v2ProjectService "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
	v2ProjectServiceClient "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service/client/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	claGroupsPlatformHost = "platform.company.invalid"
	claGroupsAuthHost     = "auth.company.invalid"
	claGroupsProjectsPath = "/project-service/v1/projects/"
)

type claGroupsTokenBody struct {
	io.Reader
	once  sync.Once
	ready chan struct{}
}

func (b *claGroupsTokenBody) Close() error {
	b.once.Do(func() { close(b.ready) })
	return nil
}

type claGroupsHTTP struct {
	t          *testing.T
	tokenReady chan struct{}
	projects   map[string]string
}

func (h *claGroupsHTTP) RoundTrip(r *http.Request) (*http.Response, error) {
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    r,
	}
	switch {
	case r.URL.Host == claGroupsAuthHost && r.URL.Path == "/oauth/token":
		response.Body = &claGroupsTokenBody{
			Reader: strings.NewReader(`{"access_token":"unit-test-token","token_type":"Bearer","expires_in":3600}`),
			ready:  h.tokenReady,
		}
		return response, nil
	case r.URL.Host == claGroupsPlatformHost && strings.HasPrefix(r.URL.Path, claGroupsProjectsPath):
		body, ok := h.projects[strings.TrimPrefix(r.URL.Path, claGroupsProjectsPath)]
		if !ok {
			response.StatusCode = http.StatusNotFound
			body = `{}`
		}
		response.Body = io.NopCloser(strings.NewReader(body))
		return response, nil
	default:
		h.t.Errorf("unexpected HTTP request (no network allowed): %s %s", r.Method, r.URL)
		return nil, fmt.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL)
	}
}

func setupClaGroupsHTTP(t *testing.T, projects map[string]string) {
	t.Helper()
	transport := &claGroupsHTTP{t: t, tokenReady: make(chan struct{}), projects: projects}
	oldTransport, oldClient := http.DefaultTransport, http.DefaultClient
	http.DefaultTransport = transport
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		http.DefaultTransport, http.DefaultClient = oldTransport, oldClient
	})
	token.Init("test-client", "test-secret", "https://"+claGroupsAuthHost+"/oauth/token", "test-audience")
	select {
	case <-transport.tokenReady:
	case <-time.After(5 * time.Second):
		t.Fatal("mock token initialization did not complete")
	}
	v2ProjectService.InitClient("https://" + claGroupsPlatformHost)
}

func newClaGroupsService(t *testing.T, ctrl *gomock.Controller, sigRepo *mock_signature_repo.MockSignatureRepository, projectRepo *mock_project_repo.MockProjectRepository, pcgRepo *mock_pcg_repo.MockRepository) *service {
	t.Helper()
	svc, ok := NewService(nil, sigRepo, projectRepo, mock_user_repo.NewMockUserRepository(ctrl), mock_company_repo.NewMockIRepository(ctrl), pcgRepo, nil).(*service)
	require.True(t, ok)
	return svc
}

func TestGetCLAGroupsUnderProjectOrFoundationMappingLookup(t *testing.T) {
	setupClaGroupsHTTP(t, map[string]string{
		"proj-none":    `{"ID":"proj-none","Name":"No CLA Group"}`,
		"proj-wrapped": `{"ID":"proj-wrapped","Name":"Wrapped"}`,
		"proj-fail":    `{"ID":"proj-fail","Name":"Lookup Failure"}`,
		"proj-ok":      `{"ID":"proj-ok","Name":"Project OK","ProjectType":"Project"}`,
	})
	lookupErr := errors.New("dynamodb failure")
	mapping := &projects_cla_groups.ProjectClaGroup{ClaGroupID: "cg-1", ProjectSFID: "proj-ok", FoundationSFID: "found-1"}

	for _, tc := range []struct {
		name            string
		projectSFID     string
		mappingErr      error
		wantErr         error
		wantProjectGone bool
		wantGroup       bool
	}{
		{name: "project not associated with a CLA group", projectSFID: "proj-none", mappingErr: projects_cla_groups.ErrProjectNotAssociatedWithClaGroup},
		{name: "wrapped not associated", projectSFID: "proj-wrapped", mappingErr: fmt.Errorf("mapping: %w", projects_cla_groups.ErrProjectNotAssociatedWithClaGroup)},
		{name: "mapping lookup failure is returned", projectSFID: "proj-fail", mappingErr: lookupErr, wantErr: lookupErr},
		{name: "project missing from the project service", projectSFID: "proj-missing", wantProjectGone: true},
		{name: "success", projectSFID: "proj-ok", wantGroup: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			pcgRepo := mock_pcg_repo.NewMockRepository(ctrl)
			projectRepo := mock_project_repo.NewMockProjectRepository(ctrl)
			switch {
			case tc.wantProjectGone:
			case tc.wantGroup:
				pcgRepo.EXPECT().GetClaGroupIDForProject(gomock.Any(), tc.projectSFID).Return(mapping, nil)
				pcgRepo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), "cg-1").Return([]*projects_cla_groups.ProjectClaGroup{mapping}, nil)
				projectRepo.EXPECT().GetCLAGroupByID(gomock.Any(), "cg-1", DontLoadRepoDetails).Return(&v1Models.ClaGroup{
					ProjectID: "cg-1", ProjectName: "CLA Group One", ProjectICLAEnabled: true,
				}, nil)
			default:
				pcgRepo.EXPECT().GetClaGroupIDForProject(gomock.Any(), tc.projectSFID).Return(nil, tc.mappingErr)
			}
			svc := newClaGroupsService(t, ctrl, mock_signature_repo.NewMockSignatureRepository(ctrl), projectRepo, pcgRepo)

			result, err := svc.getCLAGroupsUnderProjectOrFoundation(context.Background(), tc.projectSFID)

			switch {
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, result)
			case tc.wantProjectGone:
				var projectNotFound *v2ProjectServiceClient.GetProjectNotFound
				require.ErrorAs(t, err, &projectNotFound)
				assert.Nil(t, result)
			case tc.wantGroup:
				require.NoError(t, err)
				require.Len(t, result, 1)
				got := result["cg-1"]
				require.NotNil(t, got)
				assert.Equal(t, "cg-1", got.ClaGroupID)
				assert.Equal(t, "CLA Group One", got.ClaGroupName)
				assert.Equal(t, "proj-ok", got.ProjectSFID)
				assert.Equal(t, "Project OK", got.ProjectName)
				assert.Equal(t, "Project", got.ProjectType)
				assert.Equal(t, "found-1", got.FoundationSFID)
				assert.Equal(t, []string{"proj-ok"}, got.SubProjectIDs)
				assert.True(t, got.IclaEnabled)
				assert.False(t, got.CclaEnabled)
			default:
				require.NoError(t, err)
				assert.Empty(t, result)
			}
		})
	}
}

func TestGetCompanyProjectActiveCLAsMappingLookup(t *testing.T) {
	setupClaGroupsHTTP(t, map[string]string{
		"proj-active-none": `{"ID":"proj-active-none","Name":"No CLA Group"}`,
		"proj-active-fail": `{"ID":"proj-active-fail","Name":"Lookup Failure"}`,
	})
	lookupErr := errors.New("dynamodb failure")
	cclaParams := v1SignatureParams.GetCompanySignaturesParams{CompanyID: "company-1", SignatureType: aws.String("ccla")}

	t.Run("project not associated with a CLA group returns an empty list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		pcgRepo := mock_pcg_repo.NewMockRepository(ctrl)
		pcgRepo.EXPECT().GetClaGroupIDForProject(gomock.Any(), "proj-active-none").Return(nil, projects_cla_groups.ErrProjectNotAssociatedWithClaGroup)
		sigRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
		sigRepo.EXPECT().GetCompanySignatures(gomock.Any(), cclaParams, HugePageSize, signatures.DontLoadACLDetails).Return(&v1Models.Signatures{}, nil)
		svc := newClaGroupsService(t, ctrl, sigRepo, mock_project_repo.NewMockProjectRepository(ctrl), pcgRepo)

		out, err := svc.GetCompanyProjectActiveCLAs(context.Background(), "company-1", "proj-active-none")

		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Empty(t, out.List)
	})

	t.Run("mapping lookup failure is returned before signatures are read", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		pcgRepo := mock_pcg_repo.NewMockRepository(ctrl)
		pcgRepo.EXPECT().GetClaGroupIDForProject(gomock.Any(), "proj-active-fail").Return(nil, lookupErr)
		svc := newClaGroupsService(t, ctrl, mock_signature_repo.NewMockSignatureRepository(ctrl), mock_project_repo.NewMockProjectRepository(ctrl), pcgRepo)

		out, err := svc.GetCompanyProjectActiveCLAs(context.Background(), "company-1", "proj-active-fail")

		require.ErrorIs(t, err, lookupErr)
		assert.Nil(t, out)
	})
}
