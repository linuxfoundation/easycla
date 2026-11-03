// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import "testing"

func TestOrgLensURL(t *testing.T) {
	const (
		sfid       = "001000000000000AAA"
		claGroupID = "00000000-0000-4000-8000-000000000001"
	)
	tests := []struct {
		name        string
		base        string
		companySFID string
		claGroupID  string
		want        string
	}{
		{"unset base means no lens link", "", sfid, claGroupID, ""},
		{"whitespace base means no lens link", "  ", sfid, claGroupID, ""},
		{"CLA Group page", "https://app.example.org", sfid, claGroupID, "https://app.example.org/org/" + sfid + "/easycla/" + claGroupID},
		{"trailing slash on base", "https://app.example.org/", sfid, claGroupID, "https://app.example.org/org/" + sfid + "/easycla/" + claGroupID},
		{"base without scheme gets https", "app.example.org", sfid, claGroupID, "https://app.example.org/org/" + sfid + "/easycla/" + claGroupID},
		{"base with a path prefix", "https://example.org/lfx/", sfid, claGroupID, "https://example.org/lfx/org/" + sfid + "/easycla/" + claGroupID},
		{"base query and fragment dropped", "https://app.example.org/?x=1#y", sfid, claGroupID, "https://app.example.org/org/" + sfid + "/easycla/" + claGroupID},
		{"no CLA Group is the company EasyCLA page", "https://app.example.org", sfid, "", "https://app.example.org/org/" + sfid + "/easycla"},
		{"no company SFID is the lens home", "https://app.example.org", "", claGroupID, "https://app.example.org/org"},
		{"ids cannot add path segments", "https://app.example.org", "a/b", "c/../d", "https://app.example.org/org/a%2Fb/easycla/c%2F..%2Fd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OrgLensURL(tt.base, tt.companySFID, tt.claGroupID); got != tt.want {
				t.Errorf("OrgLensURL(%q, %q, %q) = %q, want %q", tt.base, tt.companySFID, tt.claGroupID, got, tt.want)
			}
		})
	}
}
