// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
)

// a missing key is expected until the allow-list is provisioned, any other failure is not, so
// the two must stay distinguishable in the logs even when the SDK error arrives wrapped
func TestIsParameterNotFound(t *testing.T) {
	assert.True(t, isParameterNotFound(awserr.New(ssm.ErrCodeParameterNotFound, "not found", nil)))
	assert.True(t, isParameterNotFound(fmt.Errorf("wrapped: %w", awserr.New(ssm.ErrCodeParameterNotFound, "not found", nil))))
	assert.False(t, isParameterNotFound(awserr.New("AccessDeniedException", "denied", nil)))
	assert.False(t, isParameterNotFound(awserr.New("ThrottlingException", "slow down", nil)))
	assert.False(t, isParameterNotFound(errors.New("connection reset")))
}

func TestParseTrustedClientIDs(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		clientIDs []string
	}{
		{"unset", "", nil},
		{"blank", "   ", nil},
		{"separators only", " , ,, ", nil},
		{"single", "ss-client", []string{"ss-client"}},
		{"padded", "  ss-client  ", []string{"ss-client"}},
		{"multiple", "ss-client, other-client ,third-client", []string{"ss-client", "other-client", "third-client"}},
		{"trailing separator", "ss-client,", []string{"ss-client"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.clientIDs, parseTrustedClientIDs(test.value))
		})
	}
}
