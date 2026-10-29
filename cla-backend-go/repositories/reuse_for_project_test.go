// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

const projectA = "project-a"

// the re-use write condition, placeholders resolved and spaces removed, as every scoped write must carry it
const reuseConditionForProjectA = `(attribute_exists(repository_id))AND((project_sfid="project-a")OR((((attribute_not_exists(project_sfid))OR(attribute_type(project_sfid,"NULL")))OR(size(project_sfid)=0))AND((((attribute_not_exists(repository_sfdc_id))OR(attribute_type(repository_sfdc_id,"NULL")))OR(size(repository_sfdc_id)=0))OR(repository_sfdc_id="project-a"))))`

func disabledUnboundItem() map[string]interface{} {
	return map[string]interface{}{
		"repository_id":          map[string]interface{}{"S": "repo-1"},
		"repository_external_id": map[string]interface{}{"S": "123456"},
		"repository_name":        map[string]interface{}{"S": "org/repo"},
		"enabled":                map[string]interface{}{"BOOL": false},
		"note":                   map[string]interface{}{"S": "disabled earlier"},
	}
}

// a record whose bindings were written as empty strings: the row model has no omitempty on its dynamodbav tags,
// so they are stored as NULL
func disabledNullBoundItem() map[string]interface{} {
	item := disabledUnboundItem()
	item["project_sfid"] = map[string]interface{}{"NULL": true}
	item["repository_sfdc_id"] = map[string]interface{}{"NULL": true}
	return item
}

func reuseInput() *models.GithubRepositoryInput {
	return &models.GithubRepositoryInput{
		RepositoryExternalID:       aws.String("123456"),
		RepositoryName:             aws.String("org/repo"),
		RepositoryOrganizationName: aws.String("org"),
		RepositoryProjectID:        aws.String("cla-group-1"),
		RepositoryType:             aws.String("github"),
		RepositoryURL:              aws.String("https://github.com/org/repo"),
		Enabled:                    aws.Bool(true),
		Note:                       "re-enabling",
	}
}

// resolvedCondition returns the ConditionExpression of a recorded update with its placeholders substituted
// and all spaces removed, "" when the update carries none
func resolvedCondition(t *testing.T, update map[string]interface{}) string {
	t.Helper()
	condition, ok := update["ConditionExpression"].(string)
	if !ok {
		return ""
	}
	names, values := placeholders(t, update)
	resolved := regexp.MustCompile(`#\w+`).ReplaceAllStringFunc(condition, func(ph string) string { return names[ph] })
	resolved = regexp.MustCompile(`:\w+`).ReplaceAllStringFunc(resolved, func(ph string) string {
		if v, isString := values[ph]["S"].(string); isString {
			return `"` + v + `"`
		}
		if n, isNumber := values[ph]["N"].(string); isNumber {
			return n
		}
		return ph
	})
	return strings.ReplaceAll(resolved, " ", "")
}

// both scoped routes write only while the record is unbound or owned by the caller's project
// (lfx-self-serve-ops#982, #747)
func TestScopedReuseWritesCarryTheProjectCondition(t *testing.T) {
	t.Run("re-enable by repository id", func(t *testing.T) {
		table := &fakeRepositoriesTable{item: disabledUnboundItem()}
		repo := fakeRepository(t, table)
		_, err := repo.GitHubUpdateRepositoryForProject(context.Background(), "repo-1", "project-a", "foundation-x", reuseInput())
		require.NoError(t, err)
		table.mu.Lock()
		defer table.mu.Unlock()
		require.Len(t, table.updates, 1)
		assert.Equal(t, reuseConditionForProjectA, resolvedCondition(t, table.updates[0]))
		assert.Empty(t, table.puts)
	})
	t.Run("re-use of a disabled record found by GitHub id", func(t *testing.T) {
		table := &fakeRepositoriesTable{item: disabledUnboundItem()}
		repo := fakeRepository(t, table)
		_, err := repo.GitHubAddRepositoryForProject(context.Background(), "foundation-x", "project-a", reuseInput())
		require.NoError(t, err)
		table.mu.Lock()
		defer table.mu.Unlock()
		require.Len(t, table.updates, 1, "the existing record is re-used, not re-created")
		assert.Equal(t, reuseConditionForProjectA, resolvedCondition(t, table.updates[0]))
		assert.Empty(t, table.puts)
	})
	t.Run("a brand-new record is still created unconditionally", func(t *testing.T) {
		table := &fakeRepositoriesTable{}
		repo := fakeRepository(t, table)
		_, err := repo.GitHubAddRepositoryForProject(context.Background(), "foundation-x", "project-a", reuseInput())
		require.NoError(t, err)
		table.mu.Lock()
		defer table.mu.Unlock()
		assert.Empty(t, table.updates)
		require.Len(t, table.puts, 1)
		_, conditioned := table.puts[0]["ConditionExpression"]
		assert.False(t, conditioned)
	})
}

