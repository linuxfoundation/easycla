package userservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/logging"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/store"
)

// Client is a minimal port of the legacy Python cla.user_service.UserService.
//
// It is used (currently) for the return-url flow to validate that CLA managers
// have the expected "cla-manager" role assignments before redirecting users
// back to the originating UI.
//
// Legacy Python reference:
//   - cla/user_service.py::UserServiceInstance
//   - cla/controllers/signing.py::return_url
//
// This client uses the Platform Auth0 client-credentials flow to obtain an access
// token and then calls platform services through PLATFORM_GATEWAY_URL.
//
// Required env vars:
//   - PLATFORM_GATEWAY_URL
//   - PLATFORM_AUTH0_URL
//   - PLATFORM_AUTH0_CLIENT_ID
//   - PLATFORM_AUTH0_CLIENT_SECRET
//   - PLATFORM_AUTH0_AUDIENCE
//
// NOTE: This mirrors the Python behavior of returning False on most upstream
// errors (rather than failing hard), because callers typically treat this as a
// best-effort eventual-consistency wait.

type Client struct {
	platformGatewayURL string
	auth0URL           string
	clientID           string
	clientSecret       string
	audience           string

	httpClient *http.Client

	mu                sync.Mutex
	accessToken       string
	accessTokenExpiry time.Time
}

func NewFromEnv(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{
		platformGatewayURL: strings.TrimSpace(os.Getenv("PLATFORM_GATEWAY_URL")),
		auth0URL:           strings.TrimSpace(os.Getenv("PLATFORM_AUTH0_URL")),
		clientID:           strings.TrimSpace(os.Getenv("PLATFORM_AUTH0_CLIENT_ID")),
		clientSecret:       strings.TrimSpace(os.Getenv("PLATFORM_AUTH0_CLIENT_SECRET")),
		audience:           strings.TrimSpace(os.Getenv("PLATFORM_AUTH0_AUDIENCE")),
		httpClient:         httpClient,
	}
}

// HasRole checks whether a given LF username has the specified role for an organization
// across the projects represented by a CLA Group.
//
// Legacy Python:
//   - UserServiceInstance.has_role
//   - UserServiceInstance._list_org_user_scopes
//   - ProjectCLAGroup.signed_at_foundation
//
// Returns (false, nil) on most upstream/parse errors to mirror Python behavior.
func (c *Client) HasRole(ctx context.Context, username, role, organizationID, claGroupID string, pcgStore *store.ProjectCLAGroupsStore) (bool, error) {
	username = strings.TrimSpace(username)
	role = strings.TrimSpace(role)
	organizationID = strings.TrimSpace(organizationID)
	claGroupID = strings.TrimSpace(claGroupID)

	if username == "" || role == "" || organizationID == "" || claGroupID == "" {
		return false, nil
	}
	if c == nil {
		return false, nil
	}

	scopes, err := c.listOrgUserScopes(ctx, organizationID, role)
	if err != nil {
		// Python logs and returns None -> has_role returns False.
		logging.Warnf("userservice.has_role scopes error (org=%s role=%s): %v", organizationID, role, err)
		return false, nil
	}
	if scopes == nil {
		return false, nil
	}

	// Load ProjectCLAGroup mappings for cla_group_id.
	if pcgStore == nil {
		return false, nil
	}
	pcgs, err := pcgStore.QueryByCLAGroupID(ctx, claGroupID)
	if err != nil {
		logging.Warnf("userservice.has_role query project-cla-groups (cla_group_id=%s) error: %v", claGroupID, err)
		return false, nil
	}
	if len(pcgs) == 0 {
		return false, nil
	}

	// Python checks pcgs[0].signed_at_foundation which internally uses foundation_sfid
	// from the first mapping.
	first := store.ItemToInterfaceMap(pcgs[0])
	foundationSFID, _ := first["foundation_sfid"].(string)
	projectSFID0, _ := first["project_sfid"].(string)

	signedAtFoundation := false
	if strings.TrimSpace(foundationSFID) != "" {
		found, err := c.isSignedAtFoundation(ctx, foundationSFID, pcgStore)
		if err != nil {
			logging.Warnf("userservice.has_role signed_at_foundation check error (foundation_sfid=%s): %v", foundationSFID, err)
			// Python would just treat this as False and continue.
			found = false
		}
		signedAtFoundation = found
	}

	if signedAtFoundation {
		// Foundation-level: check only the first mapping project_sfid.
		if strings.TrimSpace(projectSFID0) == "" {
			return false, nil
		}
		return hasProjectOrgScope(scopes, projectSFID0, organizationID, username), nil
	}

	// Project-level behavior:
	//
	// The legacy Python implementation intends to check all project mappings, but it
	// accidentally overwrites the map key on each iteration:
	//   has_role_project_org[username] = (...)
	// which means only the *last* mapping effectively determines the result.
	//
	// For strict 1:1 parity (and to keep the return-url wait loop behavior stable),
	// mirror that behavior here.
	//
	// FIXME: Once the Python backend is fully removed, consider changing this to
	// require scopes for all projects in the mapping list.
	last := false
	for _, raw := range pcgs {
		m := store.ItemToInterfaceMap(raw)
		ps, _ := m["project_sfid"].(string)
		ps = strings.TrimSpace(ps)
		last = hasProjectOrgScope(scopes, ps, organizationID, username)
	}
	return last, nil
}

