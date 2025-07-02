// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package branch_protection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bmizerany/assert"
	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	"github.com/shurcooL/githubv4"
)

func V4StringSlice(val ...string) *[]githubv4.String {
	var v []githubv4.String
	for _, c := range val {
		v = append(v, githubv4.String(c))
	}

	return &v
}

// TestMergeStatusChecks tests the functionality of where we enable/disable checks
func TestMergeStatusChecks(t *testing.T) {

	testCases := []struct {
		Name            string
		currentChecks   []string
		expectedChecks  []string
		enableContexts  []string
		disableContexts []string
	}{
		{
			Name:           "all empty",
			expectedChecks: []string{},
		},
		{
			Name:           "empty state enable",
			expectedChecks: []string{"EasyCLA"},
			enableContexts: []string{"EasyCLA"},
		},
		{
			Name:           "preserve existing enable more",
			currentChecks:  []string{"travis-ci"},
			expectedChecks: []string{"travis-ci", "EasyCLA"},
			enableContexts: []string{"EasyCLA"},
		},
		{
			Name:            "preserve existing disable some",
			currentChecks:   []string{"travis-ci", "EasyCLA"},
			expectedChecks:  []string{"travis-ci"},
			disableContexts: []string{"EasyCLA"},
		},
		{
			Name:            "add and remove in same operation",
			currentChecks:   []string{"travis-ci", "DCO", "EasyCLA"},
			expectedChecks:  []string{"travis-ci", "EasyCLA", "CodeQL"},
			enableContexts:  []string{"CodeQL"},
			disableContexts: []string{"DCO"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(tt *testing.T) {
			result := mergeStatusChecks(tc.currentChecks, tc.enableContexts, tc.disableContexts)
			assert.Equal(tt, tc.expectedChecks, result)
		})
	}
}

func TestEnableBranchProtection(t *testing.T) {
	owner := "johnenable"
	repo := "johnsrepoenable"
	branchName := DefaultBranchName

	testCases := []struct {
		Name                    string
		Checks                  []string
		CurrentProtections      *RepoBranchProtectionQueryResult
		CreateProtectionRequest *githubv4.CreateBranchProtectionRuleInput
		UpdateProtectionRequest *githubv4.UpdateBranchProtectionRuleInput
		Err                     error
	}{
		{
			Name:   "success",
			Checks: []string{"easyCLA"},
			CurrentProtections: &RepoBranchProtectionQueryResult{
				RepositoryOwner: struct {
					Repository BranchProtectionRuleRepositoryParam `graphql:"repository(name: $name)"`
				}{Repository: BranchProtectionRuleRepositoryParam{
					Name:                  "repoNameValue",
					ID:                    "repoIDValue",
					BranchProtectionRules: BranchProtectionRuleQueryParam{},
				}},
			},
			CreateProtectionRequest: &githubv4.CreateBranchProtectionRuleInput{
				RepositoryID:                githubv4.ID("repoIDValue"),
				Pattern:                     githubv4.String(branchName),
				AllowsForcePushes:           githubv4.NewBoolean(false),
				AllowsDeletions:             githubv4.NewBoolean(false),
				IsAdminEnforced:             githubv4.NewBoolean(true),
				RequiresStatusChecks:        githubv4.NewBoolean(true),
				RequiredStatusCheckContexts: V4StringSlice("easyCLA"),
			},
		},
		{
			Name:   "preserve existing checks",
			Checks: []string{"easyCLA"},
			CurrentProtections: &RepoBranchProtectionQueryResult{
				RepositoryOwner: struct {
					Repository BranchProtectionRuleRepositoryParam `graphql:"repository(name: $name)"`
				}{Repository: BranchProtectionRuleRepositoryParam{
					Name: "repoNameValue",
					ID:   "repoIDValue",
					BranchProtectionRules: BranchProtectionRuleQueryParam{
						TotalCount: 1,
						Nodes: []BranchProtectionRule{
							{
								ID:      "branchProtectionID",
								Pattern: branchName,
								RequiredStatusCheckContexts: []string{
									"circle/ci",
								},
							},
						},
					},
				}},
			},
			UpdateProtectionRequest: &githubv4.UpdateBranchProtectionRuleInput{
				BranchProtectionRuleID:      githubv4.ID("branchProtectionID"),
				Pattern:                     githubv4.NewString(githubv4.String(branchName)),
				AllowsForcePushes:           githubv4.NewBoolean(false),
				AllowsDeletions:             githubv4.NewBoolean(false),
				IsAdminEnforced:             githubv4.NewBoolean(true),
				RequiresStatusChecks:        githubv4.NewBoolean(true),
				RequiredStatusCheckContexts: V4StringSlice("circle/ci", "easyCLA"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(tt *testing.T) {
			ctrl := gomock.NewController(tt)
			defer ctrl.Finish()

			m := NewMockCombinedRepository(ctrl)
			m.
				EXPECT().
				GetRepositoryBranchProtections(gomock.Any(), owner, repo).
				Return(tc.CurrentProtections, nil)

			if tc.CreateProtectionRequest != nil {
				m.
					EXPECT().
					CreateBranchProtection(gomock.Any(), tc.CreateProtectionRequest).
					Return(nil, nil)
			}

			if tc.UpdateProtectionRequest != nil {
				m.
					EXPECT().
					UpdateBranchProtection(gomock.Any(), tc.UpdateProtectionRequest).
					Return(nil, nil)
			}

			branchProtectionRepo := newBranchProtectionRepository(m)
			err := branchProtectionRepo.EnableBranchProtection(context.Background(), owner, repo, branchName, true, tc.Checks, nil)
			if err != nil {
				tt.Errorf("enable branch proteciton failed : %v", err)
			}
		})
	}
}

func TestNonBlockingRateLimitRepositories_GetBranchProtection(t *testing.T) {
	owner := "johnblocking"
	repo := "johnsrepoblocking"
	branchName := DefaultBranchName

	t.Run("no limit reached", func(tt *testing.T) {
		ctrl := gomock.NewController(tt)
		defer ctrl.Finish()

		protection := &RepoBranchProtectionQueryResult{
			RepositoryOwner: struct {
				Repository BranchProtectionRuleRepositoryParam `graphql:"repository(name: $name)"`
			}{Repository: BranchProtectionRuleRepositoryParam{
				Name: "repoNameValue",
				ID:   "repoIDValue",
				BranchProtectionRules: BranchProtectionRuleQueryParam{
					TotalCount: 1,
					Nodes: []BranchProtectionRule{
						{
							ID:      "branchProtectionID",
							Pattern: branchName,
							RequiredStatusCheckContexts: []string{
								"circle/ci",
							},
						},
					},
				},
			}},
		}

		m := NewMockCombinedRepository(ctrl)
		m.
			EXPECT().
			GetRepositoryBranchProtections(gomock.Any(), owner, repo).
			Return(protection, nil)

		nonBlockLimitRepo := newBranchProtectionRepository(m, EnableNonBlockingLimiter())
		p, err := nonBlockLimitRepo.GetProtectedBranch(context.Background(), owner, repo, branchName)
		if err != nil {
			tt.Errorf("no error expected : %v", err)
		}
		assert.Equal(tt, protection.RepositoryOwner.Repository.BranchProtectionRules.Nodes[0], *p)
	})

	t.Run("limit reached", func(tt *testing.T) {
		ctrl := gomock.NewController(tt)
		defer ctrl.Finish()

		protection := &RepoBranchProtectionQueryResult{
			RepositoryOwner: struct {
				Repository BranchProtectionRuleRepositoryParam `graphql:"repository(name: $name)"`
			}{Repository: BranchProtectionRuleRepositoryParam{
				Name: "repoNameValue",
				ID:   "repoIDValue",
				BranchProtectionRules: BranchProtectionRuleQueryParam{
					TotalCount: 1,
					Nodes: []BranchProtectionRule{
						{
							ID:      "branchProtectionID",
							Pattern: branchName,
							RequiredStatusCheckContexts: []string{
								"circle/ci",
							},
						},
					},
				},
			}},
		}

		m := NewMockCombinedRepository(ctrl)
		m.
			EXPECT().
			GetRepositoryBranchProtections(gomock.Any(), owner, repo).
			Return(protection, nil).AnyTimes()

		nonBlockLimitRepo := newBranchProtectionRepository(m, EnableNonBlockingLimiter())
		// call it 100 times in loop to make it fail
		var expectedErr error
		for i := 0; i < 100; i++ {
			_, err := nonBlockLimitRepo.GetProtectedBranch(context.Background(), owner, repo, branchName)
			if err != nil {
				expectedErr = err
				break
			}
		}

		if expectedErr == nil {
			tt.Fatalf("no error returned")
			return
		}

		if !errors.Is(expectedErr, github.ErrRateLimited) {
			tt.Fatalf("was expecting ErrRateLimited got : %v", expectedErr)
			return
		}
	})
}

func TestBlockingRateLimitRepositories_GetBranchProtection(t *testing.T) {
	owner := "john"
	repo := "johnsrepo"
	branchName := DefaultBranchName

	t.Run("no limit reached", func(tt *testing.T) {
		ctrl := gomock.NewController(tt)
		defer ctrl.Finish()

		protection := &RepoBranchProtectionQueryResult{
			RepositoryOwner: struct {
				Repository BranchProtectionRuleRepositoryParam `graphql:"repository(name: $name)"`
			}{Repository: BranchProtectionRuleRepositoryParam{
				Name: "repoNameValue",
				ID:   "repoIDValue",
				BranchProtectionRules: BranchProtectionRuleQueryParam{
					TotalCount: 1,
					Nodes: []BranchProtectionRule{
						{
							ID:      "branchProtectionID",
							Pattern: branchName,
							RequiredStatusCheckContexts: []string{
								"circle/ci",
							},
						},
					},
				},
			}},
		}

		m := NewMockCombinedRepository(ctrl)
		m.
			EXPECT().
			GetRepositoryBranchProtections(gomock.Any(), owner, repo).
			Return(protection, nil)

		blockLimitRepo := newBranchProtectionRepository(m, EnableBlockingLimiter())
		p, err := blockLimitRepo.GetProtectedBranch(context.Background(), owner, repo, branchName)
		if err != nil {
			tt.Errorf("no error expected : %v", err)
		}
		assert.Equal(tt, protection.RepositoryOwner.Repository.BranchProtectionRules.Nodes[0], *p)
	})

	t.Run("limit reached", func(tt *testing.T) {
		ctrl := gomock.NewController(tt)
		defer ctrl.Finish()

		protection := &RepoBranchProtectionQueryResult{
			RepositoryOwner: struct {
				Repository BranchProtectionRuleRepositoryParam `graphql:"repository(name: $name)"`
			}{Repository: BranchProtectionRuleRepositoryParam{
				Name: "repoNameValue",
				ID:   "repoIDValue",
				BranchProtectionRules: BranchProtectionRuleQueryParam{
					TotalCount: 1,
					Nodes: []BranchProtectionRule{
						{
							ID:      "branchProtectionID",
							Pattern: branchName,
							RequiredStatusCheckContexts: []string{
								"circle/ci",
							},
						},
					},
				},
			}},
		}

		m := NewMockCombinedRepository(ctrl)
		m.
			EXPECT().
			GetRepositoryBranchProtections(gomock.Any(), owner, repo).
			Return(protection, nil).AnyTimes()

		blockLimitRepo := newBranchProtectionRepository(m, EnableBlockingLimiter())

		//	call it 100 times in loop to make it fail
		var expectedErr error
		start := time.Now()
		for i := 0; i < 10; i++ {
			_, err := blockLimitRepo.GetProtectedBranch(context.Background(), owner, repo, branchName)
			if err != nil {
				expectedErr = err
				break
			}
		}
		elapsed := time.Since(start)

		if expectedErr != nil {
			tt.Fatalf("no error was expected got : %v", expectedErr)
			return
		}

		if elapsed < 4*time.Second {
			tt.Fatalf("is rate limit enabled")
		}
	})
}
