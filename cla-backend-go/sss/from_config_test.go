// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sss

import "testing"

func TestNewClientFromPlatformCredentials_DisabledWhenBaseURLMissing(t *testing.T) {
	client, err := NewClientFromPlatformCredentials("", "https://sss.example/", "https://tenant.auth0.com/oauth/token", "id", "secret")
	if err != nil {
		t.Fatalf("expected no error when disabled, got: %v", err)
	}
	if client != nil {
		t.Fatalf("expected nil client when base URL is empty")
	}
}

func TestNewClientFromPlatformCredentials_DisabledWhenAudienceMissing(t *testing.T) {
	client, err := NewClientFromPlatformCredentials("https://sss.example", "", "https://tenant.auth0.com/oauth/token", "id", "secret")
	if err != nil {
		t.Fatalf("expected no error when disabled, got: %v", err)
	}
	if client != nil {
		t.Fatalf("expected nil client when audience is empty")
	}
}

func TestNewClientFromPlatformCredentials_DerivesAuth0DomainFromTokenURL(t *testing.T) {
	client, err := NewClientFromPlatformCredentials(
		"https://sanctions-screening.dev.v2.cluster.linuxfound.info",
		"https://sanctions-screening.dev.v2.cluster.linuxfound.info/",
		"https://linuxfoundation.auth0.com/oauth/token",
		"client-id",
		"client-secret",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected a configured client")
	}

	if got, want := client.cfg.Auth0Domain, "https://linuxfoundation.auth0.com"; got != want {
		t.Errorf("Auth0Domain: got %q, want %q", got, want)
	}
	// The derived domain must yield the original token endpoint, not a doubled path.
	if got, want := client.authTokenURL(), "https://linuxfoundation.auth0.com/oauth/token"; got != want {
		t.Errorf("authTokenURL: got %q, want %q", got, want)
	}
}

func TestNewClientFromPlatformCredentials_MissingCredentialsErrors(t *testing.T) {
	// base URL + audience present but no Auth0 client credentials -> NewClient validates and errors.
	client, err := NewClientFromPlatformCredentials("https://sss.example", "https://sss.example/", "https://tenant.auth0.com/oauth/token", "", "")
	if err == nil {
		t.Fatal("expected an error when client credentials are missing")
	}
	if client != nil {
		t.Fatal("expected nil client on error")
	}
}
