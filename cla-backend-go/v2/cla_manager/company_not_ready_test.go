// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

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

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/cla_manager"
	mock_project_service "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_projects_cla_groups "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	mock_signature_repo "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	v1User "github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	v2AcsService "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	v2Company "github.com/linuxfoundation/easycla/cla-backend-go/v2/company"
	organizationService "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
	v2ProjectService "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
	v2UserService "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service"
)

const (
	notReadyCompanySFID = "001410000DesignAQA"
	notReadyProjectSFID = "a0941000002wByZAAU"
	notReadyClaGroupID  = "cla-group-not-ready"
	notReadyOrgPath     = "/organization-service/v1/orgs/" + notReadyCompanySFID
	notReadyOAuthPath   = "/oauth/token"
	notReadyUsersPath   = "/user-service/v1/users"
	notReadyOrgMissing  = `{"Message":"organization not found"}`
	notReadyRoleID      = "11111111-2222-4333-8444-555555555555"
)

// notReadyHTTP fakes the platform calls of the designee and invite paths; the organization read status is
// configurable and anything else fails the test (no network)
type notReadyHTTP struct {
	t         *testing.T
	orgStatus int
	users     string
	acsScopes string
	grantCode int

	mu    sync.Mutex
	calls []string
}

func (h *notReadyHTTP) RoundTrip(r *http.Request) (*http.Response, error) {
	h.mu.Lock()
	h.calls = append(h.calls, r.Method+" "+r.URL.Path)
	h.mu.Unlock()
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Request: r}
	var body string
	switch {
	case r.URL.Host == designeeAuthHost && r.URL.Path == notReadyOAuthPath:
		body = `{"access_token":"unit-test-token","token_type":"Bearer","expires_in":3600}`
		designeeTokenClose.Do(func() { close(designeeTokenServed) })
	case r.URL.Host == designeePlatformHost && r.URL.Path == notReadyUsersPath:
		body = `{"Data":[],"Metadata":{"TotalSize":0}}`
		if h.users != "" {
			body = h.users
		}
	case r.URL.Host == designeePlatformHost && r.URL.Path == "/project-service/v1/projects/"+notReadyProjectSFID:
		body = `{"ID":"` + notReadyProjectSFID + `","Name":"Not Ready Project"}`
	case r.URL.Host == designeePlatformHost && r.Method == http.MethodGet && r.URL.Path == notReadyOrgPath:
		response.StatusCode, body = h.orgStatus, `{"ID":"`+notReadyCompanySFID+`","Name":"Designee Co"}`
		if h.orgStatus != http.StatusOK {
			body = notReadyOrgMissing
		}
	case r.URL.Host == designeePlatformHost && r.URL.Path == "/acs/v1/api/users/rolescopes/organization":
		assert.Equal(h.t, http.MethodGet, r.Method)
		assert.Equal(h.t, "false", r.Header.Get("X-LFX-CACHE"))
		body = `{"userroles":{},"metadata":{"TotalSize":0}}`
		if h.acsScopes != "" {
			body = h.acsScopes
		}
	case r.URL.Host == designeePlatformHost && r.URL.Path == "/acs/v1/api/roles":
		roleName := r.URL.Query().Get("search")
		require.Contains(h.t, []string{utils.CLADesigneeRole, utils.ContactRole}, roleName)
		body = `[{"role_id":"` + notReadyRoleID + `","role_name":"` + roleName + `"}]`
	case r.URL.Host == designeePlatformHost && r.URL.Path == "/acs/v1/api/object-types":
		body = `[{"name":"project|organization","type_id":11}]`
	case r.URL.Host == designeePlatformHost && r.URL.Path == "/acs/v1/api/roles/"+notReadyRoleID+"/members/users":
		assert.Equal(h.t, http.MethodPost, r.Method)
		var payload struct {
			Usernames    []string `json:"usernames"`
			ObjectIDs    []string `json:"object_ids"`
			ObjectTypeID int      `json:"object_type_id"`
		}
		require.NoError(h.t, json.NewDecoder(r.Body).Decode(&payload))
		assert.Equal(h.t, []string{designeeUserLFID}, payload.Usernames)
		assert.Equal(h.t, []string{notReadyProjectSFID + "|" + notReadyCompanySFID}, payload.ObjectIDs)
		assert.Equal(h.t, 11, payload.ObjectTypeID)
		response.StatusCode = h.grantCode
		body = `[{"grant_id":"66666666-7777-4888-8999-aaaaaaaaaaaa","username":"` + designeeUserLFID + `"}]`
	case r.URL.Host == designeePlatformHost && r.Method == http.MethodPost && r.URL.Path == notReadyOrgPath+"/scopes":
		response.StatusCode, body = http.StatusNotFound, notReadyOrgMissing
	default:
		h.t.Errorf("unexpected HTTP request (no network allowed): %s %s", r.Method, r.URL)
		return nil, fmt.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL)
	}
	response.Body = io.NopCloser(strings.NewReader(body))
	return response, nil
}

