// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_activity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	gitlab_api "github.com/communitybridge/easycla/cla-backend-go/gitlab_api"
	"github.com/communitybridge/easycla/cla-backend-go/v2/gitlab_organizations"
	"github.com/communitybridge/easycla/cla-backend-go/v2/gitlab_sign"

	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/gitlab_activity"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	"github.com/gofrs/uuid"
	"github.com/savaki/dynastore"
	"github.com/sirupsen/logrus"
	gitlabsdk "github.com/xanzy/go-gitlab"
)

const (
	// SessionStoreKey for cla-gitlab
	SessionStoreKey = "cla-gitlab"
)

func Configure(api *operations.EasyclaAPI, service Service, gitlabOrgService gitlab_organizations.ServiceInterface, eventService events.Service, gitLabApp *gitlab_api.App, signService gitlab_sign.Service, contributorConsoleV2Base string, sessionStore *dynastore.Store) {

	api.GitlabActivityGitlabTriggerHandler = gitlab_activity.GitlabTriggerHandlerFunc(func(params gitlab_activity.GitlabTriggerParams) middleware.Responder {
		requestID, _ := uuid.NewV4()
		reqID := requestID.String()

		if params.GitlabTriggerInput == nil || params.GitlabTriggerInput.GitlabOrganizationID == nil || params.GitlabTriggerInput.GitlabExternalRepositoryID == nil || params.GitlabTriggerInput.GitlabMrID == nil {
			return gitlab_activity.NewGitlabActivityBadRequest().WithPayload(
				utils.ErrorResponseBadRequest(reqID, "missing parameter"))
		}

		gitlabOrganizationID := *params.GitlabTriggerInput.GitlabOrganizationID
		gitlabExternalRepositoryID := *params.GitlabTriggerInput.GitlabExternalRepositoryID
		gitlabMrID := *params.GitlabTriggerInput.GitlabMrID

		f := logrus.Fields{
			"functionName":               "gitlab_activity.handlers.GitlabActivityGitlabTriggerHandler",
			"requestID":                  reqID,
			"gitlabOrganizationID":       gitlabOrganizationID,
			"gitlabExternalRepositoryID": gitlabExternalRepositoryID,
			"gitlabMrID":                 gitlabMrID,
		}

		log.WithFields(f).Debugf("handling gitlab trigger")
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID)

		gitlabOrg, err := gitlabOrgService.GetGitLabOrganizationByID(ctx, gitlabOrganizationID)
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

		encryptedOauthResponse, err := gitlabOrgService.RefreshGitLabOrganizationAuth(ctx, gitlabOrg)
		if err != nil {
			msg := fmt.Sprintf("refreshing gitlab org auth failed : %v", err)
			log.WithFields(f).Errorf(msg)
			return gitlab_activity.NewGitlabActivityBadRequest().WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		gitlabClient, err := gitlab_api.NewGitlabOauthClient(*encryptedOauthResponse, gitLabApp)
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

		// if params.XGitlabToken == "" {
		// 	return gitlab_activity.NewGitlabActivityUnauthorized().WithPayload(
		// 		utils.ErrorResponseUnauthorized(reqID, "missing webhook secret token"))
		// }

		// General note for this API endpoint:
		// Even though we had an issue - we will  a 200 request indicating that we received the event, otherwise
		// gitlab will disable the webhook after several failed requests (need to confirm this behavior)
		//
		// From GitLab:
		//   Webhooks that return response codes in the 5xx range are understood to be failing intermittently and are temporarily disabled. These webhooks are initially disabled for 1 minute, which is extended on each retry up to a maximum of 24 hours.
		//   Webhooks that return response codes in the 4xx range are understood to be misconfigured and are permanently disabled until you manually re-enable them yourself.
		//   See Troubleshooting https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#troubleshoot-webhooks for more information on disabled webhooks and how to re-enable them.

		jsonData, err := params.GitlabActivityInput.MarshalJSON()
		if err != nil {
			msg := fmt.Sprintf("unmarshall event data failed : %v", err)
			log.WithFields(f).Debugf(msg)
			// Always return 200 response
			return gitlab_activity.NewGitlabActivityOK()
		}

		event, err := gitlabsdk.ParseWebhook(gitlabsdk.EventTypeMergeRequest, jsonData)
		if err != nil {
			msg := fmt.Sprintf("parsing gitlab merge event type failed : %v", err)
			log.WithFields(f).Debugf(msg)
			// Always return 200 response
			return gitlab_activity.NewGitlabActivityOK()
		}

		mergeEvent, ok := event.(*gitlabsdk.MergeEvent)
		if !ok {
			msg := fmt.Sprintf("parsing gitlab merge event typecast failed : %v", err)
			log.WithFields(f).Debugf(msg)
			// Always return 200 response
			return gitlab_activity.NewGitlabActivityOK()
		}

		if mergeEvent.ObjectKind == "merge_request" {

			if mergeEvent.ObjectAttributes.State != "opened" && mergeEvent.ObjectAttributes.State != "update" && mergeEvent.ObjectAttributes.State != "reopen" {
				msg := fmt.Sprintf("parsing gitlab merge event : %s failed, only [open, update, reopen] accepted", mergeEvent.ObjectAttributes.State)
				log.WithFields(f).Debugf(msg)
				// Always return 200 response
				return gitlab_activity.NewGitlabActivityOK()
			}

			err = service.ProcessMergeOpenedActivity(ctx, params.XGitlabToken, mergeEvent)
			if err != nil {
				msg := fmt.Sprintf("processing gitlab merge event failed : %v", err)
				log.WithFields(f).Debugf(msg)
				// Always return 200 response
				return gitlab_activity.NewGitlabActivityOK()
			}

		} else if mergeEvent.ObjectKind == "note" && strings.Contains(mergeEvent.ObjectAttributes.Description, "/easycla") {
			log.WithFields(f).Debugf("processing gitlab merge comment event")
			err = service.ProcessMergeCommentActivity(ctx, params.XGitlabToken, mergeEvent)
			if err != nil {
				msg := fmt.Sprintf("processing gitlab merge comment event failed : %v", err)
				log.WithFields(f).Debugf(msg)
				// Always return 200 response
				return gitlab_activity.NewGitlabActivityOK()
			}
		}

		return gitlab_activity.NewGitlabActivityOK()
	})

	api.GitlabActivityGitlabUserOauthCallbackHandler = gitlab_activity.GitlabUserOauthCallbackHandlerFunc(
		func(guocp gitlab_activity.GitlabUserOauthCallbackParams) middleware.Responder {
			reqID := utils.GetRequestID(guocp.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID)
			f := logrus.Fields{
				"functionName":   "gitlab_activity.handler.GitlabActivityGitlabUserOauthCallbackHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"code":           guocp.Code,
				"state":          guocp.State,
			}

			return middleware.ResponderFunc(
				func(rw http.ResponseWriter, p runtime.Producer) {
					session, err := sessionStore.Get(guocp.HTTPRequest, SessionStoreKey)
					if err != nil {
						log.WithFields(f).WithError(err).Warn("error with session store lookup")
						http.Error(rw, err.Error(), http.StatusInternalServerError)
						return
					}

					state, ok := session.Values["gitlab_oauth2_state"].(string)
					if !ok {
						log.WithFields(f).Warn("Error getting session state - missing from session object")
						http.Error(rw, "no session state", http.StatusInternalServerError)
						return
					}

					gitlabOriginURL, ok := session.Values["gitlab_origin_url"].(string)
					if !ok {
						log.WithFields(f).Warn("Error getting gitlab_origin_url - missing from session object")
						http.Error(rw, "no return url", http.StatusInternalServerError)
						return
					}

					repositoryID, ok := session.Values["gitab_repository_id"].(string)
					if !ok {
						log.WithFields(f).Warn("Error getting gitlab_repository_id - missing from session object")
						http.Error(rw, "no return url", http.StatusInternalServerError)
						return
					}

					mergeRequestID, ok := session.Values["gitlab_merge_request_id"].(string)
					if !ok {
						log.WithFields(f).Warn("Error getting gitlab_merge_request_id - missing from session object")
						http.Error(rw, "no return url", http.StatusInternalServerError)
						return
					}

					if guocp.State != state {
						msg := fmt.Sprintf("mismatch state, received: %s from callback, but loaded our state as: %s",
							guocp.State, state)
						log.WithFields(f).Warn(msg)
						http.Error(rw, msg, http.StatusInternalServerError)
						return
					}

					log.WithFields(f).Debug("Fetching access token for user...")
					token, err := gitlab_api.FetchOauthCredentials(guocp.Code)
					if err != nil {
						msg := fmt.Sprint("unable to fetch access token for user")
						log.WithFields(f).Warn(msg)
						http.Error(rw, msg, http.StatusInternalServerError)
						return
					}

					session.Values["gitlab_oauth2_token"] = token.AccessToken
					session.Save(guocp.HTTPRequest, rw)

					// Get client
					gitlabClient, err := gitlab_api.NewGitlabOauthClientFromAccessToken(token.AccessToken)
					if err != nil {
						msg := fmt.Sprintf("unable to create gitlab client from token : %s ", token.AccessToken)
						log.WithFields(f).Warn(msg)
						http.Error(rw, msg, http.StatusInternalServerError)
						return
					}

					consoleURL, err := signService.InitiateSignRequest(ctx, guocp.HTTPRequest, gitlabClient, repositoryID, mergeRequestID, gitlabOriginURL, contributorConsoleV2Base, eventService)
					log.WithFields(f).Debugf("redirecting to :%s ", *consoleURL)
					http.Redirect(rw, guocp.HTTPRequest, *consoleURL, http.StatusSeeOther)
				})
		})

}
