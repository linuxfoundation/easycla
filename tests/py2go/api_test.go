package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

const (
	ExampleRepoID         = 466156917
	ExamplePRNumber       = 3
	InvalidUUIDProvided   = "Invalid UUID provided"
	GoUUIDValidationError = " in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{12}$'"
)

var (
	TOKEN                string
	XACL                 string
	PY_API_URL           string
	GO_API_URL           string
	DEBUG                bool
	MAX_PARALLEL         int
	PROJECT_UUID         string
	USER_UUID            string
	REPO_ID              int64
	PR_ID                int64
	ProjectAPIPath       = [3]string{"/v2/project/%s", "/v4/project/%s", "/v4/project-compat/%s"}
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
	ProjectCompatAPIKeyMapping = map[string]interface{}{
		"foundation_sfid":                      nil,
		"project_ccla_enabled":                 nil,
		"project_ccla_requires_icla_signature": nil,
		"project_icla_enabled":                 nil,
		"project_id":                           nil,
		"project_name":                         nil,
		"project_individual_documents": []interface{}{map[string]interface{}{
			"document_major_version": nil,
			"document_minor_version": nil,
		}},
		"project_corporate_documents": []interface{}{map[string]interface{}{
			"document_major_version": nil,
			"document_minor_version": nil,
		}},
		"projects": []interface{}{map[string]interface{}{
			"cla_group_id":    nil,
			"foundation_sfid": nil,
			"project_name":    nil,
			"project_sfid":    nil,
			"github_repos":    []interface{}{map[string]interface{}{"repository_name": nil}},
			"gitlab_repos":    []interface{}{map[string]interface{}{"repository_name": nil}},
			"gerrit_repos":    []interface{}{map[string]interface{}{"gerrit_url": nil}},
		}},
		// "signed_at_foundation_level":           nil,
	}
	ProjectCompatAPISortMap = map[string]interface{}{
		"github_repos": "repository_name",
		"gitlab_repos": "repository_name",
		"gerrit_repos": "gerrit_url",
	}
	UserActiveSignatureAPIPath = [2]string{"/v2/user/%s/active-signature", "/v4/user/%s/active-signature"}
	// Optional field: true means the key may be missing in both APIs and still be valid
	UserActiveSignatureAPIKeyMapping = map[string]interface{}{
		"project_id":       nil,
		"pull_request_id":  nil,
		"repository_id":    nil,
		"return_url":       nil,
		"user_id":          nil,
		"merge_request_id": true,
	}
	UserCompatAPIPath = [2]string{"/v2/user/%s", "/v3/user-compat/%s"}
	// If the value is array with an empty interface it means that each array element should be checked
	UserCompatAPIKeyMapping = map[string]interface{}{
		"lf_email":             nil,
		"lf_sub":               nil,
		"lf_username":          nil,
		"note":                 nil,
		"user_company_id":      nil,
		"user_external_id":     nil,
		"user_github_id":       nil,
		"user_github_username": nil,
		"user_gitlab_id":       nil,
		"user_gitlab_username": nil,
		"user_id":              nil,
		"user_ldap_id":         nil,
		"user_name":            nil,
		"is_sanctioned":        nil,
		"version":              nil,
		"user_emails":          []interface{}{map[string]interface{}{}},
	}
	UserCompatAPISortMap = map[string]interface{}{
		"user_emails": nil,
	}
)

func init() {
	TOKEN = os.Getenv("TOKEN")
	XACL = os.Getenv("XACL")
	PY_API_URL = os.Getenv("PY_API_URL")
	switch PY_API_URL {
	case "local", "":
		PY_API_URL = "http://127.0.0.1:5000"
	case "dev":
		PY_API_URL = "https://api.lfcla.dev.platform.linuxfoundation.org"
	case "prod":
		PY_API_URL = "https://api.easycla.lfx.linuxfoundation.org"
	}
	GO_API_URL = os.Getenv("GO_API_URL")
	switch GO_API_URL {
	case "local", "":
		GO_API_URL = "http://127.0.0.1:5001"
	case "dev":
		GO_API_URL = "https://api-gw.dev.platform.linuxfoundation.org/cla-service"
	case "prod":
		GO_API_URL = "https://api-gw.platform.linuxfoundation.org/cla-service"
	}
	DEBUG = os.Getenv("DEBUG") != ""
	MAX_PARALLEL = runtime.NumCPU()
	par := os.Getenv("MAX_PARALLEL")
	if par != "" {
		iPar, err := strconv.Atoi(par)
		if err != nil {
			fmt.Printf("MAX_PARALLEL environment value should be integer >= 1\n")
		} else if iPar > 0 {
			MAX_PARALLEL = iPar
		}
	}
	PROJECT_UUID = os.Getenv("PROJECT_UUID")
	USER_UUID = os.Getenv("USER_UUID")
	REPO_ID = ExampleRepoID
	par = os.Getenv("REPO_ID")
	if par != "" {
		iPar, err := strconv.ParseInt(par, 10, 64)
		if err != nil {
			fmt.Printf("REPO_ID environment value should be integer >= 1\n")
		} else if iPar > 0 {
			REPO_ID = iPar
		}
	}
	PR_ID = ExamplePRNumber
	par = os.Getenv("PR_ID")
	if par != "" {
		iPar, err := strconv.ParseInt(par, 10, 64)
		if err != nil {
			fmt.Printf("PR_ID environment value should be integer >= 1\n")
		} else if iPar > 0 {
			PR_ID = iPar
		}
	}
	rand.Seed(time.Now().UnixNano())
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

func compareMappedFields(t *testing.T, pyData, goData map[string]interface{}, keyMapping map[string]string) {
	for pyKey, goKey := range keyMapping {
		Debugf("checking %s - %s\n", pyKey, goKey)

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
				t.Errorf("Datetime mismatch for key '%s' (Go: '%s'): py:%s != go:%s", pyKey, goKey, pyTime, goTime)
			}
			continue
		}

		if (pyVal == nil && goVal == "") || (goVal == nil && pyVal == "") {
			continue
		}

		if fmt.Sprint(pyVal) != fmt.Sprint(goVal) {
			t.Errorf("Mismatch for key '%s' (Go: '%s'): py:%+v != go:%+v", pyKey, goKey, pyVal, goVal)
		}
	}
}

