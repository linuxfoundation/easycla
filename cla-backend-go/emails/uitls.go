// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"context"
	"errors"
	"fmt"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/project"
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
	params.Project = CLAProjectParams{
		ExternalProjectName:     projectCLAGroup.ProjectName,
		ProjectSFID:             projectSFID,
		FoundationName:          projectCLAGroup.FoundationName,
		FoundationSFID:          projectCLAGroup.FoundationSFID,
		SignedAtFoundationLevel: false,
		CorporateConsole:        "",
	}
	projects, err := repository.GetProjectsIdsForClaGroup(projectCLAGroup.ClaGroupID)
	if err != nil {
		return err
	}

	params.ChildProjectCount = len(projects)
	return nil
}

// PrefillCLAProjectParams for each supplied projectSFIDs gets the claGroup info + checks if the project is signed at
// foundation level which is important for email rendering
func PrefillCLAProjectParams(repository projects_cla_groups.Repository, projectService project.Service, projectSFIDs []string, corporateConsole string) ([]CLAProjectParams, error) {
	if len(projectSFIDs) == 0 {
		return nil, nil
	}

	var claProjectParams []CLAProjectParams
	// keeping a cache so we can safe some of the remote svc calls
	signedAtFoundationLevelCache := map[string]bool{}
	for _, pSFID := range projectSFIDs {
		projectCLAGroup, err := repository.GetClaGroupIDForProject(pSFID)
		if err != nil {
			return nil, fmt.Errorf("fetching project : %s failed: %v", pSFID, err)
		}

		params := CLAProjectParams{
			ExternalProjectName: projectCLAGroup.ProjectName,
			ProjectSFID:         pSFID,
			FoundationName:      projectCLAGroup.FoundationName,
			FoundationSFID:      projectCLAGroup.FoundationSFID,
			CorporateConsole:    corporateConsole,
		}
		signed, found := signedAtFoundationLevelCache[projectCLAGroup.FoundationSFID]
		if found {
			params.SignedAtFoundationLevel = signed
		}

		signedResult, err := projectService.SignedAtFoundationLevel(context.Background(), projectCLAGroup.FoundationSFID)
		if err != nil {
			return nil, fmt.Errorf("fetching the SignedAtFoundationLevel for foundation : %s failed : %v", projectCLAGroup.FoundationSFID, err)
		}
		params.SignedAtFoundationLevel = signedResult
		signedAtFoundationLevelCache[projectCLAGroup.FoundationSFID] = signedResult

		claProjectParams = append(claProjectParams, params)
	}

	return claProjectParams, nil
}
