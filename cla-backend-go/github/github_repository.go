// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	unknown          = "Unknown"
	failureState     = "failure"
	successState     = "success"
	svgVersion       = "?v=2"
	NegativeCacheTTL = 3 * time.Minute // Used for negative caching of missing/not-signed users
	ProjectCacheTTL  = 3 * time.Hour   // Used for per-project caching of signed users
)

// GraphQL related types
type gqlRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

type gqlError struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"` // sometimes "RATE_LIMITED"
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors,omitempty"`
}

// doGraphQL posts to /graphql using v3 client and unmarshals the "data" field into v.
// No retries; if GraphQL returns "errors", returns an error.
func doGraphQL(ctx context.Context, c *github.Client, query string, variables map[string]interface{}, v any) (*github.Response, error) {
	reqBody := gqlRequest{Query: query, Variables: variables}
	req, err := c.NewRequest("POST", "graphql", reqBody) // -> https://api.github.com/graphql
	if err != nil {
		return nil, err
	}
	var gr gqlResponse
	resp, err := c.Do(ctx, req, &gr)
	if err != nil {
		return resp, err
	}
	if len(gr.Errors) > 0 {
		first := gr.Errors[0]
		return resp, fmt.Errorf("graphql error: %s", first.Message)
	}
	if v != nil && len(gr.Data) > 0 {
		if err := json.Unmarshal(gr.Data, v); err != nil {
			return resp, fmt.Errorf("unmarshal graphql data: %w", err)
		}
	}
	return resp, nil
}

type prCommitsPage struct {
	Repository struct {
		PullRequest struct {
			Commits struct {
				TotalCount int `json:"totalCount"`
				PageInfo   struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []struct {
					Commit struct {
						OID     string `json:"oid"`
						Message string `json:"message"`
						Author  struct {
							Name  string `json:"name"`  // commit metadata author
							Email string `json:"email"` // commit metadata author
							User  struct {
								DatabaseID int    `json:"databaseId"`
								Login      string `json:"login"`
								Name       string `json:"name"`  // profile
								Email      string `json:"email"` // profile (often empty)
							} `json:"user"`
						} `json:"author"`
					} `json:"commit"`
				} `json:"nodes"`
			} `json:"commits"`
		} `json:"pullRequest"`
	} `json:"repository"`
}

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

func (c *Cache) SetWithTTL(key [2]string, value *github.User, tl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(tl),
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

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[[2]string]cacheEntry)
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

