// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

var githubAppPrivateKey string
var githubAppID int
var secretAccessToken string

// Init initializes the required GitHub variables
func Init(ghAppID int, ghAppPrivateKey string, secAccessToken string) {
	githubAppPrivateKey = ghAppPrivateKey
	githubAppID = ghAppID
	secretAccessToken = secAccessToken
}

func getGithubAppPrivateKey() string {
	return githubAppPrivateKey
}

func getGithubAppID() int {
	return githubAppID
}

func getSecretAccessToken() string {
	return secretAccessToken
}
