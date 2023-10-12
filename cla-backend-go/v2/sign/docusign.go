// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// getAccessToken retrieves an access token for the DocuSign API using a JWT assertion.
func (s *service) getAccessToken(ctx context.Context) (string, error) {
	f := logrus.Fields{
		"functionName":   "v2.getAccessToken",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	jwtAssertion, err := jwtToken(s.docsignPrivateKey)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem generating the JWT token")
		return "", err
	}

	// Create the request
	tokenRequestBody := DocuSignGetTokenRequest{
		GrantType: "urn:ietf:params:oauth:grant-type:jwt-bearer",
		Assertion: jwtAssertion,
	}

	tokenRequestBodyJSON, err := json.Marshal(tokenRequestBody)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem marshalling the token request body")
		return "", err
	}

	url := fmt.Sprintf("https://%s/oauth/token", utils.GetProperty("DOCUSIGN_AUTH_SERVER"))
	req, err := http.NewRequest("POST", url, strings.NewReader(string(tokenRequestBodyJSON)))
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem creating the HTTP request")
		return "", err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem making the HTTP request")
		return "", err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.WithFields(f).WithError(err).Warnf("problem closing the response body")
		}
	}()

	// Parse the response
	responsePayload, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem reading the response body")
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		log.WithFields(f).Warnf("problem making the HTTP request - status code: %d", resp.StatusCode)
		return "", errors.New("problem making the HTTP request")
	}

	var tokenResponse DocuSignGetTokenResponse

	err = json.Unmarshal(responsePayload, &tokenResponse)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem unmarshalling the response body")
		return "", err
	}

	return tokenResponse.AccessToken, nil

}
