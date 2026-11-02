// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	githublegacy "github.com/linuxfoundation/easycla/cla-backend-legacy/internal/legacy/github"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/store"
)

// Offline harness for the GitHub OAuth sign flow (easycla#5165): the real router, session middleware, token
// exchange, /user + /user/emails calls and DynamoDB stores run against stubs, so the Enterprise Managed User
// rejection and the unchanged public-account path are both exercised end-to-end without network or credentials.

const (
	emuSessionID = "unit-sid"
	emuRepoID    = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	emuProjectID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	emuOriginURL = "https://github.com/org-a/repo/pull/3"
)

type githubOAuthHarness struct {
	router http.Handler
	dyn    *fakeDynamo

	mu    sync.Mutex
	calls []string // outbound GitHub requests as "METHOD host/path"
}

func newGitHubOAuthHarness(t *testing.T, login string, githubID int64) *githubOAuthHarness {
	t.Helper()
	dyn := newFakeDynamo()
	dynSrv := httptest.NewServer(dyn)
	t.Cleanup(dynSrv.Close)

	missing := filepath.Join(t.TempDir(), "missing")
	for k, v := range map[string]string{
		"STAGE":                       "unit",
		"AWS_REGION":                  "us-east-1",
		"DYNAMODB_AWS_REGION":         "us-east-1",
		"AWS_ACCESS_KEY_ID":           "unit-access-key",
		"AWS_SECRET_ACCESS_KEY":       "unit-secret-key",
		"AWS_SESSION_TOKEN":           "",
		"AWS_PROFILE":                 "",
		"AWS_CONFIG_FILE":             missing,
		"AWS_SHARED_CREDENTIALS_FILE": missing,
		"AWS_EC2_METADATA_DISABLED":   "true",
		"AWS_ENDPOINT_URL_DYNAMODB":   dynSrv.URL,
		"GITHUB_API_URL":              "https://api.unit.test",
		"GH_OAUTH_CLIENT_ID":          "unit-client",
		"GH_OAUTH_SECRET":             "unit-client-credential",
		"CLA_API_BASE":                "https://api.unit.test",
		"CLA_CONTRIBUTOR_BASE":        "contributor.unit.test",
		"CLA_CONTRIBUTOR_V2_BASE":     "",
	} {
		t.Setenv(k, v)
	}

	hz := &githubOAuthHarness{dyn: dyn}
	httpClient := &http.Client{Transport: stubTransport(func(r *http.Request) (*http.Response, error) {
		hz.mu.Lock()
		hz.calls = append(hz.calls, r.Method+" "+r.URL.Host+r.URL.Path)
		hz.mu.Unlock()
		switch {
		case r.URL.Host == "github.com" && r.URL.Path == "/login/oauth/access_token":
			return stubJSONResponse(r, http.StatusOK, map[string]any{"access_token": "unit-access", "token_type": "bearer", "scope": "user:email"}), nil
		case r.URL.Host == "api.unit.test" && r.URL.Path == "/user":
			return stubJSONResponse(r, http.StatusOK, map[string]any{"id": githubID, "login": login, "name": "Unit User"}), nil
		case r.URL.Host == "api.unit.test" && r.URL.Path == "/user/emails":
			return stubJSONResponse(r, http.StatusOK, []map[string]any{{"email": "unit@example.com", "verified": true, "primary": true}}), nil
		default:
			return nil, fmt.Errorf("unexpected outbound request in test: %s %s", r.Method, r.URL)
		}
	})}

	ctx := context.Background()
	mustStore := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("store init: %v", err)
		}
	}
	h := &Handlers{github: githublegacy.New(httpClient)}
	var err error
	h.users, err = store.NewUsersStoreFromEnv(ctx)
	mustStore(err)
	h.repos, err = store.NewRepositoriesStoreFromEnv(ctx)
	mustStore(err)
	h.projects, err = store.NewProjectsStoreFromEnv(ctx)
	mustStore(err)
	h.kv, err = store.NewKVStoreFromEnv(ctx)
	mustStore(err)

	// GitHub repository 2 of installation 1 belongs to a v1 CLA group; the users table starts empty unless a
	// test seeds it.
	dyn.seed(store.TableName("repositories"), "repository_id", map[string]any{
		"repository_id": dynS(emuRepoID), "repository_external_id": dynS("2"), "repository_type": dynS("github"),
		"repository_project_id": dynS(emuProjectID), "repository_name": dynS("org-a/repo"),
	})
	dyn.seed(store.TableName("projects"), "project_id", map[string]any{
		"project_id": dynS(emuProjectID), "project_name": dynS("Unit Project"), "version": dynS("v1"),
	})
	dyn.mu.Lock()
	dyn.keys[store.TableName("users")] = "user_id"
	dyn.mu.Unlock()

	hz.router = NewRouter(h)
	return hz
}

