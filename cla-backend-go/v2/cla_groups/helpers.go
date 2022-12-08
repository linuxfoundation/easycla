// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_groups

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/communitybridge/easycla/cla-backend-go/config"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/events"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	v2ProjectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"
	psproject "github.com/communitybridge/easycla/cla-backend-go/v2/project-service/client/project"
	v2ProjectServiceModels "github.com/communitybridge/easycla/cla-backend-go/v2/project-service/models"

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
		"functionName":        "v2.cla_groups.helpers.validateClaGroupInput",
		utils.XREQUESTID:      ctx.Value(utils.XREQUESTID),
		"ClaGroupName":        claGroupName,
		"ClaGroupDescription": input.ClaGroupDescription,
		"FoundationSfid":      foundationSFID,
		"IclaEnabled":         *input.IclaEnabled,
		"CclaEnabled":         *input.CclaEnabled,
		"CclaRequiresIcla":    *input.CclaRequiresIcla,
		"ProjectSfidList":     strings.Join(input.ProjectSfidList, ","),
		"templateID":          input.TemplateFields.TemplateID,
	}

	log.WithFields(f).Debug("validating CLA Group input...")

	if input.TemplateFields.TemplateID == "" {
		msg := "missing CLA Group template ID value"
		log.WithFields(f).Warn(msg)
		return false, errors.New(msg)
	}
	if !s.v1TemplateService.CLAGroupTemplateExists(ctx, input.TemplateFields.TemplateID) {
		msg := "invalid template ID"
		log.WithFields(f).Warn(msg)
		return false, errors.New(msg)
	}
	// First, check that all the required flags are set and make sense
	if foundationSFID == "" {
		msg := "bad request: foundation_sfid cannot be empty"
		log.WithFields(f).Warn(msg)
		return false, errors.New(msg)
	}
	if !*input.IclaEnabled && !*input.CclaEnabled {
		msg := "bad request: can not create cla group with both icla and ccla disabled"
		log.WithFields(f).Warn(msg)
		return false, errors.New(msg)
	}
	if *input.CclaRequiresIcla {
		if !(*input.IclaEnabled && *input.CclaEnabled) {
			msg := "bad request: ccla_requires_icla can not be enabled if one of icla/ccla is disabled"
			log.WithFields(f).Warn(msg)
			return false, errors.New(msg)
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

	// Is our parent the LF project?
	log.WithFields(f).Debugf("looking up LF parent project record...")
	isLFParent := false
	if utils.IsProjectHaveParent(foundationProjectDetails) {
		isLFParent, err = psc.IsTheLinuxFoundation(utils.GetProjectParentSFID(foundationProjectDetails))
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("validation failure - unable to lookup parent project by SFID: %s", utils.GetProjectParentSFID(foundationProjectDetails))
			return false, err
		}
	}

	// If the foundation details in the platform project service indicates that this foundation has no parent or no
	// children/sub-project... (stand alone project situation)
	log.WithFields(f).Debug("checking to see if we have a standalone project...")
	if isLFParent && len(foundationProjectDetails.Projects) == 0 {
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

	projectLevelCLA := true
	if isFoundationIDInList(*input.FoundationSfid, input.ProjectSfidList) {
		projectLevelCLA = false
	}

	err = s.validateEnrollProjectsInput(ctx, foundationSFID, input.ProjectSfidList, projectLevelCLA, []string{})
	if err != nil {
		return false, err
	}
	return false, nil
}

func (s *service) validateEnrollProjectsInput(ctx context.Context, foundationSFID string, projectSFIDList []string, projectLevel bool, claGroupProjects []string) error { //nolint
	f := logrus.Fields{
		"functionName":     "validateEnrollProjectsInput",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"foundationSFID":   foundationSFID,
		"projectSFIDList":  strings.Join(projectSFIDList, ","),
		"projectLevel":     projectLevel,
		"claGroupProjects": strings.Join(claGroupProjects, ","),
	}

	psc := v2ProjectService.GetClient()

	if len(projectSFIDList) == 0 {
		return errors.New("validation failure - there should be at least one project provided for the enroll request")
	}

	// fetch the foundation model details from the platform project service which includes a list of its sub projects
	foundationProjectDetails, err := psc.GetProject(foundationSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("validation failure - problem fetching project details from project service for project: %s", foundationSFID)
		return err
	}
	if foundationProjectDetails == nil {
		return fmt.Errorf("validation failure - problem fetching project details for project: %s", foundationSFID)
	}

	foundationProjectSummary, err := psc.GetSummary(ctx, foundationSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("validation failure - problem fetching project details for project: %s", foundationSFID)
		return err
	}
	if foundationProjectSummary == nil {
		return fmt.Errorf("validation failure - problem fetching project details from project service for project: %s", foundationSFID)
	}

	// Combine all the projectSFID values and check to see if any are the project root - shouldn't be in the list
	if psc.IsAnyProjectTheRootParent(append(projectSFIDList, foundationSFID)) {
		return errors.New("validation failure - one of the input projects is the root Linux Foundation project")
	}

	// build Tree that tracks parent and child projects
	projectTree := buildProjectNode(foundationProjectSummary)
	log.WithFields(f).Debugf("projectTree: %+v", projectTree)

	invalidSiblingProjects := []string{}
	// Check to see if CLAGroup at ProjectLevel has no siblings
	if projectLevel {
		log.WithFields(f).Debugf("checking to see if CLAGroup at ProjectLevel has no siblings...")
		for _, projectSFID := range projectSFIDList {
			siblings := getSiblings(projectTree, projectSFID)
			log.WithFields(f).Debugf("projectSFID: %s, siblings: %v", projectSFID, siblings)
			if len(siblings) > 0 {
				for _, claProject := range claGroupProjects {
					for _, sibling := range siblings {
						if sibling == claProject {
							invalidSiblingProjects = append(invalidSiblingProjects, claProject)
						}
					}
				}
			}
		}
	}

	if len(invalidSiblingProjects) > 0 {
		log.WithFields(f).Warnf("validation failure - one of the input projects is a sibling of the project level CLA Group: %s", strings.Join(invalidSiblingProjects, ","))
		return fmt.Errorf("validation failure - one of the input projects has siblings in the CLA Group: %s", strings.Join(invalidSiblingProjects, ","))
	}

	// Is our parent the LF project?
	log.WithFields(f).Debugf("looking up LF parent project record...")
	isLFParent := false
	// if we have a project tree parent ID - check to see if it is one of our root parents
	if projectTree != nil && projectTree.Parent != nil && projectTree.Parent.ID != "" {
		log.WithFields(f).Debug("checking if parent project is the Linux Foundation or LF Projects LLC...")
		isLFParent, err = psc.IsTheLinuxFoundation(projectTree.Parent.ID)

		if err != nil {
			log.WithFields(f).WithError(err).Warnf("validation failure - unable to lookup %s or %s project", utils.TheLinuxFoundation, utils.LFProjectsLLC)
			return err
		}

		log.WithFields(f).Debugf("isLFParent: %t", isLFParent)
	}

	// Make sure each project exists in the project service
	for _, projectSFID := range projectSFIDList {
		projectDetails, projErr := psc.GetProject(projectSFID)
		if projErr != nil {
			return fmt.Errorf("validation failure - unable to lookup project by ID %s due to the error: %+v", projectSFID, err)
		}

		if projectDetails == nil {
			return fmt.Errorf("validation failure - unable to lookup project by ID %s", projectSFID)
		}

	}

	// Check to see if all the provided enrolled projects are part of this foundation
	if !allProjectsExistInTree(projectTree, projectSFIDList) {
		log.WithFields(f).Warnf("validation failure - one or more provided projects are not under the project tree: %+v", projectTree.String())
		return fmt.Errorf("bad request: invalid project_sfid: %+v. One or more provided projects are not under the parent", projectTree.String())
	}

	// check if projects are not already enabled
	enabledProjects, err := s.projectsClaGroupsRepo.GetProjectsIdsForFoundation(ctx, foundationSFID)
	if err != nil {
		return err
	}
	enabledProjectList := utils.NewStringSet()
	for _, pr := range enabledProjects {
		enabledProjectList.Add(pr.ProjectSFID)
	}

	invalidProjectSFIDs := utils.NewStringSet()
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
		return errors.New("validation failure - there should be at least one project provided for the unenroll request")
	}

	// fetch the foundation model details from the platform project service which includes a list of its sub projects
	foundationProjectDetails, err := psc.GetProject(foundationSFID)
	if err != nil {
		return err
	}
	if foundationProjectDetails == nil {
		return fmt.Errorf("validation failure - problem fetching project details for project: %s", foundationSFID)
	}

	foundationProjectSummary, err := psc.GetSummary(ctx, foundationSFID)
	if err != nil {
		return err
	}
	if foundationProjectSummary == nil {
		return fmt.Errorf("validation failure - problem fetching project details for project: %s", foundationSFID)
	}

	// Combine all the projectSFID values and check to see if any are the project root - shouldn't be in the list
	if psc.IsAnyProjectTheRootParent(append(projectSFIDList, foundationSFID)) {
		return errors.New("validation failure - one of the input projects is the root Linux Foundation project")
	}

	// Grab the existing list of project CLA Groups associated with this foundation
	existingProjectCLAGroupModels, err := s.projectsClaGroupsRepo.GetProjectsIdsForFoundation(ctx, foundationSFID)
	if err != nil {
		return err
	}
	log.WithFields(f).Debugf("before unenroll, we have %d projects associated with the CLA Group - we will be removing %d and will have %d remaining.", len(existingProjectCLAGroupModels), len(projectSFIDList), len(existingProjectCLAGroupModels)-len(projectSFIDList))

	if len(existingProjectCLAGroupModels)-len(projectSFIDList) < 0 {
		return fmt.Errorf("validation failure - must have at least one project enrolled in the CLA group under parent: %s with ID: %s", foundationProjectDetails.Name, foundationSFID)
	}

	// build Tree that tracks parent and child projects
	projectTree := buildProjectNode(foundationProjectSummary)

	// Make sure each project exists in the project service
	for _, projectSFID := range projectSFIDList {
		_, projErr := psc.GetProject(projectSFID)
		if projErr != nil {
			return fmt.Errorf("validation failure - unable to lookup project by ID %s due to the error: %+v", projectSFID, err)
		}
	}

	// Check to see if all the provided enrolled projects are part of this foundation
	if !allProjectsExistInTree(projectTree, projectSFIDList) {
		log.WithFields(f).Warnf("validation failure - one or more provided projects are not under the project tree: %+v", projectTree.String())
		return fmt.Errorf("bad request: invalid project_sfid: %+v. One or more provided projects are not under the parent", projectTree.String())
	}

	// check if projects are already enrolled/enabled
	enabledProjects, err := s.projectsClaGroupsRepo.GetProjectsIdsForFoundation(ctx, foundationSFID)
	if err != nil {
		return err
	}
	enabledProjectList := utils.NewStringSet()
	for _, pr := range enabledProjects {
		enabledProjectList.Add(pr.ProjectSFID)
	}
	invalidProjectSFIDs := utils.NewStringSet()
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
		go func(projectSFID, parentProjectSFID, claGroupID string) {
			defer wg.Done()
			log.WithFields(f).Debugf("associating cla_group with project: %s", projectSFID)
			err := s.projectsClaGroupsRepo.AssociateClaGroupWithProject(ctx, claGroupID, projectSFID, parentProjectSFID)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("associating cla_group with project: %s failed", projectSFID)
				log.WithFields(f).Debug("deleting stale entries from cla_group project association")
				deleteErr := s.projectsClaGroupsRepo.RemoveProjectAssociatedWithClaGroup(ctx, claGroupID, request.ProjectSFIDList, false)
				if deleteErr != nil {
					log.WithFields(f).WithError(deleteErr).Warn("deleting stale entries from cla_group project association failed")
				}
				// Add the error to the error list
				errorList = append(errorList, err)
			}
			// add event log entry
			s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType:         events.CLAGroupEnrolledProject,
				ProjectSFID:       projectSFID,
				ParentProjectSFID: parentProjectSFID,
				CLAGroupID:        claGroupID,
				LfUsername:        request.AuthUser.UserName,
				EventData:         &events.CLAGroupEnrolledProjectData{},
			})
		}(projectSFID, request.FoundationSFID, request.CLAGroupID)
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

	deleteErr := s.projectsClaGroupsRepo.RemoveProjectAssociatedWithClaGroup(ctx, request.CLAGroupID, request.ProjectSFIDList, false)
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
func (s *service) EnableCLAService(ctx context.Context, authUser *auth.User, claGroupID string, projectSFIDList []string) error {
	f := logrus.Fields{
		"functionName":    "EnableCLAService",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"authUserName":    authUser.UserName,
		"authUserEmail":   authUser.Email,
		"projectSFIDList": strings.Join(projectSFIDList, ","),
		"claGroupID":      claGroupID,
	}

	log.WithFields(f).Debug("enabling CLA service in platform project service")
	// Run this in parallel...
	var errorList []error
	var wg sync.WaitGroup
	wg.Add(len(projectSFIDList))
	psc := v2ProjectService.GetClient()

	for _, projectSFID := range projectSFIDList {
		// Execute as a go routine
		go func(psClient *v2ProjectService.Client, claGroupID, projectSFID string) {
			defer wg.Done()
			log.WithFields(f).Debugf("enabling project CLA service for project: %s...", projectSFID)
			enableProjectErr := psClient.EnableCLA(ctx, projectSFID)
			if enableProjectErr != nil {
				log.WithFields(f).WithError(enableProjectErr).Warnf("unable to enable CLA service for project: %s, error: %+v", projectSFID, enableProjectErr)
				errorList = append(errorList, enableProjectErr)
			} else {
				log.WithFields(f).Debugf("enabled CLA service for project: %s", projectSFID)
				// add event log entry
				s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
					EventType:  events.ProjectServiceCLAEnabled,
					ProjectID:  projectSFID,
					CLAGroupID: claGroupID,
					LfUsername: authUser.UserName,
					EventData:  &events.ProjectServiceCLAEnabledData{},
				})

				// If we should enable the CLA Service for the parent
				if config.GetConfig().EnableCLAServiceForParent {
					log.WithFields(f).Debugf("enable parent project CLA service when child is enrolled flag is enabled")
					parentProjectSFID, parentLookupErr := psc.GetParentProject(projectSFID)
					if parentLookupErr != nil || parentProjectSFID == "" {
						log.WithFields(f).WithError(parentLookupErr).Warnf("unable to lookup parent project SFID for project: %s", projectSFID)
					} else {
						isTheLF, lookupErr := psClient.IsTheLinuxFoundation(parentProjectSFID)
						if lookupErr != nil || isTheLF {
							log.WithFields(f).Debugf("skipping setting the enabled services on The Linux Foundation parent project(s) for parent project SFID: %s", parentProjectSFID)
						} else {
							log.WithFields(f).Debugf("enabling parent project CLA service for project SFID: %s...", parentProjectSFID)
							enableProjectErr := psClient.EnableCLA(ctx, parentProjectSFID)
							if enableProjectErr != nil {
								log.WithFields(f).WithError(enableProjectErr).Warnf("unable to enable CLA service for project: %s, error: %+v", parentProjectSFID, enableProjectErr)
								errorList = append(errorList, enableProjectErr)
							} else {
								log.WithFields(f).Debugf("enabled CLA service for parent project: %s", parentProjectSFID)
								// add event log entry
								s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
									EventType:  events.ProjectServiceCLAEnabled,
									ProjectID:  parentProjectSFID,
									CLAGroupID: claGroupID,
									LfUsername: authUser.UserName,
									EventData:  &events.ProjectServiceCLAEnabledData{},
								})
							}
						}
					}
				}
			}
		}(psc, claGroupID, projectSFID)
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
func (s *service) DisableCLAService(ctx context.Context, authUser *auth.User, claGroupID string, projectSFIDList []string) error {
	f := logrus.Fields{
		"functionName":    "DisableCLAService",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"authUserName":    authUser.UserName,
		"authUserEmail":   authUser.Email,
		"projectSFIDList": strings.Join(projectSFIDList, ","),
		"claGroupID":      claGroupID,
	}

	// Run this in parallel...
	var errorList []error
	var wg sync.WaitGroup
	wg.Add(len(projectSFIDList))
	psc := v2ProjectService.GetClient()

	for _, projectSFID := range projectSFIDList {
		// Execute as a go routine
		go func(psClient *v2ProjectService.Client, claGroupID, projectSFID string) {
			defer wg.Done()
			log.WithFields(f).Debugf("disabling CLA service for project: %s", projectSFID)
			disableProjectErr := psClient.DisableCLA(ctx, projectSFID)
			if disableProjectErr != nil {
				log.WithFields(f).WithError(disableProjectErr).
					Warnf("unable to disable CLA service for project: %s, error: %+v", projectSFID, disableProjectErr)
				errorList = append(errorList, disableProjectErr)
			} else {
				log.WithFields(f).Debugf("disabled CLA service for project: %s", projectSFID)
				// add event log entry
				s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
					EventType:  events.ProjectServiceCLADisabled,
					ProjectID:  projectSFID,
					CLAGroupID: claGroupID,
					LfUsername: authUser.UserName,
					EventData:  &events.ProjectServiceCLADisabledData{},
				})
			}
		}(psc, claGroupID, projectSFID)
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

