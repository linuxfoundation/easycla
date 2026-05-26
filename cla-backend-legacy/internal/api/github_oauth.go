// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	githublegacy "github.com/linuxfoundation/easycla/cla-backend-legacy/internal/legacy/github"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/respond"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/store"
)

// NOTE: This file ports the minimal GitHub OAuth/session based flows used by legacy Python:
//   - GET /v2/user-from-session
//   - GET /v2/github/installation
//   - GET /v2/repository-provider/github/sign/{installation_id}/{github_repository_id}/{change_request_id}
//
// Python sources:
//   - cla/controllers/repository_service.py user_from_session(), sign_request(), oauth2_redirect()
//   - cla/controllers/github.py user_oauth2_callback(), user_authorization_callback()
//   - cla/models/github_models.py sign_request(), oauth2_redirect(), get_or_create_user()
//   - cla/utils.py get_authorization_url_and_state(), fetch_token(), set_active_signature_metadata()

type httpErr struct {
	status  int
	payload any
	err     error
}

func (e *httpErr) Error() string {
	if e == nil {
		return ""
	}
	if e.err != nil {
		return e.err.Error()
	}
	return "http error"
}

func boolQuery(q string) bool {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "1" || q == "true" || q == "t" || q == "yes" || q == "y" {
		return true
	}
	return false
}

func randURLSafe(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Match python secrets.token_urlsafe(): base64 urlsafe, no padding.
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *Handlers) githubCallbackURL() string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("CLA_API_BASE")), "/")
	if base == "" {
		return "/v2/github/installation"
	}
	return base + "/v2/github/installation"
}

func (h *Handlers) githubOAuthClientID() string {
	return strings.TrimSpace(os.Getenv("GH_OAUTH_CLIENT_ID"))
}
func (h *Handlers) githubOAuthClientSecret() string {
	return strings.TrimSpace(os.Getenv("GH_OAUTH_SECRET"))
}

func (h *Handlers) githubAuthURLAndState(stateOverride *string) (authURL string, sessionState string, encodedState string, err error) {
	clientID := h.githubOAuthClientID()
	redirectURI := h.githubCallbackURL()
	scopes := []string{"user:email"}

	if stateOverride == nil {
		st, err := randURLSafe(16)
		if err != nil {
			return "", "", "", err
		}
		authURL, err = githublegacy.BuildOAuthAuthorizeURL(clientID, redirectURI, scopes, st)
		return authURL, st, st, err
	}

	csrf, err := randURLSafe(16)
	if err != nil {
		return "", "", "", err
	}
	statePayload := map[string]string{"csrf": csrf, "state": *stateOverride}
	jb, err := json.Marshal(statePayload)
	if err != nil {
		return "", "", "", err
	}
	encoded := base64.URLEncoding.EncodeToString(jb) // python urlsafe_b64encode uses padding
	authURL, err = githublegacy.BuildOAuthAuthorizeURL(clientID, redirectURI, scopes, encoded)
	if err != nil {
		return "", "", "", err
	}
	// For user-from-session flow, Python stores only csrf in session, but the callback receives encoded state.
	return authURL, csrf, encoded, nil
}

func sessionGetString(s middleware.Session, key string) string {
	if s == nil {
		return ""
	}
	v, ok := s[key]
	if !ok || v == nil {
		return ""
	}
	if str, ok := v.(string); ok {
		return str
	}
	// JSON unmarshalling may decode numbers as float64.
	if f, ok := v.(float64); ok {
		// Avoid scientific notation.
		return strconv.FormatInt(int64(f), 10)
	}
	return ""
}

func sessionSetString(s middleware.Session, key, val string) {
	if s == nil {
		return
	}
	s[key] = val
}

func sessionDel(s middleware.Session, key string) {
	if s == nil {
		return
	}
	delete(s, key)
}

func sessionGetMap(s middleware.Session, key string) map[string]any {
	if s == nil {
		return nil
	}
	v := s[key]
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	// When marshaled/unmarshaled, it should come back as map[string]any.
	return nil
}

