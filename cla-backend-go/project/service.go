// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"sync"

	"github.com/communitybridge/easycla/cla-backend-go/users"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/repositories"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/restapi/operations/project"
	"github.com/communitybridge/easycla/cla-backend-go/gerrits"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// Service interface defines the project service methods/functions
type Service interface {
	CreateCLAGroup(ctx context.Context, project *models.ClaGroup) (*models.ClaGroup, error)
	GetCLAGroups(ctx context.Context, params *project.GetProjectsParams) (*models.ClaGroups, error)
	GetCLAGroupByID(ctx context.Context, claGroupID string) (*models.ClaGroup, error)
	GetCLAGroupsByExternalSFID(ctx context.Context, projectSFID string) (*models.ClaGroups, error)
	GetCLAGroupsByExternalID(ctx context.Context, params *project.GetProjectsByExternalIDParams) (*models.ClaGroups, error)
	GetCLAGroupByName(ctx context.Context, projectName string) (*models.ClaGroup, error)
	GetCLAGroupCurrentICLATemplateURLByID(ctx context.Context, claGroupID string) (string, error)
	GetCLAGroupCurrentCCLATemplateURLByID(ctx context.Context, claGroupID string) (string, error)
	DeleteCLAGroup(ctx context.Context, claGroupID string) error
	UpdateCLAGroup(ctx context.Context, claGroupModel *models.ClaGroup) (*models.ClaGroup, error)
	GetClaGroupsByFoundationSFID(ctx context.Context, foundationSFID string, loadRepoDetails bool) (*models.ClaGroups, error)
	GetClaGroupByProjectSFID(ctx context.Context, projectSFID string, loadRepoDetails bool) (*models.ClaGroup, error)
	SignedAtFoundationLevel(ctx context.Context, foundationSFID string) (bool, error)
	GetCLAManagers(ctx context.Context, claGroupID string) ([]*models.ClaManagerUser, error)
}

// service
type service struct {
	repo                ProjectRepository
	repositoriesRepo    repositories.Repository
	gerritRepo          gerrits.Repository
	projectCLAGroupRepo projects_cla_groups.Repository
	usersRepo           users.UserRepository
}

// NewService returns an instance of the project service
func NewService(projectRepo ProjectRepository, repositoriesRepo repositories.Repository, gerritRepo gerrits.Repository, projectCLAGroupRepo projects_cla_groups.Repository, usersRepo users.UserRepository) Service {
	return service{
		repo:                projectRepo,
		repositoriesRepo:    repositoriesRepo,
		gerritRepo:          gerritRepo,
		projectCLAGroupRepo: projectCLAGroupRepo,
		usersRepo:           usersRepo,
	}
}

// CreateCLAGroup service method
func (s service) CreateCLAGroup(ctx context.Context, claGroupModel *models.ClaGroup) (*models.ClaGroup, error) {
	return s.repo.CreateCLAGroup(ctx, claGroupModel)
}

// GetCLAGroups service method
func (s service) GetCLAGroups(ctx context.Context, params *project.GetProjectsParams) (*models.ClaGroups, error) {
	return s.repo.GetCLAGroups(ctx, params)
}

// GetCLAGroupByID service method
func (s service) GetCLAGroupByID(ctx context.Context, claGroupID string) (*models.ClaGroup, error) {
	f := logrus.Fields{
		"functionName":    "GetCLAGroupByID",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"claGroupID":      claGroupID,
		"loadRepoDetails": LoadRepoDetails,
	}

	log.WithFields(f).Debug("locating CLA Group by ID...")
	project, err := s.repo.GetCLAGroupByID(ctx, claGroupID, LoadRepoDetails)
	if err != nil {
		return nil, err
	}

	// No Foundation SFID value? Maybe this is a v1 CLA Group record...
	if project.FoundationSFID == "" {
		log.WithFields(f).Debug("CLA Group missing FoundationSFID...")
		// Most likely this is a CLA Group v1 record - use the external ID if available
		if project.ProjectExternalID != "" {
			log.WithFields(f).Debugf("CLA Group assigning foundationID to value of external ID: %s", project.ProjectExternalID)
			project.FoundationSFID = project.ProjectExternalID
		}
	}

	if project.FoundationSFID != "" {
		signed, checkErr := s.SignedAtFoundationLevel(ctx, project.FoundationSFID)
		if checkErr != nil {
			return nil, checkErr
		}
		project.FoundationLevelCLA = signed
	}

	return project, nil
}

// GetCLAGroupsByExternalSFID returns a list of projects based on the external SFID parameter
func (s service) GetCLAGroupsByExternalSFID(ctx context.Context, projectSFID string) (*models.ClaGroups, error) {
	return s.GetCLAGroupsByExternalID(ctx, &project.GetProjectsByExternalIDParams{
		HTTPRequest: nil,
		NextKey:     nil,
		PageSize:    nil,
		ProjectSFID: projectSFID,
	})
}

