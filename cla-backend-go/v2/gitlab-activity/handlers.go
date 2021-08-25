// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_activity

import (
	"context"
	"errors"
	"fmt"
	gitlab_api "github.com/communitybridge/easycla/cla-backend-go/gitlab_api"
	"github.com/communitybridge/easycla/cla-backend-go/v2/gitlab_organizations"

	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/gitlab_activity"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	gitlabsdk "github.com/xanzy/go-gitlab"
)

func Configure(api *operations.EasyclaAPI, service Service, gitlabOrgRepo gitlab_organizations.RepositoryInterface, eventService events.Service, gitLabApp *gitlab_api.App) {

	api.GitlabActivityGitlabTriggerHandler = gitlab_activity.GitlabTriggerHandlerFunc(func(params gitlab_activity.GitlabTriggerParams) middleware.Responder {
		requestID, _ := uuid.NewV4()
		reqID := requestID.String()

		if params.GitlabTriggerInput == nil || params.GitlabTriggerInput.GitlabOrganizationID == nil || params.GitlabTriggerInput.GitlabExternalRepositoryID == nil || params.GitlabTriggerInput.GitlabMrID == nil{
			return gitlab_activity.NewGitlabActivityBadRequest().WithPayload(
				utils.ErrorResponseBadRequest(reqID, "missing parameter"))
		}

		gitlabOrganizationID := *params.GitlabTriggerInput.GitlabOrganizationID
		gitlabExternalRepositoryID := *params.GitlabTriggerInput.GitlabExternalRepositoryID
		gitlabMrID := *params.GitlabTriggerInput.GitlabMrID

		f := logrus.Fields{
			"functionName": "gitlab_activity.handlers.GitlabActivityGitlabTriggerHandler",
			"requestID":    reqID,
			"gitlabOrganizationID":gitlabOrganizationID,
			"gitlabExternalRepositoryID":gitlabExternalRepositoryID,
			"gitlabMrID":gitlabMrID,
		}

		log.WithFields(f).Debugf("handling gitlab trigger")
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID)

		gitlabOrg, err := gitlabOrgRepo.GetGitLabOrganization(ctx, gitlabOrganizationID)
		if err != nil {
			msg := fmt.Sprintf("fetching gitlab org failed : %v", err)
			log.WithFields(f).Errorf(msg)
			return gitlab_activity.NewGitlabActivityBadRequest().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		if gitlabOrg == nil {
			msg := fmt.Sprintf("fetching gitlab org failed no results returned")
			log.WithFields(f).Errorf(msg)
			return gitlab_activity.NewGitlabActivityBadRequest().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		gitlabClient, err := gitlab_api.NewGitlabOauthClient(gitlabOrg.AuthInfo, gitLabApp)
		if err != nil {
			msg := fmt.Sprintf("initializing gitlab client : %v", err)
			log.WithFields(f).Errorf(msg)
			return gitlab_activity.NewGitlabActivityBadRequest().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		log.WithFields(f).Debugf("fetching gitlab repository via external id")
		gitlabProject, err := gitlab_api.GetProjectByID(ctx, gitlabClient, int(gitlabExternalRepositoryID))
		if err != nil {
			msg := fmt.Sprintf("fetching gitlab project failed : %v", err)
			log.WithFields(f).Errorf(msg)
			return gitlab_activity.NewGitlabActivityBadRequest().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		gitlabMr, err := gitlab_api.FetchMrInfo(gitlabClient, int(gitlabExternalRepositoryID), int(gitlabMrID))
		if err != nil {
			msg := fmt.Sprintf("fetching gitlab mr failed : %v", err)
			log.WithFields(f).Errorf(msg)
			return gitlab_activity.NewGitlabActivityBadRequest().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		err = service.ProcessMergeActivity(ctx, gitlabOrg.AuthState, &ProcessMergeActivityInput{
			ProjectName:      gitlabProject.Name,
			ProjectPath:      gitlabProject.PathWithNamespace,
			ProjectNamespace: gitlabProject.Namespace.Name,
			ProjectID:        gitlabProject.ID,
			MergeID:          int(gitlabMrID),
			RepositoryPath:   gitlabProject.PathWithNamespace,
			LastCommitSha:    gitlabMr.SHA,
		})
		if err != nil {
			msg := fmt.Sprintf("processing gitlab merge event failed : %v", err)
			log.WithFields(f).Errorf(msg)
			if errors.Is(err, secretTokenMismatch) {
				return gitlab_activity.NewGitlabActivityUnauthorized().WithPayload(
					utils.ErrorResponseUnauthorized(reqID, msg))
			}
			return gitlab_activity.NewGitlabActivityInternalServerError().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		return gitlab_activity.NewGitlabActivityOK()

	})

	api.GitlabActivityGitlabActivityHandler = gitlab_activity.GitlabActivityHandlerFunc(func(params gitlab_activity.GitlabActivityParams) middleware.Responder {
		requestID, _ := uuid.NewV4()
		reqID := requestID.String()
		f := logrus.Fields{
			"functionName": "gitlab_activity.handlers.GitlabActivityGitlabActivityHandler",
			"requestID":    reqID,
		}
		log.WithFields(f).Debugf("handling gitlab activity callback")
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID)

		if params.XGitlabToken == "" {
			return gitlab_activity.NewGitlabActivityUnauthorized().WithPayload(
				utils.ErrorResponseUnauthorized(reqID, "missing webhook secret token"))
		}

		jsonData, err := params.GitlabActivityInput.MarshalJSON()
		if err != nil {
			msg := fmt.Sprintf("unmarshall event data failed : %v", err)
			log.WithFields(f).Errorf(msg)
			return gitlab_activity.NewGitlabActivityBadRequest().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		event, err := gitlabsdk.ParseWebhook(gitlabsdk.EventTypeMergeRequest, jsonData)
		if err != nil {
			msg := fmt.Sprintf("parsing gitlab merge event type failed : %v", err)
			log.WithFields(f).Errorf(msg)
			return gitlab_activity.NewGitlabActivityBadRequest().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		mergeEvent, ok := event.(*gitlabsdk.MergeEvent)
		if !ok {
			msg := fmt.Sprintf("parsing gitlab merge event typecast failed : %v", err)
			log.WithFields(f).Errorf(msg)
			return gitlab_activity.NewGitlabActivityBadRequest().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		if mergeEvent.ObjectAttributes.State != "opened" && mergeEvent.ObjectAttributes.State != "update" && mergeEvent.ObjectAttributes.State != "reopen" {
			msg := fmt.Sprintf("parsing gitlab merge event : %s failed, only [open, update, reopen] accepted", mergeEvent.ObjectAttributes.State)
			log.WithFields(f).Errorf(msg)
			return gitlab_activity.NewGitlabActivityBadRequest().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		err = service.ProcessMergeOpenedActivity(ctx, params.XGitlabToken, mergeEvent)
		if err != nil {
			msg := fmt.Sprintf("processing gitlab merge event failed : %v", err)
			log.WithFields(f).Errorf(msg)
			if errors.Is(err, secretTokenMismatch) {
				return gitlab_activity.NewGitlabActivityUnauthorized().WithPayload(
					utils.ErrorResponseUnauthorized(reqID, msg))
			}
			return gitlab_activity.NewGitlabActivityInternalServerError().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		return gitlab_activity.NewGitlabActivityOK()
	})

}
