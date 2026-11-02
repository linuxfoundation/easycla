// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_activity

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 - GitHub still sends the sha1 X-Hub-Signature header
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func hmacHex(t *testing.T, key, payload string, sha256Sum bool) string {
	t.Helper()
	var mac = hmac.New(sha1.New, []byte(key))
	if sha256Sum {
		mac = hmac.New(sha256.New, []byte(key))
	}
	_, err := mac.Write([]byte(payload))
	assert.NoError(t, err)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestSignatureCheckMiddleware(t *testing.T) {
	const hmacKey = "unit-test-webhook-hmac-key"
	const body = `{"action":"created","installation":{"id":42}}`
	const otherBody = `{"action":"created","installation":{"id":43}}`

	type recorded struct {
		called bool
		body   string
	}

	newRequest := func(headers map[string]string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/v4/github/activity", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-GitHub-Event", "installation")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		return req
	}

	run := func(t *testing.T, req *http.Request) (*httptest.ResponseRecorder, *recorded) {
		t.Helper()
		rec := &recorded{}
		handler := signatureCheckMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec.called = true
			b, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			rec.body = string(b)
			w.WriteHeader(http.StatusOK)
		}))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w, rec
	}

	t.Run("fails closed when the webhook secret is not configured", func(t *testing.T) {
		t.Setenv("GITHUB_APP_WEBHOOK_SECRET", "")
		t.Setenv("GH_APP_WEBHOOK_SECRET", "")
		w, rec := run(t, newRequest(map[string]string{
			"X-Hub-Signature-256": "sha256=" + hmacHex(t, hmacKey, body, true),
		}))
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.False(t, rec.called)
	})

	t.Run("rejects an unsigned payload", func(t *testing.T) {
		t.Setenv("GITHUB_APP_WEBHOOK_SECRET", hmacKey)
		w, rec := run(t, newRequest(nil))
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.False(t, rec.called)
	})

	t.Run("rejects a payload signed with another secret", func(t *testing.T) {
		t.Setenv("GITHUB_APP_WEBHOOK_SECRET", hmacKey)
		w, rec := run(t, newRequest(map[string]string{
			"X-Hub-Signature-256": "sha256=" + hmacHex(t, "other-hmac-key", body, true),
			"X-Hub-Signature":     "sha1=" + hmacHex(t, "other-hmac-key", body, false),
		}))
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.False(t, rec.called)
	})

	t.Run("rejects a tampered body", func(t *testing.T) {
		t.Setenv("GITHUB_APP_WEBHOOK_SECRET", hmacKey)
		req := newRequest(map[string]string{
			"X-Hub-Signature-256": "sha256=" + hmacHex(t, hmacKey, body, true),
		})
		req.Body = io.NopCloser(strings.NewReader(otherBody))
		w, rec := run(t, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.False(t, rec.called)
	})

	t.Run("rejects a signature computed over a different payload", func(t *testing.T) {
		t.Setenv("GITHUB_APP_WEBHOOK_SECRET", hmacKey)
		w, rec := run(t, newRequest(map[string]string{
			"X-Hub-Signature-256": "sha256=" + hmacHex(t, hmacKey, otherBody, true),
			"X-Hub-Signature":     "sha1=" + hmacHex(t, hmacKey, otherBody, false),
		}))
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.False(t, rec.called)
	})

	t.Run("accepts a payload signed with the GitHub App secret (sha256 and sha1 headers)", func(t *testing.T) {
		t.Setenv("GITHUB_APP_WEBHOOK_SECRET", hmacKey)
		w, rec := run(t, newRequest(map[string]string{
			"X-Hub-Signature-256": "sha256=" + hmacHex(t, hmacKey, body, true),
			"X-Hub-Signature":     "sha1=" + hmacHex(t, hmacKey, body, false),
		}))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, rec.called)
		assert.Equal(t, body, rec.body, "raw payload must reach the swagger handler unchanged")
	})

	t.Run("accepts the sha1-only header the legacy API forwards", func(t *testing.T) {
		t.Setenv("GITHUB_APP_WEBHOOK_SECRET", "")
		t.Setenv("GH_APP_WEBHOOK_SECRET", hmacKey)
		w, rec := run(t, newRequest(map[string]string{
			"X-Hub-Signature": "sha1=" + hmacHex(t, hmacKey, body, false),
		}))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, rec.called)
	})
}
