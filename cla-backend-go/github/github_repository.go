// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"

	"github.com/google/go-github/v37/github"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

var (
	// ErrGitHubRepositoryNotFound is returned when github repository is not found
	ErrGitHubRepositoryNotFound = errors.New("github repository not found")
	NoreplyIDPattern            = regexp.MustCompile(`^(\d+)\+([a-zA-Z0-9-]+)@users\.noreply\.github\.com$`)
	NoreplyUserPattern          = regexp.MustCompile(`^([a-zA-Z0-9-]+)@users\.noreply\.github\.com$`)
	GithubUsernameRegex         = regexp.MustCompile(`^[A-Za-z0-9-]{3,39}$`)
)

// Note: we use | and ||| as placeholders for inline and fenced code, then swap to backticks at render time.
const MissingCoAuthorsMessage = `

One or more co-authors of this pull request were not found. You must specify co-authors in commit message trailer via:

|||
Co-authored-by: name <email>
|||

Supported |Co-authored-by:| formats include:

1) |Anything <id+login@users.noreply.github.com>| - it will locate your GitHub user by |id| part.
2) |Anything <login@users.noreply.github.com>| - it will locate your GitHub user by |login| part.
3) |Anything <public-email>| - it will locate your GitHub user by |public-email| part. Note that this email must be made public on Github.
4) |Anything <other-email>| - it will locate your GitHub user by |other-email| part but only if that email was used before for any other CLA as a main commit author.
5) |login <any-valid-email>| - it will locate your GitHub user by |login| part, note that |login| part must be at least 3 characters long.

Please update your commit message(s) by doing |git commit --amend| and then |git push [--force]| and then request re-running CLA check via commenting on this pull request:

|||
/easycla
|||

`

const (
	help         = "https://help.github.com/en/github/committing-changes-to-your-project/why-are-my-commits-linked-to-the-wrong-user"
	unknown      = "Unknown"
	failureState = "failure"
	successState = "success"
	svgVersion   = "?v=2"
)

type cacheEntry struct {
	value     *github.User
	expiresAt time.Time
}

type Cache struct {
	data map[[2]string]cacheEntry
	mu   sync.Mutex
	ttl  time.Duration
}

func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		data: make(map[[2]string]cacheEntry),
		ttl:  ttl,
	}
}

func (c *Cache) Get(key [2]string) (*github.User, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, found := c.data[key]
	if !found || time.Now().After(entry.expiresAt) {
		if found {
			delete(c.data, key)
		}
		return nil, false
	}
	return entry.value, true
}

func (c *Cache) Set(key [2]string, value *github.User) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *Cache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.data {
		if now.After(v.expiresAt) {
			delete(c.data, k)
		}
	}
}

func (c *Cache) Delete(key [2]string) { c.mu.Lock(); delete(c.data, key); c.mu.Unlock() }

type userCacheEntry struct {
	value     *models.User
	expiresAt time.Time
}

type UserCache struct {
	data map[[3]string]userCacheEntry
	mu   sync.Mutex
	ttl  time.Duration
}

func NewUserCache(ttl time.Duration) *UserCache {
	return &UserCache{
		data: make(map[[3]string]userCacheEntry),
		ttl:  ttl,
	}
}

func (c *UserCache) Get(key [3]string) (*models.User, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, found := c.data[key]
	if !found || time.Now().After(entry.expiresAt) {
		if found {
			delete(c.data, key)
		}
		return nil, false
	}
	return entry.value, true
}

func (c *UserCache) Set(key [3]string, value *models.User) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = userCacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *UserCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.data {
		if now.After(v.expiresAt) {
			delete(c.data, k)
		}
	}
}

func (c *UserCache) Delete(key [3]string) { c.mu.Lock(); delete(c.data, key); c.mu.Unlock() }

type projectUserCacheEntry struct {
	value      *models.User
	signed     bool
	affiliated bool
	expiresAt  time.Time
}

type ProjectUserCache struct {
	data map[[4]string]projectUserCacheEntry
	mu   sync.Mutex
	ttl  time.Duration
}

func NewProjectUserCache(ttl time.Duration) *ProjectUserCache {
	return &ProjectUserCache{
		data: make(map[[4]string]projectUserCacheEntry),
		ttl:  ttl,
	}
}

func (c *ProjectUserCache) Get(key [4]string) (*models.User, bool, bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, found := c.data[key]
	if !found || time.Now().After(entry.expiresAt) {
		if found {
			delete(c.data, key)
		}
		return nil, false, false, false
	}
	return entry.value, entry.signed, entry.affiliated, true
}

