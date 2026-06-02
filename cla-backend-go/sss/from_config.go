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
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(audience) == "" {
		return nil, nil
	}

	auth0Domain := strings.TrimSpace(oauthTokenURL)
	if u, err := url.Parse(auth0Domain); err == nil && u.Host != "" {
		auth0Domain = u.Scheme + "://" + u.Host
	}

	return NewClient(SSSConfig{
		BaseURL:           baseURL,
		Auth0Domain:       auth0Domain,
		Auth0ClientID:     clientID,
		Auth0ClientSecret: clientSecret,
		Auth0Audience:     audience,
	})
}
