// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	projectMocks "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

func TestGetCLAGroupTemplateParamsFromCLAGroupUsesAuthoritativeCLAGroup(t *testing.T) {
	const (
		claGroupID   = "b941159a-7bb9-4a07-bc03-6fcce4431434"
		claGroupName = "ACES"
		v1Console    = "https://legacy-console.example.test"
		v2Console    = "https://organization.example.test"
	)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	claGroupRepository := projectMocks.NewMockProjectRepository(ctrl)
	claGroupRepository.EXPECT().
		GetCLAGroupByID(gomock.Any(), claGroupID, false).
		Return(&v1Models.ClaGroup{
			ProjectID:         claGroupID,
			ProjectName:       claGroupName,
			ProjectExternalID: "malformed-project-mapping-is-not-used",
			Version:           utils.V2,
		}, nil)

	// Mapping and project services are intentionally nil. The exact CLA Group
	// lookup must not depend on either service.
	svc := NewEmailTemplateService(claGroupRepository, nil, nil, v1Console, v2Console)
	params, err := svc.GetCLAGroupTemplateParamsFromCLAGroup(claGroupID)
	if err != nil {
		t.Fatalf("GetCLAGroupTemplateParamsFromCLAGroup() error = %v", err)
	}

	if params.CLAGroupName != claGroupName {
		t.Errorf("CLA Group name = %q, want %q", params.CLAGroupName, claGroupName)
	}
	if params.CorporateConsole != v2Console {
		t.Errorf("corporate console = %q, want %q", params.CorporateConsole, v2Console)
	}
	if params.Version != utils.V2 {
		t.Errorf("version = %q, want %q", params.Version, utils.V2)
	}
	if len(params.Projects) != 0 {
		t.Errorf("Projects length = %d, want 0", len(params.Projects))
	}
}

func TestGetCLAGroupTemplateParamsFromCLAGroupPropagatesRepositoryError(t *testing.T) {
	const claGroupID = "missing-cla-group-id"
	lookupErr := errors.New("CLA Group lookup failed")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	claGroupRepository := projectMocks.NewMockProjectRepository(ctrl)
	claGroupRepository.EXPECT().
		GetCLAGroupByID(gomock.Any(), claGroupID, false).
		Return(nil, lookupErr)

	svc := NewEmailTemplateService(claGroupRepository, nil, nil, "v1-console", "v2-console")
	_, err := svc.GetCLAGroupTemplateParamsFromCLAGroup(claGroupID)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("GetCLAGroupTemplateParamsFromCLAGroup() error = %v, want %v", err, lookupErr)
	}
}
