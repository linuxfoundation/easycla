// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"testing"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
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

func TestBestUserEmail(t *testing.T) {
	assert.Equal(t, "", bestUserEmail(nil))
	assert.Equal(t, "lf@example.org", bestUserEmail(&v1Models.User{LfEmail: "lf@example.org", Emails: []string{"other@example.org"}}))
	assert.Equal(t, "real@example.org", bestUserEmail(&v1Models.User{Emails: []string{"", "12345+octocat@users.noreply.github.com", "real@example.org"}}))
	assert.Equal(t, "", bestUserEmail(&v1Models.User{}))
}