func uniqueLowerEmails(emails []string) []string {
	seen := make(map[string]struct{}, len(emails))
	out := make([]string, 0, len(emails))
	for _, e := range emails {
		e = strings.TrimSpace(strings.ToLower(e))
		if e == "" {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out
}

func (h *Handlers) githubGetOrCreateUser(ctx context.Context, sess middleware.Session) (map[string]any, *httpErr) {
	if h.users == nil {
		return nil, &httpErr{status: http.StatusInternalServerError, payload: map[string]any{"errors": "users store not configured"}, err: errors.New("users store nil")}
	}
	if h.github == nil {
		return nil, &httpErr{status: http.StatusInternalServerError, payload: map[string]any{"errors": "github service not configured"}, err: errors.New("github service nil")}
	}

	tokMap := sessionGetMap(sess, "github_oauth2_token")
	if tokMap == nil {
		return nil, &httpErr{status: http.StatusNotFound, payload: map[string]any{"errors": "Cannot find user from session"}, err: errors.New("missing github_oauth2_token")}
	}

	userData, err := h.github.GetOAuthUser(ctx, tokMap)
	if err != nil {
		// Match Python: clear state/token and raise 400.
		sessionDel(sess, "github_oauth2_state")
		sessionDel(sess, "github_oauth2_token")
		return nil, &httpErr{status: http.StatusBadRequest, payload: map[string]any{"errors": "GitHub OAuth error, please try again.", "details": err.Error()}, err: err}
	}

	gidAny := userData["id"]
	var githubID int64
	switch v := gidAny.(type) {
	case float64:
		githubID = int64(v)
	case int64:
		githubID = v
	case int:
		githubID = int64(v)
	case json.Number:
		githubID, _ = v.Int64()
	case string:
		githubID, _ = strconv.ParseInt(v, 10, 64)
	}
	if githubID <= 0 {
		return nil, &httpErr{status: http.StatusBadRequest, payload: map[string]any{"errors": "GitHub OAuth error, please try again.", "details": "missing github user id"}, err: errors.New("missing github user id")}
	}

	emails, err := h.github.GetOAuthVerifiedEmails(ctx, tokMap)
	if err != nil {
		return nil, &httpErr{status: http.StatusBadGateway, payload: map[string]any{"errors": "Unable to retrieve GitHub emails", "details": err.Error()}, err: err}
	}
	emails = uniqueLowerEmails(emails)
	if len(emails) < 1 {
		return nil, &httpErr{status: http.StatusPreconditionFailed, payload: map[string]any{"errors": "No verified email addresses found.", "details": "Please verify at least one email address with GitHub"}, err: errors.New("no verified emails")}
	}

	// Attempt lookup by github-id-index.
	items, err := h.users.QueryByGitHubID(ctx, githubID)
	if err != nil {
		return nil, &httpErr{status: http.StatusInternalServerError, payload: map[string]any{"errors": err.Error()}, err: err}
	}
	// Operate on the raw AttributeValue map so we never round-trip pynamodb
	// types through InterfaceMapToItem's isNumericString heuristic, which
	// can silently coerce digit-only S fields to N.
	var userAV map[string]types.AttributeValue
	if len(items) > 0 {
		userAV = items[0]
	} else {
		// Fallback: look up by email.
		for _, e := range emails {
			// Fast: lf-email-index
			its, err := h.users.QueryByLFEmail(ctx, e)
			if err != nil {
				return nil, &httpErr{status: http.StatusInternalServerError, payload: map[string]any{"errors": err.Error()}, err: err}
			}
			if len(its) == 0 {
				// Slow: scan contains(user_emails, :e)
				its, err = h.users.ScanByUserEmailsContains(ctx, e)
				if err != nil {
					return nil, &httpErr{status: http.StatusInternalServerError, payload: map[string]any{"errors": err.Error()}, err: err}
				}
			}
			if len(its) > 0 {
				userAV = its[0]
				break
			}
		}
	}

	githubLogin, _ := userData["login"].(string)
	githubName, _ := userData["name"].(string)
	githubLogin = strings.TrimSpace(githubLogin)
	githubName = strings.TrimSpace(githubName)

	now := time.Now().UTC()
	if userAV != nil {
		// Update existing user: set github id, username, display name and emails
		if githubLogin != "" {
			userAV["user_github_username"] = &types.AttributeValueMemberS{Value: githubLogin}
		}
		if githubName != "" {
			userAV["user_name"] = &types.AttributeValueMemberS{Value: githubName}
		}
		// PatchedUnicodeSetAttribute on the Python side. emails is guaranteed
		// non-empty by the len(emails) < 1 guard above, so SS is always valid.
		userAV["user_emails"] = &types.AttributeValueMemberSS{Value: emails}
		userAV["user_github_id"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(githubID, 10)}
		userAV["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)}

		if err := h.users.PutItem(ctx, userAV); err != nil {
			return nil, &httpErr{status: http.StatusInternalServerError, payload: map[string]any{"errors": err.Error()}, err: err}
		}
		result := store.ItemToInterfaceMap(userAV)
		// Preserve the pre-cutover wire shape for the OAuth callers
		// (/v2/github/auth/callback no-redirect branch and /v2/user-from-session):
		// pynamodb User.to_dict() returned user_github_id as an int, and the
		// previous Go code mirrored that by overwriting the map entry with an
		// int64. ItemToInterfaceMap converts N to a string, so re-apply the
		// int64 here so JSON consumers continue to see a number.
		result["user_github_id"] = githubID
		return normalizeUserDict(result), nil
	}

	// Create new user.
	newID := uuid.New().String()
	itemAV := map[string]types.AttributeValue{
		"user_id":        &types.AttributeValueMemberS{Value: newID},
		"version":        &types.AttributeValueMemberS{Value: "v1"},
		"date_created":   &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"date_modified":  &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"user_github_id": &types.AttributeValueMemberN{Value: strconv.FormatInt(githubID, 10)},
		"user_emails":    &types.AttributeValueMemberSS{Value: emails},
	}
	if githubLogin != "" {
		itemAV["user_github_username"] = &types.AttributeValueMemberS{Value: githubLogin}
	}
	if githubName != "" {
		itemAV["user_name"] = &types.AttributeValueMemberS{Value: githubName}
	}
	if err := h.users.PutItem(ctx, itemAV); err != nil {
		return nil, &httpErr{status: http.StatusInternalServerError, payload: map[string]any{"errors": err.Error()}, err: err}
	}
	result := store.ItemToInterfaceMap(itemAV)
	// See the update branch above: keep user_github_id as int64 in the
	// response to match pre-cutover Python wire shape.
	result["user_github_id"] = githubID
	return normalizeUserDict(result), nil
}

func (h *Handlers) setActiveSignatureMetadata(ctx context.Context, userID, projectID, repositoryID, pullRequestID string, returnURLs ...string) error {
	if h.kv == nil {
		return nil
	}
	key := "active_signature:" + userID
	val := map[string]any{
		"project_id":      projectID,
		"cla_group_id":    projectID,
		"repository_id":   repositoryID,
		"pull_request_id": pullRequestID,
		"user_id":         userID,
	}
	for _, returnURL := range returnURLs {
		returnURL = strings.TrimSpace(returnURL)
		if returnURL == "" {
			continue
		}
		val["return_url"] = returnURL
		break
	}
	b, _ := json.Marshal(val)
	return h.kv.Set(ctx, key, string(b))
}

func (h *Handlers) githubRedirectToConsole(ctx context.Context, installationID, repositoryExternalID, pullRequestID, originURL string, sess middleware.Session, w http.ResponseWriter, r *http.Request) {
	// Resolve repository by external id.
	repoItem, ok, err := h.repos.GetByExternalIDAndType(ctx, repositoryExternalID, "github")
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"repository_id": err.Error()}})
		return
	}
	if !ok {
		// Legacy Python returns None (hug serializes as null).
		respond.JSON(w, http.StatusOK, nil)
		return
	}
	repo := store.ItemToInterfaceMap(repoItem)
	projectID, _ := repo["repository_project_id"].(string)
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		respond.JSON(w, http.StatusOK, nil)
		return
	}

	projectItem, ok, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		// Legacy Python catches the exception and returns an errors object with HTTP 200.
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !ok {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "DoesNotExist"}})
		return
	}
	project := store.ItemToInterfaceMap(projectItem)
	version, _ := project["version"].(string)
	version = strings.TrimSpace(strings.ToLower(version))

	user, herr := h.githubGetOrCreateUser(ctx, sess)
	if herr != nil {
		respond.JSON(w, herr.status, herr.payload)
		return
	}
	userID, _ := user["user_id"].(string)
	if strings.TrimSpace(userID) == "" {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": "missing user_id"})
		return
	}

	// Store active signature metadata (used later by signing flow).
	_ = h.setActiveSignatureMetadata(ctx, userID, projectID, repositoryExternalID, pullRequestID, originURL)

	base := strings.TrimSpace(os.Getenv("CLA_CONTRIBUTOR_BASE"))
	if version == "v2" {
		if b := strings.TrimSpace(os.Getenv("CLA_CONTRIBUTOR_V2_BASE")); b != "" {
			base = b
		}
	}
	base = strings.TrimRight(base, "/")
	consoleURL := "https://" + base + "/#/cla/project/" + projectID + "/user/" + userID
	if strings.TrimSpace(originURL) != "" {
		// Legacy Python does not URL-encode this parameter.
		consoleURL += "?redirect=" + originURL
	}

	http.Redirect(w, r, consoleURL, http.StatusFound)
}