func sortByKey(arr []interface{}, key interface{}) {
	keyStr, okStr := key.(string)
	if okStr {
		sort.Slice(arr, func(i, j int) bool {
			m1, _ := arr[i].(map[string]interface{})
			m2, _ := arr[j].(map[string]interface{})
			s1 := fmt.Sprint(m1[keyStr])
			s2 := fmt.Sprint(m2[keyStr])
			return s1 < s2
		})
	} else {
		sort.Slice(arr, func(i, j int) bool {
			return fmt.Sprintf("%v", arr[i]) < fmt.Sprintf("%v", arr[j])
		})
	}
}

func compareNestedFields(t *testing.T, pyData, goData, keyMapping map[string]interface{}, sortMap map[string]interface{}) {
	for k, v := range keyMapping {
		bV, bVOK := v.(bool)
		if v == nil || bVOK {
			Debugf("checking values of '%s'\n", k)
		}

		pyVal, pyOk := pyData[k]
		goVal, goOk := goData[k]

		// true means fields are optional (nullable), so if v is true and fileds are missing in both Py and go then this is OK
		if bVOK && bV && !pyOk && !goOk {
			Debugf("'%s' is not set in both responses, this is ok\n", k)
			continue
		}
		if !pyOk {
			t.Errorf("Missing key in Python response: %s", k)
			continue
		}
		if !goOk {
			t.Errorf("Missing key in Go response: %s", k)
			continue
		}

		nestedMapping, nested := v.(map[string]interface{})
		if nested {
			Debugf("checking nested object '%s'\n", k)
			pyNestedVal, pyOk := pyVal.(map[string]interface{})
			goNestedVal, goOk := goVal.(map[string]interface{})
			if !pyOk {
				t.Errorf("%s value in Python response is not a nested object: %+v", k, pyVal)
				continue
			}
			if !goOk {
				t.Errorf("%s value in Go response is not a nested object: %+v", k, goVal)
				continue
			}
			compareNestedFields(t, pyNestedVal, goNestedVal, nestedMapping, sortMap)
			continue
		}

		arrayMapping, array := v.([]interface{})
		if array {
			Debugf("checking nested array '%s'\n", k)
			if len(arrayMapping) < 1 {
				t.Errorf("%s value in key mapping should be array of single object: %+v", k, v)
				continue
			}
			nestedMapping, nested := arrayMapping[0].(map[string]interface{})
			if !nested {
				t.Errorf("%s value in key mapping should be array of single object: %+v", k, v)
				continue
			}
			pyArrayVal, pyOk := pyVal.([]interface{})
			goArrayVal, goOk := goVal.([]interface{})
			if pyOk && len(pyArrayVal) == 0 && goVal == nil {
				Debugf("py returned [] and go returned nil - this is OK\n")
				continue
			}
			if goOk && len(goArrayVal) == 0 && pyVal == nil {
				Debugf("py returned null and go returned [] - this is OK\n")
				continue
			}
			if goVal == nil && pyVal == nil {
				Debugf("both py and go returned null - this is OK\n")
				continue
			}
			if !pyOk {
				t.Errorf("%s value in Python response is not an array: %+v", k, pyVal)
				continue
			}
			if !goOk {
				t.Errorf("%s value in Go response is not an array: %+v", k, goVal)
				continue
			}
			lenPyArrayVal := len(pyArrayVal)
			lenGoArrayVal := len(goArrayVal)
			if lenPyArrayVal != lenGoArrayVal {
				t.Errorf("%s arrays length mismatch: %d != %d", k, lenPyArrayVal, lenGoArrayVal)
				continue
			}
			sortKey, needSort := sortMap[k]
			if needSort {
				Debugf("sorting '%s' key values by %v\n", k, sortKey)
				sortByKey(pyArrayVal, sortKey)
				sortByKey(goArrayVal, sortKey)
			}
			// This is to support plain arrays - they have single item: []interface{}{map[string]interface{}{}}
			if len(nestedMapping) == 0 {
				for idx := range pyArrayVal {
					pyVal := pyArrayVal[idx]
					goVal := goArrayVal[idx]
					pyTime, okPyTime := tryParseTime(pyVal)
					goTime, okGoTime := tryParseTime(goVal)

					if okPyTime && okGoTime {
						if !pyTime.Equal(goTime) {
							t.Errorf("Datetime mismatch for key '%s'[%d]: py:%s != go:%s", k, idx, pyTime, goTime)
						}
						continue
					}

					if (pyVal == nil && goVal == "") || (goVal == nil && pyVal == "") {
						continue
					}

					if fmt.Sprint(pyVal) != fmt.Sprint(goVal) {
						t.Errorf("Mismatch for key '%s'[%d]: py:%+v != go:%+v", k, idx, pyVal, goVal)
					}
				}
				continue
			}
			for idx := range pyArrayVal {
				pyNestedVal, pyOk := pyArrayVal[idx].(map[string]interface{})
				goNestedVal, goOk := goArrayVal[idx].(map[string]interface{})
				if !pyOk {
					t.Errorf("%s:%d value in Python response is not a nested object: %+v", k, idx, pyArrayVal[idx])
					continue
				}
				if !goOk {
					t.Errorf("%s:%d value in Go response is not a nested object: %+v", k, idx, goArrayVal[idx])
					continue
				}
				compareNestedFields(t, pyNestedVal, goNestedVal, nestedMapping, sortMap)
			}
			continue
		}

		pyTime, okPyTime := tryParseTime(pyVal)
		goTime, okGoTime := tryParseTime(goVal)

		if okPyTime && okGoTime {
			if !pyTime.Equal(goTime) {
				t.Errorf("Datetime mismatch for key '%s': py:%s != go:%s", k, pyTime, goTime)
			}
			continue
		}

		if (pyVal == nil && goVal == "") || (goVal == nil && pyVal == "") {
			continue
		}

		if fmt.Sprint(pyVal) != fmt.Sprint(goVal) {
			t.Errorf("Mismatch for key '%s': py:%+v != go:%+v", k, pyVal, goVal)
		}
	}
}

