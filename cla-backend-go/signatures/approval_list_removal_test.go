// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/go-openapi/strfmt"
	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	eventsMock "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	githubOrgMock "github.com/linuxfoundation/easycla/cla-backend-go/github_organizations/mock"
	"github.com/linuxfoundation/easycla/cla-backend-go/repositories/mock"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEmployeeIndex is the GSI the employee-signature queries hit.
const fakeEmployeeIndex = "signature-user-ccla-company-index"

// fakeSignaturesTable is a minimal in-memory DynamoDB endpoint for the signatures table. It
// resolves the equality pairs of key condition and filter expressions against wire-format items,
// honors Limit BEFORE filtering (as DynamoDB does) with LastEvaluatedKey pagination, returns only
// the projected attributes, and records every Query, UpdateItem and PutItem it receives.
type fakeSignaturesTable struct {
	mu                sync.Mutex
	items             []map[string]interface{}
	queries           []fakeCapturedQuery
	updates           []fakeUpdateRequest
	puts              []map[string]interface{}
	upserts           []string
	invalidated       map[string]int
	ccla              []string
	conditionFailures int
	// maxRawPage caps the rows a single Query evaluates regardless of Limit (the 1 MB window)
	maxRawPage int
	// failQueryAt makes the n-th Query (1-based) fail with a non-retryable ValidationException
	failQueryAt int
	// failIndex makes every Query against that index fail (an outage isolated to one GSI)
	failIndex    string
	beforeUpdate func(item map[string]interface{})
}

// fakeIndexKeys lists the attributes DynamoDB puts into LastEvaluatedKey for each index
var fakeIndexKeys = map[string][]string{
	"":                              {"signature_id"},
	SignatureProjectIDIndex:         {"signature_project_id", "signature_id"},
	SignatureProjectReferenceIndex:  {"signature_project_id", "signature_reference_id", "signature_id"},
	SignatureReferenceIndex:         {"signature_reference_id", "signature_id"},
	SignatureProjectIDTypeIndex:     {"signature_project_id", "signature_type", "signature_id"},
	SignatureReferenceSearchIndex:   {"signature_reference_id", "signature_reference_name_lower", "signature_id"},
	SignatureProjectDateIDIndex:     {"signature_project_id", "date_modified", "signature_id"},
	"sigtype_signed_approved_index": {"sigtype_signed_approved_id", "signature_id"},
	fakeEmployeeIndex:               {"signature_user_ccla_company_id", "signature_project_id", "signature_id"},
}

type fakeCapturedQuery struct {
	indexName      string
	limit          int64
	attributeNames []string
	// startKey is the signature_id of the ExclusiveStartKey, when the query continues a page
	startKey string
}

type fakeAttrValue struct {
	S    *string
	N    *string
	BOOL *bool
}

type fakeQueryRequest struct {
	IndexName                 string
	KeyConditionExpression    string
	FilterExpression          string
	ProjectionExpression      string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]fakeAttrValue
	Limit                     *int64
	ExclusiveStartKey         map[string]fakeAttrValue
}

type fakePutRequest struct {
	Item map[string]interface{}
}

type fakeUpdateRequest struct {
	Key                       map[string]fakeAttrValue
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]fakeAttrValue
	UpdateExpression          string
	ConditionExpression       string
}

var fakeEqualityPair = regexp.MustCompile(`(#[0-9A-Za-z_]+) = (:[0-9A-Za-z_]+)`)

func (f *fakeSignaturesTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch r.Header.Get("X-Amz-Target") {
	case "DynamoDB_20120810.Query":
		f.handleQuery(w, body)
	case "DynamoDB_20120810.UpdateItem":
		f.handleUpdate(w, body)
	case "DynamoDB_20120810.PutItem":
		f.handlePut(w, body)
	default:
		http.Error(w, "unsupported operation", http.StatusBadRequest)
	}
}

func (f *fakeSignaturesTable) handleQuery(w http.ResponseWriter, body []byte) {
	var req fakeQueryRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	captured := fakeCapturedQuery{indexName: req.IndexName, limit: aws.Int64Value(req.Limit)}
	for _, name := range req.ExpressionAttributeNames {
		captured.attributeNames = append(captured.attributeNames, name)
	}
	if sk, ok := req.ExclusiveStartKey["signature_id"]; ok && sk.S != nil {
		captured.startKey = *sk.S
	}
	f.mu.Lock()
	f.queries = append(f.queries, captured)
	items := f.items
	failing := (f.failQueryAt > 0 && len(f.queries) == f.failQueryAt) || (f.failIndex != "" && req.IndexName == f.failIndex)
	maxRawPage := f.maxRawPage
	f.mu.Unlock()
	if failing {
		fakeWriteError(w, "ValidationException", "injected query failure")
		return
	}

	keyPairs := fakeResolveEqualityPairs(req.KeyConditionExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues)
	filterPairs := fakeResolveEqualityPairs(req.FilterExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues)

	var keyMatched []map[string]interface{}
	for _, item := range items {
		if fakeItemMatches(item, keyPairs) {
			keyMatched = append(keyMatched, item)
		}
	}

	start := 0
	if sk, ok := req.ExclusiveStartKey["signature_id"]; ok && sk.S != nil {
		for i, item := range keyMatched {
			if fakeItemString(item, "signature_id") == *sk.S {
				start = i + 1
				break
			}
		}
	}

	// DynamoDB applies Limit (and the 1 MB window) to the key-matched items before any filter runs
	end := len(keyMatched)
	if req.Limit != nil && start+int(*req.Limit) < end {
		end = start + int(*req.Limit)
	}
	if maxRawPage > 0 && start+maxRawPage < end {
		end = start + maxRawPage
	}
	page := keyMatched[start:end]

	projected := fakeProjectedNames(req.ProjectionExpression, req.ExpressionAttributeNames)
	respItems := make([]map[string]interface{}, 0, len(page))
	for _, item := range page {
		if fakeItemMatches(item, filterPairs) {
			respItems = append(respItems, fakeProjectItem(item, projected))
		}
	}

	resp := map[string]interface{}{
		"Items":        respItems,
		"Count":        len(respItems),
		"ScannedCount": len(page),
	}
	if end < len(keyMatched) && len(page) > 0 {
		lastEvaluatedKey := map[string]interface{}{}
		for _, keyName := range fakeIndexKeys[req.IndexName] {
			if value, ok := page[len(page)-1][keyName]; ok {
				lastEvaluatedKey[keyName] = value
			}
		}
		resp["LastEvaluatedKey"] = lastEvaluatedKey
	}
	fakeWriteJSON(w, resp)
}

var fakeProjectionName = regexp.MustCompile(`#[0-9A-Za-z_]+`)

func fakeProjectedNames(projection string, names map[string]string) map[string]bool {
	if strings.TrimSpace(projection) == "" {
		return nil
	}
	projected := map[string]bool{}
	for _, placeholder := range fakeProjectionName.FindAllString(projection, -1) {
		projected[names[placeholder]] = true
	}
	return projected
}

// a projected read returns only the requested attributes - keys included only when asked for
func fakeProjectItem(item map[string]interface{}, projected map[string]bool) map[string]interface{} {
	if projected == nil {
		return item
	}
	out := map[string]interface{}{}
	for name, value := range item {
		if projected[name] {
			out[name] = value
		}
	}
	return out
}

func (f *fakeSignaturesTable) handlePut(w http.ResponseWriter, body []byte) {
	var req fakePutRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts = append(f.puts, req.Item)
	replaced := false
	for i, candidate := range f.items {
		if fakeItemString(candidate, "signature_id") == fakeItemString(req.Item, "signature_id") {
			f.items[i] = req.Item
			replaced = true
		}
	}
	if !replaced {
		f.items = append(f.items, req.Item)
	}
	fakeWriteJSON(w, map[string]interface{}{})
}

func fakeWriteError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.WriteHeader(http.StatusBadRequest)
	fakeWriteJSON(w, map[string]string{
		"__type":  "com.amazonaws.dynamodb.v20120810#" + code,
		"message": message,
	})
}

