// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/communitybridge/easycla/cla-backend-go/cmd/functional_tests/test_models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/verdverm/frisby"
)

var (
	claManagerToken            string
	claProspectiveManagerToken string
	claManagerCreateRequestID  = "no-set"
)

const (
	claManagerCompanyID           = "ee965ea2-ca83-4482-8a1b-94a468d9dcfa"
	claManagerProjectID           = "d5412846-5dda-4c58-8f62-4c111a3cd0d3"
	claProspectiveManagerLFID     = "cladevfunctionaltestuser"
	claProspectiveManagerUserName = "CLA Functional Test User Linux Foundation"
	claProspectiveManagerEmail    = "ddeal+cla+dev+functional+test+user@linuxfoundation.org"
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
		apiURL + "/v3",
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

// RunCreateCLAManagerRequestNoAuth test
func (t *TestBehaviour) RunCreateCLAManagerRequestNoAuth() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests",
		t.apiURL, claManagerCompanyID, claManagerProjectID)
	frisby.Create("CLA Manager - Create CLA Manager Request - No Auth").
		Post(url).
		Send().
		ExpectStatus(401)
}

// RunGetCLAManagerRequestsNoAuth test
func (t *TestBehaviour) RunGetCLAManagerRequestsNoAuth() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests",
		t.apiURL, claManagerCompanyID, claManagerProjectID)
	frisby.Create("CLA Manager - Get CLA Manager Requests - No Auth").
		Get(url).
		Send().
		ExpectStatus(401)
}

// RunGetCLAManagerRequestNoAuth test
func (t *TestBehaviour) RunGetCLAManagerRequestNoAuth() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests/%s",
		t.apiURL, claManagerCompanyID, claManagerProjectID, "test-request-id")
	frisby.Create("CLA Manager - Get CLA Manager Request - No Auth").
		Get(url).
		Send().
		ExpectStatus(401)
}

// RunApproveCLAManagerRequestNoAuth test
func (t *TestBehaviour) RunApproveCLAManagerRequestNoAuth() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests/%s/approve",
		t.apiURL, claManagerCompanyID, claManagerProjectID, "test-request-id")
	frisby.Create("CLA Manager - Approve CLA Manager Request - No Auth").
		Put(url).
		Send().
		ExpectStatus(401)
}

// RunDenyCLAManagerRequestNoAuth test
func (t *TestBehaviour) RunDenyCLAManagerRequestNoAuth() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests/%s/deny",
		t.apiURL, claManagerCompanyID, claManagerProjectID, "test-request-id")
	frisby.Create("CLA Manager - Deny CLA Manager Request - No Auth").
		Put(url).
		Send().
		ExpectStatus(401)
}

// RunDeleteCLAManagerRequestNoAuth test
func (t *TestBehaviour) RunDeleteCLAManagerRequestNoAuth() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests/%s",
		t.apiURL, claManagerCompanyID, claManagerProjectID, claManagerCreateRequestID)
	frisby.Create("CLA Manager - Delete CLA Manager Requests - No Auth").
		Delete(url).
		Send().
		ExpectStatus(401)
}

// RunCreateCLAManagerRequest test
func (t *TestBehaviour) RunCreateCLAManagerRequest() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests",
		t.apiURL, claManagerCompanyID, claManagerProjectID)
	frisby.Create("CLA Manager - Create CLA Manager Request").
		Post(url).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claProspectiveManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
		}).
		SetJson(map[string]string{
			"userName":  claProspectiveManagerUserName,
			"userEmail": claProspectiveManagerEmail,
			"userLFID":  claProspectiveManagerLFID,
		}).
		Send().
		ExpectStatus(200).
		ExpectJsonType("companyID", reflect.String).
		ExpectJsonType("companyName", reflect.String).
		ExpectJsonType("projectID", reflect.String).
		ExpectJsonType("projectName", reflect.String).
		ExpectJsonType("projectExternalID", reflect.String).
		ExpectJsonType("userID", reflect.String).
		ExpectJsonType("userName", reflect.String).
		ExpectJsonType("userEmail", reflect.String).
		ExpectJsonType("created", reflect.String).
		ExpectJsonType("updated", reflect.String).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			//log.Debugf("Create CLA Manager Response JSON: %+v", text)
			var claManagerResp models.ClaManagerRequest
			unmarshallErr := json.Unmarshal([]byte(text), &claManagerResp)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			claManagerCreateRequestID = claManagerResp.RequestID
			//log.Debugf("Saved CLA Manager request ID: %s", claManagerCreateRequestID)
		})
}

