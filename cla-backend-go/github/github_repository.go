// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
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
	ListCommitsParallelLimit    = 4
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

Alternatively, if the co-author should not be included, remove the |Co-authored-by:| line from the commit message.

Please update your commit message(s) by doing |git commit --amend| and then |git push [--force]| and then request re-running CLA check via commenting on this pull request:

|||
/easycla
|||

`

const (
	unknown                  = "Unknown"
	failureState             = "failure"
	successState             = "success"
	svgVersion               = "?v=2"
	NegativeCacheTTL         = 2 * time.Minute  // Used for negative caching of missing/not-signed users
	ProjectCacheTTL          = 15 * time.Minute // Used for per-project caching of signed users
	coAuthorNegativeCacheTTL = 15 * time.Minute
	MaxCommentLength         = 0xff00 // 65520 characters - leave some buffer under 64KB limit
)

type gqlError struct {
	Message    string         `json:"message"`
	Type       string         `json:"type,omitempty"` // sometimes "RATE_LIMITED"
	Path       []interface{}  `json:"path,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

type GraphQLError struct {
	Errs []gqlError
}

type compareCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commit"`
	Author *github.User `json:"author"`
}

type compareResponse struct {
	Commits []*compareCommit `json:"commits"`
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func buildCompareAuthor(c *compareCommit) *github.User {
	var out github.User
	if c != nil && c.Author != nil {
		out = *c.Author
	}
	if c == nil {
		return &out
	}

	name := firstNonEmpty(c.Commit.Author.Name, out.GetName(), out.GetLogin())
	email := firstNonEmpty(out.GetEmail(), c.Commit.Author.Email)

	if name != "" {
		out.Name = github.String(name)
	}
	if email != "" {
		out.Email = github.String(email)
	}

	return &out
}

func compareRetrySleep(attempt int) {
	switch attempt {
	case 1:
		return
	case 2:
		time.Sleep(1 * time.Second)
	case 3:
		time.Sleep(5 * time.Second)
	case 4:
		time.Sleep(15 * time.Second)
	default:
		time.Sleep(30 * time.Second)
	}
}

func comparePayloadToRepositoryCommits(payload *compareResponse) []*github.RepositoryCommit {
	if payload == nil || len(payload.Commits) == 0 {
		return nil
	}
	out := make([]*github.RepositoryCommit, 0, len(payload.Commits))
	for _, c := range payload.Commits {
		if c == nil || strings.TrimSpace(c.SHA) == "" {
			continue
		}

		rc := &github.RepositoryCommit{
			SHA:    github.String(c.SHA),
			Author: buildCompareAuthor(c),
			Commit: &github.Commit{
				Message: github.String(c.Commit.Message),
				Author:  &github.CommitAuthor{},
			},
		}

		if name := strings.TrimSpace(c.Commit.Author.Name); name != "" {
			rc.Commit.Author.Name = github.String(name)
		}
		if email := strings.TrimSpace(c.Commit.Author.Email); email != "" {
			rc.Commit.Author.Email = github.String(email)
		}

		out = append(out, rc)
	}
	return out
}

func fetchComparePage(
	ctx context.Context,
	client *github.Client,
	owner, repo, baseSHA, headSHA string,
	page, perPage, pullRequestID int,
) ([]*github.RepositoryCommit, *github.Response, error) {
	path := fmt.Sprintf("repos/%s/%s/compare/%s...%s", owner, repo, baseSHA, headSHA)
	var (
		payload compareResponse
		resp    *github.Response
		reqErr  error
	)
	for attempt := 1; attempt <= 6; attempt++ {
		req, err := client.NewRequest("GET", path, nil)
		if err != nil {
			return nil, nil, err
		}

		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Accept", "application/vnd.github+json")

		payload = compareResponse{}
		resp, reqErr = client.Do(ctx, req, &payload)
		if reqErr == nil {
			return comparePayloadToRepositoryCommits(&payload), resp, nil
		}
		if attempt >= 6 {
			return nil, resp, reqErr
		}
		log.WithFields(logrus.Fields{
			"functionName":  "github.github_repository.fetchComparePage",
			"pullRequestID": pullRequestID,
			"owner":         owner,
			"repo":          repo,
			"page":          page,
			"attempt":       attempt,
		}).WithError(reqErr).Warn("compare request failed, retrying")
		compareRetrySleep(attempt)
	}
	return nil, nil, fmt.Errorf("unreachable compare retry path")
}

func ListPullRequestCommitsCompare(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	pullRequestID int,
) ([]*github.RepositoryCommit, error) {
	pr, _, err := client.PullRequests.Get(ctx, owner, repo, pullRequestID)
	if err != nil {
		return nil, err
	}

	baseSHA := pr.GetBase().GetSHA()
	headSHA := pr.GetHead().GetSHA()
	if baseSHA == "" || headSHA == "" {
		return nil, fmt.Errorf("missing base/head SHA for %s/%s PR #%d", owner, repo, pullRequestID)
	}
	perPage := 100
	firstPageCommits, firstResp, err := fetchComparePage(ctx, client, owner, repo, baseSHA, headSHA, 1, perPage, pullRequestID)

	if err != nil {
		return nil, err
	}
	if firstResp == nil || firstResp.NextPage == 0 {
		return firstPageCommits, nil
	}

	limit := ListCommitsParallelLimit
	if limit < 1 {
		limit = 1
	}

	// Fallback to sequential paging if we do not know the last page or parallelism is disabled.
	if limit == 1 || firstResp.LastPage <= 1 {
		allCommits := append([]*github.RepositoryCommit{}, firstPageCommits...)
		nextPage := firstResp.NextPage
		for nextPage != 0 {
			pageCommits, pageResp, err := fetchComparePage(ctx, client, owner, repo, baseSHA, headSHA, nextPage, perPage, pullRequestID)
			if err != nil {
				return nil, err
			}
			allCommits = append(allCommits, pageCommits...)
			if pageResp == nil || pageResp.NextPage == 0 {
				break
			}
			nextPage = pageResp.NextPage
		}
		return allCommits, nil
	}

	lastPage := firstResp.LastPage
	pageResults := make([][]*github.RepositoryCommit, lastPage+1)
	pageResults[1] = firstPageCommits

	var wg sync.WaitGroup
	sem := make(chan struct{}, limit)
	errCh := make(chan error, 1)

	for page := 2; page <= lastPage; page++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(page int) {
			defer wg.Done()
			defer func() { <-sem }()

			pageCommits, _, err := fetchComparePage(ctx, client, owner, repo, baseSHA, headSHA, page, perPage, pullRequestID)
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			pageResults[page] = pageCommits
		}(page)
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return nil, err
	default:
	}

	allCommits := make([]*github.RepositoryCommit, 0, len(firstPageCommits)+(lastPage-1)*perPage)
	for page := 1; page <= lastPage; page++ {
		allCommits = append(allCommits, pageResults[page]...)
	}
	return allCommits, nil
}

