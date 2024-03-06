// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gerrits

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	v2Models "github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/events"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// constants
const (
	DefaultHTTPTimeout = 10 * time.Second
	LongHTTPTimeout    = 45 * time.Second
)

// LFGroup contains access information of lf LDAP group
type LFGroup struct {
	LfBaseURL     string
	ClientID      string
	ClientSecret  string
	RefreshToken  string
	EventsService events.Service
}

// LDAPGroup model
type LDAPGroup struct {
	Title string `json:"title"`
}

func (lfg *LFGroup) getAccessToken(ctx context.Context) (string, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.lf_group.getAccessToken",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	requestBody, err := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": lfg.RefreshToken,
		"scope":         "manage_groups",
	})
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem encoding access token request")
		return "", err
	}
	OauthURL := fmt.Sprintf("%s/oauth2/token", lfg.LfBaseURL)
	req, err := http.NewRequest("POST", OauthURL, bytes.NewBuffer(requestBody))
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem creating a new request to URL: %s", OauthURL)
		return "", err
	}
	req.SetBasicAuth(lfg.ClientID, lfg.ClientSecret)
	req.Header.Add("Content-Type", "application/json")

	client := http.Client{
		Timeout: DefaultHTTPTimeout,
	}
	res, err := client.Do(req)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem sending a request to URL: %s", OauthURL)
		return "", err
	}

	defer func() {
		closeErr := res.Body.Close()
		if closeErr != nil {
			log.WithFields(f).WithError(closeErr).Warn("error closing response body")
		}
	}()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem reading the response from URL: %s", OauthURL)
		return "", err
	}

	var out struct {
		AccessToken string `json:"access_token"`
	}

	err = json.Unmarshal(body, &out)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem unmarshalling the response from URL: %s", OauthURL)
		return "", err
	}

	return out.AccessToken, nil
}

// GetGroup returns LF LDAP group
func (lfg *LFGroup) GetGroup(ctx context.Context, groupID string) (*LDAPGroup, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.lf_group.GetGroup",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"groupID":        groupID,
	}

	accessToken, err := lfg.getAccessToken(ctx)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading access token")
		return nil, err
	}
	getGroupURL := fmt.Sprintf("%s/rest/auth0/og/%s", lfg.LfBaseURL, groupID)
	req, err := http.NewRequest("GET", getGroupURL, nil)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem creating a new request to URL: %s", getGroupURL)
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := http.Client{
		Timeout: DefaultHTTPTimeout,
	}
	res, err := client.Do(req)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem invoking request to URL: %s", getGroupURL)
		return nil, err
	}

	defer func() {
		closeErr := res.Body.Close()
		if closeErr != nil {
			log.WithFields(f).WithError(closeErr).Warn("error closing response body")
		}
	}()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem reading the response from URL: %s", getGroupURL)
		return nil, err
	}

	var out LDAPGroup
	err = json.Unmarshal(body, &out)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem unmarshalling the response from URL: %s", getGroupURL)
		return nil, err
	}

	return &out, nil
}

// GetUsersOfGroup returns a list of members from a group
func (lfg *LFGroup) GetUsersOfGroup(ctx context.Context, authUser *auth.User, claGroupID, groupName string) (*v2Models.GerritGroupResponse, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.lf_group.GetUsersOfGroup",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"groupName":      groupName,
		"authUserName":   authUser.UserName,
		"authUserEmail":  authUser.Email,
	}

	log.WithFields(f).Debug("getting users of group...")

	// Fetch a token for authorization
	accessToken, err := lfg.getAccessToken(ctx)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading access token")
		return nil, err
	}

	// Build the URL path - can take the groupName or numeric value
	// API Docs: https://confluence.linuxfoundation.org/display/IPM/Drupal+Identity+REST+for+Auth0
	url := fmt.Sprintf("%s/rest/auth0/og/%s", lfg.LfBaseURL, groupName)

	// Set up the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem creating a new request to URL: %s", url)
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+accessToken)
	client := http.Client{
		Timeout: LongHTTPTimeout,
	}

	// Invoke the request
	resp, err := client.Do(req)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem invoking request to URL: %s", url)
		return nil, err
	}

	// Cleanup after
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			log.WithFields(f).WithError(closeErr).Warn("error closing response body")
		}
	}()

	// Check the response code to see how it went - response payload is undefined - just looking for a successful response status code
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		log.WithFields(f).Debugf("successfully fetched members from group: %s", groupName)

		var result v2Models.GerritGroupResponse
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("problem reading response for url: %s", url)
			return nil, err
		}

		log.WithFields(f).Debugf("response body: %+v", string(body))
		err = json.Unmarshal(body, &result)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("problem unmarshalling response for url: %s", url)
			return nil, err
		}

		return &result, nil
	}

	log.WithFields(f).Warnf("error fetching users from group: %s - response status: %d", groupName, resp.StatusCode)
	return nil, nil
}