// RunDeleteCLAManagerRequestUnauthorized test
func (t *TestBehaviour) RunDeleteCLAManagerRequestUnauthorized() {
	// ProspectiveManagerToken is not approved yet - shouldn't be able to delete requests
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests/%s",
		t.apiURL, claManagerCompanyID, claManagerProjectID, claManagerCreateRequestID)
	frisby.Create(fmt.Sprintf("CLA Manager - Delete CLA Manager Requests - Unauthorized - %s", claProspectiveManagerLFID)).
		Delete(url).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claProspectiveManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
		}).
		Send().
		ExpectStatus(401)
}

// RunApproveCLAManagerRequestUnauthorized test
func (t *TestBehaviour) RunApproveCLAManagerRequestUnauthorized() {
	// ProspectiveManagerToken is not approved yet - shouldn't be able to approve requests
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests/%s/approve",
		t.apiURL, claManagerCompanyID, claManagerProjectID, claManagerCreateRequestID)
	frisby.Create(fmt.Sprintf("CLA Manager - Approve CLA Manager Requests - Unauthorized - %s", claProspectiveManagerLFID)).
		Put(url).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claProspectiveManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
		}).
		Send().
		ExpectStatus(401)
}

// RunDenyCLAManagerRequestUnauthorized test
func (t *TestBehaviour) RunDenyCLAManagerRequestUnauthorized() {
	// ProspectiveManagerToken is not approved yet - shouldn't be able to deny requests
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests/%s/deny",
		t.apiURL, claManagerCompanyID, claManagerProjectID, claManagerCreateRequestID)
	frisby.Create(fmt.Sprintf("CLA Manager - Deny CLA Manager Requests - Unauthorized - %s", claProspectiveManagerLFID)).
		Put(url).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claProspectiveManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
		}).
		Send().
		ExpectStatus(401)
}

// RunGetCLAManagerRequests test
func (t *TestBehaviour) RunGetCLAManagerRequests() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests",
		t.apiURL, claManagerCompanyID, claManagerProjectID)
	frisby.Create("CLA Manager - Get CLA Manager Requests").
		Get(url).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
		}).
		Send().
		ExpectStatus(200).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			var requests models.ClaManagerRequestList
			unmarshallErr := json.Unmarshal([]byte(text), &requests)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			if requests.Requests == nil || len(requests.Requests) == 0 {
				F.AddError("GET CLA Manager Requests - Expecting at least one request in response")
			}
			var containsEntry = false
			for _, request := range requests.Requests {
				if request.CompanyID != claManagerCompanyID {
					F.AddError(fmt.Sprintf("GET CLA Manager Requests - Company ID's do not match: %s vs %s",
						request.CompanyID, claManagerCompanyID))
				}
				if request.ProjectID != claManagerProjectID {
					F.AddError(fmt.Sprintf("GET CLA Manager Requests - Project ID's do not match: %s vs %s",
						request.ProjectID, claManagerProjectID))
				}
				if request.UserID == claProspectiveManagerLFID && request.Status == "pending" {
					containsEntry = true
				}
			}
			if !containsEntry {
				F.AddError("GET CLA Manager Requests - Missing Request for user: ")
			}
		})
}

