// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	organizationService "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
	userService "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service"

	acsService "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"

	"github.com/aws/aws-lambda-go/events"
	claEvents "github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	v2ProjectService "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
	"github.com/sirupsen/logrus"
)

// ProjectClaGroup is database model for projects_cla_group table
type ProjectClaGroup struct {
	ProjectSFID       string `json:"project_sfid"`
	ClaGroupID        string `json:"cla_group_id"`
	FoundationSFID    string `json:"foundation_sfid"`
	RepositoriesCount int64  `json:"repositories_count"`
}

//// ProjectServiceEnableCLAServiceHandler handles enabling the CLA Service attribute from the project service
//func (s *service) ProjectServiceEnableCLAServiceHandler(event events.DynamoDBEventRecord) error {
//	ctx := utils.NewContext()
//	f := logrus.Fields{
//		"functionName":   "dynamo_events.projects_cla_groups.ProjectServiceEnableCLAServiceHandler",
//		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
//		"eventID":        event.EventID,
//		"eventName":      event.EventName,
//		"eventSource":    event.EventSource,
//	}
//
//	log.WithFields(f).Debug("processing request")
//	var newProject ProjectClaGroup
//	err := unmarshalStreamImage(event.Change.NewImage, &newProject)
//	if err != nil {
//		log.WithFields(f).WithError(err).Warn("project decoding add event")
//		return err
//	}
//
//	f["projectSFID"] = newProject.ProjectSFID
//	f["claGroupID"] = newProject.ClaGroupID
//	f["foundationSFID"] = newProject.FoundationSFID
//
//	psc := v2ProjectService.GetClient()
//	log.WithFields(f).Debug("looking up project by SFID...")
//	projectDetails, prjerr := psc.GetProject(newProject.ProjectSFID)
//	if prjerr != nil {
//		log.WithError(err).Warnf("unable to get project details from SFID: %s", newProject.ProjectSFID)
//	}
//	projectName := newProject.ProjectSFID
//	if projectDetails != nil {
//		projectName = projectDetails.Name
//		f["projectName"] = projectName
//	}
//
//	start, _ := utils.CurrentTime()
//	log.WithFields(f).Debugf("enabling CLA service for project %s with ID: %s", projectName, newProject.ProjectSFID)
//	err = psc.EnableCLA(newProject.ProjectSFID)
//	if err != nil {
//		log.WithFields(f).WithError(err).Warn("enabling CLA service failed")
//		return err
//	}
//	finish, _ := utils.CurrentTime()
//	log.WithFields(f).Debugf("enabled CLA service for project %s with ID: %s", projectName, newProject.ProjectSFID)
//	log.WithFields(f).Debugf("enabling CLA service completed - took: %s", finish.Sub(start).String())
//
//	// Log the event
//	eventErr := s.eventsRepo.CreateEvent(&models.Event{
//		ContainsPII:            false,
//		EventData:              fmt.Sprintf("enabled CLA service for project: %s with ID: %s", projectName, newProject.ProjectSFID),
//		EventFoundationSFID:    newProject.FoundationSFID,
//		EventProjectExternalID: newProject.ProjectSFID,
//		EventProjectID:         newProject.ClaGroupID,
//		EventProjectName:       projectName,
//		EventProjectSFID:       newProject.ProjectSFID,
//		EventProjectSFName:     projectName,
//		EventSummary:           fmt.Sprintf("enabled CLA service for project: %s", projectName),
//		EventType:              claEvents.ProjectServiceCLAEnabled,
//		LfUsername:             "easycla system",
//		UserID:                 "easycla system",
//		UserName:               "easycla system",
//	})
//	if eventErr != nil {
//		log.WithFields(f).WithError(eventErr).Warn("problem logging event for enabling CLA service")
//		// Ok - don't fail for now
//	}
//
//	return nil
//}
//
//// ProjectServiceDisableCLAServiceHandler handles disabling/removing the CLA Service attribute from the project service
//func (s *service) ProjectServiceDisableCLAServiceHandler(event events.DynamoDBEventRecord) error {
//	ctx := utils.NewContext()
//	f := logrus.Fields{
//		"functionName":   "dynamo_events.projects_cla_groups.ProjectServiceDisableCLAServiceHandler",
//		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
//		"eventID":        event.EventID,
//		"eventName":      event.EventName,
//		"eventSource":    event.EventSource,
//	}
//
//	log.WithFields(f).Debug("processing request")
//	var oldProject ProjectClaGroup
//	err := unmarshalStreamImage(event.Change.OldImage, &oldProject)
//	if err != nil {
//		log.WithFields(f).WithError(err).Warn("problem unmarshalling stream image")
//		return err
//	}
//
//	// Add more fields for the logger
//	f["ProjectSFID"] = oldProject.ProjectSFID
//	f["ClaGroupID"] = oldProject.ClaGroupID
//	f["FoundationSFID"] = oldProject.FoundationSFID
//
//	psc := v2ProjectService.GetClient()
//	log.WithFields(f).Debug("looking up project by SFID...")
//	projectDetails, prjerr := psc.GetProject(oldProject.ProjectSFID)
//	if prjerr != nil {
//		log.WithError(err).Warnf("unable to get project details from SFID: %s", oldProject.ProjectSFID)
//	}
//	projectName := oldProject.ProjectSFID
//	if projectDetails != nil {
//		projectName = projectDetails.Name
//		f["projectName"] = projectName
//	}
//
//	// Gathering metrics - grab the time before the API call
//	before, _ := utils.CurrentTime()
//	log.WithFields(f).Debugf("disabling CLA service for project %s with ID: %s", projectName, oldProject.ProjectSFID)
//	err = psc.DisableCLA(oldProject.ProjectSFID)
//	if err != nil {
//		log.WithFields(f).WithError(err).Warn("disabling CLA service failed")
//		return err
//	}
//	log.WithFields(f).Debugf("disabled CLA service for project %s with ID: %s", projectName, oldProject.ProjectSFID)
//	log.WithFields(f).Debugf("disabling CLA service completed - took %s", time.Since(before).String())
//
//	// Log the event
//	eventErr := s.eventsRepo.CreateEvent(&models.Event{
//		ContainsPII:            false,
//		EventData:              fmt.Sprintf("disabled CLA service for project: %s with ID: %s", projectName, oldProject.ProjectSFID),
//		EventFoundationSFID:    oldProject.FoundationSFID,
//		EventProjectExternalID: oldProject.ProjectSFID,
//		EventProjectID:         oldProject.ClaGroupID,
//		EventProjectName:       projectName,
//		EventProjectSFID:       oldProject.ProjectSFID,
//		EventSummary:           fmt.Sprintf("disabled CLA service for project: %s", projectName),
//		EventType:              claEvents.ProjectServiceCLADisabled,
//		LfUsername:             "easycla system",
//		UserID:                 "easycla system",
//		UserName:               "easycla system",
//	})
//	if eventErr != nil {
//		log.WithFields(f).WithError(eventErr).Warn("problem logging event for disabling CLA service")
//		// Ok - don't fail for now
//	}
//
//	return nil
//}

