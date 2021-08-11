// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"fmt"
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
func FetchMrParticipants(client *gitlab.Client, projectID int, mergeID int, unique bool) ([]*gitlab.User, error) {
	commits, _, err := client.MergeRequests.GetMergeRequestCommits(projectID, mergeID, &gitlab.GetMergeRequestCommitsOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetching gitlab participants for project : %d and merge id : %d, failed : %v", projectID, mergeID, err)
	}

	if len(commits) == 0 {
		return nil, nil
	}

	var results []*gitlab.User
	uniqueUsers := map[int]bool{}

	for _, commit := range commits {
		authorEmail := commit.AuthorEmail
		authorName := commit.AuthorName

		log.Debugf("user email found : %s, user name : %s, searching in gitlab ...", authorEmail, authorName)

		var user *gitlab.User
		if authorName != "" {
			user, err = searchForUser(client, authorEmail)
			if err != nil {
				return nil, fmt.Errorf("searching for author email : %s, failed : %v", authorEmail, err)
			}
		}

		if authorName != "" && user == nil {
			user, err = searchForUser(client, authorName)
			if err != nil {
				return nil, fmt.Errorf("searching for author name : %s, failed : %v", authorName, err)
			}
		}

		if user == nil {
			return nil, fmt.Errorf("no users found for commit author email : %s, name : %s", authorEmail, authorName)
		}

		if uniqueUsers[user.ID] {
			continue
		}

		results = append(results, user)
		uniqueUsers[user.ID] = true
	}

	return results, nil
}

// SetCommitStatus is responsible for setting the MR status for commit sha
func SetCommitStatus(client *gitlab.Client, projectID int, commitSha string, state gitlab.BuildStateValue, message string) error {
	options := &gitlab.SetCommitStatusOptions{
		State:       state,
		Name:        gitlab.String("easyCLA Bot"),
		Description: gitlab.String(message),
	}

	if state == gitlab.Failed {
		options.TargetURL = gitlab.String("http://localhost:8080/gitlab/sign")
	}

	_, _, err := client.Commits.SetCommitStatus(projectID, commitSha, options)
	if err != nil {
		return fmt.Errorf("setting commit status for the sha : %s and project id : %d failed : %v", commitSha, projectID, err)
	}

	return nil
}

// SetMrComment is responsible for setting the comment body for project and merge id
func SetMrComment(client *gitlab.Client, projectID int, mergeID int, state gitlab.BuildStateValue, message string) error {
	covered := `<a href="http://localhost:8080">
	<img src="https://s3.amazonaws.com/cla-project-logo-dev/cla-signed.svg" alt="covered" align="left" height="28" width="328" ></a><br/>`
	failed := `<a href="http://localhost:8080">
<img src="https://s3.amazonaws.com/cla-project-logo-dev/cla-not-signed.svg" alt="covered" align="left" height="28" width="328" ></a><br/>`

	var body string
	if state == gitlab.Failed {
		body = failed
	} else {
		body = covered
	}

	notes, _, err := client.Notes.ListMergeRequestNotes(projectID, mergeID, &gitlab.ListMergeRequestNotesOptions{})
	if err != nil {
		return fmt.Errorf("fetching comments for project id : %d and merge id : %d : failed %v", projectID, mergeID, err)
	}

	var previousNote *gitlab.Note
	if len(notes) > 0 {
		for _, n := range notes {
			if strings.Contains(n.Body, "cla-signed.svg") || strings.Contains(n.Body, "cla-not-signed.svg") {
				previousNote = n
				break
			}
		}
	}

	if previousNote == nil {
		log.Debugf("no previous comments found for project id : %d and merge id : %d", projectID, mergeID)
		_, _, err = client.Notes.CreateMergeRequestNote(projectID, mergeID, &gitlab.CreateMergeRequestNoteOptions{
			Body: &body,
		})
		if err != nil {
			return fmt.Errorf("creating comment for project id : %d and merge id : %d : failed %v", projectID, mergeID, err)
		}
	} else {
		log.Debugf("previous comments found for project id : %d and merge id : %d", projectID, mergeID)
		_, _, err = client.Notes.UpdateMergeRequestNote(projectID, mergeID, previousNote.ID, &gitlab.UpdateMergeRequestNoteOptions{
			Body: &body,
		})
		if err != nil {
			return fmt.Errorf("updtae comment for project id : %d and merge id : %d : failed %v", projectID, mergeID, err)
		}
	}

	return nil
}

func searchForUser(client *gitlab.Client, search string) (*gitlab.User, error) {
	users, _, err := client.Users.ListUsers(&gitlab.ListUsersOptions{
		Search: gitlab.String(search),
	})

	if err != nil {
		return nil, fmt.Errorf("searching for user string : %s failed : %v", search, err)
	}

	if len(users) == 0 {
		return nil, nil
	}

	if len(users) > 1 {
		return nil, fmt.Errorf("found more than one gitlab user for search string : %s", search)
	}

	return users[0], nil
}
