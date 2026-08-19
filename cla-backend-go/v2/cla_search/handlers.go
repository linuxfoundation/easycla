// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_search

import (
	"context"
	"fmt"
	"strings"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	claSearchOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/cla_search"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

const missingUsernameMsg = "the authenticated principal carries no username - unable to search"

// authorized accepts a principal carrying a username, or an admin principal such as a machine token
func authorized(authUser *auth.User) bool {
	return authUser != nil && (authUser.UserName != "" || utils.IsUserAdmin(authUser))
}

// Configure sets up the CLA Group search API handlers
func Configure(api *operations.EasyclaAPI, service Service) {
	api.ClaSearchSearchClaGroupsHandler = claSearchOps.SearchClaGroupsHandlerFunc(
		func(params claSearchOps.SearchClaGroupsParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
			if authUser != nil {
				utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			}
			f := logrus.Fields{
				"functionName":   "v2.cla_search.handlers.SearchClaGroups",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUserName":   utils.StringValue(params.XUSERNAME),
				"searchTerm":     params.SearchTerm,
			}

			if !authorized(authUser) {
				log.WithFields(f).Warn(missingUsernameMsg)
				return claSearchOps.NewSearchClaGroupsUnauthorized().WithXRequestID(reqID).WithPayload(utils.ErrorResponseUnauthorized(reqID, missingUsernameMsg))
			}

			if len(strings.TrimSpace(params.SearchTerm)) < MinSearchTermLength {
				msg := fmt.Sprintf("searchTerm must contain at least %d non-whitespace characters", MinSearchTermLength)
				log.WithFields(f).Warn(msg)
				return claSearchOps.NewSearchClaGroupsBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
			}

			result, err := service.Search(ctx, params.SearchTerm, utils.Int64Value(params.Limit))
			if err != nil {
				msg := "unable to search the CLA Groups for the provided search term"
				log.WithFields(f).WithError(err).Warn(msg)
				return claSearchOps.NewSearchClaGroupsInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			return claSearchOps.NewSearchClaGroupsOK().WithXRequestID(reqID).WithPayload(result)
		})
}
