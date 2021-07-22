// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import "github.com/aws/aws-sdk-go/service/dynamodb/expression"

// buildProject is a helper function to build a common set of projection/columns for the query
func buildProjection() expression.ProjectionBuilder {
	// These are the columns we want returned
	return expression.NamesList(
		expression.Name("signature_id"),
		expression.Name("date_created"),
		expression.Name("date_modified"),
		expression.Name("signature_acl"),
		expression.Name("signature_approved"),
		expression.Name("signature_document_major_version"),
		expression.Name("signature_document_minor_version"),
		expression.Name("signature_reference_id"),
		expression.Name("signature_reference_name"),       // Added to support simplified UX queries
		expression.Name("signature_reference_name_lower"), // Added to support case insensitive UX queries
		expression.Name("signature_project_id"),
		expression.Name("signature_reference_type"),       // user or company
		expression.Name("signature_signed"),               // T/F
		expression.Name("signature_type"),                 // ccla or cla
		expression.Name("signature_user_ccla_company_id"), // reference to the company
		expression.Name("email_whitelist"),
		expression.Name("domain_whitelist"),
		expression.Name("github_whitelist"),
		expression.Name("github_org_whitelist"),
		expression.Name("gitlab_username_approval_list"), // added for GitLab support
		expression.Name("gitlab_project_approval_list"),  // added for GitLab support
		expression.Name("user_github_username"),
		expression.Name("user_lf_username"),
		expression.Name("user_name"),
		expression.Name("user_email"),
		expression.Name("signed_on"),
		expression.Name("signatory_name"),
		expression.Name("user_docusign_date_signed"),
		expression.Name("user_docusign_name"),
	)
}

// buildSignatureACLProject is a helper function to build a signature ACL response/projection
func buildSignatureACLProjection() expression.ProjectionBuilder {
	// These are the columns we want returned
	return expression.NamesList(
		expression.Name("signature_id"),
		expression.Name("signature_acl"),
	)
}

// buildCompanyIDProjection is a helper function to build a simple projection with the signature id and the company id
func buildCompanyIDProjection() expression.ProjectionBuilder {
	// These are the columns we want returned
	return expression.NamesList(
		expression.Name("signature_id"),
		expression.Name("signature_reference_id"),
	)
}
