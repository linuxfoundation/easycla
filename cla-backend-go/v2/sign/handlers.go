// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/sign"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
)

var (
	// payload is the payload for the docusign callback
	iclaGitHubPayload   []byte
	cclaDocusignPayload []byte
)

// DocusignMiddleware is used to get access to xml request body
func DocusignMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := logrus.Fields{
			"functionName": "v2.sign.handlers.docusignMiddleware",
		}
		var err error
		log.WithFields(f).Debug("docusign middleware...")
		iclaGitHubPayload, err = io.ReadAll(r.Body)
		if err != nil {
			log.Warnf("unable to read request body")
			return
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewBuffer(iclaGitHubPayload))
		log.WithFields(f).Debugf("docusign middleware...payload: %s", string(iclaGitHubPayload))
		// call the next middleware
		next.ServeHTTP(w, r)
	})
}

// CCLADocusignMiddleware used to set CCLA middleware
func CCLADocusignMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := logrus.Fields{
			"functionName": "v2.sign.handlers.cclaDocusignMiddleware",
		}
		var err error
		log.WithFields(f).Debug("docusign middleware...")
		cclaDocusignPayload, err = io.ReadAll(r.Body)
		if err != nil {
			log.Warnf("unable to read request body")
			return
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewBuffer(cclaDocusignPayload))
		log.WithFields(f).Debugf("docusign middleware...payload: %s", string(cclaDocusignPayload))
		// call the next middleware
		next.ServeHTTP(w, r)
	})
}

