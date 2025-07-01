// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package metrics

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime/middleware"
	v1Company "github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/metrics"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// Configure setups handlers on api with service
func Configure(api *operations.EasyclaAPI, service Service, v1CompanyRepo v1Company.IRepository) {
	api.MetricsGetClaManagerDistributionHandler = metrics.GetClaManagerDistributionHandlerFunc(
		func(params metrics.GetClaManagerDistributionParams, user *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			result, err := service.GetCLAManagerDistribution()
			if err != nil {
				return metrics.NewGetClaManagerDistributionBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return metrics.NewGetClaManagerDistributionOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.MetricsGetTotalCountHandler = metrics.GetTotalCountHandlerFunc(
		func(params metrics.GetTotalCountParams, user *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			result, err := service.GetTotalCountMetrics()
			if err != nil {
				return metrics.NewGetTotalCountBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return metrics.NewGetTotalCountOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.MetricsGetCompanyMetricHandler = metrics.GetCompanyMetricHandlerFunc(
		func(params metrics.GetCompanyMetricParams, user *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			result, err := service.GetCompanyMetric(params.CompanyID)
			if err != nil {
				return metrics.NewGetCompanyMetricBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return metrics.NewGetCompanyMetricOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.MetricsGetProjectMetricHandler = metrics.GetProjectMetricHandlerFunc(
		func(params metrics.GetProjectMetricParams, user *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			result, err := service.GetProjectMetric(params.ProjectID, params.IDType)
			if err != nil {
				if err.Error() == "metric not found" {
					return metrics.NewGetProjectMetricNotFound().WithXRequestID(reqID)
				}
				return metrics.NewGetProjectMetricBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return metrics.NewGetProjectMetricOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.MetricsGetTopCompaniesHandler = metrics.GetTopCompaniesHandlerFunc(
		func(params metrics.GetTopCompaniesParams, user *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			result, err := service.GetTopCompanies()
			if err != nil {
				return metrics.NewGetTopCompaniesBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return metrics.NewGetTopCompaniesOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.MetricsGetTopProjectsHandler = metrics.GetTopProjectsHandlerFunc(
		func(params metrics.GetTopProjectsParams, user *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			result, err := service.GetTopProjects()
			if err != nil {
				return metrics.NewGetTopProjectsBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return metrics.NewGetTopProjectsOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.MetricsListProjectMetricsHandler = metrics.ListProjectMetricsHandlerFunc(
		func(params metrics.ListProjectMetricsParams, user *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			result, err := service.ListProjectMetrics(params.PageSize, params.NextKey)
			if err != nil {
				return metrics.NewListProjectMetricsBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return metrics.NewListProjectMetricsOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.MetricsListCompanyProjectMetricsHandler = metrics.ListCompanyProjectMetricsHandlerFunc(
		func(params metrics.ListCompanyProjectMetricsParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			f := logrus.Fields{
				"functionName":   "MetricsListCompanyProjectMetricsHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"companyID":      params.CompanyID,
			}
			// Lookup the company by internal ID
			log.WithFields(f).Debugf("looking up company by internal ID...")
			company, compErr := v1CompanyRepo.GetCompany(ctx, params.CompanyID)
			if compErr != nil {
				log.WithFields(f).Warnf("unable to fetch company by ID:%s ", params.CompanyID)
				return metrics.NewListCompanyProjectMetricsBadRequest().WithPayload(errorResponse(reqID, compErr))
			}
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			if !utils.IsUserAuthorizedForOrganization(ctx, authUser, company.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
				return metrics.NewListCompanyProjectMetricsForbidden().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to List Company Project Metrics with Organization scope of %s",
						authUser.UserName, company.CompanyExternalID),
					XRequestID: reqID,
				})
			}

			result, err := service.ListCompanyProjectMetrics(ctx, params.CompanyID, params.ProjectSFID)
			if err != nil {
				return metrics.NewListCompanyProjectMetricsBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return metrics.NewListCompanyProjectMetricsOK().WithXRequestID(reqID).WithPayload(result)
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
