// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_groups

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/events"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	v2ProjectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"
	psproject "github.com/communitybridge/easycla/cla-backend-go/v2/project-service/client/project"

	"github.com/sirupsen/logrus"
)

// validateClaGroupInput validates the cla group input. It there is validation error then it returns the error
// if foundation_sfid is root project i.e project without parent and if it does not have subprojects then return boolean
// flag would be true
func (s *service) validateClaGroupInput(ctx context.Context, input *models.CreateClaGroupInput) (bool, error) {
	if input.FoundationSfid == nil {
		return false, fmt.Errorf("missing foundation ID parameter")
	}
	if input.ClaGroupName == nil {
		return false, fmt.Errorf("missing CLA Group parameter")
	}
	foundationSFID := *input.FoundationSfid
	claGroupName := *input.ClaGroupName

	f := logrus.Fields{
		"functionName":        "validateClaGroupInput",
		utils.XREQUESTID:      ctx.Value(utils.XREQUESTID),
		"ClaGroupName":        claGroupName,
		"ClaGroupDescription": input.ClaGroupDescription,
		"FoundationSfid":      foundationSFID,
		"IclaEnabled":         *input.IclaEnabled,
		"CclaEnabled":         *input.CclaEnabled,
		"CclaRequiresIcla":    *input.CclaRequiresIcla,
		"ProjectSfidList":     strings.Join(input.ProjectSfidList, ","),
	}

	log.WithFields(f).Debug("validating CLA Group input...")
	// First, check that all the required flags are set and make sense
	if foundationSFID == "" {
		return false, fmt.Errorf("bad request: foundation_sfid cannot be empty")
	}
	if !*input.IclaEnabled && !*input.CclaEnabled {
		return false, fmt.Errorf("bad request: can not create cla group with both icla and ccla disabled")
	}
	if *input.CclaRequiresIcla {
		if !(*input.IclaEnabled && *input.CclaEnabled) {
			return false, fmt.Errorf("bad request: ccla_requires_icla can not be enabled if one of icla/ccla is disabled")
		}
	}

	// Ensure we don't have a duplicate CLA Group Name
	log.WithFields(f).Debug("checking for duplicate CLA Group name...")
	claGroupModel, err := s.v1ProjectService.GetCLAGroupByName(ctx, claGroupName)
	if err != nil {
		return false, err
	}
	if claGroupModel != nil {
		return false, fmt.Errorf("bad request: cla_group with name '%s' already exists", claGroupName)
	}

	log.WithFields(f).Debug("looking up project in project service by Foundation SFID...")
	// Use the Platform Project Service API to lookup the Foundation details
	psc := v2ProjectService.GetClient()
	foundationProjectDetails, err := psc.GetProject(foundationSFID)
	if err != nil {
		if _, ok := err.(*psproject.GetProjectNotFound); ok {
			return false, fmt.Errorf("bad request: invalid foundation_sfid - unable to locate foundation by ID: %s", foundationSFID)
		}
		return false, err
	}

	// Look up any existing configuration with this foundation SFID in our database...
	log.WithFields(f).Debug("loading existing project IDs by foundation SFID...")
	claGroupProjectModels, lookupErr := s.projectsClaGroupsRepo.GetProjectsIdsForFoundation(foundationSFID)
	if lookupErr != nil {
		log.WithFields(f).Warnf("problem looking up foundation level CLA group using foundation ID: %s, error: %+v", foundationSFID, lookupErr)
		return false, lookupErr
	}

	// Do we have an existing Foundation Level CLA Group? We can't create a new CLA Group if we have an existing
	// Foundation Level CLA Group setup
	log.WithFields(f).Debug("checking for existing Foundation Level CLA Groups...")
	for _, projectCLAGroupModel := range claGroupProjectModels {
		// Do we have an existing Foundation Level setup? No need to check the input foundation SFID against this list
		// since we did the query based on the foundation SFID.
		if projectCLAGroupModel.FoundationSFID == projectCLAGroupModel.ProjectSFID {
			msg := fmt.Sprintf("found existing foundation level CLA Group using foundation ID: %s - can't add new CLA Groups under this configuration", foundationSFID)
			log.WithFields(f).Warn(msg)
			return false, errors.New(msg)
		}
	}
	log.WithFields(f).Debug("no existing Foundation Level CLA Groups found...")

	// Are we trying to create a Foundation Level CLA Group, but one or more of the sub-projects already in a CLA Group?
	if isFoundationIDInList(foundationSFID, input.ProjectSfidList) {
		log.WithFields(f).Debug("we have a Foundation Level CLA Group request - checking if any CLA Groups include sub-projects from the foundation...")
		// Only do this comparison if we have a input foundation level CLA group situation...
		exists, existingProjectIDs := anySubProjectsAlreadyConfigured(input.ProjectSfidList, claGroupProjectModels)
		if exists {
			// So, we have the situation where the input is a foundation level CLA (meaning this applies to all sub-projects)
			// but we have at least one project that is currently in an existing CLA group (other than the one just provided)
			// so....we don't allow this (we don't migrate or merge - just reject)
			msg := fmt.Sprintf("found existing sub-project(s) under foundation ID: %s which are already associated with an existing CLA Group - unable to create a new foundationl level CLA Group - project IDs: %+v", foundationSFID, existingProjectIDs)
			log.WithFields(f).Warn(msg)
			return false, errors.New(msg)
		}

		// Do we have any existing CLA Groups associated with this foundation?  Since this is a Foundation Level CLA
		// Group, we can't create it if we have existing CLA groups already in place for this foundation
		if len(claGroupProjectModels) > 0 {
			// Create a string array to hold the existing CLA details for the error message
			var claGroupString []string
			for _, claGroupProjectModel := range claGroupProjectModels {
				claGroupString = append(claGroupString, fmt.Sprintf("%s - %s", claGroupProjectModel.ClaGroupName, claGroupProjectModel.ClaGroupID))
			}
			msg := fmt.Sprintf("found existing CLA Groups under foundation ID: %s - unable to create a new foundationl level CLA Group - existing CLA Group(s): [%s]",
				foundationSFID, strings.Join(claGroupString, ","))
			log.WithFields(f).Warn(msg)
			return false, errors.New(msg)
		}
	}
	log.WithFields(f).Debug("we have a Foundation Level CLA Group request - good, no CLA Groups include sub-projects from the foundation...")

	// Is our parent the LF project?
	log.WithFields(f).Debugf("looking up LF parent project record...")
	isLFParent, err := psc.IsTheLinuxFoundation(foundationProjectDetails.Parent)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("validation failure - unable to lookup %s or %s project", utils.TheLinuxFoundation, utils.LFProjectsLLC)
		return false, err
	}

	// If the foundation details in the platform project service indicates that this foundation has no parent or no
	// children/sub-project... (stand alone project situation)
	log.WithFields(f).Debug("checking to see if we have a standalone project...")
	if (foundationProjectDetails.Parent == "" || isLFParent) && len(foundationProjectDetails.Projects) == 0 {
		log.WithFields(f).Debug("we have a standalone project...")
		// Did the user actually pass in any projects?  If none - add the foundation ID to the list and return to
		// indicate it is a "standalone project"
		if len(input.ProjectSfidList) == 0 {
			// Add the foundation ID into the project list - caller should do this, but we'll add for compatibility
			log.WithFields(f).Debug("no projects provided - adding foundation ID to the list of projects")
			input.ProjectSfidList = append(input.ProjectSfidList, foundationSFID)
			log.WithFields(f).Debug("foundation doesn't have a parent or any children project in SF - this is a standalone project")
			return true, nil
		}

		// If they provided a project in the list - this is ok, as long as it's the foundation ID
		if len(input.ProjectSfidList) == 1 && isFoundationIDInList(foundationSFID, input.ProjectSfidList) {
			log.WithFields(f).Debug("foundation doesn't have a parent or any children project in SF - this is a standalone project")
			return true, nil
		}

		// oops, not allowed - send error
		log.WithFields(f).Warn("this project does not have subprojects defined in SF but some are provided as input")
		return false, fmt.Errorf("bad request: invalid project_sfid_list. This project does not have subprojects defined in SF but some are provided as input")
	}

	// Any of the projects in an existing CLA Group?
	log.WithFields(f).Debug("validating enrolled projects...")
	err = s.validateEnrollProjectsInput(ctx, foundationSFID, input.ProjectSfidList)
	if err != nil {
		return false, err
	}
	return false, nil
}

