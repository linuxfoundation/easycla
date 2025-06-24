// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/linuxfoundation/easycla/cla-backend-go/cmd/functional_tests/test_models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/verdverm/frisby"
)

var (
	claManagerToken            string
	claProspectiveManagerToken string
)

const (
	companyExternalIDGoogle = "0014100000Te02DAAR"
	companyExternalIDATT    = "0014100000Te1CaAAJ"
	companyExternalIDIBM    = "0012M00002Ddo9OQAR"
	//claManagerCompanyID           = "ee965ea2-ca83-4482-8a1b-94a468d9dcfa"
	//claManagerProjectID           = "d5412846-5dda-4c58-8f62-4c111a3cd0d3"
	//claProspectiveManagerLFID     = "cladevfunctionaltestuser"
	claProspectiveManagerUserName = "CLA Functional Test User Linux Foundation"
	claProspectiveManagerEmail    = "ddeal+cla+dev+functional+test+user@linuxfoundation.org"
	//claManagerLFID                = "cladevfunctionaltestclamanageruser"
	claManagerUserName = "CLA Manager Test User Linux Foundation"
	claManagerEmail    = "ddeal+cla+dev+functional+test+cla+manager+user@linuxfoundation.org"
)

// TestBehaviour data model
type TestBehaviour struct {
	apiURL           string
	auth0User1Config test_models.Auth0Config
	auth0User2Config test_models.Auth0Config
}

// NewTestBehaviour creates a new test behavior model
func NewTestBehaviour(apiURL string, auth0User1Config, auth0User2Config test_models.Auth0Config) *TestBehaviour {
	return &TestBehaviour{
		apiURL + "/v4",
		auth0User1Config,
		auth0User2Config,
	}
}

// RunGetCLAManagerToken acquires the Auth0 token
func (t *TestBehaviour) RunGetCLAManagerToken() {
	authTokenReqPayload := map[string]string{
		"grant_type": "http://auth0.com/oauth/grant-type/password-realm",
		"realm":      "Username-Password-Authentication",
		"username":   t.auth0User2Config.Auth0UserName,
		"password":   t.auth0User2Config.Auth0Password,
		"client_id":  t.auth0User2Config.Auth0ClientID,
		"audience":   "https://api-gw.dev.platform.linuxfoundation.org/",
		"scope":      "access:api openid profile email",
	}
	frisby.Create(fmt.Sprintf("CLA Manager - Get Token - CLA Manager - %s", t.auth0User2Config.Auth0UserName)).
		Post("https://linuxfoundation-dev.auth0.com/oauth/token").
		SetJson(authTokenReqPayload).
		Send().
		ExpectStatus(200).
		ExpectJsonType("access_token", reflect.String).
		ExpectJsonType("id_token", reflect.String).
		ExpectJsonType("scope", reflect.String).
		ExpectJsonType("expires_in", reflect.String).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			//log.Debugf("JSON: %+v", text)
			var auth0Response test_models.Auth0Response
			unmarshallErr := json.Unmarshal([]byte(text), &auth0Response)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			if auth0Response.IDToken == "" {
				F.AddError("Auth0Response id_token is empty")
			}
			claManagerToken = auth0Response.IDToken
			//log.Debugf("ID Token is: %s", token)
		})
}

// RunGetCLAProspectiveManagerToken acquires the Auth0 token
func (t *TestBehaviour) RunGetCLAProspectiveManagerToken() {
	authTokenReqPayload := map[string]string{
		"grant_type": "http://auth0.com/oauth/grant-type/password-realm",
		"realm":      "Username-Password-Authentication",
		"username":   t.auth0User1Config.Auth0UserName,
		"password":   t.auth0User1Config.Auth0Password,
		"client_id":  t.auth0User1Config.Auth0ClientID,
		"audience":   "https://api-gw.dev.platform.linuxfoundation.org/",
		"scope":      "access:api openid profile email",
	}

	frisby.Create(fmt.Sprintf("CLA Manager - Get Token - Prospective CLA Manager - %s", t.auth0User1Config.Auth0UserName)).
		Post("https://linuxfoundation-dev.auth0.com/oauth/token").
		SetJson(authTokenReqPayload).
		Send().
		ExpectStatus(200).
		ExpectJsonType("access_token", reflect.String).
		ExpectJsonType("id_token", reflect.String).
		ExpectJsonType("scope", reflect.String).
		ExpectJsonType("expires_in", reflect.String).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			//log.Debugf("JSON: %+v", text)
			var auth0Response test_models.Auth0Response
			unmarshallErr := json.Unmarshal([]byte(text), &auth0Response)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			if auth0Response.IDToken == "" {
				F.AddError("Auth0Response id_token is empty")
			}
			claProspectiveManagerToken = auth0Response.IDToken
			//log.Debugf("ID Token is: %s", token)
		})
}

// RunGetCompanySignaturesNoAuth test
func (t *TestBehaviour) RunGetCompanySignaturesNoAuth() {
	url := fmt.Sprintf("%s/signatures/company/%s", t.apiURL, companyExternalIDGoogle)
	frisby.Create("Signatures - Get Company Signatures - No Auth").
		Get(url).
		Send().
		ExpectStatus(401)
}

// RunGetCompanySignaturesForbidden test
func (t *TestBehaviour) RunGetCompanySignaturesForbidden() {
	url := fmt.Sprintf("%s/signatures/company/%s", t.apiURL, companyExternalIDGoogle)
	xACL, xACLErr := GetXACLATT()
	if xACLErr != nil {
		return
	}
	frisby.Create("Signatures - Get Company Signatures - Forbidden - ATT trying Google").
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claProspectiveManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
			"X-ACL":           xACL,
			"X-EMAIL":         claProspectiveManagerEmail,
			"X-USERNAME":      claProspectiveManagerUserName,
		}).
		Get(url).
		Send().
		ExpectStatus(403)
}

// RunGetCompanySignatures test
func (t *TestBehaviour) RunGetCompanySignatures() {
	url := fmt.Sprintf("%s/signatures/company/%s", t.apiURL, companyExternalIDGoogle)
	xACL, xACLErr := GetXACLGoogle()
	if xACLErr != nil {
		return
	}
	frisby.Create("Signatures - Get Company Signatures - Google").
		Get(url).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
			"X-ACL":           xACL,
			"X-EMAIL":         claManagerEmail,
			"X-USERNAME":      claManagerUserName,
		}).
		Send().
		ExpectStatus(200).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			var signatures models.Signatures
			unmarshallErr := json.Unmarshal([]byte(text), &signatures)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			if len(signatures.Signatures) == 0 {
				F.AddError("Signatures - Get Company Signatures - Google - Expecting at least one signature in response")
			}
			for _, sig := range signatures.Signatures {
				if sig.SignatureReferenceID != companyExternalIDGoogle {
					F.AddError(fmt.Sprintf("Signatures - Get Company Signatures - Google - Company ID's do not match: %s vs %s",
						sig.SignatureReferenceID, companyExternalIDGoogle))
				}
			}
		})
}

// RunAllTests runs all the CLA Manager tests
func (t *TestBehaviour) RunAllTests() {
	// Need our authentication tokens for each persona/user
	t.RunGetCLAManagerToken()
	t.RunGetCLAProspectiveManagerToken()

	// No Credentials/Auth Tests
	t.RunGetCompanySignaturesNoAuth()
	t.RunGetCompanySignaturesForbidden()

	// Get
	t.RunGetCompanySignatures()
}
