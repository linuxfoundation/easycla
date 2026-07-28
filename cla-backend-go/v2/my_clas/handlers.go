// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	myClasOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/my_clas"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

const missingUsernameMsg = "the authenticated principal carries no username - unable to determine whose CLAs to look up"
const missingIdentityMsg = "no identity provided - provide at least one of lfUsername, email, secondaryEmail, githubId, githubUsername, gitlabId, gitlabUsername, gerritUsername"

// Configure sets up the My CLAs API handlers
func Configure(api *operations.EasyclaAPI, service Service) {
	api.MyClasGetMyClasHandler = myClasOps.GetMyClasHandlerFunc(
		func(params myClasOps.GetMyClasParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.my_clas.handlers.GetMyClas",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}

			currentUsername, admin := principal(authUser)
			if !admin && currentUsername == "" {
				log.WithFields(f).Warn(missingUsernameMsg)
				return myClasOps.NewGetMyClasUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, missingUsernameMsg))
			}

			requested := newIdentity(params.LfUsername, params.Email, params.SecondaryEmail, params.GithubID, params.GithubUsername, params.GitlabID, params.GitlabUsername, params.GerritUsername)
			if admin && currentUsername == "" && requested.IsEmpty() {
				log.WithFields(f).Warn(missingIdentityMsg)
				return myClasOps.NewGetMyClasBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, missingIdentityMsg))
			}

			result, err := service.GetMyClas(ctx, currentUsername, admin, requested)
			if err != nil {
				msg := "unable to lookup the CLAs for the provided identity"
				log.WithFields(f).WithError(err).Warn(msg)
				return myClasOps.NewGetMyClasInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			return myClasOps.NewGetMyClasOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.MyClasGetMyClaPdfHandler = myClasOps.GetMyClaPdfHandlerFunc(
		func(params myClasOps.GetMyClaPdfParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.my_clas.handlers.GetMyClaPdf",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
				"signatureID":    params.SignatureID,
			}

			currentUsername, admin := principal(authUser)
			if !admin && currentUsername == "" {
				log.WithFields(f).Warn(missingUsernameMsg)
				return myClasOps.NewGetMyClaPdfUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, missingUsernameMsg))
			}

			requested := newIdentity(params.LfUsername, params.Email, params.SecondaryEmail, params.GithubID, params.GithubUsername, params.GitlabID, params.GitlabUsername, params.GerritUsername)
			if admin && currentUsername == "" && requested.IsEmpty() {
				log.WithFields(f).Warn(missingIdentityMsg)
				return myClasOps.NewGetMyClaPdfBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, missingIdentityMsg))
			}

			result, err := service.GetMyClaPdfURL(ctx, currentUsername, admin, requested, params.SignatureID)
			if err != nil {
				msg := "unable to generate the signed document download link"
				log.WithFields(f).WithError(err).Warn(msg)
				return myClasOps.NewGetMyClaPdfInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}
			if result == nil {
				msg := "no signed ICLA with the given signature ID belongs to the provided identity"
				log.WithFields(f).Warn(msg)
				return myClasOps.NewGetMyClaPdfNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}

			return myClasOps.NewGetMyClaPdfOK().WithXRequestID(reqID).WithPayload(result)
		})
}

// principal extracts the authenticated username and admin flag from the auth principal
func principal(authUser *auth.User) (string, bool) {
	if authUser == nil {
		return "", false
	}
	return authUser.UserName, utils.IsUserAdmin(authUser)
}

// newIdentity builds the requested identity from the request parameters
func newIdentity(lfUsername *string, emails, secondaryEmails []string, githubIDs []int64, githubUsernames []string, gitlabIDs []int64, gitlabUsernames []string, gerritUsernames []string) *Identity {
	return &Identity{
		LfUsername:      utils.StringValue(lfUsername),
		Emails:          emails,
		SecondaryEmails: secondaryEmails,
		GithubIDs:       githubIDs,
		GithubUsernames: githubUsernames,
		GitlabIDs:       gitlabIDs,
		GitlabUsernames: gitlabUsernames,
		GerritUsernames: gerritUsernames,
	}
}
