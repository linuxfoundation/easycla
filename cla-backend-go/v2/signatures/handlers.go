// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/project/repository"
	"github.com/communitybridge/easycla/cla-backend-go/project/service"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/v2/organization-service/client/organizations" // nolint - lint error for import not used, but it really is

	"github.com/go-openapi/runtime"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/LF-Engineering/lfx-kit/auth"

	"github.com/communitybridge/easycla/cla-backend-go/events"
	v1Signatures "github.com/communitybridge/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/signatures"
	"github.com/communitybridge/easycla/cla-backend-go/github"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	signatureService "github.com/communitybridge/easycla/cla-backend-go/signatures"
	"github.com/go-openapi/runtime/middleware"
	"github.com/jinzhu/copier"
	"github.com/savaki/dynastore"
)

// Configure setups handlers on api with service
func Configure(api *operations.EasyclaAPI, claGroupService service.Service, projectRepo repository.ProjectRepository, companyService company.IService, v1SignatureService signatureService.SignatureService, sessionStore *dynastore.Store, eventsService events.Service, v2service ServiceInterface, projectClaGroupsRepo projects_cla_groups.Repository) { //nolint

	const problemLoadingCLAGroupByID = "problem loading cla group by ID"
	const iclaNotSupportedForCLAGroup = "individual contribution is not supported for this project"
	const cclaNotSupportedForCLAGroup = "corporate contribution is not supported for this project"

	// Get Signature
	api.SignaturesGetSignatureHandler = signatures.GetSignatureHandlerFunc(func(params signatures.GetSignatureParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesGetSignatureHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"signatureID":    params.SignatureID,
		}

		log.WithFields(f).Debug("loading signature...")
		signature, err := v1SignatureService.GetSignature(ctx, params.SignatureID)
		if err != nil {
			msg := "error retrieving signatures by signature ID"
			log.WithFields(f).WithError(err).Warn(msg)
			return signatures.NewGetSignatureBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}
		if signature == nil {
			msg := "signature search by ID not found"
			log.WithFields(f).Warn(msg)
			return signatures.NewGetSignatureNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
		}

		log.WithFields(f).Debug("checking access control permissions for user...")
		if !isUserHaveAccessToCLAGroupProjects(ctx, authUser, signature.ProjectID, projectClaGroupsRepo, projectRepo) {
			msg := fmt.Sprintf("user %s is not authorized to view project ICLA signatures", authUser.UserName)
			log.Warn(msg)
			return signatures.NewGetProjectCompanyEmployeeSignaturesForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}
		log.WithFields(f).Debug("user has access for this query")

		// Convert the signature to a v2 model
		resp, err := v2Signature(signature)
		if err != nil {
			msg := "problem converting v1 signature to v2"
			log.WithFields(f).WithError(err).Warn(msg)
			return signatures.NewGetSignatureBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debug("returning signature to caller...")
		return signatures.NewGetSignatureOK().WithXRequestID(reqID).WithPayload(resp)
	})

	api.SignaturesUpdateApprovalListHandler = signatures.UpdateApprovalListHandlerFunc(func(params signatures.UpdateApprovalListParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesUpdateApprovalListHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
			"projectSFID":    params.ProjectSFID,
			"companyID":      params.CompanyID,
		}

		companyModel, err := companyService.GetCompany(ctx, params.CompanyID)
		if err != nil {
			msg := fmt.Sprintf("User lookup for company by ID: %s failed : %v", params.CompanyID, err)
			log.Warn(msg)
			if _, ok := err.(*utils.CompanyNotFound); ok {
				return signatures.NewUpdateApprovalListBadRequest().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, fmt.Sprintf("company not found - unable to locate company by ID: %s", params.CompanyID), err))
			}
			return signatures.NewUpdateApprovalListBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, fmt.Sprintf("unable to locate company by ID: %s", params.CompanyID), err))
		}

		// Must be in the Project|Organization Scope to see this - signature ACL is double-checked in the service level when the signature is loaded
		if !utils.IsUserAuthorizedForProjectOrganizationTree(ctx, authUser, params.ProjectSFID, companyModel.CompanyExternalID, utils.DISALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("user '%s' does not have access to update Project Company Approval List with Project|Organization scope of %s | %s",
				authUser.UserName, params.ProjectSFID, companyModel.CompanyExternalID)
			log.WithFields(f).Warn(msg)
			return signatures.NewUpdateApprovalListForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		// Valid the payload input - the validator will return a middleware.Responder response/error type
		validationError := validateApprovalListInput(reqID, params)
		if validationError != nil {
			msg := "validation error of the approval list"
			log.WithFields(f).Warn(msg)
			return validationError
		}

		log.WithFields(f).Debug("loading CLA groups by projectSFID")
		projectModels, projectErr := claGroupService.GetCLAGroupsByExternalSFID(ctx, params.ProjectSFID)
		if projectErr != nil || projectModels == nil {
			msg := fmt.Sprintf("unable to locate projects by Project SFID: %s", params.ProjectSFID)
			log.WithFields(f).Warn(msg)
			return signatures.NewUpdateApprovalListNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
		}

		// Lookup the internal project ID when provided the external ID via the v1SignatureService call
		claGroupModel, projErr := claGroupService.GetCLAGroupByID(ctx, params.ClaGroupID)
		if projErr != nil || claGroupModel == nil {
			msg := fmt.Sprintf("unable to locate project by CLA Group ID: %s", params.ClaGroupID)
			log.WithFields(f).Warn(msg)
			return signatures.NewUpdateApprovalListNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
		}

		// Convert the v2 input parameters to a v1 model
		v1ApprovalList := v1Models.ApprovalList{}
		err = copier.Copy(&v1ApprovalList, params.Body)
		if err != nil {
			msg := "unable to convert v1 to v2 approval list"
			log.WithFields(f).Warn(msg)
			return signatures.NewUpdateApprovalListBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		// Invoke the update v1SignatureService function
		updatedSig, updateErr := v1SignatureService.UpdateApprovalList(ctx, authUser, claGroupModel, companyModel, params.ClaGroupID, &v1ApprovalList)
		if updateErr != nil || updatedSig == nil {
			msg := fmt.Sprintf("unable to update signature approval list using CLA Group ID: %s", params.ClaGroupID)
			log.WithFields(f).Warn(msg)
			if _, ok := err.(*signatureService.ForbiddenError); ok {
				return signatures.NewUpdateApprovalListForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbiddenWithError(reqID, msg, updateErr))
			}
			return signatures.NewUpdateApprovalListBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, updateErr))
		}

		// Convert the v1 output model to a v2 response model
		v2Sig := models.Signature{}
		err = copier.Copy(&v2Sig, updatedSig)
		if err != nil {
			msg := "unable to convert v1 to v2 signature"
			log.WithFields(f).Warn(msg)
			return signatures.NewUpdateApprovalListBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debug("returning signature to caller...")
		return signatures.NewUpdateApprovalListOK().WithXRequestID(reqID).WithPayload(&v2Sig)
	})

	// Retrieve GitHub Approval Entries
	api.SignaturesGetGitHubOrgWhitelistHandler = signatures.GetGitHubOrgWhitelistHandlerFunc(func(params signatures.GetGitHubOrgWhitelistParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesGetGitHubOrgWhitelistHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"signatureID":    params.SignatureID,
		}
		session, err := sessionStore.Get(params.HTTPRequest, github.SessionStoreKey)
		if err != nil {
			log.WithFields(f).Warnf("error retrieving session from the session store, error: %+v", err)
			return signatures.NewGetGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		githubAccessToken, ok := session.Values["github_access_token"].(string)
		if !ok {
			log.WithFields(f).Debugf("no github access token in the session - initializing to empty string")
			githubAccessToken = ""
		}

		ghOrgApprovalList, err := v1SignatureService.GetGithubOrganizationsFromApprovalList(ctx, params.SignatureID, githubAccessToken)
		if err != nil {
			log.WithFields(f).Warnf("error fetching github organization approval list entries using signature_id: %s, error: %+v",
				params.SignatureID, err)
			return signatures.NewGetGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		var response []models.GithubOrg
		err = copier.Copy(&response, ghOrgApprovalList)
		if err != nil {
			return signatures.NewGetGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		return signatures.NewGetGitHubOrgWhitelistOK().WithXRequestID(reqID).WithPayload(response)
	})

	// Add GitHub Approval Entries
	api.SignaturesAddGitHubOrgWhitelistHandler = signatures.AddGitHubOrgWhitelistHandlerFunc(func(params signatures.AddGitHubOrgWhitelistParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesAddGitHubOrgWhitelistHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"signatureID":    params.SignatureID,
		}

		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		session, err := sessionStore.Get(params.HTTPRequest, github.SessionStoreKey)
		if err != nil {
			log.WithFields(f).Warnf("error retrieving session from the session store, error: %+v", err)
			return signatures.NewAddGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		githubAccessToken, ok := session.Values["github_access_token"].(string)
		if !ok {
			log.WithFields(f).Debugf("no github access token in the session - initializing to empty string")
			githubAccessToken = ""
		}

		input := v1Models.GhOrgWhitelist{}
		err = copier.Copy(&input, &params.Body)
		if err != nil {
			return signatures.NewAddGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		ghApprovalList, err := v1SignatureService.AddGithubOrganizationToApprovalList(ctx, params.SignatureID, input, githubAccessToken)
		if err != nil {
			log.WithFields(f).Warnf("error adding github organization %s using signature_id: %s to the approval list, error: %+v",
				*params.Body.OrganizationID, params.SignatureID, err)
			return signatures.NewAddGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		// Create an event
		signatureModel, getSigErr := v1SignatureService.GetSignature(ctx, params.SignatureID)
		var projectID = ""
		var companyID = ""
		if getSigErr != nil || signatureModel == nil {
			log.Warnf("error looking up signature using signature_id: %s, error: %+v",
				params.SignatureID, getSigErr)
		}
		if signatureModel != nil {
			projectID = signatureModel.ProjectID
			companyID = signatureModel.SignatureReferenceID
		}

		eventsService.LogEvent(&events.LogEventArgs{
			EventType:  events.ApprovalListGitHubOrganizationAdded,
			ProjectID:  projectID,
			CompanyID:  companyID,
			LfUsername: authUser.UserName,
			EventData: &events.ApprovalListGitHubOrganizationAddedEventData{
				GitHubOrganizationName: utils.StringValue(params.Body.OrganizationID),
			},
		})

		var response []models.GithubOrg
		err = copier.Copy(&response, ghApprovalList)
		if err != nil {
			return signatures.NewAddGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		return signatures.NewAddGitHubOrgWhitelistOK().WithXRequestID(reqID).WithPayload(response)
	})

	// Delete GitHub Approval List Entries
	api.SignaturesDeleteGitHubOrgWhitelistHandler = signatures.DeleteGitHubOrgWhitelistHandlerFunc(func(params signatures.DeleteGitHubOrgWhitelistParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesDeleteGitHubOrgWhitelistHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"signatureID":    params.SignatureID,
		}
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		session, err := sessionStore.Get(params.HTTPRequest, github.SessionStoreKey)
		if err != nil {
			log.WithFields(f).Warnf("error retrieving session from the session store, error: %+v", err)
			return signatures.NewDeleteGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		githubAccessToken, ok := session.Values["github_access_token"].(string)
		if !ok {
			log.WithFields(f).Debugf("no github access token in the session - initializing to empty string")
			githubAccessToken = ""
		}

		input := v1Models.GhOrgWhitelist{}
		err = copier.Copy(&input, &params.Body)
		if err != nil {
			return signatures.NewDeleteGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		ghApprovalList, err := v1SignatureService.DeleteGithubOrganizationFromApprovalList(ctx, params.SignatureID, input, githubAccessToken)
		if err != nil {
			log.WithFields(f).Warnf("error deleting github organization %s using signature_id: %s from the approval list, error: %+v",
				*params.Body.OrganizationID, params.SignatureID, err)
			return signatures.NewDeleteGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		// Create an event
		signatureModel, getSigErr := v1SignatureService.GetSignature(ctx, params.SignatureID)
		var projectID = ""
		var companyID = ""
		if getSigErr != nil || signatureModel == nil {
			log.WithFields(f).Warnf("error looking up signature using signature_id: %s, error: %+v",
				params.SignatureID, getSigErr)
		}
		if signatureModel != nil {
			projectID = signatureModel.ProjectID
			companyID = signatureModel.SignatureReferenceID
		}
		eventsService.LogEvent(&events.LogEventArgs{
			EventType:  events.ApprovalListGitHubOrganizationDeleted,
			ProjectID:  projectID,
			CompanyID:  companyID,
			LfUsername: authUser.UserName,
			EventData: &events.ApprovalListGitHubOrganizationDeletedEventData{
				GitHubOrganizationName: utils.StringValue(params.Body.OrganizationID),
			},
		})
		var response []models.GithubOrg
		err = copier.Copy(&response, ghApprovalList)
		if err != nil {
			return signatures.NewDeleteGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		return signatures.NewDeleteGitHubOrgWhitelistNoContent().WithXRequestID(reqID).WithPayload(response)
	})

	// Get Project Signatures
	api.SignaturesGetProjectSignaturesHandler = signatures.GetProjectSignaturesHandlerFunc(func(params signatures.GetProjectSignaturesParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesGetProjectSignaturesHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
			"signatureType":  params.SignatureType,
		}

		log.WithFields(f).Debug("looking up CLA Group by ID...")
		claGroupModel, err := claGroupService.GetCLAGroupByID(ctx, params.ClaGroupID)
		if err != nil {
			log.WithFields(f).WithError(err).Warn(problemLoadingCLAGroupByID)
			if err == repository.ErrProjectDoesNotExist {
				return signatures.NewGetProjectSignaturesNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, problemLoadingCLAGroupByID, err))
			}
			return signatures.NewGetProjectSignaturesBadRequest().WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, problemLoadingCLAGroupByID, err))
		}
		f["foundationSFID"] = claGroupModel.FoundationSFID

		// Check to see if this CLA Group is configured for ICLAs...
		if params.SignatureType != nil && utils.StringValue(params.ClaType) == utils.ClaTypeICLA && !claGroupModel.ProjectICLAEnabled {
			log.WithFields(f).Warn(iclaNotSupportedForCLAGroup)
			// Return 200 as the retool UI can't handle 400's
			return signatures.NewGetProjectSignaturesOK().WithXRequestID(reqID).WithPayload(&models.Signatures{
				ProjectID:   params.ClaGroupID,
				ResultCount: 0,
				Signatures:  []*models.Signature{}, // empty list
				TotalCount:  0,
			})
		}

		// Check to see if this CLA Group is configured for CCLAs...
		if params.SignatureType != nil && utils.StringValue(params.ClaType) == utils.ClaTypeCCLA && !claGroupModel.ProjectCCLAEnabled {
			log.WithFields(f).Warn(cclaNotSupportedForCLAGroup)
			// Return 200 as the retool UI can't handle 400's
			return signatures.NewGetProjectSignaturesOK().WithXRequestID(reqID).WithPayload(&models.Signatures{
				ProjectID:   params.ClaGroupID,
				ResultCount: 0,
				Signatures:  []*models.Signature{}, // empty list
				TotalCount:  0,
			})
		}

		log.WithFields(f).Debug("checking access control permissions for user...")
		if !isUserHaveAccessToCLAGroupProjects(ctx, authUser, params.ClaGroupID, projectClaGroupsRepo, projectRepo) {
			msg := fmt.Sprintf("user '%s' is not authorized to view project ICLA signatures any scope of project", authUser.UserName)
			log.Warn(msg)
			return signatures.NewGetProjectSignaturesForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}
		log.WithFields(f).Debug("user has access for this query")

		log.WithFields(f).Debug("loading project signatures...")
		projectSignatures, err := v1SignatureService.GetProjectSignatures(ctx, v1Signatures.GetProjectSignaturesParams{
			HTTPRequest:   params.HTTPRequest,
			FullMatch:     params.FullMatch,
			NextKey:       params.NextKey,
			PageSize:      params.PageSize,
			ProjectID:     params.ClaGroupID,
			SearchField:   params.SearchField,
			SearchTerm:    params.SearchTerm,
			SignatureType: params.SignatureType,
			ClaType:       params.ClaType,
			SortOrder:     params.SortOrder,
			Approved:      params.Approved,
			Signed:        params.Signed,
		})
		if err != nil {
			msg := fmt.Sprintf("error retrieving project signatures for projectID: %s, error: %+v",
				params.ClaGroupID, err)
			log.WithFields(f).WithError(err).Warn(msg)
			return signatures.NewGetProjectSignaturesBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		resp, err := v2Signatures(projectSignatures)
		if err != nil {
			msg := "error converting project signatures"
			log.WithFields(f).WithError(err).Warn(msg)
			return signatures.NewGetProjectSignaturesBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debugf("returning %d project signatures", len(resp.Signatures))
		return signatures.NewGetProjectSignaturesOK().WithXRequestID(reqID).WithPayload(resp)
	})

	// Get Project Company Signatures
	api.SignaturesGetProjectCompanySignaturesHandler = signatures.GetProjectCompanySignaturesHandlerFunc(func(params signatures.GetProjectCompanySignaturesParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesGetProjectCompanySignaturesHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"projectSFID":    params.ProjectSFID,
			"companyID":      params.CompanyID,
		}

		companyModel, err := companyService.GetCompany(ctx, params.CompanyID)
		if err != nil {
			msg := fmt.Sprintf("User lookup for company by ID: %s failed : %v", params.CompanyID, err)
			log.Warn(msg)
			if _, ok := err.(*utils.CompanyNotFound); ok {
				return signatures.NewGetProjectCompanySignaturesBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Message: "EasyCLA - 404 Not Found - error getting company - " + msg,
					Code:    "404",
				})
			}
			return signatures.NewGetProjectCompanySignaturesBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Message: "EasyCLA - 400 Bad Request - error getting company - " + msg,
				Code:    "400",
			})
		}

		if !isUserHaveAccessToCLAProjectOrganization(ctx, authUser, params.ProjectSFID, companyModel.CompanyExternalID, projectClaGroupsRepo) {
			msg := fmt.Sprintf("user %s is not authorized to view project company signatures any scope of project: %s, organization %s",
				authUser.UserName, params.ProjectSFID, params.CompanyID)
			log.WithFields(f).Warn(msg)
			return signatures.NewGetProjectCompanySignaturesForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		log.WithFields(f).Debug("loading project company signatures...")
		projectSignatures, err := v2service.GetProjectCompanySignatures(ctx, params.CompanyID, companyModel.CompanyExternalID, params.ProjectSFID)
		if err != nil {
			msg := fmt.Sprintf("error retrieving project signatures for project: %s, company: %s", params.ProjectSFID, params.CompanyID)
			log.WithFields(f).Warn(msg)
			return signatures.NewGetProjectCompanySignaturesBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debugf("returning %d project company signatures", len(projectSignatures.Signatures))
		return signatures.NewGetProjectCompanySignaturesOK().WithXRequestID(reqID).WithPayload(projectSignatures)
	})

	// Get Employee Project Company Signatures
	api.SignaturesGetProjectCompanyEmployeeSignaturesHandler = signatures.GetProjectCompanyEmployeeSignaturesHandlerFunc(func(params signatures.GetProjectCompanyEmployeeSignaturesParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesGetProjectCompanyEmployeeSignaturesHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"projectSFID":    params.ProjectSFID,
			"companyID":      params.CompanyID,
			"nextKey":        aws.StringValue(params.NextKey),
			"pageSize":       aws.Int64Value(params.PageSize),
		}

		companyModel, err := companyService.GetCompany(ctx, params.CompanyID)
		if err != nil {
			msg := fmt.Sprintf("user lookup for company by ID: '%s' failed : %v", params.CompanyID, err)
			log.Warn(msg)
			if _, ok := err.(*utils.CompanyNotFound); ok {
				return signatures.NewGetProjectCompanyEmployeeSignaturesBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
			}
			return signatures.NewGetProjectCompanyEmployeeSignaturesBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}
		if companyModel == nil {
			msg := fmt.Sprintf("problem loading company by ID: %s", params.CompanyID)
			log.WithFields(f).WithError(err).Warn(msg)
			return signatures.NewGetProjectCompanyEmployeeSignaturesBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
		}

		log.WithFields(f).Debug("checking access control permissions...")
		if !isUserHaveAccessToCLAProjectOrganization(ctx, authUser, params.ProjectSFID, companyModel.CompanyExternalID, projectClaGroupsRepo) {
			msg := fmt.Sprintf("user '%s' is not authorized to view project company signatures any scope of project or project|organization for project: '%s', organization '%s'",
				authUser.UserName, params.ProjectSFID, params.CompanyID)
			log.Warn(msg)
			return signatures.NewGetProjectCompanyEmployeeSignaturesForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		// Locate the CLA Group for the provided project SFID
		log.WithFields(f).Debug("loading project signatures...")
		projectCLAGroupModel, err := projectClaGroupsRepo.GetClaGroupIDForProject(ctx, params.ProjectSFID)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("problem loading project -> cla group mapping")
			return signatures.NewGetProjectCompanyEmployeeSignaturesBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(
				reqID, fmt.Sprintf("problem loading project -> cla group mapping using project id: %s", params.ProjectSFID), err))
		}
		if projectCLAGroupModel == nil {
			log.WithFields(f).WithError(err).Warnf("problem loading project -> cla group mapping - no mapping found")
			return signatures.NewGetProjectCompanyEmployeeSignaturesBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(
				reqID, fmt.Sprintf("unable to locate cla group for project ID: %s", params.ProjectSFID)))
		}

		log.WithFields(f).Debug("loading project company signatures...")
		projectSignatures, err := v1SignatureService.GetProjectCompanyEmployeeSignatures(ctx, v1Signatures.GetProjectCompanyEmployeeSignaturesParams{
			HTTPRequest: params.HTTPRequest,
			ProjectID:   projectCLAGroupModel.ClaGroupID, // cla group ID
			CompanyID:   companyModel.CompanyID,          // internal company id
			NextKey:     params.NextKey,
			PageSize:    params.PageSize,
		}, nil)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("error retrieving employee project signatures for project: %s, company: %s, error: %+v",
				params.ProjectSFID, params.CompanyID, err)
			return signatures.NewGetProjectCompanyEmployeeSignaturesBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(
				reqID, fmt.Sprintf("unable to fetch employee signatures for project ID: %s and company: %s", params.ProjectSFID, params.CompanyID), err))
		}

		resp, err := v2Signatures(projectSignatures)
		if err != nil {
			msg := fmt.Sprintf("error converting project company signatures for project: %s, company name: %s, companyID: %s, company external ID: %s",
				params.ProjectSFID, companyModel.CompanyName, companyModel.CompanyID, companyModel.CompanyExternalID)
			log.WithFields(f).WithError(err).Warn(msg)
			return signatures.NewGetProjectCompanyEmployeeSignaturesBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debugf("returning %d employee signatures to caller...", len(resp.Signatures))
		return signatures.NewGetProjectCompanyEmployeeSignaturesOK().WithXRequestID(reqID).WithPayload(resp)
	})

	// Get Company Signatures
	api.SignaturesGetCompanySignaturesHandler = signatures.GetCompanySignaturesHandlerFunc(func(params signatures.GetCompanySignaturesParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesGetCompanySignaturesHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"companyID":      params.CompanyID,
			"companyName":    aws.StringValue(params.CompanyName),
			"signatureType":  aws.StringValue(params.SignatureType),
			"nextKey":        aws.StringValue(params.NextKey),
			"pageSize":       aws.Int64Value(params.PageSize),
		}

		companyModel, err := companyService.GetCompany(ctx, params.CompanyID)
		if err != nil {
			msg := fmt.Sprintf("User lookup for company by ID: %s failed : %v", params.CompanyID, err)
			log.Warn(msg)
			if _, ok := err.(*utils.CompanyNotFound); ok {
				return signatures.NewGetCompanySignaturesBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Message: "EasyCLA - 404 Not Found - error getting company - " + msg,
					Code:    "404",
				})
			}
			return signatures.NewGetCompanySignaturesBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Message: "EasyCLA - 400 Bad Request - error getting company - " + msg,
				Code:    "400",
			})
		}

		if companyModel == nil {
			log.WithFields(f).WithError(err).Warnf("problem loading company model by ID - returning empty response")
			// the LFX UI/Admin console wants 200 empty lists instead of non-200 status back
			return signatures.NewGetCompanySignaturesOK().WithXRequestID(reqID).WithPayload(&models.Signatures{
				Signatures:  []*models.Signature{},
				ResultCount: 0,
				TotalCount:  0,
			})
		}

		if !utils.IsUserAuthorizedForOrganization(ctx, authUser, companyModel.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("%s - user %s is not authorized to view company signatures with Organization scope: %s",
				utils.EasyCLA403Forbidden, authUser.UserName, companyModel.CompanyExternalID)
			log.WithFields(f).Warn(msg)
			return signatures.NewGetProjectCompanySignaturesForbidden().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseForbidden(reqID, msg))
		}

		log.WithFields(f).Debug("loading company signatures...")
		companySignatures, err := v1SignatureService.GetCompanySignatures(ctx, v1Signatures.GetCompanySignaturesParams{
			HTTPRequest:   params.HTTPRequest,
			CompanyID:     companyModel.CompanyID, // need to internal company ID here
			CompanyName:   params.CompanyName,
			NextKey:       params.NextKey,
			PageSize:      params.PageSize,
			SignatureType: params.SignatureType,
		})
		if err != nil {
			msg := fmt.Sprintf("error retrieving company signatures for company name: %s, companyID: %s, company external ID: %s, error: %+v",
				companyModel.CompanyName, companyModel.CompanyID, companyModel.CompanyExternalID, err)
			log.WithFields(f).WithError(err).Warn(msg)
			return signatures.NewGetCompanySignaturesBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		// Nothing in the query response - return an empty model
		if companySignatures == nil || len(companySignatures.Signatures) == 0 {
			return signatures.NewGetCompanySignaturesOK().WithXRequestID(reqID).WithPayload(&models.Signatures{
				Signatures:  []*models.Signature{},
				ResultCount: 0,
				TotalCount:  0,
			})
		}

		log.WithFields(f).Debug("updating company IDs...")
		resp, err := v2SignaturesReplaceCompanyID(companySignatures, companyModel.CompanyID, companyModel.CompanyExternalID)
		if err != nil {
			msg := fmt.Sprintf("error converting company signatures for company name: %s, companyID: %s, company external ID: %s",
				companyModel.CompanyName, companyModel.CompanyID, companyModel.CompanyExternalID)
			log.WithFields(f).WithError(err).Warn(msg)
			return signatures.NewGetCompanySignaturesBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debugf("returning %d company signatures to caller...", len(resp.Signatures))
		return signatures.NewGetCompanySignaturesOK().WithXRequestID(reqID).WithPayload(resp)
	})

	// Get User Signatures
	api.SignaturesGetUserSignaturesHandler = signatures.GetUserSignaturesHandlerFunc(func(params signatures.GetUserSignaturesParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesGetUserSignaturesHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"userID":         params.UserID,
			"userName":       aws.StringValue(params.UserName),
			"nextKey":        aws.StringValue(params.NextKey),
			"pageSize":       aws.Int64Value(params.PageSize),
		}

		userSignatures, err := v1SignatureService.GetUserSignatures(ctx, v1Signatures.GetUserSignaturesParams{
			HTTPRequest: params.HTTPRequest,
			NextKey:     params.NextKey,
			PageSize:    params.PageSize,
			UserName:    params.UserName,
			UserID:      params.UserID,
		})
		if err != nil {
			msg := fmt.Sprintf("error retrieving user signatures for userID: %s", params.UserID)
			log.WithFields(f).WithError(err).Warn(msg)
			return signatures.NewGetUserSignaturesBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		resp, err := v2Signatures(userSignatures)
		if err != nil {
			msg := "problem converting signatures from v1 to v2"
			log.WithFields(f).WithError(err).Warn(msg)
			return signatures.NewGetUserSignaturesBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debugf("returning %d user signatures to caller...", len(resp.Signatures))
		return signatures.NewGetUserSignaturesOK().WithXRequestID(reqID).WithPayload(resp)
	})

	// Download ECLAs as a CSV document
	api.SignaturesDownloadProjectSignatureEmployeeAsCSVHandler = signatures.DownloadProjectSignatureEmployeeAsCSVHandlerFunc(func(params signatures.DownloadProjectSignatureEmployeeAsCSVParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesDownloadProjectSignatureEmployeeAsCSVHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
			"companyID":      params.CompanyID,
		}
		log.WithFields(f).Debug("processing request...")

		log.WithFields(f).Debug("looking up CLA Group by ID...")
		claGroupModel, err := claGroupService.GetCLAGroupByID(ctx, params.ClaGroupID)
		if err != nil {
			log.WithFields(f).WithError(err).Warn(problemLoadingCLAGroupByID)
			if err == repository.ErrProjectDoesNotExist {
				return signatures.NewDownloadProjectSignatureEmployeeAsCSVNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, problemLoadingCLAGroupByID, err))
			}
			return signatures.NewDownloadProjectSignatureEmployeeAsCSVBadRequest().WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, problemLoadingCLAGroupByID, err))
		}
		f["foundationSFID"] = claGroupModel.FoundationSFID

		// Check to see if this CLA Group is configured for ICLAs...
		if !claGroupModel.ProjectCCLAEnabled {
			log.WithFields(f).Warn(cclaNotSupportedForCLAGroup)
			// Return 200 as the retool UI can't handle 400's
			return middleware.ResponderFunc(func(rw http.ResponseWriter, pr runtime.Producer) {
				rw.Header().Set("Content-Type", "text/csv")
				rw.Header().Set(utils.XREQUESTID, reqID)
				rw.WriteHeader(http.StatusOK)
				// Just the header information - no records
				_, writeErr := rw.Write([]byte("GitHub ID,LF_ID,Name,Email,Date Signed"))
				if writeErr != nil {
					log.WithFields(f).WithError(writeErr).Warn("error writing csv file")
				}
			})
			//return signatures.NewDownloadProjectSignatureEmployeeAsCSVBadRequest().WithXRequestID(reqID).WithPayload(
			//	utils.ErrorResponseBadRequest(reqID, cclaNotSupportedForCLAGroup))
		}

		companyModel, err := companyService.GetCompany(ctx, params.CompanyID)
		if err != nil {
			msg := fmt.Sprintf("User lookup for company by ID: %s failed : %v", params.CompanyID, err)
			log.Warn(msg)
			if _, ok := err.(*utils.CompanyNotFound); ok {
				return signatures.NewListClaGroupCorporateContributorsBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
					Message: "EasyCLA - 404 Not Found - error getting company - " + msg,
					Code:    "404",
				})
			}
			return signatures.NewListClaGroupCorporateContributorsBadRequest().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Message: "EasyCLA - 400 Bad Request - error getting company - " + msg,
				Code:    "400",
			})
		}

		// Lookup the Project to CLA Group mapping table entries - this will have the correct details
		projectCLAGroupEntries, projectCLAGroupErr := projectClaGroupsRepo.GetProjectsIdsForClaGroup(ctx, params.ClaGroupID)
		// Should have at least one entry if we're set up correctly - it will have the foundation (parent project/project group) and project details set
		if projectCLAGroupErr != nil || len(projectCLAGroupEntries) == 0 {
			msg := fmt.Sprintf("unable to load project CLA Group mappings for CLA Group: %s - has this project been migrated to v2?", params.ClaGroupID)
			log.WithFields(f).Warn(msg)
			return signatures.NewListClaGroupCorporateContributorsBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
		}
		// All the records will point to the same parent SFID
		f["foundationSFID"] = projectCLAGroupEntries[0].FoundationSFID

		log.WithFields(f).Debug("checking access control permissions for user...")
		if !isUserHaveAccessToCLAProjectOrganization(ctx, authUser, projectCLAGroupEntries[0].FoundationSFID, companyModel.CompanyExternalID, projectClaGroupsRepo) {
			msg := fmt.Sprintf(" user %s is not authorized to view project employee signatures any scope of project", authUser.UserName)
			log.Warn(msg)
			return signatures.NewDownloadProjectSignatureEmployeeAsCSVForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}
		log.WithFields(f).Debug("user has access for this query")

		log.WithFields(f).Debug("searching for corporate contributor signatures...")
		result, err := v2service.GetClaGroupCorporateContributorsCsv(ctx, params.ClaGroupID, params.CompanyID)
		if err != nil {
			msg := fmt.Sprintf("problem getting corporate contributors CSV for CLA Group: %s with company: %s", params.ClaGroupID, companyModel.CompanyExternalID)
			if _, ok := err.(*organizations.GetOrgNotFound); ok {
				formatErr := errors.New("error retrieving company using companySFID")
				return signatures.NewDownloadProjectSignatureEmployeeAsCSVNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, msg, formatErr))
			}
			if ok := err.Error() == "not Found"; ok {
				return signatures.NewDownloadProjectSignatureEmployeeAsCSVNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, msg, err))
			}
			return signatures.NewDownloadProjectSignatureEmployeeAsCSVBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debug("returning CSV response...")
		return middleware.ResponderFunc(func(rw http.ResponseWriter, pr runtime.Producer) {
			rw.Header().Set("Content-Type", "text/csv")
			rw.Header().Set(utils.XREQUESTID, reqID)
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write(result)
			if err != nil {
				log.WithFields(f).Warn("error writing csv file")
			}
		})
	})

	// GET https://api-gw.platform.linuxfoundation.org/v4/cla-group/{claGroupID}/icla/signatures
	api.SignaturesListClaGroupIclaSignatureHandler = signatures.ListClaGroupIclaSignatureHandlerFunc(func(params signatures.ListClaGroupIclaSignatureParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesListClaGroupIclaSignatureHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
			"searchTerm":     utils.StringValue(params.SearchTerm),
			"sortOrder":      utils.StringValue(params.SortOrder),
			"approved":       utils.BoolValue(params.Approved),
			"signed":         utils.BoolValue(params.Signed),
		}

		log.WithFields(f).Debug("looking up CLA Group by ID...")
		claGroupModel, err := claGroupService.GetCLAGroupByID(ctx, params.ClaGroupID)
		if err != nil {
			log.WithFields(f).WithError(err).Warn(problemLoadingCLAGroupByID)
			if err == repository.ErrProjectDoesNotExist {
				return signatures.NewListClaGroupIclaSignatureNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, problemLoadingCLAGroupByID, err))
			}
			return signatures.NewListClaGroupIclaSignatureBadRequest().WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, problemLoadingCLAGroupByID, err))
		}
		f["foundationSFID"] = claGroupModel.FoundationSFID

		// Check to see if this CLA Group is configured for ICLAs...
		if !claGroupModel.ProjectICLAEnabled {
			log.WithFields(f).Warn(iclaNotSupportedForCLAGroup)
			// Return 200 as the retool UI can't handle 400's
			return signatures.NewListClaGroupIclaSignatureOK().WithXRequestID(reqID).WithPayload(&models.IclaSignatures{
				List: []*models.IclaSignature{}, // empty list
			})
			//return signatures.NewListClaGroupIclaSignatureBadRequest().WithXRequestID(reqID).WithPayload(
			//	utils.ErrorResponseBadRequest(reqID, iclaNotSupportedForCLAGroup))
		}

		log.WithFields(f).Debug("checking access control permissions for user...")
		if !isUserHaveAccessToCLAGroupProjects(ctx, authUser, params.ClaGroupID, projectClaGroupsRepo, projectRepo) {
			msg := fmt.Sprintf("user %s is not authorized to view project ICLA signatures any scope of project", authUser.UserName)
			log.Warn(msg)
			return signatures.NewGetProjectCompanyEmployeeSignaturesForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		log.WithFields(f).Debug("searching for ICLA signatures...")

		var pageSize int64
		var nextKey string

		if params.PageSize != nil {
			pageSize = *params.PageSize
		}

		if params.NextKey != nil {
			nextKey = *params.NextKey
		}

		results, err := v2service.GetProjectIclaSignatures(ctx, params.ClaGroupID, params.SearchTerm, params.Approved, params.Signed, pageSize, nextKey)
		if err != nil {
			msg := fmt.Sprintf("problem loading ICLA signatures by CLA Group ID search term: %s", aws.StringValue(params.SearchTerm))
			log.WithFields(f).WithError(err).Warn(msg)
			return signatures.NewListClaGroupIclaSignatureBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debugf("returning %d ICLA signatures to caller...", len(results.List))
		return signatures.NewListClaGroupIclaSignatureOK().WithXRequestID(reqID).WithPayload(results)
	})

	api.SignaturesListClaGroupCorporateContributorsHandler = signatures.ListClaGroupCorporateContributorsHandlerFunc(func(params signatures.ListClaGroupCorporateContributorsParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesListClaGroupCorporateContributorsHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
			"companyID":      params.CompanyID,
		}

		// Lookup the CLA Group by ID - make sure it's valid
		claGroupModel, err := projectRepo.GetCLAGroupByID(ctx, params.ClaGroupID, repository.DontLoadRepoDetails)
		if err != nil {
			log.WithFields(f).WithError(err).Warn(problemLoadingCLAGroupByID)
			if err == repository.ErrProjectDoesNotExist {
				return signatures.NewListClaGroupCorporateContributorsNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, problemLoadingCLAGroupByID, err))
			}

			return signatures.NewListClaGroupCorporateContributorsBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequest(reqID, problemLoadingCLAGroupByID))
		}

		// Make sure the user has provided the companyID
		if params.CompanyID == nil {
			msg := "missing companyID as input"
			log.WithFields(f).Warn(msg)
			return signatures.NewListClaGroupCorporateContributorsBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequest(reqID, msg))
		}

		companyModel, err := companyService.GetCompany(ctx, *params.CompanyID)
		if err != nil {
			msg := fmt.Sprintf("User lookup for company by ID: %s failed : %v", *params.CompanyID, err)
			log.Warn(msg)
			return signatures.NewListClaGroupCorporateContributorsBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		// Make sure CCLA is enabled for this CLA Group
		if !claGroupModel.ProjectCCLAEnabled {
			msg := fmt.Sprintf("CLA Group with ID '%s' does not support corporate contribution", params.ClaGroupID)
			log.WithFields(f).Warn(msg)
			return signatures.NewListClaGroupCorporateContributorsBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, errors.New(msg)))
		}

		// Lookup the Project to CLA Group mapping table entries - this will have the correct details
		projectCLAGroupEntries, projectCLAGroupErr := projectClaGroupsRepo.GetProjectsIdsForClaGroup(ctx, params.ClaGroupID)
		// Should have at least one entry if we're set up correctly - it will have the foundation (parent project/project group) and project details set
		if projectCLAGroupErr != nil || len(projectCLAGroupEntries) == 0 {
			msg := fmt.Sprintf("unable to load project CLA Group mappings for CLA Group: %s - has this project been migrated to v2?", params.ClaGroupID)
			log.WithFields(f).Warn(msg)
			return signatures.NewListClaGroupCorporateContributorsBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
		}
		// All the records will point to the same parent SFID
		f["foundationSFID"] = projectCLAGroupEntries[0].FoundationSFID

		log.WithFields(f).Debug("checking access control permissions for user...")
		if !isUserHaveAccessToCLAProjectOrganization(ctx, authUser, projectCLAGroupEntries[0].FoundationSFID, companyModel.CompanyExternalID, projectClaGroupsRepo) {
			msg := fmt.Sprintf("user '%s' is not authorized to view project CCLA signatures project scope or project|organization scope for company ID: %s",
				authUser.UserName, companyModel.CompanyID)
			log.Warn(msg)
			return signatures.NewListClaGroupCorporateContributorsForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}
		log.WithFields(f).Debug("user has access for this query")

		log.WithFields(f).Debug("searching for CCLA signatures...")
		result, err := v2service.GetClaGroupCorporateContributors(ctx, params.ClaGroupID, *params.CompanyID, params.SearchTerm)
		if err != nil {
			msg := fmt.Sprintf("problem getting corporate contributors for CLA Group: %s with company: %s", params.ClaGroupID, *params.CompanyID)
			if _, ok := err.(*organizations.GetOrgNotFound); ok {
				formatErr := errors.New("error retrieving company using companySFID")
				return signatures.NewListClaGroupCorporateContributorsNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, msg, formatErr))
			}
			return signatures.NewListClaGroupCorporateContributorsInternalServerError().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseInternalServerErrorWithError(reqID, "unexpected error when searching for corporate contributors", err))
		}

		log.WithFields(f).Debugf("returning %d CCLA signatures to caller...", len(result.List))
		return signatures.NewListClaGroupCorporateContributorsOK().WithXRequestID(reqID).WithPayload(result)
	})

	api.SignaturesGetSignatureSignedDocumentHandler = signatures.GetSignatureSignedDocumentHandlerFunc(func(params signatures.GetSignatureSignedDocumentParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesGetSignatureSignedDocumentHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"signatureID":    params.SignatureID,
		}

		log.WithFields(f).Debug("loading signature by ID...")
		signatureModel, err := v1SignatureService.GetSignature(ctx, params.SignatureID)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem loading signature")
			return signatures.NewGetSignatureSignedDocumentBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}
		if signatureModel == nil {
			log.WithFields(f).Warn("problem loading signature - signature not found")
			return signatures.NewGetSignatureSignedDocumentNotFound().WithXRequestID(reqID).WithPayload(errorResponse(reqID, errors.New("signature not found")))
		}

		haveAccess, err := isUserHaveAccessOfSignedSignaturePDF(ctx, authUser, signatureModel, companyService, projectClaGroupsRepo, projectRepo)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem determining signature access")
			return signatures.NewGetSignatureSignedDocumentBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		if !haveAccess {
			return signatures.NewGetSignatureSignedDocumentForbidden().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseForbidden(reqID, fmt.Sprintf("user %s does not have access to the specified signature", authUser.UserName)))
		}

		doc, err := v2service.GetSignedDocument(ctx, signatureModel.SignatureID)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem fetching signed document")
			if strings.Contains(err.Error(), "bad request") {
				return signatures.NewGetSignatureSignedDocumentBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return signatures.NewGetSignatureSignedDocumentInternalServerError().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		log.WithFields(f).Debug("returning signature to caller...")
		return signatures.NewGetSignatureSignedDocumentOK().WithXRequestID(reqID).WithPayload(doc)
	})

	api.SignaturesDownloadProjectSignatureICLAsHandler = signatures.DownloadProjectSignatureICLAsHandlerFunc(func(params signatures.DownloadProjectSignatureICLAsParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesDownloadProjectSignatureICLAsHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
		}

		log.WithFields(f).Debug("loading cla group by id...")
		claGroupModel, err := claGroupService.GetCLAGroupByID(ctx, params.ClaGroupID)
		if err != nil {
			log.WithFields(f).WithError(err).Warn(problemLoadingCLAGroupByID)
			if err == repository.ErrProjectDoesNotExist {
				return signatures.NewDownloadProjectSignatureICLAsNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, problemLoadingCLAGroupByID, err))
			}
			return signatures.NewDownloadProjectSignatureICLAsBadRequest().WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, problemLoadingCLAGroupByID, err))
		}

		if !claGroupModel.ProjectICLAEnabled {
			log.WithFields(f).Warn(iclaNotSupportedForCLAGroup)
			return signatures.NewDownloadProjectSignatureICLAsBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequest(reqID, "icla is not enabled for this cla group"))
		}

		log.WithFields(f).Debug("checking access control permissions for user...")
		if !isUserHaveAccessToCLAGroupProjects(ctx, authUser, params.ClaGroupID, projectClaGroupsRepo, projectRepo) {
			msg := fmt.Sprintf("user %s is not authorized to view project ICLA signatures any scope of project", authUser.UserName)
			log.Warn(msg)
			return signatures.NewDownloadProjectSignatureICLAsForbidden().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseForbidden(reqID, msg))
		}
		log.WithFields(f).Debug("user has access for this query")

		log.WithFields(f).Debug("searching for ICLA signatures...")
		result, err := v2service.GetSignedIclaZipPdf(params.ClaGroupID)
		if err != nil {
			if err == ErrZipNotPresent {
				msg := "no icla signatures found for this cla group"
				log.WithFields(f).Warn(msg)
				return signatures.NewDownloadProjectSignatureICLAsNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, msg, err))
			}
			return signatures.NewDownloadProjectSignatureICLAsBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, "unexpected response from query", err))
		}

		log.WithFields(f).Debug("returning signatures to caller...")
		return signatures.NewDownloadProjectSignatureICLAsOK().WithXRequestID(reqID).WithPayload(result)
	})

	// Download ICLAs as a CSV document
	api.SignaturesDownloadProjectSignatureICLAAsCSVHandler = signatures.DownloadProjectSignatureICLAAsCSVHandlerFunc(func(params signatures.DownloadProjectSignatureICLAAsCSVParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesDownloadProjectSignatureICLAAsCSVHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
		}

		log.WithFields(f).Debug("looking up CLA Group by ID...")
		claGroupModel, err := claGroupService.GetCLAGroupByID(ctx, params.ClaGroupID)
		if err != nil {
			log.WithFields(f).WithError(err).Warn(problemLoadingCLAGroupByID)
			if err == repository.ErrProjectDoesNotExist {
				return signatures.NewDownloadProjectSignatureICLAAsCSVNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, problemLoadingCLAGroupByID, err))
			}
			return signatures.NewDownloadProjectSignatureICLAAsCSVBadRequest().WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, problemLoadingCLAGroupByID, err))
		}
		if !claGroupModel.ProjectICLAEnabled {
			log.WithFields(f).Warn(iclaNotSupportedForCLAGroup)
			// Return 200 as the retool UI can't handle 400's
			return middleware.ResponderFunc(func(rw http.ResponseWriter, pr runtime.Producer) {
				rw.Header().Set("Content-Type", "text/csv")
				rw.Header().Set(utils.XREQUESTID, reqID)
				rw.WriteHeader(http.StatusOK)
				// Just the header information - no records
				_, writeErr := rw.Write([]byte("GitHub ID,LF_ID,Name,Email,Date Signed"))
				if writeErr != nil {
					log.WithFields(f).WithError(writeErr).Warn("error writing csv file")
				}
			})
			//return signatures.NewDownloadProjectSignatureICLAAsCSVBadRequest().WithXRequestID(reqID).WithPayload(
			//	utils.ErrorResponseBadRequest(reqID, iclaNotSupportedForCLAGroup))
		}
		f["foundationSFID"] = claGroupModel.FoundationSFID

		log.WithFields(f).Debug("checking access control permissions for user...")
		if !isUserHaveAccessToCLAGroupProjects(ctx, authUser, params.ClaGroupID, projectClaGroupsRepo, projectRepo) {
			msg := fmt.Sprintf("user '%s' is not authorized to view project ICLA signatures any scope of project", authUser.UserName)
			log.Warn(msg)
			return signatures.NewDownloadProjectSignatureICLAAsCSVForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}
		log.WithFields(f).Debug("user has access for this query")

		log.WithFields(f).Debug("generating ICLA signatures for CSV...")
		result, err := v2service.GetProjectIclaSignaturesCsv(ctx, params.ClaGroupID)
		if err != nil {
			msg := "unable to load ICLA signatures for CSV"
			log.WithFields(f).Warn(msg)
			return signatures.NewDownloadProjectSignatureICLAAsCSVBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debug("returning CSV response...")
		return middleware.ResponderFunc(func(rw http.ResponseWriter, pr runtime.Producer) {
			rw.Header().Set("Content-Type", "text/csv")
			rw.Header().Set(utils.XREQUESTID, reqID)
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write(result)
			if err != nil {
				log.WithFields(f).WithError(err).Warn("error writing csv file")
			}
		})
	})

	api.SignaturesDownloadProjectSignatureCCLAsHandler = signatures.DownloadProjectSignatureCCLAsHandlerFunc(func(params signatures.DownloadProjectSignatureCCLAsParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesDownloadProjectSignatureCCLAsHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
		}

		log.WithFields(f).Debug("looking up CLA Group by ID...")
		claGroupModel, err := claGroupService.GetCLAGroupByID(ctx, params.ClaGroupID)
		if err != nil {
			log.WithFields(f).WithError(err).Warn(problemLoadingCLAGroupByID)
			if err == repository.ErrProjectDoesNotExist {
				return signatures.NewDownloadProjectSignatureCCLAsNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, problemLoadingCLAGroupByID, err))
			}
			return signatures.NewDownloadProjectSignatureCCLAsBadRequest().WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, problemLoadingCLAGroupByID, err))
		}
		if !claGroupModel.ProjectCCLAEnabled {
			log.WithFields(f).Warn(cclaNotSupportedForCLAGroup)
			return signatures.NewDownloadProjectSignatureCCLAsBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequest(reqID, cclaNotSupportedForCLAGroup))
		}
		f["foundationSFID"] = claGroupModel.FoundationSFID

		log.WithFields(f).Debug("checking access control permissions for user...")
		if !isUserHaveAccessToCLAGroupProjects(ctx, authUser, params.ClaGroupID, projectClaGroupsRepo, projectRepo) {
			msg := fmt.Sprintf("user %s is not authorized to view project ICLA signatures any scope of project", authUser.UserName)
			log.Warn(msg)
			return signatures.NewDownloadProjectSignatureCCLAsForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}
		log.WithFields(f).Debug("user has access for this query")

		log.WithFields(f).Debug("searching for CCLA signatures...")
		result, err := v2service.GetSignedCclaZipPdf(params.ClaGroupID)
		if err != nil {
			if err == ErrZipNotPresent {
				msg := "no ccla signatures found for this cla group"
				log.WithFields(f).Warn(msg)
				return signatures.NewDownloadProjectSignatureCCLAsNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, msg, err))
			}
			return signatures.NewDownloadProjectSignatureCCLAsBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, "unexpected response from query", err))
		}

		log.WithFields(f).Debug("returning signatures to caller...")
		return signatures.NewDownloadProjectSignatureCCLAsOK().WithXRequestID(reqID).WithPayload(result)
	})

	// Download CCLAs as a CSV document
	api.SignaturesDownloadProjectSignatureCCLAAsCSVHandler = signatures.DownloadProjectSignatureCCLAAsCSVHandlerFunc(func(params signatures.DownloadProjectSignatureCCLAAsCSVParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesDownloadProjectSignatureCCLAAsCSVHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
		}

		log.WithFields(f).Debug("looking up CLA Group by ID...")
		claGroupModel, err := claGroupService.GetCLAGroupByID(ctx, params.ClaGroupID)
		if err != nil {
			log.WithFields(f).WithError(err).Warn(problemLoadingCLAGroupByID)
			if err == repository.ErrProjectDoesNotExist {
				return signatures.NewDownloadProjectSignatureCCLAAsCSVNotFound().WithXRequestID(reqID).WithPayload(
					utils.ErrorResponseNotFoundWithError(reqID, problemLoadingCLAGroupByID, err))
			}
			return signatures.NewDownloadProjectSignatureCCLAAsCSVBadRequest().WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, problemLoadingCLAGroupByID, err))
		}
		if !claGroupModel.ProjectCCLAEnabled {
			log.WithFields(f).Warn(cclaNotSupportedForCLAGroup)
			// Return 200 as the retool UI can't handle 400's
			return middleware.ResponderFunc(func(rw http.ResponseWriter, pr runtime.Producer) {
				rw.Header().Set("Content-Type", "text/csv")
				rw.Header().Set(utils.XREQUESTID, reqID)
				rw.WriteHeader(http.StatusOK)
				// Just the header information - no records
				_, writeErr := rw.Write([]byte("GitHub ID,LF_ID,Name,Email,Date Signed"))
				if writeErr != nil {
					log.WithFields(f).WithError(writeErr).Warn("error writing csv file")
				}
			})
			//return signatures.NewDownloadProjectSignatureCCLAAsCSVBadRequest().WithXRequestID(reqID).WithPayload(
			//	utils.ErrorResponseBadRequest(reqID, cclaNotSupportedForCLAGroup))
		}
		f["foundationSFID"] = claGroupModel.FoundationSFID

		log.WithFields(f).Debug("checking access control permissions for user...")
		if !isUserHaveAccessToCLAGroupProjects(ctx, authUser, params.ClaGroupID, projectClaGroupsRepo, projectRepo) {
			msg := fmt.Sprintf("user '%s' is not authorized to view project CCLA signatures any scope of project", authUser.UserName)
			log.Warn(msg)
			return signatures.NewDownloadProjectSignatureCCLAAsCSVForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}
		log.WithFields(f).Debug("user has access for this query")

		log.WithFields(f).Debug("generating ICLA signatures for CSV...")
		result, err := v2service.GetProjectCclaSignaturesCsv(ctx, params.ClaGroupID)
		if err != nil {
			msg := "unable to load CCLA signatures for CSV"
			log.WithFields(f).Warn(msg)
			return signatures.NewDownloadProjectSignatureCCLAAsCSVBadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		log.WithFields(f).Debug("returning CSV response...")
		return middleware.ResponderFunc(func(rw http.ResponseWriter, pr runtime.Producer) {
			rw.Header().Set("Content-Type", "text/csv")
			rw.Header().Set(utils.XREQUESTID, reqID)
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write(result)
			if err != nil {
				log.WithFields(f).WithError(err).Warn("error writing csv file")
			}
		})
	})
	api.SignaturesInvalidateICLAHandler = signatures.InvalidateICLAHandlerFunc(func(params signatures.InvalidateICLAParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesInvalidateICLAHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
			"userID":         params.UserID,
		}
		log.WithFields(f).Debug("Invalidating ICLA record...")
		eventArgs := &events.LogEventArgs{
			EventType: events.InvalidatedSignature,
			EventData: &events.SignatureProjectInvalidatedEventData{
				InvalidatedCount: 1,
			},
		}
		err := v2service.InvalidateICLA(ctx, params.ClaGroupID, params.UserID, authUser, eventsService, eventArgs)
		if err != nil {
			msg := "unable to invalidate icla"
			log.WithFields(f).Warn(msg)
			// return signatures.NewInvalidateSignatureBadRequest().WithXRequestID(reqID).WithPayload(
			// 	utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			return signatures.NewInvalidateICLABadRequest().WithXRequestID(reqID).WithPayload(
				utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}
		return signatures.NewInvalidateICLAOK().WithXRequestID(reqID)
	})
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

