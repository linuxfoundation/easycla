// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"errors"
	"fmt"
	"strings"

	organization_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"

	"github.com/aws/aws-sdk-go/aws"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
)

// Configure sets up the middleware handlers
func Configure(api *operations.EasyclaAPI, service Service, projectClaGroupRepo projects_cla_groups.Repository, LFXPortalURL, v1CorporateConsole string) { // nolint

	api.CompanyGetCompanyByInternalIDHandler = company.GetCompanyByInternalIDHandlerFunc(
		func(params company.GetCompanyByInternalIDParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.company.handlers.CompanyGetCompanyByInternalIDHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"companyID":      params.CompanyID,
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}

			// Lookup the company by internal ID
			log.WithFields(f).Debugf("looking up company by internal ID...")
			v2CompanyModel, err := service.GetCompanyByID(ctx, params.CompanyID)
			if err != nil {
				msg := fmt.Sprintf("unable to lookup company by ID: %s", params.CompanyID)
				log.WithFields(f).WithError(err).Warn(msg)
				if _, ok := err.(*utils.CompanyNotFound); ok {
					return company.NewGetCompanyByInternalIDNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFoundWithError(reqID, msg, err))
				}
				return company.NewGetCompanyByInternalIDBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			if v2CompanyModel == nil {
				msg := fmt.Sprintf("unable to lookup company by ID: %s", params.CompanyID)
				log.WithFields(f).WithError(err).Warn(msg)
				return company.NewGetCompanyByInternalIDNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}

			log.WithFields(f).Debug("checking permissions")
			if !utils.IsUserAuthorizedForOrganization(ctx, authUser, v2CompanyModel.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to CompanyGetCompanyByInternalIDHandler with Organization scope of %s",
					authUser.UserName, v2CompanyModel.CompanyExternalID)
				log.WithFields(f).Warn(msg)
				return company.NewGetCompanyByInternalIDForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			return company.NewGetCompanyByInternalIDOK().WithXRequestID(reqID).WithPayload(v2CompanyModel)
		})

	api.CompanyGetCompanyByExternalIDHandler = company.GetCompanyByExternalIDHandlerFunc(
		func(params company.GetCompanyByExternalIDParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.company.handlers.CompanyGetCompanyByExternalIDHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"companySFID":    params.CompanySFID,
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}

			// Lookup the company by internal ID
			log.WithFields(f).Debugf("looking up company by SFID...")
			v2CompanyModel, err := service.GetCompanyBySFID(ctx, params.CompanySFID)
			if err != nil {
				msg := fmt.Sprintf("unable to lookup company by SFID: %s", params.CompanySFID)
				log.WithFields(f).WithError(err).Warn(msg)
				if _, ok := err.(*utils.CompanyNotFound); ok {
					return company.NewGetCompanyByExternalIDNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFoundWithError(reqID, msg, err))
				}
				if _, ok := err.(*organizations.GetOrgNotFound); ok {
					return company.NewGetCompanyByExternalIDNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFoundWithError(reqID, msg, err))
				}
				log.WithFields(f).Debugf("error type is: %T", err)
				return company.NewGetCompanyByExternalIDBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			if v2CompanyModel == nil {
				msg := fmt.Sprintf("unable to lookup company by SFID: %s", params.CompanySFID)
				log.WithFields(f).WithError(err).Warn(msg)
				return company.NewGetCompanyByExternalIDNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}

			log.WithFields(f).Debug("checking permissions")
			if !utils.IsUserAuthorizedForOrganization(ctx, authUser, params.CompanySFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to CompanyGetCompanyByExternalIDHandler with Organization scope of %s",
					authUser.UserName, v2CompanyModel.CompanyExternalID)
				log.WithFields(f).Warn(msg)
				return company.NewGetCompanyByExternalIDForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			return company.NewGetCompanyByExternalIDOK().WithXRequestID(reqID).WithPayload(v2CompanyModel)
		})

	api.CompanyGetCompanyProjectClaManagersHandler = company.GetCompanyProjectClaManagersHandlerFunc(
		func(params company.GetCompanyProjectClaManagersParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.company.handlers.CompanyGetCompanyProjectClaManagersHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"projectSFID":    params.ProjectSFID,
				"companyID":      params.CompanyID,
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}

			// Lookup the company by internal ID
			log.WithFields(f).Debugf("looking up company by internal ID...")
			v2CompanyModel, err := service.GetCompanyByID(ctx, params.CompanyID)
			if err != nil || v2CompanyModel == nil {
				msg := fmt.Sprintf("unable to lookup company by ID: %s", params.CompanyID)
				log.WithFields(f).WithError(err).Warn(msg)
				return company.NewGetCompanyProjectClaManagersBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			log.WithFields(f).Debug("checking permissions")
			if !utils.IsUserAuthorizedForOrganization(ctx, authUser, v2CompanyModel.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to GetCompanyProjectClaManagers with Project|Organization scope of %s | %s",
					authUser.UserName, params.ProjectSFID, v2CompanyModel.CompanyExternalID)
				log.WithFields(f).Warn(msg)
				return company.NewGetCompanyProjectClaManagersForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			result, err := service.GetCompanyProjectCLAManagers(ctx, v2CompanyModel, params.ProjectSFID)
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
				"functionName":   "v2.company.handlers.CompanyGetCompanyCLAGroupManagersHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"claGroupID":     params.ClaGroupID,
				"companyID":      params.CompanyID,
			}

			result, err := service.GetCompanyCLAGroupManagers(ctx, params.CompanyID, params.ClaGroupID)
			if err != nil {
				msg := "problem loading company CLA group managers"
				log.WithFields(f).WithError(err).Warn(msg)
				if _, ok := err.(*utils.CompanyNotFound); ok {
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
				"functionName":   "v2.company.handlers.CompanyGetCompanyProjectActiveClaHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"projectSFID":    params.ProjectSFID,
				"companyID":      params.CompanyID,
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}

			// Lookup the company by internal ID
			log.WithFields(f).Debugf("looking up company by internal ID...")
			v2CompanyModel, err := service.GetCompanyByID(ctx, params.CompanyID)
			if err != nil || v2CompanyModel == nil {
				msg := fmt.Sprintf("unable to lookup company by ID: %s", params.CompanyID)
				log.WithFields(f).WithError(err).Warn(msg)
				return company.NewGetCompanyProjectActiveClaBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			log.WithFields(f).Debug("checking permissions")
			if !utils.IsUserAuthorizedForOrganization(ctx, authUser, v2CompanyModel.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to GetCompanyProjectActiveCla with Project|Organization scope of %s | %s",
					authUser.UserName, params.ProjectSFID, v2CompanyModel.CompanyExternalID)
				log.WithFields(f).Warn(msg)
				return company.NewGetCompanyProjectActiveClaForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			log.WithFields(f).Debug("getting company project active CLAs...")
			result, err := service.GetCompanyProjectActiveCLAs(ctx, v2CompanyModel.CompanyID, params.ProjectSFID)
			if err != nil {
				if strings.ContainsAny(err.Error(), "getProjectNotFound") {
					msg := fmt.Sprintf("CLA Group not found with given project SFID: %s", params.ProjectSFID)
					log.WithFields(f).Warn(msg)
					return company.NewGetCompanyProjectActiveClaNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbiddenWithError(reqID, msg, err))
				}

				msg := fmt.Sprintf("error looking up active project CLAs by internal company ID: %s and project SFID: %s", v2CompanyModel.CompanyID, params.ProjectSFID)
				log.WithFields(f).Warn(msg)
				return company.NewGetCompanyProjectActiveClaBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			return company.NewGetCompanyProjectActiveClaOK().WithXRequestID(reqID).WithPayload(result)
		})

	api.CompanyGetCompanyProjectContributorsHandler = company.GetCompanyProjectContributorsHandlerFunc(
		func(params company.GetCompanyProjectContributorsParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.company.handlers.CompanyGetCompanyProjectContributorsHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"projectSFID":    params.ProjectSFID,
				"companyID":      params.CompanyID,
				"searchTerm":     utils.StringValue(params.SearchTerm),
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}

			// Lookup the company by internal ID
			log.WithFields(f).Debugf("looking up company by internal ID...")
			v1CompanyModel, err := service.GetCompanyByID(ctx, params.CompanyID)
			if err != nil || v1CompanyModel == nil {
				msg := fmt.Sprintf("unable to lookup company by ID: %s", params.CompanyID)
				log.WithFields(f).WithError(err).Warn(msg)
				if _, ok := err.(*utils.CompanyNotFound); ok {
					return company.NewGetCompanyProjectActiveClaNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFoundWithError(reqID, msg, err))
				}
				return company.NewGetCompanyProjectActiveClaBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}
			log.WithFields(f).Debugf("looked company by internal ID")

			// PM - check if authorized by project scope - allow if PM has project ID scope that matches
			// Contact,Community Program Manager,CLA Manager,CLA Manager Designee,Company Admin - check if authorized by organization scope - allow if {Contact,Community Program Manager,CLA Manager,CLA Manager Designee,Company Admin} has organization ID scope that matches
			// CLA Manager - check if authorized by project|organization scope - allow if CLA Manager (for example) has project ID + org DI scope that matches
			log.WithFields(f).Debug("checking permissions")
			if !isUserHaveAccessToCLAProjectOrganization(ctx, authUser, params.ProjectSFID, v1CompanyModel.CompanyExternalID, projectClaGroupRepo) {
				msg := fmt.Sprintf("user %s does not have access to get contributors with Project scope of %s or Project|Organization scope of %s | %s",
					authUser.UserName, params.ProjectSFID, params.ProjectSFID, params.CompanyID)
				log.WithFields(f).Warn(msg)
				return company.NewGetCompanyProjectContributorsForbidden().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseForbidden(reqID, msg))
			}

			log.WithFields(f).Debugf("querying for employee contributors...")
			//result, err := service.GetCompanyProjectContributors(ctx, params.ProjectSFID, params.CompanyID, utils.StringValue(params.SearchTerm))
			result, err := service.GetCompanyProjectContributors(ctx, &params)
			if err != nil {
				if companyErr, ok := err.(*utils.CompanyNotFound); ok {
					msg := fmt.Sprintf("Company not found with ID: %s", companyErr.CompanyID)
					log.WithFields(f).Warn(msg)
					return company.NewGetCompanyProjectContributorsNotFound().WithXRequestID(reqID).WithPayload(
						utils.ErrorResponseNotFoundWithError(reqID, msg, err))
				}
				if claGroupErr, ok := err.(*utils.CLAGroupNotFound); ok {
					msg := fmt.Sprintf("CLA Group not found with ID: %s", claGroupErr.CLAGroupID)
					log.WithFields(f).Warn(msg)
					return company.NewGetCompanyProjectContributorsNotFound().WithXRequestID(reqID).WithPayload(
						utils.ErrorResponseNotFoundWithError(reqID, msg, err))
				}
				if _, ok := err.(*utils.ProjectCLAGroupMappingNotFound); ok {
					msg := fmt.Sprintf("CLA Group not found with project SFID: %s", params.ProjectSFID)
					log.WithFields(f).Warn(msg)
					return company.NewGetCompanyProjectContributorsNotFound().WithXRequestID(reqID).WithPayload(
						utils.ErrorResponseNotFoundWithError(reqID, msg, err))
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
				"functionName":   "v2.company.handlers.CompanyGetCompanyProjectClaHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"projectSFID":    params.ProjectSFID,
				"companySFID":    params.CompanySFID,
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
			}

			log.WithFields(f).Debug("checking permissions")
			if !isUserHaveAccessToCLAProjectOrganization(ctx, authUser, params.ProjectSFID, params.CompanySFID, projectClaGroupRepo) {
				msg := fmt.Sprintf("user %s does not have access to CreateCLAManager with Project|Organization scope of %s | %s", authUser.UserName, params.ProjectSFID, params.CompanySFID)
				log.WithFields(f).Warn(msg)
				return company.NewGetCompanyProjectClaForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			log.WithFields(f).Debug("loading project company CLAs")
			result, err := service.GetCompanyProjectCLA(ctx, authUser, params.CompanySFID, params.ProjectSFID, params.CompanyID)
			if err != nil {
				msg := "unable to load project company CLAs"
				log.WithFields(f).WithError(err).Warn(msg)
				if _, ok := err.(*utils.CompanyNotFound); ok {
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
				"functionName":      "v2.company.handlers.CompanyCreateCompanyHandler",
				utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
				"userID":            params.UserID,
				"companyName":       aws.StringValue(params.Input.CompanyName),
				"companyWebsite":    aws.StringValue(params.Input.CompanyWebsite),
				"signingEntityName": params.Input.SigningEntityName,
				"userEmail":         params.Input.UserEmail.String(),
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
			companyModel, err := service.CreateCompany(ctx, &params)
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
				"functionName":   "v2.company.handlers.CompanyGetCompanyByNameHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"companyName":    params.CompanyName,
			}

			// Anyone can query for a company by name - no permissions checks

			// Weird - sometimes the UI calls us with the company name of "null"
			if params.CompanyName == "" || params.CompanyName == "null" {
				return company.NewGetCompanyByNameBadRequest().
					WithXRequestID(reqID).
					WithPayload(utils.ErrorResponseBadRequest(reqID, "company name input parameter missing or valid"))
			}

			log.WithFields(f).Debugf("loading company by name: '%s'", params.CompanyName)
			companyModel, err := service.GetCompanyByName(ctx, params.CompanyName)
			if err != nil || companyModel == nil {
				log.WithFields(f).Warnf("unable to lookup company by name '%s' in local database. trying organization service...", params.CompanyName)
				osClient := organization_service.GetClient()
				orgModels, orgLookupErr := osClient.SearchOrganization(ctx, params.CompanyName, "", "")
				if orgLookupErr != nil || len(orgModels) == 0 {
					msg := fmt.Sprintf("unable to locate organization '%s' in the organization service", params.CompanyName)
					log.WithFields(f).WithError(err).Warn(msg)
					return company.NewGetCompanyByNameNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
				}

				log.WithFields(f).Debugf("found company: '%s' in the organization service - creating local record...", params.CompanyName)
				companyModelOutput, companyCreateErr := service.CreateCompanyFromSFModel(ctx, orgModels[0], authUser)
				if companyCreateErr != nil || companyModelOutput == nil {
					msg := fmt.Sprintf("unable to create company '%s' from salesforce record", params.CompanyName)
					log.WithFields(f).WithError(err).Warn(msg)
					return company.NewGetCompanyByNameInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, companyCreateErr))
				}

				// Note: company name may have been swapped with actual value from SF or Clearbit authority - so use it below...

				log.WithFields(f).Debugf("loading company: %s by name after creation...", companyModelOutput.CompanyName)
				companyModel, err = service.GetCompanyByName(ctx, companyModelOutput.CompanyName)
				if err != nil {
					msg := fmt.Sprintf("unable to locate company '%s' after creating...", companyModelOutput.CompanyName)
					log.WithFields(f).WithError(err).Warn(msg)
					return company.NewGetCompanyByNameNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
				}
			}

			if companyModel == nil {
				msg := fmt.Sprintf("unable to load company by name: %s", params.CompanyName)
				log.WithFields(f).Warn(msg)
				return company.NewGetCompanyByNameNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}

			return company.NewGetCompanyByNameOK().WithXRequestID(reqID).WithPayload(companyModel)
		})

	api.CompanyGetCompanyBySigningEntityNameHandler = company.GetCompanyBySigningEntityNameHandlerFunc(
		func(params company.GetCompanyBySigningEntityNameParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":      "v2.company.handlers.CompanyGetCompanyByNameHandler",
				utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
				"signingEntityName": params.SigningEntityName,
			}

			// Anyone can query for a company by signing entity name

			log.WithFields(f).Debug("loading company by name")
			companyModel, err := service.GetCompanyBySigningEntityName(ctx, params.SigningEntityName)
			if err != nil {
				msg := fmt.Sprintf("unable to locate company by signing entity name: %s", params.SigningEntityName)
				log.WithFields(f).WithError(err).Warn(msg)
				return company.NewGetCompanyBySigningEntityNameBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			if companyModel == nil {
				msg := fmt.Sprintf("unable to locate company by signing entity name: %s", params.SigningEntityName)
				log.WithFields(f).Warn(msg)
				return company.NewGetCompanyBySigningEntityNameNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}

			return company.NewGetCompanyBySigningEntityNameOK().WithXRequestID(reqID).WithPayload(companyModel)
		})

	api.CompanyDeleteCompanyByIDHandler = company.DeleteCompanyByIDHandlerFunc(
		func(params company.DeleteCompanyByIDParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.company.handlers.CompanyDeleteCompanyByIDHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"companyID":      params.CompanyID,
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
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
			if !utils.IsUserAuthorizedForOrganization(ctx, authUser, companyModel.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
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
				"functionName":   "v2.company.handlers.CompanyDeleteCompanyBySFIDHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"companySFID":    params.CompanySFID,
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"authUserEmail":  utils.StringValue(params.XEMAIL),
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
			if !utils.IsUserAuthorizedForOrganization(ctx, authUser, companyModel.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
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
				"functionName":   "v2.company.handlers.CompanyContributorAssociationHandler",
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
				"functionName":   "v2.company.handlers.CompanyContributorAssociationHandler",
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
		f := logrus.Fields{
			"functionName":   "v2.company.handlers.CompanyGetCompanyByInternalIDHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"companyName":    params.CompanyName,
			"websiteName":    params.WebsiteName,
		}

		if params.CompanyName == nil && params.WebsiteName == nil {
			log.WithFields(f).Debugf("CompanyName or WebsiteName at least one required")
			return company.NewSearchCompanyLookupBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, errors.New("companyName or websiteName at least one required")))
		}

		companyName, websiteName := validateParams(params)

		result, err := service.GetCompanyLookup(ctx, companyName, websiteName)
		if err != nil {
			msg := fmt.Sprintf("error occured while search orgname %s, websitename %s", companyName, websiteName)
			log.WithFields(f).WithError(err).Warnf("error occured while search orgname %s, websitename %s. error = %s", companyName, websiteName, err.Error())
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
		"functionName":     "v2.company.handlers.isUserHaveAccessToCLAProjectOrganization",
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
	if utils.IsUserAuthorizedForProjectOrganization(ctx, authUser, projectSFID, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to project SFID and organization SFID...")
		return true
	}

	log.WithFields(f).Debug("testing if user has access to project SFID and organization SFID tree...")
	if utils.IsUserAuthorizedForProjectOrganizationTree(ctx, authUser, projectSFID, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to project SFID and organization SFID tree...")
		return true
	}

	log.WithFields(f).Debug("testing if user has access to organization SFID...")
	if utils.IsUserAuthorizedForOrganization(ctx, authUser, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to organization SFID...")
		return true
	}

	// No luck so far...let's load up the Project => CLA Group mapping and check to see if the user has access to the
	// other projects or the parent project group/foundation

	log.WithFields(f).Debug("user doesn't have direct access to the project only, project + organization, or organization only - loading CLA Group from project id...")
	projectCLAGroupModel, err := projectClaGroupsRepo.GetClaGroupIDForProject(ctx, projectSFID)
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
	if utils.IsUserAuthorizedForProjectOrganization(ctx, authUser, projectCLAGroupModel.FoundationSFID, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to foundation SFID and organization SFID...")
		return true
	}

	log.WithFields(f).Debug("testing if user has access to foundation SFID and organization SFID tree...")
	if utils.IsUserAuthorizedForProjectOrganizationTree(ctx, authUser, projectCLAGroupModel.FoundationSFID, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to foundation SFID and organization SFID tree...")
		return true
	}

	// Lookup the other project IDs associated with this CLA Group
	log.WithFields(f).Debug("looking up other projects associated with the CLA Group...")
	projectCLAGroupModels, err := projectClaGroupsRepo.GetProjectsIdsForClaGroup(ctx, projectCLAGroupModel.ClaGroupID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem loading project cla group mappings by CLA Group ID - returning false")
		return false
	}

	projectSFIDs := getProjectIDsFromModels(f, projectCLAGroupModel.FoundationSFID, projectCLAGroupModels)
	f["projectIDs"] = strings.Join(projectSFIDs, ",")
	log.WithFields(f).Debug("testing if user has access to any cla group project + organization")
	if utils.IsUserAuthorizedForAnyProjectOrganization(ctx, authUser, projectSFIDs, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
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
