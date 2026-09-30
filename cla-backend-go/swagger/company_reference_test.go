// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package swagger_test

import (
	"strings"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompanyReferencePathValidation(t *testing.T) {
	// Only handlers that resolve company references may accept external organization IDs.
	operations := map[string]bool{
		"getCompanyProjectEvents":               true,
		"createCLAManagerRequest":               true,
		"createCLAManagerDesigneeByGroup":       true,
		"createCLAManagerDesignee":              true,
		"getCompanyByInternalID":                true,
		"getCompanyProjectClaManagers":          true,
		"getCompanyCLAGroupManagers":            true,
		"getCompanyProjectActiveCla":            true,
		"getProjectCompanySignatures":           true,
		"getCompanyMetric":                      false,
		"listCompanyProjectMetrics":             false,
		"downloadProjectSignatureEmployeeAsCSV": false,
		"getCompanySignatures":                  false,
		"getProjectCompanyEmployeeSignatures":   false,
		"updateApprovalList":                    false,
		"eclaAutoCreate":                        false,
		"getCLAManagerRequests":                 false,
		"getCLAManagerRequest":                  false,
		"approveCLAManagerRequest":              false,
		"denyCLAManagerRequest":                 false,
		"createCLAManager":                      false,
		"deleteCLAManager":                      false,
		"deleteCompanyByID":                     false,
		"getCompanyProjectContributors":         false,
	}
	values := []struct {
		name     string
		id       string
		external bool
		valid    bool
	}{
		{"UUIDv4", "9b8e7d66-40a5-4cde-9f00-3e1d1a2b3c4d", false, true},
		{"compact UUIDv4", "9b8e7d6640a54cde9f003e1d1a2b3c4d", false, true},
		{"uppercase UUIDv4", "9B8E7D66-40A5-4CDE-9F00-3E1D1A2B3C4D", false, true},
		{"SFID15", "0014100000Te0yq", true, true},
		{"SFID18", "0014100000Te0yqQAB", true, true},
		{"legacy lf18", "lfbd1c2b3a4d5e6f7a", true, true},
		{"empty", "", false, false},
		{"arbitrary text", "not-a-company", false, false},
		{"SFID14", "0014100000Te0y", true, false},
		{"SFID16", "0014100000Te0yqQ", true, false},
		{"SFID17", "0014100000Te0yqQA", true, false},
		{"SFID19", "0014100000Te0yqQAB0", true, false},
		{"non-alphanumeric", "0014100000Te0yqQA_", true, false},
		{"UUIDv1", "9b8e7d66-40a5-1cde-9f00-3e1d1a2b3c4d", false, false},
		{"invalid UUID variant", "9b8e7d66-40a5-4cde-7f00-3e1d1a2b3c4d", false, false},
		{"UUID suffix", "9b8e7d66-40a5-4cde-9f00-3e1d1a2b3c4d-extra", false, false},
	}

	source, err := loads.Spec("cla.v2.yaml")
	require.NoError(t, err)
	generated, err := loads.Analyzed(restapi.SwaggerJSON, "2.0")
	require.NoError(t, err)
	for name, document := range map[string]*loads.Document{"source": source, "generated": generated} {
		t.Run(name, func(t *testing.T) {
			seen := make(map[string]bool)
			for _, path := range document.Spec().Paths.Paths {
				for _, operation := range []*spec.Operation{path.Get, path.Post, path.Put, path.Delete, path.Patch, path.Head, path.Options} {
					if operation == nil {
						continue
					}
					for _, parameter := range operation.Parameters {
						ref := parameter.Ref.String()
						if strings.HasPrefix(ref, "#/parameters/") {
							var ok bool
							parameter, ok = document.Spec().Parameters[strings.TrimPrefix(ref, "#/parameters/")]
							require.True(t, ok, "missing parameter %s", ref)
						}
						if parameter.Name != "companyID" || parameter.In != "path" {
							continue
						}
						acceptsExternal, ok := operations[operation.ID]
						require.True(t, ok, "unclassified companyID operation %s", operation.ID)
						require.False(t, seen[operation.ID], "duplicate companyID parameter in %s", operation.ID)
						seen[operation.ID] = true
						t.Run(operation.ID, func(t *testing.T) {
							require.True(t, parameter.Required)
							require.Equal(t, "string", parameter.Type)
							require.NotEmpty(t, parameter.Pattern)
							if name == "source" {
								expectedRef := "#/parameters/path-companyID"
								if acceptsExternal {
									expectedRef = "#/parameters/path-companyReference"
								}
								assert.Equal(t, expectedRef, ref)
							}
							for _, value := range values {
								t.Run(value.name, func(t *testing.T) {
									result := validate.NewParamValidator(&parameter, strfmt.Default).Validate(value.id)
									wantValid := value.valid && (!value.external || acceptsExternal)
									assert.Equal(t, wantValid, result == nil || result.IsValid(), "companyID=%q: %v", value.id, result)
								})
							}
						})
					}
				}
			}
			require.Len(t, seen, len(operations))
			for operation := range operations {
				assert.True(t, seen[operation], "missing companyID operation %s", operation)
			}
		})
	}
}
