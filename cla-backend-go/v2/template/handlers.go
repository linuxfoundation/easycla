// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package template

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	v1ProjectsCLAGroups "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"

	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	v1Events "github.com/linuxfoundation/easycla/cla-backend-go/events"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/template"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	v1Template "github.com/linuxfoundation/easycla/cla-backend-go/template"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	"github.com/jinzhu/copier"
)

// Configure API call
func Configure(api *operations.EasyclaAPI, service v1Template.ServiceInterface, v1ProjectClaGroupService v1ProjectsCLAGroups.Service, eventsService v1Events.Service) {
	// Retrieve a list of available templates
	api.TemplateGetTemplatesHandler = template.GetTemplatesHandlerFunc(func(params template.GetTemplatesParams, user *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(user, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.template.handlers.TemplateGetTemplatesHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		}

		templates, err := service.GetTemplates(ctx)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem loading templates")
			return template.NewGetTemplatesBadRequest().WithPayload(errorResponse(reqID, err))
		}
		var response []models.Template
		err = copier.Copy(&response, templates)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem converting templates")
			return template.NewGetTemplatesInternalServerError().WithPayload(errorResponse(reqID, err))
		}
		return template.NewGetTemplatesOK().WithPayload(response)
	})

	api.TemplateCreateCLAGroupTemplateHandler = template.CreateCLAGroupTemplateHandlerFunc(func(params template.CreateCLAGroupTemplateParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.template.handlers.TemplateCreateCLAGroupTemplateHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
		}

		projectCLAGroups, lookupErr := v1ProjectClaGroupService.GetProjectsIdsForClaGroup(ctx, params.ClaGroupID)
		if lookupErr != nil || len(projectCLAGroups) == 0 {
			msg := fmt.Sprintf("unable to lookup CLA Group mapping using CLA Group ID: %s", params.ClaGroupID)
			return template.NewGetTemplatesBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, lookupErr))
		}
		projectSFIDs := getProjectSFIDList(projectCLAGroups)

		// Check authorization
		if !utils.IsUserAuthorizedForAnyProjects(ctx, authUser, projectSFIDs, utils.ALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("authUser '%s' does not have access to create CLA Group template with Project scope of any %s",
				authUser.UserName, strings.Join(projectSFIDs, ","))
			log.WithFields(f).Debug(msg)
			return template.NewGetTemplatesForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		input := &v1Models.CreateClaGroupTemplate{}
		err := copier.Copy(input, &params.Body)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem converting templates")
			return template.NewGetTemplatesInternalServerError().WithPayload(errorResponse(reqID, err))
		}

		pdfUrls, err := service.CreateCLAGroupTemplate(ctx, params.ClaGroupID, input)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("Error generating PDFs from provided templates, error: %v", err)
			return template.NewGetTemplatesBadRequest().WithPayload(errorResponse(reqID, err))
		}

		// Need the template name for the event log
		templateName, lookupErr := service.GetTemplateName(ctx, input.TemplateID)
		if lookupErr != nil || templateName == "" {
			log.WithFields(f).WithError(lookupErr).Warnf("Error looking up template name with ID: %s", input.TemplateID)
			return template.NewGetTemplatesBadRequest().WithPayload(errorResponse(reqID, err))
		}

		// Grab the new POC value from the request
		newPOCValue := ""
		for _, field := range input.MetaFields {
			if field.TemplateVariable == "CONTACT_EMAIL" {
				newPOCValue = field.Value
				break
			}
		}

		eventsService.LogEvent(&events.LogEventArgs{
			EventType:         events.CLATemplateCreated,
			ProjectID:         params.ClaGroupID,
			ProjectSFID:       projectCLAGroups[0].ProjectSFID,
			ParentProjectSFID: projectCLAGroups[0].FoundationSFID,
			LfUsername:        authUser.UserName,
			EventData: &events.CLATemplateCreatedEventData{
				TemplateName: templateName,
				NewPOC:       newPOCValue,
			},
		})

		response := &models.TemplatePdfs{}
		err = copier.Copy(response, pdfUrls)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem converting templates")
			return template.NewGetTemplatesInternalServerError().WithPayload(errorResponse(reqID, err))
		}

		return template.NewCreateCLAGroupTemplateOK().WithPayload(response)
	})

	api.TemplateTemplatePreviewHandler = template.TemplatePreviewHandlerFunc(func(params template.TemplatePreviewParams, user *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(user, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.template.handlers.TemplateTemplatePreviewHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"templateFor":    params.TemplateFor,
		}

		var param v1Models.CreateClaGroupTemplate
		err := copier.Copy(&param, &params.TemplatePreviewInput)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem converting templates")
			return writeResponse(http.StatusInternalServerError, runtime.JSONMime, runtime.JSONProducer(), reqID, errorResponse(reqID, err))
		}
		pdf, err := service.CreateTemplatePreview(ctx, &param, params.TemplateFor)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("Error generating PDFs from provided templates, error: %v", err)
			return writeResponse(http.StatusBadRequest, runtime.JSONMime, runtime.JSONProducer(), reqID, errorResponse(reqID, err))
		}
		return middleware.ResponderFunc(func(rw http.ResponseWriter, pr runtime.Producer) {
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write(pdf)
			if err != nil {
				log.Warnf("Error writing pdf, error: %v", err)
			}
		})
	})

	api.TemplateGetCLATemplatePreviewHandler = template.GetCLATemplatePreviewHandlerFunc(func(params template.GetCLATemplatePreviewParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
		f := logrus.Fields{
			"functionName":   "v2.template.handlers.TemplateGetCLATemplatePreviewHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		}
		pdf, err := service.GetCLATemplatePreview(ctx, params.ClaGroupID, params.ClaType, *params.Watermark)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("Error getting PDFs for provided cla group ID : %s, error: %v", params.ClaGroupID, err)
			return writeResponse(http.StatusBadRequest, runtime.JSONMime, runtime.JSONProducer(), reqID, errorResponse(reqID, err))
		}

		return middleware.ResponderFunc(func(rw http.ResponseWriter, pr runtime.Producer) {
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write(pdf)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("Error writing pdf, error: %v", err)
			}
		})
	})
}

// getProjectSFIDList is a helper function to extract the project SFID values from the list of project to CLA group mapping records
func getProjectSFIDList(groups []*v1ProjectsCLAGroups.ProjectClaGroup) []string {
	var response []string
	for _, projectCLAGroup := range groups {
		response = append(response, projectCLAGroup.ProjectSFID)
	}
	return response
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

func writeResponse(httpStatus int, contentType string, contentProducer runtime.Producer, reqID string, data interface{}) middleware.Responder {
	return middleware.ResponderFunc(func(rw http.ResponseWriter, pr runtime.Producer) {
		rw.Header().Set(utils.XREQUESTID, reqID)
		rw.Header().Set(runtime.HeaderContentType, contentType)
		rw.WriteHeader(httpStatus)
		err := contentProducer.Produce(rw, data)
		if err != nil {
			log.Warnf("failed to write data. error = %v", err)
		}
	})
}
