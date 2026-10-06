// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	eventsMock "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	sigOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/signatures"
	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	mock_project "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_projects_cla_groups "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	v1Signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	mock_v1_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/v2/signatures/mock_users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const eclaForbiddenMessage = utils.EasyCLA403Forbidden + " - unable to invalidate ecla - error: not authorized to invalidate this employee acknowledgment"

type capturedEmail struct {
	subject    string
	body       string
	recipients []string
}

type capturingEmailSender struct {
	sent []capturedEmail
}

func (c *capturingEmailSender) SendEmail(subject string, body string, recipients []string) error {
	c.sent = append(c.sent, capturedEmail{subject: subject, body: body, recipients: recipients})
	return nil
}

func eclaItemSignature() *v1Signatures.ItemSignature {
	return &v1Signatures.ItemSignature{
		SignatureID:            "sig-1",
		SignatureReferenceType: "user",
		SignatureType:          "ecla",
		SignatureUserCompanyID: "company-1",
		SignatureProjectID:     "cla-group-1",
		SignatureReferenceID:   "user-1",
		SignatureApproved:      true,
		SignatureSigned:        true,
	}
}

func eclaEventArgs() *events.LogEventArgs {
	return &events.LogEventArgs{
		EventType: events.InvalidatedSignature,
		EventData: &events.SignatureProjectInvalidatedEventData{InvalidatedCount: 1},
	}
}

func parentCCLA(managers ...string) *v1Models.Signature {
	ccla := &v1Models.Signature{SignatureID: "ccla-1", SignatureACL: []v1Models.User{}}
	for _, manager := range managers {
		ccla.SignatureACL = append(ccla.SignatureACL, v1Models.User{LfUsername: manager})
	}
	return ccla
}

// expectParentCCLALookup pins the parent lookup to the acknowledgment's company and cla group and to an approved, signed ccla
func expectParentCCLALookup(ctx context.Context, t *testing.T, mockRepo *mock_v1_signatures.MockSignatureRepository, ccla *v1Models.Signature, err error) {
	mockRepo.EXPECT().GetCorporateSignature(ctx, "cla-group-1", "company-1", gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _, _ string, approved, signed *bool) (*v1Models.Signature, error) {
			if assert.NotNil(t, approved, "the parent lookup must ask for an approved ccla") {
				assert.True(t, *approved, "the parent lookup must ask for an approved ccla")
			}
			if assert.NotNil(t, signed, "the parent lookup must ask for a signed ccla") {
				assert.True(t, *signed, "the parent lookup must ask for a signed ccla")
			}
			return ccla, err
		})
}

// deniedEclaFixture wires the strict mocks of a refused invalidation: the parent lookup is the last
// permitted repository call - any invalidation, re-invalidation, user/cla group lookup, email or event fails the test
type deniedEclaFixture struct {
	repo   *mock_v1_signatures.MockSignatureRepository
	events *eventsMock.MockService
	sender *capturingEmailSender
	svc    *Service
}

func newDeniedEclaFixture(ctx context.Context, t *testing.T, ctrl *gomock.Controller, sig *v1Signatures.ItemSignature) *deniedEclaFixture {
	awsSession, err := ini.GetAWSSession()
	require.NoError(t, err, "unable to create AWS session")

	mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
	mockRepo.EXPECT().GetItemSignature(ctx, "sig-1").Return(sig, nil)
	mockRepo.EXPECT().InvalidateProjectRecordWithMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	mockRepo.EXPECT().ReinvalidateProjectRecordWithMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	mockCompanyService := mock_company.NewMockIService(ctrl)
	mockCompanyService.EXPECT().GetCompany(ctx, "company-1").
		Return(&v1Models.Company{CompanyID: "company-1", CompanyExternalID: "comp-sfid", CompanyName: "Acme"}, nil)

	mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
	mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-1").
		Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-sfid"}}, nil)

	mockUserService := mock_users.NewMockService(ctrl)
	mockUserService.EXPECT().GetUser(gomock.Any()).Times(0)

	mockProjectService := mock_project.NewMockService(ctrl)
	mockProjectService.EXPECT().GetCLAGroupByID(gomock.Any(), gomock.Any()).Times(0)

	mockEvents := eventsMock.NewMockService(ctrl)
	mockEvents.EXPECT().LogEventWithContext(gomock.Any(), gomock.Any()).Times(0)

	sender := &capturingEmailSender{}
	prevSender := utils.GetEmailSender()
	utils.SetEmailSender(sender)
	t.Cleanup(func() { utils.SetEmailSender(prevSender) })

	return &deniedEclaFixture{
		repo:   mockRepo,
		events: mockEvents,
		sender: sender,
		svc:    NewService(awsSession, "", mockProjectService, mockCompanyService, nil, mockProjectClaGroupsRepo, mockRepo, mockUserService, nil),
	}
}

