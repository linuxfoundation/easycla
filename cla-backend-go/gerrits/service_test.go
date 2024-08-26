// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gerrits

import (
	// "bytes"
	"context"
	// "io"
	// "net/http"
	"testing"

	// mock_apiclient "github.com/communitybridge/easycla/cla-backend-go/api_client/mocks"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	gerritsMock "github.com/communitybridge/easycla/cla-backend-go/gerrits/mocks"
	"github.com/go-openapi/strfmt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestService_AddGerrit(t *testing.T) {

	gerritName := "gerritName"
	gerritURL := "https://mockapi.gerrit.dev.itx.linuxfoundation.org/"
	// gerritHost := "mockapi.gerrit.dev.itx.linuxfoundation.org"
	repos := []*models.GerritRepo{
		{
			ClaEnabled: true,
			Connected:  true,
			ContributorAgreements: []*models.GerritRepoContributorAgreementsItems0{
				{
					Description: "CCLA (Corporate Contributor License Agreement) for SUN",
					Name:        "CCLA",
					URL:         "https://api.dev.lfcla.com/v2/gerrit/01af041c-fa69-4052-a23c-fb8c1d3bef24/corporate/agreementUrl.html",
				},
				{
					Description: "ICLA (Individual Contributor License Agreement) for SUN",
					Name:        "ICLA",
					URL:         "https://api.dev.lfcla.com/v2/gerrit/01af041c-fa69-4052-a23c-fb8c1d3bef24/individual/agreementUrl.html",
				},
			},
			Description: "Access inherited by all other projects.",
			ID:          "All-Projects",
			Name:        "All-Projects",
			State:       "ACTIVE",
			WebLinks: []*models.GerritRepoWebLinksItems0{
				{
					Name: "browse",
					URL:  "/plugins/gitiles/All-Projects",
				},
			},
		},
	}

	testCases := []struct {
		name           string
		claGroupID     string
		projectSFID    string
		params         *models.AddGerritInput
		gerritRepoList *models.GerritRepoList
		ReposExist     []*models.Gerrit
		repoListErr    error
		claGroupModel  *models.ClaGroup
		expectedResult *models.Gerrit
		expectedError  error
	}{
		{
			name:        "Valid input",
			claGroupID:  "claGroupID",
			projectSFID: "projectSFID",
			params: &models.AddGerritInput{
				GerritName: &gerritName,
				GerritURL:  &gerritURL,
				Version:    "version",
			},
			ReposExist: []*models.Gerrit{},
			gerritRepoList: &models.GerritRepoList{
				Repos: repos,
			},
			repoListErr:   nil,
			claGroupModel: &models.ClaGroup{},
			expectedResult: &models.Gerrit{
				GerritName:  gerritName,
				GerritURL:   strfmt.URI(gerritURL),
				ProjectID:   "claGroupID",
				ProjectSFID: "projectSFID",
				Version:     "version",
				GerritRepoList: &models.GerritRepoList{
					Repos: repos,
				},
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockRepo := gerritsMock.NewMockRepository(ctrl)
			mockRepo.EXPECT().ExistsByName(context.Background(), gerritName).Return(tc.ReposExist, nil)
			gerrit := &models.Gerrit{
				GerritName:     gerritName,
				GerritURL:      strfmt.URI(gerritURL),
				ProjectID:      "claGroupID",
				ProjectSFID:    "projectSFID",
				Version:        "version",
				GerritRepoList: tc.gerritRepoList,
			}

			mockRepo.EXPECT().AddGerrit(gomock.Any(), gomock.Any()).Return(gerrit, nil)

			service := NewService(mockRepo)

			result, err := service.AddGerrit(context.Background(), tc.claGroupID, tc.projectSFID, tc.params, tc.claGroupModel)

			if err != nil {
				t.Fatalf("Add Gerrit returned an error: %v", err)
			}
			assert.Equal(t, tc.expectedResult, result)
			assert.Equal(t, repos, result.GerritRepoList.Repos)
		})
	}
}