// getUniqueCLAGroupIDs logic to extract the unique
func getUniqueCLAGroupIDs(projectCLAGroupMappings []*projects_cla_groups.ProjectClaGroup) []string {
	// to ensure we get the distinct count
	claGroupsMap := map[string]bool{}
	for _, projectCLAGroupModel := range projectCLAGroupMappings {
		// ensure that following goroutine gets a copy of projectSFID
		projectCLAGroupClaGroupID := projectCLAGroupModel.ClaGroupID
		// No need to re-process the same CLA group
		if _, ok := claGroupsMap[projectCLAGroupClaGroupID]; ok {
			continue
		}

		// Add entry into our map - so we know not to re-process this CLA Group
		claGroupsMap[projectCLAGroupClaGroupID] = true
	}

	keys := make([]string, 0, len(claGroupsMap))
	for k := range claGroupsMap {
		keys = append(keys, k)
	}

	return keys
}

func buildProjectNode(projectSummaryList []*v2ProjectServiceModels.ProjectSummary) *ProjectNode {
	f := logrus.Fields{
		"functionName": "buildProjectNode",
	}
	root := &ProjectNode{
		ID:       "",
		Name:     "",
		Children: nil,
	}

	parentSFID := ""
	parentName := ""
	for _, projectSummaryEntry := range projectSummaryList {
		log.WithFields(f).Debugf("Processing project summary entry: %+v", *projectSummaryEntry)
		// Get ParentProject
		parentProjectModel, err := v2ProjectService.GetClient().GetParentProjectModel(projectSummaryEntry.ID)

		if parentSFID == "" && err == nil && parentProjectModel != nil {
			// Update our root node
			root.Parent = &ProjectNode{
				ID:       parentProjectModel.ID,
				Name:     parentProjectModel.Name,
				Children: []*ProjectNode{root},
			}

			// Save the parentSFID
			parentSFID = parentProjectModel.ID
			parentName = parentProjectModel.Name
		}

		if parentSFID != "" && err == nil && parentProjectModel != nil && parentSFID != parentProjectModel.ID {
			//We have different parents !!!
			log.Warnf("current parent Name: %s ID: %s does not match other parent Name: %s, parent ID: %s", parentName, parentSFID, parentProjectModel.Name, parentProjectModel.ID)
		}

		root.Children = append(root.Children, getLeafNodeFromProjectSFID(projectSummaryEntry.ID, parentName, parentSFID))
	}

	return root
}

