package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var (
	TOKEN                string
	XACL                 string
	PY_API_URL           string
	GO_API_URL           string
	DEBUG                bool
	PROJECT_UUID         string
	ProjectAPIPath       = [2]string{"/v2/project/%s", "/v4/project/%s"}
	ProjectAPIKeyMapping = map[string]string{
		"date_created":                         "dateCreated",
		"date_modified":                        "dateModified",
		"project_name":                         "projectName",
		"foundation_sfid":                      "foundationSFID",
		"project_acl":                          "projectACL",
		"project_ccla_enabled":                 "projectCCLAEnabled",
		"project_ccla_requires_icla_signature": "projectCCLARequiresICLA",
		"project_icla_enabled":                 "projectICLAEnabled",
		"project_id":                           "projectID",
		"project_live":                         "projectLive",
		"version":                              "version",
		// "project_individual_documents":        "projectIndividualDocuments",
		// "project_corporate_documents":         "projectCorporateDocuments",
		// "project_member_documents":            "projectMemberDocuments",
		// "signed_at_foundation_level":           "foundationLevelCLA",
		// "root_project_repositories_count":      "rootProjectRepositoriesCount",
	}
)

func init() {
	TOKEN = os.Getenv("TOKEN")
	XACL = os.Getenv("XACL")
	PY_API_URL = os.Getenv("PY_API_URL")
	if PY_API_URL == "" {
		PY_API_URL = "http://127.0.0.1:5000"
	}
	GO_API_URL = os.Getenv("GO_API_URL")
	if GO_API_URL == "" {
		GO_API_URL = "http://127.0.0.1:5001"
	}
	dbg := os.Getenv("DEBUG")
	if dbg != "" {
		DEBUG = true
	}
	PROJECT_UUID = os.Getenv("PROJECT_UUID")
}

func tryParseTime(val interface{}) (time.Time, bool) {
	str, ok := val.(string)
	if !ok {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000000Z0700",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, str); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func compareMappedFields(t *testing.T, pyData, goData map[string]interface{}, keyMapping map[string]string, dbg bool) {
	for pyKey, goKey := range keyMapping {
		if dbg {
			fmt.Printf("checking %s - %s\n", pyKey, goKey)
		}

		pyVal, pyOk := pyData[pyKey]
		goVal, goOk := goData[goKey]

		if !pyOk {
			t.Errorf("Missing key in Python response: %s", pyKey)
			continue
		}
		if !goOk {
			t.Errorf("Missing key in Go response: %s", goKey)
			continue
		}

		pyTime, okPyTime := tryParseTime(pyVal)
		goTime, okGoTime := tryParseTime(goVal)

		if okPyTime && okGoTime {
			if !pyTime.Equal(goTime) {
				t.Errorf("Datetime mismatch for key '%s' (Go: '%s'): %s != %s", pyKey, goKey, pyTime, goTime)
			}
			continue
		}

		if fmt.Sprint(pyVal) != fmt.Sprint(goVal) {
			t.Errorf("Mismatch for key '%s' (Go: '%s'): %v != %v", pyKey, goKey, pyVal, goVal)
		}
	}
}

func TestProjectAPI(t *testing.T) {
	if TOKEN == "" || XACL == "" {
		t.Fatalf("TOKEN and XACL environment variables must be set")
	}
	projectId := PROJECT_UUID
	if projectId == "" {
		projectId = uuid.New().String()
		putTestItem("projects", "project_id", projectId, "S", map[string]interface{}{
			"project_name":         "CNCF",
			"project_icla_enabled": true,
			"project_ccla_enabled": true,
			"date_created":         "2022-11-21T10:31:31Z",
			"date_modified":        "2023-02-23T13:14:48Z",
			"foundation_sfid":      "sfid",
			"version":              "2",
		}, DEBUG)
		defer deleteTestItem("projects", "project_id", projectId, "S", DEBUG)
	}

	apiURL := PY_API_URL + fmt.Sprintf(ProjectAPIPath[0], projectId)
	if DEBUG {
		fmt.Printf("Py API call: %s\n", apiURL)
	}
	oldResp, err := http.Get(apiURL)
	if err != nil {
		t.Fatalf("Failed to call API: %v", err)
	}
	assert.Equal(t, http.StatusOK, oldResp.StatusCode, "Expected 200 from PY API")
	defer oldResp.Body.Close()
	oldBody, _ := io.ReadAll(oldResp.Body)
	var oldJSON interface{}
	err = json.Unmarshal(oldBody, &oldJSON)
	assert.NoError(t, err)
	if DEBUG {
		fmt.Printf("Py raw response: %+v\n", string(oldBody))
		fmt.Printf("Py response: %+v\n", oldJSON)
	}

	apiURL = GO_API_URL + fmt.Sprintf(ProjectAPIPath[1], projectId)
	if DEBUG {
		fmt.Printf("Go API call: %s\n", apiURL)
	}
	// newResp, err := http.Get(apiURL)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+TOKEN)
	req.Header.Set("X-ACL", XACL)

	newResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to call API: %v", err)
	}
	assert.Equal(t, http.StatusOK, newResp.StatusCode, "Expected 200 from GO API")
	defer newResp.Body.Close()
	newBody, _ := io.ReadAll(newResp.Body)
	var newJSON interface{}
	err = json.Unmarshal(newBody, &newJSON)
	assert.NoError(t, err)
	if DEBUG {
		fmt.Printf("Go raw Response: %+v\n", string(newBody))
		fmt.Printf("Go response: %+v\n", newJSON)
	}

	// For full equality
	// Strict
	// assert.Equal(t, oldJSON, newJSON)
	// Smart - ignore keys order
	// assert.JSONEq(t, string(oldBody), string(newBody))
	oldMap, ok1 := oldJSON.(map[string]interface{})
	newMap, ok2 := newJSON.(map[string]interface{})

	if !ok1 || !ok2 {
		t.Fatalf("Expected both responses to be JSON objects")
	}
	compareMappedFields(t, oldMap, newMap, ProjectAPIKeyMapping, DEBUG)

	if DEBUG {
		oky := []string{}
		for k, _ := range oldMap {
			oky = append(oky, k)
		}
		sort.Strings(oky)
		nky := []string{}
		for k, _ := range newMap {
			nky = append(nky, k)
		}
		sort.Strings(nky)
		fmt.Printf("old keys: %+v\n", oky)
		fmt.Printf("new keys: %+v\n", nky)
	}
}
