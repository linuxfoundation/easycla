// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// the organization registration check of every scoped write, placeholders resolved and spaces removed
const registrationConditionForOrg = `((((attribute_exists(organization_name))AND(enabled=true))AND(project_sfid="project-a"))AND(organization_sfid="foundation-x"))AND(organization_installation_id=42)`

type scopedRoute struct {
	name  string
	item  map[string]interface{}
	kind  string
	write func(*Repository, *models.GithubOrganization) (*models.GithubRepository, error)
}

func scopedRoutes() []scopedRoute {
	return []scopedRoute{
		{name: "re-enable by repository id", item: disabledUnboundItem(), kind: "Update", write: func(r *Repository, registered *models.GithubOrganization) (*models.GithubRepository, error) {
			return r.GitHubUpdateRepositoryForProject(context.Background(), "repo-1", projectA, "foundation-x", reuseInput(), registered)
		}},
		{name: "re-use of a disabled record found by GitHub id", item: disabledUnboundItem(), kind: "Update", write: func(r *Repository, registered *models.GithubOrganization) (*models.GithubRepository, error) {
			return r.GitHubAddRepositoryForProject(context.Background(), "foundation-x", projectA, reuseInput(), registered)
		}},
		{name: "creation of a new record", kind: "Put", write: func(r *Repository, registered *models.GithubOrganization) (*models.GithubRepository, error) {
			return r.GitHubAddRepositoryForProject(context.Background(), "foundation-x", projectA, reuseInput(), registered)
		}},
	}
}

// every project-scoped enrollment write commits together with a check that the organization registration the
// enrollment was authorized through is still stored unchanged (lfx-self-serve-ops#1200)
func TestScopedWritesCheckTheOrganizationRegistration(t *testing.T) {
	for _, route := range scopedRoutes() {
		t.Run(route.name, func(t *testing.T) {
			table := &fakeRepositoriesTable{item: route.item}
			_, err := route.write(fakeRepository(t, table), registration())
			require.NoError(t, err)
			table.mu.Lock()
			defer table.mu.Unlock()
			check, write, kind := transacted(t, table)
			assert.Equal(t, "cla-test-github-orgs", check["TableName"])
			assert.Equal(t, map[string]interface{}{"organization_name": map[string]interface{}{"S": "org"}}, check["Key"])
			assert.Equal(t, registrationConditionForOrg, resolvedCondition(t, check))
			assert.Equal(t, route.kind, kind)
			assert.Equal(t, "cla-test-repositories", write["TableName"])
			assert.Empty(t, table.updates, "no write outside the transaction")
			assert.Empty(t, table.puts, "no write outside the transaction")
		})
	}
}

// a registration disabled, deleted or re-bound between the authorization and the write cancels the whole
// transaction: typed not-found, nothing written
func TestScopedWritesRejectAChangedRegistration(t *testing.T) {
	for _, route := range scopedRoutes() {
		t.Run(route.name, func(t *testing.T) {
			table := &fakeRepositoriesTable{item: route.item, failRegistration: true}
			_, err := route.write(fakeRepository(t, table), registration())
			var notFound *utils.GitHubOrgNotFound
			require.True(t, errors.As(err, &notFound), "expected the registration error, got %v", err)
			assert.True(t, errors.Is(err, ErrRegistrationChanged))
			assert.Equal(t, "org", notFound.OrganizationName)
			assert.Equal(t, projectA, notFound.ProjectSFID)
			table.mu.Lock()
			defer table.mu.Unlock()
			assert.Len(t, table.transactions, 1, "exactly one transaction was attempted")
			assert.Empty(t, table.updates)
			assert.Empty(t, table.puts)
		})
	}
}

// the scoped writes never fall back to an unchecked write
func TestScopedWritesRequireTheRegistration(t *testing.T) {
	for _, route := range scopedRoutes() {
		t.Run(route.name, func(t *testing.T) {
			table := &fakeRepositoriesTable{item: route.item}
			_, err := route.write(fakeRepository(t, table), nil)
			assert.ErrorIs(t, err, ErrRegistrationRequired)
			table.mu.Lock()
			defer table.mu.Unlock()
			assert.Empty(t, table.transactions)
			assert.Empty(t, table.updates)
			assert.Empty(t, table.puts)
		})
	}
}

// renderCondition resolves the placeholders of a built condition and removes all spaces
func renderCondition(t *testing.T, expr expression.Expression) string {
	t.Helper()
	resolved := regexp.MustCompile(`#\w+`).ReplaceAllStringFunc(aws.StringValue(expr.Condition()), func(ph string) string { return aws.StringValue(expr.Names()[ph]) })
	resolved = regexp.MustCompile(`:\w+`).ReplaceAllStringFunc(resolved, func(ph string) string {
		value := expr.Values()[ph]
		switch {
		case value == nil:
			return ph
		case value.S != nil:
			return `"` + *value.S + `"`
		case value.N != nil:
			return *value.N
		case value.BOOL != nil:
			return strconv.FormatBool(*value.BOOL)
		}
		return ph
	})
	return strings.ReplaceAll(resolved, " ", "")
}

// the check compares every binding of the registration the enrollment was authorized through; a binding the
// registration lacks must be unset on the row, an unrecorded installation must still be unrecorded
func TestRegistrationUnchangedCondition(t *testing.T) {
	cases := []struct {
		name       string
		registered *models.GithubOrganization
		want       string
	}{
		{name: "fully bound", registered: registration(), want: registrationConditionForOrg},
		{name: "foundation binding absent", registered: &models.GithubOrganization{OrganizationName: "org", OrganizationInstallationID: 42, ProjectSFID: projectA, Enabled: true},
			want: `((((attribute_exists(organization_name))AND(enabled=true))AND(project_sfid="project-a"))AND(((attribute_not_exists(organization_sfid))OR(attribute_type(organization_sfid,"NULL")))OR(size(organization_sfid)=0)))AND(organization_installation_id=42)`},
		{name: "no installation recorded", registered: &models.GithubOrganization{OrganizationName: "org", ProjectSFID: projectA, OrganizationSfid: "foundation-x", Enabled: true},
			want: `((((attribute_exists(organization_name))AND(enabled=true))AND(project_sfid="project-a"))AND(organization_sfid="foundation-x"))AND((attribute_not_exists(organization_installation_id))OR(organization_installation_id=0))`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := expression.NewBuilder().WithCondition(registrationUnchanged(tc.registered)).Build()
			require.NoError(t, err)
			assert.Equal(t, tc.want, renderCondition(t, expr))
		})
	}
}
