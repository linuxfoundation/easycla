// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerRejectsUnreadableSSS(t *testing.T) {
	const endpointEnv = "EASYCLA_TEST_SERVER_SSS_ENDPOINT"
	const configLoaded = "non-screening configuration loaded"
	const falseValue = "false"
	if endpoint := os.Getenv(endpointEnv); endpoint != "" {
		ini.Init()
		awsSession, err := ini.GetAWSSession()
		require.NoError(t, err)
		awsSession.Config.Endpoint = aws.String(endpoint)
		awsSession.Config.Credentials = credentials.NewStaticCredentials("test-access-key", "test-secret-key", "")
		awsSession.Config.MaxRetries = aws.Int(0)
		ini.ConfigVariable()
		fmt.Println(configLoaded)
		server(false)
		t.Fatal("API startup accepted an unreadable screening policy")
		return
	}

	ssmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct{ Name string }
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		if strings.HasPrefix(input.Name, "cla-sss-") {
			w.WriteHeader(http.StatusBadRequest)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{
				"__type": "ThrottlingException", "message": "Rate exceeded",
			}))
			return
		}
		value := "test-value"
		if strings.HasPrefix(input.Name, "cla-gh-app-id-") {
			value = "123"
		} else if strings.Contains(input.Name, "-enabled-") || strings.Contains(input.Name, "-enable-services-for-parent-") {
			value = falseValue
		}
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"Parameter": map[string]string{"Name": input.Name, "Value": value, "Type": "String"},
		}))
	}))
	defer ssmServer.Close()
	t.Setenv(endpointEnv, ssmServer.URL)
	t.Setenv("STAGE", "test")
	t.Setenv("DYNAMODB_AWS_REGION", "us-east-1")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, name := range []string{"DDB_API_LOGGING", "OTEL_DATADOG_API_LOGGING"} {
		t.Setenv(name, falseValue)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestServerRejectsUnreadableSSS$", "-test.v") // #nosec G204 -- re-executes this test binary with fixed test arguments.
	output, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "%s", output)
	assert.Equal(t, 1, exitErr.ExitCode(), "%s", output)
	assert.Contains(t, string(output), configLoaded)
	assert.Contains(t, string(output), "SSS configuration unreadable: unable to read SSS parameter")
	assert.Contains(t, string(output), "ThrottlingException")
	assert.NotContains(t, string(output), "SSS is required but not configured")
}