func (s *service) ProjectUnenrolledDisableRepositoryHandler(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "dynamo_events.projects_cla_groups.ProjectUnenrolledDisableRepositoryHandler",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"eventID":        event.EventID,
		"eventName":      event.EventName,
		"eventSource":    event.EventSource,
	}

	log.WithFields(f).Debug("processing request")
	var oldProject ProjectClaGroup
	err := unmarshalStreamImage(event.Change.OldImage, &oldProject)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem unmarshalling stream image")
		return err
	}

	// Add more fields for the logger
	f["ProjectSFID"] = oldProject.ProjectSFID
	f["ClaGroupID"] = oldProject.ClaGroupID
	f["FoundationSFID"] = oldProject.FoundationSFID

	// Disable GitHub repos associated with this project
	enabled := true // only care about enabled repos
	gitHubRepos, githubRepoErr := s.repositoryService.GetRepositoryByProjectSFID(ctx, oldProject.ProjectSFID, &enabled)
	if githubRepoErr != nil {
		log.WithFields(f).WithError(githubRepoErr).Warn("problem listing github repositories by project sfid")
		return githubRepoErr
	}
	if gitHubRepos != nil && len(gitHubRepos.List) > 0 {
		log.WithFields(f).Debugf("discovered %d github repositories for project with sfid: %s - disabling repositories...",
			len(gitHubRepos.List), oldProject.ProjectSFID)

		// For each GitHub repository...
		for _, gitHubRepo := range gitHubRepos.List {
			log.WithFields(f).Debugf("disabling github repository: %s with id: %s for project with sfid: %s for CLA Group: %s",
				gitHubRepo.RepositoryName, gitHubRepo.RepositoryID, gitHubRepo.RepositoryProjectSfid, gitHubRepo.RepositoryClaGroupID)
			disableErr := s.repositoryService.DisableRepository(ctx, gitHubRepo.RepositoryID)
			if disableErr != nil {
				log.WithFields(f).WithError(disableErr).Warnf("problem disabling github repository: %s with id: %s", gitHubRepo.RepositoryName, gitHubRepo.RepositoryID)
				return disableErr
			}
		}
	} else {
		log.WithFields(f).Debugf("no github repositories for project with sfid: %s - nothing to disable",
			oldProject.ProjectSFID)
	}

	gerrits, gerritRepoErr := s.gerritService.GetGerritsByProjectSFID(ctx, oldProject.ProjectSFID)
	if gerritRepoErr != nil {
		log.WithFields(f).WithError(gerritRepoErr).Warn("problem listing gerrit repositories by project sfid")
		return gerritRepoErr
	}
	if gerrits != nil && len(gerrits.List) > 0 {
		log.WithFields(f).Debugf("discovered %d gerrit repositories for project with sfid: %s - deleting gerrit instances...",
			len(gerrits.List), oldProject.ProjectSFID)
		for _, gerritRepo := range gerrits.List {
			log.WithFields(f).Debugf("deleting gerrit instance: %s with id: %s for project with sfid: %s",
				gerritRepo.GerritName, gerritRepo.GerritID.String(), gerritRepo.ProjectSFID)
			gerritDeleteErr := s.gerritService.DeleteGerrit(ctx, gerritRepo.GerritID.String())
			if gerritDeleteErr != nil {
				log.WithFields(f).WithError(gerritDeleteErr).Warnf("problem deleting gerrit instance: %s with id: %s",
					gerritRepo.GerritName, gerritRepo.GerritID.String())
				return gerritDeleteErr
			}
		}
	} else {
		log.WithFields(f).Debugf("no gerrit instances for project with sfid: %s - nothing to delete",
			oldProject.ProjectSFID)
	}

	return nil
}

