// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

func TestRecoverHandlerAnswers500ForAPanic(t *testing.T) {
	var missing *struct{ name string }
	for name, handler := range map[string]http.HandlerFunc{
		"explicit panic":  func(http.ResponseWriter, *http.Request) { panic("boom") },
		"nil dereference": func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("X-Name", missing.name) },
		"index out of range": func(w http.ResponseWriter, _ *http.Request) {
			var none []string
			w.Header().Set("X-Name", none[len(none)])
		},
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			assert.NotPanics(t, func() {
				recoverHandler(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v4/anything", nil))
			})
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.JSONEq(t, `{"Code":"500","Message":"internal server error"}`, rec.Body.String())
		})
	}
}

func TestRecoverHandlerLeavesAnAnswerAlone(t *testing.T) {
	handler := recoverHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusTeapot)
		if _, err := w.Write([]byte("answer")); err != nil {
			t.Error(err)
		}
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v3/ops/health", nil))

	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Equal(t, "answer", rec.Body.String())
}

func TestRecoverHandlerClearsTheRequestMetricsOfAPanic(t *testing.T) {
	const reqID = "recover-handler-test-request"
	handler := recoverHandler(setRequestIDHandler(responseLoggingMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))))
	req := httptest.NewRequest(http.MethodGet, "/v4/anything", nil)
	req.Header.Set(utils.XREQUESTID, reqID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Nil(t, getRequestMetrics(reqID), "the metrics entry of the request is removed")
}
