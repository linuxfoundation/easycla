// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"fmt"
	"strconv"
	"strings"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/xanzy/go-gitlab"
)

// FetchMrInfo is responsible for fetching the MR info for given project
func FetchMrInfo(client *gitlab.Client, projectID int, mergeID int) (*gitlab.MergeRequest, error) {
	m, _, err := client.MergeRequests.GetMergeRequest(projectID, mergeID, &gitlab.GetMergeRequestsOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetching merge request : %d for project : %v failed : %v", mergeID, projectID, err)
	}

	return m, nil
}

// FetchMrParticipants is responsible to get unique mr participants
func FetchMrParticipants(client *gitlab.Client, projectID int, mergeID int) ([]*gitlab.User, error) {
	commits, _, err := client.MergeRequests.GetMergeRequestCommits(projectID, mergeID, &gitlab.GetMergeRequestCommitsOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetching gitlab participants for project : %d and merge id : %d, failed : %v", projectID, mergeID, err)
	}

	if len(commits) == 0 {
		return nil, nil
	}

	var results []*gitlab.User

	for _, commit := range commits {
		authorEmail := commit.AuthorEmail
		authorName := commit.AuthorName
		log.Debugf("user email found : %s, user name : %s, searching in gitlab ...", authorEmail, authorName)

		authorID, err := strconv.Atoi(commit.ID)
		if err != nil {
			return nil, fmt.Errorf("parsing author id : %s, failed : %v", commit.ID, err)
		}

		user, err := getUser(client, authorID)

		if err != nil {
			return nil, fmt.Errorf("searching for author email : %s, failed : %v", authorEmail, err)
		}

		if user == nil {
			return nil, fmt.Errorf("no users found for commit author email : %s, name : %s", authorEmail, authorName)
		}
		results = append(results, user)
	}

	return results, nil
}

// SetCommitStatus is responsible for setting the MR status for commit sha
func SetCommitStatus(client *gitlab.Client, projectID int, commitSha string, state gitlab.BuildStateValue, message string, targetURL string) error {
	options := &gitlab.SetCommitStatusOptions{
		State:       state,
		Name:        gitlab.String("EasyCLA Bot"),
		Description: gitlab.String(message),
	}

	if targetURL != "" {
		options.TargetURL = gitlab.String(targetURL)
	}

	_, _, err := client.Commits.SetCommitStatus(projectID, commitSha, options)
	if err != nil {
		return fmt.Errorf("setting commit status for the sha : %s and project id : %d failed : %v", commitSha, projectID, err)
	}

	return nil
}

// SetMrComment is responsible for setting the comment body for project and merge id
func SetMrComment(client *gitlab.Client, projectID int, mergeID int, message string) error {

	notes, _, err := client.Notes.ListMergeRequestNotes(projectID, mergeID, &gitlab.ListMergeRequestNotesOptions{})
	if err != nil {
		return fmt.Errorf("fetching comments for project id : %d and merge id : %d : failed %v", projectID, mergeID, err)
	}

	var previousNote *gitlab.Note
	if len(notes) > 0 {
		for _, n := range notes {
			if strings.Contains(n.Body, "cla-signed.svg") || strings.Contains(n.Body, "cla-not-signed.svg") || strings.Contains(n.Body, "cla-missing-id.svg") || strings.Contains(n.Body, "cla-confirmation-needed.svg") {
				previousNote = n
				break
			}
		}
	}

	if previousNote == nil {
		log.Debugf("no previous comments found for project id : %d and merge id : %d", projectID, mergeID)
		_, _, err = client.Notes.CreateMergeRequestNote(projectID, mergeID, &gitlab.CreateMergeRequestNoteOptions{
			Body: &message,
		})
		if err != nil {
			return fmt.Errorf("creating comment for project id : %d and merge id : %d : failed %v", projectID, mergeID, err)
		}
	} else {
		log.Debugf("previous comments found for project id : %d and merge id : %d", projectID, mergeID)
		_, _, err = client.Notes.UpdateMergeRequestNote(projectID, mergeID, previousNote.ID, &gitlab.UpdateMergeRequestNoteOptions{
			Body: &message,
		})
		if err != nil {
			return fmt.Errorf("updtae comment for project id : %d and merge id : %d : failed %v", projectID, mergeID, err)
		}
	}

	return nil
}

// getUser is responsible for fetching the user info for given user id
func getUser(client *gitlab.Client, userID int) (*gitlab.User, error) {
	user, _, err := client.Users.GetUser(userID, gitlab.GetUsersOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetching user : %d failed : %v", userID, err)
	}
	return user, nil
}
