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

const (
	testAuthTokenPath     = "/oauth/token" // #nosec G101 -- endpoint path, not a hardcoded credential
	testOrgDomain         = "example.com"
	testOrgName           = "Example Corp"
	testRateLimitExceeded = "rate limit exceeded"
)

func TestGetOrganizationStatus_Success(t *testing.T) {
	authCalls := int32(0)
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != testAuthTokenPath {
			t.Fatalf("unexpected auth request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("User-Agent"); got != userAgent {
			t.Fatalf("unexpected auth user-agent: %s", got)
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
		if got := r.URL.Query().Get("domain"); got != testOrgDomain {
			t.Fatalf("unexpected domain: %s", got)
		}
		if got := r.URL.Query().Get("org_name"); got != testOrgName {
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
		if got := r.Header.Get("User-Agent"); got != userAgent {
			t.Fatalf("unexpected service user-agent: %s", got)
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
		Domain:     testOrgDomain,
		OrgName:    testOrgName,
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
	if result.Vendor != "descartes" || !result.ClearbitEnriched || result.Domain != testOrgDomain || result.OrgName != testOrgName {
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
		BaseURL:           "https://" + testOrgDomain,
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
		BaseURL:           "https://" + testOrgDomain,
		Auth0Domain:       "https://auth.example.com",
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain})
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
		fmt.Fprintf(w, `{"error":{"code":"RATE_LIMITED","message":"%s"},"request_id":"req-429"}`, testRateLimitExceeded)
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

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "RateOrg"})
	var retryErr *RetryableError
	if !errors.As(err, &retryErr) {
		t.Fatalf("expected RetryableError, got %T: %v", err, err)
	}
	if retryErr.RetryAfter != 5*time.Second {
		t.Fatalf("expected retry after 5s, got %v", retryErr.RetryAfter)
	}
	if retryErr.Message != testRateLimitExceeded {
		t.Fatalf("unexpected retry message: %s", retryErr.Message)
	}
	if retryErr.Code != "RATE_LIMITED" || retryErr.RequestID != "req-429" {
		t.Fatalf("unexpected retry details: %+v", retryErr)
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
		fmt.Fprintf(w, `{"error":{"message":"%s"}}`, testRateLimitExceeded)
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

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "RateOrg"})
	var retryErr *RetryableError
	if !errors.As(err, &retryErr) {
		t.Fatalf("expected RetryableError, got %T: %v", err, err)
	}
	if retryErr.RetryAfter != 0 {
		t.Fatalf("expected retry after 0s, got %v", retryErr.RetryAfter)
	}
	if retryErr.Message != testRateLimitExceeded {
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
		if r.Method != http.MethodPost || r.URL.Path != testAuthTokenPath {
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

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "BadOrg"})
	var badReq *BadRequestError
	if !errors.As(err, &badReq) {
		t.Fatalf("expected BadRequestError, got %T: %v", err, err)
	}
}

func TestGetOrganizationStatus_400PreservesStructuredErrorDetails(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"token-400-structured","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"code":"INVALID_DOMAIN","message":"invalid domain"},"request_id":"req-400"}`)
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

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: "bad domain", OrgName: "BadOrg"})
	var badReq *BadRequestError
	if !errors.As(err, &badReq) {
		t.Fatalf("expected BadRequestError, got %T: %v", err, err)
	}
	if badReq.Message != "invalid domain" || badReq.Code != "INVALID_DOMAIN" || badReq.RequestID != "req-400" {
		t.Fatalf("unexpected bad request details: %+v", badReq)
	}
}

func TestGetOrganizationStatus_404ReturnsNotFoundError(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"token-404","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"code":"ORG_NOT_FOUND","message":"Organization not found in any tier"},"request_id":"req-404"}`)
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

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: "missing.example", OrgName: "MissingOrg"})
	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}
	if notFound.Message != "Organization not found in any tier" || notFound.Code != "ORG_NOT_FOUND" || notFound.RequestID != "req-404" {
		t.Fatalf("unexpected not found details: %+v", notFound)
	}
}

