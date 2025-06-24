// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/aws/aws-sdk-go/aws/awsutil"

	"github.com/linuxfoundation/easycla/cla-backend-go/cmd/functional_tests/test_models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/go-openapi/swag"
	"github.com/verdverm/frisby"
)

var (
	claProjectManagerToken string
)

const (
	// OpenColor IO project sfid
	projectSFID = "a092M00001IV3zdQAD"
	// repository id for the repo : LF-Engineering/easycla-protected-branch-test-repo
	repositoryID = "debcb153-69b6-4b4a-8020-213e3bf471b3"
)

// GetXACL returns the X-ACL entry for this company
func GetXACL() (string, error) {
	xACL := auth.ACL{
		Admin:    false,
		Allowed:  true,
		Resource: "cla_groups",
		Context:  "staff",
		Scopes: []auth.Scope{
			{
				Type:  "project",
				ID:    projectSFID,
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
	fmt.Printf("auth config : %+v \n", t.auth0ProjectManagerConfig)
	authTokenReqPayload := map[string]string{
		"grant_type": "http://auth0.com/oauth/grant-type/password-realm",
		"realm":      "Username-Password-Authentication",
		"username":   t.auth0ProjectManagerConfig.Auth0UserName,
		"password":   t.auth0ProjectManagerConfig.Auth0Password,
		"client_id":  t.auth0ProjectManagerConfig.Auth0ClientID,
		"audience":   "https://api-gw.dev.platform.linuxfoundation.org/",
		"scope":      "access:api openid profile email",
	}

	frisby.Create(fmt.Sprintf("Get Protected Branch - Get Token - Project Manager Manager - %s", t.auth0ProjectManagerConfig.Auth0UserName)).
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
	xacl, err := GetXACL()
	if err != nil {
		panic(err)
	}
	return map[string]string{
		"Authorization":   "Bearer " + claProjectManagerToken,
		"Content-Type":    "application/json",
		"Accept-Encoding": "application/json",
		"X-Email":         t.auth0ProjectManagerConfig.Auth0Email,
		"X-USERNAME":      t.auth0ProjectManagerConfig.Auth0UserName,
		"X-ACL":           xacl,
	}
}

// RunGetProtectedBranch test the response for the protected branch of repo
func (t *TestBehaviour) RunGetProtectedBranch(assertBranchProtection *models.GithubRepositoryBranchProtection) {

	endpoint := fmt.Sprintf("%s/project/%s/github/repositories/%s/branch-protection", t.apiURL, projectSFID, repositoryID)
	frisby.Create(fmt.Sprintf("Get Protected Branch - ProjectSFID : %s, RepositoryID: %s", projectSFID, repositoryID)).
		Get(endpoint).
		SetHeaders(t.getCLAProjectManagerHeaders()).
		Send().
		ExpectStatus(200).
		ExpectJsonType("branch_name", reflect.String).
		ExpectJsonType("enforce_admin", reflect.Bool).
		ExpectJsonType("protection_enabled", reflect.Bool).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			if err != nil {
				F.AddError(err.Error())
				return
			}
			var response models.GithubRepositoryBranchProtection
			unmarshallErr := json.Unmarshal([]byte(text), &response)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
				return
			}

			if response.BranchName == nil || *response.BranchName != "master" {
				F.AddError("Get Protected Branch - Default Branch Name expected : master")
			}

			if len(response.StatusChecks) == 0 {
				F.AddError("Get Protected Branch - No Status Checks Returned by API")
			} else {
				var found bool
				for _, c := range response.StatusChecks {
					if *c.Name == "EasyCLA" {
						found = true
						break
					}
				}
				if !found {
					F.AddError("Get Protected Branch - EasyCLA not found it status checks (even if not enabled)")
				}
			}

			if assertBranchProtection != nil {
				if !awsutil.DeepEqual(response, assertBranchProtection) {
					F.AddError(fmt.Sprintf("Get Protected Branch - Expected Result Not found - Expected : %+v - Got : %+v", assertBranchProtection, response))
				}
			}
		})
}

// RunUpdateProtectionBranch is hits the branch protection endpoint with the given parameters
func (t *TestBehaviour) RunUpdateProtectionBranch(msg string, param *models.GithubRepositoryBranchProtectionInput) {
	//it should be checking enforce admin if worked
	//it should be checking if all of the enabled checks are there and disabled are not there as well
	endpoint := fmt.Sprintf("%s/project/%s/github/repositories/%s/branch-protection", t.apiURL, projectSFID, repositoryID)
	frisby.Create(fmt.Sprintf("Update Protection  Branch - %s - ProjectSFID : %s, RepositoryID: %s", msg, projectSFID, repositoryID)).
		Post(endpoint).
		SetHeaders(t.getCLAProjectManagerHeaders()).
		SetJson(param).
		Send().
		ExpectStatus(200).
		ExpectJsonType("branch_name", reflect.String).
		ExpectJsonType("enforce_admin", reflect.Bool).
		ExpectJsonType("protection_enabled", reflect.Bool).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			var response models.GithubRepositoryBranchProtection
			unmarshallErr := json.Unmarshal([]byte(text), &response)
			if unmarshallErr != nil {
				F.AddError(unmarshallErr.Error())
			}
		})

	t.RunGetProtectedBranch(&models.GithubRepositoryBranchProtection{
		BranchName:        swag.String("master"),
		EnforceAdmin:      *param.EnforceAdmin,
		ProtectionEnabled: true,
		StatusChecks:      param.StatusChecks,
	})
}

// RunAllTests runs all the CLA Manager tests
func (t *TestBehaviour) RunAllTests() {
	// Need our authentication tokens for each persona/user
	t.RunGetCLAProjectManagerToken()
	// do an initial get query
	t.RunGetProtectedBranch(nil)
	// first enable it
	t.RunUpdateProtectionBranch("Enable Protections", &models.GithubRepositoryBranchProtectionInput{
		EnforceAdmin: swag.Bool(true),
		StatusChecks: []*models.GithubRepositoryBranchProtectionStatusChecks{
			{Name: swag.String("EasyCLA"), Enabled: swag.Bool(true)},
		},
	})
	//then disable it again
	t.RunUpdateProtectionBranch("Disable Protections", &models.GithubRepositoryBranchProtectionInput{
		EnforceAdmin: swag.Bool(false),
		StatusChecks: []*models.GithubRepositoryBranchProtectionStatusChecks{
			{Name: swag.String("EasyCLA"), Enabled: swag.Bool(false)},
		},
	})
}