func getLeafNodeFromProjectSFID(projectSFID, parentName, parentSFID string) *ProjectNode {
	f := logrus.Fields{
		"functionName": "getLeafNodeFromProjectSFID",
		"projectSFID":  projectSFID,
		"parentName":   parentName,
		"parentSFID":   parentSFID,
	}
	log.WithFields(f).Debugf("building leaf node from projectSFID: %s", projectSFID)

	// Get ParentProject
	projectModel, err := v2ProjectService.GetClient().GetProject(projectSFID)
	if err != nil {
		return nil
	}

	node := &ProjectNode{
		ID:   projectModel.ID,
		Name: projectModel.Name,
		Parent: &ProjectNode{
			ID:   parentSFID,
			Name: parentName,
		},
	}

	// For this node, collect the list of child nodes...
	for _, childNode := range projectModel.Projects {
		node.Children = append(node.Children, getLeafNodeFromProjectSFID(childNode.ID, projectModel.Name, projectModel.ID))
	}

	return node
}

// findByID searches for given projectSFID recursively using DFS algorithm
func findByID(node *ProjectNode, projectSFID string) *ProjectNode {
	if node == nil {
		return nil
	}
	if node.ID == projectSFID {
		return node
	}

	for _, child := range node.Children {
		foundNode := findByID(child, projectSFID)
		if foundNode != nil {
			return foundNode
		}
	}

	return nil
}

