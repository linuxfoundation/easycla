// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package branch_protection

import (
	"context"
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	"github.com/shurcooL/githubv4"
)

// BranchProtectionRule is the data structure that's used to reflect the remote github branch protection rule
type BranchProtectionRule struct {
	ID                          string
	Pattern                     string
	RequiredStatusCheckContexts []string
	RequiresStatusChecks        bool
	IsAdminEnforced             bool
	AllowsDeletions             bool
	AllowsForcePushes           bool
}

// BranchProtectionRuleQueryParam is part of RepoBranchProtectionQueryResult extracted here so can
// easily be initialized
type BranchProtectionRuleQueryParam struct {
	TotalCount int
	Nodes      []BranchProtectionRule
}

// BranchProtectionRuleRepositoryParam is part of RepoBranchProtectionQueryResult extracted here so can
// easily be initialized
type BranchProtectionRuleRepositoryParam struct {
	Name                  string
	ID                    string
	BranchProtectionRules BranchProtectionRuleQueryParam `graphql:"branchProtectionRules(first:10)"`
}

// RepoBranchProtectionQueryResult is the query which queries for given owner and repository name
type RepoBranchProtectionQueryResult struct {
	RepositoryOwner struct {
		Repository BranchProtectionRuleRepositoryParam `graphql:"repository(name: $name)"`
	} `graphql:"repositoryOwner(login: $login)"`
}

// CreateRepoBranchProtectionMutation adds a new branch protection rule
type CreateRepoBranchProtectionMutation struct {
	CreateBranchProtectionRule struct {
		BranchProtectionRule struct {
			Repository struct {
				Name string
			}
			Pattern              string
			IsAdminEnforced      bool
			RequiresStatusChecks bool
			AllowsDeletions      bool
			AllowsForcePushes    bool
		}
	} `graphql:"createBranchProtectionRule(input: $input)"`
}

// UpdateRepoBranchProtectionMutation updates existing branch protection rule
type UpdateRepoBranchProtectionMutation struct {
	UpdateBranchProtectionRule struct {
		BranchProtectionRule struct {
			Repository struct {
				Name string
			}
			Pattern              string
			IsAdminEnforced      bool
			RequiresStatusChecks bool
			AllowsDeletions      bool
			AllowsForcePushes    bool
		}
	} `graphql:"updateBranchProtectionRule(input: $input)"`
}

// BranchProtectionRepositoryV4 wraps a v4 github client
type BranchProtectionRepositoryV4 struct {
	client *githubv4.Client
}

// NewBranchProtectionRepositoryV4 creates a new BranchProtectionRepositoryV4
func NewBranchProtectionRepositoryV4(installationID int64) (*BranchProtectionRepositoryV4, error) {
	client, clientErr := github.NewGithubV4AppClient(installationID)
	if clientErr != nil {
		return nil, clientErr
	}
	return &BranchProtectionRepositoryV4{
		client: client,
	}, nil
}

// GetRepositoryBranchProtections returns the repository branch protections for the specified repository
func (r *BranchProtectionRepositoryV4) GetRepositoryBranchProtections(ctx context.Context, repositoryOwner, repositoryName string) (*RepoBranchProtectionQueryResult, error) {
	var queryResult RepoBranchProtectionQueryResult

	variables := map[string]interface{}{
		"login": githubv4.String(repositoryOwner),
		"name":  githubv4.String(repositoryName),
	}

	err := r.client.Query(ctx, &queryResult, variables)
	if err != nil {
		return nil, fmt.Errorf("fetching branch protection rules for owner : %s and repo : %s failed : %v", repositoryOwner, repositoryName, err)
	}

	return &queryResult, nil
}

// CreateBranchProtection creates the repository branch protections for the specified repository
func (r *BranchProtectionRepositoryV4) CreateBranchProtection(ctx context.Context, input *githubv4.CreateBranchProtectionRuleInput) (*CreateRepoBranchProtectionMutation, error) {
	var createMutationResult CreateRepoBranchProtectionMutation
	err := r.client.Mutate(ctx, &createMutationResult, *input, nil)
	if err != nil {
		return nil, fmt.Errorf("creating new branch protection failed : %w", err)
	}
	return &createMutationResult, nil
}

// UpdateBranchProtection updates the repository branch protections for the specified repository
func (r *BranchProtectionRepositoryV4) UpdateBranchProtection(ctx context.Context, input *githubv4.UpdateBranchProtectionRuleInput) (*UpdateRepoBranchProtectionMutation, error) {
	var updateMutationResult UpdateRepoBranchProtectionMutation
	err := r.client.Mutate(ctx, &updateMutationResult, *input, nil)
	if err != nil {
		return nil, fmt.Errorf("updating current branch rule failed : %w", err)
	}
	return &updateMutationResult, nil
}

// GetRepositoryIDFromName when provided the organization and repository name, returns the repository ID
func (r *BranchProtectionRepositoryV4) GetRepositoryIDFromName(ctx context.Context, repositoryOwner, repositoryName string) (string, error) {

	// Define the graphql query
	//"query": "query{repository(name: \"test1\", owner: \"deal-test-org\") {id}}"
	var query struct {
		Viewer struct {
			Login githubv4.String
		}
		Repository struct {
			ID string
		} `graphql:"repository(owner:$repositoryOwner, name:$repositoryName)"`
	}

	// Define the variables for the query
	variables := map[string]interface{}{
		"repositoryOwner": githubv4.String(repositoryOwner),
		"repositoryName":  githubv4.String(repositoryName),
	}

	err := r.client.Query(ctx, &query, variables)
	if err != nil {
		return "", err
	}

	return query.Repository.ID, nil
}

// EnableBranchProtectionForPattern enables branch protection for the given branch protection input
func EnableBranchProtectionForPattern(ctx context.Context, repositoryOwner, repositoryName string, input *BranchProtectionRule) error {
	return nil
}