// RunGetCLAManagerRequest test
func (t *TestBehaviour) RunGetCLAManagerRequest() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests/%s",
		t.apiURL, claManagerCompanyID, claManagerProjectID, claManagerCreateRequestID)
	frisby.Create("CLA Manager - Get CLA Manager Request").
		Get(url).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
		}).
		Send().
		ExpectStatus(200).
		ExpectJsonType("companyID", reflect.String).
		ExpectJsonType("companyName", reflect.String).
		ExpectJsonType("projectID", reflect.String).
		ExpectJsonType("projectName", reflect.String).
		ExpectJsonType("projectExternalID", reflect.String).
		ExpectJsonType("userID", reflect.String).
		ExpectJsonType("userName", reflect.String).
		ExpectJsonType("userEmail", reflect.String).
		ExpectJsonType("created", reflect.String).
		ExpectJsonType("updated", reflect.String).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			//log.Debugf("Create CLA Manager Response JSON: %+v", text)
			var claManagerResp models.ClaManagerRequest
			unmarshallErr := json.Unmarshal([]byte(text), &claManagerResp)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			if claManagerResp.CompanyID != claManagerCompanyID {
				F.AddError(fmt.Sprintf("GET CLA Manager Request - Company ID's do not match: %s vs %s",
					claManagerResp.CompanyID, claManagerCompanyID))
			}
			if claManagerResp.ProjectID != claManagerProjectID {
				F.AddError(fmt.Sprintf("GET CLA Manager Request - Project ID's do not match: %s vs %s",
					claManagerResp.ProjectID, claManagerProjectID))
			}
			if claManagerResp.RequestID != claManagerCreateRequestID {
				F.AddError(fmt.Sprintf("GET CLA Manager Request - Request ID's do not match: %s vs %s",
					claManagerResp.RequestID, claManagerCreateRequestID))
			}
		})
}

// RunApproveCLAManagerRequest test
func (t *TestBehaviour) RunApproveCLAManagerRequest() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests/%s/approve",
		t.apiURL, claManagerCompanyID, claManagerProjectID, claManagerCreateRequestID)
	frisby.Create(fmt.Sprintf("CLA Manager - Approve CLA Manager Requests - %s", claManagerCreateRequestID)).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
		}).
		Put(url).
		Send().
		ExpectStatus(200).
		ExpectJsonType("companyID", reflect.String).
		ExpectJsonType("companyName", reflect.String).
		ExpectJsonType("projectID", reflect.String).
		ExpectJsonType("projectName", reflect.String).
		ExpectJsonType("projectExternalID", reflect.String).
		ExpectJsonType("userID", reflect.String).
		ExpectJsonType("userName", reflect.String).
		ExpectJsonType("userEmail", reflect.String).
		ExpectJsonType("created", reflect.String).
		ExpectJsonType("updated", reflect.String).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			//log.Debugf("Create CLA Manager Response JSON: %+v", text)
			var claManagerResp models.ClaManagerRequest
			unmarshallErr := json.Unmarshal([]byte(text), &claManagerResp)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			if claManagerResp.CompanyID != claManagerCompanyID {
				F.AddError(fmt.Sprintf("Approve CLA Manager Request - Company ID's do not match: %s vs %s",
					claManagerResp.CompanyID, claManagerCompanyID))
			}
			if claManagerResp.ProjectID != claManagerProjectID {
				F.AddError(fmt.Sprintf("Approve CLA Manager Request - Project ID's do not match: %s vs %s",
					claManagerResp.ProjectID, claManagerProjectID))
			}
			if claManagerResp.RequestID != claManagerCreateRequestID {
				F.AddError(fmt.Sprintf("Approve CLA Manager Request - Request ID's do not match: %s vs %s",
					claManagerResp.RequestID, claManagerCreateRequestID))
			}
			if claManagerResp.Status != "approved" {
				F.AddError("Approve CLA Manager Request - Status not set to approved")
			}
		})
}

