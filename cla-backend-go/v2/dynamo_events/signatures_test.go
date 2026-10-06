// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_pcg_repo "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	mock_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
)

func TestSignatureAddSigTypeSignedApprovedID(t *testing.T) {
	tests := []struct {
		name          string
		newImage      map[string]events.DynamoDBAttributeValue
		setup         func(sigRepo *mock_signatures.MockSignatureRepository, companyRepo *mock_company.MockIRepository, pcgRepo *mock_pcg_repo.MockRepository)
		wantErrSubstr string
	}{
		{
			name: "ccla unchanged",
			newImage: map[string]events.DynamoDBAttributeValue{
				"signature_id":           events.NewStringAttribute("sig-ccla"),
				"signature_type":         events.NewStringAttribute("ccla"),
				"signature_reference_id": events.NewStringAttribute("company-1"),
				"signature_signed":       events.NewBooleanAttribute(true),
				"signature_approved":     events.NewBooleanAttribute(true),
			},
			setup: func(sigRepo *mock_signatures.MockSignatureRepository, _ *mock_company.MockIRepository, _ *mock_pcg_repo.MockRepository) {
				sigRepo.EXPECT().AddSigTypeSignedApprovedID(gomock.Any(), "sig-ccla", "ccla#true#true#company-1").Return(nil)
			},
		},
		{
			name: "icla unchanged",
			newImage: map[string]events.DynamoDBAttributeValue{
				"signature_id":           events.NewStringAttribute("sig-icla"),
				"signature_type":         events.NewStringAttribute("cla"),
				"signature_reference_id": events.NewStringAttribute("user-1"),
				"signature_signed":       events.NewBooleanAttribute(true),
				"signature_approved":     events.NewBooleanAttribute(true),
			},
			setup: func(sigRepo *mock_signatures.MockSignatureRepository, _ *mock_company.MockIRepository, _ *mock_pcg_repo.MockRepository) {
				sigRepo.EXPECT().AddSigTypeSignedApprovedID(gomock.Any(), "sig-icla", "icla#true#true#user-1").Return(nil)
			},
		},
		{
			name: "docusign employee ecla unchanged",
			newImage: map[string]events.DynamoDBAttributeValue{
				"signature_id":                   events.NewStringAttribute("sig-docusign-ecla"),
				"signature_type":                 events.NewStringAttribute("cla"),
				"signature_reference_id":         events.NewStringAttribute("user-1"),
				"signature_user_ccla_company_id": events.NewStringAttribute("company-1"),
				"signature_project_id":           events.NewStringAttribute("cla-group-1"),
				"signature_signed":               events.NewBooleanAttribute(true),
				"signature_approved":             events.NewBooleanAttribute(true),
			},
			setup: func(sigRepo *mock_signatures.MockSignatureRepository, companyRepo *mock_company.MockIRepository, pcgRepo *mock_pcg_repo.MockRepository) {
				// assignContributor short-circuits: company found, but no SF projects for the CLA group
				companyRepo.EXPECT().GetCompany(gomock.Any(), "company-1").
					Return(&models.Company{CompanyID: "company-1", CompanyExternalID: "ext-sfid-1"}, nil)
				pcgRepo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), "cla-group-1").
					Return([]*projects_cla_groups.ProjectClaGroup{}, nil)
				sigRepo.EXPECT().AddSigTypeSignedApprovedID(gomock.Any(), "sig-docusign-ecla", "ecla#true#true#company-1").Return(nil)
			},
		},
		{
			name: "auto-created literal ecla populates the GSI key",
			newImage: map[string]events.DynamoDBAttributeValue{
				"signature_id":                   events.NewStringAttribute("sig-auto-ecla"),
				"signature_type":                 events.NewStringAttribute("ecla"),
				"signature_reference_id":         events.NewStringAttribute("user-1"),
				"signature_user_ccla_company_id": events.NewStringAttribute("company-1"),
				"signature_signed":               events.NewBooleanAttribute(true),
				"signature_approved":             events.NewBooleanAttribute(true),
			},
			setup: func(sigRepo *mock_signatures.MockSignatureRepository, _ *mock_company.MockIRepository, _ *mock_pcg_repo.MockRepository) {
				sigRepo.EXPECT().AddSigTypeSignedApprovedID(gomock.Any(), "sig-auto-ecla", "ecla#true#true#company-1").Return(nil)
			},
		},
		{
			name: "literal ecla without company id still errors",
			newImage: map[string]events.DynamoDBAttributeValue{
				"signature_id":       events.NewStringAttribute("sig-bad-ecla"),
				"signature_type":     events.NewStringAttribute("ecla"),
				"signature_signed":   events.NewBooleanAttribute(true),
				"signature_approved": events.NewBooleanAttribute(true),
			},
			wantErrSubstr: "invalid signature",
		},
		{
			name: "invalid signature type errors",
			newImage: map[string]events.DynamoDBAttributeValue{
				"signature_id":       events.NewStringAttribute("sig-bad"),
				"signature_type":     events.NewStringAttribute("bogus"),
				"signature_signed":   events.NewBooleanAttribute(true),
				"signature_approved": events.NewBooleanAttribute(true),
			},
			wantErrSubstr: "invalid signature",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			sigRepo := mock_signatures.NewMockSignatureRepository(ctrl)
			companyRepo := mock_company.NewMockIRepository(ctrl)
			pcgRepo := mock_pcg_repo.NewMockRepository(ctrl)
			if tc.setup != nil {
				tc.setup(sigRepo, companyRepo, pcgRepo)
			}

			s := &service{
				signatureRepo:        sigRepo,
				companyRepo:          companyRepo,
				projectsClaGroupRepo: pcgRepo,
			}

			err := s.SignatureAddSigTypeSignedApprovedID(events.DynamoDBEventRecord{
				Change: events.DynamoDBStreamRecord{NewImage: tc.newImage},
			})

			if tc.wantErrSubstr != "" {
				assert.ErrorContains(t, err, tc.wantErrSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