func (h *notReadyHTTP) count(call string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.calls {
		if c == call {
			n++
		}
	}
	return n
}

func setupNotReadyHTTP(t *testing.T, transport *notReadyHTTP) {
	t.Helper()
	transport.t = t
	oldTransport, oldClient := http.DefaultTransport, http.DefaultClient
	http.DefaultTransport = transport
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() { http.DefaultTransport, http.DefaultClient = oldTransport, oldClient })
	designeeTokenInit.Do(func() {
		token.Init("test-client", "test-secret", "https://"+designeeAuthHost+notReadyOAuthPath, "test-audience")
	})
	select {
	case <-designeeTokenServed:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "mock token initialization did not complete")
	}
	organizationService.InitClient("https://"+designeePlatformHost, nil)
	v2UserService.InitClient("https://"+designeePlatformHost, "api-key")
	v2ProjectService.InitClient("https://" + designeePlatformHost)
	v2AcsService.InitClient("https://"+designeePlatformHost, "api-key")
}

func withCreateViaMemberService(t *testing.T, enabled bool) {
	t.Helper()
	previous := companyCreateViaMemberService
	companyCreateViaMemberService = func() bool { return enabled }
	t.Cleanup(func() { companyCreateViaMemberService = previous })
	previousConfig := config.GetConfig()
	configPath := filepath.Join(t.TempDir(), "config.json")
	load := func(value config.Config) {
		data, err := json.Marshal(value)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(configPath, data, 0600))
		_, err = config.LoadConfig(configPath, nil, "")
		require.NoError(t, err)
	}
	t.Cleanup(func() { load(previousConfig) })
	updated := previousConfig
	updated.MemberService.CreateCompanyEnabled = enabled
	load(updated)
}

var notReadyCompany = &v1Models.Company{CompanyID: "company-not-ready", CompanyExternalID: notReadyCompanySFID, CompanyName: "Designee Co"}

type directDesigneeEvents struct {
	events.Service
	recorded []*events.LogEventArgs
}

func (e *directDesigneeEvents) LogEventWithContext(_ context.Context, args *events.LogEventArgs) {
	e.recorded = append(e.recorded, args)
}

func TestCreateCLAManagerDesigneeBeforeOrganizationIngestion(t *testing.T) {
	for _, status := range []int{http.StatusCreated, http.StatusConflict, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			withCreateViaMemberService(t, true)
			transport := &notReadyHTTP{orgStatus: http.StatusNotFound, users: designeeUserJSON(), grantCode: status}
			setupNotReadyHTTP(t, transport)
			svc := newNotReadyService(t)
			recorder := &directDesigneeEvents{}
			svc.eventService = recorder
			designee, err := svc.CreateCLAManagerDesignee(context.Background(), notReadyCompany.CompanyID, notReadyProjectSFID, "designee@example.invalid")
			if status == http.StatusInternalServerError {
				require.Error(t, err)
				assert.Nil(t, designee)
				assert.Empty(t, recorder.recorded)
			} else {
				require.NoError(t, err)
				require.NotNil(t, designee)
				assert.Equal(t, notReadyCompany.CompanyID, designee.CompanyID)
				assert.Equal(t, notReadyCompanySFID, designee.CompanySfid)
				assert.Equal(t, designeeUserLFID, designee.LfUsername)
				require.Len(t, recorder.recorded, 1)
				assert.Equal(t, events.AssignUserRoleScopeType, recorder.recorded[0].EventType)
				assert.Equal(t, designeeUserLFID, recorder.recorded[0].LfUsername)
			}
			assert.Equal(t, 1, transport.count("GET "+notReadyOrgPath))
			assert.Equal(t, 1, transport.count("GET "+notReadyUsersPath))
			assert.Equal(t, 1, transport.count("POST /acs/v1/api/roles/"+notReadyRoleID+"/members/users"))
			if status == http.StatusInternalServerError {
				assert.Equal(t, 1, transport.count("GET /acs/v1/api/roles"))
				assert.Zero(t, transport.count("POST "+notReadyOrgPath+"/scopes"))
			} else {
				assert.Equal(t, 2, transport.count("GET /acs/v1/api/roles"))
				assert.Equal(t, 1, transport.count("POST "+notReadyOrgPath+"/scopes"), "the preserved best-effort contact grant may fail without blocking the CLA grant")
			}
		})
	}
}