// RunDeleteCLAManagerRequest test
func (t *TestBehaviour) RunDeleteCLAManagerRequest() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/requests/%s",
		t.apiURL, claManagerCompanyID, claManagerProjectID, claManagerCreateRequestID)
	frisby.Create(fmt.Sprintf("CLA Manager - Delete CLA Manager Request - %s", claManagerCreateRequestID)).
		Delete(url).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
		}).
		Send().
		ExpectStatus(204)
}

// RunAddCLAManagerNoAuth test
func (t *TestBehaviour) RunAddCLAManagerNoAuth() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager",
		t.apiURL, claManagerCompanyID, claManagerProjectID)
	frisby.Create("CLA Manager - Add CLA Manager - No Auth").
		Post(url).
		SetJson(map[string]string{
			"userName":  claProspectiveManagerUserName,
			"userEmail": claProspectiveManagerEmail,
			"userLFID":  claProspectiveManagerLFID,
		}).
		Send().
		ExpectStatus(401)
}

// RunAddCLAManagerUnauthorized test
func (t *TestBehaviour) RunAddCLAManagerUnauthorized() {
	// Prospective Manager should not be able to add CLA managers from the ACL until after he/she is on the ACL list
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager",
		t.apiURL, claManagerCompanyID, claManagerProjectID)
	frisby.Create(fmt.Sprintf("CLA Manager - Add CLA Manager - Unauthorized - %s", claManagerCreateRequestID)).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claProspectiveManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
		}).
		Post(url).
		SetJson(map[string]string{
			"userName":  claProspectiveManagerUserName,
			"userEmail": claProspectiveManagerEmail,
			"userLFID":  claProspectiveManagerLFID,
		}).
		Send().
		ExpectStatus(401)
}

// RunAddCLAManager test
func (t *TestBehaviour) RunAddCLAManager() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager",
		t.apiURL, claManagerCompanyID, claManagerProjectID)
	frisby.Create(fmt.Sprintf("CLA Manager - Add CLA Manager - %s", claProspectiveManagerLFID)).
		Post(url).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
		}).
		SetJson(map[string]string{
			"userName":  claProspectiveManagerUserName,
			"userEmail": claProspectiveManagerEmail,
			"userLFID":  claProspectiveManagerLFID,
		}).
		Send().
		ExpectStatus(200).
		ExpectJsonType("signatureID", reflect.String).
		ExpectJsonType("projectID", reflect.String).
		ExpectJsonType("signatureCreated", reflect.String).
		ExpectJsonType("signatureModified", reflect.String).
		ExpectJsonType("signatureSigned", reflect.Bool).
		ExpectJsonType("signatureApproved", reflect.Bool).
		// ExpectJsonType("signatureEmbargoAcked", reflect.Bool).
		ExpectJsonType("signatureReferenceType", reflect.String).
		ExpectJsonType("signatureReferenceID", reflect.String).
		ExpectJsonType("signatureReferenceName", reflect.String).
		ExpectJsonType("signatureReferenceNameLower", reflect.String).
		ExpectJsonType("signatureType", reflect.String).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			var signature models.Signature
			unmarshallErr := json.Unmarshal([]byte(text), &signature)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			var containsEntry = false
			for _, aclEntry := range signature.SignatureACL {
				if aclEntry.LfUsername == claProspectiveManagerLFID {
					containsEntry = true
				}
			}
			if !containsEntry {
				F.AddError("Add CLA Manager - Signature Response missing ACL Entry")
			}
		})
}

// RunRemoveCLAManagerNoAuth test
func (t *TestBehaviour) RunRemoveCLAManagerNoAuth() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/%s",
		t.apiURL, claManagerCompanyID, claManagerProjectID, claProspectiveManagerLFID)
	frisby.Create("CLA Manager - Remove CLA Manager - No Auth").
		Delete(url).
		Send().
		ExpectStatus(401)
}

