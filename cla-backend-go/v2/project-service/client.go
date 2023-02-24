// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package project_service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/go-openapi/runtime"

	"github.com/communitybridge/easycla/cla-backend-go/token"
	"github.com/communitybridge/easycla/cla-backend-go/v2/project-service/client"
	"github.com/communitybridge/easycla/cla-backend-go/v2/project-service/client/project"
	"github.com/communitybridge/easycla/cla-backend-go/v2/project-service/models"
	runtimeClient "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

// constants
const (
	CLA = "CLA"
	NA  = "N/A"
)

// Client is client for user_service
type Client struct {
	cl *client.PMMAPI
}

var (
	projectServiceClient *Client
	// mutex is an object to allow us to lock access to the shared project service map while used by multiple go routines
	mutex = &sync.Mutex{}
	// Short term cache - only for the lifetime of this lambda
	projectServiceModels = make(map[string]*models.ProjectOutputDetailed)
	apiGWHost            string
)

// InitClient initializes the user_service client
func InitClient(APIGwURL string) {
	apiGWHost = strings.ReplaceAll(APIGwURL, "https://", "")
	projectServiceClient = &Client{
		cl: client.NewHTTPClientWithConfig(strfmt.Default, &client.TransportConfig{
			Host:     apiGWHost,
			BasePath: "project-service",
			Schemes:  []string{"https"},
		}),
	}
}

// GetClient return user_service client
func GetClient() *Client {
	return projectServiceClient
}

func (pmm *Client) getProject(projectSFID string, auth runtime.ClientAuthInfoWriter) (*models.ProjectOutputDetailed, error) {
	params := project.NewGetProjectParams()
	params.ProjectID = projectSFID
	result, err := pmm.cl.Project.GetProject(params, auth)
	if err != nil {
		return nil, err
	}
	return result.Payload, nil
}

// GetProject returns project details
func (pmm *Client) GetProject(projectSFID string) (*models.ProjectOutputDetailed, error) {
	f := logrus.Fields{
		"functionName": "v2.project-service.client.GetProject",
		"projectSFID":  projectSFID,
		"apiGWHost":    apiGWHost,
	}

	// Lookup in cache first
	mutex.Lock() // exclusive lock to the shared project service model map
	existingModel, exists := projectServiceModels[projectSFID]
	mutex.Unlock()

	if exists {
		//log.WithFields(f).Debugf("cache hit - cache size: %d", len(projectServiceModels))
		return existingModel, nil
	}
	log.WithFields(f).Debugf("cache miss - cache size: %d", len(projectServiceModels))

	tok, err := token.GetToken()
	if err != nil {
		return nil, err
	}
	clientAuth := runtimeClient.BearerToken(tok)

	// Lookup the project
	log.WithFields(f).Debugf("cache miss - looking up project in the service for: %s...", projectSFID)
	projectModel, err := pmm.getProject(projectSFID, clientAuth)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to lookup project in the project service for: %s", projectSFID)
		return nil, err
	}

	// Update our cache for next time
	mutex.Lock() // exclusive lock to the shared project service model map
	projectServiceModels[projectSFID] = projectModel
	log.WithFields(f).Debugf("added project model to cache - cache size: %d", len(projectServiceModels))
	mutex.Unlock()

	return projectModel, nil
}

// GetProjectByName returns project details for the associated project name
func (pmm *Client) GetProjectByName(ctx context.Context, projectName string) (*models.ProjectListSearch, error) {
	f := logrus.Fields{
		"functionName":   "v2.project-service.client.GetProjectByName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectName":    projectName,
		"apiGWHost":      apiGWHost,
	}
	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warning("problem retrieving token")
		return nil, err
	}

	clientAuth := runtimeClient.BearerToken(tok)
	result, err := pmm.cl.Project.SearchProjects(&project.SearchProjectsParams{
		Name:    []string{projectName},
		Context: ctx,
	}, clientAuth)
	if err != nil {
		log.WithFields(f).WithError(err).Warning("problem searching projects by name")
		return nil, err
	}

	return result.Payload, nil
}

// GetParentProject returns the parent project SFID if there is a parent, otherwise returns the provided projectSFID
func (pmm *Client) GetParentProject(projectSFID string) (string, error) {
	f := logrus.Fields{
		"functionName": "v2.project-service.client.GetParentProject",
		"projectSFID":  projectSFID,
		"apiGWHost":    apiGWHost,
	}

	// Use our helper function to find the parent, if it exists
	parentModel, err := pmm.GetParentProjectModel(projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Debugf("unable to lookup parentProjectModel using projectSFID: '%s'", projectSFID)
		return "", err
	}
	if parentModel == nil {
		log.WithFields(f).Debugf("unable to lookup parentProjectModel using projectSFID: '%s' - parent project model is nil", projectSFID)
		return "", err
	}

	return parentModel.ID, nil
}

