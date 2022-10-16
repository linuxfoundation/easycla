// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/google/go-github/v37/github"
)

var (
	// ErrGitHubRepositoryNotFound is returned when github repository is not found
	ErrGitHubRepositoryNotFound = errors.New("github repository not found")
)

func GetGitHubRepository(ctx context.Context, installationID, githubRepositoryID int64) (*github.Repository, error) {
	f := logrus.Fields{
		"functionName":       "github.github_repository.GetGitHubRepository",
		"installationID":     installationID,
		"githubRepositoryID": githubRepositoryID,
	}
	client, clientErr := NewGithubAppClient(installationID)
	if clientErr != nil {
		log.WithFields(f).WithError(clientErr).Warnf("problem loading github client for installation ID: %d", installationID)
		return nil, clientErr
	}

	log.WithFields(f).Debugf("getting github repository by id: %d", githubRepositoryID)
	repository, httpResponse, repoErr := client.Repositories.GetByID(ctx, githubRepositoryID)
	if repoErr != nil {
		log.WithFields(f).WithError(repoErr).Warnf("unable to fetch repository by ID: %d", githubRepositoryID)
		return nil, repoErr
	}
	if httpResponse.StatusCode != http.StatusOK {
		log.WithFields(f).Warnf("unexpected status code: %d", httpResponse.StatusCode)
		return nil, ErrGitHubRepositoryNotFound
	}

	//log.WithFields(f).Debugf("successfully retrieved github repository by id: %d - repository object: %+v", githubRepositoryID, repository)
	return repository, nil
}

func GetPullRequest(ctx context.Context, pullRequestID int, owner, repo string, client *github.Client) (*github.PullRequest, error) {
	f := logrus.Fields{
		"functionName":  "github.github_repository.GetPullRequest",
		"pullRequestID": pullRequestID,
		"owner":         owner,
		"repo":          repo,
	}

	pullRequest, _, err := client.PullRequests.Get(ctx, owner, repo, pullRequestID)
	if err != nil {
		logging.WithFields(f).WithError(err).Warn("unable to get pull request")
		return nil, err
	}

	return pullRequest, nil
}

// UserCommitSummary data model
type UserCommitSummary struct {
	SHA          string
	CommitAuthor *github.User
	Affiliated   bool
	Authorized   bool
}

// GetCommitAuthorID commit author username ID (numeric value as a string) if available, otherwise returns empty string
func (u UserCommitSummary) GetCommitAuthorID() string {
	if u.CommitAuthor != nil && u.CommitAuthor.ID != nil {
		return strconv.Itoa(int(*u.CommitAuthor.ID))
	}

	return ""
}

// GetCommitAuthorUsername returns commit author username if available, otherwise returns empty string
func (u UserCommitSummary) GetCommitAuthorUsername() string {
	if u.CommitAuthor != nil {
		if u.CommitAuthor.Login != nil {
			return *u.CommitAuthor.Login
		}
		if u.CommitAuthor.Name != nil {
			return *u.CommitAuthor.Name
		}
	}

	return ""
}

// GetCommitAuthorEmail returns commit author email if available, otherwise returns empty string
func (u UserCommitSummary) GetCommitAuthorEmail() string {
	if u.CommitAuthor != nil && u.CommitAuthor.Email != nil {
		return *u.CommitAuthor.Email
	}

	return ""
}

// IsValid returns true if the commit author information is available
func (u UserCommitSummary) IsValid() bool {
	valid := false
	if u.CommitAuthor != nil {
		valid = u.CommitAuthor.ID != nil && (u.CommitAuthor.Login != nil || u.CommitAuthor.Name != nil)
	}
	return valid
}

// GetDisplayText returns the display text for the user commit summary
func (u UserCommitSummary) GetDisplayText(tagUser bool) string {
	if !u.IsValid() {
		return "Invalid author details.\n"
	}
	if u.Affiliated && u.Authorized {
		return fmt.Sprintf("%s is authorized.\n ", u.getUserInfo(tagUser))
	}
	if u.Affiliated {
		return fmt.Sprintf("%s is associated with a company, but not an approval list.\n", u.getUserInfo(tagUser))
	} else {
		return fmt.Sprintf("%s is not associated with a company.\n", u.getUserInfo(tagUser))
	}
}

