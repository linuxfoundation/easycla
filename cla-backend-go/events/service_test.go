// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	user_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	eventActorTableName   = "cla-test-events"
	eventActorQueryTarget = "DynamoDB_20120810.Query"
)

type eventActorLookup struct {
	CombinedRepo
	user *models.User
	err  error
}

func (r *eventActorLookup) GetUserByUserName(string, bool) (*models.User, error) {
	return r.user, r.err
}

func (r *eventActorLookup) GetUser(string) (*models.User, error) {
	return r.user, r.err
}

type eventActorTransport struct {
	mu          sync.Mutex
	authReady   chan struct{}
	authOnce    sync.Once
	profileName string
	profileDown bool
	items       []map[string]*dynamodb.AttributeValue
}

func (tr *eventActorTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	var payload interface{}
	switch r.URL.Host {
	case "auth.events.invalid":
		defer tr.authOnce.Do(func() { close(tr.authReady) })
		payload = map[string]interface{}{"access_token": "test-events-token", "token_type": "Bearer", "expires_in": 86400}
	case "users.events.invalid":
		if r.Method != http.MethodGet || r.URL.Path != "/user-service/v1/users" || r.URL.Query().Get("username") != "lf-manager" {
			return nil, fmt.Errorf("unexpected user lookup: %s %s", r.Method, r.URL)
		}
		if tr.profileDown {
			return nil, errors.New("user-service unavailable")
		}
		payload = map[string]interface{}{"Data": []map[string]string{{"Username": "lf-manager", "Name": tr.profileName}}}
	case "dynamodb.events.invalid":
		switch r.Header.Get("X-Amz-Target") {
		case "DynamoDB_20120810.PutItem":
			var input dynamodb.PutItemInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				return nil, err
			}
			if aws.StringValue(input.TableName) != eventActorTableName {
				return nil, fmt.Errorf("unexpected table write: %s", aws.StringValue(input.TableName))
			}
			tr.items = append(tr.items, input.Item)
			payload = struct{}{}
		case eventActorQueryTarget:
			var input dynamodb.QueryInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				return nil, err
			}
			if aws.StringValue(input.TableName) != eventActorTableName ||
				aws.StringValue(input.IndexName) != EventFoundationSFIDEpochIndex {
				return nil, fmt.Errorf("unexpected company activity query: %+v", input)
			}
			payload = &dynamodb.QueryOutput{Items: tr.items}
		default:
			return nil, fmt.Errorf("unexpected DynamoDB operation: %s", r.Header.Get("X-Amz-Target"))
		}
	default:
		return nil, fmt.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Request:    r,
	}, nil
}

