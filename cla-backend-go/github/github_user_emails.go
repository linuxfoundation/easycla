// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"errors"
	"strings"

	"github.com/google/go-github/v93/github"
)

// VerifiedEmails lists the verified e-mail addresses of the GitHub account the user access token belongs to
func VerifiedEmails(ctx context.Context, accessToken string) ([]string, error) {
	client := NewGithubOauthClientWithAccessToken(accessToken)
	if client == nil {
		return nil, errors.New("unable to create the GitHub client")
	}
	return listVerifiedEmails(ctx, client)
}

func listVerifiedEmails(ctx context.Context, client *github.Client) ([]string, error) {
	emails := make([]string, 0)
	opts := &github.ListOptions{PerPage: 100}
	for {
		page, resp, err := client.Users.ListEmails(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, email := range page {
			if address := strings.TrimSpace(email.GetEmail()); email.GetVerified() && address != "" {
				emails = append(emails, address)
			}
		}
		if resp == nil || resp.NextPage == 0 {
			return emails, nil
		}
		opts.Page = resp.NextPage
	}
}
