// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package docs

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/docs"
)

// Configure setups handlers on api with Service
func Configure(api *operations.EasyclaAPI) {
	api.DocsGetDocHandler = docs.GetDocHandlerFunc(func(params docs.GetDocParams) middleware.Responder {
		return NewGetDocOK()
	})

	api.DocsGetSwaggerHandler = docs.GetSwaggerHandlerFunc(func(params docs.GetSwaggerParams) middleware.Responder {
		return NewGetSwaggerOK()
	})
}