// isUserHaveAccessOfSignedSignaturePDF returns true if the specified user has access to the provided signature, false otherwise
func isUserHaveAccessOfSignedSignaturePDF(ctx context.Context, authUser *auth.User, signature *v1Models.Signature, companyService company.IService, projectClaGroupRepo projects_cla_groups.Repository, projectRepo repository.ProjectRepository) (bool, error) {
	f := logrus.Fields{
		"functionName":           "v2.signatures.handlers.isUserHaveAccessOfSignedSignaturePDF",
		utils.XREQUESTID:         ctx.Value(utils.XREQUESTID),
		"authUserName":           authUser.UserName,
		"authUserEmail":          authUser.Email,
		"signatureID":            signature.SignatureID,
		"claGroupID":             signature.ProjectID,
		"signatureType":          signature.SignatureType,
		"signatureReferenceType": signature.SignatureReferenceType,
	}
	var projectCLAGroup *v1Models.ClaGroup

	projects, err := projectClaGroupRepo.GetProjectsIdsForClaGroup(ctx, signature.ProjectID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error loading load project IDs for CLA Group")
		return false, err
	}
	if len(projects) == 0 {
		projectCLAGroup, err = projectRepo.GetCLAGroupByID(ctx, signature.ProjectID, false)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("problem loading cla group by ID - failed permission check")
			return false, err
		}
		if projectCLAGroup == nil {
			log.WithFields(f).Debug("cla group is not found using given ID")
			return false, nil
		}

		claData := &projects_cla_groups.ProjectClaGroup{
			ProjectExternalID: projectCLAGroup.ProjectExternalID,
			ProjectSFID:       projectCLAGroup.ProjectExternalID,
			ProjectName:       projectCLAGroup.ProjectName,
			ClaGroupID:        projectCLAGroup.ProjectID,
			ClaGroupName:      projectCLAGroup.ProjectName,
			FoundationSFID:    projectCLAGroup.FoundationSFID,
		}

		projects = append(projects, claData)
	}

	// Foundation ID's should be all the same for each project ID - just grab the first one
	foundationID := projects[0].FoundationSFID
	f["foundationSFID"] = foundationID

	// First, check for PM access
	if utils.IsUserAuthorizedForProjectTree(ctx, authUser, foundationID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debugf("user is authorized for %s scope for foundation ID: %s", utils.ProjectScope, foundationID)
		return true, nil
	}

	// In case the project tree didn't pass, let's check the project list individually - if any has access, we return true
	for _, proj := range projects {
		if utils.IsUserAuthorizedForProject(ctx, authUser, proj.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
			log.WithFields(f).Debugf("user is authorized for %s scope for project ID: %s", utils.ProjectScope, proj.ProjectSFID)
			return true, nil
		}
	}

	// Corporate signature...we can check the company details
	if signature.SignatureType == utils.SignatureTypeCCLA {
		comp, err := companyService.GetCompany(ctx, signature.SignatureReferenceID)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("failed to load company record using signature reference id: %s", signature.SignatureReferenceID)
			return false, err
		}

		// No company SFID? Then, we can't check permissions...
		if comp == nil || comp.CompanyExternalID == "" {
			log.WithFields(f).Warnf("failed to load company record with external SFID using signature reference id: %s", signature.SignatureReferenceID)
			return false, err
		}

		// Check the project|org tree starting with the foundation
		if utils.IsUserAuthorizedForProjectOrganizationTree(ctx, authUser, foundationID, comp.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
			return true, nil
		}

		// In case the project organization tree didn't pass, let's check the project list individually - if any has access, we return true
		for _, proj := range projects {
			if utils.IsUserAuthorizedForProjectOrganization(ctx, authUser, proj.ProjectSFID, comp.CompanyExternalID, utils.ALLOW_ADMIN_SCOPE) {
				log.WithFields(f).Debugf("user is authorized for %s scope for project ID: %s, org iD: %s", utils.ProjectOrgScope, proj.ProjectSFID, comp.CompanyExternalID)
				return true, nil
			}
		}
	}

	log.WithFields(f).Debug("tried everything - user doesn't have access with project or project|org scope")
	return false, nil
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

