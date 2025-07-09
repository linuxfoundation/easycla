// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"
	"sort"

	"github.com/linuxfoundation/easycla/cla-backend-go/gerrits"
	v1Project "github.com/linuxfoundation/easycla/cla-backend-go/project/service"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	v2Repositories "github.com/linuxfoundation/easycla/cla-backend-go/v2/repositories"

	projectService "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
	v2ProjectServiceModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service/models"

	"github.com/sirupsen/logrus"

	"github.com/jinzhu/copier"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/LF-Engineering/lfx-kit/auth"

	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	v1ProjectOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/project"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/project"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

// Configure establishes the middleware handlers for the project service
func Configure(api *operations.EasyclaAPI, service v1Project.Service, v2Service Service, eventsService events.Service, projectsClaGroupsService projects_cla_groups.Service, v2RepositoriesService v2Repositories.ServiceInterface, gerritService gerrits.Service) { //nolint
	// Get Projects
	api.ProjectGetProjectsHandler = project.GetProjectsHandlerFunc(func(params project.GetProjectsParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)

		// No auth checks - anyone can request the list of projects
		projects, err := service.GetCLAGroups(ctx, &v1ProjectOps.GetProjectsParams{
			HTTPRequest: params.HTTPRequest,
			FullMatch:   params.FullMatch,
			NextKey:     params.NextKey,
			PageSize:    params.PageSize,
			SearchField: params.SearchField,
			SearchTerm:  params.SearchTerm,
		})
		if err != nil {
			return project.NewGetProjectsBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		result := &models.ClaGroups{}
		err = copier.Copy(result, projects)
		if err != nil {
			return project.NewGetProjectsInternalServerError().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}
		return project.NewGetProjectsOK().WithXRequestID(reqID).WithPayload(result)
	})

	// Get Project By ID
	api.ProjectGetProjectByIDHandler = project.GetProjectByIDHandlerFunc(func(params project.GetProjectByIDParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.project.handlers.ProjectGetProjectByIDHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"projectSFID":    params.ProjectSfdcID,
			"userEmail":      authUser.Email,
			"userName":       authUser.UserName,
		}

		claGroupModel, err := service.GetCLAGroupByID(ctx, params.ProjectSfdcID)
		if err != nil {

			if err.Error() == "project does not exist" {
				return project.NewGetProjectByIDNotFound().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			return project.NewGetProjectByIDBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		if claGroupModel == nil {
			return project.NewGetProjectByIDNotFound().WithXRequestID(reqID)
		}

		if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, claGroupModel.ProjectExternalID, utils.ALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("user '%s' does not have access to Get Project By ID with Project scope of %s",
				authUser.UserName, claGroupModel.ProjectExternalID)
			return project.NewGetProjectByIDForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		result, err := v2ProjectModel(claGroupModel)
		if err != nil {
			msg := fmt.Sprintf("unable to convert CLA Group '%s' with ID: '%s' to a response model", claGroupModel.ProjectName, claGroupModel.ProjectID)
			log.WithFields(f).WithError(err).Warn(msg)
			return project.NewGetProjectByIDInternalServerError().WithXRequestID(reqID).WithPayload(utils.ErrorResponseInternalServerErrorWithError(reqID, msg, err))
		}

		return project.NewGetProjectByIDOK().WithXRequestID(reqID).WithPayload(result)
	})

	api.ProjectGetProjectsByExternalIDHandler = project.GetProjectsByExternalIDHandlerFunc(func(params project.GetProjectsByExternalIDParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.project.handlers.ProjectGetProjectsByExternalIDHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"externalID":     params.ExternalID,
			"userEmail":      authUser.Email,
			"userName":       authUser.UserName,
		}

		if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, params.ExternalID, utils.ALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("user '%s' does not have access to Get Projects By External ID with Project scope of '%s'",
				authUser.UserName, params.ExternalID)
			log.WithFields(f).Debug(msg)
			return project.NewGetProjectsByExternalIDForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		claGroupModel, err := service.GetCLAGroupsByExternalID(ctx, &v1ProjectOps.GetProjectsByExternalIDParams{
			HTTPRequest: params.HTTPRequest,
			ProjectSFID: params.ExternalID,
			NextKey:     params.NextKey,
			PageSize:    params.PageSize,
		})
		if err != nil {
			return project.NewGetProjectsByExternalIDBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		results := &models.ClaGroups{}
		err = copier.Copy(results, claGroupModel)
		if err != nil {
			return project.NewGetProjectsByExternalIDInternalServerError().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}
		if results.Projects == nil {
			msg := fmt.Sprintf("project not found with id: '%s]", params.ExternalID)
			log.WithFields(f).Debug(msg)
			return project.NewGetProjectsByExternalIDNotFound().WithXRequestID(reqID).WithPayload(utils.ErrorResponseNotFound(reqID, msg))
		}
		return project.NewGetProjectsByExternalIDOK().WithXRequestID(reqID).WithPayload(results)
	})

	// Get Project By Name
	api.ProjectGetProjectByNameHandler = project.GetProjectByNameHandlerFunc(func(params project.GetProjectByNameParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		f := logrus.Fields{
			"functionName":   "v2.project.handlers.ProjectGetProjectByNameHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"projectName":    params.ProjectName,
			"userEmail":      authUser.Email,
			"userName":       authUser.UserName,
		}

		claGroupModel, err := service.GetCLAGroupByName(ctx, params.ProjectName)
		if err != nil {
			return project.NewGetProjectByNameBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}
		if claGroupModel == nil {
			return project.NewGetProjectByNameNotFound().WithXRequestID(reqID)
		}

		if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, claGroupModel.ProjectExternalID, utils.ALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("user '%s' does not have access to Get Projects By Name with Project scope of '%s'",
				authUser.UserName, claGroupModel.ProjectExternalID)
			log.WithFields(f).Debug(msg)
			return project.NewGetProjectByNameForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		result, err := v2ProjectModel(claGroupModel)
		if err != nil {
			return project.NewGetProjectByNameInternalServerError().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}
		return project.NewGetProjectByNameOK().WithXRequestID(reqID).WithPayload(result)
	})

	// Delete Project By ID
	api.ProjectDeleteProjectByIDHandler = project.DeleteProjectByIDHandlerFunc(func(params project.DeleteProjectByIDParams, authUser *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		f := logrus.Fields{
			"functionName":   "v2.project.handlers.ProjectDeleteProjectByIDHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"projectSFID":    params.ProjectSfdcID,
			"userEmail":      authUser.Email,
			"userName":       authUser.UserName,
		}
		log.WithFields(f).Debug("Processing delete request")
		utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
		claGroupModel, err := service.GetCLAGroupByID(ctx, params.ProjectSfdcID)
		if err != nil {
			if err == ErrCLAGroupDoesNotExist {
				return project.NewDeleteProjectByIDNotFound().WithXRequestID(reqID)
			}
			return project.NewDeleteProjectByIDBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		if !utils.IsUserAuthorizedForProjectTree(ctx, authUser, claGroupModel.ProjectExternalID, utils.ALLOW_ADMIN_SCOPE) {
			msg := fmt.Sprintf("user '%s' does not have access to Delete Project By ID with Project scope of %s",
				authUser.UserName, claGroupModel.ProjectExternalID)
			log.WithFields(f).Debug(msg)
			return project.NewDeleteProjectByIDForbidden().WithXRequestID(reqID).WithPayload(utils.ErrorResponseForbidden(reqID, msg))
		}

		err = service.DeleteCLAGroup(ctx, params.ProjectSfdcID)
		if err != nil {
			if err == ErrCLAGroupDoesNotExist {
				return project.NewDeleteProjectByIDNotFound()
			}
			return project.NewDeleteProjectByIDBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}
		eventsService.LogEvent(&events.LogEventArgs{
			EventType:     events.CLAGroupDeleted,
			ClaGroupModel: claGroupModel,
			ProjectSFID:   params.ProjectSfdcID,
			LfUsername:    authUser.UserName,
			EventData:     &events.CLAGroupDeletedEventData{},
		})

		return project.NewDeleteProjectByIDNoContent().WithXRequestID(reqID)
	})

	// Update Project By ID
	api.ProjectUpdateProjectHandler = project.UpdateProjectHandlerFunc(func(params project.UpdateProjectParams, user *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		utils.SetAuthUserProperties(user, params.XUSERNAME, params.XEMAIL)
		claGroupModel, err := service.GetCLAGroupByID(ctx, params.Body.ProjectID)
		if err != nil {
			if err == ErrCLAGroupDoesNotExist {
				return project.NewUpdateProjectNotFound()
			}
			return project.NewUpdateProjectNotFound().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}
		if !utils.IsUserAuthorizedForProjectTree(ctx, user, claGroupModel.ProjectExternalID, utils.ALLOW_ADMIN_SCOPE) {
			return project.NewUpdateProjectForbidden().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{
				Code: "403",
				Message: fmt.Sprintf("EasyCLA - 403 Forbidden - user %s does not have access to Update Project By ID with Project scope of %s",
					user.UserName, claGroupModel.ProjectExternalID),
				XRequestID: reqID,
			})
		}

		in, err := v1ProjectModel(&params.Body)
		if err != nil {
			return project.NewUpdateProjectInternalServerError().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		claGroupModel, err = service.UpdateCLAGroup(ctx, in)
		if err != nil {
			if err == ErrCLAGroupDoesNotExist {
				return project.NewUpdateProjectNotFound().WithXRequestID(reqID)
			}
			return project.NewUpdateProjectBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		eventData := &events.CLAGroupUpdatedEventData{
			OldClaGroupName:        claGroupModel.ProjectName,
			OldClaGroupDescription: claGroupModel.ProjectDescription,
		}

		if in.ProjectName != "" {
			eventData.NewClaGroupName = in.ProjectName
		}

		if in.ProjectDescription != "" {
			eventData.NewClaGroupDescription = in.ProjectDescription
		}

		eventsService.LogEvent(&events.LogEventArgs{
			EventType:     events.CLAGroupUpdated,
			ClaGroupModel: claGroupModel,
			LfUsername:    user.UserName,
			EventData:     eventData,
		})

		result, err := v2ProjectModel(claGroupModel)
		if err != nil {
			return project.NewUpdateProjectInternalServerError().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}
		return project.NewUpdateProjectOK().WithXRequestID(reqID).WithPayload(result)
	})

	// Get CLA enabled projects
	api.ProjectGetCLAProjectsByIDHandler = project.GetCLAProjectsByIDHandlerFunc(func(params project.GetCLAProjectsByIDParams, user *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		// No auth checks - anyone including contributors can request
		claProjects, getErr := v2Service.GetCLAProjectsByID(ctx, params.FoundationSFID)
		if getErr != nil {
			return project.NewGetCLAProjectsByIDBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, getErr))
		}

		return project.NewGetCLAProjectsByIDOK().WithXRequestID(reqID).WithPayload(claProjects)
	})

	api.ProjectGetSFProjectInfoByIDHandler = project.GetSFProjectInfoByIDHandlerFunc(func(params project.GetSFProjectInfoByIDParams, user *auth.User) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		f := logrus.Fields{
			"functionName":   "v2.project.handlers.ProjectGetSFProjectInfoByIDHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"projectSFID":    params.ProjectSFID,
			"userEmail":      user.Email,
			"userName":       user.UserName,
		}

		// No auth checks - anyone including contributors can request
		psc := projectService.GetClient()
		sfProject, err := psc.GetProject(params.ProjectSFID)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to lookup SF project by ID")
			return project.NewGetSFProjectInfoByIDBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}

		// Lookup the parent info, if it's available
		var parentName string
		if utils.IsProjectHaveParent(sfProject) {
			sfParentProject, err := psc.GetProject(utils.GetProjectParentSFID(sfProject))
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("unable to load parant project by ID: %s", utils.GetProjectParentSFID(sfProject))
			}

			if sfParentProject != nil {
				parentName = sfParentProject.Name
			}
		}

		summary := buildSFProjectSummary(sfProject, parentName)
		return project.NewGetSFProjectInfoByIDOK().WithXRequestID(reqID).WithPayload(summary)
	})

	api.ProjectGetProjectCompatHandler = project.GetProjectCompatHandlerFunc(func(params project.GetProjectCompatParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(context.Background(), utils.XREQUESTID, reqID) // nolint
		f := logrus.Fields{
			"functionName":   "v2.project.handlers.ProjectGetProjectCompatHandler",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"projectID":      params.ProjectID,
		}

		proj, err := service.GetCLAGroupByID(ctx, params.ProjectID)
		if err != nil {
			if err.Error() == "project does not exist" {
				return project.NewGetProjectCompatNotFound().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
			}
			log.WithFields(f).WithError(err).Warnf("unable to load compat project by ID: %s", params.ProjectID)
			return project.NewGetProjectCompatBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}
		if proj == nil {
			return project.NewGetProjectCompatNotFound().WithXRequestID(reqID)
		}
		projectsClaGroups, err := projectsClaGroupsService.GetProjectsIdsForClaGroup(ctx, params.ProjectID)
		if err != nil {
			return project.NewGetProjectCompatBadRequest().WithXRequestID(reqID).WithPayload(errorResponse(reqID, err))
		}
		sfidReposMap := make(map[string][][2]string)
		for _, prjClaGrp := range projectsClaGroups {
			sfid := prjClaGrp.ProjectSFID
			if sfid == "" {
				continue
			}
			_, ok := sfidReposMap[sfid]
			if ok {
				continue
			}
			repos, reposErr := v2RepositoriesService.GetRepositoriesByProjectSFID(ctx, sfid)
			if reposErr != nil {
				log.WithFields(f).WithError(err).Warnf("unable to get github/gitlab repos list for SFID: %s: %+v", sfid, reposErr)
			}
			gerrits, gerritsErr := gerritService.GetGerritsByProjectSFID(ctx, sfid)
			if gerritsErr != nil {
				log.WithFields(f).WithError(err).Warnf("unable to get gerrit repos list for SFID: %s", sfid, gerritsErr)
			}
			entry := [][2]string{}
			if reposErr == nil {
				for _, repo := range repos {
					entry = append(entry, [2]string{repo.RepositoryType, repo.RepositoryName})
				}
			}
			if gerritsErr == nil {
				for _, repo := range gerrits.List {
					entry = append(entry, [2]string{"gerrit", string(repo.GerritURL)})
				}
			}
			sort.Slice(entry, func(i, j int) bool {
				return entry[i][1] < entry[j][1]
			})
			sfidReposMap[sfid] = entry
		}
		compatProject := buildCompatProject(proj, projectsClaGroups, sfidReposMap)
		return project.NewGetProjectCompatOK().WithXRequestID(reqID).WithPayload(compatProject)
	})
}

