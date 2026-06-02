// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sss

import (
	"net/url"
	"strings"
)

// NewClientFromPlatformCredentials builds an SSS client that reuses the shared
// LFX platform M2M (Auth0) credentials already configured for EasyCLA.
//
// oauthTokenURL is the full Auth0 token endpoint used for platform auth
// (e.g. https://<tenant>/oauth/token); its scheme+host is reused as the SSS
// client's Auth0 domain, since SSS authenticates with the same client against
// the same Auth0 tenant - only the requested audience differs.
//
// It returns (nil, nil) when baseURL or audience is empty so callers can treat
// an unconfigured SSS as a disabled, no-op feature rather than an error.
func NewClientFromPlatformCredentials(baseURL, audience, oauthTokenURL, clientID, clientSecret string) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	audience = strings.TrimSpace(audience)
	if baseURL == "" || audience == "" {
		return nil, nil
	}

	return NewClient(SSSConfig{
		BaseURL:           baseURL,
		Auth0Domain:       auth0DomainFromTokenURL(oauthTokenURL),
		Auth0ClientID:     strings.TrimSpace(clientID),
		Auth0ClientSecret: strings.TrimSpace(clientSecret),
		Auth0Audience:     audience,
	})
}

// auth0DomainFromTokenURL reduces a full Auth0 token endpoint to its scheme+host
// (e.g. https://tenant.auth0.com), which is what the SSS client expects as its
// Auth0 domain. It tolerates a missing scheme: url.Parse on a scheme-less value
// puts the whole string in Path and leaves Host empty, so a value like
// "tenant.auth0.com/oauth/token" would otherwise be passed through verbatim and
// produce a doubled "/oauth/token" when the client builds the token URL.
func auth0DomainFromTokenURL(oauthTokenURL string) string {
	oauthTokenURL = strings.TrimSpace(oauthTokenURL)
	if oauthTokenURL == "" {
		return ""
	}

	parseTarget := oauthTokenURL
	if !strings.Contains(parseTarget, "://") {
		parseTarget = "https://" + parseTarget
	}
	if u, err := url.Parse(parseTarget); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return oauthTokenURL
}
