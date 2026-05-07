// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	stdmail "net/mail"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/auth"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/contracts"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/email"
	githublegacy "github.com/linuxfoundation/easycla/cla-backend-legacy/internal/legacy/github"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/legacy/lfgroup"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/legacy/salesforce"
	userservicelegacy "github.com/linuxfoundation/easycla/cla-backend-legacy/internal/legacy/userservice"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/logging"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/parity"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/pdf"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/respond"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/store"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/telemetry"
)

// Handlers implements the legacy (v1/v2) API surface in Go.
type Handlers struct {

	// Ported building blocks (incrementally used by endpoints as they are rewritten from Python).
	// AWS region used by the legacy service for AWS SDK clients.
	// Loaded from AWS_REGION (preferred) or REGION; defaults to us-east-1.
	region            string
	httpClient        *http.Client
	authValidator     *auth.Auth0Validator
	userPerms         *store.UserPermissionsStore
	users             *store.UsersStore
	companies         *store.CompaniesStore
	events            *store.EventsStore
	kv                *store.KVStore
	repos             *store.RepositoriesStore
	signatures        *store.SignaturesStore
	projects          *store.ProjectsStore
	projectCLAGroups  *store.ProjectCLAGroupsStore
	gerritInstances   *store.GerritInstancesStore
	githubOrgs        *store.GitHubOrgsStore
	gitlabOrgs        *store.GitLabOrgsStore
	companyInvites    *store.CompanyInvitesStore
	cclaAllowlistReqs *store.CCLAAllowlistRequestsStore
	salesforce        *salesforce.Service
	github            *githublegacy.Service
	lfGroup           *lfgroup.Client
	userService       *userservicelegacy.Client
}

func NewHandlers() *Handlers {
	client := telemetry.NewHTTPClient(30 * time.Second)
	h := &Handlers{
		httpClient: client,
	}

	// Ensure region is always initialized (handlers use h.region for AWS clients).
	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("REGION"))
	}
	if region == "" {
		region = "us-east-1"
	}
	h.region = region

	// These can be nil if misconfigured; endpoint handlers should fail fast when used.
	h.authValidator = auth.NewAuth0ValidatorFromEnv(client)
	h.salesforce = salesforce.NewFromEnv(client)
	h.github = githublegacy.New(client)
	h.lfGroup = lfgroup.NewFromEnv(client)
	h.userService = userservicelegacy.NewFromEnv(client)

	ctx := context.Background()
	ups, err := store.NewUserPermissionsStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("user permissions store init failed: %v", err)
	} else {
		h.userPerms = ups
	}
	us, err := store.NewUsersStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("users store init failed: %v", err)
	} else {
		h.users = us
	}
	cs, err := store.NewCompaniesStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("companies store init failed: %v", err)
	} else {
		h.companies = cs
	}
	ev, err := store.NewEventsStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("events store init failed: %v", err)
	} else {
		h.events = ev
	}
	ks, err := store.NewKVStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("kv store init failed: %v", err)
	} else {
		h.kv = ks
	}
	rs, err := store.NewRepositoriesStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("repositories store init failed: %v", err)
	} else {
		h.repos = rs
	}
	ss, err := store.NewSignaturesStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("signatures store init failed: %v", err)
	} else {
		h.signatures = ss
	}
	ps, err := store.NewProjectsStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("projects store init failed: %v", err)
	} else {
		h.projects = ps
	}
	pcgs, err := store.NewProjectCLAGroupsStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("project CLA groups store init failed: %v", err)
	} else {
		h.projectCLAGroups = pcgs
	}
	gis, err := store.NewGerritInstancesStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("gerrit instances store init failed: %v", err)
	} else {
		h.gerritInstances = gis
	}
	gos, err := store.NewGitHubOrgsStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("github orgs store init failed: %v", err)
	} else {
		h.githubOrgs = gos
	}
	glos, err := store.NewGitLabOrgsStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("gitlab orgs store init failed: %v", err)
	} else {
		h.gitlabOrgs = glos
	}

	cis, err := store.NewCompanyInvitesStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("company invites store init failed: %v", err)
	} else {
		h.companyInvites = cis
	}
	cars, err := store.NewCCLAAllowlistRequestsStoreFromEnv(ctx)
	if err != nil {
		logging.Warnf("ccla allowlist requests store init failed: %v", err)
	} else {
		h.cclaAllowlistReqs = cars
	}

	return h
}

// formatPynamoDateTimeUTC formats timestamps the way Python's pynamodb
// UTCDateTimeAttribute serializes them when written to DynamoDB.
//
// pynamodb forces tzinfo=UTC and calls strftime("%Y-%m-%dT%H:%M:%S.%f%z"),
// producing strings like "2025-05-05T14:23:45.123456+0000" — always six
// microsecond digits (zero-padded), always a "+0000" offset (no colon).
// Matching this exactly preserves byte-for-byte parity for date_created,
// date_modified, event_time, document_creation_date, etc.
func formatPynamoDateTimeUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000000-0700")
}

func boolToPythonString(b bool) string {
	// Match Python's str(True).lower() / str(False).lower() usage.
	if b {
		return "true"
	}
	return "false"
}

func boolString(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func getAttrBool(item map[string]types.AttributeValue, key string) bool {
	av, ok := item[key]
	if !ok || av == nil {
		return false
	}
	switch v := av.(type) {
	case *types.AttributeValueMemberBOOL:
		return v.Value
	case *types.AttributeValueMemberS:
		s := strings.TrimSpace(strings.ToLower(v.Value))
		return s == "true" || s == "1" || s == "yes"
	case *types.AttributeValueMemberN:
		s := strings.TrimSpace(v.Value)
		return s == "1"
	default:
		return false
	}
}

func getAttrString(item map[string]types.AttributeValue, key string) string {
	if item == nil {
		return ""
	}
	v, ok := item[key]
	if !ok || v == nil {
		return ""
	}
	switch tv := v.(type) {
	case *types.AttributeValueMemberS:
		return tv.Value
	case *types.AttributeValueMemberN:
		return tv.Value
	case *types.AttributeValueMemberBOOL:
		if tv.Value {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func getUserEmailLikePython(item map[string]types.AttributeValue) string {
	if item == nil {
		return ""
	}
	if v := strings.TrimSpace(getAttrString(item, "lf_email")); v != "" {
		return v
	}
	emails := getAttrStringSlice(item, "user_emails")
	if len(emails) > 0 {
		return strings.TrimSpace(emails[0])
	}
	return ""
}

func getAttrInt(item map[string]types.AttributeValue, key string) int {
	s := strings.TrimSpace(getAttrString(item, key))
	if s == "" {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

func decodeJSONBody(r *http.Request, dst any) error {
	if r == nil || r.Body == nil {
		return io.EOF
	}
	dec := json.NewDecoder(r.Body)
	// Hug (Python) accepts unknown fields by default; do not call DisallowUnknownFields().
	return dec.Decode(dst)
}

func validateURL(value string) (string, error) {
	// Port of cla.hug_types.url() which requires scheme and netloc/host.
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("Invalid URL specified")
	}
	return value, nil
}

func getAttrStringSlice(item map[string]types.AttributeValue, key string) []string {
	if item == nil {
		return nil
	}
	av, ok := item[key]
	if !ok || av == nil {
		return nil
	}
	switch v := av.(type) {
	case *types.AttributeValueMemberSS:
		out := make([]string, len(v.Value))
		copy(out, v.Value)
		return out
	case *types.AttributeValueMemberL:
		out := make([]string, 0, len(v.Value))
		for _, el := range v.Value {
			if s, ok := el.(*types.AttributeValueMemberS); ok {
				out = append(out, s.Value)
			}
		}
		return out
	default:
		return nil
	}
}

func stringSliceContainsExact(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// smartBool mirrors hug.types.smart_boolean behavior for common representations.
// It accepts booleans, "true"/"false" strings, and 0/1 numbers.
func smartBool(v any) (bool, error) {
	switch t := v.(type) {
	case bool:
		return t, nil
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		switch s {
		case "1", "true", "t", "yes", "y":
			return true, nil
		case "0", "false", "f", "no", "n":
			return false, nil
		default:
			return false, fmt.Errorf("invalid boolean: %q", t)
		}
	case float64:
		// JSON numbers decode as float64.
		if t == 0 {
			return false, nil
		}
		if t == 1 {
			return true, nil
		}
		return false, fmt.Errorf("invalid boolean number: %v", t)
	case int:
		if t == 0 {
			return false, nil
		}
		if t == 1 {
			return true, nil
		}
		return false, fmt.Errorf("invalid boolean number: %v", t)
	case int64:
		if t == 0 {
			return false, nil
		}
		if t == 1 {
			return true, nil
		}
		return false, fmt.Errorf("invalid boolean number: %v", t)
	default:
		return false, fmt.Errorf("invalid boolean type %T", v)
	}
}

func stringListFromAny(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	switch t := v.(type) {
	case []string:
		out := make([]string, 0, len(t))
		for _, s := range t {
			ss := strings.TrimSpace(s)
			if ss != "" {
				out = append(out, ss)
			}
		}
		return out, nil
	case []any:
		out := make([]string, 0, len(t))
		for _, el := range t {
			if el == nil {
				continue
			}
			ss := strings.TrimSpace(fmt.Sprint(el))
			if ss != "" {
				out = append(out, ss)
			}
		}
		return out, nil
	case string:
		// Accept comma-separated input (common when coming from query/form encoding).
		if strings.Contains(t, ",") {
			parts := strings.Split(t, ",")
			out := make([]string, 0, len(parts))
			for _, p := range parts {
				ss := strings.TrimSpace(p)
				if ss != "" {
					out = append(out, ss)
				}
			}
			return out, nil
		}
		ss := strings.TrimSpace(t)
		if ss == "" {
			return []string{}, nil
		}
		return []string{ss}, nil
	default:
		return nil, fmt.Errorf("invalid list type %T", v)
	}
}

func wholeNumberString(v any) (string, error) {
	if v == nil {
		return "", nil
	}
	parseString := func(s string) (string, error) {
		s = strings.TrimSpace(s)
		if s == "" {
			return "", nil
		}
		if i, err := strconv.Atoi(s); err == nil {
			return strconv.Itoa(i), nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			if f != float64(int64(f)) {
				return "", fmt.Errorf("invalid integer: %v", s)
			}
			return strconv.FormatInt(int64(f), 10), nil
		}
		return "", fmt.Errorf("invalid integer: %v", s)
	}

	switch t := v.(type) {
	case int:
		return strconv.Itoa(t), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case float64:
		if t != float64(int64(t)) {
			return "", fmt.Errorf("invalid integer: %v", t)
		}
		return strconv.FormatInt(int64(t), 10), nil
	case float32:
		if t != float32(int64(t)) {
			return "", fmt.Errorf("invalid integer: %v", t)
		}
		return strconv.FormatInt(int64(t), 10), nil
	case string:
		return parseString(t)
	}

	return parseString(fmt.Sprint(v))
}

func uniqueStringsPreserveOrder(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func normalizeAllowlist(in []string) []string {
	// Python trims values implicitly in many callers; Dynamo cannot store empty sets.
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return uniqueStringsPreserveOrder(out)
}

// normalizeUserDict matches legacy Python cla.models.dynamo_models.User.to_dict() behavior
// where some identity provider IDs can be stored as the literal string "null".
func normalizeUserDict(user map[string]any) map[string]any {
	if user == nil {
		return user
	}
	for _, k := range []string{"user_github_id", "user_ldap_id", "user_gitlab_id"} {
		if v, ok := user[k]; ok {
			if s, ok := v.(string); ok && s == "null" {
				user[k] = nil
			}
		}
	}
	return user
}

// normalizeGerritDict mirrors Gerrit.to_dict() output in legacy Python.
//
// Pynamo's dict(model) includes keys for null=True attributes even when absent in DynamoDB.
// When reading raw DynamoDB items in Go, missing optional attributes would otherwise be omitted.
func normalizeGerritDict(g map[string]any) map[string]any {
	if g == nil {
		return g
	}
	// Optional attributes (null=True)
	for _, k := range []string{
		"gerrit_name",
		"gerrit_url",
		"group_id_icla",
		"group_id_ccla",
		"group_name_icla",
		"group_name_ccla",
		"project_sfid",
	} {
		if _, ok := g[k]; !ok {
			g[k] = nil
		}
	}
	return g
}

type auditEventInput struct {
	EventType       string
	EventCompanyID  string
	EventProjectID  string // SFDC project ID (not CLA group UUID)
	EventCLAGroupID string // CLA group UUID (projects table project_id)
	EventUserID     string
	EventData       string
	EventSummary    string
	// Optional explicit names (Python create_event defaults these to "undefined" when not provided).
	EventCompanyName string
	EventProjectName string
	ContainsPII      bool
}

func isAdminUser(username string) bool {
	switch username {
	case "vnaidu", "ddeal", "bryan.stone":
		return true
	default:
		return false
	}
}

// checkUserAuthorization matches legacy Python cla.controllers.project.check_user_authorization().
// It verifies the authenticated user has the given Salesforce project ID in their authorized_projects list.
//
// Return values:
//
//	valid=true  => nil errors
//	valid=false => errors payload shaped as {"errors": { ... }}
func (h *Handlers) checkUserAuthorization(ctx context.Context, username, sfid string) (bool, map[string]any) {
	if h == nil || h.userPerms == nil {
		return false, map[string]any{"errors": map[string]any{"user does not exist": "User Permissions not found"}}
	}
	perms, err := h.userPerms.Get(ctx, username)
	if err != nil {
		return false, map[string]any{"errors": map[string]any{"user does not exist": err.Error()}}
	}
	for _, p := range perms.Projects {
		if p == sfid {
			return true, nil
		}
	}
	return false, map[string]any{"errors": map[string]any{"user is not authorized for this Salesforce ID.": sfid}}
}

// putAuditEventBestEffort mirrors (a minimal subset of) cla.models.dynamo_models.Event.create_event().
//
// It intentionally follows legacy semantics:
//   - event_date uses DD-MM-YYYY
//   - event_date_and_contains_pii uses "<date>#<true|false>"
//   - contains_pii attribute name is "contains_pii" (no event_ prefix)
//   - event_company_name/event_project_name default to "undefined" unless explicitly provided or enriched
//
// normalizeGitHubOrgDict mirrors GitHubOrg.to_dict() normalization in legacy Python.
func normalizeGitHubOrgDict(org map[string]any) map[string]any {
	if org == nil {
		return org
	}
	if v, ok := org["skip_cla"]; !ok || v == nil {
		org["skip_cla"] = map[string]any{}
	}
	if v, ok := org["enable_co_authors"]; !ok || v == nil {
		org["enable_co_authors"] = map[string]any{}
	}
	for _, k := range []string{"organization_installation_id", "organization_sfid"} {
		if _, ok := org[k]; !ok {
			org[k] = nil
			continue
		}
		if sv, ok := org[k].(string); ok {
			s := strings.ToLower(strings.TrimSpace(sv))
			if s == "" || s == "none" || s == "null" {
				org[k] = nil
			}
		}
	}
	return org
}

func (h *Handlers) putAuditEventBestEffort(ctx context.Context, in auditEventInput) {
	if h == nil || h.events == nil {
		return
	}

	now := time.Now().UTC()
	dateDDMMYYYY := now.Format("02-01-2006")
	eventID := uuid.New().String()

	companyName := strings.TrimSpace(in.EventCompanyName)
	if companyName == "" {
		companyName = "undefined"
	}
	projectName := strings.TrimSpace(in.EventProjectName)
	if projectName == "" {
		projectName = "undefined"
	}

	item := map[string]types.AttributeValue{
		"event_id":                    &types.AttributeValueMemberS{Value: eventID},
		"event_type":                  &types.AttributeValueMemberS{Value: in.EventType},
		"event_time":                  &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"event_time_epoch":            &types.AttributeValueMemberN{Value: strconv.FormatInt(now.Unix(), 10)},
		"event_date":                  &types.AttributeValueMemberS{Value: dateDDMMYYYY},
		"contains_pii":                &types.AttributeValueMemberBOOL{Value: in.ContainsPII},
		"event_date_and_contains_pii": &types.AttributeValueMemberS{Value: fmt.Sprintf("%s#%s", dateDDMMYYYY, boolToPythonString(in.ContainsPII))},
		"date_created":                &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"date_modified":               &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"version":                     &types.AttributeValueMemberS{Value: "v1"},
	}

	if in.EventCompanyID != "" {
		item["event_company_id"] = &types.AttributeValueMemberS{Value: in.EventCompanyID}
	}
	if in.EventProjectID != "" {
		item["event_project_id"] = &types.AttributeValueMemberS{Value: in.EventProjectID}
	}
	if in.EventCLAGroupID != "" {
		item["event_cla_group_id"] = &types.AttributeValueMemberS{Value: in.EventCLAGroupID}
	}
	if in.EventUserID != "" {
		item["event_user_id"] = &types.AttributeValueMemberS{Value: in.EventUserID}
	}

	if in.EventData != "" {
		item["event_data"] = &types.AttributeValueMemberS{Value: in.EventData}
		item["event_data_lower"] = &types.AttributeValueMemberS{Value: strings.ToLower(in.EventData)}
	}
	if in.EventSummary != "" {
		item["event_summary"] = &types.AttributeValueMemberS{Value: in.EventSummary}
	}

	// Best-effort enrichment to align with Python Event.set_company_details() / set_cla_group_details().
	if in.EventCompanyID != "" && h.companies != nil {
		if c, found, err := h.companies.GetByID(ctx, in.EventCompanyID); err == nil && found {
			cn := strings.TrimSpace(getAttrString(c, "company_name"))
			if cn != "" {
				companyName = cn
			}
			sfid := strings.TrimSpace(getAttrString(c, "company_external_id"))
			if sfid != "" {
				item["event_company_sfid"] = &types.AttributeValueMemberS{Value: sfid}
			}
		}
	}
	if in.EventCLAGroupID != "" && h.projects != nil {
		if p, found, err := h.projects.GetByID(ctx, in.EventCLAGroupID); err == nil && found {
			claName := strings.TrimSpace(getAttrString(p, "project_name"))
			if claName != "" {
				item["event_cla_group_name"] = &types.AttributeValueMemberS{Value: claName}
				item["event_cla_group_name_lower"] = &types.AttributeValueMemberS{Value: strings.ToLower(claName)}
			}
			projectSFID := strings.TrimSpace(getAttrString(p, "project_external_id"))
			if projectSFID != "" {
				item["event_project_sfid"] = &types.AttributeValueMemberS{Value: projectSFID}
			}
		}
	}

	// Python always sets these (defaulting to "undefined").
	item["event_company_name"] = &types.AttributeValueMemberS{Value: companyName}
	item["event_company_name_lower"] = &types.AttributeValueMemberS{Value: strings.ToLower(companyName)}
	item["event_project_name"] = &types.AttributeValueMemberS{Value: projectName}
	item["event_project_name_lower"] = &types.AttributeValueMemberS{Value: strings.ToLower(projectName)}

	_ = h.events.PutItem(ctx, item)
}

func (h *Handlers) refreshStoredUserName(ctx context.Context, item map[string]types.AttributeValue, authUser *auth.AuthUser) map[string]types.AttributeValue {
	if h == nil || h.users == nil || item == nil || authUser == nil {
		return item
	}

	userName := strings.TrimSpace(authUser.Name)
	if userName == "" || getAttrString(item, "user_name") == userName {
		return item
	}

	updated := make(map[string]types.AttributeValue, len(item)+1)
	for k, v := range item {
		updated[k] = v
	}
	updated["user_name"] = &types.AttributeValueMemberS{Value: userName}
	updated["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}

	if err := h.users.PutItem(ctx, updated); err != nil {
		logging.Warnf("refreshStoredUserName: unable to update user_id=%s lf_username=%s: %v", getAttrString(item, "user_id"), authUser.Username, err)
		return item
	}

	return updated
}

func (h *Handlers) getOrCreateUser(ctx context.Context, authUser *auth.AuthUser) (map[string]types.AttributeValue, bool, error) {
	if authUser == nil {
		return nil, false, fmt.Errorf("missing auth user")
	}

	items, err := h.users.QueryByLFUsername(ctx, authUser.Username)
	if err != nil {
		return nil, false, err
	}
	if len(items) > 0 {
		return h.refreshStoredUserName(ctx, items[0], authUser), false, nil
	}

	now := time.Now().UTC()
	userID := uuid.New().String()
	item := map[string]types.AttributeValue{
		"user_id":       &types.AttributeValueMemberS{Value: userID},
		"user_name":     &types.AttributeValueMemberS{Value: authUser.Name},
		"lf_username":   &types.AttributeValueMemberS{Value: authUser.Username},
		"lf_sub":        &types.AttributeValueMemberS{Value: authUser.Sub},
		"date_created":  &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"date_modified": &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"version":       &types.AttributeValueMemberS{Value: "v1"},
	}
	if authUser.Email != "" {
		item["lf_email"] = &types.AttributeValueMemberS{Value: strings.ToLower(authUser.Email)}
	}

	if err := h.users.PutItem(ctx, item); err != nil {
		return nil, false, err
	}

	eventData := fmt.Sprintf("CLA user added for %s", authUser.Username)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:    "CreateUser",
		EventUserID:  userID,
		EventData:    eventData,
		EventSummary: eventData,
		ContainsPII:  true,
	})

	return item, true, nil
}

// pickLatestSignature mirrors cla.models.model_interfaces.User.get_latest_signature().
//
// It selects the signature with the highest (major, minor) document version.
// When companyID is non-empty, it filters to signatures with signature_user_ccla_company_id == companyID.
// When companyID is empty, it filters to signatures where signature_user_ccla_company_id is unset.
func pickLatestSignature(items []map[string]types.AttributeValue, companyID string) map[string]types.AttributeValue {
	var latest map[string]types.AttributeValue
	lastMajor := -1
	lastMinor := -1

	for _, it := range items {
		if it == nil {
			continue
		}

		// Reference type is expected to be "user" for these endpoints.
		refType := strings.TrimSpace(getAttrString(it, "signature_reference_type"))
		if refType != "" && !strings.EqualFold(refType, "user") {
			continue
		}

		sigCompanyID := strings.TrimSpace(getAttrString(it, "signature_user_ccla_company_id"))
		if companyID == "" {
			if sigCompanyID != "" {
				continue
			}
		} else {
			if sigCompanyID != companyID {
				continue
			}
		}

		maj := getAttrInt(it, "signature_document_major_version")
		min := getAttrInt(it, "signature_document_minor_version")
		if maj > lastMajor || (maj == lastMajor && min > lastMinor) {
			lastMajor = maj
			lastMinor = min
			latest = it
		}
	}

	return latest
}

// NotImplemented returns HTTP 501 for intentionally unimplemented legacy handlers.
func (h *Handlers) NotImplemented(w http.ResponseWriter, r *http.Request) {
	respond.NotImplemented(w, r)
}

// NotFound returns HTTP 404 for unknown legacy routes.
func (h *Handlers) NotFound(w http.ResponseWriter, r *http.Request) {
	respond.NotFound(w, r)
}

// MethodNotAllowed returns HTTP 405 for unsupported methods, preserving
// legacy Hug 404 quirks for selected v2 paths.
func (h *Handlers) MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	// Python/Hug versioning parity: some endpoints exist in v2 only for GET, while
	// the same path+method exists in v1. Hug can return 404 ("not defined") for
	// these method+version combinations, not 405.
	//
	// Cypress functional tests currently expect 404 for these cases:
	//   - POST/PUT /v2/company
	//   - DELETE /v2/company/{company_id}
	//   - DELETE /v2/project/{project_id}
	//   - DELETE /v2/gerrit/{gerrit_id}
	path := r.URL.Path
	method := r.Method
	if shouldReturnNotFoundForV2MethodMismatch(path, method) {
		respond.NotFound(w, r)
		return
	}

	respond.MethodNotAllowed(w, r)
}

func shouldReturnNotFoundForV2MethodMismatch(path, method string) bool {
	// /v2/company is GET-only in v2, but POST/PUT exist in v1.
	if path == "/v2/company" {
		switch method {
		case http.MethodPost, http.MethodPut:
			return true
		}
	}
	// /v2/company/{company_id} is GET-only in v2, but DELETE exists in v1.
	if strings.HasPrefix(path, "/v2/company/") {
		if method == http.MethodDelete {
			return true
		}
	}
	// /v2/project/{project_id} is GET-only in v2, but DELETE exists in v1.
	if strings.HasPrefix(path, "/v2/project/") {
		if method == http.MethodDelete {
			return true
		}
	}
	// /v2/gerrit/{gerrit_id} is GET-only in v2, but DELETE exists in v1.
	if strings.HasPrefix(path, "/v2/gerrit/") {
		if method == http.MethodDelete {
			return true
		}
	}
	return false
}

// forwardGithubActivityToV4 forwards GitHub App webhook activity events to the Go v4 backend.
//
// Legacy Python logic: normalize PLATFORM_GATEWAY_URL to the v4 base URL and then POST
// to "/github/activity".
//
// Important: non-2xx responses from v4 should not be treated as errors, because the legacy
// behavior intentionally returns 200 OK to GitHub even when the downstream call fails.
func (h *Handlers) forwardGithubActivityToV4(ctx context.Context, body []byte, headers http.Header) error {
	baseURL, err := h.v4BaseURL()
	if err != nil {
		return err
	}
	url := baseURL + "/github/activity"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	// Copy original GitHub headers (Python forwards all request headers).
	for k, vals := range headers {
		// Avoid propagating a mismatched host.
		if strings.EqualFold(k, "Host") {
			continue
		}
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		// Legacy python logs the status + body but still returns 200 to GitHub.
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		logging.Warnf("v4 github/activity returned %d: %s", resp.StatusCode, string(b))
		return nil
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func githubActivityAction(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload["action"].(string); ok {
		return strings.TrimSpace(v)
	}
	if v, ok := payload["action"]; ok && v != nil {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}

func shouldForwardGithubActivityToV4(eventType string, action string) bool {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "installation_repositories", "integration_installation_repositories", "repository":
		return true
	case "push":
		return strings.EqualFold(strings.TrimSpace(action), "created")
	default:
		return false
	}
}

func githubActivityStringValue(payload map[string]any, keys ...string) string {
	if payload == nil || len(keys) == 0 {
		return ""
	}
	cur := payload
	for i, key := range keys {
		v, ok := cur[key]
		if !ok || v == nil {
			return ""
		}
		if i == len(keys)-1 {
			switch tv := v.(type) {
			case string:
				return strings.TrimSpace(tv)
			case float64:
				if tv == float64(int64(tv)) {
					return strconv.FormatInt(int64(tv), 10)
				}
				return strings.TrimSpace(fmt.Sprint(tv))
			default:
				return strings.TrimSpace(fmt.Sprint(tv))
			}
		}
		next, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		cur = next
	}
	return ""
}

func githubActivityInt64Value(payload map[string]any, keys ...string) (int64, bool) {
	s := githubActivityStringValue(payload, keys...)
	if s == "" {
		return 0, false
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return i, true
}

func githubCommentContainsEasyCLACommand(commentBody string) bool {
	for _, token := range strings.Fields(commentBody) {
		if token == "/easycla" {
			return true
		}
	}
	return false
}

func extractPullRequestNumberFromMergeGroupMessage(message string) (string, bool) {
	message = strings.TrimSpace(message)
	if message == "" {
		return "", false
	}
	lines := strings.Split(message, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return "", false
	}
	firstLine := strings.TrimSpace(lines[0])

	if matches := regexp.MustCompile(`^Merge pull request #(\d+)`).FindStringSubmatch(firstLine); len(matches) == 2 {
		return matches[1], true
	}
	matches := regexp.MustCompile(`\(#(\d+)\)`).FindAllStringSubmatch(firstLine, -1)
	if len(matches) > 0 {
		return matches[len(matches)-1][1], true
	}
	matches = regexp.MustCompile(`\s+#(\d+)`).FindAllStringSubmatch(firstLine, -1)
	if len(matches) > 0 {
		return matches[len(matches)-1][1], true
	}
	matches = regexp.MustCompile(`#(\d+)`).FindAllStringSubmatch(message, -1)
	if len(matches) > 0 {
		return matches[0][1], true
	}
	return "", false
}

func (h *Handlers) handleLegacyGithubInstallationEvent(ctx context.Context, action string, payload map[string]any) (any, error) {
	action = strings.TrimSpace(action)
	if action != "created" && action != "deleted" {
		return nil, nil
	}
	orgName := githubActivityStringValue(payload, "installation", "account", "login")
	if orgName == "" {
		orgName = githubActivityStringValue(payload, "organization", "login")
	}
	if orgName == "" {
		orgName = githubActivityStringValue(payload, "repository", "owner", "login")
	}
	if orgName == "" {
		return map[string]any{"status": fmt.Sprintf("GitHub installation %s event malformed.", action)}, nil
	}

	if action == "deleted" {
		return nil, h.notifyLegacyGithubProjectManagersUnableToCheck(ctx, orgName)
	}

	installationID, ok := githubActivityInt64Value(payload, "installation", "id")
	if !ok {
		return map[string]any{"status": fmt.Sprintf("GitHub installation %s event malformed.", action)}, nil
	}
	if h.githubOrgs == nil {
		return nil, errors.New("github orgs store is not configured")
	}
	org, found, err := h.githubOrgs.GetByLowerName(ctx, strings.ToLower(orgName))
	if err != nil {
		return nil, err
	}
	if !found {
		return map[string]any{"status": "Github Organization must be created through the Project Management Console."}, nil
	}

	oldInstallationID := strings.TrimSpace(getAttrString(org, "organization_installation_id"))
	org["organization_installation_id"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(installationID, 10)}
	org["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}
	if err := h.githubOrgs.PutItem(ctx, org); err != nil {
		return nil, err
	}
	if oldInstallationID == "" || oldInstallationID == "0" {
		return map[string]any{"status": "Organization Enrollment Completed. CLA System is operational"}, nil
	}
	return map[string]any{"status": "Already Enrolled Organization Updated. CLA System is operational"}, nil
}
func (h *Handlers) notifyLegacyGithubProjectManagersUnableToCheck(ctx context.Context, organizationName string) error {
	if h.repos == nil {
		return errors.New("repositories store is not configured")
	}
	if h.projects == nil {
		return errors.New("projects store is not configured")
	}
	if h.users == nil {
		return errors.New("users store is not configured")
	}
	repos, err := h.repos.QueryByOrganizationName(ctx, organizationName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return nil
	}

	projectRepos := map[string][]string{}
	for _, repo := range repos {
		projectID := strings.TrimSpace(getAttrString(repo, "repository_project_id"))
		if projectID == "" {
			continue
		}
		repoURL := strings.TrimSpace(getAttrString(repo, "repository_url"))
		projectRepos[projectID] = append(projectRepos[projectID], repoURL)
	}
	if len(projectRepos) == 0 {
		return nil
	}

	svc, err := email.NewFromEnv(ctx)
	if err != nil {
		return err
	}
	for projectID, repoURLs := range projectRepos {
		project, found, err := h.projects.GetByID(ctx, projectID)
		if err != nil {
			return err
		}
		if !found {
			logging.Warnf("notify_project_managers - unable to load project (cla_group) by project_id: %s", projectID)
			return nil
		}
		recipients, err := h.projectManagerEmails(ctx, project)
		if err != nil {
			return err
		}
		subject, body := unableToDoCLACheckEmailContent(project, repoURLs)
		if err := svc.Send(ctx, subject, body, recipients); err != nil {
			return err
		}
		logging.Debugf("github.activity - sending unable to perform CLA Check email to managers: %v for project %s with repositories: %v", recipients, getAttrString(project, "project_id"), repoURLs)
	}
	return nil
}

func (h *Handlers) projectManagerEmails(ctx context.Context, project map[string]types.AttributeValue) ([]string, error) {
	if h.users == nil {
		return nil, errors.New("users store is not configured")
	}
	acl := getAttrStringSlice(project, "project_acl")
	recipients := make([]string, 0, len(acl))
	for _, lfid := range acl {
		lfid = strings.TrimSpace(lfid)
		if lfid == "" {
			continue
		}
		users, err := h.users.QueryByLFUsername(ctx, lfid)
		if err != nil {
			return nil, err
		}
		if len(users) == 0 {
			continue
		}
		emailAddress := strings.TrimSpace(getUserEmailLikePython(users[0]))
		if emailAddress != "" {
			recipients = append(recipients, emailAddress)
		}
	}
	return recipients, nil
}

func unableToDoCLACheckEmailContent(project map[string]types.AttributeValue, repoURLs []string) (string, string) {
	claGroupName := getAttrString(project, "project_name")
	projectVersion := getAttrString(project, "version")
	subject := fmt.Sprintf("EasyCLA: Unable to check GitHub Pull Requests for CLA Group: %s", claGroupName)
	pronoun := "this repository"
	if len(repoURLs) > 1 {
		pronoun = "these repositories"
	}
	repoContent := "<ul>"
	for _, repo := range repoURLs {
		repoContent += "<li>" + repo + "</li>"
	}
	repoContent += "</ul>"
	body := fmt.Sprintf(`
	<p>Hello Project Manager,</p>
	<p>This is a notification email from EasyCLA regarding the CLA Group %s.</p>
	<p>EasyCLA is unable to check PRs on %s due to permissions issue.</p>
	%s
	<p>Please contact the repository admin/owner to enable CLA checks.</p>
	<p>Provide the Owner/Admin the following instructions:</p>
	<ul>
	<li>Go into the "Settings" tab of the GitHub Organization</li>
	<li>Click on "installed GitHub Apps" vertical navigation</li>
	<li>Then click "Configure" associated with the EasyCLA App</li>
	<li>Finally, click the "All Repositories" radio button option</li>
	</ul>
	`, claGroupName, pronoun, repoContent)
	return subject, appendEmailHelpSignOffContent(body, projectVersion)
}

func (h *Handlers) storeLegacyActivePullRequestMetadata(ctx context.Context, payload map[string]any, installationID, githubRepositoryID, changeRequestID int64) error {
	if h.kv == nil || h.repos == nil || h.githubOrgs == nil || h.projects == nil {
		return nil
	}
	repo, found, err := h.repos.GetByExternalIDAndType(ctx, strconv.FormatInt(githubRepositoryID, 10), "github")
	if err != nil || !found {
		return err
	}
	if !getAttrBool(repo, "enabled") {
		return nil
	}
	orgName := strings.TrimSpace(getAttrString(repo, "repository_organization_name"))
	ghOrg, found, err := h.githubOrgs.GetByName(ctx, orgName)
	if err != nil || !found {
		return err
	}
	if strings.TrimSpace(getAttrString(ghOrg, "organization_installation_id")) != strconv.FormatInt(installationID, 10) {
		return nil
	}
	claGroupID := strings.TrimSpace(getAttrString(repo, "repository_project_id"))
	if claGroupID == "" {
		return nil
	}
	if _, found, err := h.projects.GetByID(ctx, claGroupID); err != nil || !found {
		return err
	}

	githubAuthorUsername := githubActivityStringValue(payload, "pull_request", "user", "login")
	if githubAuthorUsername == "" {
		githubAuthorUsername = githubActivityStringValue(payload, "issue", "user", "login")
	}
	githubAuthorEmail, hasEmail := githubActivityOptionalStringValue(payload, "pull_request", "user", "email")
	if !hasEmail {
		githubAuthorEmail, hasEmail = githubActivityOptionalStringValue(payload, "issue", "user", "email")
	}

	value, err := json.Marshal(map[string]any{
		"github_author_username": githubAuthorUsername,
		"github_author_email":    githubAuthorEmail,
		"cla_group_id":           claGroupID,
		"repository_id":          strconv.FormatInt(githubRepositoryID, 10),
		"pull_request_id":        strconv.FormatInt(changeRequestID, 10),
	})
	if err != nil {
		return err
	}
	if githubAuthorUsername != "" {
		if err := h.kv.Set(ctx, "active_pr:u:"+githubAuthorUsername, string(value)); err != nil {
			return err
		}
		logging.Infof("stored active pull request details by user: %s", "active_pr:u:"+githubAuthorUsername)
	}

	if hasEmail && githubAuthorEmail != "" {
		if err := h.kv.Set(ctx, "active_pr:e:"+githubAuthorEmail, string(value)); err != nil {
			return err
		}
		logging.Infof("stored active pull request details by user email: %s", "active_pr:e:"+githubAuthorEmail)
	}
	return nil
}

func githubActivityOptionalStringValue(payload map[string]any, keys ...string) (string, bool) {
	if payload == nil || len(keys) == 0 {
		return "", false
	}
	cur := payload
	for i, key := range keys {
		v, ok := cur[key]
		if !ok {
			return "", false
		}
		if i == len(keys)-1 {
			if v == nil {
				return "", false
			}
			return strings.TrimSpace(fmt.Sprint(v)), true
		}
		next, ok := v.(map[string]any)
		if !ok {
			return "", false
		}
		cur = next
	}
	return "", false
}

func (h *Handlers) handleLegacyGithubPullRequestUpdate(ctx context.Context, payload map[string]any) error {
	installationID, ok := githubActivityInt64Value(payload, "installation", "id")
	if !ok {
		return errors.New("missing installation id")
	}
	githubRepositoryID, ok := githubActivityInt64Value(payload, "repository", "id")
	if !ok {
		return errors.New("missing github repository id")
	}
	changeRequestID, ok := githubActivityInt64Value(payload, "pull_request", "number")
	if !ok {
		return errors.New("missing pull request id")
	}
	if err := h.storeLegacyActivePullRequestMetadata(ctx, payload, installationID, githubRepositoryID, changeRequestID); err != nil {
		logging.Errorf("github.update_change_request - problem saving PR metadata for PR: %d", changeRequestID)
	}
	return h.triggerGitHubChangeRequestUpdateV4(ctx, strconv.FormatInt(installationID, 10), strconv.FormatInt(githubRepositoryID, 10), strconv.FormatInt(changeRequestID, 10))
}

func (h *Handlers) handleLegacyGithubIssueComment(ctx context.Context, payload map[string]any) error {
	commentBody := githubActivityStringValue(payload, "comment", "body")
	if !githubCommentContainsEasyCLACommand(commentBody) {
		return nil
	}
	installationID, ok := githubActivityInt64Value(payload, "installation", "id")
	if !ok {
		logging.Debugf("github issue_comment ignored: missing installation id in /easycla comment payload")
		return nil
	}
	githubRepositoryID, ok := githubActivityInt64Value(payload, "repository", "id")
	if !ok {
		logging.Debugf("github issue_comment ignored: missing github repository id in /easycla comment payload")
		return nil
	}
	changeRequestID, ok := githubActivityInt64Value(payload, "issue", "number")
	if !ok {
		logging.Debugf("github issue_comment ignored: missing pull request id in /easycla comment payload")
		return nil
	}
	if err := h.storeLegacyActivePullRequestMetadata(ctx, payload, installationID, githubRepositoryID, changeRequestID); err != nil {
		logging.Errorf("github.update_change_request - problem saving PR metadata for PR: %d", changeRequestID)
	}
	return h.triggerGitHubChangeRequestUpdateV4(ctx, strconv.FormatInt(installationID, 10), strconv.FormatInt(githubRepositoryID, 10), strconv.FormatInt(changeRequestID, 10))
}

func (h *Handlers) handleLegacyGithubMergeGroup(ctx context.Context, payload map[string]any) error {
	installationID, ok := githubActivityInt64Value(payload, "installation", "id")
	if !ok {
		return errors.New("missing installation id")
	}
	githubRepositoryID, ok := githubActivityInt64Value(payload, "repository", "id")
	if !ok {
		return errors.New("missing github repository id")
	}
	mergeGroupSHA := githubActivityStringValue(payload, "merge_group", "head_sha")
	if mergeGroupSHA == "" {
		return errors.New("missing merge_group head_sha")
	}
	message := githubActivityStringValue(payload, "merge_group", "head_commit", "message")
	changeRequestID, ok := extractPullRequestNumberFromMergeGroupMessage(message)
	if !ok {
		logging.Warnf("github merge_group ignored: unable to extract pull request number from merge_group head_commit.message")
		return nil
	}
	return h.triggerGitHubMergeGroupUpdateV4(ctx, strconv.FormatInt(installationID, 10), strconv.FormatInt(githubRepositoryID, 10), changeRequestID, mergeGroupSHA)
}

func (h *Handlers) handleLegacyGithubReceivedActivity(ctx context.Context, payload map[string]any) error {
	action := githubActivityAction(payload)
	switch action {
	case "opened", "reopened", "synchronize", "enqueued":
		return h.handleLegacyGithubPullRequestUpdate(ctx, payload)
	case "checks_requested":
		return h.handleLegacyGithubMergeGroup(ctx, payload)
	case "closed":
		return nil
	default:
		return nil
	}
}

func (h *Handlers) handleLegacyGithubActivity(ctx context.Context, eventType, action string, payload map[string]any) (any, error) {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "installation", "integration_installation":
		_, err := h.handleLegacyGithubInstallationEvent(ctx, action, payload)
		return nil, err
	case "pull_request":
		if action == "opened" || action == "reopened" || action == "synchronize" || action == "enqueued" {
			return nil, h.handleLegacyGithubReceivedActivity(ctx, payload)
		}
		return nil, nil
	case "issue_comment":
		if action == "created" || action == "edited" {
			return nil, h.handleLegacyGithubIssueComment(ctx, payload)
		}
		return nil, nil
	case "merge_group":
		if action == "checks_requested" {
			return nil, h.handleLegacyGithubReceivedActivity(ctx, payload)
		}
		return nil, nil
	default:
		return nil, nil
	}
}

func splitPlatformMaintainers(raw string) []string {
	// Legacy Python uses cla.config.PLATFORM_MAINTAINERS.split(',').
	// Keep comma-splitting semantics, but trim surrounding whitespace for safety.
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (h *Handlers) sendGithubWebhookSecretFailedEmailBestEffort(ctx context.Context, headers http.Header, payload []byte, validateErr error) {
	_ = validateErr // Legacy email body does not include the validation error text.
	maintainers := splitPlatformMaintainers(strings.TrimSpace(os.Getenv("PLATFORM_MAINTAINERS")))
	if len(maintainers) == 0 {
		logging.Warnf("github webhook secret validation failed but PLATFORM_MAINTAINERS is empty")
		return
	}

	stage := strings.TrimSpace(os.Getenv("STAGE"))
	if stage == "" {
		stage = strings.TrimSpace(os.Getenv("ENV"))
	}
	if stage == "" {
		stage = "unknown"
	}

	eventType := strings.TrimSpace(headers.Get("X-Github-Event"))
	if eventType == "" {
		eventType = strings.TrimSpace(headers.Get("X-GitHub-Event"))
	}

	var userLogin, repositoryName, repositoryOwner, repositoryURL, parentOrg string
	var repositoryID any
	var installationID any
	if len(payload) > 0 {
		var reqBody map[string]any
		if err := json.Unmarshal(payload, &reqBody); err == nil {
			if sender, ok := reqBody["sender"].(map[string]any); ok {
				if s, ok := sender["login"].(string); ok {
					userLogin = s
				}
			}
			if repo, ok := reqBody["repository"].(map[string]any); ok {
				repositoryID = repo["id"]
				if s, ok := repo["full_name"].(string); ok {
					repositoryName = s
				}
				if owner, ok := repo["owner"].(map[string]any); ok {
					if s, ok := owner["login"].(string); ok {
						repositoryOwner = s
					}
				}
				if s, ok := repo["html_url"].(string); ok {
					repositoryURL = s
				}
				if org, ok := repo["organization"].(map[string]any); ok {
					if s, ok := org["login"].(string); ok {
						parentOrg = s
					}
				}
			}
			if inst, ok := reqBody["installation"].(map[string]any); ok {
				installationID = inst["id"]
			}
		}
	}

	msg := fmt.Sprintf(`<li>stage: %s</li>
    <li>event type: %s</li>
    <li>user login: %v</li>
    <li>repository id: %v</li>
    <li>repository name: %v</li>
    <li>repository owner: %v</li>
    <li>repository url: %v</li>
    <li>parent organization: %v</li>
    <li>installation_id: %v</li>`, stage, eventType, userLogin, repositoryID, repositoryName, repositoryOwner, repositoryURL, parentOrg, installationID)

	body := fmt.Sprintf(`
    <p>Hello EasyCLA Maintainer,</p>
    <p>This is a notification email from EasyCLA regarding failure of webhook secret validation.</p>
    <p>Validation Failed:</p>
    <ul>%s</ul>
    <p>Please verify the EasyCLA settings to ensure EasyCLA webhook secret is set correctly. \
    See: <a href="https://github.com/organizations/LF-Engineering/settings/apps"> EasyCLA app settings</a>. \
    <p>For more information on how to setup GitHub webhook secret, please consult About Securing Your Webhooks\
    <a href="https://docs.github.com/en/free-pro-team@latest/developers/webhooks-and-events/securing-your-webhooks"> \
    in the GitHub Online Help Pages</a>.</p>
    %s
    `, msg, emailSignOffContent())
	body = "<p>" + strings.ReplaceAll(body, "\n", "<br>") + "</p>"
	subject := "EasyCLA: Webhook Secret Failure"

	svc, err := email.NewFromEnv(ctx)
	if err != nil {
		logging.Warnf("email service init failed (webhook secret alert): %v", err)
		return
	}
	if err := svc.Send(ctx, subject, body, maintainers); err != nil {
		logging.Warnf("failed to send webhook secret alert email: %v", err)
	}
}

// v4BaseURL resolves the base URL for the v4 Go backend behind the platform gateway.
//
// This mirrors the legacy Python webhook-forwarding logic by normalizing
// PLATFORM_GATEWAY_URL to the v4 service base.
func (h *Handlers) v4BaseURL() (string, error) {
	baseURL := strings.TrimSpace(os.Getenv("PLATFORM_GATEWAY_URL"))
	if baseURL == "" {
		return "", errors.New("PLATFORM_GATEWAY_URL is empty")
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// PLATFORM_GATEWAY_URL is stored in SSM and may be either:
	//   - https://api-gw.example.org              (gateway root)
	//   - https://api-gw.example.org/cla-service  (service root)
	//   - https://api-gw.example.org/cla-service/v4
	// We normalize to the v4 service base.
	if strings.Contains(baseURL, "/cla-service/v4") {
		return baseURL, nil
	}
	if strings.HasSuffix(baseURL, "/cla-service") {
		return baseURL + "/v4", nil
	}
	if strings.Contains(baseURL, "v4") {
		// If the URL already contains v4 (non-standard form), keep it unchanged.
		return baseURL, nil
	}
	baseURL = baseURL + "/cla-service/v4"
	return baseURL, nil
}

// doRequestToV4 performs a request to the v4 Go backend.
//
// It forwards request headers (excluding Host) and returns the raw response
// (status, headers, body). The response body is read and the response is closed.
func (h *Handlers) doRequestToV4(ctx context.Context, method, pathWithQuery string, headers http.Header, body []byte) (int, http.Header, []byte, error) {
	baseURL, err := h.v4BaseURL()
	if err != nil {
		return 0, nil, nil, err
	}
	url := baseURL + pathWithQuery
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, err
	}
	for k, vals := range headers {
		if strings.EqualFold(k, "Host") {
			continue
		}
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	// Limit for safety; signing responses are small JSON payloads.
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return 0, nil, nil, err
	}
	return resp.StatusCode, resp.Header.Clone(), b, nil
}

func copyV4ResponseHeaders(dst http.ResponseWriter, src http.Header) {
	for k, vals := range src {
		// Do not forward hop-by-hop or length/transfer headers.
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Transfer-Encoding") || strings.EqualFold(k, "Connection") {
			continue
		}
		for _, v := range vals {
			dst.Header().Add(k, v)
		}
	}
}

func translateLegacyIndividualSignatureV4Error(providerType string, status int, body []byte) ([]byte, bool) {
	providerType = strings.ToLower(strings.TrimSpace(providerType))
	if status < 400 {
		return body, false
	}
	if providerType != "github" && providerType != "gitlab" {
		return body, false
	}

	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body, false
	}
	if !strings.Contains(strings.ToLower(strings.TrimSpace(payload.Message)), "no emails found") {
		return body, false
	}

	translated, err := json.Marshal(map[string]any{
		"errors": map[string]any{
			"user_id": fmt.Sprintf("no %s user_emails found", providerType),
		},
	})
	if err != nil {
		return body, false
	}
	return translated, true
}

func headerCloneForV4(in http.Header) http.Header {
	out := make(http.Header, len(in))
	for k, vals := range in {
		if strings.EqualFold(k, "Host") {
			continue
		}
		// Avoid stale lengths from the incoming request (e.g., GET -> POST reuse).
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vals {
			out.Add(k, v)
		}
	}
	// Ensure JSON content negotiation.
	if out.Get("Content-Type") == "" {
		out.Set("Content-Type", "application/json")
	}
	if out.Get("Accept") == "" {
		out.Set("Accept", "application/json")
	}
	return out
}

func extractSignURLFromPayload(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return ""
	}
	for _, k := range []string{"sign_url", "signUrl", "signURL"} {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	// Some implementations wrap under "signature".
	if v, ok := m["signature"]; ok {
		if mm, ok := v.(map[string]any); ok {
			for _, k := range []string{"sign_url", "signUrl", "signURL"} {
				if vv, ok := mm[k]; ok {
					if s, ok := vv.(string); ok {
						return strings.TrimSpace(s)
					}
				}
			}
		}
	}
	return ""
}

func deriveReturnURLType(sig map[string]types.AttributeValue) string {
	if sig == nil {
		return "github"
	}
	if v := strings.ToLower(strings.TrimSpace(getAttrString(sig, "signature_return_url_type"))); v != "" {
		return v
	}
	if v := strings.ToLower(strings.TrimSpace(getAttrString(sig, "signature_return_url"))); v != "" {
		// Heuristics only; Python stores return_url_type explicitly.
		switch {
		case strings.Contains(v, "gitlab"):
			return "gitlab"
		case strings.Contains(v, "gerrit") || strings.Contains(v, "review"):
			return "gerrit"
		default:
			return "github"
		}
	}
	return "github"
}

// regenerateIndividualSignURLViaV4 attempts to obtain a fresh embedded signing URL for an individual signature.
//
// Legacy Python regenerates via the DocuSign signing service. For minimal-effort migration, we delegate to the
// v4 backend by reissuing the request-individual-signature call with the signature's stored return URL.
func (h *Handlers) regenerateIndividualSignURLViaV4(ctx context.Context, sig map[string]types.AttributeValue, hdr http.Header) (string, error) {
	projectID := strings.TrimSpace(getAttrString(sig, "signature_project_id"))
	userID := strings.TrimSpace(getAttrString(sig, "signature_reference_id"))
	returnURL := strings.TrimSpace(getAttrString(sig, "signature_return_url"))
	if projectID == "" || userID == "" || returnURL == "" {
		return "", nil
	}
	reqBody := map[string]any{
		"project_id":      projectID,
		"user_id":         userID,
		"return_url_type": deriveReturnURLType(sig),
		"return_url":      returnURL,
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	status, _, respBody, err := h.doRequestToV4(ctx, http.MethodPost, "/request-individual-signature", headerCloneForV4(hdr), b)
	if err != nil {
		return "", err
	}
	if status >= 400 {
		// Legacy Hug behavior often used 200+errors payloads; treat non-2xx as "no regeneration".
		logging.Warnf("v4 request-individual-signature returned %d during ttl_expired regen: %s", status, string(respBody))
		return "", nil
	}
	signURL := extractSignURLFromPayload(respBody)
	if signURL == "" {
		return "", nil
	}
	// Best-effort: persist updated sign_url for future redirects.
	sig["signature_sign_url"] = &types.AttributeValueMemberS{Value: signURL}
	sig["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}
	if h.signatures != nil {
		if err := h.signatures.PutItem(ctx, sig); err != nil {
			logging.Warnf("failed to persist regenerated sign_url (signature_id=%s): %v", getAttrString(sig, "signature_id"), err)
		}
	}
	return signURL, nil
}

// GET /v2/health
// Python: cla/routes.py:614 get_health()
// Calls: cla.salesforce.get_projects

func (h *Handlers) GetHealthV2(w http.ResponseWriter, r *http.Request) {
	// Legacy Python returns request.headers as the response payload.
	//
	// In the Hug/Falcon implementation, header names are upper-cased (e.g. HOST).
	// In Go net/http, Host is exposed as r.Host (and is not always present in r.Header).
	//
	// We flatten multi-value headers into a comma-separated string.
	out := make(map[string]string, len(r.Header)+1)
	if strings.TrimSpace(r.Host) != "" {
		out["HOST"] = r.Host
	}
	for k, vals := range r.Header {
		out[strings.ToUpper(k)] = strings.Join(vals, ",")
	}

	// Python sets a session marker (request.context["session"]["health"]= "up").
	if sess := middleware.SessionFromContext(r.Context()); sess != nil {
		sessionSetString(sess, "health", "up")
	}

	respond.JSON(w, http.StatusOK, out)
}

// GET /v2/user/{user_id}
// Python: cla/routes.py:632 get_user()
// Calls: cla.controllers.user.get_user

func (h *Handlers) GetUserV2(w http.ResponseWriter, r *http.Request) {
	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}
	if h.users == nil {
		respond.JSON(w, http.StatusInternalServerError, "users store not configured")
		return
	}

	ctx := r.Context()
	userID := chi.URLParam(r, "user_id")
	if _, err := uuid.Parse(userID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}
	userItem, found, err := h.users.GetByID(ctx, userID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]string{"user_id": "User not found"}})
		return
	}

	userDict := store.ItemToInterfaceMap(userItem)

	// Legacy python adds is_sanctioned based on the user's company, if present.
	isSanctioned := false
	if h.companies != nil {
		if cid, ok := userDict["user_company_id"].(string); ok {
			cid = strings.TrimSpace(cid)
			if cid != "" {
				companyItem, foundCompany, cerr := h.companies.GetByID(ctx, cid)
				if cerr != nil {
					respond.JSON(w, http.StatusInternalServerError, cerr.Error())
					return
				}
				if foundCompany {
					if av, ok := companyItem["is_sanctioned"].(*types.AttributeValueMemberBOOL); ok {
						isSanctioned = av.Value
					}
				}
			}
		}
	}
	userDict["is_sanctioned"] = isSanctioned
	normalizeUserDict(userDict)

	respond.JSON(w, http.StatusOK, userDict)
}

// POST /v1/user/gerrit
// Python: cla/routes.py:651 post_or_get_user_gerrit()
// Calls: cla.controllers.user.get_or_create_user

func (h *Handlers) PostOrGetUserGerritV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	item, _, err := h.getOrCreateUser(ctx, authUser)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	userDict := store.ItemToInterfaceMap(item)
	normalizeUserDict(userDict)
	respond.JSON(w, http.StatusOK, userDict)
}

// GET /v1/user/{user_id}/signatures
// Python: cla/routes.py:662 get_user_signatures()
// Calls: cla.controllers.user.get_user_signatures

func (h *Handlers) GetUserSignaturesV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	userID := chi.URLParam(r, "user_id")
	if _, err := uuid.Parse(userID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}
	_, found, err := h.users.GetByID(ctx, userID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"user_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user_id": "User not found"}})
		return
	}

	items, err := h.signatures.QueryByReferenceID(ctx, userID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		// user.get_user_signatures() always queries with signature_reference_type == "user"
		if getAttrString(it, "signature_reference_type") != "user" {
			continue
		}
		m := store.ItemToInterfaceMap(it)
		delete(m, "user_docusign_raw_xml")
		out = append(out, m)
	}

	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/users/company/{user_company_id}
// Python: cla/routes.py:672 get_users_company()
// Calls: cla.controllers.user.get_users_company

func (h *Handlers) GetUsersCompanyV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	companyID := chi.URLParam(r, "user_company_id")
	if _, err := uuid.Parse(companyID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_company_id": "invalid uuid"}})
		return
	}
	items, err := h.users.QueryByCompanyID(ctx, companyID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		m := store.ItemToInterfaceMap(it)
		normalizeUserDict(m)
		out = append(out, m)
	}
	respond.JSON(w, http.StatusOK, out)
}

// POST /v2/user/{user_id}/request-company-whitelist/{company_id}
// Python: cla/routes.py:684 request_company_allowlist()
// Calls: cla.controllers.user.request_company_allowlist

func (h *Handlers) RequestCompanyAllowlistV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.users == nil || h.companies == nil || h.projects == nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": "stores not configured"}})
		return
	}

	userID := chi.URLParam(r, "user_id")
	companyID := chi.URLParam(r, "company_id")
	if _, err := uuid.Parse(userID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}
	if _, err := uuid.Parse(companyID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"company_id": "invalid uuid"}})
		return
	}

	params, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}

	userNameVal, hasUserName := flexibleStringParam(r, params, "user_name")
	userEmailVal, hasUserEmail := flexibleStringParam(r, params, "user_email")
	projectID, hasProjectID := flexibleStringParam(r, params, "project_id")
	messageVal, hasMessage := flexibleStringParam(r, params, "message")
	recipientNameVal, hasRecipientName := flexibleStringParam(r, params, "recipient_name")
	recipientEmailVal, hasRecipientEmail := flexibleStringParam(r, params, "recipient_email")

	missingRequired := map[string]any{}
	if !hasUserName {
		missingRequired["user_name"] = "User Name is missing from the request"
	}
	if !hasUserEmail {
		missingRequired["user_email"] = "User Email is missing from the request"
	}
	if !hasProjectID {
		missingRequired["project_id"] = "Project ID is missing from the request"
	}
	if len(missingRequired) > 0 {
		// Hug enforces required parameters at the routing layer and returns 400.
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missingRequired})
		return
	}
	if !validEmailLikePython(userEmailVal) {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_email": "Invalid email address specified"}})
		return
	}

	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	// In Python routes.py, message/recipient_name/recipient_email are optional in Hug but
	// required by the controller; missing values produce a 200 response with an errors dict.
	if !hasMessage {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"message": "Message is missing from the request"}})
		return
	}
	if !hasRecipientName {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"recipient_name": "Recipient Name is missing from the request"}})
		return
	}
	if !hasRecipientEmail {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"recipient_email": "Recipient Email is missing from the request"}})
		return
	}
	if !validEmailLikePython(recipientEmailVal) {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"recipient_email": "Invalid email address specified"}})
		return
	}

	user, found, err := h.users.GetByID(ctx, userID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"user_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user_id": "User not found"}})
		return
	}

	userEmail := userEmailVal
	userEmails := getAttrStringSlice(user, "user_emails")
	if !stringSliceContainsExact(userEmails, userEmail) {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user_email": "User's email must match one of the user's existing emails in their profile"}})
		return
	}

	company, found, err := h.companies.GetByID(ctx, companyID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"company_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"company_id": "Company not found"}})
		return
	}

	project, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	userName := userNameVal
	projectName := getAttrString(project, "project_name")
	companyName := getAttrString(company, "company_name")

	subject := fmt.Sprintf("EasyCLA: Request to Authorize %s for %s", userName, projectName)
	corpURL := corporateCompanyURL(companyID)
	companyStr := pythonCompanyString(company)
	message := messageVal
	msg := fmt.Sprintf("<p>%s included the following message in the request:</p><p>%s</p>", userName, message)
	body := fmt.Sprintf(`
<p>Hello %s,</p> \
<p>This is a notification email from EasyCLA regarding the project %s.</p> \
<p>%s (%s) has requested to be added to the Approved List as an authorized contributor from \
%s to the project %s. You are receiving this message as a CLA Manager from %s for \
%s.</p> \
%s \
<p>If you want to add them to the Approved List, please \
<a href="%s" target="_blank">log into the EasyCLA Corporate \
Console</a>, where you can approve this user's request by selecting the 'Manage Approved List' and adding the \
contributor's email, the contributor's entire email domain, their GitHub ID or the entire GitHub Organization for the \
repository. This will permit them to begin contributing to %s on behalf of %s.</p> \
<p>If you are not certain whether to add them to the Approved List, please reach out to them directly to discuss.</p> 
`, recipientNameVal, projectName, userName, userEmail, companyName, projectName, companyStr, projectName, msg, corpURL, projectName, companyStr)
	body = appendEmailHelpSignOffContent(body, getAttrString(project, "version"))

	svc, err := email.NewFromEnv(ctx)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if err := svc.Send(ctx, subject, body, []string{recipientEmailVal}); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	eventMsg := fmt.Sprintf("CLA: contributor %s requests to be Approved for the project: %s organization: %s as %s <%s>", userName, projectName, companyName, userName, userEmail)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "RequestCompanyWL",
		EventUserID:     userID,
		EventCompanyID:  companyID,
		EventCLAGroupID: projectID,
		EventData:       eventMsg,
		EventSummary:    eventMsg,
		ContainsPII:     true,
	})

	// Python returns None => JSON null
	respond.JSON(w, http.StatusOK, nil)
}

// POST /v2/user/{user_id}/invite-company-admin
// Python: cla/routes.py:709 invite_company_admin()
// Calls: cla.controllers.user.invite_cla_manager

func (h *Handlers) InviteCompanyAdminV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.users == nil || h.projects == nil || h.companies == nil || h.companyInvites == nil || h.cclaAllowlistReqs == nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": "stores not configured"}})
		return
	}

	contributorID := chi.URLParam(r, "user_id")
	if _, err := uuid.Parse(contributorID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}

	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}

	contributorName, hasContributorName := flexibleStringParam(r, body, "contributor_name")
	contributorEmail, hasContributorEmail := flexibleStringParam(r, body, "contributor_email")
	claManagerName, hasCLAManagerName := flexibleStringParam(r, body, "cla_manager_name")
	claManagerEmail, hasCLAManagerEmail := flexibleStringParam(r, body, "cla_manager_email")
	projectName, hasProjectName := flexibleStringParam(r, body, "project_name")
	companyName, hasCompanyName := flexibleStringParam(r, body, "company_name")

	missing := map[string]any{}
	if !hasContributorName {
		missing["contributor_name"] = "Contributor Name is missing from the request"
	}
	if !hasContributorEmail {
		missing["contributor_email"] = "Contributor Email is missing from the request"
	}
	if !hasCLAManagerName {
		missing["cla_manager_name"] = "CLA Manager Name is missing from the request"
	}
	if !hasCLAManagerEmail {
		missing["cla_manager_email"] = "CLA Manager Email is missing from the request"
	}
	if !hasProjectName {
		missing["project_name"] = "Project Name is missing from the request"
	}
	if !hasCompanyName {
		missing["company_name"] = "Company Name is missing from the request"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}
	if !validEmailLikePython(contributorEmail) {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"contributor_email": "Invalid email address specified"}})
		return
	}
	if !validEmailLikePython(claManagerEmail) {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"cla_manager_email": "Invalid email address specified"}})
		return
	}

	user, found, err := h.users.GetByID(ctx, contributorID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"user_id": err.Error()}})
		return
	}
	if !found {
		errStr := "User not found"
		msg := fmt.Sprintf("unable to load user by id: %s for inviting company admin - error: %s", contributorID, errStr)
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user_id": contributorID, "message": msg, "error": errStr}})
		return
	}

	projects, err := h.projects.QueryByNameLower(ctx, projectName)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_name": err.Error()}})
		return
	}
	if len(projects) == 0 {
		errStr := fmt.Sprintf("Project with name %s not found", projectName)
		msg := fmt.Sprintf("unable to load project by name: %s for inviting company admin - error: %s", projectName, errStr)
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_name": projectName, "message": msg, "error": errStr}})
		return
	}
	project := projects[0]
	projectID := getAttrString(project, "project_id")

	companies, err := h.companies.QueryByName(ctx, companyName)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"company_name": err.Error()}})
		return
	}
	var company map[string]types.AttributeValue
	if len(companies) == 0 {
		// Create a new company when absent (matches Python behavior).
		newID := uuid.New().String()
		now := formatPynamoDateTimeUTC(time.Now())
		company = map[string]types.AttributeValue{
			"company_id":    &types.AttributeValueMemberS{Value: newID},
			"company_name":  &types.AttributeValueMemberS{Value: companyName},
			"date_created":  &types.AttributeValueMemberS{Value: now},
			"date_modified": &types.AttributeValueMemberS{Value: now},
			"version":       &types.AttributeValueMemberS{Value: "v1"},
		}
		if err := h.companies.PutItem(ctx, company); err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"company": err.Error()}})
			return
		}
	} else {
		company = companies[0]
	}
	companyID := getAttrString(company, "company_id")

	// Add LF username (or fallback to user_name) to company_acl.
	username := strings.TrimSpace(getAttrString(user, "lf_username"))
	if username == "" {
		username = strings.TrimSpace(getAttrString(user, "user_name"))
	}
	if username != "" {
		acl := getAttrStringSlice(company, "company_acl")
		if !stringSliceContainsExact(acl, username) {
			acl = append(acl, username)
			company["company_acl"] = &types.AttributeValueMemberSS{Value: acl}
			company["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now())}
			if err := h.companies.PutItem(ctx, company); err != nil {
				respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"company": err.Error()}})
				return
			}
		}
	}

	// Create CompanyInvite record (Python writes this before sending the email).
	inviteID := uuid.New().String()
	invNow := formatPynamoDateTimeUTC(time.Now())
	inviteItem := map[string]types.AttributeValue{
		"company_invite_id":    &types.AttributeValueMemberS{Value: inviteID},
		"requested_company_id": &types.AttributeValueMemberS{Value: companyID},
		"user_id":              &types.AttributeValueMemberS{Value: contributorID},
		"date_created":         &types.AttributeValueMemberS{Value: invNow},
		"date_modified":        &types.AttributeValueMemberS{Value: invNow},
		"version":              &types.AttributeValueMemberS{Value: "v1"},
	}
	if err := h.companyInvites.PutItem(ctx, inviteItem); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"company_invite": err.Error()}})
		return
	}

	// Python: if contributor_name is None, use user.get_user_name().
	if !hasContributorName {
		if u := strings.TrimSpace(getAttrString(user, "user_name")); u != "" {
			contributorName = u
		}
	}

	logMsg := fmt.Sprintf("sent email to CLA Manager: %s with email %s for project %s and company %s to user %s with email %s", claManagerName, claManagerEmail, projectName, companyName, contributorName, contributorEmail)

	if err := sendEmailToCLAManager(ctx, project, contributorName, contributorEmail, claManagerName, claManagerEmail, companyName, false); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	// Create allowlist request record.
	reqID := uuid.New().String()
	now := formatPynamoDateTimeUTC(time.Now())
	allowItem := map[string]types.AttributeValue{
		"request_id":           &types.AttributeValueMemberS{Value: reqID},
		"company_name":         &types.AttributeValueMemberS{Value: companyName},
		"project_name":         &types.AttributeValueMemberS{Value: projectName},
		"user_github_id":       &types.AttributeValueMemberS{Value: contributorID},
		"user_github_username": &types.AttributeValueMemberS{Value: contributorName},
		"user_emails":          &types.AttributeValueMemberSS{Value: []string{contributorEmail}},
		"request_status":       &types.AttributeValueMemberS{Value: "pending"},
		"date_created":         &types.AttributeValueMemberS{Value: now},
		"date_modified":        &types.AttributeValueMemberS{Value: now},
		"version":              &types.AttributeValueMemberS{Value: "v1"},
	}
	if err := h.cclaAllowlistReqs.PutItem(ctx, allowItem); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"ccla_allowlist_request": err.Error()}})
		return
	}

	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:        "InviteAdmin",
		EventUserID:      contributorID,
		EventProjectName: projectName,
		EventCLAGroupID:  projectID,
		EventData:        logMsg,
		EventSummary:     logMsg,
		ContainsPII:      true,
	})

	respond.JSON(w, http.StatusOK, nil)
}

// POST /v2/user/{user_id}/request-company-ccla
// Python: cla/routes.py:740 request_company_ccla()
// Calls: cla.controllers.user.request_company_ccla

func (h *Handlers) RequestCompanyCclaV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.users == nil || h.companies == nil || h.projects == nil || h.cclaAllowlistReqs == nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": "stores not configured"}})
		return
	}

	userID := chi.URLParam(r, "user_id")
	if _, err := uuid.Parse(userID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}

	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}

	userEmail, hasUserEmail := flexibleStringParam(r, body, "user_email")
	companyID, hasCompanyID := flexibleStringParam(r, body, "company_id")
	projectID, hasProjectID := flexibleStringParam(r, body, "project_id")

	missing := map[string]any{}
	if !hasUserEmail {
		missing["user_email"] = "User Email is missing from the request"
	}
	if !hasCompanyID {
		missing["company_id"] = "Company ID is missing from the request"
	}
	if !hasProjectID {
		missing["project_id"] = "Project ID is missing from the request"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}
	if !validEmailLikePython(userEmail) {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_email": "Invalid email address specified"}})
		return
	}
	if _, err := uuid.Parse(companyID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"company_id": "invalid uuid"}})
		return
	}
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	user, found, err := h.users.GetByID(ctx, userID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"user_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user_id": "User not found"}})
		return
	}

	company, found, err := h.companies.GetByID(ctx, companyID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"company_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"company_id": "Company not found"}})
		return
	}

	project, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}
	projectName := getAttrString(project, "project_name")
	companyName := getAttrString(company, "company_name")

	// EASYCLA_PARITY_FLAG: default preserves the legacy Python crash paths in request_company_ccla():
	// Company.get_managers_by_company_acl() crashes on missing usernames, and send_email_to_cla_manager()
	// crashes with the wrong positional arity when at least one manager resolves.
	if !parity.FixRequestCompanyCclaV2 {
		managers, crashMsg, qerr := h.companyManagersLikePython(ctx, company)
		if qerr != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"user": qerr.Error()}})
			return
		}
		if crashMsg != "" {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{
				"errors": map[string]any{
					"server": "legacy python parity: " + crashMsg,
				},
			})
			return
		}
		if len(managers) > 0 {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{
				"errors": map[string]any{
					"server": "legacy python parity: send_email_to_cla_manager() takes 7 positional arguments but 8 were given",
				},
			})
			return
		}
	} else {
		contributorName := strings.TrimSpace(getAttrString(user, "user_name"))
		if contributorName == "" {
			contributorName = strings.TrimSpace(getAttrString(user, "lf_username"))
		}
		if contributorName == "" {
			contributorName = userEmail
		}
		managers, qerr := h.companyManagersForSending(ctx, company)
		if qerr != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"user": qerr.Error()}})
			return
		}
		for _, admin := range managers {
			claManagerName := strings.TrimSpace(getAttrString(admin, "user_name"))
			if claManagerName == "" {
				claManagerName = strings.TrimSpace(getAttrString(admin, "lf_username"))
			}
			if claManagerName == "" {
				claManagerName = getAttrString(admin, "user_id")
			}
			claManagerEmail := strings.TrimSpace(getUserEmailLikePython(admin))
			if claManagerEmail == "" {
				respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": fmt.Sprintf("CLA manager %s does not have an email address", claManagerName)}})
				return
			}
			if err := sendEmailToCLAManager(ctx, project, contributorName, userEmail, claManagerName, claManagerEmail, companyName, true); err != nil {
				respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
				return
			}
		}
	}

	// Legacy Python records a generic event message and explicitly marks it as not containing PII.
	eventMsg := fmt.Sprintf("Sent email to sign ccla for %s", projectName)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "RequestCCLA",
		EventUserID:     userID,
		EventCompanyID:  companyID,
		EventCLAGroupID: projectID,
		EventData:       eventMsg,
		EventSummary:    eventMsg,
		ContainsPII:     false,
	})

	// Create allowlist request record.
	reqID := uuid.New().String()
	now := formatPynamoDateTimeUTC(time.Now())
	allowItem := map[string]types.AttributeValue{
		"request_id":     &types.AttributeValueMemberS{Value: reqID},
		"company_name":   &types.AttributeValueMemberS{Value: companyName},
		"project_name":   &types.AttributeValueMemberS{Value: projectName},
		"user_emails":    &types.AttributeValueMemberSS{Value: []string{userEmail}},
		"request_status": &types.AttributeValueMemberS{Value: "pending"},
		"date_created":   &types.AttributeValueMemberS{Value: now},
		"date_modified":  &types.AttributeValueMemberS{Value: now},
		"version":        &types.AttributeValueMemberS{Value: "v1"},
	}
	if v := strings.TrimSpace(getAttrString(user, "user_github_id")); v != "" {
		allowItem["user_github_id"] = &types.AttributeValueMemberS{Value: v}
	}
	if v := strings.TrimSpace(getAttrString(user, "user_github_username")); v != "" {
		allowItem["user_github_username"] = &types.AttributeValueMemberS{Value: v}
	}
	if err := h.cclaAllowlistReqs.PutItem(ctx, allowItem); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"ccla_allowlist_request": err.Error()}})
		return
	}

	respond.JSON(w, http.StatusOK, nil)
}

func stripScheme(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimRight(s, "/")
	return s
}

func corporateCompanyURL(companyID string) string {
	base := stripScheme(os.Getenv("CLA_CORPORATE_BASE"))
	if base == "" {
		// Fallback for environments using only the v2 base variable.
		base = stripScheme(os.Getenv("CLA_CORPORATE_V2_BASE"))
	}
	if base == "" {
		return ""
	}
	return fmt.Sprintf("https://%s#/company/%s", base, companyID)
}

func validEmailLikePython(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if addr, err := stdmail.ParseAddress(value); err == nil {
		return strings.Contains(addr.Address, "@")
	}
	return strings.Contains(value, "@")
}

func pythonStringSet(values []string) string {
	if len(values) == 0 {
		return "set()"
	}
	vals := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		vals = append(vals, v)
	}
	sort.Strings(vals)
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		// Best-effort; we do not attempt full Python escaping here.
		parts = append(parts, fmt.Sprintf("'%s'", v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func pythonCompanyString(company map[string]types.AttributeValue) string {
	if company == nil {
		return "None"
	}
	acl := pythonStringSet(getAttrStringSlice(company, "company_acl"))
	return fmt.Sprintf(
		"id:%s, name: %s, signing_entity_name: %s, external id: %s, manager id: %s, is_sanctioned: %s, acl: %s, note: %s",
		getAttrString(company, "company_id"),
		getAttrString(company, "company_name"),
		getAttrString(company, "signing_entity_name"),
		getAttrString(company, "company_external_id"),
		getAttrString(company, "company_manager_id"),
		boolString(getAttrBool(company, "is_sanctioned")),
		acl,
		getAttrString(company, "note"),
	)
}

func emailHelpContent(showV2HelpLink bool) string {
	// Legacy Python: cla/utils.py:get_email_help_content
	helpLink := "https://docs.linuxfoundation.org/lfx/easycla"
	if showV2HelpLink {
		// v2 help link is currently the same as v1 in legacy Python.
		helpLink = "https://docs.linuxfoundation.org/lfx/easycla"
	}
	return fmt.Sprintf(`<p>If you need help or have questions about EasyCLA, you can <a href="%s" target="_blank">read the documentation</a> or <a href="https://jira.linuxfoundation.org/servicedesk/customer/portal/4/create/143" target="_blank">reach out to us for support</a>.</p>`, helpLink)
}

func emailSignOffContent() string {
	// Legacy Python: cla/utils.py:get_email_sign_off_content
	return "<p>Thanks,</p><p>EasyCLA Support Team</p>"
}

func appendEmailHelpSignOffContent(body string, projectVersion string) string {
	// Legacy Python: cla/utils.py:append_email_help_sign_off_content
	showV2 := strings.TrimSpace(strings.ToLower(projectVersion)) == "v2"
	return body + emailHelpContent(showV2) + emailSignOffContent()
}

func sendEmailToCLAManager(ctx context.Context, project map[string]types.AttributeValue, contributorName string, contributorEmail string, claManagerName string, claManagerEmail string, companyName string, accountExists bool) error {
	_ = accountExists // Legacy Python accepts this flag but does not currently vary content.
	projectName := getAttrString(project, "project_name")
	projectVersion := getAttrString(project, "version")
	subject := fmt.Sprintf("EasyCLA: Request to start CLA signature process for %s", projectName)

	landing := os.Getenv("CLA_LANDING_PAGE")
	if strings.TrimSpace(companyName) == "" {
		companyName = "your company"
	}

	body := fmt.Sprintf(`
<p>Hello %s,</p> \
<p>This is a notification email from EasyCLA regarding the project %s.</p> \
<p>%s uses EasyCLA to ensure that before a contribution is accepted, the contributor is \
covered under a signed CLA.</p> \
<p>%s (%s) has designated you as the proposed initial CLA Manager for contributions \
from %s to %s. This would mean that, after the \
CLA is signed, you would be able to maintain the list of employees allowed to contribute to %s \
on behalf of your company, as well as the list of your company’s CLA Managers for %s.</p> \
<p>If you can be the initial CLA Manager from your company for %s, please log into the EasyCLA \
Corporate Console at %s to begin the CLA signature process. You might not be authorized to \
sign the CLA yourself on behalf of your company; if not, the signature process will prompt you to designate somebody \
else who is authorized to sign the CLA.</p> \
%s
%s
`, claManagerName, projectName, projectName, contributorName, contributorEmail, companyName, projectName, projectName, projectName, projectName, landing, emailHelpContent(strings.TrimSpace(strings.ToLower(projectVersion)) == "v2"), emailSignOffContent())

	svc, err := email.NewFromEnv(ctx)
	if err != nil {
		return err
	}
	return svc.Send(ctx, subject, body, []string{claManagerEmail})
}
func (h *Handlers) companyManagersLikePython(ctx context.Context, company map[string]types.AttributeValue) ([]map[string]types.AttributeValue, string, error) {
	if h == nil || h.users == nil {
		return nil, "", fmt.Errorf("users store not configured")
	}
	if company == nil {
		return nil, "'NoneType' object is not iterable", nil
	}
	aclAV, ok := company["company_acl"]
	if !ok || aclAV == nil {
		// Python: for username in None -> TypeError before any side effects.
		return nil, "'NoneType' object is not iterable", nil
	}

	usernames := getAttrStringSlice(company, "company_acl")
	if usernames == nil {
		return nil, "'NoneType' object is not iterable", nil
	}

	managers := make([]map[string]types.AttributeValue, 0, len(usernames))
	for _, username := range usernames {
		username = strings.TrimSpace(username)
		users, err := h.users.QueryByLFUsername(ctx, username)
		if err != nil {
			return nil, "", err
		}
		if len(users) > 1 {
			logging.Warnf("More than one user record returned for username: %s", username)
		}
		if users == nil {
			// Python: len(None) raises TypeError before the caller starts emailing managers.
			return nil, "object of type 'NoneType' has no len()", nil
		}
		if len(users) == 0 {
			// Python: users[0] on an empty list raises IndexError.
			return nil, "list index out of range", nil
		}
		managers = append(managers, users[0])
	}
	return managers, "", nil
}

func (h *Handlers) companyManagersForSending(ctx context.Context, company map[string]types.AttributeValue) ([]map[string]types.AttributeValue, error) {
	if h == nil || h.users == nil {
		return nil, fmt.Errorf("users store not configured")
	}
	if company == nil {
		return nil, nil
	}
	usernames := getAttrStringSlice(company, "company_acl")
	if len(usernames) == 0 {
		return nil, nil
	}
	managers := make([]map[string]types.AttributeValue, 0, len(usernames))
	for _, username := range usernames {
		username = strings.TrimSpace(username)
		if username == "" {
			continue
		}
		users, err := h.users.QueryByLFUsername(ctx, username)
		if err != nil {
			return nil, err
		}
		if len(users) > 1 {
			logging.Warnf("More than one user record returned for username: %s", username)
		}
		if len(users) == 0 {
			continue
		}
		managers = append(managers, users[0])
	}
	return managers, nil
}

func (h *Handlers) loadActiveSignatureMetadata(ctx context.Context, userID string) (map[string]any, bool, error) {
	if h.kv == nil {
		return nil, false, fmt.Errorf("kv store not configured")
	}
	key := fmt.Sprintf("active_signature:%s", userID)
	val, ok, err := h.kv.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	var metadata map[string]any
	if strings.TrimSpace(val) != "" {
		if uerr := json.Unmarshal([]byte(val), &metadata); uerr != nil {
			return nil, true, uerr
		}
	}
	return metadata, true, nil
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	v, ok := metadata[key]
	if !ok || v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", v))
	if s == "<nil>" {
		return ""
	}
	return s
}

func (h *Handlers) computeReturnURLFromActiveSignatureMetadata(ctx context.Context, metadata map[string]any) (string, error) {
	if metadata == nil {
		return "", nil
	}

	if returnURL := metadataString(metadata, "return_url"); returnURL != "" {
		return returnURL, nil
	}

	if _, isGitLab := metadata["merge_request_id"]; isGitLab {
		return "", nil
	}

	// GitHub flow: compute the PR URL through the GitHub API when possible. This
	// matches Python GitHub.get_return_url(), which returns pull_request.html_url.
	repoID := metadataString(metadata, "repository_id")
	prID := metadataString(metadata, "pull_request_id")
	if repoID == "" || prID == "" {
		return "", nil
	}
	if h.github != nil {
		installationID, found, err := h.githubInstallationIDFromRepository(ctx, repoID)
		if err != nil {
			return "", err
		}
		if found {
			installationIDInt, installErr := strconv.ParseInt(strings.TrimSpace(installationID), 10, 64)
			repositoryIDInt, repoErr := strconv.ParseInt(strings.TrimSpace(repoID), 10, 64)
			pullRequestIDInt, prErr := strconv.ParseInt(strings.TrimSpace(prID), 10, 64)
			if installErr == nil && repoErr == nil && prErr == nil {
				returnURL, ghErr := h.github.GetPullRequestHTMLURL(ctx, installationIDInt, repositoryIDInt, pullRequestIDInt)
				if ghErr == nil && strings.TrimSpace(returnURL) != "" {
					return strings.TrimSpace(returnURL), nil
				}
				if ghErr != nil {
					logging.Warnf("active signature return_url: github html_url lookup failed for repo=%s pr=%s installation=%s: %v", repoID, prID, installationID, ghErr)
				}
			}
		}
	}

	// Fallback for tests/local setups where the GitHub service is unavailable.
	// repository_name may be either "repo" or "org/repo"; do not prepend the
	// organization twice.
	if h.repos == nil {
		return "", nil
	}
	repo, found, err := h.repos.GetByExternalIDAndType(ctx, repoID, "github")
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	org := strings.TrimSpace(getAttrString(repo, "repository_organization_name"))
	name := strings.TrimSpace(getAttrString(repo, "repository_name"))
	if name == "" {
		return "", nil
	}
	if strings.Contains(name, "/") {
		return fmt.Sprintf("https://github.com/%s/pull/%s", strings.TrimSuffix(strings.Trim(name, "/"), ".git"), prID), nil
	}
	name = strings.TrimSuffix(name, ".git")
	if org == "" {
		return "", nil
	}
	return fmt.Sprintf("https://github.com/%s/%s/pull/%s", org, name, prID), nil
}

func legacyPythonNilSubscriptError() error {
	return errors.New("TypeError: 'NoneType' object is not subscriptable")
}

func legacyPythonKeyError(key string) error {
	return fmt.Errorf("KeyError: '%s'", key)
}

func pythonIntFromAny(value any) (int64, error) {
	switch v := value.(type) {
	case nil:
		return 0, errors.New("TypeError: int() argument must be a string, a bytes-like object or a real number, not 'NoneType'")
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i, nil
		}
		f, err := v.Float64()
		if err != nil {
			return 0, fmt.Errorf("ValueError: invalid literal for int() with base 10: %q", v.String())
		}
		return int64(f), nil
	case string:
		s := strings.TrimSpace(v)
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("ValueError: invalid literal for int() with base 10: %q", v)
		}
		return i, nil
	default:
		s := fmt.Sprintf("%v", value)
		i, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("ValueError: invalid literal for int() with base 10: %q", s)
		}
		return i, nil
	}
}

func (h *Handlers) githubInstallationIDFromRepository(ctx context.Context, githubRepositoryID string) (string, bool, error) {
	if strings.TrimSpace(githubRepositoryID) == "" || h.repos == nil || h.githubOrgs == nil {
		return "", false, nil
	}
	repo, found, err := h.repos.GetByExternalIDAndType(ctx, githubRepositoryID, "github")
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	orgName := strings.TrimSpace(getAttrString(repo, "repository_organization_name"))
	if orgName == "" {
		return "", false, nil
	}
	org, found, err := h.githubOrgs.GetByName(ctx, orgName)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	installationID := strings.TrimSpace(getAttrString(org, "organization_installation_id"))
	if installationID == "" {
		return "", false, nil
	}
	return installationID, true, nil
}

func (h *Handlers) gitlabOrganizationIDFromRepository(ctx context.Context, gitlabRepositoryID string) (string, bool, error) {
	if strings.TrimSpace(gitlabRepositoryID) == "" || h.repos == nil || h.gitlabOrgs == nil {
		return "", false, nil
	}
	repo, found, err := h.repos.GetByExternalIDAndType(ctx, gitlabRepositoryID, "gitlab")
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	orgNameLower := strings.ToLower(strings.TrimSpace(getAttrString(repo, "repository_organization_name")))
	if orgNameLower == "" {
		return "", false, nil
	}
	items, err := h.gitlabOrgs.ScanAll(ctx)
	if err != nil {
		return "", false, err
	}
	for _, it := range items {
		candidateLower := strings.ToLower(strings.TrimSpace(getAttrString(it, "organization_name_lower")))
		if candidateLower == "" {
			candidateLower = strings.ToLower(strings.TrimSpace(getAttrString(it, "organization_name")))
		}
		if candidateLower != orgNameLower {
			continue
		}
		organizationID := strings.TrimSpace(getAttrString(it, "organization_id"))
		if organizationID == "" {
			return "", false, nil
		}
		return organizationID, true, nil
	}
	return "", false, nil
}

func (h *Handlers) triggerGitHubChangeRequestUpdateV4(ctx context.Context, installationID, githubRepositoryID, changeRequestID string) error {
	payload, err := json.Marshal(map[string]any{
		"installation_id":      installationID,
		"github_repository_id": githubRepositoryID,
		"change_request_id":    changeRequestID,
	})
	if err != nil {
		return err
	}
	headers, err := legacyGitHubInternalTriggerHeaders(payload)
	if err != nil {
		return err
	}
	status, _, respBody, err := h.doRequestToV4(ctx, http.MethodPost, "/github/activity?legacy_internal_trigger=github-change-request", headers, payload)
	if err != nil {
		return err
	}
	if status >= 400 {
		if len(respBody) == 0 {
			return fmt.Errorf("v4 github trigger returned status %d", status)
		}
		return fmt.Errorf("v4 github trigger returned status %d: %s", status, string(respBody))
	}
	return nil
}

func (h *Handlers) triggerGitHubMergeGroupUpdateV4(ctx context.Context, installationID, githubRepositoryID, changeRequestID, mergeGroupSHA string) error {
	payload, err := json.Marshal(map[string]any{
		"installation_id":      installationID,
		"github_repository_id": githubRepositoryID,
		"change_request_id":    changeRequestID,
		"merge_group_sha":      mergeGroupSHA,
	})
	if err != nil {
		return err
	}
	headers, err := legacyGitHubInternalTriggerHeaders(payload)
	if err != nil {
		return err
	}
	status, _, respBody, err := h.doRequestToV4(ctx, http.MethodPost, "/github/activity?legacy_internal_trigger=github-change-request", headers, payload)
	if err != nil {
		return err
	}
	if status >= 400 {
		if len(respBody) == 0 {
			return fmt.Errorf("v4 github merge group trigger returned status %d", status)
		}
		return fmt.Errorf("v4 github merge group trigger returned status %d: %s", status, string(respBody))
	}
	return nil
}

func legacyGitHubInternalTriggerHeaders(payload []byte) (http.Header, error) {
	headers := headerCloneForV4(http.Header{})
	signature, err := githublegacy.SignWebhookPayload(payload)
	if err != nil {
		return nil, err
	}
	headers.Set("Content-Type", "application/json")
	headers.Set("X-Hub-Signature", signature)
	headers.Set("X-EasyCLA-Source-Backend", "cla-backend-legacy")
	return headers, nil
}

func (h *Handlers) triggerGitLabMergeRequestUpdateV4(ctx context.Context, organizationID *string, gitlabRepositoryID, mergeRequestID int64) error {
	payload := map[string]any{
		"gitlab_external_repository_id": gitlabRepositoryID,
		"gitlab_mr_id":                  mergeRequestID,
		"gitlab_organization_id":        nil,
	}
	if organizationID != nil {
		payload["gitlab_organization_id"] = *organizationID
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, _, _, err = h.doRequestToV4(ctx, http.MethodPost, "/gitlab/trigger", headerCloneForV4(http.Header{}), body)
	return err
}

// GET /v2/user/{user_id}/active-signature
// Python: cla/routes.py:765 get_user_active_signature()
// Calls: cla.controllers.user.get_active_signature

func (h *Handlers) GetUserActiveSignatureV2(w http.ResponseWriter, r *http.Request) {
	if h.kv == nil {
		respond.JSON(w, http.StatusInternalServerError, "kv store not configured")
		return
	}
	userID := chi.URLParam(r, "user_id")
	if _, err := uuid.Parse(userID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}

	metadata, ok, err := h.loadActiveSignatureMetadata(r.Context(), userID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		// Legacy Python returns None when no active signature exists.
		respond.JSON(w, http.StatusOK, nil)
		return
	}
	if metadata == nil {
		respond.JSON(w, http.StatusOK, nil)
		return
	}
	if metadataString(metadata, "user_id") == "" {
		metadata["user_id"] = userID
	}
	returnURL, err := h.computeReturnURLFromActiveSignatureMetadata(r.Context(), metadata)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	metadata["return_url"] = returnURL
	respond.JSON(w, http.StatusOK, metadata)
}

// GET /v2/user/{user_id}/project/{project_id}/last-signature
// Python: cla/routes.py:783 get_user_project_last_signature()
// Calls: cla.controllers.user.get_user_project_last_signature

func (h *Handlers) GetUserProjectLastSignatureV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := chi.URLParam(r, "user_id")
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(userID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	if h.users == nil || h.signatures == nil || h.projects == nil {
		respond.JSON(w, http.StatusInternalServerError, "required stores not configured")
		return
	}

	// Validate user exists (legacy behavior).
	_, foundUser, err := h.users.GetByID(ctx, userID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !foundUser {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]string{"user_id": "User not found"}})
		return
	}

	sigs, err := h.signatures.QueryByProjectAndReference(ctx, projectID, userID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	latest := pickLatestSignature(sigs, "")
	if latest == nil {
		respond.JSON(w, http.StatusOK, nil)
		return
	}

	maj, min, err := h.projects.LatestIndividualDocumentVersion(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	latestMajorStr := strconv.Itoa(maj)
	latestMinorStr := strconv.Itoa(min)

	out := store.ItemToInterfaceMap(latest)
	out["latest_document_major_version"] = latestMajorStr
	out["latest_document_minor_version"] = latestMinorStr

	requires := false
	// Legacy Python uses last_signature.get('signature_signed', True).
	// Missing signature_signed is treated as signed.
	signed := true
	if v, ok := out["signature_signed"].(bool); ok {
		signed = v
	}
	if !signed {
		requires = true
	} else {
		sigMajor, _ := out["signature_document_major_version"].(string)
		if strings.TrimSpace(sigMajor) == "" || strings.TrimSpace(sigMajor) != latestMajorStr {
			requires = true
		}
	}
	out["requires_resigning"] = requires

	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/user/{user_id}/project/{project_id}/last-signature/{company_id}
// Python: cla/routes.py:793 get_user_project_company_last_signature()
// Calls: cla.controllers.user.get_user_project_company_last_signature

func (h *Handlers) GetUserProjectCompanyLastSignatureV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := chi.URLParam(r, "user_id")
	projectID := chi.URLParam(r, "project_id")
	companyID := chi.URLParam(r, "company_id")
	if _, err := uuid.Parse(userID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}
	if _, err := uuid.Parse(companyID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"company_id": "invalid uuid"}})
		return
	}

	if h.users == nil || h.signatures == nil || h.projects == nil {
		respond.JSON(w, http.StatusInternalServerError, "required stores not configured")
		return
	}

	// Validate user exists (legacy behavior).
	_, foundUser, err := h.users.GetByID(ctx, userID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !foundUser {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]string{"user_id": "User not found"}})
		return
	}

	sigs, err := h.signatures.QueryByProjectAndReference(ctx, projectID, userID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	latest := pickLatestSignature(sigs, companyID)
	if latest == nil {
		respond.JSON(w, http.StatusOK, nil)
		return
	}

	maj, min, err := h.projects.LatestCorporateDocumentVersion(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	latestMajorStr := strconv.Itoa(maj)
	latestMinorStr := strconv.Itoa(min)

	out := store.ItemToInterfaceMap(latest)
	out["latest_document_major_version"] = latestMajorStr
	out["latest_document_minor_version"] = latestMinorStr

	sigMajor, _ := out["signature_document_major_version"].(string)
	out["requires_resigning"] = strings.TrimSpace(sigMajor) == "" || strings.TrimSpace(sigMajor) != latestMajorStr

	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/signature/{signature_id}
// Python: cla/routes.py:805 get_signature()
// Calls: cla.controllers.signature.get_signature

func (h *Handlers) GetSignatureV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	signatureID := chi.URLParam(r, "signature_id")
	if _, err := uuid.Parse(signatureID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_id": "invalid uuid"}})
		return
	}

	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	item, found, err := h.signatures.GetByID(ctx, signatureID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"signature_id": "Signature not found"}})
		return
	}

	out := store.ItemToInterfaceMap(item)
	// Python Signature.to_dict() filters out raw DocuSign XML.
	delete(out, "user_docusign_raw_xml")

	respond.JSON(w, http.StatusOK, out)
}

// POST /v1/signature
// Python: cla/routes.py:825 post_signature()
// Calls: cla.controllers.signature.create_signature

func (h *Handlers) PostSignatureV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, authResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authResp)
		return
	}

	// In Python (hug), these values can come from JSON, form-encoded, or query params.
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": err.Error()}})
		return
	}

	getString := func(key string) string {
		if v, ok := flexibleStringParam(r, body, key); ok {
			return v
		}
		return ""
	}

	getBool := func(key string) (bool, bool, error) {
		return flexibleBoolParam(r, body, key)
	}

	projectIDStr := getString("signature_project_id")
	if projectIDStr == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_project_id": "missing"}})
		return
	}
	if _, err := uuid.Parse(projectIDStr); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_project_id": "invalid uuid"}})
		return
	}

	refID := getString("signature_reference_id")
	if refID == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_reference_id": "missing"}})
		return
	}

	refType := getString("signature_reference_type")
	if refType == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_reference_type": "missing"}})
		return
	}
	if refType != "user" && refType != "company" {
		// Legacy Hug route uses one_of(["company", "user"]) and rejects invalid values with HTTP 400
		// before controller logic runs.
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_reference_type": "invalid"}})
		return
	}

	sigType := getString("signature_type")
	if sigType == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_type": "missing"}})
		return
	}
	if sigType != "cla" && sigType != "dco" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_type": "invalid"}})
		return
	}

	signed, ok, err := getBool("signature_signed")
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_signed": err.Error()}})
		return
	}
	if !ok {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_signed": "missing"}})
		return
	}

	approved, ok, err := getBool("signature_approved")
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_approved": err.Error()}})
		return
	}
	if !ok {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_approved": "missing"}})
		return
	}

	embargoAcked, ok, err := getBool("signature_embargo_acked")
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_embargo_acked": err.Error()}})
		return
	}
	if !ok {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_embargo_acked": "missing"}})
		return
	}

	returnURL := getString("signature_return_url")
	if returnURL == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_return_url": "missing"}})
		return
	}
	if _, err := validateURL(returnURL); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_return_url": "invalid"}})
		return
	}

	signURL := getString("signature_sign_url")
	if signURL == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_sign_url": "missing"}})
		return
	}
	if _, err := validateURL(signURL); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_sign_url": "invalid"}})
		return
	}

	userCclaCompanyID := getString("signature_user_ccla_company_id")

	// Load project (CLA group)
	projectItem, found, err := h.projects.GetByID(ctx, projectIDStr)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"signature_project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"signature_project_id": "project not found"}})
		return
	}

	// Validate reference and resolve document version
	if refType == "user" {
		_, foundU, err := h.users.GetByID(ctx, refID)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"signature_reference_id": err.Error()}})
			return
		}
		if !foundU {
			respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"signature_reference_id": "user not found"}})
			return
		}
	} else {
		_, foundC, err := h.companies.GetByID(ctx, refID)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"signature_reference_id": err.Error()}})
			return
		}
		if !foundC {
			respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"signature_reference_id": "company not found"}})
			return
		}
	}

	var maj, min int
	var docErr error
	if refType == "user" {
		maj, min, docErr = h.projects.LatestIndividualDocumentVersion(ctx, projectIDStr)
	} else {
		maj, min, docErr = h.projects.LatestCorporateDocumentVersion(ctx, projectIDStr)
	}
	if docErr != nil {
		// Python returns {'errors': {'signature_project_id': 'Document not found'}}
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"signature_project_id": "Document not found"}})
		return
	}

	sigID := uuid.NewString()
	now := time.Now().UTC()

	sigItem := map[string]types.AttributeValue{
		"signature_id":                     &types.AttributeValueMemberS{Value: sigID},
		"signature_project_id":             &types.AttributeValueMemberS{Value: projectIDStr},
		"signature_reference_id":           &types.AttributeValueMemberS{Value: refID},
		"signature_reference_type":         &types.AttributeValueMemberS{Value: refType},
		"signature_type":                   &types.AttributeValueMemberS{Value: sigType},
		"signature_signed":                 &types.AttributeValueMemberBOOL{Value: signed},
		"signature_approved":               &types.AttributeValueMemberBOOL{Value: approved},
		"signature_embargo_acked":          &types.AttributeValueMemberBOOL{Value: embargoAcked},
		"signature_return_url":             &types.AttributeValueMemberS{Value: returnURL},
		"signature_sign_url":               &types.AttributeValueMemberS{Value: signURL},
		"signature_document_major_version": &types.AttributeValueMemberN{Value: strconv.Itoa(maj)},
		"signature_document_minor_version": &types.AttributeValueMemberN{Value: strconv.Itoa(min)},
		"date_created":                     &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"date_modified":                    &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"version":                          &types.AttributeValueMemberS{Value: "v1"},
	}
	if userCclaCompanyID != "" {
		sigItem["signature_user_ccla_company_id"] = &types.AttributeValueMemberS{Value: userCclaCompanyID}
	}

	if err := h.signatures.PutItem(ctx, sigItem); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"signature": err.Error()}})
		return
	}

	// Audit event (best-effort)
	projectName := getAttrString(projectItem, "project_name")
	eventData := fmt.Sprintf("Signature added. Signature_id - %s for Project - %s", sigID, projectName)
	// Python sets event_summary == event_data (cla.controllers.signature.create_signature).
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "CreateSignature",
		EventCLAGroupID: projectIDStr,
		EventData:       eventData,
		EventSummary:    eventData,
		ContainsPII:     false,
	})

	// Match Python's Signature.to_dict() — return the fields we just wrote so
	// consumers (legacy console, contributor flow) get the full signature back,
	// not just the id.
	resp := map[string]any{
		"signature_id":             sigID,
		"signature_project_id":     projectIDStr,
		"signature_reference_id":   refID,
		"signature_reference_type": refType,
		"signature_type":           sigType,
		"signature_signed":         signed,
		"signature_approved":       approved,
		"signature_embargo_acked":  embargoAcked,
		"signature_return_url":     returnURL,
		"signature_sign_url":       signURL,
		// Match the rest of the codebase: store.ToInterface (dynamo_conv.go:14)
		// returns DynamoDB N as strings for pynamodb parity. GET /v1/signature
		// reads through that path and emits strings here too — so POST must do
		// the same to avoid clients seeing different types on the same field.
		"signature_document_major_version": strconv.Itoa(maj),
		"signature_document_minor_version": strconv.Itoa(min),
		"date_created":                     formatPynamoDateTimeUTC(now),
		"date_modified":                    formatPynamoDateTimeUTC(now),
		"version":                          "v1",
	}
	if userCclaCompanyID != "" {
		resp["signature_user_ccla_company_id"] = userCclaCompanyID
	}
	respond.JSON(w, http.StatusOK, resp)
}

// PUT /v1/signature
// Python: cla/routes.py:878 put_signature()
// Calls: cla.controllers.signature.update_signature

func (h *Handlers) PutSignatureV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	// Legacy hug handler accepts JSON, form-encoded, or query params.
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"request": err.Error()}})
		return
	}

	getParam := func(key string) (any, bool) {
		if v, ok := body[key]; ok {
			return v, true
		}
		_ = r.ParseForm()
		if vals, ok := r.PostForm[key]; ok {
			if len(vals) == 1 {
				return vals[0], true
			}
			out := make([]any, 0, len(vals))
			for _, v := range vals {
				out = append(out, v)
			}
			return out, true
		}
		if vals, ok := r.URL.Query()[key]; ok {
			if len(vals) == 1 {
				return vals[0], true
			}
			out := make([]any, 0, len(vals))
			for _, v := range vals {
				out = append(out, v)
			}
			return out, true
		}
		return nil, false
	}
	getString := func(key string) (string, bool) {
		if v, ok := flexibleStringParam(r, body, key); ok {
			return v, true
		}
		return "", false
	}

	signatureID, ok := getString("signature_id")
	if !ok || signatureID == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_id": "missing"}})
		return
	}
	if _, err := uuid.Parse(signatureID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_id": "invalid uuid"}})
		return
	}

	sig, found, err := h.signatures.GetByID(ctx, signatureID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"signature_id": "Signature not found"}})
		return
	}

	updateStr := "Updated Signature fields: "
	changed := false
	// EASYCLA_PARITY_FLAG: legacy Python PUT /v1/signature does not expose
	// signature_document_major_version/signature_document_minor_version; default ignores them.
	if majRaw, ok := getParam("signature_document_major_version"); ok && parity.EnablePutSignatureDocumentVersionUpdates {
		maj, err := wholeNumberString(majRaw)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_document_major_version": err.Error()}})
			return
		}
		if maj != "" {
			sig["signature_document_major_version"] = &types.AttributeValueMemberN{Value: maj}
			updateStr += fmt.Sprintf("Signature document version changed to %s.%s. ", maj, getAttrString(sig, "signature_document_minor_version"))
			changed = true
		}
	}
	if minRaw, ok := getParam("signature_document_minor_version"); ok && parity.EnablePutSignatureDocumentVersionUpdates {
		min, err := wholeNumberString(minRaw)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_document_minor_version": err.Error()}})
			return
		}
		if min != "" {
			sig["signature_document_minor_version"] = &types.AttributeValueMemberN{Value: min}
			updateStr += fmt.Sprintf("Signature document version changed to %s.%s. ", getAttrString(sig, "signature_document_major_version"), min)
			changed = true
		}
	}

	if v, ok := getString("signature_reference_id"); ok {
		sig["signature_reference_id"] = &types.AttributeValueMemberS{Value: v}
		updateStr += fmt.Sprintf("Signature reference ID changed to %s. ", v)
		changed = true
	}
	if v, ok := getString("signature_reference_type"); ok {
		sig["signature_reference_type"] = &types.AttributeValueMemberS{Value: v}
		updateStr += fmt.Sprintf("Signature reference type changed to %s. ", v)
		changed = true
	}

	if v, ok := getString("signature_project_id"); ok {
		if _, err := uuid.Parse(v); err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_project_id": "invalid uuid"}})
			return
		}
		sig["signature_project_id"] = &types.AttributeValueMemberS{Value: v}
		updateStr += fmt.Sprintf("Signature project ID changed to %s. ", v)
		changed = true
	}

	if v, ok := getString("signature_type"); ok {
		if v != "cla" && v != "dco" {
			respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"signature_type": "Signature type invalid"}})
			return
		}
		sig["signature_type"] = &types.AttributeValueMemberS{Value: v}
		updateStr += fmt.Sprintf("Signature type changed to %s. ", v)
		changed = true
	}

	if raw, ok := getParam("signature_signed"); ok {
		b, err := smartBool(raw)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_signed": err.Error()}})
			return
		}
		sig["signature_signed"] = &types.AttributeValueMemberBOOL{Value: b}
		updateStr += fmt.Sprintf("Signature signed changed to %v. ", b)
		changed = true
	}
	if raw, ok := getParam("signature_approved"); ok {
		b, err := smartBool(raw)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_approved": err.Error()}})
			return
		}
		sig["signature_approved"] = &types.AttributeValueMemberBOOL{Value: b}
		updateStr += fmt.Sprintf("Signature approved changed to %v. ", b)
		changed = true
	}
	if raw, ok := getParam("signature_embargo_acked"); ok {
		b, err := smartBool(raw)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_embargo_acked": err.Error()}})
			return
		}
		sig["signature_embargo_acked"] = &types.AttributeValueMemberBOOL{Value: b}
		updateStr += fmt.Sprintf("Signature embargo acked changed to %v. ", b)
		changed = true
	}

	if v, ok := getString("signature_return_url"); ok {
		if _, err := validateURL(v); err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_return_url": err.Error()}})
			return
		}
		sig["signature_return_url"] = &types.AttributeValueMemberS{Value: v}
		updateStr += fmt.Sprintf("Signature return URL changed to %s. ", v)
		changed = true
	}
	if v, ok := getString("signature_sign_url"); ok {
		if _, err := validateURL(v); err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_sign_url": err.Error()}})
			return
		}
		sig["signature_sign_url"] = &types.AttributeValueMemberS{Value: v}
		updateStr += fmt.Sprintf("Signature sign URL changed to %s. ", v)
		changed = true
	}
	// EASYCLA_PARITY_FLAG: legacy Python PUT /v1/signature does not expose
	// signature_user_ccla_company_id; default ignores it.
	if v, ok := getString("signature_user_ccla_company_id"); ok && parity.EnablePutSignatureAdditionalFieldUpdates {
		if v == "" {
			delete(sig, "signature_user_ccla_company_id")
		} else {
			sig["signature_user_ccla_company_id"] = &types.AttributeValueMemberS{Value: v}
		}
		updateStr += fmt.Sprintf("Signature user CCLA company ID changed to %s. ", v)
		changed = true
	}
	// EASYCLA_PARITY_FLAG: legacy Python PUT /v1/signature only exposes the legacy *_whitelist
	// API parameter names; default ignores newer *_allowlist aliases.
	// Allowlist fields: accept both legacy *_whitelist param names and newer *_allowlist.
	// IMPORTANT: DynamoDB schema for the legacy Python service uses the *_whitelist attribute names.
	// The UI models (console) also expect *_whitelist fields.
	// Prior iterations accidentally wrote *_allowlist fields which would not be visible to legacy clients.
	var githubAllowlistProvided bool
	var githubAllowlistValues []string
	allowlistParamNames := [][2]string{
		{"domain_whitelist", "domain_allowlist"},
		{"email_whitelist", "email_allowlist"},
		{"github_whitelist", "github_allowlist"},
		{"github_org_whitelist", "github_org_allowlist"},
	}
	for _, pair := range allowlistParamNames {
		legacyKey := pair[0]
		altKey := pair[1]
		raw, ok := getParam(legacyKey)
		if !ok && parity.EnablePutSignatureAllowlistAliasParams {
			// try allowlist key
			raw, ok = getParam(altKey)
		}
		if !ok {
			continue
		}
		lst, err := stringListFromAny(raw)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{legacyKey: err.Error()}})
			return
		}
		lst = normalizeAllowlist(lst)

		// Remove both keys to avoid stale data.
		delete(sig, legacyKey)
		delete(sig, altKey)

		if len(lst) > 0 {
			// Python stores these allowlists as ListAttribute (not StringSet). Store as DynamoDB list for parity.
			vals := make([]types.AttributeValue, 0, len(lst))
			for _, s := range lst {
				vals = append(vals, &types.AttributeValueMemberS{Value: s})
			}
			sig[legacyKey] = &types.AttributeValueMemberL{Value: vals}
		}

		if legacyKey == "github_whitelist" {
			githubAllowlistProvided = true
			// Preserve the normalized list for bot detection (legacy Python checks bots only when github_allowlist is provided).
			githubAllowlistValues = append([]string(nil), lst...)
		}

		switch legacyKey {
		case "domain_whitelist":
			updateStr += fmt.Sprintf("Signature domain whitelist changed to %v. ", lst)
		case "email_whitelist":
			updateStr += fmt.Sprintf("Signature email whitelist changed to %v. ", lst)
		case "github_whitelist":
			updateStr += fmt.Sprintf("Signature github whitelist changed to %v. ", lst)
		case "github_org_whitelist":
			updateStr += fmt.Sprintf("Signature github org whitelist changed to %v. ", lst)
		}
		changed = true
	}

	// Python creates bot users/signatures when github_allowlist includes bots.
	// This is invoked only when the github allowlist parameter is provided.
	if githubAllowlistProvided {
		h.handleGithubBotsFromAllowlistBestEffort(ctx, sig, githubAllowlistValues)
	}

	// Save signature only if something changed.
	if changed {
		sig["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}
		if err := h.signatures.PutItem(ctx, sig); err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
			return
		}

		// Audit event parity
		userID, _ := getString("user_id")
		companyID := getAttrString(sig, "signature_user_ccla_company_id")
		if companyID == "" {
			companyID = getAttrString(sig, "signature_reference_id")
		}
		claGroupID := getAttrString(sig, "signature_project_id")
		h.putAuditEventBestEffort(ctx, auditEventInput{
			EventType:       "UpdateSignature",
			EventCLAGroupID: claGroupID,
			EventCompanyID:  companyID,
			EventUserID:     userID,
			EventData:       updateStr,
			EventSummary:    updateStr,
			ContainsPII:     true,
		})
	}

	out := store.ItemToInterfaceMap(sig)
	delete(out, "user_docusign_raw_xml")
	respond.JSON(w, http.StatusOK, out)
}

// DELETE /v1/signature/{signature_id}
// Python: cla/routes.py:925 delete_signature()
// Calls: cla.controllers.signature.delete_signature

func (h *Handlers) DeleteSignatureV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	signatureID := chi.URLParam(r, "signature_id")
	if _, err := uuid.Parse(signatureID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_id": "invalid uuid"}})
		return
	}

	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	item, found, err := h.signatures.GetByID(ctx, signatureID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"signature_id": "Signature not found"}})
		return
	}

	claGroupID := getAttrString(item, "signature_project_id")

	if err := h.signatures.DeleteByID(ctx, signatureID); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	eventData := fmt.Sprintf("Deleted signature %s", signatureID)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "DeleteSignature",
		EventCLAGroupID: claGroupID,
		EventData:       eventData,
		EventSummary:    eventData,
		ContainsPII:     false,
	})

	respond.JSON(w, http.StatusOK, map[string]any{"success": true})
}

// GET /v1/signatures/user/{user_id}
// Python: cla/routes.py:936 get_signatures_user()
// Calls: cla.controllers.signature.get_user_signatures

func (h *Handlers) GetSignaturesUserV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := chi.URLParam(r, "user_id")
	if _, err := uuid.Parse(userID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}

	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	items, err := h.signatures.QueryByReferenceID(ctx, userID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if getAttrString(it, "signature_reference_type") != "user" {
			continue
		}
		m := store.ItemToInterfaceMap(it)
		delete(m, "user_docusign_raw_xml")
		out = append(out, m)
	}
	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/signatures/user/{user_id}/project/{project_id}
// Python: cla/routes.py:946 get_signatures_user_project()
// Calls: cla.controllers.signature.get_user_project_signatures

func (h *Handlers) GetSignaturesUserProjectV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := chi.URLParam(r, "user_id")
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(userID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	sigItems, err := h.signatures.QueryByProjectID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	out := make([]map[string]any, 0, 8)
	for _, it := range sigItems {
		if getAttrString(it, "signature_reference_type") != "user" {
			continue
		}
		if getAttrString(it, "signature_reference_id") != userID {
			continue
		}
		m := store.ItemToInterfaceMap(it)
		delete(m, "user_docusign_raw_xml")
		out = append(out, m)
	}

	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/signatures/user/{user_id}/project/{project_id}/type/{signature_type}
// Python: cla/routes.py:956 get_signatures_user_project()
// Calls: cla.controllers.signature.get_user_project_signatures

func (h *Handlers) GetSignaturesUserProjectTypeV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := chi.URLParam(r, "user_id")
	projectID := chi.URLParam(r, "project_id")
	signatureType := chi.URLParam(r, "signature_type")
	if _, err := uuid.Parse(userID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}
	if signatureType != "individual" && signatureType != "employee" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_type": "Invalid value passed. The accepted values are: (individual|employee)"}})
		return
	}
	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	sigItems, err := h.signatures.QueryByProjectID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	matching := make([]map[string]types.AttributeValue, 0, 8)
	for _, it := range sigItems {
		if getAttrString(it, "signature_reference_type") != "user" {
			continue
		}
		if getAttrString(it, "signature_reference_id") != userID {
			continue
		}
		matching = append(matching, it)
	}

	// EASYCLA_PARITY_FLAG: default preserves the legacy Python AttributeError in
	// controllers.signature.get_user_project_signatures(); the crash only occurs when the
	// project/user query returned at least one signature to iterate.
	if !parity.FixGetSignaturesUserProjectTypeV1 {
		if len(matching) == 0 {
			respond.JSON(w, http.StatusOK, []map[string]any{})
			return
		}
		respond.JSON(w, http.StatusInternalServerError, map[string]any{
			"errors": map[string]any{
				"server": "AttributeError: 'Signature' object has no attribute 'get_signature_user_ccla_employee_id'",
			},
		})
		return
	}

	out := make([]map[string]any, 0, len(matching))
	for _, it := range matching {
		sigCompanyID := strings.TrimSpace(getAttrString(it, "signature_user_ccla_company_id"))
		if signatureType == "individual" && sigCompanyID != "" {
			continue
		}
		if signatureType == "employee" && sigCompanyID == "" {
			continue
		}
		m := store.ItemToInterfaceMap(it)
		delete(m, "user_docusign_raw_xml")
		out = append(out, m)
	}

	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/signatures/company/{company_id}
// Python: cla/routes.py:971 get_signatures_company()
// Calls: cla.controllers.signature.get_company_signatures_by_acl

func (h *Handlers) GetSignaturesCompanyV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	companyID := chi.URLParam(r, "company_id")
	if _, err := uuid.Parse(companyID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"company_id": "invalid uuid"}})
		return
	}

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	items, err := h.signatures.QueryByReferenceID(ctx, companyID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if getAttrString(it, "signature_reference_type") != "company" {
			continue
		}
		sigACL := getAttrStringSlice(it, "signature_acl")
		if !stringSliceContainsExact(sigACL, authUser.Username) {
			continue
		}
		m := store.ItemToInterfaceMap(it)
		delete(m, "user_docusign_raw_xml")
		out = append(out, m)
	}
	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/signatures/project/{project_id}
// Python: cla/routes.py:981 get_signatures_project()
// Calls: cla.controllers.signature.get_project_signatures

func (h *Handlers) GetSignaturesProjectV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	sigItems, err := h.signatures.QueryByProjectID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	out := make([]map[string]any, 0, 16)
	for _, it := range sigItems {
		if !getAttrBool(it, "signature_signed") {
			continue
		}
		m := store.ItemToInterfaceMap(it)
		delete(m, "user_docusign_raw_xml")
		out = append(out, m)
	}

	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/signatures/company/{company_id}/project/{project_id}
// Python: cla/routes.py:991 get_signatures_project_company()
// Calls: cla.controllers.signature.get_project_company_signatures

func (h *Handlers) GetSignaturesProjectCompanyV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	companyID := chi.URLParam(r, "company_id")
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(companyID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"company_id": "invalid uuid"}})
		return
	}
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	sigItems, err := h.signatures.QueryByReferenceID(ctx, companyID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	out := make([]map[string]any, 0, 8)
	for _, it := range sigItems {
		if getAttrString(it, "signature_project_id") != projectID {
			continue
		}
		m := store.ItemToInterfaceMap(it)
		delete(m, "user_docusign_raw_xml")
		out = append(out, m)
	}
	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/signatures/company/{company_id}/project/{project_id}/employee
// Python: cla/routes.py:1001 get_project_employee_signatures()
// Calls: cla.controllers.signature.get_project_employee_signatures

func (h *Handlers) GetProjectEmployeeSignaturesV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	companyID := chi.URLParam(r, "company_id")
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(companyID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"company_id": "invalid uuid"}})
		return
	}
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	// Legacy Python endpoint does NOT require authentication.
	items, err := h.signatures.QueryByProjectID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if getAttrString(it, "signature_user_ccla_company_id") != companyID {
			continue
		}
		m := store.ItemToInterfaceMap(it)
		delete(m, "user_docusign_raw_xml")
		out = append(out, m)
	}
	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/signature/{signature_id}/manager
// Python: cla/routes.py:1011 get_cla_managers()
// Calls: cla.controllers.signature.get_cla_managers

// buildClaManagersResponse matches cla.controllers.signature.get_managers_dict().
func (h *Handlers) buildClaManagersResponse(ctx context.Context, signatureACL []string) ([]map[string]any, error) {
	managers := make([]map[string]any, 0, len(signatureACL))
	for _, lfid := range signatureACL {
		lfid = strings.TrimSpace(lfid)
		if lfid == "" {
			continue
		}

		mgr := map[string]any{"lfid": lfid}

		users, err := h.users.QueryByLFUsername(ctx, lfid)
		if err != nil {
			// PynamoDB would raise here; keep strict parity and bubble up.
			return nil, err
		}
		if len(users) == 0 {
			managers = append(managers, mgr)
			continue
		}
		if len(users) > 1 {
			logging.Warnf("multiple users found for lfid=%s count=%d", lfid, len(users))
		}
		u := users[0]
		email := strings.TrimSpace(getUserEmailLikePython(u))
		resolvedLFID := strings.TrimSpace(getAttrString(u, "lf_username"))
		if resolvedLFID == "" {
			resolvedLFID = lfid
		}
		mgr = map[string]any{
			"name":            getAttrString(u, "user_name"),
			"email":           email,
			"alt_emails":      getAttrStringSlice(u, "user_emails"),
			"github_user_id":  getAttrString(u, "user_github_id"),
			"github_username": getAttrString(u, "user_github_username"),
			"lfid":            resolvedLFID,
		}
		managers = append(managers, mgr)
	}
	return managers, nil
}

func (h *Handlers) sendCLAManagerEmailBestEffort(ctx context.Context, action string, lfid string, companyID string, claGroupID string, managerLFIDs []string) {
	if h == nil || h.users == nil {
		return
	}
	lfid = strings.TrimSpace(lfid)
	if lfid == "" {
		return
	}

	var user map[string]types.AttributeValue
	if users, err := h.users.QueryByLFUsername(ctx, lfid); err == nil && len(users) > 0 {
		user = users[0]
	} else if item, found, _ := h.users.GetByID(ctx, lfid); found {
		user = item
	}
	if user == nil {
		logging.Warnf("cla-manager email: user not found (lfid=%s)", lfid)
		return
	}

	primaryEmail := strings.TrimSpace(getUserEmailLikePython(user))
	if primaryEmail == "" {
		logging.Warnf("cla-manager email: no primary recipient email found (lfid=%s)", lfid)
		return
	}

	companyName := strings.TrimSpace(companyID)
	if h.companies != nil && companyID != "" {
		if comp, found, err := h.companies.GetByID(ctx, companyID); err == nil && found {
			if v := strings.TrimSpace(getAttrString(comp, "company_name")); v != "" {
				companyName = v
			}
		}
	}
	projectName := strings.TrimSpace(claGroupID)
	projectVersion := ""
	if h.projects != nil && claGroupID != "" {
		if proj, found, err := h.projects.GetByID(ctx, claGroupID); err == nil && found {
			if v := strings.TrimSpace(getAttrString(proj, "project_name")); v != "" {
				projectName = v
			}
			projectVersion = getAttrString(proj, "version")
		}
	}

	managers := make([]map[string]any, 0)
	if len(managerLFIDs) > 0 {
		if resp, err := h.buildClaManagersResponse(ctx, managerLFIDs); err == nil {
			managers = resp
		}
	}
	managerList := make([]string, 0, len(managers))
	for _, mgr := range managers {
		managerList = append(managerList, fmt.Sprintf("%s <%s>", mgr["name"], mgr["email"]))
	}
	managerListStr := strings.Join(managerList, "-") + "\n"

	subject := fmt.Sprintf("CLA: Access to Corporate CLA for Project %s", projectName)
	action = strings.ToLower(strings.TrimSpace(action))
	var body string
	if action == "removed" {
		body = fmt.Sprintf(`
    <p> Hello %s, </p> \
    <p>This is a notification email from EasyCLA regarding the project %s.</p> \
    <p>You have been removed as a CLA Manager from the project: %s for the organization \
       %s </p> \
    <p> If you have further questions, please contact one of the existing CLA Managers: </p> \
    %s
    `, lfid, projectName, projectName, companyName, managerListStr)
	} else {
		body = fmt.Sprintf(`
    <p>Hello %s, </p> \
    <p>This is a notification email from EasyCLA regarding the project %s.</p> \
    <p>You have been granted access to the project %s for the organization \
       %s.</p> \
    <p> If you have further questions, please contact one of the existing CLA Managers: </p> \
    %s
    `, lfid, projectName, projectName, companyName, managerListStr)
	}
	body = "<p>" + strings.ReplaceAll(body, "\n", "<br>") + "</p>"
	body = appendEmailHelpSignOffContent(body, projectVersion)

	svc, err := email.NewFromEnv(ctx)
	if err != nil {
		logging.Warnf("cla-manager email service init failed: %v", err)
		return
	}
	if err := svc.Send(ctx, subject, body, []string{primaryEmail}); err != nil {
		logging.Warnf("failed to send cla-manager %s email: %v", action, err)
	}
}

func (h *Handlers) GetClaManagersV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	signatureID := chi.URLParam(r, "signature_id")
	if _, err := uuid.Parse(signatureID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_id": "invalid uuid"}})
		return
	}

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	sigItem, found, err := h.signatures.GetByID(ctx, signatureID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"signature_id": "Signature not found"}})
		return
	}

	signatureACL := getAttrStringSlice(sigItem, "signature_acl")
	if !stringSliceContainsExact(signatureACL, authUser.Username) {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user_id": "You are not authorized to see the managers."}})
		return
	}

	resp, err := h.buildClaManagersResponse(ctx, signatureACL)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

// POST /v1/signature/{signature_id}/manager
// Python: cla/routes.py:1021 add_cla_manager()
// Calls: cla.controllers.signature.add_cla_manager

func (h *Handlers) AddClaManagerV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	signatureID := chi.URLParam(r, "signature_id")
	if signatureID == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_id": "missing"}})
		return
	}
	if _, err := uuid.Parse(signatureID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_id": "invalid uuid"}})
		return
	}

	// LFID can be provided as JSON, form-encoded, or query params.
	body, _ := parseFlexibleParams(r)
	lfid, _ := flexibleStringParam(r, body, "lfid")
	if lfid == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"lfid": "missing"}})
		return
	}

	// Load signature
	sig, found, err := h.signatures.GetByID(ctx, signatureID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		// EASYCLA_PARITY_FLAG: default preserves the legacy Python project_id error key bug in add_cla_manager();
		// set EASYCLA_FIX_ADD_CLA_MANAGER_V1_NOT_FOUND_ERROR_KEY=true to return signature_id instead.
		errKey := "project_id"
		if parity.FixAddClaManagerV1NotFoundErrorKey {
			errKey = "signature_id"
		}
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{errKey: "Signature not found"}})
		return
	}

	sigACL := getAttrStringSlice(sig, "signature_acl")
	if !stringSliceContainsExact(sigACL, authUser.Username) {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user_id": "You are not authorized to see the managers."}})
		return
	}

	// Python parity: cla.controllers.signature.add_cla_manager() does NOT skip
	// when lfid is already in the ACL — it always proceeds, sends email, and
	// creates an audit event. We must not short-circuit here.

	companyID := getAttrString(sig, "signature_reference_id")
	claGroupID := getAttrString(sig, "signature_project_id")

	// Best-effort add company permission (company ACL) - Python ignores return value.
	if companyID != "" {
		if comp, compFound, compErr := h.companies.GetByID(ctx, companyID); compErr == nil && compFound {
			acl := getAttrStringSlice(comp, "company_acl")
			if !stringSliceContainsExact(acl, lfid) {
				acl = append(acl, lfid)
				comp["company_acl"] = &types.AttributeValueMemberSS{Value: uniqueStringsPreserveOrder(acl)}
				comp["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}
				_ = h.companies.PutItem(ctx, comp)

				// Audit event matches cla.controllers.company.add_permission() behavior.
				h.putAuditEventBestEffort(ctx, auditEventInput{
					EventType:      "AddCompanyPermission",
					EventCompanyID: companyID,
					EventUserID:    lfid,
					EventData:      fmt.Sprintf("Added to user %s to Company %s permissions list.", lfid, getAttrString(comp, "company_name")),
					EventSummary:   fmt.Sprintf("Added to user %s to Company %s permissions list.", lfid, getAttrString(comp, "company_name")),
					ContainsPII:    true,
				})
			}
		}
	}

	// Update signature ACL. Python's signature_acl is a set, so re-adding an
	// existing lfid is a no-op there. In Go we append unconditionally (Python
	// parity does not skip on already-present), so dedupe before reusing the
	// slice for the email recipient list and the response — otherwise a stale
	// re-add would render the manager twice.
	sigACL = uniqueStringsPreserveOrder(append(sigACL, lfid))
	sig["signature_acl"] = &types.AttributeValueMemberSS{Value: sigACL}
	sig["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}
	if err := h.signatures.PutItem(ctx, sig); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	// Audit event matches cla.controllers.signature.add_cla_manager().
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "AddCLAManager",
		EventCompanyID:  companyID,
		EventCLAGroupID: claGroupID,
		EventUserID:     lfid,
		EventData:       fmt.Sprintf("%s added as cla manager to Signature ACL for %s", lfid, signatureID),
		EventSummary:    fmt.Sprintf("%s added as cla manager to Signature ACL for %s", lfid, signatureID),
		ContainsPII:     true,
	})

	// Email notifications exist in legacy Python (SES/SNS/etc).
	h.sendCLAManagerEmailBestEffort(ctx, "added", lfid, companyID, claGroupID, sigACL)

	resp, err := h.buildClaManagersResponse(ctx, sigACL)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

// DELETE /v1/signature/{signature_id}/manager/{lfid}
// Python: cla/routes.py:1031 remove_cla_manager()
// Calls: cla.controllers.signature.remove_cla_manager

func (h *Handlers) RemoveClaManagerV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	signatureID := chi.URLParam(r, "signature_id")
	lfid := chi.URLParam(r, "lfid")
	if signatureID == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_id": "missing"}})
		return
	}
	if _, err := uuid.Parse(signatureID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_id": "invalid uuid"}})
		return
	}
	if lfid == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"lfid": "missing"}})
		return
	}

	sig, found, err := h.signatures.GetByID(ctx, signatureID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"signature_id": "Signature not found"}})
		return
	}

	sigACL := getAttrStringSlice(sig, "signature_acl")
	if !stringSliceContainsExact(sigACL, authUser.Username) {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user": "You are not authorized to manage this CCLA."}})
		return
	}

	// If lfid not present, return managers list unchanged.
	if !stringSliceContainsExact(sigACL, lfid) {
		resp, err := h.buildClaManagersResponse(ctx, sigACL)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
			return
		}
		respond.JSON(w, http.StatusOK, resp)
		return
	}

	if len(sigACL) == 1 && authUser.Username == lfid {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user": "You cannot remove this manager because a CCLA must have at least one CLA manager."}})
		return
	}

	// Remove lfid
	newACL := make([]string, 0, len(sigACL)-1)
	for _, u := range sigACL {
		if u == lfid {
			continue
		}
		newACL = append(newACL, u)
	}
	newACL = uniqueStringsPreserveOrder(newACL)

	sig["signature_acl"] = &types.AttributeValueMemberSS{Value: newACL}
	sig["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}
	if err := h.signatures.PutItem(ctx, sig); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	companyID := getAttrString(sig, "signature_reference_id")
	claGroupID := getAttrString(sig, "signature_project_id")

	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "RemoveCLAManager",
		EventCompanyID:  companyID,
		EventCLAGroupID: claGroupID,
		EventUserID:     lfid,
		EventData:       fmt.Sprintf("User with lfid %s removed from project ACL with signature %s", lfid, signatureID),
		EventSummary:    fmt.Sprintf("User with lfid %s removed from project ACL with signature %s", lfid, signatureID),
		ContainsPII:     true,
	})

	// Email notifications exist in legacy Python.
	h.sendCLAManagerEmailBestEffort(ctx, "removed", lfid, companyID, claGroupID, newACL)

	resp, err := h.buildClaManagersResponse(ctx, newACL)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

// GET /v1/repository/{repository_id}
// Python: cla/routes.py:1041 get_repository()
// Calls: cla.controllers.repository.get_repository

func (h *Handlers) GetRepositoryV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	repositoryID := chi.URLParam(r, "repository_id")
	item, found, err := h.repos.GetByID(ctx, repositoryID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"repository_id": "Repository not found"}})
		return
	}

	respond.JSON(w, http.StatusOK, store.ItemToInterfaceMap(item))
}

// POST /v1/repository
// Python: cla/routes.py:1060 post_repository()
// Calls: cla.controllers.repository.create_repository

func (h *Handlers) PostRepositoryV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	type request struct {
		RepositoryProjectID        string  `json:"repository_project_id"`
		RepositoryName             string  `json:"repository_name"`
		RepositoryOrganizationName string  `json:"repository_organization_name"`
		RepositoryType             string  `json:"repository_type"`
		RepositoryURL              string  `json:"repository_url"`
		RepositoryExternalID       *string `json:"repository_external_id"`
	}
	var req request
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}
	if v, ok := flexibleStringParam(r, body, "repository_project_id"); ok {
		req.RepositoryProjectID = v
	}
	if v, ok := flexibleStringParam(r, body, "repository_name"); ok {
		req.RepositoryName = v
	}
	if v, ok := flexibleStringParam(r, body, "repository_organization_name"); ok {
		req.RepositoryOrganizationName = v
	}
	if v, ok := flexibleStringParam(r, body, "repository_type"); ok {
		req.RepositoryType = v
	}
	if v, ok := flexibleStringParam(r, body, "repository_url"); ok {
		req.RepositoryURL = v
	}
	if v, ok := flexibleStringParam(r, body, "repository_external_id"); ok && strings.TrimSpace(v) != "" {
		req.RepositoryExternalID = &v
	}
	missing := map[string]any{}
	if strings.TrimSpace(req.RepositoryProjectID) == "" {
		missing["repository_project_id"] = "missing"
	}
	if strings.TrimSpace(req.RepositoryName) == "" {
		missing["repository_name"] = "missing"
	}
	if strings.TrimSpace(req.RepositoryOrganizationName) == "" {
		missing["repository_organization_name"] = "missing"
	}
	if strings.TrimSpace(req.RepositoryType) == "" {
		missing["repository_type"] = "missing"
	}
	if strings.TrimSpace(req.RepositoryURL) == "" {
		missing["repository_url"] = "missing"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}
	if _, err := uuid.Parse(strings.TrimSpace(req.RepositoryProjectID)); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"repository_project_id": "invalid uuid"}})
		return
	}

	// Legacy Hug route uses one_of(get_supported_repository_providers().keys()) and cla.hug_types.url.
	// Invalid values are rejected with HTTP 400 before controller logic runs.
	if req.RepositoryType != "github" && req.RepositoryType != "mock_github" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"repository_type": "Invalid value passed. The accepted values are: (github|mock_github)"}})
		return
	}
	if _, err := validateURL(req.RepositoryURL); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"repository_url": "Invalid URL specified"}})
		return
	}

	// Validate GitHub organization exists.
	_, orgFound, err := h.githubOrgs.GetByName(ctx, req.RepositoryOrganizationName)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !orgFound {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"organization_name": "GitHub Org not found"}})
		return
	}

	projectItem, projectFound, err := h.projects.GetByID(ctx, req.RepositoryProjectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !projectFound {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"repository_project_id": "Project not found"}})
		return
	}
	projectSFID := getAttrString(projectItem, "project_external_id")

	// check_user_authorization() logic
	perms, err := h.userPerms.Get(ctx, authUser.Username)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	allowed := perms.Projects
	if !stringSliceContainsExact(allowed, projectSFID) {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user is not authorized for this Salesforce ID.": projectSFID}})
		return
	}

	if req.RepositoryExternalID != nil {
		if linked, found, err := h.repos.GetByExternalIDAndType(ctx, *req.RepositoryExternalID, req.RepositoryType); err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
			return
		} else if found {
			_ = linked
			respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"repository_external_id": "This repository is alredy configured for a contract group."}})
			return
		}
	}

	now := time.Now().UTC()
	repositoryID := uuid.New().String()
	item := map[string]types.AttributeValue{
		"repository_id":                &types.AttributeValueMemberS{Value: repositoryID},
		"repository_project_id":        &types.AttributeValueMemberS{Value: req.RepositoryProjectID},
		"repository_sfdc_id":           &types.AttributeValueMemberS{Value: projectSFID},
		"project_sfid":                 &types.AttributeValueMemberS{Value: projectSFID},
		"repository_name":              &types.AttributeValueMemberS{Value: req.RepositoryName},
		"repository_organization_name": &types.AttributeValueMemberS{Value: req.RepositoryOrganizationName},
		"repository_type":              &types.AttributeValueMemberS{Value: req.RepositoryType},
		"repository_url":               &types.AttributeValueMemberS{Value: req.RepositoryURL},
		"enabled":                      &types.AttributeValueMemberBOOL{Value: false},
		"date_created":                 &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"date_modified":                &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"version":                      &types.AttributeValueMemberS{Value: "v1"},
	}
	if req.RepositoryExternalID != nil {
		item["repository_external_id"] = &types.AttributeValueMemberS{Value: *req.RepositoryExternalID}
	}

	if err := h.repos.PutItem(ctx, item); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	respond.JSON(w, http.StatusOK, store.ItemToInterfaceMap(item))
}

// PUT /v1/repository
// Python: cla/routes.py:1101 put_repository()
// Calls: cla.controllers.repository.update_repository

func (h *Handlers) PutRepositoryV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	type request struct {
		RepositoryID         string  `json:"repository_id"`
		RepositoryProjectID  *string `json:"repository_project_id"`
		RepositoryName       *string `json:"repository_name"`
		RepositoryType       *string `json:"repository_type"`
		RepositoryURL        *string `json:"repository_url"`
		RepositoryExternalID *string `json:"repository_external_id"`
	}
	var req request
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}
	if v, ok := flexibleStringParam(r, body, "repository_id"); ok {
		req.RepositoryID = v
	}
	if v, ok := flexibleStringParam(r, body, "repository_project_id"); ok {
		req.RepositoryProjectID = &v
	}
	if v, ok := flexibleStringParam(r, body, "repository_name"); ok {
		req.RepositoryName = &v
	}
	if v, ok := flexibleStringParam(r, body, "repository_type"); ok {
		req.RepositoryType = &v
	}
	if v, ok := flexibleStringParam(r, body, "repository_url"); ok {
		req.RepositoryURL = &v
	}
	if v, ok := flexibleStringParam(r, body, "repository_external_id"); ok {
		req.RepositoryExternalID = &v
	}
	if strings.TrimSpace(req.RepositoryID) == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"repository_id": "Missing required value"}})
		return
	}

	item, found, err := h.repos.GetByID(ctx, req.RepositoryID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"repository_id": "Repository not found"}})
		return
	}

	if req.RepositoryProjectID != nil {
		item["repository_project_id"] = &types.AttributeValueMemberS{Value: *req.RepositoryProjectID}
	}
	if req.RepositoryName != nil {
		item["repository_name"] = &types.AttributeValueMemberS{Value: *req.RepositoryName}
	}
	if req.RepositoryType != nil {
		supported := []string{"github", "mock_github"}
		ok := false
		for _, v := range supported {
			if *req.RepositoryType == v {
				ok = true
				break
			}
		}
		if !ok {
			respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"repository_type": "Invalid value passed. The accepted values are: (github|mock_github)"}})
			return
		}
		item["repository_type"] = &types.AttributeValueMemberS{Value: *req.RepositoryType}
	}
	if req.RepositoryURL != nil {
		if _, err := validateURL(*req.RepositoryURL); err != nil {
			respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"repository_url": "Invalid URL specified"}})
			return
		}
		item["repository_url"] = &types.AttributeValueMemberS{Value: *req.RepositoryURL}
	}

	if req.RepositoryExternalID != nil {
		curType := getAttrString(item, "repository_type")
		if _, found, err := h.repos.GetByExternalIDAndType(ctx, *req.RepositoryExternalID, curType); err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
			return
		} else if found {
			respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"repository_external_id": "This repository is alredy configured for a contract group."}})
			return
		}
		item["repository_external_id"] = &types.AttributeValueMemberS{Value: *req.RepositoryExternalID}
	}

	item["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}

	// Legacy Python writes project_sfid on create; ensure it exists for compatibility.
	if _, ok := item["project_sfid"]; !ok {
		if sfidAttr, ok2 := item["repository_sfdc_id"].(*types.AttributeValueMemberS); ok2 && sfidAttr.Value != "" {
			item["project_sfid"] = &types.AttributeValueMemberS{Value: sfidAttr.Value}
		}
	}

	if err := h.repos.PutItem(ctx, item); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	respond.JSON(w, http.StatusOK, store.ItemToInterfaceMap(item))
}

// DELETE /v1/repository/{repository_id}
// Python: cla/routes.py:1129 delete_repository()
// Calls: cla.controllers.repository.delete_repository

func (h *Handlers) DeleteRepositoryV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	repositoryID := chi.URLParam(r, "repository_id")
	_, found, err := h.repos.GetByID(ctx, repositoryID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"repository_id": "Repository not found"}})
		return
	}

	if err := h.repos.DeleteByID(ctx, repositoryID); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"success": true})
}

// GET /v1/company
// Python: cla/routes.py:1143 get_companies()
// Calls: cla.controllers.company.get_companies_by_user, cla.controllers.user.get_or_create_user

func (h *Handlers) GetCompaniesV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	// Python always ensures the user exists (get_or_create_user).
	_, _, err = h.getOrCreateUser(ctx, authUser)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	items, err := h.companies.ScanAll(ctx)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	companies := make([]map[string]any, 0, len(items))
	for _, it := range items {
		acl := getAttrStringSlice(it, "company_acl")
		if stringSliceContainsExact(acl, authUser.Username) {
			companies = append(companies, store.ItemToInterfaceMap(it))
		}
	}

	// Sort by company_name.casefold().
	sort.Slice(companies, func(i, j int) bool {
		nameI, _ := companies[i]["company_name"].(string)
		nameJ, _ := companies[j]["company_name"].(string)
		return strings.ToLower(nameI) < strings.ToLower(nameJ)
	})

	respond.JSON(w, http.StatusOK, companies)
}

// GET /v2/company
// Python: cla/routes.py:1154 get_all_companies()
// Calls: cla.controllers.company.get_companies

func (h *Handlers) GetAllCompaniesV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	items, err := h.companies.ScanAll(ctx)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	companies := make([]map[string]any, 0, len(items))
	for _, it := range items {
		companies = append(companies, store.ItemToInterfaceMap(it))
	}

	// Python sorts by company_name.casefold(). We approximate with strings.ToLower.
	sort.Slice(companies, func(i, j int) bool {
		nameI, _ := companies[i]["company_name"].(string)
		nameJ, _ := companies[j]["company_name"].(string)
		return strings.ToLower(nameI) < strings.ToLower(nameJ)
	})

	respond.JSON(w, http.StatusOK, companies)
}

// GET /v2/company/{company_id}
// Python: cla/routes.py:1164 get_company()
// Calls: cla.controllers.company.get_company

func (h *Handlers) GetCompanyV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	companyID := chi.URLParam(r, "company_id")

	item, found, err := h.companies.GetByID(ctx, companyID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		// Python controller returns 200 with an errors payload.
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"company_id": "Company not found"}})
		return
	}

	respond.JSON(w, http.StatusOK, store.ItemToInterfaceMap(item))
}

// GET /v1/company/{company_id}/project/unsigned
// Python: cla/routes.py:1174 get_unsigned_projects_for_company()
// Calls: cla.controllers.project.get_unsigned_projects_for_company

func (h *Handlers) GetUnsignedProjectsForCompanyV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	companyID := chi.URLParam(r, "company_id")

	// Validate company exists (Python: company.load())
	_, found, err := h.companies.GetByID(ctx, companyID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"company_id": "Company not found"}})
		return
	}

	// Identify projects with an approved+signed CCLA for this company.
	sigItems, err := h.signatures.QueryByReferenceID(ctx, companyID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	signedProjects := make(map[string]struct{}, 64)
	for _, it := range sigItems {
		if !getAttrBool(it, "signature_signed") {
			continue
		}
		if !getAttrBool(it, "signature_approved") {
			continue
		}
		if getAttrString(it, "signature_type") != "ccla" {
			continue
		}
		if getAttrString(it, "signature_reference_type") != "company" {
			continue
		}
		// Exclude employee signatures (Python filter: attribute_not_exists(signature_user_ccla_company_id)).
		if _, ok := it["signature_user_ccla_company_id"]; ok {
			continue
		}
		pid := getAttrString(it, "signature_project_id")
		if pid != "" {
			signedProjects[pid] = struct{}{}
		}
	}

	projectItems, err := h.projects.ScanAll(ctx)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	unsigned := make([]map[string]any, 0, 16)
	for _, it := range projectItems {
		pid := getAttrString(it, "project_id")
		if pid == "" {
			continue
		}
		if _, ok := signedProjects[pid]; ok {
			continue
		}
		if !getAttrBool(it, "project_ccla_enabled") {
			continue
		}
		unsigned = append(unsigned, store.ItemToInterfaceMap(it))
	}

	respond.JSON(w, http.StatusOK, unsigned)
}

// POST /v1/company
// Python: cla/routes.py:1189 post_company()
// Calls: cla.controllers.company.create_company

func (h *Handlers) PostCompanyV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	type request struct {
		CompanyName string `json:"company_name"`
		// CompanyManagerUserName, CompanyManagerUserEmail, CompanyManagerID are parsed for
		// API compatibility but not used in company creation (mirrors Python behavior)
		IsSanctioned *bool `json:"is_sanctioned"`
	}
	var req request
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}
	if v, ok := flexibleStringParam(r, body, "company_name"); ok {
		req.CompanyName = v
	}
	// Note: company_manager_* fields are parsed for API compatibility but not used in company creation
	// This mirrors the Python behavior where these fields exist in the API but are not stored
	if b, ok, err := flexibleBoolParam(r, body, "is_sanctioned"); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"is_sanctioned": err.Error()}})
		return
	} else if ok {
		req.IsSanctioned = &b
	}
	companyName := strings.TrimSpace(req.CompanyName)
	if companyName == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"company_name": "Missing required value"}})
		return
	}

	isSanctioned := false
	if req.IsSanctioned != nil {
		isSanctioned = *req.IsSanctioned
	}

	// Manager is always the authenticated user in the legacy Python controller.
	managerItem, _, err := h.getOrCreateUser(ctx, authUser)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	managerID := getAttrString(managerItem, "user_id")

	// Duplicate check matches Python: iterate all companies and compare company_name exactly.
	companies, err := h.companies.ScanAll(ctx)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	for _, c := range companies {
		if getAttrString(c, "company_name") == companyName {
			respond.JSON(w, http.StatusConflict, map[string]any{
				"error":      "Company already exists.",
				"company_id": getAttrString(c, "company_id"),
			})
			return
		}
	}

	now := time.Now().UTC()
	companyID := uuid.New().String()
	item := map[string]types.AttributeValue{
		"company_id":          &types.AttributeValueMemberS{Value: companyID},
		"company_name":        &types.AttributeValueMemberS{Value: companyName},
		"signing_entity_name": &types.AttributeValueMemberS{Value: companyName},
		"company_manager_id":  &types.AttributeValueMemberS{Value: managerID},
		"company_acl":         &types.AttributeValueMemberSS{Value: []string{authUser.Username}},
		"is_sanctioned":       &types.AttributeValueMemberBOOL{Value: isSanctioned},
		"date_created":        &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"date_modified":       &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"version":             &types.AttributeValueMemberS{Value: "v1"},
	}

	if err := h.companies.PutItem(ctx, item); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	// Audit event (best-effort), matches controller wording.
	eventData := fmt.Sprintf("User %s created company %s with company_id: %s.", authUser.Username, companyName, companyID)
	eventSummary := fmt.Sprintf("User %s created company %s.", authUser.Username, companyName)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:      "CreateCompany",
		EventCompanyID: companyID,
		EventData:      eventData,
		EventSummary:   eventSummary,
		ContainsPII:    false,
	})

	respond.JSON(w, http.StatusOK, store.ItemToInterfaceMap(item))
}

// PUT /v1/company
// Python: cla/routes.py:1229 put_company()
// Calls: cla.controllers.company.update_company

func (h *Handlers) PutCompanyV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	type request struct {
		CompanyID        string  `json:"company_id"`
		CompanyName      *string `json:"company_name"`
		CompanyManagerID *string `json:"company_manager_id"`
		IsSanctioned     *bool   `json:"is_sanctioned"`
	}
	var req request
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}
	if v, ok := flexibleStringParam(r, body, "company_id"); ok {
		req.CompanyID = v
	}
	if v, ok := flexibleStringParam(r, body, "company_name"); ok {
		req.CompanyName = &v
	}
	if v, ok := flexibleStringParam(r, body, "company_manager_id"); ok {
		req.CompanyManagerID = &v
	}
	if b, ok, err := flexibleBoolParam(r, body, "is_sanctioned"); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"is_sanctioned": err.Error()}})
		return
	} else if ok {
		req.IsSanctioned = &b
	}
	companyID := strings.TrimSpace(req.CompanyID)
	if companyID == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"company_id": "Missing required value"}})
		return
	}

	item, found, err := h.companies.GetByID(ctx, companyID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"company_id": "Company not found"}})
		return
	}

	acl := getAttrStringSlice(item, "company_acl")
	if !stringSliceContainsExact(acl, authUser.Username) {
		respond.JSON(w, http.StatusForbidden, map[string]any{
			"title":       "Unauthorized",
			"description": "Provided Token credentials does not have sufficient permissions to access resource",
		})
		return
	}

	updateStr := ""
	if req.CompanyName != nil {
		item["company_name"] = &types.AttributeValueMemberS{Value: *req.CompanyName}
		updateStr += fmt.Sprintf("The company name was updated to %s. ", *req.CompanyName)
	}
	if req.CompanyManagerID != nil {
		parsed, err := uuid.Parse(*req.CompanyManagerID)
		if err != nil {
			// Python would raise during hug.types.uuid conversion.
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
			return
		}
		item["company_manager_id"] = &types.AttributeValueMemberS{Value: parsed.String()}
		updateStr += fmt.Sprintf("The company company manager id was updated to %s", parsed.String())
	}
	if req.IsSanctioned != nil {
		item["is_sanctioned"] = &types.AttributeValueMemberBOOL{Value: *req.IsSanctioned}
		updateStr += fmt.Sprintf("The company is_sanctioned was updated to %t. ", *req.IsSanctioned)
	}

	now := time.Now().UTC()
	item["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)}

	if err := h.companies.PutItem(ctx, item); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:      "UpdateCompany",
		EventCompanyID: companyID,
		EventData:      updateStr,
		EventSummary:   updateStr,
		ContainsPII:    false,
	})

	respond.JSON(w, http.StatusOK, store.ItemToInterfaceMap(item))
}

// DELETE /v1/company/{company_id}
// Python: cla/routes.py:1255 delete_company()
// Calls: cla.controllers.company.delete_company

func (h *Handlers) DeleteCompanyV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	companyID := chi.URLParam(r, "company_id")
	item, found, err := h.companies.GetByID(ctx, companyID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"company_id": "Company not found"}})
		return
	}

	acl := getAttrStringSlice(item, "company_acl")
	if !stringSliceContainsExact(acl, authUser.Username) {
		respond.JSON(w, http.StatusForbidden, map[string]any{
			"title":       "Unauthorized",
			"description": "Provided Token credentials does not have sufficient permissions to access resource",
		})
		return
	}

	companyName := getAttrString(item, "company_name")
	if err := h.companies.DeleteByID(ctx, companyID); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	eventData := fmt.Sprintf("The company %s with company_id %s was deleted.", companyName, companyID)
	eventSummary := fmt.Sprintf("The company %s was deleted.", companyName)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:      "DeleteCompany",
		EventCompanyID: companyID,
		EventData:      eventData,
		EventSummary:   eventSummary,
		ContainsPII:    false,
	})

	respond.JSON(w, http.StatusOK, map[string]any{"success": true})
}

// PUT /v1/company/{company_id}/import/whitelist/csv
// Python: cla/routes.py:1267 put_company_allowlist_csv()
// Calls: cla.controllers.company.update_company_allowlist_csv

func (h *Handlers) PutCompanyAllowlistCsvV1(w http.ResponseWriter, r *http.Request) {
	_, authResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authResp)
		return
	}

	companyID := chi.URLParam(r, "company_id")
	if _, err := uuid.Parse(companyID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"company_id": "invalid uuid"}})
		return
	}
	// EASYCLA_PARITY_FLAG: legacy Python has update_company_allowlist_csv commented out in
	// cla.controllers.company (route exists, controller function is missing). The runtime behavior
	// is an AttributeError -> 500, and there is not enough grounded source to add a safe fixed path here.
	respond.JSON(w, http.StatusInternalServerError, map[string]any{
		"errors": map[string]any{
			"server": "legacy python parity: update_company_allowlist_csv is not implemented",
		},
	})
}

// GET /v1/companies/{manager_id}
// Python: cla/routes.py:1280 get_manager_companies()
// Calls: cla.controllers.company.get_manager_companies

func (h *Handlers) GetManagerCompaniesV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	managerID := chi.URLParam(r, "manager_id")
	if _, err := uuid.Parse(managerID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"manager_id": "invalid uuid"}})
		return
	}

	items, err := h.companies.ScanAll(ctx)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	companies := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if getAttrString(it, "company_manager_id") == managerID {
			companies = append(companies, store.ItemToInterfaceMap(it))
		}
	}

	sort.Slice(companies, func(i, j int) bool {
		nameI, _ := companies[i]["company_name"].(string)
		nameJ, _ := companies[j]["company_name"].(string)
		return strings.ToLower(nameI) < strings.ToLower(nameJ)
	})

	respond.JSON(w, http.StatusOK, companies)
}

// GET /v1/project
// Python: cla/routes.py:1293 get_projects()
// Calls: cla.controllers.project.get_projects

func (h *Handlers) GetProjectsV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	items, err := h.projects.ScanAll(ctx)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	claLogoURL := os.Getenv("CLA_BUCKET_LOGO_URL")
	projects := make([]map[string]any, 0, len(items))
	for _, it := range items {
		m := store.ItemToInterfaceMap(it)
		ext, _ := m["project_external_id"].(string)
		m["logoUrl"] = fmt.Sprintf("%s/%s.png", claLogoURL, ext)
		delete(m, "project_external_id")
		projects = append(projects, m)
	}

	respond.JSON(w, http.StatusOK, projects)
}

// GET /v2/project/{project_id}
// Python: cla/routes.py:1309 get_project()
// Calls: cla.controllers.project.get_project, cla.log.debug, cla.log.warning

func (h *Handlers) GetProjectV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	projItem, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		// Python: cla.controllers.project.get_project returns an errors dict without changing HTTP status.
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	projectDict := store.ItemToInterfaceMap(projItem)
	projectExternalID := getAttrString(projItem, "project_external_id")
	projectDict["logoUrl"] = fmt.Sprintf("%s/%s.png", os.Getenv("CLA_BUCKET_LOGO_URL"), projectExternalID)
	delete(projectDict, "project_external_id")

	// Remove document_tabs from all project document lists.
	for _, docKey := range []string{"project_corporate_documents", "project_individual_documents", "project_member_documents"} {
		v, ok := projectDict[docKey]
		if !ok || v == nil {
			continue
		}
		list, ok := v.([]any)
		if !ok {
			continue
		}
		for _, d := range list {
			dm, ok := d.(map[string]any)
			if !ok {
				continue
			}
			delete(dm, "document_tabs")
		}
		projectDict[docKey] = list
	}

	// Map CLA group -> one or more Salesforce projects.
	projectsList := make([]map[string]any, 0)
	signedAtFoundation := false

	if h.projectCLAGroups != nil {
		mappings, err := h.projectCLAGroups.QueryByCLAGroupID(ctx, projectID)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
			return
		}
		for _, mi := range mappings {
			md := store.ItemToInterfaceMap(mi)

			projectSFID, _ := md["project_sfid"].(string)
			foundationSFID, _ := md["foundation_sfid"].(string)
			if projectSFID != "" && foundationSFID != "" && projectSFID == foundationSFID {
				signedAtFoundation = true
			}

			// Attach repositories by project_sfid.
			githubRepos := make([]map[string]any, 0)
			gitlabRepos := make([]map[string]any, 0)
			if projectSFID != "" && h.repos != nil {
				repoItems, err := h.repos.QueryByProjectSFID(ctx, projectSFID)
				if err != nil {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
					return
				}
				for _, ri := range repoItems {
					rd := store.ItemToInterfaceMap(ri)
					rt, _ := rd["repository_type"].(string)
					switch rt {
					case "github":
						githubRepos = append(githubRepos, rd)
					case "gitlab":
						gitlabRepos = append(gitlabRepos, rd)
					}
				}
			}
			md["github_repos"] = githubRepos
			md["gitlab_repos"] = gitlabRepos

			// Attach gerrit repos by project_sfid.
			gerritRepos := make([]map[string]any, 0)
			if projectSFID != "" && h.gerritInstances != nil {
				gItems, err := h.gerritInstances.QueryByProjectSFID(ctx, projectSFID)
				if err != nil {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
					return
				}
				for _, gi := range gItems {
					gerritRepos = append(gerritRepos, store.ItemToInterfaceMap(gi))
				}
			}
			md["gerrit_repos"] = gerritRepos

			// Compute standalone_project and lf_supported using project-service.
			standalone := false
			lfSupported := false
			if projectSFID != "" && h.salesforce != nil {
				if v, psErr := h.salesforce.IsStandaloneProject(ctx, projectSFID); psErr != nil {
					logging.Warnf("get_project: unable to compute standalone_project for project_sfid=%s: %v", projectSFID, psErr)
				} else {
					standalone = v
				}
				if v, psErr := h.salesforce.IsLFSupportedProject(ctx, projectSFID); psErr != nil {
					logging.Warnf("get_project: unable to compute lf_supported for project_sfid=%s: %v", projectSFID, psErr)
				} else {
					lfSupported = v
				}
			}
			md["standalone_project"] = standalone
			md["lf_supported"] = lfSupported

			projectsList = append(projectsList, md)
		}
	}

	projectDict["projects"] = projectsList
	projectDict["signed_at_foundation_level"] = signedAtFoundation

	respond.JSON(w, http.StatusOK, projectDict)
}

// GET /v1/project/{project_id}/manager
// Python: cla/routes.py:1401 get_project_managers()
// Calls: cla.controllers.project.get_project_managers

func (h *Handlers) GetProjectManagersV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	authUser, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	projItem, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	acl := getAttrStringSlice(projItem, "project_acl")
	allowed := false
	for _, u := range acl {
		if u == authUser.Username {
			allowed = true
			break
		}
	}
	if !allowed {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user_id": "You are not authorized to see the managers."}})
		return
	}

	managers := make([]map[string]any, 0, len(acl))
	for _, lfid := range acl {
		if strings.TrimSpace(lfid) == "" {
			continue
		}
		users, err := h.users.QueryByLFUsername(ctx, lfid)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"user_id": err.Error()}})
			return
		}
		if len(users) == 0 {
			managers = append(managers, map[string]any{"lfid": lfid})
			continue
		}
		u := users[0]
		managers = append(managers, map[string]any{
			"name":  getAttrString(u, "user_name"),
			"email": getUserEmailLikePython(u),
			"lfid":  getAttrString(u, "lf_username"),
		})
	}

	respond.JSON(w, http.StatusOK, managers)
}

// POST /v1/project/{project_id}/manager
// Python: cla/routes.py:1410 add_project_manager()
// Calls: cla.controllers.project.add_project_manager

func (h *Handlers) AddProjectManagerV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")

	authUser, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	// Hug passes body params from JSON, form-encoded, or query inputs.
	body, perr := parseFlexibleParams(r)
	if perr != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": perr.Error()}})
		return
	}
	lfid, _ := flexibleStringParam(r, body, "lfid")
	if strings.TrimSpace(lfid) == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"lfid": "Missing required field"}})
		return
	}
	lfid = strings.TrimSpace(lfid)

	projItem, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	acl := getAttrStringSlice(projItem, "project_acl")
	allowed := false
	for _, u := range acl {
		if u == authUser.Username {
			allowed = true
			break
		}
	}
	if !allowed {
		// Python: cla.controllers.project.add_project_manager returns a 200 with an errors dict.
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user": "You are not authorized to manage this CCLA."}})
		return
	}

	// Add to ACL set.
	seen := make(map[string]struct{}, len(acl)+1)
	for _, u := range acl {
		seen[u] = struct{}{}
	}
	seen[lfid] = struct{}{}
	newACL := make([]string, 0, len(seen))
	for u := range seen {
		if strings.TrimSpace(u) != "" {
			newACL = append(newACL, u)
		}
	}
	sort.Strings(newACL)
	projItem["project_acl"] = &types.AttributeValueMemberSS{Value: newACL}
	projItem["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}

	if err := h.projects.PutItem(ctx, projItem); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}

	// Legacy: return only managers that exist in the users table.
	managers := make([]map[string]any, 0)
	for _, u := range newACL {
		users, err := h.users.QueryByLFUsername(ctx, u)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"user_id": err.Error()}})
			return
		}
		if len(users) == 0 {
			continue
		}
		usr := users[0]
		managers = append(managers, map[string]any{
			"name":  getAttrString(usr, "user_name"),
			"email": getUserEmailLikePython(usr),
			"lfid":  getAttrString(usr, "lf_username"),
		})
	}

	projectName := getAttrString(projItem, "project_name")
	eventData := fmt.Sprintf("%s added %s to project %s", authUser.Username, lfid, projectName)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "AddProjectManager",
		EventCLAGroupID: projectID,
		EventData:       eventData,
		EventSummary:    eventData,
		ContainsPII:     true,
	})

	respond.JSON(w, http.StatusOK, managers)
}

// DELETE /v1/project/{project_id}/manager/{lfid}
// Python: cla/routes.py:1419 remove_project_manager()
// Calls: cla.controllers.project.remove_project_manager

func (h *Handlers) RemoveProjectManagerV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	lfid := chi.URLParam(r, "lfid")

	authUser, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	projItem, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	acl := getAttrStringSlice(projItem, "project_acl")
	allowed := false
	for _, u := range acl {
		if u == authUser.Username {
			allowed = true
			break
		}
	}
	if !allowed {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user": "You are not authorized to manage this CCLA."}})
		return
	}
	// Python: only prevent removing the last CLA manager when the request attempts to remove itself.
	if len(acl) == 1 && authUser.Username == lfid {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user": "You cannot remove this manager because a CCLA must have at least one CLA manager."}})
		return
	}

	newACL := make([]string, 0, len(acl))
	for _, u := range acl {
		if u != lfid {
			newACL = append(newACL, u)
		}
	}
	if len(newACL) == 0 {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"user": "You cannot remove this manager because a CCLA must have at least one CLA manager."}})
		return
	}
	sort.Strings(newACL)
	projItem["project_acl"] = &types.AttributeValueMemberSS{Value: newACL}
	projItem["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}

	if err := h.projects.PutItem(ctx, projItem); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}

	// Legacy: return only managers that exist in the users table.
	managers := make([]map[string]any, 0)
	for _, u := range newACL {
		users, err := h.users.QueryByLFUsername(ctx, u)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"user_id": err.Error()}})
			return
		}
		if len(users) == 0 {
			continue
		}
		usr := users[0]
		managers = append(managers, map[string]any{
			"name":  getAttrString(usr, "user_name"),
			"email": getUserEmailLikePython(usr),
			"lfid":  getAttrString(usr, "lf_username"),
		})
	}

	// Python: event_data = f'{lfid} removed from project {project.get_project_id()}'
	eventData := fmt.Sprintf("%s removed from project %s", lfid, projectID)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "RemoveProjectManager",
		EventCLAGroupID: projectID,
		EventData:       eventData,
		EventSummary:    eventData,
		ContainsPII:     true,
	})

	respond.JSON(w, http.StatusOK, managers)
}

// GET /v1/project/external/{project_external_id}
// Python: cla/routes.py:1428 get_external_project()
// Calls: cla.controllers.project.get_projects_by_external_id

func (h *Handlers) GetExternalProjectV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectExternalID := chi.URLParam(r, "project_external_id")

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	perms, err := h.userPerms.Get(ctx, authUser.Username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"username": "user does not exist. "}})
			return
		}
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	authorized := perms.Projects
	if !stringSliceContainsExact(authorized, projectExternalID) {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"username": "user is not authorized for this Salesforce ID. "}})
		return
	}

	items, err := h.projects.QueryByExternalID(ctx, projectExternalID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	claLogoURL := os.Getenv("CLA_BUCKET_LOGO_URL")
	projects := make([]map[string]any, 0, len(items))
	for _, it := range items {
		m := store.ItemToInterfaceMap(it)
		ext, _ := m["project_external_id"].(string)
		m["logoUrl"] = fmt.Sprintf("%s/%s.png", claLogoURL, ext)
		projects = append(projects, m)
	}

	respond.JSON(w, http.StatusOK, projects)
}

// POST /v1/project
// Python: cla/routes.py:1438 post_project()
// Calls: cla.controllers.project.create_project

func (h *Handlers) PostProjectV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	type request struct {
		ProjectExternalID                string `json:"project_external_id"`
		ProjectName                      string `json:"project_name"`
		ProjectICLAEnabled               *bool  `json:"project_icla_enabled"`
		ProjectCCLAEnabled               *bool  `json:"project_ccla_enabled"`
		ProjectCCLARequiresICLASignature *bool  `json:"project_ccla_requires_icla_signature"`
	}
	var req request
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}
	if v, ok := flexibleStringParam(r, body, "project_external_id"); ok {
		req.ProjectExternalID = v
	}
	if v, ok := flexibleStringParam(r, body, "project_name"); ok {
		req.ProjectName = v
	}
	if b, ok, err := flexibleBoolParam(r, body, "project_icla_enabled"); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_icla_enabled": err.Error()}})
		return
	} else if ok {
		req.ProjectICLAEnabled = &b
	}
	if b, ok, err := flexibleBoolParam(r, body, "project_ccla_enabled"); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_ccla_enabled": err.Error()}})
		return
	} else if ok {
		req.ProjectCCLAEnabled = &b
	}
	if b, ok, err := flexibleBoolParam(r, body, "project_ccla_requires_icla_signature"); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_ccla_requires_icla_signature": err.Error()}})
		return
	} else if ok {
		req.ProjectCCLARequiresICLASignature = &b
	}

	// Hug enforces required params; mirror that behavior.
	missing := map[string]any{}
	if strings.TrimSpace(req.ProjectExternalID) == "" {
		missing["project_external_id"] = "missing"
	}
	if strings.TrimSpace(req.ProjectName) == "" {
		missing["project_name"] = "missing"
	}
	if req.ProjectICLAEnabled == nil {
		missing["project_icla_enabled"] = "missing"
	}
	if req.ProjectCCLAEnabled == nil {
		missing["project_ccla_enabled"] = "missing"
	}
	if req.ProjectCCLARequiresICLASignature == nil {
		missing["project_ccla_requires_icla_signature"] = "missing"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}

	now := time.Now().UTC()
	projectID := uuid.New().String()

	// Build the AttributeValue map directly so InterfaceMapToItem's
	// isNumericString heuristic cannot coerce an all-digit project_name to N.
	item := map[string]types.AttributeValue{
		"project_id":                           &types.AttributeValueMemberS{Value: projectID},
		"project_external_id":                  &types.AttributeValueMemberS{Value: req.ProjectExternalID},
		"project_name":                         &types.AttributeValueMemberS{Value: req.ProjectName},
		"project_icla_enabled":                 &types.AttributeValueMemberBOOL{Value: *req.ProjectICLAEnabled},
		"project_ccla_enabled":                 &types.AttributeValueMemberBOOL{Value: *req.ProjectCCLAEnabled},
		"project_ccla_requires_icla_signature": &types.AttributeValueMemberBOOL{Value: *req.ProjectCCLARequiresICLASignature},
		"project_acl":                          &types.AttributeValueMemberSS{Value: []string{authUser.Username}},
		"date_created":                         &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"date_modified":                        &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		"version":                              &types.AttributeValueMemberS{Value: "v1"},
	}
	if err := h.projects.PutItem(ctx, item); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	eventData := fmt.Sprintf("Project-%s created", req.ProjectName)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "CreateProject",
		EventProjectID:  req.ProjectExternalID,
		EventCLAGroupID: projectID,
		EventData:       eventData,
		EventSummary:    eventData,
		ContainsPII:     false,
	})

	respond.JSON(w, http.StatusOK, store.ItemToInterfaceMap(item))
}

// PUT /v1/project
// Python: cla/routes.py:1473 put_project()
// Calls: cla.controllers.project.update_project

func (h *Handlers) PutProjectV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	type request struct {
		ProjectID                        string  `json:"project_id"`
		ProjectExternalID                *string `json:"project_external_id,omitempty"`
		ProjectName                      *string `json:"project_name,omitempty"`
		ProjectICLAEnabled               *bool   `json:"project_icla_enabled,omitempty"`
		ProjectCCLAEnabled               *bool   `json:"project_ccla_enabled,omitempty"`
		ProjectCCLARequiresICLASignature *bool   `json:"project_ccla_requires_icla_signature,omitempty"`
	}
	var req request
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}
	if v, ok := flexibleStringParam(r, body, "project_id"); ok {
		req.ProjectID = v
	}
	if v, ok := flexibleStringParam(r, body, "project_external_id"); ok {
		req.ProjectExternalID = &v
	}
	if v, ok := flexibleStringParam(r, body, "project_name"); ok {
		req.ProjectName = &v
	}
	if b, ok, err := flexibleBoolParam(r, body, "project_icla_enabled"); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_icla_enabled": err.Error()}})
		return
	} else if ok {
		req.ProjectICLAEnabled = &b
	}
	if b, ok, err := flexibleBoolParam(r, body, "project_ccla_enabled"); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_ccla_enabled": err.Error()}})
		return
	} else if ok {
		req.ProjectCCLAEnabled = &b
	}
	if b, ok, err := flexibleBoolParam(r, body, "project_ccla_requires_icla_signature"); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_ccla_requires_icla_signature": err.Error()}})
		return
	} else if ok {
		req.ProjectCCLARequiresICLASignature = &b
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "missing"}})
		return
	}
	if _, err := uuid.Parse(strings.TrimSpace(req.ProjectID)); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	item, found, err := h.projects.GetByID(ctx, req.ProjectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	projectACL := getAttrStringSlice(item, "project_acl")
	if !stringSliceContainsExact(projectACL, authUser.Username) {
		// Python raises falcon.HTTPForbidden via project_acl_verify.
		respond.JSON(w, http.StatusForbidden, map[string]any{
			"title":       "Unauthorized",
			"description": "Provided Token credentials does not have sufficient permissions to access resource",
		})
		return
	}

	// Patch the AttributeValue map directly so we never round-trip pynamodb
	// types through InterfaceMapToItem's isNumericString heuristic, which
	// can silently coerce digit-only S fields to N.
	updatedString := " "

	if req.ProjectExternalID != nil {
		item["project_external_id"] = &types.AttributeValueMemberS{Value: *req.ProjectExternalID}
		updatedString += fmt.Sprintf("project_external_id changed to %s \n", *req.ProjectExternalID)
	}
	if req.ProjectName != nil {
		item["project_name"] = &types.AttributeValueMemberS{Value: *req.ProjectName}
		updatedString += fmt.Sprintf("project_name changed to %s \n", *req.ProjectName)
	}
	if req.ProjectICLAEnabled != nil {
		item["project_icla_enabled"] = &types.AttributeValueMemberBOOL{Value: *req.ProjectICLAEnabled}
		updatedString += fmt.Sprintf("project_icla_enabled changed to %s \n", boolString(*req.ProjectICLAEnabled))
	}
	if req.ProjectCCLAEnabled != nil {
		item["project_ccla_enabled"] = &types.AttributeValueMemberBOOL{Value: *req.ProjectCCLAEnabled}
		updatedString += fmt.Sprintf("project_ccla_enabled changed to %s \n", boolString(*req.ProjectCCLAEnabled))
	}
	if req.ProjectCCLARequiresICLASignature != nil {
		item["project_ccla_requires_icla_signature"] = &types.AttributeValueMemberBOOL{Value: *req.ProjectCCLARequiresICLASignature}
		updatedString += fmt.Sprintf("project_ccla_requires_icla_signature changed to %s \n", boolString(*req.ProjectCCLARequiresICLASignature))
	}

	now := time.Now().UTC()
	item["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)}

	if err := h.projects.PutItem(ctx, item); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	eventData := fmt.Sprintf("Project- %s Updates: %s", req.ProjectID, updatedString)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "UpdateProject",
		EventCLAGroupID: req.ProjectID,
		EventData:       eventData,
		EventSummary:    eventData,
		ContainsPII:     false,
	})

	respond.JSON(w, http.StatusOK, store.ItemToInterfaceMap(item))
}

// DELETE /v1/project/{project_id}
// Python: cla/routes.py:1501 delete_project()
// Calls: cla.controllers.project.delete_project

func (h *Handlers) DeleteProjectV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	item, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	projectACL := getAttrStringSlice(item, "project_acl")
	if !stringSliceContainsExact(projectACL, authUser.Username) {
		respond.JSON(w, http.StatusForbidden, map[string]any{
			"title":       "Unauthorized",
			"description": "Provided Token credentials does not have sufficient permissions to access resource",
		})
		return
	}

	projectName := getAttrString(item, "project_name")
	eventData := fmt.Sprintf("Project-%s deleted", projectName)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "DeleteProject",
		EventCLAGroupID: projectID,
		EventData:       eventData,
		EventSummary:    eventData,
		ContainsPII:     false,
	})

	if err := h.projects.DeleteByID(ctx, projectID); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"success": true})
}

// GET /v1/project/{project_id}/repositories
// Python: cla/routes.py:1512 get_project_repositories()
// Calls: cla.controllers.project.get_project_repositories

func (h *Handlers) GetProjectRepositoriesV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	authUser, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	projItem, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"valid": false, "errors": map[string]any{"errors": map[string]any{"project_id": "Project not found"}}})
		return
	}

	projectSFID := getAttrString(projItem, "project_external_id")
	if ok, errMap := h.checkUserAuthorization(ctx, authUser.Username, projectSFID); !ok {
		respond.JSON(w, http.StatusOK, errMap)
		return
	}

	items, err := h.repos.QueryByProjectID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}

	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		m := store.ItemToInterfaceMap(it)
		out = append(out, m)
	}
	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/project/{project_id}/repositories_group_by_organization
// Python: cla/routes.py:1522 get_project_repositories_group_by_organization()
// Calls: cla.controllers.project.get_project_repositories_group_by_organization

func (h *Handlers) GetProjectRepositoriesGroupByOrganizationV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	authUser, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	projItem, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"valid": false, "errors": map[string]any{"errors": map[string]any{"project_id": "Project not found"}}})
		return
	}

	projectSFID := getAttrString(projItem, "project_external_id")
	if ok, errMap := h.checkUserAuthorization(ctx, authUser.Username, projectSFID); !ok {
		respond.JSON(w, http.StatusOK, errMap)
		return
	}

	items, err := h.repos.QueryByProjectID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}

	grouped := make(map[string][]map[string]any)
	order := make([]string, 0)
	for _, it := range items {
		org := getAttrString(it, "repository_organization_name")
		if _, ok := grouped[org]; !ok {
			order = append(order, org)
		}
		grouped[org] = append(grouped[org], store.ItemToInterfaceMap(it))
	}

	out := make([]map[string]any, 0, len(order))
	for _, org := range order {
		out = append(out, map[string]any{
			"name":         org,
			"repositories": grouped[org],
		})
	}

	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/project/{project_id}/configuration_orgs_and_repos
// Python: cla/routes.py:1532 get_project_configuration_orgs_and_repos()
// Calls: cla.controllers.project.get_project_configuration_orgs_and_repos

func (h *Handlers) GetProjectConfigurationOrgsAndReposV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	authUser, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	projItem, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"valid": false, "errors": map[string]any{"errors": map[string]any{"project_id": "Project not found"}}})
		return
	}

	projectSFID := getAttrString(projItem, "project_external_id")
	if ok, errMap := h.checkUserAuthorization(ctx, authUser.Username, projectSFID); !ok {
		respond.JSON(w, http.StatusOK, errMap)
		return
	}

	// organizations: GitHub orgs + GitHub repos visible to each org installation
	orgItems, err := h.githubOrgs.QueryBySFID(ctx, projectSFID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}

	orgsOut := make([]map[string]any, 0)
	for _, orgItem := range orgItems {
		installationID := int64(getAttrInt(orgItem, "organization_installation_id"))
		if installationID == 0 {
			// Legacy: skip orgs without an installation.
			continue
		}
		ghRepos, err := h.github.ListInstallationRepositories(ctx, installationID)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"organization_installation_id": err.Error()}})
			return
		}

		reposOut := make([]map[string]any, 0, len(ghRepos))
		for _, gr := range ghRepos {
			enabled := false
			if repoItem, found, err := h.repos.GetByExternalIDAndType(ctx, strconv.FormatInt(gr.ID, 10), "github"); err == nil && found {
				if b, ok := repoItem["enabled"].(*types.AttributeValueMemberBOOL); ok {
					enabled = b.Value
				}
			}
			reposOut = append(reposOut, map[string]any{
				"repository_github_id": gr.ID,
				"repository_name":      gr.Full,
				"repository_type":      "github",
				"repository_url":       gr.HTMLURL,
				"enabled":              enabled,
			})
		}

		orgDict := normalizeGitHubOrgDict(store.ItemToInterfaceMap(orgItem))
		orgDict["repositories"] = reposOut
		orgsOut = append(orgsOut, orgDict)
	}

	// repositories: SFDC repositories keyed by repository_sfdc_id == projectSFID
	repoItems, err := h.repos.QueryBySFDCID(ctx, projectSFID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	repos := make([]map[string]any, 0, len(repoItems))
	for _, it := range repoItems {
		repos = append(repos, store.ItemToInterfaceMap(it))
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"orgs_and_repos": orgsOut,
		"repositories":   repos,
	})
}

// GET /v2/project/{project_id}/document/{document_type}
// Python: cla/routes.py:1543 get_project_document()
// Calls: cla.controllers.project.get_project_document

func (h *Handlers) GetProjectDocumentV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	documentType := chi.URLParam(r, "document_type")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{
			"errors": map[string]any{"project_id": "invalid uuid"},
		})
		return
	}

	// Hug one_of(["individual", "corporate"]) rejects this before controller logic.
	docsKey, noDocMsg, ok := projectDocsKey(documentType)
	if !ok {
		respond.JSON(w, http.StatusBadRequest, map[string]any{
			"errors": map[string]any{"document_type": "invalid"},
		})
		return
	}

	projItem, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{
			"errors": map[string]any{"project_id": err.Error()},
		})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{
			"errors": map[string]any{"project_id": "Project not found"},
		})
		return
	}

	docsAV, hasDocs := projItem[docsKey]
	if !hasDocs {
		respond.JSON(w, http.StatusOK, map[string]any{
			"errors": map[string]any{"document": noDocMsg},
		})
		return
	}

	// NOTE: Legacy Python Project.get_project_*_document always returns the latest document and
	// ignores requested major/minor. This endpoint is v2 and doesn't accept versions anyway.
	doc, _, _, okDoc := latestDocFromDocsAV(docsAV)
	if !okDoc {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"document": noDocMsg}})
		return
	}

	respond.JSON(w, http.StatusOK, store.ItemToInterfaceMap(doc))
}

// GET /v2/project/{project_id}/document/{document_type}/pdf
// Python: cla/routes.py:1555 get_project_document_raw()
// Calls: cla.controllers.project.get_project_document_raw

func (h *Handlers) GetProjectDocumentRawV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	documentType := chi.URLParam(r, "document_type")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}
	// Hug one_of(["individual", "corporate"]) rejects this before auth/controller logic.
	docsKey, noDocMsg, ok := projectDocsKey(documentType)
	if !ok {
		respond.JSON(w, http.StatusBadRequest, map[string]any{
			"errors": map[string]any{"document_type": "invalid"},
		})
		return
	}

	// Auth required.
	_, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	projItem, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	docsAV, hasDocs := projItem[docsKey]
	if !hasDocs {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"document": noDocMsg}})
		return
	}

	doc, _, _, okDoc := latestDocFromDocsAV(docsAV)
	if !okDoc {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"document": noDocMsg}})
		return
	}

	pdfBytes, err := h.fetchProjectDocumentPDF(ctx, doc)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"document": err.Error()}})
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

// GET /v1/project/{project_id}/document/{document_type}/pdf/{document_major_version}/{document_minor_version}
// Python: cla/routes.py:1573 get_project_document_matching_version()
// Calls: cla.controllers.project.get_project_document_raw

func (h *Handlers) GetProjectDocumentMatchingVersionV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	documentType := chi.URLParam(r, "document_type")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}
	if _, err := strconv.ParseFloat(chi.URLParam(r, "document_major_version"), 64); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"document_major_version": "invalid"}})
		return
	}
	if _, err := strconv.ParseFloat(chi.URLParam(r, "document_minor_version"), 64); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"document_minor_version": "invalid"}})
		return
	}

	// Hug one_of(["individual", "corporate"]) rejects this before auth/controller logic.
	docsKey, noDocMsg, ok := projectDocsKey(documentType)
	if !ok {
		respond.JSON(w, http.StatusBadRequest, map[string]any{
			"errors": map[string]any{"document_type": "invalid"},
		})
		return
	}

	// Auth required.
	_, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	projItem, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	docsAV, hasDocs := projItem[docsKey]
	if !hasDocs {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"document": noDocMsg}})
		return
	}
	// EASYCLA_PARITY_FLAG: default preserves the legacy Python bug where requested major/minor
	// versions are ignored and the latest document is always served.
	var doc map[string]types.AttributeValue
	if !parity.FixGetProjectDocumentMatchingVersionV1 {
		var okDoc bool
		doc, _, _, okDoc = latestDocFromDocsAV(docsAV)
		if !okDoc {
			respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"document": noDocMsg}})
			return
		}
	} else {
		majorStr, err := wholeNumberString(chi.URLParam(r, "document_major_version"))
		if err != nil || majorStr == "" {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"document_major_version": "invalid"}})
			return
		}
		minorStr, err := wholeNumberString(chi.URLParam(r, "document_minor_version"))
		if err != nil || minorStr == "" {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"document_minor_version": "invalid"}})
			return
		}
		major, _ := strconv.Atoi(majorStr)
		minor, _ := strconv.Atoi(minorStr)
		var okDoc bool
		doc, okDoc = docByVersionFromDocsAV(docsAV, major, minor)
		if !okDoc {
			respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"document": "Document version not found"}})
			return
		}
	}

	pdfBytes, err := h.fetchProjectDocumentPDF(ctx, doc)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"document": err.Error()}})
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

// GET /v2/project/{project_id}/companies
// Python: cla/routes.py:1596 get_project_companies()
// Calls: cla.controllers.project.get_project_companies

func (h *Handlers) GetProjectCompaniesV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	// Legacy behavior: validate project exists (even though results are derived from signatures).
	_, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	sigs, err := h.signatures.QueryByProjectID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"signature_project_id": err.Error()}})
		return
	}

	companyIDsSet := map[string]struct{}{}
	for _, sig := range sigs {
		if !getAttrBool(sig, "signature_signed") {
			continue
		}
		if !getAttrBool(sig, "signature_approved") {
			continue
		}
		if getAttrString(sig, "signature_reference_type") != "company" {
			continue
		}
		cid := getAttrString(sig, "signature_reference_id")
		if cid == "" {
			continue
		}
		companyIDsSet[cid] = struct{}{}
	}

	companies := make([]map[string]any, 0, len(companyIDsSet))
	for cid := range companyIDsSet {
		cItem, cFound, err := h.companies.GetByID(ctx, cid)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"company_id": err.Error()}})
			return
		}
		if !cFound {
			continue
		}
		companies = append(companies, store.ItemToInterfaceMap(cItem))
	}

	sort.Slice(companies, func(i, j int) bool {
		a := strings.ToLower(fmt.Sprintf("%v", companies[i]["company_name"]))
		b := strings.ToLower(fmt.Sprintf("%v", companies[j]["company_name"]))
		return a < b
	})

	respond.JSON(w, http.StatusOK, companies)
}

// POST /v1/project/{project_id}/document/{document_type}
// Python: cla/routes.py:1613 post_project_document()
// Calls: cla.controllers.project.post_project_document

func (h *Handlers) PostProjectDocumentV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	documentType := chi.URLParam(r, "document_type")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	authUser, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	// Hug route typing rejects invalid document_type before controller logic.
	if _, _, ok := projectDocsKey(documentType); !ok {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"document_type": "invalid"}})
		return
	}

	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": err.Error()}})
		return
	}
	getString := func(key string) string {
		if v, ok := flexibleStringParam(r, body, key); ok {
			return v
		}
		return ""
	}
	newMajorVersion, _, err := flexibleBoolParam(r, body, "new_major_version")
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"new_major_version": err.Error()}})
		return
	}
	req := struct {
		DocumentName            string
		DocumentContentType     string
		DocumentContent         string
		DocumentPreamble        string
		DocumentLegalEntityName string
		NewMajorVersion         bool
	}{
		DocumentName:            getString("document_name"),
		DocumentContentType:     getString("document_content_type"),
		DocumentContent:         getString("document_content"),
		DocumentPreamble:        getString("document_preamble"),
		DocumentLegalEntityName: getString("document_legal_entity_name"),
		NewMajorVersion:         newMajorVersion,
	}
	missing := map[string]any{}
	if strings.TrimSpace(req.DocumentName) == "" {
		missing["document_name"] = "missing"
	}
	if strings.TrimSpace(req.DocumentContentType) == "" {
		missing["document_content_type"] = "missing"
	}
	if strings.TrimSpace(req.DocumentContent) == "" {
		missing["document_content"] = "missing"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}

	// Validate content types (legacy supported).
	switch req.DocumentContentType {
	case "pdf", "url+pdf", "storage+pdf":
		// ok
	default:
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"document_content_type": "invalid"}})
		return
	}

	projItem, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	// ACL verify (raises 403 in Python).
	acl := getAttrStringSlice(projItem, "project_acl")
	allowed := false
	for _, u := range acl {
		if u == authUser.Username {
			allowed = true
			break
		}
	}
	if !allowed {
		respond.JSON(w, http.StatusForbidden, map[string]any{"title": "Unauthorized", "description": "You are not authorized to perform this action."})
		return
	}

	docsKey, _, ok := projectDocsKey(documentType)
	if !ok {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"document_type": "invalid"}})
		return
	}

	docsAV := projItem[docsKey]
	lastMajor, lastMinor := lastDocVersionFromDocsAV(docsAV)
	newMajor := lastMajor
	var newMinor int
	if req.NewMajorVersion {
		newMajor = lastMajor + 1
		newMinor = 0
	} else {
		if newMajor == 0 {
			newMajor = 1
		}
		newMinor = lastMinor + 1
	}

	fileID := uuid.New().String()
	now := time.Now().UTC()

	// Persist content when using storage+pdf.
	if strings.HasPrefix(req.DocumentContentType, "storage+") {
		bucket := os.Getenv("CLA_SIGNATURE_FILES_BUCKET")
		if bucket == "" {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"bucket": "CLA_SIGNATURE_FILES_BUCKET not set"}})
			return
		}
		b, err := base64.StdEncoding.DecodeString(req.DocumentContent)
		if err != nil {
			// Python would raise; surface as 500.
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"document_content": err.Error()}})
			return
		}
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(h.region))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"aws": err.Error()}})
			return
		}
		if strings.ToLower(os.Getenv("STAGE")) == "local" {
			cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
				if service == s3.ServiceID {
					return aws.Endpoint{URL: "http://localhost:8001", SigningRegion: h.region, HostnameImmutable: true}, nil
				}
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			})
		}
		client := s3.NewFromConfig(cfg, func(o *s3.Options) {
			if strings.ToLower(os.Getenv("STAGE")) == "local" {
				o.UsePathStyle = true
			}
		})
		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(fileID),
			Body:   bytes.NewReader(b),
		})
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"storage": err.Error()}})
			return
		}
	}

	newDoc := map[string]types.AttributeValue{
		"document_name":          &types.AttributeValueMemberS{Value: req.DocumentName},
		"document_file_id":       &types.AttributeValueMemberS{Value: fileID},
		"document_content_type":  &types.AttributeValueMemberS{Value: req.DocumentContentType},
		"document_major_version": &types.AttributeValueMemberN{Value: strconv.Itoa(newMajor)},
		"document_minor_version": &types.AttributeValueMemberN{Value: strconv.Itoa(newMinor)},
		"document_creation_date": &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
		// Python DocumentModel has default document_tabs=list.
		"document_tabs": &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
	}
	if req.DocumentPreamble != "" {
		newDoc["document_preamble"] = &types.AttributeValueMemberS{Value: req.DocumentPreamble}
	}
	if req.DocumentLegalEntityName != "" {
		newDoc["document_legal_entity_name"] = &types.AttributeValueMemberS{Value: req.DocumentLegalEntityName}
	}
	// Inline content is only stored for non-storage content types.
	if !strings.HasPrefix(req.DocumentContentType, "storage+") {
		newDoc["document_content"] = &types.AttributeValueMemberS{Value: req.DocumentContent}
	}

	updatedDocsAV := appendDocToDocsAV(docsAV, newDoc)
	projItem[docsKey] = updatedDocsAV
	projItem["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)}

	if err := h.projects.PutItem(ctx, projItem); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}

	projectName := getAttrString(projItem, "project_name")
	eventData := fmt.Sprintf("Created new document for Project-%s ", projectName)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "CreateProjectDocument",
		EventCLAGroupID: projectID,
		EventData:       eventData,
		EventSummary:    eventData,
		ContainsPII:     false,
	})

	// Return project.to_dict().
	projectDict := store.ItemToInterfaceMap(projItem)
	if bucketURL := os.Getenv("CLA_BUCKET_LOGO_URL"); bucketURL != "" {
		if ext, ok := projectDict["project_external_id"].(string); ok && ext != "" {
			projectDict["logoUrl"] = fmt.Sprintf("%s/%s.png", bucketURL, ext)
		}
	}
	respond.JSON(w, http.StatusOK, projectDict)
}

// POST /v1/project/{project_id}/document/template/{document_type}
// Python: cla/routes.py:1665 post_project_document_template()
// Calls: cla.controllers.project.post_project_document_template

func (h *Handlers) PostProjectDocumentTemplateV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	documentType := chi.URLParam(r, "document_type")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	// Hug route typing rejects invalid document_type before controller logic.
	if _, _, ok := projectDocsKey(documentType); !ok {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"document_type": "invalid"}})
		return
	}

	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"request": err.Error()}})
		return
	}
	getString := func(key string) string {
		if v, ok := flexibleStringParam(r, body, key); ok {
			return v
		}
		return ""
	}
	newMajorVersion, _, err := flexibleBoolParam(r, body, "new_major_version")
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"new_major_version": err.Error()}})
		return
	}
	req := struct {
		DocumentName            string
		DocumentPreamble        string
		DocumentLegalEntityName string
		TemplateName            string
		NewMajorVersion         bool
	}{
		DocumentName:            getString("document_name"),
		DocumentPreamble:        getString("document_preamble"),
		DocumentLegalEntityName: getString("document_legal_entity_name"),
		TemplateName:            getString("template_name"),
		NewMajorVersion:         newMajorVersion,
	}
	missing := map[string]any{}
	if strings.TrimSpace(req.DocumentName) == "" {
		missing["document_name"] = "missing"
	}
	if strings.TrimSpace(req.DocumentPreamble) == "" {
		missing["document_preamble"] = "missing"
	}
	if strings.TrimSpace(req.DocumentLegalEntityName) == "" {
		missing["document_legal_entity_name"] = "missing"
	}
	if strings.TrimSpace(req.TemplateName) == "" {
		missing["template_name"] = "missing"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}

	// Validate template name (legacy Python route uses hug.types.one_of).
	switch req.TemplateName {
	case "CNCFTemplate", "OpenBMCTemplate", "TungstenFabricTemplate", "OpenColorIOTemplate", "OpenVDBTemplate", "ONAPTemplate", "TektonTemplate":
		// ok
	default:
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"template_name": "invalid template_name"}})
		return
	}

	// Load project
	projectItem, ok, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !ok {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "not found"}})
		return
	}

	// Validate ACL
	// project_acl can be stored as a DynamoDB string set (SS) or list (L).
	// Keep the legacy exact-username membership check while accepting both shapes.
	if !stringSliceContainsExact(getAttrStringSlice(projectItem, "project_acl"), authUser.Username) {
		respond.JSON(w, http.StatusForbidden, map[string]any{
			"title":       "Forbidden",
			"description": "You do not have permission to create project document templates",
		})
		return
	}

	// Select which project document list to update.
	docsKey := ""
	switch strings.ToLower(documentType) {
	case "individual":
		docsKey = "project_individual_documents"
	case "corporate":
		docsKey = "project_corporate_documents"
	default:
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"document_type": "invalid document_type"}})
		return
	}

	major, minor := lastDocVersionFromDocsAV(projectItem[docsKey])

	// Build new document entry.
	docID := uuid.NewString()
	fileID := uuid.NewString()
	creation := formatPynamoDateTimeUTC(time.Now())

	// Legacy Python default major_version=1, minor_version=0 for new Document() instances.
	newMajor := 1
	newMinor := 0
	if req.NewMajorVersion {
		newMajor = major + 1
		newMinor = 0
	} else {
		// EASYCLA_PARITY_FLAG: default preserves the legacy Python template minor-version bug
		// where the document major version can reset to the default 1 instead of the current major.
		if parity.FixPostProjectDocumentTemplateV1Versioning {
			if major == 0 {
				major = 1
			}
			newMajor = major
		}
		newMinor = minor + 1
	}

	// Render HTML and build tabs from the template.
	html, err := contracts.RenderHTML(req.TemplateName, documentType, newMajor, newMinor, req.DocumentLegalEntityName, req.DocumentPreamble)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"template": err.Error()}})
		return
	}
	tabs, err := contracts.Tabs(req.TemplateName, documentType)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"template": err.Error()}})
		return
	}

	gen, err := pdf.NewDocRaptorFromEnv()
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"pdf": err.Error()}})
		return
	}
	pdfBytes, err := gen.GeneratePDF(ctx, html)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"pdf": err.Error()}})
		return
	}

	// Store PDF in S3 (same pattern as PostProjectDocumentV1 for storage+pdf).
	bucket := strings.TrimSpace(os.Getenv("CLA_SIGNATURE_FILES_BUCKET"))
	if bucket == "" {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"bucket": "CLA_SIGNATURE_FILES_BUCKET is empty"}})
		return
	}

	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("REGION"))
	}
	if region == "" {
		region = "us-east-1"
	}

	stage := strings.TrimSpace(os.Getenv("STAGE"))
	if stage == "" {
		stage = "dev"
	}

	loadOpts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if stage == "local" {
		endpointURL := "http://localhost:8001"
		loadOpts = append(loadOpts, config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				if service == s3.ServiceID {
					return aws.Endpoint{URL: endpointURL, SigningRegion: region, HostnameImmutable: true}, nil
				}
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			}),
		))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"s3": err.Error()}})
		return
	}
	// NOTE: Matches PostProjectDocumentV1 behavior (constructing a client per request).
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if stage == "local" {
			o.UsePathStyle = true
		}
	})

	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(fileID),
		Body:        bytes.NewReader(pdfBytes),
		ContentType: aws.String("application/pdf"),
	})
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"s3": err.Error()}})
		return
	}

	newDoc := map[string]types.AttributeValue{
		"document_id":                &types.AttributeValueMemberS{Value: docID},
		"document_name":              &types.AttributeValueMemberS{Value: req.DocumentName},
		"document_preamble":          &types.AttributeValueMemberS{Value: req.DocumentPreamble},
		"document_legal_entity_name": &types.AttributeValueMemberS{Value: req.DocumentLegalEntityName},
		"document_file_id":           &types.AttributeValueMemberS{Value: fileID},
		"document_major_version":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", newMajor)},
		"document_minor_version":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", newMinor)},
		"document_content_type":      &types.AttributeValueMemberS{Value: "storage+pdf"},
		"document_creation_date":     &types.AttributeValueMemberS{Value: creation},
		"document_tabs":              &types.AttributeValueMemberL{Value: docTabsFromTemplateTabs(tabs)},
	}

	// Append doc to project docs list.
	var docs []types.AttributeValue
	if av, ok := projectItem[docsKey]; ok {
		if l, ok := av.(*types.AttributeValueMemberL); ok {
			docs = append([]types.AttributeValue{}, l.Value...)
		}
	}
	docs = append(docs, &types.AttributeValueMemberM{Value: newDoc})
	projectItem[docsKey] = &types.AttributeValueMemberL{Value: docs}
	projectItem["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}

	if err := h.projects.PutItem(ctx, projectItem); err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project": err.Error()}})
		return
	}

	// Best-effort audit event (matches other document writes).
	projectName := getAttrString(projectItem, "project_name")
	eventData := fmt.Sprintf("Project Document created for project %s created with template %s", projectName, req.TemplateName)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "CreateProjectDocumentTemplate",
		EventCLAGroupID: projectID,
		EventData:       eventData,
		EventSummary:    eventData,
		ContainsPII:     false,
	})

	respond.JSON(w, http.StatusOK, store.ToInterface(&types.AttributeValueMemberM{Value: projectItem}))
}

// DELETE /v1/project/{project_id}/document/{document_type}/{major_version}/{minor_version}
// Python: cla/routes.py:1718 delete_project_document()
// Calls: cla.controllers.project.delete_project_document

func (h *Handlers) DeleteProjectDocumentV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	documentType := chi.URLParam(r, "document_type")
	// NOTE: Route params are {major_version}/{minor_version} in router.go.
	majorStr := chi.URLParam(r, "major_version")
	minorStr := chi.URLParam(r, "minor_version")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	authUser, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	major, err := strconv.Atoi(majorStr)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"major_version": "invalid"}})
		return
	}
	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"minor_version": "invalid"}})
		return
	}

	projItem, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	// ACL verify (raises 403 in Python).
	acl := getAttrStringSlice(projItem, "project_acl")
	allowed := false
	for _, u := range acl {
		if u == authUser.Username {
			allowed = true
			break
		}
	}
	if !allowed {
		respond.JSON(w, http.StatusForbidden, map[string]any{"title": "Unauthorized", "description": "You are not authorized to perform this action."})
		return
	}

	docsKey, _, ok := projectDocsKey(documentType)
	if !ok {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"document_type": "invalid"}})
		return
	}

	docsAV := projItem[docsKey]
	newDocsAV, removed := removeDocsByVersionFromDocsAV(docsAV, major, minor)
	if !removed {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"document": "Document version not found"}})
		return
	}

	projItem[docsKey] = newDocsAV
	projItem["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}

	if err := h.projects.PutItem(ctx, projItem); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}

	projectName := getAttrString(projItem, "project_name")
	// Python formatting has some odd spacing; keep close.
	eventData := fmt.Sprintf("Project %s with %s :document type , minor version : %d, major version : %d  deleted", projectName, documentType, minor, major)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:       "DeleteProjectDocument",
		EventCLAGroupID: projectID,
		EventData:       eventData,
		EventSummary:    eventData,
		ContainsPII:     false,
	})

	respond.JSON(w, http.StatusOK, map[string]any{"success": true})
}

func projectDocsKey(documentType string) (docsKey string, noDocsMsg string, ok bool) {
	switch documentType {
	case "individual":
		return "project_individual_documents", "No individual document exists for this project", true
	case "corporate":
		return "project_corporate_documents", "No corporate document exists for this project", true
	default:
		return "", "", false
	}
}

// docTabsFromTemplateTabs converts contract template tab definitions into the DynamoDB
// representation used by the legacy Python DocumentTabModel.
//
// Legacy Python behavior:
//
//	project.post_project_document_template -> document.set_raw_document_tabs(template.get_tabs())
//	-> Document.add_raw_document_tab -> DocumentTabModel
func docTabsFromTemplateTabs(tabs []contracts.TabData) []types.AttributeValue {
	out := make([]types.AttributeValue, 0, len(tabs))
	for _, t := range tabs {
		m := map[string]types.AttributeValue{
			"document_tab_type":      &types.AttributeValueMemberS{Value: t.Type},
			"document_tab_id":        &types.AttributeValueMemberS{Value: t.ID},
			"document_tab_name":      &types.AttributeValueMemberS{Value: t.Name},
			"document_tab_width":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", t.Width)},
			"document_tab_height":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", t.Height)},
			"document_tab_page":      &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", t.Page)},
			"document_tab_is_locked": &types.AttributeValueMemberBOOL{Value: false},
			// Default in the Python model is True.
			"document_tab_is_required":                  &types.AttributeValueMemberBOOL{Value: true},
			"document_tab_anchor_ignore_if_not_present": &types.AttributeValueMemberBOOL{Value: true},
		}

		if strings.TrimSpace(t.AnchorString) != "" {
			m["document_tab_anchor_string"] = &types.AttributeValueMemberS{Value: t.AnchorString}
			m["document_tab_anchor_x_offset"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", t.AnchorXOffset)}
			m["document_tab_anchor_y_offset"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", t.AnchorYOffset)}
			// In legacy templates this is a string ("true"). Persist as a boolean.
			if strings.TrimSpace(t.AnchorIgnoreIfNotPresent) != "" {
				ignore := strings.ToLower(strings.TrimSpace(t.AnchorIgnoreIfNotPresent)) == "true"
				m["document_tab_anchor_ignore_if_not_present"] = &types.AttributeValueMemberBOOL{Value: ignore}
			}
		} else {
			// Absolute positioning tabs.
			m["document_tab_position_x"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", t.PositionX)}
			m["document_tab_position_y"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", t.PositionY)}
		}

		out = append(out, &types.AttributeValueMemberM{Value: m})
	}
	return out
}

func docInt(doc map[string]types.AttributeValue, key string, def int) int {
	av, ok := doc[key]
	if !ok || av == nil {
		return def
	}
	switch v := av.(type) {
	case *types.AttributeValueMemberN:
		if i, err := strconv.Atoi(v.Value); err == nil {
			return i
		}
	case *types.AttributeValueMemberS:
		if i, err := strconv.Atoi(v.Value); err == nil {
			return i
		}
	}
	return def
}

func docString(doc map[string]types.AttributeValue, key string) string {
	av, ok := doc[key]
	if !ok || av == nil {
		return ""
	}
	if s, ok := av.(*types.AttributeValueMemberS); ok {
		return s.Value
	}
	return ""
}

func parsePynamoDateTimeStringLocal(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	// pynamodb's canonical UTCDateTimeAttribute format uses a "+0000" suffix
	// (no colon), so the no-colon layouts must come before the colon ones.
	layouts := []string{
		"2006-01-02T15:04:05.999999-0700",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05.99999",
		"2006-01-02T15:04:05.9999",
		"2006-01-02T15:04:05.999",
		"2006-01-02T15:04:05",
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999-07:00",
		"2006-01-02T15:04:05-07:00",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func latestDocFromDocsAV(docsAV types.AttributeValue) (doc map[string]types.AttributeValue, major int, minor int, ok bool) {
	list, okList := docsAV.(*types.AttributeValueMemberL)
	if !okList {
		return nil, 0, -1, false
	}
	lastMajor := 0
	lastMinor := -1
	var lastDate time.Time
	hasDate := false
	var lastDoc map[string]types.AttributeValue
	for _, el := range list.Value {
		m, okM := el.(*types.AttributeValueMemberM)
		if !okM {
			continue
		}
		curMajor := docInt(m.Value, "document_major_version", 0)
		curMinor := docInt(m.Value, "document_minor_version", -1)
		curDate, curHasDate := parsePynamoDateTimeStringLocal(docString(m.Value, "document_creation_date"))

		if curMajor > lastMajor || (curMajor == lastMajor && curMinor > lastMinor) {
			lastMajor = curMajor
			lastMinor = curMinor
			lastDoc = m.Value
			if curHasDate {
				lastDate = curDate
				hasDate = true
			} else {
				hasDate = false
			}
			continue
		}
		if curMajor == lastMajor && curMinor == lastMinor {
			if hasDate && curHasDate {
				if curDate.After(lastDate) {
					lastDate = curDate
					lastDoc = m.Value
				}
			} else if !hasDate && curHasDate {
				lastDate = curDate
				hasDate = true
				lastDoc = m.Value
			}
		}
	}
	if lastDoc == nil {
		return nil, 0, -1, false
	}
	return lastDoc, lastMajor, lastMinor, true
}

func docByVersionFromDocsAV(docsAV types.AttributeValue, major int, minor int) (map[string]types.AttributeValue, bool) {
	list, okList := docsAV.(*types.AttributeValueMemberL)
	if !okList {
		return nil, false
	}

	var matched map[string]types.AttributeValue
	var matchedDate time.Time
	hasDate := false

	for _, el := range list.Value {
		m, okM := el.(*types.AttributeValueMemberM)
		if !okM {
			continue
		}
		if docInt(m.Value, "document_major_version", 0) != major || docInt(m.Value, "document_minor_version", -1) != minor {
			continue
		}

		curDate, curHasDate := parsePynamoDateTimeStringLocal(docString(m.Value, "document_creation_date"))
		if matched == nil {
			matched = m.Value
			if curHasDate {
				matchedDate = curDate
				hasDate = true
			}
			continue
		}

		if hasDate && curHasDate {
			if curDate.After(matchedDate) {
				matchedDate = curDate
				matched = m.Value
			}
		} else if !hasDate && curHasDate {
			matchedDate = curDate
			hasDate = true
			matched = m.Value
		}
	}

	return matched, matched != nil
}

func lastDocVersionFromDocsAV(docsAV types.AttributeValue) (major int, minor int) {
	// Legacy Python get_last_version returns (0,-1) when no docs exist.
	if _, okList := docsAV.(*types.AttributeValueMemberL); !okList {
		return 0, -1
	}
	_, maj, min, ok := latestDocFromDocsAV(docsAV)
	if !ok {
		return 0, -1
	}
	return maj, min
}

func appendDocToDocsAV(docsAV types.AttributeValue, doc map[string]types.AttributeValue) types.AttributeValue {
	if list, ok := docsAV.(*types.AttributeValueMemberL); ok {
		newList := make([]types.AttributeValue, 0, len(list.Value)+1)
		newList = append(newList, list.Value...)
		newList = append(newList, &types.AttributeValueMemberM{Value: doc})
		return &types.AttributeValueMemberL{Value: newList}
	}
	return &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberM{Value: doc}}}
}

func removeDocsByVersionFromDocsAV(docsAV types.AttributeValue, major int, minor int) (newDocs types.AttributeValue, removed bool) {
	list, okList := docsAV.(*types.AttributeValueMemberL)
	if !okList {
		return &types.AttributeValueMemberL{Value: []types.AttributeValue{}}, false
	}
	newList := make([]types.AttributeValue, 0, len(list.Value))
	removedAny := false
	for _, el := range list.Value {
		m, okM := el.(*types.AttributeValueMemberM)
		if !okM {
			newList = append(newList, el)
			continue
		}
		curMajor := docInt(m.Value, "document_major_version", 0)
		curMinor := docInt(m.Value, "document_minor_version", 0)
		if curMajor == major && curMinor == minor {
			removedAny = true
			continue
		}
		newList = append(newList, el)
	}
	return &types.AttributeValueMemberL{Value: newList}, removedAny
}

func (h *Handlers) fetchProjectDocumentPDF(ctx context.Context, doc map[string]types.AttributeValue) ([]byte, error) {
	// 1) If document_s3_url is set, fetch it.
	if s3URL := docString(doc, "document_s3_url"); s3URL != "" {
		resp, err := h.httpClient.Get(s3URL)
		if err != nil {
			return nil, fmt.Errorf("fetch document_s3_url: %w", err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read document_s3_url body: %w", err)
		}
		return b, nil
	}

	ct := docString(doc, "document_content_type")
	// 2) url+pdf (deprecated) - document_content is a URL.
	if strings.HasPrefix(ct, "url+") {
		u := docString(doc, "document_content")
		if u == "" {
			return nil, fmt.Errorf("url+ document has empty document_content")
		}
		resp, err := h.httpClient.Get(u)
		if err != nil {
			return nil, fmt.Errorf("fetch url+ document_content: %w", err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read url+ document_content body: %w", err)
		}
		return b, nil
	}

	// 3) storage+pdf - retrieve from S3 storage bucket by document_file_id.
	if strings.HasPrefix(ct, "storage+") {
		bucket := os.Getenv("CLA_SIGNATURE_FILES_BUCKET")
		if bucket == "" {
			return nil, fmt.Errorf("CLA_SIGNATURE_FILES_BUCKET not set")
		}
		key := docString(doc, "document_file_id")
		if key == "" {
			return nil, fmt.Errorf("storage+ document has empty document_file_id")
		}
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(h.region))
		if err != nil {
			return nil, fmt.Errorf("load aws config: %w", err)
		}
		if strings.ToLower(os.Getenv("STAGE")) == "local" {
			cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
				if service == s3.ServiceID {
					return aws.Endpoint{URL: "http://localhost:8001", SigningRegion: h.region, HostnameImmutable: true}, nil
				}
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			})
		}
		client := s3.NewFromConfig(cfg, func(o *s3.Options) {
			if strings.ToLower(os.Getenv("STAGE")) == "local" {
				o.UsePathStyle = true
			}
		})
		out, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		if err != nil {
			return nil, fmt.Errorf("s3 get_object: %w", err)
		}
		defer out.Body.Close()
		b, err := io.ReadAll(out.Body)
		if err != nil {
			return nil, fmt.Errorf("read s3 body: %w", err)
		}
		return b, nil
	}

	// 4) inline pdf (legacy) - use document_content as bytes.
	if av, ok := doc["document_content"]; ok && av != nil {
		switch v := av.(type) {
		case *types.AttributeValueMemberB:
			return v.Value, nil
		case *types.AttributeValueMemberS:
			// Best effort: attempt base64 decode, else treat as raw bytes.
			if decoded, err := base64.StdEncoding.DecodeString(v.Value); err == nil {
				return decoded, nil
			}
			return []byte(v.Value), nil
		}
	}

	return nil, fmt.Errorf("document has no retrievable content")
}

// POST /v2/request-individual-signature
// Python: cla/routes.py:1745 request_individual_signature()
// Calls: cla.controllers.signing.request_individual_signature

func (h *Handlers) RequestIndividualSignatureV2(w http.ResponseWriter, r *http.Request) {
	// Minimal-effort migration strategy: delegate the DocuSign heavy lifting to the v4 Go backend.
	// This eliminates the legacy Python dependency for the signing request while keeping the
	// public v2 URL stable.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"body": err.Error()}})
		return
	}

	// Parity: legacy Hug validates project_id and user_id as required UUIDs.
	// return_url_type is optional in the route and, if missing/unsupported, the Python controller
	// falls off the end and Hug returns null/200.
	var payload map[string]any
	formVals := url.Values{}
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if len(body) > 0 {
		if strings.Contains(ct, "application/json") {
			_ = json.Unmarshal(body, &payload) // Best-effort; missing/invalid JSON behaves like empty.
		} else if vals, perr := url.ParseQuery(string(body)); perr == nil {
			formVals = vals
		}
	}
	getString := func(key string) string {
		if payload != nil {
			if v, ok := payload[key]; ok {
				return strings.TrimSpace(fmt.Sprint(v))
			}
		}
		if vals, ok := formVals[key]; ok && len(vals) > 0 {
			return strings.TrimSpace(vals[0])
		}
		return strings.TrimSpace(r.URL.Query().Get(key))
	}
	projectID := getString("project_id")
	userID := getString("user_id")
	returnURLType := getString("return_url_type")
	returnURL := getString("return_url")
	missing := map[string]any{}
	if projectID == "" {
		missing["project_id"] = "missing"
	}
	if userID == "" {
		missing["user_id"] = "missing"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}
	if _, err := uuid.Parse(projectID); err != nil {
		missing["project_id"] = "invalid uuid"
	}
	if _, err := uuid.Parse(userID); err != nil {
		missing["user_id"] = "invalid uuid"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}

	// Legacy Python controller behavior: return_url_type is optional in the route.
	// If it is missing or unsupported, the controller falls off the end and Hug returns null/200.
	switch strings.ToLower(returnURLType) {
	case "github", "gitlab", "gerrit":
	default:
		respond.JSON(w, http.StatusOK, nil)
		return
	}
	if strings.TrimSpace(returnURL) == "" && h.kv != nil {
		metadata, ok, lookupErr := h.loadActiveSignatureMetadata(r.Context(), userID)
		if lookupErr != nil {
			logging.Warnf("active signature metadata lookup failed for individual signature user=%s err=%v", userID, lookupErr)
		} else if ok {
			if ru, rerr := h.computeReturnURLFromActiveSignatureMetadata(r.Context(), metadata); rerr != nil {
				logging.Warnf("active signature return URL computation failed for individual signature user=%s err=%v", userID, rerr)
			} else if strings.TrimSpace(ru) != "" {
				returnURL = strings.TrimSpace(ru)
			}
		}
	}

	forwardPayload := map[string]any{
		"project_id":      projectID,
		"user_id":         userID,
		"return_url_type": returnURLType,
	}
	if strings.TrimSpace(returnURL) != "" {
		forwardPayload["return_url"] = strings.TrimSpace(returnURL)
	}
	forwardBody, err := json.Marshal(forwardPayload)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"body": err.Error()}})
		return
	}
	path := "/request-individual-signature"
	hdrs := headerCloneForV4(r.Header)
	hdrs.Set("Content-Type", "application/json")
	status, hdr, respBody, err := h.doRequestToV4(r.Context(), http.MethodPost, path, hdrs, forwardBody)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"v4": err.Error()}})
		return
	}
	if translated, ok := translateLegacyIndividualSignatureV4Error(returnURLType, status, respBody); ok {
		respBody = translated
	}
	copyV4ResponseHeaders(w, hdr)
	// Legacy Python Hug frequently returned HTTP 200 with an "errors" payload. Preserve
	// that behavior by normalizing v4 non-2xx to 200 while passing through the body.
	if status >= 400 {
		logging.Warnf("v4 request-individual-signature returned %d: %s", status, string(respBody))
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(respBody)
}

// POST /v1/request-corporate-signature
// Python: cla/routes.py:1779 request_corporate_signature()
// Calls: cla.controllers.signing.request_corporate_signature

func (h *Handlers) RequestCorporateSignatureV1(w http.ResponseWriter, r *http.Request) {
	// Legacy parity: this route requires check_auth and Hug validates project_id/company_id as UUIDs.
	_, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	// Minimal-effort migration strategy: delegate the DocuSign heavy lifting to the v4 Go backend.
	// Normalize flexible legacy input (JSON / form / query params) into a canonical JSON body.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"body": err.Error()}})
		return
	}

	var payload map[string]any
	formVals := url.Values{}
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if len(body) > 0 {
		if strings.Contains(ct, "application/json") {
			_ = json.Unmarshal(body, &payload) // best-effort; missing/invalid behaves like empty for validation
		} else if vals, perr := url.ParseQuery(string(body)); perr == nil {
			formVals = vals
		}
	}
	getString := func(key string) string {
		if payload != nil {
			if v, ok := payload[key]; ok {
				return strings.TrimSpace(fmt.Sprint(v))
			}
		}
		if vals, ok := formVals[key]; ok && len(vals) > 0 {
			return strings.TrimSpace(vals[0])
		}
		return strings.TrimSpace(r.URL.Query().Get(key))
	}
	getBool := func(key string) (bool, bool) {
		if payload != nil {
			if v, ok := payload[key]; ok {
				switch vv := v.(type) {
				case bool:
					return vv, true
				case string:
					s := strings.TrimSpace(strings.ToLower(vv))
					if s == "true" || s == "1" || s == "yes" || s == "on" {
						return true, true
					}
					if s == "false" || s == "0" || s == "no" || s == "off" || s == "" {
						return false, true
					}
				case float64:
					return vv != 0, true
				}
			}
		}
		if vals, ok := formVals[key]; ok && len(vals) > 0 {
			s := strings.TrimSpace(strings.ToLower(vals[0]))
			if s == "true" || s == "1" || s == "yes" || s == "on" {
				return true, true
			}
			if s == "false" || s == "0" || s == "no" || s == "off" || s == "" {
				return false, true
			}
		}
		if qv := strings.TrimSpace(r.URL.Query().Get(key)); qv != "" {
			s := strings.ToLower(qv)
			if s == "true" || s == "1" || s == "yes" || s == "on" {
				return true, true
			}
			if s == "false" || s == "0" || s == "no" || s == "off" {
				return false, true
			}
		}
		return false, false
	}
	projectID := getString("project_id")
	companyID := getString("company_id")
	missing := map[string]any{}
	if projectID == "" {
		missing["project_id"] = "missing"
	}
	if companyID == "" {
		missing["company_id"] = "missing"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}
	if _, err := uuid.Parse(projectID); err != nil {
		missing["project_id"] = "invalid uuid"
	}
	if _, err := uuid.Parse(companyID); err != nil {
		missing["company_id"] = "invalid uuid"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}
	projectItem, projectFound, err := h.projects.GetByID(r.Context(), projectID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}
	if !projectFound {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}
	projectSFID := strings.TrimSpace(getAttrString(projectItem, "project_external_id"))
	if projectSFID == "" {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": "Project external ID not found"}})
		return
	}

	companyItem, companyFound, err := h.companies.GetByID(r.Context(), companyID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"company_id": err.Error()}})
		return
	}
	if !companyFound {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"company_id": "Company not found"}})
		return
	}
	companySFID := strings.TrimSpace(getAttrString(companyItem, "company_external_id"))
	if companySFID == "" {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"company_id": "Company external ID not found"}})
		return
	}

	forwardPayload := map[string]any{
		"project_sfid": projectSFID,
		"company_sfid": companySFID,
	}
	signingEntityName := getString("signing_entity_name")
	if signingEntityName == "" {
		signingEntityName = strings.TrimSpace(getAttrString(companyItem, "signing_entity_name"))
		if signingEntityName == "" {
			signingEntityName = strings.TrimSpace(getAttrString(companyItem, "company_name"))
		}
	}
	if signingEntityName != "" {
		forwardPayload["signing_entity_name"] = signingEntityName
	}
	for _, key := range []string{ /*"signing_entity_name", */ "authority_name", "authority_email", "return_url_type", "return_url"} {
		if v := getString(key); v != "" {
			forwardPayload[key] = v
		}
	}
	if b, ok := getBool("send_as_email"); ok {
		forwardPayload["send_as_email"] = b
	}
	forwardBody, err := json.Marshal(forwardPayload)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"body": err.Error()}})
		return
	}

	path := "/request-corporate-signature"
	hdrs := headerCloneForV4(r.Header)
	hdrs.Set("Content-Type", "application/json")
	status, hdr, respBody, err := h.doRequestToV4(r.Context(), http.MethodPost, path, hdrs, forwardBody)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"v4": err.Error()}})
		return
	}
	if status < 400 {
		var out map[string]any
		if err := json.Unmarshal(respBody, &out); err == nil && out != nil {
			out["project_id"] = projectID
			out["company_id"] = companyID
			if b, merr := json.Marshal(out); merr == nil {
				respBody = b
			}
		}
	}
	copyV4ResponseHeaders(w, hdr)
	// Normalize non-2xx into 200 to match legacy Hug behavior.
	if status >= 400 {
		logging.Warnf("v4 request-corporate-signature returned %d: %s", status, string(respBody))
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(respBody)
}

// POST /v2/request-employee-signature
// Python: cla/routes.py:1839 request_employee_signature()
// Calls: cla.controllers.signing.request_employee_signature

type employeeSignatureRequestV2 struct {
	ProjectID     string `json:"project_id"`
	CompanyID     string `json:"company_id"`
	UserID        string `json:"user_id"`
	ReturnURLType string `json:"return_url_type"`
	ReturnURL     string `json:"return_url"`
}

func parseFlexibleParams(r *http.Request) (map[string]any, error) {
	body := map[string]any{}
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.Contains(ct, "application/json") {
		if err := decodeJSONBody(r, &body); err != nil {
			if !errors.Is(err, io.EOF) {
				return nil, err
			}
		}
	}
	_ = r.ParseForm()
	return body, nil
}

func flexibleStringParam(r *http.Request, body map[string]any, key string) (string, bool) {
	if v, ok := body[key]; ok {
		if v == nil {
			return "", true
		}
		return strings.TrimSpace(fmt.Sprint(v)), true
	}
	if r == nil {
		return "", false
	}
	_ = r.ParseForm()
	if vals, ok := r.PostForm[key]; ok {
		if len(vals) == 0 {
			return "", true
		}
		return strings.TrimSpace(vals[0]), true
	}
	if vals, ok := r.URL.Query()[key]; ok {
		if len(vals) == 0 {
			return "", true
		}
		return strings.TrimSpace(vals[0]), true
	}
	return "", false
}

func flexibleBoolParam(r *http.Request, body map[string]any, key string) (bool, bool, error) {
	if v, ok := body[key]; ok {
		if v == nil {
			return false, true, nil
		}
		b, err := smartBool(v)
		return b, true, err
	}
	if r == nil {
		return false, false, nil
	}
	_ = r.ParseForm()
	if vals, ok := r.PostForm[key]; ok {
		if len(vals) == 0 {
			return false, true, nil
		}
		b, err := smartBool(vals[0])
		return b, true, err
	}
	if vals, ok := r.URL.Query()[key]; ok {
		if len(vals) == 0 {
			return false, true, nil
		}
		b, err := smartBool(vals[0])
		return b, true, err
	}
	return false, false, nil
}

func parseEmployeeSignatureRequestV2(r *http.Request) employeeSignatureRequestV2 {
	body := map[string]any{}
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.Contains(ct, "application/json") {
		if err := decodeJSONBody(r, &body); err != nil {
			if !errors.Is(err, io.EOF) {
				// Ignore invalid body here; legacy Hug would treat missing/invalid as empty for these endpoints.
				body = map[string]any{}
			}
		}
	}
	_ = r.ParseForm()
	getString := func(key string) string {
		if v, ok := flexibleStringParam(r, body, key); ok {
			return v
		}
		return ""
	}
	return employeeSignatureRequestV2{
		ProjectID:     getString("project_id"),
		CompanyID:     getString("company_id"),
		UserID:        getString("user_id"),
		ReturnURLType: getString("return_url_type"),
		ReturnURL:     getString("return_url"),
	}
}

func uniqueLowerTrimmedStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func emailDomainsFromEmails(emails []string) []string {
	domains := make([]string, 0, len(emails))
	for _, e := range emails {
		e = strings.ToLower(strings.TrimSpace(e))
		at := strings.LastIndex(e, "@")
		if at < 0 || at == len(e)-1 {
			continue
		}
		d := strings.TrimSpace(e[at+1:])
		if d != "" {
			domains = append(domains, d)
		}
	}
	return uniqueLowerTrimmedStrings(domains)
}

func domainPatternMatches(pattern, domain string) bool {
	p := strings.ToLower(strings.TrimSpace(pattern))
	d := strings.ToLower(strings.TrimSpace(domain))
	if p == "" || d == "" {
		return false
	}

	// Common patterns seen in legacy allowlists.
	// - "example.com" (exact)
	// - "*.example.com" (suffix)
	// - ".example.com" (suffix)
	// - "*example.com" (suffix)
	if strings.HasPrefix(p, "*.") {
		suf := strings.TrimPrefix(p, "*.")
		if suf == "" {
			return false
		}
		return d == suf || strings.HasSuffix(d, "."+suf)
	}
	if strings.HasPrefix(p, ".") {
		suf := strings.TrimPrefix(p, ".")
		if suf == "" {
			return false
		}
		return d == suf || strings.HasSuffix(d, "."+suf)
	}
	if strings.HasPrefix(p, "*") {
		suf := strings.TrimPrefix(p, "*")
		suf = strings.TrimPrefix(suf, ".")
		if suf == "" {
			return false
		}
		return strings.HasSuffix(d, suf)
	}
	return d == p
}

func getSigListAllowingBothNames(sig map[string]types.AttributeValue, whitelistKey, allowlistKey string) []string {
	if sig == nil {
		return nil
	}
	if v := getAttrStringSlice(sig, whitelistKey); len(v) > 0 {
		return v
	}
	return getAttrStringSlice(sig, allowlistKey)
}

func getUserEmailsForApproval(user map[string]types.AttributeValue) []string {
	emails := make([]string, 0, 4)
	if v := strings.TrimSpace(getAttrString(user, "lf_email")); v != "" {
		emails = append(emails, v)
	}
	emails = append(emails, getAttrStringSlice(user, "user_emails")...)
	return uniqueLowerTrimmedStrings(emails)
}

type githubOrg struct {
	Login string `json:"login"`
}

func githubAPIBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("GITHUB_API_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://api.github.com"
}

func (h *Handlers) githubListUserOrgs(ctx context.Context, username string) ([]string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return []string{}, nil
	}
	endpoint := githubAPIBaseURL() + "/users/" + url.PathEscape(username) + "/orgs?per_page=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	// req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cla-backend-legacy")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_OAUTH_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "token "+tok)
		// req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, fmt.Errorf("github orgs request failed: status=%d body=%s", resp.StatusCode, string(b))
	}
	var payload []githubOrg
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(payload))
	for _, o := range payload {
		if strings.TrimSpace(o.Login) != "" {
			out = append(out, o.Login)
		}
	}
	return uniqueLowerTrimmedStrings(out), nil
}

// githubLookupUsernameByID mirrors cla.utils.lookup_user_github_username().
//
// It is used by the legacy employee-signature precheck to backfill missing
// GitHub username data when only the numeric GitHub ID is present.
func (h *Handlers) githubLookupUsernameByID(ctx context.Context, githubID string) (string, error) {
	githubID = strings.TrimSpace(githubID)
	if githubID == "" {
		return "", nil
	}
	endpoint := githubAPIBaseURL() + "/user/" + url.PathEscape(githubID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cla-backend-legacy")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_OAUTH_TOKEN")); tok != "" {
		// Python uses Bearer.
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		logging.Warnf("github lookup username failed: github_id=%s status=%d body=%s", githubID, resp.StatusCode, string(b))
		return "", nil
	}
	var payload struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Login), nil
}

// githubLookupIDByUsername mirrors cla.utils.lookup_user_github_id().
func (h *Handlers) githubLookupIDByUsername(ctx context.Context, username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", nil
	}
	endpoint := githubAPIBaseURL() + "/users/" + url.PathEscape(username)
	if !strings.Contains(endpoint, "api.github.com") {
		// Defensive: ensure we're calling the GitHub API base.
		endpoint = "https://api.github.com/users/" + url.PathEscape(username)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cla-backend-legacy")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_OAUTH_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		logging.Warnf("github lookup id failed: username=%s status=%d body=%s", username, resp.StatusCode, string(b))
		return "", nil
	}
	var payload struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.ID == 0 {
		return "", nil
	}
	return strconv.FormatInt(payload.ID, 10), nil
}

// githubIsBot mirrors cla.controllers.signature.is_github_bot().
//
// It queries the GitHub public user endpoint and returns true if the returned "type" is "Bot".
func (h *Handlers) githubIsBot(ctx context.Context, username string) bool {
	fn := "githubIsBot"
	username = strings.TrimSpace(username)
	if username == "" {
		return false
	}

	endpoint := githubAPIBaseURL() + "/users/" + url.PathEscape(username)
	if !strings.Contains(endpoint, "api.github.com") {
		// Defensive: ensure we're calling the GitHub API base.
		endpoint = "https://api.github.com/users/" + url.PathEscape(username)
	}
	// GitHub API expects a user-agent.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		logging.Warnf("%s: build request failed: username=%s err=%v", fn, username, err)
		return false
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cla-backend-legacy")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_OAUTH_TOKEN")); tok != "" {
		// Python uses unauthenticated requests, but supporting a token reduces rate limiting.
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		logging.Warnf("%s: request failed: username=%s err=%v", fn, username, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var payload struct {
			Type string `json:"type"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			logging.Warnf("%s: decode failed: username=%s err=%v", fn, username, err)
			return false
		}
		return strings.ToLower(strings.TrimSpace(payload.Type)) == "bot"
	}
	if resp.StatusCode == http.StatusNotFound {
		// Python returns False on 404.
		_, _ = io.Copy(io.Discard, resp.Body)
		return false
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	logging.Warnf("%s: non-200 lookup: username=%s status=%d body=%s", fn, username, resp.StatusCode, string(b))
	return false
}

// githubLookupUserIDInt mirrors cla.controllers.signature.lookup_github_user().
//
// Returns 0 when user is not found or on error.
func (h *Handlers) githubLookupUserIDInt(ctx context.Context, username string) int64 {
	fn := "githubLookupUserIDInt"
	username = strings.TrimSpace(username)
	if username == "" {
		return 0
	}

	endpoint := githubAPIBaseURL() + "/users/" + url.PathEscape(username)
	if !strings.Contains(endpoint, "api.github.com") {
		endpoint = "https://api.github.com/users/" + url.PathEscape(username)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		logging.Warnf("%s: build request failed: username=%s err=%v", fn, username, err)
		return 0
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cla-backend-legacy")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_OAUTH_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		logging.Warnf("%s: request failed: username=%s err=%v", fn, username, err)
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var payload struct {
			ID int64 `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			logging.Warnf("%s: decode failed: username=%s err=%v", fn, username, err)
			return 0
		}
		return payload.ID
	}
	if resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, resp.Body)
		return 0
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	logging.Warnf("%s: non-200 lookup: username=%s status=%d body=%s", fn, username, resp.StatusCode, string(b))
	return 0
}

func zuluStamp(t time.Time) string {
	// Python uses datetime.utcnow().strftime("%Y%m%dT%H%M%SZ")
	return t.UTC().Format("20060102T150405Z")
}

// handleGithubBotsFromAllowlistBestEffort mirrors the legacy Python allowlist bot behavior:
//   - For each GitHub allowlist entry, call is_github_bot(username)
//   - For bot users, ensure a User record exists for the CCLA company
//   - Ensure an employee signature exists for the bot user/company/project
//
// Any failures are logged and do NOT fail the main signature update call.
func (h *Handlers) handleGithubBotsFromAllowlistBestEffort(ctx context.Context, companySignature map[string]types.AttributeValue, githubAllowlist []string) {
	fn := "handleGithubBotsFromAllowlistBestEffort"
	if h.users == nil || h.signatures == nil {
		return
	}
	if companySignature == nil {
		return
	}
	if strings.ToLower(strings.TrimSpace(getAttrString(companySignature, "signature_reference_type"))) != "company" {
		return
	}
	if strings.ToLower(strings.TrimSpace(getAttrString(companySignature, "signature_type"))) != "ccla" {
		return
	}
	projectID := strings.TrimSpace(getAttrString(companySignature, "signature_project_id"))
	companyID := strings.TrimSpace(getAttrString(companySignature, "signature_reference_id"))
	if projectID == "" || companyID == "" {
		return
	}

	// Load names for note formatting (best-effort).
	projectName := projectID
	if h.projects != nil {
		if p, found, err := h.projects.GetByID(ctx, projectID); err == nil && found {
			if n := strings.TrimSpace(getAttrString(p, "project_name")); n != "" {
				projectName = n
			}
		}
	}
	companyName := companyID
	if h.companies != nil {
		if c, found, err := h.companies.GetByID(ctx, companyID); err == nil && found {
			if n := strings.TrimSpace(getAttrString(c, "company_name")); n != "" {
				companyName = n
			}
		}
	}

	// Collect bot usernames.
	botNames := make([]string, 0, 4)
	for _, u := range githubAllowlist {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if h.githubIsBot(ctx, u) {
			botNames = append(botNames, u)
		}
	}
	if len(botNames) == 0 {
		return
	}

	// Preload existing employee signatures for this project/company to avoid repeated scans.
	existingEmployee := map[string]bool{}
	items, err := h.signatures.QueryByProjectID(ctx, projectID)
	if err != nil {
		logging.Warnf("%s: query project signatures failed: project_id=%s err=%v", fn, projectID, err)
	} else {
		for _, it := range items {
			if strings.ToLower(strings.TrimSpace(getAttrString(it, "signature_reference_type"))) != "user" {
				continue
			}
			if strings.ToLower(strings.TrimSpace(getAttrString(it, "signature_type"))) != "cla" {
				continue
			}
			if strings.TrimSpace(getAttrString(it, "signature_user_ccla_company_id")) != companyID {
				continue
			}
			if !getAttrBool(it, "signature_signed") || !getAttrBool(it, "signature_approved") {
				continue
			}
			rid := strings.TrimSpace(getAttrString(it, "signature_reference_id"))
			if rid != "" {
				existingEmployee[rid] = true
			}
		}
	}

	docMaj := getAttrInt(companySignature, "signature_document_major_version")
	docMin := getAttrInt(companySignature, "signature_document_minor_version")
	now := time.Now().UTC()

	for _, botName := range botNames {
		// 1) Ensure bot user exists for this company.
		botUser, err := h.ensureBotUserBestEffort(ctx, botName, companyID, projectName, companyName, now)
		if err != nil {
			logging.Warnf("%s: ensure bot user failed: bot=%s company_id=%s err=%v", fn, botName, companyID, err)
			continue
		}
		if botUser == nil {
			continue
		}
		botUserID := strings.TrimSpace(getAttrString(botUser, "user_id"))
		if botUserID == "" {
			continue
		}

		// 2) Ensure employee signature exists.
		if existingEmployee[botUserID] {
			continue
		}
		if err := h.createBotEmployeeSignatureBestEffort(ctx, botName, botUserID, projectID, companyID, projectName, companyName, docMaj, docMin, now); err != nil {
			logging.Warnf("%s: create bot employee signature failed: bot=%s user_id=%s project_id=%s company_id=%s err=%v", fn, botName, botUserID, projectID, companyID, err)
			continue
		}
		existingEmployee[botUserID] = true
	}
}

func (h *Handlers) ensureBotUserBestEffort(ctx context.Context, botName, companyID, projectName, companyName string, now time.Time) (map[string]types.AttributeValue, error) {
	fn := "ensureBotUserBestEffort"
	if h.users == nil {
		return nil, nil
	}
	botName = strings.TrimSpace(botName)
	if botName == "" {
		return nil, nil
	}

	// Find existing bot user records.
	users, err := h.users.QueryByGitHubUsername(ctx, botName)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if strings.TrimSpace(getAttrString(u, "user_company_id")) == companyID {
			return u, nil
		}
	}

	// Need to create a new user record.
	ghID := h.githubLookupUserIDInt(ctx, botName)
	if ghID == 0 {
		logging.Warnf("%s: unable to create bot user: %s - unable to lookup name in GitHub", fn, botName)
		return nil, nil
	}
	userID := uuid.New().String()
	note := fmt.Sprintf("%s Added as part of %s, approval list by %s", zuluStamp(now), projectName, companyName)
	item := map[string]types.AttributeValue{
		"user_id":         &types.AttributeValueMemberS{Value: userID},
		"user_name":       &types.AttributeValueMemberS{Value: botName},
		"user_company_id": &types.AttributeValueMemberS{Value: companyID},
		"note":            &types.AttributeValueMemberS{Value: note},
		// Legacy Python wrapper sets date_modified on save() even though it is not part of UserModel.
		"date_modified": &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
	}
	if err := h.users.PutItem(ctx, item); err != nil {
		return nil, err
	}
	logging.Debugf("%s: created bot user: %s company_id=%s github_id=%d", fn, botName, companyID, ghID)
	return item, nil
}

func (h *Handlers) createBotEmployeeSignatureBestEffort(ctx context.Context, botName, botUserID, projectID, companyID, projectName, companyName string, docMaj, docMin int, now time.Time) error {
	fn := "createBotEmployeeSignatureBestEffort"
	if h.signatures == nil {
		return nil
	}
	// Create a new employee signature.
	sigID := uuid.New().String()
	note := fmt.Sprintf("%s Added as part of %s, approval list by %s", zuluStamp(now), projectName, companyName)

	item := map[string]types.AttributeValue{
		"signature_id":                   &types.AttributeValueMemberS{Value: sigID},
		"signature_project_id":           &types.AttributeValueMemberS{Value: projectID},
		"signature_reference_id":         &types.AttributeValueMemberS{Value: botUserID},
		"signature_reference_type":       &types.AttributeValueMemberS{Value: "user"},
		"signature_type":                 &types.AttributeValueMemberS{Value: "cla"},
		"signature_signed":               &types.AttributeValueMemberBOOL{Value: true},
		"signature_approved":             &types.AttributeValueMemberBOOL{Value: true},
		"signature_embargo_acked":        &types.AttributeValueMemberBOOL{Value: true},
		"signature_user_ccla_company_id": &types.AttributeValueMemberS{Value: companyID},
		"note":                           &types.AttributeValueMemberS{Value: note},
		// Legacy Python wrapper sets date_modified on save() even though it is not part of SignatureModel.
		"date_modified": &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
	}
	if docMaj > 0 {
		item["signature_document_major_version"] = &types.AttributeValueMemberN{Value: strconv.Itoa(docMaj)}
	}
	if docMin > 0 {
		item["signature_document_minor_version"] = &types.AttributeValueMemberN{Value: strconv.Itoa(docMin)}
	}
	if err := h.signatures.PutItem(ctx, item); err != nil {
		return err
	}
	logging.Debugf("%s: created bot employee signature: bot=%s user_id=%s project_id=%s company_id=%s", fn, botName, botUserID, projectID, companyID)
	return nil
}

// findGitlabOrgByGroupURL mirrors GitlabOrg.search_organization_by_group_url().
func (h *Handlers) findGitlabOrgByGroupURL(ctx context.Context, groupURL string) (map[string]types.AttributeValue, bool, error) {
	if h.gitlabOrgs == nil {
		return nil, false, nil
	}
	groupURL = strings.TrimSpace(groupURL)
	if groupURL == "" {
		return nil, false, nil
	}
	it, found, err := h.gitlabOrgs.FindByOrganizationURL(ctx, groupURL)
	if err != nil {
		return nil, false, err
	}
	if found {
		return it, true, nil
	}
	// Python tries to add "groups/" when the allowlist contains https://gitlab.com/<group>
	// instead of https://gitlab.com/groups/<group>
	if strings.HasPrefix(groupURL, "https://gitlab.com/") && !strings.Contains(groupURL, "/groups/") {
		alt := strings.Replace(groupURL, "https://gitlab.com/", "https://gitlab.com/groups/", 1)
		it, found, err = h.gitlabOrgs.FindByOrganizationURL(ctx, alt)
		if err != nil {
			return nil, false, err
		}
		if found {
			return it, true, nil
		}
	}
	return nil, false, nil
}

// gitlabListOrgMemberUsernames calls the v4 backend to list GitLab group members.
//
// Python: cla.utils.lookup_gitlab_org_members() -> GET {PLATFORM_GATEWAY_URL}/cla-service/v4/gitlab/group/{org_id}/members
func (h *Handlers) gitlabListOrgMemberUsernames(ctx context.Context, organizationID string) ([]string, error) {
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return []string{}, nil
	}
	hdr := http.Header{}
	hdr.Set("Accept", "application/json")
	path := "/gitlab/group/" + url.PathEscape(organizationID) + "/members"
	status, _, body, err := h.doRequestToV4(ctx, http.MethodGet, path, hdr, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status > 299 {
		// Python returns a malformed error type here (set of string) and the caller is buggy.
		// For safety and operational stability we treat it as an empty member list.
		logging.Warnf("gitlab group members lookup failed: organization_id=%s status=%d body=%s", organizationID, status, string(body))
		return []string{}, nil
	}

	// v4 currently returns a JSON array of members.
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		out := make([]string, 0, len(arr))
		for _, m := range arr {
			if u, ok := m["username"].(string); ok {
				u = strings.TrimSpace(u)
				if u != "" {
					out = append(out, u)
				}
			}
		}
		return out, nil
	}

	// Defensive fallback: sometimes APIs wrap in {"data": [...]}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err == nil {
		if data, ok := obj["data"].([]any); ok {
			out := make([]string, 0, len(data))
			for _, it := range data {
				m, ok := it.(map[string]any)
				if !ok {
					continue
				}
				if u, ok := m["username"].(string); ok {
					u = strings.TrimSpace(u)
					if u != "" {
						out = append(out, u)
					}
				}
			}
			return out, nil
		}
	}

	return nil, fmt.Errorf("unexpected gitlab members payload")
}

func (h *Handlers) isUserApprovedByCCLASignature(ctx context.Context, user map[string]types.AttributeValue, cclaSig map[string]types.AttributeValue) (bool, error) {
	// Python SignatureModel: email_whitelist/domain_whitelist/github_whitelist/github_org_whitelist
	// plus employee-signature additions: gitlab_username_approval_list/gitlab_org_approval_list
	emailWL := uniqueLowerTrimmedStrings(getSigListAllowingBothNames(cclaSig, "email_whitelist", "email_allowlist"))
	domainWL := uniqueLowerTrimmedStrings(getSigListAllowingBothNames(cclaSig, "domain_whitelist", "domain_allowlist"))
	githubWL := uniqueLowerTrimmedStrings(getSigListAllowingBothNames(cclaSig, "github_whitelist", "github_allowlist"))
	githubOrgWL := uniqueLowerTrimmedStrings(getSigListAllowingBothNames(cclaSig, "github_org_whitelist", "github_org_allowlist"))
	gitlabWL := uniqueLowerTrimmedStrings(getSigListAllowingBothNames(cclaSig, "gitlab_username_approval_list", "gitlab_username_allowlist"))
	gitlabOrgWL := uniqueLowerTrimmedStrings(getSigListAllowingBothNames(cclaSig, "gitlab_org_approval_list", "gitlab_org_allowlist"))

	anyListConfigured := len(emailWL) > 0 || len(domainWL) > 0 || len(githubWL) > 0 || len(githubOrgWL) > 0 || len(gitlabWL) > 0 || len(gitlabOrgWL) > 0
	if !anyListConfigured {
		// Confirmed against Python: if there are no allowlists configured, the user is not approved.
		return false, nil
	}

	// 1) email allowlist
	userEmails := getUserEmailsForApproval(user)
	if len(emailWL) > 0 {
		for _, e := range userEmails {
			if stringSliceContainsExact(emailWL, e) {
				return true, nil
			}
		}
	}

	// 2) domain allowlist (supports *.example.org)
	userDomains := emailDomainsFromEmails(userEmails)
	if len(domainWL) > 0 {
		for _, d := range userDomains {
			for _, p := range domainWL {
				if domainPatternMatches(p, d) {
					return true, nil
				}
			}
		}
	}

	githubUsername := strings.ToLower(strings.TrimSpace(getAttrString(user, "user_github_username")))

	// 3) github username allowlist
	if githubUsername != "" && len(githubWL) > 0 {
		if stringSliceContainsExact(githubWL, githubUsername) {
			return true, nil
		}
	}

	// 4) github org allowlist
	if githubUsername != "" && len(githubOrgWL) > 0 {
		orgs, err := h.githubListUserOrgs(ctx, githubUsername)
		if err != nil {
			logging.Warnf("github org allowlist check failed: github_username=%s err=%v", githubUsername, err)
			return false, nil
		}
		for _, o := range orgs {
			if stringSliceContainsExact(githubOrgWL, o) {
				return true, nil
			}
		}
	}

	// 5) gitlab username allowlist
	gitlabUsername := strings.ToLower(strings.TrimSpace(getAttrString(user, "user_gitlab_username")))
	if gitlabUsername != "" && len(gitlabWL) > 0 {
		if stringSliceContainsExact(gitlabWL, gitlabUsername) {
			return true, nil
		}
	}

	// 6) gitlab org allowlist
	if gitlabUsername != "" && len(gitlabOrgWL) > 0 {
		for _, glName := range gitlabOrgWL {
			glOrg, found, err := h.findGitlabOrgByGroupURL(ctx, glName)
			if err != nil {
				logging.Warnf("gitlab org lookup failed: group_url=%s err=%v", glName, err)
				continue
			}
			if !found {
				logging.Debugf("gitlab org not found for group_url=%s", glName)
				continue
			}
			orgID := strings.TrimSpace(getAttrString(glOrg, "organization_id"))
			if orgID == "" {
				continue
			}
			members, err := h.gitlabListOrgMemberUsernames(ctx, orgID)
			if err != nil {
				logging.Warnf("gitlab org members lookup failed: organization_id=%s err=%v", orgID, err)
				continue
			}
			for _, m := range members {
				if strings.ToLower(strings.TrimSpace(m)) == gitlabUsername {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// employeeSignaturePrecheck loads project/company/user, ensures the user is affiliated with the company,
// validates the company CCLA exists, and checks allowlist approval.

func (h *Handlers) employeeSignaturePrecheck(ctx context.Context, projectID, companyID, userID string) (project map[string]types.AttributeValue, company map[string]types.AttributeValue, user map[string]types.AttributeValue, cclaSig map[string]types.AttributeValue, errResp map[string]any, err error) {
	if h.projects == nil || h.companies == nil || h.users == nil || h.signatures == nil {
		return nil, nil, nil, nil, map[string]any{"errors": map[string]any{"server": "required stores not configured"}}, nil
	}

	project, found, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if !found {
		return nil, nil, nil, nil, map[string]any{"errors": map[string]any{"project_id": fmt.Sprintf("Project (%s) does not exist.", projectID)}}, nil
	}

	company, found, err = h.companies.GetByID(ctx, companyID)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if !found {
		return project, nil, nil, nil, map[string]any{"errors": map[string]any{"company_id": fmt.Sprintf("Company (%s) does not exist.", companyID)}}, nil
	}

	user, found, err = h.users.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if !found {
		return project, company, nil, nil, map[string]any{"errors": map[string]any{"user_id": fmt.Sprintf("User (%s) does not exist.", userID)}}, nil
	}

	// Find an approved CCLA signature for (company_id, project_id).
	// Python: Signature.get_ccla_signatures_by_company_project()
	items, err := h.signatures.QueryByProjectAndReference(ctx, projectID, companyID)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	matches := make([]map[string]types.AttributeValue, 0, 1)
	for _, it := range items {
		if strings.ToLower(strings.TrimSpace(getAttrString(it, "signature_reference_type"))) != "company" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(getAttrString(it, "signature_type"))) != "ccla" {
			continue
		}
		// Exclude employee signatures.
		if _, ok := it["signature_user_ccla_company_id"]; ok {
			continue
		}
		if av, ok := it["signature_signed"].(*types.AttributeValueMemberBOOL); !ok || !av.Value {
			continue
		}
		if av, ok := it["signature_approved"].(*types.AttributeValueMemberBOOL); !ok || !av.Value {
			continue
		}
		matches = append(matches, it)
	}

	companyName := getAttrString(company, "company_name")
	signingEntityName := getAttrString(company, "signing_entity_name")
	companyExternalID := getAttrString(company, "company_external_id")

	if len(matches) == 0 {
		// Python: {"errors": {"missing_ccla": "Company does not have CCLA with this project.", ...}}
		return project, company, user, nil, map[string]any{"errors": map[string]any{
			"missing_ccla":        "Company does not have CCLA with this project.",
			"company_id":          companyID,
			"company_name":        companyName,
			"signing_entity_name": signingEntityName,
			"company_external_id": companyExternalID,
		}}, nil
	}
	if len(matches) > 1 {
		logging.Warnf("Why do we have more than one CCLA signature for company id=%s, project id=%s", companyID, projectID)
	}
	cclaSig = matches[0]

	// Enforce CCLA allowlists.
	ok, err := h.isUserApprovedByCCLASignature(ctx, user, cclaSig)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if !ok {
		// Python: {"errors": {"ccla_approval_list": "user not authorized for this ccla", ...}}
		return project, company, user, cclaSig, map[string]any{"errors": map[string]any{
			"ccla_approval_list":  "user not authorized for this ccla",
			"company_id":          companyID,
			"company_name":        companyName,
			"signing_entity_name": signingEntityName,
			"company_external_id": companyExternalID,
		}}, nil
	}

	// Update user_company_id (best-effort parity).
	changed := false
	curCID := strings.TrimSpace(getAttrString(user, "user_company_id"))
	if curCID != companyID {
		user["user_company_id"] = &types.AttributeValueMemberS{Value: companyID}
		changed = true
		userName := strings.TrimSpace(getAttrString(user, "user_name"))
		companyName := strings.TrimSpace(getAttrString(company, "company_name"))
		projectName := strings.TrimSpace(getAttrString(project, "project_name"))
		githubUsername := strings.TrimSpace(getAttrString(user, "user_github_username"))
		if githubUsername == "" {
			githubUsername = strings.TrimSpace(getAttrString(user, "github_username"))
		}
		githubID := strings.TrimSpace(getAttrString(user, "user_github_id"))
		if githubID == "" {
			githubID = strings.TrimSpace(getAttrString(user, "github_id"))
		}
		eventData := fmt.Sprintf("The user %s with GitHub username %s (%s) and user ID %s is now associated with company %s for project %s", userName, githubUsername, githubID, userID, companyName, projectName)
		eventSummary := fmt.Sprintf("User %s with GitHub username %s is now associated with company %s for project %s.", userName, githubUsername, companyName, projectName)
		h.putAuditEventBestEffort(ctx, auditEventInput{
			EventType:       "UserAssociatedWithCompany",
			EventCLAGroupID: projectID,
			EventCompanyID:  companyID,
			EventUserID:     userID,
			EventData:       eventData,
			EventSummary:    eventSummary,
			ContainsPII:     true,
		})
	}

	// Backfill GitHub username/id if one is missing (Python does this in the precheck).
	githubUsername := strings.TrimSpace(getAttrString(user, "user_github_username"))
	if githubUsername == "" {
		githubUsername = strings.TrimSpace(getAttrString(user, "github_username"))
	}
	githubID := strings.TrimSpace(getAttrString(user, "user_github_id"))
	if githubID == "" {
		githubID = strings.TrimSpace(getAttrString(user, "github_id"))
	}
	if githubUsername == "" && githubID != "" {
		uname, err := h.githubLookupUsernameByID(ctx, githubID)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		if uname != "" {
			githubUsername = strings.TrimSpace(uname)
			user["user_github_username"] = &types.AttributeValueMemberS{Value: githubUsername}
			changed = true
		}
	}
	if githubID == "" && githubUsername != "" {
		id, err := h.githubLookupIDByUsername(ctx, githubUsername)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		if id != "" {
			githubID = strings.TrimSpace(id)
			// UserModel.user_github_id is a pynamodb NumberAttribute (DDB N) and
			// the github-id-index GSI hash key is typed N. Writing S would leave
			// this row out of the index. githubID is already a numeric string.
			user["user_github_id"] = &types.AttributeValueMemberN{Value: githubID}
			changed = true
		}
	}

	if changed {
		user["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}
		if err := h.users.PutItem(ctx, user); err != nil {
			return nil, nil, nil, nil, nil, err
		}
	}

	return project, company, user, cclaSig, nil, nil
}

func (h *Handlers) RequestEmployeeSignatureV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := parseEmployeeSignatureRequestV2(r)

	// Hug returns 400 for missing required params.
	missing := map[string]any{}
	if strings.TrimSpace(req.ProjectID) == "" {
		missing["project_id"] = "missing"
	}
	if strings.TrimSpace(req.CompanyID) == "" {
		missing["company_id"] = "missing"
	}
	if strings.TrimSpace(req.UserID) == "" {
		missing["user_id"] = "missing"
	}
	if strings.TrimSpace(req.ReturnURLType) == "" {
		missing["return_url_type"] = "missing"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}
	if _, err := uuid.Parse(req.ProjectID); err != nil {
		missing["project_id"] = "invalid uuid"
	}
	if _, err := uuid.Parse(req.CompanyID); err != nil {
		missing["company_id"] = "invalid uuid"
	}
	if _, err := uuid.Parse(req.UserID); err != nil {
		missing["user_id"] = "invalid uuid"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}

	returnURLType := strings.ToLower(strings.TrimSpace(req.ReturnURLType))
	switch returnURLType {
	case "github", "gitlab", "gerrit":
	default:
		msg := fmt.Sprintf("cla.controllers.signing.request_employee_signature - unsupported return type %s for cla group: %s, company: %s, user: %s", req.ReturnURLType, req.ProjectID, req.CompanyID, req.UserID)
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"title": msg}})
		return
	}

	// This now happens in V4
	/*
		if strings.TrimSpace(req.ReturnURL) != "" {
			if _, err := validateURL(req.ReturnURL); err != nil {
				respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"return_url": "invalid"}})
				return
			}
		}
	*/

	project, company, user, cclaSig, errResp, err := h.employeeSignaturePrecheck(ctx, req.ProjectID, req.CompanyID, req.UserID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if errResp != nil {
		respond.JSON(w, http.StatusOK, errResp)
		return
	}

	fn := "docusign_models.check_and_prepare_employee_signature"

	// NOTE: Python does NOT do sanction checks in check_and_prepare_employee_signature().
	// It does it here (request_employee_signature / request_employee_signature_gerrit) after the precheck.
	if av, ok := company["is_sanctioned"].(*types.AttributeValueMemberBOOL); ok && av.Value {
		sanctioned := map[string]any{
			"sanctioned":  fmt.Sprintf("%s - user %s, company %s is sanctioned", fn, req.UserID, req.CompanyID),
			"description": "We’re sorry, but you are currently unable to sign the Employee Contributor License Agreement (ECLA). If you believe this may be an error, please reach out to support",
			"user_id":     req.UserID,
			"company_id":  req.CompanyID,
		}
		respond.JSON(w, http.StatusOK, map[string]any{"code": 403, "errors": sanctioned})
		return
	}

	// If the employee signature already exists, return it.
	existing, err := h.signatures.QueryByProjectAndReference(ctx, req.ProjectID, req.UserID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	for _, it := range existing {
		if strings.ToLower(strings.TrimSpace(getAttrString(it, "signature_reference_type"))) != "user" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(getAttrString(it, "signature_type"))) != "cla" {
			continue
		}
		if getAttrString(it, "signature_user_ccla_company_id") != req.CompanyID {
			continue
		}
		if av, ok := it["signature_signed"].(*types.AttributeValueMemberBOOL); ok && !av.Value {
			continue
		}
		if av, ok := it["signature_approved"].(*types.AttributeValueMemberBOOL); ok && !av.Value {
			continue
		}
		out := store.ItemToInterfaceMap(it)
		delete(out, "user_docusign_raw_xml")
		respond.JSON(w, http.StatusOK, out)
		return
	}

	// Determine the CCLA document version to attach to the employee signature.
	maj, min, err := h.projects.LatestCorporateDocumentVersion(ctx, req.ProjectID)
	if err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"project_id": err.Error()}})
		return
	}

	// Python derives the return URL from active signature metadata when not provided.
	var signatureMetadata map[string]any
	if h.kv != nil {
		metadata, ok, lookupErr := h.loadActiveSignatureMetadata(ctx, req.UserID)
		if lookupErr != nil {
			logging.Warnf("active signature metadata lookup failed for employee signature user=%s err=%v", req.UserID, lookupErr)
		} else if ok {
			signatureMetadata = metadata
			if strings.TrimSpace(req.ReturnURL) == "" {
				if ru, rerr := h.computeReturnURLFromActiveSignatureMetadata(ctx, metadata); rerr == nil && strings.TrimSpace(ru) != "" {
					req.ReturnURL = ru
				}
			}
		}
	}

	githubID := strings.TrimSpace(getAttrString(user, "user_github_id"))
	if githubID == "" {
		githubID = strings.TrimSpace(getAttrString(user, "github_id"))
	}
	githubUsername := strings.TrimSpace(getAttrString(user, "user_github_username"))
	if githubUsername == "" {
		githubUsername = strings.TrimSpace(getAttrString(user, "github_username"))
	}
	gitlabID := strings.TrimSpace(getAttrString(user, "user_gitlab_id"))
	lfUsername := strings.TrimSpace(getAttrString(user, "user_lf_username"))
	if lfUsername == "" {
		lfUsername = strings.TrimSpace(getAttrString(user, "lf_username"))
	}

	var gerrits []map[string]types.AttributeValue
	var aclValue string
	switch returnURLType {
	case "gitlab":
		if gitlabID == "" {
			gitlabID = "None"
		}
		aclValue = "gitlab:" + gitlabID
	case "gerrit":
		if h.gerritInstances != nil {
			gerrits, err = h.gerritInstances.QueryByProjectID(ctx, req.ProjectID)
			if err != nil {
				respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
				return
			}
		}
		if len(gerrits) == 0 {
			respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"missing_gerrit": "No Gerrit instance configured for this project"}})
			return
		}
		if lfUsername == "" {
			lfUsername = "None"
		}
		aclValue = lfUsername
	default:
		if githubID == "" {
			githubID = "None"
		}
		aclValue = "github:" + githubID
	}

	now := time.Now().UTC()
	// Match the rest of the codebase's signature writes — pynamodb
	// UTCDateTimeAttribute format ("YYYY-MM-DDTHH:MM:SS.ffffff+0000"), not
	// RFC3339Nano. Mixed formats break downstream date parsers tuned to the
	// six-microsecond +0000 layout.
	currentTime := formatPynamoDateTimeUTC(now)
	sigID := uuid.NewString()
	sigItem := map[string]types.AttributeValue{
		"signature_id":                     &types.AttributeValueMemberS{Value: sigID},
		"signature_project_id":             &types.AttributeValueMemberS{Value: req.ProjectID},
		"signature_document_minor_version": &types.AttributeValueMemberN{Value: strconv.Itoa(min)},
		"signature_document_major_version": &types.AttributeValueMemberN{Value: strconv.Itoa(maj)},
		"signature_reference_id":           &types.AttributeValueMemberS{Value: req.UserID},
		"signature_reference_type":         &types.AttributeValueMemberS{Value: "user"},
		"signature_type":                   &types.AttributeValueMemberS{Value: "cla"},
		"signature_signed":                 &types.AttributeValueMemberBOOL{Value: true},
		"signature_approved":               &types.AttributeValueMemberBOOL{Value: true},
		"signature_embargo_acked":          &types.AttributeValueMemberBOOL{Value: true},
		"signature_user_ccla_company_id":   &types.AttributeValueMemberS{Value: req.CompanyID},
		"signature_acl":                    &types.AttributeValueMemberSS{Value: []string{aclValue}},
		"date_created":                     &types.AttributeValueMemberS{Value: currentTime},
		"date_modified":                    &types.AttributeValueMemberS{Value: currentTime},
	}

	userName := strings.TrimSpace(getAttrString(user, "user_name"))
	if userName != "" {
		sigItem["signature_reference_name"] = &types.AttributeValueMemberS{Value: userName}
	}
	if strings.TrimSpace(req.ReturnURL) != "" {
		sigItem["signature_return_url"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(req.ReturnURL)}
	}

	// Gerrit path in Python catches save failures and returns null (HTTP 200).
	if err := h.signatures.PutItem(ctx, sigItem); err != nil {
		if returnURLType == "gerrit" {
			logging.Warnf("request_employee_signature_gerrit save failed: %v", err)
			respond.JSON(w, http.StatusOK, nil)
			return
		}
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	projectName := strings.TrimSpace(getAttrString(project, "project_name"))
	companyName := strings.TrimSpace(getAttrString(company, "company_name"))
	if returnURLType == "gerrit" {
		eventData := fmt.Sprintf("The user %s acknowledged the CLA company affiliation for company %s with ID %s, project %s with ID %s.", userName, companyName, req.CompanyID, projectName, req.ProjectID)
		eventSummary := fmt.Sprintf("The user %s acknowledged the CLA company affiliation for company %s and project %s.", userName, companyName, projectName)
		h.putAuditEventBestEffort(ctx, auditEventInput{EventType: "EmployeeSignatureCreated", EventCompanyID: req.CompanyID, EventCLAGroupID: req.ProjectID, EventUserID: req.UserID, EventData: eventData, EventSummary: eventSummary, ContainsPII: true})

		for _, gerrit := range gerrits {
			groupID := strings.TrimSpace(getAttrString(gerrit, "group_id_ccla"))
			if groupID == "" {
				continue
			}

			// LFGroup/LDAP membership updates are legacy best-effort side effects.
			// Do not let a removed or unconfigured LFGroup service change the Gerrit
			// acknowledgement response after the signature has already been saved.
			lfGroupConfigured := h.lfGroup != nil &&
				strings.TrimSpace(h.lfGroup.BaseURL) != "" &&
				strings.TrimSpace(h.lfGroup.ClientID) != "" &&
				strings.TrimSpace(h.lfGroup.ClientSecret) != "" &&
				strings.TrimSpace(h.lfGroup.RefreshToken) != ""
			if !lfGroupConfigured {
				logging.Debugf("request_employee_signature_gerrit skipping legacy LFGroup update; LFGroup client not configured group_id=%s user=%s", groupID, lfUsername)
				continue
			}

			if res := h.lfGroup.AddUserToGroup(ctx, groupID, lfUsername); res != nil {
				if _, bad := res["error"]; bad {
					logging.Warnf("request_employee_signature_gerrit ignored legacy LFGroup update failure group_id=%s user=%s result=%v", groupID, lfUsername, res)
				}
			}
		}
	} else {
		eventData := fmt.Sprintf("The user %s acknowledged the CLA employee affiliation for company %s with ID %s, cla group %s with ID %s.", userName, companyName, req.CompanyID, projectName, req.ProjectID)
		eventSummary := fmt.Sprintf("The user %s acknowledged the CLA employee affiliation for company %s and cla group %s.", userName, companyName, projectName)
		h.putAuditEventBestEffort(ctx, auditEventInput{EventType: "EmployeeSignatureCreated", EventCompanyID: req.CompanyID, EventCLAGroupID: req.ProjectID, EventUserID: req.UserID, EventData: eventData, EventSummary: eventSummary, ContainsPII: true})
		h.putAuditEventBestEffort(ctx, auditEventInput{EventType: "EmployeeSignatureSigned", EventCompanyID: req.CompanyID, EventCLAGroupID: req.ProjectID, EventUserID: req.UserID, EventData: eventData, EventSummary: eventSummary, ContainsPII: true})

		if strings.EqualFold(returnURLType, "github") {
			uid := strings.TrimSpace(getAttrString(user, "user_id"))
			aff := strings.TrimSpace(getAttrString(user, "user_company_id")) != ""
			emails := getUserEmailsForApproval(user)
			githublegacy.UpdateCacheAfterSignature(req.ProjectID, uid, githubID, githubUsername, emails, aff)
		}

		// EASYCLA_PARITY_FLAG: legacy Python also updates the repository provider when the project
		// does not require a separate ICLA, and only removes active signature metadata after that side effect succeeds.
		if av, ok := project["project_ccla_requires_icla_signature"].(*types.AttributeValueMemberBOOL); ok && !av.Value {
			switch strings.ToLower(returnURLType) {
			case "github":
				if signatureMetadata == nil {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": legacyPythonNilSubscriptError().Error()}})
					return
				}
				githubRepositoryMeta, ok := signatureMetadata["repository_id"]
				if !ok {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": legacyPythonKeyError("repository_id").Error()}})
					return
				}
				changeRequestMeta, ok := signatureMetadata["pull_request_id"]
				if !ok {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": legacyPythonKeyError("pull_request_id").Error()}})
					return
				}
				githubRepositoryID := strings.TrimSpace(fmt.Sprintf("%v", githubRepositoryMeta))
				if githubRepositoryID == "<nil>" {
					githubRepositoryID = ""
				}
				changeRequestID := strings.TrimSpace(fmt.Sprintf("%v", changeRequestMeta))
				if changeRequestID == "<nil>" {
					changeRequestID = ""
				}
				installationID, found, installErr := h.githubInstallationIDFromRepository(ctx, githubRepositoryID)
				if installErr != nil {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": installErr.Error()}})
					return
				}
				if !found {
					respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"github_repository_id": "The given github repository ID does not exist. "}})
					return
				}
				if err := h.triggerGitHubChangeRequestUpdateV4(ctx, installationID, githubRepositoryID, changeRequestID); err != nil {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
					return
				}
			case "gitlab":
				if signatureMetadata == nil {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": legacyPythonNilSubscriptError().Error()}})
					return
				}
				repositoryMeta, ok := signatureMetadata["repository_id"]
				if !ok {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": legacyPythonKeyError("repository_id").Error()}})
					return
				}
				mergeMeta, ok := signatureMetadata["merge_request_id"]
				if !ok {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": legacyPythonKeyError("merge_request_id").Error()}})
					return
				}
				gitlabRepositoryID, convErr := pythonIntFromAny(repositoryMeta)
				if convErr != nil {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": convErr.Error()}})
					return
				}
				mergeRequestID, convErr := pythonIntFromAny(mergeMeta)
				if convErr != nil {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": convErr.Error()}})
					return
				}
				organizationID, found, orgErr := h.gitlabOrganizationIDFromRepository(ctx, strconv.FormatInt(gitlabRepositoryID, 10))
				if orgErr != nil {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": orgErr.Error()}})
					return
				}
				var organizationIDPtr *string
				if found {
					organizationIDPtr = &organizationID
				}
				if err := h.triggerGitLabMergeRequestUpdateV4(ctx, organizationIDPtr, gitlabRepositoryID, mergeRequestID); err != nil {
					respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
					return
				}
				if !found {
					respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"gitlab_repository_id": "The given github repository ID does not exist. "}})
					return
				}
			}
			if h.kv != nil {
				_ = h.kv.Delete(ctx, fmt.Sprintf("active_signature:%s", req.UserID))
			}
		}
	}

	_ = cclaSig
	out := store.ItemToInterfaceMap(sigItem)
	respond.JSON(w, http.StatusOK, out)
}

// POST /v2/check-prepare-employee-signature
// Python: cla/routes.py:1865 check_and_prepare_employee_signature()
// Calls: cla.controllers.signing.check_and_prepare_employee_signature

func (h *Handlers) CheckAndPrepareEmployeeSignatureV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := parseEmployeeSignatureRequestV2(r)

	// Hug returns 400 for missing required params.
	missing := map[string]any{}
	if strings.TrimSpace(req.ProjectID) == "" {
		missing["project_id"] = "missing"
	}
	if strings.TrimSpace(req.CompanyID) == "" {
		missing["company_id"] = "missing"
	}
	if strings.TrimSpace(req.UserID) == "" {
		missing["user_id"] = "missing"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}
	if _, err := uuid.Parse(req.ProjectID); err != nil {
		missing["project_id"] = "invalid uuid"
	}
	if _, err := uuid.Parse(req.CompanyID); err != nil {
		missing["company_id"] = "invalid uuid"
	}
	if _, err := uuid.Parse(req.UserID); err != nil {
		missing["user_id"] = "invalid uuid"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}

	_, _, _, _, errResp, err := h.employeeSignaturePrecheck(ctx, req.ProjectID, req.CompanyID, req.UserID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if errResp != nil {
		respond.JSON(w, http.StatusOK, errResp)
		return
	}

	// Python returns: {'success': {'the employee is ready to sign the CCLA'}}
	// (a Python set). We serialize as a JSON list for stability.
	respond.JSON(w, http.StatusOK, map[string]any{"success": []string{"the employee is ready to sign the CCLA"}})
}

// POST /v2/signed/individual/{installation_id}/{github_repository_id}/{change_request_id}
// Python: cla/routes.py:1884 post_individual_signed()
// Calls: cla.controllers.signing.post_individual_signed

func (h *Handlers) PostIndividualSignedV2(w http.ResponseWriter, r *http.Request) {
	// Legacy parity: Hug validates these path params as numbers and rejects malformed values with 400.
	for _, key := range []string{"installation_id", "github_repository_id", "change_request_id"} {
		if _, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, key)), 10, 64); err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{key: "invalid number"}})
			return
		}
	}

	// DocuSign Connect callback endpoint.
	//
	// We forward to the v4 Go backend (source of truth for signing + callback processing).
	// For parity with legacy behavior, always respond 200 OK to DocuSign even if the
	// downstream call fails, to avoid retries.
	body, _ := io.ReadAll(r.Body)
	path := strings.TrimPrefix(r.URL.Path, "/v2")
	if path == r.URL.Path {
		// Defensive fallback.
		path = r.URL.Path
	}
	if q := strings.TrimSpace(r.URL.RawQuery); q != "" {
		path = path + "?" + q
	}
	status, hdr, respBody, err := h.doRequestToV4(r.Context(), http.MethodPost, path, r.Header, body)
	if err != nil {
		logging.Warnf("v4 post_individual_signed forward failed: %v", err)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		// FIXME: possible block if failed
		// w.WriteHeader(http.StatusBadGateway)
		// _, _ = w.Write([]byte("Bad Gateway"))
		return
	}
	if status >= 400 {
		logging.Warnf("v4 signed/individual returned %d: %s", status, string(respBody))
		// FIXME: possible block if failed
		// copyV4ResponseHeaders(w, hdr)
		// w.WriteHeader(status)
		// _, _ = w.Write(respBody)
		// return
	}

	copyV4ResponseHeaders(w, hdr)
	w.WriteHeader(http.StatusOK)
	if len(respBody) == 0 {
		_, _ = w.Write([]byte("OK"))
		return
	}
	_, _ = w.Write(respBody)
}

// POST /v2/signed/gitlab/individual/{user_id}/{organization_id}/{gitlab_repository_id}/{merge_request_id}
// Python: cla/routes.py:1906 post_individual_signed_gitlab()
// Calls: cla.controllers.signing.post_individual_signed_gitlab

func (h *Handlers) PostIndividualSignedGitlabV2(w http.ResponseWriter, r *http.Request) {
	if _, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "user_id"))); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}
	for _, key := range []string{"gitlab_repository_id", "merge_request_id"} {
		if _, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, key)), 10, 64); err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{key: "invalid number"}})
			return
		}
	}

	body, _ := io.ReadAll(r.Body)
	path := strings.TrimPrefix(r.URL.Path, "/v2")
	if path == r.URL.Path {
		path = r.URL.Path
	}
	if q := strings.TrimSpace(r.URL.RawQuery); q != "" {
		path = path + "?" + q
	}
	status, hdr, respBody, err := h.doRequestToV4(r.Context(), http.MethodPost, path, r.Header, body)
	if err != nil {
		logging.Warnf("v4 post_individual_signed_gitlab forward failed: %v", err)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		// FIXME: possible block if failed
		// w.WriteHeader(http.StatusBadGateway)
		// _, _ = w.Write([]byte("Bad Gateway"))
		return
	}
	if status >= 400 {
		logging.Warnf("v4 signed/gitlab/individual returned %d: %s", status, string(respBody))
		// copyV4ResponseHeaders(w, hdr)
		// w.WriteHeader(status)
		// _, _ = w.Write(respBody)
		// return
	}
	copyV4ResponseHeaders(w, hdr)
	w.WriteHeader(http.StatusOK)
	if len(respBody) == 0 {
		_, _ = w.Write([]byte("OK"))
		return
	}
	_, _ = w.Write(respBody)
}

// POST /v2/signed/gerrit/individual/{user_id}
// Python: cla/routes.py:1925 post_individual_signed_gerrit()
// Calls: cla.controllers.signing.post_individual_signed_gerrit

func (h *Handlers) PostIndividualSignedGerritV2(w http.ResponseWriter, r *http.Request) {
	if _, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "user_id"))); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"user_id": "invalid uuid"}})
		return
	}

	body, _ := io.ReadAll(r.Body)
	path := strings.TrimPrefix(r.URL.Path, "/v2")
	if path == r.URL.Path {
		path = r.URL.Path
	}
	if q := strings.TrimSpace(r.URL.RawQuery); q != "" {
		path = path + "?" + q
	}
	status, hdr, respBody, err := h.doRequestToV4(r.Context(), http.MethodPost, path, r.Header, body)
	if err != nil {
		logging.Warnf("v4 post_individual_signed_gerrit forward failed: %v", err)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		// FIXME: possible block if failed
		// w.WriteHeader(http.StatusBadGateway)
		// _, _ = w.Write([]byte("Bad Gateway"))
		return
	}
	if status >= 400 {
		logging.Warnf("v4 signed/gerrit/individual returned %d: %s", status, string(respBody))
		// FIXME: possible block if failed
		// copyV4ResponseHeaders(w, hdr)
		// w.WriteHeader(status)
		// _, _ = w.Write(respBody)
		// return
	}
	copyV4ResponseHeaders(w, hdr)
	w.WriteHeader(http.StatusOK)
	if len(respBody) == 0 {
		_, _ = w.Write([]byte("OK"))
		return
	}
	_, _ = w.Write(respBody)
}

// POST /v2/signed/corporate/{project_id}/{company_id}
// Python: cla/routes.py:1936 post_corporate_signed()
// Calls: cla.controllers.signing.post_corporate_signed

func (h *Handlers) PostCorporateSignedV2(w http.ResponseWriter, r *http.Request) {
	for _, key := range []string{"project_id", "company_id"} {
		if _, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, key))); err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{key: "invalid uuid"}})
			return
		}
	}

	body, _ := io.ReadAll(r.Body)
	path := strings.TrimPrefix(r.URL.Path, "/v2")
	if path == r.URL.Path {
		path = r.URL.Path
	}
	if q := strings.TrimSpace(r.URL.RawQuery); q != "" {
		path = path + "?" + q
	}
	status, hdr, respBody, err := h.doRequestToV4(r.Context(), http.MethodPost, path, r.Header, body)
	if err != nil {
		logging.Warnf("v4 post_corporate_signed forward failed: %v", err)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		// FIXME: possible block if failed
		// w.WriteHeader(http.StatusBadGateway)
		// _, _ = w.Write([]byte("Bad Gateway"))
		return
	}
	if status >= 400 {
		logging.Warnf("v4 signed/corporate returned %d: %s", status, string(respBody))
		// FIXME: possible block if failed
		// copyV4ResponseHeaders(w, hdr)
		// w.WriteHeader(status)
		// _, _ = w.Write(respBody)
		// return
	}
	copyV4ResponseHeaders(w, hdr)
	w.WriteHeader(http.StatusOK)
	if len(respBody) == 0 {
		_, _ = w.Write([]byte("OK"))
		return
	}
	_, _ = w.Write(respBody)
}

// GET /v2/return-url/{signature_id}
// Python: cla/routes.py:1950 get_return_url()
// Calls: cla.controllers.signing.return_url

var canceledSignatureHTML = template.Must(template.New("canceledSignature").Parse(`
<html lang="en">
<head>
<title>The Linux Foundation – EasyCLA Signature Failure</title>
<!-- Required meta tags -->
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
<link rel="shortcut icon" href="https://www.linuxfoundation.org/wp-content/uploads/2017/08/favicon.png">
<link rel="stylesheet"
      href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/css/bootstrap.min.css"
      integrity="sha384-Gn5384xqQ1aoWXA+058RXPxPg6fy4IWvTNh0E263XmFcJlSAwiGgFAW/dAiS6JXm"
      crossorigin="anonymous"/>
<script src="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/js/bootstrap.min.js"
        integrity="sha384-JZR6Spejh4U02d8jOt6vLEHfe/JQGiRRSQQxSfFWpi1MquVdAyjUar5+76PVCmYl"
        crossorigin="anonymous"></script>
</head>
<body style='margin-top:20;margin-left:0;margin-right:0;'>
    <div class="text-center">
        <img width=300px"
         src="https://cla-project-logo-prod.s3.amazonaws.com/lf-horizontal-color.svg"
         alt="community bridge logo"/>
    </div>
    <h2 class="text-center">EasyCLA Account Authorization</h2>
    <p class="text-center">
    The authorization process was canceled and your account is not authorized under a signed CLA.  Click the button to authorize your account for
    {{if .SignatureTypeTitle}}{{.SignatureTypeTitle}}{{end}} CLA.
    </p>
    <p class="text-center">
    <a href="{{.SignURL}}" class="btn btn-primary" role="button">
        Retry Docusign Authorization</a>
        {{if .ReturnURL}}
    <a href="{{.ReturnURL}}" class="btn btn-primary" role="button">
        Restart Authorization</a>
        {{end}}
    </p>
</body>
</html>
`))

func (h *Handlers) GetReturnUrlV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	signatureID := chi.URLParam(r, "signature_id")
	if _, err := uuid.Parse(signatureID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"signature_id": "invalid uuid"}})
		return
	}

	sig, found, err := h.signatures.GetByID(ctx, signatureID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": err.Error()})
		return
	}
	if !found {
		// Python returns an HTML response (hug.output_format.html) that serializes the error dict.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": map[string]any{"signature_id": "signature not found"}})
		return
	}

	event := r.URL.Query().Get("event")
	if event == "ttl_expired" {
		// Legacy Python behavior:
		// If signature not signed, regenerate the embedded signing URL via the DocuSign signing service
		// and redirect to the new signing URL.
		//
		// Migration strategy:
		// We delegate regeneration to the v4 Go backend (which owns the DocuSign/JWT integration).
		// This is best-effort: if v4 regeneration fails (auth/config mismatch), we fall back to the
		// existing signature_sign_url for compatibility.
		if !getAttrBool(sig, "signature_signed") {
			refType := strings.ToLower(strings.TrimSpace(getAttrString(sig, "signature_reference_type")))
			if refType == "user" {
				if signURL, err := h.regenerateIndividualSignURLViaV4(ctx, sig, r.Header); err != nil {
					logging.Warnf("ttl_expired sign url regeneration via v4 failed (signature_id=%s): %v", signatureID, err)
				} else if signURL != "" {
					http.Redirect(w, r, signURL, http.StatusFound)
					return
				}
			}
			// Fallback: redirect to existing URL.
			if signURL := getAttrString(sig, "signature_sign_url"); signURL != "" {
				http.Redirect(w, r, signURL, http.StatusFound)
				return
			}
		}
	}

	if event == "cancel" {
		signURL := getAttrString(sig, "signature_sign_url")
		returnURL := getAttrString(sig, "signature_return_url")
		sigType := getAttrString(sig, "signature_type")
		var sigTypeTitle string
		if len(sigType) > 0 {
			sigTypeTitle = strings.ToUpper(sigType[:1]) + strings.ToLower(sigType[1:])
		}
		data := struct {
			SignatureTypeTitle string
			SignURL            string
			ReturnURL          string
		}{
			SignatureTypeTitle: sigTypeTitle,
			SignURL:            signURL,
			ReturnURL:          returnURL,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = canceledSignatureHTML.Execute(w, data)
		return
	}

	if returnURL := getAttrString(sig, "signature_return_url"); returnURL != "" {
		projectID := getAttrString(sig, "signature_project_id")
		refType := strings.ToLower(strings.TrimSpace(getAttrString(sig, "signature_reference_type")))
		if !parity.DisableReturnURLCompanyManagerWait && projectID != "" && refType == "company" && h.projects != nil && h.companies != nil && h.userService != nil && h.projectCLAGroups != nil {
			proj, pFound, pErr := h.projects.GetByID(ctx, projectID)
			if pErr == nil && pFound {
				version := strings.ToLower(strings.TrimSpace(getAttrString(proj, "version")))
				if version == "v2" {
					companyID := getAttrString(sig, "signature_reference_id")
					if companyID != "" {
						comp, cFound, cErr := h.companies.GetByID(ctx, companyID)
						if cErr == nil && cFound {
							orgID := strings.TrimSpace(getAttrString(comp, "company_external_id"))
							managers := getAttrStringSlice(sig, "signature_acl")
							if orgID != "" && len(managers) > 0 {
								numTries := 10
								for i := 1; i <= numTries; i++ {
									assigned := make(map[string]bool, len(managers))
									allAssigned := true
									for _, m := range managers {
										ok, _ := h.userService.HasRole(ctx, m, "cla-manager", orgID, projectID, h.projectCLAGroups)
										assigned[m] = ok
										if !ok {
											allAssigned = false
										}
									}
									logging.Infof("return_url - cla-manager role assigned status (try %d/%d): %v", i, numTries, assigned)
									if allAssigned {
										break
									}
									time.Sleep(500 * time.Millisecond)
								}
							}
						}
					}
				}
			} else if pErr != nil {
				logging.Warnf("return_url - load project failed (project_id=%s): %v", projectID, pErr)
			}
		}

		http.Redirect(w, r, returnURL, http.StatusFound)
		return
	}

	// If no return URL was stored, Python returns a simple success payload (HTML output format).
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": "Thank you for signing"})
}

// POST /v2/send-authority-email
// Python: cla/routes.py:1969 send_authority_email()
// Calls: cla.controllers.signing.send_authority_email

func (h *Handlers) SendAuthorityEmailV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Python parity: cla/routes.py::send_authority_email requires check_auth.
	_, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	var req struct {
		CompanyName    string `json:"company_name"`
		ProjectName    string `json:"project_name"`
		AuthorityName  string `json:"authority_name"`
		AuthorityEmail string `json:"authority_email"`
	}
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.Contains(ct, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": "invalid json"})
			return
		}
	} else {
		_ = r.ParseForm()
		req.CompanyName = r.FormValue("company_name")
		req.ProjectName = r.FormValue("project_name")
		req.AuthorityName = r.FormValue("authority_name")
		req.AuthorityEmail = r.FormValue("authority_email")
	}
	missing := map[string]any{}
	if strings.TrimSpace(req.CompanyName) == "" {
		missing["company_name"] = "missing"
	}
	if strings.TrimSpace(req.ProjectName) == "" {
		missing["project_name"] = "missing"
	}
	if strings.TrimSpace(req.AuthorityName) == "" {
		missing["authority_name"] = "missing"
	}
	if strings.TrimSpace(req.AuthorityEmail) == "" {
		missing["authority_email"] = "missing"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}
	if !validEmailLikePython(req.AuthorityEmail) {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"authority_email": "Invalid email address specified"}})
		return
	}

	// 1:1 port of cla/controllers/signing.py::send_authority_email
	subject := "CLA: Invitation to Sign a Corporate Contributor License Agreement"
	body := fmt.Sprintf(`Hello %s, 
    
Your organization: %s, 
    
has requested a Corporate Contributor License Agreement Form to be signed for the following project:

%s

Please read the agreement carefully and sign the attached file. 
    

- Linux Foundation CLA System
`, req.AuthorityName, req.CompanyName, req.ProjectName)

	svc, err := email.NewFromEnv(ctx)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": err.Error()})
		return
	}
	if err := svc.Send(ctx, subject, body, []string{req.AuthorityEmail}); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": err.Error()})
		return
	}

	// Hug returns null body for successful POST requests.
	respond.JSON(w, http.StatusOK, nil)
}

// GET /v2/repository-provider/{provider}/sign/{installation_id}/{github_repository_id}/{change_request_id}
// Python: cla/routes.py:1995 sign_request()
// Calls: cla.controllers.repository_service.sign_request

func (h *Handlers) SignRequestV2(w http.ResponseWriter, r *http.Request) {
	// Port of legacy Python: GET /v2/repository-provider/{provider}/sign/{installation_id}/{github_repository_id}/{change_request_id}
	// - cla/controllers/repository_service.py::sign_request
	// - cla/models/github_models.py::sign_request
	h.githubSignRequest(w, r)
}

// GET /v2/repository-provider/{provider}/oauth2_redirect
// Python: cla/routes.py:2015 oauth2_redirect()
// Calls: cla.controllers.repository_service.oauth2_redirect

func (h *Handlers) Oauth2RedirectV2(w http.ResponseWriter, r *http.Request) {
	// NOTE: This legacy endpoint is *broken* in the current Python implementation.
	//
	// In cla/routes.py the handler calls:
	//   cla.controllers.repository_service.oauth2_redirect(provider, state, code, repository_id, change_request_id, request)
	// but cla/controllers/repository_service.py defines:
	//   oauth2_redirect(provider, state, code, installation_id, github_repository_id, change_request_id, request)
	// which raises a TypeError (missing required positional argument: 'request') and results in a 500.
	//
	// Preserve 1:1 parity here:
	//   - Requires auth (check_auth)
	//   - Returns 500 for authorized calls
	//
	// EASYCLA_PARITY_FLAG: this behavior is intentionally incorrect to match legacy Python.
	_, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}
	provider := strings.TrimSpace(strings.ToLower(chi.URLParam(r, "provider")))
	if provider != "github" && provider != "mock_github" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"provider": "invalid provider"}})
		return
	}
	q := r.URL.Query()
	missing := map[string]any{}
	for _, key := range []string{"state", "code", "repository_id", "change_request_id"} {
		if strings.TrimSpace(q.Get(key)) == "" {
			missing[key] = "missing"
		}
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}
	respond.JSON(w, http.StatusInternalServerError, map[string]any{
		"errors": map[string]any{
			"server": "legacy python parity: oauth2_redirect is broken; use /v2/github/installation",
		},
	})
}

// POST /v2/repository-provider/{provider}/activity
// Python: cla/routes.py:2038 received_activity()
// Calls: cla.controllers.repository_service.received_activity

func (h *Handlers) ReceivedActivityV2(w http.ResponseWriter, r *http.Request) {
	// Deprecated / legacy webhook endpoint.
	//
	// Legacy Python (GitHub.received_activity) behavior:
	//   - If payload is not a pull_request nor a merge_group: return a message.
	//   - Otherwise: perform side effects and return null.
	provider := strings.TrimSpace(strings.ToLower(chi.URLParam(r, "provider")))
	if provider != "github" && provider != "mock_github" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"provider": "invalid provider"}})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": "unable to read body"})
		return
	}
	_ = r.Body.Close()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": "invalid json"})
		return
	}

	_, isPR := payload["pull_request"]
	_, isMergeGroup := payload["merge_group"]
	if !isPR && !isMergeGroup {
		respond.JSON(w, http.StatusOK, map[string]any{"message": "Not a pull request nor a merge group - no action performed"})
		return
	}
	if provider == "mock_github" {
		respond.JSON(w, http.StatusOK, nil)
		return
	}

	if err := h.handleLegacyGithubReceivedActivity(r.Context(), payload); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	respond.JSON(w, http.StatusOK, nil)
}

// GET /v1/github/organizations
// Python: cla/routes.py:2053 get_github_organizations()
// Calls: cla.controllers.github.get_organizations

func (h *Handlers) GetGithubOrganizationsV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	items, err := h.githubOrgs.ScanAll(ctx)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"github_orgs": err.Error()}})
		return
	}

	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, normalizeGitHubOrgDict(store.ItemToInterfaceMap(it)))
	}

	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/github/organizations/{organization_name}
// Python: cla/routes.py:2063 get_github_organization()
// Calls: cla.controllers.github.get_organization

func (h *Handlers) GetGithubOrganizationV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgName := chi.URLParam(r, "organization_name")

	_, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	item, found, err := h.githubOrgs.GetByName(ctx, orgName)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"organization_name": err.Error()}})
		return
	}
	if !found {
		// Python parity: cla.controllers.github.get_organization() returns
		// {"errors": {"organization_name": ...}} as the function value, and the
		// Hug route does NOT accept `response`, so Hug serializes it as 200.
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"organization_name": "GitHub org not found"}})
		return
	}

	respond.JSON(w, http.StatusOK, normalizeGitHubOrgDict(store.ItemToInterfaceMap(item)))
}

// GET /v1/github/organizations/{organization_name}/repositories
// Python: cla/routes.py:2073 get_github_organization_repos()
// Calls: cla.controllers.github.get_organization_repositories

func (h *Handlers) GetGithubOrganizationReposV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgName := chi.URLParam(r, "organization_name")

	_, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	orgItem, found, err := h.githubOrgs.GetByName(ctx, orgName)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"organization_name": err.Error()}})
		return
	}
	if !found {
		// Python parity: cla.controllers.github.get_organization_repositories()
		// returns {"errors": {...}} on DoesNotExist; the Hug route has no
		// `response` parameter, so the response is HTTP 200.
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"organization_name": orgName, "error": "GitHub org not found"}})
		return
	}

	installationID := int64(getAttrInt(orgItem, "organization_installation_id"))
	if installationID == 0 {
		respond.JSON(w, http.StatusOK, []string{})
		return
	}

	repos, err := h.github.ListInstallationRepositories(ctx, installationID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"organization_installation_id": err.Error()}})
		return
	}

	out := make([]string, 0, len(repos))
	for _, gr := range repos {
		out = append(out, gr.Full)
	}

	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/sfdc/{sfid}/github/organizations
// Python: cla/routes.py:2083 get_github_organization_by_sfid()
// Calls: cla.controllers.github.get_organization_by_sfid

func (h *Handlers) GetGithubOrganizationBySfidV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sfid := chi.URLParam(r, "sfid")

	authUser, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	if ok, errMap := h.checkUserAuthorization(ctx, authUser.Username, sfid); !ok {
		respond.JSON(w, http.StatusForbidden, errMap)
		return
	}

	items, err := h.githubOrgs.QueryBySFID(ctx, sfid)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"sfid": err.Error()}})
		return
	}
	if len(items) == 0 {
		// Python parity: cla.controllers.github.get_organization_by_sfid()
		// returns {"errors": {"sfid": ...}} on DoesNotExist; Hug serializes
		// that as 200.
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"sfid": "GitHub org not found"}})
		return
	}

	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, normalizeGitHubOrgDict(store.ItemToInterfaceMap(it)))
	}

	respond.JSON(w, http.StatusOK, out)
}

// POST /v1/github/organizations
// Python: cla/routes.py:2098 post_github_organization()
// Calls: cla.controllers.github.create_organization

func (h *Handlers) PostGithubOrganizationV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	body, _ := parseFlexibleParams(r)
	orgName, _ := flexibleStringParam(r, body, "organization_name")
	sfid, _ := flexibleStringParam(r, body, "organization_sfid")

	if orgName == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"organization_name": "Missing required field"}})
		return
	}
	if sfid == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"organization_sfid": "Missing required field"}})
		return
	}

	if ok, errMap := h.checkUserAuthorization(ctx, authUser.Username, sfid); !ok {
		respond.JSON(w, http.StatusForbidden, errMap)
		return
	}

	now := time.Now().UTC()
	item, found, err := h.githubOrgs.GetByName(ctx, orgName)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"organization_name": err.Error()}})
		return
	}

	if !found {
		item = map[string]types.AttributeValue{
			"organization_name":         &types.AttributeValueMemberS{Value: orgName},
			"organization_name_lower":   &types.AttributeValueMemberS{Value: strings.ToLower(orgName)},
			"auto_enabled":              &types.AttributeValueMemberBOOL{Value: false},
			"branch_protection_enabled": &types.AttributeValueMemberBOOL{Value: false},
			"enabled":                   &types.AttributeValueMemberBOOL{Value: true},
			"note":                      &types.AttributeValueMemberS{Value: ""},
			"skip_cla":                  &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			"enable_co_authors":         &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			"date_created":              &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)},
			"version":                   &types.AttributeValueMemberS{Value: "v1"},
		}
	}

	item["organization_sfid"] = &types.AttributeValueMemberS{Value: sfid}
	item["project_sfid"] = &types.AttributeValueMemberS{Value: sfid}
	item["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(now)}

	if err := h.githubOrgs.PutItem(ctx, item); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"organization_name": err.Error()}})
		return
	}

	respond.JSON(w, http.StatusOK, normalizeGitHubOrgDict(store.ItemToInterfaceMap(item)))
}

// DELETE /v1/github/organizations/{organization_name}
// Python: cla/routes.py:2116 delete_organization()
// Calls: cla.controllers.github.delete_organization

func (h *Handlers) DeleteOrganizationV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgName := chi.URLParam(r, "organization_name")

	authUser, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}

	orgItem, found, err := h.githubOrgs.GetByName(ctx, orgName)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"organization_name": err.Error()}})
		return
	}
	if !found {
		// Python parity: cla.controllers.github.delete_organization() returns
		// {"errors": {...}} on DoesNotExist; Hug serializes as 200.
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"organization_name": "GitHub org not found"}})
		return
	}

	sfid := getAttrString(orgItem, "organization_sfid")
	if sfid == "" {
		sfid = getAttrString(orgItem, "project_sfid")
	}
	if ok, errMap := h.checkUserAuthorization(ctx, authUser.Username, sfid); !ok {
		respond.JSON(w, http.StatusForbidden, errMap)
		return
	}

	// Delete repositories in this org.
	repos, err := h.repos.QueryByOrganizationName(ctx, orgName)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"organization_name": err.Error()}})
		return
	}
	for _, repoItem := range repos {
		repoID := getAttrString(repoItem, "repository_id")
		if strings.TrimSpace(repoID) == "" {
			continue
		}
		_ = h.repos.DeleteByID(ctx, repoID)
	}

	if err := h.githubOrgs.DeleteByName(ctx, orgName); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"organization_name": err.Error()}})
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"success": true})
}

// GET /v2/github/installation
// Python: cla/routes.py:2127 github_oauth2_callback()
// Calls: cla.controllers.github.user_oauth2_callback

func (h *Handlers) GithubOauth2CallbackV2(w http.ResponseWriter, r *http.Request) {
	// Port of legacy Python: GET /v2/github/installation
	// - cla/controllers/github.py::user_oauth2_callback
	// - cla/models/github_models.py::oauth2_redirect
	h.githubOauth2Callback(w, r)
}

// POST /v2/github/installation
// Python: cla/routes.py:2140 github_app_installation()
// Calls: cla.controllers.github.user_authorization_callback

func (h *Handlers) GithubAppInstallationV2(w http.ResponseWriter, r *http.Request) {
	// Legacy Python: POST /v2/github/installation
	// (cla/controllers/github.py::user_authorization_callback)
	respond.JSON(w, http.StatusOK, map[string]any{"status": "nothing to do here."})
}

// POST /v2/github/activity
// Python: cla/routes.py:2152 github_app_activity()
// Calls: cla.config.PLATFORM_MAINTAINERS.split, cla.controllers.github.activity, cla.controllers.github.webhook_secret_failed_email, cla.controllers.github.webhook_secret_validation, cla.log.debug, cla.log.error

func (h *Handlers) GithubAppActivityV2(w http.ResponseWriter, r *http.Request) {
	// This endpoint is used by the GitHub App webhook.
	//
	// Legacy Python validates the sha1 X-Hub-Signature first, forwards only a
	// small event subset to v4, and handles PR/comment/merge-queue events locally.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": "unable to read body"})
		return
	}
	_ = r.Body.Close()

	valid, verr := githublegacy.ValidateWebhookSignature(body, r.Header.Get("X-Hub-Signature"))
	if verr != nil || !valid {
		h.sendGithubWebhookSecretFailedEmailBestEffort(r.Context(), r.Header, body, verr)
		respond.JSON(w, http.StatusUnauthorized, map[string]any{"status": "Invalid Secret Token"})
		return
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": "invalid json"})
		return
	}
	action := githubActivityAction(payload)

	eventType := strings.TrimSpace(r.Header.Get("X-Github-Event"))
	if eventType == "" {
		eventType = strings.TrimSpace(r.Header.Get("X-GitHub-Event"))
	}

	if shouldForwardGithubActivityToV4(eventType, action) {
		if err := h.forwardGithubActivityToV4(r.Context(), body, r.Header); err != nil {
			logging.Warnf("v4 github/activity forward failed: %v", err)
			respond.JSON(w, http.StatusInternalServerError, map[string]string{"status": fmt.Sprintf("v4_easycla_github_activity failed %v", err)})
			return
		}
		respond.JSON(w, http.StatusOK, map[string]string{"status": "OK"})
		return
	}
	//

	if eventType == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"status": "Invalid request"})
		return
	}
	result, err := h.handleLegacyGithubActivity(r.Context(), eventType, action, payload)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	respond.JSON(w, http.StatusOK, result)
}

// POST /v1/github/validate
// Python: cla/routes.py:2208 github_organization_validation()
// Calls: cla.controllers.github.validate_organization

func (h *Handlers) GithubOrganizationValidationV1(w http.ResponseWriter, r *http.Request) {
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": "invalid json"})
		return
	}
	endpoint, _ := flexibleStringParam(r, body, "endpoint")

	respBody, status, err := h.github.ValidateOrganization(r.Context(), endpoint)
	if err != nil {
		respond.JSON(w, status, map[string]string{"status": "error"})
		return
	}
	respond.JSON(w, status, respBody)
}

// GET /v1/github/check/namespace/{namespace}
// Python: cla/routes.py:2218 github_check_namespace()
// Calls: cla.controllers.github.check_namespace

func (h *Handlers) GithubCheckNamespaceV1(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	ok, status, err := h.github.CheckNamespace(r.Context(), namespace)
	if err != nil {
		// Keep a simple boolean response for clients (Python returns True/False).
		respond.JSON(w, status, false)
		return
	}
	respond.JSON(w, status, ok)
}

// GET /v1/github/get/namespace/{namespace}
// Python: cla/routes.py:2228 github_get_namespace()
// Calls: cla.controllers.github.get_namespace

func (h *Handlers) GithubGetNamespaceV1(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	body, status, err := h.github.GetNamespace(r.Context(), namespace)
	if err != nil {
		// Match legacy error payload shape.
		respond.JSON(w, status, map[string]any{"errors": map[string]string{"namespace": "Invalid GitHub account namespace"}})
		return
	}
	respond.JSON(w, status, body)
}

// GET /v1/project/{project_id}/gerrits
// Python: cla/routes.py:2241 get_project_gerrit_instance()
// Calls: cla.controllers.gerrit.get_gerrit_by_project_id

func (h *Handlers) GetProjectGerritInstanceV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "project_id")
	if _, err := uuid.Parse(projectID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	items, err := h.gerritInstances.QueryByProjectID(ctx, projectID)
	if err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{fmt.Sprintf("a gerrit instance does not exist with the given project ID: %s", projectID): err.Error()}})
		return
	}
	if len(items) == 0 {
		// Legacy Python returns an empty list for DoesNotExist.
		respond.JSON(w, http.StatusOK, []any{})
		return
	}

	out := make([]any, 0, len(items))
	for _, it := range items {
		out = append(out, normalizeGerritDict(store.ItemToInterfaceMap(it)))
	}
	respond.JSON(w, http.StatusOK, out)
}

// GET /v2/gerrit/{gerrit_id}
// Python: cla/routes.py:2251 get_gerrit_instance()
// Calls: cla.controllers.gerrit.get_gerrit

func (h *Handlers) GetGerritInstanceV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	gerritID := chi.URLParam(r, "gerrit_id")
	if _, err := uuid.Parse(gerritID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"gerrit_id": "invalid uuid"}})
		return
	}

	item, found, err := h.gerritInstances.GetByID(ctx, gerritID)
	if err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"a gerrit instance does not exist with the given Gerrit ID. ": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"a gerrit instance does not exist with the given Gerrit ID. ": "Gerrit Instance not found"}})
		return
	}

	respond.JSON(w, http.StatusOK, normalizeGerritDict(store.ItemToInterfaceMap(item)))
}

// POST /v1/gerrit
// Python: cla/routes.py:2261 create_gerrit_instance()
// Calls: cla.controllers.gerrit.create_gerrit

func (h *Handlers) CreateGerritInstanceV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	type request struct {
		ProjectID   string `json:"project_id"`
		GerritName  string `json:"gerrit_name"`
		GerritURL   string `json:"gerrit_url"`
		GroupIDICLA string `json:"group_id_icla"`
		GroupIDCCLA string `json:"group_id_ccla"`
	}
	var req request
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}
	if v, ok := flexibleStringParam(r, body, "project_id"); ok {
		req.ProjectID = v
	}
	if v, ok := flexibleStringParam(r, body, "gerrit_name"); ok {
		req.GerritName = v
	}
	if v, ok := flexibleStringParam(r, body, "gerrit_url"); ok {
		req.GerritURL = v
	}
	if v, ok := flexibleStringParam(r, body, "group_id_icla"); ok {
		req.GroupIDICLA = v
	}
	if v, ok := flexibleStringParam(r, body, "group_id_ccla"); ok {
		req.GroupIDCCLA = v
	}
	missing := map[string]any{}
	if strings.TrimSpace(req.ProjectID) == "" {
		missing["project_id"] = "missing"
	}
	if strings.TrimSpace(req.GerritName) == "" {
		missing["gerrit_name"] = "missing"
	}
	if strings.TrimSpace(req.GerritURL) == "" {
		missing["gerrit_url"] = "missing"
	}
	if len(missing) > 0 {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": missing})
		return
	}
	if _, err := uuid.Parse(strings.TrimSpace(req.ProjectID)); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_id": "invalid uuid"}})
		return
	}

	if strings.TrimSpace(req.GroupIDICLA) == "" && strings.TrimSpace(req.GroupIDCCLA) == "" {
		respond.JSON(w, http.StatusOK, map[string]any{"error": "Should specify at least a LDAP group for ICLA or CCLA."})
		return
	}

	validatedURL, err := validateURL(req.GerritURL)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"gerrit_url": err.Error()}})
		return
	}

	groupNameICLA := ""
	groupNameCCLA := ""

	if strings.TrimSpace(req.GroupIDICLA) != "" {
		ldap := h.lfGroup.GetGroup(ctx, req.GroupIDICLA)
		if ldap != nil {
			if _, ok := ldap["error"]; ok {
				respond.JSON(w, http.StatusOK, map[string]any{"error_icla": "The specified LDAP group for ICLA does not exist."})
				return
			}
			if t, ok := ldap["title"].(string); ok {
				groupNameICLA = t
			}
		}
	}

	if strings.TrimSpace(req.GroupIDCCLA) != "" {
		ldap := h.lfGroup.GetGroup(ctx, req.GroupIDCCLA)
		if ldap != nil {
			if _, ok := ldap["error"]; ok {
				respond.JSON(w, http.StatusOK, map[string]any{"error_ccla": "The specified LDAP group for CCLA does not exist. "})
				return
			}
			if t, ok := ldap["title"].(string); ok {
				groupNameCCLA = t
			}
		}
	}

	gerritID := uuid.New().String()
	now := formatPynamoDateTimeUTC(time.Now().UTC())

	item := map[string]types.AttributeValue{
		"gerrit_id":       &types.AttributeValueMemberS{Value: gerritID},
		"project_id":      &types.AttributeValueMemberS{Value: strings.TrimSpace(req.ProjectID)},
		"date_created":    &types.AttributeValueMemberS{Value: now},
		"date_modified":   &types.AttributeValueMemberS{Value: now},
		"version":         &types.AttributeValueMemberS{Value: "v1"},
		"gerrit_url":      &types.AttributeValueMemberS{Value: validatedURL},
		"gerrit_name":     &types.AttributeValueMemberS{Value: strings.TrimSpace(req.GerritName)},
		"group_id_icla":   &types.AttributeValueMemberS{Value: strings.TrimSpace(req.GroupIDICLA)},
		"group_id_ccla":   &types.AttributeValueMemberS{Value: strings.TrimSpace(req.GroupIDCCLA)},
		"group_name_icla": &types.AttributeValueMemberS{Value: strings.TrimSpace(groupNameICLA)},
		"group_name_ccla": &types.AttributeValueMemberS{Value: strings.TrimSpace(groupNameCCLA)},
	}

	// Mirror pynamodb null=True semantics: do not store empty strings for optional attributes.
	for _, k := range []string{"gerrit_name", "gerrit_url", "group_id_icla", "group_id_ccla", "group_name_icla", "group_name_ccla"} {
		if s, ok := item[k].(*types.AttributeValueMemberS); ok {
			if strings.TrimSpace(s.Value) == "" {
				delete(item, k)
			}
		}
	}

	if err := h.gerritInstances.PutItem(ctx, item); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	respond.JSON(w, http.StatusOK, normalizeGerritDict(store.ItemToInterfaceMap(item)))
}

// DELETE /v1/gerrit/{gerrit_id}
// Python: cla/routes.py:2277 delete_gerrit_instance()
// Calls: cla.controllers.gerrit.delete_gerrit

func (h *Handlers) DeleteGerritInstanceV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	gerritID := chi.URLParam(r, "gerrit_id")
	if _, err := uuid.Parse(gerritID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"gerrit_id": "invalid uuid"}})
		return
	}

	_, found, err := h.gerritInstances.GetByID(ctx, gerritID)
	if err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"gerrit_id": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"errors": map[string]any{"gerrit_id": "Gerrit Instance not found"}})
		return
	}

	if err := h.gerritInstances.DeleteByID(ctx, gerritID); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"success": true})
}

// GET /v2/gerrit/{gerrit_id}/{contract_type}/agreementUrl.html
// Python: cla/routes.py:2289 get_agreement_html()
// Calls: cla.controllers.gerrit.get_agreement_html

func (h *Handlers) GetAgreementHtmlV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	gerritID := chi.URLParam(r, "gerrit_id")
	contractType := chi.URLParam(r, "contract_type")
	if _, err := uuid.Parse(gerritID); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"gerrit_id": "invalid uuid"}})
		return
	}

	consoleV1 := strings.TrimSpace(os.Getenv("CLA_CONTRIBUTOR_BASE"))
	if consoleV1 == "" {
		consoleV1 = strings.TrimSpace(os.Getenv("CLA_CONTRIBUTOR_BASE_CLI"))
	}
	consoleV2 := strings.TrimSpace(os.Getenv("CLA_CONTRIBUTOR_V2_BASE"))
	if consoleV2 == "" {
		consoleV2 = strings.TrimSpace(os.Getenv("CLA_CONTRIBUTOR_V2_BASE_CLI"))
	}

	gerritItem, found, err := h.gerritInstances.GetByID(ctx, gerritID)
	if err != nil || !found {
		msg := "Gerrit Instance not found"
		if err != nil {
			msg = err.Error()
		}
		// Legacy Hug route uses output_format.html. In error cases it serializes the error dict under text/html.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": map[string]any{"gerrit_id": msg}})
		return
	}

	projectID := getAttrString(gerritItem, "project_id")
	if projectID == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": map[string]any{"project_id": "Project not found"}})
		return
	}

	projItem, projFound, err := h.projects.GetByID(ctx, projectID)
	if err != nil || !projFound {
		msg := "Project not found"
		if err != nil {
			msg = err.Error()
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": map[string]any{"project_id": msg}})
		return
	}

	projVersion := getAttrString(projItem, "version")
	gerritURL := getAttrString(gerritItem, "gerrit_url")

	consoleURL := ""
	if projVersion == "v2" {
		consoleURL = fmt.Sprintf("https://%s/#/cla/gerrit/project/%s/%s?redirect=%s", consoleV2, projectID, contractType, gerritURL)
	} else {
		consoleURL = fmt.Sprintf("https://%s/#/cla/gerrit/project/%s/%s?redirect=%s", consoleV1, projectID, contractType, gerritURL)
	}

	var contractTypeTitle string
	if len(contractType) > 0 {
		contractTypeTitle = strings.ToUpper(contractType[:1]) + strings.ToLower(contractType[1:])
	}

	htmlContent := fmt.Sprintf(`
        <html lang="en">
        <head>
        <title>The Linux Foundation – EasyCLA Gerrit %s Console Redirect</title>
        <!-- Required meta tags -->
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <link rel="shortcut icon" href="https://www.linuxfoundation.org/wp-content/uploads/2017/08/favicon.png">
        <link rel="stylesheet"
              href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/css/bootstrap.min.css"
              integrity="sha384-Gn5384xqQ1aoWXA+058RXPxPg6fy4IWvTNh0E263XmFcJlSAwiGgFAW/dAiS6JXm"
              crossorigin="anonymous"/>
        <script src="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/js/bootstrap.min.js"
                integrity="sha384-JZR6Spejh4U02d8jOt6vLEHfe/JQGiRRSQQxSfFWpi1MquVdAyjUar5+76PVCmYl"
                crossorigin="anonymous"></script>
        </head>
        <body style='margin-top:20;margin-left:0;margin-right:0;'>
            <div class="text-center">
                <img width=300px"
                 src="https://cla-project-logo-prod.s3.amazonaws.com/lf-horizontal-color.svg"
                 alt="community bridge logo"/>
            </div>
            <h2 class="text-center">EasyCLA Account Authorization</h2>
            <p class="text-center">
            Your account is not authorized under a signed CLA.  Click the button to authorize your account for a
            %s CLA.
            </p>
            <p class="text-center">
            <a href="%s" class="btn btn-primary" role="button">
                Proceed To %s Authorization</a>
            </p>
        </body>
        </html>
        `, html.EscapeString(contractTypeTitle), html.EscapeString(contractTypeTitle), html.EscapeString(consoleURL), html.EscapeString(contractTypeTitle))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(htmlContent))
}

// GET /v1/project/logo/{project_sfdc_id}
// Python: cla/routes.py:2302 upload_logo()
// Calls: cla.controllers.project_logo.create_signed_logo_url

func (h *Handlers) UploadLogoV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectSFID := chi.URLParam(r, "project_sfdc_id")

	// Validate project SFID to prevent path traversal attacks
	if projectSFID == "" || strings.Contains(projectSFID, "..") || strings.Contains(projectSFID, "/") {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project_sfdc_id"})
		return
	}

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}
	if !isAdminUser(authUser.Username) {
		respond.JSON(w, http.StatusOK, map[string]any{"error": "unauthorized"})
		return
	}

	claLogoURL := strings.TrimSpace(os.Getenv("CLA_BUCKET_LOGO_URL"))
	if claLogoURL == "" {
		respond.JSON(w, http.StatusOK, map[string]any{"error": "CLA_BUCKET_LOGO_URL is empty"})
		return
	}
	u, err := url.Parse(claLogoURL)
	if err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}
	// Legacy Python uses: logo_bucket_parts.path.replace('/', '')
	logoBucket := strings.ReplaceAll(u.Path, "/", "")
	if logoBucket == "" {
		// Legacy Python assumes path-style bucket URLs.
		respond.JSON(w, http.StatusOK, map[string]any{"error": "Unable to determine logo bucket"})
		return
	}

	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("REGION"))
	}
	if region == "" {
		region = "us-east-1"
	}

	stage := strings.TrimSpace(os.Getenv("STAGE"))
	if stage == "" {
		stage = "dev"
	}

	// Load AWS config (with optional local endpoint for local dev parity).
	loadOpts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if stage == "local" {
		endpointURL := "http://localhost:8001"
		loadOpts = append(loadOpts, config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				if service == s3.ServiceID {
					return aws.Endpoint{URL: endpointURL, SigningRegion: region, HostnameImmutable: true}, nil
				}
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			}),
		))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if stage == "local" {
			o.UsePathStyle = true
		}
	})
	presigner := s3.NewPresignClient(s3Client)

	filePath := fmt.Sprintf("%s.png", projectSFID)
	ps, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(logoBucket),
		Key:         aws.String(filePath),
		ContentType: aws.String("image/png"),
	}, s3.WithPresignExpires(300*time.Second))
	if err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"signed_url": ps.URL})
}

// POST /v1/project/permission
// Python: cla/routes.py:2307 add_project_permission()
// Calls: cla.controllers.project.add_permission

func (h *Handlers) AddProjectPermissionV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	type request struct {
		Username    string `json:"username"`
		ProjectSFID string `json:"project_sfdc_id"`
	}
	var req request
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}
	if v, ok := flexibleStringParam(r, body, "username"); ok {
		req.Username = v
	}
	if v, ok := flexibleStringParam(r, body, "project_sfdc_id"); ok {
		req.ProjectSFID = v
	}

	if strings.TrimSpace(req.Username) == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"username": "missing"}})
		return
	}
	if strings.TrimSpace(req.ProjectSFID) == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_sfdc_id": "missing"}})
		return
	}

	if !isAdminUser(authUser.Username) {
		respond.JSON(w, http.StatusOK, map[string]any{"error": "unauthorized"})
		return
	}

	if err := h.userPerms.AddProject(ctx, req.Username, req.ProjectSFID); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	eventData := fmt.Sprintf("User %s given permissions to project %s", req.Username, req.ProjectSFID)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:      "AddPermission",
		EventProjectID: req.ProjectSFID,
		EventData:      eventData,
		EventSummary:   eventData,
		ContainsPII:    true,
	})

	// Python returns None
	respond.JSON(w, http.StatusOK, nil)
}

// DELETE /v1/project/permission
// Python: cla/routes.py:2312 remove_project_permission()
// Calls: cla.controllers.project.remove_permission

func (h *Handlers) RemoveProjectPermissionV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	type request struct {
		Username    string `json:"username"`
		ProjectSFID string `json:"project_sfdc_id"`
	}
	var req request
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}
	if v, ok := flexibleStringParam(r, body, "username"); ok {
		req.Username = v
	}
	if v, ok := flexibleStringParam(r, body, "project_sfdc_id"); ok {
		req.ProjectSFID = v
	}

	if strings.TrimSpace(req.Username) == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"username": "missing"}})
		return
	}
	if strings.TrimSpace(req.ProjectSFID) == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"project_sfdc_id": "missing"}})
		return
	}

	if !isAdminUser(authUser.Username) {
		respond.JSON(w, http.StatusOK, map[string]any{"error": "unauthorized"})
		return
	}

	if err := h.userPerms.RemoveProject(ctx, req.Username, req.ProjectSFID); err != nil {
		// Mirror Python: return {'error': err} for load failures.
		respond.JSON(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}

	eventData := fmt.Sprintf("User %s permission removed to project %s", req.Username, req.ProjectSFID)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:      "RemovePermission",
		EventProjectID: req.ProjectSFID,
		EventData:      eventData,
		EventSummary:   eventData,
		ContainsPII:    true,
	})

	respond.JSON(w, http.StatusOK, nil)
}

// POST /v1/company/permission
// Python: cla/routes.py:2317 add_company_permission()
// Calls: cla.controllers.company.add_permission

func (h *Handlers) AddCompanyPermissionV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	type request struct {
		Username  string `json:"username"`
		CompanyID string `json:"company_id"`
	}
	var req request
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}
	if v, ok := flexibleStringParam(r, body, "username"); ok {
		req.Username = v
	}
	if v, ok := flexibleStringParam(r, body, "company_id"); ok {
		req.CompanyID = v
	}

	if strings.TrimSpace(req.Username) == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"username": "missing"}})
		return
	}
	if strings.TrimSpace(req.CompanyID) == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"company_id": "missing"}})
		return
	}

	if !isAdminUser(authUser.Username) {
		respond.JSON(w, http.StatusOK, map[string]any{"error": "unauthorized"})
		return
	}

	item, found, err := h.companies.GetByID(ctx, req.CompanyID)
	if err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"error": "Company not found"})
		return
	}

	acl := getAttrStringSlice(item, "company_acl")
	set := make(map[string]struct{}, len(acl)+1)
	for _, u := range acl {
		set[u] = struct{}{}
	}
	set[req.Username] = struct{}{}
	newACL := make([]string, 0, len(set))
	for u := range set {
		newACL = append(newACL, u)
	}
	sort.Strings(newACL)
	item["company_acl"] = &types.AttributeValueMemberSS{Value: newACL}
	item["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}

	if err := h.companies.PutItem(ctx, item); err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}

	companyName := getAttrString(item, "company_name")
	eventData := fmt.Sprintf("Added to user %s to Company %s permissions list.", req.Username, companyName)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:      "AddCompanyPermission",
		EventCompanyID: req.CompanyID,
		EventData:      eventData,
		EventSummary:   eventData,
		ContainsPII:    true,
	})

	// Python returns None
	respond.JSON(w, http.StatusOK, nil)
}

// DELETE /v1/company/permission
// Python: cla/routes.py:2322 remove_company_permission()
// Calls: cla.controllers.company.remove_permission

func (h *Handlers) RemoveCompanyPermissionV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	type request struct {
		Username  string `json:"username"`
		CompanyID string `json:"company_id"`
	}
	var req request
	body, err := parseFlexibleParams(r)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"body": "invalid json"}})
		return
	}
	if v, ok := flexibleStringParam(r, body, "username"); ok {
		req.Username = v
	}
	if v, ok := flexibleStringParam(r, body, "company_id"); ok {
		req.CompanyID = v
	}

	if strings.TrimSpace(req.Username) == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"username": "missing"}})
		return
	}
	if strings.TrimSpace(req.CompanyID) == "" {
		respond.JSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]any{"company_id": "missing"}})
		return
	}

	if !isAdminUser(authUser.Username) {
		respond.JSON(w, http.StatusOK, map[string]any{"error": "unauthorized"})
		return
	}

	item, found, err := h.companies.GetByID(ctx, req.CompanyID)
	if err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}
	if !found {
		respond.JSON(w, http.StatusOK, map[string]any{"error": "Company not found"})
		return
	}

	acl := getAttrStringSlice(item, "company_acl")
	set := make(map[string]struct{}, len(acl))
	for _, u := range acl {
		if u == req.Username {
			continue
		}
		set[u] = struct{}{}
	}
	newACL := make([]string, 0, len(set))
	for u := range set {
		newACL = append(newACL, u)
	}
	sort.Strings(newACL)
	item["company_acl"] = &types.AttributeValueMemberSS{Value: newACL}
	item["date_modified"] = &types.AttributeValueMemberS{Value: formatPynamoDateTimeUTC(time.Now().UTC())}

	if err := h.companies.PutItem(ctx, item); err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}

	companyName := getAttrString(item, "company_name")
	eventData := fmt.Sprintf("Removed user %s from Company %s permissions list.", req.Username, companyName)
	h.putAuditEventBestEffort(ctx, auditEventInput{
		EventType:      "RemoveCompanyPermission",
		EventCompanyID: req.CompanyID,
		EventData:      eventData,
		EventSummary:   eventData,
		ContainsPII:    true,
	})

	respond.JSON(w, http.StatusOK, nil)
}

// GET /v1/events
// Python: cla/routes.py:2327 search_events()
// Calls: cla.controllers.event.events

func (h *Handlers) SearchEventsV1(w http.ResponseWriter, r *http.Request) {
	// Port of legacy Python: cla.controllers.event.events()
	// - If request has query params: use Event.search_events(**params)
	//   - returns 404 + {"events": []} when the search yields no rows
	// - If request has no query params: return 200 + {"events": [...]} (may be empty)
	ctx := r.Context()

	items, err := h.events.ScanAll(ctx)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	requestHasParams := len(r.URL.Query()) > 0

	// Python Event.search_events() filters only on this attribute allowlist.
	allowedKeys := map[string]struct{}{
		"event_id":                 {},
		"event_company_id":         {},
		"event_project_id":         {},
		"event_type":               {},
		"event_user_id":            {},
		"event_project_name":       {},
		"event_company_name":       {},
		"event_project_name_lower": {},
		"event_company_name_lower": {},
		"event_time":               {},
		"event_time_epoch":         {},
	}

	filters := make(map[string]string)
	for key, vals := range r.URL.Query() {
		if _, ok := allowedKeys[key]; !ok {
			continue
		}
		if len(vals) == 0 {
			continue
		}
		filters[key] = vals[0]
	}

	events := make([]map[string]any, 0, len(items))
	for _, it := range items {
		m := store.ItemToInterfaceMap(it)

		// If query params are present but none are supported by the Python filter allowlist,
		// Python falls back to returning *all* events.
		if requestHasParams && len(filters) > 0 {
			match := true
			for k, want := range filters {
				got, ok := m[k]
				if !ok {
					match = false
					break
				}
				if s, ok := got.(string); ok {
					if s != want {
						match = false
						break
					}
					continue
				}
				if fmt.Sprint(got) != want {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		events = append(events, m)
	}

	if requestHasParams {
		if len(events) == 0 {
			respond.JSON(w, http.StatusNotFound, map[string]any{"events": []any{}})
			return
		}
		respond.JSON(w, http.StatusOK, map[string]any{"events": events})
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"events": events})
}

// GET /v1/events/{event_id}
// Python: cla/routes.py:2332 get_event()
// Calls: cla.controllers.event.get_event

func (h *Handlers) GetEventV1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eventID := chi.URLParam(r, "event_id")

	item, found, err := h.events.GetByID(ctx, eventID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}
	if !found {
		respond.JSON(w, http.StatusNotFound, map[string]any{"errors": map[string]any{"event_id": "Event not found"}})
		return
	}

	respond.JSON(w, http.StatusOK, store.ItemToInterfaceMap(item))
}

// GET /v2/user-from-session
// Python: cla/routes.py:2340 user_from_session()
// Calls: cla.controllers.repository_service.user_from_session

func (h *Handlers) UserFromSessionV2(w http.ResponseWriter, r *http.Request) {
	// Port of legacy Python: GET /v2/user-from-session
	// - cla/controllers/repository_service.py::user_from_session
	// - cla/models/github_models.py::user_from_session
	ctx := r.Context()
	sess := middleware.SessionFromContext(ctx)
	if sess == nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": "session middleware not initialized"})
		return
	}

	getRedirectURL := boolQuery(r.URL.Query().Get("get_redirect_url"))

	if sessionGetMap(sess, "github_oauth2_token") != nil {
		user, herr := h.githubGetOrCreateUser(ctx, sess)
		if herr != nil {
			respond.JSON(w, herr.status, herr.payload)
			return
		}
		respond.JSON(w, http.StatusOK, user)
		return
	}

	stateName := "user-from-session"
	authURL, csrf, _, err := h.githubAuthURLAndState(&stateName)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": err.Error()})
		return
	}
	// Python stores csrf token under github_oauth2_state.
	sessionSetString(sess, "github_oauth2_state", csrf)
	if getRedirectURL {
		respond.JSON(w, http.StatusAccepted, map[string]any{"redirect_url": authURL})
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

// GET /v2/user-from-token
// Python: cla/routes.py:2378 user_from_token()
// Calls: cla.controllers.user.get_or_create_user

func (h *Handlers) UserFromTokenV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authUser, authErrResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, authErrResp)
		return
	}

	item, _, err := h.getOrCreateUser(ctx, authUser)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]any{"errors": map[string]any{"server": err.Error()}})
		return
	}

	respond.JSON(w, http.StatusOK, store.ItemToInterfaceMap(item))
}

// POST /v2/clear-cache
// Python: cla/routes.py:2409 clear_cache()

func (h *Handlers) ClearCacheV2(w http.ResponseWriter, r *http.Request) {
	// Legacy Python requires a valid Bearer token and then clears GitHub caches.
	// In Go, we mirror the Python in-memory GitHub cache and clear it here.
	_, errResp, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, errResp)
		return
	}
	githublegacy.ClearCaches()

	status, hdr, respBody, err := h.doRequestToV4(r.Context(), http.MethodPost, "/clear-cache", headerCloneForV4(r.Header), nil)
	if err != nil {
		respond.JSON(w, http.StatusBadGateway, map[string]any{"errors": map[string]any{"v4": err.Error()}})
		return
	}
	if status >= 400 {
		respond.JSON(w, http.StatusBadGateway, map[string]any{"errors": map[string]any{"v4": string(respBody)}})
		return
	}

	copyV4ResponseHeaders(w, hdr)
	if len(respBody) > 0 {
		w.WriteHeader(status)
		_, _ = w.Write(respBody)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{"status": "OK"})
}

// POST /v1/events
// Python: cla/routes.py:2420 create_event()
// Calls: cla.controllers.event.create_event

func (h *Handlers) CreateEventV1(w http.ResponseWriter, r *http.Request) {
	// EASYCLA_PARITY_FLAG: in the provided legacy Python sources, cla.controllers.event does not define
	// create_event, but cla.routes defines the endpoint to call it. If invoked, Python errors with
	// AttributeError -> 500, so the default Go path preserves that behavior.
	respond.JSON(w, http.StatusInternalServerError, map[string]any{
		"errors": map[string]any{
			"server": "legacy python parity: cla.controllers.event.create_event is missing",
		},
	})
}

// ANY /v1/salesforce/projects
// Python: cla/salesforce.py:get_projects(event, context)
func (h *Handlers) SalesforceGetProjectsV1(w http.ResponseWriter, r *http.Request) {
	// Python parity: cla/salesforce.py:get_projects(event, context)
	// - auth errors => 401 'Error parsing Bearer token'
	// - invalid username => 400 'Error invalid username'
	// - not authorized => 403 'Error user not authorized to access projects'
	// - auth0 token failure => <status> 'Authentication failure'
	// - project-service failure => <status> 'Error retrieving projects'
	user, _, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, "Error parsing Bearer token")
		return
	}
	if h.userPerms == nil {
		respond.JSON(w, http.StatusInternalServerError, "user permissions store not configured")
		return
	}

	perms, err := h.userPerms.Get(r.Context(), user.Username)
	if err != nil {
		// Legacy python treats any load failure as an invalid username.
		respond.JSON(w, http.StatusBadRequest, "Error invalid username")
		return
	}
	if perms == nil || len(perms.Projects) == 0 {
		respond.JSON(w, http.StatusForbidden, "Error user not authorized to access projects")
		return
	}

	projects, status, err := h.salesforce.GetProjects(r.Context(), perms.Projects)
	if err != nil {
		var authErr *salesforce.AuthFailureError
		if errors.As(err, &authErr) {
			respond.JSON(w, status, "Authentication failure")
			return
		}
		respond.JSON(w, status, "Error retrieving projects")
		return
	}
	respond.JSON(w, status, projects)
}

// ANY /v1/salesforce/project
// Python: cla/salesforce.py:get_project(event, context)
func (h *Handlers) SalesforceGetProjectV1(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.URL.Query().Get("id"))
	if projectID == "" {
		respond.JSON(w, http.StatusBadRequest, "Missing project ID")
		return
	}

	user, _, err := h.authValidator.Authenticate(r.Header)
	if err != nil {
		respond.JSON(w, http.StatusUnauthorized, "Error parsing Bearer token")
		return
	}
	if h.userPerms == nil {
		respond.JSON(w, http.StatusInternalServerError, "user permissions store not configured")
		return
	}

	perms, err := h.userPerms.Get(r.Context(), user.Username)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, "Error invalid username")
		return
	}
	if perms == nil || len(perms.Projects) == 0 {
		respond.JSON(w, http.StatusForbidden, "Error user not authorized to access projects")
		return
	}

	allowed := false
	for _, pid := range perms.Projects {
		if strings.TrimSpace(pid) == projectID {
			allowed = true
			break
		}
	}
	if !allowed {
		respond.JSON(w, http.StatusForbidden, "Error user not authorized")
		return
	}

	project, status, err := h.salesforce.GetProject(r.Context(), projectID)
	if err != nil {
		var authErr *salesforce.AuthFailureError
		if errors.As(err, &authErr) {
			respond.JSON(w, status, "Authentication failure")
			return
		}
		respond.JSON(w, status, "Error retrieving project")
		return
	}
	respond.JSON(w, status, project)
}
