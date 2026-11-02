// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	mock_signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/cla_groups"
	"github.com/stretchr/testify/assert"
)

// a completion callback is applied only for the signature's own envelope, read from the raw row (lfx-self-serve-ops#347)
func TestSignedIndividualCallbackGithubIgnoresForeignOrUnnamedEnvelopes(t *testing.T) {
	payload := envelopeXML(recipientStatuses(DocusignCompleted), envelopeDocumentStatuses) // envelope e1, signature s1
	unnamed := []byte(strings.Replace(string(payload), "<EnvelopeID>e1</EnvelopeID>", "<EnvelopeID></EnvelopeID>", 1))
	cases := []struct {
		name    string
		payload []byte
		stored  string
	}{
		{"foreign envelope", payload, "e2"},
		{"signature without an envelope", payload, ""},
		{"callback without an envelope", unnamed, "e1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			signatureService := mock_signatures.NewMockSignatureService(ctrl)
			signatureService.EXPECT().GetSignature(gomock.Any(), "s1").Return(&v1Models.Signature{SignatureID: "s1", ProjectID: "group-a"}, nil)
			signatureService.EXPECT().GetItemSignature(gomock.Any(), "s1").Return(&signatures.ItemSignature{SignatureID: "s1", SignatureEnvelopeID: tc.stored}, nil)
			svc := &service{signatureService: signatureService}
			assert.NoError(t, svc.SignedIndividualCallbackGithub(context.Background(), tc.payload, "7", "1", "42"), "ignored callbacks are not errors: nothing is written and no status is posted")
		})
	}
}

// the API model never carries the envelope, so the guard must read the raw row: a matching row lets the completion proceed
func TestSignedIndividualCallbackGithubReadsTheEnvelopeFromTheRow(t *testing.T) {
	payload := envelopeXML(recipientStatuses(DocusignCompleted), envelopeDocumentStatuses)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	signatureService := mock_signatures.NewMockSignatureService(ctrl)
	signatureService.EXPECT().GetSignature(gomock.Any(), "s1").Return(&v1Models.Signature{SignatureID: "s1", ProjectID: "group-a"}, nil)
	signatureService.EXPECT().GetItemSignature(gomock.Any(), "s1").Return(&signatures.ItemSignature{SignatureID: "s1", SignatureEnvelopeID: "e1"}, nil)
	stop := errors.New("stop after the signature update")
	signatureService.EXPECT().UpdateSignature(gomock.Any(), "s1", gomock.Any()).Return(stop).Times(1)
	svc := &service{signatureService: signatureService}
	err := svc.SignedIndividualCallbackGithub(context.Background(), payload, "7", "1", "42")
	assert.True(t, errors.Is(err, stop), "the completion proceeded to the signature update")

	ctrl2 := gomock.NewController(t)
	defer ctrl2.Finish()
	failing := mock_signatures.NewMockSignatureService(ctrl2)
	failing.EXPECT().GetSignature(gomock.Any(), "s1").Return(&v1Models.Signature{SignatureID: "s1", ProjectID: "group-a"}, nil)
	rowErr := errors.New("dynamodb unavailable")
	failing.EXPECT().GetItemSignature(gomock.Any(), "s1").Return(nil, rowErr)
	err = (&service{signatureService: failing}).SignedIndividualCallbackGithub(context.Background(), payload, "7", "1", "42")
	assert.True(t, errors.Is(err, rowErr), "a failing row read is returned, not treated as a mismatch")
}

// a repeated completion of the signature's own envelope is processed again so a DocuSign retry can finish the remaining steps
func TestIgnoredCompletionReprocessesOwnEnvelope(t *testing.T) {
	assert.Empty(t, ignoredCompletion("s1", "e1", "e1"))
	assert.NotEmpty(t, ignoredCompletion("s1", "e1", "E1"))
}

func TestRepositoryBindingErrorsAreRecognisable(t *testing.T) {
	err := repositoryBoundToCLAGroup(&v1Models.GithubRepository{Enabled: true, RepositoryClaGroupID: "group-b"}, &v1Models.GithubOrganization{OrganizationInstallationID: 7}, 42, 7, "group-a")
	assert.True(t, errors.Is(err, errRepositoryNotBound))
}