// GetParentProjectModel returns the parent project model if there is a parent, otherwise returns nil
func (pmm *Client) GetParentProjectModel(projectSFID string) (*models.ProjectOutputDetailed, error) {
	f := logrus.Fields{
		"functionName": "v2.project-service.client.GetParentProjectModel",
		"projectSFID":  projectSFID,
		"apiGWHost":    apiGWHost,
	}

	// Lookup in cache first
	var exists bool
	var existingModel *models.ProjectOutputDetailed
	var existingParentModel *models.ProjectOutputDetailed

	// Current project in the cache?
	mutex.Lock() // exclusive lock to the shared project service model map
	existingModel, exists = projectServiceModels[projectSFID]
	mutex.Unlock()
	if exists {
		//log.WithFields(f).Debugf("cache hit - cache size: %d", len(projectServiceModels))

		if !utils.IsProjectHaveParent(existingModel) {
			//log.WithFields(f).Debugf("project %+v does not have a parent", existingModel)
			return nil, nil
		}
		log.WithFields(f).Debugf("project %+v has a parent", existingModel)

		//// Does this project they have a parent? projectModel.Parent is deprecated and no longer returned, use project.Foundation.ID/Name attribute instead
		//if existingModel.Foundation.Name == utils.TheLinuxFoundation || existingModel.Foundation.Name == utils.LFProjectsLLC {
		//	log.WithFields(f).Debugf("no parent for projectSFID %s or %s or %s is the parent...", projectSFID, utils.TheLinuxFoundation, utils.LFProjectsLLC)
		//	return nil, nil
		//}

		// Grab the parent ID once
		projectParentSFID := utils.GetProjectParentSFID(existingModel)
		if projectParentSFID == "" {
			log.WithFields(f).Debugf("unable to determine project %+v parent", existingModel)
			return nil, nil
		}

		// Parent SFID in the cache?
		mutex.Lock() // exclusive lock to the shared project service model map
		existingParentModel, exists = projectServiceModels[projectParentSFID]
		mutex.Unlock()
		if exists {
			return existingParentModel, nil
		}

		// Parent project not in the cache - lookup
		parentProjectModel, err := pmm.GetProject(projectParentSFID)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("unable to lookup parentProjectModel with projectSFID: '%s'", projectParentSFID)
			return nil, err
		}

		if parentProjectModel == nil {
			log.WithFields(f).WithError(err).Warnf("unable to lookup parentProjectModel with projectSFID: '%s' - project model is nil", projectParentSFID)
			return nil, nil
		}

		// Save/Update our cache for next time
		mutex.Lock() // exclusive lock to the shared project service model map
		projectServiceModels[projectParentSFID] = parentProjectModel
		log.WithFields(f).Debugf("added project model to cache - cache size: %d", len(projectServiceModels))
		mutex.Unlock()

		return parentProjectModel, nil
	}

	log.WithFields(f).Debugf("cache miss - looking up projectModel in projectSFID: %s", projectSFID)
	projectModel, err := pmm.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).Warnf("unable to lookup projectModel in projectModel service by projectSFID, error: %+v", err)
		return nil, err
	}
	if projectModel == nil {
		return nil, nil
	}

	// Save/Update our cache for next time
	mutex.Lock() // exclusive lock to the shared project service model map
	projectServiceModels[projectSFID] = projectModel
	log.WithFields(f).Debugf("added project model to cache - cache size: %d", len(projectServiceModels))
	mutex.Unlock()

	// No parent
	if !utils.IsProjectHaveParent(projectModel) {
		//log.WithFields(f).Debugf("project %+v does not have a parent", projectModel)
		return nil, nil
	}

	// Is the parent one of the root parents?
	if projectModel.Foundation.Name == utils.TheLinuxFoundation || projectModel.Foundation.Name == utils.LFProjectsLLC {
		log.WithFields(f).Debugf("no parent for projectSFID or %s or %s is the parent...", utils.TheLinuxFoundation, utils.LFProjectsLLC)
		return nil, nil
	}

	// Grab the parent ID once
	projectParentSFID := utils.GetProjectParentSFID(projectModel)
	if projectParentSFID == "" {
		log.WithFields(f).Debugf("unable to determine project %+v parent", projectModel)
		return nil, nil
	}

	// Parent in the cache?
	mutex.Lock() // exclusive lock to the shared project service model map
	existingParentModel, exists = projectServiceModels[projectParentSFID]
	mutex.Unlock() // exclusive lock to the shared project service model map
	if exists {
		return existingParentModel, nil
	}

	// Parent project not in the cache - lookup
	parentProjectModel, err := pmm.GetProject(projectParentSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Debugf("unable to lookup parentProjectModel with projectSFID: '%s'", projectParentSFID)
		return nil, err
	}
	if parentProjectModel == nil {
		log.WithFields(f).WithError(err).Debugf("unable to lookup parentProjectModel with projectSFID: '%s' - project model is nil", projectParentSFID)
		return nil, nil
	}

	// Save/Update our cache for next time
	mutex.Lock() // exclusive lock to the shared project service model map
	projectServiceModels[projectParentSFID] = parentProjectModel
	log.WithFields(f).Debugf("added project model to cache - cache size: %d", len(projectServiceModels))
	mutex.Unlock() // exclusive lock to the shared project service model map

	return parentProjectModel, nil
}

