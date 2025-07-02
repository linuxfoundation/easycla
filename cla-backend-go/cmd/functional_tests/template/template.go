// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package template

import (
	"bytes"
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/cmd/functional_tests/test_models"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/verdverm/frisby"
)

const (
	claGroupID = "d5412846-5dda-4c58-8f62-4c111a3cd0d3"
)

// TestBehaviour data model
type TestBehaviour struct {
	apiURL      string
	auth0Config test_models.Auth0Config
}

// NewTestBehaviour creates a new test behavior model
func NewTestBehaviour(apiURL string, auth0Config test_models.Auth0Config) *TestBehaviour {
	return &TestBehaviour{
		apiURL + "/v4",
		auth0Config,
	}
}

// RunGetICLATemplateWatermark runs tests to check if watermark was applied to icla documents
func (t *TestBehaviour) RunGetICLATemplateWatermark() {
	frisby.Create("Fetch ICLA with no watermark").
		Get(t.apiURL + fmt.Sprintf("/template/%s/preview?claType=icla", claGroupID)).
		Send().
		ExpectStatus(200).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			reader := bytes.NewReader([]byte(text))
			F.Expect(func(F *frisby.Frisby) (bool, string) {
				ok, err := api.HasWatermarks(reader, &pdfcpu.Configuration{})
				if err != nil {
					return false, fmt.Errorf("error when checking for watermark : %w", err).Error()
				}
				if ok {
					return false, "the file already has watermark can't continue with the tests for icla"
				}

				return true, "success getting icla with no watermark"
			})
		})

	frisby.Create("Fetch ICLA with watermark").
		Get(t.apiURL + fmt.Sprintf("/template/%s/preview?claType=icla&watermark=true", claGroupID)).
		Send().
		ExpectStatus(200).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			reader := bytes.NewReader([]byte(text))
			F.Expect(func(F *frisby.Frisby) (bool, string) {
				ok, err := api.HasWatermarks(reader, &pdfcpu.Configuration{})
				if err != nil {
					return false, fmt.Errorf("error when checking for watermark : %w", err).Error()
				}
				if !ok {
					return false, "missing watermark for icla"
				}

				return true, "success getting watermark for icla"
			})
		})
}

// RunGetCCLATemplateWatermark runs tests to check if watermark was applied to ccla documents
func (t *TestBehaviour) RunGetCCLATemplateWatermark() {
	frisby.Create("Fetch CCLA with no watermark").
		Get(t.apiURL + fmt.Sprintf("/template/%s/preview?claType=ccla", claGroupID)).
		Send().
		ExpectStatus(200).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			reader := bytes.NewReader([]byte(text))
			F.Expect(func(F *frisby.Frisby) (bool, string) {
				ok, err := api.HasWatermarks(reader, &pdfcpu.Configuration{})
				if err != nil {
					return false, fmt.Errorf("error when checking for watermark : %w", err).Error()
				}
				if ok {
					return false, "the file already has watermark can't continue with the tests for ccla"
				}

				return true, "success getting ccla with no watermark"
			})
		})

	frisby.Create("Fetch CCLA with watermark").
		Get(t.apiURL + fmt.Sprintf("/template/%s/preview?claType=ccla&watermark=true", claGroupID)).
		Send().
		ExpectStatus(200).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			reader := bytes.NewReader([]byte(text))
			F.Expect(func(F *frisby.Frisby) (bool, string) {
				ok, err := api.HasWatermarks(reader, &pdfcpu.Configuration{})
				if err != nil {
					return false, fmt.Errorf("error when checking for watermark : %w", err).Error()
				}
				if !ok {
					return false, "missing watermark for ccla"
				}

				return true, "success getting watermark for ccla"
			})
		})
}

// RunAllTests runs all the CLA Template tests
func (t *TestBehaviour) RunAllTests() {
	t.RunGetICLATemplateWatermark()
	t.RunGetCCLATemplateWatermark()
}