func (c *ProjectUserCache) Set(key [4]string, value *models.User, signed, affiliated bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = projectUserCacheEntry{
		value:      value,
		signed:     signed,
		affiliated: affiliated,
		expiresAt:  time.Now().Add(c.ttl),
	}
}

func (c *ProjectUserCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.data {
		if now.After(v.expiresAt) {
			delete(c.data, k)
		}
	}
}

func (c *ProjectUserCache) Delete(key [4]string) { c.mu.Lock(); delete(c.data, key); c.mu.Unlock() }

var GithubUserCache = NewCache(24 * time.Hour)
var ModelUserCache = NewUserCache(24 * time.Hour)
var ModelProjectUserCache = NewProjectUserCache(24 * time.Hour)

func init() {
	go func() {
		for {
			time.Sleep(time.Hour)
			GithubUserCache.Cleanup()
			ModelUserCache.Cleanup()
			ModelProjectUserCache.Cleanup()
		}
	}()
}

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
		if u.CommitAuthor.Login != nil && *u.CommitAuthor.Login != "" {
			sb.WriteString(fmt.Sprintf("login: %s%s / ", tagValue, *u.CommitAuthor.Login))
		}

		if u.CommitAuthor.Name != nil {
			sb.WriteString(fmt.Sprintf("%sname: %s / ", userInfo, utils.StringValue(u.CommitAuthor.Name)))
		}
	}

	return strings.Replace(sb.String(), "/ $", "", -1)
}

// SearchGithubUserByEmail searches for a GitHub user by email using the GitHub search API.
// Returns the first found *github.User, or nil if not found or on error.
func SearchGithubUserByEmail(ctx context.Context, client *github.Client, email string) (*github.User, error) {
	f := logrus.Fields{
		"functionName": "github.github_repository.SearchGithubUserByEmail",
		"email":        email,
	}
	log.WithFields(f).Debugf("Searching for GitHub user by email: %s", email)

	query := fmt.Sprintf("%s in:email", email)
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	}
	result, _, err := client.Search.Users(ctx, query, opts)
	if err != nil {
		log.WithFields(f).WithError(err).Errorf("Error searching for user by email: %s", email)
		return nil, err
	}
	if result.GetTotal() == 0 || len(result.Users) == 0 {
		log.WithFields(f).Debugf("No GitHub user found with email: %s", email)
		return nil, nil
	}
	log.WithFields(f).Debugf("Found GitHub user by email: %s", *result.Users[0].Login)
	return result.Users[0], nil
}

// GetGitHubUserByLogin fetches a GitHub user by their login (username).
// Returns (*github.User, nil) if found, (nil, nil) if not found, or (nil, error) on error.
func GetGithubUserByLogin(ctx context.Context, client *github.Client, login string) (*github.User, error) {
	f := logrus.Fields{
		"functionName": "github.github_repository.GetGitHubUserByLogin",
		"login":        login,
	}
	log.WithFields(f).Debugf("Getting GitHub user by login: %s", login)
	user, _, err := client.Users.Get(ctx, login)
	if err != nil {
		if ghErr, ok := err.(*github.ErrorResponse); ok && ghErr.Response.StatusCode == 404 {
			log.WithFields(f).Debugf("Could not find GitHub user with login: %s", login)
			return nil, nil
		}
		log.WithFields(f).WithError(err).Errorf("Error getting GitHub user with login: %s", login)
		return nil, err
	}
	if user == nil {
		log.WithFields(f).Debugf("No user object returned for login: %s", login)
		return nil, nil
	}
	log.WithFields(f).Debugf("Found GitHub user by login: %s", login)
	return user, nil
}

// GetGitHubUserByID fetches a GitHub user by their GitHubID.
// Returns (*github.User, nil) if found, (nil, nil) if not found, or (nil, error) on error.
func GetGithubUserByID(ctx context.Context, client *github.Client, githubID int64) (*github.User, error) {
	f := logrus.Fields{
		"functionName": "github.github_repository.GetGitHubUserByID",
		"githubID":     githubID,
	}
	log.WithFields(f).Debugf("Getting GitHub user by GitHub ID: %d", githubID)
	user, _, err := client.Users.GetByID(ctx, githubID)
	if err != nil {
		if ghErr, ok := err.(*github.ErrorResponse); ok && ghErr.Response.StatusCode == 404 {
			log.WithFields(f).Debugf("Could not find GitHub user with GitHub ID: %d", githubID)
			return nil, nil
		}
		log.WithFields(f).WithError(err).Errorf("Error getting GitHub user with GitHub ID: %d", githubID)
		return nil, err
	}
	if user == nil {
		log.WithFields(f).Debugf("No user object returned for GitHub ID: %d", githubID)
		return nil, nil
	}
	log.WithFields(f).Debugf("Found GitHub user by GitHub ID: %d", githubID)
	return user, nil
}

