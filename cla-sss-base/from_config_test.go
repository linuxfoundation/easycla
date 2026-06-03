// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sss

import "testing"

const (
	testAuth0Domain   = "https://linuxfoundation.auth0.com"
	testAuth0TokenURL = testAuth0Domain + "/oauth/token"
)

func TestNewClientFromPlatformCredentials_DisabledWhenBaseURLMissing(t *testing.T) {
	client, err := NewClientFromPlatformCredentials("", "https://sss.example/", testAuth0TokenURL, "id", "secret")
	if err != nil {
		t.Fatalf("expected no error when disabled, got: %v", err)
	}
	if client != nil {
		t.Fatalf("expected nil client when base URL is empty")
	}
}

func TestNewClientFromPlatformCredentials_DisabledWhenAudienceMissing(t *testing.T) {
	client, err := NewClientFromPlatformCredentials("https://sss.example", "", testAuth0TokenURL, "id", "secret")
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
		testAuth0TokenURL,
		"client-id",
		"client-secret",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected a configured client")
	}

	if got, want := client.cfg.Auth0Domain, testAuth0Domain; got != want {
		t.Errorf("Auth0Domain: got %q, want %q", got, want)
	}
	// The derived domain must yield the original token endpoint, not a doubled path.
	if got, want := client.authTokenURL(), testAuth0TokenURL; got != want {
		t.Errorf("authTokenURL: got %q, want %q", got, want)
	}
}

func TestNewClientFromPlatformCredentials_DerivesAuth0DomainFromSchemelessTokenURL(t *testing.T) {
	cases := []struct {
		name     string
		tokenURL string
	}{
		{"scheme-less with path", "linuxfoundation.auth0.com/oauth/token"},
		{"scheme-less host only", "linuxfoundation.auth0.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewClientFromPlatformCredentials(
				"https://sanctions-screening.dev.v2.cluster.linuxfound.info",
				"https://sanctions-screening.dev.v2.cluster.linuxfound.info/",
				tc.tokenURL,
				"client-id",
				"client-secret",
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if client == nil {
				t.Fatal("expected a configured client")
			}
			if got, want := client.cfg.Auth0Domain, testAuth0Domain; got != want {
				t.Errorf("Auth0Domain: got %q, want %q", got, want)
			}
			// A scheme-less token URL must not double the "/oauth/token" path.
			if got, want := client.authTokenURL(), testAuth0TokenURL; got != want {
				t.Errorf("authTokenURL: got %q, want %q", got, want)
			}
		})
	}
}

func TestNewClientFromPlatformCredentials_MissingCredentialsErrors(t *testing.T) {
	// base URL + audience present but no Auth0 client credentials -> NewClient validates and errors.
	client, err := NewClientFromPlatformCredentials("https://sss.example", "https://sss.example/", testAuth0TokenURL, "", "")
	if err == nil {
		t.Fatal("expected an error when client credentials are missing")
	}
	if client != nil {
		t.Fatal("expected nil client on error")
	}
}

func TestNewClientFromPlatformCredentials_TrimsValues(t *testing.T) {
	client, err := NewClientFromPlatformCredentials(
		"  https://sanctions-screening.dev.v2.cluster.linuxfound.info  ",
		"  https://sanctions-screening.dev.v2.cluster.linuxfound.info/  ",
		"  https://linuxfoundation.auth0.com/oauth/token  ",
		"  client-id  ",
		"  client-secret  ",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected a configured client")
	}
	if got, want := client.cfg.BaseURL, "https://sanctions-screening.dev.v2.cluster.linuxfound.info"; got != want {
		t.Errorf("BaseURL not trimmed: got %q, want %q", got, want)
	}
	if got, want := client.cfg.Auth0Audience, "https://sanctions-screening.dev.v2.cluster.linuxfound.info/"; got != want {
		t.Errorf("Auth0Audience not trimmed: got %q, want %q", got, want)
	}
	if got, want := client.cfg.Auth0ClientID, "client-id"; got != want {
		t.Errorf("Auth0ClientID not trimmed: got %q, want %q", got, want)
	}
	if got, want := client.cfg.Auth0ClientSecret, "client-secret"; got != want {
		t.Errorf("Auth0ClientSecret not trimmed: got %q, want %q", got, want)
	}
}
