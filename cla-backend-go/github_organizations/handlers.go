// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_organizations

import (
	"context"
	"errors"
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"

	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/github_organizations"
	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	v2ProjectServiceClient "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service/client/project"
)

// Configure setups handlers on api with service
func Configure(api *operations.ClaAPI, service ServiceInterface, eventService events.Service) {
	api.GithubOrganizationsGetProjectGithubOrganizationsHandler = github_organizations.GetProjectGithubOrganizationsHandlerFunc(
		func(params github_organizations.GetProjectGithubOrganizationsParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

			result, err := service.GetGitHubOrganizations(ctx, params.ProjectSFID)
			if err != nil {
				if _, ok := err.(*v2ProjectServiceClient.GetProjectNotFound); ok {
					return github_organizations.NewGetProjectGithubOrganizationsNotFound().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
						Code:    "404",
						Message: fmt.Sprintf("project not found with given ID. [%s]", params.ProjectSFID),
					})
				}
				return github_organizations.NewGetProjectGithubOrganizationsBadRequest().WithPayload(errorResponse(err))
			}
			return github_organizations.NewGetProjectGithubOrganizationsOK().WithPayload(result)
		})

	api.GithubOrganizationsAddProjectGithubOrganizationHandler = github_organizations.AddProjectGithubOrganizationHandlerFunc(
		func(params github_organizations.AddProjectGithubOrganizationParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

			if params.Body.OrganizationName == nil {
				return github_organizations.NewAddProjectGithubOrganizationBadRequest().WithPayload(&models.ErrorResponse{
					Code:    "400",
					Message: "EasyCLA - 400 Bad Request - Missing input Organization Name",
				})
			}

			if !utils.ValidateAutoEnabledClaGroupID(*params.Body.AutoEnabled, params.Body.AutoEnabledClaGroupID) {
				return github_organizations.NewAddProjectGithubOrganizationBadRequest().WithPayload(&models.ErrorResponse{
					Code:    "400",
					Message: "EasyCLA - 400 Bad Request - AutoEnabledClaGroupID can't be empty when AutoEnabled",
				})
			}

			_, err := github.GetOrganization(ctx, *params.Body.OrganizationName)
			if err != nil {
				return github_organizations.NewAddProjectGithubOrganizationNotFound().WithPayload(errorResponse(err))
			}

			result, err := service.AddGitHubOrganization(ctx, params.ProjectSFID, params.Body)
			if err != nil {
				if _, ok := err.(*v2ProjectServiceClient.GetProjectNotFound); ok {
					return github_organizations.NewAddProjectGithubOrganizationNotFound().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
						Code:    "404",
						Message: fmt.Sprintf("project not found with given ID. [%s]", params.ProjectSFID),
					})
				}

				if errors.Is(err, projects_cla_groups.ErrCLAGroupDoesNotExist) {
					return github_organizations.NewAddProjectGithubOrganizationNotFound().WithPayload(errorResponse(err))
				}

				return github_organizations.NewAddProjectGithubOrganizationBadRequest().WithPayload(errorResponse(err))
			}

			autoEnabled := false
			if params.Body.AutoEnabled != nil {
				autoEnabled = *params.Body.AutoEnabled
			}
			branchProtectionEnabled := false
			if params.Body.BranchProtectionEnabled != nil {
				branchProtectionEnabled = *params.Body.BranchProtectionEnabled
			}
			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
				UserID:      claUser.UserID,
				EventType:   events.GitHubOrganizationAdded,
				ProjectSFID: params.ProjectSFID,
				LfUsername:  claUser.LFUsername,
				EventData: &events.GitHubOrganizationAddedEventData{
					GitHubOrganizationName:  *params.Body.OrganizationName,
					AutoEnabled:             autoEnabled,
					AutoEnabledClaGroupID:   params.Body.AutoEnabledClaGroupID,
					BranchProtectionEnabled: branchProtectionEnabled,
				},
			})
			return github_organizations.NewAddProjectGithubOrganizationOK().WithPayload(result)
		})

	api.GithubOrganizationsDeleteProjectGithubOrganizationHandler = github_organizations.DeleteProjectGithubOrganizationHandlerFunc(
		func(params github_organizations.DeleteProjectGithubOrganizationParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

			_, err := github.GetOrganization(ctx, params.OrgName)
			if err != nil {
				return github_organizations.NewDeleteProjectGithubOrganizationNotFound().WithPayload(errorResponse(err))
			}

			err = service.DeleteGitHubOrganization(ctx, params.ProjectSFID, params.OrgName)
			if err != nil {
				if _, ok := err.(*v2ProjectServiceClient.GetProjectNotFound); ok {
					return github_organizations.NewDeleteProjectGithubOrganizationNotFound().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
						Code:    "404",
						Message: fmt.Sprintf("project not found with given ID. [%s]", params.ProjectSFID),
					})
				}
				return github_organizations.NewDeleteProjectGithubOrganizationBadRequest().WithPayload(errorResponse(err))
			}

			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
				UserID:      claUser.UserID,
				EventType:   events.GitHubOrganizationDeleted,
				ProjectSFID: params.ProjectSFID,
				LfUsername:  claUser.LFUsername,
				EventData: &events.GitHubOrganizationDeletedEventData{
					GitHubOrganizationName: params.OrgName,
				},
			})
			return github_organizations.NewDeleteProjectGithubOrganizationNoContent()
		})

	api.GithubOrganizationsUpdateProjectGithubOrganizationConfigHandler = github_organizations.UpdateProjectGithubOrganizationConfigHandlerFunc(
		func(params github_organizations.UpdateProjectGithubOrganizationConfigParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			if params.Body.AutoEnabled == nil {
				return github_organizations.NewUpdateProjectGithubOrganizationConfigBadRequest().WithPayload(&models.ErrorResponse{
					Code:    "400",
					Message: "EasyCLA - 400 Bad Request - missing auto enable value in body",
				})
			}

			if !utils.ValidateAutoEnabledClaGroupID(*params.Body.AutoEnabled, params.Body.AutoEnabledClaGroupID) {
				return github_organizations.NewUpdateProjectGithubOrganizationConfigBadRequest().WithPayload(&models.ErrorResponse{
					Code:    "400",
					Message: "EasyCLA - 400 Bad Request - AutoEnabledClaGroupID can't be empty when AutoEnabled",
				})
			}

			err := service.UpdateGitHubOrganization(ctx, params.ProjectSFID, params.OrgName, *params.Body.AutoEnabled, params.Body.AutoEnabledClaGroupID, params.Body.BranchProtectionEnabled)
			if err != nil {
				if errors.Is(err, projects_cla_groups.ErrCLAGroupDoesNotExist) {
					return github_organizations.NewUpdateProjectGithubOrganizationConfigNotFound().WithPayload(errorResponse(err))
				}
				return github_organizations.NewUpdateProjectGithubOrganizationConfigBadRequest().WithPayload(errorResponse(err))
			}

			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
				UserID:      claUser.UserID,
				EventType:   events.GitHubOrganizationUpdated,
				ProjectSFID: params.ProjectSFID,
				LfUsername:  claUser.LFUsername,
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

func errorResponse(err error) *models.ErrorResponse {
	code := ""
	if e, ok := err.(codedResponse); ok {
		code = e.Code()
	}

	e := models.ErrorResponse{
		Code:    code,
		Message: err.Error(),
	}

	return &e
}
