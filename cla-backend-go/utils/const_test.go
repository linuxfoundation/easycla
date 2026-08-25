// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import "testing"

func TestIsSelfServeActiveSignature(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		expected bool
	}{
		{"nil metadata", nil, false},
		{"no source", map[string]interface{}{"project_id": "abc"}, false},
		{"non string source", map[string]interface{}{"source": 1}, false},
		{"pull request source", map[string]interface{}{"repository_id": "1", "pull_request_id": "2"}, false},
		{"self serve source", map[string]interface{}{"source": SelfServeSignatureSource}, true},
		{"mixed case source", map[string]interface{}{"source": "Self-Serve"}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsSelfServeActiveSignature(test.metadata); got != test.expected {
				t.Errorf("IsSelfServeActiveSignature(%v) = %v, expected %v", test.metadata, got, test.expected)
			}
		})
	}
}
