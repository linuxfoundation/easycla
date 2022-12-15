// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCLAProjectParams_GetProjectFullURL(t *testing.T) {
	testCases := []struct {
		name          string
		projectParams CLAProjectParams
		result        template.HTML
	}{
		{
			name: "empty url",
			projectParams: CLAProjectParams{
				ExternalProjectName: "JohnsProject",
			},
			result: template.HTML("JohnsProject"),
		},
		{
			name: "foundation level project",
			projectParams: CLAProjectParams{
				ExternalProjectName:     "JohnsProject",
				ProjectSFID:             "projectSFIDValue",
				FoundationName:          "CNCF",
				FoundationSFID:          "FoundationSFIDValue",
				SignedAtFoundationLevel: true,
				IsFoundation:            true,
				CorporateConsole:        "https://corporate.dev.lfcla.com",
			},
			result: template.HTML(`<a href="https://corporate.dev.lfcla.com/foundation/FoundationSFIDValue/cla" target="_blank">JohnsProject</a>`),
		},
		{
			name: "standalone project",
			projectParams: CLAProjectParams{
				ExternalProjectName:     "JohnsProject",
				ProjectSFID:             "projectSFIDValue",
				FoundationName:          "CNCF",
				FoundationSFID:          "FoundationSFIDValue",
				SignedAtFoundationLevel: false,
				IsFoundation:            false,
				CorporateConsole:        "https://corporate.dev.lfcla.com",
			},
			result: template.HTML(`<a href="https://corporate.dev.lfcla.com/foundation/FoundationSFIDValue/project/projectSFIDValue/cla" target="_blank">JohnsProject</a>`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			resul := tc.projectParams.GetProjectFullURL()
			assert.Equal(tt, tc.result, resul)
		})
	}
}