// GetCoAuthorsFromCommit returns a slice of [2]string, each representing [name, email] of a co-author.
func GetCoAuthorsFromCommit(
	ctx context.Context,
	commit *github.RepositoryCommit,
) [][2]string {
	f := logrus.Fields{
		"functionName": "github.github_repository.GetCoAuthorsFromCommit",
	}
	var coAuthors [][2]string
	if commit != nil && commit.Commit != nil && commit.Commit.Message != nil {
		commitMessage := commit.GetCommit().GetMessage()
		// log.WithFields(f).Debugf("commit message: %s", commitMessage)

		re := regexp.MustCompile(`(?i)co-authored-by:\s*(.+?)\s*<([^<>]+)>`)
		matches := re.FindAllStringSubmatch(commitMessage, -1)
		for _, match := range matches {
			name := strings.TrimSpace(match[1])
			email := strings.ToLower(strings.TrimSpace(match[2]))
			if name != "" && email != "" {
				coAuthors = append(coAuthors, [2]string{name, email})
				log.WithFields(f).Debugf("found co-author: name: %s, email: %s", name, email)
			}
		}
	}
	return coAuthors
}

// ExpandWithCoAuthors appends UserCommitSummary objects for all co-authors to commitAuthors slice.
func ExpandWithCoAuthors(
	ctx context.Context,
	client *github.Client,
	usersService users.Service,
	commit *github.RepositoryCommit,
	pr int,
	installationID int64,
	commitAuthors *[]*UserCommitSummary,
) bool {
	f := logrus.Fields{
		"functionName": "github.github_repository.ExpandWithCoAuthors",
		"pr":           pr,
	}
	coAuthors := GetCoAuthorsFromCommit(ctx, commit)
	log.WithFields(f).Debugf("co-authors found: %s", coAuthors)
	missing := false
	for _, coAuthor := range coAuthors {
		summary, found := GetCoAuthorCommits(ctx, client, usersService, coAuthor, commit, pr, installationID)
		*commitAuthors = append(*commitAuthors, summary)
		if !missing && !found {
			missing = true
		}
	}
	return missing
}

// IsValidGitHubUsername checks if the provided username is a valid GitHub username.
func IsValidGitHubUsername(username string) bool {
	if !GithubUsernameRegex.MatchString(username) {
		return false
	}
	if strings.HasPrefix(username, "-") || strings.HasSuffix(username, "-") {
		return false
	}
	if strings.Contains(username, "--") {
		return false
	}
	return true
}

