// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	claEvents "github.com/linuxfoundation/easycla/cla-backend-go/events"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	mockProject "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	acsService "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	organizationService "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
	projectService "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
	userService "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// an ACS 409 (the signer already holds cla-manager for that project|organization) is not a failure: every project is
// still attempted and the sign event goes on to the cla-manager-designee cleanup
func TestAssignCLAManagerTreatsAnExistingRoleAsAssigned(t *testing.T) {
	const projA, projB = "proj-a", "proj-b"
	projects := []*projects_cla_groups.ProjectClaGroup{{ProjectSFID: projA}, {ProjectSFID: projB}}
	withSeams := func(t *testing.T, create func(projectSFID string) error) *[]string {
		previousLookup, previousCreator := claManagerRoleIDLookup, claManagerRoleScopeCreator
		t.Cleanup(func() { claManagerRoleIDLookup, claManagerRoleScopeCreator = previousLookup, previousCreator })
		claManagerRoleIDLookup = func() (string, error) { return "role-cla-manager", nil }
		var mu sync.Mutex
		attempted := &[]string{}
		claManagerRoleScopeCreator = func(_ context.Context, email, username, projectSFID, companySFID, roleID string) error {
			mu.Lock()
			*attempted = append(*attempted, projectSFID)
			mu.Unlock()
			assert.Equal(t, "jane@example.invalid", email)
			assert.Equal(t, "jane", username)
			assert.Equal(t, "company-sfid", companySFID)
			assert.Equal(t, "role-cla-manager", roleID)
			return create(projectSFID)
		}
		return attempted
	}

	t.Run("already assigned on one project", func(t *testing.T) {
		attempted := withSeams(t, func(projectSFID string) error {
			if projectSFID == projA {
				return &organizations.CreateOrgUsrRoleScopesConflict{}
			}
			return nil
		})
		require.NoError(t, service{}.assignCLAManager(context.Background(), "jane@example.invalid", "jane", "company-sfid", projects))
		sort.Strings(*attempted)
		assert.Equal(t, []string{projA, projB}, *attempted, "both projects are attempted")
	})
	t.Run("failures on every project are collected without a race", func(t *testing.T) {
		boomA, boomB := errors.New("proj-a failed"), errors.New("proj-b failed")
		attempted := withSeams(t, func(projectSFID string) error {
			if projectSFID == projA {
				return boomA
			}
			return boomB
		})
		err := service{}.assignCLAManager(context.Background(), "jane@example.invalid", "jane", "company-sfid", projects)
		assert.True(t, errors.Is(err, boomA) || errors.Is(err, boomB), "one of the failures is returned: %v", err)
		assert.Len(t, *attempted, 2)
	})
	t.Run("any other failure is returned", func(t *testing.T) {
		boom := errors.New("organization service down")
		attempted := withSeams(t, func(projectSFID string) error {
			if projectSFID == projB {
				return boom
			}
			return nil
		})
		assert.True(t, errors.Is(service{}.assignCLAManager(context.Background(), "jane@example.invalid", "jane", "company-sfid", projects), boom))
		assert.Len(t, *attempted, 2)
	})
}

type designeeStreamHTTP func(*http.Request) (*http.Response, error)

func (f designeeStreamHTTP) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type designeeStreamEvents struct {
	claEvents.Repository
	recorded []*v1Models.Event
}

func (e *designeeStreamEvents) CreateEvent(event *v1Models.Event) error {
	e.recorded = append(e.recorded, event)
	return nil
}

