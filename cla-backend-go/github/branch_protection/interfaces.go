// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package branch_protection

import (
	"context"

	"github.com/google/go-github/v37/github"
	"github.com/shurcooL/githubv4"
)

// V3Repositories is part of the interface working with github repositories, it's inside of the github client
// It's extracted here as interface so we can mock that functionality in the tests.
type V3Repositories interface {
	ListByOrg(ctx context.Context, org string, opt *github.RepositoryListByOrgOptions) ([]*github.Repository, *github.Response, error)
	Get(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error)
}

// V4BranchProtectionRepository has v4 (graphQL) branch protection functionality
type V4BranchProtectionRepository interface {
	GetRepositoryBranchProtections(ctx context.Context, repositoryOwner, repositoryName string) (*RepoBranchProtectionQueryResult, error)
	CreateBranchProtection(ctx context.Context, input *githubv4.CreateBranchProtectionRuleInput) (*CreateRepoBranchProtectionMutation, error)
	UpdateBranchProtection(ctx context.Context, input *githubv4.UpdateBranchProtectionRuleInput) (*UpdateRepoBranchProtectionMutation, error)
	GetRepositoryIDFromName(ctx context.Context, repositoryOwner, repositoryName string) (string, error)
}

//CombinedRepository is combination of V3Repositories and V4BranchProtectionRepository
type CombinedRepository interface {
	V3Repositories
	V4BranchProtectionRepository
}
