// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"errors"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
)

// PrefillCLAManagerTemplateParamsFromClaGroup fetches data from projects_cla_groups.Repository to prefill some of the fields
// of CLAManagerTemplateParams, like childCount and FoundationName and etc
func PrefillCLAManagerTemplateParamsFromClaGroup(repository projects_cla_groups.Repository, projectSFID string, params *CLAManagerTemplateParams) error {
	projectCLAGroup, err := repository.GetClaGroupIDForProject(projectSFID)
	if err != nil {
		if errors.Is(err, projects_cla_groups.ErrProjectNotAssociatedWithClaGroup) {
			log.Warnf("no cla group was found for externalProjectID : %s skipping the prefill", projectSFID)
			return nil
		}
		return err
	}

	params.CLAGroupName = projectCLAGroup.ClaGroupName
	params.ProjectName = projectCLAGroup.ClaGroupName
	params.FoundationName = projectCLAGroup.FoundationName
	params.ExternalProjectName = projectCLAGroup.ProjectName

	projects, err := repository.GetProjectsIdsForClaGroup(projectCLAGroup.ClaGroupID)
	if err != nil {
		return err
	}

	params.ChildProjectCount = len(projects)
	return nil
}