func (s *service) validateEnrollProjectsInput(ctx context.Context, foundationSFID string, projectSFIDList []string) error {
	f := logrus.Fields{
		"functionName":    "validateEnrollProjectsInput",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"foundationSFID":  foundationSFID,
		"projectSFIDList": strings.Join(projectSFIDList, ","),
	}

	psc := v2ProjectService.GetClient()

	if len(projectSFIDList) == 0 {
		log.WithFields(f).Warn("validation failure - there should be at least one subproject associated...")
		return fmt.Errorf("bad request: there should be at least one subproject associated")
	}

	// fetch the foundation model details from the platform project service which includes a list of its sub projects
	foundationProjectDetails, err := psc.GetProject(foundationSFID)
	if err != nil {
		log.WithFields(f).Warnf("validation failure - problem fetching project details from project service, error: %+v", err)
		return err
	}

	// Is our parent the LF project?
	log.WithFields(f).Debugf("looking up LF parent project record...")
	isLFParent, err := psc.IsTheLinuxFoundation(foundationProjectDetails.Parent)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("validation failure - unable to lookup %s or %s project", utils.TheLinuxFoundation, utils.LFProjectsLLC)
		return err
	}

	for _, projectSFID := range projectSFIDList {
		projectDetails, projErr := psc.GetProject(projectSFID)
		if projErr != nil {
			return err
		}

		if foundationProjectDetails.Parent != "" && (!isLFParent && (foundationProjectDetails.ProjectType == utils.ProjectTypeProjectGroup && projectDetails.ProjectType != utils.ProjectTypeProjectGroup)) {
			msg := fmt.Sprintf("input validation failure - foundationSFID: %s , foundationType: %s , projectSFID: %s , projectType: %s ",
				foundationProjectDetails.Parent, foundationProjectDetails.ProjectType, projectSFID, projectDetails.ProjectType)
			log.WithFields(f).Warnf(msg)
			return fmt.Errorf(msg)
		}

	}

	// Check to see if all the provided enrolled projects are part of this foundation
	foundationProjectIDList := utils.NewStringSet()
	for _, pr := range foundationProjectDetails.Projects {
		foundationProjectIDList.Add(pr.ID)
	}
	invalidProjectSFIDs := utils.NewStringSet()
	for _, projectSFID := range projectSFIDList {
		// Ok to have foundation ID in the project list - this means it's a Foundation Level CLA Group
		if foundationSFID == projectSFID {
			continue
		}

		// If the input/provided project ID is not in the SF project list...
		if !foundationProjectIDList.Include(projectSFID) {
			invalidProjectSFIDs.Add(projectSFID)
		}
	}

	if invalidProjectSFIDs.Length() != 0 {
		log.WithFields(f).Warnf("validation failure - provided projects are not under the SF foundation: %+v", invalidProjectSFIDs.List())
		return fmt.Errorf("bad request: invalid project_sfid: %+v. One or more provided projects are not under the SF foundation", invalidProjectSFIDs.List())
	}

	// check if projects are not already enabled
	enabledProjects, err := s.projectsClaGroupsRepo.GetProjectsIdsForFoundation(foundationSFID)
	if err != nil {
		return err
	}
	enabledProjectList := utils.NewStringSet()
	for _, pr := range enabledProjects {
		enabledProjectList.Add(pr.ProjectSFID)
	}
	invalidProjectSFIDs = utils.NewStringSet()
	for _, projectSFID := range projectSFIDList {
		// Ok to have foundation ID in the project list - no need to check if it's already in the sub-project enabled list
		if foundationSFID == projectSFID {
			continue
		}

		if enabledProjectList.Include(projectSFID) {
			invalidProjectSFIDs.Add(projectSFID)
		}
	}
	if invalidProjectSFIDs.Length() != 0 {
		log.WithFields(f).Warnf("validation failure - projects are already enrolled in an existing CLA Group: %+v", invalidProjectSFIDs.List())
		return fmt.Errorf("bad request: invalid project_sfid provided: %v. One or more of the provided projects are already enrolled in an existing cla_group", invalidProjectSFIDs.List())
	}

	return nil
}