// RunRemoveCLAManagerUnauthorized test
func (t *TestBehaviour) RunRemoveCLAManagerUnauthorized() {
	// Prospective Manager should not be able to remove CLA managers from the ACL until after he/she is on the ACL list
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/%s",
		t.apiURL, claManagerCompanyID, claManagerProjectID, claProspectiveManagerLFID)
	frisby.Create(fmt.Sprintf("CLA Manager - Remove CLA Manager - Unauthorized - %s", claManagerCreateRequestID)).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claProspectiveManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
		}).
		Delete(url).
		Send().
		ExpectStatus(401)
}

// RunRemoveCLAManager test
func (t *TestBehaviour) RunRemoveCLAManager() {
	url := fmt.Sprintf("%s/company/%s/project/%s/cla-manager/%s",
		t.apiURL, claManagerCompanyID, claManagerProjectID, claProspectiveManagerLFID)
	frisby.Create(fmt.Sprintf("CLA Manager - Remove CLA Manager - %s", claProspectiveManagerLFID)).
		Delete(url).
		SetHeaders(map[string]string{
			"Authorization":   "Bearer " + claManagerToken,
			"Content-Type":    "application/json",
			"Accept-Encoding": "application/json",
		}).
		Send().
		ExpectStatus(200).
		ExpectJsonType("signatureID", reflect.String).
		ExpectJsonType("projectID", reflect.String).
		ExpectJsonType("signatureCreated", reflect.String).
		ExpectJsonType("signatureModified", reflect.String).
		ExpectJsonType("signatureSigned", reflect.Bool).
		ExpectJsonType("signatureApproved", reflect.Bool).
		// ExpectJsonType("signatureEmbargoAcked", reflect.Bool).
		ExpectJsonType("signatureReferenceType", reflect.String).
		ExpectJsonType("signatureReferenceID", reflect.String).
		ExpectJsonType("signatureReferenceName", reflect.String).
		ExpectJsonType("signatureReferenceNameLower", reflect.String).
		ExpectJsonType("signatureType", reflect.String).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			var signature models.Signature
			unmarshallErr := json.Unmarshal([]byte(text), &signature)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
			var containsEntry = false
			for _, aclEntry := range signature.SignatureACL {
				if aclEntry.LfUsername == claProspectiveManagerLFID {
					containsEntry = true
				}
			}
			if containsEntry {
				F.AddError("Remove CLA Manager - Signature Response contains ACL Entry that should have been removed")
			}
		})
}

// RunAllTests runs all the CLA Manager tests
func (t *TestBehaviour) RunAllTests() {
	// Need our authentication tokens for each persona/user
	t.RunGetCLAManagerToken()
	t.RunGetCLAProspectiveManagerToken()

	// No Credentials/Auth Tests - these should return 401
	t.RunCreateCLAManagerRequestNoAuth()
	t.RunGetCLAManagerRequestsNoAuth()
	t.RunGetCLAManagerRequestNoAuth()
	t.RunApproveCLAManagerRequestNoAuth()
	t.RunDenyCLAManagerRequestNoAuth()
	t.RunDeleteCLAManagerRequestNoAuth()
	t.RunAddCLAManagerNoAuth()
	t.RunRemoveCLAManagerNoAuth()

	// Create a new request and read/verify it
	t.RunCreateCLAManagerRequest()
	t.RunGetCLAManagerRequests()
	t.RunGetCLAManagerRequest()

	// Shouldn't be allowed to Approve/Deny/Delete unless you are already a CLA Manager
	t.RunApproveCLAManagerRequestUnauthorized()
	t.RunDenyCLAManagerRequestUnauthorized()
	t.RunDeleteCLAManagerRequestUnauthorized()
	t.RunAddCLAManagerUnauthorized()
	t.RunRemoveCLAManagerUnauthorized()

	// Approve this request - should update status and add user to ACL
	t.RunApproveCLAManagerRequest()

	// Cleanup - Remove the Request
	t.RunDeleteCLAManagerRequest()
	// Cleanup - Remove the User from the ACL
	t.RunRemoveCLAManager()

	// Manually Add user (CLA Manager manually adds, not part of a designee request flow)
	t.RunAddCLAManager()
	t.RunRemoveCLAManager()
}