func expectedPyInvalidUUID(field string) map[string]interface{} {
	return map[string]interface{}{
		"errors": map[string]interface{}{
			field: InvalidUUIDProvided,
		},
	}
}

func expectedGoInvalidUUID(field string) map[string]interface{} {
	return map[string]interface{}{
		"code":    float64(605),
		"message": field + GoUUIDValidationError,
	}
}

func runProjectCompatAPIForProject(t *testing.T, projectId string) {
	apiURL := PY_API_URL + fmt.Sprintf(ProjectAPIPath[0], projectId)
	Debugf("Py API call: %s\n", apiURL)
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
	Debugf("Py raw response: %+v\n", string(oldBody))
	Debugf("Py response: %+v\n", oldJSON)

	apiURL = GO_API_URL + fmt.Sprintf(ProjectAPIPath[2], projectId)
	Debugf("Go API call: %s\n", apiURL)
	newResp, err := http.Get(apiURL)
	if err != nil {
		t.Fatalf("Failed to call API: %v", err)
	}
	assert.Equal(t, http.StatusOK, newResp.StatusCode, "Expected 200 from GO API")
	defer newResp.Body.Close()
	newBody, _ := io.ReadAll(newResp.Body)
	var newJSON interface{}
	err = json.Unmarshal(newBody, &newJSON)
	assert.NoError(t, err)
	Debugf("Go raw Response: %+v\n", string(newBody))
	Debugf("Go response: %+v\n", newJSON)

	oldMap, ok1 := oldJSON.(map[string]interface{})
	newMap, ok2 := newJSON.(map[string]interface{})

	if !ok1 || !ok2 {
		t.Fatalf("Expected both responses to be JSON objects")
	}
	compareNestedFields(t, oldMap, newMap, ProjectCompatAPIKeyMapping, ProjectCompatAPISortMap)

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
		Debugf("old keys: %+v\n", oky)
		Debugf("new keys: %+v\n", nky)
	}
}