// the GitLab and Gerrit callbacks (the self-serve one delegates to Gerrit) apply the same envelope rule
func TestOtherIndividualCallbacksApplyTheEnvelopeRule(t *testing.T) {
	payload := envelopeXML(recipientStatuses(DocusignCompleted), envelopeDocumentStatuses)
	callbacks := map[string]func(svc *service) error{
		"gitlab": func(svc *service) error {
			return svc.SignedIndividualCallbackGitlab(context.Background(), payload, "u1", "o1", "r1", "mr1")
		},
		"gerrit": func(svc *service) error {
			return svc.SignedIndividualCallbackGerrit(context.Background(), payload, "u1")
		},
	}
	for name, call := range callbacks {
		t.Run(name+" foreign envelope", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			signatureService := mock_signatures.NewMockSignatureService(ctrl)
			signatureService.EXPECT().GetSignature(gomock.Any(), "s1").Return(&v1Models.Signature{SignatureID: "s1", ProjectID: "group-a"}, nil)
			signatureService.EXPECT().GetItemSignature(gomock.Any(), "s1").Return(&signatures.ItemSignature{SignatureID: "s1", SignatureEnvelopeID: "e2"}, nil)
			assert.NoError(t, call(&service{signatureService: signatureService}), "ignored: nothing is written")
		})
		t.Run(name+" own envelope proceeds", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			signatureService := mock_signatures.NewMockSignatureService(ctrl)
			signatureService.EXPECT().GetSignature(gomock.Any(), "s1").Return(&v1Models.Signature{SignatureID: "s1", ProjectID: "group-a"}, nil)
			signatureService.EXPECT().GetItemSignature(gomock.Any(), "s1").Return(&signatures.ItemSignature{SignatureID: "s1", SignatureEnvelopeID: "e1"}, nil)
			stop := errors.New("stop after the signature update")
			signatureService.EXPECT().UpdateSignature(gomock.Any(), "s1", gomock.Any()).Return(stop).Times(1)
			assert.True(t, errors.Is(call(&service{signatureService: signatureService}), stop))
		})
	}
}

type callbackGroups struct {
	cla_groups.Service
	group *v1Models.ClaGroup
}

func (g *callbackGroups) GetCLAGroup(_ context.Context, _ string) (*v1Models.ClaGroup, error) {
	return g.group, nil
}

type callbackUsers struct {
	users.Service
	err error
}

func (u *callbackUsers) GetUser(_ string) (*v1Models.User, error) { return nil, u.err }

// the corporate callback resolves the signature by ClientUserId or by company/project and applies the envelope rule right after
func TestCorporateCallbackAppliesTheEnvelopeRule(t *testing.T) {
	byClientUser := envelopeXML(recipientStatuses(DocusignCompleted), envelopeDocumentStatuses)
	byCompany := envelopeXML(`<RecipientStatuses><RecipientStatus><Status>Completed</Status></RecipientStatus></RecipientStatuses>`, envelopeDocumentStatuses)
	signature := &v1Models.Signature{SignatureID: "s1", ProjectID: "group-a", SignatureReferenceType: "user", SignatureReferenceID: "u1"}
	row := func(envelope string) *signatures.ItemSignature {
		return &signatures.ItemSignature{SignatureID: "s1", SignatureProjectID: "group-a", SignatureReferenceType: "user", SignatureReferenceID: "u1", SignatureUserCompanyID: "c1", SignatureEnvelopeID: envelope}
	}
	newService := func(t *testing.T, signatureService *mock_signatures.MockSignatureService, userService users.Service) *service {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)
		companyRepo := mock_company.NewMockIRepository(ctrl)
		companyRepo.EXPECT().GetCompany(gomock.Any(), "c1").Return(&v1Models.Company{CompanyID: "c1"}, nil)
		return &service{claGroupService: &callbackGroups{group: &v1Models.ClaGroup{ProjectID: "group-a"}}, companyRepo: companyRepo, signatureService: signatureService, userService: userService}
	}

	t.Run("foreign envelope by ClientUserId", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		signatureService := mock_signatures.NewMockSignatureService(ctrl)
		signatureService.EXPECT().GetSignature(gomock.Any(), "s1").Return(signature, nil)
		signatureService.EXPECT().GetItemSignature(gomock.Any(), "s1").Return(row("e2"), nil)
		assert.NoError(t, newService(t, signatureService, nil).SignedCorporateCallback(context.Background(), byClientUser, "c1", "group-a"))
	})
	t.Run("foreign envelope by company and project", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		signatureService := mock_signatures.NewMockSignatureService(ctrl)
		signatureService.EXPECT().GetCorporateSignature(gomock.Any(), "group-a", "c1", gomock.Any(), gomock.Any()).Return(signature, nil)
		signatureService.EXPECT().GetItemSignature(gomock.Any(), "s1").Return(row("e2"), nil)
		assert.NoError(t, newService(t, signatureService, nil).SignedCorporateCallback(context.Background(), byCompany, "c1", "group-a"))
	})
	t.Run("own envelope proceeds", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		signatureService := mock_signatures.NewMockSignatureService(ctrl)
		signatureService.EXPECT().GetSignature(gomock.Any(), "s1").Return(signature, nil)
		signatureService.EXPECT().GetItemSignature(gomock.Any(), "s1").Return(row("e1"), nil)
		stop := errors.New("stop at the user lookup")
		err := newService(t, signatureService, &callbackUsers{err: stop}).SignedCorporateCallback(context.Background(), byClientUser, "c1", "group-a")
		assert.True(t, errors.Is(err, stop), "the guard let the callback through to the first lookup after it")
	})
	t.Run("signature of another company or CLA group is ignored", func(t *testing.T) {
		for name, other := range map[string]*signatures.ItemSignature{
			"other company":               {SignatureID: "s1", SignatureProjectID: "group-a", SignatureReferenceType: "company", SignatureReferenceID: "c2", SignatureEnvelopeID: "e1"},
			"other CLA group":             {SignatureID: "s1", SignatureProjectID: "group-b", SignatureReferenceType: "company", SignatureReferenceID: "c1", SignatureEnvelopeID: "e1"},
			"employee of another company": {SignatureID: "s1", SignatureProjectID: "group-a", SignatureReferenceType: "user", SignatureReferenceID: "u1", SignatureUserCompanyID: "c2", SignatureEnvelopeID: "e1"},
		} {
			ctrl := gomock.NewController(t)
			signatureService := mock_signatures.NewMockSignatureService(ctrl)
			signatureService.EXPECT().GetSignature(gomock.Any(), "s1").Return(signature, nil)
			signatureService.EXPECT().GetItemSignature(gomock.Any(), "s1").Return(other, nil)
			assert.NoError(t, newService(t, signatureService, nil).SignedCorporateCallback(context.Background(), byClientUser, "c1", "group-a"), name)
			ctrl.Finish()
		}
	})
}

