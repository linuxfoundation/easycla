// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"sync"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeEmployeeSignatureRepo struct {
	SignatureRepository
	employeeModels map[string]*EmployeeModel
	validateCalls  [][]string
	createCalls    []string
}

func (f *fakeEmployeeSignatureRepo) GetProjectCompanyEmployeeSignature(_ context.Context, _ *models.Company, _ *models.ClaGroup, employeeUserModel *models.User, wg *sync.WaitGroup, resultChannel chan<- *EmployeeModel, _ chan<- error) {
	defer wg.Done()
	resultChannel <- f.employeeModels[employeeUserModel.UserID]
}

func (f *fakeEmployeeSignatureRepo) ValidateProjectRecord(_ context.Context, signatureID, note string) error {
	f.validateCalls = append(f.validateCalls, []string{signatureID, note})
	return nil
}

func (f *fakeEmployeeSignatureRepo) CreateProjectCompanyEmployeeSignature(_ context.Context, _ *models.Company, _ *models.ClaGroup, employeeUserModel *models.User) error {
	f.createCalls = append(f.createCalls, employeeUserModel.UserID)
	return nil
}

func TestCreateOrUpdateEmployeeSignatureSanctionedCompany(t *testing.T) {
	s := service{}
	claGroup := &models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project"}
	company := &models.Company{CompanyID: "company-1", CompanyName: "Acme", IsSanctioned: true}

	userList, err := s.CreateOrUpdateEmployeeSignature(context.Background(), claGroup, company, nil)

	assert.Nil(t, userList)
	if assert.Error(t, err, "the sanctions gate must fire before any repository access") {
		assert.Contains(t, err.Error(), "company company-1 is sanctioned; employee (ECLA) signatures cannot be created")
	}
}

func TestProcessEmployeeSignatures(t *testing.T) {
	activeUser := &models.User{UserID: "user-active"}
	staleUser := &models.User{UserID: "user-stale"}
	newUser := &models.User{UserID: "user-new"}

	repo := &fakeEmployeeSignatureRepo{
		employeeModels: map[string]*EmployeeModel{
			"user-active": {Signature: &models.Signature{SignatureID: "sig-active", SignatureApproved: true, SignatureSigned: true}, User: activeUser},
			"user-stale":  {Signature: &models.Signature{SignatureID: "sig-stale", SignatureApproved: false, SignatureSigned: true}, User: staleUser},
			"user-new":    {Signature: nil, User: newUser},
		},
	}
	s := service{repo: repo}

	err := s.processEmployeeSignatures(context.Background(),
		&models.Company{CompanyID: "company-1", CompanyName: "Acme"},
		&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project"},
		[]*models.User{activeUser, staleUser, newUser})
	assert.Nil(t, err)

	require.Len(t, repo.validateCalls, 1, "only the existing-but-unapproved acknowledgement is re-validated")
	assert.Equal(t, "sig-stale", repo.validateCalls[0][0])
	assert.Equal(t, "signed and approved employee acknowledgement since auto_create_ecla feature flag set to true", repo.validateCalls[0][1])

	assert.Equal(t, []string{"user-new"}, repo.createCalls, "only the user without an acknowledgement gets a new record")
}
