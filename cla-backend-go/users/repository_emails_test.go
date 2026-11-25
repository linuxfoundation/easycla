// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package users

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddUserEmailsIsOneAtomicSetAdd(t *testing.T) {
	var updates []map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		if r.Header.Get("X-Amz-Target") == "DynamoDB_20120810.UpdateItem" {
			var request map[string]interface{}
			require.NoError(t, json.Unmarshal(body, &request))
			updates = append(updates, request)
			_, err = io.WriteString(w, `{}`)
		} else {
			_, err = io.WriteString(w, `{"Count":1,"Items":[{"user_id":{"S":"user-1"},"user_emails":{"SS":["lf@example.org","gh@example.org"]}}]}`)
		}
		require.NoError(t, err)
	}))
	defer server.Close()
	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(server.URL),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
		MaxRetries:  aws.Int(0),
	})
	require.NoError(t, err)
	repo := repository{stage: "test", dynamoDBClient: dynamodb.New(awsSession), tableName: "cla-test-users"}

	user, err := repo.AddUserEmails("user-1", []string{" GH@Example.org ", "gh@example.org", ""})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"lf@example.org", "gh@example.org"}, user.Emails)
	require.Len(t, updates, 1)
	assert.Equal(t, "cla-test-users", updates[0]["TableName"])
	assert.Equal(t, "ADD #E :e SET #M = :m", updates[0]["UpdateExpression"])
	assert.Equal(t, "attribute_exists(#U)", updates[0]["ConditionExpression"])
	assert.Equal(t, map[string]interface{}{"#E": "user_emails", "#M": "date_modified", "#U": "user_id"}, updates[0]["ExpressionAttributeNames"])
	values, ok := updates[0]["ExpressionAttributeValues"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, map[string]interface{}{"SS": []interface{}{"gh@example.org"}}, values[":e"])

	_, err = repo.AddUserEmails("user-1", []string{" "})

	require.NoError(t, err)
	assert.Len(t, updates, 1, "nothing is written without an address")
}