func runProjectCompatAPIForProjectExpectFail(t *testing.T, projectId string) {
	apiURL := PY_API_URL + fmt.Sprintf(ProjectAPIPath[0], projectId)
	Debugf("Py API call: %s\n", apiURL)
	oldResp, err := http.Get(apiURL)
	if err != nil {
		t.Fatalf("Failed to call API: %v", err)
	}
	assert.Equal(t, http.StatusBadRequest, oldResp.StatusCode, "Expected 400 from Py API")
	defer oldResp.Body.Close()
	oldBody, _ := io.ReadAll(oldResp.Body)
	var oldJSON interface{}
	err = json.Unmarshal(oldBody, &oldJSON)
	assert.NoError(t, err)
	Debugf("Py raw response: %+v\n", string(oldBody))
	Debugf("Py response: %+v\n", oldJSON)

	apiURL = GO_API_URL + fmt.Sprintf(ProjectAPIPath[2], projectId)
	Debugf("Go API call: %s\n", apiURL)
	newResp, err := http.Get(apiURL)
	if err != nil {
		t.Fatalf("Failed to call API: %v", err)
	}
	assert.Equal(t, http.StatusUnprocessableEntity, newResp.StatusCode, "Expected 422 from Go API")
	defer newResp.Body.Close()
	newBody, _ := io.ReadAll(newResp.Body)
	var newJSON interface{}
	err = json.Unmarshal(newBody, &newJSON)
	assert.NoError(t, err)
	Debugf("Go raw Response: %+v\n", string(newBody))
	Debugf("Go response: %+v\n", newJSON)

	assert.Equal(t, expectedPyInvalidUUID("project_id"), oldJSON)
	assert.Equal(t, expectedGoInvalidUUID("projectID"), newJSON)
}

func TestProjectCompatAPI(t *testing.T) {
	projectId := PROJECT_UUID
	if projectId == "" {
		projectId = uuid.New().String()
		putTestItem("projects", "project_id", projectId, "S", map[string]interface{}{
			"project_name":                         "CNCF",
			"project_icla_enabled":                 true,
			"project_ccla_enabled":                 true,
			"project_ccla_requires_icla_signature": true,
			"date_created":                         "2022-11-21T10:31:31Z",
			"date_modified":                        "2023-02-23T13:14:48Z",
			"foundation_sfid":                      "a09410000182dD2AAI",
			"version":                              "2",
		}, DEBUG)
		defer deleteTestItem("projects", "project_id", projectId, "S", DEBUG)
	}

	runProjectCompatAPIForProject(t, projectId)
}

func TestProjectCompatAPIWithNonV4UUID(t *testing.T) {
	projectId := "6ba7b810-9dad-11d1-80b4-00c04fd430c8" // Non-v4 UUID
	putTestItem("projects", "project_id", projectId, "S", map[string]interface{}{
		"project_name":                         "CNCF",
		"project_icla_enabled":                 true,
		"project_ccla_enabled":                 true,
		"project_ccla_requires_icla_signature": true,
		"date_created":                         "2022-11-21T10:31:31Z",
		"date_modified":                        "2023-02-23T13:14:48Z",
		"foundation_sfid":                      "a09410000182dD2AAI",
		"version":                              "2",
	}, DEBUG)
	defer deleteTestItem("projects", "project_id", projectId, "S", DEBUG)

	runProjectCompatAPIForProject(t, projectId)
}

func TestProjectCompatAPIWithInvalidUUID(t *testing.T) {
	projectId := "6ba7b810-9dad-11d1-80b4-00c04fd430cg" // Invalid UUID - "g" is not a hex digit
	putTestItem("projects", "project_id", projectId, "S", map[string]interface{}{
		"project_name":                         "CNCF",
		"project_icla_enabled":                 true,
		"project_ccla_enabled":                 true,
		"project_ccla_requires_icla_signature": true,
		"date_created":                         "2022-11-21T10:31:31Z",
		"date_modified":                        "2023-02-23T13:14:48Z",
		"foundation_sfid":                      "a09410000182dD2AAI",
		"version":                              "2",
	}, DEBUG)
	defer deleteTestItem("projects", "project_id", projectId, "S", DEBUG)

	runProjectCompatAPIForProjectExpectFail(t, projectId)
}

