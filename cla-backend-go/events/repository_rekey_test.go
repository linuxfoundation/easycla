// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEventsTable is a minimal in-memory DynamoDB endpoint for the events table: GetItem, Query on
// a single-attribute key (paginated pageSize items at a time) and conditional UpdateItem.
type fakeEventsTable struct {
	mu                sync.Mutex
	items             map[string]map[string]string
	pageSize          int
	queries           int
	updates           int
	conditionFailures int
	beforeUpdate      func(items map[string]map[string]string)
}

type fakeEventAttr struct {
	S *string
}

var fakeEventPair = regexp.MustCompile(`(#[0-9A-Za-z_]+) = (:[0-9A-Za-z_]+)`)

func (f *fakeEventsTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Header.Get("X-Amz-Target") {
	case "DynamoDB_20120810.GetItem":
		var req struct{ Key map[string]fakeEventAttr }
		fakeEventDecode(w, body, &req)
		resp := map[string]interface{}{}
		if item, ok := f.items[*req.Key["event_id"].S]; ok {
			resp["Item"] = fakeEventWire(item)
		}
		fakeEventJSON(w, resp)
	case "DynamoDB_20120810.Query":
		f.queries++
		var req struct {
			KeyConditionExpression    string
			ExpressionAttributeNames  map[string]string
			ExpressionAttributeValues map[string]fakeEventAttr
			ExclusiveStartKey         map[string]fakeEventAttr
		}
		fakeEventDecode(w, body, &req)
		m := fakeEventPair.FindStringSubmatch(req.KeyConditionExpression)
		attr, value := req.ExpressionAttributeNames[m[1]], *req.ExpressionAttributeValues[m[2]].S
		var ids []string
		for id, item := range f.items {
			if item[attr] == value {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		if start, ok := req.ExclusiveStartKey["event_id"]; ok {
			for i, id := range ids {
				if id == *start.S {
					ids = ids[i+1:]
					break
				}
			}
		}
		resp := map[string]interface{}{}
		if f.pageSize > 0 && len(ids) > f.pageSize {
			ids = ids[:f.pageSize]
			resp["LastEvaluatedKey"] = map[string]interface{}{"event_id": map[string]string{"S": ids[len(ids)-1]}}
		}
		page := []map[string]interface{}{}
		for _, id := range ids {
			page = append(page, map[string]interface{}{"event_id": map[string]string{"S": id}})
		}
		resp["Items"], resp["Count"] = page, len(page)
		fakeEventJSON(w, resp)
	case "DynamoDB_20120810.UpdateItem":
		var req struct {
			Key                       map[string]fakeEventAttr
			UpdateExpression          string
			ConditionExpression       string
			ExpressionAttributeNames  map[string]string
			ExpressionAttributeValues map[string]fakeEventAttr
		}
		fakeEventDecode(w, body, &req)
		if f.beforeUpdate != nil {
			f.beforeUpdate(f.items)
		}
		item := f.items[*req.Key["event_id"].S]
		for _, pair := range fakeEventPair.FindAllStringSubmatch(req.ConditionExpression, -1) {
			if item[req.ExpressionAttributeNames[pair[1]]] != *req.ExpressionAttributeValues[pair[2]].S {
				f.conditionFailures++
				w.Header().Set("Content-Type", "application/x-amz-json-1.0")
				w.WriteHeader(http.StatusBadRequest)
				fakeEventJSON(w, map[string]string{"__type": "com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException", "message": "condition failed"})
				return
			}
		}
		for _, pair := range fakeEventPair.FindAllStringSubmatch(strings.TrimPrefix(req.UpdateExpression, "SET "), -1) {
			item[req.ExpressionAttributeNames[pair[1]]] = *req.ExpressionAttributeValues[pair[2]].S
		}
		f.updates++
		fakeEventJSON(w, map[string]interface{}{})
	default:
		http.Error(w, "unsupported operation "+r.Header.Get("X-Amz-Target"), http.StatusBadRequest)
	}
}

func fakeEventWire(item map[string]string) map[string]interface{} {
	wire := map[string]interface{}{}
	for k, v := range item {
		wire[k] = map[string]string{"S": v}
	}
	return wire
}

func fakeEventJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func fakeEventDecode(w http.ResponseWriter, body []byte, v interface{}) {
	if err := json.Unmarshal(body, v); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func newRekeyRepo(t *testing.T, table *fakeEventsTable) RekeyRepository {
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
	return NewRekeyRepository(awsSession, "test")
}

func fakeEvent(id, sfid, foundation, project, claGroup string) map[string]string {
	item := map[string]string{"event_id": id, "event_type": "TestEvent", "event_data_lower": "data " + id}
	if sfid != "" {
		item["event_company_sfid"] = sfid
	}
	if foundation != "" {
		item["company_sfid_foundation_sfid"] = sfid + "#" + foundation
	}
	if project != "" {
		item["company_sfid_project_id"] = sfid + "#" + project
	}
	if claGroup != "" {
		item["company_sfid_cla_group_id"] = sfid + "#" + claGroup
	}
	return item
}

func TestListEventIDsByCompanySFID(t *testing.T) {
	table := &fakeEventsTable{pageSize: 2, items: map[string]map[string]string{
		"e1": fakeEvent("e1", "lf-old", "f1", "p1", "g1"),
		"e2": fakeEvent("e2", "lf-old", "", "", "g1"),
		"e3": fakeEvent("e3", "lf-old", "f1", "", ""),
		"e4": fakeEvent("e4", "0014100000Other00", "f1", "p1", "g1"),
		"e5": fakeEvent("e5", "", "", "", ""),
	}}
	repo := newRekeyRepo(t, table)

	ids, err := repo.ListEventIDsByCompanySFID(context.Background(), "lf-old")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"e1", "e2", "e3"}, ids)
	assert.Equal(t, 2, table.queries, "pagination follows LastEvaluatedKey")

	ids, err = repo.ListEventIDsByCompanySFIDCLAGroup(context.Background(), "lf-old", "g1")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"e1", "e2"}, ids)

	ids, err = repo.ListEventIDsByCompanySFID(context.Background(), "unknown")
	require.NoError(t, err)
	assert.Empty(t, ids)

	_, err = repo.ListEventIDsByCompanySFID(context.Background(), " ")
	assert.Error(t, err)
	_, err = repo.ListEventIDsByCompanySFIDCLAGroup(context.Background(), "lf-old", "")
	assert.Error(t, err)
}

