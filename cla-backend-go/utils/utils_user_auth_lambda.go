// +build aws_lambda

// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"
	"strings"

	"github.com/LF-Engineering/lfx-kit/auth"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
)

const (
	// ALLOW_ADMIN_SCOPE indicates that a given permissions check allows for admins to access to that resource
	ALLOW_ADMIN_SCOPE = true // nolint
	// DISALLOW_ADMIN_SCOPE indicates that a given permissions check does not allow for admins to access to that resource
	DISALLOW_ADMIN_SCOPE = false // nolint
)

// IsUserAdmin helper function for determining if the user is an admin
func IsUserAdmin(user *auth.User) bool {
	return user.Admin
}

// IsUserAuthorizedForOrganization helper function for determining if the user is authorized for this company
func IsUserAuthorizedForOrganization(ctx context.Context, user *auth.User, companySFID string, adminScopeAllowed bool) bool {
	f := logrus.Fields{
		"functionName":      "utils.IsUserAuthorizedForOrganization",
		XREQUESTID:          ctx.Value(XREQUESTID),
		"userName":          user.UserName,
		"userEmail":         user.Email,
		"companySFID":       companySFID,
		"adminScopeAllowed": adminScopeAllowed,
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("user is authorized - admin scope is allowed and admin scope set for user")
		return true
	}

	val := user.IsUserAuthorizedForOrganizationScope(companySFID)
	if val {
		log.WithFields(f).Debugf("user is authorized for companySFID: %s", companySFID)
	} else {
		log.WithFields(f).Debugf("user is not authorized for companySFID: %s", companySFID)
	}
	return val
}

// IsUserAuthorizedForProjectTree helper function for determining if the user is authorized for this project hierarchy/tree
func IsUserAuthorizedForProjectTree(ctx context.Context, user *auth.User, projectSFID string, adminScopeAllowed bool) bool {
	f := logrus.Fields{
		"functionName":      "utils.IsUserAuthorizedForProjectTree",
		XREQUESTID:          ctx.Value(XREQUESTID),
		"userName":          user.UserName,
		"userEmail":         user.Email,
		"projectSFID":       projectSFID,
		"adminScopeAllowed": adminScopeAllowed,
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("user is authorized - admin scope is allowed and admin scope set for user")
		return true
	}

	log.WithFields(f).Debugf("checking project scope for projectSFID: %s...", projectSFID)
	val := user.IsUserAuthorized(auth.Project, projectSFID, true)
	if val {
		log.WithFields(f).Debugf("user is authorized for projectSFID: %s", projectSFID)
	} else {
		log.WithFields(f).Debugf("user is not authorized for projectSFID: %s", projectSFID)
	}
	return val
}

// IsUserAuthorizedForProject helper function for determining if the user is authorized for this project
func IsUserAuthorizedForProject(ctx context.Context, user *auth.User, projectSFID string, adminScopeAllowed bool) bool {
	f := logrus.Fields{
		"functionName":      "utils.IsUserAuthorizedForProject",
		XREQUESTID:          ctx.Value(XREQUESTID),
		"userName":          user.UserName,
		"userEmail":         user.Email,
		"projectSFID":       projectSFID,
		"adminScopeAllowed": adminScopeAllowed,
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("user is authorized - admin scope is allowed and admin scope set for user")
		return true
	}

	log.WithFields(f).Debugf("checking project scope for projectSFID: %s...", projectSFID)
	val := user.IsUserAuthorizedForProjectScope(projectSFID)
	if val {
		log.WithFields(f).Debugf("user is authorized for projectSFID: %s", projectSFID)
	} else {
		log.WithFields(f).Debugf("user is not authorized for projectSFID: %s", projectSFID)
	}
	return val
}

// IsUserAuthorizedForAnyProjects helper function for determining if the user is authorized for any of the specified projects
func IsUserAuthorizedForAnyProjects(ctx context.Context, user *auth.User, projectSFIDs []string, adminScopeAllowed bool) bool {
	f := logrus.Fields{
		"functionName":      "utils.IsUserAuthorizedForAnyProjects",
		XREQUESTID:          ctx.Value(XREQUESTID),
		"userName":          user.UserName,
		"userEmail":         user.Email,
		"projectSFIDs":      strings.Join(projectSFIDs, ","),
		"adminScopeAllowed": adminScopeAllowed,
	}

	for _, projectSFID := range projectSFIDs {
		log.WithFields(f).Debugf("checking project tree scope for: %s...", projectSFID)
		if IsUserAuthorizedForProjectTree(ctx, user, projectSFID, adminScopeAllowed) {
			log.WithFields(f).Debugf("project tree scope check passed for: %s...", projectSFID)
			return true
		}
		log.WithFields(f).Debugf("checking project scope for: %s...", projectSFID)
		if IsUserAuthorizedForProject(ctx, user, projectSFID, adminScopeAllowed) {
			log.WithFields(f).Debugf("project scope check passed for: %s...", projectSFID)
			return true
		}
	}

	log.WithFields(f).Debugf("project scope checks failed for: %s...", strings.Join(projectSFIDs, ","))
	return false
}