//nolint:gocyclo // complexity is acceptable for now
func GetCoAuthorCommits(
	ctx context.Context,
	client *github.Client,
	usersService users.Service,
	coAuthor [2]string,
	commit *github.RepositoryCommit,
	pr int,
	installationID int64,
) (*UserCommitSummary, bool) {
	f := logrus.Fields{
		"functionName":    "github.github_repository.GetCoAuthorCommits",
		"pr":              pr,
		"installation-id": installationID,
		"co-author-name":  coAuthor[0],
		"co-author-email": coAuthor[1],
	}

	var (
		user               *github.User
		githubID           int64
		name, email, login string
		err                error
	)
	name = strings.TrimSpace(coAuthor[0])
	email = strings.TrimSpace(coAuthor[1])
	lName := strings.ToLower(name)

	cacheKey := [2]string{lName, email}
	if cachedUser, ok := GithubUserCache.Get(cacheKey); ok {
		log.WithFields(f).Debugf("GitHub user found in cache for name/email: %s/%s: %+v", name, email, cachedUser)
		found := false
		var summary *UserCommitSummary
		if cachedUser != nil {
			summary = &UserCommitSummary{
				SHA:          utils.StringValue(commit.SHA),
				CommitAuthor: cachedUser,
				Affiliated:   false,
				Authorized:   false,
			}
			found = cachedUser.ID != nil
		} else {
			summary = &UserCommitSummary{
				SHA: utils.StringValue(commit.SHA),
				CommitAuthor: &github.User{
					Login: nil,
					ID:    nil,
					Name:  &name,
					Email: &email,
				},
				Affiliated: false,
				Authorized: false,
			}
		}
		log.WithFields(f).Debugf("PR: %d, %+v (from cache)", pr, summary)
		return summary, found
	}

	log.WithFields(f).Debugf("Getting co-author details: %+v", coAuthor)

	// 1. Check for email in "id+username@users.noreply.github.com" format:
	if matches := NoreplyIDPattern.FindStringSubmatch(email); matches != nil {
		idStr, loginStr := matches[1], matches[2]
		if githubID, err = strconv.ParseInt(idStr, 10, 64); err == nil {
			log.WithFields(f).Debugf("Detected noreply GitHub email with ID: %s, login: %s", idStr, loginStr)
			user, err = GetGithubUserByID(ctx, client, githubID)
			if err != nil {
				log.WithFields(f).Warnf("Error fetching user by ID %d: %v", githubID, err)
				user = nil
			}
		}
	}

	// 2. Check for email in "username@users.noreply.github.com" format:
	if user == nil {
		if matches := NoreplyUserPattern.FindStringSubmatch(email); matches != nil {
			loginStr := matches[1]
			log.WithFields(f).Debugf("Detected noreply GitHub email with login: %s", loginStr)
			user, err = GetGithubUserByLogin(ctx, client, loginStr)
			if err != nil {
				log.WithFields(f).Warnf("Error fetching user by login %s: %v", loginStr, err)
				user = nil
			}
		}
	}

	// 3. Try to find user by email via GitHub APIs
	if user == nil {
		user, err = SearchGithubUserByEmail(ctx, client, email)
		if err != nil {
			log.WithFields(f).Debugf("Co-author GitHub user not found via github email %s: %v (error: %v)", email, coAuthor, err)
			user = nil
		}
	}

	//	3b. Try to find user by email in our database
	if user == nil {
		var githubID string
		dbUsers, err2 := usersService.GetUsersByLFEmail(email)
		if err2 == nil {
			for _, dbUser := range dbUsers {
				if dbUser.GithubID != "" {
					githubID = dbUser.GithubID
					// log.WithFields(f).Debugf("FOUND githubID.1 = %s", githubID)
					break
				}
			}
		} else {
			log.WithFields(f).Debugf("Co-author GitHub user not found via lf email %s: %v (error: %v)", email, coAuthor, err2)
		}
		if githubID == "" {
			dbUsers, err2 := usersService.GetUsersByEmail(email)
			if err2 == nil {
				for _, dbUser := range dbUsers {
					if dbUser.GithubID != "" {
						githubID = dbUser.GithubID
						// log.WithFields(f).Debugf("FOUND githubID.2 = %s", githubID)
						break
					}
				}
			} else {
				log.WithFields(f).Debugf("Co-author GitHub user not found via emails %s: %v (error: %v)", email, coAuthor, err2)
			}
		}
		if githubID != "" {
			githubIDInt, err2 := strconv.ParseInt(githubID, 10, 64)
			if err2 != nil {
				log.WithFields(f).Debugf("Co-author GitHub user not found via lf email %s, wrong GitHub ID: %s: %v (error: %v)", email, githubID, coAuthor, err2)
			} else {
				user, err = GetGithubUserByID(ctx, client, githubIDInt)
				if err != nil {
					log.WithFields(f).Debugf("Error fetching user by ID %d: %v", githubIDInt, err)
					user = nil
				}
				// log.WithFields(f).Debugf("FOUND user = (%s, %d, %s, %s)", *user.Login, *user.ID, *user.Name, *user.Email)
			}
		}
	}

	// 4. Last resort - try to find by name=login
	if user == nil && IsValidGitHubUsername(lName) {
		// Note that Co-authored-by: name <email> is not actually a GitHub login but rather a name - but we are trying hard to find a GitHub profile
		user, err = GetGithubUserByLogin(ctx, client, lName)
		if err != nil {
			log.WithFields(f).Debugf("Co-author GitHub user not found via name=login=%s: %v (error: %v)", name, coAuthor, err)
			user = nil
		}
	}

	log.WithFields(f).Debugf("Co-author: %v, user: %+v", coAuthor, user)

	var summary *UserCommitSummary
	found := false
	if user != nil {
		if user.Login != nil {
			login = *user.Login
		}
		if user.ID != nil {
			githubID = *user.ID
			found = true
		}
		if user.Name == nil || (user.Name != nil && strings.TrimSpace(*user.Name) == "") {
			user.Name = &name
		}
		if user.Email == nil || (user.Email != nil && strings.TrimSpace(*user.Email) == "") {
			user.Email = &email
		}
		log.WithFields(f).Debugf("Co-author GitHub user details found: %v, user: %+v, login: %s, id: %d for email=%s, name=%s", coAuthor, user, login, githubID, email, name)
		summary = &UserCommitSummary{
			SHA:          utils.StringValue(commit.SHA),
			CommitAuthor: user,
			Affiliated:   false,
			Authorized:   false,
		}
		log.WithFields(f).Debugf("PR: %d, %+v", pr, summary)
	} else {
		summary = &UserCommitSummary{
			SHA: utils.StringValue(commit.SHA),
			CommitAuthor: &github.User{
				Login: nil,
				ID:    nil,
				Name:  &name,
				Email: &email,
			},
			Affiliated: false,
			Authorized: false,
		}
		log.WithFields(f).Debugf("Co-author GitHub user details not found: %v", coAuthor)
	}

	GithubUserCache.Set(cacheKey, user)
	return summary, found
}

