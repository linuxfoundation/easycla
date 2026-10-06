// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeEmployeeSignatureRepo struct {
	SignatureRepository
	mu             sync.Mutex
	employeeModels map[string]*EmployeeModel
	lookupErr      error
	ccla           *models.Signature
	cclaErr        error
	lookupCalls    int
	cclaCalls      int
	validateCalls  [][]string
	createCalls    []string
}

func (f *fakeEmployeeSignatureRepo) GetProjectCompanyEmployeeSignature(_ context.Context, _ *models.Company, _ *models.ClaGroup, employeeUserModel *models.User, wg *sync.WaitGroup, resultChannel chan<- *EmployeeModel, errorChannel chan<- error) {
	defer wg.Done()
	f.mu.Lock()
	f.lookupCalls++
	f.mu.Unlock()
	if f.lookupErr != nil {
		errorChannel <- f.lookupErr
		return
	}
	resultChannel <- f.employeeModels[employeeUserModel.UserID]
}

func (f *fakeEmployeeSignatureRepo) GetCorporateSignature(_ context.Context, _, _ string, _, _ *bool) (*models.Signature, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cclaCalls++
	return f.ccla, f.cclaErr
}

func (f *fakeEmployeeSignatureRepo) ValidateProjectRecordUnlessInvalidated(_ context.Context, signatureID, note string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.validateCalls = append(f.validateCalls, []string{signatureID, note})
	return nil
}

func (f *fakeEmployeeSignatureRepo) CreateProjectCompanyEmployeeSignature(_ context.Context, _ *models.Company, _ *models.ClaGroup, employeeUserModel *models.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
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
	invalidatedUser := &models.User{UserID: "user-invalidated"}
	newUser := &models.User{UserID: "user-new"}

	repo := &fakeEmployeeSignatureRepo{
		employeeModels: map[string]*EmployeeModel{
			"user-active":      {Signature: &models.Signature{SignatureID: "sig-active", SignatureApproved: true, SignatureSigned: true}, User: activeUser},
			"user-stale":       {Signature: &models.Signature{SignatureID: "sig-stale", SignatureApproved: false, SignatureSigned: true}, User: staleUser},
			"user-invalidated": {Signature: &models.Signature{SignatureID: "sig-invalidated", SignatureApproved: false, SignatureSigned: true}, User: invalidatedUser, Invalidated: true},
			"user-new":         {Signature: nil, User: newUser},
		},
	}
	s := service{repo: repo}

	err := s.processEmployeeSignatures(context.Background(),
		&models.Company{CompanyID: "company-1", CompanyName: "Acme"},
		&models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project"},
		[]*models.User{activeUser, staleUser, invalidatedUser, newUser})
	assert.Nil(t, err)

	require.Len(t, repo.validateCalls, 1, "only the existing-but-unapproved acknowledgment is re-validated; the invalidated one is left alone")
	assert.Equal(t, "sig-stale", repo.validateCalls[0][0])
	assert.Equal(t, "signed and approved employee acknowledgment since auto_create_ecla feature flag set to true", repo.validateCalls[0][1])

	assert.Equal(t, []string{"user-new"}, repo.createCalls, "only the user without an acknowledgment gets a new record")
}

// TestProcessEmployeeSignatureGate anchors the GitHub PR-check ECLA gate (HasUserSigned ->
// ProcessEmployeeSignature) that shares the reworked acknowledgment lookup
func TestProcessEmployeeSignatureGate(t *testing.T) {
	user := &models.User{UserID: "user-1", Emails: []string{"dev@acme.example"}, GithubUsername: "dev"}
	ack := &EmployeeModel{Signature: &models.Signature{SignatureID: "ecla-1", SignatureApproved: true, SignatureSigned: true}, User: user}
	ccla := &models.Signature{SignatureID: "ccla-1", EmailApprovalList: []string{"dev@acme.example"}}
	otherCCLA := &models.Signature{SignatureID: "ccla-1", EmailApprovalList: []string{"someone.else@acme.example"}, GithubUsernameApprovalList: []string{"octocat"}}

	cases := []struct {
		name          string
		company       *models.Company
		repo          *fakeEmployeeSignatureRepo
		wantSigned    bool
		wantErr       string
		wantLookups   int
		wantCCLACalls int
	}{
		{"acknowledgment, CCLA and email approval", fakeCompany(), &fakeEmployeeSignatureRepo{employeeModels: map[string]*EmployeeModel{"user-1": ack}, ccla: ccla}, true, "", 1, 1},
		{"no acknowledgment", fakeCompany(), &fakeEmployeeSignatureRepo{employeeModels: map[string]*EmployeeModel{"user-1": {Signature: nil, User: user}}, ccla: ccla}, false, "", 1, 0},
		{"acknowledgment but not on the approval list", fakeCompany(), &fakeEmployeeSignatureRepo{employeeModels: map[string]*EmployeeModel{"user-1": ack}, ccla: otherCCLA}, false, "", 1, 1},
		{"acknowledgment without a signed CCLA", fakeCompany(), &fakeEmployeeSignatureRepo{employeeModels: map[string]*EmployeeModel{"user-1": ack}}, false, "", 1, 1},
		{"sanctioned company", &models.Company{CompanyID: "company-1", CompanyName: "Acme", IsSanctioned: true}, &fakeEmployeeSignatureRepo{employeeModels: map[string]*EmployeeModel{"user-1": ack}, ccla: ccla}, false, "", 0, 0},
		{"lookup error", fakeCompany(), &fakeEmployeeSignatureRepo{lookupErr: errors.New("dynamodb unavailable"), ccla: ccla}, false, "dynamodb unavailable", 1, 0},
		{"CCLA lookup error", fakeCompany(), &fakeEmployeeSignatureRepo{employeeModels: map[string]*EmployeeModel{"user-1": ack}, cclaErr: errors.New("ccla unavailable")}, false, "ccla unavailable", 1, 1},
		// pre-existing: the gate evaluates the approval list, not signature_approved / invalidation
		{"invalidated acknowledgment still evaluates the approval list", fakeCompany(), &fakeEmployeeSignatureRepo{employeeModels: map[string]*EmployeeModel{"user-1": {
			Signature: &models.Signature{SignatureID: "ecla-1", SignatureApproved: false, SignatureSigned: true}, User: user, Invalidated: true}}, ccla: ccla}, true, "", 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := service{repo: tc.repo}
			var signed *bool
			err := runWithDeadline(t, func() error {
				var err error
				signed, err = s.ProcessEmployeeSignature(context.Background(), tc.company, fakeClaGroup(), user)
				return err
			})
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.NotNil(t, signed)
			assert.Equal(t, tc.wantSigned, *signed)
			assert.Equal(t, tc.wantLookups, tc.repo.lookupCalls)
			assert.Equal(t, tc.wantCCLACalls, tc.repo.cclaCalls)
			assert.Empty(t, tc.repo.validateCalls, "the gate never writes")
			assert.Empty(t, tc.repo.createCalls)
		})
	}
}
