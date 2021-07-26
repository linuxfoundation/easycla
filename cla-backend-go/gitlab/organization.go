// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"fmt"

	"github.com/xanzy/go-gitlab"
)

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