func (s *service) validateUnenrollProjectsInput(ctx context.Context, foundationSFID string, projectSFIDList []string) error {
	f := logrus.Fields{
		"functionName":    "validateUnenrollProjectsInput",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"foundationSFID":  foundationSFID,
		"projectSFIDList": strings.Join(projectSFIDList, ","),
	}

	psc := v2ProjectService.GetClient()

	if len(projectSFIDList) == 0 {
		log.WithFields(f).Warn("validation failure - there should be at least one subproject associated...")
		return fmt.Errorf("bad request: there should be at least one subproject associated")
	}
	// Comment out the below as we want to support project-level projects
	/* log.WithFields(f).Debug("checking to see if foundation is in project list...")
	if !isFoundationIDInList(foundationSFID, projectSFIDList) {
		log.WithFields(f).Warn("validation failure - unable to unenroll Project Group from CLA Group")
		return fmt.Errorf("bad request: unable to unenroll Project Group from CLA Group")
	} */

	// fetch the foundation model details from the platform project service which includes a list of its sub projects
	foundationProjectDetails, err := psc.GetProject(foundationSFID)
	if err != nil {
		log.WithFields(f).Warnf("validation failure - problem fetching project details from project service, error: %+v", err)
		return err
	}

	// Is our parent the LF project?
	log.WithFields(f).Debugf("looking up LF parent project record...")
	isLFParent, err := psc.IsTheLinuxFoundation(foundationProjectDetails.Parent)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("validation failure - unable to lookup %s or %s project", utils.TheLinuxFoundation, utils.LFProjectsLLC)
		return err
	}

	for _, projectSFID := range projectSFIDList {
		projectDetails, projErr := psc.GetProject(projectSFID)
		if projErr != nil {
			return err
		}

		if foundationProjectDetails.Parent != "" && (!isLFParent && (foundationProjectDetails.ProjectType == utils.ProjectTypeProjectGroup && projectDetails.ProjectType != utils.ProjectTypeProjectGroup)) {
			msg := fmt.Sprintf("input validation failure - foundationSFID: %s , foundationType: %s , projectSFID: %s , projectType: %s ",
				foundationProjectDetails.Parent, foundationProjectDetails.ProjectType, projectSFID, projectDetails.ProjectType)
			log.WithFields(f).Warnf(msg)
			return fmt.Errorf(msg)
		}

	}

	// Comment out the below as we want to support stand-alone projects
	/* if len(foundationProjectDetails.Projects) == 0 {
		log.WithFields(f).Warn("validation failure - project does not have any subprojects")
		return fmt.Errorf("bad request: invalid input to enroll projects. project does not have any subprojects")
	} */

	// Check to see if all the provided enrolled projects are part of this foundation
	foundationProjectIDList := utils.NewStringSet()
	for _, pr := range foundationProjectDetails.Projects {
		foundationProjectIDList.Add(pr.ID)
	}
	invalidProjectSFIDs := utils.NewStringSet()
	for _, projectSFID := range projectSFIDList {
		// Ok to have foundation ID in the project list - this means it's a Foundation Level CLA Group
		if foundationSFID == projectSFID {
			continue
		}

		// If the input/provided project ID is not in the SF project list...
		if !foundationProjectIDList.Include(projectSFID) {
			invalidProjectSFIDs.Add(projectSFID)
		}
	}

	if invalidProjectSFIDs.Length() != 0 {
		log.WithFields(f).Warnf("validation failure - provided projects are not under the SF foundation: %+v", invalidProjectSFIDs.List())
		return fmt.Errorf("bad request: invalid project_sfid: %+v. One or more of the provided projects are not under the SF foundation", invalidProjectSFIDs.List())
	}

	// check if projects are already enrolled/enabled
	enabledProjects, err := s.projectsClaGroupsRepo.GetProjectsIdsForFoundation(foundationSFID)
	if err != nil {
		return err
	}
	enabledProjectList := utils.NewStringSet()
	for _, pr := range enabledProjects {
		enabledProjectList.Add(pr.ProjectSFID)
	}
	invalidProjectSFIDs = utils.NewStringSet()
	for _, projectSFID := range projectSFIDList {
		// Ok to have foundation ID in the project list - no need to check if it's already in the sub-project enabled list
		if foundationSFID == projectSFID {
			continue
		}

		// If one of our provided project IDs are not already in our list
		if !enabledProjectList.Include(projectSFID) {
			invalidProjectSFIDs.Add(projectSFID)
		}
	}

	if invalidProjectSFIDs.Length() != 0 {
		log.WithFields(f).Warnf("validation failure - projects are not enrolled in an existing CLA Group: %+v", invalidProjectSFIDs.List())
		return fmt.Errorf("bad request: invalid project_sfid provided: %v. One or more of the provided projects are not enrolled in an existing cla_group", invalidProjectSFIDs.List())
	}

	return nil
}

