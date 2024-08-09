// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package user_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/v2/user-service/client/staff"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/communitybridge/easycla/cla-backend-go/token"
	"github.com/communitybridge/easycla/cla-backend-go/v2/user-service/client"
	"github.com/communitybridge/easycla/cla-backend-go/v2/user-service/client/user"
	"github.com/communitybridge/easycla/cla-backend-go/v2/user-service/models"
	runtimeClient "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

var (
	// ErrUserNotFound is an error for users not found
	ErrUserNotFound = errors.New("user not found")
)

// Client is client for user_service
type Client struct {
	cl       *client.UserServiceAPI
	apiKey   string
	apiGwURL string
}

var (
	userServiceClient *Client
)

// InitClient initializes the user_service client
func InitClient(APIGwURL string, apiKey string) {
	APIGwURL = strings.ReplaceAll(APIGwURL, "https://", "")
	userServiceClient = &Client{
		apiKey:   apiKey,
		apiGwURL: APIGwURL,
		cl: client.NewHTTPClientWithConfig(strfmt.Default, &client.TransportConfig{
			Host:     APIGwURL,
			BasePath: "user-service/v1",
			Schemes:  []string{"https"},
		}),
	}
}

// GetClient return user_service client
func GetClient() *Client {
	return userServiceClient
}

// GetUsersByUsernames search users by lf username
func (usc *Client) GetUsersByUsernames(lfUsernames []string) ([]*models.User, error) {
	f := logrus.Fields{
		"functionName": "GetUsersByUsernames",
		"lfUsernames":  strings.Join(lfUsernames, ","),
	}

	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem obtaining token")
		return nil, err
	}

	url := fmt.Sprintf("https://%s/user-service/v1/bulk", usc.apiGwURL)
	var requestBody = models.SearchBulk{
		Type: aws.String("username"),
		List: lfUsernames,
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem marshalling the request body")
		return nil, err
	}

	request, err := http.NewRequest("POST", url, strings.NewReader(string(requestBodyBytes)))

	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem building new request")
		return nil, err
	}

	request.Header.Set("X-API-KEY", usc.apiKey)
	request.Header.Set("Authorization", "Bearer "+tok)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem searching user")
		return nil, err
	}

	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			log.WithFields(f).WithError(closeErr).Warn("error closing body")
		}

	}()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem decoding the user response")
		return nil, err
	}

	//return as []*models.User
	userList, err := getUsers(data)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem processing the user response")
		return nil, err
	}

	return userList, nil
}

// GetUserByUsername returns user by lfUsername
func (usc *Client) GetUserByUsername(lfUsername string) (*models.User, error) {
	f := logrus.Fields{
		"functionName": "GetUserByUsername",
		"lfUsername":   lfUsername,
	}

	// use the ListUsers API endpoint (actually called FindUsers) with the lfUsername filter
	userModel, err := usc.ListUsersByUsername(lfUsername)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading user by username")
		return nil, err
	}
	if userModel == nil {
		log.WithFields(f).Debug("get by username returned no results")
		return nil, ErrUserNotFound
	}

	return userModel, nil
}

// SearchUsers returns a single user based on firstName, lastName and email parameters
func (usc *Client) SearchUsers(firstName string, lastName string, email string) (*models.User, error) {
	f := logrus.Fields{
		"functionName": "SearchUsers",
		"firstName":    firstName,
		"lastName":     lastName,
		"email":        email,
	}

	// TODO: DAD - let's replace this with the client sub implementation rather than a manual HTTP request
	query := fmt.Sprintf("email=%s&firstname=%s&lastname=%s", email, firstName, lastName)
	url := fmt.Sprintf("https://%s/user-service/v1/users/search?%s", usc.apiGwURL, query)
	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem searching user")
		return nil, err
	}
	log.WithFields(f).Debug("searching for user...")
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem building new request")
		return nil, err
	}

	request.Header.Set("X-API-KEY", usc.apiKey)
	request.Header.Set("Authorization", "Bearer "+tok)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem searching user")
		return nil, err
	}

	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			log.WithFields(f).WithError(closeErr).Warn("error closing body")
		}
	}()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem decoding the user response")
		return nil, err
	}
	userList, err := getUsers(data)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem processing the user response")
		return nil, err
	}

	for _, userItem := range userList {
		for _, userEmail := range userItem.Emails {
			if *userEmail.EmailAddress == email {
				if userItem.FirstName == firstName && userItem.LastName == lastName {
					return userItem, nil
				}
			}
		}
	}

	return nil, errors.New("user not found")
}

// ListUsersByUsername returns the username
func (usc *Client) ListUsersByUsername(lfUsername string) (*models.User, error) {
	f := logrus.Fields{
		"functionName": "ListUsersByUsername",
		"lfUsername":   lfUsername,
	}

	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem obtaining token")
		return nil, err
	}

	url := fmt.Sprintf("https://%s/user-service/v1/users?username=%s", usc.apiGwURL, lfUsername)
	request, err := http.NewRequest("GET", url, nil)

	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem building new request")
		return nil, err
	}

	request.Header.Set("X-API-KEY", usc.apiKey)
	request.Header.Set("Authorization", "Bearer "+tok)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem searching user")
		return nil, err
	}

	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			log.WithFields(f).WithError(closeErr).Warn("error closing body")
		}
	}()

	data, err := io.ReadAll(response.Body)

	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem decoding the user response")
		return nil, err
	}

	userList, err := getUsers(data)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem processing the user response")
		return nil, err
	}

	if len(userList) == 0 {
		log.WithFields(f).Debug("get by lfUsername returned no results")
		return nil, ErrUserNotFound
	}

	return userList[0], nil
}

