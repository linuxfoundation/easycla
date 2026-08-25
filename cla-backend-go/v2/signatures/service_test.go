// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"testing"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	// mock_signatures "github.com/linuxfoundation/easycla/cla-backend-go/v2/signatures/mock_v1_signatures"
	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	eventsMock "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	mock_project "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	v1Signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	mock_v1_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/v2/signatures/mock_users"
	"github.com/stretchr/testify/assert"
)

func TestService_IsUserAuthorized(t *testing.T) {
	type testCase struct {
		name                           string
		lfid                           string
		projectID                      string
		userID                         string
		companyID                      string
		getUserByLFUsernameResult      *v1Models.User
		getUserByLFUsernameError       error
		claGroupRequiresICLA           bool
		getIndividualSignatureResult   *v1Models.Signature
		getIndividualSignatureError    error
		processEmployeeSignatureResult *bool
		processEmployeeSignatureError  error
		expectedAuthorized             bool
		expectedCCLARequiresICLA       bool
		expectedICLA                   bool
		expectedCCLA                   bool
		expectedCompanyAffiliation     bool
		getCompanyResult               *v1Models.Company
		getCompanyError                error
	}

	cases := []testCase{
		{
			name:                 "claGroupRequiresICLA",
			lfid:                 "foobar_1",
			projectID:            "project-123",
			userID:               "user-123",
			companyID:            "company-123",
			claGroupRequiresICLA: true,
			getUserByLFUsernameResult: &v1Models.User{
				UserID:     "user-123",
				CompanyID:  "company-123",
				LfUsername: "foobar_1",
			},
			getUserByLFUsernameError: nil,
			getIndividualSignatureResult: &v1Models.Signature{
				SignatureID: "signature-123",
			},
			getIndividualSignatureError:    nil,
			processEmployeeSignatureResult: func() *bool { b := true; return &b }(),
			processEmployeeSignatureError:  nil,
			expectedAuthorized:             true,
			expectedCCLARequiresICLA:       true,
			expectedICLA:                   true,
			expectedCCLA:                   true,
			expectedCompanyAffiliation:     true,
			getCompanyResult: &v1Models.Company{
				CompanyID: "company-123",
			},
			getCompanyError: nil,
		},
		{
			name:                 "claGroupDoesNotRequireICLA",
			lfid:                 "foobar_2",
			projectID:            "project-123",
			userID:               "user-123",
			companyID:            "company-123",
			claGroupRequiresICLA: false,
			getUserByLFUsernameResult: &v1Models.User{
				UserID:     "user-123",
				CompanyID:  "company-123",
				LfUsername: "foobar_2",
			},
			getUserByLFUsernameError: nil,
			getIndividualSignatureResult: &v1Models.Signature{
				SignatureID: "signature-123",
			},
			getIndividualSignatureError:    nil,
			processEmployeeSignatureResult: func() *bool { b := true; return &b }(),
			processEmployeeSignatureError:  nil,
			expectedAuthorized:             true,
			expectedCCLARequiresICLA:       false,
			expectedICLA:                   true,
			expectedCCLA:                   true,
			expectedCompanyAffiliation:     true,
			getCompanyResult: &v1Models.Company{
				CompanyID: "company-123",
			},
			getCompanyError: nil,
		},
		{
			name:      "icla signature  found",
			lfid:      "foobar_3",
			projectID: "project-123",
			userID:    "user-123",
			companyID: "company-123",
			getUserByLFUsernameResult: &v1Models.User{
				UserID:     "user-123",
				CompanyID:  "company-123",
				LfUsername: "foobar_3",
			},
			getUserByLFUsernameError: nil,
			claGroupRequiresICLA:     true,
			getIndividualSignatureResult: &v1Models.Signature{
				SignatureID: "signature-123",
			},
			getIndividualSignatureError:    nil,
			processEmployeeSignatureResult: nil,
			processEmployeeSignatureError:  nil,
			expectedAuthorized:             true,
			expectedCCLARequiresICLA:       true,
			expectedICLA:                   true,
			expectedCCLA:                   false,
			expectedCompanyAffiliation:     true,
			getCompanyResult: &v1Models.Company{
				CompanyID: "company-123",
			},
			getCompanyError: nil,
		},
		{
			name:      "icla signature not found",
			lfid:      "foobar_4",
			projectID: "project-123",
			userID:    "user-123",
			companyID: "company-123",
			getUserByLFUsernameResult: &v1Models.User{
				UserID:     "user-123",
				CompanyID:  "company-123",
				LfUsername: "foobar_4",
			},
			getUserByLFUsernameError:       nil,
			claGroupRequiresICLA:           true,
			getIndividualSignatureResult:   nil,
			getIndividualSignatureError:    errors.New("some error"),
			processEmployeeSignatureResult: func() *bool { b := true; return &b }(),
			processEmployeeSignatureError:  nil,
			expectedAuthorized:             true,
			expectedCCLARequiresICLA:       true,
			expectedICLA:                   false,
			expectedCCLA:                   true,
			expectedCompanyAffiliation:     true,
			getCompanyResult: &v1Models.Company{
				CompanyID: "company-123",
			},
			getCompanyError: nil,
		},
		{
			name:      "individual signature error",
			lfid:      "foobar_5",
			projectID: "project-123",
			userID:    "user-123",
			companyID: "company-123",
			getUserByLFUsernameResult: &v1Models.User{
				UserID:    "user-123",
				CompanyID: "company-123",
			},
			getUserByLFUsernameError:       nil,
			claGroupRequiresICLA:           true,
			getIndividualSignatureResult:   nil,
			getIndividualSignatureError:    errors.New("some error"),
			processEmployeeSignatureResult: func() *bool { b := false; return &b }(),
			processEmployeeSignatureError:  nil,
			expectedAuthorized:             false,
			expectedCCLARequiresICLA:       true,
			expectedICLA:                   false,
			expectedCCLA:                   false,
			expectedCompanyAffiliation:     true,
			getCompanyResult: &v1Models.Company{
				CompanyID: "company-123",
			},
			getCompanyError: nil,
		},
		{
			name:                       "user has not signed ccla and icla",
			lfid:                       "foobar_6",
			projectID:                  "project-123",
			userID:                     "user-123",
			companyID:                  "company-123",
			getUserByLFUsernameResult:  nil,
			getUserByLFUsernameError:   nil,
			claGroupRequiresICLA:       true,
			expectedAuthorized:         false,
			expectedCCLARequiresICLA:   true,
			expectedICLA:               false,
			expectedCCLA:               false,
			expectedCompanyAffiliation: false,
			getCompanyResult: &v1Models.Company{
				CompanyID: "company-123",
			},
			getCompanyError: nil,
		},
		{
			name:      "user has icla and has company id that does not exist",
			lfid:      "foobar_7",
			projectID: "project-123",
			userID:    "user-123",
			companyID: "company-123",
			getUserByLFUsernameResult: &v1Models.User{
				UserID:    "user-123",
				CompanyID: "company-123",
			},
			getUserByLFUsernameError:   nil,
			claGroupRequiresICLA:       false,
			expectedAuthorized:         true,
			expectedCCLARequiresICLA:   false,
			expectedICLA:               true,
			expectedCCLA:               false,
			expectedCompanyAffiliation: false,
			getCompanyResult:           nil,
			getCompanyError: &utils.CompanyNotFound{
				Message:   "no company matching company record",
				CompanyID: "company-123",
			},
			getIndividualSignatureResult: &v1Models.Signature{
				SignatureID: "signature-123",
			},
			getIndividualSignatureError: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var err error
			var result *models.LfidAuthorizedResponse

			awsSession, err := ini.GetAWSSession()
			if err != nil {
				assert.Fail(t, "unable to create AWS session")
			}

			mockProjectService := mock_project.NewMockService(ctrl)
			mockProjectService.EXPECT().GetCLAGroupByID(context.Background(), tc.projectID).Return(&v1Models.ClaGroup{
				ProjectID:               tc.projectID,
				ProjectCCLARequiresICLA: tc.claGroupRequiresICLA,
			}, nil)

			mockUserService := mock_users.NewMockService(ctrl)
			mockUserService.EXPECT().GetUserByLFUserName(tc.lfid).Return(tc.getUserByLFUsernameResult, tc.getUserByLFUsernameError)

			if tc.getUserByLFUsernameResult != nil {
				mockSignatureService := mock_v1_signatures.NewMockSignatureService(ctrl)

				approved := true
				signed := true
				mockSignatureService.EXPECT().GetIndividualSignature(context.Background(), tc.projectID, tc.userID, &approved, &signed).Return(tc.getIndividualSignatureResult, tc.getIndividualSignatureError)

				if tc.getCompanyError == nil {
					mockSignatureService.EXPECT().ProcessEmployeeSignature(context.Background(), gomock.Any(), gomock.Any(), gomock.Any()).Return(tc.processEmployeeSignatureResult, tc.processEmployeeSignatureError)
				}

				mockCompanyService := mock_company.NewMockIService(ctrl)
				mockCompanyService.EXPECT().GetCompany(context.Background(), tc.companyID).Return(tc.getCompanyResult, tc.getCompanyError)

				service := NewService(awsSession, "", mockProjectService, mockCompanyService, mockSignatureService, nil, nil, mockUserService, nil)

				result, err = service.IsUserAuthorized(context.Background(), tc.lfid, tc.projectID)

			} else {
				service := NewService(awsSession, "", mockProjectService, nil, nil, nil, nil, mockUserService, nil)
				result, err = service.IsUserAuthorized(context.Background(), tc.lfid, tc.projectID)
			}
			assert.Nil(t, err)
			assert.Equal(t, tc.expectedAuthorized, result.Authorized)
			assert.Equal(t, tc.expectedCCLARequiresICLA, result.CCLARequiresICLA)
			assert.Equal(t, tc.expectedICLA, result.ICLA)
			assert.Equal(t, tc.expectedCCLA, result.CCLA)
			assert.Equal(t, tc.expectedCompanyAffiliation, result.CompanyAffiliation)
		})
	}
}

