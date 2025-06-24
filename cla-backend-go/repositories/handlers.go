// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"context"
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/github_repositories"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
)

// Configure establishes the middleware handlers for the repository service
func Configure(api *operations.ClaAPI, service Service, eventService events.Service) {
	api.GithubRepositoriesGetProjectGithubRepositoriesHandler = github_repositories.GetProjectGithubRepositoriesHandlerFunc(
		func(params github_repositories.GetProjectGithubRepositoriesParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			if !claUser.IsAuthorizedForProject(params.ProjectSFID) {
				return github_repositories.NewGetProjectGithubRepositoriesForbidden().WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to Add GitHub CombinedRepository with Project scope of %s",
						claUser.LFUsername, params.ProjectSFID),
				})
			}
			enabled := true
			result, err := service.ListProjectRepositories(ctx, params.ProjectSFID, &enabled)
			if err != nil {
				return github_repositories.NewGetProjectGithubRepositoriesBadRequest().WithPayload(errorResponse(err))
			}
			return github_repositories.NewGetProjectGithubRepositoriesOK().WithPayload(result)
		})

	api.GithubRepositoriesAddProjectGithubRepositoryHandler = github_repositories.AddProjectGithubRepositoryHandlerFunc(
		func(params github_repositories.AddProjectGithubRepositoryParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			if !claUser.IsAuthorizedForProject(params.ProjectSFID) {
				return github_repositories.NewAddProjectGithubRepositoryForbidden().WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to Add GitHub CombinedRepository with Project scope of %s",
						claUser.LFUsername, params.ProjectSFID),
				})
			}
			result, err := service.AddGithubRepository(ctx, params.ProjectSFID, params.GithubRepositoryInput)
			if err != nil {
				return github_repositories.NewAddProjectGithubRepositoryBadRequest().WithPayload(errorResponse(err))
			}
			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType:   events.RepositoryAdded,
				CLAGroupID:  utils.StringValue(params.GithubRepositoryInput.RepositoryProjectID),
				ProjectSFID: params.ProjectSFID,
				UserID:      claUser.UserID,
				LfUsername:  claUser.LFUsername,
				UserModel: &models.User{
					Username: claUser.LFUsername,
				},
				EventData: &events.RepositoryAddedEventData{
					RepositoryName: utils.StringValue(params.GithubRepositoryInput.RepositoryName),
				},
			})
			return github_repositories.NewAddProjectGithubRepositoryOK().WithPayload(result)
		})

	api.GithubRepositoriesDeleteProjectGithubRepositoryHandler = github_repositories.DeleteProjectGithubRepositoryHandlerFunc(
		func(params github_repositories.DeleteProjectGithubRepositoryParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			if !claUser.IsAuthorizedForProject(params.ProjectSFID) {
				return github_repositories.NewDeleteProjectGithubRepositoryForbidden().WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to Delete GitHub CombinedRepository with Project scope of %s",
						claUser.LFUsername, params.ProjectSFID),
				})
			}
			ghRepo, err := service.GetRepository(ctx, params.RepositoryID)
			if err != nil {
				if _, ok := err.(*utils.GitHubRepositoryNotFound); ok {
					return github_repositories.NewDeleteProjectGithubRepositoryNotFound()
				}
				return github_repositories.NewDeleteProjectGithubRepositoryBadRequest().WithPayload(errorResponse(err))
			}
			err = service.DisableRepository(ctx, params.RepositoryID)
			if err != nil {
				return github_repositories.NewDeleteProjectGithubRepositoryBadRequest().WithPayload(errorResponse(err))
			}
			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType:   events.RepositoryDisabled,
				ProjectSFID: params.ProjectSFID,
				UserID:      claUser.UserID,
				LfUsername:  claUser.LFUsername,
				EventData: &events.RepositoryDisabledEventData{
					RepositoryName: ghRepo.RepositoryName,
				},
			})
			return github_repositories.NewDeleteProjectGithubRepositoryNoContent()
		})
}

// codedResponse interface
type codedResponse interface {
	Code() string
}

// errorResponse is a helper to wrap the specified error into an error response model
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
