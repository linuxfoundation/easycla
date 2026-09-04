// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	service2 "github.com/linuxfoundation/easycla/cla-backend-go/project/service"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

type fakeUsersService struct {
	users.Service
	user *models.User
}

func (f *fakeUsersService) GetUserByLFUserName(lfUserName string) (*models.User, error) {
	return f.user, nil
}

type fakeCompanyService struct {
	company.IService
	company *models.Company
}

func (f *fakeCompanyService) GetCompany(_ context.Context, companyID string) (*models.Company, error) {
	return f.company, nil
}

type fakeProjectService struct {
	service2.Service
	claGroup *models.ClaGroup
}

func (f *fakeProjectService) GetCLAGroupByID(_ context.Context, claGroupID string) (*models.ClaGroup, error) {
	return f.claGroup, nil
}

type fakeSignatureService struct {
	signatures.SignatureService
	signature   *models.Signature
	removeCalls int
}

func (f *fakeSignatureService) GetProjectCompanySignature(_ context.Context, companyID, projectID string, approved, signed *bool, nextKey *string, pageSize *int64) (*models.Signature, error) {
	return f.signature, nil
}

func (f *fakeSignatureService) RemoveCLAManager(_ context.Context, signatureID, claManagerID string) (*models.Signature, error) {
	f.removeCalls++
	return f.signature, nil
}

func TestRemoveClaManagerRejectsRemovingTheLastManager(t *testing.T) {
	sigService := &fakeSignatureService{signature: &models.Signature{
		SignatureID:  "sig-1",
		SignatureACL: []models.User{{LfUsername: "last-manager"}},
	}}
	s := service{
		usersService:   &fakeUsersService{user: &models.User{UserID: "user-1", LfUsername: "last-manager"}},
		companyService: &fakeCompanyService{company: &models.Company{CompanyID: "company-1", CompanyName: "Acme"}},
		projectService: &fakeProjectService{claGroup: &models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project"}},
		sigService:     sigService,
	}

	result, err := s.RemoveClaManager(context.Background(), &auth.User{UserName: "org-admin"}, "company-1", "cla-group-1", "last-manager", "proj-sfid")

	assert.Nil(t, result)
	if assert.Error(t, err) {
		claManagerErr, ok := err.(*utils.CLAManagerError)
		if assert.True(t, ok, "the guard must return a *utils.CLAManagerError") {
			assert.Contains(t, claManagerErr.Message, "unable to remove the only remaining CLA Manager")
		}
	}
	assert.Zero(t, sigService.removeCalls, "the signature ACL must not be modified")
}