func UserKey(id, login, email string) [3]string {
	return [3]string{id, strings.ToLower(login), strings.ToLower(strings.TrimSpace(email))}
}

func ProjectUserKey(projectID, id, login, email string) [4]string {
	return [4]string{projectID, id, strings.ToLower(login), strings.ToLower(strings.TrimSpace(email))}
}

// GetCommitAuthorSignedStatus checks if the commit author has signed the CLA for the given project
func GetCommitAuthorSignedStatus(
	ctx context.Context,
	usersService users.Service,
	hasUserSigned func(context.Context, *models.User, string) (*bool, *bool, error),
	projectID string,
	userSummary *UserCommitSummary,
	signed *[]*UserCommitSummary,
	unsigned *[]*UserCommitSummary,
) {
	f := logrus.Fields{
		"functionName": "github.github_repository.GetCommitAuthorsSignedStatuses",
		"projectID":    projectID,
		"userSummary":  *userSummary,
	}
	commitAuthorID := userSummary.GetCommitAuthorID()
	commitAuthorUsername := userSummary.GetCommitAuthorUsername()
	commitAuthorEmail := userSummary.GetCommitAuthorEmail()

	log.WithFields(f).Debugf("checking user - sha: %s, user ID: %s, username: %s, email: %s",
		userSummary.SHA, commitAuthorID, commitAuthorUsername, commitAuthorEmail)

	// LG: cache_authors - start
	// Per-project cache - also caches per-project signatures status and affiliation
	// (project_id, id, login, email) -> (user || None, authorized, affiliated)
	projectCacheKey := ProjectUserKey(projectID, commitAuthorID, commitAuthorUsername, commitAuthorEmail)
	cachedUser, authorized, affiliated, ok := ModelProjectUserCache.Get(projectCacheKey)
	if cachedUser != nil {
		log.WithFields(f).Debugf("per-project cache: %+v -> (%+v, %v, %v, %v)", projectCacheKey, *cachedUser, authorized, affiliated, ok)
	} else {
		log.WithFields(f).Debugf("per-project cache: %+v -> (%+v, nil, %v, %v)", projectCacheKey, authorized, affiliated, ok)
	}
	if ok {
		if cachedUser == nil {
			log.WithFields(f).Debugf("per-project cache: unsigned, user is null")
			*unsigned = append(*unsigned, userSummary)
			return
		}
		userSummary.Affiliated = affiliated
		if authorized {
			userSummary.Authorized = authorized
			log.WithFields(f).Debugf("per-project cache: signed")
			*signed = append(*signed, userSummary)
		} else {
			log.WithFields(f).Debugf("per-project cache: unsigned, authorized is false")
			*unsigned = append(*unsigned, userSummary)
		}
		return
	}
	// General cache (without project) - can only cache author details, but not per-project signature details
	// (id, login, email) -> (user || None)
	cacheKey := UserKey(commitAuthorID, commitAuthorUsername, commitAuthorEmail)
	cachedUser, ok = ModelUserCache.Get(cacheKey)
	if cachedUser != nil {
		log.WithFields(f).Debugf("general cache: %+v -> (%+v, %v)", cacheKey, *cachedUser, ok)
	} else {
		log.WithFields(f).Debugf("general cache: %+v -> (nil, %v)", cacheKey, ok)
	}
	if ok {
		if cachedUser == nil {
			log.WithFields(f).Debugf("general cache: unsigned, user is null")
			*unsigned = append(*unsigned, userSummary)
			ModelProjectUserCache.Set(projectCacheKey, nil, false, false)
			return
		}
		user := cachedUser
		userSigned, companyAffiliation, signedErr := hasUserSigned(ctx, user, projectID)
		if signedErr != nil {
			log.WithFields(f).WithError(signedErr).Warnf("has user signed error - user: %+v, project: %s", user, projectID)
			log.WithFields(f).Debugf("general cache: unsigned, hasUserSigned error")
			*unsigned = append(*unsigned, userSummary)
			ModelProjectUserCache.Set(projectCacheKey, user, false, false)
			return
		}

		if companyAffiliation != nil {
			userSummary.Affiliated = *companyAffiliation
		}

		if userSigned != nil {
			userSummary.Authorized = *userSigned
			if userSummary.Authorized {
				log.WithFields(f).Debugf("general cache: signed")
				*signed = append(*signed, userSummary)
				ModelProjectUserCache.Set(projectCacheKey, user, true, userSummary.Affiliated)
			} else {
				log.WithFields(f).Debugf("general cache: unsigned, authorized is false")
				*unsigned = append(*unsigned, userSummary)
				ModelProjectUserCache.Set(projectCacheKey, user, false, userSummary.Affiliated)
			}
		} else {
			log.WithFields(f).Debugf("general cache: unsigned, userSigned is null")
			*unsigned = append(*unsigned, userSummary)
			ModelProjectUserCache.Set(projectCacheKey, user, false, userSummary.Affiliated)
		}
		return
	}
	// LG: cache_authors - end

	var user *models.User
	var userErr error

	if commitAuthorID != "" {
		log.WithFields(f).Debugf("looking up user by ID: %s", commitAuthorID)
		user, userErr = usersService.GetUserByGitHubID(commitAuthorID)
		if userErr != nil {
			log.WithFields(f).WithError(userErr).Warnf("unable to get user by github id: %s", commitAuthorID)
		}
		if user != nil {
			log.WithFields(f).Debugf("found user by ID: %s", commitAuthorID)
		}
	}
	if user == nil && commitAuthorUsername != "" {
		log.WithFields(f).Debugf("looking up user by username: %s", commitAuthorUsername)
		user, userErr = usersService.GetUserByGitHubUsername(commitAuthorUsername)
		if userErr != nil {
			log.WithFields(f).WithError(userErr).Warnf("unable to get user by github username: %s", commitAuthorUsername)
		}
		if user != nil {
			log.WithFields(f).Debugf("found user by username: %s", commitAuthorUsername)
		}
	}
	if user == nil && commitAuthorEmail != "" {
		log.WithFields(f).Debugf("looking up user by email: %s", commitAuthorEmail)
		user, userErr = usersService.GetUserByEmail(commitAuthorEmail)
		if userErr != nil {
			log.WithFields(f).WithError(userErr).Warnf("unable to get user by user email: %s", commitAuthorEmail)
		}
		if user != nil {
			log.WithFields(f).Debugf("found user by email: %s", commitAuthorEmail)
		}
	}

	if user == nil {
		log.WithFields(f).Debugf("unable to find user for commit author - sha: %s, user ID: %s, username: %s, email: %s",
			userSummary.SHA, commitAuthorID, commitAuthorUsername, commitAuthorEmail)
		log.WithFields(f).Debugf("store caches: unsigned, user is null")
		*unsigned = append(*unsigned, userSummary)
		ModelProjectUserCache.Set(projectCacheKey, nil, false, false)
		ModelUserCache.Set(cacheKey, nil)
		return
	}

	log.WithFields(f).Debugf("checking to see if user has signed an ICLA or ECLA for project: %s", projectID)
	userSigned, companyAffiliation, signedErr := hasUserSigned(ctx, user, projectID)
	if signedErr != nil {
		log.WithFields(f).WithError(signedErr).Warnf("has user signed error - user: %+v, project: %s", user, projectID)
		log.WithFields(f).Debugf("store caches: unsigned, hasUserSigned error")
		*unsigned = append(*unsigned, userSummary)
		ModelProjectUserCache.Set(projectCacheKey, user, false, false)
		ModelUserCache.Set(cacheKey, user)
		return
	}

	if companyAffiliation != nil {
		userSummary.Affiliated = *companyAffiliation
	}

	if userSigned != nil {
		userSummary.Authorized = *userSigned
		if userSummary.Authorized {
			log.WithFields(f).Debugf("store caches: signed")
			*signed = append(*signed, userSummary)
			ModelProjectUserCache.Set(projectCacheKey, user, true, userSummary.Affiliated)
			ModelUserCache.Set(cacheKey, user)
		} else {
			log.WithFields(f).Debugf("store caches: unsigned, authorized is false")
			*unsigned = append(*unsigned, userSummary)
			ModelProjectUserCache.Set(projectCacheKey, user, false, userSummary.Affiliated)
			ModelUserCache.Set(cacheKey, user)
		}
	} else {
		log.WithFields(f).Debugf("store caches: unsigned, userSigned is null")
		*unsigned = append(*unsigned, userSummary)
		ModelProjectUserCache.Set(projectCacheKey, user, false, userSummary.Affiliated)
		ModelUserCache.Set(cacheKey, user)
	}
}