// isUserHaveAccessToCLAGroupProjects is a helper function to determine if the user has access to the specified CLA Group projects
func isUserHaveAccessToCLAGroupProjects(ctx context.Context, authUser *auth.User, claGroupID string, projectClaGroupsRepo projects_cla_groups.Repository, projectRepo repository.ProjectRepository) bool {
	f := logrus.Fields{
		"functionName":   "v2.signatures.handlers.isUserHaveAccessToCLAGroupProjects",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"userName":       authUser.UserName,
		"userEmail":      authUser.Email,
	}

	var projectCLAGroup *v1Models.ClaGroup

	// Lookup the project IDs for the CLA Group
	log.WithFields(f).Debug("looking up projects associated with the CLA Group...")
	projectCLAGroupModels, err := projectClaGroupsRepo.GetProjectsIdsForClaGroup(ctx, claGroupID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem loading project cla group mappings by CLA Group ID - failed permission check")
		return false
	}
	if len(projectCLAGroupModels) == 0 {
		projectCLAGroup, err = projectRepo.GetCLAGroupByID(ctx, claGroupID, false)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("problem loading cla group by ID - failed permission check")
			return false
		}
		if projectCLAGroup == nil {
			log.WithFields(f).Debug("cla group is not found using given ID")
			return false
		}

		claData := &projects_cla_groups.ProjectClaGroup{
			ProjectExternalID: projectCLAGroup.ProjectExternalID,
			ProjectSFID:       projectCLAGroup.ProjectExternalID,
			ProjectName:       projectCLAGroup.ProjectName,
			ClaGroupID:        projectCLAGroup.ProjectID,
			ClaGroupName:      projectCLAGroup.ProjectName,
			FoundationSFID:    projectCLAGroup.FoundationSFID,
		}

		projectCLAGroupModels = append(projectCLAGroupModels, claData)
	}

	foundationSFID := projectCLAGroupModels[0].FoundationSFID
	f["foundationSFID"] = foundationSFID
	log.WithFields(f).Debug("testing if user has access to parent foundation...")
	if utils.IsUserAuthorizedForProjectTree(ctx, authUser, foundationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to parent foundation tree...")
		return true
	}
	if utils.IsUserAuthorizedForProject(ctx, authUser, foundationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to parent foundation...")
		return true
	}
	log.WithFields(f).Debug("user does not have access to parent foundation...")

	projectSFIDs := getProjectIDsFromModels(f, foundationSFID, projectCLAGroupModels)
	f["projectIDs"] = strings.Join(projectSFIDs, ",")
	log.WithFields(f).Debug("testing if user has access to any projects")
	if utils.IsUserAuthorizedForAnyProjects(ctx, authUser, projectSFIDs, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debug("user has access to at least of of the projects...")
		return true
	}

	log.WithFields(f).Debug("exhausted project checks - user does not have access to project")
	return false
}

