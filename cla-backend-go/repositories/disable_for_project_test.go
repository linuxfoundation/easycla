// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// fakeRepositoriesTable serves the DynamoDB JSON protocol for one stored row: GetItem and Query return it,
// every UpdateItem, PutItem and TransactWriteItems is recorded; with failCondition set, a conditional update
// fails the way DynamoDB reports it, with failRegistration set, the organization check of a transaction fails
type fakeRepositoriesTable struct {
	mu               sync.Mutex
	item             map[string]interface{}
	updates          []map[string]interface{}
	puts             []map[string]interface{}
	transactions     []map[string]interface{}
	failCondition    bool
	failRegistration bool
}

func (f *fakeRepositoriesTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	f.mu.Lock()
	defer f.mu.Unlock()
	var response interface{}
	switch r.Header.Get("X-Amz-Target") {
	case "DynamoDB_20120810.GetItem":
		response = map[string]interface{}{"Item": f.item}
	case "DynamoDB_20120810.Query":
		items := []map[string]interface{}{}
		if f.item != nil {
			items = append(items, f.item)
		}
		response = map[string]interface{}{"Items": items, "Count": len(items)}
	case "DynamoDB_20120810.PutItem":
		f.puts = append(f.puts, body)
		response = map[string]interface{}{}
	case "DynamoDB_20120810.UpdateItem":
		f.updates = append(f.updates, body)
		if f.failCondition && body["ConditionExpression"] != nil {
			w.WriteHeader(http.StatusBadRequest)
			response = map[string]string{"__type": "com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException", "message": "The conditional request failed"}
		} else {
			response = map[string]interface{}{}
		}
	case "DynamoDB_20120810.TransactWriteItems":
		f.transactions = append(f.transactions, body)
		items, ok := body["TransactItems"].([]interface{})
		if !ok {
			http.Error(w, "TransactItems missing", http.StatusBadRequest)
			return
		}
		reasons := make([]map[string]string, len(items))
		canceled := false
		for i, raw := range items {
			reasons[i] = map[string]string{"Code": "None"}
			item, isItem := raw.(map[string]interface{})
			if !isItem {
				http.Error(w, "malformed transaction item", http.StatusBadRequest)
				return
			}
			_, isCheck := item["ConditionCheck"]
			update, isUpdate := item["Update"].(map[string]interface{})
			if (isCheck && f.failRegistration) || (isUpdate && f.failCondition && update["ConditionExpression"] != nil) {
				reasons[i] = map[string]string{"Code": "ConditionalCheckFailed", "Message": "The conditional request failed"}
				canceled = true
			}
		}
		if canceled {
			w.WriteHeader(http.StatusBadRequest)
			response = map[string]interface{}{"__type": "com.amazonaws.dynamodb.v20120810#TransactionCanceledException", "Message": "Transaction cancelled, please refer cancellation reasons for specific reasons", "CancellationReasons": reasons}
		} else {
			response = map[string]interface{}{}
		}
	default:
		http.Error(w, "unexpected operation "+r.Header.Get("X-Amz-Target"), http.StatusBadRequest)
		return
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func fakeRepository(t *testing.T, table *fakeRepositoriesTable) *Repository {
	t.Helper()
	server := httptest.NewServer(table)
	t.Cleanup(server.Close)
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(server.URL),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
		MaxRetries:  aws.Int(0),
	})
	require.NoError(t, err)
	return &Repository{stage: "test", dynamoDBClient: dynamodb.New(sess), repositoryTableName: "cla-test-repositories", githubOrgTableName: "cla-test-github-orgs"}
}

func boundRepositoryItem() map[string]interface{} {
	return map[string]interface{}{
		"repository_id":          map[string]interface{}{"S": "repo-1"},
		"repository_external_id": map[string]interface{}{"S": "123456"},
		"repository_name":        map[string]interface{}{"S": "org/repo"},
		"project_sfid":           map[string]interface{}{"S": "project-a"},
		"enabled":                map[string]interface{}{"BOOL": true},
		"note":                   map[string]interface{}{"S": "added"},
	}
}

