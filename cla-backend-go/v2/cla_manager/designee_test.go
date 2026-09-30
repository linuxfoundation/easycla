// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

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

	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_projects_cla_groups "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	organizationService "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
	v2UserService "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	designeePlatformHost = "platform.designee.invalid"
	designeeAuthHost     = "auth.designee.invalid"
	designeeCompanySFID  = "0014100000DesignEE"
	designeeUserSFID     = "0034100000DesigneE"
	designeeUserLFID     = "designee-lf"
	designeeClaGroupID   = "cla-group-designee"
)

// designeeHTTP answers the token endpoint, the user-service username lookup and the
// organization-service scope listing; anything else fails the test (no network)
type designeeHTTP struct {
	t *testing.T
	// users is the JSON returned for the user-service username lookup
	users string
	// scopesStatus overrides the status code of the scope listing when non-zero
	scopesStatus int
	// scopes holds the JSON scope listing per organization SFID
	scopes string

	mu    sync.Mutex
	calls []string
}

// token.Init spawns one asynchronous token request per call; it runs once per test binary and
// designeeTokenServed is closed once the fake transport has answered that request, so the
// goroutine no longer reads http.DefaultTransport when a subtest restores it
var (
	designeeTokenInit   sync.Once
	designeeTokenClose  sync.Once
	designeeTokenServed = make(chan struct{})
)

func (h *designeeHTTP) RoundTrip(r *http.Request) (*http.Response, error) {
	h.mu.Lock()
	h.calls = append(h.calls, r.Method+" "+r.URL.Path)
	h.mu.Unlock()
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Request: r}
	switch {
	case r.URL.Host == designeeAuthHost && r.URL.Path == "/oauth/token":
		response.Body = io.NopCloser(strings.NewReader(`{"access_token":"unit-test-token","token_type":"Bearer","expires_in":3600}`))
		designeeTokenClose.Do(func() { close(designeeTokenServed) })
	case r.URL.Host == designeePlatformHost && r.URL.Path == "/user-service/v1/users":
		response.Body = io.NopCloser(strings.NewReader(h.users))
	case r.URL.Host == designeePlatformHost && r.URL.Path == "/organization-service/v1/orgs/"+designeeCompanySFID+"/servicescopes":
		if h.scopesStatus != 0 {
			response.StatusCode = h.scopesStatus
		}
		response.Body = io.NopCloser(strings.NewReader(h.scopes))
	default:
		h.t.Errorf("unexpected HTTP request (no network allowed): %s %s", r.Method, r.URL)
		return nil, fmt.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL)
	}
	return response, nil
}

func (h *designeeHTTP) scopeCalls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.calls {
		if strings.HasSuffix(c, "/servicescopes") {
			n++
		}
	}
	return n
}

func setupDesigneeHTTP(t *testing.T, transport *designeeHTTP) {
	t.Helper()
	oldTransport, oldClient := http.DefaultTransport, http.DefaultClient
	http.DefaultTransport = transport
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		http.DefaultTransport, http.DefaultClient = oldTransport, oldClient
	})
	designeeTokenInit.Do(func() {
		token.Init("test-client", "test-secret", "https://"+designeeAuthHost+"/oauth/token", "test-audience")
	})
	select {
	case <-designeeTokenServed:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "mock token initialization did not complete")
	}
	tok, err := token.GetToken()
	require.NoError(t, err)
	require.NotEmpty(t, tok)
	organizationService.InitClient("https://"+designeePlatformHost, nil)
	v2UserService.InitClient("https://"+designeePlatformHost, "api-key")
}

func designeeUserJSON() string {
	return `{"Data":[{"ID":"` + designeeUserSFID + `","Username":"` + designeeUserLFID + `"}],"Metadata":{"TotalSize":1}}`
}

// designeeScopesJSON lists the user's cla-manager-designee scopes for the given project SFIDs
func designeeScopesJSON(projectSFIDs ...string) string {
	scopes := make([]string, 0, len(projectSFIDs))
	for _, sfid := range projectSFIDs {
		scopes = append(scopes, `{"ObjectTypeName":"project|organization","ObjectID":"`+sfid+`|`+designeeCompanySFID+`"}`)
	}
	return `{"userroles":[{"Contact":{"ID":"` + designeeUserSFID + `","Username":"` + designeeUserLFID + `"},` +
		`"RoleScopes":[{"RoleName":"` + utils.CLADesigneeRole + `","Scopes":[` + strings.Join(scopes, ",") + `]}]}],` +
		`"Metadata":{"Offset":0,"PageSize":1000,"TotalSize":1}}`
}

func designeeMappings(projectSFIDs ...string) []*projects_cla_groups.ProjectClaGroup {
	pcgs := make([]*projects_cla_groups.ProjectClaGroup, 0, len(projectSFIDs))
	for _, sfid := range projectSFIDs {
		pcgs = append(pcgs, &projects_cla_groups.ProjectClaGroup{ProjectSFID: sfid, ClaGroupID: designeeClaGroupID})
	}
	return pcgs
}

func newDesigneeService(t *testing.T, mappings []*projects_cla_groups.ProjectClaGroup, mappingErr error) *service {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	pcgRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
	pcgRepo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), designeeClaGroupID).Return(mappings, mappingErr).AnyTimes()
	return &service{projectCGRepo: pcgRepo}
}

