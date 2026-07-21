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
		templateStr string
	}{
		{name: "ICLA", templateStr: DocumentSignedICLATemplate},
		{name: "CCLA", templateStr: DocumentSignedCCLATemplate},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := RenderTemplate(utils.V2, DocumentSignedTemplateName, tc.templateStr, params)
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