func (h *Handlers) githubSignRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.github == nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": "github service not configured"})
		return
	}

	provider := strings.TrimSpace(strings.ToLower(chi.URLParam(r, "provider")))
	if provider != "github" && provider != "mock_github" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"provider": "invalid provider"}})
		return
	}

	installationID := chi.URLParam(r, "installation_id")
	repoID := chi.URLParam(r, "github_repository_id")
	changeID := chi.URLParam(r, "change_request_id")

	sess := middleware.SessionFromContext(ctx)
	if sess == nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": "session middleware not initialized"})
		return
	}
	// Store session metadata for callback.
	sessionSetString(sess, "github_installation_id", installationID)
	sessionSetString(sess, "github_repository_id", repoID)
	sessionSetString(sess, "github_change_request_id", changeID)
	// Determine origin URL from PR. Python parses all three path IDs as int()
	// before creating the GitHub PR URL/OAuth state; malformed IDs raise and
	// never proceed to OAuth. Keep that fail-fast behavior.
	inst, err := strconv.ParseInt(strings.TrimSpace(installationID), 10, 64)
	if err != nil || inst <= 0 {
		if err == nil {
			err = errors.New("installation_id must be a positive integer")
		}
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": "unable to fetch pull request", "details": err.Error()})
		return
	}
	repo, err := strconv.ParseInt(strings.TrimSpace(repoID), 10, 64)
	if err != nil || repo <= 0 {
		if err == nil {
			err = errors.New("github_repository_id must be a positive integer")
		}
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": "unable to fetch pull request", "details": err.Error()})
		return
	}
	pr, err := strconv.ParseInt(strings.TrimSpace(changeID), 10, 64)
	if err != nil || pr <= 0 {
		if err == nil {
			err = errors.New("change_request_id must be a positive integer")
		}
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": "unable to fetch pull request", "details": err.Error()})
		return
	}
	if origin, err := h.github.GetPullRequestHTMLURL(ctx, inst, repo, pr); err == nil {
		sessionSetString(sess, "github_origin_url", origin)
	} else {
		// Mirror Python: exceptions bubble up as server errors.
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": "unable to fetch pull request", "details": err.Error()})
		return
	}

	if sessionGetMap(sess, "github_oauth2_token") != nil {
		origin := sessionGetString(sess, "github_origin_url")
		h.githubRedirectToConsole(ctx, installationID, repoID, changeID, origin, sess, w, r)
		return
	}

	// Redirect to GitHub OAuth authorize.
	authURL, st, _, err := h.githubAuthURLAndState(nil)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": err.Error()})
		return
	}
	sessionSetString(sess, "github_oauth2_state", st)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func decodeUserFromSessionState(encoded string) (csrf string, value string, err error) {
	decoded, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", err
	}
	var m map[string]string
	if err := json.Unmarshal(decoded, &m); err != nil {
		return "", "", err
	}
	return m["csrf"], m["state"], nil
}

