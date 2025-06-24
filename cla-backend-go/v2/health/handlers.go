// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package health

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/jinzhu/copier"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/health"
	v1Health "github.com/linuxfoundation/easycla/cla-backend-go/health"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// Configure setups handlers on api with service
func Configure(api *operations.EasyclaAPI, service v1Health.Service) {

	api.HealthHealthCheckHandler = health.HealthCheckHandlerFunc(func(params health.HealthCheckParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		result, err := service.HealthCheck(params.HTTPRequest.Context())
		if err != nil {
			return health.NewHealthCheckBadRequest().WithPayload(errorResponse(err))
		}
		var response models.Health
		err = copier.Copy(&response, result)
		if err != nil {
			return health.NewHealthCheckBadRequest().WithPayload(errorResponse(err))
		}
		return health.NewHealthCheckOK().WithXRequestID(reqID).WithPayload(&response)
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
