// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package githublegacy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type pullRequestResponse struct {
	HTMLURL string `json:"html_url"`
}

// GetPullRequestHTMLURL fetches the pull request HTML URL using a GitHub App installation token.
//
// Legacy Python: GitHub.get_return_url() -> GitHub.get_pull_request() -> PR.html_url
func (s *Service) GetPullRequestHTMLURL(ctx context.Context, installationID int64, repositoryID int64, pullRequestNumber int64) (string, error) {
	if installationID <= 0 {
		return "", errors.New("invalid installation_id")
	}
	if repositoryID <= 0 {
		return "", errors.New("invalid repository_id")
	}
	if pullRequestNumber <= 0 {
		return "", errors.New("invalid pull_request_number")
	}

	token, err := s.getInstallationAccessToken(ctx, installationID)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("%s/repositories/%d/pulls/%d", envGitHubAPIBaseURL(), repositoryID, pullRequestNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cla-backend-legacy")

	hc := s.httpClient
	if hc == nil {
		hc = http.DefaultClient
	}

	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("github pull request request failed: status=%d body=%s", resp.StatusCode, string(b))
	}

	var pr pullRequestResponse
	if err := json.Unmarshal(b, &pr); err != nil {
		return "", err
	}
	if strings.TrimSpace(pr.HTMLURL) == "" {
		return "", errors.New("github pull request response missing html_url")
	}
	return pr.HTMLURL, nil
}