func TestService_InvalidateECLA(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, "unable to create AWS session")
	}

	// the creation path writes signature_type "ecla"; legacy rows carry "cla" - both are acknowledgments
	for _, sigType := range []string{"ecla", "cla"} {
		t.Run("signature type "+sigType, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()

			sig := eclaItemSignature()
			sig.SignatureType = sigType
			mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
			mockRepo.EXPECT().GetItemSignature(ctx, "sig-1").Return(sig, nil)
			mockRepo.EXPECT().ReinvalidateProjectRecordWithMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			var gotNote string
			var gotMetadata *v1Signatures.InvalidationMetadata
			mockRepo.EXPECT().InvalidateProjectRecordWithMetadata(ctx, "sig-1", gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, _, note string, metadata *v1Signatures.InvalidationMetadata) error {
					gotNote = note
					gotMetadata = metadata
					return nil
				})

			mockCompanyService := mock_company.NewMockIService(ctrl)
			mockCompanyService.EXPECT().GetCompany(ctx, "company-1").
				Return(&v1Models.Company{CompanyID: "company-1", CompanyExternalID: "comp-sfid", CompanyName: "Acme"}, nil)

			mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
			mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-1").
				Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-other"}, {ProjectSFID: "proj-sfid"}}, nil)
			expectParentCCLALookup(ctx, t, mockRepo, parentCCLA("org-admin"), nil)

			mockUserService := mock_users.NewMockService(ctrl)
			mockUserService.EXPECT().GetUser("user-1").
				Return(&v1Models.User{UserID: "user-1", LfUsername: "contributor", Username: "Contributor", LfEmail: strfmt.Email("contributor@example.com")}, nil)

			mockProjectService := mock_project.NewMockService(ctrl)
			mockProjectService.EXPECT().GetCLAGroupByID(ctx, "cla-group-1").
				Return(&v1Models.ClaGroup{ProjectName: "My Project", Version: "v2"}, nil)

			mockEvents := eventsMock.NewMockService(ctrl)
			var logged *events.LogEventArgs
			mockEvents.EXPECT().LogEventWithContext(ctx, gomock.Any()).Do(
				func(_ context.Context, args *events.LogEventArgs) {
					logged = args
				})

			service := NewService(awsSession, "", mockProjectService, mockCompanyService, nil, mockProjectClaGroupsRepo, mockRepo, mockUserService, nil)

			sender := &capturingEmailSender{}
			prevSender := utils.GetEmailSender()
			utils.SetEmailSender(sender)
			t.Cleanup(func() { utils.SetEmailSender(prevSender) })

			// the scope matches only the second project mapped to the CLA Group - any-match authorizes
			authUser := &auth.User{UserName: "org-admin", Email: "org-admin@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
			input := &models.EclaInvalidationInput{Reason: "compliance", Note: "per legal\r\nreview\x07"}
			result, err := service.InvalidateECLA(ctx, "cla-group-1", "sig-1", authUser, mockEvents, eclaEventArgs(), input)
			assert.Nil(t, err)

			if assert.NotNil(t, result) {
				assert.Equal(t, "sig-1", result.SignatureID)
				assert.Equal(t, "cla-group-1", result.ClaGroupID)
				assert.Equal(t, "company-1", result.CompanyID)
				assert.Equal(t, "user-1", result.UserID)
			}

			assert.Contains(t, gotNote, "Signature invalidated (approved set to false) by org-admin for Contributor")
			if assert.NotNil(t, gotMetadata) {
				assert.Equal(t, "org-admin", gotMetadata.InvalidatedBy)
				assert.Equal(t, "compliance", gotMetadata.Reason)
				assert.Equal(t, "per legal\nreview", gotMetadata.Note, "the note is sanitized before it is stored")
			}

			if assert.NotNil(t, logged) {
				eventData, ok := logged.EventData.(*events.SignatureProjectInvalidatedEventData)
				if assert.True(t, ok) {
					assert.Equal(t, "sig-1", eventData.SignatureID)
					assert.Equal(t, "org-admin", eventData.InvalidatedBy)
					assert.Equal(t, "compliance", eventData.Reason)
					assert.Equal(t, "per legal\nreview", eventData.InvalidationNote)
				}
				assert.Equal(t, "Contributor", logged.UserName)
				assert.Equal(t, "user-1", logged.UserID, "a top-level user identity is required or the events service drops the event")
				assert.Equal(t, "My Project", logged.ProjectName)
				assert.Equal(t, "cla-group-1", logged.CLAGroupID)
				assert.Equal(t, "company-1", logged.CompanyID)
				assert.Equal(t, "Acme", logged.CompanyName)
			}

			if assert.Len(t, sender.sent, 1, "the employee is notified about the invalidation") {
				assert.Equal(t, []string{"contributor@example.com"}, sender.sent[0].recipients)
				assert.Contains(t, sender.sent[0].subject, "Employee acknowledgment invalidated for My Project")
				assert.Contains(t, sender.sent[0].body, "My Project")
				assert.Contains(t, sender.sent[0].body, "Acme")
			}
		})
	}
}

