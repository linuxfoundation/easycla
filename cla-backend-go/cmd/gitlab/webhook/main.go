// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"os"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/xanzy/go-gitlab"
)

const (
	hookURL = "https://7e182f2774e2.ngrok.io/gitlab/events"
)

func main() {
	log.Println("register webhook")
	access_token := os.Getenv("GITLAB_ACCESS_TOKEN")
	if access_token == "" {
		log.Fatal("GITLAB_ACCESS_TOKEN is required")
	}

	log.Infof("The gitlab access token is : %s", access_token)

	gitlabClient, err := gitlab.NewOAuthClient(access_token)
	if err != nil {
		log.Fatalf("creating client failed : %v", err)
	}

	user, _, err := gitlabClient.Users.CurrentUser()
	if err != nil {
		log.Fatalf("fetching current user failed : %v", err)
	}

	log.Infof("fetched current user : %s", user.Name)

	projects, _, err := gitlabClient.Projects.ListUserProjects(user.ID, &gitlab.ListProjectsOptions{})
	if err != nil {
		log.Fatalf("listing projects failed : %v", err)
	}
	log.Printf("we fetched : %d projects for the account", len(projects))
	for _, p := range projects {
		log.Println("**********************")
		log.Println("Name : ", p.Name)
		log.Println("ID: ", p.ID)
		hooks, _, err := gitlabClient.Projects.ListProjectHooks(p.ID, &gitlab.ListProjectHooksOptions{})

		if err != nil {
			log.Fatalf("fetching hooks for project : %s, failed : %v", p.Name, err)
		}

		var claHookFound bool
		for _, hook := range hooks {
			log.Println("**********************")
			log.Infof("hook ID : %d", hook.ID)
			log.Infof("URL : %s", hook.URL)
			log.Infof("Merge Request Events Enabled : %v", hook.MergeRequestsEvents)
			log.Infof("Enable SSL Verification : %v", hook.EnableSSLVerification)

			if hookURL == hook.URL {
				claHookFound = true
				break
			}
		}

		if claHookFound {
			log.Infof("CLA Hook was found nothing to do")
			continue
		}

		log.Infof("adding webhook to the project : %s (%d)", p.Name, p.ID)
		if err := addCLAHookToProject(gitlabClient, p.ID); err != nil {
			log.Fatalf("adding hook to the project : %s (%d) failed : %v", p.Name, p.ID, err)
		}

	}

}

func addCLAHookToProject(gitlabClient *gitlab.Client, projectID int) error {
	_, _, err := gitlabClient.Projects.AddProjectHook(projectID, &gitlab.AddProjectHookOptions{
		URL:                   gitlab.String(hookURL),
		MergeRequestsEvents:   gitlab.Bool(true),
		EnableSSLVerification: gitlab.Bool(false),
	})
	return err
}