// the token-less callbacks index the envelope's recipient and document statuses: a payload missing either is rejected
// instead of panicking the Lambda (lfx-self-serve-ops#734)
func TestLegacyCallbacksRejectEnvelopesWithoutStatuses(t *testing.T) {
	noRecipients := envelopeXML("", envelopeDocumentStatuses)
	noDocuments := envelopeXML(recipientStatuses(DocusignCompleted), "")
	callbacks := map[string]func(svc *service, payload []byte) error{
		"github": func(svc *service, payload []byte) error {
			return svc.SignedIndividualCallbackGithub(context.Background(), payload, "7", "1", "42")
		},
		"gitlab": func(svc *service, payload []byte) error {
			return svc.SignedIndividualCallbackGitlab(context.Background(), payload, "u1", "o1", "r1", "mr1")
		},
		"gerrit": func(svc *service, payload []byte) error {
			return svc.SignedIndividualCallbackGerrit(context.Background(), payload, "u1")
		},
		"corporate": func(svc *service, payload []byte) error {
			return svc.SignedCorporateCallback(context.Background(), payload, "c1", "group-a")
		},
	}
	for name, call := range callbacks {
		for payloadName, payload := range map[string][]byte{"no recipient statuses": noRecipients, "no document statuses": noDocuments, "not xml": []byte("<EnvelopeStatus>")} {
			t.Run(name+" "+payloadName, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				// no lookups may happen: the signature service mock has no expectations
				svc := &service{signatureService: mock_signatures.NewMockSignatureService(ctrl)}
				assert.Error(t, call(svc, payload))
			})
		}
	}
}

// a corporate callback for a company and project without a CCLA row is an error, not a nil dereference
func TestCorporateCallbackWithoutACorporateSignatureIsRejected(t *testing.T) {
	byCompany := envelopeXML(`<RecipientStatuses><RecipientStatus><Status>Completed</Status></RecipientStatus></RecipientStatuses>`, envelopeDocumentStatuses)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	companyRepo := mock_company.NewMockIRepository(ctrl)
	companyRepo.EXPECT().GetCompany(gomock.Any(), "c1").Return(&v1Models.Company{CompanyID: "c1"}, nil)
	signatureService := mock_signatures.NewMockSignatureService(ctrl)
	signatureService.EXPECT().GetCorporateSignature(gomock.Any(), "group-a", "c1", gomock.Any(), gomock.Any()).Return(nil, nil)
	svc := &service{claGroupService: &callbackGroups{group: &v1Models.ClaGroup{ProjectID: "group-a"}}, companyRepo: companyRepo, signatureService: signatureService}
	err := svc.SignedCorporateCallback(context.Background(), byCompany, "c1", "group-a")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
