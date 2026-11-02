// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/linuxfoundation/easycla/cla-backend-go/cmd/user-subscribe-lambda/cmd"

	"github.com/go-openapi/strfmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-models/models/event"
	usersModels "github.com/LF-Engineering/lfx-models/models/users"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	user_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service"
	userServiceModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
	"github.com/mitchellh/mapstructure"
)

// Build and version variables defined and set during the build process
var (
	// version the application version
	version string

	// build/Commit the application build number
	commit string

	// build date
	buildDate string
)

// initConfig loads the stage configuration from SSM and initialises the token and user-service clients. It runs first in
// main() rather than in a package init() so that the package's unit tests can be built and run without AWS access.
func initConfig() {
	f := logrus.Fields{
		"functionName": "userSubscribeLambda.main.initConfig",
	}
	var awsSession = session.Must(session.NewSession(&aws.Config{}))
	stage := os.Getenv("STAGE")
	if stage == "" {
		log.WithFields(f).Fatal("stage not set")
	}
	log.WithFields(f).Infof("STAGE set to %s\n", stage)
	configFile, err := config.LoadConfig("", awsSession, stage)
	if err != nil {
		log.WithFields(f).WithError(err).Panicf("Unable to load config - Error: %v", err)
	}

	token.Init(configFile.Auth0Platform.ClientID, configFile.Auth0Platform.ClientSecret, configFile.Auth0Platform.URL, configFile.Auth0Platform.Audience)
	user_service.InitClient(configFile.APIGatewayURL, configFile.AcsAPIKey)
}

// Handler is the user subscribe handler lambda entry function
func Handler(ctx context.Context, snsEvent events.SNSEvent) error {
	f := logrus.Fields{
		"functionName":   "userSubscribeLambda.main.Handler",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	if len(snsEvent.Records) == 0 {
		log.WithFields(f).Warn("SNS event contained 0 records - ignoring message.")
		return nil
	}

	for _, message := range snsEvent.Records {
		log.WithFields(f).Infof("Processing message id: '%s' for event source '%s'", message.SNS.MessageID, message.EventSource)

		log.WithFields(f).Debugf("Unmarshalling message body: '%s'", message.SNS.Message)
		var model event.Event
		err := model.UnmarshalBinary([]byte(message.SNS.Message))
		if err != nil {
			log.WithFields(f).Warnf("Error: %v, JSON unmarshal failed - unable to process message: %s", err, message.SNS.MessageID)
			return err
		}

		f["modelType"] = model.Type
		log.WithFields(f).Debugf("Processing message type: %s", model.Type)
		switch model.Type {
		case "UserSignedUp":
			log.WithFields(f).Debugf("Detected message type: %s - processing...", model.Type)
			// a storage failure is returned so the asynchronous invocation is retried instead of the sign-up being lost
			if createErr := Create(ctx, model); createErr != nil {
				return createErr
			}
		case "UserUpdatedProfile":
			log.WithFields(f).Debugf("Detected message type: %s - processing...", model.Type)
			Update(ctx, model)
		case "UserAuthenticated":
			log.WithFields(f).Debugf("Ignoring message type: %s", model.Type)
		default:
			log.WithFields(f).Warnf("unrecognized message type: %s - unable to process message ", model.Type)
		}

	}
	return nil
}

// Create saves the user data model to persistent storage. It returns the error of a failed EasyCLA lookup or write (the
// event is worth a retry); an undecodable event is logged and dropped.
func Create(ctx context.Context, user event.Event) error {
	f := logrus.Fields{
		"functionName":   "userSubscribeLambda.main.Create",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	uc := &usersModels.UserCreated{}
	err := mapstructure.Decode(user.Data, uc)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to decode event")
		return nil
	}

	var awsSession = session.Must(session.NewSession(&aws.Config{}))

	stage := os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("stage not set")
	}
	usersRepo := users.NewRepository(awsSession, stage)

	return createUser(f, uc.Username, usersRepo, user_service.GetClient().GetUserByUsername)
}

// userLookup resolves an LF username to its user-service record (user_service.Client.GetUserByUsername in production)
type userLookup func(lfUsername string) (*userServiceModels.User, error)

