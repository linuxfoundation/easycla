//go:build !aws_lambda
// +build !aws_lambda

// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
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
func IsUserAuthorizedForOrganization(ctx context.Context, user *auth.User, companySFID string, adminScopeAllowed bool) bool {
	f := logrus.Fields{
		"functionName":      "utils.IsUserAuthorizedForOrganization",
		XREQUESTID:          ctx.Value(XREQUESTID),
		"userName":          user.UserName,
		"userEmail":         user.Email,
		"companySFID":       companySFID,
		"adminScopeAllowed": adminScopeAllowed,
	}

	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		log.WithFields(f).Debug("skipping permissions check")
		return true
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("user is authorized - admin scope is allowed and admin scope set for user")
		return true
	}

	val := user.IsUserAuthorizedForOrganizationScope(companySFID)
	if val {
		log.WithFields(f).Debugf("user '%s' is authorized for companySFID: %s  admin flag=%t",
			user.UserName, companySFID, user.Admin)
	} else {
		var scopeInfo string
		for i, scope := range user.Scopes {
			scopeInfo = fmt.Sprintf("%sscope[%d] = {type=%s, id=%s, level=%s, role=%s, related=[%s]} ",
				scopeInfo, i, scope.Type, scope.ID, scope.Level, scope.Role, strings.Join(scope.Related, ","))
		}
		log.WithFields(f).Debugf("user '%s' is not authorized for companySFID: %s, admin flag=%t, scopeInfo: %s",
			user.UserName, companySFID, user.Admin, scopeInfo)
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

	log.WithFields(f).Debugf("checking user auth for project tree")

	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		log.WithFields(f).Debug("skipping permissions check")
		return true
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("user is authorized - admin scope is allowed and admin scope set for user")
		return true
	}

	log.WithFields(f).Debugf("checking project scope for projectSFID: %s...", projectSFID)
	val := user.IsUserAuthorized(auth.Project, projectSFID, true)
	if val {
		log.WithFields(f).Debugf("user '%s' is authorized for projectSFID: %s tree, admin flag=%t",
			user.UserName, projectSFID, user.Admin)
	} else {
		var scopeInfo string
		for i, scope := range user.Scopes {
			scopeInfo = fmt.Sprintf("%sscope[%d] = {type=%s, id=%s, level=%s, role=%s, related=[%s]} ",
				scopeInfo, i, scope.Type, scope.ID, scope.Level, scope.Role, strings.Join(scope.Related, ","))
		}
		log.WithFields(f).Debugf("user '%s' is not authorized for projectSFID: %s tree, admin flag=%t, scopeInfo: %s",
			user.UserName, projectSFID, user.Admin, scopeInfo)
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

	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		log.WithFields(f).Debug("skipping permissions check")
		return true
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("user is authorized - admin scope is allowed and admin scope set for user")
		return true
	}

	log.WithFields(f).Debugf("checking project scope for projectSFID: %s...", projectSFID)
	val := user.IsUserAuthorizedForProjectScope(projectSFID)
	if val {
		log.WithFields(f).Debugf("user '%s' is authorized for projectSFID: %s, admin flag=%t",
			user.UserName, projectSFID, user.Admin)
	} else {
		var scopeInfo string
		for i, scope := range user.Scopes {
			scopeInfo = fmt.Sprintf("%sscope[%d] = {type=%s, id=%s, level=%s, role=%s, related=[%s]} ",
				scopeInfo, i, scope.Type, scope.ID, scope.Level, scope.Role, strings.Join(scope.Related, ","))
		}
		log.WithFields(f).Debugf("user '%s' is not authorized for projectSFID: %s, admin flag=%t, scopeInfo: %s",
			user.UserName, projectSFID, user.Admin, scopeInfo)
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

	var scopeInfo string
	for i, scope := range user.Scopes {
		scopeInfo = fmt.Sprintf("%sscope[%d] = {type=%s, id=%s, level=%s, role=%s, related=[%s]} ",
			scopeInfo, i, scope.Type, scope.ID, scope.Level, scope.Role, strings.Join(scope.Related, ","))
	}
	log.WithFields(f).Debugf("user '%s' is not authorized for project scope checks for any projects: %s, admin flag=%t, scopeInfo: %s",
		user.UserName, strings.Join(projectSFIDs, ","), user.Admin, scopeInfo)
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
	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		log.WithFields(f).Debug("skipping permissions check")
		return true
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("user is authorized - admin scope is allowed and admin scope set for user")
		return true
	}

	val := user.IsUserAuthorizedByProject(projectSFID, companySFID)
	if val {
		log.WithFields(f).Debugf("user '%s' is authorized for projectSFID: %s + companySFID: %s tree, admin flag=%t",
			user.UserName, projectSFID, companySFID, user.Admin)
	} else {
		var scopeInfo string
		for i, scope := range user.Scopes {
			scopeInfo = fmt.Sprintf("%sscope[%d] = {type=%s, id=%s, level=%s, role=%s, related=[%s]} ",
				scopeInfo, i, scope.Type, scope.ID, scope.Level, scope.Role, strings.Join(scope.Related, ","))
		}
		log.WithFields(f).Debugf("user '%s' is not authorized for projectSFID: %s + companySFID: %s tree, admin flag=%t, scopeInfo: %s",
			user.UserName, projectSFID, companySFID, user.Admin, scopeInfo)
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

	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		log.WithFields(f).Debug("skipping permissions check")
		return true
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

	var scopeInfo string
	for i, scope := range user.Scopes {
		scopeInfo = fmt.Sprintf("%sscope[%d] = {type=%s, id=%s, level=%s, role=%s, related=[%s]} ",
			scopeInfo, i, scope.Type, scope.ID, scope.Level, scope.Role, strings.Join(scope.Related, ","))
	}
	log.WithFields(f).Debugf("user '%s' is not authorized for any projectSFID: %s + companySFID: %s, admin flag=%t, scopeInfo: %s",
		user.UserName, strings.Join(projectSFIDs, ","), companySFID, user.Admin, scopeInfo)
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

	// If we are running locally and want to disable permission checks
	if skipPermissionChecks() {
		log.WithFields(f).Debug("skipping permissions check")
		return true
	}

	if adminScopeAllowed && user.Admin {
		log.WithFields(f).Debug("user is authorized - admin scope is allowed and admin scope set for user")
		return true
	}

	val := user.IsUserAuthorized(auth.ProjectOrganization, projectSFID+"|"+companySFID, true)
	if val {
		log.WithFields(f).Debugf("user '%s' is authorized for projectSFID: %s + companySFID: %s tree, admin flag=%t",
			user.UserName, projectSFID, companySFID, user.Admin)
	} else {
		var scopeInfo string
		for i, scope := range user.Scopes {
			scopeInfo = fmt.Sprintf("%sscope[%d] = {type=%s, id=%s, level=%s, role=%s, related=[%s]} ",
				scopeInfo, i, scope.Type, scope.ID, scope.Level, scope.Role, strings.Join(scope.Related, ","))
		}
		log.WithFields(f).Debugf("user '%s' is not authorized for projectSFID: %s + companySFID: %s tree, admin flag=%t, scopeInfo: %s",
			user.UserName, projectSFID, companySFID, user.Admin, scopeInfo)
	}

	return val
}
