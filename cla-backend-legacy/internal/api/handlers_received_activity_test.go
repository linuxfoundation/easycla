// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 - GitHub's legacy X-Hub-Signature header is sha1
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
)

// recordingV4Transport stands in for the v4 backend and counts trigger calls.
type recordingV4Transport struct {
	calls int32
	paths []string
}

func (t *recordingV4Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(&t.calls, 1)
	t.paths = append(t.paths, req.URL.RequestURI())
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"status":"OK"}`)),
		Request:    req,
	}, nil
}

func sha1Signature(secret string, body []byte) string {
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestReceivedActivityV2RequiresWebhookSignature(t *testing.T) {
	const secret = "legacy-unit-test-webhook-secret"
	t.Setenv("GITHUB_APP_WEBHOOK_SECRET", secret)
	t.Setenv("GH_APP_WEBHOOK_SECRET", "")
	t.Setenv("PLATFORM_GATEWAY_URL", "http://v4.invalid/cla-service/v4")

	prOpened := []byte(`{"action":"opened","installation":{"id":1001},"repository":{"id":2002},"pull_request":{"number":7,"head":{"sha":"0123456789abcdef0123456789abcdef01234567"}}}`)
	mergeGroup := []byte(`{"action":"checks_requested","installation":{"id":1001},"repository":{"id":2002},"merge_group":{"head_sha":"89abcdef0123456789abcdef0123456789abcdef","head_commit":{"message":"Merge pull request #7 from fork/branch"}}}`)
	prClosed := []byte(`{"action":"closed","installation":{"id":1001},"repository":{"id":2002},"pull_request":{"number":7}}`)

	do := func(t *testing.T, provider string, body []byte, headers map[string]string) (*httptest.ResponseRecorder, *recordingV4Transport) {
		t.Helper()
		transport := &recordingV4Transport{}
		h := &Handlers{v4HTTPClient: &http.Client{Transport: transport}}
		router := chi.NewRouter()
		router.Post("/v2/repository-provider/{provider}/activity", h.ReceivedActivityV2)

		req := httptest.NewRequest(http.MethodPost, "/v2/repository-provider/"+provider+"/activity", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec, transport
	}

	assertRejected := func(t *testing.T, rec *httptest.ResponseRecorder, transport *recordingV4Transport) {
		t.Helper()
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusUnauthorized, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "Invalid Secret Token") {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
		if n := atomic.LoadInt32(&transport.calls); n != 0 {
			t.Fatalf("v4 trigger calls = %d, want 0 (%v)", n, transport.paths)
		}
	}

	t.Run("unsigned pull_request payload is rejected without any v4 trigger", func(t *testing.T) {
		rec, transport := do(t, "github", prOpened, nil)
		assertRejected(t, rec, transport)
	})

	t.Run("unsigned merge_group payload is rejected without any v4 trigger", func(t *testing.T) {
		rec, transport := do(t, "github", mergeGroup, nil)
		assertRejected(t, rec, transport)
	})

	t.Run("payload signed with another secret is rejected", func(t *testing.T) {
		rec, transport := do(t, "github", prOpened, map[string]string{"X-Hub-Signature": sha1Signature("not-the-secret", prOpened)})
		assertRejected(t, rec, transport)
	})

	t.Run("signature over a different body is rejected", func(t *testing.T) {
		rec, transport := do(t, "github", prOpened, map[string]string{"X-Hub-Signature": sha1Signature(secret, prClosed)})
		assertRejected(t, rec, transport)
	})

	t.Run("properly signed pull_request payload still triggers the v4 PR check", func(t *testing.T) {
		rec, transport := do(t, "github", prOpened, map[string]string{"X-Hub-Signature": sha1Signature(secret, prOpened)})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if n := atomic.LoadInt32(&transport.calls); n != 1 {
			t.Fatalf("v4 trigger calls = %d, want 1 (%v)", n, transport.paths)
		}
		if !strings.Contains(transport.paths[0], "/github/activity?legacy_internal_trigger=github-change-request") {
			t.Fatalf("unexpected v4 path: %s", transport.paths[0])
		}
	})

	t.Run("properly signed merge_group payload still triggers the v4 merge-group check", func(t *testing.T) {
		rec, transport := do(t, "github", mergeGroup, map[string]string{"X-Hub-Signature": sha1Signature(secret, mergeGroup)})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if n := atomic.LoadInt32(&transport.calls); n != 1 {
			t.Fatalf("v4 trigger calls = %d, want 1 (%v)", n, transport.paths)
		}
	})

	t.Run("side-effect free responses are unchanged for unsigned requests", func(t *testing.T) {
		rec, transport := do(t, "github", []byte(`{"zen":"keep it logically awesome"}`), nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Not a pull request nor a merge group") {
			t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
		}
		rec, _ = do(t, "mock_github", prOpened, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("mock_github status = %d, want %d", rec.Code, http.StatusOK)
		}
		rec, _ = do(t, "github", []byte(`{"malformed":`), nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("malformed body status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
		if n := atomic.LoadInt32(&transport.calls); n != 0 {
			t.Fatalf("v4 trigger calls = %d, want 0", n)
		}
	})

	t.Run("fails closed when the webhook secret is not configured", func(t *testing.T) {
		t.Setenv("GITHUB_APP_WEBHOOK_SECRET", "")
		rec, transport := do(t, "github", prOpened, map[string]string{"X-Hub-Signature": sha1Signature(secret, prOpened)})
		assertRejected(t, rec, transport)
	})
}
