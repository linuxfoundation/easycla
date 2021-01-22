// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gerrits

import (
	"context"
	"fmt"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/gerrits"
	v1Gerrits "github.com/communitybridge/easycla/cla-backend-go/gerrits"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
	"github.com/jinzhu/copier"
)

type ProjectService interface { //nolint
	GetCLAGroupByID(ctx context.Context, claGroupID string) (*v1Models.ClaGroup, error)
}

// Configure the Gerrit api
func Configure(api *operations.EasyclaAPI, v1Service v1Gerrits.Service, projectService ProjectService, eventService events.Service, projectsClaGroupsRepo projects_cla_groups.Repository) {
	api.GerritsDeleteGerritHandler = gerrits.DeleteGerritHandlerFunc(
		func(params gerrits.DeleteGerritParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)

			gerrit, err := v1Service.GetGerrit(ctx, params.GerritID)
			if err != nil {
				if err == v1Gerrits.ErrGerritNotFound {
					return gerrits.NewDeleteGerritNotFound().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
				}
				return gerrits.NewDeleteGerritInternalServerError().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			if gerrit.ProjectSFID != params.ProjectSFID || gerrit.ProjectID != params.ClaGroupID {
				return gerrits.NewDeleteGerritBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code:       "400",
					Message:    "EasyCLA - 403 Bad Request - projectSFID or claGroupID does not match with provided gerrit record",
					XRequestID: reqID,
				})
			}
			// verify user have access to the project
			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				return gerrits.NewDeleteGerritForbidden().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to DeleteGerrit with Project scope of %s",
						authUser.UserName, gerrit.ProjectSFID),
					XRequestID: reqID,
				})
			}

			// delete the gerrit
			err = v1Service.DeleteGerrit(ctx, params.GerritID)
			if err != nil {
				return gerrits.NewDeleteGerritBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}

			// record the event
			eventService.LogEvent(&events.LogEventArgs{
				EventType:  events.GerritRepositoryDeleted,
				ProjectID:  gerrit.ProjectID,
				LfUsername: authUser.UserName,
				EventData: &events.GerritDeletedEventData{
					GerritRepositoryName: gerrit.GerritName,
				},
			})

			return gerrits.NewDeleteGerritNoContent().WithXRequestID(reqID)
		})

	api.GerritsAddGerritHandler = gerrits.AddGerritHandlerFunc(
		func(params gerrits.AddGerritParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)

			// verify user have access to the project
			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				return gerrits.NewAddGerritForbidden().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to AddGerrit with Project scope of %s",
						authUser.UserName, params.ProjectSFID),
					XRequestID: reqID,
				})
			}
			ok, err := projectsClaGroupsRepo.IsAssociated(params.ProjectSFID, params.ClaGroupID)
			if err != nil {
				return gerrits.NewAddGerritBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			if !ok {
				return gerrits.NewAddGerritBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code:       "400",
					Message:    "provided cla-group and project are not associated with each other",
					XRequestID: reqID,
				})
			}

			projectModel, err := projectService.GetCLAGroupByID(ctx, params.ClaGroupID)
			if err != nil {
				return gerrits.NewDeleteGerritBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}

			// add the gerrit
			addGerritInput := &v1Models.AddGerritInput{
				GerritName:  params.AddGerritInput.GerritName,
				GerritURL:   params.AddGerritInput.GerritURL,
				GroupIDCcla: params.AddGerritInput.GroupIDCcla,
				GroupIDIcla: params.AddGerritInput.GroupIDIcla,
				Version:     "v2",
			}
			result, err := v1Service.AddGerrit(ctx, params.ClaGroupID, params.ProjectSFID, addGerritInput, projectModel)
			if err != nil {
				if err.Error() == "gerrit_name already present in the system" {
					return gerrits.NewAddGerritConflict().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
				}
				return gerrits.NewAddGerritBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}

			// record the event
			eventService.LogEvent(&events.LogEventArgs{
				EventType:  events.GerritRepositoryAdded,
				ProjectID:  params.ClaGroupID,
				LfUsername: authUser.UserName,
				EventData: &events.GerritAddedEventData{
					GerritRepositoryName: utils.StringValue(params.AddGerritInput.GerritName),
				},
			})

			var response models.Gerrit
			err = copier.Copy(&response, result)
			if err != nil {
				return gerrits.NewAddGerritInternalServerError().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return gerrits.NewAddGerritOK().WithXRequestID(reqID).WithPayload(&response)
		})

	api.GerritsListGerritsHandler = gerrits.ListGerritsHandlerFunc(
		func(params gerrits.ListGerritsParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)

			// verify user have access to the project
			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				return gerrits.NewListGerritsForbidden().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to ListGerrits with Project scope of %s",
						authUser.UserName, params.ProjectSFID),
					XRequestID: reqID,
				})
			}

			ok, err := projectsClaGroupsRepo.IsAssociated(params.ProjectSFID, params.ClaGroupID)
			if err != nil {
				return gerrits.NewListGerritsBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			if !ok {
				return gerrits.NewListGerritsBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code:       "400",
					Message:    "provided cla-group and project are not associated with each other",
					XRequestID: reqID,
				})
			}

			result, err := v1Service.GetClaGroupGerrits(ctx, params.ClaGroupID, &params.ProjectSFID)
			if err != nil {
				return gerrits.NewListGerritsBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}

			var response models.GerritList
			err = copier.Copy(&response, result)
			if err != nil {
				return gerrits.NewListGerritsInternalServerError().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return gerrits.NewListGerritsOK().WithXRequestID(reqID).WithPayload(&response)
		})

	api.GerritsGetGerritReposHandler = gerrits.GetGerritReposHandlerFunc(
		func(params gerrits.GetGerritReposParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)

			// No specific permissions required

			// Validate input
			if params.GerritHost == nil {
				return gerrits.NewGetGerritReposBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code:       "400",
					Message:    "missing gerritHost query parameter - expecting gerrit hostname",
					XRequestID: reqID,
				})
			}

			if len(strings.TrimSpace(params.GerritHost.String())) == 0 {
				return gerrits.NewGetGerritReposBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code:       "400",
					Message:    "invalid gerritHost query parameter - expecting gerrit hostname",
					XRequestID: reqID,
				})
			}

			result, err := v1Service.GetGerritRepos(ctx, params.GerritHost.String())
			if err != nil {
				return gerrits.NewGetGerritReposBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}

			var response models.GerritRepoList
			err = copier.Copy(&response, result)
			if err != nil {
				return gerrits.NewAddGerritInternalServerError().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}

			return gerrits.NewGetGerritReposOK().WithXRequestID(reqID).WithPayload(&response)
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
