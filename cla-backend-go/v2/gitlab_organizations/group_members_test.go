// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_organizations

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
)

type groupMembersService struct {
	ServiceInterface
	orgs       map[string]*models.GitlabOrganization
	members    map[string][]*models.GitlabGroupMember
	lookupErr  error
	membersErr error
	askedURLs  []string
	askedIDs   []string
}

func (s *groupMembersService) GetGitLabOrganizationByURL(_ context.Context, url string) (*models.GitlabOrganization, error) {
	s.askedURLs = append(s.askedURLs, url)
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	return s.orgs[url], nil
}

func (s *groupMembersService) GetGitLabGroupMembers(_ context.Context, groupID string) (*models.GitlabGroupMembersList, error) {
	s.askedIDs = append(s.askedIDs, groupID)
	if s.membersErr != nil {
		return nil, s.membersErr
	}
	return &models.GitlabGroupMembersList{List: s.members[groupID]}, nil
}

// the resolver finds the group the way the merge request gate does: by the URL the approval list
// entry reduces to
func TestGroupMemberUsernamesResolvesEntriesLikeTheGate(t *testing.T) {
	cases := []struct {
		name      string
		entry     string
		askedURL  string
		usernames []string
	}{
		{name: "groups URL", entry: "https://gitlab.com/groups/acme", askedURL: "https://gitlab.com/groups/acme", usernames: []string{"alice", "bob"}},
		{name: "URL without the groups segment", entry: "https://gitlab.com/acme", askedURL: "https://gitlab.com/groups/acme", usernames: []string{"alice", "bob"}},
		{name: "subgroup URL is looked up as entered", entry: "https://gitlab.com/groups/acme/tools", askedURL: "https://gitlab.com/groups/acme/tools", usernames: []string{"carol"}},
		{name: "unregistered group covers nobody", entry: "https://gitlab.com/groups/unknown", askedURL: "https://gitlab.com/groups/unknown"},
		{name: "entry the gate cannot parse covers nobody", entry: "gitlab.com/acme"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			svc := &groupMembersService{
				orgs: map[string]*models.GitlabOrganization{
					"https://gitlab.com/groups/acme":       {OrganizationID: "org-acme"},
					"https://gitlab.com/groups/acme/tools": {OrganizationID: "org-tools"},
				},
				members: map[string][]*models.GitlabGroupMember{
					"org-acme":  {{Username: "alice"}, nil, {Username: ""}, {Username: "bob"}},
					"org-tools": {{Username: "carol"}},
				},
			}
			usernames, err := GroupMemberUsernames(svc)(context.Background(), test.entry)
			require.NoError(t, err)
			assert.Equal(t, test.usernames, usernames)
			if test.askedURL == "" {
				assert.Empty(t, svc.askedURLs)
			} else {
				assert.Equal(t, []string{test.askedURL}, svc.askedURLs)
			}
		})
	}
}

func TestGroupMemberUsernamesReportsLookupFailures(t *testing.T) {
	resolve := func(svc *groupMembersService) ([]string, error) {
		return GroupMemberUsernames(svc)(context.Background(), "https://gitlab.com/groups/acme")
	}
	_, err := resolve(&groupMembersService{lookupErr: errors.New("dynamodb: throttled")})
	require.EqualError(t, err, "dynamodb: throttled")

	svc := &groupMembersService{
		orgs:       map[string]*models.GitlabOrganization{"https://gitlab.com/groups/acme": {OrganizationID: "org-acme"}},
		membersErr: errors.New("gitlab: 503"),
	}
	_, err = resolve(svc)
	require.EqualError(t, err, "gitlab: 503")
	assert.Equal(t, []string{"org-acme"}, svc.askedIDs)
}
