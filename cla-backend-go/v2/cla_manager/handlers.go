// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/LF-Engineering/lfx-kit/auth"

	v1Company "github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/cla_manager"
	v1User "github.com/linuxfoundation/easycla/cla-backend-go/user"

	"github.com/go-openapi/runtime/middleware"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

const (
	//BadRequest error Response code
	BadRequest = "400"
	//Conflict error Response code
	Conflict = "409"
	// NotFound error Response code
	NotFound = "404"
	//Accepted Response code
	Accepted = "202"
)

// Configure is the API handler routine for CLA Manager routes
func Configure(api *operations.EasyclaAPI, service Service, v1CompanyService v1Company.IService, LfxPortalURL, CorporateConsoleV2URL string, projectClaGroupRepo projects_cla_groups.Repository, easyCLAUserRepo v1User.RepositoryService) { // nolint
	api.ClaManagerCreateCLAManagerHandler = cla_manager.CreateCLAManagerHandlerFunc(func(params cla_manager.CreateCLAManagerParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)

		f := logrus.Fields{
			"functionName":   "v2.cla_manager.handlers.ClaManagerCreateCLAManagerHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"CompanyID":      params.CompanyID,
			"ProjectSFID":    params.ProjectSFID,
			"authUser":       *params.XUSERNAME,
		}

		// Lookup the company by internal ID
		log.WithFields(f).Debugf("looking up company by internal ID...")
		v1CompanyModel, err := v1CompanyService.GetCompany(ctx, params.CompanyID)
		if err != nil || v1CompanyModel == nil {
			msg := fmt.Sprintf("unable to lookup company by ID: %s", params.CompanyID)
			log.WithFields(f).WithError(err).Warn(msg)
			return cla_manager.NewCreateCLAManagerBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debug("checking permissions...")
		if !utils.IsUserAuthorizedForProjectOrganizationTree(ctx, authUser, params.ProjectSFID, v1CompanyModel.CompanyExternalID, utils.DISALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("user %s does not have access to DeleteCLAManager with Project|Organization scope of %s | %s", authUser.UserName, params.ProjectSFID, params.CompanyID)
			log.WithFields(f).Warn(msg)
			return cla_manager.NewCreateCLAManagerForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		log.WithFields(f).Debug("looking up CLA Group for projectSFID...")
		cginfo, err := projectClaGroupRepo.GetClaGroupIDForProject(ctx, params.ProjectSFID)
		if err != nil {
			if err == projects_cla_groups.ErrProjectNotAssociatedWithClaGroup {
				msg := fmt.Sprintf("no CLA Group associated with this project: %s", params.ProjectSFID)
				log.WithFields(f).WithError(err).Warn(msg)
				return cla_manager.NewCreateCLAManagerBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}
			return cla_manager.NewCreateCLAManagerInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, err.Error(), err))
		}

		compCLAManager, errorResponse := service.CreateCLAManager(ctx, authUser, cginfo.ClaGroupID, params, authUser.UserName)
		if errorResponse != nil {
			if errorResponse.Code == BadRequest {
				return cla_manager.NewCreateCLAManagerBadRequest().WithXRequestID(reqID).WithPayload(errorResponse)
			} else if errorResponse.Code == Conflict {
				return cla_manager.NewCreateCLAManagerConflict().WithXRequestID(reqID).WithPayload(errorResponse)
			}
		}

		return cla_manager.NewCreateCLAManagerOK().WithXRequestID(reqID).WithPayload(compCLAManager)
	})

	api.ClaManagerDeleteCLAManagerHandler = cla_manager.DeleteCLAManagerHandlerFunc(func(params cla_manager.DeleteCLAManagerParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.cla_manager.handlers.ClaManagerDeleteCLAManagerHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"CompanyID":      params.CompanyID,
			"ProjectSFID":    params.ProjectSFID,
			"userLFID":       params.UserLFID,
			"authUser":       *params.XUSERNAME,
		}

		// Lookup the company by internal ID
		log.WithFields(f).Debugf("looking up company by internal ID...")
		v1CompanyModel, err := v1CompanyService.GetCompany(ctx, params.CompanyID)
		if err != nil || v1CompanyModel == nil {
			msg := fmt.Sprintf("unable to lookup company by ID: %s", params.CompanyID)
			log.WithFields(f).WithError(err).Warn(msg)
			return cla_manager.NewDeleteCLAManagerBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debug("checking permissions...")
		if !utils.IsUserAuthorizedForProjectOrganizationTree(ctx, authUser, params.ProjectSFID, v1CompanyModel.CompanyExternalID, utils.DISALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("user %s does not have access to DeleteCLAManager with Project|Organization scope of %s | %s", authUser.UserName, params.ProjectSFID, params.CompanyID)
			log.WithFields(f).Warn(msg)
			return cla_manager.NewDeleteCLAManagerBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		cginfo, err := projectClaGroupRepo.GetClaGroupIDForProject(ctx, params.ProjectSFID)
		if err != nil {
			msg := fmt.Sprintf("no CLA Group associated with this project: %s", params.ProjectSFID)
			log.WithFields(f).WithError(err).Warn(msg)
			return cla_manager.NewDeleteCLAManagerBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		errResponse := service.DeleteCLAManager(ctx, authUser, cginfo.ClaGroupID, params)
		if errResponse != nil {
			return cla_manager.NewDeleteCLAManagerBadRequest().WithXRequestID(reqID).WithPayload(errResponse)
		}

		return cla_manager.NewDeleteCLAManagerNoContent().WithXRequestID(reqID)
	})

	api.ClaManagerCreateCLAManagerDesigneeHandler = cla_manager.CreateCLAManagerDesigneeHandlerFunc(func(params cla_manager.CreateCLAManagerDesigneeParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.cla_manager.handlers.ClaManagerCreateCLAManagerDesigneeHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"CompanyID":      params.CompanyID,
			"ProjectSFID":    params.ProjectSFID,
			"authUser":       *params.XUSERNAME,
		}

		// Lookup the company by internal ID
		log.WithFields(f).Debugf("looking up company by internal ID...")
		v1CompanyModel, err := v1CompanyService.GetCompany(ctx, params.CompanyID)
		if err != nil || v1CompanyModel == nil {
			msg := fmt.Sprintf("unable to lookup company by ID: %s", params.CompanyID)
			log.WithFields(f).WithError(err).Warn(msg)
			return cla_manager.NewCreateCLAManagerDesigneeBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		// Note: anyone create assign a CLA manager designee...no permissions checks
		log.WithFields(f).Debugf("processing create CLA Manager Desginee request")
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		claManagerDesignee, err := service.CreateCLAManagerDesignee(ctx, params.CompanyID, params.ProjectSFID, params.Body.UserEmail.String())
		if err != nil {
			if err == ErrCLAManagerDesigneeConflict {
				msg := fmt.Sprintf("Conflict assigning cla manager role for Project SFID: %s ", params.ProjectSFID)
				return cla_manager.NewCreateCLAManagerDesigneeByGroupConflict().WithXRequestID(reqID).WithPayload(utils.ErrorResponseConflictWithError(reqID, msg, err))
			}
			msg := fmt.Sprintf("user :%s, error: %+v ", authUser.Email, err)
			return cla_manager.NewCreateCLAManagerBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.Debugf("CLA Manager designee created : %+v", claManagerDesignee)
		return cla_manager.NewCreateCLAManagerDesigneeOK().WithXRequestID(reqID).WithPayload(claManagerDesignee)
	})

	api.ClaManagerCreateCLAManagerDesigneeByGroupHandler = cla_manager.CreateCLAManagerDesigneeByGroupHandlerFunc(
		func(params cla_manager.CreateCLAManagerDesigneeByGroupParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.cla_manager.handlers.ClaManagerCreateCLAManagerDesigneeByGroupHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"CompanySFID":    params.CompanyID,
				"ClaGroupID":     params.ClaGroupID,
				"Email":          params.Body.UserEmail.String(),
				"authUser":       utils.StringValue(params.XUSERNAME),
			}

			// Note: anyone create assign a CLA manager designee...no permissions checks
			log.WithFields(f).Debugf("processing CLA Manager Designee by group request")

			log.WithFields(f).Debugf("getting project IDs for CLA group")
			projectCLAGroups, getErr := projectClaGroupRepo.GetProjectsIdsForClaGroup(ctx, params.ClaGroupID)
			if getErr != nil {
				msg := fmt.Sprintf("error getting SF projects for claGroup: %s ", params.ClaGroupID)
				log.WithFields(f).WithError(getErr).Warn(msg)
				return cla_manager.NewCreateCLAManagerDesigneeByGroupBadRequest().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, getErr))
			}

			log.WithFields(f).Debugf("found %d project IDs for CLA group", len(projectCLAGroups))
			if len(projectCLAGroups) == 0 {
				msg := fmt.Sprintf("no projects associated with CLA Group: %s", params.ClaGroupID)
				log.WithFields(f).Warn(msg)
				return cla_manager.NewCreateCLAManagerDesigneeByGroupNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
			}

			designeeScopes, msg, err := service.CreateCLAManagerDesigneeByGroup(ctx, params, projectCLAGroups)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("problem creating cla manager designee for CLA Group: %s with user email: %s", params.ClaGroupID, params.Body.UserEmail)
				if err == ErrCLAManagerDesigneeConflict {
					return cla_manager.NewCreateCLAManagerDesigneeByGroupConflict().WithXRequestID(reqID).WithPayload(utils.ErrorResponseConflictWithError(reqID, msg, err))
				}
				return cla_manager.NewCreateCLAManagerDesigneeByGroupBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			return cla_manager.NewCreateCLAManagerDesigneeByGroupOK().WithXRequestID(reqID).WithPayload(&models.ClaManagerDesignees{
				List: designeeScopes,
			})
		})

	api.ClaManagerIsCLAManagerDesigneeHandler = cla_manager.IsCLAManagerDesigneeHandlerFunc(func(params cla_manager.IsCLAManagerDesigneeParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

		userRoleStatus, err := service.IsCLAManagerDesignee(ctx, params.CompanySFID, params.ClaGroupID, params.UserLFID)
		if err != nil {
			msg := fmt.Sprintf("Problem checking cla-manager-designee role status for user: %s, error: %+v  ", params.UserLFID, err)
			return cla_manager.NewIsCLAManagerDesigneeBadRequest().WithXRequestID(reqID).WithPayload(
				&models.ErrorResponse{
					Code:       BadRequest,
					Message:    msg,
					XRequestID: reqID,
				})
		}

		return cla_manager.NewIsCLAManagerDesigneeOK().WithXRequestID(reqID).WithPayload(userRoleStatus)
	})

	api.ClaManagerInviteCompanyAdminHandler = cla_manager.InviteCompanyAdminHandlerFunc(func(params cla_manager.InviteCompanyAdminParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

		// Get Contributor details
		user, userErr := easyCLAUserRepo.GetUser(params.UserID)
		if userErr != nil {
			msg := fmt.Sprintf("Problem getting user by ID : %s, error: %+v ", params.UserID, userErr)
			return cla_manager.NewInviteCompanyAdminBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, userErr))
		}

		claManagerDesignees, err := service.InviteCompanyAdmin(ctx, params.Body.ContactAdmin, params.Body.CompanyID, *params.Body.ClaGroupID, params.Body.UserEmail.String(), params.Body.Name, &user)

		if err != nil {
			statusCode := buildErrorStatusCode(err)
			if statusCode == NotFound {
				return cla_manager.NewInviteCompanyAdminNotFound().WithXRequestID(reqID).WithPayload(
					&models.ErrorResponse{
						Message:    err.Error(),
						Code:       NotFound,
						XRequestID: reqID,
					})
			}
			if statusCode == Conflict {
				msg := fmt.Sprintf("User %s already has role scope assigned ", params.Body.UserEmail)
				return cla_manager.NewInviteCompanyAdminConflict().WithXRequestID(reqID).WithPayload(
					&models.ErrorResponse{
						Message:    msg,
						Code:       Conflict,
						XRequestID: reqID,
					})
			}
			if statusCode == Accepted {
				msg := fmt.Sprintf("User %s has no LF Login account ", params.Body.UserEmail)
				return cla_manager.NewInviteCompanyAdminAccepted().WithXRequestID(reqID).WithPayload(
					&models.SuccessResponse{
						Message:    msg,
						Code:       Accepted,
						XRequestID: reqID,
					})
			}

			return cla_manager.NewInviteCompanyAdminBadRequest().WithXRequestID(reqID).WithPayload(
				&models.ErrorResponse{
					Message:    err.Error(),
					Code:       BadRequest,
					XRequestID: reqID,
				})
			//return cla_manager.NewInviteCompanyAdminBadRequest().WithPayload(err)
		}

		// successfully created cla manager designee and sent invite
		return cla_manager.NewInviteCompanyAdminOK().WithXRequestID(reqID).WithPayload(&models.ClaManagerDesignees{
			List: claManagerDesignees,
		})
	})

	api.ClaManagerCreateCLAManagerRequestHandler = cla_manager.CreateCLAManagerRequestHandlerFunc(func(params cla_manager.CreateCLAManagerRequestParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.cla_manager.handlers.ClaManagerCreateCLAManagerRequestHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"CompanyID":      params.CompanyID,
			"ProjectSFID":    params.ProjectSFID,
			"contactAdmin":   params.Body.ContactAdmin,
			"userFullName":   utils.StringValue(params.Body.FullName),
			"userEmail":      params.Body.UserEmail.String(),
			"authUserName":   utils.StringValue(params.XUSERNAME),
			"authUserEmail":  utils.StringValue(params.XEMAIL),
		}

		// Lookup the company by internal ID
		log.WithFields(f).Debugf("looking up company by internal ID...")
		v1CompanyModel, err := v1CompanyService.GetCompany(ctx, params.CompanyID)
		if err != nil || v1CompanyModel == nil {
			msg := fmt.Sprintf("unable to lookup company by ID: %s", params.CompanyID)
			log.WithFields(f).WithError(err).Warn(msg)
			return cla_manager.NewCreateCLAManagerRequestBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		// Check perms...
		if !utils.IsUserAuthorizedForOrganization(ctx, authUser, v1CompanyModel.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("user %s does not have access to CreateCLAManagerRequest with Project|Organization scope of %s | %s",
				authUser.UserName, params.ProjectSFID, v1CompanyModel.CompanyExternalID)
			log.WithFields(f).Warn(msg)
			return cla_manager.NewCreateCLAManagerRequestForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		claManagerDesignee, err := service.CreateCLAManagerRequest(ctx, params.Body.ContactAdmin, v1CompanyModel.CompanyID, params.ProjectSFID, params.Body.UserEmail.String(),
			*params.Body.FullName, authUser)

		if err != nil {
			statusCode := buildErrorStatusCode(err)
			if statusCode == NotFound {
				return cla_manager.NewCreateCLAManagerRequestNotFound().WithXRequestID(reqID).WithPayload(
					&models.ErrorResponse{
						Message:    err.Error(),
						Code:       NotFound,
						XRequestID: reqID,
					})
			}
			if statusCode == Conflict {
				msg := fmt.Sprintf("User %s already has role scope assigned ", *params.Body.FullName)
				return cla_manager.NewCreateCLAManagerRequestConflict().WithXRequestID(reqID).WithPayload(
					&models.ErrorResponse{
						Message:    msg,
						Code:       Conflict,
						XRequestID: reqID,
					})
			}

			// Return Bad Request
			return cla_manager.NewCreateCLAManagerRequestBadRequest().WithXRequestID(reqID).WithPayload(
				&models.ErrorResponse{
					Message:    err.Error(),
					Code:       BadRequest,
					XRequestID: reqID,
				})
		}
		if params.Body.ContactAdmin {
			return cla_manager.NewCreateCLAManagerRequestNoContent().WithXRequestID(reqID)
		}
		return cla_manager.NewCreateCLAManagerRequestOK().WithXRequestID(reqID).WithPayload(claManagerDesignee)
	})

	api.ClaManagerNotifyCLAManagersHandler = cla_manager.NotifyCLAManagersHandlerFunc(
		func(params cla_manager.NotifyCLAManagersParams) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			f := logrus.Fields{
				"functionName":      "v2.cla_manager.handlers.ClaManagerNotifyCLAManagersHandler",
				utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
				"companyName":       params.Body.CompanyName,
				"signingEntityName": params.Body.SigningEntityName,
				"userID":            params.Body.UserID,
				"claGroupName":      params.Body.ClaGroupID,
			}
			log.WithFields(f).Debug("notifying CLA managers...")
			err := service.NotifyCLAManagers(ctx, params.Body, CorporateConsoleV2URL)
			if err != nil {
				if err == ErrCLAUserNotFound {
					msg := fmt.Sprintf("unable to notify cla managers - user not found: %s", params.Body.UserID)
					log.WithFields(f).WithError(err).Warn(err)
					return cla_manager.NewNotifyCLAManagersNotFound().WithXRequestID(reqID).WithPayload(
						utils.ErrorResponseNotFound(reqID, msg))
				}

				msg := fmt.Sprintf("unable to notify cla managers - cla group: %s, company: %s", params.Body.ClaGroupID, params.Body.CompanyName)
				log.WithFields(f).WithError(err).Warn(err)
				return cla_manager.NewNotifyCLAManagersBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			return cla_manager.NewNotifyCLAManagersNoContent().WithXRequestID(reqID)
		})
}
