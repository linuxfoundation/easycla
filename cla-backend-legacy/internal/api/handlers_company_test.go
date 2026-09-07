// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPostCompanyV1_Retired locks in the #2055 fix: legacy company creation
// is permanently gone, unconditionally, so no caller can persist a company
// row without company_external_id through POST /v1/company.
func TestPostCompanyV1_Retired(t *testing.T) {
	h := &Handlers{} // zero-value: no store/auth deps needed post-retirement
	req := httptest.NewRequest(http.MethodPost, "/v1/company", strings.NewReader(`{"company_name":"Acme"}`))
	rec := httptest.NewRecorder()

	h.PostCompanyV1(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGone)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if _, ok := body["message"]; !ok {
		t.Fatalf("body missing message key: %v", body)
	}
}

// TestPostCompanyV1_RetiredIgnoresAuth asserts the 410 is unconditional —
// no Authorization header, no body — so behavior can't vary by auth state.
func TestPostCompanyV1_RetiredIgnoresAuth(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodPost, "/v1/company", nil)
	rec := httptest.NewRecorder()

	h.PostCompanyV1(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGone)
	}
}
