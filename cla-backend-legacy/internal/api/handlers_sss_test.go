// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmTypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func TestNewHandlersSurvivesUnreadableSSS(t *testing.T) {
	if mode := os.Getenv("EASYCLA_TEST_SSS_STARTUP"); mode != "" {
		h := NewHandlers()
		if mode == "required-missing" {
			t.Fatal("startup returned for invalid required configuration")
		}
		router := NewRouter(h)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v2/health", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("SSS startup failure broke health: status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		transport := &recordingV4Transport{}
		h.v4HTTPClient = &http.Client{Transport: transport}
		body := []byte(`{"action":"opened","installation":{"id":1001},"repository":{"id":2002},"pull_request":{"number":7,"head":{"sha":"0123456789abcdef0123456789abcdef01234567"}}}`)
		webhook := httptest.NewRequest(http.MethodPost, "/v2/github/activity", bytes.NewReader(body))
		webhook.Header.Set("X-GitHub-Event", "pull_request")
		webhook.Header.Set("X-Hub-Signature", sha1Signature("test-webhook-secret", body))
		recorder = httptest.NewRecorder()
		router.ServeHTTP(recorder, webhook)
		if recorder.Code != http.StatusOK || atomic.LoadInt32(&transport.calls) != 1 {
			t.Fatalf("SSS startup failure broke PR check: status=%d calls=%d body=%s", recorder.Code, transport.calls, recorder.Body.String())
		}
		_, err := h.checkCompanyCompliance(context.Background(), map[string]types.AttributeValue{
			"company_id": &types.AttributeValueMemberS{Value: "test-company"},
		})
		if (err != nil) != (mode == "unreadable") {
			t.Fatalf("unexpected signing policy for %s: %v", mode, err)
		}
		return
	}

	for _, mode := range []string{"unreadable", "not-provisioned", "required-missing"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var input struct{ Name string }
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/x-amz-json-1.1")
				if mode != "not-provisioned" && (strings.Contains(input.Name, "required") || strings.Contains(input.Name, "enabled")) {
					if err := json.NewEncoder(w).Encode(map[string]interface{}{
						"Parameter": map[string]string{"Name": input.Name, "Value": "true", "Type": "String"},
					}); err != nil {
						t.Error(err)
					}
					return
				}
				code := "ParameterNotFound"
				if mode == "unreadable" {
					code = "ThrottlingException"
				}
				w.WriteHeader(http.StatusBadRequest)
				if err := json.NewEncoder(w).Encode(map[string]string{"__type": code, "message": "test SSM error"}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			t.Setenv("EASYCLA_TEST_SSS_STARTUP", mode)
			t.Setenv("STAGE", "test")
			t.Setenv("AWS_REGION", "us-east-1")
			t.Setenv("DYNAMODB_AWS_REGION", "us-east-1")
			t.Setenv("AWS_ACCESS_KEY_ID", "test")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
			t.Setenv("AWS_SESSION_TOKEN", "")
			t.Setenv("AWS_ENDPOINT_URL_SSM", server.URL)
			t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
			t.Setenv("AWS_MAX_ATTEMPTS", "1")
			t.Setenv("PLATFORM_MAINTAINERS", "")
			t.Setenv("PLATFORM_GATEWAY_URL", "http://v4.invalid/cla-service/v4")
			t.Setenv("GITHUB_APP_WEBHOOK_SECRET", "test-webhook-secret")
			t.Setenv("DDB_API_LOGGING", "false")
			t.Setenv("OTEL_DATADOG_API_LOGGING", "false")
			cmd := exec.Command(os.Args[0], "-test.run=^TestNewHandlersSurvivesUnreadableSSS$", "-test.v")
			output, err := cmd.CombinedOutput()
			if mode == "required-missing" {
				if err == nil || !strings.Contains(string(output), "SSS is required but not configured") {
					t.Fatalf("real required misconfiguration must still terminate startup: %v\n%s", err, output)
				}
			} else if err != nil {
				t.Fatalf("SSS startup terminated the service: %v\n%s", err, output)
			}
			wantCalls := int32(4)
			if mode == "unreadable" {
				wantCalls = 2
			}
			if calls.Load() != wantCalls {
				t.Fatalf("SSM calls = %d, want %d (health/webhook must not retry SSS)", calls.Load(), wantCalls)
			}
		})
	}
}

