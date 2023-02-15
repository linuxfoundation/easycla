// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"fmt"
	"strings"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
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
	f := logrus.Fields{
		"functionName": "gitlab_api.FetchMrParticipants",
		"projectID":    projectID,
		"mergeID":      mergeID,
	}
	log.WithFields(f).Debug("fetching mr participants...")
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

		// check if user already exists in the results
		user, err := getUser(client, &authorEmail, &authorName)

		if err != nil && user == nil {
			log.WithFields(f).Warnf("unable to find user for commit author email : %s, name : %s, error : %v", authorEmail, authorName, err)
			return nil, err
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

// getUser is responsible for fetching the user info for given user email
func getUser(client *gitlab.Client, email, name *string) (*gitlab.User, error) {
	f := logrus.Fields{
		"functionName": "gitlab_api.getUser",
		"email":        email,
	}

	users, _, err := client.Users.ListUsers(&gitlab.ListUsersOptions{
		Search: gitlab.String(*name),
	})

	log.WithFields(f).Debugf("found %d users for name : %s", len(users), *name)

	if err != nil {
		log.WithFields(f).Warnf("unable to find user for name : %s, error : %v", *name, err)
		return nil, nil
	}

	if len(users) == 0 {
		log.WithFields(f).Warnf("no user found for name : %s", *name)
		return &gitlab.User{
			Email: *email,
			Name:  *name,
		}, nil
	}

	// check if user exists for the given email
	for _, user := range users {
		log.WithFields(f).Debugf("checking user : %s for email : %s", user.Email, user.Email)
		if user.Email == *email {
			return user, nil
		}
	}
	return &gitlab.User{
		Email: *email,
		Name:  *name,
	}, fmt.Errorf("unable to find user for email : %s", *email)

}
