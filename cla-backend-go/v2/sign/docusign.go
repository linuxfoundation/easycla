// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package sign

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
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

	url := fmt.Sprintf("%s/accounts/%s/envelopes/%s/void", utils.GetProperty("DOCUSIGN_ROOT_URL"), utils.GetProperty("DOCUSIGN_ACCOUNT_ID"), envelopeID)

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

func (s *service) createEnvelope(ctx context.Context, payload *DocuSignEnvelopeRequest) (string, error) {
	f := logrus.Fields{
		"functionName":   "v2.createEnvelope",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	// Serialize the signRequest into JSON
	requestJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	log.WithFields(f).Debugf("sign request: %+v", string(requestJSON))

	// Get the access token
	accessToken, err := s.getAccessToken(ctx)

	if err != nil {
		return "", err
	}

	// Create the request
	url := fmt.Sprintf("%s/accounts/%s/envelopes", utils.GetProperty("DOCUSIGN_ROOT_URL"), utils.GetProperty("DOCUSIGN_ACCOUNT_ID"))

	req, err := http.NewRequest("POST", url, strings.NewReader(string(requestJSON)))

	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
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

	responsePayload, err := io.ReadAll(resp.Body)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem reading the response body")
		return "", err
	}

	if resp.StatusCode != http.StatusCreated {
		log.WithFields(f).Warnf("problem making the HTTP request - status code: %d - response : %s", resp.StatusCode, string(responsePayload))
		return "", errors.New("problem making the HTTP request")
	}

	var response DocuSignEnvelopeResponse

	err = json.Unmarshal(responsePayload, &response)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem unmarshalling the response body")
		return "", err
	}

	return response.EnvelopeId, nil

}

func (s *service) addDocumentToEnvelope(ctx context.Context, envelopeID, documentName string, document []byte) error {
	f := logrus.Fields{
		"functionName": "v2.addDocumentToEnvelope",
	}

	const method = "PUT"

	// Get the access token
	accessToken, err := s.getAccessToken(ctx)

	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/accounts/%s/envelopes/%s/documents/1", utils.GetProperty("DOCUSIGN_ROOT_URL"), utils.GetProperty("DOCUSIGN_ACCOUNT_ID"), envelopeID)

	log.WithFields(f).Debugf("url: %s", url)

	body := &bytes.Buffer{}

	writer := multipart.NewWriter(body)

	part, partErr := writer.CreateFormFile("file", documentName)
	if partErr != nil {
		return partErr
	}

	_, copyErr := io.Copy(part, bytes.NewReader(document))

	if copyErr != nil {
		return copyErr
	}

	closeErr := writer.Close()
	if closeErr != nil {
		return closeErr
	}

	// create the http request
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Disposition", fmt.Sprintf("filename=\"%s\"", documentName))
	req.Header.Set("Content-Type", "application/pdf")
	req.Header.Set("Accept", "application/json")

	log.WithFields(f).Debugf("adding document to envelope with url: %s %s", method, url)

	// Send HTTP request
	client := &http.Client{}
	resp, clientErr := client.Do(req)
	if clientErr != nil {
		log.WithFields(f).WithError(clientErr).Warnf("problem invoking envelope document upload request to %s %s", method, url)
		return clientErr
	}

	//log.WithFields(f).Debugf("response: %+v", resp)
	responsePayload, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		log.WithFields(f).WithError(readErr).Warnf("problem reading response body %+v", resp.Body)
		return readErr
	}

	// Expecting a 200 response
	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("problem invoking http %s request to %s - response status code is not 200: %d - response is: %+v", method, url, resp.StatusCode, string(responsePayload))
		log.WithFields(f).Warn(msg)
		return errors.New(msg)
	}

	defer func() {
		closeErr = resp.Body.Close()
		if closeErr != nil {
			log.WithFields(f).WithError(closeErr).Warnf("problem closing response body")
		}
	}()

	var documentUpdateResponseModel DocuSignUpdateDocumentResponse
	unmarshalErr := json.Unmarshal(responsePayload, &documentUpdateResponseModel)
	if unmarshalErr != nil {
		log.WithFields(f).WithError(unmarshalErr).Warnf("problem unmarshalling document update to the envelope response model JSON data")
		return unmarshalErr
	}

	log.WithFields(f).Debugf("successfully added document to envelope response body, uri: %s, documentGuid: %s, response: %+v", documentUpdateResponseModel.Uri, documentUpdateResponseModel.DocumentIdGuid, documentUpdateResponseModel)

	return nil

}

func (s *service) getEnvelopeRecipients(ctx context.Context, envelopeID string) ([]Signer, error) {
	f := logrus.Fields{
		"functionName": "v2.getEnvelopeRecipients",
		"envelopeID":   envelopeID,
	}

	// Get the access token
	accessToken, err := s.getAccessToken(ctx)

	if err != nil {
		return nil, err
	}

	log.WithFields(f).Debugf("access token: %s", accessToken)

	// Create the request
	url := fmt.Sprintf("%s/accounts/%s/envelopes/%s/recipients", utils.GetProperty("DOCUSIGN_ROOT_URL"), utils.GetProperty("DOCUSIGN_ACCOUNT_ID"), envelopeID)

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		log.WithFields(f).Debugf("%+v", err)
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	// Make the request
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem making the HTTP request")
		return nil, err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.WithFields(f).WithError(err).Warnf("problem closing the response body")
		}
	}()

	responsePayload, err := io.ReadAll(resp.Body)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem reading the response body")
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("problem getting getting recipients ")
	}

	var response *DocusignRecipientResponse

	err = json.Unmarshal(responsePayload, &response)

	if err != nil {
		log.WithFields(f).Debugf("unable to unmarshall response: %+v", err)
		return nil, err
	}

	log.WithFields(f).Debugf("got %d recipients", len(response.Signers))

	return response.Signers, nil
}