type sssParameterFunc func(context.Context, *ssm.GetParameterInput, ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)

func (f sssParameterFunc) GetParameter(ctx context.Context, in *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	return f(ctx, in, opts...)
}

func sssTestParameter(in *ssm.GetParameterInput, values map[string]string) (*ssm.GetParameterOutput, error) {
	key := strings.TrimSuffix(strings.TrimPrefix(aws.ToString(in.Name), "cla-sss-"), "-test")
	value, ok := values[key]
	if !ok {
		return nil, fmt.Errorf("wrapped: %w", &ssmTypes.ParameterNotFound{})
	}
	return &ssm.GetParameterOutput{Parameter: &ssmTypes.Parameter{Value: &value}}, nil
}

func TestLoadSSSConfig(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   sssConfiguration
	}{
		{name: "not provisioned", want: sssConfiguration{enabled: true}},
		{
			name: "configured and trimmed",
			values: map[string]string{
				"base-url": " https://sss.invalid ", "auth0-audience": " sss-audience ",
				"required": " true ", "enabled": "true",
			},
			want: sssConfiguration{baseURL: "https://sss.invalid", audience: "sss-audience", required: true, enabled: true},
		},
		{name: "kill switch", values: map[string]string{"required": "true", "enabled": "false"}, want: sssConfiguration{required: true}},
		{
			name:   "missing flags keep defaults",
			values: map[string]string{"base-url": "https://sss.invalid", "auth0-audience": "sss-audience"},
			want:   sssConfiguration{baseURL: "https://sss.invalid", audience: "sss-audience", enabled: true},
		},
		{name: "malformed flags keep defaults", values: map[string]string{"required": "invalid", "enabled": "invalid"}, want: sssConfiguration{enabled: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := sssParameterFunc(func(_ context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
				calls++
				return sssTestParameter(in, tt.values)
			})
			got, err := loadSSSConfig(context.Background(), client, "test")
			if err != nil || got != tt.want || calls != 4 {
				t.Fatalf("config=%+v error=%v calls=%d, want %+v and 4 calls", got, err, calls, tt.want)
			}
		})
	}
	for _, key := range []string{"base-url", "auth0-audience", "required", "enabled"} {
		t.Run("unreadable "+key, func(t *testing.T) {
			readErr := errors.New("SSM throttled")
			client := sssParameterFunc(func(_ context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
				if aws.ToString(in.Name) == "cla-sss-"+key+"-test" {
					return nil, readErr
				}
				return sssTestParameter(in, nil)
			})
			_, err := loadSSSConfig(context.Background(), client, "test")
			if !errors.Is(err, readErr) {
				t.Fatalf("unreadable %s became optional: %v", key, err)
			}
		})
	}
	t.Run("incomplete success is not absence", func(t *testing.T) {
		client := sssParameterFunc(func(context.Context, *ssm.GetParameterInput, ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return &ssm.GetParameterOutput{}, nil
		})
		if _, err := loadSSSConfig(context.Background(), client, "test"); err == nil {
			t.Fatal("incomplete SSM response was treated as an absent parameter")
		}
	})
}