func (f *fakeSignaturesTable) handleUpdate(w http.ResponseWriter, body []byte) {
	var req fakeUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	signatureID := ""
	if key, ok := req.Key["signature_id"]; ok && key.S != nil {
		signatureID = *key.S
	}
	invalidation := false
	for _, attrName := range req.ExpressionAttributeNames {
		if attrName == "signature_approved" {
			invalidation = true
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	item := f.find(signatureID)
	if item != nil && f.beforeUpdate != nil {
		// the hook models a concurrent writer; it may also delete the item (see removeLocked)
		f.beforeUpdate(item)
		item = f.find(signatureID)
	}
	f.updates = append(f.updates, req)
	if req.ConditionExpression != "" && !fakeEvalCondition(req.ConditionExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues, item) {
		f.conditionFailures++
		fakeWriteError(w, "ConditionalCheckFailedException", "The conditional request failed")
		return
	}
	if item == nil {
		// an unconditional UpdateItem on a missing key creates the item, as DynamoDB does
		item = map[string]interface{}{"signature_id": fakeS(signatureID)}
		f.items = append(f.items, item)
		f.upserts = append(f.upserts, signatureID)
	}
	fakeApplyUpdate(item, req)
	if invalidation {
		f.invalidated[signatureID]++
	} else {
		f.ccla = append(f.ccla, req.UpdateExpression)
	}
	fakeWriteJSON(w, map[string]interface{}{})
}

// find must be called with f.mu held
func (f *fakeSignaturesTable) find(signatureID string) map[string]interface{} {
	for _, candidate := range f.items {
		if fakeItemString(candidate, "signature_id") == signatureID {
			return candidate
		}
	}
	return nil
}

// removeLocked drops an item; only for use from a beforeUpdate hook, which already holds f.mu
func (f *fakeSignaturesTable) removeLocked(signatureID string) {
	kept := f.items[:0:0]
	for _, candidate := range f.items {
		if fakeItemString(candidate, "signature_id") != signatureID {
			kept = append(kept, candidate)
		}
	}
	f.items = kept
}

func fakeAttrToItem(value fakeAttrValue) map[string]interface{} {
	switch {
	case value.S != nil:
		return map[string]interface{}{"S": *value.S}
	case value.BOOL != nil:
		return map[string]interface{}{"BOOL": *value.BOOL}
	case value.N != nil:
		return map[string]interface{}{"N": *value.N}
	}
	return nil
}

const fakeAttributeExists = "attribute_exists"

var (
	fakeSetAssignment      = regexp.MustCompile(`^(#[0-9A-Za-z_]+) = (:[0-9A-Za-z_]+)$`)
	fakeIfNotExistsAssign  = regexp.MustCompile(`^(#[0-9A-Za-z_]+) = if_not_exists\((#[0-9A-Za-z_]+), (:[0-9A-Za-z_]+)\)$`)
	fakeUpdateClauseSplit  = regexp.MustCompile(`\b(SET|REMOVE|ADD|DELETE)\b`)
	fakeConditionTokenizer = regexp.MustCompile(`\(|\)|,|=|attribute_not_exists|attribute_exists|AND|OR|#[0-9A-Za-z_]+|:[0-9A-Za-z_]+`)
)

// applies the SET (plain and if_not_exists) and REMOVE clauses the production code emits
func fakeApplyUpdate(item map[string]interface{}, req fakeUpdateRequest) {
	expr := strings.TrimSpace(req.UpdateExpression)
	bounds := fakeUpdateClauseSplit.FindAllStringIndex(expr, -1)
	for i, b := range bounds {
		end := len(expr)
		if i+1 < len(bounds) {
			end = bounds[i+1][0]
		}
		keyword := expr[b[0]:b[1]]
		body := strings.TrimSpace(expr[b[1]:end])
		for _, part := range fakeSplitTopLevel(body) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			switch keyword {
			case "SET":
				if m := fakeSetAssignment.FindStringSubmatch(part); m != nil {
					if v := fakeAttrToItem(req.ExpressionAttributeValues[m[2]]); v != nil {
						item[req.ExpressionAttributeNames[m[1]]] = v
					}
				} else if m := fakeIfNotExistsAssign.FindStringSubmatch(part); m != nil {
					name := req.ExpressionAttributeNames[m[1]]
					if _, exists := item[name]; !exists {
						if v := fakeAttrToItem(req.ExpressionAttributeValues[m[3]]); v != nil {
							item[name] = v
						}
					}
				}
			case "REMOVE":
				delete(item, req.ExpressionAttributeNames[part])
			}
		}
	}
}

// splits on commas outside parentheses, so if_not_exists(#a, :b) stays one assignment
func fakeSplitTopLevel(body string) []string {
	var parts []string
	depth, from := 0, 0
	for i, r := range body {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, body[from:i])
				from = i + 1
			}
		}
	}
	return append(parts, body[from:])
}

// evaluates attribute_exists / attribute_not_exists / #name = :value joined by AND, OR and parentheses
func fakeEvalCondition(expr string, names map[string]string, values map[string]fakeAttrValue, item map[string]interface{}) bool {
	tokens := fakeConditionTokenizer.FindAllString(expr, -1)
	pos := 0
	var parseOr func() bool
	peek := func() string {
		if pos < len(tokens) {
			return tokens[pos]
		}
		return ""
	}
	next := func() string {
		tok := peek()
		pos++
		return tok
	}
	parseFactor := func() bool {
		switch tok := next(); tok {
		case "(":
			result := parseOr()
			next()
			return result
		case fakeAttributeExists, "attribute_not_exists":
			next()
			_, exists := item[names[next()]]
			next()
			if tok == fakeAttributeExists {
				return exists
			}
			return !exists
		default:
			next()
			return fakeItemMatches(item, map[string]fakeAttrValue{names[tok]: values[next()]})
		}
	}
	parseAnd := func() bool {
		result := parseFactor()
		for peek() == "AND" {
			next()
			result = parseFactor() && result
		}
		return result
	}
	parseOr = func() bool {
		result := parseAnd()
		for peek() == "OR" {
			next()
			result = parseAnd() || result
		}
		return result
	}
	if item == nil {
		item = map[string]interface{}{}
	}
	return parseOr()
}

func fakeResolveEqualityPairs(expr string, names map[string]string, values map[string]fakeAttrValue) map[string]fakeAttrValue {
	pairs := map[string]fakeAttrValue{}
	for _, match := range fakeEqualityPair.FindAllStringSubmatch(expr, -1) {
		name, nameOK := names[match[1]]
		value, valueOK := values[match[2]]
		if nameOK && valueOK {
			pairs[name] = value
		}
	}
	return pairs
}