func TestIsCLAManagerDesignee(t *testing.T) {
	t.Run("no role for any project reports false", func(t *testing.T) {
		// regression for lfx-self-serve#3008: every project check returning false used to fall through to true
		transport := &designeeHTTP{t: t, users: designeeUserJSON(), scopes: designeeScopesJSON("project-other")}
		setupDesigneeHTTP(t, transport)
		svc := newDesigneeService(t, designeeMappings("project-a", "project-b", "project-c"), nil)

		status, err := svc.IsCLAManagerDesignee(context.Background(), designeeCompanySFID, designeeClaGroupID, designeeUserLFID)

		require.NoError(t, err)
		require.NotNil(t, status)
		require.NotNil(t, status.HasRole)
		assert.False(t, *status.HasRole)
		assert.Equal(t, designeeUserLFID, status.LfUsername)
		assert.Equal(t, 3, transport.scopeCalls(), "every project mapping must be checked")
	})

	t.Run("no role scopes at all reports false", func(t *testing.T) {
		transport := &designeeHTTP{t: t, users: designeeUserJSON(), scopes: `{"userroles":[],"Metadata":{"Offset":0,"PageSize":1000,"TotalSize":0}}`}
		setupDesigneeHTTP(t, transport)
		svc := newDesigneeService(t, designeeMappings("project-a", "project-b"), nil)

		status, err := svc.IsCLAManagerDesignee(context.Background(), designeeCompanySFID, designeeClaGroupID, designeeUserLFID)

		require.NoError(t, err)
		require.NotNil(t, status.HasRole)
		assert.False(t, *status.HasRole)
		assert.Equal(t, 2, transport.scopeCalls())
	})

	t.Run("role on the mapped project reports true", func(t *testing.T) {
		transport := &designeeHTTP{t: t, users: designeeUserJSON(), scopes: designeeScopesJSON("project-a")}
		setupDesigneeHTTP(t, transport)
		svc := newDesigneeService(t, designeeMappings("project-a"), nil)

		status, err := svc.IsCLAManagerDesignee(context.Background(), designeeCompanySFID, designeeClaGroupID, designeeUserLFID)

		require.NoError(t, err)
		require.NotNil(t, status.HasRole)
		assert.True(t, *status.HasRole)
		assert.Equal(t, designeeUserLFID, status.LfUsername)
		assert.Equal(t, 1, transport.scopeCalls())
	})

	t.Run("role on one of several projects reports true", func(t *testing.T) {
		transport := &designeeHTTP{t: t, users: designeeUserJSON(), scopes: designeeScopesJSON("project-b")}
		setupDesigneeHTTP(t, transport)
		svc := newDesigneeService(t, designeeMappings("project-a", "project-b", "project-c"), nil)

		status, err := svc.IsCLAManagerDesignee(context.Background(), designeeCompanySFID, designeeClaGroupID, designeeUserLFID)

		require.NoError(t, err)
		require.NotNil(t, status.HasRole)
		assert.True(t, *status.HasRole)
	})

	t.Run("no project mappings reports false without role lookups", func(t *testing.T) {
		transport := &designeeHTTP{t: t, users: designeeUserJSON(), scopes: designeeScopesJSON("project-a")}
		setupDesigneeHTTP(t, transport)
		svc := newDesigneeService(t, nil, nil)

		status, err := svc.IsCLAManagerDesignee(context.Background(), designeeCompanySFID, designeeClaGroupID, designeeUserLFID)

		require.NoError(t, err)
		require.NotNil(t, status.HasRole)
		assert.False(t, *status.HasRole)
		assert.Equal(t, 0, transport.scopeCalls())
	})

	t.Run("role lookup failure is returned", func(t *testing.T) {
		transport := &designeeHTTP{t: t, users: designeeUserJSON(), scopes: `{"Message":"boom"}`, scopesStatus: http.StatusInternalServerError}
		setupDesigneeHTTP(t, transport)
		svc := newDesigneeService(t, designeeMappings("project-a"), nil)

		status, err := svc.IsCLAManagerDesignee(context.Background(), designeeCompanySFID, designeeClaGroupID, designeeUserLFID)

		require.Error(t, err)
		assert.Nil(t, status)
	})

	t.Run("mapping lookup failure is returned", func(t *testing.T) {
		transport := &designeeHTTP{t: t, users: designeeUserJSON(), scopes: designeeScopesJSON("project-a")}
		setupDesigneeHTTP(t, transport)
		mappingErr := errors.New("dynamodb unavailable")
		svc := newDesigneeService(t, nil, mappingErr)

		status, err := svc.IsCLAManagerDesignee(context.Background(), designeeCompanySFID, designeeClaGroupID, designeeUserLFID)

		require.ErrorIs(t, err, mappingErr)
		assert.Nil(t, status)
		assert.Equal(t, 0, transport.scopeCalls())
	})

	t.Run("unknown user is returned as an error", func(t *testing.T) {
		transport := &designeeHTTP{t: t, users: `{"Data":[],"Metadata":{"TotalSize":0}}`, scopes: designeeScopesJSON("project-a")}
		setupDesigneeHTTP(t, transport)
		svc := newDesigneeService(t, designeeMappings("project-a"), nil)

		status, err := svc.IsCLAManagerDesignee(context.Background(), designeeCompanySFID, designeeClaGroupID, designeeUserLFID)

		require.ErrorIs(t, err, v2UserService.ErrUserNotFound)
		assert.Nil(t, status)
		assert.Equal(t, 0, transport.scopeCalls())
	})
}
