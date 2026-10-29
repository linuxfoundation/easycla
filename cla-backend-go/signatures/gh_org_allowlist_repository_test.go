// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ghOrgAllowlistTable(orgs ...string) *fakeSignaturesTable {
	item := map[string]interface{}{
		"signature_id":  fakeS("ccla-sig"),
		"signature_acl": fakeStringList("manager-lf", "other-manager"),
	}
	if len(orgs) > 0 {
		item["github_org_whitelist"] = fakeStringList(orgs...)
	}
	return &fakeSignaturesTable{items: []map[string]interface{}{item}, invalidated: map[string]int{}}
}

func ghOrgAllowlistRepo(t *testing.T, table *fakeSignaturesTable) repository {
	t.Helper()
	return repository{stage: "test", dynamoDBClient: dynamodb.New(gitLabRemovalSession(t, table)), signatureTableName: "cla-test-signatures"}
}

// revokeManager models a concurrent CLA-manager removal between the read and the write
func revokeManager(table *fakeSignaturesTable, lfUsername string) {
	table.beforeUpdate = func(item map[string]interface{}) {
		acl, ok := item["signature_acl"].(map[string]interface{})
		if !ok {
			return
		}
		list, ok := acl["L"].([]interface{})
		if !ok {
			return
		}
		kept := make([]interface{}, 0, len(list))
		for _, e := range list {
			if em, isMap := e.(map[string]interface{}); isMap && em["S"] == lfUsername {
				continue
			}
			kept = append(kept, e)
		}
		item["signature_acl"] = map[string]interface{}{"L": kept}
	}
}

