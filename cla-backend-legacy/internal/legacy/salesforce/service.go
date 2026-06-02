// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package salesforce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ProjectService response (platform project-service).
type projectSearchResponse struct {
	Data []struct {
		Name        string  `json:"Name"`
		ID          string  `json:"ID"`
		Description *string `json:"Description"`
	} `json:"Data"`
}

// SalesforceProject is the legacy response object returned by /v1/salesforce/projects and /v1/salesforce/project.
type SalesforceProject struct {
	Name        string  `json:"name"`
	ID          string  `json:"id"`
	Description *string `json:"description"`
	LogoURL     *string `json:"logoUrl"`
}

// AuthFailureError indicates the platform Auth0 client-credentials flow failed.
//
// Python parity note: cla/salesforce.py:get_access_token() returns (None, err.response.status_code)
// on HTTP errors and (None, 500) on other exceptions. The callers then return the status code
// along with the fixed message "Authentication failure".
type AuthFailureError struct {
	Status int
	Cause  error
}

func (e *AuthFailureError) Error() string {
	if e == nil {
		return "auth failure"
	}
	if e.Cause != nil {
		return fmt.Sprintf("auth failure (status=%d): %v", e.Status, e.Cause)
	}
	return fmt.Sprintf("auth failure (status=%d)", e.Status)
}

// ProjectDetail represents a full project with foundation info for standalone/LF supported checks
type ProjectDetail struct {
	Name        string  `json:"Name"`
	ID          string  `json:"ID"`
	Description *string `json:"Description"`
	Funding     *string `json:"Funding"`
	Foundation  *struct {
		ID   string `json:"ID"`
		Name string `json:"Name"`
	} `json:"Foundation"`
	Projects []interface{} `json:"Projects"` // Sub-projects
}

// Foundation constants from the Python backend
const (
	TheLinuxFoundation = "The Linux Foundation"
	LFProjectsLLC      = "LF Projects, LLC"
)

func (e *AuthFailureError) Unwrap() error { return e.Cause }

// ProjectServiceError indicates the downstream project-service call failed (non-200).
// Callers in Python return the downstream status code with a fixed message.
type ProjectServiceError struct {
	Status int
	Body   string
	Cause  error
}

func (e *ProjectServiceError) Error() string {
	if e == nil {
		return "project-service error"
	}
	if e.Body != "" {
		return fmt.Sprintf("project-service error (status=%d): %v: %s", e.Status, e.Cause, e.Body)
	}
	return fmt.Sprintf("project-service error (status=%d): %v", e.Status, e.Cause)
}

func (e *ProjectServiceError) Unwrap() error { return e.Cause }

type Service struct {
	platformGatewayURL string
	auth0URL           string
	clientID           string
	clientSecret       string
	audience           string
	logoBaseURL        string

	httpClient *http.Client
}

func NewFromEnv(httpClient *http.Client) *Service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	logo := strings.TrimSpace(os.Getenv("CLA_BUCKET_LOGO_URL"))
	if logo == "" {
		// Python default: cla/salesforce.py
		logo = "https://s3.amazonaws.com/cla-project-logo-dev"
	}
	return &Service{
		platformGatewayURL: strings.TrimSpace(os.Getenv("PLATFORM_GATEWAY_URL")),
		auth0URL:           strings.TrimSpace(os.Getenv("PLATFORM_AUTH0_URL")),
		clientID:           strings.TrimSpace(os.Getenv("PLATFORM_AUTH0_CLIENT_ID")),
		clientSecret:       strings.TrimSpace(os.Getenv("PLATFORM_AUTH0_CLIENT_SECRET")),
		audience:           strings.TrimSpace(os.Getenv("PLATFORM_AUTH0_AUDIENCE")),
		logoBaseURL:        logo,
		httpClient:         httpClient,
	}
}

