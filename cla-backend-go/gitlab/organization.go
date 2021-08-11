// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"fmt"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/xanzy/go-gitlab"
)

// UserGroup represents gitlab group
type UserGroup struct {
	Name     string
	FullPath string
}

// GetGroupByName gets a gitlab Group by the given name
func GetGroupByName(client *gitlab.Client, name string) (*gitlab.Group, error) {
	groups, _, err := client.Groups.ListGroups(&gitlab.ListGroupsOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetching groups failed : %v", err)
	}

	for _, group := range groups {
		if group.Name == name {
			return group, nil
		}
	}

	return nil, nil
}

// ListUserProjectGroups fetches the unique groups of a gitlab users groups,
// note: it doesn't list the projects/groups the user is member of ..., it's very limited
func ListUserProjectGroups(client *gitlab.Client, userID int) ([]*UserGroup, error) {
	listOptions := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		}}

	userGroupsMap := map[string]*UserGroup{}
	for {
		log.Debugf("fetching projects for user id : %d with options : %v", userID, listOptions.ListOptions)
		projects, resp, err := client.Projects.ListUserProjects(userID, listOptions)
		if err != nil {
			return nil, fmt.Errorf("listing user : %d projects failed : %v", userID, err)
		}
		log.Printf("fetched %d projects for the user ", len(projects))

		if len(projects) == 0 {
			break
		}

		for _, p := range projects {
			log.Debugf("checking following project : %s", p.PathWithNamespace)
			log.Debugf("fetched following namespace : %+v", p.Namespace)
			userGroupsMap[p.Namespace.FullPath] = &UserGroup{
				Name:     p.Namespace.Name,
				FullPath: p.Namespace.FullPath,
			}
		}

		if listOptions.Page >= resp.NextPage {
			break
		}
		listOptions.Page = resp.NextPage
	}

	var userGroups []*UserGroup
	for _, v := range userGroupsMap {
		userGroups = append(userGroups, v)
	}

	return userGroups, nil
}
