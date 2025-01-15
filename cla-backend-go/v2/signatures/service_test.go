// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"testing"

	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/utils"

	// mock_signatures "github.com/communitybridge/easycla/cla-backend-go/v2/signatures/mock_v1_signatures"
	mock_company "github.com/communitybridge/easycla/cla-backend-go/company/mocks"
	ini "github.com/communitybridge/easycla/cla-backend-go/init"
	mock_project "github.com/communitybridge/easycla/cla-backend-go/project/mocks"
	mock_v1_signatures "github.com/communitybridge/easycla/cla-backend-go/signatures/mocks"
	mock_users "github.com/communitybridge/easycla/cla-backend-go/v2/signatures/mock_users"
	"github.com/golang/mock/gomock"
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
