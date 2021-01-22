// +build !aws_lambda

// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
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

// skipPermissionChecks determines if we can skip the permissions checks when running locally (for testing)
func skipPermissionChecks() bool {
	environmentKey := "DISABLE_LOCAL_PERMISSION_CHECKS"
	disablePermissionChecks, err := strconv.ParseBool(os.Getenv(environmentKey))
	if err != nil {
		log.Warnf("unable to check local permissions flag: %t", disablePermissionChecks)
		return false
	}

	if disablePermissionChecks {
		log.Debugf("skipping permission checks since %s is set to true", environmentKey)
	}

	return disablePermissionChecks
}

// IsUserAuthorizedForOrganization helper function for determining if the user is authorized for this company
func IsUserAuthorizedForOrganization(user *auth.User, companySFID string, adminScopeAllowed bool) bool {
	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		return true
	}

	if adminScopeAllowed && user.Admin {
		return true
	}

	return user.IsUserAuthorizedForOrganizationScope(companySFID)
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

	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		log.WithFields(f).Debug("skipping permissions check")
		return true
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("admin scope is allowed and admin scope set for user")
		return true
	}

	log.WithFields(f).Debug("checking scope...")
	val := user.IsUserAuthorized(auth.Project, projectSFID, true)
	log.WithFields(f).Debugf("user allowed: %T", val)
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

	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		log.WithFields(f).Debug("skipping permissions check")
		return true
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("admin scope is allowed and admin scope set for user")
		return true
	}

	log.WithFields(f).Debug("checking scope...")
	val := user.IsUserAuthorizedForProjectScope(projectSFID)
	log.WithFields(f).Debugf("user allowed: %T", val)
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
	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		log.WithFields(f).Debug("skipping permissions check")
		return true
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
func IsUserAuthorizedForProjectOrganization(user *auth.User, projectSFID, companySFID string, adminScopeAllowed bool) bool {
	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		return true
	}

	if adminScopeAllowed && user.Admin {
		return true
	}

	// Previously, we checked for user.Allowed, which is currently not used (future flag that is currently not implemented)
	return user.IsUserAuthorizedByProject(projectSFID, companySFID)
}

// IsUserAuthorizedForAnyProjectOrganization helper function for determining if the user is authorized for any of the specified projects with scope of project + organization
func IsUserAuthorizedForAnyProjectOrganization(user *auth.User, projectSFIDs []string, companySFID string, adminScopeAllowed bool) bool {
	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		return true
	}

	for _, projectSFID := range projectSFIDs {
		if IsUserAuthorizedForProjectOrganizationTree(user, projectSFID, companySFID, adminScopeAllowed) {
			return true
		}
		if IsUserAuthorizedForProjectOrganization(user, projectSFID, companySFID, adminScopeAllowed) {
			return true
		}
	}

	return false
}

// IsUserAuthorizedForProjectOrganizationTree helper function for determining if the user is authorized for this project organization scope and nested projects/orgs
func IsUserAuthorizedForProjectOrganizationTree(user *auth.User, projectSFID, companySFID string, adminScopeAllowed bool) bool {
	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		return true
	}

	if adminScopeAllowed && user.Admin {
		return true
	}

	// Previously, we checked for user.Admin - admins should be in a separate role
	// Previously, we checked for user.Allowed, which is currently not used (future flag that is currently not implemented)
	return user.IsUserAuthorized(auth.ProjectOrganization, projectSFID+"|"+companySFID, true)
}