func TestRekeyEventCompanySFID(t *testing.T) {
	const oldSFID, newSFID = "lf-old", "0014100000New0000"

	t.Run("rewrites the sfid and every composite key, keeps the rest", func(t *testing.T) {
		table := &fakeEventsTable{items: map[string]map[string]string{"e1": fakeEvent("e1", oldSFID, "f1", "p1", "g1")}}
		repo := newRekeyRepo(t, table)
		changed, err := repo.RekeyEventCompanySFID(context.Background(), "e1", oldSFID, newSFID)
		require.NoError(t, err)
		assert.True(t, changed)
		assert.Equal(t, map[string]string{
			"event_id":                     "e1",
			"event_type":                   "TestEvent",
			"event_data_lower":             "data e1",
			"event_company_sfid":           newSFID,
			"company_sfid_foundation_sfid": newSFID + "#f1",
			"company_sfid_project_id":      newSFID + "#p1",
			"company_sfid_cla_group_id":    newSFID + "#g1",
		}, table.items["e1"])
		assert.Equal(t, 1, table.updates)
	})

	t.Run("only the attributes present are rewritten", func(t *testing.T) {
		table := &fakeEventsTable{items: map[string]map[string]string{"e2": fakeEvent("e2", oldSFID, "", "", "g1")}}
		repo := newRekeyRepo(t, table)
		changed, err := repo.RekeyEventCompanySFID(context.Background(), "e2", oldSFID, newSFID)
		require.NoError(t, err)
		assert.True(t, changed)
		assert.Equal(t, newSFID, table.items["e2"]["event_company_sfid"])
		assert.Equal(t, newSFID+"#g1", table.items["e2"]["company_sfid_cla_group_id"])
		_, hasFoundation := table.items["e2"]["company_sfid_foundation_sfid"]
		assert.False(t, hasFoundation)
	})

	t.Run("is idempotent", func(t *testing.T) {
		table := &fakeEventsTable{items: map[string]map[string]string{"e1": fakeEvent("e1", newSFID, "f1", "p1", "g1")}}
		repo := newRekeyRepo(t, table)
		changed, err := repo.RekeyEventCompanySFID(context.Background(), "e1", oldSFID, newSFID)
		require.NoError(t, err)
		assert.False(t, changed)
		assert.Equal(t, 0, table.updates)
	})

	t.Run("finishes a half-rekeyed event", func(t *testing.T) {
		item := fakeEvent("e1", newSFID, "", "", "")
		item["company_sfid_cla_group_id"] = oldSFID + "#g1"
		table := &fakeEventsTable{items: map[string]map[string]string{"e1": item}}
		repo := newRekeyRepo(t, table)
		changed, err := repo.RekeyEventCompanySFID(context.Background(), "e1", oldSFID, newSFID)
		require.NoError(t, err)
		assert.True(t, changed)
		assert.Equal(t, newSFID+"#g1", table.items["e1"]["company_sfid_cla_group_id"])
	})

	t.Run("refuses an event of another company", func(t *testing.T) {
		table := &fakeEventsTable{items: map[string]map[string]string{"e4": fakeEvent("e4", "0014100000Other00", "f1", "p1", "g1")}}
		repo := newRekeyRepo(t, table)
		changed, err := repo.RekeyEventCompanySFID(context.Background(), "e4", oldSFID, newSFID)
		assert.ErrorIs(t, err, ErrEventCompanySFIDMismatch)
		assert.False(t, changed)
		assert.Equal(t, 0, table.updates)
	})

	t.Run("refuses a composite key of another company", func(t *testing.T) {
		item := fakeEvent("e6", oldSFID, "", "", "")
		item["company_sfid_project_id"] = "0014100000Other00#p1"
		table := &fakeEventsTable{items: map[string]map[string]string{"e6": item}}
		repo := newRekeyRepo(t, table)
		_, err := repo.RekeyEventCompanySFID(context.Background(), "e6", oldSFID, newSFID)
		assert.ErrorIs(t, err, ErrEventCompanySFIDMismatch)
		assert.Equal(t, 0, table.updates)
	})

	t.Run("missing event", func(t *testing.T) {
		repo := newRekeyRepo(t, &fakeEventsTable{items: map[string]map[string]string{}})
		_, err := repo.RekeyEventCompanySFID(context.Background(), "nope", oldSFID, newSFID)
		assert.ErrorIs(t, err, ErrEventNotFound)
	})

	t.Run("concurrent change between read and write is a retryable conflict", func(t *testing.T) {
		table := &fakeEventsTable{items: map[string]map[string]string{"e1": fakeEvent("e1", oldSFID, "f1", "", "")}}
		table.beforeUpdate = func(items map[string]map[string]string) {
			items["e1"]["company_sfid_foundation_sfid"] = oldSFID + "#f2"
		}
		repo := newRekeyRepo(t, table)
		changed, err := repo.RekeyEventCompanySFID(context.Background(), "e1", oldSFID, newSFID)
		assert.ErrorIs(t, err, ErrEventRekeyConflict)
		assert.False(t, changed)
		assert.Equal(t, 1, table.conditionFailures)

		table.beforeUpdate = nil
		changed, err = repo.RekeyEventCompanySFID(context.Background(), "e1", oldSFID, newSFID)
		require.NoError(t, err)
		assert.True(t, changed)
		assert.Equal(t, newSFID+"#f2", table.items["e1"]["company_sfid_foundation_sfid"])
	})

	t.Run("invalid arguments", func(t *testing.T) {
		repo := newRekeyRepo(t, &fakeEventsTable{items: map[string]map[string]string{}})
		_, err := repo.RekeyEventCompanySFID(context.Background(), "e1", oldSFID, oldSFID)
		assert.Error(t, err)
		_, err = repo.RekeyEventCompanySFID(context.Background(), "", oldSFID, newSFID)
		assert.Error(t, err)
	})
}
