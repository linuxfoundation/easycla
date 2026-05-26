// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package token

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
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
	tokenMutex    sync.RWMutex // Protects token and expiry variables
	// httpClient mirrors the imroc/req v0.3.0 default client (2 minute overall timeout)
	// that this package previously used, so downstream timeout behavior is preserved.
	httpClient = &http.Client{Timeout: 2 * time.Minute}
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

	tokenMutex.Lock()
	if expiry.Year() == 1 {
		expiry = time.Now()
	}
	tokenMutex.Unlock()

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

	body, err := json.Marshal(&tg)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("refresh token request marshal failed")
		return err
	}

	httpReq, err := http.NewRequest(http.MethodPost, oauthTokenURL, bytes.NewReader(body))
	if err != nil {
		log.WithFields(f).WithError(err).Warn("refresh token request build failed")
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("refresh token request failed")
		return err
	}
	defer resp.Body.Close() // nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("refresh token response read failed")
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		err = fmt.Errorf("invalid response from auth0 service %s - received error code: %d, response: %s",
			oauthTokenURL, resp.StatusCode, string(respBody))
		log.WithFields(f).WithError(err)
		return err
	}

	var tr tokenReturn
	if err = json.Unmarshal(respBody, &tr); err != nil {
		log.WithFields(f).WithError(err).Warnf("refresh token::json unmarshal failed of response: %s, error: %+v", string(respBody), err)
		return err
	}

	if tr.AccessToken == "" || tr.TokenType == "" {
		err = errors.New("error fetching authentication token - response value is empty")
		log.WithFields(f).WithError(err).Warn("empty response from auth server")
		return err
	}

	tokenMutex.Lock()
	//token = tr.TokenType + " " + tr.AccessToken
	token = tr.AccessToken
	expiry = time.Now()
	tokenMutex.Unlock()

	tokenExpiry := time.Now().Add(time.Second * time.Duration(tr.ExpiresIn))
	tokenMutex.RLock()
	tokenForLog := token
	tokenMutex.RUnlock()
	log.WithFields(f).Debugf("retrieved token: %s... expires: %s", tokenForLog[0:8], tokenExpiry.UTC().String())

	return nil
}

// GetToken returns the Auth0 Token - in necessary, refreshes the token when expired
func GetToken() (string, error) {
	f := logrus.Fields{
		"functionName": "token.GetToken",
	}

	tokenMutex.RLock()
	currentToken := token
	currentExpiry := expiry
	tokenMutex.RUnlock()

	// set 2.75 hrs duration for new token
	if (time.Now().Unix()-currentExpiry.Unix()) > 9900 || currentToken == "" {
		log.WithFields(f).Debug("token is either empty or expired, retrieving new token")
		err := retrieveToken()
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to retrieve a new token")
			return "", err
		}

		tokenMutex.RLock()
		currentToken = token
		tokenMutex.RUnlock()
	}

	return currentToken, nil
}
