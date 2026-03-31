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

	tokenBytes, err := os.ReadFile("/etc/github/oauth")
	if err != nil {
		die("unable to read /etc/github/oauth: " + err.Error())
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		die("/etc/github/oauth is empty")
	}

	client := newGithubOAuthClient(token)

	commits, err := easygh.ListPullRequestCommitsCompare(context.Background(), client, owner, repo, prNumber)
	if err != nil {
		die(err.Error())
	}

	fmt.Println("compare results:")
	fmt.Printf("count: %d\n", len(commits))
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
	}
}