func TestSSSConfigRecovery(t *testing.T) {
	readErr := errors.New("SSM throttled")
	calls := 0
	h := &Handlers{sssConfigStage: "test"}
	h.sssConfigClient = sssParameterFunc(func(_ context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
		calls++
		if calls <= 2 {
			return nil, readErr
		}
		return sssTestParameter(in, map[string]string{"required": "true", "enabled": "false"})
	})
	if err := h.initializeSSS(context.Background()); !errors.Is(err, readErr) || !h.sssRequired || !h.sssEnabled {
		t.Fatalf("startup failure must retain a required, enabled pending policy: %v", err)
	}
	for _, origin := range []string{"", "admin"} {
		blocked, err := h.checkCompanyCompliance(context.Background(), map[string]types.AttributeValue{
			"is_sanctioned":   &types.AttributeValueMemberBOOL{Value: true},
			"sanction_origin": &types.AttributeValueMemberS{Value: origin},
		})
		if !blocked || err != nil || calls != 1 {
			t.Fatalf("manual sanction must block without configuration recovery: blocked=%t err=%v calls=%d", blocked, err, calls)
		}
	}
	if _, err := h.checkCompanyCompliance(context.Background(), nil); !errors.Is(err, readErr) || calls != 2 {
		t.Fatalf("first signing retry must still fail closed: %v (calls=%d)", err, calls)
	}
	if _, err := h.checkCompanyCompliance(context.Background(), nil); !errors.Is(err, readErr) || calls != 2 {
		t.Fatalf("cached recovery failure must block without another SSM read: %v (calls=%d)", err, calls)
	}
	if !time.Now().Before(h.sssConfigRetryAfter) {
		t.Fatal("failed recovery did not arm the retry window")
	}
	h.sssConfigRetryAfter = time.Now().Add(-time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			blocked, err := h.checkCompanyCompliance(context.Background(), map[string]types.AttributeValue{
				"is_sanctioned":   &types.AttributeValueMemberBOOL{Value: true},
				"sanction_origin": &types.AttributeValueMemberS{Value: "sss"},
			})
			if blocked || err != nil {
				t.Errorf("recovered kill switch was not observed: blocked=%t err=%v", blocked, err)
			}
		}()
	}
	wg.Wait()
	if calls != 6 || h.sssConfigErr != nil || !h.sssRequired || h.sssEnabled {
		t.Fatalf("configuration must recover once, without racing or further reads: calls=%d pending=%v", calls, h.sssConfigErr)
	}
}

func TestSSSRecoveryRetainsRequiredPolicy(t *testing.T) {
	t.Setenv("PLATFORM_AUTH0_URL", "https://auth.invalid/oauth/token")
	t.Setenv("PLATFORM_AUTH0_CLIENT_ID", "test-client")
	t.Setenv("PLATFORM_AUTH0_CLIENT_SECRET", "test-secret")
	values := map[string]string{"required": "true"}
	calls := 0
	h := &Handlers{
		sssConfigStage: "test",
		sssConfigErr:   errors.New("startup SSM error"),
		sssRequired:    true,
		sssEnabled:     true,
		sssConfigClient: sssParameterFunc(func(ctx context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			calls++
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return sssTestParameter(in, values)
		}),
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.checkCompanyCompliance(canceled, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("configuration retry did not use the request context: %v", err)
	}
	if !h.sssConfigRetryAfter.IsZero() {
		t.Fatal("a canceled request must not delay recovery for other requests")
	}
	if _, err := h.checkCompanyCompliance(context.Background(), nil); err == nil || errors.Is(err, context.Canceled) || h.sssConfigErr == nil {
		t.Fatalf("required missing configuration must remain pending and return an error, not exit or disable SSS: %v", err)
	}
	if !time.Now().Before(h.sssConfigRetryAfter) {
		t.Fatal("missing required configuration did not arm the retry window")
	}
	previousCalls := calls
	pendingErr := h.sssConfigErr
	if err := h.retrySSSConfig(context.Background()); !errors.Is(err, pendingErr) || calls != previousCalls {
		t.Fatalf("cached configuration error must not re-read SSM: %v (calls=%d, want %d)", err, calls, previousCalls)
	}
	h.sssConfigRetryAfter = time.Now().Add(-time.Second)
	values["base-url"] = "https://sss.invalid"
	values["auth0-audience"] = "sss-audience"
	if err := h.retrySSSConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.sssClient == nil || h.sssConfigErr != nil || !h.sssRequired || !h.sssEnabled {
		t.Fatal("required client was not recovered")
	}
	if calls != previousCalls+4 {
		t.Fatalf("expired retry window must reload exactly once: calls=%d, want %d", calls, previousCalls+4)
	}
	if _, err := h.checkCompanyCompliance(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "external ID") {
		t.Fatalf("recovered required screening must still enforce its prerequisites: %v", err)
	}
}