func (c *UserCache) SetWithTTL(key [3]string, value *models.User, tl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = userCacheEntry{
		value:     value,
		expiresAt: time.Now().Add(tl),
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

func (c *UserCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[[3]string]userCacheEntry)
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

func (c *ProjectUserCache) SetWithTTL(key [4]string, value *models.User, signed, affiliated bool, tl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = projectUserCacheEntry{
		value:      value,
		signed:     signed,
		affiliated: affiliated,
		expiresAt:  time.Now().Add(tl),
	}
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

func (c *ProjectUserCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[[4]string]projectUserCacheEntry)
}

func (c *ProjectUserCache) Delete(key [4]string) { c.mu.Lock(); delete(c.data, key); c.mu.Unlock() }

var GithubUserCache = NewCache(12 * time.Hour)
var ModelUserCache = NewUserCache(12 * time.Hour)
var ModelProjectUserCache = NewProjectUserCache(3 * time.Hour)

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

// ClearCaches clears all in-memory caches maintained by the GitHub module.
func ClearCaches() {
	f := logrus.Fields{
		"functionName": "github.github_repository.ClearCaches",
	}
	GithubUserCache.Clear()
	ModelUserCache.Clear()
	ModelProjectUserCache.Clear()
	log.WithFields(f).Info("cleared caches")
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

	return strings.TrimSuffix(sb.String(), " / ")
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
	mu *sync.Mutex,
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
		mu.Lock()
		*commitAuthors = append(*commitAuthors, summary)
		mu.Unlock()
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
	if found {
		GithubUserCache.Set(cacheKey, user)
	} else {
		// negative cache for 30 minutes (this is for GitHub user not found)
		GithubUserCache.SetWithTTL(cacheKey, user, 30*time.Minute)
	}

	return summary, found
}

func UserKey(id, login, email string) [3]string {
	return [3]string{id, strings.ToLower(login), strings.ToLower(strings.TrimSpace(email))}
}

func ProjectUserKey(projectID, id, login, email string) [4]string {
	return [4]string{projectID, id, strings.ToLower(login), strings.ToLower(strings.TrimSpace(email))}
}

// strStripLower mirrors the Python str_strip_lower
func strStripLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// DedupAndSortCommitSummaries mirrors Python dedup_and_sort
// Dedupe key: (author_id, login, email, sha)
// Sort key:   login, name, email, sha  (all case-insensitive)
func DedupAndSortCommitSummaries(items []*UserCommitSummary) []*UserCommitSummary {
	seen := make(map[string]struct{}, len(items))
	uniq := make([]*UserCommitSummary, 0, len(items))

	for _, s := range items {
		if s == nil || s.CommitAuthor == nil {
			continue
		}
		var id int64
		if s.CommitAuthor.ID != nil {
			id = *s.CommitAuthor.ID
		}
		login := strStripLower(utils.StringValue(s.CommitAuthor.Login))
		email := strStripLower(utils.StringValue(s.CommitAuthor.Email))
		key := fmt.Sprintf("%d|%s|%s|%s", id, login, email, s.SHA)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		uniq = append(uniq, s)
	}

	sort.SliceStable(uniq, func(i, j int) bool {
		ai, aj := uniq[i], uniq[j]
		li := strStripLower(utils.StringValue(ai.CommitAuthor.Login))
		lj := strStripLower(utils.StringValue(aj.CommitAuthor.Login))
		if li != lj {
			return li < lj
		}
		ni := strStripLower(utils.StringValue(ai.CommitAuthor.Name))
		nj := strStripLower(utils.StringValue(aj.CommitAuthor.Name))
		if ni != nj {
			return ni < nj
		}
		ei := strStripLower(utils.StringValue(ai.CommitAuthor.Email))
		ej := strStripLower(utils.StringValue(aj.CommitAuthor.Email))
		if ei != ej {
			return ei < ej
		}
		return ai.SHA < aj.SHA
	})

	return uniq
}

// NormalizeComment mirrors Python normalize_comment
func NormalizeComment(s string) string {
	if s == "" {
		return ""
	}
	// Normalize newlines
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	// Trim trailing spaces per line
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	// Drop trailing blank lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
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
	mu *sync.Mutex,
) {
	// here userSummary is NOT nil
	f := logrus.Fields{
		"functionName": "github.github_repository.GetCommitAuthorSignedStatus",
		"projectID":    projectID,
	}
	commitAuthorID := userSummary.GetCommitAuthorID()
	commitAuthorUsername := userSummary.GetCommitAuthorUsername()
	commitAuthorEmail := userSummary.GetCommitAuthorEmail()
	f["authorID"] = commitAuthorID
	f["authorLogin"] = commitAuthorUsername
	f["authorEmail"] = commitAuthorEmail

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
		log.WithFields(f).Debugf("per-project cache: %+v -> (nil, %v, %v, %v)", projectCacheKey, authorized, affiliated, ok)
	}
	if ok {
		if cachedUser == nil {
			log.WithFields(f).Debugf("per-project cache: unsigned, user is null")
			mu.Lock()
			*unsigned = append(*unsigned, userSummary)
			mu.Unlock()
			return
		}
		userSummary.Affiliated = affiliated
		if authorized {
			userSummary.Authorized = authorized
			log.WithFields(f).Debugf("per-project cache: signed")
			mu.Lock()
			*signed = append(*signed, userSummary)
			mu.Unlock()
		} else {
			log.WithFields(f).Debugf("per-project cache: unsigned, authorized is false")
			mu.Lock()
			*unsigned = append(*unsigned, userSummary)
			mu.Unlock()
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
			mu.Lock()
			*unsigned = append(*unsigned, userSummary)
			mu.Unlock()
			log.WithFields(f).Debugf("store per-project cache: unsigned, user is null (%+v)", projectCacheKey)
			ModelProjectUserCache.SetWithTTL(projectCacheKey, nil, false, false, NegativeCacheTTL)
			return
		}
		user := cachedUser
		userSigned, companyAffiliation, signedErr := hasUserSigned(ctx, user, projectID)
		if signedErr != nil {
			log.WithFields(f).WithError(signedErr).Warnf("has user signed error - user: %+v, project: %s", user, projectID)
			mu.Lock()
			*unsigned = append(*unsigned, userSummary)
			mu.Unlock()
			log.WithFields(f).Debugf("store per-project cache: unsigned, hasUserSigned error (%+v)", projectCacheKey)
			ModelProjectUserCache.SetWithTTL(projectCacheKey, user, false, false, NegativeCacheTTL)
			return
		}

		if companyAffiliation != nil {
			userSummary.Affiliated = *companyAffiliation
		}

		if userSigned != nil {
			userSummary.Authorized = *userSigned
			if userSummary.Authorized {
				mu.Lock()
				*signed = append(*signed, userSummary)
				mu.Unlock()
				log.WithFields(f).Debugf("store per-project cache: signed (%+v)", projectCacheKey)
				ModelProjectUserCache.Set(projectCacheKey, user, true, userSummary.Affiliated)
			} else {
				mu.Lock()
				*unsigned = append(*unsigned, userSummary)
				mu.Unlock()
				log.WithFields(f).Debugf("store per-project cache: unsigned, authorized is false (%+v)", projectCacheKey)
				ModelProjectUserCache.SetWithTTL(projectCacheKey, user, false, userSummary.Affiliated, NegativeCacheTTL)
			}
		} else {
			mu.Lock()
			*unsigned = append(*unsigned, userSummary)
			mu.Unlock()
			log.WithFields(f).Debugf("store per-project cache: unsigned, userSigned is null (%+v)", projectCacheKey)
			ModelProjectUserCache.SetWithTTL(projectCacheKey, user, false, userSummary.Affiliated, NegativeCacheTTL)
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
		log.WithFields(f).Debugf("store caches: unsigned, user is null (%+v)", projectCacheKey)
		mu.Lock()
		*unsigned = append(*unsigned, userSummary)
		mu.Unlock()
		ModelProjectUserCache.SetWithTTL(projectCacheKey, nil, false, false, NegativeCacheTTL)
		ModelUserCache.SetWithTTL(cacheKey, nil, NegativeCacheTTL)
		return
	}

	log.WithFields(f).Debugf("checking to see if user has signed an ICLA or ECLA for project: %s", projectID)
	userSigned, companyAffiliation, signedErr := hasUserSigned(ctx, user, projectID)
	if signedErr != nil {
		log.WithFields(f).WithError(signedErr).Warnf("has user signed error - user: %+v, project: %s", user, projectID)
		log.WithFields(f).Debugf("store caches: unsigned, hasUserSigned error (%+v)", projectCacheKey)
		mu.Lock()
		*unsigned = append(*unsigned, userSummary)
		mu.Unlock()
		ModelProjectUserCache.SetWithTTL(projectCacheKey, user, false, false, NegativeCacheTTL)
		ModelUserCache.SetWithTTL(cacheKey, user, NegativeCacheTTL)
		return
	}

	if companyAffiliation != nil {
		userSummary.Affiliated = *companyAffiliation
	}

	if userSigned != nil {
		userSummary.Authorized = *userSigned
		if userSummary.Authorized {
			log.WithFields(f).Debugf("store caches: signed (%+v)", projectCacheKey)
			mu.Lock()
			*signed = append(*signed, userSummary)
			mu.Unlock()
			ModelProjectUserCache.Set(projectCacheKey, user, true, userSummary.Affiliated)
			ModelUserCache.Set(cacheKey, user)
		} else {
			log.WithFields(f).Debugf("store caches: unsigned, authorized is false (%+v)", projectCacheKey)
			mu.Lock()
			*unsigned = append(*unsigned, userSummary)
			mu.Unlock()
			ModelProjectUserCache.SetWithTTL(projectCacheKey, user, false, userSummary.Affiliated, NegativeCacheTTL)
			ModelUserCache.SetWithTTL(cacheKey, user, NegativeCacheTTL)
		}
	} else {
		log.WithFields(f).Debugf("store caches: unsigned, userSigned is null (%+v)", projectCacheKey)
		mu.Lock()
		*unsigned = append(*unsigned, userSummary)
		mu.Unlock()
		ModelProjectUserCache.SetWithTTL(projectCacheKey, user, false, userSummary.Affiliated, NegativeCacheTTL)
		ModelUserCache.SetWithTTL(cacheKey, user, NegativeCacheTTL)
	}
}

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
	log.WithFields(f).Debugf("checking %d commit authors", len(authors))
	signed := make([]*UserCommitSummary, 0, len(authors))
	unsigned := make([]*UserCommitSummary, 0, len(authors))
	var mu sync.Mutex
	var wg sync.WaitGroup
	maxConc := runtime.NumCPU()
	if maxConc < 1 {
		maxConc = 1
	}
	sem := make(chan struct{}, maxConc)
	for _, us := range authors {
		if us == nil || !us.IsValid() {
			log.WithFields(f).Debugf("invalid user summary: %v", us)
			mu.Lock()
			unsigned = append(unsigned, us)
			mu.Unlock()
			continue
		}

		wg.Add(1)
		sem <- struct{}{} // acquire a slot
		go func(userSummary *UserCommitSummary) {
			defer wg.Done()
			defer func() { <-sem }() // release slot

			GetCommitAuthorSignedStatus(ctx, usersService, hasUserSigned, projectID, userSummary, &signed, &unsigned, &mu)
		}(us)
	}

	wg.Wait()
	signed = DedupAndSortCommitSummaries(signed)
	unsigned = DedupAndSortCommitSummaries(unsigned)
	return signed, unsigned
}

func GetPullRequestCommitAuthors(
	ctx context.Context,
	usersService users.Service,
	installationID int64,
	pullRequestID int,
	owner, repo string,
	withCoAuthors bool,
) ([]*UserCommitSummary, bool, error) {
	f := logrus.Fields{
		"functionName":  "github.github_repository.GetPullRequestCommitAuthors",
		"pullRequestID": pullRequestID,
		"withCoAuthors": withCoAuthors,
	}

	client, err := NewGithubAppClient(installationID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to create Github client")
		return nil, false, err
	}

	const pageSize = 100 // GraphQL max
	const query = `
query($owner:String!, $name:String!, $number:Int!, $pageSize:Int!, $cursor:String) {
  repository(owner:$owner, name:$name) {
    pullRequest(number:$number) {
      commits(first:$pageSize, after:$cursor) {
        totalCount
        pageInfo { hasNextPage endCursor }
        nodes {
          commit {
            oid
            message
            author {
              name
              email
              user {
                databaseId
                login
                name
                email
              }
            }
          }
        }
      }
    }
  }
}`

	var (
		userCommitSummary []*UserCommitSummary
		anyMissing        atomic.Bool

		mu      sync.Mutex
		wg      sync.WaitGroup
		maxConc = runtime.NumCPU()
	)
	if maxConc < 1 {
		maxConc = 1
	}
	sem := make(chan struct{}, maxConc)

	var (
		cursor      *string
		totalLogged bool
	)

	for {
		vars := map[string]interface{}{
			"owner":    owner,
			"name":     repo,
			"number":   pullRequestID,
			"pageSize": pageSize,
			"cursor":   nil,
		}
		if cursor != nil {
			vars["cursor"] = *cursor
		}

		var page prCommitsPage
		if _, err := doGraphQL(ctx, client, query, vars, &page); err != nil {
			log.WithFields(f).WithError(err).Warnf("problem listing commits via GraphQL for %s/%s PR #%d", owner, repo, pullRequestID)
			return nil, false, err
		}

		c := page.Repository.PullRequest.Commits
		if !totalLogged {
			log.WithFields(f).Debugf("found %d commits (totalCount) for pull request: %d", c.TotalCount, pullRequestID)
			totalLogged = true
			userCommitSummary = make([]*UserCommitSummary, 0, c.TotalCount)
		}

		// Launch per-commit workers for this page
		for _, node := range c.Nodes {
			n := node // capture
			wg.Add(1)
			sem <- struct{}{} // acquire slot
			go func() {
				defer wg.Done()
				defer func() { <-sem }() // release slot

				sha := n.Commit.OID
				msg := n.Commit.Message
				a := n.Commit.Author
				u := a.User

				// Legacy precedence: user.* preferred, else commit author fields
				var (
					id64  *int64
					login *string
					name  *string
					email *string
				)
				if u.DatabaseID != 0 {
					tmp := int64(u.DatabaseID)
					id64 = &tmp
				}
				if u.Login != "" {
					tmp := u.Login
					login = &tmp
				}
				if u.Name != "" {
					tmp := u.Name
					name = &tmp
				} else if a.Name != "" {
					tmp := a.Name
					name = &tmp
				}
				if u.Email != "" {
					tmp := u.Email
					email = &tmp
				} else if a.Email != "" {
					tmp := a.Email
					email = &tmp
				}

				// Minimal go-github objects to keep ExpandWithCoAuthors working
				ghUser := &github.User{
					ID:    id64,
					Login: login,
					Name:  name,
					Email: email,
				}
				rc := &github.RepositoryCommit{
					SHA: &sha,
					Commit: &github.Commit{
						Message: github.String(msg),
						Author: &github.CommitAuthor{
							Name:  github.String(a.Name),
							Email: github.String(a.Email),
						},
					},
					Author: ghUser,
				}

				// Append main author summary
				mu.Lock()
				userCommitSummary = append(userCommitSummary, &UserCommitSummary{
					SHA:          sha,
					CommitAuthor: ghUser,
					Affiliated:   false,
					Authorized:   false,
				})
				mu.Unlock()

				if withCoAuthors {
					if ExpandWithCoAuthors(ctx, client, usersService, rc, pullRequestID, installationID, &userCommitSummary, &mu) {
						anyMissing.Store(true)
					}
				}
			}()
		}

		if !c.PageInfo.HasNextPage {
			break
		}
		cur := c.PageInfo.EndCursor
		cursor = &cur
	}

	// Wait for all workers to finish
	wg.Wait()

	log.WithFields(f).Debugf("total commit author summaries (including co-authors) for PR %d: %d, any missing: %v", pullRequestID, len(userCommitSummary), anyMissing.Load())
	return userCommitSummary, anyMissing.Load(), nil
}

// EditIssueCommentIfChanged fetches the existing comment and edits only if
// NormalizeComment(existing) != NormalizeComment(newBody). Returns true if edited.
func EditIssueCommentIfChanged(ctx context.Context, client *github.Client, owner, repo string, prNum int, commentID int64, newBody string) (bool, error) {
	f := logrus.Fields{
		"functionName": "github.github_repository.EditIssueCommentIfChanged",
		"owner":        owner,
		"repo":         repo,
		"prNum":        prNum,
		"commentID":    commentID,
	}
	existing, _, err := client.Issues.GetComment(ctx, owner, repo, commentID)
	if err != nil {
		return false, err
	}
	oldNorm := NormalizeComment(utils.StringValue(existing.Body))
	newNorm := NormalizeComment(newBody)
	if oldNorm == newNorm {
		return false, nil
	}
	log.WithFields(f).Debugf("editing comment %d on %s/%s PR #%d", commentID, owner, repo, prNum)
	log.WithFields(f).Debugf("old comment:\n%s\n---\nnew comment:\n%s\n---", oldNorm, newNorm)
	_, _, err = client.Issues.EditComment(ctx, owner, repo, commentID, &github.IssueComment{Body: &newBody})
	if err != nil {
		return false, err
	}
	return true, nil
}

// Head SHA for a PR (authoritative "last commit")
func GetPRHeadSHA(ctx context.Context, gh *github.Client, owner, repo string, prNumber int) (string, error) {
	pr, _, err := gh.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return "", err
	}
	sha := ""
	if pr.Head != nil && pr.Head.SHA != nil {
		sha = *pr.Head.SHA
	}
	if sha == "" {
		return "", fmt.Errorf("missing head SHA for %s/%s PR #%d", owner, repo, prNumber)
	}
	return sha, nil
}

