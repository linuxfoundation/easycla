// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/sign"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/communitybridge/easycla/cla-backend-go/v2/organization-service/client/organizations"
	"github.com/go-openapi/runtime/middleware"
)

// Configure API call
func Configure(api *operations.EasyclaAPI, service Service) {
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
			var preferredEmail string = ""

			session := getRequestSession(params.HTTPRequest)
			if session == nil {
				msg := "session not found"
				log.WithFields(f).Warn(msg)
				return sign.NewRequestIndividualSignatureBadRequest().WithPayload(errorResponse(reqId, errors.New(msg)))
			}

			clientID := utils.GetProperty("GH_OAUTH_CLIENT_ID")
			if clientID == "" {
				msg := "client id not found"
				log.WithFields(f).Warn(msg)
				return sign.NewRequestIndividualSignatureBadRequest().WithPayload(errorResponse(reqId, errors.New(msg)))
			}

			if strings.ToLower(params.Input.ReturnURLType) == "github" || strings.ToLower(params.Input.ReturnURLType) == "gitlab" {
				if strings.ToLower(params.Input.ReturnURLType) == "github" {
					log.WithFields(f).Debug("fetching github emails")
					emails, fetchErr := fetchGithubEmails(session, clientID)
					if fetchErr != nil {
						return sign.NewRequestIndividualSignatureBadRequest().WithPayload(errorResponse(reqId, err))
					}

					if len(emails) == 0 {
						msg := "no emails found"
						log.WithFields(f).Warn(msg)
						return sign.NewRequestIndividualSignatureBadRequest().WithPayload(errorResponse(reqId, errors.New(msg)))
					}
					for _, email := range emails {
						if email["verified"].(bool) && email["primary"].(bool) {
							if emailVal, ok := email["email"].(string); ok {
								preferredEmail = emailVal
							}
							break
						}
					}
				} else {
					log.WithFields(f).Debug("fetching gitlab emails")
					preferredEmail = "" //TODO: fetch gitlab emails for gitlab
				}

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

func getRequestSession(req *http.Request) map[string]interface{} {
	session := req.Context().Value("session")
	if session == nil {
		return nil
	}
	return session.(map[string]interface{})
}

func fetchGithubEmails(session map[string]interface{}, clientID string) ([]map[string]interface{}, error) {
	var emails []map[string]interface{}
	var token string

	if tokenVal, ok := session["token"].(string); ok {
		token = tokenVal
	} else {
		return emails, nil
	}

	if token == "" {
		return emails, nil
	}

	oauth2Config := oauth2.Config{
		ClientID: clientID,
	}

	oauth2Token := &oauth2.Token{
		AccessToken: token,
	}

	client := oauth2Config.Client(context.Background(), oauth2Token)

	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return emails, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return emails, err
	}

	err = json.NewDecoder(resp.Body).Decode(&emails)
	if err != nil {
		return emails, err
	}

	return emails, err
}