func TestService_InvalidateECLAAfterApprovalListRemoval(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, "unable to create AWS session")
	}

	byReason := eclaItemSignature()
	byReason.SignatureApproved = false
	byReason.Note = "Signature invalidated (approved set to false) by cla-manager due to Email Criteria  removal"
	byReason.DateInvalidated = "2026-09-15T10:00:00.000000+0000"
	byReason.InvalidatedBy = "cla-manager"
	byReason.InvalidationReason = v1Signatures.ApprovalListRemovalReasonPrefix + utils.EmailCriteria + ")"

	byLegacyNote := eclaItemSignature()
	byLegacyNote.SignatureApproved = false
	byLegacyNote.Note = "Signature invalidated (approved set to false) by cla-manager due to GitHub Org Criteria  removal"

	for _, tc := range []struct {
		name string
		sig  *v1Signatures.ItemSignature
	}{
		{"voided by an approval list removal", byReason},
		{"voided by a pre-attribution approval list removal", byLegacyNote},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ctx := context.Background()

			mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
			mockRepo.EXPECT().GetItemSignature(ctx, "sig-1").Return(tc.sig, nil)
			mockRepo.EXPECT().InvalidateProjectRecordWithMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			var gotNote string
			var gotMetadata *v1Signatures.InvalidationMetadata
			mockRepo.EXPECT().ReinvalidateProjectRecordWithMetadata(ctx, tc.sig, gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, _ *v1Signatures.ItemSignature, note string, metadata *v1Signatures.InvalidationMetadata) error {
					gotNote = note
					gotMetadata = metadata
					return nil
				})

			mockCompanyService := mock_company.NewMockIService(ctrl)
			mockCompanyService.EXPECT().GetCompany(ctx, "company-1").
				Return(&v1Models.Company{CompanyID: "company-1", CompanyExternalID: "comp-sfid", CompanyName: "Acme"}, nil)

			mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
			mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-1").
				Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-sfid"}}, nil)
			expectParentCCLALookup(ctx, t, mockRepo, parentCCLA("org-admin"), nil)

			mockUserService := mock_users.NewMockService(ctrl)
			mockUserService.EXPECT().GetUser("user-1").
				Return(&v1Models.User{UserID: "user-1", LfUsername: "contributor", Username: "Contributor", LfEmail: strfmt.Email("contributor@example.com")}, nil)

			mockProjectService := mock_project.NewMockService(ctrl)
			mockProjectService.EXPECT().GetCLAGroupByID(ctx, "cla-group-1").
				Return(&v1Models.ClaGroup{ProjectName: "My Project", Version: "v2"}, nil)

			mockEvents := eventsMock.NewMockService(ctrl)
			var logged *events.LogEventArgs
			mockEvents.EXPECT().LogEventWithContext(ctx, gomock.Any()).Do(
				func(_ context.Context, args *events.LogEventArgs) {
					logged = args
				})

			service := NewService(awsSession, "", mockProjectService, mockCompanyService, nil, mockProjectClaGroupsRepo, mockRepo, mockUserService, nil)

			sender := &capturingEmailSender{}
			prevSender := utils.GetEmailSender()
			utils.SetEmailSender(sender)
			t.Cleanup(func() { utils.SetEmailSender(prevSender) })

			authUser := &auth.User{UserName: "org-admin", Email: "org-admin@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
			result, err := service.InvalidateECLA(ctx, "cla-group-1", "sig-1", authUser, mockEvents, eclaEventArgs(), &models.EclaInvalidationInput{Reason: "compliance", Note: "per legal review"})
			require.NoError(t, err)
			if assert.NotNil(t, result) {
				assert.Equal(t, &models.EclaInvalidateResult{SignatureID: "sig-1", ClaGroupID: "cla-group-1", CompanyID: "company-1", UserID: "user-1"}, result)
			}
			assert.Contains(t, gotNote, "Signature invalidated (approved set to false) by org-admin for Contributor")
			if assert.NotNil(t, gotMetadata) {
				assert.Equal(t, &v1Signatures.InvalidationMetadata{InvalidatedBy: "org-admin", Reason: "compliance", Note: "per legal review"}, gotMetadata)
			}
			if assert.NotNil(t, logged) {
				eventData, ok := logged.EventData.(*events.SignatureProjectInvalidatedEventData)
				if assert.True(t, ok) {
					assert.Equal(t, "org-admin", eventData.InvalidatedBy)
					assert.Equal(t, "compliance", eventData.Reason)
				}
			}
			assert.Len(t, sender.sent, 1, "the deliberate invalidation notifies the employee")
		})
	}

	t.Run("a concurrent change to the voided ecla is reported as a conflict", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctx := context.Background()

		mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
		mockRepo.EXPECT().GetItemSignature(ctx, "sig-1").Return(byReason, nil)
		mockRepo.EXPECT().InvalidateProjectRecordWithMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		mockRepo.EXPECT().ReinvalidateProjectRecordWithMetadata(ctx, byReason, gomock.Any(), gomock.Any()).
			Return(fmt.Errorf("signature sig-1: %w", v1Signatures.ErrSignatureModifiedConcurrently))

		mockCompanyService := mock_company.NewMockIService(ctrl)
		mockCompanyService.EXPECT().GetCompany(ctx, "company-1").
			Return(&v1Models.Company{CompanyID: "company-1", CompanyExternalID: "comp-sfid", CompanyName: "Acme"}, nil)

		mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
		mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-1").
			Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-sfid"}}, nil)
		expectParentCCLALookup(ctx, t, mockRepo, parentCCLA("org-admin"), nil)

		mockUserService := mock_users.NewMockService(ctrl)
		mockUserService.EXPECT().GetUser("user-1").
			Return(&v1Models.User{UserID: "user-1", LfUsername: "contributor", Username: "Contributor", LfEmail: strfmt.Email("contributor@example.com")}, nil)

		mockProjectService := mock_project.NewMockService(ctrl)
		mockProjectService.EXPECT().GetCLAGroupByID(ctx, "cla-group-1").
			Return(&v1Models.ClaGroup{ProjectName: "My Project", Version: "v2"}, nil)

		mockEvents := eventsMock.NewMockService(ctrl)
		mockEvents.EXPECT().LogEventWithContext(gomock.Any(), gomock.Any()).Times(0)

		service := NewService(awsSession, "", mockProjectService, mockCompanyService, nil, mockProjectClaGroupsRepo, mockRepo, mockUserService, nil)

		sender := &capturingEmailSender{}
		prevSender := utils.GetEmailSender()
		utils.SetEmailSender(sender)
		t.Cleanup(func() { utils.SetEmailSender(prevSender) })

		authUser := &auth.User{UserName: "org-admin", Email: "org-admin@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
		result, err := service.InvalidateECLA(ctx, "cla-group-1", "sig-1", authUser, mockEvents, eclaEventArgs(), &models.EclaInvalidationInput{Reason: "compliance"})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, errEclaAlreadyInvalidated)
		assert.Empty(t, sender.sent, "a lost race sends no notification")
	})
}

