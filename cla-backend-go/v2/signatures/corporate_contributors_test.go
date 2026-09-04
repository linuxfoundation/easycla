// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	sigOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/signatures"
	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	mock_v1_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func corporateContributorsFixture() *v1Models.CorporateContributorList {
	return &v1Models.CorporateContributorList{
		List: []*v1Models.CorporateContributor{
			{
				SignatureID:       "sig-1",
				SignatureVersion:  "2",
				GithubID:          "gh-alice",
				LinuxFoundationID: "alice-lfid",
				Name:              "Alice Smith",
				Email:             "alice@example.com",
				Timestamp:         "2023-07-04T12:34:56Z",
			},
			{
				SignatureID:      "sig-2",
				SignatureVersion: "1",
				GithubID:         "gh-bob",
				Name:             "Bob Jones",
				Email:            "bob@example.com",
				Timestamp:        "2023-08-15T01:02:03Z",
			},
		},
	}
}

func TestService_GetClaGroupCorporateContributorsCsv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, "unable to create AWS session")
	}

	ctx := context.Background()

	mockSignatureService := mock_v1_signatures.NewMockSignatureService(ctrl)
	mockSignatureService.EXPECT().GetClaGroupCorporateContributors(ctx, "cla-group-1", gomock.Any(), nil, nil, nil).
		Return(corporateContributorsFixture(), nil)

	service := NewService(awsSession, "", nil, nil, mockSignatureService, nil, nil, nil, nil)

	csv, err := service.GetClaGroupCorporateContributorsCsv(ctx, "cla-group-1", "company-1")
	assert.Nil(t, err)

	lines := strings.Split(string(csv), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, "GitHub ID,LF_ID,Name,Email,Date Signed", lines[0])
	assert.Equal(t, `gh-alice,alice-lfid,Alice Smith,alice@example.com,"Jul 4,2023"`, lines[1])
	assert.Equal(t, `gh-bob,,Bob Jones,bob@example.com,"Aug 15,2023"`, lines[2], "a contributor without an LF login must still be exported")
}

func TestService_GetClaGroupCorporateContributorsCsvEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, "unable to create AWS session")
	}

	ctx := context.Background()

	mockSignatureService := mock_v1_signatures.NewMockSignatureService(ctrl)
	mockSignatureService.EXPECT().GetClaGroupCorporateContributors(ctx, "cla-group-1", gomock.Any(), nil, nil, nil).
		Return(&v1Models.CorporateContributorList{}, nil)

	service := NewService(awsSession, "", nil, nil, mockSignatureService, nil, nil, nil, nil)

	csv, err := service.GetClaGroupCorporateContributorsCsv(ctx, "cla-group-1", "company-1")
	assert.Nil(t, csv)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "not Found")
	}
}

func TestService_GetClaGroupCorporateContributors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, "unable to create AWS session")
	}

	ctx := context.Background()
	companyID := "company-1"

	mockSignatureService := mock_v1_signatures.NewMockSignatureService(ctrl)
	mockSignatureService.EXPECT().GetClaGroupCorporateContributors(ctx, "cla-group-1", &companyID, nil, nil, nil).
		Return(corporateContributorsFixture(), nil)

	service := NewService(awsSession, "", nil, nil, mockSignatureService, nil, nil, nil, nil)

	result, err := service.GetClaGroupCorporateContributors(ctx, sigOps.ListClaGroupCorporateContributorsParams{
		ClaGroupID: "cla-group-1",
		CompanyID:  &companyID,
	})
	assert.Nil(t, err)

	require.NotNil(t, result)
	require.Len(t, result.List, 2)
	assert.Equal(t, "sig-1", result.List[0].SignatureID, "signatureID must survive the v1 to v2 conversion")
	assert.Equal(t, "2", result.List[0].SignatureVersion, "signature_version must survive the v1 to v2 conversion")
	assert.Equal(t, "alice-lfid", result.List[0].LinuxFoundationID)
	assert.Equal(t, "gh-alice", result.List[0].GithubID)
	assert.Equal(t, "sig-2", result.List[1].SignatureID)
	assert.Empty(t, result.List[1].LinuxFoundationID, "a contributor without an LF login must not be dropped")
	assert.Equal(t, "bob@example.com", result.List[1].Email)
}