func TestAllProjectsCompatAPI(t *testing.T) {
	allProjects := getAllPrimaryKeys("projects", "project_id", "S")

	var failedProjects []string
	var mtx sync.Mutex
	sem := make(chan struct{}, MAX_PARALLEL)
	var wg sync.WaitGroup

	for _, projectID := range allProjects {
		projID, ok := projectID.(string)
		if !ok {
			t.Errorf("Expected string project_id, got: %T", projectID)
			continue
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(projID string) {
			defer wg.Done()
			defer func() { <-sem }()

			// Use t.Run in a thread-safe wrapper with a dummy parent test
			t.Run(fmt.Sprintf("ProjectId=%s", projID), func(t *testing.T) {
				runProjectCompatAPIForProject(t, projID)
				if t.Failed() {
					mtx.Lock()
					failedProjects = append(failedProjects, projID)
					mtx.Unlock()
				}
			})
		}(projID)
	}

	wg.Wait()

	if len(failedProjects) > 0 {
		fmt.Fprintf(os.Stderr, "\nFailed Project IDs (%d):\n%s\n\n",
			len(failedProjects),
			strings.Join(failedProjects, "\n"),
		)
		t.Fail() // Mark test as failed
	} else {
		fmt.Println("\nAll projects passed.")
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
			"project_name":                         "CNCF",
			"project_icla_enabled":                 true,
			"project_ccla_enabled":                 true,
			"project_ccla_requires_icla_signature": true,
			"date_created":                         "2022-11-21T10:31:31Z",
			"date_modified":                        "2023-02-23T13:14:48Z",
			"foundation_sfid":                      "a09410000182dD2AAI",
			"version":                              "2",
		}, DEBUG)
		defer deleteTestItem("projects", "project_id", projectId, "S", DEBUG)
	}

	apiURL := PY_API_URL + fmt.Sprintf(ProjectAPIPath[0], projectId)
	Debugf("Py API call: %s\n", apiURL)
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
	Debugf("Py raw response: %+v\n", string(oldBody))
	Debugf("Py response: %+v\n", oldJSON)

	apiURL = GO_API_URL + fmt.Sprintf(ProjectAPIPath[1], projectId)
	Debugf("Go API call: %s\n", apiURL)
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
	Debugf("Go raw Response: %+v\n", string(newBody))
	Debugf("Go response: %+v\n", newJSON)

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
	compareMappedFields(t, oldMap, newMap, ProjectAPIKeyMapping)

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
		Debugf("old keys: %+v\n", oky)
		Debugf("new keys: %+v\n", nky)
	}
}

func runUserActiveSignatureAPIForUser(t *testing.T, userId string) {
	apiURL := PY_API_URL + fmt.Sprintf(UserActiveSignatureAPIPath[0], userId)
	Debugf("Py API call: %s\n", apiURL)
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
	Debugf("Py raw response: %+v\n", string(oldBody))
	Debugf("Py response: %+v\n", oldJSON)

	apiURL = GO_API_URL + fmt.Sprintf(UserActiveSignatureAPIPath[1], userId)
	Debugf("Go API call: %s\n", apiURL)
	newResp, err := http.Get(apiURL)
	if err != nil {
		t.Fatalf("Failed to call API: %v", err)
	}
	assert.Equal(t, http.StatusOK, newResp.StatusCode, "Expected 200 from GO API")
	defer newResp.Body.Close()
	newBody, _ := io.ReadAll(newResp.Body)
	var newJSON interface{}
	err = json.Unmarshal(newBody, &newJSON)
	assert.NoError(t, err)
	Debugf("Go raw Response: %+v\n", string(newBody))
	Debugf("Go response: %+v\n", newJSON)

	if string(oldBody) == "null" && string(newBody) == "null" {
		return
	}

	oldMap, ok1 := oldJSON.(map[string]interface{})
	newMap, ok2 := newJSON.(map[string]interface{})

	if !ok1 || !ok2 {
		t.Fatalf("Expected both responses to be JSON objects")
	}
	compareNestedFields(t, oldMap, newMap, UserActiveSignatureAPIKeyMapping, map[string]interface{}{})

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
		Debugf("old keys: %+v\n", oky)
		Debugf("new keys: %+v\n", nky)
	}
}

func runUserActiveSignatureAPIForUserExpectFail(t *testing.T, userId string) {
	apiURL := PY_API_URL + fmt.Sprintf(UserActiveSignatureAPIPath[0], userId)
	Debugf("Py API call: %s\n", apiURL)
	oldResp, err := http.Get(apiURL)
	if err != nil {
		t.Fatalf("Failed to call API: %v", err)
	}
	assert.Equal(t, http.StatusBadRequest, oldResp.StatusCode, "Expected 400 from Py API")
	defer oldResp.Body.Close()
	oldBody, _ := io.ReadAll(oldResp.Body)
	var oldJSON interface{}
	err = json.Unmarshal(oldBody, &oldJSON)
	assert.NoError(t, err)
	Debugf("Py raw response: %+v\n", string(oldBody))
	Debugf("Py response: %+v\n", oldJSON)

	apiURL = GO_API_URL + fmt.Sprintf(UserActiveSignatureAPIPath[1], userId)
	Debugf("Go API call: %s\n", apiURL)
	newResp, err := http.Get(apiURL)
	if err != nil {
		t.Fatalf("Failed to call API: %v", err)
	}
	assert.Equal(t, http.StatusUnprocessableEntity, newResp.StatusCode, "Expected 422 from Go API")
	defer newResp.Body.Close()
	newBody, _ := io.ReadAll(newResp.Body)
	var newJSON interface{}
	err = json.Unmarshal(newBody, &newJSON)
	assert.NoError(t, err)
	Debugf("Go raw Response: %+v\n", string(newBody))
	Debugf("Go response: %+v\n", newJSON)

	assert.Equal(t, expectedPyInvalidUUID("user_id"), oldJSON)
	assert.Equal(t, expectedGoInvalidUUID("userID"), newJSON)
}

