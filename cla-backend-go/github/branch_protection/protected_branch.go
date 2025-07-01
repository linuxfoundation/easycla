// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package branch_protection

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	"github.com/shurcooL/githubv4"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	githubpkg "github.com/google/go-github/v37/github"
)

const (
	// DefaultBranchName is the default branch we'll be working with if not specified
	DefaultBranchName = "main"
)

var (
	// ErrBranchNotProtected indicates the situation where the branch is not enabled for protection on github side
	ErrBranchNotProtected = errors.New("not protected")
)

type combinedRepositoryProvider struct {
	V3Repositories
	V4BranchProtectionRepository
}

type branchProtectionRepositoryConfig struct {
	enableBlockingLimiter    bool
	enableNonBlockingLimiter bool
}

// BranchProtectionRepositoryOption enables optional parameters to BranchProtectionRepository
type BranchProtectionRepositoryOption func(config *branchProtectionRepositoryConfig)

// EnableBlockingLimiter enables the blocking limiter
func EnableBlockingLimiter() BranchProtectionRepositoryOption {
	return func(config *branchProtectionRepositoryConfig) {
		config.enableBlockingLimiter = true
	}
}

// EnableNonBlockingLimiter enables the non-blocking limiter
func EnableNonBlockingLimiter() BranchProtectionRepositoryOption {
	return func(config *branchProtectionRepositoryConfig) {
		config.enableNonBlockingLimiter = true
	}
}

// BranchProtectionRepository contains helper methods interacting with github api related to branch protection
type BranchProtectionRepository struct {
	combinedRepo CombinedRepository
}

// NewBranchProtectionRepository creates a new BranchProtectionRepository
func NewBranchProtectionRepository(installationID int64, opts ...BranchProtectionRepositoryOption) (*BranchProtectionRepository, error) {
	v4BranchProtectionRepo, err := NewBranchProtectionRepositoryV4(installationID)
	if err != nil {
		return nil, fmt.Errorf("initializing v4 github client failed : %v", err)
	}

	v3Client, err := github.NewGithubAppClient(installationID)
	if err != nil {
		return nil, fmt.Errorf("initializing v3 github client failed : %v", err)
	}

	combinedRepo := combinedRepositoryProvider{
		V3Repositories:               v3Client.Repositories,
		V4BranchProtectionRepository: v4BranchProtectionRepo,
	}

	return newBranchProtectionRepository(combinedRepo, opts...), nil
}

func newBranchProtectionRepository(combinedRepo CombinedRepository, opts ...BranchProtectionRepositoryOption) *BranchProtectionRepository {
	config := &branchProtectionRepositoryConfig{}
	for _, o := range opts {
		o(config)
	}

	if config.enableNonBlockingLimiter {
		combinedRepo = NewNonBlockLimiterRepositories(combinedRepo)
	} else if config.enableBlockingLimiter {
		combinedRepo = NewBlockLimiterRepositories(combinedRepo)
	}

	return &BranchProtectionRepository{
		combinedRepo: combinedRepo,
	}
}

// GetOwnerName retrieves the owner name of the given org and repo name
func (bp *BranchProtectionRepository) GetOwnerName(ctx context.Context, orgName, repoName string) (string, error) {
	repoName = CleanGithubRepoName(repoName)
	log.Debugf("GetOwnerName : getting owner name for org %s and repoName : %s", orgName, repoName)
	listOpt := &githubpkg.RepositoryListByOrgOptions{
		ListOptions: githubpkg.ListOptions{
			PerPage: 30,
		},
	}
	for {
		repos, resp, err := bp.combinedRepo.ListByOrg(ctx, orgName, listOpt)
		if err != nil {
			if ok, wErr := github.CheckAndWrapForKnownErrors(resp, err); ok {
				return "", wErr
			}
			return "", err
		}

		if len(repos) == 0 {
			log.Warnf("GetOwnerName : no repos found under orgName : %s (maybe no access ?)", orgName)
			return "", nil
		}

		for _, repo := range repos {
			if *repo.Name == repoName {
				if repo.Owner != nil {
					owner := *repo.Owner
					return *owner.Login, nil
				}
			}
		}

		// means we're at the end of it so exit
		if resp.NextPage == 0 {
			log.Warnf("GetOwnerName : owner name not found for org : %s and repo : %s", orgName, repoName)
			return "", nil
		}

		// set it to the next page
		listOpt.Page = resp.NextPage
	}
}

