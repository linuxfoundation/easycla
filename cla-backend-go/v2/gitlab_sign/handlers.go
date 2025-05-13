// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_sign

import (
	"context"
	"fmt"
	"net/http"

	"github.com/communitybridge/easycla/cla-backend-go/config"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/gitlab_sign"
	gitlabApi "github.com/communitybridge/easycla/cla-backend-go/gitlab_api"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	"github.com/gofrs/uuid"
	"github.com/savaki/dynastore"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	oauth_gitlab "golang.org/x/oauth2/gitlab"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

const (
	// SessionStoreKey for cla-gitlab session
	SessionStoreKey = "cla-gitlab"
)

func Configure(api *operations.EasyclaAPI, service Service, eventService events.Service, contributorConsoleV2Base string, sessionStore *dynastore.Store) {
	api.GitlabSignSignRequestHandler = gitlab_sign.SignRequestHandlerFunc(
		func(srp gitlab_sign.SignRequestParams) middleware.Responder {
			reqID := utils.GetRequestID(srp.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID)

			f := logrus.Fields{
				"functionName":   "v2.gitlab_sign.handlers.GitlabSignSignRequestHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"installationID": srp.OrganizationID,
				"repositoryID":   srp.GitlabRepositoryID,
				"mergeRequestID": srp.MergeRequestID,
			}

			return middleware.ResponderFunc(func(rw http.ResponseWriter, pr runtime.Producer) {
				session, err := sessionStore.Get(srp.HTTPRequest, SessionStoreKey)
				if err != nil {
					log.WithFields(f).WithError(err).Warn("error with session store lookup")
					http.Error(rw, err.Error(), http.StatusInternalServerError)
					return
				}
				config := config.GetConfig()

				log.WithFields(f).Debugf("Loading session : %+v", session)

				log.WithFields(f).Debug("Initiating sign request..")

				session.Values["gitlab_installation_id"] = srp.OrganizationID
				session.Values["gitlab_repository_id"] = srp.GitlabRepositoryID
				session.Values["gitlab_merge_request_id"] = srp.MergeRequestID

				originURL, err := service.GetOriginURL(ctx, srp.OrganizationID, srp.GitlabRepositoryID, srp.MergeRequestID)
				if err != nil {
					log.WithFields(f).WithError(err).Warn("error getting origin URL")
					http.Error(rw, err.Error(), http.StatusInternalServerError)
					return
				}

				session.Values["gitlab_origin_url"] = *originURL

				gitlabAuthToken, ok := session.Values["gitlab_oauth2_token"].(string)
				if ok {
					session.Save(srp.HTTPRequest, rw)
					log.WithFields(f).Debugf("using existing Gitlab Ouath2 Token: %s ", gitlabAuthToken)
					gitlabClient, err := gitlabApi.NewGitlabOauthClientFromAccessToken(gitlabAuthToken)

					if err != nil {
						msg := fmt.Sprintf("problem creating gitlab client with token : %s ", gitlabAuthToken)
						log.WithFields(f).Debug(msg)
						http.Error(rw, msg, http.StatusInternalServerError)
					}

					log.WithFields(f).Debugf("Initiating Gitlab sign request for : %+v ", srp)

					consoleURL, err := service.InitiateSignRequest(ctx, srp.HTTPRequest, gitlabClient, srp.GitlabRepositoryID, srp.MergeRequestID, *originURL, contributorConsoleV2Base, eventService)

					if err != nil {
						msg := fmt.Sprintf("problem initiating sign request for :%+v", srp)
						log.WithFields(f).Debugf("%s", msg)
						http.Error(rw, msg, http.StatusInternalServerError)
						return
					}

					http.Redirect(rw, srp.HTTPRequest, *consoleURL, http.StatusSeeOther)
				}

				log.WithFields(f).Debugf("No existing GitLab Oauth2 Token ")

				log.WithFields(f).Debug("initiating gitlab sign request ...")
				stateID, err := uuid.NewV4()
				state := fmt.Sprintf("user:%s", stateID.String())
				session.Values["gitlab_oauth2_state"] = state
				session.Save(srp.HTTPRequest, rw)
				oauthConfig := &oauth2.Config{
					ClientID: config.Gitlab.AppClientID,
					Scopes: []string{
						"read_user",
						"email",
					},
					Endpoint:    oauth_gitlab.Endpoint,
					RedirectURL: config.Gitlab.RedirectURI,
				}
				session.Values["gitlab_oauth2_state"] = state
				session.Save(srp.HTTPRequest, rw)
				http.Redirect(rw, srp.HTTPRequest, oauthConfig.AuthCodeURL(state), http.StatusFound)
			})

		})
}
