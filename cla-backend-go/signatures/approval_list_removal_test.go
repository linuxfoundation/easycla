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
// honors Limit BEFORE filtering (as DynamoDB does) with LastEvaluatedKey pagination, and records
// every Query and UpdateItem it receives.
type fakeSignaturesTable struct {
	mu          sync.Mutex
	items       []map[string]interface{}
	queries     []fakeCapturedQuery
	invalidated map[string]int
	ccla        []string
}

type fakeCapturedQuery struct {
	indexName string
	limit     int64
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
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]fakeAttrValue
	Limit                     *int64
	ExclusiveStartKey         map[string]fakeAttrValue
}

type fakeUpdateRequest struct {
	Key                      map[string]fakeAttrValue
	ExpressionAttributeNames map[string]string
	UpdateExpression         string
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

	f.mu.Lock()
	f.queries = append(f.queries, fakeCapturedQuery{indexName: req.IndexName, limit: aws.Int64Value(req.Limit)})
	items := f.items
	f.mu.Unlock()

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

	// DynamoDB applies Limit to the key-matched items before any filter expression runs
	end := len(keyMatched)
	if req.Limit != nil && start+int(*req.Limit) < end {
		end = start + int(*req.Limit)
	}
	page := keyMatched[start:end]

	respItems := make([]map[string]interface{}, 0, len(page))
	for _, item := range page {
		if fakeItemMatches(item, filterPairs) {
			respItems = append(respItems, item)
		}
	}

	resp := map[string]interface{}{
		"Items":        respItems,
		"Count":        len(respItems),
		"ScannedCount": len(page),
	}
	if end < len(keyMatched) && len(page) > 0 {
		resp["LastEvaluatedKey"] = map[string]interface{}{
			"signature_id": map[string]interface{}{"S": fakeItemString(page[len(page)-1], "signature_id")},
		}
	}
	fakeWriteJSON(w, resp)
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
	if invalidation {
		f.invalidated[signatureID]++
	} else {
		f.ccla = append(f.ccla, req.UpdateExpression)
	}
	f.mu.Unlock()
	fakeWriteJSON(w, map[string]interface{}{})
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
	// acknowledgements are reachable through the employee signature query alone, so a truncated
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

	// all matching employee acknowledgements are invalidated exactly once - including every one
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
