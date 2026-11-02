// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v4"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/auth"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/legacy/lfgroup"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/store"
)

// Offline harness for the legacy write-endpoint authorization guards (lfx-self-serve-ops#738, #745, #773,
// #774, #775, #777): Auth0 JWKS is served from an in-memory RSA key through the validator's injectable HTTP
// client, and DynamoDB is a fake JSON-protocol server reached through AWS_ENDPOINT_URL_DYNAMODB, so the real
// store code, router and handlers run without network or credentials.

const (
	authzSFIDA      = "a0941000002wBz4AAE"
	authzSFIDB      = "a0941000002wBz5AAE"
	authzCLAGroupA  = "11111111-1111-4111-8111-111111111111"
	authzCLAGroupB  = "22222222-2222-4222-8222-222222222222"
	authzCCLAID     = "33333333-3333-4333-8333-333333333333"
	authzICLAID     = "44444444-4444-4444-8444-444444444444"
	authzGerritICLA = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	authzRepoID     = "55555555-5555-4555-8555-555555555555"
	authzRepoNoSFID = "66666666-6666-4666-8666-666666666666"
	authzRepoV4     = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	authzGerritID   = "77777777-7777-4777-8777-777777777777"
	authzCompanyID  = "88888888-8888-4888-8888-888888888888"
	authzUserAlice  = "99999999-9999-4999-8999-999999999999"
	authzUnknownID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

type fakeDynamoWrite struct {
	Op    string
	Table string
	Item  map[string]any
	Key   map[string]any
}

type fakeDynamoRead struct {
	Op    string
	Table string
}

// fakeDynamo is a minimal DynamoDB JSON-protocol server: GetItem/PutItem/DeleteItem on items stored as
// DynamoDB-JSON maps (attribute -> {"S": ...}), plus Query/Scan restricted to the expression shapes the legacy
// stores use: `attr = :v` key conditions and an optional `attr = :v` / `contains(attr, :v)` filter.
type fakeDynamo struct {
	mu      sync.Mutex
	keys    map[string]string
	tables  map[string][]map[string]any
	writes  []fakeDynamoWrite
	reads   []fakeDynamoRead
	unknown []string
}

func newFakeDynamo() *fakeDynamo {
	return &fakeDynamo{keys: map[string]string{}, tables: map[string][]map[string]any{}}
}

func (f *fakeDynamo) seed(table, keyAttr string, item map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys[table] = keyAttr
	f.tables[table] = append(f.tables[table], item)
}

func (f *fakeDynamo) resetWrites() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes = nil
}

func (f *fakeDynamo) writesFor(op, table string) []fakeDynamoWrite {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []fakeDynamoWrite
	for _, w := range f.writes {
		if w.Op == op && w.Table == table {
			out = append(out, w)
		}
	}
	return out
}

func (f *fakeDynamo) readsFor(table string) []fakeDynamoRead {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []fakeDynamoRead
	for _, r := range f.reads {
		if r.Table == table {
			out = append(out, r)
		}
	}
	return out
}

var (
	fakeDynamoEqualityExpr = regexp.MustCompile(`^\s*([A-Za-z0-9_]+)\s*=\s*(:[A-Za-z0-9_]+)\s*$`)
	fakeDynamoContainsExpr = regexp.MustCompile(`^\s*contains\(\s*([A-Za-z0-9_]+)\s*,\s*(:[A-Za-z0-9_]+)\s*\)\s*$`)
)

// matchesExprLocked evaluates one `attr = :v` or `contains(attr, :v)` expression against a DynamoDB-JSON item;
// an empty expression matches everything, an unsupported one matches nothing (and is recorded as unknown).
func (f *fakeDynamo) matchesExprLocked(item map[string]any, expr string, values map[string]any) bool {
	if strings.TrimSpace(expr) == "" {
		return true
	}
	if m := fakeDynamoEqualityExpr.FindStringSubmatch(expr); m != nil {
		return reflect.DeepEqual(item[m[1]], values[m[2]])
	}
	if m := fakeDynamoContainsExpr.FindStringSubmatch(expr); m != nil {
		want, _ := values[m[2]].(map[string]any)
		needle, _ := want["S"].(string)
		attr, _ := item[m[1]].(map[string]any)
		if set, ok := attr["SS"].([]any); ok {
			for _, v := range set {
				if s, _ := v.(string); s == needle {
					return true
				}
			}
			return false
		}
		if s, ok := attr["S"].(string); ok {
			return strings.Contains(s, needle)
		}
		return false
	}
	f.unknown = append(f.unknown, "expression:"+expr)
	return false
}