// seedSession stores the cla-sid session the way the middleware persists it (KV store row, JSON value).
func (hz *githubOAuthHarness) seedSession(t *testing.T, sess map[string]any) {
	t.Helper()
	raw, err := json.Marshal(sess)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	hz.dyn.seed(store.TableName("store"), "key", map[string]any{"key": dynS(emuSessionID), "value": dynS(string(raw))})
}

// signSession is what githubSignRequest leaves behind before redirecting to GitHub for a PR sign link.
func signSession() map[string]any {
	return map[string]any{
		"github_oauth2_state":      "unit-state",
		"github_installation_id":   "1",
		"github_repository_id":     "2",
		"github_change_request_id": "3",
		"github_origin_url":        emuOriginURL,
	}
}

// userFromSessionState is the encoded state the /v2/user-from-session flow sends through GitHub.
func userFromSessionState(csrf string) string {
	raw, _ := json.Marshal(map[string]string{"csrf": csrf, "state": "user-from-session"})
	return base64.URLEncoding.EncodeToString(raw)
}

func (hz *githubOAuthHarness) callback(t *testing.T, state string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v2/github/installation?state="+state+"&code=unit-code", nil)
	req.AddCookie(&http.Cookie{Name: "cla-sid", Value: emuSessionID})
	rec := httptest.NewRecorder()
	hz.router.ServeHTTP(rec, req)
	return rec
}

func (hz *githubOAuthHarness) persistedSession(t *testing.T) map[string]any {
	t.Helper()
	hz.dyn.mu.Lock()
	defer hz.dyn.mu.Unlock()
	i := hz.dyn.findLocked(store.TableName("store"), map[string]any{"key": dynS(emuSessionID)})
	if i < 0 {
		t.Fatalf("session %s was not persisted", emuSessionID)
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(dynStr(hz.dyn.tables[store.TableName("store")][i], "value")), &out); err != nil {
		t.Fatalf("decode persisted session: %v", err)
	}
	return out
}

func (hz *githubOAuthHarness) githubCalls() []string {
	hz.mu.Lock()
	defer hz.mu.Unlock()
	return append([]string(nil), hz.calls...)
}

func decodeRecorderJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return out
}

func assertEMURejected(t *testing.T, hz *githubOAuthHarness, rec *httptest.ResponseRecorder, login string) {
	t.Helper()
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("EMU account must not be redirected to the console, got Location=%s", loc)
	}
	body := decodeRecorderJSON(t, rec)
	if got, _ := body["errors"].(string); !strings.Contains(got, "Enterprise Managed User") {
		t.Fatalf("errors = %q, want the Enterprise Managed User message", got)
	}
	if got, _ := body["details"].(string); !strings.Contains(got, "'"+login+"'") || !strings.Contains(got, "authored the pull request") {
		t.Fatalf("details = %q, want the rejected login and the remediation", got)
	}
	// Nothing is looked up or written for the EMU account, and GitHub is not asked for its e-mails.
	if reads := hz.dyn.readsFor(store.TableName("users")); len(reads) != 0 {
		t.Fatalf("expected no users-table reads, got %+v", reads)
	}
	if writes := hz.dyn.writesFor("PutItem", store.TableName("users")); len(writes) != 0 {
		t.Fatalf("expected no users-table writes, got %+v", writes)
	}
	for _, c := range hz.githubCalls() {
		if strings.HasSuffix(c, "/user/emails") {
			t.Fatalf("GitHub /user/emails must not be called for a rejected EMU login; calls=%v", hz.githubCalls())
		}
	}
	// The OAuth token and state are dropped so the next click on the sign link restarts the GitHub authorization.
	sess := hz.persistedSession(t)
	for _, k := range []string{"github_oauth2_token", "github_oauth2_state"} {
		if _, ok := sess[k]; ok {
			t.Fatalf("persisted session still holds %s: %v", k, sess)
		}
	}
	hz.dyn.mu.Lock()
	defer hz.dyn.mu.Unlock()
	if len(hz.dyn.unknown) != 0 {
		t.Fatalf("fake DynamoDB received unsupported operations: %v", hz.dyn.unknown)
	}
}

func TestGitHubOAuthCallbackRejectsEnterpriseManagedUserOnSignLink(t *testing.T) {
	hz := newGitHubOAuthHarness(t, "octocat_acme", 4242)
	hz.seedSession(t, signSession())

	rec := hz.callback(t, "unit-state")

	assertEMURejected(t, hz, rec, "octocat_acme")
	// The PR context survives for the retry with a public account.
	sess := hz.persistedSession(t)
	if sess["github_change_request_id"] != "3" || sess["github_origin_url"] != emuOriginURL {
		t.Fatalf("sign-link context lost from the session: %v", sess)
	}
	if writes := hz.dyn.writesFor("PutItem", store.TableName("store")); len(writes) != 1 {
		t.Fatalf("expected only the session persistence write on the store table, got %+v", writes)
	}
}

