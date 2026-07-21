// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"errors"
	"strings"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

type documentSignedTemplateServiceStub struct {
	params CLAGroupTemplateParams
}

func (s documentSignedTemplateServiceStub) PrefillV2CLAProjectParams(_ []string) ([]CLAProjectParams, error) {
	return nil, errors.New("unexpected PrefillV2CLAProjectParams call")
}

func (s documentSignedTemplateServiceStub) GetCLAGroupTemplateParamsFromProjectSFID(_, _ string) (CLAGroupTemplateParams, error) {
	return s.params, nil
}

func (s documentSignedTemplateServiceStub) GetCLAGroupTemplateParamsFromCLAGroup(_ string) (CLAGroupTemplateParams, error) {
	return CLAGroupTemplateParams{}, errors.New("unexpected GetCLAGroupTemplateParamsFromCLAGroup call")
}

func TestRenderDocumentSignedTemplateUsesVersionAppropriateName(t *testing.T) {
	const (
		claGroupName = "Cloud Native Computing Foundation (CNCF)"
		firstProject = "OpenTelemetry"
	)

	projectText := "regarding the project " + firstProject + "."
	claGroupText := "regarding the CLA Group " + claGroupName + "."

	testCases := []struct {
		name             string
		version          string
		prefilledVersion string
		icla             bool
		want             string
		doNotWant        string
	}{
		{
			name:             "V1 ICLA",
			version:          utils.V1,
			prefilledVersion: utils.V2,
			icla:             true,
			want:             projectText,
			doNotWant:        claGroupText,
		},
		{
			name:             "V1 CCLA",
			version:          utils.V1,
			prefilledVersion: utils.V2,
			want:             projectText,
			doNotWant:        claGroupText,
		},
		{
			name:             "V2 ICLA",
			version:          utils.V2,
			prefilledVersion: utils.V1,
			icla:             true,
			want:             claGroupText,
			doNotWant:        projectText,
		},
		{
			name:             "V2 CCLA",
			version:          utils.V2,
			prefilledVersion: utils.V1,
			want:             claGroupText,
			doNotWant:        projectText,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := documentSignedTemplateServiceStub{
				params: CLAGroupTemplateParams{
					CLAGroupName: claGroupName,
					Version:      tc.prefilledVersion,
					Projects: []CLAProjectParams{
						{ExternalProjectName: firstProject},
					},
				},
			}

			result, err := RenderDocumentSignedTemplate(
				svc,
				tc.version,
				"project-sfid",
				DocumentSignedTemplateParams{
					CommonEmailParams: CommonEmailParams{RecipientName: "Heather"},
					ICLA:              tc.icla,
					PdfLink:           "https://example.test/signed-cla.pdf",
				},
			)
			if err != nil {
				t.Fatalf("RenderDocumentSignedTemplate() error = %v", err)
			}

			if !strings.Contains(result, tc.want) {
				t.Errorf("rendered email does not contain %q: %s", tc.want, result)
			}
			if strings.Contains(result, tc.doNotWant) {
				t.Errorf("rendered email unexpectedly contains %q: %s", tc.doNotWant, result)
			}
		})
	}
}