// AddCLAPermissions handles adding CLA permissions
func (s *service) AddCLAPermissions(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "dynamo_events.projects_cla_groups.AddCLAPermissions",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"eventID":        event.EventID,
		"eventName":      event.EventName,
		"eventSource":    event.EventSource,
	}

	log.WithFields(f).Debug("processing event")
	var newProject ProjectClaGroup
	err := unmarshalStreamImage(event.Change.NewImage, &newProject)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem unmarshalling stream image")
		return err
	}

	// Add more fields for the logger
	f["ProjectSFID"] = newProject.ProjectSFID
	f["ClaGroupID"] = newProject.ClaGroupID
	f["FoundationSFID"] = newProject.FoundationSFID

	// Add any relevant CLA Manager permissions for this CLA Group/Project SFID
	permErr := s.addCLAManagerPermissions(ctx, newProject.ClaGroupID, newProject.ProjectSFID)
	if permErr != nil {
		log.WithFields(f).WithError(permErr).Warn("problem adding CLA Manager permissions for projectSFID")
		// Ok - don't fail for now
	}

	// Add any relevant CLA Manager Designee permissions for this CLA Group/Project SFID
	permErr = s.addCLAManagerDesigneePermissions(ctx, newProject.ClaGroupID, newProject.FoundationSFID, newProject.ProjectSFID)
	if permErr != nil {
		log.WithFields(f).WithError(permErr).Warn("problem adding CLA Manager Designee permissions for projectSFID")
		// Ok - don't fail for now
	}

	return nil
}

