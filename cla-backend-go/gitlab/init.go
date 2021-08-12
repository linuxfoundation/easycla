// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

var gitLabAppPrivateKey string
var gitLabAppID string

// Init initializes the required gitlab variables
func Init(glAppID string, glAppPrivateKey string) {
	gitLabAppID = glAppID
	gitLabAppPrivateKey = glAppPrivateKey
}

func getGitLabAppID() string {
	return gitLabAppID
}

func getGitLabAppPrivateKey() string {
	return gitLabAppPrivateKey
}
