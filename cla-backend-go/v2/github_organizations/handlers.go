// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_organizations

import (
	"context"
	"fmt"
	"strings"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/github_organizations"
	"github.com/communitybridge/easycla/cla-backend-go/github"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
)

// Configure setups handlers on api with service
func Configure(api *operations.EasyclaAPI, service Service, eventService events.Service) {
	api.GithubOrganizationsGetProjectGithubOrganizationsHandler = github_organizations.GetProjectGithubOrganizationsHandlerFunc(
		func(params github_organizations.GetProjectGithubOrganizationsParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

			f := logrus.Fields{
				"functionName":   "GitHubOrganizationsGetProjectGithubOrganizationsHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUser":       authUser.UserName,
				"authEmail":      authUser.Email,
				"projectSFID":    params.ProjectSFID,
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Get Project GitHub Organizations with Project scope of %s",
					authUser.UserName, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return github_organizations.NewGetProjectGithubOrganizationsForbidden().WithPayload(
					utils.ErrorResponseForbidden(reqID, msg))
			}

			result, err := service.GetGithubOrganizations(ctx, params.ProjectSFID)
			if err != nil {
				if strings.ContainsAny(err.Error(), "getProjectNotFound") {
					msg := fmt.Sprintf("github organization with project SFID not found: %s", params.ProjectSFID)
					log.WithFields(f).Debug(msg)
					return github_organizations.NewGetProjectGithubOrganizationsNotFound().WithPayload(
						utils.ErrorResponseNotFound(reqID, msg))
				}

				msg := fmt.Sprintf("failed to locate github organization by project SFID: %s, error: %+v", params.ProjectSFID, err)
				log.WithFields(f).Debug(msg)
				return github_organizations.NewGetProjectGithubOrganizationsBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			return github_organizations.NewGetProjectGithubOrganizationsOK().WithPayload(result)
		})

	api.GithubOrganizationsAddProjectGithubOrganizationHandler = github_organizations.AddProjectGithubOrganizationHandlerFunc(
		func(params github_organizations.AddProjectGithubOrganizationParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint

			f := logrus.Fields{
				"functionName":   "GitHubOrganizationsAddProjectGithubOrganizationHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUser":       authUser.UserName,
				"authEmail":      authUser.Email,
				"projectSFID":    params.ProjectSFID,
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Add Project GitHub Organizations with Project scope of %s",
					authUser.UserName, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return github_organizations.NewAddProjectGithubOrganizationForbidden().WithPayload(
					utils.ErrorResponseForbidden(reqID, msg))
			}

			// Quick check of the parameters
			if params.Body == nil || params.Body.OrganizationName == nil {
				msg := fmt.Sprintf("missing organization name in body: %+v", params.Body)
				log.WithFields(f).Warn(msg)
				return github_organizations.NewAddProjectGithubOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequest(reqID, msg))
			}
			f["organizationName"] = utils.StringValue(params.Body.OrganizationName)

			if params.Body.AutoEnabled == nil {
				msg := fmt.Sprintf("missing autoEnabled name in body: %+v", params.Body)
				log.WithFields(f).Warn(msg)
				return github_organizations.NewAddProjectGithubOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequest(reqID, msg))
			}
			f["autoEnabled"] = utils.BoolValue(params.Body.AutoEnabled)
			f["autoEnabledClaGroupID"] = params.Body.AutoEnabledClaGroupID

			log.WithFields(f).Debug("Loading organization by name")
			_, err := github.GetOrganization(ctx, *params.Body.OrganizationName)
			if err != nil {
				msg := fmt.Sprintf("unable to load organization by name: %s", utils.StringValue(params.Body.OrganizationName))
				log.WithFields(f).WithError(err).Warn(msg)
				return github_organizations.NewAddProjectGithubOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			if !utils.ValidateAutoEnabledClaGroupID(params.Body.AutoEnabled, params.Body.AutoEnabledClaGroupID) {
				msg := "AutoEnabledClaGroupID can't be empty when AutoEnabled"
				log.WithFields(f).WithError(err).Warn(msg)
				return github_organizations.NewAddProjectGithubOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			result, err := service.AddGithubOrganization(ctx, params.ProjectSFID, params.Body)
			if err != nil {
				msg := fmt.Sprintf("unable to add github organization, error: %+v", err)
				log.WithFields(f).WithError(err).Warn(msg)
				return github_organizations.NewAddProjectGithubOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			// Log the event
			eventService.LogEvent(&events.LogEventArgs{
				LfUsername:        authUser.UserName,
				EventType:         events.GitHubOrganizationAdded,
				ExternalProjectID: params.ProjectSFID,
				EventData: &events.GitHubOrganizationAddedEventData{
					GitHubOrganizationName: *params.Body.OrganizationName,
				},
			})

			return github_organizations.NewAddProjectGithubOrganizationOK().WithPayload(result)
		})

	api.GithubOrganizationsDeleteProjectGithubOrganizationHandler = github_organizations.DeleteProjectGithubOrganizationHandlerFunc(
		func(params github_organizations.DeleteProjectGithubOrganizationParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				return github_organizations.NewDeleteProjectGithubOrganizationForbidden().WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to Delete Project GitHub Organizations with Project scope of %s",
						authUser.UserName, params.ProjectSFID),
					XRequestID: reqID,
				})
			}

			err := service.DeleteGithubOrganization(ctx, params.ProjectSFID, params.OrgName)
			if err != nil {
				if strings.Contains(err.Error(), "getProjectNotFound") {
					return github_organizations.NewDeleteProjectGithubOrganizationNotFound().WithPayload(&models.ErrorResponse{
						Code:       "404",
						Message:    fmt.Sprintf("project not found with given ID. [%s]", params.ProjectSFID),
						XRequestID: reqID,
					})
				}
				return github_organizations.NewDeleteProjectGithubOrganizationBadRequest().WithPayload(errorResponse(reqID, err))
			}

			eventService.LogEvent(&events.LogEventArgs{
				LfUsername:        authUser.UserName,
				EventType:         events.GitHubOrganizationDeleted,
				ExternalProjectID: params.ProjectSFID,
				EventData: &events.GitHubOrganizationDeletedEventData{
					GitHubOrganizationName: params.OrgName,
				},
			})

			return github_organizations.NewDeleteProjectGithubOrganizationNoContent()
		})

	api.GithubOrganizationsUpdateProjectGithubOrganizationConfigHandler = github_organizations.UpdateProjectGithubOrganizationConfigHandlerFunc(
		func(params github_organizations.UpdateProjectGithubOrganizationConfigParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				return github_organizations.NewUpdateProjectGithubOrganizationConfigForbidden().WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to Update Project GitHub Organizations with Project scope of %s",
						authUser.UserName, params.ProjectSFID),
					XRequestID: reqID,
				})
			}

			if params.Body.AutoEnabled == nil {
				return github_organizations.NewUpdateProjectGithubOrganizationConfigBadRequest().WithPayload(&models.ErrorResponse{
					Code:       "400",
					Message:    "EasyCLA - 400 Bad Request - missing auto enable value in body",
					XRequestID: reqID,
				})
			}

			if !utils.ValidateAutoEnabledClaGroupID(params.Body.AutoEnabled, params.Body.AutoEnabledClaGroupID) {
				return github_organizations.NewAddProjectGithubOrganizationBadRequest().WithPayload(&models.ErrorResponse{
					Code:    "400",
					Message: "EasyCLA - 400 Bad Request - AutoEnabledClaGroupID can't be empty when AutoEnabled",
				})
			}

			err := service.UpdateGithubOrganization(ctx, params.ProjectSFID, params.OrgName, *params.Body.AutoEnabled, params.Body.AutoEnabledClaGroupID, params.Body.BranchProtectionEnabled)
			if err != nil {
				return github_organizations.NewUpdateProjectGithubOrganizationConfigBadRequest().WithPayload(errorResponse(reqID, err))
			}

			eventService.LogEvent(&events.LogEventArgs{
				LfUsername:        authUser.UserName,
				EventType:         events.GitHubOrganizationUpdated,
				ExternalProjectID: params.ProjectSFID,
				EventData: &events.GitHubOrganizationUpdatedEventData{
					GitHubOrganizationName: params.OrgName,
					AutoEnabled:            *params.Body.AutoEnabled,
				},
			})

			return github_organizations.NewUpdateProjectGithubOrganizationConfigOK()
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
