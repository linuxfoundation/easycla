// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// the company loaded for a corporate signature is handed back so the signatory e-mail and the DocuSign
// request text carry the company name (easycla#5257)
func TestPopulateUserDetailsReturnsTheCompany(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	companyRepo := mock_company.NewMockIRepository(ctrl)
	companyRepo.EXPECT().GetCompany(gomock.Any(), "company-1").Return(&v1Models.Company{CompanyID: "company-1", CompanyName: "Acme Inc"}, nil)
	svc := &service{companyRepo: companyRepo}

	details, err := svc.populateUserDetails(context.Background(), utils.SignatureReferenceTypeCompany, &signatures.ItemSignature{SignatureReferenceID: "company-1"}, "Jane Manager", "jane@example.invalid", true, "")
	require.NoError(t, err)
	require.NotNil(t, details.company)
	assert.Equal(t, "Acme Inc", details.company.CompanyName)
	assert.Equal(t, "Jane Manager", details.userSignatureName)
	assert.Equal(t, "jane@example.invalid", details.userSignatureEmail)

	details, err = svc.populateUserDetails(context.Background(), utils.SignatureReferenceTypeUser, &signatures.ItemSignature{SignatureReferenceID: "user-1"}, "", "", true, "")
	require.NoError(t, err)
	assert.Nil(t, details.company, "an individual signature carries no company")
}
