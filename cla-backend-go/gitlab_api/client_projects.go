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

// GetProjectListAll returns a complete list of GitLab projects for which the client as authorization/visibility
func GetProjectListAll(ctx context.Context, client *goGitLab.Client) ([]*goGitLab.Project, error) {
	// https://docs.gitlab.com/ce/api/projects.html#list-projects
	// Query GitLab for repos - fetch the list of repositories available to the GitLab App
	listProjectsOpts := &goGitLab.ListProjectsOptions{
		ListOptions: goGitLab.ListOptions{
			Page:    1,   // starts with one: https://docs.gitlab.com/ee/api/#offset-based-pagination
			PerPage: 100, // max is 100
		},
		SearchNamespaces: utils.Bool(true),                                     // Include ancestor namespaces when matching search criteria. Default is false.
		Membership:       utils.Bool(true),                                     // Limit by projects that the current user is a member of.
		MinAccessLevel:   goGitLab.AccessLevel(goGitLab.MaintainerPermissions), // Limit by current user minimal access level.
	}

	return getProjectListWithOptions(ctx, client, listProjectsOpts)
}

// GetProjectListByOrgName returns a list of GitLab projects under the specified Organization
func GetProjectListByOrgName(ctx context.Context, client *goGitLab.Client, organizationName string) ([]*goGitLab.Project, error) {
	// Query GitLab for repos - fetch the list of repositories available to the GitLab App
	listProjectsOpts := &goGitLab.ListProjectsOptions{
		ListOptions: goGitLab.ListOptions{
			Page:    1,   // starts with one: https://docs.gitlab.com/ee/api/#offset-based-pagination
			PerPage: 100, // max is 100
		},
		Search:           utils.StringRef(organizationName),                    // filter by our organization name
		SearchNamespaces: utils.Bool(true),                                     // Include ancestor namespaces when matching search criteria. Default is false.
		Membership:       utils.Bool(true),                                     // Limit by projects that the current user is a member of.
		MinAccessLevel:   goGitLab.AccessLevel(goGitLab.MaintainerPermissions), // Limit by current user minimal access level.
	}

	return getProjectListWithOptions(ctx, client, listProjectsOpts)
}

// getProjectListWithOptions returns a list of GitLab projects using the specified filter
func getProjectListWithOptions(ctx context.Context, client *goGitLab.Client, opts *goGitLab.ListProjectsOptions) ([]*goGitLab.Project, error) {
	f := logrus.Fields{
		"functionName":   "gitlab_api.client_projects.getProjectListWithOptions",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	var projectList []*goGitLab.Project
	for {
		// Need to use this func to get the list of projects the user has access to, see: https://gitlab.com/gitlab-org/gitlab-foss/-/issues/63811
		projects, resp, listProjectsErr := client.Projects.ListProjects(opts)
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

// GetProjectByID returns the GitLab project for the specified ID
func GetProjectByID(ctx context.Context, client *goGitLab.Client, gitLabProjectID int) (*goGitLab.Project, error) {
	f := logrus.Fields{
		"functionName":    "gitlab.client.GetProjectByID",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"gitLabProjectID": gitLabProjectID,
	}

	// Query GitLab for repos - fetch the list of repositories available to the GitLab App
	project, resp, getProjectErr := client.Projects.GetProject(gitLabProjectID, &goGitLab.GetProjectOptions{})
	if getProjectErr != nil {
		msg := fmt.Sprintf("unable to get project by ID: %d, error: %+v", gitLabProjectID, getProjectErr)
		log.WithFields(f).WithError(getProjectErr).Warn(msg)
		return nil, errors.New(msg)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := fmt.Sprintf("unable to get project by ID: %d, status code: %d", gitLabProjectID, resp.StatusCode)
		log.WithFields(f).WithError(getProjectErr).Warn(msg)
		return nil, errors.New(msg)
	}
	if project == nil {
		msg := fmt.Sprintf("unable to get project by ID: %d, project is empty", gitLabProjectID)
		log.WithFields(f).WithError(getProjectErr).Warn(msg)
		return nil, errors.New(msg)
	}

	return project, nil
}

// EnableMergePipelineProtection enables the pipeline protection on given project, by default it's
// turned off and when a new MR is raised users can merge requests bypassing the pipelines. With this
// setting gitlab disables the Merge button if any of the pipelines are failing
func EnableMergePipelineProtection(ctx context.Context, gitlabClient *goGitLab.Client, projectID int) error {
	f := logrus.Fields{
		"functionName":    "gitlab.client.EnableMergePipelineProtection",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"gitLabProjectID": projectID,
	}

	project, _, err := gitlabClient.Projects.GetProject(projectID, &goGitLab.GetProjectOptions{})
	if err != nil {
		return fmt.Errorf("fetching project failed : %v", err)
	}

	log.WithFields(f).Debugf("Merge if Pipeline is succeeds flag enabled : %v", project.OnlyAllowMergeIfPipelineSucceeds)
	if project.OnlyAllowMergeIfPipelineSucceeds {
		return nil
	}

	project.OnlyAllowMergeIfPipelineSucceeds = true
	log.WithFields(f).Debugf("Enabling Merge Pipeline protection")
	_, _, err = gitlabClient.Projects.EditProject(projectID, &goGitLab.EditProjectOptions{
		OnlyAllowMergeIfPipelineSucceeds: goGitLab.Bool(true),
	})

	if err != nil {
		return fmt.Errorf("editing project : %d failed : %v", projectID, err)
	}
	return nil
}
