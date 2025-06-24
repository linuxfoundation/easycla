// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"os"

	gitlab_api "github.com/linuxfoundation/easycla/cla-backend-go/gitlab_api"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/xanzy/go-gitlab"
)

var projectID = flag.Int("project", 0, "gitlab project id")

func main() {
	flag.Parse()

	if *projectID == 0 {
		log.Fatalf("gitlab project id is missing")
	}

	accessToken := os.Getenv("GITLAB_ACCESS_TOKEN")
	if accessToken == "" {
		log.Fatalf("GITLAB_ACCESS_TOKEN is required")
	}

	gitlabClient, err := gitlab.NewOAuthClient(accessToken)
	if err != nil {
		log.Fatalf("creating client failed : %v", err)
	}

	if err := gitlab_api.EnableMergePipelineProtection(context.Background(), gitlabClient, *projectID); err != nil {
		log.Fatalf("enabling merge pipeline protection failed : %v", err)
	}

	log.Println("merge pipeline protection enabled successfully")
}