func TestUserActiveSignatureAPIWithInvalidUUID(t *testing.T) {
	userId := "6ba7b810-9dad-11d1-80b4-00c04fd430cg" // Invalid UUID "g" is not a hex digit
	projectId := uuid.New().String()
	key := "active_signature:" + userId
	expire := time.Now().Add(time.Hour).Unix()
	iValue := map[string]interface{}{
		"user_id":         userId,
		"project_id":      projectId,
		"repository_id":   fmt.Sprintf("%d", REPO_ID),
		"pull_request_id": fmt.Sprintf("%d", PR_ID),
	}
	value, err := json.Marshal(iValue)
	if err != nil {
		t.Fatalf("failed to marshal value: %+v", err)
	}
	putTestItem("projects", "project_id", projectId, "S", map[string]interface{}{}, DEBUG)
	defer deleteTestItem("projects", "project_id", projectId, "S", DEBUG)
	putTestItem("store", "key", key, "S", map[string]interface{}{
		"value":  string(value),
		"expire": expire,
	}, DEBUG)
	defer deleteTestItem("store", "key", key, "S", DEBUG)
	runUserActiveSignatureAPIForUserExpectFail(t, userId)
}

func TestUserActiveSignatureAPIWithNonV4UUID(t *testing.T) {
	userId := "6ba7b810-9dad-11d1-80b4-00c04fd430c8" // Non-v4 UUID
	projectId := uuid.New().String()
	key := "active_signature:" + userId
	expire := time.Now().Add(time.Hour).Unix()
	iValue := map[string]interface{}{
		"user_id":         userId,
		"project_id":      projectId,
		"repository_id":   fmt.Sprintf("%d", REPO_ID),
		"pull_request_id": fmt.Sprintf("%d", PR_ID),
	}
	if rand.Intn(2) == 0 {
		mrId := rand.Intn(100)
		iValue["merge_request_id"] = fmt.Sprintf("%d", mrId)
		iValue["return_url"] = fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/merge_requests/%d", mrId)
	}
	value, err := json.Marshal(iValue)
	if err != nil {
		t.Fatalf("failed to marshal value: %+v", err)
	}
	putTestItem("projects", "project_id", projectId, "S", map[string]interface{}{}, DEBUG)
	defer deleteTestItem("projects", "project_id", projectId, "S", DEBUG)
	putTestItem("store", "key", key, "S", map[string]interface{}{
		"value":  string(value),
		"expire": expire,
	}, DEBUG)
	defer deleteTestItem("store", "key", key, "S", DEBUG)
	runUserActiveSignatureAPIForUser(t, userId)
}

func TestUserActiveSignatureAPI(t *testing.T) {
	userId := USER_UUID
	if userId == "" {
		userId = uuid.New().String()
		projectId := uuid.New().String()
		key := "active_signature:" + userId
		expire := time.Now().Add(time.Hour).Unix()
		iValue := map[string]interface{}{
			"user_id":         userId,
			"project_id":      projectId,
			"repository_id":   fmt.Sprintf("%d", REPO_ID),
			"pull_request_id": fmt.Sprintf("%d", PR_ID),
		}
		if rand.Intn(2) == 0 {
			mrId := rand.Intn(100)
			iValue["merge_request_id"] = fmt.Sprintf("%d", mrId)
			iValue["return_url"] = fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/merge_requests/%d", mrId)
		}
		value, err := json.Marshal(iValue)
		if err != nil {
			t.Fatalf("failed to marshal value: %+v", err)
		}
		putTestItem("projects", "project_id", projectId, "S", map[string]interface{}{}, DEBUG)
		defer deleteTestItem("projects", "project_id", projectId, "S", DEBUG)
		putTestItem("store", "key", key, "S", map[string]interface{}{
			"value":  string(value),
			"expire": expire,
		}, DEBUG)
		defer deleteTestItem("store", "key", key, "S", DEBUG)
	}

	runUserActiveSignatureAPIForUser(t, userId)
}