// Function to create a DocuSign envelope
func (s *service) PrepareSignRequest(ctx context.Context, signRequest *DocuSignEnvelopeRequest) (*DocusignEnvelopeResponse, error) {
	f := logrus.Fields{
		"functionName":   "v2.PrepareSignRequest",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

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

	log.WithFields(f).Debugf("access token: %s", accessToken)

	// Create the request
	url := fmt.Sprintf("%s/accounts/%s/envelopes", utils.GetProperty("DOCUSIGN_ROOT_URL"), utils.GetProperty("DOCUSIGN_ACCOUNT_ID"))

	req, err := http.NewRequest("POST", url, strings.NewReader(string(requestJSON)))

	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	// Make the request
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem making the HTTP request")
		return nil, err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.WithFields(f).WithError(err).Warnf("problem closing the response body")
		}
	}()

	responsePayload, err := io.ReadAll(resp.Body)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem reading the response body")
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		log.WithFields(f).Warnf("problem making the HTTP request - status code: %d - response : %s", resp.StatusCode, string(responsePayload))
		return nil, errors.New("problem making the HTTP request")
	}

	var response DocusignEnvelopeResponse

	err = json.Unmarshal(responsePayload, &response)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem unmarshalling the response body")
		return nil, err
	}

	return &response, nil

}

// Define a struct to represent the response from the DocuSign API.
type RecipientViewResponse struct {
	URL string `json:"url"`
}

// GetSignURL fetches the signing URL for the specified envelope and recipient

func (s *service) GetSignURL(email, recipientID, userName, clientUserId, envelopeID, returnURL string) (string, error) {

	f := logrus.Fields{
		"functionName": "v2.GetSignURL",
		"recipientID":  recipientID,
		"returnURL":    returnURL,
	}

	// Get the access token
	accessToken, err := s.getAccessToken(context.Background())

	if err != nil {
		return "", err
	}

	// Create the request

	url := fmt.Sprintf("%s/accounts/%s/envelopes/%s/views/recipient", utils.GetProperty("DOCUSIGN_ROOT_URL"), utils.GetProperty("DOCUSIGN_ACCOUNT_ID"), envelopeID)

	viewRecipientRequest := DocusignRecipientView{
		Email:               email,
		Username:            userName,
		RecipientID:         recipientID,
		ReturnURL:           returnURL,
		AuthenticaionMethod: "None",
	}

	if clientUserId != "" {
		viewRecipientRequest.ClientUserId = clientUserId
	}

	jsonRequest, err := json.Marshal(viewRecipientRequest)

	if err != nil {
		log.WithFields(f).Debugf("unable to marshal http request")
		return "", err
	}

	log.WithFields(f).Debugf("payload: %s", string(jsonRequest))

	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonRequest)))

	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		log.WithFields(f).Debugf("%+v", err)
		return "", err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.WithFields(f).WithError(err).Warnf("problem closing the response body")
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(f).Debugf("%+v", err)
		return "", err
	}

	if resp.StatusCode != http.StatusCreated {
		log.WithFields(f).Debugf("response: %+s and status code: %d", string(body), resp.StatusCode)
		return "", errors.New("failed to get signing URL")
	}

	var viewResponse RecipientViewResponse
	if err := json.Unmarshal(body, &viewResponse); err != nil {
		log.WithFields(f).Debug("failed to unmarshall response")
		return "", err
	}

	log.WithFields(f).Debugf("View response: %+v", viewResponse)

	return viewResponse.URL, nil
}

func (s service) getSignedDocument(ctx context.Context, envelopeID, documentID string) ([]byte, error) {
	f := logrus.Fields{
		"functionName": "v2.getSignedDocument",
		"envelopeID":   envelopeID,
	}

	// Get the access token
	accessToken, err := s.getAccessToken(ctx)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem getting the access token")
		return nil, err
	}

	// Create the request
	url := fmt.Sprintf("%s/accounts/%s/envelopes/%s/documents/%s", utils.GetProperty("DOCUSIGN_ROOT_URL"), utils.GetProperty("DOCUSIGN_ACCOUNT_ID"), envelopeID, documentID)

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem creating the HTTP request")
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	// Make the request
	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem making the HTTP request")
		return nil, err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.WithFields(f).WithError(err).Warnf("problem closing the response body")
		}
	}()

	responsePayload, err := io.ReadAll(resp.Body)

	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem reading the response body")
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.WithFields(f).Warnf("problem making the HTTP request - status code: %d - response : %s", resp.StatusCode, string(responsePayload))
		return nil, errors.New("problem making the HTTP request")
	}

	return responsePayload, nil

}
