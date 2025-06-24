// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	eventOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/events"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/go-openapi/runtime/middleware"
)

// Configure setups handlers on api with service
func Configure(api *operations.ClaAPI, service Service) {
	api.EventsSearchEventsHandler = eventOps.SearchEventsHandlerFunc(
		func(params eventOps.SearchEventsParams, claUser *user.CLAUser) middleware.Responder {
			result, err := service.SearchEvents(&params)
			if err != nil {
				log.Debugf("error retrieving events, error: %s", err.Error())
				return eventOps.NewSearchEventsBadRequest().WithPayload(errorResponse(err))
			}
			return eventOps.NewSearchEventsOK().WithPayload(result)
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
