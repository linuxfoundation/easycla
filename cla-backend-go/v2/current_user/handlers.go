// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package current_user

import (
	"context"
	"fmt"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/current_user"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
)

// Configure sets up the middleware handlers
func Configure(api *operations.EasyclaAPI, service Service) { // nolint
	api.CurrentUserGetUserFromTokenHandler = current_user.GetUserFromTokenHandlerFunc(
		func(params current_user.GetUserFromTokenParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.current_user.handlers.GetUserFromToken",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}
			log.WithFields(f).Debugf("looking for user from bearer token")
			userModel, err := service.UserFromContext(ctx)
			if err != nil {
				msg := fmt.Sprintf("unable to lookup user from token")
				log.WithFields(f).WithError(err).Warn(msg)
				return current_user.NewGetUserFromTokenNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}
			return current_user.NewGetUserFromTokenOK().WithXRequestID(reqID).WithPayload(userModel)
		})
}
