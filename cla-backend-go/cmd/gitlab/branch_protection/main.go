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
	possibleDefaultBranch = "main"
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

	defaultBranch, err := getDefaultBranch(gitlabClient, *projectID)
	if err != nil {
		log.Fatalf("fetching the default branch failed : %v", err)
	}

	log.Println("the default branch found is : ", defaultBranch.Name)
	if err := setOrCreateProtection(gitlabClient, *projectID, defaultBranch.Name); err != nil {
		log.Fatalf("setting branch protection for : %s failed : %v", defaultBranch.Name, err)
	}

	log.Println("branch protection set for : ", defaultBranch.Name)
}

func setOrCreateProtection(client *gitlab.Client, projectID int, protectionPattern string) error {
	var err error

	protectedBranch, resp, err := client.ProtectedBranches.GetProtectedBranch(projectID, protectionPattern)
	if err != nil && resp.StatusCode != 404 {
		return fmt.Errorf("fetching existing branch failed : %v", err)
	}

	if protectedBranch != nil {
		if isProtectedBranchSet(protectedBranch) {
			log.Println("branch protection already set, nothing to do")
			return nil
		}
		//it's an existing one try to remove it first and re-create it
		log.Println("removing old branch protection for string : ", protectionPattern)
		_, err = client.ProtectedBranches.UnprotectRepositoryBranches(projectID, protectionPattern)
		if err != nil {
			return fmt.Errorf("removing protection for existing branch failed : %v", err)
		}
	}

	log.Println("re-creating branch protection for string ", protectionPattern)
	if _, err = createBranchProtection(client, projectID, protectionPattern); err != nil {
		return fmt.Errorf("recreating : %v", err)
	}
	return nil
}

func createBranchProtection(client *gitlab.Client, projectID int, name string) (*gitlab.ProtectedBranch, error) {
	protectedBranch, _, err := client.ProtectedBranches.ProtectRepositoryBranches(projectID, &gitlab.ProtectRepositoryBranchesOptions{
		Name:                      gitlab.String(name),
		PushAccessLevel:           gitlab.AccessLevel(gitlab.NoPermissions),
		MergeAccessLevel:          gitlab.AccessLevel(gitlab.MaintainerPermissions),
		UnprotectAccessLevel:      nil,
		AllowForcePush:            gitlab.Bool(false),
		AllowedToPush:             nil,
		AllowedToMerge:            nil,
		AllowedToUnprotect:        nil,
		CodeOwnerApprovalRequired: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("creating new branch protection failed : %v", err)
	}
	return protectedBranch, nil
}

func isProtectedBranchSet(protectedBranch *gitlab.ProtectedBranch) bool {
	//log.Println("checking branch protection for : ", spew.Sdump(protectedBranch))
	if protectedBranch.AllowForcePush {
		return false
	}

	if len(protectedBranch.PushAccessLevels) != 1 {
		return false
	}

	if protectedBranch.PushAccessLevels[0].AccessLevel != gitlab.NoPermissions {
		return false
	}

	if len(protectedBranch.MergeAccessLevels) != 1 {
		return false
	}

	if protectedBranch.MergeAccessLevels[0].AccessLevel != gitlab.MaintainerPermissions {
		return false
	}

	if len(protectedBranch.UnprotectAccessLevels) != 1 {
		return false
	}

	if protectedBranch.UnprotectAccessLevels[0].AccessLevel != gitlab.MaintainerPermissions {
		return false
	}

	return true
}

// finds the default branch for the given project
func getDefaultBranch(client *gitlab.Client, projectID int) (*gitlab.Branch, error) {
	project, _, err := client.Projects.GetProject(projectID, &gitlab.GetProjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetching project failed : %v", err)
	}

	defaultBranch := project.DefaultBranch

	//	first try with the possible option
	branch, _, err := client.Branches.GetBranch(projectID, defaultBranch)
	if err != nil {
		return nil, fmt.Errorf("fetching default branch failed : %v", err)
	}

	return branch, nil
}
