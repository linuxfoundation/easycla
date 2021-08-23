// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"errors"
	"fmt"
	"strings"

	project_service "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/gitlab_repositories"

	"github.com/communitybridge/easycla/cla-backend-go/github/branch_protection"

	"github.com/sirupsen/logrus"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/communitybridge/easycla/cla-backend-go/github"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/github_repositories"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
	"github.com/jinzhu/copier"
)

// Configure establishes the middleware handlers for the repository service
func Configure(api *operations.EasyclaAPI, service ServiceInterface, eventService events.Service) { // nolint
	api.GithubRepositoriesGetProjectGithubRepositoriesHandler = github_repositories.GetProjectGithubRepositoriesHandlerFunc(
		func(params github_repositories.GetProjectGithubRepositoriesParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
			f := logrus.Fields{
				"functionName":   "v2.repositories.handlers.GitHubRepositoriesGetProjectGithubRepositoriesHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUser":       authUser.UserName,
				"authEmail":      authUser.Email,
				"projectSFID":    params.ProjectSFID,
			}

			// Load the project
			psc := project_service.GetClient()
			projectModel, err := psc.GetProject(params.ProjectSFID)
			if err != nil || projectModel == nil {
				return github_repositories.NewGetProjectGithubRepositoriesNotFound().WithPayload(
					utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Get GitHub repositories for Project %s with scope of %s",
					authUser.UserName, projectModel.Name, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return github_repositories.NewGetProjectGithubRepositoriesForbidden().WithPayload(
					utils.ErrorResponseForbidden(reqID, msg))
			}

			result, err := service.GitHubListProjectRepositories(ctx, params.ProjectSFID)
			if err != nil {
				if strings.ContainsAny(err.Error(), "getProjectNotFound") {
					msg := fmt.Sprintf("repository not found for projectSFID: %s", params.ProjectSFID)
					log.WithFields(f).WithError(err).Warn(msg)
					return github_repositories.NewGetProjectGithubRepositoriesNotFound().WithPayload(
						utils.ErrorResponseNotFound(reqID, msg))
				}

				msg := fmt.Sprintf("problem looking up repositories for projectSFID: %s", params.ProjectSFID)
				log.WithFields(f).WithError(err).Warn(msg)
				return github_repositories.NewGetProjectGithubRepositoriesBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			response := &models.GithubListRepositories{}
			err = copier.Copy(response, result)
			if err != nil {
				msg := fmt.Sprintf("problem converting response for projectSFID: %s", params.ProjectSFID)
				log.WithFields(f).WithError(err).Warn(msg)
				return github_repositories.NewGetProjectGithubRepositoriesInternalServerError().WithPayload(
					utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			return github_repositories.NewGetProjectGithubRepositoriesOK().WithPayload(response)
		})

	api.GithubRepositoriesAddProjectGithubRepositoryHandler = github_repositories.AddProjectGithubRepositoryHandlerFunc(
		func(params github_repositories.AddProjectGithubRepositoryParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
			f := logrus.Fields{
				"functionName":           "v2.repositories.handlers.GitHubRepositoriesAddProjectGithubRepositoryHandler",
				utils.XREQUESTID:         ctx.Value(utils.XREQUESTID),
				"authUser":               authUser.UserName,
				"authEmail":              authUser.Email,
				"projectSFID":            params.ProjectSFID,
				"claGroupID":             utils.StringValue(params.GithubRepositoryInput.ClaGroupID),
				"githubOrganizationName": utils.StringValue(params.GithubRepositoryInput.GithubOrganizationName),
				"repositoryGitHubID":     params.GithubRepositoryInput.RepositoryGithubID,
				"repositoryGitHubIDs":    strings.Join(params.GithubRepositoryInput.RepositoryGithubIds, ","),
			}

			// Load the project
			psc := project_service.GetClient()
			projectModel, err := psc.GetProject(params.ProjectSFID)
			if err != nil || projectModel == nil {
				return github_repositories.NewAddProjectGithubRepositoryNotFound().WithPayload(
					utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to add GitHub repositories for Project %s with scope of %s",
					authUser.UserName, projectModel.Name, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return github_repositories.NewAddProjectGithubRepositoryForbidden().WithPayload(
					utils.ErrorResponseForbidden(reqID, msg))
			}

			// If no repository GitHub ID values provided...
			// RepositoryGithubID - provided by the older retool UI which provides only one value
			// RepositoryGithubIds - provided by new PCC which passes multiple values
			if params.GithubRepositoryInput.RepositoryGithubID == "" && len(params.GithubRepositoryInput.RepositoryGithubIds) == 0 {
				msg := "missing repository GitHub ID value(s)"
				log.WithFields(f).Warn(msg)
				return github_repositories.NewAddProjectGithubRepositoryBadRequest().WithPayload(
					utils.ErrorResponseBadRequest(reqID, msg))
			}

			log.WithFields(f).Debugf("Adding GitHub repositories for project: %s", params.ProjectSFID)
			results, err := service.GitHubAddRepositories(ctx, params.ProjectSFID, params.GithubRepositoryInput)
			if err != nil {
				if _, ok := err.(*utils.GitHubRepositoryExists); ok {
					msg := fmt.Sprintf("unable to add repository - repository already exists for projectSFID: %s", params.ProjectSFID)
					log.WithFields(f).WithError(err).Warn(msg)
					return github_repositories.NewAddProjectGithubRepositoryConflict().WithPayload(
						utils.ErrorResponseConflictWithError(reqID, msg, err))
				}
				msg := fmt.Sprintf("problem adding github repositories for projectSFID: %s", params.ProjectSFID)
				log.WithFields(f).WithError(err).Warn(msg)
				return github_repositories.NewAddProjectGithubRepositoryBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			// Log the events
			for _, result := range results {
				eventService.LogEventWithContext(ctx, &events.LogEventArgs{
					EventType:   events.RepositoryAdded,
					ProjectID:   utils.StringValue(params.GithubRepositoryInput.ClaGroupID),
					ProjectSFID: params.ProjectSFID,
					LfUsername:  authUser.UserName,
					ClaGroupModel: &v1Models.ClaGroup{
						ProjectExternalID: params.ProjectSFID,
						ProjectID:         utils.StringValue(params.GithubRepositoryInput.ClaGroupID),
					},
					EventData: &events.RepositoryAddedEventData{
						RepositoryName: result.RepositoryName,
					},
				})
			}

			var v2ResponseList []*models.GithubRepository
			err = copier.Copy(&v2ResponseList, results)
			if err != nil {
				msg := fmt.Sprintf("problem converting response for projectSFID: %s", params.ProjectSFID)
				log.WithFields(f).WithError(err).Warn(msg)
				return github_repositories.NewAddProjectGithubRepositoryInternalServerError().WithPayload(
					utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			v2Response := &models.GithubListRepositories{}
			v2Response.List = v2ResponseList

			return github_repositories.NewAddProjectGithubRepositoryOK().WithPayload(v2Response)
		})

	api.GithubRepositoriesDeleteProjectGithubRepositoryHandler = github_repositories.DeleteProjectGithubRepositoryHandlerFunc(
		func(params github_repositories.DeleteProjectGithubRepositoryParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
			f := logrus.Fields{
				"functionName":   "v2.repositories.handlers.GitHubRepositoriesDeleteProjectGithubRepositoryHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUser":       authUser.UserName,
				"authEmail":      authUser.Email,
				"projectSFID":    params.ProjectSFID,
				"repositoryID":   params.RepositoryID,
			}

			// Load the project
			psc := project_service.GetClient()
			projectModel, err := psc.GetProject(params.ProjectSFID)
			if err != nil || projectModel == nil {
				return github_repositories.NewDeleteProjectGithubRepositoryNotFound().WithPayload(
					utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Get GitHub repositories for Project %s with scope of %s",
					authUser.UserName, projectModel.Name, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return github_repositories.NewDeleteProjectGithubRepositoryForbidden().WithPayload(
					utils.ErrorResponseForbidden(reqID, msg))
			}

			ghRepo, err := service.GitHubGetRepository(ctx, params.RepositoryID)
			if err != nil {
				if _, ok := err.(*utils.GitHubRepositoryNotFound); ok {
					msg := fmt.Sprintf("repository not found for projectSFID: %s", params.ProjectSFID)
					log.WithFields(f).WithError(err).Warn(msg)
					return github_repositories.NewDeleteProjectGithubRepositoryNotFound().WithPayload(
						utils.ErrorResponseNotFound(reqID, msg))
				}

				msg := fmt.Sprintf("problem looking up repository for projectSFID: %s", params.ProjectSFID)
				log.WithFields(f).WithError(err).Warn(msg)
				return github_repositories.NewDeleteProjectGithubRepositoryBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			err = service.GitHubDisableRepository(ctx, params.RepositoryID)
			if err != nil {
				msg := fmt.Sprintf("problem disabling repository for projectSFID: %s, error: %+v", params.ProjectSFID, err)
				log.WithFields(f).WithError(err).Warn(msg)
				return github_repositories.NewDeleteProjectGithubRepositoryBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType:   events.RepositoryDisabled,
				ProjectSFID: params.ProjectSFID,
				CLAGroupID:  ghRepo.RepositoryClaGroupID,
				LfUsername:  authUser.UserName,
				EventData: &events.RepositoryDisabledEventData{
					RepositoryName: ghRepo.RepositoryName,
				},
			})

			return github_repositories.NewDeleteProjectGithubRepositoryNoContent()
		})

	api.GithubRepositoriesGetProjectGithubRepositoryBranchProtectionHandler = github_repositories.GetProjectGithubRepositoryBranchProtectionHandlerFunc(
		func(params github_repositories.GetProjectGithubRepositoryBranchProtectionParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
			f := logrus.Fields{
				"functionName":   "v2.repositories.handlers.GitHubRepositoriesGetProjectGithubRepositoryBranchProtectionHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUser":       authUser.UserName,
				"authEmail":      authUser.Email,
				"projectSFID":    params.ProjectSFID,
				"repositoryID":   params.RepositoryID,
			}

			// Load the project
			psc := project_service.GetClient()
			projectModel, err := psc.GetProject(params.ProjectSFID)
			if err != nil || projectModel == nil {
				return github_repositories.NewGetProjectGithubRepositoryBranchProtectionNotFound().WithPayload(
					utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Query Protected Branch GitHub Repositories for Project %s with scope of %s",
					authUser.UserName, projectModel.Name, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return github_repositories.NewGetProjectGithubRepositoryBranchProtectionForbidden().WithPayload(
					utils.ErrorResponseForbidden(reqID, msg))
			}

			var branchName string
			if params.BranchName == nil || *params.BranchName == "" {
				branchName = branch_protection.DefaultBranchName
			} else {
				branchName = *params.BranchName
			}

			protectedBranch, err := service.GitHubGetProtectedBranch(ctx, params.ProjectSFID, params.RepositoryID, branchName)
			if err != nil {
				if _, ok := err.(*utils.GitHubRepositoryNotFound); ok {
					msg := fmt.Sprintf("unable to locatate branch protection projectSFID: %s, repository: %s", params.ProjectSFID, params.RepositoryID)
					log.WithFields(f).WithError(err).Warn(msg)
					return github_repositories.NewGetProjectGithubRepositoryBranchProtectionNotFound().WithPayload(
						utils.ErrorResponseNotFound(reqID, msg))
				}
				if errors.Is(err, github.ErrAccessDenied) {
					msg := fmt.Sprintf("access denied for branch protection for projectSFID: %s, repository: %s", params.ProjectSFID, params.RepositoryID)
					log.WithFields(f).WithError(err).Warn(msg)
					return github_repositories.NewGetProjectGithubRepositoryBranchProtectionForbidden().WithPayload(
						utils.ErrorResponseNotFound(reqID, msg))
				}

				// shall we return the actual code for rate liming ?
				if errors.Is(err, github.ErrRateLimited) {
					msg := fmt.Sprintf("problem loading branch protection for projectSFID: %s, repository: %s", params.ProjectSFID, params.RepositoryID)
					log.WithFields(f).WithError(err).Warn(msg)
					return github_repositories.NewGetProjectGithubRepositoryBranchProtectionInternalServerError().WithPayload(
						utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
				}

				msg := fmt.Sprintf("problem loading branch protection for projectSFID: %s, repository: %s, error: %+v", params.ProjectSFID, params.RepositoryID, err)
				log.WithFields(f).WithError(err).Warn(msg)
				return github_repositories.NewGetProjectGithubRepositoryBranchProtectionBadRequest().WithPayload(
					utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			return github_repositories.NewGetProjectGithubRepositoryBranchProtectionOK().WithPayload(protectedBranch)
		})

	api.GithubRepositoriesUpdateProjectGithubRepositoryBranchProtectionHandler = github_repositories.UpdateProjectGithubRepositoryBranchProtectionHandlerFunc(
		func(params github_repositories.UpdateProjectGithubRepositoryBranchProtectionParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
			f := logrus.Fields{
				"functionName":   "v2.repositories.handlers.GitHubRepositoriesUpdateProjectGitHubRepositoryBranchProtectionHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUser":       authUser.UserName,
				"authEmail":      authUser.Email,
				"projectSFID":    params.ProjectSFID,
				"repositoryID":   params.RepositoryID,
			}

			// Load the project
			psc := project_service.GetClient()
			projectModel, err := psc.GetProject(params.ProjectSFID)
			if err != nil || projectModel == nil {
				return github_repositories.NewUpdateProjectGithubRepositoryBranchProtectionNotFound().WithPayload(
					utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Update Protected Branch GitHub Repositories for Project %s with scope of %s",
					authUser.UserName, projectModel.Name, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return github_repositories.NewUpdateProjectGithubRepositoryBranchProtectionForbidden().WithPayload(
					utils.ErrorResponseForbidden(reqID, msg))
			}

			protectedBranch, err := service.GitHubUpdateProtectedBranch(ctx, params.ProjectSFID, params.RepositoryID, params.GithubRepositoryBranchProtectionInput)
			if err != nil {
				log.Warnf("update protected branch failed for gitV1Repository %s : %v", params.RepositoryID, err)
				if _, ok := err.(*utils.GitHubRepositoryNotFound); ok {
					msg := fmt.Sprintf("unable to update branch protection for projectSFID: %s, repository: %s", params.ProjectSFID, params.RepositoryID)
					log.WithFields(f).WithError(err).Warn(msg)
					return github_repositories.NewGetProjectGithubRepositoryBranchProtectionNotFound().WithPayload(
						utils.ErrorResponseNotFound(reqID, msg))
				}
				if errors.Is(err, github.ErrAccessDenied) {
					msg := fmt.Sprintf("access denied for branch protection for projectSFID: %s, repository: %s", params.ProjectSFID, params.RepositoryID)
					log.WithFields(f).WithError(err).Warn(msg)
					return github_repositories.NewGetProjectGithubRepositoryBranchProtectionForbidden().WithPayload(
						utils.ErrorResponseNotFound(reqID, msg))
				}

				// shall we return the actual code for rate liming ?
				if errors.Is(err, github.ErrRateLimited) {
					msg := fmt.Sprintf("problem updating branch protection for projectSFID: %s, repository: %s", params.ProjectSFID, params.RepositoryID)
					log.WithFields(f).WithError(err).Warn(msg)
					return github_repositories.NewGetProjectGithubRepositoryBranchProtectionInternalServerError().WithPayload(
						utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
				}

				if errors.Is(err, ErrInvalidBranchProtectionName) {
					msg := fmt.Sprintf("problem updating branch protection for projectSFID: %s, repository: %s, error: %+v", params.ProjectSFID, params.RepositoryID, err)
					log.WithFields(f).WithError(err).Warn(msg)
					return github_repositories.NewGetProjectGithubRepositoryBranchProtectionBadRequest().WithPayload(
						utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
				}

				msg := fmt.Sprintf("problem updating branch protection for projectSFID: %s, repository: %s, error: %+v", params.ProjectSFID, params.RepositoryID, err)
				log.WithFields(f).WithError(err).Warn(msg)
				return github_repositories.NewGetProjectGithubRepositoryBranchProtectionBadRequest().WithPayload(
					utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			repoModel, repoErr := service.GitHubGetRepository(ctx, params.RepositoryID)
			if repoErr != nil {
				msg := fmt.Sprintf("problem fetching the repository for projectSFID: %s, with repository: %s, error: %+v", params.ProjectSFID, params.RepositoryID, err)
				log.WithFields(f).WithError(repoErr).Warning(msg)
				return github_repositories.NewGetProjectGithubRepositoryBranchProtectionBadRequest().WithPayload(
					utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			// We could extract the parameter values from the branch protection payload to determine if it was added/remove or simply updated
			// For now, let's just set the updated event log
			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType:   events.RepositoryBranchProtectionUpdated,
				ProjectSFID: params.ProjectSFID,
				ProjectID:   params.ProjectSFID,
				LfUsername:  authUser.UserName,
				EventData: &events.RepositoryBranchProtectionUpdatedEventData{
					RepositoryName: repoModel.RepositoryName,
				},
			})

			return github_repositories.NewGetProjectGithubRepositoryBranchProtectionOK().WithPayload(protectedBranch)
		})

	api.GitlabRepositoriesGetProjectGitLabRepositoriesHandler = gitlab_repositories.GetProjectGitLabRepositoriesHandlerFunc(
		func(params gitlab_repositories.GetProjectGitLabRepositoriesParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
			f := logrus.Fields{
				"functionName":   "v2.repositories.handlers.GitlabRepositoriesGetProjectGitLabRepositoriesHandler",
				utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
				"authUser":       authUser.UserName,
				"authEmail":      authUser.Email,
				"projectSFID":    params.ProjectSFID,
			}

			// Load the project
			psc := project_service.GetClient()
			projectModel, err := psc.GetProject(params.ProjectSFID)
			if err != nil || projectModel == nil {
				return gitlab_repositories.NewGetProjectGitLabRepositoriesNotFound().WithPayload(
					utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Get GitLab Repositories for Project %s with scope of %s",
					authUser.UserName, projectModel.Name, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return gitlab_repositories.NewGetProjectGitLabRepositoriesForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			result, err := service.GitLabGetRepositoriesByProjectSFID(ctx, params.ProjectSFID)
			if err != nil {
				if strings.ContainsAny(err.Error(), "getProjectNotFound") {
					msg := fmt.Sprintf("repository not found for projectSFID: %s", params.ProjectSFID)
					log.WithFields(f).WithError(err).Warn(msg)
					return gitlab_repositories.NewGetProjectGitLabRepositoriesNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
				}

				msg := fmt.Sprintf("problem looking up repositories for projectSFID: %s", params.ProjectSFID)
				log.WithFields(f).WithError(err).Warn(msg)
				return gitlab_repositories.NewGetProjectGitLabRepositoriesBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			response := &models.GitlabRepositoriesList{}
			err = copier.Copy(response, result)
			if err != nil {
				msg := fmt.Sprintf("problem converting response for projectSFID: %s", params.ProjectSFID)
				log.WithFields(f).WithError(err).Warn(msg)
				return gitlab_repositories.NewGetProjectGitLabRepositoriesInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
			}

			return gitlab_repositories.NewGetProjectGitLabRepositoriesOK().WithPayload(response)
		})

	api.GitlabRepositoriesEnableGitLabRepositoryHandler = gitlab_repositories.EnableGitLabRepositoryHandlerFunc(
		func(params gitlab_repositories.EnableGitLabRepositoryParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
			f := logrus.Fields{
				"functionName":     "v2.repositories.handlers.GitlabRepositoriesEnableGitLabRepositoryHandler",
				utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
				"authUser":         authUser.UserName,
				"authEmail":        authUser.Email,
				"projectSFID":      params.ProjectSFID,
				"organizationName": params.GitlabRepositoriesEnable.GitlabOrganizationName,
				"claGroupID":       params.GitlabRepositoriesEnable.ClaGroupID,
				"groupFullPath":    params.GitlabRepositoriesEnable.OrganizationFullPath,
				"groupID":          params.GitlabRepositoriesEnable.OrganizationExternalID,
			}

			// Load the project
			psc := project_service.GetClient()
			projectModel, err := psc.GetProject(params.ProjectSFID)
			if err != nil || projectModel == nil {
				return gitlab_repositories.NewEnableGitLabRepositoryNotFound().WithPayload(
					utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Add GitLab Repositories for Project '%s' with scope of %s",
					authUser.UserName, projectModel.Name, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return gitlab_repositories.NewEnableGitLabRepositoryForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			if len(params.GitlabRepositoriesEnable.RepositoryGitlabIds) == 0 {
				msg := "missing repository GitLab ID values"
				return gitlab_repositories.NewEnableGitLabRepositoryBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequest(reqID, msg))
			}

			log.WithFields(f).Debugf("assigning GitLab repository for project: %s and CLA Group: %s", params.ProjectSFID, params.GitlabRepositoriesEnable.ClaGroupID)
			enableErr := service.GitLabEnableRepositories(ctx, params.GitlabRepositoriesEnable.ClaGroupID, params.GitlabRepositoriesEnable.RepositoryGitlabIds)
			if enableErr != nil {
				msg := fmt.Sprintf("problem adding GitLab repositories for projectSFID: %s", params.ProjectSFID)
				log.WithFields(f).WithError(enableErr).Warn(msg)
				return gitlab_repositories.NewEnableGitLabRepositoryBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, enableErr))
			}

			repoList, getErr := service.GitLabGetRepositoriesByProjectSFID(ctx, params.ProjectSFID)
			if getErr != nil {
				msg := fmt.Sprintf("problem fetching GitLab repositories for projectSFID: %s", params.ProjectSFID)
				log.WithFields(f).WithError(getErr).Warn(msg)
				return gitlab_repositories.NewEnableGitLabRepositoryBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, getErr))
			}

			return gitlab_repositories.NewEnableGitLabRepositoryOK().WithPayload(repoList)
		})

	api.GitlabRepositoriesUnenrollGitLabRepositoryHandler = gitlab_repositories.UnenrollGitLabRepositoryHandlerFunc(
		func(params gitlab_repositories.UnenrollGitLabRepositoryParams, authUser *auth.User) middleware.Responder {
			reqID := utils.GetRequestID(params.XREQUESTID)
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			ctx := utils.ContextWithRequestAndUser(params.HTTPRequest.Context(), reqID, authUser) // nolint
			f := logrus.Fields{
				"functionName":         "v2.repositories.handlers.GitlabRepositoriesDeleteProjectGitLabRepositoryHandler",
				utils.XREQUESTID:       ctx.Value(utils.XREQUESTID),
				"authUser":             authUser.UserName,
				"authEmail":            authUser.Email,
				"projectSFID":          params.ProjectSFID,
				"repositoryExternalID": params.RepositoryExternalID,
			}

			// Load the project
			psc := project_service.GetClient()
			projectModel, err := psc.GetProject(params.ProjectSFID)
			if err != nil || projectModel == nil {
				return gitlab_repositories.NewUnenrollGitLabRepositoryNotFound().WithPayload(
					utils.ErrorResponseNotFound(reqID, fmt.Sprintf("unable to locate project with ID: %s", params.ProjectSFID)))
			}

			if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ProjectSFID, utils.ALLOW_ADMIN_SCOPE) {
				msg := fmt.Sprintf("user %s does not have access to Unenroll Gitlab Repositories with Project scope of %s",
					authUser.UserName, params.ProjectSFID)
				log.WithFields(f).Debug(msg)
				return gitlab_repositories.NewUnenrollGitLabRepositoryForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
			}

			ghRepo, err := service.GitLabGetRepositoryByExternalID(ctx, params.RepositoryExternalID)
			if err != nil {
				if _, ok := err.(*utils.GitLabRepositoryNotFound); ok {
					msg := fmt.Sprintf("repository not found for repository external ID: %d", params.RepositoryExternalID)
					log.WithFields(f).WithError(err).Warn(msg)
					return gitlab_repositories.NewUnenrollGitLabRepositoryNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
				}

				msg := fmt.Sprintf("problem looking up repository using the repostiory external ID: %d", params.RepositoryExternalID)
				log.WithFields(f).WithError(err).Warn(msg)
				return gitlab_repositories.NewUnenrollGitLabRepositoryBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			err = service.GitLabDisableRepository(ctx, "", params.RepositoryExternalID)
			if err != nil {
				msg := fmt.Sprintf("problem disabling repository for projectSFID: %s, error: %+v", params.ProjectSFID, err)
				log.WithFields(f).WithError(err).Warn(msg)
				return gitlab_repositories.NewUnenrollGitLabRepositoryBadRequest().WithXRequestID(reqID).WithPayload(utils.ErrorResponseBadRequestWithError(reqID, msg, err))
			}

			eventService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType:   events.RepositoryDisabled,
				ProjectSFID: params.ProjectSFID,
				LfUsername:  authUser.UserName,
				EventData: &events.RepositoryDisabledEventData{
					RepositoryName:       ghRepo.RepositoryName,
					RepositoryExternalID: ghRepo.RepositoryExternalID,
				},
			})

			return gitlab_repositories.NewUnenrollGitLabRepositoryNoContent().WithXRequestID(reqID)
		})
}
