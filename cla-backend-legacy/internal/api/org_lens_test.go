// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"strings"
	"testing"
)

func TestOrgLensURL(t *testing.T) {
	const account, group = "001000000000000AAA", "00000000-0000-4000-8000-000000000001"
	tests := []struct {
		name, base, sfid, group, want string
	}{
		{"an Account id", "app.example.org", account, group, "https://app.example.org/org/" + account + "/easycla/" + group},
		{"a base with a scheme and a path", "https://app.example.org/lfx/", account, group, "https://app.example.org/lfx/org/" + account + "/easycla/" + group},
		{"no CLA Group", "app.example.org", account, "", "https://app.example.org/org/" + account + "/easycla"},
		{"the setting is unset", "", account, group, ""},
		{"a synthetic id", "app.example.org", "lf0000000000000AAA", group, ""},
		{"a short id", "app.example.org", "001000000000000", group, ""},
		{"no id", "app.example.org", "", group, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := orgLensURL(tt.base, tt.sfid, tt.group); got != tt.want {
				t.Errorf("orgLensURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRequestCompanyAllowlistBodyLinksToTheLensOrTheConsole(t *testing.T) {
	const console, lens = "https://corporate.example.org#/company/c1", "https://app.example.org/org/001000000000000AAA/easycla/g1"
	withLens := requestCompanyAllowlistBody("Boss", "Project X", "Mallory", "m@example.invalid", "Acme", "Acme Inc", "please", console, lens)
	if !strings.Contains(withLens, `<a href="`+lens+`" target="_blank">Project X in the Organization lens</a>`) {
		t.Errorf("the lens link is missing: %s", withLens)
	}
	if strings.Contains(withLens, console) || strings.Contains(withLens, "Corporate") {
		t.Errorf("the Corporate Console must not be named next to the lens link: %s", withLens)
	}
	withConsole := requestCompanyAllowlistBody("Boss", "Project X", "Mallory", "m@example.invalid", "Acme", "Acme Inc", "please", console, "")
	if !strings.Contains(withConsole, `<a href="`+console+`" target="_blank">log into the EasyCLA Corporate`) || strings.Contains(withConsole, "Organization lens") {
		t.Errorf("without a lens link the Corporate Console copy stays: %s", withConsole)
	}
}
