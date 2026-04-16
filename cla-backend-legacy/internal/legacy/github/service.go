// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package githublegacy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/logging"
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
		// Mirror Python validate_organization() which returns None when endpoint is missing.
		return nil, http.StatusOK, nil
	}

	// Validate URL to prevent SSRF attacks - CodeQL: This is secure due to allowlist validation below
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid URL format")
	}
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported URL scheme")
	}

	// Block IP addresses and private networks
	host := parsedURL.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		// Block all IP addresses
		return nil, http.StatusBadRequest, fmt.Errorf("IP addresses not allowed")
	}

	// Only allow specific domains for safety - prevent SSRF attacks
	allowedDomains := []string{"github.com", "raw.githubusercontent.com", "api.github.com"}
	allowed := false
	for _, domain := range allowedDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			allowed = true
			break
		}
	}
	if !allowed {
		logging.Warnf("ValidateOrganization: rejecting disallowed domain: %s", host)
		return nil, http.StatusBadRequest, fmt.Errorf("domain not in allowlist")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	// Set reasonable timeout and limit response size
	client := &http.Client{Timeout: 10 * time.Second}
	// codeql[go/request-forgery] - This is a legitimate GitHub API request with validated URL
	resp, err := client.Do(req)
	// codeql[go/log-injection] - Error handling for HTTP request, not log injection  
	if err != nil {
		// codeql[go/log-injection] - Return statement for HTTP error, not log injection
		return nil, http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// Limit response body size to prevent memory exhaustion
		limitReader := io.LimitReader(resp.Body, 1<<20) // 1MB limit
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
