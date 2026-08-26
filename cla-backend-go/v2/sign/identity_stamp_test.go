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
	mock_v1_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/stretchr/testify/assert"
)

// ICLA rows must carry the signer's identity themselves - MyCLAs and the PDF flows read it from
// the signature record, and the asynchronous dynamo-events back-fill has proven lossy
func TestStampUserIdentityFillsMissingAttributes(t *testing.T) {
	user := &v1Models.User{
		UserID:         "user-a",
		Username:       "Some One",
		LfUsername:     "someone",
		LfEmail:        "someone@example.org",
		GithubUsername: "octocat",
		GithubID:       "42",
		GitlabUsername: "glcat",
		GitlabID:       "43",
	}

	item := &signatures.ItemSignature{}
	stampUserIdentity(item, user)
	assert.Equal(t, "octocat", item.UserGithubUsername)
	assert.Equal(t, "42", item.UserGithubID)
	assert.Equal(t, "glcat", item.UserGitlabUsername)
	assert.Equal(t, "43", item.UserGitlabID)
	assert.Equal(t, "someone", item.UserLFUsername)
	assert.Equal(t, "Some One", item.UserName)
	assert.Equal(t, "someone@example.org", item.UserEmail)
}

func TestStampUserIdentityKeepsExistingAttributes(t *testing.T) {
	user := &v1Models.User{
		UserID:         "user-a",
		Username:       "Some One",
		LfUsername:     "someone",
		LfEmail:        "someone@example.org",
		GithubUsername: "octocat",
	}

	item := &signatures.ItemSignature{
		UserGithubUsername: "recorded-octocat",
		UserEmail:          "recorded@example.org",
	}
	stampUserIdentity(item, user)
	// attributes already on the record are authoritative - they captured the signing context
	assert.Equal(t, "recorded-octocat", item.UserGithubUsername)
	assert.Equal(t, "recorded@example.org", item.UserEmail)
	// missing ones are filled in
	assert.Equal(t, "someone", item.UserLFUsername)
	assert.Equal(t, "Some One", item.UserName)
}

func TestStampUserIdentityNilSafe(t *testing.T) {
	stampUserIdentity(nil, &v1Models.User{})
	item := &signatures.ItemSignature{}
	stampUserIdentity(item, nil)
	assert.Equal(t, signatures.ItemSignature{}, *item)
}

// the linchpin of the lossless-regenerate fix: the stored row must survive the full-record
// rewrite, with only the fields a regenerate legitimately changes overwritten
func TestRegeneratedItemSignaturePreservesStoredRow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSig := mock_v1_signatures.NewMockSignatureService(ctrl)
	svc := &service{signatureService: mockSig}

	stored := &signatures.ItemSignature{
		SignatureID:                   "sig-1",
		DateCreated:                   "2024-01-01T00:00:00Z",
		DateModified:                  "2024-02-01T00:00:00Z",
		SignatureReferenceType:        "user",
		SignatureType:                 "cla",
		SignatureReferenceID:          "user-1",
		SignatureProjectID:            "cla-group-1",
		SignatureApproved:             true,
		SignatureSigned:               true,
		SignedOn:                      "2024-01-01T00:00:00Z",
		SignatureEnvelopeID:           "envelope-1",
		UserGithubUsername:            "octocat",
		UserEmail:                     "someone@example.org",
		UserDocusignName:              "Some One",
		UserDocusignDateSigned:        "2024-01-01T00:00:00Z",
		Note:                          "invalidated once upon a time",
		SignatureReturnURL:            "https://old.example.org",
		SignatureReturnURLType:        "Github",
		SignatureCallbackURL:          "https://old.example.org/callback",
		SignatureACL:                  []string{"old-acl"},
		SignatureDocumentMajorVersion: 1,
		SignatureDocumentMinorVersion: 0,
	}
	mockSig.EXPECT().GetItemSignature(gomock.Any(), "sig-1").Return(stored, nil)

	latest := &v1Models.Signature{SignatureID: "sig-1", SignatureCreated: "2024-01-01T00:00:00Z"}
	item, err := svc.regeneratedItemSignature(context.Background(), latest, "https://new.example.org", "Gerrit", "https://new.example.org/callback", []string{"new-acl"}, 2, 1)
	assert.NoError(t, err)

	// preserved from the stored row
	assert.Equal(t, "2024-01-01T00:00:00Z", item.DateCreated)
	assert.True(t, item.SignatureSigned)
	assert.True(t, item.SignatureApproved)
	assert.Equal(t, "2024-01-01T00:00:00Z", item.SignedOn)
	assert.Equal(t, "envelope-1", item.SignatureEnvelopeID)
	assert.Equal(t, "octocat", item.UserGithubUsername)
	assert.Equal(t, "someone@example.org", item.UserEmail)
	assert.Equal(t, "Some One", item.UserDocusignName)
	assert.Equal(t, "2024-01-01T00:00:00Z", item.UserDocusignDateSigned)
	assert.Equal(t, "invalidated once upon a time", item.Note)
	// overwritten by the regenerate
	assert.Equal(t, "https://new.example.org", item.SignatureReturnURL)
	assert.Equal(t, "Gerrit", item.SignatureReturnURLType)
	assert.Equal(t, "https://new.example.org/callback", item.SignatureCallbackURL)
	assert.Equal(t, []string{"new-acl"}, item.SignatureACL)
	assert.Equal(t, 2, item.SignatureDocumentMajorVersion)
	assert.Equal(t, 1, item.SignatureDocumentMinorVersion)
	assert.True(t, item.SignatureEmbargoAcked)
	assert.NotEmpty(t, item.DateModified)
	assert.NotEqual(t, "2024-02-01T00:00:00Z", item.DateModified)
}