func fakeItemMatches(item map[string]interface{}, pairs map[string]fakeAttrValue) bool {
	for name, want := range pairs {
		got, ok := item[name].(map[string]interface{})
		if !ok {
			return false
		}
		switch {
		case want.S != nil:
			if got["S"] != *want.S {
				return false
			}
		case want.BOOL != nil:
			if got["BOOL"] != *want.BOOL {
				return false
			}
		case want.N != nil:
			if got["N"] != *want.N {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func fakeItemString(item map[string]interface{}, name string) string {
	if attr, ok := item[name].(map[string]interface{}); ok {
		if s, ok := attr["S"].(string); ok {
			return s
		}
	}
	return ""
}

func fakeWriteJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func fakeS(value string) map[string]interface{} { return map[string]interface{}{"S": value} }
func fakeTrue() map[string]interface{} {
	return map[string]interface{}{"BOOL": true}
}
func fakeStringList(values ...string) map[string]interface{} {
	list := make([]interface{}, 0, len(values))
	for _, v := range values {
		list = append(list, fakeS(v))
	}
	return map[string]interface{}{"L": list}
}

type recordingEmailSender struct {
	mu         sync.Mutex
	recipients []string
}

func (r *recordingEmailSender) SendEmail(subject, body string, recipients []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recipients = append(r.recipients, recipients...)
	return nil
}

func fakeEclaItem(position int, userEmail string) map[string]interface{} {
	return map[string]interface{}{
		"signature_id":                   fakeS(fmt.Sprintf("sig-%03d", position)),
		"signature_project_id":           fakeS("cla-group-1"),
		"signature_reference_id":         fakeS(fmt.Sprintf("user-%03d", position)),
		"signature_reference_type":       fakeS("user"),
		"signature_reference_name":       fakeS(fmt.Sprintf("User %03d", position)),
		"signature_type":                 fakeS("cla"),
		"signature_user_ccla_company_id": fakeS("company-1"),
		"signature_approved":             fakeTrue(),
		"signature_signed":               fakeTrue(),
		"user_email":                     fakeS(userEmail),
		"date_created":                   fakeS("2023-01-01T00:00:00Z"),
		"date_modified":                  fakeS(fmt.Sprintf("2023-01-01T00:00:%02dZ", position%60)),
	}
}

// TestUpdateApprovalListRemovalInvalidatesAllEmployeeSignatures reproduces #2186: removing an
// email from the Approved List must re-check every employee signature of the company, not just
// the first 10 the old page size allowed through.
func TestUpdateApprovalListRemovalInvalidatesAllEmployeeSignatures(t *testing.T) {
	target := "removed.dev@example.com"

	// positions of the ECLAs signed with the removed email - 12 matches among 25 employee
	// signatures, most of them past the 10th position the old page size truncated at
	matching := map[int]bool{3: true, 7: true}
	for i := 11; i <= 20; i++ {
		matching[i] = true
	}

	items := []map[string]interface{}{{
		"signature_id":             fakeS("ccla-sig"),
		"signature_project_id":     fakeS("cla-group-1"),
		"signature_reference_id":   fakeS("company-1"),
		"signature_reference_type": fakeS("company"),
		"signature_reference_name": fakeS("Acme"),
		"signature_type":           fakeS("ccla"),
		"signature_approved":       fakeTrue(),
		"signature_signed":         fakeTrue(),
		"email_whitelist":          fakeStringList(target, "keep@example.com"),
		"signature_acl":            fakeStringList("manager-lf"),
		"date_created":             fakeS("2023-01-01T00:00:00Z"),
		"date_modified":            fakeS("2023-01-01T00:00:00Z"),
	}}
	var expectedInvalidated []string
	for i := 1; i <= 25; i++ {
		email := fmt.Sprintf("other%03d@example.com", i)
		if matching[i] {
			email = target
			expectedInvalidated = append(expectedInvalidated, fmt.Sprintf("sig-%03d", i))
		}
		items = append(items, fakeEclaItem(i, email))
	}

	table := &fakeSignaturesTable{items: items, invalidated: map[string]int{}}
	server := httptest.NewServer(table)
	defer server.Close()

	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(server.URL),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUsers := mock_users.NewMockUserRepository(ctrl)
	mockUsers.EXPECT().GetUser(gomock.Any()).DoAndReturn(func(userID string) (*models.User, error) {
		return &models.User{UserID: userID, LfEmail: strfmt.Email(target)}, nil
	}).AnyTimes()
	mockUsers.EXPECT().GetUserByUserName("manager-lf", true).
		Return(&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"}, nil).AnyTimes()
	// the users table only knows two of the twelve contributors - the other ten employee
	// acknowledgments are reachable through the employee signature query alone, so a truncated
	// query result cannot be repaired by the per-user fallback
	mockUsers.EXPECT().SearchUsers("user_emails", target, false).
		Return(&models.Users{Users: []models.User{{UserID: "user-003"}, {UserID: "user-011"}}}, nil).AnyTimes()

	mockCompanyRepo := mock_company.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompany(gomock.Any(), "company-1").
		Return(&models.Company{CompanyID: "company-1", CompanyName: "Acme"}, nil).AnyTimes()

	var loggedEvents int64
	mockEvents := eventsMock.NewMockService(ctrl)
	mockEvents.EXPECT().LogEventWithContext(gomock.Any(), gomock.Any()).
		Do(func(context.Context, *events.LogEventArgs) { atomic.AddInt64(&loggedEvents, 1) }).AnyTimes()

	approvalRepo := &fakeApprovalRepo{}

	previousSender := utils.GetEmailSender()
	emailSender := &recordingEmailSender{}
	utils.SetEmailSender(emailSender)
	defer utils.SetEmailSender(previousSender)

	repo := repository{
		stage:              "test",
		dynamoDBClient:     dynamodb.New(awsSession),
		companyRepo:        mockCompanyRepo,
		usersRepo:          mockUsers,
		eventsService:      mockEvents,
		signatureTableName: "cla-test-signatures",
		approvalRepo:       approvalRepo,
	}

	updated, err := repo.UpdateApprovalList(context.Background(),
		&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"},
		&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project", Version: "v2"},
		"company-1",
		&models.ApprovalList{RemoveEmailApprovalList: []string{target}},
		&events.LogEventArgs{EventType: events.InvalidatedSignature})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "ccla-sig", updated.SignatureID)

	table.mu.Lock()
	defer table.mu.Unlock()

	// every employee signature query must ask for the huge page size - the old code paged by 10
	employeeQueries := 0
	for _, q := range table.queries {
		if q.indexName == fakeEmployeeIndex {
			employeeQueries++
			assert.Equal(t, int64(HugePageSize), q.limit, "employee signature queries must not page by 10")
		}
	}
	assert.NotZero(t, employeeQueries)

	// all matching employee acknowledgments are invalidated exactly once - including every one
	// past the 10th match, and none of the non-matching ones
	invalidated := make([]string, 0, len(table.invalidated))
	for signatureID, count := range table.invalidated {
		invalidated = append(invalidated, signatureID)
		assert.Equalf(t, 1, count, "signature %s invalidated more than once", signatureID)
	}
	assert.ElementsMatch(t, expectedInvalidated, invalidated)
	assert.Equal(t, int64(len(expectedInvalidated)), atomic.LoadInt64(&loggedEvents))

	// the CCLA approval list column itself is rewritten once
	require.Len(t, table.ccla, 1)
	assert.Contains(t, table.ccla[0], "#E = :e")

	// the removal is recorded in the approvals table and the removed contributor is notified
	require.Len(t, approvalRepo.added, 1)
	assert.Equal(t, target, approvalRepo.added[0].ApprovalName)
	assert.False(t, approvalRepo.added[0].Active)
	assert.Contains(t, emailSender.recipients, target)
}

// a GitHub organization removal must re-check the member against every remaining Approved List
// criteria (case-insensitively, like the enforcement gate) before invalidating - the old check
// only compared the best email and the GitHub username exactly. Remaining approved organizations
// are honored the way the gate evaluates them: through the member's public organizations, with a
// failed lookup deferring the invalidation instead of guessing
func TestVerifyUserApprovalsGitHubOrgRemovalHonorsRemainingCoverage(t *testing.T) {
	members := []string{"janegh", "otherdev"}
	tests := []struct {
		name         string
		user         *models.User
		approvals    ApprovalList
		publicOrgs   []string
		orgLookupErr error
		lookup       bool
		invalidated  bool
	}{
		{name: "member without remaining coverage", user: &models.User{UserID: "user-1", GithubUsername: "janegh", Emails: []string{"jane@corp.example"}}, invalidated: true},
		{name: "member listed with another case without remaining coverage", user: &models.User{UserID: "user-1", GithubUsername: "JaneGH"}, invalidated: true},
		{name: "member still covered by a domain entry", user: &models.User{UserID: "user-1", GithubUsername: "janegh", Emails: []string{"jane@corp.example"}}, approvals: ApprovalList{DomainApprovals: []string{"corp.example"}}},
		{name: "member still covered by a secondary email", user: &models.User{UserID: "user-1", GithubUsername: "janegh", LfEmail: "jane@personal.example", Emails: []string{"jane@corp.example"}}, approvals: ApprovalList{EmailApprovals: []string{"jane@corp.example"}}},
		{name: "member still covered by a differently cased email", user: &models.User{UserID: "user-1", GithubUsername: "janegh", LfEmail: "Jane@Corp.Example"}, approvals: ApprovalList{EmailApprovals: []string{"jane@corp.example"}}},
		{name: "member still covered by a differently cased GitHub username", user: &models.User{UserID: "user-1", GithubUsername: "JaneGH"}, approvals: ApprovalList{GitHubUsernameApprovals: []string{"janegh"}}},
		{name: "member still covered by a GitLab username", user: &models.User{UserID: "user-1", GithubUsername: "janegh", GitlabUsername: "jane.gl"}, approvals: ApprovalList{GitlabUsernameApprovals: []string{"jane.gl"}}},
		{name: "member still covered by another approved organization", user: &models.User{UserID: "user-1", GithubUsername: "janegh"}, approvals: ApprovalList{GitHubOrgApprovals: []string{"kept-org"}}, publicOrgs: []string{"Unrelated", "Kept-Org"}, lookup: true},
		{name: "member outside every remaining approved organization", user: &models.User{UserID: "user-1", GithubUsername: "janegh"}, approvals: ApprovalList{GitHubOrgApprovals: []string{"kept-org"}}, publicOrgs: []string{"unrelated"}, lookup: true, invalidated: true},
		{name: "member whose organization lookup fails is left alone", user: &models.User{UserID: "user-1", GithubUsername: "janegh"}, approvals: ApprovalList{GitHubOrgApprovals: []string{"kept-org"}}, orgLookupErr: errors.New("github: 503"), lookup: true},
		{name: "list coverage is decided without an organization lookup", user: &models.User{UserID: "user-1", GithubUsername: "janegh", Emails: []string{"jane@corp.example"}}, approvals: ApprovalList{DomainApprovals: []string{"corp.example"}, GitHubOrgApprovals: []string{"kept-org"}}, orgLookupErr: errors.New("must not be called")},
		{name: "not a member of the removed organization", user: &models.User{UserID: "user-1", GithubUsername: "someone"}, approvals: ApprovalList{GitHubOrgApprovals: []string{"kept-org"}}, orgLookupErr: errors.New("must not be called")},
		{name: "no GitHub username", user: &models.User{UserID: "user-1"}, approvals: ApprovalList{GitHubOrgApprovals: []string{"kept-org"}}, orgLookupErr: errors.New("must not be called")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			table := &fakeSignaturesTable{invalidated: map[string]int{}}
			server := httptest.NewServer(table)
			defer server.Close()
			awsSession, err := session.NewSession(&aws.Config{
				Region:      aws.String("us-east-1"),
				Endpoint:    aws.String(server.URL),
				Credentials: credentials.NewStaticCredentials("test", "test", ""),
				DisableSSL:  aws.Bool(true),
			})
			require.NoError(t, err)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockUsers := mock_users.NewMockUserRepository(ctrl)
			mockUsers.EXPECT().GetUser("user-1").Return(test.user, nil)
			lookedUp := stubListUserPublicOrgs(t, test.publicOrgs, test.orgLookupErr)

			repo := repository{
				stage:              "test",
				dynamoDBClient:     dynamodb.New(awsSession),
				usersRepo:          mockUsers,
				signatureTableName: "cla-test-signatures",
			}
			approvals := test.approvals
			approvals.Criteria = utils.GitHubOrgCriteria
			approvals.ApprovalList = []string{"removed-org"}
			approvals.GitHubUsernames = members

			user, invalidated, err := repo.verifyUserApprovals(context.Background(), "user-1", "sig-1", &models.User{LfUsername: "manager-lf"}, &approvals)
			require.NoError(t, err)
			assert.Same(t, test.user, user)
			assert.Equal(t, test.invalidated, invalidated)
			if test.lookup {
				assert.Equal(t, "janegh", *lookedUp, "the public organizations lookup must use the member's GitHub username")
			} else {
				assert.Empty(t, *lookedUp, "no organization lookup expected")
			}

			table.mu.Lock()
			defer table.mu.Unlock()
			expectedWrites := 0
			if test.invalidated {
				expectedWrites = 1
			}
			assert.Equal(t, expectedWrites, table.invalidated["sig-1"])
		})
	}
}

// TestUpdateApprovalListStandaloneGitHubOrgRemovalInvalidatesMembers covers the whole
// organization removal path: before, UpdateApprovalList never loaded the company's employee
// acknowledgments for it, so invalidateSignatures iterated nothing and a standalone organization
// removal left every member approved. Members are re-checked against the remaining criteria and
// the remaining approved organizations; already invalid acknowledgments are not touched again.
func TestUpdateApprovalListStandaloneGitHubOrgRemovalInvalidatesMembers(t *testing.T) {
	// user-001 alice: member, no other coverage           -> invalidated
	// user-002 bob:   member, publicly in kept-org         -> kept
	// user-003 carol: not a member of the removed org      -> kept
	// user-004 dave:  member, acknowledgment already invalid -> not touched again
	// user-005 erin:  member, organizations lookup fails   -> deferred (kept)
	// user-006 frank: member, still on the email list      -> kept
	logins := map[string]string{"user-001": "alice", "user-002": "bob", "user-003": "carol", "user-004": "dave", "user-005": "erin", "user-006": "frank"}
	publicOrgs := map[string][]string{"alice": {"hobby-org"}, "bob": {"Kept-Org"}, "carol": {}, "dave": {}, "frank": {}}

	items := []map[string]interface{}{{
		"signature_id":             fakeS("ccla-sig"),
		"signature_project_id":     fakeS("cla-group-1"),
		"signature_reference_id":   fakeS("company-1"),
		"signature_reference_type": fakeS("company"),
		"signature_reference_name": fakeS("Acme"),
		"signature_type":           fakeS("ccla"),
		"signature_approved":       fakeTrue(),
		"signature_signed":         fakeTrue(),
		"github_org_whitelist":     fakeStringList("removed-org", "kept-org"),
		"email_whitelist":          fakeStringList("frank@corp.example"),
		"signature_acl":            fakeStringList("manager-lf"),
		"date_created":             fakeS("2023-01-01T00:00:00Z"),
		"date_modified":            fakeS("2023-01-01T00:00:00Z"),
	}}
	for i := 1; i <= 6; i++ {
		item := fakeEclaItem(i, fmt.Sprintf("%s@corp.example", logins[fmt.Sprintf("user-%03d", i)]))
		if i == 4 {
			item["signature_approved"] = map[string]interface{}{"BOOL": false}
			item["note"] = fakeS("Signature invalidated (approved set to false) by admin due to Email  removal")
		}
		items = append(items, item)
	}

	table := &fakeSignaturesTable{items: items, invalidated: map[string]int{}}
	server := httptest.NewServer(table)
	defer server.Close()

	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(server.URL),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUsers := mock_users.NewMockUserRepository(ctrl)
	mockUsers.EXPECT().GetUser(gomock.Any()).DoAndReturn(func(userID string) (*models.User, error) {
		login, ok := logins[userID]
		require.Truef(t, ok, "unexpected user lookup: %s", userID)
		return &models.User{UserID: userID, GithubUsername: login, LfEmail: strfmt.Email(login + "@corp.example")}, nil
	}).AnyTimes()
	mockUsers.EXPECT().GetUserByUserName("manager-lf", true).
		Return(&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"}, nil).AnyTimes()

	mockCompanyRepo := mock_company.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompany(gomock.Any(), "company-1").
		Return(&models.Company{CompanyID: "company-1", CompanyName: "Acme"}, nil).AnyTimes()

	mockRepositories := mock.NewMockRepositoryInterface(ctrl)
	mockRepositories.EXPECT().GitHubGetRepositoriesByCLAGroup(gomock.Any(), "cla-group-1", true).
		Return([]*models.GithubRepository{
			{RepositoryID: "repo-1", RepositoryOrganizationName: "removed-org"},
			{RepositoryID: "repo-2", RepositoryOrganizationName: "kept-org"},
		}, nil)
	mockGitHubOrgs := githubOrgMock.NewMockRepositoryInterface(ctrl)
	mockGitHubOrgs.EXPECT().GetGitHubOrganization(gomock.Any(), "removed-org").
		Return(&models.GithubOrganization{OrganizationName: "removed-org", OrganizationInstallationID: 4242}, nil)

	var memberLookups int64
	originalMembers := getOrganizationMembers
	getOrganizationMembers = func(_ context.Context, orgName string, installationID int64) ([]string, error) {
		atomic.AddInt64(&memberLookups, 1)
		assert.Equal(t, "removed-org", orgName)
		assert.Equal(t, int64(4242), installationID)
		return []string{"Alice", "bob", "dave", "erin", "frank", "stranger"}, nil
	}
	defer func() { getOrganizationMembers = originalMembers }()

	var orgLookupMutex sync.Mutex
	var orgLookups []string
	originalPublicOrgs := listUserPublicOrgs
	listUserPublicOrgs = func(_ context.Context, login string) ([]string, error) {
		orgLookupMutex.Lock()
		orgLookups = append(orgLookups, login)
		orgLookupMutex.Unlock()
		if login == "erin" {
			return nil, errors.New("github: 503")
		}
		return publicOrgs[login], nil
	}
	defer func() { listUserPublicOrgs = originalPublicOrgs }()

	var loggedEvents int64
	mockEvents := eventsMock.NewMockService(ctrl)
	mockEvents.EXPECT().LogEventWithContext(gomock.Any(), gomock.Any()).
		Do(func(context.Context, *events.LogEventArgs) { atomic.AddInt64(&loggedEvents, 1) }).AnyTimes()

	approvalRepo := &fakeApprovalRepo{}

	previousSender := utils.GetEmailSender()
	emailSender := &recordingEmailSender{}
	utils.SetEmailSender(emailSender)
	defer utils.SetEmailSender(previousSender)

	repo := repository{
		stage:              "test",
		dynamoDBClient:     dynamodb.New(awsSession),
		companyRepo:        mockCompanyRepo,
		usersRepo:          mockUsers,
		eventsService:      mockEvents,
		repositoriesRepo:   mockRepositories,
		ghOrgRepo:          mockGitHubOrgs,
		signatureTableName: "cla-test-signatures",
		approvalRepo:       approvalRepo,
	}

	updated, err := repo.UpdateApprovalList(context.Background(),
		&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"},
		&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project", Version: "v2"},
		"company-1",
		&models.ApprovalList{RemoveGithubOrgApprovalList: []string{"removed-org"}},
		&events.LogEventArgs{EventType: events.InvalidatedSignature})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "ccla-sig", updated.SignatureID)
	assert.Equal(t, int64(1), atomic.LoadInt64(&memberLookups))

	table.mu.Lock()
	defer table.mu.Unlock()

	// the company's employee acknowledgments are loaded through the same unpaged query the
	// domain removal uses
	employeeQueries := 0
	for _, q := range table.queries {
		if q.indexName == fakeEmployeeIndex {
			employeeQueries++
			assert.Equal(t, int64(HugePageSize), q.limit, "employee signature queries must not page by 10")
		}
	}
	assert.NotZero(t, employeeQueries)

	// only alice loses her acknowledgment - once
	assert.Equal(t, map[string]int{"sig-001": 1}, table.invalidated)
	assert.Equal(t, int64(1), atomic.LoadInt64(&loggedEvents))
	assert.Equal(t, "Signature invalidated (approved set to false) by manager-lf due to GitHub Org Criteria  removal", fakeItemString(table.find("sig-001"), "note"))
	assert.Equal(t, "manager-lf", fakeItemString(table.find("sig-001"), "invalidated_by"))
	assert.Equal(t, "approved list removal (GitHub Org Criteria)", fakeItemString(table.find("sig-001"), "invalidation_reason"))
	// dave's earlier invalidation is untouched
	assert.Equal(t, "Signature invalidated (approved set to false) by admin due to Email  removal", fakeItemString(table.find("sig-004"), "note"))

	// the remaining approved organization is only consulted for members the lists do not cover:
	// carol is not a member, dave is already invalid, frank is on the email list
	sort.Strings(orgLookups)
	assert.Equal(t, []string{"alice", "bob", "erin"}, orgLookups)

	// the CCLA organization column is rewritten once with the remaining entry
	require.Len(t, table.ccla, 1)
	assert.Contains(t, table.ccla[0], "#GHO = :gho")

	// the removal is recorded in the approvals table
	require.Len(t, approvalRepo.added, 1)
	assert.Equal(t, "removed-org", approvalRepo.added[0].ApprovalName)
	assert.False(t, approvalRepo.added[0].Active)
}

// newApprovalRemovalSession points a DynamoDB session at the fake signatures table
func newApprovalRemovalSession(t *testing.T, table *fakeSignaturesTable) (*session.Session, func()) {
	t.Helper()
	server := httptest.NewServer(table)
	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(server.URL),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)
	return awsSession, server.Close
}

// the remaining-organization re-check belongs to every criteria branch of verifyUserApprovals,
// not only to organization removals: whichever entry is removed, a member of another approved
// organization stays approved, an unresolvable membership defers the invalidation, and no
// lookup is made when the lists already decide, when no approved organization remains or when
// the user has no GitHub login to look up
func TestVerifyUserApprovalsRemainingOrgCoverageAcrossCriteria(t *testing.T) {
	jane := func() *models.User {
		return &models.User{UserID: "user-1", GithubUsername: "janegh", GitlabUsername: "jane.gl", LfEmail: "jane@personal.example", Emails: []string{"jane@corp.example"}}
	}
	noLogin := func() *models.User {
		return &models.User{UserID: "user-1", GitlabUsername: "jane.gl", Emails: []string{"jane@corp.example"}}
	}
	mustNotLookup := errors.New("must not be called")
	branches := []struct {
		name     string
		criteria string
		removed  string
	}{
		{name: "domain removal", criteria: utils.EmailDomainCriteria, removed: "corp.example"},
		{name: "email removal", criteria: utils.EmailCriteria, removed: "jane@corp.example"},
		{name: "GitHub username removal", criteria: utils.GitHubUsernameCriteria, removed: "janegh"},
		{name: "GitLab username removal", criteria: utils.GitlabUsernameCriteria, removed: "jane.gl"},
	}
	cases := []struct {
		name         string
		user         *models.User
		approvals    ApprovalList
		publicOrgs   []string
		orgLookupErr error
		lookup       bool
		invalidated  bool
	}{
		{name: "no remaining coverage and no remaining organization", user: jane(), orgLookupErr: mustNotLookup, invalidated: true},
		{name: "covered by another approved organization", user: jane(), approvals: ApprovalList{GitHubOrgApprovals: []string{"kept-org"}}, publicOrgs: []string{"Unrelated", "Kept-Org"}, lookup: true},
		{name: "outside every remaining approved organization", user: jane(), approvals: ApprovalList{GitHubOrgApprovals: []string{"kept-org"}}, publicOrgs: []string{"unrelated"}, lookup: true, invalidated: true},
		{name: "organization lookup fails", user: jane(), approvals: ApprovalList{GitHubOrgApprovals: []string{"kept-org"}}, orgLookupErr: errors.New("github: 503"), lookup: true},
		{name: "list coverage decided without a lookup", user: jane(), approvals: ApprovalList{EmailApprovals: []string{"jane@personal.example"}, GitHubOrgApprovals: []string{"kept-org"}}, orgLookupErr: mustNotLookup},
		{name: "no GitHub login to look up", user: noLogin(), approvals: ApprovalList{GitHubOrgApprovals: []string{"kept-org"}}, orgLookupErr: mustNotLookup, invalidated: true},
	}
	for _, branch := range branches {
		for _, test := range cases {
			t.Run(branch.name+" - "+test.name, func(t *testing.T) {
				table := &fakeSignaturesTable{invalidated: map[string]int{}}
				awsSession, closeServer := newApprovalRemovalSession(t, table)
				defer closeServer()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				mockUsers := mock_users.NewMockUserRepository(ctrl)
				mockUsers.EXPECT().GetUser("user-1").Return(test.user, nil)
				lookedUp := stubListUserPublicOrgs(t, test.publicOrgs, test.orgLookupErr)

				repo := repository{
					stage:              "test",
					dynamoDBClient:     dynamodb.New(awsSession),
					usersRepo:          mockUsers,
					signatureTableName: "cla-test-signatures",
				}
				approvals := test.approvals
				approvals.Criteria = branch.criteria
				approvals.ApprovalList = []string{branch.removed}

				user, invalidated, err := repo.verifyUserApprovals(context.Background(), "user-1", "sig-1", &models.User{LfUsername: "manager-lf"}, &approvals)
				require.NoError(t, err)
				assert.Same(t, test.user, user)
				assert.Equal(t, test.invalidated, invalidated)
				if test.lookup {
					assert.Equal(t, "janegh", *lookedUp, "the public organizations lookup must use the user's GitHub username")
				} else {
					assert.Empty(t, *lookedUp, "no organization lookup expected")
				}

				table.mu.Lock()
				defer table.mu.Unlock()
				expectedWrites := 0
				if test.invalidated {
					expectedWrites = 1
					assert.Equal(t, fmt.Sprintf("Signature invalidated (approved set to false) by manager-lf due to %s  removal", branch.criteria), fakeItemString(table.find("sig-1"), "note"))
				}
				assert.Equal(t, expectedWrites, table.invalidated["sig-1"])
			})
		}
	}
}

// the acknowledgments an organization removal re-checks are the company's approved and signed
// ones for the CLA group - across every raw page of the employee index, without the unsigned,
// the already invalid, or another company's or CLA group's records
func TestApprovedEmployeeSignaturesLoaderBoundaries(t *testing.T) {
	var items []map[string]interface{}
	var expected []string
	for i := 1; i <= 7; i++ {
		items = append(items, fakeEclaItem(i, fmt.Sprintf("dev%03d@corp.example", i)))
		expected = append(expected, fmt.Sprintf("sig-%03d", i))
	}
	unsigned := fakeEclaItem(8, "unsigned@corp.example")
	unsigned["signature_signed"] = map[string]interface{}{"BOOL": false}
	invalid := fakeEclaItem(9, "invalid@corp.example")
	invalid["signature_approved"] = map[string]interface{}{"BOOL": false}
	otherCompany := fakeEclaItem(10, "other-company@corp.example")
	otherCompany["signature_user_ccla_company_id"] = fakeS("company-2")
	otherProject := fakeEclaItem(11, "other-project@corp.example")
	otherProject["signature_project_id"] = fakeS("cla-group-2")
	items = append(items, unsigned, invalid, otherCompany, otherProject)

	// raw pages of three rows: the nine key-matched rows come back in four pages, the last
	// approved one on the third
	table := &fakeSignaturesTable{items: items, maxRawPage: 3}
	awsSession, closeServer := newApprovalRemovalSession(t, table)
	defer closeServer()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockUsers := mock_users.NewMockUserRepository(ctrl)
	mockUsers.EXPECT().GetUser(gomock.Any()).DoAndReturn(func(userID string) (*models.User, error) {
		return &models.User{UserID: userID}, nil
	}).AnyTimes()
	mockCompanyRepo := mock_company.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompany(gomock.Any(), "company-1").
		Return(&models.Company{CompanyID: "company-1", CompanyName: "Acme"}, nil).AnyTimes()
	repo := repository{stage: "test", dynamoDBClient: dynamodb.New(awsSession), usersRepo: mockUsers, companyRepo: mockCompanyRepo, signatureTableName: "cla-test-signatures"}

	eclas, err := repo.approvedEmployeeSignatures(context.Background(), "cla-group-1", "company-1")
	require.NoError(t, err)
	var ids []string
	for _, ecla := range eclas {
		ids = append(ids, ecla.SignatureID)
	}
	sort.Strings(ids)
	assert.Equal(t, expected, ids)

	table.mu.Lock()
	defer table.mu.Unlock()
	continued := 0
	for _, q := range table.queries {
		if q.indexName == fakeEmployeeIndex && q.startKey != "" {
			continued++
		}
	}
	assert.GreaterOrEqual(t, continued, 3, "the loader must follow the employee index continuation keys")
}

// a lookup the organization removal depends on failing must fail the request before anything
// is written - in particular when the removed organization is the last one, where the column
// removal is the first write of the branch: a success there with a failure afterwards would
// leave the criterion gone while every member stays approved, and the failure isolated to the
// employee index must not turn into a successful empty sweep
func TestUpdateApprovalListGitHubOrgRemovalFailsBeforeWritingWhenLookupsFail(t *testing.T) {
	cases := []struct {
		name          string
		orgs          []string
		failIndex     string
		reposErr      error
		orgRecordErr  error
		membersErr    error
		expectedError string
	}{
		{name: "employee acknowledgments cannot be loaded", orgs: []string{"removed-org", "kept-org"}, failIndex: fakeEmployeeIndex, expectedError: "unable to load the employee acknowledgments"},
		{name: "last organization - employee acknowledgments cannot be loaded", orgs: []string{"removed-org"}, failIndex: fakeEmployeeIndex, expectedError: "unable to load the employee acknowledgments"},
		{name: "last organization - member lookup fails", orgs: []string{"removed-org"}, membersErr: errors.New("github: 502"), expectedError: "unable to fetch github organization users for org: removed-org"},
		{name: "last organization - organization record lookup fails", orgs: []string{"removed-org"}, orgRecordErr: errors.New("dynamodb: unavailable"), expectedError: "unable to get gh org by name: removed-org"},
		{name: "last organization - repositories lookup fails", orgs: []string{"removed-org"}, reposErr: errors.New("dynamodb: unavailable"), expectedError: "unable to fetch repositories for cla group ID: cla-group-1"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			items := []map[string]interface{}{{
				"signature_id":             fakeS("ccla-sig"),
				"signature_project_id":     fakeS("cla-group-1"),
				"signature_reference_id":   fakeS("company-1"),
				"signature_reference_type": fakeS("company"),
				"signature_reference_name": fakeS("Acme"),
				"signature_type":           fakeS("ccla"),
				"signature_approved":       fakeTrue(),
				"signature_signed":         fakeTrue(),
				"github_org_whitelist":     fakeStringList(test.orgs...),
				"signature_acl":            fakeStringList("manager-lf"),
				"date_created":             fakeS("2023-01-01T00:00:00Z"),
				"date_modified":            fakeS("2023-01-01T00:00:00Z"),
			}, fakeEclaItem(1, "alice@corp.example")}

			table := &fakeSignaturesTable{items: items, invalidated: map[string]int{}, failIndex: test.failIndex}
			awsSession, closeServer := newApprovalRemovalSession(t, table)
			defer closeServer()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockUsers := mock_users.NewMockUserRepository(ctrl)
			mockUsers.EXPECT().GetUserByUserName("manager-lf", true).
				Return(&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"}, nil).AnyTimes()
			mockCompanyRepo := mock_company.NewMockIRepository(ctrl)
			mockCompanyRepo.EXPECT().GetCompany(gomock.Any(), "company-1").
				Return(&models.Company{CompanyID: "company-1", CompanyName: "Acme"}, nil).AnyTimes()
			mockRepositories := mock.NewMockRepositoryInterface(ctrl)
			mockRepositories.EXPECT().GitHubGetRepositoriesByCLAGroup(gomock.Any(), "cla-group-1", true).
				Return([]*models.GithubRepository{{RepositoryID: "repo-1", RepositoryOrganizationName: "removed-org"}}, test.reposErr)
			mockGitHubOrgs := githubOrgMock.NewMockRepositoryInterface(ctrl)
			mockGitHubOrgs.EXPECT().GetGitHubOrganization(gomock.Any(), "removed-org").
				Return(&models.GithubOrganization{OrganizationName: "removed-org", OrganizationInstallationID: 4242}, test.orgRecordErr).AnyTimes()
			originalMembers := getOrganizationMembers
			getOrganizationMembers = func(context.Context, string, int64) ([]string, error) { return []string{"alice"}, test.membersErr }
			defer func() { getOrganizationMembers = originalMembers }()
			stubListUserPublicOrgs(t, nil, errors.New("must not be called"))
			mockEvents := eventsMock.NewMockService(ctrl)
			approvalRepo := &fakeApprovalRepo{}

			repo := repository{
				stage:              "test",
				dynamoDBClient:     dynamodb.New(awsSession),
				companyRepo:        mockCompanyRepo,
				usersRepo:          mockUsers,
				eventsService:      mockEvents,
				repositoriesRepo:   mockRepositories,
				ghOrgRepo:          mockGitHubOrgs,
				signatureTableName: "cla-test-signatures",
				approvalRepo:       approvalRepo,
			}

			updated, err := repo.UpdateApprovalList(context.Background(),
				&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"},
				&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project", Version: "v2"},
				"company-1",
				&models.ApprovalList{RemoveGithubOrgApprovalList: []string{"removed-org"}},
				&events.LogEventArgs{EventType: events.InvalidatedSignature})
			require.Error(t, err)
			assert.Nil(t, updated)
			assert.Contains(t, err.Error(), test.expectedError)

			table.mu.Lock()
			defer table.mu.Unlock()
			assert.Empty(t, table.updates, "no write of any kind before every lookup the removal depends on has succeeded")
			assert.Empty(t, table.invalidated)
			assert.Empty(t, table.ccla, "the organization column is neither rewritten nor removed")
			assert.Empty(t, approvalRepo.added, "the removal is not recorded either")
			assert.Empty(t, approvalRepo.updated)
		})
	}
}

// the approval list carries the organization as the CLA manager typed it while the repositories
// table carries GitHub's canonical casing; the gate approves members case-insensitively, so the
// removal has to find the organization the same way or nobody is re-checked
func TestUpdateApprovalListGitHubOrgRemovalMatchesTheRepositoryOrganizationCaseInsensitively(t *testing.T) {
	items := []map[string]interface{}{{
		"signature_id":             fakeS("ccla-sig"),
		"signature_project_id":     fakeS("cla-group-1"),
		"signature_reference_id":   fakeS("company-1"),
		"signature_reference_type": fakeS("company"),
		"signature_reference_name": fakeS("Acme"),
		"signature_type":           fakeS("ccla"),
		"signature_approved":       fakeTrue(),
		"signature_signed":         fakeTrue(),
		"github_org_whitelist":     fakeStringList("removed-org", "kept-org"),
		"signature_acl":            fakeStringList("manager-lf"),
		"date_created":             fakeS("2023-01-01T00:00:00Z"),
		"date_modified":            fakeS("2023-01-01T00:00:00Z"),
	}, fakeEclaItem(1, "alice@corp.example")}

	table := &fakeSignaturesTable{items: items, invalidated: map[string]int{}}
	awsSession, closeServer := newApprovalRemovalSession(t, table)
	defer closeServer()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockUsers := mock_users.NewMockUserRepository(ctrl)
	mockUsers.EXPECT().GetUser("user-001").
		Return(&models.User{UserID: "user-001", GithubUsername: "alice", LfEmail: "alice@corp.example"}, nil).AnyTimes()
	mockUsers.EXPECT().GetUserByUserName("manager-lf", true).
		Return(&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"}, nil).AnyTimes()
	mockCompanyRepo := mock_company.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompany(gomock.Any(), "company-1").
		Return(&models.Company{CompanyID: "company-1", CompanyName: "Acme"}, nil).AnyTimes()
	mockRepositories := mock.NewMockRepositoryInterface(ctrl)
	mockRepositories.EXPECT().GitHubGetRepositoriesByCLAGroup(gomock.Any(), "cla-group-1", true).
		Return([]*models.GithubRepository{
			{RepositoryID: "repo-1", RepositoryOrganizationName: "Removed-Org"},
			{RepositoryID: "repo-2", RepositoryOrganizationName: "Kept-Org"},
		}, nil)
	// the organization record is looked up under the repositories table's canonical name
	mockGitHubOrgs := githubOrgMock.NewMockRepositoryInterface(ctrl)
	mockGitHubOrgs.EXPECT().GetGitHubOrganization(gomock.Any(), "Removed-Org").
		Return(&models.GithubOrganization{OrganizationName: "Removed-Org", OrganizationInstallationID: 4242}, nil)
	originalMembers := getOrganizationMembers
	getOrganizationMembers = func(_ context.Context, orgName string, _ int64) ([]string, error) {
		assert.Equal(t, "Removed-Org", orgName)
		return []string{"alice"}, nil
	}
	defer func() { getOrganizationMembers = originalMembers }()
	stubListUserPublicOrgs(t, []string{"hobby-org"}, nil)
	mockEvents := eventsMock.NewMockService(ctrl)
	mockEvents.EXPECT().LogEventWithContext(gomock.Any(), gomock.Any()).Times(1)
	approvalRepo := &fakeApprovalRepo{}
	previousSender := utils.GetEmailSender()
	utils.SetEmailSender(&recordingEmailSender{})
	defer utils.SetEmailSender(previousSender)

	repo := repository{
		stage:              "test",
		dynamoDBClient:     dynamodb.New(awsSession),
		companyRepo:        mockCompanyRepo,
		usersRepo:          mockUsers,
		eventsService:      mockEvents,
		repositoriesRepo:   mockRepositories,
		ghOrgRepo:          mockGitHubOrgs,
		signatureTableName: "cla-test-signatures",
		approvalRepo:       approvalRepo,
	}

	updated, err := repo.UpdateApprovalList(context.Background(),
		&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"},
		&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project", Version: "v2"},
		"company-1",
		&models.ApprovalList{RemoveGithubOrgApprovalList: []string{"removed-org"}},
		&events.LogEventArgs{EventType: events.InvalidatedSignature})
	require.NoError(t, err)
	require.NotNil(t, updated)

	table.mu.Lock()
	defer table.mu.Unlock()
	assert.Equal(t, map[string]int{"sig-001": 1}, table.invalidated, "the member of the differently-cased organization loses the acknowledgment")
	assert.Equal(t, "Signature invalidated (approved set to false) by manager-lf due to GitHub Org Criteria  removal", fakeItemString(table.find("sig-001"), "note"))
	require.Len(t, table.ccla, 1)
	assert.Contains(t, table.ccla[0], "#GHO = :gho")
	require.Len(t, approvalRepo.added, 1)
	assert.Equal(t, "removed-org", approvalRepo.added[0].ApprovalName)
	assert.False(t, approvalRepo.added[0].Active)
}

// an add-only organization request resolves none of the removal dependencies - whether the
// removal list is omitted or decoded as an explicitly empty array - so an outage isolated to
// the employee acknowledgment index cannot block the addition.
func TestUpdateApprovalListGitHubOrgAddOnlyRequestSkipsTheRemovalLookups(t *testing.T) {
	cases := []struct {
		name   string
		remove []string
	}{
		{name: "removal list omitted", remove: nil},
		{name: "removal list explicitly empty", remove: []string{}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			items := []map[string]interface{}{{
				"signature_id":             fakeS("ccla-sig"),
				"signature_project_id":     fakeS("cla-group-1"),
				"signature_reference_id":   fakeS("company-1"),
				"signature_reference_type": fakeS("company"),
				"signature_reference_name": fakeS("Acme"),
				"signature_type":           fakeS("ccla"),
				"signature_approved":       fakeTrue(),
				"signature_signed":         fakeTrue(),
				"github_org_whitelist":     fakeStringList("kept-org"),
				"signature_acl":            fakeStringList("manager-lf"),
				"date_created":             fakeS("2023-01-01T00:00:00Z"),
				"date_modified":            fakeS("2023-01-01T00:00:00Z"),
			}, fakeEclaItem(1, "alice@corp.example")}

			// the employee acknowledgment index is down: only a removal sweep would notice
			table := &fakeSignaturesTable{items: items, invalidated: map[string]int{}, failIndex: fakeEmployeeIndex}
			awsSession, closeServer := newApprovalRemovalSession(t, table)
			defer closeServer()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockUsers := mock_users.NewMockUserRepository(ctrl)
			mockUsers.EXPECT().GetUserByUserName("manager-lf", true).
				Return(&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"}, nil).AnyTimes()
			mockCompanyRepo := mock_company.NewMockIRepository(ctrl)
			mockCompanyRepo.EXPECT().GetCompany(gomock.Any(), "company-1").
				Return(&models.Company{CompanyID: "company-1", CompanyName: "Acme"}, nil).AnyTimes()
			// no expectations: any repository or organization record lookup fails the test
			mockRepositories := mock.NewMockRepositoryInterface(ctrl)
			mockGitHubOrgs := githubOrgMock.NewMockRepositoryInterface(ctrl)
			originalMembers := getOrganizationMembers
			getOrganizationMembers = func(_ context.Context, orgName string, _ int64) ([]string, error) {
				t.Errorf("unexpected member lookup for organization: %s", orgName)
				return nil, errors.New("must not be called")
			}
			defer func() { getOrganizationMembers = originalMembers }()
			publicOrgLookup := stubListUserPublicOrgs(t, nil, errors.New("must not be called"))
			mockEvents := eventsMock.NewMockService(ctrl)
			approvalRepo := &fakeApprovalRepo{}

			repo := repository{
				stage:              "test",
				dynamoDBClient:     dynamodb.New(awsSession),
				companyRepo:        mockCompanyRepo,
				usersRepo:          mockUsers,
				eventsService:      mockEvents,
				repositoriesRepo:   mockRepositories,
				ghOrgRepo:          mockGitHubOrgs,
				signatureTableName: "cla-test-signatures",
				approvalRepo:       approvalRepo,
			}

			updated, err := repo.UpdateApprovalList(context.Background(),
				&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"},
				&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project", Version: "v2"},
				"company-1",
				&models.ApprovalList{AddGithubOrgApprovalList: []string{"new-org"}, RemoveGithubOrgApprovalList: test.remove},
				&events.LogEventArgs{EventType: events.InvalidatedSignature})
			require.NoError(t, err)
			require.NotNil(t, updated)
			assert.Equal(t, "", *publicOrgLookup, "no public organization lookup for anybody")

			table.mu.Lock()
			defer table.mu.Unlock()
			for _, q := range table.queries {
				assert.NotEqual(t, fakeEmployeeIndex, q.indexName, "no employee acknowledgment sweep for an add-only request")
			}
			assert.Empty(t, table.invalidated)
			require.Len(t, table.ccla, 1, "the organization column is rewritten once with the addition")
			assert.Contains(t, table.ccla[0], "#GHO = :gho")
			assert.Len(t, table.updates, 1, "the CCLA rewrite is the only write")
			require.Len(t, approvalRepo.added, 1, "the addition is the only approval record")
			assert.Equal(t, utils.GithubOrgApprovalCriteria, approvalRepo.added[0].ApprovalCriteria)
			assert.Equal(t, "new-org", approvalRepo.added[0].ApprovalName)
			assert.True(t, approvalRepo.added[0].Active)
			assert.Empty(t, approvalRepo.updated)
		})
	}
}

// the coverage re-check works on the approval lists as they will look after the whole request:
// an organization added by the same request already covers its members while an organization
// removed by it no longer does - whichever branch (here an email removal) triggers the re-check.
// Members losing their acknowledgment to the email removal are not re-invalidated by the
// organization removal that follows in the same request.
func TestUpdateApprovalListPendingOrganizationChangesDriveTheRemainingCoverage(t *testing.T) {
	// user-001 gina: email removed, publicly in the organization being added       -> kept
	// user-002 hank: email removed, publicly only in the organization being removed -> invalidated (email)
	// user-003 ivan: member of the removed organization, no other coverage           -> invalidated (org)
	// user-004 jack: member of the removed organization, still on the email list      -> kept
	logins := map[string]string{"user-001": "gina", "user-002": "hank", "user-003": "ivan", "user-004": "jack"}
	emails := map[string]string{"user-001": "gina@corp.example", "user-002": "hank@corp.example", "user-003": "ivan@corp.example", "user-004": "keep@corp.example"}
	publicOrgs := map[string][]string{"gina": {"New-Org"}, "hank": {"old-org"}, "ivan": {}}

	items := []map[string]interface{}{{
		"signature_id":             fakeS("ccla-sig"),
		"signature_project_id":     fakeS("cla-group-1"),
		"signature_reference_id":   fakeS("company-1"),
		"signature_reference_type": fakeS("company"),
		"signature_reference_name": fakeS("Acme"),
		"signature_type":           fakeS("ccla"),
		"signature_approved":       fakeTrue(),
		"signature_signed":         fakeTrue(),
		"email_whitelist":          fakeStringList("gina@corp.example", "hank@corp.example", "keep@corp.example"),
		"github_org_whitelist":     fakeStringList("old-org"),
		"signature_acl":            fakeStringList("manager-lf"),
		"date_created":             fakeS("2023-01-01T00:00:00Z"),
		"date_modified":            fakeS("2023-01-01T00:00:00Z"),
	}}
	for i := 1; i <= 4; i++ {
		items = append(items, fakeEclaItem(i, emails[fmt.Sprintf("user-%03d", i)]))
	}

	table := &fakeSignaturesTable{items: items, invalidated: map[string]int{}}
	awsSession, closeServer := newApprovalRemovalSession(t, table)
	defer closeServer()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUsers := mock_users.NewMockUserRepository(ctrl)
	mockUsers.EXPECT().GetUser(gomock.Any()).DoAndReturn(func(userID string) (*models.User, error) {
		login, ok := logins[userID]
		require.Truef(t, ok, "unexpected user lookup: %s", userID)
		return &models.User{UserID: userID, GithubUsername: login, LfEmail: strfmt.Email(emails[userID])}, nil
	}).AnyTimes()
	mockUsers.EXPECT().GetUserByUserName("manager-lf", true).
		Return(&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"}, nil).AnyTimes()
	mockUsers.EXPECT().SearchUsers("user_emails", "gina@corp.example", false).
		Return(&models.Users{Users: []models.User{{UserID: "user-001"}}}, nil)
	mockUsers.EXPECT().SearchUsers("user_emails", "hank@corp.example", false).
		Return(&models.Users{Users: []models.User{{UserID: "user-002"}}}, nil)

	mockCompanyRepo := mock_company.NewMockIRepository(ctrl)
	mockCompanyRepo.EXPECT().GetCompany(gomock.Any(), "company-1").
		Return(&models.Company{CompanyID: "company-1", CompanyName: "Acme"}, nil).AnyTimes()

	mockRepositories := mock.NewMockRepositoryInterface(ctrl)
	mockRepositories.EXPECT().GitHubGetRepositoriesByCLAGroup(gomock.Any(), "cla-group-1", true).
		Return([]*models.GithubRepository{{RepositoryID: "repo-1", RepositoryOrganizationName: "old-org"}}, nil)
	mockGitHubOrgs := githubOrgMock.NewMockRepositoryInterface(ctrl)
	mockGitHubOrgs.EXPECT().GetGitHubOrganization(gomock.Any(), "old-org").
		Return(&models.GithubOrganization{OrganizationName: "old-org", OrganizationInstallationID: 4242}, nil)

	originalMembers := getOrganizationMembers
	getOrganizationMembers = func(_ context.Context, orgName string, _ int64) ([]string, error) {
		assert.Equal(t, "old-org", orgName)
		return []string{"gina", "hank", "ivan", "jack"}, nil
	}
	defer func() { getOrganizationMembers = originalMembers }()

	var orgLookupMutex sync.Mutex
	var orgLookups []string
	originalPublicOrgs := listUserPublicOrgs
	listUserPublicOrgs = func(_ context.Context, login string) ([]string, error) {
		orgLookupMutex.Lock()
		orgLookups = append(orgLookups, login)
		orgLookupMutex.Unlock()
		orgs, ok := publicOrgs[login]
		require.Truef(t, ok, "unexpected organization lookup for: %s", login)
		return orgs, nil
	}
	defer func() { listUserPublicOrgs = originalPublicOrgs }()

	var loggedEvents int64
	mockEvents := eventsMock.NewMockService(ctrl)
	mockEvents.EXPECT().LogEventWithContext(gomock.Any(), gomock.Any()).
		Do(func(context.Context, *events.LogEventArgs) { atomic.AddInt64(&loggedEvents, 1) }).AnyTimes()

	approvalRepo := &fakeApprovalRepo{}
	previousSender := utils.GetEmailSender()
	emailSender := &recordingEmailSender{}
	utils.SetEmailSender(emailSender)
	defer utils.SetEmailSender(previousSender)

	repo := repository{
		stage:              "test",
		dynamoDBClient:     dynamodb.New(awsSession),
		companyRepo:        mockCompanyRepo,
		usersRepo:          mockUsers,
		eventsService:      mockEvents,
		repositoriesRepo:   mockRepositories,
		ghOrgRepo:          mockGitHubOrgs,
		signatureTableName: "cla-test-signatures",
		approvalRepo:       approvalRepo,
	}

	updated, err := repo.UpdateApprovalList(context.Background(),
		&models.User{LfUsername: "manager-lf", LfEmail: "manager@example.com"},
		&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project", Version: "v2"},
		"company-1",
		&models.ApprovalList{
			RemoveEmailApprovalList:     []string{"gina@corp.example", "hank@corp.example"},
			AddGithubOrgApprovalList:    []string{"new-org"},
			RemoveGithubOrgApprovalList: []string{"old-org"},
		},
		&events.LogEventArgs{EventType: events.InvalidatedSignature})
	require.NoError(t, err)
	require.NotNil(t, updated)

	table.mu.Lock()
	defer table.mu.Unlock()

	assert.Equal(t, map[string]int{"sig-002": 1, "sig-003": 1}, table.invalidated)
	assert.Equal(t, int64(2), atomic.LoadInt64(&loggedEvents))
	assert.Equal(t, "Signature invalidated (approved set to false) by manager-lf due to Email Criteria  removal", fakeItemString(table.find("sig-002"), "note"))
	assert.Equal(t, "Signature invalidated (approved set to false) by manager-lf due to GitHub Org Criteria  removal", fakeItemString(table.find("sig-003"), "note"))
	assert.Equal(t, "", fakeItemString(table.find("sig-001"), "note"), "gina keeps her acknowledgment through the organization being added")
	assert.Equal(t, "", fakeItemString(table.find("sig-004"), "note"), "jack keeps his acknowledgment through the remaining email entry")

	// gina is re-checked by the email removal and again as a member of the removed organization,
	// hank once by the email removal (his acknowledgment is already invalid when the organization
	// branch runs), ivan once by the organization removal, jack never (the email list covers him)
	sort.Strings(orgLookups)
	assert.Equal(t, []string{"gina", "gina", "hank", "ivan"}, orgLookups)

	// one CCLA rewrite carries both list changes
	require.Len(t, table.ccla, 1)
	assert.Contains(t, table.ccla[0], "#E = :e")
	assert.Contains(t, table.ccla[0], "#GHO = :gho")

	// the approvals table records the two email removals, the organization addition and removal
	recorded := map[string]bool{}
	for _, added := range approvalRepo.added {
		recorded[added.ApprovalCriteria+"/"+added.ApprovalName] = added.Active
	}
	assert.Equal(t, map[string]bool{
		utils.EmailApprovalCriteria + "/gina@corp.example": false,
		utils.EmailApprovalCriteria + "/hank@corp.example": false,
		utils.GithubOrgApprovalCriteria + "/new-org":       true,
		utils.GithubOrgApprovalCriteria + "/old-org":       false,
	}, recorded)

	// only the contributor invalidated by the email removal is notified about it
	assert.Contains(t, emailSender.recipients, "hank@corp.example")
	assert.NotContains(t, emailSender.recipients, "gina@corp.example")
}
