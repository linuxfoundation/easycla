// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"errors"
	"strings"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	myClasOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/my_clas"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

const missingUsernameMsg = "the authenticated principal carries no username - unable to determine whose CLAs to look up"
const missingIdentityMsg = "no identity provided - provide at least one of lfUsername, email, secondaryEmail, githubId, githubUsername, gitlabId, gitlabUsername, gerritUsername"
const missingGithubIDMsg = "no githubId provided - provide the GitHub account number the contributor selected"
const mismatchedIdentityMsg = "the gateway-authenticated principal and the token's LF identity disagree - refusing rather than recording an account against either"

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

	api.MyClasGetMyIdentitiesHandler = myClasOps.GetMyIdentitiesHandlerFunc(
		func(params myClasOps.GetMyIdentitiesParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.my_clas.handlers.GetMyIdentities",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}

			currentUsername, _ := principal(authUser)
			if currentUsername == "" {
				log.WithFields(f).Warn(missingUsernameMsg)
				return myClasOps.NewGetMyIdentitiesUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, missingUsernameMsg))
			}

			result, err := service.GetMyIdentities(ctx, currentUsername)
			if err != nil {
				msg := "unable to lookup the identities for the authenticated user"
				log.WithFields(f).WithError(err).Warn(msg)
				return myClasOps.NewGetMyIdentitiesInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			return myClasOps.NewGetMyIdentitiesOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.MyClasBindSigningIdentityHandler = myClasOps.BindSigningIdentityHandlerFunc(
		func(params myClasOps.BindSigningIdentityParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.my_clas.handlers.BindSigningIdentity",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}

			currentUsername, _ := principal(authUser)
			if currentUsername == "" {
				log.WithFields(f).Warn(missingUsernameMsg)
				return myClasOps.NewBindSigningIdentityUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, missingUsernameMsg))
			}

			// Two identities arrive on this request from two places: the one the gateway
			// authenticated and injected, and the one the token carries. Everything below
			// writes using the token's - so if they ever disagree, this endpoint would
			// record the submitted GitHub account against whichever person the gateway did
			// not authenticate. With the account number itself taken on trust, this
			// identity is the only part of the write that anything actually verified, which
			// is why a disagreement has to stop the request rather than be logged.
			if caller := user.VerifiedCallerFromContext(ctx); caller != nil && caller.LFUsername != "" && !strings.EqualFold(caller.LFUsername, currentUsername) {
				f["tokenLFUsername"] = caller.LFUsername
				f["refusalReason"] = ReasonIdentityMismatch
				log.WithFields(f).Warn(mismatchedIdentityMsg)
				return myClasOps.NewBindSigningIdentityForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, ReasonIdentityMismatch))
			}

			if params.Body.GithubID == nil {
				log.WithFields(f).Warn(missingGithubIDMsg)
				return myClasOps.NewBindSigningIdentityBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, missingGithubIDMsg))
			}

			result, err := service.BindSigningIdentity(ctx, *params.Body.GithubID, params.Body.GithubUsername)
			if err != nil {
				// Refusals are mapped one code at a time rather than collapsed into a
				// single failure response. The contributor's next step differs per reason
				// - sign in again, choose again, or contact support - and an operator
				// cannot tell an unauthenticated caller from a contested record through
				// an undifferentiated failure.
				var refusal *Refusal
				if errors.As(err, &refusal) {
					f["refusalReason"] = refusal.Reason
					log.WithFields(f).Warn("refused to record the signing GitHub identity")
					switch refusal.Reason {
					case ReasonIdentityUnavailable:
						return myClasOps.NewBindSigningIdentityForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbiddenWithError(reqID, refusal.Reason, refusal))
					default:
						return myClasOps.NewBindSigningIdentityConflict().WithXRequestID(reqID).WithPayload(utils.ErrorResponseConflictWithError(reqID, refusal.Reason, refusal))
					}
				}

				msg := "unable to record the signing GitHub identity"
				log.WithFields(f).WithError(err).Warn(msg)
				return myClasOps.NewBindSigningIdentityInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			return myClasOps.NewBindSigningIdentityOK().WithXRequestID(reqID).WithPayload(result)
		})
}

func principal(authUser *auth.User) (string, bool) {
	if authUser == nil {
		return "", false
	}
	return authUser.UserName, utils.IsUserAdmin(authUser)
}

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
