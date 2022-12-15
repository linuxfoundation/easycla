// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"context"
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/project/repository"
	service2 "github.com/communitybridge/easycla/cla-backend-go/project/service"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	v2ProjectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
)

// EmailTemplateService has utility functions needed to pre-fill the email params
type EmailTemplateService interface {
	PrefillV2CLAProjectParams(projectSFIDs []string) ([]CLAProjectParams, error)
	GetCLAGroupTemplateParamsFromProjectSFID(claGroupVersion, projectSFID string) (CLAGroupTemplateParams, error)
	GetCLAGroupTemplateParamsFromCLAGroup(claGroupID string) (CLAGroupTemplateParams, error)
}

type emailTemplateServiceProvider struct {
	claGroupRepository repository.ProjectRepository
	repository         projects_cla_groups.Repository
	projectService     service2.Service
	corporateConsoleV1 string
	corporateConsoleV2 string
}

// NewEmailTemplateService creates a new instance of email template service
func NewEmailTemplateService(claGroupRepository repository.ProjectRepository, repository projects_cla_groups.Repository, projectService service2.Service, corporateConsoleV1, corporateConsoleV2 string) EmailTemplateService {
	return &emailTemplateServiceProvider{
		claGroupRepository: claGroupRepository,
		repository:         repository,
		projectService:     projectService,
		corporateConsoleV1: corporateConsoleV1,
		corporateConsoleV2: corporateConsoleV2,
	}
}

// PrefillV2CLAProjectParams for each supplied projectSFIDs gets the claGroup info + checks if the project is signed at
// foundation level which is important for email rendering
func (s *emailTemplateServiceProvider) PrefillV2CLAProjectParams(projectSFIDs []string) ([]CLAProjectParams, error) {
	if len(projectSFIDs) == 0 {
		return nil, nil
	}

	var claProjectParams []CLAProjectParams
	// keeping a cache so we can safe some of the remote svc calls
	signedAtFoundationLevelCache := map[string]bool{}
	for _, pSFID := range projectSFIDs {
		projectCLAGroup, err := s.repository.GetClaGroupIDForProject(context.Background(), pSFID)
		if err != nil {
			return nil, fmt.Errorf("fetching project : %s failed: %v", pSFID, err)
		}

		params := CLAProjectParams{
			ExternalProjectName: projectCLAGroup.ProjectName,
			ProjectSFID:         pSFID,
			FoundationName:      projectCLAGroup.FoundationName,
			FoundationSFID:      projectCLAGroup.FoundationSFID,
			CorporateConsole:    s.corporateConsoleV2,
			IsFoundation:        false,
		}

		projectClient := v2ProjectService.GetClient()
		projectModel, err := projectClient.GetProject(pSFID)
		if err != nil {
			log.Warnf("unable to fetch project : %s details from project service : %v", pSFID, err)
			return nil, fmt.Errorf("unable to fetch project : %s details from project service : %v", pSFID, err)
		}

		isFoundation := utils.IsProjectHasRootParent(projectModel)
		params.IsFoundation = isFoundation

		signedResult, err := s.projectService.SignedAtFoundationLevel(context.Background(), projectCLAGroup.FoundationSFID)
		if err != nil {
			return nil, fmt.Errorf("fetching the SignedAtFoundationLevel for foundation : %s failed : %v", projectCLAGroup.FoundationSFID, err)
		}
		params.SignedAtFoundationLevel = signedResult
		signedAtFoundationLevelCache[projectCLAGroup.FoundationSFID] = signedResult

		claProjectParams = append(claProjectParams, params)
	}

	return claProjectParams, nil
}

// GetCLAGroupTemplateParamsFromProjectSFID creates CLAGroupTemplateParams from projectSFID
func (s *emailTemplateServiceProvider) GetCLAGroupTemplateParamsFromProjectSFID(claGroupVersion, projectSFID string) (CLAGroupTemplateParams, error) {
	if utils.V2 == claGroupVersion {
		return s.getV2CLAGroupTemplateParamsFromProjectSFID(projectSFID)
	}
	return s.getV1CLAGroupTemplateParamsFromProjectSFID(projectSFID)
}