// GetCommitAuthorsSignedStatuses returns two slices of UserCommitSummary - signed and unsigned for the given project and commit authors
func GetCommitAuthorsSignedStatuses(
	ctx context.Context,
	usersService users.Service,
	hasUserSigned func(context.Context, *models.User, string) (*bool, *bool, error),
	projectID string,
	authors []*UserCommitSummary,
) ([]*UserCommitSummary, []*UserCommitSummary) {
	f := logrus.Fields{
		"functionName": "github.github_repository.GetCommitAuthorsSignedStatuses",
		"projectID":    projectID,
	}
	signed := make([]*UserCommitSummary, 0)
	unsigned := make([]*UserCommitSummary, 0)

	// triage signed and unsigned users
	log.WithFields(f).Debugf("checking %d commit authors", len(authors))
	for _, userSummary := range authors {
		if userSummary == nil || !userSummary.IsValid() {
			if userSummary == nil {
				log.WithFields(f).Debugf("invalid user summary: nil")
			} else {
				log.WithFields(f).Debugf("invalid user summary: %+v", *userSummary)
			}
			unsigned = append(unsigned, userSummary)
			continue
		}
		GetCommitAuthorSignedStatus(ctx, usersService, hasUserSigned, projectID, userSummary, &signed, &unsigned)
	}
	return signed, unsigned
}

