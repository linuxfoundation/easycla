// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/shurcooL/githubv4"

	"github.com/google/go-github/v37/github"
	"golang.org/x/oauth2"
)

var (
	// ErrAccessDenied is returned whenever github return 403 or 401
	ErrAccessDenied = errors.New("access denied")
	// ErrRateLimited is returned when github detects rate limit abuse
	ErrRateLimited = errors.New("rate limit")
)

func isGithub403(resp *github.Response, err error) (bool, error) {
	if resp == nil {
		return false, err
	}

	statusCode := resp.StatusCode
	if statusCode == 403 || statusCode == 401 {
		var msg string
		if gErr, ok := err.(*github.ErrorResponse); ok {
			msg = gErr.Message
		}
		if msg != "" {
			return true, fmt.Errorf("%s : %w", msg, ErrAccessDenied)
		}
		return true, ErrAccessDenied
	}

	return false, nil
}

func isGithubRateLimit(err error) (bool, error) {
	if gErr, ok := err.(*github.RateLimitError); ok {
		return true, fmt.Errorf("%s : %w", gErr.Message, ErrRateLimited)
	}

	if gErr, ok := err.(*github.AbuseRateLimitError); ok {
		return true, fmt.Errorf("%s : %w", gErr.Message, ErrRateLimited)
	}

	return false, nil
}

// CheckAndWrapForKnownErrors checks for some of the known error types
func CheckAndWrapForKnownErrors(resp *github.Response, err error) (bool, error) {
	if err == nil {
		return false, err
	}
	if ok, wErr := isGithubRateLimit(err); ok {
		return ok, wErr
	}

	if ok, wErr := isGithub403(resp, err); ok {
		return ok, wErr
	}

	return false, err
}

// NewGithubAppClient creates a new github client from the supplied installationID
func NewGithubAppClient(installationID int64) (*github.Client, error) {
	itr, err := ghinstallation.New(http.DefaultTransport, int64(getGithubAppID()), installationID, []byte(getGithubAppPrivateKey()))
	if err != nil {
		return nil, err
	}
	return github.NewClient(&http.Client{Transport: itr}), nil
}

// NewGithubV4AppClient creates a new github v4 client from the supplied installationID
func NewGithubV4AppClient(installationID int64) (*githubv4.Client, error) {
	authTransport, err := ghinstallation.New(http.DefaultTransport, int64(getGithubAppID()), installationID, []byte(getGithubAppPrivateKey()))
	if err != nil {
		return nil, err
	}
	return githubv4.NewClient(&http.Client{Transport: authTransport, Timeout: 5 * time.Second}), nil
}

// NewGithubOauthClient creates github client from global accessToken
func NewGithubOauthClient() *github.Client {
	return NewGithubOauthClientWithAccessToken(getSecretAccessToken())
}

// NewGithubOauthClientWithAccessToken creates github client from specified accessToken
func NewGithubOauthClientWithAccessToken(accessToken string) *github.Client {
	ctx := context.TODO()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}