func (u UserCommitSummary) getUserInfo(tagUser bool) string {
	userInfo := ""
	tagValue := ""
	var sb strings.Builder
	sb.WriteString(userInfo)

	if tagUser {
		tagValue = "@"
	}
	if *u.CommitAuthor.Login != "" {
		sb.WriteString(fmt.Sprintf("login: %s%s / ", tagValue, u.CommitAuthor))
	}

	if u.CommitAuthor.Name != nil {
		sb.WriteString(fmt.Sprintf("%sname: %s / ", userInfo, utils.StringValue(u.CommitAuthor.Name)))
	}
	return strings.Replace(sb.String(), "/ $", "", -1)
}

func GetPullRequestCommitAuthors(ctx context.Context, installationID int64, pullRequestID int, owner, repo string) ([]*UserCommitSummary, *string, error) {
	f := logrus.Fields{
		"functionName":  "github.github_repository.GetPullRequestCommitAuthors",
		"pullRequestID": pullRequestID,
	}
	var userCommitSummary []*UserCommitSummary

	client, err := NewGithubAppClient(installationID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to create Github client")
		return nil, nil, err
	}

	commits, resp, comErr := client.PullRequests.ListCommits(ctx, owner, repo, pullRequestID, &github.ListOptions{})
	if comErr != nil {
		log.WithFields(f).WithError(comErr).Warnf("problem listing commits for repo: %s/%s pull request: %d", owner, repo, pullRequestID)
		return nil, nil, comErr
	}
	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("unexpected status code: %d - expected: %d", resp.StatusCode, http.StatusOK)
		log.WithFields(f).Warn(msg)
		return nil, nil, errors.New(msg)
	}

	log.WithFields(f).Debugf("found %d commits for pull request: %d", len(commits), pullRequestID)
	for _, commit := range commits {
		log.WithFields(f).Debugf("loaded commit: %+v", commit)
		commitAuthor := ""
		if commit.Commit != nil && commit.Commit.Author != nil && commit.Commit.Author.Login != nil {
			log.WithFields(f).Debugf("commit.Commit.Author: %s", utils.StringValue(commit.Commit.Author.Login))
			commitAuthor = utils.StringValue(commit.Commit.Author.Login)
		} else if commit.Author != nil && commit.Author.Login != nil {
			log.WithFields(f).Debugf("commit.Author.Login: %s", utils.StringValue(commit.Author.Login))
			commitAuthor = utils.StringValue(commit.Author.Login)
		}
		log.WithFields(f).Debugf("commitAuthor: %s", commitAuthor)
		userCommitSummary = append(userCommitSummary, &UserCommitSummary{
			SHA:          *commit.SHA,
			CommitAuthor: commit.Author,
			Affiliated:   false,
			Authorized:   false,
		})
	}

	// get latest commit SHA
	latestCommitSHA := commits[len(commits)-1].SHA
	return userCommitSummary, latestCommitSHA, nil
}

func UpdatePullRequest(ctx context.Context, installationID int64, pullRequestID int, owner, repo, latestSHA string, signed []*UserCommitSummary, missing []*UserCommitSummary, CLABaseAPIURL, CLALandingPage string) error {
	f := logrus.Fields{
		"functionName":   "github.github_repository.UpdatePullRequest",
		"installationID": installationID,
		"owner":          owner,
		"repo":           repo,
		"SHA":            latestSHA,
		"pullRequestID":  pullRequestID,
	}

	client, err := NewGithubAppClient(installationID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to create Github client")
		return err
	}

	if len(missing) > 0 {
		helpURL := ""
		text := ""
		var sb strings.Builder
		sb.WriteString(text)
		for _, userSummary := range missing {
			if !userSummary.IsValid() {
				helpURL = "https://help.github.com/en/github/committing-changes-to-your-project/why-are-my-commits-linked-to-the-wrong-user"
			}
			if userSummary.SHA != latestSHA {
				continue
			}
			sb.WriteString(userSummary.GetDisplayText(true))
		}
		text = sb.String()
		status := "completed"
		conclusion := "action_required"
		title := "EasyCLA: Signed CLA not found"
		summary := "One or more committers are authorized under a signed CLA"
		checkRunOptions := github.CreateCheckRunOptions{
			Name:       "CLA check",
			HeadSHA:    latestSHA,
			Status:     &status,
			Conclusion: &conclusion,
			DetailsURL: &helpURL,
			Output: &github.CheckRunOutput{
				Title:   &title,
				Summary: &summary,
				Text:    &text,
			},
		}

		checkRun, checkRunResponse, checkRunErr := client.Checks.CreateCheckRun(ctx, owner, repo, checkRunOptions)
		if checkRunErr != nil {
			log.WithFields(f).WithError(checkRunErr).Warnf("problem creating check run")
			return checkRunErr
		}
		if checkRunResponse == nil || checkRunResponse.StatusCode != http.StatusCreated {
			var statusCode int
			if checkRunResponse != nil {
				statusCode = checkRunResponse.StatusCode
			}
			msg := fmt.Sprintf("problem creating check run - status %d - expecting %d", statusCode, http.StatusCreated)
			log.WithFields(f).WithError(checkRunErr).Warn(msg)
			return errors.New(msg)
		}

		if checkRun != nil {
			log.WithFields(f).Debugf("created check run - ID: %d with name: %s",
				utils.Int64Value(checkRun.ID), utils.StringValue(checkRun.Name))
		} else {
			log.WithFields(f).Debugf("created check run - but not check run details returned from API call")
		}

	}

	return nil
}