func (h *Handlers) githubOauth2Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.github == nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": "github service not configured"})
		return
	}

	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if state == "" || code == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": "missing state or code"})
		return
	}

	sess := middleware.SessionFromContext(ctx)
	if sess == nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": "session middleware not initialized"})
		return
	}

	sessionState := sessionGetString(sess, "github_oauth2_state")
	if sessionState == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": "Invalid OAuth2 state"})
		return
	}

	clientID := h.githubOAuthClientID()
	clientSecret := h.githubOAuthClientSecret()

	// State mismatch: attempt user-from-session encoded state.
	if state != sessionState {
		csrf, value, err := decodeUserFromSessionState(state)
		if err != nil || value != "user-from-session" {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": "Invalid OAuth2 state", "details": state})
			return
		}
		if csrf != sessionState {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": "Invalid OAuth2 state", "details": state})
			return
		}

		// Exchange token.
		tok, err := h.github.ExchangeOAuthToken(ctx, clientID, clientSecret, code, state)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": "OAuth2 code is invalid or expired"})
			return
		}
		sess["github_oauth2_token"] = tok
		user, herr := h.githubGetOrCreateUser(ctx, sess)
		if herr != nil {
			respond.JSON(w, herr.status, herr.payload)
			return
		}
		respond.JSON(w, http.StatusOK, user)
		return
	}

	// Normal sign_request flow.
	installationID := sessionGetString(sess, "github_installation_id")
	repoID := sessionGetString(sess, "github_repository_id")
	changeID := sessionGetString(sess, "github_change_request_id")
	origin := sessionGetString(sess, "github_origin_url")

	// Exchange token using the stored state (Python uses session state here).
	tok, err := h.github.ExchangeOAuthToken(ctx, clientID, clientSecret, code, sessionState)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": "OAuth2 code is invalid or expired"})
		return
	}
	sess["github_oauth2_token"] = tok
	h.githubRedirectToConsole(ctx, installationID, repoID, changeID, origin, sess, w, r)
}