// createUser adds the EasyCLA user record for a UserSignedUp event of `username`, filled from the user-service record.
// A username that already has a record is left alone: profile changes arrive as UserUpdatedProfile and are applied by Update.
// A failed EasyCLA lookup does not prove the user is absent, so nothing is created and the error is returned for a retry;
// a failed write is returned as well. Every other outcome is final and returns nil.
func createUser(f logrus.Fields, username string, usersRepo users.UserRepository, lookup userLookup) error {
	log.WithFields(f).Debugf("locating user by username: %s in EasyCLA's database...", username)
	userDetails, userErr := usersRepo.GetUserByLFUserName(username)
	if userErr != nil {
		log.WithFields(f).WithError(userErr).Warnf("unable to locate user by LfUsername: %s", username)
		return userErr
	}

	if userDetails != nil {
		log.WithFields(f).Warnf("unable to create user - user already created: %s", username)
		return nil
	}

	log.WithFields(f).Debugf("locating user by username: %s in the user service...", username)
	sfdcUserObject, err := lookup(username)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to locate user by username: %s", username)
		return nil
	}
	if sfdcUserObject == nil {
		log.WithFields(f).Debugf("User-service model is nil so skipping user %s", username)
		return nil
	}

	log.WithFields(f).Debugf("Salesforce user-service object : %+v", sfdcUserObject)

	var primaryEmail string
	var emails []string
	for _, email := range sfdcUserObject.Emails {
		if email == nil || email.EmailAddress == nil {
			continue
		}
		if email.IsPrimary != nil && *email.IsPrimary {
			primaryEmail = *email.EmailAddress
		}
		emails = append(emails, *email.EmailAddress)
	}

	_, nowStr := utils.CurrentTime()
	// no UserID: a brand-new user has no EasyCLA record to take it from, the repository assigns a fresh one (ops#888)
	createUserModel := &models.User{
		Admin:          false,
		DateCreated:    nowStr,
		DateModified:   nowStr,
		Emails:         emails,
		LfEmail:        strfmt.Email(primaryEmail),
		LfUsername:     sfdcUserObject.Username,
		Note:           "Create via user-service event",
		UserExternalID: sfdcUserObject.ID,
		Username:       fmt.Sprintf("%s %s", sfdcUserObject.FirstName, sfdcUserObject.LastName),
		Version:        "v1",
	}

	log.WithFields(f).Debugf("Creating user in Dynamo DB : %+v", createUserModel)
	_, createErr := usersRepo.CreateUser(createUserModel)
	if createErr != nil {
		log.WithFields(f).WithError(createErr).Warnf("unable to create user by LfUsername: %s", username)
		return createErr
	}
	return nil
}

