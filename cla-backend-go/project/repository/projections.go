// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repository

import "github.com/aws/aws-sdk-go/service/dynamodb/expression"

// buildProject is a helper function to build a common set of projection/columns for the query
func buildProjection() expression.ProjectionBuilder {
	// These are the columns we want returned
	return expression.NamesList(
		expression.Name("project_id"),
		expression.Name("foundation_sfid"),
		expression.Name("root_project_repositories_count"),
		expression.Name("project_external_id"),
		expression.Name("project_name"),
		expression.Name("project_name_lower"),
		expression.Name("project_description"),
		expression.Name("project_acl"),
		expression.Name("project_ccla_enabled"),
		expression.Name("project_icla_enabled"),
		expression.Name("project_ccla_requires_icla_signature"),
		expression.Name("project_live"),
		expression.Name("project_corporate_documents"),
		expression.Name("project_individual_documents"),
		expression.Name("project_member_documents"),
		expression.Name("project_template_id"),
		expression.Name("date_created"),
		expression.Name("date_modified"),
		expression.Name("version"),
	)
}
