// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	eventsMock "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	sigs "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	mock_v1_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/stretchr/testify/assert"
)

func TestHasInvalidatedIcla(t *testing.T) {
	assert.False(t, hasInvalidatedIcla(nil))
	assert.False(t, hasInvalidatedIcla([]*v1Models.Signature{nil}))
	assert.False(t, hasInvalidatedIcla([]*v1Models.Signature{
		{SignatureID: "in-progress", SignatureSigned: false, SignatureApproved: true},
		{SignatureID: "valid", SignatureSigned: true, SignatureApproved: true},
		{SignatureID: "abandoned", SignatureSigned: false, SignatureApproved: false},
	}))
	assert.True(t, hasInvalidatedIcla([]*v1Models.Signature{
		{SignatureID: "valid", SignatureSigned: true, SignatureApproved: true},
		{SignatureID: "invalidated", SignatureSigned: true, SignatureApproved: false},
	}), "a signed but unapproved ICLA marks an administrator invalidation")
}

func TestUserHasInvalidatedIclaExhaustiveLookup(t *testing.T) {
	userName := "contributor"
	projectID := "cla-group-1"
	callParams := sigs.GetUserSignaturesParams{UserID: "user-1", UserName: &userName}

	tests := []struct {
		name        string
		signatures  []*v1Models.Signature
		lookupErr   error
		wantBlocked bool
		wantErr     bool
	}{
		{
			name: "an invalidated ICLA anywhere in the exhaustive result blocks",
			signatures: []*v1Models.Signature{
				{SignatureID: "valid", SignatureSigned: true, SignatureApproved: true},
				{SignatureID: "invalidated", SignatureSigned: true, SignatureApproved: false},
			},
			wantBlocked: true,
		},
		{
			name: "a clean history does not block",
			signatures: []*v1Models.Signature{
				{SignatureID: "valid", SignatureSigned: true, SignatureApproved: true},
			},
		},
		{
			name:      "a failed lookup is propagated",
			lookupErr: errors.New("dynamodb unavailable"),
			wantErr:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSignatures := mock_v1_signatures.NewMockSignatureService(ctrl)
			mockSignatures.EXPECT().GetUserSignatures(gomock.Any(), gomock.Any(), &projectID).DoAndReturn(
				func(_ context.Context, params sigs.GetUserSignaturesParams, _ *string) (*v1Models.Signatures, error) {
					if assert.NotNil(t, params.PageSize, "the block check must request an exhaustive page size, not the default of 10") {
						assert.GreaterOrEqual(t, *params.PageSize, int64(1000))
					}
					assert.Equal(t, "user-1", params.UserID)
					if tc.lookupErr != nil {
						return nil, tc.lookupErr
					}
					return &v1Models.Signatures{Signatures: tc.signatures}, nil
				})

			svc := &service{signatureService: mockSignatures}
			blocked, err := svc.userHasInvalidatedIcla(context.Background(), callParams, &projectID)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantBlocked, blocked)
			assert.Nil(t, callParams.PageSize, "the caller's default-sized params must stay untouched")
		})
	}
}

func TestLogCompanySanctionedEventIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	comp := &v1Models.Company{CompanyID: "company-1", CompanyName: "Flagged Corp"}
	tests := []struct {
		name           string
		userModel      *v1Models.User
		lfUsername     string
		wantUserID     string
		wantLfUsername string
	}{
		{
			name:           "a user model supplies the identity the events gate requires",
			userModel:      &v1Models.User{UserID: "user-1", LfUsername: "contributor"},
			wantUserID:     "user-1",
			wantLfUsername: "contributor",
		},
		{
			name:           "an explicit lf username is kept",
			userModel:      &v1Models.User{UserID: "user-1", LfUsername: "contributor"},
			lfUsername:     "manager",
			wantUserID:     "user-1",
			wantLfUsername: "manager",
		},
		{
			name:           "an lf username alone passes the gate",
			lfUsername:     "manager",
			wantLfUsername: "manager",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockEvents := eventsMock.NewMockService(ctrl)
			var logged *events.LogEventArgs
			mockEvents.EXPECT().LogEventWithContext(gomock.Any(), gomock.Any()).Do(
				func(_ context.Context, args *events.LogEventArgs) {
					logged = args
				})
			svc := &service{eventsService: mockEvents}
			svc.logCompanySanctionedEvent(context.Background(), comp, tc.userModel, tc.lfUsername)
			if assert.NotNil(t, logged) {
				assert.True(t, logged.UserID != "" || logged.LfUsername != "", "the events service drops events without a top-level user identity")
				assert.Equal(t, tc.wantUserID, logged.UserID)
				assert.Equal(t, tc.wantLfUsername, logged.LfUsername)
				assert.Same(t, comp, logged.CompanyModel)
				assert.Equal(t, events.CompanySanctioned, logged.EventType)
			}
		})
	}
}
