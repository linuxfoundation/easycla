// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package acs_service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service/models"
)

// DirectCLARolesEnabled uses the same rollout gate as member-service company creation.
func DirectCLARolesEnabled() bool {
	return config.GetConfig().MemberService.CreateCompanyEnabled
}

// HasProjectRole reads the exact user's project|organization grant without the ACS cache.
func (ac *Client) HasProjectRole(ctx context.Context, orgSFID, projectSFID, roleName, username string) (bool, error) {
	return ac.HasAnyProjectRole(ctx, orgSFID, []string{projectSFID}, roleName, username)
}

// HasAnyProjectRole checks the mapped project scopes with one fresh organization-grant read.
func (ac *Client) HasAnyProjectRole(ctx context.Context, orgSFID string, projectSFIDs []string, roleName, username string) (bool, error) {
	if len(projectSFIDs) == 0 {
		return false, nil
	}
	grants, err := ac.ListOrgGrants(ctx, orgSFID)
	if err != nil {
		return false, err
	}
	objectIDs := make(map[string]struct{}, len(projectSFIDs))
	for _, projectSFID := range projectSFIDs {
		objectIDs[projectSFID+"|"+orgSFID] = struct{}{}
	}
	for _, grant := range grants {
		if grant.Username == username && grant.RoleName == roleName &&
			grant.ObjectTypeName == utils.ProjectOrgScope {
			if _, matched := objectIDs[grant.ObjectID]; matched {
				return true, nil
			}
		}
	}
	return false, nil
}

// CreateProjectRoleGrant returns the grant created by this request; nil means it already existed.
// Callers must resolve username through the platform user service before granting a role.
func (ac *Client) CreateProjectRoleGrant(ctx context.Context, orgSFID, projectSFID, roleID, username string) (*OrgGrant, error) {
	if strings.TrimSpace(orgSFID) == "" || strings.TrimSpace(projectSFID) == "" ||
		strings.TrimSpace(roleID) == "" || strings.TrimSpace(username) == "" ||
		strings.ContainsAny(orgSFID+projectSFID, "|*") {
		err := fmt.Errorf("an exact organization, project, role and username are required for an ACS grant")
		log.WithError(err).Warn("invalid project|organization grant")
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	objectTypeID, err := ac.GetObjectTypeIDByName(utils.ProjectOrgScope)
	if err != nil {
		return nil, err
	}
	typeID := int64(objectTypeID)
	objectID := projectSFID + "|" + orgSFID
	body, err := json.Marshal(&models.CreateAssignUser{
		RoleID:       &roleID,
		ObjectTypeID: &typeID,
		ObjectIds:    []string{objectID},
		Usernames:    []string{username},
	})
	if err != nil {
		return nil, err
	}
	status, response, err := ac.userGrantRequest(ctx, http.MethodPost, "/roles/"+url.PathEscape(roleID)+"/members/users", body)
	if err != nil {
		return nil, err
	}
	if status == http.StatusConflict {
		return nil, nil
	}
	if status != http.StatusCreated {
		log.WithField("status", status).Warn("ACS create user grant failed")
		return nil, fmt.Errorf("ACS create user grant returned %d: %s", status, strings.TrimSpace(string(response[:min(len(response), 512)])))
	}
	var grants []OrgGrant
	if err = json.Unmarshal(response, &grants); err != nil {
		return nil, fmt.Errorf("decoding created ACS user grant: %w", err)
	}
	if len(grants) != 1 || grants[0].GrantID == "" {
		return nil, fmt.Errorf("ACS create user grant did not return exactly one grant ID")
	}
	grant := grants[0]
	if (grant.RoleID != "" && grant.RoleID != roleID) ||
		(grant.Username != "" && grant.Username != username) ||
		(grant.ObjectID != "" && grant.ObjectID != objectID) ||
		(grant.ObjectTypeName != "" && grant.ObjectTypeName != utils.ProjectOrgScope) {
		return nil, fmt.Errorf("ACS created grant does not match the requested user, role and project|organization")
	}
	grant.RoleID, grant.Username = roleID, username
	grant.ObjectID, grant.ObjectTypeName = objectID, utils.ProjectOrgScope
	return &grant, nil
}

// DeleteUserGrant revokes one exact grant, not the role definition or every grant on its scope.
func (ac *Client) DeleteUserGrant(ctx context.Context, roleID, grantID string) error {
	if strings.TrimSpace(roleID) == "" || strings.TrimSpace(grantID) == "" {
		err := fmt.Errorf("role ID and grant ID are required to revoke an ACS user grant")
		log.WithError(err).Warn("invalid user grant revocation")
		return err
	}
	status, response, err := ac.userGrantRequest(ctx, http.MethodDelete, "/roles/"+url.PathEscape(roleID)+"/members/users/"+url.PathEscape(grantID), nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent {
		log.WithField("status", status).Warn("ACS delete user grant failed")
		return fmt.Errorf("ACS delete user grant returned %d: %s", status, strings.TrimSpace(string(response[:min(len(response), 512)])))
	}
	return nil
}

// The generated SDK incorrectly models grant/role IDs and response object IDs as integers.
func (ac *Client) userGrantRequest(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	tok, err := token.GetToken()
	if err != nil {
		log.WithError(err).Warn("problem obtaining token for ACS user grant")
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(ac.apiGwURL, "/")+"/acs/v1/api"+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if requestID, ok := ctx.Value(utils.XREQUESTID).(string); ok {
		req.Header.Set(utils.XREQUESTID, requestID)
	}
	response, err := orgGrantsHTTPClient.Do(req)
	if err != nil {
		log.WithError(err).Warn("ACS user grant request failed")
		return 0, nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	_ = response.Body.Close()
	return response.StatusCode, data, readErr
}
