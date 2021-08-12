// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var glClientID = "124453345"
var glClientKey = "0WqnDWHnZKo2cmQ8m93EtY9ZBpfzQW4UnnEuRmgtJKM="
var oauthRespStr = `{"access_token":"a30671b8749ba5d48925712344377f11a5aba43ec630f099e464b9843796e6a6","token_type":"Bearer","expires_in":0,"refresh_token":"0838a31d0d796973eacefdf513523e6e47aa06fac9d26622964da1e473509458","created_at":1626435922}`

func TestNewGitlabOauthClient(t *testing.T) {
	Init(glClientID, glClientKey)
	t.Cleanup(func() {
		gitLabAppPrivateKey = ""
	})

	t.Logf("app private ID is : %s", getGitLabAppID())
	t.Logf("app private key is : %s", getGitLabAppPrivateKey())

	var oauthResp OauthSuccessResponse
	err := json.Unmarshal([]byte(oauthRespStr), &oauthResp)
	assert.NoError(t, err)

	encrypted, err := EncryptAuthInfo(&oauthResp)
	assert.NoError(t, err)

	client, err := NewGitlabOauthClient(encrypted)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestEncryptDecryptAuthInfo(t *testing.T) {
	Init(glClientID, glClientKey)
	t.Cleanup(func() {
		gitLabAppPrivateKey = ""
	})

	t.Logf("app private ID is : %s", getGitLabAppID())
	t.Logf("app private key is : %s", getGitLabAppPrivateKey())

	var oauthResp OauthSuccessResponse
	err := json.Unmarshal([]byte(oauthRespStr), &oauthResp)
	assert.NoError(t, err)
	t.Logf("unmarshall ok : %+v", oauthResp)

	encrypted, err := EncryptAuthInfo(&oauthResp)
	assert.NoError(t, err)
	t.Logf("encrypted auth info : %s", encrypted)

	oauthRespDecrypted, err := DecryptAuthInfo(encrypted)
	assert.NoError(t, err)

	assert.Equal(t, &oauthResp, oauthRespDecrypted)
}