// the GitHub-organization approval-list writes carry the acting manager as a condition on signature_acl:
// a caller outside the ACL at read time writes nothing, a manager revoked between the read and the write
// is refused by the write itself, and an ordinary manager's change lands with that condition
func TestGitHubOrgAllowlistWritesAreConditionedOnTheSignatureACL(t *testing.T) {
	t.Run("add by a manager", func(t *testing.T) {
		table := ghOrgAllowlistTable("org-a")
		repo := ghOrgAllowlistRepo(t, table)
		_, err := repo.AddGithubOrganizationToApprovalList(context.Background(), "ccla-sig", "org-b", "manager-lf")
		require.NoError(t, err)
		table.mu.Lock()
		defer table.mu.Unlock()
		assert.Equal(t, fakeStringList("org-a", "org-b"), table.find("ccla-sig")["github_org_whitelist"])
		require.Len(t, table.updates, 1)
		assert.Equal(t, "contains(#ACL, :actor)", table.updates[0].ConditionExpression)
		assert.Equal(t, "signature_acl", table.updates[0].ExpressionAttributeNames["#ACL"])
		if actor := table.updates[0].ExpressionAttributeValues[":actor"]; assert.NotNil(t, actor.S) {
			assert.Equal(t, "manager-lf", *actor.S)
		}
		assert.Equal(t, 0, table.conditionFailures)
	})
	t.Run("add of an already listed organization by an outsider writes nothing", func(t *testing.T) {
		table := ghOrgAllowlistTable("org-a")
		repo := ghOrgAllowlistRepo(t, table)
		_, err := repo.AddGithubOrganizationToApprovalList(context.Background(), "ccla-sig", "org-a", "outsider")
		assert.IsType(t, &ForbiddenError{}, err, "the no-op path refuses a caller outside the ACL as well")
		table.mu.Lock()
		defer table.mu.Unlock()
		assert.Empty(t, table.updates)
	})
	t.Run("add by an outsider writes nothing", func(t *testing.T) {
		table := ghOrgAllowlistTable("org-a")
		repo := ghOrgAllowlistRepo(t, table)
		_, err := repo.AddGithubOrganizationToApprovalList(context.Background(), "ccla-sig", "org-b", "outsider")
		assert.IsType(t, &ForbiddenError{}, err)
		table.mu.Lock()
		defer table.mu.Unlock()
		assert.Empty(t, table.updates)
		assert.Equal(t, fakeStringList("org-a"), table.find("ccla-sig")["github_org_whitelist"])
	})
	t.Run("add by a manager revoked before the write", func(t *testing.T) {
		table := ghOrgAllowlistTable("org-a")
		revokeManager(table, "manager-lf")
		repo := ghOrgAllowlistRepo(t, table)
		_, err := repo.AddGithubOrganizationToApprovalList(context.Background(), "ccla-sig", "org-b", "manager-lf")
		assert.IsType(t, &ForbiddenError{}, err)
		table.mu.Lock()
		defer table.mu.Unlock()
		assert.Equal(t, 1, table.conditionFailures)
		assert.Equal(t, fakeStringList("org-a"), table.find("ccla-sig")["github_org_whitelist"], "the stale read authorizes nothing")
	})
	t.Run("add on a missing signature writes nothing", func(t *testing.T) {
		table := ghOrgAllowlistTable("org-a")
		repo := ghOrgAllowlistRepo(t, table)
		_, err := repo.AddGithubOrganizationToApprovalList(context.Background(), "missing-sig", "org-b", "manager-lf")
		assert.IsType(t, &ForbiddenError{}, err)
		table.mu.Lock()
		defer table.mu.Unlock()
		assert.Empty(t, table.updates)
		assert.Empty(t, table.upserts, "no row is created")
	})
	t.Run("delete with remaining organizations by a manager", func(t *testing.T) {
		table := ghOrgAllowlistTable("org-a", "org-b")
		repo := ghOrgAllowlistRepo(t, table)
		_, err := repo.DeleteGithubOrganizationFromApprovalList(context.Background(), "ccla-sig", "org-a", "manager-lf")
		require.NoError(t, err)
		table.mu.Lock()
		defer table.mu.Unlock()
		assert.Equal(t, fakeStringList("org-b"), table.find("ccla-sig")["github_org_whitelist"])
		require.Len(t, table.updates, 1)
		assert.Equal(t, "contains(#ACL, :actor)", table.updates[0].ConditionExpression)
	})
	t.Run("delete of the last organization by a manager", func(t *testing.T) {
		table := ghOrgAllowlistTable("org-a")
		repo := ghOrgAllowlistRepo(t, table)
		orgs, err := repo.DeleteGithubOrganizationFromApprovalList(context.Background(), "ccla-sig", "org-a", "manager-lf")
		require.NoError(t, err)
		assert.Empty(t, orgs)
		table.mu.Lock()
		defer table.mu.Unlock()
		assert.Equal(t, map[string]interface{}{"NULL": true}, table.find("ccla-sig")["github_org_whitelist"])
		require.Len(t, table.updates, 1)
		assert.Equal(t, "contains(#ACL, :actor)", table.updates[0].ConditionExpression)
	})
	t.Run("delete with remaining organizations by a revoked manager", func(t *testing.T) {
		table := ghOrgAllowlistTable("org-a", "org-b")
		revokeManager(table, "manager-lf")
		repo := ghOrgAllowlistRepo(t, table)
		_, err := repo.DeleteGithubOrganizationFromApprovalList(context.Background(), "ccla-sig", "org-a", "manager-lf")
		assert.IsType(t, &ForbiddenError{}, err)
		table.mu.Lock()
		defer table.mu.Unlock()
		assert.Equal(t, 1, table.conditionFailures)
		assert.Equal(t, fakeStringList("org-a", "org-b"), table.find("ccla-sig")["github_org_whitelist"])
	})
	t.Run("delete of the last organization by a revoked manager", func(t *testing.T) {
		table := ghOrgAllowlistTable("org-a")
		revokeManager(table, "manager-lf")
		repo := ghOrgAllowlistRepo(t, table)
		_, err := repo.DeleteGithubOrganizationFromApprovalList(context.Background(), "ccla-sig", "org-a", "manager-lf")
		assert.IsType(t, &ForbiddenError{}, err)
		table.mu.Lock()
		defer table.mu.Unlock()
		assert.Equal(t, 1, table.conditionFailures)
		assert.Equal(t, fakeStringList("org-a"), table.find("ccla-sig")["github_org_whitelist"])
	})
	t.Run("delete by an outsider writes nothing", func(t *testing.T) {
		table := ghOrgAllowlistTable("org-a")
		repo := ghOrgAllowlistRepo(t, table)
		_, err := repo.DeleteGithubOrganizationFromApprovalList(context.Background(), "ccla-sig", "org-a", "outsider")
		assert.IsType(t, &ForbiddenError{}, err)
		table.mu.Lock()
		defer table.mu.Unlock()
		assert.Empty(t, table.updates)
	})
}
