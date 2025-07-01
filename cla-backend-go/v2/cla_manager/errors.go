// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/cla_manager"
)

// buildErrorMessageCreate helper function to build an error message
func buildErrorMessageCreate(params cla_manager.CreateCLAManagerParams, err error) string {
	return fmt.Sprintf("problem creating new CLA Manager using company ID: %s, project SFID: %s, firstName: %s, lastName: %s, user email: %s, error: %+v",
		params.CompanyID, params.ProjectSFID, *params.Body.FirstName, *params.Body.LastName, *params.Body.UserEmail, err)
}

// buildErrorMessage helper function to build an error message
func buildErrorMessageDelete(params cla_manager.DeleteCLAManagerParams, err error) string {
	return fmt.Sprintf("problem deleting new CLA Manager Request using company ID: %s, project SFID: %s, user ID: %s, error: %+v",
		params.CompanyID, params.ProjectSFID, params.UserLFID, err)
}

// buildErrorStatusCode helper function to build an error statusCodes
func buildErrorStatusCode(err error) string {
	if err == ErrNoOrgAdmins || err == ErrCLACompanyNotFound || err == ErrClaGroupNotFound || err == ErrCLAUserNotFound {
		return NotFound
	}
	// Check if user is already assigned scope/role
	if err == ErrRoleScopeConflict {
		return Conflict
	}
	// Check if user does exists
	if err == ErrNoLFID {
		return Accepted
	}
	// Return Bad Request
	return BadRequest
}
