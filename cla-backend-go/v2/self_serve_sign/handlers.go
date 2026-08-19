// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package self_serve_sign

import (
	"context"
	"errors"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	selfServeSignOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/self_serve_sign"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

const missingUsernameMsg = "the authenticated principal carries no username - unable to determine who is signing"

// Configure sets up the Self Serve signing API handlers
func Configure(api *operations.EasyclaAPI, service Service) {
	api.SelfServeSignPrepareSignHandler = selfServeSignOps.PrepareSignHandlerFunc(
		func(params selfServeSignOps.PrepareSignParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.self_serve_sign.handlers.PrepareSign",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}

			currentUsername, currentEmail, admin := principal(authUser)
			if !admin && currentUsername == "" {
				log.WithFields(f).Warn(missingUsernameMsg)
				return selfServeSignOps.NewPrepareSignUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, missingUsernameMsg))
			}

			result, err := service.PrepareSign(ctx, currentUsername, currentEmail, admin, &params.Body)
			if err != nil {
				switch {
				case errors.Is(err, ErrCLAGroupNotFound):
					log.WithFields(f).WithError(err).Warn(err.Error())
					return selfServeSignOps.NewPrepareSignNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, err.Error()))
				case errors.Is(err, ErrIdentityNotVerified):
					log.WithFields(f).WithError(err).Warn(err.Error())
					return selfServeSignOps.NewPrepareSignForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, err.Error()))
				case errors.Is(err, ErrIdentityRequired), errors.Is(err, ErrSigningNotEnabled), errors.Is(err, ErrReturnURLNotSupported):
					log.WithFields(f).WithError(err).Warn(err.Error())
					return selfServeSignOps.NewPrepareSignBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, err.Error()))
				}
				msg := "unable to prepare the signing session"
				log.WithFields(f).WithError(err).Warn(msg)
				return selfServeSignOps.NewPrepareSignInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			return selfServeSignOps.NewPrepareSignOK().WithXRequestID(reqID).WithPayload(result)
		})
}

func principal(authUser *auth.User) (string, string, bool) {
	if authUser == nil {
		return "", "", false
	}
	return authUser.UserName, authUser.Email, utils.IsUserAdmin(authUser)
}