func TestLogEventWithLFXActor(t *testing.T) {
	tr := &eventActorTransport{authReady: make(chan struct{})}
	oldTransport, oldClient := http.DefaultTransport, http.DefaultClient
	http.DefaultTransport = tr
	http.DefaultClient = &http.Client{Transport: tr}
	t.Cleanup(func() {
		http.DefaultTransport, http.DefaultClient = oldTransport, oldClient
	})
	token.Init("test-client", "test-secret", "https://auth.events.invalid/oauth/token", "test-audience")
	select {
	case <-tr.authReady:
	case <-time.After(5 * time.Second):
		t.Fatal("test token initialization did not run")
	}
	_, err := token.GetToken()
	require.NoError(t, err)
	user_service.InitClient("https://users.events.invalid", "test-key")

	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String("https://dynamodb.events.invalid"),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		HTTPClient:  &http.Client{Transport: tr},
		MaxRetries:  aws.Int(0),
	})
	require.NoError(t, err)
	repo := NewRepository(awsSession, "test")
	manager := &models.User{UserID: "easycla-manager-id", LfUsername: "lf-manager", Username: "Stored Manager"}
	tests := []struct {
		name         string
		user         *models.User
		providedUser bool
		lookupErr    error
		profileName  string
		profileDown  bool
		callerName   string
		wantName     string
	}{
		{name: "LFX-only actor", profileName: "Manager Display", wantName: "Manager Display"},
		{name: "LFX-only actor without profile", profileDown: true, wantName: "lf-manager"},
		{name: "LFX-only actor with caller name", profileDown: true, callerName: "Caller Display", wantName: "Caller Display"},
		{name: "user lookup error preserves actor", lookupErr: errors.New("user lookup unavailable"), profileName: "Manager Display", wantName: "Manager Display"},
		{name: "existing user", user: manager, profileName: "Manager Display", wantName: "Manager Display"},
		{name: "caller-provided user uses same display name", user: manager, providedUser: true, profileName: "Manager Display", wantName: "Manager Display"},
		{name: "existing user without profile", user: manager, profileDown: true, wantName: "Stored Manager"},
		{name: "empty profile name preserves stored name", user: manager, wantName: "Stored Manager"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr.mu.Lock()
			tr.profileName, tr.profileDown, tr.items = tt.profileName, tt.profileDown, nil
			tr.mu.Unlock()
			lookup := &eventActorLookup{user: tt.user, err: tt.lookupErr}
			service := NewService(repo, lookup)
			args := &LogEventArgs{
				EventType:         InvalidatedSignature,
				LfUsername:        "lf-manager",
				UserName:          tt.callerName,
				CompanyModel:      &models.Company{CompanyID: "company-id", CompanyExternalID: "company-sfid", CompanyName: "Test Company"},
				ClaGroupModel:     &models.ClaGroup{ProjectID: "group-id", ProjectName: "Test Group"},
				ParentProjectSFID: "foundation-sfid",
				EventData: &ECLAInvalidatedEventData{
					Contributor:   "contributor",
					CompanyName:   "Test Company",
					CLAGroupName:  "Test Group",
					InvalidatedBy: "lf-manager",
				},
			}
			if tt.providedUser {
				args.UserModel = tt.user
			}
			service.LogEventWithContext(context.Background(), args)

			tr.mu.Lock()
			items := append([]map[string]*dynamodb.AttributeValue(nil), tr.items...)
			tr.mu.Unlock()
			require.Len(t, items, 1, "the audit event must be persisted without creating an EasyCLA user")
			item := items[0]
			assert.Equal(t, &dynamodb.AttributeValue{S: aws.String("lf-manager")}, item["event_lf_username"])
			assert.Equal(t, &dynamodb.AttributeValue{S: aws.String(tt.wantName)}, item["event_user_name"])
			wantID := ""
			if tt.user != nil {
				wantID = tt.user.UserID
				assert.Equal(t, &dynamodb.AttributeValue{S: aws.String(wantID)}, item["event_user_id"])
			} else {
				assert.NotContains(t, item, "event_user_id", "never manufacture an EasyCLA user ID")
			}
			assert.Contains(t, aws.StringValue(item["event_data"].S), "by the CLA Manager "+tt.wantName)
			assert.Equal(t, "company-id", aws.StringValue(item["event_company_id"].S))
			assert.Equal(t, "group-id", aws.StringValue(item["event_cla_group_id"].S))

			result, err := service.GetCompanyFoundationEvents("company-sfid", "company-id", "foundation-sfid", nil, nil, nil, false)
			require.NoError(t, err)
			require.Len(t, result.Events, 1)
			assert.Equal(t, wantID, result.Events[0].UserID)
			assert.Equal(t, "lf-manager", result.Events[0].LfUsername)
			assert.Equal(t, tt.wantName, result.Events[0].UserName)
		})
	}
}

func TestCreateEventRequiresActor(t *testing.T) {
	repo := &repository{}
	assert.ErrorIs(t, repo.CreateEvent(&models.Event{EventType: InvalidatedSignature}), ErrUserIDRequired)
	assert.ErrorIs(t, repo.CreateEvent(&models.Event{LfUsername: "lf-manager"}), ErrEventTypeRequired)
}