// RemoveCLAPermissions handles removing existing CLA permissions
func (s *service) RemoveCLAPermissions(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "dynamo_events.projects_cla_groups.RemoveCLAPermissions",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"eventID":        event.EventID,
		"eventName":      event.EventName,
		"eventSource":    event.EventSource,
	}

	log.WithFields(f).Debug("processing event")
	var oldProject ProjectClaGroup
	err := unmarshalStreamImage(event.Change.OldImage, &oldProject)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem unmarshalling stream image")
		return err
	}

	// Add more fields for the logger
	f["ProjectSFID"] = oldProject.ProjectSFID
	f["ClaGroupID"] = oldProject.ClaGroupID
	f["FoundationSFID"] = oldProject.FoundationSFID

	// Remove any CLA related permissions
	permErr := s.removeCLAPermissions(ctx, oldProject.ProjectSFID)
	if permErr != nil {
		log.WithFields(f).WithError(permErr).Warn("problem removing CLA permissions for projectSFID")
		// Ok - don't fail for now
	}

	return nil
}

func (s *service) addCLAManagerDesigneePermissions(ctx context.Context, claGroupID, foundationSFID, projectSFID string) error {
	f := logrus.Fields{
		"functionName":   "dynamo_events.projects_cla_groups.addCLAManagerDesigneePermissions",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"projectSFID":    projectSFID,
	}

	// Lookup the project name
	log.WithFields(f).Debugf("looking up project by SFID: %s", projectSFID)
	psc := v2ProjectService.GetClient()
	projectModel, err := psc.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup project record by projectSFID")
	}
	projectName := ""
	if projectModel != nil {
		projectName = projectModel.Name
		f["projectName"] = projectName
	}

	//handle userscopes per project(users with Designee role)
	userScopes := make([]acsService.UserScope, 0)

	log.WithFields(f).Debug("adding CLA Manager Designee permissions...")
	// Check if signed at Foundation
	signedAtFoundationLevel, signedErr := s.projectService.SignedAtFoundationLevel(ctx, foundationSFID)
	if signedErr != nil {
		log.WithFields(f).Warnf("Problem getting level of CLA Group Signature for CLAGroup: %s ", claGroupID)
		return signedErr
	}
	orgClient := organizationService.GetClient()
	acsClient := acsService.GetClient()

	log.WithFields(f).Debugf("locating role ID for role: %s", utils.CLADesigneeRole)
	claManagerDesigneeRoleID, roleErr := acsClient.GetRoleID(utils.CLADesigneeRole)
	if roleErr != nil {
		log.WithFields(f).Warnf("problem looking up details for role: %s, error: %+v", utils.CLADesigneeRole, roleErr)
		return roleErr
	}

	if signedAtFoundationLevel {
		// Determine if any users have the CLA Manager Designee Role at the Foundation Level
		log.WithFields(f).Debugf("Getting users with role: %s for foundationSFID: %s  ", utils.CLADesigneeRole, foundationSFID)
		foundationUserScopes, err := acsClient.GetProjectRoleUsersScopes(foundationSFID, utils.CLADesigneeRole)
		if err != nil {
			log.WithFields(f).Warnf("problem getting userscopes for foundationSFID: %s and role: %s ", foundationSFID, utils.CLADesigneeRole)
			return err
		}
		//Tabulating userscopes for new ProjectSFID assignment
		userScopes = append(userScopes, foundationUserScopes...)
		log.WithFields(f).Debugf("Found userscopes: %+v for foundationSFID: %s ", userScopes, foundationSFID)

	} else {
		// Signed at Project level Use case
		pcgs, err := s.projectsClaGroupRepo.GetProjectsIdsForClaGroup(ctx, claGroupID)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("problem getting project cla Groups for claGroupID: %s", claGroupID)
			return err
		}
		for _, pcg := range pcgs {
			//ignore newly added project
			if pcg.ProjectSFID == projectSFID {
				continue
			}
			projectUserScopes, err := acsClient.GetProjectRoleUsersScopes(pcg.ProjectSFID, utils.CLADesigneeRole)
			if err != nil {
				log.WithFields(f).Warnf("problem getting userscopes for projectSFID: %s ", pcg.ProjectSFID)
				return err
			}
			//Tabulating userscopes for new ProjectSFID assignment
			userScopes = append(userScopes, projectUserScopes...)
			log.WithFields(f).Debugf("Found userscopes : %+v for project: %s ", userScopes, pcg.ProjectSFID)
		}

	}

	if len(userScopes) > 0 {
		log.WithFields(f).Debugf("Identified users: %+v to be updated with role: %s for project : %s ", userScopes, utils.CLADesigneeRole, projectSFID)
		// If so, for each user, add the CLA Manager Designee role for this projectSFID
		var wg sync.WaitGroup
		wg.Add(len(userScopes))

		for _, userScope := range userScopes {
			go func(userScope acsService.UserScope) {
				defer wg.Done()

				orgID := strings.Split(userScope.ObjectID, "|")[1]
				email := userScope.Email

				// Lookup the organization name
				log.WithFields(f).Debugf("looking up organization by SFID: %s", orgID)
				orgModel, orgLookupErr := orgClient.GetOrganization(ctx, orgID)
				if orgLookupErr != nil {
					log.WithFields(f).WithError(orgLookupErr).Warnf("unable to lookup organization record by organziation SFID: %s", orgID)
				}
				orgName := ""
				if orgModel != nil {
					orgName = orgModel.Name
					log.WithFields(f).Debugf("found organization by SFID: %s - Name: %s", orgID, orgName)
				}

				log.WithFields(f).Debugf("assiging role: %s to user %s with email %s for project: %s, company: %s...",
					utils.CLAManagerRole, userScope.Username, email, projectSFID, orgID)
				roleErr := orgClient.CreateOrgUserRoleOrgScopeProjectOrg(ctx, email, projectSFID, orgID, claManagerDesigneeRoleID)
				if roleErr != nil {
					log.WithFields(f).WithError(roleErr).Warnf("%s, role assignment for user %s failed for this project: %s, company: %s ",
						utils.CLADesigneeRole, email, projectSFID, orgID)
					return
				}

				msgSummary := fmt.Sprintf("assigned role: %s to user %s with email %s for project: %s, company: %s",
					utils.CLAManagerRole, userScope.Username, email, projectName, orgName)
				msg := fmt.Sprintf("assigned role: %s to user %s with email %s for project: %s with SFID: %s, company: %s with SFID: %s",
					utils.CLAManagerRole, userScope.Username, email, projectName, projectSFID, orgName, orgID)
				log.WithFields(f).Debug(msg)
				// Log the event
				eventErr := s.eventsRepo.CreateEvent(&models.Event{
					ContainsPII:      false,
					EventCompanySFID: orgID,
					EventData:        msg,
					EventProjectSFID: projectSFID,
					EventProjectID:   claGroupID,
					EventProjectName: projectName,
					EventCompanyName: orgName,
					EventSummary:     msgSummary,
					EventType:        claEvents.AssignUserRoleScopeType,
					LfUsername:       "easycla system",
					UserID:           "easycla system",
					UserName:         "easycla system",
				})
				if eventErr != nil {
					log.WithFields(f).WithError(eventErr).Warnf("unable to create event log entry for %s with msg: %s", claEvents.AssignUserRoleScopeType, msg)
				}
			}(userScope)
		}

		// Wait for the goroutines to finish
		log.WithFields(f).Debugf("waiting for role:%s  assignment to complete for project: %s ", utils.CLADesigneeRole, projectSFID)
		wg.Wait()
	}

	return nil
}

