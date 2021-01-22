// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	v1Company "github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/company"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/communitybridge/easycla/cla-backend-go/v2/organization-service/client/organizations"
	"github.com/go-openapi/runtime/middleware"
)

// Configure sets up the middleware handlers
func Configure(api *operations.EasyclaAPI, service Service, v1CompanyRepo v1Company.IRepository, projectClaGroupRepo projects_cla_groups.Repository, LFXPortalURL, v1CorporateConsole string) { // nolint

	const msgUnableToLoadCompany = "unable to load company external ID"
	api.CompanyGetCompanyProjectClaManagersHandler = company.GetCompanyProjectClaManagersHandlerFunc(
		func(params company.GetCompanyProjectClaManagersParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "CompanyGetCompanyProjectClaManagersHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"projectSFID":    params.ProjectSFID,
				"companySFID":    params.CompanySFID,
			}

			log.WithFields(f).Debug("checking permissions")
			if !utils.IsUserAuthorizedForOrganization(authUser, params.CompanySFID, utils.ALLOW_ADMIN_SCOPE) {
				return company.NewGetCompanyProjectClaManagersForbidden().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to Get Company Project CLA Managers with Organization scope of %s",
						authUser.UserName, params.CompanySFID),
					XRequestID: reqID,
				})
			}
			comp, err := v1CompanyRepo.GetCompanyByExternalID(ctx, params.CompanySFID)
			if err != nil {
				msg := "unable to load company by SFID"
				log.WithFields(f).WithError(err).Warn(msg)
				if err == v1Company.ErrCompanyDoesNotExist {
					return company.NewGetCompanyProjectClaManagersNotFound().WithXRequestID(reqID).WithPayload(
						utils.ErrorResponseNotFoundWithError(reqID, msg, err))
				}
				return company.NewGetCompanyProjectClaManagersNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, msg, err))
			}
			if comp == nil {
				log.WithFields(f).WithError(err).Warn(msgUnableToLoadCompany)
				return company.NewGetCompanyProjectClaManagersNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFound(reqID, msgUnableToLoadCompany))
			}

			result, err := service.GetCompanyProjectCLAManagers(ctx, comp.CompanyID, params.CompanySFID, params.ProjectSFID)
			if err != nil {
				msg := "unable to load company project CLA managers"
				log.WithFields(f).WithError(err).Warn(msg)
				return company.NewGetCompanyProjectClaManagersBadRequest().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			return company.NewGetCompanyProjectClaManagersOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.CompanyGetCompanyCLAGroupManagersHandler = company.GetCompanyCLAGroupManagersHandlerFunc(
		// No auth - invoked from Contributor Console
		func(params company.GetCompanyCLAGroupManagersParams) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			f := logrus.Fields{
				"functionName":   "CompanyGetCompanyCLAGroupManagersHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"claGroupID":     params.ClaGroupID,
				"companyID":      params.CompanyID,
			}

			result, err := service.GetCompanyCLAGroupManagers(ctx, params.CompanyID, params.ClaGroupID)
			if err != nil {
				msg := "problem loading company CLA group managers"
				log.WithFields(f).WithError(err).Warn(msg)
				if err == v1Company.ErrCompanyDoesNotExist {
					return company.NewGetCompanyCLAGroupManagersNotFound().WithXRequestID(reqID).WithPayload(
						utils.ErrorResponseNotFoundWithError(reqID, msg, err))
				}
				return company.NewGetCompanyCLAGroupManagersBadRequest().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			return company.NewGetCompanyCLAGroupManagersOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.CompanyGetCompanyProjectActiveClaHandler = company.GetCompanyProjectActiveClaHandlerFunc(
		func(params company.GetCompanyProjectActiveClaParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "CompanyGetCompanyProjectActiveClaHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"projectSFID":    params.ProjectSFID,
				"companySFID":    params.CompanySFID,
			}

			log.WithFields(f).Debug("checking permissions")
			if !utils.IsUserAuthorizedForOrganization(authUser, params.CompanySFID, utils.ALLOW_ADMIN_SCOPE) {
				return company.NewGetCompanyProjectActiveClaForbidden().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Code: "403",
					Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to CreateCLAManager with Project|Organization scope of %s | %s",
						authUser.UserName, params.ProjectSFID, params.CompanySFID),
					XRequestID: reqID,
				})
			}

			comp, err := v1CompanyRepo.GetCompanyByExternalID(ctx, params.CompanySFID)
			if err != nil {
				if err == v1Company.ErrCompanyDoesNotExist {
					return company.NewGetCompanyProjectActiveClaNotFound().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
						Code:       "404",
						Message:    fmt.Sprintf("Company not found with given ID. [%s]", params.CompanySFID),
						XRequestID: reqID,
					})
				}
			}
			if comp == nil {
				log.WithFields(f).WithError(err).Warn(msgUnableToLoadCompany)
				return company.NewGetCompanyProjectActiveClaNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFound(reqID, msgUnableToLoadCompany))
			}

			result, err := service.GetCompanyProjectActiveCLAs(ctx, comp.CompanyID, params.ProjectSFID)
			if err != nil {
				if strings.ContainsAny(err.Error(), "getProjectNotFound") {
					return company.NewGetCompanyProjectActiveClaNotFound().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
						Code:       "404",
						Message:    fmt.Sprintf("clagroup not found with given ID. [%s]", params.ProjectSFID),
						XRequestID: reqID,
					})
				}
				return company.NewGetCompanyProjectActiveClaBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return company.NewGetCompanyProjectActiveClaOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.CompanyGetCompanyProjectContributorsHandler = company.GetCompanyProjectContributorsHandlerFunc(
		func(params company.GetCompanyProjectContributorsParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "CompanyGetCompanyProjectContributorsHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"projectSFID":    params.ProjectSFID,
				"companySFID":    params.CompanySFID,
			}

			// PM - check if authorized by project scope - allow if PM has project ID scope that matches
			// Contact,Community Program Manager,CLA Manager,CLA Manager Designee,Company Admin - check if authorized by organization scope - allow if {Contact,Community Program Manager,CLA Manager,CLA Manager Designee,Company Admin} has organization ID scope that matches
			// CLA Manager - check if authorized by project|organization scope - allow if CLA Manager (for example) has project ID + org DI scope that matches
			log.WithFields(f).Debug("checking permissions")
			if !isUserHaveAccessToCLAProjectOrganization(ctx, authUser, params.ProjectSFID, params.CompanySFID, projectClaGroupRepo) {
				return company.NewGetCompanyProjectContributorsForbidden().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseForbidden(
						reqID,
						fmt.Sprintf("user %s does not have access to get contributors with Project scope of %s or Project|Organization scope of %s | %s",
							authUser.UserName, params.ProjectSFID, params.ProjectSFID, params.CompanySFID)))
			}

			result, err := service.GetCompanyProjectContributors(ctx, params.ProjectSFID, params.CompanySFID, utils.StringValue(params.SearchTerm))
			if err != nil {
				if err == v1Company.ErrCompanyDoesNotExist {
					return company.NewGetCompanyProjectContributorsNotFound().WithXRequestID(reqID)
				}
				return company.NewGetCompanyProjectContributorsBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return company.NewGetCompanyProjectContributorsOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.CompanyGetCompanyProjectClaHandler = company.GetCompanyProjectClaHandlerFunc(
		func(params company.GetCompanyProjectClaParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "CompanyGetCompanyProjectClaHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"projectSFID":    params.ProjectSFID,
				"companySFID":    params.CompanySFID,
			}

			log.WithFields(f).Debug("checking permissions")
			if !isUserHaveAccessToCLAProjectOrganization(ctx, authUser, params.ProjectSFID, params.CompanySFID, projectClaGroupRepo) {
				msg := fmt.Sprintf("user %s does not have access to CreateCLAManager with Project|Organization scope of %s | %s", authUser.UserName, params.ProjectSFID, params.CompanySFID)
				log.WithFields(f).Warn(msg)
				return company.NewGetCompanyProjectClaForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			log.WithFields(f).Debug("loading project company CLAs")
			result, err := service.GetCompanyProjectCLA(ctx, authUser, params.CompanySFID, params.ProjectSFID)
			if err != nil {
				msg := "unable to load project company CLAs"
				log.WithFields(f).WithError(err).Warn(msg)
				if err == v1Company.ErrCompanyDoesNotExist {
					return company.NewGetCompanyProjectClaNotFound().WithXRequestID(reqID).WithPayload(
						utils.ErrorResponseNotFoundWithError(reqID, msg, err))
				}
				if err == ErrProjectNotFound {
					return company.NewGetCompanyProjectClaNotFound().WithXRequestID(reqID).WithPayload(
						utils.ErrorResponseNotFoundWithError(reqID, msg, err))
				}
				return company.NewGetCompanyProjectClaBadRequest().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			return company.NewGetCompanyProjectClaOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.CompanyCreateCompanyHandler = company.CreateCompanyHandlerFunc(
		func(params company.CreateCompanyParams) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			f := logrus.Fields{
				"functionName":      "CompanyCreateCompanyHandler",
				utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
				"userID":            params.UserID,
				"companyName":       aws.StringValue(params.Input.CompanyName),
				"companyWebsite":    aws.StringValue(params.Input.CompanyWebsite),
				"signingEntityName": aws.StringValue(params.Input.SigningEntityName),
			}
			// No permissions needed - anyone can create a company

			// Quick validation of the input parameters
			log.WithFields(f).Debug("validating company name...")
			if !utils.ValidCompanyName(*params.Input.CompanyName) {
				msg := "company name is not valid"
				log.WithFields(f).Warn(msg)
				return company.NewCreateCompanyBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
			}

			log.WithFields(f).Debug("creating company...")
			companyModel, err := service.CreateCompany(ctx, *params.Input.CompanyName, *params.Input.SigningEntityName, *params.Input.CompanyWebsite, params.Input.UserEmail.String(), params.UserID)
			if err != nil {
				log.Warnf("error returned from create company api: %+v", err)
				if strings.Contains(err.Error(), "website already exists") {
					formatErr := errors.New("website already exists")
					return company.NewCreateCompanyConflict().WithXRequestID(reqID).WithPayload(errorResponse(reqID, formatErr))
				}
				if _, ok := err.(*organizations.CreateOrgConflict); ok {
					formatErr := errors.New("organization already exists")
					return company.NewCreateCompanyConflict().WithXRequestID(reqID).WithPayload(errorResponse(reqID, formatErr))
				}
				return company.NewCreateCompanyBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return company.NewCreateCompanyOK().WithXRequestID(reqID).WithPayload(companyModel)
		})

	api.CompanyGetCompanyByNameHandler = company.GetCompanyByNameHandlerFunc(
		func(params company.GetCompanyByNameParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			f := logrus.Fields{
				"functionName":   "CompanyGetCompanyByNameHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"companyName":    params.CompanyName,
			}
			// Anyone can query for a company by name

			log.WithFields(f).Debug("loading company by name")
			companyModel, err := service.GetCompanyByName(ctx, params.CompanyName)
			if err != nil {
				msg := fmt.Sprintf("unable to locate company by name: %s", params.CompanyName)
				log.WithFields(f).WithError(err).Warn(msg)
				return company.NewGetCompanyByNameBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			if companyModel == nil {
				msg := fmt.Sprintf("unable to locate company by name: %s", params.CompanyName)
				log.WithFields(f).Warn(msg)
				return company.NewGetCompanyByNameNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}

			return company.NewGetCompanyByNameOK().WithXRequestID(reqID).WithPayload(companyModel)
		})

	api.CompanyDeleteCompanyByIDHandler = company.DeleteCompanyByIDHandlerFunc(
		func(params company.DeleteCompanyByIDParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "CompanyDeleteCompanyByIDHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"companyID":      params.CompanyID,
			}

			// Attempt to locate the company by ID
			log.WithFields(f).Debug("loading company by ID")
			companyModel, getErr := service.GetCompanyByID(ctx, params.CompanyID)
			if getErr != nil {
				msg := "unable to load company by ID"
				log.WithFields(f).WithError(getErr).Warn(msg)
				return company.NewDeleteCompanyByIDBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, getErr))
			}

			// Didn't find the company
			if companyModel == nil {
				msg := fmt.Sprintf("unable to locate company by ID: %s", params.CompanyID)
				log.WithFields(f).Warn(msg)
				return company.NewDeleteCompanyByIDNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}

			// No external ID assigned - unable to check permissions
			if companyModel.CompanyExternalID == "" {
				msg := fmt.Sprintf("company %s does not have an external SFID assigned - unable to validate permissions, company ID: %s",
					companyModel.CompanyName, params.CompanyID)
				log.Warn(msg)
				return company.NewDeleteCompanyByIDBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
			}

			// finally, we can check permissions for the delete operation
			if !utils.IsUserAuthorizedForOrganization(authUser, companyModel.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf(" user %s does not have access to company %s with Organization scope of %s",
					authUser.UserName, companyModel.CompanyName, companyModel.CompanyExternalID)
				log.Warn(msg)
				return company.NewDeleteCompanyByIDForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			err := service.DeleteCompanyByID(ctx, params.CompanyID)
			if err != nil {
				log.Warnf("unable to delete company by ID: %s, error: %+v", params.CompanyID, err)
				return company.NewDeleteCompanyByIDBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}

			return company.NewDeleteCompanyByIDNoContent().WithXRequestID(reqID)
		})

	api.CompanyDeleteCompanyBySFIDHandler = company.DeleteCompanyBySFIDHandlerFunc(
		func(params company.DeleteCompanyBySFIDParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "CompanyDeleteCompanyBySFIDHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"companySFID":    params.CompanySFID,
			}

			// Attempt to locate the company by external SFID
			log.WithFields(f).Debug("loading company by SFID")
			companyModel, getErr := service.GetCompanyBySFID(ctx, params.CompanySFID)
			if getErr != nil {
				msg := fmt.Sprintf("error returned from get company by SFID: %s, error: %+v",
					params.CompanySFID, getErr)
				log.WithFields(f).Warn(msg)
				return company.NewDeleteCompanyBySFIDBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, getErr))
			}

			// Didn't find the company
			if companyModel == nil {
				msg := fmt.Sprintf("unable to locate company by SFID: %s", params.CompanySFID)
				log.WithFields(f).Warn(msg)
				return company.NewDeleteCompanyBySFIDNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}

			// This should never ever happen given we searched by this key, keep it here anyway
			// No external ID assigned - unable to check permissions
			if companyModel.CompanyExternalID == "" {
				msg := fmt.Sprintf("company %s does not have an external SFID assigned - unable to validate permissions, company ID: %s",
					companyModel.CompanyName, params.CompanySFID)
				log.WithFields(f).Warn(msg)
				return company.NewDeleteCompanyBySFIDBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
			}

			// finally, we can check permissions for the delete operation
			log.WithFields(f).Debug("checking permissions")
			if !utils.IsUserAuthorizedForOrganization(authUser, companyModel.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf(" user %s does not have access to company %s with Organization scope of %s",
					authUser.UserName, companyModel.CompanyName, companyModel.CompanyExternalID)
				log.WithFields(f).Warn(msg)
				return company.NewDeleteCompanyBySFIDForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			err := service.DeleteCompanyBySFID(ctx, params.CompanySFID)
			if err != nil {
				msg := "unable to delete company by SFID"
				log.WithFields(f).Warn(msg)
				return company.NewDeleteCompanyBySFIDBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			return company.NewDeleteCompanyBySFIDNoContent().WithXRequestID(reqID)
		})

	api.CompanyContributorAssociationHandler = company.ContributorAssociationHandlerFunc(
		func(params company.ContributorAssociationParams) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			f := logrus.Fields{
				"functionName":   "CompanyContributorAssociationHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"companySFID":    params.CompanySFID,
				"userEmail":      params.Body.UserEmail.String(),
			}

			log.WithFields(f).Debug("associating contributor")
			contributor, contributorErr := service.AssociateContributor(ctx, params.CompanySFID, params.Body.UserEmail.String())
			if contributorErr != nil {
				if _, ok := contributorErr.(*organizations.CreateOrgUsrRoleScopesConflict); ok {
					msg := "user already assigned contributor role for company"
					formatErr := errors.New(msg)
					log.WithFields(f).WithError(formatErr).Warn(msg)
					return company.NewContributorAssociationConflict().WithXRequestID(reqID).WithPayload(errorResponse(reqID, formatErr))
				}
				return company.NewContributorAssociationBadRequest().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, "unable to associate user with company", contributorErr))
			}
			return company.NewContributorAssociationOK().WithXRequestID(reqID).WithPayload(contributor)
		})

	api.CompanyGetCompanyAdminsHandler = company.GetCompanyAdminsHandlerFunc(
		func(params company.GetCompanyAdminsParams) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			f := logrus.Fields{
				"functionName":   "CompanyContributorAssociationHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"companySFID":    params.CompanySFID,
			}

			log.WithFields(f).Debug("loading company admins")
			adminList, adminErr := service.GetCompanyAdmins(ctx, params.CompanySFID)
			if adminErr != nil {
				msg := "unable to load company admins"
				log.WithFields(f).WithError(adminErr).Warn(msg)
				return company.NewGetCompanyAdminsBadRequest().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, adminErr))
			}

			return company.NewGetCompanyAdminsOK().WithXRequestID(reqID).WithPayload(adminList)
		})

	api.CompanyRequestCompanyAdminHandler = company.RequestCompanyAdminHandlerFunc(
		func(params company.RequestCompanyAdminParams) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

			corporateLink := ""
			// Get appropirate corporate link (v1|v2)
			if params.Body.Version == "v1" {
				corporateLink = v1CorporateConsole
			} else if params.Body.Version == "v2" {
				corporateLink = LFXPortalURL
			}

			err := service.RequestCompanyAdmin(ctx, params.UserID, params.Body.ClaManagerEmail.String(), params.Body.ClaManagerName, params.Body.ContributorName, params.Body.ContributorEmail.String(), params.Body.ProjectName, params.Body.CompanyName, corporateLink)
			if err != nil {

				if err == ErrClaGroupNotFound {
					return company.NewRequestCompanyAdminNotFound().WithXRequestID(reqID).WithPayload(
						&models.ErrorResponse{
							Message:    err.Error(),
							Code:       "404",
							XRequestID: reqID,
						})
				}
				return company.NewRequestCompanyAdminBadRequest().WithXRequestID(reqID).WithPayload(
					&models.ErrorResponse{
						Message:    err.Error(),
						Code:       "400",
						XRequestID: reqID,
					})
			}

			// successfully sent invite
			return company.NewRequestCompanyAdminOK().WithXRequestID(reqID)
		})

	api.CompanySearchCompanyLookupHandler = company.SearchCompanyLookupHandlerFunc(func(params company.SearchCompanyLookupParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

		if params.CompanyName == nil && params.WebsiteName == nil {
			log.Debugf("CompanyName or WebsiteName atleast one required")
			return company.NewSearchCompanyLookupBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, errors.New("companyName or websiteName at least one required")))
		}

		companyName, websiteName := validateParams(params)

		result, err := service.GetCompanyLookup(ctx, companyName, websiteName)
		if err != nil {
			msg := fmt.Sprintf("error occured while search orgname %s, websitename %s", companyName, websiteName)
			log.Warnf("error occured while search orgname %s, websitename %s. error = %s", companyName, websiteName, err.Error())
			if _, ok := err.(*organizations.LookupNotFound); ok {
				return company.NewSearchCompanyLookupNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, msg, err))
			}
			return company.NewSearchCompanyLookupBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}
		return company.NewSearchCompanyLookupOK().WithXRequestID(reqID).WithPayload(result)
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

