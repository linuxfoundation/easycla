// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package acs_service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/sirupsen/logrus"
)

// OrgGrant is one ACS role grant whose scope is an organization ("organization" object type) or
// a project|organization pair.
type OrgGrant struct {
	Username       string `json:"username"`
	RoleName       string `json:"role_name"`
	RoleID         string `json:"role_id"`
	GrantID        string `json:"grant_id"`
	ScopeID        string `json:"scope_id"`
	ObjectTypeName string `json:"object_type_name"`
	ObjectID       string `json:"object_id"`
	ObjectName     string `json:"object_name,omitempty"`
}

// ProjectSFID returns the project part of a project|organization scope, or "" for organization scopes.
func (g OrgGrant) ProjectSFID() string {
	if i := strings.Index(g.ObjectID, "|"); i >= 0 {
		return g.ObjectID[:i]
	}
	return ""
}

const orgGrantsPageSize = 100

var orgGrantsHTTPClient = &http.Client{Timeout: 30 * time.Second}

// ListOrgGrants returns every role grant of every user scoped to the organization, read with the
// ACS cache bypassed (X-LFX-CACHE: false) so freshly deleted grants are not reported.
func (ac *Client) ListOrgGrants(ctx context.Context, orgSFID string) ([]OrgGrant, error) {
	f := logrus.Fields{
		"functionName": "acs_service.ListOrgGrants",
		"orgSFID":      orgSFID,
	}
	if strings.TrimSpace(orgSFID) == "" {
		return nil, fmt.Errorf("organization sfid is required")
	}
	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem obtaining token")
		return nil, err
	}

	var grants []OrgGrant
	for offset := 0; ; {
		var page struct {
			Userroles map[string][]struct {
				RoleID   string `json:"role_id"`
				RoleName string `json:"role_name"`
				Scopes   []struct {
					GrantID        string `json:"grant_id"`
					ScopeID        string `json:"scope_id"`
					ObjectTypeName string `json:"object_type_name"`
					ObjectID       string `json:"object_id"`
					ObjectName     string `json:"object_name"`
				} `json:"scopes"`
			} `json:"userroles"`
			Metadata struct {
				Offset    int `json:"Offset"`
				PageSize  int `json:"PageSize"`
				TotalSize int `json:"TotalSize"`
			} `json:"metadata"`
		}
		query := url.Values{"orgid": {orgSFID}, "scopetype": {"all"}, "limit": {fmt.Sprint(orgGrantsPageSize)}, "offset": {fmt.Sprint(offset)}}
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/acs/v1/api/users/rolescopes/organization?%s", ac.apiGwURL, query.Encode()), nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-LFX-CACHE", "false")
		resp, doErr := orgGrantsHTTPClient.Do(req)
		if doErr != nil {
			log.WithFields(f).WithError(doErr).Warn("problem listing organization grants")
			return nil, doErr
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusNotFound {
			return grants, nil
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return nil, fmt.Errorf("acs rolescopes/organization returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body[:min(len(body), 512)])))
		}
		if err = json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decoding acs rolescopes/organization response: %w", err)
		}
		for username, roles := range page.Userroles {
			for _, role := range roles {
				for _, scope := range role.Scopes {
					if scope.ObjectID != orgSFID && !strings.HasSuffix(scope.ObjectID, "|"+orgSFID) {
						continue
					}
					grants = append(grants, OrgGrant{
						Username:       username,
						RoleName:       role.RoleName,
						RoleID:         role.RoleID,
						GrantID:        scope.GrantID,
						ScopeID:        scope.ScopeID,
						ObjectTypeName: scope.ObjectTypeName,
						ObjectID:       scope.ObjectID,
						ObjectName:     scope.ObjectName,
					})
				}
			}
		}
		if len(page.Userroles) == 0 {
			break
		}
		offset += len(page.Userroles)
		if offset >= page.Metadata.TotalSize {
			break
		}
	}
	sort.Slice(grants, func(i, j int) bool {
		a, b := grants[i], grants[j]
		if a.Username != b.Username {
			return a.Username < b.Username
		}
		if a.RoleName != b.RoleName {
			return a.RoleName < b.RoleName
		}
		return a.ObjectID < b.ObjectID
	})
	log.WithFields(f).Debugf("found %d organization grants", len(grants))
	return grants, nil
}
