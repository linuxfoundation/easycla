// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCompaniesTable is a minimal in-memory DynamoDB endpoint for the companies table: GetItem,
// conditional PutItem (attribute_not_exists), Query on external-company-index and the conditional
// UpdateItem used by UpdateCompanyExternalID.
type fakeCompaniesTable struct {
	mu                sync.Mutex
	items             map[string]map[string]interface{}
	hiddenFromIndex   map[string]bool // simulates GSI propagation lag
	puts              int
	conditionFailures int
	lastCondition     string
	failPuts          bool
}

type fakeAttr struct {
	S    *string
	BOOL *bool
}

var fakePlaceholderPair = regexp.MustCompile(`(#[0-9A-Za-z_]+) = (:[0-9A-Za-z_]+)`)

func (f *fakeCompaniesTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Header.Get("X-Amz-Target") {
	case "DynamoDB_20120810.GetItem":
		var req struct {
			Key map[string]fakeAttr
		}
		fakeCompanyDecode(w, body, &req)
		resp := map[string]interface{}{}
		if item, ok := f.items[*req.Key["company_id"].S]; ok {
			resp["Item"] = item
		}
		fakeCompanyJSON(w, resp)
	case "DynamoDB_20120810.PutItem":
		var req struct {
			Item                map[string]interface{}
			ConditionExpression string
		}
		fakeCompanyDecode(w, body, &req)
		id := fakeCompanyString(req.Item, "company_id")
		if f.failPuts {
			fakeCompanyError(w, "InternalServerError", "injected")
			return
		}
		if _, exists := f.items[id]; exists && strings.Contains(req.ConditionExpression, "attribute_not_exists(company_id)") {
			f.conditionFailures++
			fakeCompanyError(w, "ConditionalCheckFailedException", "exists")
			return
		}
		f.puts++
		f.items[id] = req.Item
		fakeCompanyJSON(w, map[string]interface{}{})
	case "DynamoDB_20120810.Query":
		var req struct {
			IndexName                 string
			KeyConditionExpression    string
			ExpressionAttributeNames  map[string]string
			ExpressionAttributeValues map[string]fakeAttr
		}
		fakeCompanyDecode(w, body, &req)
		m := fakePlaceholderPair.FindStringSubmatch(req.KeyConditionExpression)
		attr, value := req.ExpressionAttributeNames[m[1]], *req.ExpressionAttributeValues[m[2]].S
		matched := []map[string]interface{}{}
		for id, item := range f.items {
			if !f.hiddenFromIndex[id] && fakeCompanyString(item, attr) == value {
				matched = append(matched, item)
			}
		}
		fakeCompanyJSON(w, map[string]interface{}{"Items": matched, "Count": len(matched), "ScannedCount": len(matched)})
	case "DynamoDB_20120810.UpdateItem":
		var req struct {
			Key                       map[string]fakeAttr
			UpdateExpression          string
			ConditionExpression       string
			ExpressionAttributeNames  map[string]string
			ExpressionAttributeValues map[string]fakeAttr
		}
		fakeCompanyDecode(w, body, &req)
		item, ok := f.items[*req.Key["company_id"].S]
		if !ok {
			item = map[string]interface{}{"company_id": map[string]interface{}{"S": *req.Key["company_id"].S}}
		}
		f.lastCondition = req.ConditionExpression
		if req.ConditionExpression != "" {
			m := fakePlaceholderPair.FindStringSubmatch(req.ConditionExpression)
			if fakeCompanyString(item, req.ExpressionAttributeNames[m[1]]) != *req.ExpressionAttributeValues[m[2]].S {
				f.conditionFailures++
				fakeCompanyError(w, "ConditionalCheckFailedException", "condition failed")
				return
			}
		}
		for _, pair := range fakePlaceholderPair.FindAllStringSubmatch(strings.TrimPrefix(req.UpdateExpression, "SET "), -1) {
			item[req.ExpressionAttributeNames[pair[1]]] = map[string]interface{}{"S": *req.ExpressionAttributeValues[pair[2]].S}
		}
		f.items[*req.Key["company_id"].S] = item
		fakeCompanyJSON(w, map[string]interface{}{})
	default:
		http.Error(w, "unsupported operation "+r.Header.Get("X-Amz-Target"), http.StatusBadRequest)
	}
}

