// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	eventsMock "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	sigOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/signatures"
	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	mock_project "github.com/linuxfoundation/easycla/cla-backend-go/project/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_projects_cla_groups "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	v1Signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	mock_v1_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/v2/signatures/mock_users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func eclaItemSignature() *v1Signatures.ItemSignature {
	return &v1Signatures.ItemSignature{
		SignatureID:            "sig-1",
		SignatureReferenceType: "user",
		SignatureType:          "ecla",
		SignatureUserCompanyID: "company-1",
		SignatureProjectID:     "cla-group-1",
		SignatureReferenceID:   "user-1",
		SignatureApproved:      true,
		SignatureSigned:        true,
	}
}

func eclaEventArgs() *events.LogEventArgs {
	return &events.LogEventArgs{
		EventType: events.InvalidatedSignature,
		EventData: &events.SignatureProjectInvalidatedEventData{InvalidatedCount: 1},
	}
}

func TestService_InvalidateECLA(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, "unable to create AWS session")
	}

	// the creation path writes signature_type "ecla"; legacy rows carry "cla" - both are acknowledgements
	for _, sigType := range []string{"ecla", "cla"} {
		t.Run("signature type "+sigType, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()

			sig := eclaItemSignature()
			sig.SignatureType = sigType
			mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
			mockRepo.EXPECT().GetItemSignature(ctx, "sig-1").Return(sig, nil)
			var gotNote string
			var gotMetadata *v1Signatures.InvalidationMetadata
			mockRepo.EXPECT().InvalidateProjectRecordWithMetadata(ctx, "sig-1", gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, _, note string, metadata *v1Signatures.InvalidationMetadata) error {
					gotNote = note
					gotMetadata = metadata
					return nil
				})

			mockCompanyService := mock_company.NewMockIService(ctrl)
			mockCompanyService.EXPECT().GetCompany(ctx, "company-1").
				Return(&v1Models.Company{CompanyID: "company-1", CompanyExternalID: "comp-sfid", CompanyName: "Acme"}, nil)

			mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
			mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-1").
				Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-other"}, {ProjectSFID: "proj-sfid"}}, nil)

			mockUserService := mock_users.NewMockService(ctrl)
			mockUserService.EXPECT().GetUser("user-1").
				Return(&v1Models.User{UserID: "user-1", LfUsername: "contributor", Username: "Contributor"}, nil)

			mockProjectService := mock_project.NewMockService(ctrl)
			mockProjectService.EXPECT().GetCLAGroupByID(ctx, "cla-group-1").
				Return(&v1Models.ClaGroup{ProjectName: "My Project", Version: "v2"}, nil)

			mockEvents := eventsMock.NewMockService(ctrl)
			var logged *events.LogEventArgs
			mockEvents.EXPECT().LogEventWithContext(ctx, gomock.Any()).Do(
				func(_ context.Context, args *events.LogEventArgs) {
					logged = args
				})

			service := NewService(awsSession, "", mockProjectService, mockCompanyService, nil, mockProjectClaGroupsRepo, mockRepo, mockUserService, nil)

			// the scope matches only the second project mapped to the CLA Group - any-match authorizes
			authUser := &auth.User{UserName: "org-admin", Email: "org-admin@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
			input := &models.EclaInvalidationInput{Reason: "compliance", Note: "per legal\r\nreview\x07"}
			result, err := service.InvalidateECLA(ctx, "cla-group-1", "sig-1", authUser, mockEvents, eclaEventArgs(), input)
			assert.Nil(t, err)

			if assert.NotNil(t, result) {
				assert.Equal(t, "sig-1", result.SignatureID)
				assert.Equal(t, "cla-group-1", result.ClaGroupID)
				assert.Equal(t, "company-1", result.CompanyID)
				assert.Equal(t, "user-1", result.UserID)
			}

			assert.Contains(t, gotNote, "Signature invalidated (approved set to false) by org-admin for Contributor")
			if assert.NotNil(t, gotMetadata) {
				assert.Equal(t, "org-admin", gotMetadata.InvalidatedBy)
				assert.Equal(t, "compliance", gotMetadata.Reason)
				assert.Equal(t, "per legal\nreview", gotMetadata.Note, "the note is sanitized before it is stored")
			}

			if assert.NotNil(t, logged) {
				eventData, ok := logged.EventData.(*events.SignatureProjectInvalidatedEventData)
				if assert.True(t, ok) {
					assert.Equal(t, "sig-1", eventData.SignatureID)
					assert.Equal(t, "org-admin", eventData.InvalidatedBy)
					assert.Equal(t, "compliance", eventData.Reason)
					assert.Equal(t, "per legal\nreview", eventData.InvalidationNote)
				}
				assert.Equal(t, "Contributor", logged.UserName)
				assert.Equal(t, "user-1", logged.UserID, "a top-level user identity is required or the events service drops the event")
				assert.Equal(t, "My Project", logged.ProjectName)
				assert.Equal(t, "cla-group-1", logged.CLAGroupID)
				assert.Equal(t, "company-1", logged.CompanyID)
				assert.Equal(t, "Acme", logged.CompanyName)
			}
		})
	}
}