func TestService_InvalidateECLASanctionedCompany(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, "unable to create AWS session")
	}

	sanctionedCompany := &v1Models.Company{CompanyID: "company-1", CompanyExternalID: "comp-sfid", CompanyName: "Acme", IsSanctioned: true, SanctionOrigin: "sss"}
	managerUser := &auth.User{UserName: "org-admin", Email: "org-admin@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
	noScopeUser := &auth.User{UserName: "no-scope", Email: "no-scope@example.com", ACL: auth.ACL{Allowed: true}}

	t.Run("authorized manager is rejected with a sanctions error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctx := context.Background()

		// no InvalidateProjectRecordWithMetadata EXPECT - invalidating a sanctioned company's ECLA fails the test
		mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
		mockRepo.EXPECT().GetItemSignature(ctx, "sig-1").Return(eclaItemSignature(), nil)

		mockCompanyService := mock_company.NewMockIService(ctrl)
		mockCompanyService.EXPECT().GetCompany(ctx, "company-1").Return(sanctionedCompany, nil)

		mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
		mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-1").
			Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-sfid"}}, nil)
		expectParentCCLALookup(ctx, t, mockRepo, parentCCLA("org-admin"), nil)

		service := NewService(awsSession, "", nil, mockCompanyService, nil, mockProjectClaGroupsRepo, mockRepo, nil, nil)

		result, err := service.InvalidateECLA(ctx, "cla-group-1", "sig-1", managerUser, nil, eclaEventArgs(), nil)
		assert.Nil(t, result)
		var sanctionedErr *utils.SanctionedCompanyError
		require.True(t, errors.As(err, &sanctionedErr))
		assert.Equal(t, "company-1", sanctionedErr.CompanyID)
		assert.Equal(t, "comp-sfid", sanctionedErr.CompanySFID)
	})

	t.Run("parent acl denial is checked before the sanctions gate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctx := context.Background()

		mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
		mockRepo.EXPECT().GetItemSignature(ctx, "sig-1").Return(eclaItemSignature(), nil)
		expectParentCCLALookup(ctx, t, mockRepo, parentCCLA("cla-manager"), nil)

		mockCompanyService := mock_company.NewMockIService(ctrl)
		mockCompanyService.EXPECT().GetCompany(ctx, "company-1").Return(sanctionedCompany, nil)

		mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
		mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-1").
			Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-sfid"}}, nil)

		service := NewService(awsSession, "", nil, mockCompanyService, nil, mockProjectClaGroupsRepo, mockRepo, nil, nil)

		result, err := service.InvalidateECLA(ctx, "cla-group-1", "sig-1", managerUser, nil, eclaEventArgs(), nil)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, errEclaForbidden, "a non-manager must not learn the sanction status")
		var sanctionedErr *utils.SanctionedCompanyError
		assert.False(t, errors.As(err, &sanctionedErr))
	})

	t.Run("authorization is checked before the sanctions gate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctx := context.Background()

		mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
		mockRepo.EXPECT().GetItemSignature(ctx, "sig-1").Return(eclaItemSignature(), nil)

		mockCompanyService := mock_company.NewMockIService(ctrl)
		mockCompanyService.EXPECT().GetCompany(ctx, "company-1").Return(sanctionedCompany, nil)

		mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
		mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-1").
			Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-sfid"}}, nil)

		service := NewService(awsSession, "", nil, mockCompanyService, nil, mockProjectClaGroupsRepo, mockRepo, nil, nil)

		result, err := service.InvalidateECLA(ctx, "cla-group-1", "sig-1", noScopeUser, nil, eclaEventArgs(), nil)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, errEclaForbidden, "an unauthorized caller must not learn the sanction status")
	})
}

