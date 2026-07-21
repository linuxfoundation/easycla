// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"strings"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

func TestDocumentSignedTemplatesUseCLAGroupName(t *testing.T) {
	const (
		claGroupName = "Cloud Native Computing Foundation (CNCF)"
		firstProject = "OpenTelemetry"
	)

	params := DocumentSignedTemplateParams{
		CommonEmailParams: CommonEmailParams{RecipientName: "Heather"},
		CLAGroupTemplateParams: CLAGroupTemplateParams{
			CLAGroupName: claGroupName,
			Projects: []CLAProjectParams{
				{ExternalProjectName: firstProject},
			},
		},
		PdfLink: "https://example.test/signed-cla.pdf",
	}

	testCases := []struct {
		name        string
		version     string
		templateStr string
	}{
		{name: "V1 ICLA", version: utils.V1, templateStr: DocumentSignedICLATemplate},
		{name: "V1 CCLA", version: utils.V1, templateStr: DocumentSignedCCLATemplate},
		{name: "V2 ICLA", version: utils.V2, templateStr: DocumentSignedICLATemplate},
		{name: "V2 CCLA", version: utils.V2, templateStr: DocumentSignedCCLATemplate},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testParams := params
			testParams.Version = tc.version
			result, err := RenderTemplate(tc.version, DocumentSignedTemplateName, tc.templateStr, testParams)
			if err != nil {
				t.Fatalf("RenderTemplate() error = %v", err)
			}

			expected := "regarding the CLA Group " + claGroupName + "."
			if !strings.Contains(result, expected) {
				t.Errorf("rendered email does not contain %q: %s", expected, result)
			}
			if strings.Contains(result, firstProject) {
				t.Errorf("rendered email contains unrelated first project %q: %s", firstProject, result)
			}
		})
	}
}
