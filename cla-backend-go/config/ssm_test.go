// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// a missing key is expected until the allow-list is provisioned, any other failure is not, so
// the two must stay distinguishable in the logs even when the SDK error arrives wrapped
func TestIsParameterNotFound(t *testing.T) {
	assert.True(t, isParameterNotFound(awserr.New(ssm.ErrCodeParameterNotFound, "not found", nil)))
	assert.True(t, isParameterNotFound(fmt.Errorf("wrapped: %w", awserr.New(ssm.ErrCodeParameterNotFound, "not found", nil))))
	assert.False(t, isParameterNotFound(awserr.New("AccessDeniedException", "denied", nil)))
	assert.False(t, isParameterNotFound(awserr.New("ThrottlingException", "slow down", nil)))
	assert.False(t, isParameterNotFound(errors.New("connection reset")))
}

func TestParseTrustedClientIDs(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		clientIDs []string
	}{
		{"unset", "", nil},
		{"blank", "   ", nil},
		{"separators only", " , ,, ", nil},
		{"single", "ss-client", []string{"ss-client"}},
		{"padded", "  ss-client  ", []string{"ss-client"}},
		{"multiple", "ss-client, other-client ,third-client", []string{"ss-client", "other-client", "third-client"}},
		{"trailing separator", "ss-client,", []string{"ss-client"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.clientIDs, parseTrustedClientIDs(test.value))
		})
	}
}

