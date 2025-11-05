// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

// SignatureEmailApprovalListColumn is the name of the signature column for the email approval list
const SignatureEmailApprovalListColumn = "email_whitelist" // Note: Column name uses legacy terminology. TODO: Rename database column to email_allowlist (requires data migration)

// SignatureDomainApprovalListColumn is the name of the signature column for the domain approval list
const SignatureDomainApprovalListColumn = "domain_whitelist" // Note: Column name uses legacy terminology. TODO: Rename database column to domain_allowlist (requires data migration)

// SignatureGitHubUsernameApprovalListColumn is the name of the signature column for the GitHub username approval list
const SignatureGitHubUsernameApprovalListColumn = "github_whitelist" // Note: Column name uses legacy terminology. TODO: Rename database column to github_username_allowlist (requires data migration)

// SignatureGitHubOrgApprovalListColumn is the name of the signature column for the GitHub organization approval list
const SignatureGitHubOrgApprovalListColumn = "github_org_whitelist" // Note: Column name uses legacy terminology. TODO: Rename database column to github_org_allowlist (requires data migration)

// SignatureGitlabUsernameApprovalListColumn is the name of the signature column for gitlab username approval lists
const SignatureGitlabUsernameApprovalListColumn = "gitlab_username_approval_list"

// SignatureGitlabOrgApprovalListColumn is the name of the signature column for gitlab organization approval lists
const SignatureGitlabOrgApprovalListColumn = "gitlab_org_approval_list" // nolint G101: Potential hardcoded credentials (gosec)

// SignatureUserGitHubUsername is the name of the signature column for user gitlab username
const SignatureUserGitHubUsername = "user_github_username"

// SignatureUserGitlabUsername is the name of the signature column for user gitlab username
const SignatureUserGitlabUsername = "user_gitlab_username"