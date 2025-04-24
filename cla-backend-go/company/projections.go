// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import "github.com/aws/aws-sdk-go/service/dynamodb/expression"

// buildCompanyProjection creates a ProjectionBuilds with the columns we are interested in
func buildCompanyProjection() expression.ProjectionBuilder {
	// These are the columns we want returned
	return expression.NamesList(
		expression.Name("company_id"),
		expression.Name("company_name"),
		expression.Name("signing_entity_name"),
		expression.Name("company_acl"),
		expression.Name("company_external_id"),
		expression.Name("company_manager_id"),
		expression.Name("date_created"),
		expression.Name("date_modified"),
		expression.Name("note"),
		expression.Name("is_sanctioned"),
		expression.Name("version"),
	)
}

// buildInvitesProjection returns the list of columns for the query/scan projection
func buildInvitesProjection() expression.ProjectionBuilder {
	// These are the columns we want returned
	return expression.NamesList(
		expression.Name("company_invite_id"),
		expression.Name("requested_company_id"),
		expression.Name("user_id"),
		expression.Name("status"),
		expression.Name("date_created"),
		expression.Name("date_modified"),
		expression.Name("version"),
	)
}
