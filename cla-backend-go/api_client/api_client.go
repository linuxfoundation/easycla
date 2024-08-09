// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package apiclient

import (
	"context"
	"net/http"
)

type APIClient interface {
	GetData(ctx context.Context, url string) (*http.Response, error)
}

type RestAPIClient struct {
	Client *http.Client
}

// GetData makes a get request to the specified url

func (c *RestAPIClient) GetData(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}
