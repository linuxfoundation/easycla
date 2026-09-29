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

func TestIsSalesforceID(t *testing.T) {
	for id, want := range map[string]bool{
		"0014100000Te0yqQAB":                   true,
		"0014100000Te0yq":                      true,
		"9b8e7d66-40a5-4cde-9f00-3e1d1a2b3c4d": false,
		"0014100000Te0y":                       false,
		"0014100000Te0yqQAB1":                  false,
		"0014100000Te0yq-AB":                   false,
		"":                                     false,
	} {
		if got := isSalesforceID(id); got != want {
			t.Errorf("isSalesforceID(%q) = %v, want %v", id, got, want)
		}
	}
}

func employeeSignatureBody(companyID string) string {
	return `{"project_id":"9b8e7d66-40a5-4cde-9f00-3e1d1a2b3c4d","company_id":"` + companyID +
		`","user_id":"7f6f0b4a-1f9c-4d3e-8a2b-0c1d2e3f4a5b","return_url_type":"github"}`
}

func decodeErrors(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	errs, _ := body["errors"].(map[string]any)
	return errs
}

// A uuid or a Salesforce ID passes the company_id shape check of both employee-signature endpoints (the
// precheck then resolves it); any other non-uuid value is still rejected as before.
func TestEmployeeSignatureV2AcceptsASalesforceCompanyID(t *testing.T) {
	handlers := map[string]func(*Handlers, http.ResponseWriter, *http.Request){
		"check-prepare": (*Handlers).CheckAndPrepareEmployeeSignatureV2,
		"request":       (*Handlers).RequestEmployeeSignatureV2,
	}
	for name, handle := range handlers {
		for shape, companyID := range map[string]string{"uuid": "9b8e7d66-40a5-4cde-9f00-3e1d1a2b3c4d", "sfid": "0014100000Te0yqQAB"} {
			t.Run(name+"/"+shape, func(t *testing.T) {
				h := &Handlers{}
				req := httptest.NewRequest(http.MethodPost, "/v2/x", strings.NewReader(employeeSignatureBody(companyID)))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				handle(h, rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
				}
				if errs := decodeErrors(t, rec); errs["company_id"] != nil || errs["server"] == nil {
					t.Fatalf("expected the request to reach the precheck, got %v", errs)
				}
			})
		}
		t.Run(name+"/other shape", func(t *testing.T) {
			h := &Handlers{}
			req := httptest.NewRequest(http.MethodPost, "/v2/x", strings.NewReader(employeeSignatureBody("lf-not-a-company")))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handle(h, rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if errs := decodeErrors(t, rec); errs["company_id"] != "invalid uuid" {
				t.Fatalf("expected company_id rejected, got %v", errs)
			}
		})
	}
}
