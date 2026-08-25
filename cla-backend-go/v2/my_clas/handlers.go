// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"errors"
	"net/http"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime/middleware"
	claAuth "github.com/linuxfoundation/easycla/cla-backend-go/auth"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	myClasOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/my_clas"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

const missingUsernameMsg = "the authenticated principal carries no username - unable to determine whose CLAs to look up"
const missingIdentityMsg = "no identity provided - provide at least one of lfUsername, email, secondaryEmail, githubId, githubUsername, gitlabId, gitlabUsername, gerritUsername"
const unverifiedCallerMsg = "unable to verify the caller's bearer token"
const notOwnedEclaMsg = "no signed ECLA with the given signature ID belongs to the provided identity"

// CallerVerifier re-verifies the request bearer token in-handler - see auth.TrustedCallerVerifier
type CallerVerifier interface {
	Enabled() bool
	Verify(authorization string) (*claAuth.TrustedCaller, error)
}

// Configure sets up the My CLAs API handlers
//
//nolint:gocyclo
func Configure(api *operations.EasyclaAPI, service Service, callerVerifier CallerVerifier) {
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

			trustedCaller, err := verifyCaller(callerVerifier, params.HTTPRequest, f)
			if err != nil {
				log.WithFields(f).WithError(err).Warn(unverifiedCallerMsg)
				return myClasOps.NewGetMyClasUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, unverifiedCallerMsg))
			}

			currentUsername, admin := principal(authUser)
			trusted := trustedCaller != nil && trustedCaller.Trusted
			if !admin && !trusted && currentUsername == "" {
				log.WithFields(f).Warn(missingUsernameMsg)
				return myClasOps.NewGetMyClasUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, missingUsernameMsg))
			}

			requested := newIdentity(params.LfUsername, params.Email, params.SecondaryEmail, params.GithubID, params.GithubUsername, params.GitlabID, params.GitlabUsername, params.GerritUsername)
			if (admin || trusted) && currentUsername == "" && requested.IsEmpty() {
				log.WithFields(f).Warn(missingIdentityMsg)
				return myClasOps.NewGetMyClasBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, missingIdentityMsg))
			}
			logCallerIdentity(f, trustedCaller, requested)

			result, err := service.GetMyClas(ctx, &Caller{Username: currentUsername, Admin: admin, Trusted: trusted}, requested)
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

			trustedCaller, err := verifyCaller(callerVerifier, params.HTTPRequest, f)
			if err != nil {
				log.WithFields(f).WithError(err).Warn(unverifiedCallerMsg)
				return myClasOps.NewGetMyClaPdfUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, unverifiedCallerMsg))
			}

			currentUsername, admin := principal(authUser)
			trusted := trustedCaller != nil && trustedCaller.Trusted
			if !admin && !trusted && currentUsername == "" {
				log.WithFields(f).Warn(missingUsernameMsg)
				return myClasOps.NewGetMyClaPdfUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, missingUsernameMsg))
			}

			requested := newIdentity(params.LfUsername, params.Email, params.SecondaryEmail, params.GithubID, params.GithubUsername, params.GitlabID, params.GitlabUsername, params.GerritUsername)
			if (admin || trusted) && currentUsername == "" && requested.IsEmpty() {
				log.WithFields(f).Warn(missingIdentityMsg)
				return myClasOps.NewGetMyClaPdfBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, missingIdentityMsg))
			}
			logCallerIdentity(f, trustedCaller, requested)

			result, err := service.GetMyClaPdfURL(ctx, &Caller{Username: currentUsername, Admin: admin, Trusted: trusted}, requested, params.SignatureID)
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

	api.MyClasGetMyClaManagersHandler = myClasOps.GetMyClaManagersHandlerFunc(
		func(params myClasOps.GetMyClaManagersParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.my_clas.handlers.GetMyClaManagers",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
				"signatureID":    params.SignatureID,
			}

			trustedCaller, err := verifyCaller(callerVerifier, params.HTTPRequest, f)
			if err != nil {
				log.WithFields(f).WithError(err).Warn(unverifiedCallerMsg)
				return myClasOps.NewGetMyClaManagersUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, unverifiedCallerMsg))
			}

			currentUsername, admin := principal(authUser)
			trusted := trustedCaller != nil && trustedCaller.Trusted
			if !admin && !trusted && currentUsername == "" {
				log.WithFields(f).Warn(missingUsernameMsg)
				return myClasOps.NewGetMyClaManagersUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, missingUsernameMsg))
			}

			requested := newIdentity(params.LfUsername, params.Email, params.SecondaryEmail, params.GithubID, params.GithubUsername, params.GitlabID, params.GitlabUsername, params.GerritUsername)
			if (admin || trusted) && currentUsername == "" && requested.IsEmpty() {
				log.WithFields(f).Warn(missingIdentityMsg)
				return myClasOps.NewGetMyClaManagersBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, missingIdentityMsg))
			}
			logCallerIdentity(f, trustedCaller, requested)

			result, err := service.GetMyClaManagers(ctx, &Caller{Username: currentUsername, Admin: admin, Trusted: trusted}, requested, params.SignatureID)
			if err != nil {
				msg := "unable to lookup the CLA managers for the given signature"
				log.WithFields(f).WithError(err).Warn(msg)
				return myClasOps.NewGetMyClaManagersInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}
			if result == nil {
				msg := notOwnedEclaMsg
				log.WithFields(f).Warn(msg)
				return myClasOps.NewGetMyClaManagersNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}

			return myClasOps.NewGetMyClaManagersOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.MyClasCreateMyClaManagerRequestHandler = myClasOps.CreateMyClaManagerRequestHandlerFunc(
		func(params myClasOps.CreateMyClaManagerRequestParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.my_clas.handlers.CreateMyClaManagerRequest",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
				"signatureID":    params.SignatureID,
			}

			trustedCaller, err := verifyCaller(callerVerifier, params.HTTPRequest, f)
			if err != nil {
				log.WithFields(f).WithError(err).Warn(unverifiedCallerMsg)
				return myClasOps.NewCreateMyClaManagerRequestUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, unverifiedCallerMsg))
			}

			currentUsername, admin := principal(authUser)
			trusted := trustedCaller != nil && trustedCaller.Trusted
			if !admin && !trusted && currentUsername == "" {
				log.WithFields(f).Warn(missingUsernameMsg)
				return myClasOps.NewCreateMyClaManagerRequestUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, missingUsernameMsg))
			}

			requested := newIdentity(params.LfUsername, params.Email, params.SecondaryEmail, params.GithubID, params.GithubUsername, params.GitlabID, params.GitlabUsername, params.GerritUsername)
			if (admin || trusted) && currentUsername == "" && requested.IsEmpty() {
				log.WithFields(f).Warn(missingIdentityMsg)
				return myClasOps.NewCreateMyClaManagerRequestBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, missingIdentityMsg))
			}
			logCallerIdentity(f, trustedCaller, requested)

			result, err := service.CreateMyClaManagerRequest(ctx, &Caller{Username: currentUsername, Admin: admin, Trusted: trusted}, requested, params.SignatureID, &params.Body)
			if err != nil {
				if errors.Is(err, ErrInvalidRecipients) || errors.Is(err, ErrMissingMessage) {
					log.WithFields(f).WithError(err).Warn("invalid CLA manager request input")
					return myClasOps.NewCreateMyClaManagerRequestBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, err.Error()))
				}
				msg := "unable to create the CLA manager request"
				log.WithFields(f).WithError(err).Warn(msg)
				return myClasOps.NewCreateMyClaManagerRequestInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}
			if result == nil {
				msg := notOwnedEclaMsg
				log.WithFields(f).Warn(msg)
				return myClasOps.NewCreateMyClaManagerRequestNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}

			return myClasOps.NewCreateMyClaManagerRequestOK().WithXRequestID(reqID).WithPayload(result)
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

			if _, err := verifyCaller(callerVerifier, params.HTTPRequest, f); err != nil {
				log.WithFields(f).WithError(err).Warn(unverifiedCallerMsg)
				return myClasOps.NewGetMyIdentitiesUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, unverifiedCallerMsg))
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
}

