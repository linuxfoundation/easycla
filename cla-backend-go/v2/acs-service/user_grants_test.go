// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package acs_service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	runtimeClient "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service/client"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const acsObjectTypesPath = "/acs/v1/api/object-types"

func userGrantsTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	initFakeToken(t)
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	oldHTTP := orgGrantsHTTPClient
	orgGrantsHTTPClient = server.Client()
	t.Cleanup(func() { orgGrantsHTTPClient = oldHTTP })
	transport := runtimeClient.NewWithClient(strings.TrimPrefix(server.URL, "https://"), "/acs/v1/api", []string{"https"}, server.Client())
	return &Client{apiGwURL: server.URL, cl: client.New(transport, strfmt.Default)}
}

func TestCreateProjectRoleGrant(t *testing.T) {
	const (
		roleID  = "11111111-2222-4333-8444-555555555555"
		grantID = "66666666-7777-4888-8999-aaaaaaaaaaaa"
		org     = "0014100000Te0G7AAJ"
		project = "a09P000000DsCE5IAN"
	)
	for _, tc := range []struct {
		name     string
		status   int
		response string
		wantErr  bool
		owned    bool
	}{
		{"created UUID grant and string scope", http.StatusCreated, `[{"grant_id":"` + grantID + `","role_id":"` + roleID + `","username":"jane","object_id":"` + project + `|` + org + `","object_type_name":"project|organization"}]`, false, true},
		{"conflict is never owned", http.StatusConflict, `{"message":"already assigned"}`, false, false},
		{"denied grant fails", http.StatusForbidden, `{"message":"forbidden"}`, true, false},
		{"upstream failure fails", http.StatusInternalServerError, `{"message":"unavailable"}`, true, false},
		{"scope ID is not a grant ID", http.StatusCreated, `[{"scope_id":"not-the-grant"}]`, true, false},
		{"different returned user fails", http.StatusCreated, `[{"grant_id":"` + grantID + `","username":"somebody-else"}]`, true, false},
		{"malformed created response fails", http.StatusCreated, `{`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var creates atomic.Int32
			acs := userGrantsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == acsObjectTypesPath:
					_, err := w.Write([]byte(`[{"name":"project|organization","type_id":11}]`))
					assert.NoError(t, err)
				case r.Method == http.MethodPost && r.URL.Path == "/acs/v1/api/roles/"+roleID+"/members/users":
					creates.Add(1)
					var body models.CreateAssignUser
					if err := json.NewDecoder(r.Body).Decode(&body); !assert.NoError(t, err) {
						return
					}
					assert.Equal(t, []string{"jane"}, body.Usernames)
					assert.Equal(t, []string{project + "|" + org}, body.ObjectIds)
					require.NotNil(t, body.RoleID)
					assert.Equal(t, roleID, *body.RoleID)
					require.NotNil(t, body.ObjectTypeID)
					assert.Equal(t, int64(11), *body.ObjectTypeID)
					w.WriteHeader(tc.status)
					_, err := w.Write([]byte(tc.response))
					assert.NoError(t, err)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			})
			grant, err := acs.CreateProjectRoleGrant(context.Background(), org, project, roleID, "jane")
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, int32(1), creates.Load())
			if tc.owned {
				require.NotNil(t, grant)
				assert.Equal(t, grantID, grant.GrantID)
				assert.Equal(t, roleID, grant.RoleID)
				assert.Equal(t, project+"|"+org, grant.ObjectID)
			} else {
				assert.Nil(t, grant)
			}
		})
	}
}

func TestCreateOrganizationRoleGrant(t *testing.T) {
	const (
		roleID  = "11111111-2222-4333-8444-555555555555"
		grantID = "66666666-7777-4888-8999-aaaaaaaaaaaa"
		org     = "0014100000Te0G7AAJ"
	)
	for _, tc := range []struct {
		name, response string
		status         int
		wantErr, owned bool
	}{
		{"created organization grant", `[{"grant_id":"` + grantID + `","role_id":"` + roleID + `","username":"jane","object_id":"` + org + `","object_type_name":"organization"}]`, http.StatusCreated, false, true},
		{"grant ID only", `[{"grant_id":"` + grantID + `"}]`, http.StatusCreated, false, true},
		{"already assigned is not owned", `{"message":"already assigned"}`, http.StatusConflict, false, false},
		{"forbidden", `{"message":"forbidden"}`, http.StatusForbidden, true, false},
		{"dependency outage", `{"message":"unavailable"}`, http.StatusServiceUnavailable, true, false},
		{"unexpected success status", `[{"grant_id":"` + grantID + `"}]`, http.StatusOK, true, false},
		{"missing grant ID", `[{"scope_id":"not-a-grant"}]`, http.StatusCreated, true, false},
		{"different organization", `[{"grant_id":"` + grantID + `","object_id":"another-organization"}]`, http.StatusCreated, true, false},
		{"different role", `[{"grant_id":"` + grantID + `","role_id":"another-role"}]`, http.StatusCreated, true, false},
		{"different username", `[{"grant_id":"` + grantID + `","username":"another-user"}]`, http.StatusCreated, true, false},
		{"wrong scope type", `[{"grant_id":"` + grantID + `","object_type_name":"project|organization"}]`, http.StatusCreated, true, false},
		{"malformed response", `{`, http.StatusCreated, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var creates atomic.Int32
			acs := userGrantsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == acsObjectTypesPath:
					assert.NoError(t, json.NewEncoder(w).Encode([]map[string]interface{}{
						{"name": utils.ProjectOrgScope, "type_id": 11},
						{"name": "organization", "type_id": 12},
					}))
				case r.Method == http.MethodPost && r.URL.Path == "/acs/v1/api/roles/"+roleID+"/members/users":
					creates.Add(1)
					var body models.CreateAssignUser
					if err := json.NewDecoder(r.Body).Decode(&body); !assert.NoError(t, err) {
						return
					}
					assert.Equal(t, []string{"jane"}, body.Usernames)
					assert.Equal(t, []string{org}, body.ObjectIds)
					if assert.NotNil(t, body.RoleID) {
						assert.Equal(t, roleID, *body.RoleID)
					}
					if assert.NotNil(t, body.ObjectTypeID) {
						assert.Equal(t, int64(12), *body.ObjectTypeID)
					}
					w.WriteHeader(tc.status)
					_, err := w.Write([]byte(tc.response))
					assert.NoError(t, err)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			})
			grant, err := acs.CreateOrganizationRoleGrant(context.Background(), org, roleID, "jane")
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, int32(1), creates.Load())
			if tc.owned {
				require.NotNil(t, grant)
				assert.Equal(t, grantID, grant.GrantID)
				assert.Equal(t, roleID, grant.RoleID)
				assert.Equal(t, "jane", grant.Username)
				assert.Equal(t, org, grant.ObjectID)
				assert.Equal(t, "organization", grant.ObjectTypeName)
			} else {
				assert.Nil(t, grant)
			}
		})
	}
}