func TestService_InvalidateECLAValidation(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, "unable to create AWS session")
	}

	managerUser := &auth.User{UserName: "org-admin", Email: "org-admin@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
	staffAdmin := &auth.User{UserName: "staff-admin", Email: "staff@example.com", ACL: auth.ACL{Admin: true, Allowed: true}}
	noScopeUser := &auth.User{UserName: "no-scope", Email: "no-scope@example.com", ACL: auth.ACL{Allowed: true}}

	ccla := eclaItemSignature()
	ccla.SignatureReferenceType = "company"
	ccla.SignatureType = "ccla"
	ccla.SignatureUserCompanyID = ""

	icla := eclaItemSignature()
	icla.SignatureType = "cla"
	icla.SignatureUserCompanyID = ""

	wrongGroup := eclaItemSignature()
	wrongGroup.SignatureProjectID = "cla-group-2"

	invalidated := eclaItemSignature()
	invalidated.SignatureApproved = false

	deliberatelyInvalidated := eclaItemSignature()
	deliberatelyInvalidated.SignatureApproved = false
	deliberatelyInvalidated.Note = "Signature invalidated (approved set to false) by org-admin for Contributor due to Email Criteria  removal"
	deliberatelyInvalidated.InvalidatedBy = managerUser.UserName
	deliberatelyInvalidated.InvalidationReason = "compliance"

	legacyInvalidated := eclaItemSignature()
	legacyInvalidated.SignatureApproved = false
	legacyInvalidated.Note = "Signature invalidated (approved set to false) by pcc-admin for Contributor "

	repoDown := errors.New("dynamo down")

	testCases := []struct {
		name        string
		sig         *v1Signatures.ItemSignature
		sigErr      error
		authUser    *auth.User
		expectedErr error
	}{
		{name: "signature lookup failure is propagated", sigErr: repoDown, authUser: managerUser, expectedErr: repoDown},
		{name: "missing signature", authUser: managerUser, expectedErr: errEclaNotFound},
		{name: "ccla record is not an ecla", sig: ccla, authUser: managerUser, expectedErr: errNotEcla},
		{name: "icla record is not an ecla", sig: icla, authUser: managerUser, expectedErr: errNotEcla},
		{name: "ecla of another cla group", sig: wrongGroup, authUser: managerUser, expectedErr: errEclaWrongClaGroup},
		{name: "staff admin is rejected because admin scope is disallowed", sig: eclaItemSignature(), authUser: staffAdmin, expectedErr: errEclaForbidden},
		{name: "user without matching scope is rejected", sig: eclaItemSignature(), authUser: noScopeUser, expectedErr: errEclaForbidden},
		{name: "already invalidated ecla conflicts", sig: invalidated, authUser: managerUser, expectedErr: errEclaAlreadyInvalidated},
		{name: "deliberately invalidated ecla conflicts even with a removal-like note", sig: deliberatelyInvalidated, authUser: managerUser, expectedErr: errEclaAlreadyInvalidated},
		{name: "legacy deliberately invalidated ecla conflicts", sig: legacyInvalidated, authUser: managerUser, expectedErr: errEclaAlreadyInvalidated},
	}

	// the panic-after-write regression lock: GetCLAGroupByID returning (nil, nil) must error
	// out BEFORE the signature is invalidated
	t.Run("cla group lookup returning nil without error is rejected before any mutation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctx := context.Background()

		mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
		mockRepo.EXPECT().GetItemSignature(ctx, "sig-1").Return(eclaItemSignature(), nil)
		mockRepo.EXPECT().InvalidateProjectRecordWithMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		mockRepo.EXPECT().ReinvalidateProjectRecordWithMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		mockCompanyService := mock_company.NewMockIService(ctrl)
		mockCompanyService.EXPECT().GetCompany(ctx, "company-1").
			Return(&v1Models.Company{CompanyID: "company-1", CompanyExternalID: "comp-sfid", CompanyName: "Acme"}, nil)

		mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
		mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-1").
			Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-sfid"}}, nil)
		expectParentCCLALookup(ctx, t, mockRepo, parentCCLA("org-admin"), nil)

		mockUserService := mock_users.NewMockService(ctrl)
		mockUserService.EXPECT().GetUser("user-1").
			Return(&v1Models.User{UserID: "user-1", LfUsername: "contributor"}, nil)

		mockProjectService := mock_project.NewMockService(ctrl)
		mockProjectService.EXPECT().GetCLAGroupByID(ctx, "cla-group-1").Return(nil, nil)

		service := NewService(awsSession, "", mockProjectService, mockCompanyService, nil, mockProjectClaGroupsRepo, mockRepo, mockUserService, nil)

		result, err := service.InvalidateECLA(ctx, "cla-group-1", "sig-1", managerUser, nil, eclaEventArgs(), nil)
		assert.Nil(t, result)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "cla group not found for claGroupID: cla-group-1")
		}
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ctx := context.Background()

			mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
			mockRepo.EXPECT().GetItemSignature(ctx, "sig-1").Return(tc.sig, tc.sigErr)
			mockRepo.EXPECT().InvalidateProjectRecordWithMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			mockRepo.EXPECT().ReinvalidateProjectRecordWithMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			mockCompanyService := mock_company.NewMockIService(ctrl)
			mockCompanyService.EXPECT().GetCompany(ctx, "company-1").
				Return(&v1Models.Company{CompanyID: "company-1", CompanyExternalID: "comp-sfid", CompanyName: "Acme"}, nil).AnyTimes()

			mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
			mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-1").
				Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-sfid"}}, nil).AnyTimes()
			mockRepo.EXPECT().GetCorporateSignature(ctx, "cla-group-1", "company-1", gomock.Any(), gomock.Any()).Return(parentCCLA("org-admin"), nil).AnyTimes()

			service := NewService(awsSession, "", nil, mockCompanyService, nil, mockProjectClaGroupsRepo, mockRepo, nil, nil)

			result, err := service.InvalidateECLA(ctx, "cla-group-1", "sig-1", tc.authUser, nil, eclaEventArgs(), nil)
			assert.Nil(t, result)
			if assert.Error(t, err) {
				assert.ErrorIs(t, err, tc.expectedErr)
			}
		})
	}
}

