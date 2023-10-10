// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package docusignauth

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
)

var (
	baseURL           = os.Getenv("DOCUSIGN_AUTH_SERVER")
	oauthTokenURL     = baseURL + "/oauth/token"
	jwtGrantAssertion = "urn:ietf:params:oauth:grant-type:jwt-bearer"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func GetAccessToken(integrationKey, userGUID, privateKey string) (string, error) {
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return "", fmt.Errorf("failed to parse private key")
	}

	privateKeyParsed, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}

	// Generate the JWT token
	tokenString, err := generateJWT(integrationKey, userGUID, privateKeyParsed.D.String())
	if err != nil {
		return "", err
	}

	// Make the HTTP request to get the access token
	body := strings.NewReader(fmt.Sprintf("grant_type=%s&assertion=%s", jwtGrantAssertion, tokenString))
	req, err := http.NewRequest("POST", oauthTokenURL, body)
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() // nolint

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get access token, status: %s", resp.Status)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResponse TokenResponse
	if err := json.Unmarshal(respBody, &tokenResponse); err != nil {
		return "", err
	}

	return tokenResponse.AccessToken, nil
}

func generateJWT(integrationKey, userGUID, privateKey string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":   integrationKey,                       // Integration Key
		"sub":   userGUID,                             // User GUID
		"aud":   baseURL,                              // Base URL
		"iat":   time.Now().Unix(),                    // Issued At
		"exp":   time.Now().Add(1 * time.Hour).Unix(), // Expiration time - 1 hour is recommended
		"scope": "signature",                          // Permission scope
	})

	signKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKey))
	if err != nil {
		return "", err
	}

	tokenString, err := token.SignedString(signKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
