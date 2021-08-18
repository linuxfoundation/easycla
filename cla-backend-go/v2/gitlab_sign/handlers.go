// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_sign

import (
	"context"
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/gitlab_sign"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
	"github.com/sirupsen/logrus"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

func Configure(api *operations.EasyclaAPI, service Service, eventService events.Service, contributorConsoleV2Base string) {
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

			log.WithFields(f).Debugf("Initiating Gitlab sign request for : %+v ", srp)

			err := service.GitlabSignRequest(ctx, srp.HTTPRequest, srp.OrganizationID, srp.GitlabRepositoryID, srp.MergeRequestID, contributorConsoleV2Base, eventService)

			if err != nil {
				msg := fmt.Sprintf("problem initiating sign request for :%+v", srp)
				log.WithFields(f).Debugf(msg)
				return gitlab_sign.NewSignRequestBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			return gitlab_sign.NewSignRequestOK()
		})
}