func TestAllUserActiveSignatureAPI(t *testing.T) {
	allUserActiveSignatures := getAllPrimaryKeys("store", "key", "S")

	var failed []string
	var mtx sync.Mutex
	sem := make(chan struct{}, MAX_PARALLEL)
	var wg sync.WaitGroup

	for _, key := range allUserActiveSignatures {
		ky, ok := key.(string)
		if !ok {
			t.Errorf("Expected string key, got: %T", key)
			continue
		}
		if !strings.HasPrefix(ky, "active_signature:") {
			continue
		}
		userId := strings.TrimPrefix(ky, "active_signature:")

		wg.Add(1)
		sem <- struct{}{}

		go func(userId string) {
			defer wg.Done()
			defer func() { <-sem }()

			t.Run(fmt.Sprintf("UserId=%s", userId), func(t *testing.T) {
				runUserActiveSignatureAPIForUser(t, userId)
				if t.Failed() {
					mtx.Lock()
					failed = append(failed, userId)
					mtx.Unlock()
				}
			})
		}(userId)
	}

	wg.Wait()

	if len(failed) > 0 {
		fmt.Fprintf(os.Stderr, "\nFailed User IDs (%d):\n%s\n\n",
			len(failed),
			strings.Join(failed, "\n"),
		)
		t.Fail()
	} else {
		fmt.Println("\nAll user active signatures passed.")
	}
}

func runUserCompatAPIForUser(t *testing.T, userId string) {
	apiURL := PY_API_URL + fmt.Sprintf(UserCompatAPIPath[0], userId)
	Debugf("Py API call: %s\n", apiURL)
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
	Debugf("Py raw response: %+v\n", string(oldBody))
	Debugf("Py response: %+v\n", oldJSON)

	apiURL = GO_API_URL + fmt.Sprintf(UserCompatAPIPath[1], userId)
	Debugf("Go API call: %s\n", apiURL)
	newResp, err := http.Get(apiURL)
	if err != nil {
		t.Fatalf("Failed to call API: %v", err)
	}
	assert.Equal(t, http.StatusOK, newResp.StatusCode, "Expected 200 from GO API")
	defer newResp.Body.Close()
	newBody, _ := io.ReadAll(newResp.Body)
	var newJSON interface{}
	err = json.Unmarshal(newBody, &newJSON)
	assert.NoError(t, err)
	Debugf("Go raw Response: %+v\n", string(newBody))
	Debugf("Go response: %+v\n", newJSON)

	oldMap, ok1 := oldJSON.(map[string]interface{})
	newMap, ok2 := newJSON.(map[string]interface{})

	if !ok1 || !ok2 {
		t.Fatalf("Expected both responses to be JSON objects")
	}
	compareNestedFields(t, oldMap, newMap, UserCompatAPIKeyMapping, UserCompatAPISortMap)

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
		Debugf("old keys: %+v\n", oky)
		Debugf("new keys: %+v\n", nky)
	}
}

func runUserCompatAPIForUserExpectFail(t *testing.T, userId string) {
	apiURL := PY_API_URL + fmt.Sprintf(UserCompatAPIPath[0], userId)
	Debugf("Py API call: %s\n", apiURL)
	oldResp, err := http.Get(apiURL)
	if err != nil {
		t.Fatalf("Failed to call API: %v", err)
	}
	assert.Equal(t, http.StatusBadRequest, oldResp.StatusCode, "Expected 400 from Py API")
	defer oldResp.Body.Close()
	oldBody, _ := io.ReadAll(oldResp.Body)
	var oldJSON interface{}
	err = json.Unmarshal(oldBody, &oldJSON)
	assert.NoError(t, err)
	Debugf("Py raw response: %+v\n", string(oldBody))
	Debugf("Py response: %+v\n", oldJSON)

	apiURL = GO_API_URL + fmt.Sprintf(UserCompatAPIPath[1], userId)
	Debugf("Go API call: %s\n", apiURL)
	newResp, err := http.Get(apiURL)
	if err != nil {
		t.Fatalf("Failed to call API: %v", err)
	}
	assert.Equal(t, http.StatusUnprocessableEntity, newResp.StatusCode, "Expected 422 from Go API")
	defer newResp.Body.Close()
	newBody, _ := io.ReadAll(newResp.Body)
	var newJSON interface{}
	err = json.Unmarshal(newBody, &newJSON)
	assert.NoError(t, err)
	Debugf("Go raw Response: %+v\n", string(newBody))
	Debugf("Go response: %+v\n", newJSON)

	assert.Equal(t, expectedPyInvalidUUID("user_id"), oldJSON)
	assert.Equal(t, expectedGoInvalidUUID("userID"), newJSON)
}

func TestUserCompatAPI(t *testing.T) {
	userId := USER_UUID
	if userId == "" {
		userId = uuid.New().String()
		companyId := uuid.New().String()
		putTestItem("companies", "company_id", companyId, "S", map[string]interface{}{
			"is_sanctioned": true,
		}, DEBUG)
		defer deleteTestItem("companies", "company_id", companyId, "S", DEBUG)
		putTestItem("users", "user_id", userId, "S", map[string]interface{}{
			"lf_email":             "lgryglicki@cncf.io",
			"lf_sub":               nil,
			"lf_username":          "lukaszgryglicki",
			"note":                 "Example note",
			"user_company_id":      companyId,
			"user_external_id":     "00117000015vpjXAAQ",
			"user_github_id":       43778,
			"user_github_username": "lukaszgryglicki",
			"user_gitlab_id":       567,
			"user_gitlab_username": "lgryglicki",
			"user_ldap_id":         nil,
			"user_name":            "lgryglickilf",
			"version":              "v1",
			"user_emails":          []string{"lgryglicki@cncf.io", "lukaszgryglicki@o2.pl", "lgryglicki@contractor.linuxfoundation.com", "lukaszgryglicki1982@gmail.com"},
		}, DEBUG)
		defer deleteTestItem("users", "user_id", userId, "S", DEBUG)
	}

	runUserCompatAPIForUser(t, userId)
}

