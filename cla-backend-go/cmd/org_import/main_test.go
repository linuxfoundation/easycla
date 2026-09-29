// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunUsageErrors(t *testing.T) {
	t.Setenv("STAGE", "")
	assert.Equal(t, 2, run(nil, strings.NewReader("")))
	assert.Equal(t, 2, run([]string{"bogus"}, strings.NewReader("")))
	assert.Equal(t, 2, run([]string{"ingest", "--nope"}, strings.NewReader("")))
	assert.Equal(t, 2, run([]string{"ingest"}, strings.NewReader("")), "STAGE is required")
	t.Setenv("STAGE", "dev")
	assert.Equal(t, 2, run([]string{"ingest", "--routes", "delete"}, strings.NewReader("")))
}

func TestConfirm(t *testing.T) {
	var out bytes.Buffer
	assert.True(t, confirm("dev", &out, strings.NewReader("dev\n")))
	assert.Contains(t, out.String(), "Type the stage name")
	assert.False(t, confirm("dev", &out, strings.NewReader("prod\n")))
	assert.False(t, confirm("dev", &out, strings.NewReader("")))
	assert.True(t, confirm("prod", &out, strings.NewReader("prod")), "last line without newline")
}

func TestRunnerAndRevision(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "42")
	assert.Equal(t, "gha-42", runnerName())
	t.Setenv("GITHUB_RUN_ID", "")
	t.Setenv("USER", "lukasz")
	assert.True(t, strings.HasPrefix(runnerName(), "lukasz@"))
	assert.NotEmpty(t, buildRevision())
	commit, branch = "abc123", "unicron-x"
	defer func() { commit, branch = "", "" }()
	assert.Equal(t, "abc123 (unicron-x)", buildRevision(), "Makefile -X values win over VCS stamping")
	branch = ""
	assert.Equal(t, "abc123", buildRevision())
}
