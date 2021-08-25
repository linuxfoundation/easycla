// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_activity

import (
	"context"
	"errors"
	"fmt"

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

func Configure(api *operations.EasyclaAPI, service Service, eventService events.Service) {

	api.GitlabActivityGitlabActivityHandler = gitlab_activity.GitlabActivityHandlerFunc(func(params gitlab_activity.GitlabActivityParams) middleware.Responder {
		requestID, _ := uuid.NewV4()
		reqID := requestID.String()
		f := logrus.Fields{
			"functionName": "gitlab_activity.handlers.GitlabActivityGitlabActivityHandler",
			"requestID":    reqID,
		}
		log.WithFields(f).Debugf("handling gitlab activity callback")
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID)

		if params.XGitlabToken == ""{
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
			if errors.Is(err, secretTokenMismatch){
				return gitlab_activity.NewGitlabActivityUnauthorized().WithPayload(
					utils.ErrorResponseUnauthorized(reqID, msg))
			}
			return gitlab_activity.NewGitlabActivityInternalServerError().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		return gitlab_activity.NewGitlabActivityOK()
	})

}
