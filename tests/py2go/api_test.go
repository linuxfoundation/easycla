package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	MAX_PARALLEL         int
	PROJECT_UUID         string
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
	ProjectCompatAPISortMap = map[string]string{
		"github_repos": "repository_name",
		"gitlab_repos": "repository_name",
		"gerrit_repos": "gerrit_url",
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
	MAX_PARALLEL = 1
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
				t.Errorf("Datetime mismatch for key '%s' (Go: '%s'): %s != %s", pyKey, goKey, pyTime, goTime)
			}
			continue
		}

		if fmt.Sprint(pyVal) != fmt.Sprint(goVal) {
			t.Errorf("Mismatch for key '%s' (Go: '%s'): %v != %v", pyKey, goKey, pyVal, goVal)
		}
	}
}

func sortByKey(arr []interface{}, key string) {
	sort.Slice(arr, func(i, j int) bool {
		m1, _ := arr[i].(map[string]interface{})
		m2, _ := arr[j].(map[string]interface{})
		s1 := fmt.Sprint(m1[key])
		s2 := fmt.Sprint(m2[key])
		return s1 < s2
	})
}

func compareNestedFields(t *testing.T, pyData, goData, keyMapping map[string]interface{}, sortMap map[string]string) {
	for k, v := range keyMapping {
		if v == nil {
			Debugf("checking values of '%s'\n", k)
		}

		pyVal, pyOk := pyData[k]
		goVal, goOk := goData[k]
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
				Debugf("sorting '%s' key values by %s\n", k, sortKey)
				sortByKey(pyArrayVal, sortKey)
				sortByKey(goArrayVal, sortKey)
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
				t.Errorf("Datetime mismatch for key '%s': %s != %s", k, pyTime, goTime)
			}
			continue
		}

		if fmt.Sprint(pyVal) != fmt.Sprint(goVal) {
			t.Errorf("Mismatch for key '%s': %v != %v", k, pyVal, goVal)
		}
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
