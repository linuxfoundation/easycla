// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

// cd cla-backend-go
// go run ./cmd/get_pr_commits mochajs mocha 5803
// go run ./cmd/get_pr_commits mochajs mocha 5686
// go run ./cmd/get_pr_commits mlehotskylf-org2 easycla-dev 30
// go run ./cmd/get_pr_commits mlehotskylf-org2 easycla-dev 36

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v37/github"
	easygh "github.com/linuxfoundation/easycla/cla-backend-go/github"
	"golang.org/x/oauth2"
)

func die(msg string) {
	fmt.Fprintln(os.Stderr, "ERROR:", msg)
	os.Exit(1)
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func newGithubOAuthClient(token string) *gh.Client {
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(context.Background(), src)
	return gh.NewClient(httpClient)
}

func githubTokenFilePath() string {
	if path := strings.TrimSpace(os.Getenv("GITHUB_TOKEN_FILE")); path != "" {
		return path
	}
	return "/etc/github/oauth"
}

func loadGithubToken() (string, error) {
	tokenFile := githubTokenFilePath()
	//nolint:gosec // G304: developer helper intentionally allows explicit local token-file override via GITHUB_TOKEN_FILE.
	tokenBytes, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", fmt.Errorf("unable to read %s: %w", tokenFile, err)
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return "", fmt.Errorf("%s is empty", tokenFile)
	}
	return token, nil
}

func main() {
	if len(os.Args) != 4 {
		die(fmt.Sprintf("usage: %s <org> <repo> <pr_number>", os.Args[0]))
	}

	owner := strings.TrimSpace(os.Args[1])
	repo := strings.TrimSpace(os.Args[2])

	prNumber, err := strconv.Atoi(os.Args[3])
	if err != nil {
		die("invalid pr_number: " + os.Args[3])
	}
	token, err := loadGithubToken()
	if err != nil {
		die(err.Error())
	}

	client := newGithubOAuthClient(token)

	commits, err := easygh.ListPullRequestCommitsCompare(context.Background(), client, owner, repo, prNumber)
	if err != nil {
		die(err.Error())
	}

	fmt.Println("compare results:")
	fmt.Printf("count: %d\n", len(commits))
	ac := 0
	for _, c := range commits {
		if c == nil {
			continue
		}
		fmt.Printf(
			"%s | author_id: %d | login: %s | name: %s | email: %s | msg: %s\n",
			c.GetSHA(),
			c.GetAuthor().GetID(),
			c.GetAuthor().GetLogin(),
			c.GetAuthor().GetName(),
			c.GetAuthor().GetEmail(),
			firstLine(c.GetCommit().GetMessage()),
		)
		ac++
	}
	fmt.Printf("actual count: %d\n", ac)
}
