// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/go-openapi/runtime"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/LF-Engineering/lfx-kit/auth"
	v1Company "github.com/communitybridge/easycla/cla-backend-go/company"
	v1Events "github.com/communitybridge/easycla/cla-backend-go/events"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/events"
	v1ProjectService "github.com/communitybridge/easycla/cla-backend-go/project/service"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
)

// Configure setups handlers on api with service
func Configure(api *operations.EasyclaAPI, service v1Events.Service, v1CompanyRepo v1Company.IRepository, projectsClaGroupsRepo projects_cla_groups.Repository, projectService v1ProjectService.Service) { // nolint
	api.EventsGetRecentEventsHandler = events.GetRecentEventsHandlerFunc(
		func(params events.GetRecentEventsParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "EventsGetRecentEventsHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   authUser.UserName,
				"authUserEmail":  authUser.Email,
			}

			if !utils.IsUserAdmin(authUser) {
				return events.NewGetRecentEventsForbidden().WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to Get Recent Events - only Admins allowed to see all events.",
						authUser.UserName),
					XRequestID: reqID,
				})
			}

			result, err := service.GetRecentEvents(params.PageSize)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("problem fetching recent events")
				return events.NewGetRecentEventsBadRequest().WithPayload(errorResponse(reqID, err))
			}

			// Return an empty list
			if result == nil || len(result.Events) == 0 {
				return events.NewGetRecentEventsOK().WithPayload(&models.EventList{
					Events: []*models.Event{},
				})
			}

			resp, err := v2EventList(result)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("problem converting events to a v2 object")
				return events.NewGetRecentEventsInternalServerError().WithPayload(errorResponse(reqID, err))
			}

			return events.NewGetRecentEventsOK().WithPayload(resp)
		})

	api.EventsGetFoundationEventsAsCSVHandler = events.GetFoundationEventsAsCSVHandlerFunc(
		func(params events.GetFoundationEventsAsCSVParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "EventsGetFoundationEventsAsCSVHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   authUser.UserName,
				"authUserEmail":  authUser.Email,
				"foundationSFID": params.FoundationSFID,
			}

			log.WithFields(f).Debug("checking permission...")
			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.FoundationSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Get Foundation Events for foundation %s.", authUser.UserName, params.FoundationSFID)
				log.WithFields(f).Warn(msg)
				return WriteResponse(http.StatusForbidden, runtime.JSONMime, runtime.JSONProducer(), utils.ErrorResponseForbidden(reqID, msg))
			}

			result, err := service.GetFoundationEvents(params.FoundationSFID, nil, nil, v1Events.ReturnAllEvents, nil)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("problem fetching foundation events")
				return WriteResponse(http.StatusBadRequest, runtime.JSONMime, runtime.JSONProducer(), errorResponse(reqID, err))
			}

			filename := fmt.Sprintf("foundation-events-%s.csv", params.FoundationSFID)
			csvResponder := CSVEventsResponse(filename, result)
			return csvResponder
		})

	api.EventsGetFoundationEventsHandler = events.GetFoundationEventsHandlerFunc(
		func(params events.GetFoundationEventsParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "EventsGetFoundationEventsHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   authUser.UserName,
				"authUserEmail":  authUser.Email,
				"foundationSFID": params.FoundationSFID,
			}

			log.WithFields(f).Debug("checking permission...")
			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.FoundationSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Get Foundation Events for foundation %s.", authUser.UserName, params.FoundationSFID)
				log.WithFields(f).Warn(msg)
				return events.NewGetRecentEventsForbidden().WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			log.WithFields(f).Debug("querying foundation events...")
			result, err := service.GetFoundationEvents(params.FoundationSFID, params.NextKey, params.PageSize, aws.BoolValue(params.ReturnAllEvents), params.SearchTerm)
			if err != nil {
				msg := "problem fetching foundation events"
				log.WithFields(f).WithError(err).Warn(msg)
				return events.NewGetFoundationEventsBadRequest().WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			// Return an empty list
			if result == nil || len(result.Events) == 0 {
				log.WithFields(f).Debug("no events - returning empty success response")
				return events.NewGetFoundationEventsOK().WithPayload(&models.EventList{
					Events: []*models.Event{},
				})
			}

			resp, err := v2EventList(result)
			if err != nil {
				msg := "problem converting event response to a v2 object"
				log.WithFields(f).WithError(err).Warn(msg)
				return events.NewGetFoundationEventsInternalServerError().WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			return events.NewGetFoundationEventsOK().WithPayload(resp)
		})

	api.EventsGetProjectEventsAsCSVHandler = events.GetProjectEventsAsCSVHandlerFunc(
		func(params events.GetProjectEventsAsCSVParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "EventsGetProjectEventsAsCSVHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   authUser.UserName,
				"authUserEmail":  authUser.Email,
				"projectSFID":    params.ProjectSFID,
			}

			log.WithFields(f).Debug("checking permission...")
			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Get Project Events for foundation %s.", authUser.UserName, params.ProjectSFID)
				log.WithFields(f).Warn(msg)
				return WriteResponse(http.StatusForbidden, runtime.JSONMime, runtime.JSONProducer(), &models.ErrorResponse{
					Code:       "403",
					Message:    fmt.Sprintf("EasyCLA - 403 Forbidden - %s", msg),
					XRequestID: reqID,
				})
			}

			pm, err := projectsClaGroupsRepo.GetClaGroupIDForProject(ctx, params.ProjectSFID)
			if err != nil {
				if err == projects_cla_groups.ErrProjectNotAssociatedWithClaGroup {
					msg := fmt.Sprintf("no cla group associated with this project: %s", params.ProjectSFID)
					log.WithFields(f).Warn(msg)
					return WriteResponse(http.StatusBadRequest, runtime.JSONMime, runtime.JSONProducer(), &models.ErrorResponse{
						Code:       "400",
						Message:    fmt.Sprintf("EasyCLA - 400 Bad Request - %s", msg),
						XRequestID: reqID,
					})
				}

				msg := fmt.Sprintf("unable to get CLA Group for project: %s", params.ProjectSFID)
				return WriteResponse(http.StatusInternalServerError, runtime.JSONMime, runtime.JSONProducer(), utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			result, err := service.GetClaGroupEvents(pm.ClaGroupID, nil, nil, v1Events.ReturnAllEvents, nil)
			if err != nil {
				msg := fmt.Sprintf("problem loading events for CLA Group: %s with ID: %s", pm.ClaGroupName, pm.ClaGroupID)
				log.WithFields(f).Warn(msg)
				return events.NewGetProjectEventsAsCSVBadRequest().WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			filename := fmt.Sprintf("project-events-%s.csv", params.ProjectSFID)
			csvResponder := CSVEventsResponse(filename, result)
			return csvResponder
		})

	api.EventsGetProjectEventsHandler = events.GetProjectEventsHandlerFunc(
		func(params events.GetProjectEventsParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "EventsGetProjectEventsHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   authUser.UserName,
				"authUserEmail":  authUser.Email,
				"projectSFID":    params.ProjectSFID,
			}

			log.WithFields(f).Debug("checking permission...")
			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Get Project Events for foundation %s.", authUser.UserName, params.ProjectSFID)
				log.WithFields(f).Warn(msg)
				return events.NewGetRecentEventsForbidden().WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			// Lookup the CLA Group associated with this Project SFID...
			log.WithFields(f).Debugf("loading CLA Group for projectSFID: %s", params.ProjectSFID)
			pm, err := projectsClaGroupsRepo.GetClaGroupIDForProject(ctx, params.ProjectSFID)
			if err != nil {
				msg := fmt.Sprintf("problem loading CLA Group from Project SFID:: %s", params.ProjectSFID)
				log.WithFields(f).Warn(msg)
				if err == projects_cla_groups.ErrProjectNotAssociatedWithClaGroup {
					// Although the API should view this as a bad request since the project doesn't seem to belong to a
					// CLA Group...just return a successful 200 with an empty list to the caller - nothing to see here, move along.
					return events.NewGetProjectEventsOK().WithPayload(&models.EventList{
						Events: []*models.Event{},
					})
				}

				// Not an error that we are expecting - return an error and give up...
				return events.NewGetProjectEventsBadRequest().WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			// Lookup any events for this CLA Group....
			log.WithFields(f).Debugf("loading CLA Group %s events using ID: %s", pm.ClaGroupName, pm.ClaGroupID)
			result, err := service.GetClaGroupEvents(pm.ClaGroupID, params.NextKey, params.PageSize, aws.BoolValue(params.ReturnAllEvents), params.SearchTerm)
			if err != nil {
				msg := fmt.Sprintf("problem loading events for CLA Group: %s with ID: %s error: %v", pm.ClaGroupName, pm.ClaGroupID, err.Error())
				log.WithFields(f).Warn(msg)
				return events.NewGetProjectEventsOK().WithPayload(&models.EventList{
					Events: []*models.Event{},
				})
			}

			// Return an empty list
			if result == nil || len(result.Events) == 0 {
				log.WithFields(f).Debug("no events - returning empty success response")
				return events.NewGetProjectEventsOK().WithPayload(&models.EventList{
					Events: []*models.Event{},
				})
			}

			resp, err := v2EventList(result)
			if err != nil {
				msg := "problem converting events to a v2 object"
				log.WithFields(f).WithError(err).Warn(msg)
				return events.NewGetProjectEventsInternalServerError().WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			return events.NewGetProjectEventsOK().WithPayload(resp)
		})

	api.EventsGetCompanyProjectEventsHandler = events.GetCompanyProjectEventsHandlerFunc(
		func(params events.GetCompanyProjectEventsParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "EventsGetCompanyProjectEventsHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   authUser.UserName,
				"authUserEmail":  authUser.Email,
				"projectSFID":    params.ProjectSFID,
				"companyID":      params.CompanyID,
			}

			v1Company, compErr := v1CompanyRepo.GetCompany(ctx, params.CompanyID)
			if compErr != nil {
				log.WithFields(f).Warnf("unable to fetch company by ID:%s ", params.CompanyID)
				return events.NewGetCompanyProjectEventsBadRequest().WithPayload(errorResponse(reqID, compErr))
			}

			if !utils.IsUserAuthorizedForOrganization(ctx, authUser, v1Company.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
				return events.NewGetCompanyProjectEventsForbidden().WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to GetCompanyProject Events with Organization scope of %s",
						authUser.UserName, v1Company.CompanyExternalID),
					XRequestID: reqID,
				})
			}

			var err error

			var result *v1Models.EventList

			log.WithFields(f).Debugf("loading CLA Group for projectSFID: %s", params.ProjectSFID)

			result, err = service.GetCompanyClaGroupEvents(params.ProjectSFID, v1Company.CompanyExternalID, params.NextKey, params.PageSize, params.SearchTerm, aws.BoolValue(params.ReturnAllEvents))

			if err != nil {
				log.WithFields(f).WithError(err).Warn("problem loading events")
				return events.NewGetCompanyProjectEventsBadRequest().WithPayload(errorResponse(reqID, err))
			}

			// Return an empty list
			if result == nil || len(result.Events) == 0 {
				return events.NewGetCompanyProjectEventsOK().WithPayload(&models.EventList{
					Events: []*models.Event{},
				})
			}

			resp, err := v2EventList(result)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("problem converting events to a v2 object")
				return events.NewGetCompanyProjectEventsInternalServerError().WithPayload(errorResponse(reqID, err))
			}
			return events.NewGetCompanyProjectEventsOK().WithPayload(resp)
		})
}

// WriteResponse function writes http response.
func WriteResponse(httpStatus int, contentType string, contentProducer runtime.Producer, data interface{}) middleware.Responder {
	return middleware.ResponderFunc(func(rw http.ResponseWriter, pr runtime.Producer) {
		rw.Header().Set(runtime.HeaderContentType, contentType)
		rw.WriteHeader(httpStatus)
		err := contentProducer.Produce(rw, data)
		if err != nil {
			log.Warnf("failed to write data. error = %v", err)
		}
	})
}