func UpdatePullRequest(ctx context.Context, installationID int64, pullRequestID int, owner, repo string, repoID *int64, signed []*UserCommitSummary, missing []*UserCommitSummary, anyMissing bool, CLABaseAPIURL, CLALandingPage, CLALogoURL string) error {
	f := logrus.Fields{
		"functionName":   "github.github_repository.UpdatePullRequest",
		"installationID": installationID,
		"owner":          owner,
		"repo":           repo,
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
		if previouslyFailed {
			edited, err2 := EditIssueCommentIfChanged(ctx, client, owner, repo, pullRequestID, *comment.ID, body)
			if err2 != nil {
				log.WithFields(f).WithError(err2).Debug("unable to edit comment")
				return err2
			}
			if edited {
				log.WithFields(f).Debugf("Updated CLA comment for PR %d (body changed).", pullRequestID)
			} else {
				log.WithFields(f).Debugf("CLA comment unchanged for PR %d, skipping edit.", pullRequestID)
			}
		}
	} else {
		// One or more contributors are failing
		if previouslyFailed {
			edited, err2 := EditIssueCommentIfChanged(ctx, client, owner, repo, pullRequestID, *comment.ID, body)
			if err2 != nil {
				log.WithFields(f).WithError(err2).Debug("unable to edit comment")
				return err2
			}
			if edited {
				log.WithFields(f).Debugf("Updated failing CLA comment for PR %d (body changed).", pullRequestID)
			} else {
				log.WithFields(f).Debugf("Failing CLA comment unchanged for PR %d, skipping edit.", pullRequestID)
			}
		} else if previouslySucceeded {
			// pass => fail transition; still avoid redundant edit
			failedBody := assembleCLAComment(ctx, int(installationID), pullRequestID, repoID, signed, missing, anyMissing, CLABaseAPIURL, CLALogoURL, CLALandingPage)
			edited, err2 := EditIssueCommentIfChanged(ctx, client, owner, repo, pullRequestID, *previousSucceededComment.ID, failedBody)
			if err2 != nil {
				log.WithFields(f).WithError(err2).Debug("unable to edit previous success comment")
				return err2
			}
			if edited {
				log.WithFields(f).Debugf("Updated previously succeeded comment to failing for PR %d.", pullRequestID)
			} else {
				log.WithFields(f).Debugf("Previously succeeded comment already matches failing body for PR %d; skipping edit.", pullRequestID)
			}
		} else {
			// No previous comment - create new with the current body
			newComment := &github.IssueComment{Body: &body}
			_, _, err2 := client.Issues.CreateComment(ctx, owner, repo, pullRequestID, newComment)
			if err2 != nil {
				log.WithFields(f).WithError(err2).Debug("unable to create comment")
				return err2
			}
			log.WithFields(f).Debugf("Created new failing CLA comment for PR %d.", pullRequestID)
		}
	}

	// Update/Create the status
	ctxName := "EasyCLA"
	var statusBody string
	var state string
	var signURL string

	if len(missing) > 0 {
		state = failureState
		ctxName, statusBody = assembleCLAStatus(ctxName, false)
		signURL = getFullSignURL("github", strconv.Itoa(int(installationID)), strconv.Itoa(int(*repoID)), strconv.Itoa(pullRequestID), CLABaseAPIURL)
		log.WithFields(f).Debugf("Creating new CLA %s status - %d passed, %d missing, signing url %s", state, len(signed), len(missing), signURL)
	} else if len(signed) > 0 {
		state = successState
		ctxName, statusBody = assembleCLAStatus(ctxName, true)
		signURL = fmt.Sprintf("%s/#/?version=2", CLALandingPage)
		log.WithFields(f).Debugf("Creating new CLA %s status - %d passed, %d missing, signing url %s", state, len(signed), len(missing), signURL)

	} else {
		state = failureState
		ctxName, statusBody = assembleCLAStatus(ctxName, false)
		signURL = getFullSignURL("github", strconv.Itoa(int(installationID)), strconv.Itoa(int(*repoID)), strconv.Itoa(pullRequestID), CLABaseAPIURL)
		log.WithFields(f).Debugf("Creating new CLA %s status - %d passed, %d missing, signing url %s", state, len(signed), len(missing), signURL)
		log.WithFields(f).Debugf("This is an error condition - should have at least one committer in one of these lists: signed : %+v passed, %+v", signed, missing)
	}

	status := Status{
		State:       &state,
		TargetURL:   &signURL,
		Context:     &ctxName,
		Description: &statusBody,
	}

	log.WithFields(f).Debugf("Creating status: %+v", status)

	headSHA, err := GetPRHeadSHA(ctx, client, owner, repo, pullRequestID)
	if err != nil {
		return err
	}

	_, _, err = CreateStatus(ctx, client, owner, repo, headSHA, &status)
	if err != nil {
		log.WithFields(f).WithError(err).Debugf("unable to create status on %s", headSHA)
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
		if strings.Contains(*comment.Body, "is not linked to the GitHub account") {
			return true, comment, nil
		}
	}
	return false, nil, nil
}