// GetDefaultBranchForRepo helps with pulling the default branch for the given repo
func (bp *BranchProtectionRepository) GetDefaultBranchForRepo(ctx context.Context, owner, repoName string) (string, error) {
	repoName = CleanGithubRepoName(repoName)
	repo, resp, err := bp.combinedRepo.Get(ctx, owner, repoName)
	if err != nil {
		if ok, wErr := github.CheckAndWrapForKnownErrors(resp, err); ok {
			return "", wErr
		}
		return "", err
	}

	var defaultBranch string
	if repo.DefaultBranch == nil {
		defaultBranch = DefaultBranchName
	} else {
		defaultBranch = *repo.DefaultBranch
	}

	return defaultBranch, nil
}

// GetProtectedBranch fetches the protected branch details
func (bp *BranchProtectionRepository) GetProtectedBranch(ctx context.Context, owner, repoName, protectedBranchName string) (*BranchProtectionRule, error) {
	repoName = CleanGithubRepoName(repoName)
	branchProtections, err := bp.combinedRepo.GetRepositoryBranchProtections(ctx, owner, repoName)
	if err != nil {
		return nil, fmt.Errorf("fetching repo protections for owner : %s and repoName : %s failed : %w", owner, repoName, err)
	}

	// it's not found this pattern or branch
	if branchProtections.RepositoryOwner.Repository.BranchProtectionRules.TotalCount == 0 {
		return nil, ErrBranchNotProtected
	}

	for _, protection := range branchProtections.RepositoryOwner.Repository.BranchProtectionRules.Nodes {
		if protection.Pattern == protectedBranchName {
			return &protection, nil
		}
	}

	return nil, ErrBranchNotProtected
}

// EnableBranchProtection enables branch protection if not enabled and makes sure passed arguments such as enforceAdmin
// statusChecks are applied. The operation makes sure it doesn't override the existing checks.
func (bp *BranchProtectionRepository) EnableBranchProtection(ctx context.Context, owner, repoName, branchName string, enforceAdmin bool, enableStatusChecks, disableStatusChecks []string) error {
	repoName = CleanGithubRepoName(repoName)

	// fetch the existing ones
	queryResult, err := bp.combinedRepo.GetRepositoryBranchProtections(ctx, owner, repoName)
	if err != nil {
		return err
	}

	currentProtections := queryResult.RepositoryOwner.Repository.BranchProtectionRules.Nodes
	repoID := queryResult.RepositoryOwner.Repository.ID

	createInput, updateInput := prepareBranchProtectionMutation(repoID, currentProtections, &BranchProtectionRule{
		Pattern:                     branchName,
		RequiredStatusCheckContexts: enableStatusChecks,
		RequiresStatusChecks:        true,
		IsAdminEnforced:             enforceAdmin,
		AllowsDeletions:             false,
		AllowsForcePushes:           false,
	})
	if createInput != nil {
		_, createErr := bp.combinedRepo.CreateBranchProtection(ctx, createInput)
		if createErr != nil {
			return fmt.Errorf("creating new branch protection rule for owner : %s and repo : %s failed : %v", owner, repoName, createErr)
		}
		return nil
	}

	_, err = bp.combinedRepo.UpdateBranchProtection(ctx, updateInput)
	if err != nil {
		return fmt.Errorf("updating current branch rule for owner : %s and repo name : %s, failed : %v", owner, repoName, err)
	}

	return nil
}

