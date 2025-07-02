// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"context"
	"fmt"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

// SetOrCreateBranchProtection sets the required parameters if existing pattern exists or creates a new one
func SetOrCreateBranchProtection(ctx context.Context, client *gitlab.Client, projectID int, protectionPattern string) error {
	var err error
	f := logrus.Fields{
		"functionName":      "gitlab_api.SetOrCreateBranchProtection",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"gitlabProjectID":   projectID,
		"protectionPattern": protectionPattern,
	}

	log.WithFields(f).Debugf("setting branch protection...")

	protectedBranch, resp, err := client.ProtectedBranches.GetProtectedBranch(projectID, protectionPattern)
	if err != nil && resp.StatusCode != 404 {
		return fmt.Errorf("fetching existing branch failed : %v", err)
	}

	if protectedBranch != nil {
		if isProtectedBranchSet(protectedBranch) {
			log.WithFields(f).Debugf("branch protection already set nothing to do")
			return nil
		}

		//it's an existing one try to remove it first and re-create it
		log.WithFields(f).Debugf("removing old branch protection")
		_, err = client.ProtectedBranches.UnprotectRepositoryBranches(projectID, protectionPattern)
		if err != nil {
			return fmt.Errorf("removing protection for existing branch failed : %v", err)
		}
	}

	log.WithFields(f).Debugf("re-creating branch protection ")
	if _, err = createBranchProtection(client, projectID, protectionPattern); err != nil {
		return fmt.Errorf("recreating branch protection failed : %v", err)
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

// GetDefaultBranch finds the default branch for the given project
func GetDefaultBranch(client *gitlab.Client, projectID int) (*gitlab.Branch, error) {
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