// GetCLAGroupsByExternalID returns a list of projects based on the external ID parameters
func (s service) GetCLAGroupsByExternalID(ctx context.Context, params *project.GetProjectsByExternalIDParams) (*models.ClaGroups, error) {
	f := logrus.Fields{
		"functionName":   "GetCLAGroupsByExternalID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    params.ProjectSFID,
		"NextKey":        params.NextKey,
		"PageSize":       params.PageSize}
	log.Debugf("Project Service Handler - GetCLAGroupsByExternalID")
	projects, err := s.repo.GetCLAGroupsByExternalID(ctx, params, LoadRepoDetails)
	if err != nil {
		log.WithFields(f).Warnf("problem with query, error: %+v", err)
		return nil, err
	}
	numberOfProjects := len(projects.Projects)
	if numberOfProjects == 0 {
		return projects, nil
	}

	// Add repository information in the response model
	var wg sync.WaitGroup
	wg.Add(numberOfProjects)
	for i := range projects.Projects {
		go func(project *models.ClaGroup) {
			defer wg.Done()
			s.fillRepoInfo(ctx, project)
		}(&projects.Projects[i])
	}
	wg.Wait()

	return projects, nil
}

// GetCLAGroupByName service method
func (s service) GetCLAGroupByName(ctx context.Context, projectName string) (*models.ClaGroup, error) {
	return s.repo.GetCLAGroupByName(ctx, projectName)
}

func (s service) GetCLAGroupCurrentICLATemplateURLByID(ctx context.Context, claGroupID string) (string, error) {
	f := logrus.Fields{
		"functionName":   "GetCLAGroupCurrentICLATemplateURLByID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
	}

	claGroupModel, err := s.GetCLAGroupByID(ctx, claGroupID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to load CLA Group by ID")
		return "", &utils.CLAGroupNotFound{
			CLAGroupID: claGroupID,
			Err:        err,
		}
	}

	if claGroupModel == nil {
		log.WithFields(f).Warn("unable to load CLA Group by ID")
		return "", &utils.CLAGroupNotFound{
			CLAGroupID: claGroupID,
			Err:        nil,
		}
	}
	f["claGroupName"] = claGroupModel.ProjectName

	if !claGroupModel.ProjectICLAEnabled {
		log.WithFields(f).Warn("ICLA is not configured for this CLA Group - unable to return ICLA template URL")
		return "", &utils.CLAGroupICLANotConfigured{
			CLAGroupID:   claGroupID,
			CLAGroupName: claGroupModel.ProjectName,
			Err:          nil,
		}
	}

	docs := claGroupModel.ProjectIndividualDocuments
	if len(docs) == 0 {
		log.WithFields(f).Warn("ICLA is not configured for this CLA Group - missing document configuration")
		return "", &utils.CLAGroupICLANotConfigured{
			CLAGroupID:   claGroupID,
			CLAGroupName: claGroupModel.ProjectName,
			Err:          nil,
		}
	}

	// Fetch the current document
	currentDoc, err := GetCurrentDocument(ctx, docs)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem determining current ICLA for this CLA Group")
		return "", &utils.CLAGroupICLANotConfigured{
			CLAGroupID:   claGroupID,
			CLAGroupName: claGroupModel.ProjectName,
			Err:          err,
		}
	}

	if currentDoc == (models.ClaGroupDocument{}) {
		log.WithFields(f).WithError(err).Warn("problem determining current ICLA for this CLA Group")
		return "", &utils.CLAGroupICLANotConfigured{
			CLAGroupID:   claGroupID,
			CLAGroupName: claGroupModel.ProjectName,
			Err:          err,
		}
	}

	if currentDoc.DocumentS3URL == "" {
		log.WithFields(f).WithError(err).Warn("problem determining current ICLA for this CLA Group - document s3 url is empty")
		return "", &utils.CLAGroupICLANotConfigured{
			CLAGroupID:   claGroupID,
			CLAGroupName: claGroupModel.ProjectName,
			Err:          err,
		}
	}

	return currentDoc.DocumentS3URL, nil
}