func TestDesigneeDoesNotCallUnusableMetadataTemporaryIngestionDelay(t *testing.T) {
	withCreateViaMemberService(t, true)
	transport := &notReadyHTTP{orgStatus: http.StatusNotFound}
	setupNotReadyHTTP(t, transport)
	svc := newNotReadyService(t)
	company := *notReadyCompany
	company.CompanyName = ""
	svc.companyService = &fakeV1CompanyService{company: &company}
	_, err := svc.CreateCLAManagerDesignee(context.Background(), company.CompanyID, notReadyProjectSFID, "designee@example.invalid")
	var missing *organizations.GetOrgNotFound
	require.ErrorAs(t, err, &missing)
	assert.NotErrorIs(t, err, ErrCompanyNotReady)
	assert.Zero(t, transport.count("GET "+notReadyUsersPath))
}

func TestIsCLAManagerDesigneeUsesNativeLFIDScope(t *testing.T) {
	for _, scopeProject := range []string{notReadyProjectSFID, "another-project"} {
		t.Run(scopeProject, func(t *testing.T) {
			withCreateViaMemberService(t, true)
			scopes := `{"userroles":{"` + designeeUserLFID + `":[{"role_name":"cla-manager-designee","role_id":"` + notReadyRoleID + `","scopes":[{"grant_id":"grant","object_type_name":"project|organization","object_id":"` + scopeProject + `|` + notReadyCompanySFID + `"}]}]},"metadata":{"TotalSize":1}}`
			transport := &notReadyHTTP{orgStatus: http.StatusNotFound, users: designeeUserJSON(), acsScopes: scopes}
			setupNotReadyHTTP(t, transport)
			svc := newDesigneeService(t, designeeMappings(notReadyProjectSFID), nil)
			status, err := svc.IsCLAManagerDesignee(context.Background(), notReadyCompanySFID, designeeClaGroupID, designeeUserLFID)
			require.NoError(t, err)
			require.NotNil(t, status)
			require.NotNil(t, status.HasRole)
			assert.Equal(t, scopeProject == notReadyProjectSFID, *status.HasRole)
			assert.Zero(t, transport.count("GET "+notReadyOrgPath))
			assert.Equal(t, 1, transport.count("GET /acs/v1/api/users/rolescopes/organization"))
		})
	}
}

// newNotReadyService wires the REAL v2 company service (its CLA-manager lookup reads the organization) behind the
// designee and invite paths, with gomock repositories for everything that is not under test
func newNotReadyService(t *testing.T) *service {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	mapping := &projects_cla_groups.ProjectClaGroup{ProjectSFID: notReadyProjectSFID, ClaGroupID: notReadyClaGroupID, FoundationSFID: "a0941000002wFoundN"}
	pcgRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
	pcgRepo.EXPECT().GetClaGroupIDForProject(gomock.Any(), notReadyProjectSFID).Return(mapping, nil).AnyTimes()
	pcgRepo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), notReadyClaGroupID).Return([]*projects_cla_groups.ProjectClaGroup{mapping}, nil).AnyTimes()
	sigRepo := mock_signature_repo.NewMockSignatureRepository(ctrl)
	sigRepo.EXPECT().GetProjectCompanySignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	projectService := mock_project_service.NewMockService(ctrl)
	projectService.EXPECT().GetCLAGroupByID(gomock.Any(), notReadyClaGroupID).Return(&v1Models.ClaGroup{ProjectID: notReadyClaGroupID}, nil).AnyTimes()
	projectService.EXPECT().SignedAtFoundationLevel(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	projectRepo := mock_project_service.NewMockProjectRepository(ctrl)
	projectRepo.EXPECT().GetCLAGroupByID(gomock.Any(), notReadyClaGroupID, false).Return(&v1Models.ClaGroup{ProjectID: notReadyClaGroupID, ProjectName: "Not Ready CLA Group"}, nil).AnyTimes()
	return &service{
		companyService:   &fakeV1CompanyService{company: notReadyCompany},
		v2CompanyService: v2Company.NewService(nil, sigRepo, projectRepo, nil, nil, pcgRepo, nil),
		projectService:   projectService,
		projectCGRepo:    pcgRepo,
	}
}

