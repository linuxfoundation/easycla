// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sss

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetOrganizationStatus_Success(t *testing.T) {
	authCalls := int32(0)
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/oauth/token" {
			t.Fatalf("unexpected auth request %s %s", r.Method, r.URL.Path)
		}
		atomic.AddInt32(&authCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"token-abc","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	serviceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/organizations/status" {
			t.Fatalf("unexpected service request %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("domain"); got != "example.com" {
			t.Fatalf("unexpected domain: %s", got)
		}
		if got := r.URL.Query().Get("org_name"); got != "Example Corp" {
			t.Fatalf("unexpected org_name: %s", got)
		}
		if got := r.URL.Query().Get("country"); got != "US" {
			t.Fatalf("unexpected country: %s", got)
		}
		if got := r.URL.Query().Get("city"); got != "San Francisco" {
			t.Fatalf("unexpected city: %s", got)
		}
		if got := r.URL.Query().Get("state"); got != "CA" {
			t.Fatalf("unexpected state: %s", got)
		}
		if got := r.URL.Query().Get("postal_code"); got != "94105" {
			t.Fatalf("unexpected postal_code: %s", got)
		}
		if got := r.URL.Query().Get("sfdc_id"); got != "SFDC-123" {
			t.Fatalf("unexpected sfdc_id: %s", got)
		}
		if got := r.URL.Query().Get("clearbit_id"); got != "CLEARBIT-123" {
			t.Fatalf("unexpected clearbit_id: %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-abc" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"clean","entity_id":"org-123","source":"screening_db","screened_at":"2025-05-16T12:34:56Z","vendor":"descartes","clearbit_enriched":true,"sfdc_id":null,"domain":"example.com","org_name":"Example Corp"}`)
	}))
	defer serviceServer.Close()

	client, err := NewClient(SSSConfig{
		BaseURL:           serviceServer.URL,
		Auth0Domain:       authServer.URL,
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{
		Domain:     "example.com",
		OrgName:    "Example Corp",
		Country:    "US",
		City:       "San Francisco",
		State:      "CA",
		PostalCode: "94105",
		SFDCID:     "SFDC-123",
		ClearbitID: "CLEARBIT-123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusClean || result.EntityID != "org-123" || result.Source != SourceScreeningDB {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.SFDCID != nil {
		t.Fatalf("expected nullable sfdc_id to decode as nil, got %q", *result.SFDCID)
	}
	if result.Vendor != "descartes" || !result.ClearbitEnriched || result.Domain != "example.com" || result.OrgName != "Example Corp" {
		t.Fatalf("unexpected enriched fields: %+v", result)
	}
	if !result.ScreenedAt.Equal(time.Date(2025, 5, 16, 12, 34, 56, 0, time.UTC)) {
		t.Fatalf("unexpected screened_at: %v", result.ScreenedAt)
	}
	if got := atomic.LoadInt32(&authCalls); got != 1 {
		t.Fatalf("expected 1 auth call, got %d", got)
	}
}

func TestGetOrganizationStatus_MissingDomain(t *testing.T) {
	client, err := NewClient(SSSConfig{
		BaseURL:           "https://example.com",
		Auth0Domain:       "https://auth.example.com",
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{OrgName: "Example Org"})
	var badReq *BadRequestError
	if !errors.As(err, &badReq) {
		t.Fatalf("expected BadRequestError, got %T: %v", err, err)
	}
}

func TestGetOrganizationStatus_MissingOrgName(t *testing.T) {
	client, err := NewClient(SSSConfig{
		BaseURL:           "https://example.com",
		Auth0Domain:       "https://auth.example.com",
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: "example.com"})
	var badReq *BadRequestError
	if !errors.As(err, &badReq) {
		t.Fatalf("expected BadRequestError, got %T: %v", err, err)
	}
}

func TestGetOrganizationStatus_TooManyRequestsReturnsRetryableError(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"token-429","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limit exceeded"}}`)
	}))
	defer server.Close()

	client, err := NewClient(SSSConfig{
		BaseURL:           server.URL,
		Auth0Domain:       authServer.URL,
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: "example.com", OrgName: "RateOrg"})
	var retryErr *RetryableError
	if !errors.As(err, &retryErr) {
		t.Fatalf("expected RetryableError, got %T: %v", err, err)
	}
	if retryErr.RetryAfter != 5*time.Second {
		t.Fatalf("expected retry after 5s, got %v", retryErr.RetryAfter)
	}
	if retryErr.Message != "rate limit exceeded" {
		t.Fatalf("unexpected retry message: %s", retryErr.Message)
	}
}

