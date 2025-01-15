// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gerrits

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/gerrits"
	v1Gerrits "github.com/communitybridge/easycla/cla-backend-go/gerrits"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
	"github.com/jinzhu/copier"
)

const decodeErrorMsg = "unable to decode response as a v2 model"

type ProjectService interface { //nolint
	GetCLAGroupByID(ctx context.Context, claGroupID string) (*v1Models.ClaGroup, error)
}

// Configure the Gerrit api
func Configure(api *operations.EasyclaAPI, v1Service v1Gerrits.Service, projectService ProjectService, eventService events.Service, projectsClaGroupsRepo projects_cla_groups.Repository) { // nolint
	api.GerritsDeleteGerritHandler = gerrits.DeleteGerritHandlerFunc(
		func(params gerrits.DeleteGerritParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.gerrits.handlers.GerritsDeleteGerritHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"projectSFID":    params.ProjectSFID,
				"claGroupID":     params.ClaGroupID,
				"gerritID":       params.GerritID,
				"authUserName":   authUser.UserName,
				"authUserEmail":  authUser.Email,
			}

			log.WithFields(f).Debugf("querying for gerrits using gerrit ID: %s", params.GerritID)
			gerrit, err := v1Service.GetGerrit(ctx, params.GerritID)
			if err != nil {
				msg := fmt.Sprintf("unable to locate gerrit by ID: %s", params.GerritID)
				log.WithFields(f).Warn(msg)
				if err == v1Gerrits.ErrGerritNotFound {
					return gerrits.NewDeleteGerritNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFoundWithError(reqID, msg, err))
				}
				return gerrits.NewDeleteGerritInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			if gerrit.ProjectSFID != params.ProjectSFID || gerrit.ProjectID != params.ClaGroupID {
				msg := fmt.Sprintf("projectSFID %s or claGroupID %s does not match with provided gerrit record", params.ProjectSFID, params.ClaGroupID)
				log.WithFields(f).Warn(msg)
				return gerrits.NewDeleteGerritBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
			}

			// verify user have access to the project
			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to DeleteGerrit with Project scope of %s",
					authUser.UserName, gerrit.ProjectSFID)
				log.WithFields(f).Warn(msg)
				return gerrits.NewDeleteGerritForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			// delete the gerrit
			err = v1Service.DeleteGerrit(ctx, params.GerritID)
			if err != nil {
				msg := "unable to delete gerrit instance"
				log.WithFields(f).WithError(err).Warn(msg)
				return gerrits.NewDeleteGerritForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			// record the event
			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
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
			ok, err := projectsClaGroupsRepo.IsAssociated(ctx, params.ProjectSFID, params.ClaGroupID)
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
				GerritName: params.AddGerritInput.GerritName,
				GerritURL:  params.AddGerritInput.GerritURL,
				Version:    "v2",
			}
			result, err := v1Service.AddGerrit(ctx, params.ClaGroupID, params.ProjectSFID, addGerritInput, projectModel)
			if err != nil {
				if err.Error() == "gerrit_name already present in the system" {
					return gerrits.NewAddGerritConflict().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
				}
				return gerrits.NewAddGerritBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}

			// record the event
			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
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
			f := logrus.Fields{
				"functionName":   "v2.gerrits.handlers.GerritsListGerritsHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"projectSFID":    params.ProjectSFID,
				"claGroupID":     params.ClaGroupID,
				"authUserName":   authUser.UserName,
				"authUserEmail":  authUser.Email,
			}

			// verify user have access to the project
			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to list gerrits with Project scope of %s", authUser.UserName, params.ProjectSFID)
				log.WithFields(f).Warn(msg)
				return gerrits.NewListGerritsForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			log.WithFields(f).Debug("checking if project CLA Group mapping...")
			ok, err := projectsClaGroupsRepo.IsAssociated(ctx, params.ProjectSFID, params.ClaGroupID)
			if err != nil {
				msg := fmt.Sprintf("unable to determine project CLA group association for project: %s and CLA Group: %s", params.ProjectSFID, params.ClaGroupID)
				log.WithFields(f).WithError(err).Warn(msg)
				return gerrits.NewListGerritsBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			if !ok {
				msg := fmt.Sprintf("provided CLA Group %s and project %s are not associated with each other", params.ProjectSFID, params.ClaGroupID)
				log.WithFields(f).WithError(err).Warn(msg)
				return gerrits.NewListGerritsBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
			}

			log.WithFields(f).Debug("querying for gerrits...")
			result, err := v1Service.GetClaGroupGerrits(ctx, params.ClaGroupID)
			if err != nil {
				msg := fmt.Sprintf("problem fetching gerrit repositories using CLA Group: %s with project SFID: %s", params.ClaGroupID, params.ProjectSFID)
				log.WithFields(f).Warn(msg)
				return gerrits.NewListGerritsBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			log.WithFields(f).Debugf("discovered %d gerrits", len(result.List))

			var response models.GerritList
			err = copier.Copy(&response, result)
			if err != nil {
				log.WithFields(f).WithError(err).Warn(decodeErrorMsg)
				return gerrits.NewListGerritsInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, decodeErrorMsg, err))
			}

			return gerrits.NewListGerritsOK().WithXRequestID(reqID).WithPayload(&response)
		})

	api.GerritsGetGerritReposHandler = gerrits.GetGerritReposHandlerFunc(
		func(params gerrits.GetGerritReposParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.gerrits.handlers.GerritsGetGerritReposHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   authUser.UserName,
				"authUserEmail":  authUser.Email,
				"gerritHost":     params.GerritHost.String(),
			}

			// No specific permissions required

			// Validate input
			if params.GerritHost == nil {
				msg := "missing gerrit host query parameter - expecting gerrit hostname"
				log.WithFields(f).Warn(msg)
				return gerrits.NewGetGerritReposBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
			}

			if len(strings.TrimSpace(params.GerritHost.String())) == 0 {
				msg := "invalid gerritHost query parameter - expecting gerrit hostname"
				log.WithFields(f).Warn(msg)
				return gerrits.NewGetGerritReposBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
			}

			log.WithFields(f).Debugf("querying for gerrits using hostname: %s...", params.GerritHost.String())
			result, err := v1Service.GetGerritRepos(ctx, params.GerritHost.String())
			if err != nil {
				msg := fmt.Sprintf("problem fetching gerrit repositories using gerrit host: %s", params.GerritHost.String())
				log.WithFields(f).Warn(msg)
				return gerrits.NewGetGerritReposBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			var response models.GerritRepoList
			err = copier.Copy(&response, result)
			if err != nil {
				log.WithFields(f).WithError(err).Warn(decodeErrorMsg)
				return gerrits.NewAddGerritInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, decodeErrorMsg, err))
			}

			return gerrits.NewGetGerritReposOK().WithXRequestID(reqID).WithPayload(&response)
		})

	// api.GerritsGetGerritICLAUserHandler = gerrits.GetGerritICLAUserHandlerFunc(func(params gerrits.GetGerritICLAUserParams, authUser *auth.User) middleware.Responder {
	// 	reqID := utils.GetRequestID(params.XREQUESTID)
	// 	ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
	// 	utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
	// 	f := logrus.Fields{
	// 		"functionName":   "v2.gerrits.handlers.GerritsGetGerritICLAUserHandler",
	// 		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	// 		"authUserName":   authUser.UserName,
	// 		"authUserEmail":  authUser.Email,
	// 		"claGroupID":     params.ClaGroupID,
	// 		"projectSFID":    params.ProjectSFID,
	// 	}

	// 	// verify user have access to the project
	// 	if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
	// 		msg := fmt.Sprintf("user %s does not have access to get gerrit users with Project scope of %s", authUser.UserName, params.ProjectSFID)
	// 		log.WithFields(f).Warn(msg)
	// 		return gerrits.NewGetGerritICLAUserForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
	// 	}

	// 	log.WithFields(f).Debugf("getting user list to gerrit...")
	// 	responseModel, err := v1Service.GetUsersOfGroup(ctx, authUser, params.ClaGroupID, utils.ClaTypeICLA)
	// 	if err != nil {
	// 		msg := fmt.Sprintf("problem getting user list of CLA Group %s", params.ClaGroupID)
	// 		log.WithFields(f).WithError(err).Warn(msg)
	// 		return gerrits.NewGetGerritICLAUserInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
	// 	}

	// 	return gerrits.NewGetGerritICLAUserOK().WithXRequestID(reqID).WithPayload(responseModel)
	// })

	// api.GerritsGetGerritECLAUserHandler = gerrits.GetGerritECLAUserHandlerFunc(func(params gerrits.GetGerritECLAUserParams, authUser *auth.User) middleware.Responder {
	// 	reqID := utils.GetRequestID(params.XREQUESTID)
	// 	ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
	// 	utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
	// 	f := logrus.Fields{
	// 		"functionName":   "v2.gerrits.handlers.GerritsGetGerritECLAUserHandler",
	// 		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	// 		"authUserName":   authUser.UserName,
	// 		"authUserEmail":  authUser.Email,
	// 		"claGroupID":     params.ClaGroupID,
	// 		"projectSFID":    params.ProjectSFID,
	// 	}

	// 	// verify user have access to the project
	// 	if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
	// 		msg := fmt.Sprintf("user %s does not have access to get gerrit users with Project scope of %s", authUser.UserName, params.ProjectSFID)
	// 		log.WithFields(f).Warn(msg)
	// 		return gerrits.NewGetGerritECLAUserForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
	// 	}

	// 	log.WithFields(f).Debugf("getting user list to gerrit...")
	// 	responseModel, err := v1Service.GetUsersOfGroup(ctx, authUser, params.ClaGroupID, utils.ClaTypeECLA)
	// 	if err != nil {
	// 		msg := fmt.Sprintf("problem getting user list of CLA Group %s", params.ClaGroupID)
	// 		log.WithFields(f).WithError(err).Warn(msg)
	// 		return gerrits.NewGetGerritECLAUserInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
	// 	}

	// 	return gerrits.NewGetGerritECLAUserOK().WithXRequestID(reqID).WithPayload(responseModel)
	// })

	// api.GerritsAddGerritICLAUserHandler = gerrits.AddGerritICLAUserHandlerFunc(func(params gerrits.AddGerritICLAUserParams, authUser *auth.User) middleware.Responder {
	// 	reqID := utils.GetRequestID(params.XREQUESTID)
	// 	ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
	// 	utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
	// 	f := logrus.Fields{
	// 		"functionName":   "v2.gerrits.handlers.GerritsAddGerritICLAUserHandler",
	// 		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	// 		"authUserName":   authUser.UserName,
	// 		"authUserEmail":  authUser.Email,
	// 		"claGroupID":     params.ClaGroupID,
	// 		"projectSFID":    params.ProjectSFID,
	// 		"gerritUsers":    strings.Join(params.AddGerritUserInput, ","),
	// 	}

	// 	// verify user have access to the project
	// 	if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
	// 		msg := fmt.Sprintf("user %s does not have access to add gerrit users with Project scope of %s", authUser.UserName, params.ProjectSFID)
	// 		log.WithFields(f).Warn(msg)
	// 		return gerrits.NewAddGerritICLAUserForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
	// 	}

	// 	log.WithFields(f).Debugf("adding user list to gerrit...")
	// 	err := v1Service.AddUsersToGroup(ctx, authUser, params.ClaGroupID, params.AddGerritUserInput, utils.ClaTypeICLA)
	// 	if err != nil {
	// 		msg := fmt.Sprintf("problem adding user list %s to CLA Group %s", strings.Join(params.AddGerritUserInput, ","), params.ClaGroupID)
	// 		log.WithFields(f).WithError(err).Warn(msg)
	// 		return gerrits.NewAddGerritICLAUserInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
	// 	}

	// 	return gerrits.NewAddGerritICLAUserOK().WithXRequestID(reqID)
	// })

	// api.GerritsRemoveGerritICLAUserHandler = gerrits.RemoveGerritICLAUserHandlerFunc(func(params gerrits.RemoveGerritICLAUserParams, authUser *auth.User) middleware.Responder {
	// 	reqID := utils.GetRequestID(params.XREQUESTID)
	// 	ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
	// 	utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
	// 	f := logrus.Fields{
	// 		"functionName":   "v2.gerrits.handlers.GerritsRemoveGerritICLAUserHandler",
	// 		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	// 		"authUserName":   authUser.UserName,
	// 		"authUserEmail":  authUser.Email,
	// 		"claGroupID":     params.ClaGroupID,
	// 		"projectSFID":    params.ProjectSFID,
	// 		"gerritUsers":    strings.Join(params.RemoveGerritUserInput, ","),
	// 	}

	// 	// verify user have access to the project
	// 	if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
	// 		msg := fmt.Sprintf("user %s does not have access to remove gerrit users with Project scope of %s", authUser.UserName, params.ProjectSFID)
	// 		log.WithFields(f).Warn(msg)
	// 		return gerrits.NewRemoveGerritICLAUserForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
	// 	}

	// 	log.WithFields(f).Debugf("removing user list from gerrit...")
	// 	err := v1Service.RemoveUsersFromGroup(ctx, authUser, params.ClaGroupID, params.RemoveGerritUserInput, utils.ClaTypeICLA)
	// 	if err != nil {
	// 		msg := fmt.Sprintf("problem removing user list %s to CLA Group %s", strings.Join(params.RemoveGerritUserInput, ","), params.ClaGroupID)
	// 		log.WithFields(f).WithError(err).Warn(msg)
	// 		return gerrits.NewRemoveGerritICLAUserInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
	// 	}

	// 	return gerrits.NewRemoveGerritICLAUserOK().WithXRequestID(reqID)
	// })

	// api.GerritsAddGerritECLAUserHandler = gerrits.AddGerritECLAUserHandlerFunc(func(params gerrits.AddGerritECLAUserParams, authUser *auth.User) middleware.Responder {
	// 	reqID := utils.GetRequestID(params.XREQUESTID)
	// 	ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
	// 	utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
	// 	f := logrus.Fields{
	// 		"functionName":   "v2.gerrits.handlers.GerritsAddGerritECLAUserHandler",
	// 		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	// 		"authUserName":   authUser.UserName,
	// 		"authUserEmail":  authUser.Email,
	// 		"claGroupID":     params.ClaGroupID,
	// 		"projectSFID":    params.ProjectSFID,
	// 		"gerritUsers":    strings.Join(params.AddGerritUserInput, ","),
	// 	}

	// 	// verify user have access to the project
	// 	if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
	// 		msg := fmt.Sprintf("user %s does not have access to add gerrit users with Project scope of %s", authUser.UserName, params.ProjectSFID)
	// 		log.WithFields(f).Warn(msg)
	// 		return gerrits.NewAddGerritECLAUserForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
	// 	}

	// 	log.WithFields(f).Debugf("adding user list to gerrit...")
	// 	err := v1Service.AddUsersToGroup(ctx, authUser, params.ClaGroupID, params.AddGerritUserInput, utils.ClaTypeECLA)
	// 	if err != nil {
	// 		msg := fmt.Sprintf("problem adding user list %s to CLA Group %s", strings.Join(params.AddGerritUserInput, ","), params.ClaGroupID)
	// 		log.WithFields(f).WithError(err).Warn(msg)
	// 		return gerrits.NewAddGerritECLAUserInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
	// 	}

	// 	return gerrits.NewAddGerritECLAUserOK().WithXRequestID(reqID)
	// })

	// api.GerritsRemoveGerritECLAUserHandler = gerrits.RemoveGerritECLAUserHandlerFunc(func(params gerrits.RemoveGerritECLAUserParams, authUser *auth.User) middleware.Responder {
	// 	reqID := utils.GetRequestID(params.XREQUESTID)
	// 	ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
	// 	utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
	// 	f := logrus.Fields{
	// 		"functionName":   "v2.gerrits.handlers.GerritsRemoveGerritECLAUserHandler",
	// 		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	// 		"authUserName":   authUser.UserName,
	// 		"authUserEmail":  authUser.Email,
	// 		"claGroupID":     params.ClaGroupID,
	// 		"projectSFID":    params.ProjectSFID,
	// 		"gerritUsers":    strings.Join(params.RemoveGerritUserInput, ","),
	// 	}

	// 	// verify user have access to the project
	// 	if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
	// 		msg := fmt.Sprintf("user %s does not have access to remove gerrit users with Project scope of %s", authUser.UserName, params.ProjectSFID)
	// 		log.WithFields(f).Warn(msg)
	// 		return gerrits.NewRemoveGerritECLAUserForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
	// 	}

	// 	log.WithFields(f).Debugf("removing user list from gerrit...")
	// 	err := v1Service.RemoveUsersFromGroup(ctx, authUser, params.ClaGroupID, params.RemoveGerritUserInput, utils.ClaTypeECLA)
	// 	if err != nil {
	// 		msg := fmt.Sprintf("problem removing user list %s to CLA Group %s", strings.Join(params.RemoveGerritUserInput, ","), params.ClaGroupID)
	// 		log.WithFields(f).WithError(err).Warn(msg)
	// 		return gerrits.NewRemoveGerritECLAUserInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
	// 	}

	// 	return gerrits.NewRemoveGerritECLAUserOK().WithXRequestID(reqID)
	// })

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
