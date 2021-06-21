// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package project_service

import (
	"context"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"

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
	cl *client.PMM
}

var (
	projectServiceClient *Client
	// Short term cache - only for the lifetime of this lambda
	projectServiceModes = make(map[string]*models.ProjectOutputDetailed)
	apiGWHost           string
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
	existingModel, exists := projectServiceModes[projectSFID]
	if exists {
		log.WithFields(f).Debugf("cache hit - cache size: %d", len(projectServiceModes))
		return existingModel, nil
	}
	log.WithFields(f).Debugf("cache miss - cache size: %d", len(projectServiceModes))

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
	projectServiceModes[projectSFID] = projectModel
	log.WithFields(f).Debugf("added project model to cache - cache size: %d", len(projectServiceModes))

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

	// Lookup in cache first
	existingModel, exists := projectServiceModes[projectSFID]
	if exists {
		log.WithFields(f).Debugf("cache hit - cache size: %d", len(projectServiceModes))
		return existingModel.Parent, nil
	}
	log.WithFields(f).Debugf("cache miss - cache size: %d", len(projectServiceModes))

	log.WithFields(f).Debug("looking up projectModel in SF by projectSFID")
	projectModel, err := pmm.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).Warnf("unable to lookup projectModel in projectModel service by projectSFID, error: %+v", err)
		return "", err
	}

	// Update our cache for next time
	projectServiceModes[projectSFID] = projectModel
	log.WithFields(f).Debugf("added project model to cache - cache size: %d", len(projectServiceModes))

	// Do they have a parent?
	if projectModel.Parent == "" || (projectModel.Foundation != nil &&
		(projectModel.Foundation.Name == utils.TheLinuxFoundation || projectModel.Foundation.Name == utils.LFProjectsLLC)) {
		log.WithFields(f).Debugf("no parent for projectSFID or %s or %s is the parent...", utils.TheLinuxFoundation, utils.LFProjectsLLC)
		return projectSFID, nil
	}

	log.WithFields(f).Debugf("returning parent projectSFID: %s", projectModel.Parent)
	return projectModel.Parent, nil
}

// IsTheLinuxFoundation returns true if the specified project SFID is the The Linux Foundation project
func (pmm *Client) IsTheLinuxFoundation(projectSFID string) (bool, error) {
	f := logrus.Fields{
		"functionName": "v2.project-service.client.IsTheLinuxFoundation",
		"projectSFID":  projectSFID,
		"apiGWHost":    apiGWHost,
	}

	log.WithFields(f).Debug("querying project...")
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

	if projectModel.Parent == "" {
		return false, nil
	}

	parentProjectModel, err := pmm.GetProject(projectModel.Parent)
	if err != nil {
		log.WithFields(f).Warnf("unable to lookup parent project by ID: %s error: %+v", projectModel.Parent, err)
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

//GetSummary gets projects tree hierarchy and project details
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
