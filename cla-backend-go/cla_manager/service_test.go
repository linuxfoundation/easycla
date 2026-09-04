// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/strfmt"
	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	sigAPI "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
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
	signature         *models.Signature
	removeCalls       int
	removedSignatures []string
	removedManagers   []string
}

func (f *fakeSignatureService) GetProjectCompanySignature(_ context.Context, companyID, projectID string, approved, signed *bool, nextKey *string, pageSize *int64) (*models.Signature, error) {
	return f.signature, nil
}

func (f *fakeSignatureService) RemoveCLAManager(_ context.Context, signatureID, claManagerID string) (*models.Signature, error) {
	f.removeCalls++
	f.removedSignatures = append(f.removedSignatures, signatureID)
	f.removedManagers = append(f.removedManagers, claManagerID)
	return f.signature, nil
}

func (f *fakeSignatureService) GetProjectCompanySignatures(_ context.Context, params sigAPI.GetProjectCompanySignaturesParams) (*models.Signatures, error) {
	return &models.Signatures{Signatures: []*models.Signature{f.signature}}, nil
}

type fakeEventsService struct {
	events.Service
	logCalls int
	logged   []*events.LogEventArgs
}

func (f *fakeEventsService) LogEvent(args *events.LogEventArgs) {
	f.logCalls++
	f.logged = append(f.logged, args)
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

func TestRemoveClaManagerRemovesOneOfTwoManagers(t *testing.T) {
	sigService := &fakeSignatureService{signature: &models.Signature{
		SignatureID: "sig-1",
		SignatureACL: []models.User{
			{LfUsername: "manager-one", LfEmail: strfmt.Email("m1@example.com")},
			{LfUsername: "manager-two", LfEmail: strfmt.Email("m2@example.com")},
		},
	}}
	eventsService := &fakeEventsService{}
	s := service{
		usersService:   &fakeUsersService{user: &models.User{UserID: "user-1", LfUsername: "manager-one", LfEmail: strfmt.Email("m1@example.com")}},
		companyService: &fakeCompanyService{company: &models.Company{CompanyID: "company-1", CompanyName: "Acme"}},
		projectService: &fakeProjectService{claGroup: &models.ClaGroup{ProjectID: "cla-group-1", ProjectName: "My Project"}},
		sigService:     sigService,
		eventsService:  eventsService,
	}

	result, err := s.RemoveClaManager(context.Background(), &auth.User{UserName: "org-admin"}, "company-1", "cla-group-1", "manager-one", "proj-sfid")

	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, sigService.removeCalls, "with two managers on the ACL the removal must proceed")
	assert.Equal(t, []string{"sig-1"}, sigService.removedSignatures)
	assert.Equal(t, []string{"manager-one"}, sigService.removedManagers, "the requested manager - not the other one - must be removed")
	if assert.Equal(t, 1, eventsService.logCalls) {
		logged := eventsService.logged[0]
		assert.Equal(t, events.ClaManagerDeleted, logged.EventType)
		assert.Equal(t, "cla-group-1", logged.CLAGroupID)
		assert.Equal(t, "My Project", logged.CLAGroupName)
		assert.Equal(t, "company-1", logged.CompanyID)
		assert.Equal(t, "org-admin", logged.LfUsername)
		if data, ok := logged.EventData.(*events.CLAManagerDeletedEventData); assert.True(t, ok) {
			assert.Equal(t, "Acme", data.CompanyName)
			assert.Equal(t, "manager-one", data.UserName)
			assert.Equal(t, "m1@example.com", data.UserEmail)
			assert.Equal(t, "manager-one", data.UserLFID)
		}
	}
}
