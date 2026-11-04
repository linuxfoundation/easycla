// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package organization_service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	acsService "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setCLARoleMode(t *testing.T, enabled bool) {
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

type claRoleTransport func(*http.Request) (*http.Response, error)

func (f claRoleTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func claRoleResponse(r *http.Request, status int, body string) (*http.Response, error) {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

func TestCreateCLARoleScopeUsesAnIndependentFlag(t *testing.T) {
	const orgID, projectSFID, roleID = "0014100000Te0G7AAJ", "a09P000000DsCE5IAN", "role-manager"
	for _, createCompany := range []bool{false, true} {
		for _, directRoles := range []bool{false, true} {
			t.Run(fmt.Sprintf("create=%t/direct=%t", createCompany, directRoles), func(t *testing.T) {
				setCLARoleMode(t, createCompany)
				configPath := filepath.Join(t.TempDir(), "independent-flags.json")
				data := fmt.Sprintf(`{"member_service":{"create_company_enabled":%t},"acs_direct_cla_roles_enabled":%t}`, createCompany, directRoles)
				require.NoError(t, os.WriteFile(configPath, []byte(data), 0600))
				_, err := config.LoadConfig(configPath, nil, "")
				require.NoError(t, err)
				base := newOrgTestClient(t, false)
				acsService.InitClient("https://"+orgTestPlatformHost, "test-key")
				directCalls, legacyCalls := 0, 0
				orgSwitch.set(claRoleTransport(func(r *http.Request) (*http.Response, error) {
					switch {
					case r.URL.Path == "/acs/v1/api/object-types":
						return claRoleResponse(r, http.StatusOK, `[{"name":"project|organization","type_id":11},{"name":"organization","type_id":12}]`)
					case r.Method == http.MethodPost && r.URL.Path == "/acs/v1/api/roles/"+roleID+"/members/users":
						directCalls++
						return claRoleResponse(r, http.StatusCreated, `[{"grant_id":"native-grant","username":"jane"}]`)
					case r.Method == http.MethodPost && r.URL.Path == "/organization-service/v1/orgs/"+orgID+"/scopes":
						legacyCalls++
						return claRoleResponse(r, http.StatusCreated, `{}`)
					default:
						return base.RoundTrip(r)
					}
				}))
				require.NoError(t, GetClient().CreateCLAUserRoleScope(context.Background(), "jane@example.invalid", "jane", projectSFID, orgID, roleID))
				require.NoError(t, GetClient().CreateCLAOrgUserRoleScope(context.Background(), "jane@example.invalid", "jane", orgID, roleID))
				if directRoles {
					assert.Equal(t, 2, directCalls)
					assert.Zero(t, legacyCalls)
				} else {
					assert.Zero(t, directCalls)
					assert.Equal(t, 2, legacyCalls)
				}
			})
		}
	}
}

func TestGetOrganizationForCLAOnlyFallsBackForPendingAccounts(t *testing.T) {
	for _, tc := range []struct {
		name        string
		enabled     bool
		status      int
		sfid        string
		companyName string
		wantName    string
		wantErr     bool
	}{
		{"known org name is preserved", true, http.StatusOK, "0014100000Te0G7AAJ", "Saved Entity", "Platform Name", false},
		{"missing valid Account uses saved display name", true, http.StatusNotFound, "0014100000Te0G7AAJ", "Saved Entity", "Saved Entity", false},
		{"disabled retains legacy missing-org failure", false, http.StatusNotFound, "0014100000Te0G7AAJ", "Saved Entity", "", true},
		{"outage is not pending ingestion", true, http.StatusInternalServerError, "0014100000Te0G7AAJ", "Saved Entity", "", true},
		{"legacy external ID is not a new Account", true, http.StatusNotFound, "lf-legacy", "Saved Entity", "", true},
		{"missing saved name cannot invent metadata", true, http.StatusNotFound, "0014100000Te0G7AAJ", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setCLARoleMode(t, tc.enabled)
			base := newOrgTestClient(t, false)
			orgSwitch.set(claRoleTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == orgTestPlatformHost {
					assert.Equal(t, http.MethodGet, r.Method)
					assert.Equal(t, "/organization-service/v1/orgs/"+tc.sfid, r.URL.Path)
					body := `{"Message":"not found"}`
					if tc.status == http.StatusOK {
						body = `{"ID":"` + tc.sfid + `","Name":"Platform Name"}`
					}
					return claRoleResponse(r, tc.status, body)
				}
				return base.RoundTrip(r)
			}))
			org, err := GetClient().GetOrganizationForCLA(context.Background(), tc.sfid, tc.companyName)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, org)
			} else {
				require.NoError(t, err)
				require.NotNil(t, org)
				assert.Equal(t, tc.sfid, org.ID)
				assert.Equal(t, tc.wantName, org.Name)
				assert.Empty(t, org.Domains)
				assert.Empty(t, org.Link, "display fallback must not manufacture a screening domain")
			}
		})
	}
}

