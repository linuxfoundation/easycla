// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"github.com/aws/aws-lambda-go/events"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/project/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

func (s *service) ProcessCLAGroupUpdateEvents(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "ProcessCLAGroupUpdateEvents",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"eventID":        event.EventID,
		"eventName":      event.EventName,
		"eventSource":    event.EventSource,
		"event":          event,
		"newImage":       event.Change.NewImage,
		"oldImage":       event.Change.OldImage,
	}

	log.WithFields(f).Debug("processing event")

	var oldProject, updatedProject models.DBProjectModel
	err := unmarshalStreamImage(event.Change.NewImage, &updatedProject)
	if err != nil {
		log.WithFields(f).Warnf("unable to unmarshal new project model, error: %+v", err)
		return err
	}

	err = unmarshalStreamImage(event.Change.OldImage, &oldProject)
	if err != nil {
		log.WithFields(f).Warnf("unable to unmarshal old project model, error: %+v", err)
		return err
	}

	log.WithFields(f).Debugf("decoded project record from stream: %+v", updatedProject)

	// Update any DB records that have CLA Approval Requests from Contributors - need to update Name, etc. if that has changed
	log.WithFields(f).Debugf("updating any CLA approval requests from contributors for this CLA Group")
	approvalListRequestErr := s.approvalListRequestsRepo.UpdateRequestsByCLAGroup(&updatedProject)
	if approvalListRequestErr != nil {
		log.WithFields(f).Warnf("unable to update contributor approval list requests with updated CLA Group information, error: %+v", approvalListRequestErr)
	}

	log.WithFields(f).Debugf("updating any CLA manager requests for this CLA Group")
	managerRequestErr := s.claManagerRequestsRepo.UpdateRequestsByCLAGroup(&updatedProject)
	if managerRequestErr != nil {
		log.WithFields(f).Warnf("unable to update cla manager request with updated CLA Group information, error: %+v", approvalListRequestErr)
	}

	if oldProject.ProjectName != updatedProject.ProjectName {
		claProjects, err := s.projectsClaGroupRepo.GetProjectsIdsForClaGroup(ctx, updatedProject.ProjectID)
		if err != nil {
			log.WithFields(f).Warnf("unabled to update cla group name : %v", err)
			return nil
		}

		for _, claProject := range claProjects {
			if err := s.projectsClaGroupRepo.UpdateClaGroupName(ctx, claProject.ProjectSFID, updatedProject.ProjectName); err != nil {
				log.WithFields(f).Warnf("updating cla project : %s with name : %s failed : %v", claProject.ProjectSFID, updatedProject.ProjectName, err)
				return nil
			}
		}
		log.WithFields(f).Infof("updating related cla projects with name : %s", updatedProject.ProjectName)
	}

	// TODO - update other tables:
	//  cla-%s-metrics,
	//  cla-%s-gerrit-instances,

	return nil
}
