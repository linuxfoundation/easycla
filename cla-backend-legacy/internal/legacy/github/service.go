// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package githublegacy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type Service struct {
	httpClient *http.Client
}

func New(httpClient *http.Client) *Service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Service{httpClient: httpClient}
}

func (s *Service) ValidateOrganization(ctx context.Context, endpoint string) (map[string]string, int, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		// Python parity: cla.controllers.github.validate_organization() falls
		// off the end (returns None) when "endpoint" is missing. Hug serializes
		// None as a 200 response with empty body.
		return nil, http.StatusOK, nil
	}

	// Python parity: validate_organization() does requests.get(endpoint) with
	// no scheme/host validation. The previous SSRF allowlist (github.com,
	// api.github.com, raw.githubusercontent.com) rejected legitimate
	// non-GitHub endpoints with 400, breaking parity. Restore the permissive
	// behavior. Outbound requests are still bounded by client timeout.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		// Python: requests.MissingSchema / InvalidURL would propagate as 500.
		return nil, http.StatusInternalServerError, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// 1MB cap to prevent memory exhaustion on hostile endpoints.
		limitReader := io.LimitReader(resp.Body, 1<<20)
		b, err := io.ReadAll(limitReader)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		if strings.Contains(string(b), "http://schema.org/Organization") {
			return map[string]string{"status": "ok"}, http.StatusOK, nil
		}
		return map[string]string{"status": "invalid"}, http.StatusOK, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return map[string]string{"status": "not found"}, http.StatusOK, nil
	}
	return map[string]string{"status": "error"}, http.StatusOK, nil
}

// CheckNamespace returns true if GitHub user namespace exists.
// Python uses requests.Response.ok which is true for status_code < 400.
func (s *Service) CheckNamespace(ctx context.Context, namespace string) (bool, int, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return false, http.StatusBadRequest, errors.New("namespace is required")
	}
	url := "https://api.github.com/users/" + namespace
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return false, http.StatusInternalServerError, err
	}
	// GitHub API expects a user-agent.
	req.Header.Set("User-Agent", "easycla-legacy")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, http.StatusBadGateway, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode < 400, http.StatusOK, nil
}

func (s *Service) GetNamespace(ctx context.Context, namespace string) (any, int, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return map[string]any{"errors": map[string]string{"namespace": "Invalid GitHub account namespace"}}, http.StatusOK, nil
	}
	url := "https://api.github.com/users/" + namespace
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	req.Header.Set("User-Agent", "easycla-legacy")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 {
		var payload any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, http.StatusInternalServerError, err
		}
		return payload, http.StatusOK, nil
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return map[string]any{"errors": map[string]string{"namespace": "Invalid GitHub account namespace"}}, http.StatusOK, nil
}