func fakeCompanyString(item map[string]interface{}, name string) string {
	if attr, ok := item[name].(map[string]interface{}); ok {
		if s, ok := attr["S"].(string); ok {
			return s
		}
	}
	return ""
}

func fakeCompanyJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func fakeCompanyDecode(w http.ResponseWriter, body []byte, v interface{}) {
	if err := json.Unmarshal(body, v); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func fakeCompanyError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.WriteHeader(http.StatusBadRequest)
	fakeCompanyJSON(w, map[string]string{"__type": "com.amazonaws.dynamodb.v20120810#" + code, "message": message})
}

func fakeCompanyItem(id, name, entity, externalID string) map[string]interface{} {
	return map[string]interface{}{
		"company_id":          map[string]interface{}{"S": id},
		"company_name":        map[string]interface{}{"S": name},
		"signing_entity_name": map[string]interface{}{"S": entity},
		"company_external_id": map[string]interface{}{"S": externalID},
		"date_created":        map[string]interface{}{"S": "2024-01-01T00:00:00Z"},
		"date_modified":       map[string]interface{}{"S": "2024-01-01T00:00:00Z"},
	}
}

func newCompanyRepo(t *testing.T, table *fakeCompaniesTable) (repository, *fakeCompaniesTable) {
	if table.items == nil {
		table.items = map[string]map[string]interface{}{}
	}
	server := httptest.NewServer(table)
	t.Cleanup(server.Close)
	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(server.URL),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
		MaxRetries:  aws.Int(0),
	})
	require.NoError(t, err)
	return repository{stage: "test", dynamoDBClient: dynamodb.New(awsSession), companyTableName: "cla-test-companies"}, table
}

func TestCanonicalSigningEntity(t *testing.T) {
	assert.Equal(t, "", canonicalSigningEntity("Acme Inc", ""))
	assert.Equal(t, "", canonicalSigningEntity("Acme Inc", " acme inc "))
	assert.Equal(t, "acme gmbh", canonicalSigningEntity("Acme Inc", " Acme GmbH"))
}

func TestDeterministicCompanyID(t *testing.T) {
	id := deterministicCompanyID("0014100000Te1TUAAZ", "")
	parsed, err := uuid.FromString(id)
	require.NoError(t, err)
	assert.Equal(t, byte(4), parsed.Version())
	assert.Equal(t, uuid.VariantRFC4122, parsed.Variant())
	assert.Regexp(t, `^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[89ab][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$`, id)
	assert.Equal(t, id, deterministicCompanyID("0014100000Te1TUAAZ", ""), "stable")
	assert.NotEqual(t, id, deterministicCompanyID("0014100000Te1TUAAZ", "acme gmbh"), "entity is part of the identity")
	assert.NotEqual(t, id, deterministicCompanyID("0014100000Te1TUAAY", ""), "external id is part of the identity")
}

