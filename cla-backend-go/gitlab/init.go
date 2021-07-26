// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

var gitlabAppPrivateKey string

// Init initializes the required gitlab variables
func Init(glAppID string, glAppPrivateKey string) {
	gitlabAppPrivateKey = glAppPrivateKey
}

func getGitlabAppPrivateKey() string {
	return gitlabAppPrivateKey
}