// IsTheLinuxFoundation returns true if the specified project SFID is the The Linux Foundation project
func (pmm *Client) IsTheLinuxFoundation(projectSFID string) (bool, error) {
	f := logrus.Fields{
		"functionName": "v2.project-service.client.IsTheLinuxFoundation",
		"projectSFID":  projectSFID,
		"apiGWHost":    apiGWHost,
	}

	projectModel, err := pmm.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).Warnf("unable to lookup project by ID: %s error: %+v", projectSFID, err)
		return false, err
	}

	if projectModel.Name == utils.TheLinuxFoundation || projectModel.Name == utils.LFProjectsLLC {
		// Save into our cache for next time
		log.WithFields(f).Debugf("project is %s or %s...", utils.TheLinuxFoundation, utils.LFProjectsLLC)
		return true, nil
	}

	return false, nil
}

// IsParentTheLinuxFoundation returns true if the parent is the The Linux Foundation project
func (pmm *Client) IsParentTheLinuxFoundation(projectSFID string) (bool, error) {
	f := logrus.Fields{
		"functionName": "v2.project-service.client.IsParentTheLinuxFoundation",
		"projectSFID":  projectSFID,
		"apiGWHost":    apiGWHost,
	}

	log.WithFields(f).Debug("querying project...")
	projectModel, err := pmm.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).Warnf("unable to lookup project by ID: %s error: %+v", projectSFID, err)
		return false, err
	}

	if !utils.IsProjectHaveParent(projectModel) {
		return false, nil
	}

	parentProjectModel, err := pmm.GetProject(projectModel.Foundation.ID)
	if err != nil {
		log.WithFields(f).Warnf("unable to lookup parent project by ID: %s error: %+v", projectModel.Foundation.ID, err)
		return false, err
	}

	if parentProjectModel.Name == utils.TheLinuxFoundation || parentProjectModel.Name == utils.LFProjectsLLC {
		// Save into our cache for next time
		log.WithFields(f).Debugf("parent project is %s or %s...", utils.TheLinuxFoundation, utils.LFProjectsLLC)
		return true, nil
	}

	return false, nil
}

// EnableCLA enables CLA service in project-service
func (pmm *Client) EnableCLA(ctx context.Context, projectSFID string) error {
	f := logrus.Fields{
		"functionName":   "v2.project-service.client.EnableCLA",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"apiGWHost":      apiGWHost,
	}

	theLF, lookupErr := pmm.IsTheLinuxFoundation(projectSFID)
	if lookupErr != nil {
		log.WithFields(f).WithError(lookupErr).Warnf("unable to test if project is The Linux Foundation using projectSFID: %s", projectSFID)
		return lookupErr
	}
	if theLF {
		msg := fmt.Sprintf("unable to set the enabled CLA services for The Linux Foundation with projectSFID: %s - not allowed", projectSFID)
		log.WithFields(f).Debug(msg)
		return errors.New(msg)
	}

	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warning("problem retrieving token")
		return err
	}
	clientAuth := runtimeClient.BearerToken(tok)

	projectDetails, err := pmm.getProject(projectSFID, clientAuth)
	if err != nil {
		log.WithFields(f).WithError(err).Warning("problem retrieving project by SFID")
		return err
	}

	for _, serviceName := range projectDetails.EnabledServices {
		if serviceName == CLA {
			// CLA already enabled
			return nil
		}
	}

	enabledServices := projectDetails.EnabledServices
	enabledServices = append(enabledServices, CLA)
	return pmm.updateEnabledServices(ctx, projectSFID, enabledServices, clientAuth)
}

