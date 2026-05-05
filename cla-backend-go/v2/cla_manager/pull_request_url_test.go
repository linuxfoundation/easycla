// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizePullRequestURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{name: "empty stays empty", in: "", out: ""},
		{name: "whitespace-only treated as empty", in: "   ", out: ""},
		{name: "github PR URL accepted", in: "https://github.com/example-org/example-repo/pull/123", out: "https://github.com/example-org/example-repo/pull/123"},
		{name: "gitlab MR URL accepted", in: "https://gitlab.com/group/project/-/merge_requests/42", out: "https://gitlab.com/group/project/-/merge_requests/42"},
		{name: "gerrit change URL accepted", in: "https://gerrit.googlesource.com/c/project/+/12345", out: "https://gerrit.googlesource.com/c/project/+/12345"},
		{name: "self-hosted https accepted", in: "https://git.example.com/group/project/pull/7", out: "https://git.example.com/group/project/pull/7"},
		{name: "leading and trailing whitespace trimmed", in: "  https://github.com/o/r/pull/1  ", out: "https://github.com/o/r/pull/1"},
		{name: "http rejected", in: "http://github.com/example-org/example-repo/pull/123", out: ""},
		{name: "javascript scheme rejected", in: "javascript:alert(1)", out: ""},
		{name: "data URL rejected", in: "data:text/html,<script>alert(1)</script>", out: ""},
		{name: "relative path rejected", in: "/example-org/example-repo/pull/123", out: ""},
		{name: "missing host rejected", in: "https:///pull/1", out: ""},
		{name: "embedded whitespace rejected", in: "https://github.com/o/r/pull/ 1", out: ""},
		{name: "embedded quote rejected", in: "https://github.com/o/r/pull/1\"", out: ""},
		{name: "embedded angle bracket rejected", in: "https://github.com/o/r/pull/1<x", out: ""},
		{name: "garbage string rejected", in: "not a url", out: ""},
		{name: "scheme casing accepted", in: "HTTPS://github.com/o/r/pull/1", out: "HTTPS://github.com/o/r/pull/1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizePullRequestURL(context.Background(), tc.in)
			assert.Equal(t, tc.out, got)
		})
	}
}
