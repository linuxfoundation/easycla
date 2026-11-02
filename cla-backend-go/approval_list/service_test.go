// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package approval_list

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	repository2 "github.com/linuxfoundation/easycla/cla-backend-go/project/repository"
	mock_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/stretchr/testify/assert"
)

// the embedded interfaces leave every other method unimplemented: only the calls on the tested path are provided
type approvalRequestRepo struct {
	IRepository
	addCalls int
}

func (r *approvalRequestRepo) ListCclaApprovalListRequests(_ string, _, _, _ *string) (*models.CclaAllowlistRequestList, error) {
	return &models.CclaAllowlistRequestList{}, nil
}

func (r *approvalRequestRepo) AddCclaApprovalRequest(_ *models.Company, _ *models.ClaGroup, _ *models.User, _, _ string) (string, error) {
	r.addCalls++
	return "req-1", nil
}

type approvalRequestProjects struct {
	repository2.ProjectRepository
}

func (p *approvalRequestProjects) GetCLAGroupByID(_ context.Context, claGroupID string, _ bool) (*models.ClaGroup, error) {
	return &models.ClaGroup{ProjectID: claGroupID}, nil
}

// a company and CLA group without a signed and approved CCLA cannot take an approval request: the service returns an
// error before creating the request instead of indexing the empty signature list (lfx-self-serve-ops#789)
func TestAddCclaApprovalListRequestWithoutACorporateSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	companyRepo := mock_company.NewMockIRepository(ctrl)
	companyRepo.EXPECT().GetCompany(gomock.Any(), "c1").Return(&models.Company{CompanyID: "c1"}, nil)
	userRepo := mock_users.NewMockUserRepository(ctrl)
	userRepo.EXPECT().GetUser("u1").Return(&models.User{UserID: "u1"}, nil)
	signatureRepo := mock_signatures.NewMockSignatureRepository(ctrl)
	signatureRepo.EXPECT().GetProjectCompanySignatures(gomock.Any(), "c1", "group-a", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&models.Signatures{Signatures: []*models.Signature{}}, nil)
	repo := &approvalRequestRepo{}
	svc := service{repo: repo, userRepo: userRepo, companyRepo: companyRepo, projectRepo: &approvalRequestProjects{}, signatureRepo: signatureRepo}

	requestID, err := svc.AddCclaApprovalListRequest(context.Background(), "c1", "group-a", models.CclaAllowlistRequestInput{ContributorID: "u1", ContributorEmail: "u1@example.com"})

	assert.Empty(t, requestID)
	assert.EqualError(t, err, "no signed and approved CCLA signature found for the company and CLA group")
	assert.Equal(t, 0, repo.addCalls, "no approval request row is created")
}