func TestGetOrganizationStatus_401ReturnsAuthError(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != testAuthTokenPath {
			t.Fatalf("unexpected auth request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"token-401","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"code":"TOKEN_EXPIRED","message":"unauthorized"},"request_id":"req-401"}`)
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

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "AuthOrg"})
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T: %v", err, err)
	}
	if authErr.Message != "unauthorized" || authErr.Code != "TOKEN_EXPIRED" || authErr.RequestID != "req-401" {
		t.Fatalf("unexpected auth details: %+v", authErr)
	}
}

func TestGetOrganizationStatus_401InvalidatesCachedToken(t *testing.T) {
	authCalls := int32(0)
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := atomic.AddInt32(&authCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":"token-%d","expires_in":3600,"token_type":"Bearer"}`, index)
	}))
	defer authServer.Close()

	serviceCalls := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := atomic.AddInt32(&serviceCalls, 1)
		if index == 1 {
			if got := r.Header.Get("Authorization"); got != "Bearer token-1" {
				t.Fatalf("unexpected first auth header: %s", got)
			}
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":{"code":"TOKEN_EXPIRED","message":"token expired"},"request_id":"req-expired"}`)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-2" {
			t.Fatalf("expected refreshed token on second request, got %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"clean","entity_id":"org-refresh","source":"sfdc","screened_at":"2025-05-16T12:34:56Z","vendor":"descartes","clearbit_enriched":true,"sfdc_id":null,"domain":"example.com","org_name":"RefreshOrg"}`)
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

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "RefreshOrg"})
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T: %v", err, err)
	}

	if _, err := client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "RefreshOrg"}); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&authCalls); got != 2 {
		t.Fatalf("expected token refetch after auth failure, got %d auth calls", got)
	}
}

func TestGetOrganizationStatus_Auth0ErrorUsesAuth0Payload(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != testAuthTokenPath {
			t.Fatalf("unexpected auth request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid_client","error_description":"Client authentication failed"}`)
	}))
	defer authServer.Close()

	client, err := NewClient(SSSConfig{
		BaseURL:           "https://sss.example.com",
		Auth0Domain:       authServer.URL,
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "Auth0Org"})
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T: %v", err, err)
	}
	if authErr.Code != "invalid_client" {
		t.Fatalf("unexpected auth code: %s", authErr.Code)
	}
	if authErr.Message != "authentication failed: Client authentication failed" {
		t.Fatalf("unexpected auth message: %s", authErr.Message)
	}
	if got := authErr.Error(); got != "authentication error: authentication failed: Client authentication failed (code=invalid_client request_id=)" {
		t.Fatalf("unexpected auth error string: %s", got)
	}
}

func TestGetOrganizationStatus_503ReturnsRetryableError(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != testAuthTokenPath {
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

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "RetryOrg"})
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
		if r.Method != http.MethodPost || r.URL.Path != testAuthTokenPath {
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

	_, err = client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "TimeoutOrg"})
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
		if _, err := client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "CacheOrg"}); err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt32(&authCalls); got != 1 {
		t.Fatalf("expected 1 auth call, got %d", got)
	}
}

func TestGetOrganizationStatus_CachesTokenWhenExpiresInMissing(t *testing.T) {
	authCalls := int32(0)
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&authCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"fallback-token","expires_in":0,"token_type":"Bearer"}`)
	}))
	defer authServer.Close()

	serviceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"clean","entity_id":"org-fallback","source":"sfdc","screened_at":"2025-05-16T12:34:56Z","vendor":"descartes","clearbit_enriched":true,"sfdc_id":null,"domain":"example.com","org_name":"FallbackOrg"}`)
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
		if _, err := client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "FallbackOrg"}); err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt32(&authCalls); got != 1 {
		t.Fatalf("expected fallback token ttl to cache token, got %d auth calls", got)
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

	if _, err := client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "ExpireOrg"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetOrganizationStatus(context.Background(), OrganizationStatusRequest{Domain: testOrgDomain, OrgName: "ExpireOrg"}); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&tokenIndex); got < 2 {
		t.Fatalf("expected token refresh after expiry, got %d auth calls", got)
	}
}

func TestNewClient_DefaultTimeout(t *testing.T) {
	client, err := NewClient(SSSConfig{
		BaseURL:           "https://sss.example.com",
		Auth0Domain:       "https://auth.example.com",
		Auth0ClientID:     "id",
		Auth0ClientSecret: "secret",
		Auth0Audience:     "audience",
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.httpClient.Timeout != defaultTimeout {
		t.Fatalf("expected default timeout %v, got %v", defaultTimeout, client.httpClient.Timeout)
	}
}
