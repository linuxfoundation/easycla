// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gerrits

import (
	"context"
	"testing"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	gerritsMock "github.com/communitybridge/easycla/cla-backend-go/gerrits/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestService_AddGerrit(t *testing.T) {
	// AddGerrit test case

	gerritName := "ONAP"
	gerritURL := "https://gerrit.onap.org"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := gerritsMock.NewMockRepository(ctrl)
	mockRepo.EXPECT().AddGerrit(gomock.Any(), gomock.Any()).Return(&models.Gerrit{
		GerritID:   "e82c469a-55ea-492d-9722-fd30b31da2aa",
		GerritName: "ONAP",
		GerritURL:  "https://gerrit.onap.org",
		ProjectID:  "projectID",
	}, nil)

	//Gerrit repo by name does not exist
	mockRepo.EXPECT().ExistsByName(context.TODO(), "ONAP").Return(nil, nil)

	service := NewService(mockRepo)
	gerrit, err := service.AddGerrit(context.TODO(), "projectID", "projectSFID", &models.AddGerritInput{
		GerritName: &gerritName,
		GerritURL:  &gerritURL,
	}, &models.ClaGroup{
		ProjectID: "projectID",
	})

	assert.NotNil(t, gerrit)
	assert.NoError(t, err)

}