func (s *service) AssociateCLAGroupWithProjects(ctx context.Context, request *AssociateCLAGroupWithProjectsModel) error {
	f := logrus.Fields{
		"functionName":    "AssociateCLAGroupWithProjects",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"authUserName":    request.AuthUser.UserName,
		"authUserEmail":   request.AuthUser.Email,
		"claGroupID":      request.CLAGroupID,
		"foundationSFID":  request.FoundationSFID,
		"projectSFIDList": strings.Join(request.ProjectSFIDList, ","),
	}

	// Associate the CLA Group with the project list in a go routine
	var errorList []error
	var wg sync.WaitGroup
	wg.Add(len(request.ProjectSFIDList))

	for _, projectSFID := range request.ProjectSFIDList {
		// Invoke the go routine - any errors will be handled below
		go func(sfid string) {
			defer wg.Done()
			log.WithFields(f).Debugf("associating cla_group with project: %s", sfid)
			err := s.projectsClaGroupsRepo.AssociateClaGroupWithProject(request.CLAGroupID, sfid, request.FoundationSFID)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("associating cla_group with project: %s failed", sfid)
				log.WithFields(f).Debug("deleting stale entries from cla_group project association")
				deleteErr := s.projectsClaGroupsRepo.RemoveProjectAssociatedWithClaGroup(request.CLAGroupID, request.ProjectSFIDList, false)
				if deleteErr != nil {
					log.WithFields(f).WithError(deleteErr).Warn("deleting stale entries from cla_group project association failed")
				}
				// Add the error to the error list
				errorList = append(errorList, err)
			}
			// add event log entry
			s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType:   events.CLAGroupEnrolledProject,
				ProjectSFID: sfid,
				CLAGroupID:  request.CLAGroupID,
				LfUsername:  request.AuthUser.UserName,
				EventData:   &events.CLAGroupEnrolledProjectData{},
			})
		}(projectSFID)
	}

	// Wait for the go routines to finish
	log.WithFields(f).Debug("waiting for associate cla_group with project...")
	wg.Wait()

	// If any errors while associating - return the first one
	if len(errorList) > 0 {
		log.WithFields(f).WithError(errorList[0]).Warnf("encountered %d errors when associating %d projects with the CLA Group", len(errorList), len(request.ProjectSFIDList))
		return errorList[0]
	}

	return nil
}