// addCLAManagerPermissions handles adding the CLA Manager permissions for the specified SF project
func (s *service) addCLAManagerPermissions(ctx context.Context, claGroupID, projectSFID string) error {
	f := logrus.Fields{
		"functionName":   "dynamo_events.projects_cla_groups.addCLAManagerPermissions",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
		"claGroupID":     claGroupID,
	}
	log.WithFields(f).Debug("adding CLA Manager permissions...")

	// Lookup the project name
	log.WithFields(f).Debugf("looking up project by SFID: %s", projectSFID)
	psc := v2ProjectService.GetClient()
	projectModel, err := psc.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup project record by projectSFID")
	}
	projectName := ""
	if projectModel != nil {
		projectName = projectModel.Name
		f["projectName"] = projectName
	}

	sigModels, err := s.signatureRepo.GetProjectSignatures(ctx, signatures.GetProjectSignaturesParams{
		ClaType:   aws.String(utils.ClaTypeCCLA),
		PageSize:  aws.Int64(1000),
		ProjectID: claGroupID,
	})
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem querying CCLA signatures for CLA Group - skipping %s role review/assignment for this project", utils.CLAManagerRole)
		return err
	}
	if sigModels == nil || len(sigModels.Signatures) == 0 {
		log.WithFields(f).WithError(err).Warnf("no signatures found CLA Group - unable to determine existing CLA Managers - skipping %s role review/assignment for this project", utils.CLAManagerRole)
		return err
	}

	// ACS Client
	acsClient := acsService.GetClient()
	log.WithFields(f).Debugf("locating role ID for role: %s", utils.CLAManagerRole)
	claManagerRoleID, roleErr := acsClient.GetRoleID(utils.CLAManagerRole)
	if roleErr != nil {
		log.WithFields(f).Warnf("problem looking up details for role: %s, error: %+v", utils.CLAManagerRole, roleErr)
		return roleErr
	}
	orgClient := organizationService.GetClient()
	userClient := userService.GetClient()

	// For each signature...
	for _, sig := range sigModels.Signatures {

		// Make sure we can load the company and grab the SFID
		sig := sig
		companyInternalID := sig.SignatureReferenceID
		log.WithFields(f).Debugf("locating company by internal ID: %s", companyInternalID)
		companyModel, err := s.companyRepo.GetCompany(ctx, companyInternalID)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("problem loading company by internal ID: %s - skipping %s role review/assignment for this project", companyInternalID, utils.CLAManagerRole)
			continue
		}
		if companyModel == nil || companyModel.CompanyExternalID == "" {
			log.WithFields(f).WithError(err).Warnf("problem loading company ID: %s or external SFID for company not set - skipping %s role review/assignment for this project", companyInternalID, utils.CLAManagerRole)
			continue
		}
		log.WithFields(f).Debugf("loaded company by internal ID: %s with name: %s", companyInternalID, companyModel.CompanyName)
		companySFID := companyModel.CompanyExternalID

		// Make sure we can load the CLA Manger list (ACL)
		if len(sig.SignatureACL) == 0 {
			log.WithFields(f).Warnf("no CLA Manager list (acl) established for signature %s - skipping %s role review/assigment for this project", sig.SignatureID, utils.CLAManagerRole)
			continue
		}
		existingCLAManagers := sig.SignatureACL

		var wg sync.WaitGroup
		wg.Add(len(existingCLAManagers))

		// For each CLA manager for this company...
		log.WithFields(f).Debugf("processing %d CLA managers for company ID: %s/%s with name: %s", len(existingCLAManagers), companyInternalID, companySFID, companyModel.CompanyName)
		for _, signatureUserModel := range existingCLAManagers {
			// handle unpredictability with addresses o0f different signatureUserModel
			signatureUserModel := signatureUserModel
			go func(signatureUserModel models.User) {
				defer wg.Done()

				log.WithFields(f).Debugf("looking up existing CLA manager by LF username: %s...", signatureUserModel.LfUsername)
				userModel, userLookupErr := userClient.GetUserByUsername(signatureUserModel.LfUsername)
				if userLookupErr != nil {
					log.WithFields(f).WithError(userLookupErr).Warnf("unable to lookup user %s - skipping %s role review/assigment for this project",
						signatureUserModel.LfUsername, utils.CLAManagerRole)
					return
				}
				if userModel == nil || userModel.ID == "" || userClient.GetPrimaryEmail(userModel) == "" {
					log.WithFields(f).Warnf("unable to lookup user %s - user object is empty or missing either the ID or email - skipping %s role review/assigment for project: %s, company: %s",
						signatureUserModel.LfUsername, utils.CLAManagerRole, projectSFID, companySFID)
					return
				}

				// Determine if the user already has the cla-manager role scope for this Project and Company
				hasRole, roleLookupErr := orgClient.IsUserHaveRoleScope(ctx, utils.CLAManagerRole, userModel.ID, companySFID, projectSFID)
				if roleLookupErr != nil {
					log.WithFields(f).WithError(roleLookupErr).Warnf("unable to lookup role scope %s for user %s/%s - skipping %s role review/assigment for this project",
						utils.CLAManagerRole, signatureUserModel.LfUsername, userModel.ID, utils.CLAManagerRole)
					return
				}

				// Does the user already have the cla-manager role?
				if hasRole {
					log.WithFields(f).Debugf("user %s/%s already has role %s for the project %s and organization %s - skipping assignment",
						signatureUserModel.LfUsername, userModel.ID, utils.CLAManagerRole, projectSFID, companySFID)
					// Nothing to do here - move along...
					return
				}

				// Finally....assign the role to this user
				log.WithFields(f).Debugf("assiging role: %s to user %s/%s/%s for project: %s, company: %s...",
					utils.CLAManagerRole, signatureUserModel.LfUsername, userModel.ID, userClient.GetPrimaryEmail(userModel), projectSFID, companySFID)
				roleErr := orgClient.CreateOrgUserRoleOrgScopeProjectOrg(ctx, userClient.GetPrimaryEmail(userModel), projectSFID, companySFID, claManagerRoleID)
				if roleErr != nil {
					log.WithFields(f).WithError(roleErr).Warnf("%s, role assignment for user user %s/%s/%s failed for this project: %s, company: %s",
						utils.CLAManagerRole, signatureUserModel.LfUsername, userModel.ID, userClient.GetPrimaryEmail(userModel), projectSFID, companySFID)
					return
				}
				msg := fmt.Sprintf("assigned role: %s to user %s/%s/%s for project: %s with SFID:%s, company: %s with SFID: %s",
					utils.CLAManagerRole, signatureUserModel.LfUsername, userModel.ID, userClient.GetPrimaryEmail(userModel), projectName, projectSFID, companyModel.CompanyName, companySFID)
				msgSummary := fmt.Sprintf("assigned role: %s to user %s/%s/%s for project: %s, company: %s",
					utils.CLAManagerRole, signatureUserModel.LfUsername, userModel.ID, userClient.GetPrimaryEmail(userModel), projectName, companyModel.CompanyName)
				log.WithFields(f).Debug(msg)
				// Log the event
				eventErr := s.eventsRepo.CreateEvent(&models.Event{
					ContainsPII:      false,
					EventCompanyName: companyModel.CompanyName,
					EventCompanySFID: companySFID,
					EventData:        msg,
					EventProjectID:   claGroupID,
					EventProjectName: projectName,
					EventProjectSFID: projectSFID,
					EventSummary:     msgSummary,
					EventType:        claEvents.AssignUserRoleScopeType,
					LfUsername:       "easycla system",
					UserID:           "easycla system",
					UserName:         "easycla system",
				})
				if eventErr != nil {
					log.WithFields(f).WithError(eventErr).Warnf("unable to create event log entry for %s with msg: %s", claEvents.AssignUserRoleScopeType, msg)
				}
			}(signatureUserModel)
		}

		// Wait for the go routines to finish
		log.WithFields(f).Debugf("waiting for role assignment to complete for %d project: %s", len(sigModels.Signatures), projectSFID)
		wg.Wait()
	}

	return nil
}

