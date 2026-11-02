// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	mock_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sessionStore struct {
	store.Repository
	deleted []string
}

func (s *sessionStore) DeleteActiveSignatureMetaData(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}

// the Self Serve callback removes the signing session only for a completion that was processed and only for the signer's own session
func TestSelfServeCallbackRemovesOnlyTheProcessedSignersSession(t *testing.T) {
	payload := envelopeXML(recipientStatuses(DocusignCompleted), envelopeDocumentStatuses)
	withSeam := func(t *testing.T, sig *v1Models.Signature, err error) {
		previous := completeIndividualGerrit
		completeIndividualGerrit = func(_ *service, _ context.Context, _ []byte, _ string) (*v1Models.Signature, error) { return sig, err }
		t.Cleanup(func() { completeIndividualGerrit = previous })
	}

	t.Run("an ignored completion keeps the session", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		signatureService := mock_signatures.NewMockSignatureService(ctrl)
		signatureService.EXPECT().GetSignature(gomock.Any(), "s1").Return(&v1Models.Signature{SignatureID: "s1", ProjectID: "group-a", SignatureReferenceID: "u1"}, nil)
		signatureService.EXPECT().GetItemSignature(gomock.Any(), "s1").Return(&signatures.ItemSignature{SignatureID: "s1", SignatureEnvelopeID: "e2"}, nil)
		sessions := &sessionStore{}
		require.NoError(t, (&service{signatureService: signatureService, storeRepository: sessions}).SignedIndividualCallbackSelfServe(context.Background(), payload, "u1"))
		assert.Empty(t, sessions.deleted, "a foreign envelope must not drop the user's Self Serve session")
	})
	t.Run("a processed completion removes the signer's session", func(t *testing.T) {
		withSeam(t, &v1Models.Signature{SignatureID: "s1", SignatureReferenceID: "u1"}, nil)
		sessions := &sessionStore{}
		require.NoError(t, (&service{storeRepository: sessions}).SignedIndividualCallbackSelfServe(context.Background(), payload, "u1"))
		assert.Equal(t, []string{"active_signature:u1"}, sessions.deleted)
	})
	t.Run("a completion for another user leaves the route user's session", func(t *testing.T) {
		withSeam(t, &v1Models.Signature{SignatureID: "s1", SignatureReferenceID: "u2"}, nil)
		sessions := &sessionStore{}
		require.NoError(t, (&service{storeRepository: sessions}).SignedIndividualCallbackSelfServe(context.Background(), payload, "u1"))
		assert.Empty(t, sessions.deleted, "the route user ID is unauthenticated input and must not select the session to drop")
	})
	t.Run("a non-completed envelope keeps the session", func(t *testing.T) {
		withSeam(t, nil, nil)
		sessions := &sessionStore{}
		require.NoError(t, (&service{storeRepository: sessions}).SignedIndividualCallbackSelfServe(context.Background(), payload, "u1"))
		assert.Empty(t, sessions.deleted)
	})
	t.Run("a processing error is returned and keeps the session", func(t *testing.T) {
		boom := errors.New("docusign down")
		withSeam(t, nil, boom)
		sessions := &sessionStore{}
		assert.True(t, errors.Is((&service{storeRepository: sessions}).SignedIndividualCallbackSelfServe(context.Background(), payload, "u1"), boom))
		assert.Empty(t, sessions.deleted)
	})
}
