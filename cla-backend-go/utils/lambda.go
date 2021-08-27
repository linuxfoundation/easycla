// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

//GetHTTPOKResponse : return Get HTTP Success Response
func GetHTTPOKResponse(ctx context.Context) events.APIGatewayProxyResponse {
	resp := events.APIGatewayProxyResponse{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            "",
		Headers: map[string]string{
			"Content-Type":           "application/json",
			XREQUESTID:               fmt.Sprintf("%+v", ctx.Value(XREQUESTID)),
			"X-MyCompany-Func-Reply": "hello-handler",
		},
	}
	return resp
}