func TestService_InvalidateECLARequiresParentCCLAManager(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	managerUser := &auth.User{UserName: "org-admin", Email: "org-admin@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
	staffAdmin := &auth.User{UserName: "staff-admin", Email: "staff@example.com", ACL: auth.ACL{Admin: true, Allowed: true}}
	noScopeUser := &auth.User{UserName: "no-scope", Email: "no-scope@example.com", ACL: auth.ACL{Allowed: true}}
	lookupDown := errors.New("dynamo down")

	nilACL := parentCCLA()
	nilACL.SignatureACL = nil

	sameEmailOnly := parentCCLA("someone-else")
	sameEmailOnly.SignatureACL[0].LfEmail = strfmt.Email(managerUser.Email)

	ownACL := eclaItemSignature()
	ownACL.SignatureACL = []string{"org-admin"}

	for _, tc := range []struct {
		name        string
		sig         *v1Signatures.ItemSignature
		ccla        *v1Models.Signature
		lookupErr   error
		expectedErr error
	}{
		{name: "caller absent from a nonempty parent acl", ccla: parentCCLA("cla-manager", "another-manager"), expectedErr: errEclaForbidden},
		{name: "nil parent acl", ccla: nilACL, expectedErr: errEclaForbidden},
		{name: "empty parent acl", ccla: parentCCLA(), expectedErr: errEclaForbidden},
		{name: "username differing only by case", ccla: parentCCLA("Org-Admin"), expectedErr: errEclaForbidden},
		{name: "username differing only by surrounding whitespace", ccla: parentCCLA(" org-admin "), expectedErr: errEclaForbidden},
		{name: "same email on a different manager", ccla: sameEmailOnly, expectedErr: errEclaForbidden},
		{name: "membership in the acknowledgment's own acl", sig: ownACL, ccla: parentCCLA("cla-manager"), expectedErr: errEclaForbidden},
		{name: "no approved and signed parent ccla", expectedErr: errEclaForbidden},
		{name: "parent lookup failure is propagated", lookupErr: lookupDown, expectedErr: lookupDown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ctx := context.Background()

			sig := tc.sig
			if sig == nil {
				sig = eclaItemSignature()
			}
			fx := newDeniedEclaFixture(ctx, t, ctrl, sig)
			expectParentCCLALookup(ctx, t, fx.repo, tc.ccla, tc.lookupErr)

			result, err := fx.svc.InvalidateECLA(ctx, "cla-group-1", "sig-1", managerUser, fx.events, eclaEventArgs(), &models.EclaInvalidationInput{Reason: "compliance"})
			assert.Nil(t, result)
			assert.ErrorIs(t, err, tc.expectedErr)
			assert.Empty(t, fx.sender.sent, "a refused invalidation sends no notification")
		})
	}

	// the parent acl is an additional condition: without the acs scope it is not even consulted
	for _, authUser := range []*auth.User{staffAdmin, noScopeUser} {
		t.Run("parent acl membership without the acs scope: "+authUser.UserName, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ctx := context.Background()

			fx := newDeniedEclaFixture(ctx, t, ctrl, eclaItemSignature())
			fx.repo.EXPECT().GetCorporateSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(parentCCLA(authUser.UserName), nil).Times(0)

			result, err := fx.svc.InvalidateECLA(ctx, "cla-group-1", "sig-1", authUser, fx.events, eclaEventArgs(), nil)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, errEclaForbidden)
			assert.Empty(t, fx.sender.sent)
		})
	}

	t.Run("the lookup is pinned to the acknowledgment's internal company and cla group", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctx := context.Background()

		fx := newDeniedEclaFixture(ctx, t, ctrl, eclaItemSignature())
		// company-2 is another signing entity of the same organization (same SFID) and cla-group-2 another
		// agreement of company-1 - both list the caller, neither is the acknowledgment's parent
		fx.repo.EXPECT().GetCorporateSignature(ctx, "cla-group-1", "company-2", gomock.Any(), gomock.Any()).Return(parentCCLA("org-admin"), nil).Times(0)
		fx.repo.EXPECT().GetCorporateSignature(ctx, "cla-group-2", "company-1", gomock.Any(), gomock.Any()).Return(parentCCLA("org-admin"), nil).Times(0)
		expectParentCCLALookup(ctx, t, fx.repo, parentCCLA("cla-manager"), nil)

		result, err := fx.svc.InvalidateECLA(ctx, "cla-group-1", "sig-1", managerUser, fx.events, eclaEventArgs(), nil)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, errEclaForbidden)
		assert.Empty(t, fx.sender.sent)
	})
}