// removeCLAPermissions handles removing CLA Group (projects table) permissions for the specified project
func (s *service) removeCLAPermissions(ctx context.Context, projectSFID string) error {
	f := logrus.Fields{
		"functionName":   "dynamo_events.projects_cla_groups.removeCLAPermissions",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    projectSFID,
	}
	log.WithFields(f).Debug("removing CLA permissions...")

	// Lookup the project name
	log.WithFields(f).Debugf("looking up project by SFID: %s", projectSFID)
	psc := v2ProjectService.GetClient()
	projectModel, err := psc.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup project record by projectSFID")
	}
	projectName := ""
	if projectModel != nil {
		projectName = projectModel.Name
		f["projectName"] = projectName
	}

	client := acsService.GetClient()
	roleNames := []string{utils.CLAManagerRole, utils.CLADesigneeRole, utils.CLASignatoryRole}

	log.WithFields(f).Debugf("removing roles: %s for all users for project: %s", strings.Join(roleNames, ","), projectSFID)
	err = client.RemoveCLAUserRolesByProject(projectSFID, roleNames)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem removing CLA user roles by projectSFID")
	}
	msg := fmt.Sprintf("removed roles: %s for all users for project: %s", strings.Join(roleNames, ","), projectSFID)
	log.WithFields(f).Debug(msg)

	// Log the event
	eventErr := s.eventsRepo.CreateEvent(&models.Event{
		ContainsPII:      false,
		EventData:        msg,
		EventProjectName: projectName,
		EventProjectSFID: projectSFID,
		EventSummary:     msg,
		EventType:        claEvents.RemoveUserRoleScopeType,
		LfUsername:       "easycla system",
		UserID:           "easycla system",
		UserName:         "easycla system",
	})
	if eventErr != nil {
		log.WithFields(f).WithError(eventErr).Warnf("unable to create event log entry for %s with msg: %s", claEvents.RemoveUserRoleScopeType, msg)
	}

	return err
}

