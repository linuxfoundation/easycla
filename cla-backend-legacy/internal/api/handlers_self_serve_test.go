// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"testing"
)

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

func TestAddSelfServeEmployeeSignerToGerritGroupsIsANoOpWithoutDependencies(t *testing.T) {
	h := &Handlers{}
	for _, lfUsername := range []string{"", "None", "someuser"} {
		h.addSelfServeEmployeeSignerToGerritGroups(context.Background(), "project", "user", lfUsername)
	}
}

func TestSelfServeSessionMatchesProject(t *testing.T) {
	const claGroupID = "aa47b3e1-6f9c-4b6a-9f16-0f9d6a2e1c11"

	tests := []struct {
		name     string
		metadata map[string]any
		expected bool
	}{
		{"same cla group", map[string]any{"project_id": claGroupID}, true},
		{"another cla group", map[string]any{"project_id": "62db1b81-6f4a-4b2e-9a4a-0f2d9f0a1b22"}, false},
		{"missing project", map[string]any{"source": "self-serve"}, true},
		{"blank project", map[string]any{"project_id": "   "}, true},
		{"nil project", map[string]any{"project_id": nil}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := selfServeSessionMatchesProject(test.metadata, claGroupID); got != test.expected {
				t.Errorf("selfServeSessionMatchesProject(%v) = %v, expected %v", test.metadata, got, test.expected)
			}
		})
	}
}
