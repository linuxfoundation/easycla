// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package organization_service

import (
	"context"
	"errors"
	"strings"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	acsService "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	memberService "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/models"
)

// GetOrganizationForCLA uses a saved company's display metadata while its Account awaits ingestion.
// It must not be used for authorization, signing-entity validation or sanctions domain resolution.
func (osc *Client) GetOrganizationForCLA(ctx context.Context, companySFID, companyName string) (*models.Organization, error) {
	org, err := osc.GetOrganization(ctx, companySFID)
	var notFound *organizations.GetOrgNotFound
	if !acsService.DirectCLARolesEnabled() || !errors.As(err, &notFound) ||
		strings.TrimSpace(companySFID) == "" || strings.TrimSpace(companyName) == "" {
		return org, err
	}
	sfid, valid := memberService.SFID18(companySFID)
	if !valid || !strings.HasPrefix(sfid, "001") {
		return org, err
	}
	log.WithField("companySFID", companySFID).Debug("using saved company metadata before Organization Service ingestion")
	return &models.Organization{ID: companySFID, Name: companyName}, nil
}

func (osc *Client) IsCLAUserHaveRoleScope(ctx context.Context, roleName, userID, username, orgID, projectSFID string) (bool, error) {
	if acsService.DirectCLARolesEnabled() {
		return acsService.GetClient().HasProjectRole(ctx, orgID, projectSFID, roleName, username)
	}
	return osc.IsUserHaveRoleScope(ctx, roleName, userID, orgID, projectSFID)
}

func (osc *Client) CreateCLAUserRoleScope(ctx context.Context, email, username, projectSFID, orgID, roleID string) error {
	if acsService.DirectCLARolesEnabled() {
		_, err := acsService.GetClient().CreateProjectRoleGrant(ctx, orgID, projectSFID, roleID, username)
		return err
	}
	return osc.CreateOrgUserRoleOrgScopeProjectOrg(ctx, email, projectSFID, orgID, roleID)
}

func (osc *Client) CreateCLAOrgUserRoleScope(ctx context.Context, email, username, orgID, roleID string) error {
	if acsService.DirectCLARolesEnabled() {
		_, err := acsService.GetClient().CreateOrganizationRoleGrant(ctx, orgID, roleID, username)
		return err
	}
	return osc.CreateOrgUserRoleOrgScope(ctx, email, orgID, roleID)
}

func (osc *Client) DeleteCLAUserRoleScope(ctx context.Context, orgID, roleID, grantID string, username, email *string) error {
	if acsService.DirectCLARolesEnabled() {
		return acsService.GetClient().DeleteUserGrant(ctx, roleID, grantID)
	}
	return osc.DeleteOrgUserRoleOrgScopeProjectOrg(ctx, orgID, roleID, grantID, username, email)
}

func (osc *Client) DeleteCLARolePermissions(ctx context.Context, orgID, projectSFID, roleName string, authUser *auth.User) error {
	if !acsService.DirectCLARolesEnabled() {
		return osc.DeleteRolePermissions(ctx, orgID, projectSFID, roleName, authUser)
	}
	acs := acsService.GetClient()
	grants, err := acs.ListOrgGrants(ctx, orgID)
	if err != nil {
		return err
	}
	for _, grant := range grants {
		if grant.RoleName != roleName || grant.ObjectTypeName != utils.ProjectOrgScope ||
			grant.ObjectID != projectSFID+"|"+orgID {
			continue
		}
		if err = acs.DeleteUserGrant(ctx, grant.RoleID, grant.GrantID); err != nil {
			return err
		}
		v1EventService.LogEvent(&events.LogEventArgs{
			EventType:   events.ClaManagerRoleDeleted,
			ProjectID:   projectSFID,
			ProjectSFID: projectSFID,
			CompanyID:   orgID,
			LfUsername:  authUser.UserName,
			UserID:      authUser.UserName,
			EventData: &events.ClaManagerRoleDeletedData{
				Role:     roleName,
				Scope:    grant.ObjectTypeName,
				UserName: grant.Username,
			},
		})
	}
	return nil
}