func TestGitHubOAuthCallbackRejectsEnterpriseManagedUserOnUserFromSession(t *testing.T) {
	hz := newGitHubOAuthHarness(t, "octocat_acme", 4242)
	hz.seedSession(t, map[string]any{"github_oauth2_state": "unit-csrf"})

	rec := hz.callback(t, userFromSessionState("unit-csrf"))

	assertEMURejected(t, hz, rec, "octocat_acme")
}

func TestGitHubOAuthCallbackSignLinkStillCreatesPublicAccountUser(t *testing.T) {
	hz := newGitHubOAuthHarness(t, "octocat", 4242)
	hz.seedSession(t, signSession())

	rec := hz.callback(t, "unit-state")

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body=%s", rec.Code, rec.Body.String())
	}
	writes := hz.dyn.writesFor("PutItem", store.TableName("users"))
	if len(writes) != 1 {
		t.Fatalf("expected exactly one users PutItem, got %+v", writes)
	}
	item := writes[0].Item
	userID := dynStr(item, "user_id")
	if userID == "" {
		t.Fatalf("created user has no user_id: %+v", item)
	}
	if got := dynStr(item, "user_github_username"); got != "octocat" {
		t.Fatalf("user_github_username = %q, want octocat", got)
	}
	if gid, _ := item["user_github_id"].(map[string]any); gid["N"] != "4242" {
		t.Fatalf("user_github_id = %v, want N 4242", item["user_github_id"])
	}
	if emails, _ := item["user_emails"].(map[string]any); fmt.Sprint(emails["SS"]) != "[unit@example.com]" {
		t.Fatalf("user_emails = %v, want [unit@example.com]", item["user_emails"])
	}
	want := "https://contributor.unit.test/#/cla/project/" + emuProjectID + "/user/" + userID + "?redirect=" + emuOriginURL
	if loc := rec.Header().Get("Location"); loc != want {
		t.Fatalf("Location = %s, want %s", loc, want)
	}
	// github-id-index, lf-email-index, then the user_emails scan found nothing before the create.
	if reads := hz.dyn.readsFor(store.TableName("users")); len(reads) != 3 {
		t.Fatalf("expected 3 users-table reads (2 queries + scan), got %+v", reads)
	}
	if active := hz.dyn.writesFor("PutItem", store.TableName("store")); len(active) != 2 {
		t.Fatalf("expected the active_signature write plus the session persistence on the store table, got %+v", active)
	}
	hz.dyn.mu.Lock()
	defer hz.dyn.mu.Unlock()
	if len(hz.dyn.unknown) != 0 {
		t.Fatalf("fake DynamoDB received unsupported operations: %v", hz.dyn.unknown)
	}
}

func TestGitHubOAuthCallbackSignLinkStillUpdatesExistingPublicAccountUser(t *testing.T) {
	hz := newGitHubOAuthHarness(t, "octocat", 4242)
	hz.seedSession(t, signSession())
	hz.dyn.seed(store.TableName("users"), "user_id", map[string]any{
		"user_id": dynS(authzUserAlice), "lf_username": dynS("alice"), "user_github_id": map[string]any{"N": "4242"},
	})

	rec := hz.callback(t, "unit-state")

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body=%s", rec.Code, rec.Body.String())
	}
	writes := hz.dyn.writesFor("PutItem", store.TableName("users"))
	if len(writes) != 1 || dynStr(writes[0].Item, "user_id") != authzUserAlice {
		t.Fatalf("expected one PutItem updating %s, got %+v", authzUserAlice, writes)
	}
	if dynStr(writes[0].Item, "lf_username") != "alice" || dynStr(writes[0].Item, "user_github_username") != "octocat" {
		t.Fatalf("update must keep lf_username and set user_github_username, got %+v", writes[0].Item)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "/user/"+authzUserAlice+"?") {
		t.Fatalf("Location = %s, want the existing user id", loc)
	}
}

func TestIsEnterpriseManagedLogin(t *testing.T) {
	for login, want := range map[string]bool{
		"octocat":      false,
		"octo-cat":     false,
		"octocat_acme": true,
		"_acme":        true,
		"":             false,
	} {
		if got := isEnterpriseManagedLogin(login); got != want {
			t.Errorf("isEnterpriseManagedLogin(%q) = %v, want %v", login, got, want)
		}
	}
}