// AddUserToGroup adds the specified user to the group
func (lfg *LFGroup) AddUserToGroup(ctx context.Context, authUser *auth.User, claGroupID, groupName, userName string) error {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.lf_group.AddUserToGroup",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"groupName":      groupName,
		"userName":       userName,
		"authUserName":   authUser.UserName,
		"authUserEmail":  authUser.Email,
	}

	log.WithFields(f).Debug("adding user to group...")

	// Fetch a token for authorization
	accessToken, err := lfg.getAccessToken(ctx)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading access token")
		return err
	}

	// Build the URL path - can take the groupName or numeric value
	// API Docs: https://confluence.linuxfoundation.org/display/IPM/Drupal+Identity+REST+for+Auth0
	url := fmt.Sprintf("%s/rest/auth0/og/%s", lfg.LfBaseURL, groupName)

	// Build the request payload
	payload := map[string]interface{}{
		"username": userName,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.WithFields(f).Warnf("unable to encode payload for the request to URL: %s", url)
		return err
	}

	// Set up the request
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem creating a new request to URL: %s", url)
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+accessToken)
	client := http.Client{
		Timeout: DefaultHTTPTimeout,
	}

	// Invoke the request
	resp, err := client.Do(req)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem invoking request to URL: %s", url)
		return err
	}

	// Cleanup after
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			log.WithFields(f).WithError(closeErr).Warn("error closing response body")
		}
	}()

	// Check the response code to see how it went - response payload is undefined - just looking for a successful response status code
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		log.WithFields(f).Debugf("successfully added user: %s to group: %s", userName, groupName)
		lfUsername := ""
		username := ""
		if authUser != nil {
			lfUsername = authUser.UserName
			username = authUser.UserName
		}
		// Create a log event indicating our success
		lfg.EventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:  events.GerritUserAdded,
			LfUsername: lfUsername,
			UserName:   username,
			CLAGroupID: claGroupID,
			EventData: &events.GerritUserAddedEventData{
				Username:  userName,
				GroupName: groupName,
			},
		})
	} else {
		log.WithFields(f).Warnf("error adding added user: %s to group: %s - response status: %d", userName, groupName, resp.StatusCode)
	}

	return nil
}

// RemoveUserFromGroup removes the specified user from the group
func (lfg *LFGroup) RemoveUserFromGroup(ctx context.Context, authUser *auth.User, claGroupID, groupName, userName string) error {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.lf_group.RemoveUserFromGroup",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"groupName":      groupName,
		"authUserName":   authUser.UserName,
		"authUserEmail":  authUser.Email,
	}

	log.WithFields(f).Debug("removing user from group...")

	// Fetch a token for authorization
	accessToken, err := lfg.getAccessToken(ctx)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading access token")
		return err
	}

	// Build the URL path - can take the groupName or numeric value
	// API Docs: https://confluence.linuxfoundation.org/display/IPM/Drupal+Identity+REST+for+Auth0
	url := fmt.Sprintf("%s/rest/auth0/og/%s", lfg.LfBaseURL, groupName)

	// Build the request payload
	payload := map[string]interface{}{
		"username": userName,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.WithFields(f).Warnf("unable to encode payload for the request to URL: %s", url)
		return err
	}

	// Set up the request
	req, err := http.NewRequest("DELETE", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem creating a new request to URL: %s", url)
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+accessToken)
	client := http.Client{
		Timeout: DefaultHTTPTimeout,
	}

	// Invoke the request
	resp, err := client.Do(req)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem invoking request to URL: %s", url)
		return err
	}

	// Cleanup after
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			log.WithFields(f).WithError(closeErr).Warn("error closing response body")
		}
	}()

	// Check the response code to see how it went - response payload is undefined - just looking for a successful response status code
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		log.WithFields(f).Debugf("successfully removed user: %s from group: %s", userName, groupName)
		// Create a log event indicating our success
		lfg.EventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:  events.GerritUserRemoved,
			LfUsername: authUser.UserName,
			UserName:   authUser.UserName,
			CLAGroupID: claGroupID,
			EventData: &events.GerritUserRemovedEventData{
				Username:  userName,
				GroupName: groupName,
			},
		})
	} else {
		log.WithFields(f).Warnf("error removing user: %s from group: %s - response status: %d", userName, groupName, resp.StatusCode)
	}

	return nil
}
