// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package acs_service

import (
	"context"
	"errors"
	"fmt"
)

// ErrGrantNotFound reports that ACS holds no grant for the user, role and object.
var ErrGrantNotFound = errors.New("acs grant not found")

// GrantIDFor returns the grant id of username's roleName on objectID, or ErrGrantNotFound.
func GrantIDFor(grants []OrgGrant, objectID, roleName, username string) (string, error) {
	for _, g := range grants {
		if g.Username == username && g.RoleName == roleName && g.ObjectID == objectID {
			if g.GrantID == "" {
				return "", fmt.Errorf("%w: role %s, user %s, object %s (grant without id)", ErrGrantNotFound, roleName, username, objectID)
			}
			return g.GrantID, nil
		}
	}
	return "", fmt.Errorf("%w: role %s, user %s, object %s", ErrGrantNotFound, roleName, username, objectID)
}

// FindUserGrantID reads the organization's grants fresh from ACS and returns the grant id of username's
// roleName on projectSFID|orgSFID (lfx-self-serve#3515).
func (ac *Client) FindUserGrantID(ctx context.Context, orgSFID, projectSFID, roleName, username string) (string, error) {
	grants, err := ac.ListOrgGrants(ctx, orgSFID)
	if err != nil {
		return "", err
	}
	return GrantIDFor(grants, projectSFID+"|"+orgSFID, roleName, username)
}
