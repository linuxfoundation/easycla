// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/savaki/dynastore"
)

// Configure setups handlers on api with service
func Configure(api *operations.ClaAPI, service SignatureService, sessionStore *dynastore.Store, eventsService events.Service) { // nolint

	api.SignaturesGetSignedICLADocumentHandler = signatures.GetSignedICLADocumentHandlerFunc(func(params signatures.GetSignedICLADocumentParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

		f := logrus.Fields{
			"functionName":   "v1.signatures.handler.SignaturesGetSignedICLADocumentHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
			"userID":         params.UserID,
		}

		log.WithFields(f).Debug("querying for individual signature...")
		approved, signed := true, true
		signatureModel, sigErr := service.GetIndividualSignature(ctx, params.ClaGroupID, params.UserID, &approved, &signed)
		if sigErr != nil {
			msg := fmt.Sprintf("error retrieving signature using ClaGroupID: %s, userID: %s, error: %+v",
				params.ClaGroupID, params.UserID, sigErr)
			log.WithFields(f).WithError(sigErr).Warn(msg)
			return signatures.NewGetSignedICLADocumentInternalServerError().WithXRequestID(reqID).WithPayload(
				utils.ToV1ErrorResponse(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, sigErr)))
		}

		if signatureModel == nil {
			msg := fmt.Sprintf("error retrieving signature using claGroupID: %s, userID: %s",
				params.ClaGroupID, params.UserID)
			log.WithFields(f).Warn(msg)
			return signatures.NewGetSignedICLADocumentNotFound().WithXRequestID(reqID).WithPayload(
				utils.ToV1ErrorResponse(utils.ErrorResponseNotFound(reqID, msg)))
		}

		downloadURL := fmt.Sprintf("contract-group/%s/icla/%s/%s.pdf",
			params.ClaGroupID, params.UserID, signatureModel.SignatureID)
		log.Debugf("Retrieving PDF from path: %s", downloadURL)
		downloadLink, s3Err := utils.GetDownloadLink(downloadURL)
		if s3Err != nil {
			msg := fmt.Sprintf("unable to locate PDF from source using ClaGroupID: %s, userID: %s, s3 error: %+v",
				params.ClaGroupID, params.UserID, s3Err)
			log.Warn(msg)
			return signatures.NewGetSignedICLADocumentInternalServerError().WithXRequestID(reqID).WithPayload(
				utils.ToV1ErrorResponse(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, sigErr)))
		}

		return middleware.ResponderFunc(func(rw http.ResponseWriter, p runtime.Producer) {
			rw.Header().Set("Content-type", "text/html")
			rw.Header().Set("x-request-id", reqID)
			rw.WriteHeader(200)
			redirectDocument := generateHTMLRedirectPage(downloadLink, "ICLA")
			bytesWritten, writeErr := rw.Write([]byte(redirectDocument))
			if writeErr != nil {
				msg := fmt.Sprintf("EasyCLA - 500 Internal Server Error - generating s3 redirect for the client client using source using claGroupID: %s, userID: %s, error: %+v",
					params.ClaGroupID, params.UserID, s3Err)
				log.Warn(msg)
			}
			log.Debugf("SignaturesGetSignedICLADocumentHandler - wrote %d bytes", bytesWritten)
		})
	})

	api.SignaturesGetSignedCCLADocumentHandler = signatures.GetSignedCCLADocumentHandlerFunc(func(params signatures.GetSignedCCLADocumentParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		f := logrus.Fields{
			"functionName":   "v1.signatures.handler.SignaturesGetSignedCCLADocumentHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
			"companyID":      params.CompanyID,
		}

		approved, signed := true, true
		signatureModel, sigErr := service.GetCorporateSignature(ctx, params.ClaGroupID, params.CompanyID, &approved, &signed)
		if sigErr != nil {
			msg := fmt.Sprintf("EasyCLA - 500 Internal Server Error -  error retrieving signature using ClaGroupID: %s, CompanyID: %s, error: %+v",
				params.ClaGroupID, params.CompanyID, sigErr)
			log.WithFields(f).WithError(sigErr).Warn(msg)
			return signatures.NewGetSignedCCLADocumentInternalServerError().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Code:    "500",
				Message: msg,
			})
		}

		if signatureModel == nil {
			msg := fmt.Sprintf("EasyCLA - 404 Not Found - -  error retrieving signature using ClaGroupID: %s, CompanyID: %s",
				params.ClaGroupID, params.CompanyID)
			log.WithFields(f).Warn(msg)
			return signatures.NewGetSignedCCLADocumentNotFound().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Code:    "404",
				Message: msg,
			})
		}

		downloadURL := fmt.Sprintf("contract-group/%s/ccla/%s/%s.pdf",
			params.ClaGroupID, params.CompanyID, signatureModel.SignatureID)
		log.Debugf("Retrieving PDF from path: %s", downloadURL)
		downloadLink, s3Err := utils.GetDownloadLink(downloadURL)
		if s3Err != nil {
			msg := fmt.Sprintf("EasyCLA - 500 Internal Server Error -  unable to locate PDF from source using ClaGroupID: %s, CompanyID: %s, s3 error: %+v",
				params.ClaGroupID, params.CompanyID, s3Err)
			log.WithFields(f).WithError(s3Err).Warn(msg)
			return signatures.NewGetSignedCCLADocumentInternalServerError().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Code:    "500",
				Message: msg,
			})
		}

		return middleware.ResponderFunc(func(rw http.ResponseWriter, p runtime.Producer) {
			rw.Header().Set("Content-type", "text/html")
			rw.Header().Set("x-request-id", reqID)
			rw.WriteHeader(200)
			redirectDocument := generateHTMLRedirectPage(downloadLink, "CCLA")
			bytesWritten, writeErr := rw.Write([]byte(redirectDocument))
			if writeErr != nil {
				msg := fmt.Sprintf("EasyCLA - 500 Internal Server Error - generating s3 redirect for the client client using source using ClaGroupID: %s, CompanyID: %s, error: %+v",
					params.ClaGroupID, params.CompanyID, s3Err)
				log.WithFields(f).WithError(writeErr).Warn(msg)
			}
			log.WithFields(f).Debugf("wrote %d bytes", bytesWritten)
		})
	})

	// Get Signature
	api.SignaturesGetSignatureHandler = signatures.GetSignatureHandlerFunc(func(params signatures.GetSignatureParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		signature, err := service.GetSignature(ctx, params.SignatureID)
		if err != nil {
			log.Warnf("error retrieving signature metrics, error: %+v", err)
			return signatures.NewGetSignatureBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		if signature == nil {
			return signatures.NewGetSignatureNotFound().WithXRequestID(reqID)
		}

		return signatures.NewGetSignatureOK().WithXRequestID(reqID).WithPayload(signature)
	})

	// Retrieve GitHub Approval List Entries
	api.SignaturesGetGitHubOrgWhitelistHandler = signatures.GetGitHubOrgWhitelistHandlerFunc(func(params signatures.GetGitHubOrgWhitelistParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		session, err := sessionStore.Get(params.HTTPRequest, github.SessionStoreKey)
		if err != nil {
			log.Warnf("error retrieving session from the session store, error: %+v", err)
			return signatures.NewGetGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		githubAccessToken, ok := session.Values["github_access_token"].(string)
		if !ok {
			log.Debugf("no github access token in the session - initializing to empty string")
			githubAccessToken = ""
		}

		ghApprovalList, err := service.GetGithubOrganizationsFromApprovalList(ctx, params.SignatureID, githubAccessToken)
		if err != nil {
			log.Warnf("error fetching github organization approval list entries v using signature_id: %s, error: %+v",
				params.SignatureID, err)
			return signatures.NewGetGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		return signatures.NewGetGitHubOrgWhitelistOK().WithXRequestID(reqID).WithPayload(ghApprovalList)
	})

	// Add GitHub Approval List Entries
	api.SignaturesAddGitHubOrgWhitelistHandler = signatures.AddGitHubOrgWhitelistHandlerFunc(func(params signatures.AddGitHubOrgWhitelistParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		session, err := sessionStore.Get(params.HTTPRequest, github.SessionStoreKey)
		if err != nil {
			log.Warnf("error retrieving session from the session store, error: %+v", err)
			return signatures.NewAddGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		githubAccessToken, ok := session.Values["github_access_token"].(string)
		if !ok {
			log.Debugf("no github access token in the session - initializing to empty string")
			githubAccessToken = ""
		}

		ghApprovalList, err := service.AddGithubOrganizationToApprovalList(ctx, params.SignatureID, params.Body, githubAccessToken)
		if err != nil {
			log.Warnf("error adding github organization %s using signature_id: %s to the whitelist, error: %+v",
				*params.Body.OrganizationID, params.SignatureID, err)
			return signatures.NewAddGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		// Create an event
		signatureModel, getSigErr := service.GetSignature(ctx, params.SignatureID)
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
		eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:  events.ApprovalListGitHubOrganizationAdded,
			ProjectID:  projectID,
			CompanyID:  companyID,
			UserID:     claUser.UserID,
			LfUsername: claUser.LFUsername,
			EventData: &events.ApprovalListGitHubOrganizationAddedEventData{
				GitHubOrganizationName: utils.StringValue(params.Body.OrganizationID),
			},
		})

		return signatures.NewAddGitHubOrgWhitelistOK().WithXRequestID(reqID).WithPayload(ghApprovalList)
	})

	// Delete GitHub Approval List Entries
	api.SignaturesDeleteGitHubOrgWhitelistHandler = signatures.DeleteGitHubOrgWhitelistHandlerFunc(func(params signatures.DeleteGitHubOrgWhitelistParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint

		session, err := sessionStore.Get(params.HTTPRequest, github.SessionStoreKey)
		if err != nil {
			log.Warnf("error retrieving session from the session store, error: %+v", err)
			return signatures.NewDeleteGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		githubAccessToken, ok := session.Values["github_access_token"].(string)
		if !ok {
			log.Debugf("no github access token in the session - initializing to empty string")
			githubAccessToken = ""
		}

		ghApprovalList, err := service.DeleteGithubOrganizationFromApprovalList(ctx, params.SignatureID, params.Body, githubAccessToken)
		if err != nil {
			log.Warnf("error deleting github organization %s using signature_id: %s from the whitelist, error: %+v",
				*params.Body.OrganizationID, params.SignatureID, err)
			return signatures.NewDeleteGitHubOrgWhitelistBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		// Create an event
		signatureModel, getSigErr := service.GetSignature(ctx, params.SignatureID)
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

		eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:  events.ApprovalListGitHubOrganizationDeleted,
			ProjectID:  projectID,
			CompanyID:  companyID,
			UserID:     claUser.UserID,
			LfUsername: claUser.LFUsername,
			EventData: &events.ApprovalListGitHubOrganizationDeletedEventData{
				GitHubOrganizationName: utils.StringValue(params.Body.OrganizationID),
			},
		})

		return signatures.NewDeleteGitHubOrgWhitelistNoContent().WithXRequestID(reqID).WithPayload(ghApprovalList)
	})

	// Get Project Signatures
	api.SignaturesGetProjectSignaturesHandler = signatures.GetProjectSignaturesHandlerFunc(func(params signatures.GetProjectSignaturesParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		projectSignatures, err := service.GetProjectSignatures(ctx, params)
		if err != nil {
			log.Warnf("error retrieving project signatures for projectID: %s, error: %+v",
				params.ProjectID, err)
			return signatures.NewGetProjectSignaturesBadRequest().WithPayload(errorResponse(err))
		}

		return signatures.NewGetProjectSignaturesOK().WithXRequestID(reqID).WithPayload(projectSignatures)
	})

	api.SignaturesCreateProjectSummaryReportHandler = signatures.CreateProjectSummaryReportHandlerFunc(func(params signatures.CreateProjectSummaryReportParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		f := logrus.Fields{
			"functionName":   "signature.handlers.SignaturesCreateProjectSummaryReportHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"projectID":      params.ProjectID,
			"claType":        utils.StringValue(params.ClaType),
			"signatureType":  utils.StringValue(params.SignatureType),
			"nextKey":        utils.StringValue(params.NextKey),
			"searchField":    utils.StringValue(params.SearchField),
			"searchTerm":     utils.StringValue(params.SearchTerm),
			"sortOrder":      utils.StringValue(params.SortOrder),
			"fullMatch":      utils.BoolValue(params.FullMatch),
			"pageSize":       utils.Int64Value(params.PageSize),
		}
		projectSummaryReport, err := service.CreateProjectSummaryReport(ctx, params)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("error creating project summary report for projectID: %s, error: %+v",
				params.ProjectID, err)
			return signatures.NewGetProjectSignaturesBadRequest().WithPayload(errorResponse(err))
		}

		return signatures.NewCreateProjectSummaryReportOK().WithXRequestID(reqID).WithPayload(projectSummaryReport)
	})

	// Get Project Company Signatures
	api.SignaturesGetProjectCompanySignaturesHandler = signatures.GetProjectCompanySignaturesHandlerFunc(func(params signatures.GetProjectCompanySignaturesParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		signed, approved := true, true
		projectSignature, err := service.GetProjectCompanySignature(ctx, params.CompanyID, params.ProjectID, &signed, &approved, params.NextKey, params.PageSize)
		if err != nil {
			log.Warnf("error retrieving project signatures for project: %s, company: %s, error: %+v",
				params.ProjectID, params.CompanyID, err)
			return signatures.NewGetProjectCompanySignaturesBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		count := int64(1)
		if projectSignature == nil {
			count = int64(0)
		}
		response := models.Signatures{
			LastKeyScanned: "",
			ProjectID:      params.ProjectID,
			ResultCount:    count,
			Signatures:     []*models.Signature{projectSignature},
			TotalCount:     count,
		}
		return signatures.NewGetProjectCompanySignaturesOK().WithXRequestID(reqID).WithPayload(&response)
	})

	// Get Employee Project Company Signatures
	api.SignaturesGetProjectCompanyEmployeeSignaturesHandler = signatures.GetProjectCompanyEmployeeSignaturesHandlerFunc(func(params signatures.GetProjectCompanyEmployeeSignaturesParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		projectSignatures, err := service.GetProjectCompanyEmployeeSignatures(ctx, params, nil)
		if err != nil {
			log.Warnf("error retrieving employee project signatures for project: %s, company: %s, error: %+v",
				params.ProjectID, params.CompanyID, err)
			return signatures.NewGetProjectCompanyEmployeeSignaturesBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		return signatures.NewGetProjectCompanyEmployeeSignaturesOK().WithXRequestID(reqID).WithPayload(projectSignatures)
	})

	// Get Company Signatures
	api.SignaturesGetCompanySignaturesHandler = signatures.GetCompanySignaturesHandlerFunc(func(params signatures.GetCompanySignaturesParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		companySignatures, err := service.GetCompanySignatures(ctx, params)
		if err != nil {
			log.Warnf("error retrieving company signatures for companyID: %s, error: %+v", params.CompanyID, err)
			return signatures.NewGetCompanySignaturesBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		return signatures.NewGetCompanySignaturesOK().WithXRequestID(reqID).WithPayload(companySignatures)
	})

	// Get User Signatures
	api.SignaturesGetUserSignaturesHandler = signatures.GetUserSignaturesHandlerFunc(func(params signatures.GetUserSignaturesParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		userSignatures, err := service.GetUserSignatures(ctx, params, nil)
		if err != nil {
			log.Warnf("error retrieving user signatures for userID: %s, error: %+v", params.UserID, err)
			return signatures.NewGetUserSignaturesBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(err))
		}

		return signatures.NewGetUserSignaturesOK().WithXRequestID(reqID).WithPayload(userSignatures)
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

func generateHTMLRedirectPage(downloadLink, claType string) string {
	return fmt.Sprintf(
		`<html lang="en">
							<head>
                               <title>The Linux Foundation – EasyCLA %s PDF Redirect</title>
                               <meta http-equiv="Refresh" content="0; url='%s'"/>
                               <meta charset="utf-8">
                               <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
                               <link rel="shortcut icon" href="https://www.linuxfoundation.org/wp-content/uploads/2017/08/favicon.png">
                               <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/css/bootstrap.min.css" integrity="sha384-Gn5384xqQ1aoWXA+058RXPxPg6fy4IWvTNh0E263XmFcJlSAwiGgFAW/dAiS6JXm" crossorigin="anonymous"/>
                               <script src="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/js/bootstrap.min.js" integrity="sha384-JZR6Spejh4U02d8jOt6vLEHfe/JQGiRRSQQxSfFWpi1MquVdAyjUar5+76PVCmYl" crossorigin="anonymous"></script>
                            </head>
                            <body style='margin-top:20;margin-left:0;margin-right:0;'>
                              <div class="text-center">
                                <img width=300px" src="https://cla-project-logo-prod.s3.amazonaws.com/lf-horizontal-color.svg" alt="community bridge logo"/>
                              </div>
                              <h2 class="text-center">EasyCLA %s PDF Redirect Authorization</h2>
                              <p class="text-center">
                                 <a href="%s" class="btn btn-primary" role="button">Proceed To Download</a>
                              </p>
                              <p class="text-center">Link is only active for 15 minutes. Click on the CLA email to create a new download link.</p>
                            </body>
                        </html>`, claType, downloadLink, claType, downloadLink)
}