func (e *GraphQLError) Error() string {
	if len(e.Errs) == 0 {
		return "graphql: unknown error"
	}
	msg := "graphql: "
	for i, ge := range e.Errs {
		msg += fmt.Sprintf("#%d: %s (type=%s path=%v)", i+1, ge.Message, ge.Type, ge.Path)
		if i < len(e.Errs)-1 {
			msg += "; "
		}
	}
	return msg
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

// InvalidateByUser removes every entry whose (id, login) prefix matches,
// regardless of the email component. The login is lowercased internally to
// match how UserKey stores it, so callers may pass either the original
// GitHub login or a pre-lowercased form. Used after a signature event to
// drop stale entries keyed on commit-email shapes the caller cannot
// enumerate (e.g. the GitHub noreply form emitted when a user has email
// privacy enabled).
func (c *UserCache) InvalidateByUser(id, login string) int {
	loginLower := strings.ToLower(login)
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for k := range c.data {
		if k[0] == id && k[1] == loginLower {
			delete(c.data, k)
			n++
		}
	}
	return n
}

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

// InvalidateByProject removes every entry for the given project, regardless
// of user. Used after an approval-list mutation (UpdateApprovalList), since
// any cached signed/authorized decision under that project may now be
// stale: users newly added to email/domain/org/github approvals must flip
// red→green, and users removed must flip green→red. Cache misses for
// affected webhooks are then resolved against fresh DDB state on next read.
func (c *ProjectUserCache) InvalidateByProject(projectID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for k := range c.data {
		if k[0] == projectID {
			delete(c.data, k)
			n++
		}
	}
	return n
}

// InvalidateByUser removes every entry whose (projectID, id, login) prefix
// matches, regardless of the email component. The login is lowercased
// internally to match how ProjectUserKey stores it, so callers may pass
// either the original GitHub login or a pre-lowercased form. Used after a
// signature event to drop stale per-project entries keyed on commit-email
// shapes the caller cannot enumerate (e.g. the GitHub noreply form emitted
// when a user has email privacy enabled).
func (c *ProjectUserCache) InvalidateByUser(projectID, id, login string) int {
	loginLower := strings.ToLower(login)
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for k := range c.data {
		if k[0] == projectID && k[1] == id && k[2] == loginLower {
			delete(c.data, k)
			n++
		}
	}
	return n
}

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

	if u.CommitAuthor != nil {
		log.WithFields(f).Debugf("author: login=%s id=%s email=%s",
			utils.StringValue(u.CommitAuthor.Login),
			fmt.Sprintf("%d", utils.Int64Value(u.CommitAuthor.ID)),
			utils.StringValue(u.CommitAuthor.Email))
	}

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
		// Soft failure: caller (GetCoAuthorCommits) falls back to GetUsersByLFEmail/GetUsersByEmail.
		// Most common cause is the GitHub Search API rate limit (30/min/installation), which is expected during PR bursts.
		log.WithFields(f).WithError(err).Warnf("Could not search GitHub user by email (falling back to DB lookup): %s", email)
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
	if len(coAuthors) > 0 {
		log.WithFields(f).Debugf("co-authors found: %s", coAuthors)
	}
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
		// negative cache for GitHub user not found
		GithubUserCache.SetWithTTL(cacheKey, user, coAuthorNegativeCacheTTL)
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
	log.WithFields(f).Debugf("per-project cache: %+v -> (user=%t, authorized=%v, affiliated=%v, hit=%v)",
		projectCacheKey, cachedUser != nil, authorized, affiliated, ok)
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
	log.WithFields(f).Debugf("general cache: %+v -> (user=%t, hit=%v)", cacheKey, cachedUser != nil, ok)
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
	prCommitCount := 0
	if client, clientErr := NewGithubAppClient(installationID); clientErr == nil {
		if pr, _, prErr := client.PullRequests.Get(ctx, owner, repo, pullRequestID); prErr == nil && pr != nil {
			prCommitCount = pr.GetCommits()
		}
	}

	summaries, anyMissing, err := GetPullRequestCommitAuthorsCompare(
		ctx, usersService, installationID, pullRequestID, owner, repo, withCoAuthors,
	)
	if err == nil {
		return summaries, anyMissing, nil
	}
	if prCommitCount == 0 {
		log.WithFields(logrus.Fields{
			"functionName":  "github.github_repository.GetPullRequestCommitAuthors",
			"pullRequestID": pullRequestID,
			"owner":         owner,
			"repo":          repo,
		}).WithError(err).Warn("compare-based commit enumeration failed and PR commit count is unavailable; refusing unsafe REST fallback")
		return nil, false, err
	}

	if prCommitCount > 250 {
		log.WithFields(logrus.Fields{
			"functionName":  "github.github_repository.GetPullRequestCommitAuthors",
			"pullRequestID": pullRequestID,
			"owner":         owner,
			"repo":          repo,
			"commits":       prCommitCount,
		}).WithError(err).Warn("compare-based commit enumeration failed; refusing unsafe REST fallback for large PR")
		return nil, false, err
	}

	log.WithFields(logrus.Fields{
		"functionName":  "github.github_repository.GetPullRequestCommitAuthors",
		"pullRequestID": pullRequestID,
		"owner":         owner,
		"repo":          repo,
	}).WithError(err).Warn("compare-based commit enumeration failed, falling back to REST PR commits")

	return GetPullRequestCommitAuthorsREST(ctx, usersService, installationID, pullRequestID, owner, repo, withCoAuthors)
}