func TestEnsureCompanyForExternalID(t *testing.T) {
	const sfid = "0014100000Te1TUAAZ"

	t.Run("empty external id is rejected without a write", func(t *testing.T) {
		repo, table := newCompanyRepo(t, &fakeCompaniesTable{})
		_, created, err := repo.EnsureCompanyForExternalID(context.Background(), " ", "Acme", "")
		assert.ErrorIs(t, err, ErrEmptyExternalID)
		assert.False(t, created)
		assert.Equal(t, 0, table.puts)
	})

	t.Run("existing parent row is reused with its original id", func(t *testing.T) {
		repo, table := newCompanyRepo(t, &fakeCompaniesTable{items: map[string]map[string]interface{}{
			"legacy-id": fakeCompanyItem("legacy-id", "Acme Inc", "Acme Inc", sfid),
		}})
		comp, created, err := repo.EnsureCompanyForExternalID(context.Background(), sfid, "Acme Inc", "")
		require.NoError(t, err)
		assert.False(t, created)
		assert.Equal(t, "legacy-id", comp.CompanyID)
		assert.Equal(t, 0, table.puts)
	})

	t.Run("existing named signing entity row is reused", func(t *testing.T) {
		repo, table := newCompanyRepo(t, &fakeCompaniesTable{items: map[string]map[string]interface{}{
			"parent": fakeCompanyItem("parent", "Acme Inc", "Acme Inc", sfid),
			"child":  fakeCompanyItem("child", "Acme Inc", "Acme GmbH", sfid),
		}})
		comp, created, err := repo.EnsureCompanyForExternalID(context.Background(), sfid, "Acme Inc", "acme gmbh ")
		require.NoError(t, err)
		assert.False(t, created)
		assert.Equal(t, "child", comp.CompanyID)
		assert.Equal(t, 0, table.puts)
	})

	t.Run("a same-name row with another external id is never reused", func(t *testing.T) {
		repo, table := newCompanyRepo(t, &fakeCompaniesTable{items: map[string]map[string]interface{}{
			"other": fakeCompanyItem("other", "Acme Inc", "Acme Inc", "0014100000Other00"),
		}})
		comp, created, err := repo.EnsureCompanyForExternalID(context.Background(), sfid, "Acme Inc", "")
		require.NoError(t, err)
		assert.True(t, created)
		assert.Equal(t, deterministicCompanyID(sfid, ""), comp.CompanyID)
		assert.Equal(t, sfid, comp.CompanyExternalID)
		assert.Equal(t, "Acme Inc", comp.SigningEntityName)
		assert.Equal(t, ensureCompanyNote, comp.Note)
		assert.Equal(t, 1, table.puts)
		assert.Len(t, table.items, 2)
	})

	t.Run("named entity row is created with the requested entity name", func(t *testing.T) {
		repo, _ := newCompanyRepo(t, &fakeCompaniesTable{items: map[string]map[string]interface{}{
			"parent": fakeCompanyItem("parent", "Acme Inc", "Acme Inc", sfid),
		}})
		comp, created, err := repo.EnsureCompanyForExternalID(context.Background(), sfid, "Acme Inc", "Acme GmbH")
		require.NoError(t, err)
		assert.True(t, created)
		assert.Equal(t, deterministicCompanyID(sfid, "acme gmbh"), comp.CompanyID)
		assert.Equal(t, "Acme GmbH", comp.SigningEntityName)
	})

	t.Run("concurrent creation converges on the winning row", func(t *testing.T) {
		// the winner's row exists on the primary key but the GSI has not caught up yet
		winnerID := deterministicCompanyID(sfid, "")
		repo, table := newCompanyRepo(t, &fakeCompaniesTable{
			items:           map[string]map[string]interface{}{winnerID: fakeCompanyItem(winnerID, "Acme Inc", "Acme Inc", sfid)},
			hiddenFromIndex: map[string]bool{winnerID: true},
		})
		comp, created, err := repo.EnsureCompanyForExternalID(context.Background(), sfid, "Acme Inc", "")
		require.NoError(t, err)
		assert.False(t, created)
		assert.Equal(t, winnerID, comp.CompanyID)
		assert.Equal(t, 1, table.conditionFailures)
		assert.Equal(t, 0, table.puts)
	})

	t.Run("a foreign row on the deterministic key is a conflict", func(t *testing.T) {
		winnerID := deterministicCompanyID(sfid, "")
		table := &fakeCompaniesTable{items: map[string]map[string]interface{}{
			winnerID: fakeCompanyItem(winnerID, "Someone Else", "Someone Else", "0014100000Other00"),
		}}
		repo, _ := newCompanyRepo(t, table)
		_, created, err := repo.EnsureCompanyForExternalID(context.Background(), sfid, "Acme Inc", "")
		assert.ErrorIs(t, err, ErrEnsureCompanyConflict)
		assert.False(t, created)
		assert.Equal(t, 1, table.conditionFailures)
		assert.Equal(t, 0, table.puts)
	})

	t.Run("a write failure is returned", func(t *testing.T) {
		repo, _ := newCompanyRepo(t, &fakeCompaniesTable{failPuts: true})
		_, created, err := repo.EnsureCompanyForExternalID(context.Background(), sfid, "Acme Inc", "")
		require.Error(t, err)
		assert.False(t, created)
	})
}

