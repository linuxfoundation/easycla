// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package users

import (
	"fmt"

	"github.com/go-openapi/strfmt"

	"github.com/sirupsen/logrus"

	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/users"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
)

// Configure setups handlers on api with service
func Configure(api *operations.ClaAPI, service Service, eventsService events.Service) {

	// Create user handler
	api.UsersAddUserHandler = users.AddUserHandlerFunc(func(params users.AddUserParams, claUser *user.CLAUser) middleware.Responder {
		exitingModel, getErr := service.GetUserByUserName(claUser.LFUsername, true)
		if getErr != nil {
			msg := fmt.Sprintf("Error querying the user by username, error: %+v", getErr)
			log.Warnf("Create User Failed - %s", msg)
			return users.NewAddUserBadRequest().WithPayload(&models.ErrorResponse{
				Code:    "500",
				Message: msg,
			})
		}

		// If the user with the same name exists...
		if exitingModel != nil {
			msg := fmt.Sprintf("User with same username exists: %s", claUser.LFUsername)
			log.Warnf("Create User Failed - %s", msg)
			return users.NewAddUserConflict().WithPayload(&models.ErrorResponse{
				Code:    "409",
				Message: msg,
			})
		}

		newUser := &models.User{
			LfEmail:    strfmt.Email(claUser.LFEmail),
			LfUsername: claUser.LFUsername,
			Username:   claUser.Name,
		}
		userModel, err := service.CreateUser(newUser, claUser)
		if err != nil {
			log.Warnf("error creating user from user: %+v, error: %+v", newUser, err)
			return users.NewAddUserBadRequest().WithPayload(errorResponse(err))
		}
		// filling userID in claUser for logging event
		claUser.UserID = userModel.UserID

		return users.NewAddUserOK().WithPayload(userModel)
	})

	// Save/Update User Handler
	api.UsersUpdateUserHandler = users.UpdateUserHandlerFunc(func(params users.UpdateUserParams, claUser *user.CLAUser) middleware.Responder {
		f := logrus.Fields{"" +
			"functionName": "UpdateUserHandlerFunc",
			"authenticatedUserLFUsername": claUser.LFUsername,
			"authenticatedUserLFEmail":    claUser.LFEmail,
			"authenticatedUserUserID":     claUser.UserID,
			"authenticatedUserName":       claUser.Name,
			"paramsBodyLfUsername":        params.Body.LfUsername,
			"paramsBodyLfEmail":           params.Body.LfEmail,
			"paramsBodyGitHubID":          params.Body.GithubID,
			"paramsBodyGitHubUsername":    params.Body.GithubUsername,
			"paramsBodyUsername":          params.Body.Username,
		}
		// Update supports two scenarios:
		// 1) user has LF login and their record has the LF login as part of their existing User record - should find and match - OK, otherwise permission denied
		// 2) user has new LF login and their record does not have the LF login as part of their existing User record - need to lookup by other means, such as GitHub Username
		//   option 2 can happen when GH user gets a user record auto-created and later they need a login for v2 (create company, etc.)
		//   option 2 will be called after they create their login to update their user record with the new login details
		//   option 2 we will search by github username to find the old record - but we can't compare LF login with the existing record because it won't be set yet

		// Check to see if the provided payload includes a GH Username we can use for locating the record
		if params.Body.GithubUsername != "" {
			// Locate the user record by the user's github user name
			log.WithFields(f).Debugf("searching user by GitHubUsername: %s", params.Body.GithubUsername)
			userModel, err := service.GetUserByGitHubUsername(params.Body.GithubUsername)
			if err != nil {
				log.WithFields(f).Warnf("error locating user from by github username: %s, error: %+v",
					params.Body.GithubUsername, err)
				return users.NewUpdateUserBadRequest().WithPayload(errorResponse(err))
			}

			// Found it!
			if userModel != nil {
				// Update the record base on the specified values
				log.WithFields(f).Debugf("found user by GitHubUsername: %s - updating record",
					params.Body.GithubUsername)
				userModel, err := service.Save(params.Body, claUser)
				if err != nil {
					log.WithFields(f).Warnf("error updating user from user request with body: %+v, error: %+v",
						params.Body, err)
					return users.NewUpdateUserBadRequest().WithPayload(errorResponse(err))
				}

				return users.NewUpdateUserOK().WithPayload(userModel)
			}
		}

		// Locate the user record by the user's LF Login ID
		// Check to see if the provided payload includes a GH Username we can use for locating the record
		if params.Body.LfUsername != "" {
			log.WithFields(f).Debugf("searching user by LFUserName: %s", claUser.LFUsername)
			userModel, err := service.GetUserByLFUserName(claUser.LFUsername)
			if err != nil {
				log.WithFields(f).Warnf("error updating user from user request with body: %+v, error: %+v", params.Body, err)
				return users.NewUpdateUserBadRequest().WithPayload(errorResponse(err))
			}

			// If we found the record, but the LF Login ID's don't match up...
			if userModel != nil && claUser.LFUsername != params.Body.LfUsername {
				return users.NewUpdateUserUnauthorized().WithPayload(errorResponse(
					fmt.Errorf("user: %s not authorized to update user: %s", claUser.LFUsername, params.Body.LfUsername)))
			}

			// Found the record and the LF Login ID's match...safe to update
			if userModel != nil && claUser.LFUsername == params.Body.LfUsername {
				log.WithFields(f).Debugf("found user by LFUserName: %s - updating record", claUser.LFUsername)
				userModel, err := service.Save(params.Body, claUser)
				if err != nil {
					log.WithFields(f).Warnf("error updating user from user request with body: %+v, error: %+v", params.Body, err)
					return users.NewUpdateUserBadRequest().WithPayload(errorResponse(err))
				}

				return users.NewUpdateUserOK().WithPayload(userModel)
			}
		}

		// Fall through - couldn't lookup the record
		return users.NewUpdateUserNotFound().WithPayload(&models.ErrorResponse{
			Code:    "404",
			Message: "unable to locate user by LF login or GitHub username",
		})
	})

	// Delete User Handler
	api.UsersDeleteUserHandler = users.DeleteUserHandlerFunc(func(params users.DeleteUserParams, claUser *user.CLAUser) middleware.Responder {
		/*
				if claUser.UserID == "" {
					return users.NewDeleteUserUnauthorized().WithPayload(errorResponse(
						fmt.Errorf("auth - UsersDeleteUserHandler - user %+v not authorized to delete users - missing UserID", claUser)))
				}


			// Let's lookup the authenticated user in our database - we need to see if they have admin access
			claUserModel, err := service.GetUser(claUser.UserID)
			if err != nil || claUserModel == nil {
				return users.NewUpdateUserUnauthorized().WithPayload(errorResponse(
					fmt.Errorf("error looking up current user permissions to determine if delete is allowed, id: %s, error: %+v",
						params.UserID, err)))
			}

			// Should be an admin to delete
			if !claUserModel.Admin {
				return users.NewUpdateUserUnauthorized().WithPayload(errorResponse(
					fmt.Errorf("user with id: %s is not authorized to delete users - must be admin",
						params.UserID)))
			}
		*/

		err := service.Delete(params.UserID, claUser)
		if err != nil {
			log.Warnf("error deleting user from user table with id: %s, error: %+v", params.UserID, err)
			return users.NewUpdateUserBadRequest().WithPayload(errorResponse(err))
		}

		return users.NewDeleteUserNoContent()
	})

	// Get User by ID handler
	api.UsersGetUserHandler = users.GetUserHandlerFunc(func(params users.GetUserParams, claUser *user.CLAUser) middleware.Responder {
		if claUser.UserID == "" {
			return users.NewGetUserUnauthorized().WithPayload(errorResponse(
				fmt.Errorf("auth - UsersGetUserHandler - user %+v not authorized to get users - missing UserID", claUser)))
		}

		userModel, err := service.GetUser(params.UserID)
		if err != nil {
			log.Warnf("error retrieving user for user_id: %s, error: %+v", params.UserID, err)
			return users.NewGetUserBadRequest().WithPayload(errorResponse(err))
		}

		return users.NewGetUserOK().WithPayload(userModel)
	})

	// Get User by name handler
	api.UsersGetUserByUserNameHandler = users.GetUserByUserNameHandlerFunc(func(params users.GetUserByUserNameParams, claUser *user.CLAUser) middleware.Responder {

		userModel, err := service.GetUserByUserName(params.UserName, true)
		if err != nil {
			log.Warnf("error retrieving user for user name: '%s', error: %+v", params.UserName, err)
			return users.NewGetUserByUserNameBadRequest().WithPayload(errorResponse(err))
		}

		if userModel == nil {
			log.Warnf("Get User By User Name - '%s' was not found", params.UserName)
			return users.NewGetUserByUserNameNotFound()
		}

		return users.NewGetUserByUserNameOK().WithPayload(userModel)
	})

	// Get User by name handler
	api.UsersSearchUsersHandler = users.SearchUsersHandlerFunc(func(params users.SearchUsersParams, claUser *user.CLAUser) middleware.Responder {
		// Make sure we have good non-empty parameters
		if claUser.UserID == "" {
			return users.NewUpdateUserUnauthorized().WithPayload(errorResponse(
				fmt.Errorf("auth - UsersSearchUsersHandler - user %+v not authorized to search users - missing UserID", claUser)))
		}

		// No required params? Return empty result
		if params.SearchField == nil || params.SearchTerm == nil {
			return users.NewSearchUsersBadRequest().WithPayload(&models.ErrorResponse{
				Code:    "400",
				Message: "Missing searchField and/or searchTerm parameters",
			})
		}

		// Were we passed the full match flag? If so, use it.
		var fullMatch = false
		if params.FullMatch != nil {
			fullMatch = *params.FullMatch
		}

		// Perform the search
		userModel, err := service.SearchUsers(*params.SearchField, *params.SearchTerm, fullMatch)
		if err != nil {
			log.Warnf("error retrieving user for user with name: '%s', error: %+v", *params.SearchTerm, err)
			return users.NewSearchUsersBadRequest().WithPayload(errorResponse(err))
		}

		if userModel == nil {
			log.Warnf("Get User By User Name - '%s' was not found", *params.SearchTerm)
			return users.NewSearchUsersNotFound()
		}

		return users.NewSearchUsersOK().WithPayload(userModel)

	})

	// Get User by ID compat handler
	api.UsersGetUserCompatHandler = users.GetUserCompatHandlerFunc(func(params users.GetUserCompatParams) middleware.Responder {
		f := logrus.Fields{"" +
			"functionName": "users.GetUserCompatHandlerFunc",
			"paramsUserID": params.UserID,
		}

		userModel, err := service.GetUser(params.UserID)
		if err != nil {
			log.WithFields(f).Warnf("error retrieving user for user_id: %s, error: %+v", params.UserID, err)
			return users.NewGetUserCompatBadRequest().WithPayload(errorResponse(err))
		}

		compatModel, err := service.ConvertUserModelToUserCompatModel(userModel)
		if err != nil {
			log.WithFields(f).Warnf("error converting user model to compat model for user_id: %s, error: %+v", params.UserID, err)
			return users.NewGetUserCompatBadRequest().WithPayload(errorResponse(err))
		}
		log.WithFields(f).Debugf("returning compat user model %+v from user model %+v for user_id: %s", compatModel, userModel, params.UserID)
		return users.NewGetUserCompatOK().WithPayload(compatModel)
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