func TestUserCompatAPIWithNonV4UUID(t *testing.T) {
	userId := "6ba7b810-9dad-11d1-80b4-00c04fd430c8" // Non-v4 UUID
	companyId := uuid.New().String()
	putTestItem("companies", "company_id", companyId, "S", map[string]interface{}{
		"is_sanctioned": true,
	}, DEBUG)
	defer deleteTestItem("companies", "company_id", companyId, "S", DEBUG)
	putTestItem("users", "user_id", userId, "S", map[string]interface{}{
		"lf_email":             "lgryglicki@cncf.io",
		"lf_sub":               nil,
		"lf_username":          "lukaszgryglicki",
		"note":                 "Example note",
		"user_company_id":      companyId,
		"user_external_id":     "00117000015vpjXAAQ",
		"user_github_id":       43778,
		"user_github_username": "lukaszgryglicki",
		"user_gitlab_id":       567,
		"user_gitlab_username": "lgryglicki",
		"user_ldap_id":         nil,
		"user_name":            "lgryglickilf",
		"version":              "v1",
		"user_emails":          []string{"lgryglicki@cncf.io", "lukaszgryglicki@o2.pl", "lgryglicki@contractor.linuxfoundation.com", "lukaszgryglicki1982@gmail.com"},
	}, DEBUG)
	defer deleteTestItem("users", "user_id", userId, "S", DEBUG)

	runUserCompatAPIForUser(t, userId)
}

func TestUserCompatAPIWithInvalidUUID(t *testing.T) {
	userId := "6ba7b810-9dad-11d1-80b4-00c04fd430cg" // Invalid UUID - "g" is not a hex digit
	companyId := uuid.New().String()
	putTestItem("companies", "company_id", companyId, "S", map[string]interface{}{
		"is_sanctioned": true,
	}, DEBUG)
	defer deleteTestItem("companies", "company_id", companyId, "S", DEBUG)
	putTestItem("users", "user_id", userId, "S", map[string]interface{}{
		"lf_email":             "lgryglicki@cncf.io",
		"lf_sub":               nil,
		"lf_username":          "lukaszgryglicki",
		"note":                 "Example note",
		"user_company_id":      companyId,
		"user_external_id":     "00117000015vpjXAAQ",
		"user_github_id":       43778,
		"user_github_username": "lukaszgryglicki",
		"user_gitlab_id":       567,
		"user_gitlab_username": "lgryglicki",
		"user_ldap_id":         nil,
		"user_name":            "lgryglickilf",
		"version":              "v1",
		"user_emails":          []string{"lgryglicki@cncf.io", "lukaszgryglicki@o2.pl", "lgryglicki@contractor.linuxfoundation.com", "lukaszgryglicki1982@gmail.com"},
	}, DEBUG)
	defer deleteTestItem("users", "user_id", userId, "S", DEBUG)

	runUserCompatAPIForUserExpectFail(t, userId)
}

func TestAllUsersCompatAPI(t *testing.T) {
	allUsers := getAllPrimaryKeys("users", "user_id", "S")

	var failedUsers []string
	var mtx sync.Mutex
	sem := make(chan struct{}, MAX_PARALLEL)
	var wg sync.WaitGroup

	for _, userID := range allUsers {
		usrID, ok := userID.(string)
		if !ok {
			t.Errorf("Expected string user_id, got: %T", userID)
			continue
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(usrID string) {
			defer wg.Done()
			defer func() { <-sem }()

			// Use t.Run in a thread-safe wrapper with a dummy parent test
			t.Run(fmt.Sprintf("UserId=%s", usrID), func(t *testing.T) {
				runUserCompatAPIForUser(t, usrID)
				if t.Failed() {
					mtx.Lock()
					failedUsers = append(failedUsers, usrID)
					mtx.Unlock()
				}
			})
		}(usrID)
	}

	wg.Wait()

	if len(failedUsers) > 0 {
		fmt.Fprintf(os.Stderr, "\nFailed User IDs (%d):\n%s\n\n",
			len(failedUsers),
			strings.Join(failedUsers, "\n"),
		)
		t.Fail() // Mark test as failed
	} else {
		fmt.Println("\nAll users passed.")
	}
}
