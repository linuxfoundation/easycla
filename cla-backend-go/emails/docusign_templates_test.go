// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"errors"
	"fmt"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type documentSignedTemplateServiceStub struct {
	expectedCLAGroupID string
	params             CLAGroupTemplateParams
	claGroupLookup     bool
	projectLookup      bool
}

func (s *documentSignedTemplateServiceStub) PrefillV2CLAProjectParams(_ []string) ([]CLAProjectParams, error) {
	return s.params.Projects, nil
}

func (s *documentSignedTemplateServiceStub) GetCLAGroupTemplateParamsFromProjectSFID(_, _ string) (CLAGroupTemplateParams, error) {
	s.projectLookup = true
	return CLAGroupTemplateParams{}, errors.New("unexpected project/foundation lookup")
}

func (s *documentSignedTemplateServiceStub) GetCLAGroupTemplateParamsFromCLAGroup(claGroupID string) (CLAGroupTemplateParams, error) {
	s.claGroupLookup = true
	if claGroupID != s.expectedCLAGroupID {
		return CLAGroupTemplateParams{}, fmt.Errorf("unexpected CLA Group ID: %s", claGroupID)
	}
	return s.params, nil
}

func TestRenderDocumentSignedTemplateUsesExactCLAGroup(t *testing.T) {
	const (
		claGroupID     = "cncf-cla-group-id"
		claGroupName   = "Cloud Native Computing Foundation (CNCF)"
		unrelatedName  = "OpenTelemetry"
		corporateURL   = "https://organization.lfx.linuxfoundation.org/company/dashboard"
		signedDocument = "https://example.com/signed-cla.pdf"
	)

	testCases := []struct {
		name    string
		version string
		icla    bool
	}{
		{name: "v1 ICLA", version: utils.V1, icla: true},
		{name: "v2 ICLA", version: utils.V2, icla: true},
		{name: "v2 CCLA", version: utils.V2, icla: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &documentSignedTemplateServiceStub{
				expectedCLAGroupID: claGroupID,
				params: CLAGroupTemplateParams{
					CLAGroupName:     claGroupName,
					CorporateConsole: corporateURL,
					Version:          tc.version,
					// Deliberately make the first associated project unrelated to the
					// signed repository. The signed-document email must not use it.
					Projects: []CLAProjectParams{{ExternalProjectName: unrelatedName}},
				},
			}

			result, err := RenderDocumentSignedTemplate(
				svc,
				tc.version,
				claGroupID,
				DocumentSignedTemplateParams{
					CommonEmailParams: CommonEmailParams{RecipientName: "Contributor"},
					ICLA:              tc.icla,
					PdfLink:           signedDocument,
				},
			)
			require.NoError(t, err)

			assert.True(t, svc.claGroupLookup)
			assert.False(t, svc.projectLookup)
			assert.Contains(t, result, "regarding the CLA Group "+claGroupName+".")
			assert.NotContains(t, result, unrelatedName)
			assert.Contains(t, result, signedDocument)
			if !tc.icla {
				assert.Contains(t, result, corporateURL)
			}
		})
	}
}
