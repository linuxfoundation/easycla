// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"sort"
	"sync"

	"github.com/linuxfoundation/easycla/cla-backend-go/project/repository"
	v1Project "github.com/linuxfoundation/easycla/cla-backend-go/project/service"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/sirupsen/logrus"

	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	v2ProjectService "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
)

// Service interface defines the v2 project service methods
type Service interface {
	GetCLAProjectsByID(ctx context.Context, foundationSFID string) (*models.EnabledClaList, error)
}

// service
type service struct {
	v1ProjectService  v1Project.Service
	projectRepo       repository.ProjectRepository
	projectsClaGroups projects_cla_groups.Repository
}

// NewService returns an instance of v2 project service
func NewService(v1ProjectService v1Project.Service, projectRepo repository.ProjectRepository, pcgRepo projects_cla_groups.Repository) Service {
	return &service{
		v1ProjectService:  v1ProjectService,
		projectRepo:       projectRepo,
		projectsClaGroups: pcgRepo,
	}
}

func (s *service) GetCLAProjectsByID(ctx context.Context, foundationSFID string) (*models.EnabledClaList, error) {
	f := logrus.Fields{
		"functionName":   "v2 project/service/GetCLAProjectsByID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"foundationSFID": foundationSFID,
	}

	enabledClas := make([]*models.EnabledCla, 0)
	claGroupsMapping, err := s.projectsClaGroups.GetProjectsIdsForFoundation(ctx, foundationSFID)
	if err != nil {
		return nil, err
	}
	if len(claGroupsMapping) == 0 {
		return &models.EnabledClaList{List: enabledClas}, nil
	}
	var wg sync.WaitGroup
	rchan := make(chan *models.EnabledCla)
	wg.Add(len(claGroupsMapping))
	go func() {
		wg.Wait()
		close(rchan)
	}()

	for _, cgm := range claGroupsMapping {
		go func(projectSFID, claGroupID string) {
			defer wg.Done()
			cla := &models.EnabledCla{
				ProjectSfid: projectSFID,
			}

			psc := v2ProjectService.GetClient()
			projectDetails, err := psc.GetProject(projectSFID)
			if err != nil {
				log.WithFields(f).Warnf("unable to fetch project details of %s from project-service", projectSFID)
			} else {
				cla.ProjectName = projectDetails.Name
				cla.ProjectLogo = projectDetails.ProjectLogo
				cla.ProjectType = projectDetails.ProjectType
				cla.FoundationSfid = foundationSFID
			}

			claGroup, err := s.projectRepo.GetCLAGroupByID(ctx, claGroupID, repository.DontLoadRepoDetails)
			if err != nil {
				log.WithFields(f).Warnf("unable to fetch cla-group details of %s", claGroupID)
			} else {
				cla.CclaEnabled = aws.Bool(claGroup.ProjectCCLAEnabled)
				cla.IclaEnabled = aws.Bool(claGroup.ProjectICLAEnabled)
				cla.CclaRequiresIcla = aws.Bool(claGroup.ProjectCCLARequiresICLA)
			}
			rchan <- cla
		}(cgm.ProjectSFID, cgm.ClaGroupID)
	}

	for enabledCLA := range rchan {
		enabledClas = append(enabledClas, enabledCLA)
	}

	// sort by project names
	sort.Slice(enabledClas, func(i, j int) bool {
		return enabledClas[i].ProjectName < enabledClas[j].ProjectName
	})

	// Add the foundation level CLA flag
	foundationLevelCLA, svcErr := s.v1ProjectService.SignedAtFoundationLevel(ctx, foundationSFID)
	if svcErr != nil {
		log.WithFields(f).Warnf("unable to fetch foundation level CLA status, error: %+v", svcErr)
		return nil, svcErr
	}

	return &models.EnabledClaList{
		FoundationLevelCLA: foundationLevelCLA,
		List:               enabledClas,
	}, nil
}
