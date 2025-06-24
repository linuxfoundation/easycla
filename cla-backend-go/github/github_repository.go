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

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"

	"github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/google/go-github/v37/github"
)

var (
	// ErrGitHubRepositoryNotFound is returned when github repository is not found
	ErrGitHubRepositoryNotFound = errors.New("github repository not found")
)

const (
	help         = "https://help.github.com/en/github/committing-changes-to-your-project/why-are-my-commits-linked-to-the-wrong-user"
	unknown      = "Unknown"
	failureState = "failure"
	successState = "success"
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

	f := logrus.Fields{
		"functionName": "github.github_repository.getUserInfo",
		"tagUser":      tagUser,
	}

	userInfo := ""
	tagValue := ""
	var sb strings.Builder
	sb.WriteString(userInfo)

	log.WithFields(f).Debugf("author: %+v", u.CommitAuthor)

	if tagUser {
		tagValue = "@"
	}
	if u.CommitAuthor != nil {
		if *u.CommitAuthor.Login != "" {
			sb.WriteString(fmt.Sprintf("login: %s%s / ", tagValue, *u.CommitAuthor.Login))
		}

		if u.CommitAuthor.Name != nil {
			sb.WriteString(fmt.Sprintf("%sname: %s / ", userInfo, utils.StringValue(u.CommitAuthor.Name)))
		}
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

func UpdatePullRequest(ctx context.Context, installationID int64, pullRequestID int, owner, repo string, repoID *int64, latestSHA string, signed []*UserCommitSummary, missing []*UserCommitSummary, CLABaseAPIURL, CLALandingPage, CLALogoURL string) error {
	f := logrus.Fields{
		"functionName":   "github.github_repository.UpdatePullRequest",
		"installationID": installationID,
		"owner":          owner,
		"repo":           repo,
		"SHA":            latestSHA,
		"pullRequestID":  pullRequestID,
	}

	client, err := NewGithubAppClient(installationID)
	if err != nil || client == nil {
		log.WithFields(f).WithError(err).Warn("unable to create Github client")
		return err
	}

	// Update comments as necessary
	log.WithFields(f).Debugf("updating comment for PR: %d... ", pullRequestID)

	previouslyFailed, comment, failedErr := hasCheckPreviouslyFailed(ctx, client, owner, repo, pullRequestID)
	if failedErr != nil {
		log.WithFields(f).WithError(failedErr).Debugf("unable to check previously failed PR: %d", pullRequestID)
		return failedErr
	}

	previouslySucceeded, previousSucceededComment, succeedErr := hasCheckPreviouslySucceeded(ctx, client, owner, repo, pullRequestID)
	if succeedErr != nil {
		log.WithFields(f).WithError(succeedErr).Debugf("unable to check previously succeeded PR: %d", pullRequestID)
		return failedErr
	}

	body := assembleCLAComment(ctx, int(installationID), pullRequestID, repoID, signed, missing, CLABaseAPIURL, CLALogoURL, CLALandingPage)

	if len(missing) == 0 {
		// All contributors are passing

		// If we have previously failed, we need to update the comment
		if previouslyFailed {
			log.WithFields(f).Debugf("Found previously failed checks - updating the CLA comment in the PR : %d", pullRequestID)
			comment.Body = &body
			_, _, err = client.Issues.EditComment(ctx, owner, repo, *comment.ID, comment)
			if err != nil {
				log.WithFields(f).Debug("unable to edit comment ")
				return err
			}
		}
	} else {
		// One or more contributors are failing

		// If we have previously failed, we need to update the comment
		if previouslyFailed {
			log.WithFields(f).Debugf("Found previously failed checks - updating the CLA comment in the PR : %d", pullRequestID)
			comment.Body = &body
			_, _, err = client.Issues.EditComment(ctx, owner, repo, *comment.ID, comment)
			if err != nil {
				log.WithFields(f).Debug("unable to edit comment ")
				return err
			}
		} else if previouslySucceeded {
			// If we have previously succeeded, then we also need to update the comment (pass => fail)
			log.WithFields(f).Debugf("Found previously succeeeded checks - updating the CLA comment in the PR : %d", pullRequestID)
			// Generate a new comment with all the failed CLA info
			failedComment := assembleCLAComment(ctx, int(installationID), pullRequestID, repoID, signed, missing, CLABaseAPIURL, CLALogoURL, CLALandingPage)
			previousSucceededComment.Body = &failedComment
			_, _, err = client.Issues.EditComment(ctx, owner, repo, *previousSucceededComment.ID, previousSucceededComment)
			if err != nil {
				log.WithFields(f).Debug("unable to edit comment ")
				return err
			}
		} else {
			// no previous comment - need to create a new comment
			_, _, err = client.Issues.CreateComment(ctx, owner, repo, pullRequestID, comment)
			if err != nil {
				log.WithFields(f).Debug("unable to create comment")
			}

			log.WithFields(f).Debugf(`EasyCLA App checks fail for PR: %d.
			CLA signatures with signed authors: %+v and with missing authors: %+v`, pullRequestID, signed, missing)
		}
	}

	// Update/Create the status
	context := "EasyCLA"
	var statusBody string
	var state string
	var signURL string

	if len(missing) > 0 {
		state = failureState
		context, statusBody = assembleCLAStatus(context, false)
		signURL = getFullSignURL("github", strconv.Itoa(int(installationID)), strconv.Itoa(int(*repoID)), strconv.Itoa(pullRequestID), CLABaseAPIURL)
		log.WithFields(f).Debugf("Creating new CLA %s status - %d passed, %d missing, signing url %s", state, len(signed), len(missing), signURL)
	} else if len(signed) > 0 {
		state = successState
		context, statusBody = assembleCLAStatus(context, true)
		signURL = fmt.Sprintf("%s/#/?version=2", CLALandingPage)
		log.WithFields(f).Debugf("Creating new CLA %s status - %d passed, %d missing, signing url %s", state, len(signed), len(missing), signURL)

	} else {
		state = failureState
		context, statusBody = assembleCLAStatus(context, false)
		signURL = getFullSignURL("github", strconv.Itoa(int(installationID)), strconv.Itoa(int(*repoID)), strconv.Itoa(pullRequestID), CLABaseAPIURL)
		log.WithFields(f).Debugf("Creating new CLA %s status - %d passed, %d missing, signing url %s", state, len(signed), len(missing), signURL)
		log.WithFields(f).Debugf("This is an error condition - should have at least one committer in one of these lists: signed : %+v passed, %+v", signed, missing)
	}

	status := Status{
		State:       &state,
		TargetURL:   &signURL,
		Context:     &context,
		Description: &statusBody,
	}

	log.WithFields(f).Debugf("Creating status: %+v", status)

	_, _, err = CreateStatus(ctx, client, owner, repo, latestSHA, &status)
	if err != nil {
		log.WithFields(f).Debugf("unable to create status: %v", status)
		return err
	}

	return nil
}

func hasCheckPreviouslyFailed(ctx context.Context, client *github.Client, owner, repo string, pullRequestID int) (bool, *github.IssueComment, error) {
	f := logrus.Fields{
		"functionName": "github.github_repository.hasCheckPreviouslyFailed",
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

func hasCheckPreviouslySucceeded(ctx context.Context, client *github.Client, owner, repo string, pullRequestID int) (bool, *github.IssueComment, error) {
	f := logrus.Fields{
		"functionName": "github.github_repository.hasCheckPreviouslySucceeded",
	}

	comments, _, err := client.Issues.ListComments(ctx, owner, repo, pullRequestID, &github.IssueListCommentsOptions{})
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to get fetch comments for repo: %s, pr: %d", repo, pullRequestID)
		return false, nil, err
	}

	for _, comment := range comments {
		if strings.Contains(*comment.Body, "The committers listed above are authorized under a signed CLA.") {
			return true, comment, nil
		}
	}

	return false, nil, nil
}

func assembleCLAStatus(authorName string, signed bool) (string, string) {
	if authorName == "" {
		authorName = unknown
	}
	if signed {
		return authorName, "EasyCLA check passed. You are authorized to contribute."
	}
	return authorName, "Missing CLA Authorization."
}

func assembleCLAComment(ctx context.Context, installationID, pullRequestID int, repositoryID *int64, signed, missing []*UserCommitSummary, apiBaseURL, CLALogoURL, CLALandingPage string) string {
	f := logrus.Fields{
		"functionName":   "github.github_repository.assembleCLAComment",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"installationID": installationID,
		"repositoryID":   repositoryID,
		"pullRequestID":  pullRequestID,
		"repoID":         *repositoryID,
	}

	repositoryType := "github"
	missingID := false
	for _, userSummary := range missing {
		if userSummary.GetCommitAuthorID() == "" {
			missingID = true
		}
	}

	log.WithFields(f).Debug("Building CLAComment body ")
	signURL := getFullSignURL(repositoryType, strconv.Itoa(installationID), strconv.Itoa(int(*repositoryID)), strconv.Itoa(pullRequestID), apiBaseURL)
	commentBody := getCommentBody(repositoryType, signURL, signed, missing)
	allSigned := len(missing) == 0
	badge := getCommentBadge(allSigned, signURL, missingID, false, CLALandingPage, CLALogoURL)
	return fmt.Sprintf("%s<br >%s", badge, commentBody)
}

func getCommentBody(repositoryType, signURL string, signed, missing []*UserCommitSummary) string {
	f := logrus.Fields{
		"functionName":   "github.github_repository:getCommentBody",
		"repositoryType": repositoryType,
		"signURL":        signURL,
	}

	failed := ":x:"
	success := ":white_check_mark:"
	committersComment := strings.Builder{}
	text := ""

	if len(missing) > 0 || len(signed) > 0 {
		committersComment.WriteString("<ul>")
	}

	if len(signed) > 0 {
		committers := getAuthorInfoCommits(signed, false)

		for k, v := range committers {
			var shas []string
			for _, summary := range v {
				shas = append(shas, summary.SHA)
				log.WithFields(f).Debugf("SHAS for signed users: %s", shas)
				committersComment.WriteString(fmt.Sprintf("<li>%s%s(%s)</li>", success, k, strings.Join(shas, ", ")))
			}
		}
	}

	if len(missing) > 0 {
		log.WithFields(f).Debugf("processing %d missing contributors", len(missing))
		supportURL := "https://jira.linuxfoundation.org/servicedesk/customer/portal/4"
		committers := getAuthorInfoCommits(missing, true)
		helpURL := help

		for k, v := range committers {
			var shas []string
			for _, summary := range v {
				shas = append(shas, summary.SHA)
			}
			if k == unknown {
				committersComment.WriteString(fmt.Sprintf(`<li>%s The commit (%s). This user is missing the User's ID, preventing the EasyCLA check. <a href='%s' target='_blank'>Consult GitHub Help</a> to resolve. For further assistance with EasyCLA, <a href='%s' target='_blank'>please submit a support request ticket</a>.</li>`,
					failed, strings.Join(shas, ", "), helpURL, supportURL))
			} else {
				var missingAffiliations []*UserCommitSummary
				for _, summary := range v {
					if !summary.Affiliated && !summary.Authorized {
						missingAffiliations = append(missingAffiliations, summary)
					}
				}
				if len(missingAffiliations) > 0 {
					log.WithFields(f).Debugf("SHAs for users with missing company affiliations: %+v", shas)
					committersComment.WriteString(
						fmt.Sprintf(`<li>%s %s The commit (%s). This user is authorized, but they must confirm their affiliation with their company. Start the authorization process <a href='%s' target='_blank'> by clicking here</a>, click \"Corporate\", select the appropriate company from the list, then confirm your affiliation on the page that appears. For further assistance with EasyCLA, <a href='%s' target='_blank'>please submit a support request ticket</a>.</li>`,
							failed, k, strings.Join(shas, ", "), signURL, supportURL))
				} else {
					committersComment.WriteString(
						fmt.Sprintf(`<li><a href='%s' target='_blank'>%s</a> - %s The commit (%s) is not authorized under a signed CLA. "<a href='%s' target='_blank'>Please click here to be authorized</a>. For further assistance with EasyCLA, <a href='%s' target='_blank'>please submit a support request ticket</a>.</li>`,
							signURL, failed, k, strings.Join(shas, ", "), signURL, supportURL))
				}
			}
		}
	}

	if len(signed) > 0 || len(missing) > 0 {
		committersComment.WriteString("</ul>")
	}

	if len(signed) > 0 && len(missing) == 0 {
		text = "<br>The committers listed above are authorized under a signed CLA."
	}

	return fmt.Sprintf("%s%s", committersComment.String(), text)
}

func getCommentBadge(allSigned bool, signURL string, missingUserId, managerApproved bool, CLALandingPage, CLALogoURL string) string {
	var alt string
	var text string
	var badgeHyperLink string
	var badgeURL string

	if allSigned {
		badgeURL = fmt.Sprintf("%s/cla-signed.svg", CLALogoURL)
		badgeHyperLink = fmt.Sprintf("%s/#/?version=2", CLALandingPage)
		alt = "CLA Signed"
		return fmt.Sprintf(`<a href="%s"><img src="%s" alt="%s" align="left" height="28" width="328" >`, badgeHyperLink, badgeURL, alt)
	}
	badgeHyperLink = signURL
	if missingUserId {
		badgeURL = fmt.Sprintf("%s/cla-missing-id.svg", CLALogoURL)
		alt = "CLA Missing ID"
	} else if managerApproved {
		badgeURL = fmt.Sprintf("%s/cla-confirmation-needed.svg", CLALogoURL)
		alt = "CLA Confirmation Needed"
	} else {
		badgeURL = fmt.Sprintf("%s/cla-not-signed.svg", CLALogoURL)
		alt = "CLA Not Signed"
	}

	text = fmt.Sprintf(`<a href="%s"><img src="%s" alt="%s" align="left" height="28" width="328" >`, badgeHyperLink, badgeURL, alt)
	return fmt.Sprintf("%s<br/>", text)
}

func getFullSignURL(repositoryType, installationID, githubRepositoryID, pullRequestID, apiBaseURL string) string {
	return fmt.Sprintf("%s/v2/repository-provider/%s/sign/%s/%s/%s/#/?version=2", apiBaseURL, repositoryType, installationID, githubRepositoryID, pullRequestID)
}

func getAuthorInfoCommits(userSummary []*UserCommitSummary, tagUser bool) map[string][]*UserCommitSummary {
	f := logrus.Fields{
		"functioName": "github.github_repository.getAuthorInfoCommits",
	}
	result := make(map[string][]*UserCommitSummary)
	for _, author := range userSummary {
		log.WithFields(f).WithFields(f).Debugf("checking user summary for : %s", author.getUserInfo(tagUser))
		if _, ok := result[author.getUserInfo(tagUser)]; !ok {

			result[author.getUserInfo(tagUser)] = []*UserCommitSummary{
				author,
			}
		} else {
			result[author.getUserInfo(tagUser)] = append(result[author.getUserInfo(tagUser)], author)
		}
	}
	return result
}

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
			log.WithFields(f).Warnf("%s", msg)
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

type Status struct {
	State       *string `json:"state,omitempty"`
	TargetURL   *string `json:"target_url,omitempty"`
	Description *string `json:"description,omitempty"`
	Context     *string `json:"context,omitempty"`
}

// CreateStatus creates a new status on the specified commit.
//
// GitHub API docs:https://docs.github.com/en/rest/commits/statuses
func CreateStatus(ctx context.Context, client *github.Client, owner, repo, sha string, status *Status) (*Status, *github.Response, error) {
	u := fmt.Sprintf("repos/%v/%v/statuses/%v", owner, repo, sha)
	req, err := client.NewRequest("POST", u, status)
	if err != nil {
		return nil, nil, err
	}
	c := new(Status)
	resp, err := client.Do(ctx, req, c)
	if err != nil {
		return nil, resp, err
	}

	return c, resp, nil
}

func GetReturnURL(ctx context.Context, installationID, repositoryID int64, pullRequestID int) (string, error) {
	f := logrus.Fields{
		"functionName":   "github.github_repository.GetReturnURL",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"installationID": installationID,
		"repositoryID":   repositoryID,
		"pullRequestID":  pullRequestID,
	}

	client, err := NewGithubAppClient(installationID)

	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to create Github client")
		return "", err
	}

	log.WithFields(f).Debugf("getting github repository by id: %d", repositoryID)
	repo, _, err := client.Repositories.GetByID(ctx, repositoryID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to get repository by ID: %d", repositoryID)
		return "", err
	}

	log.WithFields(f).Debugf("getting pull request by id: %d", pullRequestID)
	pullRequest, _, err := client.PullRequests.Get(ctx, *repo.Owner.Login, *repo.Name, pullRequestID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to get pull request by ID: %d", pullRequestID)
		return "", err
	}

	log.WithFields(f).Debugf("returning pull request html url: %s", *pullRequest.HTMLURL)

	return *pullRequest.HTMLURL, nil
}
