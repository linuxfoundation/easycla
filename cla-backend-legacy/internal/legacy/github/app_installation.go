// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package githublegacy

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/config"
)

// GitHubRepo is the minimal subset we need for legacy endpoints.
type GitHubRepo struct {
	ID      int64  `json:"id"`
	Full    string `json:"full_name"`
	HTMLURL string `json:"html_url"`
}

type installationTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type installationReposResponse struct {
	TotalCount   int          `json:"total_count"`
	Repositories []GitHubRepo `json:"repositories"`
}

func envGitHubAPIBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("GITHUB_API_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://api.github.com"
}

func parseGitHubAppID() (int64, error) {
	appIDStr := strings.TrimSpace(os.Getenv("GH_APP_ID"))
	if appIDStr == "" {
		return 0, errors.New("GH_APP_ID is not set")
	}
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid GH_APP_ID: %w", err)
	}
	return appID, nil
}

func parseGitHubPrivateKey(ctx context.Context) (*rsa.PrivateKey, error) {
	pemStr, err := config.GetEnvOrSSM(ctx, "GITHUB_PRIVATE_KEY", "cla-gh-app-private-key")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(pemStr) == "" {
		return nil, errors.New("GITHUB_PRIVATE_KEY is not set")
	}
	// Serverless/SSM values sometimes arrive with literal \n sequences.
	pemStr = strings.ReplaceAll(pemStr, "\\n", "\n")
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to PEM decode GITHUB_PRIVATE_KEY")
	}

	// PKCS#1
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	// PKCS#8
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	k, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return k, nil
}

func base64URLJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func signRS256(privateKey *rsa.PrivateKey, signingInput string) (string, error) {
	h := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(sig), nil
}

func generateGitHubAppJWT(appID int64, privateKey *rsa.PrivateKey, now time.Time) (string, error) {
	header := map[string]any{"alg": "RS256", "typ": "JWT"}
	payload := map[string]any{
		"iat": now.Unix() - 60,
		"exp": now.Unix() + 9*60,
		"iss": appID,
	}

	encHeader, err := base64URLJSON(header)
	if err != nil {
		return "", err
	}
	encPayload, err := base64URLJSON(payload)
	if err != nil {
		return "", err
	}
	signingInput := encHeader + "." + encPayload
	sig, err := signRS256(privateKey, signingInput)
	if err != nil {
		return "", err
	}
	return signingInput + "." + sig, nil
}

func (s *Service) getInstallationAccessToken(ctx context.Context, installationID int64) (string, error) {
	appID, err := parseGitHubAppID()
	if err != nil {
		return "", err
	}
	pk, err := parseGitHubPrivateKey(ctx)
	if err != nil {
		return "", err
	}
	jwt, err := generateGitHubAppJWT(appID, pk, time.Now().UTC())
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("%s/app/installations/%d/access_tokens", envGitHubAPIBaseURL(), installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
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
		return "", fmt.Errorf("github access token request failed: status=%d body=%s", resp.StatusCode, string(b))
	}
	var tr installationTokenResponse
	if err := json.Unmarshal(b, &tr); err != nil {
		return "", err
	}
	if strings.TrimSpace(tr.Token) == "" {
		return "", errors.New("github access token response missing token")
	}
	return tr.Token, nil
}

// ListInstallationRepositories returns all repository full names visible to the given GitHub App installation.
func (s *Service) ListInstallationRepositories(ctx context.Context, installationID int64) ([]GitHubRepo, error) {
	token, err := s.getInstallationAccessToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	perPage := 100
	page := 1
	all := make([]GitHubRepo, 0)

	hc := s.httpClient
	if hc == nil {
		hc = http.DefaultClient
	}

	for {
		u, _ := url.Parse(envGitHubAPIBaseURL() + "/installation/repositories")
		q := u.Query()
		q.Set("per_page", strconv.Itoa(perPage))
		q.Set("page", strconv.Itoa(page))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "token "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "cla-backend-legacy")

		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return nil, fmt.Errorf("github list installation repositories failed: status=%d body=%s", resp.StatusCode, string(b))
		}
		var rr installationReposResponse
		if err := json.Unmarshal(b, &rr); err != nil {
			return nil, err
		}
		all = append(all, rr.Repositories...)

		// Pagination: stop when fewer than perPage are returned.
		if len(rr.Repositories) < perPage {
			break
		}
		page++
		// Safety guard to avoid infinite loops.
		if page > 1000 {
			break
		}
	}

	return all, nil
}
