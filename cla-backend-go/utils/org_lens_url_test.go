// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import "testing"

func TestOrgLensURL(t *testing.T) {
	const (
		sfid       = "001000000000000AAA"
		claGroupID = "00000000-0000-4000-8000-000000000001"
	)
	SetOrgLensLiveCheck(func(string) bool { return true })
	t.Cleanup(func() { SetOrgLensLiveCheck(nil) })
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
		{"surrounding whitespace on the SFID is trimmed", "https://app.example.org", " " + sfid + " ", claGroupID, "https://app.example.org/org/" + sfid + "/easycla/" + claGroupID},
		{"empty company SFID means no lens link", "https://app.example.org", "", claGroupID, ""},
		{"Org Service lf ID means no lens link", "https://app.example.org", "lf0000000000000AAA", claGroupID, ""},
		{"random 18-character ID means no lens link", "https://app.example.org", "a0P000000000000AAA", claGroupID, ""},
		{"15-character Salesforce ID means no lens link", "https://app.example.org", "001000000000000", claGroupID, ""},
		{"SFID with a path separator means no lens link", "https://app.example.org", "001000000000/00AAA", claGroupID, ""},
		{"CLA Group ID cannot add path segments", "https://app.example.org", sfid, "c/../d", "https://app.example.org/org/" + sfid + "/easycla/c%2F..%2Fd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OrgLensURL(tt.base, tt.companySFID, tt.claGroupID); got != tt.want {
				t.Errorf("OrgLensURL(%q, %q, %q) = %q, want %q", tt.base, tt.companySFID, tt.claGroupID, got, tt.want)
			}
		})
	}
}

func TestOrgLensURLNeedsALiveAccount(t *testing.T) {
	const (
		sfid = "001000000000000AAA"
		base = "https://app.example.org"
		page = base + "/org/" + sfid + "/easycla"
	)
	t.Cleanup(func() { SetOrgLensLiveCheck(nil) })

	SetOrgLensLiveCheck(nil)
	if got := OrgLensURL(base, sfid, ""); got != "" {
		t.Errorf("without a check: got %q, want no lens link", got)
	}

	var asked []string
	live := false
	SetOrgLensLiveCheck(func(companySFID string) bool {
		asked = append(asked, companySFID)
		return live
	})
	if got := OrgLensURL(base, sfid, ""); got != "" {
		t.Errorf("Account that is not live: got %q, want no lens link", got)
	}
	live = true
	if got := OrgLensURL(base, " "+sfid+" ", ""); got != page {
		t.Errorf("live Account: got %q, want %q", got, page)
	}

	OrgLensURL("", sfid, "")
	OrgLensURL("://", sfid, "")
	OrgLensURL(base, "lf0000000000000AAA", "")
	if len(asked) != 2 || asked[0] != sfid || asked[1] != sfid {
		t.Errorf("the check is asked once per link, and only for an Account id with a usable base: %q", asked)
	}
}