// isUserHaveAccessToCLAProjectOrganization is a helper function to determine if the user has access to the specified project and organization
func isUserHaveAccessToCLAProjectOrganization(ctx context.Context, authUser *auth.User, projectSFID, organizationSFID string, projectClaGroupsRepo projects_cla_groups.Repository) bool {
	f := logrus.Fields{
		"functionName":     "isUserHaveAccessToCLAProjectOrganization",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"projectSFID":      projectSFID,
		"organizationSFID": organizationSFID,
		"userName":         authUser.UserName,
		"userEmail":        authUser.Email,
	}

	log.WithFields(f).Debug("testing if user has access to project SFID...")
	if utils.IsUserAuthorizedForProject(ctx, authUser, projectSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to project SFID...")
		return true
	}

	log.WithFields(f).Debug("testing if user has access to project SFID tree...")
	if utils.IsUserAuthorizedForProjectTree(ctx, authUser, projectSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to project SFID tree...")
		return true
	}

	log.WithFields(f).Debug("testing if user has access to project SFID and organization SFID...")
	if utils.IsUserAuthorizedForProjectOrganization(authUser, projectSFID, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to project SFID and organization SFID...")
		return true
	}

	log.WithFields(f).Debug("testing if user has access to project SFID and organization SFID tree...")
	if utils.IsUserAuthorizedForProjectOrganizationTree(authUser, projectSFID, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to project SFID and organization SFID tree...")
		return true
	}

	log.WithFields(f).Debug("testing if user has access to organization SFID...")
	if utils.IsUserAuthorizedForOrganization(authUser, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to organization SFID...")
		return true
	}

	// No luck so far...let's load up the Project => CLA Group mapping and check to see if the user has access to the
	// other projects or the parent project group/foundation

	log.WithFields(f).Debug("user doesn't have direct access to the project only, project + organization, or organization only - loading CLA Group from project id...")
	projectCLAGroupModel, err := projectClaGroupsRepo.GetClaGroupIDForProject(projectSFID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem loading project -> cla group mapping - returning false")
		return false
	}
	if projectCLAGroupModel == nil {
		log.WithFields(f).WithError(err).Warnf("problem loading project -> cla group mapping - no mapping found - returning false")
		return false
	}

	// Check the foundation permissions
	f["foundationSFID"] = projectCLAGroupModel.FoundationSFID
	log.WithFields(f).Debug("testing if user has access to parent foundation...")
	if utils.IsUserAuthorizedForProject(ctx, authUser, projectCLAGroupModel.FoundationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to parent foundation...")
		return true
	}
	log.WithFields(f).Debug("testing if user has access to parent foundation truee...")
	if utils.IsUserAuthorizedForProjectTree(ctx, authUser, projectCLAGroupModel.FoundationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to parent foundation tree...")
		return true
	}

	log.WithFields(f).Debug("testing if user has access to foundation SFID and organization SFID...")
	if utils.IsUserAuthorizedForProjectOrganization(authUser, projectCLAGroupModel.FoundationSFID, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to foundation SFID and organization SFID...")
		return true
	}

	log.WithFields(f).Debug("testing if user has access to foundation SFID and organization SFID tree...")
	if utils.IsUserAuthorizedForProjectOrganizationTree(authUser, projectCLAGroupModel.FoundationSFID, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to foundation SFID and organization SFID tree...")
		return true
	}

	// Lookup the other project IDs associated with this CLA Group
	log.WithFields(f).Debug("looking up other projects associated with the CLA Group...")
	projectCLAGroupModels, err := projectClaGroupsRepo.GetProjectsIdsForClaGroup(projectCLAGroupModel.ClaGroupID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem loading project cla group mappings by CLA Group ID - returning false")
		return false
	}

	projectSFIDs := getProjectIDsFromModels(f, projectCLAGroupModel.FoundationSFID, projectCLAGroupModels)
	f["projectIDs"] = strings.Join(projectSFIDs, ",")
	log.WithFields(f).Debug("testing if user has access to any cla group project + organization")
	if utils.IsUserAuthorizedForAnyProjectOrganization(authUser, projectSFIDs, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to at least of of the projects...")
		return true
	}

	log.WithFields(f).Debug("exhausted project checks - user does not have access to project")
	return false
}

// getProjectIDsFromModels is a helper function to extract the project SFIDs from the project CLA Group models
func getProjectIDsFromModels(f logrus.Fields, foundationSFID string, projectCLAGroupModels []*projects_cla_groups.ProjectClaGroup) []string {
	// Build a list of projects associated with this CLA Group
	log.WithFields(f).Debug("building list of project IDs associated with the CLA Group...")
	var projectSFIDs []string
	projectSFIDs = append(projectSFIDs, foundationSFID)
	for _, projectCLAGroupModel := range projectCLAGroupModels {
		projectSFIDs = append(projectSFIDs, projectCLAGroupModel.ProjectSFID)
	}
	log.WithFields(f).Debugf("%d projects associated with the CLA Group...", len(projectSFIDs))
	return projectSFIDs
}

func validateParams(params company.SearchCompanyLookupParams) (string, string) {
	var companyName, websiteName string
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

	return companyName, websiteName
}
