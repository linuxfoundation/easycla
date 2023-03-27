// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"fmt"

	"github.com/xanzy/go-gitlab"
)

// SetWebHook is responsible for adding the webhook for given projectID, if webhook is there already
// tries to set the attributes if anything is missing, should be idempotent operation
func SetWebHook(gitLabClient *gitlab.Client, hookURL string, projectID int, token string) error {
	existingWebHook, err := findExistingWebHook(gitLabClient, hookURL, projectID)
	if err != nil {
		return err
	}

	if existingWebHook == nil {
		_, _, err = gitLabClient.Projects.AddProjectHook(projectID, &gitlab.AddProjectHookOptions{
			URL:                   gitlab.String(hookURL),
			MergeRequestsEvents:   gitlab.Bool(true),
			PushEvents:            gitlab.Bool(true),
			NoteEvents:            gitlab.Bool(true), // subscribe to comment events
			EnableSSLVerification: gitlab.Bool(true),
			Token:                 gitlab.String(token),
		})
		if err != nil {
			return fmt.Errorf("adding web hook for project : %d, failed : %v", projectID, err)
		}
		return nil
	}

	if !existingWebHook.EnableSSLVerification || !existingWebHook.MergeRequestsEvents || !existingWebHook.PushEvents {
		_, _, err = gitLabClient.Projects.EditProjectHook(projectID, existingWebHook.ID, &gitlab.EditProjectHookOptions{
			URL:                   gitlab.String(hookURL),
			MergeRequestsEvents:   gitlab.Bool(true),
			PushEvents:            gitlab.Bool(true),
			NoteEvents:            gitlab.Bool(true), // subscribe to comment events
			EnableSSLVerification: gitlab.Bool(true),
			Token:                 gitlab.String(token),
		})
		if err != nil {
			return fmt.Errorf("editing web hook for project : %d, failed : %v", projectID, err)
		}
	}

	return nil
}

// RemoveWebHook removes existing webhook from the given project
func RemoveWebHook(gitLabClient *gitlab.Client, hookURL string, projectID int) error {
	existingWebHook, err := findExistingWebHook(gitLabClient, hookURL, projectID)
	if err != nil {
		return err
	}

	if existingWebHook == nil {
		return nil
	}

	_, err = gitLabClient.Projects.DeleteProjectHook(projectID, existingWebHook.ID)
	return err

}

func findExistingWebHook(gitLabClient *gitlab.Client, hookURL string, projectID int) (*gitlab.ProjectHook, error) {
	hooks, _, err := gitLabClient.Projects.ListProjectHooks(projectID, &gitlab.ListProjectHooksOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetching hooks for project : %d, failed : %v", projectID, err)
	}

	var existingWebHook *gitlab.ProjectHook
	for _, hook := range hooks {
		if hook.URL == hookURL {
			existingWebHook = hook
			break
		}
	}

	return existingWebHook, nil
}
