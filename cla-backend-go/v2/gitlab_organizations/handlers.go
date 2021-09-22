// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_organizations

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/v2/common"

	"github.com/go-openapi/runtime"

	projectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/gitlab_activity"
	gitlabApi "github.com/communitybridge/easycla/cla-backend-go/gitlab_api"
	"github.com/gofrs/uuid"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/gitlab_organizations"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
	"github.com/savaki/dynastore"
)

const (
	// SessionStoreKey for cla-gitlab
	SessionStoreKey = "cla-gitlab"
)

// Configure setups handlers on api with service
func Configure(api *operations.EasyclaAPI, service ServiceInterface, eventService events.Service, sessionStore *dynastore.Store, contributorConsoleV2Base string) {

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
			psc := projectService.GetClient()
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

			result, err := service.GetGitLabOrganizationsByProjectSFID(ctx, params.ProjectSFID)
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
				"groupID":        params.Body.GroupID,
				"groupFullPath":  params.Body.OrganizationFullPath,
			}

			// Load the project
			psc := projectService.GetClient()
			projectModel, err := psc.GetProject(params.ProjectSFID)
			if err != nil || projectModel == nil {
				return gitlab_organizations.NewAddProjectGitlabOrganizationForbidden().WithPayload(
					utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
			}

			// Load the project parent
			parentProjectModel, err := psc.GetParentProjectModel(params.ProjectSFID)
			if err != nil || (parentProjectModel == nil && !utils.IsProjectHasRootParent(projectModel)) {
				return gitlab_organizations.NewAddProjectGitlabOrganizationForbidden().WithPayload(
					utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate parent project from project with ID: %s", params.ProjectSFID)))
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Add Project GitLab Organizations for Project '%s' with scope of %s",
					authUser.UserName, projectModel.Name, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return gitlab_organizations.NewAddProjectGitlabOrganizationForbidden().WithPayload(
					utils.ErrorResponseForbidden(reqID, msg))
			}

			// Quick check of the parameters
			if params.Body == nil || (params.Body.GroupID == 0 && params.Body.OrganizationFullPath == "") {
				msg := fmt.Sprintf("missing group ID or group full path in the body: %+v", params.Body)
				log.WithFields(f).Warn(msg)
				return gitlab_organizations.NewAddProjectGitlabOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequest(reqID, msg))
			}

			orgURL := params.Body.OrganizationFullPath
			// Clean up/filter the Group Full Path, if needed
			if params.Body.OrganizationFullPath != "" {
				r, regexErr := regexp.Compile(`^http(s)?://`)
				if regexErr != nil {
					msg := fmt.Sprintf("invalid regex for group/organization full path, error: %+v", regexErr)
					log.WithFields(f).WithError(regexErr).Warn(msg)
					return gitlab_organizations.NewAddProjectGitlabOrganizationInternalServerError().WithPayload(
						utils.ErrorResponseInternalServerErrorWithError(reqID, msg, regexErr))
				}
				if r.MatchString(params.Body.OrganizationFullPath) {
					groupWithUrl, urlParseErr := url.Parse(params.Body.OrganizationFullPath)
					if urlParseErr != nil {
						msg := fmt.Sprintf("invalid group full path provided, error: %+v", urlParseErr)
						log.WithFields(f).WithError(urlParseErr).Warn(msg)
						return gitlab_organizations.NewAddProjectGitlabOrganizationBadRequest().WithPayload(
							utils.ErrorResponseBadRequestWithError(reqID, msg, urlParseErr))
					}
					// Update the group full path value - just include the path and not the https://... part
					params.Body.OrganizationFullPath = groupWithUrl.Path
				}

				// Remove leading slash
				if strings.HasPrefix(params.Body.OrganizationFullPath, "/") {
					params.Body.OrganizationFullPath = params.Body.OrganizationFullPath[1:]
				}
			}

			if params.Body.AutoEnabled == nil {
				msg := fmt.Sprintf("missing autoEnabled name in body: %+v", params.Body)
				log.WithFields(f).Warn(msg)
				return gitlab_organizations.NewAddProjectGitlabOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequest(reqID, msg))
			}
			f["autoEnabled"] = utils.BoolValue(params.Body.AutoEnabled)
			f["autoEnabledClaGroupID"] = params.Body.AutoEnabledClaGroupID

			if !utils.ValidateAutoEnabledClaGroupID(*params.Body.AutoEnabled, params.Body.AutoEnabledClaGroupID) {
				msg := "AutoEnabledClaGroupID can't be empty when AutoEnabled"
				err := fmt.Errorf(msg)
				log.WithFields(f).Warn(msg)
				return gitlab_organizations.NewAddProjectGitlabOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			// If the parent is TLF, then use the same project SFID value for the parent SFID value
			parentProjectSFID := ""
			if parentProjectModel != nil {
				parentProjectSFID = parentProjectModel.ID
			}
			if utils.IsProjectHasRootParent(projectModel) {
				parentProjectSFID = params.ProjectSFID
			}

			// Convert the various input parameters and values to an add GitLab Group/Org model
			inputModel := &common.GitLabAddOrganization{
				ProjectSFID:             params.ProjectSFID,
				ParentProjectSFID:       parentProjectSFID, // could be the same SFID as the project SFID if parent is TLF
				AutoEnabled:             utils.BoolValue(params.Body.AutoEnabled),
				AutoEnabledClaGroupID:   params.Body.AutoEnabledClaGroupID,
				BranchProtectionEnabled: utils.BoolValue(params.Body.BranchProtectionEnabled),
				ExternalGroupID:         params.Body.GroupID,
				OrganizationURL:         orgURL,
				OrganizationFullPath:    params.Body.OrganizationFullPath,
			}

			result, err := service.AddGitLabOrganization(ctx, inputModel)
			if err != nil {
				if _, ok := err.(*utils.ProjectConflict); ok {
					return gitlab_organizations.NewAddProjectGitlabOrganizationConflict().WithPayload(
						utils.ErrorResponseConflict(reqID, err.Error()))
				}
				msg := fmt.Sprintf("unable to add GitLab organization, error: %+v", err)
				log.WithFields(f).WithError(err).Warn(msg)
				return gitlab_organizations.NewAddProjectGitlabOrganizationBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			// Get the current group name for the event
			for _, group := range result.List {
				if group.OrganizationExternalID == params.Body.GroupID || group.OrganizationFullPath == params.Body.OrganizationFullPath {
					// Log the event
					eventService.LogEventWithContext(ctx, &events.LogEventArgs{
						LfUsername:  authUser.UserName,
						EventType:   events.GitlabOrganizationAdded,
						ProjectSFID: params.ProjectSFID,
						EventData: &events.GitLabOrganizationAddedEventData{
							GitLabOrganizationName: group.OrganizationName,
						},
					})
				}
			}

			return gitlab_organizations.NewAddProjectGitlabOrganizationOK().WithPayload(result)
		})

	api.GitlabOrganizationsGetGitLabGroupMembersHandler = gitlab_organizations.GetGitLabGroupMembersHandlerFunc(func(params gitlab_organizations.GetGitLabGroupMembersParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := utils.NewContext()
		f := logrus.Fields{
			"functionName":   "v2.gitlab_organizations.handlers.GitlabOrganizationsGetGitLabGroupMembersHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"gitLabGroupID":  params.GitLabGroupID,
		}
		log.WithFields(f).Debug("fetching gitlab group member details")
		memberList, err := service.GetGitLabGroupMembers(ctx, params.GitLabGroupID)
		if err != nil {
			msg := fmt.Sprintf("unable to get groupID: %s  member list: %+v ", params.GitLabGroupID, err)
			return gitlab_organizations.NewGetGitLabGroupMembersBadRequest().WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
		}
		return gitlab_organizations.NewGetGitLabGroupMembersOK().WithPayload(memberList)
	})

	api.GitlabOrganizationsUpdateProjectGitlabGroupConfigHandler = gitlab_organizations.UpdateProjectGitlabGroupConfigHandlerFunc(func(params gitlab_organizations.UpdateProjectGitlabGroupConfigParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint

		f := logrus.Fields{
			"functionName":            "v2.gitlab_organizations.handlers.GitlabOrganizationsAddProjectGitlabOrganizationHandler",
			utils.XREQUESTID:          ctx.Value(utils.XREQUESTID),
			"authUser":                authUser.UserName,
			"authEmail":               authUser.Email,
			"projectSFID":             params.ProjectSFID,
			"gitLabGroupID":           params.GitLabGroupID,
			"autoEnabled":             params.Body.AutoEnabled,
			"autoEnabledCLAGroupID":   params.Body.AutoEnabledClaGroupID,
			"branchProtectionEnabled": params.Body.BranchProtectionEnabled,
		}

		// Load the project
		psc := projectService.GetClient()
		projectModel, err := psc.GetProject(params.ProjectSFID)
		if err != nil || projectModel == nil {
			return gitlab_organizations.NewUpdateProjectGitlabGroupConfigNotFound().WithPayload(
				utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
		}

		// Load the project parent
		parentProjectModel, err := psc.GetParentProjectModel(params.ProjectSFID)
		if err != nil || parentProjectModel == nil {
			msg := fmt.Sprintf("unable to locate parent project from project with ID: %s", params.ProjectSFID)
			log.WithFields(f).Warn(msg)
		}

		if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("user %s does not have access to Update Project GitLab Group/Organizations for Project '%s' with scope of %s",
				authUser.UserName, projectModel.Name, params.ProjectSFID)
			log.WithFields(f).Debug(msg)
			return gitlab_organizations.NewUpdateProjectGitlabGroupConfigForbidden().WithPayload(
				utils.ErrorResponseForbidden(reqID, msg))
		}

		if !utils.ValidateAutoEnabledClaGroupID(params.Body.AutoEnabled, params.Body.AutoEnabledClaGroupID) {
			msg := "AutoEnabledClaGroupID can't be empty when AutoEnabled is set to true"
			return gitlab_organizations.NewUpdateProjectGitlabGroupConfigBadRequest().WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
		}

		inputModel := &common.GitLabAddOrganization{
			ProjectSFID:             params.ProjectSFID,
			AutoEnabled:             params.Body.AutoEnabled,
			AutoEnabledClaGroupID:   params.Body.AutoEnabledClaGroupID,
			BranchProtectionEnabled: params.Body.BranchProtectionEnabled,
			ExternalGroupID:         params.GitLabGroupID,
			Enabled:                 true,
		}

		if parentProjectModel != nil {
			inputModel.ParentProjectSFID = parentProjectModel.ID
		}

		updateErr := service.UpdateGitLabOrganization(ctx, inputModel)
		if updateErr != nil {
			if errors.Is(updateErr, projects_cla_groups.ErrCLAGroupDoesNotExist) {
				msg := fmt.Sprintf("problem updating GitLab group/organization for project %s with SFID: %s - CLA Group wth ID: %s was not found, error: %+v", projectModel.Name, projectModel.ID, params.Body.AutoEnabledClaGroupID, updateErr)
				return gitlab_organizations.NewUpdateProjectGitlabGroupConfigNotFound().WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}
			msg := fmt.Sprintf("problem updating GitLab group/organization for project %s with SFID: %s, error: %+v", projectModel.Name, projectModel.ID, updateErr)
			return gitlab_organizations.NewUpdateProjectGitlabGroupConfigBadRequest().WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, updateErr))
		}

		eventService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:   events.GitlabOrganizationUpdated,
			ProjectSFID: params.ProjectSFID,
			ProjectName: projectModel.Name,
			CLAGroupID:  params.Body.AutoEnabledClaGroupID,
			LfUsername:  authUser.UserName,
			UserName:    authUser.UserName,
			EventData: &events.GitLabOrganizationUpdatedEventData{
				GitLabGroupID:         params.GitLabGroupID,
				AutoEnabledClaGroupID: params.Body.AutoEnabledClaGroupID,
				AutoEnabled:           params.Body.AutoEnabled,
			},
		})

		results, err := service.GetGitLabOrganizationsByProjectSFID(ctx, params.ProjectSFID)
		if err != nil {
			if strings.ContainsAny(err.Error(), "getProjectNotFound") {
				msg := fmt.Sprintf("Gitlab organization with project SFID not found: %s", params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return gitlab_organizations.NewUpdateProjectGitlabGroupConfigNotFound().WithPayload(utils.ErrorResponseNotFound(reqID, msg))
			}

			msg := fmt.Sprintf("failed to locate Gitlab organization by project SFID: %s, error: %+v", params.ProjectSFID, err)
			log.WithFields(f).Debug(msg)
			return gitlab_organizations.NewUpdateProjectGitlabGroupConfigBadRequest().WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		return gitlab_organizations.NewUpdateProjectGitlabGroupConfigOK().WithPayload(results)
	})

	api.GitlabOrganizationsDeleteProjectGitlabGroupConfigHandler = gitlab_organizations.DeleteProjectGitlabGroupConfigHandlerFunc(func(params gitlab_organizations.DeleteProjectGitlabGroupConfigParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
		f := logrus.Fields{
			"functionName":         "v2.gitlab_organizations.handlers.GitlabOrganizationsDeleteProjectGitlabGroupConfigHandler",
			utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
			"projectSFID":          params.ProjectSFID,
			"organizationFullPath": params.OrganizationFullPath,
			"authUser":             authUser.UserName,
			"authEmail":            authUser.Email,
		}

		// Load the project
		psc := projectService.GetClient()
		projectModel, err := psc.GetProject(params.ProjectSFID)
		if err != nil || projectModel == nil {
			return gitlab_organizations.NewDeleteProjectGitlabGroupConfigNotFound().WithPayload(
				utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
		}

		if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("user %s does not have access to Delete Project GitLab Group/Organizations for Project '%s' with scope of %s",
				authUser.UserName, projectModel.Name, params.ProjectSFID)
			log.WithFields(f).Debug(msg)
			return gitlab_organizations.NewDeleteProjectGitlabGroupConfigForbidden().WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		err = service.DeleteGitLabOrganizationByFullPath(ctx, params.ProjectSFID, params.OrganizationFullPath)
		if err != nil {
			if strings.Contains(err.Error(), "getProjectNotFound") {
				msg := fmt.Sprintf("project not found with given SFID: %s", params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return gitlab_organizations.NewDeleteProjectGitlabGroupConfigNotFound().WithPayload(utils.ErrorResponseNotFoundWithError(reqID, msg, err))
			}
			msg := fmt.Sprintf("problem deleting Gitlab Group with project SFID: %s with path: %s", params.ProjectSFID, params.OrganizationFullPath)
			log.WithFields(f).Warn(msg)
			return gitlab_organizations.NewDeleteProjectGitlabGroupConfigBadRequest().WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
		}

		eventService.LogEventWithContext(ctx, &events.LogEventArgs{
			LfUsername:  authUser.UserName,
			EventType:   events.GitlabOrganizationDeleted,
			ProjectSFID: params.ProjectSFID,
			EventData: &events.GitLabOrganizationDeletedEventData{
				GitLabOrganizationName: params.OrganizationFullPath,
			},
		})

		return gitlab_organizations.NewDeleteProjectGitlabGroupConfigNoContent()
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
			log.WithFields(f).Warn(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}

		if params.State == "" {
			msg := "missing state parameter"
			log.WithFields(f).Warn(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}

		codeParts := strings.Split(params.State, ":")
		if len(codeParts) != 2 {
			msg := fmt.Sprintf("invalid state variable passed : %s", params.State)
			log.WithFields(f).Warn(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}

		if codeParts[0] == "user" {
			// handle authorization
			return middleware.ResponderFunc(
				func(rw http.ResponseWriter, p runtime.Producer) {
					session, err := sessionStore.Get(params.HTTPRequest, SessionStoreKey)
					if err != nil {
						log.WithFields(f).WithError(err).Warn("error with session store lookup")
						http.Error(rw, err.Error(), http.StatusInternalServerError)
						return
					}
					log.WithFields(f).Debugf("Loaded session: %+v", session.Values)

					state, ok := session.Values["gitlab_oauth2_state"].(string)
					if !ok {
						msg := "Error getting session state - missing from session object"
						log.WithFields(f).Warn(msg)
						http.Error(rw, msg, http.StatusInternalServerError)
						return
					}

					gitlabOriginURL, ok := session.Values["gitlab_origin_url"].(string)
					if !ok {
						msg := "Error getting gitlab_origin_url - missing from session object"
						log.WithFields(f).Warn(msg)
						http.Error(rw, msg, http.StatusInternalServerError)
						return
					}

					repositoryID, ok := session.Values["gitlab_repository_id"].(string)
					if !ok {
						msg := "Error getting gitlab_repository_id - missing from session object"
						log.WithFields(f).Warn(msg)
						http.Error(rw, msg, http.StatusInternalServerError)
						return
					}

					mergeRequestID, ok := session.Values["gitlab_merge_request_id"].(string)
					if !ok {
						msg := "Error getting gitlab_merge_request_id - missing from session object"
						log.WithFields(f).Warn(msg)
						http.Error(rw, msg, http.StatusInternalServerError)
						return
					}

					if params.State != state {
						msg := fmt.Sprintf("mismatch state, received: %s from callback, but loaded our state as: %s",
							params.State, state)
						log.WithFields(f).Warn(msg)
						http.Error(rw, msg, http.StatusInternalServerError)
						return
					}

					log.WithFields(f).Debug("Fetching access token for user...")
					token, err := gitlabApi.FetchOauthCredentials(params.Code)
					if err != nil {
						msg := fmt.Sprint("unable to fetch access token for user")
						log.WithFields(f).Warn(msg)
						http.Error(rw, msg, http.StatusInternalServerError)
						return
					}

					session.Values["gitlab_oauth2_token"] = token.AccessToken
					session.Save(params.HTTPRequest, rw)

					// Get client
					gitlabClient, err := gitlabApi.NewGitlabOauthClientFromAccessToken(token.AccessToken)
					if err != nil {
						msg := fmt.Sprintf("unable to create gitlab client from token : %s ", token.AccessToken)
						log.WithFields(f).Warn(msg)
						http.Error(rw, msg, http.StatusInternalServerError)
						return
					}

					consoleURL, err := service.InitiateSignRequest(ctx, params.HTTPRequest, gitlabClient, repositoryID, mergeRequestID, gitlabOriginURL, contributorConsoleV2Base, eventService)
					log.WithFields(f).Debugf("redirecting to :%s ", *consoleURL)
					http.Redirect(rw, params.HTTPRequest, *consoleURL, http.StatusSeeOther)
				})
		}

		gitlabOrganizationID := codeParts[0]
		stateVar := codeParts[1]

		gitLabOrg, err := service.GetGitLabOrganizationByState(ctx, gitlabOrganizationID, stateVar)
		if err != nil {
			msg := fmt.Sprintf("fetching gitlab model failed : %s : %v", gitlabOrganizationID, err)
			log.WithFields(f).WithError(err).Warn(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}

		// now fetch the oauth credentials and store to db
		oauthResp, err := gitlabApi.FetchOauthCredentials(params.Code)
		if err != nil {
			msg := fmt.Sprintf("fetching gitlab credentials failed : %s : %v", gitlabOrganizationID, err)
			log.WithFields(f).WithError(err).Warn(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}
		log.WithFields(f).Debugf("oauth resp is like : %+v", oauthResp)

		updateErr := service.UpdateGitLabOrganizationAuth(ctx, gitlabOrganizationID, oauthResp)
		if updateErr != nil {
			msg := fmt.Sprintf("installation of GitLab Group and Repositories, error: %v", updateErr)
			log.WithFields(f).WithError(updateErr).Warn(msg)
			return NewServerError(reqID, "", errors.New(msg))
		}

		// Reload the GitLab organization - will have additional details now...
		updatedGitLabOrgDBModel, err := service.GetGitLabOrganizationByID(ctx, gitLabOrg.OrganizationID)
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