func (s *service) UnassociateCLAGroupWithProjects(ctx context.Context, request *UnassociateCLAGroupWithProjectsModel) error {
	f := logrus.Fields{
		"functionName":    "UnassociateCLAGroupWithProjects",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"authUserName":    request.AuthUser.UserName,
		"authUserEmail":   request.AuthUser.Email,
		"claGroupID":      request.CLAGroupID,
		"foundationSFID":  request.FoundationSFID,
		"projectSFIDList": strings.Join(request.ProjectSFIDList, ","),
	}

	deleteErr := s.projectsClaGroupsRepo.RemoveProjectAssociatedWithClaGroup(request.CLAGroupID, request.ProjectSFIDList, false)
	if deleteErr != nil {
		log.WithFields(f).Warnf("problem disassociating projects with CLA Group, error: %+v", deleteErr)
		return deleteErr
	}

	// If this is slow, we may want to run these in a go routine
	for _, projectSFID := range request.ProjectSFIDList {
		// add event log entry
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:   events.CLAGroupUnenrolledProject,
			ProjectSFID: projectSFID,
			CLAGroupID:  request.CLAGroupID,
			LfUsername:  request.AuthUser.UserName,
			EventData:   &events.CLAGroupUnenrolledProjectData{},
		})
	}

	return nil
}