// GetCommitAuthorsSignedStatusesST returns two slices of UserCommitSummary - signed and unsigned for the given project and commit authors
// ST suffix = single threaded version
func GetCommitAuthorsSignedStatusesST(
	ctx context.Context,
	usersService users.Service,
	hasUserSigned func(context.Context, *models.User, string) (*bool, *bool, error),
	projectID string,
	authors []*UserCommitSummary,
) ([]*UserCommitSummary, []*UserCommitSummary) {
	f := logrus.Fields{
		"functionName": "github.github_repository.GetCommitAuthorsSignedStatusesST",
		"projectID":    projectID,
	}
	signed := make([]*UserCommitSummary, 0)
	unsigned := make([]*UserCommitSummary, 0)

	// triage signed and unsigned users
	log.WithFields(f).Debugf("checking %d commit authors", len(authors))
	for _, userSummary := range authors {
		if userSummary == nil || !userSummary.IsValid() {
			if userSummary == nil {
				log.WithFields(f).Debugf("invalid user summary: nil")
			} else {
				log.WithFields(f).Debugf("invalid user summary: %+v", *userSummary)
			}
			unsigned = append(unsigned, userSummary)
			continue
		}
		GetCommitAuthorSignedStatus(ctx, usersService, hasUserSigned, projectID, userSummary, &signed, &unsigned)
	}
	return signed, unsigned
}