func TestService_InvalidateICLA(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, "unable to create AWS session")
	}

	ctx := context.Background()
	approved, signed := true, true

	mockSignatureService := mock_v1_signatures.NewMockSignatureService(ctrl)
	mockSignatureService.EXPECT().GetIndividualSignature(ctx, "cla-group-1", "user-1", &approved, &signed).
		Return(&v1Models.Signature{SignatureID: "sig-1"}, nil)

	mockProjectService := mock_project.NewMockService(ctrl)
	mockProjectService.EXPECT().GetCLAGroupByID(ctx, "cla-group-1").
		Return(&v1Models.ClaGroup{ProjectName: "My Project", Version: "v2"}, nil)

	mockUserService := mock_users.NewMockService(ctrl)
	mockUserService.EXPECT().GetUser("user-1").
		Return(&v1Models.User{UserID: "user-1", LfUsername: "contributor", Username: "Contributor"}, nil)

	mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
	var gotNote string
	var gotMetadata *v1Signatures.InvalidationMetadata
	mockRepo.EXPECT().InvalidateProjectRecordWithMetadata(ctx, "sig-1", gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _, note string, metadata *v1Signatures.InvalidationMetadata) error {
			gotNote = note
			gotMetadata = metadata
			return nil
		})

	mockEvents := eventsMock.NewMockService(ctrl)
	var logged *events.LogEventArgs
	mockEvents.EXPECT().LogEventWithContext(ctx, gomock.Any()).Do(
		func(_ context.Context, args *events.LogEventArgs) {
			logged = args
		})

	service := NewService(awsSession, "", mockProjectService, nil, mockSignatureService, nil, mockRepo, mockUserService, nil)

	eventArgs := &events.LogEventArgs{
		EventType: events.InvalidatedSignature,
		EventData: &events.SignatureProjectInvalidatedEventData{InvalidatedCount: 1},
	}
	input := &models.IclaInvalidationInput{Reason: "compliance", Note: "per legal\r\nreview\x07"}
	err = service.InvalidateICLA(ctx, "cla-group-1", "user-1", &auth.User{UserName: "admin-user"}, mockEvents, eventArgs, input)
	assert.Nil(t, err)

	assert.Contains(t, gotNote, "Signature invalidated (approved set to false) by admin-user for Contributor")
	if assert.NotNil(t, gotMetadata) {
		assert.Equal(t, "admin-user", gotMetadata.InvalidatedBy)
		assert.Equal(t, "compliance", gotMetadata.Reason)
		assert.Equal(t, "per legal\nreview", gotMetadata.Note, "the note is sanitized before it is stored")
	}

	if assert.NotNil(t, logged) {
		eventData, ok := logged.EventData.(*events.SignatureProjectInvalidatedEventData)
		if assert.True(t, ok) {
			assert.Equal(t, "sig-1", eventData.SignatureID)
			assert.Equal(t, "admin-user", eventData.InvalidatedBy)
			assert.Equal(t, "compliance", eventData.Reason)
			assert.Equal(t, "per legal\nreview", eventData.InvalidationNote)
		}
		assert.Equal(t, "Contributor", logged.UserName)
		assert.Equal(t, "user-1", logged.UserID, "a top-level user identity is required or the events service drops the event")
		assert.Equal(t, "My Project", logged.ProjectName)
	}
}

func TestService_InvalidateICLAWithoutBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, "unable to create AWS session")
	}

	ctx := context.Background()
	approved, signed := true, true

	mockSignatureService := mock_v1_signatures.NewMockSignatureService(ctrl)
	mockSignatureService.EXPECT().GetIndividualSignature(ctx, "cla-group-1", "user-1", &approved, &signed).
		Return(&v1Models.Signature{SignatureID: "sig-1"}, nil)

	mockProjectService := mock_project.NewMockService(ctrl)
	mockProjectService.EXPECT().GetCLAGroupByID(ctx, "cla-group-1").
		Return(&v1Models.ClaGroup{ProjectName: "My Project", Version: "v2"}, nil)

	mockUserService := mock_users.NewMockService(ctrl)
	mockUserService.EXPECT().GetUser("user-1").
		Return(&v1Models.User{UserID: "user-1", LfUsername: "contributor"}, nil)

	mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
	var gotMetadata *v1Signatures.InvalidationMetadata
	mockRepo.EXPECT().InvalidateProjectRecordWithMetadata(ctx, "sig-1", gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _, _ string, metadata *v1Signatures.InvalidationMetadata) error {
			gotMetadata = metadata
			return nil
		})

	mockEvents := eventsMock.NewMockService(ctrl)
	var logged *events.LogEventArgs
	mockEvents.EXPECT().LogEventWithContext(ctx, gomock.Any()).Do(
		func(_ context.Context, args *events.LogEventArgs) {
			logged = args
		})

	service := NewService(awsSession, "", mockProjectService, nil, mockSignatureService, nil, mockRepo, mockUserService, nil)

	eventArgs := &events.LogEventArgs{
		EventType: events.InvalidatedSignature,
		EventData: &events.SignatureProjectInvalidatedEventData{InvalidatedCount: 1},
	}
	err = service.InvalidateICLA(ctx, "cla-group-1", "user-1", &auth.User{UserName: "admin-user"}, mockEvents, eventArgs, nil)
	assert.Nil(t, err)
	if assert.NotNil(t, logged) {
		assert.Equal(t, "user-1", logged.UserID, "a top-level user identity is required or the events service drops the event")
	}
	if assert.NotNil(t, gotMetadata) {
		assert.Equal(t, "admin-user", gotMetadata.InvalidatedBy)
		assert.Empty(t, gotMetadata.Reason)
		assert.Empty(t, gotMetadata.Note)
	}
}