// removeCLAPermissionsByProjectOrganizationRole handles removal of the specified role for the given SF Project and SF Organization
func (s *service) removeCLAPermissionsByProjectOrganizationRole(ctx context.Context, projectSFID, organizationSFID string, roleNames []string) error {
	f := logrus.Fields{
		"functionName":     "dynamo_events.projects_cla_groups.removeCLAPermissionsByProjectOrganizationRole",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"projectSFID":      projectSFID,
		"organizationSFID": organizationSFID,
		"roleNames":        strings.Join(roleNames, ","),
	}

	// Lookup the project name
	log.WithFields(f).Debugf("looking up project by SFID: %s", projectSFID)
	psc := v2ProjectService.GetClient()
	projectModel, err := psc.GetProject(projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup project record by projectSFID")
	}
	projectName := ""
	if projectModel != nil {
		projectName = projectModel.Name
		f["projectName"] = projectName
	}

	// Lookup the organization name
	log.WithFields(f).Debugf("looking up organization by SFID: %s", organizationSFID)
	orgClient := organizationService.GetClient()
	orgModel, orgLookupErr := orgClient.GetOrganization(ctx, organizationSFID)
	if orgLookupErr != nil {
		log.WithFields(f).WithError(orgLookupErr).Warnf("unable to lookup organization record by organziation SFID: %s", organizationSFID)
	}
	orgName := ""
	if orgModel != nil {
		orgName = orgModel.Name
		log.WithFields(f).Debugf("found organization by SFID: %s - Name: %s", organizationSFID, orgName)
	}

	log.WithFields(f).Debugf("removing roles: %s for all users for project: %s, companay: %s", strings.Join(roleNames, ","), projectSFID, organizationSFID)
	client := acsService.GetClient()
	err = client.RemoveCLAUserRolesByProjectOrganization(projectSFID, organizationSFID, roleNames)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem removing CLA user roles by projectSFID and organizationSFID")
	}
	msg := fmt.Sprintf("removed roles: %s for all users for project: %s, companay: %s", strings.Join(roleNames, ","), projectSFID, organizationSFID)
	log.WithFields(f).Debug(msg)

	// Log the event
	eventErr := s.eventsRepo.CreateEvent(&models.Event{
		ContainsPII:      false,
		EventCompanySFID: organizationSFID,
		EventData:        msg,
		EventProjectName: projectName,
		EventProjectSFID: projectSFID,
		EventSummary:     msg,
		EventType:        claEvents.RemoveUserRoleScopeType,
		LfUsername:       "easycla system",
		UserID:           "easycla system",
		UserName:         "easycla system",
	})
	if eventErr != nil {
		log.WithFields(f).WithError(eventErr).Warnf("unable to create event log entry for %s with msg: %s", claEvents.RemoveUserRoleScopeType, msg)
	}

	return err
}