func (c *Client) isSignedAtFoundation(ctx context.Context, foundationSFID string, pcgStore *store.ProjectCLAGroupsStore) (bool, error) {
	foundationSFID = strings.TrimSpace(foundationSFID)
	if foundationSFID == "" {
		return false, nil
	}
	items, err := pcgStore.QueryByFoundationSFID(ctx, foundationSFID)
	if err != nil {
		return false, err
	}
	for _, it := range items {
		m := store.ItemToInterfaceMap(it)
		fs, _ := m["foundation_sfid"].(string)
		ps, _ := m["project_sfid"].(string)
		if strings.TrimSpace(fs) != "" && strings.TrimSpace(ps) != "" {
			if fs == ps {
				return true, nil
			}
		}
	}
	return false, nil
}

// hasProjectOrgScope matches the legacy Python _has_project_org_scope() helper.
//
// It checks the org service scopes payload for a user role whose Contact.Username
// matches and whose first RoleScopes[0].Scopes contains ObjectID == "project_sfid|organization_id".
func hasProjectOrgScope(scopes map[string]any, projectSFID, organizationID, username string) bool {
	userRolesAny, ok := scopes["userroles"]
	if !ok || userRolesAny == nil {
		return false
	}
	userRoles, ok := userRolesAny.([]any)
	if !ok {
		return false
	}
	needle := fmt.Sprintf("%s|%s", projectSFID, organizationID)

	for _, ur := range userRoles {
		urm, ok := ur.(map[string]any)
		if !ok || urm == nil {
			continue
		}

		// Contact.Username (case-sensitive as returned by org service)
		contactAny := urm["Contact"]
		contact, ok := contactAny.(map[string]any)
		if !ok || contact == nil {
			continue
		}
		uname, _ := contact["Username"].(string)
		if uname != username {
			continue
		}

		roleScopesAny := urm["RoleScopes"]
		roleScopes, ok := roleScopesAny.([]any)
		if !ok || len(roleScopes) == 0 {
			continue
		}
		rs0, ok := roleScopes[0].(map[string]any)
		if !ok || rs0 == nil {
			continue
		}
		scopesAny := rs0["Scopes"]
		scArr, ok := scopesAny.([]any)
		if !ok {
			continue
		}
		for _, sc := range scArr {
			sm, ok := sc.(map[string]any)
			if !ok || sm == nil {
				continue
			}
			objID, _ := sm["ObjectID"].(string)
			if objID == needle {
				return true
			}
		}
	}

	return false
}

func (c *Client) listOrgUserScopes(ctx context.Context, organizationID, role string) (map[string]any, error) {
	organizationID = strings.TrimSpace(organizationID)
	role = strings.TrimSpace(role)
	if organizationID == "" {
		return nil, errors.New("organization_id is required")
	}
	if role == "" {
		return nil, errors.New("role is required")
	}
	base := strings.TrimRight(strings.TrimSpace(c.platformGatewayURL), "/")
	if base == "" {
		return nil, errors.New("PLATFORM_GATEWAY_URL is empty")
	}

	tok, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/organization-service/v1/orgs/%s/servicescopes", base, url.PathEscape(organizationID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("rolename", role)
	req.URL.RawQuery = q.Encode()

	// Python uses lowercase 'bearer'.
	req.Header.Set("Authorization", "bearer "+tok)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("organization-service status %d: %s", resp.StatusCode, string(b))
	}

	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	m, ok := payload.(map[string]any)
	if !ok {
		return nil, errors.New("unexpected organization-service response")
	}
	return m, nil
}

func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	if c == nil {
		return "", errors.New("nil userservice client")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Use cached value if not expired.
	if c.accessToken != "" && time.Now().Before(c.accessTokenExpiry) {
		return c.accessToken, nil
	}

	if strings.TrimSpace(c.auth0URL) == "" {
		return "", errors.New("PLATFORM_AUTH0_URL is empty")
	}
	if c.clientID == "" || c.clientSecret == "" || c.audience == "" {
		return "", errors.New("platform auth0 client credentials are not configured")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("audience", c.audience)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.auth0URL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("auth0 token status %d: %s", resp.StatusCode, string(b))
	}

	var tr struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	if strings.TrimSpace(tr.AccessToken) == "" {
		return "", errors.New("auth0 token response missing access_token")
	}

	// Cache for expires_in, with a small safety buffer.
	// Python caches for ~30 minutes by default; this keeps behavior stable.
	exp := 30 * time.Minute
	if tr.ExpiresIn > 0 {
		exp = time.Duration(tr.ExpiresIn) * time.Second
	}
	// Safety buffer for clock skew.
	buffer := 30 * time.Second
	if exp > buffer {
		exp = exp - buffer
	}

	c.accessToken = tr.AccessToken
	c.accessTokenExpiry = time.Now().Add(exp)
	return c.accessToken, nil
}
