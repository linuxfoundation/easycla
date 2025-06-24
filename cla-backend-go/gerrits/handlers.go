// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gerrits

import (
	"context"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/gerrits"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
)

// ProjectService contains Project methods
type ProjectService interface {
	GetCLAGroupByID(ctx context.Context, claGroupID string) (*models.ClaGroup, error)
}

// Configure the gerrit api
func Configure(api *operations.ClaAPI, service Service, projectService ProjectService, eventService events.Service) {
	api.GerritsDeleteGerritHandler = gerrits.DeleteGerritHandlerFunc(
		func(params gerrits.DeleteGerritParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			claGroupModel, err := projectService.GetCLAGroupByID(ctx, params.ProjectID)
			if err != nil {
				return gerrits.NewDeleteGerritBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
			}
			// verify user have access to the project
			if !claUser.IsAuthorizedForProject(claGroupModel.ProjectExternalID) {
				return gerrits.NewDeleteGerritUnauthorized().WithXRequestID(reqID)
			}
			gerrit, err := service.GetGerrit(ctx, params.GerritID)
			if err != nil {
				return gerrits.NewDeleteGerritBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
			}
			// verify gerrit project is same as the request
			if gerrit.ProjectID != params.ProjectID {
				return gerrits.NewDeleteGerritBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code:    "400",
					Message: "provided project id does not match with gerrit project id",
				})
			}
			// delete the gerrit
			err = service.DeleteGerrit(ctx, params.GerritID)
			if err != nil {
				return gerrits.NewDeleteGerritBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
			}
			// record the event
			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType:     events.GerritRepositoryDeleted,
				ClaGroupModel: claGroupModel,
				UserID:        claUser.UserID,
				EventData: &events.GerritDeletedEventData{
					GerritRepositoryName: gerrit.GerritName,
				},
			})
			return gerrits.NewDeleteGerritNoContent().WithXRequestID(reqID)
		})

	api.GerritsAddGerritHandler = gerrits.AddGerritHandlerFunc(
		func(params gerrits.AddGerritParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			claGroupModel, err := projectService.GetCLAGroupByID(ctx, params.ProjectID)
			if err != nil {
				return gerrits.NewAddGerritBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
			}
			// verify user have access to the project
			if !claUser.IsAuthorizedForProject(claGroupModel.ProjectExternalID) {
				return gerrits.NewAddGerritUnauthorized().WithXRequestID(reqID)
			}

			if len(strings.TrimSpace(*params.AddGerritInput.GerritName)) == 0 {
				return gerrits.NewGetGerritReposBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code:    "400",
					Message: "invalid gerritName parameter - expecting gerrit hostname",
				})
			}
			params.AddGerritInput.Version = "v1"
			// add the gerrit
			result, err := service.AddGerrit(ctx, params.ProjectID, claGroupModel.ProjectExternalID, params.AddGerritInput, claGroupModel)
			if err != nil {
				if err.Error() == "gerrit_name already present in the system" {
					return gerrits.NewAddGerritConflict().WithXRequestID(reqID).WithPayload(errorResponse(err))
				}
				return gerrits.NewAddGerritBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
			}
			// record the event
			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType:     events.GerritRepositoryAdded,
				ClaGroupModel: claGroupModel,
				UserID:        claUser.UserID,
				EventData: &events.GerritAddedEventData{
					GerritRepositoryName: utils.StringValue(params.AddGerritInput.GerritName),
				},
			})
			return gerrits.NewAddGerritOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.GerritsGetGerritReposHandler = gerrits.GetGerritReposHandlerFunc(
		func(params gerrits.GetGerritReposParams, authUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

			// No specific permissions required

			// Validate input
			if params.GerritHost == nil {
				return gerrits.NewGetGerritReposBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code:    "400",
					Message: "missing gerritHost query parameter",
				})
			}

			result, err := service.GetGerritRepos(ctx, params.GerritHost.String())
			if err != nil {
				return gerrits.NewGetGerritReposBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
			}

			return gerrits.NewGetGerritReposOK().WithXRequestID(reqID).WithPayload(result)
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