func GetPullRequestCommitAuthors(ctx context.Context, usersService users.Service, installationID int64, pullRequestID int, owner, repo string, withCoAuthors bool) ([]*UserCommitSummary, *string, bool, error) {
	f := logrus.Fields{
		"functionName":  "github.github_repository.GetPullRequestCommitAuthors",
		"pullRequestID": pullRequestID,
		"withCoAuthors": withCoAuthors,
	}
	var userCommitSummary []*UserCommitSummary

	client, err := NewGithubAppClient(installationID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to create Github client")
		return nil, nil, false, err
	}

	commits, resp, comErr := client.PullRequests.ListCommits(ctx, owner, repo, pullRequestID, &github.ListOptions{})
	if comErr != nil {
		log.WithFields(f).WithError(comErr).Warnf("problem listing commits for repo: %s/%s pull request: %d", owner, repo, pullRequestID)
		return nil, nil, false, comErr
	}
	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("unexpected status code: %d - expected: %d", resp.StatusCode, http.StatusOK)
		log.WithFields(f).Warn(msg)
		return nil, nil, false, errors.New(msg)
	}

	log.WithFields(f).Debugf("found %d commits for pull request: %d", len(commits), pullRequestID)
	anyMissing := false
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
		name, email := "", ""
		if commit.Commit != nil && commit.Commit.Author != nil {
			name = utils.StringValue(commit.Commit.Author.Name)
			email = utils.StringValue(commit.Commit.Author.Email)
			if strings.TrimSpace(name) != "" && (commit.Author.Name == nil || (commit.Author.Name != nil && strings.TrimSpace(*commit.Author.Name) == "")) {
				commit.Author.Name = &name
			}
			if strings.TrimSpace(email) != "" && (commit.Author.Email == nil || (commit.Author.Email != nil && strings.TrimSpace(*commit.Author.Email) == "")) {
				commit.Author.Email = &email
			}
		}
		log.WithFields(f).Debugf("commitAuthor: %s, name: %s, email: %s", commitAuthor, name, email)
		userCommitSummary = append(userCommitSummary, &UserCommitSummary{
			SHA:          *commit.SHA,
			CommitAuthor: commit.Author,
			Affiliated:   false,
			Authorized:   false,
		})
		if withCoAuthors {
			missing := ExpandWithCoAuthors(ctx, client, usersService, commit, pullRequestID, installationID, &userCommitSummary)
			if !anyMissing && missing {
				anyMissing = true
			}
		}
	}

	// get latest commit SHA
	latestCommitSHA := commits[len(commits)-1].SHA
	// log.WithFields(f).Debugf("user commit summaries: %+v", userCommitSummary)
	// for _, summary := range userCommitSummary {
	//	if summary == nil {
	//		continue
	//	}
	//	log.WithFields(f).Debugf("user commit summary: %+v", *summary)
	//}
	return userCommitSummary, latestCommitSHA, anyMissing, nil
}

func UpdatePullRequest(ctx context.Context, installationID int64, pullRequestID int, owner, repo string, repoID *int64, latestSHA string, signed []*UserCommitSummary, missing []*UserCommitSummary, anyMissing bool, CLABaseAPIURL, CLALandingPage, CLALogoURL string) error {
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

	body := assembleCLAComment(ctx, int(installationID), pullRequestID, repoID, signed, missing, anyMissing, CLABaseAPIURL, CLALogoURL, CLALandingPage)

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
			failedComment := assembleCLAComment(ctx, int(installationID), pullRequestID, repoID, signed, missing, anyMissing, CLABaseAPIURL, CLALogoURL, CLALandingPage)
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

func assembleCLAComment(ctx context.Context, installationID, pullRequestID int, repositoryID *int64, signed, missing []*UserCommitSummary, anyMissing bool, apiBaseURL, CLALogoURL, CLALandingPage string) string {
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
	commentBody := getCommentBody(repositoryType, signURL, signed, missing, anyMissing)
	allSigned := len(missing) == 0
	badge := getCommentBadge(allSigned, signURL, missingID, false, CLALandingPage, CLALogoURL)
	return fmt.Sprintf("%s<br >%s", badge, commentBody)
}

func getCommentBody(repositoryType, signURL string, signed, missing []*UserCommitSummary, anyMissing bool) string {
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

	if anyMissing {
		committersComment.WriteString(strings.ReplaceAll(MissingCoAuthorsMessage, "|", "`"))
		log.WithFields(f).Debug("some co-authors are missing for this PR, added the missing co-author message")
	}
	return fmt.Sprintf("%s%s", committersComment.String(), text)
}

func getCommentBadge(allSigned bool, signURL string, missingUserId, managerApproved bool, CLALandingPage, CLALogoURL string) string {
	var alt string
	var text string
	var badgeHyperLink string
	var badgeURL string

	if allSigned {
		badgeURL = fmt.Sprintf("%s/cla-signed.svg%s", CLALogoURL, svgVersion)
		badgeHyperLink = fmt.Sprintf("%s/#/?version=2", CLALandingPage)
		alt = "CLA Signed"
		return fmt.Sprintf(`<a href="%s"><img src="%s" alt="%s" align="left" height="28" width="328" >`, badgeHyperLink, badgeURL, alt)
	}
	badgeHyperLink = signURL
	if missingUserId {
		badgeURL = fmt.Sprintf("%s/cla-missing-id.svg%s", CLALogoURL, svgVersion)
		alt = "CLA Missing ID"
	} else if managerApproved {
		badgeURL = fmt.Sprintf("%s/cla-confirmation-needed.svg%s", CLALogoURL, svgVersion)
		alt = "CLA Confirmation Needed"
	} else {
		badgeURL = fmt.Sprintf("%s/cla-not-signed.svg%s", CLALogoURL, svgVersion)
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