func (s *Service) GetProjects(ctx context.Context, projectIDs []string) ([]SalesforceProject, int, error) {
	if len(projectIDs) == 0 {
		return nil, http.StatusForbidden, errors.New("no authorized projects")
	}

	tok, code, err := s.getAccessToken(ctx)
	if err != nil {
		return nil, code, &AuthFailureError{Status: code, Cause: err}
	}
	if code != http.StatusOK {
		return nil, code, &AuthFailureError{Status: code, Cause: err}
	}

	endpoint, err := s.projectsSearchURL(projectIDs)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	req.Header.Set("Accept", "application/json")
	// Python uses lowercase "bearer" here.
	req.Header.Set("Authorization", "bearer "+tok)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, resp.StatusCode, &ProjectServiceError{Status: resp.StatusCode, Body: string(b), Cause: fmt.Errorf("project-service status: %d", resp.StatusCode)}
	}

	var psr projectSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&psr); err != nil {
		return nil, http.StatusInternalServerError, err
	}

	projects := make([]SalesforceProject, 0, len(psr.Data))
	for _, p := range psr.Data {
		var logoURL *string
		if strings.TrimSpace(p.ID) != "" && s.logoBaseURL != "" {
			u := strings.TrimRight(s.logoBaseURL, "/") + "/" + p.ID + ".png"
			logoURL = &u
		}
		projects = append(projects, SalesforceProject{
			Name:        p.Name,
			ID:          p.ID,
			Description: p.Description,
			LogoURL:     logoURL,
		})
	}
	return projects, http.StatusOK, nil
}

func (s *Service) GetProject(ctx context.Context, projectID string) (*SalesforceProject, int, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, http.StatusBadRequest, errors.New("project id is required")
	}

	tok, code, err := s.getAccessToken(ctx)
	if err != nil {
		return nil, code, &AuthFailureError{Status: code, Cause: err}
	}
	if code != http.StatusOK {
		return nil, code, &AuthFailureError{Status: code, Cause: err}
	}

	endpoint, err := s.projectsSearchURL([]string{projectID})
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	req.Header.Set("Accept", "application/json")
	// Python uses capital "Bearer" here.
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, resp.StatusCode, &ProjectServiceError{Status: resp.StatusCode, Body: string(b), Cause: fmt.Errorf("project-service status: %d", resp.StatusCode)}
	}

	var psr projectSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&psr); err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if len(psr.Data) == 0 {
		return nil, http.StatusNotFound, errors.New("project not found")
	}

	result := psr.Data[0]
	var logoURL *string
	if strings.TrimSpace(result.ID) != "" && s.logoBaseURL != "" {
		u := strings.TrimRight(s.logoBaseURL, "/") + "/" + result.ID + ".png"
		logoURL = &u
	}
	p := &SalesforceProject{
		Name:        result.Name,
		ID:          result.ID,
		Description: result.Description,
		LogoURL:     logoURL,
	}
	return p, http.StatusOK, nil
}

func (s *Service) projectsSearchURL(projectIDs []string) (string, error) {
	base := strings.TrimRight(s.platformGatewayURL, "/")
	if base == "" {
		return "", errors.New("PLATFORM_GATEWAY_URL is empty")
	}

	// Match python: /project-service/v1/projects/search?id=1,2,3
	ids := make([]string, 0, len(projectIDs))
	for _, id := range projectIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	q := url.Values{}
	q.Set("id", strings.Join(ids, ","))

	// Ensure we don't double slashes.
	endpoint := base + "/project-service/v1/projects/search?" + q.Encode()
	return endpoint, nil
}

// Organization represents a minimal platform organization record.
type Organization struct {
	ID      string   `json:"ID"`
	Name    string   `json:"Name"`
	Domains []string `json:"Domains"`
	Link    string   `json:"Link"`
}

// GetOrganization retrieves an organization by its Salesforce ID.
func (s *Service) GetOrganization(ctx context.Context, sfid string) (*Organization, error) {
	if sfid == "" {
		return nil, errors.New("salesforce id is required")
	}

	tok, code, err := s.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth failure (status=%d): %w", code, err)
	}

	base := strings.TrimRight(s.platformGatewayURL, "/")
	if base == "" {
		return nil, errors.New("PLATFORM_GATEWAY_URL is empty")
	}

	endpoint := fmt.Sprintf("%s/organization-service/v1/organizations/%s", base, sfid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &ProjectServiceError{Status: resp.StatusCode, Body: string(body), Cause: fmt.Errorf("failed to get organization %s", sfid)}
	}

	var org Organization
	if err := json.NewDecoder(resp.Body).Decode(&org); err != nil {
		return nil, fmt.Errorf("decode organization: %w", err)
	}
	return &org, nil
}

