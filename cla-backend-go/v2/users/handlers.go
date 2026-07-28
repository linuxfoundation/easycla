// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package users

import (
	"context"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime/middleware"
	"github.com/jinzhu/copier"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	v2Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/users"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	v1Users "github.com/linuxfoundation/easycla/cla-backend-go/users"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// Configure wires the v2 users operations. The identity union is owned by the v1 users
// service (GSI-backed); this handler only translates request/response and maps v1→v2 models.
func Configure(api *operations.EasyclaAPI, usersService v1Users.Service) { // nolint
	api.UsersGetUsersByIdentityHandler = users.GetUsersByIdentityHandlerFunc(
		func(params users.GetUsersByIdentityParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			f := logrus.Fields{
				"functionName":   "v2.users.handlers.GetUsersByIdentity",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"lfUsername":     utils.StringValue(params.LfUsername),
				"emailCount":     len(params.Email),
				"githubIDCount":  len(params.GithubID),
			}

			lfUsername := utils.StringValue(params.LfUsername)
			v1Records, err := usersService.GetUsersByIdentity(lfUsername, params.Email, params.GithubID)
			if err != nil {
				msg := "unable to resolve users by identity"
				log.WithFields(f).WithError(err).Warn(msg)
				return users.NewGetUsersByIdentityInternalServerError().WithXRequestID(reqID).
					WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			result := make([]v2Models.User, 0, len(v1Records))
			for _, v1Record := range v1Records {
				var v2Record v2Models.User
				if copyErr := copier.Copy(&v2Record, v1Record); copyErr != nil {
					log.WithFields(f).WithError(copyErr).Warnf("problem converting v1 user model to v2 for user ID: %s", userID(v1Record))
					return users.NewGetUsersByIdentityInternalServerError().WithXRequestID(reqID).
						WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, "unable to convert user record", copyErr))
				}
				result = append(result, v2Record)
			}

			log.WithFields(f).Debugf("resolved %d user record(s) by identity", len(result))
			return users.NewGetUsersByIdentityOK().WithXRequestID(reqID).WithPayload(result)
		})
}

func userID(u *v1Models.User) string {
	if u == nil {
		return ""
	}
	return u.UserID
}
