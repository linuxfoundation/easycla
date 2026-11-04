// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const (
	notifyLensBase    = "https://app.example.org"
	notifyCompanySFID = "001000000000000AAA"
	notifyOtherSFID   = "001000000000000BBB"
	notifyCLAGroupID  = "00000000-0000-4000-8000-000000000001"
)

// The gomock controller fails the test on any unexpected call, so a case that sets no expectation
// on GetCompanyBySigningEntityName also proves the notify path never looks a company up by name.
func TestNotifyCompanySFID(t *testing.T) {
	tests := []struct {
		name          string
		requestedSFID string
		expect        func(m *mock_company.MockIService)
		wantSFID      string
		wantLensLink  bool
	}{
		{
			name:          "SFID resolves to an EasyCLA company",
			requestedSFID: notifyCompanySFID,
			expect: func(m *mock_company.MockIService) {
				m.EXPECT().GetCompanyByExternalID(gomock.Any(), notifyCompanySFID).
					Return(&v1Models.Company{CompanyID: "company-a", CompanyExternalID: notifyCompanySFID}, nil)
			},
			wantSFID:     notifyCompanySFID,
			wantLensLink: true,
		},
		{
			name:          "unknown SFID keeps the Corporate Console link",
			requestedSFID: notifyOtherSFID,
			expect: func(m *mock_company.MockIService) {
				m.EXPECT().GetCompanyByExternalID(gomock.Any(), notifyOtherSFID).
					Return(nil, &utils.CompanyNotFound{Message: "not found", CompanySFID: notifyOtherSFID})
			},
		},
		{
			name:          "SFID omitted keeps the Corporate Console link without any lookup",
			requestedSFID: "",
			expect:        func(m *mock_company.MockIService) {},
		},
		{
			name:          "whitespace SFID is treated as omitted",
			requestedSFID: "   ",
			expect:        func(m *mock_company.MockIService) {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			companyService := mock_company.NewMockIService(ctrl)
			tt.expect(companyService)
			s := &service{companyService: companyService}

			got := s.notifyCompanySFID(context.Background(), logrus.Fields{}, tt.requestedSFID)
			assert.Equal(t, tt.wantSFID, got)

			link := utils.OrgLensURL(notifyLensBase, got, notifyCLAGroupID)
			if tt.wantLensLink {
				assert.Equal(t, notifyLensBase+"/org/"+notifyCompanySFID+"/easycla/"+notifyCLAGroupID, link)
			} else {
				assert.Empty(t, link)
			}
		})
	}
}

func TestNotifyCompanySFIDIgnoresDuplicateSigningEntityName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	companyService := mock_company.NewMockIService(ctrl)
	companyService.EXPECT().GetCompanyByExternalID(gomock.Any(), notifyOtherSFID).
		Return(&v1Models.Company{CompanyID: "company-b", CompanyExternalID: notifyOtherSFID, SigningEntityName: "Example Org"}, nil)
	s := &service{companyService: companyService}

	assert.Equal(t, notifyOtherSFID, s.notifyCompanySFID(context.Background(), logrus.Fields{}, notifyOtherSFID))
	assert.Empty(t, s.notifyCompanySFID(context.Background(), logrus.Fields{}, ""))
}