// SearchUsersByEmail returns a single user based on the email parameter
func (usc *Client) SearchUsersByEmail(email string) (*models.User, error) {
	f := logrus.Fields{
		"functionName": "SearchUsersByEmail",
		"email":        email,
	}

	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem obtaining token")
		return nil, err
	}

	url := fmt.Sprintf("https://%s/user-service/v1/users?email=%s", usc.apiGwURL, email)
	request, err := http.NewRequest("GET", url, nil)

	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem building new request")
		return nil, err
	}

	request.Header.Set("X-API-KEY", usc.apiKey)
	request.Header.Set("Authorization", "Bearer "+tok)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem searching user")
		return nil, err
	}

	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			log.WithFields(f).WithError(closeErr).Warn("error closing body")
		}
	}()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem decoding the user response")
		return nil, err
	}

	userList, err := getUsers(data)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem processing the user response")
		return nil, err
	}

	if len(userList) == 0 {
		log.WithFields(f).Debug("get by lfUsername returned no results")
		return nil, ErrUserNotFound
	}

	return userList[0], nil

}

func getUsers(body []byte) ([]*models.User, error) {
	var users = new(models.UserList)
	err := json.Unmarshal(body, &users)
	if err != nil {
		return nil, err
	}
	return users.Data, err
}

// ConvertToContact converts user to contact from lead
func (usc *Client) ConvertToContact(userSFID string) error {
	params := &user.ConvertToContactParams{
		SalesforceID: userSFID,
		Context:      context.Background(),
	}
	tok, err := token.GetToken()
	if err != nil {
		return err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	_, _, err = usc.cl.User.ConvertToContact(params, clientAuth) //nolint
	if err != nil {
		return err
	}
	return nil
}

// GetUser returns user from user-service
func (usc *Client) GetUser(userSFID string) (*models.User, error) {
	params := &user.GetUserParams{
		SalesforceID: userSFID,
		Context:      context.Background(),
	}
	tok, err := token.GetToken()
	if err != nil {
		return nil, err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	result, err := usc.cl.User.GetUser(params, clientAuth)
	if err != nil {
		return nil, err
	}
	return result.Payload, nil
}

// GetStaff returns staff details from user-service
func (usc *Client) GetStaff(userSFID string) (*models.Staff, error) {
	params := &staff.GetStaffParams{
		SalesforceID: userSFID,
		Context:      context.Background(),
	}
	tok, err := token.GetToken()
	if err != nil {
		return nil, err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	result, err := usc.cl.Staff.GetStaff(params, clientAuth)
	if err != nil {
		return nil, err
	}
	return result.Payload, nil
}

// GetUserEmail returns email of a user given username
func (usc *Client) GetUserEmail(username string) (string, error) {
	user, err := usc.GetUserByUsername(username)
	if err != nil {
		return "", err
	}
	if user != nil && len(user.Emails) > 0 {
		return *user.Emails[0].EmailAddress, nil
	}
	return "", nil
}

// UpdateUserAccount updates users org
func (usc *Client) UpdateUserAccount(userSFID string, orgID string) error {
	f := logrus.Fields{
		"functionName":   "UpdateUserAccount",
		"organizationID": orgID,
		"userSFID":       userSFID,
	}

	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem obtaining token")
		return err
	}

	clientAuth := runtimeClient.BearerToken(tok)

	params := &user.UpdatePartialUserParams{
		SalesforceID: userSFID,
		UpdatePartialUser: &models.UpdatePartialUser{
			AccountID: &orgID,
		},
		Context: context.Background(),
	}

	result, updateErr := usc.cl.User.UpdatePartialUser(params, clientAuth)
	if updateErr != nil {
		log.WithFields(f).WithError(updateErr).Warn("problem updating user")
		return updateErr
	}

	log.WithFields(f).Debugf("successfully updated user: %s", result)
	return nil
}

// GetPrimaryEmail gets user primary email
func (usc *Client) GetPrimaryEmail(user *models.User) string {
	f := logrus.Fields{
		"functionName": "GetPrimaryEmail",
	}
	primaryEmail := ""
	for _, email := range user.Emails {
		if *email.IsPrimary {
			log.WithFields(f).Debugf("Found primary email : %s ", *email.EmailAddress)
			primaryEmail = *email.EmailAddress
		}
	}
	return primaryEmail
}

// EmailsToSlice converts a user model's email addresses to a string slice
func (usc *Client) EmailsToSlice(user *models.User) []string {
	var emailList []string
	for _, email := range user.Emails {
		if email.EmailAddress != nil {
			emailList = append(emailList, *email.EmailAddress)
		}
	}

	return emailList
}