func TestService_InvalidateECLAValidation(t *testing.T) {
	t.Setenv("DISABLE_LOCAL_PERMISSION_CHECKS", "false")

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		assert.Fail(t, "unable to create AWS session")
	}

	managerUser := &auth.User{UserName: "org-admin", Email: "org-admin@example.com", ACL: auth.ACL{Allowed: true, Scopes: []auth.Scope{{Type: auth.ProjectOrganization, ID: "proj-sfid|comp-sfid"}}}}
	staffAdmin := &auth.User{UserName: "staff-admin", Email: "staff@example.com", ACL: auth.ACL{Admin: true, Allowed: true}}
	noScopeUser := &auth.User{UserName: "no-scope", Email: "no-scope@example.com", ACL: auth.ACL{Allowed: true}}

	ccla := eclaItemSignature()
	ccla.SignatureReferenceType = "company"
	ccla.SignatureType = "ccla"
	ccla.SignatureUserCompanyID = ""

	icla := eclaItemSignature()
	icla.SignatureType = "cla"
	icla.SignatureUserCompanyID = ""

	wrongGroup := eclaItemSignature()
	wrongGroup.SignatureProjectID = "cla-group-2"

	invalidated := eclaItemSignature()
	invalidated.SignatureApproved = false

	repoDown := errors.New("dynamo down")

	testCases := []struct {
		name        string
		sig         *v1Signatures.ItemSignature
		sigErr      error
		authUser    *auth.User
		expectedErr error
	}{
		{name: "signature lookup failure is propagated", sigErr: repoDown, authUser: managerUser, expectedErr: repoDown},
		{name: "missing signature", authUser: managerUser, expectedErr: errEclaNotFound},
		{name: "ccla record is not an ecla", sig: ccla, authUser: managerUser, expectedErr: errNotEcla},
		{name: "icla record is not an ecla", sig: icla, authUser: managerUser, expectedErr: errNotEcla},
		{name: "ecla of another cla group", sig: wrongGroup, authUser: managerUser, expectedErr: errEclaWrongClaGroup},
		{name: "staff admin is rejected because admin scope is disallowed", sig: eclaItemSignature(), authUser: staffAdmin, expectedErr: errEclaForbidden},
		{name: "user without matching scope is rejected", sig: eclaItemSignature(), authUser: noScopeUser, expectedErr: errEclaForbidden},
		{name: "already invalidated ecla conflicts", sig: invalidated, authUser: managerUser, expectedErr: errEclaAlreadyInvalidated},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ctx := context.Background()

			mockRepo := mock_v1_signatures.NewMockSignatureRepository(ctrl)
			mockRepo.EXPECT().GetItemSignature(ctx, "sig-1").Return(tc.sig, tc.sigErr)

			mockCompanyService := mock_company.NewMockIService(ctrl)
			mockCompanyService.EXPECT().GetCompany(ctx, "company-1").
				Return(&v1Models.Company{CompanyID: "company-1", CompanyExternalID: "comp-sfid", CompanyName: "Acme"}, nil).AnyTimes()

			mockProjectClaGroupsRepo := mock_projects_cla_groups.NewMockRepository(ctrl)
			mockProjectClaGroupsRepo.EXPECT().GetProjectsIdsForClaGroup(ctx, "cla-group-1").
				Return([]*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "proj-sfid"}}, nil).AnyTimes()

			service := NewService(awsSession, "", nil, mockCompanyService, nil, mockProjectClaGroupsRepo, mockRepo, nil, nil)

			result, err := service.InvalidateECLA(ctx, "cla-group-1", "sig-1", tc.authUser, nil, eclaEventArgs(), nil)
			assert.Nil(t, result)
			if assert.Error(t, err) {
				assert.ErrorIs(t, err, tc.expectedErr)
			}
		})
	}
}