type fakeEclaInvalidateService struct {
	ServiceInterface
	result        *models.EclaInvalidateResult
	err           error
	gotClaGroupID string
	gotSigID      string
	gotInput      *models.EclaInvalidationInput
}

func (f *fakeEclaInvalidateService) InvalidateECLA(_ context.Context, claGroupID string, signatureID string, _ *auth.User, _ events.Service, _ *events.LogEventArgs, input *models.EclaInvalidationInput) (*models.EclaInvalidateResult, error) {
	f.gotClaGroupID = claGroupID
	f.gotSigID = signatureID
	f.gotInput = input
	return f.result, f.err
}

func TestInvalidateECLAHandlerMapping(t *testing.T) {
	testCases := []struct {
		name           string
		serviceErr     error
		expectedStatus int
	}{
		{name: "success", expectedStatus: http.StatusOK},
		{name: "not found", serviceErr: errEclaNotFound, expectedStatus: http.StatusNotFound},
		{name: "not an ecla", serviceErr: errNotEcla, expectedStatus: http.StatusBadRequest},
		{name: "wrong cla group", serviceErr: errEclaWrongClaGroup, expectedStatus: http.StatusBadRequest},
		{name: "forbidden", serviceErr: errEclaForbidden, expectedStatus: http.StatusForbidden},
		{name: "already invalidated", serviceErr: errEclaAlreadyInvalidated, expectedStatus: http.StatusConflict},
		{name: "sanctioned company", serviceErr: &utils.SanctionedCompanyError{CompanyID: "company-1", CompanySFID: "comp-sfid", CompanyName: "Acme"}, expectedStatus: http.StatusForbidden},
		{name: "unexpected failure", serviceErr: errors.New("dynamo down"), expectedStatus: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			service := &fakeEclaInvalidateService{err: tc.serviceErr}
			if tc.serviceErr == nil {
				service.result = &models.EclaInvalidateResult{SignatureID: "sig-1", ClaGroupID: "cla-group-1", CompanyID: "company-1", UserID: "user-1"}
			}
			Configure(api, nil, nil, nil, nil, nil, nil, service, nil)
			require.NotNil(t, api.SignaturesInvalidateECLAHandler)

			username, email := "tester", "tester@example.com"
			recorder := httptest.NewRecorder()
			api.SignaturesInvalidateECLAHandler.Handle(sigOps.InvalidateECLAParams{
				HTTPRequest: httptest.NewRequest(http.MethodPut, "/v4/cla-group/cla-group-1/ecla/sig-1/invalidate", nil),
				XUSERNAME:   &username,
				XEMAIL:      &email,
				ClaGroupID:  "cla-group-1",
				SignatureID: "sig-1",
				Body:        models.EclaInvalidationInput{Reason: "compliance", Note: "per legal review"},
			}, &auth.User{UserName: "tester", Email: "tester@example.com", ACL: auth.ACL{Allowed: true}}).WriteResponse(recorder, runtime.JSONProducer())

			assert.Equal(t, tc.expectedStatus, recorder.Code)
			assert.Equal(t, "cla-group-1", service.gotClaGroupID)
			assert.Equal(t, "sig-1", service.gotSigID)
			if assert.NotNil(t, service.gotInput) {
				assert.Equal(t, "compliance", service.gotInput.Reason)
			}
			if tc.expectedStatus == http.StatusOK {
				assert.JSONEq(t, `{"signature_id":"sig-1","cla_group_id":"cla-group-1","company_id":"company-1","user_id":"user-1"}`, recorder.Body.String())
			}
			if tc.name == "forbidden" {
				var payload map[string]interface{}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
				assert.Equal(t, utils.String403, payload["Code"])
				assert.Equal(t, eclaForbiddenMessage, payload["Message"])
			}
			if tc.name == "sanctioned company" {
				var payload map[string]interface{}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
				assert.Equal(t, "company_sanctioned", payload["code"])
				assert.Equal(t, "company-1", payload["company_id"])
				assert.Equal(t, "comp-sfid", payload["company_sfid"])
			}
		})
	}
}

