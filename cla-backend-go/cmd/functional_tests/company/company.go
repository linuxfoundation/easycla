// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/linuxfoundation/easycla/cla-backend-go/cmd/functional_tests/test_models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/verdverm/frisby"
)

var (
	claManagerToken string
)

const (
	userID         = "e798ba27-98a4-11ea-8285-eee1f28f72a2"
	userEmail      = "ddeal+clamanager1+dev+intel@linuxfoundation.org"
	userName       = "clamanager1devintel"
	intelCompanyID = "2688cdd6-404e-4a31-a5b4-51e9c81f099d"
	// intelCompanyName = "Intel Corporation"
)

// GetXACL returns the X-ACL entry for this company
func GetXACL() (string, error) {
	xACL := auth.ACL{
		Admin:    false,
		Allowed:  true,
		Resource: "company_signatures",
		Context:  "staff",
		Scopes: []auth.Scope{
			{
				Type:  "organization",
				ID:    intelCompanyID,
				Role:  "cla-manager",
				Level: "staff",
			},
		},
	}

	jsonString, marshalErr := json.Marshal(xACL)
	if marshalErr != nil {
		log.Warnf("unable to marshall X-ACL entry, error: %+v", marshalErr)
		return "", marshalErr
	}
	return base64.StdEncoding.EncodeToString(jsonString), nil
}

// TestBehaviour data model
type TestBehaviour struct {
	apiURL      string
	auth0Config test_models.Auth0Config
}

// NewTestBehaviour creates a new test behavior model
func NewTestBehaviour(apiURL string, auth0Config test_models.Auth0Config) *TestBehaviour {
	return &TestBehaviour{
		apiURL + "/v4",
		auth0Config,
	}
}

func (t *TestBehaviour) buildHeaders(xACL string) map[string]string {
	return map[string]string{
		"Authorization":   "Bearer " + claManagerToken,
		"Content-Type":    "application/json",
		"Accept-Encoding": "application/json",
		"X-ACL":           xACL,
		"X-EMAIL":         userEmail,
		"X-USERNAME":      userName,
	}
}

// RunGetCLAManagerToken acquires the Auth0 token
func (t *TestBehaviour) RunGetCLAManagerToken() {
	authTokenReqPayload := map[string]string{
		"grant_type": "http://auth0.com/oauth/grant-type/password-realm",
		"realm":      "Username-Password-Authentication",
		"username":   t.auth0Config.Auth0UserName,
		"password":   t.auth0Config.Auth0Password,
		"client_id":  t.auth0Config.Auth0ClientID,
		"audience":   "https://api-gw.dev.platform.linuxfoundation.org/",
		"scope":      "access:api openid profile email",
	}
	frisby.Create(fmt.Sprintf("CLA Manager - Get Token - CLA Manager - %s", t.auth0Config.Auth0UserName)).
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

// RunCreateCompanyInvalid test
func (t *TestBehaviour) RunCreateCompanyInvalid(invalidCompanyName string) {
	url := fmt.Sprintf("%s/user/%s/company", t.apiURL, userID)
	xACL, xACLErr := GetXACL()
	if xACLErr != nil {
		return
	}

	frisby.Create(fmt.Sprintf("Company - Create Company - Invalid - %s", invalidCompanyName)).
		Post(url).
		SetHeaders(t.buildHeaders(xACL)).
		SetJson(map[string]string{
			"companyName":    invalidCompanyName,
			"companyWebsite": "https://www.ab.com",
			"email":          "user@linuxfoundation.org",
		}).
		Send().
		ExpectStatus(422)
}

// RunCreateCompanyValid test
func (t *TestBehaviour) RunCreateCompanyValid(companyName, companyURL string) string {
	url := fmt.Sprintf("%s/user/%s/company", t.apiURL, userID)
	xACL, xACLErr := GetXACL()
	if xACLErr != nil {
		return ""
	}

	var companyID = ""
	frisby.Create(fmt.Sprintf("Company - Create Company - Valid - %s", companyName)).
		Get(url).
		SetHeaders(t.buildHeaders(xACL)).
		SetJson(map[string]string{
			"companyName":    companyName,
			"companyWebsite": companyURL,
		}).
		Send().
		ExpectStatus(200).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			//log.Debugf("JSON: %+v", text)
			var companyModel models.Company
			unmarshallErr := json.Unmarshal([]byte(text), &companyModel)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			if companyModel.CompanyName != companyName {
				F.AddError(fmt.Sprintf("new company name %s does not match returned value: %s",
					companyName, companyModel.CompanyName))
			}

			companyID = companyModel.CompanyID
		})

	return companyID
}

// RunDeleteCompanyByID test
func (t *TestBehaviour) RunDeleteCompanyByID(companyID string) {
	url := fmt.Sprintf("%s/company/id/%s", t.apiURL, companyID)
	xACL, xACLErr := GetXACL()
	if xACLErr != nil {
		return
	}

	frisby.Create(fmt.Sprintf("Company - Delete Company By ID - %s", companyID)).
		Delete(url).
		SetHeaders(t.buildHeaders(xACL)).
		Send().
		ExpectStatus(204)
}

// RunAllTests runs all the CLA Group tests
func (t *TestBehaviour) RunAllTests() {
	// Need our authentication tokens for each persona/user
	t.RunGetCLAManagerToken()

	// Invalid Company Names
	// Max length is 255, exceed by 1 to test API
	var longCompanyName string
	for i := 1; i <= 256; i++ {
		longCompanyName += "a"
	}

	t.RunCreateCompanyInvalid("")
	t.RunCreateCompanyInvalid("a")             // too short
	t.RunCreateCompanyInvalid(longCompanyName) // too long
	t.RunCreateCompanyInvalid("-abc1234")
	t.RunCreateCompanyInvalid("?abc1234")
	t.RunCreateCompanyInvalid("/abc1234")
	t.RunCreateCompanyInvalid("*abc1234")
	t.RunCreateCompanyInvalid("#abc1234")
	t.RunCreateCompanyInvalid("+abc1234")
	t.RunCreateCompanyInvalid("!abc1234")
	t.RunCreateCompanyInvalid(".abc1234")

	// The EasyCLA create company logic not only creates a record within our local database, but also adds
	// a new Organization within SalesForce. Need to establish a configuration which skips these steps during
	// functional testing mode.
	if false {
		companyID := t.RunCreateCompanyValid("Harold Company", "https://harold.com")
		t.RunDeleteCompanyByID(companyID)
	}
}
