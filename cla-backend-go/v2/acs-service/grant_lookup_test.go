// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package acs_service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const orgGrantsPath = "/acs/v1/api/users/rolescopes/organization"

func TestGrantIDForMatchesUserRoleAndObjectExactly(t *testing.T) {
	grants := []OrgGrant{
		{Username: "bob", RoleName: "cla-manager", ObjectID: "p1|o1", GrantID: "g-bob-p1"},
		{Username: "bobby", RoleName: "cla-manager", ObjectID: "p1|o1", GrantID: "g-bobby"},
		{Username: "bob", RoleName: "cla-manager-designee", ObjectID: "p1|o1", GrantID: "g-bob-designee"},
		{Username: "bob", RoleName: "cla-manager", ObjectID: "p2|o1", GrantID: "g-bob-p2"},
		{Username: "carol", RoleName: "cla-manager", ObjectID: "p1|o1", GrantID: ""},
	}
	id, err := GrantIDFor(grants, "p1|o1", "cla-manager", "bob")
	require.NoError(t, err)
	assert.Equal(t, "g-bob-p1", id)
	_, err = GrantIDFor(grants, "p3|o1", "cla-manager", "bob")
	assert.True(t, errors.Is(err, ErrGrantNotFound), "a missing grant is an error, never an empty id")
	_, err = GrantIDFor(nil, "p1|o1", "cla-manager", "bob")
	assert.True(t, errors.Is(err, ErrGrantNotFound))
	id, err = GrantIDFor(grants, "p1|o1", "cla-manager", "carol")
	assert.Empty(t, id)
	assert.True(t, errors.Is(err, ErrGrantNotFound), "an entry without a grant id is never returned as a success")
}

// the lookup reads every page of the organization's grants from ACS, bypassing the gateway cache (lfx-self-serve#3515)
func TestFindUserGrantIDReadsAllPagesFresh(t *testing.T) {
	const org = "0014100000Te0G7AAJ"
	initFakeToken(t)
	pages := map[string]map[string]interface{}{
		"0": {
			"userroles": map[string]interface{}{
				"alice": []map[string]interface{}{{"role_id": "r-cm", "role_name": "cla-manager", "scopes": []map[string]interface{}{{"grant_id": "g-alice", "scope_id": "s1", "object_type_name": "project|organization", "object_id": "a09P000000DsCE5IAN|" + org}}}},
			},
			"metadata": map[string]int{"Offset": 0, "PageSize": 1, "TotalSize": 2},
		},
		"1": {
			"userroles": map[string]interface{}{
				"zed": []map[string]interface{}{{"role_id": "r-cm", "role_name": "cla-manager", "scopes": []map[string]interface{}{{"grant_id": "g-zed", "scope_id": "s1", "object_type_name": "project|organization", "object_id": "a09P000000DsCE5IAN|" + org}}}},
			},
			"metadata": map[string]int{"Offset": 1, "PageSize": 1, "TotalSize": 2},
		},
	}
	var requests []*http.Request
	acs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		page, ok := pages[r.URL.Query().Get("offset")]
		if r.URL.Path != orgGrantsPath || !ok {
			http.NotFound(w, r)
			return
		}
		if err := json.NewEncoder(w).Encode(page); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(acs.Close)
	client := &Client{apiGwURL: acs.URL}

	id, err := client.FindUserGrantID(context.Background(), org, "a09P000000DsCE5IAN", "cla-manager", "zed")
	require.NoError(t, err)
	assert.Equal(t, "g-zed", id, "a user beyond the first page is found")
	require.Len(t, requests, 2)
	for _, r := range requests {
		assert.Equal(t, "false", r.Header.Get("X-LFX-CACHE"))
	}

	_, err = client.FindUserGrantID(context.Background(), org, "a09P000000DsCE5IAN", "cla-manager", "nobody")
	assert.True(t, errors.Is(err, ErrGrantNotFound))
}
