// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stretchr/testify/assert"
)

func TestShellQuoteKeepsInjectedValuesLiteral(t *testing.T) {
	line := buildAwsCliUpdate("us-east-1", "dev", "cla-dev-signatures", "sig-1", "SET #date_created = :date_created", map[string]*string{"#date_created": aws.String("date_created")},
		map[string]*dynamodb.AttributeValue{":date_created": {S: aws.String("2024-01-01T00:00:00Z';id;'")}}, "attribute_not_exists(#date_created)")
	assert.Contains(t, line, `2024-01-01T00:00:00Z'\'';id;'\''`, "every single quote inside a value is closed, escaped and reopened")
	assert.Equal(t, 2, strings.Count(line, `'\''`))
	assert.Equal(t, 1, strings.Count(line, "aws --profile"))
	assert.Equal(t, `'it'\''s'`, shellQuote("it's"))
	assert.Equal(t, `'a$(id)`+"`"+`b'`, shellQuote("a$(id)`b"), "$( and backticks stay literal inside single quotes")
}

func TestPickersIgnoreUnparseableCandidates(t *testing.T) {
	only := []pair{{src: "sign_url", ts: "not-a-timestamp"}}
	assert.Equal(t, pair{}, pickEarliest(only))
	assert.Equal(t, pair{}, pickLatest(only))
	mixed := []pair{{src: "sign_url", ts: "garbage"}, {src: "signed_on", ts: "2024-05-01T10:00:00Z"}, {src: "docusign", ts: "2024-04-01T10:00:00Z"}}
	assert.Equal(t, "docusign", pickEarliest(mixed).src)
	assert.Equal(t, "signed_on", pickLatest(mixed).src)
}
