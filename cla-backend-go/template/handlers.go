// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package template

import (
	"context"
	"fmt"

	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/template"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// Configure API call
func Configure(api *operations.ClaAPI, service ServiceInterface, eventsService events.Service) {
	// Retrieve a list of available templates
	api.TemplateGetTemplatesHandler = template.GetTemplatesHandlerFunc(func(params template.GetTemplatesParams, claUser *user.CLAUser) middleware.Responder {

		templates, err := service.GetTemplates(params.HTTPRequest.Context())
		if err != nil {
			return template.NewGetTemplatesBadRequest().WithPayload(errorResponse(err))
		}
		return template.NewGetTemplatesOK().WithPayload(templates)
	})

	api.TemplateCreateCLAGroupTemplateHandler = template.CreateCLAGroupTemplateHandlerFunc(func(params template.CreateCLAGroupTemplateParams, claUser *user.CLAUser) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		f := logrus.Fields{
			"functionName":   "v2.signatures.handlers.SignaturesGetProjectSignaturesHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     params.ClaGroupID,
			"templateID":     params.Body.TemplateID,
		}

		pdfUrls, err := service.CreateCLAGroupTemplate(params.HTTPRequest.Context(), params.ClaGroupID, &params.Body)
		if err != nil {
			msg := fmt.Sprintf("Error generating PDFs from provided templates, error: %v", err)
			log.WithFields(f).WithError(err).Warn(msg)
			return template.NewGetTemplatesBadRequest().WithPayload(utils.ToV1ErrorResponse(utils.ErrorResponseBadRequestWithError(reqID, msg, err)))
		}

		// Need the template name for the event log
		templateName, lookupErr := service.GetTemplateName(ctx, params.Body.TemplateID)
		if lookupErr != nil || templateName == "" {
			msg := fmt.Sprintf("Error looking up template name with ID: %s", params.Body.TemplateID)
			log.WithFields(f).WithError(lookupErr).Warn(msg)
			return template.NewGetTemplatesBadRequest().WithPayload(utils.ToV1ErrorResponse(utils.ErrorResponseBadRequestWithError(reqID, msg, lookupErr)))
		}

		// Grab the new POC value from the request
		newPOCValue := ""
		for _, field := range params.Body.MetaFields {
			if field.TemplateVariable == "CONTACT_EMAIL" {
				newPOCValue = field.Value
				break
			}
		}

		eventsService.LogEvent(&events.LogEventArgs{
			EventType: events.CLATemplateCreated,
			ProjectID: params.ClaGroupID,
			UserID:    claUser.UserID,
			EventData: &events.CLATemplateCreatedEventData{
				TemplateName: templateName,
				NewPOC:       newPOCValue,
			},
		})

		return template.NewCreateCLAGroupTemplateOK().WithPayload(&pdfUrls)
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