// GetCLAGroupTemplateParamsFromCLAGroup fills up the CLAGroupTemplateParams with the basic information, it's missing the
// project information, if needed can be added later on...
func (s *emailTemplateServiceProvider) GetCLAGroupTemplateParamsFromCLAGroup(claGroupID string) (CLAGroupTemplateParams, error) {
	claGroupModel, err := s.claGroupRepository.GetCLAGroupByID(context.Background(), claGroupID, false)
	if err != nil {
		return CLAGroupTemplateParams{}, err
	}

	params := CLAGroupTemplateParams{}
	params.CLAGroupName = claGroupModel.ProjectName
	params.CorporateConsole = s.corporateConsoleV2
	params.Version = claGroupModel.Version

	return params, nil
}

func (s *emailTemplateServiceProvider) getV2CLAGroupTemplateParamsFromProjectSFID(projectSFID string) (CLAGroupTemplateParams, error) {
	projectCLAGroup, err := s.repository.GetClaGroupIDForProject(context.Background(), projectSFID)
	if err != nil {
		return CLAGroupTemplateParams{}, err
	}

	params := &CLAGroupTemplateParams{}
	params.CLAGroupName = projectCLAGroup.ClaGroupName
	params.CorporateConsole = s.corporateConsoleV2
	params.Version = projectCLAGroup.Version

	projects, err := s.repository.GetProjectsIdsForClaGroup(context.Background(), projectCLAGroup.ClaGroupID)
	if err != nil {
		return CLAGroupTemplateParams{}, fmt.Errorf("getProjectsIdsForClaGroup failed : %w", err)
	}

	params.ChildProjectCount = len(projects)
	var projectSFIDs []string
	for _, p := range projects {
		projectSFIDs = append(projectSFIDs, p.ProjectSFID)
	}

	projectParams, err := s.PrefillV2CLAProjectParams(projectSFIDs)
	if err != nil {
		return CLAGroupTemplateParams{}, fmt.Errorf("prefilling cla project params failed : %v", err)
	}
	params.Projects = projectParams
	return *params, nil
}

func (s *emailTemplateServiceProvider) getV1CLAGroupTemplateParamsFromProjectSFID(projectSFID string) (CLAGroupTemplateParams, error) {
	claGroup, err := s.claGroupRepository.GetClaGroupByProjectSFID(context.Background(), projectSFID, true)
	if err != nil {
		return CLAGroupTemplateParams{}, err
	}

	ps := v2ProjectService.GetClient()
	projectSF, projectErr := ps.GetProject(projectSFID)
	if projectErr != nil {
		return CLAGroupTemplateParams{}, fmt.Errorf("project service lookup error for SFID: %s, error : %+v", projectSFID, projectErr)
	}

	var signedResult bool
	if claGroup.FoundationSFID != "" {
		signedResult, err = s.projectService.SignedAtFoundationLevel(context.Background(), claGroup.FoundationSFID)
		if err != nil {
			log.Warnf("fetching the SignedAtFoundationLevel for foundation : %s failed : %v skipping assigning in email params", claGroup.FoundationSFID, err)
		}
	}

	return CLAGroupTemplateParams{
		CorporateConsole:  s.corporateConsoleV1,
		CLAGroupName:      claGroup.ProjectName,
		Version:           claGroup.Version,
		ChildProjectCount: 1,
		Projects: []CLAProjectParams{
			{
				ExternalProjectName:     projectSF.Name,
				ProjectSFID:             projectSFID,
				FoundationName:          projectSF.Foundation.Name,
				FoundationSFID:          projectSF.Foundation.ID,
				SignedAtFoundationLevel: signedResult,
				CorporateConsole:        s.corporateConsoleV1,
			},
		},
	}, nil

}
