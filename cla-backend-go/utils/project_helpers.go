// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service/models"

// GetProjectParentSFID returns the project parent SFID if available, otherwise returns empty string
func GetProjectParentSFID(project *models.ProjectOutputDetailed) string {
	if project == nil || project.Foundation == nil || project.Foundation.ID == "" || project.Foundation.Name == "" || project.Foundation.Slug == "" {
		return ""
	}
	return project.Foundation.ID
}

// IsProjectHaveParent returns true if the specified project has a parent
func IsProjectHaveParent(project *models.ProjectOutputDetailed) bool {
	return project != nil && project.Foundation != nil && project.Foundation.ID != "" && project.Foundation.Name != "" && project.Foundation.Slug != ""
}

// IsProjectHasRootParent determines if a given project has a root parent. A root parent is a parent that is empty parent or the parent is TLF or LFProjects
func IsProjectHasRootParent(project *models.ProjectOutputDetailed) bool {
	return project.Foundation == nil || (project.Foundation != nil && project.Foundation.ID != "" && (project.Foundation.Name == TheLinuxFoundation))
}

// IsStandaloneProject determines if a given project is a standalone project. A standalone project is a project with no parent or the parent is TLF/LFProjects and does not have any children
func IsStandaloneProject(project *models.ProjectOutputDetailed) bool {
	// standalone: No parent or parent is TLF/LFProjects....and no children
	return (project.Foundation == nil ||
		(project.Foundation != nil && (project.Foundation.Name == TheLinuxFoundation))) &&
		len(project.Projects) == 0
}

// IsProjectHaveChildren determines if a given project has children
func IsProjectHaveChildren(project *models.ProjectOutputDetailed) bool {
	// a project model with a project list means it has children
	return len(project.Projects) > 0
}

// IsProjectCategory determines if a given project is categorised as cla project sfid
func IsProjectCategory(project *models.ProjectOutputDetailed, parent *models.ProjectOutputDetailed) bool {
	return project.ProjectType == ProjectTypeProject || (!IsProjectHasRootParent(project) && parent.ProjectType == ProjectTypeProjectGroup && project.ProjectType == ProjectTypeProjectGroup)
}