// isUserHaveAccessToCLAProjectOrganization is a helper function to determine if the user has access to the specified project and organization
func isUserHaveAccessToCLAProjectOrganization(ctx context.Context, authUser *auth.User, projectSFID, organizationSFID string, projectClaGroupsRepo projects_cla_groups.Repository) bool {
	f := logrus.Fields{
		"functionName":     "v2.signatures.handlers.isUserHaveAccessToCLAProjectOrganization",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"projectSFID":      projectSFID,
		"organizationSFID": organizationSFID,
		"userName":         authUser.UserName,
		"userEmail":        authUser.Email,
	}

	log.WithFields(f).Debugf("testing if user %s/%s has access to project SFID: %s...", authUser.UserName, authUser.Email, projectSFID)
	if utils.IsUserAuthorizedForProject(ctx, authUser, projectSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debugf("user %s/%s has access to project SFID: %s...", authUser.UserName, authUser.Email, projectSFID)
		return true
	}

	log.WithFields(f).Debugf("testing if user %s/%s has access to project SFID tree...", authUser.UserName, authUser.Email)
	if utils.IsUserAuthorizedForProjectTree(ctx, authUser, projectSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debugf("user %s/%s has access to project SFID tree...", authUser.UserName, authUser.Email)
		return true
	}

	log.WithFields(f).Debugf("testing if user %s/%s has access to project SFID and organization SFID...", authUser.UserName, authUser.Email)
	if utils.IsUserAuthorizedForProjectOrganization(ctx, authUser, projectSFID, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debugf("user %s/%s has access to project SFID and organization SFID...", authUser.UserName, authUser.Email)
		return true
	}

	log.WithFields(f).Debugf("testing if user %s/%s has access to project SFID and organization SFID tree...", authUser.UserName, authUser.Email)
	if utils.IsUserAuthorizedForProjectOrganizationTree(ctx, authUser, projectSFID, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debugf("user %s/%s has access to project SFID and organization SFID tree...", authUser.UserName, authUser.Email)
		return true
	}

	log.WithFields(f).Debugf("testing if user %s/%s has access to organization SFID...", authUser.UserName, authUser.Email)
	if utils.IsUserAuthorizedForOrganization(ctx, authUser, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debugf("user %s/%s has access to organization SFID...", authUser.UserName, authUser.Email)
		return true
	}

	// No luck so far...let's load up the Project => CLA Group mapping and check to see if the user has access to the
	// other projects or the parent project group/foundation

	log.WithFields(f).Debugf("user %s/%s doesn't have direct access to the project only, project + organization, or organization only - loading CLA Group from project id...", authUser.UserName, authUser.Email)
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
	log.WithFields(f).Debugf("testing if user %s/%s has access to parent foundation SFID: %s...", authUser.UserName, authUser.Email, projectCLAGroupModel.FoundationSFID)
	if utils.IsUserAuthorizedForProject(ctx, authUser, projectCLAGroupModel.FoundationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debugf("user %s/%s has access to parent foundation SFID: %s...", authUser.UserName, authUser.Email, projectCLAGroupModel.FoundationSFID)
		return true
	}

	log.WithFields(f).Debugf("testing if user %s/%s has access to parent foundation SFID: %s tree...", authUser.UserName, authUser.Email, projectCLAGroupModel.FoundationSFID)
	if utils.IsUserAuthorizedForProjectTree(ctx, authUser, projectCLAGroupModel.FoundationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debugf("user %s/%s has access to parent foundation SFID: %s tree...", authUser.UserName, authUser.Email, projectCLAGroupModel.FoundationSFID)
		return true
	}

	log.WithFields(f).Debugf("testing if user %s/%s has access to foundation SFID %s and organization SFID %s ...", authUser.UserName, authUser.Email, projectCLAGroupModel.FoundationSFID, organizationSFID)
	if utils.IsUserAuthorizedForProjectOrganization(ctx, authUser, projectCLAGroupModel.FoundationSFID, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debugf("user %s/%s has access to foundation SFID %s and organization SFID %s...", authUser.UserName, authUser.Email, projectCLAGroupModel.FoundationSFID, organizationSFID)
		return true
	}

	log.WithFields(f).Debugf("testing if user %s/%s has access to foundation SFID %s and organization SFID %s tree...", authUser.UserName, authUser.Email, projectCLAGroupModel.FoundationSFID, organizationSFID)
	if utils.IsUserAuthorizedForProjectOrganizationTree(ctx, authUser, projectCLAGroupModel.FoundationSFID, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debugf("user %s/%s has access to foundation SFID %s and organization SFID %s tree...", authUser.UserName, authUser.Email, projectCLAGroupModel.FoundationSFID, organizationSFID)
		return true
	}

	// Lookup the other project IDs associated with this CLA Group
	log.WithFields(f).Debug("looking up other projects associated with the CLA Group...")
	projectCLAGroupModels, err := projectClaGroupsRepo.GetProjectsIdsForClaGroup(ctx, projectCLAGroupModel.ClaGroupID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem loading project cla group mappings by CLA Group ID - returning false")
		return false
	}

	// Get the list of the project group and projects associated with this CLA Group
	projectSFIDs := getProjectIDsFromModels(f, projectCLAGroupModel.FoundationSFID, projectCLAGroupModels)
	projectSFIDsCSV := strings.Join(projectSFIDs, ",") // Create a project SFID CSV for printout
	f["projectIDs"] = projectSFIDsCSV

	log.WithFields(f).Debugf("testing if user %s/%s has access to any cla group projects: %s", authUser.UserName, authUser.Email, projectSFIDsCSV)
	if utils.IsUserAuthorizedForAnyProjects(ctx, authUser, projectSFIDs, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debugf("user %s/%s has access to at least of of the projects: %s...", authUser.UserName, authUser.Email, projectSFIDsCSV)
		return true
	}

	log.WithFields(f).Debugf("testing if user %s/%s has access to any cla group projects: %s + organization SFID: %s", authUser.UserName, authUser.Email, projectSFIDsCSV, organizationSFID)
	if utils.IsUserAuthorizedForAnyProjectOrganization(ctx, authUser, projectSFIDs, organizationSFID, utils.ALLOW_ADMIN_SCOPE) {
		log.WithFields(f).Debugf("user %s/%s has access to at least of of the projects: %s + organization SFID: %s...", authUser.UserName, authUser.Email, projectSFIDsCSV, organizationSFID)
		return true
	}

	log.WithFields(f).Debugf("exhausted project checks - user %s/%s does not have access to project", authUser.UserName, authUser.Email)
	return false
}
