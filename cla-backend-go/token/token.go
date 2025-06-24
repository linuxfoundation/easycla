// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/imroc/req"
)

const (
	tokenPath = "oauth/token" // nolint
)

var (
	clientID      string
	clientSecret  string
	audience      string
	oauthTokenURL string
	token         string
	expiry        time.Time
)

type tokenGen struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Audience     string `json:"audience"`
}

type tokenReturn struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// Init is the token initialization logic
func Init(paramClientID, paramClientSecret, paramAuth0URL, paramAudience string) {
	f := logrus.Fields{
		"functionName": "token.Init",
		"auth0URL":     paramAuth0URL,
		"audience":     paramAudience,
	}
	log.WithFields(f).Debug("token init running...")

	clientID = paramClientID
	clientSecret = paramClientSecret
	audience = paramAudience
	oauthTokenURL = paramAuth0URL

	if expiry.Year() == 1 {
		expiry = time.Now()
	}

	go retrieveToken() //nolint
}

func retrieveToken() error {
	f := logrus.Fields{
		"functionName": "token.retrieveToken",
	}
	log.WithFields(f).Debug("refreshing auth0 token...")

	tg := tokenGen{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Audience:     audience,
	}

	resp, err := req.Post(oauthTokenURL, req.BodyJSON(&tg))
	if err != nil {
		log.WithFields(f).WithError(err).Warn("refresh token request failed")
		return err
	}

	if resp.Response().StatusCode < 200 || resp.Response().StatusCode > 299 {
		err = fmt.Errorf("invalid response from auth0 service %s - received error code: %d, response: %s",
			oauthTokenURL, resp.Response().StatusCode, resp.String())
		log.WithFields(f).WithError(err)
		return err
	}

	var tr tokenReturn
	err = resp.ToJSON(&tr)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("refresh token::json unmarshal failed of response: %s, error: %+v", resp.String(), err)
		return err
	}

	//token = tr.TokenType + " " + tr.AccessToken
	token = tr.AccessToken
	if tr.AccessToken == "" || tr.TokenType == "" {
		err = errors.New("error fetching authentication token - response value is empty")
		log.WithFields(f).WithError(err).Warn("empty response from auth server")
		return err
	}

	expiry = time.Now()
	tokenExpiry := time.Now().Add(time.Second * time.Duration(tr.ExpiresIn))
	log.WithFields(f).Debugf("retrieved token: %s... expires: %s", token[0:8], tokenExpiry.UTC().String())

	return nil
}

// GetToken returns the Auth0 Token - in necessary, refreshes the token when expired
func GetToken() (string, error) {
	f := logrus.Fields{
		"functionName": "token.GetToken",
	}

	// set 2.75 hrs duration for new token
	if (time.Now().Unix()-expiry.Unix()) > 9900 || token == "" {
		log.WithFields(f).Debug("token is either empty or expired, retrieving new token")
		err := retrieveToken()
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to retrieve a new token")
			return "", err
		}
	}

	return token, nil
}