type claRoleEvents struct {
	events.Service
	recorded []*events.LogEventArgs
}

func (e *claRoleEvents) LogEvent(args *events.LogEventArgs) {
	e.recorded = append(e.recorded, args)
}

func TestDeleteCLARolePermissionsUsesExactGrantsAndPreservesLegacyMode(t *testing.T) {
	const orgID, projectSFID = "0014100000Te0G7AAJ", "a09P000000DsCE5IAN"
	for _, tc := range []struct {
		name       string
		enabled    bool
		failDelete bool
	}{
		{"direct revokes only matching project role", true, false},
		{"direct propagates a failed revocation", true, true},
		{"disabled uses the existing organization service", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setCLARoleMode(t, tc.enabled)
			base := newOrgTestClient(t, false)
			recorder := &claRoleEvents{}
			InitClient("https://"+orgTestPlatformHost, recorder)
			acsService.InitClient("https://"+orgTestPlatformHost, "test-key")
			var deleted []string
			legacyCalls, directReads := 0, 0
			orgSwitch.set(claRoleTransport(func(r *http.Request) (*http.Response, error) {
				switch {
				case r.URL.Path == "/acs/v1/api/users/rolescopes/organization":
					directReads++
					assert.Equal(t, "false", r.Header.Get("X-LFX-CACHE"))
					body := `{"userroles":{"jane":[{"role_name":"cla-manager","role_id":"manager-role","scopes":[
						{"grant_id":"jane-target","object_type_name":"project|organization","object_id":"` + projectSFID + `|` + orgID + `"},
						{"grant_id":"other-project","object_type_name":"project|organization","object_id":"other-project|` + orgID + `"},
						{"grant_id":"wrong-type","object_type_name":"organization","object_id":"` + projectSFID + `|` + orgID + `"}]},
						{"role_name":"company-admin","role_id":"admin-role","scopes":[{"grant_id":"admin","object_type_name":"organization","object_id":"` + orgID + `"}]},
						{"role_name":"cla-manager-designee","role_id":"designee-role","scopes":[{"grant_id":"designee","object_type_name":"project|organization","object_id":"` + projectSFID + `|` + orgID + `"}]}],
						"mike":[{"role_name":"cla-manager","role_id":"manager-role","scopes":[{"grant_id":"mike-target","object_type_name":"project|organization","object_id":"` + projectSFID + `|` + orgID + `"}]}]},
						"metadata":{"TotalSize":2}}`
					return claRoleResponse(r, http.StatusOK, body)
				case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/acs/v1/api/roles/manager-role/members/users/"):
					deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/acs/v1/api/roles/manager-role/members/users/"))
					if tc.failDelete {
						return claRoleResponse(r, http.StatusInternalServerError, `{"message":"failed"}`)
					}
					return claRoleResponse(r, http.StatusNoContent, "")
				case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/organization-service/"):
					legacyCalls++
					return claRoleResponse(r, http.StatusOK, `{"userroles":[],"Metadata":{"TotalSize":0}}`)
				default:
					return base.RoundTrip(r)
				}
			}))
			err := GetClient().DeleteCLARolePermissions(context.Background(), orgID, projectSFID, utils.CLAManagerRole, &auth.User{UserName: "operator"})
			if tc.failDelete {
				require.Error(t, err)
				assert.Equal(t, []string{"jane-target"}, deleted)
				assert.Empty(t, recorder.recorded)
			} else {
				require.NoError(t, err)
			}
			if !tc.enabled {
				assert.Equal(t, 1, legacyCalls)
				assert.Zero(t, directReads)
				assert.Empty(t, deleted)
			} else {
				assert.Zero(t, legacyCalls)
				assert.Equal(t, 1, directReads)
				if !tc.failDelete {
					assert.Equal(t, []string{"jane-target", "mike-target"}, deleted)
					require.Len(t, recorder.recorded, 2)
					for i, event := range recorder.recorded {
						assert.Equal(t, events.ClaManagerRoleDeleted, event.EventType)
						assert.Equal(t, "operator", event.LfUsername)
						assert.Equal(t, projectSFID, event.ProjectSFID)
						assert.Equal(t, orgID, event.CompanyID)
						data, ok := event.EventData.(*events.ClaManagerRoleDeletedData)
						require.True(t, ok)
						assert.Equal(t, []string{"jane", "mike"}[i], data.UserName)
						assert.Equal(t, utils.CLAManagerRole, data.Role)
						assert.Equal(t, utils.ProjectOrgScope, data.Scope)
					}
				}
			}
		})
	}
}