// UpdateCacheAfterSignature marks the user as authorized for the given project
func UpdateCacheAfterSignature(ctx context.Context, user *models.User, projectID string) error {
	f := logrus.Fields{
		"functionName": "github.github_repository.UpdateCacheAfterSignature",
		"projectID":    projectID,
	}

	if user == nil {
		log.WithFields(f).Warn("nil user passed to UpdateCacheAfterSignature")
		return fmt.Errorf("nil user")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		log.WithFields(f).Warn("empty projectID passed to UpdateCacheAfterSignature")
		return fmt.Errorf("empty projectID")
	}

	githubID := strings.TrimSpace(user.GithubID)
	githubLogin := strings.TrimSpace(user.GithubUsername)

	if githubID == "" || githubLogin == "" {
		log.WithFields(f).Debugf("user lacks GitHub ID or username - skipping cache update (githubID=%q, login=%q)", githubID, githubLogin)
		return nil
	}

	affiliated := strings.TrimSpace(user.CompanyID) != ""

	emails := collectUserEmails(user)
	if len(emails) == 0 {
		log.WithFields(f).Debugf("no emails found for user (githubID=%s, login=%s) - nothing to cache", githubID, githubLogin)
		return nil
	}

	loginLower := strings.ToLower(githubLogin)

	for _, email := range emails {
		genKey := UserKey(githubID, loginLower, email)
		ModelUserCache.Set(genKey, user)

		projKey := ProjectUserKey(projectID, githubID, loginLower, email)
		ModelProjectUserCache.Set(projKey, user, true, affiliated)
	}

	log.WithFields(f).Infof("updated caches for user login=%s (GitHubID=%s), project=%s: marked as authorized for %d email(s)",
		loginLower, githubID, projectID, len(emails))

	return nil
}