// resolves the placeholder names and values of one recorded UpdateItem request
func placeholders(t *testing.T, update map[string]interface{}) (map[string]string, map[string]map[string]interface{}) {
	t.Helper()
	names := map[string]string{}
	rawNames, ok := update["ExpressionAttributeNames"].(map[string]interface{})
	require.True(t, ok)
	for k, v := range rawNames {
		name, isString := v.(string)
		require.True(t, isString)
		names[k] = name
	}
	values := map[string]map[string]interface{}{}
	rawValues, ok := update["ExpressionAttributeValues"].(map[string]interface{})
	require.True(t, ok)
	for k, v := range rawValues {
		value, isMap := v.(map[string]interface{})
		require.True(t, isMap)
		values[k] = value
	}
	return names, values
}

// the project binding is enforced by the write itself (lfx-self-serve-ops#748): the update carries a condition
// on project_sfid with the caller's project, next to the usual disable update
func TestGitHubDisableRepositoryForProjectSendsTheCondition(t *testing.T) {
	table := &fakeRepositoriesTable{item: boundRepositoryItem()}
	repo := fakeRepository(t, table)
	require.NoError(t, repo.GitHubDisableRepositoryForProject(context.Background(), "repo-1", "project-a"))

	table.mu.Lock()
	defer table.mu.Unlock()
	require.Len(t, table.updates, 1)
	update := table.updates[0]
	condition, ok := update["ConditionExpression"].(string)
	require.True(t, ok, "the disabling update must carry a condition")
	match := regexp.MustCompile(`^(#\w+) = (:\w+)$`).FindStringSubmatch(condition)
	require.NotNil(t, match, condition)
	names, values := placeholders(t, update)
	assert.Equal(t, "project_sfid", names[match[1]])
	assert.Equal(t, map[string]interface{}{"S": "project-a"}, values[match[2]], "the condition compares against the caller's project")

	updateExpression, ok := update["UpdateExpression"].(string)
	require.True(t, ok)
	assert.Contains(t, updateExpression, "SET ")
	assert.Contains(t, updateExpression, "REMOVE ")
	attributes := map[string]bool{}
	for _, name := range names {
		attributes[name] = true
	}
	for _, name := range []string{"enabled", "note", "date_modified", "project_sfid", "repository_sfdc_id", "repository_project_id"} {
		assert.True(t, attributes[name], "update touches %s", name)
	}
	key, ok := update["Key"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, map[string]interface{}{"S": "repo-1"}, key["repository_id"])
}

// a record rebound to another project between the read and the write fails the condition and is reported
// as not found, nothing else is attempted
func TestGitHubDisableRepositoryForProjectMapsTheConditionFailure(t *testing.T) {
	table := &fakeRepositoriesTable{item: boundRepositoryItem(), failCondition: true}
	repo := fakeRepository(t, table)
	err := repo.GitHubDisableRepositoryForProject(context.Background(), "repo-1", "project-b")
	var notFound *utils.GitHubRepositoryNotFound
	require.True(t, errors.As(err, &notFound), "expected not found, got %v", err)
	assert.Contains(t, err.Error(), "repo-1")
	assert.Contains(t, err.Error(), "project-b")
	table.mu.Lock()
	defer table.mu.Unlock()
	assert.Len(t, table.updates, 1, "exactly one conditional write was attempted")
}

// the pre-existing disable keeps writing without any condition although the helper is shared now
func TestGitHubDisableRepositoryStillSendsNoCondition(t *testing.T) {
	table := &fakeRepositoriesTable{item: boundRepositoryItem(), failCondition: true}
	repo := fakeRepository(t, table)
	require.NoError(t, repo.GitHubDisableRepository(context.Background(), "repo-1"))
	table.mu.Lock()
	defer table.mu.Unlock()
	require.Len(t, table.updates, 1)
	_, hasCondition := table.updates[0]["ConditionExpression"]
	assert.False(t, hasCondition, "the unconditional disable is unchanged")
	assert.Contains(t, table.updates[0]["UpdateExpression"], "REMOVE ")
}
