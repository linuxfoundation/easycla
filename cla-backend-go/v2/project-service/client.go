// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package project_service

import (
	"strings"

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
	cl *client.PMM
}

var (
	projectServiceClient *Client
)

// InitClient initializes the user_service client
func InitClient(APIGwURL string) {
	APIGwURL = strings.ReplaceAll(APIGwURL, "https://", "")
	projectServiceClient = &Client{
		cl: client.NewHTTPClientWithConfig(strfmt.Default, &client.TransportConfig{
			Host:     APIGwURL,
			BasePath: "project-service/v1",
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
	tok, err := token.GetToken()
	if err != nil {
		return nil, err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	return pmm.getProject(projectSFID, clientAuth)
}

// GetProjectByName returns project details for the associated project name
func (pmm *Client) GetProjectByName(projectName string) (*models.ProjectListSearch, error) {
	tok, err := token.GetToken()
	if err != nil {
		return nil, err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	result, err := pmm.cl.Project.SearchProjects(&project.SearchProjectsParams{
		Name: []string{projectName},
	}, clientAuth)
	if err != nil {
		return nil, err
	}
	return result.Payload, nil
}

// GetParentProject returns the parent project SFID if there is a parent, otherwise returns the provided projectSFID
func (pmm *Client) GetParentProject(projectSFID string) (string, error) {
	f := logrus.Fields{
		"functionName": "getParentProject",
		"projectSFID":  projectSFID,
	}

	log.WithFields(f).Debug("looking up projectModel in SF by projectSFID")
	projectModel, err := pmm.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).Warnf("unable to lookup projectModel in projectModel service by projectSFID, error: %+v", err)
		return "", err
	}

	// Do they have a parent?
	if projectModel.Parent == "" || projectModel.Parent == utils.TheLinuxFoundation || projectModel.Parent == utils.LFProjectsLLC {
		log.WithFields(f).Debugf("no parent for projectSFID or %s or %s is the parent...", utils.TheLinuxFoundation, utils.LFProjectsLLC)
		return projectSFID, nil
	}

	log.WithFields(f).Debugf("returning parent projectSFID: %s", projectModel.Parent)
	return projectModel.Parent, nil
}

// IsTheLinuxFoundation returns true if the specified project SFID is the The Linux Foundation project
func (pmm *Client) IsTheLinuxFoundation(projectSFID string) (bool, error) {
	f := logrus.Fields{
		"functionName": "project-service.IsTheLinuxFoundation",
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
		"functionName": "IsParentTheLinuxFoundation",
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
func (pmm *Client) EnableCLA(projectSFID string) error {
	tok, err := token.GetToken()
	if err != nil {
		return err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	projectDetails, err := pmm.getProject(projectSFID, clientAuth)
	if err != nil {
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
	return pmm.updateEnabledServices(projectSFID, enabledServices, clientAuth)
}

func (pmm *Client) updateEnabledServices(projectSFID string, enabledServices []string, clientAuth runtime.ClientAuthInfoWriter) error {
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
		return err
	}
	return err
}

// DisableCLA enables CLA service in project-service
func (pmm *Client) DisableCLA(projectSFID string) error {
	tok, err := token.GetToken()
	if err != nil {
		return err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	projectDetails, err := pmm.getProject(projectSFID, clientAuth)
	if err != nil {
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
	return pmm.updateEnabledServices(projectSFID, newEnabledServices, clientAuth)
}