// IsUserAuthorizedForProjectOrganization helper function for determining if the user is authorized for this project organization scope
func IsUserAuthorizedForProjectOrganization(ctx context.Context, user *auth.User, projectSFID, companySFID string, adminScopeAllowed bool) bool {
	f := logrus.Fields{
		"functionName":      "utils.IsUserAuthorizedForProjectOrganization",
		XREQUESTID:          ctx.Value(XREQUESTID),
		"userName":          user.UserName,
		"userEmail":         user.Email,
		"projectSFID":       projectSFID,
		"companySFID":       companySFID,
		"adminScopeAllowed": adminScopeAllowed,
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("user is authorized - admin scope is allowed and admin scope set for user")
		return true
	}

	val := user.IsUserAuthorizedByProject(projectSFID, companySFID)
	if val {
		log.WithFields(f).Debugf("user is authorized for projectSFID: %s + companySFID: %s", projectSFID, companySFID)
	} else {
		log.WithFields(f).Debugf("user is not authorized for projectSFID: %s + companySFID: %s", projectSFID, companySFID)
	}
	return val
}

// IsUserAuthorizedForAnyProjectOrganization helper function for determining if the user is authorized for any of the specified projects with scope of project + organization
func IsUserAuthorizedForAnyProjectOrganization(ctx context.Context, user *auth.User, projectSFIDs []string, companySFID string, adminScopeAllowed bool) bool {
	f := logrus.Fields{
		"functionName":      "utils.IsUserAuthorizedForAnyProjectOrganization",
		XREQUESTID:          ctx.Value(XREQUESTID),
		"userName":          user.UserName,
		"userEmail":         user.Email,
		"projectSFIDs":      strings.Join(projectSFIDs, ","),
		"companySFID":       companySFID,
		"adminScopeAllowed": adminScopeAllowed,
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("user is authorized - admin scope is allowed and admin scope set for user")
		return true
	}

	for _, projectSFID := range projectSFIDs {
		if IsUserAuthorizedForProjectOrganizationTree(ctx, user, projectSFID, companySFID, adminScopeAllowed) {
			log.WithFields(f).Debugf("user is authorized for projectSFID: %s + companySFID: %s tree", projectSFID, companySFID)
			return true
		}
		if IsUserAuthorizedForProjectOrganization(ctx, user, projectSFID, companySFID, adminScopeAllowed) {
			log.WithFields(f).Debugf("user is authorized for projectSFID: %s + companySFID: %s", projectSFID, companySFID)
			return true
		}
	}

	log.WithFields(f).Debugf("user is not authorized for any projectSFID: %s + companySFID: %s", strings.Join(projectSFIDs, ","), companySFID)
	return false
}

// IsUserAuthorizedForProjectOrganizationTree helper function for determining if the user is authorized for this project organization scope and nested projects/orgs
func IsUserAuthorizedForProjectOrganizationTree(ctx context.Context, user *auth.User, projectSFID, companySFID string, adminScopeAllowed bool) bool {
	f := logrus.Fields{
		"functionName":      "utils.IsUserAuthorizedForProjectOrganizationTree",
		XREQUESTID:          ctx.Value(XREQUESTID),
		"userName":          user.UserName,
		"userEmail":         user.Email,
		"projectSFID":       projectSFID,
		"companySFID":       companySFID,
		"adminScopeAllowed": adminScopeAllowed,
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("user is authorized - admin scope is allowed and admin scope set for user")
		return true
	}

	val := user.IsUserAuthorized(auth.ProjectOrganization, projectSFID+"|"+companySFID, true)
	if val {
		log.WithFields(f).Debugf("user is authorized for projectSFID: %s + companySFID: %s tree", projectSFID, companySFID)
	} else {
		log.WithFields(f).Debugf("user is not authorized for projectSFID: %s + companySFID: %s tree", projectSFID, companySFID)
	}
	return val
}
