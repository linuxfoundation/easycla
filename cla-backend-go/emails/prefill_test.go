// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"testing"

	"github.com/golang/mock/gomock"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	projectMocks "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCLAGroupTemplateParamsFromCLAGroupUsesVersionSpecificConsole(t *testing.T) {
	const (
		claGroupID   = "cla-group-id"
		claGroupName = "Cloud Native Computing Foundation (CNCF)"
		v1Console    = "https://legacy-console.example.com"
		v2Console    = "https://organization.lfx.linuxfoundation.org/company/dashboard"
	)

	testCases := []struct {
		name            string
		version         string
		expectedConsole string
	}{
		{name: "v1 CLA Group", version: utils.V1, expectedConsole: v1Console},
		{name: "v2 CLA Group", version: utils.V2, expectedConsole: v2Console},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			claGroupRepository := projectMocks.NewMockProjectRepository(ctrl)
			claGroupRepository.EXPECT().
				GetCLAGroupByID(gomock.Any(), claGroupID, false).
				Return(&v1Models.ClaGroup{
					ProjectID:   claGroupID,
					ProjectName: claGroupName,
					Version:     tc.version,
				}, nil)

			svc := NewEmailTemplateService(claGroupRepository, nil, nil, v1Console, v2Console)
			params, err := svc.GetCLAGroupTemplateParamsFromCLAGroup(claGroupID)
			require.NoError(t, err)

			assert.Equal(t, claGroupName, params.CLAGroupName)
			assert.Equal(t, tc.version, params.Version)
			assert.Equal(t, tc.expectedConsole, params.CorporateConsole)
			assert.Empty(t, params.Projects)
		})
	}
}