func (s service) GetCLAGroupCurrentCCLATemplateURLByID(ctx context.Context, claGroupID string) (string, error) {
	f := logrus.Fields{
		"functionName":   "GetCLAGroupCurrentCCLATemplateURLByID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
	}

	claGroupModel, err := s.GetCLAGroupByID(ctx, claGroupID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to load CLA Group by ID")
		return "", &utils.CLAGroupNotFound{
			CLAGroupID: claGroupID,
			Err:        err,
		}
	}

	if claGroupModel == nil {
		log.WithFields(f).Warn("unable to load CLA Group by ID")
		return "", &utils.CLAGroupNotFound{
			CLAGroupID: claGroupID,
			Err:        nil,
		}
	}
	f["claGroupName"] = claGroupModel.ProjectName

	if !claGroupModel.ProjectCCLAEnabled {
		log.WithFields(f).Warn("CCLA is not configured for this CLA Group - unable to return CCLA template URL")
		return "", &utils.CLAGroupCCLANotConfigured{
			CLAGroupID:   claGroupID,
			CLAGroupName: claGroupModel.ProjectName,
			Err:          nil,
		}
	}

	docs := claGroupModel.ProjectCorporateDocuments
	if len(docs) == 0 {
		log.WithFields(f).Warn("CCLA is not configured for this CLA Group - missing document configuration")
		return "", &utils.CLAGroupCCLANotConfigured{
			CLAGroupID:   claGroupID,
			CLAGroupName: claGroupModel.ProjectName,
			Err:          nil,
		}
	}

	// Fetch the current document
	currentDoc, err := GetCurrentDocument(ctx, docs)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem determining current CCLA for this CLA Group")
		return "", &utils.CLAGroupCCLANotConfigured{
			CLAGroupID:   claGroupID,
			CLAGroupName: claGroupModel.ProjectName,
			Err:          err,
		}
	}

	if currentDoc == (models.ClaGroupDocument{}) {
		log.WithFields(f).WithError(err).Warn("problem determining current CCLA for this CLA Group")
		return "", &utils.CLAGroupCCLANotConfigured{
			CLAGroupID:   claGroupID,
			CLAGroupName: claGroupModel.ProjectName,
			Err:          err,
		}
	}

	if currentDoc.DocumentS3URL == "" {
		log.WithFields(f).WithError(err).Warn("problem determining current CCLA for this CLA Group - document s3 url is empty")
		return "", &utils.CLAGroupCCLANotConfigured{
			CLAGroupID:   claGroupID,
			CLAGroupName: claGroupModel.ProjectName,
			Err:          err,
		}
	}

	return currentDoc.DocumentS3URL, nil
}

// DeleteCLAGroup service method
func (s service) DeleteCLAGroup(ctx context.Context, claGroupID string) error {
	return s.repo.DeleteCLAGroup(ctx, claGroupID)
}

// UpdateCLAGroup service method
func (s service) UpdateCLAGroup(ctx context.Context, claGroupModel *models.ClaGroup) (*models.ClaGroup, error) {
	// Updates to the CLA Group "projects" table will cause a DB trigger handler (separate lambda) to also update other
	// tables where we have the CLA Group name/description
	return s.repo.UpdateCLAGroup(ctx, claGroupModel)
}

// GetClaGroupsByFoundationSFID service method
func (s service) GetClaGroupsByFoundationSFID(ctx context.Context, foundationSFID string, loadRepoDetails bool) (*models.ClaGroups, error) {
	return s.repo.GetClaGroupsByFoundationSFID(ctx, foundationSFID, loadRepoDetails)
}

// GetClaGroupByProjectSFID service method
func (s service) GetClaGroupByProjectSFID(ctx context.Context, projectSFID string, loadRepoDetails bool) (*models.ClaGroup, error) {
	return s.repo.GetClaGroupByProjectSFID(ctx, projectSFID, loadRepoDetails)
}

// SignedAtFoundationLevel returns true if the specified foundation has a CLA Group at the foundation level, returns false otherwise.
func (s service) SignedAtFoundationLevel(ctx context.Context, foundationSFID string) (bool, error) {
	f := logrus.Fields{
		"functionName":   "SignedAtFoundationLevel",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"foundationSFID": foundationSFID,
	}

	log.WithFields(f).Debug("querying foundation CLA Group entries...")
	entries, pcgErr := s.projectCLAGroupRepo.GetProjectsIdsForFoundation(ctx, foundationSFID)
	if pcgErr != nil {
		return false, pcgErr
	}
	log.WithFields(f).Debugf("loaded %d CLA Group entries signed at foundation level...", len(entries))

	// Check for number of claGroups for foundation
	foundationLevelCLAGroup := false
	for _, entry := range entries {
		if entry.ProjectSFID == entry.FoundationSFID {
			foundationLevelCLAGroup = true
			break
		}
	}

	log.WithFields(f).Debugf("returning %t for signed at foundation level for: %s", foundationLevelCLAGroup, foundationSFID)
	return foundationLevelCLAGroup, nil
}

// GetCLAManagers retrieves a list of managers for the give claGroupID
func (s service) GetCLAManagers(ctx context.Context, claGroupID string) ([]*models.ClaManagerUser, error) {
	claGroupModel, err := s.GetCLAGroupByID(ctx, claGroupID)
	if err != nil {
		return nil, err
	}

	if len(claGroupModel.ProjectACL) == 0 {
		return nil, nil
	}

	var managers []*models.ClaManagerUser
	for _, lfUserName := range claGroupModel.ProjectACL {
		log.Debugf("getting cla manager  user : %s", lfUserName)
		u, err := s.usersRepo.GetUserByLFUserName(lfUserName)
		if err != nil {
			log.Warnf("fetching the user with lfUserName : %s failed : %v", lfUserName, err)
			return nil, err
		}
		managers = append(managers, &models.ClaManagerUser{
			UserEmail: u.LfEmail,
			UserLFID:  u.LfUsername,
			UserName:  u.Username,
		})
	}

	return managers, nil
}
