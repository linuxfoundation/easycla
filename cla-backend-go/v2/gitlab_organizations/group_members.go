// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_organizations

import (
	"context"
	"regexp"
)

// gateGroupURL is the pattern checkGitLabGroupApproval (v2/gitlab-activity) reduces an approval list
// entry with before it looks the registered group up by URL
var gateGroupURL = regexp.MustCompile(`(?P<base>\bhttps://gitlab\.com/\b)(?P<group>\bgroups\/\b)?(?P<name>\w+)`)

// gateSearchURL returns the URL the merge request gate looks the entry up by, "" when the gate cannot
// parse it (such an entry approves nobody)
func gateSearchURL(entry string) string {
	match := gateGroupURL.FindStringSubmatch(entry)
	if match == nil {
		return ""
	}
	if match[2] != "" {
		return entry
	}
	return match[1] + "groups/" + match[3]
}

// GroupMemberUsernames returns the resolver the signatures package uses when a GitLab group is removed
// from a corporate approval list: the usernames of the members of the registered group the merge
// request gate resolves the entry to; an entry the gate cannot resolve covers nobody.
func GroupMemberUsernames(svc ServiceInterface) func(ctx context.Context, entry string) ([]string, error) {
	return func(ctx context.Context, entry string) ([]string, error) {
		searchURL := gateSearchURL(entry)
		if searchURL == "" {
			return nil, nil
		}
		org, err := svc.GetGitLabOrganizationByURL(ctx, searchURL)
		if err != nil {
			return nil, err
		}
		if org == nil {
			return nil, nil
		}
		members, err := svc.GetGitLabGroupMembers(ctx, org.OrganizationID)
		if err != nil {
			return nil, err
		}
		var usernames []string
		if members != nil {
			for _, m := range members.List {
				if m != nil && m.Username != "" {
					usernames = append(usernames, m.Username)
				}
			}
		}
		return usernames, nil
	}
}
