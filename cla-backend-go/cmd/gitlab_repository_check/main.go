// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import "github.com/linuxfoundation/easycla/cla-backend-go/cmd/gitlab_repository_check/handler"

func main() {
	handler.Init()
	handler.RunHandler()
}