func GetPullRequestCommitAuthorsCompare(
	ctx context.Context,
	usersService users.Service,
	installationID int64,
	pullRequestID int,
	owner, repo string,
	withCoAuthors bool,
) ([]*UserCommitSummary, bool, error) {
	const fn = "github.github_repository.GetPullRequestCommitAuthorsCompare"
	f := logrus.Fields{
		"functionName":  fn,
		"pullRequestID": pullRequestID,
		"owner":         owner,
		"repo":          repo,
		"withCoAuthors": withCoAuthors,
	}

	client, err := NewGithubAppClient(installationID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to create Github client")
		return nil, false, err
	}

	commits, err := ListPullRequestCommitsCompare(ctx, client, owner, repo, pullRequestID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("compare request failed")
		return nil, false, err
	}

	userCommitSummary := make([]*UserCommitSummary, 0, len(commits))
	var (
		mu         sync.Mutex
		wg         sync.WaitGroup
		anyMissing atomic.Bool
	)
	maxConc := runtime.NumCPU()
	if maxConc < 1 {
		maxConc = 1
	}
	sem := make(chan struct{}, maxConc)

	for _, commit := range commits {
		if commit == nil {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(commit *github.RepositoryCommit) {
			defer wg.Done()
			defer func() { <-sem }()

			if commit.Author == nil {
				commit.Author = &github.User{}
			}

			name, email := "", ""
			if commit.Commit != nil && commit.Commit.Author != nil {
				name = strings.TrimSpace(utils.StringValue(commit.Commit.Author.Name))
				email = strings.TrimSpace(utils.StringValue(commit.Commit.Author.Email))

				if name != "" {
					if commit.Author.Name == nil || strings.TrimSpace(utils.StringValue(commit.Author.Name)) == "" {
						n := name
						commit.Author.Name = &n
					}
				}
				if email != "" {
					if commit.Author.Email == nil || strings.TrimSpace(utils.StringValue(commit.Author.Email)) == "" {
						e := email
						commit.Author.Email = &e
					}
				}
			}

			mu.Lock()
			userCommitSummary = append(userCommitSummary, &UserCommitSummary{
				SHA:          utils.StringValue(commit.SHA),
				CommitAuthor: commit.Author,
				Affiliated:   false,
				Authorized:   false,
			})
			mu.Unlock()

			if withCoAuthors {
				if ExpandWithCoAuthors(ctx, client, usersService, commit, pullRequestID, installationID, &userCommitSummary, &mu) {
					anyMissing.Store(true)
				}
			}
		}(commit)
	}

	wg.Wait()
	return userCommitSummary, anyMissing.Load(), nil
}

//nolint:gocyclo // complexity is acceptable for now
func GetPullRequestCommitAuthorsREST(ctx context.Context, usersService users.Service, installationID int64, pullRequestID int, owner, repo string, withCoAuthors bool) ([]*UserCommitSummary, bool, error) {
	fn := "github.github_repository.GetPullRequestCommitAuthorsREST"
	f := logrus.Fields{
		"functionName":  fn,
		"pullRequestID": pullRequestID,
		"withCoAuthors": withCoAuthors,
	}
	var userCommitSummary []*UserCommitSummary
	var mu sync.Mutex

	client, err := NewGithubAppClient(installationID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to create Github client")
		return nil, false, err
	}

	anyMissing := false
	opts := &github.ListOptions{PerPage: 100}
	for {
		commits, resp, comErr := client.PullRequests.ListCommits(ctx, owner, repo, pullRequestID, opts)
		if comErr != nil {
			log.WithFields(f).WithError(comErr).Warnf("problem listing commits for repo: %s/%s pull request: %d", owner, repo, pullRequestID)
			return nil, false, comErr
		}
		if resp.StatusCode != http.StatusOK {
			msg := fmt.Sprintf("unexpected status code: %d - expected: %d", resp.StatusCode, http.StatusOK)
			log.WithFields(f).Warn(msg)
			return nil, false, errors.New(msg)
		}

		log.WithFields(f).Debugf("found %d commits for pull request: %d", len(commits), pullRequestID)
		for _, commit := range commits {
			log.WithFields(f).Debugf("loaded commit: %+v", commit)
			commitAuthor := ""
			if commit != nil && commit.Commit != nil && commit.Commit.Author != nil && commit.Commit.Author.Login != nil {
				log.WithFields(f).Debugf("commit.Commit.Author.Login: %s", utils.StringValue(commit.Commit.Author.Login))
				commitAuthor = utils.StringValue(commit.Commit.Author.Login)
			} else if commit != nil && commit.Author != nil && commit.Author.Login != nil {
				log.WithFields(f).Debugf("commit.Author.Login: %s", utils.StringValue(commit.Author.Login))
				commitAuthor = utils.StringValue(commit.Author.Login)
			}
			name, email := "", ""
			if commit != nil && commit.Commit != nil && commit.Commit.Author != nil {
				name = strings.TrimSpace(utils.StringValue(commit.Commit.Author.Name))
				email = strings.TrimSpace(utils.StringValue(commit.Commit.Author.Email))
				if (name != "" || email != "") && commit.Author == nil {
					commit.Author = &github.User{}
				}
				if name != "" && commit.Author != nil {
					if commit.Author.Name == nil || strings.TrimSpace(utils.StringValue(commit.Author.Name)) == "" {
						n := name
						commit.Author.Name = &n
					}
				}
				if email != "" && commit.Author != nil {
					if commit.Author.Email == nil || strings.TrimSpace(utils.StringValue(commit.Author.Email)) == "" {
						e := email
						commit.Author.Email = &e
					}
				}
			}
			log.WithFields(f).Debugf("commitAuthor: %s, name: %s, email: %s", commitAuthor, name, email)
			userCommitSummary = append(userCommitSummary, &UserCommitSummary{
				SHA:          utils.StringValue(commit.SHA),
				CommitAuthor: commit.Author,
				Affiliated:   false,
				Authorized:   false,
			})
			if withCoAuthors {
				missing := ExpandWithCoAuthors(ctx, client, usersService, commit, pullRequestID, installationID, &userCommitSummary, &mu)
				if !anyMissing && missing {
					anyMissing = true
				}
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Build distinct sets
	distinctSHAs := make(map[string]struct{})
	distinctIDs := make(map[int64]struct{})
	distinctLogins := make(map[string]struct{})
	distinctEmails := make(map[string]struct{})
	distinctNames := make(map[string]struct{})

	for _, s := range userCommitSummary {
		if s == nil {
			continue
		}
		if s.SHA != "" {
			distinctSHAs[s.SHA] = struct{}{}
		}
		if s.CommitAuthor != nil {
			if s.CommitAuthor.ID != nil {
				distinctIDs[*s.CommitAuthor.ID] = struct{}{}
			}
			if s.CommitAuthor.Login != nil && *s.CommitAuthor.Login != "" {
				distinctLogins[*s.CommitAuthor.Login] = struct{}{}
			}
			if s.CommitAuthor.Email != nil && *s.CommitAuthor.Email != "" {
				distinctEmails[*s.CommitAuthor.Email] = struct{}{}
			}
			if s.CommitAuthor.Name != nil && *s.CommitAuthor.Name != "" {
				distinctNames[*s.CommitAuthor.Name] = struct{}{}
			}
		}
	}

	log.WithFields(f).Debugf(
		"%s - PR: %d, total commit authors summaries found: %d, any missing: %v, distinct SHAs: %d, distinct author IDs: %d, logins: %d, emails: %d, names: %d",
		fn, pullRequestID, len(userCommitSummary), anyMissing,
		len(distinctSHAs), len(distinctIDs), len(distinctLogins), len(distinctEmails), len(distinctNames),
	)

	return userCommitSummary, anyMissing, nil
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
	log.WithFields(f).Debugf("editing comment %d on %s/%s PR #%d (old=%d bytes, new=%d bytes)",
		commentID, owner, repo, prNum, len(oldNorm), len(newNorm))
	_, _, err = client.Issues.EditComment(ctx, owner, repo, commentID, &github.IssueComment{Body: &newBody})
	if err != nil {
		return false, err
	}
	return true, nil
}

// Commit SHA for a PR (authoritative "last commit")
func GetPRCommitSHA(ctx context.Context, gh *github.Client, owner, repo string, prNumber int) (string, error) {
	pr, _, err := gh.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		f := logrus.Fields{
			"functionName":  "github.github_repository.GetPRCommitSHA",
			"owner":         owner,
			"repo":          repo,
			"pullRequestID": prNumber,
		}
		log.WithFields(f).WithError(err).Warn("cannot get PR commit SHA using PullRequests.Get, trying PullRequests.ListCommits")
		opts := &github.ListOptions{PerPage: 1}
		commits, resp, comErr := gh.PullRequests.ListCommits(ctx, owner, repo, prNumber, opts)
		if comErr != nil {
			log.WithFields(f).WithError(comErr).Warnf("problem listing commits for repo: %s/%s pull request: %d", owner, repo, prNumber)
			return "", comErr
		}
		if resp != nil && resp.LastPage > 1 {
			opts.Page = resp.LastPage
			commits, _, comErr = gh.PullRequests.ListCommits(ctx, owner, repo, prNumber, opts)
			if comErr != nil {
				log.WithFields(f).WithError(comErr).Warnf("problem listing commits for repo: %s/%s pull request: %d (last page)", owner, repo, prNumber)
				return "", comErr
			}
		}
		if len(commits) == 0 || commits[0].SHA == nil {
			return "", fmt.Errorf("missing commit SHA for %s/%s PR #%d (via ListCommits)", owner, repo, prNumber)
		}
		return *commits[0].SHA, nil
	}
	sha := ""
	if pr != nil && pr.Head != nil && pr.Head.SHA != nil {
		sha = *pr.Head.SHA
	}
	if sha == "" {
		return "", fmt.Errorf("missing commit SHA for %s/%s PR #%d", owner, repo, prNumber)
	}
	return sha, nil
}

func UpdatePullRequest(ctx context.Context, installationID int64, pullRequestID int, owner, repo string, repoID *int64, signed []*UserCommitSummary, missing []*UserCommitSummary, anyMissing bool, CLABaseAPIURL, CLALandingPage, CLALogoURL string) error {
	return updatePullRequest(ctx, installationID, pullRequestID, owner, repo, repoID, signed, missing, anyMissing, CLABaseAPIURL, CLALandingPage, CLALogoURL, utils.V2, false)
}

func UpdatePullRequestLegacyCompat(ctx context.Context, installationID int64, pullRequestID int, owner, repo string, repoID *int64, signed []*UserCommitSummary, missing []*UserCommitSummary, anyMissing bool, CLABaseAPIURL, CLALandingPage, CLALogoURL, projectVersion string) error {
	return updatePullRequest(ctx, installationID, pullRequestID, owner, repo, repoID, signed, missing, anyMissing, CLABaseAPIURL, CLALandingPage, CLALogoURL, projectVersion, true)
}

func updatePullRequest(ctx context.Context, installationID int64, pullRequestID int, owner, repo string, repoID *int64, signed []*UserCommitSummary, missing []*UserCommitSummary, anyMissing bool, CLABaseAPIURL, CLALandingPage, CLALogoURL, projectVersion string, legacyCheckRun bool) error {
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
		return succeedErr
	}

	var commitSHA string
	if legacyCheckRun {
		commitSHA, err = GetPRCommitSHA(ctx, client, owner, repo, pullRequestID)
		log.WithFields(f).Debugf("Got commit SHA for %s/%s PR %d: %s", owner, repo, pullRequestID, commitSHA)
		if err != nil {
			return err
		}
		if len(missing) > 0 {
			checkRunErr := createLegacyActionRequiredCheckRun(ctx, client, owner, repo, commitSHA, installationID, repoID, pullRequestID, missing, CLABaseAPIURL, projectVersion)
			if checkRunErr != nil {
				// Legacy Python logs check-run creation failures and continues with comment/status updates.
				log.WithFields(f).WithError(checkRunErr).Debugf("unable to create legacy CLA check run for PR: %d", pullRequestID)
			}
		}
	}

	body := assembleCLAComment(ctx, int(installationID), pullRequestID, repoID, signed, missing, anyMissing, CLABaseAPIURL, CLALogoURL, CLALandingPage, projectVersion)

	if len(missing) == 0 {
		// All contributors are passing
		if previouslyFailed {
			edited, err2 := EditIssueCommentIfChanged(ctx, client, owner, repo, pullRequestID, *comment.ID, body)
			if err2 != nil {
				log.WithFields(f).WithError(err2).Debug("unable to edit comment")
				return err2
			}
			if edited {
				log.WithFields(f).Infof("Updated CLA comment for PR %d (body changed).", pullRequestID)
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
				log.WithFields(f).Infof("Updated failing CLA comment for PR %d (body changed).", pullRequestID)
			} else {
				log.WithFields(f).Debugf("Failing CLA comment unchanged for PR %d, skipping edit.", pullRequestID)
			}
		} else if previouslySucceeded {
			// pass => fail transition; still avoid redundant edit
			failedBody := assembleCLAComment(ctx, int(installationID), pullRequestID, repoID, signed, missing, anyMissing, CLABaseAPIURL, CLALogoURL, CLALandingPage, projectVersion)
			edited, err2 := EditIssueCommentIfChanged(ctx, client, owner, repo, pullRequestID, *previousSucceededComment.ID, failedBody)
			if err2 != nil {
				log.WithFields(f).WithError(err2).Debug("unable to edit previous success comment")
				return err2
			}
			if edited {
				log.WithFields(f).Infof("Updated previously succeeded comment to failing for PR %d.", pullRequestID)
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
			log.WithFields(f).Infof("Created new failing CLA comment for PR %d.", pullRequestID)
		}
	}

	// Update/Create the status
	ctxName := "EasyCLA"
	if legacyCheckRun {
		ctxName = strings.TrimSpace(os.Getenv("GH_STATUS_CTX_NAME"))
		if ctxName == "" {
			ctxName = "communitybridge/cla"
		}
	}
	var statusBody string
	var state string
	var signURL string

	if len(missing) > 0 {
		state = failureState
		ctxName, statusBody = assembleCLAStatus(ctxName, false)
		signURL = getFullSignURL(strconv.Itoa(int(installationID)), strconv.Itoa(int(*repoID)), strconv.Itoa(pullRequestID), CLABaseAPIURL, projectVersion)
		log.WithFields(f).Infof("CLA gate decision PR %s/%s#%d: state=%s passed=%d missing=%d signing_url=%s",
			owner, repo, pullRequestID, state, len(signed), len(missing), signURL)
	} else if len(signed) > 0 {
		state = successState
		ctxName, statusBody = assembleCLAStatus(ctxName, true)
		signURL = appendProjectVersionToURL(fmt.Sprintf("%s/#/", CLALandingPage), projectVersion)
		log.WithFields(f).Infof("CLA gate decision PR %s/%s#%d: state=%s passed=%d missing=%d signing_url=%s",
			owner, repo, pullRequestID, state, len(signed), len(missing), signURL)

	} else {
		state = failureState
		ctxName, statusBody = assembleCLAStatus(ctxName, false)
		signURL = getFullSignURL(strconv.Itoa(int(installationID)), strconv.Itoa(int(*repoID)), strconv.Itoa(pullRequestID), CLABaseAPIURL, projectVersion)
		log.WithFields(f).Warnf("CLA gate decision PR %s/%s#%d: state=%s passed=0 missing=0 (no committers identified) signing_url=%s",
			owner, repo, pullRequestID, state, signURL)
	}

	status := Status{
		State:       &state,
		TargetURL:   &signURL,
		Context:     &ctxName,
		Description: &statusBody,
	}

	log.WithFields(f).Debugf("Creating status: %+v", status)
	if commitSHA == "" {
		commitSHA, err = GetPRCommitSHA(ctx, client, owner, repo, pullRequestID)
		log.WithFields(f).Debugf("Got commit SHA for %s/%s PR %d: %s", owner, repo, pullRequestID, commitSHA)
		if err != nil {
			return err
		}
	}

	_, _, err = CreateStatus(ctx, client, owner, repo, commitSHA, &status)
	if err != nil {
		log.WithFields(f).WithError(err).Debugf("unable to create status on %s", commitSHA)
		return err
	}
	log.WithFields(f).Debugf("Created '%s' status commit SHA for %s/%s PR %d: %s", *status.State, owner, repo, pullRequestID, commitSHA)

	return nil
}

const legacyInvalidAuthorHelpURL = "https://help.github.com/en/github/committing-changes-to-your-project/why-are-my-commits-linked-to-the-wrong-user"

func createLegacyActionRequiredCheckRun(ctx context.Context, client *github.Client, owner, repo, commitSHA string, installationID int64, repoID *int64, pullRequestID int, missing []*UserCommitSummary, apiBaseURL, projectVersion string) error {
	if client == nil || repoID == nil || commitSHA == "" || len(missing) == 0 {
		return nil
	}

	signURL := ""
	seenRenderKeys := make(map[string]struct{}, len(missing))
	var text strings.Builder
	for _, userSummary := range missing {
		if userSummary == nil {
			continue
		}
		if !userSummary.IsValid() {
			signURL = legacyInvalidAuthorHelpURL
		} else {
			signURL = getFullSignURL(strconv.Itoa(int(installationID)), strconv.Itoa(int(*repoID)), strconv.Itoa(pullRequestID), apiBaseURL, projectVersion)
		}

		renderKey := legacyCheckRunRenderKey(userSummary)
		if _, ok := seenRenderKeys[renderKey]; ok {
			continue
		}
		seenRenderKeys[renderKey] = struct{}{}
		text.WriteString(userSummary.GetDisplayText(true))
	}

	payload := map[string]interface{}{
		"name":        "CLA check",
		"head_sha":    commitSHA,
		"status":      "completed",
		"conclusion":  "action_required",
		"details_url": signURL,
		"output": map[string]string{
			"title":   "EasyCLA: Signed CLA not found",
			"summary": "One or more committers are not authorized under a signed CLA.",
			"text":    text.String(),
		},
	}

	req, err := client.NewRequest("POST", fmt.Sprintf("repos/%s/%s/check-runs", owner, repo), payload)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github.antiope-preview+json")
	resp, err := client.Do(ctx, req, nil)
	if err != nil && resp != nil && resp.Response != nil {
		log.WithFields(logrus.Fields{
			"functionName":            "github.github_repository.createLegacyActionRequiredCheckRun",
			"owner":                   owner,
			"repo":                    repo,
			"installationID":          installationID,
			"pullRequestID":           pullRequestID,
			"statusCode":              resp.Response.StatusCode,
			"x-accepted-github-perms": resp.Response.Header.Get("X-Accepted-GitHub-Permissions"),
			"x-oauth-scopes":          resp.Response.Header.Get("X-OAuth-Scopes"),
			"x-github-request-id":     resp.Response.Header.Get("X-GitHub-Request-Id"),
		}).Warnf("check-run create failed; see X-Accepted-GitHub-Permissions for required GitHub App permissions")
	}
	return err
}

func legacyCheckRunRenderKey(userSummary *UserCommitSummary) string {
	if userSummary == nil || userSummary.CommitAuthor == nil {
		return ""
	}

	authorID := userSummary.GetCommitAuthorID()
	authorLogin := strings.ToLower(strings.TrimSpace(utils.StringValue(userSummary.CommitAuthor.Login)))
	authorEmail := strings.ToLower(strings.TrimSpace(userSummary.GetCommitAuthorEmail()))
	if authorID != "" || authorLogin != "" || authorEmail != "" {
		return strings.Join([]string{authorID, authorLogin, authorEmail, ""}, "|")
	}

	authorName := strings.ToLower(strings.TrimSpace(utils.StringValue(userSummary.CommitAuthor.Name)))
	return strings.Join([]string{"", "", "", authorName}, "|")
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
	loginLower := strings.ToLower(githubLogin)

	// Wipe every cache entry for this (projectID, githubID, login) tuple
	// regardless of email. The pre-signature webhook may have stored a
	// negative entry keyed on a commit-email shape we cannot enumerate from
	// the user record — most commonly the GitHub noreply form
	// "<id>+<login>@users.noreply.github.com" when the user has email
	// privacy enabled. Without this wipe, the stale negative entry survives
	// the signature callback and the PR stays red until NegativeCacheTTL
	// (2m) expires or the next webhook lands on a different Lambda.
	projInvalidated := ModelProjectUserCache.InvalidateByUser(projectID, githubID, loginLower)
	userInvalidated := ModelUserCache.InvalidateByUser(githubID, loginLower)

	// Pre-populate positive entries for the user's known emails so a webhook
	// whose commit-email matches one of them gets an immediate cache hit.
	// Webhooks for unknown email shapes (e.g. noreply) will fall through to
	// the slow path, find the freshly-recorded signature, and cache a fresh
	// positive entry — no stale negative left to mislead them.
	emails := collectUserEmails(user)
	for _, email := range emails {
		genKey := UserKey(githubID, loginLower, email)
		ModelUserCache.Set(genKey, user)

		projKey := ProjectUserKey(projectID, githubID, loginLower, email)
		ModelProjectUserCache.Set(projKey, user, true, affiliated)
	}

	log.WithFields(f).Infof("updated caches for user login=%s (GitHubID=%s), project=%s: invalidated %d project + %d user stale entries; pre-populated %d authorized email(s)",
		loginLower, githubID, projectID, projInvalidated, userInvalidated, len(emails))

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

func assembleCLAComment(ctx context.Context, installationID, pullRequestID int, repositoryID *int64, signed, missing []*UserCommitSummary, anyMissing bool, apiBaseURL, CLALogoURL, CLALandingPage, projectVersion string) string {
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

	log.WithFields(f).Debug("Building CLAComment body.")
	signURL := getFullSignURL(strconv.Itoa(installationID), strconv.Itoa(int(*repositoryID)), strconv.Itoa(pullRequestID), apiBaseURL, projectVersion)
	commentBody := getCommentBody(repositoryType, signURL, signed, missing, anyMissing)
	allSigned := len(missing) == 0
	badge := getCommentBadge(allSigned, signURL, missingID, false, CLALandingPage, CLALogoURL, projectVersion)
	body := fmt.Sprintf("%s<br >%s", badge, commentBody)
	if len(body) > MaxCommentLength {
		body = TrimComment(body, 40, 20, 20, "…")
		log.WithFields(f).Debugf("comment trimmed to (%d): %s", len(body), body)
	}
	return body
}

// TrimComment collapses any "(sha1, sha2, ...)" group where all tokens look like SHAs (7–40 hex).
// If a group has > maxItems, it keeps the first `head`, then an ellipsis, then the last `tail`.
func TrimComment(html string, maxItems, head, tail int, ellipsis string) string {
	if head+tail > maxItems {
		if maxItems > head {
			tail = maxItems - head
		} else {
			tail = 0
		}
	}
	// Match any parenthesized group without nested parens.
	re := regexp.MustCompile(`\(([^()]*)\)`)
	return re.ReplaceAllStringFunc(html, func(group string) string {
		// Strip the surrounding parentheses.
		inner := group[1 : len(group)-1]
		parts := splitAndTrimByComma(inner)
		if len(parts) == 0 {
			return group
		}
		// Ensure every token looks like a SHA.
		for _, p := range parts {
			if !isSHA(p) {
				return group
			}
		}
		// Collapse if too many.
		if len(parts) > maxItems {
			var out []string
			if head > 0 && head < len(parts) {
				out = append(out, parts[:head]...)
			}
			out = append(out, ellipsis)
			if tail > 0 && tail < len(parts) {
				out = append(out, parts[len(parts)-tail:]...)
			}
			return "(" + strings.Join(out, ", ") + ")"
		}
		return group
	})
}

func splitAndTrimByComma(s string) []string {
	if s == "" {
		return nil
	}
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		t := strings.TrimSpace(r)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func isSHA(s string) bool {
	// 7..40 hex chars
	if n := len(s); n < 7 || n > 40 {
		return false
	}
	for _, r := range s {
		if !isHex(r) {
			return false
		}
	}
	return true
}

func isHex(r rune) bool {
	return (r >= '0' && r <= '9') ||
		(r >= 'a' && r <= 'f') ||
		(r >= 'A' && r <= 'F')
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
		supportURL := "https://easycla.lfx.linuxfoundation.org/"
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
						"<a href='%s' target='_blank'>please visit our EasyCLA portal</a> and chat with our support bot.</li>",
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
						`For further assistance with EasyCLA, <a href='%s' target='_blank'>please visit our EasyCLA portal</a> and chat with our support bot.</li>`,
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
						`For further assistance with EasyCLA, <a href='%s' target='_blank'>please visit our EasyCLA portal</a> and chat with our support bot.</li>`,
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

func getCommentBadge(allSigned bool, signURL string, missingUserId, managerApproved bool, CLALandingPage, CLALogoURL, projectVersion string) string {
	var alt string
	var text string
	var badgeHyperLink string
	var badgeURL string

	if allSigned {
		badgeURL = fmt.Sprintf("%s/cla-signed.svg%s", CLALogoURL, svgVersion)
		badgeHyperLink = appendProjectVersionToURL(fmt.Sprintf("%s/#/", CLALandingPage), projectVersion)
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

func getFullSignURL(installationID, githubRepositoryID, pullRequestID, apiBaseURL, projectVersion string) string {
	baseURL := fmt.Sprintf("%s/v2/repository-provider/github/sign/%s/%s/%s/#/", apiBaseURL, installationID, githubRepositoryID, pullRequestID)
	return appendProjectVersionToURL(baseURL, projectVersion)
}

func appendProjectVersionToURL(address, projectVersion string) string {
	version := "1"
	if strings.TrimSpace(projectVersion) == utils.V2 {
		version = "2"
	}
	if strings.Contains(address, "version=") {
		return address
	}
	separator := "?"
	if strings.Contains(address, "?") {
		separator = "&"
	}
	return address + separator + "version=" + version
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

	if installationID <= 0 {
		err := errors.New("invalid installation ID")
		log.WithFields(f).WithError(err).Warn("invalid installation ID")
		return "", err
	}
	if repositoryID <= 0 {
		err := errors.New("invalid repository ID")
		log.WithFields(f).WithError(err).Warn("invalid repository ID")
		return "", err
	}
	if pullRequestID <= 0 {
		err := errors.New("invalid pull request ID")
		log.WithFields(f).WithError(err).Warn("invalid pull request ID")
		return "", err
	}

	client, err := NewGithubAppClient(installationID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to create Github client")
		return "", err
	}

	log.WithFields(f).Debugf("getting github repository by id: %d", repositoryID)
	repo, resp, err := client.Repositories.GetByID(ctx, repositoryID)
	if err != nil {
		if ok, wrapped := CheckAndWrapForKnownErrors(resp, err); ok {
			log.WithFields(f).WithError(wrapped).Warnf("unable to get repository by ID: %d", repositoryID)
			return "", wrapped
		}
		log.WithFields(f).WithError(err).Warnf("unable to get repository by ID: %d", repositoryID)
		return "", err
	}
	if repo == nil {
		err = fmt.Errorf("missing repository for repository ID %d", repositoryID)
		log.WithFields(f).WithError(err).Warn("invalid repository metadata")
		return "", err
	}

	owner := repo.GetOwner().GetLogin()
	name := repo.GetName()
	if owner == "" || name == "" {
		err = fmt.Errorf("invalid repository owner/name for repository ID %d", repositoryID)
		log.WithFields(f).WithError(err).Warn("invalid repository metadata")
		return "", err
	}

	log.WithFields(f).Debugf("getting pull request by id: %d", pullRequestID)
	pullRequest, resp, err := client.PullRequests.Get(ctx, owner, name, pullRequestID)
	if err != nil {
		if ok, wrapped := CheckAndWrapForKnownErrors(resp, err); ok {
			log.WithFields(f).WithError(wrapped).Warnf("unable to get pull request by ID: %d", pullRequestID)
			return "", wrapped
		}
		log.WithFields(f).WithError(err).Warnf("unable to get pull request by ID: %d", pullRequestID)
		return "", err
	}
	if pullRequest == nil {
		err := fmt.Errorf("missing pull request %d/%s/%s", pullRequestID, owner, name)
		log.WithFields(f).WithError(err).Warn("invalid pull request metadata")
		return "", err
	}

	htmlURL := pullRequest.GetHTMLURL()
	if htmlURL == "" {
		err := fmt.Errorf("missing html url for pull request %d/%s/%s", pullRequestID, owner, name)
		log.WithFields(f).WithError(err).Warn("invalid pull request metadata")
		return "", err
	}

	log.WithFields(f).Debugf("returning pull request html url: %s", htmlURL)

	return htmlURL, nil
}
