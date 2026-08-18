// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
