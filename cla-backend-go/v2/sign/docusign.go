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

// Void envelope
func (s *service) VoidEnvelope(ctx context.Context, envelopeID, message string) error {
	f := logrus.Fields{
		"functionName":   "v2.VoidEnvelope",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"envelopeID":     envelopeID,
		"message":        message,
	}

	accessToken, err := s.getAccessToken(ctx)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem getting the access token")
		return err
	}

	voidRequest := struct {
		VoidReason string `json:"voidReason"`
	}{
		VoidReason: message,
	}

	voidRequestJSON, err := json.Marshal(voidRequest)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem marshalling the void request")
		return err
	}

	url := fmt.Sprintf("https://%s/restapi/v2.1/accounts/%s/envelopes/%s/void", utils.GetProperty("DOCUSIGN_ROOT_URL"), utils.GetProperty("DOCUSIGN_ACCOUNT_ID"), envelopeID)

	req, err := http.NewRequest("PUT", url, strings.NewReader(string(voidRequestJSON)))

	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("Content-Type", "application/json")

	// Make the request
	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.WithFields(f).WithError(err).Warnf("problem closing the response body")
		}
	}()

	_, err = io.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New("problem making the HTTP request")
	}

	return nil

}

// Function to create a DocuSign envelope
func (s *service) PrepareSignRequest(ctx context.Context, signRequest *DocuSignEnvelopeRequest) (*DocuSignEnvelopeResponse, error) {
	// Serialize the signRequest into JSON
	requestJSON, err := json.Marshal(signRequest)
	if err != nil {
		return nil, err
	}

	// Get the access token
	accessToken, err := s.getAccessToken(ctx)

	if err != nil {
		return nil, err
	}

	// Create the request

	url := fmt.Sprintf("https://%s/restapi/v2.1/accounts/%s/envelopes", utils.GetProperty("DOCUSIGN_ROOT_URL"), utils.GetProperty("DOCUSIGN_ACCOUNT_ID"))

	req, err := http.NewRequest("POST", url, strings.NewReader(string(requestJSON)))

	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("Content-Type", "application/json")

	// Make the request
	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.Warnf("problem closing the response body")
		}
	}()

	// Parse the response
	responsePayload, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("problem making the HTTP request")
	}

	var envelopeResponse DocuSignEnvelopeResponse

	err = json.Unmarshal(responsePayload, &envelopeResponse)

	if err != nil {
		return nil, err
	}

	return &envelopeResponse, nil
}

// GetSignURL fetches the signing URL for the specified envelope and recipient

func (s *service) GetSignURL(envelopeID, recipientID, returnURL string) (string, error) {

	f := logrus.Fields{
		"functionName": "v2.GetSignURL",
		"envelopeID":   envelopeID,
		"recipientID":  recipientID,
		"returnURL":    returnURL,
	}

	// Get the access token
	accessToken, err := s.getAccessToken(context.Background())

	if err != nil {
		return "", err
	}

	// Create the request

	url := fmt.Sprintf("https://%s/restapi/v2.1/accounts/%s/envelopes/%s/views/recipient", utils.GetProperty("DOCUSIGN_ROOT_URL"), utils.GetProperty("DOCUSIGN_ACCOUNT_ID"), envelopeID)

	req, err := http.NewRequest("POST", url, nil)

	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("Content-Type", "application/json")

	// Create the request body
	requestBody := struct {
		ReturnURL    string `json:"returnUrl"`
		ClientUserID string `json:"clientUserId"`
		RecipientID  string `json:"recipientId"`
	}{
		ReturnURL:    returnURL,
		ClientUserID: recipientID,
		RecipientID:  recipientID,
	}

	requestBodyJSON, err := json.Marshal(requestBody)

	if err != nil {
		return "", err
	}

	req.Body = io.NopCloser(strings.NewReader(string(requestBodyJSON)))

	// Make the request
	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return "", err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.WithFields(f).WithError(err).Warnf("problem closing the response body")
		}
	}()

	// Parse the response

	// Parse the response JSON
	var response struct {
		Url string `json:"url"`
	}

	responsePayload, err := io.ReadAll(resp.Body)

	if err != nil {
		return "", err
	}

	err = json.Unmarshal(responsePayload, &response)

	if err != nil {
		return "", err
	}

	return response.Url, nil
}