// Update saves the user data model to persistent storage
func Update(ctx context.Context, user event.Event) {
	f := logrus.Fields{
		"functionName":   "userSubscribeLambda.main.Update",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	uc := &usersModels.UserUpdated{}
	err := mapstructure.Decode(user.Data, uc)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to decode event")
		return
	}

	var userDetails *models.User
	var userErr error
	var awsSession = session.Must(session.NewSession(&aws.Config{}))

	stage := os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("stage not set")
	}
	usersRepo := users.NewRepository(awsSession, stage)

	userDetails, userErr = usersRepo.GetUserByLFUserName(*uc.Username)
	if userErr != nil {
		log.WithFields(f).WithError(userErr).Warnf("unable to locate user by LfUsername: %s", *uc.Username)
	}

	if userDetails == nil {
		for _, email := range uc.Emails {
			userDetails, userErr = usersRepo.GetUserByEmail(*email.EmailAddress)
			if userErr != nil {
				log.WithFields(f).WithError(userErr).Warnf("unable to locate user by LfUsername: %s", *uc.Username)
			}
		}
	}

	if userDetails == nil {
		userDetails, userErr = usersRepo.GetUserByExternalID(uc.UserID)
		if userErr != nil {
			log.WithFields(f).WithError(userErr).Warnf("unable to locate user by UserExternalID: %s", uc.UserID)
		}
	}

	if userDetails == nil {
		log.WithFields(f).Debugf("User model is nil - adding as new user %s...", *uc.Username)
		// Attempt to create the user from the upate model
		createFromUpdateErr := createUserFromUpdatedModel(uc)
		if createFromUpdateErr != nil {
			log.WithFields(f).WithError(createFromUpdateErr).Warnf("unable to create new user record from user service update message: %s", uc.UserID)
		}
		return
	}

	userServiceClient := user_service.GetClient()
	sfdcUserObject, err := userServiceClient.GetUser(uc.UserID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to locate user by SFID: %s, error: %+v", uc.UserID, userErr)
		return
	}

	log.WithFields(f).Debugf("Salesforce user-service object : %+v", sfdcUserObject)

	if sfdcUserObject == nil {
		log.WithFields(f).Debugf("User-service model is nil so skipping user %s with SFID %s", *uc.Username, uc.UserID)
		return
	}

	var primaryEmail string
	var emails []string
	for _, email := range sfdcUserObject.Emails {
		if *email.IsPrimary {
			primaryEmail = *email.EmailAddress
		}
		emails = append(emails, *email.EmailAddress)
	}

	updateUserModel := &models.UserUpdate{
		Emails:         emails,
		LfEmail:        primaryEmail,
		LfUsername:     sfdcUserObject.Username,
		Note:           "Update via user-service event",
		UserExternalID: sfdcUserObject.ID,
		UserID:         userDetails.UserID,
		Username:       fmt.Sprintf("%s %s", sfdcUserObject.FirstName, sfdcUserObject.LastName),
	}

	log.WithFields(f).Debugf("Updating user in Dynamo DB : %+v", updateUserModel)
	_, updateErr := usersRepo.Save(updateUserModel)
	if updateErr != nil {
		log.WithFields(f).Warnf("Error - unable to update user by LfUsername: %s, error: %+v", *uc.Username, updateErr)
		return
	}
}

func createUserFromUpdatedModel(userModelUpdated *usersModels.UserUpdated) error {
	f := logrus.Fields{
		"functionName": "userSubscribeLambda.main.createUserFromUpdatedModel",
		"userID":       userModelUpdated.UserID,
		"userName":     userModelUpdated.Username,
	}

	var awsSession = session.Must(session.NewSession(&aws.Config{}))

	stage := os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("stage not set")
	}
	userServiceClient := user_service.GetClient()
	sfdcUserObject, err := userServiceClient.GetUser(userModelUpdated.UserID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to locate user by ID: %s", userModelUpdated.UserID)
		return err
	}

	var primaryEmail string
	var emails []string
	for _, email := range sfdcUserObject.Emails {
		if *email.IsPrimary {
			primaryEmail = *email.EmailAddress
		}
		emails = append(emails, *email.EmailAddress)
	}

	newUserModel := &models.User{
		Emails:         emails,
		LfEmail:        strfmt.Email(primaryEmail),
		LfUsername:     sfdcUserObject.Username,
		Note:           "Update via user-service event",
		UserExternalID: sfdcUserObject.ID,
		UserID:         userModelUpdated.UserID,
		Username:       fmt.Sprintf("%s %s", sfdcUserObject.FirstName, sfdcUserObject.LastName),
	}

	log.WithFields(f).Debugf("Creating user in Dynamo DB : %+v", newUserModel)
	usersRepo := users.NewRepository(awsSession, stage)

	_, createErr := usersRepo.CreateUser(newUserModel)
	if createErr != nil {
		log.WithFields(f).WithError(createErr).Warnf("unable to create user by LfUsername: %s", *userModelUpdated.Username)
		return createErr
	}

	return nil
}

func main() {
	f := logrus.Fields{
		"functionName": "userSubscribeLambda.main.main",
	}
	var err error

	initConfig()

	// Show the version and build info
	log.WithFields(f).Infof("Name                  : userSubscribe handler")
	log.WithFields(f).Infof("Version               : %s", version)
	log.WithFields(f).Infof("Git commit hash       : %s", commit)
	log.WithFields(f).Infof("Build date            : %s", buildDate)
	log.WithFields(f).Infof("Golang OS             : %s", runtime.GOOS)
	log.WithFields(f).Infof("Golang Arch           : %s", runtime.GOARCH)

	err = cmd.Start(Handler)
	if err != nil {
		log.WithFields(f).WithError(err).Fatal(err)
	}
}