// a stale index answer (disabled and unbound) that another project rebound meanwhile fails the condition on
// the write itself: not found, nothing written, no fall-through to an unconditional path
func TestScopedReuseWritesRejectAStaleIndexAnswer(t *testing.T) {
	for _, route := range []string{"update", "add"} {
		t.Run(route, func(t *testing.T) {
			table := &fakeRepositoriesTable{item: disabledUnboundItem(), failCondition: true}
			repo := fakeRepository(t, table)
			var err error
			if route == "update" {
				_, err = repo.GitHubUpdateRepositoryForProject(context.Background(), "repo-1", "project-a", "foundation-x", reuseInput())
			} else {
				_, err = repo.GitHubAddRepositoryForProject(context.Background(), "foundation-x", "project-a", reuseInput())
			}
			var notFound *utils.GitHubRepositoryNotFound
			require.True(t, errors.As(err, &notFound), "expected not found, got %v", err)
			assert.Contains(t, err.Error(), "project-a")
			table.mu.Lock()
			defer table.mu.Unlock()
			assert.Len(t, table.updates, 1, "exactly one conditional write was attempted")
			assert.Empty(t, table.puts)
		})
	}
}

// the pre-existing public add and update keep writing without any condition
func TestUnscopedAddAndUpdateStillSendNoCondition(t *testing.T) {
	table := &fakeRepositoriesTable{item: disabledUnboundItem(), failCondition: true}
	repo := fakeRepository(t, table)
	_, err := repo.GitHubUpdateRepository(context.Background(), "repo-1", "project-a", "foundation-x", reuseInput())
	require.NoError(t, err)
	_, err = repo.GitHubAddRepository(context.Background(), "foundation-x", "project-a", reuseInput())
	require.NoError(t, err)
	table.mu.Lock()
	defer table.mu.Unlock()
	require.Len(t, table.updates, 2)
	for _, update := range table.updates {
		assert.Equal(t, "", resolvedCondition(t, update), "unchanged legacy behaviour")
	}
}

// reusableByProject mirrors repositoryBoundToOtherProject: project_sfid is authoritative, repository_sfdc_id
// only matters while project_sfid is absent or empty, and the record must still exist
func TestReusableByProjectSemantics(t *testing.T) {
	holds := func(exists bool, project, legacy string) bool {
		if !exists {
			return false
		}
		if project == projectA {
			return true
		}
		if project != "" {
			return false
		}
		return legacy == "" || legacy == projectA
	}
	cases := []struct {
		name            string
		exists          bool
		project, legacy string
		want            bool
	}{
		{name: "unbound, attributes absent", exists: true, want: true},
		{name: "unbound, present empty strings", exists: true, project: "", legacy: "", want: true},
		{name: "same owner with a different foundation", exists: true, project: projectA, legacy: "foundation-other", want: true},
		{name: "legacy binding to the same project", exists: true, legacy: projectA, want: true},
		{name: "legacy binding to a foreign project", exists: true, legacy: "project-b", want: false},
		{name: "foreign project even when the parent matches", exists: true, project: "project-b", legacy: projectA, want: false},
		{name: "deleted record is not recreated", exists: false, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, holds(tc.exists, tc.project, tc.legacy))
			if tc.exists {
				allowed := repositoryBoundToOtherProject(&models.GithubRepository{RepositoryProjectSfid: tc.project, RepositorySfdcID: tc.legacy}, projectA) == nil
				assert.Equal(t, tc.want, allowed, "the write condition and the pure predicate agree for existing records")
			}
		})
	}
}

func TestEmptyBindingsAreStoredAsNull(t *testing.T) {
	item, err := dynamodbattribute.MarshalMap(&RepositoryDBModel{RepositoryID: "repo-1"})
	require.NoError(t, err)
	assert.True(t, aws.BoolValue(item["project_sfid"].NULL))
	assert.True(t, aws.BoolValue(item["repository_sfdc_id"].NULL))
}

// stored NULL bindings are unbound for the Go predicate and for the write condition alike
func TestScopedReuseAcceptsStoredNullBindings(t *testing.T) {
	routes := map[string]func(*Repository) (*models.GithubRepository, error){
		"re-enable by repository id": func(r *Repository) (*models.GithubRepository, error) {
			return r.GitHubUpdateRepositoryForProject(context.Background(), "repo-1", "project-a", "foundation-x", reuseInput())
		},
		"re-use of a disabled record found by GitHub id": func(r *Repository) (*models.GithubRepository, error) {
			return r.GitHubAddRepositoryForProject(context.Background(), "foundation-x", "project-a", reuseInput())
		},
	}
	for name, route := range routes {
		t.Run(name, func(t *testing.T) {
			table := &fakeRepositoriesTable{item: disabledNullBoundItem()}
			_, err := route(fakeRepository(t, table))
			require.NoError(t, err)
			table.mu.Lock()
			defer table.mu.Unlock()
			require.Len(t, table.updates, 1, "the NULL-bound record is re-used, not re-created")
			assert.Equal(t, reuseConditionForProjectA, resolvedCondition(t, table.updates[0]))
			assert.Empty(t, table.puts)
		})
	}
}