func TestGetOrganizationStatus_TooManyRequestsClampsNegativeRetryAfter(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"token-negative-retry","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "-5")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limit exceeded"}}`)
	}))
	defer server.Close()

	client, err := NewClient(SSSConfig{
		BaseURL:           server.URL,
		Auth0Domain:       authServer.URL,
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: "example.com", OrgName: "RateOrg"})
	var retryErr *RetryableError
	if !errors.As(err, &retryErr) {
		t.Fatalf("expected RetryableError, got %T: %v", err, err)
	}
	if retryErr.RetryAfter != 0 {
		t.Fatalf("expected retry after 0s, got %v", retryErr.RetryAfter)
	}
	if retryErr.Message != "rate limit exceeded" {
		t.Fatalf("unexpected retry message: %s", retryErr.Message)
	}
}

func TestGetOrganizationStatus_FlaggedResponse(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"token-flagged","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	serviceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"flagged","entity_id":"org-flagged","source":"descartes_api","screened_at":"2025-05-16T12:34:56Z","vendor":"descartes","clearbit_enriched":false,"sfdc_id":"SFDC-456","domain":"example.org","org_name":"Flagged Org"}`)
	}))
	defer serviceServer.Close()

	client, err := NewClient(SSSConfig{
		BaseURL:           serviceServer.URL,
		Auth0Domain:       authServer.URL,
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{
		Domain:  "example.org",
		OrgName: "Flagged Org",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusFlagged || result.EntityID != "org-flagged" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Source != SourceDescartesAPI {
		t.Fatalf("unexpected source: %s", result.Source)
	}
	if result.SFDCID == nil || *result.SFDCID != "SFDC-456" {
		t.Fatalf("unexpected sfdc_id: %+v", result.SFDCID)
	}
}

func TestGetOrganizationStatus_400ReturnsBadRequestError(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/oauth/token" {
			t.Fatalf("unexpected auth request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"token-400","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `bad request from server`)
	}))
	defer server.Close()

	client, err := NewClient(SSSConfig{
		BaseURL:           server.URL,
		Auth0Domain:       authServer.URL,
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: "example.com", OrgName: "BadOrg"})
	var badReq *BadRequestError
	if !errors.As(err, &badReq) {
		t.Fatalf("expected BadRequestError, got %T: %v", err, err)
	}
}

func TestGetOrganizationStatus_401ReturnsAuthError(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/oauth/token" {
			t.Fatalf("unexpected auth request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"token-401","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `unauthorized`)
	}))
	defer server.Close()

	client, err := NewClient(SSSConfig{
		BaseURL:           server.URL,
		Auth0Domain:       authServer.URL,
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: "example.com", OrgName: "AuthOrg"})
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T: %v", err, err)
	}
}

func TestGetOrganizationStatus_503ReturnsRetryableError(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/oauth/token" {
			t.Fatalf("unexpected auth request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"token-503","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `service unavailable`)
	}))
	defer server.Close()

	client, err := NewClient(SSSConfig{
		BaseURL:           server.URL,
		Auth0Domain:       authServer.URL,
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: "example.com", OrgName: "RetryOrg"})
	var retryErr *RetryableError
	if !errors.As(err, &retryErr) {
		t.Fatalf("expected RetryableError, got %T: %v", err, err)
	}
	if retryErr.RetryAfter != 3*time.Second {
		t.Fatalf("expected retry after 3s, got %v", retryErr.RetryAfter)
	}
}

func TestGetOrganizationStatus_TimeoutReturnsTimeoutError(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/oauth/token" {
			t.Fatalf("unexpected auth request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"token-timeout","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"clean","entity_id":"org-timeout","source":"sfdc","screened_at":"2025-05-16T12:34:56Z","vendor":"descartes","clearbit_enriched":true,"sfdc_id":null,"domain":"example.com","org_name":"TimeoutOrg"}`)
	}))
	defer server.Close()

	client, err := NewClient(SSSConfig{
		BaseURL:           server.URL,
		Auth0Domain:       authServer.URL,
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: "example.com", OrgName: "TimeoutOrg"})
	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("expected TimeoutError, got %T: %v", err, err)
	}
}

func TestGetOrganizationStatus_UsesCachedToken(t *testing.T) {
	authCalls := int32(0)
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&authCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"cached-token","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	serviceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"clean","entity_id":"org-cache","source":"sfdc","screened_at":"2025-05-16T12:34:56Z","vendor":"descartes","clearbit_enriched":true,"sfdc_id":null,"domain":"example.com","org_name":"CacheOrg"}`)
	}))
	defer serviceServer.Close()

	client, err := NewClient(SSSConfig{
		BaseURL:           serviceServer.URL,
		Auth0Domain:       authServer.URL,
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if _, err := client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: "example.com", OrgName: "CacheOrg"}); err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt32(&authCalls); got != 1 {
		t.Fatalf("expected 1 auth call, got %d", got)
	}
}

func TestGetOrganizationStatus_RefreshesExpiredToken(t *testing.T) {
	tokenIndex := int32(0)
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := atomic.AddInt32(&tokenIndex, 1)
		accessToken := fmt.Sprintf("token-%d", index)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":"%s","expires_in":1,"token_type":"Bearer"}`,
			accessToken)
	}))
	defer authServer.Close()

	serviceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got == "" {
			t.Fatalf("missing authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"clean","entity_id":"org-expire","source":"screening_db","screened_at":"2025-05-16T12:34:56Z","vendor":"descartes","clearbit_enriched":true,"sfdc_id":null,"domain":"example.com","org_name":"ExpireOrg"}`)
	}))
	defer serviceServer.Close()

	client, err := NewClient(SSSConfig{
		BaseURL:           serviceServer.URL,
		Auth0Domain:       authServer.URL,
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: "example.com", OrgName: "ExpireOrg"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: "example.com", OrgName: "ExpireOrg"}); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&tokenIndex); got < 2 {
		t.Fatalf("expected token refresh after expiry, got %d auth calls", got)
	}
}