// GetProjectDescendants returns all descendants of given projectSummary (salesforce)
func GetProjectDescendants(projectSummary []*v2ProjectServiceModels.ProjectSummary) []string {

	descendants := make([]string, 0)
	for _, project := range projectSummary {
		if len(project.Projects) > 0 {
			descendants = append(descendants, project.ID)
			for _, child := range project.Projects {
				descendants = append(descendants, child.ID)
			}
		}
	}

	return descendants
}

// getSiblings returns all siblings of a given projectSFID
func getSiblings(root *ProjectNode, projectSFID string) []string {
	f := logrus.Fields{
		"functionName": "v2.project.utils.getSiblings",
		"projectSFID":  projectSFID,
	}

	log.WithFields(f).Debugf("getting siblings for projectSFID: %s", projectSFID)

	if root == nil {
		return []string{}
	}

	siblings := make([]string, 0)

	// stores nodes level wise
	var queue ProjectStack

	// push root node
	queue.Push(root)

	// traverse all levels
	for !queue.IsEmpty() {
		log.WithFields(f).Debugf("queue is not empty, processing next level")
		tempNode := queue.Peek()
		queue, _ = queue.Pop()
		for _, child := range tempNode.Children {
			if child.ID == projectSFID {
				// add all children of tempNode to siblings aside from projectSFID
				for _, sibling := range tempNode.Children {
					if sibling.ID != projectSFID {
						siblings = append(siblings, sibling.ID)
					}
				}
				break
			}
			// push child node
			queue.Push(child)
		}

	}
	log.WithFields(f).Debugf("returning siblings: %+v", siblings)

	return siblings
}

// allProjectsExistInTree searches for given list of projects in foundation items
func allProjectsExistInTree(node *ProjectNode, projectSFIDs []string) bool {
	for _, projectSFID := range projectSFIDs {
		found := findByID(node, projectSFID)
		if found == nil {
			return false
		}
	}
	return true
}

func (n *ProjectNode) String() string {
	if n == nil {
		return ""
	}
	msg := fmt.Sprintf("projectSFID: '%s', projectName: '%s'\n", n.ID, n.Name)

	if n.Parent == nil {
		msg = fmt.Sprintf("%s 'parentProjectSFID':'%s','parentProjectName':'%s'\n", msg, "", "")
	} else {
		msg = fmt.Sprintf("%s 'parentProjectSFID':'%s','parentProjectName':'%s'\n", msg, n.Parent.ID, n.Parent.Name)
	}

	if len(n.Children) == 0 {
		msg = fmt.Sprintf("%s children: no children\n", msg)
	} else {
		for _, child := range n.Children {
			if child != nil {
				msg = fmt.Sprintf("%s 'child': %s\n", msg, child.String())
			}
		}
	}

	return msg
}
