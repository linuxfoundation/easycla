// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_organizations

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-openapi/runtime"

	project_service "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/gitlab_activity"
	gitlab_api "github.com/communitybridge/easycla/cla-backend-go/gitlab_api"
	"github.com/gofrs/uuid"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/gitlab_organizations"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
)

// Configure setups handlers on api with service
func Configure(api *operations.EasyclaAPI, service ServiceInterface, eventService events.Service) {

	api.GitlabOrganizationsGetProjectGitlabOrganizationsHandler = gitlab_organizations.GetProjectGitlabOrganizationsHandlerFunc(
		func(params gitlab_organizations.GetProjectGitlabOrganizationsParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint

			f := logrus.Fields{
				"functionName":   "v2.gitlab_organizations.handlers.GitlabOrganizationsGetProjectGitlabOrganizationsHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUser":       authUser.UserName,
				"authEmail":      authUser.Email,
				"projectSFID":    params.ProjectSFID,
			}

			// Load the project
			psc := project_service.GetClient()
			projectModel, err := psc.GetProject(params.ProjectSFID)
			if err != nil || projectModel == nil {
				return gitlab_organizations.NewGetProjectGitlabOrganizationsNotFound().WithPayload(
					utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Get Project GitLab Organizations for Project '%s' with scope of %s",
					authUser.UserName, projectModel.Name, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return gitlab_organizations.NewGetProjectGitlabOrganizationsForbidden().WithPayload(
					utils.ErrorResponseForbidden(reqID, msg))
			}

			result, err := service.GetGitlabOrganizations(ctx, params.ProjectSFID)
			if err != nil {
				if strings.ContainsAny(err.Error(), "getProjectNotFound") {
					msg := fmt.Sprintf("Gitlab organization with project SFID not found: %s", params.ProjectSFID)
					log.WithFields(f).Debug(msg)
					return gitlab_organizations.NewGetProjectGitlabOrganizationsNotFound().WithPayload(
						utils.ErrorResponseNotFound(reqID, msg))
				}

				msg := fmt.Sprintf("failed to locate Gitlab organization by project SFID: %s, error: %+v", params.ProjectSFID, err)
				log.WithFields(f).Debug(msg)
				return gitlab_organizations.NewGetProjectGitlabOrganizationsBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			return gitlab_organizations.NewGetProjectGitlabOrganizationsOK().WithPayload(result)
		})

	api.GitlabOrganizationsAddProjectGitlabOrganizationHandler = gitlab_organizations.AddProjectGitlabOrganizationHandlerFunc(
		func(params gitlab_organizations.AddProjectGitlabOrganizationParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint

			f := logrus.Fields{
				"functionName":   "v2.gitlab_organizations.handlers.GitlabOrganizationsAddProjectGitlabOrganizationHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUser":       authUser.UserName,
				"authEmail":      authUser.Email,
				"projectSFID":    params.ProjectSFID,
			}

			// Load the project
			psc := project_service.GetClient()
			projectModel, err := psc.GetProject(params.ProjectSFID)
			if err != nil || projectModel == nil {
				return gitlab_organizations.NewAddProjectGitlabOrganizationForbidden().WithPayload(
					utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Add Project GitLab Organizations for Project '%s' with scope of %s",
					authUser.UserName, projectModel.Name, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return gitlab_organizations.NewAddProjectGitlabOrganizationForbidden().WithPayload(
					utils.ErrorResponseForbidden(reqID, msg))
			}

			// Quick check of the parameters
			if params.Body == nil || params.Body.OrganizationName == nil {
				msg := fmt.Sprintf("missing organization name in body: %+v", params.Body)
				log.WithFields(f).Warn(msg)
				return gitlab_organizations.NewAddProjectGitlabOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequest(reqID, msg))
			}
			f["organizationName"] = utils.StringValue(params.Body.OrganizationName)

			if params.Body.AutoEnabled == nil {
				msg := fmt.Sprintf("missing autoEnabled name in body: %+v", params.Body)
				log.WithFields(f).Warn(msg)
				return gitlab_organizations.NewAddProjectGitlabOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequest(reqID, msg))
			}
			f["autoEnabled"] = utils.BoolValue(params.Body.AutoEnabled)
			f["autoEnabledClaGroupID"] = params.Body.AutoEnabledClaGroupID

			if !utils.ValidateAutoEnabledClaGroupID(params.Body.AutoEnabled, params.Body.AutoEnabledClaGroupID) {
				msg := "AutoEnabledClaGroupID can't be empty when AutoEnabled"
				err := fmt.Errorf(msg)
				log.WithFields(f).Warn(msg)
				return gitlab_organizations.NewAddProjectGitlabOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			result, err := service.AddGitlabOrganization(ctx, params.ProjectSFID, params.Body)
			if err != nil {
				msg := fmt.Sprintf("unable to add GitLab organization, error: %+v", err)
				log.WithFields(f).WithError(err).Warn(msg)
				return gitlab_organizations.NewAddProjectGitlabOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			// Log the event
			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
				LfUsername:  authUser.UserName,
				EventType:   events.GitlabOrganizationAdded,
				ProjectSFID: params.ProjectSFID,
				EventData: &events.GitlabOrganizationAddedEventData{
					GitlabOrganizationName: *params.Body.OrganizationName,
				},
			})

			return gitlab_organizations.NewAddProjectGitlabOrganizationOK().WithPayload(result)
		})

	api.GitlabOrganizationsUpdateProjectGitlabOrganizationConfigHandler = gitlab_organizations.UpdateProjectGitlabOrganizationConfigHandlerFunc(func(params gitlab_organizations.UpdateProjectGitlabOrganizationConfigParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
		if params.Body.AutoEnabled == nil {
			return gitlab_organizations.NewUpdateProjectGitlabOrganizationConfigBadRequest().WithPayload(&models.ErrorResponse{
				Code:    "400",
				Message: "EasyCLA - 400 Bad Request - missing auto enable value in body",
			})
		}

		if !utils.ValidateAutoEnabledClaGroupID(params.Body.AutoEnabled, params.Body.AutoEnabledClaGroupID) {
			return gitlab_organizations.NewUpdateProjectGitlabOrganizationConfigBadRequest().WithPayload(&models.ErrorResponse{
				Code:    "400",
				Message: "EasyCLA - 400 Bad Request - AutoEnabledClaGroupID can't be empty when AutoEnabled",
			})
		}

		err := service.UpdateGitlabOrganization(ctx, params.ProjectSFID, params.OrgName, *params.Body.AutoEnabled, params.Body.AutoEnabledClaGroupID, params.Body.BranchProtectionEnabled)
		if err != nil {
			if errors.Is(err, projects_cla_groups.ErrCLAGroupDoesNotExist) {
				return gitlab_organizations.NewUpdateProjectGitlabOrganizationConfigNotFound().WithPayload(utils.ErrorResponseNotFound(reqID, err.Error()))
			}
			return gitlab_organizations.NewUpdateProjectGitlabOrganizationConfigBadRequest().WithPayload(utils.ErrorResponseBadRequestWithError(reqID, "updating  gitlab org", err))
		}

		eventService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:   events.GitlabOrganizationUpdated,
			ProjectSFID: params.ProjectSFID,
			LfUsername:  authUser.UserName,
			UserName:    authUser.UserName,
			EventData: &events.GitlabOrganizationUpdatedEventData{
				GitlabOrganizationName: params.OrgName,
				AutoEnabled:            *params.Body.AutoEnabled,
			},
		})

		return gitlab_organizations.NewUpdateProjectGitlabOrganizationConfigOK()
	})

	api.GitlabOrganizationsDeleteProjectGitlabOrganizationHandler = gitlab_organizations.DeleteProjectGitlabOrganizationHandlerFunc(func(params gitlab_organizations.DeleteProjectGitlabOrganizationParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
		f := logrus.Fields{
			"functionName":   "v2.gitlab_organizations.handlers.GitlabOrganizationsDeleteProjectGitlabOrganizationHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"projectSFID":    params.ProjectSFID,
			"orgName":        params.OrgName,
			"authUser":       authUser.UserName,
			"authEmail":      authUser.Email,
		}

		// Load the project
		psc := project_service.GetClient()
		projectModel, err := psc.GetProject(params.ProjectSFID)
		if err != nil || projectModel == nil {
			return gitlab_organizations.NewDeleteProjectGitlabOrganizationNotFound().WithPayload(
				utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
		}

		if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("user %s does not have access to Delete Project GitLab Organizations for Project '%s' with scope of %s",
				authUser.UserName, projectModel.Name, params.ProjectSFID)
			log.WithFields(f).Debug(msg)
			return gitlab_organizations.NewDeleteProjectGitlabOrganizationForbidden().WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		err = service.DeleteGitlabOrganization(ctx, params.ProjectSFID, params.OrgName)
		if err != nil {
			if strings.Contains(err.Error(), "getProjectNotFound") {
				msg := fmt.Sprintf("project not found with given SFID: %s", params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return gitlab_organizations.NewDeleteProjectGitlabOrganizationNotFound().WithPayload(utils.ErrorResponseNotFoundWithError(reqID, msg, err))
			}
			msg := fmt.Sprintf("problem deleting Gitlab Organization with project SFID: %s for organization: %s", params.ProjectSFID, params.OrgName)
			log.WithFields(f).Debug(msg)
			return gitlab_organizations.NewDeleteProjectGitlabOrganizationBadRequest().WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		eventService.LogEventWithContext(ctx, &events.LogEventArgs{
			LfUsername:  authUser.UserName,
			EventType:   events.GitlabOrganizationDeleted,
			ProjectSFID: params.ProjectSFID,
			EventData: &events.GitlabOrganizationDeletedEventData{
				GitlabOrganizationName: params.OrgName,
			},
		})

		return gitlab_organizations.NewDeleteProjectGitlabOrganizationNoContent()
	})

	api.GitlabActivityGitlabOauthCallbackHandler = gitlab_activity.GitlabOauthCallbackHandlerFunc(func(params gitlab_activity.GitlabOauthCallbackParams) middleware.Responder {
		ctx := utils.NewContext()
		f := logrus.Fields{
			"functionName":   "gitlab_organization.handlers.GitlabActivityGitlabOauthCallbackHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"code":           params.Code,
			"state":          params.State,
		}

		requestID, _ := uuid.NewV4()
		reqID := requestID.String()
		if params.Code == "" {
			msg := "missing code parameter"
			log.WithFields(f).Errorf(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}

		if params.State == "" {
			msg := "missing state parameter"
			log.WithFields(f).Errorf(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}

		codeParts := strings.Split(params.State, ":")
		if len(codeParts) != 2 {
			msg := fmt.Sprintf("invalid state variable passed : %s", params.State)
			log.WithFields(f).Errorf(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}

		gitlabOrganizationID := codeParts[0]
		stateVar := codeParts[1]

		gitLabOrg, err := service.GetGitlabOrganizationByState(ctx, gitlabOrganizationID, stateVar)
		if err != nil {
			msg := fmt.Sprintf("fetching gitlab model failed : %s : %v", gitlabOrganizationID, err)
			log.WithFields(f).Errorf(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}

		// now fetch the oauth credentials and store to db
		oauthResp, err := gitlab_api.FetchOauthCredentials(params.Code)
		if err != nil {
			msg := fmt.Sprintf("fetching gitlab credentials failed : %s : %v", gitlabOrganizationID, err)
			log.WithFields(f).Errorf(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}
		log.WithFields(f).Debugf("oauth resp is like : %+v", oauthResp)

		err = service.UpdateGitlabOrganizationAuth(ctx, gitlabOrganizationID, oauthResp)
		if err != nil {
			msg := fmt.Sprintf("updating gitlab credentials failed : %s : %v", gitlabOrganizationID, err)
			log.WithFields(f).Errorf(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}

		// Reload the GitLab organization - will have additional details now...
		updatedGitLabOrgDBModel, err := service.GetGitlabOrganizationByID(ctx, gitLabOrg.OrganizationID)
		if err != nil {
			msg := fmt.Sprintf("problem loading updated gitlab organization by ID: %s : %v", gitlabOrganizationID, err)
			log.WithFields(f).Errorf(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}

		return NewSuccessResponse(reqID, updatedGitLabOrgDBModel.ProjectSFID, updatedGitLabOrgDBModel.OrganizationName)
	})
}

// SuccessResponse Success
type SuccessResponse struct {
	ReqID           string
	ProjectSFID     string
	GitLabGroupName string
}

// NewSuccessResponse creates a new redirect handler
func NewSuccessResponse(reqID, projectSFID, gitLabGroupName string) *SuccessResponse {
	return &SuccessResponse{reqID, projectSFID, gitLabGroupName}
}

// WriteResponse to the client
func (o *SuccessResponse) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {
	configPage := "https://gitlab.com/-/profile/applications"

	html := fmt.Sprintf(`<!DOCTYPE html>
    <html lang="en">
	  <head>
			<title>LFX EasyCLA Service GitLab App Installation Status</title>
			<!-- Required meta tags -->
			<meta charset="utf-8">
			<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
			<link rel="shortcut icon" href="https://www.linuxfoundation.org/wp-content/uploads/2017/08/favicon.png">
			<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/css/bootstrap.min.css" integrity="sha384-Gn5384xqQ1aoWXA+058RXPxPg6fy4IWvTNh0E263XmFcJlSAwiGgFAW/dAiS6JXm" crossorigin="anonymous"/>
			<script src="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/js/bootstrap.min.js" integrity="sha384-JZR6Spejh4U02d8jOt6vLEHfe/JQGiRRSQQxSfFWpi1MquVdAyjUar5+76PVCmYl" crossorigin="anonymous"></script>
			<style>h1 { text-align:center;}</style>
		</head>
		<body style='margin-top:20;margin-left:0;margin-right:0;'>
			<div class="text-center">
				<img width=300px" src="https://cla-project-logo-prod.s3.amazonaws.com/lf-horizontal-color.svg" alt="lf logo"/>
			</div> 
 			<h2 class="text-center">LFx EasyCLA Service GitLab App - Installation Successful</h2> 
			<p class="text-center">Thank you for installing the LFX EasyCLA GitLab Application/Bot.  Your GitLab Group and repositories are now onboarded.</p>
			<p class="text-center">To review the configuration or revoke the application, navigate to <a href="%s" target="_blank">the GitLab Applications under your User Settings.</a></p>
			<p class="text-center">You may now close this window and return to the LFX Project Control Center and select the repositories for EasyCLA.</p>
		</body>
	</html>`, configPage)

	rw.Header().Set("Content-Type", "text/html")
	rw.Header().Set(utils.XREQUESTID, o.ReqID)
	rw.WriteHeader(http.StatusOK)
	_, err := rw.Write([]byte(html))
	if err != nil {
		panic(err)
	}
}

// ServerError Success
type ServerError struct {
	ReqID           string
	GitLabGroupName string
	Error           error
}

// NewServerError creates a new redirect handler
func NewServerError(reqID string, gitLabGroupName string, theError error) *ServerError {
	return &ServerError{
		ReqID:           reqID,
		GitLabGroupName: gitLabGroupName,
		Error:           theError,
	}
}

// WriteResponse to the client
func (o *ServerError) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {
	html := fmt.Sprintf(`<!DOCTYPE html>
    <html lang="en">
		<head>
			<title>LFX EasyCLA Service GitLab App Installation Status</title>
			<!-- Required meta tags -->
			<meta charset="utf-8">
			<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
			<link rel="shortcut icon" href="https://www.linuxfoundation.org/wp-content/uploads/2017/08/favicon.png">
			<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/css/bootstrap.min.css" integrity="sha384-Gn5384xqQ1aoWXA+058RXPxPg6fy4IWvTNh0E263XmFcJlSAwiGgFAW/dAiS6JXm" crossorigin="anonymous"/>
			<script src="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/js/bootstrap.min.js" integrity="sha384-JZR6Spejh4U02d8jOt6vLEHfe/JQGiRRSQQxSfFWpi1MquVdAyjUar5+76PVCmYl" crossorigin="anonymous"></script>
			<style>h1 { text-align:center;}</style>
		</head>
		<body style='margin-top:20;margin-left:0;margin-right:0;'>
			<div class="text-center">
				<img width=300px" src="https://cla-project-logo-prod.s3.amazonaws.com/lf-horizontal-color.svg" alt="lf logo"/>
			</div> 
 			<h2 class="text-center">LFx EasyCLA Service GitLab App - Installation Issue</h2> 
			<p class="text-center">Unable to install the GitLab Group %s due to the following error: %s.</p>
		</body>
	</html>`, o.GitLabGroupName, o.Error.Error())

	rw.Header().Set("Content-Type", "text/html")
	rw.Header().Set(utils.XREQUESTID, o.ReqID)
	_, err := rw.Write([]byte(html))
	if err != nil {
		panic(err)
	}
}