type fakeEclaInvalidateService struct {
	ServiceInterface
	result        *models.EclaInvalidateResult
	err           error
	gotClaGroupID string
	gotSigID      string
	gotInput      *models.EclaInvalidationInput
}

func (f *fakeEclaInvalidateService) InvalidateECLA(_ context.Context, claGroupID string, signatureID string, _ *auth.User, _ events.Service, _ *events.LogEventArgs, input *models.EclaInvalidationInput) (*models.EclaInvalidateResult, error) {
	f.gotClaGroupID = claGroupID
	f.gotSigID = signatureID
	f.gotInput = input
	return f.result, f.err
}

func TestInvalidateECLAHandlerMapping(t *testing.T) {
	testCases := []struct {
		name           string
		serviceErr     error
		expectedStatus int
	}{
		{name: "success", expectedStatus: http.StatusOK},
		{name: "not found", serviceErr: errEclaNotFound, expectedStatus: http.StatusNotFound},
		{name: "not an ecla", serviceErr: errNotEcla, expectedStatus: http.StatusBadRequest},
		{name: "wrong cla group", serviceErr: errEclaWrongClaGroup, expectedStatus: http.StatusBadRequest},
		{name: "forbidden", serviceErr: errEclaForbidden, expectedStatus: http.StatusForbidden},
		{name: "already invalidated", serviceErr: errEclaAlreadyInvalidated, expectedStatus: http.StatusConflict},
		{name: "unexpected failure", serviceErr: errors.New("dynamo down"), expectedStatus: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			api := operations.NewEasyclaAPI(nil)
			service := &fakeEclaInvalidateService{err: tc.serviceErr}
			if tc.serviceErr == nil {
				service.result = &models.EclaInvalidateResult{SignatureID: "sig-1", ClaGroupID: "cla-group-1", CompanyID: "company-1", UserID: "user-1"}
			}
			Configure(api, nil, nil, nil, nil, nil, nil, service, nil)
			require.NotNil(t, api.SignaturesInvalidateECLAHandler)

			username, email := "tester", "tester@example.com"
			recorder := httptest.NewRecorder()
			api.SignaturesInvalidateECLAHandler.Handle(sigOps.InvalidateECLAParams{
				HTTPRequest: httptest.NewRequest(http.MethodPut, "/v4/cla-group/cla-group-1/ecla/sig-1/invalidate", nil),
				XUSERNAME:   &username,
				XEMAIL:      &email,
				ClaGroupID:  "cla-group-1",
				SignatureID: "sig-1",
				Body:        models.EclaInvalidationInput{Reason: "compliance", Note: "per legal review"},
			}, &auth.User{UserName: "tester", Email: "tester@example.com", ACL: auth.ACL{Allowed: true}}).WriteResponse(recorder, runtime.JSONProducer())

			assert.Equal(t, tc.expectedStatus, recorder.Code)
			assert.Equal(t, "cla-group-1", service.gotClaGroupID)
			assert.Equal(t, "sig-1", service.gotSigID)
			if assert.NotNil(t, service.gotInput) {
				assert.Equal(t, "compliance", service.gotInput.Reason)
			}
			if tc.expectedStatus == http.StatusOK {
				assert.JSONEq(t, `{"signature_id":"sig-1","cla_group_id":"cla-group-1","company_id":"company-1","user_id":"user-1"}`, recorder.Body.String())
			}
		})
	}
}

func TestEclaInvalidateJSONContracts(t *testing.T) {
	result, err := json.Marshal(models.EclaInvalidateResult{})
	assert.Nil(t, err)
	assert.JSONEq(t, `{"signature_id":"","cla_group_id":"","company_id":"","user_id":""}`, string(result), "all result fields must serialize even when empty")

	var input models.EclaInvalidationInput
	err = json.Unmarshal([]byte(`{"reason":"compliance","note":"per legal review"}`), &input)
	assert.Nil(t, err)
	assert.Equal(t, "compliance", input.Reason)
	assert.Equal(t, "per legal review", input.Note)
}
