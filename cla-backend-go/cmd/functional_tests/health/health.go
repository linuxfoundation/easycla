// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package health

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/linuxfoundation/easycla/cla-backend-go/cmd/functional_tests/test_models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/verdverm/frisby"
)

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

// RunAllTests runs all the CLA Group tests
func (t *TestBehaviour) RunAllTests() {
	frisby.Create("Health and Status").
		Get(t.apiURL+"/ops/health").
		Send().
		ExpectStatus(200).
		ExpectJsonType("Branch", reflect.String).
		ExpectJsonType("BuildTimeStamp", reflect.String).
		ExpectJsonType("Githash", reflect.String).
		ExpectJsonType("Healths", reflect.Slice).
		AfterText(func(F *frisby.Frisby, text string, err error) {
			var healthModel models.Health
			unmarshallErr := json.Unmarshal([]byte(text), &healthModel)
			F.Expect(func(F *frisby.Frisby) (bool, string) {
				return unmarshallErr == nil, fmt.Sprintf("Success unmarshalling JSON response: %+v", unmarshallErr)
			})
			for _, healthItem := range healthModel.Healths {
				if !healthItem.Healthy || healthItem.Error != "" {
					F.AddError(fmt.Sprintf("%s is not health - error: %s", healthItem.Name, healthItem.Error))
				}
			}
		})
}
