// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package acs_service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initFakeToken points the shared token package at a fake Auth0 endpoint for the test.
func initFakeToken(t *testing.T) {
	auth0 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "fake-token", "token_type": "Bearer", "expires_in": 3600}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(auth0.Close)
	token.Init("id", "secret", auth0.URL, "aud")
}

func TestListOrgGrants(t *testing.T) {
	const org = "0014100000Te0G7AAJ"
	initFakeToken(t)

	pages := map[string]map[string]interface{}{
		"0": {
			"userroles": map[string]interface{}{
				"bob": []map[string]interface{}{{
					"role_id": "r-cm", "role_name": "cla-manager",
					"scopes": []map[string]interface{}{
						{"grant_id": "g1", "scope_id": "s1", "object_type_name": "project|organization", "object_id": "a09P000000DsCE5IAN|" + org, "object_name": "SUN|Infosys"},
						{"grant_id": "g-other", "scope_id": "s-other", "object_type_name": "project|organization", "object_id": "a09P000000DsCE5IAN|0014100000Other00", "object_name": "SUN|Other"},
					},
				}},
				"alice": []map[string]interface{}{{
					"role_id": "r-admin", "role_name": "company-admin",
					"scopes": []map[string]interface{}{{"grant_id": "g2", "scope_id": "s2", "object_type_name": "organization", "object_id": org, "object_name": "Infosys"}},
				}},
			},
			"metadata": map[string]int{"Offset": 0, "PageSize": 2, "TotalSize": 3},
		},
		"2": {
			"userroles": map[string]interface{}{
				"carol": []map[string]interface{}{{
					"role_id": "r-sig", "role_name": "cla-signatory",
					"scopes": []map[string]interface{}{{"grant_id": "g3", "scope_id": "s3", "object_type_name": "project|organization", "object_id": "a09P000000DsCE5IAN|" + org}},
				}},
			},
			"metadata": map[string]int{"Offset": 2, "PageSize": 1, "TotalSize": 3},
		},
	}
	var requests []*http.Request
	acs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		if r.URL.Path != "/acs/v1/api/users/rolescopes/organization" {
			http.NotFound(w, r)
			return
		}
		page, ok := pages[r.URL.Query().Get("offset")]
		if !ok {
			http.Error(w, "unexpected offset", http.StatusBadRequest)
			return
		}
		if err := json.NewEncoder(w).Encode(page); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(acs.Close)

	client := &Client{apiGwURL: acs.URL}
	grants, err := client.ListOrgGrants(context.Background(), org)
	require.NoError(t, err)
	require.Len(t, requests, 2, "follows metadata.TotalSize across pages")
	for _, r := range requests {
		assert.Equal(t, "Bearer fake-token", r.Header.Get("Authorization"))
		assert.Equal(t, "false", r.Header.Get("X-LFX-CACHE"))
		assert.Equal(t, org, r.URL.Query().Get("orgid"))
		assert.Equal(t, "all", r.URL.Query().Get("scopetype"))
		assert.Equal(t, strconv.Itoa(orgGrantsPageSize), r.URL.Query().Get("limit"))
	}
	assert.Equal(t, []OrgGrant{
		{Username: "alice", RoleName: "company-admin", RoleID: "r-admin", GrantID: "g2", ScopeID: "s2", ObjectTypeName: "organization", ObjectID: org, ObjectName: "Infosys"},
		{Username: "bob", RoleName: "cla-manager", RoleID: "r-cm", GrantID: "g1", ScopeID: "s1", ObjectTypeName: "project|organization", ObjectID: "a09P000000DsCE5IAN|" + org, ObjectName: "SUN|Infosys"},
		{Username: "carol", RoleName: "cla-signatory", RoleID: "r-sig", GrantID: "g3", ScopeID: "s3", ObjectTypeName: "project|organization", ObjectID: "a09P000000DsCE5IAN|" + org},
	}, grants, "sorted, foreign-organization scopes dropped")
	assert.Equal(t, "a09P000000DsCE5IAN", grants[1].ProjectSFID())
	assert.Equal(t, "", grants[0].ProjectSFID())
}

func TestListOrgGrantsErrors(t *testing.T) {
	initFakeToken(t)
	status := http.StatusInternalServerError
	acs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if _, err := w.Write([]byte(`{"Message":"boom"}`)); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(acs.Close)
	client := &Client{apiGwURL: acs.URL}

	_, err := client.ListOrgGrants(context.Background(), "0014100000Te0G7AAJ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")

	status = http.StatusNotFound
	grants, err := client.ListOrgGrants(context.Background(), "0014100000Te0G7AAJ")
	require.NoError(t, err)
	assert.Empty(t, grants)

	_, err = client.ListOrgGrants(context.Background(), " ")
	assert.Error(t, err)
}
