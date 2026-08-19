// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import "testing"

func TestIsSelfServeSignatureMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		expected bool
	}{
		{"nil metadata", nil, false},
		{"no source", map[string]any{"repository_id": "1", "pull_request_id": "2"}, false},
		{"nil source", map[string]any{"source": nil}, false},
		{"self serve source", map[string]any{"source": "self-serve"}, true},
		{"mixed case source", map[string]any{"source": "Self-Serve"}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isSelfServeSignatureMetadata(test.metadata); got != test.expected {
				t.Errorf("isSelfServeSignatureMetadata(%v) = %v, expected %v", test.metadata, got, test.expected)
			}
		})
	}
}
