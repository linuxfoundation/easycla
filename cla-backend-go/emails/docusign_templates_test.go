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
	projectParams  CLAGroupTemplateParams
	claGroupParams CLAGroupTemplateParams
	projectErr     error
	claGroupErr    error

	projectLookupCalls  int
	claGroupLookupCalls int
	requestedVersion    string
	requestedProjectID  string
	requestedCLAGroupID string
}

var _ EmailTemplateService = (*documentSignedTemplateServiceStub)(nil)

func (s *documentSignedTemplateServiceStub) PrefillV2CLAProjectParams(_ []string) ([]CLAProjectParams, error) {
	panic("unexpected PrefillV2CLAProjectParams call")
}

func (s *documentSignedTemplateServiceStub) GetCLAGroupTemplateParamsFromProjectSFID(claGroupVersion, projectSFID string) (CLAGroupTemplateParams, error) {
	s.projectLookupCalls++
	s.requestedVersion = claGroupVersion
	s.requestedProjectID = projectSFID
	return s.projectParams, s.projectErr
}

func (s *documentSignedTemplateServiceStub) GetCLAGroupTemplateParamsFromCLAGroup(claGroupID string) (CLAGroupTemplateParams, error) {
	s.claGroupLookupCalls++
	s.requestedCLAGroupID = claGroupID
	return s.claGroupParams, s.claGroupErr
}

func TestRenderDocumentSignedTemplateUsesVersionAppropriateLookupAndName(t *testing.T) {
	const (
		claGroupID   = "d8cead54-92b7-48c5-a2c8-b1e295e8f7f1"
		projectSFID  = "a0941000002wBz4AAE"
		claGroupName = "Cloud Native Computing Foundation (CNCF)"
		projectName  = "OpenTelemetry"
		pdfLink      = "https://example.test/signed-cla.pdf"
		v1Console    = "https://legacy-console.example.test"
		v2Console    = "https://organization.example.test"
	)

	projectText := "regarding the project " + projectName + "."
	claGroupText := "regarding the CLA Group " + claGroupName + "."

	testCases := []struct {
		name      string
		version   string
		projectID string
		icla      bool
		want      string
		doNotWant string
		console   string
	}{
		{
			name:      "V1 ICLA keeps project lookup and project name",
			version:   utils.V1,
			projectID: projectSFID,
			icla:      true,
			want:      projectText,
			doNotWant: claGroupText,
		},
		{
			name:      "V1 CCLA keeps project lookup and project name",
			version:   utils.V1,
			projectID: projectSFID,
			want:      projectText,
			doNotWant: claGroupText,
			console:   v1Console,
		},
		{
			name:      "V2 ICLA uses exact CLA Group and ignores project mapping",
			version:   utils.V2,
			projectID: "malformed-project-sfid-is-ignored-for-v2",
			icla:      true,
			want:      claGroupText,
			doNotWant: projectText,
		},
		{
			name:      "V2 CCLA uses exact CLA Group and ignores project mapping",
			version:   utils.V2,
			projectID: "malformed-project-sfid-is-ignored-for-v2",
			want:      claGroupText,
			doNotWant: projectText,
			console:   v2Console,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &documentSignedTemplateServiceStub{
				projectParams: CLAGroupTemplateParams{
					CLAGroupName:     claGroupName,
					CorporateConsole: v1Console,
					Version:          utils.V2, // deliberately wrong; the renderer argument is authoritative
					Projects: []CLAProjectParams{
						{ExternalProjectName: projectName},
					},
				},
				claGroupParams: CLAGroupTemplateParams{
					CLAGroupName:     claGroupName,
					CorporateConsole: v2Console,
					Version:          utils.V1, // deliberately wrong; the renderer argument is authoritative
					// No Projects are supplied. V2 rendering must not evaluate Project().
				},
			}

			result, err := RenderDocumentSignedTemplate(
				svc,
				tc.version,
				claGroupID,
				tc.projectID,
				DocumentSignedTemplateParams{
					CommonEmailParams: CommonEmailParams{RecipientName: "Heather"},
					ICLA:              tc.icla,
					PdfLink:           pdfLink,
				},
			)
			if err != nil {
				t.Fatalf("RenderDocumentSignedTemplate() error = %v", err)
			}

			for _, expected := range []string{"Hello Heather", tc.want, pdfLink} {
				if !strings.Contains(result, expected) {
					t.Errorf("rendered email does not contain %q: %s", expected, result)
				}
			}
			if strings.Contains(result, tc.doNotWant) {
				t.Errorf("rendered email unexpectedly contains %q: %s", tc.doNotWant, result)
			}
			if tc.console != "" && !strings.Contains(result, tc.console) {
				t.Errorf("rendered email does not contain corporate console %q: %s", tc.console, result)
			}

			if tc.version == utils.V2 {
				if svc.projectLookupCalls != 0 {
					t.Errorf("project lookup calls = %d, want 0", svc.projectLookupCalls)
				}
				if svc.claGroupLookupCalls != 1 {
					t.Errorf("CLA Group lookup calls = %d, want 1", svc.claGroupLookupCalls)
				}
				if svc.requestedCLAGroupID != claGroupID {
					t.Errorf("requested CLA Group ID = %q, want %q", svc.requestedCLAGroupID, claGroupID)
				}
			} else {
				if svc.projectLookupCalls != 1 {
					t.Errorf("project lookup calls = %d, want 1", svc.projectLookupCalls)
				}
				if svc.claGroupLookupCalls != 0 {
					t.Errorf("CLA Group lookup calls = %d, want 0", svc.claGroupLookupCalls)
				}
				if svc.requestedVersion != tc.version {
					t.Errorf("requested version = %q, want %q", svc.requestedVersion, tc.version)
				}
				if svc.requestedProjectID != tc.projectID {
					t.Errorf("requested project SFID = %q, want %q", svc.requestedProjectID, tc.projectID)
				}
			}
		})
	}
}

func TestRenderDocumentSignedTemplatePropagatesLookupError(t *testing.T) {
	projectLookupErr := errors.New("project lookup failed")
	claGroupLookupErr := errors.New("CLA Group lookup failed")

	testCases := []struct {
		name    string
		version string
		svc     *documentSignedTemplateServiceStub
		wantErr error
	}{
		{
			name:    "V1 project lookup error",
			version: utils.V1,
			svc:     &documentSignedTemplateServiceStub{projectErr: projectLookupErr},
			wantErr: projectLookupErr,
		},
		{
			name:    "V2 exact CLA Group lookup error",
			version: utils.V2,
			svc:     &documentSignedTemplateServiceStub{claGroupErr: claGroupLookupErr},
			wantErr: claGroupLookupErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RenderDocumentSignedTemplate(
				tc.svc,
				tc.version,
				"cla-group-id",
				"project-sfid",
				DocumentSignedTemplateParams{},
			)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("RenderDocumentSignedTemplate() error = %v, want %v", err, tc.wantErr)
			}

			if tc.version == utils.V2 {
				if tc.svc.projectLookupCalls != 0 || tc.svc.claGroupLookupCalls != 1 {
					t.Errorf("lookup calls: project=%d CLAGroup=%d, want project=0 CLAGroup=1", tc.svc.projectLookupCalls, tc.svc.claGroupLookupCalls)
				}
			} else if tc.svc.projectLookupCalls != 1 || tc.svc.claGroupLookupCalls != 0 {
				t.Errorf("lookup calls: project=%d CLAGroup=%d, want project=1 CLAGroup=0", tc.svc.projectLookupCalls, tc.svc.claGroupLookupCalls)
			}
		})
	}
}
