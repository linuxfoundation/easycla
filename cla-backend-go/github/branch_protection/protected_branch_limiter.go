// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package branch_protection

import (
	"context"
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	githubpkg "github.com/google/go-github/v37/github"
	"github.com/shurcooL/githubv4"
	"go.uber.org/ratelimit"
	"golang.org/x/time/rate"
)

// rate limiting variables
var (
	// blockingRateLimit is useful for background tasks where the interaction is more predictable
	blockingRateLimit = ratelimit.New(2)
	// nonBlockingRateLimit is preferred when the github methods would be called realtime
	// in this case we can call Allow method to check if can proceed or return error
	nonBlockingRateLimit = rate.NewLimiter(2, 5)
)

type blockingRateLimitRepositories struct {
	CombinedRepository
}

// NewBlockLimiterRepositories returns a new instance of V3Repositories interface with blocking rate limiting
// where when the limit is reached the next call blocks till the bucket is ready again
func NewBlockLimiterRepositories(repo CombinedRepository) CombinedRepository {
	return blockingRateLimitRepositories{
		CombinedRepository: repo,
	}
}

func (b blockingRateLimitRepositories) ListByOrg(ctx context.Context, org string, opt *githubpkg.RepositoryListByOrgOptions) ([]*githubpkg.Repository, *githubpkg.Response, error) {
	blockingRateLimit.Take()
	return b.CombinedRepository.ListByOrg(ctx, org, opt)
}

func (b blockingRateLimitRepositories) Get(ctx context.Context, owner, repo string) (*githubpkg.Repository, *githubpkg.Response, error) {
	blockingRateLimit.Take()
	return b.CombinedRepository.Get(ctx, owner, repo)
}

func (b blockingRateLimitRepositories) GetRepositoryBranchProtections(ctx context.Context, repositoryOwner, repositoryName string) (*RepoBranchProtectionQueryResult, error) {
	blockingRateLimit.Take()
	return b.CombinedRepository.GetRepositoryBranchProtections(ctx, repositoryOwner, repositoryName)
}
func (b blockingRateLimitRepositories) CreateBranchProtection(ctx context.Context, input *githubv4.CreateBranchProtectionRuleInput) (*CreateRepoBranchProtectionMutation, error) {
	blockingRateLimit.Take()
	return b.CombinedRepository.CreateBranchProtection(ctx, input)
}
func (b blockingRateLimitRepositories) UpdateBranchProtection(ctx context.Context, input *githubv4.UpdateBranchProtectionRuleInput) (*UpdateRepoBranchProtectionMutation, error) {
	blockingRateLimit.Take()
	return b.CombinedRepository.UpdateBranchProtection(ctx, input)
}
func (b blockingRateLimitRepositories) GetRepositoryIDFromName(ctx context.Context, repositoryOwner, repositoryName string) (string, error) {
	blockingRateLimit.Take()
	return b.CombinedRepository.GetRepositoryIDFromName(ctx, repositoryOwner, repositoryName)
}

type nonBlockingRateLimitRepositories struct {
	CombinedRepository
}

// NewNonBlockLimiterRepositories returns a new instance of V3Repositories interface with non blocking rate limiting
func NewNonBlockLimiterRepositories(repo CombinedRepository) CombinedRepository {
	return nonBlockingRateLimitRepositories{CombinedRepository: repo}
}

func (nb nonBlockingRateLimitRepositories) ListByOrg(ctx context.Context, org string, opt *githubpkg.RepositoryListByOrgOptions) ([]*githubpkg.Repository, *githubpkg.Response, error) {
	if nonBlockingRateLimit.Allow() {
		return nb.CombinedRepository.ListByOrg(ctx, org, opt)
	}
	return nil, nil, fmt.Errorf("too many requests : %w", github.ErrRateLimited)
}

func (nb nonBlockingRateLimitRepositories) Get(ctx context.Context, owner, repo string) (*githubpkg.Repository, *githubpkg.Response, error) {
	if nonBlockingRateLimit.Allow() {
		return nb.CombinedRepository.Get(ctx, owner, repo)
	}
	return nil, nil, fmt.Errorf("too many requests : %w", github.ErrRateLimited)
}

func (nb nonBlockingRateLimitRepositories) GetRepositoryBranchProtections(ctx context.Context, repositoryOwner, repositoryName string) (*RepoBranchProtectionQueryResult, error) {
	if nonBlockingRateLimit.Allow() {
		return nb.CombinedRepository.GetRepositoryBranchProtections(ctx, repositoryOwner, repositoryName)
	}
	return nil, fmt.Errorf("too many requests : %w", github.ErrRateLimited)
}

func (nb nonBlockingRateLimitRepositories) CreateBranchProtection(ctx context.Context, input *githubv4.CreateBranchProtectionRuleInput) (*CreateRepoBranchProtectionMutation, error) {
	if nonBlockingRateLimit.Allow() {
		return nb.CombinedRepository.CreateBranchProtection(ctx, input)
	}
	return nil, fmt.Errorf("too many requests : %w", github.ErrRateLimited)
}

func (nb nonBlockingRateLimitRepositories) UpdateBranchProtection(ctx context.Context, input *githubv4.UpdateBranchProtectionRuleInput) (*UpdateRepoBranchProtectionMutation, error) {
	if nonBlockingRateLimit.Allow() {
		return nb.CombinedRepository.UpdateBranchProtection(ctx, input)
	}
	return nil, fmt.Errorf("too many requests : %w", github.ErrRateLimited)
}

func (nb nonBlockingRateLimitRepositories) GetRepositoryIDFromName(ctx context.Context, repositoryOwner, repositoryName string) (string, error) {
	if nonBlockingRateLimit.Allow() {
		return nb.CombinedRepository.GetRepositoryIDFromName(ctx, repositoryOwner, repositoryName)
	}
	return "", fmt.Errorf("too many requests : %w", github.ErrRateLimited)
}