func TestUpdateCompanyExternalID(t *testing.T) {
	t.Run("rewrites and remembers the previous id", func(t *testing.T) {
		repo, table := newCompanyRepo(t, &fakeCompaniesTable{items: map[string]map[string]interface{}{
			"c1": fakeCompanyItem("c1", "Acme Inc", "Acme Inc", "lf-old"),
		}})
		require.NoError(t, repo.UpdateCompanyExternalID(context.Background(), "c1", "lf-old", "0014100000New0000"))
		record, err := repo.GetCompanyRecord(context.Background(), "c1")
		require.NoError(t, err)
		assert.Equal(t, "0014100000New0000", record.CompanyExternalID)
		assert.Equal(t, "lf-old", record.PreviousCompanyExternalID)
		assert.NotEqual(t, "2024-01-01T00:00:00Z", record.Updated)
		assert.Equal(t, 0, table.conditionFailures)
	})

	t.Run("a row that no longer carries the old id fails the condition", func(t *testing.T) {
		repo, table := newCompanyRepo(t, &fakeCompaniesTable{items: map[string]map[string]interface{}{
			"c1": fakeCompanyItem("c1", "Acme Inc", "Acme Inc", "0014100000New0000"),
		}})
		err := repo.UpdateCompanyExternalID(context.Background(), "c1", "lf-old", "0014100000New0000")
		assert.ErrorIs(t, err, ErrExternalIDConditionFailed)
		assert.Equal(t, 1, table.conditionFailures)
	})

	t.Run("a blank old id fills a row without an external id and records no previous value", func(t *testing.T) {
		missing := fakeCompanyItem("c1", "Blank Inc", "Blank Inc", "")
		delete(missing, "company_external_id")
		repo, table := newCompanyRepo(t, &fakeCompaniesTable{items: map[string]map[string]interface{}{
			"c1": missing,
			"c2": fakeCompanyItem("c2", "Blank Two", "Blank Two", ""),
		}})
		require.NoError(t, repo.UpdateCompanyExternalID(context.Background(), "c1", "", "0014100000New0000"))
		require.NoError(t, repo.UpdateCompanyExternalID(context.Background(), "c2", "  ", "0014100000New0001"))
		for id, want := range map[string]string{"c1": "0014100000New0000", "c2": "0014100000New0001"} {
			record, err := repo.GetCompanyRecord(context.Background(), id)
			require.NoError(t, err)
			assert.Equal(t, want, record.CompanyExternalID)
			assert.Equal(t, "", record.PreviousCompanyExternalID)
		}
		assert.Equal(t, 0, table.conditionFailures)
		assert.Contains(t, table.lastCondition, "attribute_not_exists(#E) OR #E = :old")
	})

	t.Run("a blank old id never overwrites an existing external id", func(t *testing.T) {
		repo, table := newCompanyRepo(t, &fakeCompaniesTable{items: map[string]map[string]interface{}{
			"c1": fakeCompanyItem("c1", "Acme Inc", "Acme Inc", "lf-old"),
		}})
		err := repo.UpdateCompanyExternalID(context.Background(), "c1", "", "0014100000New0000")
		assert.ErrorIs(t, err, ErrExternalIDConditionFailed)
		assert.Equal(t, 1, table.conditionFailures)
		record, err := repo.GetCompanyRecord(context.Background(), "c1")
		require.NoError(t, err)
		assert.Equal(t, "lf-old", record.CompanyExternalID)
	})

	t.Run("blank company or new id is rejected", func(t *testing.T) {
		repo, _ := newCompanyRepo(t, &fakeCompaniesTable{})
		assert.Error(t, repo.UpdateCompanyExternalID(context.Background(), "", "old", "new"))
		assert.Error(t, repo.UpdateCompanyExternalID(context.Background(), "c1", "old", ""))
	})
}
