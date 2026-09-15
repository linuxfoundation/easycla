// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
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
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	failQueryAt  int
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
}

type fakeCapturedQuery struct {
	indexName      string
	limit          int64
	attributeNames []string
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
	f.mu.Lock()
	f.queries = append(f.queries, captured)
	items := f.items
	failing := f.failQueryAt > 0 && len(f.queries) == f.failQueryAt
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
		if q.indexName == "signature-user-ccla-company-index" {
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
// only compared the best email and the GitHub username exactly
func TestVerifyUserApprovalsGitHubOrgRemovalHonorsRemainingCoverage(t *testing.T) {
	members := []string{"janegh", "otherdev"}
	tests := []struct {
		name        string
		user        *models.User
		approvals   ApprovalList
		invalidated bool
	}{
		{"member without remaining coverage", &models.User{UserID: "user-1", GithubUsername: "janegh", Emails: []string{"jane@corp.example"}}, ApprovalList{}, true},
		{"member listed with another case without remaining coverage", &models.User{UserID: "user-1", GithubUsername: "JaneGH"}, ApprovalList{}, true},
		{"member still covered by a domain entry", &models.User{UserID: "user-1", GithubUsername: "janegh", Emails: []string{"jane@corp.example"}}, ApprovalList{DomainApprovals: []string{"corp.example"}}, false},
		{"member still covered by a secondary email", &models.User{UserID: "user-1", GithubUsername: "janegh", LfEmail: "jane@personal.example", Emails: []string{"jane@corp.example"}}, ApprovalList{EmailApprovals: []string{"jane@corp.example"}}, false},
		{"member still covered by a differently cased email", &models.User{UserID: "user-1", GithubUsername: "janegh", LfEmail: "Jane@Corp.Example"}, ApprovalList{EmailApprovals: []string{"jane@corp.example"}}, false},
		{"member still covered by a differently cased GitHub username", &models.User{UserID: "user-1", GithubUsername: "JaneGH"}, ApprovalList{GitHubUsernameApprovals: []string{"janegh"}}, false},
		{"member still covered by a GitLab username", &models.User{UserID: "user-1", GithubUsername: "janegh", GitlabUsername: "jane.gl"}, ApprovalList{GitlabUsernameApprovals: []string{"jane.gl"}}, false},
		{"not a member of the removed organization", &models.User{UserID: "user-1", GithubUsername: "someone"}, ApprovalList{}, false},
		{"no GitHub username", &models.User{UserID: "user-1"}, ApprovalList{}, false},
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
