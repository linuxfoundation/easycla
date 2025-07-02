// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"context"
	"errors"
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	goGitLab "github.com/xanzy/go-gitlab"
)

// UserGroup represents gitlab group
type UserGroup struct {
	Name     string
	FullPath string
}

// GetGroupsListAll returns a complete list of GitLab groups for which the client as authorization/visibility
func GetGroupsListAll(ctx context.Context, client *goGitLab.Client, minAccessLevel goGitLab.AccessLevelValue) ([]*goGitLab.Group, error) {
	f := logrus.Fields{
		"functionName":   "gitlab_api.client_groups.GetGroupsListAll",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	// https://docs.gitlab.com/ee/api/groups.html#list-groups
	// Query GitLab for repos - fetch the list of repositories available to the GitLab App
	listGroupsOpts := &goGitLab.ListGroupsOptions{
		ListOptions: goGitLab.ListOptions{
			Page:    1,   // starts with one: https://docs.gitlab.com/ee/api/#offset-based-pagination
			PerPage: 100, // max is 100
		},
		AllAvailable:   utils.Bool(true),                     // Show all the groups you have access to (defaults to false for authenticated users, true for administrators); Attributes owned and min_access_level have precedence
		MinAccessLevel: goGitLab.AccessLevel(minAccessLevel), // Limit by current user minimal access level.
	}

	var groupList []*goGitLab.Group
	for {
		groups, resp, listGroupsErr := client.Groups.ListGroups(listGroupsOpts)
		if listGroupsErr != nil {
			msg := fmt.Sprintf("unable to list groups, error: %+v", listGroupsErr)
			log.WithFields(f).WithError(listGroupsErr).Warn(msg)
			return nil, errors.New(msg)
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			msg := fmt.Sprintf("unable to list groups, status code: %d", resp.StatusCode)
			log.WithFields(f).WithError(listGroupsErr).Warn(msg)
			return nil, errors.New(msg)
		}

		// Append to our response
		groupList = append(groupList, groups...)

		// Do we have any records to process?
		if resp.NextPage == 0 {
			break
		}
	}

	return groupList, nil
}

// GetGroupByName gets a gitlab Group by the given name
func GetGroupByName(ctx context.Context, client *goGitLab.Client, name string) (*goGitLab.Group, error) {
	f := logrus.Fields{
		"functionName":   "gitlab_api.client_groups.GetGroupByName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	groups, resp, err := client.Groups.SearchGroup(name)
	//groups, _, err := client.Groups.ListGroups(&goGitLab.ListGroupsOptions{})
	if err != nil {
		msg := fmt.Sprintf("problem fetching groups, error: %+v", err)
		log.WithFields(f).WithError(err).Warn(msg)
		return nil, errors.New(msg)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := fmt.Sprintf("unable to search groups using query: %s, status code: %d", name, resp.StatusCode)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	for _, group := range groups {
		log.WithFields(f).Debugf("testing %s == %s or %s", name, group.Name, group.FullPath)
		if group.Name == name {
			return group, nil
		}
		if group.FullPath == name {
			return group, nil
		}
	}

	return nil, nil
}

// GetGroupByID gets a gitlab Group by the given name
func GetGroupByID(ctx context.Context, client *goGitLab.Client, groupID int) (*goGitLab.Group, error) {
	f := logrus.Fields{
		"functionName":   "gitlab_api.client_groups.GetGroupByName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	group, resp, err := client.Groups.GetGroup(groupID)
	if err != nil {
		msg := fmt.Sprintf("problem fetching group by ID: %d, error: %+v", groupID, err)
		log.WithFields(f).WithError(err).Warn(msg)
		return nil, errors.New(msg)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := fmt.Sprintf("unable to find group by ID: %d, status code: %d", groupID, resp.StatusCode)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	return group, nil
}

// GetGroupByFullPath gets a gitlab Group by the given full path
func GetGroupByFullPath(ctx context.Context, client *goGitLab.Client, fullPath string) (*goGitLab.Group, error) {
	f := logrus.Fields{
		"functionName":   "gitlab_api.client_groups.GetGroupByName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	groups, err := GetGroupsListAll(ctx, client, goGitLab.MaintainerPermissions)
	//groups, _, err := client.Groups.ListGroups(&goGitLab.ListGroupsOptions{})
	if err != nil {
		msg := fmt.Sprintf("problem fetching groups, error: %+v", err)
		log.WithFields(f).WithError(err).Warn(msg)
		return nil, errors.New(msg)
	}

	for _, group := range groups {
		log.WithFields(f).Debugf("testing %s == %s", fullPath, group.FullPath)
		if group.FullPath == fullPath {
			return group, nil
		}
	}

	return nil, nil
}

// GetGroupProjectListByGroupID returns a list of GitLab projects under the specified Organization
func GetGroupProjectListByGroupID(ctx context.Context, client *goGitLab.Client, groupID int) ([]*goGitLab.Project, error) {
	f := logrus.Fields{
		"functionName":   "gitlab_api.client_groups.GetGroupProjectListByGroupID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	if groupID == 0 {
		return nil, errors.New("invalid groupID value - 0")
	}

	opts := &goGitLab.ListGroupProjectsOptions{
		ListOptions: goGitLab.ListOptions{
			Page:    1,   // starts with one: https://docs.gitlab.com/ee/api/#offset-based-pagination
			PerPage: 100, // max is 100
		},
		IncludeSubgroups: utils.Bool(true),                                     // Include projects in subgroups of this group. Default is false
		MinAccessLevel:   goGitLab.AccessLevel(goGitLab.MaintainerPermissions), // Limit by current user minimal access level.
	}

	var projectList []*goGitLab.Project
	for {
		// https://docs.gitlab.com/ee/api/groups.html#list-a-groups-projects
		projects, resp, listProjectsErr := client.Groups.ListGroupProjects(groupID, opts)
		if listProjectsErr != nil {
			msg := fmt.Sprintf("unable to list projects, error: %+v", listProjectsErr)
			log.WithFields(f).WithError(listProjectsErr).Warn(msg)
			return nil, errors.New(msg)
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			msg := fmt.Sprintf("unable to list projects, status code: %d", resp.StatusCode)
			log.WithFields(f).WithError(listProjectsErr).Warn(msg)
			return nil, errors.New(msg)
		}

		// Append to our response
		projectList = append(projectList, projects...)

		// Do we have any records to process?
		if resp.NextPage == 0 {
			break
		}
	}

	return projectList, nil
}

// ListGroupMembers lists the members of a given groupID
func ListGroupMembers(ctx context.Context, client *goGitLab.Client, groupID int) ([]*goGitLab.GroupMember, error) {
	f := logrus.Fields{
		"functionName":   "gitlab_api.client_groups.GetGroupMembers",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	log.WithFields(f).Debugf("fetching gitlab members for groupID: %d", groupID)

	opts := &goGitLab.ListGroupMembersOptions{}
	members, _, err := client.Groups.ListGroupMembers(groupID, opts)
	if err != nil {
		log.WithFields(f).Debugf("unable to fetch members for gitlab GroupID : %d", groupID)
		return nil, err
	}
	return members, err
}

// ListUserProjectGroups fetches the unique groups of a gitlab users groups,
// note: it doesn't list the projects/groups the user is member of ..., it's very limited
func ListUserProjectGroups(ctx context.Context, client *goGitLab.Client, userID int) ([]*UserGroup, error) {
	f := logrus.Fields{
		"functionName":   "gitlab_api.client_groups.ListUserProjectGroups",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	listOptions := &goGitLab.ListProjectsOptions{
		ListOptions: goGitLab.ListOptions{
			PerPage: 100,
		}}

	userGroupsMap := map[string]*UserGroup{}
	for {
		log.WithFields(f).Debugf("fetching projects for user id : %d with options : %v", userID, listOptions.ListOptions)
		projects, resp, err := client.Projects.ListUserProjects(userID, listOptions)
		if err != nil {
			msg := fmt.Sprintf("listing user : %d projects failed : %v", userID, err)
			log.WithFields(f).Warn(msg)
			return nil, errors.New(msg)
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			msg := fmt.Sprintf("unable to list user projects using userID: %d, status code: %d", userID, resp.StatusCode)
			log.WithFields(f).Warn(msg)
			return nil, errors.New(msg)
		}
		log.Debugf("fetched %d projects for the user ", len(projects))

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
