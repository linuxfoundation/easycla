// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package userservice

import (
	"context"
	"net/http"
	"strings"
)

// NewForAudience is NewFromEnv with a token for another audience than PLATFORM_AUTH0_AUDIENCE
func NewForAudience(httpClient *http.Client, audience string) *Client {
	c := NewFromEnv(httpClient)
	c.audience = strings.TrimSpace(audience)
	return c
}

// AccessToken returns the client-credentials token of the client
func (c *Client) AccessToken(ctx context.Context) (string, error) {
	return c.getAccessToken(ctx)
}