func TestDesigneePropagationRequiresExactLFID(t *testing.T) {
	const (
		platformURL    = "https://stream-designee.invalid"
		roleID         = "11111111-2222-4333-8444-555555555555"
		username       = "verified-designee"
		foundationSFID = "foundation"
		claGroupID     = "cla-group"
	)
	originalConfig := config.GetConfig()
	configPath := filepath.Join(t.TempDir(), "config.json")
	loadConfig := func(value config.Config) {
		data, err := json.Marshal(value)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(configPath, data, 0600))
		_, err = config.LoadConfig(configPath, nil, "")
		require.NoError(t, err)
	}
	t.Cleanup(func() { loadConfig(originalConfig) })

	var scopeUsername, returnedUsername string
	var lookupErr error
	var lookups, writes, legacyWrites int
	tokenServed := make(chan struct{})
	var tokenOnce sync.Once
	transport := designeeStreamHTTP(func(r *http.Request) (*http.Response, error) {
		status, body := http.StatusOK, ""
		switch {
		case r.URL.Path == "/oauth/token":
			body = `{"access_token":"unit-test-token","token_type":"Bearer","expires_in":3600}`
			tokenOnce.Do(func() { close(tokenServed) })
		case r.URL.Path == "/project-service/v1/projects/new-project":
			body = `{"ID":"new-project","Name":"Project"}`
		case r.URL.Path == "/acs/v1/api/roles":
			assert.Equal(t, utils.CLADesigneeRole, r.URL.Query().Get("search"))
			body = `[{"role_id":"` + roleID + `","role_name":"cla-manager-designee"}]`
		case r.URL.Path == "/acs/v1/api/object-types":
			body = `[{"name":"project|organization","type_id":11}]`
		case strings.HasPrefix(r.URL.Path, "/acs/v1/api/object-types/11"):
			body = `{"data":[{"user":{"username":"` + scopeUsername + `","email":"designee@example.invalid"},"roles":[{"name":"cla-manager-designee","scopes":[{"object_id":"` + foundationSFID + `|company"}]}]}]}`
		case r.URL.Path == "/user-service/v1/users":
			lookups++
			assert.Equal(t, scopeUsername, r.URL.Query().Get("username"))
			if lookupErr != nil {
				return nil, lookupErr
			}
			body = `{"Data":[{"ID":"contact","Username":"` + returnedUsername + `"}]}`
		case r.URL.Path == "/organization-service/v1/orgs/company":
			body = `{"ID":"company","Name":"Company"}`
		case r.Method == http.MethodPost && r.URL.Path == "/acs/v1/api/roles/"+roleID+"/members/users":
			writes++
			var payload struct {
				Usernames []string `json:"usernames"`
				ObjectIDs []string `json:"object_ids"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			assert.Equal(t, []string{returnedUsername}, payload.Usernames)
			assert.Equal(t, []string{"new-project|company"}, payload.ObjectIDs)
			status, body = http.StatusCreated, `[{"grant_id":"grant","username":"`+returnedUsername+`"}]`
		case r.Method == http.MethodPost && r.URL.Path == "/organization-service/v1/orgs/company/scopes":
			legacyWrites++
			status, body = http.StatusCreated, `{}`
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			return nil, fmt.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	oldTransport, oldClient := http.DefaultTransport, http.DefaultClient
	http.DefaultTransport, http.DefaultClient = transport, &http.Client{Transport: transport}
	t.Cleanup(func() { http.DefaultTransport, http.DefaultClient = oldTransport, oldClient })
	token.Init("test-client", "test-secret", platformURL+"/oauth/token", "test-audience")
	select {
	case <-tokenServed:
	case <-time.After(5 * time.Second):
		t.Fatal("mock token initialization did not complete")
	}
	acsService.InitClient(platformURL, "test-key")
	organizationService.InitClient(platformURL, nil)
	projectService.InitClient(platformURL)
	userService.InitClient(platformURL, "test-key")

	logger := log.GetLogger()
	oldLevel := logger.GetLevel()
	logger.SetLevel(logrus.WarnLevel)
	oldHooks := logger.ReplaceHooks(make(logrus.LevelHooks))
	t.Cleanup(func() {
		logger.ReplaceHooks(oldHooks)
		logger.SetLevel(oldLevel)
	})
	hook := logtest.NewLocal(logger)
	for _, tc := range []struct {
		name, requested, returned string
		legacy                    bool
		lookupFailure             bool
		wantLookups, wantWrites   int
	}{
		{name: "missing username", returned: username},
		{name: "blank username", requested: "   ", returned: username},
		{name: "platform lookup failure", requested: username, lookupFailure: true, wantLookups: 1},
		{name: "platform user without LFID", requested: username, wantLookups: 1},
		{name: "mismatched username", requested: username, returned: "another-user", wantLookups: 1},
		{name: "exact username", requested: username, returned: username, wantLookups: 1, wantWrites: 1},
		{name: "legacy empty username unchanged", legacy: true, wantWrites: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updated := originalConfig
			updated.MemberService.CreateCompanyEnabled = !tc.legacy
			loadConfig(updated)
			scopeUsername, returnedUsername = tc.requested, tc.returned
			lookupErr = nil
			if tc.lookupFailure {
				lookupErr = fmt.Errorf("private platform lookup detail for %s", tc.requested)
			}
			lookups, writes, legacyWrites = 0, 0, 0
			hook.Reset()
			ctrl := gomock.NewController(t)
			projects := mockProject.NewMockService(ctrl)
			projects.EXPECT().SignedAtFoundationLevel(gomock.Any(), foundationSFID).Return(true, nil)
			recorder := &designeeStreamEvents{}
			svc := service{projectService: projects, eventsRepo: recorder}

			require.NoError(t, svc.addCLAManagerDesigneePermissions(context.Background(), claGroupID, foundationSFID, "new-project"))

			assert.Equal(t, tc.wantLookups, lookups)
			if tc.legacy {
				assert.Zero(t, writes)
				assert.Equal(t, tc.wantWrites, legacyWrites)
			} else {
				assert.Equal(t, tc.wantWrites, writes)
				assert.Zero(t, legacyWrites)
			}
			assert.Len(t, recorder.recorded, tc.wantWrites)
			if tc.wantWrites == 0 {
				warnings := 0
				for _, entry := range hook.AllEntries() {
					if entry.Data["claGroupID"] == claGroupID {
						warnings++
						assert.NotContains(t, fmt.Sprint(entry.Message, entry.Data), username)
					}
				}
				assert.Equal(t, 1, warnings, "skipped invalid identities must be logged without their LFID")
			}
		})
	}
}
