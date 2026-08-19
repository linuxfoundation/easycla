// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"testing"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

const selfServeUserID = "6c2d5a11-0e2e-4f5a-9a0f-1f0a3b4c5d6e"

func selfServeMetadata(returnURL string) map[string]interface{} {
	metadata := map[string]interface{}{
		"user_id":    selfServeUserID,
		"project_id": "aa47b3e1-6f9c-4b6a-9f16-0f9d6a2e1c11",
		"source":     utils.SelfServeSignatureSource,
	}
	if returnURL != "" {
		metadata["return_url"] = returnURL
	}
	return metadata
}

func TestGetIndividualSignatureCallbackURLSelfServe(t *testing.T) {
	svc := &service{ClaV4ApiURL: "https://api.dev.lfx.linuxfoundation.org"}

	callbackURL, err := svc.getIndividualSignatureCallbackURL(context.Background(), selfServeUserID, selfServeMetadata(""))

	assert.NoError(t, err)
	assert.Equal(t, "https://api.dev.lfx.linuxfoundation.org/v4/signed/self-serve/individual/"+selfServeUserID, callbackURL)
}

func TestGetActiveSignatureReturnURLSelfServe(t *testing.T) {
	svc := &service{}

	returnURL, err := svc.getActiveSignatureReturnURL(context.Background(), selfServeUserID, selfServeMetadata("https://openprofile.dev/my-clas"))
	assert.NoError(t, err)
	assert.Equal(t, "https://openprofile.dev/my-clas", returnURL)

	returnURL, err = svc.getActiveSignatureReturnURL(context.Background(), selfServeUserID, selfServeMetadata(""))
	assert.NoError(t, err)
	assert.Equal(t, "", returnURL)
}

func TestSelfServeSignatureACL(t *testing.T) {
	noACL := map[string]interface{}{}
	assert.Equal(t, "github:26589865", selfServeSignatureACL(noACL, &v1Models.User{GithubID: "26589865", GitlabID: "77", LfUsername: "lgryglicki"}))
	assert.Equal(t, "gitlab:77", selfServeSignatureACL(noACL, &v1Models.User{GitlabID: "77", LfUsername: "lgryglicki"}))
	assert.Equal(t, "lgryglicki", selfServeSignatureACL(noACL, &v1Models.User{LfUsername: "lgryglicki"}))
}

func TestSelfServeSignatureACLPrefersTheSessionIdentity(t *testing.T) {
	metadata := selfServeMetadata("")
	metadata["acl"] = "gitlab:77"

	// the record's GitHub identity would otherwise win, but the session was prepared under GitLab
	assert.Equal(t, "gitlab:77", selfServeSignatureACL(metadata, &v1Models.User{GithubID: "26589865", GitlabID: "77"}))

	metadata["acl"] = "   "
	assert.Equal(t, "github:26589865", selfServeSignatureACL(metadata, &v1Models.User{GithubID: "26589865", GitlabID: "77"}))

	metadata["acl"] = 26589865
	assert.Equal(t, "github:26589865", selfServeSignatureACL(metadata, &v1Models.User{GithubID: "26589865", GitlabID: "77"}))
}

func TestGetIndividualSignatureCallbackURLGitlabSelfServe(t *testing.T) {
	svc := &service{ClaV4ApiURL: "https://api.dev.lfx.linuxfoundation.org"}

	callbackURL, err := svc.getIndividualSignatureCallbackURLGitlab(context.Background(), selfServeUserID, selfServeMetadata(""))

	assert.NoError(t, err)
	assert.Equal(t, "https://api.dev.lfx.linuxfoundation.org/v4/signed/self-serve/individual/"+selfServeUserID, callbackURL)
}

const envelopeDocumentStatuses = `<DocumentStatuses><DocumentStatus><ID>1</ID></DocumentStatus></DocumentStatuses>`

func envelopeXML(recipientStatuses, documentStatuses string) []byte {
	return []byte(`<?xml version="1.0" encoding="utf-8"?><DocuSignEnvelopeInformation><EnvelopeStatus><EnvelopeID>e1</EnvelopeID>` +
		recipientStatuses + documentStatuses + `</EnvelopeStatus></DocuSignEnvelopeInformation>`)
}

func recipientStatuses(status string) string {
	return `<RecipientStatuses><RecipientStatus><Status>` + status + `</Status><ClientUserId>s1</ClientUserId></RecipientStatus></RecipientStatuses>`
}

func TestParseEnvelope(t *testing.T) {
	info, err := parseEnvelope(envelopeXML(recipientStatuses(DocusignCompleted), envelopeDocumentStatuses))
	assert.NoError(t, err)
	assert.Equal(t, DocusignCompleted, info.EnvelopeStatus.RecipientStatuses[0].Status)

	info, err = parseEnvelope(envelopeXML(recipientStatuses("Sent"), envelopeDocumentStatuses))
	assert.NoError(t, err)
	assert.Equal(t, "Sent", info.EnvelopeStatus.RecipientStatuses[0].Status)

	// the shared processing indexes both lists, so neither may be empty
	_, err = parseEnvelope(envelopeXML("", envelopeDocumentStatuses))
	assert.Error(t, err)

	_, err = parseEnvelope(envelopeXML(recipientStatuses(DocusignCompleted), ""))
	assert.Error(t, err)

	_, err = parseEnvelope([]byte(`<?xml version="1.0" encoding="utf-8"?><DocuSignEnvelopeInformation></DocuSignEnvelopeInformation>`))
	assert.Error(t, err)

	_, err = parseEnvelope([]byte("not xml"))
	assert.Error(t, err)
}

func TestSignedIndividualCallbackSelfServeRejectsAnEmptyEnvelope(t *testing.T) {
	// no collaborators are wired, so reaching the shared processing or the store would panic -
	// the validation has to happen before either
	svc := &service{}

	for _, payload := range [][]byte{
		[]byte("not xml"),
		envelopeXML("", envelopeDocumentStatuses),
		envelopeXML(recipientStatuses(DocusignCompleted), ""),
	} {
		assert.Error(t, svc.SignedIndividualCallbackSelfServe(context.Background(), payload, selfServeUserID))
	}
}

func TestSelfServeSessionMatchesProject(t *testing.T) {
	claGroupID := "aa47b3e1-6f9c-4b6a-9f16-0f9d6a2e1c11"

	assert.True(t, selfServeSessionMatchesProject(selfServeMetadata(""), claGroupID))
	assert.False(t, selfServeSessionMatchesProject(selfServeMetadata(""), "62db1b81-6f4a-4b2e-9a4a-0f2d9f0a1b22"))

	// a session written without the key, or with a non-string one, keeps the previous behaviour
	assert.True(t, selfServeSessionMatchesProject(map[string]interface{}{"source": utils.SelfServeSignatureSource}, claGroupID))
	assert.True(t, selfServeSessionMatchesProject(map[string]interface{}{"project_id": 7}, claGroupID))
	assert.True(t, selfServeSessionMatchesProject(map[string]interface{}{"project_id": "   "}, claGroupID))
}
