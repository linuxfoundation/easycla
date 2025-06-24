// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_activity

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	"github.com/google/go-github/v37/github" // with go modules enabled (GO111MODULE=on or outside GOPATH)0:w

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/github_activity"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
	"github.com/gofrs/uuid"
)

// signatureCheckMiddleware is used to get access to raw http request so can do the
// signature validation properly
func signatureCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := github.ValidatePayload(r, nil)
		if err != nil {
			http.Error(w, "signature check failure", 401)
			return
		}
		defer r.Body.Close()
		r.Body = ioutil.NopCloser(bytes.NewBuffer(payload))
		// call the next middleware
		next.ServeHTTP(w, r)
	})
}

// Configure setups handlers on api with service
func Configure(api *operations.EasyclaAPI, service Service) {
	api.GithubActivityGithubActivityHandler = github_activity.GithubActivityHandlerFunc(
		func(params github_activity.GithubActivityParams) middleware.Responder {
			githubEvent := utils.GetGithubEvent(params.XGITHUBEVENT)
			if githubEvent == "" {
				return github_activity.NewGithubActivityBadRequest().WithPayload(&models.ErrorResponse{
					Code:    "400",
					Message: "missing github event",
				})
			}

			// we need the raw payload so we can use the github utilities
			payload, err := params.GithubActivityInput.MarshalJSON()
			if err != nil {
				return github_activity.NewGithubActivityBadRequest().WithPayload(&models.ErrorResponse{
					Code:    "400",
					Message: "json marshall",
				})
			}

			event, err := github.ParseWebHook(githubEvent, payload)
			if err != nil {
				return github_activity.NewGithubActivityBadRequest().WithPayload(&models.ErrorResponse{
					Code:    "400",
					Message: fmt.Sprintf("parsing event failed : %v", err),
				})
			}

			var processError error
			switch event := event.(type) {
			case *github.InstallationRepositoriesEvent:
				processError = service.ProcessInstallationRepositoriesEvent(event)
			case *github.RepositoryEvent:
				processError = service.ProcessRepositoryEvent(event)
			default:
				log.Warnf("unsupported event sent : %s", githubEvent)
			}

			if processError != nil {
				log.Warnf("processing event : %s failed with : %v", githubEvent, processError)
			}

			return github_activity.NewGithubActivityOK()
		})
	api.AddMiddlewareFor("POST", "/github/activity", signatureCheckMiddleware)
}

type codedResponse interface {
	Code() string
}

func errorResponse(reqID string, err error) *models.ErrorResponse {
	if reqID == "" {
		requestID, _ := uuid.NewV4()
		reqID = requestID.String()
	}
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