// Configure API call
func Configure(api *operations.EasyclaAPI, service Service, userService users.Service) {
	// Retrieve a list of available templates
	api.SignRequestCorporateSignatureHandler = sign.RequestCorporateSignatureHandlerFunc(
		func(params sign.RequestCorporateSignatureParams, user *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, user) // nolint
			utils.SetAuthUserProperties(user, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.sign.handlers.SignRequestCorporateSignatureHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"CompanyID":      params.Input.CompanySfid,
				"ProjectSFID":    params.Input.ProjectSfid,
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}

			if !utils.IsUserAuthorizedForProjectOrganizationTree(ctx, user, utils.StringValue(params.Input.ProjectSfid), utils.StringValue(params.Input.CompanySfid), utils.DISALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Request Corporate Signature with Project|Organization scope tree of %s | %s - allow admin scope: false",
					user.UserName, utils.StringValue(params.Input.ProjectSfid), utils.StringValue(params.Input.CompanySfid))
				log.WithFields(f).Warn(msg)
				return sign.NewRequestCorporateSignatureForbidden().WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			resp, err := service.RequestCorporateSignature(ctx, utils.StringValue(params.XUSERNAME), params.Authorization, params.Input)
			if err != nil {
				if strings.Contains(err.Error(), "does not exist") {
					return sign.NewRequestCorporateSignatureNotFound().WithPayload(errorResponse(reqID, err))
				}
				if strings.Contains(err.Error(), "internal server error") {
					return sign.NewRequestCorporateSignatureInternalServerError().WithPayload(errorResponse(reqID, err))
				}
				if strings.Contains(err.Error(), "is sanctioned") {
					desc := "We’re sorry, but you are currently unable to sign the Corporate Contributor License Agreement (CCLA). If you believe this may be an error, please reach out to support"
					return sign.NewRequestCorporateSignatureForbidden().WithPayload(errorResponseWithDesc(reqID, err, desc))
				}
				if err == projects_cla_groups.ErrProjectNotAssociatedWithClaGroup {
					return sign.NewRequestCorporateSignatureBadRequest().WithPayload(errorResponse(reqID, err))
				}
				if err == ErrCCLANotEnabled || err == ErrTemplateNotConfigured {
					return sign.NewRequestCorporateSignatureBadRequest().WithPayload(errorResponse(reqID, err))
				}
				if _, ok := err.(*organizations.ListOrgUsrAdminScopesNotFound); ok {
					formatErr := errors.New("user role scopes not found for cla-signatory role ")
					return sign.NewRequestCorporateSignatureNotFound().WithPayload(errorResponse(reqID, formatErr))
				}
				if _, ok := err.(*organizations.CreateOrgUsrRoleScopesConflict); ok {
					formatErr := errors.New("user role scope conflict")
					return sign.NewRequestCorporateSignatureConflict().WithPayload(errorResponse(reqID, formatErr))
				}
				if err == ErrNotInOrg {
					return sign.NewRequestCorporateSignatureConflict().WithPayload(errorResponse(reqID, err))
				}
				return sign.NewRequestCorporateSignatureBadRequest().WithPayload(errorResponse(reqID, err))
			}
			return sign.NewRequestCorporateSignatureOK().WithPayload(resp)
		})

	api.SignRequestIndividualSignatureHandler = sign.RequestIndividualSignatureHandlerFunc(
		func(params sign.RequestIndividualSignatureParams) middleware.Responder {
			reqId := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTIDKey, reqId)
			f := logrus.Fields{
				"functionName":   "v2.sign.handlers.SignRequestIndividualSignatureHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"projectID":      params.Input.ProjectID,
				"returnURL":      params.Input.ReturnURL,
				"returnURLType":  params.Input.ReturnURLType,
				"userID":         params.Input.UserID,
			}
			var resp *models.IndividualSignatureOutput
			var err error
			var preferredEmail string

			if strings.ToLower(params.Input.ReturnURLType) == Github || strings.ToLower(params.Input.ReturnURLType) == Gitlab {
				log.WithFields(f).Debug("fetching user emails")
				user, userErr := userService.GetUser(*params.Input.UserID)
				if userErr != nil {
					return sign.NewRequestIndividualSignatureBadRequest().WithPayload(errorResponse(reqId, userErr))
				}
				if len(user.Emails) == 0 {
					msg := "no emails found"
					log.WithFields(f).Warn(msg)
					return sign.NewRequestIndividualSignatureBadRequest().WithPayload(errorResponse(reqId, errors.New(msg)))
				}
				preferredEmail = user.Emails[0]
				log.WithFields(f).Debug("requesting individual signature for github/gitlab")
				resp, err = service.RequestIndividualSignature(ctx, params.Input, preferredEmail)
			} else if strings.ToLower(params.Input.ReturnURLType) == "gerrit" {
				log.WithFields(f).Debug("requesting individual signature for gerrit")
				resp, err = service.RequestIndividualSignatureGerrit(ctx, params.Input)
			} else {
				msg := fmt.Sprintf("invalid return URL type: %s", params.Input.ReturnURLType)
				log.WithFields(f).Warn(msg)
				return sign.NewRequestIndividualSignatureBadRequest().WithPayload(errorResponse(reqId, errors.New(msg)))
			}
			if err != nil {
				return sign.NewRequestIndividualSignatureBadRequest().WithPayload(errorResponse(reqId, err))
			}
			return sign.NewRequestIndividualSignatureOK().WithPayload(resp)
		})

	api.SignIclaCallbackGithubHandler = sign.IclaCallbackGithubHandlerFunc(
		func(params sign.IclaCallbackGithubParams) middleware.Responder {
			reqId := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTIDKey, reqId)

			f := logrus.Fields{
				"functionName":   "v2.sign.handlers.SignIclaCallbackGithubHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			}

			err := service.SignedIndividualCallbackGithub(ctx, iclaGitHubPayload, params.InstallationID, params.ChangeRequestID, params.GithubRepositoryID)
			if err != nil {
				return sign.NewIclaCallbackGithubBadRequest()
			}

			log.WithFields(f).Debug("github callback")
			// err := service.SignedIndividualCallbackGithub(ctx, payload, params.UserID)
			return sign.NewCclaCallbackOK()
		})

	api.SignIclaCallbackGitlabHandler = sign.IclaCallbackGitlabHandlerFunc(
		func(params sign.IclaCallbackGitlabParams) middleware.Responder {
			reqId := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTIDKey, reqId)
			f := logrus.Fields{
				"functionName":   "v2.sign.handlers.SignIclaCallbackGitlabHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			}
			log.WithFields(f).Debug("gitlab callback")

			err := service.SignedIndividualCallbackGitlab(ctx, iclaGitHubPayload, params.UserID, params.OrganizationID, params.GitlabRepositoryID, params.MergeRequestID)
			if err != nil {
				return sign.NewIclaCallbackGitlabBadRequest()
			}
			return sign.NewCclaCallbackOK()
		})

	api.SignIclaCallbackGerritHandler = sign.IclaCallbackGerritHandlerFunc(
		func(params sign.IclaCallbackGerritParams) middleware.Responder {
			reqId := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTIDKey, reqId)
			f := logrus.Fields{
				"functionName":   "v2.sign.handlers.SignIclaCallbackGerritHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			}

			log.WithFields(f).Debug("gerrit callback")

			err := service.SignedIndividualCallbackGerrit(ctx, iclaGitHubPayload, params.UserID)
			if err != nil {
				return sign.NewIclaCallbackGerritBadRequest()
			}
			return sign.NewCclaCallbackOK()
		})

	api.SignCclaCallbackHandler = sign.CclaCallbackHandlerFunc(
		func(params sign.CclaCallbackParams) middleware.Responder {
			reqId := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTIDKey, reqId)
			f := logrus.Fields{
				"functionName":   "v2.sign.handlers.SignCclaCallbackHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			}

			log.WithFields(f).Debug("ccla callback")
			err := service.SignedCorporateCallback(ctx, cclaDocusignPayload, params.CompanyID, params.ProjectID)
			if err != nil {
				return sign.NewCclaCallbackBadRequest()
			}
			return sign.NewCclaCallbackOK()
		})

	api.SignGetUserActiveSignatureHandler = sign.GetUserActiveSignatureHandlerFunc(
		func(params sign.GetUserActiveSignatureParams) middleware.Responder {
			reqId := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTIDKey, reqId)
			f := logrus.Fields{
				"functionName":   "v2.sign.handlers.SignGetUserActiveSignatureHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"userID":         params.UserID,
			}
			var resp *models.UserActiveSignature
			var err error

			log.WithFields(f).Debug("getting user active signature")
			resp, err = service.GetUserActiveSignature(ctx, params.UserID)
			if err != nil {
				return sign.NewGetUserActiveSignatureBadRequest().WithPayload(errorResponse(reqId, err))
			}
			return sign.NewGetUserActiveSignatureOK().WithPayload(resp)
		})

}

type codedResponse interface {
	Code() string
}

func errorResponse(reqID string, err error) *models.ErrorResponse {
	code := ""
	if e, ok := err.(codedResponse); ok {
		code = e.Code()
	}

	e := models.ErrorResponse{
		Code:       code,
		Message:    err.Error(),
		XRequestID: reqID,
	}

	return &e
}

func errorResponseWithDesc(reqID string, err error, desc string) *models.ErrorResponse {
	code := ""
	if e, ok := err.(codedResponse); ok {
		code = e.Code()
	}

	e := models.ErrorResponse{
		Code:       code,
		Message:    err.Error() + "\n" + desc,
		XRequestID: reqID,
	}

	return &e
}
