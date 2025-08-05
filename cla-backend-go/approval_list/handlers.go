// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package approval_list

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/company"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/savaki/dynastore"
)

// Configure setups handlers on api with service
func Configure(api *operations.ClaAPI, service IService, sessionStore *dynastore.Store, signatureService signatures.SignatureService, eventsService events.Service) {

	api.CompanyAddCclaAllowlistRequestHandler = company.AddCclaAllowlistRequestHandlerFunc(
		func(params company.AddCclaAllowlistRequestParams) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			requestID, err := service.AddCclaApprovalListRequest(ctx, params.CompanyID, params.ProjectID, params.Body)
			if err != nil {
				return company.NewAddCclaAllowlistRequestBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
			}

			eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType: events.CCLAApprovalListRequestCreated,
				ProjectID: params.ProjectID,
				CompanyID: params.CompanyID,
				UserID:    params.Body.ContributorID,
				EventData: &events.CCLAApprovalListRequestCreatedEventData{RequestID: requestID},
			})

			return company.NewAddCclaAllowlistRequestOK().WithXRequestID(reqID)
		})

	api.CompanyApproveCclaAllowlistRequestHandler = company.ApproveCclaAllowlistRequestHandlerFunc(
		func(params company.ApproveCclaAllowlistRequestParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			err := service.ApproveCclaApprovalListRequest(ctx, claUser, params.CompanyID, params.ProjectID, params.RequestID)
			if err != nil {
				return company.NewApproveCclaAllowlistRequestBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
			}

			eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType: events.CCLAApprovalListRequestApproved,
				ProjectID: params.ProjectID,
				CompanyID: params.CompanyID,
				UserID:    claUser.UserID,
				EventData: &events.CCLAApprovalListRequestApprovedEventData{RequestID: params.RequestID},
			})

			return company.NewApproveCclaAllowlistRequestOK().WithXRequestID(reqID)
		})

	api.CompanyRejectCclaAllowlistRequestHandler = company.RejectCclaAllowlistRequestHandlerFunc(
		func(params company.RejectCclaAllowlistRequestParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			err := service.RejectCclaApprovalListRequest(ctx, params.CompanyID, params.ProjectID, params.RequestID)
			if err != nil {
				return company.NewRejectCclaAllowlistRequestBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
			}

			eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType: events.CCLAApprovalListRequestRejected,
				ProjectID: params.ProjectID,
				CompanyID: params.CompanyID,
				UserID:    claUser.UserID,
				EventData: &events.CCLAApprovalListRequestRejectedEventData{RequestID: params.RequestID},
			})

			return company.NewRejectCclaAllowlistRequestOK().WithXRequestID(reqID)
		})

	api.CompanyListCclaAllowlistRequestsHandler = company.ListCclaAllowlistRequestsHandlerFunc(
		func(params company.ListCclaAllowlistRequestsParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			f := logrus.Fields{
				"functionName":   "CompanyListCclaAllowlistRequestsHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			}
			log.WithFields(f).Debugf("Invoking ListCclaApprovalListRequests with Company ID: %+v, Project ID: %+v, Status: %+v",
				params.CompanyID, params.ProjectID, params.Status)
			result, err := service.ListCclaApprovalListRequest(params.CompanyID, params.ProjectID, params.Status)
			if err != nil {
				return company.NewListCclaAllowlistRequestsBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
			}

			return company.NewListCclaAllowlistRequestsOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.CompanyListCclaAllowlistRequestsByCompanyAndProjectHandler = company.ListCclaAllowlistRequestsByCompanyAndProjectHandlerFunc(
		func(params company.ListCclaAllowlistRequestsByCompanyAndProjectParams, claUser *user.CLAUser) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			f := logrus.Fields{
				"functionName":      "v1.approval_list.handlers.CompanyListCclaAllowlistRequestsByCompanyAndProjectHandler",
				utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
				"companyID":         params.CompanyID,
				"projectID":         params.ProjectID,
				"status":            utils.StringValue(params.Status),
				"claUserName":       claUser.Name,
				"claUserUserID":     claUser.UserID,
				"claUserLFEmail":    claUser.LFEmail,
				"claUserLFUsername": claUser.LFUsername,
			}
			log.WithFields(f).Debugf("Invoking ListCclaApprovalListRequestByCompanyProjectUser with Company ID: %+v, Project ID: %+v, Status: %+v",
				params.CompanyID, params.ProjectID, params.Status)
			result, err := service.ListCclaApprovalListRequestByCompanyProjectUser(params.CompanyID, &params.ProjectID, params.Status, nil)
			if err != nil {
				return company.NewListCclaAllowlistRequestsByCompanyAndProjectBadRequest().WithPayload(errorResponse(err))
			}

			return company.NewListCclaAllowlistRequestsByCompanyAndProjectOK().WithPayload(result)
		})

	api.CompanyListCclaAllowlistRequestsByCompanyAndProjectAndUserHandler = company.ListCclaAllowlistRequestsByCompanyAndProjectAndUserHandlerFunc(
		func(params company.ListCclaAllowlistRequestsByCompanyAndProjectAndUserParams, claUser *user.CLAUser) middleware.Responder {
			log.Debugf("Invoking ListCclaApprovalListRequestByCompanyProjectUser with Company ID: %+v, Project ID: %+v, Status: %+v, User: %+v",
				params.CompanyID, params.ProjectID, params.Status, claUser.LFUsername)
			result, err := service.ListCclaApprovalListRequestByCompanyProjectUser(params.CompanyID, &params.ProjectID, params.Status, &claUser.LFUsername)
			if err != nil {
				return company.NewListCclaAllowlistRequestsByCompanyAndProjectAndUserBadRequest().WithPayload(errorResponse(err))
			}

			return company.NewListCclaAllowlistRequestsByCompanyAndProjectAndUserOK().WithPayload(result)
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