// EnableCLAService enable CLA service attribute in the project service for the specified project list
func (s *service) EnableCLAService(ctx context.Context, authUser *auth.User, projectSFIDList []string) error {
	f := logrus.Fields{
		"functionName":    "EnableCLAService",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"authUserName":    authUser.UserName,
		"authUserEmail":   authUser.Email,
		"projectSFIDList": strings.Join(projectSFIDList, ","),
	}

	log.WithFields(f).Debug("enabling CLA service in platform project service")
	// Run this in parallel...
	var errorList []error
	var wg sync.WaitGroup
	wg.Add(len(projectSFIDList))
	psc := v2ProjectService.GetClient()

	for _, projectSFID := range projectSFIDList {
		// Execute as a go routine
		go func(psClient *v2ProjectService.Client, sfid string) {
			defer wg.Done()
			enableProjectErr := psClient.EnableCLA(sfid)
			if enableProjectErr != nil {
				log.WithFields(f).WithError(enableProjectErr).
					Warnf("unable to enable CLA service for project: %s, error: %+v", sfid, enableProjectErr)
				errorList = append(errorList, enableProjectErr)
			} else {
				log.WithFields(f).Debugf("enabled CLA service for project: %s", sfid)
				// add event log entry
				s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
					EventType:  events.ProjectServiceCLAEnabled,
					ProjectID:  sfid,
					LfUsername: authUser.UserName,
					EventData:  &events.ProjectServiceCLAEnabledData{},
				})
			}
		}(psc, projectSFID)
	}
	// Wait until all go routines are done
	wg.Wait()

	if len(errorList) > 0 {
		log.WithFields(f).WithError(errorList[0]).Warnf("encountered %d errors when enabling CLA service for %d projects", len(errorList), len(projectSFIDList))
		return errorList[0]
	}

	log.WithFields(f).Debugf("enabled %d projects successfully for CLA Group", len(projectSFIDList))
	return nil
}

