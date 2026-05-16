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
		if got := r.URL.Query().Get("organization_id"); got != "org-123" {
			t.Fatalf("unexpected organization_id: %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-abc" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"clear","entity_id":"org-123","source":"ofac","screened_at":"2025-05-16T12:34:56Z"}`)
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

	result, err := client.GetOrganizationStatus(context.Background(), "org-123")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "clear" || result.EntityID != "org-123" || result.Source != "ofac" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !result.ScreenedAt.Equal(time.Date(2025, 5, 16, 12, 34, 56, 0, time.UTC)) {
		t.Fatalf("unexpected screened_at: %v", result.ScreenedAt)
	}
	if got := atomic.LoadInt32(&authCalls); got != 1 {
		t.Fatalf("expected 1 auth call, got %d", got)
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
		fmt.Fprint(w, `{"status":"flagged","entity_id":"org-flagged","source":"ofac","screened_at":"2025-05-16T12:34:56Z"}`)
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

	result, err := client.GetOrganizationStatus(context.Background(), "org-flagged")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "flagged" || result.EntityID != "org-flagged" {
		t.Fatalf("unexpected result: %+v", result)
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

	_, err = client.GetOrganizationStatus(context.Background(), "org-400")
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

	_, err = client.GetOrganizationStatus(context.Background(), "org-401")
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

	_, err = client.GetOrganizationStatus(context.Background(), "org-503")
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
		fmt.Fprint(w, `{"status":"clear","entity_id":"org-timeout","source":"ofac","screened_at":"2025-05-16T12:34:56Z"}`)
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

	_, err = client.GetOrganizationStatus(context.Background(), "org-timeout")
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
		fmt.Fprint(w, `{"status":"clear","entity_id":"org-cache","source":"ofac","screened_at":"2025-05-16T12:34:56Z"}`)
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
		if _, err := client.GetOrganizationStatus(context.Background(), "org-cache"); err != nil {
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
		fmt.Fprint(w, `{"status":"clear","entity_id":"org-expire","source":"ofac","screened_at":"2025-05-16T12:34:56Z"}`)
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

	if _, err := client.GetOrganizationStatus(context.Background(), "org-expire"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetOrganizationStatus(context.Background(), "org-expire"); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&tokenIndex); got < 2 {
		t.Fatalf("expected token refresh after expiry, got %d auth calls", got)
	}
}