func buildCompatProject(project *v1Models.ClaGroup, projectClaGroups []*projects_cla_groups.ProjectClaGroup, sfidReposMap map[string][][2]string) *models.ProjectCompat {
	projectCorporateDocuments := []*models.ProjectCompatProjectCorporateDocumentsItems0{}
	for _, doc := range project.ProjectCorporateDocuments {
		projectCorporateDocuments = append(projectCorporateDocuments, &models.ProjectCompatProjectCorporateDocumentsItems0{
			DocumentMajorVersion: doc.DocumentMajorVersion,
			DocumentMinorVersion: doc.DocumentMinorVersion,
		})
	}
	projectIndividualDocuments := []*models.ProjectCompatProjectIndividualDocumentsItems0{}
	for _, doc := range project.ProjectIndividualDocuments {
		projectIndividualDocuments = append(projectIndividualDocuments, &models.ProjectCompatProjectIndividualDocumentsItems0{
			DocumentMajorVersion: doc.DocumentMajorVersion,
			DocumentMinorVersion: doc.DocumentMinorVersion,
		})
	}
	projects := []*models.ProjectCompatProjectsItems0{}
	for _, prjClaGrp := range projectClaGroups {
		gerritRepos := []*models.ProjectCompatProjectsItems0GerritReposItems0{}
		githubRepos := []*models.ProjectCompatProjectsItems0GithubReposItems0{}
		gitlabRepos := []*models.ProjectCompatProjectsItems0GitlabReposItems0{}
		sfid := prjClaGrp.ProjectSFID
		if sfid != "" {
			repos, ok := sfidReposMap[sfid]
			if ok {
				for _, repo := range repos {
					if repo[0] == "github" {
						githubRepos = append(githubRepos, &models.ProjectCompatProjectsItems0GithubReposItems0{RepositoryName: repo[1]})
					} else if repo[0] == "gitlab" {
						gitlabRepos = append(gitlabRepos, &models.ProjectCompatProjectsItems0GitlabReposItems0{RepositoryName: repo[1]})
					} else if repo[0] == "gerrit" {
						gerritRepos = append(gerritRepos, &models.ProjectCompatProjectsItems0GerritReposItems0{GerritURL: repo[1]})
					}
				}
			}
		}
		projects = append(projects, &models.ProjectCompatProjectsItems0{
			ClaGroupID:     prjClaGrp.ClaGroupID,
			FoundationSfid: prjClaGrp.FoundationSFID,
			ProjectName:    prjClaGrp.ProjectName,
			ProjectSfid:    prjClaGrp.ProjectSFID,
			GerritRepos:    gerritRepos,
			GithubRepos:    githubRepos,
			GitlabRepos:    gitlabRepos,
		})
	}
	// fmt.Printf("GERRITS: %+v\n", *project.Gerrits[0])
	// fmt.Printf("GITHUBS: %+v\n", *project.GithubRepositories[0])
	return &models.ProjectCompat{
		FoundationSfid:                   project.FoundationSFID,
		ProjectName:                      project.ProjectName,
		ProjectCclaEnabled:               project.ProjectCCLAEnabled,
		ProjectCclaRequiresIclaSignature: project.ProjectCCLARequiresICLA,
		ProjectIclaEnabled:               project.ProjectICLAEnabled,
		ProjectID:                        project.ProjectID,
		SignedAtFoundationLevel:          project.FoundationLevelCLA,
		ProjectCorporateDocuments:        projectCorporateDocuments,
		ProjectIndividualDocuments:       projectIndividualDocuments,
		Projects:                         projects,
	}
}

func buildSFProjectSummary(sfProject *v2ProjectServiceModels.ProjectOutputDetailed, parentName string) *models.SfProjectSummary {
	return &models.SfProjectSummary{
		EntityName:   utils.StringValue(sfProject.EntityName),
		EntityType:   sfProject.EntityType,
		Funding:      *sfProject.Funding,
		ID:           sfProject.ID,
		LfSupported:  sfProject.LFSponsored,
		Name:         sfProject.Name,
		ParentID:     utils.GetProjectParentSFID(sfProject),
		ParentName:   parentName,
		Slug:         sfProject.Slug,
		Status:       sfProject.Status,
		Type:         sfProject.Type,
		IsStandalone: utils.IsStandaloneProject(sfProject),
	}
}