func TestInvalidateECLAHandlerParentACLDenial(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, "unable to create AWS session")
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// the handler builds its own request context - match it loosely and pin the business arguments
	mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
	mockRepo.EXPECT().GetItemSignature(gomock.Any(), "sig-1").Return(eclaItemSignature(), nil)
	mockRepo.EXPECT().GetCorporateSignature(gomock.Any(), "cla-group-1", "company-1", gomock.Any(), gomock.Any()).Return(parentCCLA("cla-manager"), nil)
	mockRepo.EXPECT().InvalidateProjectRecordWithMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	mockRepo.EXPECT().ReinvalidateProjectRecordWithMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	mockCompanyService := mock_company.NewMockIService(ctrl)
	mockCompanyService.EXPECT().GetCompany(gomock.Any(), "company-1").
		Return(&v1Models.Company{CompanyID: "company-1", CompanyExternalID: "comp-sfid", CompanyName: "Acme"}, nil)

	mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
	mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), "cla-group-1").
		Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-sfid"}}, nil)

	mockEvents := eventsMock.NewMockService(ctrl)
	mockEvents.EXPECT().LogEventWithContext(gomock.Any(), gomock.Any()).Times(0)

	sender := &capturingEmailSender{}
	prevSender := utils.GetEmailSender()
	utils.SetEmailSender(sender)
	t.Cleanup(func() { utils.SetEmailSender(prevSender) })

	v2Service := NewService(awsSession, "", nil, mockCompanyService, nil, mockProjectClaGroupsRepo, mockRepo, nil, nil)
	api := operations.NewEasyclaAPI(nil)
	Configure(api, nil, nil, mockCompanyService, nil, nil, mockEvents, v2Service, mockProjectClaGroupsRepo)
	require.NotNil(t, api.SignaturesInvalidateECLAHandler)

	authUser := &auth.User{UserName: "org-admin", Email: "org-admin@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
	username, email, reqID := authUser.UserName, authUser.Email, "req-3127"
	recorder := httptest.NewRecorder()
	api.SignaturesInvalidateECLAHandler.Handle(sigOps.InvalidateECLAParams{
		HTTPRequest: httptest.NewRequest(http.MethodPut, "/v4/cla-group/cla-group-1/ecla/sig-1/invalidate", nil),
		XUSERNAME:   &username, XEMAIL: &email, XREQUESTID: &reqID,
		ClaGroupID: "cla-group-1", SignatureID: "sig-1",
		Body: models.EclaInvalidationInput{Reason: "compliance"},
	}, authUser).WriteResponse(recorder, runtime.JSONProducer())

	assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	assert.Equal(t, "req-3127", recorder.Header().Get("X-Request-Id"))
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "403", payload["Code"])
	assert.Equal(t, "req-3127", payload["x-request-id"])
	assert.Equal(t, eclaForbiddenMessage, payload["Message"])
	assert.NotContains(t, payload, "signature_id", "a refusal carries no success payload")
	assert.Empty(t, sender.sent)
}

func TestEclaInvalidateJSONContracts(t *testing.T) {
	result, err := json.Marshal(models.EclaInvalidateResult{})
	assert.Nil(t, err)
	assert.JSONEq(t, `{"signature_id":"","cla_group_id":"","company_id":"","user_id":""}`, string(result), "all result fields must serialize even when empty")

	var input models.EclaInvalidationInput
	err = json.Unmarshal([]byte(`{"reason":"compliance","note":"per legal review"}`), &input)
	assert.Nil(t, err)
	assert.Equal(t, "compliance", input.Reason)
	assert.Equal(t, "per legal review", input.Note)
}
