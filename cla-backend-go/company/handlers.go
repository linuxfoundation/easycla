// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
		// Persisted row when it exists, otherwise a non-persisted virtual company backed by the Salesforce org (#2751)
		companyModel, err := service.ResolveCompany(ctx, params.CompanySFID)
		if err != nil {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - unable to get associated salesforce Organization using SFID: %s, error: %v", params.CompanySFID, err)
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

		// Only existing company ACL members may add others to the ACL
		if !claUserHasLFLogin(claUser) {
			return company.NewAddUsertoCompanyAccessListUnauthorized().WithXRequestID(reqID).WithPayload(missingLFLoginError())
		}
		if aclErr := claUserInCompanyACL(ctx, service, params.CompanyID, claUser); aclErr != nil {
			log.Warnf("company access list update for company id: %s by user %s refused: %v", params.CompanyID, claUser.LFUsername, aclErr)
			return company.NewAddUsertoCompanyAccessListForbidden().WithXRequestID(reqID).WithPayload(errorResponse(aclErr))
		}

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
		if !claUserHasLFLogin(claUser) {
			return company.NewApproveCompanyAccessRequestUnauthorized().WithXRequestID(reqID).WithPayload(missingLFLoginError())
		}
		if aclErr := companyInviteDecisionAllowed(ctx, service, params.CompanyID, params.RequestID, claUser); aclErr != nil {
			log.Warnf("approve company access request ID: %s for company id: %s by user %s refused: %v", params.RequestID, params.CompanyID, claUser.LFUsername, aclErr)
			return company.NewApproveCompanyAccessRequestForbidden().WithXRequestID(reqID).WithPayload(errorResponse(aclErr))
		}
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
		if !claUserHasLFLogin(claUser) {
			return company.NewRejectCompanyAccessRequestUnauthorized().WithXRequestID(reqID).WithPayload(missingLFLoginError())
		}
		if aclErr := companyInviteDecisionAllowed(ctx, service, params.CompanyID, params.RequestID, claUser); aclErr != nil {
			log.Warnf("reject company access request ID: %s for company id: %s by user %s refused: %v", params.RequestID, params.CompanyID, claUser.LFUsername, aclErr)
			return company.NewRejectCompanyAccessRequestForbidden().WithXRequestID(reqID).WithPayload(errorResponse(aclErr))
		}
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

		// Short prefixes (e.g. "Te") expand to huge Salesforce result sets and have
		// caused lambda timeouts. When companyName is shorter than 3 characters,
		// return an empty result set rather than an error so autocomplete UIs that
		// fire requests on every keystroke don't surface a 4xx to the user.
		// websiteName / filter searches are unaffected.
		if params.CompanyName != nil && len(strings.TrimSpace(*params.CompanyName)) < 3 {
			log.WithFields(f).Debugf("companyName shorter than 3 characters; returning empty result")
			return organization.NewSearchOrganizationOK().WithXRequestID(reqID).WithPayload(&models.OrgList{List: []*models.Org{}})
		}

		companyName, websiteName, filter := validateParams(params)

		result, err := service.SearchOrganizationByName(ctx, companyName, websiteName, utils.BoolValue(params.IncludeSigningEntityName), filter)
		if err != nil {
			companyName := "<nil>"
			if params.CompanyName != nil {
				companyName = *params.CompanyName
			}
			log.Warnf("error occurred while search org %s. error = %s", companyName, err.Error())
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

// companyACLError is returned by the v3 company ACL guards - it carries the 403 code for errorResponse
type companyACLError struct {
	message string
}

func (e *companyACLError) Error() string { return e.message }
func (e *companyACLError) Code() string  { return "403" }

// claUserHasLFLogin reports whether the authorizer supplied a user with an LF login (needed for company_acl checks)
func claUserHasLFLogin(claUser *user.CLAUser) bool {
	return claUser != nil && claUser.LFUsername != ""
}

// missingLFLoginError is the 401 payload returned when no authenticated LF user is available
func missingLFLoginError() *models.ErrorResponse {
	return &models.ErrorResponse{
		Code:    "401",
		Message: "EasyCLA - 401 Unauthorized - missing authenticated LF user",
	}
}

// claUserInCompanyACL loads the company and refuses v3 callers whose LF username is not on its company_acl
func claUserInCompanyACL(ctx context.Context, service IService, companyID string, claUser *user.CLAUser) error {
	if !claUserHasLFLogin(claUser) {
		return &companyACLError{message: "EasyCLA - 403 Forbidden - no authenticated LF user"}
	}
	companyModel, err := service.GetCompany(ctx, companyID)
	if err != nil {
		return &companyACLError{message: fmt.Sprintf("EasyCLA - 403 Forbidden - unable to load company %s: %v", companyID, err)}
	}
	if companyModel == nil {
		return &companyACLError{message: fmt.Sprintf("EasyCLA - 403 Forbidden - company %s not found", companyID)}
	}
	for _, lfUsername := range companyModel.CompanyACL {
		if lfUsername == claUser.LFUsername {
			return nil
		}
	}
	return &companyACLError{message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s is not a CLA manager of company %s", claUser.LFUsername, companyID)}
}

// companyInviteDecisionAllowed refuses approve/reject calls unless the caller is on the company ACL, the request
// belongs to the company named in the path and the caller is not the requester. The service approves by invite ID
// into the invite's own company, so a request missing from this company's list is refused (fail-closed) rather than
// trusted; the list is a single-partition query that only pages past 1 MB of invites for one company.
func companyInviteDecisionAllowed(ctx context.Context, service IService, companyID, requestID string, claUser *user.CLAUser) error {
	if err := claUserInCompanyACL(ctx, service, companyID, claUser); err != nil {
		return err
	}
	invites, err := service.GetCompanyInviteRequests(ctx, companyID, nil)
	if err != nil {
		return &companyACLError{message: fmt.Sprintf("EasyCLA - 403 Forbidden - unable to load access requests of company %s: %v", companyID, err)}
	}
	for _, invite := range invites {
		if invite.InviteID != requestID {
			continue
		}
		if invite.UserLFID == claUser.LFUsername {
			return &companyACLError{message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s cannot decide their own access request %s", claUser.LFUsername, requestID)}
		}
		return nil
	}
	return &companyACLError{message: fmt.Sprintf("EasyCLA - 403 Forbidden - access request %s does not belong to company %s", requestID, companyID)}
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
