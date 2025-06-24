// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_group

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/linuxfoundation/easycla/cla-backend-go/cmd/functional_tests/test_models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/verdverm/frisby"
)

var (
	claProjectManagerToken string
)

// TestBehaviour data model
type TestBehaviour struct {
	apiURL                    string
	auth0ProjectManagerConfig test_models.Auth0Config
}

// NewTestBehaviour creates a new test behavior model
func NewTestBehaviour(apiURL string, auth0ProjectManagerConfig test_models.Auth0Config) *TestBehaviour {
	return &TestBehaviour{
		apiURL,
		auth0ProjectManagerConfig,
	}
}

// RunGetCLAProjectManagerToken acquires the Auth0 token
func (t *TestBehaviour) RunGetCLAProjectManagerToken() {
	authTokenReqPayload := map[string]string{
		"grant_type": "http://auth0.com/oauth/grant-type/password-realm",
		"realm":      "Username-Password-Authentication",
		"username":   t.auth0ProjectManagerConfig.Auth0UserName,
		"password":   t.auth0ProjectManagerConfig.Auth0Password,
		"client_id":  t.auth0ProjectManagerConfig.Auth0ClientID,
		"audience":   "https://api-gw.dev.platform.linuxfoundation.org/",
		"scope":      "access:api openid profile email",
	}

	frisby.Create(fmt.Sprintf("CLA Group Validate - Get Token - Project Manager Manager - %s", t.auth0ProjectManagerConfig.Auth0UserName)).
		Post("https://linuxfoundation-dev.auth0.com/oauth/token").
		SetJson(authTokenReqPayload).
		Send().
		ExpectStatus(200).
		ExpectJsonType("access_token", reflect.String).
		ExpectJsonType("id_token", reflect.String).
		ExpectJsonType("scope", reflect.String).
		ExpectJsonType("expires_in", reflect.String).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			// log.Debugf("response: %s", text)
			var auth0Response test_models.Auth0Response
			unmarshallErr := json.Unmarshal([]byte(text), &auth0Response)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			if auth0Response.IDToken == "" {
				F.AddError("Auth0Response id_token is empty")
			}
			claProjectManagerToken = auth0Response.IDToken
			//log.Debugf("ID Token is: %s", token)
		})
}

// getCLAProjectManagerHeaders is a helper function to get the request headers
func (t *TestBehaviour) getCLAProjectManagerHeaders() map[string]string {
	return map[string]string{
		"Authorization":   "Bearer " + claProjectManagerToken,
		"Content-Type":    "application/json",
		"Accept-Encoding": "application/json",
		"X-Email":         t.auth0ProjectManagerConfig.Auth0Email,
		"X-USERNAME":      t.auth0ProjectManagerConfig.Auth0UserName,
	}
}

// RunValidateCLAGroupValid runs a validation test against the specified parameters - expecting a successful validation
func (t *TestBehaviour) RunValidateCLAGroupValid(claGroupName, claGroupDescription string) {
	endpoint := fmt.Sprintf("%s/v4/cla-group/validate", t.apiURL)
	frisby.Create(fmt.Sprintf("CLA Group Validate - Valid Input - CLA Group Name: %s, Description: %s", claGroupName, claGroupDescription)).
		Post(endpoint).
		SetHeaders(t.getCLAProjectManagerHeaders()).
		SetJson(map[string]string{
			"cla_group_name":        claGroupName,
			"cla_group_description": claGroupDescription,
		}).
		Send().
		ExpectStatus(200).
		ExpectJsonType("valid", reflect.Bool).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			var response models.ClaGroupValidationResponse
			unmarshallErr := json.Unmarshal([]byte(text), &response)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			if !response.Valid {
				F.AddError(fmt.Sprintf("CLA Group Validate - Valid Input Expected - CLA Group Name: %s, Description: %s failed: %+v",
					claGroupName, claGroupDescription, response))
			}
		})
}

// RunValidateCLAGroupInvalid runs a validation test against the specified parameters - expecting a unsuccessful validation
func (t *TestBehaviour) RunValidateCLAGroupInvalid(claGroupName, claGroupDescription string) {
	endpoint := fmt.Sprintf("%s/v4/cla-group/validate", t.apiURL)
	frisby.Create(fmt.Sprintf("CLA Group Validate - Invalid Input Min/Max - CLA Group Name: %s, Description: %s", claGroupName, claGroupDescription)).
		Post(endpoint).
		SetHeaders(t.getCLAProjectManagerHeaders()).
		SetJson(map[string]string{
			"cla_group_name":        claGroupName,
			"cla_group_description": claGroupDescription,
		}).
		Send().
		ExpectStatus(422).
		// ExpectJsonType("code", reflect.String). // type seems to be json.Number ?
		ExpectJsonType("message", reflect.String)
}

// RunValidateCLAGroupInvalidDuplicate runs a validation test against the specified parameters - expecting a unsuccessful validation
func (t *TestBehaviour) RunValidateCLAGroupInvalidDuplicate(claGroupName, claGroupDescription string) {
	endpoint := fmt.Sprintf("%s/v4/cla-group/validate", t.apiURL)
	frisby.Create(fmt.Sprintf("CLA Group Validate - Invalid Input Duplicate - CLA Group Name: %s, Description: %s", claGroupName, claGroupDescription)).
		Post(endpoint).
		SetHeaders(t.getCLAProjectManagerHeaders()).
		SetJson(map[string]string{
			"cla_group_name":        claGroupName,
			"cla_group_description": claGroupDescription,
		}).
		Send().
		ExpectStatus(200).
		ExpectJsonType("valid", reflect.Bool).
		// ExpectJsonType("validation_errors", reflect.String). // map[string]interface{}
		AfterText(func(F *frisby.Frisby, text string, err error) {
			var response models.ClaGroupValidationResponse
			unmarshallErr := json.Unmarshal([]byte(text), &response)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			if response.Valid {
				F.AddError(fmt.Sprintf("CLA Group Validate - Invalid Input Expected - CLA Group Name: %s, Description: %s failed: %+v",
					claGroupName, claGroupDescription, response))
			}
			if response.ValidationErrors == nil {
				F.AddError(fmt.Sprintf("CLA Group Validate - Invalid Input Expected - CLA Group Name: %s, Description: %s failed: %+v",
					claGroupName, claGroupDescription, response))
			}
		})
}

// RunAllTests runs all the CLA Manager tests
func (t *TestBehaviour) RunAllTests() {
	// Need our authentication tokens for each persona/user
	t.RunGetCLAProjectManagerToken()
	t.RunValidateCLAGroupValid("functional-test", "functional test description")
	t.RunValidateCLAGroupInvalid("fu", "functional test description")
	t.RunValidateCLAGroupInvalid("fu", "functional test description")
	t.RunValidateCLAGroupInvalid("functional-test", "fu")
	t.RunValidateCLAGroupInvalid("    ", "          ")
	t.RunValidateCLAGroupInvalid("    ", "functional test description")
	t.RunValidateCLAGroupInvalid("functional-test", "          ")
	// This CLA Group name is already in the DB - should trigger a duplicate violation
	t.RunValidateCLAGroupInvalidDuplicate("CLA group name QA", "doesn't matter")
}