func (f *fakeDynamo) findLocked(table string, key map[string]any) int {
	for i, item := range f.tables[table] {
		match := true
		for k, v := range key {
			if !reflect.DeepEqual(item[k], v) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func (f *fakeDynamo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TableName                 string         `json:"TableName"`
		Key                       map[string]any `json:"Key"`
		Item                      map[string]any `json:"Item"`
		KeyConditionExpression    string         `json:"KeyConditionExpression"`
		FilterExpression          string         `json:"FilterExpression"`
		ExpressionAttributeValues map[string]any `json:"ExpressionAttributeValues"`
	}
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &req)
	op := strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "DynamoDB_20120810.")

	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	switch op {
	case "GetItem":
		if i := f.findLocked(req.TableName, req.Key); i >= 0 {
			_ = json.NewEncoder(w).Encode(map[string]any{"Item": f.tables[req.TableName][i]})
			return
		}
		_, _ = w.Write([]byte("{}"))
	case "Query", "Scan":
		f.reads = append(f.reads, fakeDynamoRead{Op: op, Table: req.TableName})
		items := []map[string]any{}
		for _, item := range f.tables[req.TableName] {
			if f.matchesExprLocked(item, req.KeyConditionExpression, req.ExpressionAttributeValues) &&
				f.matchesExprLocked(item, req.FilterExpression, req.ExpressionAttributeValues) {
				items = append(items, item)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Items": items, "Count": len(items), "ScannedCount": len(f.tables[req.TableName])})
	case "PutItem":
		f.writes = append(f.writes, fakeDynamoWrite{Op: op, Table: req.TableName, Item: req.Item})
		keyAttr := f.keys[req.TableName]
		if keyAttr != "" {
			if i := f.findLocked(req.TableName, map[string]any{keyAttr: req.Item[keyAttr]}); i >= 0 {
				f.tables[req.TableName][i] = req.Item
			} else {
				f.tables[req.TableName] = append(f.tables[req.TableName], req.Item)
			}
		}
		_, _ = w.Write([]byte("{}"))
	case "DeleteItem":
		f.writes = append(f.writes, fakeDynamoWrite{Op: op, Table: req.TableName, Key: req.Key})
		if i := f.findLocked(req.TableName, req.Key); i >= 0 {
			f.tables[req.TableName] = append(f.tables[req.TableName][:i], f.tables[req.TableName][i+1:]...)
		}
		_, _ = w.Write([]byte("{}"))
	default:
		f.unknown = append(f.unknown, op)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"__type":"com.amazonaws.dynamodb.v20120810#ValidationException","message":"unsupported operation in fake"}`))
	}
}

func dynS(v string) map[string]any     { return map[string]any{"S": v} }
func dynBool(v bool) map[string]any    { return map[string]any{"BOOL": v} }
func dynSS(v ...string) map[string]any { return map[string]any{"SS": v} }
func dynStr(item map[string]any, attr string) string {
	m, _ := item[attr].(map[string]any)
	s, _ := m["S"].(string)
	return s
}

type stubTransport func(*http.Request) (*http.Response, error)

func (f stubTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func stubJSONResponse(r *http.Request, status int, body any) *http.Response {
	raw, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(raw)),
		Request:    r,
	}
}

type offlineAuth struct {
	key *rsa.PrivateKey
	kid string
}

func newOfflineAuth(t *testing.T) *offlineAuth {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return &offlineAuth{key: key, kid: "unit-kid"}
}

func (a *offlineAuth) jwks() map[string]any {
	return map[string]any{"keys": []map[string]any{{
		"kty": "RSA",
		"kid": a.kid,
		"use": "sig",
		"n":   base64.RawURLEncoding.EncodeToString(a.key.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(a.key.PublicKey.E)).Bytes()),
	}}}
}

func (a *offlineAuth) bearer(t *testing.T, username string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"username": username,
		"name":     "Unit " + username,
		"email":    username + "@example.com",
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = a.kid
	signed, err := tok.SignedString(a.key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return "Bearer " + signed
}

type legacyAuthzHarness struct {
	router http.Handler
	dyn    *fakeDynamo
	auth   *offlineAuth
}

func newLegacyAuthzHarness(t *testing.T) *legacyAuthzHarness {
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
		"AUTH0_DOMAIN":                "auth.unit.test",
		"AUTH0_ALGORITHM":             "RS256",
		"AUTH0_USERNAME_CLAIM":        "username",
		"LF_GROUP_CLIENT_URL":         "https://lfgroup.unit.test",
		"LF_GROUP_CLIENT_ID":          "unit-client",
		"LF_GROUP_CLIENT_SECRET":      "unit-client-credential",
		"LF_GROUP_REFRESH_TOKEN":      "unit-refresh",
	} {
		t.Setenv(k, v)
	}

	oa := newOfflineAuth(t)
	httpClient := &http.Client{Transport: stubTransport(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Host == "auth.unit.test" && strings.HasSuffix(r.URL.Path, "/.well-known/jwks.json"):
			return stubJSONResponse(r, http.StatusOK, oa.jwks()), nil
		case r.URL.Host == "lfgroup.unit.test" && strings.HasSuffix(r.URL.Path, "/oauth2/token"):
			return stubJSONResponse(r, http.StatusOK, map[string]any{"access_token": "unit-access"}), nil
		case r.URL.Host == "lfgroup.unit.test" && strings.Contains(r.URL.Path, "/rest/auth0/og/"):
			return stubJSONResponse(r, http.StatusOK, map[string]any{"title": "Unit LDAP Group"}), nil
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
	h := &Handlers{authValidator: auth.NewAuth0ValidatorFromEnv(httpClient), lfGroup: lfgroup.NewFromEnv(httpClient)}
	var err error
	h.userPerms, err = store.NewUserPermissionsStoreFromEnv(ctx)
	mustStore(err)
	h.users, err = store.NewUsersStoreFromEnv(ctx)
	mustStore(err)
	h.companies, err = store.NewCompaniesStoreFromEnv(ctx)
	mustStore(err)
	h.repos, err = store.NewRepositoriesStoreFromEnv(ctx)
	mustStore(err)
	h.signatures, err = store.NewSignaturesStoreFromEnv(ctx)
	mustStore(err)
	h.projects, err = store.NewProjectsStoreFromEnv(ctx)
	mustStore(err)
	h.gerritInstances, err = store.NewGerritInstancesStoreFromEnv(ctx)
	mustStore(err)
	h.githubOrgs, err = store.NewGitHubOrgsStoreFromEnv(ctx)
	mustStore(err)

	// Two Salesforce projects (A, B), each with one CLA group; "manager-a"/"manager-b" hold user permissions for
	// one project, "manager-ab" for both; "pm-a" is in CLA group A's project_acl; "cla-manager" is the company's
	// CCLA manager (company_acl + signature_acl); "alice" owns the ICLA; "outsider" has nothing.
	dyn.seed(store.TableName("projects"), "project_id", map[string]any{
		"project_id": dynS(authzCLAGroupA), "project_external_id": dynS(authzSFIDA), "project_name": dynS("Project A"), "project_acl": dynSS("pm-a"),
	})
	dyn.seed(store.TableName("projects"), "project_id", map[string]any{
		"project_id": dynS(authzCLAGroupB), "project_external_id": dynS(authzSFIDB), "project_name": dynS("Project B"), "project_acl": dynSS("pm-b"),
	})
	dyn.seed(store.TableName("user-permissions"), "username", map[string]any{"username": dynS("manager-a"), "projects": dynSS(authzSFIDA)})
	dyn.seed(store.TableName("user-permissions"), "username", map[string]any{"username": dynS("manager-b"), "projects": dynSS(authzSFIDB)})
	dyn.seed(store.TableName("user-permissions"), "username", map[string]any{"username": dynS("manager-ab"), "projects": dynSS(authzSFIDA, authzSFIDB)})
	dyn.seed(store.TableName("users"), "user_id", map[string]any{"user_id": dynS(authzUserAlice), "lf_username": dynS("alice"), "user_name": dynS("Alice")})
	dyn.seed(store.TableName("companies"), "company_id", map[string]any{
		"company_id": dynS(authzCompanyID), "company_name": dynS("Acme"), "company_acl": dynSS("cla-manager", "vnaidu"), "is_sanctioned": dynBool(true),
	})
	dyn.seed(store.TableName("signatures"), "signature_id", map[string]any{
		"signature_id": dynS(authzCCLAID), "signature_reference_type": dynS("company"), "signature_reference_id": dynS(authzCompanyID),
		"signature_project_id": dynS(authzCLAGroupA), "signature_type": dynS("ccla"), "signature_acl": dynSS("cla-manager"),
		"signature_signed": dynBool(true), "signature_approved": dynBool(true),
	})
	dyn.seed(store.TableName("signatures"), "signature_id", map[string]any{
		"signature_id": dynS(authzICLAID), "signature_reference_type": dynS("user"), "signature_reference_id": dynS(authzUserAlice),
		"signature_project_id": dynS(authzCLAGroupA), "signature_type": dynS("cla"),
		"signature_signed": dynBool(true), "signature_approved": dynBool(true),
	})
	// invalidated Gerrit-style ICLA: v2 stores the signer's own LF username in signature_acl (v2/sign/service.go)
	dyn.seed(store.TableName("signatures"), "signature_id", map[string]any{
		"signature_id": dynS(authzGerritICLA), "signature_reference_type": dynS("user"), "signature_reference_id": dynS(authzUserAlice),
		"signature_project_id": dynS(authzCLAGroupA), "signature_type": dynS("cla"), "signature_acl": dynSS("alice"),
		"signature_signed": dynBool(true), "signature_approved": dynBool(false),
	})
	dyn.seed(store.TableName("repositories"), "repository_id", map[string]any{
		"repository_id": dynS(authzRepoID), "repository_project_id": dynS(authzCLAGroupA), "repository_sfdc_id": dynS(authzSFIDA), "project_sfid": dynS(authzSFIDA),
		"repository_name": dynS("org-a/repo"), "repository_type": dynS("github"), "repository_url": dynS("https://github.com/org-a/repo"), "enabled": dynBool(true),
	})
	dyn.seed(store.TableName("repositories"), "repository_id", map[string]any{
		"repository_id": dynS(authzRepoNoSFID), "repository_project_id": dynS(authzCLAGroupA),
		"repository_name": dynS("org-a/legacy"), "repository_type": dynS("github"), "repository_url": dynS("https://github.com/org-a/legacy"),
	})
	// v4-registered row: the parent foundation (A) sits in repository_sfdc_id, the owning child project (B) in project_sfid
	dyn.seed(store.TableName("repositories"), "repository_id", map[string]any{
		"repository_id": dynS(authzRepoV4), "repository_project_id": dynS(authzCLAGroupB), "repository_sfdc_id": dynS(authzSFIDA), "project_sfid": dynS(authzSFIDB),
		"repository_name": dynS("org-b/repo"), "repository_type": dynS("github"), "repository_url": dynS("https://github.com/org-b/repo"), "enabled": dynBool(true),
	})
	dyn.seed(store.TableName("gerrit-instances"), "gerrit_id", map[string]any{
		"gerrit_id": dynS(authzGerritID), "project_id": dynS(authzCLAGroupA), "gerrit_name": dynS("Gerrit A"), "gerrit_url": dynS("https://gerrit.a.example.org"),
	})
	dyn.seed(store.TableName("github-orgs"), "organization_name", map[string]any{
		"organization_name": dynS("org-b"), "organization_name_lower": dynS("org-b"), "organization_sfid": dynS(authzSFIDB), "project_sfid": dynS(authzSFIDB),
	})

	r := chi.NewRouter()
	RegisterRoutes(r, h)
	return &legacyAuthzHarness{router: r, dyn: dyn, auth: oa}
}

// call sends method+path with an optional JSON body; username "" means anonymous.
func (hz *legacyAuthzHarness) call(t *testing.T, username, method, path string, body map[string]any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if username != "" {
		req.Header.Set("Authorization", hz.auth.bearer(t, username))
	}
	rec := httptest.NewRecorder()
	hz.router.ServeHTTP(rec, req)
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func (hz *legacyAuthzHarness) assertNoWrites(t *testing.T) {
	t.Helper()
	hz.dyn.mu.Lock()
	defer hz.dyn.mu.Unlock()
	if len(hz.dyn.writes) != 0 {
		t.Fatalf("expected no DynamoDB writes, got %+v", hz.dyn.writes)
	}
	if len(hz.dyn.unknown) != 0 {
		t.Fatalf("fake DynamoDB received unsupported operations: %v", hz.dyn.unknown)
	}
}

func (hz *legacyAuthzHarness) assertSingleWrite(t *testing.T, op, tableSuffix string) fakeDynamoWrite {
	t.Helper()
	writes := hz.dyn.writesFor(op, store.TableName(tableSuffix))
	if len(writes) != 1 {
		t.Fatalf("expected exactly one %s on %s, got %+v (all writes: %+v)", op, tableSuffix, writes, hz.dyn.writes)
	}
	return writes[0]
}

func assertInsufficientPermissions(t *testing.T, code int, body map[string]any) {
	t.Helper()
	if code != http.StatusForbidden || body["title"] != "Unauthorized" {
		t.Fatalf("expected legacy 403 insufficient-permissions reply, got %d %v", code, body)
	}
}

func assertNotAuthorizedForSFID(t *testing.T, code int, body map[string]any, sfid string) {
	t.Helper()
	errs, _ := body["errors"].(map[string]any)
	if code != http.StatusForbidden || errs["user is not authorized for this Salesforce ID."] != sfid {
		t.Fatalf("expected 403 for SFID %s, got %d %v", sfid, code, body)
	}
}

func TestLegacyWriteGuards_UnauthenticatedRequestsAreRejected(t *testing.T) {
	hz := newLegacyAuthzHarness(t)
	cases := []struct {
		name, method, path string
		body               map[string]any
	}{
		{"create gerrit (#774)", http.MethodPost, "/v1/gerrit", map[string]any{"project_id": authzCLAGroupA, "gerrit_name": "g", "gerrit_url": "https://gerrit.example.org", "group_id_icla": "g1"}},
		{"delete gerrit (#774)", http.MethodDelete, "/v1/gerrit/" + authzGerritID, nil},
		{"put repository (#777)", http.MethodPut, "/v1/repository", map[string]any{"repository_id": authzRepoID, "repository_name": "x"}},
		{"delete repository (#777)", http.MethodDelete, "/v1/repository/" + authzRepoID, nil},
		{"put signature (#775)", http.MethodPut, "/v1/signature", map[string]any{"signature_id": authzCCLAID, "signature_approved": false}},
		{"delete signature (#738)", http.MethodDelete, "/v1/signature/" + authzCCLAID, nil},
		{"put company (#745)", http.MethodPut, "/v1/company", map[string]any{"company_id": authzCompanyID, "is_sanctioned": false}},
		{"post github org (#773)", http.MethodPost, "/v1/github/organizations", map[string]any{"organization_name": "org-b", "organization_sfid": authzSFIDA}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hz.dyn.resetWrites()
			code, _ := hz.call(t, "", tc.method, tc.path, tc.body)
			if code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", code)
			}
			hz.assertNoWrites(t)
		})
	}
}

func TestLegacyGerritInstanceGuards(t *testing.T) {
	hz := newLegacyAuthzHarness(t)
	createBody := func(projectID string) map[string]any {
		return map[string]any{"project_id": projectID, "gerrit_name": "Gerrit", "gerrit_url": "https://gerrit.example.org", "group_id_icla": "ldap-icla"}
	}

	t.Run("create: manager of another project is forbidden", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-b", http.MethodPost, "/v1/gerrit", createBody(authzCLAGroupA))
		assertNotAuthorizedForSFID(t, code, body, authzSFIDA)
		hz.assertNoWrites(t)
	})
	t.Run("create: unknown CLA group is not writable", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-a", http.MethodPost, "/v1/gerrit", createBody(authzUnknownID))
		errs, _ := body["errors"].(map[string]any)
		if code != http.StatusOK || errs["project_id"] != "Project not found" {
			t.Fatalf("expected legacy project-not-found reply, got %d %v", code, body)
		}
		hz.assertNoWrites(t)
	})
	t.Run("create: project manager succeeds", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-a", http.MethodPost, "/v1/gerrit", createBody(authzCLAGroupA))
		if code != http.StatusOK || body["gerrit_id"] == "" || body["gerrit_id"] == nil {
			t.Fatalf("expected created gerrit instance, got %d %v", code, body)
		}
		w := hz.assertSingleWrite(t, "PutItem", "gerrit-instances")
		if dynStr(w.Item, "project_id") != authzCLAGroupA {
			t.Fatalf("unexpected stored gerrit item: %v", w.Item)
		}
	})
	t.Run("delete: manager of another project is forbidden", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-b", http.MethodDelete, "/v1/gerrit/"+authzGerritID, nil)
		assertNotAuthorizedForSFID(t, code, body, authzSFIDA)
		hz.assertNoWrites(t)
	})
	t.Run("delete: LF admin bypasses project permissions", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "ddeal", http.MethodDelete, "/v1/gerrit/"+authzGerritID, nil)
		if code != http.StatusOK || body["success"] != true {
			t.Fatalf("expected success, got %d %v", code, body)
		}
		w := hz.assertSingleWrite(t, "DeleteItem", "gerrit-instances")
		if dynStr(w.Key, "gerrit_id") != authzGerritID {
			t.Fatalf("unexpected delete key: %v", w.Key)
		}
	})
}

func TestLegacyRepositoryGuards(t *testing.T) {
	hz := newLegacyAuthzHarness(t)

	t.Run("put: manager of another project is forbidden", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-b", http.MethodPut, "/v1/repository", map[string]any{"repository_id": authzRepoID, "repository_name": "renamed"})
		assertNotAuthorizedForSFID(t, code, body, authzSFIDA)
		hz.assertNoWrites(t)
	})
	t.Run("put: owning project manager succeeds", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-a", http.MethodPut, "/v1/repository", map[string]any{"repository_id": authzRepoID, "repository_name": "renamed"})
		if code != http.StatusOK || body["repository_name"] != "renamed" {
			t.Fatalf("expected updated repository, got %d %v", code, body)
		}
		w := hz.assertSingleWrite(t, "PutItem", "repositories")
		if dynStr(w.Item, "repository_name") != "renamed" || dynStr(w.Item, "repository_project_id") != authzCLAGroupA {
			t.Fatalf("unexpected stored repository: %v", w.Item)
		}
	})
	t.Run("put: re-pointing needs the target project too", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-a", http.MethodPut, "/v1/repository", map[string]any{"repository_id": authzRepoID, "repository_project_id": authzCLAGroupB})
		assertNotAuthorizedForSFID(t, code, body, authzSFIDB)
		hz.assertNoWrites(t)

		code, body = hz.call(t, "manager-ab", http.MethodPut, "/v1/repository", map[string]any{"repository_id": authzRepoID, "repository_project_id": authzCLAGroupB})
		if code != http.StatusOK || body["repository_project_id"] != authzCLAGroupB {
			t.Fatalf("expected re-pointed repository, got %d %v", code, body)
		}
		hz.assertSingleWrite(t, "PutItem", "repositories")
	})
	t.Run("put: v4 row is owned by project_sfid, not the foundation in repository_sfdc_id", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-a", http.MethodPut, "/v1/repository", map[string]any{"repository_id": authzRepoV4, "repository_name": "foundation-renamed"})
		assertNotAuthorizedForSFID(t, code, body, authzSFIDB)
		hz.assertNoWrites(t)

		code, body = hz.call(t, "manager-b", http.MethodPut, "/v1/repository", map[string]any{"repository_id": authzRepoV4, "repository_name": "child-renamed"})
		if code != http.StatusOK || body["repository_name"] != "child-renamed" {
			t.Fatalf("expected child project manager to update the repository, got %d %v", code, body)
		}
		hz.assertSingleWrite(t, "PutItem", "repositories")
	})
	t.Run("put: record without SFID resolves its CLA group's project", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-b", http.MethodPut, "/v1/repository", map[string]any{"repository_id": authzRepoNoSFID, "repository_name": "legacy-renamed"})
		assertNotAuthorizedForSFID(t, code, body, authzSFIDA)
		hz.assertNoWrites(t)

		code, body = hz.call(t, "manager-a", http.MethodPut, "/v1/repository", map[string]any{"repository_id": authzRepoNoSFID, "repository_name": "legacy-renamed"})
		if code != http.StatusOK || body["repository_name"] != "legacy-renamed" {
			t.Fatalf("expected updated legacy repository, got %d %v", code, body)
		}
		hz.assertSingleWrite(t, "PutItem", "repositories")
	})
	t.Run("delete: manager of another project is forbidden", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-b", http.MethodDelete, "/v1/repository/"+authzRepoNoSFID, nil)
		assertNotAuthorizedForSFID(t, code, body, authzSFIDA)
		hz.assertNoWrites(t)
	})
	t.Run("delete: owning project manager succeeds", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-a", http.MethodDelete, "/v1/repository/"+authzRepoNoSFID, nil)
		if code != http.StatusOK || body["success"] != true {
			t.Fatalf("expected success, got %d %v", code, body)
		}
		w := hz.assertSingleWrite(t, "DeleteItem", "repositories")
		if dynStr(w.Key, "repository_id") != authzRepoNoSFID {
			t.Fatalf("unexpected delete key: %v", w.Key)
		}
	})
}

func TestLegacySignatureGuards(t *testing.T) {
	hz := newLegacyAuthzHarness(t)
	approveFalse := func(id string) map[string]any { return map[string]any{"signature_id": id, "signature_approved": false} }

	t.Run("put: unrelated user cannot rewrite a company signature", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "outsider", http.MethodPut, "/v1/signature", approveFalse(authzCCLAID))
		assertInsufficientPermissions(t, code, body)
		hz.assertNoWrites(t)
	})
	t.Run("put: unrelated user cannot rewrite an individual signature", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "outsider", http.MethodPut, "/v1/signature", approveFalse(authzICLAID))
		assertInsufficientPermissions(t, code, body)
		hz.assertNoWrites(t)
	})
	t.Run("put: signature ACL member succeeds", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "cla-manager", http.MethodPut, "/v1/signature", approveFalse(authzCCLAID))
		if code != http.StatusOK || body["signature_approved"] != false {
			t.Fatalf("expected updated signature, got %d %v", code, body)
		}
		hz.assertSingleWrite(t, "PutItem", "signatures")
	})
	t.Run("put: CLA group manager succeeds", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, _ := hz.call(t, "pm-a", http.MethodPut, "/v1/signature", approveFalse(authzCCLAID))
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d", code)
		}
		hz.assertSingleWrite(t, "PutItem", "signatures")
	})
	t.Run("put: referenced user may not touch their own individual signature", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "alice", http.MethodPut, "/v1/signature", approveFalse(authzICLAID))
		assertInsufficientPermissions(t, code, body)
		hz.assertNoWrites(t)
	})
	t.Run("put: signer listed in their own ICLA's signature_acl cannot re-approve it", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "alice", http.MethodPut, "/v1/signature", map[string]any{"signature_id": authzGerritICLA, "signature_approved": true})
		assertInsufficientPermissions(t, code, body)
		hz.assertNoWrites(t)
	})
	t.Run("put: CLA group manager may re-approve an individual signature", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "pm-a", http.MethodPut, "/v1/signature", map[string]any{"signature_id": authzGerritICLA, "signature_approved": true})
		if code != http.StatusOK || body["signature_approved"] != true {
			t.Fatalf("expected re-approved signature, got %d %v", code, body)
		}
		hz.assertSingleWrite(t, "PutItem", "signatures")
	})
	t.Run("put: signature ACL member cannot re-point the signature", func(t *testing.T) {
		for _, change := range []map[string]any{
			{"signature_id": authzCCLAID, "signature_project_id": authzCLAGroupB},
			{"signature_id": authzCCLAID, "signature_reference_id": authzUserAlice},
			{"signature_id": authzCCLAID, "signature_reference_type": "user"},
		} {
			hz.dyn.resetWrites()
			code, body := hz.call(t, "cla-manager", http.MethodPut, "/v1/signature", change)
			assertInsufficientPermissions(t, code, body)
			hz.assertNoWrites(t)
		}
		// re-sending the stored values is not a re-point
		hz.dyn.resetWrites()
		code, _ := hz.call(t, "cla-manager", http.MethodPut, "/v1/signature", map[string]any{"signature_id": authzCCLAID, "signature_project_id": authzCLAGroupA, "signature_approved": false})
		if code != http.StatusOK {
			t.Fatalf("expected 200 for unchanged project id, got %d", code)
		}
		hz.assertSingleWrite(t, "PutItem", "signatures")
	})
	t.Run("put: LF admin may re-point the signature", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "vnaidu", http.MethodPut, "/v1/signature", map[string]any{"signature_id": authzCCLAID, "signature_project_id": authzCLAGroupB})
		if code != http.StatusOK || body["signature_project_id"] != authzCLAGroupB {
			t.Fatalf("expected re-pointed signature, got %d %v", code, body)
		}
		hz.assertSingleWrite(t, "PutItem", "signatures")
		hz.dyn.resetWrites()
		if code, _ := hz.call(t, "vnaidu", http.MethodPut, "/v1/signature", map[string]any{"signature_id": authzCCLAID, "signature_project_id": authzCLAGroupA}); code != http.StatusOK {
			t.Fatalf("restore failed: %d", code)
		}
	})
	t.Run("delete: referenced user may not delete their own individual signature", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "alice", http.MethodDelete, "/v1/signature/"+authzICLAID, nil)
		assertInsufficientPermissions(t, code, body)
		hz.assertNoWrites(t)
	})
	t.Run("delete: signer listed in their own ICLA's signature_acl may not delete it", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "alice", http.MethodDelete, "/v1/signature/"+authzGerritICLA, nil)
		assertInsufficientPermissions(t, code, body)
		hz.assertNoWrites(t)
	})
	t.Run("delete: unrelated user is forbidden and the record stays", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "outsider", http.MethodDelete, "/v1/signature/"+authzCCLAID, nil)
		assertInsufficientPermissions(t, code, body)
		hz.assertNoWrites(t)
		if code, _ := hz.call(t, "cla-manager", http.MethodPut, "/v1/signature", approveFalse(authzCCLAID)); code != http.StatusOK {
			t.Fatalf("signature should still exist after the rejected delete, got %d", code)
		}
	})
	t.Run("delete: signature ACL member succeeds", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "cla-manager", http.MethodDelete, "/v1/signature/"+authzCCLAID, nil)
		if code != http.StatusOK || body["success"] != true {
			t.Fatalf("expected success, got %d %v", code, body)
		}
		w := hz.assertSingleWrite(t, "DeleteItem", "signatures")
		if dynStr(w.Key, "signature_id") != authzCCLAID {
			t.Fatalf("unexpected delete key: %v", w.Key)
		}
	})
	t.Run("delete: LF admin may delete an individual signature", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "vnaidu", http.MethodDelete, "/v1/signature/"+authzICLAID, nil)
		if code != http.StatusOK || body["success"] != true {
			t.Fatalf("expected success, got %d %v", code, body)
		}
		hz.assertSingleWrite(t, "DeleteItem", "signatures")
	})
}

func TestLegacyPutCompanySanctionGuard(t *testing.T) {
	hz := newLegacyAuthzHarness(t)

	t.Run("company ACL member cannot lift the sanction block", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "cla-manager", http.MethodPut, "/v1/company", map[string]any{"company_id": authzCompanyID, "is_sanctioned": false})
		assertInsufficientPermissions(t, code, body)
		hz.assertNoWrites(t)
	})
	t.Run("company ACL member can still edit other fields", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "cla-manager", http.MethodPut, "/v1/company", map[string]any{"company_id": authzCompanyID, "company_name": "Acme Inc"})
		if code != http.StatusOK || body["company_name"] != "Acme Inc" {
			t.Fatalf("expected updated company, got %d %v", code, body)
		}
		w := hz.assertSingleWrite(t, "PutItem", "companies")
		if sanctioned, _ := w.Item["is_sanctioned"].(map[string]any); sanctioned["BOOL"] != true {
			t.Fatalf("sanction flag must be untouched, stored item: %v", w.Item)
		}
	})
	t.Run("outsider is still rejected by the company ACL", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "outsider", http.MethodPut, "/v1/company", map[string]any{"company_id": authzCompanyID, "company_name": "Evil"})
		assertInsufficientPermissions(t, code, body)
		hz.assertNoWrites(t)
	})
	t.Run("LF admin in the ACL may change the sanction flag", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "vnaidu", http.MethodPut, "/v1/company", map[string]any{"company_id": authzCompanyID, "is_sanctioned": false})
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d %v", code, body)
		}
		w := hz.assertSingleWrite(t, "PutItem", "companies")
		if sanctioned, _ := w.Item["is_sanctioned"].(map[string]any); sanctioned["BOOL"] != false {
			t.Fatalf("expected sanction flag cleared, stored item: %v", w.Item)
		}
	})
}

func TestLegacyPostGithubOrganizationRepointGuard(t *testing.T) {
	hz := newLegacyAuthzHarness(t)

	t.Run("manager of project A cannot re-point project B's org", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-a", http.MethodPost, "/v1/github/organizations", map[string]any{"organization_name": "org-b", "organization_sfid": authzSFIDA})
		assertNotAuthorizedForSFID(t, code, body, authzSFIDB)
		hz.assertNoWrites(t)
	})
	t.Run("manager of both projects may re-point it", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-ab", http.MethodPost, "/v1/github/organizations", map[string]any{"organization_name": "org-b", "organization_sfid": authzSFIDA})
		if code != http.StatusOK || body["organization_sfid"] != authzSFIDA {
			t.Fatalf("expected re-pointed org, got %d %v", code, body)
		}
		w := hz.assertSingleWrite(t, "PutItem", "github-orgs")
		if dynStr(w.Item, "organization_sfid") != authzSFIDA || dynStr(w.Item, "project_sfid") != authzSFIDA {
			t.Fatalf("unexpected stored org: %v", w.Item)
		}
	})
	t.Run("registering a new org only needs the requested project", func(t *testing.T) {
		hz.dyn.resetWrites()
		code, body := hz.call(t, "manager-a", http.MethodPost, "/v1/github/organizations", map[string]any{"organization_name": "org-new", "organization_sfid": authzSFIDA})
		if code != http.StatusOK || body["organization_name"] != "org-new" {
			t.Fatalf("expected created org, got %d %v", code, body)
		}
		hz.assertSingleWrite(t, "PutItem", "github-orgs")
	})
}

// Pure decision helpers on a zero-value Handlers (no stores): the branches that need no lookups.
func TestSignatureWriteAuthorizedWithoutStores(t *testing.T) {
	h := &Handlers{}
	ccla := map[string]types.AttributeValue{
		"signature_reference_type": &types.AttributeValueMemberS{Value: "company"},
		"signature_acl":            &types.AttributeValueMemberSS{Value: []string{"cla-manager"}},
	}
	icla := map[string]types.AttributeValue{
		"signature_reference_type": &types.AttributeValueMemberS{Value: "user"},
		"signature_reference_id":   &types.AttributeValueMemberS{Value: authzUserAlice},
		"signature_project_id":     &types.AttributeValueMemberS{Value: authzCLAGroupA},
	}
	gerritICLA := map[string]types.AttributeValue{
		"signature_reference_type": &types.AttributeValueMemberS{Value: "user"},
		"signature_reference_id":   &types.AttributeValueMemberS{Value: authzUserAlice},
		"signature_acl":            &types.AttributeValueMemberSS{Value: []string{"alice", "cla-manager"}},
	}
	cases := []struct {
		name     string
		username string
		sig      map[string]types.AttributeValue
		want     bool
	}{
		{"empty username", "", ccla, false},
		{"admin bypass", "bryan.stone", icla, true},
		{"signature ACL member", "cla-manager", ccla, true},
		{"outsider on company signature", "outsider", ccla, false},
		{"referenced user is never an owner", "alice", icla, false},
		{"signature_acl of a user signature grants nothing to the signer", "alice", gerritICLA, false},
		{"signature_acl of a user signature grants nothing to a manager either", "cla-manager", gerritICLA, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := h.signatureWriteAuthorized(context.Background(), tc.username, tc.sig)
			if err != nil || got != tc.want {
				t.Fatalf("got (%v, %v), want (%v, nil)", got, err, tc.want)
			}
		})
	}
}

func TestOwningSFIDFallbacks(t *testing.T) {
	item := func(k, v string) map[string]types.AttributeValue {
		return map[string]types.AttributeValue{k: &types.AttributeValueMemberS{Value: v}}
	}
	if got := githubOrgOwningSFID(item("organization_sfid", " "+authzSFIDA+" ")); got != authzSFIDA {
		t.Fatalf("organization_sfid: got %q", got)
	}
	if got := githubOrgOwningSFID(item("project_sfid", authzSFIDB)); got != authzSFIDB {
		t.Fatalf("project_sfid fallback: got %q", got)
	}
	if got := repositoryOwningSFID(item("repository_sfdc_id", authzSFIDA)); got != authzSFIDA {
		t.Fatalf("repository_sfdc_id fallback: got %q", got)
	}
	if got := repositoryOwningSFID(item("project_sfid", authzSFIDB)); got != authzSFIDB {
		t.Fatalf("repository project_sfid: got %q", got)
	}
	// v4 stores the parent foundation in repository_sfdc_id and the owning child project in project_sfid
	if got := repositoryOwningSFID(map[string]types.AttributeValue{
		"repository_sfdc_id": &types.AttributeValueMemberS{Value: authzSFIDA},
		"project_sfid":       &types.AttributeValueMemberS{Value: authzSFIDB},
	}); got != authzSFIDB {
		t.Fatalf("project_sfid must win over repository_sfdc_id: got %q", got)
	}
	if got := repositoryOwningSFID(map[string]types.AttributeValue{}); got != "" {
		t.Fatalf("unbound record: got %q", got)
	}
}