func (pmm *Client) updateEnabledServices(ctx context.Context, projectSFID string, enabledServices []string, clientAuth runtime.ClientAuthInfoWriter) error {
	f := logrus.Fields{
		"functionName":    "v2.project-service.client.updateEnabledServices",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"projectSFID":     projectSFID,
		"enabledServices": enabledServices,
		"apiGWHost":       apiGWHost,
	}

	params := project.NewUpdateProjectParams()
	params.ProjectID = projectSFID
	if len(enabledServices) == 0 {
		enabledServices = append(enabledServices, NA)
	}

	params.Body = &models.ProjectInput{
		ProjectCommon: models.ProjectCommon{
			EnabledServices: enabledServices,
		},
	}

	_, err := pmm.cl.Project.UpdateProject(params, clientAuth) //nolint
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem updating project enabled services")
	}

	return err
}

// DisableCLA enables CLA service in project-service
func (pmm *Client) DisableCLA(ctx context.Context, projectSFID string) error {
	f := logrus.Fields{
		"functionName":   "v2.project-service.client.DisableCLA",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"apiGWHost":      apiGWHost,
	}

	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warning("problem retrieving token")
		return err
	}
	clientAuth := runtimeClient.BearerToken(tok)

	projectDetails, err := pmm.getProject(projectSFID, clientAuth)
	if err != nil {
		log.WithFields(f).WithError(err).Warning("problem retrieving project by SFID")
		return err
	}

	newEnabledServices := make([]string, 0)
	var claFound bool
	for _, serviceName := range projectDetails.EnabledServices {
		if serviceName != CLA {
			newEnabledServices = append(newEnabledServices, serviceName)
		} else {
			claFound = true
		}
	}
	if !claFound {
		// CLA already disabled
		return nil
	}
	return pmm.updateEnabledServices(ctx, projectSFID, newEnabledServices, clientAuth)
}

// GetSummary gets projects tree hierarchy and project details
func (pmm *Client) GetSummary(ctx context.Context, projectSFID string) ([]*models.ProjectSummary, error) {
	f := logrus.Fields{
		"functionName":   "v2.project-service.client.Summary",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectID":      projectSFID,
	}

	tok, err := token.GetToken()
	if err != nil {
		return nil, err
	}

	clientAuth := runtimeClient.BearerToken(tok)

	filter := fmt.Sprintf("id eq %s", projectSFID)
	log.WithFields(f).Debugf("Getting project summary for :%s ", projectSFID)
	view := "pcc"
	offsetDefault := int64(0)
	orderByDefault := string("createddate")
	pageSizeDefault := int64(100)

	params := &project.GetSummaryParams{
		DollarFilter: &filter,
		MyProjects:   nil,
		Offset:       &offsetDefault,
		OrderBy:      &orderByDefault,
		PageSize:     &pageSizeDefault,
		View:         &view,
		Context:      ctx, // must set for the GetSummary API call, otherwise we get a Err:context.deadlineExceededError{}
	}

	result, err := pmm.cl.Project.GetSummary(params, clientAuth)

	if err != nil {
		log.WithFields(f).WithError(err).Debugf("unable to query project summary for : %s , error: %+v ", projectSFID, err)
		return nil, err
	}

	return result.Payload.Data, nil
}

// IsAnyProjectTheRootParent returns true if one or more of the project ID's in the list is one of the root parents, returns false otherwise
func (pmm *Client) IsAnyProjectTheRootParent(sliceProjectSFID []string) bool {
	var retVal bool

	// Check each project to see if it is one of the root parents
	for _, projectSFID := range sliceProjectSFID {
		// If so, return true, we're done
		if isTLF, err := pmm.IsTheLinuxFoundation(projectSFID); isTLF && err == nil {
			retVal = isTLF
			break
		}
	}

	return retVal
}

// RemoveLinuxFoundationParentsFromProjectList removes any Linux Foundation root/parent projects from the list
func (pmm *Client) RemoveLinuxFoundationParentsFromProjectList(projectList []string) []string {
	var filteredProjectList []string
	for _, projectSFID := range projectList {
		// If not one of our Linux Foundation root/parent projects, then add it to the list
		if isTLF, err := pmm.IsTheLinuxFoundation(projectSFID); !isTLF && err == nil {
			filteredProjectList = append(filteredProjectList, projectSFID)
		}
	}
	return filteredProjectList
}
