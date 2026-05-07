// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package pdf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DocRaptorGenerator implements a small subset of DocRaptor's "Create Doc" API.
//
// Legacy Python equivalent: cla.models.docraptor_models.DocRaptor.generate.
type DocRaptorGenerator struct {
	apiKey     string
	baseURL    string
	testMode   bool
	httpClient *http.Client
}

// NewDocRaptorFromEnv constructs a DocRaptor generator using environment variables.
//
// Required:
//   - DOCRAPTOR_API_KEY
//
// Optional:
//   - DOCRAPTOR_TEST_MODE: "true" to enable DocRaptor test mode.
func NewDocRaptorFromEnv() (*DocRaptorGenerator, error) {
	apiKey := strings.TrimSpace(os.Getenv("DOCRAPTOR_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("DOCRAPTOR_API_KEY not set")
	}
	testMode := strings.ToLower(strings.TrimSpace(os.Getenv("DOCRAPTOR_TEST_MODE"))) == "true"
	return &DocRaptorGenerator{
		apiKey:   apiKey,
		baseURL:  "https://api.docraptor.com",
		testMode: testMode,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

type createDocRequest struct {
	Test            bool   `json:"test"`
	Name            string `json:"name"`
	DocumentType    string `json:"document_type"`
	DocumentContent string `json:"document_content"`
	Javascript      bool   `json:"javascript"`
}

// GeneratePDF converts HTML to PDF bytes.
func (g *DocRaptorGenerator) GeneratePDF(ctx context.Context, html string) ([]byte, error) {
	reqBody := createDocRequest{
		Test:            g.testMode,
		Name:            "cla.pdf",
		DocumentType:    "pdf",
		DocumentContent: html,
		Javascript:      true,
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal docraptor request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/docs", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/pdf")
	req.SetBasicAuth(g.apiKey, "")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docraptor request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read docraptor response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// DocRaptor returns JSON errors. Preserve body for easier debugging.
		return nil, fmt.Errorf("docraptor: status=%d body=%s", resp.StatusCode, string(body))
	}
	return body, nil
}
