// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/organization"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/company"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	orgService "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"

	"github.com/go-openapi/runtime/middleware"
)

// Configure sets up the middleware handlers
func Configure(api *operations.ClaAPI, service IService, usersService users.Service, companyUserValidation bool, eventsService events.Service) { // nolint

	api.CompanyGetCompaniesHandler = company.GetCompaniesHandlerFunc(func(params company.GetCompaniesParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		companiesModel, err := service.GetCompanies(ctx)
		if err != nil {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - unable to query all companies, error: %v", err)
			log.Warnf("%s", msg)
			return company.NewGetCompaniesBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Code:    "400",
				Message: msg,
			})
		}

		return company.NewGetCompaniesOK().WithXRequestID(reqID).WithPayload(companiesModel)
	})

	api.CompanyGetCompanyHandler = company.GetCompanyHandlerFunc(func(params company.GetCompanyParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		companyModel, err := service.GetCompany(ctx, params.CompanyID)
		if err != nil {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - unable to query company by ID: %s, error: %v", params.CompanyID, err)
			log.Warnf("%s", msg)
			return company.NewGetCompanyBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Code:    "400",
				Message: msg,
			})
		}

		if companyModel.CompanyID == "" || companyModel.CompanyName == "" {
			return company.NewGetCompanyNotFound().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Code:    "404",
				Message: fmt.Sprintf("Company Not Found with ID: %s", params.CompanyID),
			})
		}

		return company.NewGetCompanyOK().WithXRequestID(reqID).WithPayload(companyModel)
	})

	api.CompanyGetCompanyByExternalIDHandler = company.GetCompanyByExternalIDHandlerFunc(func(params company.GetCompanyByExternalIDParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		// Check for Salesforce org
		orgClient := orgService.GetClient()
		org, getErr := orgClient.GetOrganization(ctx, params.CompanySFID)

		if getErr != nil {
			msg := fmt.Sprintf("Failed to get salesforce org for ID: %s ", params.CompanySFID)
			log.Warn(msg)
			return company.NewGetCompanyByExternalIDBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Code:    "400",
				Message: msg,
			})
		}
		companyModel, err := service.GetCompanyByExternalID(ctx, params.CompanySFID)
		if err != nil {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - unable to get associated salesforce Organization: %s using SFID: %s, error: %v", org.Name, params.CompanySFID, err)
			log.Warnf("%s", msg)
			return company.NewGetCompanyByExternalIDBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Code:    "400",
				Message: msg,
			})
		}
		return company.NewGetCompanyByExternalIDOK().WithXRequestID(reqID).WithPayload(companyModel)
	})

	api.CompanyGetCompanyBySigningEntityNameHandler = company.GetCompanyBySigningEntityNameHandlerFunc(func(params company.GetCompanyBySigningEntityNameParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		f := logrus.Fields{
			"functionName":      "company.handler.CompanyGetCompanyBySigningEntityNameHandler",
			"signingEntityName": params.Name,
			"companySFID":       params.CompanySFID,
		}
		log.WithFields(f).Debug("Searching by signing entity name...")
		companyModel, err := service.GetCompanyBySigningEntityName(ctx, params.Name, params.CompanySFID)
		if err != nil {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - Unable to locate Company with Signing Entity Request of %s", params.Name)
			log.Warnf("%s", msg)
			return company.NewGetCompanyBySigningEntityNameBadRequest().WithPayload(&models.ErrorResponse{
				Code:    "400",
				Message: msg,
			})
		}
		return company.NewGetCompanyBySigningEntityNameOK().WithXRequestID(reqID).WithPayload(companyModel)
	})

	api.CompanySearchCompanyHandler = company.SearchCompanyHandlerFunc(func(params company.SearchCompanyParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		f := logrus.Fields{
			"functionName": "company.handler.CompanySearchCompanyHandler",
			"CompanyName":  params.CompanyName,
			"NextKey":      params.NextKey,
		}
		log.WithFields(f).Debug("Searching company...")
		var nextKey = ""
		if params.NextKey != nil {
			nextKey = *params.NextKey
		}

		companiesModel, err := service.SearchCompanyByName(ctx, params.CompanyName, nextKey)
		if err != nil {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - unable to query company by name: %s, error: %v", params.CompanyName, err)
			log.Warnf("%s", msg)
			return company.NewSearchCompanyBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Code:    "400",
				Message: msg,
			})
		}

		return company.NewSearchCompanyOK().WithXRequestID(reqID).WithPayload(companiesModel)
	})

	api.CompanyGetCompaniesByUserManagerHandler = company.GetCompaniesByUserManagerHandlerFunc(func(params company.GetCompaniesByUserManagerParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		f := logrus.Fields{
			"functionName": "company.handler.CompanyGetCompaniesByUserManagerHandler",
			"UserID":       params.UserID,
		}
		log.WithFields(f).Debug("Searching company by user manager...")
		if companyUserValidation {
			log.Debugf("Company User Validation - claUser: %+v", claUser)
			userModel, userErr := usersService.GetUserByUserName(claUser.LFUsername, true)
			if userErr != nil {
				return company.NewGetCompaniesByUserManagerUnauthorized().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code:    "401",
					Message: fmt.Sprintf("unauthorized - unable to find current logged in user by lf_username: %s", claUser.LFUsername),
				})
			}

			if params.UserID == "" || params.UserID != userModel.UserID {
				return company.NewGetCompaniesByUserManagerUnauthorized().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code:    "401",
					Message: fmt.Sprintf("unauthorized - userID mismatch: param user id: %s, claUser id: %s", params.UserID, userModel.UserID),
				})
			}
		}

		companies, err := service.GetCompaniesByUserManager(ctx, params.UserID)
		if err != nil {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - unable to query companies by user manager id: %s, error: %v", params.UserID, err)
			log.Warnf("%s", msg)
			return company.NewGetCompaniesByUserManagerBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Code:    "400",
				Message: msg,
			})
		}

		return company.NewGetCompaniesByUserManagerOK().WithXRequestID(reqID).WithPayload(companies)
	})

	api.CompanyGetCompaniesByUserManagerWithInvitesHandler = company.GetCompaniesByUserManagerWithInvitesHandlerFunc(func(params company.GetCompaniesByUserManagerWithInvitesParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		if companyUserValidation {
			log.Debugf("Company User Validation - GetUserByUserName() - claUser: %+v", claUser)
			userModel, userErr := usersService.GetUserByUserName(claUser.LFUsername, true)
			if userErr != nil {
				return company.NewGetCompaniesByUserManagerUnauthorized().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code:    "401",
					Message: fmt.Sprintf("unauthorized - unable to find current logged in user by lf_username: %s", claUser.LFUsername),
				})
			}

			if params.UserID == "" || params.UserID != userModel.UserID {
				return company.NewGetCompaniesByUserManagerUnauthorized().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code:    "401",
					Message: fmt.Sprintf("unauthorized - userID mismatch: param user id: %s, claUser id: %s", params.UserID, userModel.UserID),
				})
			}
		}

		companies, err := service.GetCompaniesByUserManagerWithInvites(ctx, params.UserID)
		if err != nil {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - unable to query companies by user manager id: %s, error: %v", params.UserID, err)
			log.Warnf("%s", msg)
			return company.NewGetCompaniesByUserManagerWithInvitesBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Code:    "400",
				Message: msg,
			})
		}

		return company.NewGetCompaniesByUserManagerWithInvitesOK().WithXRequestID(reqID).WithPayload(companies)
	})

	api.CompanyGetCompanyInviteRequestsHandler = company.GetCompanyInviteRequestsHandlerFunc(func(params company.GetCompanyInviteRequestsParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		log.Debugf("Processing get company invite request for company ID: %s", params.CompanyID)
		result, err := service.GetCompanyInviteRequests(ctx, params.CompanyID, params.Status)
		if err != nil {
			log.Warnf("error getting company invite using company id: %s, error: %v", params.CompanyID, err)
			return company.NewGetCompanyInviteRequestsBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		return company.NewGetCompanyInviteRequestsOK().WithXRequestID(reqID).WithPayload(result)
	})

	api.CompanyGetCompanyUserInviteRequestsHandler = company.GetCompanyUserInviteRequestsHandlerFunc(func(params company.GetCompanyUserInviteRequestsParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		log.Debugf("Processing get company user invite request for company ID: %s and user ID: %s", params.CompanyID, params.UserID)
		result, err := service.GetCompanyUserInviteRequests(ctx, params.CompanyID, params.UserID)
		if err != nil {
			log.Warnf("error getting company user invite using company id: %s, user id: %s, error: %v", params.CompanyID, params.UserID, err)
			return company.NewGetCompanyUserInviteRequestsBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		if result == nil {
			return company.NewGetCompanyUserInviteRequestsNotFound().WithXRequestID(reqID)
		}

		return company.NewGetCompanyUserInviteRequestsOK().WithXRequestID(reqID).WithPayload(result)
	})

	api.CompanyAddUsertoCompanyAccessListHandler = company.AddUsertoCompanyAccessListHandlerFunc(func(params company.AddUsertoCompanyAccessListParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		err := service.AddUserToCompanyAccessList(ctx, params.CompanyID, params.User.UserLFID)
		if err != nil {
			log.Warnf("error adding user to company access list using company id: %s, invite id: %s, and user LFID: %s, error: %v",
				params.CompanyID, params.User.InviteID, params.User.UserLFID, err)
			return company.NewAddUsertoCompanyAccessListBadRequest().WithXRequestID(reqID)
		}

		eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType: events.CompanyACLUserAdded,
			CompanyID: params.CompanyID,
			UserID:    claUser.UserID,
			EventData: &events.CompanyACLUserAddedEventData{
				UserLFID: params.User.UserLFID,
			},
		})

		return company.NewAddUsertoCompanyAccessListOK().WithXRequestID(reqID)
	})

	api.CompanyRequestCompanyAccessRequestHandler = company.RequestCompanyAccessRequestHandlerFunc(func(params company.RequestCompanyAccessRequestParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		log.Debugf("Processing company access request for company ID: %s, by user %+v", params.CompanyID, claUser)
		newInvite, err := service.AddPendingCompanyInviteRequest(ctx, params.CompanyID, claUser.UserID)
		if err != nil {
			log.Warnf("error creating company access request for company id: %s, User: %+v, error: %v", params.CompanyID, claUser, err)
			return company.NewRequestCompanyAccessRequestBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		// Add an event to the log
		eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType: events.CompanyACLRequestAdded,
			CompanyID: params.CompanyID,
			UserID:    claUser.UserID,
			EventData: &events.CompanyACLRequestAddedEventData{
				UserID:    newInvite.UserID,
				UserName:  newInvite.UserName,
				UserEmail: newInvite.UserEmail,
			},
		})

		return company.NewRequestCompanyAccessRequestOK().WithXRequestID(reqID)
	})

	api.CompanyApproveCompanyAccessRequestHandler = company.ApproveCompanyAccessRequestHandlerFunc(func(params company.ApproveCompanyAccessRequestParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		log.Debugf("Processing approve company access request for request ID: %s, company ID: %s, by user %+v", params.RequestID, params.CompanyID, claUser)
		inviteModel, err := service.ApproveCompanyAccessRequest(ctx, params.RequestID)
		if err != nil {
			log.Warnf("error approving company access for request ID: %s, company id: %s, error: %v", params.RequestID, params.CompanyID, err)
			return company.NewApproveCompanyAccessRequestBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		// Add an event to the log
		eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType: events.CompanyACLRequestApproved,
			CompanyID: params.CompanyID,
			UserID:    claUser.UserID,
			EventData: &events.CompanyACLRequestApprovedEventData{
				UserID:    inviteModel.UserID,
				UserName:  inviteModel.UserName,
				UserEmail: inviteModel.UserEmail,
			},
		})

		return company.NewApproveCompanyAccessRequestOK().WithXRequestID(reqID)
	})

	api.CompanyRejectCompanyAccessRequestHandler = company.RejectCompanyAccessRequestHandlerFunc(func(params company.RejectCompanyAccessRequestParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		log.Debugf("Processing reject company access request for request ID: %s, company ID: %s, by user %+v", params.RequestID, params.CompanyID, claUser)
		inviteModel, err := service.RejectCompanyAccessRequest(ctx, params.RequestID)
		if err != nil {
			log.Warnf("error rejecting company access for request ID: %s, company id: %s, error: %v", params.RequestID, params.CompanyID, err)
			return company.NewRejectCompanyAccessRequestBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		// Add an event to the log
		eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType: events.CompanyACLRequestDenied,
			CompanyID: params.CompanyID,
			UserID:    claUser.UserID,
			EventData: &events.CompanyACLRequestDeniedEventData{
				UserID:    inviteModel.UserID,
				UserName:  inviteModel.UserName,
				UserEmail: inviteModel.UserEmail,
			},
		})

		return company.NewRejectCompanyAccessRequestOK().WithXRequestID(reqID)
	})

	api.OrganizationSearchOrganizationHandler = organization.SearchOrganizationHandlerFunc(func(params organization.SearchOrganizationParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		f := logrus.Fields{
			"functionName":             "company.handler.OrganizationSearchOrganizationHandler",
			"companyName":              params.CompanyName,
			"websiteName":              params.WebsiteName,
			"includeSigningEntityName": params.IncludeSigningEntityName,
		}

		if params.CompanyName == nil && params.WebsiteName == nil && params.DollarFilter == nil {
			log.WithFields(f).Debugf("CompanyName or WebsiteName or filter at least one required")
			return organization.NewSearchOrganizationBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(errors.New("companyName or websiteName or filter at least one required")))
		}

		companyName, websiteName, filter := validateParams(params)

		result, err := service.SearchOrganizationByName(ctx, companyName, websiteName, utils.BoolValue(params.IncludeSigningEntityName), filter)
		if err != nil {
			log.Warnf("error occurred while search org %s. error = %s", *params.CompanyName, err.Error())
			return organization.NewSearchOrganizationInternalServerError().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}
		return organization.NewSearchOrganizationOK().WithXRequestID(reqID).WithPayload(result)
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

func validateParams(params organization.SearchOrganizationParams) (string, string, string) {
	var companyName, websiteName, filter string
	if params.CompanyName == nil {
		companyName = ""
	} else {
		companyName = *params.CompanyName
	}

	if params.WebsiteName == nil {
		websiteName = ""
	} else {
		websiteName = *params.WebsiteName
	}

	if params.DollarFilter == nil {
		filter = ""
	} else {
		filter = *params.DollarFilter
	}
	return companyName, websiteName, filter
}
