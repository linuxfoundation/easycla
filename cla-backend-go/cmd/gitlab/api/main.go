// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/xanzy/go-gitlab"
)

const (
	ProjectsURL = "https://gitlab.com/api/v4/projects"
)

var state = flag.String("state", "failed", "the state of the MR to set")

func main() {
	flag.Parse()

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
		log.Println("Name : ", p.Name)
		log.Println("ID: ", p.ID)
	}

	projectID := 28118160
	commitSha := "f7036ab67a4e464e83e16af0b02d447c53fffa74"

	statuses, _, err := gitlabClient.Commits.GetCommitStatuses(projectID, commitSha,
		&gitlab.GetCommitStatusesOptions{})
	if err != nil {
		log.Fatalf("fetching commit statuses failed : %v", err)
	}

	if len(statuses) == 0 {
		log.Infof("no statuses found for commit sha")
		setState := gitlab.Failed
		if *state != string(gitlab.Failed) {
			setState = gitlab.Success
		}

		_, _, err = gitlabClient.Commits.SetCommitStatus(projectID, commitSha, &gitlab.SetCommitStatusOptions{
			State:       setState,
			Name:        gitlab.String("easyCLA Bot"),
			Description: gitlab.String(getDescription(setState)),
			TargetURL:   gitlab.String(getTargetURL("deniskurov@gmail.com")),
		})
		if err != nil {
			log.Fatalf("setting commit status for the sha failed : %v", err)
		}

		statuses, _, err = gitlabClient.Commits.GetCommitStatuses(projectID, commitSha,
			&gitlab.GetCommitStatusesOptions{})
		if err != nil {
			log.Fatalf("fetching commit statuses failed : %v", err)
		}

	}

	for _, status := range statuses {
		log.Println("Status : ", status.Status)
		if status.Status != *state {
			log.Infof("setting state of commit sha to %s", *state)
			_, _, err = gitlabClient.Commits.SetCommitStatus(projectID, commitSha, &gitlab.SetCommitStatusOptions{
				State:       gitlab.BuildStateValue(*state),
				Name:        gitlab.String("easyCLA Bot"),
				Description: gitlab.String(getDescription(gitlab.BuildStateValue(*state))),
				TargetURL:   gitlab.String(getTargetURL("deniskurov@gmail.com")),
			})
			if err != nil {
				log.Fatalf("setting commit status for the sha failed : %v", err)
			}
		}
		log.Println("Status Name : ", status.Name)
		log.Println("Status Description : ", status.Description)
		log.Println("Status Author : ", status.Author.Name)
		log.Println("Status Author Email : ", status.Author.Email)
	}
}

func getDescription(status gitlab.BuildStateValue) string {
	if status == gitlab.Failed {
		return "User hasn't signed CLA"
	}
	return "User signed CLA"
}

func getTargetURL(email string) string {
	return fmt.Sprintf("http://localhost:8080/gitlab/sign/%s", email)
}
