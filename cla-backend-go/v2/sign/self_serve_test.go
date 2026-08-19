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
	assert.Equal(t, "github:26589865", selfServeSignatureACL(&v1Models.User{GithubID: "26589865", GitlabID: "77", LfUsername: "lgryglicki"}))
	assert.Equal(t, "gitlab:77", selfServeSignatureACL(&v1Models.User{GitlabID: "77", LfUsername: "lgryglicki"}))
	assert.Equal(t, "lgryglicki", selfServeSignatureACL(&v1Models.User{LfUsername: "lgryglicki"}))
}
