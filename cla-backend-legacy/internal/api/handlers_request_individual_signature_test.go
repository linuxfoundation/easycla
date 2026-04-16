// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"encoding/json"
	"testing"
)

func TestTranslateLegacyIndividualSignatureV4Error(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		status       int
		body         string
		translated   bool
		expected     map[string]any
	}{
		{
			name:         "github no emails found",
			providerType: "github",
			status:       400,
			body:         `{"message":"no emails found"}`,
			translated:   true,
			expected: map[string]any{
				"errors": map[string]any{"user_id": "no github user_emails found"},
			},
		},
		{
			name:         "gitlab no emails found",
			providerType: "gitlab",
			status:       400,
			body:         `{"message":"no emails found"}`,
			translated:   true,
			expected: map[string]any{
				"errors": map[string]any{"user_id": "no gitlab user_emails found"},
			},
		},
		{
			name:         "gerrit is not translated",
			providerType: "gerrit",
			status:       400,
			body:         `{"message":"no emails found"}`,
			translated:   false,
		},
		{
			name:         "non email error is not translated",
			providerType: "github",
			status:       400,
			body:         `{"message":"something else"}`,
			translated:   false,
		},
		{
			name:         "success response is not translated",
			providerType: "github",
			status:       200,
			body:         `{"message":"no emails found"}`,
			translated:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, translated := translateLegacyIndividualSignatureV4Error(tc.providerType, tc.status, []byte(tc.body))
			if translated != tc.translated {
				t.Fatalf("translated=%v, want %v", translated, tc.translated)
			}
			if !tc.translated {
				if string(got) != tc.body {
					t.Fatalf("body changed unexpectedly: got %s want %s", string(got), tc.body)
				}
				return
			}

			var gotObj map[string]any
			if err := json.Unmarshal(got, &gotObj); err != nil {
				t.Fatalf("unmarshal translated body: %v", err)
			}
			if gotErrors, ok := gotObj["errors"].(map[string]any); ok {
				if wantErrors, ok := tc.expected["errors"].(map[string]any); ok {
					if gotErrors["user_id"] != wantErrors["user_id"] {
						t.Fatalf("translated user_id=%v, want %v", gotErrors["user_id"], wantErrors["user_id"])
					}
					return
				}
			}
			t.Fatalf("translated body missing expected errors object: %v", gotObj)
		})
	}
}