// collectUserEmails returns a de-duplicated, lowercased list of the user's emails.
func collectUserEmails(u *models.User) []string {
	uniq := make(map[string]struct{}, 4)
	add := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" {
			uniq[s] = struct{}{}
		}
	}

	add(string(u.LfEmail))

	for _, em := range u.Emails {
		add(em)
	}

	out := make([]string, 0, len(uniq))
	for e := range uniq {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
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

// getCommentBody mirrors the Python get_comment_body behavior.
//
//nolint:gocyclo // complexity is acceptable for now
func getCommentBody(repositoryType, signURL string, signed, missing []*UserCommitSummary, anyMissing bool) string {
	f := logrus.Fields{
		"functionName":   "github.github_repository.getCommentBody",
		"repositoryType": repositoryType,
		"signURL":        signURL,
	}
	failed := ":x:"
	success := ":white_check_mark:"

	var committersComment strings.Builder
	text := ""

	numSigned := len(signed)
	numMissing := len(missing)

	// Start of the HTML list
	if numSigned > 0 || numMissing > 0 {
		committersComment.WriteString("<ul>")
	}

	// ---------- Signed section (group by author) ----------
	if numSigned > 0 {
		committers := make(map[string][]*UserCommitSummary, numSigned)
		for _, ucs := range signed {
			var authorInfo string
			if ucs != nil && ucs.IsValid() {
				authorInfo = ucs.getUserInfo(false)
			} else {
				authorInfo = unknown
			}
			committers[authorInfo] = append(committers[authorInfo], ucs)
		}

		// sort keys for stable output
		keys := make([]string, 0, len(committers))
		for k := range committers {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, authorInfo := range keys {
			summaries := committers[authorInfo]
			shas := make([]string, 0, len(summaries))
			for _, s := range summaries {
				if s != nil {
					shas = append(shas, s.SHA)
				}
			}
			log.WithFields(f).Debugf("SHAs for signed users: %v", shas)
			committersComment.WriteString(
				fmt.Sprintf("<li>%s %s (%s)</li>", success, authorInfo, strings.Join(shas, ", ")),
			)
		}
	}

	// ---------- Missing section (group by author) ----------
	if numMissing > 0 {
		supportURL := "https://jira.linuxfoundation.org/servicedesk/customer/portal/4"
		missingIDHelpURL := "https://linuxfoundation.atlassian.net/wiki/spaces/LP/pages/160923756/Missing+ID+on+Commit+but+I+have+an+agreement+on+file"
		githubHelpURL := "https://help.github.com/en/github/committing-changes-to-your-project/why-are-my-commits-linked-to-the-wrong-user"

		committers := make(map[string][]*UserCommitSummary, numMissing)
		for _, ucs := range missing {
			var authorInfo string
			if ucs != nil && ucs.IsValid() {
				authorInfo = ucs.getUserInfo(true)
				if strings.TrimSpace(authorInfo) == "" {
					authorInfo = unknown
				}
			} else {
				authorInfo = unknown
			}
			committers[authorInfo] = append(committers[authorInfo], ucs)
		}

		// sort keys for stable output
		keys := make([]string, 0, len(committers))
		for k := range committers {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, authorInfo := range keys {
			summaries := committers[authorInfo]
			if authorInfo == unknown {
				shas := make([]string, 0, len(summaries))
				for _, s := range summaries {
					if s != nil {
						shas = append(shas, s.SHA)
					}
				}
				committersComment.WriteString(fmt.Sprintf(
					"<li> %s The email address for the commit (%s) is not linked to the GitHub account, preventing the EasyCLA check. "+
						"Consult <a href='%s' target='_blank'>this Help Article</a> and "+
						"<a href='%s' target='_blank'>GitHub Help</a> to resolve. "+
						"(To view the commit's email address, add .patch at the end of this PR page's URL.) "+
						"For further assistance with EasyCLA, "+
						"<a href='%s' target='_blank'>please submit a support request ticket</a>.</li>",
					failed, strings.Join(shas, ", "), missingIDHelpURL, githubHelpURL, supportURL,
				))
				continue
			}

			missingAffiliations := make([]*UserCommitSummary, 0, len(summaries))
			for _, s := range summaries {
				if s != nil && !s.Affiliated && s.Authorized {
					missingAffiliations = append(missingAffiliations, s)
				}
			}
			if len(missingAffiliations) > 0 {
				shas := make([]string, 0, len(missingAffiliations))
				for _, s := range missingAffiliations {
					if s != nil {
						shas = append(shas, s.SHA)
					}
				}
				log.WithFields(f).Debugf("SHAs for users with missing company affiliations: %v", shas)
				committersComment.WriteString(fmt.Sprintf(
					`<li>%s %s (%s). This user is authorized, but they must confirm their affiliation with their company. `+
						`Start the authorization process <a href='%s' target='_blank'> by clicking here</a>, `+
						`click "Corporate", select the appropriate company from the list, then confirm your affiliation on the page that appears. `+
						`For further assistance with EasyCLA, <a href='%s' target='_blank'>please submit a support request ticket</a>.</li>`,
					failed, authorInfo, strings.Join(shas, ", "),
					signURL, supportURL,
				))
			} else {
				shas := make([]string, 0, len(summaries))
				for _, s := range summaries {
					if s != nil {
						shas = append(shas, s.SHA)
					}
				}
				committersComment.WriteString(fmt.Sprintf(
					`<li><a href='%s' target='_blank'>%s</a> - %s. The commit (%s) is not authorized under a signed CLA. `+
						`<a href='%s' target='_blank'>Please click here to be authorized</a>. `+
						`For further assistance with EasyCLA, <a href='%s' target='_blank'>please submit a support request ticket</a>.</li>`,
					signURL, failed, authorInfo, strings.Join(shas, ", "),
					signURL, supportURL,
				))
			}
		}
	}

	// End of list
	if numSigned > 0 || numMissing > 0 {
		committersComment.WriteString("</ul>")
	}

	// Python has a Date Modified footer, but that causes churn; intentionally omitted.

	// Success note if everyone is signed
	if numSigned > 0 && numMissing == 0 {
		text = "The committers listed above are authorized under a signed CLA."
	}

	// Missing co-authors notice
	if anyMissing {
		committersComment.WriteString(strings.ReplaceAll(MissingCoAuthorsMessage, "|", "`"))
		log.WithFields(f).Info("some co-authors are missing for this PR, added the missing co-author message")
	}

	// Python returns: text + committers_comment
	return text + committersComment.String()
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
		return fmt.Sprintf(`<a href="%s"><img src="%s" alt="%s" align="left" height="28" width="328" ></a>`, badgeHyperLink, badgeURL, alt)
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

	text = fmt.Sprintf(`<a href="%s"><img src="%s" alt="%s" align="left" height="28" width="328" ></a>`, badgeHyperLink, badgeURL, alt)
	return fmt.Sprintf("%s<br/>", text)
}

func getFullSignURL(repositoryType, installationID, githubRepositoryID, pullRequestID, apiBaseURL string) string {
	return fmt.Sprintf("%s/v2/repository-provider/%s/sign/%s/%s/%s/#/?version=2", apiBaseURL, repositoryType, installationID, githubRepositoryID, pullRequestID)
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