func TestDesigneeCompanyReadiness(t *testing.T) {
	const orgRead = "GET " + notReadyOrgPath

	t.Run("unknown organization does not block the stored company", func(t *testing.T) {
		withCreateViaMemberService(t, true)
		transport := &notReadyHTTP{orgStatus: http.StatusNotFound}
		setupNotReadyHTTP(t, transport)

		designee, err := newNotReadyService(t).CreateCLAManagerDesignee(context.Background(), "company-not-ready", notReadyProjectSFID, "designee@example.invalid")

		require.ErrorIs(t, err, ErrLFXUserNotFound, "the LF user must still be verified before any grant")
		assert.NotErrorIs(t, err, ErrCompanyNotReady)
		assert.Nil(t, designee)
		assert.Equal(t, 1, transport.count(orgRead), "exactly the existing organization read, no extra probe")
		assert.Equal(t, 1, transport.count("GET "+notReadyUsersPath))
	})

	t.Run("known organization keeps the ordinary flow and errors", func(t *testing.T) {
		withCreateViaMemberService(t, true)
		transport := &notReadyHTTP{orgStatus: http.StatusOK}
		setupNotReadyHTTP(t, transport)

		_, err := newNotReadyService(t).CreateCLAManagerDesignee(context.Background(), "company-not-ready", notReadyProjectSFID, "designee@example.invalid")

		assert.True(t, errors.Is(err, ErrLFXUserNotFound), "the next ordinary step (no LF user) is reached: %v", err)
		assert.False(t, errors.Is(err, ErrCompanyNotReady))
	})

	t.Run("switch off keeps the original error and makes no new call", func(t *testing.T) {
		withCreateViaMemberService(t, false)
		transport := &notReadyHTTP{orgStatus: http.StatusNotFound}
		setupNotReadyHTTP(t, transport)

		_, err := newNotReadyService(t).CreateCLAManagerDesignee(context.Background(), "company-not-ready", notReadyProjectSFID, "designee@example.invalid")

		require.Error(t, err)
		assert.False(t, errors.Is(err, ErrCompanyNotReady), "%v", err)
		var notFound *organizations.GetOrgNotFound
		assert.True(t, errors.As(err, &notFound), "the wrapped organization-service error is returned unchanged: %v", err)
		assert.Equal(t, 1, transport.count(orgRead))
	})

	t.Run("by-group designee does not wait for organization ingestion", func(t *testing.T) {
		withCreateViaMemberService(t, true)
		transport := &notReadyHTTP{orgStatus: http.StatusNotFound}
		setupNotReadyHTTP(t, transport)
		params := cla_manager.CreateCLAManagerDesigneeByGroupParams{CompanyID: "company-not-ready", ClaGroupID: notReadyClaGroupID, Body: cla_manager.CreateCLAManagerDesigneeByGroupBody{UserEmail: "designee@example.invalid"}}

		designees, _, err := newNotReadyService(t).CreateCLAManagerDesigneeByGroup(context.Background(), params, []*projects_cla_groups.ProjectClaGroup{{ProjectSFID: notReadyProjectSFID, ClaGroupID: notReadyClaGroupID}})

		require.ErrorIs(t, err, ErrLFXUserNotFound)
		assert.NotErrorIs(t, err, ErrCompanyNotReady)
		assert.Nil(t, designees)
	})
}