func TestRegeneratedItemSignatureFallsBackWhenRowNotFound(t *testing.T) {
	latest := &v1Models.Signature{
		SignatureID:                   "sig-1",
		SignatureCreated:              "2024-01-01T00:00:00Z",
		SignatureReferenceType:        "user",
		SignatureType:                 "cla",
		SignatureReferenceID:          "user-1",
		ProjectID:                     "cla-group-1",
		SignatureApproved:             true,
		SignatureSigned:               true,
		SignedOn:                      "2024-01-01T00:00:00Z",
		SignatureEnvelopeID:           "envelope-1",
		SignatureReferenceName:        "Some One",
		SignatureReferenceNameLower:   "some one",
		SignatureDocumentMajorVersion: "1",
		SignatureDocumentMinorVersion: "0",
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSig := mock_v1_signatures.NewMockSignatureService(ctrl)
	// a genuine not-found (nil, nil) is the only case allowed to rebuild from the API model
	mockSig.EXPECT().GetItemSignature(gomock.Any(), "sig-1").Return(nil, nil)
	svc := &service{signatureService: mockSig}

	item, err := svc.regeneratedItemSignature(context.Background(), latest, "https://new.example.org", "Gerrit", "https://new.example.org/callback", []string{"new-acl"}, 2, 1)
	assert.NoError(t, err)

	// reconstructed from the API model, incl. the previously dropped creation date
	assert.Equal(t, "sig-1", item.SignatureID)
	assert.Equal(t, "2024-01-01T00:00:00Z", item.DateCreated)
	assert.Equal(t, "user", item.SignatureReferenceType)
	assert.Equal(t, "cla", item.SignatureType)
	assert.Equal(t, "user-1", item.SignatureReferenceID)
	assert.Equal(t, "cla-group-1", item.SignatureProjectID)
	assert.True(t, item.SignatureApproved)
	assert.True(t, item.SignatureSigned)
	assert.True(t, item.SignatureEmbargoAcked)
	assert.Equal(t, "envelope-1", item.SignatureEnvelopeID)
	assert.Equal(t, "https://new.example.org", item.SignatureReturnURL)
	assert.Equal(t, "Gerrit", item.SignatureReturnURLType)
	assert.Equal(t, []string{"new-acl"}, item.SignatureACL)
	assert.Equal(t, 2, item.SignatureDocumentMajorVersion)
	assert.Equal(t, 1, item.SignatureDocumentMinorVersion)
}

// a transient read error must abort the regenerate instead of rewriting the row from the lossy
// API model - the caller fails the request and the user simply retries
func TestRegeneratedItemSignaturePropagatesLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSig := mock_v1_signatures.NewMockSignatureService(ctrl)
	loadErr := errors.New("dynamo down")
	mockSig.EXPECT().GetItemSignature(gomock.Any(), "sig-1").Return(nil, loadErr)
	svc := &service{signatureService: mockSig}

	latest := &v1Models.Signature{SignatureID: "sig-1", SignatureCreated: "2024-01-01T00:00:00Z"}
	item, err := svc.regeneratedItemSignature(context.Background(), latest, "https://new.example.org", "Gerrit", "https://new.example.org/callback", []string{"new-acl"}, 2, 1)
	assert.ErrorIs(t, err, loadErr)
	assert.Equal(t, signatures.ItemSignature{}, item)
}
