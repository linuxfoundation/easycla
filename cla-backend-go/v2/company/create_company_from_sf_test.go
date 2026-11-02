// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/golang/mock/gomock"
	mock_company_repo "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	orgModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// an existing organization chosen by name is recorded as is: no org-service create/update, no domain re-lookup (lfx-self-serve-ops#343)
func TestCreateCompanyFromSFModelUsesTheGivenOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mock_company_repo.NewMockIRepository(ctrl)
	var created *v1Models.Company
	repo.EXPECT().CreateCompany(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, model *v1Models.Company) (*v1Models.Company, error) {
		created = model
		return model, nil
	})
	svc := &service{companyRepo: repo}
	org := &orgModels.Organization{ID: "0014100000Existing", Name: "Existing Org", Link: "https://existing.example.invalid", LogoURL: "https://logo.example.invalid/e.png"}
	out, err := svc.CreateCompanyFromSFModel(context.Background(), org, &auth.User{UserName: "caller", Email: "caller@example.invalid"})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "0014100000Existing", out.CompanyID)
	assert.Equal(t, "Existing Org", out.CompanyName)
	assert.Equal(t, "https://existing.example.invalid", out.CompanyWebsite)
	require.NotNil(t, created)
	assert.Equal(t, "0014100000Existing", created.CompanyExternalID)
	assert.Equal(t, "caller", created.CompanyManagerID)
	assert.Equal(t, "Existing Org", created.CompanyName)
	assert.Equal(t, "Existing Org", created.SigningEntityName)
	assert.Empty(t, created.CompanyACL)
	assert.Contains(t, created.Note, "0014100000Existing")
}
