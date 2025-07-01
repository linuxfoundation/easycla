// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/linuxfoundation/easycla/cla-backend-go/cmd"
	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	token "github.com/linuxfoundation/easycla/cla-backend-go/token"
)

var (
	// version the application version
	version string

	// build/Commit the application build number
	commit string

	// branch the build branch
	branch string

	// build date
	buildDate string
)

func init() {
	ini.ConfigVariable()
	configFile := ini.GetConfig()
	token.Init(configFile.Auth0Platform.ClientID, configFile.Auth0Platform.ClientSecret, configFile.Auth0Platform.URL, configFile.Auth0Platform.Audience)
}

func main() {
	cmd.Version = version
	cmd.Commit = commit
	cmd.Branch = branch
	cmd.BuildDate = buildDate

	cmd.Execute()
}
