// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package self_serve_sign

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	selfServeSignOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/self_serve_sign"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
	v2Sign "github.com/linuxfoundation/easycla/cla-backend-go/v2/sign"
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

	api.SelfServeSignSelfServeRequestCorporateSignatureHandler = selfServeSignOps.SelfServeRequestCorporateSignatureHandlerFunc(
		func(params selfServeSignOps.SelfServeRequestCorporateSignatureParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.self_serve_sign.handlers.SelfServeRequestCorporateSignature",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"projectSFID":    utils.StringValue(params.Input.ProjectSfid),
				"companySFID":    utils.StringValue(params.Input.CompanySfid),
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}

			if !utils.IsUserAuthorizedForProjectOrganizationTree(ctx, authUser, utils.StringValue(params.Input.ProjectSfid), utils.StringValue(params.Input.CompanySfid), utils.DISALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Request Corporate Signature with Project|Organization scope tree of %s | %s - allow admin scope: false",
					authUser.UserName, utils.StringValue(params.Input.ProjectSfid), utils.StringValue(params.Input.CompanySfid))
				log.WithFields(f).Warn(msg)
				return selfServeSignOps.NewSelfServeRequestCorporateSignatureForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			result, err := service.RequestCorporateSignature(ctx, utils.StringValue(params.XUSERNAME), params.Authorization, &params.Input)
			if err != nil {
				log.WithFields(f).WithError(err).Warn("unable to request the corporate signature")
				return requestCorporateSignatureError(reqID, err)
			}

			return selfServeSignOps.NewSelfServeRequestCorporateSignatureOK().WithXRequestID(reqID).WithPayload(result)
		})
}

// requestCorporateSignatureError maps the errors of the shared corporate signing service to the
// same statuses as /v4/request-corporate-signature, plus the Self Serve attestation error
func requestCorporateSignatureError(reqID string, err error) middleware.Responder {
	switch {
	case errors.Is(err, ErrAttestationRequired):
		return selfServeSignOps.NewSelfServeRequestCorporateSignatureBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, err.Error()))
	case strings.Contains(err.Error(), "does not exist"):
		return selfServeSignOps.NewSelfServeRequestCorporateSignatureNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, err.Error()))
	case strings.Contains(err.Error(), "internal server error"):
		return selfServeSignOps.NewSelfServeRequestCorporateSignatureInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerError(reqID, err.Error()))
	case strings.Contains(err.Error(), "requires further review for trade compliance"):
		desc := "We're sorry, but this organization requires additional trade compliance review, so the Contributor License Agreement (CLA) cannot be completed at this time. If you believe this is an error, please contact EasyCLA Support via the chat widget."
		return selfServeSignOps.NewSelfServeRequestCorporateSignatureForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, err.Error()+"\n"+desc))
	case errors.Is(err, projects_cla_groups.ErrProjectNotAssociatedWithClaGroup), errors.Is(err, v2Sign.ErrCCLANotEnabled), errors.Is(err, v2Sign.ErrTemplateNotConfigured):
		return selfServeSignOps.NewSelfServeRequestCorporateSignatureBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, err.Error()))
	}
	var scopesNotFound *organizations.ListOrgUsrAdminScopesNotFound
	if errors.As(err, &scopesNotFound) {
		return selfServeSignOps.NewSelfServeRequestCorporateSignatureNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, "user role scopes not found for cla-signatory role"))
	}
	var roleScopesConflict *organizations.CreateOrgUsrRoleScopesConflict
	if errors.As(err, &roleScopesConflict) {
		return selfServeSignOps.NewSelfServeRequestCorporateSignatureConflict().WithXRequestID(reqID).WithPayload(utils.ErrorResponseConflict(reqID, "user role scope conflict"))
	}
	return selfServeSignOps.NewSelfServeRequestCorporateSignatureBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, err.Error()))
}

func principal(authUser *auth.User) (string, string, bool) {
	if authUser == nil {
		return "", "", false
	}
	return authUser.UserName, authUser.Email, utils.IsUserAdmin(authUser)
}