// DisableCLAService disable CLA service attribute in the project service for the specified project list
func (s *service) DisableCLAService(ctx context.Context, authUser *auth.User, projectSFIDList []string) error {
	f := logrus.Fields{
		"functionName":    "DisableCLAService",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"authUserName":    authUser.UserName,
		"authUserEmail":   authUser.Email,
		"projectSFIDList": strings.Join(projectSFIDList, ","),
	}

	log.WithFields(f).Debug("disabling CLA service in platform project service")
	// Run this in parallel...
	var errorList []error
	var wg sync.WaitGroup
	wg.Add(len(projectSFIDList))
	psc := v2ProjectService.GetClient()

	for _, projectSFID := range projectSFIDList {
		// Execute as a go routine
		go func(psClient *v2ProjectService.Client, sfid string) {
			defer wg.Done()
			disableProjectErr := psClient.DisableCLA(sfid)
			if disableProjectErr != nil {
				log.WithFields(f).WithError(disableProjectErr).
					Warnf("unable to disable CLA service for project: %s, error: %+v", sfid, disableProjectErr)
				errorList = append(errorList, disableProjectErr)
			} else {
				log.WithFields(f).Debugf("disabled CLA service for project: %s", sfid)
				// add event log entry
				s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
					EventType:  events.ProjectServiceCLADisabled,
					ProjectID:  sfid,
					LfUsername: authUser.UserName,
					EventData:  &events.ProjectServiceCLADisabledData{},
				})
			}
		}(psc, projectSFID)
	}
	// Wait until all go routines are done
	wg.Wait()

	if len(errorList) > 0 {
		log.WithFields(f).WithError(errorList[0]).Warnf("encountered %d errors when disabling CLA service for %d projects", len(errorList), len(projectSFIDList))
		return errorList[0]
	}

	log.WithFields(f).Debugf("disabled %d projects successfully for CLA Group", len(projectSFIDList))
	return nil
}

func anySubProjectsAlreadyConfigured(inputProjectIDs []string, existingProjectIDs []*projects_cla_groups.ProjectClaGroup) (bool, []string) {
	// Build a quick map of the existing project IDs on file...
	set := make(map[string]struct{})
	var exists = struct{}{}
	for _, existingProject := range existingProjectIDs {
		set[existingProject.ProjectSFID] = exists
	}

	// Look through the input project ID list - if any matches set the flag and add to the response list
	var foundIDs []string
	var response = false

	for _, id := range inputProjectIDs {
		if _, ok := set[id]; ok {
			response = true
			foundIDs = append(foundIDs, id)
		}
	}

	return response, foundIDs
}

func toFoundationMapping(list []*projects_cla_groups.ProjectClaGroup) *models.FoundationMappingList {
	out := &models.FoundationMappingList{List: make([]*models.FoundationMapping, 0)}
	foundationMap := make(map[string]*models.FoundationMapping)
	claGroups := make(map[string]*models.ClaGroupProjects)
	for _, in := range list {
		cgp, ok := claGroups[in.ClaGroupID]
		if !ok {
			cgp = &models.ClaGroupProjects{
				ClaGroupID:      in.ClaGroupID,
				ProjectSfidList: []string{in.ProjectSFID},
			}
			claGroups[in.ClaGroupID] = cgp
			foundation, ok := foundationMap[in.FoundationSFID]
			if !ok {
				foundation = &models.FoundationMapping{
					ClaGroups:      []*models.ClaGroupProjects{cgp},
					FoundationSfid: in.FoundationSFID,
				}
				foundationMap[in.FoundationSFID] = foundation
				out.List = append(out.List, foundation)
			} else {
				foundation.ClaGroups = append(foundation.ClaGroups, cgp)
			}
		} else {
			cgp.ProjectSfidList = append(cgp.ProjectSfidList, in.ProjectSFID)
		}
	}
	return out
}

// isFoundationLevelCLA is a helper function to determine if the list of projects includes the Foundation ID - if it does
// it is considered a foundation level CLA - meaning that the foundation and all the projects under it are part of the
// same CLA group. The way we can tell is if the Foundation was selected as part of the project list.
func isFoundationLevelCLA(foundationSFID string, projects []*projects_cla_groups.ProjectClaGroup) bool {
	for _, project := range projects {
		if project.ProjectSFID == foundationSFID {
			return true
		}
	}
	return false
}

// isFoundationIDInList is a helper function to determine if the list of project IDs includes the Foundation ID - if it
// does it is considered a foundation level CLA - meaning that the foundation and all the projects under it are part of
// the same CLA group. The way we can tell is if the Foundation was selected as part of the project list.
func isFoundationIDInList(foundationSFID string, projectsSFIDs []string) bool {
	for _, projectSFID := range projectsSFIDs {
		if projectSFID == foundationSFID {
			return true
		}
	}
	return false
}