// mergeStatusChecks merges the current checks with the new ones and disable the ones that are specified
func mergeStatusChecks(currentChecks []string, enableContexts, disableContexts []string) []string {

	// seems github api is not happy with nils for arrays ;)
	if len(enableContexts) == 0 {
		enableContexts = []string{}
	}

	if currentChecks == nil {
		currentChecks = []string{}
	}

	finalContexts := []string{}
	uniqueEnableContexts := map[string]bool{}

	for _, c := range currentChecks {
		// first disable the ones we're not interested into
		found := false
		if len(disableContexts) > 0 {
			for _, disableContext := range disableContexts {
				if disableContext == c {
					found = true
					break
				}
			}
		}

		if found {
			continue
		}

		uniqueEnableContexts[c] = true
		finalContexts = append(finalContexts, c)
	}

	for _, c := range enableContexts {
		if uniqueEnableContexts[c] {
			continue
		}

		uniqueEnableContexts[c] = true
		finalContexts = append(finalContexts, c)
	}

	return finalContexts
}

// prepareBranchProtectionMutation creates the mutation input objects to modify the branch protection
// the logic is pulled out so we can unit test it without mocking the connections
func prepareBranchProtectionMutation(repoID string, currentProtections []BranchProtectionRule, input *BranchProtectionRule) (*githubv4.CreateBranchProtectionRuleInput, *githubv4.UpdateBranchProtectionRuleInput) {
	var foundBranchProtectionRule *BranchProtectionRule
	if len(currentProtections) > 0 {
		for _, protection := range currentProtections {
			if protection.Pattern == input.Pattern {
				currentProtection := protection
				foundBranchProtectionRule = &currentProtection
				break
			}
		}
	}

	if foundBranchProtectionRule == nil {
		var statusChecks []githubv4.String
		for _, check := range input.RequiredStatusCheckContexts {
			statusChecks = append(statusChecks, githubv4.String(check))
		}

		createInput := githubv4.CreateBranchProtectionRuleInput{
			RepositoryID:                repoID,
			Pattern:                     githubv4.String(input.Pattern),
			AllowsForcePushes:           githubv4.NewBoolean(false),
			AllowsDeletions:             githubv4.NewBoolean(false),
			IsAdminEnforced:             githubv4.NewBoolean(githubv4.Boolean(input.IsAdminEnforced)),
			RequiresStatusChecks:        githubv4.NewBoolean(true),
			RequiredStatusCheckContexts: &statusChecks,
		}

		return &createInput, nil
	}

	// it's an existing one we need to update and make sure all of them it's at state we want it
	mergedStatusChecks := mergeStatusChecks(foundBranchProtectionRule.RequiredStatusCheckContexts, input.RequiredStatusCheckContexts, nil)
	var finalStatusChecks []githubv4.String

	for _, check := range mergedStatusChecks {
		finalStatusChecks = append(finalStatusChecks, githubv4.String(check))
	}

	updateInput := githubv4.UpdateBranchProtectionRuleInput{
		BranchProtectionRuleID:      githubv4.ID(foundBranchProtectionRule.ID),
		Pattern:                     githubv4.NewString(githubv4.String(input.Pattern)),
		IsAdminEnforced:             githubv4.NewBoolean(githubv4.Boolean(input.IsAdminEnforced)),
		RequiresStatusChecks:        githubv4.NewBoolean(true),
		AllowsDeletions:             githubv4.NewBoolean(false),
		AllowsForcePushes:           githubv4.NewBoolean(false),
		RequiredStatusCheckContexts: &finalStatusChecks,
	}

	return nil, &updateInput
}

// IsEnforceAdminEnabled checks if enforce admin option is enabled for the branch protection
func IsEnforceAdminEnabled(protection *BranchProtectionRule) bool {
	return protection.IsAdminEnforced
}

// CleanGithubRepoName removes the orgname if existing in the string
func CleanGithubRepoName(githubRepoName string) string {
	if strings.Contains(githubRepoName, "/") {
		parts := strings.Split(githubRepoName, "/")
		githubRepoName = parts[len(parts)-1]
	}
	return githubRepoName
}
