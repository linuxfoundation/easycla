// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	eventsMock "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
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
