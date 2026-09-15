// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_activity

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	signatures1 "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	mock_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xanzy/go-gitlab"
)

type isSignedDeps struct {
	signatures *mock_signatures.MockSignatureRepository
	company    *mock_company.MockIRepository
}

func newIsSignedService(t *testing.T) (*service, *isSignedDeps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	deps := &isSignedDeps{
		signatures: mock_signatures.NewMockSignatureRepository(ctrl),
		company:    mock_company.NewMockIRepository(ctrl),
	}
	return &service{
		signaturesRepository: deps.signatures,
		signatureRepository:  deps.signatures,
		companyRepository:    deps.company,
	}, deps
}

// TestIsSignedGate anchors the GitLab MR-check gate: ICLA first, then the company's CCLA approval
// list and the employee acknowledgment lookup (the plural criteria query, unchanged by the
// acknowledgment lookup rework)
func TestIsSignedGate(t *testing.T) {
	const claGroupID = "cla-group-1"
	gitlabUser := &gitlab.User{ID: 42, Username: "dev", Email: "dev@acme.example"}
	employee := &models.User{UserID: "user-1", Username: "dev", CompanyID: "company-1", Emails: []string{"dev@acme.example"}}
	ccla := &models.Signature{SignatureID: "ccla-1", EmailApprovalList: []string{"dev@acme.example"}}
	ackParams := signatures1.GetProjectCompanyEmployeeSignaturesParams{CompanyID: "company-1", ProjectID: claGroupID, PageSize: aws.Int64(100)}
	emailCriteria := &signatures.ApprovalCriteria{UserEmail: "dev@acme.example"}

	t.Run("ICLA passes before any company lookup", func(t *testing.T) {
		s, deps := newIsSignedService(t)
		deps.signatures.EXPECT().GetIndividualSignature(gomock.Any(), claGroupID, "user-1", aws.Bool(true), aws.Bool(true)).
			Return(&models.Signature{SignatureID: "icla-1"}, nil)

		signed, err := s.isSigned(context.Background(), employee, claGroupID, gitlabUser)
		require.NoError(t, err)
		assert.True(t, signed)
	})

	t.Run("ICLA lookup error", func(t *testing.T) {
		s, deps := newIsSignedService(t)
		deps.signatures.EXPECT().GetIndividualSignature(gomock.Any(), claGroupID, "user-1", gomock.Any(), gomock.Any()).
			Return(nil, errors.New("dynamodb unavailable"))

		signed, err := s.isSigned(context.Background(), employee, claGroupID, gitlabUser)
		require.EqualError(t, err, "dynamodb unavailable")
		assert.False(t, signed)
	})

	t.Run("no company affiliation", func(t *testing.T) {
		s, deps := newIsSignedService(t)
		deps.signatures.EXPECT().GetIndividualSignature(gomock.Any(), claGroupID, "user-2", gomock.Any(), gomock.Any()).Return(nil, nil)

		signed, err := s.isSigned(context.Background(), &models.User{UserID: "user-2"}, claGroupID, gitlabUser)
		require.EqualError(t, err, "user hasn't signed yet")
		assert.False(t, signed)
	})

	t.Run("sanctioned company", func(t *testing.T) {
		s, deps := newIsSignedService(t)
		deps.signatures.EXPECT().GetIndividualSignature(gomock.Any(), claGroupID, "user-1", gomock.Any(), gomock.Any()).Return(nil, nil)
		deps.company.EXPECT().GetCompany(gomock.Any(), "company-1").Return(&models.Company{CompanyID: "company-1", IsSanctioned: true, SanctionOrigin: "sss"}, nil)

		signed, err := s.isSigned(context.Background(), employee, claGroupID, gitlabUser)
		require.EqualError(t, err, "company company-1 is sanctioned")
		assert.False(t, signed)
	})

	t.Run("no signed CCLA", func(t *testing.T) {
		s, deps := newIsSignedService(t)
		deps.signatures.EXPECT().GetIndividualSignature(gomock.Any(), claGroupID, "user-1", gomock.Any(), gomock.Any()).Return(nil, nil)
		deps.company.EXPECT().GetCompany(gomock.Any(), "company-1").Return(&models.Company{CompanyID: "company-1"}, nil)
		deps.signatures.EXPECT().GetCorporateSignature(gomock.Any(), claGroupID, "company-1", aws.Bool(true), aws.Bool(true)).Return(nil, nil)

		signed, err := s.isSigned(context.Background(), employee, claGroupID, gitlabUser)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no corporate signature (CCLA) record found for company : company-1")
		assert.False(t, signed)
	})

	t.Run("not on the approval list", func(t *testing.T) {
		s, deps := newIsSignedService(t)
		deps.signatures.EXPECT().GetIndividualSignature(gomock.Any(), claGroupID, "user-1", gomock.Any(), gomock.Any()).Return(nil, nil)
		deps.company.EXPECT().GetCompany(gomock.Any(), "company-1").Return(&models.Company{CompanyID: "company-1"}, nil)
		deps.signatures.EXPECT().GetCorporateSignature(gomock.Any(), claGroupID, "company-1", aws.Bool(true), aws.Bool(true)).
			Return(&models.Signature{SignatureID: "ccla-1", EmailApprovalList: []string{"someone.else@acme.example"}}, nil)

		signed, err := s.isSigned(context.Background(), employee, claGroupID, gitlabUser)
		require.EqualError(t, err, "user is not approved in signature : ccla-1")
		assert.False(t, signed)
	})

	t.Run("approved without an acknowledgment", func(t *testing.T) {
		s, deps := newIsSignedService(t)
		deps.signatures.EXPECT().GetIndividualSignature(gomock.Any(), claGroupID, "user-1", gomock.Any(), gomock.Any()).Return(nil, nil)
		deps.company.EXPECT().GetCompany(gomock.Any(), "company-1").Return(&models.Company{CompanyID: "company-1"}, nil)
		deps.signatures.EXPECT().GetCorporateSignature(gomock.Any(), claGroupID, "company-1", aws.Bool(true), aws.Bool(true)).Return(ccla, nil)
		deps.signatures.EXPECT().GetProjectCompanyEmployeeSignatures(gomock.Any(), ackParams, emailCriteria).
			Return(&models.Signatures{Signatures: []*models.Signature{}}, nil)

		signed, err := s.isSigned(context.Background(), employee, claGroupID, gitlabUser)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no employee signature records found for company : company-1 user : user-1")
		assert.False(t, signed)
	})

	t.Run("acknowledgment lookup error", func(t *testing.T) {
		s, deps := newIsSignedService(t)
		deps.signatures.EXPECT().GetIndividualSignature(gomock.Any(), claGroupID, "user-1", gomock.Any(), gomock.Any()).Return(nil, nil)
		deps.company.EXPECT().GetCompany(gomock.Any(), "company-1").Return(&models.Company{CompanyID: "company-1"}, nil)
		deps.signatures.EXPECT().GetCorporateSignature(gomock.Any(), claGroupID, "company-1", aws.Bool(true), aws.Bool(true)).Return(ccla, nil)
		deps.signatures.EXPECT().GetProjectCompanyEmployeeSignatures(gomock.Any(), ackParams, emailCriteria).Return(nil, errors.New("dynamodb unavailable"))

		signed, err := s.isSigned(context.Background(), employee, claGroupID, gitlabUser)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dynamodb unavailable")
		assert.False(t, signed)
	})

	t.Run("approved with an acknowledgment", func(t *testing.T) {
		s, deps := newIsSignedService(t)
		deps.signatures.EXPECT().GetIndividualSignature(gomock.Any(), claGroupID, "user-1", gomock.Any(), gomock.Any()).Return(nil, nil)
		deps.company.EXPECT().GetCompany(gomock.Any(), "company-1").Return(&models.Company{CompanyID: "company-1"}, nil)
		deps.signatures.EXPECT().GetCorporateSignature(gomock.Any(), claGroupID, "company-1", aws.Bool(true), aws.Bool(true)).Return(ccla, nil)
		deps.signatures.EXPECT().GetProjectCompanyEmployeeSignatures(gomock.Any(), ackParams, emailCriteria).
			Return(&models.Signatures{Signatures: []*models.Signature{{SignatureID: "ecla-1"}}}, nil)

		signed, err := s.isSigned(context.Background(), employee, claGroupID, gitlabUser)
		require.NoError(t, err)
		assert.True(t, signed)
	})

	t.Run("username criteria when the GitLab profile has no email", func(t *testing.T) {
		s, deps := newIsSignedService(t)
		noEmail := &gitlab.User{ID: 42, Username: "dev"}
		byLogin := &models.Signature{SignatureID: "ccla-1", GitlabUsernameApprovalList: []string{"dev"}}
		deps.signatures.EXPECT().GetIndividualSignature(gomock.Any(), claGroupID, "user-1", gomock.Any(), gomock.Any()).Return(nil, nil)
		deps.company.EXPECT().GetCompany(gomock.Any(), "company-1").Return(&models.Company{CompanyID: "company-1"}, nil)
		deps.signatures.EXPECT().GetCorporateSignature(gomock.Any(), claGroupID, "company-1", aws.Bool(true), aws.Bool(true)).Return(byLogin, nil)
		deps.signatures.EXPECT().GetProjectCompanyEmployeeSignatures(gomock.Any(), ackParams, &signatures.ApprovalCriteria{GitlabUsername: "dev"}).
			Return(&models.Signatures{Signatures: []*models.Signature{{SignatureID: "ecla-1"}}}, nil)

		signed, err := s.isSigned(context.Background(), employee, claGroupID, noEmail)
		require.NoError(t, err)
		assert.True(t, signed)
	})
}