// verifyCaller re-verifies the bearer token because /v4 otherwise trusts its invoke path
// unconditionally: the gateway-injected X-ACL/X-USERNAME headers are decoded but never
// signature-checked, so anything able to invoke the lambda directly could forge them. Returns
// (nil, nil) while no allow-list is configured and no bearer token is required.
func verifyCaller(callerVerifier CallerVerifier, r *http.Request, f logrus.Fields) (*claAuth.TrustedCaller, error) {
	if callerVerifier == nil || !callerVerifier.Enabled() {
		return nil, nil
	}

	authorization := ""
	if r != nil {
		authorization = r.Header.Get("Authorization")
	}
	trustedCaller, err := callerVerifier.Verify(authorization)
	if err != nil {
		return nil, err
	}
	if trustedCaller == nil {
		return nil, errors.New("the caller verifier returned no result")
	}

	f["callerClientID"] = trustedCaller.ClientID
	f["callerSubject"] = trustedCaller.Subject
	f["trustedCaller"] = trustedCaller.Trusted
	return trustedCaller, nil
}

func logCallerIdentity(f logrus.Fields, trustedCaller *claAuth.TrustedCaller, requested *Identity) {
	if trustedCaller == nil {
		return
	}
	if trustedCaller.Trusted {
		log.WithFields(f).Infof("trusted caller requested the identities: %s", requested.Summary())
		return
	}
	log.WithFields(f).Debugf("untrusted caller requested the identities: %s", requested.Summary())
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