func TestInviteCompanyAdminCompanyNotReady(t *testing.T) {
	const orgRead = "GET " + notReadyOrgPath
	contributor := &v1User.User{UserID: "user-not-ready", LFUsername: "contributor", LFEmail: "contributor@example.invalid", UserName: "Contributor"}

	t.Run("unknown organization is reported as not ready before any invite", func(t *testing.T) {
		withCreateViaMemberService(t, true)
		transport := &notReadyHTTP{orgStatus: http.StatusNotFound}
		setupNotReadyHTTP(t, transport)

		designees, err := newNotReadyService(t).InviteCompanyAdmin(context.Background(), true, "company-not-ready", notReadyClaGroupID, "admin@example.invalid", "Admin", contributor, "")

		var notReady *CompanyNotReadyError
		require.True(t, errors.As(err, &notReady), "%v", err)
		assert.Equal(t, "company-not-ready", notReady.CompanyID)
		assert.Equal(t, notReadyCompanySFID, notReady.CompanySFID)
		assert.Nil(t, designees)
		assert.Equal(t, 1, transport.count(orgRead))
		assert.Equal(t, 0, transport.count("GET /project-service/v1/projects/"+notReadyProjectSFID), "nothing after the readiness failure")
	})

	t.Run("switch off keeps the original error", func(t *testing.T) {
		withCreateViaMemberService(t, false)
		transport := &notReadyHTTP{orgStatus: http.StatusNotFound}
		setupNotReadyHTTP(t, transport)

		_, err := newNotReadyService(t).InviteCompanyAdmin(context.Background(), true, "company-not-ready", notReadyClaGroupID, "admin@example.invalid", "Admin", contributor, "")

		var notFound *organizations.GetOrgNotFound
		require.True(t, errors.As(err, &notFound), "%v", err)
		assert.False(t, errors.Is(err, ErrCompanyNotReady))
	})
}

type notReadyWriteService struct{ Service }

func (notReadyWriteService) notReady() error {
	return &CompanyNotReadyError{CompanyID: "company-1", CompanySFID: "comp-sfid"}
}

func (s notReadyWriteService) CreateCLAManagerDesignee(_ context.Context, _ string, _ string, _ string) (*models.ClaManagerDesignee, error) {
	return nil, s.notReady()
}

func (s notReadyWriteService) CreateCLAManagerDesigneeByGroup(_ context.Context, _ cla_manager.CreateCLAManagerDesigneeByGroupParams, _ []*projects_cla_groups.ProjectClaGroup) ([]*models.ClaManagerDesignee, string, error) {
	return nil, "Creating cla manager failed for Project SFID: proj-sfid", s.notReady()
}

func (s notReadyWriteService) InviteCompanyAdmin(_ context.Context, _ bool, _ string, _ string, _ string, _ string, _ *v1User.User, _ string) ([]*models.ClaManagerDesignee, error) {
	return nil, s.notReady()
}

func (s notReadyWriteService) CreateCLAManagerRequest(_ context.Context, _ bool, _ string, _ string, _ string, _ string, _ *auth.User) (*models.ClaManagerDesignee, error) {
	return nil, s.notReady()
}

func TestWriteHandlersMapCompanyNotReady(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")
	managerUser := &auth.User{UserName: "manager-user", Email: "manager@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
	orgAdminUser := &auth.User{UserName: "org-admin-user", Email: "org-admin@example.com", ACL: auth.ACL{Admin: true, Allowed: true}}
	users := map[string]*auth.User{opDesignee: managerUser, opDesigneeByGroup: managerUser, opInviteAdmin: nil, opCreateRequest: orgAdminUser}
	for _, op := range []string{opDesignee, opDesigneeByGroup, opInviteAdmin, opCreateRequest} {
		t.Run(op, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			companyService := &fakeV1CompanyService{company: &v1Models.Company{CompanyID: "company-1", CompanyName: "Acme", CompanyExternalID: "comp-sfid"}}
			pcgRepo := &fakePCGRepoWithProjects{fakeProjectClaGroupRepo{cginfo: &projects_cla_groups.ProjectClaGroup{ClaGroupID: "cla-group-1"}}}
			Configure(api, notReadyWriteService{}, companyService, "", "", pcgRepo, &fakeEasyCLAUserRepo{})

			status, body, headers := callWriteOp(t, api, op, users[op])

			assert.Equal(t, http.StatusConflict, status, body)
			assert.Equal(t, testReqID, headers.Get("X-Request-Id"))
			var payload map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(body), &payload))
			assert.Equal(t, companyNotReadyCode, payload["code"])
			assert.Equal(t, "company-1", payload["company_id"])
			assert.Equal(t, "comp-sfid", payload["company_sfid"])
			assert.Contains(t, payload["message"], "try again later")
			assert.Equal(t, testReqID, payload["x-request-id"])
		})
	}
}