/*
func hasCheckPreviouslyFailed(ctx context.Context, client *github.Client, owner, repo string, pullRequestID int) (bool, *github.IssueComment, error) {
	f := logrus.Fields{
		"functionName": "github.github_repository.GetPullRequest",
	}

	comments, _, err := client.Issues.ListComments(ctx, owner, repo, pullRequestID, &github.IssueListCommentsOptions{})
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to get fetch comments for repo: %s, pr: %d", repo, pullRequestID)
		return false, nil, err
	}

	for _, comment := range comments {
		if strings.Contains(*comment.Body, "is not authorized under a signed CLA") {
			return true, comment, nil
		}
		if strings.Contains(*comment.Body, "they must confirm their affiliation") {
			return true, comment, nil
		}
		if strings.Contains(*comment.Body, "is missing the User") {
			return true, comment, nil
		}
	}
	return false, nil, nil
}
*/

// GetRepositoryByExternalID finds github repository by github repository id
func GetRepositoryByExternalID(ctx context.Context, installationID, id int64) (*github.Repository, error) {
	client, err := NewGithubAppClient(installationID)
	if err != nil {
		return nil, err
	}
	org, resp, err := client.Repositories.GetByID(ctx, id)
	if err != nil {
		logging.Warnf("GitHubGetRepository %v failed. error = %s", id, err.Error())
		if resp.StatusCode == 404 {
			return nil, ErrGitHubRepositoryNotFound
		}
		return nil, err
	}
	return org, nil
}

// GetRepositories gets github repositories by organization
func GetRepositories(ctx context.Context, organizationName string) ([]*github.Repository, error) {
	f := logrus.Fields{
		"functionName":     "GetRepositories",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"organizationName": organizationName,
	}

	// Get the client with token
	client := NewGithubOauthClient()

	var responseRepoList []*github.Repository
	var nextPage = 1
	for {
		// API https://docs.github.com/en/free-pro-team@latest/rest/reference/repos
		// API Pagination: https://docs.github.com/en/free-pro-team@latest/rest/guides/traversing-with-pagination
		repoList, resp, err := client.Repositories.ListByOrg(ctx, organizationName, &github.RepositoryListByOrgOptions{
			Type:      "public",
			Sort:      "full_name",
			Direction: "asc",
			ListOptions: github.ListOptions{
				Page:    nextPage,
				PerPage: 100,
			},
		})
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to list repositories for organization")
			if resp != nil && resp.StatusCode == 404 {
				return nil, ErrGithubOrganizationNotFound
			}
			return nil, err
		}

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			msg := fmt.Sprintf("GetRepositories %s failed with no success response code %d. error = %s", organizationName, resp.StatusCode, err.Error())
			log.WithFields(f).Warnf(msg)
			return nil, errors.New(msg)
		}

		// Append our results to the response...
		responseRepoList = append(responseRepoList, repoList...)
		// if no more pages...
		if resp.NextPage == 0 {
			break
		}

		// update our next page value
		nextPage = resp.NextPage
	}

	return responseRepoList, nil
}