// getAccessToken performs the platform Auth0 client_credentials flow.
//
// Python parity: cla/salesforce.py:get_access_token() uses x-www-form-urlencoded
// payload encoding (requests.post(..., data=payload)) with:
//   - Content-Type: application/x-www-form-urlencoded
//   - Accept: application/json
func (s *Service) getAccessToken(ctx context.Context) (string, int, error) {
	if strings.TrimSpace(s.auth0URL) == "" {
		return "", http.StatusInternalServerError, errors.New("PLATFORM_AUTH0_URL is empty")
	}
	if s.clientID == "" || s.clientSecret == "" || s.audience == "" {
		return "", http.StatusInternalServerError, errors.New("platform auth0 client credentials are not configured")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("audience", s.audience)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.auth0URL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", http.StatusInternalServerError, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", http.StatusInternalServerError, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		// Match python: return err.response.status_code.
		return "", resp.StatusCode, fmt.Errorf("auth0 token status %d: %s", resp.StatusCode, string(body))
	}

	var tr struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", http.StatusInternalServerError, err
	}
	if strings.TrimSpace(tr.AccessToken) == "" {
		return "", http.StatusInternalServerError, errors.New("auth0 token response missing access_token")
	}
	return tr.AccessToken, http.StatusOK, nil
}

// IsStandaloneProject checks if a Salesforce project is a standalone project.
// A project is standalone if it has no parent or its parent is The Linux Foundation/LF Projects LLC
// and it has no sub-projects.
func (s *Service) IsStandaloneProject(ctx context.Context, projectSFID string) (bool, error) {
	project, err := s.getProjectDetailByID(ctx, projectSFID)
	if err != nil {
		//return false, err
		// Python ProjectService.get_project_by_id() returns None on downstream
		// project-service HTTP errors; is_standalone() then returns False.
		return false, nil
	}
	if project == nil {
		return false, nil
	}

	parentName := s.getParentName(project)
	if parentName == nil {
		//return false, nil
		// Python: parent_name is None => standalone.
		return true, nil
	}
	if *parentName == TheLinuxFoundation || *parentName == LFProjectsLLC {
		if len(project.Projects) == 0 {
			return true, nil
		}
	}
	return false, nil
}

// IsLFSupportedProject checks if a Salesforce project is an LF-supported project.
// A project is LF-supported if its funding is "Unfunded" or "Supported By Parent"
// and its parent is The Linux Foundation or LF Projects LLC.
func (s *Service) IsLFSupportedProject(ctx context.Context, projectSFID string) (bool, error) {
	project, err := s.getProjectDetailByID(ctx, projectSFID)
	if err != nil {
		//return false, err
		// Python ProjectService.get_project_by_id() returns None on downstream
		// project-service HTTP errors; is_lf_supported() then returns False.
		return false, nil
	}
	if project == nil {
		return false, nil
	}

	parentName := s.getParentName(project)
	if parentName == nil {
		return false, nil
	}

	fundingOK := project.Funding != nil &&
		(*project.Funding == "Unfunded" || *project.Funding == "Supported By Parent")
	parentOK := *parentName == TheLinuxFoundation || *parentName == LFProjectsLLC

	return fundingOK && parentOK, nil
}

// getProjectDetailByID fetches full project details including foundation info
func (s *Service) getProjectDetailByID(ctx context.Context, projectID string) (*ProjectDetail, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project ID is required")
	}

	accessToken, status, err := s.getAccessToken(ctx)
	if err != nil {
		return nil, &AuthFailureError{Status: status, Cause: err}
	}

	base := strings.TrimRight(s.platformGatewayURL, "/")
	if base == "" {
		return nil, errors.New("PLATFORM_GATEWAY_URL is empty")
	}

	// Legacy Python uses /project-service/v1/projects/{project_id}.
	projectURL := fmt.Sprintf("%s/project-service/v1/projects/%s", base, url.PathEscape(projectID))
	req, err := http.NewRequestWithContext(ctx, "GET", projectURL, nil)
	if err != nil {
		return nil, err
	}
	// req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Authorization", "bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, &ProjectServiceError{Status: http.StatusInternalServerError, Cause: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Legacy Python catches HTTPError and returns None.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8192))
		return nil, nil
	}

	var project ProjectDetail
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, &ProjectServiceError{Status: http.StatusInternalServerError, Cause: err}
	}
	return &project, nil
}

// getParentName returns the project parent name if it exists, otherwise returns nil
func (s *Service) getParentName(project *ProjectDetail) *string {
	if project == nil || project.Foundation == nil {
		return nil
	}
	if project.Foundation.ID == "" || project.Foundation.Name == "" {
		return nil
	}
	return &project.Foundation.Name
}