func TestDeleteUserGrantUsesExactUUID(t *testing.T) {
	const roleID, grantID = "11111111-2222-4333-8444-555555555555", "66666666-7777-4888-8999-aaaaaaaaaaaa"
	for _, status := range []int{http.StatusNoContent, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var deletes atomic.Int32
			acs := userGrantsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodDelete, r.Method)
				assert.Equal(t, "/acs/v1/api/roles/"+roleID+"/members/users/"+grantID, r.URL.Path)
				deletes.Add(1)
				w.WriteHeader(status)
			})
			err := acs.DeleteUserGrant(context.Background(), roleID, grantID)
			if status == http.StatusNoContent {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			assert.Equal(t, int32(1), deletes.Load())
			require.Error(t, acs.DeleteUserGrant(context.Background(), roleID, ""))
			assert.Equal(t, int32(1), deletes.Load(), "empty IDs must not trigger broader deletion")
		})
	}
}

func TestUserGrantRejectsWildcardAndCanceledRequests(t *testing.T) {
	acs := userGrantsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	})
	_, err := acs.CreateProjectRoleGrant(context.Background(), "*", "project", "role", "jane")
	require.Error(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = acs.CreateProjectRoleGrant(ctx, "org", "project", "role", "jane")
	require.ErrorIs(t, err, context.Canceled)
	_, err = acs.CreateOrganizationRoleGrant(ctx, "org", "role", "jane")
	require.ErrorIs(t, err, context.Canceled)
	for _, input := range []struct{ org, role, username string }{
		{"", "role", "jane"},
		{"*", "role", "jane"},
		{"project|org", "role", "jane"},
		{"org", "", "jane"},
		{"org", "role", " "},
	} {
		_, err = acs.CreateOrganizationRoleGrant(context.Background(), input.org, input.role, input.username)
		require.Error(t, err)
	}
	require.ErrorIs(t, acs.DeleteUserGrant(ctx, "role", "grant"), context.Canceled)
}

func TestHasProjectRoleMatchesUserRoleAndScope(t *testing.T) {
	var reads atomic.Int32
	acs := userGrantsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		assert.Equal(t, "/acs/v1/api/users/rolescopes/organization", r.URL.Path)
		assert.Equal(t, "false", r.Header.Get("X-LFX-CACHE"))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"userroles":{"jane":[{"role_id":"role","role_name":"cla-manager-designee","scopes":[{"grant_id":"grant","object_type_name":"project|organization","object_id":"project|org"},{"grant_id":"org-grant","object_type_name":"organization","object_id":"forged|org"}]}]},"metadata":{"TotalSize":1}}`))
		assert.NoError(t, err)
	})
	for _, tc := range []struct {
		username, role, project string
		want                    bool
	}{
		{"jane", utils.CLADesigneeRole, "project", true},
		{"other-user", utils.CLADesigneeRole, "project", false},
		{"jane", utils.CLAManagerRole, "project", false},
		{"jane", utils.CLADesigneeRole, "other-project", false},
		{"jane", utils.CLADesigneeRole, "forged", false},
		{"jane", utils.CLADesigneeRole, "", false},
	} {
		got, err := acs.HasProjectRole(context.Background(), "org", tc.project, tc.role, tc.username)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got)
		got, err = acs.HasAnyProjectRole(context.Background(), "org", []string{"unmapped", tc.project}, tc.role, tc.username)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got)
	}
	before := reads.Load()
	got, err := acs.HasAnyProjectRole(context.Background(), "org", nil, utils.CLADesigneeRole, "jane")
	require.NoError(t, err)
	assert.False(t, got)
	assert.Equal(t, before, reads.Load(), "no mapped projects must not fetch organization grants")
}
