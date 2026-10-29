// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	signatureService "github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	mock_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
)

// the v4 GitHub organization approval list writes only run for a CLA manager of the exact signature
// (lfx-self-serve-ops#979)
func TestRequireSignatureManager(t *testing.T) {
	managed := &v1Models.Signature{SignatureID: "sig-1", ProjectID: "cla-group-1", SignatureReferenceID: "company-1", SignatureACL: []v1Models.User{{LfUsername: "manager"}}}
	unhydrated := &v1Models.Signature{SignatureID: "sig-1", ProjectID: "cla-group-1", SignatureReferenceID: "company-1"}
	stored := &signatureService.ItemSignature{SignatureID: "sig-1", SignatureACL: []string{"manager"}}
	cases := []struct {
		name       string
		user       *auth.User
		signature  *v1Models.Signature
		readErr    error
		lookups    int
		row        *signatureService.ItemSignature
		rowErr     error
		rowLookups int
		allowed    bool
	}{
		{name: "exact manager", user: &auth.User{UserName: "manager"}, signature: managed, lookups: 1, allowed: true},
		{name: "manager dropped by the hydration", user: &auth.User{UserName: "manager"}, signature: unhydrated, lookups: 1, row: stored, rowLookups: 1, allowed: true},
		{name: "another user", user: &auth.User{UserName: "outsider"}, signature: managed, lookups: 1, row: stored, rowLookups: 1},
		{name: "username in another case", user: &auth.User{UserName: "Manager"}, signature: managed, lookups: 1, row: stored, rowLookups: 1},
		{name: "empty identity", user: &auth.User{UserName: " "}, signature: managed},
		{name: "no user", signature: managed},
		{name: "empty acl", user: &auth.User{UserName: "manager"}, signature: &v1Models.Signature{SignatureID: "sig-1"}, lookups: 1, row: &signatureService.ItemSignature{SignatureID: "sig-1"}, rowLookups: 1},
		{name: "stored row missing", user: &auth.User{UserName: "outsider"}, signature: managed, lookups: 1, rowLookups: 1},
		{name: "stored row unreadable", user: &auth.User{UserName: "outsider"}, signature: managed, lookups: 1, rowErr: errors.New("dynamo down"), rowLookups: 1},
		{name: "unknown signature", user: &auth.User{UserName: "manager"}, lookups: 1},
		{name: "read failure", user: &auth.User{UserName: "manager"}, readErr: errors.New("dynamo down"), lookups: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			sigService := mock_signatures.NewMockSignatureService(ctrl)
			sigService.EXPECT().GetSignature(gomock.Any(), "sig-1").Return(tc.signature, tc.readErr).Times(tc.lookups)
			sigService.EXPECT().GetItemSignature(gomock.Any(), "sig-1").Return(tc.row, tc.rowErr).Times(tc.rowLookups)
			signature, err := requireSignatureManager(context.Background(), logrus.Fields{}, sigService, "sig-1", tc.user)
			if tc.allowed {
				require.NoError(t, err)
				assert.Same(t, tc.signature, signature)
				return
			}
			require.Error(t, err)
			assert.Nil(t, signature)
			if tc.readErr != nil {
				assert.ErrorIs(t, err, tc.readErr)
			}
			if tc.rowErr != nil {
				assert.ErrorIs(t, err, tc.rowErr)
			}
		})
	}
}