func TestLoadConfigRecordsUnreadableSSS(t *testing.T) {
	const configValue = "test-value"
	oldConfig := easyCLAConfig
	t.Cleanup(func() { easyCLAConfig = oldConfig })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Name  string
			Names []string
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		if strings.HasPrefix(input.Name, "cla-sss-") || len(input.Names) > 0 {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"__type":"ThrottlingException","message":"Rate exceeded"}`))
			assert.NoError(t, err)
			return
		}
		value := configValue
		if input.Name == "cla-acs-direct-roles-enabled-test" {
			value = "true"
		} else if strings.HasPrefix(input.Name, "cla-gh-app-id-") {
			value = "123"
		} else if strings.Contains(input.Name, "-enabled-") || strings.Contains(input.Name, "-enable-services-for-parent-") {
			value = "false"
		}
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"Parameter": map[string]string{"Name": input.Name, "Value": value, "Type": "String"},
		})
		assert.NoError(t, err)
	}))
	defer server.Close()
	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(server.URL),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		MaxRetries:  aws.Int(0),
	})
	require.NoError(t, err)
	cfg, err := LoadConfig("", awsSession, "test")
	require.NoError(t, err, "SSS configuration must not prevent non-screening workers from loading their configuration")
	assert.Equal(t, configValue, cfg.Auth0.Domain)
	assert.True(t, cfg.ACSDirectCLARolesEnabled)
	assert.False(t, cfg.MemberService.CreateCompanyEnabled)
	assert.Contains(t, cfg.SSS.ReadError, "ThrottlingException")
	assert.Equal(t, SSS{ReadError: cfg.SSS.ReadError}, cfg.SSS, "do not publish a partial screening policy")
}

func TestLoadOptionalSSSConfig(t *testing.T) {
	tests := []struct {
		name       string
		values     map[string]string
		failKey    string
		errorCode  string
		incomplete bool
		want       SSS
	}{
		{name: "not provisioned", want: SSS{Enabled: true}},
		{
			name: "configured and trimmed",
			values: map[string]string{
				"base-url": " https://sss.invalid ", "auth0-audience": " sss-audience ",
				"required": " true ", "enabled": "true",
			},
			want: SSS{BaseURL: "https://sss.invalid", Audience: "sss-audience", Required: true, Enabled: true},
		},
		{
			name:   "kill switch",
			values: map[string]string{"required": "true", "enabled": "false"},
			want:   SSS{Required: true},
		},
		{
			name:   "missing flags keep defaults",
			values: map[string]string{"base-url": "https://sss.invalid", "auth0-audience": "sss-audience"},
			want:   SSS{BaseURL: "https://sss.invalid", Audience: "sss-audience", Enabled: true},
		},
		{
			name:   "malformed flags keep defaults",
			values: map[string]string{"required": "invalid", "enabled": "invalid"},
			want:   SSS{Enabled: true},
		},
		{name: "incomplete success is not absence", failKey: "base-url", incomplete: true},
		{name: "access denied is not absence", failKey: "required", errorCode: "AccessDeniedException"},
	}
	for _, key := range []string{"base-url", "auth0-audience", "required", "enabled"} {
		tests = append(tests, struct {
			name       string
			values     map[string]string
			failKey    string
			errorCode  string
			incomplete bool
			want       SSS
		}{name: "throttled " + key, failKey: key, errorCode: "ThrottlingException"})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				assert.Equal(t, "AmazonSSM.GetParameter", r.Header.Get("X-Amz-Target"))
				var input ssm.GetParameterInput
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Error(err)
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				key := strings.TrimSuffix(strings.TrimPrefix(aws.StringValue(input.Name), "cla-sss-"), "-test")
				w.Header().Set("Content-Type", "application/x-amz-json-1.1")
				if key == tt.failKey && tt.incomplete {
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{}))
					return
				}
				value, found := tt.values[key]
				code := ""
				if key == tt.failKey {
					code = tt.errorCode
				} else if !found {
					code = ssm.ErrCodeParameterNotFound
				}
				if code != "" {
					w.WriteHeader(http.StatusBadRequest)
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{"__type": code, "message": "test SSM error"}))
					return
				}
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
					"Parameter": map[string]string{"Name": aws.StringValue(input.Name), "Value": value},
				}))
			}))
			defer server.Close()
			awsSession, err := session.NewSession(&aws.Config{
				Region:      aws.String("us-east-1"),
				Endpoint:    aws.String(server.URL),
				Credentials: credentials.NewStaticCredentials("test", "test", ""),
				MaxRetries:  aws.Int(0),
			})
			require.NoError(t, err)
			original := SSS{BaseURL: "unchanged", Required: true}
			cfg := Config{SSS: original}
			err = loadOptionalSSSConfig(ssm.New(awsSession), "test", &cfg, nil)
			if tt.failKey != "" {
				require.Error(t, err)
				assert.Equal(t, original, cfg.SSS, "do not publish partial or defaulted policy after a read failure")
				assert.Contains(t, err.Error(), tt.failKey)
				if tt.errorCode != "" {
					assert.Contains(t, err.Error(), tt.errorCode)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, cfg.SSS)
				assert.Equal(t, int32(4), calls.Load())
			}
		})
	}
}

// the publisher flag: absent = off, an unreadable parameter is an error (not a silent off), values are case-insensitive
func TestGetOptionalSSMBool(t *testing.T) {
	cases := []struct {
		name       string
		code       string
		value      string
		want       bool
		wantErr    string
		incomplete bool
	}{
		{name: "not provisioned", code: ssm.ErrCodeParameterNotFound},
		{name: "throttled", code: "ThrottlingException", wantErr: "ThrottlingException"},
		{name: "enabled", value: " True ", want: true},
		{name: "disabled", value: "False"},
		{name: "malformed", value: "yes", wantErr: "must be true or false"},
		{name: "empty", value: "", wantErr: "must be true or false"},
		{name: "numeric one", value: "1", wantErr: "must be true or false"},
		{name: "numeric zero", value: "0", wantErr: "must be true or false"},
		{name: "incomplete response", incomplete: true, wantErr: "incomplete response"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const key = "cla-member-service-cla-admins-enabled-test"
			got, err := getOptionalSSMBool(flagSSMClient(t, key, tc.code, tc.value, tc.incomplete), key, nil)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				assert.False(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// flagSSMClient answers every GetParameter with the given error code, an incomplete response or the value
func TestLoadCreateCompanyFlag(t *testing.T) {
	cases := []struct {
		name  string
		code  string
		value string
		want  bool
	}{
		{name: "not provisioned", code: ssm.ErrCodeParameterNotFound},
		{name: "enabled", value: "true", want: true},
		{name: "disabled", value: "false"},
		{name: "throttled stays off", code: "ThrottlingException"},
		{name: "malformed stays off", value: "1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			loadCreateCompanyFlag(flagSSMClient(t, "cla-member-service-create-company-enabled-test", tc.code, tc.value, false), "test", &cfg, nil)
			assert.Equal(t, tc.want, cfg.MemberService.CreateCompanyEnabled)
		})
	}
}

func TestLoadACSDirectRolesFlag(t *testing.T) {
	for _, tc := range []struct {
		name, code, value string
		incomplete, want  bool
		warn              bool
	}{
		{name: "not provisioned", code: ssm.ErrCodeParameterNotFound},
		{name: "enabled and trimmed", value: " TRUE ", want: true},
		{name: "disabled", value: "false"},
		{name: "denied", code: "AccessDeniedException", warn: true},
		{name: "throttled", code: "ThrottlingException", warn: true},
		{name: "malformed", value: "yes", warn: true},
		{name: "numeric", value: "1", warn: true},
		{name: "empty", warn: true},
		{name: "incomplete", incomplete: true, warn: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := log.GetLogger()
			oldLevel := logger.GetLevel()
			oldHooks := logger.ReplaceHooks(make(logrus.LevelHooks))
			logger.SetLevel(logrus.WarnLevel)
			t.Cleanup(func() {
				logger.ReplaceHooks(oldHooks)
				logger.SetLevel(oldLevel)
			})
			hook := logtest.NewLocal(logger)
			cfg := Config{ACSDirectCLARolesEnabled: true, MemberService: MemberService{CreateCompanyEnabled: true}}
			loadACSDirectRolesFlag(flagSSMClient(t, "cla-acs-direct-roles-enabled-test", tc.code, tc.value, tc.incomplete), "test", &cfg, nil)
			assert.Equal(t, tc.want, cfg.ACSDirectCLARolesEnabled)
			assert.True(t, cfg.MemberService.CreateCompanyEnabled, "the creation rollout must not be changed by the role flag")
			if tc.warn {
				require.Len(t, hook.AllEntries(), 1)
				assert.Contains(t, hook.LastEntry().Message, "switch unreadable")
			} else {
				assert.Empty(t, hook.AllEntries(), "an absent flag is not an operational failure")
			}
		})
	}
}

func flagSSMClient(t *testing.T, key, code, value string, incomplete bool) *ssm.SSM {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input ssm.GetParameterInput
		if err := json.NewDecoder(r.Body).Decode(&input); !assert.NoError(t, err) {
			http.Error(w, "invalid parameter request", http.StatusBadRequest)
			return
		}
		assert.Equal(t, key, aws.StringValue(input.Name))
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		if code != "" {
			w.WriteHeader(http.StatusBadRequest)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{"__type": code, "message": "test SSM error"}))
			return
		}
		if incomplete {
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{}))
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"Parameter": map[string]string{"Name": key, "Value": value},
		}))
	}))
	t.Cleanup(server.Close)
	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(server.URL),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		MaxRetries:  aws.Int(0),
	})
	require.NoError(t, err)
	return ssm.New(awsSession)
}

// the publisher switch never fails LoadConfig: a missing parameter is off, an unreadable or malformed one is
// off with the reason recorded for the publisher paths to refuse
func TestLoadCLAAdminsFlag(t *testing.T) {
	cases := []struct {
		name      string
		code      string
		value     string
		want      bool
		wantError string
	}{
		{name: "not provisioned", code: ssm.ErrCodeParameterNotFound},
		{name: "enabled", value: "true", want: true},
		{name: "throttled", code: "ThrottlingException", wantError: "ThrottlingException"},
		{name: "malformed", value: "yes", wantError: "must be true or false"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			loadCLAAdminsFlag(flagSSMClient(t, "cla-member-service-cla-admins-enabled-test", tc.code, tc.value, false), "test", &cfg, nil)
			assert.Equal(t, tc.want, cfg.MemberService.CLAAdminsEnabled)
			if tc.wantError == "" {
				assert.Empty(t, cfg.MemberService.CLAAdminsFlagError)
				return
			}
			assert.Contains(t, cfg.MemberService.CLAAdminsFlagError, tc.wantError)
		})
	}
}

const acsDirectRolesTestKey = "cla-acs-direct-roles-enabled-test"

// fakeParameterStore serves GetParameter and GetParameters and records the calls of each kind
type fakeParameterStore struct {
	missing     map[string]bool
	values      map[string]string
	batchError  string
	failBatchOf string

	mu          sync.Mutex
	batchSizes  []int
	failedSize  int
	batchNames  []string
	singleNames []string
}

func (f *fakeParameterStore) value(name string) (string, bool) {
	if f.missing[name] {
		return "", false
	}
	if value, ok := f.values[name]; ok {
		return value, true
	}
	switch {
	case strings.HasPrefix(name, "cla-gh-app-id-"):
		return "123", true
	case strings.Contains(name, "-enabled-"), strings.Contains(name, "-enable-services-for-parent-"), strings.Contains(name, "cla-sss-required-"):
		return strconv.FormatBool(false), true
	}
	return "test-value", true
}

func (f *fakeParameterStore) session(t *testing.T) *session.Session {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Name  string
			Names []string
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		fail := func(code string) {
			w.WriteHeader(http.StatusBadRequest)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{"__type": code, "message": "test SSM error"}))
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Header.Get("X-Amz-Target") == "AmazonSSM.GetParameters" {
			f.batchSizes = append(f.batchSizes, len(input.Names))
			f.batchNames = append(f.batchNames, input.Names...)
			code := f.batchError
			for _, name := range input.Names {
				if name == f.failBatchOf {
					code = "ThrottlingException"
					f.failedSize = len(input.Names)
				}
			}
			if code != "" {
				fail(code)
				return
			}
			parameters, invalid := []map[string]string{}, []string{}
			for _, name := range input.Names {
				if value, ok := f.value(name); ok {
					parameters = append(parameters, map[string]string{"Name": name, "Value": value, "Type": "String"})
				} else {
					invalid = append(invalid, name)
				}
			}
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"Parameters": parameters, "InvalidParameters": invalid}))
			return
		}
		f.singleNames = append(f.singleNames, input.Name)
		value, ok := f.value(input.Name)
		if !ok {
			fail(ssm.ErrCodeParameterNotFound)
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"Parameter": map[string]string{"Name": input.Name, "Value": value, "Type": "String"},
		}))
	}))
	t.Cleanup(server.Close)
	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(server.URL),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		MaxRetries:  aws.Int(0),
	})
	require.NoError(t, err)
	return awsSession
}

func rolloutParameterStore() *fakeParameterStore {
	return &fakeParameterStore{
		missing: map[string]bool{"cla-org-lens-base-test": true, "cla-ss-trusted-client-ids-test": true},
		values: map[string]string{
			"cla-member-service-create-company-enabled-test": "true",
			"cla-acs-direct-roles-enabled-test":              "true",
			"cla-sss-base-url-test":                          "https://sss.invalid",
			"cla-sss-auth0-audience-test":                    "sss-audience",
			"cla-sss-enabled-test":                           "true",
			"cla-member-service-base-url-test":               "https://member-service.invalid",
		},
	}
}

func assertRolloutConfig(t *testing.T, cfg Config) {
	t.Helper()
	assert.Equal(t, "test-value", cfg.Auth0.Domain)
	assert.Equal(t, 123, cfg.GitHub.AppID)
	assert.Equal(t, "test-value", cfg.DocuSignPrivateKey)
	assert.True(t, cfg.MemberService.CreateCompanyEnabled)
	assert.True(t, cfg.ACSDirectCLARolesEnabled)
	assert.False(t, cfg.MemberService.CLAAdminsEnabled)
	assert.Empty(t, cfg.MemberService.CLAAdminsFlagError)
	assert.Equal(t, "https://member-service.invalid", cfg.MemberService.BaseURL)
	assert.Equal(t, "", cfg.OrgLensURL)
	assert.Nil(t, cfg.SelfServe.TrustedClientIDs)
	assert.Equal(t, SSS{BaseURL: "https://sss.invalid", Audience: "sss-audience", Enabled: true}, cfg.SSS)
}

// easycla#5311: one GetParameter call per name at every cold start exceeded the Parameter Store rate limit
func TestLoadConfigReadsParametersInBatches(t *testing.T) {
	oldConfig := easyCLAConfig
	t.Cleanup(func() { easyCLAConfig = oldConfig })
	store := rolloutParameterStore()

	cfg, err := LoadConfig("", store.session(t), "test")
	require.NoError(t, err)
	assertRolloutConfig(t, cfg)

	assert.Empty(t, store.singleNames, "no parameter needs a call of its own")
	names := map[string]bool{}
	for _, name := range store.batchNames {
		assert.False(t, names[name], "%s is read twice", name)
		names[name] = true
	}
	assert.True(t, names["cla-member-service-create-company-enabled-test"])
	assert.True(t, names["cla-acs-direct-roles-enabled-test"])
	assert.GreaterOrEqual(t, len(names), 60)
	assert.Len(t, store.batchSizes, (len(names)+9)/10)
	for _, size := range store.batchSizes {
		assert.LessOrEqual(t, size, 10)
	}
}

// lfx-self-serve-ops#1668: three debug lines printed the first and the last four characters of the GitLab values
func TestLoadConfigLogsNoPartOfTheGitLabValues(t *testing.T) {
	oldConfig := easyCLAConfig
	t.Cleanup(func() { easyCLAConfig = oldConfig })
	store := rolloutParameterStore()
	values := map[string]string{
		"cla-gitlab-app-id-test":          "AAAAgitlab-app-idZZZZ",
		"cla-gitlab-app-secret-test":      "BBBBgitlab-app-secretYYYY",
		"cla-gitlab-app-private-key-test": "CCCCgitlab-app-private-keyXXXX",
	}
	for name, value := range values {
		store.values[name] = value
	}
	logger := log.GetLogger()
	oldLevel := logger.GetLevel()
	oldHooks := logger.ReplaceHooks(make(logrus.LevelHooks))
	logger.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() {
		logger.ReplaceHooks(oldHooks)
		logger.SetLevel(oldLevel)
	})
	hook := logtest.NewLocal(logger)

	cfg, err := LoadConfig("", store.session(t), "test")
	require.NoError(t, err)
	assert.Equal(t, values["cla-gitlab-app-id-test"], cfg.Gitlab.AppClientID)
	assert.Equal(t, values["cla-gitlab-app-secret-test"], cfg.Gitlab.AppClientSecret)
	assert.Equal(t, values["cla-gitlab-app-private-key-test"], cfg.Gitlab.AppPrivateKey)

	gitLabLines := 0
	for _, entry := range hook.AllEntries() {
		if strings.Contains(entry.Message, "CLA GitLab") {
			gitLabLines++
		}
		for _, value := range values {
			assert.NotContains(t, entry.Message, value[:4])
			assert.NotContains(t, entry.Message, value[len(value)-4:])
		}
	}
	assert.Equal(t, 3, gitLabLines, "the lines still say that the three values were read")
}

func TestLoadConfigReadsOneByOneWhenBatchedReadsAreDenied(t *testing.T) {
	oldConfig := easyCLAConfig
	t.Cleanup(func() { easyCLAConfig = oldConfig })
	store := rolloutParameterStore()
	store.batchError = "AccessDeniedException"

	cfg, err := LoadConfig("", store.session(t), "test")
	require.NoError(t, err)
	assertRolloutConfig(t, cfg)
	assert.NotEmpty(t, store.batchSizes)
	assert.Len(t, store.singleNames, len(store.batchNames), "every parameter is read on its own once")
}

func TestLoadConfigReadsOnlyTheFailedBatchOneByOne(t *testing.T) {
	oldConfig := easyCLAConfig
	t.Cleanup(func() { easyCLAConfig = oldConfig })
	store := rolloutParameterStore()
	store.failBatchOf = acsDirectRolesTestKey

	cfg, err := LoadConfig("", store.session(t), "test")
	require.NoError(t, err)
	assertRolloutConfig(t, cfg)
	require.NotZero(t, store.failedSize)
	assert.Len(t, store.singleNames, store.failedSize)
	assert.Contains(t, store.singleNames, acsDirectRolesTestKey)
}

func TestBatchedParametersGetParameter(t *testing.T) {
	store := &fakeParameterStore{}
	batched := &batchedParameters{
		client:   ssm.New(store.session(t)),
		found:    map[string]*ssm.Parameter{"cla-found-test": {Name: aws.String("cla-found-test"), Value: aws.String("batched")}},
		notFound: map[string]bool{"cla-missing-test": true},
	}

	for _, name := range []string{"cla-found-test", "/cla-found-test"} {
		out, err := batched.GetParameter(&ssm.GetParameterInput{Name: aws.String(name)})
		require.NoError(t, err)
		assert.Equal(t, "batched", aws.StringValue(out.Parameter.Value))
	}
	_, err := batched.GetParameter(&ssm.GetParameterInput{Name: aws.String("cla-missing-test")})
	assert.True(t, isParameterNotFound(err))
	assert.Empty(t, store.singleNames)

	out, err := batched.GetParameter(&ssm.GetParameterInput{Name: aws.String("cla-other-test")})
	require.NoError(t, err)
	assert.Equal(t, "test-value", aws.StringValue(out.Parameter.Value))
	assert.Equal(t, []string{"cla-other-test"}, store.singleNames)
}
